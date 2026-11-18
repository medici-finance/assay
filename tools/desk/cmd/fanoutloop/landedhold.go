package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/statusgen/briefdoc"
)

// landedhold.go — the next step the plan prints for a LANDED-UNRECONCILED row (#5853).
//
// The usual step is `statusgen reconcile --backfill --apply`. But statusgen's lint HOLDS that flip
// for two kinds of brief, and pointing a reader at the reconcile then only produces a red lint:
//
//   - the design-approval gate (spec/lifecycle-v1.md §4.4): a risk-gated brief (gate: human, or any
//     risk answer yes) authored strictly after briefdoc.DesignGateCutover may not sit at
//     in-progress-or-later without a `design:` reference;
//   - the decision-gate hold (spec/lifecycle-v1.md §4.5): a gate: human brief may not move to
//     implemented while it records no `decision-issue:` and `ruling:`.
//
// For such a row the plan names the missing key instead. Recording it is human work (an approved
// design record, a human ruling) — the planner never writes it. The prediction is the offline half
// only: a present key is not proven to dereference, so a row whose keys are all present still gets
// the reconcile step, and the lint stays the authority.

// leadingAuthoredDate is statusgen's own reading of `authored:` (its leading YYYY-MM-DD).
var leadingAuthoredDate = regexp.MustCompile(`^\s*(\d{4}-\d{2}-\d{2})`)

// splitUnchecked separates the plan's landed rows from its could-not-check rows.
func splitUnchecked(rows []landedRow) (landed, unchecked []landedRow) {
	for _, l := range rows {
		if l.unchecked != "" {
			unchecked = append(unchecked, l)
		} else {
			landed = append(landed, l)
		}
	}
	return landed, unchecked
}

// landedNextStep is the parenthetical the plan prints after a landed row's merged PR.
func landedNextStep(root, briefID string) string {
	if keys := reconcileHoldKeys(root, briefID); len(keys) > 0 {
		return "reconcile is held by statusgen's lint until the brief records " + strings.Join(keys, " and ") +
			" — human work; `statusgen reconcile` will not flip it before then"
	}
	return "run `statusgen reconcile --backfill --apply` to flip the board cell"
}

// reconcileHoldKeys returns the frontmatter keys the brief is missing for its reconcile flip to pass
// statusgen's lint, or nil when none is predicted missing — including when the brief file cannot be
// found or parsed (the plan then falls back to the reconcile step and the lint decides).
func reconcileHoldKeys(root, briefID string) []string {
	stream, num, ok := strings.Cut(briefID, "/")
	if !ok || root == "" {
		return nil
	}
	matches, _ := filepath.Glob(filepath.Join(root, "docs", "streams", stream, "brief-"+num+"-*.md"))
	if len(matches) != 1 {
		return nil
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		return nil
	}
	block, _, err := briefdoc.Split(string(raw))
	if err != nil {
		return nil
	}
	fields, err := briefdoc.ParseFields(block)
	if err != nil {
		return nil
	}
	v := fields.Values
	gate := strings.ToLower(strings.TrimSpace(v["gate"]))
	r := fields.Risk
	riskGated := gate == "human" || r.Regulatory || r.Customer || r.Irreversible || r.SensitiveData
	var keys []string
	if m := leadingAuthoredDate.FindStringSubmatch(v["authored"]); riskGated && m != nil && m[1] > briefdoc.DesignGateCutover && strings.TrimSpace(v["design"]) == "" {
		keys = append(keys, "`design:`")
	}
	if gate == "human" {
		if strings.TrimSpace(v["decision-issue"]) == "" {
			keys = append(keys, "`decision-issue:`")
		}
		if strings.TrimSpace(v["ruling"]) == "" {
			keys = append(keys, "`ruling:`")
		}
	}
	return keys
}
