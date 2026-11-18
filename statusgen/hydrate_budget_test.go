package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"testing"
	"time"
)

// statusgen/44: the hydrated load (loadHydratedStreams) must stay cheap next to
// the bare load (loadStreams). Hydration discards every notice it computes, yet it
// used to ask git for main's history once per waiting brief, which cost ~11 s on a
// 614-brief tree whose bare load takes well under 0.2 s and whose synthetic
// benchmark corpus (no git history, no FAIL evidence) never showed it.

// The brief proposes hydrated <= 10x bare. Measured in-process, the floor of
// hydration is one open, stat, read and YAML parse per brief file (~100 us each),
// while the bare load only reads the boards, so the two sit at 7x to 13x on this
// synthetic corpus even with the history queries gone (10.3x, 10.8x, 11.4x, 12.6x
// in four consecutive runs on a loaded host). A 10x bound would flake on the fix
// itself, so the ratio bounds are 20x (synthetic) and 100x (real tree: 623 briefs,
// bare ~9 ms, hydrated ~0.2 s, 27x). Both still sit ~50x under the 1000x the history
// queries cost, and the count guard (zero history queries, one parse per brief) is
// the primary, host-independent assertion. Both bounds are declared reversible
// defaults in the PR.
const (
	hydrateRatioBudget    = 20.0
	hydrateRealTreeBudget = 100.0
	hydrateRuns           = 3
	hydrateSynthRuns      = 9 // cheap (ms each), so more than the required three
)

type hydrateShape struct{ streams, briefsPerStream, merges int }

// hydrateRealShapeRoot builds a git repository whose briefs all wait on a
// verifier with a recorded FAIL (testdata/hydrate-plant/brief.md) and whose main
// carries merge commits after the FAIL date. Returns the root.
func hydrateRealShapeRoot(t *testing.T, shape hydrateShape) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "docs/streams/alpha")); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/hydrate-plant/brief.md")
	if err != nil {
		t.Fatal(err)
	}
	issue := 5000
	for i := 0; i < shape.streams; i++ {
		name := fmt.Sprintf("work-%02d", i)
		dir := filepath.Join(root, "docs/streams", name)
		var board strings.Builder
		fmt.Fprintf(&board, "---\nstream: %s\nstatus: active\npriority: P1\ntrack: platform\n---\n\n# Workload\n\n| # | Brief | Wave | Status | Verified | Reviewed |\n|---|---|---|---|---|---|\n", name)
		for j := 1; j <= shape.briefsPerStream; j++ {
			num := fmt.Sprintf("%02d", j)
			issue++
			fmt.Fprintf(&board, "| %s | [Unit](brief-%s.md) | 0 | implemented | — | — |\n", num, num)
			text := strings.NewReplacer("{{stream}}", name, "{{num}}", num, "{{issue}}", fmt.Sprint(issue)).Replace(string(raw))
			mustWrite(t, filepath.Join(dir, "brief-"+num+".md"), text)
		}
		mustWrite(t, filepath.Join(dir, "README.md"), board.String())
	}
	mustWrite(t, filepath.Join(root, "src/check.go"), "package check\n")
	gitInit(t, root, "T", "t@example.invalid")
	runGit(t, root, "checkout", "-b", "main")
	runGit(t, root, "add", "-A")
	day := func(d int) []string {
		stamp := fmt.Sprintf("2026-02-%02dT12:00:00Z", d)
		return []string{"GIT_AUTHOR_DATE=" + stamp, "GIT_COMMITTER_DATE=" + stamp}
	}
	runGitEnv(t, root, []string{"GIT_AUTHOR_DATE=2026-01-01T12:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T12:00:00Z"}, "commit", "-qm", "initial")
	for k := 1; k <= shape.merges; k++ {
		branch := fmt.Sprintf("change-%d", k)
		runGit(t, root, "checkout", "-qb", branch)
		mustWrite(t, filepath.Join(root, "src/check.go"), fmt.Sprintf("package check\n// %d\n", k))
		runGit(t, root, "add", "src/check.go")
		runGitEnv(t, root, day(1+k%27), "commit", "-qm", fmt.Sprintf("change %d", k))
		runGit(t, root, "checkout", "-q", "main")
		runGitEnv(t, root, day(1+k%27), "merge", "-q", "--no-ff", "-m", fmt.Sprintf("land change %d", k), branch)
	}
	runGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	return root
}

// hydrateMeasurement is the best of N cold runs of each load on one tree, in one
// process, plus the countable work the hydrated load did.
type hydrateMeasurement struct {
	bare, hydrated time.Duration
	briefs         int
	historyQueries int64 // newestFailFix runs during the last hydrated load
	parses         int64 // distinct brief parses during the last hydrated load
}

func (m hydrateMeasurement) ratio() float64 {
	return float64(m.hydrated) / float64(max(m.bare, time.Microsecond))
}

