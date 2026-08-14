package main

// DORA metrics emitter (`statusgen --dora`).
//
// Emits the five DORA Core metrics as a SYSTEM — the two throughput metrics
// (deployment frequency, change lead time) and the failed-deploy recovery time,
// plus the two instability metrics (change failure rate, rework rate) — never a
// single metric in isolation. The four families interact; reading one alone is
// how the metric gets gamed (DORA's own warning).
//
// Data sources, in order of automatability:
//   - history log (docs/streams/.history.jsonl):
//     implemented→done lead time — the outcome the 2026-07-09 velocity read
//     exposed as invisible (35 merged, 5 done).
//   - git log: commit / merge frequency in the window.
//   - gh (pr list --state merged, issue list --label bug): merged-PR count +
//     commit→merge lead time, and new bug issues (the automatable slice of
//     change failure rate).
//
// Anti-gaming discipline (the reason recovery / rework / the verify-desk slice
// of change-failure are NOT fabricated): an input that is not yet automatable is
// printed with an explicit `needs: verify-desk|manual` marker and an "unknown"
// placeholder. The emitter never invents a number for a metric whose input it
// cannot compute — a made-up recovery time is worse than an honest "unknown".

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// doraAntiGamingNote is printed in --dora's header (text) and carried in the
// JSON `note` field. Stream-wide rule: these
// metrics are diagnostic, per-project, for continuous improvement — never a
// target, an individual scorecard, or a cross-team comparison.
const doraAntiGamingNote = "DORA metrics are DIAGNOSTIC, per-project, for continuous improvement — " +
	"never a target, an individual scorecard, or a cross-team comparison (Goodhart's law: a measure " +
	"that becomes a target ceases to be a good measure). A metric that starts driving perverse " +
	"behavior is itself a retro finding."

// Canonical metric keys — the JSON `metrics` object always carries exactly
// these five (Verify item 3). Stable identifiers for downstream consumers
// (the retro and --trend).
const (
	doraDeployFreq = "deployment_frequency"
	doraLeadTime   = "change_lead_time"
	doraRecovery   = "failed_deploy_recovery_time"
	doraChangeFail = "change_failure_rate"
	doraRework     = "rework_rate"
)

// Ordered key groups drive the grouped text rendering (throughput first, then
// instability) so the five are always presented together as a system.
var (
	doraThroughputKeys  = []string{doraDeployFreq, doraLeadTime, doraRecovery}
	doraInstabilityKeys = []string{doraChangeFail, doraRework}
)

// defaultDoraWindowDays is the look-back when --since is omitted.
const defaultDoraWindowDays = 28

// DoraMetric is one of the five metrics. Value is a human-readable string (or
// "unknown"); Needs is non-empty ONLY when an input is not automatable and the
// value is therefore a placeholder — the anti-gaming contract. Computed is true
// when Value is derived from real data (even a partial slice), false when it is
// a placeholder.
type DoraMetric struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Family   string `json:"family"` // "throughput" | "instability"
	Value    string `json:"value"`
	Needs    string `json:"needs,omitempty"` // "verify-desk" | "manual" | "verify-desk|manual"
	Computed bool   `json:"computed"`
	Detail   string `json:"detail,omitempty"`
}

// DoraReport is the full emitted system. Metrics is keyed by the canonical
// metric key so a consumer can look up any of the five directly.
type DoraReport struct {
	Since     string                `json:"since"`
	Until     string                `json:"until"`
	Generated string                `json:"generated"`
	Note      string                `json:"note"`
	Metrics   map[string]DoraMetric `json:"metrics"`
}

// doraPR is the slice of a merged pull request the lead-time / frequency
// computations need.
type doraPR struct {
	Number    int
	CreatedAt time.Time
	MergedAt  time.Time
}

// doraInputs is the pure input to computeDora: everything gathered from the
// history log + git + gh. Kept as data so the metric math is testable over a
// testdata history and fixture git/gh values with no exec/network.
type doraInputs struct {
	Since, Until, Now time.Time
	History           []HistoryEntry
	Commits, Merges   int
	GitOK             bool
	MergedPRs         []doraPR
	GHMergedOK        bool
	BugIssues         int
	GHBugsOK          bool
}

// leadTimeImplToDone returns the median implemented→done duration over briefs
// that REACHED done within [since, until]. The first observed "implemented"
// transition per brief is the clock start; the done transition (in window) is
// the stop. Returns ok=false when no brief reached done in the window — that is
// "no data yet", reported honestly as unknown, never a fabricated zero.
func leadTimeImplToDone(history []HistoryEntry, since, until time.Time) (median time.Duration, n int, ok bool) {
	firstImpl := map[string]time.Time{}
	var durs []time.Duration
	for _, e := range history {
		ts, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			continue
		}
		switch e.To {
		case "implemented":
			if _, seen := firstImpl[e.Brief]; !seen {
				firstImpl[e.Brief] = ts
			}
		case "done":
			if ts.Before(since) || ts.After(until) {
				continue
			}
			impl, seen := firstImpl[e.Brief]
			if !seen {
				continue // done with no recorded implemented start — cannot time it
			}
			durs = append(durs, ts.Sub(impl))
		}
	}
	if len(durs) == 0 {
		return 0, 0, false
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	return durs[len(durs)/2], len(durs), true
}

