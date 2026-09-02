package deskkit

import (
	"testing"
)

// prstate_test.go — the canonical reducer's edge cases. The "at head" / "stale" / "none"
// trichotomy is what stalled's stall test pivots on, and #247/#236 are exactly the
// fail-open shapes a wrong reduction here would reintroduce.

const appBot = "assay-reviewer-app[bot]"

func TestReduceAppVerdict(t *testing.T) {
	cases := []struct {
		name    string
		reviews []AppReview
		head    string
		want    AppVerdict
	}{
		{
			name: "no app reviews at all",
			head: "h1",
			want: AppVerdictNone,
		},
		{
			name: "only non-app reviews",
			reviews: []AppReview{
				{AuthorLogin: "human", State: "APPROVED", CommitID: "h1"},
			},
			head: "h1",
			want: AppVerdictNone,
		},
		{
			name: "only commented app reviews",
			reviews: []AppReview{
				{AuthorLogin: appBot, State: "COMMENTED", CommitID: "h1"},
			},
			head: "h1",
			want: AppVerdictNone,
		},
		{
			name: "changes requested at head",
			reviews: []AppReview{
				{AuthorLogin: appBot, State: "CHANGES_REQUESTED", CommitID: "h1"},
			},
			head: "h1",
			want: AppVerdictChangesRequested,
		},
		{
			name: "approved at head",
			reviews: []AppReview{
				{AuthorLogin: appBot, State: "APPROVED", CommitID: "h1"},
			},
			head: "h1",
			want: AppVerdictApproved,
		},
		{
			name: "changes requested then approved at head — latest governs",
			reviews: []AppReview{
				{AuthorLogin: appBot, State: "CHANGES_REQUESTED", CommitID: "h1"},
				{AuthorLogin: appBot, State: "APPROVED", CommitID: "h1"},
			},
			head: "h1",
			want: AppVerdictApproved,
		},
		{
			name: "approved then new push — stale (decisive exists, not at head)",
			reviews: []AppReview{
				{AuthorLogin: appBot, State: "APPROVED", CommitID: "old"},
			},
			head: "h1",
			want: AppVerdictStale,
		},
		{
			name: "changes requested at an old head — stale",
			reviews: []AppReview{
				{AuthorLogin: appBot, State: "CHANGES_REQUESTED", CommitID: "old"},
			},
			head: "h1",
			want: AppVerdictStale,
		},
		{
			// This case is carried for completeness, but note that `last.CommitID != head`
			// ALONE already yields Stale for it: "h1" != "". It does NOT pin the
			// `head == ""` clause, and must not be mistaken for the case that does — see
			// the next one.
			name: "empty head, non-empty review commit — stale (the inequality suffices)",
			reviews: []AppReview{
				{AuthorLogin: appBot, State: "CHANGES_REQUESTED", CommitID: "h1"},
			},
			head: "",
			want: AppVerdictStale,
		},
		{
			// THE case the `head == ""` clause exists for, and the only input that can
			// distinguish it. With both empty, `last.CommitID != head` is FALSE ("" ==
			// ""), so without the clause the reduction falls through to the state switch
			// and answers ChangesRequested — treating a review as blocking AT a head that
			// was never read. An unreadable head must never collapse into "the verdict is
			// current"; it is Stale, and a consumer gated on IsBlockingAtHead then
			// declines to act on evidence it does not have.
			name: "empty head AND empty review commit — stale, never blocking-at-head",
			reviews: []AppReview{
				{AuthorLogin: appBot, State: "CHANGES_REQUESTED", CommitID: ""},
			},
			head: "",
			want: AppVerdictStale,
		},
		{
			// The approving mirror of the same clause: an unknown head must not turn a
			// review into an approval either. Both decisive states go through Stale.
			name: "empty head AND empty review commit, approved — still stale",
			reviews: []AppReview{
				{AuthorLogin: appBot, State: "APPROVED", CommitID: ""},
			},
			head: "",
			want: AppVerdictStale,
		},
		{
			// A review with no commit id against a KNOWN head is stale for the ordinary
			// reason (the inequality), not by the empty-head clause.
			name: "review with no commit id against a known head — stale",
			reviews: []AppReview{
				{AuthorLogin: appBot, State: "CHANGES_REQUESTED", CommitID: ""},
			},
			head: "h1",
			want: AppVerdictStale,
		},
		{
			name: "interspersed non-app reviews do not break the order",
			reviews: []AppReview{
				{AuthorLogin: "human", State: "CHANGES_REQUESTED", CommitID: "h1"},
				{AuthorLogin: appBot, State: "CHANGES_REQUESTED", CommitID: "h1"},
				{AuthorLogin: "human", State: "APPROVED", CommitID: "h1"},
			},
			head: "h1",
			want: AppVerdictChangesRequested,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ReduceAppVerdict(appBot, c.reviews, c.head); got != c.want {
				t.Errorf("ReduceAppVerdict(head=%q) = %q, want %q", c.head, got, c.want)
			}
		})
	}
}

