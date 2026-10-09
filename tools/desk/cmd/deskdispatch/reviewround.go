package main

// reviewround.go — the review ROUND a dispatch opens (#2444): its scope and its number.
//
// A reviewer lane that already holds a verdict at an earlier head of the same change does not
// need to read the whole change again. The dispatcher decides, from forge reads, whether the
// lane's next round is a DELTA round (the previous verdict's findings, the diff between the
// two heads, the description check) or a FULL pass, and says so in the assignment. It also
// states which round of the lane this is and the head of the lane's first review, which is
// what review-kit clause 20 (the lane round cap) reads.
//
// THE DECISIONS ARE PURE. decideReviewScope and decideLaneRound take facts and return a
// decision; they read nothing. reviewroundread.go gathers the facts. Every fact the gatherer
// cannot read is carried as "not known", and not-known is a FULL pass (scope) or "the cap
// does not apply" (round) — the two answers that narrow nothing.
//
// PER LANE. A lane's scope and round are computed from that lane's OWN verdicts only: reviews
// the forge attributes to the reviewer identity whose body carries that lane's verdict line.
// The other lane's verdicts, and anyone else's reviews, are not counted.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The "large delta" measure. A delta is large when the first-parent chain between the two
// heads holds more than reviewDeltaMaxCommits commits, or touches more than
// reviewDeltaMaxPaths of the change's own paths. Both are counts the typed forge reads
// already carry; the forge reads used here carry no changed-line counts.
const (
	reviewDeltaMaxCommits = 20
	reviewDeltaMaxPaths   = 10
)

// scopeReason names ONE full-pass condition. The set is closed: every FULL decision carries
// at least one of these and a DELTA decision carries none.
type scopeReason string

const (
	// reasonUndetermined — a fact the decision needs could not be read.
	reasonUndetermined scopeReason = "could-not-determine"
	// reasonNoVerdict — the lane has no verdict of its own on the change.
	reasonNoVerdict scopeReason = "no-verdict-of-its-own"
	// reasonSameHead — the lane's latest verdict is at the head being dispatched, so there
	// is no earlier head to scope a delta from.
	reasonSameHead scopeReason = "verdict-already-at-this-head"
	// reasonNoDiff — the diff between the two heads could not be computed.
	reasonNoDiff scopeReason = "inter-head-diff-not-computable"
	// reasonLargeDelta — the delta exceeds reviewDeltaMaxCommits or reviewDeltaMaxPaths.
	reasonLargeDelta scopeReason = "large-delta"
	// reasonMergeInOwnFiles — a merge in the delta changed a file the change touches, or its
	// merged-in side did, whichever way a conflict in that file was resolved.
	reasonMergeInOwnFiles scopeReason = "merge-in-files-the-change-touches"
	// reasonNewPath — the change touches a path it did not touch at the previous head.
	reasonNewPath scopeReason = "path-not-reviewed-before"
)

// scopeFacts is what the scope decision reads. The zero value is "the lane has no verdict".
type scopeFacts struct {
	// unknown, when not empty, says which fact could not be read. It wins over everything.
	unknown string
	// verdict is true when the lane holds at least one verdict of its own on the change.
	verdict bool
	// prevHead is the head of the lane's latest own verdict; head is the head dispatched.
	prevHead, head string
	// diffErr, when not empty, says why the inter-head diff could not be computed.
	diffErr string
	// commits is the number of commits on the first-parent chain from prevHead to head.
	commits int
	// delta is every path the forge's per-commit read lists for those commits.
	delta []string
	// merged is every path that read lists for the chain's commits with two or more parents,
	// and every path each such commit's merged-in sides changed (the comparison of its first
	// parent with each other parent).
	merged []string
	// prevFiles and headFiles are the change's own file lists at prevHead and at head.
	prevFiles, headFiles []string
}

// scopeDecision is the outcome: full or delta, why, and the figures the assignment states.
type scopeDecision struct {
	full    bool
	reasons []scopeReason
	// detail is one sentence per reason, in the same order, for the assignment.
	detail []string
	// commits and ownPaths are the delta's size: chain commits, and paths of the change's
	// own file lists the delta touches. Meaningful only when the diff was computed.
	commits, ownPaths int
}

