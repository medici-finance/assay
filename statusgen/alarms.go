package main

// Alarm KPIs for the FINDINGS register (ISA-18.2 / EEMUA-191 alarm management).
// FINDINGS entries are the methodology's alarms: each
// flags one or more briefs ⚠ stale until resolved (see applyFindings). Process
// control has decades of practice on keeping an alarm system healthy; the three
// KPIs that matter here are:
//
//   - alarm rate       — how many findings are opened per week. A rising rate
//                        means the process is generating knowledge-invalidations
//                        faster than it's absorbing them.
//   - standing-alarm age — how long an unresolved finding has stood (now − opened).
//                        A finding that stands past one retro-cycle is exactly what
//                        RETRO.md's manual "FINDINGS age > 1 retro" check hunts for;
//                        this makes it mechanical.
//   - flood            — too many active (unresolved) findings at once. Past a
//                        threshold the register drowns the operator and standing
//                        alarms train everyone to ignore the whole list
//                        (scada-ooda-lineage.md).
//
// All computation is pure over the parsed FINDINGS register (Finding.Date is the
// open date, from the entry's `date:` frontmatter field) plus an injected
// clock, so it is deterministic and offline — the same discipline as --lint.

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// Alarm-KPI thresholds. Named constants, never magic numbers buried in logic
// (Task 3) — each is overridable from the CLI.
const (
	// defaultStandingAgeDays is the standing-alarm age threshold: an unresolved
	// finding older than this is flagged. One retro-cycle — RETRO.md runs weekly,
	// and RETRO's own rule schedules/parks anything stale-flagged > 1 retro.
	defaultStandingAgeDays = 7
	// defaultFloodThreshold is the active-unresolved count above which the register
	// is in a flood condition. Aligned with the Next-up span-of-control cap
	// (EEMUA-191, 7 ± 2): more live alarms than an operator can hold at once.
	defaultFloodThreshold = 7
	// alarmRatePeriodDays is the window for the "current" alarm rate — findings
	// opened per week. A calendar week, matching the retro cadence.
	alarmRatePeriodDays = 7
)

// Alarm-KPI config, defaulted to the constants above and overridden by main()'s
// --standing-age-days / --flood-threshold flags before any run (same wiring
// pattern as spanOfControl in nextup.go).
var (
	standingAgeDays = defaultStandingAgeDays
	floodThreshold  = defaultFloodThreshold
)

// AlarmConfig captures the tunable thresholds for one alarm computation.
type AlarmConfig struct {
	StandingAgeDays int
	FloodThreshold  int
}

// currentAlarmConfig snapshots the wired-in package config for a run.
func currentAlarmConfig() AlarmConfig {
	return AlarmConfig{StandingAgeDays: standingAgeDays, FloodThreshold: floodThreshold}
}

// StandingAlarm is one unresolved finding with its computed age.
type StandingAlarm struct {
	ID      string
	Date    string
	Title   string
	AgeDays int
	Over    bool // age strictly past the standing-age threshold
}

// AlarmReport is the computed alarm state over the FINDINGS register.
type AlarmReport struct {
	Now           time.Time
	Config        AlarmConfig
	OpenedTotal   int             // findings opened in the observed window (all, resolved incl.)
	WindowDays    int             // earliest open date → now, floored at alarmRatePeriodDays
	AvgPerWeek    float64         // OpenedTotal over the window, per 7-day week
	RecentOpened  int             // findings opened in the last alarmRatePeriodDays
	ActiveCount   int             // unresolved findings
	Flood         bool            // ActiveCount strictly over FloodThreshold
	Standing      []StandingAlarm // unresolved findings, oldest first
	PastThreshold int             // standing alarms strictly past the age threshold

	// Undated carries the IDs of findings whose date could not be parsed. They are
	// absent from EVERY count above — OpenedTotal, ActiveCount, Flood, Standing — so a
	// report with a non-empty Undated is a FLOOR, not a total, and the renderers must
	// say so. Silently dropping them made a date typo delete a finding from the flood
	// and standing-age alarms with no output at all: a could-not-check rendered as a
	// clean read (docs/three-state-instrument-rule.md, sub-rule 1). The sibling
	// instrument over the intake register already reports this per entry
	// (intake_alarm.go's BadDates); this brings the findings register into line.
	Undated []string
}

