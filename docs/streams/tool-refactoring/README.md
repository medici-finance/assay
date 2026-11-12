---
stream: tool-refactoring
repo: medici-finance/assay
serves: assay
status: parked
priority: P1
track: platform
board: generated
spec: docs/streams/tool-refactoring/spec.md
---

# Tool refactoring — shared contracts behind the five busiest operations

Draft planning only. The stream stays parked pending review and scope approval.
See [scope and evidence](spec.md), [existing work](existing-work.md), and
[public issue routing](issue-routing.csv). New briefs fill class-level gaps; they
neither replace the approved architecture nor claim existing fixes are absent.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Bind operation subjects to explicit tracking, delivery and evidence repositories](brief-01-bind-operation-subjects-to-explicit-tracking-delivery-and-evidence-repositories.md) | 0 | M | todo | — | — |
| 02 | [Make dispatch planning and execution consume one validated operation subject](brief-02-make-dispatch-planning-and-execution-consume-one-validated-operation-subject.md) | 1 | M | todo | — | — |
| 03 | [Compile Verify rows and declared inputs through the existing document owner](brief-03-compile-verify-rows-and-declared-inputs-through-the-existing-document-owner.md) | 0 | M | todo | — | — |
| 04 | [Produce bounded row observations with base and head provenance](brief-04-produce-bounded-row-observations-with-base-and-head-provenance.md) | 1 | M | todo | — | — |
| 05 | [Share outcome decoding and reduction between verification producers and readers](brief-05-share-outcome-decoding-and-reduction-between-verification-producers-and-readers.md) | 0 | M | todo | — | — |
| 06 | [Model local cell lifecycle transitions using the existing cell-operation contract](brief-06-model-local-cell-lifecycle-transitions-using-the-existing-cell-operation-contract.md) | 0 | M | todo | — | — |
| 07 | [Qualify the real cellctl binary with isolated lifecycle and environment fixtures](brief-07-qualify-the-real-cellctl-binary-with-isolated-lifecycle-and-environment-fixtures.md) | 1 | L | todo | — | — |
| 08 | [Separate immutable publication preparation from fresh effect admission](brief-08-separate-immutable-publication-preparation-from-fresh-effect-admission.md) | 1 | M | todo | — | — |
| 09 | [Migrate PR create update and edit to the shared publication plan](brief-09-migrate-pr-create-update-and-edit-to-the-shared-publication-plan.md) | 2 | M | todo | — | — |
| 10 | [Migrate Evidence publication to bounded deltas and typed outcome admission](brief-10-migrate-evidence-publication-to-bounded-deltas-and-typed-outcome-admission.md) | 2 | M | todo | — | — |
| 11 | [Qualify the joined tool flow and prepare consumer-by-consumer retirement](brief-11-qualify-the-joined-tool-flow-and-prepare-consumer-by-consumer-retirement.md) | 3 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

The structural longest new-work chain is **01 → 08 → 10 → 11**, with
**01 → 08 → 09 → 11** in parallel. No duration estimate was made. Completion
also needs retained statusgen/37 for the v2 producer/consumer cohort.

I verified that operation-subject binding is a real head by reading
`tools/desk/cmd/deskdispatch/deliverable.go`: “repo is the deliverable repo the
alias resolves to — the claim, token, worktree and PR repo”, followed by separate
`trackingRepo` and `trackingRoot` fields; `dispatch.go` still calls
`stepDecision(o, plan.gateHuman, repo, ...)`. This showed that the source already
observes two repositories but passes a broad delivery coordinate to a decision
consumer. Sharing a forge client or renaming the CLI first cannot settle the
subject each operation must use. /01 supplies that missing typed seam.

Independent heads are /03's compiled definition, /05's shared outcome reducer,
and /06's local transition model. `verifyoutcomes.go` says “keep the two in sync
by hand”, despite public statusgen subpackages already being importable from
`tools/desk/go.mod`; another module is therefore not the blocker for /05.

Smallest unblocking move: review the /01 subject/caller matrix against the
existing work-selection and candidate contracts. Do not wait for a new daemon,
PostgreSQL deployment, CLI migration or a blanket scanner-policy decision.

## Dependency waves

Waves below describe this authoring graph, not lifecycle or milestone acceptance.

- Wave 0: /01, /03, /05, /06.
- Wave 1: /02 after /01; /04 after /03; /07 after /06; /08 after /01.
- Wave 2: /09 after /01 and /08; /10 after /01, /03, /05 and /08.
- Wave 3: /11 after /02, /04, /05, /07, /09 and /10, and retained statusgen/37.

Critical path: **01 → 08 → 10 → 11**. A held protocol or acceptance decision
blocks only its affected consumer/cohort; independent syntax and local lifecycle
work does not inherit a whole-stream hold.

## Delivery milestones

The four proposed outcomes and their acceptance are in [scope](spec.md#proposed-delivery-milestones).
These are prose proposals while complete reader capability is unestablished;
no milestone YAML, new schema, UUID migration or eligibility change is authored.

## Implementation handoff

Use [existing-work.md](existing-work.md) and the shipped `statusgen context`
receipt under `context/`. The receipt covers declared source snapshots only;
source inspection and issue classification do not establish production root
cause. Refresh active work and actual APIs before effectful pickup. The stream
stays parked pending review; its inclusion on disk is not approval to dispatch.
