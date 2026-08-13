package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func containsSubstr(problems []string, want string) bool {
	for _, p := range problems {
		if strings.Contains(p, want) {
			return true
		}
	}
	return false
}

// ----- duplicate-id checks -----

func TestRegisterIntegrityDuplicateIntake(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)
	writeTemp(t, idir, "2026-07-08-a.md", "---\nid: I-01\ndate: \"2026-07-08\"\ntitle: A\ndisposition: new\n---\n\nBody.")
	writeTemp(t, idir, "2026-07-08-b.md", "---\nid: I-01\ndate: \"2026-07-08\"\ntitle: B\ndisposition: new\n---\n\nBody.")
	problems := registerIntegrityProblems(root)
	if !containsSubstr(problems, "duplicate id I-01") {
		t.Errorf("expected duplicate I-01 problem, got %v", problems)
	}
}

func TestRegisterIntegrityDuplicateFindings(t *testing.T) {
	root := t.TempDir()
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)
	writeTemp(t, fdir, "2026-07-08-a.md", "---\nid: F-01\ndate: \"2026-07-08\"\ntitle: A\naffects: []\nresolved: true\n---\n\nBody.")
	writeTemp(t, fdir, "2026-07-08-b.md", "---\nid: F-01\ndate: \"2026-07-08\"\ntitle: B\naffects: []\nresolved: true\n---\n\nBody.")
	problems := registerIntegrityProblems(root)
	if !containsSubstr(problems, "duplicate id F-01") {
		t.Errorf("expected duplicate F-01 problem, got %v", problems)
	}
}

func TestRegisterIntegrityNoDuplicates(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, idir)
	mustMkdirAll(t, fdir)
	writeTemp(t, idir, "2026-07-08-first.md", "---\nid: I-first-intake-test\ndate: \"2026-07-08\"\ntitle: First\ndisposition: new\n---\n\nBody.")
	writeTemp(t, idir, "2026-07-08-second.md", "---\nid: I-second-intake-test\ndate: \"2026-07-08\"\ntitle: Second\ndisposition: new\n---\n\nBody.")
	writeTemp(t, fdir, "2026-07-08-first.md", "---\nid: F-first-finding-test\ndate: \"2026-07-08\"\ntitle: First\naffects: []\nresolved: true\n---\n\nBody.")
	problems := registerIntegrityProblems(root)
	if len(problems) != 0 {
		t.Errorf("no duplicates, expected 0 problems, got %v", problems)
	}
}

// ----- reserved-directory skip -----

func TestLoadStreamsSkipsReservedDirs(t *testing.T) {
	root := t.TempDir()
	sdir := filepath.Join(root, "docs", "streams")
	// Create a real stream
	mustMkdirAll(t, filepath.Join(sdir, "operator"))
	writeTemp(t, filepath.Join(sdir, "operator"), "README.md", sampleReadme)
	// Create reserved dirs (should be skipped even with READMEs)
	mustMkdirAll(t, filepath.Join(sdir, "intake"))
	mustMkdirAll(t, filepath.Join(sdir, "findings"))
	writeTemp(t, filepath.Join(sdir, "intake"), "README.md", "# Not a stream")
	writeTemp(t, filepath.Join(sdir, "findings"), "README.md", "# Not a stream")

	streams, findings, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 || streams[0].Name != "operator" {
		t.Errorf("expected 1 stream (operator), got %d: %v", len(streams), streams)
	}
	_ = findings
	for _, s := range streams {
		if s.Name == "intake" || s.Name == "findings" {
			t.Errorf("reserved directory %s was treated as a stream", s.Name)
		}
	}
}

// ----- per-entry finding parse -----

func TestParseFindingsFromDir(t *testing.T) {
	root := t.TempDir()
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)
	writeTemp(t, fdir, "2026-07-08-blocker.md", "---\nid: F-01\ndate: \"2026-07-08\"\ntitle: Brief-11 blocker text stale\naffects:\n  - deploy-hardening/brief-01\nresolved: true\n---\n\nBody text.")
	writeTemp(t, fdir, "2026-07-08-something.md", "---\nid: F-02\ndate: \"2026-07-08\"\ntitle: Something open\naffects:\n  - operator\n  - frontend/brief-02\nack: \"2026-07-09 desk\"\nresolved: false\n---\n\nMore body.")

	fs, err := parseFindings(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 2 {
		t.Fatalf("got %d findings, want 2", len(fs))
	}
	if fs[0].ID != "F-01" || !fs[0].Resolved || len(fs[0].Affects) != 1 {
		t.Errorf("F-01 wrong: %+v", fs[0])
	}
	if fs[1].Resolved || len(fs[1].Affects) != 2 || fs[1].Affects[1] != "frontend/brief-02" {
		t.Errorf("F-02 wrong: %+v", fs[1])
	}
	if fs[1].Ack != "2026-07-09 desk" {
		t.Errorf("F-02 Ack wrong: %q", fs[1].Ack)
	}
}

// TestParseFindingsDirCRLFBodyPreserved is the register-level regression test
// for CRLF handling: a CRLF-terminated finding entry file must keep its
// prose body. Under the old byte-prefix splitFrontmatterYAML, the CRLF
// opening fence ("---\r\n") never matched the literal "---\n" prefix check,
// so the whole file (fences included) was returned as "frontmatter" with a
// nil body — the entry parsed "successfully" but its prose silently vanished
// from the generated FINDINGS.md view.
func TestParseFindingsDirCRLFBodyPreserved(t *testing.T) {
	root := t.TempDir()
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)
	crlf := strings.Join([]string{
		"---",
		"id: F-crlf-body-test",
		`date: "2026-07-08"`,
		"title: CRLF entry",
		"affects: []",
		"resolved: false",
		"---",
		"",
		"This body must survive a CRLF-terminated file.",
		"",
	}, "\r\n")
	writeTemp(t, fdir, "2026-07-08-crlf.md", crlf)

	entries, err := parseFindingsDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.ID != "F-crlf-body-test" {
		t.Errorf("ID = %q, want F-crlf-body-test", e.ID)
	}
	const wantBody = "This body must survive a CRLF-terminated file."
	if e.Body != wantBody {
		t.Errorf("Body = %q, want %q — CRLF register entry silently lost its body", e.Body, wantBody)
	}
}

