package update

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fixtureTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c := r.Clone(r.Context())
	u := *r.URL
	u.Scheme = f.target.Scheme
	u.Host = f.target.Host
	c.URL = &u
	resp, err := f.base.RoundTrip(c)
	if resp != nil {
		resp.Request = r
	}
	return resp, err
}

func releaseEnvelope(t *testing.T, key ed25519.PrivateKey, version string, binary []byte) []byte {
	h := sha256.Sum256(binary)
	m := Manifest{Schema: 1, Version: version, Artifacts: map[string]Artifact{"linux/amd64": {Path: "boops_" + version + "_amd64.binary", Size: int64(len(binary)), SHA256: hex.EncodeToString(h[:])}}}
	payload, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return signedBytes(t, payload, key)
}

func fixtureOptions(t *testing.T, handler http.HandlerFunc, pub ed25519.PublicKey) Options {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	client := &http.Client{Transport: fixtureTransport{target: u, base: srv.Client().Transport}}
	dir := t.TempDir()
	exe := filepath.Join(dir, "boops")
	if err := os.WriteFile(exe, []byte("old executable"), 0751); err != nil {
		t.Fatal(err)
	}
	return Options{CurrentVersion: "0.2.0", BaseURL: "https://file.booyah.dev/BoopsDB-Client/", ExecutablePath: exe, StateDir: filepath.Join(dir, "state"), GOOS: "linux", GOARCH: "amd64", PublicKey: pub, HTTPClient: client, Now: func() time.Time { return time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC) }, Probe: func(_ context.Context, path, version string) error {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if string(b) != "new executable" || version != "0.3.0" {
			return errors.New("unexpected probe input")
		}
		return nil
	}}
}

