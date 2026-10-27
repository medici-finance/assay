package main

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
// owns none of root A's streams.
func newCrossRootB(t *testing.T, msg string) string {
	t.Helper()
	dir := t.TempDir()
	gitInit(t, dir, "Root B Bot", "root-b-bot@example.com")
	commitEmpty(t, dir, "chore: init")
	commitEmpty(t, dir, msg)
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
func TestCrossRootUnreadableRoot(t *testing.T) {
	rootA := siblingBriefTree(t, "")
	gitCommitTree(t, rootA)
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
