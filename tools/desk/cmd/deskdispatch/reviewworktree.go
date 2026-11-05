package main

import (
	"crypto/rand"
	"encoding/hex"
	"io"
)

// Only the local directory changes between review passes. The canonical claim
// still serializes the lane; fresh scratch space is never a second claim key.
var reviewEntropy io.Reader = rand.Reader

func freshReviewName(base string) (string, error) {
	var nonce [8]byte
	if _, err := io.ReadFull(reviewEntropy, nonce[:]); err != nil {
		return "", err
	}
	const maxPrefix = worktreeNameMax - len("review-") - 1 - 16
	if len(base) > maxPrefix {
		base = base[:maxPrefix]
	}
	return "review-" + base + "-" + hex.EncodeToString(nonce[:]), nil
}

// Every worktree allocation crosses this seam. Review lanes always allocate
// detached; the plan cannot accidentally route a reviewer through a branch arm.
func createDispatchWorktree(o dispatchOpts, plan dispatchPlan) runResult {
	base, err := dispatchBase(o, plan)
	if err != nil {
		return runResult{err: err}
	}
	args := []string{"add", plan.wtName}
	if plan.detached || reviewKit(o.kit) {
		args = append(args, "--detach")
	} else {
		args = append(args, "--branch", plan.branch)
	}
	if workerResume(o) {
		// The claim for plan.claimKey is held by now, so a home stamped with that run key
		// is a FINISHED one; deskwt adopts it only on its own clean/pushed/at-head proof (#4272).
		args = append(args, "--upstream", "refs/remotes/origin/"+plan.branch, "--adopt-run", plan.claimKey)
	}
	args = append(args, "--base", base, "--role", plan.identityRole)
	return runCmd(o.root, "deskwt", args...)
}
