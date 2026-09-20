package deskkit

// reviewlanes_test.go — the lane table, the dispatch selection path, and the
// fact-check claims contract.
//
// FAIL-FIRST (the lane table): the mutations committed for this change (see
// internal/deskkit/mutations.json, entries naming reviewlanes.go) break each
// guarded behaviour — the contributor set widened to the deep lanes, the
// unverified start state rounded to confirmed, CarriesState accepting any
// state, the fail-first lane dropped from the unknown set, and the selection
// path ignoring the resolved tier — and this suite catches every one.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// laneNames renders a lane set for logging and comparison.
func laneNames(lanes []Lane) []string {
	out := make([]string, len(lanes))
	for i, l := range lanes {
		out[i] = l.Name
	}
	return out
}

// namesEqual reports whether a and b are equal element-wise, in order.
func namesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestReviewLanesTableIsTotalAndDeterministic(t *testing.T) {
	for _, tier := range []Tier{TierUnknown, TierBlessedOnce, TierContributor, TierMaintainer} {
		set := LanesFor(tier)
		if len(set) == 0 {
			t.Errorf("LanesFor(%s) is empty — every tier must carry an explicit lane set", tier)
			continue
		}
		again := LanesFor(tier)
		if !namesEqual(laneNames(set), laneNames(again)) {
			t.Errorf("LanesFor(%s) is not deterministic: %v then %v", tier, laneNames(set), laneNames(again))
		}
		// The returned slice is a copy: mutating it must not reach the table.
		set[0].Name = "mutated"
		if LanesFor(tier)[0].Name == "mutated" {
			t.Errorf("LanesFor(%s) leaks the table — a caller mutation reached it", tier)
		}
	}
	// An out-of-range tier resolves the fail-closed default: the deep set, never
	// a free pass to the standard path.
	if got := laneNames(LanesFor(Tier(99))); !namesEqual(got, laneNames(LanesFor(TierUnknown))) {
		t.Errorf("LanesFor(out-of-range) = %v, want the unknown tier's set — the default must fail closed", got)
	}
}

// TestReviewLanesUnknownTierGetsTheDeepSet: an unknown author's pull request is
// reviewed with the deep set — the correctness lane at strong tier, the security
// lane, the claims-versus-diff fact check, and the mandatory fail-first
// reproduction. Each lane name is LOGGED so the deep set is visible in the test
// output, not only in the assertion.
func TestReviewLanesUnknownTierGetsTheDeepSet(t *testing.T) {
	lanes := LanesFor(TierUnknown)
	for _, l := range lanes {
		t.Logf("unknown-tier lane: %s — %s", l.Name, l.Summary)
	}
	want := []string{LaneCorrectness.Name, LaneSecurity.Name, LaneFactCheck.Name, LaneFailFirst.Name}
	if got := laneNames(lanes); !namesEqual(got, want) {
		t.Errorf("LanesFor(unknown) = %v, want the deep set %v", got, want)
	}
	// The deep set's correctness lane runs at strong tier.
	for _, l := range lanes {
		if l.Name == LaneCorrectness.Name && l.ExecTier != "strong" {
			t.Errorf("the deep set's correctness lane ExecTier = %q, want strong", l.ExecTier)
		}
	}
}

// TestReviewLanesBlessedOnceTierGetsTheDeepSet: blessed-once is still one
// admitted item from an otherwise-unknown identity — the same deep set. The
// one-time admission is checked, not trusted.
func TestReviewLanesBlessedOnceTierGetsTheDeepSet(t *testing.T) {
	got := laneNames(LanesFor(TierBlessedOnce))
	want := laneNames(LanesFor(TierUnknown))
	if !namesEqual(got, want) {
		t.Errorf("LanesFor(blessed-once) = %v, want the same deep set as unknown %v", got, want)
	}
}

