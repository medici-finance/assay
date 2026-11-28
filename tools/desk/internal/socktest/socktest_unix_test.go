//go:build unix

package socktest

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// noSkip turns a skip by Dir into a failure: where /tmp is usable, a skip would hide a
// lost fallback behind a passing run.
type noSkip struct{ testing.TB }

func (n noSkip) Skipf(format string, args ...any) {
	n.Helper()
	n.Fatalf("Dir skipped although /tmp is usable: "+format, args...)
}

// Under a TMPDIR too long for any socket, Dir returns a directory under /tmp that binds.
// The test is could-not-check only where /tmp itself cannot hold a socket.
//
// regression: #7835
func TestDirBindsUnderLongTmpdir(t *testing.T) {
	root, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		t.Skipf("could-not-check: no /tmp: %v", err)
	}
	probe, err := os.MkdirTemp(root, "sk")
	if err != nil {
		t.Skipf("could-not-check: /tmp is not writable: %v", err)
	}
	_ = os.Remove(probe)
	if len(filepath.Join(probe, "gw.sock")) >= len(syscall.RawSockaddrUnix{}.Path) {
		t.Skipf("could-not-check: /tmp is too long for a socket: %s", root)
	}
	long := filepath.Join(t.TempDir(), strings.Repeat("l", syscall.SizeofSockaddrUnix))
	if err := os.MkdirAll(long, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", long)
	dir := Dir(noSkip{t}, "sk", "gw.sock")
	ln, err := net.Listen("unix", filepath.Join(dir, "gw.sock"))
	if err != nil {
		t.Fatalf("Dir returned a directory that cannot bind a socket: %v", err)
	}
	_ = ln.Close()
	if filepath.Dir(dir) != root {
		t.Fatalf("Dir did not fall back to /tmp: %s", dir)
	}
}
