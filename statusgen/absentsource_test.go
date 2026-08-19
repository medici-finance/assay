package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests pin the fix for #887: an ABSENT or UNREADABLE data source must
// surface could-not-check, never render as an affirmative clean read. Each test
// carries a positive control (the readable/absent case still behaves) so a
// future regression that swallows the read error again fails here rather than
// silently passing. See docs/three-state-instrument-rule.md, sub-rule 1.

// makeIntakeBase creates docs/streams and returns the streams dir. The intake
// register is created (or not) by the caller.
func makeStreamsDir(t *testing.T, root string) string {
	t.Helper()
	streams := filepath.Join(root, "docs", "streams")
	if err := os.MkdirAll(streams, 0o755); err != nil {
		t.Fatal(err)
	}
	return streams
}

// ---- Site 1: intake register (registerentries.go listIntakeFiles / parseIntakeDir) ----

func TestParseIntakeDirUnreadableIsCouldNotCheck(t *testing.T) {
	root := t.TempDir()
	streams := makeStreamsDir(t, root)
	// Make docs/streams/intake a FILE where a directory is expected: os.ReadDir
	// then fails with a non-IsNotExist error (ENOTDIR). Portable and root-safe
	// (unlike a mode-0000 fixture, which root can still read).
	writeTemp(t, streams, "intake", "not a directory\n")

	_, err := parseIntakeDir(root)
	if err == nil {
		t.Fatal("parseIntakeDir over an unreadable intake register returned nil error — the could-not-check was swallowed and would render as a clean read")
	}
	if !strings.Contains(err.Error(), "unreadable") {
		t.Errorf("error does not name the could-not-check condition: %v", err)
	}
	// loadIntake must propagate it (this is the intake-alarm path in main).
	if _, err := loadIntake(root); err == nil {
		t.Error("loadIntake over an unreadable intake register returned nil error")
	}
}

