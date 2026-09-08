package main

// noncommit_reresolve_test.go — the re-review trigger must ALSO fire on a finding-relevant
// NON-COMMIT resolution (a `*:skip` resolution label added, or a body/title edit) after the
// last review, not on head-sha alone.
//
// The defect: pr-review-desk's RE-REVIEW classifier keys on the PR HEAD SHA. A worker who
// resolves a finding WITHOUT a commit — by adding `changelog:skip`, or editing the PR body —
// moves no head, so the standing CHANGES_REQUESTED at that same head keeps the row BLOCKED
// forever and the fix is never re-examined. These tests fail first against the pre-fix
// classifier (blocking-at-head always → BLOCKED) and the pre-fix reduceReviews (no
// lastReviewAt baseline).

import (
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestClassify_NonCommitResolution_ReFlags — the pure classifier. A blocking row with a
// detected non-commit resolution re-flags RE-REVIEW; without it, it stays BLOCKED; and a
// suspected forged no-op flip is unaffected (that arm wins, never masked by this signal).
func TestClassify_NonCommitResolution_ReFlags(t *testing.T) {
	cases := []struct {
		name string
		in   classifyInput
		want string
	}{
		{
			name: "blocking + non-commit resolution → RE-REVIEW",
			in:   classifyInput{ever: true, atHead: true, blocking: true, draft: true, nonCommitResolution: true},
			want: actReReview,
		},
		{
			name: "blocking, no non-commit resolution → BLOCKED (unchanged)",
			in:   classifyInput{ever: true, atHead: true, blocking: true, draft: true},
			want: actBlocked,
		},
		{
			// A suspected forged no-op flip must NOT be laundered into RE-REVIEW by the
			// non-commit signal — the suspectNoOp arm precedes it and must still win.
			name: "blocking + suspectNoOp + non-commit resolution → SUSPECT-APPROVAL",
			in:   classifyInput{ever: true, atHead: true, blocking: true, suspectNoOp: true, nonCommitResolution: true},
			want: actSuspectApproval,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, note := classify(c.in)
			if got != c.want {
				t.Fatalf("classify() = %q, want %q (note: %s)", got, c.want, note)
			}
		})
	}
}

// TestDetectNonCommitResolution drives the detector over the label-event + body-edit
// signals, with ghRun stubbed to serve the issues-events payload.
func TestDetectNonCommitResolution(t *testing.T) {
	const repo = "example-org/tracker"
	reviewAt := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	before := reviewAt.Add(-2 * time.Hour).Format(time.RFC3339)
	after := reviewAt.Add(2 * time.Hour).Format(time.RFC3339)

	// stubEvents installs the fake forge with a ListLabelEvents hook returning one `labeled`
	// event at the given time — the typed replacement for the old issues/events gh stub.
	stubEvents := func(t *testing.T, name, createdAt string) {
		installFakeForge(t)
		forgeHooks.labelEvents = func(string, int) ([]deskkit.LabelEvent, error) {
			return []deskkit.LabelEvent{{Name: name, CreatedAt: createdAt}}, nil
		}
	}

	labelPR := func() prBase {
		var p prBase
		p.Number = 341
		p.Labels = []struct {
			Name string `json:"name"`
		}{{Name: "changelog:skip"}}
		return p
	}

	t.Run("resolution label added AFTER the last review → re-flag", func(t *testing.T) {
		stubEvents(t, "changelog:skip", after)
		got, note, err := detectNonCommitResolution(repo, labelPR(), reviewAt)
		if err != nil {
			t.Fatal(err)
		}
		if !got {
			t.Fatalf("a resolution label added after the last review must re-flag; note=%q", note)
		}
	})

	t.Run("resolution label added BEFORE the last review → no re-flag", func(t *testing.T) {
		stubEvents(t, "changelog:skip", before)
		got, _, err := detectNonCommitResolution(repo, labelPR(), reviewAt)
		if err != nil {
			t.Fatal(err)
		}
		if got {
			t.Fatal("a label already present at review time is not a post-review resolution")
		}
	})

	t.Run("body edited AFTER the last review → re-flag", func(t *testing.T) {
		// No resolution label present, so no timeline fetch happens; any gh call is a bug.
		installFakeForge(t)
		forgeHooks.labelEvents = func(string, int) ([]deskkit.LabelEvent, error) {
			t.Fatal("no label-events fetch expected on this path")
			return nil, nil
		}
		var p prBase
		p.Number = 323
		p.LastEditedAt = after
		got, note, err := detectNonCommitResolution(repo, p, reviewAt)
		if err != nil {
			t.Fatal(err)
		}
		if !got {
			t.Fatalf("a body edit after the last review must re-flag; note=%q", note)
		}
	})

	t.Run("unrelated no-op (no label, body edited BEFORE review) → no re-flag", func(t *testing.T) {
		installFakeForge(t)
		forgeHooks.labelEvents = func(string, int) ([]deskkit.LabelEvent, error) {
			t.Fatal("no label-events fetch expected on this path")
			return nil, nil
		}
		var p prBase
		p.Number = 500
		p.LastEditedAt = before
		got, _, err := detectNonCommitResolution(repo, p, reviewAt)
		if err != nil {
			t.Fatal(err)
		}
		if got {
			t.Fatal("no post-review label add and no post-review body edit must NOT re-flag")
		}
	})

	t.Run("no review baseline (zero time) → no re-flag", func(t *testing.T) {
		installFakeForge(t)
		forgeHooks.labelEvents = func(string, int) ([]deskkit.LabelEvent, error) {
			t.Fatal("no label-events fetch expected on this path")
			return nil, nil
		}
		p := labelPR()
		p.LastEditedAt = after
		got, _, err := detectNonCommitResolution(repo, p, time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		if got {
			t.Fatal("a zero review baseline can never manufacture a re-review")
		}
	})
}

// TestReduceReviews_CapturesLastReviewAt — a standing CHANGES_REQUESTED at head records its
// submitted time as lastReviewAt, the baseline the non-commit detector compares against.
func TestReduceReviews_CapturesLastReviewAt(t *testing.T) {
	head := "deadbeefcafe"
	var r review
	r.User.Login = reviewerBotDisplay()
	r.State = "CHANGES_REQUESTED"
	r.CommitID = head
	r.SubmittedAt = "2026-09-02T12:00:00Z"

	st := reduceReviews([]review{r}, head)
	if !st.blocking {
		t.Fatal("expected blocking=true")
	}
	want := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	if !st.lastReviewAt.Equal(want) {
		t.Fatalf("lastReviewAt = %v, want %v", st.lastReviewAt, want)
	}
}
