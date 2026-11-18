package main

// crossinvocation_test.go — FAIL-FIRST coverage for #5999.
//
// THE DEFECT. For a cross-repository item (a tracking brief whose deliverable is another repo)
// `verifyloop plan` printed only ` root=<tracking checkout>`. Every deskdispatch invocation an
// operator derived from that either refused (--root at the tracking checkout) or claimed in the
// wrong repo, so no printed shape both claimed and landed Evidence.
//
// WHAT THIS PINS. The plan prints the ONE deskdispatch command for the item, built from the
// association the scan already computed: --repo/--root at the deliverable, --claim-root and the
// absolute --brief at the tracking checkout. A same-repository or legacy item prints none.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #5999
func TestPlanPrintsCrossInvocation(t *testing.T) {
	v := associationQueueFixture(t, 3)
	tracking := v.Root
	source := deskkit.RootForRepo("example-org/agents")
	stubPreflight(t, func(string) error { return nil })

	var err error
	out := captureStdout(t, func() { err = cmdPlan([]string{"--root=" + tracking}) })
	if err != nil {
		t.Fatalf("cmdPlan: %v\n%s", err, out)
	}
	want := "deskdispatch sample/01 --kit verifier --repo example-org/agents --root " + source +
		" --claim-root " + tracking + " --brief " + filepath.Join(tracking, "docs/streams/sample/brief-01-fixture.md") +
		" --model <actual-model> --tier <selected-tier>"
	if !strings.Contains(out, want) {
		t.Fatalf("plan does not print the cross-repository dispatch invocation\nwant: %s\nplan:\n%s", want, out)
	}
	if !strings.Contains(out, "claims and lands in example-org/tracker") {
		t.Fatalf("plan does not name the tracking repo as the claim/landing namespace:\n%s", out)
	}
	// Exactly one invocation: the same-repository (02) and legacy (03) items print none.
	if n := strings.Count(out, "deskdispatch "); n != 1 {
		t.Fatalf("want exactly one printed invocation, got %d:\n%s", n, out)
	}
}
