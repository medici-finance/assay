package main

// numberspace.go detects NUMBERING-SPACE COLLISIONS — two authors allocating the
// same identifier out of a shared, hand-maintained numbering space in parallel.
//
// THE DEFECT (desk-hardening/05, #54). `docs/brief-rules.md` numbers its rules by
// hand and the file's own preamble makes the numbering APPEND-ONLY, because other
// docs, skills, workflows and Go comments cite rules BY NUMBER. Two branches that
// each append "the next free number" are each internally consistent; neither diff
// contains the other's number; both pass every check that reads one tree. The
// collision exists only in the MERGED tree, and it is not a textual conflict —
// the two additions are usually in different sections, hundreds of lines apart,
// so git merges them cleanly and nothing goes red.
//
// Measured on main at 2026-08-13: rule numbers 25 and 26 each appear TWICE
// (`^25. **` at both the Evidence and the Row-runner sections, likewise 26).
// Two briefs authored into the same numbering space in parallel; both merged
// green. "brief-rule 26" is now ambiguous, and there is a live citation resting
// on it — docs/streams/desk-hardening/brief-01-three-state-instrument-invariant.md
// cites "brief-rules 26/28" meaning the Row-runner pair.
//
// WHERE THIS CHECK CAN AND CANNOT SEE IT — stated here because it is the whole
// point of the brief:
//
//   --lint      reads the WORKING TREE's copy. On a branch that is the branch's
//               own copy, which by construction does NOT contain the parallel
//               branch's rule. So --lint CANNOT see a collision before it merges;
//               it sees it only once main already carries both — after main has
//               gone red. Severity is therefore NOTICE (see below).
//   mergecheck  reads the copy in the TRIAL-MERGED tree (mergecheck.go). That is
//               the tree the PR actually lands in, so this is the reading that
//               can see the collision BEFORE the merge. Same detector, different
//               tree; the tree is the load-bearing part, not the parser.
//
// SEVERITY IS NOTICE in --lint, deliberately, following mergedstatus.go's
// precedent. The corpus was already dirty when the check was written (2 duplicate
// numbers on main), and arming a hard gate against an already-red corpus reds
// every unrelated PR on day one. The reconciliation backlog is named in the brief
// (docs/streams/desk-hardening/brief-05-merge-time-recheck.md, "Reconciliation
// backlog"). In `mergecheck` a MERGE-INTRODUCED collision is exit 1, because
// there the finding is scoped to what this merge itself creates — the same
// "closure this branch made is a PROBLEM, the inherited backlog is a NOTICE"
// split verify-inscope.sh already uses.
//
// THREE-STATE. The file may be missing or unreadable (a scoped checkout, a
// non-adopter repo, a permission error). That is could-not-check and it emits a
// NOTICE saying so; it is never reported as "no collisions".
//
// A NUMBERING SPACE CONSIDERED AND DELIBERATELY NOT CHECKED: brief FILE numbers
// within a stream directory (`docs/streams/<stream>/brief-<NN>-*.md`). Measured
// the same day: desk-hardening carries two `brief-06-` files, and that is
// legitimate — brief-06-contract-durable-watchdog.md is a sidecar contract the
// README links from the 06 row, not a second brief 06. A check there would fire
// on a correct corpus, so it is not armed. Naming it is the point (no silent
// caps): this detector covers ONE numbering space, the one in brief-rules.md.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// briefRulesRelPath is the declared source for the rule-number space. One
// declared source per fact (derive-or-diff): the numbers live here and nowhere
// else, and every other mention of a rule number is a citation of this file.
const briefRulesRelPath = "docs/brief-rules.md"

// ruleNumRe matches a top-level, bolded, numbered rule: `25. **The ...`.
// Anchored at column 0 on purpose — the file indents continuation paragraphs and
// nested lists, and an indented `1.` inside a rule's body is not a rule.
var ruleNumRe = regexp.MustCompile(`(?m)^(\d+)\.[ \t]+\*\*(.*)$`)

