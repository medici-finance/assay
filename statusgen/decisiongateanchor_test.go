package main

import (
	"strings"
	"testing"
)

// Tests for the THIRD human-stamp corroboration anchor (tracker ruling #2237): a
// linked, blessed-human-CLOSED needs-decision issue carrying this brief's per-brief
// decision-gate marker. Under the fixture roster (rosterfixture_test.go):
//   ASSAY_BLESS_LOGIN=ada:100001   -> the blessed closer is "ada"
//   ASSAY_HUMAN_LOGIN_MAP=alex:ada -> the stamp name "alex" resolves to login "ada"
//
// Each test drives corroborateStamps with EMPTY PR data (&ghPRData{}), so neither PR
// anchor can fire — the verdict is decided by the third anchor alone. That isolation
// is the point: it proves the third path, and that removing it (its one call in
// corroborateStamps) reverts the positive case to MISSING-CORROBORATION.

const (
	dgBriefFile = "docs/streams/sdlc/brief-05-design-gate.md" // -> brief id "sdlc/05"
	dgMarker    = "<!-- decision-gate: sdlc/05 -->"           // the marker for THIS brief
)

func dgStamp() []stamp { return []stamp{{Name: "alex", File: dgBriefFile}} }

// POSITIVE CONTROL: all three conditions satisfied -> CORROBORATED.
//
// A needs-decision issue the brief LINKS (present under the brief's key), CLOSED by
// the blessed human "ada", carrying THIS brief's decision-gate marker.
func TestDecisionGateAnchor_AllThreeSatisfied_Corroborated(t *testing.T) {
	gates := decisionGateLinks{
		dgBriefFile: {{
			Ref:      "o/r#77",
			ClosedBy: "ada",
			Body:     "Ratified: Option A.\n\n" + dgMarker + "\n",
		}},
	}
	results := corroborateStamps(dgStamp(), &ghPRData{}, "o/r", 1, gates)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Verdict != verdictCorroborated {
		t.Fatalf("verdict = %v, want CORROBORATED (all three conditions held) — evidence: %s",
			results[0].Verdict, results[0].Evidence)
	}
	if !strings.Contains(results[0].Evidence, "o/r#77") {
		t.Errorf("evidence should cite the corroborating issue, got: %s", results[0].Evidence)
	}
}

// CONDITION (a) REQUIRED: wrong closer -> MISSING.
//
// The issue is linked and carries the correct marker, but it was CLOSED by a
// non-ASSAY_BLESS_LOGIN login. Only the blessed human's close ratifies.
func TestDecisionGateAnchor_WrongCloser_Missing(t *testing.T) {
	gates := decisionGateLinks{
		dgBriefFile: {{
			Ref:      "o/r#77",
			ClosedBy: "someone-else", // not the bless login "ada"
			Body:     "Ratified: Option A.\n\n" + dgMarker + "\n",
		}},
	}
	results := corroborateStamps(dgStamp(), &ghPRData{}, "o/r", 1, gates)
	if results[0].Verdict != verdictMissing {
		t.Fatalf("verdict = %v, want MISSING-CORROBORATION (closed by a non-blessed login) — evidence: %s",
			results[0].Verdict, results[0].Evidence)
	}
}

// CONDITION (b) REQUIRED: marker names a DIFFERENT brief -> MISSING.
//
// The issue is linked and CLOSED by the blessed human, but its decision-gate marker
// names sdlc/06, not the sdlc/05 brief being corroborated. A decision issue for one
// brief must not ratify another.
func TestDecisionGateAnchor_MarkerForDifferentBrief_Missing(t *testing.T) {
	gates := decisionGateLinks{
		dgBriefFile: {{
			Ref:      "o/r#77",
			ClosedBy: "ada",
			Body:     "Ratified: Option A.\n\n<!-- decision-gate: sdlc/06 -->\n", // wrong brief
		}},
	}
	results := corroborateStamps(dgStamp(), &ghPRData{}, "o/r", 1, gates)
	if results[0].Verdict != verdictMissing {
		t.Fatalf("verdict = %v, want MISSING-CORROBORATION (marker names a different brief) — evidence: %s",
			results[0].Verdict, results[0].Evidence)
	}
}

// CONDITION (c) REQUIRED: unlinked issue -> MISSING.
//
// A qualifying issue EXISTS (blessed-closed, correct marker) but the sdlc/05 brief
// does NOT link it: it is present in the pool only under a DIFFERENT brief's key, so
// the sdlc/05 stamp has no linked issues of its own. The anchor keys on the link, so
// a real ratification of some other brief cannot leak across.
func TestDecisionGateAnchor_UnlinkedIssue_Missing(t *testing.T) {
	gates := decisionGateLinks{
		// The qualifying issue is linked by a DIFFERENT brief, never by sdlc/05.
		"docs/streams/sdlc/brief-06-other.md": {{
			Ref:      "o/r#77",
			ClosedBy: "ada",
			Body:     "Ratified: Option A.\n\n" + dgMarker + "\n",
		}},
	}
	results := corroborateStamps(dgStamp(), &ghPRData{}, "o/r", 1, gates)
	if results[0].Verdict != verdictMissing {
		t.Fatalf("verdict = %v, want MISSING-CORROBORATION (brief does not link the issue) — evidence: %s",
			results[0].Verdict, results[0].Evidence)
	}
}

// The third anchor must not disturb the two PR anchors: with no decision-gate data
// at all, an APPROVED review still corroborates exactly as before.
func TestDecisionGateAnchor_PRAnchorStillWinsWithNoGateData(t *testing.T) {
	data := makeData(
		`[{"author":{"login":"ada"},"body":"","state":"APPROVED","submittedAt":"2026-07-10T16:29:18Z","id":"PRR_1"}]`,
		`[]`,
	)
	results := corroborateStamps(dgStamp(), data, "o/r", 1, nil)
	if results[0].Verdict != verdictCorroborated {
		t.Fatalf("verdict = %v, want CORROBORATED (PR approval anchor, no gate data)", results[0].Verdict)
	}
	if !strings.Contains(results[0].Evidence, "APPROVED review") {
		t.Errorf("evidence should be the APPROVED-review anchor, got: %s", results[0].Evidence)
	}
}
