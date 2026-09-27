package main

// branchname.go — the ONE place a worker's default branch name is derived from an item key.
//
// THE DEFECT IT CLOSES. The default branch used to be `feat/<item-key>` everywhere, and a worker
// names its changelog fragment after its branch. An item key can carry ANOTHER repo's short
// label as its first `--` segment — an issue key of the `<alias>--issue-<N>` shape filed on a
// private tracking repo, dispatched with a public repo as its --repo. That label and the private
// issue number then landed in the public repo's branch list and, through the fragment filename,
// in its committed tree. A branch name and a tree path are published content; the claim key is a
// ref in the claim store and stays byte-for-byte the item key.
//
// THE RULE. When the resolved target repo's roster entry says `:public` AND the item key's repo
// label (its first `--` segment) is not that repo's own label, the default branch is NEUTRAL:
//
//   - `feat/<stream>-<NN>` when the key is a brief claim key (`<label>--<stream>--<NN>`, NN digits);
//   - `feat/item-<hash>` otherwise, <hash> the first 10 hex digits of sha256(item key).
//
// Anything else — a private or unstated-visibility target, a key with no `--` label (a plan key
// `<stream>/<NN>`, a bare `item-1`), a key whose label IS the target's own — keeps the historical
// `feat/<item-key>` derivation unchanged. An explicit --branch always wins over this function.
//
// The hash is a LABEL, not a secret: it keeps the foreign label out of public names, and it is
// deterministic so every dispatcher derives the same branch for the same item. It does not make
// the key unrecoverable to someone who already knows the label and can enumerate issue numbers.
//
// Every site that needs "the branch this item would get by default" calls defaultBranch; the
// structural test in branchname_test.go fails on any other non-test string literal that starts a
// `feat/` branch name, so a second derivation cannot drift back to the raw key.

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// defaultBranch returns the branch a worker dispatch of item into repo gets when no --branch is
// given. See the file comment for the rule.
func defaultBranch(item, repo string) string {
	if !neutralBranchNeeded(item, repo) {
		return "feat/" + sanitizeSegment(item)
	}
	parts := strings.Split(strings.Trim(item, "/"), "--")
	if len(parts) >= 3 {
		stream, nn := parts[len(parts)-2], parts[len(parts)-1]
		if stream != "" && isDigits(nn) && !strings.Contains(stream, "/") {
			return "feat/" + stream + "-" + nn
		}
	}
	sum := sha256.Sum256([]byte(item))
	return "feat/item-" + hex.EncodeToString(sum[:])[:10]
}

// neutralBranchNeeded reports whether item's default branch must not carry its key into repo:
// repo is configured `:public` and the key's repo label names some OTHER repo.
func neutralBranchNeeded(item, repo string) bool {
	if deskkit.RepoVisibility(repo) != deskkit.VisibilityPublic {
		return false
	}
	label := itemRepoLabel(item)
	if label == "" {
		return false
	}
	return label != deskkit.RepoShortLabel(repo) && label != shortRepo(repo)
}

// itemRepoLabel is the repo label an item key carries: its first `--` segment, or "" for a key
// with no `--` (a plan key or a bare item name carries no repo label).
func itemRepoLabel(item string) string {
	s := strings.Trim(item, "/")
	i := strings.Index(s, "--")
	if i <= 0 {
		return ""
	}
	return s[:i]
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
