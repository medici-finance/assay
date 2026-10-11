package testledger_test

import (
	"archive/tar"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/medici-finance/assay/tools/desk/internal/testledger"
)

// Test-only flags: `go test ./internal/testledger/ -run TestReportTestLedger -v -args
// -base=<rev|dir> -head=<rev|dir>`. There is no default range: on a merged checkout
// `git merge-base origin/main HEAD` is HEAD itself (#1657), so the caller names the base.
var (
	baseFlag = flag.String("base", "", "the earlier tree: a git revision (read via `git archive`) or a directory")
	headFlag = flag.String("head", "", "the later tree: a git revision (read via `git archive`) or a directory")
)

func fixtureLines(t *testing.T) ([]string, []testledger.Retirement) {
	t.Helper()
	base, err := testledger.Tests(os.DirFS("testdata/base"))
	if err != nil {
		t.Fatalf("Tests(testdata/base): %v", err)
	}
	headFS := os.DirFS("testdata/head")
	head, err := testledger.Tests(headFS)
	if err != nil {
		t.Fatalf("Tests(testdata/head): %v", err)
	}
	if len(base) != 6 || len(head) != 5 {
		t.Fatalf("fixture test counts: base %d (want 6: TestMain, Testhelper and the decoy excluded), head %d (want 5)", len(base), len(head))
	}
	log, err := os.ReadFile("testdata/log.txt")
	if err != nil {
		t.Fatalf("read log.txt: %v", err)
	}
	retired := testledger.ParseTrailers(string(log))
	rows := func(name string) []testledger.Row {
		rs, err := testledger.RowsNaming(headFS, name)
		if err != nil {
			t.Fatalf("RowsNaming(%s): %v", name, err)
		}
		return rs
	}
	at := func(testledger.TestFunc) string { return "fixture" }
	return testledger.Lines(testledger.Diff(base, head), retired, at, rows), retired
}

