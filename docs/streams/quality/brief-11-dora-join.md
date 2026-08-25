---
brief: quality/11
title: DORA join — quality denominator + traced-CFR refinement + pluggable delivery-metrics source
why: >-
  A flow board measures delivery; it cannot say whether the change it shipped was
  DURABLE or whether its failure rate reflects TRACED defects rather than only incidents.
  This brief contributes a quality denominator (durable-change volume) and a traced-CFR
  refinement into the existing DORA collection, joined on the keys the board already uses,
  so delivery and quality numbers sit side by side and stay comparable to public
  baselines — without this tool becoming a delivery-metrics platform.
wave: 3
depends: ["quality/02", "quality/07"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by quality-stream authoring session
sources:
  - "docs/streams/quality/spec.md §8 — DORA join: quality denominator, CFR refinement with evidence-tier split, join keys, pluggable delivery-metrics source, DevLake domain-schema naming for comparability (not a dependency)"
  - "docs/streams/quality/spec.md §4.1/§4.2 — line-operation taxonomy and the 14-day churn window that define durable-change volume (consumes M1, brief-02)"
  - "docs/streams/quality/spec.md §5.3 — defect-inducing change rate and the trace-rate/evidence-tier that the traced CFR must ship with (consumes M2, brief-07)"
---

# Brief 11 — DORA join: quality denominator + traced-CFR, pluggable delivery-metrics source

## Context
files:
- NEW `qualgen/dorajoin/denominator.go` (planned) — the **quality denominator**:
  durable-change volume = landed lines MINUS 14-day churn MINUS `copied`, computed from
  the M1 aggregates (brief-02: line-operation taxonomy + churn). Reported per window,
  stream, and author-identity class, three-state where inputs are unmeasured.
- NEW `qualgen/dorajoin/cfr.go` (planned) — the **traced-CFR refinement**: the traced
  defect-inducing rate (from brief-07) reported ALONGSIDE incident-based CFR, never
  replacing it, with the evidence-tier split (tiers 1-3) visible on every number and the
  trace-rate published beside it (spec §10 honest-claims).
- NEW `qualgen/dorajoin/joinkeys.go` (planned) — the join-key resolver: **PR number +
  merge SHA + stream/task ID** (the keys the board already uses). Emits a three-state
  match flag; an unjoinable delivery record is `could-not-join`, never dropped.
- NEW `qualgen/dorajoin/source.go` (planned) — the `DeliveryMetricsSource` interface (a
  pluggable delivery-metrics source) with an in-tree reference adapter reading a
  file-based delivery-metrics feed; house wiring to a live collector is configuration.
- NEW `qualgen/dorajoin/schema.go` (planned) — output field naming that FOLLOWS DevLake's
  domain-layer names where a concept matches (`commits`, `pull_requests`, `incidents`),
  purely for cross-tool comparability. DevLake itself is NOT a dependency — this is naming
  only.
- CONSUMES `qualgen/metrics/` (planned) M1 aggregates (brief-02) and `qualgen/szz/` (planned) traces
  (brief-07).

facts:
- The delivery-metrics collector keeps collecting; this tool JOINS to it (spec §2, §8),
  it does not replace it. The reference source is file-based; the interface is the seam.
- Durable-change volume is exactly `landed_lines - churn_14d - copied` (spec §8); the
  churn window default is 14 days (spec §4.2), configurable, 14d the comparable default.
- Traced CFR is REPORTED ALONGSIDE incident CFR, not merged into it; every traced number
  ships its trace-rate and evidence-tier composition ("X% at Y% trace coverage", never
  bare X — spec §10).
- Join keys are PR number + merge SHA + stream/task ID; a squash or rewritten merge that
  breaks a key yields `could-not-join` (three-state, spec §3.2), never a silent zero.
- DevLake domain-schema naming is for COMPARABILITY only; no DevLake code, schema import,
  or runtime dependency is introduced.
- Out of scope: the M3 stage joins (brief-10) and the M4 reflexivity joins (brief-12);
  changing how delivery metrics are collected.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit the single-writer `QUALITY.md` on a branch (CI is its only writer).
- Traced CFR NEVER ships without its trace-rate and evidence-tier split beside it.
- PUBLIC-REPO / self-contained: no private repo names, no individuals, no withheld
  internal identifiers. Describe verdict issues and lanes generically.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `denominator.go`: compute durable-change volume =
   `landed_lines - churn_14d - copied` from brief-02 M1 aggregates, per window/stream/
   identity class, marking any component that is `could-not-measure` rather than zeroing.
2. Implement `cfr.go`: emit the traced defect-inducing rate from brief-07 alongside
   incident-based CFR, carrying the evidence-tier split and the trace-rate on every
   number. Refuse to emit a bare traced CFR with no trace-rate attached.
3. Implement `joinkeys.go`: resolve delivery records to quality records on PR number +
   merge SHA + stream/task ID; emit `matched` / `no-match` / `could-not-join`
   three-state; never drop an unjoinable record.
4. Implement `source.go`: define `DeliveryMetricsSource`, ship the file-based reference
   adapter, and register it as default; document that a live collector is a config-time
   adapter swap.
5. Implement `schema.go`: name output fields after DevLake domain-layer names where the
   concept matches, with a doc comment stating this is naming-for-comparability, not a
   dependency.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./dorajoin/` | exit 0 |
| 2 | `cd qualgen && go test ./dorajoin/` | exit 0; denominator, cfr, join-key, and source tests pass |
| 3 (DEREFERENCING) | `cd qualgen && go test ./dorajoin/ -run TestDurableVolumeFixture -v` | exit 0; a fixture with landed=1200 lines, 200 lines churned within 14d, 150 `copied` lines yields durable-change volume EXACTLY 850 (`1200-200-150`) — a specific asserted value, not a presence check |
| 4 (DEREFERENCING) | `cd qualgen && go test ./dorajoin/ -run TestTracedCFRCarriesTierAndRate -v` | exit 0; the emitted traced-CFR record carries a non-empty trace-rate and a tier-1/2/3 split; a serialization with a bare rate and no trace-rate FAILS the emit guard |
| 5 | `cd qualgen && go test ./dorajoin/ -run TestJoinKeyThreeState` | exit 0; a record whose merge SHA is unreachable via squash is emitted `could-not-join`, not dropped and not counted as a match |
| 6 | `cd qualgen && grep -c -E -e pull_requests -e incidents -e commits qualgen/dorajoin/schema.go` | exit 0; count >= 1 (output field names follow DevLake domain-layer names for comparability) |
| 7 | `cd qualgen && go test ./dorajoin/ -run TestDeliverySourcePluggable` | exit 0; the file-based reference adapter satisfies `DeliveryMetricsSource` and a second stub source swaps in without touching join logic |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (all four risk answers no — repo-agnostic OSS Go joining already-mined M1/M2
outputs to a pluggable delivery-metrics feed; read-only, no product value changed, no new
delivery-metrics platform). Reviewer confirms the traced CFR never ships without its
trace-rate + tier split, join keys are three-state, and DevLake naming carries no
dependency. Reviewer records verdict + date in the stream README table.
