package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	weightP0         = 3000
	weightP1         = 2000
	weightP2         = 1000
	stalenessCapDays = 30
	stalenessPerDay  = 10
	// perStreamCap: max in-flight briefs Next-up offers from any one stream, so a
	// single stream can't monopolize the board. Raised 2 -> 4 (human:<name>, 2026-07-16):
	// with agents (not humans) working the queue, and critical paths + dependency
	// waves already constraining what's eligible, a wider per-stream draw is safe
	// and keeps the agent-scale span-of-control cap (20) actually fillable rather
	// than the binding constraint being an old human-scale 2.
	perStreamCap = 4

	// Next-up value + held-up-by terms (human:<name>'s R-01
	// decision). These are TUNABLE HEURISTIC WEIGHTS, not truths — the whole
	// score is an evolving heuristic (F-09 discipline: staleness still only
	// rewards age, value is a coarse three-way knob, blockedCount has no effort
	// term). When I-13 lands a better score these move; the gate-score
	// follows the same weights.
	//
	// value: med is the neutral zero point so an absent/med field leaves the
	// score identical to the pre-14 priority+staleness score; high nudges up,
	// low nudges down. Kept below the priority-tier gap (1000) so value re-orders
	// WITHIN a priority band without silently outranking a whole tier.
	valueWeightLow  = -200
	valueWeightMed  = 0
	valueWeightHigh = 200

	// unblocksWeight scores each not-done brief this one transitively holds up
	// (blockedCount). Set so a brief blocking ~3 others can float above a
	// higher-priority brief blocking none — 500 × 3 = 1500
	// > one priority tier (1000) — giving the score a notion of what a brief
	// unblocks, not just how long it has waited.
	unblocksWeight = 500

	// defaultSpanOfControl caps how many items Next-up shows at once. EEMUA-191's
	// "span of control" (7 ± 2) is a *human* operator's cognitive limit — how many
	// active alarms a person can attend to before quality drops. But this queue is
	// worked by AGENTS, not humans (human:<name>, 2026-07-16): the human cognitive ceiling
	// doesn't apply, so the cap is set far more liberally. 20 keeps overflow a real
	// signal (a genuinely huge eligible backlog still alarms) without throttling the
	// fanout to a human-scale band. This overall cap sits on top of the per-stream
	// cap. Configurable via the --span flag.
	defaultSpanOfControl = 20
)

// spanOfControl and overflowThreshold are package vars (not consts) so the CLI
// flags (--span, --overflow-threshold) and tests can override them:
//
//	spanOfControl     — the cap: max items Next-up shows.
//	overflowThreshold — the eligible-brief count above which Next-up renders the
//	                    overflow indicator and emits the --lint NOTICE. Defaults
//	                    to spanOfControl ("overflow" == more eligible than shown).
//
// Both default to defaultSpanOfControl; main() wires the flags before run().
var (
	spanOfControl     = defaultSpanOfControl
	overflowThreshold = defaultSpanOfControl
)

// activeDriveSet is the operator-declared drive campaign(s) in effect for this
// run (methodology-metrics/45). main() loads it from docs/roadmap/drives/ before
// nextUp and wires it here; tests set it via withDrives. It follows the
// spanOfControl package-var precedent so every existing nextUp caller stays
// three-argument. The ZERO value (no active drive) is INERT: driveBoost returns 0
// for every brief, no banner renders, and a board built with no manifest is
// byte-identical to the pre-drives board (TestDriveAbsentIsInert).
var activeDriveSet DriveSet

// activeFindings carries the unresolved-finding set for the current run, feeding the
// critical tier's reviewer-finding arm (drivecritical.go). It follows the
// activeDriveSet package-var precedent so every existing nextUp caller stays
// three-argument. main()/runRoadmap wire it (from the same findings applyFindings
// consumes) before nextUp; tests set it via withFindings. The ZERO value (nil) is
// INERT: no brief qualifies via the reviewer-finding arm, and — because the critical
// tier is only applied when a drive is active — it has no effect on a no-drive board.
var activeFindings []Finding

