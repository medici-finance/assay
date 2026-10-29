# Verification candidate planning

`PlanCandidates` is the pure verification-role planning entrypoint for desk-tools/45.
Both the skill adapter (desk-tools/51) and the discrete role binding
(graph-execution/27) consume it; neither integration is activated by this package.

Inputs are a versioned `Observation`, explicit policy owner/revision, a supplied
clock and oldest acceptable observation time, and nonnegative capacity. The caller
qualifies policy approval, authorized scope and source provenance. This API performs
no provider, filesystem, process, credential, global-state or wall-clock read.
`statusgen/verifyqueue` is its importable wire contract; statusgen's existing
`placeAwaiting` remains the classifier. `Compose` is the single demotion-only
base/overlay join also used by the existing verifyloop plan.

`verifyloop plan --base-queue --json` includes `candidateObservation`. It records
the actual canonical rows, overlay outcomes, supplied exact subject revisions and
all requested root coverage. An explicit single-root scan cannot establish whole-desk
idle. Missing revision or source coverage holds the affected candidate. Evidence
repair discovery is explicitly unknown until /51 supplies it; accepted-run continuation
remains /47. Existing risk, attestation, admission and landing controls remain independent.

Selection unions identical source-lane/alias observations, rejects conflicting heads
or dispositions, preserves all held/unknown/watch rows, ranks eligible proposals by
explicit priority (then the existing observed order), and retains capacity-deferred
work. Unmapped priority follows the explicit observed-score or hold policy. Unobserved
priority entries remain coverage gaps. Paused and parked rows are watched; known owed
work prevents idle, and unknown coverage never proves idle. A proposal grants no authority.

The shared fixture and independent oracle run in `cmd/verifyloop` as
`TestVerifyPlanParity`; mutation controls live beside that test. The original three
`TestFlowQueue` surfaces retain producer/board/planner compatibility coverage.
