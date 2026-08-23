package main

// --cynefin is the Cynefin-domain view (agentic-metrics/10): it classifies each
// unit of active work by the `domain:` frontmatter field — clear | complicated |
// complex | chaotic (Snowden's Cynefin) — and surfaces THREE things a reader
// acts on:
//
//   - the domain DISTRIBUTION across active work (how much of the frontier is
//     Complex, where a single ToC constraint misleads and you need
//     probe-sense-respond, vs Ordered, where ToC applies);
//   - DRIFT over time — the domain mix of the transitions the historian
//     recorded per period, so you can see the work's centre of gravity move;
//   - the DISORDER list — active brief-v1 briefs with NO domain, i.e. work the
//     author never classified. (Absence defaults to `complicated` for the
//     ToC-switch, but the view still names it so it gets classified.)
//
// It is a pure READ, same STATUS.md-free discipline as --dora/--trend/
// --bottleneck: it never reads or writes STATUS.md and never mutates the log.
//
// Three-state (docs/three-state-instrument-rule.md). The guarded question is
// "is active work classified into a Cynefin domain?":
//   - checked-clean   — active work exists and every brief is classified;
//   - checked-failed  — active work exists but >=1 brief is in Disorder (untagged);
//   - could-not-check — there is no active brief-v1 work to classify (absence of
//     evidence is reported as could-not-check with a reason, never as clean).
// The state rides the OUTPUT (the `state` field / a marker line); the process
// exit code stays 0 on a well-formed invocation, like every other diagnostic
// view — --cynefin is a lens, never a gate, and must not redden a board.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// cynefin state markers (the three-state instrument invariant).
const (
	cynefinClean   = "checked-clean"
	cynefinFailed  = "checked-failed"
	cynefinUnknown = "could-not-check"
)

// defaultDomain is the operational fallback for an absent `domain:` — the safe
// Ordered default (most engineering briefs are Complicated). It governs the
// ToC↔Cynefin switch; the view still lists an absent-domain brief as Disorder.
const defaultDomain = "complicated"

// disorderDomain is the display bucket for work that carries no `domain:` — the
// Cynefin "Disorder" central region: you do not yet know which domain you are in.
const disorderDomain = "disorder"

// cynefinDomainOrder is the render/JSON order: the four named domains, then the
// Disorder bucket last (untagged, needs classification).
var cynefinDomainOrder = []string{"clear", "complicated", "complex", "chaotic", disorderDomain}

// cynefinLever is the one-line diagnostic each domain reaches for — the payload
// of the whole lens (which diagnostic to use), echoed in the text view.
var cynefinLever = map[string]string{
	"clear":        "Ordered → ToC (sense-categorize-respond; best practice)",
	"complicated":  "Ordered → ToC (sense-analyze-respond; good practice, expertise)",
	"complex":      "Complex → probe-sense-respond (enabling constraints; emergent practice)",
	"chaotic":      "Chaotic → act-sense-respond (novel practice; impose a constraint first)",
	disorderDomain: "untagged — the author should classify this brief",
}

// cynefinDriftBucket is one period's rolled-up domain mix of the transitions the
// historian recorded within it.
type cynefinDriftBucket struct {
	Period       string         `json:"period"`       // YYYY-MM-DD bucket start
	Distribution map[string]int `json:"distribution"` // domain → transition count in period
	Transitions  int            `json:"transitions"`  // total transitions in period
}

// cynefinReport is the whole view (JSON payload + render input).
type cynefinReport struct {
	State        string               `json:"state"`            // checked-clean | checked-failed | could-not-check
	Reason       string               `json:"reason,omitempty"` // why, on could-not-check / checked-failed
	Total        int                  `json:"total"`            // active brief-v1 briefs classified
	Distribution map[string]int       `json:"distribution"`     // domain → count of active briefs (all 5 buckets present)
	Disorder     []string             `json:"disorder"`         // active brief IDs with no domain (sorted)
	Drift        []cynefinDriftBucket `json:"drift"`            // per-period transition mix by domain
	Period       string               `json:"period"`           // drift bucketing: weekly | daily
	// Mismatch is the money diagnostic (cynefinmismatch.go): complex-tagged
	// active briefs managed with ordered tools — the domain-approach mismatch
	// that makes Complex work thrash.
	Mismatch []cynefinMismatch `json:"mismatch"`
	// ComplexMeasures is the measure set the Complex domain runs on; each
	// measure is three-state, reporting could-not-check when its source is not
	// wired rather than a fabricated zero.
	ComplexMeasures cynefinComplexMeasures `json:"complex-measures"`
}

