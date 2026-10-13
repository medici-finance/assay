package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/commstransport"
)

// slowGateway listens on a fresh protected local endpoint and answers every
// request with reply, but only after delay — a stand-in for a gateway whose
// prose gate consults its advisor synchronously before it replies.
func slowGateway(t *testing.T, delay time.Duration, reply string) string {
	t.Helper()
	// A short path: Unix socket paths are capped at ~104 bytes on macOS, and
	// t.TempDir() names can exceed that.
	dir, err := os.MkdirTemp("", "dcg")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "gw.sock")
	if runtime.GOOS == "windows" {
		path = fmt.Sprintf(`\\.\pipe\assay-dcg-test-%d-%d`, os.Getpid(), time.Now().UnixNano())
	}
	ln, err := commstransport.Listen(path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				if !bufio.NewScanner(c).Scan() {
					return
				}
				time.Sleep(delay)
				_, _ = c.Write([]byte(reply + "\n"))
			}(conn)
		}
	}()
	return path
}

// A gateway that accepts a submission only after a gate consult longer than the
// dial budget must still be reported as accepted: the client's exit code has to
// agree with the gateway's state, never report could-not-submit for a message
// the gateway accepted.
//
// regression: #2726
func TestSubmitOutlastsSlowGate(t *testing.T) {
	path := slowGateway(t, dialTimeout+time.Second, `{"receipt":{"id":"m-1","accepted":true}}`)
	rc, err := socketGateway{addr: path}.Submit([]byte(`{}`))
	if err != nil {
		t.Fatalf("Submit through a slow-but-accepting gateway = %v, want the gateway's receipt", err)
	}
	if !rc.Accepted || rc.ID != "m-1" {
		t.Fatalf("receipt = %+v, want accepted m-1", rc)
	}
}

