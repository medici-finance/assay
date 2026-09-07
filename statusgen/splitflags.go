package main

// splitflags.go — split-flag conservation.
//
// Splitting a brief is AUTHORING, and the author-brief rules apply — but nothing
// checked that a split conserved the parent's RISK. It is the one authoring move
// where a human gate is most likely to evaporate: a large, human-gated,
// irreversible brief is uncomfortable to move, so it is split; the "small
// mechanical part" lands in one child and the "risky part" in another; and the
// small child then FEELS low-risk to the author, so it is authored `gate: model`,
// `irreversible: no`. But risk is a property of what the change DOES, not of how
// big the diff is — a twenty-line change that makes a shared field non-optional is
// small AND breaks every caller. A parent that entered the day `gate: human`,
// `irreversible: yes`, regulatory + customer, has been observed to split into a
// child that is `gate: model`, everything `no` — a breaking change to a shared
// interface would then merge on a model sign-off. The split is the moment the
// human gate is most likely to dissolve, and it was unpoliced.
//
// The rule this file enforces is conservation: a child brief may not carry a
// WEAKER flag than the brief it was split from. For `gate` the child must be at
// least as strict (`human` >= `model`); for each of the four canonical risk
// answers the child must be at least as high (`yes` >= `no`). Equivalently:
//
//	gate  = the stricter of (parent, declared)
//	risk  = MAX(parent, declared), per canonical key
//
// A shard can never be less-gated or lower-risk than the brief it came from. When
// the split is genuinely clean — the risky work really did all go to a sibling —
// the answer is to keep the child's flags up, not down: over-gating costs a human
// glance, under-gating merges the fault. So conservation fails SAFE, toward more
// gating, and it is a hard PROBLEM (not a downgrade-to-pass).
//
// # Two ways a parent is named
//
// A split needs SOMETHING to compare against, so this reads the parentage from
// whichever of two signals is present, and both fail safe:
//
//   - numeric-stem lineage — the assay split convention. `02` splits into `02a`,
//     `02b`, `02c`: a lettered shard shares the numeric stem of the brief it came
//     from. When the un-lettered parent (`02`) is still in the tree it IS the
//     floor. When the parent was retired in the same change, the strictest SIBLING
//     shard stands in as the floor — so the incident pattern (02b faithful,
//     02a/02c downgraded) is caught even with `02` deleted, because 02b's flags
//     pull the floor up under 02a and 02c.
//   - a declared `split-from: <stream>/<NN>` — the explicit parent, needed when
//     the lineage is NOT in the numbering (a split across streams, or a
//     renumbered child). Today lineage otherwise lives only in prose, which
//     nothing can compare. A `split-from` that does not resolve in the tree is a
//     could-not-check NOTICE, never a silent pass: if the parent was retired, the
//     conservation could not be verified against it and that is said out loud.
//
// # Envelope
//
// Pure over the tree, offline, the same envelope every other --lint check keeps.
// It re-parses the brief set (as verifySectionProblems does) rather than threading
// state through checkBriefFiles, so the rule is one self-contained file.

import (
	"fmt"
	"sort"
)

// briefFlagRec is the flag-bearing identity of one brief, indexed so a child can
// resolve the parent it was split from and compare against it.
type briefFlagRec struct {
	id     string // "<stream>/<num>", e.g. "example-app/02a"
	path   string
	stream string
	num    string // "02", "02a"
	gate   string // model | human
	risk   map[string]string
	// splitFrom is the declared `<stream>/<NN>` parent, "" when absent.
	splitFrom string
}

// briefNumStem splits a brief number into its numeric stem and whether it carries
// a shard letter. briefNameRe guarantees the shape `[0-9]+[a-z]?`, so the stem is
// the leading digits and `lettered` is true exactly when a trailing letter
// remains ("02a" -> "02", true; "02" -> "02", false).
func briefNumStem(num string) (stem string, lettered bool) {
	i := 0
	for i < len(num) && num[i] >= '0' && num[i] <= '9' {
		i++
	}
	return num[:i], i < len(num)
}

// gateRank orders the gate values so "at least as strict" is a numeric >=.
func gateRank(g string) int {
	if g == "human" {
		return 1
	}
	return 0
}

// flagDowngrades returns one human-readable phrase per dimension on which `child`
// carries a WEAKER flag than `parent` — an empty slice means the child conserves
// (or exceeds) every one of the parent's flags. The comparison is deliberately
// one-directional: a child that is STRICTER than its parent (an escalation) is
// always fine and is never reported.
func flagDowngrades(child, parent briefFlagRec) []string {
	var d []string
	if gateRank(child.gate) < gateRank(parent.gate) {
		d = append(d, fmt.Sprintf("gate is %q but the parent's is %q", child.gate, parent.gate))
	}
	for _, k := range canonicalRiskKeys {
		if parent.risk[k] == "yes" && child.risk[k] != "yes" {
			d = append(d, fmt.Sprintf("risk.%s is %q but the parent's is \"yes\"", k, child.risk[k]))
		}
	}
	return d
}

