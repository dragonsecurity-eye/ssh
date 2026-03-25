package main

import (
	"fmt"
	"io"
	"log"

	"eye.dragonsecurity.io/ssh"
)

func main() {
	ssh.Handle(func(s ssh.Session) {
		io.WriteString(s, fmt.Sprintf("Hello %s\n", s.User()))
	})

	log.Println("starting ssh server on port 2222...")
	log.Fatal(ssh.ListenAndServe(":2222", nil,
		ssh.PasswordAuth(func(ctx ssh.Context, pass string) bool {
			return pass == "secret"
		}),
	))
}
