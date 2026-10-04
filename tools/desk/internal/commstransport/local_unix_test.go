//go:build unix

package commstransport

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// privateDir returns a short owner-only directory (socket paths are capped near
// 104 bytes on macOS, which t.TempDir names can exceed).
func privateDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "ct-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0700); os.RemoveAll(dir) })
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestListenRefusesSharedParent(t *testing.T) {
	for _, mode := range []os.FileMode{0750, 0705, 0755} {
		dir := privateDir(t)
		if err := os.Chmod(dir, mode); err != nil {
			t.Fatal(err)
		}
		address := filepath.Join(dir, "gw.sock")
		if ln, err := Listen(address); err == nil {
			ln.Close()
			t.Fatalf("Listen accepted a parent with mode %04o", mode)
		}
		if _, err := os.Lstat(address); err == nil {
			t.Fatalf("Listen created a socket under a mode %04o parent", mode)
		}
	}
}

func TestListenSocketIsOwnerOnly(t *testing.T) {
	address := filepath.Join(privateDir(t), "gw.sock")
	ln, err := Listen(address)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	fi, err := os.Lstat(address)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0600 {
		t.Fatalf("socket mode %04o, want 0600", perm)
	}
}

func TestDialRefusesUnprotectedEndpoint(t *testing.T) {
	dir := privateDir(t)
	address := filepath.Join(dir, "gw.sock")
	ln, err := Listen(address)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	c, err := Dial(address, time.Second)
	if err != nil {
		t.Fatalf("positive control: protected endpoint refused: %v", err)
	}
	c.Close()
	// A parent another account can write lets that account replace the socket.
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatal(err)
	}
	if c, err := Dial(address, time.Second); err == nil {
		c.Close()
		t.Fatal("Dial accepted an endpoint whose parent is not owner-only")
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	// A regular file in place of the socket is never dialled.
	plain := filepath.Join(dir, "plain")
	if err := os.WriteFile(plain, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if c, err := Dial(plain, time.Second); err == nil {
		c.Close()
		t.Fatal("Dial accepted a non-socket endpoint")
	}
}

func TestRemoveStaleOnlyReclaimsDeadSocket(t *testing.T) {
	dir := privateDir(t)

	// Stale: a listener that exited without unlinking.
	stale := filepath.Join(dir, "stale.sock")
	ln, err := net.Listen("unix", stale)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close()
	if err := RemoveStale(stale); err != nil {
		t.Fatalf("stale socket not reclaimed: %v", err)
	}
	if _, err := os.Lstat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale socket still present: %v", err)
	}

	// Live: a serving gateway is never unlinked.
	live := filepath.Join(dir, "live.sock")
	liveLn, err := Listen(live)
	if err != nil {
		t.Fatal(err)
	}
	defer liveLn.Close()
	go func() {
		for {
			c, err := liveLn.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	if err := RemoveStale(live); err == nil {
		t.Fatal("RemoveStale unlinked a live gateway socket")
	}
	if _, err := os.Lstat(live); err != nil {
		t.Fatalf("live socket removed: %v", err)
	}

	// Not a socket: refused, left in place.
	plain := filepath.Join(dir, "plain")
	if err := os.WriteFile(plain, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := RemoveStale(plain); err == nil {
		t.Fatal("RemoveStale accepted a regular file")
	}
	if _, err := os.Lstat(plain); err != nil {
		t.Fatalf("regular file removed: %v", err)
	}

	// Absent: nothing to do.
	if err := RemoveStale(filepath.Join(dir, "absent.sock")); err != nil {
		t.Fatalf("absent endpoint: %v", err)
	}
}
