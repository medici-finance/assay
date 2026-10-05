package main

import (
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"regexp"
)

var pinnedHeadRe = regexp.MustCompile("^[0-9a-f]{40}$|^[0-9a-f]{64}$")

// The upstream is a tracking coordinate, never the checkout start point. Check
// it before branch reclamation or allocation, then keep the base SHA immutable
// even if another process advances the remote-tracking ref during creation.
func checkPinnedUpstream(dir, head, upstream string) error {
	tip, err := runGit(dir, "rev-parse", "--verify", "--quiet", upstream+"^{commit}")
	if err != nil || tip != head {
		return deskkit.Unverifiable("refused: --upstream does not resolve to the pinned --base commit", err)
	}
	return nil
}
