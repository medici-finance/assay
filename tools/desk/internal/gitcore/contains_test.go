package gitcore

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	commitgraph "github.com/go-git/go-git/v5/plumbing/format/commitgraph/v2"
	"github.com/medici-finance/assay/tools/desk/internal/gittest"
)

// containsFixture is a small but shape-complete history for the --contains contract:
//
//	seed --- c1 --- c2 (main, origin/main, origin/dup) --- l1 (lonely: local only)
//	          \                \
//	           f1 --- f2 (feature, origin/feature)   \
//	                    \                              m (merged, origin/merged)  [m = merge(c2, f2)]
//	                     ----------------------------/
//	origin/behind = seed
//	refs/tags/t-f2      = annotated tag -> f2
//	refs/tags/t-t       = annotated tag -> t-f2 (a tag of a tag)
//	refs/tags/t-tree    = annotated tag -> c2's TREE (not a commit: never listed)
//	refs/tags/lw        = lightweight tag -> f1
//
// Every commit carries the SAME fixed author/committer date (gittest.Fixture pins it),
// so nothing in this test can pass by comparing dates.
type containsFixture struct {
	f                               *gittest.Fixture
	seed, c1, c2, f1, f2, m, l1     string
	commitNames, targetsByName      map[string]string
	interestingTargets, allPrefixes []string
}

func newContainsFixture(t *testing.T) *containsFixture {
	t.Helper()
	f := gittest.NewFixture(t)
	git := func(args ...string) string {
		t.Helper()
		out, err := f.Git(args...)
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return out
	}
	cf := &containsFixture{f: f}
	cf.seed = git("rev-parse", "HEAD")
	cf.c1 = f.CommitFile(t, "a.txt", "1\n", "c1")
	git("checkout", "-q", "-b", "feature")
	cf.f1 = f.CommitFile(t, "f.txt", "1\n", "f1")
	cf.f2 = f.CommitFile(t, "f.txt", "2\n", "f2")
	git("checkout", "-q", "main")
	cf.c2 = f.CommitFile(t, "a.txt", "2\n", "c2")
	git("checkout", "-q", "-b", "merged")
	git("merge", "-q", "--no-ff", "-m", "m", "feature")
	cf.m = git("rev-parse", "HEAD")
	git("checkout", "-q", "-b", "lonely", "main")
	cf.l1 = f.CommitFile(t, "l.txt", "1\n", "l1")
	git("checkout", "-q", "main")

	git("update-ref", "refs/remotes/origin/main", cf.c2)
	git("update-ref", "refs/remotes/origin/dup", cf.c2)
	git("update-ref", "refs/remotes/origin/feature", cf.f2)
	git("update-ref", "refs/remotes/origin/merged", cf.m)
	git("update-ref", "refs/remotes/origin/behind", cf.seed)

	git("tag", "-a", "-m", "t-f2", "t-f2", cf.f2)
	git("tag", "-a", "-m", "t-t", "t-t", "t-f2")
	tree := git("rev-parse", cf.c2+"^{tree}")
	git("tag", "-a", "-m", "t-tree", "t-tree", tree)
	git("tag", "lw", cf.f1)

	cf.commitNames = map[string]string{
		"seed": cf.seed, "c1": cf.c1, "c2": cf.c2, "f1": cf.f1, "f2": cf.f2, "m": cf.m, "l1": cf.l1,
	}
	cf.targetsByName = map[string]string{}
	for k, v := range cf.commitNames {
		cf.targetsByName[k] = v
	}
	cf.targetsByName["t-f2 (annotated tag as target)"] = "t-f2"
	// A tag-of-a-tag is exercised as a ref TIP (refs/tags/t-t below), not as a target:
	// go-git's ResolveRevision, which Repo.Resolve wraps, does not peel a nested tag
	// name ("unsupported object type") — a pre-existing limit of Resolve, and every
	// caller of RefsContaining passes a commit id.
	for k := range cf.targetsByName {
		cf.interestingTargets = append(cf.interestingTargets, k)
	}
	sort.Strings(cf.interestingTargets)
	cf.allPrefixes = []string{"refs/remotes/", "refs/tags/", "refs/heads/", "refs/"}
	return cf
}