// medianPROpenToMerge returns the median PR-open→merge duration — the git/gh
// commit→merge lead-time proxy. Zero-value CreatedAt PRs are skipped.
func medianPROpenToMerge(prs []doraPR) (time.Duration, int) {
	var durs []time.Duration
	for _, p := range prs {
		if p.CreatedAt.IsZero() || p.MergedAt.IsZero() {
			continue
		}
		durs = append(durs, p.MergedAt.Sub(p.CreatedAt))
	}
	if len(durs) == 0 {
		return 0, 0
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	return durs[len(durs)/2], len(durs)
}

// humanDur formats a duration as days (≥1d) or hours, e.g. "6.0d", "4.5h".
func humanDur(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	if days := d.Hours() / 24; days >= 1 {
		return fmt.Sprintf("%.1fd", days)
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// computeDora is the pure metric computation: doraInputs → DoraReport. No exec,
// no network, no clock — every value is derived from the passed inputs so the
// math is deterministic and testable.
func computeDora(in doraInputs) DoraReport {
	rep := DoraReport{
		Since:     in.Since.Format("2006-01-02"),
		Until:     in.Until.Format("2006-01-02"),
		Generated: in.Now.UTC().Format(time.RFC3339),
		Note:      doraAntiGamingNote,
		Metrics:   map[string]DoraMetric{},
	}
	days := in.Until.Sub(in.Since).Hours() / 24
	if days < 1 {
		days = 1
	}

	// 1. Deployment frequency (throughput) — commits/merges per day from git.
	df := DoraMetric{Key: doraDeployFreq, Name: "Deployment frequency", Family: "throughput"}
	if in.GitOK {
		df.Computed = true
		df.Value = fmt.Sprintf("%.2f commits/day", float64(in.Commits)/days)
		detail := fmt.Sprintf("%d commits (%d merges) over %.0f day(s)", in.Commits, in.Merges, days)
		if in.GHMergedOK {
			detail += fmt.Sprintf("; %d PR(s) merged (%.2f/day)", len(in.MergedPRs), float64(len(in.MergedPRs))/days)
		}
		df.Detail = detail
	} else {
		df.Value = "unknown"
		df.Needs = "manual"
		df.Detail = "git log unavailable in this checkout"
	}
	rep.Metrics[doraDeployFreq] = df

	// 2. Change lead time (throughput) — implemented→done from the historian,
	//    plus commit→merge (PR open→merge) from gh as a supplementary signal.
	lt := DoraMetric{Key: doraLeadTime, Name: "Change lead time", Family: "throughput"}
	if med, n, ok := leadTimeImplToDone(in.History, in.Since, in.Until); ok {
		lt.Computed = true
		lt.Value = humanDur(med)
		lt.Detail = fmt.Sprintf("median implemented→done over %d brief(s) reaching done in window", n)
		if in.GHMergedOK {
			if pm, pn := medianPROpenToMerge(in.MergedPRs); pn > 0 {
				lt.Detail += fmt.Sprintf("; commit→merge (PR open→merge) median %s over %d PR(s)", humanDur(pm), pn)
			}
		}
	} else {
		// Automatable, but no implemented→done transition fell in the window —
		// honest "unknown" (no data), NOT a needs-input placeholder.
		lt.Value = "unknown"
		lt.Detail = "no implemented→done transitions recorded in window (historian)"
	}
	rep.Metrics[doraLeadTime] = lt

	// 3. Failed-deploy recovery time (throughput) — NOT automatable. broken-main
	//    →green timing is not tracked in git/gh/history; needs verify-desk
	//    incident records or manual timing. Explicit placeholder, no number.
	rep.Metrics[doraRecovery] = DoraMetric{
		Key: doraRecovery, Name: "Failed-deploy recovery time", Family: "throughput",
		Value: "unknown", Needs: "verify-desk|manual", Computed: false,
		Detail: "broken-main→green duration is not automated; supply from verify-desk incident records or manual timing",
	}

	// 4. Change failure rate (instability) — the bug-issue slice is automatable
	//    (new bug issues ÷ merged); the verify-desk VERIFY:FAIL slice is not.
	//    Emit the partial, explicitly labelled, with a needs marker for the rest.
	cf := DoraMetric{Key: doraChangeFail, Name: "Change failure rate", Family: "instability"}
	if in.GHBugsOK && in.GHMergedOK && len(in.MergedPRs) > 0 {
		rate := float64(in.BugIssues) / float64(len(in.MergedPRs))
		cf.Computed = true
		cf.Needs = "verify-desk" // full rate still needs VERIFY:FAIL records
		cf.Value = fmt.Sprintf("%.0f%% (partial: bug-issue signal only)", rate*100)
		cf.Detail = fmt.Sprintf("%d new bug issue(s) ÷ %d merged PR(s); add verify-desk VERIFY:FAIL records for the full rate",
			in.BugIssues, len(in.MergedPRs))
	} else {
		cf.Value = "unknown"
		cf.Needs = "verify-desk|manual"
		cf.Detail = "needs merged-PR count (gh) + new bug issues (gh) + verify-desk VERIFY:FAIL records"
	}
	rep.Metrics[doraChangeFail] = cf

	// 5. Rework rate (instability) — NOT automatable. Requires classifying which
	//    follow-up briefs/bugs stem from post-merge defects; not derivable from
	//    git/gh alone. Explicit placeholder, no number.
	rep.Metrics[doraRework] = DoraMetric{
		Key: doraRework, Name: "Rework rate", Family: "instability",
		Value: "unknown", Needs: "verify-desk|manual", Computed: false,
		Detail: "follow-up briefs/bugs from post-merge defects ÷ merged requires post-merge-defect classification (verify-desk/manual)",
	}

	return rep
}

// doraNotAutomated reports whether a metric is a not-yet-automated placeholder:
// no computed value AND an explicit needs marker naming the missing input. This
// is NOT the same state as a computed zero, and NOT the same as the honest
// "no data in this window" unknown (Computed:false with an EMPTY Needs, e.g.
// change lead time with no implemented→done transitions) — that one still
// renders as a row, because the input IS automated and simply had nothing to
// say. Only the could-not-compute-at-all state is trimmed to the footnote.
func doraNotAutomated(m DoraMetric) bool { return !m.Computed && m.Needs != "" }

// renderDoraText renders the report grouped throughput-then-instability, with
// the anti-gaming note in the header. Never renders a single family alone.
//
// The retro-facing text render TRIMS not-yet-automated metrics (see
// doraNotAutomated) out of the family tables — a permanently blank row teaches a
// retro reader nothing — and names them instead in a compact footnote carrying
// the needs marker. The gap is never silently dropped: trimmed must not read as
// clean, and must never read as zero. The JSON export (`--dora --json`) is
// unchanged and still carries every metric, so consumers still see the unknown.
func renderDoraText(rep DoraReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "DORA metrics — %s … %s\n", rep.Since, rep.Until)
	fmt.Fprintf(&b, "%s\n\n", rep.Note)

	var trimmed []DoraMetric
	family := func(heading string, keys []string) {
		fmt.Fprintf(&b, "%s\n", heading)
		rows := 0
		for _, k := range keys {
			m := rep.Metrics[k]
			if doraNotAutomated(m) {
				trimmed = append(trimmed, m)
				continue
			}
			b.WriteString(renderDoraLine(m))
			rows++
		}
		if rows == 0 {
			b.WriteString("  (nothing computed in this window — see the not-yet-automated note below)\n")
		}
	}
	family("Throughput", doraThroughputKeys)
	b.WriteString("\n")
	family("Instability", doraInstabilityKeys)

	b.WriteString(renderDoraNotAutomated(trimmed))
	return b.String()
}

// renderDoraNotAutomated renders the footnote for the trimmed metrics: one line
// per distinct needs marker, listing the metric names in canonical render order.
// In the ordinary case (recovery + rework, both needing verify-desk|manual) that
// is exactly one line. Returns "" when nothing was trimmed.
func renderDoraNotAutomated(trimmed []DoraMetric) string {
	if len(trimmed) == 0 {
		return ""
	}
	var order []string
	names := map[string][]string{}
	for _, m := range trimmed {
		if _, seen := names[m.Needs]; !seen {
			order = append(order, m.Needs)
		}
		names[m.Needs] = append(names[m.Needs], strings.ToLower(m.Name))
	}
	var b strings.Builder
	b.WriteString("\n")
	for _, needs := range order {
		fmt.Fprintf(&b, "not yet automated (no value computed — not zero): %s  [needs: %s]\n",
			strings.Join(names[needs], ", "), needs)
	}
	return b.String()
}

func renderDoraLine(m DoraMetric) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  %-28s %s", m.Name, m.Value)
	if m.Needs != "" {
		fmt.Fprintf(&b, "  [needs: %s]", m.Needs)
	}
	b.WriteString("\n")
	if m.Detail != "" {
		fmt.Fprintf(&b, "      %s\n", m.Detail)
	}
	return b.String()
}

// --- data gathering (exec/network — overridable in tests) ---------------------

// doraGitCommits counts commits and merge commits in [since, until] on the
// current branch/HEAD. ok=false on any git failure (non-git checkout, etc.).
var doraGitCommits = func(root string, since, until time.Time) (total, merges int, ok bool) {
	out, err := exec.Command("git", "-C", root, "log",
		"--since="+since.Format(time.RFC3339),
		"--until="+until.Format(time.RFC3339),
		"--format=%p").Output()
	if err != nil {
		return 0, 0, false
	}
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		total++
		if len(strings.Fields(l)) > 1 { // >1 parent => merge commit
			merges++
		}
	}
	return total, merges, true
}

