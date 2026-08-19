package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// loadVGStreams copies the verify-gate fixture tree into a temp root and loads it.
// Returns the temp root and the parsed streams.
func loadVGStreams(t *testing.T) (string, []*Stream) {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/verifygate")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, streams
}

func TestVerifyIssuesSelection(t *testing.T) {
	root, streams := loadVGStreams(t)

	tests := []struct {
		name     string
		existing map[string]bool
		want     []string // brief ids expected, in emitted order
	}{
		{
			name:     "no existing markers → all gate:human + verified briefs + irreversible-at-implemented",
			existing: map[string]bool{},
			// vg/01, vg/02, vg/07, vg/08, vg/11 are all gate:human + verified — emitted
			// even when already human-reviewed (vg/07, vg/11), because the done-close is
			// a distinct acceptance touch. vg/09 is gate:human + irreversible + implemented
			// with VERIFY: PASS — emitted per the chicken-and-egg fix.
			// vg/03 is model, vg/04 implemented (no pass), vg/05 done,
			// vg/06 todo, vg/10 implemented no pass → excluded.
			want: []string{"vg/01", "vg/02", "vg/07", "vg/08", "vg/09", "vg/11"},
		},
		{
			name:     "an existing marker suppresses its brief",
			existing: map[string]bool{verifyMarker("vg/02"): true},
			want:     []string{"vg/01", "vg/07", "vg/08", "vg/09", "vg/11"},
		},
		{
			name: "all markers present → nothing new",
			existing: map[string]bool{
				verifyMarker("vg/01"): true,
				verifyMarker("vg/02"): true,
				verifyMarker("vg/07"): true,
				verifyMarker("vg/08"): true,
				verifyMarker("vg/09"): true,
				verifyMarker("vg/11"): true,
			},
			want: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := verifyIssues(root, streams, tc.existing)
			var ids []string
			for _, iss := range got {
				ids = append(ids, iss.Brief)
			}
			if strings.Join(ids, ",") != strings.Join(tc.want, ",") {
				t.Errorf("emitted briefs = %v, want %v", ids, tc.want)
			}
		})
	}
}

// TestVerifyIssuesEmitsAlreadyHumanReviewed pins the two-touch model: a
// gate:human + verified brief whose Reviewed cell already names a human
// (vg/07) IS still emitted — the done-close is a distinct acceptance touch,
// separate from the verified-stage review.
func TestVerifyIssuesEmitsAlreadyHumanReviewed(t *testing.T) {
	root, streams := loadVGStreams(t)
	found := false
	for _, iss := range verifyIssues(root, streams, map[string]bool{}) {
		if iss.Brief == "vg/07" {
			found = true
		}
	}
	if !found {
		t.Error("vg/07 (gate:human + verified, already human-reviewed) must still be emitted — the done-close is a distinct acceptance touch")
	}
}

func TestVerifyIssueShape(t *testing.T) {
	root, streams := loadVGStreams(t)
	issues := verifyIssues(root, streams, map[string]bool{})
	var vg01 *verifyIssue
	for i := range issues {
		if issues[i].Brief == "vg/01" {
			vg01 = &issues[i]
		}
	}
	if vg01 == nil {
		t.Fatal("vg/01 not emitted")
	}

	if want := "verify-gate: vg/01 — Human-gated verified brief (eligible for a verify-gate issue)"; vg01.Title != want {
		t.Errorf("title = %q, want %q", vg01.Title, want)
	}
	if len(vg01.Labels) != 1 || vg01.Labels[0] != "verify-gate" {
		t.Errorf("labels = %v, want [verify-gate]", vg01.Labels)
	}
	if vg01.Marker != "<!-- verify-gate: vg/01 -->" {
		t.Errorf("marker = %q", vg01.Marker)
	}

	// Body must be self-contained: marker first line, gate reason (the yes risk
	// keys), the Verify table, the recorded Evidence, the PR link, the brief
	// link, and the "before you close" checklist.
	body := vg01.Body
	if !strings.HasPrefix(body, "<!-- verify-gate: vg/01 -->") {
		t.Errorf("body must start with the marker; got:\n%s", body)
	}
	for _, sub := range []string{
		"Gate reason",
		"regulatory, irreversible", // both yes keys, canonical order
		"### Verify",
		"go test ./...",
		"### Recorded results (Evidence)",
		"glm-verifier",
		"pull/42",
		"blob/main/docs/streams/vg/brief-01-human-verified.md",
		"- [ ] I accept this brief as done",
		"Only a human closer is honored.",
	} {
		if !strings.Contains(body, sub) {
			t.Errorf("body missing %q; got:\n%s", sub, body)
		}
	}
}

