package ssh

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/anmitsu/go-shlex"
	gossh "golang.org/x/crypto/ssh"
)

const (
	maxEnvVars   = 256
	maxEnvBytes  = 32 * 1024
	maxSigLength = 32
)

// validSignals is the set of POSIX signals defined in RFC 4254.
var validSignals = map[Signal]struct{}{
	SIGABRT: {}, SIGALRM: {}, SIGFPE: {}, SIGHUP: {},
	SIGILL: {}, SIGINT: {}, SIGKILL: {}, SIGPIPE: {},
	SIGQUIT: {}, SIGSEGV: {}, SIGTERM: {}, SIGUSR1: {},
	SIGUSR2: {},
}

// Session provides access to information about an SSH session and methods
// to read and write to the SSH channel with an embedded Channel interface from
// crypto/ssh.
//
// When Command() returns an empty slice, the user requested a shell. Otherwise
// the user is performing an exec with those command arguments.
type Session interface {
	gossh.Channel

	// User returns the username used when establishing the SSH connection.
	User() string

	// RemoteAddr returns the net.Addr of the client side of the connection.
	RemoteAddr() net.Addr

	// LocalAddr returns the net.Addr of the server side of the connection.
	LocalAddr() net.Addr

	// Environ returns a copy of strings representing the environment set by the
	// user for this session, in the form "key=value".
	Environ() []string

	// Exit sends an exit status and then closes the session.
	Exit(code int) error

	// Command returns a shell parsed slice of arguments that were provided by the
	// user. Shell parsing splits the command string according to POSIX shell rules,
	// which considers quoting not just whitespace.
	Command() []string

	// RawCommand returns the exact command that was provided by the user.
	RawCommand() string

	// Subsystem returns the subsystem requested by the user.
	Subsystem() string

	// PublicKey returns the PublicKey used to authenticate. If a public key was not
	// used it will return nil.
	PublicKey() PublicKey

	// Context returns the connection's context. The returned context is always
	// non-nil and holds the same data as the Context passed into auth
	// handlers and callbacks.
	//
	// The context is canceled when the client's connection closes or I/O
	// operation fails.
	Context() Context

	// Permissions returns a copy of the Permissions object that was available for
	// setup in the auth handlers via the Context.
	Permissions() Permissions

	// Pty returns PTY information, a channel of window size changes, and a boolean
	// of whether or not a PTY was accepted for this session.
	Pty() (Pty, <-chan Window, bool)

	// Signals registers a channel to receive signals sent from the client. The
	// channel must handle signal sends or it will block the SSH request loop.
	// Registering nil will unregister the channel from signal sends. During the
	// time no channel is registered signals are buffered up to a reasonable amount.
	// If there are buffered signals when a channel is registered, they will be
	// sent in order on the channel immediately after registering.
	Signals(c chan<- Signal)

	// Break registers a channel to receive notifications of break requests sent
	// from the client. The channel must handle break requests, or it will block
	// the request handling loop. Registering nil will unregister the channel.
	// During the time that no channel is registered, breaks are ignored.
	Break(c chan<- bool)
}

// maxSigBufSize is how many signals will be buffered
// when there is no signal channel specified
const maxSigBufSize = 128

// DefaultSessionHandler is the default channel handler for "session" channels.
func DefaultSessionHandler(srv *Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx Context) {
	ch, reqs, err := newChan.Accept()
	if err != nil {
		return
	}
	sess := &session{
		Channel:           ch,
		conn:              conn,
		handler:           srv.Handler,
		ptyCb:             srv.PtyCallback,
		sessReqCb:         srv.SessionRequestCallback,
		agentCb:           srv.AgentForwardingCallback,
		subsystemHandlers: srv.SubsystemHandlers,
		ctx:               ctx,
	}
	sess.handleRequests(reqs)
}

