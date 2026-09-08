package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// reconcile_test.go (cmd) — the reconciliation STEP: dry-run suppression, the terminal-releases
// / held-does-not-release split, the blind keep-the-run path, and the eligible pass-through to
// the liveness step. Every read is the offline fixture source, so no forge and no git.

func fixtureClaim(key string, elig *eligibilityFixture) claimRecord {
	return claimRecord{
		Key: key, Item: "s/" + key, Owner: "owner", Repo: "medici-finance/assay",
		Branch: "feat/" + key, PR: 1, Tier: "cheap", State: "dispatched",
		DispatchedAt: "2026-09-02T11:50:00Z", Eligibility: elig,
	}
}

func mustNow(t *testing.T) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, "2026-09-02T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// TestReconcile_TerminalArmsThenReleases proves a TERMINAL verdict (merged PR) arms the per-run
// stop BEFORE it releases the claim — the same order property the liveness reclaim pins.
func TestReconcile_TerminalArmsThenReleases(t *testing.T) {
	var order []string
	arm := func(c claimRecord, reason string) error {
		if reason == "" {
			t.Error("arm called with empty reason")
		}
		order = append(order, "arm:"+c.Key)
		return nil
	}
	reclaim := func(c claimRecord) error { order = append(order, "release:"+c.Key); return nil }

	claims := []claimRecord{fixtureClaim("merged-01", &eligibilityFixture{PRState: "merged"})}
	var out bytes.Buffer
	eligible, results, anyBlind, err := reconcile(claims, fixtureEligibilitySource(), mustNow(t), false, reclaim, arm, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if anyBlind {
		t.Fatal("a merged PR is not blind")
	}
	if len(eligible) != 0 {
		t.Fatalf("a terminal claim must be excluded from the liveness step, got %d eligible", len(eligible))
	}
	if len(order) != 2 || order[0] != "arm:merged-01" || order[1] != "release:merged-01" {
		t.Fatalf("arm must precede release for a terminal reconcile, got %v", order)
	}
	if len(results) != 1 || !results[0].Verdict.Terminal() || results[0].Action != "STOP+RELEASE" {
		t.Fatalf("unexpected results: %+v", results)
	}
	if !strings.Contains(out.String(), "INELIGIBLE(pr-merged)") || !strings.Contains(out.String(), "action=STOP+RELEASE") {
		t.Fatalf("output missing terminal line: %q", out.String())
	}
}

// TestReconcile_HeldArmsButNeverReleases proves a HELD verdict (needs-decision label) arms the
// stop but NEVER releases the claim — releasing would let a fresh worker re-dispatch into the
// held state, the exact regression row 5 of the Verify table guards.
func TestReconcile_HeldArmsButNeverReleases(t *testing.T) {
	armed := false
	arm := func(claimRecord, string) error { armed = true; return nil }
	reclaim := func(claimRecord) error { t.Fatal("a HELD verdict must NEVER release the claim"); return nil }

	claims := []claimRecord{fixtureClaim("held-01", &eligibilityFixture{PRLabels: []string{"needs-decision"}})}
	var out bytes.Buffer
	_, results, _, err := reconcile(claims, fixtureEligibilitySource(), mustNow(t), false, reclaim, arm, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !armed {
		t.Fatal("a HELD verdict must arm the per-run stop")
	}
	if len(results) != 1 || !results[0].Verdict.Held() || results[0].Action != "STOP" {
		t.Fatalf("unexpected results: %+v", results)
	}
	if strings.Contains(out.String(), "RELEASE") {
		t.Fatalf("a HELD line must never mention RELEASE: %q", out.String())
	}
}

// TestReconcile_DryRunActsOnNothing: under --dry-run neither arm nor release runs, for a
// terminal verdict.
func TestReconcile_DryRunActsOnNothing(t *testing.T) {
	armed, released := false, false
	arm := func(claimRecord, string) error { armed = true; return nil }
	reclaim := func(claimRecord) error { released = true; return nil }

	claims := []claimRecord{fixtureClaim("merged-01", &eligibilityFixture{PRState: "merged"})}
	var out bytes.Buffer
	if _, _, _, err := reconcile(claims, fixtureEligibilitySource(), mustNow(t), true, reclaim, arm, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if armed || released {
		t.Fatalf("--dry-run must act on nothing (armed=%v released=%v)", armed, released)
	}
	if !strings.Contains(out.String(), "INELIGIBLE(pr-merged)") {
		t.Fatalf("dry-run still prints the classification: %q", out.String())
	}
}

// TestReconcile_BlindKeepsRunNeverActs: a could-not-check reconcile keeps the run (excluded
// from eligible so the liveness step does not touch it either), never acts, and sets anyBlind.
func TestReconcile_BlindKeepsRunNeverActs(t *testing.T) {
	arm := func(claimRecord, string) error { t.Fatal("a BLIND reconcile must never arm"); return nil }
	reclaim := func(claimRecord) error { t.Fatal("a BLIND reconcile must never release"); return nil }

	claims := []claimRecord{fixtureClaim("blind-01", &eligibilityFixture{PRUnreadable: true})}
	var out bytes.Buffer
	eligible, results, anyBlind, err := reconcile(claims, fixtureEligibilitySource(), mustNow(t), false, reclaim, arm, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !anyBlind {
		t.Fatal("expected anyBlind=true")
	}
	if len(eligible) != 0 {
		t.Fatal("a blind claim must be kept out of the liveness step (retry next tick)")
	}
	if len(results) != 1 || !results[0].Blind || results[0].Source != "pr" {
		t.Fatalf("unexpected results: %+v", results)
	}
	got := out.String()
	if !strings.Contains(got, "BLIND(pr)") {
		t.Fatalf("output missing BLIND(pr): %q", got)
	}
	if strings.Contains(got, "INELIGIBLE") || strings.Contains(got, "STOP") {
		t.Fatalf("a BLIND line must not read as a verdict: %q", got)
	}
}

// TestReconcile_EligiblePassesThrough: a claim with no eligibility block, and one that reads
// eligible, both reach the liveness step untouched.
func TestReconcile_EligiblePassesThrough(t *testing.T) {
	arm := func(claimRecord, string) error { t.Fatal("an eligible claim must not be acted on"); return nil }
	reclaim := func(claimRecord) error { t.Fatal("an eligible claim must not be released"); return nil }

	claims := []claimRecord{
		fixtureClaim("nofix-01", nil),
		fixtureClaim("live-01", &eligibilityFixture{PRState: "open", BoardStatus: "in-progress"}),
	}
	var out bytes.Buffer
	eligible, results, anyBlind, err := reconcile(claims, fixtureEligibilitySource(), mustNow(t), false, reclaim, arm, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if anyBlind {
		t.Fatal("neither claim is blind")
	}
	if len(eligible) != 2 {
		t.Fatalf("both eligible claims must pass to the liveness step, got %d", len(eligible))
	}
	if len(results) != 0 {
		t.Fatalf("an eligible claim prints no reconcile line, got %+v", results)
	}
	if out.Len() != 0 {
		t.Fatalf("reconcile must print nothing for eligible claims: %q", out.String())
	}
}

// TestReconcile_AllBinaryScenarios re-runs the exact five reconcile fixtures the Verify table's
// rows 2-6 exercise, in-process (no built binary), as a fast regression net.
func TestReconcile_AllBinaryScenarios(t *testing.T) {
	noopArm := func(claimRecord, string) error { return nil }
	noopReclaim := func(claimRecord) error { return nil }

	cases := []struct {
		name           string
		file           string
		wantContains   []string
		wantNotContain []string
		wantBlind      bool
	}{
		{"pr-merged", "testdata/pr-merged.json", []string{"INELIGIBLE(pr-merged)", "action=STOP+RELEASE"}, nil, false},
		{"pr-closed", "testdata/pr-closed.json", []string{"INELIGIBLE(pr-closed)", "action=STOP+RELEASE"}, nil, false},
		{"brief-flipped", "testdata/brief-flipped.json", []string{"INELIGIBLE(board-row-implemented)"}, nil, false},
		{"needs-decision", "testdata/needs-decision.json", []string{"INELIGIBLE(needs-decision)", "action=STOP"}, []string{"RELEASE"}, false},
		{"forge-unreachable", "testdata/forge-unreachable.json", []string{"BLIND(pr)"}, []string{"INELIGIBLE", "STOP"}, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			claims, err := loadClaimsFixture(tc.file)
			if err != nil {
				t.Fatalf("loadClaimsFixture: %v", err)
			}
			var out bytes.Buffer
			_, _, anyBlind, rerr := reconcile(claims, fixtureEligibilitySource(), mustNow(t), true, noopReclaim, noopArm, &out)
			if rerr != nil {
				t.Fatalf("unexpected error: %v", rerr)
			}
			if anyBlind != tc.wantBlind {
				t.Fatalf("anyBlind = %v, want %v", anyBlind, tc.wantBlind)
			}
			got := out.String()
			for _, w := range tc.wantContains {
				if !strings.Contains(got, w) {
					t.Errorf("output missing %q:\n%s", w, got)
				}
			}
			for _, nw := range tc.wantNotContain {
				if strings.Contains(got, nw) {
					t.Errorf("output must not contain %q:\n%s", nw, got)
				}
			}
		})
	}
}

// TestReconcile_ClaimAxisVerdicts covers the two claim-axis reasons the binary fixtures do not
// (they focus on the PR/board axes): a RELEASED claim is Terminal (the ref is already gone, so
// the delete is a no-op), a REASSIGNED claim is Held (the ref belongs to a new live holder now).
func TestReconcile_ClaimAxisVerdicts(t *testing.T) {
	cases := []struct {
		name     string
		elig     *eligibilityFixture
		reason   string
		terminal bool
	}{
		{"released", &eligibilityFixture{ClaimReleased: true}, "claim-released", true},
		{"reassigned", &eligibilityFixture{ClaimHolder: "someone-else"}, "claim-reassigned", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			v, err := fixtureEligibilitySource()(fixtureClaim("c-01", tc.elig))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.Reason != tc.reason || v.Terminal() != tc.terminal {
				t.Fatalf("expected reason=%s terminal=%v, got %+v", tc.reason, tc.terminal, v)
			}
		})
	}
}

// TestReconcile_ClaimReassignedNeverReleases is the pin the review asked for: a claim-reassigned
// verdict must STOP the run (arm the per-run stop) but must NEVER call reclaim — deleting the
// claim ref by key would delete the NEW live holder's ref and re-free an item they are working
// (a double-dispatch). The `reclaim` seam here is a t.Fatal, so any regression that reclassifies
// claim-reassigned back to Terminal (release=true) genuinely fails this test.
func TestReconcile_ClaimReassignedNeverReleases(t *testing.T) {
	armed := false
	arm := func(claimRecord, string) error { armed = true; return nil }
	reclaim := func(claimRecord) error {
		t.Fatal("a claim-reassigned verdict must NEVER release/delete the ref — it belongs to the new holder")
		return nil
	}
	claims := []claimRecord{fixtureClaim("reassigned-01", &eligibilityFixture{ClaimHolder: "someone-else"})}
	var out bytes.Buffer
	eligible, results, anyBlind, err := reconcile(claims, fixtureEligibilitySource(), mustNow(t), false, reclaim, arm, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if anyBlind || len(eligible) != 0 {
		t.Fatalf("a reassigned claim is stopped, not eligible and not blind (eligible=%d blind=%v)", len(eligible), anyBlind)
	}
	if !armed {
		t.Fatal("a claim-reassigned verdict must arm the per-run stop")
	}
	if len(results) != 1 || !results[0].Verdict.Held() || results[0].Action != "STOP" {
		t.Fatalf("expected a single Held STOP result, got %+v", results)
	}
	if strings.Contains(out.String(), "RELEASE") {
		t.Fatalf("a claim-reassigned line must never mention RELEASE: %q", out.String())
	}
}
