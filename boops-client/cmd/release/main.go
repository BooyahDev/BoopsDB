// release builds and signs an immutable local Linux client release.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"boops/update"
)

func main() {
	version := flag.String("version", "", "canonical release version, e.g. 0.3.0")
	key := flag.String("key", "", "PKCS#8 Ed25519 signing-key PEM outside the repository")
	out := flag.String("out", "", "new local release directory (must not already exist)")
	flag.Parse()
	if flag.NArg() != 0 || *key == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: release -version 0.3.0 -key <private-key-pem> -out <new-output-dir>")
		os.Exit(1)
	}
	pub, err := update.TrustedPublicKey()
	if err == nil {
		err = createRelease(context.Background(), *version, *key, *out, pub, buildArtifact)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
	fmt.Println("Signed release created:", *out)
}

func readKey(path string) (ed25519.PrivateKey, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("signing key must be a regular file readable only by its owner")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, rest := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" || len(bytes.TrimSpace(rest)) != 0 {
		return nil, fmt.Errorf("signing key must be a single PKCS#8 PRIVATE KEY PEM")
	}
	value, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS#8 signing key: %w", err)
	}
	key, ok := value.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("signing key must use Ed25519")
	}
	return key, nil
}

func createRelease(ctx context.Context, version, keyPath, out string, trusted ed25519.PublicKey, build func(context.Context, string, string, string) error) error {
	if _, err := update.CompareVersions(version, version); err != nil {
		return err
	}
	key, err := readKey(keyPath)
	if err != nil {
		return err
	}
	if len(trusted) != ed25519.PublicKeySize || !bytes.Equal(key.Public().(ed25519.PublicKey), trusted) {
		return fmt.Errorf("signing key does not match the embedded trusted public key")
	}
	if _, err = os.Lstat(out); err == nil {
		return fmt.Errorf("output directory already exists; releases are never overwritten")
	} else if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(out)
	if err = os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(parent, ".boops-release-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	manifest := update.Manifest{Schema: 1, Version: version, Artifacts: make(map[string]update.Artifact)}
	for _, arch := range []string{"amd64", "arm64"} {
		name := "boops_" + version + "_" + arch + ".binary"
		path := filepath.Join(staging, name)
		if err = build(ctx, arch, version, path); err != nil {
			return fmt.Errorf("build linux/%s: %w", arch, err)
		}
		artifact, err := update.DescribeArtifact(path)
		if err != nil {
			return fmt.Errorf("describe linux/%s artifact: %w", arch, err)
		}
		if err = os.Chmod(path, 0755); err != nil {
			return err
		}
		manifest.Artifacts["linux/"+arch] = artifact
	}
	data, err := update.SignManifest(manifest, key)
	if err != nil {
		return err
	}
	if _, err = update.VerifyManifest(data, trusted); err != nil {
		return fmt.Errorf("verify generated release: %w", err)
	}
	if err = os.WriteFile(filepath.Join(staging, "latest.json"), data, 0644); err != nil {
		return err
	}
	pubDER, err := x509.MarshalPKIXPublicKey(trusted)
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(staging, "public-key.pem"), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}), 0644); err != nil {
		return err
	}
	var checksums strings.Builder
	for _, arch := range []string{"amd64", "arm64"} {
		artifact := manifest.Artifacts["linux/"+arch]
		fmt.Fprintf(&checksums, "%s  %s\n", artifact.SHA256, artifact.Path)
	}
	if err = os.WriteFile(filepath.Join(staging, "SHA256SUMS"), []byte(checksums.String()), 0644); err != nil {
		return err
	}
	if err = os.Chmod(staging, 0755); err != nil {
		return err
	}
	// All generated files have been verified before exposing the directory.
	return os.Rename(staging, out)
}

func buildArtifact(ctx context.Context, arch, version, dst string) error {
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags", "-s -w -X main.version="+version, "-o", dst, ".")
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "GOOS=") && !strings.HasPrefix(env, "GOARCH=") && !strings.HasPrefix(env, "CGO_ENABLED=") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	cmd.Env = append(cmd.Env, "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
