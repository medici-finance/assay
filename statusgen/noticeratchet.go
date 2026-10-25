package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Notice ratchet (statusgen/27).
//
// The differential lint (difflint.go) compares PROBLEMs only: a NOTICE the diff
// introduces used to pass through with a green result, so new verification
// defects replenished every cleanup. The ratchet closes that on the briefs the
// diff touches: an ACTIONABLE finding that is on an added or edited brief at
// head and was not on that brief at the merge-base is promoted to PROBLEM.
// Everything else stays a visible NOTICE.
//
// One classifier owns the decision (parseLintNotice). It turns a rendered NOTICE
// line into a structured finding (rule, repository-relative path, row identity,
// subject) and compares THOSE, never the rendered line, so explanatory wording
// and a temporary absolute root cannot mint a "new" finding. The classifier is
// the only parser of rendered text in this file; the emitters that feed it are
// listed in ratchetActionable below and every one is exercised by
// TestNoticeRatchet.
//
// Inclusion set (the public rule-family inventory in
// docs/streams/statusgen/README.md, "A/B findings on changed briefs"):
//
//   - Family A, structural Verify rows (verifyrows.go, verifyportability.go).
//   - Family B, row strength (strength.go) and identifier dereference
//     (linkcheck.go).
//   - Family B, risk x files (riskfilescrossread.go) and the brief-file
//     rationale checks in checkBriefFiles: absent or substance-invalid why,
//     absent exec-tier-why.
//   - Family B, consumer routing (consumers.go), except the heuristic prompt.
//
// Explicitly NOT members, so they stay visible and can never become an
// authoring failure by accident:
//
//   - COULD-NOT-CHECK lines of any rule: an unknown is reported as unknown, it
//     is neither a finding nor a clean read.
//   - consumers-missing-list: a heuristic prompt, "never a verdict".
//   - consumer-delivery-range and the outcome / decision-issue / sign-off
//     metadata notices: not in the inventory's A or B rows.
//   - Family C (board ownership, merged delivery, attribution, register links)
//     and the ordering-gate suggestions: they belong to their existing loop
//     owners and are not attributable to one brief file.
//   - Historical and closed-record summaries, operational debt counters and
//     informational lines: they carry no brief path and no row identity.
//
// Adding a rule to the ratchet is an inventory amendment, not a code edit here.

// ratchetRowRules are the bracket tags of per-row findings that are actionable.
// They are spelled through the emitters' own constants so a rename breaks the
// build instead of silently dropping a rule from the gate.
var ratchetRowRules = map[string]bool{
	// Family A.
	ruleERELiteralPipe:       true,
	ruleGrepZeroCount:        true,
	ruleExitSwallowed:        true,
	ruleRE2LiteralPipe:       true,
	ruleMetavar:              true,
	ruleGoRunExit:            true,
	ruleBREAlternation:       true,
	ruleShreddedCell:         true,
	ruleMovingRef:            true,
	rulePortability:          true,
	ruleGoTestRunVacuous:     true,
	ruleProseLedCommand:      true,
	ruleCmdMarkerAmbiguous:   true,
	ruleCmdMarkerOverrides:   true,
	ruleCmdMarkerNotHonoured: true,
	ruleCmdMarkerVacuous:     true,
	ruleGrepPipeZeroCount:    true,
	"verify-row-portability": true, // verifyportability.go has no constant
	// Family B, row strength.
	ruleTriviallyGreen: true,
	ruleNoOutputAssert: true,
}

// ratchetBriefRules are the bracket tags of brief-level findings that carry a
// path and a subject (the identifier, or the declared path).
var ratchetBriefRules = map[string]bool{
	"id-dereference":       true, // linkcheck.go
	ruleRiskFilesCrossRead: true,
	ruleTableNoFiles:       true,
}

