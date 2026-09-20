package main

// unresolvable_test.go — FAIL-FIRST coverage for the fail-closed unresolvable brief (#1309 item 5).
//
// THE DEFECT. resolveBrief returned a zero-value frontmatter when `brief-<num>-*.md` was not
// found (or unreadable): gate "", every risk flag false — which classifyItem read as a
// risk-clear model-gated brief and routed to DISPATCH with any human gate erased. Latent on the
// live board (0 unresolvable rows), but a fail-open. These pin that an unresolvable row is
// could-not-check: listed with its reason, never dispatchable.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnresolvableBrief_IsCouldNotCheckNeverDispatch(t *testing.T) {
	root := t.TempDir()
	table := "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | missing | 0 | S | implemented | — | — |\n" + // no brief-01-*.md on disk
		"| 02 | present | 0 | S | implemented | — | — |\n"
	writeFixtureStream(t, root, "example-stream", table, map[string]string{"02": planBrief("model", "no", "no", "no", "no")})

	it := selectOne(t, root, "example-stream/01")
	if got := payloadValue(it, "could_not_check"); !strings.HasPrefix(got, "brief file not found: ") {
		t.Fatalf("could_not_check payload = %q, want a 'brief file not found' reason", got)
	}
	tier, _ := (&VerifyLoop{}).TierPolicy(it)
	if disp, _ := classifyItem(it, tier); disp != dispCouldNotCheck {
		t.Fatalf("unresolvable brief classified %v; want could-not-check (the zero frontmatter is not a risk-clear brief)", disp)
	}

	var perr error
	out := captureStdout(t, func() { perr = cmdPlan([]string{"--root", root}) })
	if perr != nil {
		t.Fatalf("cmdPlan: %v", perr)
	}
	if strings.Contains(out, "=== DISPATCH example-stream/01") {
		t.Fatalf("unresolvable brief reached DISPATCH:\n%s", out)
	}
	if !strings.Contains(out, "-- could-not-check (1): the brief file could not be resolved/read") ||
		!strings.Contains(out, "example-stream/01 — brief file not found: docs/streams/example-stream/brief-01-*.md") {
		t.Fatalf("plan does not list the unresolvable brief under could-not-check with its reason:\n%s", out)
	}
	if !strings.Contains(out, "=== DISPATCH example-stream/02") {
		t.Fatalf("the resolvable sibling must still dispatch:\n%s", out)
	}
}

// An unreadable brief file (present but not readable) is could-not-check too, and the
// could-not-check arm precedes every other marker: nothing can be read off a file that did
// not open.
func TestUnresolvableBrief_UnreadableFileIsCouldNotCheck(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every file; the unreadable fixture cannot be built")
	}
	root := t.TempDir()
	writeFixtureStream(t, root, "example-stream", oneRowTable, map[string]string{"01": planBrief("human", "no", "no", "yes", "no")})
	p := filepath.Join(root, "docs", "streams", "example-stream", "brief-01-x.md")
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	it := selectOne(t, root, "example-stream/01")
	tier, _ := (&VerifyLoop{}).TierPolicy(it)
	disp, reason := classifyItem(it, tier)
	if disp != dispCouldNotCheck || !strings.HasPrefix(reason, "brief file unreadable: ") {
		t.Fatalf("unreadable brief classified %v (%q); want could-not-check with an 'unreadable' reason", disp, reason)
	}
}
