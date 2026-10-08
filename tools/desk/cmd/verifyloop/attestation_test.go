package main

import (
	"errors"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
	"testing"
)

func TestVerifierAdapterAdmissionBeforeFeederAndLanding(t *testing.T) {
	calls := 0
	v := &VerifyLoop{Feeder: func(loopengine.Item, loopengine.Tier, string) (loopengine.Result, error) {
		calls++
		return loopengine.Result{}, nil
	}, Attest: func(loopengine.Item, string, string) (deskkit.VerifierReceipt, error) {
		return deskkit.VerifierReceipt{}, errors.New("missing stamp")
	}}
	if _, err := v.Dispatch(fixtureItem(), loopengine.TierLocal); err == nil {
		t.Fatal("unadmitted feeder dispatched")
	}
	if calls != 0 {
		t.Fatal("feeder ran before admission")
	}
	sink := newRec()
	v.DurableSink = sink
	if err := v.Land(loopengine.Result{Item: fixtureItem(), Verdict: loopengine.VerdictPass}); err == nil {
		t.Fatal("fabricated PASS admitted")
	}
	if len(sink.evidence) != 0 {
		t.Fatal("fabricated evidence landed")
	}
}
func TestVerifierNativeAdmissionFailureRetainsRunBeforeSpawn(t *testing.T) {
	v := nativeLoop(t, "roundtrip")
	cleaned := false
	v.MakeWorktree = func(loopengine.Item) (string, func(), error) { return t.TempDir(), func() { cleaned = true }, nil }
	v.Attest = func(loopengine.Item, string, string) (deskkit.VerifierReceipt, error) {
		return deskkit.VerifierReceipt{}, errors.New("unverifiable stamp")
	}
	if _, err := v.Dispatch(fixtureItem(), loopengine.TierLocal); err == nil {
		t.Fatal("unverified native runner launched")
	}
	if cleaned {
		t.Fatal("failed native admission destroyed recovery metadata")
	}
}