// ruleRef is one rule heading found in the file.
type ruleRef struct {
	Num   int    // the allocated number
	Line  int    // 1-based line number, for the finding text
	Title string // the bolded heading text, trimmed — this is what "cite by expression" cites
}

// briefRuleNumbers extracts every allocated rule number from brief-rules.md
// content, in file order.
//
// Fenced code blocks are blanked first (registerrefs.go's stripFences, which
// preserves line numbering so a finding's reported line still matches the file):
// a numbered line inside a ```-fenced example is a sample, not an allocation.
func briefRuleNumbers(content string) []ruleRef {
	stripped := stripFences(content)
	var out []ruleRef
	for _, m := range ruleNumRe.FindAllStringSubmatchIndex(stripped, -1) {
		numStr := stripped[m[2]:m[3]]
		n, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		title := strings.TrimSpace(stripped[m[4]:m[5]])
		title = strings.TrimSuffix(title, "**")
		title = strings.TrimSpace(title)
		if len(title) > 72 {
			title = title[:72] + "…"
		}
		line := 1 + strings.Count(stripped[:m[0]], "\n")
		out = append(out, ruleRef{Num: n, Line: line, Title: title})
	}
	return out
}

// numberSpaceCollisions returns one finding string per number allocated more than
// once, sorted by number. An empty slice means checked-clean for THIS content —
// it says nothing about any other tree, which is why the caller decides which
// tree to hand it.
func numberSpaceCollisions(content string) []string {
	byNum := map[int][]ruleRef{}
	for _, r := range briefRuleNumbers(content) {
		byNum[r.Num] = append(byNum[r.Num], r)
	}
	nums := make([]int, 0, len(byNum))
	for n, refs := range byNum {
		if len(refs) > 1 {
			nums = append(nums, n)
		}
	}
	sort.Ints(nums)

	out := make([]string, 0, len(nums))
	for _, n := range nums {
		refs := byNum[n]
		where := make([]string, 0, len(refs))
		for _, r := range refs {
			where = append(where, fmt.Sprintf("L%d %q", r.Line, r.Title))
		}
		out = append(out, fmt.Sprintf(
			"rule number %d is allocated %d times in %s — %s. A citation of \"brief-rule %d\" "+
				"is ambiguous and cannot be resolved. The numbering is append-only because other "+
				"files cite by number: give the later allocation the next number above the current "+
				"maximum and update anything citing it.",
			n, len(refs), briefRulesRelPath, strings.Join(where, " and "), n))
	}
	return out
}

// briefRuleNumberNotices is the --lint half. It reads the WORKING TREE copy and
// returns NOTICEs, including an explicit could-not-check NOTICE when the file
// exists but cannot be read.
//
// A MISSING file is not a finding and not a could-not-check: most repos that
// adopt statusgen have no brief-rules.md, and inventing a NOTICE for every one of
// them is alarm flooding, not a check.
func briefRuleNumberNotices(root string) []string {
	path := filepath.Join(root, filepath.FromSlash(briefRulesRelPath))
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return []string{fmt.Sprintf("could-not-check: the rule-number space in %s could not be read — %v. "+
			"No conclusion about numbering collisions is drawn from this run.", briefRulesRelPath, err)}
	}
	findings := numberSpaceCollisions(string(b))
	if len(findings) == 0 {
		return nil
	}
	out := make([]string, 0, len(findings)+1)
	for _, f := range findings {
		out = append(out, f)
	}
	out = append(out, fmt.Sprintf(
		"the %d numbering collision(s) above are read from the WORKING TREE, which on a branch "+
			"contains only that branch's rules — this reading cannot see a collision with a "+
			"parallel branch before it merges. `statusgen mergecheck` runs the same detector "+
			"against the trial-merged tree, which can.", len(findings)))
	return out
}
