package main

// The RELEASE HOME is configuration (ASSAY_RELEASE_REPO), not a compiled constant.
//
// Two properties, and the second is the one worth the tests. First: with the variable
// UNSET, everything about this tool is byte-identical to what the compiled-in slug
// gave — an adopter who configures nothing must not notice that anything moved.
// Second: making it configurable did not make the tool RETARGETABLE by a caller. The
// value comes from the roster's config-home file, never from argv, stdin or the
// environment, and whatever it resolves to is still screened by IsAllowedRepo.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// setReleaseRepo rewrites the fixture roster inside home with an ASSAY_RELEASE_REPO
// line (or without one, for slug ""), and reloads the configuration.
//
// It writes a FILE rather than exporting a variable on purpose: deskrelease declares
// ClassWrite, which does not consult the environment at all. A test that "configured"
// the release home with os.Setenv would pass against a tool that ignores it — and
// TestReleaseRepoUnsettableFromEnvironment below pins exactly that.
//
// It takes the home EXPLICITLY because newHarness plants its own private HOME with a
// clean fixture roster in it. Staging the roster first and building the harness after
// silently discards the staging — the first draft of these tests did exactly that and
// the C-4 row passed for the wrong reason (nothing was configured, so nothing was
// refused). Harness-based rows pass h.home here, AFTER newHarness.
func setReleaseRepo(t *testing.T, home, slug string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("staging the roster: %v", err)
	}
	body := fixtureRoster
	if slug != "" {
		body += "ASSAY_RELEASE_REPO=" + slug + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(body), 0o600); err != nil {
		t.Fatalf("staging the roster: %v", err)
	}
	deskkit.ReloadConfig()

	old := repoSlug
	t.Cleanup(func() {
		repoSlug = old
		deskkit.ReloadConfig()
	})
}

// withReleaseRepo is setReleaseRepo for a test with no harness: it makes its own
// private HOME first.
func withReleaseRepo(t *testing.T, slug string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	setReleaseRepo(t, home, slug)
}

// TestReleaseRepoUnsetKeepsShippedDefault — Verify row 8. With nothing configured, the
// tool targets the shipped public home, exactly as the compiled-in slug did.
func TestReleaseRepoUnsetKeepsShippedDefault(t *testing.T) {
	withReleaseRepo(t, "")
	if deskkit.ConfiguredReleaseRepo() != "" {
		t.Fatal("the fixture roster configures a release home; this test cannot see the default")
	}
	repoSlug = "sentinel/unset-me"
	applyConfiguredReleaseRepo()
	if repoSlug != "sentinel/unset-me" {
		t.Fatalf("an UNSET %s still wrote to repoSlug (%q) — 'unset is byte-identical' is not true",
			deskkit.EnvReleaseRepo, repoSlug)
	}
	if defaultReleaseRepo != "medici-finance/assay" {
		t.Fatalf("the shipped default moved to %q", defaultReleaseRepo)
	}
}

// TestReleaseRepoUnsetCutsAgainstTheDefault is the end-to-end half of row 8: a real
// `cut` with nothing configured writes the tag in the default repo.
func TestReleaseRepoUnsetCutsAgainstTheDefault(t *testing.T) {
	h := newHarness(t)
	setReleaseRepo(t, h.home, "")
	repoSlug = defaultReleaseRepo

	if code := run([]string{"cut", goodTag}); code != 0 {
		t.Fatalf("exit %d, want 0: %s", code, h.errb.String())
	}
	if repoSlug != "medici-finance/assay" {
		t.Fatalf("the release target drifted to %q with the variable unset", repoSlug)
	}
	if h.gh.posts != 1 {
		t.Fatalf("posts = %d, want exactly 1", h.gh.posts)
	}
}

// TestConfiguredReleaseRepoRetargetsTheTool — the point of the variable: a fork names
// its own release home without editing and rebuilding the binary.
func TestConfiguredReleaseRepoRetargetsTheTool(t *testing.T) {
	// A repo that IS in the fixture write-authorisation set, so the C-4 screen passes
	// and what is under test is the retarget rather than the refusal.
	withReleaseRepo(t, "example-org/console")
	repoSlug = defaultReleaseRepo
	applyConfiguredReleaseRepo()
	if repoSlug != "example-org/console" {
		t.Fatalf("repoSlug = %q, want the configured release home", repoSlug)
	}
}

// TestConfiguredReleaseRepoStaysScreened — C-4 stands regardless of the
// configured value. This is the property that keeps the variable from being a way to
// widen the write boundary: it SELECTS a target, it never ADMITS one.
func TestConfiguredReleaseRepoStaysScreened(t *testing.T) {
	h := newHarness(t)
	setReleaseRepo(t, h.home, "attacker/elsewhere")
	repoSlug = defaultReleaseRepo

	if code := run([]string{"cut", goodTag}); code != deskkit.ExitRefused {
		t.Fatalf("exit %d, want 5 — a configured release home outside the write set must be "+
			"refused exactly as any out-of-set repo is", code)
	}
	if repoSlug != "attacker/elsewhere" {
		t.Fatalf("the configuration was not applied (%q), so this proves nothing about the screen", repoSlug)
	}
	if !strings.Contains(h.errb.String(), "outside the fixed desk repo set") {
		t.Fatalf("stderr does not name the repo-scope refusal: %s", h.errb.String())
	}
	if len(h.gh.calls()) != 0 {
		t.Fatalf("acted on an out-of-set repo: %v", h.gh.calls())
	}
}

// TestReleaseRepoUnsettableFromEnvironment is the property that makes the
// conversion safe: deskrelease ACTS, so it is ClassWrite, so the environment is not a
// route to its release home. A steered session that can export a variable still cannot
// point this tool at another repository.
func TestReleaseRepoUnsettableFromEnvironment(t *testing.T) {
	withReleaseRepo(t, "") // nothing in the FILE
	t.Setenv(deskkit.EnvReleaseRepo, "attacker/elsewhere")
	deskkit.ReloadConfig()

	if got := deskkit.ConfiguredReleaseRepo(); got != "" {
		t.Fatalf("the environment reached the release home (%q). deskrelease is a write-class "+
			"tool: an exported variable must have no effect on it", got)
	}
	repoSlug = defaultReleaseRepo
	applyConfiguredReleaseRepo()
	if repoSlug != defaultReleaseRepo {
		t.Fatalf("repoSlug = %q — the environment retargeted the release tool", repoSlug)
	}
}

// TestReleaseRepoUnsettableFromArgv — the argv parser has no --repo, and adding one
// is the change this pins against. Every shape is a usage or refusal exit, never a
// retarget.
func TestReleaseRepoUnsettableFromArgv(t *testing.T) {
	for _, argv := range [][]string{
		{"cut", goodTag, "--repo", "attacker/elsewhere"},
		{"cut", goodTag, "--repo=attacker/elsewhere"},
		{"cut", goodTag, "attacker/elsewhere"},
	} {
		t.Run(strings.Join(argv, " "), func(t *testing.T) {
			h := newHarness(t)
			setReleaseRepo(t, h.home, "")
			repoSlug = defaultReleaseRepo
			if code := run(argv); code == 0 {
				t.Fatalf("argv %v was ACCEPTED", argv)
			}
			if repoSlug != defaultReleaseRepo {
				t.Fatalf("argv retargeted the tool to %q", repoSlug)
			}
			if h.gh.posts != 0 {
				t.Fatalf("argv %v reached a write: %v", argv, h.gh.calls())
			}
		})
	}
}