// TestReviewLanesContributorTierKeepsTheStandardPath: the negative control. A
// contributor with a standing recorded grant keeps today's standard path — the
// change must not become a blanket slowdown nobody asked for.
func TestReviewLanesContributorTierKeepsTheStandardPath(t *testing.T) {
	lanes := LanesFor(TierContributor)
	for _, l := range lanes {
		t.Logf("contributor-tier lane: %s — %s", l.Name, l.Summary)
	}
	want := []string{LaneCorrectness.Name, LaneSecurity.Name}
	if got := laneNames(lanes); !namesEqual(got, want) {
		t.Errorf("LanesFor(contributor) = %v, want the standard path %v", got, want)
	}
	// The standard set's correctness lane stays risk-keyed, as today.
	for _, l := range lanes {
		if l.Name == LaneCorrectness.Name && l.ExecTier != "" {
			t.Errorf("the standard set's correctness lane ExecTier = %q, want the risk-keyed default", l.ExecTier)
		}
	}
}

// TestReviewLanesMaintainerTierKeepsTheStandardPath: roster identities keep the
// standard path too — the highest tier buys no extra scrutiny off itself, and
// none is needed.
func TestReviewLanesMaintainerTierKeepsTheStandardPath(t *testing.T) {
	got := laneNames(LanesFor(TierMaintainer))
	want := []string{LaneCorrectness.Name, LaneSecurity.Name}
	if !namesEqual(got, want) {
		t.Errorf("LanesFor(maintainer) = %v, want the standard path %v", got, want)
	}
}

// ---------------------------------------------------------------------------
// The claims contract
// ---------------------------------------------------------------------------

// sampleBody is a realistic pull-request body: a summary heading, prose with
// the canonical unchecked-claim shape, highlight bullets, a fenced command
// block, and a table row. The heading, the fence and the table row must NOT be
// extracted; the prose claim and every bullet must be.
const sampleBody = `## Summary

This fixes the retry loop so it no longer spins forever. The queue now drains
cleanly and all tests pass.

- fixed the retry backoff calculation
- removed the duplicate unlock call
- 2. updated the docs to match

` + "```" + `
make test   # a fenced command, not a claim
` + "```" + `

| field | before | after |
|---|---|---|
| retries | unbounded | 3 |

Thanks for the review.`

// TestClaimStateUnverifiedIsRepresentable: an unverifiable claim has its OWN
// state, distinct from both verified outcomes, and extraction never rounds it
// to confirmed. This is the exact failure the lane exists to prevent: a claim
// the reviewer could not check, recorded as though it had been.
func TestClaimStateUnverifiedIsRepresentable(t *testing.T) {
	if ClaimUnverified == ClaimConfirmed || ClaimUnverified == ClaimContradicted {
		t.Fatalf("ClaimUnverified must be its own state, not an alias of a verified outcome")
	}
	if string(ClaimUnverified) != "unverified" {
		t.Errorf(`ClaimUnverified = %q, want "unverified"`, string(ClaimUnverified))
	}

	// The canonical unchecked claim — "fixed, all tests pass" — extracts as a
	// claim and starts UNVERIFIED. Extraction asserts nothing.
	claims := ExtractClaims("This fixes the retry loop and all tests pass.")
	if len(claims) == 0 {
		t.Fatalf("the canonical claim body extracted no claims — the sweep must find it")
	}
	for _, c := range claims {
		if c.State != ClaimUnverified {
			t.Errorf("claim %q extracted at state %s — extraction must start every claim unverified, never confirm it", c.Text, c.State)
		}
	}

	// A hand-built claim that skipped its disposition does not carry a state,
	// and is never silently readable as confirmed.
	if (BodyClaim{Text: "x"}).CarriesState() {
		t.Error("the zero-value BodyClaim reports CarriesState — an unstated disposition must be detectable as missing")
	}
	if (BodyClaim{Text: "x", State: ClaimState("probably")}).CarriesState() {
		t.Error(`a BodyClaim with an invented state "probably" reports CarriesState — only the three published states count`)
	}
}

