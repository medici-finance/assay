package weight_test

import (
	"archive/tar"
	"bytes"
	"flag"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/weight"
)

// Test-only flags (see the brief's files list). Pass with `go test -args
// -root=... -rev=... -mode=... -base=...` — the `-args` marker hands everything after it
// to the test binary's own flag.Parse rather than `go test`'s.
var (
	rootFlag = flag.String("root", "", "measure this directory instead of the repository root")
	revFlag  = flag.String("rev", "", "measure a git revision (via `git archive` into a temp dir) instead of the working tree")
	modeFlag = flag.String("mode", "", "override ceiling.txt's mode (advisory|blocking) for this run")
	baseFlag = flag.String("base", "", "compare ceiling.txt's ratcheted values against this git revision's ceiling.txt, and require a \"# grow\" annotation above any raised line")
)

// TestCountsFixture asserts the hand-known counts of testdata/tree (brief-03 Task step 2):
// 2 verbs, 3 flags (one of them a …Var form), 4 refusals (one of each constructor, one
// qualified), 10 total rule-text lines — plus the three decoys (a package-lib cmd/
// subdirectory, a flag registration with a non-literal name, a _test.go refusal) which
// this assertion proves excluded simply by the counts coming out exact.
func TestCountsFixture(t *testing.T) {
	w, err := weight.Count(os.DirFS("testdata/tree"))
	if err != nil {
		t.Fatalf("Count(testdata/tree): %v", err)
	}
	if w.RuleTextCouldNotCheck {
		t.Fatalf("ruletext could-not-check: %s", w.RuleTextReason)
	}
	t.Logf("verbs=%d flags=%d refusals=%d ruletext=%d golines=%d",
		w.Verbs, w.Flags, w.Refusals, w.RuleText, w.GoLines)

	if w.Verbs != 2 {
		t.Errorf("verbs = %d, want 2 (a package-lib cmd/ subdirectory must not count)", w.Verbs)
	}
	if w.Flags != 3 {
		t.Errorf("flags = %d, want 3 (one …Var form; a non-literal-name registration must not count)", w.Flags)
	}
	if w.Refusals != 4 {
		t.Errorf("refusals = %d, want 4 (one per constructor, one qualified; the _test.go call must not count)", w.Refusals)
	}
	if w.RuleText != 10 {
		t.Errorf("ruletext = %d, want 10", w.RuleText)
	}
}

// TestCeilingRedOnGrowthFixture is the negative control (brief-03 Task step 3): a fixture
// ceiling one below the fixture's actual flags count must produce a failure report naming
// the dimension and the delta. It proves Evaluate/GrowthMessage — not just Count — because
// a counter that silently rounded growth to clean would pass TestCountsFixture and still
// be useless as a ratchet.
func TestCeilingRedOnGrowthFixture(t *testing.T) {
	w, err := weight.Count(os.DirFS("testdata/tree"))
	if err != nil {
		t.Fatalf("Count(testdata/tree): %v", err)
	}

	// One below the fixture's actual flags count (3); every other dimension matches
	// exactly, so growth in exactly one dimension is the only signal under test.
	const oneBelowFlags = "# mode: blocking\n" +
		"verbs 2\n" +
		"flags 2\n" +
		"refusals 4\n" +
		"ruletext 10\n"
	c, err := weight.ParseCeiling([]byte(oneBelowFlags))
	if err != nil {
		t.Fatalf("ParseCeiling: %v", err)
	}

	results, err := weight.Evaluate(w, c)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	var grown []weight.DimensionResult
	for _, r := range results {
		if r.Grown() {
			grown = append(grown, r)
		}
	}
	if len(grown) != 1 || grown[0].Dimension != "flags" {
		t.Fatalf("grown dimensions = %+v, want exactly one: flags", grown)
	}
	if grown[0].Delta() != 1 {
		t.Fatalf("flags delta = %d, want 1", grown[0].Delta())
	}

	msg := weight.GrowthMessage(grown[0])
	t.Log(msg)
	if !strings.Contains(msg, "flags: 3 > ceiling 2 (+1)") {
		t.Fatalf("growth message = %q, want it to name the dimension and the delta", msg)
	}
}

