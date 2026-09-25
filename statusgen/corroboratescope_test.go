package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Tests for the #1395 defect class: QUOTED NOTATION READ AS A CLAIM
// (corroboratescope.go). Stamp tokens are spelled with the HSTAMP sentinel and
// expanded at runtime, as the sibling corroborate tests do, so this source file
// carries no bare stamp literal. The fixture roster maps "alex" to login "ada".

func hstamp(s string) string { return strings.ReplaceAll(s, "HSTAMP", "human:") }

// quotedNotationDiff reproduces, with neutral names, the shapes a consumer PR's
// required --corroborate check reddened on: fixture DATA in a test script, stamp
// notation in workflow COMMENTS, and a ruling citation on the REMOVED side of a
// diff embedded in a committed .patch file. None of these lines is a claim.
func quotedNotationDiff() string {
	return hstamp(`+++ b/.github/scripts/gate-selfproof.test.sh
@@ -0,0 +1,3 @@
+# fixture: the row the gate is expected to write
+	revd="${today} HSTAMPalex"
+	[ "$(grep -o "HSTAMPalex" "$work/remote.md" | wc -l)" -eq 1 ]
+++ b/.github/workflows/gate.yml
@@ -0,0 +1,3 @@
+          # placeholder ` + "`HSTAMPreviewer`" + ` and, on its implemented-to-done step
+          # row's Reviewed cell carries the token ` + "`HSTAMPNAME`" + `
+          # substring, so ` + "`HSTAMPalex`" + ` does not match ` + "`HSTAMPalexa`" + `.
+++ b/.github/staged/gate-change.patch
@@ -0,0 +1,6 @@
+diff --git a/.github/workflows/gate.yml b/.github/workflows/gate.yml
+--- a/.github/workflows/gate.yml
++++ b/.github/workflows/gate.yml
+@@ -1,2 +1,2 @@
+-          # after alex's sign-off closed tr#12. This workflow is the sole
+-          # writer of the HSTAMPalex stamp
`)
}

// TestQuotedNotationIsNotAClaim is the behavioural fail-first for #1395: on the
// unfixed walker every line of quotedNotationDiff reads as a claim (five stamps and
// an unlinked citation, each MISSING-CORROBORATION); fixed, none does.
func TestQuotedNotationIsNotAClaim(t *testing.T) {
	d := quotedNotationDiff()
	if stamps := stampsInDiff("", d); len(stamps) != 0 {
		for _, s := range stamps {
			t.Errorf("quoted notation read as a stamp: human:%s in %s (%q)", s.Name, s.File, s.Line)
		}
	}
	for _, c := range citationsInDiff("", d) {
		if c.Source == ".github/staged/gate-change.patch" {
			t.Errorf("removed (-) side of an embedded patch read as a citation: %+v", c)
		}
	}
}

