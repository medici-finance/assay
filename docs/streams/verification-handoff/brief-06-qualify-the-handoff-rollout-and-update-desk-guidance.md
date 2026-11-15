---
brief: assay:assay:verification-handoff:06
title: "Qualify the handoff rollout and update desk guidance"
why: "A set of individually passing adapters can still strand work at their boundaries. A source-to-repair-to-independent-recheck replay must prove the new path before any live adoption."
wave: 8
depends: ["verification-handoff/05", "desk-tools/51", "statusgen/38", "desktools-v2/73"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
gate-why: "Changes the handoff integration qualification boundary used to schedule, preserve or resolve verification work; human review must confirm that trust, ownership, retention and independent acceptance cannot be inferred from routing."
decision-trigger: start
issues: []
schema: brief-v2
version: 2
id: adba75c3-f110-4c38-bbe8-f2dcf0240002
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
  - "tools/desk/internal/handofftest/: follow-up verification-handoff/06 (this delivery; existing owner or planned extension)"
  - "statusgen/testdata/verification-handoff/: follow-up verification-handoff/06 (this delivery; existing owner or planned extension)"
  - "tools/desk/README.md: follow-up verification-handoff/06 (this delivery; existing owner or planned extension)"
  - "plugins/assay/skills/verify-desk/SKILL.md: follow-up verification-handoff/06 (this delivery; existing owner or planned extension)"
  - "plugins/assay/skills/intake-desk/SKILL.md: follow-up verification-handoff/06 (this delivery; existing owner or planned extension)"
  - "plugins/assay/skills/worker-desk/SKILL.md: follow-up verification-handoff/06 (this delivery; existing owner or planned extension)"
  - "docs/verification-handoff.md: follow-up verification-handoff/06 (this delivery; existing owner or planned extension)"
  - "statusgen/testdata/verification-handoff/06-mutations.json: follow-up verification-handoff/06 (this delivery; existing owner or planned extension)"
  - "changelog/verification-handoff-06.md: follow-up verification-handoff/06 (this delivery; existing owner or planned extension)"
---

# Brief 06 — Qualify the handoff rollout and update desk guidance

## Context

files: tools/desk/internal/handofftest/ (planned), statusgen/testdata/verification-handoff/ (planned), tools/desk/README.md, plugins/assay/skills/verify-desk/SKILL.md, plugins/assay/skills/intake-desk/SKILL.md, plugins/assay/skills/worker-desk/SKILL.md, docs/verification-handoff.md (planned), statusgen/testdata/verification-handoff/06-mutations.json (planned), changelog/verification-handoff-06.md (planned)

facts:
- Baseline main fbe0331e2fc9f5c822c4edeafb8a18865c916828, inspected 23 October 2026; source was draft and stream was parked at authoring. Scope approval and stream activation take effect on the maintainer's merge of PR #4206; per-brief gates remain separate.
- Intake discovery sees ordinary verifier bugs; 14 of a bounded 28 recent verifier-filed issues carried explicit intake triage, not proof of claims or repairs.
- The default loop filing/repair sinks print dry-run results. The live skill uses sanctioned deskfile/deskevidence paths; a printed URL is not durable work.
- Existing intake-exit-v1 has two writers and exact-key conformance tests. Its local log is not a fleet-wide source of assignment authority.
- Current scanner/readers differ on addressed handling. Default fanout STATUS.md input has no issue labels; the worker inbox needs an actual read.
- deskcore is public and imported by desk-tools; statusgen does not currently import it. Shared policy packages are planned, not shipped.
- Brief schemas v1-v3 are supported; no milestone or b-prefixed operational records are introduced by this authoring.

layering: handoff integration qualification owns this slice. Shared decision logic is an in-process public Go library call; no separate binary, daemon or subprocess protocol. Existing typed forge, custody, claims and evidence adapters own effects. No private import across modules and no new lifecycle database.

design-fit:
  owner: "handoff integration qualification"
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

**Owner: complete-loop qualification through the existing production-adapter replay.** Depend on
/05, desk-tools/51 and statusgen/38; no additional qualification stage or record. This fixture
qualification does not depend on exhausting the real /35 corpus or on the larger
shared-launcher/recorded-read-set programs.

- Exercise prospective review context, independently admitted D/A, execution W, later recording E,
  main health H, row reuse, partial landing, repair and independent return through the real existing
  owners. Advance H and repair bookkeeping without changing valid D acceptance; change a relevant
  input/criterion/condition and hold its affected proof. A later applicable failure within the same
  context must supersede a pass. - Replay actual historical shapes from PRs 4559/4561/4562 without
  treating them as approved closures: missing consumer subject, scope/outcome refusal and protected
  command approval. Include desk-supervision/13's substantive two-route failure, mixed/unrecoverable
  old evidence, verified-but-not-done continuation and unchanged completed-record hashes. Use
  synthetic public fixtures, not private incident artifacts. - Crash before/after every owned write
  and replay duplicate/out-of-order events: one accepted subject and one remaining authorized
  effect, no lost artifacts, silent false-ready state or duplicate verifier lane. Check existing CI
  actually triggers for supported Evidence merge paths (4486); keep a real H regression visible
  without replacing D evidence. - Existing repair completion prerequisites remain explicit: /73 and PR 4102 supply consistent rendered/admitted Evidence; PR 4371 and issue 4596 supply canonical path/CI identity handling in /37; issue 4486 supplies supported Evidence merges that trigger CI. Consume the existing fixing work, never reimplement its policy in this qualification brief. Missing merged/released/activated support is a named adoption hold, not a simulated success.
- Extend the existing handoff integration and mutation suites, then
  qualify the released paired tools/skills and actual consumer pins/installed identities with /51.
  Report unsupported or unreadable paths as unknown. No workflow promotion, release dispatch, live
  sink activation or runtime implementation is authorized by this planning PR.

### Existing task obligations

1. Create a controlled end-to-end fixture that exercises production adapters with real module boundaries: verifier blocked observation -> durable issue/outcome -> intake disposition -> successor claim -> repair event -> fresh independent verification. Include a #4112-shaped case where source support already exists and the next work is installation qualification; no duplicate implementation brief should be produced. Include #4181-style partial landing/custody loss and #4200-style stopped scan-carrier recovery.
2. Replay duplicate and out-of-order delivery, crash between writes, unreadable/unauthorized roots, old unknown fields, closed duplicates, multiple failures on one brief and intentionally addressed inboxes. Prove one accountable successor per case, no silent work loss and no false ready/verified state. Test old tools refusing new required semantics before effects and an explicit compatible legacy observation path. Fixtures are synthetic/public, not a copy of private incident material.
3. Update only the handoff sections of verifier/intake/worker guidance and dispatch kits at their canonical owners. Reconcile desk-tools/51's existing lane-to-action rewrite; do not fork it or expand its scope. Reconcile the separate evidence-format PR #4202 if/when delivered; this stream does not reimplement that guard or claim it is installed. Enumerate actual consumers and keep command/version examples truthful.
4. Prepare a bounded candidate/adoption checklist with source/module versions, supported tool/reader capabilities, release artifacts, reviewed consumer pins, exact installed identities and replay evidence. Default/reference mode remains non-durable where not configured. Live sink activation, operator installation, release dispatch and autonomy cutover remain separate human-authorized acts; this brief supplies qualification artifacts only. Preserve rollback readability and visible unresolved handoffs, never delete records to make a rollback green.

## Interface and completion contract

DoD: deliver the production behavior, regression and fault-injection coverage named below; route every affected caller to the canonical owner; update the stated contract/operator guidance and changelog. A contract declaration or function import alone is insufficient. Consumer routes are prospective authoring obligations, not already-correlated implementation evidence. Keep unknown/unreadable distinct from empty/ready, and retain explicit missing source, external owner and activation holds.

## Verify

These named tests and mutation manifests are FUTURE DELIVERABLES, not authoring-time evidence. Each exercises the actual entrypoint/adapter, with the wrong-but-plausible mutation named in Expect. Implement executable mutation commands in the declared manifest and record fail-first/red/green observations. The named PASS assertion prevents an absent test selector from passing.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd tools/desk && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestHandoffEndToEnd$' ./internal/handofftest/... > "$handoff_out" && grep -q -- '--- PASS: TestHandoffEndToEnd ' "$handoff_out")` | exit 0 and named PASS; production adapters complete the scoped repair and independent recheck once; already-delivered support becomes qualification work; stopping after a routing comment, dropping outcome custody or skipping inbox discovery fails. |
| 2 | check:ci +flow +mutation | sh | `(cd tools/desk && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestHandoffMixedVersions$' ./internal/handofftest/... > "$handoff_out" && grep -q -- '--- PASS: TestHandoffMixedVersions ' "$handoff_out")` | exit 0 and named PASS; old/new operation pairs refuse missing required support before writes, legacy observations stay visible, rollback loses no handoff; claiming installation from source-only tests or enabling live default fails. |

## Evidence

<!-- Empty by design. A non-implementer records actual command/exit/output, Date and Runner, admitted source, evidence links and remaining coverage. No current test or acceptance result is claimed. -->

## Review

Gate: human, derived from risk. Independent review checks source/consumer contracts, operation compatibility and negative controls; this plan authorizes neither acceptance nor activation.
