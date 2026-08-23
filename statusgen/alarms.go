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

	// ParkedUntil carries the finding's bounded-park expiry (statusgen/06) when
	// this StandingAlarm describes a parked finding (in AlarmReport.Parked or
	// AlarmReport.ExpiredParks); "" for an ordinary standing alarm.
	ParkedUntil string
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

	// Bounded shelving (statusgen/06 — ISA-18.2 / EEMUA-191). A park is a snooze,
	// not a mute.
	//
	//   - Parked holds findings under a LIVE, well-formed park (now < parked-until):
	//     their standing NOTICE is SUPPRESSED and they are EXCLUDED from ActiveCount
	//     (so a consciously-shelved finding does not inflate the flood count). They
	//     never appear in Standing.
	//   - ExpiredParks holds findings whose park window has CLOSED (now >=
	//     parked-until): they RE-ANNUNCIATE with a distinct, louder NOTICE
	//     ("park expired — re-decide") and COUNT AGAIN toward ActiveCount/flood. A
	//     park buys a bounded window, then forces a fresh decision — it never
	//     silently becomes permanent. They are reported via ExpiredParks rather
	//     than Standing so the desk/retro cannot mistake an expired park for a
	//     fresh standing alarm.
	//
	// A park that is not well-formed (a required field missing, or an unparseable
	// parked-until date) does NOT shelve: the finding is counted and alarmed as an
	// ordinary open finding, and parkFieldProblems raises a hard --lint PROBLEM.
	Parked       []StandingAlarm
	ExpiredParks []StandingAlarm

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

// parkClass is a finding's bounded-shelving state for alarm accounting
// (statusgen/06).
type parkClass int

const (
	parkNone      parkClass = iota // no parked-* fields → an ordinary open finding
	parkActive                     // well-formed park, now < parked-until → shelved (standing NOTICE suppressed, excluded from flood)
	parkExpired                    // parked-until has passed → re-annunciate louder, count again
	parkMalformed                  // some parked-* field set but the park is not well-formed and not expired → NOT shelved (parkFieldProblems raises the PROBLEM)
)

// parkIsAuthorizedVocab reports whether a parked-by value is written in the
// `human:<name>` authority vocabulary (same token shape as lifecycle stamps).
// This is the SYNTACTIC check the advisory alarm layer uses; whether that named
// human is real and actually authorized is enforced HARD elsewhere —
// guttedRegisterFields requires the name to resolve in the configured
// ASSAY_HUMAN_LOGIN_MAP before a park may be ADDED to a landed finding, and CI's
// `--corroborate` requires that human to have acted on the PR. A park that clears
// this vocabulary check but names an unmapped human still fails those hard gates,
// so a syntactic check here cannot silence an unauthorized park in practice.
func parkIsAuthorizedVocab(parkedBy string) bool {
	return humanStampRe.MatchString(parkedBy)
}