// TestLedgerFixture asserts the exact report over testdata/{base,head}: head deletes one
// tagged test (reported untrailed, with its tag) and one untagged test (covered by log.txt's
// trailer, so reported as trailed with the trailer's commit and reason, never hidden), renames
// one with an identical body and one with the same tag and a changed body, keeps two and adds
// one. TestMain, a lower-case Testhelper and a TestDecoy in a non-test .go file all leave too,
// and none of them may appear.
func TestLedgerFixture(t *testing.T) {
	lines, retired := fixtureLines(t)
	want := []string{
		"retired-untrailed: pkg/parse.TestAlpha [regression F-fixture-alpha] deleted in fixture",
		`retired-trailed: pkg/parse.TestBeta deleted in fixture; Retires-test in 111111111111: "the length check moved into TestEpsilon's table"`,
		"renamed-untrailed: pkg/parse.TestDelta → TestDeltaRewritten (same tag) in fixture",
		"renamed-untrailed: pkg/parse.TestGamma → TestGammaRenamed (body identical) in fixture",
		"verify-rows-naming: TestAlpha → docs/streams/fixture/brief-01-parse.md:1",
		"verify-rows-naming: TestBeta → docs/streams/fixture/brief-01-parse.md:5",
		"verify-rows-naming: TestGamma → docs/streams/fixture/brief-01-parse.md:2",
	}
	if got, w := strings.Join(lines, "\n"), strings.Join(want, "\n"); got != w {
		t.Errorf("report:\n%s\nwant:\n%s", got, w)
	}
	if len(retired) != 1 || retired[0].Test != "TestBeta" || retired[0].Commit != strings.Repeat("1", 40) {
		t.Errorf("trailers: got %+v, want exactly TestBeta at commit 1111… (the separator-less one dropped)", retired)
	}

	src := func(fns ...string) *fstest.MapFile {
		return &fstest.MapFile{Data: []byte("package p\n\nimport \"testing\"\n\n" + strings.Join(fns, "\n"))}
	}

	t.Run("prose after the tag prefix is not a tag", func(t *testing.T) {
		// A doc comment that wraps a sentence onto a line starting `regression: ` is prose:
		// only a line holding grammar refs (`#<N>`, `F-<slug>`, `class #<N>`) and nothing
		// else is a tag line. The first case is the seeded deskdispatch test's shape.
		fns, err := testledger.Tests(fstest.MapFS{"p/p_test.go": src(
			"// TestWrapped is the headline\n// regression: a verifier dispatched from a shared checkout carries an UNRELATED\n// identity.\n// regression: #1490\nfunc TestWrapped(t *testing.T) {}\n",
			"// TestProse is the durable-coverage\n// regression: it iterates EVERY codepoint in the set\nfunc TestProse(t *testing.T) {}\n",
			"// regression: #12 and the follow-up\nfunc TestTrailingProse(t *testing.T) {}\n",
			"// regression: F-Bad_Slug\nfunc TestBadSlug(t *testing.T) {}\n",
			"// regression: class 12\nfunc TestBadClass(t *testing.T) {}\n",
			"// regression: class #1x\nfunc TestBadClassNum(t *testing.T) {}\n",
			"// regression: #\nfunc TestNoNumber(t *testing.T) {}\n",
			"// regression: #07\nfunc TestLeadingZero(t *testing.T) {}\n",
			"// regression: #7,\nfunc TestTrailingComma(t *testing.T) {}\n",
			// Controls: every legal ref shape still reads, alone, listed, and over two lines.
			"// regression: #12, F-some-slug-2, class #34\nfunc TestListed(t *testing.T) {}\n",
			"// regression: F-fixture-alpha\n// regression: class #3\nfunc TestTwoLines(t *testing.T) {}\n",
		)})
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]string{
			"TestWrapped": "#1490", "TestProse": "", "TestTrailingProse": "", "TestBadSlug": "",
			"TestBadClass": "", "TestBadClassNum": "", "TestNoNumber": "", "TestLeadingZero": "",
			"TestTrailingComma": "", "TestListed": "#12, F-some-slug-2, class #34",
			"TestTwoLines": "F-fixture-alpha, class #3",
		}
		if len(fns) != len(want) {
			t.Fatalf("read %d tests, want %d", len(fns), len(want))
		}
		for _, f := range fns {
			if w, ok := want[f.Name]; !ok || f.Tag != w {
				t.Errorf("%s: tag %q, want %q", f.Name, f.Tag, w)
			}
		}
	})

	t.Run("shared tag is not a rename", func(t *testing.T) {
		// Two deleted tests share a tag and one added test carries it: no unique pairing,
		// so the report shows two deletions, never one rename that hides the other.
		base, err := testledger.Tests(fstest.MapFS{"p/p_test.go": src(
			"// regression: #7\nfunc TestOne(t *testing.T) { t.Log(1) }\n",
			"// regression: #7\nfunc TestTwo(t *testing.T) { t.Log(2) }\n",
		)})
		if err != nil {
			t.Fatal(err)
		}
		head, err := testledger.Tests(fstest.MapFS{"p/p_test.go": src(
			"// regression: #7\nfunc TestThree(t *testing.T) { t.Log(3) }\n",
		)})
		if err != nil {
			t.Fatal(err)
		}
		r := testledger.Diff(base, head)
		if len(r.Renamed) != 0 || len(r.Deleted) != 2 || len(r.Added) != 1 {
			t.Errorf("shared tag over-matched: renamed %d, deleted %d, added %d (want 0, 2, 1)", len(r.Renamed), len(r.Deleted), len(r.Added))
		}
	})
}

