package main

import (
	"os"
	"strings"
	"testing"
)

// regression: milestone-support/11 — the generated region is deterministic,
// separate from the Briefs region, preserves authored prose, escapes hostile
// record text and is refused on a stream that did not opt in.
func TestMilestoneReadmeRegion(t *testing.T) {
	opted, legacy := milestoneStreams(t)
	path := opted.Dir + "/README.md"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	authored := string(raw) + "\nAuthored prose stays.\n\n" + briefsMarkerBegin + "\n| # | Brief |\n" + briefsMarkerEnd + "\n\n## Milestones\n\n" + milestonesMarkerBegin + "\nstale\n" + milestonesMarkerEnd + "\n\nTail prose.\n"
	if err := os.WriteFile(path, []byte(authored), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := rewriteMilestoneRegion(opted, path)
	if err != nil || !changed {
		t.Fatalf("first rewrite: changed=%v err=%v", changed, err)
	}
	got, _ := os.ReadFile(path)
	for _, keep := range []string{"Authored prose stays.", "Tail prose.", briefsMarkerBegin + "\n| # | Brief |\n" + briefsMarkerEnd} {
		if !strings.Contains(string(got), keep) {
			t.Errorf("authored or Briefs content lost: %q", keep)
		}
	}
	for _, want := range []string{"assay:assay:delivery:m-03@1", "Membership", "assay:assay:delivery:b-11"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("region lacks %q:\n%s", want, got)
		}
	}
	for _, banned := range []string{"work remaining |", "supported |", "accepted |"} {
		if strings.Contains(string(got), banned) {
			t.Errorf("region carries a hand-maintained status column %q", banned)
		}
	}
	if changed, err := rewriteMilestoneRegion(opted, path); err != nil || changed {
		t.Fatalf("second rewrite must be a no-op: changed=%v err=%v", changed, err)
	}
	if p := checkMilestoneRegions([]*Stream{opted}); len(p) != 0 {
		t.Errorf("fresh region reported drift: %v", p)
	}
	hand := strings.Replace(string(got), "Coverage: known", "Coverage: known and finished", 1)
	if err := os.WriteFile(path, []byte(hand), 0o644); err != nil {
		t.Fatal(err)
	}
	if p := checkMilestoneRegions([]*Stream{opted}); len(p) != 1 || !strings.Contains(p[0], "hand edit") {
		t.Errorf("hand edit not reported: %v", p)
	}

	// A stream that did not opt in carries no markers; with markers it is refused,
	// and without them it is byte-identical.
	lp := legacy.Dir + "/README.md"
	before, _ := os.ReadFile(lp)
	if changed, err := rewriteMilestoneRegion(legacy, lp); err != nil || changed {
		t.Fatalf("legacy stream without markers must be untouched: %v %v", changed, err)
	}
	after, _ := os.ReadFile(lp)
	if string(before) != string(after) {
		t.Error("legacy README changed")
	}
	if err := os.WriteFile(lp, []byte(string(before)+"\n"+milestonesMarkerBegin+"\n"+milestonesMarkerEnd+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := rewriteMilestoneRegion(legacy, lp); err == nil || !strings.Contains(err.Error(), "not opted in") && !strings.Contains(err.Error(), "has not opted in") {
		t.Errorf("markers on a legacy stream must be refused: %v", err)
	}
	if p := checkMilestoneRegions([]*Stream{legacy}); len(p) != 1 || !strings.Contains(p[0], "has not opted in") {
		t.Errorf("lint must refuse fake milestones: %v", p)
	}

	// Hostile record text cannot break the table or open a heading.
	hostile := &Stream{Name: "x", Milestones: &MilestonePlan{OptedIn: true, Coverage: MilestoneCoverageKnown,
		Records: []MilestoneRecord{{Key: "a:b:c:m-01", ContractVersion: 1, Title: "a|b `c`\n# Heading\n<script>"}}}}
	out := renderMilestoneRegion(hostile)
	if strings.Contains(out, "\n# Heading") {
		t.Errorf("record title opened a heading:\n%s", out)
	}
	if !strings.Contains(out, `a\|b`) {
		t.Errorf("pipe in a title was not escaped:\n%s", out)
	}
}

// regression: an overlapping milestone region — a region that starts inside
// the Briefs region, encloses it or ends inside it is malformed in both regen
// and lint, and regen leaves the README (and its Briefs table) untouched.
func TestMilestoneRegionOverlap(t *testing.T) {
	opted, _ := milestoneStreams(t)
	path := opted.Dir + "/README.md"
	base, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mb, me := milestonesMarkerBegin+"\nstale\n", milestonesMarkerEnd+"\n"
	bb, be := briefsMarkerBegin+"\n| # | Brief |\n", briefsMarkerEnd+"\n"
	layouts := map[string]string{
		"begin-inside":  bb + mb + me + be,
		"enclosing":     mb + bb + be + me,
		"overlap-end":   mb + bb + me + be,
		"overlap-start": bb + mb + be + me,
	}
	for name, body := range layouts {
		content := string(base) + "\n" + body
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		changed, err := rewriteMilestoneRegion(opted, path)
		if err == nil || changed || !strings.Contains(err.Error(), "malformed milestone region") {
			t.Errorf("%s: regen must refuse an overlapping region: changed=%v err=%v", name, changed, err)
		}
		if got, _ := os.ReadFile(path); string(got) != content {
			t.Errorf("%s: regen wrote the README despite the overlap", name)
		}
		if p := checkMilestoneRegions([]*Stream{opted}); len(p) != 1 || !strings.Contains(p[0], "malformed milestone region") {
			t.Errorf("%s: lint must report the overlap as malformed: %v", name, p)
		}
	}
	// Control: the same markers with the regions apart are accepted.
	for name, body := range map[string]string{"before": mb + me + bb + be, "after": bb + be + mb + me} {
		if err := os.WriteFile(path, []byte(string(base)+"\n"+body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := rewriteMilestoneRegion(opted, path); err != nil {
			t.Errorf("%s: separate regions refused: %v", name, err)
		}
	}
}