// TestRenderVerifyGateWhy covers gate-why-rationale: renderVerifyBody emits the
// `> **Why you're being asked to sign off:** …` blockquote (under the title,
// above the mechanical Gate reason line) when the brief carries a gate-why, and
// omits it entirely when absent. Constructed BriefFiles keep the test offline
// (briefBody tolerates a non-existent path).
func TestRenderVerifyGateWhy(t *testing.T) {
	base := &BriefFile{
		Brief: "vg/01",
		Title: "A human-gated brief",
		Path:  "docs/streams/vg/brief-01.md",
		Risk:  map[string]string{"regulatory": "yes", "customer": "no", "irreversible": "yes", "sensitive-data": "no"},
	}

	t.Run("present → blockquote rendered above Gate reason", func(t *testing.T) {
		bf := *base
		bf.GateWhy = "Rewrites an immutable on-ledger invariant; wrong math forks balances with no fix."
		body := renderVerifyBody(".", &bf)
		quote := "> **Why you're being asked to sign off:** " + bf.GateWhy
		if !strings.Contains(body, quote) {
			t.Errorf("body missing gate-why blockquote; got:\n%s", body)
		}
		// Ordering: the blockquote must appear BEFORE the mechanical Gate reason.
		if qi, gi := strings.Index(body, quote), strings.Index(body, "**Gate reason**"); qi < 0 || gi < 0 || qi > gi {
			t.Errorf("gate-why blockquote (%d) must precede Gate reason (%d); got:\n%s", qi, gi, body)
		}
	})

	t.Run("absent → no blockquote", func(t *testing.T) {
		bf := *base // GateWhy == ""
		body := renderVerifyBody(".", &bf)
		if strings.Contains(body, "Why you're being asked to sign off") {
			t.Errorf("no gate-why should render no blockquote; got:\n%s", body)
		}
	})
}

