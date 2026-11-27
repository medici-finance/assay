package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// capturePipe points *target (os.Stdout or os.Stderr) at a pipe for the duration of fn
// and returns everything fn wrote there. The reader runs in its own goroutine, started
// BEFORE fn, so a write larger than the pipe buffer never blocks: the buffer is 64 KiB on
// most hosts and can be as small as 8 KiB in a container, and a lint run that printed past
// it under a read-after-return helper once hung `go test ./statusgen/...` until the 10m
// default timeout fired.
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
		// Restore and close even when fn fails the test (t.Fatal exits via Goexit), so
		// the reader always sees EOF and the real stream comes back.
		defer func() {
			*target = old
			_ = w.Close()
		}()
		fn()
	}()
	return string(<-read)
}

// captureStdout runs fn with os.Stdout redirected and returns what it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	return capturePipe(t, &os.Stdout, fn)
}

// captureStderr runs fn with os.Stderr redirected and returns what it wrote. NOTICE
// lines (emitNotices) go to stderr, so the lint-integration tests need this to observe
// them.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	return capturePipe(t, &os.Stderr, fn)
}

// captureRun runs fn with stdout and stderr both redirected, returning each plus the
// exit code fn produced.
func captureRun(t *testing.T, fn func() int) capturedRun {
	t.Helper()
	var run capturedRun
	run.log = capturePipe(t, &os.Stdout, func() {
		run.err = capturePipe(t, &os.Stderr, func() { run.code = fn() })
	})
	return run
}

// pipeOverflow is larger than any pipe buffer a host or container hands out, so a
// capture that reads only after fn returns cannot hold it.
const pipeOverflow = 1 << 20

// TestCaptureDrainsWhileRunning pins that the capture helpers read the pipe while fn
// runs. A helper that reads only after fn returns deadlocks once fn writes more than
// the pipe buffer: the write blocks, fn never returns, and the test binary hangs until
// `go test -timeout` panics.
//
// regression: #7771
func TestCaptureDrainsWhileRunning(t *testing.T) {
	payload := strings.Repeat("x", pipeOverflow)
	for _, c := range []struct {
		name    string
		capture func() string
	}{
		{"stdout", func() string {
			return captureStdout(t, func() { _, _ = os.Stdout.WriteString(payload) })
		}},
		{"stderr", func() string {
			return captureStderr(t, func() { _, _ = os.Stderr.WriteString(payload) })
		}},
		{"run", func() string {
			got := captureRun(t, func() int {
				_, _ = os.Stdout.WriteString(payload)
				_, _ = os.Stderr.WriteString(payload)
				return 0
			})
			if len(got.err) != pipeOverflow {
				return got.err
			}
			return got.log
		}},
	} {
		done := make(chan string, 1)
		go func() { done <- c.capture() }()
		select {
		case got := <-done:
			if len(got) != pipeOverflow {
				t.Fatalf("%s: captured %d bytes, want %d", c.name, len(got), pipeOverflow)
			}
		case <-time.After(20 * time.Second):
			t.Fatalf("%s capture deadlocked: %d bytes into a pipe nobody reads", c.name, pipeOverflow)
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

// TestOneCapturePipeHelper keeps capturePipe the only capture pipe in this package's
// tests, so a new helper cannot bring back the read-after-return deadlock. The planted
// fixture is the positive control: if the matcher stops matching, this test fails
// instead of passing.
//
// regression: #7771
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
		t.Fatalf("test files open their own capture pipe %v; use capturePipe, which drains while fn runs", got)
	}
}
