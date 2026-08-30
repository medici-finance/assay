---
brief: quality/06
title: M2 fix identification — pluggable fix-linkage adapter + GitHub-labels reference adapter + evidence tiers
why: >-
  SZZ defect lineage begins by knowing which commits are FIXES. Getting that wrong poisons every
  downstream defect number. This brief builds the entry point: a pluggable fix-linkage adapter (an
  issue is defect-classed; a commit/PR closes that issue), a GitHub-labels reference adapter that works
  on any GitHub repo from day one, and three ranked evidence tiers so a keyword-guess fix is never
  silently merged with a defect-labeled one. Each fix records its tier and tier composition is itself a
  reported metric — the honest basis on which the B-SZZ trace (quality/07) then stands.
wave: 1
depends: ["quality/01"]
unblocks: ["quality/07"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by quality-stream authoring session
sources:
  - "docs/streams/quality/spec.md §5.1 — fix identification: precedence-ordered evidence tiers, pluggable linkage adapter"
  - "docs/streams/quality/spec.md §3.1 — profile-B adapter rules (adapter-based fix identification; no in-repo writes)"
  - "docs/streams/quality/spec.md §10 — honest-claims discipline (evidence-tier composition reported, never merged)"
  - "docs/streams/quality/spec.md §3.2 — three-state instrument invariant"
---

# Brief 06 — M2 fix identification (pluggable linkage adapter + evidence tiers)

## Context

files:
- NEW `qualgen/fixlinkage.go` (planned) (+ `qualgen/fixlinkage_test.go` (planned)) — the pluggable fix-linkage
  ADAPTER INTERFACE and the `DefectFix` record. The interface answers two questions per candidate:
  is a referenced issue DEFECT-CLASSED, and does a commit/PR CLOSE that issue. It classifies each
  candidate fix into one of three precedence-ordered evidence tiers and records the tier on the fix.
- NEW `qualgen/adapters/githublabels.go` (planned) (+ test) — the reference fix-linkage adapter:
  resolves `Fixes #N` / `Closes #N` linkage and reads GitHub issue LABELS to decide defect-classed
  (bug/defect/incident-labeled). It also treats a repo's configured verdict-issue LABEL SET (a generic
  issue-label source, configured per target) as defect-classed where present — no internal identifiers
  are hardcoded.
- NEW `qualgen/testdata/fixid/` (planned) — a fixture repo + a stubbed issue-label source with PLANTED
  fixes at each tier (a defect-labeled-issue closer, a `fix/`-branch / `fix:`-title PR, a
  keyword-only-message commit) plus a non-fix, so tiering is dereferenceable against known answers.
- Reads the miner skeleton history access + three-state plumbing from `qualgen/` (quality/01). Emits the
  `DefectFix` records into the `defects.jsonl` schema declared by quality/05.

facts:
- THREE EVIDENCE TIERS, precedence order (§5.1), strongest first:
  - **tier 1** — the commit/PR closes/references a DEFECT-CLASSED issue (`Fixes #N` where N carries a
    bug/defect/incident label, including a repo's configured verdict-issue label lane).
  - **tier 2** — the PR is classified `fix` by the repo's PR TAXONOMY (branch prefix `fix/`,
    conventional-commit `fix:` title).
  - **tier 3** — KEYWORD FALLBACK (fix/bug/defect/regression in the message) — the weakest tier,
    reported SEPARATELY, NEVER silently merged with tiers 1–2.
- Every `DefectFix` RECORDS its evidence tier. TIER COMPOSITION (the tier-1/2/3 split) is itself a
  reported metric (§5.1, §10) — a defect-inducing rate must ship with its tier composition.
- INTERFACE CONTRACT for quality/07: the `DefectFix` record is exactly what the B-SZZ inducing trace
  (quality/07) consumes. It carries at minimum: the fix commit/PR identity, the closed issue reference
  (or none), the evidence TIER, and a three-state `identified` flag. quality/07 blames the fix's
  changed lines at its parent — it depends on this record shape and MUST NOT re-derive fix
  identification. Declaring this seam is a load-bearing deliverable of THIS brief.
- ADAPTER-BASED (§3.1 profile-B): fix identification is behind the pluggable interface; the
  GitHub-labels adapter is the first reference adapter, other linkage sources are CONFIG, not new
  mining code. NO in-repo writes to the target — records land under the tracking root (`--out`).
- Three-state (§3.2): a candidate whose linkage cannot be resolved is `could-not-identify`, never
  silently treated as a non-fix.
- Out of scope: the inducing-commit blame/trace (quality/07); derived defect metrics beyond tier
  composition; any provenance/stage attribution (quality/10).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch + draft PR
  only.
- Stop at `implemented` — you do not set verified/done.
- NEVER write into the target repo — records go under the tracking root only (§3.1). NEVER commit a
  generated artifact on a branch (single writer = CI, quality/05).
- Three-state everywhere (§3.2); keyword-tier fixes reported separately, never merged upward.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Define `qualgen/fixlinkage.go` (planned): the fix-linkage ADAPTER INTERFACE (given a candidate commit/PR:
   resolve any closed-issue reference; decide whether that issue is defect-classed) and the `DefectFix`
   record. State the record's field contract explicitly (fix identity, closed-issue ref or none,
   evidence tier, three-state `identified` flag) — this is the seam quality/07 consumes. Implement the
   three-tier precedence classifier: tier 1 (defect-labeled issue) > tier 2 (PR taxonomy `fix`) > tier 3
   (keyword fallback), stopping at the strongest tier that matches, and record the tier on the fix.
