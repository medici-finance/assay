package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// deskclose_forgeverify_test.go — the two named Verify deliverables for brief 13's deskclose
// slice: the close goes THROUGH the backend op (CloseIssue), and the acting login comes from the
// MINTED role's roster binding, not a `viewer{login}` forge read.

// TestDeskcloseClosesThroughBackend proves a reviewer-confirmed superseded close reaches the
// forge through CloseIssue (the stub records the close as the backend op it served), never a
// bespoke `gh pr/issue close` shell-out.
func TestDeskcloseClosesThroughBackend(t *testing.T) {
	s, rul := prWorld(t)
	s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
	// reviewer confirms (baseWorld sets s.viewer = reviewerLogin → mintedRole = reviewer).
	code, out := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
	if code != deskkit.ExitOK {
		t.Fatalf("reviewer confirm should close (exit 0), got %d\n%s", code, out)
	}
	if s.closes() != 1 {
		t.Fatalf("want exactly one close through the backend (CloseIssue), got %d: %v", s.closes(), s.writes())
	}
}

// TestDeskcloseActingLoginFromRoster proves the acting identity comes from the MINTED role's
// roster binding (RoleAppLogin), not a `viewer{login}` forge round-trip: the verdict comment
// carries the reviewer App's roster login, and deskclose made NO `api graphql viewer` call.
func TestDeskcloseActingLoginFromRoster(t *testing.T) {
	s, rul := prWorld(t)
	s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
	code, _ := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
	if code != deskkit.ExitOK {
		t.Fatalf("reviewer confirm should close, got %d", code)
	}

	reviewer, ok := deskkit.RoleAppLogin(roleReviewer)
	if !ok {
		t.Fatal("the reviewer role is unbound in the fixture roster")
	}
	// The verdict comment (the reviewer's) must carry the reviewer App's roster login — the
	// acting identity came from the minted role, not a forge read of the token.
	body := commentBody(t, s)
	if !strings.Contains(body, reviewer) {
		t.Fatalf("the verdict does not carry the reviewer App's roster login %q:\n%s", reviewer, body)
	}
	// And no `viewer{login}` whoami was issued: the login is an identity-layer answer now.
	for _, c := range s.calls {
		if len(c) >= 2 && c[0] == "api" && c[1] == "graphql" {
			t.Fatalf("deskclose issued a viewer/graphql whoami; the acting login must come from the "+
				"minted role, not a forge read: %v", c)
		}
	}
}
