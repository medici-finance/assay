package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ----- issue-loop/15: intake directory split by disposition -----
//
// Fixtures cover: split layout, flat layout (compat), mixed (root + subdirs
// together), a moved entry (id-identity, not path-identity), a truly deleted
// entry (real removal still fires), and a copy-without-delete (duplicate id
// closes the hole a naive id-only relaxation would open).

// ----- parseIntakeDir: subdir-aware parsing -----

func TestParseIntakeFlat(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)
	writeTemp(t, idir, "2026-07-08-a.md",
		"---\nid: I-flat-a\ndate: \"2026-07-08\"\ntitle: A\ndisposition: new\n---\n\nBody.")

	entries, err := parseIntakeDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Subdir != "" {
		t.Errorf("root-level entry Subdir = %q, want empty (flat-layout compat)", entries[0].Subdir)
	}
}

func TestParseIntakeDirSplitLayout(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	for _, sub := range []string{"new", "decision-needed", "watching", "completed", "rejected"} {
		mustMkdirAll(t, filepath.Join(idir, sub))
	}
	writeTemp(t, filepath.Join(idir, "new"), "2026-07-08-a.md",
		"---\nid: I-split-new\ndate: \"2026-07-08\"\ntitle: New\ndisposition: new\n---\n\nBody.")
	writeTemp(t, filepath.Join(idir, "decision-needed"), "2026-07-08-b.md",
		"---\nid: I-split-decision\ndate: \"2026-07-08\"\ntitle: Decision\ndisposition: decision-needed\n---\n\nBody.")
	writeTemp(t, filepath.Join(idir, "watching"), "2026-07-08-c.md",
		"---\nid: I-split-watching\ndate: \"2026-07-08\"\ntitle: Watching\ndisposition: watching\n---\n\nBody.")
	writeTemp(t, filepath.Join(idir, "completed"), "2026-07-08-d.md",
		"---\nid: I-split-completed\ndate: \"2026-07-08\"\ntitle: Completed\ndisposition: scoped\nscoped-to: somestream\n---\n\nBody.")
	writeTemp(t, filepath.Join(idir, "rejected"), "2026-07-08-e.md",
		"---\nid: I-split-rejected\ndate: \"2026-07-08\"\ntitle: Rejected\ndisposition: rejected\nwhy: not now\n---\n\nBody.")
	// README.md inside a subdir must be skipped, same as at the root.
	writeTemp(t, filepath.Join(idir, "new"), "README.md", "# new/ — untriaged entries\n")

	entries, err := parseIntakeDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("got %d entries, want 5 (README.md must be skipped): %+v", len(entries), entries)
	}
	got := map[string]string{}
	for _, e := range entries {
		got[e.ID] = e.Subdir
	}
	want := map[string]string{
		"I-split-new":       "new",
		"I-split-decision":  "decision-needed",
		"I-split-watching":  "watching",
		"I-split-completed": "completed",
		"I-split-rejected":  "rejected",
	}
	for id, subdir := range want {
		if got[id] != subdir {
			t.Errorf("%s: Subdir = %q, want %q", id, got[id], subdir)
		}
	}
}

func TestParseIntakeDirMixedLayout(t *testing.T) {
	// Root-level files (not-yet-migrated) coexist with split subdirs — both
	// must parse; this is the normal state DURING a migration.
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, filepath.Join(idir, "new"))
	writeTemp(t, idir, "2026-07-08-root.md",
		"---\nid: I-mixed-root\ndate: \"2026-07-08\"\ntitle: Root\ndisposition: new\n---\n\nBody.")
	writeTemp(t, filepath.Join(idir, "new"), "2026-07-08-split.md",
		"---\nid: I-mixed-split\ndate: \"2026-07-08\"\ntitle: Split\ndisposition: new\n---\n\nBody.")

	entries, err := parseIntakeDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	for _, e := range entries {
		switch e.ID {
		case "I-mixed-root":
			if e.Subdir != "" {
				t.Errorf("I-mixed-root Subdir = %q, want empty", e.Subdir)
			}
		case "I-mixed-split":
			if e.Subdir != "new" {
				t.Errorf("I-mixed-split Subdir = %q, want new", e.Subdir)
			}
		default:
			t.Errorf("unexpected entry %s", e.ID)
		}
	}
}

