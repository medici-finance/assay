---
brief: assay:assay:verification-handoff:02
title: "Persist verifier blockers without losing the handoff"
why: "A recorded failure can lose its outcome when a verifier home is reaped, and the reference loop can print a filing without creating one. A durable, truthful handoff must survive partial success and restart."
wave: 2
depends: ["verification-handoff/01", "desk-tools/49"]
unblocks: ["verification-handoff/04"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
gate-why: "Changes the verifier filing and evidence landing boundary used to schedule, preserve or resolve verification work; human review must confirm that trust, ownership, retention and independent acceptance cannot be inferred from routing."
decision-trigger: start
issues: []
schema: brief-v2
version: 1
id: 4a9cfb95-927f-4d86-bcd8-781293f51290
authored: "2026-10-23 by assay-worker-app[bot]"
exec-tier: strong
exec-tier-why: "Cross-module shared policy, partial writes and independent actor/source binding require coordinated design and negative controls."
domain: complicated
sources:
  - "docs/streams/verification-handoff/README.md — controlling implementation scope"
  - "spec/intake-exit-v1.md; docs/streams/desk-supervision/repair-obligation-v1.md; docs/verify-wake.md"
  - "freshness-checked 2026-10-23 @ fbe0331e2fc9f5c822c4edeafb8a18865c916828"
consumers:
  - "tools/desk/cmd/verifyloop/durable.go: follow-up verification-handoff/02 (this delivery; existing owner or planned extension)"
  - "tools/desk/cmd/verifyloop/repair.go: follow-up verification-handoff/02 (this delivery; existing owner or planned extension)"
  - "tools/desk/cmd/verifyloop/land.go: follow-up verification-handoff/02 (this delivery; existing owner or planned extension)"
  - "tools/desk/cmd/deskevidence/: follow-up verification-handoff/02 (this delivery; existing owner or planned extension)"
  - "tools/desk/cmd/deskfile/: follow-up verification-handoff/02 (this delivery; existing owner or planned extension)"
  - "tools/desk/internal/deskkit/repairobligation.go: follow-up verification-handoff/02 (this delivery; existing owner or planned extension)"
  - "plugins/assay/skills/verify-desk/SKILL.md: follow-up verification-handoff/02 (this delivery; existing owner or planned extension)"
  - "statusgen/testdata/verification-handoff/02-mutations.json: follow-up verification-handoff/02 (this delivery; existing owner or planned extension)"
  - "changelog/verification-handoff-02.md: follow-up verification-handoff/02 (this delivery; existing owner or planned extension)"
---

# Brief 02 — Persist verifier blockers without losing the handoff

## Context

files: tools/desk/cmd/verifyloop/durable.go, tools/desk/cmd/verifyloop/repair.go, tools/desk/cmd/verifyloop/land.go, tools/desk/cmd/deskevidence/, tools/desk/cmd/deskfile/, tools/desk/internal/deskkit/repairobligation.go, plugins/assay/skills/verify-desk/SKILL.md, statusgen/testdata/verification-handoff/02-mutations.json (planned), changelog/verification-handoff-02.md (planned)

facts:
- Baseline main fbe0331e2fc9f5c822c4edeafb8a18865c916828, inspected 23 October 2026; source was draft and stream was parked at authoring. Scope approval and stream activation take effect on the maintainer's merge of PR #4206; per-brief gates remain separate.
- Intake discovery sees ordinary verifier bugs; 14 of a bounded 28 recent verifier-filed issues carried explicit intake triage, not proof of claims or repairs.
- The default loop filing/repair sinks print dry-run results. The live skill uses sanctioned deskfile/deskevidence paths; a printed URL is not durable work.
- Existing intake-exit-v1 has two writers and exact-key conformance tests. Its local log is not a fleet-wide source of assignment authority.
- Current scanner/readers differ on addressed handling. Default fanout STATUS.md input has no issue labels; the worker inbox needs an actual read.
- deskcore is public and imported by desk-tools; statusgen does not currently import it. Shared policy packages are planned, not shipped.
- Brief schemas v1-v3 are supported; no milestone or b-prefixed operational records are introduced by this authoring.

layering: verifier filing and evidence landing owns this slice. Shared decision logic is an in-process public Go library call; no separate binary, daemon or subprocess protocol. Existing typed forge, custody, claims and evidence adapters own effects. No private import across modules and no new lifecycle database.

design-fit:
  owner: "verifier filing and evidence landing"
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

1. Consume the reviewed canonical case lookup owned by desk-tools/49; recheck its actual API at pickup and hold this dependent slice if it is absent. Do not duplicate exact-key search, title dedupe, ambiguous-response reconciliation or suppress-all-on-brief logic. Bind the original brief, admitted source/run, exact failing or held rows, existing Evidence PR and blocker issue across the issue and outcome. Missing causation stays unknown: the verified brief is not automatically the defective implementation owner.
2. Exercise both FAIL and BLOCKED/unlandable Land branches. Implement the live configured adapter behind the existing filing/repair interfaces through sanctioned typed forge/custody/evidence writers. Keep reference/dry-run modes explicitly non-durable and non-assigned. Do not activate the autonomous sink or widen any App permission as part of implementation. Ordinary bug filing needs no blanket to:intake; use deliberate role handoffs only under /01. A required filing failure remains an incomplete handoff, not a printed success.
3. Reconcile create-ack loss and retries to the same case. If issue creation succeeds but outcome or Evidence landing fails, retain the known issue ref, diagnostic and original run artifacts; report partial completion and an actionable recovery owner. Do not forge an outcome when Evidence admission refuses. Protect admitted homes/artifacts from normal cleanup until required landings succeed or custody is transferred to a durable recovery record. Reference retained artifacts without exposing private material or relying on a disappearing scratch path.
4. Complete a new immutable outcome only through existing deskevidence guards, exact actor/source and required receipt checks. Preserve old outcomes/Evidence and execution attribution; no manual backfill from counts. Where original artifacts were lost, arrange fresh independent dispatch and append a new record instead of reconstructing a historical witness. Keep existing verifier-queue classification and budget controls.

## Interface and completion contract

DoD: deliver the production behavior, regression and fault-injection coverage named below; route every affected caller to the canonical owner; update the stated contract/operator guidance and changelog. A contract declaration or function import alone is insufficient. Consumer routes are prospective authoring obligations, not already-correlated implementation evidence. Keep unknown/unreadable distinct from empty/ready, and retain explicit missing source, external owner and activation holds.

## Verify

These named tests and mutation manifests are FUTURE DELIVERABLES, not authoring-time evidence. Each exercises the actual entrypoint/adapter, with the wrong-but-plausible mutation named in Expect. Implement executable mutation commands in the declared manifest and record fail-first/red/green observations. The named PASS assertion prevents an absent test selector from passing.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd tools/desk && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestHandoffProducerRecovery$' ./cmd/verifyloop > "$handoff_out" && grep -q -- '--- PASS: TestHandoffProducerRecovery ' "$handoff_out")` | exit 0 and named PASS; FAIL and BLOCKED/unlandable Land branches each persist through the configured live adapter; FileBug, repair-sink or outcome-write failure leaves an incomplete handoff and retains recovery links; lost create acknowledgement reconciles one case; inactive/default mode reports non-durable truthfully; distinct row/case/root remains distinct; treating print as assignment fails. |
| 2 | check:ci +flow +mutation | sh | `(cd tools/desk && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestHandoffPartialLanding$' ./cmd/deskevidence > "$handoff_out" && grep -q -- '--- PASS: TestHandoffPartialLanding ' "$handoff_out")` | exit 0 and named PASS; issue-success/outcome-failure and cleanup interruption retain issue/run recovery links, no invented outcome, and retry joins the same case; deleting custody early or declaring complete before both writes fails. |

## Evidence

<!-- Empty by design. A non-implementer records actual command/exit/output, Date and Runner, admitted source, evidence links and remaining coverage. No current test or acceptance result is claimed. -->

## Review

Gate: human, derived from risk. Independent review checks source/consumer contracts, operation compatibility and negative controls; this plan authorizes neither acceptance nor activation.