// TestSameActor — the identity fold. GitHub renders ONE App two ways depending on which
// endpoint you asked, and a comparison that does not cross the renderings answers
// "different actor" for every App on the watched repos. Verified live against
// an App-authored PR: `gh pr view --json author` gives
// "app/assay-worker-app" while GET /issues/457/comments gives "assay-worker-app[bot]".
func TestSameActor(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		// The whole point: the two renderings of one App are the same actor.
		{"gh rendering vs REST rendering", "app/assay-worker-app", "assay-worker-app[bot]", true},
		{"REST rendering vs gh rendering", "assay-worker-app[bot]", "app/assay-worker-app", true},
		{"identical gh renderings", "app/assay-worker-app", "app/assay-worker-app", true},
		{"identical REST renderings", "assay-worker-app[bot]", "assay-worker-app[bot]", true},
		{"logins are case-insensitive", "app/Assay-Worker-App", "assay-worker-app[bot]", true},
		{"surrounding whitespace does not change identity", " app/assay-worker-app ", "assay-worker-app[bot]", true},

		// The fold must not over-reach. These are the false MATCHES a sloppier
		// normalisation would produce, and each one would let a stranger's comment read
		// as the PR author's reply.
		{"two different apps", "app/assay-worker-app", "assay-reviewer-app[bot]", false},
		{"a human is not the app of the same name", "app/ada", "ada", false},
		{"a bot-suffixed app is not the human of the same name", "ada[bot]", "ada", false},
		{"different humans", "ada", "example-org", false},
		{"a substring is not an identity", "app/assay-worker", "assay-worker-app[bot]", false},

		// Absence is not an identity. Matching "" against "" would resolve two unknowns
		// into a claimed match — the #236 shape at the identity layer.
		{"two absent logins are not the same actor", "", "", false},
		{"an absent login matches nobody", "", "app/assay-worker-app", false},
		{"nobody matches an absent login", "assay-worker-app[bot]", "", false},
		{"whitespace-only is absent", "   ", "app/assay-worker-app", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := SameActor(c.a, c.b); got != c.want {
				t.Errorf("SameActor(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
			}
			if got := SameActor(c.b, c.a); got != c.want {
				t.Errorf("SameActor is not symmetric: SameActor(%q, %q) = %v, want %v", c.b, c.a, got, c.want)
			}
		})
	}
}

func TestNormalizeActorLogin(t *testing.T) {
	cases := []struct {
		login    string
		wantName string
		wantApp  bool
	}{
		{"app/assay-worker-app", "assay-worker-app", true},
		{"assay-worker-app[bot]", "assay-worker-app", true},
		{"dependabot[bot]", "dependabot", true},
		{"ada", "ada", false},
		{"Example-ORG", "example-org", false},
		{"", "", false},
	}
	for _, c := range cases {
		t.Run(c.login, func(t *testing.T) {
			name, isApp := NormalizeActorLogin(c.login)
			if name != c.wantName || isApp != c.wantApp {
				t.Errorf("NormalizeActorLogin(%q) = (%q, %v), want (%q, %v)",
					c.login, name, isApp, c.wantName, c.wantApp)
			}
		})
	}
}

