package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, key
}

func signedBytes(t *testing.T, payload []byte, key ed25519.PrivateKey) []byte {
	t.Helper()
	b, err := json.Marshal(Envelope{Payload: base64.StdEncoding.EncodeToString(payload), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestVerifyManifestSignsRawDecodedPayload(t *testing.T) {
	pub, key := fixtureKey(t)
	payload := []byte(`{ "artifacts": {"linux/amd64":{"sha256":"` + strings.Repeat("a", 64) + `","size":4,"path":"boops_0.3.0_amd64.binary"}}, "version":"0.3.0", "schema":1 }`)
	data := signedBytes(t, payload, key)
	m, err := VerifyManifest(data, pub)
	if err != nil || m.Version != "0.3.0" {
		t.Fatalf("valid raw signature rejected: %v", err)
	}
	var e Envelope
	if err := json.Unmarshal(data, &e); err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	reserialized, _ := json.Marshal(decoded)
	e.Payload = base64.StdEncoding.EncodeToString(reserialized)
	changed, _ := json.Marshal(e)
	if _, err := VerifyManifest(changed, pub); err == nil {
		t.Fatal("signature survived reserialization")
	}
	e.Payload = base64.StdEncoding.EncodeToString([]byte(strings.Replace(string(payload), "0.3.0", "0.3.1", 1)))
	changed, _ = json.Marshal(e)
	if _, err := VerifyManifest(changed, pub); err == nil {
		t.Fatal("tampered payload accepted")
	}
	otherPub, _ := fixtureKey(t)
	if _, err := VerifyManifest(data, otherPub); err == nil {
		t.Fatal("untrusted key accepted")
	}
}

func TestManifestRejectsInvalidSignedValues(t *testing.T) {
	pub, key := fixtureKey(t)
	for _, path := range []string{"../escape", "/tmp/escape", "https://other.test/file", "nested/file", "a\\b", "x%2f..", "x?query", ".."} {
		t.Run(path, func(t *testing.T) {
			payload, _ := json.Marshal(map[string]any{"schema": 1, "version": "0.3.0", "artifacts": map[string]any{"linux/amd64": map[string]any{"path": path, "size": 4, "sha256": strings.Repeat("a", 64)}}})
			if _, err := VerifyManifest(signedBytes(t, payload, key), pub); err == nil {
				t.Fatal("unsafe path accepted")
			}
		})
	}
	for _, payload := range []string{
		`{"schema":2,"version":"0.3.0","artifacts":{}}`,
		`{"schema":1,"version":"dev","artifacts":{}}`,
		`{"schema":1,"version":"0.3.0","artifacts":{"linux/amd64":{"path":"a","size":67108865,"sha256":"` + strings.Repeat("a", 64) + `"}}}`,
		`{"schema":1,"version":"0.3.0","artifacts":{"linux/amd64":{"path":"a","size":0,"sha256":"` + strings.Repeat("a", 64) + `"}}}`,
		`{"schema":1,"version":"0.3.0","artifacts":{"linux/amd64":{"path":"a","size":4,"sha256":"broken"}}}`,
		`{"schema":1,"version":"0.3.0","artifacts":{"windows/amd64":{"path":"a","size":4,"sha256":"` + strings.Repeat("a", 64) + `"}}}`,
	} {
		if _, err := VerifyManifest(signedBytes(t, []byte(payload), key), pub); err == nil {
			t.Fatalf("bad manifest accepted: %s", payload)
		}
	}
	if _, err := VerifyManifest([]byte(strings.Repeat("x", 1048577)), pub); err == nil {
		t.Fatal("oversized envelope accepted")
	}
}

func TestCompareVersions(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{{"0.3.9", "0.3.10", -1}, {"1.0.0", "0.99.99", 1}, {"0.3.0", "0.3.0", 0}} {
		got, err := CompareVersions(c.a, c.b)
		if err != nil || got != c.want {
			t.Fatalf("%s vs %s: %d %v", c.a, c.b, got, err)
		}
	}
	for _, version := range []string{"dev", "v0.3.0", "0.3", "0.3.0-rc1", "00.3.0", "0.3.-1", "0.3.0 ", "99999999999999999999999.3.0"} {
		if _, err := CompareVersions(version, "0.3.0"); err == nil {
			t.Fatalf("invalid version %q accepted", version)
		}
	}
}

func TestDescribeArtifactComputesBytesAndRejectsOversize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "client.binary")
	if err := os.WriteFile(path, []byte("abc"), 0600); err != nil {
		t.Fatal(err)
	}
	a, err := DescribeArtifact(path)
	if err != nil || a.Path != "client.binary" || a.Size != 3 || a.SHA256 != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("wrong size/hash: %+v %v", a, err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(64*1024*1024 + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := DescribeArtifact(path); err == nil {
		t.Fatal("oversized file accepted")
	}
}

func TestGoManifestVerifiesWithOpenSSL(t *testing.T) {
	openssl, err := exec.LookPath("openssl")
	if err != nil {
		t.Skip("OpenSSL is unavailable")
	}
	pub, key := fixtureKey(t)
	m := Manifest{Schema: 1, Version: "0.3.0", Artifacts: map[string]Artifact{"linux/amd64": {Path: "client.binary", Size: 3, SHA256: "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"}}}
	data, err := SignManifest(m, key)
	if err != nil {
		t.Fatal(err)
	}
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatal(err)
	}
	payload, _ := base64.StdEncoding.DecodeString(env.Payload)
	sig, _ := base64.StdEncoding.DecodeString(env.Signature)
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, value := range map[string][]byte{"public-key.pem": pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), "payload.json": payload, "signature.bin": sig} {
		if err := os.WriteFile(filepath.Join(dir, name), value, 0600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"pkeyutl", "-verify", "-rawin", "-pubin", "-inkey", filepath.Join(dir, "public-key.pem"), "-in", filepath.Join(dir, "payload.json"), "-sigfile", filepath.Join(dir, "signature.bin")}
	if output, err := exec.Command(openssl, args...).CombinedOutput(); err != nil {
		t.Fatalf("OpenSSL rejected Go raw payload signature: %v %s", err, output)
	}
	payload[0] = '['
	if err := os.WriteFile(filepath.Join(dir, "payload.json"), payload, 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(openssl, args...).CombinedOutput(); err == nil {
		t.Fatalf("OpenSSL accepted tampered payload: %s", output)
	}
}
