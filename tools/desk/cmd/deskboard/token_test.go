package main

// token_test.go — a repo outside the resolved forge's installation is per-repo
// could-not-check rather than a dead board.
//
// The read path used to authenticate as the session role's App by injecting GH_TOKEN into a
// child `gh` (token.go); the forge-seam migration retired the last `gh` read in
// cmd/deskboard, so that injection machinery — and the tests that pinned it — are gone with
// ghRun. What survives is the property those reads were FOR: one repo the forge cannot resolve
// demotes to a could-not-check ROW, and every OTHER repo's rows are still read.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// outOfInstallationErr is the error the forge returns for a repo the authenticated identity
// cannot see.
func outOfInstallationErr(repo string) error {
	return deskkit.Unverifiable("cannot read open PRs for "+repo,
		fmt.Errorf("gh pr list: GraphQL: Could not resolve to a Repository with the name '%s'. (repository)", repo))
}

// stubPRList serves one open PR through the TYPED ListOpenChanges op for every repo EXCEPT
// `unreachable`, which fails the way an out-of-installation repo fails. The open-PR read
// reaches the forge through forgeFor, so the per-repo behavior is stubbed at that seam.
func stubPRList(t *testing.T, unreachable string, failOther error) {
	t.Helper()
	stubForgeList(t, func(repo string) (*deskkit.OpenChanges, error) {
		if repo == unreachable {
			if failOther != nil {
				return nil, failOther
			}
			return nil, outOfInstallationErr(repo)
		}
		return &deskkit.OpenChanges{Cap: prListLimit, Changes: []deskkit.OpenChange{{
			Number: 7, Title: "a change", State: "OPEN", Draft: true,
			Author: deskkit.Account{Login: "ada"}, CreatedAt: "2026-01-01T00:00:00Z",
			HeadSHA: "abc123", MergeStateStatus: "CLEAN",
			Rollup: []deskkit.RollupNode{{Typename: "CheckRun", Name: "ci", Status: "COMPLETED", Conclusion: "SUCCESS"}},
		}}}, nil
	})
}

func TestOutOfInstallationRepoIsCouldNotCheck(t *testing.T) {
	installFakeGH(t)
	const unreachable = "example-org/proposals"
	stubPRList(t, unreachable, nil)

	rep, err := cmdPRs(Header{AsOf: "2026-01-01T00:00:00Z"})
	if err != nil {
		t.Fatalf("one repo outside the App installation took the whole board down: %v", err)
	}
	view, ok := rep.value.(prsReport)
	if !ok {
		t.Fatalf("report value is %T, want prsReport", rep.value)
	}
	cov := view.Header.RepoCoverage
	if cov == nil {
		t.Fatal("the report states no coverage — a repo that could not be read must never be " +
			"indistinguishable from a repo with no open PRs")
	}
	if len(cov.Unreadable) != 1 || cov.Unreadable[0].Repo != unreachable {
		t.Fatalf("unreadable = %v, want exactly %s", cov.Unreadable, unreachable)
	}
	if cov.Read != len(deskkit.AllowedRepos())-1 {
		t.Fatalf("read = %d, want %d", cov.Read, len(deskkit.AllowedRepos())-1)
	}
	// Every OTHER repo's rows are still there — the point of demoting rather than aborting.
	if len(view.PRs) != len(deskkit.AllowedRepos())-1 {
		t.Fatalf("rows = %d, want one per readable repo (%d)", len(view.PRs), len(deskkit.AllowedRepos())-1)
	}
	for _, r := range view.PRs {
		if r.Repo == unreachable {
			t.Fatalf("a row was invented for the repo that could not be read: %+v", r)
		}
	}
	// And the table path says so out loud.
	var b strings.Builder
	rep.render(&b)
	if !strings.Contains(b.String(), "COULD NOT CHECK") {
		t.Fatalf("the table does not state the gap:\n%s", b.String())
	}
}

// The demotion is DELIBERATELY narrow. Any other read failure — a 401, a rate limit, a
// timeout — still fails the whole run closed, because each of those can be transient and
// could be hiding rows in a repo the desk CAN see.
func TestOtherReadErrorsStillFailTheRunClosed(t *testing.T) {
	installFakeGH(t)
	stubPRList(t, "example-org/proposals",
		deskkit.Unverifiable("cannot read open PRs for example-org/proposals",
			errors.New("gh pr list: HTTP 401: Requires authentication")))

	if _, err := cmdPRs(Header{AsOf: "2026-01-01T00:00:00Z"}); err == nil {
		t.Fatal("a 401 was demoted to could-not-check — a transient auth failure can hide rows in " +
			"a repo the desk CAN see, so it must fail the run")
	} else if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want %d (unverifiable)", got, deskkit.ExitUnverifiable)
	}
}

// The matcher's positive/negative control: a guard that stopped matching would silently
// turn every out-of-installation repo back into a dead board, and a guard that matched too
// broadly would swallow real failures.
func TestOutOfInstallationMatcherIsLive(t *testing.T) {
	if !outOfInstallation(outOfInstallationErr("example-org/proposals")) {
		t.Error("the matcher did not fire on GitHub's own out-of-installation message — a clean " +
			"result from it would mean nothing")
	}
	for _, e := range []error{
		nil,
		errors.New("HTTP 401: Requires authentication"),
		errors.New("You have exceeded a secondary rate limit"),
		errors.New("timed out after 2m0s (wedged subprocess killed)"),
	} {
		if outOfInstallation(e) {
			t.Errorf("the matcher fired on %v — that failure can hide rows and must fail the run", e)
		}
	}
}