// TestReduceAppVerdict_CrossesLoginRenderings — the reducer's own identity test goes
// through SameActor, so a caller may name the App in either rendering and the reviews
// may arrive in either. Before this, a caller holding the gh rendering would have found
// NO App reviews at all and read the PR as unreviewed.
func TestReduceAppVerdict_CrossesLoginRenderings(t *testing.T) {
	reviews := []AppReview{{AuthorLogin: "assay-reviewer-app[bot]", State: "CHANGES_REQUESTED", CommitID: "h1"}}
	if got := ReduceAppVerdict("app/assay-reviewer-app", reviews, "h1"); got != AppVerdictChangesRequested {
		t.Errorf("ReduceAppVerdict(gh rendering) = %q, want %q", got, AppVerdictChangesRequested)
	}
	if _, ok := LastAppDecisiveReview("app/assay-reviewer-app", reviews); !ok {
		t.Error("LastAppDecisiveReview did not find the App review named in the gh rendering")
	}
	// And it still refuses a DIFFERENT App in either rendering.
	if got := ReduceAppVerdict("app/assay-worker-app", reviews, "h1"); got != AppVerdictNone {
		t.Errorf("ReduceAppVerdict(a different App) = %q, want %q", got, AppVerdictNone)
	}
}

func TestAppVerdict_IsBlockingAtHead(t *testing.T) {
	if !AppVerdictChangesRequested.IsBlockingAtHead() {
		t.Error("ChangesRequested must be blocking at head")
	}
	for _, v := range []AppVerdict{AppVerdictNone, AppVerdictApproved, AppVerdictStale} {
		if v.IsBlockingAtHead() {
			t.Errorf("%q must NOT be blocking at head", v)
		}
	}
}

func TestLastAppDecisiveReview(t *testing.T) {
	reviews := []AppReview{
		{AuthorLogin: appBot, State: "CHANGES_REQUESTED", CommitID: "h1", SubmittedAt: "2026-08-01T00:00:00Z"},
		{AuthorLogin: "human", State: "APPROVED", CommitID: "h1", SubmittedAt: "2026-08-02T00:00:00Z"},
		{AuthorLogin: appBot, State: "APPROVED", CommitID: "h1", SubmittedAt: "2026-08-03T00:00:00Z"},
	}
	last, ok := LastAppDecisiveReview(appBot, reviews)
	if !ok {
		t.Fatal("expected a decisive App review")
	}
	if last.SubmittedAt != "2026-08-03T00:00:00Z" {
		t.Errorf("last decisive = %s, want the chronologically latest", last.SubmittedAt)
	}
	if _, ok := LastAppDecisiveReview(appBot, nil); ok {
		t.Error("nil reviews must report no decisive review")
	}
}

