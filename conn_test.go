package ssh

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestServerConnIdleTimeout(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sc := &serverConn{
		Conn:          server,
		idleTimeout:   50 * time.Millisecond,
		closeCanceler: cancel,
	}

	// Write should succeed initially
	go func() {
		buf := make([]byte, 64)
		for {
			_, err := client.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	_, err := sc.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("initial write failed: %v", err)
	}

	// After idle timeout, reads should fail
	time.Sleep(100 * time.Millisecond)

	buf := make([]byte, 64)
	_, err = sc.Read(buf)
	if err == nil {
		t.Fatal("expected read to fail after idle timeout")
	}

	// Context should be cancelled
	select {
	case <-ctx.Done():
	default:
		t.Fatal("expected context to be cancelled after net error")
	}
}

func TestServerConnMaxDeadline(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	sc := &serverConn{
		Conn:          server,
		maxDeadline:   time.Now().Add(50 * time.Millisecond),
		idleTimeout:   10 * time.Second, // long idle, short max
		closeCanceler: cancel,
	}

	// Trigger deadline update
	go func() {
		buf := make([]byte, 64)
		for {
			_, err := client.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	sc.Write([]byte("hi"))

	// Wait for max deadline to pass
	time.Sleep(100 * time.Millisecond)

	buf := make([]byte, 64)
	_, err := sc.Read(buf)
	if err == nil {
		t.Fatal("expected read to fail after max deadline")
	}
}

func TestServerConnHandshakeDeadline(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sc := &serverConn{
		Conn:              server,
		handshakeDeadline: time.Now().Add(50 * time.Millisecond),
		idleTimeout:       10 * time.Second,
		closeCanceler:     cancel,
	}

	go func() {
		buf := make([]byte, 64)
		for {
			_, err := client.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	sc.Write([]byte("hi"))

	time.Sleep(100 * time.Millisecond)

	buf := make([]byte, 64)
	_, err := sc.Read(buf)
	if err == nil {
		t.Fatal("expected read to fail after handshake deadline")
	}

	select {
	case <-ctx.Done():
	default:
		t.Fatal("expected context to be cancelled")
	}
}

func TestServerConnClose(t *testing.T) {
	_, server := net.Pipe()
	cancelled := false
	sc := &serverConn{
		Conn:          server,
		closeCanceler: func() { cancelled = true },
	}

	err := sc.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !cancelled {
		t.Fatal("expected closeCanceler to be called")
	}
}

func TestServerConnNoIdleTimeout(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	sc := &serverConn{
		Conn: server,
		// No idle timeout set
	}

	go func() {
		buf := make([]byte, 64)
		for {
			if _, err := client.Read(buf); err != nil {
				return
			}
		}
	}()

	// Write should succeed without updating deadline
	_, err := sc.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	sc.Close()
}
