package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Tests for the issue-loop scan-carrier trailer exemption. The machine-derived scan body
// carries deskkit.ScanBodyMarker at its head and structurally cannot name a single
// Brief:/Issue: work item, so it is the one body exempt from the trailer gate — while every
// human-authored body still faces the full gate (TestCreateRefusesNoTrailer covers the
// unchanged refusal). Kept in their own file so the diff carries only this new content.

// TestRequireTrailerExemptsScanBodyMarker pins the carve-out at the unit boundary: a body
// carrying the marker AT ITS HEAD is exempt with no trailer (returning issue number 0), an
// ordinary trailer-less body still refuses (exit 5), and a body that merely QUOTES the
// marker mid-prose is NOT exempt — the match is head-anchored, mirroring how the emitter
// (deskkit.ScanPRBody) writes it as the first line. root/dir are irrelevant on the refusing
// branches — the trailer-less body refuses before brief resolution — so empty strings
// exercise the gate in isolation.
func TestRequireTrailerExemptsScanBodyMarker(t *testing.T) {
	// (a) exempt: a derived scan-carrier body (marker at head) passes with no trailer,
	// reporting issue number 0 (a derived body names no reactions surface).
	scanBody := []byte(deskkit.ScanBodyMarker + "\nAutomated inbound scan. This body is DERIVED.\n\n- **created:** 78\n- **retired:** 128\n")
	if n, err := requireTrailer(scanBody, "", ""); err != nil {
		t.Fatalf("scanbody-marker body must be exempt from the trailer gate; got: %v", err)
	} else if n != 0 {
		t.Fatalf("exempt scan-carrier body must report issue 0; got %d", n)
	}

	// (b) unchanged: an ordinary body with no trailer still refuses (exit 5), naming the line.
	if _, err := requireTrailer([]byte("just a normal PR body, no trailer\n"), "", ""); err == nil {
		t.Fatal("trailer-less non-marker body must still refuse")
	} else if !deskkit.IsRefused(err) {
		t.Fatalf("trailer-less non-marker body err = %v, want exit-5 refusal", err)
	} else if !strings.Contains(err.Error(), "Brief: <stream>/<NN>") {
		t.Fatalf("refusal must still name the missing line; got: %v", err)
	}

	// (c) tightened contract: a body that only QUOTES the marker somewhere in its prose —
	// not at the head — is NOT exempt and still refuses. A whole-body substring match would
	// wrongly exempt this hand-authored body; the head-anchored check does not.
	quoting := []byte("This PR discusses the `" + deskkit.ScanBodyMarker + "` marker but is hand-written.\n")
	if _, err := requireTrailer(quoting, "", ""); err == nil {
		t.Fatal("a body merely quoting the marker in prose must NOT be exempt")
	} else if !deskkit.IsRefused(err) {
		t.Fatalf("marker-quoting body err = %v, want exit-5 refusal", err)
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
