package main

// Code-efficiency metrics emitter (`statusgen --code`, methodology-metrics/19).
//
// Emits five code-efficiency metrics from ledger artifacts (git + issue register):
//   - SLOC delta/day: added / removed / net per day
//   - Churn ratio: lines touched again within N days ÷ lines added
//   - Defect density: bug issues ÷ KSLOC-changed
//   - Change spread: median files per change
//   - Review depth: PR review comments ÷ merged PR
//
// F-12 posture (stated in the output header): every figure here is recomputed from
// repo ledger artifacts (git history, the issue register) — which is the F-12
// EXEMPTION. The header says so, so a consumer knows these are quotable where a
// leverage estimate is not. No leverage/person-day/tier-mix figure is emitted.
//
// Composes with --series: --code --series buckets per ISO week (mm/16). Same
// Goodhart header as --dora (diagnostic, never a target); small-n honesty (a
// window with <N commits prints "–" rather than a misleading ratio).
// NOT wired into the offline --lint gate (diagnostic emitter; may read gh).

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// codeAntiGamingNote is printed in --code's header (text) and carried in the
// JSON `note` field. Same discipline as --dora: diagnostic, per-project, never a
// target, an individual scorecard, or a cross-team comparison.
const codeAntiGamingNote = "Code-efficiency metrics are DIAGNOSTIC, per-project, for continuous " +
	"improvement — never a target, an individual scorecard, or a cross-team comparison " +
	"(Goodhart's law: a measure that becomes a target ceases to be a good measure)."

// codeF12Posture is the F-12 exemption header, printed in every --code output.
// It states the ledger provenance that makes these figures quotable.
const codeF12Posture = "F-12 posture: every figure is recomputed from repo ledger artifacts " +
	"(git history, the issue register) — quotable where a leverage estimate is not. " +
	"No leverage, person-day, or tier-mix figure is emitted."

// Defaults
const (
	defaultChurnDays      = 7
	codeSmallNSuppress    = 3 // minimum commits/PRs for a metric to be reported
	defaultCodeWindowDays = 28
)

// Canonical code-efficiency metric keys (stable identifiers for downstream consumers).
const (
	codeSLOCDelta     = "sloc_delta_per_day"
	codeChurnRatio    = "churn_ratio"
	codeDefectDensity = "defect_density"
	codeChangeSpread  = "change_spread"
	codeReviewDepth   = "review_depth"
)

// Ordered groups for text rendering.
var codeMetricKeys = []string{codeSLOCDelta, codeChurnRatio, codeDefectDensity, codeChangeSpread, codeReviewDepth}

// CodeMetric is one of the five code-efficiency metrics.
type CodeMetric struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	Computed bool   `json:"computed"`
	Detail   string `json:"detail,omitempty"`
}

// CodeReport is the full emitted system.
type CodeReport struct {
	Since      string                `json:"since"`
	Until      string                `json:"until"`
	Generated  string                `json:"generated"`
	Note       string                `json:"note"`
	F12Posture string                `json:"f12_posture"`
	Metrics    map[string]CodeMetric `json:"metrics"`
}

// CodeSeriesPoint is one period bucket in the --series output.
type CodeSeriesPoint struct {
	Period        string `json:"period"`
	Added         int    `json:"added"`
	Removed       int    `json:"removed"`
	Net           int    `json:"net"`
	FilesTouched  int    `json:"files_touched"`
	ChurnLines    int    `json:"churn_lines"`
	AddedLines    int    `json:"added_lines"`
	ChurnRatio    string `json:"churn_ratio"`
	BugIssues     int    `json:"bug_issues"`
	DefectDensity string `json:"defect_density"`
	ReviewDepth   string `json:"review_depth"`
	NumPRs        int    `json:"num_prs"`
	NumCommits    int    `json:"num_commits"`
}

// codeCommit is a single parsed non-merge, non-regen commit.
type codeCommit struct {
	Date     time.Time
	Added    int
	Removed  int
	Files    int
	FileList []string // paths touched in this commit
}

// codePR is merged PR info for review-depth computation.
type codePR struct {
	Number         int
	MergedAt       time.Time
	ReviewComments int
}

