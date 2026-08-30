package main

// lifecycle.go — the PURE lifecycle derivation (derived-board/03).
//
// Every stream-board lifecycle cell is a claim about the world, and this fold is
// what computes that claim from witnesses an instrument actually read: open and
// merged PRs carrying a `Brief:` trailer, a `verifyrun --check` witness, an App
// approval at head, a human ruling, and a linked issue's labels. It has NO I/O —
// the reconcile verb (ghfetch.go + the verb wiring in main.go) gathers the inputs;
// this file only folds them. That separation is the point: the derivation is
// unit-testable against a fixture matrix with no network and no tree.
//
// The invariant it enforces (docs/streams/derived-board/spec.md §2 and the
// three-state-instrument rule): a derived NEGATIVE — a `todo` — is legal only
// when the search actually RAN and found nothing. When the instrument could not
// look (LookedAt == false: offline, an API error, a rate-limit), every PR-derived
// cell is `unknown` and carries the reason. `unknown` is a first-class cell here,
// never rounded down to `todo`.
//
// Precedence (highest witnessed wins), spec §2:
//   done > verified > implemented > in-progress > blocked > todo
// with `blocked` overlaying `in-progress`/`todo` ONLY, and `unknown` replacing any
// PR-derived cell the instrument could not read. Demotion is automatic: a
// reverted/reopened merge, a red verify witness, a dismissed approval, or a
// witness whose version no longer matches the brief all fall back to the highest
// state still witnessed.

import (
	"fmt"
	"sort"
)

// PRState is the state of a pull request carrying a brief trailer.
type PRState string

const (
	prOpen   PRState = "open"   // an open PR (draft or ready)
	prMerged PRState = "merged" // merged to the default branch
	prClosed PRState = "closed" // closed unmerged
)

// PRRecord is one pull request that carries a `Brief: <ref>` trailer, as read by
// ghfetch. BriefRef is the trailer's brief id. Reopened marks a merge that was
// later reverted and the PR reopened — the demotion signal that a merged PR no
// longer witnesses `implemented`.
type PRRecord struct {
	BriefRef string
	Number   int
	State    PRState
	Draft    bool
	HeadSHA  string
	MergeSHA string
	Reopened bool
}

// WitnessInfo is the `verifyrun --check` result for a brief — the `verified`
// witness. Version is the brief version the witness was run against; a mismatch
// against the brief's current version is the stale-Verify demotion (spec §5).
type WitnessInfo struct {
	Passed  bool
	Version int
}

// ApprovalInfo is the App approval at head — the `done` witness for a gate:model
// brief. A dismissed or stale approval is Approved==false or AtHead==false, which
// demotes `done` back to `verified`.
type ApprovalInfo struct {
	Approved bool
	AtHead   bool
}

// BriefIdent is the per-brief identity the fold needs: its id, its gate (which
// witness closes `done`), and its current version (for the stale-witness demotion).
type BriefIdent struct {
	ID      string
	Gate    string // "model" | "human"
	Version int
}

// LifecycleInput is the complete witness set for one reconcile run. Maps are keyed
// by brief ID. LookedAt is whether the PR fetch SUCCEEDED; when false, every
// PR-derived cell is `unknown` with Reason.
type LifecycleInput struct {
	Briefs      []BriefIdent
	PRs         []PRRecord
	Witnesses   map[string]WitnessInfo
	Approvals   map[string]ApprovalInfo
	Rulings     map[string]bool
	IssueLabels map[string][]string
	LookedAt    bool
	Reason      string
}

// BriefCell is the derived lifecycle cell for one brief. Source names the witness
// class the cell came from ("pr" | "witness" | "none"); Witness is the human-
// readable witness the instrument read; Reason is why, for a negative or unknown.
type BriefCell struct {
	ID      string `json:"id"`
	Cell    string `json:"cell"`
	Source  string `json:"source"`
	Witness string `json:"witness"`
	Reason  string `json:"reason"`
	Version int    `json:"version"`
}

// blockingIssueLabels are the labels that overlay `blocked` on an in-progress or
// todo brief (spec §2). `blocked-by: env` is handled separately by the caller.
var blockingIssueLabels = map[string]bool{
	"question":       true,
	"needs-decision": true,
	"help wanted":    true,
}

// DeriveLifecycle folds the witness set into one cell per brief. It is pure and
// deterministic: the same input yields byte-identical output (the fixtures assert
// this), and briefs come back in the input order.
func DeriveLifecycle(in LifecycleInput) []BriefCell {
	prsByBrief := map[string][]PRRecord{}
	for _, pr := range in.PRs {
		prsByBrief[pr.BriefRef] = append(prsByBrief[pr.BriefRef], pr)
	}
	out := make([]BriefCell, 0, len(in.Briefs))
	for _, b := range in.Briefs {
		out = append(out, deriveOne(b, prsByBrief[b.ID], in))
	}
	return out
}

