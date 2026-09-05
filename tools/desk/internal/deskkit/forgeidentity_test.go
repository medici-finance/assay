package deskkit

// Property tests for forge-qualified identity (the forge-qualified-identity brief).
//
// Each test here pins a security property of the grammar, the renderings, or the
// forge-agreement gate, and every negative path was chosen because its failure mode is
// SILENT: a same-named account on the wrong forge, a bare slug quietly accepted, or a
// mismatched entry dropped rather than refused would each surface only as a believed
// write by an actor the trust gate never meant to admit.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forgeRoster is a minimal configured roster (bless + one bot line) for these tests.
func forgeRoster(botSlugs string, extra map[string]string) map[string]string {
	m := map[string]string{
		EnvBlessLogin:      "ada:2001",
		EnvTrustedBotSlugs: botSlugs,
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

// TestRosterForgeQualifiedEntry — Verify row 2. An explicit gitlab: and an explicit
// github: entry both parse, and the forge is recorded (explicit, not inferred) on each.
func TestRosterForgeQualifiedEntry(t *testing.T) {
	withRoster(t, forgeRoster(
		"reviewer=gitlab:assay-reviewer-bot:41987965,worker=github:assay-worker-app:300000004", nil))
	c := EffectiveConfig()

	gl, ok := c.BotIdents["assay-reviewer-bot"]
	if !ok {
		t.Fatal("gitlab entry did not parse into BotIdents")
	}
	if gl.Forge != ForgeGitLab || gl.ForgeInferred {
		t.Fatalf("gitlab entry forge=%q inferred=%v, want gitlab + explicit", gl.Forge, gl.ForgeInferred)
	}
	if gl.ID != 41987965 {
		t.Fatalf("gitlab entry id=%d, want 41987965", gl.ID)
	}

	gh, ok := c.BotIdents["assay-worker-app"]
	if !ok {
		t.Fatal("github entry did not parse into BotIdents")
	}
	if gh.Forge != ForgeGitHub || gh.ForgeInferred {
		t.Fatalf("explicit github entry forge=%q inferred=%v, want github + explicit", gh.Forge, gh.ForgeInferred)
	}
	if gh.ID != 300000004 {
		t.Fatalf("github entry id=%d, want 300000004", gh.ID)
	}

	// Renderings: GitLab registers the username; GitHub the two decorated forms.
	if !c.Logins["assay-reviewer-bot"] {
		t.Error("gitlab username is not an accepted login")
	}
	if !c.Logins["assay-worker-app[bot]"] || !c.Logins["app/assay-worker-app"] {
		t.Error("github renderings are not both accepted")
	}
}

// TestRosterUnqualifiedEntryDefaultsToGitHub — Verify row 3. A legacy unqualified entry
// still parses, resolves to github, and is flagged INFERRED rather than explicit.
func TestRosterUnqualifiedEntryDefaultsToGitHub(t *testing.T) {
	withRoster(t, forgeRoster("reviewer=assay-reviewer-app:300000004", nil))
	c := EffectiveConfig()

	b, ok := c.BotIdents["assay-reviewer-app"]
	if !ok {
		t.Fatal("legacy unqualified entry did not parse")
	}
	if b.Forge != ForgeGitHub {
		t.Fatalf("legacy entry forge=%q, want github", b.Forge)
	}
	if !b.ForgeInferred {
		t.Fatal("a legacy github forge must be flagged INFERRED — a caller has to be able to tell it " +
			"from an explicit github, which is what makes the backward-compat exemption addressable")
	}
	if b.ID != 300000004 {
		t.Fatalf("legacy entry id=%d, want 300000004", b.ID)
	}
	if !c.Logins["assay-reviewer-app[bot]"] || !c.Logins["app/assay-reviewer-app"] {
		t.Error("legacy github renderings are not both accepted")
	}
}

// TestRosterBareSlugStillRejected — Verify row 5. The pre-existing property must survive
// the new grammar: on GitHub the undecorated App slug is never accepted (only the
// decorated forms are); on GitLab the GitHub decorations are never minted for the
// username (only the genuine username is).
func TestRosterBareSlugStillRejected(t *testing.T) {
	// GitHub entry: the bare app slug is rejected; the decorated form is accepted.
	withRoster(t, forgeRoster("reviewer=github:assay-reviewer-app:300000004", nil))
	if TrustedAuthor("assay-reviewer-app") {
		t.Error("bare GitHub App slug accepted — the username-squatting hole the rendering set closes")
	}
	if !TrustedAuthor("assay-reviewer-app[bot]") {
		t.Error("decorated GitHub rendering should be accepted")
	}

	// GitLab entry: the username dressed in GitHub bot decoration is rejected on the
	// wrong-forge namespace; the genuine username is accepted.
	withRoster(t, forgeRoster("reviewer=gitlab:assay-reviewer-bot:41987965", nil))
	if TrustedAuthor("assay-reviewer-bot[bot]") {
		t.Error("a GitLab username dressed as <name>[bot] was accepted — cross-forge decoration must not spoof")
	}
	if TrustedAuthor("app/assay-reviewer-bot") {
		t.Error("a GitLab username dressed as app/<name> was accepted — cross-forge decoration must not spoof")
	}
	if !TrustedAuthor("assay-reviewer-bot") {
		t.Error("the genuine GitLab service-account username should be accepted")
	}
}

// TestRosterForgeMismatchRefuses — Verify row 4 (negative path). A gitlab: entry against
// a repo whose resolved forge is github yields a refusal naming BOTH forges; the entry is
// NOT silently dropped (it parses) and NOT accepted (ForgeFor and AssertRoleForgeMatches
// both refuse before any credential is read).
func TestRosterForgeMismatchRefuses(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "gh-repo"}
	withRoster(t, forgeRoster(
		"reviewer=gitlab:assay-reviewer-bot:41987965",
		map[string]string{EnvRepoForges: repo.Slug() + "=github"}))

	// Parsed and stored — not silently dropped.
	if _, ok := EffectiveConfig().BotIdents["assay-reviewer-bot"]; !ok {
		t.Fatal("the mismatched entry was silently dropped — it must parse, then be refused at use")
	}

	// ForgeFor refuses before custody.
	f, err := ForgeFor(repo, "reviewer")
	if err == nil {
		t.Fatalf("a gitlab entry was accepted for a github repo: %#v", f)
	}
	if f != nil {
		t.Fatal("ForgeFor returned a non-nil Forge alongside the refusal")
	}
	if got := ExitCodeOf(err); got != ExitRefused {
		t.Fatalf("exit=%d, want %d (refused) — a forge mismatch is a deployment error, not a could-not-check", got, ExitRefused)
	}
	m := strings.ToLower(err.Error())
	if !strings.Contains(m, "gitlab") || !strings.Contains(m, "github") {
		t.Fatalf("the refusal must name BOTH forges so an operator can act on it: %v", err)
	}

	// The standalone repo-taking form refuses identically.
	if AssertRoleForgeMatches("reviewer", repo) == nil {
		t.Fatal("AssertRoleForgeMatches did not refuse the forge mismatch")
	}
}

// TestForgeAgreementExemptsInferredGitHub pins the backward-compat carve-out the human
// gate is confirming: a LEGACY unqualified (inferred-github) entry is not refused even
// against a non-github repo — breaking every deployed roster is not how a field is added.
func TestForgeAgreementExemptsInferredGitHub(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "gl-repo"}
	withRoster(t, forgeRoster(
		"reviewer=assay-reviewer-app:300000004", // unqualified => inferred github
		map[string]string{EnvRepoForges: repo.Slug() + "=gitlab"}))
	if err := AssertRoleForgeMatches("reviewer", repo); err != nil {
		t.Fatalf("an inferred-github (legacy) entry was refused against a gitlab repo: %v — the "+
			"backward-compat exemption is what keeps deployed rosters working", err)
	}
}

