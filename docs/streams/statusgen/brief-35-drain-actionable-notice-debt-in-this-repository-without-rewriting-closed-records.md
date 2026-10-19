---
brief: assay:assay:statusgen:35
title: "Drain actionable notice debt in this repository without rewriting closed records"
why: "Repair tools do not remove the existing backlog until someone applies and reviews their output. One bounded corpus cleanup proves that the tools reduce real debt without changing old evidence."
wave: 3
depends: ["statusgen/27", "statusgen/28", "statusgen/29", "statusgen/30", "statusgen/31", "statusgen/32"]
unblocks: ["statusgen/33"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: "2026-10-17 by the maintainer-directed lint-debt refresh"
sources:
  - "docs/streams/statusgen/README.md"
  - "freshness-checked 2026-10-17 @ 8f1206fe5eb4b4ab19f5f2f7266c515aa71f8eae"
exec-tier: strong
exec-tier-why: "Cross-artifact classification, rewrite semantics and consumer verification must agree."
domain: complicated
outcome: none
version: 1
id: 3f2aaf9c-d1b0-4d3e-9771-92bc874aed2a
consumers:
  - "docs/streams/*/brief-*.md: follow-up statusgen/35 (this brief; change is owed at implementation)"
  - "docs/streams/statusgen/active-notice-cleanup.md: follow-up statusgen/35 (this brief; change is owed at implementation)"
  - "changelog/active-notice-cleanup.md: follow-up statusgen/35 (this brief; change is owed at implementation)"
---

# Brief 35 — Drain actionable notice debt in this repository without rewriting closed records

## Context

files: docs/streams/*/brief-*.md, docs/streams/statusgen/active-notice-cleanup.md, changelog/active-notice-cleanup.md
facts:
- Rule membership, exclusions and derivable/author-owed dispositions are defined by the [public rule-family inventory](README.md#public-rule-family-inventory); use that shared inventory for this brief's Task and Verify acceptance.
- Use the released/pinned binary containing statusgen/27 through statusgen/32, never an unrelated local build for before/after comparisons.
- Only active brief Task/Context/Verify declarations may change; never alter Expect thresholds to obtain green.
- No same-identity implementation can serve as independent verification of its own rewritten commands.

design-fit:
  owner: statusgen
  contract: none — extend existing reporting, reconciliation and pin contracts; no new semantic owner.
  retires: ["the replaced per-site notice behavior or unresolved corpus debt named in Task"]
  weight: "No additional daemon or independent rule table. Exact flags and production code delta are measured by the implementation PR."
  why-add: "Reuse the existing owner; any new CLI entry point is limited to the reviewed repair/reporting operation."

layering: Keep parsing/classification pure and reuse the existing command owner for filesystem, git and forge effects. Fixtures exercise the boundary without live infrastructure.

single-point-of-failure: A mislabeled finding could evade the authoring gate; generator refusal, differential comparison and closure/witness checks are exercised independently.

## Ground rules

- Use an isolated feature branch and a draft PR; merge and workflow activation keep their existing human gates.
- Do not rewrite recorded Evidence or frozen closed commands. Do not query clusters or production endpoints.
- Stop at implementation; an independent verifier records the execution evidence.

## Task

1. Capture binary version and digest, exact base revision, per-rule/per-status counts and closed-record hashes before editing.
2. Apply both repair tools in dry run, review the proposed changes, and apply only semantics-preserving active changes. Repair remaining active author-owed defects with explicit evidence; do not fabricate names or execute out-of-scope live infrastructure.
3. Record before/after measurements from the same binary and original immutable base. Require zero active Family A and derivable Family B findings; every genuinely judgment-dependent residual needs a named owner and tracked disposition, not a suppression.
4. Have an independent verifier run the changed commands, retain original intent and prove that a second planted defect is still rejected by the ratchet. Publish a reviewable cleanup report; keep generated single-writer views out of the branch.

## Verify

Commands run from this repository root. Named new test contracts below are planned deliverables; they must be added by the implementer and the PASS assertion must fail if the selector finds nothing.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && go test ./... -run "^TestNoticeRatchet$" -count=1 -v > "${TMPDIR:-/tmp}/notice-35.log" 2>&1 && grep -F -e "--- PASS: TestNoticeRatchet " "${TMPDIR:-/tmp}/notice-35.log"` | exit 0; output contains the named PASS line. Matrix: the installed ratchet still rejects a second newly introduced bad row after cleanup | check:ci +mutation +flow +dereference |
| 2 | `cd statusgen && go test ./... -count=1` | exit 0; existing closure, witness and differential tests still pass | check:ci +neighbour |
| 3 | `statusgen --root . --lint --json > "${TMPDIR:-/tmp}/active-notices.json"` | exit 0; JSON report proves zero active Family A and derivable Family B findings; residuals match the reviewed report | check +dereference |
| 4 | `git diff --check` | exit 0; output is empty | check:ci |
| 5 | `git diff -- docs/streams/statusgen/active-notice-cleanup.md` | exit 0; reviewer dereferences recorded binary digest, baseline, counts and closed-record hashes against the run artifacts | gate:model +dereference |
| 6 | `statusgen --root . --consumers` | exit 0; the implementation diff corroborates the routing commitments | check +flow |

## Evidence

<!-- Independent verification records command, exit/output, revision, binary digest, date and runner here. This authoring PR executes no future implementation check. -->

## Review

Gate: model. Review the fixture matrix and frozen-history boundary, not just the presence of new tests.

## Failure map

- A selector runs zero tests: the named PASS assertion in the test row fails.
- Historical data is changed to reduce the count: the required pre/post hash comparison and independent review fail.
- Only the first reported example is repaired: the matrix plants a second defect at another site.
- Source ships but consumer stays stale: adoption briefs require the published version and digest receipt.