// TestParseIntakeDirCRLFBodyPreserved mirrors
// TestParseFindingsDirCRLFBodyPreserved for the intake register.
func TestParseIntakeDirCRLFBodyPreserved(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)
	crlf := strings.Join([]string{
		"---",
		"id: I-crlf-body-test",
		`date: "2026-07-08"`,
		"title: CRLF intake entry",
		"disposition: new",
		"---",
		"",
		"Prose that must survive a CRLF-terminated intake file.",
		"",
	}, "\r\n")
	writeTemp(t, idir, "2026-07-08-crlf.md", crlf)

	entries, err := parseIntakeDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.ID != "I-crlf-body-test" {
		t.Errorf("ID = %q, want I-crlf-body-test", e.ID)
	}
	const wantBody = "Prose that must survive a CRLF-terminated intake file."
	if e.Body != wantBody {
		t.Errorf("Body = %q, want %q — CRLF register entry silently lost its body", e.Body, wantBody)
	}
}

// TestDispositionKeyCheckCRLFDoesNotMaskMissingKey is the combined regression
// test for the interaction flagged earlier: a CRLF file whose
// real frontmatter genuinely lacks "disposition:" but whose BODY happens to
// contain a line starting "disposition:" (e.g. documenting the field's
// format). Under the old code this was doubly broken: the CRLF opening fence
// defeated splitFrontmatterYAML's byte-prefix check, dumping the whole file
// (frontmatter + body) into what the disposition-key check treated as
// "frontmatter"; its raw substring scan for "\ndisposition:" then matched the
// BODY line and wrongly concluded the key was present, masking the real
// missing-key problem. The fix parses the frontmatter structurally (via the
// shared splitFrontmatter + yaml.Unmarshal into a key set), so body text is
// never in scope for this check regardless of file line endings.
func TestDispositionKeyCheckCRLFDoesNotMaskMissingKey(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)
	crlf := strings.Join([]string{
		"---",
		"id: I-crlf-missing-disposition",
		`date: "2026-07-08"`,
		"title: CRLF entry with no real disposition key",
		"---",
		"",
		"Format note, written as:",
		"",
		"disposition: <value>",
		"",
	}, "\r\n")
	writeTemp(t, idir, "2026-07-08-crlf-missing.md", crlf)

	problems := registerIntegrityProblems(root)
	if !containsSubstr(problems, "missing disposition key") {
		t.Errorf("expected missing disposition key problem for I-crlf-missing-disposition, got %v", problems)
	}
}

func TestParseFindingsMissingDir(t *testing.T) {
	fs, err := parseFindings(t.TempDir())
	if err != nil || fs != nil {
		t.Errorf("missing dir should be nil, nil; got %v, %v", fs, err)
	}
}

// ----- view generation -----

func TestGenerateFindingsView(t *testing.T) {
	root := t.TempDir()
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)
	writeTemp(t, fdir, "2026-07-08-first.md", "---\nid: F-01\ndate: \"2026-07-08\"\ntitle: First finding\naffects:\n  - stream/brief-01\nresolved: true\n---\n\nBody text.\n")

	view, err := generateFindingsView(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(view, "## F-01 — 2026-07-08 — First finding") {
		t.Errorf("view missing F-01 heading: %s", view[:200])
	}
	if !strings.Contains(view, "Body text.") {
		t.Errorf("view missing body text")
	}
	if !strings.Contains(view, "Resolved: yes") {
		t.Errorf("view missing Resolved line")
	}
}

func TestGenerateIntakeView(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)
	writeTemp(t, idir, "2026-07-08-first.md", "---\nid: I-01\ndate: \"2026-07-08\"\ntitle: First idea\ndisposition: new\n---\n\nBody text.")
	writeTemp(t, idir, "2026-07-08-scoped.md", "---\nid: I-02\ndate: \"2026-07-08\"\ntitle: Scoped idea\ndisposition: scoped\nscoped-to: methodology\n---\n\nScoped body.")

	view, err := generateIntakeView(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(view, "## I-01 — 2026-07-08 — First idea") {
		t.Errorf("view missing I-01: %s", view[:200])
	}
	if !strings.Contains(view, "Disposition: new") {
		t.Errorf("view missing Disposition: %s", view[:200])
	}
	if !strings.Contains(view, "Disposition: scoped → methodology") {
		t.Errorf("view missing scoped disposition: %s", view[:200])
	}
}

// ----- deleted-file integrity (Task 6 / Verify 5) -----
// This test simulates a deleted entry file in a git-tracked tree.
// It runs only when the test is executed from within a real git checkout
// (not a tempdir) — deletedRegisterFiles reads git history.

func TestDeletedFileIntegrityInGitFixture(t *testing.T) {
	// This test validates the mechanic by calling deletedRegisterFiles on a
	// real directory known to have git history. In a tempdir the check
	// short-circuits (no .git), returning nil.
	root := t.TempDir()
	// No .git in tempdir → deletedRegisterFiles returns nil
	deleted := deletedRegisterFiles(root)
	if len(deleted) != 0 {
		t.Errorf("tempdir with no .git should return nil, got %v", deleted)
	}
	// The real fixture test requires a git repo; run manually against the
	// actual tree: `go test -run TestDeletedFileInLiveTree` after creating
	// and deleting a fixture entry file in a test branch.
}