// TestRenderVerifyWhy covers the why blockquote: renderVerifyBody emits the
// `> **Why this work exists:** …` blockquote (under the gate-why block,
// above the mechanical Gate reason line) when the brief carries a why:, and
// omits it entirely when absent.
func TestRenderVerifyWhy(t *testing.T) {
	base := &BriefFile{
		Brief: "vg/01",
		Title: "A human-gated brief",
		Path:  "docs/streams/vg/brief-01.md",
		Risk:  map[string]string{"regulatory": "yes", "customer": "no", "irreversible": "yes", "sensitive-data": "no"},
	}

	t.Run("present → blockquote rendered above Gate reason", func(t *testing.T) {
		bf := *base
		bf.Why = "All 22 pages are eagerly imported into a single JS chunk, so every user pays a multi-MB first load; splitting cuts the initial payload ~5x."
		body := renderVerifyBody(".", &bf)
		quote := "> **Why this work exists:** " + bf.Why
		if !strings.Contains(body, quote) {
			t.Errorf("body missing why blockquote; got:\n%s", body)
		}
		// Ordering: the why blockquote must appear BEFORE the mechanical Gate reason.
		if qi, gi := strings.Index(body, quote), strings.Index(body, "**Gate reason**"); qi < 0 || gi < 0 || qi > gi {
			t.Errorf("why blockquote (%d) must precede Gate reason (%d); got:\n%s", qi, gi, body)
		}
	})

	t.Run("absent → no why blockquote", func(t *testing.T) {
		bf := *base // Why == ""
		body := renderVerifyBody(".", &bf)
		if strings.Contains(body, "Why this work exists") {
			t.Errorf("no why should render no why blockquote; got:\n%s", body)
		}
	})

	// When both gate-why and why are present, why appears after gate-why but
	// both appear before Gate reason.
	t.Run("both present → ordered gate-why then why, both before Gate reason", func(t *testing.T) {
		bf := *base
		bf.GateWhy = "Rewrites an immutable on-ledger invariant."
		bf.Why = "Users currently have no way to verify without re-running."
		body := renderVerifyBody(".", &bf)
		gq := "> **Why you're being asked to sign off:** " + bf.GateWhy
		wq := "> **Why this work exists:** " + bf.Why
		gi := strings.Index(body, gq)
		wi := strings.Index(body, wq)
		gateIdx := strings.Index(body, "**Gate reason**")
		if gi < 0 || wi < 0 || gateIdx < 0 {
			t.Fatalf("missing expected text; gate-why=%d why=%d gate=%d", gi, wi, gateIdx)
		}
		if gi > wi {
			t.Errorf("gate-why (%d) must appear before why (%d)", gi, wi)
		}
		if wi > gateIdx {
			t.Errorf("why (%d) must appear before Gate reason (%d)", wi, gateIdx)
		}
	})
}

func TestVerifyIssuesExistingMarkersFile(t *testing.T) {
	// loadExistingMarkers extracts markers from raw content (issue bodies OK).
	dir := t.TempDir()

	t.Run("missing file → empty set", func(t *testing.T) {
		set, err := loadExistingMarkers(filepath.Join(dir, "does-not-exist.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if len(set) != 0 {
			t.Errorf("want empty set, got %v", set)
		}
	})

	t.Run("empty path → empty set", func(t *testing.T) {
		set, err := loadExistingMarkers("")
		if err != nil || len(set) != 0 {
			t.Errorf("want empty set, got %v err=%v", set, err)
		}
	})

	t.Run("extracts markers from mixed content", func(t *testing.T) {
		p := filepath.Join(dir, "markers.txt")
		content := "<!-- verify-gate: vg/01 -->\nsome issue body text\n<!-- verify-gate: vg/02 -->\nmore text\n"
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		set, err := loadExistingMarkers(p)
		if err != nil {
			t.Fatal(err)
		}
		if !set[verifyMarker("vg/01")] || !set[verifyMarker("vg/02")] {
			t.Errorf("want vg/01 and vg/02 markers, got %v", set)
		}
	})
}

// TestVerifyIssueTitleTruncation proves the G6 guard: the verifyIssues emitter
// routes its title through issueTitle() so that an over-long brief title cannot
// 422 the issue-creation batch. This is the discriminating test requested by the
// re-review: reverting the issueTitle call to raw string concatenation must
// produce a break that this test catches.
func TestVerifyIssueTitleTruncation(t *testing.T) {
	root, streams := loadVGStreams(t)
	issues := verifyIssues(root, streams, map[string]bool{})
	var vg08 *verifyIssue
	for i := range issues {
		if issues[i].Brief == "vg/08" {
			vg08 = &issues[i]
		}
	}
	if vg08 == nil {
		t.Fatal("vg/08 (gate:human, verified, long title > 256 runes) not emitted - check fixture")
	}
	// The raw concatenation "verify-gate: vg/08 - <long-title>" would easily
	// exceed 256 runes. The whole point of issueTitle() is to cap it.
	t.Logf("vg/08 title is %d runes", utf8.RuneCountInString(vg08.Title))
	if n := utf8.RuneCountInString(vg08.Title); n > 256 {
		t.Errorf("vg/08 verify-gate title is %d runes > 256, issueTitle not truncating or emitter bypasses it", n)
	}
	if !strings.HasSuffix(vg08.Title, "…") {
		t.Error("vg/08 title is > 256 runes raw but lacks trailing ellipsis (U+2026), issueTitle may not have been reached")
	}
}

func TestCloseVerifyFlipsRow(t *testing.T) {
	root, _ := loadVGStreams(t)
	now := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)

	if err := closeVerify(root, "vg/01", now); err != nil {
		t.Fatalf("close-verify vg/01: %v", err)
	}

	// Re-read the README and confirm the row flipped + Reviewed stamped.
	raw, err := os.ReadFile(filepath.Join(root, "docs/streams/vg/README.md"))
	if err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	var s *Stream
	for _, st := range streams {
		if st.Name == "vg" {
			s = st
		}
	}
	row := findRow(s, "01")
	if row.Status != "done" {
		t.Errorf("status = %q, want done", row.Status)
	}
	// Date-first ordering, matching the repo convention.
	if row.Reviewed != "2026-07-09 human:reviewer" {
		t.Errorf("reviewed = %q, want %q", row.Reviewed, "2026-07-09 human:reviewer")
	}
	// Other rows are untouched.
	if r2 := findRow(s, "02"); r2.Status != "verified" {
		t.Errorf("vg/02 status changed to %q, want verified", r2.Status)
	}
	if !strings.Contains(string(raw), "2026-07-09 human:reviewer") {
		t.Error("README missing the stamped Reviewed cell")
	}
}