// doraMergedPRs lists merged PRs via gh, filtered to those merged in the window.
// ok=false when gh is missing/unauthenticated/errors — the caller degrades to a
// needs marker rather than failing the run (offline discipline).
var doraMergedPRs = func(root string, since, until time.Time) ([]doraPR, bool) {
	out, err := exec.Command("gh", "pr", "list", "--state", "merged",
		"--limit", "500", "--json", "number,createdAt,mergedAt").Output()
	if err != nil {
		return nil, false
	}
	var raw []struct {
		Number    int       `json:"number"`
		CreatedAt time.Time `json:"createdAt"`
		MergedAt  time.Time `json:"mergedAt"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, false
	}
	var prs []doraPR
	for _, p := range raw {
		if p.MergedAt.IsZero() || p.MergedAt.Before(since) || p.MergedAt.After(until) {
			continue
		}
		prs = append(prs, doraPR{Number: p.Number, CreatedAt: p.CreatedAt, MergedAt: p.MergedAt})
	}
	return prs, true
}

// doraBugIssues counts bug-labelled issues opened in the window via gh.
// ok=false on any gh failure (degrades to a needs marker).
var doraBugIssues = func(root string, since, until time.Time) (int, bool) {
	out, err := exec.Command("gh", "issue", "list", "--state", "all", "--label", "bug",
		"--limit", "1000", "--json", "number,createdAt").Output()
	if err != nil {
		return 0, false
	}
	var raw []struct {
		Number    int       `json:"number"`
		CreatedAt time.Time `json:"createdAt"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return 0, false
	}
	n := 0
	for _, i := range raw {
		if i.CreatedAt.IsZero() || i.CreatedAt.Before(since) || i.CreatedAt.After(until) {
			continue
		}
		n++
	}
	return n, true
}

// gatherDoraInputs assembles doraInputs from the history log + git + gh. The
// history read tolerates a missing/unreadable log (empty history); git/gh
// failures degrade to their ok=false flags.
func gatherDoraInputs(root string, since, until, now time.Time) doraInputs {
	in := doraInputs{Since: since, Until: until, Now: now}
	if h, err := LoadHistory(filepath.Join(root, filepath.FromSlash(historyRelPath))); err == nil {
		in.History = h
	}
	in.Commits, in.Merges, in.GitOK = doraGitCommits(root, since, until)
	in.MergedPRs, in.GHMergedOK = doraMergedPRs(root, since, until)
	in.BugIssues, in.GHBugsOK = doraBugIssues(root, since, until)
	return in
}

