---
brief: assay:assay:statusgen:29
title: "Run the existing structural row checks before writing a new brief"
why: "The authoring generator can still write rows that the linter later calls broken. Refusing these rows before any file is written prevents repair work at its cheapest point."
wave: 2
depends: ["statusgen/27"]
unblocks: ["statusgen/35"]
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
id: 11564430-4dfa-4343-8c70-057c17b3eb1d
gate-why: "Changes tooling behavior relied on by adopters. Human sign-off confirms the new authoring or repair behavior preserves historical evidence and the existing closure gates."
decision-trigger: start
consumers:
  - "statusgen/newbrief.go: follow-up statusgen/29 (this brief; change is owed at implementation)"
  - "statusgen/newbrief_test.go: follow-up statusgen/29 (this brief; change is owed at implementation)"
  - "statusgen/verifyrows.go: follow-up statusgen/29 (this brief; change is owed at implementation)"
  - "plugins/assay/skills/author-brief/SKILL.md: follow-up statusgen/29 (this brief; change is owed at implementation)"
  - "plugins/assay/skills/worker-desk/SKILL.md: follow-up statusgen/29 (this brief; change is owed at implementation)"
  - "changelog/newbrief-row-refusal.md: follow-up statusgen/29 (this brief; change is owed at implementation)"
---

# Brief 29 — Run the existing structural row checks before writing a new brief

## Context

files: statusgen/newbrief.go, statusgen/newbrief_test.go, statusgen/verifyrows.go, plugins/assay/skills/author-brief/SKILL.md, plugins/assay/skills/worker-desk/SKILL.md, changelog/newbrief-row-refusal.md
facts:
- Rule membership, exclusions and derivable/author-owed dispositions are defined by the [public rule-family inventory](README.md#public-rule-family-inventory); use that shared inventory for this brief's Task and Verify acceptance.
- newbrief already derives risk, gate, wave and inverse edges, refuses placeholders/unusable commands, and writes atomically.
- rowFindingsCtx is the canonical structural row evaluator; reuse it instead of copying its regular expressions.
- The differential remains necessary for hand-authored briefs which bypass the generator.

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

Default if no answer: hold; no sign-off is inferred.

## Ground rules

- Use an isolated feature branch and a draft PR; merge and workflow activation keep their existing human gates.
- Do not rewrite recorded Evidence or frozen closed commands. Do not query clusters or production endpoints.
- Stop at implementation; an independent verifier records the execution evidence.

## Task

1. Apply all applicable structural row checks and identifier checks to the rendered candidate before the first write. Resolve explicitly planned names without treating a planned name as evidence that a test exists.
2. On refusal, name the rule and remedy and leave the brief, inverse dependencies and generated board byte-identical. Preserve supported sh, cmd and pwsh semantics; do not run commands or require undeclared operating systems during authoring.
3. Update the two skill pointers to the actual supported differential invocation. Preserve the generator, PR ratchet and closure gate as separately exercised layers.

## Verify

Commands run from this repository root. Named new test contracts below are planned deliverables; they must be added by the implementer and the PASS assertion must fail if the selector finds nothing.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && go test ./... -run "^TestNewBriefRowRefusal$" -count=1 -v > "${TMPDIR:-/tmp}/notice-29.log" 2>&1 && grep -F -e "--- PASS: TestNewBriefRowRefusal " "${TMPDIR:-/tmp}/notice-29.log"` | exit 0; output contains the named PASS line. Matrix: one rejected fixture per structural rule; planned and unplanned names; no partial writes to dependencies or README; valid portable commands pass; a hand-authored second bad brief bypasses the generator and is still caught by the PR ratchet | check:ci +mutation +flow +dereference |
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
