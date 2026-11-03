package index

import (
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// This file is the attention derivation (decision-model spec §1.2, §1.3), the
// "08b" derivation layer on top of the read surface's schema. Its input set is CLOSED
// and authority-tested: DeriveAttention may read only (adapter-sourced index
// rows, cell config, the clock) — nothing else, ever. There is no field,
// endpoint, tool parameter, header, file, env var, beacon, or config key by
// which an automation asserts or nudges its own state (the no-sensor
// constraint, spec §1.2). Read/Acked never reach here.
//
// The enforcement is not a comment: TestDerivationInputs (the §11 authority
// suite) parses this file and FAILS on any addition — a new import beyond
// {time, internal/domain}, or a new field on AttentionInputs / AttentionConfig.
// Adding an env-var read means importing "os", which the test rejects; adding
// a request field means a new struct field, which the test rejects.

// DefaultFirstArtifactN is N's implementer constant (spec §1.3): 300 seconds.
// It is NOT a ruled SLA (D3) — the SLA proper is set later from dogfood data.
// Per-role overridable beside silence_threshold via AttentionConfig.
const DefaultFirstArtifactN = 300 * time.Second

// firstArtifactFloor is the poller tick (spec §5.1): the poller ticks at 60s,
// so an N below it is unmeasurable and the deadline never fires under it. A
// false "failed-to-start" from a sub-poll N would be a fact about the poller,
// not the worker.
const firstArtifactFloor = 60 * time.Second

// Artifact is one durable record an automation left on the artifact plane —
// the only liveness signal the derivation trusts. F-08: artifacts are
// agent-writable, so "working" means "left artifacts", not "is healthy".
type Artifact struct {
	// At is the artifact's platform-assigned timestamp.
	At time.Time
	// NoOp marks a "ran, nothing to do" tick (spec §1.2 idle row), told apart
	// from a work artifact because idle and working are different states and
	// a class that leaves no no-op artifact can never honestly derive idle.
	NoOp bool
}

// DispatchRecord is a PLATFORM-ASSIGNED dispatch marker (spec §1.3): a
// claim-lease creation, a Job/workflow-run creation, or the dispatch loop's
// own filing. It starts the first-artifact clock. A nil dispatch means the
// automation was never dispatched, so the deadline does not apply.
type DispatchRecord struct {
	// At is when the automation was dispatched (platform-assigned).
	At time.Time
}

// PodPhase is the platform-assigned liveness sensor that arrives with §9.3
// phase-2 pods (spec §1.3 upgrade path): orchestrator-written, not
// agent-writable — the F-08-hard sensor class. The derivation PREFERS it
// where a pod exists, keeping the artifact deadline as the fallback. An empty
// phase means no pod record (the pre-pod substrate this build runs on), and
// the derivation falls back to the artifact deadline. A strict improvement,
// not a redesign.
type PodPhase string

const (
	PodUnknown   PodPhase = ""
	PodPending   PodPhase = "Pending"
	PodRunning   PodPhase = "Running"
	PodSucceeded PodPhase = "Succeeded"
	PodFailed    PodPhase = "Failed"
)

// AttentionInputs is the INDEX-ROW half of the derivation's closed input set
// (spec §1.2): everything the adapter sourced from GitHub for ONE automation.
// Every field here is an input source, so the set is authority-tested — adding
// a field is adding a source, and the derivation may read only index rows,
// cell config, and the clock.
type AttentionInputs struct {
	// OpenEscalations counts open escalations whose Automation is this
	// automation. The waiting predicate is exactly this and nothing else
	// (spec §1.2: "≥ 1 open Escalation … Nothing else").
	OpenEscalations int
	// Artifacts are the automation's durable records, in any order.
	Artifacts []Artifact
	// Dispatch is the newest dispatch record, or nil when never dispatched.
	Dispatch *DispatchRecord
	// PodPhase, when non-empty, is the platform-assigned pod phase preferred
	// over the artifact deadline (spec §1.3 upgrade path).
	PodPhase PodPhase
}

// AttentionConfig is the CELL-CONFIG half of the closed input set (spec §1.2).
// Every value comes from cells.yaml — the cell and its per-role overrides —
// never from an automation.
type AttentionConfig struct {
	// Window (W) = schedule interval + grace (spec §1.2 working row): the age
	// under which the newest artifact still reads as in-flight.
	Window time.Duration
	// FirstArtifactN (N) is the first-artifact deadline (spec §1.3). Zero
	// means "use DefaultFirstArtifactN". Per-role overridable beside
	// silence_threshold; an N below firstArtifactFloor never fires.
	FirstArtifactN time.Duration
	// SilenceThreshold is the cell's silence_threshold (spec §1.2 silent row):
	// past it, silent renders loud (amber "? silent Nd"). It governs the
	// render intensity of the honest fallback, not a distinct state.
	SilenceThreshold time.Duration
	// IdleLeavesArtifact records whether THIS automation class leaves a no-op
	// tick artifact. The idle-honesty rule (spec §1.2, binding): idle is
	// reachable ONLY for classes whose no-op tick leaves an artifact; where a
	// class leaves nothing, an idle automation is indistinguishable from a
	// dead one and derives Silent, never Idle.
	IdleLeavesArtifact bool
}

// DeriveAttention computes one automation's attention state (spec §1.2), in
// precedence order failed-to-start > waiting > working > idle > silent, from
// (index rows, cell config, clock) and NOTHING ELSE.
//
// A loop may be working AND waiting; the human-actionable fact (waiting) wins
// the rollup, and a pane may render the compound ("working · 2 waiting") from
// the same inputs — but the single rolled-up state this returns is the
// higher-precedence one.
func DeriveAttention(in AttentionInputs, cfg AttentionConfig, now time.Time) domain.AttentionState {
	// Prefer the platform-assigned pod phase where a pod exists (spec §1.3):
	// it is orchestrator-written, not agent-writable, so it outranks the
	// artifact deadline as a born-dead signal. Running/Succeeded fall through
	// to the artifact-based derivation; the pod being up does not by itself
	// assert working (F-08: only artifacts do).
	switch in.PodPhase {
	case PodFailed:
		return domain.FailedToStart
	case PodPending:
		if bornDead(in, cfg, now) {
			return domain.FailedToStart
		}
	case PodUnknown:
		// No pod record: fall back to the artifact deadline.
		if bornDead(in, cfg, now) {
			return domain.FailedToStart
		}
	}

	// waiting — exactly one predicate (spec §1.2), outranking working.
	if in.OpenEscalations >= 1 {
		return domain.Waiting
	}

	// working — newest WORK artifact younger than W.
	if a, ok := newestWork(in.Artifacts); ok && now.Sub(a.At) < cfg.Window {
		return domain.Working
	}

	// idle — only when the class leaves a no-op artifact AND a no-op tick is
	// within W (spec §1.2 idle row + honesty rule). No IdleLeavesArtifact ⇒
	// this branch is unreachable and an idle loop derives Silent below.
	if cfg.IdleLeavesArtifact {
		if a, ok := newestNoOp(in.Artifacts); ok && now.Sub(a.At) < cfg.Window {
			return domain.Idle
		}
	}

	// silent — the honest fallback (spec §1.2): nothing known in flight. The
	// board admits unknown rather than inventing a healthier state.
	return domain.Silent
}

// bornDead reports the first-artifact deadline (spec §1.3): a dispatch record
// with no SUBSEQUENT artifact, past N.
//
// Any artifact after dispatch clears it — state is recomputed each poll, so a
// late first artifact clears it with no sticky flag and no reset procedure. An
// N below the poller floor never fires, because it is unmeasurable under the
// 60s tick (spec §5.1).
func bornDead(in AttentionInputs, cfg AttentionConfig, now time.Time) bool {
	if in.Dispatch == nil {
		return false
	}
	n := cfg.FirstArtifactN
	if n <= 0 {
		n = DefaultFirstArtifactN
	}
	if n < firstArtifactFloor {
		return false
	}
	for _, a := range in.Artifacts {
		if a.At.After(in.Dispatch.At) {
			return false
		}
	}
	return now.Sub(in.Dispatch.At) > n
}

// newestWork returns the most recent non-no-op artifact.
func newestWork(arts []Artifact) (Artifact, bool) {
	var best Artifact
	found := false
	for _, a := range arts {
		if a.NoOp {
			continue
		}
		if !found || a.At.After(best.At) {
			best, found = a, true
		}
	}
	return best, found
}

// newestNoOp returns the most recent no-op tick artifact.
func newestNoOp(arts []Artifact) (Artifact, bool) {
	var best Artifact
	found := false
	for _, a := range arts {
		if !a.NoOp {
			continue
		}
		if !found || a.At.After(best.At) {
			best, found = a, true
		}
	}
	return best, found
}
