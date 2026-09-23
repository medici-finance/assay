package main

import (
	"strings"
	"testing"
)

// TestPosixTokenRow is the fail-first fixture windows-port/12's Verify row 7
// names: it plants each of the three POSIX-only literals the row exists to
// catch (a bare `mktemp` mention, a `/tmp/` path, a `~/.config` reference)
// outside any fenced block, and asserts PosixTokenIssues flags every one of
// them — the row fires on a planted token, it is not a lint that only ever
// passes.
func TestPosixTokenRow(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "some-worker", strings.Join([]string{
		"---",
		"name: some-worker",
		"description: x",
		"---",
		"",
		"Mint the file per invocation (`mktemp`) before posting.",
		"",
		"Write the sweep to `> /tmp/actions.json` and read it back.",
		"",
		"The desk tools refuse to run when `~/.config/assay/HEARTBEAT` is stale.",
		"",
	}, "\n"))

	checked, notices, err := PosixTokenIssues(root)
	if err != nil {
		t.Fatalf("PosixTokenIssues: %v", err)
	}
	if checked != 1 {
		t.Fatalf("checked = %d, want 1", checked)
	}
	if len(notices) != 3 {
		t.Fatalf("notices = %d, want 3 (mktemp, /tmp/, ~/.config), got %+v", len(notices), notices)
	}
}

// TestPosixTokenRow_UnixExampleFenceExempt proves the exemption half of the
// same row: a token inside a fenced block whose info string names "unix" is a
// deliberate example, not an accidental POSIX-only literal, and must NOT be
// flagged — otherwise the row would also fire on desk-shell.md's own worked
// examples (a false positive the row must not produce) and a skill author
// would have no way to show a real unix command at all.
func TestPosixTokenRow_UnixExampleFenceExempt(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "some-worker", strings.Join([]string{
		"---",
		"name: some-worker",
		"description: x",
		"---",
		"",
		"See desk-shell.md §Scratch files for the mechanism.",
		"",
		"```bash unix example",
		"BODY=$(mktemp \"${TMPDIR:-/tmp}/pr-body.XXXXXX\")",
		"```",
		"",
	}, "\n"))

	checked, notices, err := PosixTokenIssues(root)
	if err != nil {
		t.Fatalf("PosixTokenIssues: %v", err)
	}
	if checked != 1 {
		t.Fatalf("checked = %d, want 1", checked)
	}
	if len(notices) != 0 {
		t.Fatalf("expected the fenced unix-example block to be exempt, got %+v", notices)
	}
}

// TestPosixTokenRow_CleanSkillNoNotices proves the row is quiet on a skill
// body that already names the mechanism rather than the POSIX literal — the
// exact shape windows-port/12's prose rewrite produces — so the row does not
// false-positive on desk-shell.md cross-references.
func TestPosixTokenRow_CleanSkillNoNotices(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "some-worker", strings.Join([]string{
		"---",
		"name: some-worker",
		"description: x",
		"---",
		"",
		"Mint a per-invocation scratch file (desk-shell.md §Scratch files) before posting.",
		"",
		"The desk tools refuse to run when the HEARTBEAT file in the config home",
		"(desk-shell.md §Config home) is stale.",
		"",
	}, "\n"))

	_, notices, err := PosixTokenIssues(root)
	if err != nil {
		t.Fatalf("PosixTokenIssues: %v", err)
	}
	if len(notices) != 0 {
		t.Fatalf("expected no notices on mechanism-naming prose, got %+v", notices)
	}
}
