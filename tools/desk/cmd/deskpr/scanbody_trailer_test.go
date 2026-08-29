package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Tests for the issue-loop scan-carrier trailer exemption (assay-toolkit#1604). The
// machine-derived scan body carries deskkit.ScanBodyMarker and structurally cannot name a
// single Brief:/Issue: work item, so it is the one body exempt from the trailer gate — while
// every human-authored body still faces the full gate (TestCreateRefusesNoTrailer covers the
// unchanged refusal). Kept in their own file so the diff carries only this new content.

// TestRequireTrailerExemptsScanBodyMarker pins the carve-out at the unit boundary: a body
// carrying the marker is exempt with no trailer, while an ordinary trailer-less body still
// refuses (exit 5). root/dir are irrelevant on both branches — the marker returns before any
// brief resolution and the trailer-less body refuses before it — so empty strings exercise
// the gate in isolation.
func TestRequireTrailerExemptsScanBodyMarker(t *testing.T) {
	// (a) exempt: a derived scan-carrier body passes with no trailer.
	scanBody := []byte(deskkit.ScanBodyMarker + "\nAutomated inbound scan. This body is DERIVED.\n\n- **created:** 78\n- **retired:** 128\n")
	if err := requireTrailer(scanBody, "", ""); err != nil {
		t.Fatalf("scanbody-marker body must be exempt from the trailer gate; got: %v", err)
	}

	// (b) unchanged: an ordinary body with no trailer still refuses (exit 5), naming the line.
	if err := requireTrailer([]byte("just a normal PR body, no trailer\n"), "", ""); err == nil {
		t.Fatal("trailer-less non-marker body must still refuse")
	} else if !deskkit.IsRefused(err) {
		t.Fatalf("trailer-less non-marker body err = %v, want exit-5 refusal", err)
	} else if !strings.Contains(err.Error(), "Brief: <stream>/<NN>") {
		t.Fatalf("refusal must still name the missing line; got: %v", err)
	}
}

// TestCreateExemptsScanBodyMarker is the end-to-end half: `deskpr create` on a scan-carrier
// body (marker present, NO trailer) must reach `gh pr create --draft` rather than refusing at
// the gate — the intake placeholder lane the missing exemption blocked.
func TestCreateExemptsScanBodyMarker(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)

	rc := run([]string{"create", "--title", "chore(issue-loop): scan — 78 created, 128 retired",
		"--body-min", deskkit.ScanBodyMarker + "\nAutomated inbound scan — derived body, no trailer"})
	if rc != deskkit.ExitOK {
		t.Fatalf("scanbody-marker create rc = %d, want 0 (gate exempt, PR created)", rc)
	}
	if !anyCall(ghCalls(*calls), "pr", "create", "--draft") {
		t.Fatalf("expected `gh pr create --draft` for an exempt scan-carrier body; gh calls: %v", ghCalls(*calls))
	}
}
