package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fakeGHSource is compiled once (TestMain) into a temp dir placed FIRST on PATH, so the
// tool's `gh` and `desktoken` invocations hit this canned stand-in instead of the
// network. `pr list` returns [] (or one PR when FAKEGH_LIST_HAS_PR=1); `pr create` prints
// a PR URL; `desktoken worker` creates a fake token file and prints its path. It also
// appends its argv to FAKEGH_LOG when set (the literal PATH-shim recorder); the in-process
// execCommand recorder is the authoritative one used by assertions.
//
// The `worker` and `list`/`create` cases model the REAL cross-installation failure mode
// from #562/#565, not just a canned success: `desktoken worker` resolves an "owner"
// from `--repo <slug>` — defaulting to "example-org" when the flag is ABSENT, exactly like
// the production tool — and mints a token whose value encodes that owner
// ("fake-worker-installation-token-for-<owner>"). `gh pr list`/`pr create` then check
// that GH_TOKEN's embedded owner matches the owner of the `-R <repo>` target: a mismatch
// exits 1 with the same "Could not resolve to a Repository" wording GitHub's real API
// returns for an installation token scoped to the wrong org. Without deskpr forwarding
// --repo, this fake would reproduce the bug and the regression tests below would fail
// exactly as the real tool did.
const fakeGHSource = `package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	args := os.Args[1:]
	if log := os.Getenv("FAKEGH_LOG"); log != "" {
		if f, err := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			fmt.Fprintln(f, strings.Join(args, "\x00"))
			f.Close()
		}
	}
	has := func(s string) bool {
		for _, a := range args {
			if a == s {
				return true
			}
		}
		return false
	}
	val := func(flag string) string {
		for i, a := range args {
			if a == flag && i+1 < len(args) {
				return args[i+1]
			}
		}
		return ""
	}
	owner := func(repoSlug string) string {
		if i := strings.IndexByte(repoSlug, '/'); i >= 0 {
			return repoSlug[:i]
		}
		return repoSlug
	}
	repoOf := func(repoSlug string) string {
		if repoSlug != "" {
			return repoSlug
		}
		return "example-org/tracker"
	}
	switch {
	case has("worker"):
		// desktoken worker [--repo <slug>]: mirror the REAL desktoken default — owner is
		// parsed from --repo when present, else defaults to "example-org" (the bug #562/
		// #565 fixed was deskreply/deskpr never passing --repo, so this always resolved
		// "example-org").
		o := "example-org"
		if repo := val("--repo"); repo != "" {
			o = owner(repo)
		}
		f, err := os.CreateTemp("", "fake-worker-token-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "create temp token: %v\n", err)
			os.Exit(1)
		}
		if _, werr := f.WriteString("fake-worker-installation-token-for-" + o + "\n"); werr != nil {
			f.Close()
			os.Remove(f.Name())
			os.Exit(1)
		}
		f.Close()
		if cerr := os.Chmod(f.Name(), 0o600); cerr != nil {
			os.Remove(f.Name())
			os.Exit(1)
		}
		abs, _ := filepath.Abs(f.Name())
		fmt.Println(abs)
	case has("list"):
		target := val("-R")
		tokenOwner := strings.TrimPrefix(os.Getenv("GH_TOKEN"), "fake-worker-installation-token-for-")
		if target != "" && tokenOwner != "" && owner(target) != tokenOwner {
			fmt.Fprintf(os.Stderr, "GraphQL: Could not resolve to a Repository with the name '%s'. (repository)\n", target)
			os.Exit(1)
		}
		if os.Getenv("FAKEGH_LIST_HAS_PR") == "1" {
			draft := "true"
			if os.Getenv("FAKEGH_LIST_DRAFT") == "0" {
				draft = "false"
			}
			// The PR's headRefName echoes the --head the tool asked for, so matchHead
			// matches by construction. FAKEGH_LIST_HEAD overrides it with a divergent
			// branch name so the wrong-branch (head-owns-branch) guard can be exercised.
			head := val("--head")
			if o := os.Getenv("FAKEGH_LIST_HEAD"); o != "" {
				head = o
			}
			fmt.Printf("[{\"number\":42,\"url\":\"https://github.com/%s/pull/42\",\"isDraft\":%s,\"headRefName\":%q}]\n", repoOf(target), draft, head)
		} else {
			fmt.Println("[]")
		}
	case has("create"):
		target := val("-R")
		tokenOwner := strings.TrimPrefix(os.Getenv("GH_TOKEN"), "fake-worker-installation-token-for-")
		if target != "" && tokenOwner != "" && owner(target) != tokenOwner {
			fmt.Fprintf(os.Stderr, "GraphQL: Could not resolve to a Repository with the name '%s'. (repository)\n", target)
			os.Exit(1)
		}
		fmt.Printf("https://github.com/%s/pull/101\n", repoOf(target))
	case has("view"):
		// example-stream/02: gh pr view --json body serves the PR body for the update
		// trailer check. FAKEGH_PR_BODY overrides; the default carries a resolving trailer.
		if has("--json") && has("body") {
			b := os.Getenv("FAKEGH_PR_BODY")
			if b == "" {
				b = "Brief: fixture/01\n"
			}
			fmt.Printf("{\"body\":%q}\n", b)
			return
		}
		mergeable := os.Getenv("FAKEGH_MERGEABLE")
		if mergeable == "" {
			mergeable = "MERGEABLE"
		}
		mss := os.Getenv("FAKEGH_MERGESTATE")
		if mss == "" {
			mss = "CLEAN"
		}
		// Model GitHub's ASYNC mergeable computation (#1264): while a counter file sits
		// at or below FAKEGH_UNKNOWN_UNTIL, report UNKNOWN (state UNKNOWN too) and bump
		// the counter, so a polling reader sees the field settle only after that many
		// views. FAKEGH_UNKNOWN_UNTIL with no bound (or huge) models a value that never
		// settles.
		if cf := os.Getenv("FAKEGH_VIEW_COUNT_FILE"); cf != "" {
			until := 0
			fmt.Sscanf(os.Getenv("FAKEGH_UNKNOWN_UNTIL"), "%d", &until)
			n := 0
			if b, rerr := os.ReadFile(cf); rerr == nil {
				fmt.Sscanf(string(b), "%d", &n)
			}
			n++
			os.WriteFile(cf, []byte(fmt.Sprintf("%d", n)), 0o600)
			if n <= until {
				mergeable = "UNKNOWN"
				mss = "UNKNOWN"
			}
		}
		fmt.Printf("{\"mergeable\":%q,\"mergeStateStatus\":%q}\n", mergeable, mss)
	}
}
`

var (
	fakeGHDir string
	origPATH  string
)

func TestMain(m *testing.M) {
	rosterCleanup, rerr := installFixtureRoster()
	if rerr != nil {
		panic("cannot install the test-fixture roster: " + rerr.Error())
	}
	defer rosterCleanup()
	origPATH = os.Getenv("PATH")
	dir, err := os.MkdirTemp("", "deskpr-fakegh")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if werr := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fakegh\n\ngo 1.25\n"), 0o644); werr != nil {
		fmt.Fprintln(os.Stderr, werr)
		os.Exit(1)
	}
	if werr := os.WriteFile(filepath.Join(dir, "main.go"), []byte(fakeGHSource), 0o644); werr != nil {
		fmt.Fprintln(os.Stderr, werr)
		os.Exit(1)
	}
	build := exec.Command("go", "build", "-o", filepath.Join(dir, "gh"), ".")
	build.Dir = dir
	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	if out, berr := build.CombinedOutput(); berr != nil {
		fmt.Fprintf(os.Stderr, "build fake gh: %v\n%s\n", berr, out)
		os.Exit(1)
	}
	// Also build as desktoken (same fake binary, different name — the tool shells
	// out to `desktoken worker` when --as-app is set; the fake handles the "worker"
	// argv check).
	buildDT := exec.Command("go", "build", "-o", filepath.Join(dir, "desktoken"), ".")
	buildDT.Dir = dir
	buildDT.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	if out, berr := buildDT.CombinedOutput(); berr != nil {
		fmt.Fprintf(os.Stderr, "build fake desktoken: %v\n%s\n", berr, out)
		os.Exit(1)
	}
	fakeGHDir = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

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
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

