package container

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// detachFakeDocker puts a `docker` on PATH that stands in for the engine, and
// returns the file every invocation's argv is appended to (one line each).
// No real container engine is touched.
//
//   - `docker inspect …` prints true (the container is running).
//   - `docker exec -i -t …` is deskd's interactive exec. It prints the
//     in-container pid line the real attach command prints (4242), then a
//     marker, then either stays up until the pty hangs up (mode "live") or
//     exits at once (mode "ends"). Mode "slow" is "live" with the pid line
//     held back 300 ms, the window in which a Close can arrive before the
//     session has read it.
//   - any other `docker exec …` (the detach call) is recorded and exits 0,
//     or 2 in mode "detach-fails" (what bash 5.x exits with when the
//     container's environment exports an `unset` function).
func detachFakeDocker(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\n" +
		"echo \"$*\" >> '" + log + "'\n" +
		"case \"$1\" in\n" +
		"inspect) echo true; exit 0 ;;\n" +
		"esac\n" +
		"if [ \"$1\" = exec ] && [ \"$2\" = -i ]; then\n" +
		"  if [ '" + mode + "' = slow ]; then sleep 0.3; fi\n" +
		"  printf '4242\\r\\nfake-shell-ready\\r\\n'\n" +
		"  if [ '" + mode + "' = ends ]; then exit 0; fi\n" +
		"  exec sleep 30\n" +
		"fi\n" +
		"if [ '" + mode + "' = detach-fails ]; then exit 2; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

// readUntil drains sess until want appears or the channel closes, and returns
// everything read.
func readUntil(t *testing.T, sess terminal.Session, want string, timeout time.Duration) []byte {
	t.Helper()
	var got []byte
	deadline := time.After(timeout)
	for {
		select {
		case b, ok := <-sess.Read():
			got = append(got, b...)
			if !ok || bytes.Contains(got, []byte(want)) {
				return got
			}
		case <-deadline:
			t.Fatalf("session never printed %q within %s; got %q", want, timeout, got)
		}
	}
}

// TestDetachEndsTheExecShell (the service contract review finding F-cor-1): the
// container engine does NOT end an exec'd process when its client goes away,
// so closing the host pty alone leaves deskd's `bash -l` running inside the
// desk. Close must end that shell itself: it signals the in-container pid the
// attach command reported, through a second `docker exec` on the same
// container. The pid line is deskd's own bookkeeping and never reaches the
// scrollback.
func TestDetachEndsTheExecShell(t *testing.T) {
	log := detachFakeDocker(t, "live")
	spec := &terminal.TargetSpec{Kind: "container", Container: "c-review"}
	sess, err := NewDriver().Attach(context.Background(), spec, terminal.Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	out := readUntil(t, sess, "fake-shell-ready", 10*time.Second)
	if err := sess.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	calls, _ := os.ReadFile(log)
	var detach string
	for _, line := range strings.Split(string(calls), "\n") {
		if strings.HasPrefix(line, "exec c-review ") && !strings.HasPrefix(line, "exec -i") {
			detach = line
		}
	}
	if detach == "" || !strings.HasSuffix(detach, " deskd-detach 4242") {
		t.Fatalf("Close did not end deskd's shell inside the container: no `docker exec c-review … deskd-detach 4242` call.\ncalls:\n%s", calls)
	}
	if bytes.Contains(out, []byte("4242")) {
		t.Fatalf("the attach command's pid line leaked into the scrollback: %q", out)
	}
}

// TestDetachBeforeThePIDLineStillEndsTheShell (the service contract review
// advisory A-cor-detach-pid-race): the supervisor detaches straight after
// Attach returns when Stop races an in-flight attempt, so Close can arrive
// before the session has read the pid line. Close must wait, within its
// detach bound, for that line (or for the exec to end) instead of giving up
// on an unknown pid, which would leak the shell F-cor-1 is about.
func TestDetachBeforeThePIDLineStillEndsTheShell(t *testing.T) {
	log := detachFakeDocker(t, "slow")
	spec := &terminal.TargetSpec{Kind: "container", Container: "c-review"}
	sess, err := NewDriver().Attach(context.Background(), spec, terminal.Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	start := time.Now()
	if err := sess.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if took := time.Since(start); took > detachTimeout {
		t.Fatalf("Close took %s, over the %s detach bound", took, detachTimeout)
	}
	calls, _ := os.ReadFile(log)
	if !strings.Contains(string(calls), " deskd-detach 4242") {
		t.Fatalf("Close before the pid line was read signalled nothing: deskd's shell leaks.\ncalls:\n%s", calls)
	}
}

// TestDetachAfterTheShellEndedSignalsNothing is the negative path: when the
// exec already ended by itself (the shell exited, the container stopped), the
// pid it reported may belong to another process by now, so Close must not
// signal it.
func TestDetachAfterTheShellEndedSignalsNothing(t *testing.T) {
	log := detachFakeDocker(t, "ends")
	spec := &terminal.TargetSpec{Kind: "container", Container: "c-review"}
	sess, err := NewDriver().Attach(context.Background(), spec, terminal.Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	deadline := time.After(10 * time.Second)
	for open := true; open; {
		select {
		case _, open = <-sess.Read():
		case <-deadline:
			t.Fatal("the fake exec never ended")
		}
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "deskd-detach") {
		t.Fatalf("Close signalled a shell that had already ended:\n%s", calls)
	}
}

// TestContainerDetachLeavesNoExecShell is the live half of F-cor-1 (brief
// Verify row 13's shape, against the named fixture): attach through the real
// driver, see ONE more `bash -l` in the container (counted host-side with
// `docker top`, end-anchored, the same instrument row 13 uses), Close, and
// see it gone. Skips without COCKPIT_VERIFY_CONTAINER, so CI never needs a
// daemon.
func TestContainerDetachLeavesNoExecShell(t *testing.T) {
	name := testContainerName(t)
	count := func() int {
		out, err := exec.Command("docker", "top", name, "-o", "pid,args").Output()
		if err != nil {
			t.Fatalf("docker top %s: %v", name, err)
		}
		n := 0
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasSuffix(strings.TrimRight(line, " \r"), "bash -l") {
				n++
			}
		}
		return n
	}
	waitCount := func(want int, within time.Duration) int {
		deadline := time.Now().Add(within)
		for {
			n := count()
			if n == want || time.Now().After(deadline) {
				return n
			}
			time.Sleep(250 * time.Millisecond)
		}
	}

	before := count()
	sess, err := NewDriver().Attach(context.Background(), &terminal.TargetSpec{Kind: "container", Container: name}, terminal.Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if n := waitCount(before+1, 20*time.Second); n != before+1 {
		_ = sess.Close()
		t.Fatalf("after attach: %d `bash -l` in %s, want %d", n, name, before+1)
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if n := waitCount(before, 6*time.Second); n != before {
		t.Fatalf("6s after Close: %d `bash -l` in %s, want %d — deskd's exec shell leaked", n, name, before)
	}
}

// TestDetachFailureIsReported (review advisory A-cor-unset-import-abort): a
// detach exec that fails leaves deskd's shell running inside the container,
// because the signal was never sent. One known cause is a container
// environment that exports an `unset` function: bash 5.x then aborts with
// rc 2 before the script runs. Close must say so, naming the container and
// the pid, so the supervisor can log it; a silent nil reads as a clean
// detach.
func TestDetachFailureIsReported(t *testing.T) {
	log := detachFakeDocker(t, "detach-fails")
	spec := &terminal.TargetSpec{Kind: "container", Container: "c-review"}
	sess, err := NewDriver().Attach(context.Background(), spec, terminal.Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	readUntil(t, sess, "fake-shell-ready", 10*time.Second)
	err = sess.Close()
	calls, _ := os.ReadFile(log)
	if !strings.Contains(string(calls), " deskd-detach 4242") {
		t.Fatalf("could-not-check: Close made no detach exec.\ncalls:\n%s", calls)
	}
	if err == nil {
		t.Fatal("Close returned nil although the detach exec failed — deskd's shell may still be running and nothing says so")
	}
	for _, want := range []string{"c-review", "4242", "may still be running"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Close error %q does not name %q", err, want)
		}
	}
}