// gitRun runs a git command in root for the integrity fixture tests, failing on error.
func gitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	// -c commit.gpgsign=false: the fixture must not depend on the developer's
	// global signing config (and signing is unavailable in the CI sandbox).
	full := append([]string{"-C", root, "-c", "commit.gpgsign=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestDeletedFileIntegrityScopedToMain proves the tombstone check is scoped to
// the HEAD/origin-main merge-base, not the current branch's HEAD: deleting a
// register file that a branch ADDED itself (never landed at the merge-base) must
// NOT fire, while deleting a file that existed at the merge-base MUST fire. This
// is the fix for the false positive that blocked a register PR removing its own
// branch-local duplicate.
func TestDeletedFileIntegrityScopedToMain(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)

	// Commit F-01 on "main", then point origin/main at it.
	writeTemp(t, fdir, "2026-07-08-on-main.md",
		"---\nid: F-01\ndate: \"2026-07-08\"\ntitle: On main\naffects: []\nresolved: true\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "main: add F-01")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Branch-local: add F-02 in a new commit AHEAD of origin/main.
	writeTemp(t, fdir, "2026-07-08-branch-local.md",
		"---\nid: F-02\ndate: \"2026-07-08\"\ntitle: Branch local\naffects: []\nresolved: false\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "branch: add F-02")

	// Case 1: delete the branch-local file (added after the merge-base) → must NOT fire.
	if err := os.Remove(filepath.Join(fdir, "2026-07-08-branch-local.md")); err != nil {
		t.Fatal(err)
	}
	if d := deletedRegisterFiles(root); len(d) != 0 {
		t.Errorf("deleting a branch-local file (not landed at merge-base) must not fire; got %v", d)
	}

	// Case 2: restore F-02, delete F-01 (which existed at the merge-base) → must fire.
	writeTemp(t, fdir, "2026-07-08-branch-local.md",
		"---\nid: F-02\ndate: \"2026-07-08\"\ntitle: Branch local\naffects: []\nresolved: false\n---\n\nBody.")
	if err := os.Remove(filepath.Join(fdir, "2026-07-08-on-main.md")); err != nil {
		t.Fatal(err)
	}
	d := deletedRegisterFiles(root)
	if len(d) == 0 || !strings.Contains(strings.Join(d, " "), "on-main") {
		t.Errorf("deleting a file that existed at the merge-base must fire; got %v", d)
	}
}

// TestDeletedFileIntegrityBranchBehindMain proves a branch simply BEHIND main —
// missing a register file main gained after the merge-base — is not a tombstone
// violation. Only deletion of files landed at the shared merge-base fires.
func TestDeletedFileIntegrityBranchBehindMain(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)

	writeTemp(t, fdir, "2026-07-08-f01.md",
		"---\nid: F-01\ndate: \"2026-07-08\"\ntitle: F01\naffects: []\nresolved: true\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "C1: F-01")

	writeTemp(t, fdir, "2026-07-08-f03.md",
		"---\nid: F-03\ndate: \"2026-07-08\"\ntitle: F03\naffects: []\nresolved: true\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "C2: F-03 on main")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Reset the branch back to C1: now BEHIND main, legitimately lacking F-03
	// (added after the merge-base). Must NOT fire.
	gitRun(t, root, "reset", "--hard", "HEAD~1")
	if d := deletedRegisterFiles(root); len(d) != 0 {
		t.Errorf("a branch behind main (missing a post-merge-base main addition) must not fire; got %v", d)
	}
}

// TestDeletedFileIntegrityNoOriginFallsClosed proves the fail-closed fallback:
// when origin/main can't be resolved (no ref — shallow clone / offline), the check
// falls back to strict HEAD-history behavior, so even a branch-only add-then-delete
// fires. It must NEVER fail open (silently allow a deletion) when it can't find the
// merge-base.
func TestDeletedFileIntegrityNoOriginFallsClosed(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)

	// Add F-01 and commit — NO origin/main ref is ever created.
	writeTemp(t, fdir, "2026-07-08-f01.md",
		"---\nid: F-01\ndate: \"2026-07-08\"\ntitle: F01\naffects: []\nresolved: true\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "add F-01")

	// Delete it. merge-base(HEAD, origin/main) fails (no origin/main) → base=HEAD
	// (strict). F-01 was added in HEAD history → the deletion MUST fire (fail closed).
	if err := os.Remove(filepath.Join(fdir, "2026-07-08-f01.md")); err != nil {
		t.Fatal(err)
	}
	d := deletedRegisterFiles(root)
	if len(d) == 0 || !strings.Contains(strings.Join(d, " "), "f01") {
		t.Errorf("with origin/main unresolvable the check must fail closed (strict HEAD history); got %v", d)
	}
}

// TestDeletedFileIntegrityRename — T8: a landed register file that is RENAMED
// (same ID, new filename) must NOT leave its old path as a permanent false
// tombstone. The old path is tracked by register ID, not by frozen path.
func TestDeletedFileIntegrityRename(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "checkout", "-q", "-b", "base")
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)

	// Land F-01 at path old-slug.md.
	writeTemp(t, fdir, "2026-07-08-old-slug.md",
		"---\nid: F-01\ndate: \"2026-07-08\"\ntitle: Original name\naffects: []\nresolved: true\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "land F-01 at old path")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Rename: delete old-slug.md, create new-slug.md with SAME ID.
	if err := os.Remove(filepath.Join(fdir, "2026-07-08-old-slug.md")); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, fdir, "2026-07-08-new-slug.md",
		"---\nid: F-01\ndate: \"2026-07-08\"\ntitle: Renamed to new slug\naffects: []\nresolved: true\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "rename F-01")

	// T8: the rename must NOT fire as a deletion — the ID F-01 still exists.
	d := deletedRegisterFiles(root)
	if len(d) != 0 {
		t.Errorf("T8: renaming a landed file (same ID at new path) must not fire; got %v", d)
	}

	// Contrast: deleting without replacement still fires.
	if err := os.Remove(filepath.Join(fdir, "2026-07-08-new-slug.md")); err != nil {
		t.Fatal(err)
	}
	d2 := deletedRegisterFiles(root)
	if len(d2) == 0 || !strings.Contains(strings.Join(d2, " "), "old-slug") {
		t.Errorf("deleting without rename replacement must still fire; got %v", d2)
	}
}

// TestDeletedFileIntegrityNonSquashMergeTransient is the regression guard.
// A branch that ADDS a register file and then deletes/renames it before merging is
// a branch-local correction (the documented exemption). A NON-SQUASH merge keeps
// both the add and the delete in main's full history; without --first-parent the
// transient add is enumerated as "landed" and, being absent from the tree, fires a
// spurious tombstone PROBLEM on main. With
// --first-parent the add+delete net to nothing along the merge's first-parent diff,
// so the transient must NOT fire — while deleting a genuinely-landed file still must.
func TestDeletedFileIntegrityNonSquashMergeTransient(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "checkout", "-q", "-b", "base") // deterministic branch name (init default varies)
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)

	// base: land F-01, and point origin/main at it.
	writeTemp(t, fdir, "2026-07-08-on-main.md",
		"---\nid: F-01\ndate: \"2026-07-08\"\ntitle: On main\naffects: []\nresolved: true\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "base: add F-01")

	// feature: add transient F-02, then remove it AND add a persistent entry F-03.
	// This mirrors the real rename: the branch added the new-path (a persistent
	// file) in the SAME directory as the transient old-path it deleted. That persistent
	// add is load-bearing for the test — without it, once F-02 is removed the --no-ff
	// merge is TREESAME to base for docs/streams/findings, so git's default history
	// simplification PRUNES the merge and never visits the transient add commit. Under
	// that pruning even the OLD (pre-fix) code never enumerates F-02, so the test would
	// pass without --first-parent and guard nothing. The persistent F-03 makes the merge
	// non-TREESAME for the directory, forcing the transient add onto the enumerated path
	// under OLD code — so this test now FAILS without the fix and PASSES with it.
	gitRun(t, root, "checkout", "-q", "-b", "feature")
	writeTemp(t, fdir, "2026-07-08-transient.md",
		"---\nid: F-02\ndate: \"2026-07-08\"\ntitle: Transient\naffects: []\nresolved: false\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "feature: add transient F-02")
	if err := os.Remove(filepath.Join(fdir, "2026-07-08-transient.md")); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, fdir, "2026-07-08-persisted.md",
		"---\nid: F-03\ndate: \"2026-07-08\"\ntitle: Persisted (rename target)\naffects: []\nresolved: true\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "feature: drop transient F-02, add persisted F-03 (rename target)")

	// NON-SQUASH merge into base, then point origin/main at the merge commit.
	gitRun(t, root, "checkout", "-q", "base")
	gitRun(t, root, "merge", "-q", "--no-ff", "-m", "Merge feature (non-squash)", "feature")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	// On main (HEAD == origin/main): the transient add+delete must NOT fire.
	if d := deletedRegisterFiles(root); len(d) != 0 {
		t.Errorf("a non-squash-merged branch-local add+delete must NOT fire on main; got %v", d)
	}

	// Contrast: deleting the genuinely-landed F-01 MUST still fire.
	if err := os.Remove(filepath.Join(fdir, "2026-07-08-on-main.md")); err != nil {
		t.Fatal(err)
	}
	d := deletedRegisterFiles(root)
	if len(d) == 0 || !strings.Contains(strings.Join(d, " "), "on-main") {
		t.Errorf("deleting a landed file must still fire after the fix; got %v", d)
	}
}

// ----- decision-needed disposition + decision-issue -----

func TestDecisionNeededParse(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)

	writeTemp(t, idir, "2026-07-15-decision-with-issue.md",
		"---\nid: I-01\ndate: \"2026-07-15\"\ntitle: Strategy call needed\ndisposition: decision-needed\ndecision-issue: \"42\"\n---\n\nWe need to decide on the pricing model.\n")

	writeTemp(t, idir, "2026-07-15-decision-no-issue.md",
		"---\nid: I-02\ndate: \"2026-07-15\"\ntitle: Another decision\ndisposition: decision-needed\n---\n\nAnother call that needs a decision.\n")

	writeTemp(t, idir, "2026-07-15-normal.md",
		"---\nid: I-03\ndate: \"2026-07-15\"\ntitle: Normal idea\ndisposition: new\n---\n\nJust an idea.\n")

	entries, err := parseIntakeDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}

	if entries[0].ID != "I-01" {
		t.Errorf("first entry id = %s, want I-01", entries[0].ID)
	}
	if entries[0].Disposition != "decision-needed" {
		t.Errorf("I-01 disposition = %q, want decision-needed", entries[0].Disposition)
	}
	if entries[0].DecisionIssue != "42" {
		t.Errorf("I-01 decision-issue = %q, want 42", entries[0].DecisionIssue)
	}

	if entries[1].ID != "I-02" {
		t.Errorf("second entry id = %s, want I-02", entries[1].ID)
	}
	if entries[1].Disposition != "decision-needed" {
		t.Errorf("I-02 disposition = %q, want decision-needed", entries[1].Disposition)
	}
	if entries[1].DecisionIssue != "" {
		t.Errorf("I-02 decision-issue = %q, want empty", entries[1].DecisionIssue)
	}

	if entries[2].ID != "I-03" {
		t.Errorf("third entry id = %s, want I-03", entries[2].ID)
	}
	if entries[2].Disposition != "new" {
		t.Errorf("I-03 disposition = %q, want new", entries[2].Disposition)
	}
}

func TestDecisionNeededView(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)

	writeTemp(t, idir, "2026-07-15-decision.md",
		"---\nid: I-01\ndate: \"2026-07-15\"\ntitle: Strategy call\ndisposition: decision-needed\ndecision-issue: \"42\"\n---\n\nNeeds a human call.\n")
	writeTemp(t, idir, "2026-07-15-new.md",
		"---\nid: I-02\ndate: \"2026-07-15\"\ntitle: New idea\ndisposition: new\n---\n\nAn untriaged idea.\n")
	writeTemp(t, idir, "2026-07-15-scoped.md",
		"---\nid: I-03\ndate: \"2026-07-15\"\ntitle: Already scoped\ndisposition: scoped\nscoped-to: something\n---\n\nAlready triaged.\n")

	view, err := generateIntakeView(root)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(view, "## Decision queue — waiting on a human") {
		t.Error("view missing Decision queue section")
	}
	if !strings.Contains(view, "### I-01 — 2026-07-15 — Strategy call") {
		t.Error("I-01 missing from decision section")
	}
	if !strings.Contains(view, "decision-needed → issue #42") {
		t.Errorf("view missing 'decision-needed → issue #42':\n%s", view)
	}
	if !strings.Contains(view, "this section is a pointer into that queue, not a") {
		t.Error("decision section missing pointer language")
	}
	if !strings.Contains(view, "second decision queue") {
		t.Error("decision section missing 'second decision queue'")
	}
	if !strings.Contains(view, "## I-02 — 2026-07-15 — New idea") {
		t.Error("I-02 missing from view")
	}
	if !strings.Contains(view, "## I-03 — 2026-07-15 — Already scoped") {
		t.Error("I-03 missing from view")
	}
	if !strings.Contains(view, "Disposition: scoped → something") {
		t.Error("I-03 scoped disposition not rendered correctly")
	}
}

