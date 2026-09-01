package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeInScope materialises a SKILL.md with body under
// plugins/assay/skills/<dir>/ (an in-scope byte-scan surface) and returns its
// repo-relative slash path.
func writeInScope(t *testing.T, root, dir, body string) string {
	t.Helper()
	d := filepath.Join(root, "plugins", "assay", "skills", dir)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", d, err)
	}
	if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	return "plugins/assay/skills/" + dir + "/SKILL.md"
}

// TestScanInstructionSurfaces_HiddenClasses is the per-class table test: each
// attack class (bidi override, bidi isolate, zero-width, word joiner, C0 NUL, C1
// control, non-leading BOM, invalid UTF-8) must produce a hidden-character issue
// naming the codepoint, and each legitimate case (clean ASCII, printable
// non-ASCII, a leading BOM, tabs and CRLF) must produce none.
//
// The attack codepoints are written as explicit \u / \x escapes, never as actual
// invisible bytes, so the fixture is fully reviewable — the thing under test is
// spelled out in the source rather than lurking invisibly in it. (Go also forbids
// a literal U+FEFF anywhere but byte 0, which forces the escape for the BOM rows.)
//
// FAIL-FIRST: before hidden.go existed, ScanInstructionSurfaces did not exist and
// every hidden case here reported zero issues (the file passed every gate the
// tool had). Narrowing hiddenKind to drop any one class — e.g. removing the
// unicode.Is(bidiControls, r) case — reddens exactly the rows for that class.
func TestScanInstructionSurfaces_HiddenClasses(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantHit   bool
		wantInMsg string // substring the violation message must contain
	}{
		{"zero-width-space", "hello​world\n", true, "U+200B ZERO WIDTH SPACE"},
		{"zero-width-joiner", "he‍llo\n", true, "U+200D ZERO WIDTH JOINER"},
		{"bidi-rlo-override", "amount ‮ drawkcab\n", true, "U+202E RIGHT-TO-LEFT OVERRIDE"},
		{"bidi-isolate", "x ⁦ y\n", true, "U+2066 LEFT-TO-RIGHT ISOLATE"},
		{"word-joiner", "a⁠b\n", true, "U+2060 WORD JOINER"},
		{"nul-control", "a\x00b\n", true, "U+0000 NULL"},
		{"c1-control-nel", "ab\n", true, "CONTROL CHARACTER"},
		{"non-leading-bom", "ok\ufeffno\n", true, "U+FEFF ZERO WIDTH NO-BREAK SPACE"},
		{"invalid-utf8-byte", "a\xffb\n", true, "not valid UTF-8"},
		// The wider Cf-category coverage: the bidi family the check's title names
		// (LRM/RLM/ALM), soft hyphen, invisible math operators, the Unicode Tag
		// block (the canonical LLM ASCII-smuggling vector), and variation selectors
		// (Mn, not Cf \u2014 covered explicitly). Each must be flagged.
		{"lrm-mark", "abc\u200edef\n", true, "U+200E LEFT-TO-RIGHT MARK"},
		{"rlm-mark", "abc\u200fdef\n", true, "U+200F RIGHT-TO-LEFT MARK"},
		{"alm-mark", "abc\u061cdef\n", true, "U+061C ARABIC LETTER MARK"},
		{"soft-hyphen", "ab\u00adcd\n", true, "U+00AD SOFT HYPHEN"},
		{"invisible-times", "a\u2062b\n", true, "U+2062 INVISIBLE TIMES"},
		{"tag-block-char", "hi\U000e0041there\n", true, "TAG CHARACTER"},
		{"tag-block-cancel", "hi\U000e007fthere\n", true, "TAG CHARACTER"},
		{"variation-selector-16", "warn\ufe0f\n", true, "VARIATION SELECTOR"},
		{"variation-selector-supp", "x\U000e0100y\n", true, "VARIATION SELECTOR"},
		// legitimate content — must NOT be flagged. Printable non-ASCII is spelled
		// literally on purpose: it is exactly what must stay legal.
		{"clean-ascii", "just plain text\nwith two lines\n", false, ""},
		{"printable-non-ascii", "café — emoji ✅ arrows → box ─\n", false, ""},
		{"leading-bom-ok", "\ufeff# a real BOM at file start is legitimate\n", false, ""},
		{"tabs-and-crlf", "a\tb\r\nc\td\r\n", false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			rel := writeInScope(t, root, "t", tc.body)

			checked, issues, _, err := ScanInstructionSurfaces(root)
			if err != nil {
				t.Fatalf("unexpected scan error: %v", err)
			}
			if checked != 1 {
				t.Fatalf("checked = %d, want 1", checked)
			}
			if tc.wantHit {
				if len(issues) == 0 {
					t.Fatalf("expected a hidden-character issue, got none")
				}
				if issues[0].Path != rel {
					t.Errorf("issue path = %q, want %q", issues[0].Path, rel)
				}
				if !strings.Contains(issues[0].Msg, tc.wantInMsg) {
					t.Errorf("issue msg %q does not contain %q", issues[0].Msg, tc.wantInMsg)
				}
			} else if len(issues) != 0 {
				t.Errorf("legitimate content flagged: %v", issues)
			}
		})
	}
}

