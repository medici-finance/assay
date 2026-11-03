---
stream: verification-handoff
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: []
board: generated
spec: docs/streams/verification-handoff/README.md
---

# Verification blockers reach accountable repair and recheck

**Version:** v1.0
**Status:** approved — approval of this implementation scope and stream activation take effect on the maintainer's merge of [PR #4206](https://github.com/medici-finance/assay/pull/4206), the ruling record under lifecycle §8.4.

**Routes-to:** `docs/streams/verification-handoff/`

This stream closes the verifier -> intake -> worker -> independent verification handoff.
With v1.0.57 published, this change proposes scope approval and stream activation together
on the maintainer's merge of PR #4206. Brief completion and individual decision/verification
gates remain separate; no earlier forge-recorded approval is asserted.
Existing work and evidence remain authoritative.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Share the blocker disposition and routing contract](brief-01-share-the-blocker-disposition-and-routing-contract.md) | 0 | M | implemented | — | — |
| 02 | [Persist verifier blockers without losing the handoff](brief-02-persist-verifier-blockers-without-losing-the-handoff.md) | 2 | M | todo | — | — |
| 03 | [Require an accountable intake disposition before repair scheduling](brief-03-require-an-accountable-intake-disposition-before-repair-scheduling.md) | 1 | M | implemented | — | — |
| 04 | [Discover and claim the successor repair work](brief-04-discover-and-claim-the-successor-repair-work.md) | 3 | M | todo | — | — |
| 05 | [Return repaired blockers to independent verification](brief-05-return-repaired-blockers-to-independent-verification.md) | 4 | M | todo | — | — |
| 06 | [Qualify the handoff rollout and update desk guidance](brief-06-qualify-the-handoff-rollout-and-update-desk-guidance.md) | 5 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Implementation scope

### Observed problem and controlling sources

Baseline: Assay main `fbe0331e2fc9f5c822c4edeafb8a18865c916828` (23 October 2026). The source files and
canonical contracts below were inspected; they describe observed behavior, not live installation.
[Library-first](../../library-first.md), [semantic owners](../../contracts.md),
[intake exits](../../../spec/intake-exit-v1.md), [wake receipts](../../verify-wake.md)
and [repair obligations](../desk-supervision/repair-obligation-v1.md) are inherited inputs.
The intake exit contract remains a draft reference contract; this scope does not approve it.

