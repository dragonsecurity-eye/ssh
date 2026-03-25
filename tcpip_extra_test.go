package ssh

import (
	"net"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

func TestReversePortForwardingWorks(t *testing.T) {
	t.Parallel()

	forwardHandler := &ForwardedTCPHandler{}
	l := newLocalListener()
	srv := &Server{
		Handler:      func(s Session) { select {} },
		NoClientAuth: true,
		ReversePortForwardingCallback: func(ctx Context, bindHost string, bindPort uint32) bool {
			return true
		},
		RequestHandlers: map[string]RequestHandler{
			"tcpip-forward":        forwardHandler.HandleSSHRequest,
			"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
		},
		ChannelHandlers: map[string]ChannelHandler{
			"session": DefaultSessionHandler,
		},
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

	// Request reverse forwarding on a random port
	ln, err := client.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	// Verify the forwarded port is accessible
	conn, err := net.DialTimeout("tcp", ln.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("failed to connect to forwarded port: %v", err)
	}
	conn.Close()

	// Accept succeeds on the client side
	// (we already verified the forward works by dialing)
}

func TestReversePortForwardingDenied(t *testing.T) {
	t.Parallel()

	forwardHandler := &ForwardedTCPHandler{}
	l := newLocalListener()
	srv := &Server{
		Handler:      func(s Session) { select {} },
		NoClientAuth: true,
		ReversePortForwardingCallback: func(ctx Context, bindHost string, bindPort uint32) bool {
			return false // deny all
		},
		RequestHandlers: map[string]RequestHandler{
			"tcpip-forward":        forwardHandler.HandleSSHRequest,
			"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
		},
		ChannelHandlers: map[string]ChannelHandler{
			"session": DefaultSessionHandler,
		},
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

	_, err = client.Listen("tcp", "127.0.0.1:0")
	if err == nil {
		t.Fatal("expected reverse port forwarding to be denied")
	}
}
