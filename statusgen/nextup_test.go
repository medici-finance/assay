package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func day(d int) time.Time { return time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, d) }

func TestNextUpScoringAndCaps(t *testing.T) {
	hot := mkStream("hot", "active", "P0",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "todo"},
		Brief{Num: "03", Wave: 0, Status: "todo"},
		Brief{Num: "04", Wave: 0, Status: "todo"},
		Brief{Num: "05", Wave: 0, Status: "todo"}, // > perStreamCap so the cap actually caps
	)
	hot.LastTouch = day(10) // the max → staleness 0
	stale := mkStream("stale", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
	stale.LastTouch = day(0) // 10 days stale → 2000 + 100
	paused := mkStream("paused", "paused", "P0", Brief{Num: "01", Wave: 0, Status: "todo"})
	paused.LastTouch = day(0)

	picks := nextUp([]*Stream{hot, stale, paused}, nil, nil).Picks
	if len(picks) != perStreamCap+1 {
		t.Fatalf("got %d picks, want %d (%d hot capped + 1 stale, paused excluded)", len(picks), perStreamCap+1, perStreamCap)
	}
	if picks[0].Stream.Name != "hot" || picks[0].Score != 3000 {
		t.Errorf("pick 0: %+v", picks[0])
	}
	hotCount := 0
	for _, p := range picks {
		if p.Stream.Name == "hot" {
			hotCount++
		}
		if p.Stream.Name == "paused" {
			t.Error("paused stream must not be picked")
		}
	}
	if hotCount != perStreamCap {
		t.Errorf("per-stream cap violated: %d hot picks, want %d", hotCount, perStreamCap)
	}
	for _, p := range picks {
		if p.Stream.Name == "stale" && p.Score != 2100 {
			t.Errorf("stale score = %d, want 2100", p.Score)
		}
	}
}

func TestNextUpWaveGatingAndStale(t *testing.T) {
	s := mkStream("s", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo", StaleRef: "F-01"}, // stale → excluded
		Brief{Num: "02", Wave: 1, Status: "todo"},                   // wave 1 gated by 01 not done
		Brief{Num: "03", Wave: 0, Status: "in-progress"},            // in-progress always eligible
	)
	s.LastTouch = day(0)
	picks := nextUp([]*Stream{s}, nil, nil).Picks
	if len(picks) != 1 || picks[0].Brief.Num != "03" {
		t.Fatalf("want only brief 03, got %+v", picks)
	}
}

// TestNextUpStreamFindingNotDispatchGate is the regression at
// the dispatch level: applyFindings + nextUp end to end. A stream-level finding
// (bare `affects: <stream>`) used to stamp StaleRef on every brief, and
// eligible() hard-excludes any StaleRef — so the whole issue-loop stream (its
// many todo placeholders) fell out of Next-up. The stream must stay dispatchable,
// while a brief-specific entry still removes exactly its own brief.
func TestNextUpStreamFindingNotDispatchGate(t *testing.T) {
	s := mkStream("issue-loop", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo", Schema: "placeholder-v1"},
		Brief{Num: "02", Wave: 0, Status: "todo", Schema: "placeholder-v1"},
		Brief{Num: "03", Wave: 0, Status: "todo", Schema: "placeholder-v1"},
	)
	s.LastTouch = day(0)
	findings := []Finding{
		{ID: "F-guardrails-dup", Affects: []string{"issue-loop"}},     // stream-level: gates nothing
		{ID: "F-bug-close", Affects: []string{"issue-loop/brief-03"}}, // brief-specific: gates 03
	}
	applyFindings([]*Stream{s}, findings)

	nu := nextUp([]*Stream{s}, nil, nil)
	if nu.Eligible != 2 {
		t.Fatalf("want 2 eligible (01, 02 — only the brief-specific finding gates), got %d: %+v", nu.Eligible, nu.Picks)
	}
	for _, p := range nu.Picks {
		if p.Brief.Num == "03" {
			t.Errorf("brief-specific finding must still exclude brief 03: %+v", p)
		}
	}
}