The read-only operational sample was the latest 100 verifier-created forge entries, filtered
for non-PR/non-attestation issues: 28 issues created 22–23 October, 14 with explicit intake
triage comments. The other 14 include 11 questions, one closed environment issue and two bugs
without such a comment. This is bounded discovery/triage evidence, not a fleet ownership census.
[Issue 4112](https://github.com/medici-finance/assay/issues/4112) has investigation but no observed
accepted intake successor. [Issue 4181](https://github.com/medici-finance/assay/issues/4181)
records three lost outcome landings. [Issue 4200](https://github.com/medici-finance/assay/issues/4200)
explicitly relies on a routing comment while its scan carrier is not publishing.
No incident is claimed resolved by this plan.

Facts verified in source: verifyloop's default Durable and RepairSink are dry-run;
issueboard intake sees ordinary unaddressed verifier issues; work-ready excludes addressed
items; default fanout consumes label-less STATUS.md; scanner addressed handling differs
from readers; scanloop's mechanical creation is not semantic intake judgment.
Existing outcome validation checks blocker existence and wake eligibility, not repair assignment.
No quantitative service/throughput target was measured; this scope promises observable safety,
recovery and ownership, not an invented latency SLA or causation claim.

### Required behavior and authority

Every required verifier blocker must remain visible until it has an accountable disposition:
scoping/brief-authoring work, a directly implementable repair, an external role/action,
or a linked duplicate owner. Unknown is explicit. A comment, issue label, placeholder,
source-code fix or release publication alone proves none of assignment, installation or acceptance.
Ordinary bugs need no mandatory to:intake label. An intentional addressee needs its real inbox
reader and onward disposition; no leakage into generic work-ready/scanner selection.

Reuse canonical issue/PR, intake exit, case identity, claim, outcome and repair-obligation
owners. Per-file projections and local metrics can describe them, not create a second task
or acceptance database. Contract evolution must distinguish mechanical event exit from
accountable repair disposition, preserve old unknown records and qualify both writers/readers.
A pure public library in the existing deskcore module is the proposed shared policy owner;
statusgen and desk-tools call it directly. No new module or binary is proposed.

Reliability criteria: partial writes retain known IDs and admitted artifact custody; retry
reconciles before another create; restart retains unacknowledged/successor-owed work; the
same issue/case has one active claim across views. Unavailable evidence is never manufactured.
Security criteria: trust, label history, source admission, actor independence and human holds
remain separately checked. Repair merge only wakes a recheck; valid independent evidence at
the repaired revision and original closure gates establish acceptance.

### Proposed outcomes and criterion coverage

Milestone proposals are prose only. Current advertised brief v1-v3 support does not establish
milestone/prefixed-reference support, so no operational milestone YAML or new required fields
are written. Legacy supported brief IDs remain in executable plans until negotiated adoption.
The intended milestone owner is the process owner; an independent verifier supplies evidence,
and the existing human/source gates authorize acceptance. No outcome is accepted here.

| Proposed promise version | Outcome / criteria | Contributors | Contributor wave span |
|---|---|---|---|
| assay:assay:verification-handoff:m-01@1 | C01 one shared selection/disposition meaning; C02 durable truthful failure handoff; C03 intake produces real successor or external owner | b-01, b-02, b-03 | 0–2 |
| assay:assay:verification-handoff:m-02@1 | C04 real worker discovery and one claim; C05 repair wakes independent exact-revision recheck; C06 interruption preserves evidence/ownership | b-04, b-05 (C06 also b-02) | 3–4 |
| assay:assay:verification-handoff:m-03@1 | C07 end-to-end replay; C08 mixed-version and adoption proof without activation | b-06 | 5 |

Contribution membership is owned only in this proposed scope; no duplicate brief milestone field.
M02 acceptance requires M01@1; M03 acceptance requires M02@1. These are outcome acceptance
relationships, not extra start gates on every contributor. Explicit brief edges own scheduling.
Criteria are checked by each contributor's named Verify tests plus the joined replay in /06;
fixture results do not substitute for independent live acceptance or an installation receipt.

### Existing owners and overlap

- desk-tools/49 owns canonical verifier filing dedupe; /02 explicitly depends on it. Do not
  implement its search/reconciliation twice. Its existing Task, Verify and Evidence stay intact;
  this authoring only adds the reciprocal scheduling edge to its unblocks metadata.
- desk-supervision/17 and /33 supply repair model and intake-exit records. Extend existing owners,
  not a new work database. Their implementation/source records do not authorize autonomous cutover.
- desk-tools/45 owns verification queue classification and /50 blocker classification; preserve them.
- desk-tools/51 owns the larger verifier lane-to-action rewrite; /06 is a bounded handoff delta
  reconciled with that delivery. It is not a prerequisite for the pure contract to start.
- Evidence-attribution PR #4202 is independent. Consume its delivered behavior if present;
  do not duplicate it or infer installation. This stream does not rewrite PR #4114's evidence.
- The future milestone resolver/capability plan is independent: no new milestone grammar is required
  to deliver this repair using current supported IDs and operation-specific compatibility checks.

A title/branch scan of 40 open Assay PRs found the known evidence-format PR and Evidence landings; it did not establish an overlapping implementation of this handoff. This bounded search is not file-level ownership clearance.

Coverage: inspected this public Assay root and its actual Go modules at the baseline above.
Other configured/private consumer roots, live claims, installed consumers and infrastructure
were not audited for implementation readiness. Recheck affected roots, open PRs and owners
at pickup; private evidence stays in its own trust boundary. The work does not claim fleet readiness.

### Critical path and current head

The new implementation head is /01: existing public deskcore and internal issueflow are present;
the shared disposition extension and statusgen import do not exist. This slice needs no release,
cluster or new service. The other actual head is existing desk-tools/49, currently todo in the
inspected board: it blocks /02 and must deliver the canonical identity seam. Stream activation
and source-only fixtures do not make that implementation prerequisite available.

```text
01 shared contract -> 03 intake disposition ----+
01 + desk-tools/49 -> 02 durable producer -------+-> 04 worker pickup -> 05 independent recheck -> 06 qualification
```

The new critical chain has four dependency transitions; the wave span also preserves the existing external prerequisite wave. Neither is a time estimate.
Scope approval and stream activation take effect together on the maintainer's merge of
PR #4206. Risk decisions and independent verification remain separate gates. The v1.0.57 baseline release is published; future artifact
publication, consumer pins, operator installation and human-authorized live cutover remain
requirements only where the implementation needs them.
Do not activate an existing reference sink merely because its adapter passes fixture tests.

### Dependency waves

- Wave 0: 01 Share the blocker disposition and routing contract.
- Wave 1: 03 Require an accountable intake disposition before repair scheduling.
- Wave 2: 02 Persist verifier blockers without losing the handoff.
- Wave 3: 04 Discover and claim the successor repair work.
- Wave 4: 05 Return repaired blockers to independent verification.
- Wave 5: 06 Qualify the handoff rollout and update desk guidance.

### Admission and authoring validation

This change proposes scope approval and stream activation for ratification by the maintainer's
merge of PR #4206, following the existing measured-status approval-at-merge pattern.
All implementation tests and mutation maps remain future deliverables; Evidence is intentionally
empty. Adopted contract versions, individual human decisions, actual consumer qualification and
autonomous sink activation remain separate obligations. The source approval here does not approve
the inherited draft root contracts or authorize release, installation, acceptance or live cutover.

Authoring validation passed: source-built diff-scoped read-only lint (`--changed`), dependency/inverse
and wave checks, link/metadata checks and whitespace validation. Independent authoring review
checked owner reuse, activation holds and separate FAIL/BLOCKED producer coverage. Those checks are not runtime
verification or evidence that any handoff has been fixed.