// TestClaimEveryClaimCarriesAState: the fact-check output contract's invariant —
// every extracted claim carries exactly one state; structure (headings, fenced
// code, tables) extracts nothing; and a claim set with a missing state fails
// the check rather than passing vacuously.
func TestClaimEveryClaimCarriesAState(t *testing.T) {
	claims := ExtractClaims(sampleBody)
	if len(claims) == 0 {
		t.Fatalf("the sample body extracted no claims")
	}
	t.Logf("extracted %d claims from the sample body", len(claims))
	for _, c := range claims {
		t.Logf("claim: %q", c.Text)
		if !c.CarriesState() {
			t.Errorf("claim %q carries no state", c.Text)
		}
		// Extraction asserts nothing: EVERY extracted claim — bullet or sentence
		// — starts unverified, whatever the body says. A site that confirms on
		// extraction is the exact rounding this contract forbids.
		if c.State != ClaimUnverified {
			t.Errorf("claim %q extracted at state %s — extraction must start every claim unverified", c.Text, c.State)
		}
	}
	if !EveryClaimCarriesAState(claims) {
		t.Errorf("EveryClaimCarriesAState(extracted) = false, want true — extraction must satisfy its own contract")
	}

	// Structure is not a claim: nothing from the heading, the fence or the
	// table row may appear.
	for _, c := range claims {
		low := strings.ToLower(c.Text)
		if strings.Contains(low, "summary") && c.Text == "## Summary" {
			t.Errorf("a heading was extracted as a claim: %q", c.Text)
		}
		if strings.Contains(c.Text, "make test") {
			t.Errorf("fenced code was extracted as a claim: %q", c.Text)
		}
		if strings.HasPrefix(c.Text, "|") {
			t.Errorf("a table row was extracted as a claim: %q", c.Text)
		}
	}

	// The invariant fails — never passes vacuously — when a state is missing or
	// invented. This is the guard the fact-check lane's output is held to.
	if EveryClaimCarriesAState([]BodyClaim{{Text: "no state"}}) {
		t.Error("EveryClaimCarriesAState accepted a claim with no state")
	}
	if EveryClaimCarriesAState([]BodyClaim{{Text: "invented", State: ClaimState("maybe")}}) {
		t.Error("EveryClaimCarriesAState accepted an invented state")
	}
}

// ---------------------------------------------------------------------------
// The dispatch reference and the selection path
// ---------------------------------------------------------------------------

// referencePath is the dispatch reference this package's lane table is held to.
// The path is relative to this package's directory and MUST stay inside this
// module (tools/desk) — it may never climb above the module root and descend
// again. The mutation harness (cmd/muhar) runs the suite against an isolated
// COPY of the module root whenever more than one mutation is in flight (`-j 0`
// on a multi-core runner, which is what the truth-suite mutation gate uses), and
// in that copy nothing exists above the module root. A path that round-tripped
// up to the repository root and back down (`../../../../tools/desk/...`)
// resolved in a full checkout but not in the copy, so the harness read a red
// baseline it could not distinguish from a pre-existing failure and discarded
// the whole run — the gate for this package went red on every push to main.
var referencePath = filepath.Join("..", "..", "cmd", "deskdispatch", "references", "review-lanes.md")

// TestReviewLanesReferencePathStaysInsideTheModule holds the invariant the
// comment above states, so the path cannot silently drift back out of the
// module and redden the mutation gate again. It asserts two things: the
// reference resolves UNDER the module root (the nearest go.mod above this
// package — the exact tree cmd/muhar copies per worker), and a file is
// actually there. A reference reachable only from a full checkout is a
// could-not-check for the mutation harness, not a pass.
func TestReviewLanesReferencePathStaysInsideTheModule(t *testing.T) {
	abs, err := filepath.Abs(referencePath)
	if err != nil {
		t.Fatalf("cannot resolve the dispatch reference path %s: %v", referencePath, err)
	}
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("cannot resolve this package's directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod found above this package's directory — cannot locate the module root")
		}
		dir = parent
	}
	rel, err := filepath.Rel(dir, abs)
	if err != nil {
		t.Fatalf("cannot relate the dispatch reference to the module root %s: %v", dir, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Errorf("the dispatch reference path %s escapes the module root (resolves to %s, outside %s) — "+
			"the mutation harness runs the suite against a copy of the module root alone, where nothing above it exists",
			referencePath, abs, dir)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Errorf("no dispatch reference at %s (resolved to %s): %v", referencePath, abs, err)
	}
}

