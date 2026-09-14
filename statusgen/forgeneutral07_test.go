package main

// forgeneutral07_test.go — the discriminating controls for forge-neutral/07:
// statusgen's Evidence-actor check and verifyrun witness recognise the ACTING FORGE
// identity, on GitLab as well as GitHub, and an unrecognised forge is could-not-check,
// never a pass.
//
// The controls that matter here are the NEGATIVE ones. A check "fixed" by reporting
// every GitLab row backed passes the positive rows and defeats the whole purpose, so
// TestEvidenceActorSelfAttestedStillUnbacked is the row that fails on a rubber stamp,
// and TestEvidenceActorUnknownForgeIsCouldNotCheck is the row that fails on a forge
// resolving to a pass instead of could-not-check.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GitLab fixture identities. Neutral example names and ids — the roster is adopter
// configuration, and a fixture keyed to one house's logins would not prove the grammar.
const (
	glVerifierName  = "example-verifier-bot"
	glVerifierEmail = "service_account_group_9619193_v1@noreply.gitlab.com"
	glWorkerName    = "example-worker-bot"
	glWorkerEmail   = "service_account_group_9619193_w1@noreply.gitlab.com"
)

// withRemoteOrigin stubs the origin-remote reader so detectForge resolves a chosen
// forge without a real remote, matching forge_test.go's injection pattern. An empty url
// resolves to forgeUnknown (as a repo with no origin would).
func withRemoteOrigin(t *testing.T, url string) {
	t.Helper()
	prev := remoteOriginURL
	remoteOriginURL = func(string) (string, error) { return url, nil }
	t.Cleanup(func() { remoteOriginURL = prev })
}

// gitlabRoster is a Problem-free roster binding a GitLab verifier and worker. It is
// fully configured (bless + a trusted human) so the tests exercise the real load path.
func gitlabRoster() map[string]string {
	return map[string]string{
		scanEnvBlessLogin:    "ada:100001",
		scanEnvTrustedLogins: "ada:100001",
		scanEnvTrustedBotSlugs: "verifier=gitlab:" + glVerifierName + ":41987969," +
			"worker=gitlab:" + glWorkerName + ":41987966",
	}
}

// TestEvidenceActorGitLabVerifier (Verify row 2). Evidence committed by a
// gitlab:-qualified verifier service account is reported BACKED — the row the pilot
// (D-3) reported as unbacked because the check could not recognise a GitLab identity.
// The worker-committed twin beside it stays flagged, so this is not "everything backed".
func TestEvidenceActorGitLabVerifier(t *testing.T) {
	scanWithRoster(t, gitlabRoster())

	root := t.TempDir()
	evActorGit(t, root, "init", "-q", "-b", "main")

	table := "| # | Command | Result | Date | Runner |\n" +
		"|---|---------|--------|------|--------|\n" +
		"| 1 | `true` | pass exit=0 | 2026-09-02 | " + glVerifierName + " |\n"

	mk := func(stream string) string {
		dir := filepath.Join(root, "docs", "streams", stream)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "brief-01-x.md"),
			[]byte(briefWithEvidence(table)), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	backedDir := mk("glbacked")
	selfDir := mk("glself")

	// The worker authors both files first; then the verifier re-commits the backed
	// stream's Evidence. The only difference between the two rows is which GitLab
	// service account authored the commit carrying its Evidence lines.
	gitCommitAs(t, root, glWorkerName, glWorkerEmail, "worker writes both briefs")
	if err := os.WriteFile(filepath.Join(backedDir, "brief-01-x.md"),
		[]byte(briefWithEvidence(table+"| 2 | `true` | pass exit=0 | 2026-09-02 | "+glVerifierName+" |\n")),
		0o644); err != nil {
		t.Fatal(err)
	}
	gitCommitAs(t, root, glVerifierName, glVerifierEmail, "verifier commits Evidence for glbacked/01")

	streams := []*Stream{
		{Name: "glbacked", Dir: backedDir, Briefs: []Brief{{Num: "01", Status: "verified", Verified: "2026-09-02 verifier"}}},
		{Name: "glself", Dir: selfDir, Briefs: []Brief{{Num: "01", Status: "verified", Verified: "2026-09-02 verifier"}}},
	}
	joined := strings.Join(evidenceActorNotices(root, streams), "\n")
	if strings.Contains(joined, "glbacked/01") {
		t.Fatalf("a GitLab verifier's Evidence must read as BACKED, not flagged (the D-3 fix).\nnotices:\n%s", joined)
	}
	if !strings.Contains(joined, "glself/01") {
		t.Fatalf("the worker-self-attested GitLab row must be flagged unbacked.\nnotices:\n%s", joined)
	}
}

