---
brief: assay:assay:desk-tools:38
title: Correct desk performance attribution and counter coverage
why: "Ordinary desk operations collapse to other and some subprocess/HTTP paths escape existing counters, hiding the cost of the behavior being optimized."
wave: 2
depends: ["desk-tools/37"]
unblocks: ["desk-tools/40", "desk-tools/41", "desk-tools/42"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 3
authored: 2026-10-17 by statusgen newbrief
sources:
  - docs/desk-workflow-efficiency-spec.md
  - docs/desk-workflow-efficiency-change-map.md
  - freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d (App-authenticated main fetch)
  - 'desk-tools: Correct desk performance attribution and counter coverage'
id: fb4b42dd-6877-4b9e-947f-09f9f4b8a201
exec-tier: strong
exec-tier-why: Semantic extraction, cost coverage and cross-caller failure behavior require independent contract judgment.
domain: complicated
consumers:
  - "tools/desk/cmd/deskperf/: out-of-scope (unchanged reader accepts additive verb strings; exercised by existing command suite and LoadPerfHistory flow)"
  - "tools/desk/internal/deskkit/: fixed-here (production attribution/counters and regression paths)"
  - "tools/desk/cmd/deskboard/: fixed-here (production attribution/counters and regression paths)"
  - "tools/desk/cmd/deskdispatch/: fixed-here (production attribution/counters and regression paths)"
  - "tools/desk/cmd/issueboard/: fixed-here (production attribution/counters and regression paths)"
  - "tools/desk/internal/reviewtest/perf_test.go: fixed-here (actual entrypoints, optional sink and planted production mutations)"
  - "tools/desk/internal/deskkit/perf-mutations.json: fixed-here (retain the argv-leak mutation at the changed production record construction)"
  - "docs/review-efficiency-qualification.md: fixed-here (production entrypoint acceptance and honest remaining coverage)"
  - "docs/telemetry.md: fixed-here (closed vocabulary, inventory and measurement limits)"
  - "tools/desk/cmd/deskread/: out-of-scope (existing reader/parser is unchanged; shared process/client helpers gain counters without caller changes)"
  - "tools/desk/cmd/deskwt/: out-of-scope (existing hook caller and process policy unchanged)"
  - "tools/desk/cmd/deskroster/: out-of-scope (existing liveness/visibility readers preserve results; shared transport now observes attempts)"
  - "tools/desk/cmd/verifyloop/: out-of-scope (existing verifier Git caller and policy unchanged)"
  - "tools/desk/cmd/{cellctl,deskautolane,deskclose,deskevidence,deskflip,deskgit,deskinbox,deskmerge,deskmonitor,deskpathguard,deskpost,deskrun,desksupervise,deskverdict,scanloop}/: out-of-scope (class sweep: shared helper counters apply; caller policy, argv and parsing contracts are unchanged)"
  - "perfrec/: out-of-scope (closed schema and offline writer unchanged)"
---

# Brief 38 — Correct desk performance attribution and counter coverage

## Context

files: `tools/desk/internal/deskkit/{perf,runtool,forgetransport,roletoken,verifierattestation,verifiertrackingcheckout,hooks,killswitch,dispatchrecord,callout,untrustscan,riskcallout,layaadvisor,trustliveness,trustliveness_gitlab,repovis}.go` (existing), `tools/desk/internal/deskkit/{perfprocess,perfhttp,citrigger}_test.go` (new/existing), `tools/desk/internal/deskkit/perf-mutations.json` (existing), `tools/desk/cmd/deskboard/{board,dispatch,nextup}.go` (existing), `tools/desk/cmd/deskboard/{perfchild,perfleaf,perfentry}_test.go` (new), `tools/desk/cmd/deskdispatch/{dispatch,stamponly,kits}.go` (existing), `tools/desk/cmd/{deskdispatch,issueboard}/perfentry_test.go` (new), `tools/desk/internal/deskkit/perf.go` (existing), `tools/desk/internal/deskkit/perf_test.go` (existing), `tools/desk/internal/deskkit/perfcoverage_test.go` (existing), `tools/desk/cmd/deskboard/main.go` (existing), `tools/desk/cmd/deskdispatch/main.go` (existing), `tools/desk/cmd/issueboard/main.go` (existing), `tools/desk/internal/reviewtest/` (planned), `docs/telemetry.md` (existing), `docs/review-efficiency-qualification.md` (existing), `changelog/desk-tools-38.md` (planned)

facts: 17 October 2026 at the source pin above. Version 2 expands the declared paths to the actual shared process/client observers and test-only entrypoint adapters; no execution policy or perf-v1 field changes. Version 3 retains the existing argv-leak mutation at its current source anchor and strengthens row 2 with successful ordinary-entrypoint work, nonempty audit history and actual budget/breaker reader observations plus production mutations. desk-tools/37 is merged through PR #3331 and integrated with a two-parent main merge. The deskboard CLI port #3248 merged at pickup and is integrated at its actual recording callback; its help/version/parse refusals keep their no-sample contract. The deskdispatch port #3302 remains open; preserve its parser acceptance separately. Existing implementation is described in
approved spec §2. All named tests and paths marked planned below are deliverables, not
passing claims. desk-tools/23 remains implemented, /24–25 done, /27 retains its own
verification debt; desktools-v2/09 is verified and /45 implemented. Refresh actual source,
claims and open PRs at pickup. #3248 is merged and #3302 remains an active CLI port; #3325 is proposed
supervision guidance and #3305 proposed generic improvement planning. Neither draft is
assumed shipped. The initial blocker is the missing complete observation and attribution,
not the full future runtime.

layering: deskkit PerfRun and existing subprocess/transport counters. Contracts and pure computation remain separated from
credentialed collection/effects; adapters consume values through legal module APIs.
No new shipped service, runtime store or credential owner is authorized. The flow and boundary
mutation cases below enforce this placement.

design-fit:
  owner: deskkit PerfRun and existing subprocess/transport counters
  contract: perf-v1 closed-field schema and existing consent; performance-collection/01–02 own richer events
  retires: duplicate work only after the applicable caller and equivalence acceptance
  weight: verbs +0, flags +0, refusals +0, rule-text +0; existing counters only
  why-add: reuse existing contracts/runner/typed reads; a complete observation is required before removing the second read, and a new cache would preserve that duplication

## Read first

- [Approved source](../../desk-workflow-efficiency-spec.md) and [consolidation](../../desk-workflow-efficiency-change-map.md).
- [Delivery waves](README.md#review-efficiency-follow-ons) and existing source files declared above.
- `docs/library-first.md`, `docs/brief-rules.md` and `docs/telemetry.md` for the applicable boundary/consent rules.

## Ground rules

- Preserve exact-head independent verdicts, claims, trust, budgets, audit and consequential rechecks.
- Use fake providers and disposable local Git for acceptance; live activation and collection are separate gates.
- Stop at implemented; independent verification supplies Evidence. Do not rewrite completed brief Evidence.

## Task

1. Correct command vocabulary and actual dispatch-kit attribution at the production wrappers.
   Preserve perf-v1 fields and readers. If a kit needs a new enum value, demonstrate old-reader
   compatibility and document its mapping; do not add raw argv or infer kit from role. Unknown stays
   explicit.
2. Inventory process launches and forge transports used by the three tools; route bypasses through
   the existing shared runner/counter where semantics allow. Preserve child error/exit handling and
   redaction from desk-tools/21. Document uncovered paths and own-process CPU limitations. Never
   relabel request counts as GraphQL points.
3. Do not add a new spool, daemon, remote sink, listener or legacy schema fields. Phase/correlation
   and quota distinctions belong to performance-collection/01–02; annotate what cannot yet be
   reconstructed. Preserve desk-tools/23 Evidence and /24–27 delivered savings.
4. Reconcile actual main with active CLI ports #3248 and #3302 at pickup. Keep parser/stdout/exit
   compatibility and add attribution tests on the selected adapter; do not expand those ports or
   supersede their acceptance.
5. Update affected command/operator contracts and the declared changelog; regenerate applicable
   documentation with the existing statusgen README generator. No generated global board is branch-
   committed. All planned tests must be wired to actual production paths, not a parallel test
   implementation.

## Verify

Run from the public repository root; each named test is a planned deliverable and its PASS
line is mandatory (zero selected tests is a failure). No authoring-time execution Evidence
is asserted. Each test covers production flow plus the stated wrong-but-plausible mutation.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfCommandKit$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestPerfCommandKit ' "$o")` | exit 0; named PASS. Real entrypoints record stable allowlisted subcommands/kit mappings, including aliases/help/error forms; unknown/private argv never leak. Existing perf-v1 fixtures decode unchanged. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfCoverageFlow$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestPerfCoverageFlow ' "$o")` | exit 0; named PASS. Production runner and transport record planted child/HTTP attempts, retry and 304 exactly once on instrumented paths; consent-off and broken optional sinks preserve exit/stdout/work/audit/budget. Successful file-list, intake and stamp entrypoints preserve seeded history, append exactly one successful audit record and leave the same real budget/breaker decisions; skipped-work and erased-history production mutations fail these assertions. |
| 3 | check:ci +dereference | `statusgen --root . --consumers --brief desk-tools/38` | exit 0; implementation consumer routing is corroborated against the actual diff. Out-of-scope judgments remain review obligations. |

## Pre-mortem and detection

| Failure mode | Caught by |
|---|---|
| Adapter produces plausible output while duplicating or omitting work | row 1, independent oracle and actual entrypoint/count instrumentation |
| A partial/unsupported observation or hidden consumer creates a false success | row 2, planted invalid input and explicit refusal/coverage |
| Operational activation or control deletion inferred from source qualification | review-only: review scope and separate existing adoption gates; no live effects in the test harness |

## Evidence

<!-- Independent verifier appends one row per Verify command after implementation. -->

## Review

Retain the derived frontmatter gate. Approval of the source spec is recorded; this brief
adds implementation acceptance rather than reopening that decision. Existing adopter
writer/custody/equivalence and collection consent gates remain separate.