func TestEligibilityDepPrecise(t *testing.T) {
	// Fixture modelled on the ledger-hardening/01 case: a wave-1 brief-v1
	// brief whose typed dep is verified, with an unrelated wave-0 todo in
	// the same stream. Under the old whole-wave rule, 02 is ineligible
	// because 01 (lower wave) is not done; under the new dep-precise rule,
	// 02 is eligible because dep/01 is verified.

	// --- subtest: dep verified → eligible ---
	depStream := mkStream("dep", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "verified"},
	)
	depStream.LastTouch = day(0)

	target := mkStream("target", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo"}, // unrelated — does not block 02
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"dep/01"}},
	)
	target.LastTouch = day(0)

	picks := nextUp([]*Stream{target, depStream}, nil, nil).Picks
	found02 := false
	for _, p := range picks {
		if p.Stream.Name == "target" && p.Brief.Num == "02" {
			found02 = true
		}
	}
	if !found02 {
		t.Fatalf("brief-v1 02 with verified dep should be eligible, got %+v", picks)
	}

	// --- subtest: dep merely implemented → ineligible ---
	depStream2 := mkStream("dep", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented"},
	)
	depStream2.LastTouch = day(0)

	target2 := mkStream("target", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"dep/01"}},
	)
	target2.LastTouch = day(0)

	picks2 := nextUp([]*Stream{target2, depStream2}, nil, nil).Picks
	for _, p := range picks2 {
		if p.Stream.Name == "target" && p.Brief.Num == "02" {
			t.Fatalf("brief-v1 02 with implemented dep should be INELIGIBLE, got %+v", picks2)
		}
	}

	// --- subtest: empty depends → eligible now ---
	emptyDep := mkStream("empty", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo"}, // unrelated todo
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{}},
	)
	emptyDep.LastTouch = day(0)

	picks3 := nextUp([]*Stream{emptyDep}, nil, nil).Picks
	found02empty := false
	for _, p := range picks3 {
		if p.Stream.Name == "empty" && p.Brief.Num == "02" {
			found02empty = true
		}
	}
	if !found02empty {
		t.Fatalf("brief-v1 02 with empty depends should be eligible, got %+v", picks3)
	}

	// --- subtest: legacy stream — wave rule unchanged ---
	legacy := mkStream("legacy", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 1, Status: "todo"}, // no Schema → legacy
	)
	legacy.LastTouch = day(0)

	picks4 := nextUp([]*Stream{legacy}, nil, nil).Picks
	for _, p := range picks4 {
		if p.Stream.Name == "legacy" && p.Brief.Num == "02" {
			t.Fatalf("legacy 02 should be wave-gated by 01, got %+v", picks4)
		}
	}
	// 01 should be eligible (wave 0, todo, no lower-wave blockers)
	found01 := false
	for _, p := range picks4 {
		if p.Stream.Name == "legacy" && p.Brief.Num == "01" {
			found01 = true
		}
	}
	if !found01 {
		t.Fatalf("legacy 01 (wave 0 todo) should be eligible, got %+v", picks4)
	}
}

func TestNextUpClaimAware(t *testing.T) {
	s := mkStream("hot", "active", "P0",
		Brief{Num: "06", Wave: 0, Status: "todo"},
		Brief{Num: "07", Wave: 0, Status: "todo"},
	)
	s.LastTouch = day(0)

	claimed := map[string]bool{"hot/06": true}
	picks := nextUp([]*Stream{s}, claimed, nil).Picks
	if len(picks) != 1 || picks[0].Brief.Num != "07" {
		t.Fatalf("want only brief 07 (06 claimed), got %+v", picks)
	}

	// Claim lifts (PR merged/closed) → the brief reappears.
	picks = nextUp([]*Stream{s}, map[string]bool{}, nil).Picks
	if len(picks) != 2 {
		t.Fatalf("want both briefs once the claim lifts, got %+v", picks)
	}
}

