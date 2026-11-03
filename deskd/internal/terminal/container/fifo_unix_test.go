//go:build unix

package container

import (
	"errors"
	"os"
	"syscall"
	"testing"
	"time"
)

// makeFIFO creates a named pipe at path.
func makeFIFO(t *testing.T, path string) {
	t.Helper()
	if err := syscall.Mkfifo(path, 0o644); err != nil {
		t.Fatalf("mkfifo %s: %v", path, err)
	}
}

// serveFIFO waits until a reader has the FIFO at path open, then writes
// content to it and closes it. The write happens only once the reader is
// there, so the reader sees exactly this content on this open: a handshake,
// not a timer. It gives up and returns false once exited is closed (the
// reader will never come) or after 10 s.
func serveFIFO(t *testing.T, path, content string, exited <-chan struct{}) bool {
	t.Helper()
	f := awaitFIFOReader(t, path, exited)
	if f == nil {
		return false
	}
	_, werr := f.WriteString(content)
	f.Close()
	if werr != nil {
		t.Fatalf("write %s: %v", path, werr)
	}
	return true
}

// awaitFIFOReader waits until a reader has the FIFO at path open and returns
// the write end, still empty: the reader is then blocked on it until the
// caller writes and closes. It returns nil once exited is closed (the reader
// will never come), and fails the test after 10 s.
func awaitFIFOReader(t *testing.T, path string, exited <-chan struct{}) *os.File {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		f, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err == nil {
			return f
		}
		if !errors.Is(err, syscall.ENXIO) {
			t.Fatalf("open %s for writing: %v", path, err)
		}
		select {
		case <-exited:
			return nil
		case <-deadline:
			t.Fatalf("no reader opened %s within 10s", path)
		case <-time.After(5 * time.Millisecond):
		}
	}
}