func TestDecisionNeededViewNoDecisionSection(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)

	writeTemp(t, idir, "2026-07-15-new.md",
		"---\nid: I-01\ndate: \"2026-07-15\"\ntitle: Normal\ndisposition: new\n---\n\nJust an idea.\n")
	writeTemp(t, idir, "2026-07-15-rejected.md",
		"---\nid: I-02\ndate: \"2026-07-15\"\ntitle: Rejected\ndisposition: rejected\nwhy: not now\n---\n\nRejected.\n")

	view, err := generateIntakeView(root)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(view, "Decision queue") {
		t.Error("view has Decision queue section but no decision-needed entries exist")
	}
	if !strings.Contains(view, "## I-01 — 2026-07-15 — Normal") {
		t.Error("I-01 missing from view")
	}
	if !strings.Contains(view, "Disposition: rejected — not now") {
		t.Error("I-02 rejected disposition not rendered correctly")
	}
}

func TestDecisionNeededNotice(t *testing.T) {
	t.Run("decision-needed with no issue — NOTICE fires", func(t *testing.T) {
		entries := []intakeEntry{
			{ID: "I-01", Disposition: "decision-needed", DecisionIssue: ""},
			{ID: "I-02", Disposition: "decision-needed", DecisionIssue: "42"},
			{ID: "I-03", Disposition: "new", DecisionIssue: ""},
		}
		n := intakeDecisionIssueNotices(entries)
		if len(n) != 1 {
			t.Fatalf("got %d notices, want 1", len(n))
		}
		if !strings.Contains(n[0], "I-01") {
			t.Errorf("notice missing I-01: %s", n[0])
		}
		if !strings.Contains(n[0], "decision-needed") {
			t.Errorf("notice missing 'decision-needed': %s", n[0])
		}
		if !strings.Contains(n[0], "decision-issue") {
			t.Errorf("notice missing 'decision-issue': %s", n[0])
		}
		if !strings.Contains(n[0], "frontmatter") {
			t.Errorf("notice missing frontmatter reference: %s", n[0])
		}
	})

	t.Run("all have issues — no NOTICE", func(t *testing.T) {
		entries := []intakeEntry{
			{ID: "I-01", Disposition: "decision-needed", DecisionIssue: "42"},
			{ID: "I-02", Disposition: "decision-needed", DecisionIssue: "43"},
		}
		n := intakeDecisionIssueNotices(entries)
		if len(n) != 0 {
			t.Errorf("got %d notices, want 0: %v", len(n), n)
		}
	})

	t.Run("no decision-needed entries — no NOTICE", func(t *testing.T) {
		entries := []intakeEntry{
			{ID: "I-01", Disposition: "new", DecisionIssue: ""},
			{ID: "I-02", Disposition: "scoped", ScopedTo: "foo", DecisionIssue: ""},
		}
		n := intakeDecisionIssueNotices(entries)
		if len(n) != 0 {
			t.Errorf("got %d notices, want 0: %v", len(n), n)
		}
	})

	t.Run("empty entries — no NOTICE", func(t *testing.T) {
		n := intakeDecisionIssueNotices(nil)
		if len(n) != 0 {
			t.Errorf("nil entries got %d notices, want 0", len(n))
		}
	})
}