func TestCheckPreservesRegistrationState(t *testing.T) {
	pub, key := fixtureKey(t)
	binary := []byte("new executable")
	env := releaseEnvelope(t, key, "0.3.0", binary)
	opts := fixtureOptions(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "latest.json") {
			w.Write(env)
		} else {
			w.Write(binary)
		}
	}, pub)
	if err := os.MkdirAll(opts.StateDir, 0700); err != nil {
		t.Fatal(err)
	}
	config := []byte("{\n  \"id\": \"existing-uuid\", \"extra\": true\n}\n")
	state := []byte("{\"interfaces\": {}}\n")
	for name, data := range map[string][]byte{"config.json": config, "machine_state.json": state} {
		if err := os.WriteFile(filepath.Join(opts.StateDir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := Check(context.Background(), opts)
	if err != nil || !r.Updated || r.Version != "0.3.0" {
		t.Fatalf("update failed: %+v %v", r, err)
	}
	for name, want := range map[string][]byte{"config.json": config, "machine_state.json": state, "../boops": binary, "../boops.previous": []byte("old executable")} {
		got, err := os.ReadFile(filepath.Join(opts.StateDir, name))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s changed: %q %v", name, got, err)
		}
	}
	info, _ := os.Stat(opts.ExecutablePath)
	if info.Mode().Perm() != 0751 {
		t.Fatalf("mode changed: %o", info.Mode().Perm())
	}
}

func TestCheckKeepsCurrentOnVerificationFailure(t *testing.T) {
	for _, fail := range []string{"signature", "hash", "size", "probe", "http", "redirect-host", "redirect-http", "previous-copy", "rename", "state-save"} {
		t.Run(fail, func(t *testing.T) {
			pub, key := fixtureKey(t)
			binary := []byte("new executable")
			env := releaseEnvelope(t, key, "0.3.0", binary)
			if fail == "signature" {
				_, wrong := fixtureKey(t)
				env = releaseEnvelope(t, wrong, "0.3.0", binary)
			}
			var requests atomic.Int32
			opts := fixtureOptions(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if strings.HasSuffix(r.URL.Path, "latest.json") {
					if fail == "http" {
						http.Error(w, "bad", 503)
						return
					}
					if fail == "redirect-host" {
						http.Redirect(w, r, "https://elsewhere.test/file", 302)
						return
					}
					if fail == "redirect-http" {
						http.Redirect(w, r, "http://file.booyah.dev/BoopsDB-Client/latest.json", 302)
						return
					}
					w.Write(env)
				} else {
					if fail == "hash" {
						w.Write([]byte("bad executable"))
					} else if fail == "size" {
						w.Write(append(binary, 'x'))
					} else {
						w.Write(binary)
					}
				}
			}, pub)
			os.MkdirAll(opts.StateDir, 0700)
			for _, name := range []string{"config.json", "machine_state.json"} {
				if err := os.WriteFile(filepath.Join(opts.StateDir, name), []byte("unchanged registration bytes\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if fail == "probe" {
				opts.Probe = func(context.Context, string, string) error { return errors.New("probe failure") }
			}
			if fail == "previous-copy" {
				if err := os.Mkdir(opts.ExecutablePath+".previous", 0700); err != nil {
					t.Fatal(err)
				}
			}
			if fail == "rename" {
				opts.Probe = func(_ context.Context, path, _ string) error { return os.Remove(path) }
			}
			if fail == "state-save" {
				if err := os.MkdirAll(filepath.Join(opts.StateDir, "update-state.json"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			r, err := Check(context.Background(), opts)
			if err == nil {
				t.Fatal("failure accepted")
			}
			want := []byte("old executable")
			if fail == "state-save" {
				want = binary
				if !r.Updated {
					t.Fatal("state-save failure misreported as unmodified")
				}
			} else if r.Updated {
				t.Fatal("failed update marked updated")
			}
			got, e := os.ReadFile(opts.ExecutablePath)
			if e != nil || !bytes.Equal(got, want) {
				t.Fatalf("current damaged: %q %v", got, e)
			}
			if fail == "signature" && requests.Load() != 1 {
				t.Fatal("artifact fetched before signature verification")
			}
			for _, name := range []string{"config.json", "machine_state.json"} {
				got, err := os.ReadFile(filepath.Join(opts.StateDir, name))
				if err != nil || string(got) != "unchanged registration bytes\n" {
					t.Fatalf("%s changed during %s failure", name, fail)
				}
			}
		})
	}
}

func TestCheckCadenceAndLock(t *testing.T) {
	pub, key := fixtureKey(t)
	env := releaseEnvelope(t, key, "0.2.0", []byte("new executable"))
	var requests atomic.Int32
	opts := fixtureOptions(t, func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.Write(env) }, pub)
	if _, err := Check(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	r, err := Check(context.Background(), opts)
	if err != nil || r.SkippedReason != "cadence" || requests.Load() != 1 {
		t.Fatalf("cadence: %+v %v requests=%d", r, err, requests.Load())
	}
	opts.Force = true
	if _, err := Check(context.Background(), opts); err != nil || requests.Load() != 2 {
		t.Fatalf("force: %v", err)
	}
	l, err := acquireLock(filepath.Join(opts.StateDir, "update.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	r, err = Check(context.Background(), opts)
	if err == nil || r.Updated {
		t.Fatalf("force lock not reported: %+v %v", r, err)
	}
	opts.Force = false
	r, err = Check(context.Background(), opts)
	if err != nil || r.SkippedReason != "locked" {
		t.Fatalf("periodic lock: %+v %v", r, err)
	}
}

func TestCheckMissingCorruptFutureStateChecksAgain(t *testing.T) {
	for _, state := range []string{"", `bad json`, `{"last_check":"2027-01-01T00:00:00Z"}`, `{"last_check":"2026-10-03T23:00:00Z"}`} {
		pub, key := fixtureKey(t)
		env := releaseEnvelope(t, key, "0.2.0", []byte("new executable"))
		var count atomic.Int32
		opts := fixtureOptions(t, func(w http.ResponseWriter, r *http.Request) { count.Add(1); w.Write(env) }, pub)
		if state != "" {
			os.MkdirAll(opts.StateDir, 0700)
			os.WriteFile(filepath.Join(opts.StateDir, "update-state.json"), []byte(state), 0600)
		}
		if _, err := Check(context.Background(), opts); err != nil || count.Load() != 1 {
			t.Fatalf("state %q: %v count=%d", state, err, count.Load())
		}
	}
}

func TestCheckRejectsSourceAndSkipsUnsupported(t *testing.T) {
	pub, key := fixtureKey(t)
	for _, base := range []string{"http://file.booyah.dev/BoopsDB-Client/", "https://attacker.test/BoopsDB-Client/", "https://file.booyah.dev/other/", "https://file.booyah.dev:444/BoopsDB-Client/", "https://user@file.booyah.dev/BoopsDB-Client/"} {
		opts := fixtureOptions(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid source requested") }, pub)
		opts.BaseURL = base
		if _, err := Check(context.Background(), opts); err == nil {
			t.Fatalf("bad source accepted %s", base)
		}
	}
	for _, kind := range []string{"same", "older", "windows", "arch", "dev"} {
		env := releaseEnvelope(t, key, "0.2.0", []byte("new executable"))
		var count atomic.Int32
		opts := fixtureOptions(t, func(w http.ResponseWriter, r *http.Request) { count.Add(1); w.Write(env) }, pub)
		switch kind {
		case "older":
			opts.CurrentVersion = "0.3.0"
		case "windows":
			opts.GOOS = "windows"
		case "arch":
			opts.GOARCH = "386"
		case "dev":
			opts.CurrentVersion = "dev"
		}
		r, err := Check(context.Background(), opts)
		if err != nil || r.Updated {
			t.Fatalf("%s %+v %v", kind, r, err)
		}
		if (kind == "windows" || kind == "arch" || kind == "dev") && count.Load() != 0 {
			t.Fatal("unsupported build made network request")
		}
	}
}

func TestCheckFailedCommunicationIsThrottled(t *testing.T) {
	pub, _ := fixtureKey(t)
	var count atomic.Int32
	opts := fixtureOptions(t, func(w http.ResponseWriter, r *http.Request) { count.Add(1); http.Error(w, "unavailable", 503) }, pub)
	if _, err := Check(context.Background(), opts); err == nil {
		t.Fatal("HTTP failure ignored")
	}
	r, err := Check(context.Background(), opts)
	if err != nil || r.SkippedReason != "cadence" || count.Load() != 1 {
		t.Fatalf("retry not throttled: %+v %v", r, err)
	}
}

func TestCheckManifestLimit(t *testing.T) {
	pub, _ := fixtureKey(t)
	opts := fixtureOptions(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(w, strings.NewReader(strings.Repeat(" ", 1048577)))
	}, pub)
	if _, err := Check(context.Background(), opts); err == nil {
		t.Fatal("oversized manifest accepted")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCheckRequestDeadlineLimits(t *testing.T) {
	pub, key := fixtureKey(t)
	binary := []byte("new executable")
	env := releaseEnvelope(t, key, "0.3.0", binary)
	opts := fixtureOptions(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "latest.json") {
			w.Write(env)
		} else {
			w.Write(binary)
		}
	}, pub)
	base := opts.HTTPClient.Transport
	opts.HTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("request has no deadline")
		}
		remaining := time.Until(deadline)
		limit := 120 * time.Second
		if strings.HasSuffix(r.URL.Path, "latest.json") {
			limit = 15 * time.Second
		}
		if remaining <= 0 || remaining > limit {
			t.Fatalf("request deadline %v exceeds %v", remaining, limit)
		}
		return base.RoundTrip(r)
	})
	if r, err := Check(context.Background(), opts); err != nil || !r.Updated {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestCheckContextCancellationKeepsCurrent(t *testing.T) {
	pub, key := fixtureKey(t)
	env := releaseEnvelope(t, key, "0.3.0", []byte("new executable"))
	opts := fixtureOptions(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "latest.json") {
			w.Write(env)
			return
		}
		<-r.Context().Done()
	}, pub)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if r, err := Check(ctx, opts); err == nil || r.Updated {
		t.Fatalf("cancellation ignored: %+v %v", r, err)
	}
	got, err := os.ReadFile(opts.ExecutablePath)
	if err != nil || string(got) != "old executable" {
		t.Fatalf("current changed: %s %v", got, err)
	}
}

func TestDefaultVersionProbe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture needs Unix")
	}
	for _, script := range []string{"#!/bin/sh\necho 0.3.0\n", "#!/bin/sh\necho 0.3.1\n", "#!/bin/sh\nexit 1\n"} {
		p := filepath.Join(t.TempDir(), "probe")
		if err := os.WriteFile(p, []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		err := probeVersion(context.Background(), p, "0.3.0")
		if strings.Contains(script, "echo 0.3.0") {
			if err != nil {
				t.Fatal(err)
			}
		} else if err == nil {
			t.Fatal("bad version probe accepted")
		}
	}
}

func TestDefaultVersionProbeRejectsInheritedPipes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture needs Unix")
	}
	path := filepath.Join(t.TempDir(), "probe")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nsleep 2 &\necho 0.3.0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := probeVersion(ctx, path, "0.3.0")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("pipe-holding descendant accepted: elapsed=%v context=%v", elapsed, ctx.Err())
	}
	if elapsed > time.Second {
		t.Fatalf("probe exceeded deadline allowance: elapsed=%v error=%v", elapsed, err)
	}
}

