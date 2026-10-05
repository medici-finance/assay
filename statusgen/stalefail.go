package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var failHeadingRe = regexp.MustCompile(`^#{3,6}[ \t]`)
var failDateRe = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`)
var fixIssueRe = regexp.MustCompile(`(?i)\b(?:fix(?:es|ed)?|repair(?:s|ed)?)(?:\s+issue)?\s+#([0-9]+)\b`)

// cleanFailEvidence shares the verdict reader's quotation exclusions. Dates and
// strict PASS markers must not be borrowed from examples or retracted records.
func cleanFailEvidence(evidence string) string {
	evidence, _ = stripRowComments(stripFences(evidence))
	var lines []string
	for _, line := range strings.Split(evidence, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), ">") {
			lines = append(lines, strikethroughRe.ReplaceAllString(line, ""))
		}
	}
	return strings.Join(lines, "\n")
}

// failEvidenceDate uses only the last run: a new heading or verdict bounds its
// date search. Day-only Evidence cannot order two events within the same day,
// so only commits on a later UTC day qualify as stale.
func failEvidenceDate(evidence string) (time.Time, bool) {
	lines := strings.Split(evidence, "\n")
	last := -1
	for i, line := range lines {
		if verifyVerdictRe.MatchString(line) {
			last = i
		}
	}
	if last < 0 {
		return time.Time{}, false
	}
	start, end := last, len(lines)
	for start > 0 && !failHeadingRe.MatchString(strings.TrimSpace(lines[start])) {
		previous := lines[start-1]
		// A heading begins THIS run even when it repeats its closing verdict.
		// Include its date, but never cross it into an earlier run.
		if failHeadingRe.MatchString(strings.TrimSpace(previous)) {
			start--
			break
		}
		if verifyVerdictRe.MatchString(previous) {
			break
		}
		start--
	}
	for i := last + 1; i < len(lines); i++ {
		if failHeadingRe.MatchString(strings.TrimSpace(lines[i])) {
			end = i
			break
		}
	}
	var date time.Time
	for _, s := range failDateRe.FindAllString(strings.Join(lines[start:end], "\n"), -1) {
		d, err := time.Parse("2006-01-02", s)
		if err == nil && d.After(date) {
			date = d
		}
	}
	return date, !date.IsZero()
}

// newestFailFix reads only locally available main history, never HEAD or a
// network endpoint. An absent/shallow history cannot prove a FAIL current.
func newestFailFix(root string, bf *BriefFile, date time.Time) (string, bool) {
	shallow, err := exec.Command("git", "-C", root, "rev-parse", "--is-shallow-repository").Output()
	if err != nil || strings.TrimSpace(string(shallow)) != "false" {
		return "", false
	}
	ref := ""
	for _, candidate := range []string{"refs/remotes/origin/main", "refs/heads/main"} {
		if exec.Command("git", "-C", root, "rev-parse", "--verify", candidate+"^{commit}").Run() == nil {
			ref = candidate
			break
		}
	}
	if ref == "" {
		return "", false
	}
	var newest time.Time
	sha := ""
	read := func(extra []string, issue *regexp.Regexp) bool {
		// First-parent path history measures when the main tree changed, including
		// a merge whose feature-side repair predates the failed main run.
		args := []string{"-C", root, "log", "--first-parent", ref, "--format=%ct%x00%h%x00%B%x00%P%x00%x1e"}
		args = append(args, extra...)
		out, err := exec.Command("git", args...).Output()
		if err != nil {
			return false
		}
		for _, record := range strings.Split(string(out), "\x1e") {
			fields := strings.Split(strings.TrimSpace(record), "\x00")
			if len(fields) < 4 {
				continue
			}
			sec, err := strconv.ParseInt(fields[0], 10, 64)
			if err != nil {
				return false
			}
			when := time.Unix(sec, 0).UTC()
			if when.Before(date.AddDate(0, 0, 1)) || !when.After(newest) {
				continue
			}
			if issue != nil && !issue.MatchString(fields[2]) {
				parents := strings.Fields(fields[3])
				if len(parents) < 2 {
					continue
				}
				// A merge need not repeat the issue named by the repair it
				// introduces. Attribute those newly reachable issue references
				// to the landing commit, without consulting unmerged branches.
				messages, err := exec.Command("git", "-C", root, "log", parents[0]+".."+fields[1], "--format=%B").Output()
				if err != nil {
					return false
				}
				if !issue.Match(messages) {
					continue
				}
			}
			newest = when
			sha = fields[1]
		}
		return true
	}
	checked := false
	if len(bf.DeclaredEntriesRaw) > 0 {
		if !read(append([]string{"--"}, bf.DeclaredEntriesRaw...), nil) {
			return "", false
		}
		checked = true
	}
	issues := append([]int(nil), bf.Issues...)
	for _, m := range fixIssueRe.FindAllStringSubmatch(bf.Body, -1) {
		n, _ := strconv.Atoi(m[1])
		issues = append(issues, n)
	}
	if len(issues) > 0 {
		var ids []string
		for _, n := range issues {
			ids = append(ids, strconv.Itoa(n))
		}
		issue := regexp.MustCompile(`#(?:` + strings.Join(ids, "|") + `)(?:\b)`)
		if !read(nil, issue) {
			return "", false
		}
		checked = true
	}
	return sha, checked
}

// waitingBriefNotice is the single routing point for all waiting gate types.
// It changes advice only; no history observation certifies a passing Verify run.
func waitingBriefNotice(root, path string, bf *BriefFile, status string) string {
	if status != "in-progress" && status != "implemented" && status != "verified" {
		return ""
	}
	fallback := ""
	if bf.Gate == "human" && bf.DecisionIssue == 0 {
		fallback = fmt.Sprintf("%s: brief %s is gate:human at %s but has no decision-issue — file one via --decision-issues", path, bf.Brief, status)
	}
	evidence := cleanFailEvidence(bf.Evidence)
	if lastVerifyVerdict(evidence) == verdictFail {
		date, ok := failEvidenceDate(evidence)
		sha := ""
		if ok {
			sha, ok = newestFailFix(root, bf, date)
		}
		if !ok {
			return strings.TrimSpace(fallback + "\n" + fmt.Sprintf("%s: could-not-check stale FAIL — Evidence date or main history unavailable; verify-desk must assess and arrange independent re-verification", path))
		}
		if sha != "" {
			return fmt.Sprintf("%s: brief %s has stale FAIL — newest relevant main commit %s postdates Evidence %s; a non-implementer RE-VERIFY supersedes it", path, bf.Brief, sha, date.Format("2006-01-02"))
		}
	} else if fallback != "" && lastVerifyVerdict(evidence) == verdictPass && hasVerifyPass(evidence) && !verdictFailAfterStrictPass(evidence) {
		return fmt.Sprintf("%s: brief %s sign-off card missing — verify-desk lands the marker / verify-gate-open files the card", path, bf.Brief)
	}
	return fallback
}
