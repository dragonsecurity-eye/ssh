package main

import (
	"fmt"
	"io"
	"log"

	"eye.dragonsecurity.io/ssh"
	"github.com/pkg/sftp"
)

func sftpHandler(sess ssh.Session) {
	debugStream := io.Discard
	serverOptions := []sftp.ServerOption{
		sftp.WithDebug(debugStream),
	}
	server, err := sftp.NewServer(
		sess,
		serverOptions...,
	)
	if err != nil {
		log.Printf("sftp server init error: %s\n", err)
		return
	}
	if err := server.Serve(); err == io.EOF {
		server.Close()
		fmt.Println("sftp client exited session.")
	} else if err != nil {
		fmt.Println("sftp server completed with error:", err)
	}
}

func main() {
	server := &ssh.Server{
		Addr: "127.0.0.1:2222",
		PasswordHandler: func(ctx ssh.Context, pass string) bool {
			return pass == "secret"
		},
		SubsystemHandlers: map[string]ssh.SubsystemHandler{
			"sftp": sftpHandler,
		},
	}
	log.Fatal(server.ListenAndServe())
}
