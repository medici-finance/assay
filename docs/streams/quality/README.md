---
stream: quality
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: []
---

# quality Stream

A repo-agnostic git-history mining tool — working name `qualgen`, a statusgen-sibling
Go binary — that answers three questions with numbers the flow board cannot:
**are we getting better** (durable change vs bug-churn over time), **where is the
codebase brittle** (hotspots, defect density, knowledge concentration, hidden
coupling), and **how do we compare** (the industry's GitClear-published metrics and
SZZ-derived defect lineage, computed the published way so the numbers are
industry-comparable — and, unlike the proprietary tools, auditable).

The tool targets **any git repository, not only Assay-managed ones**, and mines
**full history, not only the from-now-on window**. Both are load-bearing: the
"are we getting better" question needs a pre-adoption baseline from the same
codebase's own history, and the comparability claim is only testable on codebases
that never adopted the methodology. See [spec.md](spec.md) for the layer model (M1
comparability / M2 defect lineage / M3 stage attribution / M4 reflexivity), the
target-profile capability ladder, and the honest-claims discipline.

**Everything in this stream is repo-agnostic and OSS.** House-specific integration is
supplied as *configuration* against the pluggable interfaces each layer defines
(fix-linkage, provenance-linkage, telemetry-source, issue-filer, delivery-metrics) —
the reference adapter for each is shipped here; the house wiring and the actual
numbers live outside this stream. The one genuinely-private artifact is the
shared-telemetry-corpus privacy ruling that gates §7.3, which is a separate downstream
decision, not code in this stream.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [miner skeleton — go-git extraction, incremental runs, three-state plumbing](brief-01-miner-skeleton.md) | 0 | L | verified | 2026-08-26 opus-4.8[1m]-verifier | — |
| 02 | [M1 line-operation taxonomy + churn/rework rate](brief-02-m1-taxonomy-churn.md) | 1 | M | todo | — | — |
| 03 | [M1 hotspots + knowledge distribution (SPOF) + change coupling](brief-03-m1-hotspots-coupling.md) | 1 | M | implemented | — | — |
| 04 | [M1 instruction-layer brittleness (reference-validity + doc↔code drift)](brief-04-m1-instruction-brittleness.md) | 1 | M | implemented | — | — |
| 05 | [`QUALITY.md` single-writer trend view + metrics artifacts](brief-05-quality-view-artifacts.md) | 2 | M | todo | — | — |
| 06 | [M2 fix identification — pluggable linkage adapter + evidence tiers](brief-06-m2-fix-identification.md) | 1 | M | implemented | — | — |
| 07 | [M2 B-SZZ inducing trace + derived defect metrics](brief-07-m2-szz-trace-metrics.md) | 2 | L | todo | — | — |
| 08 | [`pr <n>` mode — per-file risk features (generic riskscore feed)](brief-08-pr-riskscore-features.md) | 3 | M | todo | — | — |
| 09 | [`check <files>` mode — brittleness screen for a named file set](brief-09-check-brittleness-screen.md) | 2 | M | todo | — | — |
| 10 | [M3 stage attribution — dossier + ledger, pluggable provenance-linkage adapter](brief-10-m3-stage-attribution.md) | 3 | L | todo | — | — |
| 11 | [DORA join — quality denominator + traced-CFR, pluggable delivery-metrics source](brief-11-dora-join.md) | 3 | M | todo | — | — |
| 12 | [M4 gate-yield accounting + ritual-effectiveness joins](brief-12-m4-gate-yield-rituals.md) | 4 | M | todo | — | — |
| 13 | [M4 session forensics — pluggable telemetry-source interface + reference adapters](brief-13-m4-session-forensics.md) | 3 | M | todo | — | — |
| 14 | [auto-filed refactor work + quality error-budgets + RETRO output feed](brief-14-autofile-budgets-retro.md) | 5 | M | todo | — | — |
| 15 | [learned riskscore graduation — JIT defect-prediction model](brief-15-learned-riskscore.md) | 3 | M | todo | — | — |

Brief 01 implemented on branch `brief/quality-01-miner-skeleton` (new `qualgen/` module: go-git extraction, incremental extend-never-replace mine, three-state `Measure[T]` plumbing, append-only artifact store; `mine` mode live, `report`/`pr`/`check` scaffolded). Draft-PR link to be attached when the PR is opened.

Brief 04 implemented on branch `feat/quality-04` (new `qualgen/driftdetect.go`: a generic source↔render drift-detection capability shared with later passes; new `qualgen/instructionbrittle.go`: trended reference-validity — dead file-path/symbol/typed-ID detection over a configured instruction-doc glob set — plus doc↔code co-change staleness applying the §4.5 coupling analysis in the doc→code direction; planted-fixture tests under `qualgen/testdata/instrbrittle/`). Draft-PR link to be attached when the PR is opened.

## Critical path

`quality/01` (miner skeleton) → `quality/06` (M2 fix identification) → `quality/07`
(B-SZZ trace + defect corpus) → `quality/10` (M3 stage attribution) → `quality/12`
(gate-yield + ritual-effectiveness) → `quality/14` (closing the loop).

The **real blocker at the head is `01`**, and it is verified to have no hidden upstream
dependency: `qualgen` is a statusgen-sibling with its own read-only go-git usage, so it
does **not** block on the `desktools-go-git` gitcore migration (that is desk-*write*
plumbing in a different module) — the two may share read patterns opportunistically,
but `01` starts clean. The **pacing item is not code, it is corpus maturity**: the M4
and learned-model briefs (12, 13, 14, 15) each consume an M1–M3 corpus that must exist
and season first (the spec's §11 sequencing), so they are calendar-gated on ≥ 2 windows
of measurement even once their code is written. M2 tier-1 fix-linkage lights up further when the house verdict lane
goes live, but is never a blocker — tiers 2–3 and the GitHub-issues reference adapter
work from day one.

## Dependency waves

```
Wave 0: [01]
Wave 1: [02, 03, 04, 06]      ← 01
Wave 2: [05, 07, 09]          ← 02,03,04 (05, 09); 06 (07)
Wave 3: [08, 10, 11, 13, 15]  ← 02,03,07 (08); 07 (10, 11, 13, 15)
Wave 4: [12]                  ← 10 (review-escape overlay) seasoned
Wave 5: [14]                  ← 12 (gate-yield) + 03,04,07 corpus
```

One-line critical path: `01 → 06 → 07 → 10 → 12 → 14`.

Waves 0–2 are pure history mining — useful standalone even if later waves change shape
in review. The M4 and learned-model briefs (12 in wave 4, 14 in wave 5, and 13/15 in
wave 3) are corpus-gated: each consumes an M1–M3 corpus that must season first.

## Shared conventions

- **Three-state invariant everywhere** (spec §3.2): every check and metric distinguishes
  *measured*, *measured-zero*, and *could-not-measure*. Never a silent zero.
- **Committed-artifact model, single-writer views**: runs emit/update diffable JSONL;
  the rendered `QUALITY.md` view is CI-written only (local runs read-and-discard), in
  the STATUS.md discipline.
- **Pluggable interfaces, reference adapters in-tree**: every house-shaped seam
  (fix-linkage, provenance, telemetry, issue-filer, delivery-metrics) is an interface
  with a generic reference adapter shipped here; house wiring is config, not a fork.
- **Honest-claims discipline** (spec §10): "computed per GitClear's *published*
  definitions" never "GitClear-equivalent"; every SZZ number ships its trace-rate and
  evidence-tier composition; stage attribution is "evidence-assembled,
  judgment-classified, spot-audited," never "measured."
