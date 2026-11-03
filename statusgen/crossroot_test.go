package main

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// crossroot_test.go — the cross-root delivery witness. A brief's file lives in
// root A; the PR that delivered it merged in root B and carries the brief's
// `Brief:` trailer. Both roots are configured (DESK_ROOTS). The planner must
// hold the row out of Next-up and reconcile must write it with root B's PR as
// the witness. Every fixture names example-org/* only.

// crossRootRepo is root B's <owner>/<repo> key in the operator map.
const crossRootRepo = "example-org/root-b"

// newCrossRootB builds root B: a real git repo whose first-parent history holds
// one squash-merged PR (#7) carrying msg. It has no docs/streams tree, so it
// owns none of root A's streams. Its integration branch (refs/remotes/origin/main,
// what the forge last confirmed) is at the merge, as in a real clone.
func newCrossRootB(t *testing.T, msg string) string {
	t.Helper()
	dir := t.TempDir()
	gitInit(t, dir, "Root B Bot", "root-b-bot@example.com")
	commitEmpty(t, dir, "chore: init")
	commitEmpty(t, dir, msg)
	runGit(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	return dir
}

// gitCommitTree turns root A's on-disk fixture into a git repo with one plain
// commit, so its own merge-history read succeeds (and finds no merged PR).
func gitCommitTree(t *testing.T, dir string) {
	t.Helper()
	gitInit(t, dir, "Root A Bot", "root-a-bot@example.com")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "chore: board fixture")
}

// withCrossRoot points DESK_ROOTS at root B (and only root B), with the
// sibling-merge opt-in OFF: the cross-root witness must not depend on it.
func withCrossRoot(t *testing.T, rootB string) {
	t.Helper()
	t.Setenv("DESK_ROOTS", crossRootRepo+"="+rootB)
	t.Setenv("ASSAY_SIBLING_MERGE", "")
	orig := siblingMergeFlagValue
	siblingMergeFlagValue = false
	t.Cleanup(func() { siblingMergeFlagValue = orig })
}

// withCrossRoots points DESK_ROOTS at several roots, repo key -> checkout path,
// with the sibling-merge opt-in OFF.
func withCrossRoots(t *testing.T, roots map[string]string) {
	t.Helper()
	var keys []string
	for k := range roots {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"="+roots[k])
	}
	t.Setenv("DESK_ROOTS", strings.Join(parts, ","))
	t.Setenv("ASSAY_SIBLING_MERGE", "")
	orig := siblingMergeFlagValue
	siblingMergeFlagValue = false
	t.Cleanup(func() { siblingMergeFlagValue = orig })
}

// publishRoots makes the board at root publish each repo in its
// docs/streams/graph-repos.yaml (creating the registry when the fixture has
// none): the only way a configured root is read for a delivery.
func publishRoots(t *testing.T, board string, repos ...string) {
	t.Helper()
	path := filepath.Join(board, "docs", "streams", "graph-repos.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		raw = []byte("schema: graph-repos-v1\ncell: fx\nrepos:\n")
	}
	body := string(raw)
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	for i, r := range repos {
		body += "  pubroot" + string(rune('a'+i)) + ": {cell: fx, repo: " + r + "}\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// carryStream makes the checkout at dir carry docs/streams/<stream>.
func carryStream(t *testing.T, dir, stream string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "streams", stream), 0o755); err != nil {
		t.Fatal(err)
	}
}

// runReconcileBoth runs reconcile and returns its exit code, stdout and stderr.
func runReconcileBoth(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	errf, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	code = runReconcile(args, out, errf)
	b, _ := os.ReadFile(out.Name())
	e, _ := os.ReadFile(errf.Name())
	return code, string(b), string(e)
}

// offered reports whether id is on --next-up.
func offered(t *testing.T, root, id string) bool {
	t.Helper()
	for _, got := range nextUpRows(t, root) {
		if got == id {
			return true
		}
	}
	return false
}