// TestScanInstructionSurfaces_MessageNamesLineAndCol pins the message shape the
// brief requires: file, line, column, and the codepoint. FAIL-FIRST: a scanHidden
// that dropped line/column tracking (e.g. reported only the codepoint) fails this.
func TestScanInstructionSurfaces_MessageNamesLineAndCol(t *testing.T) {
	root := t.TempDir()
	// The override sits on line 3, at rune column 5 ("abc " then the RLO).
	writeInScope(t, root, "t", "line one\nline two\nabc ‮\n")

	_, issues, _, err := ScanInstructionSurfaces(root)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %v", len(issues), issues)
	}
	msg := issues[0].Msg
	for _, want := range []string{"line 3", "col 5", "U+202E", "RIGHT-TO-LEFT OVERRIDE"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q missing %q", msg, want)
		}
	}
}

// TestScanInstructionSurfaces_BudgetNoticeIsAdvisory proves the budget half is a
// NOTICE, not a gate: an over-budget file yields a NOTICE line but ZERO issues,
// so the exit code is unmoved. FAIL-FIRST: wiring word count into issues (making
// it exit-affecting) reddens the "len(issues) == 0" assertion — this is the guard
// against the pre-mortem "NOTICE accidentally wired into the exit code".
func TestScanInstructionSurfaces_BudgetNoticeIsAdvisory(t *testing.T) {
	root := t.TempDir()
	rel := writeInScope(t, root, "big", strings.Repeat("word ", budgetThresholdSkillMd+500)+"\n")

	_, issues, notices, err := ScanInstructionSurfaces(root)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("budget overrun must not produce a hard issue, got %v", issues)
	}
	if len(notices) != 1 {
		t.Fatalf("want 1 NOTICE, got %d: %v", len(notices), notices)
	}
	n := notices[0]
	for _, want := range []string{"NOTICE", rel, "context-bloat candidate", "budget 3000"} {
		if !strings.Contains(n, want) {
			t.Errorf("notice %q missing %q", n, want)
		}
	}
}

// TestScanInstructionSurfaces_UnderBudgetIsSilent proves a file under threshold
// emits no NOTICE — the advisory does not cry wolf on every file.
func TestScanInstructionSurfaces_UnderBudgetIsSilent(t *testing.T) {
	root := t.TempDir()
	writeInScope(t, root, "small", "a short skill\n")

	_, _, notices, err := ScanInstructionSurfaces(root)
	if err != nil {
		t.Fatalf("unexpected scan error: %v", err)
	}
	if len(notices) != 0 {
		t.Errorf("under-budget file emitted a NOTICE: %v", notices)
	}
}

// TestBudgetThreshold_ClaudeMdHigher pins the two thresholds apart: CLAUDE.md
// carries a higher ceiling than a SKILL.md.
func TestBudgetThreshold_ClaudeMdHigher(t *testing.T) {
	if got := budgetThreshold("plugins/assay/skills/x/SKILL.md"); got != budgetThresholdSkillMd {
		t.Errorf("SKILL.md threshold = %d, want %d", got, budgetThresholdSkillMd)
	}
	if got := budgetThreshold("CLAUDE.md"); got != budgetThresholdClaudeMd {
		t.Errorf("CLAUDE.md threshold = %d, want %d", got, budgetThresholdClaudeMd)
	}
}

// TestScanInstructionSurfaces_EmptyRootFindsNothing proves the could-not-check
// contract: a root with no in-scope surface returns checked == 0 (which main
// turns into HIDDEN-CHARS: COULD-NOT-CHECK, exit 2 — never a silent pass).
func TestScanInstructionSurfaces_EmptyRootFindsNothing(t *testing.T) {
	root := t.TempDir()
	checked, issues, _, err := ScanInstructionSurfaces(root)
	if err != nil {
		t.Fatalf("empty root should not be a structural error, got %v", err)
	}
	if checked != 0 {
		t.Errorf("checked = %d, want 0 on an empty root", checked)
	}
	if len(issues) != 0 {
		t.Errorf("empty root produced issues: %v", issues)
	}
}

// TestScanInstructionSurfaces_RealRepoIsClean is the retro-scan (Verify row 4) as
// an enforcing test: the byte-level lint run over THIS repo's real instruction
// surfaces must find zero hidden characters. It is the standing counterpart to
// the synthetic table above — those prove the rule fires; this proves the shipped
// tree satisfies it, and reddens if a hidden-character payload ever lands.
//
// CROSS-MODULE READER: reads the repo root two directories above this module.
func TestScanInstructionSurfaces_RealRepoIsClean(t *testing.T) {
	const repoRoot = "../.."
	if _, err := os.Stat(filepath.Join(repoRoot, "plugins", "assay", "skills")); err != nil {
		t.Fatalf("cannot find %s/plugins/assay/skills — run from inside the assay checkout (%v)", repoRoot, err)
	}

	checked, issues, _, err := ScanInstructionSurfaces(repoRoot)
	if err != nil {
		t.Fatalf("byte-scan could not run over the real tree: %v", err)
	}
	if checked == 0 {
		t.Fatal("scanned 0 real instruction files — this check proved nothing")
	}
	for _, is := range issues {
		t.Errorf("%s: %s", is.Path, is.Msg)
	}
}
