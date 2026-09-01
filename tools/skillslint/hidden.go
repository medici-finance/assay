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
// The basis. The invisible/zero-width class this control rejects IS the assigned
// Unicode Default_Ignorable_Code_Point property (plus the C0/C1 controls and
// invalid UTF-8, which DI does not cover). Targeting that whole property — not an
// enumeration of the codepoints seen so far — is what makes the control durable:
// a point patch closes one smuggling codepoint, but the property closes the CLASS
// so the next adversarial probe finds nothing. See the defaultIgnorable table for
// the assigned-only scoping decision (unassigned/reserved DI is deliberately left
// legal). Zs visible whitespace is deliberately legal too (see below).
//
// What it rejects (hard, exit 1), by CATEGORY/PROPERTY not by an enumerated
// blacklist — an enumeration inevitably misses members of the class:
//
//	format chars    the WHOLE unicode.Cf category. This is the general
//	                invisible-formatting class and it subsumes every smuggling
//	                vector by construction: bidi controls (U+202A–U+202E,
//	                U+2066–U+2069) AND the directional marks the Trojan-Source
//	                family also uses (LRM U+200E, RLM U+200F, ALM U+061C);
//	                zero-width (U+200B–U+200D, U+2060); the invisible math
//	                operators (U+2061–U+2064); the soft hyphen (U+00AD); the
//	                Unicode Tag block (U+E0001, U+E0020–U+E007F — the canonical
//	                LLM ASCII-smuggling vector); and U+FEFF, EXCEPT a U+FEFF that
//	                is the file's leading BOM, which is legitimate and allowed.
//	variation sel.  U+FE00–U+FE0F and U+E0100–U+E01EF. These are Mn/other, NOT
//	                Cf, so they are covered explicitly — a run of them appended to
//	                a carrier smuggles arbitrary bytes past a human reader.
//	other invisibles a curated set that render to nothing yet are neither Cf, VS
//	                nor Cc: U+034F combining grapheme joiner, the Hangul fillers
//	                (U+115F, U+1160, U+3164, U+FFA0), the Khmer inherent vowels
//	                (U+17B4, U+17B5), U+2800 braille blank, and the line/paragraph
//	                separators (U+2028, U+2029). "invisible ⊆ Cf ∪ VS ∪ Cc" is
//	                false; this closes it.
//	default-ignore  the ASSIGNED Default_Ignorable_Code_Point property, as its own
//	                property signal (defaultIgnorable) — the durable basis. Ensures
//	                every assigned-DI codepoint flags even if reclassified out of a
//	                category above (e.g. the Mongolian free variation selectors
//	                U+180B–180D, 180F, which are Mn — also folded into the
//	                variation-selector set for naming).
//	C0/C1 controls  everything unicode.Cc EXCEPT \t \n \r
//	invalid UTF-8   a byte that does not decode — an instruction file is text
//
// DELIBERATELY LEGAL: Zs space separators (the ordinary space, U+00A0 NBSP,
// U+2000–U+200A, U+202F, U+205F, U+3000). They are VISIBLE whitespace, not an
// invisible-smuggling class, and rejecting them would false-positive on every
// ordinary space. See hiddenKind for the stated rationale.
//
// Non-ASCII PRINTABLE text stays legal: accented names, arrows, box drawing,
// emoji whose base glyph carries its own presentation. The lint targets
// invisibility, not foreignness — it rejects the invisible CATEGORIES (Cf, the
// variation selectors, Cc) rather than enumerating permitted codepoints. One
// consequence worth stating: an emoji written with an explicit variation
// selector (e.g. U+26A0 U+FE0F) is flagged; the fix is the base glyph alone.
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

