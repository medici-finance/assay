---
brief: assay:assay:verification-handoff:04
title: "Discover and claim the successor repair work"
why: "A worker label is not a dispatch. Addressed issues are absent from work-ready, while the default fanout board reads a file without issue labels, so a routed repair can remain undiscovered."
wave: 6
depends: ["verification-handoff/02", "verification-handoff/03"]
unblocks: ["verification-handoff/05"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
gate-why: "Changes the worker discovery and dispatch boundary used to schedule, preserve or resolve verification work; human review must confirm that trust, ownership, retention and independent acceptance cannot be inferred from routing."
decision-trigger: start
issues: []
schema: brief-v2
version: 1
id: a0b9cdf1-fc8d-4056-9006-33dbd55b5934
authored: "2026-10-23 by assay-worker-app[bot]"
exec-tier: strong
exec-tier-why: "Cross-module shared policy, partial writes and independent actor/source binding require coordinated design and negative controls."
domain: complicated
sources:
  - "docs/streams/verification-handoff/README.md — controlling implementation scope"
  - "spec/intake-exit-v1.md; docs/streams/desk-supervision/repair-obligation-v1.md; docs/verify-wake.md"
  - "freshness-checked 2026-10-23 @ fbe0331e2fc9f5c822c4edeafb8a18865c916828"
consumers:
  - "tools/desk/cmd/fanoutloop/: follow-up verification-handoff/04 (this delivery; existing owner or planned extension)"
  - "tools/desk/cmd/deskdispatch/: follow-up verification-handoff/04 (this delivery; existing owner or planned extension)"
  - "tools/desk/cmd/issueboard/: follow-up verification-handoff/04 (this delivery; existing owner or planned extension)"
  - "tools/desk/internal/issueflow/: follow-up verification-handoff/04 (this delivery; existing owner or planned extension)"
  - "plugins/assay/skills/worker-desk/SKILL.md: follow-up verification-handoff/04 (this delivery; existing owner or planned extension)"
  - "statusgen/testdata/verification-handoff/04-mutations.json: follow-up verification-handoff/04 (this delivery; existing owner or planned extension)"
  - "changelog/verification-handoff-04.md: follow-up verification-handoff/04 (this delivery; existing owner or planned extension)"
---

# Brief 04 — Discover and claim the successor repair work

## Context

files: tools/desk/cmd/fanoutloop/, tools/desk/cmd/deskdispatch/, tools/desk/cmd/issueboard/, tools/desk/internal/issueflow/, plugins/assay/skills/worker-desk/SKILL.md, statusgen/testdata/verification-handoff/04-mutations.json (planned), changelog/verification-handoff-04.md (planned)

facts:
- Baseline main fbe0331e2fc9f5c822c4edeafb8a18865c916828, inspected 23 October 2026; source was draft and stream was parked at authoring. Scope approval and stream activation take effect on the maintainer's merge of PR #4206; per-brief gates remain separate.
- Intake discovery sees ordinary verifier bugs; 14 of a bounded 28 recent verifier-filed issues carried explicit intake triage, not proof of claims or repairs.
- The default loop filing/repair sinks print dry-run results. The live skill uses sanctioned deskfile/deskevidence paths; a printed URL is not durable work.
- Existing intake-exit-v1 has two writers and exact-key conformance tests. Its local log is not a fleet-wide source of assignment authority.
- Current scanner/readers differ on addressed handling. Default fanout STATUS.md input has no issue labels; the worker inbox needs an actual read.
- deskcore is public and imported by desk-tools; statusgen does not currently import it. Shared policy packages are planned, not shipped.
- Brief schemas v1-v3 are supported; no milestone or b-prefixed operational records are introduced by this authoring.

layering: worker discovery and dispatch owns this slice. Shared decision logic is an in-process public Go library call; no separate binary, daemon or subprocess protocol. Existing typed forge, custody, claims and evidence adapters own effects. No private import across modules and no new lifecycle database.

design-fit:
  owner: "worker discovery and dispatch"
  contract: S-claim
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

1. Wire explicit intake/worker successor observations and the worker addressed inbox into the real default worker discovery path, using existing typed forge/issueboard adapters and /01 assessment. Preserve separate population coverage: work-ready is not an inbox read, and STATUS.md cannot prove labels were observed. A failed repo or partial scan remains could-not-check; never substitute a cached empty result or route around a trust refusal.
2. Follow the accepted disposition to the existing repair issue, merged executable brief/placeholder or external lane. A brief-authoring task is not permission to implement the still-unapproved design. Reuse existing issue/brief claim keys and canonical case linkage so inbox, placeholder and successor views cannot dispatch duplicate work. Existing active claims, open fixing PRs and human/parked holds retain precedence; a typed successor grants no new eligibility.
3. Carry original brief/rows, blocker ref, disposition and repair target onto the existing claim/workpad and draft PR handoff. Record claim/actor observation back to the existing tracked handoff, so scheduled/claimed is distinguishable from merely routed. When the original delivery already merged, use a follow-up repair branch; do not resurrect its old claim or overwrite another worker. Lease loss resumes the same work under existing claim authority.
4. Keep pending external actions visible but non-worker-dispatchable, and preserve independent decision/verification card semantics. Reconcile concurrent queue updates before effects; no new worker loop, daemon, label-based trust bypass or special repair credential.

## Interface and completion contract

DoD: deliver the production behavior, regression and fault-injection coverage named below; route every affected caller to the canonical owner; update the stated contract/operator guidance and changelog. A contract declaration or function import alone is insufficient. Consumer routes are prospective authoring obligations, not already-correlated implementation evidence. Keep unknown/unreadable distinct from empty/ready, and retain explicit missing source, external owner and activation holds.

## Verify

These named tests and mutation manifests are FUTURE DELIVERABLES, not authoring-time evidence. Each exercises the actual entrypoint/adapter, with the wrong-but-plausible mutation named in Expect. Implement executable mutation commands in the declared manifest and record fail-first/red/green observations. The named PASS assertion prevents an absent test selector from passing.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd tools/desk && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRepairInboxDiscovery$' ./cmd/fanoutloop > "$handoff_out" && grep -q -- '--- PASS: TestRepairInboxDiscovery ' "$handoff_out")` | exit 0 and named PASS; actual default discovery obtains worker inbox items omitted from work-ready and accounts for unreadable roots; label-less STATUS rows cannot masquerade as inbox coverage; removing the inbox read fails. |
| 2 | check:ci +flow +mutation | sh | `(cd tools/desk && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRepairSuccessorClaim$' ./cmd/deskdispatch > "$handoff_out" && grep -q -- '--- PASS: TestRepairSuccessorClaim ' "$handoff_out")` | exit 0 and named PASS; issue/inbox/placeholder aliases reach one canonical claim, existing holder or ambiguous successor blocks effects, one eligible successor launches once; minting a new key for the second view fails. |

## Evidence

<!-- Empty by design. A non-implementer records actual command/exit/output, Date and Runner, admitted source, evidence links and remaining coverage. No current test or acceptance result is claimed. -->

## Review

Gate: human, derived from risk. Independent review checks source/consumer contracts, operation compatibility and negative controls; this plan authorizes neither acceptance nor activation.
