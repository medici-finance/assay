---
brief: assay:assay:statusgen:33
title: "Drive active notice severity from the existing registry after cleanup"
why: "A registry that only describes severity can disagree with actual lint behavior. One runtime owner should make the approved promotion real while preserving legacy and closure boundaries."
wave: 4
depends: ["statusgen/35"]
unblocks: []
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
id: c4be6ba0-f4bb-4070-83a1-0a579d9d6f75
gate-why: "Changes tooling behavior relied on by adopters. Human sign-off confirms the new authoring or repair behavior preserves historical evidence and the existing closure gates."
decision-trigger: start
consumers:
  - "statusgen/enforcementstatus.go: follow-up statusgen/33 (this brief; change is owed at implementation)"
  - "statusgen/enforcementstatus_test.go: follow-up statusgen/33 (this brief; change is owed at implementation)"
  - "statusgen/main.go: follow-up statusgen/33 (this brief; change is owed at implementation)"
  - "statusgen/verifyrows.go: follow-up statusgen/33 (this brief; change is owed at implementation)"
  - "statusgen/linkcheck.go: follow-up statusgen/33 (this brief; change is owed at implementation)"
  - "statusgen/riskfilescrossread.go: follow-up statusgen/33 (this brief; change is owed at implementation)"
  - "changelog/registry-severity.md: follow-up statusgen/33 (this brief; change is owed at implementation)"
---

# Brief 33 — Drive active notice severity from the existing registry after cleanup

## Context

files: statusgen/enforcementstatus.go, statusgen/enforcementstatus_test.go, statusgen/main.go, statusgen/verifyrows.go, statusgen/linkcheck.go, statusgen/riskfilescrossread.go, changelog/registry-severity.md
facts:
- Rule membership, exclusions and derivable/author-owed dispositions are defined by the [public rule-family inventory](README.md#public-rule-family-inventory); use that shared inventory for this brief's Task and Verify acceptance.
- lintRuleRegistry currently documents enforcement; individual emitters and closure audits decide severity.
- The registry already describes conditional fatal-on-closure behavior; replacing it with one unconditional boolean would weaken or overextend gates.
- Consumer cleanup and release adoption are separate operations; a tool source merge does not prove a consumer is clean.

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

1. Make severity evaluation use one existing registry owner with explicit active-authoring, new-closure and historical contexts. Delete replaced per-rule severity constants rather than retain two authorities.
2. After source cleanup and corroborated participating-consumer cleanup receipts, promote approved structural Family A and risk/files checks for active changed briefs. Keep untouched legacy debt from reddening an unrelated PR; preserve the differential and existing stricter closure/witness gates.
3. Render enforcement-status from the same runtime policy. Include safe report-only measurements when a participating consumer has not completed cleanup; do not silently declare promotion complete.
4. Document the release/adoption boundary and reversal as an ordinary reviewed policy change. No workflow activation, release tag or consumer pin is authorized by this implementation alone.

## Verify

Commands run from this repository root. Named new test contracts below are planned deliverables; they must be added by the implementer and the PASS assertion must fail if the selector finds nothing.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && go test ./... -run "^TestRuntimeNoticeSeverity$" -count=1 -v > "${TMPDIR:-/tmp}/notice-33.log" 2>&1 && grep -F -e "--- PASS: TestRuntimeNoticeSeverity " "${TMPDIR:-/tmp}/notice-33.log"` | exit 0; output contains the named PASS line. Matrix: every registered promoted rule agrees between runtime and rendered table; untouched legacy stays unchanged; newly introduced active defect fails; historical record stays frozen; new closure remains strict; missing cleanup receipt cannot be reported as promoted | check:ci +mutation +flow +dereference |
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
