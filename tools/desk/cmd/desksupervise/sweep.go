package main

import (
	"fmt"
	"io"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// sweep.go — the one classification-and-action loop `tick` runs, whether its claims and
// observations come from --claims-fixture/--observations-fixture (offline, the Verify
// table's path) or from a live claim tool + loopengine.HouseProbes() (live.go). Both paths
// converge on observationSource — sweep itself never knows which one it was handed.

// observationSource resolves one claim's cross-probe reading for this tick. The fixture
// path (fixtureObservationSource) looks the claim key up in a canned map; the live path
// (liveObservationSource in live.go) runs the real probes.
type observationSource func(claim claimRecord) (resolvedObservation, error)

// fixtureObservationSource is the offline observationSource --observations-fixture builds.
// A claim key ABSENT from the fixture is "never observed, checked cleanly" — not an error
// and not could-not-check; a fixture author who wants COULD-NOT-CHECK says so explicitly
// (`{"couldNotCheck": true}`).
func fixtureObservationSource(byKey map[string]observationRecord) observationSource {
	return func(claim claimRecord) (resolvedObservation, error) {
		rec, ok := byKey[claim.Key]
		if !ok {
			return resolvedObservation{observed: true}, nil
		}
		return rec.resolve()
	}
}

// sweepResult is one claim's classification, for a caller (tick's own printer, or a test)
// that wants the structured outcome rather than a parsed stdout line.
type sweepResult struct {
	Claim  claimRecord
	Disp   loopengine.Disposition // meaningless when Blind is true
	Blind  bool
	Last   time.Time
	Via    string
	Action string
}

// sweep classifies every claim once against pol at now, executing (or, under dryRun, only
// printing) the implied action for each, and writes one rendered line per claim to out. It
// returns the structured results (so tests can assert on fields rather than re-parsing
// stdout) and whether any claim was BLIND — the caller maps that to exit 6.
//
// A malformed claim record (bad tier, bad dispatchedAt) or an unresolvable observation
// (malformed fixture `at`) aborts the WHOLE tick as could-not-check: these are config
// errors in the tick's own INPUT, not liveness questions about the claim, and guessing a
// default would silently misclassify every claim after the bad one.
func sweep(claims []claimRecord, obsSource observationSource, pol loopengine.LivenessPolicy, now time.Time, dryRun bool, reclaim reclaimFunc, fileBT fileBlockedTimeoutFunc, out io.Writer) ([]sweepResult, bool, error) {
	runTag := fmt.Sprintf("desksupervise-%d", now.UnixNano())
	var results []sweepResult
	anyBlind := false

	for _, claim := range claims {
		tier, terr := tierOf(claim.Tier)
		if terr != nil {
			return results, anyBlind, deskkit.Unverifiable("claim "+claim.Key+": "+terr.Error(), terr)
		}
		dispatchedAt, derr := time.Parse(time.RFC3339, claim.DispatchedAt)
		if derr != nil {
			return results, anyBlind, deskkit.Unverifiable(
				"claim "+claim.Key+": dispatchedAt "+claim.DispatchedAt+" is not RFC3339", derr)
		}

		resolved, rerr := obsSource(claim)
		if rerr != nil {
			return results, anyBlind, deskkit.Unverifiable("claim "+claim.Key+": cannot resolve its observation", rerr)
		}

		if !resolved.observed {
			anyBlind = true
			results = append(results, sweepResult{Claim: claim, Blind: true, Action: "BLIND"})
			fmt.Fprintf(out, "%s  COULD-NOT-CHECK last=none via=- action=BLIND\n", claim.Key)
			continue
		}

		clock := loopengine.ClaimClock{DispatchedAt: dispatchedAt}
		disp := loopengine.ClassifyLiveness(pol, clock, tier, now, resolved.obs)
		action, aerr := runAction(claim, disp, dryRun, reclaim, fileBT, runTag)
		if aerr != nil {
			return results, anyBlind, deskkit.Unverifiable("claim "+claim.Key+": action "+action+" failed", aerr)
		}

		last := "none"
		via := "-"
		if !resolved.obs.At.IsZero() {
			last = resolved.obs.At.UTC().Format(time.RFC3339)
			if resolved.obs.What != "" {
				via = resolved.obs.What
			}
		}
		results = append(results, sweepResult{
			Claim: claim, Disp: disp, Last: resolved.obs.At, Via: via, Action: action,
		})
		fmt.Fprintf(out, "%s  %s last=%s via=%s action=%s\n", claim.Key, disp, last, via, action)
	}
	return results, anyBlind, nil
}