// TestEvidenceActorGitHubVerifierUnchanged (Verify row 3). The GitHub id-pinned path
// behaves exactly as before — a regression here would silently retire the control on
// the forge it already worked on.
func TestEvidenceActorGitHubVerifierUnchanged(t *testing.T) {
	p := evidenceActorPolicy{
		Verifier:      actorRef{Login: "assay-verifier-app", ID: 300000005},
		VerifierForge: forgeGitHub,
		Humans:        []actorRef{{Login: "ada", ID: 100001}},
	}
	if !p.idPinned() {
		t.Fatal("a GitHub verifier carrying a numeric id must report id-pinned")
	}
	cases := []struct {
		name, email string
		want        actorVerdict
		reasonHas   string
	}{
		{fixtureVerifierName, fixtureVerifierEmail, actorVerifier, "verifier App"},
		{fixtureHumanName, fixtureHumanEmail, actorHuman, "human:ada"},
		{fixtureWorkerName, fixtureWorkerEmail, actorRejected, "does not accept"},
		{fixtureVerifierName, "999+assay-verifier-app[bot]@users.noreply.github.com", actorImpostor, "pins the verifier role to id"},
		{fixtureVerifierName, "me@example.com", actorImpostor, "free text"},
	}
	for _, c := range cases {
		got, reason := p.classify(c.name, c.email)
		if got != c.want {
			t.Errorf("classify(%q,%q) = %v, want %v (reason: %s)", c.name, c.email, got, c.want, reason)
		}
		if !strings.Contains(reason, c.reasonHas) {
			t.Errorf("classify(%q,%q) reason must name %q; got: %s", c.name, c.email, c.reasonHas, reason)
		}
	}
}

// TestEvidenceActorSelfAttestedStillUnbacked (Verify row 4) — THE discriminating
// negative control. Evidence committed by the IMPLEMENTER's identity is reported
// unbacked on BOTH forges, while the verifier's own commit stays backed on both.
// Without this row the brief could ship a check that reports everything backed and pass
// rows 2 and 3.
func TestEvidenceActorSelfAttestedStillUnbacked(t *testing.T) {
	// GitHub verifier binding.
	gh := evidenceActorPolicy{Verifier: actorRef{Login: "assay-verifier-app", ID: 300000005}, VerifierForge: forgeGitHub}
	if v, _ := gh.classify(fixtureWorkerName, fixtureWorkerEmail); v != actorRejected {
		t.Errorf("GitHub: the implementer's own Evidence commit must be unbacked, got %v", v)
	}
	if v, _ := gh.classify(fixtureVerifierName, fixtureVerifierEmail); v != actorVerifier {
		t.Errorf("GitHub anti-rubber-stamp: the verifier's own commit must be backed, got %v", v)
	}

	// GitLab verifier binding.
	gl := evidenceActorPolicy{Verifier: actorRef{Login: glVerifierName}, VerifierForge: forgeGitLab}
	if v, _ := gl.classify(glWorkerName, glWorkerEmail); v != actorRejected {
		t.Errorf("GitLab: the implementer's own service-account commit must be unbacked, got %v", v)
	}
	if v, _ := gl.classify(glVerifierName, glVerifierEmail); v != actorVerifier {
		t.Errorf("GitLab anti-rubber-stamp: the verifier's own commit must be backed, got %v", v)
	}
	// A non-service-account address must not back a GitLab row either.
	if v, _ := gl.classify(glVerifierName, "someone@example.com"); v != actorRejected {
		t.Errorf("GitLab: a non-service-account address must be unbacked even under the verifier's name, got %v", v)
	}
}

