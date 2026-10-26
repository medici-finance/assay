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
	"github.com/medici-finance/assay/statusgen/workref/trailers"

	"bytes"
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
	// Delivers / Authors are the ids a `Brief:` / `Authors:` trailer in the merge
	// commit (or, for a merge commit, its branch's commits) names. A PR that names a
	// brief only in its subject or branch carries neither: whether it delivered
	// the brief is then unconfirmed, not assumed (mergeDelivery).
	Delivers []string
	Authors  []string
}

// mergeDelivery is what a merged PR is known to have done to one brief.
type mergeDelivery int

const (
	// mergeUnconfirmed — the PR names the brief in its subject/branch and carries
	// no trailer either way. A lead for the NOTICE, never proof of delivery.
	mergeUnconfirmed mergeDelivery = iota
	// mergeDelivered — a `Brief:` trailer names the brief: the implementation.
	mergeDelivered
	// mergeAuthoring — only an `Authors:` trailer names the brief: the PR WROTE the
	// brief and delivered none of it. It never advances a status.
	mergeAuthoring
)

// deliveryOf classifies what this merge did to brief id. A `Brief:` trailer
// outranks an `Authors:` one (a PR may author one brief and deliver another, but a
// brief both authored and delivered by one PR was delivered).
func (p mergedPR) deliveryOf(id string) mergeDelivery {
	for _, d := range p.Delivers {
		if d == id {
			return mergeDelivered
		}
	}
	for _, a := range p.Authors {
		if a == id {
			return mergeAuthoring
		}
	}
	return mergeUnconfirmed
}

// mergeTrailerKeys reads the Brief: and Authors: trailer values out of a commit
// body with the same fence-aware grammar parseBriefLikeTrailers uses, each reduced
// to its canonical `<stream>/<NN>` key. Pure; no I/O.
func mergeTrailerKeys(body string) (delivers, authors []string) {
	for _, t := range trailers.Scan([]byte(body)) {
		if t.Kind != trailers.TrailerBrief && t.Kind != trailers.TrailerAuthors {
			continue
		}
		key := canonicalBriefKey(strings.TrimRight(t.Value, ".,;:"))
		if t.Kind == trailers.TrailerBrief {
			delivers = append(delivers, key)
		} else {
			authors = append(authors, key)
		}
	}
	return delivers, authors
}

// reMergeTildeFence is the `~~~` fence opener/closer rePRLinkFence (backticks only)
// does not cover: a trailer shown inside a tilde fence is documentation, not delivery.

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
//
// NO COMMIT TEXT IS EVER PARSED FOR STRUCTURE. A commit message is author-controlled
// bytes, and a crafted object can carry any byte, NUL included (git refuses NUL only on
// the porcelain path). So the topology comes from `rev-list --parents`, whose output is
// nothing but object names, each validated as a full hex name before it is used as a
// key or an argument; and the messages come from `cat-file --batch`, whose records are
// length-prefixed, so no byte inside a message can end a record early or invent one.
// A branch read that fails is a failed read: the merges it would have confirmed are
// not reported as trailer-less (could-not-check, never "no `Brief:` trailer").
func mergedPRsFromGit(root string) ([]mergedPR, error) {
	window, parents, err := revListParents(root, []string{"--first-parent", "-n", strconv.Itoa(mergedPRLimit)}, "HEAD")
	if err != nil {
		return nil, err
	}
	if len(window) == 0 {
		return nil, nil
	}
	// The branch commits of every merge in the window: everything reachable from HEAD
	// that is not behind the oldest window commit's FIRST parent. Excluding only the
	// first parent keeps the oldest merge's own branch (its second parent) in the read.
	rangeArgs := []string{"HEAD"}
	if p := parents[window[len(window)-1]]; len(p) > 0 {
		rangeArgs = append(rangeArgs, "^"+p[0])
	}
	all, allParents, err := revListParents(root, nil, rangeArgs...)
	if err != nil {
		return nil, fmt.Errorf("branch-commit read: %w", err)
	}
	msgs, err := commitMessages(root, all)
	if err != nil {
		return nil, fmt.Errorf("commit-message read: %w", err)
	}
	commits := make(map[string]gitCommit, len(all))
	for _, sha := range all {
		commits[sha] = gitCommit{parents: allParents[sha], msg: msgs[sha]}
	}
	return parseMergedLog(window, commits, branchBodiesFrom(commits, window)), nil
}

