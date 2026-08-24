---
brief: quality/10
title: M3 stage attribution — deterministic dossier + judgment stage-call + per-stage defect ledger
why: >-
  "Are we getting better" only becomes actionable when it decomposes by stage: a spec
  that keeps changing, a plan that misses its own surface, and an implementation that
  violates its plan each need a DIFFERENT remedy, and aggregate defect counts hide which
  one to apply. M3 walks each traced defect's inducing PR back through its provenance
  chain and names the stage the defect escaped at — the novel measurement in this stream,
  and the corpus every M4 reflexivity join reads.
wave: 3
depends: ["quality/07"]
unblocks: ["quality/12"]
effort: L
gate: model
exec-tier: strong
exec-tier-why: cross-artifact provenance reasoning (walking inducing PR -> brief -> spec and comparing at-inducing-time text against the diff) plus designing a judgment-classification that stays deterministic in its dossier and spot-auditable in its call (questions a, b).
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by quality-stream authoring session
sources:
  - "docs/streams/quality/spec.md §6 — M3 stage attribution: stage table, deterministic-dossier/judgment-call split, per-stage defect ledger, pluggable provenance-linkage adapter"
  - "docs/streams/quality/spec.md §3.2 — three-state instrument invariant (untraceable is a first-class output, never a silent zero)"
  - "docs/streams/quality/spec.md §10 — honest-claims discipline (stage attribution is evidence-assembled, judgment-classified, spot-audited; never measured)"
---

# Brief 10 — M3 stage attribution: dossier + ledger + pluggable provenance-linkage adapter

