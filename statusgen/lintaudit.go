package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

// lint-audit (statusgen/01) — 30-day check-firing audit.
//
// --lint-audit replays `git log --since=30.days` over docs/streams/** and statusgen/**,
// sampling ONE commit per UTC day (newest commit of each day), runs --lint at each
// sample with the CURRENT binary, and tallies per-rule PROBLEM/NOTICE firing counts.
// A rule with 0 firings AND no referencing _test.go assertion is flagged
// COLD — retirement candidate. The audit only REPORTS — it never retires a rule
// itself (retirement is a human judgment; a cold rule may still be a correctly-quiet
// guardrail), and it never gates CI (advisory, a NOTICE at most).
//
// Rule attribution: emitted check messages already carry a stable [rule-tag] bracket
// token (e.g. "[gnu-only]", "[shredded-cell]", "[gorun-exit]"). ruleTagFor extracts
// it. A line with no bracket tag falls into a deterministic unattributed bucket so
// nothing is silently dropped. Rules that exist on the CURRENT tree but never fired
// in the window are seeded from one HEAD lint run so they still appear as 0-firing
// rows instead of vanishing.

type auditSample struct {
	sha  string
	date string // YYYY-MM-DD (UTC)
}

type lintAuditConfig struct {
	root        string
	logSampler  func(root string) ([]auditSample, error)
	worktreeFn  func(root, sha string) (dir string, cleanup func(), err error)
	lintRunner  func(treeDir, executable string) []string
	testGrepper func(pkgDir, rule string) bool
}

// ruleTagFor maps one emitted PROBLEM/NOTICE line to its stable rule tag.
func ruleTagFor(line string) string {
	t := line
	for _, p := range []string{"PROBLEM:", "NOTICE:"} {
		t = strings.TrimSpace(strings.TrimPrefix(t, p))
	}
	if i := strings.IndexByte(t, '['); i >= 0 {
		if j := strings.IndexByte(t[i+1:], ']'); j >= 0 {
			if tag := strings.TrimSpace(t[i+1 : i+1+j]); tag != "" {
				return tag
			}
		}
	}
	// Untagged lines usually open with the file path they diagnose
	// (e.g. "/root/docs/streams/x/README.md: message", which differs per sampled
	// worktree). Strip a leading single path token — no spaces or commas,
	// followed by ": " — so one logical rule keys one bucket across samples.
	if strings.HasPrefix(t, "/") {
		if i := strings.Index(t, ": "); i > 0 && i < 160 && !strings.ContainsAny(t[:i], " ,\t") {
			t = t[i+2:]
		}
	}
	return "unattributed:" + sanitizeTag(t)
}

