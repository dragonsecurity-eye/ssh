package main

import (
	"log"
	"time"

	"eye.dragonsecurity.io/ssh"
)

var (
	DeadlineTimeout = 30 * time.Second
	IdleTimeout     = 10 * time.Second
)

func main() {
	log.Println("starting ssh server on port 2222...")
	log.Printf("connections will only last %s\n", DeadlineTimeout)
	log.Printf("and timeout after %s of no activity\n", IdleTimeout)

	server := &ssh.Server{
		Addr:        ":2222",
		MaxTimeout:  DeadlineTimeout,
		IdleTimeout: IdleTimeout,
		Handler: func(s ssh.Session) {
			log.Println("new connection")
			i := 0
			for {
				i++
				log.Println("active seconds:", i)
				select {
				case <-time.After(time.Second):
					continue
				case <-s.Context().Done():
					log.Println("connection closed")
					return
				}
			}
		},
		PasswordHandler: func(ctx ssh.Context, pass string) bool {
			return pass == "secret"
		},
	}
	log.Fatal(server.ListenAndServe())
}
