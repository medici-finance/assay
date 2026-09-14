package main

// The MEASUREMENT rows for brief desk-tools/26 (issue #1037).
//
// Everything here runs against a SYNTHETIC repository the test itself builds under its own
// t.TempDir(). Nothing in this file touches a real checkout, and nothing reads a path it
// did not create — a performance test for a destructive verb must never be pointed at a
// tree somebody is working in.
//
// The fixture has to be big enough to be a measurement and cheap enough to be a test:
//
//   - The history is built with `git fast-import`, which writes N commits in one pass. The
//     shell-per-commit alternative is minutes at N=5,000; fast-import is under a second.
//   - The worktrees are FABRICATED admin entries rather than `git worktree add` checkouts
//     (which would be ~50 ms each, i.e. half a minute at N=600 before any measuring). The
//     fabrication is verified against git's own oracle: the test asserts `git worktree
//     list --porcelain` reports exactly the population it planted before it measures
//     anything, so a fixture that drifted from what git considers a worktree fails LOUDLY
//     rather than measuring a sweep with nothing in it.
//
// Size is read from DESKWT_PERF_N so one test body serves the 50 / 200 / 600 rows. The
// default is small, so a plain `go test ./cmd/deskwt/` stays fast.

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// perfN reads the fixture size for this run.
func perfN(t testing.TB) int {
	t.Helper()
	raw := os.Getenv("DESKWT_PERF_N")
	if raw == "" {
		return 50
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		t.Fatalf("DESKWT_PERF_N must be a positive integer, got %q", raw)
	}
	return n
}

// perfBudget is the wall-clock gate for a sweep over n worktrees. The 600 figure is the
// target issue #1037 states; the smaller sizes are scaled from it with generous headroom,
// because the point of the gate is to catch a RETURN of per-candidate history walking
// (which was 0.77-1.28 s PER WORKTREE), not to police a few hundred milliseconds.
func perfBudget(n int) time.Duration {
	switch {
	case n >= 600:
		return 30 * time.Second
	case n >= 200:
		return 20 * time.Second
	default:
		return 15 * time.Second
	}
}

// buildSyntheticHistory writes `commits` commits onto refs/heads/main via git fast-import
// and points refs/remotes/origin/main at the tip. Returns the tip SHA.
func buildSyntheticHistory(t testing.TB, work string, commits int) string {
	t.Helper()
	var b strings.Builder
	for i := 0; i < commits; i++ {
		fmt.Fprintf(&b, "commit refs/heads/main\nmark :%d\n", i+1)
		b.WriteString("author Fixture <fixture@example.invalid> 1600000000 +0000\n")
		b.WriteString("committer Fixture <fixture@example.invalid> 1600000000 +0000\n")
		msg := fmt.Sprintf("synthetic commit %d", i)
		fmt.Fprintf(&b, "data %d\n%s\n", len(msg), msg)
		// Chain onto the fixture's EXISTING main so the import is a fast-forward — git
		// fast-import refuses a ref update that would drop the current tip.
		if i == 0 {
			b.WriteString("from refs/heads/main^0\n")
		} else {
			fmt.Fprintf(&b, "from :%d\n", i)
		}
		body := fmt.Sprintf("commit %d\n", i)
		fmt.Fprintf(&b, "M 100644 inline history.txt\ndata %d\n%s", len(body), body)
		b.WriteString("\n")
	}
	b.WriteString("done\n")

	cmd := execCommand("git", "fast-import", "--done", "--quiet")
	cmd.Dir = work
	cmd.Stdin = strings.NewReader(b.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git fast-import: %v\n%s", err, out)
	}
	mustGit(t, work, "reset", "--hard", "refs/heads/main")
	tip := mustGit(t, work, "rev-parse", "refs/heads/main")
	mustGit(t, work, "update-ref", "refs/remotes/origin/main", tip)
	return tip
}