// variationSelectors are the Unicode variation selectors. Category Mn/other, NOT
// Cf, so hiddenKind checks them explicitly: a run of them appended to a carrier
// glyph is the emoji-variation-selector steganography channel — invisible to a
// human reader, arbitrary bytes to a parser.
var variationSelectors = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x180B, Hi: 0x180D, Stride: 1}, // MONGOLIAN FREE VARIATION SELECTOR ONE..THREE
		{Lo: 0x180F, Hi: 0x180F, Stride: 1}, // MONGOLIAN FREE VARIATION SELECTOR FOUR (U+180E is Cf, caught there)
		{Lo: 0xFE00, Hi: 0xFE0F, Stride: 1}, // VARIATION SELECTOR-1 .. -16
	},
	R32: []unicode.Range32{
		{Lo: 0xE0100, Hi: 0xE01EF, Stride: 1}, // VARIATION SELECTOR-17 .. -256
	},
}

// tagBlock is the Unicode Tags block (U+E0000–U+E007F). Its members ARE Cf, so
// hiddenKind already rejects them via unicode.Cf; this table exists only so
// codepointName can name them "TAG CHARACTER" rather than a generic descriptor.
var tagBlock = &unicode.RangeTable{
	R32: []unicode.Range32{
		{Lo: 0xE0000, Hi: 0xE007F, Stride: 1},
	},
}

// otherInvisibles is the curated set of invisible codepoints that render to
// nothing (or to blank width) yet fall OUTSIDE Cf, the variation selectors, and
// Cc — so the category branches above miss them. The closure assumption
// "invisible ⊆ Cf ∪ variationSelectors ∪ Cc" is false, and this table is the
// counter-set that closes it. Each entry is an invisible-smuggling vector, not a
// visible glyph:
//
//	U+034F   COMBINING GRAPHEME JOINER   Mn  — zero-width; the direct analog of ZWSP
//	U+115F   HANGUL CHOSEONG FILLER      Lo  — invisible filler
//	U+1160   HANGUL JUNGSEONG FILLER     Lo  — invisible filler
//	U+17B4   KHMER VOWEL INHERENT AQ     Mn  — renders to nothing
//	U+17B5   KHMER VOWEL INHERENT AA     Mn  — renders to nothing
//	U+2028   LINE SEPARATOR              Zl  — invisible line break
//	U+2029   PARAGRAPH SEPARATOR         Zp  — invisible paragraph break
//	U+2800   BRAILLE PATTERN BLANK       So  — renders blank
//	U+3164   HANGUL FILLER               Lo  — invisible filler
//	U+FFA0   HALFWIDTH HANGUL FILLER     Lo  — invisible filler
//
// It is curated by codepoint on purpose: unlike Cf there is no single Unicode
// category that means "invisible", so a category check here would sweep in
// visible glyphs. Range16 entries must stay sorted by Lo.
var otherInvisibles = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x034F, Hi: 0x034F, Stride: 1}, // COMBINING GRAPHEME JOINER
		{Lo: 0x115F, Hi: 0x1160, Stride: 1}, // HANGUL CHOSEONG/JUNGSEONG FILLER
		{Lo: 0x17B4, Hi: 0x17B5, Stride: 1}, // KHMER VOWEL INHERENT AQ/AA
		{Lo: 0x2028, Hi: 0x2029, Stride: 1}, // LINE / PARAGRAPH SEPARATOR
		{Lo: 0x2800, Hi: 0x2800, Stride: 1}, // BRAILLE PATTERN BLANK
		{Lo: 0x3164, Hi: 0x3164, Stride: 1}, // HANGUL FILLER
		{Lo: 0xFFA0, Hi: 0xFFA0, Stride: 1}, // HALFWIDTH HANGUL FILLER
	},
}