// gitCommit is one commit reduced to what the merge read needs: its parents (from
// rev-list, which honours a shallow boundary) and its raw message (%B).
type gitCommit struct {
	parents []string
	msg     string
}

// mergeSHARe is a full object name, SHA-1 or SHA-256. Every name read out of git
// passes it before it is used as a map key or a git argument.
var mergeSHARe = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// revListParents runs `git rev-list --parents <opts> --end-of-options <revs>` and
// returns the commits in output order with each one's parent list. Every token must
// be a full object name; anything else is a malformed read, reported as an error
// rather than skipped.
func revListParents(root string, opts []string, revs ...string) ([]string, map[string][]string, error) {
	args := append([]string{"-C", root, "rev-list", "--parents"}, opts...)
	args = append(append(args, "--end-of-options"), revs...)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, nil, err
	}
	return parseRevListParents(string(out))
}

// parseRevListParents is the pure half of revListParents.
func parseRevListParents(out string) ([]string, map[string][]string, error) {
	var order []string
	parents := map[string][]string{}
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, " ")
		for _, sha := range f {
			if !mergeSHARe.MatchString(sha) {
				return nil, nil, fmt.Errorf("rev-list printed a malformed object name %q", sha)
			}
		}
		order = append(order, f[0])
		parents[f[0]] = f[1:]
	}
	return order, parents, nil
}

// commitMessages reads the raw message of every named commit in ONE
// `git cat-file --batch` call.
func commitMessages(root string, shas []string) (map[string]string, error) {
	if len(shas) == 0 {
		return map[string]string{}, nil
	}
	cmd := exec.Command("git", "-C", root, "cat-file", "--batch")
	cmd.Stdin = strings.NewReader(strings.Join(shas, "\n") + "\n")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseCatFileBatch(out, shas)
}

// parseCatFileBatch splits `git cat-file --batch` output into each commit's message.
// Records are `<name> <type> <size>\n<size bytes>\n`, in request order; the size is
// what bounds a record, so the content's own bytes are never scanned for a boundary.
// The message is everything after the header block's first blank line.
func parseCatFileBatch(out []byte, want []string) (map[string]string, error) {
	msgs := make(map[string]string, len(want))
	pos := 0
	for _, sha := range want {
		nl := bytes.IndexByte(out[pos:], '\n')
		if nl < 0 {
			return nil, fmt.Errorf("cat-file output ends before %s", sha)
		}
		hdr := strings.Fields(string(out[pos : pos+nl]))
		pos += nl + 1
		if len(hdr) != 3 || hdr[0] != sha || hdr[1] != "commit" {
			return nil, fmt.Errorf("cat-file record for %s has header %q", sha, strings.Join(hdr, " "))
		}
		size, err := strconv.Atoi(hdr[2])
		if err != nil || size < 0 || pos+size+1 > len(out) || out[pos+size] != '\n' {
			return nil, fmt.Errorf("cat-file record for %s is truncated", sha)
		}
		obj := out[pos : pos+size]
		pos += size + 1
		if i := bytes.Index(obj, []byte("\n\n")); i >= 0 {
			msgs[sha] = string(obj[i+2:])
		} else {
			msgs[sha] = ""
		}
	}
	return msgs, nil
}

// splitCommitMessage is git's %s / %b split: the title paragraph folded onto one
// line, and the body after it.
func splitCommitMessage(msg string) (subject, body string) {
	lines := strings.Split(strings.TrimLeft(msg, "\n"), "\n")
	i := 0
	var title []string
	for ; i < len(lines) && strings.TrimSpace(lines[i]) != ""; i++ {
		title = append(title, strings.TrimSpace(lines[i]))
	}
	return strings.Join(title, " "), strings.TrimLeft(strings.Join(lines[i:], "\n"), "\n")
}