// buildSideCommit adds ONE commit that is not on the mainline, and returns its SHA. It is
// what makes a fabricated worktree genuinely UNMERGED — the case the old code walked
// origin/main to exhaustion for, and the case 19 in 20 candidates were in when issue #1037
// was measured.
func buildSideCommit(t testing.TB, work string) string {
	var b strings.Builder
	b.WriteString("commit refs/heads/side\nmark :1\n")
	b.WriteString("author Fixture <fixture@example.invalid> 1600000000 +0000\n")
	b.WriteString("committer Fixture <fixture@example.invalid> 1600000000 +0000\n")
	msg := "side work, never merged"
	fmt.Fprintf(&b, "data %d\n%s\n", len(msg), msg)
	b.WriteString("from refs/heads/main^0\n")
	body := "side\n"
	fmt.Fprintf(&b, "M 100644 inline side.txt\ndata %d\n%s", len(body), body)
	b.WriteString("\ndone\n")

	cmd := execCommand("git", "fast-import", "--done", "--quiet")
	cmd.Dir = work
	cmd.Stdin = strings.NewReader(b.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git fast-import (side): %v\n%s", err, out)
	}
	return mustGit(t, work, "rev-parse", "refs/heads/side")
}

// fabricateWorktrees plants n registered worktrees by writing the admin entries git itself
// writes. Each gets a real working directory under the sanctioned prefix so the path guard
// accepts it and the tracked-clean gate has something to read.
//
// Heads are distributed to match the measured population: 19 in 20 sit on `unmergedHead`
// (genuinely not an ancestor of origin/main — the case that used to force a full walk to
// exhaustion) and the rest on `mergedHead`.
//
// Returns the planted paths.
func fabricateWorktrees(t testing.TB, work string, n int, mergedHead, unmergedHead string) []string {
	t.Helper()
	commonDir := filepath.Join(work, ".git")
	var planted []string
	for i := 0; i < n; i++ {
		head := unmergedHead
		if i%20 == 0 {
			head = mergedHead
		}
		name := fmt.Sprintf("tracker-synth-%04d", i)
		wtDir := filepath.Join(tmpBaseDir, name)
		adminDir := filepath.Join(commonDir, "worktrees", name)
		if err := os.MkdirAll(adminDir, 0o755); err != nil {
			t.Fatalf("mkdir admin %s: %v", adminDir, err)
		}
		if err := os.MkdirAll(wtDir, 0o755); err != nil {
			t.Fatalf("mkdir worktree %s: %v", wtDir, err)
		}
		writeFile(t, filepath.Join(wtDir, ".git"), "gitdir: "+adminDir+"\n")
		writeFile(t, filepath.Join(adminDir, "gitdir"), filepath.Join(wtDir, ".git")+"\n")
		writeFile(t, filepath.Join(adminDir, "commondir"), "../..\n")
		writeFile(t, filepath.Join(adminDir, "HEAD"), head+"\n")
		planted = append(planted, wtDir)
	}
	return planted
}

// assertGitSeesWorktrees is the fixture's own oracle: git must agree that every fabricated
// entry is a registered worktree. Without this the measurement could be sweeping nothing.
func assertGitSeesWorktrees(t testing.TB, work string, want []string) {
	t.Helper()
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	for _, p := range want {
		if !strings.Contains(list, p) {
			t.Fatalf("git does not see the fabricated worktree %s — the fixture is not faithful, so the measurement would measure nothing", p)
		}
	}
}

// perfEnv is withEnv's testing.TB twin: the same isolation (a fixture HOME, a bound
// working directory, a relocated sanctioned prefix) reachable from a Benchmark as well as
// a Test. testing.TB carries TempDir and Cleanup but not Setenv, so the environment is set
// and restored by hand rather than through t.Setenv.
func perfEnv(t testing.TB, work string) {
	t.Helper()
	fixtureHome := t.TempDir()
	setenvTB(t, "HOME", fixtureHome)
	setenvTB(t, "DESK_TOOLS_DISABLED", "")
	setenvTB(t, "CLAUDE_SESSION_ID", "test")

	oldwd := getwd
	getwd = func() (string, error) { return work, nil }
	t.Cleanup(func() { getwd = oldwd })

	oldTmp := tmpBaseDir
	tmpBaseDir = filepath.Join(t.TempDir(), "wtroot")
	if err := os.MkdirAll(tmpBaseDir, 0o755); err != nil {
		t.Fatalf("mkdir tmpBaseDir: %v", err)
	}
	t.Cleanup(func() { tmpBaseDir = oldTmp })
}

func setenvTB(t testing.TB, k, v string) {
	t.Helper()
	old, had := os.LookupEnv(k)
	if err := os.Setenv(k, v); err != nil {
		t.Fatalf("setenv %s: %v", k, err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(k, old)
			return
		}
		_ = os.Unsetenv(k)
	})
}

