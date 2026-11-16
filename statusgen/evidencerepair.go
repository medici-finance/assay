package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// A sanctioned editorial repair of historical Evidence is its own entry in the
// acceptance chain (#6616):
//
//	Evidence-Repair: commit=<full-sha> approval=<issue-or-PR-comment URL>
//
// The byte-prefix check stays strict. A repair only moves the baseline that
// check compares against, from the pre-amendment Evidence to the Evidence at
// the repair commit, and only after the repair is shown to be editorial: it
// leaves Verify untouched, keeps every table row and every non-Date/Runner
// cell byte-identical, splits a combined "Date / Runner" column or adds one
// `(on-behalf-of human:<name>)` principal to a Runner, and adds only prose
// that carries no marker or verdict. A mapped human's own unedited comment,
// read through the same forge resolver as Acceptance-Transition approvals,
// must name the brief, and the repair commit's message must name the
// approval's issue or pull request.
type evidenceRepair struct {
	commit, approval string
	link             rulingLink
}

// evidenceRepairURLRe accepts an issue or pull-request comment. A pull
// request's conversation comments are issue comments on the same number.
var evidenceRepairURLRe = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)/(issues|pull)/([1-9][0-9]*)#issuecomment-([1-9][0-9]*)$`)

var evidenceRepairPrincipalRe = regexp.MustCompile(`^ \(on-behalf-of human:[0-9A-Za-z_]+\)$`)

var evidenceRepairBriefRe = regexp.MustCompile(`^brief-([0-9]+[a-z]?)-`)

// evidenceRepairs validates the descriptor grammar before any history read.
func evidenceRepairs(run string) ([]evidenceRepair, string) {
	var repairs []evidenceRepair
	seen := map[string]bool{}
	for _, m := range protectedMarkersNamed(normalizedProtectedEvidence(run), "Evidence-Repair:") {
		f := strings.Fields(m.text)
		if !m.literal || len(f) != 3 || !strings.HasPrefix(f[1], "commit=") || !strings.HasPrefix(f[2], "approval=") {
			return nil, "evidence repair: expected Evidence-Repair: commit=<full-sha> approval=<comment-url>"
		}
		c, u := strings.TrimPrefix(f[1], "commit="), strings.TrimPrefix(f[2], "approval=")
		if !isObjectID(c) {
			return nil, "evidence repair: a full immutable repair commit is required"
		}
		p := evidenceRepairURLRe.FindStringSubmatch(u)
		if p == nil {
			return nil, "evidence repair: a valid issue or pull-request comment approval URL is required"
		}
		n, e1 := strconv.Atoi(p[4])
		id, e2 := strconv.ParseInt(p[5], 10, 64)
		if e1 != nil || e2 != nil {
			return nil, "evidence repair: a valid issue or pull-request comment approval URL is required"
		}
		link := rulingLink{Repo: p[1] + "/" + p[2], Issue: n, CommentID: id}
		if seen[c] || seen[fmt.Sprint(id)] {
			return nil, "evidence repair: a repair commit or approval cannot appear twice"
		}
		seen[c], seen[fmt.Sprint(id)] = true, true
		repairs = append(repairs, evidenceRepair{commit: c, approval: u, link: link})
	}
	return repairs, ""
}

