package hotspot_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/hotspot"
)

// Test-only flags (the brief's files list; no shipped binary gains a flag). Pass with
// `go test -args -root=... -since=... -until=... -top=...`.
var (
	rootFlag  = flag.String("root", "", "read this checkout's history instead of the repository this test runs in")
	sinceFlag = flag.String("since", "", "window start, YYYY-MM-DD or RFC 3339, passed to git verbatim; default 90 days before -until")
	untilFlag = flag.String("until", "", "window end, YYYY-MM-DD or RFC 3339, passed to git verbatim; default now (UTC)")
	topFlag   = flag.Int("top", 10, "how many ranked files to print")
)

const (
	fA = "tools/desk/cmd/alpha/main.go"
	fB = "tools/desk/cmd/alpha/run.go"
	fC = "tools/desk/internal/beta/beta.go"
	fD = "tools/desk/internal/beta/store.go"
	fE = "tools/desk/internal/gamma/gamma.go"
	fF = "tools/desk/internal/decoy/prefix.go"
)

func loadFixture(t *testing.T) []hotspot.Commit {
	t.Helper()
	f, err := os.Open("testdata/log.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	commits, err := hotspot.Parse(f)
	if err != nil {
		t.Fatalf("Parse(testdata/log.txt): %v", err)
	}
	return commits
}

// TestHotspotFixture asserts churn, fixes, complexity, lines and rank for every file of the
// hand-built fixture (12 commits; testdata/tree holds the six in-scope files at the window's
// end). The decoys, each of which the exact counts below would catch:
//
//   - tools/desk/internal/beta/beta_test.go (a _test.go path) must not be ranked;
//   - tools/desk/README.md and docs/guide.md (not in-scope .go) must not be ranked;
//   - the 40-file merge counts once toward each file's churn, and its 34 wide/ files are
//     absent from the tree, so they are not ranked;
//   - "refactor: prefix handling" and "chore: fixture update" must NOT count as fixes
//     ("prefix-decoy fixes=0" is Verify row 5's negative control);
//   - the Revert commit is dated 04:00 -0500, i.e. 09:00 UTC, which the window sub-check
//     below depends on.
//
// Hand-known values (churn from the commit list in testdata/log.txt; complexity is each
// tree file's leading-tab sum, counted by hand):
//
//	file         churn fixes complexity lines score rank
//	alpha/run.go     7     4         10    10    70    1
//	beta/beta.go     6     3          6     9    36    2  (tie with gamma: churn breaks it)
//	gamma.go         3     2         12    12    36    3
//	alpha/main.go    8     4          4     7    32    4
//	beta/store.go    5     2          2     9    10    5
//	decoy/prefix.go  2     0          1     6     2    6  (a tab inside a literal is not leading)
func TestHotspotFixture(t *testing.T) {
	commits := loadFixture(t)
	if len(commits) != 12 {
		t.Fatalf("parsed %d commits, want 12", len(commits))
	}
	var merge *hotspot.Commit
	for i := range commits {
		if strings.HasPrefix(commits[i].Subject, "Merge ") {
			merge = &commits[i]
		}
	}
	if merge == nil || len(merge.Files) != 40 {
		t.Fatalf("fixture's merge commit missing or not 40 files: %+v", merge)
	}

	got, err := hotspot.Score(commits, os.DirFS("testdata/tree"), hotspot.Options{})
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	type want struct {
		label                                string
		churn, fixes, cx, lines, score, rank int
	}
	wants := map[string]want{
		fB: {"alpha-run", 7, 4, 10, 10, 70, 1},
		fC: {"beta", 6, 3, 6, 9, 36, 2},
		fE: {"gamma", 3, 2, 12, 12, 36, 3},
		fA: {"alpha-main", 8, 4, 4, 7, 32, 4},
		fD: {"beta-store", 5, 2, 2, 9, 10, 5},
		fF: {"prefix-decoy", 2, 0, 1, 6, 2, 6},
	}
	for _, s := range got {
		w, ok := wants[s.Path]
		if !ok {
			t.Errorf("ranked an out-of-scope or absent file: %s", s.Path)
			continue
		}
		t.Logf("%s fixes=%d churn=%d complexity=%d lines=%d score=%d rank=%d pct=%.1f %s",
			w.label, s.Fixes, s.Churn, s.Complexity, s.Lines, s.Score, s.Rank, s.Pct, s.Path)
		if s.Churn != w.churn || s.Fixes != w.fixes || s.Complexity != w.cx || s.Lines != w.lines || s.Score != w.score || s.Rank != w.rank {
			t.Errorf("%s: got churn=%d fixes=%d complexity=%d lines=%d score=%d rank=%d; want %d %d %d %d %d %d",
				s.Path, s.Churn, s.Fixes, s.Complexity, s.Lines, s.Score, s.Rank,
				w.churn, w.fixes, w.cx, w.lines, w.score, w.rank)
		}
		if wantPct := 100 * float64(w.rank) / 6; s.Pct != wantPct {
			t.Errorf("%s: pct=%v, want %v", s.Path, s.Pct, wantPct)
		}
	}
	if len(got) != len(wants) {
		t.Errorf("ranked %d files, want %d", len(got), len(wants))
	}

	// The window is inclusive and in UTC. Since the second commit: alpha/main.go loses the
	// first (non-fix) commit. Until 08:59:59 UTC on the 8th: the Revert (04:00 -0500 = 09:00
	// UTC) falls outside, so gamma keeps only the merge — a parser that dropped the zone
	// would read it as 04:00 UTC and count it.
	since := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 9, 8, 8, 59, 59, 0, time.UTC)
	win, err := hotspot.Score(commits, os.DirFS("testdata/tree"), hotspot.Options{Since: since})
	if err != nil {
		t.Fatal(err)
	}
	if s := find(win, fA); s == nil || s.Churn != 7 || s.Fixes != 4 {
		t.Errorf("since %s: alpha/main.go = %+v, want churn 7 fixes 4", since.Format(time.RFC3339), s)
	}
	win, err = hotspot.Score(commits, os.DirFS("testdata/tree"), hotspot.Options{Until: until})
	if err != nil {
		t.Fatal(err)
	}
	if s := find(win, fE); s == nil || s.Churn != 1 || s.Fixes != 0 {
		t.Errorf("until %s: gamma.go = %+v, want churn 1 fixes 0", until.Format(time.RFC3339), s)
	}
}

