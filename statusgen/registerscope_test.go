package main

import (
	"path/filepath"
	"testing"
)

// Register-integrity --changed scoping (registerIntegrityScoped).
//
// The PR-side gate runs `--lint --changed <file>`. Before this scoping the
// register-integrity check walked the whole register regardless of the diff, so
// a pre-existing defect on main hard-failed EVERY open PR that touched
// docs/streams/**, even one that never touched the defective entry, and the
// stale red never cleared until the unrelated main-side defect was fixed. These
// tests pin the fix and its correctness boundary:
//
//	(a) a main-side defect on a register file the diff never changed no longer
//	    fails a PR that didn't touch it (it demotes to a surfaced NOTICE);
//	(b) a defect the diff DOES introduce or touch still fails.

// writeScopeIntake lays down one defective intake entry (an unparseable date —
// a git-independent register-integrity PROBLEM) and one clean entry, and returns
// their repo-relative paths.
func writeScopeIntake(t *testing.T) (root, defectRel, cleanRel string) {
	t.Helper()
	root = t.TempDir()
	ndir := filepath.Join(root, "docs", "streams", "intake", "new")
	mustMkdirAll(t, ndir)
	writeTemp(t, ndir, "2026-07-08-defect.md",
		"---\nid: I-scope-defect-a\ndate: not-a-date\ntitle: Defect\ndisposition: new\n---\n\nBody.")
	writeTemp(t, ndir, "2026-07-08-clean.md",
		"---\nid: I-scope-clean-a\ndate: \"2026-07-08\"\ntitle: Clean\ndisposition: new\n---\n\nBody.")
	return root,
		"docs/streams/intake/new/2026-07-08-defect.md",
		"docs/streams/intake/new/2026-07-08-clean.md"
}

// (baseline) The defect is a real register-integrity PROBLEM on a whole-tree run
// and with NO --changed set — behavior must be unchanged there.
func TestRegisterScopeUnchangedWhenNoChangedSet(t *testing.T) {
	root, _, _ := writeScopeIntake(t)

	// Whole-tree (the historical flat entry point).
	if !containsSubstr(registerIntegrityProblems(root), "I-scope-defect-a") {
		t.Fatalf("whole-tree register lint must flag the unparseable-date defect")
	}
	// Scoped entry point with an empty changed set == whole-tree, no notices.
	problems, notices := registerIntegrityScoped(root, nil)
	if !containsSubstr(problems, "I-scope-defect-a") {
		t.Fatalf("empty --changed set must behave exactly as the whole-tree run (defect is a PROBLEM); got %v", problems)
	}
	if len(notices) != 0 {
		t.Fatalf("empty --changed set must not demote anything to a NOTICE; got %v", notices)
	}
}

// (a) A main-side defect on a path the PR's diff never touched must NOT fail the
// PR — it demotes to a surfaced NOTICE (never silently dropped).
func TestRegisterScopePreexistingDefectOnUnchangedPathDemotesToNotice(t *testing.T) {
	root, _, cleanRel := writeScopeIntake(t)

	// The PR only touches the CLEAN entry; the defect sits on a file it never changed.
	problems, notices := registerIntegrityScoped(root, []string{cleanRel})

	if containsSubstr(problems, "I-scope-defect-a") {
		t.Fatalf("a pre-existing defect on a path outside the PR's --changed set must NOT be a hard PROBLEM; got %v", problems)
	}
	if !containsSubstr(notices, "I-scope-defect-a") {
		t.Fatalf("the demoted defect must be surfaced as a NOTICE, never silently dropped; got %v", notices)
	}
	if !containsSubstr(notices, "pre-existing register defect") {
		t.Fatalf("the demotion NOTICE must name itself pre-existing; got %v", notices)
	}
}

// (b) A defect the diff DOES introduce or touch must still fail — the gate keeps
// its teeth for the case it exists for.
func TestRegisterScopeDefectInChangedSetStillFails(t *testing.T) {
	root, defectRel, _ := writeScopeIntake(t)

	// The PR touches the DEFECTIVE entry itself.
	problems, notices := registerIntegrityScoped(root, []string{defectRel})

	if !containsSubstr(problems, "I-scope-defect-a") {
		t.Fatalf("a defect on a file in the PR's --changed set must remain a hard PROBLEM; got %v", problems)
	}
	if containsSubstr(notices, "I-scope-defect-a") {
		t.Fatalf("a defect the diff touches must not be demoted to a NOTICE; got %v", notices)
	}
}

// A duplicate id is attributed to EVERY file that claims it, so touching either
// participant keeps the duplicate a hard PROBLEM (the boundary must not let a
// two-file defect slip when the PR edits just one of the two).
func TestRegisterScopeDuplicateIdAttributedToBothFiles(t *testing.T) {
	root := t.TempDir()
	ndir := filepath.Join(root, "docs", "streams", "intake", "new")
	mustMkdirAll(t, ndir)
	writeTemp(t, ndir, "2026-07-08-dup-a.md",
		"---\nid: I-scope-dup-a\ndate: \"2026-07-08\"\ntitle: DupA\ndisposition: new\n---\n\nBody.")
	writeTemp(t, ndir, "2026-07-08-dup-b.md",
		"---\nid: I-scope-dup-a\ndate: \"2026-07-08\"\ntitle: DupB\ndisposition: new\n---\n\nBody.")
	dupB := "docs/streams/intake/new/2026-07-08-dup-b.md"

	// Touching only the SECOND file still fails the shared duplicate-id defect.
	problems, _ := registerIntegrityScoped(root, []string{dupB})
	if !containsSubstr(problems, "duplicate id I-scope-dup-a") {
		t.Fatalf("a duplicate id must stay a PROBLEM when the diff touches either file that carries it; got %v", problems)
	}

	// Touching an unrelated file demotes it.
	unrelated := "docs/streams/intake/new/2026-07-08-elsewhere.md"
	problems2, notices2 := registerIntegrityScoped(root, []string{unrelated})
	if containsSubstr(problems2, "duplicate id I-scope-dup-a") {
		t.Fatalf("a duplicate id on files outside the changed set must demote; got %v", problems2)
	}
	if !containsSubstr(notices2, "duplicate id I-scope-dup-a") {
		t.Fatalf("the demoted duplicate-id defect must be surfaced as a NOTICE; got %v", notices2)
	}
}
