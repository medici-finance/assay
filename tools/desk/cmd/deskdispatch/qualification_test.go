package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/reviewtest"
)

func TestQualificationProbe(t *testing.T) {
	q := reviewtest.Read(t)
	if strings.HasPrefix(q.Case, "caller-prepare") {
		callerPreparationProbe(t, q)
		return
	}
	f := roundFixture()
	if q.Head != "" {
		if len(q.Head) != 40 || len(q.Base) != 40 {
			t.Fatal("fixture Git endpoints must be full SHAs")
		}
		f.pr.HeadSHA = q.Head
		f.reviews[0].CommitID = q.Base
		f.compares = map[string]*deskkit.RefComparison{
			"main..." + q.Base:      {Files: changed("a.go", "b.go"), CommitsComplete: true},
			q.Base + "..." + q.Head: {CommitsComplete: true, Commits: []deskkit.RepoCommit{{SHA: q.Head, Parents: []string{q.Base}}}},
		}
		f.commits = map[string]*deskkit.RepoCommit{q.Head: {SHA: q.Head, Parents: []string{q.Base}, Files: changed("a.go"), FilesComplete: true}}
	}
	key := rrKeyC
	switch q.Case {
	case "delta":
	case "security":
		key = rrKeyS
	case "duplicate-lane":
		f.reviews = append(f.reviews, verdict(502, laneCorrectness, "request-changes", rrHead1), verdict(503, laneSecurity, "pass", rrHead2))
	case "access-failure":
		f.reviewsErr = errors.New("fixture access failure")
	case "required-check-red":
		f.required = []string{"build"}
		f.checks = checksOf([]deskkit.CheckRun{run1("build", "completed", "failure", "2026-03-04T06:00:00Z")})
	default:
		t.Fatalf("unknown dispatch fixture %q", q.Case)
	}
	rr, err := roundOf(t, f, key)
	var text strings.Builder
	if err == nil {
		writeReviewRound(&text, rr)
	} // a held dispatch produces no assignment
	r := reviewtest.Response{Output: text.String(), Exit: deskkit.ExitCodeOf(err), Facts: map[string]string{
		"head": rr.head, "lane": rr.lane, "round": fmt.Sprint(rr.round.number), "known": fmt.Sprint(rr.round.known), "full": fmt.Sprint(rr.scope.full), "reasons": reasonsOf(rr.scope),
	}, Calls: f.calls}
	if err != nil {
		r.Facts["hold"] = firstLine(err.Error())
	}
	reviewtest.Write(t, r)
}
