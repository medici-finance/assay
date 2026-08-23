package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// stubMergedClosedBranches substitutes the PR-state lister for one test, so the
// dead-claim decay is exercised offline — the same injection pattern
// stubRemoteBranches uses for listRemoteBranches.
func stubMergedClosedBranches(t *testing.T, dead map[string]bool, err error) {
	t.Helper()
	prev := listMergedClosedBranches
	listMergedClosedBranches = func(string) (map[string]bool, error) { return dead, err }
	t.Cleanup(func() { listMergedClosedBranches = prev })
}

// TestDecayDeadClaimsDropsMergedAndClosed is the unit-level property: a branch
// whose PR merged or closed is a corpse and must be dropped before it becomes a
// claim; a branch with an OPEN PR — or no PR at all (a worker that pushed but has
// not opened one) — is a live claim and is kept.
func TestDecayDeadClaimsDropsMergedAndClosed(t *testing.T) {
	branches := []string{
		"main",
		"fix/issue-loop-01-live",   // OPEN PR — keep
		"fix/issue-loop-02-merged", // MERGED PR — drop
		"fix/issue-loop-03-closed", // CLOSED PR — drop
		"fix/issue-loop-04-nopr",   // no PR yet — keep
	}
	stubMergedClosedBranches(t, map[string]bool{
		"fix/issue-loop-02-merged": true,
		"fix/issue-loop-03-closed": true,
	}, nil)

	got := decayDeadClaims("/repo", branches)
	want := []string{"main", "fix/issue-loop-01-live", "fix/issue-loop-04-nopr"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decayDeadClaims = %v, want %v", got, want)
	}
}

// TestDecayDeadClaimsFailsToTheSuperset pins the fail direction: a PR-state read
// error must NOT drop any branch (that would risk dropping a live open-PR claim
// and double-dispatching). It falls back to the full open-branch set — the
// pre-decay behaviour — and says so on stderr.
func TestDecayDeadClaimsFailsToTheSuperset(t *testing.T) {
	branches := []string{"main", "fix/issue-loop-02-merged"}
	stubMergedClosedBranches(t, nil, errors.New("gh: not authenticated"))

	var got []string
	stderr := captureStderr(t, func() { got = decayDeadClaims("/repo", branches) })
	if !reflect.DeepEqual(got, branches) {
		t.Fatalf("failed decay dropped branches: got %v, want the full set %v", got, branches)
	}
	if !strings.Contains(stderr, "dead-claim decay unavailable") {
		t.Errorf("a failed decay must announce itself on stderr; got:\n%s", stderr)
	}
}

// TestResolveClaimsDecaysDeadClaims wires the decay through resolveClaims: the
// merged/closed-PR corpses must not appear as claims, while the live open-PR
// branch still does — and the read is still Known (ls-remote succeeded; the
// board is filtered, not a degraded superset).
func TestResolveClaimsDecaysDeadClaims(t *testing.T) {
	streams := []*Stream{mkStream("issue-loop", "active", "P0",
		Brief{Num: "01", Status: "todo"},
		Brief{Num: "02", Status: "todo"},
		Brief{Num: "03", Status: "todo"},
	)}
	stubRemoteBranches(t, []string{
		"fix/issue-loop-01-live",
		"fix/issue-loop-02-merged",
		"fix/issue-loop-03-closed",
	}, nil)
	stubMergedClosedBranches(t, map[string]bool{
		"fix/issue-loop-02-merged": true,
		"fix/issue-loop-03-closed": true,
	}, nil)

	claimed, src := resolveClaims("/repo", streams)
	if !src.Known {
		t.Error("ls-remote succeeded — the claim source must be Known even after decay")
	}
	if !claimed["issue-loop/01"] {
		t.Error("a live open-PR branch must still claim its brief")
	}
	if claimed["issue-loop/02"] {
		t.Error("a merged-PR corpse must not claim (dead-claim decay)")
	}
	if claimed["issue-loop/03"] {
		t.Error("a closed-PR corpse must not claim (dead-claim decay)")
	}
}

// TestDeadClaimsNoLongerSuppressStream is the board-level regression for the live
// bug: a stream whose entire perStreamCap budget is spent on merged/closed-PR
// corpses shows ZERO rows before decay (its real backlog is silently held), and
// its rows come back once the corpses are decayed. The contrast case proves the
// suppression is real — four LIVE claims at perStreamCap=4 still hold the stream
// to zero, so the decay is not simply disabling the cap.
func TestDeadClaimsNoLongerSuppressStream(t *testing.T) {
	newStream := func() *Stream {
		return mkStream("issue-loop", "active", "P0",
			Brief{Num: "10", Title: "Ten", Status: "todo"},
			Brief{Num: "11", Title: "Eleven", Status: "todo"},
			Brief{Num: "12", Title: "Twelve", Status: "todo"},
			Brief{Num: "13", Title: "Thirteen", Status: "todo"},
			Brief{Num: "14", Title: "Fourteen", Status: "todo"},
			Brief{Num: "15", Title: "Fifteen", Status: "todo"},
		)
	}
	corpses := []string{
		"fix/issue-loop-01-corpse",
		"fix/issue-loop-02-corpse",
		"fix/issue-loop-03-corpse",
		"fix/issue-loop-04-corpse",
	}
	stubRemoteBranches(t, corpses, nil)

	// All four claiming branches are merged/closed corpses → decayed → cap restored.
	deadAll := map[string]bool{}
	for _, b := range corpses {
		deadAll[b] = true
	}
	stubMergedClosedBranches(t, deadAll, nil)
	claimed, src := resolveClaims("/repo", []*Stream{newStream()})
	nuDecayed := nextUp([]*Stream{newStream()}, ClaimView{Claimed: claimed, Source: src}, nil)
	if len(nuDecayed.Picks) == 0 {
		t.Fatalf("decaying dead claims must restore the stream's board rows; got 0 picks (HeldByStreamCap=%d)", nuDecayed.HeldByStreamCap)
	}
	if len(nuDecayed.Picks) != perStreamCap {
		t.Errorf("want %d picks after decay (full perStreamCap budget), got %d", perStreamCap, len(nuDecayed.Picks))
	}

	// Contrast: none of the four PRs are dead (all still OPEN) → all four remain
	// live claims → perStreamCap=4 is fully consumed → the stream is suppressed to
	// zero, exactly as it should be for genuine in-flight work.
	stubMergedClosedBranches(t, map[string]bool{}, nil)
	claimedLive, srcLive := resolveClaims("/repo", []*Stream{newStream()})
	nuLive := nextUp([]*Stream{newStream()}, ClaimView{Claimed: claimedLive, Source: srcLive}, nil)
	if len(nuLive.Picks) != 0 {
		t.Fatalf("four LIVE claims at perStreamCap=%d must still suppress the stream; got %d picks", perStreamCap, len(nuLive.Picks))
	}
	if nuLive.HeldByStreamCap != 6 {
		t.Errorf("HeldByStreamCap = %d, want 6 (all eligible briefs held by the consumed cap)", nuLive.HeldByStreamCap)
	}
}
