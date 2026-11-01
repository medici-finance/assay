package main

import (
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore"
)

// regression: #2395
func TestFlipHistoryParity(t *testing.T) {
	t.Run("selected-commit-set", flipHistoryOracle)
	t.Run("every-criss-cross-base", TestVflipEveryMergeBase)
	t.Run("criss-cross-no-net", TestVflipCrissCrossNoNet)
	t.Run("criss-cross-drops-fail", TestVflipCrissCrossDropsFail)
	t.Run("octopus-third-parent", TestVflipOctopusDropsFail)
	t.Run("merge-drops-fail", TestVflipMergeDropsFail)
	t.Run("rename", TestVflipRenamedBrief)
	t.Run("rename-and-readd", TestVflipMovedAndBack)
	t.Run("replace-isolation", TestVflipReplaceObject)
	t.Run("uncommitted-pass", TestVflipUncommittedPass)
	t.Run("uncommitted-delete", TestVflipUncommittedDelete)
	t.Run("shallow-unknown", TestVflipShallowCannotCheck)
}

func flipHistoryOracle(t *testing.T) {
	root, _ := vfFixture(t, vfDefaults())
	since := vfRev(t, root, "HEAD")
	runGit(t, root, "checkout", "-qb", "side", since)
	vfAppend(t, root, "\n\n"+vfLaterFail)
	vfCommit(t, root, vfMailVerifier)
	side := vfRev(t, root, "HEAD")
	runGit(t, root, "checkout", "-q", "main")
	vfOther(t, root, "unrelated.txt", vfMailWorker)
	vfGit(t, root, vfMailWorker, "merge", "-q", "-s", "ours", "--no-edit", "side")
	oracle := func() []string {
		t.Helper()
		cmd := exec.Command("git", "--no-replace-objects", "-c", "diff.algorithm=myers", "-C", root, "rev-list", "--full-history", since+"..HEAD", "--", vfBriefRel)
		cmd.Env = append(historyCleanEnv(os.Environ()), "GIT_GRAFT_FILE="+os.DevNull)
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.Fields(string(out))
	}
	check := func() {
		t.Helper()
		want := oracle()
		got, err := flipHistoryWalk(root, vfBriefRel, since)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("full-history selected set %v (%v), native %v", got, err, want)
		}
	}
	check()
	want := oracle()
	found := false
	for _, id := range want {
		found = found || id == side
	}
	if !found {
		t.Fatal("fixture did not expose hidden side FAIL")
	}
	simple := historyGit(root, "rev-list", since+"..HEAD", "--", vfBriefRel)
	out, err := simple.Output()
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(strings.Fields(string(out)), want) {
		t.Fatal("fixture did not discriminate default history simplification")
	}
	r, err := gitcore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	var candidate []string
	if err := r.PathHistory("HEAD", vfBriefRel, func(c gitcore.PathCommit) bool { candidate = append(candidate, c.Hash); return true }); err != nil {
		t.Fatal(err)
	}
	sorted := append([]string(nil), want...)
	sort.Strings(sorted)
	sort.Strings(candidate)
	if reflect.DeepEqual(candidate, sorted) {
		t.Fatal("retained PathHistory semantic gap not demonstrated")
	}
	// A subsequent actual two-parent merge retains the first parent's deletion
	// while its new side parent still holds the path.
	runGit(t, root, "checkout", "-q", "side")
	vfAppend(t, root, "\n\nverifier side note")
	vfCommit(t, root, vfMailVerifier)
	newSide := vfRev(t, root, "HEAD")
	runGit(t, root, "checkout", "-q", "main")
	runGit(t, root, "rm", vfBriefRel)
	vfCommit(t, root, vfMailWorker)
	vfGit(t, root, vfMailWorker, "merge", "-q", "-s", "ours", "--no-edit", newSide)
	parents := strings.Fields(string(parityGitBytes(t, root, "show", "-s", "--format=%P", "HEAD")))
	if len(parents) != 2 {
		t.Fatalf("drop-path fixture is not a merge: %v", parents)
	}
	if _, err := historyGit(root, "cat-file", "-e", "HEAD:"+vfBriefRel).Output(); err == nil {
		t.Fatal("drop-path merge retained the path")
	}
	parityGitBytes(t, root, "show", "HEAD^2:"+vfBriefRel)
	check()
	if err := flipJudgeHistory(evidenceActorPolicyFromRoster(), root, vfBriefRel, []string{since}); err == nil || !strings.Contains(err.Error(), "not at this path") {
		t.Fatalf("deleted path did not retain unknown provenance: %v", err)
	}
}
