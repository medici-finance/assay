package adapters

import (
	"fmt"
	"testing"
)

// stubLabelSource is a minimal IssueLabelSource for these tests: it answers
// from a fixed map and returns an error for numbers listed in missing.
type stubLabelSource struct {
	labels  map[int][]string
	missing map[int]bool
}

func (s stubLabelSource) IssueLabels(repo string, number int) ([]string, error) {
	if s.missing[number] {
		return nil, fmt.Errorf("issue #%d: not found", number)
	}
	return s.labels[number], nil
}

func TestGithubLabels_ClosedIssueNumber(t *testing.T) {
	cases := []struct {
		message string
		wantNum int
		wantOK  bool
	}{
		{"Fixes #101: stop the crash", 101, true},
		{"fixes #7", 7, true},
		{"Closes #42", 42, true},
		{"closes: #42", 42, true},
		{"this closes nothing in particular", 0, false},
		{"just a regular commit message", 0, false},
		{"related to #5 but does not close it", 0, false},
	}
	for _, tc := range cases {
		n, ok := ClosedIssueNumber(tc.message)
		if ok != tc.wantOK || (ok && n != tc.wantNum) {
			t.Errorf("ClosedIssueNumber(%q) = (%d, %v), want (%d, %v)", tc.message, n, ok, tc.wantNum, tc.wantOK)
		}
	}
}

func TestGithubLabels_IssueDefectClassed(t *testing.T) {
	source := stubLabelSource{
		labels: map[int][]string{
			101: {"bug", "priority-2"},
			102: {"enhancement"},
			103: {"HOUSE-VERDICT-LANE"},
		},
		missing: map[int]bool{999: true},
	}

	g := NewGithubLabels(source)
	if classed, err := g.IssueDefectClassed("", 101); err != nil || !classed {
		t.Errorf("expected #101 (bug-labeled) to be defect-classed, got (%v, %v)", classed, err)
	}
	if classed, err := g.IssueDefectClassed("", 102); err != nil || classed {
		t.Errorf("expected #102 (enhancement-only) to NOT be defect-classed, got (%v, %v)", classed, err)
	}
	if classed, err := g.IssueDefectClassed("", 103); err != nil || classed {
		t.Errorf("expected #103 to NOT be defect-classed without a configured verdict label, got (%v, %v)", classed, err)
	}
	if _, err := g.IssueDefectClassed("", 999); err == nil {
		t.Error("expected an error for an unresolvable issue lookup, got nil")
	}

	// A per-target configured verdict-issue label lane is generic config, not
	// a hardcoded identifier — the same #103 becomes defect-classed once its
	// label is configured.
	gWithVerdictLane := NewGithubLabels(source, "house-verdict-lane")
	if classed, err := gWithVerdictLane.IssueDefectClassed("", 103); err != nil || !classed {
		t.Errorf("expected #103 to be defect-classed once its label is configured, got (%v, %v)", classed, err)
	}
}

func TestGithubLabels_IsFixTaxonomy(t *testing.T) {
	g := NewGithubLabels(stubLabelSource{})
	cases := []struct {
		branch, title string
		want          bool
	}{
		{"fix/empty-input-crash", "", true},
		{"", "fix: crash on empty input", true},
		{"", "fix(parser): crash on empty input", true},
		{"chore/cleanup", "clean up the parser", false},
		{"feature/widget", "add the new widget", false},
	}
	for _, tc := range cases {
		if got := g.IsFixTaxonomy(tc.branch, tc.title); got != tc.want {
			t.Errorf("IsFixTaxonomy(%q, %q) = %v, want %v", tc.branch, tc.title, got, tc.want)
		}
	}
}

func TestGithubLabels_HasFixKeyword(t *testing.T) {
	g := NewGithubLabels(stubLabelSource{})
	cases := []struct {
		message string
		want    bool
	}{
		{"this also happens to fix a regression in the parser", true},
		{"fixed a subtle bug", true},
		{"add a new widget to the dashboard", false},
		{"refactor the widget renderer", false},
	}
	for _, tc := range cases {
		if got := g.HasFixKeyword(tc.message); got != tc.want {
			t.Errorf("HasFixKeyword(%q) = %v, want %v", tc.message, got, tc.want)
		}
	}
}
