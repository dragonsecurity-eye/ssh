package ssh

import (
	"io"
	"log"
	"net"
	"strconv"
	"sync"

	gossh "golang.org/x/crypto/ssh"
)

const (
	forwardedTCPChannelType = "forwarded-tcpip"
)

// direct-tcpip data struct as specified in RFC4254, Section 7.2
type localForwardChannelData struct {
	DestAddr string
	DestPort uint32

	OriginAddr string
	OriginPort uint32
}

// DirectTCPIPHandler can be enabled by adding it to the server's
// ChannelHandlers under direct-tcpip.
func DirectTCPIPHandler(srv *Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx Context) {
	d := localForwardChannelData{}
	if err := gossh.Unmarshal(newChan.ExtraData(), &d); err != nil {
		newChan.Reject(gossh.ConnectionFailed, "port forwarding request denied")
		return
	}

	if srv.LocalPortForwardingCallback == nil || !srv.LocalPortForwardingCallback(ctx, d.DestAddr, d.DestPort) {
		newChan.Reject(gossh.Prohibited, "port forwarding is disabled")
		return
	}

	dest := net.JoinHostPort(d.DestAddr, strconv.FormatInt(int64(d.DestPort), 10))

	var dialer net.Dialer
	dconn, err := dialer.DialContext(ctx, "tcp", dest)
	if err != nil {
		newChan.Reject(gossh.ConnectionFailed, "port forwarding request denied")
		return
	}

	ch, reqs, err := newChan.Accept()
	if err != nil {
		dconn.Close()
		return
	}
	go gossh.DiscardRequests(reqs)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(ch, dconn)
		ch.CloseWrite()
	}()
	go func() {
		defer wg.Done()
		io.Copy(dconn, ch)
		if tc, ok := dconn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()
	wg.Wait()
	ch.Close()
	dconn.Close()
}

type remoteForwardRequest struct {
	BindAddr string
	BindPort uint32
}

type remoteForwardSuccess struct {
	BindPort uint32
}

type remoteForwardCancelRequest struct {
	BindAddr string
	BindPort uint32
}

type remoteForwardChannelData struct {
	DestAddr   string
	DestPort   uint32
	OriginAddr string
	OriginPort uint32
}

// ForwardedTCPHandler can be enabled by creating a ForwardedTCPHandler and
// adding the HandleSSHRequest callback to the server's RequestHandlers under
// tcpip-forward and cancel-tcpip-forward.
type ForwardedTCPHandler struct {
	forwards map[string]net.Listener
	sync.Mutex
}

func (h *ForwardedTCPHandler) HandleSSHRequest(ctx Context, srv *Server, req *gossh.Request) (bool, []byte) {
	h.Lock()
	if h.forwards == nil {
		h.forwards = make(map[string]net.Listener)
	}
	h.Unlock()
	conn := ctx.Value(ContextKeyConn).(*gossh.ServerConn)
	switch req.Type {
	case "tcpip-forward":
		var reqPayload remoteForwardRequest
		if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
			return false, []byte{}
		}
		if srv.ReversePortForwardingCallback == nil || !srv.ReversePortForwardingCallback(ctx, reqPayload.BindAddr, reqPayload.BindPort) {
			return false, []byte("port forwarding is disabled")
		}
		addr := net.JoinHostPort(reqPayload.BindAddr, strconv.Itoa(int(reqPayload.BindPort)))
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return false, []byte{}
		}
		_, destPortStr, _ := net.SplitHostPort(ln.Addr().String())
		destPort, _ := strconv.Atoi(destPortStr)
		h.Lock()
		h.forwards[addr] = ln
		h.Unlock()
		go func() {
			<-ctx.Done()
			h.Lock()
			ln, ok := h.forwards[addr]
			h.Unlock()
			if ok {
				ln.Close()
			}
		}()
		go func() {
			for {
				c, err := ln.Accept()
				if err != nil {
					break
				}
				originAddr, originPortStr, _ := net.SplitHostPort(c.RemoteAddr().String())
				originPort, _ := strconv.Atoi(originPortStr)
				payload := gossh.Marshal(&remoteForwardChannelData{
					DestAddr:   reqPayload.BindAddr,
					DestPort:   uint32(destPort),
					OriginAddr: originAddr,
					OriginPort: uint32(originPort),
				})
				go func() {
					ch, reqs, err := conn.OpenChannel(forwardedTCPChannelType, payload)
					if err != nil {
						log.Println(err)
						c.Close()
						return
					}
					go gossh.DiscardRequests(reqs)
					var wg sync.WaitGroup
					wg.Add(2)
					go func() {
						defer wg.Done()
						io.Copy(ch, c)
					}()
					go func() {
						defer wg.Done()
						io.Copy(c, ch)
					}()
					wg.Wait()
					ch.Close()
					c.Close()
				}()
			}
			h.Lock()
			delete(h.forwards, addr)
			h.Unlock()
		}()
		return true, gossh.Marshal(&remoteForwardSuccess{uint32(destPort)})

	case "cancel-tcpip-forward":
		var reqPayload remoteForwardCancelRequest
		if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
			return false, []byte{}
		}
		addr := net.JoinHostPort(reqPayload.BindAddr, strconv.Itoa(int(reqPayload.BindPort)))
		h.Lock()
		ln, ok := h.forwards[addr]
		h.Unlock()
		if ok {
			ln.Close()
		}
		return true, nil
	default:
		return false, nil
	}
}