// TestTrailerGrammar pins the Retires-test: grammar: `<TestName> — <why>`, the rename form
// `<Old> — renamed <New>; <why>`, ` -- ` for the em dash, and the malformed shapes dropped.
func TestTrailerGrammar(t *testing.T) {
	cases := []struct {
		in            string
		ok            bool
		test, nn, why string
	}{
		{"TestFoo — superseded by TestBar", true, "TestFoo", "", "superseded by TestBar"},
		{"TestFoo -- superseded by TestBar", true, "TestFoo", "", "superseded by TestBar"},
		{"TestOld — renamed TestNew; the behaviour name changed", true, "TestOld", "TestNew", "the behaviour name changed"},
		{"BenchmarkScan — the hot path was deleted", true, "BenchmarkScan", "", "the hot path was deleted"},
		{"TestOld — renamed TestNew", false, "", "", ""},           // rename without a reason
		{"TestOld — renamed not-an-ident; why", false, "", "", ""}, // rename target not a name
		{"TestFoo — ", false, "", "", ""},                          // no reason
		{"TestFoo superseded", false, "", "", ""},                  // no separator
		{"pkg.TestFoo — qualified", false, "", "", ""},             // not an identifier
	}
	for _, c := range cases {
		r, ok := testledger.ParseRetirement(c.in)
		if ok != c.ok || r.Test != c.test || r.NewName != c.nn || r.Why != c.why {
			t.Errorf("ParseRetirement(%q) = %+v, %v; want test %q new %q why %q, %v", c.in, r, ok, c.test, c.nn, c.why, c.ok)
		}
	}
	log := "aaaa\nTestOld — renamed TestNew; clearer name\nTestGone — duplicate of TestKept\n--\nbbbb\n--\n"
	got := testledger.ParseTrailers(log)
	if len(got) != 2 || got[0].Commit != "aaaa" || got[0].NewName != "TestNew" || got[1].Test != "TestGone" || got[1].Commit != "aaaa" {
		t.Errorf("ParseTrailers: got %+v", got)
	}
}

// probeTrees is one base and one head over two packages. pkg/a keeps an untagged TestParse;
// pkg/b deletes TestGone (#5), TestParse (#7) and the untagged TestRe, renames TestOld (#8)
// to TestActual with an identical body, adds TestReNew (TestRe rewritten, so no pairing),
// keeps TestKeep while dropping its tag and keeps TestList while dropping one of its refs.
func probeTrees(t *testing.T) (base, head []testledger.TestFunc) {
	t.Helper()
	read := func(files map[string]string) []testledger.TestFunc {
		m := fstest.MapFS{}
		for p, body := range files {
			m[p] = &fstest.MapFile{Data: []byte("package p\n\nimport \"testing\"\n\n" + body)}
		}
		fns, err := testledger.Tests(m)
		if err != nil {
			t.Fatal(err)
		}
		return fns
	}
	base = read(map[string]string{
		"pkg/a/a_test.go": "func TestParse(t *testing.T) { t.Log(\"a\") }\n",
		"pkg/b/b_test.go": "// regression: #5\nfunc TestGone(t *testing.T) { t.Log(\"gone\") }\n" +
			"// regression: #7\nfunc TestParse(t *testing.T) { t.Log(\"b\") }\n" +
			"func TestRe(t *testing.T) { t.Log(\"re\") }\n" +
			"// regression: #8\nfunc TestOld(t *testing.T) { t.Log(\"old\") }\n" +
			"// regression: #9\nfunc TestKeep(t *testing.T) { t.Log(\"keep\") }\n" +
			"// regression: #1, #2\nfunc TestList(t *testing.T) { t.Log(\"list\") }\n",
	})
	head = read(map[string]string{
		"pkg/a/a_test.go": "func TestParse(t *testing.T) { t.Log(\"a\") }\n",
		"pkg/b/b_test.go": "func TestReNew(t *testing.T) { t.Log(\"re, rewritten\") }\n" +
			"// regression: #8\nfunc TestActual(t *testing.T) { t.Log(\"old\") }\n" +
			"func TestKeep(t *testing.T) { t.Log(\"keep\") }\n" +
			"// regression: #1\nfunc TestList(t *testing.T) { t.Log(\"list\") }\n",
	})
	return base, head
}

