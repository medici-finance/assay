package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// stubRemoteOriginURL substitutes the origin-URL reader for one test, so forge
// detection is exercised without a git checkout — the same injection pattern
// listRemoteBranches / listMergedClosedBranches use.
func stubRemoteOriginURL(t *testing.T, url string, err error) {
	t.Helper()
	prev := remoteOriginURL
	remoteOriginURL = func(string) (string, error) { return url, err }
	t.Cleanup(func() { remoteOriginURL = prev })
}

// TestClassifyForgeURL pins the host classifier: it keys on the remote's HOST
// labels, never a substring of the whole URL, so a GitHub repo literally named
// "gitlab-migration" is not misread as GitLab. Both the SSH scp-like form and the
// URL forms resolve, and an unrecognisable host stays unknown.
func TestClassifyForgeURL(t *testing.T) {
	for _, tc := range []struct {
		url  string
		want forgeKind
	}{
		{"https://github.com/medici-finance/assay.git", forgeGitHub},
		{"git@github.com:medici-finance/assay.git", forgeGitHub},
		{"ssh://git@github.com/medici-finance/assay.git", forgeGitHub},
		{"https://gitlab.com/some-group/some-project.git", forgeGitLab},
		{"git@gitlab.com:some-group/some-project.git", forgeGitLab},
		{"ssh://git@gitlab.example.com:2222/g/p.git", forgeGitLab},
		{"https://gitlab.example.com/g/p.git", forgeGitLab},
		// The false-positive the host-label rule exists to defeat: a GitHub repo
		// whose NAME contains "gitlab" must still classify as GitHub.
		{"https://github.com/acme/gitlab-migration.git", forgeGitHub},
		{"git@github.com:acme/gitlab-migration.git", forgeGitHub},
		// Neither forge in the host → unknown (a self-hosted Gitea, say).
		{"https://git.example.com/g/p.git", forgeUnknown},
		{"git@bitbucket.org:g/p.git", forgeUnknown},
		{"", forgeUnknown},
	} {
		if got := classifyForgeURL(tc.url); got != tc.want {
			t.Errorf("classifyForgeURL(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

// TestParseForgeFlag covers the --forge flag mapping: the two forges resolve,
// empty means "not given" (detection takes over), and anything else is rejected
// rather than silently ignored.
func TestParseForgeFlag(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    forgeKind
		wantErr bool
	}{
		{"", forgeUnknown, false},
		{"github", forgeGitHub, false},
		{"GitHub", forgeGitHub, false},
		{"gitlab", forgeGitLab, false},
		{" gitlab ", forgeGitLab, false},
		{"bitbucket", forgeUnknown, true},
	} {
		got, err := parseForgeFlag(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("parseForgeFlag(%q) err = %v, wantErr %v", tc.in, err, tc.wantErr)
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("parseForgeFlag(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestInitScaffoldsGitLabCIForGitLabForge is the core #349 scaffold fix: on
// a GitLab forge, init writes a `.gitlab-ci.yml` running the same two halves and
// does NOT write the (inert) GitHub workflow — so the adopter's board has a single
// writer rather than none. The closing next-steps text names the file it actually
// wrote.
func TestInitScaffoldsGitLabCIForGitLabForge(t *testing.T) {
	dir := t.TempDir()

	next := captureStdout(t, func() {
		if code := runInitForge(dir, forgeGitLab, true, "", false); code != 0 {
			t.Fatalf("runInitForge(gitlab) exit = %d, want 0", code)
		}
	})

	// The GitLab CI half exists; the GitHub workflow does NOT (only one half is
	// scaffolded — the matching one).
	if _, err := os.Stat(filepath.Join(dir, ".gitlab-ci.yml")); err != nil {
		t.Errorf("gitlab forge did not scaffold .gitlab-ci.yml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(".github/workflows/assay-statusgen.yml"))); !os.IsNotExist(err) {
		t.Errorf("gitlab forge must NOT scaffold the inert GitHub workflow (err=%v)", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, ".gitlab-ci.yml"))
	if err != nil {
		t.Fatalf("read .gitlab-ci.yml: %v", err)
	}
	gl := string(raw)
	// Both halves are present, keyed to GitLab pipeline sources, and the
	// bootstrap-safe + no-loop + refuse-don't-guess properties are carried over.
	for _, want := range []string{
		"statusgen --lint",
		"statusgen --root .",
		`$CI_PIPELINE_SOURCE == "merge_request_event"`,
		"$CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH",
		"git status --porcelain -- STATUS.md",
		"skip-status-regen",
		"STATUSGEN_PUSH_TOKEN",
		"sha256sum -c -",
		// The push credential must be MASKED AND PROTECTED: a masked-only
		// variable is still injected into merge_request_event pipelines, which
		// run the MR branch's own CI file, so a member who can open an MR could
		// read it and push to the default branch past the merge gate. Both the
		// guidance comment and the regen job's stop message say so.
		"set it as a MASKED and PROTECTED CI/CD variable named",
		"set it as a masked and protected CI/CD variable named STATUSGEN_PUSH_TOKEN",
		"still injected into merge-request pipelines",
	} {
		if !strings.Contains(gl, want) {
			t.Errorf(".gitlab-ci.yml missing %q", want)
		}
	}
	// No line may still describe the variable as masked-only.
	for _, stale := range []string{
		"a MASKED CI/CD variable named",
		"a masked CI/CD variable named",
	} {
		if strings.Contains(gl, stale) {
			t.Errorf(".gitlab-ci.yml still describes STATUSGEN_PUSH_TOKEN as masked-only: %q", stale)
		}
	}
	// The GitLab half must not shell `gh` — that is exactly the GitHub-only
	// dependency that makes a GitHub workflow inert on GitLab.
	if strings.Contains(gl, "gh release download") || strings.Contains(gl, "gh pr") {
		t.Errorf(".gitlab-ci.yml shells `gh`, a GitHub-only client:\n%s", gl)
	}

	// #688: the scaffold must WARN about the runner precondition it cannot satisfy.
	// A GitLab pipeline fires but sits in stuck_pending_no_matching_runners when no
	// runner takes untagged jobs, and the template used to be silent on runners
	// (zero hits for "runner" / "run_untagged" / a tags: placeholder). The header
	// comment names the executor requirement, each job carries the ADOPTER: runner
	// placeholder, and neither hardcodes an instance-local tag (linux-dind was the
	// live instance's tag — must not be baked in).
	for _, want := range []string{
		"run_untagged",                      // names the exact runner attribute that must be true
		"stuck_pending_no_matching_runners", // the failure mode being warned about
		"ADOPTER: runner",                   // the commented tags: placeholder, GitHub-house shape
		"# tags: [REPLACE_WITH_YOUR_RUNNER_TAG]",
	} {
		if !strings.Contains(gl, want) {
			t.Errorf(".gitlab-ci.yml missing runner guidance %q (#688)", want)
		}
	}
	if strings.Contains(gl, "linux-dind") {
		t.Errorf(".gitlab-ci.yml hardcodes the instance-local tag linux-dind (#688) — the required tag set is instance-local and must not be baked in:\n%s", gl)
	}

	// The next-steps text names the file it actually wrote, not the GitHub one.
	if !strings.Contains(next, ".gitlab-ci.yml") {
		t.Errorf("next-steps must name the scaffolded .gitlab-ci.yml; got:\n%s", next)
	}
	if strings.Contains(next, ".github/workflows/assay-statusgen.yml") {
		t.Errorf("next-steps names the GitHub workflow the gitlab scaffold did not write:\n%s", next)
	}

	// #688: on GitLab the next-steps must add the runner Verify — a job must LEAVE
	// pending before CI is "installed"; a stuck-pending job is could-not-check.
	for _, want := range []string{"runner", "run_untagged", "pending"} {
		if !strings.Contains(next, want) {
			t.Errorf("gitlab next-steps missing runner note %q (#688); got:\n%s", want, next)
		}
	}
}

// TestInitNextStepsRunnerNoteIsGitLabOnly pins the forge boundary of the #688
// runner note: the GitHub half runs on hosted ubuntu-latest and must NOT inherit
// the GitLab runner warning (it would be false guidance there). Asserted directly
// so driving the GitLab assertions green cannot be done by leaking the note into
// the shared next-steps every GitHub adopter also sees.
func TestInitNextStepsRunnerNoteIsGitLabOnly(t *testing.T) {
	dir := t.TempDir()
	next := captureStdout(t, func() {
		if code := runInitForge(dir, forgeGitHub, true, "", false); code != 0 {
			t.Fatalf("runInitForge(github) exit = %d, want 0", code)
		}
	})
	if strings.Contains(next, "stuck_pending_no_matching_runners") || strings.Contains(next, "run_untagged") {
		t.Errorf("github next-steps leaked the GitLab-only runner note (#688); got:\n%s", next)
	}

	// The scaffolded tree must still lint clean (the CI file is not a stream
	// input, but the scaffold as a whole must pass its own tool).
	if code := run(dir, "lint", nil, nil, ""); code != 0 {
		t.Errorf("lint of gitlab-scaffolded tree exit = %d, want 0", code)
	}
}

// TestInitDefaultsToGitHubForNoRemote pins the ONE surviving GitHub fallback: a
// tree with NO readable origin remote (remotePresent=false — a fresh `t.TempDir()`,
// a repo whose origin is not yet set) keeps the historical GitHub scaffold
// byte-for-byte, which is what every no-remote test relies on. This is deliberately
// NARROWER than the pre-#349-era fallback: a readable remote whose host names
// neither forge no longer defaults to GitHub — it writes no CI half (forge-neutral
// row 4, TestInitUnresolvedForgeWritesNoCIHalf), the shape forge-neutral/01 forbids
// defaulting to GitHub.
func TestInitDefaultsToGitHubForNoRemote(t *testing.T) {
	dir := t.TempDir()
	if code := runInitForge(dir, forgeUnknown, false /* no remote */, "", false); code != 0 {
		t.Fatalf("runInitForge(unknown, no remote) exit = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(".github/workflows/assay-statusgen.yml"))); err != nil {
		t.Errorf("a no-remote tree must fall back to the GitHub workflow: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitlab-ci.yml")); !os.IsNotExist(err) {
		t.Errorf("a no-remote tree must not scaffold .gitlab-ci.yml (err=%v)", err)
	}
}

// TestDecayNotApplicableOnGitLabForge is the #349 honesty fix: on a GitLab
// remote the dead-claim decay does not shell `gh` at all — it returns the branch
// set unchanged and prints a DISTINCT "not applicable on this forge" NOTICE, never
// the "unavailable this run" message that reads as a transient could-not-check.
func TestDecayNotApplicableOnGitLabForge(t *testing.T) {
	stubRemoteOriginURL(t, "git@gitlab.com:group/project.git", nil)
	// If the gh lister is reached at all on a GitLab remote, that is the bug —
	// fail loudly. It would also (wrongly) decay a branch, which we assert against.
	called := false
	prev := listMergedClosedBranches
	listMergedClosedBranches = func(string) (map[string]bool, error) {
		called = true
		return map[string]bool{"fix/issue-loop-02-merged": true}, nil
	}
	t.Cleanup(func() { listMergedClosedBranches = prev })

	branches := []string{"main", "fix/issue-loop-02-merged"}
	var got []string
	stderr := captureStderr(t, func() { got = decayDeadClaims("/repo", branches) })

	if called {
		t.Error("decay shelled the GitHub-only lister on a GitLab remote")
	}
	if !reflect.DeepEqual(got, branches) {
		t.Fatalf("gitlab decay altered the branch set: got %v, want unchanged %v", got, branches)
	}
	if !strings.Contains(stderr, "NOT APPLICABLE") {
		t.Errorf("gitlab decay must say NOT APPLICABLE; got:\n%s", stderr)
	}
	// The transient could-not-check wording ("dead-claim decay unavailable — …")
	// must NOT be what a GitLab adopter sees: this is a permanent not-applicable,
	// not a "gh was not authed this run".
	if strings.Contains(stderr, "decay unavailable") {
		t.Errorf("gitlab decay must NOT reuse the transient \"decay unavailable\" wording; got:\n%s", stderr)
	}
}

// TestDecayStillRunsOnGitHubAndUnknown proves the forge gate did not disable the
// decay everywhere: on a GitHub remote it still drops merged/closed corpses, and
// on an UNKNOWN remote (could not tell — never rounded to "not GitHub") it still
// ATTEMPTS the gh read and degrades loudly on failure, exactly as before.
func TestDecayStillRunsOnGitHubAndUnknown(t *testing.T) {
	branches := []string{"main", "fix/issue-loop-02-merged"}

	// GitHub remote: decay runs, corpse dropped.
	stubRemoteOriginURL(t, "https://github.com/acme/repo.git", nil)
	stubMergedClosedBranches(t, map[string]bool{"fix/issue-loop-02-merged": true}, nil)
	if got := decayDeadClaims("/repo", branches); !reflect.DeepEqual(got, []string{"main"}) {
		t.Errorf("github decay did not drop the corpse: got %v", got)
	}

	// Unknown remote (self-hosted, neither forge): still attempts gh; on failure
	// degrades to the full set with the transient "unavailable" wording — the
	// correct answer when we cannot confirm the forge is not GitHub.
	stubRemoteOriginURL(t, "https://git.example.com/g/p.git", nil)
	stubMergedClosedBranches(t, nil, errors.New("gh: not authenticated"))
	var got []string
	stderr := captureStderr(t, func() { got = decayDeadClaims("/repo", branches) })
	if !reflect.DeepEqual(got, branches) {
		t.Errorf("unknown-forge decay must fall back to the full set: got %v", got)
	}
	if !strings.Contains(stderr, "unavailable") {
		t.Errorf("unknown-forge decay must keep the transient \"unavailable\" wording; got:\n%s", stderr)
	}
}

// TestInitGitLabCIMaterialisesNonSecretRoster pins #1110: the scaffolded GitLab
// regen job used to run statusgen with no roster at all — on a GitLab runner
// there is no GITHUB_ACTIONS, so statusgen is in its file-only class and read
// `$HOME/.config/assay/roster.env`, which no step ever wrote. The Evidence-actor
// check then reported could-not-check on every row of every board regen while
// the job stayed green, so the gap was silent. The template now materialises the
// NON-secret half of the roster from a CI/CD variable (STATUSGEN_ROSTER_ENV)
// into that path with owner-only permissions before statusgen runs, in BOTH
// halves (--lint on merge requests, regen on the default branch), and prints a
// clear NOTICE naming the variable when it is absent — loud, never silent.
func TestInitGitLabCIMaterialisesNonSecretRoster(t *testing.T) {
	gl := initGitlabCI
	for _, want := range []string{
		// The variable, the path it lands at, and the permissions the loader enforces.
		"STATUSGEN_ROSTER_ENV",
		`"${HOME}/.config/assay/roster.env"`,
		`install -d -m 700 "${HOME}/.config/assay"`,
		`chmod 600 "${HOME}/.config/assay/roster.env"`,
		// A File-type CI/CD variable arrives as a PATH; a Variable-type one as the
		// contents. Both are accepted, so the adopter's choice of type cannot
		// silently produce a one-line roster holding a temp-file path.
		`if [ -f "${STATUSGEN_ROSTER_ENV}" ]; then`,
		// The loud half: an absent variable prints a NOTICE that names the
		// variable AND says what statusgen will report without it.
		"NOTICE: STATUSGEN_ROSTER_ENV is not set",
		"could-not-check",
		// The roster is the NON-secret half only; a secret-shaped key refuses.
		"never a token, key, or password",
		// It is a shared anchor so both halves get the same roster.
		".statusgen-roster: &statusgen-roster",
	} {
		if !strings.Contains(gl, want) {
			t.Errorf(".gitlab-ci.yml missing %q (#1110)", want)
		}
	}
	// Both jobs must apply the anchor BEFORE their statusgen invocation.
	if n := strings.Count(gl, "- *statusgen-roster"); n != 2 {
		t.Errorf(".gitlab-ci.yml applies *statusgen-roster %d time(s), want 2 (lint + regen) (#1110)", n)
	}
	lintJobAt, regenJobAt := strings.Index(gl, "statusgen-lint:"), strings.Index(gl, "statusgen-regen:")
	if lintJobAt < 0 || regenJobAt < lintJobAt {
		t.Fatalf("expected statusgen-lint: ahead of statusgen-regen: (lint at %d, regen at %d)", lintJobAt, regenJobAt)
	}
	regen := gl[regenJobAt:]
	rosterAt := strings.Index(regen, "- *statusgen-roster")
	runAt := strings.Index(regen, "- statusgen --root .")
	if rosterAt < 0 || runAt < 0 || rosterAt > runAt {
		t.Errorf("regen job must materialise the roster before `statusgen --root .` (roster at %d, run at %d) (#1110)", rosterAt, runAt)
	}
	lint := gl[lintJobAt:regenJobAt]
	rosterAt = strings.Index(lint, "- *statusgen-roster")
	runAt = strings.Index(lint, "- statusgen --lint")
	if rosterAt < 0 || runAt < 0 || rosterAt > runAt {
		t.Errorf("lint job must materialise the roster before `statusgen --lint` (roster at %d, run at %d) (#1110)", rosterAt, runAt)
	}
	// The push credential must never be folded into the roster: the roster block
	// must not reference STATUSGEN_PUSH_TOKEN, and the header must say the roster
	// variable is not the place for a token.
	anchorAt := strings.Index(gl, ".statusgen-roster: &statusgen-roster")
	lintAt := strings.Index(gl, "statusgen-lint:")
	if anchorAt < 0 || lintAt < anchorAt {
		t.Fatalf("no .statusgen-roster anchor ahead of the lint job (anchor at %d, lint at %d) (#1110)", anchorAt, lintAt)
	}
	anchor := gl[anchorAt:lintAt]
	if strings.Contains(anchor, "STATUSGEN_PUSH_TOKEN") {
		t.Errorf("the roster anchor references STATUSGEN_PUSH_TOKEN — the roster is the NON-secret half and must never carry the push credential (#1110)")
	}
}