func TestParseIntakeUnknownSkip(t *testing.T) {
	// An unknown-named directory under intake/ is not silently walked — its
	// entries do not appear in parseIntakeDir's output (registerIntegrityProblems
	// flags the directory itself separately; see TestUnknownSubdirProblem).
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, filepath.Join(idir, "archive"))
	writeTemp(t, filepath.Join(idir, "archive"), "2026-07-08-x.md",
		"---\nid: I-in-unknown-dir\ndate: \"2026-07-08\"\ntitle: X\ndisposition: rejected\n---\n\nBody.")

	entries, err := parseIntakeDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (unknown subdir not descended), got %+v", entries)
	}
}

// ----- registerIntegrityProblems: dir↔disposition mismatch -----

func TestDirDispoMismatch(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, filepath.Join(idir, "completed"))
	// disposition: new sitting in completed/ — a mismatch.
	writeTemp(t, filepath.Join(idir, "completed"), "2026-07-08-x.md",
		"---\nid: I-mismatch\ndate: \"2026-07-08\"\ntitle: Mismatch\ndisposition: new\n---\n\nBody.")

	problems := registerIntegrityProblems(root)
	if !containsSubstr(problems, "dir↔disposition mismatch") || !containsSubstr(problems, "I-mismatch") {
		t.Errorf("expected dir↔disposition mismatch problem for I-mismatch, got %v", problems)
	}
}

func TestDirDispoMatchOK(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, filepath.Join(idir, "new"))
	writeTemp(t, filepath.Join(idir, "new"), "2026-07-08-x.md",
		"---\nid: I-match\ndate: \"2026-07-08\"\ntitle: Match\ndisposition: new\n---\n\nBody.")

	problems := registerIntegrityProblems(root)
	if containsSubstr(problems, "dir↔disposition mismatch") {
		t.Errorf("matching dir/disposition must not fire a mismatch problem, got %v", problems)
	}
}

func TestDirDispoRootExempt(t *testing.T) {
	// A root-level (flat-layout) file makes no placement claim — never a
	// mismatch, regardless of its disposition value.
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)
	writeTemp(t, idir, "2026-07-08-x.md",
		"---\nid: I-root-exempt\ndate: \"2026-07-08\"\ntitle: Root\ndisposition: rejected\nwhy: no\n---\n\nBody.")

	problems := registerIntegrityProblems(root)
	if containsSubstr(problems, "dir↔disposition mismatch") {
		t.Errorf("root-level file must never fire a dir↔disposition mismatch, got %v", problems)
	}
}

// ----- registerIntegrityProblems: unknown subdir -----

func TestUnknownSubdirProblem(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, filepath.Join(idir, "archive"))
	writeTemp(t, filepath.Join(idir, "archive"), "2026-07-08-x.md",
		"---\nid: I-unknown-subdir\ndate: \"2026-07-08\"\ntitle: X\ndisposition: rejected\n---\n\nBody.")

	problems := registerIntegrityProblems(root)
	if !containsSubstr(problems, "unknown subdir intake/archive/") {
		t.Errorf("expected unknown-subdir problem for intake/archive/, got %v", problems)
	}
}

func TestKnownSubdirsClean(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	for _, sub := range intakeKnownSubdirs {
		mustMkdirAll(t, filepath.Join(idir, sub))
	}
	problems := registerIntegrityProblems(root)
	if containsSubstr(problems, "unknown subdir") {
		t.Errorf("all-known subdirs must not fire unknown-subdir problem, got %v", problems)
	}
}

// ----- intakeRootFileNotices: root file coexisting with split layout -----

func TestRootFileNoticeFires(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, filepath.Join(idir, "new"))
	writeTemp(t, idir, "2026-07-08-root.md",
		"---\nid: I-notice-root\ndate: \"2026-07-08\"\ntitle: Root\ndisposition: new\n---\n\nBody.")

	entries, err := parseIntakeDir(root)
	if err != nil {
		t.Fatal(err)
	}
	notices := intakeRootFileNotices(root, entries)
	if !containsSubstr(notices, "I-notice-root") {
		t.Errorf("expected root-file notice for I-notice-root, got %v", notices)
	}
}

