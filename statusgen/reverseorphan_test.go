package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeBriefFileStub drops a minimal brief file so briefFilePaths/expectedBriefID
// sees a file for that number. Content need not be a valid brief — the
// reverse-orphan lint only checks for the FILE's existence by number.
func writeBriefFileStub(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("---\nschema: brief-v1\n---\n# stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestReverseOrphanPhantomRowIsProblem: a README brief ROW whose brief FILE is
// absent is a hard PROBLEM — the reverse of the forward file→row check. Row 01
// is backed by a file; the phantom row 02 has no brief-02*.md and must red.
func TestReverseOrphanPhantomRowIsProblem(t *testing.T) {
	dir := t.TempDir()
	writeBriefFileStub(t, dir, "brief-01-real.md")
	s := &Stream{Name: "alpha", Dir: dir, Briefs: []Brief{
		{Num: "01"}, // backed by brief-01-real.md
		{Num: "02"}, // PHANTOM — no file behind it
	}}
	problems, notices := reverseOrphanProblems([]*Stream{s})
	if len(problems) != 1 {
		t.Fatalf("want exactly 1 reverse-orphan problem, got %d: %v (notices: %v)", len(problems), problems, notices)
	}
	if !strings.Contains(problems[0], "alpha") || !strings.Contains(problems[0], `"02"`) {
		t.Errorf("problem must name the stream and the phantom row 02; got %q", problems[0])
	}
	if !strings.Contains(problems[0], "phantom") {
		t.Errorf("problem must explain it is a phantom row; got %q", problems[0])
	}
}

// TestReverseOrphanCleanBoardIsSilent: when every README row is backed by a
// brief file, the lint is silent.
func TestReverseOrphanCleanBoardIsSilent(t *testing.T) {
	dir := t.TempDir()
	writeBriefFileStub(t, dir, "brief-01-a.md")
	writeBriefFileStub(t, dir, "brief-02-b.md")
	s := &Stream{Name: "alpha", Dir: dir, Briefs: []Brief{{Num: "01"}, {Num: "02"}}}
	problems, notices := reverseOrphanProblems([]*Stream{s})
	if len(problems) != 0 || len(notices) != 0 {
		t.Errorf("a fully file-backed board must be silent; got problems=%v notices=%v", problems, notices)
	}
}

// TestReverseOrphanTableOnlyStreamExempt: a stream with ZERO brief files is
// table-only (legacy / not-yet-split) and never adopted the file-backed
// convention, so its rows are exempt — asserting the convention would red an
// entire legitimate board rather than catch a stray phantom.
func TestReverseOrphanTableOnlyStreamExempt(t *testing.T) {
	dir := t.TempDir() // no brief-*.md files at all
	s := &Stream{Name: "legacy", Dir: dir, Briefs: []Brief{{Num: "01"}, {Num: "02"}}}
	problems, _ := reverseOrphanProblems([]*Stream{s})
	if len(problems) != 0 {
		t.Errorf("a table-only stream (no brief files) must be exempt; got %v", problems)
	}
}

// TestReverseOrphanPlaceholderRowExempt: a synthetic placeholder-v1 row is
// backed by an issue-NN.md file, not a brief-NN.md, so it must NOT be flagged as
// a phantom brief row even though no brief-*.md exists for it. The stream still
// has a real brief file so it is not exempt as table-only.
func TestReverseOrphanPlaceholderRowExempt(t *testing.T) {
	dir := t.TempDir()
	writeBriefFileStub(t, dir, "brief-01-a.md")
	s := &Stream{Name: "alpha", Dir: dir, Briefs: []Brief{
		{Num: "01"},
		{Num: "issue-300", Schema: "placeholder-v1"}, // synthetic — exempt
	}}
	problems, _ := reverseOrphanProblems([]*Stream{s})
	if len(problems) != 0 {
		t.Errorf("a placeholder-v1 synthetic row must be exempt from the reverse-orphan lint; got %v", problems)
	}
}

// TestReverseOrphanUnreadableDirIsCouldNotCheck: an unreadable stream directory
// is reported as a could-not-check NOTICE, never a silent pass.
func TestReverseOrphanUnreadableDirIsCouldNotCheck(t *testing.T) {
	s := &Stream{Name: "gone", Dir: filepath.Join(t.TempDir(), "does-not-exist"), Briefs: []Brief{{Num: "01"}}}
	problems, notices := reverseOrphanProblems([]*Stream{s})
	if len(problems) != 0 {
		t.Errorf("an unreadable dir must not produce a hard PROBLEM; got %v", problems)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "could-not-check") {
		t.Errorf("want a could-not-check NOTICE for the unreadable dir; got %v", notices)
	}
}
