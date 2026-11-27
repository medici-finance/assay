//go:build unix

package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/socktest"
)

// After a certain stop the supervisor removes the socket its killed gateway
// could not close, so the next run can start. After an uncertain stop it keeps
// the socket for recover --confirm-stopped, and it never removes a non-socket.
// regression: #7835
func TestRemoveStoppedEndpoint(t *testing.T) {
	dir := socktest.Dir(t, "cce", "gw.sock")
	sock := filepath.Join(dir, "gw.sock")
	plant := func() {
		ln, err := net.Listen("unix", sock)
		if err != nil {
			t.Fatal(err)
		}
		ln.(*net.UnixListener).SetUnlinkOnClose(false)
		_ = ln.Close()
	}
	plant()
	removeStoppedEndpoint(cellcadence.Result{Uncertain: true}, sock)
	if _, err := os.Lstat(sock); err != nil {
		t.Fatalf("uncertain stop removed the socket: %v", err)
	}
	removeStoppedEndpoint(cellcadence.Result{ExitCode: 1}, sock)
	if _, err := os.Lstat(sock); !os.IsNotExist(err) {
		t.Fatalf("certain stop left the socket behind: %v", err)
	}
	plain := filepath.Join(dir, "plain")
	if err := os.WriteFile(plain, nil, 0600); err != nil {
		t.Fatal(err)
	}
	removeStoppedEndpoint(cellcadence.Result{}, plain)
	if _, err := os.Lstat(plain); err != nil {
		t.Fatalf("a non-socket endpoint was removed: %v", err)
	}
}
