# Verification candidate planning

`PlanCandidates` is the pure verification-role planning entrypoint.
Both the skill adapter and the discrete role binding consume it; neither integration is activated by this package.

Inputs are a versioned `Observation`, explicit policy owner/revision, a supplied
clock and oldest acceptable observation time, and nonnegative capacity. The caller
qualifies policy approval, authorized scope and source provenance. This API performs
no provider, filesystem, process, credential, global-state or wall-clock read.
`statusgen/verifyqueue` is its importable wire contract; statusgen's existing
`placeAwaiting` remains the classifier. `Compose` is the single demotion-only
base/overlay join also used by the existing verifyloop plan.

`verifyloop plan --base-queue --json` includes `candidateObservation`. It records
the actual canonical rows, overlay outcomes, exact subject revisions and all
requested root coverage. A subject revision is the item's exact target (`--sha`, or a
cross-repository deliverable's source snapshot), else, for a same-repository item in
multi-root mode (which refuses `--sha`), its own root's merged-main snapshot; a root
whose snapshot cannot be read or whose origin is not the configured repository supplies
none. An explicit single-root scan cannot establish whole-desk idle. Missing revision or
source coverage makes the affected candidate could-not-check. `Observation.Revision` is
`ContentRevision`, a digest of the content with the collection time excluded;
`PlanCandidates` recomputes it and refuses a mismatch. That is a consistency check, not an
authenticity control: anyone holding the content can recompute it. Evidence
repair discovery is explicitly unknown until its collector supplies it; accepted-run
continuation remains with its existing owner. Existing risk, attestation, admission and landing controls remain independent.

Selection unions identical source-lane/alias observations, rejects conflicting heads
or dispositions, preserves all held/unknown/watch rows, ranks eligible proposals by
explicit priority (then the existing observed order), and retains capacity-deferred
work. Unmapped priority follows the explicit observed-score or hold policy; hold applies
only to eligible proposals, so watched, held and unknown rows keep their disposition. Unobserved
priority entries remain coverage gaps. Paused and parked rows are watched; known owed
work prevents idle, and unknown coverage never proves idle. A proposal grants no authority;
each decision's `authority` (`flip`, `evidence-only` or `none`) names the ceiling a caller's
dispatch would carry, keeping Evidence-only work apart from judgment work.

The shared fixture and independent oracle run in `cmd/verifyloop` as
`TestVerifyPlanParity`; mutation controls live beside that test. The original three
`TestFlowQueue` surfaces retain producer/board/planner compatibility coverage.