// withSpan sets the span-of-control cap + overflow threshold for the duration of
// a test and restores them afterward.
func withSpan(t *testing.T, cap, threshold int) {
	t.Helper()
	oldSpan, oldThresh := spanOfControl, overflowThreshold
	spanOfControl, overflowThreshold = cap, threshold
	t.Cleanup(func() { spanOfControl, overflowThreshold = oldSpan, oldThresh })
}

// eligibleStreams builds `n` eligible (active, P2, wave-0 todo) briefs spread
// across streams of at most `perStream` briefs each, so the per-stream cap
// doesn't collapse the count. All share the same LastTouch → equal scores.
func eligibleStreams(n, perStream int) []*Stream {
	var streams []*Stream
	made := 0
	for i := 0; made < n; i++ {
		k := perStream
		if made+k > n {
			k = n - made
		}
		var briefs []Brief
		for j := 0; j < k; j++ {
			briefs = append(briefs, Brief{Num: fmt.Sprintf("%02d", j+1), Wave: 0, Status: "todo"})
		}
		s := mkStream(fmt.Sprintf("s%02d", i), "active", "P2", briefs...)
		s.LastTouch = day(0)
		streams = append(streams, s)
		made += k
	}
	return streams
}

// TestNextUpSpanCapOverflow — 23 eligible briefs, span cap 7: exactly 7 shown,
// overflow flagged, 16 held back, and the STATUS view renders the "7 of 23
// eligible" overflow line (Verify item 2).
func TestNextUpSpanCapOverflow(t *testing.T) {
	withSpan(t, 7, 7)
	streams := eligibleStreams(23, 2) // 12 streams, per-stream cap keeps 7 spread-able
	nu := nextUp(streams, nil, nil)
	if nu.Eligible != 23 {
		t.Fatalf("eligible = %d, want 23", nu.Eligible)
	}
	if len(nu.Picks) != 7 {
		t.Fatalf("shown = %d, want 7 (span cap)", len(nu.Picks))
	}
	if !nu.Overflow() {
		t.Fatal("want Overflow() = true when 23 eligible > cap 7")
	}
	if nu.HeldBack() != 16 {
		t.Fatalf("held back = %d, want 16", nu.HeldBack())
	}
	out := emit(streams, nil, nu, nil, IntakeAlarmResult{}, nil, "")
	if !strings.Contains(out, "7 of 23 eligible") {
		t.Errorf("STATUS Next-up missing explicit overflow line:\n%s", out)
	}
	if !strings.Contains(out, "held back") {
		t.Error("overflow line must state how many are held back")
	}
}

// TestNextUpNoOverflow — 5 eligible briefs, span cap 7: all 5 shown, no overflow
// indicator anywhere in the STATUS view (Verify item 3).
func TestNextUpNoOverflow(t *testing.T) {
	withSpan(t, 7, 7)
	streams := eligibleStreams(5, 1) // 5 streams × 1 brief → all 5 fit under the cap
	nu := nextUp(streams, nil, nil)
	if nu.Eligible != 5 {
		t.Fatalf("eligible = %d, want 5", nu.Eligible)
	}
	if len(nu.Picks) != 5 {
		t.Fatalf("shown = %d, want 5", len(nu.Picks))
	}
	if nu.Overflow() {
		t.Fatal("want Overflow() = false when 5 eligible <= cap 7")
	}
	out := emit(streams, nil, nu, nil, IntakeAlarmResult{}, nil, "")
	if strings.Contains(out, "held back") || strings.Contains(out, "eligible —") {
		t.Errorf("no overflow indicator expected with 5 ≤ cap 7:\n%s", out)
	}
}

// nextUpAllScores returns every eligible brief's raw score keyed by "stream/NN",
// bypassing the span/per-stream caps so a test can assert the score directly. It
// mirrors nextUp's per-brief formula (stream-touch clock; briefTouch not needed
// here).
func nextUpAllScores(streams []*Stream) map[string]int {
	var now time.Time
	if len(streams) > 0 {
		now = streams[0].LastTouch
	}
	for _, s := range streams {
		if s.LastTouch.After(now) {
			now = s.LastTouch
		}
	}
	rev, status := buildRevDeps(streams)
	out := map[string]int{}
	for _, s := range streams {
		for _, b := range s.Briefs {
			if !eligible(streams, s, b, nil) {
				continue
			}
			days := int(now.Sub(s.LastTouch).Hours() / 24)
			if days < 0 {
				days = 0
			}
			if days > stalenessCapDays {
				days = stalenessCapDays
			}
			out[s.Name+"/"+b.Num] = priorityWeight(s.Priority) + days*stalenessPerDay +
				valueWeight(b.Value) + unblocksWeight*blockedCount(rev, status, s.Name+"/"+b.Num)
		}
	}
	return out
}