// TestReviewLanesReferenceMatchesTable: the dispatch reference's per-tier lane
// sets are PARSED out of the machine-checkable block in the reference document
// and compared to LanesFor, so a reference that describes a lane set the
// dispatcher does not select fails here. The reference is the document the
// review desk and the dispatched fact-check and fail-first reviewers read; if
// it drifts from the table, the desk dispatches one thing and the reviewers
// are told another.
func TestReviewLanesReferenceMatchesTable(t *testing.T) {
	data, err := os.ReadFile(referencePath)
	if err != nil {
		t.Fatalf("cannot read the review-lanes dispatch reference at %s: %v", referencePath, err)
	}
	lines := strings.Split(string(data), "\n")

	// The machine-checkable block sits between explicit markers so prose around
	// it can never be parsed as a lane set.
	const begin = "<!-- reviewlanes:begin -->"
	const end = "<!-- reviewlanes:end -->"
	inBlock := false
	rows := map[string][]string{}
	var order []string
	for _, line := range lines {
		switch {
		case strings.Contains(line, begin):
			inBlock = true
			continue
		case strings.Contains(line, end):
			inBlock = false
		}
		if !inBlock || !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if len(cells) != 2 {
			continue
		}
		first := strings.TrimSpace(cells[0])
		// Skip the markdown header row and the separator row — structure, not data.
		if first == "tier" || strings.Trim(first, "-: ") == "" {
			continue
		}
		tier := strings.TrimSpace(cells[0])
		var lanes []string
		for _, l := range strings.Split(cells[1], ",") {
			if l = strings.TrimSpace(l); l != "" {
				lanes = append(lanes, l)
			}
		}
		rows[tier] = lanes
		order = append(order, tier)
	}

	for _, tier := range []Tier{TierUnknown, TierBlessedOnce, TierContributor, TierMaintainer} {
		doc, ok := rows[tier.String()]
		if !ok {
			t.Errorf("the dispatch reference's lane block has no row for tier %q", tier.String())
			continue
		}
		code := laneNames(LanesFor(tier))
		if !namesEqual(doc, code) {
			t.Errorf("the reference's lane set for %s is %v, but LanesFor selects %v — the document and the dispatcher disagree", tier.String(), doc, code)
		}
	}
	// No extra tiers the table does not know.
	for _, name := range order {
		switch name {
		case "unknown", "blessed-once", "contributor", "maintainer":
		default:
			t.Errorf("the dispatch reference's lane block carries a tier %q the lane table does not know", name)
		}
	}
	if len(order) != 4 {
		t.Errorf("the reference's lane block carries %d tier rows, want exactly the four published tiers", len(order))
	}
}

