package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// github.go — the READ half. deskmerge makes no mutating GitHub call on any path; the
// only write it can perform is a `git push` of the PR's own branch.

// prInfo is the subset of a PR's state deskmerge's decisions depend on.
type prInfo struct {
	Number            int    `json:"number"`
	State             string `json:"state"`
	IsDraft           bool   `json:"isDraft"`
	HeadRefName       string `json:"headRefName"`
	HeadRefOid        string `json:"headRefOid"`
	BaseRefName       string `json:"baseRefName"`
	IsCrossRepository bool   `json:"isCrossRepository"`
}

// prFields is the exact field set fetched. Named once so the query and the struct
// cannot drift apart.
var prFields = []string{
	"number", "state", "isDraft", "headRefName", "headRefOid", "baseRefName",
	"isCrossRepository",
}

// fetchPR reads the PR's state.
//
// A failed or unparseable read is Unverifiable (6) — could-not-check — never an assumed
// open draft. deskkit.ParsePRKind exists because "not stated" reading as "open" is the
// #247 shape, and a merge tool that guesses "open" would push to the
// branch of a PR that has already landed.
func fetchPR(repo string, number int) (prInfo, error) {
	raw, err := runGH("pr", "view", fmt.Sprint(number), "-R", repo,
		"--json", strings.Join(prFields, ","))
	if err != nil {
		return prInfo{}, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot read %s#%d's state — deskmerge will not act on a PR whose "+
				"open/merged/draft status it could not establish", deskkit.StripControl(repo), number), err)
	}
	var p prInfo
	if jerr := json.Unmarshal([]byte(raw), &p); jerr != nil {
		return prInfo{}, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d's state did not parse as JSON",
			deskkit.StripControl(repo), number), jerr)
	}
	if p.HeadRefOid == "" {
		return prInfo{}, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d reported no head oid — every currency verdict is relative to a "+
				"head, and an unknown head makes every one of them unverifiable",
			deskkit.StripControl(repo), number), nil)
	}
	return p, nil
}

// eligibleForMerge applies the state preconditions R-5 binds `deskmerge merge` to. It
// is separate from the read so `check` can report on any PR while `merge` refuses most
// of them.
//
// Each refusal is a POSITIVE determination, hence exit 5 rather than 6.
func eligibleForMerge(repo string, p prInfo) error {
	switch kind := deskkit.ParsePRKind(p.State); kind {
	case deskkit.PROpen:
		// the only eligible state
	case deskkit.PRUnknown:
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d reported state %q, which is neither OPEN, MERGED nor CLOSED — "+
				"an unrecognised state is not an open PR", deskkit.StripControl(repo), p.Number,
			deskkit.StripControl(p.State)), nil)
	default:
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d is %s — deskmerge never pushes to the branch of a PR that is no longer "+
				"open (the deskpushguard shape)", deskkit.StripControl(repo), p.Number, kind))
	}

	// R-5's flip bound, in code. The ready-flip criterion is "reviewer App APPROVED at
	// the current head". A desk merge REPLACES the head. So a desk merge on an
	// already-flipped PR would let the desk supply the very commit its own flip check
	// is evaluated against — the tool would be manufacturing its own gate's input.
	if !p.IsDraft {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d is already flipped ready-for-review — R-5's flip bound forbids a desk "+
				"merge here, because the merge would replace the head that the desk's own "+
				"approval-at-head check is evaluated against. Merge-currency belongs BEFORE the "+
				"review, not after the flip (tracker#1544).",
			deskkit.StripControl(repo), p.Number))
	}

	// A cross-repository PR's head branch lives in a fork the desk has no push
	// authority over. Refusing here is honest; attempting the push and reporting the
	// remote's rejection would burn a charged write to learn something already known.
	if p.IsCrossRepository {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d's head branch lives in a fork — deskmerge pushes only to the PR's own "+
				"branch in the base repository", deskkit.StripControl(repo), p.Number))
	}
	return nil
}
