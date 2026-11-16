package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// capturePipe points *target (os.Stdout or os.Stderr) at a pipe for the duration of fn and
// returns everything fn wrote there. The reader runs in its own goroutine, started BEFORE
// fn, so a write larger than the pipe buffer never blocks: the buffer is 64 KiB on most
// hosts but 8 KiB in a container whose user is over pipe-user-pages-soft, and a 9 KiB
// `prune --help` screen once hung the whole package there until `go test -timeout` fired.
//
// It is the ONE place this package's tests open a capture pipe; TestOneCapturePipeHelper
// refuses any other `os.Pipe()` in a _test.go file here.
func capturePipe(t testing.TB, target **os.File, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	read := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(r)
		_ = r.Close()
		read <- b
	}()
	old := *target
	*target = w
	func() {
		// Restore and close even when fn fails the test (t.Fatal exits via Goexit), so the
		// reader always sees EOF and the real stream comes back.
		defer func() {
			*target = old
			_ = w.Close()
		}()
		fn()
	}()
	return string(<-read)
}

// bigOutput is larger than any pipe buffer a host or container hands out, so a capture
// that reads only after the command returns cannot hold it.
const bigOutput = 1 << 20

// TestCaptureDrainsWhileRunning pins that the capture helpers read the pipe while the
// command runs. A helper that reads only after `run()` returns deadlocks once the command
// writes more than the pipe buffer: the write blocks, `run()` never returns, and the test
// binary hangs until `go test -timeout` panics.
//
// regression: #4179
func TestCaptureDrainsWhileRunning(t *testing.T) {
	payload := strings.Repeat("x", bigOutput)
	for _, c := range []struct {
		name    string
		capture func() string
	}{
		{"stdout", func() string {
			return runCapOut(t, func() { _, _ = os.Stdout.WriteString(payload) })
		}},
		{"stderr", func() string {
			return capturePipe(t, &os.Stderr, func() { _, _ = os.Stderr.WriteString(payload) })
		}},
	} {
		done := make(chan string, 1)
		go func() { done <- c.capture() }()
		select {
		case got := <-done:
			if len(got) != bigOutput {
				t.Fatalf("%s: captured %d bytes, want %d", c.name, len(got), bigOutput)
			}
		case <-time.After(20 * time.Second):
			t.Fatalf("%s capture deadlocked: %d bytes into a pipe nobody reads", c.name, bigOutput)
		}
	}
}

// rawPipeSites lists the files among paths that open an `os.Pipe()` themselves.
func rawPipeSites(t *testing.T, paths []string) []string {
	t.Helper()
	var hits []string
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if strings.Contains(string(b), "os.Pipe()") {
			hits = append(hits, filepath.Base(p))
		}
	}
	return hits
}

// TestOneCapturePipeHelper keeps capturePipe the only capture pipe in this package's tests,
// so a new helper cannot bring back the read-after-return deadlock. The planted fixture is
// the positive control: if the matcher stops matching, this test fails instead of passing.
//
// regression: #4179
func TestOneCapturePipeHelper(t *testing.T) {
	if got := rawPipeSites(t, []string{"testdata/raw-capture-pipe.go.txt"}); len(got) != 1 {
		t.Fatalf("guard missed the planted raw capture pipe: %v", got)
	}
	files, err := filepath.Glob("*_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("could not list this package's tests: %v (%d files)", err, len(files))
	}
	var others []string
	for _, f := range files {
		if f != "capture_test.go" {
			others = append(others, f)
		}
	}
	if got := rawPipeSites(t, others); len(got) != 0 {
		t.Fatalf("test files open their own capture pipe %v; use capturePipe, which drains while the command runs", got)
	}
}
