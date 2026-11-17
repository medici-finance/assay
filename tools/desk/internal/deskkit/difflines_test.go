package deskkit

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"
)

// difflines_test.go — DiffLineIndex (#3196): the stripped text a branch-diff scan
// reads, and the index that maps each of its lines back to a file, a line in it and a side.

// indexDiffs are the diffs TestDiffLineIndex pins and TestAddedLinesParity reads: one
// multi-file, multi-hunk diff carrying a new file without a trailing newline, a deleted
// file, a rename with an edit and a malformed hunk header, and one diff whose hunk runs
// past maxPlacedLine.
var indexDiffs = map[string]string{
	"multi": strings.Join([]string{
		"diff --git a/one.txt b/one.txt",
		"index 1111111..2222222 100644",
		"--- a/one.txt",
		"+++ b/one.txt",
		"@@ -10,4 +10,5 @@ func One() {",
		" c10",
		"-o11",
		"+n11",
		"+n12",
		" c12",
		"@@ -40,3 +41,3 @@",
		" c40",
		"-o41",
		"+n42",
		"diff --git a/two.txt b/two.txt",
		"new file mode 100644",
		"index 0000000..3333333",
		"--- /dev/null",
		"+++ b/two.txt",
		"@@ -0,0 +1,2 @@",
		"+first",
		"+second",
		`\ No newline at end of file`,
		"diff --git a/gone.txt b/gone.txt",
		"deleted file mode 100644",
		"index 4444444..0000000",
		"--- a/gone.txt",
		"+++ /dev/null",
		"@@ -1,2 +0,0 @@",
		"-g1",
		"-g2",
		"diff --git a/old/name.txt b/new/name.txt",
		"similarity index 80%",
		"rename from old/name.txt",
		"rename to new/name.txt",
		"index 5555555..6666666 100644",
		"--- a/old/name.txt",
		"+++ b/new/name.txt",
		"@@ -5,3 +5,3 @@",
		" r5",
		"-r6",
		"+r6b",
		" r7",
		"diff --git a/bad.txt b/bad.txt",
		"index 7777777..8888888 100644",
		"--- a/bad.txt",
		"+++ b/bad.txt",
		"@@ malformed @@",
		"+x",
		"-y",
		" z",
		"",
	}, "\n"),
	"far": strings.Join([]string{
		"diff --git a/far.txt b/far.txt",
		"index 1111111..2222222 100644",
		"--- a/far.txt",
		"+++ b/far.txt",
		"@@ -199998,3 +199999,3 @@",
		" f1",
		"-f2",
		"+f3",
		" f4",
		"+f5",
		"",
	}, "\n"),
}

type wantRef struct {
	text string
	path string
	side DiffSide
	line int
}