// runDora is the --dora entrypoint. It never reads or writes STATUS.md and never
// runs the source-check suite — a self-contained diagnostic sub-command, same
// discipline as --verify-issues.
func runDora(root, since string, asJSON bool) int {
	now := nowFunc()
	until := now
	var sinceT time.Time
	if since != "" {
		t, err := time.Parse("2006-01-02", since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "statusgen: --since must be YYYY-MM-DD: %v\n", err)
			return 1
		}
		sinceT = t.UTC()
	} else {
		sinceT = until.AddDate(0, 0, -defaultDoraWindowDays)
	}
	if sinceT.After(until) {
		fmt.Fprintln(os.Stderr, "statusgen: --since is in the future")
		return 1
	}

	rep := computeDora(gatherDoraInputs(root, sinceT, until, now))
	if asJSON {
		enc, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "statusgen:", err)
			return 1
		}
		fmt.Println(string(enc))
		return 0
	}
	fmt.Print(renderDoraText(rep))
	return 0
}

// --- DORA time series (--dora --series) -------------------

// doraSeriesPoint is one period bucket in the time series.
type doraSeriesPoint struct {
	Period        string `json:"period"`
	MergedPRs     int    `json:"merged_prs"`
	Commits       int    `json:"commits"`
	BugIssues     int    `json:"bug_issues"`
	CFR           string `json:"cfr"`
	PRLeadTime    string `json:"pr_lead_time"`
	BriefLeadTime string `json:"brief_lead_time"`
}

// smallNSuppress is the minimum n for a median to be reported rather than
// suppressed as misleading (small-n honesty — same discipline as the
// aggregate's "unknown" rows).
const smallNSuppress = 3

// periodMedian returns the median of durs as a human-readable string, or "–"
// when n < smallNSuppress. Returns (value, n, computed).
func periodMedian(durs []time.Duration) (string, int, bool) {
	if len(durs) < smallNSuppress {
		return "–", len(durs), false
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	return humanDur(durs[len(durs)/2]), len(durs), true
}

// computeDoraSeries buckets the DORA data sources per period (ISO week or day),
// reusing trend.go's bucketStart/periodStep. Returns points for each period in
// [since, until] that has data, or nil when there's no data at all.
func computeDoraSeries(
	since, until time.Time,
	period string,
	mergedPRs []doraPR,
	commitDates []time.Time,
	bugIssueDates []time.Time,
	history []HistoryEntry,
) []doraSeriesPoint {
	// Determine the first and last bucket boundaries.
	firstStart := bucketStart(since, period)
	lastStart := bucketStart(until, period)

	if firstStart.After(lastStart) {
		return nil
	}

	// Pre-compute implemented→done lead times per brief from the historian,
	// keyed by the done-transition timestamp so we can bucket them.
	type doneTransition struct {
		ts  time.Time
		dur time.Duration
	}
	var doneTransitions []doneTransition
	firstImpl := map[string]time.Time{}
	for _, e := range history {
		ts, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			continue
		}
		switch e.To {
		case "implemented":
			if _, seen := firstImpl[e.Brief]; !seen {
				firstImpl[e.Brief] = ts
			}
		case "done":
			impl, seen := firstImpl[e.Brief]
			if !seen {
				continue
			}
			doneTransitions = append(doneTransitions, doneTransition{ts: ts, dur: ts.Sub(impl)})
		}
	}

	// Index data by bucket start for O(1) lookup per period.
	prByBucket := map[time.Time][]doraPR{}
	for _, p := range mergedPRs {
		bs := bucketStart(p.MergedAt, period)
		prByBucket[bs] = append(prByBucket[bs], p)
	}
	commitByBucket := map[time.Time]int{}
	for _, t := range commitDates {
		commitByBucket[bucketStart(t, period)]++
	}
	bugByBucket := map[time.Time]int{}
	for _, t := range bugIssueDates {
		bugByBucket[bucketStart(t, period)]++
	}
	doneByBucket := map[time.Time][]time.Duration{}
	for _, dt := range doneTransitions {
		bs := bucketStart(dt.ts, period)
		doneByBucket[bs] = append(doneByBucket[bs], dt.dur)
	}

	var points []doraSeriesPoint
	for bs := firstStart; !bs.After(lastStart); bs = periodStep(bs, period) {
		prs := prByBucket[bs]
		nPR := len(prs)
		nCommits := commitByBucket[bs]
		nBugs := bugByBucket[bs]
		durs := doneByBucket[bs]

		// CFR: bug issues ÷ merged PRs, with the same partial label.
		cfr := "–"
		if nPR > 0 {
			rate := float64(nBugs) / float64(nPR)
			cfr = fmt.Sprintf("%.0f%% (partial)", rate*100)
		}

		// PR lead time: median open→merge for PRs merged in this period.
		prDurs := make([]time.Duration, 0, nPR)
		for _, p := range prs {
			if p.CreatedAt.IsZero() || p.MergedAt.IsZero() {
				continue
			}
			prDurs = append(prDurs, p.MergedAt.Sub(p.CreatedAt))
		}
		prLT, _, _ := periodMedian(prDurs)

		// Brief lead time: median implemented→done for briefs reaching done
		// in this period.
		briefLT, _, _ := periodMedian(durs)

		dateFmt := "2006-01-02"
		label := bs.Format(dateFmt)

		points = append(points, doraSeriesPoint{
			Period:        label,
			MergedPRs:     nPR,
			Commits:       nCommits,
			BugIssues:     nBugs,
			CFR:           cfr,
			PRLeadTime:    prLT,
			BriefLeadTime: briefLT,
		})
	}
	return points
}