type Pick struct {
	Stream *Stream
	Brief  Brief
	// Score is the BASE score (priority + staleness + value + unblocks) — the
	// drive steer is kept separate in DriveTerm so the display can decompose it
	// (base + named term) and never present a merged number.
	Score int
	// DriveTerm is the additive drive steer for this pick (methodology-metrics/45),
	// 0 when no active drive covers it. DriveSlug names the drive that supplied the
	// term (the max over overlapping drives). Total() is Score + DriveTerm.
	DriveTerm int
	DriveSlug string
	// CriticalTier marks a pick in the HARD, never-buried critical tier (phase 3).
	// The board sorts lexicographically by (CriticalTier, Total()): a critical member
	// ranks ABOVE every score, so no intensity — surge included — can bury a live
	// fire. Membership is machine-derived / stamped, never self-declared
	// (drivecritical.go), and is set ONLY when a drive is active, so a no-drive board
	// is byte-identical to the pre-drives baseline. CriticalArm names the qualifying
	// arm (main-red[deferred] / security / high-unblocks / reviewer-finding), "" when
	// not critical.
	CriticalTier bool
	CriticalArm  string
}

// Total is the ranked score: the base score plus any drive steer. With no active
// drive DriveTerm is 0 for every pick, so Total == Score and the ordering — and
// the rendered board — is byte-identical to the pre-drives generator.
func (p Pick) Total() int { return p.Score + p.DriveTerm }

// NextUp is the computed Next-up view: the capped list of picks plus the counts
// needed to render the span-of-control overflow indicator.
type NextUp struct {
	Picks     []Pick
	Eligible  int // total eligible briefs before span/per-stream capping
	Span      int // span-of-control cap applied to Picks
	Threshold int // eligible count above which overflow is signalled
	// Claims records whether the claim filter behind this view actually ran.
	// The zero value is {Known:false} on purpose: a view nobody told about
	// claims is unfiltered, and emit renders it as degraded rather than
	// letting it pass for a filtered board.
	Claims ClaimSource
	// HeldByStreamCap counts eligible briefs a PER-STREAM cap held back
	// (perStreamCap, or a stream's declared max-concurrent, or a serialized
	// stream zeroed because claims are unknown) — as distinct from the ones the
	// span-of-control cap held back. The board used to attribute every held-back
	// brief to the span cap, sending a reader chasing WIP pressure to the wrong
	// knob.
	HeldByStreamCap int
	// HeldByStreamDetail decomposes HeldByStreamCap per stream ("<stream>" →
	// count held back by THAT stream's per-stream cap), so the board can name
	// WHICH streams are at their dispatch cap and which one holds the most back
	// — an aggregate "N held back" reads as a drained/empty board when in fact a
	// single capped stream (e.g. a busy issue-loop) is sitting on the whole
	// backlog. Populated alongside every HeldByStreamCap increment; the map sum
	// equals HeldByStreamCap.
	HeldByStreamDetail map[string]int
	// SerializedUnknown names the streams that DECLARED max-concurrent but were
	// held back to zero because claim filtering did not run — the could-not-check
	// state. It is reported, never silently treated as "no claims, full budget".
	SerializedUnknown []string
	// MeasuresGated names the briefs ("<stream>/<NN>", sorted) that would have
	// been eligible but declare `measures: <queue>` on a queue currently over its
	// alarm threshold — drain-before-instrument. A brief here is held back, not
	// retired: it returns to the board the moment the queue drains.
	MeasuresGated []string
	// MeasuresUnknown names the briefs held back because the queue their
	// `measures:` field points at could not be read at all (unwired or misspelled
	// name) — the could-not-check state. Held CLOSED and NAMED: see queueBlocks
	// for why that direction, and emit for the banner that makes it visible.
	MeasuresUnknown []string
	// --- Drive fields (methodology-metrics/45). All zero when no drive is active,
	// keeping a no-manifest board byte-identical to the pre-drives generator. ---
	//
	// DriveBanner is the ACTIVE DRIVE honesty banner, set iff a SHOWN pick is
	// boosted; emit renders it above the picks. DriveNotApplied is the fail-neutral
	// "DRIVE NOT APPLIED" banner for a rejected manifest. DriveCoverageNotices carry
	// the anti-Goodhart coverage>threshold NOTICEs (also surfaced on --lint).
	DriveBanner          string
	DriveNotApplied      string
	DriveCoverageNotices []string
	// HeldByDriveCap counts drive-boosted briefs the anti-starvation floor held OFF
	// the board (mirrors HeldByStreamCap): drive picks past driveSlotCap that pass 2
	// could not backfill because the span filled with non-drive work. Populated by
	// the ≤15/20 2-pass fill (phase 3). 0 whenever no drive is active, so a no-drive
	// board is byte-identical to the pre-drives baseline.
	HeldByDriveCap int
}

