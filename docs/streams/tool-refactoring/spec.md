# Tool refactoring — class-level scope

**Status:** draft proposal, 2026-11-12. **Routes-to:** this parked stream.
This is an authoring deliverable. It authorizes no implementation dispatch,
acceptance-policy change, release, permission change or infrastructure activation.

## Outcome and evidence

The selected operations are `statusgen verifyrun`, `deskdispatch`, `cellctl`,
`deskpr` and `deskevidence`. Their open reports repeatedly cross the same seams:
which subject a command means, which definition was executed, what its result
means, who may act, and which conditional effect actually happened.

The bounded public audit contains **216 open issues** at the issue snapshot used
for this authoring: verifyrun 49, dispatch 42, cellctl 64, deskpr 23, Evidence 38.
[Routing](issue-routing.csv) retains every public issue identity, its original tool
association, a candidate failure class and owner, and an observed open repair PR
where found. Association is not root-cause proof, and reports are not independent
incidents. Some are table defects, gate decisions, adoption tasks or already
repaired behavior. This plan does not promise 216 new fixes or close any issue.

Source inspection is pinned to public main `680102a0877755feab9ef85dc4e1d3f52a137a54`. The currentness receipt
[source observations](source-observations.json) records relevant brief versions,
file digests and observed board states. The open-PR snapshot had 84 entries;
[retained work](existing-work.md) names the material overlaps. The issue snapshot
was originally collected at 2026-11-12 19:56 UTC; source and PR inspection followed
in this session. No runtime failure is claimed reproduced against production.

Quality evidence is structural and historical: `verifyrun.go` contains 3,109 lines;
row execution, expectation interpretation, Markdown and witness assembly are
coupled there. `tools/desk/internal/deskkit/verifyoutcomes.go` explicitly says the other module keeps
an independent read-half copy. `tools/desk/internal/deskkit/declaredscope.go` has its own Context grammar
although briefdoc and evaluation already expose syntax/declaration owners.
These observations nominate extraction seams; line count alone is no rewrite
criterion. Broad complexity/churn measurements and full defect de-duplication
were not established. Existing regression corpora and resolved-fix controls are
inputs; this is not an exhaustive closed-incident census.

## Selected architecture and transition

Adopt the direction in [future architecture](../../proposals/non-ci-architecture/11-target-architecture.md),
[planning model](../../proposals/non-ci-architecture/12-future-state-model.md),
[regression obligations](../../proposals/non-ci-architecture/15-regression-from-day-one.md)
and [library-first](../../library-first.md). Whole-target ratification is the
revision-bound record on medici-finance/assay#2500; later amendments retain their
own approval and qualification. This proposal changes no approved policy.

The target is a modular application around intent, admission, execution,
evidence, acceptance and observation. The five executable names are adapters,
not five semantic domains. Reuse `statusgen/workref`, `briefdoc`, `evaluation`,
`domain`, `deskcore`, `cellop` plan envelopes, existing `cellcadence` role state, the shared `workspace` contract, existing forge adapters and the runner protocol.
The desk module already imports public statusgen subpackages; statusgen cannot
import internal deskkit. An existing package name is not proof of a ready API:
each brief identifies the missing bounded seam and its migrated callers.

Transition: consolidate pure meaning and migrate current adapters first.
Preserve consequential effect-boundary rechecks, role custody and explicit
capability profiles. Keep the current CLI/record shapes unless their existing
owner approves and versions a change. There is no new service, generic workflow,
credential broker, journal, acceptance database or parallel runtime here.
PostgreSQL coordination and managed execution remain with the approved runtime
owners. One local command is not forced to await the whole future controller.
Workspace allocation identity, lifecycle generation and scratch references already
belong to `workspace/contract.go` and `spec/cell-workspaces-v1.md` under S-worktree.
Role/process state is a different type. The new operation subject references an
allocation; it cannot create a competing allocator or infer managed readiness.

The fixed-delivery proposal is **not** silently approved by this refactor.
Current policy remains until its exact governing decisions land. The D/A/W/E/H
separation is useful vocabulary, but it does not authorize old evidence reuse.
`statusgen/36–38` and `desk-tools/46–51` retain their precise scope and holds.

## Failure classes and chosen repair

| Recurring mechanism | Shared refactor | Existing work preserved |
|---|---|---|
| Commands independently choose tracking/delivery/decision/publication homes | /01 typed operation subject; /02 dispatch plan; /09–10 consumers | Shared workref/subject resolver and milestone-support/08; active issue repairs |
| Document forms disagree between lint, execution and landing | /03 compiler in briefdoc/evaluation, with adapters | statusgen/29–31 authoring/debt repair, /34 scratch, /37 v2 writes, /40 approved transitions |
| Final exit or base-red hides whether/how a check executed | /04 bounded typed base/head/stage observations | Existing bracket, row-selector and witness-v2 owners; cross-repo fixes |
| Producer and reader disagree on outcome/state/held rows | /05 canonical codec/reducer; /10 writer adapter | /46 PASS reuse, /47 continuation, /49 dedupe; no policy expansion |
| Lifecycle and tests infer owners/environment differently | /06 cadence-owned decision model; /07 actual Go-binary qualification | Credentials /06, operator roadmap and shared loop-admin/profile work |
| Publication mixes slow preparation, admission, transformation and partial effects | /08 preparation seam; /09 PR; /10 Evidence | Shared scanner /61–64, forge library /75, recovery owner and current narrow repairs |
| Locally green changes never reach all consumers | /11 joined qualification and last-consumer retirement | Existing releases, pins, verifier and operator activation authority |

