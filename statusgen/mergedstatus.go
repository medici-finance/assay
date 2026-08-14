package main

// mergedstatus.go RECONCILES merged pull requests against the stream-README status
// cells, so a brief whose work has already landed cannot keep being offered as
// unclaimed work.
//
// The defect it closes (#270, with siblings #357, #339, #665, #656): merge 3585164
// (PR #255) landed desk-hardening/01's deliverables, but the README row stayed `todo`
// with empty Evidence — so Next-up went on offering desk-hardening/01 at score 3500 and
// the board pointed the fleet at re-doing merged work. The status cell is a HAND-COPIED
// second copy of a fact that git already records authoritatively.
//
// Derive-or-diff says: the merge is the source, the cell is the copy, and the copy gets
// DIFFED against the source in the lint. This is the diff half — the cell stays
// hand-written (a status transition is a judgement, not a mechanical consequence of a
// merge: a PR can land part of a brief), but a cell that contradicts a merge is now
// visible instead of silent.
//
// SEVERITY IS NOTICE, deliberately and per the brief. Promotion to PROBLEM is a later
// ruling, once the standing backlog of already-drifted rows is reconciled — arming a hard
// gate against a corpus that is already red would red every unrelated PR on day one.
//
// THREE-STATE. The merge history is read from git, which can fail (a `git archive`
// export, a shallow clone with no merges in range, git missing). A failed read emits a
// could-not-check NOTICE and no findings. It never reports "no drift" for history it
// could not read.

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// mergedPR is one merge commit, reduced to what the reconciliation needs.
type mergedPR struct {
	Number  int      // PR number from the merge subject, 0 when not a `Merge pull request` subject
	Subject string   // the merge commit subject, for the NOTICE text
	Briefs  []string // "<stream>/<NN>" ids named by the branch or the subject
}

// mergeSubjectRe matches GitHub's merge-commit subject and captures PR number + branch.
var mergeSubjectRe = regexp.MustCompile(`^Merge pull request #(\d+) from [^/\s]+/(\S+)`)

// squashSubjectRe matches the squash-merge subject form, `... (#1234)`. It carries the PR
// number but no branch name, so a brief id can only come from the subject text itself.
var squashSubjectRe = regexp.MustCompile(`\(#(\d+)\)\s*$`)

// briefIDRe matches an explicit `<stream>/<NN>` reference in free text.
var briefIDRe = regexp.MustCompile(`\b([a-z][a-z0-9]*(?:-[a-z0-9]+)*)/(\d{2}[a-z]?)\b`)

// briefBranchRe matches the fleet's branch convention, `brief/<stream>-<NN>-<slug>` (and
// the `shepherd/`, `fix/` variants that keep the same tail). The stream part is
// non-greedy so `brief/desk-hardening-01-three-state` splits at the FIRST two-digit group
// that is followed by a separator or the end — `desk-hardening` + `01`.
var briefBranchRe = regexp.MustCompile(`(?:^|/)(?:brief|shepherd)/([a-z][a-z0-9-]*?)-(\d{2}[a-z]?)(?:-|$)`)

