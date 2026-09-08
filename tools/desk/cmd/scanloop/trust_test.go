package main

import (
	"errors"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func inbound(repo string, n int) Inbound {
	return Inbound{Repo: repo, Number: n, UpdatedAt: time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)}
}

func probeReturning(author string, bodyEdited time.Time, events []deskkit.ContentEvent, complete bool, err error) TrustProbe {
	return func(string, int) (string, time.Time, []deskkit.ContentEvent, bool, error) {
		return author, bodyEdited, events, complete, err
	}
}

// TestGate_TrustedAuthorIsAdmitted is the positive control.
func TestGate_TrustedAuthorIsAdmitted(t *testing.T) {
	adm := ApplyTrustGate([]Inbound{inbound("example-org/tracker", 1)},
		probeReturning("ada", time.Time{}, nil, true, nil))
	if !adm[0].Admitted() {
		t.Fatalf("a trusted author was not admitted: %+v", adm[0])
	}
}

// TestGate_UntrustedUnblessedIsQuarantinedNotDropped — quarantine is VISIBLE. An item that
// disappeared from the gate's output would be indistinguishable from one that never arrived.
func TestGate_UntrustedUnblessedIsQuarantinedNotDropped(t *testing.T) {
	adm := ApplyTrustGate([]Inbound{inbound("example-org/tracker", 2)},
		probeReturning("outsider", time.Time{}, nil, true, nil))
	if len(adm) != 1 {
		t.Fatalf("the gate DROPPED a row: %v", adm)
	}
	if adm[0].State != AdmissionQuarantined || adm[0].Admitted() {
		t.Fatalf("state = %s, want QUARANTINED and not admitted", adm[0].State)
	}
}

// TestGate_BlessingAdmits — one comment from the blessing authority admits an untrusted item.
func TestGate_BlessingAdmits(t *testing.T) {
	bless := time.Date(2026, 8, 24, 8, 0, 0, 0, time.UTC)
	events := []deskkit.ContentEvent{{Author: "ada", AuthorID: 2001, CreatedAt: bless}}
	adm := ApplyTrustGate([]Inbound{inbound("example-org/tracker", 3)},
		probeReturning("outsider", time.Time{}, events, true, nil))
	if !adm[0].Admitted() {
		t.Fatalf("a blessed item was not admitted: %+v", adm[0])
	}
}

// TestGate_BlessThenEditVoidsTheBlessing — the blessing covers the content as of the comment. A
// body edited after it re-quarantines the item.
func TestGate_BlessThenEditVoidsTheBlessing(t *testing.T) {
	bless := time.Date(2026, 8, 24, 8, 0, 0, 0, time.UTC)
	events := []deskkit.ContentEvent{{Author: "ada", AuthorID: 2001, CreatedAt: bless}}
	adm := ApplyTrustGate([]Inbound{inbound("example-org/tracker", 4)},
		probeReturning("outsider", bless.Add(time.Minute), events, true, nil))
	if adm[0].Admitted() {
		t.Fatal("an item whose body was edited AFTER the blessing was admitted")
	}
}

// TestGate_OverflowedThreadFailsClosed — a blessing cannot be read off a partial thread.
func TestGate_OverflowedThreadFailsClosed(t *testing.T) {
	bless := time.Date(2026, 8, 24, 8, 0, 0, 0, time.UTC)
	events := []deskkit.ContentEvent{{Author: "ada", AuthorID: 2001, CreatedAt: bless}}
	adm := ApplyTrustGate([]Inbound{inbound("example-org/tracker", 5)},
		probeReturning("outsider", time.Time{}, events, false, nil))
	if adm[0].Admitted() {
		t.Fatal("an item whose thread overflowed a page was admitted on a partial read")
	}
	if adm[0].State != AdmissionQuarantined {
		t.Fatalf("state = %s, want QUARANTINED", adm[0].State)
	}
}

