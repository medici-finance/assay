package dorajoin

import (
	"errors"
	"testing"
)

// TestJoinKeyThreeState (Verify item 5): a record whose merge SHA is
// unreachable via squash is emitted could-not-join, not dropped and not
// counted as a match.
func TestJoinKeyThreeState(t *testing.T) {
	quality := []JoinKey{
		{PRNumber: 42, MergeSHA: "deadbeef", StreamTaskID: "quality/11"},
		{PRNumber: 7, MergeSHA: "c0ffee00", StreamTaskID: "quality/02"},
	}
	reachable := func(sha string) (bool, error) {
		// deadbeef is reachable (a normal merge); c0ffee00 was squashed away.
		return sha == "deadbeef", nil
	}

	t.Run("matched", func(t *testing.T) {
		got := ResolveJoin(JoinKey{PRNumber: 42, MergeSHA: "deadbeef"}, quality, reachable)
		if got.State != MatchMatched {
			t.Fatalf("state = %q, want matched", got.State)
		}
		if got.QualityKey == nil || got.QualityKey.PRNumber != 42 {
			t.Fatalf("expected matched quality key PR 42, got %+v", got.QualityKey)
		}
	})

	t.Run("could-not-join on squashed SHA", func(t *testing.T) {
		got := ResolveJoin(JoinKey{PRNumber: 7, MergeSHA: "c0ffee00"}, quality, reachable)
		if got.State != MatchCouldNotJoin {
			t.Fatalf("state = %q, want could-not-join for an unreachable squashed SHA", got.State)
		}
		if got.Reason == "" {
			t.Fatalf("expected a non-empty reason naming why the join could not resolve")
		}
		// Never dropped: it is still a JoinResult carrying its DeliveryKey.
		if got.DeliveryKey.PRNumber != 7 {
			t.Fatalf("delivery key not preserved on a could-not-join result: %+v", got.DeliveryKey)
		}
	})

	t.Run("no-match on a clean but unknown key", func(t *testing.T) {
		got := ResolveJoin(JoinKey{PRNumber: 99, MergeSHA: "deadbeef"}, quality, nil)
		// No reachable callback supplied: reachability is skipped, key
		// comparison alone decides. PR 99 has no counterpart.
		if got.State != MatchNoMatch {
			t.Fatalf("state = %q, want no-match", got.State)
		}
	})

	t.Run("reachability check error is could-not-join, not dropped", func(t *testing.T) {
		boom := errors.New("git plumbing: object database unavailable")
		got := ResolveJoin(JoinKey{PRNumber: 42, MergeSHA: "deadbeef"}, quality, func(string) (bool, error) {
			return false, boom
		})
		if got.State != MatchCouldNotJoin {
			t.Fatalf("state = %q, want could-not-join on a reachability-check error", got.State)
		}
	})
}

// TestResolveJoinsNeverDrops: every delivery record yields exactly one
// JoinResult regardless of outcome.
func TestResolveJoinsNeverDrops(t *testing.T) {
	quality := []JoinKey{{PRNumber: 1, MergeSHA: "aaa"}}
	deliveries := []JoinKey{
		{PRNumber: 1, MergeSHA: "aaa"},
		{PRNumber: 2, MergeSHA: "bbb"},
		{PRNumber: 3, MergeSHA: "ccc"},
	}
	reachable := func(sha string) (bool, error) {
		return sha != "ccc", nil // ccc unreachable
	}
	got := ResolveJoins(deliveries, quality, reachable)
	if len(got) != len(deliveries) {
		t.Fatalf("got %d results for %d delivery records — records were dropped", len(got), len(deliveries))
	}
	want := []MatchState{MatchMatched, MatchNoMatch, MatchCouldNotJoin}
	for i, w := range want {
		if got[i].State != w {
			t.Fatalf("result[%d].State = %q, want %q", i, got[i].State, w)
		}
	}
}

func TestKeysEqualFallsBackToStreamTaskID(t *testing.T) {
	a := JoinKey{StreamTaskID: "quality/11"}
	b := JoinKey{StreamTaskID: "quality/11"}
	got := ResolveJoin(a, []JoinKey{b}, nil)
	if got.State != MatchMatched {
		t.Fatalf("expected a stream/task-ID-only match, got %q", got.State)
	}
}
