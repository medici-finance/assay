---
brief: assay:assay:statusgen:27
title: "Reject newly introduced actionable lint notices on changed briefs"
why: "New verification defects currently merge with a successful lint result and replenish every cleanup. A differential gate must stop new debt without failing unrelated changes on legacy debt."
wave: 1
depends: []
unblocks: ["statusgen/29", "statusgen/35"]
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
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
id: 5b2f5c79-4a30-40b7-9467-b9168b65e198
gate-why: "Changes tooling behavior relied on by adopters. Human sign-off confirms the new authoring or repair behavior preserves historical evidence and the existing closure gates."
decision-trigger: start
consumers:
  - "statusgen/difflint.go: follow-up statusgen/27 (this brief; change is owed at implementation)"
  - "statusgen/difflint_test.go: follow-up statusgen/27 (this brief; change is owed at implementation)"
  - "statusgen/main.go: follow-up statusgen/27 (this brief; change is owed at implementation)"
  - "docs/verify-row-strength.md: follow-up statusgen/27 (this brief; change is owed at implementation)"
  - "changelog/notice-ratchet.md: follow-up statusgen/27 (this brief; change is owed at implementation)"
---

# Brief 27 — Reject newly introduced actionable lint notices on changed briefs

## Context

files: statusgen/difflint.go, statusgen/difflint_test.go, statusgen/noticeratchet.go, statusgen/noticeratchet_test.go, statusgen/main.go, docs/verify-row-strength.md, changelog/notice-ratchet.md
facts:
- Rule membership, exclusions and derivable/author-owed dispositions are defined by the [public rule-family inventory](README.md#public-rule-family-inventory); use that shared inventory for this brief's Task and Verify acceptance.
- runDiffLintOne currently compares head PROBLEMs with base PROBLEMs and passes head NOTICEs through unchanged.
- Existing closure gates in unfailableRowAudit must remain effective; a no-new-notices gate is an additional authoring boundary.
- The base and head must be evaluated by the same binary, against immutable merge-base and head revisions.

design-fit:
  owner: statusgen
  contract: none — extend existing reporting, reconciliation and pin contracts; no new semantic owner.
  retires: ["the replaced per-site notice behavior or unresolved corpus debt named in Task"]
  weight: "No additional daemon or independent rule table. Exact flags and production code delta are measured by the implementation PR."
  why-add: "Reuse the existing owner; any new CLI entry point is limited to the reviewed repair/reporting operation."

layering: Keep parsing/classification pure and reuse the existing command owner for filesystem, git and forge effects. Fixtures exercise the boundary without live infrastructure.

single-point-of-failure: A mislabeled finding could evade the authoring gate; generator refusal, differential comparison and closure/witness checks are exercised independently.

## Human decision

Confirm the implemented behavior rejects new actionable defects while preserving historical evidence, existing closure checks and explicit unknown states. Review the concrete implementation and its independent verification before approval.

Options:
1. Approve the implementation as verified.
2. Request the identified corrections before approval.
3. Hold the change.

Default if no answer: none — blocks until answered.

Hold the change; no sign-off is inferred.

## Ground rules

- Use an isolated feature branch and a draft PR; merge and workflow activation keep their existing human gates.
- Do not rewrite recorded Evidence or frozen closed commands. Do not query clusters or production endpoints.
- Stop at implementation; an independent verifier records the execution evidence.

## Task

1. Extend the existing differential, rather than add another lint command. Compare normalized rule, repository-relative path and row identity; explanatory wording and temporary absolute roots must not create a new finding.
2. Promote newly introduced actionable findings on added or edited briefs to PROBLEM. Begin with the existing structural Verify-row rules, identifier dereference, risk/files and routing metadata. Classify historical summaries, environment/eligibility unknowns, operational debt counters and informational lines explicitly; they remain visible and cannot become authoring failures accidentally.
3. Preserve base-equals-head and unavailable-base safety: do not claim a no-new-notices pass when comparison was unavailable. Retain existing full-strength PROBLEMs and an explicit could-not-check notice.
4. Keep all existing closure and witness gates. Pin structured comparison before render; if a temporary parser is needed, centralize it and cover every included emitter in the test matrix.

## Verify

Commands run from this repository root. Named new test contracts below are planned deliverables; they must be added by the implementer and the PASS assertion must fail if the selector finds nothing.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && go test ./... -run "^TestNoticeRatchet$" -count=1 -v > "${TMPDIR:-/tmp}/notice-27.log" 2>&1 && grep -F -e "--- PASS: TestNoticeRatchet " "${TMPDIR:-/tmp}/notice-27.log"` | exit 0; output contains the named PASS line. Matrix: unchanged legacy debt passes; an added and an edited brief each introduce a second defect and fail; unrelated-file changes pass; moved absolute roots do not inflate debt; historical summaries and missing siblings remain visible; unavailable base cannot print a clean comparison | check:ci +mutation +flow +dereference |
| 2 | `cd statusgen && go test ./... -count=1` | exit 0; existing closure, witness and differential tests still pass | check:ci +neighbour |
| 3 | `statusgen --root . --consumers` | exit 0; the implementation diff corroborates the routing commitments | check +flow |

## Evidence

<!-- Independent verification records command, exit/output, revision, binary digest, date and runner here. This authoring PR executes no future implementation check. -->

## Review

Gate: human. Review the fixture matrix and frozen-history boundary, not just the presence of new tests.

## Failure map

- A selector runs zero tests: the named PASS assertion in the test row fails.
- Historical data is changed to reduce the count: the required pre/post hash comparison and independent review fail.
- Only the first reported example is repaired: the matrix plants a second defect at another site.
- Source ships but consumer stays stale: adoption briefs require the published version and digest receipt.
