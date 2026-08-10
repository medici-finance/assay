package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// runGit runs a git command in dir, failing the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// runGitEnv is runGit with extra environment entries ("K=V"), so a test can pin
// a commit's author/committer dates and get a fixed, machine-independent %ct.
func runGitEnv(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// fixedDate is deliberately expressed with a non-UTC offset (+05:30). git
// normalises it to epoch seconds, so a correct UTC conversion lands on
// 2021-03-03T23:36:07Z — a different calendar day from the local rendering in
// the committing zone. Any implementation that leaked local time would fail.
const (
	fixedDate    = "2021-03-04T05:06:07+05:30"
	fixedDateUTC = "2021-03-03T23:36:07Z"
)

// initRepoAt creates a git repo containing one commit stamped at fixedDate.
func initRepoAt(t *testing.T, dir, date string) {
	t.Helper()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "f.txt")
	runGitEnv(t, dir, []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date},
		"commit", "-q", "-m", "init")
}

// TestGitCommitTimeIsUTC pins HEAD's timestamp to a known instant expressed in
// a non-UTC zone and asserts gitCommitTime returns it as UTC — both the
// location and the wall clock, so a local-time leak cannot pass.
func TestGitCommitTimeIsUTC(t *testing.T) {
	dir := t.TempDir()
	initRepoAt(t, dir, fixedDate)

	got := gitCommitTime(dir)
	if got.IsZero() {
		t.Fatal("gitCommitTime returned the zero time for a repo with a commit")
	}
	if got.Location() != time.UTC {
		t.Errorf("gitCommitTime location = %v, want UTC", got.Location())
	}
	if s := got.Format(time.RFC3339); s != fixedDateUTC {
		t.Errorf("gitCommitTime = %s, want %s", s, fixedDateUTC)
	}
}

// TestGitCommitTimeDeterministic is the property the helper exists for: for a
// fixed tree the value never varies between calls, and it moves only when HEAD
// moves. Wall-clock time advances across the calls; the result must not.
func TestGitCommitTimeDeterministic(t *testing.T) {
	dir := t.TempDir()
	initRepoAt(t, dir, fixedDate)

	first := gitCommitTime(dir)
	for i := 0; i < 3; i++ {
		if got := gitCommitTime(dir); !got.Equal(first) {
			t.Fatalf("call %d = %v, want %v — not deterministic for a fixed tree", i+2, got, first)
		}
	}

	// A new HEAD is a new tree, so the value must track it (otherwise the
	// helper would be "deterministic" by being constant, which is useless).
	const laterDate = "2022-06-07T08:09:10Z"
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "f.txt")
	runGitEnv(t, dir, []string{"GIT_AUTHOR_DATE=" + laterDate, "GIT_COMMITTER_DATE=" + laterDate},
		"commit", "-q", "-m", "second")

	second := gitCommitTime(dir)
	if second.Equal(first) {
		t.Fatal("gitCommitTime did not change after a new commit — it is not tracking HEAD")
	}
	if s := second.Format(time.RFC3339); s != laterDate {
		t.Errorf("gitCommitTime after second commit = %s, want %s", s, laterDate)
	}
}

// TestGitCommitTimeShallowClone asserts a --depth 1 clone still yields the
// exact HEAD timestamp — CI checkouts are routinely shallow, and a helper that
// degraded there would silently push callers onto the wall-clock fallback.
func TestGitCommitTimeShallowClone(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "src")
	dst := filepath.Join(base, "shallow")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	initRepoAt(t, src, fixedDate)

	// --depth needs a transport that supports it; file:// gives us one.
	runGit(t, base, "clone", "-q", "--depth", "1", "file://"+src, dst)

	got := gitCommitTime(dst)
	if s := got.Format(time.RFC3339); s != fixedDateUTC {
		t.Errorf("gitCommitTime on a shallow clone = %s, want %s", s, fixedDateUTC)
	}
}

// TestGitCommitTimeUnbornHEAD covers a repo with no commits: `git show HEAD`
// fails, and the contract is the zero time rather than a panic or a bogus date.
func TestGitCommitTimeUnbornHEAD(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")

	if got := gitCommitTime(dir); !got.IsZero() {
		t.Errorf("gitCommitTime on an unborn HEAD = %v, want the zero time", got)
	}
}

// TestGitCommitTimeNonRepo covers a plain directory — the testdata-fixture case
// gitCurrentSHA already tolerates. Skipped if the temp dir happens to sit
// inside a repo, since git would then walk up and legitimately find a HEAD.
func TestGitCommitTimeNonRepo(t *testing.T) {
	dir := t.TempDir()
	probe := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
	if err := probe.Run(); err == nil {
		t.Skip("temp dir is inside a git repo; cannot exercise the non-repo path here")
	}

	if got := gitCommitTime(dir); !got.IsZero() {
		t.Errorf("gitCommitTime outside a repo = %v, want the zero time", got)
	}
}

