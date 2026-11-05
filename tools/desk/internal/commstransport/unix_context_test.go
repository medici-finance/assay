//go:build unix

package commstransport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
	missing := filepath.Join(dir, "missing", "endpoint")
	for _, timeout := range []time.Duration{0, -time.Second} {
		if conn, err := DialUnixContext(context.Background(), missing, timeout); err == nil || conn != nil {
			t.Fatal("context dial admitted a nonpositive timeout")
		}
	}
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