// TestReviewLanesDispatchEndToEnd: tier in, lane set out, through the
// dispatcher's own selection path (ReviewLanesForAuthor — resolution and
// selection in one call) rather than through LanesFor alone. The seams are the
// same ones trusttier_test.go drives: an in-memory ledger, a private roster,
// and the fail-closed default.
func TestReviewLanesDispatchEndToEnd(t *testing.T) {
	const repo = "medici-finance/example"

	// No roster, no ledger: every external identity is unknown — today's bar —
	// and gets the deep set. This is the inert-on-landing property: until a
	// ledger exists the change adds scrutiny to external pull requests and
	// touches nothing else.
	withNoRoster(t)
	fakeLedgerAbsent(t)
	lanes, tier, prov := ReviewLanesForAuthor(repo, "stranger", 7001)
	t.Logf("absent ledger: tier=%s provenance=%s lanes=%v", tier, prov, laneNames(lanes))
	if tier != TierUnknown || prov != ProvenanceDefaultAbsent {
		t.Errorf("absent ledger resolved tier=%s provenance=%s, want unknown/default-absent", tier, prov)
	}
	if !namesEqual(laneNames(lanes), laneNames(LanesFor(TierUnknown))) {
		t.Errorf("absent ledger selected %v, want the deep set", laneNames(lanes))
	}

	// A standing contributor row (strict pinned-id path): the standard path.
	fakeLedger(t, []LedgerRow{
		{Repo: repo, Login: "regular", ID: 7002, Tier: "contributor",
			Date: "2026-09-18", Recorder: "human:example", Reason: "second landed change judged sound"},
	})
	lanes, tier, prov = ReviewLanesForAuthor(repo, "regular", 7002)
	t.Logf("contributor row: tier=%s provenance=%s lanes=%v", tier, prov, laneNames(lanes))
	if tier != TierContributor || prov != ProvenanceLedgerRow {
		t.Errorf("contributor row resolved tier=%s provenance=%s, want contributor/ledger-row", tier, prov)
	}
	if !namesEqual(laneNames(lanes), []string{LaneCorrectness.Name, LaneSecurity.Name}) {
		t.Errorf("contributor row selected %v, want the standard path", laneNames(lanes))
	}

	// A blessed-once row: one admitted item from an otherwise-unknown identity
	// — still the deep set.
	fakeLedger(t, []LedgerRow{
		{Repo: repo, Login: "first-timer", ID: 7003, Tier: "blessed-once",
			Date: "2026-09-18", Recorder: "human:example", Reason: "one item admitted for review"},
	})
	lanes, tier, prov = ReviewLanesForAuthor(repo, "first-timer", 7003)
	t.Logf("blessed-once row: tier=%s provenance=%s lanes=%v", tier, prov, laneNames(lanes))
	if tier != TierBlessedOnce || prov != ProvenanceLedgerRow {
		t.Errorf("blessed-once row resolved tier=%s provenance=%s, want blessed-once/ledger-row", tier, prov)
	}
	if !namesEqual(laneNames(lanes), laneNames(LanesFor(TierUnknown))) {
		t.Errorf("blessed-once row selected %v, want the deep set", laneNames(lanes))
	}

	// An unreadable ledger is an anomaly that STILL resolves unknown — a broken
	// ledger can only narrow what automation does, and here narrowing means the
	// deep set: it can never widen review to the standard path.
	fakeLedgerBroken(t, "ledger truncated mid-write")
	lanes, tier, prov = ReviewLanesForAuthor(repo, "anybody", 7004)
	t.Logf("broken ledger: tier=%s provenance=%s lanes=%v", tier, prov, laneNames(lanes))
	if tier != TierUnknown || prov != ProvenanceDefaultUnreadable {
		t.Errorf("broken ledger resolved tier=%s provenance=%s, want unknown/default-unreadable — the anomaly must be visible, not collapsed", tier, prov)
	}
	if !namesEqual(laneNames(lanes), laneNames(LanesFor(TierUnknown))) {
		t.Errorf("broken ledger selected %v, want the deep set — a ledger failure must never widen review", laneNames(lanes))
	}

	// A roster identity is maintainer and keeps the standard path; the ledger is
	// not even consulted.
	withRoster(t, map[string]string{
		EnvBlessLogin:    "housemate:7005",
		EnvTrustedLogins: "housemate:7005",
	})
	lanes, tier, prov = ReviewLanesForAuthor(repo, "housemate", 7005)
	t.Logf("roster identity: tier=%s provenance=%s lanes=%v", tier, prov, laneNames(lanes))
	if tier != TierMaintainer || prov != ProvenanceRoster {
		t.Errorf("roster identity resolved tier=%s provenance=%s, want maintainer/roster", tier, prov)
	}
	if !namesEqual(laneNames(lanes), []string{LaneCorrectness.Name, LaneSecurity.Name}) {
		t.Errorf("roster identity selected %v, want the standard path", laneNames(lanes))
	}
}
