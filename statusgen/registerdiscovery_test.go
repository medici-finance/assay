package main

import (
	"path/filepath"
	"testing"
)

// Stream discovery must treat a REGISTER directory as a register, not a stream
// board (issue #616). A register README legitimately carries no `--- … ---`
// frontmatter (spec/registers-v1.md §7), so parsing it as a stream aborts the
// whole discovery run with "no frontmatter: first line must be ---" — and every
// statusgen-driven --next-up/--consumers Verify row then returns could-not-check
// instead of its real result.
//
// The registers the spec fixes by directory NAME (findings/intake/requirements/
// decisions) are skipped by reservedRegisterNames. These tests pin the general
// case: a register directory whose README DECLARES itself a register per §7 but
// whose name is NOT in that set must be skipped too, not aborted on.

// selfDeclaringRegisterREADME is the frontmatter-free, decisions/README.md-shaped
// register index that reproduces the #616 abort against the pre-fix scanner: its
// directory name ("retro") is not in reservedRegisterNames, and it opens with a
// heading rather than `---` frontmatter, so the pre-fix loadStreams hands it to
// parseStreamREADME and aborts the whole run. The canonical self-declaration
// ("register, not a stream — stream discovery skips it") is the §7 marker the fix
// keys off.
const selfDeclaringRegisterREADME = `# RETRO register — retrospective records

This directory is the RETRO register (spec/registers-v1.md §8). It is a register,
not a stream — stream discovery skips it. One record per file, ` + "`R-<slug>.md`" + `,
with no wave, no DoD and no status table.
`

// writeDiscoveryFixture lays down one real stream plus one self-declaring register
// directory (a non-reserved name, frontmatter-free) under root's docs/streams.
func writeDiscoveryFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	streamsDir := filepath.Join(root, "docs", "streams")

	// A genuine stream board — this is what discovery must still return.
	sdir := filepath.Join(streamsDir, "operator")
	mustMkdirAll(t, sdir)
	writeTemp(t, sdir, "README.md", sampleReadme)

	// A register directory whose name is NOT in reservedRegisterNames but whose
	// README declares itself a register per spec §7. The pre-fix scanner aborts
	// on this file; the fix must skip it.
	rdir := filepath.Join(streamsDir, "retro")
	mustMkdirAll(t, rdir)
	writeTemp(t, rdir, "README.md", selfDeclaringRegisterREADME)

	return root
}

// TestLoadSkipsRegister is the #616 regression: a
// self-declaring register directory whose name is not reserved must be SKIPPED,
// leaving discovery to succeed (exit 0) and return only the real stream — never
// aborted on the register's absent frontmatter.
func TestLoadSkipsRegister(t *testing.T) {
	root := writeDiscoveryFixture(t)

	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatalf("loadStreams aborted on a self-declared register README (issue #616): %v", err)
	}
	if len(streams) != 1 {
		t.Fatalf("got %d streams, want 1 (the register must be skipped, not parsed as a stream)", len(streams))
	}
	if streams[0].Name != "operator" {
		t.Fatalf("discovered stream = %q, want the real stream %q", streams[0].Name, "operator")
	}
}

// TestHydratedSkipsRegister exercises the exact path
// --next-up/--consumers use (loadHydratedStreams → loadStreams): the register
// must not abort the hydrated discovery run either.
func TestHydratedSkipsRegister(t *testing.T) {
	root := writeDiscoveryFixture(t)

	streams, _, err := loadHydratedStreams(root)
	if err != nil {
		t.Fatalf("loadHydratedStreams aborted on a self-declared register README (issue #616): %v", err)
	}
	if len(streams) != 1 || streams[0].Name != "operator" {
		t.Fatalf("got %d streams (want 1: operator) — the register was not skipped", len(streams))
	}
}

// TestLoadAbortsMalformedStream is the correctness boundary: the
// skip must fire ONLY for a self-declaring register. A README that is neither a
// valid stream (no frontmatter) NOR a self-declared register is a genuinely
// malformed stream and MUST still abort — the fix must not swallow real
// malformations by treating every frontmatter-free README as a register.
func TestLoadAbortsMalformedStream(t *testing.T) {
	root := t.TempDir()
	bad := filepath.Join(root, "docs", "streams", "broken")
	mustMkdirAll(t, bad)
	// No frontmatter, and no register self-declaration — a broken stream README.
	writeTemp(t, bad, "README.md", "# Broken stream\n\nSomeone forgot the frontmatter.\n")

	if _, _, err := loadStreams(root); err == nil {
		t.Fatal("a frontmatter-free README that does NOT declare itself a register must still abort discovery, not be silently skipped")
	}
}

// TestSelfDeclaredRegisterMarker pins the §7 marker detector: it recognizes the
// canonical declaration, does not match an ordinary stream README that merely
// uses the words "not a stream" in prose, and returns false (never a silent skip)
// for an unreadable path.
func TestSelfDeclaredRegisterMarker(t *testing.T) {
	dir := t.TempDir()

	reg := writeTemp(t, dir, "register.md", selfDeclaringRegisterREADME)
	if !isSelfDeclaredRegisterREADME(reg) {
		t.Fatal("the canonical §7 self-declaration must be recognized as a register marker")
	}

	// "not a stream blocker" (windows-port/README.md prose) must NOT match — the
	// anchor is "register, not a stream", not the bare words "not a stream".
	prose := writeTemp(t, dir, "prose.md",
		"# A stream\n\nThis open question gates a smoke test; it is not a stream blocker.\n")
	if isSelfDeclaredRegisterREADME(prose) {
		t.Fatal("a stream README using \"not a stream\" in ordinary prose must NOT match the register marker")
	}

	// Unreadable path → false (could-not-check is never rounded to a skip).
	if isSelfDeclaredRegisterREADME(filepath.Join(dir, "does-not-exist.md")) {
		t.Fatal("an unreadable README must return false, never a silent register skip")
	}
}
