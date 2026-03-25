# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Security

- **Ed25519 host keys**: Default auto-generated host keys changed from RSA-2048 to Ed25519
- **Authentication required**: Servers no longer silently fall back to `NoClientAuth`. An authentication handler must be configured or `NoClientAuth` must be explicitly set to `true`. Serve returns `ErrNoAuthConfigured` otherwise
- **Agent forwarding denied by default**: Agent forwarding requests are now rejected unless `AgentForwardingCallback` is set on the server
- **Signal validation**: Signal names are validated against RFC 4254 POSIX signals before buffering. Oversized signal strings are rejected
- **Environment variable limits**: Environment variable accumulation capped at 256 entries, 32KB per entry
- **Error message sanitization**: Port forwarding error messages no longer leak internal network topology information to clients

### Fixed

- **Race condition in `Signals()` buffer drain**: The goroutine draining buffered signals now receives copies of the buffer and channel under lock, preventing data races
- **Race condition in PTY access**: `Write()` and `Pty()` now hold the session mutex when reading `sess.pty`, preventing races with `handleRequests`
- **Race condition in `SetOption`**: Refactored `AddHostKey` to use internal `addHostKeyLocked` helper; `SetOption` now acquires the mutex
- **Inconsistent `net.Error` detection**: `Read` method in `serverConn` now uses `errors.AsType[net.Error]` matching `Write`
- **Non-blocking window-change**: Window change events use non-blocking send to prevent blocking the SSH request loop
- **Deprecated `Temporary()` call**: Replaced `ne.Temporary()` with `ne.Timeout()` in accept loop
- **`Shutdown` goroutine leak**: Context cancellation now force-closes remaining connections so the wait goroutine can exit
- **Agent listener temp directory leak**: `NewAgentListener` now returns a wrapped listener that cleans up the temp directory on `Close()`
- **Port forwarding goroutine lifecycle**: `DirectTCPIPHandler` and `ForwardedTCPHandler` copy goroutines now use `sync.WaitGroup` with proper `CloseWrite` signaling
- **`SetDeadline` error handling**: Connection deadline errors now trigger context cancellation
- **`gossh.Unmarshal` errors**: All SSH request payload unmarshaling now checks for errors and rejects malformed payloads
- **Context accessor panics**: `User()`, `SessionID()`, `ClientVersion()`, `ServerVersion()`, `LocalAddr()`, and `Permissions()` now use safe comma-ok type assertions instead of panicking on nil values
- **`break` handler lock ordering**: `req.Reply` now called after mutex unlock

### Changed

- **`Context` interface**: Removed embedded `sync.Locker`. `Lock()`/`Unlock()` are no longer exposed through the interface. Use `SetValue()` for connection-scoped state
- **`ErrPermissionDenied` sentinel error**: Authentication callbacks now return `ErrPermissionDenied` instead of `fmt.Errorf("permission denied")`
- **`interface{}` to `any`**: Context internals modernized to use `any` (Go 1.18+)
- **Removed dead code**: Cleaned up commented-out code blocks in `server.go` and stale TODO comments in `tcpip.go`

### Added

- `NoClientAuth` field on `Server` struct for explicit unauthenticated access
- `AgentForwardingCallback` type and field on `Server` struct
- `ErrNoAuthConfigured` sentinel error
- `ErrPermissionDenied` sentinel error
- Doc comments on `SubsystemHandler`, `RequestHandler`, `ChannelHandler`, `DefaultSessionHandler`
- Comprehensive test suite (60 tests, 77% coverage)
- `README.md` with full API documentation
- `CHANGELOG.md`
- `CONTRIBUTING.md` with development, testing, CI/CD, and release instructions
- Examples for all major features (simple, pty, publickey, agent forwarding, remote forwarding, sftp, timeouts, docker)
- GitHub Actions CI workflow (build, vet, race-detected tests, coverage, example verification)
- GitHub Actions security workflow (weekly `govulncheck`, dependency verification)
- GitHub Actions release workflow with immutability enforcement
- Tag protection ruleset preventing deletion or modification of `v*` tags
