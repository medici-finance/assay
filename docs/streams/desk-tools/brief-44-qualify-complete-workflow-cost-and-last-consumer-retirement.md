---
brief: assay:assay:desk-tools:44
title: Qualify complete workflow cost and last-consumer retirement
why: "A faster command can shift cost into agents or leave old callers active; deletion needs full-cycle evidence and a supported-consumer inventory."
wave: 5
depends: ["desk-tools/41", "desk-tools/42", "desk-tools/43"]
unblocks: ["forge-neutral/45"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 2
authored: 2026-10-17 by statusgen newbrief
sources:
  - docs/desk-workflow-efficiency-spec.md
  - docs/desk-workflow-efficiency-change-map.md
  - freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d (App-authenticated main fetch)
  - 'desk-tools: Qualify complete workflow cost and last-consumer retirement'
id: b1210181-d8e4-408f-b94f-55ba689aa1be
exec-tier: strong
exec-tier-why: Semantic extraction, cost coverage and cross-caller failure behavior require independent contract judgment.
domain: complicated
consumers:
  - "plugins/assay/skills/pr-review-desk/SKILL.md: follow-up desk-tools/44"
  - "plugins/assay/scripts/pr-monitor.sh: follow-up desk-tools/44"
  - "plugins/assay/scripts/assay-inbox.sh: follow-up desk-tools/44"
  - "Makefile: follow-up desk-tools/44"
  - "docs/review-efficiency-retirements.md: follow-up desk-tools/44"
---

# Brief 44 — Qualify complete workflow cost and last-consumer retirement

## Context

files: `tools/desk/internal/reviewtest/` (planned), `docs/review-efficiency-qualification.md` (planned), `docs/review-efficiency-compatibility.md` (planned), `docs/review-efficiency-retirements.md` (planned), `plugins/assay/scripts/` (existing), `Makefile` (existing), `changelog/desk-tools-44.md` (planned)

facts: 17 October 2026 at the source pin above. Existing implementation is described in
approved spec §2. All named tests and paths marked planned below are deliverables, not
passing claims. desk-tools/23 remains implemented, /24–25 done, /27 retains its own
verification debt; desktools-v2/09 is verified and /45 implemented. Refresh actual source,
claims and open PRs at pickup. #3248 and #3302 are active CLI ports; #3325 is proposed
supervision guidance and #3305 proposed generic improvement planning. Neither draft is
assumed shipped. The initial blocker is the missing complete observation and attribution,
not the full future runtime.

layering: existing review-efficiency qualification record and last-consumer retirement discipline. Contracts and pure computation remain separated from
credentialed collection/effects; adapters consume values through legal module APIs.
No new shipped service, runtime store or credential owner is authorized. The flow and boundary
mutation cases below enforce this placement.

design-fit:
  owner: existing review-efficiency qualification record and last-consumer retirement discipline
  contract: approved spec A1–A10; performance-collection reports optional, existing effect/control gates unchanged
  retires: duplicate work only after the applicable caller and equivalence acceptance
  weight: verbs +0, flags +0, refusals +0, rule-text +0; bounded qualification and retirement records
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

1. Run the frozen oracle and workload suite against baseline and subject for review, issue discovery
   and preparation with all actual callers. Separate tool-only, supervision-only (#3325 when
   adopted) and combined arms; record revisions/versions, cache and permission state, fixture
   hashes, correctness, coverage and CPU/wall/request/points vector. Compare full dispatcher-plus-
   agent work. Unknown children/quota and overlapping wall cannot be scored as savings.
2. Publish a local, reproducible qualification and limitations record. Reuse performance-
   collection/07 exports when available without depending on the parked collection stream, its
   DB/API or new consent. Pending #3305 owns reusable scorecard/experiment mechanisms; consume them
   when qualified rather than creating a second system. A failed/no-go result is valid and blocks an
   improvement claim.
3. Inventory executable, documented/tested and supported external consumers, responsible owner,
   supported pins, replacement, semantic fixture, rollback boundary and removal release. Remove
   duplicate CALLS first, duplicate implementations after parity, subcommands after last consumer,
   and binaries only after all responsibilities migrate. Keep worker/verifier dispatch, intake/raw
   parsing, claims, attestation, budgets, audit, ready/effect boundaries and monitor health until
   their separate qualified replacement.
4. Record candidate retirements and remove only qualified redundant read wrappers in this scope.
   This brief cannot delete a shared control or claim the full desk-simplify equivalence/retirement
   gate. Uninspected pins/cells remain unresolved consumers. Independently verify the merged
   qualification before adopting or retiring at a consumer.
5. Update affected command/operator contracts and the declared changelog; regenerate applicable
   documentation with the existing statusgen README generator. No generated global board is branch-
   committed. All planned tests must be wired to actual production paths, not a parallel test
   implementation.

## Shared execution qualification consumer — 2026-10-18

When graph-execution/27 is qualified, include its invocation-attributed supervisor and
bounded-child records in this brief's existing matched cohort and retirement report.
Its minimal offline launch-count proof is owned by /27 and does not depend on this
whole read-path optimization chain; this broad qualification consumes that proof when
available. No new benchmark, spool, scorecard or analytics owner is introduced. Retain
separate tool-only, supervision-only and combined arms; include total child consumption,
matched completed outcomes, exact-head review/verification coverage and rework. Missing
usage stays unknown; silence caused by blocked/lost work is not a saving.

## Verify

Run from the public repository root; each named test is a planned deliverable and its PASS
line is mandatory (zero selected tests is a failure). No authoring-time execution Evidence
is asserted. Each test covers production flow plus the stated wrong-but-plausible mutation.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestWorkflowCostAcceptance$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestWorkflowCostAcceptance ' "$o")` | exit 0; named PASS. Matched complete-cycle replay passes correctness and preserves declared CPU/wall/attempt/points tradeoffs across workload/fault cohorts; extra work moved into child preparation or duplicate watch fails the cost/coverage claim. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestRetirementConsumerHold$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestRetirementConsumerHold ' "$o")` | exit 0; named PASS. Planted supported legacy consumer/pin, missing rollback version or unqualified replacement prevents retirement. Call removal can pass while shared binary remains; unsupported completeness cannot count as zero use. |
| 3 | check:ci +dereference | `statusgen --root . --consumers --brief desk-tools/44` | exit 0; implementation consumer routing is corroborated against the actual diff. Out-of-scope judgments remain review obligations. |

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