// TestGrowthAnnotationAbove is a pure unit test of the "# grow" presence check the
// Growth-approval facts describe (spec §4.5): the ONE thing this brief's ceiling check
// verifies when comparing against a -base revision (the URL's author is review stage 06's
// job, not this test's — see brief-03's single-point-of-failure note). No Verify row
// exercises this directly since it needs a second ceiling.txt revision; this test fixes
// that gap with an in-memory Ceiling rather than a git fixture.
func TestGrowthAnnotationAbove(t *testing.T) {
	withAnnotation := []string{
		"# mode: blocking",
		"verbs 2",
		"# grow flags +1 https://github.com/medici-finance/assay/issues/1",
		"flags 3",
		"refusals 4",
	}
	if !weight.GrowthAnnotationAbove(withAnnotation, "flags") {
		t.Errorf("GrowthAnnotationAbove(flags) = false, want true: annotation sits directly above the line")
	}
	if weight.GrowthAnnotationAbove(withAnnotation, "refusals") {
		t.Errorf("GrowthAnnotationAbove(refusals) = true, want false: no annotation precedes it")
	}

	blankBreaksAdjacency := []string{
		"# grow verbs +1 https://github.com/medici-finance/assay/issues/1",
		"",
		"verbs 2",
	}
	if weight.GrowthAnnotationAbove(blankBreaksAdjacency, "verbs") {
		t.Errorf("GrowthAnnotationAbove(verbs) = true, want false: a blank line breaks adjacency")
	}
}

// TestCeiling reads the real tree — the repository root, or -root/-rev when given — against
// ceiling.txt (brief-03 Task step 4). In blocking mode (ceiling.txt's own mode, or -mode)
// it fails on growth; in advisory mode (the landing state) it logs the same text prefixed
// GROWTH-NOTICE and passes. It logs "slack <dim>=<n>" for every dimension below its
// ceiling. When -base names a revision, it additionally requires a "# grow" annotation
// above any dimension whose ceiling rose since that revision; without -base that check is
// skipped and says so (facts: "Growth approval").
//
// Each ratcheted dimension runs as its own subtest so that a could-not-check ruletext
// dimension shows up as an explicit SKIP rather than silently dropping out of the parent
// test's result: countRuleText's three-state "ok=false" (an absent plugin tree, or one
// listed skill body that cannot be read) must never look like "measured zero and clean",
// and a single flat PASS over a partial `results` slice was exactly that trap — go test
// ./... without -v never printed the t.Logf, so the ratchet gap was invisible in CI.
func TestCeiling(t *testing.T) {
	root := findRepoRoot(t)
	fsys, desc := targetFS(t, root)

	w, err := weight.Count(fsys)
	if err != nil {
		t.Fatalf("Count(%s): %v", desc, err)
	}

	ceilingPath := filepath.Join(root, "tools", "desk", "internal", "weight", "ceiling.txt")
	ceilingBytes, err := os.ReadFile(ceilingPath) //nolint:gosec // fixed, repo-relative path
	if err != nil {
		t.Fatalf("reading ceiling.txt: %v", err)
	}
	c, err := weight.ParseCeiling(ceilingBytes)
	if err != nil {
		t.Fatalf("ParseCeiling: %v", err)
	}

	mode := c.Mode
	if *modeFlag != "" {
		mode = *modeFlag
	}

	results, err := weight.Evaluate(w, c)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	resultByDim := make(map[string]weight.DimensionResult, len(results))
	for _, r := range results {
		resultByDim[r.Dimension] = r
	}

	for _, dim := range weight.RatchetedDimensions {
		dim := dim
		t.Run(dim, func(t *testing.T) {
			if dim == "ruletext" && w.RuleTextCouldNotCheck {
				t.Skipf("could-not-check ruletext: %s", w.RuleTextReason)
			}
			r, ok := resultByDim[dim]
			if !ok {
				t.Fatalf("Evaluate produced no result for ratcheted dimension %q", dim)
			}
			if !r.Grown() {
				t.Log(weight.SlackMessage(r))
				return
			}
			msg := weight.GrowthMessage(r)
			if mode == "blocking" {
				t.Error(msg)
			} else {
				t.Logf("GROWTH-NOTICE %s", msg)
			}
		})
	}

	t.Run("grow-annotation", func(t *testing.T) {
		if *baseFlag == "" {
			t.Log("grow-line check skipped: no -base given")
			return
		}
		baseData := fileAtRev(t, root, *baseFlag, "tools/desk/internal/weight/ceiling.txt")
		baseCeiling, berr := weight.ParseCeiling(baseData)
		if berr != nil {
			t.Fatalf("ParseCeiling(base %s): %v", *baseFlag, berr)
		}
		for _, r := range results {
			baseVal, ok := baseCeiling.Values[r.Dimension]
			if !ok || r.Ceiling <= baseVal {
				continue
			}
			if !weight.GrowthAnnotationAbove(c.Lines, r.Dimension) {
				t.Errorf("%s: ceiling raised %d -> %d with no \"# grow %s +<n> <url>\" line above it",
					r.Dimension, baseVal, r.Ceiling, r.Dimension)
			}
		}
	})
}

