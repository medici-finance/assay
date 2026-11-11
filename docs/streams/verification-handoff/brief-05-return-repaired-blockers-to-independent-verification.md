---
brief: assay:assay:verification-handoff:05
title: "Return repaired blockers to independent verification"
why: "A repair merge or issue closure can make a retry worthwhile, but neither proves the original brief now works. The original verifier must recover the correct subject and evidence without losing repair ownership history."
wave: 7
depends: ["verification-handoff/04", "desk-tools/46"]
unblocks: ["verification-handoff/06", "statusgen/35"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
gate-why: "Changes the verification wake and repair reconciliation boundary used to schedule, preserve or resolve verification work; human review must confirm that trust, ownership, retention and independent acceptance cannot be inferred from routing."
decision-trigger: start
issues: []
schema: brief-v2
version: 2
id: d5c6a0bd-ced8-496d-b74b-3f1ba3df4d43
authored: "2026-10-23 by assay-worker-app[bot]"
exec-tier: strong
exec-tier-why: "Cross-module shared policy, partial writes and independent actor/source binding require coordinated design and negative controls."
domain: complicated
sources:
  - "docs/proposals/fixed-delivery-verification/README.md — fixed-delivery completion amendment, 2026-10-27; planning only"
  - "freshness-checked 2026-10-27 @ 9a58a792c; existing PR 4579 and source/caller work map"
  - "docs/streams/verification-handoff/README.md — controlling implementation scope"
  - "spec/intake-exit-v1.md; docs/streams/desk-supervision/repair-obligation-v1.md; docs/verify-wake.md"
  - "freshness-checked 2026-10-23 @ fbe0331e2fc9f5c822c4edeafb8a18865c916828"
consumers:
  - "tools/desk/cmd/verifyloop/briefscan.go: follow-up verification-handoff/05 (this delivery; existing owner or planned extension)"
  - "tools/desk/cmd/verifyloop/repair.go: follow-up verification-handoff/05 (this delivery; existing owner or planned extension)"
  - "tools/desk/cmd/deskevidence/receiptvalidate.go: follow-up verification-handoff/05 (this delivery; existing owner or planned extension)"
  - "tools/desk/internal/deskkit/repairobligation.go: follow-up verification-handoff/05 (this delivery; existing owner or planned extension)"
  - "tools/desk/cmd/fanoutloop/repair.go: follow-up verification-handoff/05 (this delivery; existing owner or planned extension)"
  - "docs/verify-wake.md: follow-up verification-handoff/05 (this delivery; existing owner or planned extension)"
  - "statusgen/testdata/verification-handoff/05-mutations.json: follow-up verification-handoff/05 (this delivery; existing owner or planned extension)"
  - "changelog/verification-handoff-05.md: follow-up verification-handoff/05 (this delivery; existing owner or planned extension)"
---

# Brief 05 — Return repaired blockers to independent verification

## Context

files: tools/desk/cmd/verifyloop/briefscan.go, tools/desk/cmd/verifyloop/repair.go, tools/desk/cmd/deskevidence/receiptvalidate.go, tools/desk/internal/deskkit/repairobligation.go, tools/desk/cmd/fanoutloop/repair.go, docs/verify-wake.md, statusgen/testdata/verification-handoff/05-mutations.json (planned), changelog/verification-handoff-05.md (planned)

facts:
- Baseline main fbe0331e2fc9f5c822c4edeafb8a18865c916828, inspected 23 October 2026; source was draft and stream was parked at authoring. Scope approval and stream activation take effect on the maintainer's merge of PR #4206; per-brief gates remain separate.
- Intake discovery sees ordinary verifier bugs; 14 of a bounded 28 recent verifier-filed issues carried explicit intake triage, not proof of claims or repairs.
- The default loop filing/repair sinks print dry-run results. The live skill uses sanctioned deskfile/deskevidence paths; a printed URL is not durable work.
- Existing intake-exit-v1 has two writers and exact-key conformance tests. Its local log is not a fleet-wide source of assignment authority.
- Current scanner/readers differ on addressed handling. Default fanout STATUS.md input has no issue labels; the worker inbox needs an actual read.
- deskcore is public and imported by desk-tools; statusgen does not currently import it. Shared policy packages are planned, not shipped.
- Brief schemas v1-v3 are supported; no milestone or b-prefixed operational records are introduced by this authoring.

layering: verification wake and repair reconciliation owns this slice. Shared decision logic is an in-process public Go library call; no separate binary, daemon or subprocess protocol. Existing typed forge, custody, claims and evidence adapters own effects. No private import across modules and no new lifecycle database.

design-fit:
  owner: "verification wake and repair reconciliation"
  contract: none — reuse the existing issue projection, intake-exit, repair-obligation and wake owners; update their canonical index on implementation.
  retires: ["implicit routing/assignment inference and the duplicate predicate or partial-success ambiguity named in Task"]
  weight: "New binaries 0; new services 0; bounded contract/library/adapter changes. Record actual rule, field, refusal and flag deltas at implementation review."
  why-add: "Reuse existing semantic owners; a shared meaning is needed across existing callers. A new process, queue or acceptance authority would add failure modes without closing the handoff."

single-point-of-failure: stale disposition or successor identity; independent trust/admission/claim and exact-source acceptance checks remain at effect boundaries.

## Read first

- [Controlling scope, coverage and owner decisions](README.md#implementation-scope).
- [Intake exit contract](../../../spec/intake-exit-v1.md).
- [Existing repair-obligation contract](../desk-supervision/repair-obligation-v1.md).
- [Wake and retry contract](../../verify-wake.md).
- [Library-first direction](../../library-first.md).
- [Semantic owners](../../contracts.md).

## Human decision

Confirm that the concrete implementation preserves independent verification and existing human-only decisions while changing how failed work is retained and assigned. A routing record must never grant acceptance or wider execution permissions.

Options:
1. Approve the bounded implementation after its independent review and verification.
2. Request corrections while retaining the existing holds.
3. Hold the proposed change.

Default if no answer: none — blocks until answered.

## Ground rules

- Isolated feature branch and draft PR; no merge, source/stream/brief/PR-ready flip, release, installed pin change, workflow dispatch or live infrastructure effect in this brief.
- Recheck current main, claims, active PR overlap, controlling sources, semantic owners and module imports at pickup/resume; record observed revisions/coverage and continue, adapt or hold. A future API or draft spec is not shipped support.
- Preserve actor/authority separation, legacy identities and immutable Evidence/outcomes. Do not fabricate intake acknowledgement, assignment, historical execution or verified/done from labels or issue closure.
- New shared parsing/policy must be direct library calls, never a new helper executable. Preserve existing online read/custody boundaries.

## Task

### Fixed-delivery completion amendment — 2026-10-27

This is a planning amendment under [fixed-delivery
verification](../../proposals/fixed-delivery-verification/README.md), not runtime implementation or
activation. Preserve the original Verify table and Evidence. The changed fixed-subject,
historical-subject and row-reuse policy waits for the precise existing authority decisions; an old
decision token grants none of these changes. Existing approved work retains its original contract.
At pickup, pin this brief/version, the controlling contracts, actual producer/consumer versions and
open PRs; adapt or hold only the affected slice. No new evaluator, record store, desk, tracer or
qualification stage is authorized.

**Owner: exact-subject return after repair.** Consume desk-tools/46's admitted run/subject identity
as well as /04 linkage. A bookkeeping or prerequisite repair resumes the original D/A where still
applicable; a code or acceptance repair requires independently attributed successor D/A. Preserve
the original failed delivery and backlink; do not move its immutable binding or relabel an old
observation.

- Wake only the original affected obligation under the existing receipt/wake owner. Repair merge or
  issue closure grants neither independent acceptance nor authority for a changed original row. Keep
  unaffected applicable results; require new proof where source, criteria or conditions changed. -
  Extend the existing wake/recheck fixtures with metadata-only historical repair, true product
  repair at a successor delivery, repaired explicit range needing exact authority,
  duplicate/out-of-order events and a wrong-source recheck. Only admissible independent proof plus
  remaining original human gates resolves the matching handoff. /35 records unresolved historical
  dispositions instead of silently closing them.

### Existing task obligations

1. Consume /02 and /04 linkage through the existing receipt validator, wake classifier and repair-obligation state. Repair merge, external prerequisite completion and valid input change may wake the correct original brief/held rows, never mark it verified. Reuse the current independent-verifier and exact-repaired-revision resolution guard; preserve open-blocker holds, unreadable refs as could-not-check and explicit reasoned recheck behavior.
2. Route one current recheck request to the existing verification owner/Evidence PR with immutable original acceptance and repair revisions. Reconcile duplicate merge events and restarts without multiple verifier lanes. If that owner/home is gone, transfer through fresh sanctioned dispatch/admission and retain the old blocked evidence, rather than relying on a stale local pointer or modifying original commands.
3. Resolve only the matching handoff/repair obligation after admissible independent verification at the repaired revision and the original brief's remaining acceptance/human gates. Same actor, wrong revision, missing witness or a closed issue alone cannot resolve it. Partial pass, continued environment wait and new distinct failures retain their actual row accounting and ownership; no automatic acceptance transition or blanket issue closure.
4. Record the final backlink to the original blocker, repair PR and new outcome through existing immutable evidence/projection writers. A failed final landing leaves a visible incomplete transfer. Preserve desk-tools/45 queue semantics, /50 blocker classification and any unchanged old receipt compatibility; no second evaluator or lifecycle database.

## Interface and completion contract

DoD: deliver the production behavior, regression and fault-injection coverage named below; route every affected caller to the canonical owner; update the stated contract/operator guidance and changelog. A contract declaration or function import alone is insufficient. Consumer routes are prospective authoring obligations, not already-correlated implementation evidence. Keep unknown/unreadable distinct from empty/ready, and retain explicit missing source, external owner and activation holds.

## Verify

These named tests and mutation manifests are FUTURE DELIVERABLES, not authoring-time evidence. Each exercises the actual entrypoint/adapter, with the wrong-but-plausible mutation named in Expect. Implement executable mutation commands in the declared manifest and record fail-first/red/green observations. The named PASS assertion prevents an absent test selector from passing.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd tools/desk && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRepairWakeRecheck$' ./cmd/verifyloop > "$handoff_out" && grep -q -- '--- PASS: TestRepairWakeRecheck ' "$handoff_out")` | exit 0 and named PASS; real brief scan holds an open unchanged blocker, wakes after its repair, and requests the existing verifier lane once; unreadable or external-still-waiting stays held; treating merge as pass fails. |
| 2 | check:ci +flow +mutation | sh | `(cd tools/desk && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRepairIndependentResolution$' ./cmd/deskevidence > "$handoff_out" && grep -q -- '--- PASS: TestRepairIndependentResolution ' "$handoff_out")` | exit 0 and named PASS; actual landing/reconciliation rejects same-worker, wrong revision and missing witness while a qualified independent pass resolves only its case; failed backlink landing remains incomplete. |

## Evidence

<!-- Empty by design. A non-implementer records actual command/exit/output, Date and Runner, admitted source, evidence links and remaining coverage. No current test or acceptance result is claimed. -->

## Review

Gate: human, derived from risk. Independent review checks source/consumer contracts, operation compatibility and negative controls; this plan authorizes neither acceptance nor activation.