// evidenceRepairBaseline returns the Evidence the current record must retain as
// its byte prefix, and the repairs that moved it. historical is the Evidence at
// predecessor (the amendment's first parent). first requires every repair to
// follow predecessor, so no repair can sit unused before the chain.
func evidenceRepairBaseline(path, historical, run, predecessor string, first bool, read func(string) (string, error)) (string, []evidenceRepair, string) {
	repairs, why := evidenceRepairs(run)
	if why != "" || len(repairs) == 0 {
		return historical, nil, why
	}
	dir, file := filepath.Dir(path), filepath.Base(path)
	order, e := transitionFirstParents(dir, predecessor)
	if e != nil {
		return "", nil, "evidence repair: cannot read repair ancestry"
	}
	position := map[string]int{}
	for i, id := range strings.Fields(string(order)) {
		position[id] = i
	}
	byCommit := map[string]evidenceRepair{}
	var applied []string
	for _, r := range repairs {
		if _, ok := position[r.commit]; !ok {
			if first {
				return "", nil, fmt.Sprintf("evidence repair: %.12s is not after the amendment's parent on the observed first-parent chain", r.commit)
			}
			continue
		}
		byCommit[r.commit] = r
		applied = append(applied, r.commit)
	}
	// Apply in first-parent order, never descriptor order.
	for i := 1; i < len(applied); i++ {
		for j := i; j > 0 && position[applied[j]] < position[applied[j-1]]; j-- {
			applied[j], applied[j-1] = applied[j-1], applied[j]
		}
	}
	baseline := historical
	var used []evidenceRepair
	for _, commit := range applied {
		short := fmt.Sprintf("evidence repair %.12s: ", commit)
		pp, e := provenanceParents(dir, commit)
		if e != nil || len(pp) < 1 {
			return "", nil, short + "cannot read the repair's parent"
		}
		before, e1 := flipEvidenceAt(dir, pp[0], file)
		after, e2 := flipEvidenceAt(dir, commit, file)
		if e1 != nil || e2 != nil || before == flipAbsent || after == flipAbsent {
			return "", nil, short + "cannot read Evidence around the repair"
		}
		if !strings.HasPrefix(strings.TrimSpace(before), strings.TrimSpace(baseline)) {
			return "", nil, short + "historical Evidence was not preserved before the repair"
		}
		verifyBefore, e1 := read(pp[0])
		verifyAfter, e2 := read(commit)
		if e1 != nil || e2 != nil || verifyBefore != verifyAfter {
			return "", nil, short + "a repair cannot touch any Verify row"
		}
		if before == after {
			return "", nil, short + "the repair does not change Evidence"
		}
		if why := evidenceRepairEditorial(before, after); why != "" {
			return "", nil, short + why
		}
		msg, e := historyGit(dir, "log", "-1", "--format=%B", commit).Output()
		if e != nil || !commentNamesRecord(string(msg), fmt.Sprintf("#%d", byCommit[commit].link.Issue)) {
			return "", nil, short + "the repair commit does not name its approval's issue or pull request"
		}
		baseline = after
		used = append(used, byCommit[commit])
	}
	return baseline, used, ""
}

// evidenceRepairEditorial is the content rule. Table lines pair 1:1 in order;
// only the Date/Runner split and an added principal may differ. Every other
// prior line survives in order; an added line is prose with no marker or
// verdict.
func evidenceRepairEditorial(before, after string) string {
	isRow := func(l string) bool { return strings.HasPrefix(strings.TrimSpace(l), "|") }
	var oldRows, newRows, oldText, newText []string
	for _, l := range strings.Split(before, "\n") {
		if isRow(l) {
			oldRows = append(oldRows, l)
		} else {
			oldText = append(oldText, l)
		}
	}
	for _, l := range strings.Split(after, "\n") {
		if isRow(l) {
			newRows = append(newRows, l)
		} else {
			newText = append(newText, l)
		}
	}
	if len(oldRows) != len(newRows) {
		return "a repair cannot add or remove table rows"
	}
	var header []string
	split := false
	for i := range oldRows {
		o, n := cellsTrimmed(oldRows[i]), cellsTrimmed(newRows[i])
		if oldRows[i] == newRows[i] {
			if isHeaderCells(o) {
				header, split = o, false
			}
			continue
		}
		if isSeparatorCells(o) {
			if !isSeparatorCells(n) || len(n) != len(o)+1 || !split {
				return "a repair changed a table separator"
			}
			continue
		}
		if isHeaderCells(o) {
			at := repairColumn(o, "Date / Runner")
			if at < 0 || len(n) != len(o)+1 {
				return "a repair may only split the Date / Runner header"
			}
			want := append(append(append([]string{}, o[:at]...), "Date", "Runner"), o[at+1:]...)
			if strings.Join(want, "\x00") != strings.Join(n, "\x00") {
				return "a repair may only split the Date / Runner header"
			}
			header, split = o, true
			continue
		}
		if header == nil {
			return "a repair changed a row outside any table"
		}
		if split {
			at := repairColumn(header, "Date / Runner")
			if len(o) != len(header) || len(n) != len(o)+1 {
				return "a repair changed a row's shape"
			}
			for j := range o {
				if j == at {
					continue
				}
				k := j
				if j > at {
					k = j + 1
				}
				if o[j] != n[k] {
					return fmt.Sprintf("a repair changed the %s cell of row %s", header[j], o[0])
				}
			}
			if !principalOnly(o[at], n[at]+" "+n[at+1]) {
				return fmt.Sprintf("a repair changed the Date / Runner text of row %s", o[0])
			}
			continue
		}
		runner := repairColumn(header, "Runner")
		if runner < 0 || len(o) != len(n) {
			return "a repair changed a row outside the Date/Runner split"
		}
		for j := range o {
			if j == runner {
				continue
			}
			if o[j] != n[j] {
				name := "unnamed"
				if j < len(header) {
					name = header[j]
				}
				return fmt.Sprintf("a repair changed the %s cell of row %s", name, o[0])
			}
		}
		if !principalOnly(o[runner], n[runner]) {
			return fmt.Sprintf("a repair changed the Runner text of row %s", o[0])
		}
	}
	// Prior prose survives in order; additions are commentary only.
	var added []string
	j := 0
	for _, l := range newText {
		if j < len(oldText) && l == oldText[j] {
			j++
			continue
		}
		added = append(added, l)
	}
	if j != len(oldText) {
		return "a repair removed or altered historical Evidence text"
	}
	for _, l := range added {
		for _, m := range []string{"Verification-Table:", "Acceptance-Transition:", "Evidence-Repair:", "Verification-Attestation:"} {
			if strings.Contains(l, m) {
				return "a repair cannot add a marker"
			}
		}
	}
	if len(flipVerdictTokens(added)) > 0 {
		return "a repair cannot add a verdict"
	}
	return ""
}