// TestNextUpValueOrdering — at equal priority and staleness, the explicit value
// field re-orders briefs: high > med > low, and an absent value scores exactly
// as med (the neutral zero point).
func TestNextUpValueOrdering(t *testing.T) {
	s := mkStream("v", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo", Value: "low"},
		Brief{Num: "02", Wave: 0, Status: "todo", Value: "high"},
		Brief{Num: "03", Wave: 0, Status: "todo"}, // absent → med
	)
	s.LastTouch = day(0)

	withSpan(t, 7, 7)
	picks := nextUp([]*Stream{s}, nil, nil).Picks
	if len(picks) != 3 {
		t.Fatalf("all 3 fit under per-stream cap %d → 3 picks, got %d", perStreamCap, len(picks))
	}
	// value re-orders within the stream: high (02) tops; med/absent (03) second;
	// low (01) third. (All three fit — the per-stream cap is >= 3.)
	if picks[0].Brief.Num != "02" {
		t.Fatalf("high-value 02 should rank first, got %q", picks[0].Brief.Num)
	}
	if picks[1].Brief.Num != "03" {
		t.Fatalf("med (absent-value) 03 should rank second, got %q", picks[1].Brief.Num)
	}
	if picks[2].Brief.Num != "01" {
		t.Fatalf("low-value 01 should rank third, got %q", picks[2].Brief.Num)
	}

	scores := nextUpAllScores([]*Stream{s})
	if scores["v/03"] != 2000 {
		t.Errorf("absent value must score as med (2000), got %d", scores["v/03"])
	}
	if scores["v/02"] != 2000+valueWeightHigh {
		t.Errorf("high value score = %d, want %d", scores["v/02"], 2000+valueWeightHigh)
	}
	if scores["v/01"] != 2000+valueWeightLow {
		t.Errorf("low value score = %d, want %d", scores["v/01"], 2000+valueWeightLow)
	}
}

// TestNextUpBlockedCountPromotes — a brief that transitively holds up others
// (blockedCount ≥ 1) outranks a same-priority sibling that blocks nothing, even
// with identical priority, staleness and value. Chain: blocker/01 ← b/02 ← b/03.
func TestNextUpBlockedCountPromotes(t *testing.T) {
	blocker := mkStream("blocker", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo", Schema: "brief-v1", Depends: []string{}},
		Brief{Num: "09", Wave: 0, Status: "todo", Schema: "brief-v1", Depends: []string{}}, // blocks nothing
	)
	blocker.LastTouch = day(0)
	chained := mkStream("b", "active", "P1",
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"blocker/01"}},
		Brief{Num: "03", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"b/02"}},
	)
	chained.LastTouch = day(0)

	scores := nextUpAllScores([]*Stream{blocker, chained})
	if got := scores["blocker/01"]; got != 2000+2*unblocksWeight {
		t.Fatalf("blocker/01 score = %d, want %d (blockedCount 2)", got, 2000+2*unblocksWeight)
	}
	if got := scores["blocker/09"]; got != 2000 {
		t.Fatalf("blocker/09 blocks nothing, score = %d, want 2000", got)
	}
	if scores["blocker/01"] <= scores["blocker/09"] {
		t.Fatalf("a blocker must outrank a same-priority non-blocker: %d vs %d",
			scores["blocker/01"], scores["blocker/09"])
	}
}

