//go:build unix

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/commstransport"
)

// shortPrivateDir keeps socket paths under the ~104-byte macOS cap.
func shortPrivateDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "gwl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func waitForEndpoint(t *testing.T, path string, exited <-chan error) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-exited:
			t.Fatalf("socket server exited before serving: %v", err)
		default:
		}
		if c, err := commstransport.Dial(path, 100*time.Millisecond); err == nil {
			_ = c.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("socket server never became reachable at %s", path)
}

func serveInBackground(t *testing.T, s SocketServer, path string) (context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.ListenAndServeContext(ctx, path) }()
	t.Cleanup(cancel)
	return cancel, done
}

func waitStopped(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("clean stop returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("socket server did not stop when its context ended")
	}
}

// A clean stop must remove the socket so the same gateway can start again, in
// both modes.
func TestSocketRestartAfterCleanStop(t *testing.T) {
	for _, local := range []bool{false, true} {
		path := filepath.Join(shortPrivateDir(t), "gw.sock")
		for round := 0; round < 2; round++ {
			s := newBypassGateway(t).server()
			s.Cell, s.LocalOnly = "cell-a", local
			cancel, done := serveInBackground(t, s, path)
			waitForEndpoint(t, path, done)
			cancel()
			waitStopped(t, done)
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatalf("local-only=%v round %d: clean stop left the socket behind (%v)", local, round, err)
			}
		}
	}
}

// A socket left by a crashed run: a standalone gateway reclaims it once a dial
// is refused; a local-only gateway leaves it for cellctl's operator recovery.
func TestStaleSocketReclaim(t *testing.T) {
	plantStale := func(t *testing.T) string {
		path := filepath.Join(shortPrivateDir(t), "gw.sock")
		ln, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		ln.(*net.UnixListener).SetUnlinkOnClose(false)
		_ = ln.Close()
		return path
	}

	t.Run("standalone reclaims", func(t *testing.T) {
		path := plantStale(t)
		s := newBypassGateway(t).server()
		cancel, done := serveInBackground(t, s, path)
		waitForEndpoint(t, path, done)
		cancel()
		waitStopped(t, done)
	})

	t.Run("local-only refuses", func(t *testing.T) {
		path := plantStale(t)
		s := newBypassGateway(t).server()
		s.Cell, s.LocalOnly = "cell-a", true
		_, done := serveInBackground(t, s, path)
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("local-only gateway returned without error over a stale socket")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("local-only gateway bound over an existing endpoint")
		}
	})

	t.Run("standalone never takes a live socket", func(t *testing.T) {
		path := filepath.Join(shortPrivateDir(t), "gw.sock")
		first := newBypassGateway(t).server()
		_, firstDone := serveInBackground(t, first, path)
		waitForEndpoint(t, path, firstDone)
		second := newBypassGateway(t).server()
		_, done := serveInBackground(t, second, path)
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("second gateway returned without error")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("second gateway took a live socket")
		}
		waitForEndpoint(t, path, firstDone)
	})
}

// Local-only mode serves the socket and nothing else: a listen address left in
// the environment must not start the A2A listener. The second round proves a
// clean stop leaves nothing that blocks the next start.
func TestLocalOnlyRunServesSocketOnly(t *testing.T) {
	dir := shortPrivateDir(t)
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	trust := filepath.Join(dir, "trust.json")
	if err := os.WriteFile(trust, []byte(`{"cell-a":"`+base64.StdEncoding.EncodeToString(pub)+`"}`), 0600); err != nil {
		t.Fatal(err)
	}
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listen := probe.Addr().String()
	_ = probe.Close()
	sock := filepath.Join(dir, "gw.sock")
	env := map[string]string{
		EnvEnable: "1", EnvCell: "cell-a", EnvQueueDir: filepath.Join(dir, "queue"), EnvSocket: sock,
		EnvTrustStore: trust, "ASSAY_COMMS_LOCAL_ONLY": "1", EnvListen: listen,
	}
	getenv := func(k string) string { return env[k] }
	for round := 0; round < 2; round++ {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan int, 1)
		go func() { done <- runContext(ctx, getenv) }()
		deadline := time.Now().Add(5 * time.Second)
		for {
			select {
			case code := <-done:
				cancel()
				t.Fatalf("round %d: gateway exited before serving (code %d)", round, code)
			default:
			}
			if c, err := commstransport.Dial(sock, 100*time.Millisecond); err == nil {
				_ = c.Close()
				break
			}
			if time.Now().After(deadline) {
				cancel()
				t.Fatalf("round %d: socket never became reachable", round)
			}
			time.Sleep(20 * time.Millisecond)
		}
		time.Sleep(200 * time.Millisecond)
		select {
		case code := <-done:
			cancel()
			t.Fatalf("round %d: gateway exited while serving (code %d)", round, code)
		default:
		}
		if c, err := net.DialTimeout("tcp", listen, 200*time.Millisecond); err == nil {
			_ = c.Close()
			cancel()
			t.Fatalf("round %d: A2A listener started in local-only mode", round)
		}
		cancel()
		select {
		case code := <-done:
			if code != 0 {
				t.Fatalf("round %d: clean stop exit %d", round, code)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("round %d: gateway did not stop", round)
		}
		if _, err := os.Lstat(sock); !os.IsNotExist(err) {
			t.Fatalf("round %d: clean stop left the socket behind (%v)", round, err)
		}
	}
}