func nextUpRows(t *testing.T, root string) []string {
	t.Helper()
	out := captureStdout(t, func() {
		if code := runNextUp(root); code != 0 {
			t.Fatalf("runNextUp exited %d", code)
		}
	})
	var view dispatchView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("--next-up did not emit valid JSON: %v\n%s", err, out)
	}
	var ids []string
	for _, r := range view.Rows {
		ids = append(ids, r.Brief)
	}
	return ids
}

// regression: #4614
func TestCrossRootPlannerHolds(t *testing.T) {
	rootA := siblingBriefTree(t, "")
	gitCommitTree(t, rootA)
	publishRoots(t, rootA, crossRootRepo, crossRootC)
	rootB := newCrossRootB(t, "feat: land the thing (#7)\n\nBody.\n\nBrief: example-stream/02\n")
	withCrossRoot(t, rootB)

	for _, id := range nextUpRows(t, rootA) {
		if id == "example-stream/02" {
			t.Fatalf("--next-up offers example-stream/02, delivered by %s PR #7 (Brief: trailer)", crossRootRepo)
		}
	}

	if code := run(rootA, "write", nil, nil, ""); code != 0 {
		t.Fatalf("write exited %d", code)
	}
	status, err := os.ReadFile(filepath.Join(rootA, "STATUS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "example-stream/02 → already-merged-unflipped (merged PR " + crossRootRepo + "#7, Brief: trailer)"; !strings.Contains(string(status), want) {
		t.Errorf("STATUS.md must name the cross-root hold %q:\n%s", want, status)
	}
}

// TestCrossRootMentionIsNoWitness is the negative control: a root-B merge that
// names the brief only in its subject (no `Brief:` trailer) is a lead, never
// delivery — the row stays offered.
func TestCrossRootMentionIsNoWitness(t *testing.T) {
	rootA := siblingBriefTree(t, "")
	gitCommitTree(t, rootA)
	publishRoots(t, rootA, crossRootRepo, crossRootC)
	rootB := newCrossRootB(t, "feat: about example-stream/02 (#7)")
	withCrossRoot(t, rootB)

	found := false
	for _, id := range nextUpRows(t, rootA) {
		found = found || id == "example-stream/02"
	}
	if !found {
		t.Fatal("a subject-only mention in another root must not hold the row")
	}
}

// TestCrossRootUnreadableRoot pins three-state: a configured root that is not a
// git checkout is could-not-check, and the row is not held on its say-so.
// regression: #4614
func TestCrossRootUnreadableRoot(t *testing.T) {
	rootA := siblingBriefTree(t, "")
	gitCommitTree(t, rootA)
	publishRoots(t, rootA, crossRootRepo, crossRootC)
	withCrossRoot(t, filepath.Join(t.TempDir(), "absent"))

	stderr := captureStderr(t, func() { _ = run(rootA, "lint", nil, nil, "") })
	if !strings.Contains(stderr, "could-not-check: cross-root delivery witness: "+crossRootRepo) {
		t.Errorf("an unreadable configured root must report could-not-check; stderr:\n%s", stderr)
	}
	found := false
	for _, id := range nextUpRows(t, rootA) {
		found = found || id == "example-stream/02"
	}
	if !found {
		t.Fatal("an unreadable root is not evidence of delivery; the row must stay offered")
	}
}

// regression: #4614
func TestCrossRootReconcileWrites(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	rootA := copyDesignGateFixture(t)
	publishRoots(t, rootA, crossRootRepo)
	// Root A's forge has no PR for dg/03 at all; its delivery merged in root B.
	stubPullsServer(t, http.StatusOK, []map[string]any{})
	rootB := newCrossRootB(t, "feat: implement dg/03 (#7)\n\nBrief: dg/03\n")
	withCrossRoot(t, rootB)

	code, stdout := runReconcileCaptured(t, "--apply", "--root", rootA, "--repo", "example-org/root-a", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d, want 0; stdout:\n%s", code, stdout)
	}
	var res reconcileResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("decoding --json: %v\n%s", err, stdout)
	}
	if len(res.Applied) != 1 || res.Applied[0].ID != "dg/03" {
		t.Fatalf("want dg/03 applied on the cross-root witness, got %+v", res.Applied)
	}
	if w := res.Applied[0].Witness; !strings.HasPrefix(w, "PR "+crossRootRepo+"#7 (merged ") {
		t.Errorf("the witness must name root B's PR, got %q", w)
	}
	if row := res.Applied[0].RowAfter; res.Applied[0].To != "implemented" || !strings.Contains(readFixtureReadme(t, rootA), row) {
		t.Errorf("dg/03's row must be written implemented; to=%q row=%q", res.Applied[0].To, row)
	}
}

// TestDeskRootsCleared pins TestMain's unset: a DESK_ROOTS set by the caller
// must not reach the tests, or every fixture board would read the operator's
// real checkouts as cross-root witnesses.
// regression: #4614
func TestDeskRootsCleared(t *testing.T) {
	if os.Getenv("STATUSGEN_DESKROOTS_PROBE") == "1" {
		if v, ok := os.LookupEnv("DESK_ROOTS"); ok {
			t.Fatalf("the test binary inherited DESK_ROOTS=%q", v)
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestDeskRootsCleared$", "-test.count=1")
	cmd.Env = append(os.Environ(), "DESK_ROOTS=example-org/root-b=/nonexistent", "STATUSGEN_DESKROOTS_PROBE=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("a DESK_ROOTS set by the caller reached the tests: %v\n%s", err, out)
	}
}

// ---------------------------------------------------------------------------
// Review round 1: what counts as a cross-root witness.
// ---------------------------------------------------------------------------

// unmergedRootB is root B whose integration branch holds only "chore: init" and
// whose checked-out HEAD sits on a branch carrying msg: a closed PR's head, an
// unpushed commit or a contributor's branch. msg is NOT a merge.
func unmergedRootB(t *testing.T, msg string) string {
	t.Helper()
	dir := t.TempDir()
	gitInit(t, dir, "Root B Bot", "root-b-bot@example.com")
	commitEmpty(t, dir, "chore: init")
	runGit(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	runGit(t, dir, "checkout", "-b", "closed-pr-head")
	commitEmpty(t, dir, msg)
	return dir
}

// reconcileDG runs reconcile --apply over a fresh design-gate board whose forge
// has no PR, and returns the applied "<id> <witness>" lines and stderr.
func reconcileDG(t *testing.T) (applied []string, stderr string) {
	t.Helper()
	applied, _, stderr = reconcileDGPublishing(t, crossRootRepo, crossRootC)
	return applied, stderr
}

// crossRootC is a third configured root's key.
const crossRootC = "example-org/root-c"

// reconcileDGPublishing is reconcileDG over a board whose registry publishes
// exactly the given repos. It also returns reconcile's stdout.
func reconcileDGPublishing(t *testing.T, published ...string) (applied []string, stdout, stderr string) {
	t.Helper()
	t.Setenv("GITHUB_TOKEN", "test-token")
	rootA := copyDesignGateFixture(t)
	if len(published) > 0 {
		publishRoots(t, rootA, published...)
	}
	stubPullsServer(t, http.StatusOK, []map[string]any{})
	code, stdout, stderr := runReconcileBoth(t, "--apply", "--root", rootA, "--repo", "example-org/root-a", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d, want 0; stdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	var res reconcileResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("decoding --json: %v\n%s", err, stdout)
	}
	for _, a := range res.Applied {
		applied = append(applied, a.ID+" "+a.Witness)
	}
	return applied, stdout, stderr
}

// A merge is a commit on the other root's integration branch. A checkout's HEAD
// that sits on a never-merged branch is no witness, for the planner or reconcile.
// regression: #4614
func TestCrossRootUnmergedHead(t *testing.T) {
	t.Run("planner", func(t *testing.T) {
		rootA := siblingBriefTree(t, "")
		gitCommitTree(t, rootA)
		publishRoots(t, rootA, crossRootRepo, crossRootC)
		withCrossRoot(t, unmergedRootB(t, "feat: never merged (#7)\n\nBrief: example-stream/02\n"))
		if !offered(t, rootA, "example-stream/02") {
			t.Fatal("a commit on an unmerged branch at root B's HEAD held the row")
		}
	})
	t.Run("reconcile", func(t *testing.T) {
		withCrossRoot(t, unmergedRootB(t, "feat: never merged (#7)\n\nBrief: dg/03\n"))
		if applied, _ := reconcileDG(t); len(applied) != 0 {
			t.Fatalf("an unmerged branch head was written as a merged delivery: %v", applied)
		}
	})
}

// A root whose integration branch cannot be resolved is could-not-check, not
// "read HEAD instead".
// regression: #4614
func TestCrossRootNoIntegrationRef(t *testing.T) {
	rootA := siblingBriefTree(t, "")
	gitCommitTree(t, rootA)
	publishRoots(t, rootA, crossRootRepo, crossRootC)
	rootB := newCrossRootB(t, "feat: land the thing (#7)\n\nBrief: example-stream/02\n")
	runGit(t, rootB, "update-ref", "-d", "refs/remotes/origin/main")
	withCrossRoot(t, rootB)

	stderr := captureStderr(t, func() { _ = run(rootA, "lint", nil, nil, "") })
	if !strings.Contains(stderr, "could-not-check: cross-root delivery witness: "+crossRootRepo) ||
		!strings.Contains(stderr, "integration branch not resolvable") {
		t.Errorf("an unresolvable integration branch must be could-not-check; stderr:\n%s", stderr)
	}
	if !offered(t, rootA, "example-stream/02") {
		t.Fatal("an unresolvable integration branch held the row")
	}
}

// The integration branch is what origin/HEAD names, not a fixed "main".
// regression: #4614
func TestCrossRootOriginHeadRef(t *testing.T) {
	rootA := siblingBriefTree(t, "")
	gitCommitTree(t, rootA)
	publishRoots(t, rootA, crossRootRepo, crossRootC)
	rootB := newCrossRootB(t, "feat: land the thing (#7)\n\nBrief: example-stream/02\n")
	runGit(t, rootB, "update-ref", "refs/remotes/origin/trunk", "refs/remotes/origin/main")
	runGit(t, rootB, "update-ref", "-d", "refs/remotes/origin/main")
	runGit(t, rootB, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")
	withCrossRoot(t, rootB)
	if offered(t, rootA, "example-stream/02") {
		t.Fatal("a merge on the branch origin/HEAD names did not hold the row")
	}
}

// One merge carrying two distinct `Brief:` trailers is not a derivable
// PR->brief edge (the own-repo rule): it witnesses neither brief.
// regression: #4614
func TestCrossRootMultiLinkNoWitness(t *testing.T) {
	t.Run("planner", func(t *testing.T) {
		rootA := siblingBriefTree(t, "")
		gitCommitTree(t, rootA)
		publishRoots(t, rootA, crossRootRepo, crossRootC)
		withCrossRoot(t, newCrossRootB(t, "feat: two (#8)\n\nBrief: example-stream/02\nBrief: example-stream/03\n"))
		stderr := captureStderr(t, func() { _ = run(rootA, "lint", nil, nil, "") })
		if !offered(t, rootA, "example-stream/02") {
			t.Fatal("a multi-linked merge held the row")
		}
		if !strings.Contains(stderr, "multi-linked merge is not a derivable delivery edge") {
			t.Errorf("the multi-linked merge must be reported; stderr:\n%s", stderr)
		}
	})
	t.Run("reconcile", func(t *testing.T) {
		withCrossRoot(t, newCrossRootB(t, "feat: two (#8)\n\nBrief: dg/03\nBrief: dg/05\n"))
		applied, stderr := reconcileDG(t)
		if len(applied) != 0 {
			t.Fatalf("a multi-linked merge was written: %v", applied)
		}
		if !strings.Contains(stderr, "multi-linked merge is not a derivable delivery edge") {
			t.Errorf("the multi-linked merge must be reported; stderr:\n%s", stderr)
		}
	})
	t.Run("repeated trailer is one brief", func(t *testing.T) {
		rootA := siblingBriefTree(t, "")
		gitCommitTree(t, rootA)
		publishRoots(t, rootA, crossRootRepo, crossRootC)
		withCrossRoot(t, newCrossRootB(t, "feat: one (#8)\n\nBrief: example-stream/02\nBrief: example-stream/02\n"))
		if offered(t, rootA, "example-stream/02") {
			t.Fatal("the same brief named twice is still one link and must hold the row")
		}
	})
}

// A stream name is a flat per-repo namespace. A trailer for a stream that
// ANOTHER configured root also carries cannot say which board it means: it is
// credited to none, and reported.
// regression: #4614
func TestCrossRootAmbiguousStream(t *testing.T) {
	const rootC = crossRootC
	t.Run("planner", func(t *testing.T) {
		rootA := siblingBriefTree(t, "")
		gitCommitTree(t, rootA)
		publishRoots(t, rootA, crossRootRepo, crossRootC)
		rootB := newCrossRootB(t, "feat: land the thing (#7)\n\nBrief: example-stream/02\n")
		c := t.TempDir()
		gitInit(t, c, "Root C Bot", "root-c-bot@example.com")
		carryStream(t, c, "example-stream")
		withCrossRoots(t, map[string]string{crossRootRepo: rootB, rootC: c})
		stderr := captureStderr(t, func() { _ = run(rootA, "lint", nil, nil, "") })
		if !offered(t, rootA, "example-stream/02") {
			t.Fatal("a trailer for a stream two roots carry was credited to this board")
		}
		if !strings.Contains(stderr, "a trailer cannot say which board it means") || !strings.Contains(stderr, rootC) {
			t.Errorf("the ambiguous trailer must be reported naming the other carrier; stderr:\n%s", stderr)
		}
	})
	t.Run("reconcile", func(t *testing.T) {
		rootB := newCrossRootB(t, "feat: implement dg/03 (#7)\n\nBrief: dg/03\n")
		c := t.TempDir()
		gitInit(t, c, "Root C Bot", "root-c-bot@example.com")
		carryStream(t, c, "dg")
		withCrossRoots(t, map[string]string{crossRootRepo: rootB, rootC: c})
		applied, stderr := reconcileDG(t)
		if len(applied) != 0 {
			t.Fatalf("a trailer for a stream two roots carry was written: %v", applied)
		}
		if !strings.Contains(stderr, "a trailer cannot say which board it means") {
			t.Errorf("the ambiguous trailer must be reported; stderr:\n%s", stderr)
		}
	})
	t.Run("a root carrying other streams does not make it ambiguous", func(t *testing.T) {
		rootA := siblingBriefTree(t, "")
		gitCommitTree(t, rootA)
		publishRoots(t, rootA, crossRootRepo, crossRootC)
		rootB := newCrossRootB(t, "feat: land the thing (#7)\n\nBrief: example-stream/02\n")
		c := t.TempDir()
		gitInit(t, c, "Root C Bot", "root-c-bot@example.com")
		carryStream(t, c, "unrelated-stream")
		withCrossRoots(t, map[string]string{crossRootRepo: rootB, rootC: c})
		if offered(t, rootA, "example-stream/02") {
			t.Fatal("an unrelated third root suppressed a real cross-root witness")
		}
	})
}

// A trailer naming a stream the MERGING root itself carries is that root's own
// brief, not this board's.
func TestCrossRootMergerOwns(t *testing.T) {
	rootA := siblingBriefTree(t, "")
	gitCommitTree(t, rootA)
	publishRoots(t, rootA, crossRootRepo, crossRootC)
	rootB := newCrossRootB(t, "feat: land the thing (#7)\n\nBrief: example-stream/02\n")
	carryStream(t, rootB, "example-stream")
	withCrossRoot(t, rootB)
	stderr := captureStderr(t, func() { _ = run(rootA, "lint", nil, nil, "") })
	if !offered(t, rootA, "example-stream/02") {
		t.Fatal("a trailer for a stream the merging root carries was credited to this board")
	}
	if !strings.Contains(stderr, "a trailer cannot say which board it means") {
		t.Errorf("the ambiguous trailer must be reported; stderr:\n%s", stderr)
	}
}

// The board's own root is never read as "another root", by repo or by path.
func TestCrossRootOwnRootIsSkipped(t *testing.T) {
	boardWithMerge := func(t *testing.T) string {
		rootA := siblingBriefTree(t, "")
		gitCommitTree(t, rootA)
		publishRoots(t, rootA, crossRootRepo, crossRootC)
		commitEmpty(t, rootA, "feat: land the thing (#7)\n\nBrief: example-stream/02\n")
		runGit(t, rootA, "update-ref", "refs/remotes/origin/main", "HEAD")
		return rootA
	}
	check := func(t *testing.T, rootA string) {
		t.Helper()
		got, notices := crossRootDeliveries(rootA, "example-org/root-a")
		if len(got) != 0 || len(notices) != 0 {
			t.Fatalf("the board's own root was read as another root: %+v %v", got, notices)
		}
	}
	t.Run("by owner/repo", func(t *testing.T) {
		rootA := boardWithMerge(t)
		other := newCrossRootB(t, "feat: land the thing (#7)\n\nBrief: example-stream/02\n")
		withCrossRoots(t, map[string]string{"example-org/root-a": other})
		check(t, rootA)
	})
	t.Run("by path", func(t *testing.T) {
		rootA := boardWithMerge(t)
		withCrossRoots(t, map[string]string{"example-org/alias": rootA})
		check(t, rootA)
	})
}

// A cross-root merge the brief's own `delivery:` list declares partial is work
// in progress, not a witness that the brief is implemented — the same as for a
// merge in the brief's own repo.
// regression: #4614
func TestCrossRootDeclaredPartial(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	rootA := copyDesignGateFixture(t)
	declareDelivery(t, rootA, "03", `{in: "xb#7", covers: partial, note: "skeleton"}`)
	reg := filepath.Join(rootA, "docs", "streams", "graph-repos.yaml")
	if err := os.WriteFile(reg, []byte(partialRegistry+"  xb:  {cell: fx, repo: "+crossRootRepo+"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stubPullsServer(t, http.StatusOK, []map[string]any{})
	withCrossRoot(t, newCrossRootB(t, "feat: skeleton (#7)\n\nBrief: dg/03\n"))

	code, stdout, stderr := runReconcileBoth(t, "--apply", "--root", rootA, "--repo", "o/r", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d:\n%s\n%s", code, stdout, stderr)
	}
	if got := appliedIDs(t, stdout); len(got) != 0 {
		t.Fatalf("a declared-partial cross-root merge was written: %v", got)
	}
	if c := cellOf(t, stdout, "dg/03"); c.Cell != "in-progress" || !strings.Contains(c.Witness, "partial") {
		t.Fatalf("dg/03 want in-progress on a declared partial, got %+v", c)
	}
	if !strings.Contains(stderr, "PR "+crossRootRepo+"#7 names dg/03") {
		t.Errorf("the partial NOTICE must name the PR in its own repo; stderr:\n%s", stderr)
	}
}

// When the brief has a merged PR in its own repo AND one in another root, the
// own repo's is the witness whatever the PR numbers (they are not comparable).
// regression: #4614
func TestCrossRootOwnRepoFirst(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	rootA := copyDesignGateFixture(t)
	publishRoots(t, rootA, crossRootRepo)
	authoringServer(t, []map[string]any{mergedPull(3, "feat/dg-03", "Brief: dg/03\n")},
		map[int][]map[string]any{3: deliveryFiles}, nil)
	withCrossRoot(t, newCrossRootB(t, "feat: also (#700)\n\nBrief: dg/03\n"))

	code, stdout, stderr := runReconcileBoth(t, "--apply", "--root", rootA, "--repo", "example-org/root-a", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d:\n%s\n%s", code, stdout, stderr)
	}
	got := appliedIDs(t, stdout)
	if len(got) != 1 || !strings.HasPrefix(got[0], "dg/03<PR #3 (merged ") {
		t.Fatalf("the brief's own-repo PR must be the witness, got %v", got)
	}
}

func TestClassifyPRsOwnRepoFirst(t *testing.T) {
	prs := []PRRecord{
		{BriefRef: "dg/03", Number: 900, State: prMerged, Repo: "example-org/root-b"},
		{BriefRef: "dg/03", Number: 3, State: prMerged},
		{BriefRef: "dg/03", Number: 800, State: prMerged, Repo: "example-org/root-b"},
	}
	merged, _ := classifyPRs(prs)
	if merged == nil || merged.Repo != "" || merged.Number != 3 {
		t.Fatalf("latest merged = %+v, want the own-repo PR #3", merged)
	}
}

// ---------------------------------------------------------------------------
// Review round 2: an unread root is not an empty one, and a root the tree does
// not publish is never read, named or located.
// ---------------------------------------------------------------------------

// A configured root that cannot be read might carry the stream, so a trailer
// for a stream the board owns is credited to none while one is unread — the
// readable-carrier case's refusal must not depend on whether the carrier could
// be read.
// regression: #4614
func TestCrossRootUnreadableCarrier(t *testing.T) {
	absentC := filepath.Join(t.TempDir(), "absent")
	t.Run("planner", func(t *testing.T) {
		rootA := siblingBriefTree(t, "")
		gitCommitTree(t, rootA)
		publishRoots(t, rootA, crossRootRepo, crossRootC)
		rootB := newCrossRootB(t, "feat: land the thing (#7)\n\nBrief: example-stream/02\n")
		withCrossRoots(t, map[string]string{crossRootRepo: rootB, crossRootC: absentC})
		stderr := captureStderr(t, func() { _ = run(rootA, "lint", nil, nil, "") })
		if !offered(t, rootA, "example-stream/02") {
			t.Fatal("a trailer was credited while a configured root that might carry the stream could not be read")
		}
		if !strings.Contains(stderr, crossRootC+" (unreadable)") || !strings.Contains(stderr, "a trailer cannot say which board it means") {
			t.Errorf("the unread root must be named as the reason; stderr:\n%s", stderr)
		}
	})
	t.Run("reconcile", func(t *testing.T) {
		rootB := newCrossRootB(t, "feat: implement dg/03 (#7)\n\nBrief: dg/03\n")
		withCrossRoots(t, map[string]string{crossRootRepo: rootB, crossRootC: absentC})
		applied, stderr := reconcileDG(t)
		if len(applied) != 0 {
			t.Fatalf("a trailer was written while a root that might carry the stream could not be read: %v", applied)
		}
		if !strings.Contains(stderr, crossRootC+" (unreadable)") {
			t.Errorf("the unread root must be named as the reason; stderr:\n%s", stderr)
		}
	})
	t.Run("a readable root without the stream is still no carrier", func(t *testing.T) {
		rootA := siblingBriefTree(t, "")
		gitCommitTree(t, rootA)
		publishRoots(t, rootA, crossRootRepo, crossRootC)
		rootB := newCrossRootB(t, "feat: land the thing (#7)\n\nBrief: example-stream/02\n")
		c := t.TempDir()
		gitInit(t, c, "Root C Bot", "root-c-bot@example.com")
		withCrossRoots(t, map[string]string{crossRootRepo: rootB, crossRootC: c})
		if offered(t, rootA, "example-stream/02") {
			t.Fatal("a readable root with no docs/streams/<stream> suppressed a real cross-root witness")
		}
	})
}

// A root the board's registry does not publish is never read for a delivery and
// never named or located in any output the run writes: not the STATUS.md hold,
// not reconcile's witness, not a notice. The registry is where a tree says
// which sibling repos it is willing to name.
// regression: #4614
func TestCrossRootUnpublishedRoot(t *testing.T) {
	const slug = crossRootRepo
	leaks := func(t *testing.T, where, text, path string) {
		t.Helper()
		if strings.Contains(text, slug) || strings.Contains(text, filepath.Base(slug)) || (path != "" && strings.Contains(text, path)) {
			t.Errorf("%s names an unpublished root (%s) or its checkout path:\n%s", where, slug, text)
		}
	}
	t.Run("planner and STATUS.md", func(t *testing.T) {
		rootA := siblingBriefTree(t, "") // registry publishes example-sibling only
		gitCommitTree(t, rootA)
		rootB := newCrossRootB(t, "feat: land the thing (#7)\n\nBrief: example-stream/02\n")
		withCrossRoot(t, rootB)
		stderr := captureStderr(t, func() { _ = run(rootA, "lint", nil, nil, "") })
		if !offered(t, rootA, "example-stream/02") {
			t.Fatal("an unpublished root's trailer held the row")
		}
		if !strings.Contains(stderr, "1 configured root(s) are not published") {
			t.Errorf("the unread root must be counted as could-not-check; stderr:\n%s", stderr)
		}
		leaks(t, "stderr", stderr, rootB)
		if code := run(rootA, "write", nil, nil, ""); code != 0 {
			t.Fatalf("write exited %d", code)
		}
		status, err := os.ReadFile(filepath.Join(rootA, "STATUS.md"))
		if err != nil {
			t.Fatal(err)
		}
		leaks(t, "STATUS.md", string(status), rootB)
	})
	t.Run("a tree with no registry publishes nothing", func(t *testing.T) {
		rootA := siblingBriefTree(t, "")
		if err := os.Remove(filepath.Join(rootA, "docs", "streams", "graph-repos.yaml")); err != nil {
			t.Fatal(err)
		}
		gitCommitTree(t, rootA)
		rootB := newCrossRootB(t, "feat: land the thing (#7)\n\nBrief: example-stream/02\n")
		withCrossRoot(t, rootB)
		got, notices := crossRootDeliveries(rootA, "")
		if len(got) != 0 {
			t.Fatalf("a root was read although the tree has no registry: %+v", got)
		}
		leaks(t, "notices", strings.Join(notices, "\n"), rootB)
		if len(notices) != 1 || !strings.Contains(notices[0], "this tree has no registry") {
			t.Errorf("want one aggregate could-not-check naming the missing registry, got %v", notices)
		}
	})
	t.Run("reconcile witness and JSON", func(t *testing.T) {
		rootB := newCrossRootB(t, "feat: implement dg/03 (#7)\n\nBrief: dg/03\n")
		withCrossRoot(t, rootB)
		applied, stdout, stderr := reconcileDGPublishing(t) // registry publishes nothing
		if len(applied) != 0 {
			t.Fatalf("an unpublished root's trailer was written: %v", applied)
		}
		leaks(t, "reconcile stdout", stdout, rootB)
		leaks(t, "reconcile stderr", stderr, rootB)
	})
	t.Run("an unpublished carrier makes the stream ambiguous without being named", func(t *testing.T) {
		rootA := siblingBriefTree(t, "")
		gitCommitTree(t, rootA)
		publishRoots(t, rootA, slug) // B is published; C is not
		rootB := newCrossRootB(t, "feat: land the thing (#7)\n\nBrief: example-stream/02\n")
		c := t.TempDir()
		gitInit(t, c, "Root C Bot", "root-c-bot@example.com")
		carryStream(t, c, "example-stream")
		withCrossRoots(t, map[string]string{slug: rootB, crossRootC: c})
		stderr := captureStderr(t, func() { _ = run(rootA, "lint", nil, nil, "") })
		if !offered(t, rootA, "example-stream/02") {
			t.Fatal("a trailer for a stream an unpublished root carries was credited to this board")
		}
		if !strings.Contains(stderr, "1 other configured root(s)") {
			t.Errorf("the unpublished carrier must be counted; stderr:\n%s", stderr)
		}
		if strings.Contains(stderr, crossRootC) || strings.Contains(stderr, c) {
			t.Errorf("the unpublished carrier was named or located:\n%s", stderr)
		}
	})
	t.Run("a published unreadable root's notice carries no path", func(t *testing.T) {
		rootA := siblingBriefTree(t, "")
		gitCommitTree(t, rootA)
		publishRoots(t, rootA, slug)
		absent := filepath.Join(t.TempDir(), "absent")
		withCrossRoot(t, absent)
		_, notices := crossRootDeliveries(rootA, "")
		if len(notices) != 1 || !strings.Contains(notices[0], slug) {
			t.Fatalf("want one could-not-check naming the published root, got %v", notices)
		}
		if strings.Contains(notices[0], absent) || strings.Contains(notices[0], filepath.Dir(absent)) {
			t.Errorf("the notice carries the checkout path:\n%s", notices[0])
		}
	})
}