// Overflow reports whether the eligible backlog exceeds the overflow threshold —
// i.e. Next-up is holding briefs back. An overflow is itself an alarm (SCADA /
// EEMUA-191): it must be rendered explicitly, never silently truncated.
func (n NextUp) Overflow() bool { return n.Eligible > n.Threshold }

// HeldBack is the number of eligible briefs not shown (Eligible − shown).
func (n NextUp) HeldBack() int { return n.Eligible - len(n.Picks) }

// HeldBySpan is the number of eligible briefs not shown because the
// span-of-control cap filled: everything held back that a per-stream cap or the
// drive anti-starvation floor did not already hold back. With no active drive
// HeldByDriveCap is 0, so this is unchanged from the pre-phase-3 accounting.
func (n NextUp) HeldBySpan() int { return n.HeldBack() - n.HeldByStreamCap - n.HeldByDriveCap }

// heldBackReason names WHICH cap actually held briefs back, for the overflow
// line and the --lint NOTICE. Both used to attribute every held-back brief to
// the span-of-control cap, which is the wrong knob whenever a per-stream cap
// (perStreamCap, a declared max-concurrent, or a serialized stream zeroed by
// could-not-check) is what fired — and it is the wrong knob most of the time,
// since perStreamCap is 4 and the span is 20.
func heldBackReason(n NextUp) string {
	switch {
	case n.HeldByStreamCap == 0:
		return fmt.Sprintf("span-of-control cap %d", n.Span)
	case n.HeldBySpan() <= 0:
		return fmt.Sprintf("per-stream caps; span-of-control cap %d not reached", n.Span)
	default:
		return fmt.Sprintf("%d by per-stream caps, %d by the span-of-control cap %d",
			n.HeldByStreamCap, n.HeldBySpan(), n.Span)
	}
}

// heldByStreamTop ranks HeldByStreamDetail by count (descending, then name for a
// deterministic board) and renders the top `max` as "<stream> (<n>)" fragments.
// It also returns the single top holder's name and the total held across all
// streams. This is what lets the overflow section say "N held by per-stream caps
// (top: issue-loop)" instead of a bare count that reads like an empty board: a
// stream sitting at its dispatch cap is holding work back, not out of work. A
// non-positive `max` renders every stream.
func heldByStreamTop(detail map[string]int, max int) (frags []string, top string, total int) {
	type kv struct {
		name string
		n    int
	}
	xs := make([]kv, 0, len(detail))
	for name, n := range detail {
		if n <= 0 {
			continue
		}
		xs = append(xs, kv{name, n})
		total += n
	}
	sort.Slice(xs, func(i, j int) bool {
		if xs[i].n != xs[j].n {
			return xs[i].n > xs[j].n
		}
		return xs[i].name < xs[j].name
	})
	if len(xs) > 0 {
		top = xs[0].name
	}
	for i, x := range xs {
		if max > 0 && i >= max {
			break
		}
		frags = append(frags, fmt.Sprintf("%s (%d)", x.name, x.n))
	}
	return frags, top, total
}

func priorityWeight(p string) int {
	switch p {
	case "P0":
		return weightP0
	case "P1":
		return weightP1
	default:
		return weightP2
	}
}

// valueWeight maps the optional brief `value:` field to its score term. An
// absent field ("") and an explicit "med" are the same neutral zero point, so a
// brief that does not opt in scores exactly as it did before.
func valueWeight(v string) int {
	switch v {
	case "low":
		return valueWeightLow
	case "high":
		return valueWeightHigh
	default: // "" (absent) or "med"
		return valueWeightMed
	}
}

// buildRevDeps builds the reverse typed-`depends:` graph and a status index over
// every brief in the given streams. rev[X] lists the briefs that directly depend
// on X (i.e. are gated on X reaching done); status[X] is X's lifecycle token.
// This is the single shared machinery for blockedCount — the
// gate-score reuses it rather than re-deriving the graph.
func buildRevDeps(streams []*Stream) (rev map[string][]string, status map[string]string) {
	rev = map[string][]string{}
	status = map[string]string{}
	for _, s := range streams {
		for _, b := range s.Briefs {
			id := s.Name + "/" + b.Num
			status[id] = b.Status
			for _, dep := range b.Depends {
				rev[dep] = append(rev[dep], id)
			}
		}
	}
	return rev, status
}