func find(xs []hotspot.FileScore, path string) *hotspot.FileScore {
	for i := range xs {
		if xs[i].Path == path {
			return &xs[i]
		}
	}
	return nil
}

// TestCouplingFixture asserts exactly the pairs that pass both filters. In the fixture only
// alpha/main.go–alpha/run.go does: 6 shared commits, ratio 6/min(8,7). beta.go–store.go
// shares 4 commits outside the merge; counting the 40-file merge would lift it to 5 (ratio
// 5/5) and add it to the report, and lift the alpha pair to 7 — so the exact result pins
// the merge's exclusion. The synthetic cases pin the ratio filter and the changeset bound.
func TestCouplingFixture(t *testing.T) {
	got := hotspot.Coupling(loadFixture(t), hotspot.Options{})
	for _, p := range got {
		t.Logf("coupling: %s %s count=%d ratio=%.2f", p.A, p.B, p.Count, p.Ratio)
	}
	if len(got) != 1 || got[0].A != fA || got[0].B != fB || got[0].Count != 6 || got[0].Ratio != 6.0/7.0 {
		t.Fatalf("pairs = %+v, want exactly {%s %s 6 %.4f}", got, fA, fB, 6.0/7.0)
	}

	// Ratio filter: X and Y share 5 commits but each has churn 25 → ratio 0.2 < 0.3.
	day := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	var syn []hotspot.Commit
	add := func(files ...string) {
		syn = append(syn, hotspot.Commit{Hash: fmt.Sprintf("%040x", len(syn)+1), Date: day, Subject: "change", Files: files})
	}
	x, y := "tools/desk/x.go", "tools/desk/y.go"
	for i := 0; i < 20; i++ {
		add(x)
		add(y)
	}
	for i := 0; i < 5; i++ {
		add(x, y)
	}
	if p := hotspot.Coupling(syn, hotspot.Options{}); len(p) != 0 {
		t.Errorf("ratio 0.2 pair reported: %+v", p)
	}
	if p := hotspot.Coupling(syn, hotspot.Options{MinRatio: 0.1}); len(p) != 1 || p[0].Count != 5 || p[0].Ratio != 0.2 {
		t.Errorf("with MinRatio 0.1 want the x–y pair at 5/0.2, got %+v", p)
	}

	// Changeset bound: 5 commits of exactly 8 in-scope files count; of 9 they do not.
	changeset := func(n int) []hotspot.Commit {
		var cs []hotspot.Commit
		for i := 0; i < 5; i++ {
			files := []string{"tools/desk/p.go", "tools/desk/q.go"}
			for j := 2; j < n; j++ {
				files = append(files, fmt.Sprintf("tools/desk/pad%d_%d.go", i, j))
			}
			cs = append(cs, hotspot.Commit{Hash: fmt.Sprintf("%040x", i+1), Date: day, Subject: "change", Files: files})
		}
		return cs
	}
	if p := hotspot.Coupling(changeset(8), hotspot.Options{}); len(p) != 1 || p[0].A != "tools/desk/p.go" || p[0].Count != 5 {
		t.Errorf("8-file commits: want the p–q pair at 5, got %+v", p)
	}
	if p := hotspot.Coupling(changeset(9), hotspot.Options{}); len(p) != 0 {
		t.Errorf("9-file commits must be skipped, got %+v", p)
	}
}

