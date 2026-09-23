---
brief: assay:assay:statusgen:09
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
decision-issue: 217
schema: brief-v2
authored: 2026-08-26 (re-authored clean for the statusgen board)
sources:
  - "A free-tier feedback-flywheel candidate: feature ideas arriving as GitHub issues are visible to every competitor, but an opt-in anonymized ping accumulates a proprietary corpus of how agent fleets actually drift"
  - "freshness note (2026-07-17): statusgen has no telemetry, no network calls at all"
why: >
  The data-flywheel version of the feedback moat: feature ideas arriving as GitHub issues are
  visible to every competitor, but an opt-in, anonymized telemetry ping (lint-failure categories,
  lifecycle-transition stats) accumulates a proprietary corpus of how agent fleets actually drift —
  feeding future features and, eventually, risk-gate defaults.
version: 1
id: ff2ec63d-f6f6-4cfa-a37e-c5f7aaba1caa
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
### Verify pass 2026-09-22 (non-implementer, VERIFY: PASS — Evidence-only, gate:human customer+sensitive; held at implemented, routes to human gate #217)

Runner: `assay-verifier-app[bot] / claude-opus-4.8[1m]` (non-implementer). Merged main `f48d4ed17d80223d3f4ec875acd1fd05784f3f2a`. Offline (`KUBECONFIG=/dev/null`; `go test`/`go run` direct, no network). Implementing commit c7e1715ac confirmed ancestor of HEAD.

| # | Command | Expect | Observed (exit → key output) | Date | Runner |
|---|---------|--------|------------------------------|------|--------|
| 1 | `cd statusgen && go test ./...` | exit 0 incl payload-leak test | exit 0 — `ok .../statusgen 32.124s`; PASS: TestTelemetryEndpointEmptyByDefault, TestTelemetryDefaultOff (6 subcases), TestClassifyLintProblemNeverEchoes, TestNormalizeStatusFixedVocabulary, TestBuildPayloadNoLeak_InMemory, TestCollectTelemetryPayload_FixtureTreeNoLeak | 2026-09-22 | opus-4.8-verifier |
| 2 | `cd statusgen && go run . --root .. --lint` | no telemetry output (default off) | exit 0, LINT: PASS; no telemetry feature output (grep `^telemetry:`/banner → RC 1) | 2026-09-22 | opus-4.8-verifier |
| 3 | `ASSAY_TELEMETRY=1 go run . --root .. --telemetry --telemetry-dry-run` | prints payload; no path/title strings | exit 0 — counts-only JSON (schema telemetry-v1, stream_count 22, brief_count 260, status counts by fixed vocab, empty lint/lifecycle), then `telemetry: dry-run — nothing sent.`; path/title grep over payload → RC 1 (none) | 2026-09-22 | opus-4.8-verifier |
| 4 | `grep -i "off by default" docs/telemetry.md` | match | exit 0 — 3 matches | 2026-09-22 | opus-4.8-verifier |

Scope traceability: every Evidence row maps 1:1 to its Verify row. Deliverables present: docs/telemetry.md (fields table + mechanics + retention + promise), README index row, docs/distribution.md default-off (:14-16).

RISK-VALUE: DERIVED — `telemetryEndpoint = ""` @ `statusgen/telemetry.go:60` (guard :245) — empty is the ONLY correct value; `sendTelemetry` returns `errTelemetryNoEndpoint` BEFORE constructing any http request, so even an armed run never dials — transmission is STRUCTURALLY impossible, not merely off. A `var` only to permit deliberate `-ldflags -X` stamping later; no runtime flag/env sets it. Proven by TestTelemetryEndpointEmptyByDefault. A committed URL would be an irreversible leak — the top control.
RISK-VALUE: DERIVED — double opt-in `flagSet && os.Getenv("ASSAY_TELEMETRY") == "1"` @ `statusgen/telemetry.go:65` (both flags default false @ main.go:1633-1634) — boolean AND of two independent switches with EXACT "1" match (rejects "true"/"0"/unset); inaction = off; closes the "CI vendor/inherited-env flips it silently" hole. Proven by TestTelemetryDefaultOff.
RISK-VALUE: DERIVED — `TelemetryPayload` has NO free-text field; all 7 fields are string(tool constants)/int/map[string]int keyed by fixed-vocabulary @ `statusgen/telemetry.go:159-167` — the type itself is the anonymization guarantee (no []string/free-text; map keys only from classify/normalize which return in-file constants, never input substrings). Proven by TestBuildPayloadNoLeak_InMemory + TestCollectTelemetryPayload_FixtureTreeNoLeak (planted sentinels never survive into JSON; counts still non-vacuous).

**VERIFY: PASS** on all 4 Verify rows against merged main. gate:human + risk {customer:yes, sensitive-data:yes} → a model records Evidence and does NOT sign off. Status LEFT at `implemented`. These three DERIVED risk-values are the independent integrity base for the human signer (this is the Task-1 home of gtm/08's telemetry client). Routes to the human gate — decision issue #217: human:Ian (own login, not a bot relay) records verdict+date with a `human:<name>` token in the statusgen README, closing #217.

## Review
Gate: human — a human signs the field list, opt-in wording, retention, endpoint. Verdict + date in
the stream README (a `human:<name>` token; a bare model sign-off does not close this brief).
