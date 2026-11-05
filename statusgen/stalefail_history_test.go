package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func staleHistoryFixture(t *testing.T) (root, landing string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root = t.TempDir()
	gitInit(t, root, "T", "t@example.invalid")
	runGit(t, root, "checkout", "-b", "main")
	runGit(t, root, "config", "core.abbrev", "7")
	mustWrite(t, filepath.Join(root, "check.go"), "before\n")
	runGit(t, root, "add", "check.go")
	runGitEnv(t, root, []string{"GIT_AUTHOR_DATE=2026-08-01T12:00:00Z", "GIT_COMMITTER_DATE=2026-08-01T12:00:00Z"}, "commit", "-qm", "initial")
	runGit(t, root, "checkout", "-b", "repair")
	mustWrite(t, filepath.Join(root, "check.go"), "fixed\n")
	runGit(t, root, "add", "check.go")
	runGitEnv(t, root, []string{"GIT_AUTHOR_DATE=2026-08-03T12:00:00Z", "GIT_COMMITTER_DATE=2026-08-03T12:00:00Z"}, "commit", "-qm", "fix #72")
	runGit(t, root, "checkout", "main")
	runGitEnv(t, root, []string{"GIT_AUTHOR_DATE=2026-08-04T12:00:00Z", "GIT_COMMITTER_DATE=2026-08-04T12:00:00Z"}, "merge", "--no-ff", "-m", "land repair", "repair")
	landing = strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD")))
	runGit(t, root, "update-ref", "refs/remotes/origin/main", landing)
	return root, landing
}

func staleHistoryLogCounter(t *testing.T, failFirst bool) string {
	t.Helper()
	native, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "log-starts")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	fault := ""
	if failFirst {
		fault = "if [ ! -e " + quote(filepath.Join(dir, "fault-used")) + " ]; then : >" + quote(filepath.Join(dir, "fault-used")) + "; exit 73; fi\n"
	}
	script := "#!/bin/sh\nfor arg do\nif [ \"$arg\" = log ]; then echo log >>" + quote(marker) + "\n" + fault + "break\nfi\ndone\nexec " + quote(native) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}

func staleHistoryStarts(t *testing.T, marker string) int {
	t.Helper()
	b, err := os.ReadFile(marker)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return len(strings.Fields(string(b)))
}