## Context
files:
- NEW `qualgen/attribution/provenance.go` (planned) — the `ProvenanceLinkage` adapter
  interface: given an inducing PR/commit, resolve its chain links (PR -> task/brief ->
  stream -> spec/ruling) as far as the target's provenance permits. A **generic
  commit->issue reference adapter** ships as the reference implementation (parses
  `Fixes/Closes/Refs #N`-style references and the PR's linked issue); richer chains are
  supplied as configuration, not new mining code.
- NEW `qualgen/attribution/dossier.go` (planned) — DETERMINISTIC dossier assembly: given a
  trace from brief-07, gather brief text as it stood at inducing-merge time, the inducing
  diff, the at-head review verdicts, and any postdating rulings, into a stable, ordered,
  content-addressed dossier. Same inputs -> byte-identical dossier.
- NEW `qualgen/attribution/stage.go` (planned) — the stage classifier producing one of
  `spec` / `brief` / `implementation` / `untraceable`, plus the `review-escape` overlay
  recorded for every defect (which lanes approved the inducing PR at head). The call MAY
  be model-assisted, but every call records the exact dossier it decided from and is
  spot-auditable; a deterministic rule-based classifier is the default and the fallback.
- NEW `qualgen/attribution/ledger.go` (planned) — the per-stage defect ledger writer:
  defects/window by stage, per stream, as an APPEND-ONLY artifact under the tracking root
  (`docs/quality/attribution/`, one file per defect), correctable only by TOMBSTONE
  amendment, never silent edit.
- NEW `qualgen/attribution/commitissue_adapter.go` (planned) — the reference
  commit->issue linkage adapter registered by default.
- CONSUMES `qualgen/szz/` (planned) trace output from brief-07 (`defects.jsonl`: fix_pr,
  inducing_commit(s), inducing_pr(s), confidence, evidence tier).

single-point-of-failure: the stage CALL is a recorded judgment, not a measurement — the
control that keeps it honest is that the deterministic DOSSIER (the defensible layer) is
assembled independently of the call and is re-derivable and spot-auditable, so a wrong
call is catchable against a fixed dossier rather than trusted blind.

facts:
- Stage tests (spec §6): `spec` = change faithfully implements its brief AND the brief
  faithfully reflects the spec as it stood (the requirement was wrong); `brief` =
  implementation matches the brief but the plan did not cover the defect surface (the
  plan was wrong); `implementation` = the plan covered it and the change violates it (the
  work was wrong); `review-escape` = orthogonal overlay recorded for every defect, naming
  the lanes that approved the inducing PR at head; `untraceable` = chain broken
  (pre-provenance history, no linkage, could-not-trace) — reported as such, NEVER binned
  elsewhere.
- Three-state invariant (spec §3.2): a broken or missing chain link is emitted as
  `untraceable`/`could-not-attribute`, never as a silent stage guess.
- Honest-claims (spec §10): artifacts and README language say "evidence-assembled,
  judgment-classified, spot-audited," never "measured."
- **Critical-path contract**: this brief is on the head chain (01 -> 06 -> 07 -> 10 ->
  12 -> 14). Its two outputs are consumed downstream by brief-12 (quality/12): the
  **per-stage defect ledger** (gate-yield and ritual joins read defect counts by stage)
  and the **`review-escape` overlay** (gate-yield accounting attributes each escape to the
  lanes that approved its inducing PR). Keep both stable and documented as a seam.
- Model assistance is OPTIONAL: the deterministic rule-based classifier must produce a
  defensible stage for every dossier on its own; the model-assisted path only refines it,
  and records its dossier hash either way.
- Out of scope: any new history mining (M3 consumes brief-07 traces); the M4 joins
  (brief-12); wiring a house-specific richer provenance adapter (that is configuration).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit the single-writer `QUALITY.md` on a branch (CI is its only writer).
- Artifacts are APPEND-ONLY: amend by tombstone, never edit or delete a prior dossier.
- PUBLIC-REPO / self-contained: no private repo names, no individuals, no withheld
  internal identifiers. Describe review lanes and verdict issues generically.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Define `ProvenanceLinkage` in `provenance.go`: `Resolve(inducing) (Chain, error)`
   returning the ordered chain links reachable for the target, each link marked
   `resolved` / `absent` / `could-not-resolve` (three-state). Ship the generic
   commit->issue reference adapter in `commitissue_adapter.go` and register it as the
   default; document that richer chains register additional adapters via config.
2. Implement DETERMINISTIC dossier assembly in `dossier.go`: for a brief-07 trace, gather
   (a) the inducing brief/plan text as of inducing-merge time, (b) the inducing diff,
   (c) at-head review verdicts, (d) postdating rulings. Emit a stable, ordered,
   content-addressed dossier; assert byte-identical output for identical inputs.
3. Implement the stage classifier in `stage.go`: a deterministic rule-based classifier
   applying the §6 stage tests over the dossier, returning `spec` / `brief` /
   `implementation` / `untraceable`, PLUS the `review-escape` overlay for every defect.
   Provide an optional model-assisted hook that refines the call but records the dossier
   hash it decided from; a broken chain yields `untraceable`, never a guess.
4. Implement the per-stage ledger in `ledger.go`: write one append-only file per defect
   under `docs/quality/attribution/`, and emit a per-stage/per-stream/per-window rollup
   (`spec` / `brief` / `implementation` / `untraceable` counts + `review-escape`
   distribution). Support tombstone amendment.
5. Document the two downstream seams consumed by brief-12 (per-stage ledger schema +
   `review-escape` overlay schema) in package doc comments, and keep them stable.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./attribution/` | exit 0 |
| 2 | `cd qualgen && go test ./attribution/` | exit 0; provenance-adapter, dossier-determinism, stage, and ledger tests pass |
| 3 | `cd qualgen && go test ./attribution/ -run TestDossierDeterministic -count=2` | exit 0; the same trace input yields a byte-identical (same content hash) dossier across runs |
| 4 (DEREFERENCING) | `cd qualgen && go test ./attribution/ -run TestStagePlantedBriefGap -v` | exit 0; fixture repo has a planted defect whose inducing change FAITHFULLY implements its brief while the brief text does NOT cover the defect surface — the deterministic classifier returns stage `brief` (plan-gap), NOT `implementation` and NOT `spec` |
| 5 (DEREFERENCING) | `cd qualgen && go test ./attribution/ -run TestStageUntraceableNotZeroed -v` | exit 0; a fixture inducing commit with a BROKEN provenance chain classifies as `untraceable` and is counted in the `untraceable` bucket — never silently placed in `implementation` |
| 6 | `cd qualgen && go test ./attribution/ -run TestReviewEscapeOverlay` | exit 0; the overlay for a defect lists exactly the lanes recorded as approving its inducing PR at head (the seam brief-12 consumes) |
| 7 | `cd qualgen && go test ./attribution/ -run TestLedgerAppendOnlyTombstone` | exit 0; a correction adds a tombstone amendment; the prior dossier file is unchanged on disk |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (all four risk answers no — repo-agnostic OSS Go over already-mined traces;
read-only against the target, append-only artifacts, no shared product value changed).
Reviewer confirms: (a) the deterministic dossier is re-derivable and independent of the
stage call; (b) a broken chain yields `untraceable`, never a binned guess; (c) the
per-stage ledger + `review-escape` overlay seams consumed by brief-12 are documented and
stable. Reviewer records verdict + date in the stream README table.
