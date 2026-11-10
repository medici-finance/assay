---
brief: assay:assay:graph-execution:16
title: Cell ownership, cumulative budgets and restoration fencing
why: Recovery can repeat effects or restart a paused Cell if ownership, budgets and stop state exist only in process memory. Durable state must constrain the old worker as well as its replacement.
wave: 5
depends:
- graph-execution/04
- graph-execution/09
- graph-execution/28
- graph-execution/29
unblocks:
- graph-execution/14
- graph-execution/25
- graph-execution/19
- graph-execution/26
- "forge-neutral/40"
- performance-collection/09
effort: L
gate: human
risk:
  regulatory: no
  customer: no
  irreversible: yes
  sensitive-data: no
gate-why: Ownership and restore behavior constrain external effects; the owner confirms stale writers cannot retain independent authority.
decision-trigger: spec
design: DR-graph-exec-16
decision-issue: 5111
ruling: "https://github.com/medici-finance/assay/issues/5111#issuecomment-6612666889"
issues: []
schema: brief-v2
authored: 2026-09-19 by Codex (author-brief)
sources:
- docs/streams/graph-execution/work-input-amendment.md
- freshness-checked 2026-09-30 @ 8485778515c041fc87966902a14eb9d195492be3 (pending scope, not implementation)
- docs/streams/graph-execution/admission-assurance-spec.md
- freshness-checked 2026-09-18 @ 951ca784d100a7d201a28a34033da6709ec2ec8f
- docs/streams/graph-execution/task-workflow-program.md — execution routing amendment 2026-10-02
- freshness-checked 2026-10-02 @ 307fe16992caef53fa46c52622753dd400c7b42a
- docs/proposals/non-ci-architecture/20-amendment-manifest.md — R2 owner row and R2 amendment scope (operator incarnation binding)
- docs/proposals/non-ci-architecture/14-storage-forge-graph.md — lines 50-56 (fresh authority incarnation and key; held outside PostgreSQL; mismatch starts paused)
- docs/proposals/non-ci-architecture/15-regression-from-day-one.md — lines 51, 58 (rollback with a surviving old capability; failover with stale generation)
- docs/proposals/non-ci-architecture/16-delivery-sequence.md — line 56 (R2 operator incarnation binding)
- "medici-finance/assay#2500 — maintainer ratification of the whole target @ 23f655f5d (approval activates nothing)"
- freshness-checked 2026-10-13 @ 4696d598e58820806d0bcf8c9b2dd87f29115400
exec-tier: strong
exec-tier-why: Cross-component contracts and independent failure controls must agree; the implementation requires design judgment.
domain: complicated
consumers:
- 'drainloop: fixed-here'
- 'tools/desk: follow-up graph-execution/14'
- 'workflow/store: follow-up graph-execution/19'
- 'workflow/controller: follow-up graph-execution/21'
- 'spec/operator-incarnation-v1.md: fixed-here (record contract landed by the 2026-10-31 schema/test preparation; enforcement remains owed)'
- 'loopadmin supervisor launch admission (checks the current incarnation): follow-up graph-execution/26'
version: 8
id: db922112-28f0-44c0-b120-9d6c4f56e103
---

# Brief 16 — Cell ownership, cumulative budgets and restoration fencing

## Context

Schema/test preparation (2026-10-31) added `spec/cell-checkpoint-v1.md`, `drainloop/testdata/cell-checkpoint.json`, `drainloop/testdata/effect_mutations.py` and `docs/superpowers/plans/2026-10-31-ge16-schema-tests.md` to the `files:` line below. These are partial preparation; all original delivery obligations remain.

files: `drainloop/ownership.go`, `drainloop/ownership_test.go`, `drainloop/budget.go`, `drainloop/budget_test.go`, `drainloop/restore_test.go`, `drainloop/checkpoint_decode_test.go`, `drainloop/README.md`, `docs/enforcement-model.md`, `changelog/graph-execution-16-cell-ownership-budgets.md`, `spec/operator-incarnation-v1.md`, `drainloop/incarnation.go`, `drainloop/incarnation_test.go`, `spec/cell-checkpoint-v1.md`, `drainloop/testdata/cell-checkpoint.json`, `drainloop/testdata/effect_mutations.py`, `docs/superpowers/plans/2026-10-31-ge16-schema-tests.md` — Added by the R2 amendment (2026-10-13): `spec/operator-incarnation-v1.md`, `drainloop/incarnation.go`, `drainloop/incarnation_test.go`.

facts: 04 owns basic effect receipts and reconcile-before-retry. This extension adds ownership/budgets without replacing that protocol or widening the six-method Loop interface.

single-point-of-failure: the new contract or policy alone cannot establish safe execution — independent boundary enforcement and independently read fixture/evidence results must still reject a bypass.

## Human decision

Decision-trigger: spec. At implementation pickup, prepare concrete policy choices and negative-path evidence, then file a self-contained decision issue. No response permits no activation; schema/test work may proceed within the declared scope.

### Schema/test preparation — 2026-10-31

This preparation adds passive record contracts, an operator-current-value read port
and offline negative fixtures. It does not deliver this brief. The existing Task,
Verify rows 1–11, Evidence, dependencies and human activation gate remain owed.
No generated board status or lifecycle completion is changed.

#### Human decision block — ruled: option 3, design only

**Ruling recorded.** Option 3, design only: custody 1A, reopen 2A, budget 3A,
provider 4A, and pacing rule 5 unchanged. Two restrictions apply to 1A: role
executors cannot read signing material on the volume they mount, and snapshots or
backups of that volume never restore operator-control state as current. Both are
contract text in `spec/operator-incarnation-v1.md`, each pinned by a negative-control
test and committed mutants. The accountable operator for the concrete profile is
the maintainer the ruling names, `human:ian`. The ruling grants no deployment,
credentials or admission reopening; activation still needs the separate human gate.

Ruling: https://github.com/medici-finance/assay/issues/5111#issuecomment-6561259515

The options as offered:

**Decision:** approve a bounded first Kubernetes operator profile for subsequent
implementation and qualification of ownership, budgets and restoration fencing.
A response authorizes only the selected design scope; activation still requires
the existing separate human gate and consumer qualification. No response leaves
mutation admission paused. Scope is one Cell execution/publisher group; automatic
multi-host recovery and R3 repository assignment remain excluded.

Responses offered:

1. **Approve the recommended bounded design profile:** custody 1A, reopen 2A,
   budget 3A, provider 4A, and pacing rule 5 unchanged, as detailed below. This
   approves subsequent implementation/qualification only, never live activation.
