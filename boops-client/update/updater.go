package update

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const DefaultBaseURL = "https://file.booyah.dev/BoopsDB-Client/"

type Options struct {
	CurrentVersion string
	BaseURL        string
	ExecutablePath string
	StateDir       string
	GOOS           string
	GOARCH         string
	PublicKey      ed25519.PublicKey
	Force          bool
	HTTPClient     *http.Client
	Now            func() time.Time
	Probe          func(context.Context, string, string) error
}

type Result struct {
	Updated       bool
	Version       string
	SkippedReason string
}

type checkState struct {
	LastCheck time.Time `json:"last_check"`
}

func validateURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" || u.Host != "file.booyah.dev" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.RawPath != "" || path.Clean(u.Path) != u.Path || !strings.HasPrefix(u.Path, "/BoopsDB-Client/") {
		return nil, errors.New("update URL must use the fixed HTTPS distribution host")
	}
	return u, nil
}

func updateClient(client *http.Client) *http.Client {
	var c http.Client
	if client != nil {
		c = *client
	}
	previous := c.CheckRedirect
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many update redirects")
		}
		if _, err := validateURL(req.URL.String()); err != nil {
			return err
		}
		if previous != nil {
			return previous(req, via)
		}
		return nil
	}
	return &c
}

func get(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	if _, err := validateURL(url); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("update request returned HTTP %d", resp.StatusCode)
	}
	return resp, nil
}

