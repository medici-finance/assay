---
brief: assay:assay:statusgen:20
title: Offline future-state resolver with receipt and amendment validation
why: "An author can only amend the briefs they find. A reproducible, offline resolver that enumerates owners and consumers, records what it could not see, and rechecks a patch against its base turns 'did we miss a consumer or overwrite history?' into a check instead of a reviewer's memory."
wave: 2
depends: ["statusgen/19"]
unblocks: ["statusgen/21"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-13 by worker-desk (author-brief), non-CI architecture reconciliation
sources:
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

files: `statusgen/contexthandoff/` (planned) (resolve, recheck and amend files plus tests; the package
exists after statusgen/19), `statusgen/context.go` (planned) (new CLI adapter), `statusgen/main.go`
(subcommand dispatch), `statusgen/testdata/contexthandoff/journeys/` (planned),
`docs/context-handoff.md` (planned), `changelog/statusgen-20-offline-context-resolver.md` (planned).

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

- Isolated branch; draft PR only. No merge or release.
- Neutral synthetic fixtures (two example repositories, withheld consumers). Do not hard-code the
  expected answer list into production discovery; tests withhold expected identities from the
  resolver input.
- Stop at implemented; independent verification owns verified/done.

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

## Pre-mortem

Likely wrong builds: typed-edge-only discovery (rows 1 and 3), over-invalidation (row 2), missing
input read as empty (row 4), history rewritten by an applying patch (row 5), silent rebinding of
active work (row 6), textual merge treated as semantic compatibility (row 7), delivered briefs
edited in place (row 8) and a CLI that drifts from the package (row 9). Recall beyond the fixture
corpus is not claimed; the fixture coverage limits go in `docs/context-handoff.md` (planned).

## Evidence

<!-- Independent verifier records command, exit, named result, subject revision, environment and date. No implementation evidence is asserted by this authoring change. -->

## Review

Gate: model (all four risk answers no). The reviewer confirms the expected answers are withheld
from production discovery and that every refusal has a failing control.