func TestDecisionNeededViewDeterministic(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)

	writeTemp(t, idir, "2026-07-15-a.md",
		"---\nid: I-03\ndate: \"2026-07-15\"\ntitle: Third\ndisposition: decision-needed\ndecision-issue: \"99\"\n---\n\nThird call.\n")
	writeTemp(t, idir, "2026-07-15-b.md",
		"---\nid: I-01\ndate: \"2026-07-15\"\ntitle: First\ndisposition: decision-needed\ndecision-issue: \"42\"\n---\n\nFirst call.\n")
	writeTemp(t, idir, "2026-07-15-c.md",
		"---\nid: I-02\ndate: \"2026-07-15\"\ntitle: Second\ndisposition: new\n---\n\nNormal idea.\n")

	view1, err := generateIntakeView(root)
	if err != nil {
		t.Fatal(err)
	}
	view2, err := generateIntakeView(root)
	if err != nil {
		t.Fatal(err)
	}
	if view1 != view2 {
		t.Error("two renders of the same entries produced different output")
	}

	i01pos := strings.Index(view1, "I-01")
	i03pos := strings.Index(view1, "I-03")
	if i01pos == -1 || i03pos == -1 {
		t.Fatal("I-01 or I-03 not found in view")
	}
	if i01pos > i03pos {
		t.Error("I-01 should appear before I-03 (sorted by ID)")
	}
}

// ----- in-place field-gutting guard -----

// gutFixture lands a finding at origin/main, then returns the repo root and the
// finding's on-disk path so a test can mutate it in the working tree and re-check.
func gutFixture(t *testing.T, landed string) (root, findingPath string) {
	t.Helper()
	root = t.TempDir()
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "checkout", "-q", "-b", "base")
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)
	findingPath = filepath.Join(fdir, "2026-07-17-f-gut.md")
	if err := os.WriteFile(findingPath, []byte(landed), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "land finding")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	// Move onto a feature branch AHEAD of origin/main so working-tree edits are
	// measured against the landed merge-base (not HEAD).
	gitRun(t, root, "checkout", "-q", "-b", "feature")
	return root, findingPath
}

const landedOpenFinding = "---\n" +
	"id: F-gut\n" +
	"date: \"2026-07-17\"\n" +
	"title: Register guard gap\n" +
	"affects: [\"stream-y\", \"stream-z/brief-01\"]\n" +
	"resolved: false\n" +
	"---\n\nBody.\n"

// landedAckedFinding is the same entry carrying a well-formed desk ack — the
// third load-bearing field check() reads (checks.go: with a matching ack an
// unresolved finding on an in-progress/implemented brief is a hard PROBLEM;
// without one it degrades to an "awaits desk ack" NOTICE).
const landedAckedFinding = "---\n" +
	"id: F-gut\n" +
	"date: \"2026-07-17\"\n" +
	"title: Register guard gap\n" +
	"affects: [\"stream-y\", \"stream-z/brief-01\"]\n" +
	"ack: \"2026-07-18 desk\"\n" +
	"resolved: false\n" +
	"---\n\nBody.\n"