// blockedCount is the held-up-by term: the number of
// distinct NOT-done briefs that transitively depend on `target` reaching done —
// i.e. how many open items this brief unblocks. It reverse-walks the typed
// depends graph built by buildRevDeps; a visited set makes it cycle-safe, and an
// unknown/unresolvable dep simply contributes no reachable node. target itself
// is never counted.
func blockedCount(rev map[string][]string, status map[string]string, target string) int {
	seen := map[string]bool{target: true}
	stack := append([]string(nil), rev[target]...)
	count := 0
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[n] {
			continue
		}
		seen[n] = true
		if status[n] != "done" {
			count++
		}
		stack = append(stack, rev[n]...)
	}
	return count
}

// measuredQueue is the resolved state of one process queue that a brief's
// `measures:` field may name.
//
// Known is deliberately separate from Breached: "the queue is fine" and "we
// could not find out" are different answers and must not collapse into one
// boolean. See queueBlocks for which way the could-not-check state falls.
type measuredQueue struct {
	Known    bool // the depth of this queue could actually be read
	Breached bool // the depth is over this queue's OWN alarm threshold
}

// wiredQueues resolves every process queue statusgen can measure, for the
// drain-before-instrument gate. Exactly ONE queue is wired: verification-debt,
// whose depth and threshold already live in debtCounts/verificationDebtThreshold
// (mm/10). Any other name a brief writes is unresolvable here and lands in the
// could-not-check arm of queueBlocks.
//
// Deliberately NOT extensible-by-guess: adding a key is a promise that a real
// depth and a real threshold exist behind the name.
func wiredQueues(streams []*Stream) map[string]measuredQueue {
	return map[string]measuredQueue{
		"verification-debt": {Known: true, Breached: debtBreached(streams)},
	}
}

// queueBlocks decides whether a brief's `measures:` field must hold it out of
// Next-up. It returns (blocked, unknown):
//
//	measures absent          → (false, false) — the neutral default. An ordinary
//	                           brief is untouched by this gate, full stop.
//	queue known, over        → (true,  false) — drain before you instrument.
//	queue known, under       → (false, false) — the field is inert.
//	queue unreadable/unknown → (true,  true)  — COULD NOT CHECK.
//
// The could-not-check arm FAILS CLOSED, and this is the load-bearing choice, so
// here is the reasoning rather than an assertion:
//
//   - Failing open re-permits, silently, exactly the dispatch this gate exists
//     to stop — and it does so on the briefs whose authors opted IN to being
//     gated. The gate would be strongest when least needed and absent when a
//     name is fat-fingered.
//   - Failing closed hides eligible work, which is the class of bug that hid 596
//     briefs from a dispatcher (statusgen's StaleRef broadcast, fixed in v0.8.2).
//     That bug was dangerous because it was SILENT. So closed-plus-silent is not
//     on the menu here: every brief held by this arm is named in
//     NextUp.MeasuresUnknown and rendered as a COULD NOT CHECK banner, and the
//     same bad name is a hard `--lint` PROBLEM naming the file.
//   - The blast radius is bounded and self-selected: only briefs that wrote a
//     `measures:` field can be held. A board where nobody wrote one cannot lose
//     a single brief to this arm.
//
// Bounded, loud, and self-selected beats unbounded, silent, and universal —
// which is the same call desk-hardening/01 made for serialized streams whose
// in-flight set could not be read (SerializedUnknown).
func queueBlocks(queues map[string]measuredQueue, measures *string) (blocked, unknown bool) {
	if measures == nil {
		return false, false
	}
	q, ok := queues[strings.TrimSpace(*measures)]
	if !ok || !q.Known {
		return true, true
	}
	return q.Breached, false
}

// eligible reports whether a brief may be offered on the Next-up board.
// queues carries the resolved measured-queue state for the drain-before-
// instrument gate; a nil map makes every `measures:` field unresolvable, i.e.
// could-not-check (fail closed) — pass wiredQueues(streams) to gate for real.
func eligible(streams []*Stream, s *Stream, b Brief, claimed map[string]bool, queues map[string]measuredQueue) bool {
	if !eligibleBase(streams, s, b, claimed) {
		return false
	}
	// Drain-before-instrument: a TODO brief that instruments a queue is not
	// dispatchable while that queue is over its own alarm threshold. An
	// eligibility exclusion exactly like claim-awareness — ZERO score-input
	// change (F-09 boundary): a brief that survives the gate scores what it
	// always scored.
	//
	// TODO only, on purpose. Excluding an in-progress brief would evict work
	// already in flight from the board rather than prevent it being started —
	// hiding the thing someone is holding is not the same as not handing it out.
	if b.Status == "todo" {
		if blocked, _ := queueBlocks(queues, b.Measures); blocked {
			return false
		}
	}
	return true
}

