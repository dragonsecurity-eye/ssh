module ssh-remoteforward

go 1.26.1

replace eye.dragonsecurity.io/ssh => ../..

require eye.dragonsecurity.io/ssh v0.0.0-00010101000000-000000000000

require (
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
)