2. Implement `qualgen/adapters/githublabels.go` (planned): the reference adapter. Resolve `Fixes/Closes #N`
   linkage, read issue labels to decide defect-classed (bug/defect/incident + a per-target-configured
   verdict-issue label set, referenced generically as a configured issue-label source — no internal IDs
   hardcoded). Provide the tier-2 PR-taxonomy check (`fix/` branch prefix, `fix:` title) and the tier-3
   keyword matcher.
3. Compute and emit TIER COMPOSITION as a reported metric: the count/share of identified fixes at each
   tier, carried alongside the fix set so downstream numbers (quality/07) can publish it. Keep tier 3
   reported separately from tiers 1–2.
4. Emit `DefectFix` records into the `defects.jsonl` schema from quality/05, each with its tier and
   three-state flag; a candidate whose linkage is unresolvable is `could-not-identify`.
5. Add `qualgen/testdata/fixid/` (planned): a fixture with one planted fix at EACH tier plus a non-fix and an
   unresolvable candidate — the known-answer inputs the Verify rows dereference.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | exit 0 |
| 2 | `cd qualgen && go test ./... -run FixLinkage && go test ./... -run GithubLabels` | exit 0; adapter-interface + reference-adapter tests pass |
| 3 | `cd qualgen && go test ./... -run TestFixID_TierPrecedence_PlantedFixtures -v` | exit 0; the planted defect-labeled-issue closer is identified at TIER 1, the `fix/`-branch/`fix:`-title PR at TIER 2, and the keyword-only commit at TIER 3 — EXACTLY those tiers on those planted fixes (a DEREFERENCE of the known-answer fixture, not a presence check) |
| 4 | `cd qualgen && go test ./... -run TestFixID_TierComposition_Reported -v` | exit 0; tier composition over `testdata/fixid/` reports the known split (1 tier-1 / 1 tier-2 / 1 tier-3), with tier 3 reported SEPARATELY, never folded into tiers 1–2 |
| 5 | `cd qualgen && go test ./... -run TestFixID_NonFixNotClassified -v` | exit 0; the planted non-fix is NOT recorded as a DefectFix; the unresolvable candidate is `could-not-identify`, never a silent non-fix |
| 6 | `cd qualgen && go test ./... -run TestDefectFix_RecordContract_ForSZZ -v` | exit 0; a `DefectFix` round-trips carrying fix identity, closed-issue ref (or none), evidence tier, and three-state flag — the exact field set quality/07's B-SZZ trace consumes |
| 7 | `cd qualgen && grep -c 'interface' qualgen/fixlinkage.go` | exit 0; count ≥ 1 (fix identification is behind a pluggable adapter interface, not a hardcoded GitHub path) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). "verified" in the stream
     README requires this section filled by someone who did NOT implement. -->

Independently verified 2026-08-30 by opus-4.8[1m]-verifier (verify-desk, non-implementer) against merged head b506390d (Merge PR #240). Offline envelope (KUBECONFIG=/dev/null). VERIFY: PASS 7/7 (6/7 as-written; row 7 rotted-table — deliverable intact). Deliverables present: qualgen/fixlinkage.go(+test), qualgen/adapters/githublabels.go(+test), qualgen/testdata/fixid/planted.json.

| # | Command | Exit | Output | Date · Runner |
|---|---------|------|--------|---------------|
| 1 | cd qualgen && go build ./... && go vet ./... | 0 | clean | 2026-08-30 · opus-4.8[1m]-verifier |
| 2 | go test -run FixLinkage; -run GithubLabels | 0 | ok qualgen; ok qualgen/adapters | 2026-08-30 · opus-4.8[1m]-verifier |
| 3 | go test -run TestFixID_TierPrecedence_PlantedFixtures -v | 0 | PASS — tier1 defect-labeled-issue-closer, tier2 fix-branch-and-title, tier3 keyword-only; exact planted tiers dereferenced | 2026-08-30 · opus-4.8[1m]-verifier |
| 4 | go test -run TestFixID_TierComposition_Reported -v | 0 | PASS — known 1/1/1 split, tier-3 separate | 2026-08-30 · opus-4.8[1m]-verifier |
| 5 | go test -run TestFixID_NonFixNotClassified -v | 0 | PASS — non-fix not recorded; unresolvable → could-not-identify | 2026-08-30 · opus-4.8[1m]-verifier |
| 6 | go test -run TestDefectFix_RecordContract_ForSZZ -v | 0 | PASS — round-trip fix identity / closed-issue ref / tier / three-state flag | 2026-08-30 · opus-4.8[1m]-verifier |
| 7 | grep -c 'interface' fixlinkage.go (re-baselined from `qualgen/fixlinkage.go` — the `cd qualgen &&` prefix double-nested the path) | 0 | 4 — LinkageAdapter + IssueLabelSource interfaces present (fixlinkage.go:75) | 2026-08-30 · opus-4.8[1m]-verifier — rotted-table, deliverable present |

RISK-VALUE: DERIVED — DefaultDefectLabels = {"bug","defect","incident"} @ qualgen/adapters/githublabels.go:50 — matches spec §5.1 tier-1 definition verbatim; a generic default merged (never replaced) with any per-target lane via NewGithubLabels(source, extra...), no internal identifier hardcoded; reversible. Other literals (closedIssuePattern, fixKeywordPattern, taxonomy prefixes) are reversible pattern config, rank below; the evidence tiers are a precedence ordering (1>2>3), not a numeric cutoff.
gate: model, all risk no, irreversible: no — desk flips implemented→verified; verified→done stays CI's on the reviewer approval. Row 7's command-path typo is a brief-authoring nit (a reviewer/author fix), not a deliverable defect.

## Review
Gate: model (all four risk answers no — repo-agnostic OSS read-only linkage analysis behind a
pluggable adapter; a generic configured issue-label source, no internal identifiers hardcoded; no
writes into any target repo). Reviewer records verdict + date in the stream README table.
