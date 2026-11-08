package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// loadLPRoot copies the latest-PASS close fixtures (testdata/closelatestpass)
// into a temp root.
func loadLPRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/closelatestpass")); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestCloseStampsLatestPassRun pins the one-step implemented → done close to
// the run verifyflip reads: the Verified stamp names the latest strict PASS
// run, never the first Date/Runner table in Evidence; a latest verdict that is
// not a PASS, or no PASS at all, refuses with no stamp.
// regression: #5229
func TestCloseStampsLatestPassRun(t *testing.T) {
	now := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	const want = "2026-07-20 opus-verifier @ 2222222bbbbb"
	for _, tc := range []struct {
		name, brief, stamp string
		refuse             []string
	}{
		{"earlier FAIL run, later PASS run", "lp/01", want, nil},
		{"one PASS run", "lp/02", want, nil},
		{"earlier below-floor PASS, later PASS", "lp/05", want, nil},
		{"PASS run, later FAIL run", "lp/03", "", []string{"lp/03", "VERIFY: FAIL, not PASS", "latest strict **VERIFY: PASS** run only"}},
		{"no PASS run", "lp/04", "", []string{"lp/04", "has no **VERIFY: PASS** marker"}},
		{"verdict-first FAIL run, later loose PASS run", "lp/06", "2026-07-20 opus-verifier", nil},
		{"loose PASS run whose rows disagree", "lp/07", "", []string{"lp/07", "no verifier date/runner"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := loadLPRoot(t)
			_, out, _, err := closeVerifyFlipPlan(root, tc.brief, now)
			if tc.refuse != nil {
				if err == nil {
					t.Fatalf("close of %s must refuse; planned:\n%s", tc.brief, out)
				}
				for _, w := range tc.refuse {
					if !strings.Contains(err.Error(), w) {
						t.Errorf("refusal must name %q; got %q", w, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("close of %s refused: %v", tc.brief, err)
			}
			if !strings.Contains(string(out), "| done | "+tc.stamp+" | 2026-07-25 ") {
				t.Errorf("close of %s did not stamp %q:\n%s", tc.brief, tc.stamp, out)
			}
		})
	}
}

// TestGateCardNamesLatestPassRun pins the sign-off card's "recorded model
// verify pass" to the run the close stamps.
// regression: #5229
func TestGateCardNamesLatestPassRun(t *testing.T) {
	root := loadLPRoot(t)
	for _, f := range []string{"brief-01-fail-then-pass.md", "brief-05-pass-then-pass.md"} {
		bf, ok, err := parseBriefFile(root + "/docs/streams/lp/" + f)
		if err != nil || !ok {
			t.Fatalf("parse %s: ok=%v err=%v", f, ok, err)
		}
		body := renderImplementedGateBody(root, bf, false, gateCard{})
		if !strings.Contains(body, "(Evidence: `2026-07-20 opus-verifier @ 2222222bbbbb`)") || strings.Contains(body, "(Evidence: `2026-07-18") {
			t.Errorf("%s: card does not name the latest PASS run:\n%s", f, body)
		}
	}
}

// TestStampFallbackNamesLatestRun pins evidenceVerifierInfo's fallback (the
// latest PASS run's rows lack `@ sha`, so flipLatestPass refuses) to the run
// flipLatestPass would select: an earlier entry's rows inside the span are
// never stamped, and a run whose rows do not read as one run stamps nothing.
// regression: #5229
func TestStampFallbackNamesLatestRun(t *testing.T) {
	tbl := "| # | Result | Date | Runner |\n|---|---|---|---|\n"
	row := func(n, date, runner string) string { return "| " + n + " | ok | " + date + " | " + runner + " |\n" }
	for _, tc := range []struct{ name, ev, date, runner string }{
		{"verdict-first FAIL entry above the PASS entry",
			"### Run — 2026-07-18\n\n**VERIFY: FAIL**\n\n" + tbl + row("1", "2026-07-18", "opus-verifier") +
				"\n### Run — 2026-07-20\n\n" + tbl + row("1", "2026-07-20", "opus-verifier") + "\n**VERIFY: PASS**\n",
			"2026-07-20", "opus-verifier"},
		{"one loose run", "### Run\n\n" + tbl + row("1", "2026-07-20", "opus-verifier") + row("2", "2026-07-20", "opus-verifier") + "\n**VERIFY: PASS**\n",
			"2026-07-20", "opus-verifier"},
		{"rows disagree on the date", "### Run\n\n" + tbl + row("1", "2026-07-20", "opus-verifier") + row("2", "2026-07-21", "opus-verifier") + "\n**VERIFY: PASS**\n", "", ""},
		{"rows disagree on the runner", "### Run\n\n" + tbl + row("1", "2026-07-20", "opus-verifier") + row("2", "2026-07-20", "sonnet-verifier") + "\n**VERIFY: PASS**\n", "", ""},
		{"strict and loose rows mixed", "### Run\n\n" + tbl + row("1", "2026-07-20", "opus-verifier @ 2222222bbbbb") + row("2", "2026-07-20", "opus-verifier") + "\n**VERIFY: PASS**\n", "", ""},
		{"a placeholder date", "### Run\n\n" + tbl + row("1", "—", "opus-verifier") + row("2", "2026-07-20", "opus-verifier") + "\n**VERIFY: PASS**\n", "", ""},
		{"not a calendar date", "### Run\n\n" + tbl + row("1", "2026-02-30", "opus-verifier") + "\n**VERIFY: PASS**\n", "", ""},
		{"a future date", "### Run\n\n" + tbl + row("1", "2999-01-01", "opus-verifier") + "\n**VERIFY: PASS**\n", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, r := evidenceVerifierInfo(tc.ev)
			if d != tc.date || r != tc.runner {
				t.Errorf("evidenceVerifierInfo = (%q, %q), want (%q, %q)", d, r, tc.date, tc.runner)
			}
		})
	}
}

// TestGateCardOnlyWhereCloseCanStamp pins the implemented-gate card to the
// close it offers: a record whose latest PASS run cannot stamp the Verified
// cell (lp/03: a later FAIL; lp/04: no PASS; lp/07: rows that do not read as
// one run) gets no card, since its one-step close refuses.
// regression: #5229
func TestGateCardOnlyWhereCloseCanStamp(t *testing.T) {
	root := loadLPRoot(t)
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, iss := range verifyIssues(root, streams, map[string]bool{}) {
		got[iss.Brief] = true
	}
	for _, b := range []string{"lp/01", "lp/02", "lp/05", "lp/06"} {
		if !got[b] {
			t.Errorf("%s: no sign-off card for a record its close stamps", b)
		}
	}
	for _, b := range []string{"lp/03", "lp/04", "lp/07"} {
		if got[b] {
			t.Errorf("%s: sign-off card raised for a record its close refuses", b)
		}
	}
}

// stampRowReaders are the only functions outside tests that may call the raw
// Date/Runner row reader flipDateRunnerRows. Each reads lines a run selector
// already bounded, or asks only whether a span holds such a row; none names a
// verifier run from the whole Evidence section. A new reader of "the recorded
// verify pass" takes its run from latestPassSpan / flipLatestPass — it does
// not join this list (#5229).
var stampRowReaders = map[string]bool{
	"latestPassSpan":       true, // rows after the latest PASS: a refusal
	"flipReadRun":          true, // the span latestPassSpan bounds
	"evidenceVerifierInfo": true, // the run latestPassRunFrom bounds
	"flipReadLooseRun":     true, // the run latestPassRunFrom bounds
	"coverageCrossSource":  true, // flipLatestPass's own run
	"heldCreditedAbove":    true, // existence only, never a runner
	"heldRunDated":         true, // existence and completeness only
}

// rawRowReaderCalls returns "<func>" for every function in src (one Go file)
// that calls flipDateRunnerRows and is not in stampRowReaders.
func rawRowReaderCalls(t *testing.T, name, src string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var bad []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil || stampRowReaders[fn.Name.Name] {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "flipDateRunnerRows" {
					bad = append(bad, name+": "+fn.Name.Name)
					return false
				}
			}
			return true
		})
	}
	return bad
}

// TestOneVerifierRunSelector closes the defect class behind #5229: two
// readers of one Evidence section that pick "the recorded verify pass"
// differently (the close stamp read the first Date/Runner row, verifyflip the
// latest PASS run). Every caller of the raw row reader is a listed reader of
// an already-selected span; a planted whole-Evidence reader is flagged (the
// positive control below), so a broken matcher fails instead of passing.
// regression: #5229
func TestOneVerifierRunSelector(t *testing.T) {
	const planted = "package main\nfunc plantedFirstRunner(e string) string {\n\trows, _ := flipDateRunnerRows(strings.Split(e, \"\\n\"))\n\treturn rows[0][1]\n}\n"
	if got := rawRowReaderCalls(t, "planted.go", planted); len(got) != 1 || got[0] != "planted.go: plantedFirstRunner" {
		t.Fatalf("positive control: planted whole-Evidence reader not flagged; got %v", got)
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range rawRowReaderCalls(t, f, string(src)) {
			t.Errorf("%s calls flipDateRunnerRows outside the run selectors — take the run from latestPassSpan / flipLatestPass instead", b)
		}
	}
}