func TestCheckDefaultProbeRejectsInheritedPipesAndKeepsCurrent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture needs Unix")
	}
	marker := filepath.Join(t.TempDir(), "probe-started")
	script := []byte("#!/bin/sh\nsleep 2 &\nprintf started > '" + strings.ReplaceAll(marker, "'", "'\\''") + "'\necho 0.3.0\n")
	pub, key := fixtureKey(t)
	env := releaseEnvelope(t, key, "0.3.0", script)
	opts := fixtureOptions(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "latest.json") {
			w.Write(env)
		} else {
			w.Write(script)
		}
	}, pub)
	opts.Probe = nil
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	start := time.Now()
	result, err := Check(ctx, opts)
	elapsed := time.Since(start)
	if _, e := os.Stat(marker); e != nil {
		t.Fatalf("fixture never reached version probe: %v (check error %v)", e, err)
	}
	if err == nil || result.Updated {
		t.Fatalf("pipe-holding candidate installed: %+v %v (elapsed=%v)", result, err, elapsed)
	}
	if elapsed > time.Second {
		t.Fatalf("update exceeded probe deadline allowance: %v", elapsed)
	}
	got, e := os.ReadFile(opts.ExecutablePath)
	if e != nil || string(got) != "old executable" {
		t.Fatalf("current executable changed: %q %v", got, e)
	}
}

func TestCheckArm64SelectsOnlyItsSignedArtifact(t *testing.T) {
	pub, key := fixtureKey(t)
	binary := []byte("new executable")
	h := sha256.Sum256(binary)
	m := Manifest{Schema: 1, Version: "0.3.0", Artifacts: map[string]Artifact{
		"linux/amd64": {Path: "boops_0.3.0_amd64.binary", Size: 14, SHA256: strings.Repeat("a", 64)},
		"linux/arm64": {Path: "boops_0.3.0_arm64.binary", Size: 14, SHA256: hex.EncodeToString(h[:])},
	}}
	payload, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	env := signedBytes(t, payload, key)
	opts := fixtureOptions(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/BoopsDB-Client/latest.json":
			w.Write(env)
		case "/BoopsDB-Client/boops_0.3.0_arm64.binary":
			w.Write(binary)
		default:
			http.Error(w, "wrong architecture", 404)
		}
	}, pub)
	opts.GOARCH = "arm64"
	if r, err := Check(context.Background(), opts); err != nil || !r.Updated {
		t.Fatalf("arm64 update failed: %+v %v", r, err)
	}
}