// decideReviewScope is the scope decision. FULL when any full-pass condition holds; DELTA
// only when none does. Each case of the switch ends the decision with one reason (nothing
// after it can be evaluated); the three conditions below the switch are evaluated together,
// so a delta that is both large and carries a merge names both.
func decideReviewScope(f scopeFacts) scopeDecision {
	full := func(r scopeReason, why string) scopeDecision {
		return scopeDecision{full: true, reasons: []scopeReason{r}, detail: []string{why}}
	}
	switch {
	case f.unknown != "":
		return full(reasonUndetermined, f.unknown)
	case !f.verdict:
		return full(reasonNoVerdict, "this lane has no verdict of its own on the change")
	case !isCommitID(f.prevHead) || !isCommitID(f.head):
		return full(reasonUndetermined, "the previous verdict's head or the dispatched head is not a full commit id")
	case strings.EqualFold(f.prevHead, f.head):
		return full(reasonSameHead, "this lane's latest verdict is already at the dispatched head")
	case f.diffErr != "":
		return full(reasonNoDiff, f.diffErr)
	case f.commits < 1:
		return full(reasonNoDiff, "the interval between the two heads holds no commit")
	case len(f.headFiles) == 0:
		return full(reasonUndetermined, "the change's file list at the dispatched head is empty")
	}

	prev := pathSet(f.prevFiles)
	own := pathSet(f.prevFiles)
	for _, p := range f.headFiles {
		own[p] = true
	}
	d := scopeDecision{commits: f.commits, ownPaths: countIn(f.delta, own)}
	add := func(r scopeReason, why string) {
		d.full = true
		d.reasons = append(d.reasons, r)
		d.detail = append(d.detail, why)
	}
	if d.commits > reviewDeltaMaxCommits || d.ownPaths > reviewDeltaMaxPaths {
		add(reasonLargeDelta, fmt.Sprintf("the delta is %d commit(s) touching %d of the change's own path(s); "+
			"the limits are %d commits and %d paths", d.commits, d.ownPaths, reviewDeltaMaxCommits, reviewDeltaMaxPaths))
	}
	if n := countIn(f.merged, own); n > 0 {
		add(reasonMergeInOwnFiles, fmt.Sprintf("a merge in the delta changed, or brought in a change to, %d file(s) the change touches", n))
	}
	fresh := 0
	for p := range pathSet(f.headFiles) {
		if !prev[p] {
			fresh++
		}
	}
	if fresh > 0 {
		add(reasonNewPath, fmt.Sprintf("the change now touches %d path(s) it did not touch at the previous head", fresh))
	}
	return d
}

// pathSet is the set of non-empty paths in list.
func pathSet(list []string) map[string]bool {
	set := make(map[string]bool, len(list))
	for _, p := range list {
		if p != "" {
			set[p] = true
		}
	}
	return set
}

// countIn counts the distinct paths of list that are in set.
func countIn(list []string, set map[string]bool) int {
	n := 0
	for p := range pathSet(list) {
		if set[p] {
			n++
		}
	}
	return n
}

// commitIDRe is a full commit id as a forge reports one: 40 hex digits (SHA-1) or 64
// (SHA-256). Only a value of this shape is ever written into the assignment as a commit.
var commitIDRe = regexp.MustCompile(`^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)

func isCommitID(s string) bool { return commitIDRe.MatchString(s) }

// laneRound is which round of a lane this dispatch opens, and the head of the lane's first
// review. known is false when it could not be determined; why then says what was missing.
type laneRound struct {
	known     bool
	why       string
	number    int
	firstHead string // "" on round 1: the lane has not reviewed the change yet
}

// decideLaneRound counts a lane's rounds. A round is one HEAD the lane reviewed: two verdicts
// at one head are one round, and a dispatch at a head the lane already reviewed re-opens that
// round rather than starting a new one. verdictHeads are the heads of the lane's own
// verdicts, oldest first. A verdict the forge cannot pin to a commit makes the count
// unknown: a round that cannot be counted is not counted low or high.
func decideLaneRound(verdictHeads []string, head string) laneRound {
	if !isCommitID(head) {
		return laneRound{why: "the dispatched head is not a full commit id"}
	}
	seen := map[string]bool{}
	for _, h := range verdictHeads {
		if !isCommitID(h) {
			return laneRound{why: "the forge does not pin one of this lane's earlier verdicts to a commit"}
		}
		seen[strings.ToLower(h)] = true
	}
	r := laneRound{known: true}
	if len(verdictHeads) > 0 {
		r.firstHead = verdictHeads[0]
	}
	seen[strings.ToLower(head)] = true
	r.number = len(seen)
	return r
}

// laneVerdict is one verdict a lane holds on the change.
type laneVerdict struct {
	id       int64
	head     string
	body     string
	blocking bool // every verdict line in the body is request-changes or fail
}

// laneVerdictsOf picks a lane's own verdicts out of a change's reviews, keeping the forge's
// order (oldest first). A review counts only when the forge names the reviewer identity as
// its author, it was submitted, and its body carries this lane's verdict line and no other
// lane's. A body carrying both lanes' lines is neither lane's verdict here.
func laneVerdictsOf(reviews []deskkit.Review, reviewer, lane string) []laneVerdict {
	var out []laneVerdict
	for _, r := range reviews {
		if !byReviewer(r, reviewer) || strings.EqualFold(strings.TrimSpace(r.State), "PENDING") {
			continue
		}
		if lane == "" || reviewBodyLane(r.Body) != lane {
			continue
		}
		out = append(out, laneVerdict{id: r.ID, head: strings.TrimSpace(r.CommitID), body: r.Body, blocking: verdictBlocks(r.Body)})
	}
	return out
}

// A verdict's head is the commit the forge records the review against (its commit id field),
// and that field has been seen to disagree with the head the review's own record names. So
// where a verdict's own record names a head, the two are compared (disputedVerdictHead), and
// a disagreement is could-not-determine. A record names a head in these places and no other:
//
//   - its text, where one of three shapes is followed by a FULL commit id (40 or 64 hex
//     digits, optionally opened by one backtick, not followed by a further letter or digit):
//     "Head reviewed:" anywhere; "Head:" opening a line, after list, quote or emphasis
//     markup; "at head" on the body's opening line only — its first non-empty line that is
//     not a verdict line;
//   - its typed finding block: each finding's evidence head, and each finding's origin head.
//
// Any other commit id in the body — a merge base, the base branch's tip, an input revision,
// "at head" further down — is not read, and an abbreviated id is not read anywhere. A typed
// block that is present and cannot be read is not "names nothing": what it records cannot be
// checked, and that too is could-not-determine.
var (
	headLabelRe   = regexp.MustCompile("(?mi)(?:\\bhead reviewed\\*{0,2}:|^[ \\t>*_-]*head\\*{0,2}:)\\*{0,2}[ \\t]*`?((?:[0-9a-f]{64}|[0-9a-f]{40}))\\b")
	headOpeningRe = regexp.MustCompile("(?i)\\bat head[ \\t]+`?((?:[0-9a-f]{64}|[0-9a-f]{40}))\\b")
)

