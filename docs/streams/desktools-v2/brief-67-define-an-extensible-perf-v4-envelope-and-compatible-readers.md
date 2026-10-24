---
brief: assay:assay:desktools-v2:67
title: Define an extensible perf-v4 envelope and compatible readers
why: "Optional observation additions should not force readers to discard an otherwise valid performance record."
wave: 3
depends: ["desktools-v2/63", "desktools-v2/69"]
unblocks: ["desktools-v2/68"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Selected workflow diagnostics retain identifiable local work references; compatibility and privacy need human acceptance."
issues: []
schema: brief-v2
version: 1
authored: 2026-10-21 by statusgen newbrief
sources:
  - 'desktools-v2: Define an extensible perf-v4 envelope and compatible readers'
  - freshness-checked 2026-10-21 @ af897c3c1 (origin/main)
exec-tier: strong
exec-tier-why: "Cross-contract compatibility and consumer closure require design judgment."
domain: complicated
outcome: none
consumers:
  - "perfrec/: fixed-here, canonical codec/typed facets/selectors/reducer"
  - "schemas/perf-v4.json: fixed-here, derived member contract"
  - "tools/desk/cmd/deskperf/ and tools/desk/internal/deskkit/perfread.go: fixed-here, actual reader/report adapters"
  - "writer profile selection and all producer wrappers: follow-up desktools-v2/68"
  - "performance-collection reader: out-of-scope (explicit v4 reader amendment required before import)"
---

# Brief 67 — Define an extensible perf-v4 envelope and compatible readers

## Context

files: perfrec/{schema,v3,context,reduce}.go; tools/desk/internal/deskkit/{perf,perfread}.go; tools/desk/cmd/deskperf/report.go; docs/performance-attribution-spec.md (21 October successor section).
facts: At af897c3c112bd426cbbc794839dca6d41df911d6, v3 is a whole-record schema with strict fields and unversioned V3Data. Existing local records/legacy meanings must remain readable. /63 recorded PASS is a dependency; the human-gated board row alone is not proof it lacks PASS.

Design-fit: extend perfrec's existing pure codec/observation types, not a second recorder or reducer. The /69 specification owns the wire design. The file adapter remains deskkit/statusgen's existing owner; deskperf projects the same reduction. No writer default changes in this brief.

## Human decision

Accept the concrete v4 core/four-extension wire contract and bounded optional-version
handling in the successor spec, retaining existing identifiable workflow profile
consent. Recommended: reader-first v4 with old readers retained and no default
selection change here. Alternative: retain v3-only readers, which preserves current
behavior but delays independently evolving observations. Do not weaken privacy or
reinterpret existing data. Existing /63 start decisions are not asked again; this
is the new contract's acceptance, through the normal human-gated flow.

## Ground rules

Use normal native worker admission, one isolated worktree/PR and role-bound publication.
No workflow triggers, releases, installs, uploads or live infrastructure. Preserve other
owners and completed evidence. Stop at implemented; independent verification owns Evidence.

## Task

1. At implementation, replace this authoring-time self-routing with fixed-here for actual changed paths. Implement SchemaV4 and publish a machine-readable contract/fixtures from the spec. Keep existing v1/v2/v3 wire fixtures and meanings unchanged. Derive extension coverage/field checks from the reference registry so schema and validator cannot drift.
2. Implement bounded exact-key/duplicate-aware core decode and four registered version-1 extensions. Unknown optional extensions/versions retain independent core observations with unsupported coverage; malformed known data refuses. Never expose arbitrary producer RawMessage payloads or echo untrusted names/data.
3. Normalize supported observations into shared reduction types. Add a distinct perfV4 JSON report group while keeping old report groups unchanged when v4 is absent. Core role is literal none for tool profile, and assigned execution role or none for workflow; supported attribution must agree. Legacy --role excludes all v3/v4; --assigned-role requires supported matching workflow attribution and never matches core role alone. Exercise both filters/profiles in mixed v1-v4 history, including combined flags, invalid/conflicting IDs, unavailable attribution, query not-applicable versus unavailable by observation kind and boot-step exclusions without double charging.
4. Test the real typed writer → JSONL file reader → reducer → compiled deskperf report and a public external Go consumer. Preserve profile/consent isolation and file permissions; default producer selection remains unchanged pending /68.
5. Run cold/warm matched codec/reducer fixtures, report p50/p95 with inputs/counts and no invented threshold. The output is local measurement evidence, not collection activation or a process startup savings claim.

## Implementation pickup — 24 October 2026

Design-fit **continue** at main `61eb62a4a61bce0f2e0c69b98f1906de80400316`.
The reader branch retains the published command-cost report from PR #4352 and
merges actual main; it does not overwrite that parent or reinterpret its totals.
The canonical library remains `perfrec`; desk/statusgen adapters retain their
current authority. Optional facets use historical observation validators and
reduction types. No authenticated online adapter moves into statusgen.

Fixed-here: codec, typed facet registry, bounds/privacy/identity tests, reference
schema, external consumer and actual deskperf file/report projections. The original six
Verify row identities, commands and Evidence below remain unchanged. Row 7 adds
the explicit local mutation qualification required for the new reader controls. /68 remains
a separate reader-dependent selected-producer change; this implementation does
not advance lifecycle, independently verify itself or claim consumer rollout.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow | `cd perfrec && GOWORK=off go test -count=1 -v -run '^TestV4WireCompatibility$' .` | exit 0 and named PASS; golden v1/v2/v3 unchanged; v4 supported and unknown optional versions retain correct core |
| 2 | check:ci +flow | `cd perfrec && GOWORK=off go test -count=1 -v -run '^TestV4ExtensionsFailClosed$' .` | exit 0 and named PASS; negative controls reject duplicate/case aliases, invalid known payload, core override and privacy leaks; deleting each relevant guard makes its mutation fail |
| 3 | check:ci +flow | `cd perfrec && GOWORK=off go test -count=1 -v -run '^TestV4CodecBounds$' .` | exit 0 and named PASS; maximum typed record plus every byte/depth/count/version boundary; no oversized allocation or partial record |
| 4 | check:ci +flow | `cd tools/desk && GOWORK=off go test -count=1 -v -run '^TestV4ReadReportFlow$' ./cmd/deskperf` | exit 0 and named PASS; actual files and compiled CLI JSON; tool role none, workflow assigned role, legacy --role excludes v3/v4, assigned-role needs supported matching workflow attribution, including combined flags; query not-applicable/unavailable by kind; no double-counted summaries |
| 5 | check:ci +flow | `cd perfrec && GOWORK=off go test -count=1 -v -run '^TestV4ExternalConsumer$' .` | exit 0 and named PASS; separate consumer module uses public API with supported/unsupported optional extensions |
| 6 | check:ci +flow | `statusgen --root . --lint` | exit 0 |
| 7 | check +mutation | `cd perfrec && muhar -j 1 -spec v4-mutations.json && muhar -j 1 -spec overflow-mutations.json && muhar -j 1 -spec mutations.json && cd ../tools/desk && muhar -j 1 -spec cmd/deskperf/v4-reader-mutations.json` | exit 0; healthy baseline, distinct caught control, all v4/overflow and retained v3 mutations caught, zero survivors/could-not-mutate; exact restoration |

Pre-mortem: permissive known fields/privacy leaks → row 2; unsupported extensions
blanking all timing → rows 1/4; bound bypass → row 3; desk-internal coupling → row 5.
The implementation must create the exact named tests; an empty go-test match is not PASS.

## Evidence

<!-- Independent postmerge verifier records actual commands, source and outcomes here. -->

## Review

Independent correctness and security review of the exact head; normal ready/merge gates.