2. **Hold:** leave all policy choices pending and affected mutation admission paused.
3. **Approve a narrower or explicit alternative design:** name the selected dimensions
   and restrictions below; every unspecified dimension stays held. This also grants
   no deployment, credential provisioning or reopening authority.

Default if no answer (superseded by the ruling): none — blocks until answered; keep all policy choices pending and mutation admission paused; schema/test preparation alone may proceed.

**Dimension 1 — Incarnation custody and rotation (1A / 1B).** Recommend protected operator-control state
   on a dedicated persistent volume outside PostgreSQL/CNPG backup and PITR, mounted
   only by the trusted operator and qualified role executors; mutation is restricted
   to the operator, with signing material unavailable to model/runner containers.
   The profile must pin its Kubernetes namespace, execution-group identity, volume,
   access identities and key reference before implementation activation. Reconstruct,
   PITR rollback and transfer each rotate identity AND key, even if restored SQL
   matches an old generation. Alternative: a separately qualified external secret
   authority with equivalent non-restorable-current semantics. A Git revision or
   restored database value is never current authority. This choice creates no
   Kubernetes resources or credentials.
   Ruled restrictions on 1A: (a) role executors never read the signing material
   on the volume they mount, and only the operator writes control state; (b) a
   snapshot or backup of the volume never restores operator-control state as
   current, so the operator rotates identity and key away from every restored
   value before a match counts.
**Dimension 2 — Reopen procedure (2A / 2B).** Recommend operator-confirmed whole-group stop plus old
   socket/capability invalidation and independent credential revocation/restriction
   (or expiry), followed by external reconciliation, rotation and fresh executors.
   Persist a scoped stop-confirmation reference and the new activation binding.
   Unknown stop, incomplete credential inventory, missing operator state or a
   mismatched activation remains paused. Alternative: require full manual teardown
   and reconciliation on every transfer until the automated group-stop boundary is
   qualified. Neither permits an SQL lease or process list to stand in for fencing.
**Dimension 3 — Budget and runner boundary (3A / 3B).** Recommend no conservative fallback for unavailable
   brakes: hold until authoritative usage/reservations are recovered or a separately
   authorized reset is recorded. Keep account/provider/unit/window balances separate,
   with configured operator headroom and outstanding unknown reservations charged.
   All preparation, summary, failed, fallback and verification attempts charge
   original work; pricing/model identities remain references. Only a request-limiting
   runner may claim a hard per-request cap; launch-only control must reserve a proven
   request upper bound or hold. Alternative: a separately reviewed per-profile
   conservative upper bound, never an assumed zero or cross-provider fallback.
   Synthetic fixture values (100 units, 10 headroom, 30 reserved) are not live policy.
**Dimension 4 — Unknown effects and handback (4A / 4B).** Recommend retain old ownership restrictions and
   unknown-effect reservations until authoritative reconciliation and independently
   fenced old writers permit transfer. Providers lacking enforceable dedupe/fencing
   or confirmed old-writer stop stay held for authorized resolution. Alternative:
   support those providers in report-only mode with no mutating cohort. Neither
   claims exactly-once or releases unknown usage because a process restarted.
**Dimension 5 — Pacing boundary (unchanged).** Retain the already accepted brake/pacing distinction: unknown
   brakes hold; effect-class pacing re-derives shared window intents/receipts on every
   admission, read-class seeds retained shared state, with only one recorded unseeded
   reset per window/scope/incarnation. Missing/second reset exhausts that window.
   No relaxing alternative is proposed under this gate.

**Exact response:** name this brief in the reply. Reply `graph-execution/16: 1`
to approve only the recommended bounded design (1A/2A/3A/4A + 5 unchanged),
`graph-execution/16: 2` to hold, or `graph-execution/16: 3` followed by the
explicit alternative selections/narrowed scope and rationale. A bare option
number does not name the brief and is not recorded as a ruling. For either
approval, name the operator accountable for the concrete profile and any
restrictions on the selected options. Missing or ambiguous selections
stay held. No reply authorizes deployment, credentials or admission reopening.

**Evidence available for the ruling:** implementation tests
`TestCellFenceBoundary`, `TestIncarnationContractFields`,
`TestCellCheckpointRoundTrip`, `TestCheckpointAbsentFieldsHeld`,
`TestExecutorKeyReadRefused` and `TestRestoredControlNotCurrent`, with a
committed negative mutation control per guard, exercise the pure contract and
separately invoked fake effect boundary. They cover restored owner/generation
under an old incarnation, stale tuple, absent or unsupported protected current
state, activation/key mismatch, contract wire names, refusal of a checkpoint with
any field absent, executor signing-material reads, restored control state
presented as current, retained pause/cancellation/unknown usage and all attempt phases. They do not
qualify Kubernetes containment, signature custody, real provider enforcement,
atomic reservations, concurrent owners or PostgreSQL rollback. Those checks remain
could-not-check in this preparation and required before activation. Independent
verification has not been performed and no Verify Evidence is recorded here.

**Remaining delivery:** complete Task 1–5 and all original Verify rows; couple
reservations/ownership/intent at the existing authority; implement real adapter
conformance, concurrency/crash/restore/handback and brake/pacing tests; qualify
credential cutover and all consumers. GE/19 owns real PostgreSQL rollback; GE/26
owns launch enforcement. The preparation includes the incarnation and checkpoint
contracts described above; neither activates a runtime consumer.

## Read first

- [Admission and assurance amendment](admission-assurance-spec.md).
- [Stream specification](spec.md) and the typed prerequisites above.

## Ground rules

- Work in an isolated branch and draft PR under repository rules; no merge, deployment or live infrastructure query.
- Stop at implemented; independent verification owns verified/done.
- Existing authority and human gates remain binding. Missing prerequisite evidence is could-not-check.
- Public fixtures use example-org and synthetic data; do not copy adopter evidence.

## Work-input amendment — 2026-09-30

Implement WI-3/WI-4 by extending this brief's durable reservation seam, not by adding a meter
service. Persist account/provider unit, model/pricing identity, attempt/request lineage,
raw usage, uncertainty and outstanding reservations. Charge preparation, summaries, failed
attempts, fallback and verification to the original work. Preserve missing telemetry as
unknown and budgets/stops across restart; test concurrent admission against one balance.

Keep quota units separate and make rolling/longer-period constraints and operator headroom
expressible. Fake-provider fixtures prove accounting/concurrency semantics only. Actual
model adapters and plan calibration remain adopting-pilot work. State whether a runner can
limit requests or only launches; a late alarm is not hard enforcement. No live provider
calls, fixed commercial rates or silent cross-provider fallback are added here.