type session struct {
	sync.Mutex
	gossh.Channel
	conn              *gossh.ServerConn
	handler           Handler
	subsystemHandlers map[string]SubsystemHandler
	handled           bool
	exited            bool
	pty               *Pty
	winch             chan Window
	env               []string
	ptyCb             PtyCallback
	sessReqCb         SessionRequestCallback
	agentCb           AgentForwardingCallback
	rawCmd            string
	subsystem         string
	ctx               Context
	sigCh             chan<- Signal
	sigBuf            []Signal
	breakCh           chan<- bool
}

func (sess *session) Write(p []byte) (n int, err error) {
	sess.Lock()
	isPty := sess.pty != nil
	sess.Unlock()
	if isPty {
		m := len(p)
		// normalize \n to \r\n when pty is accepted.
		// this is a hardcoded shortcut since we don't support terminal modes.
		p = bytes.ReplaceAll(p, []byte{'\n'}, []byte{'\r', '\n'})
		p = bytes.ReplaceAll(p, []byte{'\r', '\r', '\n'}, []byte{'\r', '\n'})
		n, err = sess.Channel.Write(p)
		if n > m {
			n = m
		}
		return
	}
	return sess.Channel.Write(p)
}

func (sess *session) PublicKey() PublicKey {
	sessionkey := sess.ctx.Value(ContextKeyPublicKey)
	if sessionkey == nil {
		return nil
	}
	return sessionkey.(PublicKey)
}

func (sess *session) Permissions() Permissions {
	// use context permissions because its properly
	// wrapped and easier to dereference
	perms := sess.ctx.Value(ContextKeyPermissions).(*Permissions)
	return *perms
}

func (sess *session) Context() Context {
	return sess.ctx
}

func (sess *session) Exit(code int) error {
	sess.Lock()
	defer sess.Unlock()
	if sess.exited {
		return errors.New("Session.Exit called multiple times")
	}
	sess.exited = true

	status := struct{ Status uint32 }{uint32(code)}
	_, err := sess.SendRequest("exit-status", false, gossh.Marshal(&status))
	if err != nil {
		return err
	}
	return sess.Close()
}

func (sess *session) User() string {
	return sess.conn.User()
}

func (sess *session) RemoteAddr() net.Addr {
	return sess.conn.RemoteAddr()
}

func (sess *session) LocalAddr() net.Addr {
	return sess.conn.LocalAddr()
}

func (sess *session) Environ() []string {
	return append([]string(nil), sess.env...)
}

func (sess *session) RawCommand() string {
	return sess.rawCmd
}

func (sess *session) Command() []string {
	cmd, _ := shlex.Split(sess.rawCmd, true)
	return append([]string(nil), cmd...)
}

func (sess *session) Subsystem() string {
	return sess.subsystem
}

func (sess *session) Pty() (Pty, <-chan Window, bool) {
	sess.Lock()
	defer sess.Unlock()
	if sess.pty != nil {
		return *sess.pty, sess.winch, true
	}
	return Pty{}, sess.winch, false
}

func (sess *session) Signals(c chan<- Signal) {
	sess.Lock()
	sess.sigCh = c
	buf := sess.sigBuf
	sess.sigBuf = nil
	sess.Unlock()
	if len(buf) > 0 && c != nil {
		go func() {
			for _, sig := range buf {
				c <- sig
			}
		}()
	}
}

func (sess *session) Break(c chan<- bool) {
	sess.Lock()
	defer sess.Unlock()
	sess.breakCh = c
}