// eligibleBase is the eligibility rule set that does NOT depend on measured
// queue state: stream/stale/claim exclusions plus the status, dependency and
// wave rules. Split out so the pick loop can ask "would this brief have been
// eligible but for the drain-before-instrument gate?" and attribute the
// held-back brief honestly, instead of guessing.
func eligibleBase(streams []*Stream, s *Stream, b Brief, claimed map[string]bool) bool {
	if s.Status != "active" || b.StaleRef != "" {
		return false
	}
	if claimed[s.Name+"/"+b.Num] {
		return false
	}
	switch b.Status {
	case "in-progress":
		return true
	case "todo":
		// placeholder-v1: an issue-loop placeholder is issue-driven — never
		// wave/dep gated. Claim-awareness (checked above) still
		// excludes one whose issue has an open branch.
		if b.Schema == "placeholder-v1" {
			if b.Blocked != "" {
				return false // blocked awaiting issue response
			}
			return true
		}
		// brief-v1: gate on the brief's own typed depends list.
		// Non-brief-v1 (legacy): keep the existing whole-wave rule.
		// claim-aware filtering removes in-flight on open branches/PRs;
		// this changes todo eligibility gating only — zero score-input
		// changes (F-09/I-13 boundary).
		if b.Schema == "brief-v1" {
			for _, dep := range b.Depends {
				if !depIsSatisfied(streams, dep) {
					return false
				}
			}
			return true // empty Depends → eligible now
		}
		// Legacy wave rule: every lower-wave brief in the same stream
		// must be done or verified.
		for _, o := range s.Briefs {
			if o.Wave < b.Wave && o.Status != "done" && o.Status != "verified" {
				return false
			}
		}
		return true
	}
	return false
}

// depIsSatisfied checks whether a typed dep "<stream>/<NN>" is done or verified.
// An unresolvable dep (shouldn't happen — checkRef validates at lint time)
// returns false, gating the dependent brief.
func depIsSatisfied(streams []*Stream, dep string) bool {
	parts := strings.SplitN(dep, "/", 2)
	if len(parts) != 2 {
		return false
	}
	for _, s := range streams {
		if s.Name != parts[0] {
			continue
		}
		for _, b := range s.Briefs {
			if b.Num == parts[1] {
				return b.Status == "done" || b.Status == "verified"
			}
		}
	}
	return false // unresolved dep → not satisfied
}

// GateScore is a scored awaiting brief for gate-queue prioritization. The
// formula mirrors Next-up: priorityWeight +
// staleness×stalenessPerDay + valueWeight + unblocksWeight×blockedCount.
type GateScore struct {
	Stream       *Stream
	Brief        Brief
	Score        int
	BlockedCount int
}

// gateScores computes and returns all awaiting (implemented/verified) briefs
// sorted by gate-score descending. The formula is the same as Next-up — the
// weights are an EVOLVING HEURISTIC (F-09 discipline).
// briefTouch holds per-brief last-transition times from the historian; nil means
// fall back to stream LastTouch. Staleness is capped at stalenessCapDays.
func gateScores(streams []*Stream, briefTouch map[string]time.Time) []GateScore {
	if len(streams) == 0 {
		return nil
	}
	// "now" mirrors nextUp: the most-recent activity anywhere.
	var now = streams[0].LastTouch
	for _, s := range streams {
		if s.LastTouch.After(now) {
			now = s.LastTouch
		}
	}
	for _, t := range briefTouch {
		if t.After(now) {
			now = t
		}
	}
	rev, status := buildRevDeps(streams)
	// ageDays mirrors nextUp's closure: per-brief staleness from the historian
	// when known, else the stream touch.
	ageDays := func(s *Stream, b Brief) int {
		ref := s.LastTouch
		if briefTouch != nil {
			if t, ok := briefTouch[s.Name+"/"+b.Num]; ok {
				ref = t
			}
		}
		days := int(now.Sub(ref).Hours() / 24)
		if days < 0 {
			days = 0
		}
		if days > stalenessCapDays {
			days = stalenessCapDays
		}
		return days
	}
	var results []GateScore
	for _, s := range streams {
		for _, b := range s.Briefs {
			if b.Status != "implemented" && b.Status != "verified" {
				continue
			}
			bc := blockedCount(rev, status, s.Name+"/"+b.Num)
			score := priorityWeight(s.Priority) +
				ageDays(s, b)*stalenessPerDay +
				valueWeight(b.Value) +
				unblocksWeight*bc
			results = append(results, GateScore{
				Stream: s, Brief: b, Score: score, BlockedCount: bc,
			})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Stream.Name != results[j].Stream.Name {
			return results[i].Stream.Name < results[j].Stream.Name
		}
		return results[i].Brief.Num < results[j].Brief.Num
	})
	return results
}