// classifyPark determines a finding's bounded-shelving state against an injected
// clock. A finding is "parked" the moment ANY parked-* field is set; the all-empty
// case is the common open/resolved finding (parkNone). An EXPIRED park always
// re-annunciates (parkExpired) — an out-of-window park must never stay quiet, even
// if it is otherwise malformed — so the expiry test precedes the well-formedness
// test. Shelving (parkActive) requires a park that is BOTH well-formed (all three
// fields present, a parseable parked-until, an authorized-vocabulary parked-by) AND
// still inside its window (now < parked-until).
func classifyPark(f Finding, now time.Time) parkClass {
	pu := strings.TrimSpace(f.ParkedUntil)
	pb := strings.TrimSpace(f.ParkedBy)
	pr := strings.TrimSpace(f.ParkedReason)
	if pu == "" && pb == "" && pr == "" {
		return parkNone
	}
	until, err := time.Parse("2006-01-02", pu)
	if err == nil && !now.Before(until) {
		return parkExpired
	}
	wellFormed := pu != "" && pb != "" && pr != "" && err == nil && parkIsAuthorizedVocab(pb)
	if wellFormed {
		return parkActive
	}
	return parkMalformed
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
			// A resolved finding is resolved regardless of any stale park fields;
			// the park machinery never overrides a resolve.
			continue
		}
		ageDays := int(now.Sub(opened).Hours() / 24)

		// Bounded shelving (statusgen/06). A LIVE well-formed park is shelved: it
		// is excluded from the active/flood count and never produces a standing
		// NOTICE. An EXPIRED park re-annunciates louder and counts again. A
		// malformed park is treated as an ordinary open finding here (and raises a
		// hard --lint PROBLEM via parkFieldProblems).
		switch classifyPark(f, now) {
		case parkActive:
			rep.Parked = append(rep.Parked, StandingAlarm{
				ID: f.ID, Date: f.Date, Title: f.Title, AgeDays: ageDays,
				ParkedUntil: strings.TrimSpace(f.ParkedUntil),
			})
			continue
		case parkExpired:
			rep.ActiveCount++
			rep.ExpiredParks = append(rep.ExpiredParks, StandingAlarm{
				ID: f.ID, Date: f.Date, Title: f.Title, AgeDays: ageDays, Over: true,
				ParkedUntil: strings.TrimSpace(f.ParkedUntil),
			})
			continue
		}

		rep.ActiveCount++
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
	// Deterministic ordering for the park lists too: expired parks oldest-window
	// first (earliest parked-until leads — most overdue for a re-decision), live
	// parks by ID for a stable view.
	sort.SliceStable(rep.ExpiredParks, func(i, j int) bool {
		if rep.ExpiredParks[i].ParkedUntil != rep.ExpiredParks[j].ParkedUntil {
			return rep.ExpiredParks[i].ParkedUntil < rep.ExpiredParks[j].ParkedUntil
		}
		return rep.ExpiredParks[i].ID < rep.ExpiredParks[j].ID
	})
	sort.SliceStable(rep.Parked, func(i, j int) bool {
		return rep.Parked[i].ID < rep.Parked[j].ID
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
	// Expired parks re-annunciate FIRST and LOUDER than a plain standing alarm:
	// the park bought a bounded window, that window has closed, and the desk/retro
	// must make a FRESH decision (extend, resolve, or act) rather than let the
	// snooze silently become permanent. The distinct "park EXPIRED" wording is
	// deliberately un-mistakable for a fresh standing alarm (design §A).
	for _, a := range rep.ExpiredParks {
		out = append(out, fmt.Sprintf(
			"park EXPIRED — re-decide: %s was parked until %s, which has passed — extend the park, resolve it, or act on it now. "+
				"A park is a bounded snooze, not a mute (ISA-18.2): %s",
			a.ID, a.ParkedUntil, a.Title))
	}
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

// parkFieldProblems raises one hard --lint PROBLEM per finding whose bounded-park
// (statusgen/06) is not well-formed. A park is a snooze, not a mute, and MUST be
// bounded and attributed: the moment ANY parked-* field is set, all three are
// required and parked-until must be a real future-or-past date. The checks:
//
//   - parked-until MISSING → PROBLEM. An open-ended park is a disguised resolve;
//     bounded shelving requires an explicit expiry.
//   - parked-until present but UNPARSEABLE (not YYYY-MM-DD) → PROBLEM. An
//     unparseable expiry can never fire the re-annunciation, so it would mute
//     forever.
//   - parked-by MISSING, or not in the `human:<name>` authority vocabulary →
//     PROBLEM. A park must name its authorizing party; an agent cannot self-park.
//   - parked-reason MISSING → PROBLEM. A park must record why it is accepted-deferred.
//
// This is the SCHEMA-completeness gate (map-independent). It is distinct from the
// AUTHORIZATION gate (guttedRegisterFields), which additionally requires the
// parked-by name to resolve in ASSAY_HUMAN_LOGIN_MAP before a park may be ADDED to
// a finding that was already landed at the merge-base — and from CI's
// `--corroborate`, which requires that human to have acted on the PR. All three
// must agree for a park to both stand and go quiet.
func parkFieldProblems(findings []Finding) []string {
	var problems []string
	for _, f := range findings {
		pu := strings.TrimSpace(f.ParkedUntil)
		pb := strings.TrimSpace(f.ParkedBy)
		pr := strings.TrimSpace(f.ParkedReason)
		if pu == "" && pb == "" && pr == "" {
			continue // not a park
		}
		var missing []string
		if pu == "" {
			missing = append(missing, "parked-until (a bounded YYYY-MM-DD expiry — no open-ended parks)")
		} else if _, err := time.Parse("2006-01-02", pu); err != nil {
			missing = append(missing, fmt.Sprintf("a parseable parked-until date (got %q, want YYYY-MM-DD)", pu))
		}
		if pb == "" {
			missing = append(missing, "parked-by (the authorizing party in human:<name> form)")
		} else if !parkIsAuthorizedVocab(pb) {
			missing = append(missing, fmt.Sprintf("a parked-by in human:<name> form (got %q)", pb))
		}
		if pr == "" {
			missing = append(missing, "parked-reason (why the finding is accepted-deferred)")
		}
		if len(missing) == 0 {
			continue
		}
		id := strings.TrimSpace(f.ID)
		if id == "" {
			id = "(unidentified finding)"
		}
		problems = append(problems, fmt.Sprintf(
			"findings register: %s: malformed park — missing/invalid: %s. A bounded park (ISA-18.2 shelving) is a snooze, not a mute: it REQUIRES parked-until, parked-by, and parked-reason together, or it silences the standing alarm without a bounded, attributed, reasoned decision.",
			id, strings.Join(missing, "; ")))
	}
	sort.Strings(problems)
	return problems
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
	// Bounded shelving (statusgen/06). Expired parks are LOUD — they re-annunciate
	// and demand a fresh decision — so they print before live parks and before the
	// standing list. Live parks are consciously shelved and excluded from the
	// flood count; they are shown for visibility, not as alarms.
	if len(rep.ExpiredParks) > 0 {
		w("")
		w("## Parks EXPIRED — re-decide")
		w("")
		w("| ID | Parked until | Title |")
		w("|---|---|---|")
		for _, a := range rep.ExpiredParks {
			w("| %s ⚠ | %s | %s |", a.ID, a.ParkedUntil, a.Title)
		}
		w("")
		w("_A park is a bounded snooze, not a mute: each above has passed its `parked-until` and now counts toward the active/flood total again. Extend, resolve, or act._")
	}
	if len(rep.Parked) > 0 {
		w("")
		w("## Parked (shelved, excluded from flood)")
		w("")
		w("| ID | Parked until | Title |")
		w("|---|---|---|")
		for _, a := range rep.Parked {
			w("| %s | %s | %s |", a.ID, a.ParkedUntil, a.Title)
		}
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
