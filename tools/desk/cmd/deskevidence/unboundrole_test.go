package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The unbound-verifier-role regression (a review blocker).
//
// checkAttribution has three deliberate states, and the THIRD — could-not-check for an
// empty commit author — is the one the comment says has actually been seen in a real
// response. With the verifier role unbound, the empty author matched the PROVEN arm
// first (`author == verifierBotLogin()` where both sides were ""), so the could-not-check
// branch was dead and an Evidence commit carrying no verifier identity was reported as
// carrying it.
//
// CAN-FAIL by demonstration: with checkAttribution's first arm removed and isVerifierBot
// reverted to a bare `author == want` comparison, TestUnboundVerifierRoleRefuses fails
// with detail "author=" and a nil error.

const rosterNoVerifierPrefix = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=reviewer=assay-reviewer-app:300000004,assay-verifier-app:300000005
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private
ASSAY_HUMAN_LOGIN_MAP=alex:ada
`

func installEvidenceRoster(t *testing.T, body string) {
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

func TestUnboundVerifierRoleRefuses(t *testing.T) {
	installEvidenceRoster(t, rosterNoVerifierPrefix)

	if !deskkit.EffectiveConfig().Configured() {
		t.Fatal("precondition: this roster must load as CONFIGURED — that is what made the defect silent")
	}
	if _, ok := deskkit.RoleAppLogin("verifier"); ok {
		t.Fatal("precondition: the verifier role must be UNBOUND in this roster")
	}

	// The empty author is the response shape the third state exists for.
	detail, err := checkAttribution("")
	if err == nil {
		t.Errorf("checkAttribution(\"\") returned err=nil under an unbound verifier role — "+
			"an Evidence commit with NO author was accepted (detail=%q)", detail)
	}
	if strings.Contains(detail, "author=") && !strings.Contains(detail, "REFUSED") {
		t.Errorf("detail = %q — an unbound role must REFUSE, never report an attribution it "+
			"cannot check", detail)
	}
	// A correctly-attributed author is refused too: with no binding there is nothing to
	// check it against, and guessing in either direction is the failure mode.
	if _, err := checkAttribution("assay-verifier-app[bot]"); err == nil {
		t.Error("an unbound verifier role must refuse even a correct-looking author — " +
			"the tool cannot know it is correct")
	}
}

// With the role BOUND, the three states must be distinct and in the documented order.
func TestBoundVerifierRoleKeepsThreeStates(t *testing.T) {
	want, ok := deskkit.RoleAppLogin("verifier")
	if !ok {
		t.Fatal("precondition: the package fixture roster binds the verifier role")
	}
	if want != "assay-verifier-app[bot]" {
		t.Fatalf("RoleAppLogin(\"verifier\") = %q, want the slug the fixture roster binds", want)
	}

	if detail, err := checkAttribution(want); err != nil || !strings.Contains(detail, want) {
		t.Errorf("proven state: checkAttribution(%q) = (%q, %v), want the author recorded and no error", want, detail, err)
	}
	detail, err := checkAttribution("")
	if err != nil {
		t.Errorf("could-not-check state: an empty author must NOT be an error, got %v", err)
	}
	if detail != "attribution=could-not-check" {
		t.Errorf("could-not-check state: detail = %q, want %q — this branch was DEAD before the "+
			"unbound-role fix, because the empty author matched the proven arm first",
			detail, "attribution=could-not-check")
	}
	if _, err := checkAttribution("someone-else"); err == nil {
		t.Error("proven-wrong state: a different author must be Unverifiable")
	}
}
