package ssh

import (
	"net"
	"os"
	"strings"
	"testing"
)

func TestNewAgentListener(t *testing.T) {
	l, err := NewAgentListener()
	if err != nil {
		t.Fatal(err)
	}

	addr := l.Addr().String()
	if !strings.Contains(addr, "listener.sock") {
		t.Fatalf("expected listener.sock in address, got %s", addr)
	}

	// Verify the socket file exists
	if _, err := os.Stat(addr); err != nil {
		t.Fatalf("socket file should exist: %v", err)
	}

	// Verify we can connect
	conn, err := net.Dial("unix", addr)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()

	// Close should clean up the temp directory
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	// Verify cleanup happened
	if _, err := os.Stat(addr); !os.IsNotExist(err) {
		t.Fatal("socket file should be cleaned up after Close")
	}
}

func TestAgentListenerType(t *testing.T) {
	l, err := NewAgentListener()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	al, ok := l.(*agentListener)
	if !ok {
		t.Fatal("expected *agentListener type")
	}
	if al.dir == "" {
		t.Fatal("expected non-empty dir")
	}
}