// TestListRemoteBranches round-trips against a local bare-repo "origin" — no
// live network required, so it stays green offline and in a sandboxed CI.
func TestListRemoteBranches(t *testing.T) {
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	work := filepath.Join(base, "work")

	runGit(t, base, "init", "--bare", "-q", "-b", "main", origin)
	runGit(t, base, "clone", "-q", origin, work)
	runGit(t, work, "config", "user.email", "test@example.com")
	runGit(t, work, "config", "user.name", "test")
	runGit(t, work, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(work, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, "add", "f.txt")
	runGit(t, work, "commit", "-q", "-m", "init")
	runGit(t, work, "push", "-q", "-u", "origin", "main")
	runGit(t, work, "checkout", "-q", "-b", "fix/ledger-hardening-06-idempotency")
	runGit(t, work, "push", "-q", "-u", "origin", "fix/ledger-hardening-06-idempotency")

	branches, err := listRemoteBranches(work)
	if err != nil {
		t.Fatalf("listRemoteBranches against a live local remote: %v", err)
	}
	sort.Strings(branches)
	want := []string{"fix/ledger-hardening-06-idempotency", "main"}
	if len(branches) != len(want) || branches[0] != want[0] || branches[1] != want[1] {
		t.Fatalf("listRemoteBranches = %v, want %v", branches, want)
	}
}

// TestListRemoteBranchesDegradesGracefully asserts that an unreachable/missing
// remote returns a NAMED error rather than hanging, panicking, or — the
// assay-toolkit#305 defect — an empty branch list indistinguishable from
// "nothing is claimed". The error text must name the cause, since it is what
// the degraded board reports to its reader.
func TestListRemoteBranchesDegradesGracefully(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	// No "origin" remote configured at all.
	branches, err := listRemoteBranches(dir)
	if err == nil {
		t.Fatalf("listRemoteBranches on a repo with no origin = (%v, nil), want an error", branches)
	}
	if branches != nil {
		t.Fatalf("listRemoteBranches returned branches %v alongside an error", branches)
	}
	if !strings.Contains(err.Error(), "ls-remote") {
		t.Fatalf("error %q does not name the failing command", err)
	}
}

// TestListRemoteBranchesTimeoutIsNamed forces the REAL timeout path — a git
// remote helper that never returns — and asserts the error says "timed out"
// and points at the override knob. This is the exact path that fired in
// assay-toolkit#305 (a 3s deadline on a slow remote); without a test that
// observes it failing, the fix is unproven.
func TestListRemoteBranchesTimeoutIsNamed(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")

	// A "remote helper" on PATH that hangs: git invokes git-remote-<scheme> for
	// an unknown URL scheme, so `hang://x` blocks until the context deadline.
	bin := t.TempDir()
	helper := filepath.Join(bin, "git-remote-hang")
	script := "#!/bin/sh\nwhile :; do sleep 1; done\n"
	if err := os.WriteFile(helper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "remote", "add", "origin", "hang://example.invalid/repo.git")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(remoteTimeoutEnv, "300ms")

	start := time.Now()
	branches, err := listRemoteBranches(dir)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("listRemoteBranches against a hanging remote = (%v, nil), want a timeout error", branches)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error %q does not identify itself as a timeout", err)
	}
	if !strings.Contains(err.Error(), remoteTimeoutEnv) {
		t.Fatalf("timeout error %q does not name the %s override", err, remoteTimeoutEnv)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("timeout not honored: took %s", elapsed)
	}
}

// TestRemoteBranchTimeoutEnv pins the override parsing, including the
// fail-safe: a garbage or non-positive value falls back to the default rather
// than collapsing the window to zero (which would make every run degrade).
func TestRemoteBranchTimeoutEnv(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want time.Duration
	}{
		{"", defaultRemoteBranchTimeout},
		{"45s", 45 * time.Second},
		{"nonsense", defaultRemoteBranchTimeout},
		{"0s", defaultRemoteBranchTimeout},
		{"-5s", defaultRemoteBranchTimeout},
	} {
		t.Setenv(remoteTimeoutEnv, tc.env)
		if got := remoteBranchTimeout(); got != tc.want {
			t.Errorf("remoteBranchTimeout() with %s=%q = %s, want %s", remoteTimeoutEnv, tc.env, got, tc.want)
		}
	}
	// The default must be long enough that ordinary slowness (a cold DNS cache,
	// a busy runner) does not silently drop claim filtering — the #305 trigger.
	if defaultRemoteBranchTimeout <= 3*time.Second {
		t.Errorf("defaultRemoteBranchTimeout = %s; the 3s default is what tripped in #305", defaultRemoteBranchTimeout)
	}
}