// newPerfFixture builds the whole synthetic repository and returns its root plus the
// planted worktree paths.
func newPerfFixture(t testing.TB, n, commits int) (string, []string) {
	t.Helper()
	work := newRepo(t)
	perfEnv(t, work)
	tip := buildSyntheticHistory(t, work, commits)
	// Every fabricated worktree sits at a commit one BELOW the tip, so the fresh-worktree
	// guard does not hold them and they reach the merge gate — the gate being measured.
	below := mustGit(t, work, "rev-parse", "refs/heads/main~1")
	if below == tip {
		t.Fatalf("fixture history is too short to place worktrees below the tip")
	}
	side := buildSideCommit(t, work)
	planted := fabricateWorktrees(t, work, n, below, side)
	assertGitSeesWorktrees(t, work, planted)
	return work, planted
}

// TestPruneSweepScalesAtN — the scaling row. It gates on wall time AND on the walk count:
// the wall gate catches a regression on the machine that runs it, and the walk count is
// the machine-independent assertion that origin/main is walked ONCE per sweep however many
// candidates there are. The second is the real invariant; the first is the symptom.
func TestPruneSweepScalesAtN(t *testing.T) {
	n := perfN(t)
	work, planted := newPerfFixture(t, n, 5000)

	guard, err := newPathGuard(work)
	if err != nil {
		t.Fatalf("newPathGuard: %v", err)
	}
	cwd := resolvePath(mustAbsOrRaw(work))

	walks := 0
	origWalk := logOriginMain
	logOriginMain = func(sc *sweepCtx, dir string) ([]string, error) {
		walks++
		return origWalk(sc, dir)
	}
	t.Cleanup(func() { logOriginMain = origWalk })

	start := time.Now()
	res, serr := pruneSweep(guard, work, cwd, pruneOpts{singletonTTL: 0})
	elapsed := time.Since(start)
	if serr != nil {
		t.Fatalf("pruneSweep: %v", serr)
	}

	if walks != 1 {
		t.Fatalf("origin/main was walked %d time(s) for %d candidates — the walk is not hoisted out of the loop", walks, n)
	}
	if budget := perfBudget(n); elapsed > budget {
		t.Fatalf("sweep over %d worktrees took %s, over the %s budget (held %d, removed %d)",
			n, elapsed.Round(time.Millisecond), budget, len(res.skips), res.removed)
	}
	if len(res.skips)+res.removed < len(planted) {
		t.Fatalf("the sweep only accounted for %d of %d planted worktrees — it did not examine the fixture",
			len(res.skips)+res.removed, len(planted))
	}
	t.Logf("N=%d commits=5000 sweep=%s walks=%d removed=%d held=%d",
		n, elapsed.Round(time.Millisecond), walks, res.removed, len(res.skips))
}

// TestConcurrentPrunesCostOne — the singleton's measurement. Five sweeps launched together
// against one repository must cost about one, because exactly one takes the lock and the
// other four report held.
//
// flock is associated with the OPEN FILE DESCRIPTION, not the process, so five independent
// opens inside one test contend exactly as five processes would.
func TestConcurrentPrunesCostOne(t *testing.T) {
	n := perfN(t)
	work, _ := newPerfFixture(t, n, 5000)

	guard, err := newPathGuard(work)
	if err != nil {
		t.Fatalf("newPathGuard: %v", err)
	}
	cwd := resolvePath(mustAbsOrRaw(work))

	// Baseline: one sweep on this fixture, with the singleton disabled entirely.
	start := time.Now()
	if _, serr := pruneSweep(guard, work, cwd, pruneOpts{singletonTTL: 0}); serr != nil {
		t.Fatalf("baseline pruneSweep: %v", serr)
	}
	single := time.Since(start)

	var wg sync.WaitGroup
	var mu sync.Mutex
	swepts, skipped := 0, 0
	start = time.Now()
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// singletonTTL 0: the DEBOUNCE is off, so this measures the LOCK alone.
			_, swept, _, serr := sweepWithSingleton(guard, work, cwd, pruneOpts{singletonTTL: 0})
			mu.Lock()
			defer mu.Unlock()
			if serr != nil {
				t.Errorf("concurrent sweep: %v", serr)
				return
			}
			if swept {
				swepts++
			} else {
				skipped++
			}
		}()
	}
	wg.Wait()
	concurrent := time.Since(start)

	if swepts != 1 || skipped != 4 {
		t.Fatalf("five concurrent sweeps produced %d sweep(s) and %d skip(s), want 1 and 4 — the singleton did not serialise them",
			swepts, skipped)
	}
	// "About one sweep": a generous multiple, because the four skips still pay process and
	// lock overhead, and because the point is to catch FIVE full sweeps (which was 1.64x
	// per-sweep contention on top of five times the work), not to police jitter.
	if budget := 2*single + 5*time.Second; concurrent > budget {
		t.Fatalf("five concurrent sweeps took %s against a single sweep's %s (budget %s) — they are not being deduplicated",
			concurrent.Round(time.Millisecond), single.Round(time.Millisecond), budget.Round(time.Millisecond))
	}
	t.Logf("N=%d single=%s five-concurrent=%s (1 swept, 4 held)",
		n, single.Round(time.Millisecond), concurrent.Round(time.Millisecond))
}

