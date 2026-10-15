package main

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestClaimReadSessionSharesObservationAndRefreshesNextRun(t *testing.T) {
	prevBranches, prevDead := listRemoteBranches, listMergedClosedBranches
	t.Cleanup(func() { listRemoteBranches, listMergedClosedBranches = prevBranches, prevDead })
	branchCalls, decayCalls := 0, 0
	branches := []string{"fix/demo-01-live", "fix/demo-02-live"}
	listRemoteBranches = func(string) ([]string, error) {
		branchCalls++
		return branches, nil
	}
	listMergedClosedBranches = func(string) (map[string]bool, error) {
		decayCalls++
		return nil, nil
	}
	first := []*Stream{mkStream("demo", "active", "P0", Brief{Num: "01", Status: "todo"})}
	full := []*Stream{mkStream("demo", "active", "P0", Brief{Num: "01", Status: "todo"}, Brief{Num: "02", Status: "todo"})}
	func() {
		end := beginGitReadSession()
		defer end()
		got, src := resolveClaims("/first-root", first)
		if !src.Known || !got["demo/01"] {
			t.Fatalf("first claim read = %v, %+v", got, src)
		}
		got["injected/99"] = true
		got, src = resolveClaims("/first-root", full)
		if !src.Known || !got["demo/02"] || got["injected/99"] {
			t.Fatalf("a fresh map must use the current stream universe: %v, %+v", got, src)
		}
		if branchCalls != 1 || decayCalls != 1 {
			t.Fatalf("same root repeated external reads: branches=%d decay=%d", branchCalls, decayCalls)
		}
		resolveClaims("/second-root", full)
		if branchCalls != 2 || decayCalls != 2 {
			t.Fatalf("different roots shared a read: branches=%d decay=%d", branchCalls, decayCalls)
		}
	}()
	branches = nil
	func() {
		end := beginGitReadSession()
		defer end()
		got, src := resolveClaims("/first-root", full)
		if !src.Known || len(got) != 0 || branchCalls != 3 || decayCalls != 2 {
			t.Fatalf("new run did not re-read changed claims: %v, %+v, branches=%d decay=%d", got, src, branchCalls, decayCalls)
		}
	}()
}

func TestClaimReadSessionKeepsUnknownAndDecayFailure(t *testing.T) {
	prevBranches, prevDead := listRemoteBranches, listMergedClosedBranches
	t.Cleanup(func() { listRemoteBranches, listMergedClosedBranches = prevBranches, prevDead })
	streams := []*Stream{mkStream("demo", "active", "P0", Brief{Num: "01", Status: "todo"})}
	reads := 0
	listRemoteBranches = func(string) ([]string, error) {
		reads++
		return nil, errors.New("fixture remote unavailable")
	}
	func() {
		end := beginGitReadSession()
		defer end()
		for i := 0; i < 2; i++ {
			got, src := resolveClaims("/root", streams)
			if src.Known || len(got) != 0 || !strings.Contains(src.Reason, "fixture remote unavailable") {
				t.Fatalf("unknown read became known-empty: %v, %+v", got, src)
			}
		}
		if reads != 1 {
			t.Fatalf("one run should have one observation, got %d", reads)
		}
	}()
	listRemoteBranches = func(string) ([]string, error) { return []string{"fix/demo-01-live"}, nil }
	listMergedClosedBranches = func(string) (map[string]bool, error) { return nil, errors.New("fixture decay unavailable") }
	func() {
		end := beginGitReadSession()
		defer end()
		for i := 0; i < 2; i++ {
			got, src := resolveClaims("/root", streams)
			if !src.Known || !got["demo/01"] || !strings.Contains(src.DecayReason, "fixture decay unavailable") {
				t.Fatalf("failed decay lost its claim superset or reason: %v, %+v", got, src)
			}
		}
	}()
}

func TestClaimReadSessionCoalescesConcurrentConsumers(t *testing.T) {
	prevBranches, prevDead := listRemoteBranches, listMergedClosedBranches
	t.Cleanup(func() { listRemoteBranches, listMergedClosedBranches = prevBranches, prevDead })
	var reads, decays atomic.Int32
	listRemoteBranches = func(string) ([]string, error) {
		reads.Add(1)
		return []string{"fix/demo-01-live"}, nil
	}
	listMergedClosedBranches = func(string) (map[string]bool, error) {
		decays.Add(1)
		return nil, nil
	}
	end := beginGitReadSession()
	defer end()
	streams := []*Stream{mkStream("demo", "active", "P0", Brief{Num: "01", Status: "todo"})}
	var wg sync.WaitGroup
	results := make([]bool, 24)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got, src := resolveClaims("/root", streams)
			results[i] = src.Known && got["demo/01"]
		}(i)
	}
	wg.Wait()
	for i, ok := range results {
		if !ok {
			t.Errorf("consumer %d lost the claim", i)
		}
	}
	if reads.Load() != 1 || decays.Load() != 1 {
		t.Fatalf("concurrent reads not coalesced: branches=%d decay=%d", reads.Load(), decays.Load())
	}
}