// TestTrailerCoverage pins what a Retires-test: trailer does to a departure. A trailer never
// hides one: a covered departure is shown under its own label with the trailer's commit and
// reason, so a trailer naming a same-named test in another package (P1) is visible. A rename
// trailer covers a rename only when its new name is the test's new name, and a deletion only
// when that name was added in the same package; otherwise the departure stays untrailed. A
// kept test that loses a tag ref is shown too.
func TestTrailerCoverage(t *testing.T) {
	base, head := probeTrees(t)
	at := func(testledger.TestFunc) string { return "X" }
	rows := func(string) []testledger.Row { return nil }
	untrailed := []string{
		"retired-untrailed: pkg/b.TestGone [regression #5] deleted in X",
		"retired-untrailed: pkg/b.TestParse [regression #7] deleted in X",
		"retired-untrailed: pkg/b.TestRe deleted in X",
		"renamed-untrailed: pkg/b.TestOld → TestActual (body identical) in X",
	}
	dropped := []string{
		"tag-dropped: pkg/b.TestKeep [regression #9] → untagged",
		"tag-dropped: pkg/b.TestList [regression #1, #2] → [regression #1]",
	}
	cases := []struct {
		name, log string
		want      []string
	}{
		{"no trailer", "", append(append([]string{}, untrailed...), dropped...)},
		{
			"wrong or absent new names cover nothing; a bare name shows its reason",
			"c1\nTestParse — obsolete after the a-side rewrite\nTestOld — renamed TestSomethingElse; tidy\n" +
				"TestGone — renamed TestNowhere; tidy\nTestRe — renamed TestReNew; body rewritten\n--\n",
			append([]string{
				untrailed[0],
				`retired-trailed: pkg/b.TestParse [regression #7] deleted in X; Retires-test in c1: "obsolete after the a-side rewrite"`,
				`retired-trailed: pkg/b.TestRe deleted in X; Retires-test in c1: renamed TestReNew, "body rewritten"`,
				untrailed[3],
			}, dropped...),
		},
		{
			"a matching rename trailer shows every trailer naming the test",
			"c2\nTestOld — renamed TestActual; clearer name\n--\nc3\nTestOld — superseded\n--\n",
			append(append(append([]string{}, untrailed[:3]...),
				`renamed-trailed: pkg/b.TestOld → TestActual (body identical) in X; Retires-test in c2: renamed TestActual, "clearer name"; Retires-test in c3: "superseded"`),
				dropped...),
		},
	}
	for _, c := range cases {
		got := testledger.Lines(testledger.Diff(base, head), testledger.ParseTrailers(c.log), at, rows)
		if g, w := strings.Join(got, "\n"), strings.Join(c.want, "\n"); g != w {
			t.Errorf("%s:\n%s\nwant:\n%s", c.name, g, w)
		}
	}
}

// departureLine is a report line about one test: its label, then `<pkg>.<Name>`.
var departureLine = regexp.MustCompile(`^([a-z]+(?:-[a-z]+)*): (\S+)\.([A-Za-z0-9_]+)(?: |$)`)

// TestEveryDepartureIsShown is the class guard for a departure that leaves no line, the
// shape that let a trailer turn a deleted tagged test into `clean`. It works out the
// departures from the two trees without Diff (a base test whose package and name are gone
// from head; a kept test whose tag lost a ref) and requires exactly one report line for each,
// and no line for anything else, both with no trailers and with a trailer covering every
// departure in every form. So a label that a trailer, a rename or a tag change can silence
// fails here, wherever in Lines it is added.
func TestEveryDepartureIsShown(t *testing.T) {
	fixBase, err := testledger.Tests(os.DirFS("testdata/base"))
	if err != nil {
		t.Fatal(err)
	}
	fixHead, err := testledger.Tests(os.DirFS("testdata/head"))
	if err != nil {
		t.Fatal(err)
	}
	pBase, pHead := probeTrees(t)
	at := func(testledger.TestFunc) string { return "X" }
	rows := func(string) []testledger.Row { return []testledger.Row{{File: "f.md", ID: "1"}} }
	for _, tr := range []struct {
		name       string
		base, head []testledger.TestFunc
	}{{"fixture", fixBase, fixHead}, {"probe", pBase, pHead}, {"unchanged", pBase, pBase}} {
		inHead := map[string]testledger.TestFunc{}
		for _, f := range tr.head {
			inHead[f.Pkg+"."+f.Name] = f
		}
		want := map[string]bool{}
		var all []testledger.Retirement
		for _, f := range tr.base {
			h, kept := inHead[f.Pkg+"."+f.Name]
			if !kept {
				want[f.Pkg+"."+f.Name] = true
				all = append(all, testledger.Retirement{Commit: "c", Test: f.Name, Why: "w"})
				for _, a := range tr.head {
					all = append(all, testledger.Retirement{Commit: "c", Test: f.Name, NewName: a.Name, Why: "w"})
				}
				continue
			}
			for _, ref := range strings.Split(f.Tag, ", ") {
				if ref != "" && !slicesContains(strings.Split(h.Tag, ", "), ref) {
					want[f.Pkg+"."+f.Name] = true
				}
			}
		}
		for _, set := range []struct {
			name    string
			retired []testledger.Retirement
		}{{"no trailers", nil}, {"every departure trailed", all}} {
			lines := testledger.Lines(testledger.Diff(tr.base, tr.head), set.retired, at, rows)
			seen := map[string]int{}
			for _, l := range lines {
				m := departureLine.FindStringSubmatch(l)
				if m == nil {
					if !strings.HasPrefix(l, "verify-rows-naming: ") {
						t.Errorf("%s, %s: line names no test: %q", tr.name, set.name, l)
					}
					continue
				}
				seen[m[2]+"."+m[3]]++
				if !want[m[2]+"."+m[3]] {
					t.Errorf("%s, %s: line for a test that did not depart: %q", tr.name, set.name, l)
				}
			}
			for k := range want {
				if seen[k] != 1 {
					t.Errorf("%s, %s: %s departed and has %d report lines, want 1:\n%s", tr.name, set.name, k, seen[k], strings.Join(lines, "\n"))
				}
			}
			if len(want) == 0 && len(lines) != 0 {
				t.Errorf("%s, %s: nothing departed and the report is not clean: %q", tr.name, set.name, lines)
			}
		}
		if tr.name != "unchanged" && len(want) == 0 {
			t.Errorf("%s: no departure worked out, so this check saw nothing", tr.name)
		}
	}
}

func slicesContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// TestRangeProbes runs the report end to end, in revision mode, over a synthetic repository:
// each probe is a range that removes or renames a tagged test and must never print `clean`.
// A trailer anywhere in the range (the deleting commit, a later commit, a merge) is shown with
// its own commit beside the deleting one, so the reviewer can see whose reason it is.
func TestRangeProbes(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=probe", "-c", "user.email=probe@example.invalid",
			"-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(p, body string) {
		t.Helper()
		f := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	src := func(fns ...string) string {
		return "package b\n\nimport \"testing\"\n\n" + strings.Join(fns, "\n")
	}
	parse := "// regression: #7\nfunc TestParse(t *testing.T) { t.Log(\"b\") }\n"
	old := "// regression: #8\nfunc TestOld(t *testing.T) { t.Log(\"old\") }\n"
	actual := "// regression: #8\nfunc TestActual(t *testing.T) { t.Log(\"old\") }\n"
	keep := "// regression: #9\nfunc TestKeep(t *testing.T) { t.Log(\"keep\") }\n"
	untaggedKeep := "func TestKeep(t *testing.T) { t.Log(\"keep\") }\n"

	run("init", "-q", "-b", "main")
	write("tools/desk/go.mod", "module probe\n")
	write("pkg/a/a_test.go", "package a\n\nimport \"testing\"\n\nfunc TestParse(t *testing.T) { t.Log(\"a\") }\n")
	write("pkg/b/b_test.go", src(parse, old, keep))
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	base := run("rev-parse", "HEAD")
	t.Chdir(dir)

	// commit checks out base, rewrites pkg/b and commits with the given message paragraphs.
	commit := func(body string, msg ...string) string {
		t.Helper()
		run("checkout", "-q", "--detach", base)
		write("pkg/b/b_test.go", body)
		args := []string{"commit", "-q", "-a"}
		for _, m := range msg {
			args = append(args, "-m", m)
		}
		run(args...)
		return run("rev-parse", "HEAD")
	}
	short := func(c string) string { // as report's own git call abbreviates it
		out, err := git(dir, "log", "-1", "--format=%h", c)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out)
	}
	trailer := func(c, v string) string { return "Retires-test in " + c[:12] + ": " + v }

	type probe struct {
		name, tip string
		want      string // the line the report must carry; it is never clean
	}
	var probes []probe
	ctl := commit(src(old, keep), "drop parse")
	probes = append(probes, probe{"control: untrailed deletion", ctl,
		"retired-untrailed: pkg/b.TestParse [regression #7] deleted in " + short(ctl)})
	p1 := commit(src(old, keep), "drop parse", "Retires-test: TestParse — obsolete after the a-side rewrite")
	probes = append(probes, probe{"P1: a bare-name trailer is shown, never clean", p1,
		"retired-trailed: pkg/b.TestParse [regression #7] deleted in " + short(p1) + "; " + trailer(p1, `"obsolete after the a-side rewrite"`)})
	p2 := commit(src(parse, actual, keep), "rename old", "Retires-test: TestOld — renamed TestSomethingElse; tidy")
	probes = append(probes, probe{"P2: a rename trailer naming another new name covers nothing", p2,
		"renamed-untrailed: pkg/b.TestOld → TestActual (body identical) in " + short(p2)})
	p2ok := commit(src(parse, actual, keep), "rename old", "Retires-test: TestOld — renamed TestActual; clearer name")
	probes = append(probes, probe{"P2 control: a matching rename trailer is shown", p2ok,
		"renamed-trailed: pkg/b.TestOld → TestActual (body identical) in " + short(p2ok) + "; " + trailer(p2ok, `renamed TestActual, "clearer name"`)})
	p2b := commit(src(parse, keep), "drop old", "Retires-test: TestOld — renamed TestNowhere; tidy")
	probes = append(probes, probe{"P2b: a rename trailer on a deletion covers nothing", p2b,
		"retired-untrailed: pkg/b.TestOld [regression #8] deleted in " + short(p2b)})
	del := commit(src(old, keep), "drop parse")
	run("commit", "-q", "--allow-empty", "-m", "later", "-m", "Retires-test: TestParse — said later")
	p3c := run("rev-parse", "HEAD")
	probes = append(probes, probe{"P3c: a trailer on a later commit shows its own commit", p3c,
		"retired-trailed: pkg/b.TestParse [regression #7] deleted in " + short(del) + "; " + trailer(p3c, `"said later"`)})
	run("checkout", "-q", "-b", "side", base)
	run("commit", "-q", "--allow-empty", "-m", "side")
	run("checkout", "-q", "--detach", del)
	run("merge", "-q", "--no-ff", "-m", "merge side", "-m", "Retires-test: TestParse — said in the merge", "side")
	p3 := run("rev-parse", "HEAD")
	probes = append(probes, probe{"P3: a trailer on a merge commit shows its own commit", p3,
		"retired-trailed: pkg/b.TestParse [regression #7] deleted in " + short(del) + "; " + trailer(p3, `"said in the merge"`)})
	a4 := commit(src(parse, old, untaggedKeep), "untag keep")
	probes = append(probes, probe{"A4: a tag removed in place is shown", a4,
		"tag-dropped: pkg/b.TestKeep [regression #9] → untagged"})

	for _, p := range probes {
		lines, _, err := report(base, p.tip)
		if err != nil {
			t.Errorf("%s: could-not-check (%v)", p.name, err)
			continue
		}
		if !slicesContains(lines, p.want) {
			t.Errorf("%s: report\n%s\nwant a line\n%s", p.name, strings.Join(lines, "\n"), p.want)
		}
	}

	// A2: a symbolic name that is also a directory where the command runs is ambiguous, so it
	// is could-not-check, never read as whichever one tree() tries first.
	write("HEAD/pkg/b/b_test.go", src(parse, old, keep))
	if lines, _, err := report(base, "HEAD"); err == nil || len(lines) != 0 {
		t.Errorf("-head=HEAD with a directory named HEAD in the working directory: %q, %v; want could-not-check", lines, err)
	} else if !strings.Contains(err.Error(), "both a directory and a revision") {
		t.Errorf("-head=HEAD ambiguity: reason %q does not say so", err)
	}
}

