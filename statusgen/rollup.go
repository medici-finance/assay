package main

// rollup.go — `statusgen --requirements-rollup [--since <date>] [--json]`, the
// per-release traceability rollup sdlc/02 produces and the audit-pack brief
// (sdlc/08) consumes as an INPUT to --export-evidence, never a second bundler.
//
// For each requirement it prints: the requirement, its acceptance criteria, the
// briefs that name it (via `satisfies:`) plus any the requirement's own
// `satisfied-by` claims, each of those briefs' status, and each brief's Evidence.
// The per-requirement `state` is THREE-STATE and honest by construction:
//
//   - satisfied       — at least one brief backs it and EVERY backing brief is
//                       `done`. Never reported for a requirement with no backing
//                       brief: "all of the empty set are done" is the silent pass
//                       the three-state rule forbids.
//   - partial         — briefs back it but not all are done, OR nothing traces to
//                       it yet. Not a pass, and reported as itself.
//   - could-not-check — a `satisfied-by` brief the tool could not resolve to a
//                       row. Reported as itself, never rounded to satisfied.
//
// The rollup reports what was AUTHORED, not what was MEASURED (registers-v1 §6.4,
// spec/README.md's normative preamble): a `done` backing brief means the board says
// the work landed, NOT that this tool re-ran the acceptance criteria. It is the map
// that makes the walk ask → work → evidence possible; it does not walk it.
//
// Read-only, offline: it reads the same register and brief corpus --lint parses,
// touches no network, writes no STATUS.md.

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	rollupStateSatisfied      = "satisfied"
	rollupStatePartial        = "partial"
	rollupStateCouldNotCheck  = "could-not-check"
	rollupExitOK              = 0
	rollupExitCouldNotResolve = 2
)

// rollupBrief is one backing brief of a requirement in the rollup.
type rollupBrief struct {
	Brief    string `json:"brief"`
	Status   string `json:"status"`   // README row status, or "" when unresolved
	Resolved bool   `json:"resolved"` // false = named by satisfied-by but no brief row found
	Evidence string `json:"evidence,omitempty"`
}

// rollupRequirement is one requirement's rollup object.
type rollupRequirement struct {
	ID         string        `json:"id"`
	Title      string        `json:"title"`
	Impact     string        `json:"impact"`
	Status     string        `json:"status"` // the register lifecycle status
	Date       string        `json:"date"`
	Acceptance []string      `json:"acceptance"`
	Briefs     []rollupBrief `json:"briefs"`
	State      string        `json:"state"` // satisfied | partial | could-not-check
}

// rollupReport is the whole emitted document.
type rollupReport struct {
	Since        string              `json:"since,omitempty"`
	Note         string              `json:"note"`
	Requirements []rollupRequirement `json:"requirements"`
}

// rollupAuthoredNote is the honest weaker-and-true claim carried on every rollup:
// it reports authored state, not a re-measurement of the acceptance criteria.
const rollupAuthoredNote = "reports what was AUTHORED (the register + board rows), not what was MEASURED — a done backing brief means the board says the work landed, not that this tool re-ran the acceptance criteria (registers-v1 §6.4)"

// runRequirementsRollup builds the rollup for one root. since, when non-empty,
// keeps only requirements dated on or after it (YYYY-MM-DD). Exit 2 (could-not-
// check) when the register itself is unreadable; exit 0 otherwise, including an
// empty register (a legitimate empty is not a failure).
func runRequirementsRollup(root, since string, asJSON bool) int {
	report, code := buildRollupReport(root, since)
	if code != rollupExitOK {
		return code
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintln(os.Stderr, "statusgen:", err)
			return rollupExitCouldNotResolve
		}
		return rollupExitOK
	}
	printRollupText(report)
	return rollupExitOK
}

// buildRollupReport builds the rollup document for one root, split from the IO so
// it is testable without capturing stdout. It returns the report and an exit code:
// rollupExitCouldNotResolve (2, with a stderr diagnostic) when --since is malformed
// or the register/streams cannot be read, rollupExitOK (0) otherwise — including an
// empty register, a legitimate empty that is not a failure.
func buildRollupReport(root, since string) (rollupReport, int) {
	if strings.TrimSpace(since) != "" {
		if _, err := time.Parse("2006-01-02", strings.TrimSpace(since)); err != nil {
			fmt.Fprintf(os.Stderr, "statusgen: --since %q is not a YYYY-MM-DD date\n", since)
			return rollupReport{}, rollupExitCouldNotResolve
		}
	}
	entries, err := parseRequirementsDir(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "statusgen: requirements register unreadable: %v\n", err)
		return rollupReport{}, rollupExitCouldNotResolve
	}
	streams, _, lerr := loadStreams(root)
	if lerr != nil {
		fmt.Fprintf(os.Stderr, "statusgen: could not load streams: %v\n", lerr)
		return rollupReport{}, rollupExitCouldNotResolve
	}
	attachPlaceholders(streams)
	briefs, _ := scanTracedBriefs(streams)

	// Index briefs two ways: by key (for satisfied-by resolution) and by the
	// requirement ids they cite (for the naming direction).
	byKey := map[string]tracedBrief{}
	citedBy := map[string][]tracedBrief{} // REQ id -> briefs whose satisfies names it
	for _, b := range briefs {
		byKey[b.Key] = b
		for _, ref := range b.Satisfies {
			if id, cross := requirementRefKind(ref); id != "" && !cross {
				citedBy[id] = append(citedBy[id], b)
			}
		}
	}

	report := rollupReport{Since: strings.TrimSpace(since), Note: rollupAuthoredNote}
	for _, e := range entries {
		if since != "" && !dateOnOrAfter(e.Date, since) {
			continue
		}
		report.Requirements = append(report.Requirements, buildRollupRequirement(e, citedBy, byKey))
	}
	// Deterministic order: impact descending, then id.
	sort.SliceStable(report.Requirements, func(i, j int) bool {
		ri, _ := requirementImpactRank(strings.TrimSpace(report.Requirements[i].Impact))
		rj, _ := requirementImpactRank(strings.TrimSpace(report.Requirements[j].Impact))
		if ri != rj {
			return ri > rj
		}
		return report.Requirements[i].ID < report.Requirements[j].ID
	})
	return report, rollupExitOK
}