// measureHydration times loadStreams and loadHydratedStreams on root. Every run
// starts with cleared read and parse memos so none is a warm replay, and runs
// inside a git read session as the command line does (main.go), which is the
// cheaper of the two ways a real invocation can reach this code.
func measureHydration(t *testing.T, root string, runs int, profile *os.File) hydrateMeasurement {
	t.Helper()
	var m hydrateMeasurement
	defer beginGitReadSession()()
	for i := 0; i < runs; i++ {
		// Interleave the two loads so a load spike on the host hits both.
		resetPerfInputs()
		start := time.Now()
		if _, _, err := loadStreams(root); err != nil {
			t.Fatal(err)
		}
		if d := time.Since(start); i == 0 || d < m.bare {
			m.bare = d
		}
		resetPerfInputs()
		queries := staleFailHistoryQueries.Load()
		if profile != nil && i == runs-1 {
			if err := pprof.StartCPUProfile(profile); err != nil {
				t.Fatal(err)
			}
		}
		start = time.Now()
		streams, _, err := loadHydratedStreams(root)
		d := time.Since(start)
		if profile != nil && i == runs-1 {
			pprof.StopCPUProfile()
		}
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 || d < m.hydrated {
			m.hydrated = d
		}
		m.historyQueries = staleFailHistoryQueries.Load() - queries
		m.parses = briefParseCount.Load()
		m.briefs = 0
		for _, s := range streams {
			m.briefs += len(briefFilePaths(s))
		}
	}
	return m
}

// hydrateBudgetViolation returns "" when the measurement is within budget, else a
// message naming what broke. The count checks come first because they do not
// vary with the host: hydration discards its notices, so it must make no history
// query at all, and it must parse each brief file at most once.
func hydrateBudgetViolation(m hydrateMeasurement, ratioBudget float64) string {
	var out []string
	if m.historyQueries != 0 {
		out = append(out, fmt.Sprintf("count: %d git history queries during hydration (%d briefs), want 0", m.historyQueries, m.briefs))
	}
	if m.parses > int64(m.briefs) {
		out = append(out, fmt.Sprintf("count: %d brief parses for %d briefs, want at most one per brief", m.parses, m.briefs))
	}
	if m.ratio() > ratioBudget {
		out = append(out, fmt.Sprintf("ratio: hydrated %v is %.1fx bare %v, budget %.0fx", m.hydrated, m.ratio(), m.bare, ratioBudget))
	}
	return strings.Join(out, "; ")
}

var hydrateCorpus = hydrateShape{streams: 4, briefsPerStream: 8, merges: 24}

// regression: statusgen/44
func TestHydrateBudget(t *testing.T) {
	installStatusgenFixture(t)
	root := hydrateRealShapeRoot(t, hydrateCorpus)
	m := measureHydration(t, root, hydrateSynthRuns, nil)
	t.Logf("%d briefs: bare %v, hydrated %v (%.1fx), history queries %d, parses %d", m.briefs, m.bare, m.hydrated, m.ratio(), m.historyQueries, m.parses)
	if m.briefs != hydrateCorpus.streams*hydrateCorpus.briefsPerStream {
		t.Fatalf("corpus has %d briefs", m.briefs)
	}
	if msg := hydrateBudgetViolation(m, hydrateRatioBudget); msg != "" {
		t.Fatalf("hydration budget: %s", msg)
	}
}

// TestHydrateBudgetPlant puts the removed cost back behind its test hook and
// asserts the budget check reports it by count and by ratio.
// regression: statusgen/44
func TestHydrateBudgetPlant(t *testing.T) {
	installStatusgenFixture(t)
	root := hydrateRealShapeRoot(t, hydrateCorpus)
	briefHistoryNoticesInHydration = true
	t.Cleanup(func() { briefHistoryNoticesInHydration = false })
	m := measureHydration(t, root, 1, nil)
	msg := hydrateBudgetViolation(m, hydrateRatioBudget)
	t.Logf("planted: %s", msg)
	if !strings.Contains(msg, "count: ") || !strings.Contains(msg, "git history queries") {
		t.Fatalf("the budget did not report the planted history queries: %q", msg)
	}
	if !strings.Contains(msg, "ratio: ") {
		t.Fatalf("the budget did not report the planted ratio: %q", msg)
	}
}

// TestHydrateRealTree runs the same budget on the tree named by
// ASSAY_HYDRATE_TREE and writes a CPU profile of the last hydrated load under
// ${TMPDIR:-/tmp}; the profile is how statusgen/bench/hydration-profile.md was read.
// regression: statusgen/44
func TestHydrateRealTree(t *testing.T) {
	root := os.Getenv("ASSAY_HYDRATE_TREE")
	if root == "" {
		t.Skip("set ASSAY_HYDRATE_TREE to a repository root to measure it")
	}
	profile, err := os.CreateTemp(os.TempDir(), "statusgen-hydrate-*.prof")
	if err != nil {
		t.Fatal(err)
	}
	defer profile.Close()
	m := measureHydration(t, root, hydrateRuns, profile)
	t.Logf("tree %s: %d briefs, bare %v, hydrated %v (%.1fx), history queries %d, parses %d; cpu profile %s",
		filepath.Base(root), m.briefs, m.bare, m.hydrated, m.ratio(), m.historyQueries, m.parses, profile.Name())
	if msg := hydrateBudgetViolation(m, hydrateRealTreeBudget); msg != "" {
		t.Fatalf("hydration budget: %s", msg)
	}
}