// TestReportTestLedger is the report: it logs every departure between -base and -head (an
// untrailed one for the reviewer to judge, a trailed one with its trailer's commit and reason,
// a tag dropped in place), or `clean` when nothing departed. It never fails on what it finds —
// a reported line is for the reviewer to judge (review kit §4) — and it skips with
// `could-not-check (<reason>)` when a side cannot be read, so an unreadable range never passes
// as clean. Without -base and -head it skips.
func TestReportTestLedger(t *testing.T) {
	if *baseFlag == "" || *headFlag == "" {
		t.Skip("could-not-check (no range: pass -args -base=<rev|dir> -head=<rev|dir>)")
	}
	lines, notes, err := report(*baseFlag, *headFlag)
	for _, n := range notes {
		t.Log(n)
	}
	if err != nil {
		t.Skipf("could-not-check (%v)", err)
	}
	if len(lines) == 0 {
		t.Log("clean")
	}
	for _, l := range lines {
		t.Log(l)
	}
}

// TestUnresolvableBaseIsCouldNotCheck: a base that names no commit is a could-not-check
// reason, never an empty (clean) report.
func TestUnresolvableBaseIsCouldNotCheck(t *testing.T) {
	lines, _, err := report("deadbeefdeadbeef", "testdata/head")
	if err == nil || len(lines) != 0 {
		t.Errorf("report(deadbeefdeadbeef, testdata/head) = %q, %v; want no lines and a could-not-check reason", lines, err)
	}
}

