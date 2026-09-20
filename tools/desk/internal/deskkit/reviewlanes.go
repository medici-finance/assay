package deskkit

// reviewlanes.go — review depth by tier.
//
// THE PROBLEM. Review dispatch reads no author property: every pull request gets
// the same lane set, so an unknown author's body claims are read with a
// maintainer's credence. The one thing that was provably wrong in the first
// unsolicited arrivals was exactly that shape — "fixed, all tests pass",
// asserted without being checked, on a pull request whose diff did not support
// it. Making depth a function of tier costs a known contributor nothing and
// puts the cheapest possible control — read each claim, check it against the
// diff, say which ones could not be confirmed — in front of the submissions
// where claims are least likely to have been verified.
//
// THE MODEL. The lane SET a review runs is keyed on the author's contributor-
// trust tier (trusttier.go): `unknown` and `blessed-once` get the deep set — the
// correctness lane at strong tier, the security lane, a claims-versus-diff fact
// check, and a mandatory fail-first reproduction; `contributor` and `maintainer`
// keep the standard path, unchanged from today. Only the lane set varies: the
// verdict shape, the reviewer identity, the ready flip and the merge authority
// are the same at every tier, and no tier merges anything.
//
// WHY THE DEEP SET SITS ON THE LOW TIERS. The deep lanes are a cost imposed
// where claims are least likely to have been verified, not a grant a tier earns
// — which is also why TierUnknown holds no capability in trusttier.go's table
// yet gets the deepest lane here. Fail-closed means MORE scrutiny by default: a
// recorded standing grant (`contributor`) or roster membership (`maintainer`)
// is what returns review to the standard path. The capability table's
// review-depth column records that a RECORDED tier answers the depth question
// for the identity; the lane table here is the precise per-tier answer, and it
// is deliberately fail-closed: the two tiers below a standing grant get the
// deep set whatever a capability cell says.
//
// INERT UNTIL A LEDGER EXISTS. With no ledger configured every external
// identity resolves `unknown` (trusttier.go's fail-closed default), so external
// pull requests get the deep set and roster identities — `maintainer` — keep
// the standard path. Nothing here changes any predicate that admits,
// authorizes or writes: the tier is READ, and the only thing keyed on it is how
// much scrutiny a review runs, never what anyone may do.
//
// The dispatch reference under cmd/deskdispatch/references/review-lanes.md
// states the per-tier lane sets, the fact-check output contract and the
// fail-first reproduction's two required records;
// TestReviewLanesReferenceMatchesTable holds that document to the table below,
// so a reference describing a lane set this code does not select fails.

import (
	"regexp"
	"strings"
)

// Lane is one review lane a dispatched reviewer runs. Name is the canonical
// short name the dispatch reference and the reviewer prompts carry; Summary is
// one line saying what the lane does; ExecTier is the execution tier the lane
// is dispatched at — "strong" where the lane demands it, "" where the ordinary
// risk-keyed tiering (the review desk's existing rule) decides.
type Lane struct {
	Name     string
	Summary  string
	ExecTier string
}

// The four lanes. The names are the contract: the dispatch reference keys on
// them, and the review desk's dispatch ceremony uses them for lane-suffixed
// claim keys exactly as it does for the security lane today.
var (
	// LaneCorrectness is the standard correctness review. On the deep set it is
	// dispatched at strong tier; on the standard set its tier stays risk-keyed
	// as it is today.
	LaneCorrectness = Lane{
		Name:    "correctness",
		Summary: "the correctness review of the change against the main it will merge into",
	}
	// LaneSecurity is the security lane, unchanged from today's dispatch: it
	// rides on risk-classed pull requests at strong tier whatever the author's
	// tier is.
	LaneSecurity = Lane{
		Name:     "security",
		Summary:  "the security review of the change, dispatched as its own lane on risk-classed pull requests",
		ExecTier: "strong",
	}
	// LaneFactCheck is the claims-versus-diff fact check: every assertion the
	// pull-request body makes, each answered confirmed, contradicted or
	// unverified. See the ClaimState contract below.
	LaneFactCheck = Lane{
		Name:     "fact-check",
		Summary:  "a claims-versus-diff fact check: every assertion the pull-request body makes, each confirmed, contradicted or unverified",
		ExecTier: "strong",
	}
	// LaneFailFirst is the mandatory fail-first reproduction: the reported
	// problem demonstrated failing at the merge base, then the same case
	// passing at the head. Two records, both required.
	LaneFailFirst = Lane{
		Name:     "fail-first",
		Summary:  "a fail-first reproduction: the reported problem recorded failing at the merge base, then the same case recorded passing at the head",
		ExecTier: "strong",
	}
)