func TestReduceCIVerdict(t *testing.T) {
	cases := []struct {
		name       string
		checks     []CICheck
		ciRequired bool
		want       CIVerdict
	}{
		{"empty ci-required repo", nil, true, CIVerdictUnknown},
		{"empty ci-less repo is vacuously green", nil, false, CIVerdictPass},
		{"one pass", []CICheck{{Status: "COMPLETED", Conclusion: "SUCCESS"}}, true, CIVerdictPass},
		{"one pending", []CICheck{{Status: "IN_PROGRESS"}}, true, CIVerdictPending},
		{"one fail", []CICheck{{Status: "COMPLETED", Conclusion: "FAILURE"}}, true, CIVerdictFail},
		{"skipped ignored, pass governs", []CICheck{
			{Status: "COMPLETED", Conclusion: "SKIPPED"},
			{Status: "COMPLETED", Conclusion: "SUCCESS"},
		}, true, CIVerdictPass},
		{"fail ranks above pending", []CICheck{
			{Status: "IN_PROGRESS"},
			{Status: "COMPLETED", Conclusion: "FAILURE"},
		}, true, CIVerdictFail},
		{"all skipped on ci-required is unknown (no signal)", []CICheck{
			{Status: "COMPLETED", Conclusion: "SKIPPED"},
		}, true, CIVerdictUnknown},
		{"all skipped on ci-less is vacuously green", []CICheck{
			{Status: "COMPLETED", Conclusion: "SKIPPED"},
		}, false, CIVerdictPass},
		// Latest-run-per-name (#282/#289): a superseded run of a NAME must not count against
		// a PR whose current run for that name is green. Pre-reduction these reddened —
		// the CANCELLED and the QUEUED both outranked the later SUCCESS.
		{"cancelled predecessor + green latest, same name → pass", []CICheck{
			{Name: "changelog", Status: "COMPLETED", Conclusion: "CANCELLED", CompletedAt: ciTOld},
			{Name: "changelog", Status: "COMPLETED", Conclusion: "SUCCESS", CompletedAt: ciTNew},
		}, true, CIVerdictPass},
		{"stale-queued orphan + green latest, same name → pass", []CICheck{
			{Name: "control-sweep", Status: "QUEUED"},
			{Name: "control-sweep", Status: "COMPLETED", Conclusion: "SUCCESS", CompletedAt: ciTNew},
		}, true, CIVerdictPass},
		// The anti-over-loosen direction: when the LATEST run of a name is red (an older
		// SUCCESS superseded by a newer FAILURE) the verdict must stay Fail. The reduction
		// ignores superseded runs, never a real failing latest run.
		{"older success superseded by newer failure, same name → fail", []CICheck{
			{Name: "build", Status: "COMPLETED", Conclusion: "SUCCESS", CompletedAt: ciTOld},
			{Name: "build", Status: "COMPLETED", Conclusion: "FAILURE", CompletedAt: ciTNew},
		}, true, CIVerdictFail},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ReduceCIVerdict(c.checks, c.ciRequired); got != c.want {
				t.Errorf("ReduceCIVerdict(ciRequired=%v) = %q, want %q", c.ciRequired, got, c.want)
			}
		})
	}
}

// ciTOld sorts before ciTNew both lexicographically and chronologically — the property
// recencyKey relies on.
const (
	ciTOld = "2026-01-01T00:00:00Z"
	ciTNew = "2026-01-01T01:00:00Z"
)

// TestLatestRunPerName exercises the shared reducer directly: newest-per-name wins, a
// stampless run loses, entries with no identity are never collapsed, and a genuinely
// multi-name set is preserved with first-appearance order.
func TestLatestRunPerName(t *testing.T) {
	in := []CICheck{
		{Name: "a", Conclusion: "FAILURE", Status: "COMPLETED", CompletedAt: ciTOld},
		{Name: "a", Conclusion: "SUCCESS", Status: "COMPLETED", CompletedAt: ciTNew},
		{Name: "b", Status: "QUEUED"}, // stampless
		{Name: "b", Conclusion: "SUCCESS", Status: "COMPLETED", CompletedAt: ciTNew},
		{Context: "legacy", CreatedAt: ciTOld},
		{Context: "legacy", CreatedAt: ciTNew, Status: "ctx-new"},
		{Status: "COMPLETED", Conclusion: "SUCCESS"}, // no identity
		{Status: "COMPLETED", Conclusion: "FAILURE"}, // no identity — must NOT merge with the above
	}
	got := LatestRunPerName(in, CICheck.groupKey, CICheck.recencyKey)
	if len(got) != 5 { // a, b, legacy + the two identity-less entries
		t.Fatalf("reduced to %d entries, want 5: %+v", len(got), got)
	}
	byName := map[string]CICheck{}
	identityless := 0
	for _, c := range got {
		if c.groupKey() == "" {
			identityless++
			continue
		}
		byName[c.groupKey()] = c
	}
	if byName["a"].Conclusion != "SUCCESS" {
		t.Errorf("name a kept %q, want the newer SUCCESS", byName["a"].Conclusion)
	}
	if byName["b"].Conclusion != "SUCCESS" || byName["b"].Status != "COMPLETED" {
		t.Errorf("name b kept %+v, want the completed SUCCESS over the stampless QUEUED", byName["b"])
	}
	if byName["legacy"].Status != "ctx-new" {
		t.Errorf("context legacy kept %+v, want the newer entry", byName["legacy"])
	}
	if identityless != 2 {
		t.Errorf("identity-less entries collapsed to %d, want 2 (no name to reduce by)", identityless)
	}
}