// defaultIgnorable is the ASSIGNED members of the Unicode
// Default_Ignorable_Code_Point property (DerivedCoreProperties.txt) — the
// property that, by definition, marks codepoints a renderer should show as
// nothing when it cannot render them. That property IS the invisible/zero-width
// class this control defends, so it is the control's durable basis: point patches
// close named codepoints one at a time, but targeting the whole DI class closes
// it so the next adversarial probe finds nothing.
//
// This is a curated table rather than a category test because Go's stdlib ships
// no RangeTable for Default_Ignorable, and no single General_Category equals it
// (its members span Cf, Mn, Lo, Zl and Zp). It is checked as its own branch in
// hiddenKind so a codepoint that a future Unicode revision moves OUT of Cf still
// flags on the DI property — a signal independent of the category branches.
//
// BAR: ASSIGNED DI only. The property also lists UNASSIGNED/reserved ranges
// (U+2065, U+FFF0–FFF8, U+E0000, U+E0002–E001F, U+E0080–E00FF, U+E01F0–E0FFF, and
// the reserved gaps in the tag/VS-supplement blocks). Those are DELIBERATELY
// EXCLUDED: an unassigned codepoint carries no payload today, rejecting reserved
// space is churny (it changes as Unicode assigns), and the reviewer scoped the
// bar to assigned members. When Unicode assigns one, it is added here explicitly.
//
// Ranges must stay sorted by Lo within R16/R32.
var defaultIgnorable = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x00AD, Hi: 0x00AD, Stride: 1}, // SOFT HYPHEN
		{Lo: 0x034F, Hi: 0x034F, Stride: 1}, // COMBINING GRAPHEME JOINER
		{Lo: 0x061C, Hi: 0x061C, Stride: 1}, // ARABIC LETTER MARK
		{Lo: 0x115F, Hi: 0x1160, Stride: 1}, // HANGUL CHOSEONG/JUNGSEONG FILLER
		{Lo: 0x17B4, Hi: 0x17B5, Stride: 1}, // KHMER VOWEL INHERENT AQ/AA
		{Lo: 0x180B, Hi: 0x180F, Stride: 1}, // MONGOLIAN FVS ONE..FOUR + VOWEL SEPARATOR (180E)
		{Lo: 0x200B, Hi: 0x200F, Stride: 1}, // ZWSP..ZWJ, LRM, RLM
		{Lo: 0x202A, Hi: 0x202E, Stride: 1}, // bidi embeddings/overrides
		{Lo: 0x2060, Hi: 0x2064, Stride: 1}, // WORD JOINER..INVISIBLE PLUS (2065 unassigned, excluded)
		{Lo: 0x2066, Hi: 0x206F, Stride: 1}, // bidi isolates + deprecated format
		{Lo: 0x3164, Hi: 0x3164, Stride: 1}, // HANGUL FILLER
		{Lo: 0xFE00, Hi: 0xFE0F, Stride: 1}, // VARIATION SELECTORS
		{Lo: 0xFEFF, Hi: 0xFEFF, Stride: 1}, // ZERO WIDTH NO-BREAK SPACE (leading BOM excepted in hiddenKind)
		{Lo: 0xFFA0, Hi: 0xFFA0, Stride: 1}, // HALFWIDTH HANGUL FILLER
	},
	R32: []unicode.Range32{
		{Lo: 0x1BCA0, Hi: 0x1BCA3, Stride: 1}, // SHORTHAND FORMAT controls
		{Lo: 0x1D173, Hi: 0x1D17A, Stride: 1}, // MUSICAL SYMBOL BEGIN/END BEAM..PHRASE
		{Lo: 0xE0001, Hi: 0xE0001, Stride: 1}, // LANGUAGE TAG
		{Lo: 0xE0020, Hi: 0xE007F, Stride: 1}, // TAG characters
		{Lo: 0xE0100, Hi: 0xE01EF, Stride: 1}, // VARIATION SELECTORS SUPPLEMENT
	},
}