// principalOnly: unchanged, or exactly one added on-behalf-of principal.
func principalOnly(old, next string) bool {
	return next == old || (strings.HasPrefix(next, old) && evidenceRepairPrincipalRe.MatchString(next[len(old):]) && !strings.Contains(old, "(on-behalf-of "))
}

func cellsTrimmed(line string) []string {
	cells := splitRowEscaped(line)
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

func isHeaderCells(c []string) bool { return len(c) > 1 && c[0] == "#" }

func isSeparatorCells(c []string) bool {
	if len(c) == 0 {
		return false
	}
	for _, s := range c {
		if strings.Trim(s, ":-") != "" || s == "" {
			return false
		}
	}
	return true
}

func repairColumn(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

// evidenceRepairBriefID is the stream/NN id a repair approval must name.
func evidenceRepairBriefID(rel string) string {
	m := evidenceRepairBriefRe.FindStringSubmatch(filepath.Base(rel))
	if m == nil {
		return ""
	}
	return filepath.Base(filepath.Dir(rel)) + "/" + m[1]
}

// corroborateEvidenceRepairs reads each used repair's approval through the
// Acceptance-Transition resolver: same repository, unedited, non-bot, mapped
// human, and naming the brief. A pull-request comment is admitted only here.
func corroborateEvidenceRepairs(repo, rel string, repairs []evidenceRepair, client func() *ghClient) string {
	id := evidenceRepairBriefID(rel)
	if id == "" && len(repairs) > 0 {
		return "evidence repair: cannot derive the brief id an approval must name"
	}
	for _, r := range repairs {
		fmt.Fprintln(os.Stderr, "acceptance transition: reading evidence-repair approval", r.approval)
		canonical := fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-%d", r.link.Repo, r.link.Issue, r.link.CommentID)
		out := resolveRulingOn(client(), repo, decisionRecordStamp{ID: id, Ruling: canonical}, scanEffectiveConfig().HumanLogins, rulingIssueBinding{
			what:      "evidence repair for " + rel,
			allowPull: true,
			rules:     func(rulingIssue, rulingLink) bool { return true },
		})
		if !out.Present || out.Reason != rulingOK {
			if out.Reason == rulingUnresolvable {
				return fmt.Sprintf("evidence repair %.12s: could-not-check human authority: %s", r.commit, out.Detail)
			}
			return fmt.Sprintf("evidence repair %.12s: human approval could not be corroborated: %s", r.commit, out.Detail)
		}
	}
	return ""
}
