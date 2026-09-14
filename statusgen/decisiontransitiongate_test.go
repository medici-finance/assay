package main

import (
	"strings"
	"testing"
)

// Tests for brief-18 (Make the human gate BINDING) Tasks 1+2: the pure core that decides
// whether a gate:human brief's status-table row may move to implemented/verified. The
// fixture roster (rosterfixture_test.go) blesses login "ada" — that is the "driver".

// dtIssue builds a *decisionIssueState with the given open/closed flag and comments,
// each comment given as "author:body".
func dtIssue(ref string, open bool, comments ...string) *decisionIssueState {
	iss := &decisionIssueState{Ref: ref, Open: open}
	for _, c := range comments {
		author, body, _ := strings.Cut(c, ":")
		iss.Comments = append(iss.Comments, decisionIssueComment{Author: author, Body: body})
	}
	return iss
}

func humanBrief(id string, decisionIssue int) *BriefFile {
	return &BriefFile{Brief: id, Gate: "human", DecisionIssue: decisionIssue}
}

// TestDecisionTransitionRefusal_Table drives the Verify table's rows 1-5 and 9 directly
// against the pure core, each row a fixture in miniature (constructed BriefFile + status +
// pre-fetched issue state, no fixtures on disk, no network).
func TestDecisionTransitionRefusal_Table(t *testing.T) {
	cases := []struct {
		name       string
		bf         *BriefFile
		status     string
		iss        *decisionIssueState
		unreadable bool
		wantRefuse bool
	}{
		{
			// Verify row 1: OPEN, unruled (no comments at all) -> REFUSED.
			name:       "row1 open unruled -> refused",
			bf:         humanBrief("dc/20", 501),
			status:     "implemented",
			iss:        dtIssue("o/r#501", true),
			wantRefuse: true,
		},
		{
			// Verify row 2: same issue, now carrying a driver (blessed login "ada")
			// comment -> ALLOWED, even though still open (closing is not required).
			name:       "row2 driver ruling recorded -> allowed",
			bf:         humanBrief("dc/20", 501),
			status:     "implemented",
			iss:        dtIssue("o/r#501", true, "ada:Ratified. Proceed as briefed."),
			wantRefuse: false,
		},
		{
			// Verify row 3: NO decision issue at all (DecisionIssue == 0) -> REFUSED.
			// Silence must not read as permission.
			name:       "row3 no decision issue at all -> refused",
			bf:         humanBrief("dc/21", 0),
			status:     "implemented",
			iss:        nil,
			wantRefuse: true,
		},
		{
			// Verify row 4a: the issue carries ONLY a desk/bot relay ("assay-desk-app"),
			// never a comment from the blessed driver login -> REFUSED.
			name:       "row4a desk relay only -> refused",
			bf:         humanBrief("dc/22", 502),
			status:     "verified",
			iss:        dtIssue("o/r#502", true, "assay-desk-app:Ian said in standup: ship it."),
			wantRefuse: true,
		},
		{
			// Verify row 4b: perturb the SAME issue into a real driver ruling -> row 2's
			// shape now applies and it goes ALLOWED. Proves the detector distinguishes a
			// relay from the real thing rather than merely reacting to comment count.
			name:   "row4b perturbed into a real driver ruling -> allowed",
			bf:     humanBrief("dc/22", 502),
			status: "verified",
			iss: dtIssue("o/r#502", true,
				"assay-desk-app:Ian said in standup: ship it.",
				"ada:Confirmed — proceed."),
			wantRefuse: false,
		},
		{
			// Verify row 5: gate: model is NOT gated by this check at all, regardless of
			// decision-issue state -- scoped to gate: human only.
			name:   "row5 gate model -> never refused",
			bf:     &BriefFile{Brief: "dc/23", Gate: "model", DecisionIssue: 0},
			status: "implemented",
			iss:    nil,
			// no wantRefuse override needed (defaults to false)
		},
		{
			// Verify row 9: replay drain-harness/08's shape -- a bare status-cell flip to
			// implemented with the decision issue OPEN (and no ruling at all) -> REFUSED.
			name:       "row9 drain-harness/08 replay -> refused",
			bf:         humanBrief("dc/24", 599),
			status:     "implemented",
			iss:        dtIssue("o/r#599", true),
			wantRefuse: true,
		},
		{
			// Scope check: todo/in-progress/done are NOT guarded by this transition block
			// (done carries its own, stricter, pre-existing human-review requirement; and
			// in-progress is the design-approval gate's territory, not this one's).
			name:       "in-progress is not guarded",
			bf:         humanBrief("dc/25", 0),
			status:     "in-progress",
			iss:        nil,
			wantRefuse: false,
		},
		{
			// A closed-but-unruled issue still refuses: closing alone is not a ruling.
			name:       "closed with no driver comment -> still refused",
			bf:         humanBrief("dc/26", 503),
			status:     "verified",
			iss:        dtIssue("o/r#503", false, "assay-desk-app:Closing as ratified."),
			wantRefuse: true,
		},
		{
			// A could-not-check read (network failure) is refused, never rounded up to a
			// pass -- C4, and distinct wording from "open, unruled".
			name:       "issue state unreadable -> refused (could-not-check is never a pass)",
			bf:         humanBrief("dc/27", 504),
			status:     "implemented",
			iss:        nil,
			unreadable: true,
			wantRefuse: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg, refuse := decisionTransitionRefusal(tc.bf, tc.status, tc.iss, tc.unreadable)
			if refuse != tc.wantRefuse {
				t.Fatalf("decisionTransitionRefusal(%s, %q) refuse = %v, want %v (msg=%q)",
					tc.bf.Brief, tc.status, refuse, tc.wantRefuse, msg)
			}
			if refuse && !strings.Contains(msg, tc.bf.Brief) {
				t.Errorf("refusal message must name the brief %q, got: %s", tc.bf.Brief, msg)
			}
		})
	}
}