// TestCloseVerifyAppendsToExistingHuman confirms close-verify is strictly
// additive: a brief already carrying a human sign-off in Reviewed keeps that
// original date/reviewer verbatim, and the acceptance touch is APPENDED so both
// are distinguishable in the cell (two-touch model).
func TestCloseVerifyAppendsToExistingHuman(t *testing.T) {
	root, _ := loadVGStreams(t)
	now := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)

	if err := closeVerify(root, "vg/07", now); err != nil {
		t.Fatalf("close-verify vg/07: %v", err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	var s *Stream
	for _, st := range streams {
		if st.Name == "vg" {
			s = st
		}
	}
	row := findRow(s, "07")
	if row.Status != "done" {
		t.Errorf("status = %q, want done", row.Status)
	}
	// Original sign-off preserved verbatim; acceptance touch appended.
	want := "2026-07-08 human:alex; accepted 2026-07-09 human:reviewer"
	if row.Reviewed != want {
		t.Errorf("reviewed = %q, want %q", row.Reviewed, want)
	}
	if !strings.HasPrefix(row.Reviewed, "2026-07-08 human:alex") {
		t.Errorf("original sign-off must be preserved byte-for-byte at the front; got %q", row.Reviewed)
	}
}

// TestCloseVerifyAppends_UndatedExisting reproduces the
// verify-gate-close CI failure on a hardening stream's gate:human brief: its
// Reviewed cell at `verified` was a bare, UNDATED "human:alex" — a sanctioned
// value (hasHumanReviewer does not require a date). Prior to
// the fix, close-verify appended its acceptance stamp AFTER that undated
// content ("human:alex; accepted 2026-07-09 human:alex"), producing a cell that
// does not start with a date and so fails methodology/19's own --lint rule
// (brieffile.go: status done needs verifiedCellRe to match) in the very same
// binary. The writer must never emit a value its own --lint rejects.
func TestCloseVerifyAppends_UndatedExisting(t *testing.T) {
	root, _ := loadVGStreams(t)
	now := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)

	if err := closeVerify(root, "vg/11", now); err != nil {
		t.Fatalf("close-verify vg/11: %v", err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	var s *Stream
	for _, st := range streams {
		if st.Name == "vg" {
			s = st
		}
	}
	row := findRow(s, "11")
	if row.Status != "done" {
		t.Errorf("status = %q, want done", row.Status)
	}
	// The new dated stamp must lead (nothing to anchor on in the undated
	// prior), with the original undated sign-off preserved as a trailing note.
	want := "2026-07-09 human:reviewer; prior human:alex"
	if row.Reviewed != want {
		t.Errorf("reviewed = %q, want %q", row.Reviewed, want)
	}
	if !strings.Contains(row.Reviewed, "human:alex") {
		t.Errorf("original undated sign-off must be preserved somewhere in the cell; got %q", row.Reviewed)
	}
	// The whole point: the composed cell must satisfy the SAME --lint rule
	// (methodology/19) that closeVerify's own package enforces at done.
	if !verifiedCellRe.MatchString(row.Reviewed) {
		t.Errorf("composed Reviewed cell %q does not start with a dated stamp — fails methodology/19's done-shape lint", row.Reviewed)
	}
	problems, _ := checkBriefFiles(streams, streams)
	for _, p := range problems {
		if strings.Contains(p, "vg/11") || strings.Contains(p, "brief-11") {
			t.Errorf("checkBriefFiles reported a PROBLEM against the brief close-verify just closed: %s", p)
		}
	}
}

func TestCloseVerifyRefuses(t *testing.T) {
	tests := []struct {
		name  string
		brief string
	}{
		{"model-gated brief is refused", "vg/03"},
		{"non-verified (implemented) brief is refused", "vg/04"},
		{"already-done brief is refused", "vg/05"},
		{"todo brief is refused", "vg/06"},
		{"unknown brief is refused", "vg/99"},
		{"unknown stream is refused", "nope/01"},
	}
	now := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := loadVGStreams(t)
			before, err := os.ReadFile(filepath.Join(root, "docs/streams/vg/README.md"))
			if err != nil {
				t.Fatal(err)
			}
			if err := closeVerify(root, tc.brief, now); err == nil {
				t.Fatalf("close-verify %s should have refused", tc.brief)
			}
			after, err := os.ReadFile(filepath.Join(root, "docs/streams/vg/README.md"))
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Errorf("refused close-verify %s must not write the README", tc.brief)
			}
		})
	}
}