// codeInputs is the pure input to computeCodeEfficiency: everything gathered
// from git + gh. Kept as data so the metric math is testable over fixtures.
type codeInputs struct {
	Since, Until, Now time.Time
	Commits           []codeCommit
	GitOK             bool
	BugIssues         int
	GHBugsOK          bool
	MergedPRs         []codePR
	GHMergedOK        bool
	ChurnDays         int
}

// --- data gathering (exec/network — overridable in tests) -------------------

// codeGitStats runs git log --numstat for non-merge, non-regen commits in
// [since, until]. Returns parsed commit stats. ok=false on any git failure.
var codeGitStats = func(root string, since, until time.Time) ([]codeCommit, bool) {
	// Use git log with --numstat to get per-commit added/removed/file counts.
	// --no-merges excludes merge commits. We also filter "--grep" for regen.
	// Format: %H commit hash, %aI author date ISO8601, %s subject
	out, err := exec.Command("git", "-C", root, "log",
		"--no-merges",
		"--since="+since.Format(time.RFC3339),
		"--until="+until.Format(time.RFC3339),
		"--format=format:%x00%H %aI %s",
		"--numstat",
	).Output()
	if err != nil {
		return nil, false
	}

	var commits []codeCommit
	lines := strings.Split(string(out), "\n")
	var cur *codeCommit

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Commit marker: starts with NUL (0x00)
		if len(line) > 0 && line[0] == 0 {
			// Save previous commit if any
			if cur != nil {
				commits = append(commits, *cur)
			}
			fields := strings.Fields(line[1:]) // skip the 0x00 prefix
			if len(fields) < 3 {
				cur = nil
				continue
			}
			// fields[0] = hash, fields[1] = date, fields[2:] = subject
			date, err := time.Parse(time.RFC3339, fields[1])
			if err != nil {
				cur = nil
				continue
			}
			// Skip regen commits ([skip-status-regen])
			subject := strings.Join(fields[2:], " ")
			if strings.Contains(subject, "[skip-status-regen]") {
				cur = nil
				continue
			}
			cur = &codeCommit{Date: date.UTC()}
			continue
		}

		// numstat line: added\tremoved\tfilename
		if cur == nil || line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		added, _ := strconv.Atoi(parts[0])
		removed := 0
		if parts[1] != "-" {
			removed, _ = strconv.Atoi(parts[1])
		}
		cur.Added += added
		cur.Removed += removed
		cur.Files++
		// Track file path for churn (parts[2] if present, otherwise parts[0])
		if len(parts) >= 3 && parts[2] != "" {
			cur.FileList = append(cur.FileList, parts[2])
		}
	}

	// Save last commit
	if cur != nil {
		commits = append(commits, *cur)
	}

	return commits, true
}

