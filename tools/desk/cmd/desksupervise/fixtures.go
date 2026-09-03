package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// fixtures.go — the two fixture JSON shapes that let the Verify table run with no network
// and no filesystem audit dependency (--claims-fixture / --observations-fixture), and the
// tier-string parser both the fixture and live claim paths share.

// claimRecord is one dispatch claim, either read from --claims-fixture (a JSON array of
// these) or, in live mode, assembled from the claim tool's own records (see live.go).
type claimRecord struct {
	Key          string `json:"key"`
	Item         string `json:"item"`
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	PR           int    `json:"pr,omitempty"`
	Branch       string `json:"branch"`
	Tier         string `json:"tier"` // local | cheap | session | human
	State        string `json:"state,omitempty"`
	DispatchedAt string `json:"dispatchedAt"` // RFC3339
}

// observationRecord is one claim's cross-probe observation, keyed by claim key in
// --observations-fixture's JSON object. It stands in for a live ObservableProbes.Latest()
// read: CouldNotCheck true means "at least one probe errored and none saw life" (BLIND);
// otherwise At (empty/zero = never observed) and Via are exactly what Latest() would have
// returned as Observation.At / Observation.What.
type observationRecord struct {
	CouldNotCheck bool   `json:"couldNotCheck,omitempty"`
	Error         string `json:"error,omitempty"`
	At            string `json:"at,omitempty"`
	Via           string `json:"via,omitempty"`
}

// tierOf parses a claim's tier string. An empty/unrecognised tier is a malformed claim
// record, not a liveness question — the caller treats it as a hard config error (fail the
// whole tick) rather than guessing a default that would silently apply the wrong wall cap.
func tierOf(s string) (loopengine.Tier, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "local":
		return loopengine.TierLocal, nil
	case "cheap":
		return loopengine.TierCheap, nil
	case "session":
		return loopengine.TierSession, nil
	case "human":
		return loopengine.TierHuman, nil
	default:
		return 0, fmt.Errorf("unrecognised tier %q (want one of: local, cheap, session, human)", s)
	}
}

// loadClaimsFixture reads a --claims-fixture file: a JSON array of claimRecord. Any read or
// parse failure is could-not-check (exit 6) — a fixture file that cannot be read is never
// treated as "no claims".
func loadClaimsFixture(path string) ([]claimRecord, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, deskkit.Unverifiable("cannot read --claims-fixture "+path, err)
	}
	var claims []claimRecord
	if err := json.Unmarshal(b, &claims); err != nil {
		return nil, deskkit.Unverifiable("cannot parse --claims-fixture "+path+" as a JSON array of claim records", err)
	}
	for i, c := range claims {
		if strings.TrimSpace(c.Key) == "" {
			return nil, deskkit.Unverifiable(fmt.Sprintf("--claims-fixture %s: claim %d has no key", path, i), nil)
		}
	}
	return claims, nil
}

// loadObservationsFixture reads a --observations-fixture file: a JSON object keyed by claim
// key. A read/parse failure is could-not-check; a MISSING file is treated as "no
// observations fixture given" only by the caller checking the flag itself — this function
// always requires the file to exist once the flag names one.
func loadObservationsFixture(path string) (map[string]observationRecord, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, deskkit.Unverifiable("cannot read --observations-fixture "+path, err)
	}
	var obs map[string]observationRecord
	if err := json.Unmarshal(b, &obs); err != nil {
		return nil, deskkit.Unverifiable("cannot parse --observations-fixture "+path+" as a JSON object keyed by claim key", err)
	}
	return obs, nil
}

// resolvedObservation is the outcome of resolving one claim's cross-probe reading for this
// tick, in the shape ClassifyLiveness (and the could-not-check branch above it) needs:
// either a clean Observation (obs, observed=true — obs.At zero means cleanly silent), or
// could-not-check (observed=false), never both.
type resolvedObservation struct {
	obs      loopengine.Observation
	observed bool
}

// fromFixture converts one observationRecord into a resolvedObservation. An unparseable
// non-empty `at` is treated as could-not-check on the FIXTURE itself — a malformed test
// fixture must fail loud, not silently read as "never observed".
func (r observationRecord) resolve() (resolvedObservation, error) {
	if r.CouldNotCheck {
		return resolvedObservation{observed: false}, nil
	}
	if strings.TrimSpace(r.At) == "" {
		return resolvedObservation{obs: loopengine.Observation{}, observed: true}, nil
	}
	ts, err := time.Parse(time.RFC3339, r.At)
	if err != nil {
		return resolvedObservation{}, fmt.Errorf("observation `at` %q is not RFC3339: %w", r.At, err)
	}
	return resolvedObservation{obs: loopengine.Observation{At: ts, What: r.Via}, observed: true}, nil
}
