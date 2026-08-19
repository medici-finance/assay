package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// github.go — every remote READ deskclose performs, plus the only two WRITES it can
// emit (`issue comment` / `issue close`, and their `pr` equivalents). There is no
// merge verb, no reopen verb and no edit verb anywhere in this package.

// decisionLabels are the labels that put an item on a human's decision queue.
//
// Their presence is an ABSOLUTE refusal, in every mode including a manifest row, and
// it is checked before the lane's own preconditions. A decision item leaves the queue
// through the decision digest, where the human sees it — never through a dedupe or
// supersession sweep, which is a batch nobody reads item by item.
//
// `human-decided` is refused as firmly as `needs-decision`. It is tempting to read it
// as "already handled, safe to close", and that reading is exactly wrong: it marks an
// item whose ruling has been given and whose TRACKER condition (R-1 lane A: the close
// comment must NAME the brief/PR/issue carrying the remaining work) this tool cannot
// discharge. Lane A closes are not in deskclose's mode set.
var decisionLabels = []string{"needs-decision", "human-decided"}

// item is the subset of a GitHub issue/PR deskclose reads.
type item struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	State       string `json:"state"`
	StateReason string `json:"state_reason"`
	Body        string `json:"body"`
	Labels      []struct {
		Name string `json:"name"`
	} `json:"labels"`
	PullRequest *struct {
		MergedAt *string `json:"merged_at"`
	} `json:"pull_request"`
}

func (i item) isPR() bool   { return i.PullRequest != nil }
func (i item) closed() bool { return strings.EqualFold(i.State, "closed") }

func (i item) labelNames() []string {
	out := make([]string, 0, len(i.Labels))
	for _, l := range i.Labels {
		out = append(out, l.Name)
	}
	return out
}

// fetchItem reads one issue or PR. The issues endpoint serves both, and carries the
// `pull_request` key exactly when the number is a PR.
//
// A read failure is Unverifiable (exit 6): deskclose cannot know whether the item
// carries a decision label, so it must not close it. There is no "assume no labels"
// arm — that is the unread-precondition failure this tool exists to make impossible.
func fetchItem(repo string, n int) (item, error) {
	raw, err := runGH("api", "-H", "Accept: application/vnd.github+json",
		fmt.Sprintf("repos/%s/issues/%d", repo, n))
	if err != nil {
		return item{}, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot read %s#%d — its labels and state are unknown, so it is not "+
				"closeable (an unread precondition is never a satisfied one)", repo, n), err)
	}
	var it item
	if jerr := json.Unmarshal([]byte(raw), &it); jerr != nil {
		return item{}, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d did not parse as a GitHub item", repo, n), jerr)
	}
	return it, nil
}

// refuseDecisionItem is the label gate. Called for every close in every mode.
func refuseDecisionItem(repo string, it item) error {
	for _, want := range decisionLabels {
		for _, have := range it.labelNames() {
			if strings.EqualFold(strings.TrimSpace(have), want) {
				return deskkit.Refused(fmt.Sprintf(
					"refused: %s#%d carries the %q label. Decision items exit through the decision digest "+
						"where a human reads them one at a time — never through a dedupe or "+
						"supersession sweep. This refusal has no override in any mode, including a manifest row.",
					repo, it.Number, want))
			}
		}
	}
	return nil
}

// pullRequest is the subset of the PULLS endpoint that settles merged-vs-closed.
type pullRequest struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	Merged bool   `json:"merged"`
}

// requireMergedPR is the merged-vs-closed gate for lanes 2 and 3.
//
// It reads `merged` from the PULLS endpoint, not `state` from the issues one. The
// distinction is the whole check: a PR that was closed WITHOUT merging carries
// state=closed and merged=false, and treating it as a supersession source closes a live
// issue in favour of work that never landed. `state == "closed"` is not a merge, and
// `merged_at != null` on the issues payload is not read here either — one endpoint, one
// field, no inference.
func requireMergedPR(repo string, n int) error {
	raw, err := runGH("api", "-H", "Accept: application/vnd.github+json",
		fmt.Sprintf("repos/%s/pulls/%d", repo, n))
	if err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot read %s#%d as a pull request — whether it MERGED is unknown, "+
				"so it cannot stand as the target of a close", repo, n), err)
	}
	var pr pullRequest
	if jerr := json.Unmarshal([]byte(raw), &pr); jerr != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d did not parse as a pull request", repo, n), jerr)
	}
	if !pr.Merged {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d is a pull request in state %q that has NOT merged. Closed-unmerged never "+
				"satisfies a supersession or review-request lane: the work it carried did not land, so "+
				"closing an issue in its favour retires a live problem in favour of nothing.",
			repo, n, deskkit.StripControl(pr.State)))
	}
	return nil
}

// prRefRe finds `#<N>`, `<owner>/<repo>#<N>` and PR permalinks in an issue body.
var prRefRe = regexp.MustCompile(
	`(?:https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+/pull/(\d+)\b)|(?:\B#(\d+)\b)`)

// extractPRRef pulls the single PR reference out of a review-request issue body.
//
// Zero refs and MORE THAN ONE DISTINCT ref are both refusals (exit 5). Picking the
// first of several is a guess, and a guess here closes the wrong issue — the caller is
// told to use `superseded --by` and state the target explicitly.
func extractPRRef(repo string, it item) (int, error) {
	seen := map[int]bool{}
	var uniq []int
	for _, m := range prRefRe.FindAllStringSubmatch(it.Body, -1) {
		s := m[1]
		if s == "" {
			s = m[2]
		}
		n, err := atoiPositive(s, "PR reference")
		if err != nil || n == it.Number {
			continue
		}
		if !seen[n] {
			seen[n] = true
			uniq = append(uniq, n)
		}
	}
	switch len(uniq) {
	case 1:
		return uniq[0], nil
	case 0:
		return 0, deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d names no pull request in its body, so the review-request lane has nothing "+
				"to verify as merged. Use `deskclose superseded -R %s %d --by <ref>` and state the target.",
			repo, it.Number, repo, it.Number))
	default:
		return 0, deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d names %d distinct pull requests %v — ambiguous. Choosing one would be a "+
				"guess, and a guess here closes the wrong item; state the target with "+
				"`deskclose superseded --by <ref>`.", repo, it.Number, len(uniq), uniq))
	}
}

// postComment writes the pre-close comment. It runs BEFORE the close, always, so the
// trail naming the lane, the canonical target and the authorizing ruling survives even
// a batch that turns out to be wrong — a reader of a wrongly-closed issue can see why
// it was closed and by whose authority, and reopen is cheap.
func postComment(repo string, n int, body string) error {
	_, err := runGH("issue", "comment", fmt.Sprintf("%d", n), "-R", repo, "--body", body)
	if err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: the pre-close comment on %s#%d may or may not have posted", repo, n), err)
	}
	return nil
}

// closeItem performs the close. reason is a GitHub state_reason and is only meaningful
// for issues; a PR has no state_reason, so the lane is carried by the comment written
// immediately before this call.
func closeItem(repo string, n int, isPR bool, reason string) error {
	var err error
	if isPR {
		_, err = runGH("pr", "close", fmt.Sprintf("%d", n), "-R", repo)
	} else {
		_, err = runGH("issue", "close", fmt.Sprintf("%d", n), "-R", repo, "--reason", reason)
	}
	if err != nil {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: closing %s#%d did not confirm", repo, n), err)
	}
	return nil
}