// renderDoraSeriesText renders the time series as an aligned table with a
// spark-bar row for CFR (like --trend's sparklines).
func renderDoraSeriesText(points []doraSeriesPoint, since, until time.Time, period string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "DORA time series — %s … %s (%s)\n", since.Format("2006-01-02"), until.Format("2006-01-02"), period)
	fmt.Fprintf(&b, "%s\n\n", doraAntiGamingNote)

	// Header
	fmt.Fprintf(&b, "%-12s %7s %8s %6s %18s %10s %11s\n",
		"period", "merged", "commits", "bugs", "CFR", "PR-lead", "brief-lead")

	// Data rows
	for _, p := range points {
		fmt.Fprintf(&b, "%-12s %7d %8d %6d %18s %10s %11s\n",
			p.Period, p.MergedPRs, p.Commits, p.BugIssues, p.CFR, p.PRLeadTime, p.BriefLeadTime)
	}

	// Spark-bar row for CFR
	if len(points) > 1 {
		// Extract CFR values as ints for sparkline (parse "N%" or "–"=0).
		cfrVals := make([]int, len(points))
		for i, p := range points {
			if p.CFR == "–" || p.MergedPRs == 0 {
				cfrVals[i] = 0
			} else {
				var pct int
				fmt.Sscanf(p.CFR, "%d%%", &pct)
				cfrVals[i] = pct
			}
		}
		hasNonZero := false
		for _, v := range cfrVals {
			if v > 0 {
				hasNonZero = true
				break
			}
		}
		if hasNonZero {
			fmt.Fprintf(&b, "\n              %s  (CFR spark-bar: change-failure rate per period)\n", sparkline(cfrVals))
		}
	}

	return b.String()
}

// renderDoraSeriesJSON renders the series as a JSON array of period objects.
func renderDoraSeriesJSON(points []doraSeriesPoint) string {
	if points == nil {
		points = []doraSeriesPoint{}
	}
	out, err := json.MarshalIndent(points, "", "  ")
	if err != nil {
		return "[]"
	}
	return string(out)
}

// --- series data gathering (timestamped variants for bucketing) ----------------

// doraGitCommitDates returns the author date of every commit in [since, until].
// ok=false on any git failure (non-git checkout, etc.).
var doraGitCommitDates = func(root string, since, until time.Time) ([]time.Time, bool) {
	out, err := exec.Command("git", "-C", root, "log",
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
		t, err := time.Parse(time.RFC3339, l)
		if err != nil {
			continue
		}
		dates = append(dates, t.UTC())
	}
	return dates, true
}

