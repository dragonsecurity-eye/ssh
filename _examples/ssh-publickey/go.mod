module ssh-publickey

go 1.26.2

replace eye.dragonsecurity.io/ssh => ../..

require (
	eye.dragonsecurity.io/ssh v0.0.0-00010101000000-000000000000
	golang.org/x/crypto v0.50.0
)

require (
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be // indirect
	golang.org/x/sys v0.43.0 // indirect
)