func TestRootFileNoticeSilent(t *testing.T) {
	// No subdirs exist at all — a flat-only repo (or this repo's own INTAKE.md
	// mechanism, which never populates docs/streams/intake/ subdirs) must not
	// be flagged; there is no split layout to be inconsistent with.
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)
	writeTemp(t, idir, "2026-07-08-root.md",
		"---\nid: I-notice-silent\ndate: \"2026-07-08\"\ntitle: Root\ndisposition: new\n---\n\nBody.")

	entries, err := parseIntakeDir(root)
	if err != nil {
		t.Fatal(err)
	}
	notices := intakeRootFileNotices(root, entries)
	if len(notices) != 0 {
		t.Errorf("no split layout in use — expected 0 notices, got %v", notices)
	}
}

// ----- deletedRegisterFiles: id-identity across the intake split -----

// TestDeletedIntakeMoveRootSub proves a git mv from the
// flat root into a disposition subdir is a TRANSITION, not a delete — the
// core contract of issue-loop/15.
func TestDeletedIntakeMoveRootSub(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "checkout", "-q", "-b", "base")
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)

	writeTemp(t, idir, "2026-07-08-old.md",
		"---\nid: I-moved\ndate: \"2026-07-08\"\ntitle: Moved\ndisposition: new\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "land I-moved at root")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	// git mv into new/ — same id, different path, disposition unchanged.
	mustMkdirAll(t, filepath.Join(idir, "new"))
	if err := os.Rename(
		filepath.Join(idir, "2026-07-08-old.md"),
		filepath.Join(idir, "new", "2026-07-08-old.md"),
	); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "triage: move I-moved into new/")

	d := deletedRegisterFiles(root)
	if len(d) != 0 {
		t.Errorf("a move within the intake register dir (root -> subdir) must not fire tombstone-not-delete; got %v", d)
	}
}

// TestDeletedIntakeMoveSubSub proves the same for a
// transition between two subdirs (the normal steady-state triage-verb move).
func TestDeletedIntakeMoveSubSub(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "checkout", "-q", "-b", "base")
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, filepath.Join(idir, "new"))

	writeTemp(t, filepath.Join(idir, "new"), "2026-07-08-old.md",
		"---\nid: I-transition\ndate: \"2026-07-08\"\ntitle: Transition\ndisposition: new\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "land I-transition in new/")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Triage verb: frontmatter flip to watching + git mv new/ -> watching/, one commit.
	mustMkdirAll(t, filepath.Join(idir, "watching"))
	if err := os.Remove(filepath.Join(idir, "new", "2026-07-08-old.md")); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, filepath.Join(idir, "watching"), "2026-07-08-old.md",
		"---\nid: I-transition\ndate: \"2026-07-08\"\ntitle: Transition\ndisposition: watching\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "triage: watch I-transition")

	d := deletedRegisterFiles(root)
	if len(d) != 0 {
		t.Errorf("a triage-verb move between two subdirs must not fire tombstone-not-delete; got %v", d)
	}
}

// TestDeletedIntakeCrossRegFires proves the
// id-identity relaxation is scoped to the SAME register dir: moving an
// intake entry's id to the FINDINGS register (a different register
// entirely) must still fire — the relaxation must not disguise a real
// delete-from-intake as a move just because some file somewhere carries the
// same id token.
func TestDeletedIntakeCrossRegFires(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "checkout", "-q", "-b", "base")
	idir := filepath.Join(root, "docs", "streams", "intake")
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, idir)
	mustMkdirAll(t, fdir)

	writeTemp(t, idir, "2026-07-08-old.md",
		"---\nid: I-cross-register\ndate: \"2026-07-08\"\ntitle: Cross register\ndisposition: new\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "land I-cross-register in intake")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Delete from intake; "recreate" the same id token under findings/ — a
	// different register dir. This must NOT be recognized as a move.
	if err := os.Remove(filepath.Join(idir, "2026-07-08-old.md")); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, fdir, "2026-07-08-copy.md",
		"---\nid: I-cross-register\ndate: \"2026-07-08\"\ntitle: Copied into findings\naffects: []\nresolved: true\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "delete from intake, plant same id under findings")

	d := deletedRegisterFiles(root)
	if len(d) == 0 || !strings.Contains(strings.Join(d, " "), "old.md") {
		t.Errorf("a same-id file resurfacing in a DIFFERENT register dir must not exempt the intake deletion; got %v", d)
	}
}