// wantContaining is the oracle: real git's answer, minus symbolic refs (which
// gitcore.Refs does not surface — see RefsContaining's doc).
func (cf *containsFixture) wantContaining(t *testing.T, target, prefix string) []string {
	t.Helper()
	out, err := cf.f.Git("for-each-ref", "--contains="+target, "--format=%(refname)", prefix)
	if err != nil {
		t.Fatalf("for-each-ref --contains=%s %s: %v", target, prefix, err)
	}
	var want []string
	for _, ln := range strings.Split(out, "\n") {
		if ln == "" {
			continue
		}
		if _, serr := cf.f.Git("symbolic-ref", "-q", ln); serr == nil {
			continue
		}
		want = append(want, ln)
	}
	sort.Strings(want)
	return want
}

// assertMatchesGit checks every interesting target against every prefix, on a FRESH
// Repo (so the index is built from scratch under whatever on-disk state the caller
// arranged), and returns that Repo for stats inspection.
func (cf *containsFixture) assertMatchesGit(t *testing.T, mode string) *Repo {
	t.Helper()
	repo, err := Open(cf.f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	nonEmpty := 0
	for _, name := range cf.interestingTargets {
		target := cf.targetsByName[name]
		for _, prefix := range cf.allPrefixes {
			want := cf.wantContaining(t, target, prefix)
			got, err := repo.RefsContaining(target, prefix)
			if err != nil {
				t.Fatalf("[%s] RefsContaining(%s, %s): %v", mode, name, prefix, err)
			}
			if got == nil {
				got = []string{}
			}
			if want == nil {
				want = []string{}
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("[%s] RefsContaining(%s=%s, %s) = %v, want (git) %v", mode, name, target, prefix, got, want)
			}
			nonEmpty += len(want)
		}
	}
	if nonEmpty == 0 {
		t.Fatalf("[%s] COULD-NOT-CHECK: git listed no containing refs for any target", mode)
	}
	// The negative side must be exercised too: l1 is on no remote ref at all.
	if got, _ := repo.RefsContaining(cf.l1, "refs/remotes/"); len(got) != 0 {
		t.Fatalf("[%s] l1 must be contained by no remote ref, got %v", mode, got)
	}
	return repo
}

// TestRefsContainingMatchesGitAcrossGraphStates is the contract test in every on-disk
// state the accelerator can meet: no commit-graph at all, a complete graph, a STALE graph
// (target and some tips newer than the file), a split chain, and a graph the config
// disables. The answer must be git's in all of them.
func TestRefsContainingMatchesGitAcrossGraphStates(t *testing.T) {
	t.Run("no-commit-graph", func(t *testing.T) {
		cf := newContainsFixture(t)
		repo := cf.assertMatchesGit(t, "no-graph")
		if st := repo.reachStats(); st.graph || st.graphLoads != 0 {
			t.Fatalf("no commit-graph on disk, yet index reports graph=%v graphLoads=%d", st.graph, st.graphLoads)
		}
	})
	t.Run("complete-commit-graph", func(t *testing.T) {
		cf := newContainsFixture(t)
		if _, err := cf.f.Git("commit-graph", "write", "--reachable"); err != nil {
			t.Skipf("git commit-graph write unavailable: %v", err)
		}
		repo := cf.assertMatchesGit(t, "full-graph")
		st := repo.reachStats()
		if !st.graph {
			t.Fatal("commit-graph written but not opened — the accelerator is not being exercised")
		}
		if st.graphLoads == 0 {
			t.Fatal("commit-graph opened but no commit was served from it")
		}
	})
	t.Run("stale-commit-graph", func(t *testing.T) {
		cf := newContainsFixture(t)
		// Write the graph BEFORE the history grows: everything after this line is
		// outside the file, exactly like a brand-new local commit at push time.
		if _, err := cf.f.Git("commit-graph", "write", "--reachable"); err != nil {
			t.Skipf("git commit-graph write unavailable: %v", err)
		}
		if _, err := cf.f.Git("checkout", "-q", "feature"); err != nil {
			t.Fatal(err)
		}
		f3 := cf.f.CommitFile(t, "f.txt", "3\n", "f3 (after graph)")
		if _, err := cf.f.Git("checkout", "-q", "main"); err != nil {
			t.Fatal(err)
		}
		if _, err := cf.f.Git("update-ref", "refs/remotes/origin/feature", f3); err != nil {
			t.Fatal(err)
		}
		cf.commitNames["f3"], cf.targetsByName["f3 (outside graph)"] = f3, f3
		cf.interestingTargets = append(cf.interestingTargets, "f3 (outside graph)")
		repo := cf.assertMatchesGit(t, "stale-graph")
		st := repo.reachStats()
		if !st.graph || st.graphLoads == 0 || st.odbLoads == 0 {
			t.Fatalf("stale graph must serve old commits from the file AND new ones from the ODB; got %+v", st)
		}
	})
	t.Run("split-chain", func(t *testing.T) {
		cf := newContainsFixture(t)
		if _, err := cf.f.Git("commit-graph", "write", "--reachable", "--split=no-merge"); err != nil {
			t.Skipf("git commit-graph write --split unavailable: %v", err)
		}
		f3 := cf.f.CommitFile(t, "z.txt", "1\n", "z1 (second layer)")
		if _, err := cf.f.Git("update-ref", "refs/remotes/origin/main", f3); err != nil {
			t.Fatal(err)
		}
		if _, err := cf.f.Git("commit-graph", "write", "--reachable", "--split=no-merge"); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(cf.f.Dir + "/.git/objects/info/commit-graphs/commit-graph-chain"); err != nil {
			t.Skipf("git did not produce a commit-graph chain: %v", err)
		}
		cf.commitNames["z1"], cf.targetsByName["z1 (second layer)"] = f3, f3
		cf.interestingTargets = append(cf.interestingTargets, "z1 (second layer)")
		repo := cf.assertMatchesGit(t, "split-chain")
		if st := repo.reachStats(); !st.graph {
			t.Fatal("commit-graph chain written but not opened")
		}
	})
	t.Run("core.commitGraph=false", func(t *testing.T) {
		cf := newContainsFixture(t)
		if _, err := cf.f.Git("commit-graph", "write", "--reachable"); err != nil {
			t.Skipf("git commit-graph write unavailable: %v", err)
		}
		if _, err := cf.f.Git("config", "core.commitGraph", "false"); err != nil {
			t.Fatal(err)
		}
		repo := cf.assertMatchesGit(t, "graph-disabled")
		if st := repo.reachStats(); st.graph {
			t.Fatal("core.commitGraph=false must leave the commit-graph unopened, as git does")
		}
	})
	t.Run("shallow-clone", func(t *testing.T) {
		// A shallow repository's boundary commits have parents that are absent BY
		// DESIGN (.git/shallow), not a broken object store. git's --contains treats
		// each one as parentless and answers from the history it has; so must this,
		// including for the push-time shape — a fresh local commit contained by no
		// ref, where the walk explores every tip's whole (truncated) history and
		// there is no commit-graph to cut it short.
		cf := newContainsFixture(t)
		dir := t.TempDir()
		gitIn := func(args ...string) string {
			t.Helper()
			cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("git -C <clone> %v: %v\n%s", args, err, out)
			}
			return strings.TrimSpace(string(out))
		}
		// --no-single-branch: every fixture branch tip arrives; --depth 1: every tip IS
		// a boundary. That covers both shapes of the bug: a boundary whose parent is
		// absent (feature's f2 -> f1, lonely's l1 -> c1: an eager parent load errors),
		// and a boundary whose parents happen to be present as other tips (merged's
		// m -> c2, f2) — git still treats m as parentless, so origin/merged must NOT be
		// listed as containing c2, even though the objects to walk there exist.
		clone := exec.Command("git", "clone", "-q", "--depth", "1", "--no-single-branch", "file://"+cf.f.Dir, dir)
		if out, err := clone.CombinedOutput(); err != nil {
			t.Skipf("git clone --depth unavailable: %v\n%s", err, out)
		}
		shallowFile, err := os.ReadFile(dir + "/.git/shallow")
		if err != nil || len(strings.TrimSpace(string(shallowFile))) == 0 {
			t.Skipf("clone produced no shallow boundary: %v", err)
		}
		boundary := strings.Fields(string(shallowFile))[0]
		newTip := ""
		{
			cmd := exec.Command("git", "-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid",
				"commit", "-q", "--allow-empty", "-m", "local tip")
			if out, cerr := cmd.CombinedOutput(); cerr != nil {
				t.Fatalf("local commit: %v\n%s", cerr, out)
			}
			newTip = gitIn("rev-parse", "HEAD")
		}
		targets := map[string]string{
			"local tip (on no ref)": newTip,
			"origin/main tip":       gitIn("rev-parse", "refs/remotes/origin/main"),
			"shallow boundary":      boundary,
		}
		repo, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		for name, target := range targets {
			for _, prefix := range []string{"refs/remotes/", "refs/heads/", "refs/"} {
				var want []string
				for _, ln := range strings.Split(gitIn("for-each-ref", "--contains="+target, "--format=%(refname)", prefix), "\n") {
					if ln == "" {
						continue
					}
					if err := exec.Command("git", "-C", dir, "symbolic-ref", "-q", ln).Run(); err == nil {
						continue // origin/HEAD: symbolic, not surfaced by gitcore.Refs
					}
					want = append(want, ln)
				}
				sort.Strings(want)
				got, gerr := repo.RefsContaining(target, prefix)
				if gerr != nil {
					t.Fatalf("shallow clone: RefsContaining(%s, %s) errored: %v — git answers %v; a shallow "+
						"boundary is not a broken object store", name, prefix, gerr, want)
				}
				if len(got) == 0 && len(want) == 0 {
					continue
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("shallow clone: RefsContaining(%s, %s) = %v, want %v (git)", name, prefix, got, want)
				}
			}
		}
		if st := repo.reachStats(); st.shallowCuts == 0 {
			t.Fatalf("the walk never reached the shallow boundary — the fixture no longer exercises it: %+v", st)
		}
	})
}

