// Package update verifies signed releases and replaces the client executable.
package update

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	MaxManifestSize int64 = 1 << 20
	MaxArtifactSize int64 = 64 << 20
)

type Envelope struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

type Artifact struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	Schema    int                 `json:"schema"`
	Version   string              `json:"version"`
	Artifacts map[string]Artifact `json:"artifacts"`
}

// DescribeArtifact measures a local release file using the same bounded digest
// calculation as the downloader. It never reads more than the release limit.
func DescribeArtifact(path string) (Artifact, error) {
	f, err := os.Open(path)
	if err != nil {
		return Artifact{}, err
	}
	defer f.Close()
	size, hash, err := digestCopy(io.Discard, f, MaxArtifactSize)
	if err != nil {
		return Artifact{}, err
	}
	if size <= 0 || size > MaxArtifactSize {
		return Artifact{}, fmt.Errorf("artifact size %d is outside the supported limit", size)
	}
	return Artifact{Path: filepath.Base(path), Size: size, SHA256: hash}, nil
}

func digestCopy(dst io.Writer, src io.Reader, limit int64) (int64, string, error) {
	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(dst, h), io.LimitReader(src, limit+1))
	return size, hex.EncodeToString(h.Sum(nil)), err
}

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var filenamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func versionParts(version string) ([3]uint64, error) {
	var values [3]uint64
	if !versionPattern.MatchString(version) {
		return values, fmt.Errorf("invalid release version %q", version)
	}
	for i, p := range strings.Split(version, ".") {
		n, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return values, fmt.Errorf("invalid release version %q: %w", version, err)
		}
		values[i] = n
	}
	return values, nil
}

// CompareVersions compares canonical three-component numeric release versions.
func CompareVersions(a, b string) (int, error) {
	av, err := versionParts(a)
	if err != nil {
		return 0, err
	}
	bv, err := versionParts(b)
	if err != nil {
		return 0, err
	}
	for i := range av {
		if av[i] < bv[i] {
			return -1, nil
		}
		if av[i] > bv[i] {
			return 1, nil
		}
	}
	return 0, nil
}

func decodeJSON(data []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("unexpected trailing JSON")
	}
	return nil
}

func validateManifest(m Manifest) error {
	if m.Schema != 1 {
		return fmt.Errorf("unsupported manifest schema %d", m.Schema)
	}
	if _, err := versionParts(m.Version); err != nil {
		return err
	}
	if len(m.Artifacts) == 0 {
		return errors.New("manifest contains no artifacts")
	}
	for target, a := range m.Artifacts {
		if target != "linux/amd64" && target != "linux/arm64" {
			return fmt.Errorf("unsupported artifact target %q", target)
		}
		if !filenamePattern.MatchString(a.Path) || strings.Contains(a.Path, "..") {
			return fmt.Errorf("unsafe artifact path %q", a.Path)
		}
		if a.Size <= 0 || a.Size > MaxArtifactSize {
			return fmt.Errorf("invalid artifact size %d", a.Size)
		}
		h, err := hex.DecodeString(a.SHA256)
		if err != nil || len(h) != 32 {
			return fmt.Errorf("invalid SHA-256 for %s", target)
		}
	}
	return nil
}

// VerifyManifest authenticates the raw decoded payload before interpreting it.
func VerifyManifest(data []byte, publicKey ed25519.PublicKey) (Manifest, error) {
	var m Manifest
	if int64(len(data)) > MaxManifestSize {
		return m, errors.New("manifest exceeds 1 MiB")
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return m, errors.New("invalid trusted Ed25519 key")
	}
	var envelope Envelope
	if err := decodeJSON(data, &envelope); err != nil {
		return m, fmt.Errorf("invalid manifest envelope: %w", err)
	}
	if strings.ContainsAny(envelope.Payload+envelope.Signature, "\r\n") {
		return m, errors.New("base64 must use a single line")
	}
	payload, err := base64.StdEncoding.Strict().DecodeString(envelope.Payload)
	if err != nil {
		return m, fmt.Errorf("invalid payload base64: %w", err)
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(envelope.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return m, errors.New("invalid Ed25519 signature encoding")
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return m, errors.New("manifest signature verification failed")
	}
	if err := decodeJSON(payload, &m); err != nil {
		return Manifest{}, fmt.Errorf("invalid signed payload: %w", err)
	}
	if err := validateManifest(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// SignManifest is shared by the offline release command and its verifier.
func SignManifest(m Manifest, key ed25519.PrivateKey) ([]byte, error) {
	if len(key) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid Ed25519 signing key")
	}
	if err := validateManifest(m); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(Envelope{Payload: base64.StdEncoding.EncodeToString(payload), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))})
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > MaxManifestSize {
		return nil, errors.New("manifest exceeds 1 MiB")
	}
	return append(data, '\n'), nil
}
