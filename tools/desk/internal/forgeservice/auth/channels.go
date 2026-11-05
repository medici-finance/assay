package auth

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/commstransport"
)

func (c *CurrentClient) probe(ctx context.Context, p CurrentProbe) (SignedProof, error) {
	var zero SignedProof
	if ctx == nil || ctx.Err() != nil || c.clock.Now().IsZero() {
		return zero, ErrHeld
	}
	b, e := encode(wireRequest{Schema: Schema, Operation: "current", Probe: &p}, c.config.MaxFrameBytes)
	if e != nil {
		return zero, ErrHeld
	}
	call, cancel := context.WithTimeout(ctx, c.config.IOTimeout)
	defer cancel()
	conn, e := commstransport.DialUnixContext(call, c.config.Path, c.config.IOTimeout)
	if e != nil {
		return zero, ErrHeld
	}
	defer conn.Close()
	stop := context.AfterFunc(call, func() { conn.Close() })
	defer stop()
	deadline, _ := call.Deadline()
	if conn.SetDeadline(deadline) != nil {
		return zero, ErrHeld
	}
	if _, e = io.Copy(conn, bytes.NewReader(b)); e != nil {
		return zero, ErrHeld
	}
	u, ok := conn.(*net.UnixConn)
	if !ok || u.CloseWrite() != nil {
		return zero, ErrHeld
	}
	var response wireResponse
	if readFrame(conn, c.config.MaxFrameBytes, &response) != nil || response.Schema != Schema || response.Error != "" || response.Proof == nil || response.Grant != nil || response.Receipt != nil || call.Err() != nil {
		return zero, ErrHeld
	}
	return *response.Proof, nil
}

func (c *Controller) ServeCurrent(ctx context.Context, _ CurrentChannel) error {
	return c.serve(ctx, c.resources.Current, "current.sock", c.dataSlots, func(ctx context.Context, q wireRequest) wireResponse {
		if q.Schema != Schema || q.Operation != "current" || q.Probe == nil || q.Control != nil || q.Readback {
			return heldResponse()
		}
		p, e := c.current(ctx, *q.Probe)
		if e != nil {
			return heldResponse()
		}
		return wireResponse{Schema: Schema, Proof: &p}
	})
}
func (c *Controller) ServeGrant(ctx context.Context, ch GrantChannel) error {
	r := c.resources.Grants[ch.LaunchID]
	if r == nil || !name(ch.LaunchID) {
		return ErrConfig
	}
	return c.serve(ctx, r, "grant.sock", c.dataSlots, func(ctx context.Context, q wireRequest) wireResponse {
		if q.Schema != Schema || q.Operation != "grant" || q.Probe != nil || q.Control != nil || q.Readback {
			return heldResponse()
		}
		g, e := c.grant(ctx, ch.LaunchID)
		if e != nil {
			return heldResponse()
		}
		return wireResponse{Schema: Schema, Grant: &g}
	})
}
func heldResponse() wireResponse { return wireResponse{Schema: Schema, Error: "held"} }

// claimChannel runs only for the owner's fixed channel paths, under the
// original exclusive gate. An active or uncertain holder is never recovered.
func (c *Controller) claimChannel(ctx context.Context, root *os.Root, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx.Err() != nil || !c.liveGate() {
		return ErrHeld
	}
	known := (root == c.resources.Private && name == "control.sock") || (root == c.resources.Current && name == "current.sock")
	if name == "grant.sock" {
		for _, r := range c.resources.Grants {
			known = known || root == r
		}
	}
	if !known {
		return ErrHeld
	}
	key := channelKey{root, name}
	if c.channels[key] {
		return ErrHeld
	}
	if c.channels == nil {
		c.channels = map[channelKey]bool{}
	}
	c.channels[key] = true
	st, e := root.Lstat(name)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil || st.Mode()&os.ModeSocket == 0 {
		return ErrHeld
	}
	at, e := os.Lstat(filepath.Join(root.Name(), name))
	if e != nil || !os.SameFile(st, at) {
		return ErrHeld
	}
	again, e := root.Lstat(name)
	if e != nil || again.Mode()&os.ModeSocket == 0 || !os.SameFile(st, again) || ctx.Err() != nil || !c.liveGate() {
		return ErrHeld
	}
	if root.Remove(name) != nil {
		return ErrHeld
	}
	return nil
}

