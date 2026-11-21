package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Exercise the same add that deskdispatch calls. Keep a staged change and merge
// state throughout every refusal and through replay of an interrupted re-key.
// regression: #7345
func TestBriefCheckoutHandoff(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	t.Setenv("DESK_SESSION", "handoff-session")
	repo := "example-org/tracker"
	prefix := deskkit.RepoShortLabel(repo)
	prev, next := prefix+"--stream--02", prefix+"--pr-17"
	ref := prev + "@20261029T000000Z.0123456789ab"
	head := mustGit(t, work, "rev-parse", "HEAD")
	up := "refs/remotes/origin/feature"
	mustGit(t, work, "update-ref", up, head)
	add := []string{"add", "source", "--branch", "feature", "--base", head, "--upstream", up}
	if rc, stderr := runCapErr(t, add); rc != 0 {
		t.Fatal(stderr)
	}
	home := resolvePath(filepath.Join(tmpBaseDir, "tracker-source"))
	mustGit(t, home, "config", "extensions.worktreeConfig", "true")
	for k, v := range map[string]string{"assay.runKey": prev, "assay.dispatchRef": ref, "assay.claimRepo": repo, "assay.claimStore": deskkit.ClaimStoreForgeRef} {
		mustGit(t, home, "config", "--worktree", k, v)
	}
	kit, tier, stamp, branch := "worker", "strong", "pending", "feature"
	record := deskkit.DispatchRecord{Event: deskkit.DispatchEventDispatched, ClaimKey: prev, DispatchRef: &ref, Repo: repo,
		Item: &prev, Branch: &branch, Kit: &kit, Tier: &tier, ModelStamp: &stamp}
	if err := deskkit.AppendDispatchRecord(&record); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(home, "README.md"), "staged\n")
	mustGit(t, home, "add", "README.md")
	merge := mustGit(t, home, "rev-parse", "--path-format=absolute", "--git-path", "MERGE_HEAD")
	writeFile(t, merge, head+"\n")
	wantIndex := mustGit(t, home, "write-tree")
	change := &deskkit.PullRequest{Number: 17, State: "open", CrossRepo: deskkit.CrossRepoSame, HeadRef: branch, HeadSHA: head, Body: "Brief: stream/02"}
	keys := map[string]bool{next: true}
	owned := true
	oldRead, oldClaims, oldOwner := handoffChange, listDispatchClaims, handoffOwnedClaim
	t.Cleanup(func() { handoffChange, listDispatchClaims, handoffOwnedClaim = oldRead, oldClaims, oldOwner })
	handoffChange = func(string, int) (*deskkit.PullRequest, error) { return change, nil }
	listDispatchClaims = func(string) (map[string]bool, error) { return keys, nil }
	handoffOwnedClaim = func(string, string, string, string) (bool, error) { return owned, nil }
	args := append([]string{"add", "destination", "--branch", branch, "--base", head, "--upstream", up}, "--role", "worker", "--adopt-run", next, "--handoff-brief")
	check := func(why string) {
		t.Helper()
		rc, out, stderr := runCapBoth(t, args)
		if why == "" {
			if rc != 0 || strings.TrimSpace(out) != home+"\n"+deskkit.BriefHandoffReceipt {
				t.Fatalf("rc %d out %q: %s", rc, out, stderr)
			}
		} else if rc == 0 || !strings.Contains(stderr, why) {
			t.Fatalf("wanted refusal %q, got %d %s", why, rc, stderr)
		}
		b, err := os.ReadFile(merge)
		if err != nil || string(b) != head+"\n" || mustGit(t, home, "write-tree") != wantIndex {
			t.Fatal("handoff changed the index or merge in progress")
		}
	}
	check("")
	change.Body = "Brief: other/02"
	check("does not deliver")
	change.Body = "Brief: stream/02"
	keys[prev] = true
	check("brief claim must be released")
	delete(keys, prev)
	owned = false
	check("claim does not belong")
	owned = true
	t.Setenv("DESK_SESSION", "other")
	check("does not match this session")
	t.Setenv("DESK_SESSION", "handoff-session")
	mustGit(t, work, "worktree", "lock", "--reason", "session=other", home)
	check("another owner's lock")
	mustGit(t, work, "worktree", "unlock", home)
	change.HeadSHA = strings.Repeat("0", 40)
	check("no longer matches")
	change.HeadSHA = head
	check("")
	mustGit(t, home, "config", "--worktree", "assay.runKey", next)
	check("")
}
