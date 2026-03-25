# Contributing to DragonEye SSH

## Development Setup

```bash
git clone <repo-url>
cd ssh
go mod download
```

## Running Tests

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -cover ./...

# Run a specific test
go test -run TestName ./...
```

## Code Standards

- Run `go vet ./...` before committing
- Run `go test -race ./...` to check for data races
- Follow standard Go conventions (`gofmt`, `goimports`)
- Add tests for new functionality
- Use the existing test helpers (`newTestSession`, `newTestSessionWithOptions`, `serveOnce`) for integration tests

## Project Structure

```
ssh.go          - Public API types, package-level functions (Serve, ListenAndServe, Handle, KeysEqual)
server.go       - Server struct, connection handling, lifecycle management
session.go      - Session interface and implementation, SSH request handling
context.go      - Connection-scoped context with SSH metadata
conn.go         - Connection wrapper with timeout/deadline management
tcpip.go        - Local and reverse port forwarding handlers
agent.go        - SSH agent forwarding support
options.go      - Functional options (PasswordAuth, PublicKeyAuth, HostKeyFile, etc.)
util.go         - Host key generation and SSH wire format parsing
wrap.go         - Type wrappers around golang.org/x/crypto/ssh
doc.go          - Package documentation

_examples/      - Working example applications
```

## Authentication Requirement

All servers **must** have authentication configured or `NoClientAuth` explicitly set. This is enforced at `Serve()` time. When writing examples or tests, always include authentication:

```go
// In tests, serveOnce auto-sets NoClientAuth if no auth handler is present
// For production code, always set an auth handler:
srv := &ssh.Server{
    Handler: handler,
    PasswordHandler: func(ctx ssh.Context, pass string) bool {
        return pass == "secret"
    },
}
```

## Security Considerations

When contributing, keep these in mind:

- Never leak internal error details to SSH clients (use generic error messages)
- Validate all SSH wire format data before use
- Use `subtle.ConstantTimeCompare` for key comparisons
- Cap unbounded accumulations (env vars, signals, etc.)
- Protect shared state with mutexes; use the race detector to verify
- Agent forwarding and port forwarding should be denied by default

## Submitting Changes

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Ensure `go vet` and `go test -race` pass
5. Submit a pull request

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