// strongTier returns lane with the dispatch tier pinned to strong — the deep
// set's correctness lane runs at strong tier whatever the risk keying would
// pick, so a helper (not a variant of LaneCorrectness) keeps the pinned-ness
// visible at the table site rather than in a second near-identical variable.
func strongTier(l Lane) Lane {
	l.ExecTier = "strong"
	return l
}

// laneTable is the whole lane policy, tier by tier, as DATA rather than
// scattered conditionals — so a reader (and TestReviewLanesReferenceMatchesTable,
// which checks it against the dispatch reference) can see it in one place.
//
// The deep set (unknown, blessed-once): correctness at strong tier, security,
// fact-check, fail-first. The standard set (contributor, maintainer):
// correctness and security, unchanged from today — a known contributor's pull
// request is reviewed exactly as it is now, which is the negative control
// TestReviewLanesContributorTierKeepsTheStandardPath pins.
var laneTable = map[Tier][]Lane{
	TierUnknown:     {strongTier(LaneCorrectness), LaneSecurity, LaneFactCheck, LaneFailFirst},
	TierBlessedOnce: {strongTier(LaneCorrectness), LaneSecurity, LaneFactCheck, LaneFailFirst},
	TierContributor: {LaneCorrectness, LaneSecurity},
	TierMaintainer:  {LaneCorrectness, LaneSecurity},
}

// LanesFor returns the review lane set for tier, in dispatch order (correctness
// first, security second, the deep lanes after — the order the desk dispatches
// them in). The table IS the policy; this function only reads it, and returns
// a fresh slice so no caller can mutate the table through it.
//
// A tier value outside the four published ones resolves the deep set — the same
// fail-closed direction every other tier reader here takes. An out-of-range
// tier is a caller defect, and the safe answer to it is MORE scrutiny, never an
// empty lane set that would dispatch no review at all.
func LanesFor(tier Tier) []Lane {
	set, ok := laneTable[tier]
	if !ok || len(set) == 0 {
		set = laneTable[TierUnknown]
	}
	out := make([]Lane, len(set))
	copy(out, set)
	return out
}

// ReviewLanesForAuthor is the DISPATCH SELECTION PATH: the one call a review
// dispatch makes to go from an author identity to the lane set that author's
// pull request is reviewed under. It resolves the author's tier exactly as
// trusttier.go defines it — roster membership first (maintainer; the ledger can
// neither grant nor lower it), the ledger row second, unknown as the fail-closed
// default — and selects the lane set from the same table LanesFor reads, so the
// tier a dispatch keys on and the lane set it dispatches can never disagree.
//
// The provenance is returned beside the lanes so a caller can tell a
// legitimately-absent ledger from an unreadable one rather than reading both as
// an unexplained "unknown" — and so a broken ledger is visible as the anomaly
// it is while STILL resolving to the deep set: a ledger failure can only narrow
// what automation does, never widen it, and "narrow" here means MORE scrutiny,
// never less.
func ReviewLanesForAuthor(repo, login string, id int64) ([]Lane, Tier, Provenance) {
	tier, prov := ResolveTier(repo, login, id)
	return LanesFor(tier), tier, prov
}

// ---------------------------------------------------------------------------
// The fact-check lane's claims contract
// ---------------------------------------------------------------------------

// ClaimState is the three-state answer the fact-check lane records for ONE
// claim. Every claim gets EXACTLY ONE of these — never none, never two — and
// the third state is a legitimate, expected outcome:
//
//	confirmed    — the diff, or a run, supports the claim.
//	contradicted — the diff, or a run, disproves the claim. A contradicted
//	               claim is a review finding ON ITS OWN, independent of
//	               whether the code is correct: the claim, not only the code,
//	               is what the reviewer answers.
//	unverified   — could not be checked from the change alone. NOT a failure,
//	               and never rounded to confirmed: an instrument that did not
//	               look has cleared nothing.
type ClaimState string

const (
	ClaimConfirmed    ClaimState = "confirmed"
	ClaimContradicted ClaimState = "contradicted"
	ClaimUnverified   ClaimState = "unverified"
)

// BodyClaim is one assertion extracted from a pull-request body, with the state the
// fact-check lane recorded for it. Extraction itself asserts nothing: every
// claim ExtractClaims returns starts at ClaimUnverified, and only the
// reviewer's comparison of the claim against the diff — or a run — moves it to
// confirmed or contradicted.
type BodyClaim struct {
	Text  string
	State ClaimState
}

// CarriesState reports whether c carries exactly one of the three claim states.
// It is deliberately FALSE for the zero value and for any invented state, so a
// BodyClaim that skipped its disposition is detectable as such rather than reading
// as a claim that was checked — the same three-state discipline everywhere
// else in this package restated for one struct.
func (c BodyClaim) CarriesState() bool {
	switch c.State {
	case ClaimConfirmed, ClaimContradicted, ClaimUnverified:
		return true
	default:
		return false
	}
}