// tagLine is a tag line as docs/contracts.md spells it, matched over the raw text so the
// check below does not share the parser it checks.
var tagLine = regexp.MustCompile(`^// regression: \s*((?:#[1-9][0-9]*|F-[a-z0-9]+(?:-[a-z0-9]+)*|class #[1-9][0-9]*)(?:\s*,\s*(?:#[1-9][0-9]*|F-[a-z0-9]+(?:-[a-z0-9]+)*|class #[1-9][0-9]*))*)\s*$`)

// TestRepoTagsReadAsWritten is the class guard for prose read as a tag: over every test in
// this repository it checks that the tag Tests reads is exactly the refs of the tag lines in
// the comment block above the func, so a doc comment anywhere whose wrapped prose starts a
// line with `regression: ` adds nothing. Verify row 10 counts tag lines and cannot see a tag
// read wrongly; this can. It checks the parser against the tree, never the tree itself: a
// malformed tag line is untagged on both sides, so no test file can turn it red.
func TestRepoTagsReadAsWritten(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Skipf("could-not-check (%v)", err)
	}
	fns, err := testledger.Tests(os.DirFS(root))
	if err != nil || len(fns) == 0 {
		t.Fatalf("Tests(%s): %d tests, %v", root, len(fns), err)
	}
	files, tagged := map[string][]string{}, 0
	for _, f := range fns {
		lines, ok := files[f.File]
		if !ok {
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.File)))
			if err != nil {
				t.Fatal(err)
			}
			lines = strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
			files[f.File] = lines
		}
		decl := -1
		for i, l := range lines {
			if strings.HasPrefix(l, "func "+f.Name+"(") {
				if decl >= 0 {
					decl = -2 // declared twice in the text (once in a string): not checkable here
					break
				}
				decl = i
			}
		}
		if decl < 0 {
			continue
		}
		var want []string
		for i := decl - 1; i >= 0 && strings.HasPrefix(lines[i], "//"); i-- {
			if m := tagLine.FindStringSubmatch(lines[i]); m != nil {
				var refs []string
				for _, r := range strings.Split(m[1], ",") {
					refs = append(refs, strings.TrimSpace(r))
				}
				want = append(refs, want...)
			}
		}
		if w := strings.Join(want, ", "); f.Tag != w {
			t.Errorf("%s.%s (%s): tag read as %q, the tag lines say %q", f.Pkg, f.Name, f.File, f.Tag, w)
		}
		if f.Tag != "" {
			tagged++
		}
	}
	if tagged == 0 {
		t.Errorf("%d tests read, none tagged: the seeded tags were not seen, so this check saw nothing", len(fns))
	}
	t.Logf("%d tests read, %d tagged", len(fns), tagged)
}

