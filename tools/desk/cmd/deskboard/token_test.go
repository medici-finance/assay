package main

// token_test.go — the read path authenticates as the session role's App, and a repo
// outside that App's installation is per-repo could-not-check rather than a dead board.
//
// FAIL-FIRST. Both halves were observed RED on the pre-fix code:
// TestReadInjectsTheAppTokenIntoTheChild saw an empty GH_TOKEN (the whole defect — the
// child fell through to the HOME keyring), and TestOutOfInstallationRepoIsCouldNotCheck
// returned exit 6 with no rows at all, taking the entire board down over one repo the App
// was never installed on.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// resetTokenState clears the process-wide per-owner token memo so one test's answer cannot
// leak into the next.
func resetTokenState(t *testing.T) {
	t.Helper()
	restore := func() {
		tokenState.mu.Lock()
		defer tokenState.mu.Unlock()
		tokenState.roleOnce = false
		tokenState.role = ""
		tokenState.byOwner = map[string]string{}
		tokenState.noticed = map[string]bool{}
	}
	restore()
	t.Cleanup(restore)
}

// stubToken binds the token seams to fixed answers for one test.
func stubToken(t *testing.T, role string, roleErr error, token string, tokenErr error) *[]string {
	t.Helper()
	resetTokenState(t)
	var asked []string
	prevRole, prevTok := sessionTokenRoleFn, ownerTokenFn
	t.Cleanup(func() { sessionTokenRoleFn, ownerTokenFn = prevRole, prevTok })
	sessionTokenRoleFn = func(verb string) (string, string, error) {
		return role, "pr-review-desk", roleErr
	}
	ownerTokenFn = func(r, owner string) (string, string, error) {
		asked = append(asked, r+" "+owner)
		return token, "/config/home/" + r + "-token-1", tokenErr
	}
	return &asked
}

func TestOwnerFromArgsReadsEveryShapeThisPackageEmits(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"pr list -R", []string{"pr", "list", "-R", "example-org/tracker", "--state", "open"}, "example-org"},
		{"pr diff -R", []string{"pr", "diff", "7", "-R", "medici-finance/assay"}, "medici-finance"},
		{"rest path", []string{"api", "repos/example-org/tracker/pulls/7/reviews?per_page=100&page=1"}, "example-org"},
		{"rest repo metadata", []string{"api", "repos/example-org/console"}, "example-org"},
		{"owner search", []string{"search", "prs", "--owner", "example-org", "--state", "open"}, "example-org"},
		{"graphql owner field", []string{"api", "graphql", "-f", "query=query(...)", "-f", "owner=medici-finance", "-f", "name=assay"}, "medici-finance"},
		{"unattributable", []string{"api", "rate_limit"}, ""},
	}
	for _, c := range cases {
		if got := ownerFromArgs(c.args); got != c.want {
			t.Errorf("%s: ownerFromArgs(%v) = %q, want %q", c.name, c.args, got, c.want)
		}
	}
}

// An argv the reader cannot attribute must yield NO token rather than one minted for some
// other account: a mismatched installation token does not 401, it reports "could not
// resolve to a repository", which reads like a missing repo instead of a wrong identity.
func TestUnattributableArgvGetsNoToken(t *testing.T) {
	stubToken(t, "reviewer", nil, "installation-token-stub", nil)
	if tok := ghTokenForOwner(ownerFromArgs([]string{"api", "rate_limit"})); tok != "" {
		t.Fatalf("an unattributable call was handed the token for some other account: %q", tok)
	}
}

// The core assertion of the fix: the token reaches the CHILD's environment. The original
// defect was exactly this step missing — the write verbs resolved a token, the read path
// never put one in GH_TOKEN, and gh fell through to the HOME keyring.
func TestReadInjectsTheAppTokenIntoTheChild(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A gh that reports only what it was handed as GH_TOKEN.
	shim := "#!/bin/sh\nprintf %s \"$GH_TOKEN\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GH_TOKEN", "")

	stubToken(t, "reviewer", nil, "installation-token-for-this-test", nil)

	out, err := ghRun("pr", "list", "-R", "example-org/tracker")
	if err != nil {
		t.Fatalf("ghRun: %v", err)
	}
	if string(out) != "installation-token-for-this-test" {
		t.Fatalf("the child saw GH_TOKEN=%q — the read still falls through to the HOME keyring", out)
	}
}

// No loop identity means no App role, and a READ then proceeds on the ambient credential
// rather than refusing: this is a diagnostic a human runs from a plain shell on their own
// login, and taking that away would cost more than it protects. It says so on stderr.
func TestNoLoopIdentityLeavesTheReadOnTheAmbientCredential(t *testing.T) {
	stubToken(t, "", deskkit.Refused("$DESK_LOOP is unset"), "installation-token-stub", nil)
	if tok := ghTokenForOwner("example-org"); tok != "" {
		t.Fatalf("a session with no App role was handed a token: %q", tok)
	}
}

// A per-owner token that cannot be resolved is memoised, so a 13-repo sweep does not shell
// out to the minter once per read for an owner it already knows it has no credential for.
func TestUnavailableOwnerTokenIsAskedForOnce(t *testing.T) {
	asked := stubToken(t, "reviewer", nil, "", errors.New("no installation for this account"))
	for i := 0; i < 5; i++ {
		if tok := ghTokenForOwner("example-org"); tok != "" {
			t.Fatalf("a failed lookup returned a token: %q", tok)
		}
	}
	if len(*asked) != 1 {
		t.Fatalf("the minter was asked %d times for one account (%v) — a failed lookup must be "+
			"remembered", len(*asked), *asked)
	}
}

// --- the secondary observation: out-of-installation repos ------------------------

// outOfInstallationErr is the error GitHub actually returns for a repo the authenticated
// App installation cannot see.
func outOfInstallationErr(repo string) error {
	return deskkit.Unverifiable("cannot read open PRs for "+repo,
		fmt.Errorf("gh pr list: GraphQL: Could not resolve to a Repository with the name '%s'. (repository)", repo))
}

// stubPRList installs a ghRun serving one open PR for every repo EXCEPT `unreachable`,
// which fails the way an out-of-installation repo fails.
func stubPRList(t *testing.T, unreachable string, failOther error) {
	t.Helper()
	prev := ghRun
	t.Cleanup(func() { ghRun = prev })
	ghRun = func(args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if len(args) >= 2 && args[0] == "pr" && args[1] == "list" {
			repo := ownerRepoFromDashR(args)
			if repo == unreachable {
				if failOther != nil {
					return nil, failOther
				}
				return nil, outOfInstallationErr(repo)
			}
			return []byte(`[{"number":7,"title":"a change","state":"OPEN","isDraft":true,` +
				`"author":{"login":"ada"},"createdAt":"2026-01-01T00:00:00Z","headRefOid":"abc123",` +
				`"mergeStateStatus":"CLEAN","statusCheckRollup":[{"__typename":"CheckRun","name":"ci",` +
				`"status":"COMPLETED","conclusion":"SUCCESS"}]}]`), nil
		}
		if strings.Contains(joined, "graphql") {
			return []byte(`{"data":{}}`), nil
		}
		return []byte("[]"), nil
	}
}

func ownerRepoFromDashR(args []string) string {
	for i, a := range args {
		if a == "-R" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestOutOfInstallationRepoIsCouldNotCheck(t *testing.T) {
	installFakeGH(t)
	stubToken(t, "reviewer", nil, "installation-token-stub", nil)
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
	stubToken(t, "reviewer", nil, "installation-token-stub", nil)
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
