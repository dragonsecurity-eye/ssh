package ssh

import (
	"io"
	"net"
	"os"
	"path"
	"sync"

	gossh "golang.org/x/crypto/ssh"
)

const (
	agentRequestType = "auth-agent-req@openssh.com"
	agentChannelType = "auth-agent@openssh.com"

	agentTempDir    = "auth-agent"
	agentListenFile = "listener.sock"
)

// contextKeyAgentRequest is an internal context key for storing if the
// client requested agent forwarding
var contextKeyAgentRequest = &contextKey{"auth-agent-req"}

// SetAgentRequested sets up the session context so that AgentRequested
// returns true.
func SetAgentRequested(ctx Context) {
	ctx.SetValue(contextKeyAgentRequest, true)
}

// AgentRequested returns true if the client requested agent forwarding.
func AgentRequested(sess Session) bool {
	return sess.Context().Value(contextKeyAgentRequest) == true
}

// NewAgentListener sets up a temporary Unix socket that can be communicated
// to the session environment and used for forwarding connections. The returned
// listener cleans up its temporary directory when closed.
func NewAgentListener() (net.Listener, error) {
	dir, err := os.MkdirTemp("", agentTempDir)
	if err != nil {
		return nil, err
	}
	l, err := net.Listen("unix", path.Join(dir, agentListenFile))
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	return &agentListener{Listener: l, dir: dir}, nil
}

// agentListener wraps a net.Listener to clean up the temp directory on Close.
type agentListener struct {
	net.Listener
	dir string
}

func (l *agentListener) Close() error {
	err := l.Listener.Close()
	os.RemoveAll(l.dir)
	return err
}

// ForwardAgentConnections takes connections from a listener to proxy into the
// session on the OpenSSH channel for agent connections. It blocks and services
// connections until the listener stops accepting.
func ForwardAgentConnections(l net.Listener, s Session) {
	sshConn := s.Context().Value(ContextKeyConn).(gossh.Conn)
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go func(conn net.Conn) {
			defer conn.Close()
			channel, reqs, err := sshConn.OpenChannel(agentChannelType, nil)
			if err != nil {
				return
			}
			defer channel.Close()
			go gossh.DiscardRequests(reqs)
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				io.Copy(conn, channel)
				conn.(*net.UnixConn).CloseWrite()
				wg.Done()
			}()
			go func() {
				io.Copy(channel, conn)
				channel.CloseWrite()
				wg.Done()
			}()
			wg.Wait()
		}(conn)
	}
}
