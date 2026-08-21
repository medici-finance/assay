package main

// ladder.go — the adoption-ladder POSITION indicator (methodology-metrics/42,
// #766). Where `--autonomy` (autonomy.go, mm/41) emits the raw behavioral gauges
// as a system, `--ladder` reduces them to ONE watchable number: which adoption
// STEP (0–4) the desk system is actually AT, computed from behavior — never from
// what tooling is installed (#766 addendum point 1).
//
// PUBLIC + DEGRADES, NEVER DEPENDS (mm/42, Ian ruling 2026-08-20 #3). The two
// higher rungs read the mm/40 opmetrics day-file, which is an OPERATOR-MACHINE-
// LOCAL artifact and is ABSENT in any public / adopter checkout. When it is
// absent the ladder does NOT error and does NOT need the private collector: it
// renders an explicit **unmeasured RANGE** ("2–3, operator axes unmeasured
// today") naming the axes it could not read — never a false-precision single
// number, and never a silent zero. This is exactly the three-state degrade
// pattern autonomy.go already uses (a missing source reads "unmeasured", never
// zero), so `--ladder` ships publicly with the private input as a pure
// enhancement, not a requirement.
//
// STEP MAPPING IS A DOCUMENTED LOOKUP, NOT MODEL JUDGMENT (#766). The step is a
// monotone prefix ladder: rung N is reached only when every rung 1..N passes.
// Each rung's predicate is a NAMED threshold const carrying the ladder citation
// (below) — a drive-by threshold edit is how the number gets gamed, so the
// thresholds are the whole semantics and move only via a reviewed change.
//
// ANTI-GAMING. Diagnostic, per-project, never a target or per-person scorecard —
// the same Goodhart clause the autonomy gauges carry.

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Adoption-ladder rung thresholds (methodology-metrics/42, #766 — the ladder
// citation). Each is a NAMED const so a threshold change is a reviewed, visible
// edit: the thresholds ARE the ladder's semantics (anti-gaming). Percentages.
const (
	// Rung 1 — the CI-computable autonomy proxy (merged-PR authorship): the loop
	// is doing a meaningful share of the merges. Public — reads gh, no day-file.
	ladderLoopShareStep1 = 25.0
	// Rung 2 — deterministic-gate share: merges are guarded by code gates, not
	// model judgment alone. Public — reads gh, no day-file.
	ladderGateShareStep2 = 50.0
	// Rung 3 — dispatch-level autonomy (mm/40 day-file): most dispatches are
	// loop-initiated, not operator-initiated. PRIVATE input → unmeasured in a
	// public checkout.
	ladderDispatchShareStep3 = 60.0
	// Rung 4 — token efficiency (mm/40 day-file): the loop is not burning tokens
	// on no-op dispatches. PRIVATE input → unmeasured in a public checkout. Lower
	// is better, so this rung passes when the no-op rate is AT OR BELOW the cap.
	ladderNoopRateStep4Max = 20.0
)

// ladderRung is one rung of the monotone ladder — the behavioral gate that
// confers Step. Measured=false carries a Reason (mirrors AutonomyAxis) and can
// never confer or deny a rung on its own: an unmeasured rung widens the reported
// step into a RANGE rather than fabricating a pass or a fail.
type ladderRung struct {
	Step     int    // the step this rung confers when it passes (1..4)
	Key      string // axis key, for --json and the constraint line
	Name     string // human name, for the one-line binding-constraint render
	Measured bool
	Pass     bool   // meaningful only when Measured
	Detail   string // measured value or the reason it is unmeasured
	Reason   string // reason code when !Measured (e.g. "no-opmetrics-dayfile")
}

// LadderRungResult is the JSON-facing view of a rung.
type LadderRungResult struct {
	Step     int    `json:"step"`
	Key      string `json:"key"`
	Name     string `json:"name"`
	Measured bool   `json:"measured"`
	Pass     bool   `json:"pass"`
	Detail   string `json:"detail,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// LadderReport is the emitted indicator. When Exact, StepLow==StepHigh is the
// position; otherwise [StepLow,StepHigh] is the range the unmeasured axes leave
// open. Constraint is the one-line binding-constraint verdict (mm/18 style):
// what is holding the system at this step (a measured-failed rung) or what
// leaves the position ambiguous (the unmeasured axes).
type LadderReport struct {
	Since          string             `json:"since"`
	Until          string             `json:"until"`
	Generated      string             `json:"generated"`
	Note           string             `json:"note"`
	StepLow        int                `json:"step_low"`
	StepHigh       int                `json:"step_high"`
	Exact          bool               `json:"exact"`
	Constraint     string             `json:"constraint"`
	ConstraintAxes []string           `json:"constraint_axes,omitempty"`
	Rungs          []LadderRungResult `json:"rungs"`
}

const ladderAntiGamingNote = "Adoption-ladder position is DIAGNOSTIC, per-project, for steering by " +
	"intent — never a target, an individual scorecard, or a cross-team comparison (Goodhart's law). The " +
	"step is computed from BEHAVIOR (autonomy ratio, gate share, dispatch autonomy, token efficiency), " +
	"never from installed tooling. An axis with no source reads 'unmeasured' and widens the step to a " +
	"RANGE — never a false-precision single number, never a silent zero."

// computeLadder is the PURE step computation: an ordered rung list → the
// position. No exec, no clock — every value derives from the passed rungs, so
// the whole mapping is testable with hand-built fixtures (#766 acceptance:
// fixture axis inputs → expected step; missing-axis range; constraint naming).
//
// The ladder is a MONOTONE PREFIX: rung i (0-based, conferring step i+1) counts
// only if every earlier rung already counts. From that, two bounds:
//
//   - StepLow  — assume every UNMEASURED rung FAILS: the highest prefix whose
//     rungs are all measured-and-passing. This is the floor we can stand on.
//   - StepHigh — assume every UNMEASURED rung PASSES: the highest prefix with no
//     measured-and-FAILED rung (unmeasured rungs are allowed through). This is
//     the ceiling the missing evidence still permits.
//
// A measured-failed rung stops BOTH walks (it is a real ceiling), so StepHigh
// never climbs past a known failure. When StepLow==StepHigh the position is
// exact; otherwise the gap is exactly the span the unmeasured axes leave open.
func computeLadder(rungs []ladderRung) LadderReport {
	low := 0
	for _, r := range rungs {
		if r.Measured && r.Pass {
			low++
			continue
		}
		break
	}
	high := 0
	for _, r := range rungs {
		if r.Measured && !r.Pass {
			break // a known failure is a hard ceiling
		}
		high++ // measured-pass OR unmeasured (optimistic) both climb
	}

	rep := LadderReport{
		Note:     ladderAntiGamingNote,
		StepLow:  low,
		StepHigh: high,
		Exact:    low == high,
	}
	for _, r := range rungs {
		rep.Rungs = append(rep.Rungs, LadderRungResult{
			Step: r.Step, Key: r.Key, Name: r.Name,
			Measured: r.Measured, Pass: r.Pass, Detail: r.Detail, Reason: r.Reason,
		})
	}

	// Binding constraint (mm/18 one-line verdict style, #766 addendum point 2:
	// the binding constraint IS the progression metric).
	if rep.Exact {
		if low < len(rungs) {
			// The first rung beyond the floor is measured-and-failed (if it were
			// unmeasured, high would exceed low and this would not be exact).
			blk := rungs[low]
			rep.Constraint = fmt.Sprintf("held at %d by: %s", low, blk.Name)
			rep.ConstraintAxes = []string{blk.Key}
		} else {
			rep.Constraint = fmt.Sprintf("at the top rung (%d) — every measured axis clears its threshold", low)
		}
		return rep
	}
	// A range: the rungs in [low, high) are non-failing but include the
	// unmeasured axes that leave the position open. Name them.
	var names, keys []string
	for i := low; i < high && i < len(rungs); i++ {
		if !rungs[i].Measured {
			names = append(names, rungs[i].Name)
			keys = append(keys, rungs[i].Key)
		}
	}
	rep.ConstraintAxes = keys
	rep.Constraint = fmt.Sprintf("held at %d–%d by: %s unmeasured today", low, high, strings.Join(names, ", "))
	return rep
}

// stepLabel renders the position as a single step or a range.
func (rep LadderReport) stepLabel() string {
	if rep.Exact {
		return fmt.Sprintf("%d", rep.StepLow)
	}
	return fmt.Sprintf("%d–%d", rep.StepLow, rep.StepHigh)
}

// renderLadderText renders the one-line indicator plus per-rung detail. The
// literal word "step" always appears (a consumer greps for it), and when any
// axis is unmeasured the literal "unmeasured" appears in the constraint line —
// the public/degraded contract (mm/42 Verify).
func renderLadderText(rep LadderReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Adoption-ladder step: %s — %s\n", rep.stepLabel(), rep.Constraint)
	if !rep.Exact {
		fmt.Fprintf(&b, "  (operator axes unmeasured today — position is a range, not a fabricated point value)\n")
	}
	fmt.Fprintf(&b, "  %s … %s\n", rep.Since, rep.Until)
	for _, r := range rep.Rungs {
		mark := "unmeasured"
		if r.Measured {
			if r.Pass {
				mark = "pass"
			} else {
				mark = "below threshold"
			}
		}
		fmt.Fprintf(&b, "    step %d — %s: %s", r.Step, r.Name, mark)
		if r.Measured {
			if r.Detail != "" {
				fmt.Fprintf(&b, " (%s)", r.Detail)
			}
		} else if r.Reason != "" {
			fmt.Fprintf(&b, " [reason: %s]", r.Reason)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "  %s\n", rep.Note)
	fmt.Fprintf(&b, "  source: methodology-metrics/42 adoption-ladder — behavioral axes (autonomy ratio, gate share, dispatch autonomy, token efficiency); rungs 3–4 read the mm/40 opmetrics day-file (operator-machine-local; unmeasured in a public checkout)\n")
	return b.String()
}

// ladderRungsFromAutonomy builds the four ordered rungs from the SAME inputs
// autonomy.go gathers — reusing the gh authorship/gate reads and the mm/40
// day-file loader with their existing three-state degrade. This is the seam that
// makes `--ladder` inherit the "unmeasured, never zero" discipline for free: a
// missing gh read leaves rungs 1–2 unmeasured; a missing opmetrics day-file
// leaves rungs 3–4 unmeasured, and the ladder renders a range.
func ladderRungsFromAutonomy(in autonomyInputs) []ladderRung {
	rungs := make([]ladderRung, 0, 4)

	// Rung 1 — CI-proxy autonomy (merged-PR authorship loop share). Public.
	r1 := ladderRung{Step: 1, Key: "autonomy_ci_proxy", Name: "autonomy ratio (CI proxy)"}
	if in.AuthorsOK {
		b := bucketAuthors(in.MergedAuthors, in.HumanLogins)
		if b.Total > 0 {
			share := float64(b.LoopInitiated) / float64(b.Total) * 100
			r1.Measured = true
			r1.Pass = share >= ladderLoopShareStep1
			r1.Detail = fmt.Sprintf("%.0f%% loop-initiated (threshold %.0f%%)", share, ladderLoopShareStep1)
		} else {
			r1.Reason = "no-merged-prs"
		}
	} else {
		r1.Reason = "gh-unreadable"
	}
	rungs = append(rungs, r1)

	// Rung 2 — deterministic-gate share. Public.
	r2 := ladderRung{Step: 2, Key: "deterministic_gate_share", Name: "deterministic-gate share"}
	if in.GateOK {
		total := len(in.GateData)
		if total > 0 {
			gated := 0
			for _, p := range in.GateData {
				if hasDeterministicGate(p.CheckNames) {
					gated++
				}
			}
			share := float64(gated) / float64(total) * 100
			r2.Measured = true
			r2.Pass = share >= ladderGateShareStep2
			r2.Detail = fmt.Sprintf("%.0f%% deterministic-gated (threshold %.0f%%)", share, ladderGateShareStep2)
		} else {
			r2.Reason = "no-merged-prs"
		}
	} else {
		r2.Reason = "gh-unreadable"
	}
	rungs = append(rungs, r2)

	// Rung 3 — dispatch-level autonomy (mm/40 day-file). PRIVATE → unmeasured
	// when the opmetrics collector did not run (the public/degrade path).
	r3 := ladderRung{Step: 3, Key: "autonomy_dispatch", Name: "dispatch autonomy (mm/40 day-file)"}
	switch {
	case in.DayFile == nil:
		r3.Reason = "no-opmetrics-dayfile"
	case in.DayFile.Dispatch == nil || in.DayFile.Dispatch.LoopInitiated == nil || in.DayFile.Dispatch.OperatorInitiated == nil:
		r3.Reason = "dayfile-no-dispatch-split"
	default:
		loop := *in.DayFile.Dispatch.LoopInitiated
		op := *in.DayFile.Dispatch.OperatorInitiated
		total := loop + op
		if total == 0 {
			r3.Reason = "dayfile-no-dispatches"
		} else {
			share := float64(loop) / float64(total) * 100
			r3.Measured = true
			r3.Pass = share >= ladderDispatchShareStep3
			r3.Detail = fmt.Sprintf("%.0f%% loop-initiated dispatches (threshold %.0f%%)", share, ladderDispatchShareStep3)
		}
	}
	rungs = append(rungs, r3)

	// Rung 4 — token efficiency / no-op dispatch rate (mm/40 day-file). PRIVATE.
	r4 := ladderRung{Step: 4, Key: "token_efficiency", Name: "token efficiency (mm/40 day-file)"}
	switch {
	case in.DayFile == nil:
		r4.Reason = "no-opmetrics-dayfile"
	case in.DayFile.TokenEfficiency == nil:
		r4.Reason = "dayfile-no-token-block"
	default:
		te := in.DayFile.TokenEfficiency
		if te.NoopDispatches != nil && te.TotalDispatches != nil && *te.TotalDispatches > 0 {
			rate := float64(*te.NoopDispatches) / float64(*te.TotalDispatches) * 100
			r4.Measured = true
			r4.Pass = rate <= ladderNoopRateStep4Max
			r4.Detail = fmt.Sprintf("%.0f%% no-op dispatch rate (cap %.0f%%)", rate, ladderNoopRateStep4Max)
		} else {
			r4.Reason = "dayfile-token-block-empty"
		}
	}
	rungs = append(rungs, r4)

	return rungs
}

// runLadder is the --ladder entrypoint: a self-contained diagnostic sub-command,
// same STATUS.md-free discipline as --autonomy/--dora. It never reads or writes
// STATUS.md. Exit 0 always on a well-formed invocation — a missing (private)
// source is a "unmeasured range", not an error (the public-degrade contract).
func runLadder(root, since string, asJSON bool) int {
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
		sinceT = until.AddDate(0, 0, -defaultAutonomyWindowDays)
	}
	if sinceT.After(until) {
		fmt.Fprintln(os.Stderr, "statusgen: --since is in the future")
		return 1
	}

	in := gatherAutonomyInputs(root, sinceT, until, now)
	rep := computeLadder(ladderRungsFromAutonomy(in))
	rep.Since = sinceT.Format("2006-01-02")
	rep.Until = until.Format("2006-01-02")
	rep.Generated = now.UTC().Format(time.RFC3339)

	if asJSON {
		enc, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "statusgen:", err)
			return 1
		}
		fmt.Println(string(enc))
		return 0
	}
	fmt.Print(renderLadderText(rep))
	return 0
}