// TestEvidenceActorUnknownForgeIsCouldNotCheck (Verify row 5) — negative path. An entry
// naming an unrecognised forge yields could-not-check naming the forge, asserted on the
// MESSAGE TEXT (not merely on a non-zero result), so a "clean" answer fails the row. The
// roster still loads (Configured), which is what distinguishes the dedicated third
// could-not-check reason from a whole-roster parse refusal.
func TestEvidenceActorUnknownForgeIsCouldNotCheck(t *testing.T) {
	scanWithRoster(t, map[string]string{
		scanEnvBlessLogin:      "ada:100001",
		scanEnvTrustedLogins:   "ada:100001",
		scanEnvTrustedBotSlugs: "verifier=bitbucket:some-verifier:12345",
	})
	if !scanEffectiveConfig().Configured() {
		t.Fatalf("the roster with an unrecognised-forge verifier must still LOAD (could-not-check is a "+
			"verdict about the forge, not a parse refusal): %v", scanEffectiveConfig().Problems)
	}

	p := evidenceActorPolicyFromRoster()
	if p.Unavailable == "" {
		t.Fatal("an unrecognised verifier forge must make the policy Unavailable (could-not-check)")
	}
	if !strings.Contains(p.Unavailable, "bitbucket") {
		t.Errorf("the could-not-check reason must NAME the forge; got: %s", p.Unavailable)
	}
	if !strings.Contains(p.Unavailable, "does not understand") {
		t.Errorf("the reason must say the build does not understand the forge; got: %s", p.Unavailable)
	}

	// The full notice path: exactly one could-not-check notice, naming the forge, and
	// NOT reporting any row backed/unbacked.
	notices := evidenceActorNotices(t.TempDir(), []*Stream{{Name: "x"}})
	if len(notices) != 1 {
		t.Fatalf("want exactly one could-not-check notice, got %d: %v", len(notices), notices)
	}
	if !strings.Contains(notices[0], "could-not-check") || !strings.Contains(notices[0], "bitbucket") {
		t.Errorf("the notice must be could-not-check naming the forge; got: %s", notices[0])
	}
	for _, banned := range []string{"Rows:", "row(s) are backed", "no accepted verifier actor committed"} {
		if strings.Contains(notices[0], banned) {
			t.Errorf("an unrecognised forge must NOT be reported as backed/unbacked (found %q); got: %s", banned, notices[0])
		}
	}
}

// ---------------------------------------------------------------------------
// verifyrun — the witness names the acting forge identity (tasks 4-5)
// ---------------------------------------------------------------------------

// TestVerifyrunRunnerFromForgeIdentity (Verify row 6). With a role identity bound for
// the repo's forge, the witness Runner is that identity and the recorded source says
// so — on both forges. This is the D-9 fix: on GitLab the git identity is a service
// account with no `[bot]`, which the host-derived path would stamp as human:<name>.
func TestVerifyrunRunnerFromForgeIdentity(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITHUB_ACTOR", "")

	// GitLab: the git identity is the verifier service account; the runner resolves to
	// the bound role username with source forge-identity.
	scanWithRoster(t, gitlabRoster())
	withRemoteOrigin(t, "git@gitlab.com:example-org/tracker.git")
	glRoot := t.TempDir()
	gitInit(t, glRoot, glVerifierName, glVerifierEmail)
	runner, source, ok := executingRunner(glRoot)
	if !ok || runner != glVerifierName || source != runnerSourceForge {
		t.Fatalf("GitLab forge identity: runner=(%q, %q, %v), want (%q, %q, true)",
			runner, source, ok, glVerifierName, runnerSourceForge)
	}
	// The source is rendered on the witness row so a reader can see it.
	w := witness{ID: "1", Command: "true", State: statePass, Exit: 0, OutHash: "0123456789ab",
		Date: "2026-09-02", Runner: runner, RunnerSource: source, Tree: "abc123abc123"}
	if !strings.Contains(w.row(), "("+runnerSourceForge+")") {
		t.Errorf("the witness row must record the runner source; got: %s", w.row())
	}

	// GitHub: a bound App identity resolves to <slug>[bot] with source forge-identity.
	scanWithRoster(t, scanExampleRoster()) // reviewer + worker apps, github inferred
	withRemoteOrigin(t, "https://github.com/example-org/tracker.git")
	ghRoot := t.TempDir()
	gitInit(t, ghRoot, "assay-worker-app[bot]", "300000006+assay-worker-app[bot]@users.noreply.github.com")
	runner, source, ok = executingRunner(ghRoot)
	if !ok || runner != "assay-worker-app[bot]" || source != runnerSourceForge {
		t.Fatalf("GitHub forge identity: runner=(%q, %q, %v), want (assay-worker-app[bot], %q, true)",
			runner, source, ok, runnerSourceForge)
	}
}

