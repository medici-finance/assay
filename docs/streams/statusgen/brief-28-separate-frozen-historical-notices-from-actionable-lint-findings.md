---
brief: assay:assay:statusgen:28
title: "Separate frozen historical notices from actionable lint findings"
why: "Thousands of repeated notices include closed evidence records that must not be rewritten. A short default report and inspectable detail let operators see work they can act on."
wave: 1
depends: []
unblocks: ["statusgen/30", "statusgen/31", "statusgen/35"]
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
id: d24c4a8f-13d2-4842-b7ad-5b4b49c7db6c
gate-why: "Changes tooling behavior relied on by adopters. Human sign-off confirms the new authoring or repair behavior preserves historical evidence and the existing closure gates."
decision-trigger: start
consumers:
  - "statusgen/main.go: follow-up statusgen/28 (this brief; change is owed at implementation)"
  - "statusgen/verifyrows.go: follow-up statusgen/28 (this brief; change is owed at implementation)"
  - "statusgen/linkcheck.go: follow-up statusgen/28 (this brief; change is owed at implementation)"
  - "statusgen/riskfilescrossread.go: follow-up statusgen/28 (this brief; change is owed at implementation)"
  - "statusgen/orderinggate.go: follow-up statusgen/28 (this brief; change is owed at implementation)"
  - "statusgen/lifecyclelint.go: follow-up statusgen/28 (this brief; change is owed at implementation)"
  - "statusgen/consumers.go: follow-up statusgen/28 (this brief; change is owed at implementation)"
  - "statusgen/brieffile.go: follow-up statusgen/28 (this brief; change is owed at implementation)"
  - "statusgen/notice_report.go: follow-up statusgen/28 (this brief; change is owed at implementation)"
  - "statusgen/notice_report_test.go: follow-up statusgen/28 (this brief; change is owed at implementation)"
  - "changelog/historical-notices.md: follow-up statusgen/28 (this brief; change is owed at implementation)"
---

# Brief 28 — Separate frozen historical notices from actionable lint findings

## Context

files: statusgen/main.go, statusgen/verifyrows.go, statusgen/linkcheck.go, statusgen/riskfilescrossread.go, statusgen/orderinggate.go, statusgen/lifecyclelint.go, statusgen/consumers.go, statusgen/brieffile.go, statusgen/notice_report.go, statusgen/notice_report_test.go, changelog/historical-notices.md
facts:
- Rule membership, exclusions and derivable/author-owed dispositions are defined by the [public rule-family inventory](README.md#public-rule-family-inventory); use that shared inventory for this brief's Task and Verify acceptance.
- unfailableRowAudit already aggregates gotest-run-vacuous, prose-led-command and strength findings on pre-existing closed records; most other emitters still print individual records.
- The enforcement registry describes conditional closure severity and is not a runtime severity switch.
- A new closure on this branch is not historical merely because the head README says done.

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

1. Extend existing closed-record scoping across Family A, identifier dereference, risk/files, why, ordering and legacy review-metadata notices. Preserve per-file errors on active work and the stricter checks for closures introduced by the branch.
2. Default text prints actionable findings and bounded per-rule historical totals. Add one explicit detail opt-in (--all) and a deterministic --lint --json report containing per-rule and per-status counts, actionable and historical partitions, and could-not-check states. Do not hide ordinary PROBLEMs or change their exit code.
3. Use one reporting/normalization owner shared with the differential; do not introduce a second independent notice-classification table. A historical count can rise when a new detector discovers old debt; distinguish that from a newly closed defective brief rather than falsifying or deleting history.
4. Exclude docs/archive from live lifecycle classification as previously intended. Do not blanket-exempt unrelated active documents or create fake historical security-review records.

## Verify

Commands run from this repository root. Named new test contracts below are planned deliverables; they must be added by the implementer and the PASS assertion must fail if the selector finds nothing.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && go test ./... -run "^TestHistoricalNoticeReport$" -count=1 -v > "${TMPDIR:-/tmp}/notice-28.log" 2>&1 && grep -F -e "--- PASS: TestHistoricalNoticeReport " "${TMPDIR:-/tmp}/notice-28.log"` | exit 0; output contains the named PASS line. Matrix: active, pre-existing done, pre-existing verified, unknown-status and newly closed briefs; every included emitter; text/JSON total agreement; detail mode restores individual historical lines; closure and witness failures retain nonzero exit | check:ci +mutation +flow +dereference |
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