// TestRunVerifyIssuesEmptyJSON confirms the process-level entrypoint emits a
// valid empty JSON array (not null) when nothing is eligible.
func TestRunVerifyIssuesEmptyJSON(t *testing.T) {
	root, streams := loadVGStreams(t)
	// Suppress every eligible brief via an existing-markers set.
	existing := map[string]bool{
		verifyMarker("vg/01"): true,
		verifyMarker("vg/02"): true,
		verifyMarker("vg/07"): true,
		verifyMarker("vg/08"): true,
		verifyMarker("vg/09"): true,
		verifyMarker("vg/11"): true,
	}
	if got := verifyIssues(root, streams, existing); len(got) != 0 {
		t.Fatalf("expected no eligible issues, got %d", len(got))
	}
}

// TestVerifyIssuesIrreversibleAtImplemented covers the chicken-and-egg fix:
// an irreversible brief at implemented whose Evidence records a model verify pass
// IS emitted; one without the marker is NOT emitted; a non-irreversible brief at
// implemented is never emitted regardless of marker.
func TestVerifyIssuesIrreversibleAtImplemented(t *testing.T) {
	root, streams := loadVGStreams(t)
	all := verifyIssues(root, streams, map[string]bool{})

	// vg/09: irreversible, implemented, WITH **VERIFY: PASS** → emitted.
	found09 := false
	for _, iss := range all {
		if iss.Brief == "vg/09" {
			found09 = true
			break
		}
	}
	if !found09 {
		t.Error("vg/09 (irreversible, implemented, VERIFY: PASS) must be emitted")
	}

	// vg/10: irreversible, implemented, WITHOUT **VERIFY: PASS** → NOT emitted.
	for _, iss := range all {
		if iss.Brief == "vg/10" {
			t.Error("vg/10 (irreversible, implemented, no VERIFY: PASS) must NOT be emitted")
		}
	}

	// vg/04: irreversible, implemented, empty Evidence (no VERIFY: PASS) → NOT emitted
	// (this was already excluded by the existing test; re-affirm here).
	for _, iss := range all {
		if iss.Brief == "vg/04" {
			t.Error("vg/04 (irreversible, implemented, empty Evidence) must NOT be emitted")
		}
	}
}