// TestPrintWeight logs exactly one line — "weight: verbs=<n> flags=<n> refusals=<n>
// ruletext=<n|could-not-check> golines=<n>" — for -root (default: the repository root) or
// -rev (a `git archive` of that revision into a temp dir). This is the line PR bodies,
// reviewers and an adopting project's baseline/close-out all quote (this brief's Task
// step 5, and later briefs in this stream that build on the counter).
func TestPrintWeight(t *testing.T) {
	fsys, desc := targetFS(t, findRepoRoot(t))
	w, err := weight.Count(fsys)
	if err != nil {
		t.Fatalf("Count(%s): %v", desc, err)
	}
	t.Log(weight.PrintWeight(w))
}

// --- shared test helpers ---

// targetFS resolves -root/-rev into an fs.FS to measure, defaulting to root (the
// repository root) with neither given.
func targetFS(t *testing.T, root string) (fs.FS, string) {
	t.Helper()
	switch {
	case *rootFlag != "" && *revFlag != "":
		t.Fatalf("-root and -rev are mutually exclusive")
		return nil, ""
	case *rootFlag != "":
		return os.DirFS(*rootFlag), *rootFlag
	case *revFlag != "":
		dir := archiveAt(t, root, *revFlag)
		return os.DirFS(dir), *revFlag
	default:
		return os.DirFS(root), "HEAD"
	}
}

// findRepoRoot walks up from the package's own directory (weight tests always run with
// that as their working directory) to the ancestor containing tools/desk/go.mod — the
// repository root the brief's Task step 4 measures by default.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "tools", "desk", "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate the repository root (a directory containing tools/desk/go.mod) above %s", wd)
		}
		dir = parent
	}
}

// archiveAt materializes rev's tree into a temp directory via `git archive`, read
// in-process with archive/tar rather than shelling a second `tar` process — this is the
// test file's own confined git read (brief-03 facts: "layering"); weight.go itself never
// touches git or the network.
func archiveAt(t *testing.T, repoRoot, rev string) string {
	t.Helper()
	dir := t.TempDir()

	cmd := exec.Command("git", "archive", "--format=tar", rev)
	cmd.Dir = repoRoot
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("git archive %s: stdout pipe: %v", rev, err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("git archive %s: start: %v", rev, err)
	}

	tr := tar.NewReader(stdout)
	for {
		hdr, terr := tr.Next()
		if terr == io.EOF {
			break
		}
		if terr != nil {
			t.Fatalf("git archive %s: reading tar stream: %v", rev, terr)
		}
		target := filepath.Join(dir, filepath.FromSlash(hdr.Name)) //nolint:gosec // rev is operator-supplied, archive is this repo's own history
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil { //nolint:gosec
				t.Fatalf("git archive %s: mkdir %s: %v", rev, target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil { //nolint:gosec
				t.Fatalf("git archive %s: mkdir %s: %v", rev, filepath.Dir(target), err)
			}
			out, cerr := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644) //nolint:gosec
			if cerr != nil {
				t.Fatalf("git archive %s: create %s: %v", rev, target, cerr)
			}
			if _, cerr := io.Copy(out, tr); cerr != nil { //nolint:gosec // bounded by this repo's own tree
				out.Close()
				t.Fatalf("git archive %s: write %s: %v", rev, target, cerr)
			}
			out.Close()
		}
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("git archive %s: %v: %s", rev, err, strings.TrimSpace(stderr.String()))
	}
	return dir
}

// fileAtRev reads one path's raw content at rev via `git show <rev>:<path>` — cheaper than
// a full archive when only ceiling.txt itself is needed (the -base grow-line check).
func fileAtRev(t *testing.T, repoRoot, rev, relPath string) []byte {
	t.Helper()
	cmd := exec.Command("git", "show", rev+":"+relPath)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git show %s:%s: %v", rev, relPath, err)
	}
	return out
}