// findingOpenDate parses a Finding.Date (`YYYY-MM-DD` from the heading) as a UTC
// midnight time. Malformed/empty dates yield ok=false and are skipped by callers
// — a register heading that fails findingHeadRe never produces a Finding at all,
// so this is defensive.
func findingOpenDate(f Finding) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(f.Date))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// computeAlarms derives the alarm KPIs from the parsed register and an injected
// clock. Pure and deterministic: same findings + same now → same report.
func computeAlarms(findings []Finding, cfg AlarmConfig, now time.Time) AlarmReport {
	rep := AlarmReport{Now: now, Config: cfg, WindowDays: alarmRatePeriodDays}

	var earliest time.Time
	recentCutoff := now.AddDate(0, 0, -alarmRatePeriodDays)
	for _, f := range findings {
		opened, ok := findingOpenDate(f)
		if !ok {
			// Could-not-check, not "no finding": record it so the renderers can
			// declare the counts a floor rather than a total.
			id := strings.TrimSpace(f.ID)
			if id == "" {
				id = "(unidentified finding)"
			}
			rep.Undated = append(rep.Undated, id)
			continue
		}
		rep.OpenedTotal++
		if earliest.IsZero() || opened.Before(earliest) {
			earliest = opened
		}
		if !opened.Before(recentCutoff) {
			rep.RecentOpened++
		}
		if f.Resolved {
			continue
		}
		rep.ActiveCount++
		ageDays := int(now.Sub(opened).Hours() / 24)
		over := ageDays > cfg.StandingAgeDays
		if over {
			rep.PastThreshold++
		}
		rep.Standing = append(rep.Standing, StandingAlarm{
			ID: f.ID, Date: f.Date, Title: f.Title, AgeDays: ageDays, Over: over,
		})
	}

	// Window = earliest open date → now, floored at one period so a young register
	// never divides the rate up into a spurious spike.
	if !earliest.IsZero() {
		if d := int(now.Sub(earliest).Hours() / 24); d > alarmRatePeriodDays {
			rep.WindowDays = d
		}
	}
	weeks := float64(rep.WindowDays) / float64(alarmRatePeriodDays)
	if weeks > 0 {
		rep.AvgPerWeek = float64(rep.OpenedTotal) / weeks
	}

	rep.Flood = rep.ActiveCount > cfg.FloodThreshold

	// Oldest first: the standing alarm most in need of attention leads.
	sort.SliceStable(rep.Standing, func(i, j int) bool {
		return rep.Standing[i].AgeDays > rep.Standing[j].AgeDays
	})
	return rep
}

// standingAlarmNotices renders one --lint NOTICE per standing alarm past the age
// threshold, plus a single flood NOTICE when active findings overflow. These make
// the alarm state visible to the desk/retro without a manual scan of FINDINGS.md
// (Task 2). Advisory only — never a hard problem.
func standingAlarmNotices(findings []Finding, cfg AlarmConfig, now time.Time) []string {
	rep := computeAlarms(findings, cfg, now)
	var out []string
	for _, a := range rep.Standing {
		if a.Over {
			out = append(out, fmt.Sprintf(
				"standing alarm: %s open %d days (> %d-day threshold, ~1 retro-cycle) — resolve or park it: %s",
				a.ID, a.AgeDays, cfg.StandingAgeDays, a.Title))
		}
	}
	if rep.Flood {
		out = append(out, fmt.Sprintf(
			"alarm flood: %d active findings (> %d threshold) — the register is drowning; clear before opening more (ISA-18.2)",
			rep.ActiveCount, cfg.FloodThreshold))
	}
	if n := len(rep.Undated); n > 0 {
		out = append(out, fmt.Sprintf(
			"alarm COULD-NOT-CHECK: %d finding(s) have an unparseable date (%s) and are counted in NOTHING above — "+
				"the active count %d and the standing-alarm list are a FLOOR, not a total; fix the date heading(s) to bring them under the alarms",
			n, strings.Join(rep.Undated, ", "), rep.ActiveCount))
	}
	return out
}

