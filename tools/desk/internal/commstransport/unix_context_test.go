//go:build unix

package commstransport

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestDialUnixContextOwnershipAndCancellation(t *testing.T) {
	dir := privateDir(t)
	address := filepath.Join(dir, "context.sock")
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
	conn, err := DialUnixContext(context.Background(), address, time.Second)
	if err != nil {
		t.Fatalf("actual checked Unix dial refused: %v", err)
	}
	conn.Close()
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if conn, err := DialUnixContext(context.Background(), address, time.Second); err == nil {
		conn.Close()
		t.Fatal("context dial accepted a shared endpoint parent")
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(dir, "plain")
	if err := os.WriteFile(plain, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if conn, err := DialUnixContext(context.Background(), plain, time.Second); err == nil {
		conn.Close()
		t.Fatal("context dial accepted a nonsocket endpoint")
	}
	for _, timeout := range []time.Duration{0, -time.Second} {
		conn, err := DialUnixContext(context.Background(), address, timeout)
		if conn != nil {
			conn.Close()
		}
		if conn != nil || err == nil || !strings.Contains(err.Error(), "positive timeout") {
			t.Fatalf("valid endpoint did not refuse nonpositive timeout %s: %v", timeout, err)
		}
	}
	missing := filepath.Join(dir, "missing", "endpoint")
	if conn, err := DialUnixContext(nil, missing, time.Second); err == nil || conn != nil {
		t.Fatal("context dial admitted a missing context")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if conn, err := DialUnixContext(ctx, missing, time.Second); !errors.Is(err, context.Canceled) || conn != nil {
		t.Fatalf("cancellation was not checked before endpoint I/O: %v", err)
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if conn, err := DialUnixContext(ctx, missing, time.Second); !errors.Is(err, context.DeadlineExceeded) || conn != nil {
		t.Fatalf("expired deadline was not checked before endpoint I/O: %v", err)
	}
}

func TestDialUnixContextRefusesForeignSocketOwner(t *testing.T) {
	address := filepath.Join(privateDir(t), "foreign.sock")
	listener, err := Listen(address)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	// The private parent still restricts access to the fixture process. Let
	// kernel permissions admit the dial so socket ownership alone decides it.
	if err := os.Chmod(address, 0666); err != nil {
		t.Fatal(err)
	}
	foreignUID := 65534
	if os.Getuid() == foreignUID {
		foreignUID = 0
	}
	if err := os.Chown(address, foreignUID, -1); err != nil {
		if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.ENOTSUP) {
			t.Skipf("disposable fixture cannot assign a second socket UID: %v", err)
		}
		t.Fatal(err)
	}
	info, err := os.Lstat(address)
	if err != nil {
		t.Fatal(err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != foreignUID {
		t.Fatal("fixture did not establish the foreign socket owner")
	}
	raw, err := net.DialTimeout("unix", address, time.Second)
	if err != nil {
		t.Fatalf("kernel refused the disposable socket: %v", err)
	}
	raw.Close()
	conn, err := DialUnixContext(context.Background(), address, time.Second)
	if conn != nil {
		conn.Close()
	}
	if conn != nil || err == nil || !strings.Contains(err.Error(), "not owned by the current user") {
		t.Fatalf("context dial did not refuse the foreign socket owner: %v", err)
	}
}
