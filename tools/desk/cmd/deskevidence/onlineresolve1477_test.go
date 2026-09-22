package main

// onlineresolve1477_test.go — the ONLINE attribution resolution for a GitLab verifier (#1477).
//
// On GitLab the file write reports NO author, so checkAttribution used to give up at
// could-not-check for EVERY GitLab landing. These tests pin the online counterpart to
// statusgen's offline display-name fallback: the landed commit's account is resolved to its
// USERNAME through the TYPED forge (GetCommit -> gitlabLoginForEmail, a GET /users?search=, never
// a forge CLI), which is the roster login the verifier binding carries — the field the commit's
// author_name (a DISPLAY name on GitLab) does not hold.
//
// The behaviour tests are FAIL-FIRST against the pre-fix checkAttribution: with author=="" it
// returned "attribution=could-not-check" unconditionally, so the "resolved online" accept and
// the online WRONG verdict did not exist. The negative controls keep it honest: an unresolvable
// account and an absent sha both STAY could-not-check (never rounded up to a pass), and a
// DIFFERENT resolved username is WRONG.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// gitlabVerifierRoster binds the verifier role to a GitLab service account, so
// RoleAppLogin("verifier") resolves to the bare USERNAME (GitLab has no `[bot]` rendering) —
// the value GetCommit resolves a commit's account to. example-org/tracker stays allowed so the
// end-to-end run() path is unchanged but for the forge.
const gitlabVerifierRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=verifier=gitlab:example-verifier-bot:41987969,worker=gitlab:example-worker-bot:41987966
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private
ASSAY_REPO_FORGES=example-org/tracker=gitlab
`

func plantGitLabVerifierRoster(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("planting the GitLab roster: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(gitlabVerifierRoster), 0o600); err != nil {
		t.Fatalf("planting the GitLab roster: %v", err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}

// TestOnlineAttributionResolvesGitLabVerifier is the issue's online path: author=="" (GitLab's
// write reports none), but the typed forge resolves the landed commit's account to the verifier
// USERNAME, so the attribution is PROVEN — not left could-not-check.
func TestOnlineAttributionResolvesGitLabVerifier(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	plantGitLabVerifierRoster(t, os.Getenv("HOME"))
	want, ok := deskkit.RoleAppLogin("verifier")
	if !ok || want != "example-verifier-bot" {
		t.Fatalf("precondition: GitLab verifier binding must resolve to the bare username; got %q (ok=%v)", want, ok)
	}

	f := &fakeForge{commitAuthorLogin: "example-verifier-bot"}
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}

	// FAIL-FIRST: pre-fix this returned "attribution=could-not-check" and err==nil.
	detail, err := checkAttribution(f, fr, "head-sha-1", "")
	if err != nil {
		t.Fatalf("a commit the forge resolves to the verifier username must be PROVEN, got err=%v (detail=%q)", err, detail)
	}
	if !strings.Contains(detail, "resolved online") {
		t.Errorf("the proven detail must say the account was resolved online; got %q", detail)
	}
	if f.getCommitCalls == 0 {
		t.Error("checkAttribution must have consulted the typed forge (GetCommit) to resolve the account")
	}
}

// TestOnlineAttributionWrongAccount — the forge resolves the commit to a DIFFERENT account than
// the bound verifier: PROVEN WRONG (an error), the same disposition an OTHER login gets on the
// GitHub path.
func TestOnlineAttributionWrongAccount(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	plantGitLabVerifierRoster(t, os.Getenv("HOME"))

	f := &fakeForge{commitAuthorLogin: "example-worker-bot"}
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
	detail, err := checkAttribution(f, fr, "head-sha-1", "")
	if err == nil {
		t.Fatalf("a commit resolved to a NON-verifier account must be proven WRONG, got no error (detail=%q)", detail)
	}
	if !strings.Contains(detail, "WRONG") || !strings.Contains(detail, "example-worker-bot") {
		t.Errorf("the WRONG detail must name the resolved account; got %q", detail)
	}
}

// TestOnlineAttributionUnresolvedStaysCouldNotCheck — the forge could not resolve the commit's
// account (empty AuthorLogin): STAYS could-not-check, never rounded up to a pass and never a
// fabricated rejection.
func TestOnlineAttributionUnresolvedStaysCouldNotCheck(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	plantGitLabVerifierRoster(t, os.Getenv("HOME"))

	f := &fakeForge{commitAuthorLogin: ""} // the forge resolved no account
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
	detail, err := checkAttribution(f, fr, "head-sha-1", "")
	if err != nil {
		t.Fatalf("an unresolvable account must be could-not-check (no error), got %v", err)
	}
	if !strings.Contains(detail, "could-not-check") {
		t.Errorf("an unresolvable account must report could-not-check; got %q", detail)
	}
}

// TestOnlineAttributionNoShaStaysCouldNotCheck — with no sha to resolve (e.g. the head sha could
// not be fetched), the online read is not even attempted and the verdict stays could-not-check.
func TestOnlineAttributionNoShaStaysCouldNotCheck(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	plantGitLabVerifierRoster(t, os.Getenv("HOME"))

	f := &fakeForge{commitAuthorLogin: "example-verifier-bot"}
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
	detail, err := checkAttribution(f, fr, "", "")
	if err != nil || !strings.Contains(detail, "could-not-check") {
		t.Fatalf("no sha must stay could-not-check, got (detail=%q, err=%v)", detail, err)
	}
	if f.getCommitCalls != 0 {
		t.Errorf("with no sha the forge must not be consulted; GetCommit was called %d time(s)", f.getCommitCalls)
	}
}

// TestGitLabDraftLandingResolvesAuthorOnline is the end-to-end plumbing: on the GitLab
// draft-landing path (default branch closed, side-branch write reports no author) the run reads
// the draft change's head sha and resolves the committing account online, so the audit records
// the attribution as PROVEN rather than could-not-check.
func TestGitLabDraftLandingResolvesAuthorOnline(t *testing.T) {
	f, _ := setupFake(t)
	// Re-plant a GitLab verifier roster over the GitHub fixture setupFake installed, so the
	// verifier binding (and thus RoleAppLogin) is GitLab-shaped for this run.
	plantGitLabVerifierRoster(t, os.Getenv("HOME"))

	f.defaultBranch = "main"                     // the closed default routes to the draft lane
	f.emptyAuthor = true                         // GitLab's write reports no author
	f.prHeadSHA = "head-sha-1"                   // the draft change's head sha
	f.commitAuthorLogin = "example-verifier-bot" // the forge resolves it to the verifier username

	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "row\n")

	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("GitLab draft-landing exit = %d, want 0", code)
	}
	if f.getCommitCalls == 0 {
		t.Fatal("the draft-landing path must resolve the committing account online (GetCommit)")
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "resolved online") {
		t.Fatalf("audit detail must record the online-resolved attribution; got %q", d)
	}
}
