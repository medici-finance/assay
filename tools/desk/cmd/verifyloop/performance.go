package main

import "github.com/medici-finance/assay/perfrec"

// Labels are compiled vocabulary. No root, brief, blocker, command or error is a label.
var verifyPerfVerbs = []string{"plan", "verdict"}
var verifyPerfPhases = []string{
	"startup", "guard", "preflight", "queue-scan", "blocker",
	"pending-discovery", "pending-evidence", "attestation",
	"base-queue", "classify", "render", "rows", "sign",
	"association", "association-root", "association-registry",
}

// Invocation-local and optional. Nested phases are inclusive; the shared recorder
// marks overlap. Neither timings nor missing observations affect an operational result.
type verifyPerformance struct {
	session *perfrec.Session
	context perfrec.Context
}

func (p verifyPerformance) phase(name string) func() { return p.session.Phase(name) }
