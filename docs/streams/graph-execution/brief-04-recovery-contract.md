---
brief: assay:assay:graph-execution:04
title: Recovery contract for effect-bearing nodes in drainloop
why: >-
  A worker can make an external change — open a pull request, post a comment, push a tag —
  and then die before it writes down that it did so. The next worker to pick the item up
  sees no record and does it again. Today the drain engine's journal is best-effort by
  contract, so nothing stops that second effect. This brief makes an effect's receipt the
  thing a retry must find first, so an interrupted run is reconciled, never repeated.
wave: 1
depends: ["graph-execution/02"]
unblocks: ["graph-execution/05"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by graph-execution authoring session (fable-5.1, author-brief)
sources:
  - "docs/streams/graph-execution/admission-assurance-spec.md — 2026-09-18 integration amendment"
  - "freshness-checked 2026-09-18 @ 951ca784d100a7d201a28a34033da6709ec2ec8f"
  - "docs/streams/graph-execution/spec.md §2.4 (recovery semantics beyond an audit log; the lost-acknowledgment case) and §4 (the recovery case in the experiment)"
  - "drainloop/journal.go (Journal contract: 'a returned error is logged but does not abort the drain') and drainloop/engine.go `Config.record` (the best-effort implementation of that contract)"
  - "drainloop/item.go (Item carries ID, Implementer, Payload, Retry — no run or attempt identity) and drainloop/retry.go (the three-state retry taxonomy this brief's Reconcile step runs before)"
  - "docs/three-state-instrument-rule.md (a receipt lookup that cannot read the authoritative record is could-not-check, never 'not found')"
  - "Temporal workshop: deterministic orchestration over nondeterministic activities at https://youtu.be/QjVpE6-G18U?t=306, retry/recovery demonstration at https://youtu.be/QjVpE6-G18U?t=1209 — motivation only; it does not demonstrate lost-acknowledgment recovery"
  - "freshness-checked 2026-09-16 @ d96fd3ba: `grep -rn 'Effect\\b\\|Receipt\\|RunID\\|AttemptID\\|Reconcile\\|idempoten' drainloop/*.go` returns only the three 'Release is idempotent' comments in engine.go, engine_test.go and claim.go — no effect adapter, receipt, run/attempt identity or reconcile step exists; not already satisfied"
exec-tier: strong
exec-tier-why: "(c) concurrency and safety plumbing — a subtle ordering error (record after apply, or reconcile after retry) passes every happy-path test and only shows as a duplicated external effect in production; (a) the receipt/idempotency-key shape is a design decision the facts do not fully pre-specify."
domain: complicated
consumers:
  - "drainloop/README.md §Optional layers (the Journal row's 'best-effort' wording is now conditional on whether the adapter registers effects): follow-up graph-execution/04 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/streams/graph-execution/spec.md §3 seam table row 'Drain engine' (state text): out-of-scope (the spec describes the starting state; the stream README's end-state paragraph is the forward statement)"
  - "graph-execution/05's experiment harness (the recovery case consumes the Effect/Receipt/Reconcile seam by name): follow-up graph-execution/05"
version: 2
id: 961a1aa6-80a6-4712-b8ca-5a71cbdf6ece
---

# Brief 04 — Recovery contract for effect-bearing nodes in drainloop

## Context
files: `drainloop/effect.go` (planned) (Effect, Receipt, Reconcile), `drainloop/effect_test.go` (planned), `drainloop/item.go` (RunID/AttemptID on Item), `drainloop/engine.go` (Config.record becomes two-tier; Reconcile before an effect retry), `drainloop/journal.go` (the new Event kinds and the conditional contract), `drainloop/README.md` (§The contract, §Optional layers), `docs/enforcement-model.md` (one row: what the recovery contract enforces vs attributes), `changelog/graph-execution-04-recovery-contract.md` (planned)
facts:
  - Module: `github.com/medici-finance/assay/drainloop`, standalone (`drainloop/go.mod`, `go 1.25.0`); CI runs it with `GOWORK=off`. `TestNoDeskkitImports` (`drainloop/nodeskkit_test.go`) asserts the package imports no house package — the new file must keep that green.
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
6. **Docs.** `drainloop/README.md`: a §Effect layer subsection under §Optional layers stating the reconcile → intent → apply → done order and the conditional journal contract; the Journal row's wording updated. `docs/enforcement-model.md`: one row stating that the recovery contract ENFORCES exactly-once *within adapters that route effects through this layer* and only ATTRIBUTES for anything that bypasses it. `changelog/graph-execution-04-recovery-contract.md` (planned).

## Verify
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd drainloop && GOWORK=off go test ./...` | exit 0 |
| 2 | check:ci | `cd drainloop && GOWORK=off go test -run 'TestLostAckIsReconciledNotRepeated' -v ./...` | exit 0; output contains `RECONCILE` and the store assertion line `records=1` |
| 3 | check:ci +mutation | `cd drainloop && cp engine.go /tmp/ge04-engine.bak && sed -i '' 's/reconcileBeforeApply(/reconcileBeforeApplyDISABLED(/' engine.go && (GOWORK=off go test -run 'TestLostAckIsReconciledNotRepeated' ./... ; echo rc=$?) ; cp /tmp/ge04-engine.bak engine.go` | `rc=1` — with the reconcile step bypassed the store ends with two records and the test reddens; restored after |
| 4 | check:ci +mutation | `cd drainloop && GOWORK=off go test -run 'TestIntentRecordFailureAbortsEffect' ./... && GOWORK=off go test -run 'TestJournalRequiredForEffects' ./...` | exit 0 — both negative-path tests pass: a failed intent record yields zero effects, a missing journal refuses to run |
| 5 | check:ci +neighbour | `cd drainloop && GOWORK=off go test -run 'TestNoDeskkitImports' ./... && GOWORK=off go test -run 'TestJournalRecordsScheduling' ./...` | exit 0 — the public core stays deskkit-free and the non-effect journal semantics are unchanged |
| 6 | check +dereference | `grep -c 'EFFECT-INTENT' drainloop/journal.go drainloop/engine.go drainloop/README.md` | each file count ≥ 1 — the documented kind is the emitted kind; a README that names a kind the engine never emits fails this row |
| 7 | check +dereference | `grep -n 'reconcile' docs/enforcement-model.md` | ≥ 1 line, and that line names the bypass boundary ("adapters that route effects through this layer") — a row that only claims exactly-once fails the reviewer's reading |
| 8 | check +flow | `cd drainloop && GOWORK=off go run ./cmd/demo > /tmp/ge04-demo.txt 2>&1; grep -c LAND /tmp/ge04-demo.txt` | 5 — the demo's five non-effect items still drain unchanged with the layer off |
| 9 | check | `statusgen --root . --consumers --diff-base $(git merge-base HEAD origin/main)` | exit 0 — the `consumers:` routing above is corroborated by the diff |
| 10 | check:ci +mutation | `cd drainloop && GOWORK=off go test -count=1 -v -run TestDeclaredEffectCannotBypassJournal ./...` | exit 0; named test PASS; an artifact node with a declared effect cannot bypass mandatory intent recording |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
The reviewer answers, in the verdict: does any Verify row prove the receipt lookup catches the
duplicate with the journal layer bypassed (row 3), and does any row prove the journal intent
catches it with the store unreadable (row 4, second test)? Both must be yes.
