---
brief: quality/12
title: M4 gate-yield accounting + ritual-effectiveness natural-experiment joins
why: >-
  M1-M3 measure the CODE; M4 turns the same instruments on the PROCESS, so a review lane
  or an authoring ritual is judged on outcomes rather than on faith. Gate-yield accounting
  says which lanes catch defects and which let them through; the ritual joins ask whether
  a stronger execution tier or a deeper Verify table measurably pays for itself. Because
  this is a natural experiment, not a controlled one, the same brief must bake in the
  honest-claims caveat so a bare ritual-effectiveness number is never published.
wave: 4
depends: ["quality/10"]
unblocks: ["quality/14"]
effort: M
gate: model
exec-tier: strong
exec-tier-why: observational-validity reasoning — designing joins that acknowledge confounders and enforce brittleness-band stratification as the minimum control so no naive causal number leaks (question b).
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by quality-stream authoring session
sources:
  - "docs/streams/quality/spec.md §7.1 — gate-yield accounting: catch-rate, escape-rate, latency per review lane, using M3's review-escape overlay"
  - "docs/streams/quality/spec.md §7.2 — ritual effectiveness (natural experiments): cost per durable KLOC by model tier x brittleness band, Verify-depth vs escape rate, agent-PR survival + first-pass approval rates, review-discipline guardrails; confounders acknowledged, brittleness-band stratification the minimum control"
  - "docs/streams/quality/spec.md §10 — honest-claims discipline (never a bare observational number; state confounders and stratification)"
---

# Brief 12 — M4 gate-yield accounting + ritual-effectiveness joins

## Context
files:
- NEW `qualgen/reflex/gateyield.go` (planned) — gate-yield accounting PER REVIEW LANE:
  defects caught pre-merge (request-changes findings whose flagged surface matches a later
  trace) vs escapes attributed to that lane by **M3's `review-escape` overlay** (from
  brief-10). Output: catch-rate, escape-rate, latency cost per lane.
- NEW `qualgen/reflex/ritual.go` (planned) — the natural-experiment joins: authoring
  attributes (model tier, Verify-depth, lane coverage) joined to downstream M1/M2
  outcomes. Headline joins: **cost per durable KLOC by model tier x brittleness band** and
  **Verify-depth vs escape rate**; plus the industry-named agent metrics (spec §7.2):
  **agent-PR survival rate**, **first-pass approval rate**, and the
  **review-discipline guardrails** (% PRs merged without review, time-in-review trend —
  emitted as alarmed budgets, not dashboard lines). NO new mining — a JOIN of existing
  outputs only.
- NEW `qualgen/reflex/stratify.go` (planned) — the OBSERVATIONAL-VALIDITY guard:
  brittleness-band stratification as the MINIMUM control, and a confounders block attached
  to every readout. This module REFUSES to emit a ritual-effectiveness number that is not
  brittleness-band stratified — the honest-claims enforcement point.
- CONSUMES `qualgen/attribution/` (planned) per-stage ledger + `review-escape` overlay (brief-10),
  and `qualgen/metrics/` (planned) (M1, brief-02) + `qualgen/szz/` (planned) (M2, brief-07) outcomes it joins.

single-point-of-failure: the ONE control preventing a misleading causal claim is the
stratify-guard in `stratify.go`; the design keeps it independent by making it a
serialization/emit gate that BOTH the gate-yield and ritual outputs must pass, and by
attaching the confounders block as data, so a naive number cannot be emitted even if a
caller forgets to ask for stratification.

facts:
- **Corpus-maturity precondition**: M4 needs a SEASONED M3 corpus. Per the spec's
  sequencing, wave 4 is deliberately late — it consumes an M1-M3 corpus that must EXIST
  and season first (>= 2 windows of measurement). This brief's code can be written and
  unit-tested against FIXTURES now, but any real readout is calendar-gated on corpus
  maturity; the tests here run on synthetic corpora, not on live numbers.
- M4 is a JOIN of existing outputs — no new mining (spec §7). Gate-yield reads M3's
  `review-escape` overlay (brief-10); ritual joins read M1 durable-change and M2 traces.
- Ritual effectiveness is a NATURAL EXPERIMENT, not a controlled one (spec §7.2): harder
  code gets stronger tiers AND more churn, so confounders are acknowledged in EVERY
  readout and brittleness-band stratification is the minimum control.
- Honest-claims (spec §10): never publish a bare ritual-effectiveness number; a readout
  states its confounders and its stratification, or it is not emitted.
- **Critical-path contract**: this brief consumes brief-10's per-stage ledger and
  `review-escape` overlay directly and unblocks brief-14 (quality/14, closing the loop).
- Review lanes and verdict issues are described GENERICALLY (public repo); no lane is
  named after a private process, individual, or withheld identifier.
- Out of scope: session-forensics telemetry joins (brief-13); any live/outward readout
  (calendar-gated on corpus maturity + any operator publication decision).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit the single-writer `QUALITY.md` on a branch (CI is its only writer).
- NEVER emit a ritual-effectiveness number that is not brittleness-band stratified and
  carrying its confounders block — the guard is mandatory, not advisory.
- PUBLIC-REPO / self-contained: no private repo names, no individuals, no withheld
  internal identifiers. Review lanes and verdict issues stay generic.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `gateyield.go`: per review lane, count defects caught pre-merge (a
   request-changes finding whose flagged surface matches a later trace) vs escapes the
   M3 `review-escape` overlay attributes to that lane; emit catch-rate, escape-rate, and
   latency cost per lane, three-state where inputs are unmeasured.
