package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	shallow, err := gitShallowOutput(root)
	if err != nil || strings.TrimSpace(string(shallow)) != "false" {
		return "", false
	}
	ref := ""
	mainID := ""
	for _, candidate := range []string{"refs/remotes/origin/main", "refs/heads/main"} {
		resolved, err, supported := commitReadOutput(ordinaryCommitRead(root, candidate), candidate)
		if !supported {
			err = exec.Command("git", "-C", root, "rev-parse", "--verify", candidate+"^{commit}").Run()
		}
		if err == nil {
			ref = candidate
			if supported {
				mainID = strings.TrimSpace(string(resolved))
			}
			break
		}
	}
	if ref == "" {
		return "", false
	}
	issues := append([]int(nil), bf.Issues...)
	for _, m := range fixIssueRe.FindAllStringSubmatch(bf.Body, -1) {
		n, _ := strconv.Atoi(m[1])
		issues = append(issues, n)
	}
	if len(bf.DeclaredEntriesRaw) == 0 && len(issues) == 0 {
		return "", false
	}
	// Ordinary native Git still owns these operations and their exact argv.
	// The existing run owner shares successful bytes only when the repository,
	// selected main, native config/reference namespace and environment match.
	// Config/includes and symbolic aliases are observed by Git, not re-parsed here.
	// An unsupported profile or failed optional observation leaves the original
	// native reads uncached; it never changes a checked/could-not-check outcome.
	context := ""
	formattingNative := true
	native := func(args ...string) ([]byte, error) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if context == "" || activeGitReads == nil || len(args) == 0 || args[0] != "log" {
			return cmd.Output()
		}
		bound, err := json.Marshal([]any{context, cmd.Args, cmd.Environ()})
		if err != nil {
			return cmd.Output()
		}
		key := fmt.Sprintf("%x", sha256.Sum256(bound))
		return activeGitReads.staleFailRead(key, cmd.Output)
	}
	if activeGitReads != nil && mainID != "" {
		absolute, pathErr := filepath.Abs(root)
		physical, physicalErr := filepath.EvalSymlinks(absolute)
		cwd, cwdErr := os.Getwd()
		config, configErr := native("config", "--null", "--list", "--show-origin", "--show-scope")
		refs, refsErr := native("for-each-ref", "--format=%(refname)%00%(objectname)%00%(symref)")
		if configErr == nil {
			formattingNative = strings.Contains(strings.ToLower(string(config)), "i18n.logoutputencoding") || strings.Contains(strings.ToLower(string(config)), "i18n.commitencoding")
		}
		if pathErr == nil && physicalErr == nil && cwdErr == nil && configErr == nil && refsErr == nil {
			bound, err := json.Marshal([]any{physical, cwd, ref, mainID, config, refs})
			if err == nil {
				context = fmt.Sprintf("%x", sha256.Sum256(bound))
			}
		}
	}
	var newest time.Time
	sha := ""
	read := func(extra []string, issue *regexp.Regexp) bool {
		// First-parent path history measures when the main tree changed, including
		// a merge whose feature-side repair predates the failed main run.
		args := []string{"log", "--first-parent", ref, "--format=%ct%x00%h%x00%B%x00%P%x00%H%x00%x1e"}
		args = append(args, extra...)
		out, err := native(args...)
		if err != nil {
			return false
		}
		for _, record := range strings.Split(string(out), "\x1e") {
			fields := strings.Split(strings.TrimSpace(record), "\x00")
			if len(fields) < 5 {
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
				messages, err := staleFailRangeMessages(root, parents[0], fields[1], fields[4], formattingNative, native)
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

// staleFailRangeMessages moves graph work, not log output formatting, into the
// canonical Git owner. The native summary has already selected the displayed
// abbreviation; a shadowing native ref preserves the original range command.
// All actual object/graph errors remain could-not-check, with no native retry.
func staleFailRangeMessages(root, base, tip, fullTip string, formattingNative bool, native func(...string) ([]byte, error)) ([]byte, error) {
	if formattingNative {
		return native("log", base+".."+tip, "--format=%B")
	}
	read := ordinaryGitCore(root)
	if read.err != nil {
		return nil, read.err
	}
	if read.native != "" {
		return native("log", base+".."+tip, "--format=%B")
	}
	why, err := read.repo.NativeAbbreviationReason(tip)
	if err != nil {
		return nil, err
	}
	if why != "" {
		return native("log", base+".."+tip, "--format=%B")
	}
	rows, err := read.repo.CommitRange(base, fullTip)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.NativeMessageReason != "" {
			return native("log", base+".."+tip, "--format=%B")
		}
	}
	var messages strings.Builder
	for _, row := range rows {
		messages.WriteString(row.Message)
		messages.WriteByte('\n')
	}
	return []byte(messages.String()), nil
}