func (c *Controller) serve(ctx context.Context, root *os.Root, name string, slots chan struct{}, handle func(context.Context, wireRequest) wireResponse) error {
	if ctx == nil || ctx.Err() != nil {
		return ErrHeld
	}
	if c.claimChannel(ctx, root, name) != nil {
		return ErrHeld
	}
	path := filepath.Join(root.Name(), name)
	endpoint, e := commstransport.Listen(path)
	if e != nil {
		return ErrHeld
	}
	listener, ok := endpoint.(*net.UnixListener)
	if !ok || listener == nil {
		endpoint.Close()
		return ErrHeld
	}
	listener.SetUnlinkOnClose(false)
	info, e := root.Lstat(name)
	if e != nil || info.Mode()&os.ModeSocket == 0 {
		listener.Close()
		return ErrHeld
	}
	defer func() {
		listener.Close()
		if st, e := root.Lstat(name); e == nil && st.Mode()&os.ModeSocket != 0 && os.SameFile(st, info) {
			root.Remove(name)
		}
	}()
	c.mu.Lock()
	live := ctx.Err() == nil && c.liveGate()
	c.mu.Unlock()
	if !live || root.Chmod(name, 0600) != nil {
		return ErrHeld
	}
	stop := context.AfterFunc(ctx, func() { listener.Close() })
	defer stop()
	var active sync.WaitGroup
	defer active.Wait()
	for {
		conn, e := listener.AcceptUnix()
		if e != nil {
			if ctx.Err() != nil {
				return nil
			}
			return ErrHeld
		}
		select {
		case slots <- struct{}{}:
		default:
			conn.Close()
			continue
		}
		active.Add(1)
		go func() {
			defer active.Done()
			defer func() { <-slots }()
			defer conn.Close()
			call, cancel := context.WithTimeout(ctx, c.config.IOTimeout)
			defer cancel()
			stop := context.AfterFunc(call, func() { conn.Close() })
			defer stop()
			deadline, _ := call.Deadline()
			if conn.SetDeadline(deadline) != nil {
				return
			}
			var q wireRequest
			response := heldResponse()
			if readFrame(conn, c.config.MaxFrameBytes, &q) == nil && call.Err() == nil {
				response = handle(call, q)
			}
			b, e := encode(response, c.config.MaxFrameBytes)
			if e != nil {
				return
			}
			if call.Err() == nil {
				io.Copy(conn, bytes.NewReader(b))
			}
		}()
	}
}

// Grant delivery is bound to the protected per-launch endpoint. No role, grant
// selector or arbitrary mint payload is accepted by the request protocol.
func ReadGrant(ctx context.Context, path string, max int, timeout time.Duration) (SignedGrant, error) {
	if ctx == nil || !filepath.IsAbs(path) || !text(path) || max <= 0 || max > 128*1024 || timeout <= 0 {
		return SignedGrant{}, ErrConfig
	}
	call, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, e := commstransport.DialUnixContext(call, path, timeout)
	if e != nil {
		return SignedGrant{}, ErrHeld
	}
	defer conn.Close()
	stop := context.AfterFunc(call, func() { conn.Close() })
	defer stop()
	deadline, _ := call.Deadline()
	if conn.SetDeadline(deadline) != nil {
		return SignedGrant{}, ErrHeld
	}
	b, e := encode(wireRequest{Schema: Schema, Operation: "grant"}, max)
	if e != nil {
		return SignedGrant{}, ErrHeld
	}
	if _, e = io.Copy(conn, bytes.NewReader(b)); e != nil {
		return SignedGrant{}, ErrHeld
	}
	u, ok := conn.(*net.UnixConn)
	if !ok || u.CloseWrite() != nil {
		return SignedGrant{}, ErrHeld
	}
	var r wireResponse
	if readFrame(conn, max, &r) != nil || r.Schema != Schema || r.Error != "" || r.Grant == nil || r.Proof != nil || r.Receipt != nil || call.Err() != nil {
		return SignedGrant{}, ErrHeld
	}
	return *r.Grant, nil
}
