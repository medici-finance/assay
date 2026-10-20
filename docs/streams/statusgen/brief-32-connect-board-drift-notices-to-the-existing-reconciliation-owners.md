---
brief: assay:assay:statusgen:32
title: "Connect board drift notices to the existing reconciliation owners"
why: "A notice about a non-dispatchable or already-delivered row should lead to the existing state transition. Printing the same diagnostic forever wastes dispatch and verification capacity."
wave: 1
depends: []
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
id: 88cd2027-42b9-4227-b6ba-ecc9a23c13d5
gate-why: "Changes tooling behavior relied on by adopters. Human sign-off confirms the new authoring or repair behavior preserves historical evidence and the existing closure gates."
decision-trigger: start
consumers:
  - "statusgen/boardhonesty.go: follow-up statusgen/32 (this brief; change is owed at implementation)"
  - "statusgen/boardhonesty_test.go: follow-up statusgen/32 (this brief; change is owed at implementation)"
  - "statusgen/mergedstatus.go: follow-up statusgen/32 (this brief; change is owed at implementation)"
  - "statusgen/registerlinks.go: follow-up statusgen/32 (this brief; change is owed at implementation)"
  - "statusgen/verifiedrunneragree.go: follow-up statusgen/32 (this brief; change is owed at implementation)"
  - "tools/desk/cmd/deskevidence/flip.go: follow-up statusgen/32 (this brief; change is owed at implementation)"
  - "changelog/notice-loop-closure.md: follow-up statusgen/32 (this brief; change is owed at implementation)"
---

# Brief 32 — Connect board drift notices to the existing reconciliation owners

## Context

files: statusgen/boardhonesty.go, statusgen/boardhonesty_test.go, statusgen/mergedstatus.go, statusgen/registerlinks.go, statusgen/verifiedrunneragree.go, tools/desk/cmd/deskevidence/flip.go, changelog/notice-loop-closure.md
facts:
- Rule membership, exclusions and derivable/author-owed dispositions are defined by the [public rule-family inventory](README.md#public-rule-family-inventory); use that shared inventory for this brief's Task and Verify acceptance.
- homed-in already exists; do not add another dehousing field.
- deskevidence already has a flip implementation; inspect and extend it rather than implement a duplicate Evidence writer.
- Lifecycle boards are derived from corroborated events, not hand-edited status cells.

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

1. Measure each remaining Family C class against current code. A discussion of a foreign-ownership classification in a brief is not itself an ownership declaration; include a negative-control fixture for that false positive. Reuse the existing foreign-owned/already-delivered classifier in eligibility and derived board reconciliation; do not create a separate dispatch allowlist.
2. Keep missing forge state could-not-check. A merged implementation trailer may feed the existing reconciliation engine; an Authors trailer must never count as delivery.
3. Ensure the existing Evidence/flip path derives Verified actor from the accepted run record. Keep old runner-attribution notices as historical records where correction would rewrite closed evidence.
4. Extend the existing register-link rewrite only where a generated-view reference maps uniquely to its durable entry. List ambiguous links; do not invent targets. Remove superseded notices only after their owner actually consumes the condition.

## Verify

Commands run from this repository root. Named new test contracts below are planned deliverables; they must be added by the implementer and the PASS assertion must fail if the selector finds nothing.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && go test ./... -run "^TestNoticeLoopClosure$" -count=1 -v > "${TMPDIR:-/tmp}/notice-32.log" 2>&1 && grep -F -e "--- PASS: TestNoticeLoopClosure " "${TMPDIR:-/tmp}/notice-32.log"` | exit 0; output contains the named PASS line. Matrix: foreign-owned/already-delivered rows cannot dispatch; authoring-only PR does not advance status; absent forge state remains unknown; Verified actor follows accepted run; unique register link repairs once; ambiguous link does not change | check:ci +mutation +flow +dereference |
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
