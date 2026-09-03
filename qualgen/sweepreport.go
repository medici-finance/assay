package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/medici-finance/assay/qualgen/verifier"
)

// sweepreport.go is leg 3 of the sweep lane: the evidenced report emitter. It
// renders one markdown report per run from the suspects + verdicts the
// orchestrator assembled. The report is the human-triage surface, and its rules
// are the lane's honesty contract:
//
//   - the header states the target SHA, the per-category tool + three-state
//     measure-state (a could-not-measure category is NAMED as such, never
//     rendered as clean);
//   - actionable sections carry ONLY evidence-bearing verdicts (confirmed /
//     needs-human with a non-empty pointer) — the emitter's evidence gate has
//     already reclassified the rest;
//   - false-positive verdicts render in a SUPPRESSED section WITH their reasons;
//   - could-not-verify items are LISTED as such, never dropped.
//
// The report NAMES, per actionable suspect, its file path and the rule that
// flagged it, so a reader can dereference the evidence — the report is not a
// well-formed document that lost the trail between legs.

// SweepReportName returns the report filename for a run date.
func SweepReportName(runDate string) string {
	return fmt.Sprintf("report-%s.md", runDate)
}

// writeSweepReport renders the run and writes it under the tracking root's
// sweep subdir, returning the path written. It is the only leg that writes a
// human-facing file; the append-only tables are written by the orchestrator.
func writeSweepReport(store *Store, run *SweepRun) (string, error) {
	dir := filepath.Join(store.dir(), sweepSubdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, SweepReportName(run.RunDate))
	if err := os.WriteFile(path, []byte(renderSweepReport(run)), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// renderSweepReport renders the whole report to a string (kept separate from
// the write so tests assert on content without touching the filesystem).
func renderSweepReport(run *SweepRun) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Code-slop sweep report — %s\n\n", run.RunDate)
	fmt.Fprintf(&b, "- Target SHA: `%s`\n", run.TargetSHA)
	fmt.Fprintf(&b, "- Suspects this run: %d new / %d persistent / %d cleared\n\n",
		len(run.New), len(run.Persistent), len(run.Cleared))

	// Per-category coverage — the three-state honesty header.
	b.WriteString("## Coverage by category\n\n")
	b.WriteString("| Category | Tool | Measure-state |\n|---|---|---|\n")
	for _, cat := range run.Categories {
		tool := "(no tool configured)"
		if tc, ok := run.Config.Tools[cat.Category]; ok && len(tc.Command) > 0 {
			tool = mdInline(strings.Join(tc.Command, " "))
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", mdInline(cat.Category), tool, measureStateCell(cat.State))
	}
	b.WriteString("\n> Version probing is intentionally not performed offline; the Tool column records the configured command. A `could-not-measure` category was NOT looked at — it is never the same as `measured-zero`.\n\n")

	// New — actionable, evidence-bearing.
	writeSuspectSection(&b, "New suspects — actionable", run.New, run.Verdicts, run.Reclassified, sectionActionable)
	// Persistent — actionable, carried-forward verdicts.
	writeSuspectSection(&b, "Persistent suspects — actionable", run.Persistent, run.Verdicts, run.Reclassified, sectionActionable)
	// Suppressed false positives (across new + persistent).
	writeSuspectSection(&b, "Suppressed — false positives", append(append([]verifier.Suspect{}, run.New...), run.Persistent...), run.Verdicts, run.Reclassified, sectionFalsePositive)
	// Could-not-verify (across new + persistent) — listed, never dropped.
	writeSuspectSection(&b, "Could-not-verify", append(append([]verifier.Suspect{}, run.New...), run.Persistent...), run.Verdicts, run.Reclassified, sectionCouldNotVerify)
	// Cleared — suspects gone from the current tree.
	writeClearedSection(&b, run.Cleared)

	return b.String()
}

// sectionFilter selects which verdict classes a section renders.
type sectionFilter int

const (
	sectionActionable sectionFilter = iota
	sectionFalsePositive
	sectionCouldNotVerify
)

// writeSuspectSection renders one section, filtered by verdict class. The
// actionable filter admits ONLY confirmed / needs-human with a non-empty
// evidence pointer — the emitter has already reclassified evidence-free ones to
// could-not-verify, but the pointer is re-checked here as a belt-and-braces on
// the report boundary.
func writeSuspectSection(b *strings.Builder, title string, suspects []verifier.Suspect, verdicts map[string]verifier.Verdict, reclassified map[string]bool, filter sectionFilter) {
	// Stable order for deterministic reports.
	ordered := append([]verifier.Suspect{}, suspects...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].File != ordered[j].File {
			return ordered[i].File < ordered[j].File
		}
		return ordered[i].LineStart < ordered[j].LineStart
	})

	var rows []string
	for _, s := range ordered {
		v, ok := verdicts[s.Fingerprint]
		if !ok {
			// No verdict at all is could-not-verify by omission.
			if filter == sectionCouldNotVerify {
				rows = append(rows, fmt.Sprintf("- `%s` L%d — category %s, rule %s — could-not-verify: no verdict produced\n",
					s.File, s.LineStart, s.Category, mdInline(nonEmpty(s.Rule))))
			}
			continue
		}
		switch filter {
		case sectionActionable:
			if v.Class.Actionable() && strings.TrimSpace(v.EvidencePointer) != "" {
				rows = append(rows, fmt.Sprintf("- **%s** `%s` L%d — category %s, rule %s\n  - evidence: %s\n  - rationale: %s\n",
					v.Class, s.File, s.LineStart, s.Category, mdInline(nonEmpty(s.Rule)),
					mdInline(v.EvidencePointer), mdInline(nonEmpty(v.Rationale))))
			}
		case sectionFalsePositive:
			if v.Class == verifier.ClassFalsePositive {
				rows = append(rows, fmt.Sprintf("- `%s` L%d — category %s, rule %s — suppressed: %s\n",
					s.File, s.LineStart, s.Category, mdInline(nonEmpty(s.Rule)), mdInline(nonEmpty(v.Rationale))))
			}
		case sectionCouldNotVerify:
			if v.Class == verifier.ClassCouldNotVerify {
				note := nonEmpty(v.Rationale)
				if reclassified[s.Fingerprint] {
					note = "reclassified by the evidence gate: " + note
				}
				rows = append(rows, fmt.Sprintf("- `%s` L%d — category %s, rule %s — could-not-verify: %s\n",
					s.File, s.LineStart, s.Category, mdInline(nonEmpty(s.Rule)), mdInline(note)))
			}
		}
	}

	fmt.Fprintf(b, "## %s\n\n", title)
	if len(rows) == 0 {
		b.WriteString("_none._\n\n")
		return
	}
	for _, r := range rows {
		b.WriteString(r)
	}
	b.WriteString("\n")
}

