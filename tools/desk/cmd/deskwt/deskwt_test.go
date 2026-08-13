package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const ghURL = "https://github.com/example-org/tracker.git"

// --- fixtures -------------------------------------------------------------------

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// newRepo builds a scratch main checkout on `main`, one commit deep, whose origin URL
// parses to an allowed repo and whose refs/remotes/origin/{main,HEAD} are set locally
// (no network) so `origin/main` resolves for `deskwt add --base`.
func newRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	mustGit(t, "", "init", "-b", "main", work)
	mustGit(t, work, "config", "user.email", "t@e.st")
	mustGit(t, work, "config", "user.name", "Test")
	mustGit(t, work, "config", "commit.gpgsign", "false")
	mustGit(t, work, "remote", "add", "origin", ghURL)

	writeFile(t, filepath.Join(work, "README.md"), "seed\n")
	mustGit(t, work, "add", "README.md")
	mustGit(t, work, "commit", "-m", "init")
	mainSHA := mustGit(t, work, "rev-parse", "HEAD")
	mustGit(t, work, "update-ref", "refs/remotes/origin/main", mainSHA)
	mustGit(t, work, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	return work
}

// withEnv points deskkit's runtime dir at a fresh HOME, binds getwd to work, overrides
// the sanctioned tmp prefix at a fresh temp dir (so add/remove is portable & offline),
// and installs the in-process command recorder. Returns the recorded argv slice.
func withEnv(t *testing.T, work string) *[][]string {
	t.Helper()
	fixtureHome := t.TempDir()
	t.Setenv("HOME", fixtureHome)
	plantFixtureRoster(t, fixtureHome)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "test")

	oldwd := getwd
	getwd = func() (string, error) { return work, nil }
	t.Cleanup(func() { getwd = oldwd })

	oldTmp := tmpBaseDir
	tmpBaseDir = filepath.Join(t.TempDir(), "wtroot")
	if err := os.MkdirAll(tmpBaseDir, 0o755); err != nil {
		t.Fatalf("mkdir tmpBaseDir: %v", err)
	}
	t.Cleanup(func() { tmpBaseDir = oldTmp })

	calls := &[][]string{}
	oldExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, append([]string{name}, args...))
		return oldExec(name, args...)
	}
	t.Cleanup(func() { execCommand = oldExec })
	return calls
}

func runCapErr(t *testing.T, args []string) (int, string) {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	rc := run(args)
	_ = w.Close()
	os.Stderr = old
	b, _ := io.ReadAll(r)
	return rc, string(b)
}

// --- argv assertions ------------------------------------------------------------

func gitCalls(calls [][]string) [][]string {
	var out [][]string
	for _, c := range calls {
		if len(c) > 0 && filepath.Base(c[0]) == "git" {
			out = append(out, c)
		}
	}
	return out
}

func anyGitForce(calls [][]string) bool {
	for _, c := range gitCalls(calls) {
		for _, a := range c[1:] {
			if strings.HasPrefix(a, "--force") || a == "-f" {
				return true
			}
		}
	}
	return false
}

// hasWorktreeVerb reports whether any recorded git argv is `git worktree <verb> ...`.
func hasWorktreeVerb(calls [][]string, verb string) bool {
	for _, c := range gitCalls(calls) {
		for i, a := range c {
			if a == "worktree" && i+1 < len(c) && c[i+1] == verb {
				return true
			}
		}
	}
	return false
}

func resetCalls(calls *[][]string) { *calls = nil }

// assertExists fails if path was destroyed — the strongest proof that a refusal path
// touched nothing on disk.
func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to still exist (a refusal must destroy nothing); got: %v", path, err)
	}
}

// --- add: happy path + clean remove -----------------------

func TestAddThenCleanRemoveSucceeds(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)

	if rc := run([]string{"add", "happy"}); rc != deskkit.ExitOK {
		t.Fatalf("add rc = %d, want 0", rc)
	}
	target := filepath.Join(tmpBaseDir, "tracker-happy")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected worktree dir %s to exist: %v", target, err)
	}
	if !hasWorktreeVerb(*calls, "add") {
		t.Fatalf("expected a `git worktree add`; git calls: %v", gitCalls(*calls))
	}
	if anyGitForce(*calls) {
		t.Fatalf("a git argv carried a force flag on add: %v", gitCalls(*calls))
	}

	resetCalls(calls)
	if rc := run([]string{"remove", target}); rc != deskkit.ExitOK {
		t.Fatalf("remove rc = %d, want 0", rc)
	}
	// Genuinely gone: dir removed AND not in `git worktree list`.
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("worktree dir %s still exists after remove (err=%v)", target, err)
	}
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if strings.Contains(list, target) {
		t.Fatalf("worktree still registered after remove:\n%s", list)
	}
	if !hasWorktreeVerb(*calls, "prune") {
		t.Fatalf("expected a `git worktree prune`; git calls: %v", gitCalls(*calls))
	}
	if anyGitForce(*calls) {
		t.Fatalf("a git argv carried a force flag on remove: %v", gitCalls(*calls))
	}
}

