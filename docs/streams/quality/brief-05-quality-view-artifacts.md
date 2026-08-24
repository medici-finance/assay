---
brief: quality/05
title: single-writer QUALITY.md trend view + metrics/defects/attribution artifact schemas
why: >-
  The miner's numbers are only useful if they are diffable, honest, and readable at a glance. This
  brief defines the committed-artifact contract every other layer writes into — append-only JSONL for
  M1 metrics, M2 defect traces, and M3 dossiers — and the single-writer rendered QUALITY.md trend view
  that CI alone writes. The single-writer discipline is what keeps the view trustworthy: a local run
  reads and discards, so the committed view is never a developer's laptop state. Every number carries
  its three-state flag and its industry-comparable baseline beside the local number, so the artifact
  cannot quietly overclaim.
wave: 2
depends: ["quality/02", "quality/03", "quality/04"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by quality-stream authoring session
sources:
  - "docs/streams/quality/spec.md §9.3 — trend view: single-writer QUALITY.md (CI only writer, local runs read-and-discard)"
  - "docs/streams/quality/spec.md §9.4 — artifacts: metrics.jsonl / defects.jsonl / attribution/ (append-only)"
  - "docs/streams/quality/spec.md §10 — honest-claims discipline (industry-comparable number beside local number)"
  - "docs/streams/quality/spec.md §3.2 — three-state instrument invariant"
---

# Brief 05 — QUALITY.md single-writer trend view + metrics artifacts

## Context

files:
- NEW `qualgen/artifacts.go` (planned) (+ `qualgen/artifacts_test.go` (planned)) — the append-only JSONL artifact
  schema + reader/writer for the three artifact families, all relative to the operator-chosen tracking
  root (`--out <dir>`, spec §3.1): `docs/quality/metrics.jsonl` (M1 aggregates), `docs/quality/defects.jsonl`
  (M2 traces — schema declared here for quality/06–07 to fill), `docs/quality/attribution/` (M3 per-defect
  dossiers, one file per defect — schema declared here for quality/10 to fill).
- NEW `qualgen/report.go` (planned) (+ `qualgen/report_test.go` (planned)) — the `report` mode renderer that reads
  the JSONL artifacts and renders the `QUALITY.md` trend view. SINGLE-WRITER: CI is the only writer; a
  local `report` run READS the artifacts and renders to stdout / a temp path and DISCARDS (never writes
  the committed `QUALITY.md`), in the STATUS.md discipline. A `--write` path exists but is guarded to the
  CI context only.
- NEW `qualgen/testdata/report/` (planned) — a fixture artifact set (known metric values) so the render
  can be dereferenced against specific expected rendered numbers.
- Reads M1 aggregates produced by quality/02 (taxonomy + churn), quality/03 (hotspots + coupling +
  SPOF), and quality/04 (instruction brittleness). This brief renders and schematizes them; it does not
  compute any metric.

facts:
- SINGLE-WRITER rule (§9.3): the committed `QUALITY.md` is written only by CI on main. Local runs
  read-and-discard — the tool MUST refuse to overwrite the committed view outside the CI-writer path.
  This mirrors the STATUS.md discipline exactly.
- Artifacts are APPEND-ONLY (§9.4): `metrics.jsonl` and `defects.jsonl` append records; `attribution/`
  is one file per defect with tombstone amendments (never silent edits). A run EXTENDS, never rewrites,
  the baseline.
- Every emitted number carries a THREE-STATE field (§3.2): `measured` / `measured-zero` /
  `could-not-measure`. A rendered row for a could-not-measure metric shows the unmeasured state, never a
  zero.
- INDUSTRY-COMPARABLE beside LOCAL (§9.3, §10): where a metric has a published industry baseline (e.g.
  churn rate, copy/paste ratio, duplicate-block rate), the view renders the industry number beside the
  local number. Honest-claims wording (§10): "computed per GitClear's PUBLISHED definitions," never
  "GitClear-equivalent"; where windows/thresholds differ the artifact states both.
- The tracking root is `--out <dir>` (profile-B rule, §3.1) — artifacts NEVER land inside a foreign
  target repo. The single-writer discipline applies to the tracking root.
- Out of scope: computing any metric (quality/02–04); the M2/M3 record BODIES (quality/06–07, /10 fill
  the schemas declared here).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch + draft PR
  only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit a generated `QUALITY.md` (or any artifact) on a branch — single writer = main's CI. The
  local render path must PROVE it discards.
- Three-state everywhere (§3.2); never a silent zero.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `qualgen/artifacts.go` (planned): define and implement the append-only JSONL schema for the three
   families under the tracking root — `metrics.jsonl`, `defects.jsonl`, `attribution/`. Each record
   carries its three-state field; each number that has an industry baseline carries both the local value
   and the industry-comparable value (plus the window/threshold caveat when they differ). Writer appends
   (never rewrites); reader is order-stable. Declare the `defects.jsonl` and `attribution/` record shapes
   even though this brief does not populate them — quality/06–07 and /10 fill them.
2. Implement `qualgen/report.go` (planned) `report` mode: read the JSONL artifacts and render `QUALITY.md` — churn
   trend, copy/paste ratio, duplicate-block rate, defect-inducing rate (placeholder until M2), per-stage
   ledger (placeholder until M3), top-10 hotspots, bus-factor alarms, instruction reference-validity
   trend — each with its industry-comparable number beside the local number where one exists, and each
   honoring the three-state flag.
3. Enforce SINGLE-WRITER: a local `report` run renders to stdout / a temp path and DISCARDS; the
   committed-view write is reachable ONLY through the CI-writer guard. Attempting to write the committed
   `QUALITY.md` outside that guard is refused. Add a test proving the local path leaves the committed
   view untouched.
4. Add `qualgen/testdata/report/` (planned) with a fixture artifact set whose metric values are KNOWN, so the
   render can be checked against specific expected rendered strings.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | exit 0 |
| 2 | `cd qualgen && go test ./... -run Artifacts && go test ./... -run Report` | exit 0; schema + render tests pass |
| 3 | `cd qualgen && go test ./... -run TestReport_RendersKnownMetric -v` | exit 0; the render of `testdata/report/` produces a `QUALITY.md` string containing the SPECIFIC expected value from the fixture (e.g. the known copy/paste ratio `0.42` rendered beside its industry baseline) — a DEREFERENCE of a real rendered number, not a presence check |
| 4 | `cd qualgen && go test ./... -run TestReport_IndustryBesideLocal -v` | exit 0; a metric with a published baseline renders BOTH the local number and the industry-comparable number on the same row; the header text reads "per GitClear's published definitions" (not "GitClear-equivalent") |
| 5 | `cd qualgen && go test ./... -run TestReport_LocalRunDiscards -v` | exit 0; a local (non-CI) `report` run does NOT write the committed `QUALITY.md` — the file on disk is byte-identical before and after; the CI-writer guard is the only path that writes |
| 6 | `cd qualgen && go test ./... -run TestArtifacts_AppendOnly -v` | exit 0; a second `mine`/emit run APPENDS to `metrics.jsonl` (record count strictly increases, prior records unchanged), never rewrites the baseline |
| 7 | `cd qualgen && go test ./... -run TestArtifacts_ThreeStateField -v` | exit 0; a could-not-measure metric serializes with the unmeasured state and renders as unmeasured (never `0`) |
| 8 | `cd qualgen && go test ./... -run TestArtifacts_SchemaDeclaresDefectsAndAttribution -v` | exit 0; the `defects.jsonl` record (with evidence tier + confidence fields) and the `attribution/` per-defect file shape are declared and round-trip, ready for quality/06–07 and /10 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). "verified" in the stream
     README requires this section filled by someone who did NOT implement. -->

## Review
Gate: model (all four risk answers no — repo-agnostic OSS artifact schema + a rendered view;
single-writer discipline is enforced in code and proven by a discard test; no product surface, no
writes into any target repo). Reviewer records verdict + date in the stream README table.
