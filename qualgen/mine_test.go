package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// --- git fixture harness: stands up real repositories from the local `git`
// binary so the miner is dereferenced against genuine history, not a mock. ---

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary unavailable")
	}
}

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{
		"-c", "user.name=Fixture Author",
		"-c", "user.email=fixture@example.test",
		"-c", "commit.gpgsign=false",
		"-C", dir,
	}, args...)
	cmd := exec.Command("git", full...)
	// Deterministic identities/dates so nothing depends on ambient git config.
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2020-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2020-01-01T00:00:00Z",
	)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, errb.String())
	}
	return strings.TrimSpace(out.String())
}

// initFixtureRepo creates a repo with a text edit, a binary blob, and a
// zero-content-change re-commit — the three shapes Verify calls for.
func initFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")

	// 1) normal text file.
	writeFile(t, dir, "readme.txt", "hello\nworld\n")
	gitCmd(t, dir, "add", "readme.txt")
	gitCmd(t, dir, "commit", "-q", "-m", "add readme")

	// 2) a binary blob (bytes that are not valid line text).
	writeFileBytes(t, dir, "blob.bin", []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0x00, 0x10})
	gitCmd(t, dir, "add", "blob.bin")
	gitCmd(t, dir, "commit", "-q", "-m", "add binary blob")

	// 3) a text edit (real line change).
	writeFile(t, dir, "readme.txt", "hello\nbrave\nworld\n")
	gitCmd(t, dir, "add", "readme.txt")
	gitCmd(t, dir, "commit", "-q", "-m", "edit readme")

	// 4) an empty re-commit (no tree change) — history advances, no diff.
	gitCmd(t, dir, "commit", "-q", "--allow-empty", "-m", "empty re-commit")

	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	writeFileBytes(t, dir, name, []byte(content))
}