const ghURL = "https://github.com/example-org/tracker.git"

// newBaseFixture builds a scratch worktree on feature/test-branch, 1 commit ahead of
// the default branch, whose origin remote parses to an allowed repo but whose PUSHes go
// to a local bare repo (offline). refs/remotes/origin/{main,HEAD} are set explicitly so
// preflight can resolve the default branch without a network fetch.
func newBaseFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	bare := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	bareURL := "file://" + bare

	mustGit(t, "", "init", "--bare", "-b", "main", bare)
	mustGit(t, "", "init", "-b", "main", work)
	mustGit(t, work, "config", "user.email", "t@e.st")
	mustGit(t, work, "config", "user.name", "Test")
	mustGit(t, work, "config", "commit.gpgsign", "false")
	mustGit(t, work, "remote", "add", "origin", ghURL)
	// origin.url parses to the allowed repo; pushes route to the local bare.
	mustGit(t, work, "remote", "set-url", "--push", "origin", bareURL)

	writeFile(t, filepath.Join(work, "README.md"), "seed\n")
	mustGit(t, work, "add", "README.md")
	mustGit(t, work, "commit", "-m", "init")

	// example-stream/02: the fixture carries a real brief so `Brief: fixture/01`
	// trailers in test bodies resolve under --root (docs/streams). Committed BEFORE the
	// origin/main refs are set, so the fixture brief is part of origin/main itself and
	// the zero-commits-ahead precondition still measures zero.
	if merr := os.MkdirAll(filepath.Join(work, "docs", "streams", "fixture"), 0o755); merr != nil {
		t.Fatalf("mkdir fixture streams: %v", merr)
	}
	writeFile(t, filepath.Join(work, "docs", "streams", "fixture", "brief-01-test.md"),
		"---\nschema: brief-v1\nbrief: fixture/01\ntitle: fixture brief\n---\n\nFixture brief for deskpr tests.\n")
	mustGit(t, work, "add", "docs")
	mustGit(t, work, "commit", "-m", "fixture brief")
	mainSHA := mustGit(t, work, "rev-parse", "HEAD")

	mustGit(t, work, "update-ref", "refs/remotes/origin/main", mainSHA)
	mustGit(t, work, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	mustGit(t, work, "checkout", "-b", "feature/test-branch")
	writeFile(t, filepath.Join(work, "feature.txt"), "work\n")
	mustGit(t, work, "add", "feature.txt")
	mustGit(t, work, "commit", "-m", "feature work")
	return work
}

const medforURL = "https://github.com/medici-finance/assay.git"

// newMediciFixture is newBaseFixture's twin, but its origin resolves to
// medici-finance/assay — an org OTHER than desktoken's hardcoded "example-org"
// default owner. It exists to reproduce #565 (mirroring #562/#563 for deskreply): the
// worker App is installed on BOTH example-org and medici-finance, so a worktree whose
// origin is under medici-finance is exactly the case where forgetting to forward --repo
// to `desktoken worker` silently mints a token for the wrong installation instead of
// failing loudly.
func newMediciFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	bare := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	bareURL := "file://" + bare

	mustGit(t, "", "init", "--bare", "-b", "main", bare)
	mustGit(t, "", "init", "-b", "main", work)
	mustGit(t, work, "config", "user.email", "t@e.st")
	mustGit(t, work, "config", "user.name", "Test")
	mustGit(t, work, "config", "commit.gpgsign", "false")
	mustGit(t, work, "remote", "add", "origin", medforURL)
	mustGit(t, work, "remote", "set-url", "--push", "origin", bareURL)

	writeFile(t, filepath.Join(work, "README.md"), "seed\n")
	mustGit(t, work, "add", "README.md")
	mustGit(t, work, "commit", "-m", "init")

	// example-stream/02: the fixture carries a real brief so `Brief: fixture/01`
	// trailers in test bodies resolve under --root (docs/streams). Committed BEFORE the
	// origin/main refs are set, so the fixture brief is part of origin/main itself and
	// the zero-commits-ahead precondition still measures zero.
	if merr := os.MkdirAll(filepath.Join(work, "docs", "streams", "fixture"), 0o755); merr != nil {
		t.Fatalf("mkdir fixture streams: %v", merr)
	}
	writeFile(t, filepath.Join(work, "docs", "streams", "fixture", "brief-01-test.md"),
		"---\nschema: brief-v1\nbrief: fixture/01\ntitle: fixture brief\n---\n\nFixture brief for deskpr tests.\n")
	mustGit(t, work, "add", "docs")
	mustGit(t, work, "commit", "-m", "fixture brief")
	mainSHA := mustGit(t, work, "rev-parse", "HEAD")

	mustGit(t, work, "update-ref", "refs/remotes/origin/main", mainSHA)
	mustGit(t, work, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	mustGit(t, work, "checkout", "-b", "feature/test-branch")
	writeFile(t, filepath.Join(work, "feature.txt"), "work\n")
	mustGit(t, work, "add", "feature.txt")
	mustGit(t, work, "commit", "-m", "feature work")
	return work
}

// withEnv points deskkit's runtime dir at a fresh HOME, prepends the fake gh to PATH,
// binds getwd to work, and installs the in-process command recorder. Returns the
// recorded argv slice.
func withEnv(t *testing.T, work string) *[][]string {
	t.Helper()
	fixtureHome := t.TempDir()
	t.Setenv("HOME", fixtureHome)
	plantFixtureRoster(t, fixtureHome)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "test")
	t.Setenv("PATH", fakeGHDir+string(os.PathListSeparator)+origPATH)

	oldwd := getwd
	getwd = func() (string, error) { return work, nil }
	t.Cleanup(func() { getwd = oldwd })

	calls := &[][]string{}
	oldExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, append([]string{name}, args...))
		return oldExec(name, args...)
	}
	t.Cleanup(func() { execCommand = oldExec })

	oldGate := publicRepoGateFn
	publicRepoGateFn = func(_ deskkit.RepoInfoFetcher, owner, repo string, issueNumber int) error { return nil }
	t.Cleanup(func() { publicRepoGateFn = oldGate })

	return calls
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

func ghCalls(calls [][]string) [][]string {
	var out [][]string
	for _, c := range calls {
		if len(c) > 0 && filepath.Base(c[0]) == "gh" {
			out = append(out, c)
		}
	}
	return out
}

// anyGitForce reports whether ANY recorded git argv carries a force flag. The
// draft-only-by-construction guarantee requires this to be false on every path.
func anyGitForce(calls [][]string) bool {
	for _, c := range gitCalls(calls) {
		for _, a := range c[1:] {
			if strings.HasPrefix(a, "--force") { // --force and --force-with-lease
				return true
			}
		}
	}
	return false
}

