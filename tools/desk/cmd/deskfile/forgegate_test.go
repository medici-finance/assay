package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forgegate_test.go — the forge routing since the write-verbs-C migration.
//
// #687/#691 shipped an INTERIM named-refusal: deskfile shelled `gh` (GitHub only), so on a
// GitLab-configured repo it refused (exit 5) rather than emit a misleading GitHub error. This
// migration SUPERSEDES that: deskfile now reaches the forge through the resolver, and the GitLab
// backend serves the issue ops (dedupe search, label probe, create, attach). So a
// GitLab-configured repo FILES rather than refusing. The two facts the interim refusal traded on
// are pinned here: (1) GitLab no longer refuses with the "GitHub only" message, and (2) a repo
// whose forge cannot be resolved, or whose custody yields no token, still fails closed.

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

// TestDeskfileFilesOnGitLabThroughBackend — the #691 supersession. On a GitLab-configured repo
// `new` no longer refuses with the "GitHub only" message: it dedupes and FILES through the
// resolved backend. (The GitLab backend's actual wire for SearchIssues / ListLabels / FileIssue
// is pinned by the deskkit gitlab golden corpus; here the deskfile-side behaviour is that it
// routes through the forge rather than refusing.)
func TestDeskfileFilesOnGitLabThroughBackend(t *testing.T) {
	withEnv(t)
	plantRosterWithForges(t, allowedRepo+"=gitlab")
	body := bodyFileWith(t, "a real blocker that needs a human")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "escalate the gitlab filing blocker", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("new on a GitLab-configured repo should FILE (exit 0) now the backend serves GitLab, got %d\n%s", rc, out)
	}
	if strings.Contains(out, "GitHub only") {
		t.Fatalf("the interim #691 GitLab refusal is superseded and must not fire on a GitLab repo:\n%s", out)
	}
	if curForge.filed == nil {
		t.Fatal("new did not file through the forge backend on a GitLab-configured repo")
	}
}

// TestDeskfileRefusesWithoutMintedToken — the negative path. With the custody binding yielding no
// token (what forgeFor does when the mint fails / ForgeFor refuses), `new` REFUSES and files
// NOTHING, never falling back to an ambient identity — the retired ambient-credential design.
func TestDeskfileRefusesWithoutMintedToken(t *testing.T) {
	withEnv(t)
	rec := curForge
	forgeForFn = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return nil, deskkit.ForgeRepo{}, deskkit.Refused(
			"cannot obtain the session-role App installation token — ForgeFor never falls back to an ambient identity")
	}
	body := bodyFileWith(t, "should refuse without a token")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "a title with substantive words here", "--body-file", body})
	if rc == deskkit.ExitOK {
		t.Fatalf("new SUCCEEDED with no minted token — it must refuse, never fall back:\n%s", out)
	}
	if rec.filed != nil || rec.searchCalls != 0 {
		t.Fatalf("a forge op ran despite the custody refusal (filed=%v search=%d)", rec.filed != nil, rec.searchCalls)
	}
}

// TestDeskfileUnresolvableForgeCouldNotCheck — the RETAINED refusal: a repo whose forge cannot be
// resolved (the mint/resolve step returns a could-not-check) fails closed rather than assuming
// GitHub. Modelled by forgeForFn returning the resolver's Unverifiable.
func TestDeskfileUnresolvableForgeCouldNotCheck(t *testing.T) {
	withEnv(t)
	rec := curForge
	forgeForFn = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return nil, deskkit.ForgeRepo{}, deskkit.Unverifiable(
			"cannot resolve which forge serves "+repo+": configure ASSAY_REPO_FORGES", nil)
	}
	rc, out := runCapture([]string{"check", "-R", allowedRepo, "--title", "some unique title here"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("an unresolvable forge must be could-not-check (exit 6), got %d\n%s", rc, out)
	}
	if rec.searchCalls != 0 {
		t.Fatal("check searched despite an unresolvable forge — it must fail closed before any forge op")
	}
}

// TestGitHubForgeStillFiles proves a GitHub-configured repo is unaffected: the same `check`
// runs through to the dedupe search and exits 0.
func TestGitHubForgeStillFiles(t *testing.T) {
	calls := withEnv(t)
	plantRosterWithForges(t, allowedRepo+"=github")

	rc, out := runCapture([]string{"check", "-R", allowedRepo, "--title", "some unique title here"})
	if rc != deskkit.ExitOK {
		t.Fatalf("github-configured check should pass (exit 0), got %d\noutput:\n%s", rc, out)
	}
	if !anyCall(ghCalls(*calls), "search", "issues") {
		t.Fatalf("expected the dedupe search to run on a github repo; calls: %v", ghCalls(*calls))
	}
}