// TestDeletedIntakeRealDelFires proves an actual
// removal (id resolves nowhere in the tree) still trips the tamper wire
// after the split — the relaxation only recognizes a move, never a delete.
func TestDeletedIntakeRealDelFires(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "checkout", "-q", "-b", "base")
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, filepath.Join(idir, "rejected"))

	writeTemp(t, filepath.Join(idir, "rejected"), "2026-07-08-gone.md",
		"---\nid: I-really-deleted\ndate: \"2026-07-08\"\ntitle: Really deleted\ndisposition: rejected\nwhy: no\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "land I-really-deleted in rejected/")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	if err := os.Remove(filepath.Join(idir, "rejected", "2026-07-08-gone.md")); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "actually delete the tombstone (not allowed)")

	d := deletedRegisterFiles(root)
	if len(d) == 0 || !strings.Contains(strings.Join(d, " "), "gone.md") {
		t.Errorf("a real removal from the register dir must still fire; got %v", d)
	}
}

// TestIntakeCopyDupIDCloses proves the duplicate-id check
// closes the copy-without-delete hole the id-identity relaxation on its own
// would open: leaving the OLD path in place while also creating the NEW path
// with the same id (never deleting the original) must not silently pass —
// it must fire as a duplicate-id PROBLEM, not be waved through as a move.
func TestIntakeCopyDupIDCloses(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, filepath.Join(idir, "new"))
	mustMkdirAll(t, filepath.Join(idir, "watching"))

	writeTemp(t, filepath.Join(idir, "new"), "2026-07-08-old.md",
		"---\nid: I-copied\ndate: \"2026-07-08\"\ntitle: Copied\ndisposition: new\n---\n\nBody.")
	// Copy without delete: same id also exists at the new path.
	writeTemp(t, filepath.Join(idir, "watching"), "2026-07-08-new.md",
		"---\nid: I-copied\ndate: \"2026-07-08\"\ntitle: Copied\ndisposition: watching\n---\n\nBody.")

	problems := registerIntegrityProblems(root)
	if !containsSubstr(problems, "duplicate id I-copied") {
		t.Errorf("copy-without-delete (same id at two paths) must fire the duplicate-id problem; got %v", problems)
	}
}

// ----- buildRegisterMap: split-layout id resolution -----

func TestBuildMapWalksSubdirs(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, filepath.Join(idir, "completed"))
	writeTemp(t, filepath.Join(idir, "completed"), "2026-07-08-x.md",
		"---\nid: I-map-split\ndate: \"2026-07-08\"\ntitle: Split\ndisposition: scoped\nscoped-to: somestream\n---\n\nBody.")

	_, iMap := buildRegisterMap(root)
	got := iMap["I-map-split"]
	want := filepath.Join("docs", "streams", "intake", "completed", "2026-07-08-x.md")
	if got != want {
		t.Errorf("iMap[I-map-split] = %q, want %q", got, want)
	}
}

// ----- generateIntakeView: depth-sensitive body-link rebase -----

func TestIntakeViewRebaseSplit(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, filepath.Join(idir, "new"))
	// A body link authored relative to the entry file's OWN directory
	// (docs/streams/intake/new/) must rebase to be relative to
	// docs/streams/ (where INTAKE.md lives) — one level deeper than a
	// root-level entry's "intake" prefix.
	writeTemp(t, filepath.Join(idir, "new"), "2026-07-08-x.md",
		"---\nid: I-rebase-split\ndate: \"2026-07-08\"\ntitle: Rebase split\ndisposition: new\n---\n\nSee [sibling](2026-07-01-sibling.md).\n")

	view, err := generateIntakeView(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(view, "(intake/new/2026-07-01-sibling.md)") {
		t.Errorf("expected split-layout entry's body link rebased to intake/new/..., got:\n%s", view)
	}
}

func TestIntakeViewRebaseRoot(t *testing.T) {
	// Unchanged pre-split behavior: a root-level entry keeps the "intake"
	// (not "intake/<subdir>") rebase prefix.
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)
	writeTemp(t, idir, "2026-07-08-x.md",
		"---\nid: I-rebase-root\ndate: \"2026-07-08\"\ntitle: Rebase root\ndisposition: new\n---\n\nSee [sibling](2026-07-01-sibling.md).\n")

	view, err := generateIntakeView(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(view, "(intake/2026-07-01-sibling.md)") {
		t.Errorf("expected root-level entry's body link rebased to intake/..., got:\n%s", view)
	}
}
