# Examples

Working examples demonstrating DragonEye SSH library features.

## Running

Each example is a standalone Go module. To run:

```bash
cd _examples/ssh-simple
go run simple.go
```

Then connect with: `ssh -p 2222 localhost` (default password: `secret`).

## Examples

| Example | Description |
|---------|-------------|
| [ssh-simple](ssh-simple/) | Minimal SSH server that greets the user |
| [ssh-pty](ssh-pty/) | PTY allocation with `top` command and window resize handling |
| [ssh-publickey](ssh-publickey/) | Public key authentication (accepts all keys) |
| [ssh-forwardagent](ssh-forwardagent/) | SSH agent forwarding with `ssh-add -l` |
| [ssh-remoteforward](ssh-remoteforward/) | Local and reverse TCP port forwarding |
| [ssh-sftpserver](ssh-sftpserver/) | SFTP subsystem server using `github.com/pkg/sftp` |
| [ssh-timeouts](ssh-timeouts/) | Idle and max connection timeouts |
| [ssh-docker](ssh-docker/) | Docker container execution over SSH |

## Dependencies

Most examples only depend on the SSH library itself. Some have additional dependencies:

- **ssh-pty**: `github.com/creack/pty` (PTY allocation)
- **ssh-sftpserver**: `github.com/pkg/sftp` (SFTP protocol)
- **ssh-docker**: `github.com/docker/docker` (Docker API client)
