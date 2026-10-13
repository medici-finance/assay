---
brief: assay:assay:statusgen:23
title: Product target binding and context drift assessment
why: "Architecture-only context cannot detect a brief serving an obsolete product target. Bind both kinds of target and route mismatches through the existing reconciliation owner."
wave: 3
depends: ["statusgen/22", "statusgen/20"]
unblocks: ["statusgen/25", "statusgen/21"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-13 by statusgen newbrief
sources:
  - docs/proposals/non-ci-architecture/21-product-intent-and-portfolio.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - freshness-checked 2026-10-13 @ d484dd5df24f2aa8d557dafce41687b38173f227
exec-tier: strong
exec-tier-why: Cross-contract identity, compatibility and evidence semantics require design and consumer reasoning.
domain: complicated
outcome: none
id: cc77a185-fe5b-42a4-9476-6d4ee4f7c0f8
consumers:
  - "spec/brief-v1.md, brief schemas, statusgen/brieffile.go, statusgen/contexthandoff: follow-up statusgen/23"
  - "plugins/assay/skills/author-spec, plugins/assay/skills/author-brief: follow-up statusgen/21"
  - "statusgen/portfolio: follow-up statusgen/25"
---

# Brief 23 — Product target binding and context drift assessment

## Context

files: `spec/brief-v1.md`, `spec/context-handoff-v1.md` (planned), `schemas/context-handoff-v1.json`, `schemas/amendment-manifest-v1.json`, brief schemas/readers, `statusgen/contexthandoff/`, `statusgen/brieffile.go`, `docs/context-handoff.md` (planned), `docs/contracts.md`; `changelog/statusgen-23-portfolio.md` (planned).

facts (d484dd5df, 13 October 2026):
- Statusgen/19 owns the initial handoff contract and has an active claim; /20 owns discovery/recheck.
- outcome accepts an existing REQ reference or none; appending @revision is not its grammar.
- The optional binding is not shipped at the source pin; this is an additive successor, not a rewrite of /19.

layering: pure typed records/evaluators in `statusgen/contexthandoff`; filesystem and CLI adapters remain outside the package. No network, credentials, scheduler or runtime database. The flow and boundary fixtures below enforce this separation.

single-point-of-failure: an incorrect record interpretation could create a false alignment or completion claim; schema/record validation and independently supplied source/approval/coverage fixtures exercise different failure inputs, while existing admission and publication controls remain outside this read-only capability.

design-fit:
  owner: statusgen/contexthandoff
  contract: product-target binding extension of context-handoff-v1 (planned extension; register its semantic owner in docs/contracts.md)
  retires: []
  weight: "one bounded contract/evaluation capability; no new service, scheduler or dispatch authority"
  why-add: "Existing records do not carry this meaning; extend their owning contracts rather than adding an independent product-state system."

## Read first

- `docs/proposals/non-ci-architecture/21-product-intent-and-portfolio.md` — exact record semantics, boundaries and owner mapping.
- `spec/registers-v1.md`, `spec/brief-v1.md` and the declared dependency briefs.
- `docs/streams/continuing-operations/spec.md` for operational identity and impact ownership.

## Ground rules

- Isolated branch and draft PR; no merge, release or activation.
- Synthetic offline inputs only; no collection from real transcripts or production systems.
- Preserve existing records and populated Evidence; stop at implemented for independent verification.
- Named tests and planned interfaces below must be delivered; they are not current commands claimed to pass.

## Task

1. Add optional outcome-revision qualified by outcome and separate from the architecture baseline. Publish a versioned compatible handoff extension after inspecting /19 and /20; readers reject unsupported mandatory semantics rather than silently dropping them. Preserve legacy unbound briefs.
2. Extend existing discovery/recheck records with declared product bindings, exact mismatch evaluation, coverage and materiality/disposition inputs. Reuse one reconciliation engine. Preserve original selection context and append authorized continue/adapt/stop records with expected revision and rationale; refuse stale concurrent disposition.
3. Expose unapproved rivals, unavailable approvals and undiscovered secondary relationships distinctly. Compatible work may continue under recorded disposition; no automatic cancellation or status change. Extend CLI output through /20’s adapter and publish consumer migration/version tests.

## Interface contract

Input: qualified product target from /22, architecture baseline and /20 snapshot/receipt. Output: separate bindings, mismatch/coverage results and revision-bound amendment/disposition obligations. stale-context is a projection, not a lifecycle status or execution veto.

## Verify

Run from the repository root after implementation. Each named test asserts the stated
behavior and includes the specified failing mutation. A missing test/package or absent
named PASS fails the row; do not substitute a skipped or zero-test run.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductBindingIndependentTargets$" ./contexthandoff/ > "$check_output" && grep -F -- "--- PASS: TestProductBindingIndependentTargets " "$check_output")` | exit 0; named PASS; product target change preserves applicable architecture baseline and original binding; mutation: use one revision field for both |
| 2 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductBindingApprovalCoverage$" ./contexthandoff/ > "$check_output" && grep -F -- "--- PASS: TestProductBindingApprovalCoverage " "$check_output")` | exit 0; named PASS; unapproved rival does not stale approved work; missing binding/secondary relationship is coverage unknown; mutation: treat absent binding as aligned |
| 3 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductBindingCheckpointFlow$" . > "$check_output" && grep -F -- "--- PASS: TestProductBindingCheckpointFlow " "$check_output")` | exit 0; named PASS; real brief reader through resolver/CLI and disposition round trip retains historical evidence and rejects stale rebinding; mutation: replace original selection revision |

## Evidence

<!-- Independent verifier records subject revision, command, exit, named result and environment. No implementation or execution evidence is asserted here. -->

## Review

Gate: model; all risk answers no because this slice processes synthetic/explicit offline
snapshots and makes no live policy, publication or deployment change. Review semantic
compatibility and the real consumer flow; later live integration derives its own risk gate.