// codeMergedPRs lists merged PRs via gh, filtered to those merged in the window.
// ok=false when gh is missing/unauthenticated/errors.
var codeMergedPRs = func(root string, since, until time.Time) ([]codePR, bool) {
	out, err := exec.Command("gh", "pr", "list", "--state", "merged",
		"--limit", "500", "--json", "number,mergedAt").Output()
	if err != nil {
		return nil, false
	}
	var raw []struct {
		Number   int       `json:"number"`
		MergedAt time.Time `json:"mergedAt"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, false
	}
	var prs []codePR
	for _, p := range raw {
		if p.MergedAt.IsZero() || p.MergedAt.Before(since) || p.MergedAt.After(until) {
			continue
		}
		prs = append(prs, codePR{Number: p.Number, MergedAt: p.MergedAt})
	}
	return prs, true
}

// codeBugIssues reuses doraBugIssues for defect density.
var codeBugIssues = doraBugIssues

// --- computation -----------------------------------------------------------

// computeCodeChurn computes churn: lines added in commits where at least one
// file was previously touched within churnDays. For each commit (chronological),
// check if any of its files appear in any earlier commit within the churnDays
// window. If so, this commit's added lines are churn.
//
// Approximation: a multi-file commit counts ALL its added lines as churn if ANY
// file was recently touched. The true per-file churn count would be lower.
func computeCodeChurn(commits []codeCommit, churnDays int) (churnLines, totalAdded int) {
	if churnDays <= 0 {
		churnDays = defaultChurnDays
	}

	// Sort by date
	sorted := make([]codeCommit, len(commits))
	copy(sorted, commits)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Date.Before(sorted[j].Date) })

	// Track per-file last-touch dates
	type fileTouch struct {
		date  time.Time
		added int
	}
	lastTouch := map[string]time.Time{}

	for i := range sorted {
		c := &sorted[i]
		hasRecentTouch := false
		for _, f := range c.FileList {
			if lt, ok := lastTouch[f]; ok {
				if c.Date.Sub(lt) <= time.Duration(churnDays)*24*time.Hour {
					hasRecentTouch = true
					break
				}
			}
		}
		totalAdded += c.Added
		if hasRecentTouch {
			churnLines += c.Added
		}
		// Update last-touch for all files
		for _, f := range c.FileList {
			lastTouch[f] = c.Date
		}
	}
	return
}

// medianFilesPerCommit returns the median files-touched count across commits.
func medianFilesPerCommit(commits []codeCommit) int {
	if len(commits) == 0 {
		return 0
	}
	counts := make([]int, len(commits))
	for i, c := range commits {
		counts[i] = c.Files
	}
	sort.Ints(counts)
	return counts[len(counts)/2]
}

// computeCodeEfficiency is the pure metric computation: codeInputs -> CodeReport.
// No exec, no network, no clock — deterministic and testable.
func computeCodeEfficiency(in codeInputs) CodeReport {
	rep := CodeReport{
		Since:      in.Since.Format("2006-01-02"),
		Until:      in.Until.Format("2006-01-02"),
		Generated:  in.Now.UTC().Format(time.RFC3339),
		Note:       codeAntiGamingNote,
		F12Posture: codeF12Posture,
		Metrics:    map[string]CodeMetric{},
	}

	days := in.Until.Sub(in.Since).Hours() / 24
	if days < 1 {
		days = 1
	}

	nCommits := len(in.Commits)
	// Total added/removed/net across all commits
	totalAdded, totalRemoved := 0, 0
	for _, c := range in.Commits {
		totalAdded += c.Added
		totalRemoved += c.Removed
	}

	// 1. SLOC delta/day
	sd := CodeMetric{Key: codeSLOCDelta, Name: "SLOC delta/day"}
	if in.GitOK && nCommits >= codeSmallNSuppress {
		sd.Computed = true
		sd.Value = fmt.Sprintf("+%.1f / -%.1f / net %+.1f per day",
			float64(totalAdded)/days, float64(totalRemoved)/days, float64(totalAdded-totalRemoved)/days)
		sd.Detail = fmt.Sprintf("%d added, %d removed, net %+d over %.0f day(s) in %d non-merge non-regen commit(s)",
			totalAdded, totalRemoved, totalAdded-totalRemoved, days, nCommits)
	} else {
		sd.Value = "–"
		sd.Detail = fmt.Sprintf("insufficient data: %d commit(s) in window (need ≥%d)", nCommits, codeSmallNSuppress)
	}
	rep.Metrics[codeSLOCDelta] = sd

	// 2. Churn ratio
	cr := CodeMetric{Key: codeChurnRatio, Name: "Churn ratio"}
	churnDays := in.ChurnDays
	if churnDays <= 0 {
		churnDays = defaultChurnDays
	}
	churnLines, churnTotal := computeCodeChurn(in.Commits, churnDays)
	if in.GitOK && nCommits >= codeSmallNSuppress && churnTotal > 0 {
		cr.Computed = true
		ratio := float64(churnLines) / float64(churnTotal)
		cr.Value = fmt.Sprintf("%.0f%%", ratio*100)
		cr.Detail = fmt.Sprintf("%d churn lines (re-touched within %dd) ÷ %d total added lines in %d non-merge non-regen commit(s)",
			churnLines, churnDays, churnTotal, nCommits)
	} else {
		cr.Value = "–"
		cr.Detail = fmt.Sprintf("insufficient data: %d commit(s), %d added lines (need ≥%d commits, >0 added lines)",
			nCommits, churnTotal, codeSmallNSuppress)
	}
	rep.Metrics[codeChurnRatio] = cr

	// 3. Defect density: bugs ÷ KSLOC changed
	dd := CodeMetric{Key: codeDefectDensity, Name: "Defect density"}
	totalChanged := totalAdded + totalRemoved
	if in.GHBugsOK && in.GitOK && totalChanged > 0 && nCommits >= codeSmallNSuppress {
		dd.Computed = true
		dd.Value = fmt.Sprintf("%.2f bugs/KSLOC", float64(in.BugIssues)/(float64(totalChanged)/1000.0))
		dd.Detail = fmt.Sprintf("%d bug issue(s) ÷ %.1f KSLOC changed (%d added + %d removed) in %d non-merge non-regen commit(s)",
			in.BugIssues, float64(totalChanged)/1000.0, totalAdded, totalRemoved, nCommits)
	} else {
		dd.Value = "–"
		dd.Detail = fmt.Sprintf("insufficient data: %d bug(s), %d lines changed, %d commit(s)",
			in.BugIssues, totalChanged, nCommits)
	}
	rep.Metrics[codeDefectDensity] = dd

	// 4. Change spread: median files per change
	cs := CodeMetric{Key: codeChangeSpread, Name: "Change spread"}
	if in.GitOK && nCommits >= codeSmallNSuppress {
		cs.Computed = true
		med := medianFilesPerCommit(in.Commits)
		cs.Value = fmt.Sprintf("%d files/change (median)", med)
		cs.Detail = fmt.Sprintf("median files touched per change over %d non-merge non-regen commit(s)", nCommits)
	} else {
		cs.Value = "–"
		cs.Detail = fmt.Sprintf("insufficient data: %d commit(s) (need ≥%d)", nCommits, codeSmallNSuppress)
	}
	rep.Metrics[codeChangeSpread] = cs

	// 5. Review depth: review comments ÷ merged PR
	rd := CodeMetric{Key: codeReviewDepth, Name: "Review depth"}
	totalComments := 0
	nMerged := 0
	if in.GHMergedOK {
		nMerged = len(in.MergedPRs)
		for _, p := range in.MergedPRs {
			totalComments += p.ReviewComments
		}
	}
	if in.GHMergedOK && nMerged >= codeSmallNSuppress {
		rd.Computed = true
		rd.Value = fmt.Sprintf("%.1f comments/PR", float64(totalComments)/float64(nMerged))
		rd.Detail = fmt.Sprintf("%d total review comment(s) ÷ %d merged PR(s)", totalComments, nMerged)
	} else {
		rd.Value = "–"
		rd.Detail = fmt.Sprintf("insufficient data: %d merged PR(s) (need ≥%d)", nMerged, codeSmallNSuppress)
	}
	rep.Metrics[codeReviewDepth] = rd

	return rep
}

// --- text rendering --------------------------------------------------------

func renderCodeText(rep CodeReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Code-efficiency metrics — %s … %s\n", rep.Since, rep.Until)
	fmt.Fprintf(&b, "%s\n", rep.F12Posture)
	fmt.Fprintf(&b, "%s\n\n", rep.Note)
	for _, k := range codeMetricKeys {
		m := rep.Metrics[k]
		fmt.Fprintf(&b, "  %-20s %s\n", m.Name, m.Value)
		if m.Detail != "" {
			fmt.Fprintf(&b, "      %s\n", m.Detail)
		}
	}
	return b.String()
}

// --- series computation ----------------------------------------------------

// computeCodeSeries buckets code-efficiency data per period (ISO week or day).
// Reuses trend.go's bucketStart/periodStep.
func computeCodeSeries(
	since, until time.Time,
	period string,
	churnDays int,
	commits []codeCommit,
	bugIssueDates []time.Time,
	mergedPRs []codePR,
) []CodeSeriesPoint {
	firstStart := bucketStart(since, period)
	lastStart := bucketStart(until, period)

	if firstStart.After(lastStart) {
		return nil
	}

	if churnDays <= 0 {
		churnDays = defaultChurnDays
	}

	// Index data by bucket start.
	type bucketData struct {
		commits       []codeCommit
		bugIssues     int
		prs           []codePR
		totalComments int
	}
	buckets := map[time.Time]*bucketData{}
	for bs := firstStart; !bs.After(lastStart); bs = periodStep(bs, period) {
		buckets[bs] = &bucketData{}
	}

	for _, c := range commits {
		bs := bucketStart(c.Date, period)
		if bd, ok := buckets[bs]; ok {
			bd.commits = append(bd.commits, c)
		}
	}
	for _, t := range bugIssueDates {
		bs := bucketStart(t, period)
		if bd, ok := buckets[bs]; ok {
			bd.bugIssues++
		}
	}
	for _, p := range mergedPRs {
		bs := bucketStart(p.MergedAt, period)
		if bd, ok := buckets[bs]; ok {
			bd.prs = append(bd.prs, p)
			bd.totalComments += p.ReviewComments
		}
	}

	var points []CodeSeriesPoint
	for bs := firstStart; !bs.After(lastStart); bs = periodStep(bs, period) {
		bd := buckets[bs]
		nCommits := len(bd.commits)
		nPRs := len(bd.prs)

		added, removed, files := 0, 0, 0
		for _, c := range bd.commits {
			added += c.Added
			removed += c.Removed
			files += c.Files
		}

		churnLines, totalAdded := computeCodeChurn(bd.commits, churnDays)
		chr := "–"
		if nCommits >= codeSmallNSuppress && totalAdded > 0 {
			chr = fmt.Sprintf("%.0f%%", float64(churnLines)/float64(totalAdded)*100)
		}

		totalChanged := added + removed
		dd := "–"
		if bd.bugIssues > 0 && totalChanged > 0 && nCommits >= codeSmallNSuppress {
			dd = fmt.Sprintf("%.2f bugs/KSLOC", float64(bd.bugIssues)/(float64(totalChanged)/1000.0))
		}

		rd := "–"
		if nPRs >= codeSmallNSuppress {
			rd = fmt.Sprintf("%.1f comments/PR", float64(bd.totalComments)/float64(nPRs))
		}

		points = append(points, CodeSeriesPoint{
			Period:        bs.Format("2006-01-02"),
			Added:         added,
			Removed:       removed,
			Net:           added - removed,
			FilesTouched:  files,
			ChurnLines:    churnLines,
			AddedLines:    totalAdded,
			ChurnRatio:    chr,
			BugIssues:     bd.bugIssues,
			DefectDensity: dd,
			ReviewDepth:   rd,
			NumPRs:        nPRs,
			NumCommits:    nCommits,
		})
	}

	return points
}

// --- rendering -------------------------------------------------------------

func renderCodeSeriesText(points []CodeSeriesPoint, since, until time.Time, period string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Code-efficiency time series — %s … %s (%s)\n", since.Format("2006-01-02"), until.Format("2006-01-02"), period)
	fmt.Fprintf(&b, "%s\n", codeF12Posture)
	fmt.Fprintf(&b, "%s\n\n", codeAntiGamingNote)

	// Header
	fmt.Fprintf(&b, "%-12s %6s %7s %7s %6s %7s %12s %11s\n",
		"period", "added", "removed", "net", "files", "churn", "defects", "review")
	fmt.Fprintf(&b, "%-12s %6s %7s %7s %6s %7s %12s %11s\n",
		"", "", "", "", "", "ratio", "/KSLOC", "depth")

	// Data rows
	for _, p := range points {
		fmt.Fprintf(&b, "%-12s %6d %7d %+7d %6d %7s %12s %11s\n",
			p.Period, p.Added, p.Removed, p.Net, p.FilesTouched,
			p.ChurnRatio, p.DefectDensity, p.ReviewDepth)
	}

	return b.String()
}

func renderCodeSeriesJSON(points []CodeSeriesPoint) string {
	if points == nil {
		points = []CodeSeriesPoint{}
	}
	out, err := json.MarshalIndent(points, "", "  ")
	if err != nil {
		return "[]"
	}
	return string(out)
}

// --- series data gathering -------------------------------------------------

// codeGitCommitDates returns dates of non-merge, non-regen commits in [since, until].
var codeGitCommitDates = func(root string, since, until time.Time) ([]time.Time, bool) {
	out, err := exec.Command("git", "-C", root, "log",
		"--no-merges",
		"--since="+since.Format(time.RFC3339),
		"--until="+until.Format(time.RFC3339),
		"--format=%aI").Output()
	if err != nil {
		return nil, false
	}
	var dates []time.Time
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		// Skip regen commits by checking message (approximate: if the line is a commit hash, skip)
		// Actually we can't check message from --format=%aI alone. Keep simple: git log
		// returns author dates only. The caller should filter regen separately.
		t, err := time.Parse(time.RFC3339, l)
		if err != nil {
			continue
		}
		dates = append(dates, t.UTC())
	}
	return dates, true
}

// gatherCodeInputs assembles codeInputs from git + gh.
func gatherCodeInputs(root string, since, until, now time.Time, churnDays int) codeInputs {
	in := codeInputs{Since: since, Until: until, Now: now, ChurnDays: churnDays}
	in.Commits, in.GitOK = codeGitStats(root, since, until)
	in.BugIssues, in.GHBugsOK = codeBugIssues(root, since, until)

	prs, ok := codeMergedPRs(root, since, until)
	in.GHMergedOK = ok
	if ok && len(prs) > 0 {
		for i := range prs {
			prs[i].ReviewComments = countPRReviewComments(root, prs[i].Number)
		}
		in.MergedPRs = prs
	}
	return in
}

// countPRReviewComments counts review comments for a single PR via gh api.
func countPRReviewComments(root string, prNumber int) int {
	out, err := exec.Command("gh", "api",
		fmt.Sprintf("repos/:owner/:repo/pulls/%d/comments", prNumber),
		"--jq", "length").Output()
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return n
}

// --- entrypoints -----------------------------------------------------------

// runCode is the --code entrypoint. It never reads or writes STATUS.md.
func runCode(root, since string, asJSON, asSeries bool) int {
	now := nowFunc()
	until := now
	var sinceT time.Time
	if since != "" {
		t, err := parseSinceDate(since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "statusgen: --since must be YYYY-MM-DD: %v\n", err)
			return 1
		}
		sinceT = t.UTC()
	} else {
		sinceT = until.AddDate(0, 0, -defaultCodeWindowDays)
	}
	if sinceT.After(until) {
		fmt.Fprintln(os.Stderr, "statusgen: --since is in the future")
		return 1
	}

	if asSeries {
		period := "weekly"
		return runCodeSeries(root, sinceT, until, period, asJSON)
	}

	rep := computeCodeEfficiency(gatherCodeInputs(root, sinceT, until, now, defaultChurnDays))
	if asJSON {
		enc, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "statusgen:", err)
			return 1
		}
		fmt.Println(string(enc))
		return 0
	}
	fmt.Print(renderCodeText(rep))
	return 0
}

// runCodeSeries is the --code --series entrypoint.
func runCodeSeries(root string, since, until time.Time, period string, asJSON bool) int {
	commits, gitOK := codeGitStats(root, since, until)
	if !gitOK {
		commits = nil
	}

	prs, ghOK := codeMergedPRs(root, since, until)
	if !ghOK {
		prs = nil
	}

	// Enrich PRs with review comment counts
	for i := range prs {
		prs[i].ReviewComments = countPRReviewComments(root, prs[i].Number)
	}

	bugs, _ := doraBugIssueDates(root, since, until)

	// Clamp since to the first bucket boundary
	clampedSince := since
	if period == "weekly" {
		clampedSince = bucketStart(since, period)
	}

	points := computeCodeSeries(clampedSince, until, period, defaultChurnDays, commits, bugs, prs)
	if len(points) == 0 {
		fmt.Println("no data in window")
		return 0
	}

	if asJSON {
		fmt.Print(renderCodeSeriesJSON(points))
	} else {
		fmt.Print(renderCodeSeriesText(points, clampedSince, until, period))
	}
	return 0
}
