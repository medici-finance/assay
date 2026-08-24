---
brief: quality/03
title: M1 hotspots + knowledge distribution (SPOF) + change coupling
why: >-
  Most defects concentrate in a small number of files that change often and are hard to
  read, are known to only one author, or are silently coupled to a file a change forgets to
  touch. These are the cheapest, best-established predictors of where a codebase will break
  next. Computing them per file and per package tells the agents working the code exactly
  where to slow down, add coverage, and check the forgotten partner — before the bug ships,
  not after.
wave: 1
depends: ["quality/01"]
unblocks: ["quality/05", "quality/08", "quality/09", "quality/14"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by quality-stream authoring session
sources:
  - "docs/streams/quality/spec.md §4.3 — hotspots (decayed change-frequency x indentation-complexity proxy)"
  - "docs/streams/quality/spec.md §4.4 — knowledge distribution / bus factor / role-SPOF (ownership concentration by author-identity class AND dispatching role)"
  - "docs/streams/quality/spec.md §4.5 — change coupling (co-change above baseline) and the inverse missing-coupling-partner signal"
  - "docs/streams/quality/spec.md §4 — M1 aggregation grains (per file / package) and honest-claims discipline"
---

# Brief 03 — M1 hotspots + knowledge distribution (SPOF) + change coupling

## Context

files:
- NEW `qualgen/hotspot.go` (planned) (+ `hotspot_test.go`) (planned) — per-file hotspot score:
  decayed change-frequency x an indentation-based complexity proxy.
- NEW `qualgen/ownership.go` (planned) (+ `ownership_test.go`) (planned) — knowledge distribution:
  ownership concentration by author-identity class AND dispatching role, plus bus factor.
- NEW `qualgen/coupling.go` (planned) (+ `coupling_test.go`) (planned) — change coupling (files that
  co-change above baseline) and the inverse missing-partner signal.
- READ-ONLY, from quality/01: `qualgen/measure.go` (planned) (`Measure[T]`), `qualgen/commit.go` (planned)
  (`Commit`), `qualgen/diff.go` (planned) (`FileDiff`), `qualgen/store.go` (planned) (`Store` reader +
  append-only writer). This brief consumes that seam; it does not redefine it.

facts:
- consumes quality/01's interface contract: reads the commit + diff tables through the
  `Store`, wraps every emitted number in `Measure[T]`, and appends aggregates via the same
  single-writer `Store` to `docs/quality/metrics.jsonl`. Aggregation grains here are per
  file and per package.
- hotspot (spec §4.3): `hotspot = change_frequency(decayed) x complexity_proxy`. Change
  frequency is commit-touch count with EXPONENTIAL time decay (recent commits weigh more;
  decay half-life is a configurable parameter). Complexity proxy is INDENTATION-based
  (language-agnostic) as the base; cyclomatic complexity MAY refine it per language but is
  not required here. The PRODUCT predicts defects, not either factor alone.
- knowledge distribution (spec §4.4): per file/package, ownership concentration = share of
  SURVIVING lines per author-identity class; bus factor = the minimum identity set owning
  `> K%` of the code (K configurable). In an agent-fleet repo, "author" is BOTH the identity
  class AND the dispatching role — concentration in a single ROLE is a process SPOF even when
  line-authors vary, so ownership is computed on both axes.
- change coupling (spec §4.5): files that change together (co-commit / co-PR frequency ABOVE
  a baseline) without a static dependency. The consumer-facing signal is the INVERSE — a
  change touching file A but NOT its historical coupling partner B is the flagged case (the
  strongest cheap brittleness predictor in the coupling literature).
- three-state (spec §3.2): a file whose complexity proxy cannot be computed (binary /
  unreadable) is `could-not-measure` with a reason; a file present in the window but never
  changed is `measured-zero` change-frequency — never conflated.
- prior art (informative): Tornhill / code-maat / CodeScene for hotspot, coupling, and
  knowledge-map analyses. Cite the method, not any proprietary score.

single-point-of-failure: none — read-only aggregation over quality/01's artifacts, writing
only to the tracking root; no funds/auth/ledger surface, so the core-system
defense-in-depth obligation does not apply.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch
  + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit a generated view / `STATUS.md`-class artifact on a branch (single writer =
  main's CI). Write artifacts only under a temp/test tracking root.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `hotspot.go`: per file, compute decayed change-frequency (commit-touch count
   with exponential time decay, configurable half-life) and the indentation-based complexity
   proxy (language-agnostic), and emit their product. A file whose proxy cannot be computed
   emits `could-not-measure` with a reason; a file present but unchanged in the window emits
   `measured-zero` change-frequency.
2. Implement `ownership.go`: per file/package, compute ownership concentration as the share
   of SURVIVING lines per author-identity class AND per dispatching role, and the bus factor
   (minimum identity/role set owning `> K%`, K configurable). Surface a role-SPOF where a
   single role owns above threshold even when line-authors vary.
3. Implement `coupling.go`: from co-commit / co-PR frequency, identify file pairs coupling
   above baseline; then emit the inverse signal — given a coupled pair (A, B), a change
   touching A but not B is flagged as a missing-partner risk.
4. Append all three families to `docs/quality/metrics.jsonl` via the `Store`, each value a
   `Measure[...]`, wired into the `mine` pipeline so `qualgen mine` produces them.
5. Add fixtures: a file that is both frequently changed and deeply nested (expect top
   hotspot rank) vs files high on only one factor; a file owned `> K%` by one identity
   (expect bus factor 1) vs an evenly-shared file; a co-changing pair (expect coupled) and a
   change touching only one of them (expect missing-partner flag); a binary file (expect
   `could-not-measure` complexity).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | exit 0 |
| 2 | `cd qualgen && go test ./... -run TestHotspotIsProductNotFactor` | exit 0; a file high in BOTH decayed change-frequency and complexity outranks a file high in only one — the product drives the score, not either factor alone |
| 3 | `cd qualgen && go test ./... -run TestIndentationComplexityProxy` | exit 0; deeper indentation nesting yields a strictly higher proxy on the same line count (language-agnostic) |
| 4 | `cd qualgen && go test ./... -run TestBusFactorConcentration` | exit 0; a file owned `> K%` by one identity/role yields bus factor 1, an evenly-shared file yields a higher bus factor; a single role over threshold surfaces a role-SPOF |
| 5 | `cd qualgen && go test ./... -run TestMissingCouplingPartner` | exit 0; a co-changing pair (A,B) is flagged coupled, and a change touching A but not B raises the missing-partner signal; independent files raise neither |
| 6 | `cd qualgen && go test ./... -run TestComplexityUnmeasurableIsThreeState` | exit 0; a binary/unreadable file emits `could-not-measure` complexity (with a reason), an unchanged text file emits `measured-zero` change-frequency — never conflated |
| 7 | `TMP=$(mktemp -d); cd qualgen && go run . mine --repo .. --out "$TMP" && test -f "$TMP/docs/quality/metrics.jsonl" && grep -q '"hotspot"' "$TMP/docs/quality/metrics.jsonl" && TOP=$(grep '"hotspot"' "$TMP/docs/quality/metrics.jsonl" \| head -1) && echo "$TOP" \| grep -q '"path"'` | exit 0; the mine pipeline emits per-file hotspot rows carrying a real `path` — DEREFERENCES an emitted hotspot record against the artifact rather than only counting rows |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). "verified" in the stream
     README requires this section filled by someone who did NOT implement. -->

## Review
Gate: model (all four risk answers no — read-only aggregation over quality/01's artifacts,
writing only to the tracking root; no funds/auth/ledger surface). Reviewer records verdict +
date in the stream README table.