func callContainsAll(c []string, want ...string) bool {
	for _, w := range want {
		found := false
		for _, a := range c {
			if a == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func anyCall(calls [][]string, want ...string) bool {
	for _, c := range calls {
		if callContainsAll(c, want...) {
			return true
		}
	}
	return false
}

// withStderrCapture swaps deskprStderr for a buffer for the duration of the test and
// restores it on cleanup — the seam warnIfConflicting (#770) writes its WARNING through.
func withStderrCapture(t *testing.T) *strings.Builder {
	t.Helper()
	var buf strings.Builder
	old := deskprStderr
	deskprStderr = &buf
	t.Cleanup(func() { deskprStderr = old })
	return &buf
}

// --- tests ----------------------------------------------------------------------

func TestCreateSuccessAlwaysDraftNeverForce(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	stderr := withStderrCapture(t)

	rc := run([]string{"create", "--title", "add feature", "--body-min", "does the thing\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create rc = %d, want 0", rc)
	}
	// git push happened, argv constructed literally, NEVER --force (core assertion).
	if !anyCall(gitCalls(*calls), "push", "-u", "origin", "feature/test-branch") {
		t.Fatalf("expected a plain `git push -u origin feature/test-branch`; git calls: %v", gitCalls(*calls))
	}
	if anyGitForce(*calls) {
		t.Fatalf("a git argv carried --force; draft-only-by-construction requires none: %v", gitCalls(*calls))
	}
	// gh pr create --draft is ALWAYS present (the always-draft argv assertion).
	if !anyCall(ghCalls(*calls), "pr", "create", "--draft") {
		t.Fatalf("expected `gh pr create --draft`; gh calls: %v", ghCalls(*calls))
	}
	// The post-create mergeable probe (#770) ran ...
	if !anyCall(ghCalls(*calls), "pr", "view", "101") {
		t.Fatalf("expected a post-create `gh pr view 101 --json mergeable,...`; gh calls: %v", ghCalls(*calls))
	}
	// ... and a clean/MERGEABLE PR (the fake's default) never warns.
	if got := stderr.String(); strings.Contains(got, "CONFLICTING") {
		t.Fatalf("clean PR should not warn CONFLICTING; stderr: %q", got)
	}
}

// TestCreateConflictingPRWarnsLoudly is the direct regression test for #770: a
// newly-created PR that GitHub reports mergeable=CONFLICTING must print a loud WARNING
// (not read identically to "checks pending") and the audit detail line must record it.
func TestCreateConflictingPRWarnsLoudly(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	stderr := withStderrCapture(t)
	t.Setenv("FAKEGH_MERGEABLE", "CONFLICTING")
	t.Setenv("FAKEGH_MERGESTATE", "DIRTY")

	rc := run([]string{"create", "--title", "add feature", "--body-min", "does the thing\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create rc = %d, want 0 — a CONFLICTING mergeable state is advisory, never a failure", rc)
	}
	if !anyCall(ghCalls(*calls), "pr", "view", "101") {
		t.Fatalf("expected a post-create `gh pr view 101 --json mergeable,...`; gh calls: %v", ghCalls(*calls))
	}
	got := stderr.String()
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "CONFLICTING") {
		t.Fatalf("expected a loud CONFLICTING WARNING on stderr, got: %q", got)
	}
	if !strings.Contains(got, "DIRTY") {
		t.Fatalf("expected the WARNING to name mergeStateStatus=DIRTY, got: %q", got)
	}
}

// noPollSleep swaps pollSleep for a no-op for the duration of a test so the polling loop
// (#1264) runs without real delay, and restores it on cleanup.
func noPollSleep(t *testing.T) {
	t.Helper()
	old := pollSleep
	pollSleep = func() {}
	t.Cleanup(func() { pollSleep = old })
}

// countCalls returns how many recorded gh calls contain all of `want`.
func countCalls(calls [][]string, want ...string) int {
	n := 0
	for _, c := range calls {
		if callContainsAll(c, want...) {
			n++
		}
	}
	return n
}

// TestUpdateConflictingPRWarnsLoudly is the update-path twin of the create regression
// (#1264): a push that lands the PR in mergeable=CONFLICTING must warn loudly too, since a
// conflicted head gets zero pull_request runs exactly as a conflicted create does. The
// fake reports the open PR as number 42.
func TestUpdateConflictingPRWarnsLoudly(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	stderr := withStderrCapture(t)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1") // open draft PR (number 42) on the branch
	t.Setenv("FAKEGH_MERGEABLE", "CONFLICTING")
	t.Setenv("FAKEGH_MERGESTATE", "DIRTY")

	rc := run([]string{"update"})
	if rc != deskkit.ExitOK {
		t.Fatalf("update rc = %d, want 0 — a CONFLICTING mergeable state is advisory, never a failure", rc)
	}
	// The push happened AND the post-update mergeable probe ran on the listed PR (#42).
	if !anyCall(gitCalls(*calls), "push", "-u", "origin", "feature/test-branch") {
		t.Fatalf("update did not push the branch: %v", gitCalls(*calls))
	}
	if !anyCall(ghCalls(*calls), "pr", "view", "42") {
		t.Fatalf("expected a post-update `gh pr view 42 --json mergeable,...`; gh calls: %v", ghCalls(*calls))
	}
	got := stderr.String()
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "CONFLICTING") {
		t.Fatalf("expected a loud CONFLICTING WARNING on stderr after update, got: %q", got)
	}
}

// TestWarnPollsUntilMergeableSettles pins the async-settle behaviour (#1264): GitHub
// returns UNKNOWN for the first reads while it computes the test-merge, then reveals
// CONFLICTING. warnIfConflicting must poll past the UNKNOWNs and still warn — a single
// read would have swallowed the conflict as a transient UNKNOWN.
func TestWarnPollsUntilMergeableSettles(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	stderr := withStderrCapture(t)
	noPollSleep(t)
	countFile := filepath.Join(t.TempDir(), "views")
	t.Setenv("FAKEGH_VIEW_COUNT_FILE", countFile)
	t.Setenv("FAKEGH_UNKNOWN_UNTIL", "2") // first two views UNKNOWN, third settles
	t.Setenv("FAKEGH_MERGEABLE", "CONFLICTING")
	t.Setenv("FAKEGH_MERGESTATE", "DIRTY")

	rc := run([]string{"create", "--title", "add feature", "--body-min", "does the thing\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create rc = %d, want 0", rc)
	}
	if n := countCalls(ghCalls(*calls), "pr", "view", "101"); n < 3 {
		t.Fatalf("expected the mergeable probe to poll past the UNKNOWNs (>=3 views), got %d: %v", n, ghCalls(*calls))
	}
	if got := stderr.String(); !strings.Contains(got, "WARNING") || !strings.Contains(got, "CONFLICTING") {
		t.Fatalf("expected the settled CONFLICTING to warn after polling, got: %q", got)
	}
}

// TestWarnUnknownNeverSettles pins the give-up path (#1264): a mergeable field that stays
// UNKNOWN for the whole poll window must NOT be reported as CONFLICTING (no false alarm),
// and must leave a soft advisory note naming UNKNOWN — never a returned failure.
func TestWarnUnknownNeverSettles(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	stderr := withStderrCapture(t)
	noPollSleep(t)
	countFile := filepath.Join(t.TempDir(), "views")
	t.Setenv("FAKEGH_VIEW_COUNT_FILE", countFile)
	t.Setenv("FAKEGH_UNKNOWN_UNTIL", "9999") // never settles within the poll window
	t.Setenv("FAKEGH_MERGEABLE", "CONFLICTING")

	rc := run([]string{"create", "--title", "add feature", "--body-min", "does the thing\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create rc = %d, want 0 — an unsettled UNKNOWN is advisory, never a failure", rc)
	}
	if n := countCalls(ghCalls(*calls), "pr", "view", "101"); n != pollAttempts {
		t.Fatalf("expected exactly pollAttempts (%d) polls before giving up, got %d", pollAttempts, n)
	}
	got := stderr.String()
	if strings.Contains(got, "CONFLICTING") {
		t.Fatalf("an unsettled UNKNOWN must not be reported as CONFLICTING; stderr: %q", got)
	}
	if !strings.Contains(got, "UNKNOWN") || !strings.Contains(got, "did not settle") {
		t.Fatalf("expected a soft UNKNOWN-did-not-settle note, got: %q", got)
	}
}

func TestCreateAsAppMintsWorkerToken(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)

	rc := run([]string{"create", "--as-app", "--title", "worker app PR", "--body-min", "posted as worker app\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create --as-app rc = %d, want 0", rc)
	}
	// desktoken worker was called to mint the worker App token.
	found := false
	for _, c := range *calls {
		if len(c) >= 2 && filepath.Base(c[0]) == "desktoken" && c[1] == "worker" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected `desktoken worker` call when --as-app is set; calls: %v", *calls)
	}
	// The gh calls happened with GH_TOKEN set in the environment.
	if !anyCall(ghCalls(*calls), "pr", "create", "--draft") {
		t.Fatalf("expected `gh pr create --draft`; gh calls: %v", ghCalls(*calls))
	}
}