// TestIrreversibleGateBodyShape verifies the distinguished card body for an
// irreversible-at-implemented brief: it states the one-step advance, renders
// UNRUN rows, and includes the trade-offs and prior-state sections.
func TestIrreversibleGateBodyShape(t *testing.T) {
	root, streams := loadVGStreams(t)
	all := verifyIssues(root, streams, map[string]bool{})
	var vg09 *verifyIssue
	for i := range all {
		if all[i].Brief == "vg/09" {
			vg09 = &all[i]
		}
	}
	if vg09 == nil {
		t.Fatal("vg/09 not emitted")
	}

	body := vg09.Body

	// Marker must be the first line.
	if !strings.HasPrefix(body, "<!-- verify-gate: vg/09 -->") {
		t.Errorf("body must start with marker; got:\n%s", body)
	}

	// Title distinguishes it as irreversible.
	if !strings.Contains(body, "Human sign-off required (irreversible)") {
		t.Error("body must state 'irreversible' in the title")
	}

	// Must state the one-step advance language.
	if !strings.Contains(body, "implemented → verified → done") {
		t.Error("body must state the one-step advance (implemented→verified→done)")
	}

	// UNRUN rows must be surfaced prominently.
	if !strings.Contains(body, "### Deferred / UNRUN rows") {
		t.Error("body must have a Deferred / UNRUN rows section")
	}
	if !strings.Contains(body, "UNRUN — mutating money-path") {
		t.Errorf("body must render the UNRUN row text; got:\n%s", body)
	}

	// Trade-offs section must be present.
	if !strings.Contains(body, "### Trade-offs") {
		t.Error("body must have a Trade-offs section")
	}
	if !strings.Contains(body, "TRADE-OFFS: not provided") {
		t.Errorf("body must flag missing trade-offs; got:\n%s", body)
	}

	// Prior-state remediation section for risk-classed briefs.
	if !strings.Contains(body, "### Prior-state remediation") {
		t.Error("body must have a Prior-state remediation section (risk-classed brief)")
	}
	if !strings.Contains(body, "PRIOR-STATE: unassessed") {
		t.Errorf("body must flag unassessed prior-state; got:\n%s", body)
	}

	// Checklist includes irreversible-specific items.
	if !strings.Contains(body, "- [ ] Model verify pass is acceptable") {
		t.Error("body checklist must include model verify pass acceptance")
	}

	// Label and marker are compatible with the existing workflow.
	if len(vg09.Labels) != 1 || vg09.Labels[0] != "verify-gate" {
		t.Errorf("labels = %v, want [verify-gate]", vg09.Labels)
	}
	if vg09.Marker != "<!-- verify-gate: vg/09 -->" {
		t.Errorf("marker = %q", vg09.Marker)
	}
}

