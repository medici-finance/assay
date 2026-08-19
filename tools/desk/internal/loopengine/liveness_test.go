package loopengine

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// probeReturning builds an ObservableProbe that always yields the same observation/error —
// the deterministic stand-in for a real read-only source (audit scan, git ls-remote, PR API).
func probeReturning(obs Observation, err error) ObservableProbe {
	return func(Item, time.Time) (Observation, error) { return obs, err }
}

// trackWith registers one item on a fresh tracker and returns both, so a test can set the
// worker's clock (dispatchedAt via register, startedAt/lastObserved via markStarted) before it
// sweeps.
func trackWith(it Item, tier Tier, dispatchedAt time.Time) (*livenessTracker, *livenessState) {
	t := newLivenessTracker()
	t.register(it, tier, dispatchedAt)
	return t, t.states[it.ID]
}

// markStarted forces the worker's started/heartbeat clock to `at`, simulating a worker that
// emitted its first (and last) observable event `at`.
func markStarted(s *livenessState, at time.Time) {
	s.startedAt = at
	s.lastObserved = at
}

// dispositions reduces a sweep result to a per-item disposition map for terse assertions.
func dispositions(out []livenessOutcome) map[string]livenessDisposition {
	m := map[string]livenessDisposition{}
	for _, o := range out {
		m[o.item.ID] = o.disp
	}
	return m
}

// A never-booted worker is reclaimed at ScheduleToStart (minutes), NOT left to the 120m
// StaleClaim backstop — the whole point of decomposing the coarse knob.
func TestLivenessNeverBootedReclaimedAtScheduleToStart(t *testing.T) {
	now := time.Now()
	pol := LivenessPolicy{ScheduleToStart: 10 * time.Minute, HeartbeatGap: 20 * time.Minute}
	// Dispatched 15m ago, well UNDER the 120m StaleClaim — so if only StaleClaim existed this
	// worker would still be wedged. A CLEAN probe that saw nothing is what unlocks the fast call.
	tr, _ := trackWith(Item{ID: "sample/09"}, TierCheap, now.Add(-15*time.Minute))
	cfg := Config{
		StaleClaim: 120 * time.Minute,
		Liveness:   &pol,
		Observe:    &ObservableProbes{AuditScan: probeReturning(Observation{}, nil)}, // checked, no sign of life
	}
	got := dispositions(cfg.sweepLiveness(tr, now))
	if got["sample/09"] != livenessReclaimNeverStarted {
		t.Fatalf("never-booted worker: got %v; want livenessReclaimNeverStarted at ScheduleToStart (15m elapsed, StaleClaim 120m not yet reached)", got["sample/09"])
	}
}

// A worker that is silent on audit but STILL PUSHING COMMITS is alive: branch sha movement is a
// heartbeat, so it must not be reclaimed however quiet its other signals.
func TestLivenessSilentButPushingNotReclaimed(t *testing.T) {
	now := time.Now()
	pol := LivenessPolicy{ScheduleToStart: 10 * time.Minute, HeartbeatGap: 20 * time.Minute}
	// Dispatched 40m ago — long past every timer if it were silent.
	tr, _ := trackWith(Item{ID: "brief-x"}, TierCheap, now.Add(-40*time.Minute))
	cfg := Config{
		Liveness: &pol,
		Observe: &ObservableProbes{
			AuditScan:   probeReturning(Observation{}, nil),                                           // quiet audit
			BranchMoved: probeReturning(Observation{At: now.Add(-1 * time.Minute), What: "sha"}, nil), // fresh commit
		},
	}
	out := cfg.sweepLiveness(tr, now)
	if len(out) != 0 {
		t.Fatalf("silent-but-pushing worker reclaimed: %v; want alive (branch movement is a heartbeat)", dispositions(out))
	}
}

// A worker that has gone silent AND still (no observable event for HeartbeatGap) is reclaimed at
// the heartbeat timer.
func TestLivenessSilentAndStillReclaimedAtHeartbeatGap(t *testing.T) {
	now := time.Now()
	pol := LivenessPolicy{
		ScheduleToStart: 10 * time.Minute,
		StartToClose:    map[Tier]time.Duration{TierCheap: 90 * time.Minute},
		HeartbeatGap:    20 * time.Minute,
	}
	tr, s := trackWith(Item{ID: "issue-841"}, TierCheap, now.Add(-60*time.Minute))
	markStarted(s, now.Add(-30*time.Minute)) // last sign of life 30m ago > 20m gap, but only 30m running < 90m cap
	cfg := Config{
		Liveness: &pol,
		Observe:  &ObservableProbes{AuditScan: probeReturning(Observation{}, nil)}, // checked, nothing new
	}
	got := dispositions(cfg.sweepLiveness(tr, now))
	if got["issue-841"] != livenessReclaimHeartbeat {
		t.Fatalf("silent-and-still worker: got %v; want livenessReclaimHeartbeat", got["issue-841"])
	}
}

