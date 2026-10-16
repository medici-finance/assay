---
brief: assay:assay:performance-collection:08
title: "Daily report generation and report-pack qualification"
why: "A scheduled run and a manual invocation must produce the same charts from the same saved snapshot, with missing collection visible."
wave: 3
depends: ["performance-collection/07"]
unblocks: ["performance-collection/09"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Report collection, exposure and recurring effect boundaries need acceptance of the concrete implementation; no live permission is granted by this draft."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: bcf69d42-62dd-4b7f-a609-af76d88ee19c
authored: "2026-10-15 by reporting authoring session"
sources:
  - docs/streams/performance-collection/README.md
  - docs/streams/continuing-operations/spec.md
  - docs/proposals/non-ci-architecture/23-runtime-contract-alignment.md
  - "freshness-checked 2026-10-15 at public main d430e8f18"
exec-tier: strong
exec-tier-why: "Cross-source identity, coverage, authority and migration must agree."
domain: complicated
outcome: none
consumers:
  - "perfdata/reportpack/: follow-up performance-collection/08"
  - "perfdata/cmd/perfdata/: follow-up performance-collection/08"
  - "perfdata/testdata/reportpack/: follow-up performance-collection/08"
  - "components/perfdata/: follow-up performance-collection/08"
  - "ci/staged-workflows/perfdata-reports.yml: follow-up performance-collection/08"
  - "docs/report-packs.md: follow-up performance-collection/08"
  - "docs/performance-collection.md: follow-up performance-collection/08"
  - "changelog/daily-operating-report.md: follow-up performance-collection/08"
---

# Brief 08 — Daily report generation and report-pack qualification

## Context

files: `perfdata/reportpack/` (planned), `perfdata/cmd/perfdata/` (planned), `perfdata/testdata/reportpack/` (planned), `components/perfdata/` (planned), `ci/staged-workflows/perfdata-reports.yml` (planned), `docs/report-packs.md`, `docs/performance-collection.md` (planned), `changelog/daily-operating-report.md` (planned)

facts:
- This is authored todo scope, not deployed capability. Collection briefs 01/02 produce the bounded source contract; no database or remote endpoint is needed by local reports.
- Existing work/acceptance, runner usage and operational audit retain their owners. `statusgen/flow.go` is package main, not an importable library; consume its versioned saved export, not an invented cross-module import.
- perf-v1 has seven-day raw retention, own-process CPU and partial forge/child counters. Preserve source limits; absent history stays unknown.
- Source pin is in frontmatter; refresh actual source, claims and prerequisites at pickup.

layering: pure validation/aggregation and typed snapshot consumers; adapters own I/O and effects. Future cell coordination and effect execution remain with R2/R3/R4; optional reporting cannot grant authority.

design-fit:
  owner: perfdata report/proposal consumer over existing source semantics
  contract: performance-event-v1 and bounded snapshots from 01/02; register the consumer in docs/contracts.md
  retires: [temporary local perf-v1 triage reader after released parity qualification]
  weight: "Measure added verbs, flags, refusals and cost at implementation; do not invent savings."
  why-add: "Existing source owners lack this composed operator journey. Reuse their outputs rather than a second historian, scheduler or acceptance evaluator."

single-point-of-failure: source coverage and publication scope — independent expected fixtures and lower-boundary authorization tests reject unsupported complete/authorized claims.

## Read first

Read the stream README, docs/library-first.md, docs/report-packs.md, continuing-operations spec sections 5/9, and runtime-contract-alignment. All required outcomes are restated in Tasks and Verify.

## Human decision

Select the concrete daily host, reporting timezone, publication audience, source scopes and retention/byte limits from measured evidence. Accept staged packaging only; actual recurring publication is a separate operator activation.

Options:
1. Accept the demonstrated scope, source coverage and access/effect boundaries; select the concrete operating profile before activation.
2. Hold the unsupported boundary and continue offline fixture work only.

Default if no answer: none — acceptance/activation remains held; offline implementation can proceed.

## Ground rules

- Own isolated branch and draft PR; no main push, merge, live deployment or workflow dispatch.
- Synthetic sources and fake effect boundaries until the applicable operating policy is recorded.
- Stop at implemented; independent verifier fills Evidence.
- Missing coverage, authority or effect outcome never becomes zero, success or permission to retry.

## Task

1. Implement report-pack init/config and a daily wrapper over the exact 07 entrypoint. Collection and rendering are distinct configured operations: collection closes a bounded snapshot; report generation consumes that identity once. A user can regenerate a saved date/window without collection or provider calls. Do not add a scheduler or wait for database/API qualification for local operation.
2. Package pinned released artifacts and generated adopter configuration per docs/report-packs.md. Only the authorized daily writer may replace the committed daily index/report. On-demand output goes to stdout or an explicit user output path and cannot overwrite the canonical daily file. PR validation renders/discards; it never commits generated output. Supply staged workflow only; promotion/host activation remains an operator act.
3. Publish HTML/JSON plus manifest with snapshot/config/build/timezone, source cutoffs and coverage. Preserve historical versions/corrections and configured bounded distribution retention. Missing inputs produce a visible partial report and source gap; an unreadable snapshot does not destroy the last good result or claim no work. Backfill labels approximate source times and cannot invent old CPU or token data.
4. Use stable report occurrence identity, atomic artifact publication and duplicate/concurrent-run reconciliation. A crash before publish or repeated trigger must not create competing current reports. Record missed windows, collection/render duration, failure, data lag and last successful artifact independently; surface a missing report through the existing host health channel.
5. Qualify collection→snapshot→manual generation→daily generation on a 90-day synthetic corpus. Same snapshot/config yields equivalent outputs; late arrival creates an attributable revision. Document command/skill use, source-specific rollout and revert. Runtime migration uses R2 schedules and R7 projections over the same report library, never DB triggers or a second publisher.

## Verify

Named commands below are implementation deliverables. Each must run the named test (not succeed with no matching tests).

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v ./reportpack/... -run '^TestDailyManualParity$' > "$o" && grep -F -- '--- PASS: TestDailyManualParity ' "$o")` | exit 0; named PASS; all three panels and source manifests agree for the same saved snapshot |
| 2 | check:ci +flow | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v ./reportpack/... -run '^TestReportPublicationRecovery$' > "$o" && grep -F -- '--- PASS: TestReportPublicationRecovery ' "$o")` | exit 0; named PASS; duplicate triggers/concurrent publishers/crash preserve one current artifact and previous good version |
| 3 | check:ci | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v ./reportpack/... -run '^TestReportCoverageAndRetention$' > "$o" && grep -F -- '--- PASS: TestReportCoverageAndRetention ' "$o")` | exit 0; named PASS; 90-day trends preserve distribution/coverage; missing capture and expired raw are not zero |
| 4 | check:ci +flow | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v ./reportpack/... -run '^TestReportPackWriterBoundary$' > "$o" && grep -F -- '--- PASS: TestReportPackWriterBoundary ' "$o")` | exit 0; named PASS; unapproved canonical writes refuse; PR render and explicit manual export remain usable without network |
| 5 | check:ci +dereference | `statusgen --root . --consumers --brief performance-collection/08` | exit 0; routing claims corroborated against the implementation diff, including each changed consumer. |

## Evidence

<!-- Independent verifier: revision, environment, command, exit, observed evidence/date/runner. -->

## Review

Gate: human. Verify evidence and lower-boundary failure cases; acceptance does not activate a destination, cadence, credential, policy change or merge permission.