// TestNextUpStalenessFromBriefHistory — the F-09 clock fix. A brief ages from its
// OWN last transition (briefTouch, the historian) rather than the stream's git
// LastTouch, so a recent touch of the stream (sibling-brief activity) does not
// reset an unrelated brief's aging.
func TestNextUpStalenessFromBriefHistory(t *testing.T) {
	s := mkStream("h", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo"}, // last transitioned day(0)
		Brief{Num: "02", Wave: 0, Status: "todo"}, // last transitioned day(10)
	)
	// Stream touched recently (day 10) — under the OLD clock both briefs read
	// staleness 0. Under the fix, 01 ages from its own day(0).
	s.LastTouch = day(10)
	briefTouch := map[string]time.Time{
		"h/01": day(0),  // 10 days stale
		"h/02": day(10), // fresh
	}

	withSpan(t, 7, 7)
	picks := nextUp([]*Stream{s}, nil, briefTouch).Picks
	scores := map[string]int{}
	for _, p := range picks {
		scores[p.Brief.Num] = p.Score
	}
	if scores["01"] != 2100 {
		t.Errorf("brief 01 should age from its own day(0) transition → 2100, got %d", scores["01"])
	}
	if scores["02"] != 2000 {
		t.Errorf("brief 02 transitioned at the touch day → 2000, got %d", scores["02"])
	}
	if picks[0].Brief.Num != "01" {
		t.Errorf("the brief stale by its own history should rank first, got %q", picks[0].Brief.Num)
	}
}

// TestNextUpStalenessFallsBackToStreamTouch — a brief with no historian row
// (absent from briefTouch) falls back to the stream's LastTouch, preserving the
// pre-14 behaviour.
func TestNextUpStalenessFallsBackToStreamTouch(t *testing.T) {
	s := mkStream("f", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
	s.LastTouch = day(0)
	fresh := mkStream("g", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
	fresh.LastTouch = day(10)

	// briefTouch knows nothing about f/01 → falls back to stream LastTouch day(0),
	// 10 days behind g's day(10): 2100 vs 2000.
	scores := map[string]int{}
	for _, p := range nextUp([]*Stream{s, fresh}, nil, map[string]time.Time{}).Picks {
		scores[p.Stream.Name] = p.Score
	}
	if scores["f"] != 2100 {
		t.Errorf("f/01 with no history row should fall back to stream touch → 2100, got %d", scores["f"])
	}
	if scores["g"] != 2000 {
		t.Errorf("g/01 fresh → 2000, got %d", scores["g"])
	}
}

// TestNextUpBlockedCountOutranksInRender — Verify item 3: in the rendered
// Next-up table, a brief with blockedCount ≥3 sorts ABOVE a same-priority
// sibling that blocks nothing. ablock/00 is depended on (transitively) by three
// not-done briefs; zfree/00 blocks none. Both P1, equal staleness.
func TestNextUpBlockedCountOutranksInRender(t *testing.T) {
	blocker := mkStream("ablock", "active", "P1",
		Brief{Num: "00", Wave: 0, Status: "todo", Schema: "brief-v1", Depends: []string{}},
	)
	blocker.LastTouch = day(0)
	// Three briefs gated on ablock/00 (still todo → not done): blockedCount = 3.
	deps := mkStream("deps", "active", "P1",
		Brief{Num: "01", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"ablock/00"}},
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"ablock/00"}},
		Brief{Num: "03", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"ablock/00"}},
	)
	deps.LastTouch = day(0)
	free := mkStream("zfree", "active", "P1",
		Brief{Num: "00", Wave: 0, Status: "todo", Schema: "brief-v1", Depends: []string{}},
	)
	free.LastTouch = day(0)

	withSpan(t, 7, 7)
	streams := []*Stream{blocker, deps, free}
	nu := nextUp(streams, nil, nil)
	if got := blockedCountFor(streams, "ablock/00"); got != 3 {
		t.Fatalf("ablock/00 blockedCount = %d, want 3", got)
	}
	// The dep briefs are ineligible (their dep is still todo) so only the two
	// blockedCount-comparison briefs show.
	var order []string
	for _, p := range nu.Picks {
		order = append(order, p.Stream.Name+"/"+p.Brief.Num)
	}
	if len(order) < 2 || order[0] != "ablock/00" {
		t.Fatalf("blocker (blockedCount 3) must rank first; order = %v", order)
	}
	out := emit(streams, nil, nu, nil, IntakeAlarmResult{}, nil, "")
	iBlock := strings.Index(out, "ablock | 00")
	iFree := strings.Index(out, "zfree | 00")
	if iBlock < 0 || iFree < 0 || iBlock > iFree {
		t.Fatalf("rendered Next-up must place the blocker above the non-blocker:\n%s", out)
	}
	t.Logf("Verify-3 observed Next-up ordering: %v", order)
}

