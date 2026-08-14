package main

import (
	"errors"
	"strings"
	"testing"
)

// mergedStatusFixture is the #270 shape itself: desk-hardening/01 landed via PR #255 and
// the README row never moved.
func mergedStatusFixture() []*Stream {
	return []*Stream{
		{
			Name: "desk-hardening",
			Briefs: []Brief{
				{Num: "01", Title: "three-state instrument invariant", Status: "todo"},
				{Num: "05", Title: "merge-time re-check", Status: "implemented"},
			},
		},
		{
			Name: "ground-truth",
			Briefs: []Brief{
				{Num: "05", Title: "derived surfaces", Status: "in-progress"},
				{Num: "07", Title: "desk preflight", Status: "done"},
			},
		},
	}
}

// TestMergedPRStatusNotice is the check's PROOF IT CAN FAIL: every sub-test below
// is a positive control that must produce (or must NOT produce) a NOTICE. The
// drift cases reproduce #270 literally — a merged PR naming a brief whose row is
// still todo — so a regression that stops firing turns this red rather than going
// quietly green.
func TestMergedPRStatusNotice(t *testing.T) {
	streams := mergedStatusFixture()

	t.Run("positive control: merged PR vs a todo row FIRES", func(t *testing.T) {
		merged := []mergedPR{{
			Number:  255,
			Subject: "Merge pull request #255 from medici-finance/brief/desk-hardening-01-three-state",
			Briefs:  []string{"desk-hardening/01"},
		}}
		got := mergedPRStatusNotices(streams, merged, nil)
		if len(got) != 1 {
			t.Fatalf("want exactly 1 NOTICE for the #270 shape, got %d: %v", len(got), got)
		}
		for _, want := range []string{"merged PR #255", "desk-hardening/01", `still "todo"`, "#270"} {
			if !strings.Contains(got[0], want) {
				t.Errorf("NOTICE must name %q so it is actionable; got:\n%s", want, got[0])
			}
		}
	})

	t.Run("positive control: in-progress row also FIRES", func(t *testing.T) {
		merged := []mergedPR{{
			Number:  900,
			Subject: "Merge pull request #900 from medici-finance/brief/ground-truth-05-derived",
			Briefs:  []string{"ground-truth/05"},
		}}
		if got := mergedPRStatusNotices(streams, merged, nil); len(got) != 1 {
			t.Fatalf("an in-progress row contradicted by a merge must fire; got %v", got)
		}
	})

	t.Run("implemented and done rows are silent", func(t *testing.T) {
		merged := []mergedPR{
			{Number: 1, Subject: "Merge pull request #1 from x/brief/desk-hardening-05-a", Briefs: []string{"desk-hardening/05"}},
			{Number: 2, Subject: "Merge pull request #2 from x/brief/ground-truth-07-b", Briefs: []string{"ground-truth/07"}},
		}
		if got := mergedPRStatusNotices(streams, merged, nil); len(got) != 0 {
			t.Fatalf("rows at implemented/done are past the line this check draws; got %v", got)
		}
	})

	t.Run("false-positive control: an id naming no known row is ignored", func(t *testing.T) {
		merged := []mergedPR{{
			Number:  3,
			Subject: "Merge pull request #3 from x/brief/no-such-stream-01-thing",
			Briefs:  []string{"no-such-stream/01", "desk-hardening/99"},
		}}
		if got := mergedPRStatusNotices(streams, merged, nil); len(got) != 0 {
			t.Fatalf("unknown stream/brief ids must never manufacture a finding; got %v", got)
		}
	})

	t.Run("one NOTICE per brief, newest merge named", func(t *testing.T) {
		merged := []mergedPR{
			{Number: 400, Subject: "Merge pull request #400 from x/brief/desk-hardening-01-later", Briefs: []string{"desk-hardening/01"}},
			{Number: 255, Subject: "Merge pull request #255 from x/brief/desk-hardening-01-first", Briefs: []string{"desk-hardening/01"}},
		}
		got := mergedPRStatusNotices(streams, merged, nil)
		if len(got) != 1 {
			t.Fatalf("five merges of one brief must not print five identical lines; got %d: %v", len(got), got)
		}
		if !strings.Contains(got[0], "#400") {
			t.Errorf("the most recent merge is the one to reconcile against; got:\n%s", got[0])
		}
	})

	t.Run("three-state: a failed history read is could-not-check, never clean", func(t *testing.T) {
		got := mergedPRStatusNotices(streams, nil, errors.New("fatal: not a git repository"))
		if len(got) != 1 || !strings.Contains(got[0], "could-not-check") {
			t.Fatalf("a read that could not be performed must report could-not-check; got %v", got)
		}
		if !strings.Contains(got[0], "No conclusion") {
			t.Errorf("could-not-check must say no conclusion was drawn; got:\n%s", got[0])
		}
	})

	t.Run("three-state: a failed read discards partial data", func(t *testing.T) {
		merged := []mergedPR{{Number: 255, Subject: "Merge pull request #255 from x/brief/desk-hardening-01-a", Briefs: []string{"desk-hardening/01"}}}
		got := mergedPRStatusNotices(streams, merged, errors.New("boom"))
		if len(got) != 1 || !strings.Contains(got[0], "could-not-check") {
			t.Fatalf("partial data behind a read error must not be reported as a finding; got %v", got)
		}
	})
}

// TestBriefRefsIn pins the id extraction, which is where a false positive would come
// from: every candidate here is either a real fleet branch name or a shape that has
// appeared on main and must NOT resolve.
func TestBriefRefsIn(t *testing.T) {
	cases := []struct {
		name    string
		subject string
		branch  string
		want    []string
	}{
		{
			name:    "the #270 merge itself",
			subject: "Merge pull request #255 from medici-finance/brief/desk-hardening-01-three-state",
			branch:  "brief/desk-hardening-01-three-state",
			want:    []string{"desk-hardening/01"},
		},
		{
			name:    "single-word stream",
			subject: "Merge pull request #900 from medici-finance/brief/gtm-05-evidence",
			branch:  "brief/gtm-05-evidence",
			want:    []string{"gtm/05"},
		},
		{
			name:    "explicit id in a squash subject",
			subject: "feat(ground-truth): land ground-truth/05 derived surfaces (#1234)",
			branch:  "",
			want:    []string{"ground-truth/05"},
		},
		{
			name:    "letter-suffixed brief number",
			subject: "Merge pull request #7 from x/brief/issue-loop-12a-thing",
			branch:  "brief/issue-loop-12a-thing",
			want:    []string{"issue-loop/12a"},
		},
		{
			// The live main history carries this one; a greedy split would invent
			// desk-risk-extension/05 and reconcile against a row nobody merged.
			name:    "multi-brief unified branch resolves to nothing",
			subject: "Merge pull request #835 from medici-finance/brief/desk-risk-extension-0507-unified",
			branch:  "brief/desk-risk-extension-0507-unified",
			want:    nil,
		},
		{
			name:    "a plain merge of main names no brief",
			subject: "Merge remote-tracking branch 'refs/remotes/origin/main' into shepherd/835-0507-current",
			branch:  "",
			want:    nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := briefRefsIn(tc.subject, tc.branch)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("briefRefsIn(%q, %q) = %v, want %v", tc.subject, tc.branch, got, tc.want)
			}
		})
	}
}