// verdictNamedHeads reads what one verdict's own record says about heads. reviewed is every
// full commit id it names as the head it reviewed: the recognised head lines of its text and
// each typed finding's evidence head. origins is each typed finding's origin head that is a
// full commit id: the head the finding was first raised at, which on a carried finding is an
// earlier head of the lane's. readable is false when the body carries a typed block that
// cannot be read.
func verdictNamedHeads(body string) (reviewed, origins []string, readable bool) {
	for _, m := range headLabelRe.FindAllStringSubmatch(body, -1) {
		reviewed = append(reviewed, m[1])
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" || reviewVerdictLine.MatchString(line) {
			continue
		}
		for _, m := range headOpeningRe.FindAllStringSubmatch(line, -1) {
			reviewed = append(reviewed, m[1])
		}
		break // the opening line only
	}
	block, present, err := deskkit.ParseFindingBlock(body)
	if present && (err != nil || block == nil) {
		return reviewed, nil, false
	}
	if present {
		for _, f := range block.Findings {
			if h := strings.TrimSpace(f.EvidenceHead); isCommitID(h) {
				reviewed = append(reviewed, h)
			}
			if h := strings.TrimSpace(f.OriginHead); isCommitID(h) {
				origins = append(origins, h)
			}
		}
	}
	return reviewed, origins, true
}

// disputedVerdictHead says which of a lane's verdicts, oldest first, names in its own record
// a head the forge's record of it does not bear out; "" when none does. A head the record
// names as reviewed must be the commit the forge records for that verdict. A finding's origin
// head must be the commit the forge records for that verdict or for an earlier one of the
// lane's. Every head a record names is checked: one that agrees does not excuse one that
// does not. A record that names no head is not disputed; one whose typed block cannot be
// read is, because what it records cannot be checked.
func disputedVerdictHead(verdicts []laneVerdict) string {
	held := map[string]bool{}
	for _, v := range verdicts {
		held[strings.ToLower(v.head)] = true
		reviewed, origins, readable := verdictNamedHeads(v.body)
		if !readable {
			return fmt.Sprintf("this lane's review %d carries a typed finding block that cannot be read, so the head it records cannot be checked", v.id)
		}
		disputed := false
		for _, h := range reviewed {
			disputed = disputed || !strings.EqualFold(h, v.head)
		}
		for _, h := range origins {
			disputed = disputed || !held[strings.ToLower(h)]
		}
		if disputed {
			return fmt.Sprintf("this lane's review %d names, in its own text or typed block, a different head than the forge records for it", v.id)
		}
	}
	return ""
}

// verdictBlocks reports whether every verdict line in body is a blocking one.
func verdictBlocks(body string) bool {
	lines := reviewVerdictLine.FindAllStringSubmatch(body, -1)
	if len(lines) == 0 {
		return false
	}
	for _, m := range lines {
		switch strings.ToLower(m[2]) {
		case "request-changes", "fail":
		default:
			return false
		}
	}
	return true
}