// Per-tier start-to-close caps: a short-clocked tier over its wall cap lands blocked-timeout
// while a long-clocked tier at the same age is still within budget.
func TestLivenessPerTierStartToCloseCapsApplied(t *testing.T) {
	now := time.Now()
	pol := LivenessPolicy{
		StartToClose: map[Tier]time.Duration{TierLocal: 20 * time.Minute, TierCheap: 90 * time.Minute},
		HeartbeatGap: 120 * time.Minute, // large so heartbeat does not confound the cap test
	}
	tr := newLivenessTracker()
	tr.register(Item{ID: "verify/local"}, TierLocal, now.Add(-45*time.Minute))
	tr.register(Item{ID: "impl/cheap"}, TierCheap, now.Add(-45*time.Minute))
	markStarted(tr.states["verify/local"], now.Add(-30*time.Minute)) // 30m running > 20m local cap
	markStarted(tr.states["impl/cheap"], now.Add(-30*time.Minute))   // 30m running < 90m cheap cap
	cfg := Config{
		Liveness: &pol,
		Observe:  &ObservableProbes{AuditScan: probeReturning(Observation{}, nil)},
	}
	got := dispositions(cfg.sweepLiveness(tr, now))
	if got["verify/local"] != livenessBlockedStartToClose {
		t.Fatalf("TierLocal over its 20m cap: got %v; want livenessBlockedStartToClose", got["verify/local"])
	}
	if _, present := got["impl/cheap"]; present {
		t.Fatalf("TierCheap within its 90m cap was acted on: %v; want alive (per-tier caps differ)", got["impl/cheap"])
	}
}

// A could-not-check probe is NEVER read as "no life": a never-booted-looking worker whose only
// probe errored is left alive, deferring to the StaleClaim backstop rather than reclaiming blind
// (the double-dispatch guard).
func TestLivenessCouldNotCheckNeverReclaims(t *testing.T) {
	now := time.Now()
	pol := LivenessPolicy{ScheduleToStart: 10 * time.Minute, HeartbeatGap: 20 * time.Minute}
	tr, _ := trackWith(Item{ID: "unreachable"}, TierCheap, now.Add(-30*time.Minute)) // past every timer
	cfg := Config{
		Liveness: &pol,
		Observe:  &ObservableProbes{AuditScan: probeReturning(Observation{}, errors.New("git ls-remote: network unreachable"))},
	}
	out := cfg.sweepLiveness(tr, now)
	if len(out) != 0 {
		t.Fatalf("errored probe triggered reclaim: %v; want alive (could-not-check is never 'no life')", dispositions(out))
	}
	// The same worker with a CLEAN probe (looked, saw nothing) IS reclaimed — proving it was the
	// could-not-check, not the elapsed time, that held the reclaim.
	cfg.Observe = &ObservableProbes{AuditScan: probeReturning(Observation{}, nil)}
	if got := dispositions(cfg.sweepLiveness(tr, now)); got["unreachable"] != livenessReclaimNeverStarted {
		t.Fatalf("clean probe past ScheduleToStart: got %v; want livenessReclaimNeverStarted", got["unreachable"])
	}
}

// Reclaim frees the claim through the lock-guarded compare-and-delete, and that primitive has a
// SINGLE winner under a race: N concurrent reclaimers of one claim, exactly one succeeds — the
// property that keeps a too-eager sweep from double-dispatching.
func TestLivenessReclaimSingleWinnerUnderRace(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{ClaimsDir: dir, StaleClaim: 120 * time.Minute, RunnerID: "runner-a", Branch: "brief/x"}
	it := Item{ID: "sample/09"}
	if ok, err := Claim(cfg, it); err != nil || !ok {
		t.Fatalf("seed claim: ok=%v err=%v", ok, err)
	}

	const racers = 12
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	wg.Add(racers)
	for i := 0; i < racers; i++ {
		go func() {
			defer wg.Done()
			// reclaimForLiveness is the exact primitive the sweep uses to free a claim.
			if ok, err := reclaimForLiveness(cfg, it.ID); err == nil && ok {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("reclaim winners = %d; want exactly 1 (lock-guarded compare-and-delete single winner)", wins)
	}
	// After the single reclaim the claim is gone, so a fresh Acquire succeeds — the item is
	// genuinely re-dispatchable, which is what "reclaim-eligible" buys.
	if ok, err := deskkit.Acquire(toClaimConfig(cfg), deskkit.Claim{Kind: deskkit.KindDispatch, Item: it.ID, Owner: "runner-b", Branch: "brief/y"}); err != nil || !ok {
		t.Fatalf("re-acquire after reclaim: ok=%v err=%v; want a fresh claim to succeed", ok, err)
	}
}

// DefaultLivenessPolicy is coherent: every timer is set, positive, and stays UNDER the 120m
// StaleClaim backstop so StaleClaim remains the last resort, never the first responder.
func TestLivenessDefaultPolicyUnderStaleClaimBackstop(t *testing.T) {
	p := DefaultLivenessPolicy()
	if p.ScheduleToStart <= 0 || p.HeartbeatGap <= 0 {
		t.Fatalf("default timers must be positive: %+v", p)
	}
	if p.ScheduleToStart >= deskkit.DefaultStaleClaim || p.HeartbeatGap >= deskkit.DefaultStaleClaim {
		t.Fatalf("schedule-to-start / heartbeat must stay under the %s StaleClaim backstop: %+v", deskkit.DefaultStaleClaim, p)
	}
	for tier, cap := range p.StartToClose {
		if cap <= 0 || cap > deskkit.DefaultStaleClaim {
			t.Fatalf("start-to-close cap for tier %s = %s; want (0, %s]", tier, cap, deskkit.DefaultStaleClaim)
		}
	}
}