// TestQuotedNotationTruePositivesStillCount is the control: every narrowing is
// fail-closed, so a REAL claim beside each quoted shape is still read, still
// MISSING without an anchor, and still CORROBORATED by a real approval.
func TestQuotedNotationTruePositivesStillCount(t *testing.T) {
	d := hstamp(`+++ b/docs/streams/somestream/README.md
+| 01 | Real board row | 0 | S | done | 2026-09-01 opus | 2026-09-01 HSTAMPalex |
+- 2026-09-01 HSTAMPsam signed the runbook
+++ b/docs/streams/somestream/manifest.yaml
+approved-by: HSTAMPkim
+++ b/tools/Makefile
+	@echo HSTAMPlee
+++ b/.github/staged/board-change.patch
+@@ -1,2 +1,2 @@
+-| 02 | Old row | 0 | S | todo | - | - |
++| 02 | New row | 0 | S | done | 2026-09-01 opus | 2026-09-01 HSTAMPpat |
+ | 03 | Context row | 0 | S | done | 2026-09-01 opus | 2026-09-01 HSTAMPjo |
+++ b/scripts/flip-row.sh
+printf '| 03 | Row | 0 | S | done | x | 2026-09-01 HSTAMPlin |\n' >> docs/streams/s/README.md
+++ b/tools/writer/main.go
+	fmt.Println("| 04 | Row | 0 | S | done | x | 2026-09-01 HSTAMPmax |")
+++ b/testdata/fixture.sh
+	revd="2026-09-01 HSTAMPned"
+++ b/.github/staged/0001-flip-row.patch
+From 1111111111111111111111111111111111111111 Mon Sep 17 00:00:00 2001
+Subject: [PATCH] flip row 05
+
+- 2026-09-01 HSTAMPrin approved the flip
+---
+ docs/streams/s/README.md | 2 +-
+diff --git a/docs/streams/s/README.md b/docs/streams/s/README.md
+--- a/docs/streams/s/README.md
++++ b/docs/streams/s/README.md
+@@ -1 +1 @@
+-| 05 | Old row | 0 | S | todo | - | 2026-08-01 HSTAMPquin |
++| 05 | New row | 0 | S | done | x | - |
`)
	want := map[string]string{
		"alex": "docs/streams/somestream/README.md",     // a board row
		"sam":  "docs/streams/somestream/README.md",     // a Markdown bullet: "-" is only removed INSIDE a patch
		"kim":  "docs/streams/somestream/manifest.yaml", // a YAML value line, not a comment
		"lee":  "tools/Makefile",                        // an extension not on the closed inert list
		"pat":  ".github/staged/board-change.patch",     // the ADDED side of an embedded patch
		"jo":   ".github/staged/board-change.patch",     // the context side of an embedded patch
		// Program source is skipped only when it is a TEST file (pr1593-F1): a
		// non-test script or program that writes a stamp is still read, and a
		// directory name never makes a non-test file a test one.
		"lin": "scripts/flip-row.sh",
		"max": "tools/writer/main.go",
		"ned": "testdata/fixture.sh",
		// A `-` line OUTSIDE a hunk of an embedded patch — a format-patch
		// commit-message preamble — is not the removed side (pr1593-A1).
		"rin": ".github/staged/0001-flip-row.patch",
		// ("quin", on the removed side of that patch's hunk, is quoted: absent.)
	}
	got := map[string]string{}
	for _, s := range stampsInDiff("", d) {
		got[s.Name] = s.File
	}
	for name, file := range want {
		if got[name] != file {
			t.Errorf("true-positive stamp human:%s in %s was dropped (got file %q)", name, file, got[name])
		}
	}
	if len(got) != len(want) {
		t.Errorf("stamps = %v, want exactly %v", got, want)
	}

	// A real corroboration still counts: an APPROVED review by the mapped login
	// corroborates the board-row stamp; with no anchor it is MISSING.
	var board []stamp
	for _, s := range stampsInDiff("", d) {
		if s.Name == "alex" {
			board = append(board, s)
		}
	}
	if r := corroborateStamps(board, &ghPRData{}, "example-org/example", 7, nil); len(r) != 1 || r[0].Verdict != verdictMissing {
		t.Fatalf("board stamp with no anchor: got %+v, want one MISSING-CORROBORATION", r)
	}
	approved := &ghPRData{Reviews: []ghReview{{Author: ghAuthor{Login: "ada"}, State: "APPROVED", Id: "PRR_1"}}}
	if r := corroborateStamps(board, approved, "example-org/example", 7, nil); len(r) != 1 || r[0].Verdict != verdictCorroborated {
		t.Fatalf("board stamp with an APPROVED review by its mapped login: got %+v, want CORROBORATED", r)
	}

	// The citation lane keeps its whole scope: a ruling claim in program source or
	// a YAML comment is still read, and the ADDED side of an embedded patch too.
	cd := `+++ b/scripts/gate.sh
+# per alex's sign-off on #12 this branch is the sole writer
+++ b/.github/workflows/gate.yml
+  # after alex's sign-off closed #13.
+++ b/.github/staged/gate-change.patch
++  # after alex's sign-off closed example-org/tracker#14.
+++ b/docs/runbook.md
+- alex approved the rollback on #15
+++ b/.github/staged/0002-rollback.patch
+Subject: [PATCH] rollback
+
+- per alex's sign-off on #16 this rollback is approved
+---
+diff --git a/docs/runbook.md b/docs/runbook.md
+@@ -1 +1 @@
+- after alex's sign-off closed #17.
++ rolled back
`
	gotCit := map[int]string{}
	for _, c := range citationsInDiff("", cd) {
		gotCit[c.Number] = c.Source
	}
	for n, src := range map[int]string{12: "scripts/gate.sh", 13: ".github/workflows/gate.yml", 14: ".github/staged/gate-change.patch", 15: "docs/runbook.md", 16: ".github/staged/0002-rollback.patch"} {
		if gotCit[n] != src {
			t.Errorf("true-positive citation #%d in %s was dropped (got %q)", n, src, gotCit[n])
		}
	}
	if src, ok := gotCit[17]; ok {
		t.Errorf("citation #17 on the removed side of an embedded patch hunk was read (source %q)", src)
	}
	// A linked citation is corroborated by the named human acting on the cited artifact.
	for _, c := range citationsInDiff("", cd) {
		if c.Number != 14 {
			continue
		}
		fetched := map[string]*citedArtifact{"example-org/tracker#14": {Comments: []ghComment{{Author: ghAuthor{Login: "ada"}, Body: "ok"}}}}
		if r := corroborateCitations([]citation{c}, fetched, "example-org/example"); len(r) != 1 || r[0].Verdict != verdictCorroborated {
			t.Errorf("patch ADDED-side citation with the human's comment on the cited issue: got %+v, want CORROBORATED", r)
		}
	}
}

