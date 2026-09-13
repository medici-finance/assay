package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Options drives one Disable run.
type Options struct {
	Root    string   // tree root to discover component.yaml under
	Target  string   // the component id to disable
	Cascade []string // dependent component ids to disable first, in order
	DryRun  bool     // compute and describe the plan; write nothing
}

// StepResult is one line of the executed (or previewed) plan.
type StepResult struct {
	Component string
	StepID    string
	Boundary  string // "inside" | "outside"
	Kind      string // "reversed" | "would-reverse" | "human-checklist"
	Detail    string
}

// Plan is the full result of a Disable run: every component actually touched
// (cascade first, then the target), in order, and every step taken or
// described within each.
type Plan struct {
	Target  string
	Cascade []string
	Steps   []StepResult
}

// Disable replays component.yaml's reverses for Target — and, if Cascade
// names dependents, for each of those first — in LIFO order of each
// manifest's own apply list (component-model.md §4/§7). It refuses (and
// changes nothing) when:
//
//   - Target has no manifest under Root;
//   - a name in Cascade has no manifest under Root;
//   - after peeling off Cascade and Target in the given order, some OTHER
//     still-present component would be left requiring a key one of the
//     peeled components provides (a stranded dependent, unless it too is
//     named in Cascade, in the right order);
//   - an inside step has no registered automatic reverse (executors.go) —
//     deskdisable never guesses one.
//
// The safety check runs to completion BEFORE any step of any component is
// executed, so a refusal — for any component in the order — leaves the whole
// tree untouched, not just the component it names.
func Disable(opts Options) (*Plan, error) {
	manifests, err := discoverManifests(opts.Root)
	if err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf("could not read manifests under %s: %v", opts.Root, err), err)
	}
	all := byID(manifests)

	if _, ok := all[opts.Target]; !ok {
		return nil, deskkit.Refused(fmt.Sprintf("component %q has no manifest under %s", opts.Target, opts.Root))
	}

	order := append(append([]string{}, opts.Cascade...), opts.Target)

	present := make(map[string]bool, len(all))
	for id := range all {
		present[id] = true
	}

	// Safety pass: verify the WHOLE order is strandless before touching
	// anything. Each entry, once notionally removed, must leave no remaining
	// component depending on a key it provided.
	for _, id := range order {
		m, ok := all[id]
		if !ok {
			return nil, deskkit.Refused(fmt.Sprintf("--cascade names %q, which has no manifest under %s", id, opts.Root))
		}
		present[id] = false
		if deps := dependentsOf(m, all, present); len(deps) > 0 {
			return nil, deskkit.Refused(fmt.Sprintf(
				"refusing to disable %s: still required by %s (provides %s); pass --cascade with the dependent(s) listed first, in reverse dependency order",
				id, strings.Join(deps, ", "), strings.Join(m.Provides, ", ")))
		}
	}

	// Second safety pass, still before any write: every inside step of every
	// component in the order must have a registered automatic reverse. This
	// runs to completion first so a missing executor on, say, the SECOND
	// component in a cascade never leaves the FIRST partially reversed —
	// deskinstall's own "verify every component before placing any of them"
	// shape (tools/desk/cmd/deskinstall/install.go), applied to removal.
	for _, id := range order {
		m := all[id]
		for _, step := range m.Apply {
			switch step.Boundary {
			case "inside":
				if _, ok := insideExecutors[execKey(m.Component, step.ID)]; !ok {
					return nil, deskkit.Refused(fmt.Sprintf(
						"%s: inside step %q has no registered automatic reverse — refusing before touching anything, rather than guess (component-model.md §4); manifest inverse text: %q",
						m.Component, step.ID, step.Inverse))
				}
			case "outside":
				// No pre-flight requirement: an outside step is always safe to
				// describe (a printed checklist line), never executed.
			default:
				return nil, deskkit.Refused(fmt.Sprintf("%s: apply step %q has unknown boundary %q", m.Component, step.ID, step.Boundary))
			}
		}
	}

	ledger, err := deskkit.ReadLedger(opts.Root)
	if err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf("could not read the ledger under %s: %v", opts.Root, err), err)
	}

	plan := &Plan{Target: opts.Target, Cascade: opts.Cascade}
	for _, id := range order {
		steps, err := disableOne(opts, all[id], ledger)
		plan.Steps = append(plan.Steps, steps...)
		if err != nil {
			return plan, err
		}
	}
	return plan, nil
}

// disableOne replays one manifest's own apply steps in LIFO (reverse) order.
func disableOne(opts Options, m *manifest, ledger []deskkit.LedgerEntry) ([]StepResult, error) {
	var results []StepResult
	entries := deskkit.LedgerForComponent(ledger, m.Component)

	for i := len(m.Apply) - 1; i >= 0; i-- {
		step := m.Apply[i]
		switch step.Boundary {
		case "outside":
			results = append(results, StepResult{
				Component: m.Component,
				StepID:    step.ID,
				Boundary:  "outside",
				Kind:      "human-checklist",
				Detail:    describeCompensation(step, entries),
			})
		case "inside":
			exec, ok := insideExecutors[execKey(m.Component, step.ID)]
			if !ok {
				// Disable's pre-flight pass already refuses the whole run
				// before anything is touched when any step lacks an
				// executor; reaching here would mean that pass regressed.
				return results, deskkit.Refused(fmt.Sprintf(
					"%s: inside step %q has no registered automatic reverse (pre-flight should have caught this)", m.Component, step.ID))
			}
			if opts.DryRun {
				results = append(results, StepResult{
					Component: m.Component,
					StepID:    step.ID,
					Boundary:  "inside",
					Kind:      "would-reverse",
					Detail:    step.Inverse,
				})
				continue
			}
			if err := exec(opts.Root); err != nil {
				return results, deskkit.Unverifiable(fmt.Sprintf("reversing %s/%s: %v", m.Component, step.ID, err), err)
			}
			results = append(results, StepResult{
				Component: m.Component,
				StepID:    step.ID,
				Boundary:  "inside",
				Kind:      "reversed",
				Detail:    step.Inverse,
			})
		default:
			return results, deskkit.Refused(fmt.Sprintf("%s: apply step %q has unknown boundary %q", m.Component, step.ID, step.Boundary))
		}
	}
	return results, nil
}

// describeCompensation renders one outside step's checklist line. Every
// non-"none" compensation is prefixed "list-for-human:" REGARDLESS of the
// manifest's own wording — the printed contract is driven by whether the step
// is marked unattended (never true anywhere in this brief; component-model.md
// §5: "compensations default to listed for a human"), not by string-matching
// the manifest's prose. Ledger entries recorded for this step, if any, are
// named so a human has something concrete to act on; their absence is also
// named, since an outside step with no ledger line is itself a defect
// (`deskmanifest lint` catches it separately).
func describeCompensation(step applyStep, entries []deskkit.LedgerEntry) string {
	comp := strings.TrimSpace(step.Compensation)
	if comp == "" {
		comp = "(no compensation recorded in the manifest)"
	}
	var ids []string
	for _, e := range entries {
		if e.Step == step.ID {
			ids = append(ids, e.ID)
		}
	}
	sort.Strings(ids)

	prefix := "list-for-human: "
	if strings.HasPrefix(strings.ToLower(comp), "none") {
		prefix = "no action needed: "
	}
	if len(ids) > 0 {
		return fmt.Sprintf("%s%s (ledger: %s)", prefix, comp, strings.Join(ids, ", "))
	}
	return fmt.Sprintf("%s%s (no ledger entries found for this step)", prefix, comp)
}
