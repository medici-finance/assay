---
stream: tool-refactoring
repo: medici-finance/assay
serves: assay
status: active
priority: P1
track: platform
board: generated
spec: docs/streams/tool-refactoring/spec.md
---

# Tool refactoring — shared contracts behind the five busiest operations

The maintainer's merge of the scope-activation PR reopens all eleven briefs.
Their existing start decisions and dependency waves govern pickup. The
[scope](spec.md), [related work](existing-work.md) and [original issue
routing](issue-routing.csv) identify the shared libraries and their consumers.

## Activation and qualification

The activation depends on a lifecycle reader that accepts a routed scope and still refuses a draft. The correction is in [PR 6874](https://github.com/medici-finance/assay/pull/6874). The maintainer’s merge of that correction into the default branch precedes the merge that activates this stream. Consumers running changed-path lint with a released binary need a version that includes the correction.

Start with the four independent owners in wave 0. Obtain the brief's existing
human decision before its gated adoption. A caller migrates when its public
input, output, failure and authority checks pass on the old and new paths.

| Work | Existing owner and next observable outcome |
|---|---|
| Tracking and delivery identity | /01–02 supply the operation subject to dispatch. /09–11 check the actual PR and Evidence destinations. Keep the tracking-repo decision repair in [PR 6813](https://github.com/medici-finance/assay/pull/6813) as a regression. |
| Verification meaning | /03–05 compile the same rows and interpret the recorded result across readers. The existing statusgen/37 owns witness-v2 writers and readers; /10–11 qualify publication and acceptance. |
| Runtime portability | /06–07 qualify the local binary with isolated files and exact capabilities. The first real second environment is a Kubernetes-hosted desk owned by [desk-containers/28–29](../desk-containers/README.md). Record the released image, actual role and witnessed end-to-end result. An offline pod fixture establishes only fixture behavior. |
| Controls at publication | /08–10 consume the existing policy decision. [desk-tools/61–64](../desk-tools/README.md) owns the scanner registry and controls. Preserve both an accepted ordinary input and a refused protected counterpart on each migrated caller. |
| Recovery | /08–11 share publication preparation and reconciliation. [desk-tools/46–49](../desk-tools/README.md) own reuse, continuation, capacity and deduplication. After an interrupted publish, reconcile the observed effect before resuming the missing step. |
| API cost and capacity | [desk-tools/37–44](../desk-tools/README.md) own the review baseline and consumer adoption. Count provider calls per completed operation with identical results and permissions. Keep the cost of an unreadable, partial or changing response visible. |

The local fixtures can run independently of cluster availability. A release or
deployment owner records real Kubernetes qualification through the existing
sanctioned lane. Activation of the plan supplies no new environment access.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Bind operation subjects to explicit tracking, delivery and evidence repositories](brief-01-bind-operation-subjects-to-explicit-tracking-delivery-and-evidence-repositories.md) | 0 | M | implemented | — | — |
| 02 | [Make dispatch planning and execution consume one validated operation subject](brief-02-make-dispatch-planning-and-execution-consume-one-validated-operation-subject.md) | 1 | M | todo | — | — |
| 03 | [Compile Verify rows and declared inputs through the existing document owner](brief-03-compile-verify-rows-and-declared-inputs-through-the-existing-document-owner.md) | 0 | M | implemented | — | — |
| 04 | [Produce bounded row observations with base and head provenance](brief-04-produce-bounded-row-observations-with-base-and-head-provenance.md) | 1 | M | todo | — | — |
| 05 | [Share outcome decoding and reduction between verification producers and readers](brief-05-share-outcome-decoding-and-reduction-between-verification-producers-and-readers.md) | 0 | M | todo | — | — |
| 06 | [Model local cell lifecycle transitions using the existing cell-operation contract](brief-06-model-local-cell-lifecycle-transitions-using-the-existing-cell-operation-contract.md) | 0 | M | implemented | — | — |
| 07 | [Qualify the real cellctl binary with isolated lifecycle and environment fixtures](brief-07-qualify-the-real-cellctl-binary-with-isolated-lifecycle-and-environment-fixtures.md) | 1 | L | todo | — | — |
| 08 | [Separate immutable publication preparation from fresh effect admission](brief-08-separate-immutable-publication-preparation-from-fresh-effect-admission.md) | 1 | M | todo | — | — |
| 09 | [Migrate PR create, update and edit to the shared publication plan](brief-09-migrate-pr-create-update-and-edit-to-the-shared-publication-plan.md) | 2 | M | todo | — | — |
| 10 | [Migrate Evidence publication to bounded deltas and typed outcome admission](brief-10-migrate-evidence-publication-to-bounded-deltas-and-typed-outcome-admission.md) | 2 | M | todo | — | — |
| 11 | [Qualify the joined tool flow and prepare consumer-by-consumer retirement](brief-11-qualify-the-joined-tool-flow-and-prepare-consumer-by-consumer-retirement.md) | 3 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

The structural longest new-work chain is **01 → 08 → 10 → 11**, with
**01 → 08 → 09 → 11** in parallel. No duration estimate was made. Completion
also needs retained statusgen/37 for the v2 producer/consumer cohort.

The original scope found dispatch passing the delivery repository to the
decision gate. [PR 6813](https://github.com/medici-finance/assay/pull/6813)
corrected that caller before activation. Issue [6274](https://github.com/medici-finance/assay/issues/6274)
also identifies the verifier admission, outcome binding and publication callers.
This leaves /01 at the head of the publication chain. Recheck the remaining
callers against their actual tracking and delivery destinations at pickup.

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
cause. Refresh active work and actual APIs before effectful pickup. The source-pinned handoff records the original draft authoring. Recheck the
current source and decision for the individual brief at pickup.