// TestQuotedNotationPlaceholderFormIsNotAStamp pins the `<name>` placeholder form
// on a record surface: humanStampRe needs a name character after the colon, so the
// placeholder is notation, not a stamp. (A DR decided-by placeholder is gated by
// the ruling lane, decisionruling.go, not by this diff scan.)
func TestQuotedNotationPlaceholderFormIsNotAStamp(t *testing.T) {
	d := hstamp("+++ b/docs/streams/somestream/README.md\n+| 04 | Row | 0 | S | todo | - | HSTAMP<name> |\n")
	if s := stampsInDiff("", d); len(s) != 0 {
		t.Errorf("placeholder form read as a stamp: %+v", s)
	}
}

// TestQuotedNotationNoticesAreVisible pins the visibility half: every stamp or
// citation a quoted line would have produced is announced as NOT-A-CLAIM, and a
// real claim is never listed there.
func TestQuotedNotationNoticesAreVisible(t *testing.T) {
	d := quotedNotationDiff() + hstamp("+++ b/docs/streams/somestream/README.md\n+| 01 | Row | 0 | S | done | x | 2026-09-01 HSTAMPkim |\n")
	joined := strings.Join(quotedClaimNotices("", d), "\n")
	for _, want := range []string{
		hstamp("HSTAMPalex in .github/scripts/gate-selfproof.test.sh NOT-A-CLAIM — test source (*.test.sh)"),
		hstamp("HSTAMPreviewer in .github/workflows/gate.yml NOT-A-CLAIM — YAML # line"),
		hstamp("HSTAMPalexa in .github/workflows/gate.yml NOT-A-CLAIM"),
		hstamp("HSTAMPalex in .github/staged/gate-change.patch NOT-A-CLAIM — ") + reasonEmbeddedPatchRemoved,
		"citation of alex in .github/staged/gate-change.patch NOT-A-CLAIM — " + reasonEmbeddedPatchRemoved,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("notice missing: %q\n--- notices ---\n%s", want, joined)
		}
	}
	if strings.Contains(joined, hstamp("HSTAMPkim")) {
		t.Errorf("a real board-row stamp was listed as NOT-A-CLAIM:\n%s", joined)
	}
}

// TestIsTestSourceFile pins the stamp lane's test-file recognition (pr1593-F1): a
// closed program-source extension AND a test-file NAME convention on the stem. A
// directory name never qualifies a file, and a non-test script never does.
func TestIsTestSourceFile(t *testing.T) {
	for file, want := range map[string]bool{
		".github/scripts/gate-selfproof.test.sh": true,
		"statusgen/corroborate_test.go":          true,
		"web/src/board.test.ts":                  true,
		"tools/lint_test.py":                     true,
		"web/Board.Test.JS":                      true,
		"scripts/flip-row.sh":                    false, // non-test script
		"tools/writer/main.go":                   false, // non-test program
		"testdata/fixture.sh":                    false, // a directory is not a name convention
		"tests/run.sh":                           false,
		"fixtures/rows.go":                       false,
		"scripts/latest.sh":                      false, // "test" is not "_test" / ".test"
		"docs/notes_test.md":                     false, // extension not on the closed list
		"ci/gate.test.yaml":                      false,
		"tools/check_test":                       false, // extensionless
		"pkg/_test.go":                           false, // an empty stem is not a test file
	} {
		if got, _ := isTestSourceFile(file); got != want {
			t.Errorf("isTestSourceFile(%q) = %v, want %v", file, got, want)
		}
	}
}

// TestCorroborateDiffWalkersShareOneWalker is the CLASS GUARD. The defect lived in
// a loop copied into each --corroborate lane, so fixing one copy left the others
// reading quoted notation as claims. This test parses every non-test source file
// in the package and fails if the added-line idiom — a strings.HasPrefix against a
// "+++" diff-header literal — appears in any function other than the one shared
// walker. Planting a second hand-rolled walker (a new lane, or an old lane's loop
// restored) reddens it; the lane must read through addedDiffLines instead.
//
// Its reach is that ONE idiom, the one every copied lane loop used: it guards
// against the old loop being copied back, and a walker spelled some other way is
// not detected. It is a correctness guard for the class, not a proof that no
// second walker exists — review still owns that.
func TestCorroborateDiffWalkersShareOneWalker(t *testing.T) {
	const allowed = "walkAddedDiffLines"
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var offenders []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		af, err := parser.ParseFile(fset, f, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		for _, decl := range af.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) != 2 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "HasPrefix" {
					return true
				}
				lit, ok := call.Args[1].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				v, err := strconv.Unquote(lit.Value)
				if err != nil || !strings.HasPrefix(v, "+++") {
					return true
				}
				if fn.Name.Name != allowed {
					offenders = append(offenders, fset.Position(call.Pos()).String()+" in "+fn.Name.Name)
				}
				return true
			})
		}
	}
	for _, o := range offenders {
		t.Errorf("hand-rolled diff walker at %s — read added lines through addedDiffLines (corroboratescope.go) so quoted notation is scoped once (#1395)", o)
	}
}
