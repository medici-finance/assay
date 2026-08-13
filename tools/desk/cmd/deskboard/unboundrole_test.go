package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The unbound-reviewer-role regression for the board's review reduction — the surface
// that drives the desk's FLIP recommendation (a review-blocker bug).
//
// CAN-FAIL by demonstration: with isReviewerBot reverted to `want, _ :=
// RoleAppLogin("reviewer"); return login == want`, reduceReviews counts the author-less
// APPROVED review and the assertions below report the defect.

const boardRosterNoReviewerPrefix = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=assay-reviewer-app:300000004,verifier=assay-verifier-app:300000005
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private
ASSAY_HUMAN_LOGIN_MAP=alex:ada
`

func installBoardRoster(t *testing.T, body string) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}

func TestBoardUnboundReviewerRoleCountsNoReview(t *testing.T) {
	installBoardRoster(t, boardRosterNoReviewerPrefix)
	if !deskkit.EffectiveConfig().Configured() {
		t.Fatal("precondition: the roster must load as CONFIGURED")
	}
	if _, ok := deskkit.RoleAppLogin("reviewer"); ok {
		t.Fatal("precondition: the reviewer role must be UNBOUND")
	}

	head := "deadbeef"
	authorless := []review{
		{State: "APPROVED", CommitID: head, Body: "Verdict: approve"},
		{State: "APPROVED", CommitID: head, Body: "Security-Review: pass"},
	}
	st := reduceReviews(authorless, head)
	if st.approved || st.ever || st.atHead {
		t.Error("reduceReviews reported an App APPROVAL at head from a review with NO author, " +
			"under an unbound reviewer role — this drives the desk's flip recommendation")
	}
	if st.securityPass {
		t.Error("reduceReviews reported a security PASS at head from a review with NO author")
	}
}
