package main

import (
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// publishProof binds a checked range to its destination, branch and source. The
// push rechecks that binding and requires the remote ref to retain the read tip.
// No local tracking ref or caller-supplied SHA can create this proof.
type publishProof struct {
	destination, url, ref, head, tip string
}

// prepareCreatePush establishes the range independently of PR existence: an
// Evidence branch may already be published before its first PR is opened.
func prepareCreatePush(s pushSpec, base string) (pushSpec, error) {
	dest, endpoint, err := pushEndpoint(s)
	if err != nil {
		return s, err
	}
	refs, err := gitcore.List(endpoint)
	if err != nil && !errors.Is(err, transport.ErrEmptyRemoteRepository) {
		return s, deskkit.Unverifiable("cannot confirm the destination branch for publish-identity", err)
	}
	ref := "refs/heads/" + s.dstBranch
	tip := ""
	for _, r := range refs {
		if r.Name().String() != ref {
			continue
		}
		if r.Type() != plumbing.HashReference || r.Hash().IsZero() || tip != "" {
			return s, deskkit.Refused("refused: destination returned an invalid or ambiguous branch head")
		}
		tip = r.Hash().String()
	}
	repo, err := gitcore.Open(s.dir)
	if err != nil {
		return s, deskkit.Unverifiable("cannot open the repository for publish-identity", err)
	}
	head, err := repo.Resolve(s.srcRef)
	if err != nil {
		return s, deskkit.Unverifiable("cannot resolve the source for publish-identity", err)
	}
	current, err := repo.Resolve("HEAD")
	if err != nil || current != head {
		return s, deskkit.Refused("refused: push source differs from the worktree HEAD; re-run from the source branch")
	}
	// Existing branches must be fast-forwardable. An unreadable or divergent tip
	// is not evidence for excluding commits, even if the wide range would pass.
	if tip != "" {
		ancestor, err := repo.IsAncestor(tip, head.String())
		if err != nil || !ancestor {
			return s, deskkit.Refused(fmt.Sprintf("refused: cannot prove HEAD descends from destination %s at %s; fetch and merge before retrying", ref, shortSHA(tip)))
		}
	}
	if err := publishIdentityGate(s.dir, base, tip, "", head.String()); err != nil {
		return s, err
	}
	s.proof = &publishProof{destination: dest, url: endpoint.URL, ref: ref, head: head.String(), tip: tip}
	return s, nil
}