// --- add refusals ---------------------------------------------------------------

func TestAddExistingTargetRefuses(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	target := filepath.Join(tmpBaseDir, "tracker-dup")
	if err := os.MkdirAll(target, 0o755); err != nil { // pre-existing target
		t.Fatalf("mkdir target: %v", err)
	}
	if rc := run([]string{"add", "dup"}); rc != deskkit.ExitRefused {
		t.Fatalf("add over existing target rc = %d, want 5", rc)
	}
	if hasWorktreeVerb(*calls, "add") {
		t.Fatalf("git worktree add ran despite an existing target: %v", gitCalls(*calls))
	}
}

func TestAddBadBaseUnverifiable(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	if rc := run([]string{"add", "bb", "--base", "origin/does-not-exist"}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("add with unresolvable --base rc = %d, want 6", rc)
	}
	if hasWorktreeVerb(*calls, "add") {
		t.Fatalf("git worktree add ran despite a bad --base: %v", gitCalls(*calls))
	}
}

func TestAddFlagAfterNameParsed(t *testing.T) {
	// Usage puts <name> before flags; verify that order works and a bad
	// --base placed AFTER the name is still caught (exit 6, not mis-parsed).
	work := newRepo(t)
	withEnv(t, work)
	if rc := run([]string{"add", "ord", "--base", "origin/nope"}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("flags-after-name bad base rc = %d, want 6", rc)
	}
}

func TestAddRepoNotAllowedRefuses(t *testing.T) {
	work := newRepo(t)
	mustGit(t, work, "remote", "set-url", "origin", "https://github.com/otheruser/otherrepo.git")
	calls := withEnv(t, work)
	if rc := run([]string{"add", "x"}); rc != deskkit.ExitRefused {
		t.Fatalf("add in repo outside the set rc = %d, want 5", rc)
	}
	if hasWorktreeVerb(*calls, "add") {
		t.Fatalf("git worktree add ran in a disallowed repo: %v", gitCalls(*calls))
	}
}

func TestAddBadNameRefuses(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	for _, bad := range []string{"../escape", "a/b", "..", "-flaggy"} {
		resetCalls(calls)
		if rc := run([]string{"add", bad}); rc != deskkit.ExitRefused {
			t.Fatalf("add name %q rc = %d, want 5", bad, rc)
		}
		if hasWorktreeVerb(*calls, "add") {
			t.Fatalf("git worktree add ran for a bad name %q: %v", bad, gitCalls(*calls))
		}
	}
}

// --- remove refusals ------------------------------------------------------------

// TestRemoveSymlinkOutsidePrefixRefuses is the meaningful path-safety test: the arg is under the
// worktrees prefix TEXTUALLY, but resolves (EvalSymlinks) to a dir OUTSIDE every prefix,
// so a resolved-path check refuses it (a naive textual prefix check would wrongly pass).
func TestRemoveSymlinkOutsidePrefixRefuses(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	wtDir := filepath.Join(work, ".claude", "worktrees")
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatalf("mkdir worktrees: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside-target")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	link := filepath.Join(wtDir, "sneaky")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	rc, errout := runCapErr(t, []string{"remove", link})
	if rc != deskkit.ExitRefused {
		t.Fatalf("symlink-outside remove rc = %d, want 5", rc)
	}
	if !strings.Contains(errout, "outside the sanctioned") {
		t.Fatalf("expected an outside-prefix refusal; got: %s", errout)
	}
	assertExists(t, outside)
	if hasWorktreeVerb(*calls, "prune") {
		t.Fatalf("a destructive prune ran on a symlink resolving outside the prefix: %v", gitCalls(*calls))
	}
}

