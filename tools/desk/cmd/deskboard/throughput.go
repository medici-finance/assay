package main

// throughput.go — the SIGNAL half of "can the desk tell a bottlenecked desk to widen?".
//
// The knob (deskroster set --role LOOP --width N) is useless without an instrument that
// says WHICH desk to move, and says it the same way twice so a coordinator can compare two
// consecutive ticks. This verb is that instrument: for each stage of the pipeline it reports
// how much work is waiting (DEPTH) against how many agents that stage is allowed to run
// (SLOTS), and names the stage with the worst ratio as the bottleneck.
//
// IT DERIVES, IT DOES NOT RE-PARSE. Every depth here comes from a report deskboard already
// builds — the dispatch queue from `dispatch`, the review queue from `actions`, the
// verification queue from `awaiting`. Nothing reads STATUS.md, and nothing reimplements a
// classification. A second copy of "which PRs need review" would eventually disagree with
// the one the review desk actually dispatches from, and the ratio would then be describing
// a queue nobody works.
//
// SLOTS ARE CAPACITY, NOT OCCUPANCY. The denominator is the loop's resolved pool WIDTH — the
// number of agents that window is permitted to run — not a live count of agents currently
// working. Nothing in the tree counts the latter (the roster records open PRs per session,
// not in-flight agents), and inventing an occupancy figure would make the ratio look precise
// while being a guess. Capacity is also the honest denominator for the decision this signal
// exists to inform, which is "should this stage be ALLOWED more agents?".
//
// BLIND IS NEVER GREEN. A stage whose depth could not be read is reported as could-not-check
// and is EXCLUDED from bottleneck selection — never counted as zero, which would present an
// unreadable queue as a drained one and steer the desk to widen some other stage. The
// bottleneck line always states how many of the four stages it actually read.

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// stageRow is one pipeline stage's throughput reading.
//
// Depth and Ratio are pointers so that ABSENT is representable and distinct from zero. A
// stage that could not be read must not serialise as `"depth": 0` — that is the exact
// conflation this whole file is arranged to avoid.
type stageRow struct {
	Stage string `json:"stage"`
	// Loop is the canonical loop name whose width sizes this stage — the value a
	// coordinator passes to `deskroster set --role <loop> --width N`. Printing it means
	// the signal names the knob it wants moved rather than leaving the reader to map
	// "review" onto a loop name.
	Loop string `json:"loop"`
	// Depth is the queue waiting on this stage; nil when it could not be read.
	Depth *int `json:"depth"`
	// Slots is the loop's resolved pool WIDTH (capacity, not live occupancy), and
	// SlotsSource says where that number came from — shipped default, a width a desk set,
	// or a stored width a ceiling clamped.
	Slots       int    `json:"slots"`
	SlotsSource string `json:"slotsSource"`
	// Ratio is Depth/Slots; nil whenever Depth is nil. It is the comparable number, since
	// a depth of 8 means something different against 1 slot than against 8.
	Ratio *float64 `json:"ratio"`
	// MaxSlots is the widest this stage may be set to right now, so the reader can tell
	// "this stage is the bottleneck AND can be widened" from "this stage is the bottleneck
	// and is already at its ceiling" — two situations with completely different responses.
	MaxSlots  int    `json:"maxSlots"`
	BoundBy   string `json:"boundBy"`
	Blind     string `json:"blind,omitempty"`
	DepthNote string `json:"depthNote"`
}

type throughputReport struct {
	Header
	Stages []stageRow `json:"stages"`
	// Bottleneck names the stage with the highest ratio among the stages that were READ.
	// Empty when nothing was readable or every readable stage has an empty queue.
	Bottleneck string `json:"bottleneck"`
	// StagesRead / StagesTotal make the coverage explicit on every emission, so a
	// bottleneck computed from two of four stages can never be read as the whole picture.
	StagesRead  int `json:"stagesRead"`
	StagesTotal int `json:"stagesTotal"`
	// Roots states the ROOT-axis coverage (the dispatch and verify depths are read from
	// statusgen roots), beside the header's repo-axis `scope`. This verb spans both axes,
	// so it states both: omitting either would claim a coverage it does not have.
	Roots []deskkit.RootConfig `json:"roots"`
	// Advice is the one line a coordinator acts on — either the exact deskroster command
	// to widen the bottleneck, or why there is nothing to do.
	Advice string `json:"advice"`
}

// intp / f64p are the "absent is not zero" constructors.
func intp(n int) *int         { return &n }
func f64p(f float64) *float64 { return &f }

// slotsFor resolves one stage's capacity from the SAME single reader the loop itself uses
// (deskkit.ResolvedWidth), so the denominator of this ratio and the pool it describes cannot
// disagree. A width that cannot be resolved leaves the row blind rather than defaulting.
func slotsFor(row *stageRow) {
	w, source, err := deskkit.ResolvedWidth(row.Loop)
	if err != nil {
		row.Blind = appendBlind(row.Blind, "slots: "+err.Error())
		return
	}
	row.Slots, row.SlotsSource = w, source
	if max, why, merr := deskkit.MaxWidth(row.Loop); merr == nil {
		row.MaxSlots, row.BoundBy = max, why
	} else {
		row.Blind = appendBlind(row.Blind, "ceiling: "+merr.Error())
	}
}