// report reads both sides and renders the lines. A non-nil error is a could-not-check reason.
func report(base, head string) (lines, notes []string, err error) {
	root, err := repoRoot()
	if err != nil {
		return nil, nil, err
	}
	baseFS, baseRev, err := tree(root, base)
	if err != nil {
		return nil, nil, err
	}
	headFS, headRev, err := tree(root, head)
	if err != nil {
		return nil, nil, err
	}
	baseTests, err := testledger.Tests(baseFS)
	if err != nil {
		return nil, nil, fmt.Errorf("base %s: %v", base, err)
	}
	headTests, err := testledger.Tests(headFS)
	if err != nil {
		return nil, nil, fmt.Errorf("head %s: %v", head, err)
	}
	if len(baseTests) == 0 || len(headTests) == 0 {
		return nil, nil, fmt.Errorf("no _test.go functions in %s (%d) or %s (%d)", base, len(baseTests), head, len(headTests))
	}
	var retired []testledger.Retirement
	at := func(testledger.TestFunc) string { return head }
	if baseRev != "" && headRev != "" {
		out, err := git(root, "log", "--format=%H%n%(trailers:key=Retires-test,valueonly,unfold)%n--", baseRev+".."+headRev)
		if err != nil {
			return nil, nil, fmt.Errorf("git log %s..%s: %v", base, head, err)
		}
		retired = testledger.ParseTrailers(out)
		at = func(f testledger.TestFunc) string {
			out, err := git(root, "log", "-1", "--format=%h", "-S", "func "+f.Name+"(", baseRev+".."+headRev, "--", f.Pkg)
			if c := strings.TrimSpace(out); err == nil && c != "" {
				return c
			}
			return head
		}
	} else {
		notes = append(notes, "note: Retires-test: trailers are read only when -base and -head are both revisions")
	}
	var rowErr error
	rows := func(name string) []testledger.Row {
		rs, err := testledger.RowsNaming(headFS, name)
		if err != nil && rowErr == nil {
			rowErr = err
		}
		return rs
	}
	lines = testledger.Lines(testledger.Diff(baseTests, headTests), retired, at, rows)
	if rowErr != nil {
		notes = append(notes, fmt.Sprintf("note: Verify rows partly unread: %v", rowErr))
	}
	return lines, notes, nil
}

// tree reads arg as a directory or as a revision (returning its commit id). A name that is
// both is could-not-check: which one it reads would depend on where the command runs.
func tree(root, arg string) (fs.FS, string, error) {
	st, err := os.Stat(arg)
	isDir := err == nil && st.IsDir()
	rev := ""
	if !strings.HasPrefix(arg, "-") {
		out, _ := git(root, "rev-parse", "--verify", "--quiet", arg+"^{commit}")
		rev = strings.TrimSpace(out)
	}
	switch {
	case isDir && rev != "":
		return nil, "", fmt.Errorf("%q is both a directory and a revision in %s; pass a full commit id or an absolute directory", arg, root)
	case isDir:
		return os.DirFS(arg), "", nil
	case rev == "":
		return nil, "", fmt.Errorf("%q is neither a directory nor a revision in %s", arg, root)
	}
	fsys, err := archive(root, rev)
	if err != nil {
		return nil, "", fmt.Errorf("git archive %s: %v", arg, err)
	}
	return fsys, rev, nil
}

// archive reads the files the report needs (every *_test.go, every docs/streams brief) out of
// `git archive` in-process; nothing is written to disk.
func archive(root, rev string) (fs.FS, error) {
	cmd := exec.Command("git", "archive", "--format=tar", rev)
	cmd.Dir = root
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	fsys := fstest.MapFS{}
	tr := tar.NewReader(stdout)
	var readErr error
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			readErr = err
			break
		}
		name := h.Name
		if h.Typeflag != tar.TypeReg || !fs.ValidPath(name) {
			continue
		}
		if !strings.HasSuffix(name, "_test.go") && !(strings.HasPrefix(name, "docs/streams/") && strings.HasSuffix(name, ".md")) {
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			readErr = err
			break
		}
		fsys[name] = &fstest.MapFile{Data: b}
	}
	if readErr != nil {
		_, _ = io.Copy(io.Discard, stdout)
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	return fsys, readErr
}

func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	return string(out), err
}

// repoRoot walks up from the package directory to the directory holding tools/desk/go.mod.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "tools", "desk", "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no repository root above the package directory")
		}
		dir = parent
	}
}
