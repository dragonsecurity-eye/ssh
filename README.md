# DragonEye SSH

A high-level Go library for building SSH servers, inspired by the `net/http` API.

## Quick Start

```go
package main

import (
    "io"
    "log"

    "eye.dragonsecurity.io/ssh"
)

func main() {
    ssh.Handle(func(s ssh.Session) {
        io.WriteString(s, "Hello, "+s.User()+"!\n")
    })

    log.Fatal(ssh.ListenAndServe(":2222", nil,
        ssh.PasswordAuth(func(ctx ssh.Context, pass string) bool {
            return pass == "secret"
        }),
    ))
}
```

## Authentication

At least one authentication method **must** be configured, or `NoClientAuth` must be explicitly set to `true`. Attempting to serve without authentication configuration returns `ErrNoAuthConfigured`.

### Password Authentication

```go
ssh.ListenAndServe(":2222", handler,
    ssh.PasswordAuth(func(ctx ssh.Context, password string) bool {
        return password == "secret"
    }),
)
```

### Public Key Authentication

```go
ssh.ListenAndServe(":2222", handler,
    ssh.PublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
        data, _ := os.ReadFile("/path/to/authorized_keys")
        allowed, _, _, _, _ := ssh.ParseAuthorizedKey(data)
        return ssh.KeysEqual(key, allowed)
    }),
)
```

### Keyboard-Interactive Authentication

```go
ssh.ListenAndServe(":2222", handler,
    ssh.KeyboardInteractiveAuth(func(ctx ssh.Context, challenger gossh.KeyboardInteractiveChallenge) bool {
        // implement challenge-response
        return true
    }),
)
```

### No Authentication (explicit opt-in)

```go
srv := &ssh.Server{
    Addr:         ":2222",
    Handler:      handler,
    NoClientAuth: true, // required - will not silently default to open
}
log.Fatal(srv.ListenAndServe())
```

## Host Keys

If no host key is provided, an Ed25519 key is generated automatically on each start. For production, specify a persistent host key:

```go
// From file
ssh.ListenAndServe(":2222", handler, ssh.HostKeyFile("/etc/ssh/ssh_host_ed25519_key"))

// From PEM bytes
ssh.ListenAndServe(":2222", handler, ssh.HostKeyPEM(pemBytes))

// Programmatically
srv := &ssh.Server{Handler: handler}
srv.AddHostKey(signer)
```

## Session Handling

The `Session` interface provides access to the SSH session:

```go
func handler(s ssh.Session) {
    // Connection info
    user := s.User()
    addr := s.RemoteAddr()

    // Command execution
    rawCmd := s.RawCommand()   // exact command string
    args := s.Command()         // POSIX shell-parsed arguments
    // When Command() returns empty, the user requested a shell

    // Environment variables (set by client before exec/shell)
    env := s.Environ()

    // PTY support
    pty, winCh, isPty := s.Pty()
    if isPty {
        // pty.Term = terminal type (e.g. "xterm-256color")
        // pty.Window = initial window size
        // winCh receives window resize events
    }

    // Signals
    sigCh := make(chan ssh.Signal, 1)
    s.Signals(sigCh)

    // Read/Write (s embeds gossh.Channel)
    io.WriteString(s, "Hello!\n")
    io.Copy(s, s) // echo stdin back

    // Exit with status code
    s.Exit(0)
}
```

## Subsystems

Register named subsystem handlers on the server:

```go
srv := &ssh.Server{
    Handler: sessionHandler,
    SubsystemHandlers: map[string]ssh.SubsystemHandler{
        "sftp": sftpHandler,
    },
}
```

## Port Forwarding

### Local (direct-tcpip)

```go
srv := &ssh.Server{
    Handler: handler,
    LocalPortForwardingCallback: func(ctx ssh.Context, host string, port uint32) bool {
        return host == "localhost" // only allow forwarding to localhost
    },
    ChannelHandlers: map[string]ssh.ChannelHandler{
        "session":      ssh.DefaultSessionHandler,
        "direct-tcpip": ssh.DirectTCPIPHandler,
    },
}
```

### Reverse (tcpip-forward)

```go
forwardHandler := &ssh.ForwardedTCPHandler{}
srv := &ssh.Server{
    Handler: handler,
    ReversePortForwardingCallback: func(ctx ssh.Context, host string, port uint32) bool {
        return port >= 1024 // only allow unprivileged ports
    },
    RequestHandlers: map[string]ssh.RequestHandler{
        "tcpip-forward":        forwardHandler.HandleSSHRequest,
        "cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
    },
}
```

## Agent Forwarding

Agent forwarding is **denied by default**. Enable it with a callback:

```go
srv := &ssh.Server{
    Handler: handler,
    AgentForwardingCallback: func(ctx ssh.Context) bool {
        return true // allow agent forwarding
    },
}
```

Then in your session handler:

```go
func handler(s ssh.Session) {
    if ssh.AgentRequested(s) {
        l, err := ssh.NewAgentListener()
        if err != nil {
            log.Fatal(err)
        }
        defer l.Close()
        go ssh.ForwardAgentConnections(l, s)
        // l.Addr().String() can be set as SSH_AUTH_SOCK
    }
}
```

## Connection Timeouts

```go
srv := &ssh.Server{
    Handler:          handler,
    HandshakeTimeout: 10 * time.Second,  // max time to complete SSH handshake
    IdleTimeout:      5 * time.Minute,   // disconnect after inactivity
    MaxTimeout:       1 * time.Hour,     // absolute max connection lifetime
}
```

## Server Lifecycle

```go
srv := &ssh.Server{...}

// Graceful shutdown
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
srv.Shutdown(ctx)

// Immediate close
srv.Close()
```

## Functional Options

Configure servers using the functional option pattern:

| Option | Description |
|--------|-------------|
| `PasswordAuth(fn)` | Set password authentication handler |
| `PublicKeyAuth(fn)` | Set public key authentication handler |
| `KeyboardInteractiveAuth(fn)` | Set keyboard-interactive handler |
| `HostKeyFile(path)` | Load host key from PEM file |
| `HostKeyPEM(bytes)` | Load host key from PEM bytes |
| `NoPty()` | Deny all PTY requests |
| `WrapConn(fn)` | Wrap `net.Conn` before handling |

## Context

The `ssh.Context` interface extends `context.Context` with SSH-specific metadata:

```go
ctx.User()           // username
ctx.SessionID()      // hex-encoded session hash
ctx.ClientVersion()  // client version string
ctx.ServerVersion()  // server version string
ctx.RemoteAddr()     // client address
ctx.LocalAddr()      // server address
ctx.Permissions()    // auth permissions
ctx.SetValue(k, v)   // store custom values
```

## Security Notes

- **Ed25519 host keys** are generated by default (not RSA)
- **Authentication is required** unless `NoClientAuth` is explicitly set
- **Agent forwarding is denied** unless `AgentForwardingCallback` is configured
- **Port forwarding callbacks** deny by default when nil
- **`KeysEqual`** uses constant-time comparison to prevent timing attacks
- Signal names are validated against RFC 4254 POSIX signals
- Environment variable accumulation is capped at 256 entries (32KB each)
- Port forwarding error messages are sanitized to prevent information leakage

## License

Apache License 2.0
