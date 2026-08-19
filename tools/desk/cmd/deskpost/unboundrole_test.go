package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The unbound-role regression suite (an earlier review blocker, both lanes).
//
// A roster can load as fully CONFIGURED — blessing authority, trusted humans, allowed
// repos, all present, zero problems — while a desk role is UNBOUND, because the `role=`
// prefix on ASSAY_TRUSTED_BOT_SLUGS is optional per entry and dropping one is a typo the
// loader accepts. That state used to turn every reviewer-identity comparison into
// `login == ""`, and GitHub renders a deleted review author as `"user": null` → `Login: ""`.
// So a review nobody wrote satisfied both flip gates.
//
// These tests construct exactly that state. They are CAN-FAIL by demonstration: with
// isReviewerBot reverted to the pre-fix shape (`want, _ := RoleAppLogin(...); return
// login == want`) all three assertions below report DEFECT.

// rosterNoReviewerPrefix is the fixture roster with `reviewer=` dropped and NOTHING else
// changed — the single-character-class typo the security review named.
const rosterNoReviewerPrefix = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=assay-reviewer-app:300000004,verifier=assay-verifier-app:300000005
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private
ASSAY_HUMAN_LOGIN_MAP=alex:ada
`

func installRoster(t *testing.T, body string) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	prev, had := os.LookupEnv("HOME")
	t.Setenv("HOME", home)
	deskkit.ReloadConfig()
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("HOME", prev)
		}
		deskkit.ReloadConfig()
	})
}

func TestUnboundReviewerRoleAdmitsNothing(t *testing.T) {
	installRoster(t, rosterNoReviewerPrefix)

	// The two preconditions are what make this defect SILENT rather than loud.
	if !deskkit.EffectiveConfig().Configured() {
		t.Fatal("precondition: this roster must load as CONFIGURED — a roster that refused " +
			"outright would never reach the comparison, and the test would prove nothing")
	}
	if _, ok := deskkit.RoleAppLogin("reviewer"); ok {
		t.Fatal("precondition: the reviewer role must be UNBOUND in this roster")
	}

	head := "deadbeef"
	authorless := []reviewInfo{
		{State: "APPROVED", CommitID: head, Body: "Reviewed by execution.\n\nVerdict: approve\n"},
		{State: "APPROVED", CommitID: head, Body: "## Security review\n\nSecurity-Review: pass\n"},
	}

	if _, _, found, _ := latestAppVerdict(authorless); found {
		t.Error("the CORRECTNESS lane accepted a review with no author under an unbound role")
	}
	if v := securityVerdictAtHead(authorless, head); v == secPass {
		t.Error("the SECURITY lane returned pass on a review with no author under an unbound role")
	}
	if dup, _ := appReviewExistsAt(authorless, head, "APPROVED", "correctness", digOf(okReviewBody)); dup {
		t.Error("appReviewExistsAt matched an author-less review under an unbound role")
	}
}
func TestReviewerRoleResolvesToTheConfiguredSlug(t *testing.T) {
	got, ok := deskkit.RoleAppLogin("reviewer")
	if !ok {
		t.Fatal("the fixture roster binds reviewer=assay-reviewer-app; the role must resolve")
	}
	const want = "assay-reviewer-app[bot]"
	if got != want {
		t.Fatalf("RoleAppLogin(\"reviewer\") = %q, want %q (the value ASSAY_TRUSTED_BOT_SLUGS "+
			"binds in the fixture roster) — a role lookup that answers any other string, "+
			"including \"\", is not the configured identity", got, want)
	}
	if !isReviewerBot(want) {
		t.Fatalf("isReviewerBot(%q) must be true under the fixture roster", want)
	}
	for _, bad := range []string{"", "assay-reviewer-app", "app/assay-reviewer-app", "ada", "attacker[bot]"} {
		if isReviewerBot(bad) {
			t.Errorf("isReviewerBot(%q) must be false", bad)
		}
	}
}