// TestRemoveSharedCheckoutRefusedByIdentity proves the identity refusal even when the
// dirty/upstream/ahead checks would all PASS: main is clean, has an upstream, and is at
// its upstream — so ONLY the git-common-dir-parent identity check can refuse it.
func TestRemoveSharedCheckoutRefusedByIdentity(t *testing.T) {
	work := newRepo(t)
	mustGit(t, work, "branch", "--set-upstream-to=origin/main", "main")
	calls := withEnv(t, work)
	rc, errout := runCapErr(t, []string{"remove", work})
	if rc != deskkit.ExitRefused {
		t.Fatalf("remove shared checkout rc = %d, want 5", rc)
	}
	if !strings.Contains(errout, "identity") {
		t.Fatalf("expected an identity refusal; got: %s", errout)
	}
	assertExists(t, work)
	if hasWorktreeVerb(*calls, "prune") {
		t.Fatalf("a destructive prune ran on the shared checkout: %v", gitCalls(*calls))
	}
}

func TestRemoveUnregisteredRefuses(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	// A path UNDER the sanctioned tmp prefix that is not a registered worktree.
	ghost := filepath.Join(tmpBaseDir, "tracker-ghost")
	if err := os.MkdirAll(ghost, 0o755); err != nil {
		t.Fatalf("mkdir ghost: %v", err)
	}
	if rc := run([]string{"remove", ghost}); rc != deskkit.ExitRefused {
		t.Fatalf("remove unregistered path rc = %d, want 5", rc)
	}
	assertExists(t, ghost)
	if hasWorktreeVerb(*calls, "prune") {
		t.Fatalf("a destructive prune ran on an unregistered path: %v", gitCalls(*calls))
	}
}

func TestRemoveDirtyStagedTrackedRefuses(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	if rc := run([]string{"add", "dirty"}); rc != deskkit.ExitOK {
		t.Fatalf("add rc = %d, want 0", rc)
	}
	target := filepath.Join(tmpBaseDir, "tracker-dirty")
	// Create a file and stage it (a TRACKED, uncommitted change) — do not commit.
	writeFile(t, filepath.Join(target, "wip.txt"), "work in progress\n")
	mustGit(t, target, "add", "wip.txt")

	resetCalls(calls)
	if rc := run([]string{"remove", target}); rc != deskkit.ExitRefused {
		t.Fatalf("remove dirty (staged tracked) rc = %d, want 5", rc)
	}
	assertExists(t, target)
	if hasWorktreeVerb(*calls, "prune") {
		t.Fatalf("a destructive prune ran on a dirty worktree: %v", gitCalls(*calls))
	}
}

func TestRemoveModifiedTrackedRefuses(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	if rc := run([]string{"add", "mod"}); rc != deskkit.ExitOK {
		t.Fatalf("add rc = %d, want 0", rc)
	}
	target := filepath.Join(tmpBaseDir, "tracker-mod")
	// Modify an already-tracked file (unstaged) — a dirty tracked change.
	writeFile(t, filepath.Join(target, "README.md"), "seed\nmodified\n")

	resetCalls(calls)
	if rc := run([]string{"remove", target}); rc != deskkit.ExitRefused {
		t.Fatalf("remove dirty (modified tracked) rc = %d, want 5", rc)
	}
	assertExists(t, target)
	if hasWorktreeVerb(*calls, "prune") {
		t.Fatalf("a destructive prune ran on a modified-tracked worktree: %v", gitCalls(*calls))
	}
}

// TestRemoveUntrackedArtifactDoesNotBlock proves the carve-out: an untracked build
// artifact must NOT block removal (else the verb would fire on every real worktree).
func TestRemoveUntrackedArtifactDoesNotBlock(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	if rc := run([]string{"add", "arti"}); rc != deskkit.ExitOK {
		t.Fatalf("add rc = %d, want 0", rc)
	}
	target := filepath.Join(tmpBaseDir, "tracker-arti")
	writeFile(t, filepath.Join(target, "node_modules", "pkg", "index.js"), "// build artifact\n")
	writeFile(t, filepath.Join(target, "untracked.log"), "noise\n")

	if rc := run([]string{"remove", target}); rc != deskkit.ExitOK {
		t.Fatalf("remove with only untracked artifacts rc = %d, want 0 (must not block)", rc)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("worktree dir %s still exists after remove", target)
	}
}