// TestRefsContainingErrorsOnUnanswerable pins the guard-facing contract: a target that
// is not a readable commit is an ERROR (could-not-check), never an empty answer that a
// security caller would read as "no ref contains it".
func TestRefsContainingErrorsOnUnanswerable(t *testing.T) {
	cf := newContainsFixture(t)
	repo, err := Open(cf.f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := repo.RefsContaining("t-tree", "refs/"); err == nil {
		t.Fatalf("a tag of a tree is not a commit: want error, got %v", got)
	}
	if got, err := repo.RefsContaining("no-such-rev", "refs/"); err == nil {
		t.Fatalf("an unresolvable target: want error, got %v", got)
	}
	if got, err := repo.RefsContaining(strings.Repeat("0", 40), "refs/"); err == nil {
		t.Fatalf("a missing object id: want error, got %v", got)
	}
}

// linearFixture builds a straight history of n commits (seed included) with a
// remote-tracking ref every `every` commits, and returns the commit shas oldest-first.
func linearFixture(t testing.TB, n, every int) (*gittest.Fixture, []string) {
	t.Helper()
	f := &gittest.Fixture{Dir: t.TempDir()}
	must := func(args ...string) string {
		t.Helper()
		out, err := f.Git(args...)
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return out
	}
	must("init", "-q", "-b", "main")
	must("config", "user.name", "test")
	must("config", "user.email", "test@example.invalid")
	shas := make([]string, 0, n)
	for i := 0; i < n; i++ {
		must("commit", "-q", "--allow-empty", "-m", fmt.Sprintf("c%d", i))
		shas = append(shas, must("rev-parse", "HEAD"))
		if i%every == 0 {
			must("update-ref", fmt.Sprintf("refs/remotes/origin/b%d", i), shas[i])
		}
	}
	return f, shas
}

// TestRefsContainingSharesOneWalk is the regression guard for the N-independent-walks
// defect, stated in WORK rather than wall-clock so it cannot flake: for a brand-new tip
// contained by no ref, the pre-fix code decoded ~(refs x history / 2) commits; the fix
// decodes each commit at most once, and with a commit-graph present decodes almost
// none (generation numbers cut every tip off immediately).
func TestRefsContainingSharesOneWalk(t *testing.T) {
	const commits, every = 240, 4
	f, shas := linearFixture(t, commits, every)
	refs := commits / every
	// A fresh local commit off the tip: on no remote ref, the worst case.
	newTip := f.CommitFile(t, "new.txt", "new\n", "new tip")
	regressed := refs * commits / 2 // the order of work the per-ref walk did

	t.Run("no-commit-graph", func(t *testing.T) {
		repo, err := Open(f.Dir)
		if err != nil {
			t.Fatal(err)
		}
		got, err := repo.RefsContaining(newTip, "refs/remotes/")
		if err != nil || len(got) != 0 {
			t.Fatalf("RefsContaining(newTip) = %v, %v; want empty, nil", got, err)
		}
		st := repo.reachStats()
		if st.odbLoads > commits+2 {
			t.Fatalf("decoded %d commits for %d refs over %d commits — the shared walk has regressed "+
				"(one decode per commit expected, the old per-ref walk did ~%d)", st.odbLoads, refs, commits, regressed)
		}
		// And a hit: the oldest commit is on every ref.
		got, err = repo.RefsContaining(shas[0], "refs/remotes/")
		if err != nil || len(got) != refs {
			t.Fatalf("RefsContaining(root) = %d refs, %v; want %d", len(got), err, refs)
		}
		if st2 := repo.reachStats(); st2.odbLoads > commits+2 {
			t.Fatalf("second call decoded again (%d total) — the per-Repo node cache is not shared", st2.odbLoads)
		}
	})

	t.Run("with-commit-graph", func(t *testing.T) {
		if _, err := f.Git("commit-graph", "write", "--reachable"); err != nil {
			t.Skipf("git commit-graph write unavailable: %v", err)
		}
		// newTip is IN this graph now (it was reachable from HEAD); cut a newer one so
		// the target is, as at push time, outside the file.
		newer := f.CommitFile(t, "new.txt", "newer\n", "newer tip")
		repo, err := Open(f.Dir)
		if err != nil {
			t.Fatal(err)
		}
		got, err := repo.RefsContaining(newer, "refs/remotes/")
		if err != nil || len(got) != 0 {
			t.Fatalf("RefsContaining(newer) = %v, %v; want empty, nil", got, err)
		}
		st := repo.reachStats()
		if !st.graph {
			t.Fatal("commit-graph not opened")
		}
		if st.odbLoads > 2 {
			t.Fatalf("with a commit-graph only the out-of-graph target (and at most its parent) should hit the ODB; decoded %d", st.odbLoads)
		}
		if st.graphLoads >= commits {
			t.Fatalf("generation cut-off did not engage: %d graph loads for %d refs (whole history is %d)", st.graphLoads, refs, commits)
		}
		// Exactness under the cut-off: git's answer for a mid-history commit.
		mid := shas[commits/2]
		want, err := f.Git("for-each-ref", "--contains="+mid, "--format=%(refname)", "refs/remotes/")
		if err != nil {
			t.Fatal(err)
		}
		wantList := strings.Split(want, "\n")
		sort.Strings(wantList)
		got, err = repo.RefsContaining(mid, "refs/remotes/")
		if err != nil || !reflect.DeepEqual(got, wantList) {
			t.Fatalf("RefsContaining(mid) = %v, %v; want %v", got, err, wantList)
		}
	})
}

// BenchmarkRefsContaining measures the worst case (a fresh tip on no ref) with and
// without a commit-graph. Run with:
//
//	go test ./internal/gitcore/ -run '^$' -bench RefsContaining -benchmem
func BenchmarkRefsContaining(b *testing.B) {
	f, _ := linearFixture(b, 400, 2)
	if _, err := f.Git("commit", "-q", "--allow-empty", "-m", "new tip"); err != nil {
		b.Fatal(err)
	}
	newTip, err := f.Git("rev-parse", "HEAD")
	if err != nil {
		b.Fatal(err)
	}
	for _, withGraph := range []bool{false, true} {
		name := "no-graph"
		if withGraph {
			if _, err := f.Git("commit-graph", "write", "--reachable"); err != nil {
				b.Skipf("git commit-graph write unavailable: %v", err)
			}
			name = "commit-graph"
		}
		b.Run(name+"/fresh-index", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				repo, err := Open(f.Dir)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := repo.RefsContaining(newTip, "refs/remotes/"); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(name+"/warm-index", func(b *testing.B) {
			repo, err := Open(f.Dir)
			if err != nil {
				b.Fatal(err)
			}
			for i := 0; i < b.N; i++ {
				if _, err := repo.RefsContaining(newTip, "refs/remotes/"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestRefsContainingRealRepoTiming is an OPT-IN measurement against a real checkout —
// the regression was only ever visible at real scale (~1000 refs over ~17k commits).
// Point GITCORE_BENCH_REPO at a worktree; the test asserts git's answer for HEAD over
// refs/remotes/ and that the call completes well inside the desk preflight's 45s budget.
func TestRefsContainingRealRepoTiming(t *testing.T) {
	dir := os.Getenv("GITCORE_BENCH_REPO")
	if dir == "" {
		t.Skip("set GITCORE_BENCH_REPO=<worktree> to time RefsContaining against a real repository")
	}
	f := &gittest.Fixture{Dir: dir}
	head, err := f.Git("rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	nRefs, _ := f.Git("for-each-ref", "--format=%(refname)", "refs/remotes/")
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	got, err := repo.RefsContaining(head, "refs/remotes/")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	st := repo.reachStats()
	t.Logf("RefsContaining(HEAD, refs/remotes/) over %d refs: %v (graph=%v graphLoads=%d odbLoads=%d) -> %d refs",
		len(strings.Split(nRefs, "\n")), elapsed, st.graph, st.graphLoads, st.odbLoads, len(got))
	start = time.Now()
	if _, err := repo.RefsContaining(head, "refs/remotes/"); err != nil {
		t.Fatal(err)
	}
	t.Logf("second call (warm index): %v", time.Since(start))
	wantOut, err := f.Git("for-each-ref", "--contains="+head, "--format=%(refname)", "refs/remotes/")
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, ln := range strings.Split(wantOut, "\n") {
		if ln == "" {
			continue
		}
		if _, serr := f.Git("symbolic-ref", "-q", ln); serr == nil {
			continue
		}
		want = append(want, ln)
	}
	sort.Strings(want)
	if got == nil {
		got = []string{}
	}
	if want == nil {
		want = []string{}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RefsContaining(HEAD) = %v, want (git) %v", got, want)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("RefsContaining took %v — the shared-walk fix has regressed (budget: well under the 45s preflight probe)", elapsed)
	}
}

// zeroLevelIndex wraps a real commit-graph index and reports every commit's level as 0
// ("not available"), the shape of a graph written by git before 2.19 stored generation
// numbers. Parents and lookups are the real file's.
type zeroLevelIndex struct{ commitgraph.Index }

func (z zeroLevelIndex) GetCommitDataByIndex(i uint32) (*commitgraph.CommitData, error) {
	cd, err := z.Index.GetCommitDataByIndex(i)
	if err != nil {
		return nil, err
	}
	c := *cd
	c.Generation = 0
	return &c, nil
}

// TestRefsContainingZeroLevelGraphDerivesEachNodeOnce pins generation()'s work bound on
// a graph whose stored levels are all zero, the case where every level must be derived
// from parents: derivation resolves from the roots (a root is level 1), so every node is
// derived exactly once and never re-entered from a second child. A chain of nested merge
// diamonds is the shape where any re-derivation would compound exponentially; the bound
// holds it to one push per commit, and the answer stays git's.
func TestRefsContainingZeroLevelGraphDerivesEachNodeOnce(t *testing.T) {
	const diamonds = 10
	f := gittest.NewFixture(t)
	git := func(args ...string) string {
		t.Helper()
		out, err := f.Git(args...)
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return out
	}
	for i := 0; i < diamonds; i++ {
		side := fmt.Sprintf("side-%d", i)
		git("checkout", "-q", "-b", side)
		f.CommitFile(t, "side.txt", fmt.Sprintf("%d\n", i), side)
		git("checkout", "-q", "main")
		f.CommitFile(t, "main.txt", fmt.Sprintf("%d\n", i), fmt.Sprintf("main-%d", i))
		git("merge", "-q", "--no-ff", "-m", fmt.Sprintf("merge-%d", i), side)
	}
	git("update-ref", "refs/remotes/origin/main", "HEAD")
	if _, err := f.Git("commit-graph", "write", "--reachable"); err != nil {
		t.Skipf("git commit-graph write unavailable: %v", err)
	}
	// A commit outside the graph, so its level is derived from parents at call time.
	newTip := f.CommitFile(t, "new.txt", "new\n", "new tip")
	commits, err := strconv.Atoi(strings.TrimSpace(git("rev-list", "--count", "--all")))
	if err != nil {
		t.Fatal(err)
	}

	repo, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	ri := repo.reach()
	ri.mu.Lock()
	if ri.graph == nil {
		ri.mu.Unlock()
		t.Fatal("commit-graph not opened")
	}
	ri.graph = zeroLevelIndex{ri.graph}
	ri.mu.Unlock()

	got, err := repo.RefsContaining(newTip, "refs/remotes/")
	if err != nil || len(got) != 0 {
		t.Fatalf("RefsContaining(newTip) = %v, %v; want empty, nil", got, err)
	}
	st := repo.reachStats()
	t.Logf("all-zero-level graph: %d commits, %d derivation pushes", commits, st.genPushes)
	if st.genPushes > commits {
		t.Fatalf("generation() pushed %d frames over %d commits — a node is being re-derived on "+
			"every path that reaches it (%d nested diamonds)", st.genPushes, commits, diamonds)
	}
	// Exactness is unaffected by the missing levels: git's answer for the root.
	root := strings.TrimSpace(git("rev-list", "--max-parents=0", "HEAD"))
	want := strings.Split(strings.TrimSpace(git("for-each-ref", "--contains="+root, "--format=%(refname)", "refs/remotes/")), "\n")
	sort.Strings(want)
	got, err = repo.RefsContaining(root, "refs/remotes/")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("RefsContaining(root) = %v, %v; want %v", got, err, want)
	}
}
