package main

import (
	"bytes"
	"os"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var (
	landedClosureFn  = landedClosureTree
	landedBaselineFn = landedBaseline
	// landedFetchFn is the network seam: tests serve the same refspecs from a
	// local repository through the same in-process gitcore fetch.
	landedFetchFn = fetchLandedRefs
)

// landedFetchRefs is private to the read root. A cross-repository read root is
// the verifier's own disposable tracking clone, so these refs move nothing any
// other worktree reads (its origin/main is left exactly as admitted).
const landedFetchRefs = "refs/deskevidence-fetch/"

// Both immutable per-file outcomes and legacy appended sidecars enter here.
// Ordinary Evidence and non-verified outcomes keep their existing checks/root.
func landedClosureTree(root, target string, before, after []byte, exists bool, fg deskkit.Forge, repo deskkit.ForgeRepo, branch string, receipt deskkit.VerifierReceipt) (string, string, func(), error) {
	if root == "" {
		root = "."
	}
	noop := func() {}
	if !isVerifyOutcomesSidecar(target) || len(verifiedBriefsAdded(before, after, exists)) == 0 {
		return root, "", noop, nil
	}
	tip, err := fg.GetCommit(repo, branch)
	if err != nil || tip == nil || len(tip.SHA) != 40 {
		return "", "", noop, deskkit.Unverifiable("cannot resolve exact landed closure commit", err)
	}
	// The earlier destination read may have used a publication snapshot. It must
	// agree with this pinned tree before any of its bytes become a write base.
	current, err := fg.ReadFile(repo, deskkit.ReadFileInput{File: target, Ref: tip.SHA})
	if err != nil && !(deskkit.IsForgeNotFound(err) && !exists) {
		return "", "", noop, err
	}
	if err == nil {
		if err := plainFileRead(current, repo, target, tip.SHA, "the verified outcome destination", false); err != nil {
			return "", "", noop, err
		}
	}
	if (current != nil && current.Exists) != exists || (exists && !bytes.Equal(current.Content, before)) {
		return "", "", noop, deskkit.Refused("verified outcome destination moved while resolving landed closure; reconcile before retrying")
	}
	baseline, err := landedBaselineFn(root, receipt, repo, branch)
	if err != nil {
		return "", "", noop, err
	}
	tmp, err := os.MkdirTemp("", "deskevidence-landed-")
	if err != nil {
		return "", "", noop, deskkit.Unverifiable("cannot create landed closure checkout", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }
	if err := gitcore.CheckoutLocalCommit(root, tmp, tip.SHA, baseline); err != nil {
		cleanup()
		return "", "", noop, deskkit.Unverifiable("cannot read exact landed closure history", err)
	}
	return tmp, tip.SHA, cleanup, nil
}

// landedBaseline names the read root's ref for the landed repository's main.
// A same-repository root already tracks it as origin/main. A cross-repository
// root is a clone of a tracking checkout: its origin/main is whatever that
// checkout's local main held, possibly another repository's history, and it
// lacks the commits landed since. Fetch the target repository's own main and
// the landing branch into private refs instead (#5008).
func landedBaseline(root string, receipt deskkit.VerifierReceipt, repo deskkit.ForgeRepo, branch string) (string, error) {
	if receipt.Binding.Version != 2 {
		return "refs/remotes/origin/main", nil
	}
	if plumbing.NewBranchReferenceName(branch).Validate() != nil {
		return "", deskkit.Refused("refused: landing branch is not a valid branch name: " + branch)
	}
	baseline := landedFetchRefs + "main"
	specs := []string{"+refs/heads/main:" + baseline}
	if branch != "main" {
		specs = append(specs, "+refs/heads/"+branch+":"+landedFetchRefs+"branch/"+branch)
	}
	if err := landedFetchFn(root, repo.Slug(), specs); err != nil {
		return "", deskkit.Unverifiable("cannot fetch the landed repository's main for the landed closure", err)
	}
	return baseline, nil
}

// fetchLandedRefs fetches through the root's own configured origin, bound to
// the session's token role, the same read path a resume refresh uses.
func fetchLandedRefs(root, repo string, specs []string) error {
	role, _, err := deskkit.SessionTokenRole("deskevidence landed fetch")
	if err != nil {
		return err
	}
	endpoint, err := deskkit.CheckoutGitRead(root, repo, role)
	if err != nil {
		return err
	}
	r, err := gitcore.Open(root)
	if err != nil {
		return err
	}
	return r.Fetch(gitcore.FetchOpts{URL: endpoint.URL, Auth: endpoint.Auth, RefSpecs: specs})
}
