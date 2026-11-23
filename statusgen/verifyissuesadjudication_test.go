package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// chVerifiedCard returns ch/04's verify-gate card after its README row is set
// to verified and its Evidence to ev.
func chVerifiedCard(t *testing.T, ev string) (verifyIssue, bool) {
	t.Helper()
	root := chWithEvidence(t, ev)
	readme := filepath.Join(root, "docs/streams/ch/README.md")
	const row = "| 0 | M | implemented | — | — |"
	raw := getFile(t, readme)
	if !strings.Contains(raw, row) {
		t.Fatal("fixture drifted: ch/04 row shape changed")
	}
	putFile(t, readme, strings.Replace(raw, "(./brief-04-implemented-held.md) "+row,
		"(./brief-04-implemented-held.md) | 0 | M | verified | — | — |", 1))
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, iss := range verifyIssues(root, streams, map[string]bool{}) {
		if iss.Brief == "ch/04" {
			return iss, true
		}
	}
	return verifyIssue{}, false
}

// A verified gate: human brief whose Evidence the full hold scan holds is
// routed to the adjudicated acceptance route (#4993): its card carries the
// adjudicationRoute hint, so the ordinary issuer files nothing for it, for any
// brief rather than one fixed marker. A clean brief carries no hint and its
// JSON is unchanged for adopters that never read the field.
func TestVerifyIssuesMarksHeldBriefForAdjudication(t *testing.T) {
	held := chSkippedEvidence(false)
	if ok, _ := unroutedHeldLine(held, false); !ok {
		t.Fatal("fixture drifted: the skipped-row shape must be held by the full scan")
	}
	iss, ok := chVerifiedCard(t, held)
	if !ok || !iss.AdjudicationRoute {
		t.Fatalf("held verified brief: card=%v adjudicationRoute=%v, want the routing hint", ok, iss.AdjudicationRoute)
	}
	if err := closeVerifyHeldRefusal("ch/04", "verified", held); err == nil {
		t.Fatal("the hint must match the ordinary close, which refuses this brief")
	}

	clean := chSkippedEvidence(true)
	iss, ok = chVerifiedCard(t, clean)
	if !ok || iss.AdjudicationRoute {
		t.Fatalf("clean verified brief: card=%v adjudicationRoute=%v, want an ordinary card", ok, iss.AdjudicationRoute)
	}
	out, err := json.Marshal(iss)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "adjudicationRoute") {
		t.Errorf("an ordinary card's JSON must omit the hint: %s", out)
	}
}
