package main

// reviewlane.go — the review kit, emitted per lane.
//
// WHY. The review kit rides on every request a reviewer makes, so a clause the dispatched
// lane never runs is paid for on each one. Some of the kit is one lane's alone: the
// design-fit pass, the board-row flip check, the same-head re-approve exemptions and the
// prompt-audit procedure are the correctness lane's. A security reviewer needs to know
// those passes exist and who owns them, not the procedure.
//
// HOW. references/review-prompt.md stays ONE file — one wording of every shared clause, the
// same reason the common clauses are one file. A lane-specific stretch sits between two
// whole-line markers:
//
//	<!-- lane:correctness:begin -->
//	…
//	<!-- lane:correctness:end -->
//
// Everything outside a marked stretch reaches every lane. Clause headings stay outside, so
// clause numbers — which other documents cite — are the same on every lane.
//
// FAIL-SAFE DIRECTION. A dispatch whose lane cannot be told gets every PROCEDURE: the cost
// of not knowing is a longer kit, never a missing rule. The security output is the only one
// that leaves a procedure out, and it is selected only by a claim key that says "security"
// in so many words.
//
// A SECURITY STRETCH IS A STAND-IN. It is the note the security lane reads in place of the
// correctness stretch directly above it ("not run in this lane, owned by ..."). A reader who
// has the procedure must not also be told not to run it, so the whole kit carries the
// procedure and never the note. A test holds every security stretch to that position; a
// security-only RULE would need the cut taught about it first.

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// laneMarkerPrefix is what makes a line a lane marker. A line carrying it that is not a
// well-formed whole-line marker is a kit defect, never text to emit.
const laneMarkerPrefix = "<!-- lane:"

var laneMarkerRE = regexp.MustCompile(`^<!-- lane:([a-z-]+):(begin|end) -->$`)

// reviewKitLanes is the CLOSED set of lanes the review kit can be cut for. The other lanes
// a tier can dispatch carry their output contracts elsewhere and receive the whole kit.
func reviewKitLanes() []string {
	return []string{deskkit.LaneCorrectness.Name, deskkit.LaneSecurity.Name}
}

func isReviewKitLane(lane string) bool {
	for _, l := range reviewKitLanes() {
		if l == lane {
			return true
		}
	}
	return false
}

// reviewLaneForClaim reads the dispatched lane off the claim key, the only place a review
// dispatch states it: `<label>--pr-<N>` is the correctness lane (so is the explicit
// `--correctness` suffix) and `<label>--pr-<N>--security` is the security lane. Any other
// key — no PR number, another lane's suffix, a tagged re-dispatch — returns "", which emits
// the whole kit.
func reviewLaneForClaim(claimKey, repo string, pr int) string {
	ref := deskkit.DispatchClaimActiveRefsPrefix + strings.TrimSpace(claimKey)
	for _, family := range deskkit.ReviewClaimFamilies(repo, pr) {
		switch ref {
		case family, family + "--" + deskkit.LaneCorrectness.Name:
			return deskkit.LaneCorrectness.Name
		case family + "--" + deskkit.LaneSecurity.Name:
			return deskkit.LaneSecurity.Name
		}
	}
	return ""
}

// reviewKitForLane cuts the review kit for one lane. lane "" is the whole kit: every
// procedure and no stand-in note, which is the correctness lane's stretches. The marker
// lines are never emitted. A malformed, unknown, nested, stray or unclosed marker is
// UNVERIFIABLE: a kit whose lane boundaries cannot be read must not be dispatched on,
// because the cut could silently drop a clause from the lane it binds.
func reviewKitForLane(kit, lane string) (string, error) {
	if lane != "" && !isReviewKitLane(lane) {
		return "", deskkit.Unverifiable("review kit has no lane "+lane+" — want one of: "+
			strings.Join(reviewKitLanes(), ", "), nil)
	}
	keep := lane
	if keep == "" {
		keep = deskkit.LaneCorrectness.Name
	}
	var out []string
	open := ""
	for i, line := range strings.Split(kit, "\n") {
		if strings.Contains(line, laneMarkerPrefix) {
			m := laneMarkerRE.FindStringSubmatch(line)
			if m == nil {
				return "", laneMarkerDefect(i, "is not a whole-line `<!-- lane:<name>:begin|end -->` marker")
			}
			name, edge := m[1], m[2]
			switch {
			case !isReviewKitLane(name):
				return "", laneMarkerDefect(i, "names lane "+name+", which the review kit is not cut for")
			case edge == "begin" && open != "":
				return "", laneMarkerDefect(i, "opens "+name+" inside the open "+open+" stretch")
			case edge == "end" && open != name:
				return "", laneMarkerDefect(i, "closes "+name+" with no matching begin")
			}
			if edge == "begin" {
				open = name
			} else {
				open = ""
			}
			continue
		}
		if open != "" && open != keep {
			continue
		}
		// A stretch cut out between two blank lines leaves them adjacent; the kit itself
		// never carries two in a row, so folding them only repairs the cut's own seam.
		if strings.TrimSpace(line) == "" && len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			continue
		}
		out = append(out, line)
	}
	if open != "" {
		return "", deskkit.Unverifiable("review kit lane marker for "+open+" is never closed — "+
			"do not dispatch on a kit whose lane boundaries cannot be read", nil)
	}
	text := strings.TrimSpace(strings.Join(out, "\n"))
	if text == "" {
		return "", deskkit.Unverifiable("review kit is EMPTY after the lane cut — an agent "+
			"dispatched with an empty clause set looks like a successful dispatch and is not one", nil)
	}
	return text, nil
}

func laneMarkerDefect(index int, what string) error {
	return deskkit.Unverifiable("review kit line "+strconv.Itoa(index+1)+" "+what+" — do not dispatch on "+
		"a kit whose lane boundaries cannot be read", nil)
}

// checkReviewKitLanes proves every cut of the review kit can be produced. It runs before the
// claim, for the reason the kits are read before the claim: a kit that cannot be cut must
// not take a claim and then fail to produce a prompt.
func checkReviewKitLanes(kit string) error {
	for _, lane := range append([]string{""}, reviewKitLanes()...) {
		if _, err := reviewKitForLane(kit, lane); err != nil {
			return err
		}
	}
	return nil
}

// classKitText is the class kit as THIS dispatch emits it: the review kit cut for the lane
// the claim key names, every other kit whole.
func classKitText(o dispatchOpts, plan dispatchPlan) (text, lane string, err error) {
	kit, err := kitText(o.kit)
	if err != nil || !reviewKit(o.kit) {
		return kit, "", err
	}
	lane = reviewLaneForClaim(plan.claimKey, plan.repo, o.pr)
	text, err = reviewKitForLane(kit, lane)
	return text, lane, err
}