// ratchetUntagged are the actionable emitters whose NOTICE carries no bracket
// tag. Each is keyed by the registry identity the inventory names (consumers)
// or by the checkBriefFiles function plus its condition (the inventory's rule
// for an unregistered check). Order matters only for readability: the patterns
// do not overlap.
var ratchetUntagged = []struct {
	rule string
	re   *regexp.Regexp
}{
	{"consumers-prose", regexp.MustCompile(`^consumers: is a prose paragraph`)},
	{"consumers-no-verify-row", regexp.MustCompile(`^brief \S+ carries consumers: but no Verify row runs `)},
	{"consumers-unrouted", regexp.MustCompile(`^consumers entry ("(?:[^"\\]|\\.)*") names no routing`)},
	{"consumers-followup-no-target", regexp.MustCompile(`^consumers entry ("(?:[^"\\]|\\.)*") routes to a follow-up but names no`)},
	{"consumers-followup-one-way", regexp.MustCompile(`^consumers entry ("(?:[^"\\]|\\.)*") routes to follow-up \S+, but \S+ never references `)},
	{"consumers-out-of-scope-no-reason", regexp.MustCompile(`^consumers entry ("(?:[^"\\]|\\.)*") is routed out-of-scope with no substantive reason`)},
	{"checkBriefFiles/why-absent", regexp.MustCompile(`^brief \S+ has no why:`)},
	{"checkBriefFiles/why-substance", regexp.MustCompile(`^brief \S+ has a why: that fails the substance floor`)},
	{"checkBriefFiles/exec-tier-why-absent", regexp.MustCompile(`^brief \S+ has exec-tier: strong but no exec-tier-why`)},
}

var (
	// "<path>: Verify row <id> [<rule>] ..." (or "a Verify row" when unnumbered).
	// The id is the author's raw "#" cell, so it may be empty (verifyportability.go
	// renders an empty cell as "Verify row  [") or hold spaces and brackets:
	// matchRowLine takes the first bracket tag after it that names a row rule.
	ratchetRowLeadRe = regexp.MustCompile(`^(\S+\.md): (Verify row |a Verify row)`)
	ratchetRowTagRe  = regexp.MustCompile(` \[([A-Za-z0-9-]+)\]`) // a following space is checked by matchRowLine
	// "[<rule>] <path>: ..." (id-dereference, risk-files-crossread).
	ratchetLeadTagRe = regexp.MustCompile(`^\[([A-Za-z0-9-]+)\] (\S+\.md): (.*)$`)
	// "<path>: [<rule>] ..." (table-touches-no-files).
	ratchetPathTagRe = regexp.MustCompile(`^(\S+\.md): \[([A-Za-z0-9-]+)\] (.*)$`)
	// "<path>: <message>" (the untagged emitters).
	ratchetPlainRe = regexp.MustCompile(`^(\S+\.md): (.*)$`)

	// ratchetUnknownLead matches the emitter-owned START of a could-not-check
	// message of a member rule, after its tag and path have been consumed:
	// table-touches-no-files ("COULD-NOT-CHECK — ..."), id-dereference
	// ("identifier dereference COULD-NOT-CHECK: ...") and risk-files-crossread
	// ("brief <id> COULD-NOT-CHECK the risk x files cross-read ..."). It is
	// anchored at the start on purpose: the token is only an unknown when the
	// emitter put it there. The same token inside a quoted command, identifier or
	// declared path is brief-controlled text and must never hide a finding.
	ratchetUnknownLead = regexp.MustCompile(`^(?:COULD-NOT-CHECK|identifier dereference COULD-NOT-CHECK|brief \S+ COULD-NOT-CHECK)\b`)

	ratchetBacktickRe = regexp.MustCompile("`([^`]*)`")
	ratchetDeclaredRe = regexp.MustCompile(`declares path ("(?:[^"\\]|\\.)*")`)

	// fullObjectIDRe is a full SHA-1 or SHA-256 object id, as git merge-base prints.
	fullObjectIDRe = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
)

// lintFinding is the structured form of one actionable NOTICE. Equality of the
// (rule, row, subject) triple at one path is the whole comparison; the rendered
// wording is kept only for display.
type lintFinding struct {
	rule    string
	path    string // repository-relative, slash-separated
	row     string // Verify row id, "" for a brief-level finding
	subject string // discriminator for brief-level findings (identifier, declared path, consumers entry)
	raw     string // the original line, prefix stripped, for display
}

// identity is the comparison key WITHOUT the path, so a renamed brief can be
// compared at its base path.
func (f lintFinding) identity() string {
	return f.rule + "\x00" + f.row + "\x00" + f.subject
}

