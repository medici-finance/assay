---
brief: assay:assay:desk-tools:43
title: Migrate review callers with qualified versions and rollback
why: "Shipping new verbs alone leaves duplicate enumeration in skills, monitors and reporting, and unsupported pins can reintroduce it."
wave: 4
depends: ["desk-tools/40", "desk-tools/41"]
unblocks: ["desk-tools/44", "forge-neutral/45", "graph-execution/27"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 3
authored: 2026-10-17 by statusgen newbrief
sources:
  - "Planning amendment 2026-10-23; source checked on App-fetched main 9c4307a19a190964db084104ecc1f7bd62cba433; candidate-plus-priority parity and GH-call audit"
  - docs/desk-workflow-efficiency-spec.md
  - docs/desk-workflow-efficiency-change-map.md
  - freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d (App-authenticated main fetch)
  - 'desk-tools: Migrate review callers with qualified versions and rollback'
id: 784d05b3-1dec-4cec-9f81-b1583732cf20
exec-tier: strong
exec-tier-why: Semantic extraction, cost coverage and cross-caller failure behavior require independent contract judgment.
domain: complicated
gate-why: Review/issue observations can expose scoped operational facts; implemented coverage and authority boundaries require the existing human acceptance.
decision-trigger: spec
consumers:
  - "plugins/assay/skills/pr-review-desk/SKILL.md: follow-up desk-tools/43"
  - "plugins/assay/skills/the-desk/SKILL.md: follow-up desk-tools/43"
  - "plugins/assay/scripts/pr-monitor.sh: follow-up desk-tools/43"
  - "plugins/assay/scripts/assay-inbox.sh: follow-up desk-tools/43"
  - "tools/desk/cmd/deskboard/throughput.go: follow-up desk-tools/43"
---

# Brief 43 — Migrate review callers with qualified versions and rollback

## Context

files: `plugins/assay/skills/pr-review-desk/SKILL.md` (existing), `plugins/assay/skills/the-desk/SKILL.md` (existing), `plugins/assay/references/desk-common.md` (existing), `plugins/assay/references/codex.md` (existing), `plugins/assay/scripts/pr-monitor.sh` (existing), `plugins/assay/scripts/assay-inbox.sh` (existing), `tools/desk/cmd/deskboard/throughput.go` (existing), `tools/desk/internal/deskkit/reviewcallers_test.go` (planned), `tools/desk/internal/deskkit/citrigger_test.go` (existing), `docs/review-efficiency-compatibility.md` (planned), `changelog/desk-tools-43.md` (planned)

facts: 17 October 2026 at the source pin above. Existing implementation is described in
approved spec §2. All named tests and paths marked planned below are deliverables, not
passing claims. desk-tools/23 remains implemented, /24–25 done, /27 retains its own
verification debt; desktools-v2/09 is verified and /45 implemented. Refresh actual source,
claims and open PRs at pickup. #3248 and #3302 are active CLI ports; #3325 is proposed
supervision guidance and #3305 proposed generic improvement planning. Neither draft is
assumed shipped. The initial blocker is the missing complete observation and attribution,
not the full future runtime.

layering: review skill, shared supervision references, monitors and review-depth consumers. Contracts and pure computation remain separated from
credentialed collection/effects; adapters consume values through legal module APIs.
No new shipped service, runtime store or credential owner is authorized. The flow and boundary
mutation cases below enforce this placement.

design-fit:
  owner: review skill, shared supervision references, monitors and review-depth consumers
  contract: approved spec §8 and A9; existing cadence/wake/independent verdict gates; #3325 proposed shared procedure
  retires: duplicate work only after the applicable caller and equivalence acceptance
  weight: verbs +0, flags +0, refusals +0, rule-text reduced through shared procedure references
  why-add: reuse existing contracts/runner/typed reads; a complete observation is required before removing the second read, and a new cache would preserve that duplication

## Read first

- [Approved source](../../desk-workflow-efficiency-spec.md) and [consolidation](../../desk-workflow-efficiency-change-map.md).
- [Delivery waves](README.md#review-efficiency-follow-ons) and existing source files declared above.
- `docs/library-first.md`, `docs/brief-rules.md` and `docs/telemetry.md` for the applicable boundary/consent rules.

## Ground rules

- Preserve exact-head independent verdicts, claims, trust, budgets, audit and consequential rechecks.
- Use fake providers and disposable local Git for acceptance; live activation and collection are separate gates.
- Stop at implemented; independent verification supplies Evidence. Do not rewrite completed brief Evidence.

## Candidate and priority amendment — 2026-10-23

Migrate both skill/prose and discrete review consumers to the same pure PlanReview result,
including explicit policy/time/capacity and qualified observation applicability. The watcher
hands off its complete observation when compatible; it is not merely a cue for another full
scan. Derive coordinator depth from that same population only at a matching cutoff. Retain
monitor health, due reconciliation, legacy compatibility and final admission/verdict checks.
Document graph-execution/27's later review binding and source surfaces; it consumes this
contract but does not create another classifier. Extend the existing caller test surface and
retirement inventory; both drivers must use policy rank, not operation-key print order.

## Task

1. Publish exact producer/tool, plugin/skill, observation schema, harness/launcher and supported pin
   versions before callers require new forms. Release producer first through existing release owner;
   no invented future tag. Old supported forms remain usable until the declared window ends.
   CI/fixture evidence is distinct from installed-cell adoption. Before activation, qualify
   the whole-cycle hold when one PR is quarantined or degraded, including an externally
   opened public PR. Preserve the prior supported recipe on hold, surface the affected
   population and recovery action, and prove recovery without silently dropping that PR
   or running both effect paths.
2. Replace review actions+prs+plan sequence with one ReviewQueue observation, PlanReview and
   PrepareReview; migrate actual classified watcher/PR monitor, review-depth throughput, coordinator
   guidance and inbox readers. Reuse only a compatible cutoff; preserve independent wake and
   monitor-health responsibilities and fresh-sweep idle gate. Inventory scripts, examples, recipes
   and external pins beyond grep matches.
3. Reconcile open #3325 at pickup: use its one-watch-owner, bounded-handle waits, same-item
   continuation and fresh-new-item context procedure if qualified. Amend that shared reference
   rather than restating it in each skill. If unmerged/unavailable, record the version gap and
   qualify equivalent supported behavior separately; it does not block direct-reader implementation.
   Final independent verdict and consequential freshness checks remain mandatory. Do not claim the
   guidance enforces runtime behavior.
4. Keep active #3248/#3302 parser compatibility and pending #3305 generic scorecard ownership. Do
   not introduce a new scheduler, checkpoint schema or writer switch. Test rollback without running
   both effect paths; register cross-module fixture/source reads and widen existing CI path coverage
   where required.
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
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewCallerFlow$' ./internal/deskkit/... > "$o" && grep -F -- '--- PASS: TestReviewCallerFlow ' "$o")` | exit 0; named PASS. Fixture supervisor uses production queue/plan/preparation path plus monitor/report adapters with one observation per cutoff; no second actions/prs join, overlapping routine watch or duplicate effect occurs. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewCallerRollback$' ./internal/deskkit/... > "$o" && grep -F -- '--- PASS: TestReviewCallerRollback ' "$o")` | exit 0; named PASS. Supported version matrix covers old producer/new skill, new producer/old skill and rollback; incompatible schema fails explicitly. Missed completion, stale head and failed watcher cannot produce idle/ready or drop prior findings. One quarantined/degraded PR (including an external public PR) holds the complete cycle visibly; the supported compatibility recipe and recovery are qualified without dropping the PR or duplicating effects. |
| 3 | check:ci +dereference | `statusgen --root . --consumers --brief desk-tools/43` | exit 0; implementation consumer routing is corroborated against the actual diff. Out-of-scope judgments remain review obligations. |
| 4 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewDriverParity$' ./... > "$o" && grep -F -- '--- PASS: TestReviewDriverParity ' "$o")` | exit 0; named PASS. actual prose/skill recipe adapter and existing reviewloop bridge produce the same limited-slot choices, holds and exact heads. Compatible watcher/caller reuse does not recollect; incompatible scope/time and moved head refresh or refuse. A caller-local ranker or omitted monitor-health obligation fails. |

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