// TestDiffLineIndex pins every kept line's text, path, side and file line.
func TestDiffLineIndex(t *testing.T) {
	cases := map[string][]wantRef{
		"multi": {
			{"@@ -10,4 +10,5 @@", "one.txt", "", 0},
			{"c10", "one.txt", DiffSideContext, 10},
			{"o11", "one.txt", DiffSideRemoved, 11},
			{"n11", "one.txt", DiffSideAdded, 11},
			{"n12", "one.txt", DiffSideAdded, 12},
			{"c12", "one.txt", DiffSideContext, 13},
			{"@@ -40,3 +41,3 @@", "one.txt", "", 0},
			{"c40", "one.txt", DiffSideContext, 41},
			{"o41", "one.txt", DiffSideRemoved, 41},
			{"n42", "one.txt", DiffSideAdded, 42},
			{"@@ -0,0 +1,2 @@", "two.txt", "", 0},
			{"first", "two.txt", DiffSideAdded, 1},
			{"second", "two.txt", DiffSideAdded, 2},
			{`\ No newline at end of file`, "two.txt", "", 0},
			{"@@ -1,2 +0,0 @@", "", "", 0},
			{"g1", "gone.txt", DiffSideRemoved, 1},
			{"g2", "gone.txt", DiffSideRemoved, 2},
			{"@@ -5,3 +5,3 @@", "new/name.txt", "", 0},
			{"r5", "new/name.txt", DiffSideContext, 5},
			{"r6", "old/name.txt", DiffSideRemoved, 6},
			{"r6b", "new/name.txt", DiffSideAdded, 6},
			{"r7", "new/name.txt", DiffSideContext, 7},
			{"@@ malformed @@", "bad.txt", "", 0},
			{"x", "bad.txt", DiffSideAdded, 0},
			{"y", "bad.txt", DiffSideRemoved, 0},
			{"z", "bad.txt", DiffSideContext, 0},
			{"", "bad.txt", "", 0},
		},
		"far": {
			{"@@ -199998,3 +199999,3 @@", "far.txt", "", 0},
			{"f1", "far.txt", DiffSideContext, 199999},
			{"f2", "far.txt", DiffSideRemoved, 199999},
			{"f3", "far.txt", DiffSideAdded, 200000},
			{"f4", "far.txt", DiffSideContext, 0},
			{"f5", "far.txt", DiffSideAdded, 0},
			{"", "far.txt", "", 0},
		},
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			stripped, refs := DiffLineIndex(indexDiffs[name])
			lines := strings.Split(stripped, "\n")
			if len(lines) != len(refs) {
				t.Fatalf("%d stripped lines but %d refs", len(lines), len(refs))
			}
			if len(lines) != len(want) {
				t.Fatalf("%d stripped lines, want %d:\n%s", len(lines), len(want), stripped)
			}
			for i, w := range want {
				got := wantRef{lines[i], refs[i].Path, refs[i].Side, refs[i].FileLine}
				if got != w {
					t.Errorf("stripped line %d = %+v, want %+v", i+1, got, w)
				}
			}
		})
	}

	// Text with no `diff --git` line is not git output: kept whole, nothing located.
	doc := "--- a/foo\n+++ b/foo\n@@ -1 +1 @@\n-old\n+new"
	stripped, refs := DiffLineIndex(doc)
	if stripped != doc {
		t.Errorf("a doc fragment must pass through unchanged, got:\n%s", stripped)
	}
	for i, r := range refs {
		if r != (DiffLineRef{}) {
			t.Errorf("doc fragment line %d located as %+v, want nothing", i+1, r)
		}
	}
}

// legacyStripDiffMetaLines is a frozen, verbatim copy of the meta-line strip deskpr ran
// before the #3196 fix moved it into DiffLineIndex. It is the oracle that proves the bytes
// the branch-diff scan reads did not change. Never edit it to match the index.
func legacyStripDiffMetaLines(diff string) string {
	lines := strings.Split(diff, "\n")
	kept := make([]string, 0, len(lines))
	inHeader, isGitOutput := false, false
	for _, ln := range lines {
		if strings.HasPrefix(ln, "diff --git ") {
			inHeader, isGitOutput = true, true
			continue
		}
		if inHeader && strings.HasPrefix(ln, "@@ ") {
			inHeader = false
		}
		if inHeader {
			continue
		}
		if isGitOutput {
			if m := legacyHunkHeader.FindStringSubmatch(ln); m != nil {
				ln = m[1]
			} else if len(ln) > 0 && (ln[0] == '+' || ln[0] == '-' || ln[0] == ' ') {
				ln = ln[1:]
			}
		}
		kept = append(kept, ln)
	}
	return strings.Join(kept, "\n")
}

var legacyHunkHeader = regexp.MustCompile(`^(@@ -[0-9]+(?:,[0-9]+)? \+[0-9]+(?:,[0-9]+)? @@)`)