// blockedCountFor is a test helper wrapping buildRevDeps + blockedCount.
func blockedCountFor(streams []*Stream, target string) int {
	rev, status := buildRevDeps(streams)
	return blockedCount(rev, status, target)
}

// --- Gate-score tests ---

// TestGateScoresChain verifies blockedCount walks a transitive dependency chain:
// blocker/01 ← chained/02 ← chained/03 (all not-done). blocker/01 has blockedCount 2.
func TestGateScoresChain(t *testing.T) {
	blocker := mkStream("blocker", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented", Schema: "brief-v1", Depends: []string{}},
	)
	blocker.LastTouch = day(0)
	chained := mkStream("chained", "active", "P1",
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"blocker/01"}},
		Brief{Num: "03", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"chained/02"}},
	)
	chained.LastTouch = day(0)

	gates := gateScores([]*Stream{blocker, chained}, nil)
	if len(gates) != 1 {
		t.Fatalf("expected 1 awaiting brief, got %d", len(gates))
	}
	if gates[0].BlockedCount != 2 {
		t.Errorf("blocker/01 blockedCount = %d, want 2", gates[0].BlockedCount)
	}
	if gates[0].Score != 2000+2*unblocksWeight {
		t.Errorf("blocker/01 score = %d, want %d", gates[0].Score, 2000+2*unblocksWeight)
	}
}

// TestGateScoresDiamond verifies blockedCount handles a diamond dependency:
// A ← B, A ← C (B and C both depend on A). A has blockedCount 2.
func TestGateScoresDiamond(t *testing.T) {
	root := mkStream("root", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented", Schema: "brief-v1", Depends: []string{}},
	)
	root.LastTouch = day(0)
	leaves := mkStream("leaves", "active", "P1",
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"root/01"}},
		Brief{Num: "03", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"root/01"}},
	)
	leaves.LastTouch = day(0)

	gates := gateScores([]*Stream{root, leaves}, nil)
	if len(gates) != 1 {
		t.Fatalf("expected 1 awaiting brief, got %d", len(gates))
	}
	if gates[0].BlockedCount != 2 {
		t.Errorf("root/01 blockedCount = %d, want 2 (diamond: two deps)", gates[0].BlockedCount)
	}
}

// TestGateScoresCrossStream verifies blockedCount walks across stream boundaries:
// stream-a/01 ← stream-b/02 (cross-stream dep).
func TestGateScoresCrossStream(t *testing.T) {
	a := mkStream("stream-a", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented", Schema: "brief-v1", Depends: []string{}},
	)
	a.LastTouch = day(0)
	b := mkStream("stream-b", "active", "P1",
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"stream-a/01"}},
	)
	b.LastTouch = day(0)

	gates := gateScores([]*Stream{a, b}, nil)
	if len(gates) != 1 {
		t.Fatalf("expected 1 awaiting brief, got %d", len(gates))
	}
	if gates[0].BlockedCount != 1 {
		t.Errorf("stream-a/01 blockedCount = %d, want 1 (cross-stream dep)", gates[0].BlockedCount)
	}
}

