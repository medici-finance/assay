package gitcore

// write.go — the local-repository write primitives added for the brief 07
// migration (deskmerge's non-merge write verbs): an explicit-parent Commit, a
// parent-hash reader for the defense-in-depth check that consumes it, and local ref
// deletion.
//
// DELIBERATELY NOT ADDED HERE: an `Add` wrapper over go-git's `Worktree.Add`. Verified
// empirically (brief 07): `Worktree.Add` does not clear a path's
// merge-conflict index entries — after `git merge --no-ff --no-commit` leaves a path at
// stages 1/2/3 and the working-tree file is rewritten (deskmerge's regenerable-conflict
// resolution), `Worktree.Add` leaves all three stages in the on-disk index untouched
// (`git ls-files -u` still lists them afterward) and a subsequent Commit — which builds
// its tree from that same index — writes a TREE WITH DUPLICATE ENTRIES for the path
// (`git fsck` reports `duplicateEntries`). go-git's index/tree-building path has no
// concept of conflict stages; it is a genuine gap in the same family as the trial
// merge's own (no three-way merge, no conflict-stage awareness), not a caller error.
// The fix that IS safe — proven by the same experiment — is staging the resolved path
// with the git BINARY first (which clears the stages correctly on disk) and only then
// building the commit here from the now-clean index; Commit does not care how the index
// reached stage 0. So deskmerge's regenerable-conflict `add` stays fenced through
// internal/gitexec alongside the trial merge and its conflict enumeration — see that
// package's allowlist doc — and this package exposes no `Add` for it to almost-fit.
//
// A generic Add IS still safe for a caller with no conflict-stage index — no such
// caller exists in this stream yet (deskmerge is the first tool to migrate a write
// verb), so none is added ahead of that need; see this file's own doc for the one shape
// that is NOT safe.

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// CommitOpts configures an in-process Commit. Parents is EXPLICIT — the written
// commit's parent list is exactly what the caller passes (each resolved fresh via
// Resolve), never inferred from HEAD or a MERGE_HEAD file the way `git commit` infers a
// merge commit's parents. That makes a caller's parent list a CONSTRUCTION property of
// the commit rather than something a separate read-back has to verify after the fact.
//
// Author, left nil, falls back to the repository's own configured identity (go-git's
// own git-commit-compatible resolution: local config, then global, then system) —
// matching `git commit`'s behaviour when invoked with no explicit identity flags, which
// is how every migrated caller invoked it before this package existed. Set it
// explicitly only when a caller has its own identity to assert.
type CommitOpts struct {
	Message string
	Parents []string
	Author  *object.Signature
}

// Commit writes a commit object from the CURRENT INDEX — as left by a prior git-binary
// `add` (see this file's doc for why Add itself is not offered here), or by any process
// that wrote the same on-disk index — with the given explicit parents, matching
// `git commit --no-verify -m <msg>` immediately followed by `rev-parse HEAD`: no hooks
// ever run (go-git spawns none, so --no-verify's effect is inherent) and the new
// commit's hash is returned directly, with no separate read-back required.
func (r *Repo) Commit(opts CommitOpts) (string, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("gitcore: commit: %w", err)
	}
	parents := make([]plumbing.Hash, 0, len(opts.Parents))
	for _, p := range opts.Parents {
		h, err := r.Resolve(p)
		if err != nil {
			return "", fmt.Errorf("gitcore: commit: parent %q: %w", p, err)
		}
		parents = append(parents, h)
	}
	h, err := wt.Commit(opts.Message, &git.CommitOptions{
		Author:  opts.Author,
		Parents: parents,
	})
	if err != nil {
		return "", fmt.Errorf("gitcore: commit: %w", err)
	}
	return h.String(), nil
}

// CommitParents returns the parent hashes of the commit rev resolves to, in order,
// matching the parent fields of `git rev-list --parents -n 1 <rev>` (that command's own
// first field — the commit itself — is rev's own resolved hash, which the caller
// already has).
func (r *Repo) CommitParents(rev string) ([]string, error) {
	hash, err := r.Resolve(rev)
	if err != nil {
		return nil, err
	}
	commit, err := r.repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("gitcore: commit-parents %s: %w", rev, err)
	}
	out := make([]string, len(commit.ParentHashes))
	for i, p := range commit.ParentHashes {
		out[i] = p.String()
	}
	return out, nil
}

// DeleteLocalRef removes a local reference, matching `git update-ref -d <name>`.
// Deleting an already-absent reference is a no-op success, matching real git's own
// `update-ref -d` on a ref that does not exist.
func (r *Repo) DeleteLocalRef(name string) error {
	err := r.repo.Storer.RemoveReference(plumbing.ReferenceName(name))
	if err != nil {
		return fmt.Errorf("gitcore: delete-ref %s: %w", name, err)
	}
	return nil
}
