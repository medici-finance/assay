---
brief: assay:assay:graph-execution:04
title: Recovery contract for effect-bearing nodes in drainloop
why: >-
  A worker can make an external change — open a pull request, post a comment, push a tag —
  and then die before it writes down that it did so. The next worker to pick the item up
  sees no record and does it again. Today the drain engine's journal is best-effort by
  contract, so nothing stops that second effect. This brief makes an effect's receipt the
  thing a retry must find first, so an interrupted run is reconciled, never repeated.
wave: 4
depends: ["graph-execution/02", "graph-execution/28", "graph-execution/29"]
unblocks:
- graph-execution/05
- graph-execution/21
- graph-execution/25
- graph-execution/16
- graph-execution/19
- "forge-neutral/38"
- "forge-neutral/41"
- performance-collection/09
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by graph-execution authoring session (fable-5.1, author-brief)
sources:
- docs/streams/graph-execution/admission-assurance-spec.md — 2026-09-18 integration amendment
- freshness-checked 2026-09-18 @ 951ca784d100a7d201a28a34033da6709ec2ec8f
- docs/streams/graph-execution/spec.md §2.4 (recovery semantics beyond an audit log; the lost-acknowledgment case)
  and §4 (the recovery case in the experiment)
- 'drainloop/journal.go (Journal contract: ''a returned error is logged but does not abort the drain'') and drainloop/engine.go
  `Config.record` (the best-effort implementation of that contract)'
