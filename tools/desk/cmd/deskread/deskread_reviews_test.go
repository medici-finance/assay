package main

// deskread_reviews_test.go — the `reviews` kind (forge-neutral brief 18): a change's whole review
// array over deskkit.Forge.ReviewsAtHead, for statusgen's brief-flow metrics. No new Forge
// operation: the seam already serves the array; the kind is transport.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ReviewsAtHead answers two submissions by one reviewer (a request-changes, then an approval — the
// cycle a rework count reads and a latest-per-author view erases) for an ordinary change, an empty
// array for #11, and the standard could-not-check / forge-404 answers for 404 / 405.
func (f *recForge) ReviewsAtHead(repo deskkit.ForgeRepo, number int) ([]deskkit.Review, error) {
	f.rec(recCall{Op: "ReviewsAtHead", Repo: repo.Slug(), Number: number})
	if err := failFor(repo, number); err != nil {
		return nil, err
	}
	if number == 11 {
		return nil, nil
	}
	app := deskkit.Account{Login: "example-app[bot]", ID: 300000002, Type: "Bot"}
	return []deskkit.Review{
		{ID: 1, Author: app, State: "CHANGES_REQUESTED", CommitID: "a1", SubmittedAt: "2026-09-02T00:00:00Z", Body: "fix it"},
		{ID: 2, Author: app, State: "APPROVED", CommitID: "b2", SubmittedAt: "2026-09-03T00:00:00Z"},
	}, nil
}

// regression: a change's whole review array had no read on the seam, so statusgen's brief-flow metrics shelled `gh api pulls/N/reviews`
func TestDeskreadReviewsKind(t *testing.T) {
	t.Run("the whole array, oldest first, with each state and commit", func(t *testing.T) {
		f := stubRec(t)
		env, code, errs := runItemEnvelope(t, "reviews", "--change", "example-org/alpha#9", "--change", "example-org/alpha#11", "--change", "example-org/alpha#404")
		if code != deskkit.ExitOK || env.Kind != "reviews" || env.Schema != envelopeSchema {
			t.Fatalf("exit=%d env=%+v stderr=%s", code, env, errs)
		}
		if len(env.Items) != 2 || len(env.Partial) != 1 || env.Partial[0].Number != 404 || env.Partial[0].Reason == "" {
			t.Fatalf("items=%+v partial=%+v, want #9 and #11 read and #404 in partial with a reason", env.Items, env.Partial)
		}
		rs := env.Items[0].Reviews
		if env.Items[0].Number != 9 || rs == nil || len(*rs) != 2 {
			t.Fatalf("#9 reviews = %+v, want both submissions", rs)
		}
		a, b := (*rs)[0], (*rs)[1]
		if a.State != "CHANGES_REQUESTED" || a.CommitID != "a1" || a.AuthorLogin != "example-app[bot]" || a.AuthorID != 300000002 ||
			b.State != "APPROVED" || b.SubmittedAt != "2026-09-03T00:00:00Z" {
			t.Errorf("reviews = %+v, want the seam's order and fields carried", *rs)
		}
		// No review is a real answer ([]), a pointer so it is not mistaken for an absent payload.
		if e := env.Items[1].Reviews; e == nil || len(*e) != 0 {
			t.Errorf("#11 reviews = %+v, want an explicit empty list", e)
		}
		for _, c := range f.recorded() {
			if c.Op != "ReviewsAtHead" {
				t.Errorf("call %+v: the kind reached for an operation other than ReviewsAtHead", c)
			}
		}
	})

	t.Run("the review body is not carried", func(t *testing.T) {
		stubRec(t)
		s, code, _ := rawOut("reviews", "--change", "example-org/alpha#9")
		if code != deskkit.ExitOK || s == "" {
			t.Fatalf("exit=%d out=%q", code, s)
		}
		if strings.Contains(s, "fix it") {
			t.Errorf("the envelope carries a review body:\n%s", s)
		}
	})

	t.Run("it is a change kind, addressed by --change only", func(t *testing.T) {
		f := stubRec(t)
		for _, args := range [][]string{
			{"reviews", "--issue", "example-org/alpha#9"},
			{"reviews", "--repo", "example-org/alpha"},
			{"reviews"},
		} {
			out, code, _ := rawOut(args...)
			if code != deskkit.ExitRefused || out != "" {
				t.Errorf("%v: exit=%d out=%q, want a refusal with no output", args, code, out)
			}
		}
		if c := f.recorded(); len(c) != 0 {
			t.Errorf("a refused run made forge calls: %+v", c)
		}
	})
}
