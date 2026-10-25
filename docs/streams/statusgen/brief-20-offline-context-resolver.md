---
brief: assay:assay:statusgen:20
title: Offline future-state resolver with receipt and amendment validation
why: "An author can only amend the briefs they find. A reproducible, offline resolver that enumerates owners and consumers, records what it could not see, and rechecks a patch against its base turns 'did we miss a consumer or overwrite history?' into a check instead of a reviewer's memory."
wave: 2
depends: ["statusgen/19"]
unblocks: ["statusgen/21", "statusgen/23"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 4
authored: 2026-10-13 by worker-desk (author-brief), non-CI architecture reconciliation
sources:
  - "assay:assay:statusgen:26 — consumer routing for the 2026-10-16 performance amendment"
  - "2026-10-16 performance authoring amendment, inspected origin/main eba941721; docs/streams/desktools-v2/README.md performance priority"
  - docs/proposals/non-ci-architecture/18-authoring-context-delivery.md — packages "Offline future-state resolver" and "Receipt and amendment validation"
  - docs/proposals/non-ci-architecture/15-regression-from-day-one.md — "Authoring and plan-evolution qualification"
  - docs/proposals/non-ci-architecture/17-context-substrate.md
  - docs/proposals/non-ci-architecture/20-amendment-manifest.md — step-2 owner row
  - "medici-finance/assay#2500 — maintainer ratification of the whole target @ 23f655f5d (approval activates nothing)"
  - freshness-checked 2026-10-13 @ a435badbd (origin/main)
exec-tier: strong
exec-tier-why: Discovery completeness, history protection and active-work rebinding are cross-component semantic judgments; a wrong rule silently drops obligations.
domain: complicated
outcome: none
consumers:
  - "statusgen/contexthandoff (resolve, recheck, amend), statusgen/context.go, statusgen/main.go, statusgen/testdata/contexthandoff/journeys, docs/context-handoff.md: follow-up statusgen/20"
  - "plugins/assay/skills/author-spec, plugins/assay/skills/author-brief: follow-up statusgen/21"
  - "statusgen graph export (statusgen/graph.go): out-of-scope (read through its existing API; export meaning unchanged)"
id: e4824b70-a2fc-4469-a929-b783a3ec7086
---

# Brief 20 — Offline future-state resolver with receipt and amendment validation

## Context

files: `statusgen/contexthandoff/` (planned) (resolve, recheck and amend files plus tests; the package exists after statusgen/19), `statusgen/context.go` (planned) (new CLI adapter), `statusgen/main.go` (subcommand dispatch), `statusgen/testdata/contexthandoff/journeys/` (planned), `docs/context-handoff.md` (planned), `changelog/statusgen-20-offline-context-resolver.md` (planned).

facts (a435badbd, 2026-10-13):
- statusgen dispatches positional subcommands from `main.go` (for example `verifyrun`,
  `shardcheck`, `reconcile`); no `context` subcommand exists.
- statusgen already parses briefs, typed edges and `consumers:` and exports a deterministic graph;
  the resolver reuses that parsing and adds independent directory enumeration so a prose-only
  consumer without README rows or typed edges is still found.
- Populated `## Evidence` sections and dated source accounts (for example dated amendment
  sections and `freshness-checked` lines) are history: a patch that rewrites them fails even when
  it applies cleanly.
- Claims are dispatch refs on the forge; the resolver reads a supplied fixture snapshot of
  them, never a live forge.
- Effort L is deliberate: resolver and receipt/amendment validation share one snapshot model and
  one fixture corpus. Re-cut into resolve and recheck briefs before dispatch if the snapshot model
  needs parser changes outside `statusgen/contexthandoff` (planned).

layering: `statusgen/contexthandoff` (planned) stays pure and works over an explicit snapshot (files, typed
edges, claims, approvals) handed to it; `statusgen/context.go` (planned) is the only adapter that reads the
working tree. No network, service, database or private corpus.

single-point-of-failure: the history-preservation check on a candidate amendment — behind it, the
existing brief lint (`statusgen --lint`) independently refuses malformed Evidence tables, and the
statusgen/19 contract refuses an amendment manifest with an unbound "amend" obligation.

design-fit:
  owner: statusgen/contexthandoff (same owner as statusgen/19)
  contract: context-handoff-v1 and amendment-manifest-v1 (no new contract)
  retires: [manual impact inventories as the only discovery record]
  weight: "verbs 1 (statusgen context with discover, recheck and check), flags 2 (--snapshot, --json), refusals 3 (unbound amendment, rewritten history, silent rebinding)"
  why-add: "Doc 18 requires a reproducible offline operation both skills share; no existing verb discovers consumers outside typed edges or rechecks a patch against history."

## Read first

- `docs/proposals/non-ci-architecture/15-regression-from-day-one.md` ("Authoring and
  plan-evolution qualification" table: these cases are this brief's tests).
- `docs/proposals/non-ci-architecture/17-context-substrate.md`,
  `docs/proposals/non-ci-architecture/18-authoring-context-delivery.md`.
- `docs/streams/statusgen/brief-19-context-handoff-contracts.md`.

## Ground rules

- CLI built on the desktools-v2 Cobra/Viper foundation (desktools-v2 cli-contract.md §7): every new command, verb or launcher this brief adds uses that shared foundation (added 2026-10-18).
- Isolated branch; draft PR only. No merge or release.
- Neutral synthetic fixtures (two example repositories, withheld consumers). Do not hard-code the
  expected answer list into production discovery; tests withhold expected identities from the
  resolver input.
- Stop at implemented; independent verification owns verified/done.

## Portfolio scope amendment — 13 October 2026

Statusgen/19 has merged; preserve its shipped contract and Evidence. This brief continues
architecture discovery and receipt checks. Product-target records and binding semantics
are owned by statusgen/22–23; /23 consumes this brief rather than expanding /19.
Discovery output must identify architecture-only coverage until that extension is supported.
No product binding means unchecked/unbound, not product alignment. The named new fixture
below checks that an architecture receipt is not represented as a product-alignment result.

## Implementation pickup amendment — 18 October 2026

Extend the existing snapshot/query/recheck path to include applicable project
engineering guidelines, selected approved target/transition, semantic owners,
producer versions and planned replacements. Include newly added guideline/consumer
sources outside the previous read set. Reuse receipts and non-intersection assessment;
an unapproved proposal cannot invalidate approved work. No new target registry or
unrelated graph-service dependency. Missing required source is scoped unknown.

## Task

1. Resolve: given a target and an explicit snapshot, enumerate owners and consumers from typed
   edges, `consumers:` fields and independent directory enumeration with deterministic prose
   candidate discovery; record limits and coverage, including unavailable inputs as scoped
   could-not-check without revealing private content or counts.
2. Recheck: given a prior receipt and a new snapshot, distinguish an unrelated edit (unaffected
   conclusions preserved), a new or deleted consumer outside the prior read set (affected
   reconciliation required) and a missing input (scoped hold).
3. Amend check: bind each candidate patch to its base revision; refuse a patch that does not apply,
   rewrites a populated Evidence section or a dated source account, silently rebinds an active
   worker's claim, or combines incompatible approved deltas. Route new obligations for a delivered
   brief with populated Evidence to an unfinished owner or a linked follow-on.
4. Add `statusgen context discover|recheck|check --snapshot DIR [--json]` as a thin adapter over
   the package, `docs/context-handoff.md` (planned), and the changelog fragment.

## Interface contract

Input: a target record (statusgen/19) and a snapshot directory. Output: a discovery receipt and,
for `check`, a pass/refusal per obligation, all in `context-handoff-v1`. statusgen/21's skills
call only this shipped verb and schema version.

## Verify

The named tests are deliverables of this brief and do not exist at a435badbd. Each mirrors a row
of doc 15's authoring qualification table.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextResolverWithheldConsumers$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextResolverWithheldConsumers " "$out")` | exit 0; named PASS; withheld synthetic owners and consumers (including a prose-only consumer with no typed edge) are discovered and dispositioned; mutation: discover from typed edges only — the test must fail |
| 2 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextRecheckUnrelatedEdit$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextRecheckUnrelatedEdit " "$out")` | exit 0; named PASS; same inputs reproduce the receipt digest, an unrelated edit preserves unaffected conclusions; mutation: invalidate every conclusion on any edit — the test must fail |
| 3 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextRecheckNewConsumerOutsideReadSet$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextRecheckNewConsumerOutsideReadSet " "$out")` | exit 0; named PASS; a new consumer outside the prior read set forces reconciliation although every previously read file hash is unchanged; mutation: recheck only previously read file hashes — the test must fail |
| 4 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextMissingInputScoped$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextMissingInputScoped " "$out")` | exit 0; named PASS; an unavailable input yields scoped could-not-check, unrelated authorized work continues, and output carries no private names or counts; mutation: report the unavailable source as empty — the test must fail |
| 5 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextAmendmentPreservesDatedSources$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextAmendmentPreservesDatedSources " "$out")` | exit 0; named PASS; a cleanly applying patch that rewrites a populated Evidence row or a dated source account in an unfinished brief fails, and the additive amendment passes; mutation: check only that the patch applies — the test must fail |
| 6 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextActiveWorkerRefusesSilentRebind$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextActiveWorkerRefusesSilentRebind " "$out")` | exit 0; named PASS; with a fixture claim at an older brief revision the amendment is refused until an acknowledged adjustment or cancellation is recorded; mutation: ignore claims — the test must fail |
| 7 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextCompetingProposals$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextCompetingProposals " "$out")` | exit 0; named PASS; a clean merge of two incompatible approved deltas is refused, and an unapproved rival does not block the approved one; mutation: accept any textually clean merge — the test must fail |
| 8 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextDeliveredBriefRoutesFollowOn$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextDeliveredBriefRoutesFollowOn " "$out")` | exit 0; named PASS; a new obligation on a delivered brief with populated Evidence routes to an unfinished owner or linked follow-on and leaves the delivered brief unchanged; mutation: patch the delivered brief's Task — the test must fail |
| 9 | check:ci +flow | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextCLIDiscoverRoundTrip$" . > "$out" && grep -F -- "--- PASS: TestContextCLIDiscoverRoundTrip " "$out")` | exit 0; named PASS; the built `statusgen context discover --snapshot <fixture> --json` output decodes with the statusgen/19 package and `context check` on its own receipt passes |
| 10 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextArchitectureCoverageNotProductAlignment$" . > "$check_output" && grep -F -- "--- PASS: TestContextArchitectureCoverageNotProductAlignment " "$check_output")` | exit 0; named PASS; architecture discovery through CLI explicitly leaves unsupported product alignment unchecked; mutation: label a receipt covering architecture only as aligned to the product target |
| 11 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPickupContextRecheck$' . > "$out" && grep -q '^--- PASS: TestPickupContextRecheck ' "$out")` | exit 0; snapshot fixtures distinguish approved owner/guideline changes, new consumer outside old membership, unrelated edit, unavailable required input and draft-only rival; mutation: ignore a new guideline/replacement source and require the corresponding case to fail |

## Pre-mortem

Likely wrong builds: typed-edge-only discovery (rows 1 and 3), over-invalidation (row 2), missing
input read as empty (row 4), history rewritten by an applying patch (row 5), silent rebinding of
active work (row 6), textual merge treated as semantic compatibility (row 7), delivered briefs
edited in place (row 8), a CLI that drifts from the package (row 9) and an architecture receipt
presented as product alignment (row 10). Recall beyond the fixture
corpus is not claimed; the fixture coverage limits go in `docs/context-handoff.md` (planned).

## Performance reuse amendment — 2026-10-16

Statusgen/26 supplies shared-recorder instrumentation and invocation-local parse reuse;
the resolver keeps its existing membership and receipt owner. Reuse only explicit authorized
source snapshots with qualified root/repository identity, parser/evaluator version, content
revisions and independently established complete membership. No credential or deskkit import
is introduced. A new consumer outside the previous read set, nested directory addition or
dirty source change forces the existing reconciliation result even when HEAD and every
previously read file are unchanged. A cached projection never authorizes dispatch.

Optional measurement hooks may report source enumeration, parsing and evaluation work through
the same library when available. This is not a new dependency for the pure resolver, not a
persistent parse-cache implementation, and does not expand its unfinished scope into the
future online mirror or a service.


## Evidence

<!-- Independent verifier records command, exit, named result, subject revision, environment and date. No implementation evidence is asserted by this authoring change. -->

### Verifier pass — 2026-10-21 at b9263b40b67a

Independent non-implementer verification of merged main `b9263b40b67aeb976099e337af74484acfa20451` (the merge delivering this brief). Every check:ci row ran network-off: a docker container from image assay-verify:go1.27-py (go1.27.2 linux/arm64), `--network none`, uid 65534, GOPROXY=off over a pre-fetched read-only module cache, GOTOOLCHAIN=local, in a full-history scratch clone at the verified sha. The witness table below is the `statusgen verifyrun --sandbox container-netns` output (statusgen built from the same sha), appended verbatim; the Expect/Observed table after it records each row's named result. Row 11's output hash is the empty-input digest because its command ends in `grep -q`.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextResolverWithheldConsumers$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextResolverWithheldConsumers " "$out")` | pass exit=0 sandbox=container-netns | sha256:576d288aed27 | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (on-behalf-of human:ian) (git-config) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextRecheckUnrelatedEdit$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextRecheckUnrelatedEdit " "$out")` | pass exit=0 sandbox=container-netns | sha256:00d08ae01f17 | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (on-behalf-of human:ian) (git-config) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextRecheckNewConsumerOutsideReadSet$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextRecheckNewConsumerOutsideReadSet " "$out")` | pass exit=0 sandbox=container-netns | sha256:fff49ef38745 | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (on-behalf-of human:ian) (git-config) |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextMissingInputScoped$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextMissingInputScoped " "$out")` | pass exit=0 sandbox=container-netns | sha256:1e2ae5e72094 | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (on-behalf-of human:ian) (git-config) |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextAmendmentPreservesDatedSources$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextAmendmentPreservesDatedSources " "$out")` | pass exit=0 sandbox=container-netns | sha256:652663c80655 | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (on-behalf-of human:ian) (git-config) |
| 6 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextActiveWorkerRefusesSilentRebind$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextActiveWorkerRefusesSilentRebind " "$out")` | pass exit=0 sandbox=container-netns | sha256:81bab8ab50bb | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (on-behalf-of human:ian) (git-config) |
| 7 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextCompetingProposals$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextCompetingProposals " "$out")` | pass exit=0 sandbox=container-netns | sha256:fef75297db88 | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (on-behalf-of human:ian) (git-config) |
| 8 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextDeliveredBriefRoutesFollowOn$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextDeliveredBriefRoutesFollowOn " "$out")` | pass exit=0 sandbox=container-netns | sha256:f324b57b2f16 | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (on-behalf-of human:ian) (git-config) |
| 9 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextCLIDiscoverRoundTrip$" . > "$out" && grep -F -- "--- PASS: TestContextCLIDiscoverRoundTrip " "$out")` | pass exit=0 sandbox=container-netns | sha256:1d2bf5ed2a86 | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (on-behalf-of human:ian) (git-config) |
| 10 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextArchitectureCoverageNotProductAlignment$" . > "$check_output" && grep -F -- "--- PASS: TestContextArchitectureCoverageNotProductAlignment " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:29466755e986 | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (on-behalf-of human:ian) (git-config) |
| 11 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPickupContextRecheck$' . > "$out" && grep -q '^--- PASS: TestPickupContextRecheck ' "$out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (on-behalf-of human:ian) (git-config) |

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextResolverWithheldConsumers$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextResolverWithheldConsumers " "$out")` | exit 0; named PASS; withheld synthetic owners and consumers (including a prose-only consumer with no typed edge) are discovered and dispositioned; mutation: discover from typed edges only — the test must fail | exit 0; `--- PASS: TestContextResolverWithheldConsumers (0.01s)`; mutation (only rank-0 typed-edge candidates offered in discover) killed: test FAIL, prose-only consumer, delivered-brief routing and history not found | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextRecheckUnrelatedEdit$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextRecheckUnrelatedEdit " "$out")` | exit 0; named PASS; same inputs reproduce the receipt digest, an unrelated edit preserves unaffected conclusions; mutation: invalidate every conclusion on any edit — the test must fail | exit 0; `--- PASS: TestContextRecheckUnrelatedEdit (0.00s)`; mutation (every unrelated file edit reconciles and touches every impact) killed: FAIL, unrelated edit forced reconciliation | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextRecheckNewConsumerOutsideReadSet$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextRecheckNewConsumerOutsideReadSet " "$out")` | exit 0; named PASS; a new consumer outside the prior read set forces reconciliation although every previously read file hash is unchanged; mutation: recheck only previously read file hashes — the test must fail | exit 0; `--- PASS: TestContextRecheckNewConsumerOutsideReadSet (0.00s)`; mutation (new subjects, new prose candidates and dropped subjects ignored, only prior read-set hashes compared) killed: FAIL, findings empty, want a reconcile for the new consumer | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextMissingInputScoped$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextMissingInputScoped " "$out")` | exit 0; named PASS; an unavailable input yields scoped could-not-check, unrelated authorized work continues, and output carries no private names or counts; mutation: report the unavailable source as empty — the test must fail | exit 0; `--- PASS: TestContextMissingInputScoped (0.00s)`; mutation (an unreadable source loaded as available with no files) killed: FAIL, coverage reported complete, want the source unavailable | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextAmendmentPreservesDatedSources$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextAmendmentPreservesDatedSources " "$out")` | exit 0; named PASS; a cleanly applying patch that rewrites a populated Evidence row or a dated source account in an unfinished brief fails, and the additive amendment passes; mutation: check only that the patch applies — the test must fail | exit 0; `--- PASS: TestContextAmendmentPreservesDatedSources (0.01s)`; mutation (history-rewrite check disabled, apply-only) killed: FAIL, verdict pass, want refuse history-rewrite | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextActiveWorkerRefusesSilentRebind$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextActiveWorkerRefusesSilentRebind " "$out")` | exit 0; named PASS; with a fixture claim at an older brief revision the amendment is refused until an acknowledged adjustment or cancellation is recorded; mutation: ignore claims — the test must fail | exit 0; `--- PASS: TestContextActiveWorkerRefusesSilentRebind (0.01s)`; mutation (claims ignored) killed: FAIL, unacknowledged claim verdict pass, want refuse silent-rebind | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextCompetingProposals$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextCompetingProposals " "$out")` | exit 0; named PASS; a clean merge of two incompatible approved deltas is refused, and an unapproved rival does not block the approved one; mutation: accept any textually clean merge — the test must fail | exit 0; `--- PASS: TestContextCompetingProposals (0.01s)`; mutation (approved-rival conflict check disabled) killed: FAIL, incompatible approved rival verdict pass, want refuse incompatible-delta | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextDeliveredBriefRoutesFollowOn$" ./contexthandoff/ > "$out" && grep -F -- "--- PASS: TestContextDeliveredBriefRoutesFollowOn " "$out")` | exit 0; named PASS; a new obligation on a delivered brief with populated Evidence routes to an unfinished owner or linked follow-on and leaves the delivered brief unchanged; mutation: patch the delivered brief's Task — the test must fail | exit 0; `--- PASS: TestContextDeliveredBriefRoutesFollowOn (0.01s)`; mutation (delivered-brief guard removed so the patch applies to it) killed: FAIL, verdict pass, want refuse amend-unbound naming the linked follow-on | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextCLIDiscoverRoundTrip$" . > "$out" && grep -F -- "--- PASS: TestContextCLIDiscoverRoundTrip " "$out")` | exit 0; named PASS; the built `statusgen context discover --snapshot <fixture> --json` output decodes with the statusgen/19 package and `context check` on its own receipt passes | exit 0; `--- PASS: TestContextCLIDiscoverRoundTrip (2.71s)` (built binary, discover JSON decoded, check on its own receipt passes) | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextArchitectureCoverageNotProductAlignment$" . > "$check_output" && grep -F -- "--- PASS: TestContextArchitectureCoverageNotProductAlignment " "$check_output")` | exit 0; named PASS; architecture discovery through CLI explicitly leaves unsupported product alignment unchecked; mutation: label a receipt covering architecture only as aligned to the product target | exit 0; `--- PASS: TestContextArchitectureCoverageNotProductAlignment (1.12s)`; mutation (coverage-scope product-alignment parameter set to aligned) killed: FAIL, want architecture with product alignment unchecked | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 11 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPickupContextRecheck$' . > "$out" && grep -q '^--- PASS: TestPickupContextRecheck ' "$out")` | exit 0; snapshot fixtures distinguish approved owner/guideline changes, new consumer outside old membership, unrelated edit, unavailable required input and draft-only rival; mutation: ignore a new guideline/replacement source and require the corresponding case to fail | exit 0 (grep -q matched the PASS line); mutation A (guideline comparison removed) killed: FAIL, guideline added/changed read as preserved; mutation B (declared owner/version/replacement edge comparison removed) killed: FAIL, want reconcile for owner, producer version or planned replacement change | 2026-10-21 | assay-verifier-app[bot] @ b9263b40b67a (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Mutation controls: each mutant was a one-function edit in its own scratch worktree (never in this checkout), and each row's exact command was re-run against it in the same offline container. All eleven mutants turned their row red (exit 1, no PASS line), and a diagnostic re-run without the grep showed assertion failures, not build errors. Row 9 is +flow only and carries no mutation obligation. Supplemental check: production code under statusgen/contexthandoff and statusgen/context.go names none of the fixture repositories, streams or brief slugs, so expected answers are withheld from discovery.

