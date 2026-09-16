package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// github.go — every remote READ deskclose performs, plus the WRITES it can emit: a comment
// and a close (issue or change), and ONE narrow reopen. There is no merge verb and no edit
// verb anywhere in this package. The reopen (reopenItem) is not a general capability: it
// has exactly one caller, the verify-gate-refire lane (lanes.go), which is role-scoped to
// the verifier session and label-scoped to issues already carrying verify-gate, and which
// always follows the reopen with a close in the SAME invocation — on a surface (the
// verify-gate sign-off card) the desk cannot unilaterally complete a sign-off on
// regardless, because the repository's verify-gate close workflow reopens any bot close.

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

// item is the subset of an issue/PR deskclose reads, built from a single Forge.GetIssue (which
// carries the item's kind, state, labels and body since the write-verbs-C migration added
// Labels/Body to the Issue shape).
type item struct {
	Number int
	Title  string
	State  string // open | closed
	Body   string
	Labels []string
	IsPR   bool
}

func (i item) isPR() bool   { return i.IsPR }
func (i item) closed() bool { return strings.EqualFold(i.State, "closed") }

func (i item) labelNames() []string { return i.Labels }

// fetchItem reads one issue or PR through the resolved forge (GetIssue, which answers the item's
// kind AND carries its labels and body in one read — so an unread label set can never be mistaken
// for an empty one).
//
// A read failure is Unverifiable (exit 6): deskclose cannot know whether the item carries a
// decision label, so it must not close it. There is no "assume no labels" arm — that is the
// unread-precondition failure this tool exists to make impossible.
func fetchItem(repo string, n int, kind deskkit.TargetKind) (item, error) {
	fg, fr, ferr := forgeForFn(repo)
	if ferr != nil {
		return item{}, ferr
	}
	var iss *deskkit.Issue
	var err error
	if kind != "" {
		iss, err = fg.GetIssueTyped(fr, n, kind)
	} else {
		iss, err = fg.GetIssue(fr, n)
	}
	if err != nil {
		detail := ""
		if kind == "" {
			// The forge could not resolve a bare number to one kind — the case a project
			// carrying both an issue and a merge request at that number produces. The refusal
			// underneath is right; what it used to be missing is the spelling of the operation
			// it tells the caller to use, which deskclose now exposes on every mode.
			detail = " State which kind you mean: " + deskkit.TypedRefForms() + "."
		}
		return item{}, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot read %s#%d — its labels and state are unknown, so it is not "+
				"closeable (an unread precondition is never a satisfied one).%s", repo, n, detail), err)
	}
	return item{
		Number: iss.Number,
		Title:  iss.Title,
		State:  iss.State,
		Body:   iss.Body,
		Labels: append([]string(nil), iss.Labels...),
		IsPR:   iss.IsPullRequest,
	}, nil
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

// requireMergedPR is the merged-vs-closed gate for lanes 2 and 3.
//
// It reads the forge's own `Merged` flag (GetPullRequest), not `state`. The distinction is the
// whole check: a PR that was closed WITHOUT merging carries state=closed and merged=false, and
// treating it as a supersession source closes a live issue in favour of work that never landed.
// `state == "closed"` is not a merge, and the seam's PullRequest.Merged flag (independent of
// MergedAt) is read here — one field, no inference.
func requireMergedPR(repo string, n int) error {
	fg, fr, ferr := forgeForFn(repo)
	if ferr != nil {
		return ferr
	}
	pr, err := fg.GetPullRequest(fr, n)
	if err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot read %s#%d as a pull request — whether it MERGED is unknown, "+
				"so it cannot stand as the target of a close", repo, n), err)
	}
	merged := pr.Merged || pr.MergedAt != ""
	if !merged {
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
func postComment(repo string, n int, kind deskkit.TargetKind, body string) error {
	fg, fr, ferr := forgeForFn(repo)
	if ferr != nil {
		return ferr
	}
	if _, err := fg.PostCommentTyped(fr, n, kind, body); err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: the pre-close comment on %s#%d may or may not have posted", repo, n), err)
	}
	return nil
}

// reopenItem reopens ONE issue through the seam's ReopenIssue. One caller: the
// verify-gate-refire lane, which has already read the item, refused a change, and required
// the verify-gate label before reaching this — so the untyped op (issue sequence only) is
// the right one, not a guess. See the package doc above for why this is not a reopen verb.
func reopenItem(repo string, n int) error {
	fg, fr, ferr := forgeForFn(repo)
	if ferr != nil {
		return ferr
	}
	if err := fg.ReopenIssue(fr, n); err != nil {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: reopening %s#%d did not confirm", repo, n), err)
	}
	return nil
}

// closeItem performs the close via CloseIssueTyped, so the close addresses the kind of object
// that was actually READ. The untyped CloseIssue reaches only the ISSUE sequence on a forge
// that numbers the two kinds separately, which on a project carrying both an issue and a
// merge request at one number means the close lands on the other object — a wrong write, not
// a failed one.
//
// reason is a state reason meaningful only for issues; a change carries none on either forge,
// so it is dropped here rather than handed to the seam (which refuses it), and the lane is
// carried by the comment written immediately before this call.
func closeItem(repo string, n int, kind deskkit.TargetKind, reason string) error {
	fg, fr, ferr := forgeForFn(repo)
	if ferr != nil {
		return ferr
	}
	if kind == deskkit.TargetChange {
		reason = ""
	}
	if err := fg.CloseIssueTyped(fr, n, kind, reason); err != nil {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: closing %s#%d did not confirm", repo, n), err)
	}
	return nil
}
