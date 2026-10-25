package deskkit

// worldbrief.go — what a brief says its run needs, and the one place a
// verifier home's prerequisite record is admitted and bound on the shipped
// prepare path (example-stream/50).
//
// Both the dispatcher's prepare step and the loop's in-process plan read the
// same derivation here, so the declared siblings and the pinned instruments a
// record names cannot differ between the two.

import (
	"os"
	"path/filepath"
)

// WorldSpecFromBrief reads what one run needs from its brief text: the
// declared sibling checkouts, and the pinned instruments among commands, the
// Command cells the deterministic runner executes. The hub does not read a
// Verify table's Command column itself; the caller lists the cells (see
// internal/runnercmds), so the hub's import set stays as it is.
func WorldSpecFromBrief(root, brief string, commands []string) WorldSpec {
	return WorldSpec{
		Root:        root,
		Siblings:    declaredSiblings(brief),
		Instruments: WorldInstruments(root, commands),
	}
}

// verifierWorldCapability is the capability evidence a prepared run carries:
// the attestation round trip that just completed (the stamp was created,
// closed and read back as the verifier-attributed stamp) is itself the read of
// the dispatcher's write capability for this repo.
const verifierWorldCapability = "verifier-attestation"

// AdmitAndBindVerifierWorld is the shipped prepare path's admission of the
// run's prerequisite record. Called with the home of a run whose attestation
// has just completed, it reads the brief the run is bound to, admits its
// prerequisites against the verifier home (so the recorded source is the
// run's own attested source, not an ambient checkout), and persists the record
// on the run. A prerequisite that cannot be read or does not conform returns
// the admission's own error and binds nothing, so the dispatch emits no prompt
// for a world it could not record. commands lists the brief's runner-executed
// Command cells (internal/runnercmds.Commands), supplied by the caller.
func AdmitAndBindVerifierWorld(home string, commands func(brief string) []string) (VerifierReceipt, error) {
	if commands == nil {
		return VerifierReceipt{}, Unverifiable("environment could-not-check: no reader for the brief's runner commands", nil)
	}
	r, err := verifierLoad(home)
	if err != nil {
		return r.VerifierReceipt, err
	}
	briefRoot := r.Home
	if r.Binding.Version == 2 {
		briefRoot = r.TrackingHome
	}
	body, err := os.ReadFile(filepath.Join(briefRoot, filepath.FromSlash(r.Binding.Brief)))
	raw := string(body)
	if err != nil {
		return r.VerifierReceipt, Unverifiable("environment could-not-check: cannot read the brief's prerequisites: "+oneLine(err.Error()), nil)
	}
	spec := WorldSpecFromBrief(r.Home, raw, commands(raw))
	spec.Repo, spec.Source = r.Binding.Repo, r.Binding.Source
	spec.Capability = clean(verifierWorldCapability, "stamp created, closed and read back", "")
	adm := AdmitWorld(spec, worldRecheckProbes)
	if !adm.Ready() {
		return r.VerifierReceipt, adm.Err()
	}
	return BindVerifierWorld(home, adm.World)
}
