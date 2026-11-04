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
	conn, e := (&net.Dialer{Timeout: c.config.IOTimeout}).DialContext(call, "unix", c.config.Path)
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
	if readFrame(conn, c.config.MaxFrameBytes, &response) != nil || response.Schema != Schema || response.Error != "" || response.Proof == nil || response.Grant != nil || call.Err() != nil {
		return zero, ErrHeld
	}
	return *response.Proof, nil
}

func (c *Controller) ServeCurrent(ctx context.Context, _ CurrentChannel) error {
	return c.serve(ctx, c.resources.Current, "current.sock", func(ctx context.Context, q wireRequest) wireResponse {
		if q.Schema != Schema || q.Operation != "current" || q.Probe == nil {
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
	return c.serve(ctx, r, "grant.sock", func(ctx context.Context, q wireRequest) wireResponse {
		if q.Schema != Schema || q.Operation != "grant" || q.Probe != nil {
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

func (c *Controller) serve(ctx context.Context, root *os.Root, name string, handle func(context.Context, wireRequest) wireResponse) error {
	if ctx == nil || ctx.Err() != nil {
		return ErrHeld
	}
	c.mu.Lock()
	live := c.liveGate()
	c.mu.Unlock()
	if !live {
		return ErrHeld
	}
	path := filepath.Join(root.Name(), name)
	listener, e := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if e != nil {
		return ErrHeld
	}
	listener.SetUnlinkOnClose(false)
	info, e := root.Lstat(name)
	if e != nil || os.Chmod(path, 0600) != nil {
		listener.Close()
		return ErrHeld
	}
	defer func() {
		listener.Close()
		if st, e := root.Lstat(name); e == nil && os.SameFile(st, info) {
			root.Remove(name)
		}
	}()
	stop := context.AfterFunc(ctx, func() { listener.Close() })
	defer stop()
	slots := make(chan struct{}, c.config.MaxConnections)
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
	conn, e := (&net.Dialer{Timeout: timeout}).DialContext(call, "unix", path)
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
	if readFrame(conn, max, &r) != nil || r.Schema != Schema || r.Error != "" || r.Grant == nil || r.Proof != nil || call.Err() != nil {
		return SignedGrant{}, ErrHeld
	}
	return *r.Grant, nil
}
