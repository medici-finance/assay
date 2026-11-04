package auth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/commstransport"
)

func retainedSocket(t *testing.T, r *os.Root) os.FileInfo {
	t.Helper()
	l, e := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(r.Name(), "current.sock"), Net: "unix"})
	if e != nil {
		t.Fatal(e)
	}
	l.SetUnlinkOnClose(false)
	if e = l.Close(); e != nil {
		t.Fatal(e)
	}
	st, e := r.Lstat("current.sock")
	if e != nil || st.Mode()&os.ModeSocket == 0 {
		t.Fatal("actual retained socket unavailable")
	}
	return st
}
func newChannelController(t *testing.T) (*Controller, ProducerResources) {
	t.Helper()
	cfg, b := fixtureConfig()
	r := fixtureResources(t, b)
	c, e := NewController(cfg, r, &fixtureWall{value: time.Now().UTC()})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	return c, r
}

func TestForgeLocalAuthorityChannelRestart(t *testing.T) {
	t.Run("retained-socket", func(t *testing.T) {
		c, r := newChannelController(t)
		old := retainedSocket(t, r.Current)
		path := filepath.Join(r.Current.Name(), "current.sock")
		prior, priorErr := commstransport.DialUnixContext(context.Background(), path, 25*time.Millisecond)
		if prior != nil {
			prior.Close()
		}
		if !errors.Is(priorErr, syscall.ECONNREFUSED) {
			t.Fatal("retained fixture is not a closed listener", priorErr)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- c.ServeCurrent(ctx, CurrentChannel{}) }()
		t.Cleanup(func() {
			cancel()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Error("restarted channel did not stop")
			}
		})
		// One absolute budget spans connection, request and response. An inode
		// can be reused; a closed old socket cannot return this framed reply.
		deadline := time.Now().Add(time.Second)
		started := time.Now()
		probeCtx, stopProbe := context.WithDeadline(context.Background(), deadline)
		defer stopProbe()
		for {
			select {
			case e := <-done:
				done <- e
				t.Fatal("retained fixed channel refused after exclusive owner acquisition", e)
			default:
			}
			conn, dialErr := commstransport.DialUnixContext(probeCtx, path, time.Until(deadline))
			if dialErr == nil {
				if e := conn.SetDeadline(deadline); e != nil {
					conn.Close()
					t.Fatal(e)
				}
				request, e := encode(wireRequest{Schema: Schema, Operation: "current", Probe: &CurrentProbe{Schema: Schema}}, c.config.MaxFrameBytes)
				if e != nil {
					conn.Close()
					t.Fatal(e)
				}
				if _, e = io.Copy(conn, bytes.NewReader(request)); e != nil {
					conn.Close()
					t.Fatal("rebound current request", e)
				}
				u, ok := conn.(*net.UnixConn)
				if !ok || u.CloseWrite() != nil {
					conn.Close()
					t.Fatal("rebound current request not framed")
				}
				var response wireResponse
				e = readFrame(conn, c.config.MaxFrameBytes, &response)
				conn.Close()
				if e != nil || response.Schema != Schema || response.Error != "held" || response.Grant != nil || response.Proof != nil || response.Receipt != nil {
					t.Fatal("rebound current channel did not return typed held", e)
				}
				st, e := r.Current.Lstat("current.sock")
				if e != nil || st.Mode()&os.ModeSocket == 0 || st.Mode().Perm() != 0600 || probeCtx.Err() != nil || !time.Now().Before(deadline) {
					t.Fatal("rebound current reply was not owner-only and within the absolute deadline", e)
				}
				break
			}
			if time.Now().After(deadline) {
				current, statErr := r.Current.Lstat("current.sock")
				var stat any
				if current != nil {
					stat = current.Sys()
				}
				t.Fatalf("retained channel never replied by deadline: elapsed=%s path=%s old_stat=%#v current_stat=%#v stat_error=%v dial_error=%v", time.Since(started), path, old.Sys(), stat, statErr, dialErr)
			}
			if !errors.Is(dialErr, os.ErrNotExist) && !errors.Is(dialErr, syscall.ECONNREFUSED) {
				t.Fatal("rebound current dial", dialErr)
			}
			time.Sleep(time.Millisecond)
		}
	})
	for _, kind := range []string{"regular", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			c, r := newChannelController(t)
			if kind == "regular" {
				f, e := r.Current.OpenFile("current.sock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if e != nil {
					t.Fatal(e)
				}
				f.Write([]byte("fixture"))
				f.Close()
			} else {
				if e := r.Current.Symlink("fixture-target", "current.sock"); e != nil {
					t.Fatal(e)
				}
			}
			old, e := r.Current.Lstat("current.sock")
			if e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			if e = c.ServeCurrent(ctx, CurrentChannel{}); !errors.Is(e, ErrHeld) {
				t.Fatal("non-socket channel accepted")
			}
			now, e := r.Current.Lstat("current.sock")
			if e != nil || !os.SameFile(now, old) {
				t.Fatal("non-socket channel was unlinked")
			}
		})
	}
	t.Run("changed-gate", func(t *testing.T) {
		c, r := newChannelController(t)
		old := retainedSocket(t, r.Current)
		if e := r.Private.Rename("live.lock", "prior.lock"); e != nil {
			t.Fatal(e)
		}
		f, e := r.Private.OpenFile("live.lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			t.Fatal(e)
		}
		f.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if !errors.Is(c.ServeCurrent(ctx, CurrentChannel{}), ErrHeld) {
			t.Fatal("changed gate accepted channel")
		}
		now, e := r.Current.Lstat("current.sock")
		if e != nil || !os.SameFile(now, old) {
			t.Fatal("changed gate unlinked retained channel")
		}
	})
	t.Run("active-owner", func(t *testing.T) {
		f := newAuthorityFixture(t, nil)
		old, e := f.resources.Current.Lstat("current.sock")
		if e != nil {
			t.Fatal(e)
		}
		other, e := NewController(f.config, f.resources, f.clock)
		if other != nil {
			other.Close()
		}
		if e == nil {
			t.Fatal("active producer lock was bypassed")
		}
		now, e := f.resources.Current.Lstat("current.sock")
		if e != nil || !os.SameFile(now, old) {
			t.Fatal("active producer channel changed")
		}
		r, e := f.verifier.Authorize(context.Background(), f.grant, f.config.Launches[0].Scope.Scope)
		if e != nil || f.verifier.CheckRelease(context.Background(), r) != nil {
			t.Fatal("active producer disturbed by refused owner")
		}
	})
	t.Run("changed-root", func(t *testing.T) {
		c, r := newChannelController(t)
		old := retainedSocket(t, r.Current)
		path := r.Current.Name()
		if e := os.Rename(path, path+"-old"); e != nil {
			t.Fatal(e)
		}
		if e := os.Mkdir(path, 0700); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if !errors.Is(c.ServeCurrent(ctx, CurrentChannel{}), ErrHeld) {
			t.Fatal("changed root accepted channel")
		}
		now, e := r.Current.Lstat("current.sock")
		if e != nil || !os.SameFile(now, old) {
			t.Fatal("changed root unlinked retained channel")
		}
		if _, e = os.Lstat(filepath.Join(path, "current.sock")); !os.IsNotExist(e) {
			t.Fatal("changed root received a channel")
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		f := newAuthorityFixture(t, nil)
		old, e := f.resources.Current.Lstat("current.sock")
		if e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if !errors.Is(f.controller.ServeCurrent(ctx, CurrentChannel{}), ErrHeld) {
			t.Fatal("duplicate current listener accepted")
		}
		now, e := f.resources.Current.Lstat("current.sock")
		if e != nil || !os.SameFile(now, old) {
			t.Fatal("duplicate startup unlinked active listener")
		}
	})
}

