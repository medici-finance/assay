//go:build unix

package cellprocess

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestRunCleansDescendants(t *testing.T) {
	for _, mode := range []string{"spawn-exit", "spawn-wait"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			pidPath, heartPath := filepath.Join(dir, "child.pid"), filepath.Join(dir, "heartbeat")
			argv, env := fixture(t, mode, pidPath, heartPath)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			type result struct {
				code      int
				uncertain bool
				err       error
			}
			done := make(chan result, 1)
			go func() {
				code, uncertain, err := Run(ctx, argv, env, dir, io.Discard, io.Discard)
				done <- result{code, uncertain, err}
			}()
			until := time.Now().Add(5 * time.Second)
			var pid int
			for pid == 0 {
				if raw, err := os.ReadFile(pidPath); err == nil {
					pid, _ = strconv.Atoi(string(raw))
				}
				if time.Now().After(until) {
					cancel()
					t.Fatal("fixture descendant never started")
				}
				time.Sleep(5 * time.Millisecond)
			}
			// Last-resort fixture cleanup even if the tested supervisor regresses.
			t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
			if mode == "spawn-wait" {
				cancel()
			}
			select {
			case got := <-done:
				if got.uncertain {
					t.Fatalf("cleanup uncertain: %v", got.err)
				}
				if mode == "spawn-wait" && !errors.Is(got.err, context.Canceled) {
					t.Fatalf("cancellation lost: %+v", got)
				}
				if mode == "spawn-exit" && got.code != 0 {
					t.Fatalf("child exit changed: %+v", got)
				}
			case <-time.After(6 * time.Second):
				t.Fatal("bounded supervisor did not return")
			}
			if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
				t.Fatalf("descendant remains observable: %v", err)
			}
			before, _ := os.ReadFile(heartPath)
			time.Sleep(100 * time.Millisecond)
			after, _ := os.ReadFile(heartPath)
			if len(after) != len(before) {
				t.Fatal("descendant continued writing after Run")
			}
		})
	}
}
