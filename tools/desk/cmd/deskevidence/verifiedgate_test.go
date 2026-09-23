package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestVerifiedBriefsAdded is the unit contract of the added-verified-row scanner: it names only
// the briefs whose `"outcome":"verified"` rows THIS commit adds relative to the branch, deduped,
// skipping verify-fail rows, malformed lines, and rows the branch already carries.
func TestVerifiedBriefsAdded(t *testing.T) {
	remote := `{"ts":"2026-09-07T01:00:00Z","brief":"s/01","outcome":"verify-fail","sha":"a"}
{"ts":"2026-09-07T02:00:00Z","brief":"s/02","outcome":"verified","sha":"b"}
`
	commit := remote +
		`{"ts":"2026-09-07T03:00:00Z","brief":"s/03","outcome":"verify-fail","sha":"c"}
{"ts":"2026-09-07T04:00:00Z","brief":"s/04","outcome":"verified","sha":"d"}
{"ts":"2026-09-07T05:00:00Z","brief":"s/04","outcome":"verified","sha":"e"}
not json at all
{"ts":"2026-09-07T06:00:00Z","brief":"","outcome":"verified","sha":"f"}
{"ts":"2026-09-07T07:00:00Z","brief":"s/05","outcome":"verified","sha":"g"}
`
	got := verifiedBriefsAdded([]byte(remote), []byte(commit), true)
	want := []string{"s/04", "s/05"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("verifiedBriefsAdded = %v, want %v", got, want)
	}

	// s/02 is verified but already on the branch — an append that re-states it adds nothing.
	if in(got, "s/02") {
		t.Fatalf("s/02 was already on the branch and must not be reported as added: %v", got)
	}

	// A brand-new sidecar (no remote base) is read whole.
	whole := verifiedBriefsAdded(nil, []byte(commit), false)
	if !in(whole, "s/02") || !in(whole, "s/04") || !in(whole, "s/05") {
		t.Fatalf("brand-new sidecar scan = %v, want it to include s/02, s/04, s/05", whole)
	}
}

func in(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// sidecarBase is a small, realistic verify-outcomes.jsonl already on the branch.
const sidecarBase = `{"ts":"2026-09-07T01:00:00Z","brief":"example-stream/01","outcome":"verify-fail","rows_passed":4,"rows_total":5,"sha":"0000001"}
`

// TestVerifiedSidecarRefusedWhenClosureNotAccepted is the fail-first regression guard for #1309:
// a landing that APPENDS an `"outcome":"verified"` row for a brief whose tree does NOT present a
// lint-valid verified closure (Evidence filled, but no witness / board still implemented) is
// refused, and NOTHING is written. Before this gate existed the row landed unconditionally and
// verifyloop then bucketed the mismatch as a stuck-flip review had to refuse.
func TestVerifiedSidecarRefusedWhenClosureNotAccepted(t *testing.T) {
	f, errBuf := setupFake(t)

	// The tree does not accept a verified closure for the appended brief.
	var askedFor []string
	verifiedClosureCheckFn = func(root, brief string) (closureVerdict, string, error) {
		askedFor = append(askedFor, brief)
		return closureNotAccepted, "example-stream/14: NOT accepted — Status is \"implemented\", not verified/done", nil
	}

	evidencePath := "docs/streams/verify-outcomes.jsonl"
	appended := sidecarBase +
		`{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verified","rows_passed":5,"rows_total":5,"sha":"0000002"}` + "\n"
	root := rootWithFile(t, evidencePath, appended)
	f.setFile(evidencePath, sidecarBase)

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitRefused {
		t.Fatalf("verified-without-closure sidecar landing exit = %d, want %d (stderr %q)", code, deskkit.ExitRefused, errBuf.String())
	}
	if len(f.hits) != 1 || !strings.HasPrefix(f.hits[0], "GET ") {
		t.Fatalf("refusal must not write: hits = %v (want a single GET, no PUT)", f.hits)
	}
	if f.putCalls != 0 {
		t.Fatalf("refusal still wrote %d time(s)", f.putCalls)
	}
	if len(askedFor) != 1 || askedFor[0] != "example-stream/14" {
		t.Fatalf("closure check asked for %v, want exactly [example-stream/14]", askedFor)
	}
	if s := errBuf.String(); !strings.Contains(s, "example-stream/14") || !strings.Contains(s, "verified") {
		t.Fatalf("refusal message = %q, want it to name the brief and the verified outcome", s)
	}
}

// TestVerifiedSidecarAllowedWhenClosureAccepted is the AFTER-fix companion: a genuinely
// lint-valid verified closure still appends `"outcome":"verified"` exactly as before.
func TestVerifiedSidecarAllowedWhenClosureAccepted(t *testing.T) {
	f, errBuf := setupFake(t)

	verifiedClosureCheckFn = func(root, brief string) (closureVerdict, string, error) {
		return closureAccepted, "", nil
	}

	evidencePath := "docs/streams/verify-outcomes.jsonl"
	appended := sidecarBase +
		`{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verified","rows_passed":5,"rows_total":5,"sha":"0000002"}` + "\n"
	root := rootWithFile(t, evidencePath, appended)
	f.setFile(evidencePath, sidecarBase)

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("accepted verified sidecar landing exit = %d, want 0 (stderr %q)", code, errBuf.String())
	}
	if f.putCalls != 1 {
		t.Fatalf("expected exactly 1 write, got %d", f.putCalls)
	}
	if !strings.Contains(f.putContent, `"brief":"example-stream/14","outcome":"verified"`) {
		t.Fatalf("landed content did not carry the verified row: %q", f.putContent)
	}
}

