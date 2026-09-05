package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what it printed —
// the dry-run plan goes to stdout, so this is how a test reads it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

// TestDispatchRecordsRunKey pins Layer A of the per-run stop: step 2
// records the run key worktree-locally (`git config --worktree assay.runKey <claim-key>`) in
// the worktree deskwt just created, AFTER the worktree exists (the config write needs the
// tree) and with the CLAIM KEY as the value — the same key deskkit.Guard resolves from cwd to
// find a STOP.run.<key> flag. The worktreeConfig extension is enabled first so the write
// reaches a linked worktree's own config.
func TestDispatchRecordsRunKey(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	// A REAL directory: deskdispatch runs `git config` with the worktree as its cwd, so a
	// fake path would fail to start the child (chdir error) rather than exercise the write.
	home := t.TempDir()
	s.replies = happyReplies(home)

	// A key already in claim-key form (carrying "--") reaches the claim tool unchanged, so
	// the value recorded must be exactly this key.
	const key = "run-key--01"
	promptFile := filepath.Join(t.TempDir(), "p.md")
	if rc := run([]string{key, "--root", root, "--prompt-file", promptFile}); rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0", rc)
	}

	// The extension is enabled and the run key is written into the worktree deskwt reported,
	// with the claim key as the value.
	if !s.ran("git config extensions.worktreeConfig true") {
		t.Error("deskdispatch did not enable extensions.worktreeConfig — the --worktree write would fail in a linked worktree")
	}
	wantWrite := "git config --worktree assay.runKey " + key
	sawWrite, wtIdx, keyIdx := false, -1, -1
	for i, c := range s.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "deskwt add") {
			wtIdx = i
		}
		if strings.Contains(j, wantWrite) {
			sawWrite = true
			keyIdx = i
		}
	}
	if !sawWrite {
		t.Fatalf("deskdispatch never recorded the run key (%q); calls: %v", wantWrite, s.calls)
	}
	if wtIdx < 0 || keyIdx < 0 || keyIdx < wtIdx {
		t.Errorf("the run key was recorded before the worktree existed (deskwt add at %d, config at %d)", wtIdx, keyIdx)
	}
}

// TestDispatchRunKeyFailureNeverFailsTheDispatch: recording the run key is Layer A of two —
// a failure degrades to Layer B (the desk-window sweep) and must NOT fail a dispatch that is
// already claimed and homed. A worktree-config write that errors is reported, not fatal.
func TestDispatchRunKeyFailureNeverFailsTheDispatch(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	home := t.TempDir()
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "deskwt add", stdout: home},
		{match: "config extensions.worktreeConfig", stderr: "error: could not lock config file", code: 1},
	}
	promptFile := filepath.Join(t.TempDir(), "p.md")
	if rc := run([]string{"item-1", "--root", root, "--prompt-file", promptFile}); rc != deskkit.ExitOK {
		t.Fatalf("a run-key recording failure failed the whole dispatch (rc=%d) — Layer A is best-effort", rc)
	}
	// The prompt was still emitted: the dispatch completed.
	if _, err := os.Stat(promptFile); err != nil {
		t.Errorf("no prompt was emitted despite the dispatch succeeding: %v", err)
	}
}

// TestDryRunPrintsTheRunKey: --dry-run touches nothing but must name the key it WOULD record,
// so an operator can see which STOP.run.<key> stops this run before launching it.
func TestDryRunPrintsTheRunKey(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)

	out := captureStdout(t, func() {
		if rc := run([]string{"dry--run--03", "--root", root, "--repo", allowedRepo,
			"--dry-run", "--prompt-file", filepath.Join(t.TempDir(), "p.md")}); rc != deskkit.ExitOK {
			t.Fatalf("dry-run rc = %d, want 0", rc)
		}
	})
	if len(s.calls) != 0 {
		t.Fatalf("--dry-run ran %d child processes: %v", len(s.calls), s.calls)
	}
	if !strings.Contains(out, "run key") || !strings.Contains(out, "dry--run--03") {
		t.Errorf("the dry-run plan did not name the run key it would record:\n%s", out)
	}
}
