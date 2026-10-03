package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"boops/update"
)

func releaseFixture(t *testing.T) (ed25519.PublicKey, string) {
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(p, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	return pub, p
}

func TestReleaseSignsBothArtifacts(t *testing.T) {
	pub, keyPath := releaseFixture(t)
	dir := filepath.Join(t.TempDir(), "release")
	builder := func(_ context.Context, arch, version, dst string) error {
		return os.WriteFile(dst, []byte("fixture "+arch+" "+version), 0755)
	}
	if err := createRelease(context.Background(), "0.3.0", keyPath, dir, pub, builder); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := update.VerifyManifest(data, pub)
	if err != nil || m.Version != "0.3.0" || len(m.Artifacts) != 2 {
		t.Fatalf("bad release: %+v %v", m, err)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		a := m.Artifacts["linux/"+arch]
		if a.Path != "boops_0.3.0_"+arch+".binary" || a.Size != 19 {
			t.Fatalf("bad artifact metadata %+v", a)
		}
	}
	before, _ := os.ReadFile(filepath.Join(dir, "boops_0.3.0_amd64.binary"))
	if err := createRelease(context.Background(), "0.3.0", keyPath, dir, pub, builder); err == nil {
		t.Fatal("existing release overwritten")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "boops_0.3.0_amd64.binary"))
	if !bytes.Equal(before, after) {
		t.Fatal("existing binary changed")
	}
}

func TestReleaseRejectsWrongKeyBeforeBuilding(t *testing.T) {
	pub, _ := releaseFixture(t)
	_, path := releaseFixture(t)
	err := createRelease(context.Background(), "0.3.0", path, t.TempDir(), pub, func(context.Context, string, string, string) error {
		t.Fatal("untrusted key built artifacts")
		return nil
	})
	if err == nil {
		t.Fatal("untrusted signing key accepted")
	}
}

func TestReleaseBuildFailureDoesNotExposePartialOutput(t *testing.T) {
	pub, keyPath := releaseFixture(t)
	parent := t.TempDir()
	out := filepath.Join(parent, "release")
	err := createRelease(context.Background(), "0.3.0", keyPath, out, pub, func(_ context.Context, arch, version, dst string) error {
		if arch == "arm64" {
			return errors.New("fixture build failure")
		}
		return os.WriteFile(dst, []byte("binary"), 0755)
	})
	if err == nil {
		t.Fatal("build failure accepted")
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("partial output retained: %v %v", entries, err)
	}
}

func TestReleaseRejectsInsecureKeyPermissions(t *testing.T) {
	_, path := releaseFixture(t)
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readKey(path); err == nil {
		t.Fatal("world-readable signing key accepted")
	}
}
