package main

import (
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

	// Next-up value + held-up-by terms (methodology-metrics/14, human:<name>'s R-01
	// decision). These are TUNABLE HEURISTIC WEIGHTS, not truths — the whole
	// score is an evolving heuristic (F-09 discipline: staleness still only
	// rewards age, value is a coarse three-way knob, blockedCount has no effort
	// term). When I-13 lands a better score these move; the gate-score
	// (methodology-metrics/11) follows the same weights.
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
	// higher-priority brief blocking none (the mm/11 rationale) — 500 × 3 = 1500
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
	// cap. Configurable via the --span flag. (methodology-metrics/06)
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

type Pick struct {
	Stream *Stream
	Brief  Brief
	Score  int
}

// NextUp is the computed Next-up view: the capped list of picks plus the counts
// needed to render the span-of-control overflow indicator (methodology-metrics/06).
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
}

// Overflow reports whether the eligible backlog exceeds the overflow threshold —
// i.e. Next-up is holding briefs back. An overflow is itself an alarm (SCADA /
// EEMUA-191): it must be rendered explicitly, never silently truncated.
func (n NextUp) Overflow() bool { return n.Eligible > n.Threshold }

// HeldBack is the number of eligible briefs not shown (Eligible − shown).
func (n NextUp) HeldBack() int { return n.Eligible - len(n.Picks) }

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
// brief that does not opt in scores exactly as it did before methodology-metrics/14.
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
// This is the single shared machinery for blockedCount — methodology-metrics/11's
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

// blockedCount is the held-up-by term (methodology-metrics/14): the number of
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

func eligible(streams []*Stream, s *Stream, b Brief, claimed map[string]bool) bool {
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
		// wave/dep gated (issue-loop/01). Claim-awareness (checked above) still
		// excludes one whose issue has an open branch.
		if b.Schema == "placeholder-v1" {
			if b.Blocked != "" {
				return false // issue-loop/03: blocked awaiting issue response
			}
			return true
		}
		// brief-v1: gate on the brief's own typed depends list.
		// Non-brief-v1 (legacy): keep the existing whole-wave rule.
		// mm/08 (claim-aware) filters in-flight on open branches/PRs;
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

// GateScore is a scored awaiting brief for gate-queue prioritization
// (methodology-metrics/11). The formula mirrors Next-up: priorityWeight +
// staleness×stalenessPerDay + valueWeight + unblocksWeight×blockedCount.
type GateScore struct {
	Stream       *Stream
	Brief        Brief
	Score        int
	BlockedCount int
}

// gateScores computes and returns all awaiting (implemented/verified) briefs
// sorted by gate-score descending. The formula is the same as Next-up — the
// weights are an EVOLVING HEURISTIC (methodology-metrics/14, F-09 discipline).
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
// span shows (methodology-metrics/06 — overflow is an alarm, never a silent
// truncation).
//
// score(brief) = priorityWeight(stream) + staleness×stalenessPerDay
//   - valueWeight(brief.value) + unblocksWeight×blockedCount(brief)
//
// The weights are an EVOLVING HEURISTIC (methodology-metrics/14, F-09 discipline),
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
// claimed holds "stream/NN" keys (see claimedBriefs) for briefs that already
// have an open remote branch or PR — those are excluded from the candidate set
// so two sessions don't converge on the same pick (issue #156). Pass an
// empty/nil map to disable claim filtering.
//
// Per-stream max-concurrent (methodology-metrics/13): a stream README may set
// `max-concurrent: N` (1 <= N <= perStreamCap) to restrict its draw below the
// global perStreamCap. The effective per-stream limit for a stream s is
// max(0, min(perStreamCap, s.MaxConcurrent) - inFlight(s)) where inFlight(s)
// counts the number of claimed briefs for that stream. This enforces the F-20
// serialization mandate (e.g. daml-hardening with max-concurrent: 1) at the
// board level — a stream with one claimed brief and max-concurrent: 1 offers
// zero additional picks until the claim lifts. The cap is a zero-score-input
// change (F-09 boundary): it gating only, never re-ranks.
func nextUp(streams []*Stream, claimed map[string]bool, briefTouch map[string]time.Time) NextUp {
	nu := NextUp{Span: spanOfControl, Threshold: overflowThreshold}
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
	var all []Pick
	for _, s := range streams {
		for _, b := range s.Briefs {
			if !eligible(streams, s, b, claimed) {
				continue
			}
			score := priorityWeight(s.Priority) +
				ageDays(s, b)*stalenessPerDay +
				valueWeight(b.Value) +
				unblocksWeight*blockedCount(rev, status, s.Name+"/"+b.Num)
			all = append(all, Pick{Stream: s, Brief: b, Score: score})
		}
	}
	nu.Eligible = len(all)
	sort.Slice(all, func(i, j int) bool {
		if all[i].Score != all[j].Score {
			return all[i].Score > all[j].Score
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
	// (methodology-metrics/13, F-20).
	streamCaps := map[string]int{}
	for _, s := range streams {
		cap := perStreamCap
		if s.MaxConcurrent != nil && *s.MaxConcurrent < cap {
			cap = *s.MaxConcurrent
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
	perStream := map[string]int{}
	var picks []Pick
	for _, p := range all {
		if len(picks) == spanOfControl {
			break
		}
		cap := streamCaps[p.Stream.Name]
		if cap == 0 || perStream[p.Stream.Name] >= cap {
			continue
		}
		perStream[p.Stream.Name]++
		picks = append(picks, p)
	}
	nu.Picks = picks
	return nu
}
