// Byte-level lint of the instruction surfaces: an invisible-character /
// Trojan-Source check (HARD — a violation is exit 1) and an advisory
// context-budget NOTICE (never exit-affecting).
//
// Why bytes, not structure. The structural lint in lint.go reads the header the
// way the harness does; it says nothing about the raw bytes underneath. A skill
// or instruction file can carry a Unicode payload — a bidi override that
// reorders how a line RENDERS, a zero-width joiner spliced mid-word, a C0/C1
// control — that a human reviewing the rendered text cannot see, yet the model
// reads on activation. That is the exact failure class human review is built to
// miss, so the layer that catches it must read a different signal (the bytes)
// in a different component (this lint) than the review gate above it.
//
// What it rejects (hard, exit 1), by CATEGORY not by codepoint blacklist:
//
//	bidi controls   U+202A–U+202E, U+2066–U+2069   (Trojan-Source reordering)
//	zero-width      U+200B–U+200D, U+2060, U+FEFF   (invisible splices; a U+FEFF
//	                that is the file's leading BOM is legitimate and allowed)
//	C0/C1 controls  everything unicode.Cc EXCEPT \t \n \r
//	invalid UTF-8   a byte that does not decode — an instruction file is text
//
// Non-ASCII PRINTABLE text stays legal: accented names, arrows, box drawing,
// emoji. The lint targets invisibility, not foreignness — the allowlist is the
// visible/printable categories, expressed as "is this rune in one of the hidden
// ranges", never an enumeration of permitted codepoints.
//
// The budget half counts words per file and prints a NOTICE over a threshold
// (SKILL.md 3000, CLAUDE.md 5000). Larger instruction files correlate with more
// hallucination, so an over-budget file is a context-bloat CANDIDATE worth a
// human's eye — a judgment call, so it is advisory: it moves no exit code.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Context-budget thresholds, in words. Over the threshold prints an advisory
// NOTICE; it never changes the exit code (see the package header).
const (
	budgetThresholdSkillMd  = 3000
	budgetThresholdClaudeMd = 5000
)

// bidiControls are the Unicode bidirectional formatting characters used to
// reorder how a line renders while leaving its logical byte order intact — the
// Trojan-Source class. Category-based: unicode.Is over this range table.
var bidiControls = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x202A, Hi: 0x202E, Stride: 1}, // LRE, RLE, PDF, LRO, RLO
		{Lo: 0x2066, Hi: 0x2069, Stride: 1}, // LRI, RLI, FSI, PDI
	},
}

// zeroWidth are the zero-width / invisible format characters that can be spliced
// into or between words without any visible trace. U+FEFF is included here for a
// non-leading occurrence; a leading BOM is handled as a legitimate exception in
// hiddenKind.
var zeroWidth = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x200B, Hi: 0x200D, Stride: 1}, // ZWSP, ZWNJ, ZWJ
		{Lo: 0x2060, Hi: 0x2060, Stride: 1}, // WORD JOINER
		{Lo: 0xFEFF, Hi: 0xFEFF, Stride: 1}, // ZERO WIDTH NO-BREAK SPACE
	},
}

// hiddenNames gives a human-readable name for the enumerated attack codepoints,
// so a violation message reads "U+202E RIGHT-TO-LEFT OVERRIDE" rather than a bare
// hex value. Anything not in the map falls back to a category descriptor in
// codepointName — the map is a courtesy, never the check (the check is by
// category, in hiddenKind).
var hiddenNames = map[rune]string{
	0x0000: "NULL",
	0x200B: "ZERO WIDTH SPACE",
	0x200C: "ZERO WIDTH NON-JOINER",
	0x200D: "ZERO WIDTH JOINER",
	0x2060: "WORD JOINER",
	0xFEFF: "ZERO WIDTH NO-BREAK SPACE",
	0x202A: "LEFT-TO-RIGHT EMBEDDING",
	0x202B: "RIGHT-TO-LEFT EMBEDDING",
	0x202C: "POP DIRECTIONAL FORMATTING",
	0x202D: "LEFT-TO-RIGHT OVERRIDE",
	0x202E: "RIGHT-TO-LEFT OVERRIDE",
	0x2066: "LEFT-TO-RIGHT ISOLATE",
	0x2067: "RIGHT-TO-LEFT ISOLATE",
	0x2068: "FIRST STRONG ISOLATE",
	0x2069: "POP DIRECTIONAL ISOLATE",
}

// codepointName returns a name for r for the violation message: the specific
// name when known, otherwise the category descriptor.
func codepointName(r rune) string {
	if n, ok := hiddenNames[r]; ok {
		return n
	}
	switch {
	case unicode.Is(bidiControls, r):
		return "BIDI CONTROL"
	case unicode.Is(zeroWidth, r):
		return "ZERO-WIDTH FORMAT"
	case unicode.Is(unicode.Cc, r):
		return "CONTROL CHARACTER"
	}
	return "HIDDEN CHARACTER"
}