func TestRemoveUnpushedCommitsRefuses(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	if rc := run([]string{"add", "ahead"}); rc != deskkit.ExitOK {
		t.Fatalf("add rc = %d, want 0", rc)
	}
	target := filepath.Join(tmpBaseDir, "tracker-ahead")
	// A commit ahead of the upstream (origin/main) — unpushed.
	writeFile(t, filepath.Join(target, "new.txt"), "new\n")
	mustGit(t, target, "add", "new.txt")
	mustGit(t, target, "commit", "-m", "unpushed work")

	resetCalls(calls)
	if rc := run([]string{"remove", target}); rc != deskkit.ExitRefused {
		t.Fatalf("remove with unpushed commits rc = %d, want 5", rc)
	}
	assertExists(t, target)
	if hasWorktreeVerb(*calls, "prune") {
		t.Fatalf("a destructive prune ran with unpushed commits: %v", gitCalls(*calls))
	}
}

func TestRemoveNoUpstreamRefuses(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	if rc := run([]string{"add", "noup"}); rc != deskkit.ExitOK {
		t.Fatalf("add rc = %d, want 0", rc)
	}
	target := filepath.Join(tmpBaseDir, "tracker-noup")
	// Drop the upstream the tracking-add set up → a branch with NO upstream.
	mustGit(t, target, "branch", "--unset-upstream")

	resetCalls(calls)
	if rc := run([]string{"remove", target}); rc != deskkit.ExitRefused {
		t.Fatalf("remove with no upstream rc = %d, want 5", rc)
	}
	assertExists(t, target)
	if hasWorktreeVerb(*calls, "prune") {
		t.Fatalf("a destructive prune ran on a no-upstream branch: %v", gitCalls(*calls))
	}
}

func TestRemoveNoForceFlag(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	if rc := run([]string{"add", "force"}); rc != deskkit.ExitOK {
		t.Fatalf("add rc = %d, want 0", rc)
	}
	target := filepath.Join(tmpBaseDir, "tracker-force")
	// Make it dirty so a real --force WOULD have removed it — the tool must reject the
	// flag itself, never honor it.
	writeFile(t, filepath.Join(target, "README.md"), "seed\ndirty\n")

	resetCalls(calls)
	if rc := run([]string{"remove", "--force", target}); rc != deskkit.ExitRefused {
		t.Fatalf("remove --force rc = %d, want 5 (no force flag exists)", rc)
	}
	assertExists(t, target)
	if hasWorktreeVerb(*calls, "prune") {
		t.Fatalf("a --force invocation reached the destructive path: %v", gitCalls(*calls))
	}
	if anyGitForce(*calls) {
		t.Fatalf("a force flag reached a git argv: %v", gitCalls(*calls))
	}
}

// --- kill switch (both verbs) ---------------------------------------------------

func TestKillSwitchDisablesBothVerbs(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	t.Setenv("DESK_TOOLS_DISABLED", "1")

	if rc := run([]string{"add", "x"}); rc != deskkit.ExitDisabled {
		t.Fatalf("add under kill switch rc = %d, want 3", rc)
	}
	if rc := run([]string{"remove", filepath.Join(tmpBaseDir, "tracker-x")}); rc != deskkit.ExitDisabled {
		t.Fatalf("remove under kill switch rc = %d, want 3", rc)
	}
	if _, err := os.Stat(filepath.Join(tmpBaseDir, "tracker-x")); !os.IsNotExist(err) {
		t.Fatalf("add created a worktree while the kill switch was armed")
	}
	if hasWorktreeVerb(*calls, "add") || hasWorktreeVerb(*calls, "prune") {
		t.Fatalf("a worktree op ran while the kill switch was armed: %v", gitCalls(*calls))
	}
}

// --- misc -----------------------------------------------------------------------

func TestUnknownSubcommandRefuses(t *testing.T) {
	if rc := run([]string{"frobnicate"}); rc != deskkit.ExitRefused {
		t.Fatalf("unknown subcommand rc = %d, want 5", rc)
	}
}

func TestNoArgsRefuses(t *testing.T) {
	if rc := run(nil); rc != deskkit.ExitRefused {
		t.Fatalf("no args rc = %d, want 5", rc)
	}
}

func TestVersionOK(t *testing.T) {
	if rc := run([]string{"--version"}); rc != deskkit.ExitOK {
		t.Fatalf("--version rc = %d, want 0", rc)
	}
}

func TestParseRepo(t *testing.T) {
	cases := map[string]string{
		"https://github.com/example-org/tracker.git":              "example-org/tracker",
		"git@github.com:example-org/agents.git":                   "example-org/agents",
		"ssh://git@github.com/example-org/example-reconciler.git": "example-org/example-reconciler",
	}
	for in, want := range cases {
		got, err := parseRepo(in)
		if err != nil || got != want {
			t.Errorf("parseRepo(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}