// TestGate_UnwiredProbeIsCouldNotCheck_NotQuarantine — the three states are distinct on purpose.
// Quarantine is a verdict about an item we looked at; an unread gate has no verdict at all, and
// collapsing the two would let a broken probe read as a working trust gate.
func TestGate_UnwiredProbeIsCouldNotCheck_NotQuarantine(t *testing.T) {
	adm := ApplyTrustGate([]Inbound{inbound("example-org/tracker", 6)}, nil)
	if adm[0].State != AdmissionCouldNotCheck {
		t.Fatalf("state = %s, want COULD-NOT-CHECK", adm[0].State)
	}
	if adm[0].Admitted() {
		t.Fatal("an unread gate ADMITTED an item")
	}
}

// TestGate_ProbeErrorIsCouldNotCheck — a read that failed never becomes an admission.
func TestGate_ProbeErrorIsCouldNotCheck(t *testing.T) {
	adm := ApplyTrustGate([]Inbound{inbound("example-org/tracker", 7)},
		probeReturning("", time.Time{}, nil, false, errors.New("boom")))
	if adm[0].State != AdmissionCouldNotCheck || adm[0].Admitted() {
		t.Fatalf("adm = %+v, want COULD-NOT-CHECK and not admitted", adm[0])
	}
}

// TestGate_CountsAreThreeState — the report has to be able to say "we could not look" separately
// from "we looked and refused".
func TestGate_CountsAreThreeState(t *testing.T) {
	adm := []Admission{
		{State: AdmissionAdmitted},
		{State: AdmissionQuarantined},
		{State: AdmissionQuarantined},
		{State: AdmissionCouldNotCheck},
	}
	a, q, u := AdmissionCounts(adm)
	if a != 1 || q != 2 || u != 1 {
		t.Fatalf("counts = %d/%d/%d, want 1/2/1", a, q, u)
	}
}

// fakeForge is a minimal Forge for the trust-probe tests: it embeds the interface (so every
// method the probe never calls is present and would panic if reached) and implements only the
// two reads the probe uses. That the probe reaches NO other method is part of what these tests
// prove — a probe that grew a third read would panic on the nil-embedded method rather than
// pass unnoticed.
type fakeForge struct {
	deskkit.Forge
	getIssueCalls   int
	issueTrustCalls int
	author          string
	trust           *deskkit.TrustPayload
}

func (f *fakeForge) GetIssue(deskkit.ForgeRepo, int) (*deskkit.Issue, error) {
	f.getIssueCalls++
	return &deskkit.Issue{Author: deskkit.Account{Login: f.author}}, nil
}

func (f *fakeForge) IssueTrustEvents(deskkit.ForgeRepo, int) (*deskkit.TrustPayload, error) {
	f.issueTrustCalls++
	return f.trust, nil
}

func fakeResolve(f *fakeForge) func(string) (deskkit.Forge, deskkit.ForgeRepo, error) {
	return func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return f, deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, nil
	}
}

// TestForgeTrustProbe_TrustedAuthorCostsOneRead — API growth on a busy scope is bounded by NOT
// reading the thread of an item whose author is already trusted.
func TestForgeTrustProbe_TrustedAuthorCostsOneRead(t *testing.T) {
	f := &fakeForge{author: "ada", trust: &deskkit.TrustPayload{Complete: true}}
	p := forgeTrustProbe(fakeResolve(f))
	author, _, _, complete, err := p("example-org/tracker", 9)
	if err != nil || author != "ada" || !complete {
		t.Fatalf("probe = %q %v %v", author, complete, err)
	}
	if f.getIssueCalls != 1 || f.issueTrustCalls != 0 {
		t.Fatalf("calls = author:%d thread:%d, want 1/0 (a trusted author must not trigger a thread read)",
			f.getIssueCalls, f.issueTrustCalls)
	}
}

// TestForgeTrustProbe_UntrustedAuthorReadsTheThread — and only then.
func TestForgeTrustProbe_UntrustedAuthorReadsTheThread(t *testing.T) {
	f := &fakeForge{author: "outsider", trust: &deskkit.TrustPayload{Complete: true}}
	p := forgeTrustProbe(fakeResolve(f))
	if _, _, _, _, err := p("example-org/tracker", 10); err != nil {
		t.Fatal(err)
	}
	if f.getIssueCalls != 1 || f.issueTrustCalls != 1 {
		t.Fatalf("calls = author:%d thread:%d, want the author read then ONE bounded thread read",
			f.getIssueCalls, f.issueTrustCalls)
	}
}
