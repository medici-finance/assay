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
	forgeForFn = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, deskkit.ForgeKind, error) {
		return nil, deskkit.ForgeRepo{}, "", deskkit.Refused(
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
	forgeForFn = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, deskkit.ForgeKind, error) {
		return nil, deskkit.ForgeRepo{}, "", deskkit.Unverifiable(
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

// TestDeskfileMissingLabelHintIsForgeSelectedOnGitLab — #887 item 2. On a GitLab-configured
// repo the label-missing NOTICE must name a remedy the operator can actually run there
// (`glab label create`), never the GitHub CLI: a `gh label create` printed on a GitLab repo
// cannot work, so the label never gets created and every later filing stays UNSTAMPED. The
// degrade itself (file UNSTAMPED / UNADDRESSED rather than fail the create) is unchanged, and
// the parenthetical no longer claims a `gh issue create` on a path that shells nothing.
func TestDeskfileMissingLabelHintIsForgeSelectedOnGitLab(t *testing.T) {
	withEnv(t)
	plantRosterWithForges(t, allowedRepo+"=gitlab")
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "bug", "question")) // no raised-by:* / to:* at all
	body := bodyFileWith(t, "a gitlab filing whose stamp labels are not created yet")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "gitlab filing with missing stamp labels", "--body-file", body,
		"--raised-by", "reviewer", "--to", "worker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0 — a missing label must never stop a filing; out=%s", rc, out)
	}
	if curForge.filed == nil {
		t.Fatal("new did not file through the forge backend on a GitLab-configured repo")
	}
	for _, want := range []string{
		"glab label create --name raised-by:reviewer --repo " + allowedRepo + ` --description "filed by the reviewer desk"`,
		"glab label create --name to:worker --repo " + allowedRepo + ` --description "addressed to the worker desk"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the NOTICE must carry the GitLab remedy command %q; out=%s", want, out)
		}
	}
	for _, banned := range []string{"gh label create", "gh issue create"} {
		if strings.Contains(out, banned) {
			t.Errorf("a GitLab-resolved repo must not be told to run %q; out=%s", banned, out)
		}
	}
	if d := lastNewDetail(t); !strings.Contains(d, "raised-by=UNSTAMPED:label-missing") ||
		!strings.Contains(d, "to=UNADDRESSED:label-missing") {
		t.Fatalf("audit detail = %q, want both label-missing outcomes recorded", d)
	}
}

// TestDeskfileMissingLabelHintStaysGitHubOnGitHub — the GitHub twin: the hint is SELECTED, not
// swapped wholesale, so a GitHub-resolved repo keeps the `gh label create … --force` command the
// raisedby/addressto tests already pin.
func TestDeskfileMissingLabelHintStaysGitHubOnGitHub(t *testing.T) {
	withEnv(t)
	plantRosterWithForges(t, allowedRepo+"=github")
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "bug"))
	body := bodyFileWith(t, "a github filing whose stamp label is not created yet")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "github filing with a missing stamp label", "--body-file", body,
		"--raised-by", "reviewer"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; out=%s", rc, out)
	}
	if !strings.Contains(out, "gh label create raised-by:reviewer --repo "+allowedRepo) {
		t.Fatalf("a GitHub-resolved repo must keep the gh remedy; out=%s", out)
	}
	if strings.Contains(out, "glab ") {
		t.Fatalf("a GitHub-resolved repo must not be told to run glab; out=%s", out)
	}
}

// TestDeskfileHelpNoLongerClaimsGitHubOnlyRefusal — #887 item 3. `deskfile --help` documented the
// interim #691 refusal ("shell gh and support GitHub ONLY … every verb REFUSES (exit 5)") after
// the verbs had been routed through the forge backend and GitLab was served. That paragraph told
// an operator to escalate to a human instead of using a tool that works; it must describe the
// delivered behaviour, and the stamp blurbs must not promise a `gh label create` on every forge.
func TestDeskfileHelpNoLongerClaimsGitHubOnlyRefusal(t *testing.T) {
	for _, stale := range []string{
		"GitHub ONLY",
		"every verb REFUSES",
		"not yet delivered",
		"the NOTICE prints the one-off gh label create",
		"a NOTICE prints the one-off gh label",
		"ambient gh credential",
	} {
		if strings.Contains(usage, stale) {
			t.Errorf("deskfile --help still carries the retired claim %q", stale)
		}
	}
	for _, want := range []string{"GitHub AND\nGitLab are both served", "glab label create on GitLab"} {
		if !strings.Contains(usage, want) {
			t.Errorf("deskfile --help must state the delivered forge behaviour %q", want)
		}
	}
}