// TestVerifyrunFallbackOrder (Verify row 7). With no bound forge identity the CI-env
// path is used, and with neither the git-config path — each recording its own source.
func TestVerifyrunFallbackOrder(t *testing.T) {
	scanWithRoster(t, scanExampleRoster())

	// CI-env: GitHub Actions actor, with a git identity the roster does NOT know, so the
	// forge-identity source does not fire and CI-env wins.
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_ACTOR", "octo-cat")
	withRemoteOrigin(t, "https://github.com/example-org/tracker.git")
	ciRoot := t.TempDir()
	gitInit(t, ciRoot, "Stranger", "stranger@example.com")
	if runner, source, ok := executingRunner(ciRoot); !ok || runner != "octo-cat" || source != runnerSourceCIEnv {
		t.Fatalf("CI-env fallback: runner=(%q, %q, %v), want (octo-cat, %q, true)", runner, source, ok, runnerSourceCIEnv)
	}

	// git-config: no CI env, an unknown-forge repo (no origin match), a human identity.
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITHUB_ACTOR", "")
	withRemoteOrigin(t, "git@example.invalid:example-org/tracker.git") // neither github nor gitlab
	gcRoot := t.TempDir()
	gitInit(t, gcRoot, "Alex Rivera", "alex@example.com")
	if runner, source, ok := executingRunner(gcRoot); !ok || runner != "human:alex" || source != runnerSourceGitConfig {
		t.Fatalf("git-config fallback: runner=(%q, %q, %v), want (human:alex, %q, true)", runner, source, ok, runnerSourceGitConfig)
	}
}

// TestVerifyrunStillRefusesSuppliedRunner (Verify row 8) — negative path. A runner
// supplied on the command line is still a usage error, and with no identity available
// at all the tool still refuses to write a witness. Both pre-existing properties survive
// the new forge-identity source.
func TestVerifyrunStillRefusesSuppliedRunner(t *testing.T) {
	for _, arg := range []string{"--runner=someone", "--as=x", "--identity=y", "--who=me"} {
		out, errOut := captureVerifyrun(t, []string{arg, "--brief", "x.md"})
		if out.code != verifyrunExitUsageError {
			t.Errorf("%s: exit = %d, want %d", arg, out.code, verifyrunExitUsageError)
		}
		if !strings.Contains(errOut, "derived from the executing identity") {
			t.Errorf("%s: message must explain the runner is derived, not supplied; got: %s", arg, errOut)
		}
	}

	// No derivable identity anywhere: still a refusal, never an unattributed witness.
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITHUB_ACTOR", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "absent"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(t.TempDir(), "absent"))
	withRemoteOrigin(t, "") // no origin: forge unknown, so the forge-identity source cannot fire
	root := t.TempDir()
	runGit(t, root, "init")
	if runner, _, ok := executingRunner(root); ok {
		t.Fatalf("with no identity available, executingRunner must refuse; got runner %q", runner)
	}
}

// ---------------------------------------------------------------------------
// Roster grammar parity (task 1 / Verify row 9)
// ---------------------------------------------------------------------------