// stripCases are the inputs deskpr's own strip tests use (TestStripDiffMetaLines, the
// hunk-collision, doc-fragment, funcname and funcname-only-tail cases), with every planted
// run replaced by a generated one.
func stripCases() map[string]string {
	run := seededRun(rand.New(rand.NewSource(62)), 40)
	return map[string]string{
		"meta lines": strings.Join([]string{
			"diff --git a/tools/desk/internal/deskkit/config.go b/tools/desk/internal/deskkit/config.go",
			"index e69de29..4b825dc 100644",
			"--- a/tools/desk/internal/deskkit/config.go",
			"+++ b/tools/desk/internal/deskkit/config.go",
			"similarity index 100%",
			"rename from old/tools/desk/internal/deskkit/config.go",
			"rename to tools/desk/internal/deskkit/config.go",
			"@@ -0,0 +1 @@",
			"+package deskkit",
			" context line unchanged",
			"-removed line",
		}, "\n"),
		"hunk collision": strings.Join([]string{
			"diff --git a/leak.txt b/leak.txt",
			"new file mode 100644",
			"index 0000000..abc1234",
			"--- /dev/null",
			"+++ b/leak.txt",
			"@@ -0,0 +1 @@",
			"+ ++ " + run,
		}, "\n"),
		"doc fragment": strings.Join([]string{
			"The diff looked like:", "```", "--- a/foo", "+++ b/foo", "@@ -1 +1 @@", "-old", "+new", "```",
		}, "\n"),
		"funcname": strings.Join([]string{
			"diff --git a/.assay-versions b/.assay-versions",
			"index 9c77ca2..f23d684 100644",
			"--- a/.assay-versions",
			"+++ b/.assay-versions",
			"@@ -27,5 +27,5 @@ statusgen statusgen/v0.6.0 " + run,
			"-statusgen-darwin-arm64 statusgen/v0.6.0 " + run,
			"+statusgen-darwin-arm64 statusgen/v0.7.0 " + run,
		}, "\n"),
		"funcname doc": "The diff looked like:\n@@ -1,2 +1,2 @@ func Example() {\n-old\n+new\n",
		"funcname only tail": strings.Join([]string{
			"diff --git a/notes.txt b/notes.txt",
			"--- a/notes.txt",
			"+++ b/notes.txt",
			"@@ -1,3 +1,3 @@ func Example() {",
			"+@@ not a header, just content " + run,
			" @@ context line that begins with at-at " + run,
			"@@ malformed header " + run,
		}, "\n"),
	}
}

// TestDiffLineIndexStrips proves DiffLineIndex's text is byte-identical to what deskpr
// scanned before: over the index diffs, deskpr's strip-test inputs and a seeded stream of
// generated diff-shaped text.
func TestDiffLineIndexStrips(t *testing.T) {
	inputs := map[string]string{}
	for k, v := range indexDiffs {
		inputs["index "+k] = v
	}
	for k, v := range stripCases() {
		inputs["strip "+k] = v
	}
	r := rand.New(rand.NewSource(6200))
	for i := 0; i < 300; i++ {
		inputs["generated "+string(rune('a'+i%26))+strings.Repeat("'", i/26)] = generatedDiff(r)
	}
	for name, in := range inputs {
		got, refs := DiffLineIndex(in)
		if want := legacyStripDiffMetaLines(in); got != want {
			t.Errorf("%s: stripped text differs from the legacy strip\nwant:\n%q\ngot:\n%q", name, want, got)
		}
		// Every kept line has one ref; a diff whose every line is a header keeps none.
		if n := strings.Count(got, "\n") + 1; n != len(refs) && !(got == "" && len(refs) == 0) {
			t.Errorf("%s: %d stripped lines but %d refs", name, n, len(refs))
		}
	}
}