// TestCloseVerifyIrreversibleAdvance confirms that close-verify on an irreversible
// brief at implemented with VERIFY: PASS advances both the Verified and Reviewed
// cells in one step, satisfying the irreversible lint.
func TestCloseVerifyIrreversibleAdvance(t *testing.T) {
	root, _ := loadVGStreams(t)
	now := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)

	if err := closeVerify(root, "vg/09", now); err != nil {
		t.Fatalf("close-verify vg/09 (irreversible, implemented, VERIFY: PASS): %v", err)
	}

	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	var s *Stream
	for _, st := range streams {
		if st.Name == "vg" {
			s = st
		}
	}
	row := findRow(s, "09")

	// Status advanced to done.
	if row.Status != "done" {
		t.Errorf("status = %q, want done", row.Status)
	}

	// Verified cell stamped from Evidence (opus-verifier with 2026-07-17 date).
	if row.Verified != "2026-07-17 opus-verifier" {
		t.Errorf("verified = %q, want %q", row.Verified, "2026-07-17 opus-verifier")
	}

	// Reviewed cell stamped with human sign-off.
	if row.Reviewed != "2026-07-17 human:reviewer" {
		t.Errorf("reviewed = %q, want %q", row.Reviewed, "2026-07-17 human:reviewer")
	}
}

// TestCloseVerifyIrreversibleRefuses confirms close-verify refuses an irreversible
// brief at implemented WITHOUT a VERIFY: PASS marker, and a non-irreversible brief
// at implemented.
func TestCloseVerifyIrreversibleRefuses(t *testing.T) {
	now := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		brief string
		err   string
	}{
		{
			name:  "irreversible implemented without VERIFY: PASS",
			brief: "vg/10",
			err:   "VERIFY: PASS",
		},
		{
			name:  "irreversible implemented empty evidence (no VERIFY: PASS)",
			brief: "vg/04",
			err:   "VERIFY: PASS",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := loadVGStreams(t)
			err := closeVerify(root, tc.brief, now)
			if err == nil {
				t.Fatalf("close-verify %s should have refused", tc.brief)
			}
			if !strings.Contains(err.Error(), tc.err) {
				t.Errorf("refusal message should mention %q; got: %v", tc.err, err)
			}
		})
	}
}

// TestEvidenceVerifierInfo confirms evidenceVerifierInfo correctly extracts date
// and runner from an Evidence table.
func TestEvidenceVerifierInfo(t *testing.T) {
	evidence := `Verifier run:

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | go test | 0 | PASS | 2026-07-17 | opus-verifier |
| 2 | go vet  | 0 | clean | 2026-07-17 | opus-verifier |

**VERIFY: PASS**`

	date, runner := evidenceVerifierInfo(evidence)
	if date != "2026-07-17" {
		t.Errorf("date = %q, want 2026-07-17", date)
	}
	if runner != "opus-verifier" {
		t.Errorf("runner = %q, want opus-verifier", runner)
	}
}

// TestEvidenceVerifierInfoEmpty confirms evidenceVerifierInfo returns empty
// strings when no Evidence table with Date/Runner columns is found.
func TestEvidenceVerifierInfoEmpty(t *testing.T) {
	date, runner := evidenceVerifierInfo("no table here")
	if date != "" || runner != "" {
		t.Errorf("want empty, got date=%q runner=%q", date, runner)
	}
}

// TestHasVerifyPass confirms hasVerifyPass correctly detects the marker.
func TestHasVerifyPass(t *testing.T) {
	if !hasVerifyPass("**VERIFY: PASS** (model) — done") {
		t.Error("hasVerifyPass must detect **VERIFY: PASS**")
	}
	if hasVerifyPass("no marker here") {
		t.Error("hasVerifyPass must not false-positive")
	}
}

// TestUnrunRowsText confirms unrunRowsText extracts UNRUN rows.
func TestUnrunRowsText(t *testing.T) {
	evidence := `| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | go test | 0 | PASS | 2026-07-17 | opus-verifier |
| 2 | live test | — | UNRUN — mutating | 2026-07-17 | opus-verifier |
**VERIFY: PASS**`

	text := unrunRowsText(evidence)
	if !strings.Contains(text, "UNRUN — mutating") {
		t.Errorf("unrunRowsText must extract UNRUN row; got: %q", text)
	}

	if unrunRowsText("no unrun here") != "" {
		t.Error("unrunRowsText must return empty on no UNRUN")
	}
}