// TestGateScoresCycleSafe verifies blockedCount is safe against dependency cycles.
// A depends on B, B depends on A — blockedCount must not loop forever.
func TestGateScoresCycleSafe(t *testing.T) {
	a := mkStream("cycle", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented", Schema: "brief-v1", Depends: []string{"cycle/02"}},
		Brief{Num: "02", Wave: 0, Status: "todo", Schema: "brief-v1", Depends: []string{"cycle/01"}},
	)
	a.LastTouch = day(0)

	gates := gateScores([]*Stream{a}, nil)
	if len(gates) != 1 {
		t.Fatalf("expected 1 awaiting brief (cycle/01), got %d", len(gates))
	}
	// In a cycle, each node's blockedCount should terminate and count the other node
	// at most once. 01 → 02 (but 02 already seen) = 1.
	if gates[0].BlockedCount != 1 {
		t.Errorf("cycle/01 blockedCount = %d, want 1 (cycle: 02 counted once)", gates[0].BlockedCount)
	}
}

// TestGateScoresUnknownDep verifies an unknown/unresolvable dep contributes 0
// blockedCount and never panics.
func TestGateScoresUnknownDep(t *testing.T) {
	s := mkStream("s", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented", Schema: "brief-v1", Depends: []string{"nonexistent/99"}},
	)
	s.LastTouch = day(0)

	gates := gateScores([]*Stream{s}, nil)
	if len(gates) != 1 {
		t.Fatalf("expected 1 awaiting brief, got %d", len(gates))
	}
	if gates[0].BlockedCount != 0 {
		t.Errorf("unknown dep blockedCount = %d, want 0", gates[0].BlockedCount)
	}
}

// TestGateScoresOrdering verifies gate scores are sorted descending by score,
// with stream name then brief num as tie-breakers.
func TestGateScoresOrdering(t *testing.T) {
	// P0 stream — any brief here outranks P1.
	p0 := mkStream("p0s", "active", "P0",
		Brief{Num: "10", Wave: 0, Status: "implemented", Value: "high"},
		Brief{Num: "01", Wave: 0, Status: "implemented"},
	)
	p0.LastTouch = day(0)
	// P1 stream with a blocker.
	blocker := mkStream("blocker", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented", Schema: "brief-v1", Depends: []string{}},
	)
	blocker.LastTouch = day(0)
	deps := mkStream("deps", "active", "P1",
		Brief{Num: "01", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"blocker/01"}},
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"blocker/01"}},
		Brief{Num: "03", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"blocker/01"}},
	)
	deps.LastTouch = day(0)

	gates := gateScores([]*Stream{p0, blocker, deps}, nil)
	if len(gates) != 3 {
		t.Fatalf("expected 3 awaiting briefs, got %d", len(gates))
	}
	// p0s/10 (P0 + high value = 3200) > p0s/01 (P0 = 3000) > blocker/01 (P1 + 3*500 = 3500)
	// Wait: 3500 > 3200, so blocker should rank second.
	// p0s/10: 3000 + 0 + 200 + 0 = 3200
	// p0s/01: 3000 + 0 + 0 + 0 = 3000
	// blocker/01: 2000 + 0 + 0 + 3*500 = 3500
	// So order: blocker/01 (3500), p0s/10 (3200), p0s/01 (3000)
	if gates[0].Stream.Name != "blocker" || gates[0].Brief.Num != "01" {
		t.Errorf("first should be blocker/01 (score 3500), got %s/%s (score %d)",
			gates[0].Stream.Name, gates[0].Brief.Num, gates[0].Score)
	}
	if gates[1].Stream.Name != "p0s" || gates[1].Brief.Num != "10" {
		t.Errorf("second should be p0s/10 (score 3200), got %s/%s (score %d)",
			gates[1].Stream.Name, gates[1].Brief.Num, gates[1].Score)
	}
	if gates[2].Stream.Name != "p0s" || gates[2].Brief.Num != "01" {
		t.Errorf("third should be p0s/01 (score 3000), got %s/%s (score %d)",
			gates[2].Stream.Name, gates[2].Brief.Num, gates[2].Score)
	}
}