// relativizeNoticeLine strips the PROBLEM:/NOTICE: prefix and the tree's own
// root from one line, leaving repository-relative paths. Unlike a bare replace
// of the root string it leaves a "." root alone (replacing "." deletes every
// dot in the line, ".md" included).
func relativizeNoticeLine(line, treeRoot string) string {
	s := stripProblemPrefix(line)
	if abs, err := filepath.Abs(treeRoot); err == nil && abs != "" && abs != "/" {
		s = strings.ReplaceAll(s, abs+string(filepath.Separator), "")
	}
	if tr := filepath.Clean(treeRoot); tr != "." && tr != "" && tr != "/" {
		s = strings.ReplaceAll(s, tr+string(filepath.Separator), "")
	}
	s = strings.ReplaceAll(s, "./docs/", "docs/")
	return filepath.ToSlash(s)
}

func isBriefPath(p string) bool {
	b := filepath.Base(filepath.FromSlash(p))
	return strings.HasPrefix(b, "brief-") && strings.HasSuffix(b, ".md")
}

// parseLintNotice is the single classifier. ok is true only for an actionable
// finding on a brief file; every other line (a could-not-check, a heuristic
// prompt, a summary, a non-member rule, a notice with no brief path) returns
// false and stays a plain NOTICE.
//
// A could-not-check is recognised by the emitter-owned shape of the line
// (ratchetUnknownLead), never by the token anywhere in the line: the row rules
// echo the row's own Command cell, and a command that mentions the token is
// still a real finding. No member row rule has a could-not-check form of its
// own (verifyrows.go, strength.go and verifyportability.go emit none).
// matchRowLine splits a Verify-row line. tagged reports that the line is a
// row line carrying at least one bracket tag; rule is then the first tag that
// names an included row rule ("" when none does), row is the text before it
// (the raw "#" cell, empty for "a Verify row") and rest the message after it.
func matchRowLine(s string) (path, row, rule, rest string, tagged bool) {
	m := ratchetRowLeadRe.FindStringSubmatch(s)
	if m == nil {
		return "", "", "", "", false
	}
	tail := s[len(m[0]):]
	for _, ix := range ratchetRowTagRe.FindAllStringSubmatchIndex(tail, -1) {
		if ix[1] >= len(tail) || tail[ix[1]] != ' ' {
			continue
		}
		tagged = true
		id := tail[:ix[0]]
		if !ratchetRowRules[tail[ix[2]:ix[3]]] || (m[2] == "a Verify row" && id != "") {
			continue
		}
		return m[1], id, tail[ix[2]:ix[3]], tail[ix[1]+1:], true
	}
	return m[1], "", "", "", tagged
}

func parseLintNotice(line, treeRoot string) (lintFinding, bool) {
	s := strings.TrimSpace(relativizeNoticeLine(line, treeRoot))
	f := lintFinding{raw: s}
	if path, row, rule, rest, tagged := matchRowLine(s); tagged {
		if rule == "" || !isBriefPath(path) {
			return lintFinding{}, false
		}
		f.path, f.row, f.rule = path, row, rule
		if f.row == "" { // an unnumbered row: the first code span is its identity
			f.subject = firstBacktick(rest)
		}
		return f, true
	}
	if m := ratchetLeadTagRe.FindStringSubmatch(s); m != nil {
		if !ratchetBriefRules[m[1]] || !isBriefPath(m[2]) || ratchetUnknownLead.MatchString(m[3]) {
			return lintFinding{}, false
		}
		f.rule, f.path = m[1], m[2]
		switch m[1] {
		case "id-dereference":
			f.subject = firstBacktick(m[3])
		case ruleRiskFilesCrossRead:
			if d := ratchetDeclaredRe.FindStringSubmatch(m[3]); d != nil {
				f.subject = d[1]
			}
		}
		return f, true
	}
	if m := ratchetPathTagRe.FindStringSubmatch(s); m != nil {
		if !ratchetBriefRules[m[2]] || !isBriefPath(m[1]) || ratchetUnknownLead.MatchString(m[3]) {
			return lintFinding{}, false
		}
		f.path, f.rule = m[1], m[2]
		return f, true
	}
	if m := ratchetPlainRe.FindStringSubmatch(s); m != nil && isBriefPath(m[1]) {
		for _, u := range ratchetUntagged {
			if um := u.re.FindStringSubmatch(m[2]); um != nil {
				f.path, f.rule = m[1], u.rule
				if len(um) > 1 {
					f.subject = um[1]
				}
				return f, true
			}
		}
	}
	return lintFinding{}, false
}