## Task workflow amendment — 2026-10-02

Persist backend ownership/transfer generation, outstanding launch reservations, spend and stop state. Handback requires drain/reconciliation and independently fenced old writers. Missing confirmation of an old effect blocks conflicting admission. Use the existing ownership authority across hosts; a local workflow database is not a second lease authority.

Implement the named failure/flow case below in the declared test surface. This amendment does not record implementation evidence or authorize live activation.

## Non-CI R2 amendment — 2026-10-13 (operator incarnation binding)

The maintainer ratified the non-CI target (medici-finance/assay#2500, revision 23f655f5d);
approval activates nothing. Owner row: package R2 in the
[amendment manifest](../../proposals/non-ci-architecture/20-amendment-manifest.md). The
2026-10-02 amendment above stands; this section adds the incarnation that makes it hold
across database deletion and rollback.

- **What is added.** Every ownership generation is valid only inside one **operator
  incarnation**: a fresh random identity and capability-signing key minted on every complete
  reconstruction, point-in-time rollback and ownership transfer (target 14:50). The trusted
  operator holds the current incarnation outside PostgreSQL, in protected host control state
  beside the repository assignment; a database backup or a Git configuration revision cannot
  restore an old incarnation as current.
- **Where it is checked.** At the effect boundary this brief already fences: every new effect
  admission checks the current incarnation together with owner and generation, so a stale
  queued request or capability fails even when its old SQL row and generation reappear
  (14:52, 15:51). Evidence keeps the old incarnation as provenance without reactivating it.
- **Start paused.** A mismatched or missing incarnation, or a missing activation record,
  starts admission paused (14:56). Restore rotates the incarnation and key and requires the
  operator's confirmation that the whole execution group was stopped before admission
  reopens; a list of attempts recovered from SQL is not that confirmation (14:52).
- **Contract.** `spec/operator-incarnation-v1.md` (planned) states the incarnation fields,
  the rotation triggers, the paused-start rule and how a consumer reads the current value.
  graph-execution/19 stores the activation record; graph-execution/26 checks it before a
  launch. Neither imports `drainloop`; they conform to the contract.
- **Unchanged.** The drainloop seam stays store-neutral (fake stores in this brief's tests);
  the real PostgreSQL rollback proof is graph-execution/19 row 7. Read Task 4's "not assumed from
  SQLite transactions" as "not assumed from database transactions": the runtime store is
  PostgreSQL (graph-execution/19's R2 amendment), and its transactions prove no fencing.
- Risk answers and gate unchanged (`irreversible: yes`, human). The decision issue filed at
  `spec` pickup now also carries the incarnation storage location and rotation procedure.

## R3 follow-up — not in this amendment

Package R3 later amends this brief for **exclusive repository assignment**: operator admission
rejects overlapping mutating assignment of one external repository before capabilities issue
(15:49). That is not authored here; do not implement it under these rows.

## Operation-count accounting amendment — 2026-10-15

Apply the target's [brake and pacing contract](../../proposals/non-ci-architecture/11-target-architecture.md#pacing-counts-are-best-effort-brakes-fail-closed).
This brief's cumulative attempt budgets, reservations and any per-target or per-repository
write ceiling it admits against are **brakes**: they stay fail-closed. When such a count is
unavailable or unverifiable, admission holds or applies a declared conservative fallback;
it never assumes the actor is under budget. Rows 4 and 5 keep their exact contracts.

Only **pacing counts** (desk/tool throughput pacing of already-authorized work) are best
effort. They need no exact budget-slot reservation, global lock across network work or
complete audit reconstruction. Their estimate is shared across processes, never
per-process. An effect-class count (its admissions leave a durable admission record) is
always re-derived before each admission from the window's recorded effect intents and
receipts; a cached pacing row is only an optimisation and never the source. A read-class
count is seeded from its last retained count. Neither starts from zero. When neither can be read,
one recorded reset per window and operator incarnation is allowed; a second treats the
window as exhausted. Effect-class counts are bounded by cap + C, or 2 × (cap + C) after the
reset; read-class counts have no numeric bound. A missing estimate is reported as
unavailable, not measured zero.

Ownership generations, cancellation/stop state, hard resource/time/cost limits and
unresolved-effect reservations keep their contracts. Qualify against target doc 15's
§Operation-count qualification cases; rows 10 and 11 below carry that obligation. They are
red until the named tests exist. Relaxing a brake is a separate human decision; this
amendment relaxes none. No completed Evidence or current runtime behavior changes.

## Task

1. Define durable reservation and ownership-generation seams, cancellation intent, cumulative attempt budgets and stop/override retention. Use existing storage where possible; adapter conformance must reject non-durable required stores.
2. Fence stale workers at the effect boundary. Require cohort cutover to remove legacy direct credentials or prove equivalent provider restrictions; a database lease alone is insufficient. Keep provider outcome unknown until authoritative reconciliation.
3. Add crash/restore/concurrent-owner fixtures with a separately enforcing fake provider. Bypass the scheduler to prove stale-generation rejection at the provider boundary; preserve pauses and unknown effects across restore. No production credentials or cluster calls.
4. Document limitations for providers without fencing or idempotency: hold and require authorized resolution, never claim exactly-once. Restore/failover is a tested adapter capability, not assumed from SQLite transactions.
5. (R2, 2026-10-13) Write `spec/operator-incarnation-v1.md` (planned) and extend the ownership-generation seam so effect admission checks the current operator incarnation with owner and generation. Mint a fresh incarnation and key on reconstruction, rollback and transfer; start paused on a mismatched or missing incarnation; keep the old incarnation as evidence provenance only. Prove it at the effect boundary with the scheduler bypassed. Add a test that resolves the contract's named fields against the incarnation type.

## Interface contract

Persisted Cell state → restart → admission/effect boundary preserves budget and stop state while rejecting a stale writer.

Every shared consumer above must be reconciled against the implementing diff. Planned commands/tests below are deliverables, not claims that they already exist. Update the declared documentation and changelog with the implementation.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd drainloop && GOWORK=off go test -count=1 -v -run "^TestCellOwnership" ./...` | exit 0; output includes PASS for TestCellOwnership, with no [no tests to run] for its owning package |
| 2 | check:ci +mutation | `cd drainloop && GOWORK=off go test -count=1 -v -run "^TestCellOwnershipStaleWriterDenied" ./...` | exit 0; output includes PASS for TestCellOwnershipStaleWriterDenied, with no [no tests to run] for its owning package |
| 3 | check:ci +flow | `cd drainloop && GOWORK=off go test -count=1 -v -run "^TestCellOwnershipRestoreBudgetPause" ./...` | exit 0; output includes PASS for TestCellOwnershipRestoreBudgetPause, with no [no tests to run] for its owning package |
| 4 | check:ci +mutation | `(cd drainloop && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCellBudgetConcurrentReservations.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCellBudgetConcurrentReservations$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCellBudgetConcurrentReservations " "$wi_out")` | exit 0; named PASS; concurrent admissions cannot spend the same available allowance; mutation: exclude outstanding reservations from available allowance — the named test must fail |
| 5 | check:ci +flow | `(cd drainloop && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCellBudgetRestoreAllAttempts.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCellBudgetRestoreAllAttempts$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCellBudgetRestoreAllAttempts " "$wi_out")` | exit 0; named PASS; failed-attempt spend, reservations, unknown usage and stop state survive restore |
| 6 | check:ci +flow +mutation | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestOwnershipBackendHandbackUnknown$" ./... > "$routing_out" && grep -q -- "--- PASS: TestOwnershipBackendHandbackUnknown " "$routing_out")` | exit 0; named PASS; transfer with unknown effect holds, restore retains usage/stop, stale backend cannot write; mutation: release ownership on handback while an effect is unknown — the named test must fail |
| 7 | check:ci +flow +mutation | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestIncarnationStaleRefused$" ./... > "$routing_out" && grep -q -- "--- PASS: TestIncarnationStaleRefused " "$routing_out")` | exit 0; named PASS; with the scheduler bypassed, a capability minted under an earlier incarnation is refused at the effect boundary even when its owner and generation reappear from a restored store, and no effect is applied; mutation: compare only owner and generation, ignoring the incarnation — the named test must fail |
| 8 | check:ci +flow +mutation | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestIncarnationMismatchPaused$" ./... > "$routing_out" && grep -q -- "--- PASS: TestIncarnationMismatchPaused " "$routing_out")` | exit 0; named PASS; a missing or mismatched operator incarnation starts admission paused, a store snapshot cannot set the current incarnation, and reopening requires rotation plus confirmed execution-group stop; mutation: take the current incarnation from the restored store's activation record — the named test must fail |
| 9 | check:ci +dereference +mutation | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestIncarnationSpecMatchesFields$" ./... > "$routing_out" && grep -q -- "--- PASS: TestIncarnationSpecMatchesFields " "$routing_out")` | exit 0; named PASS; every field `spec/operator-incarnation-v1.md` (planned) names exists on the incarnation type in `drainloop/incarnation.go` (planned), and every exported field is named in the contract; mutation: rename one field in the contract text — the named test must fail |
| 10 | check:ci +flow +mutation | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCellBrakeCountUnavailableHolds$" ./... > "$routing_out" && grep -q -- "--- PASS: TestCellBrakeCountUnavailableHolds " "$routing_out")` | exit 0; named PASS; with the pacing path bypassed, an unavailable or unverifiable cumulative-budget or per-target write-ceiling count holds admission and no effect is applied at the fake provider; mutation: treat an unavailable brake count as under budget — the named test must fail |
| 11 | check:ci +flow +mutation | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCellPacingRestartNoMultiply$" ./... > "$routing_out" && grep -q -- "--- PASS: TestCellPacingRestartNoMultiply " "$routing_out")` | exit 0; named PASS; with at least two processes and a seed at most half the cap, after the shared pacing counter of an effect-class count is lost mid-window, K restarts and N processes re-derive the estimate from recorded intents and receipts before each admission, so every process's admissions are counted, and admit at most cap + C, or 2 × (cap + C) after one recorded reset; with intents committed but every pacing-row update skipped, admissions still stay within cap + C; a second unseeded reset, or an absent reset record, treats the window as exhausted; mutation: start the fallback estimate at zero, keep a per-process estimate that ignores other processes' admissions, or admit against the cached pacing row instead of the re-derived estimate — the named test must fail |


The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Store-neutral implementation — 3 November 2026

The library implements `CellAuthority`, `CellStore`, `CellJournal`, authenticated
capability binding, all-balance reservations, retained usage settlement and held
transfer. `CellJournal` consumes the existing transactional handoff and the frozen
Loop/Config/effect flow unchanged. `CellStore` is the existing coordination owner:
its compare/commit must publish ownership, reservation and intent together, or
publish none. No runtime backend or additional claim authority is installed.

`CheckEffect` belongs inside the provider's independently enforcing atomic effect
boundary, with current protected incarnation, authenticated capability and matching
intent/reservations. A wrapper that checks and then invokes an unrestricted write
is not a conforming provider. Required durability, current operator custody,
authentication/expiry/policy, credential cutover, reset history and authoritative
usage are explicit refusing ports, not inferred from record contents.

Offline qualification uses a test-only local-filesystem transactional fixture and
a separately invoked fake provider, plus actual child processes sharing one state
file and an independent reset anchor. These qualify library semantics and the
fixture's acknowledged sync/rename behavior only. GE/19 still owes PostgreSQL
transactions/rollback/conformance; GE/26 and the effect consumers still owe their
concrete launch/provider integration. Real control-volume custody, provider fencing
and credential cutover remain could-not-check until those profiles are qualified.
No runtime consumer, activation, signing material or deployment is supplied.

The group ownership and per-item claim generations are distinct. The existing
claim authority retains exact work/attempt/owner bindings in `CellState.Claims`;
admission and the separately invoked effect boundary reject stale or released item
claims. Crash-before-apply and proven-failure retirement require authoritative absence,
independent old-writer fencing and complete known usage through an explicit port;
unknowns stay held. A fresh attempt/request may then resume in the same claim
generation without losing failed spend or creating a free retry. Provider requests
bind one signed phase, and budget-window checks use an injected authority clock.
The original eleven Verify Class/Command/Expect rows are unchanged. Their
implementation tests are not independent acceptance Evidence or an activation ruling.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->
### Non-implementer verifier run — 2026-11-04 — claude-opus-5-5 verifier (verify-desk dispatch), merged main db877f68ef27

Runner is not the implementer of PR #5386 (delivery merge 9ec2680d0d24, an ancestor of the verified commit) or of the earlier preparation PR #5113. Offline envelope (KUBECONFIG=/dev/null), own detached worktree at merged main db877f68ef273e5796df5ddf92e89b0d5bc05ffe, admitted by deskdispatch --check-verifier (exit 0 before and after the run). gate: human; risk irreversible: yes. All 11 rows are check:ci Go tests in the drainloop package and need no host capability, so the whole table ran in one closed admission bracket: statusgen verifyrun --sandbox=container-netns in a --network none container from the pinned verify-runner v1.0.73 image (sha256:100dde7f1f3a), home mounted read-only, bracket-digest verdict=match. The host row selection is empty; the two selections are disjoint and cover rows 1-11.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd drainloop && GOWORK=off go test -count=1 -v -run "^TestCellOwnership" ./...` | pass exit=0 sandbox=container-netns | sha256:206f373e4eff | 2026-11-04 | assay-verifier-app[bot] @ db877f68ef27 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd drainloop && GOWORK=off go test -count=1 -v -run "^TestCellOwnershipStaleWriterDenied" ./...` | pass exit=0 sandbox=container-netns | sha256:80876d04733f | 2026-11-04 | assay-verifier-app[bot] @ db877f68ef27 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd drainloop && GOWORK=off go test -count=1 -v -run "^TestCellOwnershipRestoreBudgetPause" ./...` | pass exit=0 sandbox=container-netns | sha256:0c6bfda9a9d2 | 2026-11-04 | assay-verifier-app[bot] @ db877f68ef27 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd drainloop && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCellBudgetConcurrentReservations.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCellBudgetConcurrentReservations$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCellBudgetConcurrentReservations " "$wi_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-04 | assay-verifier-app[bot] @ db877f68ef27 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd drainloop && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCellBudgetRestoreAllAttempts.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCellBudgetRestoreAllAttempts$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCellBudgetRestoreAllAttempts " "$wi_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-04 | assay-verifier-app[bot] @ db877f68ef27 (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestOwnershipBackendHandbackUnknown$" ./... > "$routing_out" && grep -q -- "--- PASS: TestOwnershipBackendHandbackUnknown " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-04 | assay-verifier-app[bot] @ db877f68ef27 (on-behalf-of human:ian) (forge-identity) |
| 7 | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestIncarnationStaleRefused$" ./... > "$routing_out" && grep -q -- "--- PASS: TestIncarnationStaleRefused " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-04 | assay-verifier-app[bot] @ db877f68ef27 (on-behalf-of human:ian) (forge-identity) |
| 8 | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestIncarnationMismatchPaused$" ./... > "$routing_out" && grep -q -- "--- PASS: TestIncarnationMismatchPaused " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-04 | assay-verifier-app[bot] @ db877f68ef27 (on-behalf-of human:ian) (forge-identity) |
| 9 | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestIncarnationSpecMatchesFields$" ./... > "$routing_out" && grep -q -- "--- PASS: TestIncarnationSpecMatchesFields " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-04 | assay-verifier-app[bot] @ db877f68ef27 (on-behalf-of human:ian) (forge-identity) |
| 10 | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCellBrakeCountUnavailableHolds$" ./... > "$routing_out" && grep -q -- "--- PASS: TestCellBrakeCountUnavailableHolds " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-04 | assay-verifier-app[bot] @ db877f68ef27 (on-behalf-of human:ian) (forge-identity) |
| 11 | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCellPacingRestartNoMultiply$" ./... > "$routing_out" && grep -q -- "--- PASS: TestCellPacingRestartNoMultiply " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-04 | assay-verifier-app[bot] @ db877f68ef27 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:90feb2f363705b5697e93fb395a6f3dfedb537b0b501a6fe4bdeb15ccf91818c
Container-Image: sha256:100dde7f1f3a8be53ae45e9c1ff21fcd9fc82760595c91c422670db36c25d8d9
Verification-Attestation: medici-finance/assay#5454 run=198ede9a5dc8809793ba34683f54130d2b613a5721f4c3f61a4aae019e04710b source=db877f68ef273e5796df5ddf92e89b0d5bc05ffe brief=docs/streams/graph-execution/brief-16-cell-ownership-budgets.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=198ede9a5dc8809793ba34683f54130d2b613a5721f4c3f61a4aae019e04710b source=db877f68ef273e5796df5ddf92e89b0d5bc05ffe tree=sha256:90feb2f363705b5697e93fb395a6f3dfedb537b0b501a6fe4bdeb15ccf91818c witness=sha256:693025ccc3347855bfb1caf1ef106569ad156f36b489432862c8de16326fb8da opened=2026-11-04T19:50:19Z closed=2026-11-04T19:50:46Z before=receipt after=check-verifier

Observed output (supplemental, no acceptance credit; the exact row selectors re-run on an archived copy of the verified commit in the same image, logs kept outside the home): every named test prints its "--- PASS" line and the owning package prints "ok github.com/medici-finance/assay/drainloop"; the cmd/demo package prints "[no test files]", so no selector matched zero tests. Hand-read against each Expect: row 4 admits exactly 3 of 24 concurrent reservations (100 limit, 10 headroom, 30 reserved); row 11 uses two processes, a seed of 3 against a cap of 10 with C = 2, keeps 7 + 3 + 10 = 20 admissions after the one recorded reset, within 2 × (cap + C) = 24, and an anchor of 2 or a missing anchor admits nothing.

Mutation probes (supplemental, no acceptance credit; independent mutants built on the archived copy, never in the home). Killed means the row's named test failed:
- M4 outstanding reservations left out of the charge: row 4 killed (admitted=24).
- M5 unknown reservation ignored in Transfer, M6 handback with an unknown effect allowed: row 6 killed both.
- M7 owner and generation compared, incarnation ignored: row 7 killed ("restored SQL tuple revived old incarnation").
- M8 current incarnation taken from the store activation: row 8 killed.
- M9a contract Go name renamed, M9b wire name renamed: row 9 killed both.
- M10a unavailable brake count treated as zero at admission, M10c treated as known at admission and effect: row 10 killed both.
- M11a seed of zero, M11b per-process estimate, M11c cached pacing row as source: row 11 killed all three (intents 10, 28, 48).
- M2 presented generation ignored in Match Cell Fence alone survived row 2, because the item-claim owner check rejects the same stale writer; M2b with both guards removed is killed by row 2 ("scheduler bypass applied stale writer"). Defence in depth, not a gap.
- M10b effect-boundary brake check removed alone survives row 10's test (its apply step already fails on a missing admitted intent) but is killed by row 1's restored-records unknown-brake subtest.

Risk-bearing value (kit section 4). Enumeration over the delivery diff (9ec2680d: drainloop budget.go, ownership.go, incarnation.go and the two specs). Test fixture values (100, 10, 30, Cap 10, C 2, seed 3, 24 × 30) are test-only and the brief marks them as not live policy; Cap, C, Limit and Headroom are configuration-owned, with no production default in code.
- epoch bound p.Epoch > 1 (and p.Epoch != epoch) @ drainloop/budget.go:157
- pacing admission n >= p.Cap @ drainloop/budget.go:177
- budget admission used > found.Limit after charging Headroom and outstanding reservations @ drainloop/budget.go:110-120
- launch-only control ControlBoundary "request" / "launch" with UpperBoundRef required @ drainloop/ownership.go:328
- item claim count matches != 1 @ drainloop/ownership.go:724
- overflow guards math.MaxUint64 @ drainloop/budget.go:104 and :166, ^uint64(0) @ drainloop/ownership.go:697
- pacing classes "effect" / "read", other class refused @ drainloop/budget.go:162-175
- charged phases "preparation", "summary", "execution", "fallback", "verification" @ drainloop/ownership.go:324 and :634
- control roles ControlOperator = "operator", ControlExecutor = "executor" @ drainloop/incarnation.go:62-63
- IncarnationSchema = "operator-incarnation/v1" @ drainloop/incarnation.go:9; CellCheckpointSchema = "cell-checkpoint/v1" @ drainloop/ownership.go:18

Ranking: the irreversible act is a provider spend or an external effect admitted past a brake, or by a stale owner. The epoch, cap, limit, boundary and claim-count bounds decide that and rank first. Overflow guards, class and phase sets fail closed (an unknown value is refused) and rank next. The schema tags and role labels are wire identifiers whose mismatch refuses, undone by an edit and redeploy, and rank last.

RISK-VALUE: DERIVED — epoch bound p.Epoch > 1 @ drainloop/budget.go:157 — the amendment allows "one recorded reset per window and operator incarnation; a second treats the window as exhausted"; epochs 0 and 1 admit and 2 or more refuse, and the caller's epoch must equal the recorded one; this is what gives the 2 × (cap + C) ceiling (M11a and row 11).
RISK-VALUE: DERIVED — pacing admission n >= p.Cap @ drainloop/budget.go:177 — the count includes the shared seed plus every distinct recorded effect intent, so admission stops once Cap are counted; that is tighter than the brief's cap + C bound (C only absorbs cross-process lag and grants no slot), and with one reset it stays within 2 × (cap + C).
RISK-VALUE: DERIVED — budget admission used > found.Limit @ drainloop/budget.go:120 with Headroom charged first @ drainloop/budget.go:110 — Dimension 3 requires "configured operator headroom and outstanding unknown reservations charged"; consumed + headroom + outstanding + new must not exceed the limit, so spending exactly to the limit is allowed and one unit more is refused; a missing consumed count refuses ("brake usage unavailable") rather than reading as zero (M4, M10a).
RISK-VALUE: DERIVED — ControlBoundary "request" or "launch" with UpperBoundRef @ drainloop/ownership.go:328 — Dimension 3: "only a request-limiting runner may claim a hard per-request cap; launch-only control must reserve a proven request upper bound or hold"; launch without an upper-bound reference is refused, and any other boundary is refused.
RISK-VALUE: DERIVED — item claim count matches != 1 @ drainloop/ownership.go:724 — exactly one live claim for the work item, owned by the current fenced owner and generation, must back an effect; zero (released or transferred) or two (ambiguous) refuse, so a restored Cell tuple cannot revive a released claim (M2b).

Observations (outside the Verify table, for the desk to route; none fails a row):
1. Row 10's named test does not tell apart the effect-boundary brake check on its own (M10b); row 1 covers it. A row-10-local assertion would make that row self-sufficient.
2. Row 10's Expect mentions a per-target write-ceiling count. The implementation admits against no per-target or per-repository ceiling, so that clause has nothing to check at this commit.
3. The admitted container run passed GOFLAGS=-mod=mod, which was unnecessary (drainloop has no module dependencies); the tree digest was identical before and after.

**VERIFY: PASS** — 11/11 rows pass at db877f68ef27 in one closed container-netns admission bracket (bracket-digest verdict=match); every row-named mutation killed. gate: human, irreversible: yes — status stays implemented for the human gate; a model does not sign it off.
### Fail-first re-run — 2026-11-10 — claude-opus-5-5 verifier (verify-desk dispatch), merged main fcc9289faeb9

Runner is not the implementer of PR #5386 (delivery merge 9ec2680d0d24) or of the earlier preparation PR #5113. This run exists because the close gate found no fail-first witness (no Base cell) for rows 1-11. Subject: merged main fcc9289faeb96dcca9d664b92b0283ea317e3659, admitted by deskdispatch --check-verifier (attestation below). Offline envelope (KUBECONFIG=/dev/null), one closed container-netns admission bracket: --open-bracket with --module-cache and --history-out on the host, then the container run with --fail-first --base and --history-pack in the pinned verify-runner image (network none, read-only root, module cache mounted read-only, GOPROXY=off, GOTOOLCHAIN=local), then --close-bracket on the host. Nothing was edited in the home between the steps.

Fail-first base: 0f97096fc827a05617e708abcde8f70f99530589. How it was derived: the Delivery reconciliation names delivery PR #5386 as commit 9ec2680d0d242abd0fc13584feddb1459ddbd634. That commit is a two-parent merge, with first parent 0f97096fc827 and second parent a897a51553d5. It lies on the first-parent chain of fcc9289faeb9 (git rev-list --first-parent). git log 9ec2680d0d24..fcc9289faeb9 over drainloop and spec/operator-incarnation-v1.md is empty, so the verified drainloop is the delivered one. The base holds the brief, and it holds none of the 11 test functions the rows name.

Witness (closed bracket; statusgen verifyrun --bracket-digest reports verdict=match):

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `cd drainloop && GOWORK=off go test -count=1 -v -run "^TestCellOwnership" ./...` | pass exit=0 sandbox=container-netns | sha256:f7b4a71c99e6 | 2026-11-10 | assay-verifier-app[bot] @ fcc9289faeb9 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=0f97096fc827 green rc=0 · head=fcc9289faeb9 |
| 2 | `cd drainloop && GOWORK=off go test -count=1 -v -run "^TestCellOwnershipStaleWriterDenied" ./...` | pass exit=0 sandbox=container-netns | sha256:9fb79afdc71a | 2026-11-10 | assay-verifier-app[bot] @ fcc9289faeb9 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=0f97096fc827 green rc=0 · head=fcc9289faeb9 |
| 3 | `cd drainloop && GOWORK=off go test -count=1 -v -run "^TestCellOwnershipRestoreBudgetPause" ./...` | pass exit=0 sandbox=container-netns | sha256:96884639b797 | 2026-11-10 | assay-verifier-app[bot] @ fcc9289faeb9 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=0f97096fc827 green rc=0 · head=fcc9289faeb9 |
| 4 | `(cd drainloop && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCellBudgetConcurrentReservations.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCellBudgetConcurrentReservations$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCellBudgetConcurrentReservations " "$wi_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ fcc9289faeb9 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=0f97096fc827 unproven · head=fcc9289faeb9 |
| 5 | `(cd drainloop && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCellBudgetRestoreAllAttempts.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCellBudgetRestoreAllAttempts$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCellBudgetRestoreAllAttempts " "$wi_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ fcc9289faeb9 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=0f97096fc827 unproven · head=fcc9289faeb9 |
| 6 | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestOwnershipBackendHandbackUnknown$" ./... > "$routing_out" && grep -q -- "--- PASS: TestOwnershipBackendHandbackUnknown " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ fcc9289faeb9 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=0f97096fc827 unproven · head=fcc9289faeb9 |
| 7 | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestIncarnationStaleRefused$" ./... > "$routing_out" && grep -q -- "--- PASS: TestIncarnationStaleRefused " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ fcc9289faeb9 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=0f97096fc827 unproven · head=fcc9289faeb9 |
| 8 | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestIncarnationMismatchPaused$" ./... > "$routing_out" && grep -q -- "--- PASS: TestIncarnationMismatchPaused " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ fcc9289faeb9 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=0f97096fc827 unproven · head=fcc9289faeb9 |
| 9 | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestIncarnationSpecMatchesFields$" ./... > "$routing_out" && grep -q -- "--- PASS: TestIncarnationSpecMatchesFields " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ fcc9289faeb9 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=0f97096fc827 unproven · head=fcc9289faeb9 |
| 10 | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCellBrakeCountUnavailableHolds$" ./... > "$routing_out" && grep -q -- "--- PASS: TestCellBrakeCountUnavailableHolds " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ fcc9289faeb9 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=0f97096fc827 unproven · head=fcc9289faeb9 |
| 11 | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCellPacingRestartNoMultiply$" ./... > "$routing_out" && grep -q -- "--- PASS: TestCellPacingRestartNoMultiply " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ fcc9289faeb9 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=0f97096fc827 unproven · head=fcc9289faeb9 |
Tree-Digest: sha256:0c52b03fa4ed04fe0003dd3fbd67ee268ac4c40816a437ca33f1c8139936d03b
Container-Image: sha256:03a2949dfe483f2645e14b85369c1923f1fb4710c7e5ea37a9b915d0073dde59
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6080 run=2a162949d5c802de72dd3f1aa23c81892cf41a5aa566d7e0409d422febeebe6e source=fcc9289faeb96dcca9d664b92b0283ea317e3659 brief=docs/streams/graph-execution/brief-16-cell-ownership-budgets.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=2a162949d5c802de72dd3f1aa23c81892cf41a5aa566d7e0409d422febeebe6e source=fcc9289faeb96dcca9d664b92b0283ea317e3659 tree=sha256:0c52b03fa4ed04fe0003dd3fbd67ee268ac4c40816a437ca33f1c8139936d03b witness=sha256:d7dd3dcf20ceb571e6623752622bb1cfbc81cb16cb622f94375ebda86df8a1fa opened=2026-11-10T18:24:51Z closed=2026-11-10T18:28:02Z before=receipt after=check-verifier

Result: every row passes at head fcc9289faeb9. The container verifyrun exited 2 because of its fail-first fold. The witness Base cells read green for rows 1-3 and unproven for rows 4-11. Neither reading reflects the rows' Expect column, as set out below.

Judged against Expect, every row is red at base and green at head. These were checked by hand on the output of a supplemental probe, listed further down.
- Rows 1-3 (Expect: exit 0; output includes PASS for the named test, with no [no tests to run] for its owning package). At base the owning package github.com/medici-finance/assay/drainloop prints "ok … [no tests to run]" and no PASS line, so the Expect is not met: red. At head each named test prints its "--- PASS" line, and the owning package prints "ok" without [no tests to run], so the Expect is met: green. The named tests are at drainloop/ownership_test.go lines 259, 397 and 417.
- Rows 4-11 (Expect: exit 0; named PASS). At base no named test exists. go test exits 0 and the owning package prints [no tests to run]. The row's own grep for "--- PASS: <Name> " exits 1, so the row is red. At head the grep exits 0: green.

Why the witness Base cells cannot record this (instrument and definition limits, not a deliverable failure):
- Rows 1-3: each Command is a bare go test -run, and it does not assert its own Expect. Base and head both exit 0, and the Base cell scores the exit code only, so it reads green. Only an amendment can make the fail-first witness show the red: a reviewed Verify-row amendment that adds the PASS assertion the Expect already states, as rows 4-11 carry.
- Rows 4-11: at base the row exits 1 with its output in a temp file, so it prints nothing. The base tree's module roots also require modules missing from the bracket's module cache, which --open-bracket downloaded for the head tree only. At 0f97096fc827, tools/desk requires github.com/go-logr/logr v1.4.3, github.com/mattn/go-isatty v0.0.20 and google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af, none of them cached. verifyrun records a non-zero exit with no output, under a module-cache gap, as could-not-run, so the Base cell reads unproven. drainloop's go.mod has no requirements, so the gap did not stop these rows, but the tool rightly does not credit that inference.

Supplemental probe (no acceptance credit): the exact row selectors on archived copies of drainloop at 0f97096fc827 and at fcc9289faeb9, outside the home, in the same pinned verify-runner image (network none, read-only root, GOPROXY=off, GOTOOLCHAIN=local):
- Rows 1-3 at base: go test exits 0; the owning package prints "ok github.com/medici-finance/assay/drainloop [no tests to run]"; there is no "--- PASS" line.
- Rows 1-3 at head: go test exits 0; "--- PASS: TestCellOwnership", "--- PASS: TestCellOwnershipStaleWriterDenied" and "--- PASS: TestCellOwnershipRestoreBudgetPause" each print; the owning package prints "ok github.com/medici-finance/assay/drainloop" without [no tests to run].
- Rows 4-11 at base: go test exits 0, the owning package prints [no tests to run], and the row's grep exits 1 (red).
- Rows 4-11 at head: the row's grep exits 0. Row 9 reads the operator incarnation spec from the repository root, so its head probe archived spec/operator-incarnation-v1.md beside drainloop.

Risk-bearing value (kit section 4). Enumeration over the delivery diff (0f97096fc827..9ec2680d0d24: drainloop budget.go and ownership.go, plus incarnation.go from the preparation work, and the two specs). Every literal was re-read at fcc9289faeb9 at the cited lines:
- epoch bound p.Epoch > 1 (and p.Epoch != epoch) @ drainloop/budget.go:157
- pacing admission n >= p.Cap @ drainloop/budget.go:177
- budget admission used > found.Limit after charging Headroom and outstanding reservations @ drainloop/budget.go:110-120
- launch-only control ControlBoundary "request" / "launch" with UpperBoundRef required @ drainloop/ownership.go:328
- item claim count matches != 1 @ drainloop/ownership.go:724
- overflow guards math.MaxUint64 @ drainloop/budget.go:104 and :166, ^uint64(0) @ drainloop/ownership.go:697
- pacing classes "effect" / "read", any other class refused @ drainloop/budget.go:162-175
- charged phases "preparation", "summary", "execution", "fallback", "verification" @ drainloop/ownership.go:324 and :634
- control roles ControlOperator = "operator", ControlExecutor = "executor" @ drainloop/incarnation.go:62-63
- IncarnationSchema = "operator-incarnation/v1" @ drainloop/incarnation.go:9; CellCheckpointSchema = "cell-checkpoint/v1" @ drainloop/ownership.go:18

Ranking: the irreversible act is a provider spend or an external effect admitted past a brake, or admitted for a stale owner. The epoch, cap, limit, boundary and claim-count bounds decide that act, so they rank first. The overflow guards and the class and phase sets fail closed, refusing any unknown value, so they rank next. The schema tags and role labels are wire identifiers: a mismatch refuses, and an edit and redeploy undo it, so they rank last.

RISK-VALUE: DERIVED — epoch bound p.Epoch > 1 @ drainloop/budget.go:157 — the amendment allows one recorded reset per window and operator incarnation, and a second reset exhausts the window. Epochs 0 and 1 admit, 2 or more refuse, and the caller's epoch must equal the recorded one. This gives the 2 × (cap + C) ceiling (row 11).
RISK-VALUE: DERIVED — pacing admission n >= p.Cap @ drainloop/budget.go:177 — the count is the shared seed plus every distinct recorded effect intent, so admission stops once Cap are counted. That is tighter than the brief's cap + C bound, and with one reset it stays within 2 × (cap + C).
RISK-VALUE: DERIVED — budget admission used > found.Limit @ drainloop/budget.go:120 with Headroom charged first @ drainloop/budget.go:110 — consumed + headroom + outstanding + new may not exceed the limit. Spending exactly to the limit is admitted and one unit more is refused, and a missing consumed count refuses rather than reading as zero (row 10).
RISK-VALUE: DERIVED — ControlBoundary "request" or "launch" with UpperBoundRef @ drainloop/ownership.go:328 — only a request-limiting runner may claim a hard per-request cap. A launch without an upper-bound reference is refused, and any other boundary is refused.
RISK-VALUE: DERIVED — item claim count matches != 1 @ drainloop/ownership.go:724 — exactly one live claim, owned by the current fenced owner and generation, must back an effect. Zero or two claims refuse, so a restored Cell tuple cannot revive a released claim (row 2).

Holds for the desk to route (this verifier posts to no forge):
1. Check definition, rows 1-3: each Command does not assert its own Expect, so the fail-first witness reads green at base. A reviewed Verify-row amendment, a driver act, that adds the PASS assertion would make the close gate's fail-first check satisfiable, followed by a fresh dispatch.
2. Tool capability, rows 4-11: the container fail-first lane cannot fill the module cache for the base tree. Any row that prints nothing at a base whose module roots differ from head is recorded unproven.
3. The verified flip is also held while lint is red (medici-finance/assay#4823).

**VERIFY: PASS** — all 11 rows meet their Expect at head fcc9289faeb9 in one closed container-netns admission bracket (bracket-digest verdict=match), and each row is red at base 0f97096fc827 by its Expect (supplemental same-image probe). The fail-first witness cannot record that red: rows 1-3 assert exit only, and rows 4-11 hit a base module-cache gap, so the close gate stays unsatisfied. gate: human, irreversible: yes — status stays implemented.

## Review

Review a usage-record fixture through persistence and restore: a synthetic credential
must be rejected or replaced with an authorized reference, while numeric usage-token
counts survive unchanged. Check the WI-1 source-trust binding referenced by WI-4.

Gate: human. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.

## Delivery reconciliation — 2026-11-03

Source inspection on main `3342a32ab90440ce7f2f6bf8957b83de626d70dd` confirms the
actual implementation delivery is merged PR #5386, commit
`9ec2680d0d242abd0fc13584feddb1459ddbd634`. PR #5113 remains the earlier schema/test
preparation, not the implementation witness. Its historical remaining-work statement
describes that preparation checkpoint.

| Original obligation | Delivered source to assess independently |
|---|---|
| Task 1: durable ownership/reservations, cumulative budgets and stop retention | `drainloop/ownership.go`: `CellStore`, `CellAuthority.Commit`, `CellJournal`; `drainloop/budget.go` reservation/balance/pacing owners |
| Task 2: separate effect-boundary fencing and unknown outcomes | `CellAuthority.CheckEffect`, capability/current-authority ports and `ResolvePending`; real provider credential cutover remains profile qualification |
| Task 3: concurrency/crash/restore fixtures | `drainloop/ownership_test.go`, `budget_test.go`, `restore_test.go` and the test-only filesystem/provider adapters; these are source witnesses, not independent PASS evidence |
| Task 4: qualified hold/reconciliation limits | `drainloop/README.md`, `docs/enforcement-model.md`, authoritative resolution and settlement ports; no production backend or exactly-once guarantee |
| Task 5: current incarnation and restore authority | `spec/operator-incarnation-v1.md`, `drainloop/incarnation.go` and current-incarnation checks at admission/effect boundaries; concrete custody/signature/consumer qualification remains separate |

The record-specific human binding is now recorded in `DR-graph-exec-16` and the
frontmatter `design:`/`ruling:` fields. It preserves the earlier bounded design
and both custody restrictions:
https://github.com/medici-finance/assay/issues/5111#issuecomment-6612666889

No board row is hand-edited here. After this metadata lands, use native
`statusgen reconcile` to judge the implemented transition against the actual
#5386 delivery. Independent verification, final human acceptance and activation
remain subsequent distinct acts.

This reconciliation changes no Task, original Verify definition, Evidence, dependency
or accepted policy. It installs no runtime/backend and claims no test execution.
