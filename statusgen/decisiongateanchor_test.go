package main

import (
	"strings"
	"testing"
)

// Tests for the THIRD human-stamp corroboration anchor (the house tracker's ruling
// (Option 1: a linked decision issue closed by the blessed login corroborates)): a
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

// DR-<slug>.md decision-record placeholders (house placeholder names). A DR's
// `decided-by:` names the approving human, but the human act is CLOSING the linked
// needs-decision issue — so the DR anchor corroborates a concrete `human:<name>`
// stamp found in the DR file against that closed issue, exactly as the brief anchor
// does for a brief-file stamp.
const (
	dgDRFile   = "docs/streams/decisions/DR-example-choice-rec.md" // -> record id "DR-example-choice-rec"
	dgDRMarker = "<!-- decision-gate: DR-example-choice-rec -->"   // the marker for THIS record
)

// TestDecisionGateAnchor_Table pins BOTH the new DR-<slug>.md anchor AND the original
// brief-<NN>.md anchor through the one pure core, and pins that each of conditions
// (a)/(b)/(c) is independently required for the DR form. Each row drives
// corroborateStamps with EMPTY PR data, so only the third anchor can decide the
// verdict. Under the fixture roster: bless login "ada"; stamp name "alex" -> login
// "ada".
func TestDecisionGateAnchor_Table(t *testing.T) {
	cases := []struct {
		name  string
		stamp stamp
		gates decisionGateLinks
		want  verdict
		// evidenceContains, when non-empty, must appear in the corroborated evidence.
		evidenceContains string
	}{
		{
			// NEW FORM, positive control: all three conditions hold for a DR record.
			name:  "DR record, all three satisfied",
			stamp: stamp{Name: "alex", File: dgDRFile},
			gates: decisionGateLinks{dgDRFile: {{
				Ref:      "o/r#88",
				ClosedBy: "ada",
				Body:     "Ratified: Option A.\n\n" + dgDRMarker + "\n",
			}}},
			want:             verdictCorroborated,
			evidenceContains: "o/r#88",
		},
		{
			// OLD FORM still works: a brief-file stamp corroborates unchanged.
			name:  "brief file, all three satisfied",
			stamp: stamp{Name: "alex", File: dgBriefFile},
			gates: decisionGateLinks{dgBriefFile: {{
				Ref:      "o/r#77",
				ClosedBy: "ada",
				Body:     "Ratified: Option A.\n\n" + dgMarker + "\n",
			}}},
			want:             verdictCorroborated,
			evidenceContains: "o/r#77",
		},
		{
			// (a) required for a DR: a non-blessed closer does not ratify.
			name:  "DR record, wrong closer",
			stamp: stamp{Name: "alex", File: dgDRFile},
			gates: decisionGateLinks{dgDRFile: {{
				Ref:      "o/r#88",
				ClosedBy: "someone-else",
				Body:     "Ratified: Option A.\n\n" + dgDRMarker + "\n",
			}}},
			want: verdictMissing,
		},
		{
			// (b) required for a DR: the marker names a DIFFERENT record.
			name:  "DR record, marker for a different record",
			stamp: stamp{Name: "alex", File: dgDRFile},
			gates: decisionGateLinks{dgDRFile: {{
				Ref:      "o/r#88",
				ClosedBy: "ada",
				Body:     "Ratified: Option A.\n\n<!-- decision-gate: DR-other-choice-rec -->\n",
			}}},
			want: verdictMissing,
		},
		{
			// (b) required, cross-form: a brief marker must NOT ratify a DR record even
			// under the DR's own key (the two id namespaces are distinct).
			name:  "DR record, brief marker does not ratify it",
			stamp: stamp{Name: "alex", File: dgDRFile},
			gates: decisionGateLinks{dgDRFile: {{
				Ref:      "o/r#88",
				ClosedBy: "ada",
				Body:     "Ratified: Option A.\n\n" + dgMarker + "\n",
			}}},
			want: verdictMissing,
		},
		{
			// (c) required for a DR: a qualifying issue exists but is linked by a
			// DIFFERENT record's key, so this DR stamp has no linked issue of its own.
			name:  "DR record, unlinked issue",
			stamp: stamp{Name: "alex", File: dgDRFile},
			gates: decisionGateLinks{"docs/streams/decisions/DR-some-other-record.md": {{
				Ref:      "o/r#88",
				ClosedBy: "ada",
				Body:     "Ratified: Option A.\n\n" + dgDRMarker + "\n",
			}}},
			want: verdictMissing,
		},
		{
			// N/A file: a stamp on a file that is neither a brief nor a DR record is
			// outside the third anchor entirely — MISSING with no PR anchor to fall to.
			name:  "non-brief non-DR file",
			stamp: stamp{Name: "alex", File: "docs/streams/decisions/README.md"},
			gates: decisionGateLinks{"docs/streams/decisions/README.md": {{
				Ref:      "o/r#88",
				ClosedBy: "ada",
				Body:     "Ratified: Option A.\n\n" + dgDRMarker + "\n",
			}}},
			want: verdictMissing,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results := corroborateStamps([]stamp{tc.stamp}, &ghPRData{}, "o/r", 1, tc.gates)
			if len(results) != 1 {
				t.Fatalf("got %d results, want 1", len(results))
			}
			if results[0].Verdict != tc.want {
				t.Fatalf("verdict = %v, want %v — evidence: %s",
					results[0].Verdict, tc.want, results[0].Evidence)
			}
			if tc.evidenceContains != "" && !strings.Contains(results[0].Evidence, tc.evidenceContains) {
				t.Errorf("evidence should contain %q, got: %s", tc.evidenceContains, results[0].Evidence)
			}
		})
	}
}