// TestGuttedResolvedFlipUnauthorized: flipping resolved no->yes with no verified-
// human anchor is a HARD lint problem.
func TestGuttedResolvedFlipUnauthorized(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	gutted := strings.Replace(landedOpenFinding, "resolved: false", "resolved: true", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	p := guttedRegisterFields(root)
	if !containsSubstr(p, "field-gutting (unauthorized)") || !containsSubstr(p, "resolved flipped no->yes") {
		t.Fatalf("resolved no->yes without human auth must fire; got %v", p)
	}
}

// TestGuttedResolvedFlipAuthorized: the same flip WITH a verified-human anchor
// under the designated frontmatter key (authorized-by: human:alex, mapped in
// HumanLoginMap) passes.
func TestGuttedResolvedFlipAuthorized(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	gutted := strings.Replace(landedOpenFinding, "resolved: false", "resolved: true\nauthorized-by: human:alex", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	if p := guttedRegisterFields(root); len(p) != 0 {
		t.Fatalf("resolved flip WITH authorized-by: human:alex must pass; got %v", p)
	}
}

// TestGuttedResolvedFlipUnknownNameNotAuthorized: a human:<name> whose name is not
// in HumanLoginMap is an agent-written token, not a verified human — still fires.
func TestGuttedResolvedFlipUnknownNameNotAuthorized(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	gutted := strings.Replace(landedOpenFinding, "resolved: false", "resolved: true\nauthorized-by: human:bot", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	if p := guttedRegisterFields(root); !containsSubstr(p, "field-gutting (unauthorized)") {
		t.Fatalf("unknown human name must NOT authorize; got %v", p)
	}
}

// TestGuttedAffectsEmptied: emptying affects unblocks every brief it demoted — a
// HARD problem without human auth.
func TestGuttedAffectsEmptied(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	gutted := strings.Replace(landedOpenFinding,
		"affects: [\"stream-y\", \"stream-z/brief-01\"]", "affects: []", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	p := guttedRegisterFields(root)
	if !containsSubstr(p, "field-gutting (unauthorized)") || !containsSubstr(p, "affects dropped") {
		t.Fatalf("emptied affects without human auth must fire; got %v", p)
	}
}

// TestGuttedAffectsNarrowed: dropping one of several affects entries (partial
// gutting) fires and names the dropped entry.
func TestGuttedAffectsNarrowed(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	gutted := strings.Replace(landedOpenFinding,
		"affects: [\"stream-y\", \"stream-z/brief-01\"]", "affects: [\"stream-y\"]", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	p := guttedRegisterFields(root)
	if !containsSubstr(p, "affects dropped") || !containsSubstr(p, "stream-z/brief-01") {
		t.Fatalf("narrowed affects must fire naming the dropped entry; got %v", p)
	}
}

// TestGuttedAckRemoved closes BLOCKER 2 of the security review: `ack` is
// the THIRD load-bearing field check() reads. Deleting the ack: line downgrades
// checks.go's hard "unresolved F-xx (desk-acked) — demote to todo (re-gate) or
// resolve the finding" PROBLEM to a mere "awaits desk ack" NOTICE, turning a red
// check green while resolved:/affects: sit untouched. It is caught nowhere else:
// the malformed-ack check is guarded by `f.Ack != ""`, so CORRUPTING ack errors
// but REMOVING it is silent.
func TestGuttedAckRemoved(t *testing.T) {
	root, path := gutFixture(t, landedAckedFinding)
	gutted := strings.Replace(landedAckedFinding, "ack: \"2026-07-18 desk\"\n", "", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	p := guttedRegisterFields(root)
	if !containsSubstr(p, "field-gutting (unauthorized)") || !containsSubstr(p, "ack removed") {
		t.Fatalf("removing a landed ack must fire; got %v", p)
	}
}

// TestGuttedAckRemovedDowngradesCheckSeverity is the motivation test for the
// clause above: it proves against the REAL check() that dropping ack converts a
// hard problem into a notice. If this ever stops holding, the ack clause in
// guttedRegisterFields is guarding something that no longer gates.
func TestGuttedAckRemovedDowngradesCheckSeverity(t *testing.T) {
	mk := func() []*Stream {
		return []*Stream{{
			Name:     "stream-z",
			Dir:      "stream-z",
			Status:   "active",
			Priority: "P1",
			Track:    "product",
			Briefs:   []Brief{{Num: "01", Status: "implemented"}},
		}}
	}
	acked := []Finding{{ID: "F-gut", Affects: []string{"stream-z/brief-01"}, Ack: "2026-07-18 desk"}}
	noAck := []Finding{{ID: "F-gut", Affects: []string{"stream-z/brief-01"}}}

	pAck, nAck := check(mk(), acked)
	if !containsSubstr(pAck, "unresolved F-gut (desk-acked)") {
		t.Fatalf("with ack, check() must emit the hard re-gate PROBLEM; got problems=%v notices=%v", pAck, nAck)
	}
	pNo, nNo := check(mk(), noAck)
	if containsSubstr(pNo, "unresolved F-gut (desk-acked)") {
		t.Fatalf("premise broken: without ack the hard PROBLEM should be gone; got %v", pNo)
	}
	if !containsSubstr(nNo, "unresolved F-gut awaits desk ack") {
		t.Fatalf("without ack, check() must degrade to the awaits-ack NOTICE; got notices=%v", nNo)
	}
}

// TestGuttedAckAddedNotFlagged: ADDING an ack adds caution — never gutting.
func TestGuttedAckAddedNotFlagged(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	widened := strings.Replace(landedOpenFinding, "resolved: false",
		"ack: \"2026-07-18 desk\"\nresolved: false", 1)
	if err := os.WriteFile(path, []byte(widened), 0o644); err != nil {
		t.Fatal(err)
	}
	if p := guttedRegisterFields(root); len(p) != 0 {
		t.Fatalf("adding an ack must not fire; got %v", p)
	}
}

// TestGuttedWideningAffectsNotFlagged: WIDENING affects or re-opening a finding
// adds caution — never gutting, never flagged.
func TestGuttedWideningAffectsNotFlagged(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	widened := strings.Replace(landedOpenFinding,
		"affects: [\"stream-y\", \"stream-z/brief-01\"]",
		"affects: [\"stream-y\", \"stream-z/brief-01\", \"stream-w\"]", 1)
	if err := os.WriteFile(path, []byte(widened), 0o644); err != nil {
		t.Fatal(err)
	}
	if p := guttedRegisterFields(root); len(p) != 0 {
		t.Fatalf("widening affects must not fire; got %v", p)
	}
}

// TestGuttedResolvedReopenNotFlagged: yes->no (re-opening) is the opposite of
// gutting and must not fire. The landed entry here is resolved: true.
func TestGuttedResolvedReopenNotFlagged(t *testing.T) {
	landedResolved := strings.Replace(landedOpenFinding, "resolved: false", "resolved: true", 1)
	root, path := gutFixture(t, landedResolved)
	reopened := strings.Replace(landedResolved, "resolved: true", "resolved: false", 1)
	if err := os.WriteFile(path, []byte(reopened), 0o644); err != nil {
		t.Fatal(err)
	}
	if p := guttedRegisterFields(root); len(p) != 0 {
		t.Fatalf("re-opening a resolved finding must not fire; got %v", p)
	}
}

// TestGuttedNewBranchLocalFindingNotFlagged: an entry ADDED on the branch (not
// present at the merge-base) is not a mutation of landed history — skipped even
// when it is resolved: true from the start.
//
// The `resolved: true` in the fixture is load-bearing, and is a deliberate
// divergence from the upstream repo's version of this test. That one wrote a branch-local
// entry that was resolved: FALSE, so the absent-at-base skip it claims to cover
// was never exercised: with the skip removed, the missing base parses to a
// zero-value entry (resolved false, affects nil) and a resolved:false branch-local
// copy diffs to nothing, so the test passed either way. Mutation-tested: deleting
// the skip in guttedRegisterFields must turn this test RED.
func TestGuttedNewBranchLocalFindingNotFlagged(t *testing.T) {
	root, _ := gutFixture(t, landedOpenFinding)
	fdir := filepath.Join(root, "docs", "streams", "findings")
	newPath := filepath.Join(fdir, "2026-07-18-branch-local.md")
	branchLocal := strings.Replace(landedOpenFinding, "id: F-gut", "id: F-branch-local", 1)
	branchLocal = strings.Replace(branchLocal, "resolved: false", "resolved: true", 1)
	if err := os.WriteFile(newPath, []byte(branchLocal), 0o644); err != nil {
		t.Fatal(err)
	}
	if p := guttedRegisterFields(root); len(p) != 0 {
		t.Fatalf("a branch-local new finding must not be flagged as gutting; got %v", p)
	}
}

// TestGuttedDeletionPathUnchanged: deleting a landed entry stays owned by
// deletedRegisterFiles (tombstone-not-delete); the gutting guard must NOT
// double-report it (it cannot read a deleted file).
func TestGuttedDeletionPathUnchanged(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if p := guttedRegisterFields(root); len(p) != 0 {
		t.Fatalf("gutting guard must not report a deletion; got %v", p)
	}
	if d := deletedRegisterFiles(root); len(d) == 0 || !containsSubstr(d, "f-gut") {
		t.Fatalf("deletion path must still catch the removed entry; got %v", d)
	}
	// The guard's silence here is defended twice over, and mutation testing
	// confirms EITHER guard alone still holds the property: (1) the loop
	// enumerates the WORKING TREE (os.ReadDir), so a deleted entry never enters
	// it, and (2) the os.ReadFile skip catches it if enumeration ever grows a
	// base-side source. This test goes RED only when BOTH are removed — at which
	// point every deletion double-reports as `affects dropped [...]` on top of
	// the tombstone problem.
}

// TestGuttedNoGitFixtureReturnsNil: outside a git checkout the guard is inert.
func TestGuttedNoGitFixtureReturnsNil(t *testing.T) {
	if p := guttedRegisterFields(t.TempDir()); p != nil {
		t.Fatalf("no .git => nil; got %v", p)
	}
}

// ----- BLOCKER 1: the authorization anchor is frontmatter-scoped -----

// bodyHumanStampFinding reproduces the shape of the real upstream finding that made
// BLOCKER 1 live: docs/streams/findings/2026-07-17-human-stamps-never-corroborated-
// by-ci.md is an OPEN finding demoting a live stream whose PROSE says human:alex
// (twice) while describing this very weakness. With the upstream repo's whole-file scan, that
// entry was permanently exempt from the guard. assay carries no per-entry
// findings register of its own, so the fixture is constructed with the same shape.
const bodyHumanStampFinding = "---\n" +
	"id: F-gut\n" +
	"date: \"2026-07-17\"\n" +
	"title: human stamps are never corroborated by CI\n" +
	"affects: [\"stream-y\", \"stream-z/brief-01\"]\n" +
	"resolved: false\n" +
	"---\n\n" +
	"statusgen accepts a `human:alex` anchor offline with no online corroboration.\n" +
	"Nothing in CI proves that human:alex actually acted; --corroborate is manual.\n"

// TestGuttedBodyHumanStampDoesNotAuthorize is the BLOCKER 1 regression test: a
// finding whose BODY mentions human:alex must NOT be authorized. The upstream repo's
// authorizedByVerifiedHuman ran humanStampRe over the entire raw file, so this
// gutting produced zero problems.
func TestGuttedBodyHumanStampDoesNotAuthorize(t *testing.T) {
	root, path := gutFixture(t, bodyHumanStampFinding)
	gutted := strings.Replace(bodyHumanStampFinding, "resolved: false", "resolved: true", 1)
	gutted = strings.Replace(gutted,
		"affects: [\"stream-y\", \"stream-z/brief-01\"]", "affects: []", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	p := guttedRegisterFields(root)
	if !containsSubstr(p, "field-gutting (unauthorized)") {
		t.Fatalf("prose mentioning human:alex must NOT authorize gutting; got %v", p)
	}
}

// TestGuttedRenameAndMutate — T8: a landed finding renamed AND gutted in the
// same commit must still fire. Before the T8 fix, git show base:new-path would
// fail (the base tree only knows the old path) and the entry would be silently
// skipped — a one-commit bypass of the field-gutting gate (S4/S5 in
// an independent review). The fix resolves the landed counterpart by register
// ID, so a rename+gut is still diffed against the original landed version.
func TestGuttedRenameAndMutate(t *testing.T) {
	root, oldPath := gutFixture(t, landedOpenFinding)
	fdir := filepath.Dir(oldPath)

	// S4: rename the file AND flip resolved no->yes in the same commit.
	newPath := filepath.Join(fdir, "2026-07-17-f-gut-renamed.md")
	gutted := strings.Replace(landedOpenFinding, "resolved: false", "resolved: true", 1)
	if err := os.Remove(oldPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}

	// S4 must fire: the rename+gut bypass is closed.
	p := guttedRegisterFields(root)
	if !containsSubstr(p, "field-gutting (unauthorized)") || !containsSubstr(p, "resolved flipped no->yes") {
		t.Fatalf("T8 S4: rename+gut must fire as unauthorized gutting; got %v", p)
	}
}

// TestAuthorizedByVerifiedHumanScope pins the anchor's scope directly: only the
// designated frontmatter key authorizes.
func TestAuthorizedByVerifiedHumanScope(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"designated frontmatter key", "---\nid: F-a\nauthorized-by: human:alex\n---\n\nBody.\n", true},
		{"designated key, quoted", "---\nid: F-a\nauthorized-by: \"human:alex\"\n---\n\nBody.\n", true},
		{"designated key, name uppercased", "---\nid: F-a\nauthorized-by: human:ALEX\n---\n\nBody.\n", true},
		{"designated key, unknown name", "---\nid: F-a\nauthorized-by: human:bot\n---\n\nBody.\n", false},
		{"body prose only", "---\nid: F-a\n---\n\nSee human:alex for context.\n", false},
		{"other frontmatter key", "---\nid: F-a\ntitle: ask human:alex\n---\n\nBody.\n", false},
		{"frontmatter comment", "---\nid: F-a\n# human:alex said ok\n---\n\nBody.\n", false},
		{"no anchor at all", "---\nid: F-a\n---\n\nBody.\n", false},
		{"no frontmatter, prose anchor", "Just prose with human:alex in it.\n", false},
		{"unparseable frontmatter fails closed", "---\nid: [F-a\nauthorized-by: human:alex\n---\n\nBody.\n", false},

		// ---- this is where the unanchored regex was an
		// actual FORGEABLE AUTHORIZATION, not just a parsing curiosity. The
		// `authorized-by:` key gates in-place gutting of a finding's load-bearing
		// fields — the check that stops a demoted brief being silently unblocked.
		// With `human:(\w+)` unanchored, every one of these authorized the gut,
		// because `superhuman:alex` matched with name "alex", which resolves in
		// HumanLoginMap. Each is paired above/below with the genuine token it
		// imitates, which must keep working.
		{"superhuman prefix does not authorize", "---\nid: F-a\nauthorized-by: superhuman:alex\n---\n\nBody.\n", false},
		{"non-human prefix does not authorize", "---\nid: F-a\nauthorized-by: non-human:alex\n---\n\nBody.\n", false},
		{"inhuman prefix does not authorize", "---\nid: F-a\nauthorized-by: inhuman:alex\n---\n\nBody.\n", false},
		{"subhuman prefix does not authorize", "---\nid: F-a\nauthorized-by: sub-human:alex\n---\n\nBody.\n", false},
		{"confusable name does not authorize", "---\nid: F-a\nauthorized-by: human:іan\n---\n\nBody.\n", false},
		// ...and the genuine token still does, in the surroundings people write.
		{"genuine token in parens still authorizes", "---\nid: F-a\nauthorized-by: (human:alex)\n---\n\nBody.\n", true},
		{"genuine token with a note still authorizes", "---\nid: F-a\nauthorized-by: human:alex on 2026-08-01\n---\n\nBody.\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := authorizedByVerifiedHuman([]byte(tc.raw)); got != tc.want {
				t.Fatalf("authorizedByVerifiedHuman = %v, want %v for:\n%s", got, tc.want, tc.raw)
			}
		})
	}
}

// ----- base=HEAD fail-open NOTICE -----

// TestRegisterBaseFallbackNoticeFires: with origin/main unresolvable the guard
// runs degraded (committed gutting compares against itself) — say so.
func TestRegisterBaseFallbackNoticeFires(t *testing.T) {
	root, _ := gutFixture(t, landedOpenFinding)
	gitRun(t, root, "update-ref", "-d", "refs/remotes/origin/main")
	n := registerBaseFallbackNotices(root)
	if !containsSubstr(n, "running degraded") {
		t.Fatalf("unresolvable origin/main must emit the degraded NOTICE; got %v", n)
	}
	// And prove the fail-open it warns about is real: commit the gutting, and
	// the guard sees nothing.
	path := filepath.Join(root, "docs", "streams", "findings", "2026-07-17-f-gut.md")
	if err := os.WriteFile(path, []byte(strings.Replace(landedOpenFinding,
		"resolved: false", "resolved: true", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "gut it")
	if p := guttedRegisterFields(root); len(p) != 0 {
		t.Fatalf("premise broken: committed gutting under base=HEAD should be invisible; got %v", p)
	}
}

// TestRegisterBaseFallbackNoticeSilentWhenResolved: the healthy path says nothing.
func TestRegisterBaseFallbackNoticeSilentWhenResolved(t *testing.T) {
	root, _ := gutFixture(t, landedOpenFinding)
	if n := registerBaseFallbackNotices(root); len(n) != 0 {
		t.Fatalf("resolvable origin/main must be silent; got %v", n)
	}
}

// TestRegisterBaseFallbackNoticeSilentWithNoFindings: nothing to guard, nothing
// to say — a repo with no per-entry findings register (this one) stays quiet.
func TestRegisterBaseFallbackNoticeSilentWithNoFindings(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	mustMkdirAll(t, filepath.Join(root, "docs", "streams"))
	if n := registerBaseFallbackNotices(root); len(n) != 0 {
		t.Fatalf("no findings register => silent; got %v", n)
	}
	if n := registerBaseFallbackNotices(t.TempDir()); len(n) != 0 {
		t.Fatalf("no .git, no findings => silent; got %v", n)
	}
}

// TestRegisterBaseFallbackNoticeFiresWithNoGitAtAll: a tree with
// NO .git directory at all — a `git archive` export — must ALSO get the
// degraded NOTICE when there are findings to guard, not silence. Before this
// fix, the notice function's own .git-absence early-return suppressed it
// specifically in this case, even though guttedRegisterFields also skipped
// the guard outright: the check silently did not run, and nothing said so.
func TestRegisterBaseFallbackNoticeFiresWithNoGitAtAll(t *testing.T) {
	root := t.TempDir()
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)
	if err := os.WriteFile(filepath.Join(fdir, "2026-07-17-f-gut.md"), []byte(landedOpenFinding), 0o644); err != nil {
		t.Fatal(err)
	}
	// No git init at all — this directory is a plain extracted tree.

	n := registerBaseFallbackNotices(root)
	if !containsSubstr(n, "no .git directory") {
		t.Fatalf("a tree with no .git directory and findings to guard must emit a NOTICE naming that cause; got %v", n)
	}
	// And the guard really did skip outright (distinct from the base=HEAD
	// fail-open, which still compares — just against itself).
	if p := guttedRegisterFields(root); len(p) != 0 {
		t.Fatalf("guttedRegisterFields must skip outright with no .git directory; got %v", p)
	}
}
