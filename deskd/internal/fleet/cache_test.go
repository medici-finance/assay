package fleet

import (
	"context"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
)

func TestClassifyPR(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		pr   adapter.RawPullRequest
		want State
	}{
		{"conflicting outranks review state", adapter.RawPullRequest{Mergeable: "CONFLICTING", ReviewDecision: "APPROVED"}, StateConflicting},
		{"no review yet, not draft", adapter.RawPullRequest{Mergeable: "MERGEABLE", ReviewDecision: ""}, StateAwaitingReview},
		{"review required, not draft", adapter.RawPullRequest{Mergeable: "MERGEABLE", ReviewDecision: "REVIEW_REQUIRED"}, StateAwaitingReview},
		{"draft with no review is working, not awaiting-review", adapter.RawPullRequest{Draft: true, Mergeable: "UNKNOWN", ReviewDecision: ""}, StateWorking},
		{"changes requested is working (the author's move)", adapter.RawPullRequest{Mergeable: "MERGEABLE", ReviewDecision: "CHANGES_REQUESTED"}, StateWorking},
		{"approved and landing is working", adapter.RawPullRequest{Mergeable: "MERGEABLE", ReviewDecision: "APPROVED"}, StateWorking},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyPR(tc.pr, now)
			if got.State != tc.want {
				t.Errorf("classifyPR(%+v) state = %q, want %q", tc.pr, got.State, tc.want)
			}
			if got.Palette() != tc.want.Palette() {
				t.Errorf("palette mismatch")
			}
		})
	}
}

func TestClassifyClaim(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	staleAfter := 30 * time.Minute

	fresh := classifyClaim(adapter.RawClaim{RefName: "refs/dispatch/a", ClaimedAt: now.Add(-5 * time.Minute), Holder: "worker-1"}, now, staleAfter)
	if fresh.State != StateWorking {
		t.Errorf("fresh claim state = %q, want working", fresh.State)
	}
	if fresh.Key != "a" {
		t.Errorf("claim key = %q, want %q (refs/dispatch/ prefix stripped)", fresh.Key, "a")
	}

	stale := classifyClaim(adapter.RawClaim{RefName: "refs/dispatch/b", ClaimedAt: now.Add(-45 * time.Minute), Holder: "worker-2"}, now, staleAfter)
	if stale.State != StaleClaimState {
		t.Errorf("old claim state = %q, want stale-claim (NEGATIVE PATH: a stuck claim must not render as working)", stale.State)
	}
	if stale.Palette() != "amber" {
		t.Errorf("stale-claim palette = %q, want amber", stale.Palette())
	}

	// A claim whose ref target carried no committed date (ClaimedAt is the
	// zero value) must never be read as "just claimed" — that would be
	// exactly the false-freshness the brief calls worse than no map.
	unknown := classifyClaim(adapter.RawClaim{RefName: "refs/dispatch/c"}, now, staleAfter)
	if unknown.State != StateWorking || unknown.Age != 0 {
		t.Errorf("unknown-age claim should render age 0 / working (never a fabricated stale OR fresh verdict from a missing timestamp), got state=%q age=%v", unknown.State, unknown.Age)
	}
}

// fakeClient lets tests control FetchOpenPullRequests / FetchDispatchClaims
// results without a real GitHub API — this exercises [Cache] through the
// same *adapter.Client construction NewCache uses, so we build a Cache by
// hand for the parts that need a controllable failure, and exercise
// [Cache.fallback] / [Cache.snapshotOne] directly.

