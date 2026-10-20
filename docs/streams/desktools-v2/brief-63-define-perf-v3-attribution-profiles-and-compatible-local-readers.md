---
brief: assay:assay:desktools-v2:63
title: "Define perf-v3 attribution, profiles and compatible local readers"
why: "Operators cannot associate current local measurements with a brief, issue or agent. One versioned local contract must add that attribution and forge-cost vocabulary without widening the old records or silently collecting identifiable workflow data."
wave: 2
depends: ["desktools-v2/61"]
unblocks: ["desktools-v2/64", "desktools-v2/65", "desktools-v2/62", "statusgen/26", "performance-collection/02"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Workflow attribution exposes local work linkage; review the implemented field, consent and propagation boundary before acceptance."
decision-trigger: spec
issues: []
schema: brief-v3
version: 2
id: 3c6c55aa-ddde-467a-8567-763d7072541f
authored: 2026-10-19 by performance attribution authoring
sources:
  - docs/performance-attribution-spec.md
  - docs/streams/desktools-v2/spec.md
  - docs/telemetry.md
  - docs/library-first.md
  - "freshness-checked 2026-10-19 @ 2e02111ed; perf-v2 implementation 2013b8dc6, no perf-v3 or perfdata module"
exec-tier: strong
exec-tier-why: "Cross-process identity, compatibility and negative-path coverage require design judgment across shared consumers."
domain: complicated
outcome: none
consumers:
  - "perfrec/: fixed-here"
  - "tools/desk/internal/deskkit/perf.go and perfread.go: fixed-here"
  - "tools/desk/cmd/deskperf/: fixed-here"
  - "tools/desk/internal/cli/ and launchers: follow-up desktools-v2/64"
  - "tools/desk/internal/deskkit/forgetransport.go: follow-up desktools-v2/65"
  - "tools/desk/cmd/deskboard and deskdispatch: follow-up desktools-v2/62"
  - "statusgen/: follow-up statusgen/26"
  - "perfdata/: follow-up performance-collection/01"
  - "perfdata/adapters/: follow-up performance-collection/02"
  - "docs/telemetry.md: fixed-here"
---

# Brief 63 — Define perf-v3 attribution, profiles and compatible local readers

## Context

files:
- `perfrec/schema.go`
- `perfrec/session.go`
- `perfrec/perfrec.go`
- `perfrec/context.go (planned)`
- `perfrec/context_test.go (planned)`
- `perfrec/testdata/`
- `perfrec/README.md`
- `tools/desk/internal/deskkit/perf.go`
- `tools/desk/internal/deskkit/perfread.go`
- `tools/desk/cmd/deskperf/`
- `docs/telemetry.md`
- `docs/performance-attribution-spec.md`
- `changelog/perf-v3-contract.md (planned)`

facts (source and date in frontmatter):
- perf-v2 is implemented by 2013b8dc6; its closed schema excludes all work/session IDs. Its Session is a timer, not agent identity. Preserve the completed /61 and its Evidence.
- The existing stdlib-only perfrec module is importable from standalone modules via relative require/replace; deskkit is internal. Do not move credential, forge or database dependencies into the recorder.
- perfdata remains absent. Its future spool/alias/transport contract is separate; it must not block local v3 or be duplicated here.

layering: perfrec owns typed optional observations; existing CLI, launcher and provider adapters retain effects and authority. No new service or credential boundary.

single-point-of-failure: a producer can mislabel optional context or coverage; independent writer/profile validation and production-flow negative controls expose this without granting operational authority.

design-fit:
  owner: perfrec and existing producer adapters
  contract: docs/performance-attribution-spec.md
  retires: [unattributed new performance samples and implicit unknown-as-zero accounting]
  weight: "verbs +0; profiles +2; shared flags +8 across the track, owned by /64; no new sender, service or credential holder"
  why-add: "Reuse the local recorder and effect seams; three bounded M briefs separate schema, propagation and provider observation."

## Read first

Read docs/performance-attribution-spec.md, the Context sources and the implemented /61.
Refresh current source and active work before pickup; all new paths and named tests below are
deliverables, not existing implementation or acceptance evidence.

## Human decision

Review the implemented identifiable local workflow profile and its demonstrated field and
propagation boundaries. Confirm that opt-out and tool-only use cannot retain work or agent
linkage, and that invalid metadata cannot affect the underlying operation.

Options:
1. **Accept the bounded implementation** — accept the demonstrated local disclosure and isolation; retain separate remote activation controls.
2. **Hold for correction** — identify the field, lifecycle or failure boundary that needs correction.

Default if no answer: none — acceptance waits for concrete implementation evidence.

## Ground rules

Feature branch and draft PR only; no merge, release, workflow dispatch or live infrastructure.
Synthetic providers, isolated homes and disposable local processes only. Preserve custody,
claims, budgets, audit, help/version and operation results. Optional collection cannot become
required accounting. Stop at implemented; independent verification owns Evidence.

## Interface contract

Freeze perf-v3 per docs/performance-attribution-spec.md: explicit tool/workflow profile, bounded typed context, session/agent/parent and fresh invocation identity, qualified local brief/issue references, deskRole versus assigned role, and request-duration/GraphQL-cost units and coverage. Existing opt-in with no profile retains existing schemas. Only workflow permits identity; tool-only cannot carry it. Flag > named environment > unavailable; present empty clears. Bad optional context cannot fail the operation. perfrec owns typed resolution; CLI adapters only bind values.

## Task

1. Freeze the wire schema, bounds, vocabulary, work JSON-object flag encoding and env JSON-array encoding, coverage and synthetic examples. Keep raw qualified work references local under workflow consent, restrictive permissions and seven-day retention. Unknown/invalid input is never copied; writer/decoder independently reject disallowed fields even with parser bypassed. Document the aliased perfdata export boundary as future work, without building it.
2. Implement credential-free typed context resolution, random per-invocation identity and the selected-profile measurement API. Session/agent IDs are caller-supplied validated random IDs; absence remains unavailable. Do not search branches, files, transcripts or Git for attribution. Freeze immutable per-invocation context for concurrency. Without an explicit profile preserve legacy producer behavior.
3. Extend the same writer and mixed-version reader/report. Keep v1/v2 output/meanings and unsupported data intact; v3 grouping/filtering by work/session/agent/deskRole/role requires workflow permission. Deduplicate v3 by source/invocation and expose conflicting duplicates; local read filters do not resolve private references through provider calls. Request distributions retain sample/coverage counts; measured, estimated and unknown points never merge.
4. Supply reusable API fixtures for /64, /65, /62, statusgen/26 and the collection importer. Prove old fixtures remain readable, tool profile strips/refuses workflow fields, and bad context/full disk preserves original results. Match on/off timing, concurrency, bytes and drops; record p50/p95/max overhead rather than claiming zero overhead. Update the existing docs/help/API references; no docs-site generator is required for these Markdown/API pages.

## Verify

Named tests below are planned. Each command requires its exact PASS line; a missing test
cannot pass. Plant the stated defect through the production boundary, not a duplicate checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3ContextAndConsent$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3ContextAndConsent ' "$out")` | exit 0; named PASS. Direct library/writer bypass rejects workflow identifiers in tool profile; invalid/empty/overridden values and qualified same-number issues differ; no-profile v1/v2 fixtures remain unchanged. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3RecordToReportFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3RecordToReportFlow ' "$out")` | exit 0; named PASS. Real writer to mixed-version deskperf report; duplicates/conflicts, missing IDs, measured-zero/missing points and inclusive times retain correct grouping and coverage. |
| 3 | check:ci +flow +mutation | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3IsolationAndOverhead$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3IsolationAndOverhead ' "$out")` | exit 0; named PASS. External module builds without desk/network/credential/DB imports; off creates nothing, bounded failed writes preserve results and matched on/off overhead is reported. |
| 4 | check +dereference | `statusgen --root . --consumers --brief desktools-v2/63` | exit 0; changed consumer routing corroborates the actual diff; unavailable is not pass. |