func writeFileBytes(t *testing.T, dir, name string, b []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func revListCount(t *testing.T, dir string) int {
	t.Helper()
	out := gitCmd(t, dir, "rev-list", "--count", "HEAD")
	n, err := strconv.Atoi(out)
	if err != nil {
		t.Fatalf("parse rev-list count %q: %v", out, err)
	}
	return n
}

// TestMineExtractsAllCommits is Verify row 3: the mined commit-record count
// equals `git rev-list --count HEAD` on the fixture, dereferencing extraction
// against real history.
func TestMineExtractsAllCommits(t *testing.T) {
	requireGit(t)
	repo := initFixtureRepo(t)
	out := t.TempDir()

	if err := mine(repo, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("mine: %v", err)
	}

	store := NewStore(out)
	commits, err := store.ReadCommits()
	if err != nil {
		t.Fatalf("read commits: %v", err)
	}
	want := revListCount(t, repo)
	if len(commits) != want {
		t.Fatalf("mined %d commits, git rev-list --count HEAD = %d", len(commits), want)
	}

	// The header's tip must equal the repo's actual HEAD.
	head := gitCmd(t, repo, "rev-parse", "HEAD")
	h, err := store.ReadHeader()
	if err != nil || h == nil {
		t.Fatalf("read header: %v (nil=%v)", err, h == nil)
	}
	if h.TipSHA != head {
		t.Fatalf("header tip %q != HEAD %q", h.TipSHA, head)
	}

	// The binary blob must have been recorded could-not-measure, and the text
	// edits measured — so coverage reflects the three-state split, never a
	// silent zero.
	if h.Coverage.CouldNotMeasure == 0 {
		t.Error("expected at least one could-not-measure diff (the binary blob)")
	}
	if h.Coverage.Measured == 0 {
		t.Error("expected at least one measured diff (the text edits)")
	}
}

// TestMineIncrementalExtends is Verify row 4: a second mine over added commits
// appends records and advances the recorded tip/horizon, and re-mining with no
// new commits is a no-op — prior JSONL lines stay byte-identical
// (extend-never-replace).
func TestMineIncrementalExtends(t *testing.T) {
	requireGit(t)
	repo := initFixtureRepo(t)
	out := t.TempDir()

	// First mine.
	if err := mine(repo, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("mine 1: %v", err)
	}
	store := NewStore(out)
	firstCount := len(mustReadCommits(t, store))
	h1 := mustReadHeader(t, store)

	commitsBytes1 := readRaw(t, out, commitsTable)
	diffsBytes1 := readRaw(t, out, diffsTable)

	// A re-mine with NO new commits must not touch the tables.
	if err := mine(repo, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("re-mine no-op: %v", err)
	}
	if got := readRaw(t, out, commitsTable); !bytes.Equal(got, commitsBytes1) {
		t.Fatal("no-op re-mine rewrote commits.jsonl — extend-never-replace violated")
	}
	if got := readRaw(t, out, diffsTable); !bytes.Equal(got, diffsBytes1) {
		t.Fatal("no-op re-mine rewrote diffs.jsonl — extend-never-replace violated")
	}
	h2 := mustReadHeader(t, store)
	if h2.TipSHA != h1.TipSHA || h2.Horizon != h1.Horizon {
		t.Fatalf("no-op re-mine moved tip/horizon: %+v -> %+v", h1, h2)
	}

	// Now add commits and mine again: records appended, tip advanced, prior
	// lines untouched.
	writeFile(t, repo, "more.txt", "another file\n")
	gitCmd(t, repo, "add", "more.txt")
	gitCmd(t, repo, "commit", "-q", "-m", "add more")
	writeFile(t, repo, "more.txt", "another file\nsecond line\n")
	gitCmd(t, repo, "add", "more.txt")
	gitCmd(t, repo, "commit", "-q", "-m", "extend more")

	if err := mine(repo, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("mine 2: %v", err)
	}

	// Prior commit lines must be a byte-identical PREFIX of the new file.
	commitsBytes2 := readRaw(t, out, commitsTable)
	if !bytes.HasPrefix(commitsBytes2, commitsBytes1) {
		t.Fatal("incremental mine did not preserve prior commit lines as a prefix — extend-never-replace violated")
	}
	if !bytes.HasPrefix(readRaw(t, out, diffsTable), diffsBytes1) {
		t.Fatal("incremental mine did not preserve prior diff lines as a prefix")
	}

	secondCount := len(mustReadCommits(t, store))
	if secondCount != firstCount+2 {
		t.Fatalf("expected %d commits after adding 2, got %d", firstCount+2, secondCount)
	}
	h3 := mustReadHeader(t, store)
	newHead := gitCmd(t, repo, "rev-parse", "HEAD")
	if h3.TipSHA != newHead {
		t.Fatalf("tip did not advance: header %q != HEAD %q", h3.TipSHA, newHead)
	}
	if h3.TipSHA == h1.TipSHA {
		t.Fatal("tip must have advanced after new commits")
	}
	// Horizon (earliest reachable) must NOT move backward on an extend.
	if h3.Horizon != h1.Horizon {
		t.Fatalf("horizon moved on an incremental extend: %q -> %q", h1.Horizon, h3.Horizon)
	}
}

// TestMineOpensLinkedWorktree pins a real bug found running Verify #7 from
// inside a dispatched worker's own worktree (the desk's standard isolation
// model, C1): plain go-git PlainOpen does not follow a linked worktree's
// `commondir` file, so `r.Head()` fails with "reference not found" for a
// branch ref that lives in the main repo's `.git` rather than the worktree's
// private gitdir. `mine` must succeed when --repo points at a linked
// worktree, not only a normal clone.
func TestMineOpensLinkedWorktree(t *testing.T) {
	requireGit(t)
	repo := initFixtureRepo(t)

	wtDir := filepath.Join(t.TempDir(), "wt")
	gitCmd(t, repo, "worktree", "add", "-b", "wt-branch", wtDir)

	out := t.TempDir()
	if err := mine(wtDir, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("mine against a linked worktree must succeed, got: %v", err)
	}

	store := NewStore(out)
	commits := mustReadCommits(t, store)
	want := revListCount(t, wtDir)
	if len(commits) != want {
		t.Fatalf("mined %d commits from the worktree, git rev-list --count HEAD = %d", len(commits), want)
	}
}

func mustReadCommits(t *testing.T, s *Store) []Commit {
	t.Helper()
	c, err := s.ReadCommits()
	if err != nil {
		t.Fatalf("read commits: %v", err)
	}
	return c
}

func mustReadHeader(t *testing.T, s *Store) *MineHeader {
	t.Helper()
	h, err := s.ReadHeader()
	if err != nil || h == nil {
		t.Fatalf("read header: %v (nil=%v)", err, h == nil)
	}
	return h
}

func readRaw(t *testing.T, root, table string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, qualityDir, table))
	if err != nil {
		t.Fatalf("read %s: %v", table, err)
	}
	return b
}
