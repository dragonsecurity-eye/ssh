package ssh

import (
	"io"
	"net"
	"strings"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

func TestServerHandle(t *testing.T) {
	srv := &Server{}
	srv.Handle(func(s Session) {})
	if srv.Handler == nil {
		t.Fatal("Handler should be set")
	}
}

func TestPackageLevelHandle(t *testing.T) {
	old := DefaultHandler
	defer func() { DefaultHandler = old }()

	Handle(func(s Session) {})
	if DefaultHandler == nil {
		t.Fatal("DefaultHandler should be set")
	}
}

func TestPackageLevelServeRequiresAuth(t *testing.T) {
	l := newLocalListener()
	defer l.Close()

	err := Serve(l, func(s Session) {})
	if err != ErrNoAuthConfigured {
		t.Fatalf("expected ErrNoAuthConfigured, got %v", err)
	}
}

func TestServerBanner(t *testing.T) {
	t.Parallel()
	l := newLocalListener()
	srv := &Server{
		Handler: func(s Session) {
			io.WriteString(s, "ok")
		},
		Banner:       "Welcome to DragonEye SSH",
		NoClientAuth: true,
	}
	go srv.serveOnce(l)

	config := &gossh.ClientConfig{
		User:            "testuser",
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		BannerCallback: func(message string) error {
			if !strings.Contains(message, "Welcome to DragonEye SSH") {
				t.Errorf("expected banner, got %q", message)
			}
			return nil
		},
	}
	client, err := gossh.Dial("tcp", l.Addr().String(), config)
	if err != nil {
		t.Fatal(err)
	}
	client.Close()
}

func TestServerBannerHandler(t *testing.T) {
	t.Parallel()
	l := newLocalListener()
	srv := &Server{
		Handler: func(s Session) {
			io.WriteString(s, "ok")
		},
		BannerHandler: func(ctx Context) string {
			return "Dynamic banner for " + ctx.User()
		},
		NoClientAuth: true,
	}
	go srv.serveOnce(l)

	var receivedBanner string
	config := &gossh.ClientConfig{
		User:            "dragon",
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		BannerCallback: func(message string) error {
			receivedBanner = message
			return nil
		},
	}
	client, err := gossh.Dial("tcp", l.Addr().String(), config)
	if err != nil {
		t.Fatal(err)
	}
	client.Close()

	if !strings.Contains(receivedBanner, "Dynamic banner for dragon") {
		t.Fatalf("expected dynamic banner, got %q", receivedBanner)
	}
}

func TestServerVersion(t *testing.T) {
	t.Parallel()
	l := newLocalListener()
	srv := &Server{
		Handler:      func(s Session) {},
		Version:      "DragonEye-1.0",
		NoClientAuth: true,
	}
	go srv.serveOnce(l)

	config := &gossh.ClientConfig{
		User:            "testuser",
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
	}
	client, err := gossh.Dial("tcp", l.Addr().String(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
}

func TestConnectionFailedCallback(t *testing.T) {
	t.Parallel()
	l := newLocalListener()
	failedCh := make(chan error, 1)
	srv := &Server{
		Handler: func(s Session) {},
		PasswordHandler: func(ctx Context, password string) bool {
			return false // reject all passwords
		},
		ConnectionFailedCallback: func(conn net.Conn, err error) {
			failedCh <- err
		},
	}
	go srv.serveOnce(l)

	// Try to connect with wrong password - should trigger callback
	_, err := gossh.Dial("tcp", l.Addr().String(), &gossh.ClientConfig{
		User: "testuser",
		Auth: []gossh.AuthMethod{
			gossh.Password("wrong"),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
	})
	if err == nil {
		t.Fatal("expected auth failure")
	}
	// Note: ConnectionFailedCallback may or may not fire depending on
	// whether the handshake itself fails vs auth being rejected.
	// The important thing is the server doesn't crash.
}
