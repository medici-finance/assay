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
		if code := runInitForge(dir, forgeGitLab); code != 0 {
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
	} {
		if !strings.Contains(gl, want) {
			t.Errorf(".gitlab-ci.yml missing %q", want)
		}
	}
	// The GitLab half must not shell `gh` — that is exactly the GitHub-only
	// dependency that makes a GitHub workflow inert on GitLab.
	if strings.Contains(gl, "gh release download") || strings.Contains(gl, "gh pr") {
		t.Errorf(".gitlab-ci.yml shells `gh`, a GitHub-only client:\n%s", gl)
	}

	// The next-steps text names the file it actually wrote, not the GitHub one.
	if !strings.Contains(next, ".gitlab-ci.yml") {
		t.Errorf("next-steps must name the scaffolded .gitlab-ci.yml; got:\n%s", next)
	}
	if strings.Contains(next, ".github/workflows/assay-statusgen.yml") {
		t.Errorf("next-steps names the GitHub workflow the gitlab scaffold did not write:\n%s", next)
	}

	// The scaffolded tree must still lint clean (the CI file is not a stream
	// input, but the scaffold as a whole must pass its own tool).
	if code := run(dir, "lint", nil, nil, ""); code != 0 {
		t.Errorf("lint of gitlab-scaffolded tree exit = %d, want 0", code)
	}
}

// TestInitDefaultsToGitHubForUnknownForge pins the fallback: an undetectable forge
// (no remote, self-hosted host naming neither forge) keeps the historical GitHub
// scaffold byte-for-byte — the established default, and what every no-remote
// `t.TempDir()` test relies on.
func TestInitDefaultsToGitHubForUnknownForge(t *testing.T) {
	dir := t.TempDir()
	if code := runInitForge(dir, forgeUnknown); code != 0 {
		t.Fatalf("runInitForge(unknown) exit = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(".github/workflows/assay-statusgen.yml"))); err != nil {
		t.Errorf("unknown forge must fall back to the GitHub workflow: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitlab-ci.yml")); !os.IsNotExist(err) {
		t.Errorf("unknown forge must not scaffold .gitlab-ci.yml (err=%v)", err)
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
