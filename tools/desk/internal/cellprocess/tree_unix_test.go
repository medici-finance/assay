//go:build unix

package cellprocess

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func TestInteractiveTerminalFixture(t *testing.T) {
	if os.Getenv("CELLPROCESS_TERMINAL_FIXTURE") != "1" {
		return
	}
	foreground, err := unix.IoctlGetInt(int(os.Stdin.Fd()), unix.TIOCGPGRP)
	if err != nil || foreground != syscall.Getpgrp() {
		os.Exit(97)
	}
	os.Exit(0)
}

// Run the compiled test binary directly in a terminal to exercise foreground
// handoff; ordinary nonterminal CI still covers inherited file stdin separately.
func TestInteractiveTerminalHandback(t *testing.T) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		t.Skip("requires a controlling terminal")
	}
	before, err := unix.IoctlGetInt(int(os.Stdin.Fd()), unix.TIOCGPGRP)
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	code, uncertain, err := RunInteractive(context.Background(), []string{exe, "-test.run=^TestInteractiveTerminalFixture$"}, append(os.Environ(), "CELLPROCESS_TERMINAL_FIXTURE=1"), t.TempDir(), os.Stdin, os.Stdout, os.Stderr)
	if code != 0 || uncertain || err != nil {
		t.Fatalf("foreground child: %d %v %v", code, uncertain, err)
	}
	after, err := unix.IoctlGetInt(int(os.Stdin.Fd()), unix.TIOCGPGRP)
	if err != nil || before != after {
		t.Fatalf("terminal not restored: before=%d after=%d error=%v", before, after, err)
	}
	// A failed exec must restore the foreground group too.
	_, _, err = RunInteractive(context.Background(), []string{"/cellprocess-fixture-no-such-executable"}, nil, t.TempDir(), os.Stdin, os.Stdout, os.Stderr)
	if !errors.Is(err, exec.ErrNotFound) && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected failed exec: %v", err)
	}
	after, err = unix.IoctlGetInt(int(os.Stdin.Fd()), unix.TIOCGPGRP)
	if err != nil || before != after {
		t.Fatalf("failed exec terminal not restored: %d %d %v", before, after, err)
	}
}

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
