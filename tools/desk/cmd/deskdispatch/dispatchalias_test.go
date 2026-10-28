package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A refused acquire whose `show` says the key is FREE but another spelling of the same PR is
// ALIAS-HELD is a live-holder collision (medici-finance/assay#3700), not "no live holder, a
// claim-acquire error", and it is never classified STALE from the alias line's own state/age.
func TestAliasHeldIsReportedAsALiveHolder(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "dispatch-claim.sh acquire", code: deskkit.ExitRefused},
		{match: "dispatch-claim.sh show", stdout: "dispatch-claim: FREE assay--item-1 (no refs/dispatch/assay--item-1 in the repo)\n" +
			"dispatch-claim: ALIAS-HELD other--item-1 — dispatch-claim other--item-1 owner=x state=claimed branch=feat/y " +
			"at=2026-08-24T00:00:00Z age=500m (same PR as assay--item-1)"},
	}
	err := cmdDispatch([]string{"item-1", "--root", root})
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("want a refusal, got %v", err)
	}
	for _, want := range []string{"already claimed by a LIVE holder", "another spelling", "other--item-1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not say %q: %s", want, err.Error())
		}
	}
	if strings.Contains(err.Error(), "STALE") || strings.Contains(err.Error(), "NOT a collision") {
		t.Errorf("an alias holder was mislabelled: %s", err.Error())
	}
	if s.ran("steal") {
		t.Fatal("deskdispatch stole a claim held under another spelling")
	}
}

// When the key is itself HELD and an alias is live too, the key's OWN holder is the one named,
// and it still goes through the staleness read; the alias line does not displace it.
func TestOwnHeldClaimBeatsAliasLine(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "dispatch-claim.sh acquire", code: deskkit.ExitRefused},
		{match: "dispatch-claim.sh show", stdout: "dispatch-claim: HELD assay--item-1 owner=me state=dispatched branch=feat/x " +
			"at=2026-08-24T00:00:00Z age=5m\n" +
			"dispatch-claim: ALIAS-HELD other--item-1 — owner=x state=claimed branch=feat/y at=2026-08-24T00:00:00Z age=500m (same PR as assay--item-1)"},
	}
	err := cmdDispatch([]string{"item-1", "--root", root})
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("want a refusal, got %v", err)
	}
	if strings.Contains(err.Error(), "another spelling") || !strings.Contains(err.Error(), "owner=me") {
		t.Errorf("the key's own holder was displaced by the alias line: %s", err.Error())
	}
}

// A `show` that exits unverifiable (it could not read the claim set or an alias leg) is not "no
// live holder, a claim-acquire error": a collision cannot be ruled out. Still a refusal.
func TestShowUnverifiableIsNotAKeyError(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "dispatch-claim.sh acquire", code: deskkit.ExitRefused},
		{match: "dispatch-claim.sh show", code: deskkit.ExitUnverifiable, stderr: "could not list dispatch claims"},
	}
	err := cmdDispatch([]string{"item-1", "--root", root})
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("want a refusal, got %v", err)
	}
	if strings.Contains(err.Error(), "NOT a collision") || !strings.Contains(err.Error(), "unverifiable") {
		t.Errorf("an unreadable alias set was reported as a key error: %s", err.Error())
	}
}