// generatedDiff builds diff-shaped text from fragments that exercise every branch of the
// strip: file headers, well-formed and malformed hunk headers, funcname tails, content on
// every side, header look-alikes inside hunks, no-newline markers and blank lines.
func generatedDiff(r *rand.Rand) string {
	frags := []func() string{
		func() string { return "diff --git a/f" + seededRun(r, 3) + " b/f" + seededRun(r, 3) },
		func() string { return "index 1234567..89abcde 100644" },
		func() string { return "--- a/f" + seededRun(r, 3) },
		func() string { return "+++ b/f" + seededRun(r, 3) },
		func() string { return "--- /dev/null" },
		func() string { return "+++ /dev/null" },
		func() string { return "rename from x" },
		func() string {
			return "@@ -" + digits(r) + "," + digits(r) + " +" + digits(r) + "," + digits(r) + " @@"
		},
		func() string { return "@@ -" + digits(r) + " +" + digits(r) + " @@ func " + seededRun(r, 5) },
		func() string { return "@@ malformed " + seededRun(r, 4) },
		func() string { return "+" + seededRun(r, 1+r.Intn(12)) },
		func() string { return "-" + seededRun(r, 1+r.Intn(12)) },
		func() string { return " " + seededRun(r, 1+r.Intn(12)) },
		func() string { return "+++ " + seededRun(r, 4) },
		func() string { return "--- " + seededRun(r, 4) },
		func() string { return `\ No newline at end of file` },
		func() string { return "" },
		func() string { return seededRun(r, 6) },
	}
	n := 1 + r.Intn(40)
	out := make([]string, n)
	for i := range out {
		out[i] = frags[r.Intn(len(frags))]()
	}
	return strings.Join(out, "\n")
}

func digits(r *rand.Rand) string { return string(rune('0' + r.Intn(10))) }

// seededRun returns n characters drawn from a fixed alphabet by r. Tests never carry a
// credential-shaped literal; every planted run comes from a fixed seed.
func seededRun(r *rand.Rand, n int) string {
	// Built from short pieces so the alphabet is not itself a long high-entropy run.
	const alphabet = "ABCDEFGHJKLMN" + "PQRSTUVWXYZ" + "abcdefghijk" + "mnopqrstuvwxyz" + "23456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[r.Intn(len(alphabet))]
	}
	return string(b)
}

// TestAddedLinesParity runs AddedLinesByFile and the index over every diff
// TestDiffLineIndex covers and asserts they name the same files and place the same added
// lines at the same new-side line numbers. The reconstruction below applies
// AddedLinesByFile's own placement rule (placed at FileLine when it is next in order,
// appended after the placed lines otherwise) to the index's added refs.
func TestAddedLinesParity(t *testing.T) {
	for name, diff := range indexDiffs {
		t.Run(name, func(t *testing.T) {
			want := map[string]string{}
			for _, fa := range AddedLinesByFile(diff) {
				if strings.TrimSpace(fa.Text) != "" {
					want[fa.Path] = fa.Text
				}
			}
			got := addedTextFromIndex(diff)
			for p, txt := range got {
				if strings.TrimSpace(txt) == "" {
					delete(got, p)
				}
			}
			if len(got) != len(want) {
				t.Errorf("index names %d files with added lines, AddedLinesByFile %d\nindex: %q\nouter: %q",
					len(got), len(want), got, want)
			}
			for p, w := range want {
				if g, ok := got[p]; !ok {
					t.Errorf("index has no added lines for %s", p)
				} else if g != w {
					t.Errorf("%s: added-line placement differs\nAddedLinesByFile: %q\nindex:            %q", p, w, g)
				}
			}
		})
	}
}

func addedTextFromIndex(diff string) map[string]string {
	stripped, refs := DiffLineIndex(diff)
	lines := strings.Split(stripped, "\n")
	placed := map[string][]string{}
	overflow := map[string][]string{}
	for i, r := range refs {
		if r.Side != DiffSideAdded || r.Path == "" {
			continue
		}
		pl := placed[r.Path]
		if r.FileLine >= 1 {
			for len(pl) < r.FileLine-1 {
				pl = append(pl, "")
			}
			if len(pl) == r.FileLine-1 {
				pl = append(pl, lines[i])
			} else {
				overflow[r.Path] = append(overflow[r.Path], lines[i])
			}
		} else {
			overflow[r.Path] = append(overflow[r.Path], lines[i])
		}
		placed[r.Path] = pl
	}
	out := map[string]string{}
	for p := range placed {
		out[p] = strings.Join(append(placed[p], overflow[p]...), "\n")
	}
	return out
}
