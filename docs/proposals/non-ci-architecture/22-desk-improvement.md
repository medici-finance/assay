# Desk-process improvement — future-state amendment

**Status:** draft — proposal of 2026-10-17. Adds a reviewable capability delta to the current future
architecture; it is not part of the earlier revision-bound ratification and activates nothing.
Canonical intent: [desk improvement](../../streams/desk-improvement/spec.md).
Execution decomposition: [public plan](../../streams/desk-improvement/README.md).

## Change to the future state

The existing execution/evidence/acceptance/observation path gains a complete improvement
cycle: **measure → diagnose → bounded experiment → independent assessment → authorized
decision → observed follow-up**. The domain remains continuing operations and quality.
The desk process is an enduring operational subject using existing assessment/impact identity;
the five supplied desk profiles are configurable examples, not a fixed topology.

| Boundary | Proposed obligation / owner |
|---|---|
| Runtime → evidence | Native invocation/attempt/role-subject facts, all outcomes, exact candidate/contract, usage with completeness; source owners emit and reconcile |
| Evidence → scorecard | Same versioned calculation for offline tools and API/views; eligible/observed/qualified denominators, gaps and censored/failed work |
| Card → diagnosis | Competing explanations, materiality and a discriminating test; change in workload/measurement is considered |
| Diagnosis → experiment | Frozen original inputs/labels, conditions, budgets, quality margins and stop rules before outcomes |
| Assessment → adoption | Independent result, existing human adoption/cadence; a recommendation grants no authority |
| Adoption → follow-up | Exposure and original outcome/quality measures at the declared horizon; keep/revert/hold, including rejected/inconclusive cases |

The coordinator role profile completes cycles; independent assessors challenge its own changes.
Product targets and portfolio allocation retain their existing owners. Better desk metrics
cannot rewrite a target, fill work quotas, relax acceptance or select a provider automatically.

## Runtime evidence work and ownership

| Capability | New owner brief | Existing implementation prerequisites |
|---|---|---|
| Source-to-assessment binding | desk-improvement/05 | graph-execution/06 and brief-quality/05 |
| Supervisor lifecycle/usage capture | desk-improvement/06 | /05, graph-execution/26 and /22 |
| Workflow subject/decision capture | desk-improvement/07 | /05–06 and graph-execution/21 |
| Standing desk per-item outcome capture | desk-improvement/08 | /05–06, graph-execution/27 and brief-quality/07 |
| Calculation, protocol and procedure | desk-improvement/01–04 | Offline source inspection; no adopter corpus/runtime gate |
| Read-only Improve view | desk-improvement/09 | /02 and /03 |
| Native cross-mode qualification | desk-improvement/10 | /04, /07–09 |

The existing runtime owners keep PostgreSQL coordination, transactional/recovery rules,
source IDs and authority. The new binding is a mapping manifest, not another execution
event family or store. Existing source facts are reused; absent semantics get explicit
capture in the owning module. A schema test alone cannot qualify emission. Pure reducers
and retained offline inputs remain useful before native producers land.

This target has two readiness boundaries: offline assessment availability and native-mode
measurement qualification. Execution migration retains its current gates. A deployment may
migrate a supported runtime before all optional metrics exist, but must disclose gaps and
cannot claim a measured improvement without the required evidence. Native qualification
uses actual producer code and exact build/profile versions, not fabricated normalized JSON.

## Change treatment and retirement

Existing graph-execution/06, brief-quality/05 and /07 supply source contracts; the new /05
adapts/adds compatible associations without reimplementing their lifecycle. The future
supervisor/controller/desk-client scopes are consumed before /06–08 add capture. Add source
routing notes to those unfinished plans; do not make their foundations depend on their own
measurement consumers. This prevents a dependency cycle and preserves existing Evidence.

All code, public specifications, reusable tooling and generic role procedures live in public
Assay. Adopter source inventories, private corpora, permissions, numerical budgets, actual
trials and adoption acts remain in their own boundary and consume public contract versions.
There is no dependency from the public method back to private implementation or evidence.
Retire duplicate adopter calculation/validation scripts after public replacement parity;
keep source-specific mappings and permissions with the adopter.

Semantic prerequisites: desk-improvement/01 consumes continuing-operations/01 (after
statusgen/22); /03 consumes continuing-operations/02. These are explicit build gates,
so generic source contracts are implemented once, before their assessment consumers.