// TestCreateMintsWorkerTokenScopedToOwnRepo is the direct regression test for #565
// (mirroring #563/#562 for deskreply): deskpr must pass `--repo <this worktree's own
// repo>` to `desktoken worker`, not mint blind. Before the fix, mintWorkerToken called
// `desktoken worker` with no --repo at all, so desktoken (in production) fell back to
// its hardcoded "example-org" default owner regardless of the actual target repo.
func TestCreateMintsWorkerTokenScopedToOwnRepo(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)

	rc := run([]string{"create", "--as-app", "--title", "scoped mint", "--body-min", "desktoken must be told which repo it's minting for\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create rc = %d, want 0", rc)
	}
	found := false
	for _, c := range *calls {
		if len(c) >= 2 && filepath.Base(c[0]) == "desktoken" && c[1] == "worker" {
			found = true
			if !callContainsAll(c, "--repo", "example-org/tracker") {
				t.Fatalf("desktoken worker call did not forward --repo <this worktree's repo>: %v", c)
			}
		}
	}
	if !found {
		t.Fatalf("expected a `desktoken worker` call; calls: %v", *calls)
	}
}

// TestCreateMediciFinanceRepoSucceeds is the end-to-end regression test for #565
// (mirroring #563/#562 for deskreply): a worktree whose origin is under an org OTHER
// than desktoken's hardcoded "example-org" default (medici-finance/assay here —
// the worker App is installed on both accounts) must still mint a token that gh can
// actually use against that repo.
//
// Before the fix, mintWorkerToken never forwarded --repo, so the fake (and, per #563's
// investigation, the REAL) desktoken resolution defaulted to owner "example-org" —
// minting a token for the WRONG installation. The subsequent `gh pr list`/`gh pr
// create` then failed exactly as GitHub's real API does for a token with no access to
// the target repo: "GraphQL: Could not resolve to a Repository with the name '...'",
// surfacing as deskpr exit 6. This test fails the same way on the pre-fix code path and
// passes once --repo is forwarded.
func TestCreateMediciFinanceRepoSucceeds(t *testing.T) {
	work := newMediciFixture(t)
	calls := withEnv(t, work)

	rc := run([]string{"create", "--as-app", "--title", "medici finance PR", "--body-min", "creating on a repo under an org other than example-org\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create on a medici-finance-origin worktree rc = %d, want 0 (#565 regression: "+
			"desktoken must resolve the medici-finance installation, not silently default to example-org)", rc)
	}
	if !anyCall(ghCalls(*calls), "pr", "create", "--draft") {
		t.Fatalf("expected a `gh pr create --draft`; gh calls: %v", ghCalls(*calls))
	}
}

// TestCreateDefaultMintsWorkerToken pins the worker-App cutover itself: with NO
// --as-app flag at all, `create` must mint the worker App token. TestCreateAsApp…
// passes the flag explicitly and so pins the flag, not the default — reverting
// deskpr.go's `fs.Bool("as-app", true, …)` to false leaves that test green.
func TestCreateDefaultMintsWorkerToken(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)

	rc := run([]string{"create", "--title", "default is as-app", "--body-min", "posted as worker app by default\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create (no --as-app) rc = %d, want 0", rc)
	}
	found := false
	for _, c := range *calls {
		if len(c) >= 2 && filepath.Base(c[0]) == "desktoken" && c[1] == "worker" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected `desktoken worker` call with NO --as-app flag (default must be true); calls: %v", *calls)
	}
	if !anyCall(ghCalls(*calls), "pr", "create", "--draft") {
		t.Fatalf("expected `gh pr create --draft`; gh calls: %v", ghCalls(*calls))
	}
}

// TestUpdateDefaultMintsWorkerToken is the `update` half of the same default pin.
func TestUpdateDefaultMintsWorkerToken(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1") // open draft PR on the branch

	rc := run([]string{"update"})
	if rc != deskkit.ExitOK {
		t.Fatalf("update (no --as-app) rc = %d, want 0", rc)
	}
	found := false
	for _, c := range *calls {
		if len(c) >= 2 && filepath.Base(c[0]) == "desktoken" && c[1] == "worker" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected `desktoken worker` call with NO --as-app flag on update (default must be true); calls: %v", *calls)
	}
}

func TestCreateNoAsAppUsesAmbientIdentity(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)

	rc := run([]string{"create", "--as-app=false", "--title", "ambient test", "--body-min", "posted as example-org\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create --as-app=false rc = %d, want 0", rc)
	}
	// No desktoken worker call should have been made — ambient gh identity used.
	for _, c := range *calls {
		if len(c) >= 2 && filepath.Base(c[0]) == "desktoken" && c[1] == "worker" {
			t.Fatalf("unexpected `desktoken worker` call when --as-app=false: %v", *calls)
		}
	}
	// Still pushes and creates the PR normally.
	if !anyCall(ghCalls(*calls), "pr", "create", "--draft") {
		t.Fatalf("expected `gh pr create --draft`; gh calls: %v", ghCalls(*calls))
	}
}

