package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// verdictLanes are the three CLI entry points that post a review verdict.
var verdictLanes = []struct {
	name string
	args func(bodyFile string) []string
	body string
}{
	{"review", func(bf string) []string { return reviewArgs(exampleRepo, "1", "approve", testHead, bf) }, okReviewBody},
	{"security-review", func(bf string) []string { return secReviewArgs(exampleRepo, "1", "pass", testHead, bf) }, okSecurityBody},
	{"finish", func(bf string) []string { return finishArgs("review", "1", "approve", testHead, bf, finishClaimKey) }, okReviewBody},
}

// Every loop that does not act as the reviewer is refused a verdict, on every verdict
// verb, before any mint, read or write. the-desk is in the set: a coordinator window must
// not post a review.
// regression: #6069
func TestVerdictCallerRole(t *testing.T) {
	var sawDesk bool
	for _, loop := range deskkit.KnownLoopNames() {
		canonical, _ := deskkit.CanonicalLoopName(loop)
		if role, ok := deskkit.TokenRoleForLoop(canonical); ok && role == "reviewer" {
			continue // the existing review and finish success tests cover the reviewer loop
		}
		sawDesk = sawDesk || loop == "the-desk"
		for _, lane := range verdictLanes {
			t.Run(loop+"/"+lane.name, func(t *testing.T) {
				f, errBuf := setupFake(t)
				t.Setenv("DESK_LOOP", loop)
				f.pullHeads = []string{testHead}
				bf := writeBody(t, "verdict.md", lane.body)
				code := run(lane.args(bf))
				if code != deskkit.ExitRefused || f.postedReview != 0 || len(f.hits) != 0 {
					t.Fatalf("loop %s %s: exit=%d reviews=%d network=%d; want refused before mint/read/write\n%s",
						loop, lane.name, code, f.postedReview, len(f.hits), errBuf.String())
				}
				for _, want := range []string{"#6069", "reviewer", "pr-review-desk"} {
					if !strings.Contains(errBuf.String(), want) {
						t.Errorf("refusal missing %q: %s", want, errBuf.String())
					}
				}
				if e := lastAudit(t); e.Result != deskkit.ResultRefused || e.BodyDigest == "" {
					t.Errorf("refusal audit = %+v", e)
				}
			})
		}
	}
	if !sawDesk {
		t.Fatal("the-desk is not a known loop; the coordinator case went untested")
	}
}

// A second caller reaching the shared verdict path without the CLI is checked there too.
// regression: #6069
func TestVerdictRoleBypass(t *testing.T) {
	shape, _ := correctnessShapeFor("approve")
	for _, tc := range []struct {
		loop string
		code int
	}{
		{"", deskkit.ExitRefused},
		{"unknown-loop", deskkit.ExitUnverifiable},
		{"the-desk", deskkit.ExitRefused},
		{"worker-desk", deskkit.ExitRefused},
	} {
		t.Run(tc.loop, func(t *testing.T) {
			f, errBuf := setupFake(t)
			t.Setenv("DESK_LOOP", tc.loop)
			f.pullHeads = []string{testHead}
			code := postVerdictReview("example-org", "tracker", 1, shape, testHead,
				[]byte(okReviewBody), []string{"review"}, postOpts{})
			if code != tc.code || f.postedReview != 0 || len(f.hits) != 0 {
				t.Fatalf("direct caller: exit=%d reviews=%d network=%d; want exit=%d and zero contact\n%s",
					code, f.postedReview, len(f.hits), tc.code, errBuf.String())
			}
		})
	}
}
