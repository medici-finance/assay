package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

func TestClaimPageUncertaintyKeepsHealthyClaims(t *testing.T) {
	now := mustNow(t)
	for _, mode := range []string{"unknown-owner", "future-stamp"} {
		t.Run(mode, func(t *testing.T) {
			refs := []*plumbing.Reference{}
			for i := 1; i <= 2; i++ {
				refs = append(refs, plumbing.NewHashReference(plumbing.ReferenceName(fmt.Sprintf("%sr--issue-%d", deskkit.DispatchClaimActiveRefsPrefix, i)), plumbing.NewHash(fmt.Sprintf("%040x", i))))
			}
			claims, _, err := claimPage(refs, "org/repo", now, 50, "", func(h plumbing.Hash) (gitcore.TagPayload, error) {
				owner, at := "session", now.Add(-time.Minute)
				key := "r--issue-2"
				if h == refs[0].Hash() {
					key = "r--issue-1"
					if mode == "unknown-owner" {
						owner = "unknown"
					} else {
						at = now.Add(time.Minute)
					}
				}
				return gitcore.TagPayload{Message: fmt.Sprintf("dispatch-claim %s owner=%s state=dispatched branch=topic", key, owner), When: at}, nil
			})
			if err != nil || len(claims) != 2 {
				t.Fatalf("uncertain claim blocked healthy page: len=%d err=%v", len(claims), err)
			}
			reads := 0
			src := func(c claimRecord) (loopengine.EligibilityVerdict, error) {
				reads++
				if c.Key != "r--issue-2" {
					t.Fatal("uncertain claim reached eligibility reads")
				}
				return loopengine.Eligible, nil
			}
			eligible, results, blind, err := reconcile(claims, src, now, false, func(claimRecord) error { t.Fatal("uncertain claim released"); return nil }, func(claimRecord, string) error { t.Fatal("uncertain claim stopped"); return nil }, io.Discard)
			if err != nil || !blind || reads != 1 || len(eligible) != 1 || len(results) != 1 || !results[0].Blind {
				t.Fatalf("page classification: eligible=%d results=%+v blind=%t reads=%d err=%v", len(eligible), results, blind, reads, err)
			}
		})
	}
}

func TestReclaimRefusalKeepsRemainingClaims(t *testing.T) {
	first := heartbeatDeadClaim()
	first.Ref = deskkit.DispatchClaimActiveRefsPrefix + first.Key
	first.Version = strings.Repeat("1", 40)
	reclaim := func(c claimRecord) error {
		return reclaimVersion(c, gitcore.ListOpts{}, func(context.Context, gitcore.RefUpdate) (gitcore.RefUpdateResult, error) {
			return gitcore.RefUpdateRejected, nil
		})
	}
	second := fixtureClaim("healthy", nil)
	first.Eligibility = &eligibilityFixture{PRState: "merged"}
	eligible, results, blind, err := reconcile([]claimRecord{first, second}, fixtureEligibilitySource(), mustNow(t), false, reclaim, noopArm, io.Discard)
	if err != nil || !blind || len(eligible) != 1 || len(results) != 1 || !results[0].Blind {
		t.Fatalf("CAS refusal aborted reconcile: eligible=%d results=%+v blind=%t err=%v", len(eligible), results, blind, err)
	}
	obs := func(c claimRecord) (resolvedObservation, error) {
		if c.Key == first.Key {
			return heartbeatDeadObs()(c)
		}
		return resolvedObservation{observed: true, obs: loopengine.Observation{At: mustNow(t)}}, nil
	}
	swept, blind, err := sweep([]claimRecord{first, second}, obs, loopengine.DefaultLivenessPolicy(), mustNow(t), false, reclaim, func(claimRecord) error { t.Fatal("unexpected timeout"); return nil }, noopArm, io.Discard)
	if err != nil || !blind || len(swept) != 2 || !swept[0].Blind || swept[1].Blind {
		t.Fatalf("CAS refusal aborted sweep: results=%+v blind=%t err=%v", swept, blind, err)
	}
}

func TestReconcileReleasedClaimSkipsDelete(t *testing.T) {
	c := fixtureClaim("released", &eligibilityFixture{ClaimReleased: true})
	calls := 0
	_, results, blind, err := reconcile([]claimRecord{c}, fixtureEligibilitySource(), mustNow(t), false, func(claimRecord) error { calls++; return fmt.Errorf("already absent") }, noopArm, io.Discard)
	if err != nil || blind || calls != 0 || len(results) != 1 || results[0].Action != "STOP" {
		t.Fatalf("absent release was not a no-op: calls=%d results=%+v blind=%t err=%v", calls, results, blind, err)
	}
}

func TestStatusInventoryUncertainty(t *testing.T) {
	for _, mode := range []string{"unknown", "future"} {
		c := fixtureClaim(mode, nil)
		if mode == "unknown" {
			c.Owner = "unknown"
		} else {
			c.DispatchedAt = mustNow(t).Add(time.Minute).Format(time.RFC3339)
		}
		snapshot, blind, err := buildSnapshot([]claimRecord{c}, func(claimRecord) (claimObs, error) {
			t.Fatal("uncertain claim reached liveness probe")
			return claimObs{}, nil
		}, fixtureResourceSource(nil), nil, false, loopengine.DefaultLivenessPolicy(), mustNow(t))
		if err != nil || !blind || len(snapshot.Claims) != 1 || snapshot.Claims[0].Liveness != "COULD-NOT-CHECK" {
			t.Fatalf("uncertain status: %+v blind=%t err=%v", snapshot, blind, err)
		}
	}
}

func TestPageSnapshotIncludesReconcileBlind(t *testing.T) {
	c := fixtureClaim("unknown", nil)
	snapshot := pageSnapshot(nil, []reconcileResult{{Claim: c, Blind: true, Source: "claim-owner-unknown", Action: "none"}}, loopengine.DefaultLivenessPolicy(), mustNow(t), false)
	if len(snapshot.Claims) != 1 || len(snapshot.Aggregates.BlindSources) != 1 || snapshot.Aggregates.ByLiveness["COULD-NOT-CHECK"] != 1 {
		t.Fatalf("reconcile uncertainty lost from run status: %+v", snapshot)
	}
}