// Check holds a nonblocking process lock until verification, replacement, and
// cadence-state persistence finish. Registration and machine state are untouched.
func Check(ctx context.Context, options Options) (result Result, err error) {
	if options.GOOS == "" {
		options.GOOS = runtime.GOOS
	}
	if options.GOARCH == "" {
		options.GOARCH = runtime.GOARCH
	}
	if options.GOOS != "linux" || (options.GOARCH != "amd64" && options.GOARCH != "arm64") {
		return Result{SkippedReason: "unsupported"}, nil
	}
	if _, e := versionParts(options.CurrentVersion); e != nil {
		return Result{SkippedReason: "development"}, nil
	}
	if options.BaseURL == "" {
		options.BaseURL = DefaultBaseURL
	}
	if options.BaseURL != DefaultBaseURL {
		return result, errors.New("update base URL must be the fixed HTTPS distribution directory")
	}
	if options.StateDir == "" {
		options.StateDir = "/etc/boops"
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Probe == nil {
		options.Probe = probeVersion
	}
	if options.PublicKey == nil {
		options.PublicKey, err = TrustedPublicKey()
		if err != nil {
			return result, err
		}
	}
	if len(options.PublicKey) != ed25519.PublicKeySize {
		return result, errors.New("invalid trusted Ed25519 key")
	}
	if options.ExecutablePath == "" {
		options.ExecutablePath, err = os.Executable()
		if err != nil {
			return result, err
		}
	}
	if err = os.MkdirAll(options.StateDir, 0700); err != nil {
		return result, fmt.Errorf("create update state directory: %w", err)
	}
	lock, e := acquireLock(filepath.Join(options.StateDir, "update.lock"))
	if e != nil {
		if errors.Is(e, errLocked) && !options.Force {
			return Result{SkippedReason: "locked"}, nil
		}
		return result, fmt.Errorf("lock update: %w", e)
	}
	defer lock.Close()
	now := options.Now().UTC()
	statePath := filepath.Join(options.StateDir, "update-state.json")
	if !options.Force {
		data, e := os.ReadFile(statePath)
		var state checkState
		if e == nil && json.Unmarshal(data, &state) == nil && !state.LastCheck.IsZero() && !state.LastCheck.After(now) && now.Sub(state.LastCheck) < time.Hour {
			return Result{SkippedReason: "cadence"}, nil
		}
	}
	// Failed communication is throttled too. If persistence fails after the
	// rename, the caller receives Updated=true along with the persistence error.
	defer func() {
		if e := saveCheckState(statePath, now); e != nil {
			err = errors.Join(err, fmt.Errorf("save update state: %w", e))
		}
	}()
	client := updateClient(options.HTTPClient)
	manifestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	resp, e := get(manifestCtx, client, options.BaseURL+"latest.json")
	if e != nil {
		cancel()
		return result, e
	}
	data, e := io.ReadAll(io.LimitReader(resp.Body, MaxManifestSize+1))
	resp.Body.Close()
	cancel()
	if e != nil {
		return result, e
	}
	m, e := VerifyManifest(data, options.PublicKey)
	if e != nil {
		return result, e
	}
	result.Version = m.Version
	comparison, e := CompareVersions(m.Version, options.CurrentVersion)
	if e != nil {
		return result, e
	}
	if comparison <= 0 {
		result.SkippedReason = "not-newer"
		return result, nil
	}
	artifact, ok := m.Artifacts[options.GOOS+"/"+options.GOARCH]
	if !ok {
		return result, errors.New("release has no artifact for this platform")
	}
	result.Updated, e = replaceExecutable(ctx, client, options, artifact, m.Version)
	return result, e
}

func saveCheckState(path string, now time.Time) error {
	data, err := json.Marshal(checkState{LastCheck: now})
	if err != nil {
		return err
	}
	return atomicWrite(path, append(data, '\n'), 0600)
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".boops-state-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

func syncDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func replaceExecutable(ctx context.Context, client *http.Client, options Options, artifact Artifact, version string) (updated bool, err error) {
	info, err := os.Lstat(options.ExecutablePath)
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, errors.New("current executable must be a regular file")
	}
	if info.Mode().Perm()&0111 == 0 {
		return false, errors.New("current executable has no execution permission")
	}
	dir := filepath.Dir(options.ExecutablePath)
	f, err := os.CreateTemp(dir, ".boops-update-*")
	if err != nil {
		return false, err
	}
	name := f.Name()
	defer os.Remove(name)
	defer f.Close()
	downloadCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	resp, err := get(downloadCtx, client, options.BaseURL+artifact.Path)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.ContentLength > MaxArtifactSize || resp.ContentLength > artifact.Size {
		return false, errors.New("artifact response exceeds signed size")
	}
	size, hash, err := digestCopy(f, resp.Body, artifact.Size)
	if err != nil {
		return false, err
	}
	if size != artifact.Size {
		return false, fmt.Errorf("artifact size mismatch: got %d, expected %d", size, artifact.Size)
	}
	if !strings.EqualFold(hash, artifact.SHA256) {
		return false, errors.New("artifact SHA-256 mismatch")
	}
	if err = f.Chmod(info.Mode().Perm()); err != nil {
		return false, err
	}
	if err = f.Sync(); err != nil {
		return false, err
	}
	if err = f.Close(); err != nil {
		return false, err
	}
	probeCtx, probeCancel := context.WithTimeout(ctx, 15*time.Second)
	err = options.Probe(probeCtx, name, version)
	probeCancel()
	if err != nil {
		return false, fmt.Errorf("candidate version probe: %w", err)
	}
	if err = copyPrevious(options.ExecutablePath, info.Mode().Perm()); err != nil {
		return false, fmt.Errorf("preserve previous executable: %w", err)
	}
	if err = os.Rename(name, options.ExecutablePath); err != nil {
		return false, fmt.Errorf("replace executable: %w", err)
	}
	// The rename is committed even if directory durability cannot be confirmed.
	return true, syncDir(dir)
}

func copyPrevious(path string, mode os.FileMode) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	f, err := os.CreateTemp(filepath.Dir(path), ".boops-previous-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	defer f.Close()
	if _, err = io.Copy(f, in); err == nil {
		err = f.Chmod(mode)
	}
	if err == nil {
		err = f.Sync()
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, path+".previous"); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

func probeVersion(ctx context.Context, path, version string) error {
	var out limitedOutput
	command := exec.CommandContext(ctx, path, "version")
	command.Stdout = &out
	command.Stderr = &out
	if err := command.Run(); err != nil {
		return err
	}
	if strings.TrimSpace(out.String()) != version {
		return errors.New("candidate reported a different version")
	}
	return nil
}

// Bound command output so a faulty signed candidate cannot exhaust memory.
type limitedOutput struct{ bytes []byte }

func (w *limitedOutput) Write(p []byte) (int, error) {
	if len(w.bytes)+len(p) > 4096 {
		return 0, errors.New("version output exceeds 4 KiB")
	}
	w.bytes = append(w.bytes, p...)
	return len(p), nil
}
func (w *limitedOutput) String() string { return string(w.bytes) }