func TestForgeLocalAuthorityControlCapacity(t *testing.T) {
	f := newAuthorityFixture(t, nil)
	var connections []net.Conn
	t.Cleanup(func() {
		for _, conn := range connections {
			conn.Close()
		}
	})
	for range f.config.MaxConnections {
		conn, e := net.DialTimeout("unix", filepath.Join(f.resources.Current.Name(), "current.sock"), time.Second)
		if e != nil {
			t.Fatal(e)
		}
		connections = append(connections, conn)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for len(f.controller.dataSlots) != f.config.MaxConnections {
		if time.Now().After(deadline) {
			t.Fatal("actual slow data slots not occupied")
		}
		time.Sleep(time.Millisecond)
	}
	claims, e := decodeClaims(f.grant, f.config.MaxFrameBytes)
	if e != nil {
		t.Fatal(e)
	}
	r, e := RequestControl(context.Background(), filepath.Join(f.resources.Private.Name(), "control.sock"), f.config.MaxFrameBytes, f.config.IOTimeout, &OperatorCommand{Operation: Revoke, ExpectedVersion: f.version, GrantID: claims.GrantID})
	if e != nil || r.Version != f.version+1 {
		t.Fatal("slow data slots blocked private revocation")
	}
	for _, conn := range connections {
		conn.Close()
	}
	read, e := f.verifier.Authorize(context.Background(), f.grant, claims.Scope.Scope)
	assertHeld(t, read, e)
}