// sanitizeTag lowercases and collapses non-alnum runs to single dashes, capped at 48
// chars — a deterministic bucket key for unattributed lines.
func sanitizeTag(s string) string {
	s = strings.ToLower(s)
	if len(s) > 48 {
		s = s[:48]
	}
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// runLintAudit is the --lint-audit entry point (production config).
func runLintAudit(root string) int {
	cfg := lintAuditConfig{
		root:        root,
		logSampler:  productionLogSampler,
		worktreeFn:  productionWorktree,
		lintRunner:  productionLintRunner,
		testGrepper: productionTestGrepper,
	}
	out, err := lintAuditOutput(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: lint-audit could-not-check:", err)
		return 6
	}
	fmt.Print(out)
	return 0
}

func lintAuditReport(cfg lintAuditConfig) string {
	out, _ := lintAuditOutput(cfg)
	return out
}

func lintAuditOutput(cfg lintAuditConfig) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		exe = "statusgen"
	}
	firings := map[string]int{}
	registry := map[string]bool{}

	// HEAD seed: register every rule alive on the current tree so a rule that
	// never fired in the window still appears as a 0-firing row.
	for _, l := range cfg.lintRunner(cfg.root, exe) {
		registry[ruleTagFor(l)] = true
	}

	samples, err := cfg.logSampler(cfg.root)
	if err != nil {
		return "", fmt.Errorf("git log sampler: %v", err)
	}
	if len(samples) == 0 {
		return "no sampled commits in the 30-day window (git log --since=30.days over docs/streams and statusgen/)\n", nil
	}

	var notes strings.Builder
	for _, s := range samples {
		dir, cleanup, err := cfg.worktreeFn(cfg.root, s.sha)
		if err != nil {
			fmt.Fprintf(&notes, "# sample %s (%s): could-not-check worktree: %v\n", s.date, s.sha, err)
			continue
		}
		for _, l := range cfg.lintRunner(dir, exe) {
			tag := ruleTagFor(l)
			registry[tag] = true
			firings[tag]++
		}
		cleanup()
	}

	type row struct {
		rule      string
		firings   int
		gatesTest bool
		cold      bool
	}
	var rows []row
	for rule := range registry {
		gates := cfg.testGrepper(filepath.Join(cfg.root, "statusgen"), rule)
		rows = append(rows, row{rule: rule, firings: firings[rule], gatesTest: gates, cold: firings[rule] == 0 && !gates})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].firings != rows[j].firings {
			return rows[i].firings < rows[j].firings
		}
		return rows[i].rule < rows[j].rule
	})

	var out strings.Builder
	fmt.Fprintf(&out, "lint-audit: %d sampled commits over 30 days\n", len(samples))
	out.WriteString("rule | 30-day firings | gates-a-test?\n")
	for _, r := range rows {
		gt := "no"
		if r.gatesTest {
			gt = "yes"
		}
		line := fmt.Sprintf("%s | %d | %s", r.rule, r.firings, gt)
		if r.cold {
			line += "  COLD — retirement candidate"
		}
		out.WriteString(line + "\n")
	}
	if notes.Len() > 0 {
		out.WriteString(notes.String())
	}
	return out.String(), nil
}

// productionLogSampler: newest commit per UTC day over the last 30 days, restricted
// to the paths the checks actually read (docs/streams and statusgen/).
func productionLogSampler(root string) ([]auditSample, error) {
	cmd := exec.Command("git", "-C", root, "log", "--since=30.days", "--format=%H %ct", "--", "docs/streams", "statusgen")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var samples []auditSample
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		var unix int64
		if _, err := fmt.Sscanf(fields[1], "%d", &unix); err != nil {
			continue
		}
		date := time.Unix(unix, 0).UTC().Format("2006-01-02")
		if seen[date] {
			continue // git log is newest-first: the first commit of a day is its newest
		}
		seen[date] = true
		samples = append(samples, auditSample{sha: fields[0], date: date})
	}
	return samples, nil
}

// productionWorktree materializes one sampled commit as a detached worktree so
// --lint can run against a full tree (git-dependent checks included), then removes it.
func productionWorktree(root, sha string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "statusgen-lint-audit-")
	if err != nil {
		return "", nil, err
	}
	cmd := exec.Command("git", "-C", root, "worktree", "add", "--detach", "--quiet", dir, sha)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		return "", nil, fmt.Errorf("worktree add %s: %v: %s", sha, err, strings.TrimSpace(string(out)))
	}
	cleanup := func() {
		exec.Command("git", "-C", root, "worktree", "remove", "--force", "--quiet", dir).Run()
		os.RemoveAll(dir)
	}
	return dir, cleanup, nil
}

// productionLintRunner runs the current binary's --lint against one tree and returns
// only its PROBLEM/NOTICE lines; the lint exit code is deliberately ignored (the audit
// tallies firings, and a sample tree may legitimately be red on unrelated checks).
func productionLintRunner(treeDir, executable string) []string {
	cmd := exec.Command(executable, "--root", treeDir, "--lint")
	out, _ := cmd.CombinedOutput()
	var lines []string
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, "PROBLEM:") || strings.HasPrefix(l, "NOTICE:") {
			lines = append(lines, l)
		}
	}
	return lines
}

// productionTestGrepper: does any statusgen _test.go reference the rule tag?
func productionTestGrepper(pkgDir, rule string) bool {
	matches, err := filepath.Glob(filepath.Join(pkgDir, "*_test.go"))
	if err != nil {
		return false
	}
	for _, f := range matches {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if strings.Contains(string(b), rule) {
			return true
		}
	}
	return false
}