// findingIDRe is the shape of a finding id the assignment will print. A reviewer writes its
// own ids, but it writes them after reading the change, so an id of any other shape is
// counted and not printed.
var findingIDRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,39}$`)

// previousFindings reads the ids of the findings a verdict body states in its typed finding
// block. typed is false when the body carries no readable block; the reviewer then answers
// the findings the body states in prose. unprintable counts ids not of findingIDRe's shape.
func previousFindings(body string) (ids []string, unprintable int, typed bool) {
	block, present, err := deskkit.ParseFindingBlock(body)
	if err != nil || !present || block == nil {
		return nil, 0, false
	}
	seen := map[string]bool{}
	for _, f := range block.Findings {
		id := strings.TrimSpace(f.ID)
		switch {
		case !findingIDRe.MatchString(id):
			unprintable++
		case !seen[id]:
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, unprintable, true
}

// reviewRound is what the pre-claim read hands the prompt: the lane, the scope decision, the
// round, and the previous verdict the delta is measured from. The zero value is "nothing was
// read" and renders as a full pass with the round not determined.
type reviewRound struct {
	read  bool // the pre-claim read ran (false on --dry-run and when no forge read is wired)
	lane  string
	head  string
	scope scopeDecision
	round laneRound
	// prev is the lane's latest own verdict, the one a delta round answers; nil when none.
	prev *laneVerdict
}

// writeReviewRound writes the round's statement into the reviewer's assignment: the round
// number and first-review head (clause 20), then the scope and what it rests on (clause 19).
// Every commit id written here passed isCommitID and every finding id passed findingIDRe; no
// path, title or other text from the change is written.
func writeReviewRound(b *strings.Builder, rr reviewRound) {
	b.WriteString("### This round — stated by the dispatcher\n\n")
	if !rr.read {
		b.WriteString("- Round: NOT DETERMINED (no forge read ran for this dispatch). Clause 20's lane round cap does not apply.\n")
		b.WriteString("- Scope: FULL PASS — reason `" + string(reasonUndetermined) + "`: no forge read ran for this dispatch.\n\n")
		return
	}
	lane := rr.lane
	if lane == "" {
		lane = "not readable off the claim key"
	}
	switch {
	case !rr.round.known:
		fmt.Fprintf(b, "- Lane: %s. Round: NOT DETERMINED (%s). Clause 20's lane round cap does not apply.\n", lane, rr.round.why)
	case rr.round.firstHead == "":
		fmt.Fprintf(b, "- Lane: %s. Round: %d — this lane has not reviewed the change before.\n", lane, rr.round.number)
	default:
		fmt.Fprintf(b, "- Lane: %s. Round: %d. This lane's FIRST review was at head `%s`.\n", lane, rr.round.number, rr.round.firstHead)
	}
	if !rr.scope.full && rr.prev == nil {
		// A delta is measured from a previous verdict. Without one there is nothing to state
		// a delta against, so the round is a full pass.
		rr.scope = decideReviewScope(scopeFacts{unknown: "no previous verdict to measure a delta from"})
	}
	if rr.scope.full {
		names := make([]string, 0, len(rr.scope.reasons))
		for _, r := range rr.scope.reasons {
			names = append(names, "`"+string(r)+"`")
		}
		fmt.Fprintf(b, "- Scope: FULL PASS — reason %s: %s.\n\n", strings.Join(names, ", "), strings.Join(rr.scope.detail, "; "))
		return
	}
	fmt.Fprintf(b, "- Scope: DELTA (clause 19) — from this lane's previous verdict (review %d, at head `%s`) to head `%s`: "+
		"%d commit(s) touching %d of the change's own path(s).\n", rr.prev.id, rr.prev.head, rr.head, rr.scope.commits, rr.scope.ownPaths)
	ids, unprintable, typed := previousFindings(rr.prev.body)
	switch {
	case !typed:
		b.WriteString("- Findings to answer: review " + fmt.Sprint(rr.prev.id) + " carries no readable typed finding block — answer every finding its body states.\n")
	case len(ids) == 0 && unprintable == 0:
		b.WriteString("- Findings to answer: review " + fmt.Sprint(rr.prev.id) + "'s typed block lists none.\n")
	default:
		line := fmt.Sprintf("- Findings to answer, each resolved / not resolved with evidence: %d in review %d's typed block", len(ids)+unprintable, rr.prev.id)
		if len(ids) > 0 {
			line += " — `" + strings.Join(ids, "`, `") + "`"
		}
		if unprintable > 0 {
			line += fmt.Sprintf(" — %d more whose ids are not printed here; read them in the review", unprintable)
		}
		b.WriteString(line + ".\n")
	}
	fmt.Fprintf(b, "- The delta: `git diff %s %s`. If that command fails, or you find this scope wrong, do the full pass and say so.\n\n",
		rr.prev.head, rr.head)
}
