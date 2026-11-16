package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// Repository-format tolerance for qualgen's five repository openers (mine,
// check, instructionbrittle, pr, sweep). A repository whose config carries
// core.repositoryformatversion = 0 plus extensions.worktreeConfig = true is
// an ordinary git repository (`git config --worktree` sets exactly that), and
// every opener must read it. go-git's raw opener refuses it ("does not support
// extension: worktreeconfig"); gitcore's opener tolerates that one extension
// and nothing else, so an unknown extension still fails closed.

// extFixture is a mined fixture repository plus a linked worktree of it. The
// store is mined BEFORE the extension is set, so each opener below is the
// first reader to meet the extension.
type extFixture struct {
	main, wt string
	out      string
	store    *Store
}

func newExtFixture(t *testing.T) extFixture {
	t.Helper()
	requireGit(t)
	repo := initFixtureRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	gitCmd(t, repo, "worktree", "add", "-q", "-b", "wt-branch", wt)
	out := t.TempDir()
	if err := mine(repo, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("mine fixture: %v", err)
	}
	return extFixture{main: repo, wt: wt, out: out, store: NewStore(out)}
}

// openErrs runs every qualgen opener against dir and returns one entry per
// opener: the error it reported, or "" when it read the repository.
func openErrs(t *testing.T, f extFixture, dir string) map[string]string {
	t.Helper()
	got := map[string]string{}
	errStr := func(err error) string {
		if err == nil {
			return ""
		}
		return err.Error()
	}
	got["mine"] = errStr(mine(dir, t.TempDir(), &bytes.Buffer{}))
	_, err := screenPaths(dir, f.store, []string{"readme.txt"})
	got["check"] = errStr(err)
	_, err = RunInstructionBrittleness(dir, f.store, InstructionBrittleConfig{Globs: []string{"*.txt"}})
	got["instructionbrittle"] = errStr(err)
	var stdout, stderr bytes.Buffer
	if rc := prMain(prOptions{PR: "1", Out: f.out, Repo: dir, Head: "HEAD", Base: "HEAD~1"}, &stdout, &stderr); rc != 0 {
		got["pr"] = strings.TrimSpace(stderr.String())
	} else {
		got["pr"] = ""
	}
	if sha := targetSHA(dir); strings.HasPrefix(sha, "could-not-measure") {
		got["sweep"] = sha
	} else {
		got["sweep"] = ""
	}
	return got
}

// regression: #6606
func TestOpenersWorktreeConfig(t *testing.T) {
	f := newExtFixture(t)
	// Baseline: extension OFF, every opener reads both checkouts.
	for _, dir := range []string{f.main, f.wt} {
		for opener, e := range openErrs(t, f, dir) {
			if e != "" {
				t.Fatalf("baseline %s on %s: %s", opener, filepath.Base(dir), e)
			}
		}
	}
	gitCmd(t, f.main, "config", "core.repositoryformatversion", "0")
	gitCmd(t, f.main, "config", "extensions.worktreeConfig", "true")
	for _, dir := range []string{f.main, f.wt} {
		for opener, e := range openErrs(t, f, dir) {
			if e != "" {
				t.Errorf("worktreeConfig=true: %s on %s refused: %s", opener, filepath.Base(dir), e)
			}
		}
	}
}

// TestOpenersUnknownExtension is the fail-closed control: tolerating
// worktreeConfig must not make a repository with an extension qualgen cannot
// honour readable. Every opener must still refuse it.
func TestOpenersUnknownExtension(t *testing.T) {
	f := newExtFixture(t)
	gitCmd(t, f.main, "config", "core.repositoryformatversion", "1")
	gitCmd(t, f.main, "config", "extensions.frobnicate", "true")
	for _, dir := range []string{f.main, f.wt} {
		for opener, e := range openErrs(t, f, dir) {
			if e == "" {
				t.Errorf("unknown extension: %s on %s read the repository; it must refuse", opener, filepath.Base(dir))
			}
		}
	}
}
