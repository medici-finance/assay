---
brief: assay:assay:graph-execution:31
title: Accountable principal and assurance provenance
why: >-
  Different keys or role Apps do not prove that an author and reviewer are independent.
  Evidence needs corroborated responsible-principal lineage and an honest proof class
  so shared issuance infrastructure neither invents nor conceals independence.
wave: 3
depends: ["graph-execution/28"]
unblocks: ["graph-execution/14", "graph-execution/25", "graph-execution/15"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: Identity/delegation provenance affects acceptance; the owner confirms the declared trust profile and privacy scope without treating role attribution as independent approval.
decision-trigger: start
decision-issue: 4800
ruling: https://github.com/medici-finance/assay/issues/4800#issuecomment-6505931139
design: DR-evidence-provenance
issues: [4800]
schema: brief-v2
version: 1
id: 83d2df63-42b8-4e12-9b6a-c202e0a0a2aa
authored: 2026-10-14 by worker-desk (author-brief)
sources:
  - docs/proposals/non-ci-architecture/23-runtime-contract-alignment.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md — requester, represented actor and provider identity
  - docs/streams/graph-execution/brief-25-publication-review-bridge.md — independent verdict binding
  - docs/contracts.md — S-identity, S-evidence-coverage and S-review-verdict
  - freshness-checked 2026-10-14 @ 9a8df91dc; attribution/role checks exist but no responsible-principal domain contract
exec-tier: strong
exec-tier-why: Independence, delegation and assurance claims cross identity, evidence and publication boundaries.
domain: complicated
consumers:
  - "spec/evidence-provenance-v1.md, schemas/evidence-provenance-v1.json, statusgen/domain, statusgen/testdata/domain/provenance: fixed-here (delivered by PR #4801)"
  - "tools/desk/internal/deskkit/graphadmission.go: follow-up graph-execution/14"
  - "workflow/publication: follow-up graph-execution/25"
  - "control exports: follow-up graph-execution/15 (consume proof class; no certification claim)"
  - "runtime controller: follow-up graph-execution/21 (through admission /14)"
  - "provider credentials and threshold signer infrastructure: out-of-scope (current custody owners remain; no keys, roster edits or crypto deployment here)"
---

# Brief 31 — Accountable principal and assurance provenance

## Context

files: `spec/evidence-provenance-v1.md` (planned), `schemas/evidence-provenance-v1.json` (planned), `statusgen/domain/provenance.go` (planned), `statusgen/domain/provenance_test.go` (planned), `statusgen/testdata/domain/provenance/` (planned), `docs/contracts.md`,
`docs/enforcement-model.md`, `changelog/graph-execution-31-evidence-provenance.md` (planned).

facts (9a8df91dc, 2026-10-14):
- /28 owns shared domain records and remains a prerequisite, not a second implementation.
- Existing evidence attribution and role/candidate checks are not a principal/delegation
  independence proof. /25's publishing profile is disabled pending separate activation.
- Existing S-identity authenticates provider/session actors; this brief consumes corroborated
  identity facts and does not mint actors, keys, roles or provider credentials.
- Runtime, store and providers remain prospective. Fake principals test contract meaning,
  not hardware custody, process isolation or legal non-repudiation.

layering: pure evidence provenance and duty applicability in /28's domain package; authenticating
principals and protecting evidence selection remain trusted adapters. Keep one evidence owner.

single-point-of-failure: lineage data could be uncorroborated; /14 and /25 must independently
obtain it from trusted composition, not model text. Row 4 bypasses the pure upper validator
and proves a synthetic trusted-source selector refuses caller-chosen provenance.

design-fit:
  owner: statusgen/domain evidence records; S-identity owns authentication
  contract: S-verification-domain (planned by /28), extended by evidence-provenance-v1
  retires: [assumption that different session keys or provider Apps establish independent duties]
  weight: "verbs 0, flags 0; additive provenance validation"
  why-add: n/a; reuse the domain/evidence owners instead of a new identity broker

## Human decision

The evidence profile will record authenticated actor, responsible principal, delegation
lineage and observed custody/assurance without changing existing grants. Unknown lineage
must hold claims requiring independence. This work must not claim hardware-backed human
approval or threshold signing without separately qualified evidence.

Options:
1. **Use corroborated provenance** — implement the pure contract and require trusted
   corroboration at admission/publication; deployment and any exception remain separate decisions.
2. **Keep report-only provenance** — retain current gates and block consumers from claiming
   the stronger independence guarantee until a later qualified profile exists.

Default if no answer: none — blocks until answered.

No activation or new independence claim; contract/tests may be reviewed.

## Read first

- [Runtime alignment](../../proposals/non-ci-architecture/23-runtime-contract-alignment.md).
- /28 domain records, /14 admission, /25 publication, /15 scoped control exports.
- `docs/contracts.md` identity/evidence/review owner rows and `docs/enforcement-model.md`.

## Ground rules

- No enrollment, roster/policy grant edits, credentials, production data or crypto deployment.
- Isolated branch/draft PR; no merge, live infrastructure or activation. Stop at implemented.
- Synthetic identities; redact sensitive provenance under existing access policy.
- Do not add an automatic one-human waiver or relax existing independent/human gates.

## Task

1. Define versioned provenance: authenticated requester, represented actor, executing provider
   identity, responsible principal, delegating-principal lineage, sponsor accountability,
   issuing infrastructure, candidate/contract binding, capture actor/source, environment
   manifest and declared proof class. Separate authenticated attribution, attested execution,
   signature verification, custody/isolation evidence and duty independence. Missing required
   corroboration is explicit; a claimed signature or issuer label is not verified proof.
2. Implement pure validation/applicability using trusted normalized input facts. Compare the
   responsible/delegating principals required by the selected existing duty policy, excluding
   common issuance infrastructure. Sponsors establish accountability; they are not automatically
   every acting principal's delegator. Distinct keys for one responsible principal do not
   satisfy a required independent duty. Never interpret a provider bot action as a human act.
3. Add fixtures for independent principals with a shared issuer, overlapping delegation,
   different keys for the same principal, sponsor versus delegation, missing corroboration,
   candidate mismatch and forged higher proof class. If the sole human acts and controls
   the reviewers' responsible/delegating lineage, required independence is unsatisfied.
   Any exception must be an explicitly applicable retained human decision; no exception
   is created or approved here. Never satisfy a mandatory independent gate merely by
   displaying a waiver label.
4. Exercise real /28 Assess/evidence binding with provenance validation and a separate
   synthetic trusted-source selector. A model-supplied principal or proof label cannot
   override the selector. /14 and /25 consume corroborated records with their current
   effect/role controls, not a new token broker. /15 exports accurate proof and omissions.
5. Update the owner row, enforcement-model limitations, schema and changelog. Environment
   identity makes evidence attributable and comparable; it does not prove reproducibility.
   Preserve recorded decisions and all historical Evidence; no signer ceremony is invented.

## Interface contract

Trusted adapter identity/capture observations -> domain provenance/applicability -> existing
admission/evidence consumers. Proof class has no authority to mint a grant or satisfy a
stronger required claim without corroboration. Actual authentication/custody stays outside
this pure package and needs separate R3/R4 qualification before managed use.

## Verify

Named tests are future deliverables; fixtures prove contract meaning only.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDutyPrincipals$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDutyPrincipals " "$out")` | exit 0; named PASS; shared issuer with independent principals passes; overlapping delegation and same principal with different keys fail; mutation: compare only leaf keys |
| 2 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDutySponsor$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDutySponsor " "$out")` | exit 0; named PASS; sponsor accountability is separate from delegation; an actual sole-human delegation overlap cannot satisfy independence; mutation: ignore delegation overlap |
| 3 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestProofClass$" ./domain/ > "$out" && grep -F -- "--- PASS: TestProofClass " "$out")` | exit 0; named PASS; bot-for-human, self-report and unverified signature cannot satisfy a stronger claim; missing corroboration holds; mutation: trust the declared proof label |
| 4 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDutyEvidenceFlow$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDutyEvidenceFlow " "$out")` | exit 0; named PASS; real candidate/Assess flow refuses wrong-subject evidence; synthetic trusted-source selector rejects caller-chosen principal with upper validation bypassed; mutation: source identity from model text |
| 5 | check:ci +dereference +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestProvenanceSchema$" ./domain/ > "$out" && grep -F -- "--- PASS: TestProvenanceSchema " "$out")` | exit 0; named PASS; positive/negative fixtures and spec agree with mandatory fields, environment/capture provenance and omitted-proof states; mutation: drop a required provenance field |

## Definition of Done

Verify rows discriminate and declared docs/schema/changelog agree. /14 and /25 consumption
is explicit; current identity/evidence owners and human gates remain. No stronger deployed
assurance is claimed by fixture success. Trust-profile/privacy adequacy stays human review.

## Pre-mortem

Leaf-key independence (row 1), sponsor/delegator confusion (row 2), invented proof strength
(row 3), model-selected provenance (row 4), and schema drift (row 5) have failing controls.

## Evidence

<!-- Independent verifier records each row, exact subject/environment, command, exit and observed result. -->

## Review

Gate: human. Confirm corroboration, accountable principals and truthful assurance/privacy claims.