- drainloop/item.go (Item carries ID, Implementer, Payload, Retry — no run or attempt identity) and drainloop/retry.go
  (the three-state retry taxonomy this brief's Reconcile step runs before)
- docs/three-state-instrument-rule.md (a receipt lookup that cannot read the authoritative record is could-not-check,
  never 'not found')
- 'Temporal workshop: deterministic orchestration over nondeterministic activities at https://youtu.be/QjVpE6-G18U?t=306,
  retry/recovery demonstration at https://youtu.be/QjVpE6-G18U?t=1209 — motivation only; it does not demonstrate
  lost-acknowledgment recovery'
- 'freshness-checked 2026-09-16 @ d96fd3ba: `grep -rn ''Effect\b\|Receipt\|RunID\|AttemptID\|Reconcile\|idempoten''
  drainloop/*.go` returns only the three ''Release is idempotent'' comments in engine.go, engine_test.go and claim.go
  — no effect adapter, receipt, run/attempt identity or reconcile step exists; not already satisfied'
- docs/streams/graph-execution/task-workflow-program.md — execution routing amendment 2026-10-02
- freshness-checked 2026-10-02 @ 307fe16992caef53fa46c52622753dd400c7b42a
- docs/proposals/non-ci-architecture/20-amendment-manifest.md — R2 owner row and R2 amendment scope (transactional intent/outbox handoff)
- docs/proposals/non-ci-architecture/14-storage-forge-graph.md — lines 34, 36, 38 (intent committed with admission and claim generation; outbox/inbox; no per-effect Git append)
- docs/proposals/non-ci-architecture/13-migration-and-decisions.md — line 71 (never replay stale outbox entries blindly)
- docs/proposals/non-ci-architecture/16-delivery-sequence.md — lines 69, 104 (transaction/outbox handoff; no silent store fallback)
- "medici-finance/assay#2500 — maintainer ratification of the whole target @ 23f655f5d (approval activates nothing)"
- freshness-checked 2026-10-13 @ 4696d598e58820806d0bcf8c9b2dd87f29115400
exec-tier: strong
exec-tier-why: "(c) concurrency and safety plumbing — a subtle ordering error (record after apply, or reconcile after retry) passes every happy-path test and only shows as a duplicated external effect in production; (a) the receipt/idempotency-key shape is a design decision the facts do not fully pre-specify."
domain: complicated
consumers:
- 'drainloop/README.md §Optional layers (the Journal row''s ''best-effort'' wording is now conditional on whether
  the adapter registers effects): fixed-here (engine-owned declared-effect protocol and conditional journal documentation)'
- 'docs/streams/graph-execution/spec.md §3 seam table row ''Drain engine'' (state text): out-of-scope (the spec
  describes the starting state; the stream README''s end-state paragraph is the forward statement)'
- 'graph-execution/05''s experiment harness (the recovery case consumes the Effect/Receipt/Reconcile seam by name):
  follow-up graph-execution/05'
- 'workflow/controller: follow-up graph-execution/21'
- 'workflow/publication: follow-up graph-execution/25'
- 'workflow/store PostgreSQL outbox (implements the handoff seam): follow-up graph-execution/19'
version: 5
id: 961a1aa6-80a6-4712-b8ca-5a71cbdf6ece
---

# Brief 04 — Recovery contract for effect-bearing nodes in drainloop

## Context
files: `drainloop/effect.go` (planned) (Effect, Receipt, Reconcile), `drainloop/effect_test.go` (planned), `drainloop/item.go` (RunID/AttemptID on Item), `drainloop/engine.go` (Config.record becomes two-tier; Reconcile before an effect retry), `drainloop/journal.go` (the new Event kinds and the conditional contract), `drainloop/README.md` (§The contract, §Optional layers), `docs/enforcement-model.md` (one row: what the recovery contract enforces vs attributes), `changelog/graph-execution-04-recovery-contract.md` (planned), `drainloop/handoff.go` (planned), `drainloop/handoff_test.go` (planned) — Added by the R2 amendment (2026-10-13): `drainloop/handoff.go` (planned), `drainloop/handoff_test.go` (planned).
facts:
  - Module: `github.com/medici-finance/assay/drainloop`, standalone (`drainloop/go.mod`, `go 1.27.0`); CI runs it with `GOWORK=off`. `TestNoDeskkitImports` (`drainloop/nodeskkit_test.go`) asserts the package imports no house package — the new file must keep that green.
  - Journal today (2026-09-16 @ d96fd3ba): `Config.record` in `drainloop/engine.go` logs a sink error as `JOURNAL ... (non-fatal)` and continues. `journal.go` documents the kinds the engine emits: CLAIM, DISPATCH, LAND, RELEASE (via LAND), SKIP, HOLD, GUARD, EVIDENCE, IDLE. Nothing in the engine distinguishes an event that precedes an external effect from one that does not.
  - `Item` (`drainloop/item.go`) carries `ID`, `Implementer`, `Payload`, `Retry`. There is no run identity and no attempt identity; `Retry` is a count a queue bumps on re-selection.
  - Retry (`drainloop/retry.go`) is a facility `Land`/re-selection consults, deliberately off the six-method `Loop` contract. A `DecisionRetry` re-selects and re-dispatches the item — today with no step between the decision and the re-dispatch.
  - Vocabulary (stream README, shared conventions): node kinds `artifact | check | decision | effect`; every node with declared effects registers each Effect. The node contract from graph-execution/02 declares `effects:[{kind, target}]`; this brief gives the executor the adapter that carries one out.
  - Single-point-of-failure note: the ONE control is the receipt lookup before re-apply. Second, independent layer: the journal record BEFORE the effect is mandatory for effect nodes, so a crash after the effect leaves a RECORDED intent the resuming worker reconciles against even when the authoritative store is unreadable (could-not-check ⇒ hold, never re-apply). Third, out-of-band: the idempotency key is passed to the authoritative store on `Apply`, so a store that honours keys refuses the duplicate even if both engine layers were bypassed.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Public tree: fixtures and tests use `example-org/*` placeholders; no adopter names, numbers or repos.
- The six-method `Loop` contract is not widened. Effect handling is an opt-in layer on `Config`, like `Evidence` and `Journal`.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Integration amendment — 2026-09-18

This remains the basic mandatory-intent/receipt/reconcile contract. Apply it to EVERY declared external effect, including a non-effect-kind node legally declaring effects within its output boundary under pattern §4.2.2; do not let node kind bypass recording. Keys bind logical run, operation, target and subject, NOT attempt, so retries reuse the same operation key. Preserve succeeded/failed/unknown outcomes and hold an unreadable lookup. Cell fencing/cumulative budgets are the separate extension 16; do not enlarge this brief into that work.

## Task workflow amendment — 2026-10-02

Controller launches and publication use this intent/receipt protocol. Preserve stable operation keys across attempts. Replace any unconditional exactly-once claim with the provider-specific enforced boundary: if dedupe or authoritative receipt lookup is unavailable, an unknown result holds; it never licenses a blind retry. The controller is a consumer, not another effect journal.

Implement the named failure/flow case below in the declared test surface. This amendment does not record implementation evidence or authorize live activation.

## Non-CI R2 amendment — 2026-10-13 (transactional intent/outbox handoff)

The maintainer ratified the non-CI target (medici-finance/assay#2500, revision 23f655f5d);
approval activates nothing. Owner row: package R2 in the
[amendment manifest](../../proposals/non-ci-architecture/20-amendment-manifest.md). The
reconcile → intent → apply → done contract above is unchanged; this section says where the
intent is committed.

- **Handoff seam.** Add an opt-in, store-neutral handoff on `Config`: a journal that declares
  transactional handoff commits the `EFFECT-INTENT` together with the item's claim generation
  in one durable commit before `Apply`, and exposes pending outbox entries on resume, keyed by
  the stable operation key (target 14:34). drainloop stays standalone and imports no database
  driver; the PostgreSQL implementation is graph-execution/19's outbox.
- **Resume.** With the store intact, a pending entry resumes through Reconcile first, then
  Apply at most once. An entry whose claim generation is no longer current is never replayed
  blindly: it reconciles against the authoritative store or holds (13:71).
- **No fallback.** When the configuration requires transactional handoff for effect items and
  the journal cannot provide it, `Run` refuses before any pass; it never drops to the
  best-effort journal or to another store (16:104).
- **No Git journal per effect.** The handoff is the coordination store's commit; no Git append
  is added on the critical path of an effect (14:36).
- Six-method `Loop` unchanged; `TestNoDeskkitImports` stays green. Risk answers and gate
  unchanged.

## R4 follow-up — not in this amendment

Package R4 later amends this brief for role-bound effect identity and lost acknowledgment at
the publisher. Nothing here pre-empts it.

## Architecture pickup — implementation

Continue at source `30ca40aae3e9154c001af244fd7e73b4832eaff5`: brief v4 plus
its R2 amendment, admission-assurance-spec, non-CI architecture 13/14/16/20,
`docs/go-engineering.md`, `docs/library-first.md`, `docs/contracts.md` and the
standalone drainloop implementation were inspected together. Prerequisites /02,
/28 and /29 are independently recorded verified in the current board. The current
module uses Go 1.27.0; the earlier Context baseline was Go 1.25.0.

This is a bounded manual pickup, not a claim that a future context resolver ran.
The approved R2 scope is compatible with engine-owned Apply and an opt-in
TransactionalJournal. Standard-library contracts, hashing and testing fit the
standalone boundary; pgx/Goose belong to /19, and a managed workflow library would
replace rather than implement this small public seam. No dependency is added.
The six-method Loop stays unchanged. No existing effect owner is duplicated or
retired: none existed in drainloop at pickup. Every declaration follows one path,
regardless of node kind. The semantic owner row in `docs/contracts.md` and the
runtime mutation harness in `drainloop/testdata/effect_mutations.py` are included.

Consumers /05, /21, /25, /19 and the fencing extension /16 remain future work;
no database, provider, publisher, live controller or credential profile is activated.
The in-memory transactional fake proves the seam and ordering, not PostgreSQL
atomicity or provider containment. Native dispatch retained the existing ownership
checks; its recent-window phantom ceiling remains a disclosed coverage limit.
The implementation replaces row 3's compile-only rename with a runtime mutation
that records two physical external writes. Independent verification still owns
Evidence and lifecycle changes.

## Task
1. **Identity.** Add `RunID` and `AttemptID` to `Item` (`drainloop/item.go`): `RunID` is stable across attempts of one unit of work; `AttemptID` is minted per dispatch. Document that a queue re-selecting a failed item keeps `RunID` and mints a new `AttemptID`. Both are strings; empty means "the adapter did not assign one", legal only for items with no declared external effects (step 3).
2. **Effect adapter** (`drainloop/effect.go` (planned)):
   ```go
   type Receipt struct { Key string; Ref string; At time.Time }   // Ref points at the authoritative record (a URL/path/id)
   type Effect interface {
       Key() string                                              // idempotency key, stable across attempts
       Receipt(ctx context.Context, key string) (r Receipt, found bool, err error) // err ⇒ could-not-check
       Apply(ctx context.Context) (Receipt, error)               // the one external write; passes Key() to the store
   }
   ```
   `Config.Effects func(Item) ([]Effect, error)` — opt-in; nil ⇒ the layer is off and nothing below runs. A non-nil error is could-not-check: the item is SKIPped this pass, never dispatched.
3. **Journal becomes required for effect items.** When `Effects(it)` returns one or more effects, `Config.Journal == nil` is a `Run` configuration error (returned before any pass, like `PoolSize < 1`). Add Event kinds `EFFECT-INTENT` (recorded BEFORE `Apply`, carrying `RunID`, `AttemptID`, `Key`), `EFFECT-DONE` (after, carrying `Receipt.Ref`) and `RECONCILE`. A `Journal.Record` error on `EFFECT-INTENT` ABORTS the effect (the item lands `VerdictError` with an EvidenceRow naming the journal failure); the existing best-effort behaviour of `Config.record` is kept for every other kind, and `journal.go`'s comment says which kinds are which.
4. **Reconcile before retry.** Before any dispatch of an item whose `Retry > 0` (or whose journal shows an `EFFECT-INTENT` without `EFFECT-DONE` for the same `RunID`), call `Receipt(ctx, Key())` for each effect: `found` ⇒ record `RECONCILE` with the receipt, mark the effect done and skip `Apply`; `!found, err == nil` ⇒ proceed to `Apply`; `err != nil` ⇒ could-not-check: land `VerdictHold` with a row `reconcile could-not-check`, do NOT apply. The order is fixed: reconcile → intent → apply → done.
5. **Tests** (`drainloop/effect_test.go` (planned)), each with an in-memory authoritative store adapter:
   - `TestLostAckIsReconciledNotRepeated` (planned): worker A applies (store gets one record), the process "dies" before `EFFECT-DONE`; worker B resumes the same `RunID` → store still holds exactly ONE record; journal has `RECONCILE`; no second `Apply`.
   - `TestDuplicateDeliveryAppliesOnce` (planned): the same item delivered twice in one queue read → one `Apply`.
   - `TestLostEventTriggersReconcile` (planned): `EFFECT-DONE` dropped by the sink → the next attempt reconciles from the store, not from the journal.
   - `TestJournalRequiredForEffects` (planned): effects registered, `Journal == nil` → `Run` returns an error before any pass.
   - `TestIntentRecordFailureAbortsEffect` (planned): sink errors on `EFFECT-INTENT` → store has ZERO records; item lands `VerdictError`.
   - `TestReceiptCouldNotCheckHolds` (planned): store returns an error → `VerdictHold`, zero `Apply`.
   - `TestNonEffectJournalStaysBestEffort` (planned): a sink error on `CLAIM` still does not abort (the existing `TestJournalRecordsScheduling` semantics for non-effect kinds).
6. **Docs.** `drainloop/README.md`: a §Effect layer subsection under §Optional layers stating the reconcile → intent → apply → done order and the conditional journal contract; the Journal row's wording updated. `docs/enforcement-model.md`: one row stating that the recovery contract states the adapter-specific idempotency/fencing guarantee and holds unknown outcomes where it cannot enforce deduplication; bypasses are only ATTRIBUTED. `changelog/graph-execution-04-recovery-contract.md` (planned).
7. **Handoff (R2, 2026-10-13).** Implement the handoff seam in `drainloop/handoff.go` (planned) with an in-memory transactional fake: intent and claim generation commit together before `Apply`; pending entries resume through Reconcile; a stale-generation entry reconciles or holds; a required handoff that the journal cannot provide refuses `Run`.

## Verify
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd drainloop && GOWORK=off go test ./...` | exit 0 |
| 2 | check:ci | `cd drainloop && GOWORK=off go test -run 'TestLostAckIsReconciledNotRepeated' -v ./...` | exit 0; output contains `RECONCILE` and the store assertion line `records=1` |
| 3 | check:ci +mutation | `cd drainloop && python3 testdata/effect_mutations.py receipt-bypassed` | exit 0; `KILLED receipt-bypassed (runtime assertion)` with `records=2`; the mutated test exits nonzero and source is restored (compile-only failure is rejected) |
| 4 | check:ci +mutation | `cd drainloop && GOWORK=off go test -run 'TestIntentRecordFailureAbortsEffect' ./... && GOWORK=off go test -run 'TestJournalRequiredForEffects' ./...` | exit 0 — both negative-path tests pass: a failed intent record yields zero effects, a missing journal refuses to run |
| 5 | check:ci +neighbour | `cd drainloop && GOWORK=off go test -run 'TestNoDeskkitImports' ./... && GOWORK=off go test -run 'TestJournalRecordsScheduling' ./...` | exit 0 — the public core stays deskkit-free and the non-effect journal semantics are unchanged |
| 6 | check +dereference | `grep -c 'event.Kind = "EFFECT-INTENT"' drainloop/effect.go; grep -c 'EFFECT-INTENT' drainloop/journal.go drainloop/README.md` | each count ≥ 1 — the documented kind is the emitted kind: the first grep reads the assignment in `effect.go` that emits it, so deleting the emission fails this row, and a README that names a kind never emitted fails it too |
| 7 | check +dereference | `grep -n 'reconcile' docs/enforcement-model.md` | ≥ 1 line, and that line states the adapter-specific idempotency/fencing guarantee, holds unknown outcomes where deduplication cannot be enforced, and says bypasses are only attributed (the bypass boundary named in Task 6) — a row that only claims exactly-once fails the reviewer's reading |
| 8 | check +flow | `cd drainloop && GOWORK=off go run ./cmd/demo > /tmp/ge04-demo.txt 2>&1; grep -c LAND /tmp/ge04-demo.txt` | 5 — the demo's five non-effect items still drain unchanged with the layer off |
| 9 | check | `(consumer_out=$(mktemp) && trap 'rm -f "$consumer_out"' 0 && statusgen --consumers --root . --brief graph-execution/04 --delivery 1746936e41cccd237d9bd9021cff1a128d3c1dfb --base 30ca40aae3e9154c001af244fd7e73b4832eaff5 > "$consumer_out" && grep -Fx 'summary: 5 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing' "$consumer_out")` | exit 0; exact summary shown. The four follow-up routes are corroborated declarations, not consumer implementation or behavioral acceptance; the out-of-scope historical spec remains unchecked and requires review. Actual immutable delivery and its first parent are pinned. |
| 10 | check:ci +mutation | `cd drainloop && GOWORK=off go test -count=1 -v -run TestDeclaredEffectCannotBypassJournal ./...` | exit 0; named test PASS; an artifact node with a declared effect cannot bypass mandatory intent recording |
| 11 | check:ci +flow +mutation | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestEffectUnsupportedIdempotencyHolds$" ./... > "$routing_out" && grep -q -- "--- PASS: TestEffectUnsupportedIdempotencyHolds " "$routing_out")` | exit 0; named PASS; unreadable receipt and unsupported dedupe never permit a duplicate Apply; mutation: treat an unreadable receipt as not-applied and re-run Apply — the named test must fail |
| 12 | check:ci +flow +mutation | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestEffectIntentOutboxHandoff$" ./... > "$routing_out" && grep -q -- "--- PASS: TestEffectIntentOutboxHandoff " "$routing_out")` | exit 0; named PASS; the intent and claim generation commit together before Apply, a crash after that commit resumes through Reconcile with at most one Apply, and a pending entry from a superseded generation reconciles or holds; mutation: replay pending outbox entries on resume without the generation and reconcile checks — the named test must fail |
| 13 | check:ci +mutation | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestOutboxRequiredNoFallback$" ./... > "$routing_out" && grep -q -- "--- PASS: TestOutboxRequiredNoFallback " "$routing_out")` | exit 0; named PASS; with transactional handoff required and a journal that cannot provide it, `Run` returns an error before any pass and no effect is applied; mutation: fall back to the best-effort journal when the handoff is unavailable — the named test must fail |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->
### Non-implementer verifier run — 2026-10-29 — claude-opus-5-5 (strong) as assay-verifier-app[bot]

Source 5729430698d44ffd874bb25275d056f5a5404dc6 (merged main; the delivery 1746936e41cccd237d9bd9021cff1a128d3c1dfb is an ancestor). Admission: deskdispatch --check-verifier exited 0 for this detached home and brief before any row ran. The runner is not the implementer. KUBECONFIG=/dev/null, no commit, no push, no forge write. The 13 rows were split into two disjoint selections that together cover the whole table.

- Rows 6, 7, 8 and 9 (classes check, run on the darwin/arm64 host with go1.27.2; row 9 needs git history, which the container does not mount): the first table below is the tool's own write-mode statusgen verifyrun --rows 6,7,8,9 output, run 17:19:02Z-17:19:13Z after the container bracket closed, left as written.
- Rows 1, 2, 3, 4, 5, 10, 11, 12 and 13 (every check:ci row): statusgen verifyrun --open-bracket on the host at 17:14:19Z (receipt outside the home), then statusgen verifyrun --sandbox=container-netns --rows 1,2,3,4,5,10,11,12,13 inside docker run --network none --read-only --cap-drop ALL on the pinned verify-runner v1.0.65 image (digest in the Container-Image line; go1.27.3, git 2.56.0, Python 3.11.2, statusgen v1.0.65 release binary, linux/amd64 under emulation on an arm64 host), non-root uid, GOPROXY=off, GOTOOLCHAIN=local, tmpfs HOME, /tmp and build cache, no forge credential. The home was mounted read-write at its own path because row 3's mutation script edits a source file and restores it; the container step's own after-row tree check and the host close both confirmed the tree unchanged. The drainloop module has no module dependencies, so no module cache was mounted. Run 17:14:49Z-17:16:12Z; --close-bracket at 17:16:30Z appended the Admission-Bracket line and --bracket-digest read verdict=match. The second table below, with its Tree-Digest, Container-Image, attestation and Admission-Bracket lines, is that witness file's bytes.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 6 | `grep -c 'event.Kind = "EFFECT-INTENT"' drainloop/effect.go; grep -c 'EFFECT-INTENT' drainloop/journal.go drainloop/README.md` | pass exit=0 | sha256:1c2ce2bcbab2 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -n 'reconcile' docs/enforcement-model.md` | pass exit=0 | sha256:c19652bcd5a4 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd drainloop && GOWORK=off go run ./cmd/demo > /tmp/ge04-demo.txt 2>&1; grep -c LAND /tmp/ge04-demo.txt` | pass exit=0 | sha256:f0b5c2c2211c | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 9 | `(consumer_out=$(mktemp) && trap 'rm -f "$consumer_out"' 0 && statusgen --consumers --root . --brief graph-execution/04 --delivery 1746936e41cccd237d9bd9021cff1a128d3c1dfb --base 30ca40aae3e9154c001af244fd7e73b4832eaff5 > "$consumer_out" && grep -Fx 'summary: 5 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing' "$consumer_out")` | pass exit=0 | sha256:76cf4afdba5c | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4819 run=b37c73fb2590f3102ccc136704e1af3d37b914483254e2e2ab11206a131a27f6 source=5729430698d44ffd874bb25275d056f5a5404dc6 brief=docs/streams/graph-execution/brief-04-recovery-contract.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd drainloop && GOWORK=off go test ./...` | pass exit=0 sandbox=container-netns | sha256:27043617a689 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd drainloop && GOWORK=off go test -run 'TestLostAckIsReconciledNotRepeated' -v ./...` | pass exit=0 sandbox=container-netns | sha256:79f7b2f4f1f4 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd drainloop && python3 testdata/effect_mutations.py receipt-bypassed` | pass exit=0 sandbox=container-netns | sha256:12385f2e6e1b | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd drainloop && GOWORK=off go test -run 'TestIntentRecordFailureAbortsEffect' ./... && GOWORK=off go test -run 'TestJournalRequiredForEffects' ./...` | pass exit=0 sandbox=container-netns | sha256:c5b048d2e229 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd drainloop && GOWORK=off go test -run 'TestNoDeskkitImports' ./... && GOWORK=off go test -run 'TestJournalRecordsScheduling' ./...` | pass exit=0 sandbox=container-netns | sha256:5d71e9efe93b | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd drainloop && GOWORK=off go test -count=1 -v -run TestDeclaredEffectCannotBypassJournal ./...` | pass exit=0 sandbox=container-netns | sha256:86a985a506a8 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 11 | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestEffectUnsupportedIdempotencyHolds$" ./... > "$routing_out" && grep -q -- "--- PASS: TestEffectUnsupportedIdempotencyHolds " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 12 | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestEffectIntentOutboxHandoff$" ./... > "$routing_out" && grep -q -- "--- PASS: TestEffectIntentOutboxHandoff " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 13 | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestOutboxRequiredNoFallback$" ./... > "$routing_out" && grep -q -- "--- PASS: TestOutboxRequiredNoFallback " "$routing_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:14caf0ea56811f25c67d30cc8f736f33e114ece3a6504c05fc4a241f9def8d9f
Container-Image: sha256:1af55e60c9222ad09dab8376b629725135c54912bb3583c13591833db90f2d8f
Verification-Attestation: medici-finance/assay#4819 run=b37c73fb2590f3102ccc136704e1af3d37b914483254e2e2ab11206a131a27f6 source=5729430698d44ffd874bb25275d056f5a5404dc6 brief=docs/streams/graph-execution/brief-04-recovery-contract.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=b37c73fb2590f3102ccc136704e1af3d37b914483254e2e2ab11206a131a27f6 source=5729430698d44ffd874bb25275d056f5a5404dc6 tree=sha256:14caf0ea56811f25c67d30cc8f736f33e114ece3a6504c05fc4a241f9def8d9f witness=sha256:4c138a201c2dcd367af156fc4f6e68dc7c392bcc7af682ab6c1442e50a422074 opened=2026-10-29T17:14:19Z closed=2026-10-29T17:16:30Z before=receipt after=check-verifier

**Supplemental observations (no acceptance credit; diagnostic only).** The witness records exit status and an output hash, so the Expect text of each row was read from a second run of the same commands. Container, same image and flags, 17:16:42Z-17:17:21Z: row 2 printed "effect_test.go:98: RECONCILE records=1" and its named test PASS; row 3 printed "KILLED receipt-bypassed (runtime assertion)", then "effect_test.go:96: records=2 kinds=map[CLAIM:2 EFFECT-DONE:1 EFFECT-INTENT:2 IDLE:2 LAND:2]" and "Totals: 1 killed, 0 survived, 0 compile-only", and the source was restored (git status clean afterwards); rows 4, 5 and 10-13 each printed PASS for every named test. The full mutation sweep (all 16 mutations in the script, 17:17:34Z-17:18:54Z) printed "Totals: 16 killed, 0 survived, 0 compile-only", which includes the mutations Expect names for rows 4, 10, 11, 12 and 13 (intent-error-ignored, missing-journal-accepted, declared-artifact-bypass, unknown-treated-as-absence, stale-generation-unchecked, non-atomic-intent, outbox-falls-back). Host, 17:19Z: row 6 counts 1, journal.go 2, README.md 3 (each at least 1); row 7 printed line 210, which states that the adapter-specific idempotency/fencing guarantee depends on provider dedupe and ownership enforcement, that unknown outcomes hold where deduplication or authoritative absence cannot be enforced, and that bypasses are only attributed, never an unconditional exactly-once guarantee; row 8 counted 5 LAND lines; row 9 printed the exact summary line, and its one entry the tool leaves to review is the out-of-scope historical spec row, which Expect leaves to review.

**Risk-bearing values.** The brief's risk answers are all no and it is not irreversible; the enumeration still ran over the delivery diff (drainloop effect.go, handoff.go, engine.go, item.go, journal.go and the mutation script). Literals found: the operation-key format at drainloop/effect.go:48, the event kinds at drainloop/effect.go:131, 134, 145 and 166, the verdict bindings at drainloop/effect.go:122, 152, 159 and 161, and the mutation harness timeouts at drainloop/testdata/effect_mutations.py:64-65. Ranked by irreversibility, the key format and the hold bindings come first: a wrong one produces a duplicate external effect (a second pull request, comment or tag), which no edit and redeploy can take back. The harness timeouts are reversible test knobs and rank last.

- RISK-VALUE: DERIVED — OperationKey format = "effect-v1:%x" over sha256 of the JSON array [run, operation, target, subject] @ drainloop/effect.go:48 — the JSON array fixes coordinate boundaries, so no two coordinate tuples concatenate to the same input; sha256 makes a collision between distinct operations infeasible; the attempt identity is excluded, as the 2026-09-18 amendment requires, so a retry reuses the same key; and the version prefix keeps a later format distinguishable instead of silently missing a key already persisted by a store.
- RISK-VALUE: DERIVED — event.Kind = "EFFECT-INTENT" @ drainloop/effect.go:145 (with "RECONCILE" @ drainloop/effect.go:131 and "EFFECT-DONE" @ drainloop/effect.go:134 and 166) — these are the exact kind names Task 3 fixes; the intent kind is emitted before Apply at line 154, giving the reconcile, intent, apply, done order Task 4 requires; and journal.go line 8 documents the same three names.
- RISK-VALUE: DERIVED — receipt lookup error ⇒ VerdictHold "reconcile could-not-check" @ drainloop/effect.go:122; intent record error ⇒ VerdictError "effect intent journal" @ drainloop/effect.go:152 — Task 4 and Task 3 name these outcomes literally, and the three-state instrument rule forbids reading an unreadable lookup as not-found.
- RISK-VALUE: DERIVED — unclassified Apply error ⇒ VerdictHold "effect outcome unknown" @ drainloop/effect.go:161 (only a provider-proven EffectFailure maps to VerdictFail, line 159) — the 2026-10-02 amendment says an unknown result holds and never licenses a blind retry, and holding is the only safe reading of an error that may have followed the external write.

Review-gate questions, for the reviewer: row 3 removes the receipt lookup while the journal intent stays in place, and the store then holds records=2, so the receipt lookup is the control that catches the duplicate. Row 4's first test shows a failed intent record yields zero effects, and row 11 shows an unreadable receipt holds with zero re-apply.

**VERIFY: PASS** — 13 of 13 rows pass: 9 check:ci rows in the closed container bracket and 4 host rows; all 13 rows executed.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
The reviewer answers, in the verdict: does any Verify row prove the receipt lookup catches the
duplicate with the journal layer bypassed (row 3), and does any row prove the journal intent
catches it with the store unreadable (row 4, second test)? Both must be yes.
