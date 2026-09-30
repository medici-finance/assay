package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// remotecause_test.go — end to end through the live store: when the server refuses a claim
// write, the fail-closed attribution an operator reads carries what the server SAID, not only
// its one-word report-status. A local bare repo with a refusing pre-receive hook stands in for
// the forge; the hook's message travels on the receive-pack sideband.

const hookSays = "refusing this credential for refs/dispatch"

func refusingServer(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not on PATH")
	}
	server := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", "--bare", "-b", "main", server).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, out)
	}
	return server
}

func installHook(t *testing.T, server string) {
	t.Helper()
	body := "#!/bin/sh\necho '" + hookSays + "' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(server, "hooks", "pre-receive"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestRefusedCreateCauseCarriesServerMessage(t *testing.T) {
	server := refusingServer(t)
	installHook(t, server)
	g := &gogitStore{url: server, host: "local-test-server"}
	id := "at--issue-4243"
	if res := g.CreateIfAbsent(id, claimMessage(id, "sess-A", "claimed", "-", "")); res != deskkit.ClaimWriteRejected {
		t.Fatalf("create against a refusing server = %v, want Rejected", res)
	}
	if c := g.TransportCause(); !strings.Contains(c, hookSays) {
		t.Fatalf("TransportCause = %q, want it to carry the server's message %q", c, hookSays)
	}
}

func TestRefusedReleaseCauseCarriesServerMessage(t *testing.T) {
	server := refusingServer(t)
	g := &gogitStore{url: server, host: "local-test-server"}
	id := "at--issue-4244"
	if res := g.CreateIfAbsent(id, claimMessage(id, "sess-A", "claimed", "-", "")); res != deskkit.ClaimWriteApplied {
		t.Fatalf("seed create = %v, want Applied; cause=%q", res, g.TransportCause())
	}
	installHook(t, server)
	if out, _ := g.Remove(id); out != deskkit.ClaimWriteUnverifiable {
		t.Fatalf("release against a refusing server = %v, want Unverifiable", out)
	}
	if c := g.TransportCause(); !strings.Contains(c, hookSays) {
		t.Fatalf("TransportCause = %q, want it to carry the server's message %q", c, hookSays)
	}
}
