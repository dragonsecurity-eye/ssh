package ssh

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestAddHostKey(t *testing.T) {
	s := Server{}
	signer, err := generateSigner()
	if err != nil {
		t.Fatal(err)
	}
	s.AddHostKey(signer)
	if len(s.HostSigners) != 1 {
		t.Fatal("Key was not properly added")
	}
	signer, err = generateSigner()
	if err != nil {
		t.Fatal(err)
	}
	s.AddHostKey(signer)
	if len(s.HostSigners) != 1 {
		t.Fatal("Key was not properly replaced")
	}
}

func TestServerShutdown(t *testing.T) {
	l := newLocalListener()
	testBytes := []byte("Hello world\n")
	s := &Server{
		Handler: func(s Session) {
			s.Write(testBytes)
			time.Sleep(50 * time.Millisecond)
		},
		NoClientAuth: true,
	}
	var serveErr error
	var serveWg sync.WaitGroup
	serveWg.Add(1)
	go func() {
		defer serveWg.Done()
		serveErr = s.Serve(l)
	}()

	sessDone := make(chan struct{})
	sess, _, cleanup := newClientSession(t, l.Addr().String(), nil)
	var sessErr error
	var stdoutBuf bytes.Buffer
	go func() {
		defer cleanup()
		defer close(sessDone)
		sess.Stdout = &stdoutBuf
		sessErr = sess.Run("")
	}()

	srvDone := make(chan struct{})
	var shutdownErr error
	go func() {
		defer close(srvDone)
		shutdownErr = s.Shutdown(context.Background())
	}()

	timeout := time.After(2 * time.Second)
	select {
	case <-timeout:
		t.Fatal("timeout")
		return
	case <-srvDone:
		<-sessDone
		if shutdownErr != nil {
			t.Fatalf("shutdown error: %v", shutdownErr)
		}
		if sessErr != nil {
			t.Fatalf("session error: %v", sessErr)
		}
		if !bytes.Equal(stdoutBuf.Bytes(), testBytes) {
			t.Fatalf("expected = %s; got %s", testBytes, stdoutBuf.Bytes())
		}
		serveWg.Wait()
		if serveErr != nil && serveErr != ErrServerClosed {
			t.Fatalf("serve error: %v", serveErr)
		}
		return
	}
}

func TestServerClose(t *testing.T) {
	l := newLocalListener()
	s := &Server{
		Handler: func(s Session) {
			time.Sleep(5 * time.Second)
		},
		NoClientAuth: true,
	}
	var serveErr error
	var serveWg sync.WaitGroup
	serveWg.Add(1)
	go func() {
		defer serveWg.Done()
		serveErr = s.Serve(l)
	}()

	closeDoneChan := make(chan struct{})

	sess, _, cleanup := newClientSession(t, l.Addr().String(), nil)
	clientDoneChan := make(chan error, 1)
	go func() {
		defer cleanup()
		<-closeDoneChan
		clientDoneChan <- sess.Run("")
	}()

	var closeErr error
	go func() {
		closeErr = s.Close()
		close(closeDoneChan)
	}()

	timeout := time.After(100 * time.Millisecond)
	select {
	case <-timeout:
		t.Error("timeout")
		return
	case <-s.getDoneChan():
		sessErr := <-clientDoneChan
		if sessErr != nil && sessErr != io.EOF {
			t.Fatalf("client error: %v", sessErr)
		}
		if closeErr != nil {
			t.Fatalf("close error: %v", closeErr)
		}
		serveWg.Wait()
		if serveErr != nil && serveErr != ErrServerClosed {
			t.Fatalf("serve error: %v", serveErr)
		}
		return
	}
}

func TestServerHandshakeTimeout(t *testing.T) {
	l := newLocalListener()

	s := &Server{
		HandshakeTimeout: time.Millisecond,
		NoClientAuth:     true,
	}
	go func() {
		if err := s.Serve(l); err != nil && err != ErrServerClosed {
			// can't use t.Error from goroutine, but this is a background server
		}
	}()

	conn, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		io.Copy(io.Discard, conn)
	}()

	select {
	case <-ch:
		return
	case <-time.After(time.Second):
		t.Fatal("client connection was not force-closed")
		return
	}
}

func TestNoAuthConfiguredError(t *testing.T) {
	l := newLocalListener()
	s := &Server{
		Handler: func(s Session) {},
	}
	err := s.Serve(l)
	if err != ErrNoAuthConfigured {
		t.Fatalf("expected ErrNoAuthConfigured but got %v", err)
	}
}