// nextUp ranks eligible briefs by a priority + staleness + value + held-up-by
// score, capped per-stream and by the overall span-of-control cap (spanOfControl).
// It returns a NextUp carrying the capped picks plus the total eligible count, so
// the caller can render the overflow indicator when the backlog exceeds what the
// span shows (overflow is an alarm, never a silent
// truncation).
//
// score(brief) = priorityWeight(stream) + staleness×stalenessPerDay
//   - valueWeight(brief.value) + unblocksWeight×blockedCount(brief)
//
// The weights are an EVOLVING HEURISTIC (F-09 discipline),
// documented as tunables at their const declarations — not a claim of truth.
//
// Staleness clock (F-09 fix): a brief ages from its OWN last recorded status
// transition (briefTouch, from the historian log) rather than the stream's git
// LastTouch, so a sibling brief's activity no longer resets an unrelated item's
// aging. briefTouch maps "stream/NN" → last-transition time; a brief absent from
// it (or a nil map, as in unit tests) falls back to the stream LastTouch — the
// pre-14 behaviour. Aging still accumulates without bound below the cap, so value
// and blockedCount never starve an old brief: it eventually floats regardless.
//
// claims carries the "stream/NN" claim keys (see claimedBriefs) for briefs that
// already have an open remote branch or PR — those are excluded from the
// candidate set so two sessions don't converge on the same pick — TOGETHER with
// whether that claim set could actually be read (ClaimView). The zero
// ClaimView means claim filtering did not run.
//
// Per-stream max-concurrent: a stream README may set
// `max-concurrent: N` (1 <= N <= perStreamCap) to restrict its draw below the
// global perStreamCap. The effective per-stream limit for a stream s is
// max(0, min(perStreamCap, s.MaxConcurrent) - inFlight(s)) where inFlight(s)
// counts the number of claimed briefs for that stream. This enforces the F-20
// serialization mandate (e.g. a hardening stream with max-concurrent: 1) at the
// board level — a stream with one claimed brief and max-concurrent: 1 offers
// zero additional picks until the claim lifts. The cap is a zero-score-input
// change (F-09 boundary): it gates only, never re-ranks.
//
// Three-state, not two: when claim filtering did NOT run, in-flight is
// UNKNOWABLE, and the declaration cannot be honoured. A stream that DECLARED
// max-concurrent is then held back to zero and named in SerializedUnknown —
// offering its declared budget anyway would re-permit exactly the parallel
// dispatch the declaration forbids, silently, on the streams that asked
// hardest not to have it. Streams that declared NOTHING are untouched by this:
// their semantics are precisely what they were, and the board-wide degraded
// banner already labels them.
func nextUp(streams []*Stream, claims ClaimView, briefTouch map[string]time.Time) NextUp {
	claimed := claims.Claimed
	nu := NextUp{Span: spanOfControl, Threshold: overflowThreshold, Claims: claims.Source, HeldByStreamDetail: map[string]int{}}
	if len(streams) == 0 {
		return nu
	}
	// "now" is the most-recent activity anywhere — the latest stream git touch
	// or recorded brief transition. Deterministic (no wall clock) so STATUS.md
	// stays byte-stable; everything ages relative to it.
	var now = streams[0].LastTouch
	for _, s := range streams {
		if s.LastTouch.After(now) {
			now = s.LastTouch
		}
	}
	for _, t := range briefTouch {
		if t.After(now) {
			now = t
		}
	}
	rev, status := buildRevDeps(streams)
	// ageDays returns the capped staleness (in days) for one brief, measured from
	// its own last transition when the historian knows it, else the stream touch.
	ageDays := func(s *Stream, b Brief) int {
		ref := s.LastTouch
		if briefTouch != nil {
			if t, ok := briefTouch[s.Name+"/"+b.Num]; ok {
				ref = t
			}
		}
		days := int(now.Sub(ref).Hours() / 24)
		if days < 0 {
			days = 0
		}
		if days > stalenessCapDays {
			days = stalenessCapDays
		}
		return days
	}
	// Resolve the measured-queue state ONCE for the whole board: the depth is a
	// property of the board, not of a brief, and re-deriving it per brief would
	// be quadratic over every Evidence body on the page.
	queues := wiredQueues(streams)
	var all []Pick
	for _, s := range streams {
		for _, b := range s.Briefs {
			if !eligible(streams, s, b, claimed, queues) {
				// Attribute the drain-before-instrument exclusions so the board
				// can name them. Only a brief that was OTHERWISE eligible counts:
				// a paused-stream or claimed brief carrying `measures:` is held by
				// something else and must not be reported as gate pressure.
				if blocked, unknown := queueBlocks(queues, b.Measures); blocked &&
					b.Status == "todo" && eligibleBase(streams, s, b, claimed) {
					id := s.Name + "/" + b.Num
					if unknown {
						nu.MeasuresUnknown = append(nu.MeasuresUnknown, id)
					} else {
						nu.MeasuresGated = append(nu.MeasuresGated, id)
					}
				}
				continue
			}
			bc := blockedCount(rev, status, s.Name+"/"+b.Num)
			score := priorityWeight(s.Priority) +
				ageDays(s, b)*stalenessPerDay +
				valueWeight(b.Value) +
				unblocksWeight*bc
			p := Pick{Stream: s, Brief: b, Score: score}
			// Hard critical tier (phase 3): mark machine-derived / stamped critical
			// briefs so the sort ranks them ABOVE every score. GATED on a drive being
			// active — with no drive there is nothing to bury and the ordinary score
			// already orders the board, so a no-drive board stays byte-identical to
			// the baseline (a high-unblocks brief would otherwise reorder it). The
			// derivation is pure/deterministic over board-graph facts + stamped labels
			// only (drivecritical.go).
			if activeDriveSet.applied() {
				if arm := criticalTierArm(b, s.Name, bc, activeFindings); arm != "" {
					p.CriticalTier = true
					p.CriticalArm = arm
				}
			}
			all = append(all, p)
		}
	}
	nu.Eligible = len(all)
	sort.Strings(nu.MeasuresGated)
	sort.Strings(nu.MeasuresUnknown)
	// Drive steer (methodology-metrics/45). Applied HERE — AFTER eligibility and
	// the base score are fixed — so it can only RE-RANK the eligible set, never
	// change what is eligible (exactly like value/blockedCount). A malformed/
	// expired/over-concurrent manifest sets DriveNotApplied and contributes zero
	// term; the zero DriveSet contributes nothing at all, so this whole block is a
	// no-op on a no-manifest board (byte-identical baseline).
	if activeDriveSet.NotApplied {
		nu.DriveNotApplied = driveNotAppliedText(activeDriveSet)
	}
	if activeDriveSet.applied() {
		coverage, covNotices := activeDriveSet.driveCoverage(all)
		nu.DriveCoverageNotices = covNotices
		for i := range all {
			term, slug := activeDriveSet.driveBoost(all[i].Stream.Name, all[i].Brief.Num, coverage)
			all[i].DriveTerm = term
			all[i].DriveSlug = slug
		}
	}
	sort.Slice(all, func(i, j int) bool {
		// Lexicographic (CriticalTier, Total()): the HARD critical tier ranks ABOVE
		// every score, so no intensity — surge included — can bury a live fire. The
		// drive term re-ranks only WITHIN a tier. CriticalTier is set only when a
		// drive is active, so with no drive every pick is non-critical here and this
		// clause never fires — the ordering is then the exact pre-drives ordering.
		if all[i].CriticalTier != all[j].CriticalTier {
			return all[i].CriticalTier // critical members sort first
		}
		// Rank by the TOTAL (base + drive term). With no drive every term is 0, so
		// Total == Score and this is the exact pre-drives ordering.
		if all[i].Total() != all[j].Total() {
			return all[i].Total() > all[j].Total()
		}
		if all[i].Stream.Name != all[j].Stream.Name {
			return all[i].Stream.Name < all[j].Stream.Name
		}
		return all[i].Brief.Num < all[j].Brief.Num
	})
	// Build per-stream effective caps before the pick loop: a stream with
	// max-concurrent: N gets a tighter cap than perStreamCap (the knob restricts,
	// never widens), and in-flight claimed briefs subtract from that budget so a
	// serialized stream with one claimed brief offers zero additional picks
	// (F-20).
	streamCaps := map[string]int{}
	for _, s := range streams {
		cap := perStreamCap
		declared := s.MaxConcurrent != nil
		if declared && *s.MaxConcurrent < cap {
			cap = *s.MaxConcurrent
		}
		// effectiveCap = min(perStreamCap-or-MaxConcurrent, driveStreamCap): an active
		// drive may declare a per-stream `drive-stream-cap:` that RESTRICTS a stream's
		// draw further (phase 3). It is a min, so a stream's declared MaxConcurrent
		// (mm/13) ALWAYS wins — the drive cap can never widen a stream above it. With
		// no active drive, streamDriveCap reports (0,false) for every stream, so this
		// is a no-op and the baseline is unchanged.
		if dc, ok := activeDriveSet.streamDriveCap(s.Name); ok && dc < cap {
			cap = dc
		}
		// Could-not-check: a declared max-concurrent is a request to serialize,
		// and honouring it requires knowing what is already in flight. With no
		// claim read there is no in-flight signal, so the request cannot be
		// honoured — hold the stream back to zero and report it, rather than
		// fall back to its nominal budget and re-permit the parallel dispatch
		// the declaration exists to forbid.
		if declared && !claims.Source.Known {
			streamCaps[s.Name] = 0
			nu.SerializedUnknown = append(nu.SerializedUnknown, s.Name)
			continue
		}
		if claimed != nil {
			prefix := s.Name + "/"
			for k := range claimed {
				if strings.HasPrefix(k, prefix) {
					cap--
				}
			}
		}
		if cap < 0 {
			cap = 0
		}
		streamCaps[s.Name] = cap
	}
	sort.Strings(nu.SerializedUnknown)
	// Anti-starvation 2-PASS fill (phase 3). Because staleness caps at +300, a buried
	// same-tier brief can never out-age a drive, so drive work must be FLOORED or it
	// starves the rest of the board permanently-by-arithmetic. Pass 1 offers non-drive
	// picks and drive-boosted picks up to driveSlotCap (15) of the 20 spanOfControl
	// slots, reserving ≥5 non-drive-consumable slots; a drive pick past the cap is
	// held back. Pass 2 backfills any span the non-drive picks left empty with the
	// held-back drive picks — so the reserved slots go to non-drive work when it
	// exists, and the board is never left short when it does not. A held-back drive
	// pick that stays off the board attributes to HeldByDriveCap (mirrors
	// HeldByStreamCap). With no active drive no pick is drive-boosted, so pass 1 is the
	// exact pre-phase-3 single loop and pass 2 is a no-op (byte-identical baseline).
	perStream := map[string]int{}
	var picks []Pick
	var heldDrive []Pick // drive picks the driveSlotCap floor held back from pass 1
	driveShown := 0
	// Pass 1.
	for _, p := range all {
		if len(picks) >= spanOfControl {
			break
		}
		cap := streamCaps[p.Stream.Name]
		if cap == 0 || perStream[p.Stream.Name] >= cap {
			nu.HeldByStreamCap++
			nu.HeldByStreamDetail[p.Stream.Name]++
			continue
		}
		if p.DriveTerm > 0 && driveShown >= driveSlotCap {
			// Floored: hold this drive pick back so a non-drive brief can take the
			// reserved slot. It does NOT consume the stream's cap here — pass 2
			// re-checks the cap if it backfills.
			heldDrive = append(heldDrive, p)
			continue
		}
		perStream[p.Stream.Name]++
		if p.DriveTerm > 0 {
			driveShown++
		}
		picks = append(picks, p)
	}
	// Pass 2: backfill remaining span with the held-back drive picks (rank order),
	// still honoring per-stream caps. A backfilled pick is SHOWN (not held); one that
	// stays off the board is attributed — by the drive floor (span full) or by a
	// per-stream cap.
	for _, p := range heldDrive {
		if len(picks) >= spanOfControl {
			nu.HeldByDriveCap++
			continue
		}
		cap := streamCaps[p.Stream.Name]
		if cap == 0 || perStream[p.Stream.Name] >= cap {
			nu.HeldByStreamCap++
			nu.HeldByStreamDetail[p.Stream.Name]++
			continue
		}
		perStream[p.Stream.Name]++
		picks = append(picks, p)
	}
	nu.Picks = picks
	// Honesty (brief-44 Verify row 3): the ACTIVE DRIVE banner renders iff a SHOWN
	// pick actually carries a drive term. A boosted pick reaching the board without
	// this banner is a --lint PROBLEM (driveBannerProblems) — the steer is never
	// presented as a merged, unattributed number.
	for _, p := range picks {
		if p.DriveTerm > 0 {
			nu.DriveBanner = driveBannerText(activeDriveSet)
			break
		}
	}
	return nu
}
