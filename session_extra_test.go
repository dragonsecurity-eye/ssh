package ssh

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

func TestCommandAndRawCommand(t *testing.T) {
	t.Parallel()
	session, _, cleanup := newTestSession(t, &Server{
		Handler: func(s Session) {
			fmt.Fprintf(s, "raw:%s\n", s.RawCommand())
			fmt.Fprintf(s, "cmd:%s\n", strings.Join(s.Command(), "|"))
		},
	}, nil)
	defer cleanup()
	var stdout bytes.Buffer
	session.Stdout = &stdout
	if err := session.Run("echo hello world"); err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	if !strings.Contains(output, "raw:echo hello world") {
		t.Fatalf("expected raw command in output, got %q", output)
	}
	if !strings.Contains(output, "cmd:echo|hello|world") {
		t.Fatalf("expected parsed command in output, got %q", output)
	}
}

func TestEnviron(t *testing.T) {
	t.Parallel()
	ready := make(chan struct{})
	session, _, cleanup := newTestSession(t, &Server{
		Handler: func(s Session) {
			<-ready
			env := s.Environ()
			for _, e := range env {
				fmt.Fprintln(s, e)
			}
		},
	}, nil)
	defer cleanup()
	var stdout bytes.Buffer
	session.Stdout = &stdout

	if err := session.Setenv("FOO", "bar"); err != nil {
		t.Fatal(err)
	}
	if err := session.Setenv("BAZ", "qux"); err != nil {
		t.Fatal(err)
	}
	close(ready)
	if err := session.Run(""); err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	if !strings.Contains(output, "FOO=bar") {
		t.Fatalf("expected FOO=bar in output, got %q", output)
	}
	if !strings.Contains(output, "BAZ=qux") {
		t.Fatalf("expected BAZ=qux in output, got %q", output)
	}
}

func TestDoubleExit(t *testing.T) {
	t.Parallel()
	errCh := make(chan error, 1)
	session, _, cleanup := newTestSession(t, &Server{
		Handler: func(s Session) {
			err1 := s.Exit(0)
			err2 := s.Exit(0)
			if err1 != nil {
				errCh <- fmt.Errorf("first Exit should succeed: %v", err1)
				return
			}
			if err2 == nil {
				errCh <- fmt.Errorf("second Exit should return error")
				return
			}
			errCh <- nil
		},
	}, nil)
	defer cleanup()
	session.Run("")

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for handler")
	}
}

