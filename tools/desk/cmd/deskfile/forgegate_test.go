package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forgegate_test.go — the forge-support gate (#687).
//
// deskfile's issue ops shell `gh` (GitHub only). On a repo whose configured forge is GitLab
// they used to reach GitHub's API for a repo that does not exist there and fail with a
// misleading "Could not resolve to a Repository" — sending the operator to check their token
// when the real cause is that deskfile has no GitLab path. requireSupportedForge replaces that
// with a NAMED refusal (exit 5) emitted BEFORE any gh call.
//
// FAIL-FIRST: without the gate, each verb below proceeds to shell the fake gh — `check`/`new`
// reach the dedupe search (fake gh returns an empty result set, so the tool would exit 0 or
// pass on to create) and `attach` reaches `gh issue view`; every one makes at least one gh
// call and none returns exit 5. The two assertions each case makes — exit == ExitRefused and
// ZERO gh calls — therefore both fail on the pre-gate code, and the message assertion pins the
// refusal to THIS gate rather than any other exit-5 path.

// plantRosterWithForges rewrites the test HOME's roster to the base fixture plus an
// ASSAY_REPO_FORGES binding, then reloads config. HOME must already be set (by withEnv).
func plantRosterWithForges(t *testing.T, forges string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir config home: %v", err)
	}
	roster := fixtureRoster + "ASSAY_REPO_FORGES=" + forges + "\n"
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatalf("write roster: %v", err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}

// assertGitLabRefusal is the shared shape: exit 5, the message names the forge and the
// GitHub-only limitation, and NOT ONE gh call was made (the refusal precedes every gh op).
func assertGitLabRefusal(t *testing.T, rc int, out string, calls [][]string) {
	t.Helper()
	if rc != deskkit.ExitRefused {
		t.Fatalf("want exit %d (refused), got %d\noutput:\n%s", deskkit.ExitRefused, rc, out)
	}
	for _, want := range []string{"gitlab", "GitHub only", allowedRepo} {
		if !strings.Contains(out, want) {
			t.Fatalf("refusal message missing %q\noutput:\n%s", want, out)
		}
	}
	if gh := ghCalls(calls); len(gh) != 0 {
		t.Fatalf("gate must refuse BEFORE any gh call, but %d were made: %v", len(gh), gh)
	}
}

func TestGitLabForgeRefusesNew(t *testing.T) {
	calls := withEnv(t)
	plantRosterWithForges(t, allowedRepo+"=gitlab")
	body := bodyFileWith(t, "a real blocker that needs a human")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "escalate the gitlab filing blocker", "--body-file", body,
		"--raised-by", "reviewer", "--to", "desk", "--label", "help wanted"})
	assertGitLabRefusal(t, rc, out, *calls)
}

// TestGitLabForgeRefusesForceNew — --force-new is not an escape hatch: the create is itself
// the failing GitHub call, so the gate must refuse it too, before the create is attempted.
func TestGitLabForgeRefusesForceNew(t *testing.T) {
	calls := withEnv(t)
	plantRosterWithForges(t, allowedRepo+"=gitlab")
	body := bodyFileWith(t, "a real blocker that needs a human")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "escalate the gitlab filing blocker", "--body-file", body,
		"--force-new", "--reason", "urgent escalation"})
	assertGitLabRefusal(t, rc, out, *calls)
}

func TestGitLabForgeRefusesCheck(t *testing.T) {
	calls := withEnv(t)
	plantRosterWithForges(t, allowedRepo+"=gitlab")

	rc, out := runCapture([]string{"check", "-R", allowedRepo,
		"--title", "escalate the gitlab filing blocker"})
	assertGitLabRefusal(t, rc, out, *calls)
}

func TestGitLabForgeRefusesAttach(t *testing.T) {
	calls := withEnv(t)
	plantRosterWithForges(t, allowedRepo+"=gitlab")
	body := bodyFileWith(t, "an observation for the class issue")

	rc, out := runCapture([]string{"attach", "-R", allowedRepo,
		"--to", "42", "--body-file", body})
	assertGitLabRefusal(t, rc, out, *calls)
}

// TestGitHubForgeStillFiles proves the gate is a no-op on a GitHub-configured repo: the same
// `check` that the GitLab binding refuses runs through to the dedupe search and exits 0 when
// the forge is explicitly github. This is the "only ever ADDS a refusal on a non-GitHub forge,
// never turns a working GitHub filing into one" property, held against an EXPLICIT binding so
// it does not depend on the test host's origin remote.
func TestGitHubForgeStillFiles(t *testing.T) {
	calls := withEnv(t)
	plantRosterWithForges(t, allowedRepo+"=github")

	rc, out := runCapture([]string{"check", "-R", allowedRepo, "--title", "some unique title here"})
	if rc != deskkit.ExitOK {
		t.Fatalf("github-configured check should pass (exit 0), got %d\noutput:\n%s", rc, out)
	}
	// The dedupe search DID run — the gate did not stand in front of the gh path.
	if !anyCall(ghCalls(*calls), "search", "issues") {
		t.Fatalf("expected the dedupe `gh search issues` to run on a github repo; calls: %v", ghCalls(*calls))
	}
}
