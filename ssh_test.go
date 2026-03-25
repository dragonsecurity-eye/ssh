package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

func TestKeysEqual(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("The code did panic")
		}
	}()

	if KeysEqual(nil, nil) {
		t.Error("two nil keys should not return true")
	}
}

func TestKeysEqualWithRealKeys(t *testing.T) {
	_, priv1, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, priv2, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer1, err := gossh.NewSignerFromKey(priv1)
	if err != nil {
		t.Fatal(err)
	}
	signer2, err := gossh.NewSignerFromKey(priv2)
	if err != nil {
		t.Fatal(err)
	}

	pub1 := signer1.PublicKey()
	pub2 := signer2.PublicKey()

	if !KeysEqual(pub1, pub1) {
		t.Error("same key should be equal")
	}
	if KeysEqual(pub1, pub2) {
		t.Error("different keys should not be equal")
	}
	if KeysEqual(pub1, nil) {
		t.Error("key and nil should not be equal")
	}
	if KeysEqual(nil, pub1) {
		t.Error("nil and key should not be equal")
	}
}

func TestKeysEqualParsedKey(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	pub := signer.PublicKey()

	// Parse the key back from wire format
	parsed, err := ParsePublicKey(pub.Marshal())
	if err != nil {
		t.Fatal(err)
	}
	if !KeysEqual(pub, parsed) {
		t.Error("parsed key should equal original")
	}
}
