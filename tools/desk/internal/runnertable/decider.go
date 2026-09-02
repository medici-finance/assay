package runnertable

// decider.go — the pinned DECIDER runner entry: the fixed, configuration-only
// runner that drives the comms prose router and outbound gate (companion
// packages landing separately). Model assignment for the decider is itself a
// table lookup, never a runtime choice — this is that lookup's other half.
// cmd/commsloop's assign.go resolves ACTION -> Tier -> a pinned dispatchable
// runner for WORK the desk fires; this file resolves the DECIDER'S OWN pinned
// runner, which is not keyed by Tier at all because the decider is not
// dispatched work — it runs on every message.
//
// CONTAINMENT. The decider reads untrusted inbound text on every message (a
// dual-LLM / quarantined-reader pattern), so its runner entry must be
// launchable under the refuse-everything acp policy (internal/acp
// DefaultRefusePermission, empty FSRoot, terminal refused). The actual POLICY
// wiring lives in a different component (the router/gate's own boot, which
// constructs the acp.Client); this package's job is the CONFIG-side
// guarantee: a decider entry not explicitly declared for that containment
// profile is refused at boot — the same fail-closed posture as an unpinned
// runner or a C-floor isolate=false entry. Nothing here launches anything.

import (
	"encoding/json"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// envDeciderKey holds the decider's ONE runner entry as a JSON object — the
// same shape as ASSAY_RUNNER_<TIER>, plus the mandatory `contained`
// attestation. It is deliberately its own key, not a fourth entry in
// dispatchableTierNames: the decider is never resolved via Resolve(Tier) and
// must never be reachable through the dispatchable-tier path.
const envDeciderKey = "ASSAY_RUNNER_DECIDER"

// DeciderEntry is the pinned runner-table entry for the decider.
type DeciderEntry struct {
	RunnerEntry
	// Contained is a boot-time self-attestation that this entry is meant to
	// launch under the refuse-everything acp policy. Absent/false refuses — a
	// decider entry not declared contained is a config error, never an
	// accepted-but-uncontained entry.
	Contained bool `json:"contained"`
}

// LoadDecider loads and validates the pinned decider entry. getenv is
// injected for tests (a nil-map envGetter closure, matching LoadRunnerTable's
// convention). Missing entirely, missing a version pin, or Contained=false
// each refuse with a distinct message — the decider has no default and no
// degraded/uncontained mode.
func LoadDecider(getenv func(string) string) (*DeciderEntry, error) {
	val := strings.TrimSpace(getenv(envDeciderKey))
	if val == "" {
		return nil, deskkit.Refused(
			"decider: no entry configured (set " + envDeciderKey + ") — the comms decider has no default runner")
	}
	var e DeciderEntry
	if err := json.Unmarshal([]byte(val), &e); err != nil {
		return nil, deskkit.Refused("decider: " + envDeciderKey + " is not valid JSON: " + err.Error())
	}
	// Reuse the same fail-closed entry rules (mandatory pin, C-floor isolate)
	// every dispatchable-tier entry is held to — a decider runner is safety
	// plumbing too, and a second hand-rolled validator is exactly the kind of
	// drift the reuse ladder forecloses.
	if err := validateEntry("decider", e.RunnerEntry); err != nil {
		return nil, err
	}
	if !e.Contained {
		return nil, deskkit.Refused(
			"decider: entry does not declare contained:true — refusing (a decider entry not declared for the " +
				"refuse-everything acp policy is a config error; containment never silently degrades, C floor)")
	}
	return &e, nil
}
