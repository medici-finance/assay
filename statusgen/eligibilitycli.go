package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// eligibilitycli.go — the `--eligibility` CLI (graph-execution/01 Task item
// 3): self-contained, STATUS.md-free, offline. Same discipline as --next-up /
// --gate-scores: it never reads or writes the generated board.

// runEligibility loads the tree, runs the SAME evaluateEligibility() every
// in-tree consumer (Next-up, the drive frontier) reads, and prints the
// verdict for every brief — one line per brief by default, or the full JSON
// structure with --json. Exit 0 on any verdict (a held/could-not-check
// verdict is not itself a failure of this command); exit 2 only when the
// tree cannot be read at all.
func runEligibility(root string, jsonMode bool) int {
	streams, _, err := loadHydratedStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 2
	}
	elig := eligibilityForStreams(streams)
	ids := make([]string, 0, len(elig))
	for id := range elig {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	if jsonMode {
		rows := make([]Eligibility, 0, len(ids))
		for _, id := range ids {
			rows = append(rows, elig[id])
		}
		out, err := json.Marshal(rows)
		if err != nil {
			fmt.Fprintln(os.Stderr, "statusgen:", err)
			return 1
		}
		fmt.Println(string(out))
		return 0
	}

	for _, id := range ids {
		e := elig[id]
		line := fmt.Sprintf("%s  %s", e.ID, e.Verdict)
		if len(e.Holds) > 0 {
			frags := make([]string, 0, len(e.Holds))
			for _, h := range e.Holds {
				frags = append(frags, fmt.Sprintf("%s(%s)", h.Ref, h.State))
			}
			line += "  held by " + strings.Join(frags, ", ")
		}
		if len(e.Notices) > 0 {
			frags := make([]string, 0, len(e.Notices))
			for _, n := range e.Notices {
				frags = append(frags, fmt.Sprintf("%s(%s)", n.Ref, n.State))
			}
			line += "  feather " + strings.Join(frags, ", ")
		}
		fmt.Println(line)
	}
	return 0
}

// ruleEligibilityCouldNotCheck is the stable [rule-tag] bracket token for the
// --lint NOTICE below, and the tag registered in enforcementstatus.go's
// lintRuleRegistry.
const ruleEligibilityCouldNotCheck = "eligibility-could-not-check"

// eligibilityCouldNotCheckNotices is the graph-execution/01 Task item 4
// --lint surface: one NOTICE per brief the evaluator HOLDS because of a
// could-not-check edge (a registry or sibling-checkout gap, never a genuine
// "not done yet"), so that class of hold is visible on a full lint — not only
// in a dispatcher's --eligibility/--next-up output. An ordinary unsatisfied
// hold (the target brief is simply still todo) is not reported here; it is
// the everyday case every dependency graph has and is not itself a defect to
// surface. Advisory severity — it never changes the exit code, mirroring
// every other reserved/graph NOTICE in this file family (checkBriefV2Semantics).
func eligibilityCouldNotCheckNotices(streams []*Stream) []string {
	elig := eligibilityForStreams(streams)
	ids := make([]string, 0, len(elig))
	for id := range elig {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var notices []string
	for _, id := range ids {
		e := elig[id]
		for _, h := range e.Holds {
			if h.State != StateCouldNotCheck {
				continue
			}
			notices = append(notices, fmt.Sprintf("[%s] %s: held by %s — could-not-check (%s)", ruleEligibilityCouldNotCheck, id, h.Ref, h.Why))
		}
	}
	return notices
}
