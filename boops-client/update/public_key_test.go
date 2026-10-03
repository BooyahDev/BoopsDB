package update

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"
)

func TestTrustedKeyMatchesPublishedPEM(t *testing.T) {
	pub, err := TrustedPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("public-key.pem")
	if err != nil {
		t.Fatal(err)
	}
	block, rest := pem.Decode(data)
	if block == nil || len(bytes.TrimSpace(rest)) != 0 {
		t.Fatal("bad PEM")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	ed, ok := key.(ed25519.PublicKey)
	if !ok || !bytes.Equal(pub, ed) {
		t.Fatal("embedded and distributed keys differ")
	}
}