// TestRosterGrammarParity (Verify row 9). The same roster fixture used by the desk-tools
// suite loads identically here, including legacy unqualified entries; and the
// forge-qualified grammar (explicit github / gitlab, and legacy unqualified) parses with
// the forge, the inferred flag, and the per-forge renderings the shared identity.md
// documents.
func TestRosterGrammarParity(t *testing.T) {
	// 1. The SHARED cross-tree vector's bot entries load identically here. They are all
	//    legacy unqualified, so this binds the backward-compatibility rule to the same
	//    fixture deskkit's coupling suite reads.
	raw, err := os.ReadFile(couplingVectorPath)
	if err != nil {
		t.Fatalf("cannot read the shared roster vector %s: %v", couplingVectorPath, err)
	}
	var vec couplingVectors
	if err := json.Unmarshal(raw, &vec); err != nil {
		t.Fatalf("%s does not parse: %v", couplingVectorPath, err)
	}
	scanWithRoster(t, vec.Roster)
	cfg := scanEffectiveConfig()
	if !cfg.Configured() {
		t.Fatalf("the shared coupling roster did not load: %v", cfg.Problems)
	}
	for _, slug := range []string{"example-reviewer-app", "example-worker-app"} {
		b, ok := cfg.BotIdents[slug]
		if !ok {
			t.Fatalf("shared vector bot %q did not parse into BotIdents", slug)
		}
		if b.Forge != forgeGitHub || !b.ForgeInferred {
			t.Errorf("shared vector bot %q: forge=%v inferred=%v, want github/inferred (legacy unqualified)", slug, b.Forge, b.ForgeInferred)
		}
		if got := b.acceptedLogins(); len(got) != 2 || got[0] != slug+"[bot]" || got[1] != "app/"+slug {
			t.Errorf("shared vector bot %q renderings = %v, want the two decorated GitHub forms", slug, got)
		}
	}

	// 2. The forge-qualified grammar: explicit github, explicit gitlab, and a legacy
	//    unqualified entry, all in one roster.
	scanWithRoster(t, map[string]string{
		scanEnvBlessLogin:    "ada:100001",
		scanEnvTrustedLogins: "ada:100001",
		scanEnvTrustedBotSlugs: "reviewer=github:example-reviewer-app:300000004," +
			"verifier=gitlab:example-verifier-bot:41987969," +
			"worker=example-worker-app:300000006",
	})
	cfg = scanEffectiveConfig()
	if !cfg.Configured() {
		t.Fatalf("the forge-qualified roster did not load: %v", cfg.Problems)
	}

	gh, ok := cfg.BotIdents["example-reviewer-app"]
	if !ok || gh.Forge != forgeGitHub || gh.ForgeInferred || gh.ID != 300000004 {
		t.Errorf("explicit github entry = %+v, want github (NOT inferred), id 300000004", gh)
	}
	gl, ok := cfg.BotIdents["example-verifier-bot"]
	if !ok || gl.Forge != forgeGitLab || gl.ForgeInferred || gl.ID != 41987969 {
		t.Errorf("explicit gitlab entry = %+v, want gitlab (NOT inferred), id 41987969", gl)
	}
	if got := gl.acceptedLogins(); len(got) != 1 || got[0] != "example-verifier-bot" {
		t.Errorf("gitlab renderings = %v, want the username only (no [bot]/app decoration)", got)
	}
	legacy, ok := cfg.BotIdents["example-worker-app"]
	if !ok || legacy.Forge != forgeGitHub || !legacy.ForgeInferred {
		t.Errorf("legacy unqualified entry = %+v, want github INFERRED", legacy)
	}
	// A gitlab username dressed in GitHub bot clothing is not an accepted login.
	if cfg.Logins["example-verifier-bot[bot]"] || cfg.Logins["app/example-verifier-bot"] {
		t.Error("a GitLab username must not be accepted under the GitHub [bot]/app renderings")
	}
	// The bare GitHub slug is never accepted, forge-qualified or not.
	if cfg.Logins["example-reviewer-app"] {
		t.Error("the bare GitHub App slug must never be an accepted login")
	}
}
