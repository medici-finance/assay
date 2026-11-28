package deskkit

import (
	"errors"
	"strings"
	"testing"
)

// fakeIssueGetter answers GetIssue from a fixed issue or error.
type fakeIssueGetter struct {
	issue *Issue
	err   error
	calls int
}

func (g *fakeIssueGetter) GetIssue(ForgeRepo, int) (*Issue, error) {
	g.calls++
	return g.issue, g.err
}

var holdRepo = ForgeRepo{Owner: "example-org", Name: "example"}

func TestRereadDecisionHold(t *testing.T) {
	cases := []struct {
		name  string
		get   IssueGetter
		issue int
		want  DecisionHoldState
		why   string
	}{
		{"closed human-decided lifts", &fakeIssueGetter{issue: &Issue{Number: 9, State: "closed", Labels: []string{"needs-decision", "Human-Decided"}}}, 9, DecisionHoldLifted, "closed with human-decided"},
		{"open keeps", &fakeIssueGetter{issue: &Issue{Number: 9, State: "open", Labels: []string{"human-decided"}}}, 9, DecisionHoldLive, "is open"},
		{"closed without label keeps", &fakeIssueGetter{issue: &Issue{Number: 9, State: "closed", Labels: []string{"needs-decision"}}}, 9, DecisionHoldLive, "closed without human-decided"},
		{"read error is could-not-check", &fakeIssueGetter{err: errors.New("forge down")}, 9, DecisionHoldUnknown, "forge down"},
		{"missing issue is could-not-check", &fakeIssueGetter{}, 9, DecisionHoldUnknown, "could not be read"},
		{"change number is could-not-check", &fakeIssueGetter{issue: &Issue{Number: 9, State: "closed", IsPullRequest: true, Labels: []string{"human-decided"}}}, 9, DecisionHoldUnknown, "is a change"},
		{"unknown state is could-not-check", &fakeIssueGetter{issue: &Issue{Number: 9, State: "locked", Labels: []string{"human-decided"}}}, 9, DecisionHoldUnknown, "unrecognised state"},
		{"no issue number is could-not-check", &fakeIssueGetter{}, 0, DecisionHoldUnknown, "names no decision issue"},
		{"no forge is could-not-check", nil, 9, DecisionHoldUnknown, "no forge"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RereadDecisionHold(c.get, holdRepo, c.issue)
			if got.State != c.want {
				t.Fatalf("state = %v, want %v (%s)", got.State, c.want, got.Why)
			}
			if !strings.Contains(got.Why, c.why) {
				t.Errorf("why = %q, want it to contain %q", got.Why, c.why)
			}
			if got.Holds() != (c.want != DecisionHoldLifted) {
				t.Errorf("Holds() = %v for state %v", got.Holds(), got.State)
			}
		})
	}
}

func TestParseIssueRef(t *testing.T) {
	fr, n, err := ParseIssueRef(" example-org/example#12 ")
	if err != nil || fr != holdRepo || n != 12 {
		t.Fatalf("ParseIssueRef = %v %d %v", fr, n, err)
	}
	for _, bad := range []string{"#12", "example#12", "example-org/example", "example-org/example#", "example-org/example#0", "example-org/example#-3", "example-org/example#+3", "example-org/example#1x"} {
		if _, _, err := ParseIssueRef(bad); err == nil {
			t.Errorf("ParseIssueRef(%q) accepted", bad)
		}
	}
}