// TestDecisionRecordID pins the DR-<slug>.md basename recognition (and the brief and
// non-record files it must reject).
func TestDecisionRecordID(t *testing.T) {
	cases := []struct {
		path   string
		wantID string
		wantOK bool
	}{
		{"docs/streams/decisions/DR-example-choice-rec.md", "DR-example-choice-rec", true},
		{"DR-example-choice-rec.md", "DR-example-choice-rec", true}, // dir is not constrained, as with briefs
		{"docs/streams/decisions/DR-short.md", "", false},           // slug too short for decisionIDRe
		{"docs/streams/sdlc/brief-05-design-gate.md", "", false},    // a brief is not a DR
		{"docs/streams/decisions/README.md", "", false},
		{"docs/streams/decisions/DR-example-choice-rec.txt", "", false},
	}
	for _, tc := range cases {
		id, ok := decisionRecordID(tc.path)
		if ok != tc.wantOK || id != tc.wantID {
			t.Errorf("decisionRecordID(%q) = (%q, %v), want (%q, %v)", tc.path, id, ok, tc.wantID, tc.wantOK)
		}
	}
}

// TestDecisionGateAnchorID pins that the combined resolver returns a brief id for a
// brief file, a DR id for a DR record, and nothing for anything else — and that the
// two id namespaces are distinguishable (a brief id carries "/", a DR id does not).
func TestDecisionGateAnchorID(t *testing.T) {
	if id, ok := decisionGateAnchorID(dgBriefFile); !ok || id != "sdlc/05" {
		t.Errorf("brief: got (%q, %v), want (\"sdlc/05\", true)", id, ok)
	}
	if id, ok := decisionGateAnchorID(dgDRFile); !ok || id != "DR-example-choice-rec" {
		t.Errorf("DR: got (%q, %v), want (\"DR-example-choice-rec\", true)", id, ok)
	}
	if id, ok := decisionGateAnchorID("docs/streams/decisions/README.md"); ok {
		t.Errorf("non-record: got (%q, %v), want (\"\", false)", id, ok)
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