// effectiveDomain maps a raw `domain:` value to the domain that governs the
// ToC-switch: an empty (untagged) value defaults to complicated. It does NOT
// collapse Disorder for the VIEW — the distribution reports untagged as
// Disorder so it gets classified — it is the operational reading callers use
// when they need one domain per brief regardless.
func effectiveDomain(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return defaultDomain
	}
	return raw
}

// displayDomain maps a raw `domain:` value to its VIEW bucket: untagged → the
// Disorder bucket (so the reader sees it needs classification); a recognized
// value → itself; anything else (which the lint already PROBLEMs) → Disorder.
func displayDomain(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" || !validDomain[v] {
		return disorderDomain
	}
	return v
}

// computeCynefin builds the report from the loaded streams (README rows carry
// status) and the historian log (transitions carry the time axis). Scope is
// brief-v1 files only — legacy/opted-out briefs have no domain concept and are
// exempt, exactly as everywhere else.
func computeCynefin(streams []*Stream, history []HistoryEntry, period string) cynefinReport {
	dist := map[string]int{}
	for _, d := range cynefinDomainOrder {
		dist[d] = 0
	}
	var disorder []string
	total := 0
	domainByID := map[string]string{} // brief id → display domain (all brief-v1 files, for drift)

	for _, s := range streams {
		// index rows by brief num for a status lookup
		rowStatus := map[string]string{}
		for i := range s.Briefs {
			rowStatus[s.Briefs[i].Num] = s.Briefs[i].Status
		}
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				// malformed → reported by --lint; legacy/opted-out → exempt.
				continue
			}
			id, num, okName := expectedBriefID(path)
			if !okName {
				continue
			}
			dd := displayDomain(bf.Domain)
			domainByID[id] = dd

			status, hasRow := rowStatus[num]
			if !hasRow || status == "done" {
				// distribution is over ACTIVE work only (done has left the
				// pipeline; a rowless file is not on the board).
				continue
			}
			total++
			dist[dd]++
			if dd == disorderDomain {
				disorder = append(disorder, id)
			}
		}
	}
	sort.Strings(disorder)

	drift := computeCynefinDrift(history, domainByID, period)

	rep := cynefinReport{
		Total:           total,
		Distribution:    dist,
		Disorder:        disorder,
		Drift:           drift,
		Period:          period,
		Mismatch:        computeCynefinMismatch(streams),
		ComplexMeasures: computeCynefinComplexMeasures(streams),
	}
	switch {
	case total == 0:
		rep.State = cynefinUnknown
		rep.Reason = "no active brief-v1 work to classify"
	case len(disorder) > 0:
		rep.State = cynefinFailed
		rep.Reason = fmt.Sprintf("%d active brief(s) in Disorder — untagged, the author should classify", len(disorder))
	default:
		rep.State = cynefinClean
	}
	return rep
}

// computeCynefinDrift buckets the historian's transitions by period and, within
// each period, by the transitioning brief's CURRENT display domain (frontmatter
// is the domain source; the log carries only the time axis). Transitions for a
// brief we can no longer classify (deleted / legacy) are skipped — they cannot
// be attributed to a domain. Empty history → nil (an honest "no drift to show",
// distinct from a fabricated flat series).
func computeCynefinDrift(history []HistoryEntry, domainByID map[string]string, period string) []cynefinDriftBucket {
	if len(history) < 1 {
		return nil
	}
	sorted, times, err := sortedEntries(history)
	if err != nil {
		// a malformed timestamp is a machine-format defect; degrade to no drift
		// rather than crash the diagnostic.
		return nil
	}
	byBucket := map[string]*cynefinDriftBucket{}
	var order []string
	for i, e := range sorted {
		dd, ok := domainByID[e.Brief]
		if !ok {
			continue
		}
		bs := bucketStart(times[i], period).Format("2006-01-02")
		b := byBucket[bs]
		if b == nil {
			b = &cynefinDriftBucket{Period: bs, Distribution: map[string]int{}}
			for _, d := range cynefinDomainOrder {
				b.Distribution[d] = 0
			}
			byBucket[bs] = b
			order = append(order, bs)
		}
		b.Distribution[dd]++
		b.Transitions++
	}
	sort.Strings(order)
	out := make([]cynefinDriftBucket, 0, len(order))
	for _, k := range order {
		out = append(out, *byBucket[k])
	}
	return out
}