// TestGateScoresIgnoresNonAwaiting verifies only implemented/verified briefs are scored.
func TestGateScoresIgnoresNonAwaiting(t *testing.T) {
	s := mkStream("s", "active", "P0",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "in-progress"},
		Brief{Num: "03", Wave: 0, Status: "implemented"},
		Brief{Num: "04", Wave: 0, Status: "verified"},
		Brief{Num: "05", Wave: 0, Status: "done"},
	)
	s.LastTouch = day(0)

	gates := gateScores([]*Stream{s}, nil)
	if len(gates) != 2 {
		t.Fatalf("expected 2 awaiting briefs (implemented + verified), got %d", len(gates))
	}
	nums := map[string]bool{}
	for _, g := range gates {
		nums[g.Brief.Num] = true
	}
	if !nums["03"] || !nums["04"] {
		t.Errorf("want 03 and 04, got %v", nums)
	}
}

// TestNextUpMaxConcurrent tests the per-stream max-concurrent cap. A stream
// with max-concurrent: 1 and two eligible todo
// briefs should offer at most one pick; when one is already claimed (in-flight),
// the offer budget is zero.
func TestNextUpMaxConcurrent(t *testing.T) {
	withSpan(t, 7, 7)

	// --- subtest: max-concurrent: 1 → one pick from two eligible briefs ---
	one := 1
	s := mkStream("serial", "active", "P0",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "todo"},
	)
	s.LastTouch = day(0)
	s.MaxConcurrent = &one

	picks := nextUp([]*Stream{s}, nil, nil).Picks
	if len(picks) != 1 {
		t.Fatalf("max-concurrent: 1 with 2 eligible: want 1 pick, got %d", len(picks))
	}

	// --- subtest: one claimed → zero picks (budget exhausted) ---
	claimed := map[string]bool{"serial/01": true}
	picks = nextUp([]*Stream{s}, claimed, nil).Picks
	if len(picks) != 0 {
		t.Fatalf("max-concurrent: 1 with 1 claimed: want 0 picks, got %d", len(picks))
	}

	// --- subtest: claimed map nil (fail-open) → one pick ---
	picks = nextUp([]*Stream{s}, nil, nil).Picks
	if len(picks) != 1 {
		t.Fatalf("max-concurrent: 1 with nil claimed (fail-open): want 1 pick, got %d", len(picks))
	}

	// --- subtest: streams without the knob unchanged ---
	hot := mkStream("hot", "active", "P0",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "todo"},
		Brief{Num: "03", Wave: 0, Status: "todo"},
	)
	hot.LastTouch = day(0)

	picks = nextUp([]*Stream{hot}, nil, nil).Picks
	// Without max-concurrent, perStreamCap (4) controls; all 3 briefs should appear.
	if len(picks) != 3 {
		t.Fatalf("stream without max-concurrent: want 3 picks (under perStreamCap=4), got %d", len(picks))
	}

	// --- subtest: mixed — serial stream capped at 1, unconstrained stream at perStreamCap ---
	mixedSerial := mkStream("serial", "active", "P0",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "todo"},
	)
	mixedSerial.LastTouch = day(0)
	mixedSerial.MaxConcurrent = &one

	unconstrained := mkStream("free", "active", "P0",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "todo"},
		Brief{Num: "03", Wave: 0, Status: "todo"},
		Brief{Num: "04", Wave: 0, Status: "todo"},
		Brief{Num: "05", Wave: 0, Status: "todo"},
	)
	unconstrained.LastTouch = day(0)

	nu := nextUp([]*Stream{mixedSerial, unconstrained}, nil, nil)
	serialCount := 0
	for _, p := range nu.Picks {
		if p.Stream.Name == "serial" {
			serialCount++
		}
	}
	if serialCount != 1 {
		t.Fatalf("mixed: serial stream should have exactly 1 pick, got %d (picks=%v)", serialCount, nu.Picks)
	}
	// The unconstrained stream can fill the rest of the 7-slot span.
	freeCount := 0
	for _, p := range nu.Picks {
		if p.Stream.Name == "free" {
			freeCount++
		}
	}
	if freeCount < 3 {
		t.Fatalf("mixed: unconstrained stream should have at least 3 picks, got %d (picks=%v)", freeCount, nu.Picks)
	}
	if len(nu.Picks) < 4 {
		t.Fatalf("mixed: total picks should be at least 4 (1 serial + 3+ free), got %d", len(nu.Picks))
	}
}
