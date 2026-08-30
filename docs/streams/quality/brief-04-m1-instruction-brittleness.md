---
brief: quality/04
title: M1 instruction-layer brittleness — reference-validity + doc↔code co-change staleness
why: >-
  Agents are only as good as the instruction layer they run on. Config/instruction files,
  task briefs, specs, and skills quietly rot: they reference file paths, symbols, and typed
  IDs that no longer exist, and normative docs drift from the code they describe. That rot
  is invisible to delivery metrics and to source-only quality miners, yet it is a leading
  cause of an agent acting on stale instructions. This brief measures it — a decaying
  reference-validity trend plus doc↔code co-change staleness — so context rot becomes a
  number that trends, not a surprise.
wave: 1
depends: ["quality/01"]
unblocks: ["quality/05", "quality/09", "quality/14"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by quality-stream authoring session
sources:
  - "docs/streams/quality/spec.md §4.6 — instruction-layer brittleness (context rot): reference-validity + doc↔code co-change"
  - "docs/streams/quality/spec.md §4.5 — change coupling, applied doc-to-code"
  - "docs/streams/quality/spec.md §3.2 — three-state instrument invariant"
---

# Brief 04 — M1 instruction-layer brittleness (reference-validity + doc↔code drift)

## Context

files:
- NEW `qualgen/instructionbrittle.go` (planned) (+ `qualgen/instructionbrittle_test.go` (planned)) — the
  instruction-layer brittleness pass: over a configured set of instruction-bearing docs it (a)
  extracts and resolves references (file paths, code symbols, typed IDs) and scores
  reference-validity, trended over history, and (b) computes doc↔code co-change staleness by
  applying the §4.5 coupling analysis in the doc→code direction.
- NEW `qualgen/driftdetect.go` (planned) (+ `qualgen/driftdetect_test.go` (planned)) — a GENERIC
  source↔render drift-detection capability: given a `source` artifact and the `rendered`/target
  artifact(s) it is supposed to track, it reports whether they have drifted at a given commit.
  This is the tool-wide home of the point-in-time drift check; the trended reference-validity
  layer is its measured, historical generalization and MUST consume this capability rather than
  grow a second copy of the same logic. It is exported for other passes (e.g. the `check` mode,
  quality/09) to reuse.
- NEW `qualgen/testdata/instrbrittle/` (planned) — fixture repos with PLANTED dead references and a
  PLANTED doc-drifts-from-code history, so the pass has known-answer inputs.
- Reads the miner skeleton's history/blob access and three-state plumbing from `qualgen/` (quality/01);
  emits aggregates through the same M1 aggregation path the other wave-1 briefs feed. This brief adds
  the instruction-layer signals to that stream; it does not define the artifact schema or the rendered
  view (that is quality/05).

facts:
- The instruction-doc SET is CONFIG, not hardcoded: a per-target glob list of instruction-bearing
  paths (config/instruction files, briefs, specs, skills). An empty/unconfigured set yields
  `could-not-measure`, never a silent zero (§3.2).
- Reference kinds detected: dead FILE PATH (path referenced in a doc that resolves to nothing at that
  commit), dead SYMBOL (a named code symbol no longer present), dead TYPED ID (a structured ID token,
  matched by a configured pattern, with no live referent). A reference that cannot be classified
  resolvable/unresolvable is reported `could-not-measure`, distinct from measured-dead.
- Reference-validity is TRENDED: computed per time window over history so a decaying validity curve is
  the rot signal, not a single snapshot. Doc↔code co-change staleness reuses §4.5 coupling: a doc
  whose described code changes repeatedly while the doc does not is presumptively stale.
- The generic drift detector (`driftdetect.go`) is deliberately NOT tied to any one internal document
  pipeline — it takes a source and its render target(s) as inputs, so any source↔render pair (a doc and
  the code it describes, a template and its output) is checkable through the one capability.
- Out of scope: the rendered `QUALITY.md` view and JSONL schema (quality/05); source semantics /
  linting (spec §2); building a second, parallel drift checker.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch + draft PR
  only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `QUALITY.md` or any generated artifact on a branch (single writer = main's CI, quality/05).
- Three-state everywhere (§3.2): measured / measured-zero / could-not-measure; never a silent zero.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `qualgen/driftdetect.go` (planned): a generic, exported source↔render drift-detection capability.
   Input is a source artifact and the render/target artifact(s) it should track, resolved at a given
   commit; output is a three-state drift verdict (in-sync / drifted / could-not-check) with the
   evidence (which referents resolved, which did not). Keep it free of any single-pipeline assumption
   so the instruction-brittleness pass and later `check` mode both consume it.
2. Implement `qualgen/instructionbrittle.go` (planned) reference-validity: over the configured instruction-doc
   glob set, extract references (file paths, symbols, typed IDs), resolve each at the commit via the
   drift capability from step 1, and score reference-validity. Compute it per history window so the
   output is a TREND, not a snapshot. Unresolvable-to-classify references are `could-not-measure`.
3. Implement doc↔code co-change staleness: apply the §4.5 coupling analysis in the doc→code direction —
   a doc whose described/coupled code changes repeatedly without the doc changing is flagged
   presumptively stale, with the co-change counts as evidence.
4. Emit both signals as M1 aggregates through the skeleton's aggregation path (per file/window),
   carrying the three-state flag on every value. Do NOT render a view or define JSONL schema here.
5. Add `qualgen/testdata/instrbrittle/` (planned) fixtures: at least one repo with a KNOWN number of planted dead
   references (a mix of dead path, dead symbol, dead typed ID, plus one live reference and one
   could-not-classify), and one repo with a planted doc-drifts-from-code history (code changes N times,
   doc unchanged). These are the known-answer inputs the Verify rows dereference.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | exit 0 |
| 2 | `cd qualgen && go test ./... -run Drift && go test ./... -run InstructionBrittle` | exit 0; drift-detector + instruction-brittleness tests pass |
| 3 | `cd qualgen && go test ./... -run TestReferenceValidity_PlantedDeadRefs -v` | exit 0; the pass reports EXACTLY the planted dead-reference count from `testdata/instrbrittle/` (dead path + dead symbol + dead typed ID), the one live reference as valid, and the unclassifiable one as `could-not-measure` — a DEREFERENCE of the known-answer fixture, not a presence check |
| 4 | `cd qualgen && go test ./... -run TestDocCodeStaleness_PlantedDrift -v` | exit 0; the doc↔code pass flags the planted stale doc (code changed N times, doc unchanged) as presumptively stale with co-change count = N |
| 5 | `cd qualgen && go test ./... -run TestReferenceValidity_TrendedNotSnapshot -v` | exit 0; reference-validity is emitted per window (≥2 windows over the fixture history), proving it is trended rather than a single snapshot |
| 6 | `cd qualgen && grep -c 'driftdetect' qualgen/instructionbrittle.go` | exit 0; count ≥ 1 (the brittleness pass consumes the shared drift capability, not a second copy) |
| 7 | `cd qualgen && go test ./... -run TestInstructionSet_Unconfigured_ThreeState -v` | exit 0; an empty/unconfigured instruction-doc set yields `could-not-measure`, never measured-zero |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). "verified" in the stream
     README requires this section filled by someone who did NOT implement. -->

Independently verified 2026-08-30 by opus-4.8[1m]-verifier (verify-desk, non-implementer) against merged head b506390d (Merge PR #240). Offline envelope (KUBECONFIG=/dev/null). VERIFY: PASS 7/7 (6/7 as-written; row 6 rotted-table — deliverable intact). Deliverables present: qualgen/driftdetect.go(+test), qualgen/instructionbrittle.go(+test), qualgen/testdata/instrbrittle/{deadrefs,staledoc}/.

| # | Command | Exit | Output | Date · Runner |
|---|---------|------|--------|---------------|
| 1 | cd qualgen && go build ./... && go vet ./... | 0 | clean | 2026-08-30 · opus-4.8[1m]-verifier |
| 2 | go test -run Drift; -run InstructionBrittle | 0 | ok qualgen (both suites) | 2026-08-30 · opus-4.8[1m]-verifier |
| 3 | go test -run TestReferenceValidity_PlantedDeadRefs -v | 0 | PASS — Live=1, Dead=3, CouldNotMeasure=1; by-kind exactly 1 each (dead-path/dead-symbol/dead-typed-ID/live-path/unclassifiable) over 5 planted refs; dereference not presence | 2026-08-30 · opus-4.8[1m]-verifier |
| 4 | go test -run TestDocCodeStaleness_PlantedDrift -v | 0 | PASS — CodeOnlyChanges=5 (planted N), Stale=true, DocPath=doc.md, EstablishedAt SHA match | 2026-08-30 · opus-4.8[1m]-verifier |
| 5 | go test -run TestReferenceValidity_TrendedNotSnapshot -v | 0 | PASS — Trend>=2 windows, distinct AtSHA (first!=last) | 2026-08-30 · opus-4.8[1m]-verifier |
| 6 | grep -c 'driftdetect' instructionbrittle.go (re-baselined from `qualgen/instructionbrittle.go` — `cd qualgen &&` prefix double-nested the path) | 0 | 8 — consumes the shared driftdetect capability (no second copy) | 2026-08-30 · opus-4.8[1m]-verifier — rotted-table, deliverable present |
| 7 | go test -run TestInstructionSet_Unconfigured_ThreeState -v | 0 | PASS — unconfigured → could-not-measure+reason, Trend nil, Staleness nil; never measured-zero | 2026-08-30 · opus-4.8[1m]-verifier |

RISK-VALUE: NAMED, NOT DERIVED (out-of-scope-by-reversibility) — defaultStaleCoChangeThreshold = 2 @ qualgen/instructionbrittle.go:37 — min code-only change count before a doc is presumptively stale. NOT a risk-bearing gate: the brief is irreversible:no, all-risk-no, on a read-only OSS measurement path (not a risk-classed path, no CLAUDE.md-pinned value), so the fail-safe risk-value trigger does not fire. It is a reversible measurement tuning DEFAULT (a wrong value only mis-flags a metric, undone by edit+redeploy) and is overridable per-run via InstructionBrittleConfig.StaleCoChangeThreshold — derivation from first principles is neither affordable nor meaningful for a measurement default. Recorded for the reviewer's judgment. (defaultWindowCount=4, defaultTypedIDPattern rank below; all reversible config.)
gate: model, all risk no, irreversible: no — desk flips implemented→verified; verified→done stays CI's on the reviewer approval. Row 6's command-path typo is a brief-authoring nit, not a deliverable defect.

## Review
Gate: model (all four risk answers no — repo-agnostic OSS read-only history analysis over a
configured doc set; no writes to the target repo, no shared product surface). Reviewer records
verdict + date in the stream README table.
