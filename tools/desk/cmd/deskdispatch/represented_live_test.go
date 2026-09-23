package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// represented_live_test.go — the phantom check driven through the REAL transport path
// (Forge.ListChanges → deskkit.RepresentedPRRefs → []PRRef → BriefRepresentedPR), not a hand-
// built PRRef slice. It proves the wiring this PR adds actually reduces a merged/open change list
// into a phantom refusal, with no live forge.

// fakeForge answers ONLY ListChanges; every other Forge method promotes off the embedded nil
// interface and would panic if the transport ever reached for one — which pins that the phantom
// read touches nothing but ListChanges.
type fakeForge struct {
	deskkit.Forge
	cl *deskkit.ChangeList
}

func (f fakeForge) ListChanges(deskkit.ForgeRepo, deskkit.ChangeStates) (*deskkit.ChangeList, error) {
	return f.cl, nil
}

// realTransport is what liveRepresentedPRs does minus ResolveForge (which needs credentials): it
// runs the actual deskkit.RepresentedPRRefs reduction over a fake forge's ListChanges result.
func realTransport(f deskkit.Forge) func(string) ([]deskkit.PRRef, error) {
	return func(repo string) ([]deskkit.PRRef, error) {
		fr, err := forgeRepoOf(repo)
		if err != nil {
			return nil, err
		}
		refs, _, err := deskkit.RepresentedPRRefs(f, fr)
		return refs, err
	}
}

// TestPhantomCheckRefusesViaRealTransport: a MERGED change carrying the brief's `Brief:` trailer,
// read through ListChanges and reduced by RepresentedPRRefs, makes the fresh dispatch a phantom.
func TestPhantomCheckRefusesViaRealTransport(t *testing.T) {
	f := fakeForge{cl: &deskkit.ChangeList{Changes: []deskkit.ChangeRef{
		{Number: 700, State: "MERGED", Body: "Delivers it.\n\nBrief: example-a/00\n"},
	}}}
	withRepresentedPRs(t, realTransport(f))

	err := phantomCheck(dispatchOpts{item: "assay--example-a--00", kit: "worker", pr: 0}, allowedRepo)
	if err == nil {
		t.Fatal("a merged change representing the brief, read through the real transport, must refuse the dispatch")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("phantom is a REFUSAL (exit 5), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "#700") {
		t.Errorf("the refusal must name the representing PR #700: %v", err)
	}
}

// TestPhantomCheckAllows_ViaRealTransport_WhenUnrepresented is the control: the same real transport,
// a change list that represents a DIFFERENT brief, dispatches normally — so the refusal above is
// provoked by the match, not by the transport path itself.
func TestPhantomCheckAllows_ViaRealTransport_WhenUnrepresented(t *testing.T) {
	f := fakeForge{cl: &deskkit.ChangeList{Changes: []deskkit.ChangeRef{
		{Number: 501, State: "OPEN", Body: "Brief: example-other/03"},
	}}}
	withRepresentedPRs(t, realTransport(f))

	if err := phantomCheck(dispatchOpts{item: "assay--example-a--00", kit: "worker"}, allowedRepo); err != nil {
		t.Fatalf("a brief with no representing change must dispatch through the real transport: %v", err)
	}
}