// writeClearedSection lists suspects that were adjudicated in a prior run but
// are absent from the current tree.
func writeClearedSection(b *strings.Builder, cleared []SweepSuspect) {
	ordered := append([]SweepSuspect{}, cleared...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].File != ordered[j].File {
			return ordered[i].File < ordered[j].File
		}
		return ordered[i].LineStart < ordered[j].LineStart
	})
	b.WriteString("## Cleared suspects\n\n")
	if len(ordered) == 0 {
		b.WriteString("_none._\n\n")
		return
	}
	for _, s := range ordered {
		fmt.Fprintf(b, "- `%s` L%d — category %s, rule %s — gone from the current tree (first seen %s)\n",
			s.File, s.LineStart, s.Category, mdInline(nonEmpty(s.Rule)), s.FirstSeen)
	}
	b.WriteString("\n")
}

// measureStateCell renders a category's three-state measure for the coverage
// table. A could-not-measure carries its reason; a measured count is shown.
func measureStateCell(m Measure[int]) string {
	switch m.State {
	case StateMeasured:
		return fmt.Sprintf("measured (%d suspect(s))", m.Value)
	case StateMeasuredZero:
		return "measured-zero (0 suspects)"
	case StateCouldNotMeasure:
		return "could-not-measure — " + mdInline(m.Reason)
	default:
		return "unknown"
	}
}

// nonEmpty returns a placeholder for an empty field so a table cell is never
// blank.
func nonEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

// mdInline neutralizes pipes and newlines so a value stays inside its markdown
// cell/line.
func mdInline(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}
