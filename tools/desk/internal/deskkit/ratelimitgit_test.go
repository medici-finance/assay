package deskkit

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Historical fetch records must be noncharging through either reader, without
// removing audit rows from the breakers or exempting actual/ambiguous writes.
func TestGitReadBudget(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		t.Run(fmt.Sprint("fallback=", fallback), func(t *testing.T) {
			dir := setup(t)
			if fallback {
				old := boundedReadMaxLines
				boundedReadMaxLines = 0
				t.Cleanup(func() { boundedReadMaxLines = old })
			}
			now := time.Now().UTC()
			for i := 0; i < RateLimitPerRepoPerHour+1; i++ {
				result := ResultOK
				if i%2 != 0 {
					result = ResultUnverifiable
				}
				appendEntry(t, dir, Entry{Tool: "deskgit.test", Verb: "fetch", Repo: testRepo, Result: result})
			}
			mine, err := pointsFor("deskgit", now)
			if err != nil {
				t.Fatal(err)
			}
			for name, gate := range map[string]func() error{
				"per-target": func() error { return checkPRBudget("deskgit", testRepo, 0, now, mine) },
				"per-repo":   func() error { return checkRepoBudget("deskgit", testRepo, now, mine) },
				"repo-wide":  func() error { return checkRepoWideBudget("deskgit", testRepo, now, mine) },
				"tool-wide":  func() error { return checkToolBudget("deskgit", now, mine) },
			} {
				if err := gate(); err != nil {
					t.Fatalf("%s charged fetch-only history: %v", name, err)
				}
			}
			for i := 0; i < RateLimitPerPRPerHour-1; i++ {
				appendEntry(t, dir, Entry{Tool: "deskgit", Verb: "push", Repo: testRepo, Result: ResultOK})
			}
			if err := AllowWriteAt("deskgit", testRepo, 0, now); err != nil {
				t.Fatalf("fetches consumed the remaining push slot: %v", err)
			}
			appendEntry(t, dir, Entry{Tool: "deskgit", Verb: "push", Repo: testRepo, Result: ResultUnverifiable})
			err = AllowWriteAt("deskgit", testRepo, 0, now)
			if meterOf(err) != "pr-budget" || !strings.Contains(err.Error(), fmt.Sprintf("%d charged writes", RateLimitPerPRPerHour)) {
				t.Fatalf("actual pushes must hit the unchanged cap without fetch inflation: %v", err)
			}
		})
	}
}

func TestGitBudgetUnknowns(t *testing.T) {
	for _, tc := range []struct{ tool, verb, result string }{
		{"deskgit", "push", ResultOK}, {"deskgit", "push", ResultUnverifiable},
		{"deskgit", "", ResultOK}, {"deskgit", "future-verb", ResultOK},
		{"deskgit", "fetch", "future-result"}, {"deskpost", "fetch", ResultOK},
	} {
		t.Run(tc.tool+"/"+tc.verb+"/"+tc.result, func(t *testing.T) {
			dir := setup(t)
			for i := 0; i < RateLimitPerPRPerHour; i++ {
				appendEntry(t, dir, Entry{Tool: tc.tool, Verb: tc.verb, Repo: testRepo, Result: tc.result})
			}
			if got := meterOf(AllowWrite(tc.tool, testRepo, 0)); got != "pr-budget" {
				t.Fatalf("unknown or write operation escaped the cap: %s", got)
			}
		})
	}
}

// Fetch rows still reset or extend the existing breaker exactly as before.
func TestGitReadBreakers(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		t.Run(fmt.Sprint("fallback=", fallback), func(t *testing.T) {
			dir := setup(t)
			if fallback {
				old := boundedReadMaxLines
				boundedReadMaxLines = 0
				t.Cleanup(func() { boundedReadMaxLines = old })
			}
			for i := 0; i < BreakerTrip-1; i++ {
				appendEntry(t, dir, Entry{Tool: "deskgit", Verb: "push", Repo: testRepo, Result: ResultRefused})
			}
			appendEntry(t, dir, Entry{Tool: "deskgit", Verb: "fetch", Repo: testRepo, Result: ResultOK})
			appendEntry(t, dir, Entry{Tool: "deskgit", Verb: "push", Repo: testRepo, Result: ResultRefused})
			if err := AllowWrite("deskgit", testRepo, 0); err != nil {
				t.Fatalf("fetch progress no longer resets the existing refusal run: %v", err)
			}
			for i := 0; i < BreakerTrip-1; i++ {
				appendEntry(t, dir, Entry{Tool: "deskgit", Verb: "fetch", Repo: testRepo, Result: ResultRefused})
			}
			if got := meterOf(AllowWrite("deskgit", testRepo, 0)); got != "breaker" {
				t.Fatalf("fetch rows disappeared from the unchanged breaker: %s", got)
			}
		})
	}
}