// hiddenNames gives a human-readable name for the enumerated attack codepoints,
// so a violation message reads "U+202E RIGHT-TO-LEFT OVERRIDE" rather than a bare
// hex value. Anything not in the map falls back to a category descriptor in
// codepointName — the map is a courtesy, never the check (the check is by
// category, in hiddenKind).
var hiddenNames = map[rune]string{
	0x0000: "NULL",
	0x00AD: "SOFT HYPHEN",
	0x034F: "COMBINING GRAPHEME JOINER",
	0x061C: "ARABIC LETTER MARK",
	0x115F: "HANGUL CHOSEONG FILLER",
	0x1160: "HANGUL JUNGSEONG FILLER",
	0x17B4: "KHMER VOWEL INHERENT AQ",
	0x17B5: "KHMER VOWEL INHERENT AA",
	0x180B: "MONGOLIAN FREE VARIATION SELECTOR ONE",
	0x180C: "MONGOLIAN FREE VARIATION SELECTOR TWO",
	0x180D: "MONGOLIAN FREE VARIATION SELECTOR THREE",
	0x180E: "MONGOLIAN VOWEL SEPARATOR",
	0x180F: "MONGOLIAN FREE VARIATION SELECTOR FOUR",
	0x2028: "LINE SEPARATOR",
	0x2029: "PARAGRAPH SEPARATOR",
	0x2800: "BRAILLE PATTERN BLANK",
	0x3164: "HANGUL FILLER",
	0xFFA0: "HALFWIDTH HANGUL FILLER",
	0x200B: "ZERO WIDTH SPACE",
	0x200C: "ZERO WIDTH NON-JOINER",
	0x200D: "ZERO WIDTH JOINER",
	0x200E: "LEFT-TO-RIGHT MARK",
	0x200F: "RIGHT-TO-LEFT MARK",
	0x2060: "WORD JOINER",
	0x2061: "FUNCTION APPLICATION",
	0x2062: "INVISIBLE TIMES",
	0x2063: "INVISIBLE SEPARATOR",
	0x2064: "INVISIBLE PLUS",
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
	case unicode.Is(variationSelectors, r):
		return "VARIATION SELECTOR"
	case unicode.Is(otherInvisibles, r):
		return "INVISIBLE CHARACTER"
	case unicode.Is(tagBlock, r):
		return "TAG CHARACTER"
	case unicode.Is(bidiControls, r):
		return "BIDI CONTROL"
	case unicode.Is(zeroWidth, r):
		return "ZERO-WIDTH FORMAT"
	case unicode.Is(unicode.Cf, r):
		return "FORMAT CHARACTER"
	case unicode.Is(unicode.Cc, r):
		return "CONTROL CHARACTER"
	case unicode.Is(defaultIgnorable, r):
		return "DEFAULT-IGNORABLE CHARACTER"
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
	case unicode.Is(unicode.Cf, r):
		// The WHOLE format category — bidi controls and directional marks,
		// zero-width, invisible math operators, soft hyphen, the Tag block, and
		// non-leading U+FEFF. A category check cannot miss a member the way an
		// enumeration does.
		return true
	case unicode.Is(variationSelectors, r):
		// Mn/other, not Cf — checked explicitly (emoji-VS steganography channel).
		return true
	case unicode.Is(otherInvisibles, r):
		// The curated invisibles that are neither Cf, VS nor Cc (combining
		// grapheme joiner, Hangul/Khmer fillers, braille blank, line/paragraph
		// separators). See otherInvisibles for the full set and the reason it is
		// a codepoint list, not a category.
		return true
	case unicode.Is(defaultIgnorable, r):
		// The assigned Default_Ignorable_Code_Point class — the durable basis.
		// Redundant with the branches above for today's members BY DESIGN: it is
		// an independent PROPERTY signal, so a codepoint reclassified out of a
		// category still flags here. See defaultIgnorable.
		return true
	case unicode.Is(unicode.Cc, r):
		// C0/C1 controls. Cc is disjoint from Cf; \t \n \r are excused above.
		return true
	}
	// DELIBERATELY NOT REJECTED: Zs (space separators) — the ordinary space
	// U+0020, the non-breaking space U+00A0, U+2000–U+200A, U+202F, U+205F,
	// U+3000. They are VISIBLE whitespace, not invisible-smuggling: they occupy
	// width and a human reader sees a gap. Rejecting them carries a real
	// false-positive cost (every normal space) for no invisibility gain, so they
	// stay legal. This is a decision, not an omission.
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
