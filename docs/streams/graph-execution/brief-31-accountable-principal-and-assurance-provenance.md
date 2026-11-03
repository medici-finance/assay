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
### Non-implementer verifier run (Linux container witness) — VERIFY: PASS — 2026-10-30 claude-opus-5-5-verifier

Subject: merged main 5fdae0d07b58 (the work was delivered by PR #4801, commits 201e92d44 and 6cd1786cd). All five rows are check:ci. They ran in the closed admission bracket: the host opened the bracket, the rows ran in a network-off container (the verify-runner v1.0.67 image for linux/arm64, Go 1.27.3, GOPROXY=off), and the host closed the bracket. The bracket digest check reports verdict=match. The witness below is the closed witness file, byte for byte.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDutyPrincipals$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDutyPrincipals " "$out")` | pass exit=0 sandbox=container-netns | sha256:d20ce6079609 | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDutySponsor$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDutySponsor " "$out")` | pass exit=0 sandbox=container-netns | sha256:ce2fd3c6307d | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestProofClass$" ./domain/ > "$out" && grep -F -- "--- PASS: TestProofClass " "$out")` | pass exit=0 sandbox=container-netns | sha256:422af381c9dc | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestDutyEvidenceFlow$" ./domain/ > "$out" && grep -F -- "--- PASS: TestDutyEvidenceFlow " "$out")` | pass exit=0 sandbox=container-netns | sha256:22693238c21a | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestProvenanceSchema$" ./domain/ > "$out" && grep -F -- "--- PASS: TestProvenanceSchema " "$out")` | pass exit=0 sandbox=container-netns | sha256:2388825ea663 | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:cfdf07f40a2df989e39bf8731b9ab1560820a9f060814e7bcda533b83a0590da
Container-Image: sha256:5fa242b9d02a5f769fe9d541f5a8ad33ae291e418a735617faf4812ef21e8629
Verification-Attestation: medici-finance/assay#4987 run=61f925c5b58ae34581d22f685820abd905a4fd9d9cab2517d0b70b2dfeef3479 source=5fdae0d07b58aba9042b112258727ac33862bd93 brief=docs/streams/graph-execution/brief-31-accountable-principal-and-assurance-provenance.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=61f925c5b58ae34581d22f685820abd905a4fd9d9cab2517d0b70b2dfeef3479 source=5fdae0d07b58aba9042b112258727ac33862bd93 tree=sha256:cfdf07f40a2df989e39bf8731b9ab1560820a9f060814e7bcda533b83a0590da witness=sha256:d05484887046ef7b8359a3f7201030a2d5176f81cb1a2bf3322eef567eba18d0 opened=2026-10-30T16:12:55Z closed=2026-10-30T16:13:43Z before=receipt after=check-verifier

Rows 1-5 pass in the witness: each exits 0 and prints its own named PASS line.

#### Expect coverage and supplemental mutation probes (no acceptance credit)

The witness decides each row on its exit status and named PASS line. To check the rest of each Expect cell, the verifier read the test bodies at this commit. A planted mutation was then run for each row in a throwaway export of this source, outside the admitted home. The export used the same runner image, ran with the network off, and ran the row's own command. Each probe ran unmutated first, then mutated, and the file was restored byte for byte afterwards. These probes earn no acceptance credit.

- Row 1 probe (2026-10-30). The test asserts three cases: review-independent shares the author's issuer and is satisfied; review-overlap is unsatisfied with principal-overlap; review-same-key is unsatisfied with principal-overlap. Each fixture shares the issuer and differs in key. The mutation "compare only leaf keys" makes sharesLineage compare Key only. Unmutated: exit 0. Mutated: exit 1, with review-overlap and review-same-key failing ("got satisfied [], want unsatisfied [principal-overlap]").
- Row 2 probe (2026-10-30). The test asserts that a sponsor who is the author's principal is satisfied, so sponsorship is not delegation. A sole human who sits in the reviewer's delegation is unsatisfied with principal-overlap. A weaker proof hold does not soften that overlap. The mutation "ignore delegation overlap" drops the delegation loop from lineage. Unmutated: exit 0. Mutated: exit 1 at the sole-human case.
- Row 3 probe (2026-10-30). The test asserts four things:
  - a forged signed label reads as attested and holds;
  - self-report holds;
  - bot-for-human, and a human acting on another's request or for another, are each unsatisfied with not-human-act;
  - claimed attribution, uncorroborated lineage and missing duty evidence each hold.
  The mutation "trust the declared proof label" makes EffectiveProof return the declared class. Unmutated: exit 0. Mutated: exit 1 ("effective proof signed").
- Row 4 probe (2026-10-30). The test binds to a real Assess result on the candidate-a fixture. It asserts that the synthetic trusted-source selector refuses three claims, each with provenance-not-selected:
  - a hidden-lineage claim;
  - a raised-proof claim;
  - a fully corroborated forged claim passed straight to the duty check.
  An unknown item and a nil selector are each could-not-check. Evidence for another candidate is refused with provenance-subject, and a stale assessment with stale-generation. The mutation "source identity from model text" makes Corroborate return the caller's claim. Unmutated: exit 0. Mutated: exit 1 ("got <nil>, want refusal provenance-not-selected").
- Row 5 probe (2026-10-30). The test asserts four things:
  - the schema defs are named in the spec;
  - work-identity and candidate are identical to verification-domain-v1;
  - the principal pattern equals the Go grammar, and every Go field is a required schema property;
  - the 28 positive and negative fixtures, including missing-environment, missing-capture-actor, omitted-signature, waiver-label and declared-independence, get the expected schema and semantic verdicts.
  The mutation "drop a required provenance field" removes environment from the schema's required list. Unmutated: exit 0. Mutated: exit 1 ("schema fields drift").

Supplemental read: the delivered spec, schema, owner row in the contracts doc and enforcement-model section are present at this commit. The changelog fragment was folded into the CHANGELOG under the v1.0.65 aggregation.

#### Risk-bearing values (enumerate, rank, derive)

Trigger: the frontmatter sets sensitive-data: yes. Irreversible is no.

The enumeration covered the non-test diff of PR #4801 under the statusgen domain package (provenance.go), the evidence-provenance-v1 schema and the brief's Deliverables. The literals found:

- ProvenanceSchema = "evidence-provenance-v1" @ statusgen/domain/provenance.go:14
- actorKinds = human, provider-app, agent-session, service @ provenance.go:18
- proofClasses = self-report, authenticated, attested, signed, custody-qualified @ provenance.go:19
- corroborationStates = corroborated, claimed, missing, withheld, could-not-check @ provenance.go:20
- dutyStates = satisfied, held, unsatisfied @ provenance.go:22
- principalRE = ^[a-z][a-z0-9-]*:[a-z0-9][a-z0-9._-]{0,127}$ @ provenance.go:26
- corroborated = "corroborated" @ provenance.go:29
- the required capability "provenance@1" @ provenance.go:129
- the EffectiveProof ladder order Attribution, Execution, Signature, Custody @ provenance.go:183
- the human-act literal Executor.Kind != "human" @ provenance.go:321
- a test-only fixture floor, len(files) < 20 @ statusgen/domain/provenance_test.go:329

Ranked by what breaks if the value is wrong:

1. The proof ladder and its dimension mapping. A wrong order would let a weaker corroboration satisfy a stronger required claim.
2. The single counting state "corroborated". If another state counted, an uncorroborated fact would pass.
3. The duty-state severity order. A wrong order could soften an overlap into a hold.
4. The principal grammar. It is a privacy bound on what a record can carry.
5. The schema id, capability, actor kinds and fixture floor. These are reversible names and test bounds and need no derivation.

The contract is pure and has no wired consumer, so every value can be changed by an edit and a release.

RISK-VALUE: DERIVED — proofClasses = self-report, authenticated, attested, signed, custody-qualified @ statusgen/domain/provenance.go:19 (ladder Attribution, Execution, Signature, Custody @ provenance.go:183) — this is the order the brief's Task 1 separates the dimensions in (authenticated attribution, attested execution, signature verification, custody/isolation evidence), and the spec states the same weakest-first ladder. EffectiveProof is cumulative and stops at the first uncorroborated rung, so no rung is reached without every rung below it. Row 5 pins the enum against the schema, and the row 3 mutation shows the declared label cannot raise it.
RISK-VALUE: DERIVED — corroborated = "corroborated" @ statusgen/domain/provenance.go:29 — the brief requires that missing corroboration be explicit and that a claimed signature or issuer label is not proof. Counting only this one state is fail-closed: claimed, missing, withheld and could-not-check all hold (the spec says withheld counts as uncorroborated). Row 3 asserts claimed attribution and uncorroborated lineage hold.
RISK-VALUE: DERIVED — dutyStates = satisfied, held, unsatisfied @ statusgen/domain/provenance.go:22 — the spec orders the states by severity, and note() only ever moves a result rightwards, so an observed principal overlap is never downgraded by a later hold. Row 2 asserts unsatisfied survives a concurrent proof-uncorroborated hold.
RISK-VALUE: NAMED, NOT DERIVED — principalRE id bound {0,127} @ statusgen/domain/provenance.go:26 — the grammar's shape is derived. It has a lower-case kind prefix and an opaque id with no "@", space or upper case, so names and email addresses do not fit, as the spec's privacy rule and the brief's synthetic-identity ground rule require. No source states the 128-character id ceiling. The bound is reversible and is a privacy-scope question for the human gate.

Gate: human, and the item is risk-classed (sensitive-data). This run records Evidence only and makes no lifecycle change. Trust-profile and privacy adequacy stay with the owner.

**VERIFY: PASS** — 5 of 5 rows pass in the bracketed container witness at 5fdae0d07b58 (bracket digest verdict=match). All five supplemental mutations are killed. No implementation defect was found. The item stays at its current state for the human gate.

## Review

Gate: human. Confirm corroboration, accountable principals and truthful assurance/privacy claims.
