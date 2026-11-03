package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #3697 — the reported instance: desk-supervision/33's first files:
// bullet wraps its description onto a line that starts with "(".
func TestScopeWrappedBulletParen(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("testdata", "declaredscope", "desk-supervision-33.md"))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := BriefDeclaredFiles(string(content))
	if err != nil || strings.Join(paths, ",") != "spec/registers-v1.md" {
		t.Fatalf("wrapped bullet: got %v %v; want [spec/registers-v1.md]", paths, err)
	}
}

// joinWrapped is the one-line form an author's reflow would produce: every
// indented continuation line joined onto the line above it.
func joinWrapped(entry string) string {
	var lines []string
	for _, l := range strings.Split(entry, "\n") {
		if len(lines) > 0 && l != "" && l != strings.TrimLeft(l, " ") {
			lines[len(lines)-1] += " " + strings.TrimLeft(l, " ")
			continue
		}
		lines = append(lines, l)
	}
	return strings.Join(lines, "\n")
}

// regression: #3697 — the class: a bullet's description wrapped onto an
// indented line parses exactly as the same bullet joined onto one line.
func TestScopeWrappedEqualsJoined(t *testing.T) {
	for entry, want := range map[string]string{
		// The other wrapped bullets of the same brief, each a different first character.
		"- **add** `spec/intake-exit-v1.md` (first planned elsewhere) — the record schema: fields\n  table, closed vocabularies, the never-recorded list.": "spec/intake-exit-v1.md",
		"- **edit** `statusgen/registerentries.go` (+ test) — `intakeEntry` gains `Triaged`,\n  `TriagerTier`; value checks when present.":                 "statusgen/registerentries.go",
		"- **edit** `plugins/x/SKILL.md` — the triage steps (lines 31-42 table,\n  408-433 intake lane): stamp the entry.":                                 "plugins/x/SKILL.md",
		"- **add** `tools/x/land.go` (planned) — the record type; `verb.go` (planned) — the\n  `scanloop land` verb for judgment exits.":                   "tools/x/land.go",
		"- `tools/a.go`: the reader, which\n  (unlike the writer) never locks.":                                                                            "tools/a.go",
		"- `tools/a.go` — one\n  two (three\n  four) five.\n- `tools/b.go`":                                                                                "tools/a.go,tools/b.go",
		"  - `tools/a.go` — an indented bullet whose\n    continuation is indented deeper.":                                                                "tools/a.go",
	} {
		joined, jerr := BriefDeclaredFiles("## Context\nfiles:\n" + joinWrapped(entry) + "\nfacts: x\n")
		if jerr != nil || strings.Join(joined, ",") != want {
			t.Fatalf("test premise: joined %q: got %v %v; want %s", entry, joined, jerr, want)
		}
		paths, err := BriefDeclaredFiles("## Context\nfiles:\n" + entry + "\nfacts: x\n")
		if err != nil || strings.Join(paths, ",") != want {
			t.Errorf("wrapped %q: got %v %v; want %s (the joined form)", entry, paths, err, want)
		}
	}
}

// regression: #3697 — the boundary: only a description's wrap is prose. A
// continuation line that reads as a declaration still declares, and every
// other shape is refused exactly as before.
func TestScopeWrappedBoundary(t *testing.T) {
	for entry, want := range map[string]string{
		"- `tools/a.go` — reader\n  `tools/b.go`":           "tools/a.go,tools/b.go",
		"- `tools/a.go` — reader\n  tools/b.go — writer":    "tools/a.go,tools/b.go",
		"- `tools/a.go` — reader\n  - `tools/b.go`":         "tools/a.go,tools/b.go",
		"- `tools/a.go`,\n  `tools/b.go` — both":            "tools/a.go,tools/b.go",
		"- `tools/a.go` — reader\n  (note)\n  `tools/b.go`": "tools/a.go,tools/b.go",
	} {
		paths, err := BriefDeclaredFiles("## Context\nfiles:\n" + entry + "\nfacts: x\n")
		if err != nil || strings.Join(paths, ",") != want {
			t.Errorf("%q: got %v %v; want %s", entry, paths, err, want)
		}
	}
	for _, entry := range []string{
		"- `tools/a.go`,\n  some prose here",              // no description to continue
		"- `tools/a.go`\n  (note)",                        // no description to continue
		"- `tools/a.go` — reader\n(lazy) at column zero",  // not indented
		" - `tools/a.go` — reader\n (note)",               // not deeper than the bullet
		"- `tools/a.go` — reader\n  `../../x`",            // a declaration, unsupported
		"- `tools/a.go` — reader\n  files to change here", // a files marker, never prose
		"- `tools/a.go` — reader\n  > some prose",         // a quote, not a wrap
		"- `tools/a.go` — reader\n  - some prose",         // a nested item, not a wrap
		"- `tools/a.go` — reader\n  1. some prose",        // an ordered item, not a wrap
	} {
		content := "## Context\nfiles:\n" + entry + "\nfacts: x\n"
		if paths, err := BriefDeclaredFiles(content); ExitCodeOf(err) != ExitRefused {
			t.Errorf("silent partial scope: %q: %v %v", entry, paths, err)
		}
	}
	// An inline files: declaration has no bullet to continue.
	if paths, err := BriefDeclaredFiles("## Context\nfiles: `tools/a.go` — reader\n  (note)\n"); ExitCodeOf(err) != ExitRefused {
		t.Errorf("inline declaration continued: %v %v", paths, err)
	}
}