func TestSubsystemHandler(t *testing.T) {
	t.Parallel()
	resultCh := make(chan string, 1)
	session, _, cleanup := newTestSession(t, &Server{
		Handler: func(s Session) {},
		SubsystemHandlers: map[string]SubsystemHandler{
			"test-subsystem": func(s Session) {
				resultCh <- s.Subsystem()
				io.WriteString(s, "subsystem-ok")
			},
		},
	}, nil)
	defer cleanup()
	var stdout bytes.Buffer
	session.Stdout = &stdout
	if err := session.RequestSubsystem("test-subsystem"); err != nil {
		t.Fatal(err)
	}
	session.Wait()

	select {
	case name := <-resultCh:
		if name != "test-subsystem" {
			t.Fatalf("expected subsystem name 'test-subsystem', got %q", name)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for subsystem handler")
	}
}

func TestPublicKeyAuth(t *testing.T) {
	t.Parallel()
	signer, err := generateSigner()
	if err != nil {
		t.Fatal(err)
	}
	session, _, cleanup := newTestSessionWithOptions(t, &Server{
		Handler: func(s Session) {
			if s.PublicKey() == nil {
				t.Fatal("expected public key to be set")
			}
			io.WriteString(s, "authenticated")
		},
	}, &gossh.ClientConfig{
		User: "testuser",
		Auth: []gossh.AuthMethod{
			gossh.PublicKeys(signer),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
	}, PublicKeyAuth(func(ctx Context, key PublicKey) bool {
		return KeysEqual(key, signer.PublicKey())
	}))
	defer cleanup()
	var stdout bytes.Buffer
	session.Stdout = &stdout
	if err := session.Run(""); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "authenticated" {
		t.Fatalf("expected 'authenticated', got %q", stdout.String())
	}
}

func TestPublicKeyAuthRejected(t *testing.T) {
	t.Parallel()
	signer, err := generateSigner()
	if err != nil {
		t.Fatal(err)
	}
	l := newLocalListener()
	srv := &Server{
		Handler: func(s Session) {},
	}
	srv.SetOption(PublicKeyAuth(func(ctx Context, key PublicKey) bool {
		return false // reject all
	}))
	go srv.serveOnce(l)

	_, err = gossh.Dial("tcp", l.Addr().String(), &gossh.ClientConfig{
		User: "testuser",
		Auth: []gossh.AuthMethod{
			gossh.PublicKeys(signer),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
	})
	if err == nil {
		t.Fatal("expected authentication failure")
	}
	if !strings.Contains(err.Error(), "unable to authenticate") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSessionRequestCallback(t *testing.T) {
	t.Parallel()
	session, _, cleanup := newTestSession(t, &Server{
		Handler: func(s Session) {
			io.WriteString(s, "should not reach here")
		},
		SessionRequestCallback: func(sess Session, requestType string) bool {
			return false // deny all sessions
		},
	}, nil)
	defer cleanup()
	err := session.Run("")
	if err == nil {
		t.Fatal("expected error from denied session")
	}
}

func TestIdleTimeout(t *testing.T) {
	t.Parallel()
	l := newLocalListener()
	srv := &Server{
		Handler: func(s Session) {
			time.Sleep(500 * time.Millisecond)
		},
		IdleTimeout:  50 * time.Millisecond,
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

	start := time.Now()
	session.Run("")
	elapsed := time.Since(start)

	// Should be cut short by the idle timeout, not the full 500ms sleep
	if elapsed > 400*time.Millisecond {
		t.Fatalf("expected connection to be cut short by idle timeout, took %v", elapsed)
	}
}

func TestMaxTimeout(t *testing.T) {
	t.Parallel()
	l := newLocalListener()
	srv := &Server{
		Handler: func(s Session) {
			time.Sleep(500 * time.Millisecond)
		},
		MaxTimeout:   100 * time.Millisecond,
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

	start := time.Now()
	session.Run("")
	elapsed := time.Since(start)

	if elapsed > 400*time.Millisecond {
		t.Fatalf("expected connection to be cut short by max timeout, took %v", elapsed)
	}
}

func TestAgentForwardingDeniedByDefault(t *testing.T) {
	t.Parallel()
	session, _, cleanup := newTestSession(t, &Server{
		Handler: func(s Session) {
			if AgentRequested(s) {
				t.Fatal("agent forwarding should not be requested when callback is nil")
			}
		},
	}, nil)
	defer cleanup()
	// Try to request agent forwarding
	ok, err := session.SendRequest("auth-agent-req@openssh.com", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected agent forwarding to be denied")
	}
	session.Run("")
}

func TestSessionRemoteAndLocalAddr(t *testing.T) {
	t.Parallel()
	session, _, cleanup := newTestSession(t, &Server{
		Handler: func(s Session) {
			if s.RemoteAddr() == nil {
				t.Error("RemoteAddr should not be nil")
			}
			if s.LocalAddr() == nil {
				t.Error("LocalAddr should not be nil")
			}
			fmt.Fprintf(s, "remote:%s", s.RemoteAddr().String())
		},
	}, nil)
	defer cleanup()
	var stdout bytes.Buffer
	session.Stdout = &stdout
	if err := session.Run(""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "remote:127.0.0.1:") {
		t.Fatalf("expected remote addr, got %q", stdout.String())
	}
}

func TestPtyWriteNormalization(t *testing.T) {
	t.Parallel()
	session, _, cleanup := newTestSession(t, &Server{
		Handler: func(s Session) {
			_, _, isPty := s.Pty()
			if !isPty {
				t.Fatal("expected PTY")
			}
			// Write with bare \n which should get normalized to \r\n
			s.Write([]byte("line1\nline2\n"))
		},
	}, nil)
	defer cleanup()
	if err := session.RequestPty("xterm", 80, 40, gossh.TerminalModes{}); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	session.Stdout = &stdout
	if err := session.Run(""); err != nil {
		t.Fatal(err)
	}
	got := stdout.Bytes()
	if !bytes.Contains(got, []byte("\r\n")) {
		t.Fatalf("expected \\r\\n normalization in PTY output, got %q", got)
	}
	// Should not have \r\r\n (double normalization)
	if bytes.Contains(got, []byte("\r\r\n")) {
		t.Fatalf("unexpected double \\r\\r\\n in output, got %q", got)
	}
}

func TestAgentForwardingWithCallback(t *testing.T) {
	t.Parallel()
	session, _, cleanup := newTestSession(t, &Server{
		Handler: func(s Session) {
			if !AgentRequested(s) {
				t.Fatal("expected agent forwarding to be requested")
			}
			io.WriteString(s, "agent-ok")
		},
		AgentForwardingCallback: func(ctx Context) bool {
			return true
		},
	}, nil)
	defer cleanup()
	var stdout bytes.Buffer
	session.Stdout = &stdout

	ok, err := session.SendRequest("auth-agent-req@openssh.com", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected agent forwarding to be accepted")
	}
	if err := session.Run(""); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "agent-ok" {
		t.Fatalf("expected 'agent-ok', got %q", stdout.String())
	}
}