// TestPrintHotspots is the REPORT (never a gate): the ranking and the coupling pairs over
// the first-parent history of refs/remotes/origin/main (HEAD when that ref is absent) in
// -root, for the window [-since, -until]. On a shallow clone it skips as
// could-not-check (shallow) — never an empty ranking.
func TestPrintHotspots(t *testing.T) {
	root := *rootFlag
	if root == "" {
		root = "."
	}
	if reason := historyGap(root); reason != "" {
		t.Skipf("could-not-check (%s): %s", reason, root)
	}
	s, u, err := window(*sinceFlag, *untilFlag, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ref := "refs/remotes/origin/main"
	if _, err := git(root, "log", "-1", "--format=%H", ref); err != nil {
		ref = "HEAD"
	}
	logOut, err := git(root, "-c", "core.quotePath=false", "log", "--first-parent",
		"--since="+s, "--until="+u, "--name-only", "--format=%H%x00%ci%x00%s", ref)
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	commits, err := hotspot.Parse(bytes.NewReader(logOut))
	if err != nil {
		t.Fatalf("parsing git log: %v", err)
	}
	// git has already applied the window; the zero Options bounds add no second reading
	// of it.
	opts := hotspot.Options{}
	tip, err := git(root, "log", "-1", "--first-parent", "--until="+u, "--format=%H", ref)
	if err != nil {
		t.Fatalf("git log (tip at until): %v", err)
	}
	tipSHA := strings.TrimSpace(string(tip))
	tree := fstest.MapFS{}
	if tipSHA != "" {
		seen := map[string]bool{}
		for _, c := range commits {
			for _, f := range c.Files {
				if seen[f] || !hotspot.InScope(f, "tools/desk/") {
					continue
				}
				seen[f] = true
				src, err := git(root, "show", tipSHA+":"+f)
				if err != nil {
					if strings.Contains(err.Error(), "does not exist in") || strings.Contains(err.Error(), "but not in") {
						continue // deleted or renamed away by the window's end
					}
					t.Fatalf("git show %s:%s: %v", tipSHA, f, err)
				}
				tree[f] = &fstest.MapFile{Data: src}
			}
		}
	}
	scores, err := hotspot.Score(commits, tree, opts)
	if err != nil {
		t.Fatal(err)
	}
	pairs := hotspot.Coupling(commits, opts)
	t.Logf("hotspot-window: since=%s until=%s (as git reads them, inclusive; a bare date takes the current time of day); ref=%s tip=%s commits=%d ranked=%d; scope=tools/desk/ .go minus _test.go, testdata/, vendor/; churn=first-parent commits touching the file; fixes=subject matching %s; complexity=leading-tab sum at tip; score=churn*complexity; pct=100*rank/ranked; coupling=commits with <=8 in-scope files, pair count>=5 and count/min(churn)>=0.3",
		s, u, ref, tipSHA, len(commits), len(scores), hotspot.FixPattern)
	for i, f := range scores {
		if i >= *topFlag {
			break
		}
		t.Logf("hotspot: rank=%d score=%d churn=%d fixes=%d complexity=%d lines=%d pct=%.1f %s",
			f.Rank, f.Score, f.Churn, f.Fixes, f.Complexity, f.Lines, f.Pct, f.Path)
	}
	for _, p := range pairs {
		t.Logf("coupling: %s %s count=%d ratio=%.2f", p.A, p.B, p.Count, p.Ratio)
	}
}

// TestShallowIsCouldNotCheck proves the three-state gate TestPrintHotspots uses: a
// `git clone --depth 1` of this repository reads as could-not-check (shallow), and — when
// the source itself has full history — the source does not.
func TestShallowIsCouldNotCheck(t *testing.T) {
	src := *rootFlag
	if src == "" {
		src = repoTop(t)
	}
	if reason := historyGap(src); reason != "" && reason != "shallow" {
		t.Skipf("could-not-check (%s): %s", reason, src)
	} else if reason == "" {
		t.Logf("source %s has full history (positive control)", src)
	} else {
		t.Logf("source %s is itself shallow; positive control not observable here", src)
	}
	abs, err := filepath.Abs(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "shallow")
	if out, err := exec.Command("git", "clone", "-q", "--depth", "1", "file://"+abs, dst).CombinedOutput(); err != nil {
		t.Fatalf("git clone --depth 1: %v\n%s", err, out)
	}
	if got := historyGap(dst); got != "shallow" {
		t.Fatalf("shallow clone: historyGap = %q, want \"shallow\"", got)
	}
	t.Logf("could-not-check (shallow) confirmed for a depth-1 clone")
}

// historyGap is "" when root has full git history, "shallow" for a shallow clone, or
// another could-not-check reason.
func historyGap(root string) string {
	out, err := git(root, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return "no git history: " + err.Error()
	}
	if strings.TrimSpace(string(out)) == "true" {
		return "shallow"
	}
	return ""
}

// repoTop walks up from the working directory to the directory holding .git (a directory
// in a clone, a file in a linked worktree).
func repoTop(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("could-not-check (no git checkout above the working directory)")
		}
		dir = parent
	}
}