// EveryClaimCarriesAState reports whether every claim in claims carries a
// state (an empty list carries one vacuously: there is nothing to check). This
// is the fact-check output contract's invariant as a function — the dispatch
// reference states it, and the reviewer's output is checked against it.
func EveryClaimCarriesAState(claims []BodyClaim) bool {
	for _, c := range claims {
		if !c.CarriesState() {
			return false
		}
	}
	return true
}

// claimMarkerRe is the deliberately coarse assertion-marker sweep. A prose
// sentence is a candidate claim when it contains one of these; a bullet or
// numbered item is a candidate claim unconditionally (a body's highlight
// bullets ARE its assertions). The set errs toward over-extraction on purpose:
// a false candidate costs one unverified row, while a missed claim is the
// exact failure the fact-check lane exists to catch. Extraction QUALITY is a
// review-gate judgement, not a contract — the contract is only that every
// claim extracted carries exactly one state.
var claimMarkerRe = regexp.MustCompile(`(?i)\b(fix(?:e[sd])?|fixing|add(?:e[sd])?|adding|remov(?:e[sd]|ing)|updat(?:e[sd]|ing)|chang(?:e[sd]|ing)|introduc(?:e[sd]|ing)|make[sd]?|now|no longer|ensur(?:e[sd])?|prevent(?:e[sd])?|allow(?:e[sd])?|enabl(?:e[sd])?|support(?:e[sd])?|handl(?:e[sd])?|resolv(?:e[sd])?|clos(?:e[sd])?|pass(?:e[sd])?|fail(?:e[sd]|ing)?|tests?|all tests|ci|build[s]?|lint|check[s]?|clean|green|regression|bug|work[s]?|correct(?:ly)?|properly|crash(?:e[sd])?|hang(?:s)?|leak(?:s|ed)?)\b`)

// ExtractClaims is the starting sweep the fact-check lane is dispatched with:
// it enumerates the CANDIDATE assertions in a pull-request body so the reviewer
// begins from a list rather than a re-reading. It skips headings, fenced code
// blocks, HTML comments and table rows — structure, not assertions — keeps
// every bullet and numbered item, and keeps a prose sentence only when it
// carries an assertion marker (claimMarkerRe).
//
// Every claim is returned at ClaimUnverified. The helper asserts nothing about
// any of them; confirming or contradicting a claim is the reviewer's act,
// performed against the diff or a run.
func ExtractClaims(body string) []BodyClaim {
	var claims []BodyClaim
	inFence := false
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)

		// Fenced code blocks: toggle on the fence line, skip the content. The
		// fence line itself carries no assertion either.
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		// Headings, HTML comments and table rows are structure, not claims.
		if line == "" || strings.HasPrefix(line, "#") ||
			strings.HasPrefix(line, "<!--") || strings.Contains(line, "<!--") ||
			strings.HasPrefix(line, "|") {
			continue
		}

		if text, ok := stripBullet(line); ok {
			// A bullet or numbered item is an assertion by construction — the
			// body's own highlight list.
			if text != "" {
				claims = append(claims, BodyClaim{Text: text, State: ClaimUnverified})
			}
			continue
		}
		// Prose: each sentence that carries an assertion marker is a candidate.
		for _, sent := range splitSentences(line) {
			if claimMarkerRe.MatchString(sent) {
				claims = append(claims, BodyClaim{Text: sent, State: ClaimUnverified})
			}
		}
	}
	return claims
}

// stripBullet removes a leading markdown bullet or numbered-item marker and
// reports whether the line was one. A quoted line ("> …") is prose, not a
// bullet, and falls through to the sentence path.
func stripBullet(line string) (string, bool) {
	for _, pfx := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(line, pfx) {
			return strings.TrimSpace(line[len(pfx):]), true
		}
	}
	// "1. ", "12) " — a digit run followed by '.' or ')' and a space.
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i > 0 && i < len(line) && (line[i] == '.' || line[i] == ')') && i+1 < len(line) && line[i+1] == ' ' {
		return strings.TrimSpace(line[i+2:]), true
	}
	return line, false
}

// splitSentences splits one line of prose into sentences on sentence-ending
// punctuation followed by whitespace or end-of-line. Abbreviation stops ("e.g.",
// "i.e.") are left alone deliberately: over-splitting costs an unverified row,
// under-splitting can hide a claim.
func splitSentences(line string) []string {
	var out []string
	start := 0
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c != '.' && c != '!' && c != '?' {
			continue
		}
		if i+1 == len(line) || line[i+1] == ' ' {
			if s := strings.TrimSpace(line[start : i+1]); s != "" {
				out = append(out, s)
			}
			start = i + 1
		}
	}
	if s := strings.TrimSpace(line[start:]); s != "" {
		out = append(out, s)
	}
	return out
}
