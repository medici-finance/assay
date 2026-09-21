# repair-obligation-v1 — durable worker repair obligations

Status: implemented (desk-supervision/17). Scheduling state only — a repair obligation can never
grant `verified`/`done`, satisfy acceptance, or authorize a write. The per-item claim and the
reviewer/verifier identity gates remain the barriers an obligation sits beside.

## Why

desk-supervision/16 records WHY a failed verification should not simply re-run (a wake receipt).
This is its sibling for the OTHER half: an **actionable** failed verification is durable worker
work that must survive the reporting agent, be worked exactly once, and return to **independent
reverification** after its repair merges. A filed verification failure could otherwise sit
unassigned while new briefs consumed workers, or be quietly closed by the same agent that reported
it. A **repair obligation** is the durable unit that closes both gaps: one obligation per failed
outcome, keyed so it cannot be duplicated, resolvable only by an independent pass.

This governs **actionable failed/blocked outcomes**. A `verified` outcome whose flip is stuck is
the stuck-flip lane's concern; a human/environment blocker stays `waiting-external` with its exact
required action and is never dispatched to a worker.

## Where it lives

An obligation is a **versioned, structured marker** — an append-only JSONL projection at
`docs/streams/repair-obligations.jsonl`, a **sibling** of desk-supervision/16's
`verify-outcomes.jsonl` kept in a separate file so a repair line never parses as an incomplete wake
receipt and vice versa. Its first two keys are the legacy sidecar header (`ts`, `brief`), so an
older reader keeps working. The **durable authority** remains the target-repository issue/PR records
plus the dispatch claim; this marker is the machine-readable projection over them, never a second
lifecycle database and never an authority grant.

The log is append-only, so one immutable obligation ID may appear many times (a duplicate delivery,
a lost acknowledgement re-posted, a claim then a repair then a merge).
`deskkit.ReconcileObligations` folds each ID to the **furthest-progressed** record before anything
acts on it — the read side IS the partial-write recovery: an idempotent re-post never creates a
second obligation, and a partial write is repaired by reading, never by blindly appending again.

## The immutable key

`deskkit.RepairObligationID(repo, brief, receiptID, rows)` is a content hash of the failed
verification (rows sorted first, so row order never changes the key). Identical inputs yield an
identical key, so a duplicate delivery, a lost acknowledgement, or a process restart all address the
SAME obligation. An expired worker lease returns the SAME ID to `needs-assignment` — a replacement
worker resumes it, never a duplicate.

## Fields (`repair_schema: "repair-obligation-v1"`)

| Field | Meaning |
|---|---|
| `obligation_id` | stable, IMMUTABLE dedupe key (`repair/<hash>`) |
| `repo` | the DELIVERABLE repo (owner/repo) — where the fix lands |
| `receipt_id` | the source desk-supervision/16 failure-receipt ID |
| `rows` | the failing Verify-row numbers |
| `blocker_kind` | one of the closed set (see below) |
| `responsible_role` | the next actor (worker / brief-author / human / operator / verifier) |
| `state` | one of the closed obligation-state set (see below) |
| `reproduction`, `expected` | the failed check and what it must do once repaired, so a replacement worker starts from the failure |
| `required_action` | the exact next act for a `waiting-external` obligation (never a worker task) |
| `attempt`, `claim_id`, `lease_ts` | the current attempt/lease; an expired lease makes the SAME ID assignable again |
| `repair_pr`, `repair_sha`, `repaired_by` | the linked repair PR, the revision it produced/merged at, and the worker identity that produced it |
| `original_pr`, `original_merged` | the original (failed) deliverable PR, and whether it merged — a merged original demands a fresh follow-up branch |
| `resolved_by_verifier`, `resolved_at_sha` | resolution provenance, set ONLY by a valid independent reverification |

### Blocker kinds (closed set) → routing

`implementation` → worker (repair the code) · `check-definition` → worker (amend the check under
the existing review policy) · `human-action` → human (`waiting-external`) · `environment` → operator
(`waiting-external`) · `unknown` → verifier (explicit triage, **never** an invented implementation
bug). A value outside the set makes the marker incomplete (visibly unclassified).

### Obligation states (closed set)

`needs-assignment` · `repairing` · `awaiting-review` · `awaiting-merge` ·
`awaiting-reverification` · `waiting-external` · `resolved`. These are OBLIGATION states, not brief
lifecycle cells: they describe where a repair stands, never what a board Status reads.

## The load-bearing invariant — resolution

A repair obligation is **scheduling state, not acceptance**. Worker completion, issue closure and a
merge ALONE can never resolve an implementation obligation — only a **valid independent verification
at the repaired revision** does. `deskkit.RepairObligation.Resolve` is the one door to `resolved`,
and it resolves only when all three hold:

1. a legitimate wake event exists — the state is `awaiting-reverification` (the repair merged);
2. the verification is INDEPENDENT — the verifier is non-empty and is NOT the worker that produced
   the repair (a same-actor pass is self-certification and is refused);
3. it is at the REPAIRED revision — the verified SHA equals the merged repair SHA (a stale or
   wrong-revision pass did not observe the fix and is refused).

A **merge WAKES reverification** (`MarkRepairMerged` → `awaiting-reverification`); it does not close
the obligation. The existing per-item claim and the reviewer/verifier identity gates remain the
barriers; this marker sits beside them.

## Where each piece runs

| Concern | Home |
|---|---|
| The obligation model, immutable key, reconcile, resolve guard, follow-up branch | `tools/desk/internal/deskkit/repairobligation.go` |
| Creating/reconciling an obligation on a failed/blocked verify | `tools/desk/cmd/verifyloop` (Land → `RepairObligationSink`; the SAFE dry-run default records nothing until the human-gated cutover) |
| Reading outstanding obligations across roots as a worker rework source, and forge-transition reconciliation | `tools/desk/cmd/fanoutloop` (`repair.go`) |
| Carrying the obligation onto the worker's claim + workpad, with the follow-up branch on a merged original | `tools/desk/cmd/fanoutloop` (`renderDispatchPrompt`) |

Invariants the tests pin (`tools/desk/cmd/fanoutloop/repair_test.go`):

- One verifier failure creates exactly one rework item with its reproduction and target repository.
- A duplicate delivery and a lost response reconcile to ONE obligation; a replacement worker resumes
  after a dead claim without duplicating it.
- A repair merge wakes independent verification; a same-actor or wrong-revision pass cannot resolve;
  a valid independent pass at the repaired revision resolves.
- Failed work delivered in a sibling with a merged original PR produces the correct repo, a fresh
  follow-up branch and the linked repair, with no duplicate of the original work.

## Migration

- **Legacy rows** in the projection (no `repair_schema`) parse as incomplete markers and stay
  visibly unclassified — never a fabricated obligation.
- **Rollback** is the previous released binary/skill pin: the additive projection is a new sidecar
  file the older reader never consults, and obligations are never erased to make a rollback look
  clean.
- **Reference/interim build.** The obligation-creating sink and the cross-root reader are wired with
  SAFE dry-run / injectable defaults; the autonomous cutover (the standing windows actually driving
  these paths, and the real issue-filing sink) is gate:human — BLOCKED-ON-IAN — exactly as the rest
  of the loopengine reference build.
