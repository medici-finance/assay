package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// Durable is the sink that makes a verify Result durable. It is an interface so the default
// (a SAFE dry-run emitter — the default must NEVER push to main; the autonomous cutover is
// BLOCKED-ON-HUMAN) can be swapped for the real git-pushing implementation at cutover, and so
// tests can assert exactly what Land does with a structured Result.
type Durable interface {
	// WriteEvidenceAndFlip writes evidence into the brief's ## Evidence section; when flip is
	// true it also flips the README status implemented -> verified (dated + runner-attributed)
	// and commits straight to main via the push-race retry. flip=false writes Evidence only.
	WriteEvidenceAndFlip(it loopengine.Item, evidence string, flip bool) error
	// Checkpoint opens a human-review checkpoint PR (irreversible path: Evidence written, NO
	// model status flip — the human's approve+merge IS the flip).
	Checkpoint(it loopengine.Item, evidence string) (string, error)
	// RouteHuman labels an issue / opens a checkpoint for a risk-flagged (TierHuman) item that
	// a model may not sign off; the drain continues.
	RouteHuman(it loopengine.Item) (string, error)
	// FileBug files a `bug` issue for a failed or unlandable verify and returns its URL.
	FileBug(it loopengine.Item, detail string) (string, error)
}

// Land consumes ONLY the structured Result (arch doc §4) — no free-text verdict reaches a
// board cell. It renders dated, runner-attributed Evidence rows from Result.Rows and routes:
//
//   - ROUTE_HUMAN (risk-flagged, never dispatched) -> RouteHuman, drain continues;
//   - FAIL -> record the failing Evidence (a change failure is filed, not buried) + FileBug,
//     leave the brief at implemented;
//   - irreversible + PASS -> Evidence written, NO flip, checkpoint PR for a human;
//   - PASS -> Evidence + status flip implemented->verified (push-race retry in code);
//   - BLOCKED / anything unlandable -> FileBug and continue (FILE don't ask, DRAIN don't wait).
func (v *VerifyLoop) Land(r loopengine.Result) error {
	d := v.durable()

	if r.Verdict == loopengine.VerdictRouteHuman {
		_, err := d.RouteHuman(r.Item)
		return err
	}

	evidence := renderEvidence(r, v.now())

	switch r.Verdict {
	case loopengine.VerdictPass:
		if r.Item.Risk.Irreversible {
			// Evidence recorded, NO flip; checkpoint PR carries the review packet for a human.
			if err := d.WriteEvidenceAndFlip(r.Item, evidence, false); err != nil {
				_, _ = d.FileBug(r.Item, "irreversible verify: could not write Evidence: "+err.Error())
				return err
			}
			_, err := d.Checkpoint(r.Item, evidence)
			return err
		}
		// Reversible PASS: Evidence + flip, committed straight to main via the push-race loop.
		if err := d.WriteEvidenceAndFlip(r.Item, evidence, true); err != nil {
			// A land that cannot succeed files a bug and continues — the drain never wedges.
			_, _ = d.FileBug(r.Item, "verify PASS but landing failed: "+err.Error())
			return err
		}
		return nil

	case loopengine.VerdictFail:
		// Record the failing Evidence AND file the change failure; brief stays implemented.
		_ = d.WriteEvidenceAndFlip(r.Item, evidence, false)
		_, err := d.FileBug(r.Item, "VERIFY FAIL (change failure): "+failSummary(r))
		return err

	default: // BLOCKED / NEEDS_CONTEXT / unknown
		_, err := d.FileBug(r.Item, "verify could not complete ("+r.Verdict+")")
		return err
	}
}

// renderEvidence builds the ## Evidence rows from the STRUCTURED Result — one row per Verify
// item, dated + attributed to the runner the engine dispatched, never a bare ✓, never free
// text. This is the whole integrity point: the board cell is derived from observed
// command/exit/output, not an operator's transcription.
func renderEvidence(r loopengine.Result, date string) string {
	runner := r.RunnerID
	if runner == "" {
		runner = "unknown-runner"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "| # | Command | Exit | Output | When | Runner |\n")
	fmt.Fprintf(&b, "|---|---------|------|--------|------|--------|\n")
	for i, row := range r.Rows {
		out := strings.ReplaceAll(strings.TrimSpace(row.Output), "|", `\|`)
		out = strings.ReplaceAll(out, "\n", " ")
		fmt.Fprintf(&b, "| %d | `%s` | %d | %s | %s | %s |\n",
			i+1, row.Command, row.Exit, out, date, runner)
	}
	fmt.Fprintf(&b, "\nVerdict: %s — %s %s\n", r.Verdict, date, runner)
	return b.String()
}

func failSummary(r loopengine.Result) string {
	for _, row := range r.Rows {
		if row.Exit != 0 {
			return fmt.Sprintf("`%s` exited %d: %s", row.Command, row.Exit, strings.TrimSpace(row.Output))
		}
	}
	return "see recorded Evidence"
}

func (v *VerifyLoop) now() string {
	if v.Now != nil {
		return v.Now().UTC().Format("2006-01-02")
	}
	return time.Now().UTC().Format("2006-01-02")
}
