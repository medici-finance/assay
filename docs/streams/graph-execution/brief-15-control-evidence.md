---
brief: assay:assay:graph-execution:15
title: Control profiles and complete scoped evidence exports
why: An auditor cannot distinguish complete evidence from a selected set of passing examples without knowing the expected population and omissions. A scope-aware export makes missing controls visible and records what the evidence actually proves.
wave: 4
depends:
- graph-execution/06
- iso-9001/01
- iso-9001/05
unblocks:
- graph-execution/18
effort: L
gate: human
risk:
  regulatory: yes
  customer: no
  irreversible: no
  sensitive-data: no
gate-why: The owner confirms control scope and truthful assurance claims; the export must not imply certification or conceal missing evidence.
decision-trigger: spec
issues: []
schema: brief-v2
authored: 2026-09-19 by Codex (author-brief)
sources:
- docs/streams/graph-execution/admission-assurance-spec.md
- freshness-checked 2026-09-18 @ 951ca784d100a7d201a28a34033da6709ec2ec8f
exec-tier: strong
exec-tier-why: Cross-component contracts and independent failure controls must agree; the implementation requires design judgment.
domain: complicated
consumers:
- 'statusgen: fixed-here'
- 'docs/evidence-bundle.md: fixed-here'
- 'docs/streams/iso-9001: out-of-scope (existing 03/04 own effectiveness and authorizer implementation)'
version: 1
id: 1699cd1a-941a-4d9a-920c-67ef5d1411d8
---

# Brief 15 — Control profiles and complete scoped evidence exports

## Context

files: `spec/control-assurance-v1.md` (planned), `schemas/control-profile-v1.json` (planned), `schemas/control-execution-v1.json` (planned), `statusgen/controlassurance.go` (planned), `statusgen/controlassurance_test.go` (planned), `statusgen/testdata/controlassurance/` (planned), `docs/evidence-bundle.md`, `docs/records-and-retention.md`, `changelog/graph-execution-15-control-evidence.md` (planned)

facts: Reuse the ISO validation pack and records policy. Organizational quality policy, audit programme and management review remain adopter obligations; this brief implements no certification claim.

single-point-of-failure: the new contract or policy alone cannot establish safe execution — independent boundary enforcement and independently read fixture/evidence results must still reject a bypass.

## Human decision

Decision-trigger: spec. At implementation pickup, prepare concrete policy choices and negative-path evidence, then file a self-contained decision issue. No response permits no activation; schema/test work may proceed within the declared scope.

## Read first

- [Admission and assurance amendment](admission-assurance-spec.md).
- [Stream specification](spec.md) and the typed prerequisites above.

## Ground rules

- Work in an isolated branch and draft PR under repository rules; no merge, deployment or live infrastructure query.
- Stop at implemented; independent verification owns verified/done.
- Existing authority and human gates remain binding. Missing prerequisite evidence is could-not-check.
- Public fixtures use example-org and synthetic data; do not copy adopter evidence.

## Task

1. Define profile/edition/owner/scope, run and period-level control executions, expected populations, exceptions, retention/access references, provenance and omissions. Link independent effectiveness receipts using iso-9001/03 rather than a second corrective-action model.
2. Implement an offline export/verify seam over permission-filtered input records. Include manifest, digests, expected versus included population, denied/missing records and verification instructions. No secret payload in envelopes; revoked access cannot leak a restricted artifact through export.
3. Keep required-record failures separate from diagnostic loss. Include tamper, incomplete-population, expired-exception and claimed-versus-observed fixtures. Regenerate factual export claims from actual fixtures; state storage immutability and legal retention enforcement as external capabilities unless implemented.
4. Document human review of applicable standards/criteria editions and scope. Integrate existing ISO release-authorizer and corrective-action work through references; neither a green graph nor AssayScore is an audit opinion.

## Interface contract

Run/evidence/control records → scoped export → independent offline reader detects omission, tamper and unauthorized payload.

Every shared consumer above must be reconciled against the implementing diff. Planned commands/tests below are deliverables, not claims that they already exist. Update the declared documentation and changelog with the implementation.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestControlAssurance" ./...` | exit 0; output includes PASS for TestControlAssurance, with no [no tests to run] for its owning package |
| 2 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestControlAssuranceMissingPopulation" ./...` | exit 0; output includes PASS for TestControlAssuranceMissingPopulation, with no [no tests to run] for its owning package |
| 3 | check:ci +flow | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestControlAssuranceExportVerify" ./...` | exit 0; output includes PASS for TestControlAssuranceExportVerify, with no [no tests to run] for its owning package |

The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->

## Review

Gate: human. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
