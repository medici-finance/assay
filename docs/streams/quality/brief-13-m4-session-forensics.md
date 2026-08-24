---
brief: quality/13
title: M4 session forensics — pluggable telemetry-source interface + file reference adapter
wave: 3
why: >-
  A code-only miner cannot see the scaffolding that produced a PR — only the model gets
  credited or blamed. Joining per-session harness telemetry (retries, context length,
  tool-call churn, interruptions, refusals) to the M1/M2 outcomes of the PRs those sessions
  produced tells us which HARNESS behaviors predict churn-heavy or defective work, so the
  process is tuned on evidence rather than folklore.
depends: ["quality/02", "quality/07"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by quality-stream authoring session
sources:
  - "docs/streams/quality/spec.md §7.3 — session forensics (harness telemetry × outcome)"
  - "docs/streams/quality/spec.md §7 — M4 reflexivity is a join over recorded artifacts, no new mining"
  - "docs/streams/quality/spec.md §3.2 — three-state instrument invariant"
  - "docs/streams/quality/spec.md §8 — join keys (PR number + merge SHA + stream/task ID)"
---

# Brief 13 — M4 session forensics: telemetry-source interface + file reference adapter

## Context

files:
- NEW `qualgen/m4/forensics.go` (planned) (+ `forensics_test.go`) — the session-forensics
  join: for each PR in the M1/M2 corpus, gather the harness telemetry of the session(s)
  that produced it and join it to that PR's M1 churn/rework outcome (quality/02) and M2
  defect outcome (quality/07); emit per-harness-behavior correlations — which behaviors
  predict defective or churn-heavy PRs. Measures the scaffolding, not the model.
- NEW `qualgen/telemetry/source.go` (planned) — the pluggable `TelemetrySource` interface:
  given a session/PR key, return typed telemetry records (retries, context length,
  tool-call churn, interruptions, refusals) OR a three-state could-not-measure. No concrete
  data source is named here — the interface is the seam.
- NEW `qualgen/telemetry/fileadapter.go` (planned) (+ `fileadapter_test.go`) — the
  file-based REFERENCE adapter shipped IN-TREE: reads a documented telemetry JSONL layout
  from an operator-supplied path (`--telemetry <path>`). This adapter is the OSS-ability
  proof — the interface plus one working, generic adapter, with no house-private source
  wired in.
- CONSUMES `qualgen/m1/*` (planned) churn outcomes (quality/02) and `qualgen/m2/*` (planned) traces + defect
  density (quality/07), joined by PR number + merge SHA + stream/task ID.

facts:
- **OSS boundary (load-bearing).** The CODE is OSS: the `TelemetrySource` interface AND
  the file reference adapter both ship in this stream. The telemetry DATA itself — and any
  shared/telemetry-corpus inclusion, retention window, and audit — is governed by a
  SEPARATE operator privacy ruling (a downstream, human-gated decision). This brief does
  NOT author that ruling, MUST NOT assume a specific corpus, and MUST NOT hardcode any
  concrete data source. It defines the seam the ruling later configures; the reference
  adapter reads only an operator-supplied path.
- Three-state invariant (spec §3.2): a PR whose session telemetry is absent or unreadable
  is emitted as could-not-measure, never a silent zero. Distinguishing "zero retries" from
  "no telemetry available" is the whole point of measuring the scaffolding.
- No new mining (spec §7): M4 is a join over artifacts M1/M2 already recorded, plus
  telemetry read through the interface. This brief adds no history-walking code.
- Join keys are PR number + merge SHA + stream/task ID — the keys M1/M2 artifacts already
  carry (spec §8).
- **Preconditions.** Wave 3: requires a seasoned M1/M2 corpus to correlate against
  (calendar-gated on ≥2 windows of measurement, spec §11); AND the telemetry source stays
  inert until an operator privacy ruling authorizes a data source and configures the
  adapter. The code is testable against fixtures without either precondition — the
  fixtures stand in for a live corpus/source.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `QUALITY.md`/`STATUS.md` on a branch (single writer = main's CI).
- Do NOT author the operator privacy ruling, and do NOT hardcode, name, or assume any
  concrete/house telemetry source — the reference adapter reads an operator-supplied path
  only. If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Define `TelemetrySource` in `qualgen/telemetry/source.go` (planned): a method keyed by session/PR
   returning a typed telemetry record (retries, context length, tool-call churn,
   interruptions, refusals) plus a three-state status (measured / measured-zero /
   could-not-measure). Document each field's meaning in the interface doc comment.
2. Implement the file reference adapter in `qualgen/telemetry/fileadapter.go` (planned): read the
   documented telemetry JSONL layout from `--telemetry <path>`; a missing key, missing
   file, or unreadable record yields could-not-measure for that key — never zero. Document
   the JSONL layout in a package doc comment so an operator can produce it.
3. Implement the forensics join in `qualgen/m4/forensics.go` (planned): for each PR in the M1/M2
   corpus, pull telemetry via the interface, join to that PR's churn and defect outcome,
   and emit per-behavior correlation output (e.g. retries-band × churn-rate, refusal-count
   × defect-inducing rate) with the three-state coverage reported beside every number
   (honest-claims §10 — the coverage rate ships with the correlation, never bare).
4. Wire the source as an injected dependency (constructor takes a `TelemetrySource`), so
   the forensics package compiles against the INTERFACE only — house or other adapters are
   configuration, not a fork of this code.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./telemetry/ ./m4/` | exit 0 |
| 2 | `cd qualgen && go test ./telemetry/ ./m4/` | exit 0; interface, file-adapter, and forensics-join tests pass |
| 3 (DEREFERENCE) | `cd qualgen && go test ./m4/ -run TestForensics_JoinDereferencesTelemetry -v` | exit 0 — fixture: file adapter reads a telemetry JSONL where session S has retries=7; a fixture M1 outcome marks S's PR high-churn; the emitted join row for S carries retries=7 pulled THROUGH the interface AND the churn outcome from M1 (proves the join resolved telemetry to the correct PR, not merely that a row exists) |
| 4 (three-state) | `cd qualgen && go test ./m4/ -run TestForensics_MissingTelemetryIsCouldNotMeasure -v` | exit 0 — a fixture PR with no telemetry record emits status could-not-measure for that PR, never zero |
| 5 (OSS boundary) | `cd qualgen && grep -L -e fileadapter -e 'Adapter{' m4/forensics.go && d=$(mktemp) && go list -deps ./m4/ > "$d" && grep -c 'qualgen/telemetry$' "$d"` | exit 0 — forensics.go names NO concrete adapter (depends on the interface package only); the only concrete `TelemetrySource` implementation in-tree is the file reference adapter, and it takes an operator-supplied path |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (all four risk answers no — repo-agnostic OSS Go: an interface plus a
file-based reference adapter plus a read-only join over already-recorded artifacts; no
private data source is wired in and the operator privacy ruling that governs the DATA is
explicitly out of scope). Reviewer confirms the OSS boundary holds — code shipped here,
data + corpus-privacy deferred to the operator ruling — and records verdict + date in the
stream README table.