2. Implement `ritual.go`: join authoring attributes (model tier, Verify-depth, lane
   coverage) to downstream M1 durable-change and M2 trace outcomes; compute cost per
   durable KLOC by model tier x brittleness band and Verify-depth vs escape rate. Read
   only existing outputs — no new mining.
3. Implement `stratify.go`: enforce brittleness-band stratification as the minimum control
   and attach a confounders block to every readout. Make it the emit gate BOTH gateyield
   and ritual outputs pass through; refuse (error, not warn) any un-stratified
   ritual-effectiveness serialization.
4. Wire the brief-10 seams: read the per-stage ledger and `review-escape` overlay through
   their documented schemas; fail three-state (`could-not-join`) rather than guess when a
   defect has no overlay entry.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./reflex/` | exit 0 |
| 2 | `cd qualgen && go test ./reflex/` | exit 0; gate-yield, ritual-join, and stratify-guard tests pass |
| 3 (DEREFERENCING) | `cd qualgen && go test ./reflex/ -run TestGateYieldFixture -v` | exit 0; a fixture lane with 8 pre-merge catches and 2 escapes (per the planted `review-escape` overlay) yields catch-rate EXACTLY 0.8 and escape-rate EXACTLY 0.2 — specific asserted values, not presence |
| 4 (DEREFERENCING, NEGATIVE-PATH) | `cd qualgen && go test ./reflex/ -run TestRitualNumberRefusedWithoutStratification -v` | exit 0; attempting to emit a cost-per-durable-KLOC number WITHOUT brittleness-band stratification returns an error and produces NO serialized readout — the honest-claims guard bites |
| 5 | `cd qualgen && go test ./reflex/ -run TestRitualReadoutCarriesConfounders` | exit 0; a properly stratified readout serializes WITH a non-empty confounders block attached |
| 6 | `cd qualgen && go test ./reflex/ -run TestReviewEscapeOverlayThreeState` | exit 0; a defect with no `review-escape` overlay entry is emitted `could-not-join`, never attributed to a lane by guess |
| 7 | `cd qualgen && go test ./reflex/ -run TestNoNewMining` | exit 0; the join reads only M1/M2/M3 artifact fixtures — no git-history access path is invoked (mining seam is not called) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->

Runner: opus-4.8[1m]-verifier (verify-desk dispatch, non-implementer) · Date: 2026-09-02 · public-assay merged HEAD `ecf722d068e0fa3c6273eff68931ba6c1fb96e84` (brief-12 code at `841ca8e`) · offline (`KUBECONFIG=/dev/null`).

| # | Command | Exit | Key observed output |
|---|---------|------|---------------------|
| 1 | `cd qualgen && go build ./... && go vet ./reflex/` | 0 | clean build + vet (go1.26.5 darwin/arm64) |
| 2 | `cd qualgen && go test ./reflex/` | 0 | `ok …/qualgen/reflex` |
| 3 (dereferencing) | `go test ./reflex/ -run` the `GateYieldFixture` case | 0 | PASS — asserts `Catches==8`, `Escapes==2`, `CatchRate` measured `==0.8`, `EscapeRate==0.2`; a different-lane escape does not pollute; zero/zero lane could-not-measure |
| 4 (dereferencing, negative) | `go test ./reflex/ -run` the `RitualNumberRefusedWithoutStratification` case | 0 | PASS — asserts `err != nil` AND `b == nil` (no bytes on refusal); error text contains `un-stratified`; explicit `unknown` band refused identically |
| 5 | `go test ./reflex/ -run` the `RitualReadoutCarriesConfounders` case | 0 | PASS — stratified readout serializes WITH non-empty confounders |
| 6 | `go test ./reflex/ -run` the `ReviewEscapeOverlayThreeState` case | 0 | PASS — a defect with no overlay entry emits could-not-join, never guessed |
| 7 | `go test ./reflex/ -run` the `NoNewMining` case | 0 | PASS — join reads only M1/M2/M3 artifact fixtures; no git-history mining seam invoked |

RISK-VALUE: DERIVED — `bandSplitLow`/`bandSplitHigh` = `1.0/3.0`, `2.0/3.0` @ `qualgen/reflex/stratify.go:51-52` — §7.2 requires three strata so a stronger tier's expected concentration on the HIGH band is not collapsed into a neighbour; equal-thirds is the canonical assumption-free split of a bounded percentile into three ordered bands. Reversible bucketing const in a read-only, calendar-gated analysis tool. Other literals (`defaultDeepVerifyThreshold=5`, band label strings) rank last. gate:model, all risk `no`.

Verdict: PASS — all 7 rows exit 0; the two dereferencing rows assert exact values (0.8/0.2) and the negative path bites (error + no bytes + `un-stratified`), not presence checks. Advancing `implemented → verified`.

## Review
Gate: model (all four risk answers no — repo-agnostic OSS Go joining already-recorded
M1-M3 artifacts; read-only, no product value changed). Reviewer confirms: (a) no
ritual-effectiveness number can be emitted un-stratified or without its confounders block
(the negative-path row proves the guard); (b) gate-yield reads the brief-10
`review-escape` overlay and fails three-state on a missing entry; (c) the brief bakes in
that M4 is a natural experiment, never a controlled one. Reviewer records verdict + date
in the stream README table.