// TestForgeQualifiedEntryEchoed — the P3 echo carries the forge-qualified identity so a
// GitLab bot is as visible in run output as a GitHub one.
func TestForgeQualifiedEntryEchoed(t *testing.T) {
	withRoster(t, forgeRoster("reviewer=gitlab:assay-reviewer-bot:41987965", nil))
	var b strings.Builder
	for _, l := range EffectiveConfig().EffectiveConfigLines() {
		b.WriteString(l)
		b.WriteString("\n")
	}
	if !strings.Contains(b.String(), "gitlab:assay-reviewer-bot:41987965") {
		t.Fatalf("the run echo does not carry the forge-qualified gitlab identity:\n%s", b.String())
	}
}

// writeRosterFile is the low-level installer used to prove a MALFORMED entry refuses the
// WHOLE configuration (fail-closed), which withRoster's Configured() convenience hides.
func writeRosterFile(t *testing.T, vals map[string]string) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for k, v := range vals {
		fmt.Fprintf(&b, "%s=%s\n", k, v)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	ReloadConfig()
	t.Cleanup(ReloadConfig)
}

// TestRosterEmptyForgeSegmentRefuses proves the grammar stays fail-closed: an entry with
// a forge qualifier but no slug is a parse failure that refuses the whole roster, never a
// silently registered empty identity.
func TestRosterEmptyForgeSegmentRefuses(t *testing.T) {
	writeRosterFile(t, forgeRoster("reviewer=gitlab:", nil))
	if EffectiveConfig().Configured() {
		t.Fatal("an entry with a forge qualifier but no slug was accepted — it must refuse the whole roster")
	}
}
