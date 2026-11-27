//go:build !windows

package socktest

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Under a TMPDIR too long for any socket the directory still binds (via /tmp) or the
// test skips as could-not-check; it never hands back a path that cannot bind.
//
// regression: #7835
func TestDirBindsUnderLongTmpdir(t *testing.T) {
	long := filepath.Join(t.TempDir(), strings.Repeat("l", syscall.SizeofSockaddrUnix))
	if err := os.MkdirAll(long, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", long)
	dir := Dir(t, "sk", "gw.sock")
	ln, err := net.Listen("unix", filepath.Join(dir, "gw.sock"))
	if err != nil {
		t.Fatalf("Dir returned a directory that cannot bind a socket: %v", err)
	}
	ln.Close()
	if strings.HasPrefix(dir, long) {
		t.Fatalf("Dir stayed under the over-long TMPDIR: %s", dir)
	}
}