// hiddenKind reports whether r is a rejected invisible/hidden character. atFileStart
// is true only for the very first rune of the file, which is where a U+FEFF is a
// legitimate byte-order mark rather than a zero-width payload. \t \n \r are the
// only control characters instruction files legitimately carry.
func hiddenKind(r rune, atFileStart bool) (bad bool) {
	switch {
	case r == '\t' || r == '\n' || r == '\r':
		return false
	case r == 0xFEFF && atFileStart:
		return false // leading BOM is legitimate; a non-leading U+FEFF is not
	case unicode.Is(bidiControls, r):
		return true
	case unicode.Is(zeroWidth, r):
		return true
	case unicode.Is(unicode.Cc, r):
		return true
	}
	return false
}

// scanHidden decodes raw as UTF-8 and returns one Issue per invisible/hidden
// character (or invalid UTF-8 byte), each naming line, 1-based rune column, and
// the codepoint. rel is the repo-relative path used in the message.
func scanHidden(rel string, raw []byte) []Issue {
	var issues []Issue
	line, col := 1, 0
	for i := 0; i < len(raw); {
		r, size := utf8.DecodeRune(raw[i:])
		if r == utf8.RuneError && size == 1 {
			// A byte that is not valid UTF-8. An instruction file is text; a raw
			// high byte is either corruption or a smuggled payload.
			col++
			issues = append(issues, Issue{Path: rel, Msg: fmt.Sprintf(
				"line %d col %d: byte 0x%02X is not valid UTF-8 — an instruction file must be valid UTF-8 text", line, col, raw[i])})
			i += size
			continue
		}
		if r == '\n' {
			line++
			col = 0
			i += size
			continue
		}
		col++
		if hiddenKind(r, i == 0) {
			issues = append(issues, Issue{Path: rel, Msg: fmt.Sprintf(
				"line %d col %d: U+%04X %s — invisible/hidden character not permitted in an instruction file (human review of the rendered text cannot see it)",
				line, col, r, codepointName(r))})
		}
		i += size
	}
	return issues
}

// budgetThreshold is the word count over which a file earns a context-bloat
// NOTICE. CLAUDE.md carries more legitimately than a SKILL.md, so it has a higher
// ceiling.
func budgetThreshold(path string) int {
	if strings.EqualFold(filepath.Base(path), "CLAUDE.md") {
		return budgetThresholdClaudeMd
	}
	return budgetThresholdSkillMd
}

// inScopeMarkdown returns the absolute paths of every instruction surface under
// root that the byte-level scans cover: every *.md under plugins/assay/skills/
// and under .claude/skills/ (the shipped bundle and the house skill homes), plus
// plugins/assay/resident-rules.md and a top-level CLAUDE.md when present. A
// surface that does not exist in a given root is simply skipped — the scan reads
// what is there, and its could-not-check condition is "found nothing to read"
// (handled by the caller), not "a named surface is absent".
func inScopeMarkdown(root string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(abs string) {
		if !seen[abs] {
			seen[abs] = true
			out = append(out, abs)
		}
	}
	for _, sub := range []string{
		filepath.Join("plugins", "assay", "skills"),
		filepath.Join(".claude", "skills"),
	} {
		dir := filepath.Join(root, sub)
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		werr := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.EqualFold(filepath.Ext(p), ".md") {
				add(p)
			}
			return nil
		})
		if werr != nil {
			return nil, fmt.Errorf("walk %s: %w", dir, werr)
		}
	}
	for _, single := range []string{
		filepath.Join("plugins", "assay", "resident-rules.md"),
		"CLAUDE.md",
	} {
		p := filepath.Join(root, single)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			add(p)
		}
	}
	sort.Strings(out)
	return out, nil
}

// ScanInstructionSurfaces walks the in-scope markdown surfaces under root and
// returns the number scanned, one Issue per invisible/hidden-character violation
// (the HARD half), one NOTICE line per over-budget file (the ADVISORY half), and
// a non-nil error only for a structural failure of the walk itself. checked == 0
// with no error means no in-scope surface was found — the caller treats that as
// could-not-check, never a pass.
func ScanInstructionSurfaces(root string) (checked int, issues []Issue, notices []string, err error) {
	files, ferr := inScopeMarkdown(root)
	if ferr != nil {
		return 0, nil, nil, ferr
	}
	for _, abs := range files {
		rel, rerr := filepath.Rel(root, abs)
		if rerr != nil {
			rel = abs
		}
		rel = filepath.ToSlash(rel)

		raw, readErr := os.ReadFile(abs)
		if readErr != nil {
			// A file we located but cannot read is could-not-check for that file,
			// reported as an issue so it fails rather than passing silently.
			issues = append(issues, Issue{Path: rel, Msg: fmt.Sprintf("cannot read for byte scan: %v", readErr)})
			continue
		}
		checked++
		issues = append(issues, scanHidden(rel, raw)...)
		if n, t := len(strings.Fields(string(raw))), budgetThreshold(abs); n > t {
			notices = append(notices, fmt.Sprintf(
				"skillslint: NOTICE: %s: %d words (budget %d) — context-bloat candidate", rel, n, t))
		}
	}
	return checked, issues, notices, nil
}