func staleHistoryAnswer(t *testing.T, root string, issue int, day int) string {
	t.Helper()
	got, ok := newestFailFix(root, &BriefFile{Issues: []int{issue}}, time.Date(2026, 8, day, 0, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("history became could-not-check")
	}
	return got
}

// Repeated briefs share the native history bytes, not the issue/date decision.
func TestStaleFailHistoryQueryReuse(t *testing.T) {
	root, landing := staleHistoryFixture(t)
	marker := staleHistoryLogCounter(t, false)
	end := beginGitReadSession()
	defer end()
	if got := staleHistoryAnswer(t, root, 72, 1); got != landing[:7] {
		t.Fatalf("first repair=%q want %q", got, landing[:7])
	}
	first := staleHistoryStarts(t, marker)
	if got := staleHistoryAnswer(t, root, 72, 1); got != landing[:7] {
		t.Fatalf("repeated repair=%q", got)
	}
	if got := staleHistoryAnswer(t, root, 73, 1); got != "" {
		t.Fatalf("another issue borrowed repair %q", got)
	}
	if got := staleHistoryAnswer(t, root, 72, 5); got != "" {
		t.Fatalf("later FAIL borrowed an older repair %q", got)
	}
	if got := staleHistoryStarts(t, marker); got != first {
		t.Fatalf("identical history queries restarted Git: first=%d total=%d", first, got)
	}
}

func TestStaleFailHistoryFreshConfig(t *testing.T) {
	root, landing := staleHistoryFixture(t)
	end := beginGitReadSession()
	defer end()
	if got := staleHistoryAnswer(t, root, 72, 1); got != landing[:7] {
		t.Fatalf("initial abbreviation=%q", got)
	}
	runGit(t, root, "config", "core.abbrev", "12")
	if got := staleHistoryAnswer(t, root, 72, 1); got != landing[:12] {
		t.Fatalf("changed native config served stale abbreviation %q", got)
	}
}

func TestStaleFailHistoryFreshNamespace(t *testing.T) {
	root, landing := staleHistoryFixture(t)
	base := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD^1")))
	runGit(t, root, "update-ref", "refs/heads/decoy", base)
	end := beginGitReadSession()
	defer end()
	if got := staleHistoryAnswer(t, root, 72, 1); got != landing[:7] {
		t.Fatalf("initial repair=%q", got)
	}
	// Native resolution of the existing short range endpoint must stay native.
	runGit(t, root, "symbolic-ref", "refs/heads/"+landing[:7], "refs/heads/decoy")
	outer := activeGitReads
	activeGitReads = nil
	want := staleHistoryAnswer(t, root, 72, 1)
	activeGitReads = outer
	if got := staleHistoryAnswer(t, root, 72, 1); got != want {
		t.Fatalf("changed native alias served stale repair %q; native=%q", got, want)
	}
}

func TestStaleFailHistorySessionBoundary(t *testing.T) {
	root, _ := staleHistoryFixture(t)
	marker := staleHistoryLogCounter(t, false)
	end := beginGitReadSession()
	staleHistoryAnswer(t, root, 72, 1)
	end()
	first := staleHistoryStarts(t, marker)
	end = beginGitReadSession()
	defer end()
	staleHistoryAnswer(t, root, 72, 1)
	if got := staleHistoryStarts(t, marker); got != first*2 {
		t.Fatalf("a new run reused old history: first=%d total=%d", first, got)
	}
}

func TestStaleFailHistoryFailureRetry(t *testing.T) {
	root, landing := staleHistoryFixture(t)
	staleHistoryLogCounter(t, true)
	end := beginGitReadSession()
	defer end()
	if _, ok := newestFailFix(root, &BriefFile{Issues: []int{72}}, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)); ok {
		t.Fatal("failed history read became a checked answer")
	}
	if got := staleHistoryAnswer(t, root, 72, 1); got != landing[:7] {
		t.Fatalf("transient failure remained cached: %q", got)
	}
}

func TestStaleFailHistoryCopies(t *testing.T) {
	s := &gitReadSession{}
	source := []byte("native history")
	first, err := s.staleFailRead("bound query", func() ([]byte, error) { return source, nil })
	if err != nil {
		t.Fatal(err)
	}
	first[0] = 'X'
	read := func() ([]byte, error) { t.Fatal("successful query reran"); return nil, nil }
	second, err := s.staleFailRead("bound query", read)
	if err != nil || string(second) != "native history" {
		t.Fatalf("first caller corrupted shared data: %q %v", second, err)
	}
	second[0] = 'Y'
	third, err := s.staleFailRead("bound query", read)
	if err != nil || string(third) != "native history" {
		t.Fatalf("cache-hit caller corrupted shared data: %q %v", third, err)
	}
}

func TestStaleFailHistoryOutsideSession(t *testing.T) {
	root, _ := staleHistoryFixture(t)
	marker := staleHistoryLogCounter(t, false)
	previous := activeGitReads
	activeGitReads = nil
	defer func() { activeGitReads = previous }()
	staleHistoryAnswer(t, root, 72, 1)
	first := staleHistoryStarts(t, marker)
	staleHistoryAnswer(t, root, 72, 1)
	if got := staleHistoryStarts(t, marker); got != 2*first {
		t.Fatalf("an unowned invocation reused history: first=%d total=%d", first, got)
	}
}

func TestStaleFailHistoryPhysicalRoot(t *testing.T) {
	root, landing := staleHistoryFixture(t)
	other := t.TempDir()
	if err := os.CopyFS(other, os.DirFS(root)); err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD^1")))
	if err := os.Remove(filepath.Join(other, ".git", "objects", base[:2], base[2:])); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "current")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	end := beginGitReadSession()
	defer end()
	if got := staleHistoryAnswer(t, alias, 72, 1); got != landing[:7] {
		t.Fatalf("initial alias repair=%q", got)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, alias); err != nil {
		t.Fatal(err)
	}
	if got, ok := newestFailFix(alias, &BriefFile{Issues: []int{72}}, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)); ok {
		t.Fatalf("another object database borrowed a successful history: %q", got)
	}
}
