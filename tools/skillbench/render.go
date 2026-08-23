// render.go — the markdown report: the artifact a consuming adoption brief (and
// every future overlay brief) gates adoption on.
//
// The rule, inherited from the house AI-free collector pattern: THE STATE IS RENDERED
// IN THE CELL. A figure that was not measured never renders as a numeral — it
// renders `could-not-check` with the reason, in the same place the number would
// have been. Each metric emits exactly one `delta:` line so the report carries
// an unambiguous per-metric delta plus its `n`.
package main

import (
	"fmt"
	"strings"
)

func renderReport(r report) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Skill-efficacy report — %s — %s\n\n", r.OverlaySlug, r.Date)
	b.WriteString("AI-free reducer over session artifacts (`tools/skillbench`). ")
	b.WriteString("The harness never invokes an agent and never reads GitHub or git; it reduces the committed per-run artifacts of two arms into the deltas below.\n\n")

	// --- arms -------------------------------------------------------------
	b.WriteString("## Arms\n\n")
	b.WriteString("| Arm | Runs |\n|---|---|\n")
	for _, name := range armOrder {
		if r.ArmPresent[name] {
			fmt.Fprintf(&b, "| `%s` | %d |\n", name, r.ArmRuns[name])
		} else {
			fmt.Fprintf(&b, "| `%s` | could-not-check — %s |\n", name, r.ArmNote[name])
		}
	}

	// --- per-metric deltas ------------------------------------------------
	b.WriteString("\n## Per-metric deltas\n\n")
	b.WriteString("Each figure is a mean over the runs that carried it; `n` is that count. ")
	b.WriteString("A metric absent from a run (for example, no usage log) is `could-not-check`, never a measured zero, ")
	b.WriteString("and a delta is emitted only when both arms measured the metric.\n")
	for _, m := range r.Metrics {
		fmt.Fprintf(&b, "\n### %s\n\n", m.Meta.Label)
		fmt.Fprintf(&b, "- with-overlay: %s\n", armCell(m.With, m.Meta))
		fmt.Fprintf(&b, "- without-overlay: %s\n", armCell(m.Without, m.Meta))
		fmt.Fprintf(&b, "- delta: %s\n", deltaCell(m.Delta, m.Meta))
	}

	// --- verdict ----------------------------------------------------------
	b.WriteString("\n## Verdict — input to an adoption decision, not an adoption\n\n")
	b.WriteString("This report states per-metric deltas and their `n`. The adopt/hold decision belongs to the consuming adoption brief; the harness draws no conclusion of its own.\n\n")
	fmt.Fprintf(&b, "- Safety floor (task-check pass rate): %s\n", r.SafetyNote)
	fmt.Fprintf(&b, "- Cost-side movement (overlay vs baseline): %s\n", costDirectionLine(r))

	return b.String()
}

// armCell renders one arm's aggregate of one metric.
func armCell(a armMetric, meta metricMeta) string {
	if a.State == cellMeasured {
		return fmt.Sprintf("%s (n=%d/%d)", fmtNum(a.Mean, meta), a.N, a.Total)
	}
	note := a.Note
	if note == "" {
		note = "no measured runs"
	}
	return fmt.Sprintf("could-not-check (0/%d runs) — %s", a.Total, note)
}

// deltaCell renders one metric's delta, with its direction relative to what is
// better for that metric.
func deltaCell(d deltaMetric, meta metricMeta) string {
	if d.State != cellMeasured {
		return fmt.Sprintf("could-not-check — %s", d.Note)
	}
	dir := directionWord(d.Abs, meta)
	abs := fmtSigned(d.Abs, meta)
	if d.PctState == cellMeasured {
		return fmt.Sprintf("%s (%+.1f%%)%s", abs, d.Pct, dir)
	}
	return fmt.Sprintf("%s (%% undefined — baseline mean is 0)%s", abs, dir)
}

func directionWord(abs float64, meta metricMeta) string {
	switch {
	case abs == 0:
		return " (no change)"
	case abs < 0 && meta.LowerBetter, abs > 0 && !meta.LowerBetter:
		return " (improvement)"
	default:
		return " (regression vs baseline)"
	}
}

// fmtNum renders a measured mean.
func fmtNum(v float64, meta metricMeta) string {
	switch {
	case meta.Rate:
		return fmt.Sprintf("%.0f%%", v*100)
	case meta.Float:
		return fmt.Sprintf("%.4f", v)
	default:
		return fmt.Sprintf("%.1f", v)
	}
}

// fmtSigned renders a signed delta. A rate delta is in percentage points.
func fmtSigned(v float64, meta metricMeta) string {
	switch {
	case meta.Rate:
		return fmt.Sprintf("%+.0f pp", v*100)
	case meta.Float:
		return fmt.Sprintf("%+.4f", v)
	default:
		return fmt.Sprintf("%+.1f", v)
	}
}

// costDirectionLine summarizes whether the cost-side metrics (diff, files,
// tokens, cost, wall) moved down with the overlay, for those that were
// measured in both arms.
func costDirectionLine(r report) string {
	var improved, regressed, cnc int
	for _, m := range r.Metrics {
		if m.Meta.Rate {
			continue // the safety floor is reported on its own line
		}
		switch {
		case m.Delta.State != cellMeasured:
			cnc++
		case m.Delta.Abs < 0: // lower is better for all cost-side metrics
			improved++
		case m.Delta.Abs > 0:
			regressed++
		}
	}
	return fmt.Sprintf("%d improved, %d regressed, %d could-not-check (of the 5 cost-side metrics)",
		improved, regressed, cnc)
}