func appendBlind(existing, add string) string {
	if existing == "" {
		return add
	}
	return existing + "; " + add
}

// finish computes the ratio once depth and slots are both known. A row with no depth keeps a
// nil ratio: it is not comparable, and a stage that is not comparable cannot be the
// bottleneck.
func (r *stageRow) finish() {
	if r.Depth == nil || r.Slots < 1 {
		return
	}
	r.Ratio = f64p(float64(*r.Depth) / float64(r.Slots))
}

// cmdThroughput is the per-stage depth-vs-slots signal.
//
// It reuses the existing report builders rather than re-deriving their populations. Each is
// allowed to FAIL independently: this verb's job is to compare stages, and one unreadable
// stage must not deny the coordinator a reading on the other three. Every failure becomes a
// named blind row, never a zero.
func cmdThroughput(hdr Header, mergeNowThreshold, unreviewedThreshold time.Duration) (*Report, error) {
	// Repo-axis coverage. This verb sweeps deskkit.AllowedRepos for its review depth, so it
	// carries `scope` like every other repo-sweeping verb (#359).
	hdr.Scope = boardScope()

	rep := throughputReport{Header: hdr, StagesTotal: 4, Roots: []deskkit.RootConfig{}}

	dispatchStage := stageRow{
		Stage: "dispatch", Loop: "worker-desk",
		DepthNote: "eligible, unclaimed Next-up briefs across every configured root " +
			"(statusgen's own selection, via `deskboard dispatch`) — the work with nobody on it",
	}
	reviewStage := stageRow{
		Stage: "review", Loop: "pr-review-desk",
		DepthNote: "open PRs the classifier puts at " + actNeedsReview + " or " + actReReview +
			" — no reviewer verdict at the current head (via `deskboard actions`)",
	}
	verifyStage := stageRow{
		Stage: "verify", Loop: "verify-desk",
		DepthNote: "briefs at `implemented` awaiting a verifier across every configured root " +
			"(via `deskboard awaiting`)",
	}
	intakeStage := stageRow{
		Stage: "intake", Loop: "intake-desk",
		DepthNote: "the raw-intake queue — NOT read here: it is derived by `issueboard intake`, " +
			"a separate binary whose loader is not importable from this one",
	}

	// ---- dispatch depth ----
	if drep, err := cmdDispatch(hdr, "dispatch"); err != nil {
		dispatchStage.Blind = appendBlind(dispatchStage.Blind, "depth: "+err.Error())
	} else if dv, ok := drep.value.(dispatchReport); ok {
		// Eligible, not Shown: the caps that hold a brief back are a throughput fact about
		// the BOARD, not about the pool. Sizing a pool against `shown` would make the pool
		// look adequate precisely when the queue is deepest.
		dispatchStage.Depth = intp(dv.Eligible)
		rep.Roots = dv.Roots
		if len(dv.ClaimsDegraded) > 0 {
			dispatchStage.Blind = appendBlind(dispatchStage.Blind,
				fmt.Sprintf("depth is an UNFILTERED superset: the claim read did not run for %d root(s)",
					len(dv.ClaimsDegraded)))
		}
	} else {
		dispatchStage.Blind = appendBlind(dispatchStage.Blind, "depth: unexpected dispatch report shape")
	}

	// ---- review depth ----
	ahdr := hdr
	if arep, err := cmdActions(&ahdr, mergeNowThreshold, unreviewedThreshold); err != nil {
		reviewStage.Blind = appendBlind(reviewStage.Blind, "depth: "+err.Error())
	} else if av, ok := arep.value.(actionsReport); ok {
		n := 0
		for _, row := range av.Rows {
			if row.Action == actNeedsReview || row.Action == actReReview {
				n++
			}
		}
		reviewStage.Depth = intp(n)
	} else {
		reviewStage.Blind = appendBlind(reviewStage.Blind, "depth: unexpected actions report shape")
	}

	// ---- verify depth ----
	if nrep, err := cmdAwaiting(hdr, "awaiting"); err != nil {
		verifyStage.Blind = appendBlind(verifyStage.Blind, "depth: "+err.Error())
	} else if nv, ok := nrep.value.(nextupReport); ok {
		n := 0
		for _, row := range nv.Rows {
			// `awaiting` carries implemented AND verified; only the first is work a
			// verifier has to do. Counting both would inflate the queue with rows whose
			// verification already happened.
			if row.Status == "implemented" {
				n++
			}
		}
		verifyStage.Depth = intp(n)
		if len(rep.Roots) == 0 {
			rep.Roots = nv.Roots
		}
	} else {
		verifyStage.Blind = appendBlind(verifyStage.Blind, "depth: unexpected awaiting report shape")
	}

	// ---- intake depth: could-not-check BY CONSTRUCTION ----
	// The intake queue is derived by `issueboard`, whose loader is `package main` in another
	// binary. Shelling out to it, or lifting it into internal/, is a real change to this
	// board's dependency surface and is deliberately NOT smuggled in behind a throughput
	// verb. It is reported as unread rather than omitted: a stage missing from the list
	// reads as a stage with no queue.
	intakeStage.Blind = appendBlind(intakeStage.Blind,
		"depth: not read — `issueboard intake` owns this population and is a separate binary")

	for _, s := range []*stageRow{&dispatchStage, &reviewStage, &verifyStage, &intakeStage} {
		slotsFor(s)
		s.finish()
		rep.Stages = append(rep.Stages, *s)
	}

	// Bottleneck: the highest ratio among stages that were actually READ. Ties break on
	// stage name so two consecutive ticks over identical state name the same stage — the
	// desk's rule is "the same stage over two ticks", and a signal that alternated between
	// tied stages would never satisfy it.
	best := -1.0
	for _, s := range rep.Stages {
		if s.Ratio == nil {
			continue
		}
		rep.StagesRead++
		if *s.Ratio > best || (*s.Ratio == best && s.Stage < rep.Bottleneck) {
			best, rep.Bottleneck = *s.Ratio, s.Stage
		}
	}
	if best <= 0 {
		// Every readable stage has an empty queue. That is not a bottleneck, and naming one
		// would have the desk widening a pool with nothing to put in it.
		rep.Bottleneck = ""
	}
	rep.Advice = throughputAdvice(&rep)

	return &Report{
		value:  rep,
		detail: throughputDetail(&rep),
		render: func(w io.Writer) { renderThroughput(w, &rep) },
	}, nil
}

