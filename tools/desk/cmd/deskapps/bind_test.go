package main

import (
	"strings"
	"testing"
)

// TestBindLoopbackOnly — Verify row 7. The listener address is 127.0.0.1:<port>;
// "0.0.0.0" and "::" never appear, whichever bind path is taken.
func TestBindLoopbackOnly(t *testing.T) {
	t.Run("requested port free", func(t *testing.T) {
		ln, port, err := listenLoopback(0) // 0 asks the OS for any free port — exercises the "bind succeeds" path
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		assertLoopbackOnly(t, ln.Addr().String(), port)
	})

	t.Run("requested port busy falls back to a free one", func(t *testing.T) {
		holder, heldPort, err := listenLoopback(0)
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Close()

		ln, port, err := listenLoopback(heldPort)
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		if port == heldPort {
			t.Fatalf("fallback bound the SAME port that was already held: %d", port)
		}
		assertLoopbackOnly(t, ln.Addr().String(), port)
	})
}

func assertLoopbackOnly(t *testing.T, addr string, port int) {
	t.Helper()
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Fatalf("listener address %q does not start with 127.0.0.1:", addr)
	}
	if strings.Contains(addr, "0.0.0.0") {
		t.Fatalf("listener address %q contains 0.0.0.0", addr)
	}
	if strings.Contains(addr, "::") {
		t.Fatalf("listener address %q contains ::", addr)
	}
	if port <= 0 {
		t.Fatalf("bound port = %d, want > 0", port)
	}
}