func (sess *session) handleRequests(reqs <-chan *gossh.Request) {
	for req := range reqs {
		switch req.Type {
		case "shell", "exec":
			if sess.handled {
				req.Reply(false, nil)
				continue
			}

			var payload = struct{ Value string }{}
			if len(req.Payload) > 0 {
				if err := gossh.Unmarshal(req.Payload, &payload); err != nil {
					req.Reply(false, nil)
					continue
				}
			}
			sess.rawCmd = payload.Value

			// If there's a session policy callback, we need to confirm before
			// accepting the session.
			if sess.sessReqCb != nil && !sess.sessReqCb(sess, req.Type) {
				sess.rawCmd = ""
				req.Reply(false, nil)
				continue
			}

			sess.handled = true
			req.Reply(true, nil)

			go func() {
				sess.handler(sess)
				sess.Exit(0)
			}()
		case "subsystem":
			if sess.handled {
				req.Reply(false, nil)
				continue
			}

			var payload = struct{ Value string }{}
			if len(req.Payload) == 0 || gossh.Unmarshal(req.Payload, &payload) != nil {
				req.Reply(false, nil)
				continue
			}
			sess.subsystem = payload.Value

			// If there's a session policy callback, we need to confirm before
			// accepting the session.
			if sess.sessReqCb != nil && !sess.sessReqCb(sess, req.Type) {
				sess.rawCmd = ""
				req.Reply(false, nil)
				continue
			}

			handler := sess.subsystemHandlers[payload.Value]
			if handler == nil {
				handler = sess.subsystemHandlers["default"]
			}
			if handler == nil {
				req.Reply(false, nil)
				continue
			}

			sess.handled = true
			req.Reply(true, nil)

			go func() {
				handler(sess)
				sess.Exit(0)
			}()
		case "env":
			if sess.handled {
				req.Reply(false, nil)
				continue
			}
			var kv struct{ Key, Value string }
			if err := gossh.Unmarshal(req.Payload, &kv); err != nil {
				req.Reply(false, nil)
				continue
			}
			if len(sess.env) >= maxEnvVars {
				req.Reply(false, nil)
				continue
			}
			envEntry := fmt.Sprintf("%s=%s", kv.Key, kv.Value)
			if len(envEntry) > maxEnvBytes {
				req.Reply(false, nil)
				continue
			}
			sess.env = append(sess.env, envEntry)
			req.Reply(true, nil)
		case "signal":
			var payload struct{ Signal string }
			if err := gossh.Unmarshal(req.Payload, &payload); err != nil {
				req.Reply(false, nil)
				continue
			}
			sig := Signal(payload.Signal)
			if _, ok := validSignals[sig]; !ok && len(payload.Signal) > maxSigLength {
				req.Reply(false, nil)
				continue
			}
			sess.Lock()
			if sess.sigCh != nil {
				sess.sigCh <- sig
			} else {
				if len(sess.sigBuf) < maxSigBufSize {
					sess.sigBuf = append(sess.sigBuf, sig)
				}
			}
			sess.Unlock()
		case "pty-req":
			sess.Lock()
			alreadyHandled := sess.handled || sess.pty != nil
			sess.Unlock()
			if alreadyHandled {
				req.Reply(false, nil)
				continue
			}
			ptyReq, ok := parsePtyRequest(req.Payload)
			if !ok {
				req.Reply(false, nil)
				continue
			}
			if sess.ptyCb != nil {
				ok := sess.ptyCb(sess.ctx, ptyReq)
				if !ok {
					req.Reply(false, nil)
					continue
				}
			}
			sess.Lock()
			sess.pty = &ptyReq
			sess.winch = make(chan Window, 1)
			sess.Unlock()
			sess.winch <- ptyReq.Window
			defer func() {
				close(sess.winch)
			}()
			req.Reply(ok, nil)
		case "window-change":
			sess.Lock()
			hasPty := sess.pty != nil
			sess.Unlock()
			if !hasPty {
				req.Reply(false, nil)
				continue
			}
			win, ok := parseWinchRequest(req.Payload)
			if ok {
				sess.Lock()
				sess.pty.Window = win
				sess.Unlock()
				select {
				case sess.winch <- win:
				default:
				}
			}
			req.Reply(ok, nil)
		case agentRequestType:
			if sess.agentCb != nil && sess.agentCb(sess.ctx) {
				SetAgentRequested(sess.ctx)
				req.Reply(true, nil)
			} else {
				req.Reply(false, nil)
			}
		case "break":
			sess.Lock()
			ok := false
			if sess.breakCh != nil {
				sess.breakCh <- true
				ok = true
			}
			sess.Unlock()
			req.Reply(ok, nil)
		default:
			req.Reply(false, nil)
		}
	}
}
