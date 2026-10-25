---
brief: assay:assay:desktools-v2:69
title: Specify extensible JSON contracts and migration targets
why: "Whole-record version bumps couple unrelated additions and make supported transitions hard to qualify."
wave: 1
depends: []
unblocks: ["desktools-v2/67"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-21 by statusgen newbrief
sources:
  - 'desktools-v2: Specify extensible JSON contracts and migration targets'
  - "freshness-checked 2026-10-21 @ af897c3c112bd426cbbc794839dca6d41df911d6 (origin/main fetched)"
exec-tier: strong
exec-tier-why: "Cross-contract compatibility and consumer closure require design judgment."
domain: complicated
outcome: none
consumers:
  - "docs/proposals/non-ci-architecture/: fixed-here"
  - "docs/performance-attribution-spec.md: fixed-here"
  - "docs/contracts.md: fixed-here"
  - "perfrec/ and tools/desk/cmd/deskperf/: follow-up desktools-v2/67"
  - "producer selection: follow-up desktools-v2/68"
  - "other JSON implementations: out-of-scope (migration targets specified here; implementation belongs to their owning contract revisions)"
---

# Brief 69 — Specify extensible JSON contracts and migration targets

## Context

files: docs/contracts.md; docs/proposals/non-ci-architecture/{11,12,16,20,29}-*.md; docs/performance-attribution-spec.md; docs/streams/desktools-v2/brief-67* and brief-68*.
facts: Source census pinned to af897c3c112bd426cbbc794839dca6d41df911d6 on 2026-10-21. perfrec has v1/v2/v3 whole-record dispatch; runner requests already have optional ext/required capabilities; opmetrics/2 documents additive fields. These are source observations, not deployed inventory. Driver requested perf-v4 and future architecture extension migration on 21 October.

Design-fit: update the existing future architecture, not a parallel spec. This is docs/spec delivery plus authorship of /67 and /68; it delivers neither runtime brief. All risk answers are no because no codec, data collection, credentials, live configuration or authority transition changes in this item.

## Ground rules

Use normal native worker admission, one isolated worktree/PR and role-bound publication.
No workflow triggers, releases, installs, uploads or live infrastructure. Preserve other
owners and completed evidence. Stop at implemented; independent verification owns Evidence.

## Task

1. Amend the controlling target, model, delivery sequence and owner manifest to use stable cores and independently versioned extensions. Preserve revision-bound prior approvals and current-versus-target distinctions.
2. Publish the pinned source census with actual codec witnesses and ownership/transition dispositions, including existing extensible precedents and authority-bearing contracts. State discovery gaps; do not equate grep sites with unique protocols or tool deployment counts.
3. Freeze the performance successor contract in the existing attribution spec: fields, bounds, optional-version handling, privacy, historical reads, mixed reports and reader-first selection. Author /67 and /68 with named discriminating tests and explicit dependencies.
4. Preserve /26 snapshot reuse/bench and /62 integration obligations and all independent /60/64/65 holds. Reconcile future format targets in the architecture without changing completed Verify/Evidence or someone else's active code.

## Verify

| # | Command | Expect |
|---|---|---|
| 1 | `git diff --check` | exit 0 |
| 2 | `statusgen --root . --lint` | exit 0; no new lint failures |
| 3 | `git show af897c3c112bd426cbbc794839dca6d41df911d6:loopadmin/runner/request.go` | exit 0; DecodeRequest separates strict core from ext, and CheckCapabilities refuses unsupported require entries, corroborating the census precedent |
| 4 | `git show af897c3c112bd426cbbc794839dca6d41df911d6:tools/desk/cmd/opmetrics/emit.go` | exit 0; SchemaVersion comment explicitly permits additive fields, disproving a blanket claim that all JSON is closed |
| 5 | `git show af897c3c112bd426cbbc794839dca6d41df911d6:perfrec/v3.go` | exit 0; V3Data matches all mapped payload members, including requestDrops and nullable query, with no invented queryCoverage field; validV3 requires tool role none and workflow role equality, and query observations only apply to client/handler kinds |
| 6 | `git show af897c3c112bd426cbbc794839dca6d41df911d6:tools/desk/internal/deskkit/perfread.go` | exit 0; FilterPerf explicitly excludes v3 from the legacy desk-role filter, corroborating the retained distinction from workflow assigned-role filtering |

Pre-mortem: falsely describing every decoder as closed is caught by rows 3–4;
losing requestDrops or fabricating query coverage is caught by row 5; conflating
legacy desk role with assigned role is caught by rows 5–6. Adequacy of
family grouping, future authority boundaries and consumer migration is review-only:
reviewers must challenge the source witnesses and retained dependency obligations,
not accept section presence as architectural conformance.

## Evidence

<!-- Independent postmerge verifier records actual commands, source and outcomes here. -->

Verified against medici-finance/assay origin/main 917772b3ee4709bc47e5d0888cb5cc698444b13e, which contains delivery merge 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (PR 3830). Totals: 6 of 6 required rows run, 6 pass, 0 fail, 0 could-not-run.

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `git diff --check` | exit 0 | exit 0; no whitespace errors reported | 2026-10-21 | assay-verifier-app[bot] @ 917772b3ee4709bc47e5d0888cb5cc698444b13e (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `statusgen --root . --lint` | exit 0; no new lint failures | exit 0; LINT: PASS (statusgen v1.0.54); no PROBLEM line names desktools-v2/67, /68 or /69 | 2026-10-21 | assay-verifier-app[bot] @ 917772b3ee4709bc47e5d0888cb5cc698444b13e (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `git show af897c3c112bd426cbbc794839dca6d41df911d6:loopadmin/runner/request.go` | exit 0; DecodeRequest separates strict core from ext, and CheckCapabilities refuses unsupported require entries | exit 0; 475 lines; LaunchRequest carries Extensions under json ext; DecodeRequest runs scanStrict, checkKeys and DisallowUnknownFields and its comment says optional additions travel under ext; CheckCapabilities returns ErrUnsupportedMandatory for an undeclared ext: entry, an undefined capability and an undeclared capability | 2026-10-21 | assay-verifier-app[bot] @ 917772b3ee4709bc47e5d0888cb5cc698444b13e (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `git show af897c3c112bd426cbbc794839dca6d41df911d6:tools/desk/cmd/opmetrics/emit.go` | exit 0; SchemaVersion comment explicitly permits additive fields | exit 0; 259 lines; comment above SchemaVersion = opmetrics/2 reads: Bump on any field REMOVAL or meaning change; additive fields do not require a bump | 2026-10-21 | assay-verifier-app[bot] @ 917772b3ee4709bc47e5d0888cb5cc698444b13e (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `git show af897c3c112bd426cbbc794839dca6d41df911d6:perfrec/v3.go` | exit 0; V3Data matches all mapped payload members incl. requestDrops and nullable query, no queryCoverage; validV3 tool role none and workflow role equality; query only for client/handler | exit 0; 459 lines; V3Data has 7 members (profile, attribution, observationKind, requests, requestDrops, requestCoverage, query as pointer with omitempty), each mapped by the perf-v4 roster in the attribution spec (core profile/observationKind; requests extension carries requests, requestDrops, requestCoverage; query extension carries query); zero matches for queryCoverage; validV3 rejects tool profile with role other than none and workflow profile whose role differs from the attribution role (none allowed for empty); validQuery returns false for a non-nil query unless kind is client or handler | 2026-10-21 | assay-verifier-app[bot] @ 917772b3ee4709bc47e5d0888cb5cc698444b13e (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | `git show af897c3c112bd426cbbc794839dca6d41df911d6:tools/desk/internal/deskkit/perfread.go` | exit 0; FilterPerf explicitly excludes v3 from the legacy desk-role filter | exit 0; 527 lines; FilterPerf comment says a legacy role filter excludes v3, which needs the workflow-aware ReduceV3 filter; code drops a record when role is set and Schema equals perfrec.SchemaV3 | 2026-10-21 | assay-verifier-app[bot] @ 917772b3ee4709bc47e5d0888cb5cc698444b13e (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Verification-Attestation: medici-finance/assay#3847 run=e75a5c355045adecc0cdda6a3d3c55032510297b376a95a15cb4654f4835c673 source=917772b3ee4709bc47e5d0888cb5cc698444b13e brief=docs/streams/desktools-v2/brief-69-specify-extensible-json-contracts-and-migration-targets.md model=claude-opus-5-5 tier=strong

Risk-bearing values: the frontmatter carries risk metadata with every answer no, and the delivery is docs/spec only (no codec, config or authority change), so the fail-safe trigger does not fire. These literals were enumerated from the delivery diff anyway. All are proposed design bounds for desktools-v2/67, reversible by a docs edit, so they rank as reversible:

- RISK-VALUE: DERIVED — v4 JSON line size < 1 MiB including newline @ docs/performance-attribution-spec.md:356 — stays below the existing reader scanner ceiling sc.Buffer(..., 1<<20) at tools/desk/internal/deskkit/perfread.go:121. This derives scanner byte capacity only, not schema acceptance: pre-v4 readers may reject perf-v4, and qualified readers must be distributed before selecting the writer (docs/performance-attribution-spec.md:387).
- RISK-VALUE: NAMED, NOT DERIVED — max extensions = eight; name grammar = [a-z][a-z0-9-]{0,31}; max version = 2147483647; max nesting depth = 32 @ docs/performance-attribution-spec.md:357-359 — 2147483647 is int32 max and eight is four initial groups plus headroom, per the spec's own text; the depth-32 adequacy depends on the largest typed v4 record, which the spec assigns to /67 boundary tests, not yet implemented.
- RISK-VALUE question filed: #3851 (perf-v4 envelope bounds; the brief advances, the derivation stays open there).

**VERIFY: PASS** — 6/6 rows

### Interpretation correction — 2026-10-21

Correction of F-v4-reader-claim and F-v4-reader-guarantee on PR #3852 by gpt-6-astra, strong tier. The corrected derivation above is limited to scanner byte capacity. Static source inspection at 116a2f1494fca6c97209f10883861467f729216d confirms that tools/desk/internal/deskkit/perfread.go:128 skips schemas outside v1/v2/v3 and perfrec/schema.go:228-281 rejects unknown schemas; docs/performance-attribution-spec.md:356 and :387 retain the strict size bound and reader-first rollout requirement. Those reader files and these spec passages match the original verification source and reviewed PR head.

This admission covers only the interpretation correction. No Verify-table rerun, execution witness, new acceptance result or lifecycle advancement is claimed. The original six observations, date, runner, source, totals and PASS marker above remain attributed to run #3847 and its original binding. The named-but-not-derived bounds and question #3851 remain unchanged.

Verification-Attestation: medici-finance/assay#3928 run=59a39fbe97a350fdcf76b628305403442ef2de849af7c247fa32fbcaf0b2e9e2 source=116a2f1494fca6c97209f10883861467f729216d brief=docs/streams/desktools-v2/brief-69-specify-extensible-json-contracts-and-migration-targets.md model=gpt-6-astra tier=strong

### Execution witness re-verification — 2026-10-24

Verified against medici-finance/assay origin/main 385cb29e5682dcb9a886b298076f722b4823ec1b, which contains delivery merge 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (PR 3830). The earlier blocks above are unchanged. This block adds the statusgen verifyrun (v1.0.59) execution witness that the earlier hand-recorded table lacks. All six rows are legacy-class (none is check:ci), so one host verifyrun pass on a clean admitted home ran the whole table; rows 1 and 3-6 also read the home's git history, which the container lane does not mount. Totals: 6 of 6 required rows run, 6 pass, 0 fail, 0 could-not-run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `git diff --check` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 2 | `statusgen --root . --lint` | pass exit=0 | sha256:a621a2dd6336 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 3 | `git show af897c3c112bd426cbbc794839dca6d41df911d6:loopadmin/runner/request.go` | pass exit=0 | sha256:290349cc0d7f | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 4 | `git show af897c3c112bd426cbbc794839dca6d41df911d6:tools/desk/cmd/opmetrics/emit.go` | pass exit=0 | sha256:7fc5609df4aa | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 5 | `git show af897c3c112bd426cbbc794839dca6d41df911d6:perfrec/v3.go` | pass exit=0 | sha256:02bb1ae8df6f | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 6 | `git show af897c3c112bd426cbbc794839dca6d41df911d6:tools/desk/internal/deskkit/perfread.go` | pass exit=0 | sha256:4901dd4ca761 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4298 run=68eaa8b9252f59343de330065facd47d0b8e749a118347c9c2c142e94abd502f source=385cb29e5682dcb9a886b298076f722b4823ec1b brief=docs/streams/desktools-v2/brief-69-specify-extensible-json-contracts-and-migration-targets.md model=claude-opus-5-5 tier=strong

Expect-content reading of the witnessed output, same run, 2026-10-24, assay-verifier-app[bot] at 385cb29e5682 (each digest above was recomputed from the same command's stdout and matches):

- Row 1: empty output (sha256:e3b0c44298fc is the empty-string digest), so no whitespace errors.
- Row 2: LINT: PASS; no PROBLEM line names desktools-v2/67, /68 or /69 (NOTICE lines only).
- Row 3: 475 lines. LaunchRequest carries Extensions under json ext; DecodeRequest runs scanStrict, checkKeys and DisallowUnknownFields, and its comment says optional additions travel under ext. CheckCapabilities returns ErrUnsupportedMandatory for a require entry naming an undeclared ext: extension, a capability this version does not define, or a capability the adapter does not declare.
- Row 4: 259 lines. The comment above SchemaVersion = opmetrics/2 reads: Bump on any field REMOVAL or meaning change; additive fields do not require a bump.
- Row 5: 459 lines. V3Data has seven members (profile, attribution, observationKind, requests, requestDrops, requestCoverage, and query as a pointer with omitempty). Each is mapped in the perf-v4 roster of the attribution spec: profile and observationKind in the core, attribution in the attribution extension, requests, requestDrops and requestCoverage in the requests extension, query in the query extension. queryCoverage has zero matches in the file. validV3 rejects a tool profile whose role is anything other than RoleNone, and a workflow profile whose role differs from the attribution role (RoleNone allowed only for an empty attribution role). validQuery returns false for a non-nil query unless the kind is client or handler.
- Row 6: 527 lines. The FilterPerf comment says a legacy role filter excludes v3, which needs the workflow-aware ReduceV3 filter, and the code drops a record when role is set and Schema equals perfrec.SchemaV3.

Risk-bearing values: the frontmatter carries risk metadata with every answer no, and the delivery is docs/spec only, so the fail-safe trigger does not fire. Re-enumerated at this source anyway; the literals and their lines are unchanged from the earlier block, and all are reversible proposed design bounds for desktools-v2/67:

- RISK-VALUE: DERIVED — v4 JSON line size < 1 MiB including newline @ docs/performance-attribution-spec.md:356 — stays below the reader scanner ceiling sc.Buffer(..., 1<<20) at tools/desk/internal/deskkit/perfread.go:121. This covers scanner byte capacity only; pre-v4 readers may reject perf-v4, and the spec requires qualified readers before the writer is selected (docs/performance-attribution-spec.md:387).
- RISK-VALUE: NAMED, NOT DERIVED — max extensions = eight; name grammar = [a-z][a-z0-9-]{0,31}; max version = 2147483647; max nesting depth = 32 @ docs/performance-attribution-spec.md:357-359 — 2147483647 is int32 max and eight is four initial groups plus headroom, per the spec's own text; whether depth 32 is enough depends on the largest typed v4 record, which the spec assigns to /67 boundary tests. Question #3851 stays the route for that derivation.

**VERIFY: PASS** — 6/6 rows

## Review

Independent correctness and security review of the exact head; normal ready/merge gates.
