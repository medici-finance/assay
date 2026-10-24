---
brief: assay:assay:verification-handoff:03
title: "Require an accountable intake disposition before repair scheduling"
why: "Intake is triaging real verification issues, but comments such as routed to worker or owned by brief author can leave no accepted successor work. New issue scanning also does not enforce that semantic readiness was assessed."
wave: 1
depends: ["verification-handoff/01"]
unblocks: ["verification-handoff/04"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Changes the intake and scanner adapters boundary used to schedule, preserve or resolve verification work; human review must confirm that trust, ownership, retention and independent acceptance cannot be inferred from routing."
decision-trigger: start
issues: []
schema: brief-v2
version: 1
id: c2da5fe1-953a-4605-8bed-76f89fe848f7
authored: "2026-10-23 by assay-worker-app[bot]"
exec-tier: strong
exec-tier-why: "Cross-module shared policy, partial writes and independent actor/source binding require coordinated design and negative controls."
domain: complicated
sources:
  - "docs/streams/verification-handoff/README.md — controlling implementation scope"
  - "spec/intake-exit-v1.md; docs/streams/desk-supervision/repair-obligation-v1.md; docs/verify-wake.md"
  - "freshness-checked 2026-10-23 @ fbe0331e2fc9f5c822c4edeafb8a18865c916828"
consumers:
  - "tools/desk/cmd/scanloop/adapter.go: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "tools/desk/cmd/scanloop/lane.go: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "tools/desk/cmd/scanloop/exitrecord.go: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "tools/desk/cmd/issueboard/: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "statusgen/scanissues.go: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "statusgen/intakeexits.go: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "spec/intake-exit-v1.md: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "plugins/assay/skills/intake-desk/SKILL.md: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "statusgen/testdata/verification-handoff/03-mutations.json: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "changelog/verification-handoff-03.md: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
---

# Brief 03 — Require an accountable intake disposition before repair scheduling

## Context

files: tools/desk/cmd/scanloop/adapter.go, tools/desk/cmd/scanloop/lane.go, tools/desk/cmd/scanloop/exitrecord.go, tools/desk/cmd/issueboard/, statusgen/scanissues.go, statusgen/intakeexits.go, spec/intake-exit-v1.md, plugins/assay/skills/intake-desk/SKILL.md, statusgen/testdata/verification-handoff/03-mutations.json (planned), changelog/verification-handoff-03.md (planned)

facts:
- Baseline main fbe0331e2fc9f5c822c4edeafb8a18865c916828, inspected 23 October 2026; source was draft and stream was parked at authoring. Scope approval and stream activation take effect on the maintainer's merge of PR #4206; per-brief gates remain separate.
- Intake discovery sees ordinary verifier bugs; 14 of a bounded 28 recent verifier-filed issues carried explicit intake triage, not proof of claims or repairs.
- The default loop filing/repair sinks print dry-run results. The live skill uses sanctioned deskfile/deskevidence paths; a printed URL is not durable work.
- Existing intake-exit-v1 has two writers and exact-key conformance tests. Its local log is not a fleet-wide source of assignment authority.
- Current scanner/readers differ on addressed handling. Default fanout STATUS.md input has no issue labels; the worker inbox needs an actual read.
- deskcore is public and imported by desk-tools; statusgen does not currently import it. Shared policy packages are planned, not shipped.
- Brief schemas v1-v3 are supported; no milestone or b-prefixed operational records are introduced by this authoring.

layering: intake and scanner adapters owns this slice. Shared decision logic is an in-process public Go library call; no separate binary, daemon or subprocess protocol. Existing typed forge, custody, claims and evidence adapters own effects. No private import across modules and no new lifecycle database.

design-fit:
  owner: "intake and scanner adapters"
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

1. Use /01 to expose observed-but-unacknowledged, disposition-owed and successor-owed blockers in existing intake reads. Do not lose an inbound item when polling advances or a session restarts. Acknowledge only through the existing authorized issue/record path and retain source-event identity, deciding role and evidence. Do not treat raised-by as trust or ownership and do not infer triage from issue closure, label presence or a mechanical scan PR.
2. Extend the existing intake judgment/land interface to record one current accountable disposition with a real successor or explicit owner/action; preserve per-event history. Scope/design-needed work queues author-spec/author-brief with a strong author and a linked authoring task/PR; only a sufficiently scoped direct repair can proceed as issue work. A source fix already delivered routes to installation/qualification rather than another implementation brief. External prerequisites require role, exact next action and wake condition; human/risk decisions keep their existing independent gate.
3. Make scanner placeholder scheduling and issueboard projections consume /01 rather than differing label predicates. Raw intake representation may exist before judgment, but must not be dispatchable repair work until its required disposition is settled. Keep untrusted, could-not-check, needs-decision and addressed work outside the ordinary ready lane. Old records lacking readiness remain visibly classification-needed; no bulk promotion or destructive regeneration.
4. Reconcile issue-lane and committed-register writers with the canonical versioned intake-exit contract, including restart/repeated-event behavior. A closed duplicate points to the existing owner and work; a stopped scan carrier leaves successor-owed work visible instead of treating the triage comment as a completed transfer. No new automatic judgment engine, global queue or unconditional to:intake stamping.

## Interface and completion contract

DoD: deliver the production behavior, regression and fault-injection coverage named below; route every affected caller to the canonical owner; update the stated contract/operator guidance and changelog. A contract declaration or function import alone is insufficient. Consumer routes are prospective authoring obligations, not already-correlated implementation evidence. Keep unknown/unreadable distinct from empty/ready, and retain explicit missing source, external owner and activation holds.

## Verify

These named tests and mutation manifests are FUTURE DELIVERABLES, not authoring-time evidence. Each exercises the actual entrypoint/adapter, with the wrong-but-plausible mutation named in Expect. Implement executable mutation commands in the declared manifest and record fail-first/red/green observations. The named PASS assertion prevents an absent test selector from passing.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd tools/desk && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestIntakeHandoffDisposition$' ./cmd/scanloop > "$handoff_out" && grep -q -- '--- PASS: TestIntakeHandoffDisposition ' "$handoff_out")` | exit 0 and named PASS; actual lane/land path distinguishes needs-authoring, simple repair, already-delivered qualification and external prerequisite; restart retains unacknowledged work; a bare routing comment or mechanical placeholder cannot satisfy readiness. |
| 2 | check:ci +flow +mutation | sh | `(cd statusgen && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanHandoffAdmission$' . > "$handoff_out" && grep -q -- '--- PASS: TestScanHandoffAdmission ' "$handoff_out")` | exit 0 and named PASS; actual scanner and intake export preserve legacy records as unknown and align ordinary/addressed/held eligibility with issueboard fixtures; scheduling before disposition or ignoring the addressed exclusion fails. |

## Evidence

<!-- Empty by design. A non-implementer records actual command/exit/output, Date and Runner, admitted source, evidence links and remaining coverage. No current test or acceptance result is claimed. -->

## Review

Gate: human, derived from risk. Independent review checks source/consumer contracts, operation compatibility and negative controls; this plan authorizes neither acceptance nor activation.
