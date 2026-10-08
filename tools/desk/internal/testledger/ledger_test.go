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
// tagged test (reported, with its tag) and one untagged test (covered by log.txt's trailer,
// so not reported), renames one with an identical body and one with the same tag and a
// changed body, keeps two and adds one. TestMain, a lower-case Testhelper and a TestDecoy in
// a non-test .go file all leave too, and none of them may appear.
func TestLedgerFixture(t *testing.T) {
	lines, retired := fixtureLines(t)
	want := []string{
		"retired-untrailed: pkg/parse.TestAlpha [regression F-fixture-alpha] deleted in fixture",
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

// TestReportTestLedger is the report: it logs every untrailed departure between -base and
// -head, or `clean`. It never fails on what it finds — a reported line is for the reviewer to
// judge (review kit §4) — and it skips with `could-not-check (<reason>)` when a side cannot be
// read, so an unreadable range never passes as clean. Without -base and -head it skips.
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

// tree reads arg as a directory, else as a revision (returning its commit id).
func tree(root, arg string) (fs.FS, string, error) {
	if st, err := os.Stat(arg); err == nil && st.IsDir() {
		return os.DirFS(arg), "", nil
	}
	out, err := git(root, "rev-parse", "--verify", "--quiet", arg+"^{commit}")
	rev := strings.TrimSpace(out)
	if err != nil || rev == "" {
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
