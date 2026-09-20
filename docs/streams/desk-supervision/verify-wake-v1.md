# verify-wake-v1 — verification wake receipts

Status: implemented (desk-supervision/16). Scheduling evidence only — a receipt can never grant
`verified`/`done`, satisfy acceptance, or authorize a write. The per-item claim and the
reviewer/verifier identity gates remain the barriers a receipt sits beside.

## Why

A verifier run that ends `verify-fail` or `blocked` was re-listed as a DISPATCH candidate every
plan and re-run every pass, reproducing the identical non-verdict, even when nothing its outcome
depends on had changed. A **wake receipt** keeps that failure VISIBLE while making the next
expensive run depend on a CHECKABLE change: it records what was observed, the inputs it was
observed against, the class of blocker, and the condition that must change before re-running is
worth a verifier slot.

This governs **failed/blocked retries only**. A `verified` outcome whose flip is stuck is the
stuck-flip lane's concern; a longitudinal `blocked-until:` window and an online verify lane keep
their existing buckets.

## Where it lives

A receipt is an **optional, versioned extension of the existing append-only verify-outcomes
sidecar row** (`docs/streams/verify-outcomes.jsonl`), not a second lifecycle database. Its first
four keys are exactly the legacy row (`ts`, `brief`, `outcome`, `sha`), so:

- an older reader that knows only those four keeps working, ignoring the additive fields;
- a legacy row (no `wake_schema`) parses as an **incomplete** receipt and stays visibly
  unclassified — never a fabricated hold;
- the additive keys never invalidate or shrink an existing row (the sidecar stays append-only).

## Fields (`wake_schema: "verify-wake-v1"`)

| Field | Meaning |
|---|---|
| `receipt_id` | stable ID; a duplicate ID is the SAME event, never a second one |
| `repo`, `brief` | owner/repo and `<stream>/<NN>` the receipt is for |
| `verifier` | the TRUSTED-WRITER identity (the engine's RunnerID / signed verdict principal), never a free-text assertion |
| `outcome` | observed outcome (`verify-fail` / `blocked` / …) |
| `rows` | the Verify-row numbers this receipt holds (empty = the whole brief) |
| `inputs` | declared input scope: `input-key -> revision observed at receipt time` |
| `tool_version` | applicable tool version at receipt time |
| `blocker_kind` | one of the closed set below |
| `blocker_ref` | issue/PR/action the blocker points at |
| `wake_predicate` | the checkable wake condition, one of the closed set below |
| `deadline` | RFC3339/date, for `declared-deadline-reached` |
| `recheck_reason` | required for `explicit-recheck-with-reason` |

Input revisions cover the **Verify definition**, the **relevant deliverable/dependency
revisions**, and the **applicable tool version**. Keys are opaque to the evaluator; the
already-authorized reader interprets them (the offline reader understands `tool` and
`file:<repo-relative-path>`).

### Blocker kinds (closed set) → next actor

`implementation` → worker · `check-definition` → brief-author · `human-action` → human ·
`environment` → operator · `unknown` → verifier (reclassify). A value outside the set makes the
receipt incomplete.

### Wake predicates (closed set)

- `relevant-input-changed` — a declared input's current revision differs from the recorded one.
- `referenced-action-completed` — the referenced external action has completed.
- `declared-deadline-reached` — a declared calendar deadline is at/after now.
- `explicit-recheck-with-reason` — an operator asked for a recheck, with a stated reason.

## Evaluation — the four states

The evaluator (`deskkit.WakeReceipt.EvaluateWake`) is pure: all external observation arrives
through an **already-authorized reader** (`deskkit.WakeInputs`) and the clock is injected. This
work adds **no production probes** — the offline reader answers revision queries from the local
tree (content hashes + the tool version) and NEVER observes an external action (that stays
could-not-check until an authorized online reader supplies it).

| State | When | Scheduling effect |
|---|---|---|
| **hold** | a complete receipt whose wake condition is not met | a VISIBLE `wait` row naming the blocker + next actor; excluded from costly dispatch |
| **fire** | the wake condition was met (input/tool/Verify-def changed, action completed, deadline reached, explicit recheck) | dispatch the affected work |
| **could-not-check** | a declared input could not be read | surfaced as could-not-check — never rounded up to unchanged, never an empty queue or a pass |
| **unclassified** | a legacy or incomplete receipt (missing schema/id, unknown vocabulary, or an empty input scope for `relevant-input-changed`) | eligible for exactly one ordinary classification pass; never a fabricated hold |

Invariants the tests pin (`cmd/verifyloop/wake_test.go`, `internal/deskkit/verifywake_test.go`):

- An unchanged receipt stays a WAIT across a process restart (the receipt lives in the sidecar;
  the classifier is stateless).
- An **unrelated** commit does not wake work when the declared inputs are known unchanged; an
  **incomplete input scope cannot establish unchangedness**, so it refuses to claim a hold.
- A **partial** hold lets a newly-runnable row dispatch while the held rows are recorded as
  explicitly unrun — and **no partial result closes the whole brief** (Land flips only on a
  whole-brief PASS).
- A duplicate `receipt_id` (the latest row per brief wins in the append-only log) does not create
  another event.

## Migration

- **Legacy rows** (pre-v1) stay valid and are read exactly as before; they classify as
  `unclassified` (one pass), which reproduces today's behaviour — the first complete receipt a
  run produces begins the wake scheduling.
- **Rollback** is the previous released binary/skill pin: the additive fields are ignored by the
  older reader, and receipts are never erased to make a rollback look clean.
- **Explicit recheck** is the operator's escape hatch: an `explicit-recheck-with-reason` receipt
  wakes the work regardless of the other predicates, with the reason recorded.