## Boundaries and compatibility

An operation subject preserves existing qualified work identity and exact source,
with separate tracking, delivery, review, decision and Evidence coordinates.
Missing/unreadable source is unknown; a successful cache or dry-run grants nothing.
Row definitions come from one compiler. Observations distinguish execution
availability, functional result and closure; a row pass cannot become a verified
outcome. Intentional grammar/attestation/legacy profiles remain explicit.

Publication preparation contains immutable input bindings, never credentials or
reusable authority. Fresh admission, decision/trust, source, destination and budget
checks remain at the conditional effect boundary. Moving computation outside the
audit lock cannot weaken its serialization or cap. Unknown writes require
observation-based reconciliation, not a retry that might duplicate an effect.
A delta must preserve concurrent non-Evidence edits and historical evidence.

Wire additions route through witness-v2 and the existing outcome schema owners.
No third receipt or blanket numeric exit-code normalization. Old readers must not
silently credit records they cannot interpret. Preserve meaningful distinctions
between offline inspection, an admitted run and accepted evidence.

## Proposed delivery milestones

These are authored proposals, not operational milestone YAML. Installed statusgen
v1.0.83 supports context operations and negotiated brief-v4; the required dispatcher,
claim, acceptance and reporting cohort's complete milestone support was not established.
The briefs use supported brief-v2 identities and existing dependency edges.

| Proposed outcome | Contributors | Independently observable acceptance |
|---|---|---|
| `assay:assay:tool-refactoring:m-01@1` — one cross-repo dispatch means the same work everywhere | /01–02; /11 joined check | Decision stays in tracking repo; execution candidate and review PR stay in delivery repo; prompt retains task; wrong/missing coordinate refuses without an effect. |
| `assay:assay:tool-refactoring:m-02@1` — one verification definition and interpretable result | /03–05 and retained statusgen/37; /10–11 actual consumers | Same row/inputs through compile/run/publication/queue; base and head output bound; malformed outcome or prose-only check never earns acceptance; legacy compatibility accounted. |
| `assay:assay:tool-refactoring:m-03@1` — local lifecycle is repeatable and isolated | /06–07; /11 joined check | Actual Go binary up/plan/apply/down/up settles recorded owners, refuses stale/reused owners, exports no synthetic ambient credential, cleans up owned children only. |
| `assay:assay:tool-refactoring:m-04@1` — publication preserves concurrency and authority | /08–10; /11 | Equivalent effects are identified, relevant input change refuses, unrelated destination edits survive, global budget cannot be double-spent and uncertain provider outcomes are not replayed blindly. |

CLI/application code, libraries, Git repositories and local transitions are real
in these tests. Provider behavior is a stateful fake; harness/model output and
operator credentials are synthetic. Fixtures prove declared failure semantics,
not live GitHub/GitLab fidelity, real model behavior, cluster activation or adoption.
Replacing those substitutes requires the existing provider/profile owners' real
qualification and fresh accepted evidence for that environment. Release manifests
state those limits; no mocked success is promoted to deployed readiness.

## Proof, gates and pickup

Every brief contains a real-caller flow and a second-site class guard/positive
plant. The implementation records pre-fix red and post-fix green. The named tests
are future deliverables, not tests that exist at authoring. Require the named PASS
line as well as exit 0 to reject a vacuous selector. Existing stronger custody,
protected-acceptance and provider fences remain independently tested by bypassing
the upper planner. The joined check /11 tests actual migrated consumers.

Each unit is M except /07's L real-binary/oracle harness cohort. The refactors touch repository binding, verification admission/outcomes, role
process ownership or publication authority; all eleven are human-gated with
self-contained start decisions. The stream is parked: pending
implementation decisions do not prevent reviewing these concrete drafts, and the
proposal does not acquire veto authority over existing approved work.

At pickup/resume/publication, refresh affected source, issue/PR and claim state,
check current approved target/contract revisions and record continue/adapt/hold.
Fresh active claims were not enumerated here. No executing Task/Verify contract
is rebound. The only proposed existing-brief edit is statusgen/37's inverse
unblocks edge to /11; its Task, version, sources and Evidence are preserved.
Any later material amendment must include the owner checkpoint/acknowledgement
or explicit cancellation/reconciliation. Keep relevant unknowns scoped.

No performance improvement or new latency budget is promised from a source
refactor. Retain current output/time/memory bounds and existing lock deadlines.
Measure same-workload preparation, lock-held time, complete producer/consumer
coverage and repeat attempts through perfrec where available. Acceptance must
include no lost checks or changed authority; throughput alone cannot pass it.