// TestDecisionRuled pins the "ruled" predicate in isolation: only a comment from the
// blessed driver login counts, case-insensitively; a nil issue, an issue with no
// comments, and a bot/relay-only issue all come back unruled.
func TestDecisionRuled(t *testing.T) {
	if _, ruled := decisionRuled(nil); ruled {
		t.Errorf("nil issue must not be ruled")
	}
	if _, ruled := decisionRuled(dtIssue("o/r#1", true)); ruled {
		t.Errorf("an issue with no comments must not be ruled")
	}
	if _, ruled := decisionRuled(dtIssue("o/r#1", true, "assay-desk-app:relaying the driver's verdict")); ruled {
		t.Errorf("a desk/bot relay must not be ruled")
	}
	if _, ruled := decisionRuled(dtIssue("o/r#1", true, "ADA:Ratified (case-insensitive login match)")); !ruled {
		t.Errorf("a driver comment must be ruled regardless of login case")
	}
	if evidence, ruled := decisionRuled(dtIssue("o/r#1", true, "other:noise", "ada:Ratified.")); !ruled || !strings.Contains(evidence, "o/r#1") {
		t.Errorf("a driver comment among others must still be ruled and cite the issue; got ruled=%v evidence=%q", ruled, evidence)
	}
}

// TestDecisionTransitionGateProblems_FullCorpus exercises the corpus-walking wrapper
// against the REAL on-disk "dc" fixture tree (decisionissues_test.go's loadDCStreams),
// with statusOverride == nil so it reads each brief's CURRENT README-row status directly
// -- the fixture-file equivalent of "a brief already sitting at implemented/verified".
func TestDecisionTransitionGateProblems_FullCorpus(t *testing.T) {
	_, streams := loadDCStreams(t)

	fetch := func(bf *BriefFile) (*decisionIssueState, bool) {
		if bf.DecisionIssue == 999 {
			// dc/10's fixture decision-issue: OPEN, no driver comment -> still refused.
			return dtIssue("o/r#999", true), false
		}
		return nil, true // any other issue number: simulate unreadable
	}

	got := decisionTransitionGateProblems(streams, nil, fetch)

	mustContainBrief := func(id string) {
		for _, p := range got {
			if strings.Contains(p, id+":") {
				return
			}
		}
		t.Errorf("expected a PROBLEM naming %s; got: %v", id, got)
	}
	mustNotContainBrief := func(id string) {
		for _, p := range got {
			if strings.Contains(p, id+":") {
				t.Errorf("did not expect a PROBLEM naming %s; got: %v", id, got)
			}
		}
	}

	// dc/01: gate:human, implemented, no decision issue at all -> refused (row 3 shape).
	mustContainBrief("dc/01")
	// dc/02: gate:human, verified, no decision issue at all -> refused (row 3 shape).
	mustContainBrief("dc/02")
	// dc/03: gate:model, implemented -> never refused, regardless of anything (row 5 shape).
	mustNotContainBrief("dc/03")
	// dc/04: gate:human, todo -> not a guarded status at all.
	mustNotContainBrief("dc/04")
	// dc/05: gate:human, done -> not a guarded status (done has its own, stricter rule).
	mustNotContainBrief("dc/05")
	// dc/10: gate:human, implemented, decision-issue 999 (open, unruled) -> refused.
	mustContainBrief("dc/10")
}

// TestTransitionsInDiff pins the pure diff-scoping helper: it extracts brief-id -> new
// Status cell for every ADDED status-table row, and ignores everything else (context
// lines, removed lines, non-table prose).
func TestTransitionsInDiff(t *testing.T) {
	diff := "diff --git a/docs/streams/dc/README.md b/docs/streams/dc/README.md\n" +
		"--- a/docs/streams/dc/README.md\n" +
		"+++ b/docs/streams/dc/README.md\n" +
		"@@ -10,1 +10,1 @@\n" +
		"-| 20 | [Some brief](brief-20-x.md) | 0 | M | todo | — | — |\n" +
		"+| 20 | [Some brief](brief-20-x.md) | 0 | M | implemented | — | — |\n" +
		"+not a table row, just prose\n"

	got := transitionsInDiff(diff)
	if got["dc/20"] != "implemented" {
		t.Fatalf("transitionsInDiff = %v, want dc/20 -> implemented", got)
	}
	if len(got) != 1 {
		t.Fatalf("transitionsInDiff should find exactly one transition, got %v", got)
	}
}

// TestTransitionsInDiff_Empty pins that a diff touching NOTHING status-table-shaped
// yields an empty map, never a false positive.
func TestTransitionsInDiff_Empty(t *testing.T) {
	diff := "diff --git a/README.md b/README.md\n+++ b/README.md\n+just some prose, no table\n"
	got := transitionsInDiff(diff)
	if len(got) != 0 {
		t.Fatalf("transitionsInDiff on a non-table diff = %v, want empty", got)
	}
}