// TestVerifyFailSidecarRowNeverGated proves the gate is scoped to `verified`: a landing that
// appends only a `verify-fail` row never consults the closure check and lands unchanged.
func TestVerifyFailSidecarRowNeverGated(t *testing.T) {
	f, errBuf := setupFake(t)

	verifiedClosureCheckFn = func(root, brief string) (closureVerdict, string, error) {
		t.Fatalf("closure check must NOT run for a verify-fail-only landing (brief %q)", brief)
		return closureCouldNotErr, "", nil
	}

	evidencePath := "docs/streams/verify-outcomes.jsonl"
	appended := sidecarBase +
		`{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","rows_passed":2,"rows_total":5,"sha":"0000002"}` + "\n"
	root := rootWithFile(t, evidencePath, appended)
	f.setFile(evidencePath, sidecarBase)

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("verify-fail sidecar landing exit = %d, want 0 (stderr %q)", code, errBuf.String())
	}
	if f.putCalls != 1 {
		t.Fatalf("expected exactly 1 write, got %d", f.putCalls)
	}
}

// TestVerifiedSidecarCouldNotCheckRefusesUnverifiable: a closure check that could not evaluate
// the brief (statusgen absent, unreadable board) refuses the landing as Unverifiable — never a
// silent pass (three-state instrument rule, C4).
func TestVerifiedSidecarCouldNotCheckRefusesUnverifiable(t *testing.T) {
	f, errBuf := setupFake(t)

	verifiedClosureCheckFn = func(root, brief string) (closureVerdict, string, error) {
		return closureCouldNotErr, "", deskkit.Unverifiable("statusgen verifyclosure could not evaluate "+brief, nil)
	}

	evidencePath := "docs/streams/verify-outcomes.jsonl"
	appended := sidecarBase +
		`{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verified","rows_passed":5,"rows_total":5,"sha":"0000002"}` + "\n"
	root := rootWithFile(t, evidencePath, appended)
	f.setFile(evidencePath, sidecarBase)

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("could-not-check verified sidecar landing exit = %d, want %d (stderr %q)", code, deskkit.ExitUnverifiable, errBuf.String())
	}
	if f.putCalls != 0 {
		t.Fatalf("could-not-check still wrote %d time(s)", f.putCalls)
	}
}
