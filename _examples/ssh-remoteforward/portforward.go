package main

import (
	"io"
	"log"

	"eye.dragonsecurity.io/ssh"
)

func main() {
	log.Println("starting ssh server on port 2222...")

	forwardHandler := &ssh.ForwardedTCPHandler{}

	server := &ssh.Server{
		Addr: ":2222",
		Handler: func(s ssh.Session) {
			io.WriteString(s, "Remote forwarding available...\n")
			select {}
		},
		PasswordHandler: func(ctx ssh.Context, pass string) bool {
			return pass == "secret"
		},
		LocalPortForwardingCallback: func(ctx ssh.Context, dhost string, dport uint32) bool {
			log.Println("Accepted forward", dhost, dport)
			return true
		},
		ReversePortForwardingCallback: func(ctx ssh.Context, host string, port uint32) bool {
			log.Println("attempt to bind", host, port, "granted")
			return true
		},
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"session":      ssh.DefaultSessionHandler,
			"direct-tcpip": ssh.DirectTCPIPHandler,
		},
		RequestHandlers: map[string]ssh.RequestHandler{
			"tcpip-forward":        forwardHandler.HandleSSHRequest,
			"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
		},
	}

	log.Fatal(server.ListenAndServe())
}
