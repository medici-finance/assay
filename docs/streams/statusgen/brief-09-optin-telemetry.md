---
brief: statusgen/09
title: Opt-in statusgen telemetry — anonymized fleet-drift corpus (off by default)
wave: 1
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: yes}
gate-why: >
  Telemetry from third-party repos is a data-collection boundary: even anonymized category
  counts leave a user's machine, and the free tier's trust posture ("nothing leaves your
  repo") changes the moment any ping exists. A human signs the collected-field list, the
  off-by-default/opt-in wording, retention, and the endpoint before any release carries it.
issues: []
schema: brief-v1
authored: 2026-08-26 (re-authored clean for the statusgen board)
sources:
  - "A free-tier feedback-flywheel candidate: feature ideas arriving as GitHub issues are visible to every competitor, but an opt-in anonymized ping accumulates a proprietary corpus of how agent fleets actually drift"
  - "freshness note (2026-07-17): statusgen has no telemetry, no network calls at all"
why: >
  The data-flywheel version of the feedback moat: feature ideas arriving as GitHub issues are
  visible to every competitor, but an opt-in, anonymized telemetry ping (lint-failure categories,
  lifecycle-transition stats) accumulates a proprietary corpus of how agent fleets actually drift —
  feeding future features and, eventually, risk-gate defaults.
---

# Brief 09 — Opt-in statusgen telemetry — anonymized fleet-drift corpus

## Context
files: `statusgen/` (telemetry package + flag wiring + tests), `docs/telemetry.md` (new — collected
  fields, opt-in mechanics, retention, the promise), README.md (index row)
facts:
  - OFF by default, single explicit opt-in (`--telemetry` flag AND `ASSAY_TELEMETRY=1` env — both required, so no CI vendor default can flip it silently); every run with telemetry on prints what was sent
  - payload: category counts ONLY (lint-failure categories, lifecycle-transition tallies, stream/brief COUNTS, statusgen version) — never repo names, brief titles, file paths, register text, identities; payload schema versioned in docs/telemetry.md
  - no endpoint exists yet: this brief implements the client behind a compile-time-default-empty endpoint + a --telemetry-dry-run that prints the payload; standing up the receiver is out of scope (note as v-next in docs/telemetry.md)
  - statusgen dependency rule holds: stdlib + yaml.v3 only (net/http is stdlib)
consumers: docs/distribution.md (install story must mention the default-off posture — one line, updated here)

## Ground rules
- NEVER git push / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement the telemetry client per facts (dry-run mode, double-opt-in, printed payload),
   with tests asserting the payload NEVER contains strings from brief titles/paths in a
   fixture tree.
2. Write docs/telemetry.md (fields, mechanics, retention, promise) + README row + the one
   distribution.md line.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go test ./...` | exit 0 (incl. payload-leak test) |
| 2 | `cd statusgen && go run . --root .. --lint` | no telemetry output (default off) |
| 3 | `cd statusgen && ASSAY_TELEMETRY=1 go run . --root .. --telemetry --telemetry-dry-run` | prints payload; payload contains no path/title strings |
| 4 | `grep -i "off by default" docs/telemetry.md` | match |

## Evidence
<!-- filled at implementation time by a non-implementer -->

## Review
Gate: human — a human signs the field list, opt-in wording, retention, endpoint. Verdict + date in
the stream README (a `human:<name>` token; a bare model sign-off does not close this brief).
