package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2972
func TestRosterPartialClaims(t *testing.T) {
	home := rosterSetup(t)
	writeTestBeacon(t, home, Beacon{Session: "healthy", Role: "worker-desk"})
	writeTestClaim(t, home, Claim{Brief: "good/01", Session: "reader"})
	bad := filepath.Join(home, ".config", "assay", "claims", "bad.claim")
	if err := os.WriteFile(bad, []byte(`{broken`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKEGH_LIST_PRS", "99:true:possibly owned")
	var err error
	out := captureStdout(t, func() { err = cmdList() })
	if !deskkit.IsUnverifiable(err) || !strings.Contains(err.Error(), "bad.claim") {
		t.Fatalf("claim blindness hidden: %v", err)
	}
	for _, want := range []string{"healthy", "good/01", "ownership unverified", "(unknown)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q from partial roster: %q", want, out)
		}
	}
	if strings.Contains(out, "unclaimed (no session registered)") || strings.Contains(out, "(none)") {
		t.Fatalf("partial ownership reported absent: %q", out)
	}
}