// renderCynefinText formats the terminal view: distribution table with bars and
// the per-domain diagnostic lever, the Disorder list, the drift table with a
// sparkline, and the three-state marker line.
func renderCynefinText(rep cynefinReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "statusgen --cynefin — Cynefin domain view (%s drift)\n", rep.Period)
	fmt.Fprintf(&b, "%d active brief-v1 brief(s) classified\n\n", rep.Total)

	// Distribution table.
	max := 1
	for _, d := range cynefinDomainOrder {
		if rep.Distribution[d] > max {
			max = rep.Distribution[d]
		}
	}
	fmt.Fprintf(&b, "%-12s %5s  %-20s %s\n", "domain", "count", "share", "diagnostic")
	for _, d := range cynefinDomainOrder {
		n := rep.Distribution[d]
		bar := strings.Repeat("█", n*20/max)
		fmt.Fprintf(&b, "%-12s %5d  %-20s %s\n", d, n, bar, cynefinLever[d])
	}

	// The switch, stated so the reader can act on it.
	b.WriteString("\nswitch: Ordered (clear/complicated) → Theory of Constraints; " +
		"Complex → probe-sense-respond / enabling constraints; Chaotic → act first, then sense.\n")

	// Disorder list.
	if len(rep.Disorder) > 0 {
		fmt.Fprintf(&b, "\nDisorder — %d untagged brief(s), add a `domain:` field:\n", len(rep.Disorder))
		for _, id := range rep.Disorder {
			fmt.Fprintf(&b, "  - %s\n", id)
		}
	}

	// Mismatch — the money diagnostic: Complex work managed with ordered tools.
	if len(rep.Mismatch) > 0 {
		fmt.Fprintf(&b, "\nmismatch — %d complex brief(s) managed with ordered tools (single-answer Verify, no probe/experiment marker):\n", len(rep.Mismatch))
		for _, m := range rep.Mismatch {
			fmt.Fprintf(&b, "  - %s  [%s]\n", m.ID, strings.Join(m.Signals, "; "))
		}
		b.WriteString("  (probe-sense-respond, not plan-verify: add a probe/experiment marker or reclassify the domain.)\n")
	}

	// Drift.
	if len(rep.Drift) > 0 {
		b.WriteString("\ndrift — domain mix of recorded transitions per period:\n")
		fmt.Fprintf(&b, "%-12s", "period")
		for _, d := range cynefinDomainOrder {
			fmt.Fprintf(&b, " %8s", d)
		}
		fmt.Fprintf(&b, " %8s\n", "total")
		var totals []int
		for _, bk := range rep.Drift {
			fmt.Fprintf(&b, "%-12s", bk.Period)
			for _, d := range cynefinDomainOrder {
				fmt.Fprintf(&b, " %8d", bk.Distribution[d])
			}
			fmt.Fprintf(&b, " %8d\n", bk.Transitions)
			totals = append(totals, bk.Transitions)
		}
		fmt.Fprintf(&b, "\ntransitions per period: %s  (%s)\n", sparkline(totals), seriesRange(totals))
	}

	// Complex-domain measures (three-state each; could-not-check names an absent
	// source, never a zero).
	b.WriteString("\ncomplex measures:\n")
	renderMeasure(&b, "learning-velocity", rep.ComplexMeasures.LearningVelocity)
	renderMeasure(&b, "probe-rate", rep.ComplexMeasures.ProbeRate)
	renderMeasure(&b, "surprise", rep.ComplexMeasures.Surprise)

	// State marker line (three-state).
	fmt.Fprintf(&b, "\nstate: %s", rep.State)
	if rep.Reason != "" {
		fmt.Fprintf(&b, " — %s", rep.Reason)
	}
	b.WriteString("\n")
	return b.String()
}

// runCynefin is the --cynefin entrypoint. Same STATUS.md-free discipline as
// --dora/--trend: a pure read, never a gate. Exit 1 only on a hard load error
// (the tree could not be read at all); otherwise exit 0, with the three-state
// verdict carried in the output.
func runCynefin(root, period string, asJSON bool) int {
	streams, _, err := loadStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: cynefin:", err)
		return 1
	}
	historyPath := filepath.Join(root, filepath.FromSlash(historyRelPath))
	history, _ := LoadHistory(historyPath) // missing/unreadable log → nil, not an error

	rep := computeCynefin(streams, history, period)

	if asJSON {
		enc, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "statusgen: cynefin:", err)
			return 1
		}
		fmt.Println(string(enc))
		return 0
	}
	fmt.Print(renderCynefinText(rep))
	return 0
}
