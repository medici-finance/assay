package main

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// preparedNotLaunchedMark is the marker `show` and `list` append to a claim that was acquired
// (state=claimed) and never advanced to dispatched inside its grace window.
const preparedNotLaunchedMark = "PREPARED-NOT-LAUNCHED"

// regression: #2747
func TestShowReportsAClaimThatNeverProgressedAsPreparedNotLaunched(t *testing.T) {
	cases := []struct {
		name  string
		state string
		age   time.Duration
		want  bool
	}{
		{"claimed past the window", "claimed", 25 * time.Minute, true},
		{"claimed exactly at the window", "claimed", 20 * time.Minute, true},
		{"claimed inside the window", "claimed", 5 * time.Minute, false},
		{"dispatched is launched, however old", "dispatched", 300 * time.Minute, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newStore()
			f.seedClaim("at--issue-9", "desk-1", c.state, "", c.age)
			run, so, _ := harness(t, f)
			if rc := run("show", "at--issue-9", "--repo", "medici-finance/assay"); rc != exitOK {
				t.Fatalf("show rc = %d, want 0", rc)
			}
			if got := strings.Contains(so.String(), preparedNotLaunchedMark); got != c.want {
				t.Fatalf("show marks prepared-not-launched = %v, want %v:\n%s", got, c.want, so.String())
			}
			// The annotation rides AFTER the fields the readers parse, so they extract the same
			// values with and without it.
			line := so.String()
			if m := regexp.MustCompile(`state=([A-Za-z]+)`).FindStringSubmatch(line); m == nil || m[1] != c.state {
				t.Errorf("state= regex on %q -> %v, want %q", line, m, c.state)
			}
			if m := regexp.MustCompile(`age=(\d+)m`).FindStringSubmatch(line); m == nil {
				t.Errorf("age= regex found nothing in %q", line)
			}
		})
	}
}

// regression: #2747
func TestListReportsOnlyTheNeverProgressedClaimAsPreparedNotLaunched(t *testing.T) {
	f := newStore()
	f.seedClaim("at--issue-1", "desk-1", "claimed", "", 40*time.Minute)
	f.seedClaim("at--issue-2", "desk-1", "dispatched", "feat/x", 40*time.Minute)
	f.seedClaim("at--issue-3", "desk-1", "claimed", "", 2*time.Minute)
	run, so, _ := harness(t, f)
	if rc := run("list", "--repo", "medici-finance/assay"); rc != exitOK {
		t.Fatalf("list rc = %d, want 0", rc)
	}
	marked := map[string]bool{}
	for _, line := range strings.Split(so.String(), "\n") {
		for _, id := range []string{"at--issue-1", "at--issue-2", "at--issue-3"} {
			if strings.Contains(line, "HELD "+id+" ") {
				marked[id] = strings.Contains(line, preparedNotLaunchedMark)
			}
		}
	}
	want := map[string]bool{"at--issue-1": true, "at--issue-2": false, "at--issue-3": false}
	for id, w := range want {
		if got, seen := marked[id]; !seen || got != w {
			t.Errorf("%s: listed=%v marked=%v, want marked=%v\n%s", id, seen, got, w, so.String())
		}
	}
}

// The grace window is the claim tool's own CLAIMED_TTL_MIN, so the marker and the reclaim in
// `acquire` always flip at the same age.
// regression: #2747
func TestPreparedNotLaunchedWindowFollowsClaimedTTLEnv(t *testing.T) {
	t.Setenv("CLAIMED_TTL_MIN", "3")
	f := newStore()
	f.seedClaim("at--issue-9", "desk-1", "claimed", "", 4*time.Minute)
	run, so, _ := harness(t, f)
	if rc := run("show", "at--issue-9", "--repo", "medici-finance/assay"); rc != exitOK {
		t.Fatalf("show rc = %d, want 0", rc)
	}
	if !strings.Contains(so.String(), preparedNotLaunchedMark) {
		t.Fatalf("a 4m claim past a 3m window is not marked:\n%s", so.String())
	}
}

// The verify sketch end to end: prepare a dispatch (acquire) and never launch it; once the
// grace window has passed `show` says so and a second dispatcher takes the claim.
// regression: #2747
func TestUnlaunchedDispatchIsReportedThenTakenByASecondDispatcher(t *testing.T) {
	f := newStore()
	run, so, _ := harness(t, f)
	if rc := run("acquire", "at--issue-9", "--repo", "medici-finance/assay", "--owner", "first"); rc != exitOK {
		t.Fatalf("first acquire rc = %d, want 0", rc)
	}
	// Fresh: a live holder, not yet reported as stalled.
	if rc := run("show", "at--issue-9", "--repo", "medici-finance/assay"); rc != exitOK || strings.Contains(so.String(), preparedNotLaunchedMark) {
		t.Fatalf("fresh claim: rc=%d, marked=%v\n%s", rc, strings.Contains(so.String(), preparedNotLaunchedMark), so.String())
	}
	if rc := run("acquire", "at--issue-9", "--repo", "medici-finance/assay", "--owner", "second"); rc != exitRefused {
		t.Fatalf("second acquire inside the window rc = %d, want 5", rc)
	}
	// Advance the clock past the window: the store dates the claim in the past.
	c := f.claims["at--issue-9"]
	c.date = f.now.Add(-25 * time.Minute).Format(time.RFC3339)
	f.claims["at--issue-9"] = c
	if rc := run("show", "at--issue-9", "--repo", "medici-finance/assay"); rc != exitOK || !strings.Contains(so.String(), preparedNotLaunchedMark) {
		t.Fatalf("stalled claim: rc=%d, not reported:\n%s", rc, so.String())
	}
	if rc := run("acquire", "at--issue-9", "--repo", "medici-finance/assay", "--owner", "second"); rc != exitOK {
		t.Fatalf("second acquire past the window rc = %d, want 0 (the stalled claim is stealable); out=%s", rc, so.String())
	}
	if !strings.Contains(f.claims["at--issue-9"].msg, "owner=second") {
		t.Errorf("the claim did not move to the second dispatcher: %s", f.claims["at--issue-9"].msg)
	}
}
