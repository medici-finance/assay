package main

// inheritedtokengitlab_test.go — issue 1631 / sec-1650-S2. verifyInheritedToken's GitLab branch
// (dispatch.go, resolveClaimAuth's "kind == deskkit.ForgeGitLab" arm) had no test: mutating the
// byte-equality check ("if strings.TrimSpace(custody) == tok") to always-true, or the
// unreadable-custody refusal to an honour, left `go test ./cmd/deskdispatch/` green. A regression
// on either control would let a claim run under whatever GH_TOKEN was inherited on a GitLab
// repo, with nothing going red.
//
// These mirror inheritedtoken_test.go's GitHub-arm coverage one lane over: reuse
// claimauthgitlab_test.go's fixture (installGLStamp: a GitLab-resolved roster plus a
// gitlab-<role>.token custody file per DispatcherRole) and claimauth_test.go's REAL fake claim
// child, so "the claim ran under the custody PAT, not the inherited token" is asserted on what
// the child observed.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// glReplies is the canned git/deskwt reply set every GitLab-served dispatch in this file needs:
// a GitLab-shaped origin remote (what actually resolves the forge is the roster entry) plus a
// worktree path for deskwt add.
func glReplies() []reply {
	return []reply{
		{match: "remote get-url origin", stdout: "git@gitlab.com:" + glProject + ".git"},
		{match: "deskwt add", stdout: "/private/tmp/worker-home"},
	}
}

// A GH_TOKEN inherited on a GitLab-served dispatch that is NOT byte-equal to the dispatching
// role's GitLab PAT custody is not an override: a NOTICE names it, the value never reaches the
// claim child, and the claim runs under the role's own custody PAT exactly as with nothing
// exported. The GitHub App minter is still never called — this stays the GitLab lane.
func TestGitLabInheritedNonRolePATIsIgnoredAndTheCustodyPATIsUsed(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	installGLStamp(t, home)
	plantGoClaimChild(t)
	record := installClaimChild(t, s)
	mints := stubMint(t, stubMintedToken, nil)
	s.replies = glReplies()
	t.Setenv("GH_TOKEN", "example-inherited-non-role-pat")

	rc, stderr := runCapturingStderr(t, []string{"item-1", "--root", root, "--repo", glProject,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("GitLab dispatch rc = %d, want 0:\n%s", rc, stderr)
	}
	if len(*mints) != 0 {
		t.Fatalf("the GitHub App minter was called %d time(s) on a GitLab-served dispatch: %v", len(*mints), *mints)
	}
	r := acquireRecord(t, record)
	if strings.TrimSpace(r.tokenFileContent) != glPATPlaceholder {
		t.Fatalf("the claim child read token-file content %q, want the desk role's GitLab PAT %q — an ignored "+
			"inherited token must not change which credential the claim runs under (argv %s)",
			r.tokenFileContent, glPATPlaceholder, r.argv)
	}
	if r.ghToken == "example-inherited-non-role-pat" || strings.Contains(r.tokenFileContent, "example-inherited-non-role-pat") {
		t.Fatalf("the ignored inherited token reached the claim child: %+v", r)
	}
	if os.Getenv("GH_TOKEN") != "" {
		t.Errorf("the ignored token is still in the dispatcher's environment, where every later child inherits it")
	}
	for _, want := range []string{"NOTICE", "IGNORED", "GitLab role PAT"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr does not carry %q:\n%s", want, stderr)
		}
	}
	if strings.Contains(stderr, "example-inherited-non-role-pat") {
		t.Errorf("the inherited token VALUE was printed:\n%s", stderr)
	}
}

// REGRESSION FLOOR: an inherited GH_TOKEN that IS byte-equal to the dispatching role's GitLab
// PAT custody is the deliberate operator override, honoured with no NOTICE and no fall-through
// to the custody-file read — the claim child sees the export as-is.
func TestGitLabInheritedTokenEqualToRolePATIsHonoured(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	installGLStamp(t, home)
	plantGoClaimChild(t)
	record := installClaimChild(t, s)
	mints := stubMint(t, stubMintedToken, nil)
	s.replies = glReplies()
	t.Setenv("GH_TOKEN", glPATPlaceholder)

	rc, stderr := runCapturingStderr(t, []string{"item-1", "--root", root, "--repo", glProject,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("GitLab dispatch rc = %d, want 0:\n%s", rc, stderr)
	}
	if len(*mints) != 0 {
		t.Fatalf("the GitHub App minter was called %d time(s) although the export is the role's own GitLab PAT: %v", len(*mints), *mints)
	}
	r := acquireRecord(t, record)
	if r.ghToken != glPATPlaceholder || r.tokenFile != "" {
		t.Errorf("the child saw GH_TOKEN=%q --token-file=%q, want the verified export inherited and no token file", r.ghToken, r.tokenFile)
	}
	if strings.Contains(stderr, "IGNORED") {
		t.Errorf("a verified export earned the ignored-token NOTICE:\n%s", stderr)
	}
	if !strings.Contains(stderr, "verified: it is the desk GitLab role PAT") {
		t.Errorf("the claim-acquire line does not say the export was verified:\n%s", stderr)
	}
}

// FAIL CLOSED: on a GitLab-served dispatch, an inherited GH_TOKEN whose identity cannot be
// checked — the role's PAT custody file is unreadable — refuses the whole dispatch (exit 5)
// rather than falling either way: never honoured on trust, never silently swapped for a mint
// (there is no GitLab App to mint against), and no claim child ever runs.
func TestGitLabInheritedTokenWithUnreadableCustodyRefusesBeforeAnyClaim(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	installGLStamp(t, home)
	custody := filepath.Join(home, ".config", "assay", "gitlab-"+deskkit.DispatcherRole+".token")
	if err := os.Remove(custody); err != nil {
		t.Fatalf("removing the %s role's GitLab PAT custody fixture: %v", deskkit.DispatcherRole, err)
	}
	plantGoClaimChild(t)
	record := installClaimChild(t, s)
	mints := stubMint(t, stubMintedToken, nil)
	s.replies = glReplies()
	t.Setenv("GH_TOKEN", "example-inherited-token")

	err := cmdDispatch([]string{"item-1", "--root", root, "--repo", glProject,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if err == nil {
		t.Fatal("an inherited token was used on a GitLab repo whose role PAT custody could not be read")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d: %v", deskkit.ExitCodeOf(err), deskkit.ExitRefused, err)
	}
	for _, want := range []string{stepClaimAcquire, "could not be read", "NO claim", "unset GH_TOKEN"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not carry %q: %s", want, err.Error())
		}
	}
	if strings.Contains(err.Error(), "example-inherited-token") {
		t.Errorf("the refusal carries the token VALUE: %s", err.Error())
	}
	if _, statErr := os.Stat(record); statErr == nil {
		t.Fatal("the claim child RAN although the GitLab role PAT custody could not be read to verify the export")
	}
	if len(*mints) != 0 {
		t.Errorf("the GitHub App minter was called %d time(s) on a GitLab-served repo", len(*mints))
	}
}