// TestCacheServesLastGoodMarkedStaleOnFetchFailure is the negative-path
// pin for the brief's Verify row 2: "kill the data fetch ... tokens render
// in the stale visual state with age badges — not last-known-good-as-fresh."
//
// It exercises [Cache.fallback] directly (the code path snapshotOne takes
// when a live read fails) rather than standing up a fake GitHub server,
// because the property under test is what the CACHE does with a failure,
// not how the failure was produced — [TestFetchOpenPullRequestsCappedIsNotOK]
// and its siblings in internal/adapter already cover the fetch layer itself.
func TestCacheServesLastGoodMarkedStaleOnFetchFailure(t *testing.T) {
	t0 := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	c := NewCache("https://api.github.com", mapSource{"example-org": "tok"}, []string{"example-org"}, nil,
		time.Minute, 30*time.Minute, func() time.Time { return t0 })

	// Seed a good snapshot as if a prior live read had succeeded.
	c.mu.Lock()
	c.last["example-org/console"] = cachedSnapshot{
		tokens:    []Token{{Kind: KindPR, Key: "#1", State: StateWorking, Age: 5 * time.Minute}},
		fetchedAt: t0,
	}
	c.mu.Unlock()

	later := t0.Add(10 * time.Minute)
	lane := c.fallback("example-org/console", later, "simulated fetch failure")

	if !lane.OK {
		t.Fatalf("fallback with a cached snapshot must still report OK=true (we DO have data, it is just aging), got OK=false reason=%q", lane.Reason)
	}
	if !lane.Stale {
		t.Fatalf("NEGATIVE PATH FAILED: a lane served from the last-good cache after a fetch failure must render Stale=true, not last-known-good-as-fresh")
	}
	if lane.Reason == "" {
		t.Fatalf("a stale lane must carry the reason the live refresh failed")
	}
	if len(lane.Tokens) != 1 || lane.Tokens[0].Age != 15*time.Minute {
		t.Fatalf("token age must be re-derived against NOW (5m at cache time + 10m drift = 15m), got %+v", lane.Tokens)
	}

	// No cache at all: could-not-check, never an empty-looking pass.
	empty := c.fallback("example-org/never-fetched", later, "no read token")
	if empty.OK {
		t.Fatalf("a repo with no cached snapshot and a failed fetch must report OK=false (could-not-check), got OK=true")
	}
	if empty.Reason == "" {
		t.Fatalf("could-not-check with no reason is not a finding")
	}
}

// TestCacheDegradesOneRepoNotTheWholeSnapshot pins the "one repo, one
// slice" discipline: an owner with no routed token degrades only that
// repo's lane.
func TestCacheDegradesOneRepoNotTheWholeSnapshot(t *testing.T) {
	c := NewCache("https://api.github.com", mapSource{"example-org": "tok"}, []string{"example-org"}, nil,
		time.Minute, 30*time.Minute, nil)

	lanes := c.Snapshot(context.Background(), []string{"unrouted-org/some-repo"})
	if len(lanes) != 1 {
		t.Fatalf("expected one lane, got %d", len(lanes))
	}
	if lanes[0].OK {
		t.Fatalf("an owner with no routed token must report could-not-check, not OK")
	}
	if lanes[0].Reason == "" {
		t.Fatalf("could-not-check with no reason is not a finding")
	}
}

func TestStaleThresholdIsTheNamedMultiplierTimesCadence(t *testing.T) {
	got := StaleThreshold(60 * time.Second)
	want := time.Duration(StaleThresholdMultiplier) * 60 * time.Second
	if got != want {
		t.Fatalf("StaleThreshold(60s) = %v, want %v", got, want)
	}
}

// mapSource is a test-only [adapter.TokenSource] over a fixed owner->token
// map. Production code gets a TokenSource from the env or the token-file
// directory, both re-read on every Resolve.
type mapSource map[string]string

func (m mapSource) Resolve(owners []string) (map[string]string, []adapter.MissingToken) {
	out := map[string]string{}
	var missing []adapter.MissingToken
	for _, o := range owners {
		if t, ok := m[o]; ok && t != "" {
			out[o] = t
		} else {
			missing = append(missing, adapter.MissingToken{Owner: o, Detail: "test map"})
		}
	}
	return out, missing
}
