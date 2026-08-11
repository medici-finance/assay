package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// loadDCStreams copies the decision fixture tree into a temp root and loads it.
func loadDCStreams(t *testing.T) (string, []*Stream) {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/decision")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, streams
}

func TestDecisionIssuesSelection(t *testing.T) {
	root, streams := loadDCStreams(t)

	tests := []struct {
		name     string
		existing map[string]bool
		want     []string // brief ids expected, in emitted order
	}{
		{
			name:     "no existing markers -> all gate:human + implemented/verified briefs",
			existing: map[string]bool{},
			// dc/01 (implemented), dc/02 (verified), dc/06 (implemented, gate-why),
			// dc/07 (verified, gate-why), dc/11 (implemented, long title) are gate:human + implemented/verified.
			// dc/03 is model, dc/04 is todo, dc/05 is done -> excluded.
			// dc/10 (implemented, has decision-issue: 999) -> excluded by G1 dedup guard.
			want: []string{"dc/01", "dc/02", "dc/06", "dc/07", "dc/11"},
		},
		{
			name:     "gate:model brief -> nothing emitted",
			existing: map[string]bool{},
			// This test verifies that model-gated briefs are never in the output.
			// dc/03 is model + implemented -> must not appear.
			want: []string{"dc/01", "dc/02", "dc/06", "dc/07", "dc/11"},
		},
		{
			name:     "an existing marker suppresses its brief",
			existing: map[string]bool{decisionMarker("dc/02"): true},
			want:     []string{"dc/01", "dc/06", "dc/07", "dc/11"},
		},
		{
			name: "all markers present -> nothing new",
			existing: map[string]bool{
				decisionMarker("dc/01"): true,
				decisionMarker("dc/02"): true,
				decisionMarker("dc/06"): true,
				decisionMarker("dc/07"): true,
				decisionMarker("dc/11"): true,
			},
			want: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := decisionIssues(root, streams, tc.existing)
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

func TestDecisionIssueShape(t *testing.T) {
	root, streams := loadDCStreams(t)
	issues := decisionIssues(root, streams, map[string]bool{})
	var dc06 *decisionIssue
	for i := range issues {
		if issues[i].Brief == "dc/06" {
			dc06 = &issues[i]
		}
	}
	if dc06 == nil {
		t.Fatal("dc/06 not emitted")
	}

	if want := "needs-decision: dc/06 — Human-gated implemented brief with gate-why rationale"; dc06.Title != want {
		t.Errorf("title = %q, want %q", dc06.Title, want)
	}
	if len(dc06.Labels) != 1 || dc06.Labels[0] != "needs-decision" {
		t.Errorf("labels = %v, want [needs-decision]", dc06.Labels)
	}
	if dc06.Marker != "<!-- needs-decision: dc/06 -->" {
		t.Errorf("marker = %q", dc06.Marker)
	}

	// Body must be self-contained: marker first line, situation, gate-why blockquote,
	// options with pros/cons, what happens table, links, verify section.
	body := dc06.Body
	if !strings.HasPrefix(body, "<!-- needs-decision: dc/06 -->") {
		t.Errorf("body must start with the marker; got:\n%s", body)
	}
	for _, sub := range []string{
		"## Situation",
		"Gate reason",
		"regulatory, irreversible", // both yes keys
		"Why this needs your decision",
		"Rewrites an immutable on-ledger invariant", // gate-why text
		"## Options",
		"Option A: Sign off",
		"Option B: Request changes",
		"Why we'd want it",
		"What it limits or costs",
		"## What happens on each answer",
		"dc/08, dc/09",
		"## Links",
		"blob/main/docs/streams/dc/brief-06-human-implemented-gatewhy.md",
		"pull/56",
		"### Recorded results (Evidence)",
		"### Verify",
		"### Decider",
		"Only a verified human account is honored",
	} {
		if !strings.Contains(body, sub) {
			t.Errorf("body missing %q; got:\n%s", sub, body)
		}
	}
}

// TestDecisionIssueGateWhy verifies that when gate-why is present, it appears in
// the Situation section as a blockquote; when absent, no blockquote is rendered.
func TestDecisionIssueGateWhy(t *testing.T) {
	root, streams := loadDCStreams(t)
	issues := decisionIssues(root, streams, map[string]bool{})

	t.Run("present -> blockquote in Situation", func(t *testing.T) {
		var dc06 *decisionIssue
		for i := range issues {
			if issues[i].Brief == "dc/06" {
				dc06 = &issues[i]
			}
		}
		if dc06 == nil {
			t.Fatal("dc/06 not emitted")
		}
		quote := "> **Why this needs your decision:** Rewrites an immutable on-ledger invariant; wrong math forks balances with no fix."
		if !strings.Contains(dc06.Body, quote) {
			t.Errorf("body missing gate-why blockquote; got:\n%s", dc06.Body)
		}
	})

	t.Run("absent -> no blockquote", func(t *testing.T) {
		var dc01 *decisionIssue
		for i := range issues {
			if issues[i].Brief == "dc/01" {
				dc01 = &issues[i]
			}
		}
		if dc01 == nil {
			t.Fatal("dc/01 not emitted")
		}
		if strings.Contains(dc01.Body, "Why this needs your decision") {
			t.Errorf("body should not contain gate-why blockquote when absent; got:\n%s", dc01.Body)
		}
	})
}

func TestDecisionIssuesExistingMarkersFile(t *testing.T) {
	dir := t.TempDir()

	t.Run("missing file -> empty set", func(t *testing.T) {
		set, err := loadDecisionMarkers(filepath.Join(dir, "does-not-exist.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if len(set) != 0 {
			t.Errorf("want empty set, got %v", set)
		}
	})

	t.Run("empty path -> empty set", func(t *testing.T) {
		set, err := loadDecisionMarkers("")
		if err != nil || len(set) != 0 {
			t.Errorf("want empty set, got %v err=%v", set, err)
		}
	})

	t.Run("extracts markers from mixed content", func(t *testing.T) {
		p := filepath.Join(dir, "markers.txt")
		content := "<!-- needs-decision: dc/01 -->\nsome issue body text\n<!-- needs-decision: dc/02 -->\nmore text\n"
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		set, err := loadDecisionMarkers(p)
		if err != nil {
			t.Fatal(err)
		}
		if !set[decisionMarker("dc/01")] || !set[decisionMarker("dc/02")] {
			t.Errorf("want dc/01 and dc/02 markers, got %v", set)
		}
	})
}

// TestDecisionIssueWhy covers methodology/27: renderDecisionBody emits the
// `> **Why this work exists:** …` blockquote (under the gate-why block, above the
// mechanical Gate reason line) when the brief carries a why:, and omits it when
// absent. Constructed BriefFiles keep the test offline.
func TestDecisionIssueWhy(t *testing.T) {
	base := &BriefFile{
		Brief: "dc/01",
		Title: "A human-gated brief",
		Path:  "docs/streams/dc/brief-01.md",
		Risk:  map[string]string{"regulatory": "yes", "customer": "no", "irreversible": "yes", "sensitive-data": "no"},
	}
	row := &Brief{Status: "implemented"}

	t.Run("present -> blockquote rendered in Situation", func(t *testing.T) {
		bf := *base
		bf.Why = "All 22 pages are eagerly imported into a single JS chunk, so every user pays a multi-MB first load; splitting cuts the initial payload ~5x."
		body := renderDecisionBody(".", &bf, row)
		quote := "> **Why this work exists:** " + bf.Why
		if !strings.Contains(body, quote) {
			t.Errorf("body missing why blockquote; got:\n%s", body)
		}
		// Ordering: why blockquote must appear BEFORE the mechanical Gate reason.
		if qi, gi := strings.Index(body, quote), strings.Index(body, "**Gate reason**"); qi < 0 || gi < 0 || qi > gi {
			t.Errorf("why blockquote (%d) must precede Gate reason (%d); got:\n%s", qi, gi, body)
		}
	})

	t.Run("absent -> no why blockquote", func(t *testing.T) {
		bf := *base // Why == ""
		body := renderDecisionBody(".", &bf, row)
		if strings.Contains(body, "Why this work exists") {
			t.Errorf("no why should render no why blockquote; got:\n%s", body)
		}
	})

	// When both gate-why and why are present, gate-why appears before why, both
	// before Gate reason.
	t.Run("both present -> ordered gate-why then why, both before Gate reason", func(t *testing.T) {
		bf := *base
		bf.GateWhy = "Rewrites an immutable on-ledger invariant."
		bf.Why = "Users currently have no way to verify without re-running."
		body := renderDecisionBody(".", &bf, row)
		gq := "> **Why this needs your decision:** " + bf.GateWhy
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

// TestDecisionIssuesEmptyJSON confirms the selection function returns an empty
// slice (not nil) when nothing is eligible.
func TestDecisionIssuesEmptyJSON(t *testing.T) {
	root, streams := loadDCStreams(t)
	// Suppress every eligible brief via an existing-markers set.
	existing := map[string]bool{
		decisionMarker("dc/01"): true,
		decisionMarker("dc/02"): true,
		decisionMarker("dc/06"): true,
		decisionMarker("dc/07"): true,
		decisionMarker("dc/11"): true,
	}
	got := decisionIssues(root, streams, existing)
	if len(got) != 0 {
		t.Fatalf("expected no eligible issues, got %d", len(got))
	}
}

// TestDecisionIssuesSkipsNonHumanGate confirms that model-gated briefs are never
// selected, even when at implemented/verified.
func TestDecisionIssuesSkipsNonHumanGate(t *testing.T) {
	root, streams := loadDCStreams(t)
	issues := decisionIssues(root, streams, map[string]bool{})
	for _, iss := range issues {
		if iss.Brief == "dc/03" {
			t.Error("dc/03 is model-gated — must not emit a decision issue")
		}
	}
}

// noticeContaining reports whether any notice contains ALL the given substrings.
func noticeContaining(notices []string, subs ...string) bool {
	for _, n := range notices {
		all := true
		for _, s := range subs {
			if !strings.Contains(n, s) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// Part (a) of the issue-loop/06 lint: the status-wide NOTICE fires only for
// gate:human briefs someone is waiting on (in-progress/implemented/verified),
// NEVER for backlog todo briefs — the register-flood guard from the #427 review.
func TestDecisionLintNoticeScope(t *testing.T) {
	_, streams := loadDCStreams(t)
	_, notices := checkBriefFiles(streams)

	for _, id := range []string{"dc/01", "dc/02", "dc/06", "dc/07"} {
		if !noticeContaining(notices, "brief "+id+" is gate:human at", "has no decision-issue") {
			t.Errorf("expected the part (a) NOTICE for %s (implemented/verified, no decision-issue)", id)
		}
	}
	if noticeContaining(notices, "brief dc/04 is gate:human", "has no decision-issue") {
		t.Error("part (a) NOTICE fired for a backlog todo brief (dc/04) — the flood guard regressed (#427 review)")
	}
	if noticeContaining(notices, "brief dc/05", "decision-issue") {
		t.Error("dc/05 is done with NO decision-issue linkage — neither lint leg should mention it")
	}
}

// Part (b): a done brief still carrying decision-issue NOTICEs only when the
// body never records the outcome; a body reference to #<NN> silences it, and
// the message must tell the maintainer to KEEP the linkage (#427 review —
// never advise deleting the audit trail).
func TestDecisionLintStaleLinkage(t *testing.T) {
	_, streams := loadDCStreams(t)
	_, notices := checkBriefFiles(streams)

	if !noticeContaining(notices, "brief dc/09 is done", "decision-issue #99", "keep the frontmatter linkage") {
		t.Error("expected the part (b) NOTICE for dc/09 (done, decision-issue: 99, outcome unrecorded)")
	}
	if noticeContaining(notices, "brief dc/08 is done", "decision-issue") {
		t.Error("part (b) NOTICE fired for dc/08, whose body records the decision (references #88)")
	}
	for _, n := range notices {
		if strings.Contains(n, "decision-issue") && strings.Contains(n, "remove it") {
			t.Errorf("a decision-issue notice advises removing the linkage (the audit record): %q", n)
		}
	}
}

// TestDecisionIssuesSkipsBackfilledBriefMidStream proves the G1 dedup guard:
// a gate:human brief at implemented that already carries decision-issue: NN in
// its frontmatter (manually backfilled — issue-loop/06 task 4) MUST NOT be
// emitted by decisionIssues(). This is the discriminating test requested by the
// #427 re-review: the guard's removal must be caught by a failing test.
func TestDecisionIssuesSkipsBackfilledBriefMidStream(t *testing.T) {
	root, streams := loadDCStreams(t)
	issues := decisionIssues(root, streams, map[string]bool{})
	for _, iss := range issues {
		if iss.Brief == "dc/10" {
			t.Error("dc/10 (gate:human, implemented) carries decision-issue: 999 in frontmatter — the G1 dedup guard must skip it, but it was emitted. The guard at decisionissues.go:162 may have been removed or broken.")
		}
	}
	// Double check: dc/10 should be present in the fixture (not a missing-file
	// false pass). All other gate:human + implemented/verified briefs without
	// decision-issue should still emit normally (dc/01, dc/02, dc/06, dc/07).
	foundDC01 := false
	for _, iss := range issues {
		if iss.Brief == "dc/01" {
			foundDC01 = true
		}
	}
	if !foundDC01 {
		t.Error("dc/01 (gate:human, implemented, no decision-issue) should still emit — fixture load may be broken")
	}
}

// TestDecisionIssueTitleTruncation proves the G6 guard: the decisionIssues
// emitter routes its title through issueTitle() so that an over-long brief
// title cannot 422 the issue-creation batch. This is the discriminating test
// requested by the #427 re-review: reverting the issueTitle call to raw string
// concatenation must produce a break that this test catches.
func TestDecisionIssueTitleTruncation(t *testing.T) {
	root, streams := loadDCStreams(t)
	issues := decisionIssues(root, streams, map[string]bool{})
	var dc11 *decisionIssue
	for i := range issues {
		if issues[i].Brief == "dc/11" {
			dc11 = &issues[i]
		}
	}
	if dc11 == nil {
		t.Fatal("dc/11 (gate:human, implemented, long title > 256 runes) not emitted — check fixture")
	}
	// The raw concatenation "needs-decision: dc/11 — <long-title>" would easily
	// exceed 256 runes. The whole point of issueTitle() is to cap it.
	t.Logf("dc/11 title is %d runes", utf8.RuneCountInString(dc11.Title))
	if n := utf8.RuneCountInString(dc11.Title); n > 256 {
		t.Errorf("dc/11 decision-issue title is %d runes > 256 — issueTitle() is NOT truncating, or the emitter bypasses it", n)
	}
	if !strings.HasSuffix(dc11.Title, "…") {
		t.Error("dc/11 title is > 256 runes raw but lacks trailing ellipsis — issueTitle() may not have been reached (title looks untruncated)")
	}
}

// The Next-up leg of part (a): a gate:human todo brief PICKED for Next-up
// with no decision-issue gets a NOTICE; non-todo picks are left to the
// status-wide check (no duplicates).
func TestNextUpDecisionNotices(t *testing.T) {
	_, streams := loadDCStreams(t)
	var dc *Stream
	for _, s := range streams {
		if s.Name == "dc" {
			dc = s
		}
	}
	if dc == nil {
		t.Fatal("dc stream not loaded")
	}
	var picks []Pick
	for _, b := range dc.Briefs {
		if b.Num == "04" || b.Num == "01" { // 04: gate:human todo; 01: gate:human implemented
			picks = append(picks, Pick{Stream: dc, Brief: b})
		}
	}
	got := nextUpDecisionNotices(NextUp{Picks: picks})
	if len(got) != 1 {
		t.Fatalf("nextUpDecisionNotices = %d notices, want exactly 1 (the todo pick): %v", len(got), got)
	}
	if !strings.Contains(got[0], "brief dc/04") || !strings.Contains(got[0], "picked for Next-up at todo") {
		t.Errorf("unexpected notice: %q", got[0])
	}
}