// buildRollupRequirement assembles one requirement's rollup, reconciling the two
// traceability directions: the briefs that CITE it (`satisfies:`) and the briefs it
// CLAIMS satisfied it (`satisfied-by`). A satisfied-by brief that does not resolve
// to a row is could-not-check for this requirement.
func buildRollupRequirement(e requirementEntry, citedBy map[string][]tracedBrief, byKey map[string]tracedBrief) rollupRequirement {
	seen := map[string]bool{}
	var backing []rollupBrief
	anyUnresolved := false

	add := func(rb rollupBrief) {
		if seen[rb.Brief] {
			return
		}
		seen[rb.Brief] = true
		backing = append(backing, rb)
	}

	// Direction 1: briefs whose satisfies names this requirement. These always
	// resolve — they came out of the corpus walk.
	for _, b := range citedBy[e.ID] {
		add(rollupBrief{Brief: b.Key, Status: b.Status, Resolved: true, Evidence: strings.TrimSpace(b.Evidence)})
	}
	// Direction 2: the requirement's own satisfied-by claims. An in-repo ref may or
	// may not resolve to a brief row; a cross-repo ref cannot be resolved here.
	for _, ref := range nonEmptyStrings(e.SatisfiedBy) {
		if b, ok := byKey[ref]; ok {
			add(rollupBrief{Brief: b.Key, Status: b.Status, Resolved: true, Evidence: strings.TrimSpace(b.Evidence)})
			continue
		}
		anyUnresolved = true
		add(rollupBrief{Brief: ref, Status: "", Resolved: false})
	}

	sort.SliceStable(backing, func(i, j int) bool { return backing[i].Brief < backing[j].Brief })

	return rollupRequirement{
		ID:         e.ID,
		Title:      e.Title,
		Impact:     strings.TrimSpace(e.Impact),
		Status:     strings.TrimSpace(e.Status),
		Date:       strings.TrimSpace(e.Date),
		Acceptance: nonEmptyStrings(e.Acceptance),
		Briefs:     backing,
		State:      rollupState(backing, anyUnresolved),
	}
}

// rollupState is the three-state verdict. A backing brief that did not resolve
// makes the whole requirement could-not-check. Otherwise it is satisfied ONLY when
// there is at least one backing brief and every one is done — an empty backing set
// is partial, never a vacuous pass. A backing brief whose status is "" (a real
// brief with no board row) is treated as not-done, so it too holds the requirement
// at partial rather than letting an unknown round up.
func rollupState(backing []rollupBrief, anyUnresolved bool) string {
	if anyUnresolved {
		return rollupStateCouldNotCheck
	}
	if len(backing) == 0 {
		return rollupStatePartial
	}
	for _, b := range backing {
		if strings.TrimSpace(b.Status) != "done" {
			return rollupStatePartial
		}
	}
	return rollupStateSatisfied
}

// dateOnOrAfter reports whether date (YYYY-MM-DD) is on or after since. An
// unparseable date is KEPT (returns true) rather than silently dropped — a
// malformed date is the register's own PROBLEM to surface, not the rollup's to
// hide by omission.
func dateOnOrAfter(date, since string) bool {
	d, derr := time.Parse("2006-01-02", strings.TrimSpace(date))
	s, serr := time.Parse("2006-01-02", strings.TrimSpace(since))
	if derr != nil || serr != nil {
		return true
	}
	return !d.Before(s)
}

func printRollupText(report rollupReport) {
	fmt.Println("REQUIREMENTS ROLLUP")
	fmt.Println("note:", report.Note)
	if report.Since != "" {
		fmt.Println("since:", report.Since)
	}
	if len(report.Requirements) == 0 {
		fmt.Println("(no requirements)")
		return
	}
	for _, r := range report.Requirements {
		fmt.Printf("\n%s  [%s]  impact=%s  status=%s\n", r.ID, r.State, r.Impact, r.Status)
		fmt.Printf("  title: %s\n", r.Title)
		if len(r.Acceptance) > 0 {
			fmt.Println("  acceptance:")
			for _, a := range r.Acceptance {
				fmt.Printf("    - %s\n", a)
			}
		}
		if len(r.Briefs) == 0 {
			fmt.Println("  briefs: (none — nothing traces to this requirement)")
			continue
		}
		fmt.Println("  briefs:")
		for _, b := range r.Briefs {
			status := b.Status
			if !b.Resolved {
				status = "could-not-check (no brief row found)"
			} else if status == "" {
				status = "could-not-check (no board row)"
			}
			fmt.Printf("    - %s: %s\n", b.Brief, status)
			if ev := strings.TrimSpace(b.Evidence); ev != "" {
				fmt.Println("      evidence:")
				for _, line := range strings.Split(ev, "\n") {
					fmt.Printf("        %s\n", line)
				}
			}
		}
	}
}