// branchBodiesFrom returns the branch-commit reader parseMergedLog consults for a
// `Merge pull request` commit whose own body carries no trailer (a merge commit's body
// is only the PR title; the trailers live on the branch's own commits). firstParent
// lists the first-parent shas newest first. Walking merges oldest first and claiming
// every branch commit it visits keeps a branch that merged main into itself from
// absorbing main's commits: those are either on the first-parent line or already
// claimed by the older merge that brought them in.
func branchBodiesFrom(commits map[string]gitCommit, firstParent []string) func(sha string) string {
	onLine := map[string]bool{}
	for _, sha := range firstParent {
		onLine[sha] = true
	}
	claimed := map[string]bool{}
	bodies := map[string]string{}
	for i := len(firstParent) - 1; i >= 0; i-- {
		m, ok := commits[firstParent[i]]
		if !ok || len(m.parents) < 2 {
			continue
		}
		var b strings.Builder
		stack := []string{m.parents[1]}
		for len(stack) > 0 {
			sha := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			c, ok := commits[sha]
			if !ok || onLine[sha] || claimed[sha] {
				continue
			}
			claimed[sha] = true
			b.WriteString(c.msg)
			b.WriteString("\n")
			for j := len(c.parents) - 1; j >= 0; j-- {
				stack = append(stack, c.parents[j])
			}
		}
		bodies[firstParent[i]] = b.String()
	}
	return func(sha string) string { return bodies[sha] }
}

// parseMergedLog turns the first-parent window (newest first) into merged PRs.
// branchBody, when non-nil, supplies the branch commits' bodies for a `Merge pull
// request` commit whose own body carries no trailer. Pure given branchBody — the git
// reads live in the caller.
func parseMergedLog(window []string, commits map[string]gitCommit, branchBody func(sha string) string) []mergedPR {
	var prs []mergedPR
	for _, sha := range window {
		c, ok := commits[sha]
		if !ok {
			continue
		}
		subject, body := splitCommitMessage(c.msg)
		if subject == "" {
			continue
		}
		num, branch, isMerge := 0, "", false
		if m := mergeSubjectRe.FindStringSubmatch(subject); m != nil {
			num, _ = strconv.Atoi(m[1])
			branch = m[2]
			isMerge = true
		} else if m := squashSubjectRe.FindStringSubmatch(subject); m != nil {
			num, _ = strconv.Atoi(m[1])
		} else {
			continue // an ordinary commit on main: not a merged PR
		}
		refs := briefRefsIn(subject, branch)
		delivers, authors := mergeTrailerKeys(body)
		// A trailer-less merge commit reads its trailers from the branch commits whatever
		// the branch is called: the house shape (`feat/<repo>--<stream>--NN`) names no
		// brief, so gating on a ref already found would never read the dominant shape.
		if isMerge && branchBody != nil && len(delivers) == 0 && len(authors) == 0 {
			delivers, authors = mergeTrailerKeys(branchBody(sha))
		}
		seen := map[string]bool{}
		for _, r := range refs {
			seen[r] = true
		}
		for _, d := range delivers {
			if !seen[d] {
				seen[d] = true
				refs = append(refs, d)
			}
		}
		if len(refs) == 0 {
			continue
		}
		sort.Strings(refs)
		prs = append(prs, mergedPR{Number: num, Subject: subject, Briefs: refs, Delivers: delivers, Authors: authors})
	}
	return prs
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
			if pr.deliveryOf(id) == mergeAuthoring {
				continue // an Authors:-only PR WROTE the brief; it advances nothing
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
		confirm := ""
		if h.pr.deliveryOf(id) != mergeDelivered {
			confirm = " Delivery is UNCONFIRMED (no `Brief:` trailer names it, so this may be an authoring or " +
				"partial PR): the row is a lead, not held from dispatch on this evidence alone."
		}
		notices = append(notices, fmt.Sprintf(
			"%s names %s but its README row is still %q — Next-up will keep offering work that has "+
				"already landed (#270). Reconcile the row, or say in the PR why the merge did not "+
				"advance it.%s Merge subject: %s",
			where, id, h.status, confirm, h.pr.Subject))
	}
	return notices
}