// briefRefsIn pulls every candidate `<stream>/<NN>` id out of one merge subject and its
// branch name. Candidates are NOT validated here — validation is against the real stream
// set in mergedPRStatusNotices, which is what keeps an unrelated `foo/12` in a commit
// subject from inventing a finding.
func briefRefsIn(subject, branch string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(stream, num string) {
		id := stream + "/" + num
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, m := range briefIDRe.FindAllStringSubmatch(subject, -1) {
		add(m[1], m[2])
	}
	if branch != "" {
		if m := briefBranchRe.FindStringSubmatch("/" + branch); m != nil {
			add(m[1], m[2])
		}
		for _, m := range briefIDRe.FindAllStringSubmatch(branch, -1) {
			add(m[1], m[2])
		}
	}
	sort.Strings(out)
	return out
}

// mergedPRLimit bounds how far back the reconciliation looks. Deep enough to cover the
// window in which a row could still be wrongly `todo` (months of merges at this fleet's
// rate), shallow enough that --lint stays a sub-second offline check.
const mergedPRLimit = 400

// mergedPRsFromGit reads recent first-parent merges of the current branch.
//
// --first-parent is what keeps this to the integration history: without it every merged
// side branch's own merges appear, and a topic branch that merged main into itself would
// read as a PR merge of main.
func mergedPRsFromGit(root string) ([]mergedPR, error) {
	out, err := exec.Command("git", "-C", root, "log", "--first-parent",
		"-n", strconv.Itoa(mergedPRLimit), "--format=%s", "HEAD").Output()
	if err != nil {
		return nil, err
	}
	var prs []mergedPR
	for _, subject := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		subject = strings.TrimSpace(subject)
		if subject == "" {
			continue
		}
		num, branch := 0, ""
		if m := mergeSubjectRe.FindStringSubmatch(subject); m != nil {
			num, _ = strconv.Atoi(m[1])
			branch = m[2]
		} else if m := squashSubjectRe.FindStringSubmatch(subject); m != nil {
			num, _ = strconv.Atoi(m[1])
		} else {
			continue // an ordinary commit on main: not a merged PR
		}
		refs := briefRefsIn(subject, branch)
		if len(refs) == 0 {
			continue
		}
		prs = append(prs, mergedPR{Number: num, Subject: subject, Briefs: refs})
	}
	return prs, nil
}

// unreconciledStatuses are the row states a merged PR contradicts. `blocked` is NOT in
// the set: a brief can legitimately be blocked after a partial merge. `implemented` and
// beyond are past the line this check draws.
var unreconciledStatuses = map[string]bool{"todo": true, "in-progress": true}

// mergedPRStatusNotices diffs merged PRs against the status cells and returns one NOTICE
// per contradicting row.
//
// readErr is the git read's own verdict; non-nil produces exactly one could-not-check
// NOTICE and no findings, regardless of what partial data came back.
//
// A brief id that names no known stream/row is IGNORED. That is the false-positive
// control: `.github/workflows/foo-12` style noise, or a stream that has since been
// archived, must not manufacture a finding on a board it is not on.
func mergedPRStatusNotices(streams []*Stream, merged []mergedPR, readErr error) []string {
	if readErr != nil {
		return []string{fmt.Sprintf("could-not-check: merged-PR/status reconciliation (#270) could not "+
			"read the merge history — %v. No conclusion about status drift is drawn from this run.", readErr)}
	}

	status := map[string]string{} // "<stream>/<NN>" → README-row status
	for _, s := range streams {
		for _, b := range s.Briefs {
			status[s.Name+"/"+b.Num] = b.Status
		}
	}

	// Keep the FIRST (most recent) merge that names each drifted brief: naming the
	// newest merge is what a reader needs to reconcile, and one NOTICE per brief
	// keeps a brief merged in five PRs from producing five identical lines.
	type hit struct {
		pr     mergedPR
		status string
	}
	seen := map[string]hit{}
	for _, pr := range merged {
		for _, id := range pr.Briefs {
			st, known := status[id]
			if !known || !unreconciledStatuses[st] {
				continue
			}
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = hit{pr: pr, status: st}
		}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	notices := make([]string, 0, len(ids))
	for _, id := range ids {
		h := seen[id]
		where := "a merged PR"
		if h.pr.Number > 0 {
			where = fmt.Sprintf("merged PR #%d", h.pr.Number)
		}
		notices = append(notices, fmt.Sprintf(
			"%s names %s but its README row is still %q — Next-up will keep offering work that has "+
				"already landed (#270). Reconcile the row, or say in the PR why the merge did not "+
				"advance it. Merge subject: %s",
			where, id, h.status, h.pr.Subject))
	}
	return notices
}