## Pre-mortem and detection

Rows 1–3 plant the counterexamples named in Expect: identity/consent or request-count bugs,
false-zero/compatibility bugs, and changed operational behavior. Row 4 checks shared consumer
routing. Sufficiency of real-world coverage and local disclosure remains reviewer judgment;
fixture success never asserts live adoption or permission.

## Query/cache measurement amendment — 19 October 2026

Extend the schema/reducer fixtures in docs/performance-attribution-spec.md's query/cache
section. Distinguish client logical calls/transport attempts, handler observations and actual
provider attempts; include bounded direct/passthrough/refresh/poll/repair origin, cache source
and coalescing coverage. Optional diagnostic links require the workflow profile and an
explicitly admitted service-protocol option, never automatic raw work/identity forwarding.
Use explicit observation kinds to avoid counting request detail plus a process summary.
Handler-scoped CPU cannot be inferred from concurrent whole-process CPU. Extend
TestPerfV3RecordToReportFlow with warm hit, 50-to-one shared three-page refresh, passthrough
retry, background poll and missing server data; planted duplicate points, false zero and
client-plus-server double logical query counts must fail. This is synthetic contract/reducer
work; it adds no dependency on an unshipped service.

## Evidence

<!-- Independent verifier records subject revision, environment, command, exit and named result. -->

## Review

Gate: human. Reject changed legacy meanings, guessed identity/cost, lost coverage or optional measurement that changes an operation.