func TestParseIntakeDirAbsentIsLegitimateEmpty(t *testing.T) {
	// Positive control: an ABSENT intake register (no docs/streams/intake at all)
	// is a legitimate empty, not a could-not-check — it must NOT error, so a
	// bootstrap repo with no intake register still builds its board.
	root := t.TempDir()
	makeStreamsDir(t, root) // docs/streams exists, intake does not
	entries, err := parseIntakeDir(root)
	if err != nil {
		t.Fatalf("absent intake register must be a legitimate empty, got error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("absent intake register must yield zero entries, got %d", len(entries))
	}
}

func TestParseIntakeDirReadableEmptyIsClean(t *testing.T) {
	// Positive control: a readable, genuinely-empty intake dir is a clean zero.
	root := t.TempDir()
	streams := makeStreamsDir(t, root)
	if err := os.MkdirAll(filepath.Join(streams, "intake"), 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := parseIntakeDir(root)
	if err != nil {
		t.Fatalf("readable empty intake dir must not error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("empty intake dir must yield zero entries, got %d", len(entries))
	}
}

func TestIntakeBoardLineUnreadableIsNotCleanRead(t *testing.T) {
	// The renderer's affirmative line must be conditional on having actually
	// read the register: an unreadable result renders could-not-check, NEVER
	// "the front door is clear".
	got := intakeBoardLine(IntakeAlarmResult{Unreadable: true, Reason: "boom"})
	if strings.Contains(got, "front door is clear") {
		t.Errorf("unreadable intake rendered the false clean read: %q", got)
	}
	if !strings.Contains(got, "COULD-NOT-CHECK") {
		t.Errorf("unreadable intake did not render could-not-check: %q", got)
	}
	if !strings.Contains(got, "boom") {
		t.Errorf("could-not-check line dropped the reason: %q", got)
	}
	// Positive control: a genuine zero still reads clean.
	clean := intakeBoardLine(IntakeAlarmResult{Untriaged: 0})
	if !strings.Contains(clean, "front door is clear") {
		t.Errorf("a genuine zero must still render the clean line, got: %q", clean)
	}
}

// ---- Site 2: findings register dir (registerentries.go parseFindingsDir) ----

func TestParseFindingsDirUnreadableIsCouldNotCheck(t *testing.T) {
	root := t.TempDir()
	streams := makeStreamsDir(t, root)
	writeTemp(t, streams, "findings", "not a directory\n") // ENOTDIR on ReadDir
	if _, err := parseFindingsDir(root); err == nil {
		t.Fatal("parseFindingsDir over an unreadable findings register returned nil error — an unreadable source was rendered as empty/clean")
	}
}

func TestParseFindingsDirAbsentIsLegitimateEmpty(t *testing.T) {
	// Positive control: absent findings register → (nil, nil), a legitimate empty.
	root := t.TempDir()
	makeStreamsDir(t, root)
	entries, err := parseFindingsDir(root)
	if err != nil {
		t.Fatalf("absent findings register must not error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("absent findings register must yield zero entries, got %d", len(entries))
	}
}

// ---- Site 3: legacy findings file (load.go parseFindingsLegacy) ----

func TestParseFindingsLegacyUnreadableIsCouldNotCheck(t *testing.T) {
	root := t.TempDir()
	// A directory where the FINDINGS.md file is expected: os.ReadFile fails with
	// a non-IsNotExist error (EISDIR). Portable and root-safe.
	p := filepath.Join(root, "FINDINGS.md")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := parseFindingsLegacy(p); err == nil {
		t.Fatal("parseFindingsLegacy over an unreadable path returned nil error — an unreadable source was rendered as empty/clean")
	}
}

func TestParseFindingsLegacyAbsentIsLegitimateEmpty(t *testing.T) {
	// Positive control: absent legacy file → (nil, nil).
	fs, err := parseFindingsLegacy(filepath.Join(t.TempDir(), "nope.md"))
	if err != nil {
		t.Fatalf("absent legacy findings file must not error: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("absent legacy findings file must yield zero findings, got %d", len(fs))
	}
}

// ---- Site 4: docs walk (linkcheck.go docFiles) ----

func TestDocFilesUnreadableSubtreeSurfacesProblem(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; mode 0000 is still readable")
	}
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, root, "CLAUDE.md", "x")
	// A docs subtree that exists but cannot be read: WalkDir hands the callback a
	// non-IsNotExist error, which must be surfaced as a could-not-check rather
	// than dropped (which would enumerate fewer files and print LINT: PASS).
	blocked := filepath.Join(docs, "blocked")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, blocked, "hidden.md", "x")
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	_, walkProblems := docFiles(root)
	if len(walkProblems) == 0 {
		t.Fatal("an unreadable docs subtree produced zero walk problems — the link lint would print LINT: PASS on a source it never read")
	}
	joined := strings.Join(walkProblems, "\n")
	if !strings.Contains(joined, "blocked") {
		t.Errorf("walk problem does not name the unreadable subtree: %s", joined)
	}
	if !strings.Contains(joined, "could-not-check") {
		t.Errorf("walk problem is not framed as could-not-check: %s", joined)
	}
}

func TestDocFilesReadableTreeHasNoWalkProblems(t *testing.T) {
	// Positive control: a fully-readable docs tree produces no walk problems.
	root := t.TempDir()
	sub := filepath.Join(root, "docs", "streams", "x")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, root, "CLAUDE.md", "x")
	writeTemp(t, sub, "README.md", "x")
	files, walkProblems := docFiles(root)
	if len(walkProblems) != 0 {
		t.Fatalf("readable docs tree produced walk problems: %v", walkProblems)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2 (CLAUDE.md + README.md): %v", len(files), files)
	}
}

// ---- Site 5: brief-file glob (brieffile.go briefFilePaths) ----

func TestBriefFilePathsGlobMetacharDirNoLongerBlind(t *testing.T) {
	// A stream directory whose name contains an unbalanced glob metacharacter
	// made filepath.Glob(filepath.Join(s.Dir, "brief-*.md")) return ErrBadPattern
	// with zero matches, and the discarded error meant the stream's briefs
	// silently vanished. os.ReadDir treats the name literally, closing that blind
	// spot.
	base := t.TempDir()
	dir := filepath.Join(base, "weird[stream")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, dir, "brief-01.md", "x")
	writeTemp(t, dir, "brief-02-slug.md", "x")
	writeTemp(t, dir, "README.md", "x") // must be excluded

	got := briefFilePaths(&Stream{Name: "weird", Dir: dir})
	if len(got) != 2 {
		t.Fatalf("briefFilePaths lost briefs under a metacharacter-named dir: got %d, want 2: %v", len(got), got)
	}
	// Confirm the previous idiom WAS blind here (positive control that the
	// fixture actually exercises the bug), and that ours is not.
	if globbed, _ := filepath.Glob(filepath.Join(dir, "brief-*.md")); len(globbed) != 0 {
		t.Skipf("this platform's Glob did not choke on %q (got %v); the blind-spot fixture is a no-op here", dir, globbed)
	}
}

func TestBriefFilePathsOrdinaryDirStillWorks(t *testing.T) {
	// Positive control: an ordinary stream dir enumerates brief files, sorted,
	// excluding README.md and non-brief files.
	dir := filepath.Join(t.TempDir(), "ordinary")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, dir, "brief-02.md", "x")
	writeTemp(t, dir, "brief-01.md", "x")
	writeTemp(t, dir, "README.md", "x")
	writeTemp(t, dir, "notes.txt", "x")
	got := briefFilePaths(&Stream{Name: "ordinary", Dir: dir})
	if len(got) != 2 {
		t.Fatalf("got %d brief paths, want 2: %v", len(got), got)
	}
	if filepath.Base(got[0]) != "brief-01.md" || filepath.Base(got[1]) != "brief-02.md" {
		t.Errorf("brief paths not sorted: %v", got)
	}
}
