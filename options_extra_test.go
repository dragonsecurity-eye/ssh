package ssh

import (
	"os"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

func TestNoPtyOption(t *testing.T) {
	srv := &Server{}
	if err := srv.SetOption(NoPty()); err != nil {
		t.Fatal(err)
	}
	if srv.PtyCallback == nil {
		t.Fatal("PtyCallback should be set")
	}
	// PtyCallback should deny all
	ctx, cancel := newContext(nil)
	defer cancel()
	if srv.PtyCallback(ctx, Pty{}) {
		t.Fatal("NoPty should deny PTY requests")
	}
}

func TestKeyboardInteractiveAuthOption(t *testing.T) {
	srv := &Server{}
	handler := func(ctx Context, challenger gossh.KeyboardInteractiveChallenge) bool {
		return true
	}
	if err := srv.SetOption(KeyboardInteractiveAuth(handler)); err != nil {
		t.Fatal(err)
	}
	if srv.KeyboardInteractiveHandler == nil {
		t.Fatal("KeyboardInteractiveHandler should be set")
	}
}

func TestHostKeyPEMOption(t *testing.T) {
	srv := &Server{}
	if err := srv.SetOption(HostKeyPEM([]byte("not a valid pem"))); err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

func TestHostKeyFileOption(t *testing.T) {
	// Test with non-existent file
	srv := &Server{}
	if err := srv.SetOption(HostKeyFile("/nonexistent/path")); err == nil {
		t.Fatal("expected error for non-existent file")
	}

	// Test with invalid content
	tmp, err := os.CreateTemp("", "hostkey-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString("not a valid pem")
	tmp.Close()

	if err := srv.SetOption(HostKeyFile(tmp.Name())); err == nil {
		t.Fatal("expected error for invalid PEM file")
	}
}

func TestParseAuthorizedKey(t *testing.T) {
	// Generate a key and format it as authorized_keys
	signer, err := generateSigner()
	if err != nil {
		t.Fatal(err)
	}
	pubBytes := gossh.MarshalAuthorizedKey(signer.PublicKey())

	key, _, _, _, err := ParseAuthorizedKey(pubBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !KeysEqual(key, signer.PublicKey()) {
		t.Fatal("parsed key should match original")
	}
}