// runAlarms is the self-contained `--alarms` sub-command: load the register,
// compute the KPIs, and print the human-readable view to stdout. It does not
// read or write STATUS.md and never runs the source-check suite — the same
// offline discipline as --verify-issues.
func runAlarms(root string) int {
	// Per-entry directory (docs/streams/findings/) — the source of truth, same
	// as loadStreams/--lint's standingAlarmNotices/emit. FINDINGS.md is a
	// generated, main-CI-only artifact; a branch never regenerates it, so
	// reading it here would silently disagree with --lint on that same branch.
	findings, err := parseFindings(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: alarms:", err)
		return 1
	}
	fmt.Print(renderAlarms(computeAlarms(findings, currentAlarmConfig(), nowFunc())))
	return 0
}

// renderAlarms formats an AlarmReport as the --alarms view.
func renderAlarms(rep AlarmReport) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }

	w("# FINDINGS alarm KPIs (ISA-18.2)")
	w("")
	w("_As of %s · thresholds: standing-age %d days (~1 retro-cycle), flood %d active._",
		rep.Now.Format("2006-01-02"), rep.Config.StandingAgeDays, rep.Config.FloodThreshold)
	w("")
	w("## Alarm rate")
	w("")
	w("- Opened in last %d days: **%d**", alarmRatePeriodDays, rep.RecentOpened)
	w("- Average over %d-day window: **%.2f / week** (%d opened total)",
		rep.WindowDays, rep.AvgPerWeek, rep.OpenedTotal)
	w("")
	w("## Flood")
	w("")
	if rep.Flood {
		w("- ⚠ FLOOD: **%d** active findings (> %d threshold) — the register is drowning.",
			rep.ActiveCount, rep.Config.FloodThreshold)
	} else {
		w("- %d active findings (threshold %d) — within span of control.",
			rep.ActiveCount, rep.Config.FloodThreshold)
	}
	// Could-not-check, printed BEFORE the standing-alarm section so it is read before
	// any count is trusted — including the early return on an empty Standing list,
	// which would otherwise print "_None — no unresolved findings._" over findings the
	// instrument merely failed to date.
	if n := len(rep.Undated); n > 0 {
		w("")
		w("## Could-not-check")
		w("")
		w("- ⓘ **%d finding(s) have an unparseable date** (%s) and are counted in NOTHING above.",
			n, strings.Join(rep.Undated, ", "))
		w("  Every number in this report is a **floor, not a total**, until their date headings are fixed.")
	}
	w("")
	w("## Standing alarms")
	w("")
	if len(rep.Standing) == 0 {
		if len(rep.Undated) > 0 {
			w("_No DATED unresolved finding — but %d finding(s) could not be dated (above), so this is not a clean register._",
				len(rep.Undated))
		} else {
			w("_None — no unresolved findings._")
		}
		return b.String()
	}
	w("| ID | Opened | Age (days) | Title |")
	w("|---|---|---|---|")
	for _, a := range rep.Standing {
		flag := ""
		if a.Over {
			flag = " ⚠"
		}
		w("| %s%s | %s | %d | %s |", a.ID, flag, a.Date, a.AgeDays, a.Title)
	}
	w("")
	w("_%d of %d standing alarm(s) past the %d-day age threshold (⚠)._",
		rep.PastThreshold, len(rep.Standing), rep.Config.StandingAgeDays)
	return b.String()
}