Risk-bearing value. The risk answers are all present and all `no`, and the item is not irreversible, so the trigger does not fire. Enumeration over the delivered resolver (statusgen/contexthandoff non-test files and statusgen/context.go) found: `DefaultLimits = {MaxFiles: 20000, MaxFileBytes: 1 << 20, MaxTotalBytes: 256 << 20, MaxResults: 2000}` @ statusgen/contexthandoff/snapshot.go:88; `revisionRe = ^[0-9a-f]{7,64}$` @ statusgen/contexthandoff/decode.go:68; `BlobRevision = sha1("blob <len>\0" + raw)` @ statusgen/contexthandoff/plan.go:306; `isoDateRe = \b[0-9]{4}-[0-9]{2}-[0-9]{2}\b` @ statusgen/contexthandoff/plan.go:26 with `longDateRe` @ statusgen/contexthandoff/plan.go:27 (dated-source detection); `coverageScopeResult = "architecture"` @ statusgen/contexthandoff/discover.go:25; `"product-alignment": "unchecked"` @ statusgen/contexthandoff/discover.go:317. Ranked: all are reversible by an edit and a re-run (the resolver is offline and writes nothing outside its receipt); the history and coverage literals rank first because a wrong value silently passes a rewrite or a partial read.

RISK-VALUE: DERIVED — BlobRevision = sha1("blob <len>\0" + raw) @ statusgen/contexthandoff/plan.go:306 — this is git's blob object id for a SHA-1 repository; recomputed for this brief file at the verified sha it equals `git hash-object` exactly, so read-set and patch-base revisions are the same ids git reports.
RISK-VALUE: DERIVED — revisionRe = ^[0-9a-f]{7,64}$ @ statusgen/contexthandoff/decode.go:68 — 7 is git's default minimum abbreviated object id and 64 is the hex length of a SHA-256 object id, so the pattern admits every git revision spelling from shortest abbreviation to a full SHA-256 id and nothing else.
RISK-VALUE: DERIVED — "product-alignment": "unchecked" @ statusgen/contexthandoff/discover.go:317 — the brief's 13 October amendment requires that no product binding reads as unchecked, never aligned; product binding is owned by statusgen/22 and statusgen/23, so unchecked is the only correct value here.
RISK-VALUE: NAMED, NOT DERIVED — DefaultLimits.MaxResults = 2000 @ statusgen/contexthandoff/snapshot.go:88 — a reversible operational cap with no spec figure to derive from; exceeding it is reported as truncated (partial coverage and a hold), never as empty, so a wrong value costs coverage, not correctness. Question filed: #4127 (derivation or owner confirmation of the cap).

Verification-Attestation: medici-finance/assay#3997 run=1937312b1dfa2adeec7075f759dc03168f37dda4ab9e220d11654cab26d0c8c3 source=b9263b40b67aeb976099e337af74484acfa20451 brief=docs/streams/statusgen/brief-20-offline-context-resolver.md model=claude-opus-5-5 tier=strong

**VERIFY: PASS** — 11 of 11 rows pass at b9263b40b67a, offline; all eleven mutation controls are killed.

## Review

Gate: model (all four risk answers no). The reviewer confirms the expected answers are withheld
from production discovery and that every refusal has a failing control.