// window resolves the flags into the two bound strings handed to git. A given bound goes to
// git VERBATIM, so the window is exactly the one an independent `git log --since --until`
// with the same flags reads (Verify row 4 relies on that). Note git reads a bare
// YYYY-MM-DD as that date at the CURRENT time of day; for a window that reproduces at any
// time of day, pass RFC 3339 bounds (2026-06-26T00:00:00Z). The defaults are RFC 3339:
// until = now, since = until − 90 days, both UTC.
func window(since, until string, now time.Time) (string, string, error) {
	for _, v := range []string{since, until} {
		if v == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", v); err == nil {
			continue
		}
		if _, err := time.Parse(time.RFC3339, v); err != nil {
			return "", "", fmt.Errorf("window bound %q: want YYYY-MM-DD or RFC 3339", v)
		}
	}
	n := now.UTC().Truncate(time.Second)
	if until == "" {
		until = n.Format(time.RFC3339)
	}
	if since == "" {
		u, err := time.Parse(time.RFC3339, until)
		if err != nil {
			u, _ = time.Parse("2006-01-02", until)
		}
		since = u.AddDate(0, 0, -90).UTC().Format(time.RFC3339)
	}
	return since, until, nil
}

// git runs one read-only git command in root. Only log, show and rev-parse
// --is-shallow-repository are called; nothing here writes history.
func git(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