// doraBugIssueDates returns the creation date of every bug-labelled issue
// opened in [since, until]. ok=false on any gh failure.
var doraBugIssueDates = func(root string, since, until time.Time) ([]time.Time, bool) {
	out, err := exec.Command("gh", "issue", "list", "--state", "all", "--label", "bug",
		"--limit", "1000", "--json", "number,createdAt").Output()
	if err != nil {
		return nil, false
	}
	var raw []struct {
		Number    int       `json:"number"`
		CreatedAt time.Time `json:"createdAt"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, false
	}
	var dates []time.Time
	for _, i := range raw {
		if i.CreatedAt.IsZero() || i.CreatedAt.Before(since) || i.CreatedAt.After(until) {
			continue
		}
		dates = append(dates, i.CreatedAt.UTC())
	}
	return dates, true
}

// runDoraSeries is the --dora --series entrypoint. It never reads or writes
// STATUS.md — a self-contained diagnostic sub-command, same discipline as
// --dora and --trend.
func runDoraSeries(root, since, period string, asJSON bool) int {
	now := nowFunc()
	until := now
	var sinceT time.Time
	if since != "" {
		t, err := time.Parse("2006-01-02", since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "statusgen: --since must be YYYY-MM-DD: %v\n", err)
			return 1
		}
		sinceT = t.UTC()
	} else {
		sinceT = until.AddDate(0, 0, -defaultDoraWindowDays)
	}
	if sinceT.After(until) {
		fmt.Fprintln(os.Stderr, "statusgen: --since is in the future")
		return 1
	}

	merges, mergeOK := doraMergedPRs(root, sinceT, until)
	commits, commitOK := doraGitCommitDates(root, sinceT, until)
	bugs, bugsOK := doraBugIssueDates(root, sinceT, until)

	var history []HistoryEntry
	if h, err := LoadHistory(filepath.Join(root, filepath.FromSlash(historyRelPath))); err == nil {
		history = h
	}

	// Degrade gracefully when data sources are unavailable.
	if !mergeOK {
		merges = nil
	}
	if !commitOK {
		commits = nil
	}
	if !bugsOK {
		bugs = nil
	}

	// Clamp since to the first ISO-week Monday on or before the data start so
	// the first bucket is a complete period boundary — same as --trend.
	clampedSince := sinceT
	if period == "weekly" {
		clampedSince = bucketStart(sinceT, period)
	}

	points := computeDoraSeries(clampedSince, until, period, merges, commits, bugs, history)
	if len(points) == 0 {
		fmt.Println("no data in window")
		return 0
	}

	if asJSON {
		fmt.Print(renderDoraSeriesJSON(points))
	} else {
		fmt.Print(renderDoraSeriesText(points, clampedSince, until, period))
	}
	return 0
}

// --- DORA grouped breakdowns (--dora --by stream|goal) ---

// goalPriorityOrder is the fixed display order for per-goal DORA grouping
// (example-app first, example-service second, assay supporting).
// Any goal not listed here sorts after platform then alphabetically.
var goalPriorityOrder = []string{"example-app", "example-service", "assay", "platform"}

// doraSmallNThreshold is the minimum n for a group's median to be reported rather
// than suppressed with an n=<x> annotation (brief-26: groups with <5 events).
const doraSmallNThreshold = 5

// goalPriorityRank returns a sort key for the fixed goal priority order.
// Unlisted goals sort after the canonical set, then alphabetically.
func goalPriorityRank(goal string) int {
	for i, g := range goalPriorityOrder {
		if g == goal {
			return i
		}
	}
	return len(goalPriorityOrder)
}

// streamToGoal maps a stream name to its product goal via the stream README's
// serves: frontmatter. An absent/unset serves: maps to "untagged".
func streamToGoal(streams []*Stream) map[string]string {
	m := map[string]string{}
	for _, s := range streams {
		g := s.Serves
		if g == "" {
			g = "untagged"
		}
		m[s.Name] = g
	}
	return m
}

// briefDoneCounts returns, per group key, the count of briefs reaching "done"
// in [since, until] from the historian. The groupKey function maps a brief ID
// to its group key (stream name or goal).
func briefDoneCounts(history []HistoryEntry, since, until time.Time, groupKey func(string) string) map[string]int {
	done := map[string]bool{}  // brief ID -> reached done in window
	counts := map[string]int{} // group -> count
	for _, e := range history {
		ts, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			continue
		}
		if e.To == "done" && !ts.Before(since) && !ts.After(until) {
			if done[e.Brief] {
				continue
			}
			done[e.Brief] = true
			gk := groupKey(e.Brief)
			counts[gk]++
		}
	}
	return counts
}

// briefLeadTimes returns, per group key, the implemented->done durations for
// briefs whose done transition falls in [since, until]. The groupKey function
// maps a brief ID to its group key.
func briefLeadTimes(history []HistoryEntry, since, until time.Time, groupKey func(string) string) map[string][]time.Duration {
	firstImpl := map[string]time.Time{}
	out := map[string][]time.Duration{}
	for _, e := range history {
		ts, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			continue
		}
		switch e.To {
		case "implemented":
			if _, seen := firstImpl[e.Brief]; !seen {
				firstImpl[e.Brief] = ts
			}
		case "done":
			if ts.Before(since) || ts.After(until) {
				continue
			}
			impl, seen := firstImpl[e.Brief]
			if !seen {
				continue
			}
			gk := groupKey(e.Brief)
			out[gk] = append(out[gk], ts.Sub(impl))
		}
	}
	return out
}

// briefReverts counts backward transitions (implemented/verified/done ->
// todo/in-progress/blocked) from the historian per group key as a change-failure
// proxy signal.
func briefReverts(history []HistoryEntry, since, until time.Time, groupKey func(string) string) map[string]int {
	revertFrom := map[string]bool{"implemented": true, "verified": true, "done": true}
	revertTo := map[string]bool{"todo": true, "in-progress": true, "blocked": true}
	out := map[string]int{}
	for _, e := range history {
		ts, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			continue
		}
		if ts.Before(since) || ts.After(until) {
			continue
		}
		if revertFrom[e.From] && revertTo[e.To] {
			gk := groupKey(e.Brief)
			out[gk]++
		}
	}
	return out
}

// findingsPerGroup counts unresolved findings whose Affects entries map to the
// given group key, as a change-failure proxy signal.
func findingsPerGroup(findings []Finding, groupKey func(string) string) map[string]int {
	out := map[string]int{}
	for _, f := range findings {
		if f.Resolved {
			continue
		}
		for _, a := range f.Affects {
			gk := groupKey(a)
			out[gk]++
		}
	}
	return out
}

// p90Duration returns the 90th percentile of the durations.
func p90Duration(durs []time.Duration) time.Duration {
	if len(durs) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(durs))
	copy(sorted, durs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)) * 0.9)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// medianDur returns the median of the durations.
func medianDur(durs []time.Duration) time.Duration {
	if len(durs) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(durs))
	copy(sorted, durs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[len(sorted)/2]
}

// DoraGroup holds the four DORA metrics for one grouping dimension
// (a stream or a product goal).
type DoraGroup struct {
	Key     string                `json:"key"`
	Label   string                `json:"label"`
	N       int                   `json:"n"`
	SmallN  bool                  `json:"small_n"`
	Metrics map[string]DoraMetric `json:"metrics"`
}

// DoraGroupedReport is the full grouped-DORA output.
type DoraGroupedReport struct {
	Since      string      `json:"since"`
	Until      string      `json:"until"`
	Generated  string      `json:"generated"`
	Note       string      `json:"note"`
	By         string      `json:"by"`
	Groups     []DoraGroup `json:"groups"`
	GlobalMTTR DoraMetric  `json:"global_mttr"`
}

// groupKeyForBrief maps a brief ID (e.g. "example-app/01") to a group
// key. For "stream" mode this is the stream name; for "goal" mode this is the
// goal name (from the stream->goal map). An unrecognized brief ID gets "unknown".
func groupKeyForBrief(briefID string, by string, s2g map[string]string) string {
	parts := strings.SplitN(briefID, "/", 2)
	stream := parts[0]
	switch by {
	case "stream":
		return stream
	case "goal":
		if g, ok := s2g[stream]; ok {
			return g
		}
		return "untagged"
	}
	return "unknown"
}

// groupKeyForFindingAffects maps a finding Affects entry (e.g. "stream-name" or
// "stream-name/brief-01") to a group key. Strips any brief suffix.
func groupKeyForFindingAffects(affects string, by string, s2g map[string]string) string {
	stream := affects
	if idx := strings.Index(affects, "/"); idx >= 0 {
		stream = affects[:idx]
	}
	switch by {
	case "stream":
		return stream
	case "goal":
		if g, ok := s2g[stream]; ok {
			return g
		}
		return "untagged"
	}
	return "unknown"
}

// computeDoraGrouped computes the four per-group DORA metrics + global MTTR.
func computeDoraGrouped(in doraInputs, streams []*Stream, findings []Finding, by string) DoraGroupedReport {
	rep := DoraGroupedReport{
		Since:     in.Since.Format("2006-01-02"),
		Until:     in.Until.Format("2006-01-02"),
		Generated: in.Now.UTC().Format(time.RFC3339),
		Note:      doraAntiGamingNote,
		By:        by,
	}
	days := in.Until.Sub(in.Since).Hours() / 24
	if days < 1 {
		days = 1
	}
	weeks := days / 7
	if weeks < 1 {
		weeks = 1
	}

	s2g := streamToGoal(streams)

	gkB := func(briefID string) string { return groupKeyForBrief(briefID, by, s2g) }
	gkF := func(affects string) string { return groupKeyForFindingAffects(affects, by, s2g) }

	doneN := briefDoneCounts(in.History, in.Since, in.Until, gkB)
	lts := briefLeadTimes(in.History, in.Since, in.Until, gkB)
	reverts := briefReverts(in.History, in.Since, in.Until, gkB)
	findingsG := findingsPerGroup(findings, gkF)

	// Collect all group keys and sort them deterministically.
	allKeys := map[string]bool{}
	for k := range doneN {
		allKeys[k] = true
	}
	for k := range lts {
		allKeys[k] = true
	}
	for k := range reverts {
		allKeys[k] = true
	}
	for k := range findingsG {
		allKeys[k] = true
	}
	// Ensure every stream's key appears, even with zero data.
	for _, s := range streams {
		k := s.Name
		if by == "goal" {
			if g, ok := s2g[s.Name]; ok {
				k = g
			} else {
				k = "untagged"
			}
		}
		allKeys[k] = true
	}

	sortedKeys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Slice(sortedKeys, func(i, j int) bool {
		if by == "goal" {
			ri, rj := goalPriorityRank(sortedKeys[i]), goalPriorityRank(sortedKeys[j])
			if ri != rj {
				return ri < rj
			}
			if ri >= len(goalPriorityOrder) && rj >= len(goalPriorityOrder) {
				return sortedKeys[i] < sortedKeys[j]
			}
		}
		return sortedKeys[i] < sortedKeys[j]
	})

	for _, k := range sortedKeys {
		n := doneN[k]
		smallN := n > 0 && n < doraSmallNThreshold
		group := DoraGroup{
			Key:     k,
			Label:   groupLabel(k, by),
			N:       n,
			SmallN:  smallN,
			Metrics: map[string]DoraMetric{},
		}

		nNote := ""
		if smallN {
			nNote = fmt.Sprintf(" (n=%d)", n)
		}

		// 1. Deployment frequency -- briefs done per group / week (proxy).
		df := DoraMetric{
			Key: doraDeployFreq, Name: "Deployment frequency", Family: "throughput",
			Computed: n > 0,
		}
		if n > 0 {
			df.Value = fmt.Sprintf("%.1f briefs/week", float64(n)/weeks)
			df.Detail = fmt.Sprintf("%d brief(s) reaching done in window (proxy: brief-completion rate%s; aggregate deploy freq uses git commits)", n, nNote)
		} else {
			df.Value = "no briefs done in window"
			df.Detail = "brief-completion proxy -- no briefs reached done in this group during the window"
		}
		group.Metrics[doraDeployFreq] = df

		// 2. Change lead time -- implemented->done median + p90.
		lt := DoraMetric{
			Key: doraLeadTime, Name: "Change lead time", Family: "throughput",
		}
		if durs, ok := lts[k]; ok && len(durs) > 0 {
			lt.Computed = true
			med := medianDur(durs)
			p90 := p90Duration(durs)
			lt.Value = fmt.Sprintf("%s / %s", humanDur(med), humanDur(p90))
			detail := fmt.Sprintf("median/p90 implemented->done over %d brief(s)%s", len(durs), nNote)
			if smallN && len(durs) > 0 {
				detail += " [small-n: a median over <5 briefs is an anecdote, not a metric]"
			}
			lt.Detail = detail
		} else {
			lt.Value = "unknown"
			lt.Detail = "no implemented->done transitions recorded for this group in window"
		}
		group.Metrics[doraLeadTime] = lt

		// 3. Change-failure proxy -- (findings + reverts) / done briefs.
		cf := DoraMetric{
			Key: doraChangeFail, Name: "Change failure rate", Family: "instability",
		}
		fCount := findingsG[k]
		rCount := reverts[k]
		if n > 0 && (fCount > 0 || rCount > 0) {
			rate := float64(fCount+rCount) / float64(n)
			cf.Computed = true
			cf.Value = fmt.Sprintf("%.0f%% (proxy)", rate*100)
			cf.Detail = fmt.Sprintf("%d finding(s) + %d revert(s) / %d done brief(s) = %.0f%% (proxy: findings/reverts over done briefs)%s",
				fCount, rCount, n, rate*100, nNote)
		} else if n == 0 {
			cf.Value = "unknown"
			cf.Detail = "no briefs done in window -- cannot compute a rate"
		} else {
			cf.Value = "0% (proxy)"
			cf.Detail = fmt.Sprintf("0 findings/reverts / %d done brief(s) = 0%% (proxy)%s", n, nNote)
		}
		group.Metrics[doraChangeFail] = cf

		// 4. Rework rate -- NOT automatable (same as the aggregate).
		group.Metrics[doraRework] = DoraMetric{
			Key: doraRework, Name: "Rework rate", Family: "instability",
			Value: "unknown", Needs: "verify-desk|manual", Computed: false,
			Detail: fmt.Sprintf("follow-up briefs/bugs from post-merge defects / merged requires post-merge-defect classification (verify-desk/manual)%s", nNote),
		}

		rep.Groups = append(rep.Groups, group)
	}

	// Global MTTR -- stays global-only in v1 (brief-26).
	rep.GlobalMTTR = DoraMetric{
		Key: doraRecovery, Name: "Failed-deploy recovery time (global)", Family: "throughput",
		Value: "unknown", Needs: "verify-desk|manual", Computed: false,
		Detail: "MTTR stays global-only in v1 -- per-group attribution is noisy; broken-main->green duration is not automated. Supply from verify-desk incident records or manual timing.",
	}

	return rep
}

// groupLabel returns a human-readable label for a group key.
func groupLabel(key, by string) string {
	if by == "goal" && key == "untagged" {
		return "untagged (no serves: tag)"
	}
	return key
}

// renderDoraGroupedText renders the grouped report as text.
func renderDoraGroupedText(rep DoraGroupedReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "DORA metrics by %s -- %s ... %s\n", rep.By, rep.Since, rep.Until)
	fmt.Fprintf(&b, "%s\n", rep.Note)
	fmt.Fprintf(&b, "Per-group proxy definitions:\n")
	fmt.Fprintf(&b, "  deploy freq = briefs reaching done / window weeks (proxy; aggregate uses git commits)\n")
	fmt.Fprintf(&b, "  lead time   = implemented->done median / p90 from historian\n")
	fmt.Fprintf(&b, "  change fail = (unresolved findings + reverts) / done briefs (proxy; aggregate uses bug issues / merged PRs)\n")
	fmt.Fprintf(&b, "  rework      = unknown (needs: verify-desk|manual, same as aggregate)\n")
	fmt.Fprintf(&b, "  MTTR        = global-only in v1 (per-group attribution noisy)\n")
	fmt.Fprintf(&b, "Groups with n < %d briefs annotate every figure with n=<x> (small-n honesty).\n\n", doraSmallNThreshold)

	for _, g := range rep.Groups {
		sn := ""
		if g.SmallN {
			sn = fmt.Sprintf(" [n=%d, small-n]", g.N)
		}
		fmt.Fprintf(&b, "=== %s%s ===\n", g.Label, sn)
		for _, k := range []string{doraDeployFreq, doraLeadTime, doraChangeFail, doraRework} {
			m := g.Metrics[k]
			fmt.Fprintf(&b, "  %-28s %s", m.Name, m.Value)
			if m.Needs != "" {
				fmt.Fprintf(&b, "  [needs: %s]", m.Needs)
			}
			b.WriteString("\n")
			if m.Detail != "" {
				fmt.Fprintf(&b, "      %s\n", m.Detail)
			}
		}
		b.WriteString("\n")
	}

	// Global MTTR
	fmt.Fprintf(&b, "=== MTTR (global) ===\n")
	fmt.Fprintf(&b, "  %-28s %s", rep.GlobalMTTR.Name, rep.GlobalMTTR.Value)
	if rep.GlobalMTTR.Needs != "" {
		fmt.Fprintf(&b, "  [needs: %s]", rep.GlobalMTTR.Needs)
	}
	b.WriteString("\n")
	if rep.GlobalMTTR.Detail != "" {
		fmt.Fprintf(&b, "      %s\n", rep.GlobalMTTR.Detail)
	}

	return b.String()
}

// renderDoraGroupedJSON renders the grouped report as JSON.
func renderDoraGroupedJSON(rep DoraGroupedReport) string {
	enc, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(enc)
}

// runDoraGrouped is the --dora --by stream|goal entrypoint.
func runDoraGrouped(root, since, by string, asJSON bool) int {
	now := nowFunc()
	until := now
	var sinceT time.Time
	if since != "" {
		t, err := time.Parse("2006-01-02", since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "statusgen: --since must be YYYY-MM-DD: %v\n", err)
			return 1
		}
		sinceT = t.UTC()
	} else {
		sinceT = until.AddDate(0, 0, -defaultDoraWindowDays)
	}
	if sinceT.After(until) {
		fmt.Fprintln(os.Stderr, "statusgen: --since is in the future")
		return 1
	}

	// Load streams and findings for the serves: mapping.
	streams, findings, err := loadStreams(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "statusgen: loading streams: %v\n", err)
		return 1
	}

	in := gatherDoraInputs(root, sinceT, until, now)
	rep := computeDoraGrouped(in, streams, findings, by)
	if asJSON {
		fmt.Print(renderDoraGroupedJSON(rep))
		return 0
	}
	fmt.Print(renderDoraGroupedText(rep))
	return 0
}

// runDoraSeriesGrouped is the --dora --series --by stream|goal entrypoint.
// The grouped-series bucketing is pending -- currently falls back to aggregate series.
func runDoraSeriesGrouped(root, since, by string, asJSON bool) int {
	fmt.Fprintf(os.Stderr, "NOTICE: --dora --series --by %s: per-group time-series bucketing is pending; showing aggregate series.\n", by)

	now := nowFunc()
	until := now
	var sinceT time.Time
	if since != "" {
		t, err := time.Parse("2006-01-02", since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "statusgen: --since must be YYYY-MM-DD: %v\n", err)
			return 1
		}
		sinceT = t.UTC()
	} else {
		sinceT = until.AddDate(0, 0, -defaultDoraWindowDays)
	}
	if sinceT.After(until) {
		fmt.Fprintln(os.Stderr, "statusgen: --since is in the future")
		return 1
	}

	period := "weekly"
	clampedSince := sinceT
	if period == "weekly" {
		clampedSince = bucketStart(sinceT, period)
	}

	merges, _ := doraMergedPRs(root, sinceT, until)
	commits, _ := doraGitCommitDates(root, sinceT, until)
	bugs, _ := doraBugIssueDates(root, sinceT, until)

	var history []HistoryEntry
	if h, err := LoadHistory(filepath.Join(root, filepath.FromSlash(historyRelPath))); err == nil {
		history = h
	}

	points := computeDoraSeries(clampedSince, until, period, merges, commits, bugs, history)
	if len(points) == 0 {
		fmt.Println("no data in window")
		return 0
	}

	if asJSON {
		fmt.Print(renderDoraSeriesJSON(points))
	} else {
		fmt.Print(renderDoraSeriesText(points, clampedSince, until, period))
	}
	return 0
}