// siblingFloor is the strictest-flag fold over every lettered shard that shares
// (stream, stem): the proxy parent used when the un-lettered parent has been
// retired. Including the child itself in the fold is harmless — a flag never
// lowers its own floor — and it means the fold needs no special-case for the
// caller.
func siblingFloor(index map[string]briefFlagRec, stream, stem string) briefFlagRec {
	floor := briefFlagRec{gate: "model", risk: map[string]string{}}
	for _, k := range canonicalRiskKeys {
		floor.risk[k] = "no"
	}
	for _, r := range index {
		if r.stream != stream {
			continue
		}
		st, lettered := briefNumStem(r.num)
		if !lettered || st != stem {
			continue
		}
		if gateRank(r.gate) > gateRank(floor.gate) {
			floor.gate = r.gate
		}
		for _, k := range canonicalRiskKeys {
			if r.risk[k] == "yes" {
				floor.risk[k] = "yes"
			}
		}
	}
	return floor
}

// splitFlagProblems enforces split-flag conservation across a brief tree. It is
// wired into run() alongside checkBriefFiles: `streams` is the (possibly
// product-scoped) set whose children are checked, and `allStreams` is the full
// house set the parent is resolved against — a split may legitimately name a
// parent in a stream that scoping dropped, exactly like checkBriefFiles's
// depends:/unblocks: resolution.
func splitFlagProblems(streams, allStreams []*Stream) (problems, notices []string) {
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	notice := func(format string, a ...any) { notices = append(notices, fmt.Sprintf(format, a...)) }

	// Index every brief in the full house set by its id, so both the numeric-stem
	// parent and a cross-stream split-from resolve.
	index := map[string]briefFlagRec{}
	for _, s := range allStreams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue // malformed / legacy: reported (or exempt) by checkBriefFiles
			}
			id, num, okName := expectedBriefID(path)
			if !okName {
				continue
			}
			index[id] = briefFlagRec{
				id: id, path: path, stream: s.Name, num: num,
				gate: bf.Gate, risk: bf.Risk, splitFrom: bf.SplitFrom,
			}
		}
	}

	// Which ids are in scope for CHECKING (children). Only these are compared; a
	// parent out of scope is still resolvable through `index`.
	scoped := map[string]bool{}
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			if id, _, ok := expectedBriefID(path); ok {
				scoped[id] = true
			}
		}
	}

	ids := make([]string, 0, len(index))
	for id := range index {
		if scoped[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids) // deterministic message order for a CI diff

	const conserveWhy = "a split may not carry a weaker risk/gate flag than the brief it was split from (gate = the stricter of parent and child; risk = MAX(parent, child) per key). Raise the child's flags to conserve the parent's — never lower a flag to pass this check"

	for _, id := range ids {
		child := index[id]

		// (1) An explicit `split-from` declaration.
		if child.splitFrom != "" {
			switch {
			case !refInRepoBriefRe.MatchString(child.splitFrom):
				add("%s: brief %s has split-from %q, which is not a <stream>/<NN> brief id", child.path, child.id, child.splitFrom)
			case child.splitFrom == child.id:
				add("%s: brief %s declares split-from of itself", child.path, child.id)
			default:
				if parent, ok := index[child.splitFrom]; ok {
					for _, d := range flagDowngrades(child, parent) {
						add("%s: brief %s was split from %s but its %s — %s", child.path, child.id, parent.id, d, conserveWhy)
					}
				} else {
					notice("%s: brief %s declares split-from %s, which is not a brief in the tree — if the parent was retired in this change, split-flag conservation could not be verified against it", child.path, child.id, child.splitFrom)
				}
			}
		}

		// (2) Numeric-stem lineage: a lettered shard (02a) against its parent (02),
		// or — when the parent is retired — against the strictest sibling shard.
		stem, lettered := briefNumStem(child.num)
		if !lettered {
			continue
		}
		parentID := child.stream + "/" + stem
		if parent, ok := index[parentID]; ok {
			for _, d := range flagDowngrades(child, parent) {
				add("%s: brief %s is a shard of %s but its %s — %s", child.path, child.id, parent.id, d, conserveWhy)
			}
			continue
		}
		// Parent retired in the same change: the strictest sibling sets the floor.
		floor := siblingFloor(index, child.stream, stem)
		for _, d := range flagDowngrades(child, floor) {
			add("%s: brief %s is a shard split from %s (retired in this change) but its %s — a sibling shard carries that flag, so the split downgraded it; %s", child.path, child.id, parentID, d, conserveWhy)
		}
	}

	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}