func TestUpdateAsAppMintsWorkerToken(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1") // open draft PR on the branch

	rc := run([]string{"update", "--as-app"})
	if rc != deskkit.ExitOK {
		t.Fatalf("update --as-app rc = %d, want 0", rc)
	}
	found := false
	for _, c := range *calls {
		if len(c) >= 2 && filepath.Base(c[0]) == "desktoken" && c[1] == "worker" {
			found = true
			// #565 regression: the update call site must forward --repo too, not just
			// create's.
			if !callContainsAll(c, "--repo", "example-org/tracker") {
				t.Fatalf("desktoken worker call on update did not forward --repo <this worktree's repo>: %v", c)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected `desktoken worker` call when --as-app is set on update; calls: %v", *calls)
	}
}

func TestUpdateNoAsAppUsesAmbientIdentity(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1") // open draft PR on the branch

	rc := run([]string{"update", "--as-app=false"})
	if rc != deskkit.ExitOK {
		t.Fatalf("update --as-app=false rc = %d, want 0", rc)
	}
	for _, c := range *calls {
		if len(c) >= 2 && filepath.Base(c[0]) == "desktoken" && c[1] == "worker" {
			t.Fatalf("unexpected `desktoken worker` call when --as-app=false on update; calls: %v", *calls)
		}
	}
}

// TestCreateRefusesNoTrailer is the example-stream/02 regression test: create refuses
// (exit 5) a body without a Brief:/Issue: trailer BEFORE any network call — neither the
// fake gh nor the fake desktoken binary is reached. In-process call so the refusal
// error is asserted directly (the subprocess path prints to the real stderr).
func TestCreateRefusesNoTrailer(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)

	err := cmdCreate([]string{"--title", "add feature", "--body-min", "does the thing"})
	if !deskkit.IsRefused(err) {
		t.Fatalf("trailer-less create err = %v, want exit-5 refusal", err)
	}
	if !strings.Contains(err.Error(), "Brief: <stream>/<NN>") {
		t.Fatalf("refusal must name the missing line; got: %v", err)
	}
	for _, c := range *calls {
		if len(c) > 0 && (c[0] == "gh" || c[0] == "desktoken") {
			t.Fatalf("trailer refusal must precede any network call; calls: %v", *calls)
		}
	}
}

// TestUpdateRefusesNoTrailer — update refuses (exit 5) when the EXISTING PR's body
// carries no trailer, naming the line to add (the migration-window behavior for
// pre-trailer PRs). No push happens.
func TestUpdateRefusesNoTrailer(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	t.Setenv("FAKEGH_PR_BODY", "no trailer here\n")

	err := cmdUpdate(nil)
	if !deskkit.IsRefused(err) {
		t.Fatalf("trailer-less update err = %v, want exit-5 refusal", err)
	}
	if !strings.Contains(err.Error(), "Brief: <stream>/<NN>") {
		t.Fatalf("refusal must name the missing line; got: %v", err)
	}
	if anyCall(gitCalls(*calls), "push") {
		t.Fatalf("no push on trailer refusal: %v", gitCalls(*calls))
	}
}

func TestCreateIdempotentNoopWhenPRExists(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1") // fake gh reports an existing open PR on the branch

	rc := run([]string{"create", "--title", "add feature", "--body-min", "x\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("idempotent create rc = %d, want 0 (noop)", rc)
	}
	// No PR was created and nothing was pushed (the #140/#148 duplicate-PR class).
	if anyCall(ghCalls(*calls), "pr", "create") {
		t.Fatalf("a PR was created despite an existing open PR: %v", ghCalls(*calls))
	}
	if anyCall(gitCalls(*calls), "push") {
		t.Fatalf("a push happened on the idempotent noop path: %v", gitCalls(*calls))
	}
}

func TestCreateOnDefaultBranchRefuses(t *testing.T) {
	work := newBaseFixture(t)
	mustGit(t, work, "checkout", "main")
	calls := withEnv(t, work)

	rc := run([]string{"create", "--title", "x", "--body-min", "y\nBrief: fixture/01"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("on default branch rc = %d, want 5", rc)
	}
	assertNoPushNoCreate(t, *calls)
}

func TestCreateZeroCommitsAheadRefuses(t *testing.T) {
	work := newBaseFixture(t)
	mustGit(t, work, "checkout", "main")
	mustGit(t, work, "checkout", "-b", "feature/empty") // no commits ahead of origin/main
	calls := withEnv(t, work)

	rc := run([]string{"create", "--title", "x", "--body-min", "y\nBrief: fixture/01"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("zero commits ahead rc = %d, want 5", rc)
	}
	assertNoPushNoCreate(t, *calls)
}

func TestCreateStagedUncommittedRefuses(t *testing.T) {
	work := newBaseFixture(t)
	writeFile(t, filepath.Join(work, "staged.txt"), "wip\n")
	mustGit(t, work, "add", "staged.txt") // staged, not committed
	calls := withEnv(t, work)

	rc := run([]string{"create", "--title", "x", "--body-min", "y\nBrief: fixture/01"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("staged-uncommitted rc = %d, want 5", rc)
	}
	assertNoPushNoCreate(t, *calls)
}

func TestCreateOriginOutsideRepoSetRefuses(t *testing.T) {
	work := newBaseFixture(t)
	mustGit(t, work, "remote", "set-url", "origin", "https://github.com/otheruser/otherrepo.git")
	calls := withEnv(t, work)

	rc := run([]string{"create", "--title", "x", "--body-min", "y\nBrief: fixture/01"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("origin outside repo set rc = %d, want 5", rc)
	}
	assertNoPushNoCreate(t, *calls)
}

func TestCreateUnreadableOriginHeadUnverifiable(t *testing.T) {
	work := newBaseFixture(t)
	mustGit(t, work, "symbolic-ref", "--delete", "refs/remotes/origin/HEAD")
	calls := withEnv(t, work)

	rc := run([]string{"create", "--title", "x", "--body-min", "y\nBrief: fixture/01"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("unreadable origin/HEAD rc = %d, want 6", rc)
	}
	assertNoPushNoCreate(t, *calls)
}

// TestCreateStrayLocalOriginMainBranchStillCreates reproduces #840: a stray LOCAL branch
// literally named `origin/main` (planted by the `deskwt --branch origin/main` gotcha) makes
// the short name `origin/main` ambiguous. The old preflight derived the base ref via
// `symbolic-ref --short` + `"origin/" + name`, which under this ambiguity produced the
// unresolvable `origin/remotes/origin/main` and aborted `git rev-list` at exit 128 (rc 6)
// BEFORE any push. The decoy is planted at the feature tip so a wrongly-resolved base would
// also miscount/misdiff; the fix resolves the fully-qualified `refs/remotes/origin/main` and
// creates the PR normally.
func TestCreateStrayLocalOriginMainBranchStillCreates(t *testing.T) {
	work := newBaseFixture(t)
	// Stray local ref refs/heads/origin/main at the feature tip — NOT the real main.
	mustGit(t, work, "branch", "origin/main", "HEAD")
	// Sanity: the short name really is ambiguous now (heads + remotes both match).
	if got := mustGit(t, work, "rev-parse", "--symbolic-full-name", "refs/heads/origin/main"); got != "refs/heads/origin/main" {
		t.Fatalf("decoy branch not planted: %q", got)
	}
	calls := withEnv(t, work)

	rc := run([]string{"create", "--title", "add feature", "--body-min", "does the thing\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create with stray local origin/main rc = %d, want 0 (regression #840)", rc)
	}
	if !anyCall(gitCalls(*calls), "push", "-u", "origin", "feature/test-branch") {
		t.Fatalf("expected a plain `git push -u origin feature/test-branch`; git calls: %v", gitCalls(*calls))
	}
	if !anyCall(ghCalls(*calls), "pr", "create", "--draft") {
		t.Fatalf("expected `gh pr create --draft`; gh calls: %v", ghCalls(*calls))
	}
	// The base ref used for rev-list/diff must be the fully-qualified remote ref, never the
	// double-prefixed spelling that this bug produced.
	for _, c := range gitCalls(*calls) {
		for _, a := range c[1:] {
			if strings.Contains(a, "origin/remotes/origin/") {
				t.Fatalf("git argv carried the double-prefixed base ref %q: %v", a, c)
			}
		}
	}
}

// TestCreateNonDefaultBaseCountsAgainstThatBase pins the #55 fix: the "commits ahead"
// precondition must count HEAD against the base the PR ACTUALLY opens against (--base),
// not against origin/HEAD (the repo default). A branch that is legitimately one commit
// ahead of its intended base — here a stacked base whose tip is BEHIND the default branch
// — but zero ahead of the default branch used to be false-refused with "no commits ahead
// of refs/remotes/origin/main", forcing the caller onto the ambient-identity `gh pr
// create` fallback that mis-attributes the PR author. It must now create normally, and the
// ahead-count must be taken against refs/remotes/origin/<base>.
func TestCreateNonDefaultBaseCountsAgainstThatBase(t *testing.T) {
	work := newBaseFixture(t)
	// feature/test-branch = init + "feature work". Simulate that commit having landed on
	// origin/main, so HEAD is ZERO commits ahead of the default branch...
	head := mustGit(t, work, "rev-parse", "HEAD")
	mustGit(t, work, "update-ref", "refs/remotes/origin/main", head)
	// ...while the intended stacked base sits at the original main (one commit behind HEAD).
	initSHA := mustGit(t, work, "rev-parse", "HEAD~1")
	mustGit(t, work, "update-ref", "refs/remotes/origin/stacked-base", initSHA)
	// Sanity: 0 ahead of the default branch, 1 ahead of the intended base.
	if got := mustGit(t, work, "rev-list", "--count", "refs/remotes/origin/main..HEAD"); got != "0" {
		t.Fatalf("precondition setup: ahead of origin/main = %q, want 0", got)
	}
	if got := mustGit(t, work, "rev-list", "--count", "refs/remotes/origin/stacked-base..HEAD"); got != "1" {
		t.Fatalf("precondition setup: ahead of origin/stacked-base = %q, want 1", got)
	}
	calls := withEnv(t, work)

	rc := run([]string{"create", "--title", "stacked feature", "--body-min", "does the thing\nBrief: fixture/01", "--base", "stacked-base"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create --base stacked-base rc = %d, want 0 (#55: must not false-refuse a branch ahead of its --base)", rc)
	}
	if !anyCall(gitCalls(*calls), "push", "-u", "origin", "feature/test-branch") {
		t.Fatalf("expected a plain `git push -u origin feature/test-branch`; git calls: %v", gitCalls(*calls))
	}
	if !anyCall(ghCalls(*calls), "pr", "create", "--draft") {
		t.Fatalf("expected `gh pr create --draft`; gh calls: %v", ghCalls(*calls))
	}
	// The ahead-count must have been taken against the --base's remote ref, never origin/main.
	countedAgainstBase := false
	for _, c := range gitCalls(*calls) {
		if len(c) >= 2 && c[1] == "rev-list" {
			for _, a := range c[2:] {
				if a == "refs/remotes/origin/stacked-base..HEAD" {
					countedAgainstBase = true
				}
				if a == "refs/remotes/origin/main..HEAD" {
					t.Fatalf("ahead-count was taken against origin/main, not the --base: %v", c)
				}
			}
		}
	}
	if !countedAgainstBase {
		t.Fatalf("no `git rev-list --count refs/remotes/origin/stacked-base..HEAD` was issued; git calls: %v", gitCalls(*calls))
	}
}

func TestCreateKillSwitchDisabled(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("DESK_TOOLS_DISABLED", "1")

	rc := run([]string{"create", "--title", "x", "--body-min", "y\nBrief: fixture/01"})
	if rc != deskkit.ExitDisabled {
		t.Fatalf("kill switch rc = %d, want 3", rc)
	}
	assertNoPushNoCreate(t, *calls)
}

func TestCreateSecretInBodyRefuses(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)

	rc := run([]string{"create", "--title", "x", "--body-min", "token ghp_" + strings.Repeat("a", 30)})
	if rc != deskkit.ExitRefused {
		t.Fatalf("secret in body rc = %d, want 5", rc)
	}
	assertNoPushNoCreate(t, *calls)
}

// TestCreateDiffHeaderLongPathPasses is #1052's second vector: a real PR (#1065)
// touching tools/desk/internal/deskkit/config.go produced a git-diff header
// (`diff --git a/tools/desk/internal/deskkit/config.go b/…`) whose `a/tools/desk/
// internal/deskkit/config` run is 36 chars of deskkit's [A-Za-z0-9+/=] charset — enough
// to trip the high-entropy-run refusal on the diff header ALONE, with no secret anywhere
// in the diff. Reproduce the exact path depth/shape here; `create` must succeed.
func TestCreateDiffHeaderLongPathPasses(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)

	longPathDir := filepath.Join(work, "tools", "desk", "internal", "deskkit")
	if err := os.MkdirAll(longPathDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(longPathDir, "config.go"), "package deskkit\n")
	mustGit(t, work, "add", "tools/desk/internal/deskkit/config.go")
	mustGit(t, work, "commit", "-m", "add config")

	rc := run([]string{"create", "--title", "add config", "--body-min", "adds a config file\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create rc = %d, want 0 (diff header path must not trip the secret scan); git calls: %v", rc, gitCalls(*calls))
	}
}

// TestCreateDiffContentSecretStillRefuses is the guard-strength half of #1052's second
// vector: stripping git-diff HEADER lines must not reach into an ADDED content line.
// This 40-char run rides in on a `+` content line (not a header line) and must still be
// refused — the meta-line skip is exact-format, not path-shape, so it never touches
// content.
func TestCreateDiffContentSecretStillRefuses(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)

	writeFile(t, filepath.Join(work, "leak.txt"), "token: Qx7pLk2wZt9mNc4bYf6RhVs8Ju3XoAeG5idWn1Dz\n")
	mustGit(t, work, "add", "leak.txt")
	mustGit(t, work, "commit", "-m", "add leak")

	rc := run([]string{"create", "--title", "x", "--body-min", "adds a file\nBrief: fixture/01"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("added-content secret rc = %d, want 5 (refused)", rc)
	}
	assertNoPushNoCreate(t, *calls)
}

// TestCreateDiffAddedPathLinePasses is the added-diff-marker repro (9th shape):
// an ADDED content line that is a bare word-shaped path — a .gitignore entry
// `/tools/approvalguard/approvalguard` (34 chars) — renders in the unified diff as
// `+/tools/approvalguard/approvalguard` (35 chars). isPathLike's "no +/=" rule saw the
// leading diff `+` as disqualifying and refused the whole push, though the real path is
// clean. With the per-line marker strip the 34-char path is scanned on its own and passes.
// The guard partner is TestCreateDiffContentSecretStillRefuses above: a real secret on a
// `+` line still refuses after the marker is stripped.
func TestCreateDiffAddedPathLinePasses(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)

	writeFile(t, filepath.Join(work, ".gitignore"), "/tools/approvalguard/approvalguard\n")
	mustGit(t, work, "add", ".gitignore")
	mustGit(t, work, "commit", "-m", "ignore approvalguard binary")

	rc := run([]string{"create", "--title", "add gitignore", "--body-min", "ignores the built binary\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create rc = %d, want 0 (a +-glued word-shaped path line must not trip the scan); git calls: %v", rc, gitCalls(*calls))
	}
}

func TestCreateMissingBodyRefuses(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	if rc := run([]string{"create", "--title", "x"}); rc != deskkit.ExitRefused {
		t.Fatalf("missing body rc = %d, want 5", rc)
	}
	if rc := run([]string{"create", "--title", "x", "--body-min", "a", "--body-file", "f"}); rc != deskkit.ExitRefused {
		t.Fatalf("both bodies rc = %d, want 5", rc)
	}
}

// TestUpdateNoOpenPRRefuses exercises the empty-listing refusal: with no PR on the branch,
// gh returns [] and update refuses at exit 5 — the same refusal a CLOSED or MERGED PR
// surfaces as, because listOpenPRs asks gh for `--state open` and those states are absent
// from the listing. This test does NOT by itself prove the filter (the empty-list path is
// reachable with or without it); the `--state open` argv is asserted in
// TestUpdateReadyPRPushes, which reds if the filter drifts. Lifting the draft-only
// restriction leaves this refusal untouched: update still only pushes to an OPEN PR.
func TestUpdateNoOpenPRRefuses(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	// FAKEGH_LIST_HAS_PR unset → gh pr list --state open returns [] → no open PR for the
	// branch (a closed/merged PR would be filtered out by --state open the same way).
	rc := run([]string{"update"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("update with no open PR rc = %d, want 5", rc)
	}
	assertNoPushNoCreate(t, *calls)
}

// TestUpdateReadyPRPushes is #788's fix: a ready-flipped (non-draft) OPEN PR on the
// branch is now a valid update target. Before the fix this refused at exit 5 ("PR is not
// a draft"), stranding approved-but-stale PRs with no sanctioned push path. All other
// guards stay in force — this exercises the draft distinction being lifted, nothing else.
func TestUpdateReadyPRPushes(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	t.Setenv("FAKEGH_LIST_DRAFT", "0") // open PR on the branch, but ready (NOT a draft)

	rc := run([]string{"update"})
	if rc != deskkit.ExitOK {
		t.Fatalf("update on a ready (non-draft) open PR rc = %d, want 0; git calls: %v", rc, gitCalls(*calls))
	}
	if !anyCall(gitCalls(*calls), "push", "-u", "origin", "feature/test-branch") {
		t.Fatalf("update did not push the branch to the ready PR: %v", gitCalls(*calls))
	}
	if anyGitForce(*calls) {
		t.Fatalf("update emitted a git --force: %v", gitCalls(*calls))
	}
	// The merged/closed refusal now rests ENTIRELY on this listing filter: with the
	// draft distinction lifted, --state open is the only barrier keeping update off a
	// merged PR (a merged PR is never a draft, so !IsDraft used to catch it too). Assert
	// the literal argv, in the same style as the create --draft assertions — if it ever
	// drifts to --state all, gh returns the merged PR, matchHead matches it by branch,
	// and the push lands on a branch whose PR is already merged.
	if !anyCall(ghCalls(*calls), "pr", "list", "--state", "open") {
		t.Fatalf("no `gh pr list --state open` — merged/closed PRs would be listed; gh calls: %v", ghCalls(*calls))
	}
}

// TestUpdateWrongBranchRefuses proves the head-owns-branch guard (matchHead) survives the
// #788 change: an open PR whose headRefName is a DIFFERENT branch than the caller's is not
// this worktree's PR, so update must refuse and never push — draft/ready is irrelevant.
func TestUpdateWrongBranchRefuses(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	t.Setenv("FAKEGH_LIST_DRAFT", "0")                   // ready PR, to prove it's matchHead (not draft) refusing
	t.Setenv("FAKEGH_LIST_HEAD", "someone-elses-branch") // PR's head != the caller's branch

	rc := run([]string{"update"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("update on a PR whose head is a different branch rc = %d, want 5", rc)
	}
	assertNoPushNoCreate(t, *calls)
}

func TestUpdateDraftPRPushes(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1") // open draft PR on the branch

	rc := run([]string{"update"})
	if rc != deskkit.ExitOK {
		t.Fatalf("update on draft PR rc = %d, want 0", rc)
	}
	if !anyCall(gitCalls(*calls), "push", "-u", "origin", "feature/test-branch") {
		t.Fatalf("update did not push the branch: %v", gitCalls(*calls))
	}
	if anyGitForce(*calls) {
		t.Fatalf("update emitted a git --force: %v", gitCalls(*calls))
	}
}

func assertNoPushNoCreate(t *testing.T, calls [][]string) {
	t.Helper()
	if anyCall(gitCalls(calls), "push") {
		t.Fatalf("a git push happened on a refusal path: %v", gitCalls(calls))
	}
	if anyCall(ghCalls(calls), "pr", "create") {
		t.Fatalf("a gh pr create happened on a refusal path: %v", ghCalls(calls))
	}
}

// TestStripDiffMetaLines exercises reDiffMetaLine/stripDiffMetaLines directly (#1052
// second vector), at line granularity: every git-generated header form is dropped, every
// content line (context/added/removed) survives, and its leading unified-diff marker
// (+/-/space) is stripped so the SCAN sees the content, not the diff syntax (#781).
func TestStripDiffMetaLines(t *testing.T) {
	in := strings.Join([]string{
		"diff --git a/tools/desk/internal/deskkit/config.go b/tools/desk/internal/deskkit/config.go",
		"index e69de29..4b825dc 100644",
		"--- a/tools/desk/internal/deskkit/config.go",
		"+++ b/tools/desk/internal/deskkit/config.go",
		"similarity index 100%",
		"rename from old/tools/desk/internal/deskkit/config.go",
		"rename to tools/desk/internal/deskkit/config.go",
		"@@ -0,0 +1 @@",
		"+package deskkit",
		" context line unchanged",
		"-removed line",
	}, "\n")

	out := stripDiffMetaLines(in)

	// Content survives, but with its leading +/-/space marker stripped (#781).
	for _, mustHave := range []string{"@@ -0,0 +1 @@", "package deskkit", "context line unchanged", "removed line"} {
		if !strings.Contains(out, mustHave) {
			t.Errorf("stripDiffMetaLines dropped a content line it must keep: %q\nout:\n%s", mustHave, out)
		}
	}
	// The unified-diff line markers must be gone (proves the strip actually ran).
	for _, marker := range []string{"+package deskkit", "-removed line"} {
		if strings.Contains(out, marker) {
			t.Errorf("stripDiffMetaLines left a diff line marker glued to content: %q\nout:\n%s", marker, out)
		}
	}
	for _, mustNotHave := range []string{
		"diff --git a/tools/desk/internal/deskkit/config.go",
		"index e69de29..4b825dc 100644",
		"--- a/tools/desk/internal/deskkit/config.go",
		"+++ b/tools/desk/internal/deskkit/config.go",
		"similarity index 100%",
		"rename from old/tools/desk/internal/deskkit/config.go",
		"rename to tools/desk/internal/deskkit/config.go",
	} {
		if strings.Contains(out, mustNotHave) {
			t.Errorf("stripDiffMetaLines kept a header line it must drop: %q\nout:\n%s", mustNotHave, out)
		}
	}
}

// TestStripDiffMetaLinesHunkCollision verifies that a hunk content line whose text
// starts with `++ ` or `-- ` (which in the unified diff renders as `+++ ...` or
// `--- ...`, reconstructing the header prefix) is NOT stripped as a header line. Under
// the old prefix-based strip these lines were silently dropped; under the state-machine
// strip (header-mode exited at `@@ `) they survive and would be caught by BodyCheck.
func TestStripDiffMetaLinesHunkCollision(t *testing.T) {
	const secret = "Qx7pLk2wZt9mNc4bYf6RhVs8Ju3XoAeG5idWn1Dz"
	in := strings.Join([]string{
		"diff --git a/leak.txt b/leak.txt",
		"new file mode 100644",
		"index 0000000..abc1234",
		"--- /dev/null",
		"+++ b/leak.txt",
		"@@ -0,0 +1 @@",
		"+ ++ " + secret, // content line `++ <secret>` renders `+++ <secret>` in diff
	}, "\n")

	out := stripDiffMetaLines(in)

	// The secret-bearing content line must survive the strip.
	if !strings.Contains(out, secret) {
		t.Fatalf("stripDiffMetaLines dropped the ++ <secret> content line (hunk collision); out:\n%s", out)
	}
	// Header lines must still be dropped.
	if strings.Contains(out, "diff --git") || strings.Contains(out, "index ") || strings.Contains(out, "--- /dev/null") {
		t.Fatalf("stripDiffMetaLines kept header lines it must drop; out:\n%s", out)
	}
}

// TestStripDiffMetaLinesDocFragment verifies that a diff fragment quoted in
// documentation (no `^diff --git ` line) is never stripped. The scanner never enters
// header mode, so `---`, `+++`, etc. lines are kept and scanned by BodyCheck.
func TestStripDiffMetaLinesDocFragment(t *testing.T) {
	in := strings.Join([]string{
		"The diff looked like:",
		"```",
		"--- a/foo",
		"+++ b/foo",
		"@@ -1 +1 @@",
		"-old",
		"+new",
		"```",
	}, "\n")

	out := stripDiffMetaLines(in)

	// In a doc fragment without `diff --git`, nothing should be stripped.
	for _, mustHave := range []string{"--- a/foo", "+++ b/foo", "@@ -1 +1 @@"} {
		if !strings.Contains(out, mustHave) {
			t.Errorf("stripDiffMetaLines dropped a doc line: %q\nout:\n%s", mustHave, out)
		}
	}
}

// TestCreateDiffHunkContentPlusSecretRefuses verifies end-to-end that a file whose
// content line is `++ <40-char-secret>` (rendered as `+++ <secret>` in the unified
// diff) is still caught by the secret scan. The old prefix-based strip would silently
// drop this line as a `+++` header; the state-machine strip keeps it.
func TestCreateDiffHunkContentPlusSecretRefuses(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)

	const secret = "Qx7pLk2wZt9mNc4bYf6RhVs8Ju3XoAeG5idWn1Dz"
	writeFile(t, filepath.Join(work, "leak.txt"), "++ "+secret+"\n")
	mustGit(t, work, "add", "leak.txt")
	mustGit(t, work, "commit", "-m", "add file with ++ secret")

	rc := run([]string{"create", "--title", "x", "--body-min", "adds a file\nBrief: fixture/01"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("++ content line secret rc = %d, want 5 (refused)", rc)
	}
	assertNoPushNoCreate(t, *calls)
}

func TestParseRepo(t *testing.T) {
	cases := map[string]string{
		"https://github.com/example-org/tracker.git":              "example-org/tracker",
		"https://github.com/example-org/agents":                   "example-org/agents",
		"git@github.com:example-org/examples.git":                 "example-org/examples",
		"ssh://git@github.com/example-org/example-reconciler.git": "example-org/example-reconciler",
	}
	for in, want := range cases {
		got, err := parseRepo(in)
		if err != nil || got != want {
			t.Errorf("parseRepo(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

// --- outward-write budget (#439, third review) ----------------------------

// fixtureRepo is the slug `create` resolves from the fixture's origin, and so the repo its
// audit lines carry. Seeds must use it or they land in a bucket no gate here reads.
const fixtureRepo = "example-org/tracker"

// seedDeskprAudit appends n charged `deskpr` entries in the shape `shape` produces.
// HOME must already be set by withEnv.
func seedDeskprAudit(t *testing.T, n int, shape func(i int) deskkit.Entry) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir audit dir: %v", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open audit: %v", err)
	}
	defer f.Close()
	for i := 0; i < n; i++ {
		e := shape(i)
		e.TS = time.Now().UTC().Format(time.RFC3339)
		e.Tool = "deskpr"
		if e.Result == "" {
			e.Result = deskkit.ResultOK
		}
		b, _ := json.Marshal(e)
		if _, werr := f.Write(append(b, '\n')); werr != nil {
			t.Fatalf("seed audit: %v", werr)
		}
	}
}

// createAuditShape is the line a SUCCESSFUL create writes: this repo, and the number of the
// PR it just made — a new, previously unseen number every time (deskpr.go, `ac.pr = &n`).
func createAuditShape(i int) deskkit.Entry {
	n := 5000 + i
	return deskkit.Entry{Repo: fixtureRepo, PR: &n, Verb: "create"}
}

// TestCreateGateCountsItsOwnSuccessfulWrites is the regression test for the third review's
// finding 1, and the one that must redden if the create gate is ever aimed at a bucket
// creates do not fill.
//
// The defect it pins was invisible to every other test here: `create` gated on the repo's
// UNNUMBERED bucket while auditing each success with a real PR number, so no successful
// create could land in the bucket its own gate counted. Ninety-nine successes still admitted
// the hundredth; only the 100/hr per-repo tier was holding a verb that was hard-capped at 10
// before the tiers existed — and that verb is PR creation, the historical flood.
//
// Seeding the cap in the shape a create ACTUALLY writes is what makes this bite. A seed
// written in the shape the gate wants would pass against the broken code too, which is how
// the original slipped through.
func TestCreateGateCountsItsOwnSuccessfulWrites(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	seedDeskprAudit(t, deskkit.RateLimitPerPRPerHour, createAuditShape)

	rc := run([]string{"create", "--title", "one too many", "--body-min", "does the thing\nBrief: fixture/01"})
	if rc != deskkit.ExitRateLimited {
		t.Fatalf("create rc = %d, want %d — %d successful creates in the window must exhaust the budget; "+
			"the gate is reading a bucket creates do not fill",
			rc, deskkit.ExitRateLimited, deskkit.RateLimitPerPRPerHour)
	}
	if anyCall(ghCalls(*calls), "pr", "create") {
		t.Fatalf("a PR was created past an exhausted budget: %v", ghCalls(*calls))
	}
	if anyCall(gitCalls(*calls), "push", "-u", "origin", "feature/test-branch") {
		t.Fatalf("a push happened past an exhausted budget: %v", gitCalls(*calls))
	}
}

// TestCreateUnderBudgetStillCreates is the other half — the gate is a budget, not a block.
// Without it the test above is satisfied by a gate that refuses everything.
func TestCreateUnderBudgetStillCreates(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	seedDeskprAudit(t, deskkit.RateLimitPerPRPerHour-1, createAuditShape)

	rc := run([]string{"create", "--title", "still under", "--body-min", "does the thing\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create rc = %d, want 0 at cap-1", rc)
	}
	if !anyCall(ghCalls(*calls), "pr", "create", "--draft") {
		t.Fatalf("expected the create to go through under budget; gh calls: %v", ghCalls(*calls))
	}
}

// TestCreateGateIsNotInverted pins the direction the old scoping got backwards. FAILED
// creates record no PR number, so under the unnumbered-bucket gate they were the ONLY
// lines that accumulated: ten failures locked the tool out for an hour while ninety-nine
// successes did not move it. Failures must count the same as successes here — neither more
// nor less — so a handful of them cannot starve a working tool.
func TestCreateGateIsNotInverted(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	// Unverifiable charges budget and records no PR number — the failed-create shape.
	seedDeskprAudit(t, deskkit.RateLimitPerPRPerHour-1, func(i int) deskkit.Entry {
		return deskkit.Entry{Repo: fixtureRepo, Verb: "create", Result: deskkit.ResultUnverifiable}
	})

	rc := run([]string{"create", "--title", "after failures", "--body-min", "does the thing\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create rc = %d, want 0 — %d failed creates must not lock the tool out below the cap",
			rc, deskkit.RateLimitPerPRPerHour-1)
	}
	if !anyCall(ghCalls(*calls), "pr", "create", "--draft") {
		t.Fatalf("expected the create to go through; gh calls: %v", ghCalls(*calls))
	}
}

// TestUpdateGateCountsItsOwnWrites is the same invariant for `update`, which gates on a
// real PR number and audits with the same one. Aligned already — pinned so it stays that way.
func TestUpdateGateCountsItsOwnWrites(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	// 42 is the number the fake `gh pr list` returns, i.e. the PR `update` resolves and
	// then records — seed the bucket the call site actually writes to.
	seedDeskprAudit(t, deskkit.RateLimitPerPRPerHour, func(i int) deskkit.Entry {
		n := 42
		return deskkit.Entry{Repo: fixtureRepo, PR: &n, Verb: "update"}
	})

	rc := run([]string{"update"})
	if rc != deskkit.ExitRateLimited {
		t.Fatalf("update rc = %d, want %d — PR #42's budget is spent", rc, deskkit.ExitRateLimited)
	}
	if anyCall(gitCalls(*calls), "push", "-u", "origin", "feature/test-branch") {
		t.Fatalf("a push happened past an exhausted budget: %v", gitCalls(*calls))
	}
}
