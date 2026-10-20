---
brief: assay:assay:statusgen:30
title: "Repair deterministic defects in active Verify rows with a reviewed dry run"
why: "Repeated mechanical command defects are repairable, but manual edits across the corpus are slow and can change the claimed test. A conservative repair tool should remove only the defects it can prove it understands."
wave: 2
depends: ["statusgen/28"]
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
id: b5d51203-db48-4860-9b8f-f727c5bb8727
gate-why: "Changes tooling behavior relied on by adopters. Human sign-off confirms the new authoring or repair behavior preserves historical evidence and the existing closure gates."
decision-trigger: start
consumers:
  - "statusgen/fixrows.go: follow-up statusgen/30 (this brief; change is owed at implementation)"
  - "statusgen/fixrows_test.go: follow-up statusgen/30 (this brief; change is owed at implementation)"
  - "statusgen/main.go: follow-up statusgen/30 (this brief; change is owed at implementation)"
  - "statusgen/verifyrows.go: follow-up statusgen/30 (this brief; change is owed at implementation)"
  - "docs/verify-row-strength.md: follow-up statusgen/30 (this brief; change is owed at implementation)"
  - "changelog/fix-rows.md: follow-up statusgen/30 (this brief; change is owed at implementation)"
---

# Brief 30 — Repair deterministic defects in active Verify rows with a reviewed dry run

## Context

files: statusgen/fixrows.go, statusgen/fixrows_test.go, statusgen/main.go, statusgen/verifyrows.go, docs/verify-row-strength.md, changelog/fix-rows.md
facts:
- Rule membership, exclusions and derivable/author-owed dispositions are defined by the [public rule-family inventory](README.md#public-rule-family-inventory); use that shared inventory for this brief's Task and Verify acceptance.
- No fix-rows command exists at the checked source revision.
- Markdown escaping and shell quoting are distinct; changing a regex flag alone can invert a check.
- Recorded Evidence, Expect cells and closed brief commands are outside the rewriter write set.

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

1. Add fix-rows, dry-run by default, --write only on an explicit selected active corpus. Return a deterministic per-row proposed diff and a separately listed unresolved set.
2. Rewrite only unambiguous alternatives, portable equivalents, built-binary exit checks and safe no-match grep checks. Prove shell/table semantics for each pattern. A moving ref must be resolved to an actual immutable revision in the run, not replaced by a new moving expression.
3. List ambiguous shredded cells, missing identifiers, metavariables and uncertain expected semantics for author repair. Never invent test names, choose thresholds, add blanket success swallowing, or alter Expect/Evidence.
4. Reparse and run the same row evaluator before writing; refuse a non-clean candidate, write atomically, and make a second run empty. Keep closed records byte-identical even when --all is requested for reporting.

## Verify

Commands run from this repository root. Named new test contracts below are planned deliverables; they must be added by the implementer and the PASS assertion must fail if the selector finds nothing.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && go test ./... -run "^TestFixRows$" -count=1 -v > "${TMPDIR:-/tmp}/notice-30.log" 2>&1 && grep -F -e "--- PASS: TestFixRows " "${TMPDIR:-/tmp}/notice-30.log"` | exit 0; output contains the named PASS line. Matrix: dry-run changes no bytes; each deterministic repair removes its finding and preserves expectation; unresolved patterns stay untouched; a newly planted second bad row is found; malformed rewrite rolls back all files; historical records and Evidence hashes remain unchanged; repeated write is a no-op | check:ci +mutation +flow +dereference |
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