func deriveOne(b BriefIdent, prs []PRRecord, in LifecycleInput) BriefCell {
	c := BriefCell{ID: b.ID, Version: b.Version, Source: "pr"}

	// 1. PR-derived base. When the instrument could NOT look (offline, API error,
	// rate-limit), the PR-derived cell is `unknown` with the reason — never a
	// silent todo (the three-state invariant applied to the board). When it DID
	// look: `implemented` at the LATEST merge; else `in-progress` for any open PR;
	// else `todo` — and the todo carries the evidence the search ran. Witnesses
	// (verified/done) still resolve below in BOTH cases, since a verify witness and
	// an Evidence ruling are tree-readable and do not depend on the PR fetch.
	if !in.LookedAt {
		c.Cell = "unknown"
		c.Reason = in.Reason
	} else {
		latestMerged, open := classifyPRs(prs)
		switch {
		case latestMerged != nil:
			c.Cell = "implemented"
			c.Witness = fmt.Sprintf("PR #%d (merged %s)", latestMerged.Number, shortSHA(latestMerged.MergeSHA))
		case open != nil:
			state := "ready"
			if open.Draft {
				state = "draft"
			}
			c.Cell = "in-progress"
			c.Witness = fmt.Sprintf("PR #%d (%s %s)", open.Number, state, shortSHA(open.HeadSHA))
		default:
			c.Cell = "todo"
			c.Reason = "PR search ran; no open or merged PR carries this brief's trailer"
		}
	}

	// 3. Verify witness overlay (verified / done), and its demotions.
	if w, ok := in.Witnesses[b.ID]; ok {
		if b.Version != 0 && w.Version != 0 && w.Version != b.Version {
			// Stale-Verify demotion (spec §5): the witness was run against a
			// different version than the brief now carries. It cannot claim
			// `verified` — the board says so instead of a verifier's could-not-check.
			c.Cell = "unknown"
			c.Source = "witness"
			c.Reason = fmt.Sprintf("witness for v%d, brief is v%d", w.Version, b.Version)
			c.Witness = fmt.Sprintf("verifyrun --check witness for v%d, brief is v%d", w.Version, b.Version)
			return c
		}
		if w.Passed {
			c.Cell = "verified"
			c.Source = "witness"
			c.Witness = fmt.Sprintf("verifyrun --check pass (v%d)", w.Version)
			if done, why := isDone(b, in); done {
				c.Cell = "done"
				c.Witness = why
			}
			return c
		}
		// Red witness: verify was RUN and FAILED → not verified. The cell falls
		// back to the highest state still witnessed (the PR base above). No promotion.
	}

	// 4. Blocked overlay — in-progress / todo ONLY (spec §2). A merged/verified/done
	// brief is not overlaid; a blocking label there is a data problem for another
	// check, not a lifecycle demotion.
	if c.Cell == "in-progress" || c.Cell == "todo" {
		if label, blocked := blockingLabel(in.IssueLabels[b.ID]); blocked {
			c.Cell = "blocked"
			c.Source = "none"
			c.Witness = fmt.Sprintf("linked issue label %q", label)
			c.Reason = "linked issue carries a blocking label"
		}
	}
	return c
}

// classifyPRs returns the latest non-reverted merged PR and the first open PR
// among a brief's PRs. A merged-then-reopened (reverted) PR is NOT counted as a
// merge — it is treated as open work, the reverted-merge demotion. "Latest" merge
// is the highest PR number, a stable proxy for merge order across a deterministic
// fixture set.
func classifyPRs(prs []PRRecord) (latestMerged, open *PRRecord) {
	sorted := make([]PRRecord, len(prs))
	copy(sorted, prs)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Number > sorted[j].Number })
	for i := range sorted {
		pr := &sorted[i]
		switch pr.State {
		case prMerged:
			if pr.Reopened {
				if open == nil {
					open = pr
				}
				continue
			}
			if latestMerged == nil {
				latestMerged = pr
			}
		case prOpen:
			if open == nil {
				open = pr
			}
		case prClosed:
			// closed unmerged: no witness — neither implemented nor in-progress.
		}
	}
	return latestMerged, open
}

// isDone reports whether a verified brief is `done`, per its gate: a gate:model
// brief closes on an App approval at the merged head (the existing auto-flip); a
// gate:human brief closes on a human:<login> Evidence ruling.
func isDone(b BriefIdent, in LifecycleInput) (bool, string) {
	switch b.Gate {
	case "model":
		if a, ok := in.Approvals[b.ID]; ok && a.Approved && a.AtHead {
			return true, "App approval at merged head"
		}
	case "human":
		if in.Rulings[b.ID] {
			return true, "human:<login> Evidence ruling"
		}
	}
	return false, ""
}

func blockingLabel(labels []string) (string, bool) {
	for _, l := range labels {
		if blockingIssueLabels[l] {
			return l, true
		}
	}
	return "", false
}

// shortSHA abbreviates a commit SHA to 7 chars for a witness string, leaving a
// short or empty SHA untouched.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