// throughputAdvice is the ONE line a coordinator acts on. It states the exact command when
// widening is available, and — just as important — states plainly when it is not, so a
// bottleneck at its ceiling is never answered by a width change that would be refused.
func throughputAdvice(rep *throughputReport) string {
	if rep.StagesRead == 0 {
		return "COULD-NOT-CHECK: no stage's depth was readable, so nothing here says which desk is the " +
			"bottleneck. Blind is not idle — fix the reads before acting on this signal."
	}
	if rep.Bottleneck == "" {
		return fmt.Sprintf("no bottleneck: every one of the %d readable stage(s) has an empty queue. "+
			"Widening a pool with nothing to put in it buys nothing.", rep.StagesRead)
	}
	for _, s := range rep.Stages {
		if s.Stage != rep.Bottleneck {
			continue
		}
		if s.Slots >= s.MaxSlots {
			return fmt.Sprintf("%s is the deepest stage (%d waiting / %d slots) but is ALREADY AT its "+
				"ceiling of %d, bound by %s. Widening is not the lever here — the queue has to drain, "+
				"or the bound has to change, and the bound is a separate decision.",
				s.Stage, *s.Depth, s.Slots, s.MaxSlots, s.BoundBy)
		}
		return fmt.Sprintf("%s is the bottleneck (%d waiting / %d slots = %.2f). It may widen to %d "+
			"(bound by %s): deskroster set --role %s --width %d",
			s.Stage, *s.Depth, s.Slots, *s.Ratio, s.MaxSlots, s.BoundBy, s.Loop, s.MaxSlots)
	}
	return ""
}

func throughputDetail(rep *throughputReport) string {
	parts := make([]string, 0, len(rep.Stages))
	for _, s := range rep.Stages {
		if s.Depth == nil {
			parts = append(parts, s.Stage+"=could-not-check")
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%d/%d", s.Stage, *s.Depth, s.Slots))
	}
	sort.Strings(parts)
	b := rep.Bottleneck
	if b == "" {
		b = "none"
	}
	return fmt.Sprintf("throughput %s bottleneck=%s (%d/%d stages read)",
		strings.Join(parts, " "), b, rep.StagesRead, rep.StagesTotal)
}

func renderThroughput(w io.Writer, rep *throughputReport) {
	fmt.Fprintf(w, "asOf %s\n", rep.Header.AsOf)
	fmt.Fprintf(w, "%-9s %-15s %-7s %-6s %-7s %-5s %s\n",
		"STAGE", "LOOP", "DEPTH", "SLOTS", "RATIO", "MAX", "NOTE")
	for _, s := range rep.Stages {
		depth, ratio := "n/a", "n/a"
		if s.Depth != nil {
			depth = fmt.Sprintf("%d", *s.Depth)
		}
		if s.Ratio != nil {
			ratio = fmt.Sprintf("%.2f", *s.Ratio)
		}
		note := s.SlotsSource
		if s.Blind != "" {
			note = "COULD-NOT-CHECK — " + s.Blind
		}
		fmt.Fprintf(w, "%-9s %-15s %-7s %-6d %-7s %-5d %s\n",
			s.Stage, s.Loop, depth, s.Slots, ratio, s.MaxSlots, note)
	}
	fmt.Fprintf(w, "\nread %d of %d stages\n", rep.StagesRead, rep.StagesTotal)
	fmt.Fprintf(w, "%s\n", rep.Advice)
	for _, r := range rep.Roots {
		fmt.Fprintf(w, "root: %s (%s)\n", r.Path, r.Repo)
	}
	renderScopeLine(w, rep.Header.Scope)
}
