package main

import (
	"bytes"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/gitcore"
)

var landedClosureFn = landedClosureTree

// Both immutable per-file outcomes and legacy appended sidecars enter here.
// Ordinary Evidence and non-verified outcomes keep their existing checks/root.
func landedClosureTree(root, target string, before, after []byte, exists bool, fg deskkit.Forge, repo deskkit.ForgeRepo, branch string) (string, string, func(), error) {
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
	tmp, err := os.MkdirTemp("", "deskevidence-landed-")
	if err != nil {
		return "", "", noop, deskkit.Unverifiable("cannot create landed closure checkout", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }
	if err := gitcore.CheckoutLocalCommit(root, tmp, tip.SHA); err != nil {
		cleanup()
		return "", "", noop, deskkit.Unverifiable("cannot read exact landed closure history", err)
	}
	return tmp, tip.SHA, cleanup, nil
}