func firstBacktick(s string) string {
	if m := ratchetBacktickRe.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

// noticeRatchetResult is the outcome of one root's notice comparison.
type noticeRatchetResult struct {
	introduced []string // original NOTICE lines promoted to PROBLEM
	carried    int      // actionable findings on changed briefs already present at the base (stay NOTICE)
	examined   int      // changed briefs the comparison covered
}

// evaluateNoticeRatchet compares the actionable findings of the changed briefs
// at head against the base. changed maps each head brief path to its base path
// ("" for an added brief). A head finding is carried when the same
// (rule, row, subject) is at the brief's base path, and introduced otherwise.
func evaluateNoticeRatchet(headNotices, baseNotices []string, headRoot, baseRoot string, changed map[string]string) noticeRatchetResult {
	res := noticeRatchetResult{}
	for p := range changed {
		if isBriefPath(p) {
			res.examined++
		}
	}
	baseSet := map[string]map[string]bool{} // base path -> identity set
	for _, n := range baseNotices {
		if f, ok := parseLintNotice(n, baseRoot); ok {
			if baseSet[f.path] == nil {
				baseSet[f.path] = map[string]bool{}
			}
			baseSet[f.path][f.identity()] = true
		}
	}
	seen := map[string]bool{}
	for _, n := range headNotices {
		f, ok := parseLintNotice(n, headRoot)
		if !ok {
			continue
		}
		basePath, isChanged := changed[f.path]
		if !isChanged {
			continue
		}
		key := f.path + "\x00" + f.identity()
		if seen[key] {
			continue
		}
		seen[key] = true
		if basePath != "" && baseSet[basePath][f.identity()] {
			res.carried++
			continue
		}
		res.introduced = append(res.introduced, n)
	}
	sort.Strings(res.introduced)
	return res
}

// productionChangedBriefs lists the brief files the working tree adds or edits
// relative to baseSHA, as head path -> base path ("" when added). Paths are
// relative to root. A rename carries its old path so debt that merely moved is
// not re-counted as new. Untracked brief files count as added.
func productionChangedBriefs(root, baseSHA string) (map[string]string, error) {
	// baseSHA is the resolved merge-base: a full object id. Anything else (an
	// option-shaped value above all) is refused before git parses it, so it can
	// never turn into a git flag and an empty, falsely clean change set.
	if !fullObjectIDRe.MatchString(baseSHA) {
		return nil, fmt.Errorf("base %q is not a full object id", baseSHA)
	}
	out, err := exec.Command("git", "-C", root, "diff", "--name-status", "-M", "-z", "--relative", "--no-color", baseSHA, "--", ".").Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-status %s: %w", short(baseSHA), err)
	}
	changed := map[string]string{}
	tok := bytes.Split(bytes.TrimRight(out, "\x00"), []byte{0})
	for i := 0; i < len(tok); {
		status := string(tok[i])
		i++
		if status == "" {
			continue
		}
		need := 1
		if status[0] == 'R' || status[0] == 'C' {
			need = 2
		}
		if i+need > len(tok) {
			return nil, fmt.Errorf("git diff --name-status: truncated record %q", status)
		}
		switch status[0] {
		case 'D':
		case 'R':
			if np := filepath.ToSlash(string(tok[i+1])); isBriefPath(np) {
				changed[np] = filepath.ToSlash(string(tok[i]))
			}
		case 'C':
			if np := filepath.ToSlash(string(tok[i+1])); isBriefPath(np) {
				changed[np] = ""
			}
		default:
			if p := filepath.ToSlash(string(tok[i])); isBriefPath(p) {
				if status[0] == 'A' {
					changed[p] = ""
				} else {
					changed[p] = p
				}
			}
		}
		i += need
	}
	un, err := exec.Command("git", "-C", root, "ls-files", "--others", "--exclude-standard", "-z").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files --others: %w", err)
	}
	for _, p := range bytes.Split(bytes.TrimRight(un, "\x00"), []byte{0}) {
		if sp := filepath.ToSlash(string(p)); isBriefPath(sp) {
			changed[sp] = ""
		}
	}
	return changed, nil
}
