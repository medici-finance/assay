package main

import (
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/medici-finance/assay/gitcore"
	"os/exec"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const firstReviewRun = finishClaimKey + "@20260101T010101Z.000000000001"
const nextReviewRun = finishClaimKey + "@20260101T010101Z.000000000002"

type reviewLeaseFixture struct {
	deskkit.ClaimStore
	record deskkit.ClaimStoreRecord
	next   *deskkit.ClaimStoreRecord
}

func (s *reviewLeaseFixture) Read(k string) (deskkit.ClaimStoreRecord, deskkit.ClaimReadStatus) {
	if s.record.Version == "" {
		return s.record, deskkit.ClaimReadFree
	}
	return s.record, deskkit.ClaimReadHeld
}

func (s *reviewLeaseFixture) Remove(k string) (deskkit.ClaimWriteOutcome, bool) {
	s.record = deskkit.ClaimStoreRecord{}
	return deskkit.ClaimWriteApplied, true
}

func (s *reviewLeaseFixture) RemoveFrom(k, old string) deskkit.ClaimWriteOutcome {
	if s.next != nil {
		s.record = *s.next
	}
	if s.record.Version != old {
		return deskkit.ClaimWriteRejected
	}
	s.record = deskkit.ClaimStoreRecord{}
	return deskkit.ClaimWriteApplied
}

// regression: #7432
func TestDelayedFinishKeepsSameSessionSuccessor(t *testing.T) {
	setupFinish(t)
	t.Setenv("DESK_SESSION", "reviews")
	const prefix = "dispatch-claim " + finishClaimKey + " owner=reviews state=dispatched branch=- review-run="
	s := &reviewLeaseFixture{record: deskkit.ClaimStoreRecord{Msg: prefix + nextReviewRun, Version: "successor"}}
	deskkit.SetForgeRefClaimStoreOpener(func(string) (deskkit.ClaimStore, error) { return s, nil })
	t.Cleanup(func() { deskkit.SetForgeRefClaimStoreOpener(nil) })
	if state, err := releaseReviewClaim("example-org", "tracker", finishClaimKey); err == nil || s.record.Version != "successor" {
		t.Fatalf("delayed finish reported %q, %v; successor now at %q", state, err, s.record.Version)
	}
}

func TestFinishChecksCheckoutLeaseBeforeExactRemoval(t *testing.T) {
	const msg = "dispatch-claim " + finishClaimKey + " owner=reviews state=dispatched branch=-"
	for _, tc := range []struct {
		name, message, checkoutRun, checkoutOwner, version string
		race                                               bool
		want                                               int
	}{
		{"original", msg + " review-run=" + firstReviewRun, firstReviewRun, "reviews", "one", false, 0},
		{"successor during delete", msg + " review-run=" + firstReviewRun, firstReviewRun, "reviews", "one", true, deskkit.ExitRefused},
		{"different holder", msg + " review-run=" + firstReviewRun, firstReviewRun, "someone-else", "one", false, deskkit.ExitRefused},
		{"older checkout", msg + " review-run=" + firstReviewRun, "", "reviews", "one", false, deskkit.ExitRefused},
		{"holder unknown", msg + " review-run=" + firstReviewRun, firstReviewRun, "unknown", "one", false, deskkit.ExitRefused},
		{"older claim", msg, firstReviewRun, "reviews", "one", false, deskkit.ExitRefused},
		{"duplicate owner", msg + " owner=other review-run=" + firstReviewRun, firstReviewRun, "reviews", "one", false, deskkit.ExitRefused},
		{"note cannot supply proof", msg + " note=anything review-run=" + firstReviewRun, firstReviewRun, "reviews", "one", false, deskkit.ExitRefused},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupFinish(t)
			checkoutReviewIdentity = func(_, _ string) (string, string) { return tc.checkoutOwner, tc.checkoutRun }
			s := &reviewLeaseFixture{record: deskkit.ClaimStoreRecord{Msg: tc.message, Version: tc.version}}
			if tc.race {
				s.next = &deskkit.ClaimStoreRecord{Msg: msg + " review-run=" + nextReviewRun, Version: "successor"}
			}
			deskkit.SetForgeRefClaimStoreOpener(func(string) (deskkit.ClaimStore, error) { return s, nil })
			t.Cleanup(func() { deskkit.SetForgeRefClaimStoreOpener(nil) })
			_, err := releaseReviewClaim("example-org", "tracker", finishClaimKey)
			if code := deskkit.ExitCodeOf(err); code != tc.want {
				t.Fatalf("exit=%d, want %d: %v", code, tc.want, err)
			}
			if tc.want == 0 {
				if s.record.Version != "" {
					t.Fatal("claim still held")
				}
				return
			}
			if s.record.Version == "" || (tc.race && s.record.Version != "successor") {
				t.Fatalf("lost successor: %+v", s.record)
			}
		})
	}
}

// Exercise the legacy resolver's exact conditional delete against a disposable Git receiver.
// regression: #7432
func TestFinishPreservesReplacementOnForgeRef(t *testing.T) {
	setupFinish(t)
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", repo).CombinedOutput(); err != nil {
		t.Fatalf("init: %v: %s", err, out)
	}
	lease := &deskkit.ForgeClaimLease{Endpoint: gitcore.ListOpts{URL: repo}}
	var old plumbing.Hash
	put := func(run string) {
		t.Helper()
		msg := "dispatch-claim " + finishClaimKey + " owner=reviews state=dispatched branch=- review-run=" + run
		objects, hash, err := gitcore.MintClaimTag(finishClaimKey, msg, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		res, err := gitcore.PushRefUpdate(t.Context(), gitcore.RefUpdate{URL: repo, Ref: plumbing.ReferenceName(deskkit.DispatchClaimActiveRefsPrefix + finishClaimKey), Old: old, New: hash, Objects: objects})
		if err != nil || res != gitcore.RefUpdateApplied {
			t.Fatalf("place review claim: result=%v err=%v", res, err)
		}
		old = hash
	}
	put(firstReviewRun)
	// A review target has advertised refs even after its last dispatch claim goes away.
	if out, err := exec.Command("git", "-C", repo, "update-ref", "refs/tags/unrelated", old.String()).CombinedOutput(); err != nil {
		t.Fatalf("seed unrelated ref: %v: %s", err, out)
	}
	put(nextReviewRun)
	reviewerLease = func(deskkit.ForgeRepo) (finishLease, error) { return lease, nil }
	if _, err := releaseReviewClaim("example-org", "tracker", finishClaimKey); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("former reviewer: %v", err)
	}
	rec, status := lease.Read(finishClaimKey)
	if status != deskkit.ClaimReadHeld || rec.Version != old.String() {
		t.Fatalf("replacement gone: status=%v version=%q cause=%s", status, rec.Version, lease.TransportCause())
	}
	checkoutReviewIdentity = func(_, _ string) (string, string) { return "reviews", nextReviewRun }
	for _, want := range []string{finishClaimReleased, finishClaimAlreadyReleased} {
		state, err := releaseReviewClaim("example-org", "tracker", finishClaimKey)
		if err != nil || state != want {
			t.Fatalf("successor finish: got %q, %v; want %q", state, err, want)
		}
	}
}
