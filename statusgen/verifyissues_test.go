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

// TestNormalizeBriefKey pins the two-forms-one-identity reduction (issue #804):
// a brief-v2 <cell>:<repo>:<stream>:<NN> key collapses to the canonical
// <stream>/<NN>; a brief-v1 <stream>/<NN> key is already canonical; anything
// else is returned unchanged so it fails downstream on its own terms.
func TestNormalizeBriefKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"svc/17", "svc/17"},               // v1 slash form — unchanged
		{"example:app:svc:17", "svc/17"},   // v2 colon form → canonical
		{" example:app:svc:17 ", "svc/17"}, // surrounding space tolerated
		{"vg/09", "vg/09"},                 // v1 with no trailing letter
		{"example:app:svc:12a", "svc/12a"}, // v2, NN carries a trailing letter
		{"not-a-key", "not-a-key"},         // no colon, no slash — unchanged
		{"a:b:c", "a:b:c"},                 // 3 colon segments (not v2) — unchanged
		{"a:b:c:d:e", "a:b:c:d:e"},         // 5 colon segments (not v2) — unchanged
	}
	for _, c := range cases {
		if got := normalizeBriefKey(c.in); got != c.want {
			t.Errorf("normalizeBriefKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestVerifyMarkerFormsCollapse is the render half of issue #804: verifyMarker
// renders ONE marker for a brief whether it is named in the v1 slash form or the
// v2 colon form, and that marker is the canonical <stream>/<NN> the close side
// already speaks. Pre-fix, the colon key rendered a distinct marker and the two
// were never equal — the duplicate-card bug.
func TestVerifyMarkerFormsCollapse(t *testing.T) {
	slash := verifyMarker("svc/17")
	colon := verifyMarker("example:app:svc:17")
	if slash != colon {
		t.Errorf("verifyMarker must collapse both key forms to one marker: slash=%q colon=%q", slash, colon)
	}
	if want := "<!-- verify-gate: svc/17 -->"; slash != want {
		t.Errorf("verifyMarker renders %q, want the canonical %q", slash, want)
	}
}

// TestLoadExistingMarkersCrossForm is the match half of issue #804: a card whose
// body carries the COLON marker suppresses a brief the emitter names in the SLASH
// form, and vice versa — loadExistingMarkers normalizes both to one set key.
func TestLoadExistingMarkersCrossForm(t *testing.T) {
	dir := t.TempDir()

	t.Run("colon-marker body matches slash-form verifyMarker", func(t *testing.T) {
		p := filepath.Join(dir, "colon.txt")
		body := "<!-- verify-gate: example:app:svc:17 -->\nHuman sign-off required — example:app:svc:17\n"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		set, err := loadExistingMarkers(p)
		if err != nil {
			t.Fatal(err)
		}
		if !set[verifyMarker("svc/17")] {
			t.Errorf("a colon-keyed existing card must suppress the slash-keyed brief; set=%v", set)
		}
		if !set[verifyMarker("example:app:svc:17")] {
			t.Errorf("and must equally match the colon-keyed render; set=%v", set)
		}
	})

	t.Run("slash-marker body matches colon-form verifyMarker", func(t *testing.T) {
		p := filepath.Join(dir, "slash.txt")
		body := "<!-- verify-gate: svc/17 -->\n"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		set, err := loadExistingMarkers(p)
		if err != nil {
			t.Fatal(err)
		}
		if !set[verifyMarker("example:app:svc:17")] {
			t.Errorf("a slash-keyed existing card must suppress the colon-keyed (v2) brief; set=%v", set)
		}
	})
}

// TestVerifyIssuesSuppressesCrossForm proves the end-to-end dedupe across forms
// through the real code paths (loadExistingMarkers → verifyIssues): a v1 fixture
// brief (vg/01) is suppressed by an existing card whose marker is written in the
// v2 colon form. Pre-fix this brief was re-emitted — the #804 duplicate.
func TestVerifyIssuesSuppressesCrossForm(t *testing.T) {
	root, streams := loadVGStreams(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "existing.txt")
	// vg/01 is emitted with an empty existing set (see TestVerifyIssuesSelection).
	// Here its ONLY existing card carries the colon-form marker.
	if err := os.WriteFile(p, []byte("<!-- verify-gate: example:app:vg:01 -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	existing, err := loadExistingMarkers(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, iss := range verifyIssues(root, streams, existing) {
		if iss.Brief == "vg/01" {
			t.Errorf("vg/01 must be suppressed by its colon-keyed existing card, but it was re-emitted")
		}
	}
}

// TestCloseVerifyAcceptsColonForm is the close half of issue #804: --close-verify
// accepts a brief-v2 <cell>:<repo>:<stream>:<NN> id and flips the same row the
// v1 slash id flips. Pre-fix, closeVerify refused the colon id as "not a
// <stream>/<NN> id" and nothing advanced.
func TestCloseVerifyAcceptsColonForm(t *testing.T) {
	root, _ := loadVGStreams(t)
	now := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)

	if err := closeVerify(root, "example:app:vg:01", now); err != nil {
		t.Fatalf("close-verify with the colon (brief-v2) form must succeed: %v", err)
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
		t.Errorf("status = %q, want done (colon-form close must flip vg/01)", row.Status)
	}
	if row.Reviewed != "2026-07-09 human:reviewer" {
		t.Errorf("reviewed = %q, want %q", row.Reviewed, "2026-07-09 human:reviewer")
	}
}

func TestVerifyIssuesSelection(t *testing.T) {
	root, streams := loadVGStreams(t)

	tests := []struct {
		name     string
		existing map[string]bool
		want     []string // brief ids expected, in emitted order
	}{
		{
			name:     "no existing markers → all gate:human + verified briefs + gate:human-at-implemented-with-pass",
			existing: map[string]bool{},
			// vg/01, vg/02, vg/07, vg/08, vg/11 are all gate:human + verified — emitted
			// even when already human-reviewed (vg/07, vg/11), because the done-close is
			// a distinct acceptance touch. vg/09 is gate:human + irreversible + implemented
			// with VERIFY: PASS — the original one-step path. vg/12 (irreversible:no) and
			// vg/14 (irreversible:no, marker but no Date/Runner table) are the generalized
			// non-irreversible one-step path — both emit on the strict marker alone.
			// vg/03 is model, vg/04 implemented (no pass), vg/05 done, vg/06 todo,
			// vg/10 implemented no pass, vg/13 implemented FAIL, vg/15 model+pass → excluded.
			want: []string{"vg/01", "vg/02", "vg/07", "vg/08", "vg/09", "vg/11", "vg/12", "vg/14"},
		},
		{
			name:     "an existing marker suppresses its brief",
			existing: map[string]bool{verifyMarker("vg/02"): true},
			want:     []string{"vg/01", "vg/07", "vg/08", "vg/09", "vg/11", "vg/12", "vg/14"},
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
				verifyMarker("vg/12"): true,
				verifyMarker("vg/14"): true,
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
		verifyMarker("vg/12"): true,
		verifyMarker("vg/14"): true,
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

// TestVerifyIssuesNonIrreversibleAtImplemented is the mm/49 generalization: a
// gate:human brief at `implemented` with the strict **VERIFY: PASS** marker is
// emitted EVEN WHEN irreversible:no. It also pins the fail-closed exclusions —
// no recorded pass (vg/13) and gate:model (vg/15) never emit.
func TestVerifyIssuesNonIrreversibleAtImplemented(t *testing.T) {
	root, streams := loadVGStreams(t)
	all := verifyIssues(root, streams, map[string]bool{})
	emitted := map[string]bool{}
	for _, iss := range all {
		emitted[iss.Brief] = true
	}

	// (a) vg/12: gate:human, irreversible:no, implemented, VERIFY: PASS +
	// Date/Runner row → EMITTED (the class the brief exists to surface).
	if !emitted["vg/12"] {
		t.Error("vg/12 (gate:human, irreversible:no, implemented, VERIFY: PASS) must be emitted")
	}
	// (c) vg/14: gate:human, irreversible:no, implemented, VERIFY: PASS present but
	// NO Date/Runner table → EMITTED. Design call: emission depends only on the
	// strict marker (byte-identical to the irreversible path); the missing table is
	// caught at close time (closeVerify refuses — see TestCloseVerifyNonIrreversibleRefuses).
	if !emitted["vg/14"] {
		t.Error("vg/14 (gate:human, implemented, VERIFY: PASS, no Date/Runner table) must be emitted; the table is a close-time gate, not an emission gate")
	}
	// (b) vg/13: gate:human, irreversible:no, implemented, VERIFY: FAIL (no strict
	// pass marker) → NOT emitted (fail-closed on hasVerifyPass).
	if emitted["vg/13"] {
		t.Error("vg/13 (gate:human, implemented, recorded FAIL, no **VERIFY: PASS**) must NOT be emitted")
	}
	// (d) vg/15: gate:model, implemented, VERIFY: PASS → never reaches the human gate.
	if emitted["vg/15"] {
		t.Error("vg/15 (gate:model, implemented, VERIFY: PASS) must NOT be emitted — the human gate is gate:human only")
	}
}

// TestImplementedGateBodyNonIrreversible pins the body of a non-irreversible
// one-step card: the heading does NOT claim irreversibility, it states the
// one-step advance and names the recorded runner, and it keeps the UNRUN/Trade-offs
// prominence shared with the irreversible card.
func TestImplementedGateBodyNonIrreversible(t *testing.T) {
	root, streams := loadVGStreams(t)
	all := verifyIssues(root, streams, map[string]bool{})
	var vg12 *verifyIssue
	for i := range all {
		if all[i].Brief == "vg/12" {
			vg12 = &all[i]
		}
	}
	if vg12 == nil {
		t.Fatal("vg/12 not emitted")
	}
	body := vg12.Body

	if !strings.HasPrefix(body, "<!-- verify-gate: vg/12 -->") {
		t.Errorf("body must start with the marker; got:\n%s", body)
	}
	// Distinguished heading for the non-irreversible class.
	if !strings.Contains(body, "Human sign-off required (verified-by-model at implemented)") {
		t.Errorf("body must carry the non-irreversible heading; got:\n%s", body)
	}
	// It must NOT claim irreversibility for a non-irreversible brief.
	if strings.Contains(body, "(irreversible)") {
		t.Errorf("non-irreversible body must not claim irreversibility in its heading; got:\n%s", body)
	}
	// One-step advance language + the recorded runner named.
	if !strings.Contains(body, "implemented → verified → done") {
		t.Error("body must state the one-step advance (implemented→verified→done)")
	}
	if !strings.Contains(body, "2026-07-18 glm-verifier") {
		t.Errorf("body must name the recorded model run (date + runner); got:\n%s", body)
	}
}

// TestCloseVerifyNonIrreversibleAdvance confirms close-verify advances a
// non-irreversible gate:human brief at implemented with VERIFY: PASS in one step —
// Verified stamped from the recorded Evidence date+runner, Reviewed from the human
// close — the mm/49 close-side generalization.
func TestCloseVerifyNonIrreversibleAdvance(t *testing.T) {
	root, _ := loadVGStreams(t)
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	if err := closeVerify(root, "vg/12", now); err != nil {
		t.Fatalf("close-verify vg/12 (gate:human, irreversible:no, implemented, VERIFY: PASS): %v", err)
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
	row := findRow(s, "12")
	if row.Status != "done" {
		t.Errorf("status = %q, want done", row.Status)
	}
	// Verified cell stamped from the RECORDED run, not the close time.
	if row.Verified != "2026-07-18 glm-verifier" {
		t.Errorf("verified = %q, want %q (stamped from the recorded Evidence run)", row.Verified, "2026-07-18 glm-verifier")
	}
	if row.Reviewed != "2026-07-20 human:reviewer" {
		t.Errorf("reviewed = %q, want %q", row.Reviewed, "2026-07-20 human:reviewer")
	}
}

// TestCloseVerifyNonIrreversibleRefuses pins the fail-closed close-side gates for
// the generalized path: no recorded pass (vg/13) is refused, and a recorded pass
// with NO Date/Runner Evidence row (vg/14) is STILL refused — the marker alone
// does not license the one-step advance, the recorded runner must exist to stamp
// the Verified cell.
func TestCloseVerifyNonIrreversibleRefuses(t *testing.T) {
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		brief string
		err   string
	}{
		{
			name:  "non-irreversible implemented without VERIFY: PASS",
			brief: "vg/13",
			err:   "VERIFY: PASS",
		},
		{
			name:  "non-irreversible implemented with pass but no Date/Runner table",
			brief: "vg/14",
			err:   "date/runner",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := loadVGStreams(t)
			before, err := os.ReadFile(filepath.Join(root, "docs/streams/vg/README.md"))
			if err != nil {
				t.Fatal(err)
			}
			err = closeVerify(root, tc.brief, now)
			if err == nil {
				t.Fatalf("close-verify %s should have refused", tc.brief)
			}
			if !strings.Contains(err.Error(), tc.err) {
				t.Errorf("refusal message should mention %q; got: %v", tc.err, err)
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

// TestVerifyMarkerRegex pins the ratified marker regex
// (verifyVerdictBoldRe / hasVerifyPass): a bold VERIFY verdict token followed
// by arbitrary prose up to the closing `**` matches, but the verdict token
// itself is anchored to PASS|FAIL — BLOCKED (or any other spelling) never
// does, whatever prose surrounds it. The pre-fix `strings.Contains(evidence,
// "**VERIFY: PASS**")` failed the first two rows below: a marker with any
// prose before its closing `**` did not literally contain that fixed
// substring, so a real verifier line was read as no pass at all.
func TestVerifyMarkerRegex(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"bare bold pass", "**VERIFY: PASS**", true},
		{"bold pass with row-count prose", "**VERIFY: PASS (4/4 offline-runnable rows)**", true},
		{"bold pass with em-dash summary", "**VERIFY: PASS — all 6 rows green.**", true},
		{"bold BLOCKED never matches", "**VERIFY: BLOCKED (human-gate)**", false},
		{"bold FAIL is not a pass", "**VERIFY: FAIL (row #3)**", false},
		{"no marker at all", "no marker here", false},
		{"non-bold VERIFY: PASS does not satisfy the strict gate", "VERIFY: PASS", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasVerifyPass(c.text); got != c.want {
				t.Errorf("hasVerifyPass(%q) = %v, want %v", c.text, got, c.want)
			}
		})
	}
}

// TestVerifyPassHeldContradictionSameLineLaundering pins verifyPassHeldContradiction
// against the SAME proximity-laundering shape PR #1244 found and fixed in the
// predecessor mechanism (entryIsHeld, statusgen/verifyissues.go), adapted to
// #1304's structurally different, LINE-scanning mechanism.
//
// entryIsHeld segmented an Evidence entry by "row N" mentions and treated
// everything up to the next "row N" mention as belonging to that row, so a
// trailing un-numbered HELD/could-not-check mention got swept into a prior
// numbered row's segment POSITIONALLY. verifyPassHeldContradiction does not
// segment by row at all — it walks the Evidence body LINE by LINE and, for
// any line containing HELD/could-not-check, excuses the WHOLE LINE the moment
// that line ALSO contains a routingKeywordRe match and a routingRefRe match
// anywhere on it. Neither regex is anchored to which HELD/could-not-check
// occurrence it corroborates: a routed disposition and a second, wholly
// unrelated, un-routed disposition sharing one physical line (a very ordinary
// shape — a verifier's Result cell often reads "deferred to X; separately,
// the smoke run HELD, no runner online") both launder through, because the
// exclusion check operates on line PRESENCE, not on binding a specific
// HELD/could-not-check occurrence to the routing phrase+reference nearest it
// (or after it) — the same "positional, not semantic" gap #1244 closed in the
// row-based mechanism, now reproduced at line granularity.
func TestVerifyPassHeldContradictionSameLineLaundering(t *testing.T) {
	// Row 2 is genuinely, correctly routed (routing phrase "deferred to" +
	// reference "verify-integrity/05"): decideModelFlip's own test
	// (TestAutoflipRefusesHeldPass) already proves that exclusion is correct
	// on its own. Here the SAME line ALSO carries a second, unrelated
	// disposition — a separate check ("the nightly smoke run") that went
	// HELD, trailing the routed clause, with no routing of its own. The
	// routing tokens on the line corroborate the FIRST disposition, not the
	// second — verifyPassHeldContradiction must still refuse.
	laundered := "**VERIFY: PASS (2/2 offline-runnable rows)**\n\n" +
		"| # | Command | Exit | Result | Date | Runner |\n" +
		"|---|---------|------|--------|------|--------|\n" +
		"| 1 | `go vet ./...` | 0 | ok | 2026-07-08 | fixture-verifier |\n" +
		"| 2 | `go test ./integration/...` | — | could-not-check: deferred to follow-up brief verify-integrity/05, and separately the nightly smoke run went HELD, no runner online | 2026-07-08 | fixture-verifier |\n"

	if held, why := verifyPassHeldContradiction(laundered); !held {
		t.Fatalf("a routed disposition sharing a line with a second, unrelated, un-routed HELD mention must still refuse the PASS — proximity-laundering shape got through unrefused (why=%q)", why)
	}

	// Control: the SAME routed row, with no second disposition trailing it,
	// stays clean — the routing exclusion itself must still work.
	clean := "**VERIFY: PASS (2/2 offline-runnable rows)**\n\n" +
		"| # | Command | Exit | Result | Date | Runner |\n" +
		"|---|---------|------|--------|------|--------|\n" +
		"| 1 | `go vet ./...` | 0 | ok | 2026-07-08 | fixture-verifier |\n" +
		"| 2 | `go test ./integration/...` | — | could-not-check: deferred to follow-up brief verify-integrity/05 | 2026-07-08 | fixture-verifier |\n"

	if held, why := verifyPassHeldContradiction(clean); held {
		t.Errorf("a genuinely routed row with no other disposition on its line must not contradict the PASS, got held=true why=%q", why)
	}

	// Control: the same unrelated HELD mention with NO routing anywhere on
	// its line must already be caught (this is the pre-#1304-fix baseline,
	// not new behavior).
	unrouted := "**VERIFY: PASS (2/2 offline-runnable rows)**\n\n" +
		"| # | Command | Exit | Result | Date | Runner |\n" +
		"|---|---------|------|--------|------|--------|\n" +
		"| 1 | `go vet ./...` | 0 | ok | 2026-07-08 | fixture-verifier |\n" +
		"| 2 | `go test ./integration/...` | — | the nightly smoke run went HELD, no runner online | 2026-07-08 | fixture-verifier |\n"

	if held, why := verifyPassHeldContradiction(unrouted); !held {
		t.Fatalf("an un-routed HELD mention with no routing tokens at all on its line must refuse the PASS, got held=false why=%q", why)
	}
}

// TestVerifyPassHeldContradictionNegatedMentionIsNotADisposition pins the
// fix for the false-positive class where heldOrCouldNotCheckRe fires on prose
// that MERELY MENTIONS a held/could-not-check state without the line actually
// carrying one — a bare substring/word-boundary match cannot tell "this row
// IS held" from "this row is NOT held" or "there are ZERO held rows" apart,
// because both contain the literal marker word.
//
// Two concrete shapes, both observed refusing genuinely clean Evidence:
//   - "no could-not-check" — asserting the ABSENCE of a could-not-check row.
//   - "0 HELD" — a zero-count summary, also a clean state.
//
// Each negative-control case must NOT contradict the PASS. The trailing
// positive control proves the fix did not broaden into a false negative: a
// genuine, un-routed, un-negated HELD row on its own line must still
// contradict the PASS exactly as before.
func TestVerifyPassHeldContradictionNegatedMentionIsNotADisposition(t *testing.T) {
	cases := []struct {
		name     string
		evidence string
		wantHeld bool
	}{
		{
			name: "no could-not-check negates the mention",
			evidence: "**VERIFY: PASS** — clean run, no could-not-check rows remain.\n\n" +
				"| # | Command | Exit | Result | Date | Runner |\n" +
				"|---|---------|------|--------|------|--------|\n" +
				"| 1 | `go test ./...` | 0 | ok | 2026-07-10 | fixture-verifier |\n",
			wantHeld: false,
		},
		{
			name: "0 HELD is a zero-count summary, not a disposition",
			evidence: "**VERIFY: PASS** — summary: 0 HELD, 1 PASS.\n\n" +
				"| # | Command | Exit | Result | Date | Runner |\n" +
				"|---|---------|------|--------|------|--------|\n" +
				"| 1 | `go test ./...` | 0 | ok | 2026-07-10 | fixture-verifier |\n",
			wantHeld: false,
		},
		{
			name: "PASS ... no could-not-check, with arbitrary prose between",
			evidence: "**VERIFY: PASS (1/1 rows)** — all rows executed; no could-not-check anywhere in this run.\n\n" +
				"| # | Command | Exit | Result | Date | Runner |\n" +
				"|---|---------|------|--------|------|--------|\n" +
				"| 1 | `go test ./...` | 0 | ok | 2026-07-10 | fixture-verifier |\n",
			wantHeld: false,
		},
		{
			// Positive control: a negated mention elsewhere in the Evidence
			// must NOT blind the scan to a real, un-routed, un-negated HELD
			// row on a different line — only the negated OCCURRENCE is
			// excused, never the whole scan.
			name: "negated summary line does not excuse a real HELD row elsewhere",
			evidence: "**VERIFY: PASS** — summary: 0 HELD across the first table.\n\n" +
				"| # | Command | Exit | Result | Date | Runner |\n" +
				"|---|---------|------|--------|------|--------|\n" +
				"| 1 | `go test ./...` | 0 | ok | 2026-07-10 | fixture-verifier |\n" +
				"| 2 | `go test ./integration/...` | — | HELD — no runner online | 2026-07-10 | fixture-verifier |\n",
			wantHeld: true,
		},
		{
			// Positive control: an un-negated, un-routed HELD row alone must
			// still contradict the PASS after the fix — the fix must not
			// broaden into failing to catch a genuinely held row.
			name: "genuine un-routed HELD row still contradicts the PASS",
			evidence: "**VERIFY: PASS** — row 1 green.\n\n" +
				"| # | Command | Exit | Result | Date | Runner |\n" +
				"|---|---------|------|--------|------|--------|\n" +
				"| 1 | `go test ./...` | 0 | ok | 2026-07-10 | fixture-verifier |\n" +
				"| 2 | `go test ./integration/...` | — | HELD — no runner online | 2026-07-10 | fixture-verifier |\n",
			wantHeld: true,
		},
		{
			// Counter-example: a bare
			// "0" that is NOT in a count position — here it is an exit code,
			// not a count of held rows — must never excuse the HELD it
			// happens to precede. "row 3 exit 0 HELD" is a REAL held row
			// report; the exit code coincidentally reads "0" right before the
			// marker. Wrongly excusing this is a false NEGATIVE: a
			// verification gate silently passing something broken, which is
			// worse than the original false-positive class this fix addresses.
			name: "exit code 0 immediately before HELD is not a zero-count negation",
			evidence: "**VERIFY: PASS** — row 1 green.\n\n" +
				"| # | Command | Exit | Result | Date | Runner |\n" +
				"|---|---------|------|--------|------|--------|\n" +
				"| 1 | `go test ./...` | 0 | ok | 2026-07-10 | fixture-verifier |\n" +
				"| 2 | `go test ./integration/...` | — | row 3 exit 0 HELD for human diff-read | 2026-07-10 | fixture-verifier |\n",
			wantHeld: true,
		},
		{
			// Counter-example: "row 0" is a row LABEL, not a
			// count of held rows — "row 0 HELD" is a genuine held report for
			// the row numbered 0.
			name: "row label 0 immediately before HELD is not a zero-count negation",
			evidence: "**VERIFY: PASS** — row 1 green.\n\n" +
				"| # | Command | Exit | Result | Date | Runner |\n" +
				"|---|---------|------|--------|------|--------|\n" +
				"| 1 | `go test ./...` | 0 | ok | 2026-07-10 | fixture-verifier |\n" +
				"| 2 | `go test ./integration/...` | — | row 0 HELD | 2026-07-10 | fixture-verifier |\n",
			wantHeld: true,
		},
		{
			// Counter-example: a version number ending in ".0"
			// reads as a bare "0" token right before HELD, but it is a
			// version, not a count.
			name: "version number ending in .0 immediately before HELD is not a zero-count negation",
			evidence: "**VERIFY: PASS** — row 1 green.\n\n" +
				"| # | Command | Exit | Result | Date | Runner |\n" +
				"|---|---------|------|--------|------|--------|\n" +
				"| 1 | `go test ./...` | 0 | ok | 2026-07-10 | fixture-verifier |\n" +
				"| 2 | `go test ./integration/...` | — | row 3 on v1.0 HELD pending runner | 2026-07-10 | fixture-verifier |\n",
			wantHeld: true,
		},
		{
			// Counter-example: same version-number shape, no
			// leading "v".
			name: "bare decimal ending in .0 immediately before HELD is not a zero-count negation",
			evidence: "**VERIFY: PASS** — row 1 green.\n\n" +
				"| # | Command | Exit | Result | Date | Runner |\n" +
				"|---|---------|------|--------|------|--------|\n" +
				"| 1 | `go test ./...` | 0 | ok | 2026-07-10 | fixture-verifier |\n" +
				"| 2 | `go test ./integration/...` | — | 2.0 HELD | 2026-07-10 | fixture-verifier |\n",
			wantHeld: true,
		},
		{
			// Mixed same-line case: the leading "0" is a genuine count
			// negating the HELD marker it sits directly in front of
			// ("summary: 0 HELD"), but the trailing "1" is not a negation
			// word/zero-count and must not excuse the could-not-check
			// occurrence that follows it — that clause is reporting a REAL,
			// non-zero could-not-check count.
			name: "mixed line negates one marker but not the other",
			evidence: "**VERIFY: PASS** — summary: 0 HELD, 1 could-not-check.\n\n" +
				"| # | Command | Exit | Result | Date | Runner |\n" +
				"|---|---------|------|--------|------|--------|\n" +
				"| 1 | `go test ./...` | 0 | ok | 2026-07-10 | fixture-verifier |\n",
			wantHeld: true,
		},
		{
			// Per-occurrence pin: a negated occurrence earlier ON
			// THE SAME LINE must not excuse a second, genuine, un-negated
			// occurrence of the SAME marker word later on that same line.
			// This kills a line-level ("any negated mention anywhere on the
			// line excuses the whole line") shortcut that a per-occurrence
			// implementation must not take.
			name: "negated could-not-check does not excuse a second could-not-check on the same line",
			evidence: "**VERIFY: PASS** — no could-not-check rows except row 4: could-not-check.\n\n" +
				"| # | Command | Exit | Result | Date | Runner |\n" +
				"|---|---------|------|--------|------|--------|\n" +
				"| 1 | `go test ./...` | 0 | ok | 2026-07-10 | fixture-verifier |\n",
			wantHeld: true,
		},
		{
			// Per-occurrence pin: a zero-counted HELD occurrence
			// earlier on the line must not excuse a second, genuine,
			// un-negated HELD occurrence later on that same line.
			name: "zero-counted HELD does not excuse a second HELD on the same line",
			evidence: "**VERIFY: PASS** — summary: 0 HELD; row 4 HELD — no runner online.\n\n" +
				"| # | Command | Exit | Result | Date | Runner |\n" +
				"|---|---------|------|--------|------|--------|\n" +
				"| 1 | `go test ./...` | 0 | ok | 2026-07-10 | fixture-verifier |\n",
			wantHeld: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			held, why := verifyPassHeldContradiction(c.evidence)
			if held != c.wantHeld {
				t.Errorf("verifyPassHeldContradiction(%q) held=%v why=%q, want held=%v", c.evidence, held, why, c.wantHeld)
			}
		})
	}
}

// TestVerifyPassHeldContradictionNegationCuePosition pins WHERE a negation or
// zero-count cue may sit and still excuse the HELD/could-not-check occurrence
// right after it. A cue word or a "0" directly before the marker is not, by
// itself, a negation: a labelled exit code ("exit: 0 HELD", "rc: 0 HELD"), an
// answer to a question ("available? no HELD") or a field value ("green: no
// HELD") put the same tokens in front of a LIVE hold. Each must-refuse line
// below is a genuine hold that a PASS must not proceed over; each must-excuse
// line is a clean count or negation that must not refuse. Every position
// rule has at least one must-refuse line with NO hold reason, so deleting
// that rule turns a case red rather than being masked by the reason check.
func TestVerifyPassHeldContradictionNegationCuePosition(t *testing.T) {
	const header = "**VERIFY: PASS** — row 1 green.\n\n"
	cell := func(result string) string {
		return header +
			"| # | Command | Exit | Result | Date | Runner |\n" +
			"|---|---------|------|--------|------|--------|\n" +
			"| 1 | `go test ./...` | 0 | ok | 2026-07-10 | fixture-verifier |\n" +
			"| 3 | `go test ./integration/...` | — | " + result + " | 2026-07-10 | fixture-verifier |\n"
	}
	prose := func(line string) string { return header + line + "\n" }

	cases := []struct {
		name     string
		evidence string
		wantHeld bool
	}{
		// Must refuse: a labelled exit/status zero is not a held-row count.
		{"colon-labelled exit code before HELD", prose("row 3 exit: 0 HELD"), true},
		{"colon-labelled exit code with hold reason", cell("row 3 exit code: 0 HELD pending human read"), true},
		{"rc label before HELD", prose("rc: 0 HELD"), true},
		{"list of exit codes ending in 0 before HELD", prose("exit codes: 1, 0 HELD"), true},
		{"parenthesised exit code before HELD", prose("row 3 (0 HELD)"), true},
		// Must refuse: a "no" that answers a question or fills a field.
		{"question answered no before HELD", prose("runner available? no HELD pending runner"), true},
		{"field value no before HELD", cell("row 3 green: no HELD pending runner"), true},
		{"field value not before HELD", prose("row 3: not HELD"), true},
		{"assigned no before HELD", prose("runner=no HELD"), true},
		{"table-cell no before HELD", prose("| 3 | no HELD |"), true},
		// No hold reason on these, so only the position rule can refuse them.
		{"question answered no, no hold reason", prose("runner available? no HELD"), true},
		{"question answered no across a non-breaking space", prose("runner available? no HELD"), true},
		{"question answered no across a zero-width space", prose("runner available?​no HELD"), true},
		{"bold field label before no", prose("**row 3 green:** no HELD"), true},
		{"bold question-style field label before no", prose("**Runner available:** no HELD"), true},
		{"bold row label before not", prose("**row 3:** not HELD"), true},
		{"hyphen before no", prose("row 3 green - no HELD"), true},
		{"closing parenthesis before no", prose("(runner up) no HELD"), true},
		{"non-zero is not a zero count", prose("exit codes: 0 clean / non-zero could-not-check"), true},
		// Must refuse: an exit status spelled out, or a number that is not a
		// verdict count, is not a count position.
		{"exit status zero before HELD", prose("row 3 exit zero HELD"), true},
		{"exit code zero before HELD", prose("row 3 exit code zero HELD"), true},
		{"rc zero before HELD", prose("rc zero HELD"), true},
		{"returned zero before HELD", prose("row 3 returned zero HELD"), true},
		{"non-count item before zero", prose("row 3 exit, 0 HELD"), true},
		{"non-count item closed by semicolon before zero", prose("step 2 exit; 0 HELD"), true},
		// Must refuse: a negation followed by a hold reason contradicts itself.
		{"dash-answered no with hold reason", prose("runner available — no HELD pending runner"), true},
		{"zero count with hold reason", prose("0 HELD until the runner is back"), true},
		{"zero count, comma, hold reason", prose("0 HELD, pending runner"), true},
		{"zero count, parenthesised hold reason", prose("0 HELD (awaiting runner)"), true},
		{"zero count, hyphen, hold reason", prose("0 HELD - awaiting runner"), true},
		{"zero count, em dash, hold reason", prose("summary: 0 HELD — until the runner is back"), true},
		{"negation, comma, hold reason", prose("row 3 is not HELD, pending runner"), true},
		{"dash-answered no, comma, hold reason", prose("runner available — no HELD, pending runner"), true},
		{"marker used as a label with a value", prose("0 HELD: human read owed"), true},
		// Must refuse: struck text never joins a cue to a marker, and never
		// stands in for what precedes a cue.
		{"struck span between cue and marker", prose("row 3 not ~~yet green, still~~ HELD"), true},
		{"struck span between count label and zero", prose("summary: ~~3~~0 HELD"), true},
		{"struck span right before the cue", prose("row 3 ~~ok~~ no HELD"), true},

		// Must excuse: genuine negations and zero counts.
		{"not negates the marker in prose", prose("row 3 is not HELD; every row ran green."), false},
		{"zero negates the marker", prose("zero could-not-check rows in this run."), false},
		{"zero count at line start", prose("0 HELD, 7 PASS."), false},
		{"zero count continuing a count list", prose("totals: 7 PASS, 0 HELD."), false},
		{"zero count after a count label", prose("Count: 0 could-not-check."), false},
		{"negation after a count label", prose("summary: no could-not-check rows."), false},
		{"dash then negation, no hold reason", prose("all rows ran — no could-not-check."), false},
		{"negation after a bold count label", prose("**Summary:** no could-not-check"), false},
		{"zero count after a bold count label", prose("**Summary:** 0 HELD"), false},
		{"negation after a list marker", prose("- no could-not-check rows"), false},
		{"sentence-start negation", prose("every row passes. No HELD rows remain."), false},
		{"linking word before zero", prose("certificate issued with zero could-not-check."), false},
		{"linking word otherwise before no", prose("otherwise no could-not-check rows, no invented scope."), false},
		{"semicolon before zero", prose("all rows green; zero could-not-check."), false},
		{"arrow before no", prose("`go` present → no could-not-check."), false},
		{"parenthesised negation", prose("every row ran (no could-not-check)."), false},
		{"clause break then a non-breaking space before no", prose("all rows ran,\u00a0no could-not-check."), false},
		{"zero count after a rows count, dash note", prose("**VERIFY: PASS (5/5 rows, 0 HELD — live cluster access was available)**"), false},
		{"zero count after a fail count", prose("9/9 rows pass, 0 fail, 0 held."), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			held, why := verifyPassHeldContradiction(c.evidence)
			if held != c.wantHeld {
				t.Errorf("verifyPassHeldContradiction(%q) held=%v why=%q, want held=%v", c.evidence, held, why, c.wantHeld)
			}
		})
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