// BenchmarkPruneSweepPerCandidate reports the cost of ONE sweep over the synthetic fixture,
// and — through b.ReportMetric — the cost PER CANDIDATE, which is the number issue #1037
// measured at 0.77-1.28 s and the number a later regression would move. It reports;
// TestPruneSweepScalesAtN is what gates.
func BenchmarkPruneSweepPerCandidate(b *testing.B) {
	n := perfN(b)
	work, planted := newPerfFixture(b, n, 5000)
	guard, err := newPathGuard(work)
	if err != nil {
		b.Fatalf("newPathGuard: %v", err)
	}
	cwd := resolvePath(mustAbsOrRaw(work))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, serr := pruneSweep(guard, work, cwd, pruneOpts{singletonTTL: 0}); serr != nil {
			b.Fatalf("pruneSweep: %v", serr)
		}
	}
	b.StopTimer()
	perCandidate := float64(b.Elapsed().Nanoseconds()) / float64(b.N) / float64(len(planted)) / 1e6
	b.ReportMetric(perCandidate, "ms/candidate")
}

// TestLegacyPerCandidateWalkCost measures, on the SAME fixture, what the sweep used to do
// per candidate: a fresh gitcore.Open (its own object cache), go-git's unmemoized
// IsAncestor, and — for the candidates that come back unmerged, which is 19 in 20 here —
// AheadCount's two full history walks.
//
// It is not a gate. It exists so the improvement is a MEASUREMENT with a baseline attached
// rather than a claim, and so the baseline is re-runnable by a reviewer on their own
// machine instead of being a number in a PR body that nobody can reproduce. It is capped at
// a small sample because the legacy cost is linear in the candidate count by construction —
// that linearity IS the defect.
func TestLegacyPerCandidateWalkCost(t *testing.T) {
	const sample = 20
	work, planted := newPerfFixture(t, sample, 5000)

	start := time.Now()
	for _, rt := range planted {
		repo, err := gitcore.Open(rt) // a FRESH object cache per candidate, as before
		if err != nil {
			t.Fatalf("open %s: %v", rt, err)
		}
		merged, aerr := repo.IsAncestor("HEAD", "refs/remotes/origin/main")
		if aerr != nil {
			t.Fatalf("IsAncestor %s: %v", rt, aerr)
		}
		if !merged {
			if upstream, uerr := repo.UpstreamRef(); uerr == nil {
				_, _ = repo.AheadCount(upstream, "HEAD")
			}
		}
	}
	legacy := time.Since(start)

	guard, err := newPathGuard(work)
	if err != nil {
		t.Fatalf("newPathGuard: %v", err)
	}
	start = time.Now()
	if _, serr := pruneSweep(guard, work, resolvePath(mustAbsOrRaw(work)), pruneOpts{singletonTTL: 0}); serr != nil {
		t.Fatalf("pruneSweep: %v", serr)
	}
	current := time.Since(start)

	t.Logf("BASELINE per-candidate ancestry work over %d candidates on a 5,000-commit main: legacy=%s (%s/candidate); WHOLE current sweep (all gates, same fixture)=%s",
		sample, legacy.Round(time.Millisecond), (legacy / sample).Round(time.Microsecond), current.Round(time.Millisecond))
}
