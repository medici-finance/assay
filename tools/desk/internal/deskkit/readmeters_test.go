package deskkit

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestReadMetersAgreesWithGate drives each budget tier to its limit on a ledger fixture and
// asserts the read-only ReadMeters says what the enforcing gate does, one below the limit and
// at it. A reading that reports room the gate refuses (or the reverse) is the defect class
// this test closes; so is a read that only agrees because it never reached the tier.
//
// regression: F-landing-capacity
func TestReadMetersAgreesWithGate(t *testing.T) {
	type tier struct {
		name   string
		limit  int
		target MeterTarget
		seed   func(i int) Entry
		gate   func(now time.Time) error
	}
	tiers := []tier{
		{
			// Spread over ten PRs so no per-PR bucket fills before the repo does; the
			// target PR has none of them.
			name: "per-repo", limit: RateLimitPerRepoPerHour,
			target: MeterTarget{Gate: GateWriteAt, Tool: "deskpost", Repo: testRepo, PR: 999},
			seed: func(i int) Entry {
				return Entry{Repo: testRepo, PR: iptr(i%10 + 1), Tool: "deskpost", Verb: "comment"}
			},
			gate: func(now time.Time) error { return AllowWriteAt("deskpost", testRepo, 999, now) },
		},
		{
			name: "repo-wide", limit: RateLimitPerPRPerHour,
			target: MeterTarget{Gate: GateRepoWide, Tool: "deskfile", Repo: testRepo},
			seed: func(i int) Entry {
				return Entry{Repo: testRepo, PR: iptr(i + 1), Tool: "deskfile", Verb: "new"}
			},
			gate: func(now time.Time) error { return AllowWriteRepoWideAt("deskfile", testRepo, now) },
		},
		{
			name: "new-PR", limit: RateLimitPerPRPerHour,
			target: MeterTarget{Gate: GatePRCreate, Tool: "deskpr", Repo: testRepo},
			seed: func(i int) Entry {
				return Entry{Repo: testRepo, PR: iptr(i + 1), Tool: "deskpr", Verb: "create"}
			},
			gate: func(now time.Time) error { return AllowPRCreateAt(testRepo, now) },
		},
		{
			name: "tool-wide", limit: RateLimitPerPRPerHour,
			target: MeterTarget{Gate: GateWriteAt, Tool: "deskpost"},
			seed: func(i int) Entry {
				return Entry{Tool: "deskpost", Verb: "comment"}
			},
			gate: func(now time.Time) error { return AllowWriteAt("deskpost", "", 0, now) },
		},
	}
	for _, tc := range tiers {
		t.Run(tc.name, func(t *testing.T) {
			dir := setup(t)
			for i := 0; i < tc.limit-1; i++ {
				appendEntry(t, dir, tc.seed(i))
			}
			now := time.Now().Add(time.Second)
			agree(t, tc.name, tc.limit, tc.target, tc.gate(now), now, 1)
			appendEntry(t, dir, tc.seed(tc.limit-1))
			now = time.Now().Add(time.Second)
			gateErr := tc.gate(now)
			if !IsRateLimited(gateErr) || !strings.Contains(gateErr.Error(), "hit its "+tc.name+" write budget") {
				t.Fatalf("at the limit the gate must refuse on the %s tier, got %v", tc.name, gateErr)
			}
			agree(t, tc.name, tc.limit, tc.target, gateErr, now, 0)
		})
	}
}

// agree asserts ReadMeters at `now` matches the gate's verdict `gateErr` and that the named
// tier reads `remaining` against `limit`.
func agree(t *testing.T, name string, limit int, target MeterTarget, gateErr error, now time.Time, remaining int) {
	t.Helper()
	snap, err := ReadMeters(target, now)
	if err != nil {
		t.Fatalf("ReadMeters(%s) = %v, want a reading the gate agrees with (gate: %v)", name, err, gateErr)
	}
	if snap.Admits != (gateErr == nil) {
		t.Fatalf("ReadMeters(%s).Admits = %v but the gate returned %v", name, snap.Admits, gateErr)
	}
	var got *MeterReading
	for i := range snap.Meters {
		if snap.Meters[i].Tier == name {
			got = &snap.Meters[i]
		}
	}
	if got == nil {
		t.Fatalf("ReadMeters(%s) has no %s meter: %+v", name, name, snap.Meters)
	}
	wantState := MeterAvailable
	if remaining == 0 {
		wantState = MeterExhausted
	}
	if got.Limit != limit || got.Remaining != remaining || got.State != wantState {
		t.Fatalf("%s meter = %s, want limit %d, %d remaining, %s",
			name, fmt.Sprintf("%+v", *got), limit, remaining, wantState)
	}
	if snap.Remaining != remaining {
		t.Fatalf("ReadMeters(%s).Remaining = %d, want %d", name, snap.Remaining, remaining)
	}
}
