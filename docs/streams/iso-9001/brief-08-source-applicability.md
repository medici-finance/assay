---
brief: assay:assay:iso-9001:08
title: Versioned source obligations and project applicability
why: Projects need to know which source revision a requirement came from and who decided it applies. A citation in prose cannot reliably reopen the right reviews when the source or project scope changes.
wave: 0
depends: []
unblocks:
- iso-9001/09
effort: M
gate: human
risk:
  regulatory: 'yes'
  customer: 'no'
  irreversible: 'no'
  sensitive-data: 'no'
gate-why: The accountable owner reviews applicability, evidence boundaries and permitted source use; an automated preparation result must not become a conformity or authorization claim.
decision-trigger: spec
decision-issue: 4400
ruling: "https://github.com/medici-finance/assay/issues/4400#issuecomment-6399479447"
design: DR-iso-9001-08
issues: []
schema: brief-v2
authored: 2026-10-03 by Codex (author-brief)
sources:
- docs/streams/iso-9001/project-assurance-spec.md
- freshness-checked 2026-10-03 @ cf31c32418ba49f93c679913813768542db1c072
exec-tier: strong
exec-tier-why: Cross-artifact authority, version applicability and evidence completeness must agree across independent readers.
domain: complicated
consumers:
- 'statusgen: follow-up iso-9001/08'
- 'spec: follow-up iso-9001/08'
- 'schemas: follow-up iso-9001/08'
- 'docs/iso9001-mapping.md: follow-up iso-9001/08'
- 'docs/evidence-bundle.md: out-of-scope (this brief defines upstream source records)'
version: 2
id: 7722e206-2451-45b4-b077-5c6f17ecf540
---

# Brief 08 — Versioned source obligations and project applicability

## Context

files: `spec/project-obligations-v1.md` (planned), `schemas/project-obligations-v1.json` (planned), `statusgen/projectobligations.go` (planned), `statusgen/projectobligations_test.go` (planned), `statusgen/testdata/projectobligations/` (planned), `statusgen/decisiongateanchor.go` (existing offline corroboration seam; reuse), `statusgen/decisionruling.go` (existing ruling provenance; reuse), `spec/registers-v1.md` (existing contract; extend), `docs/iso9001-mapping.md` (existing contract; extend), `changelog/iso-9001-08-source-applicability.md` (planned)

facts:
- requirements-seam: at the source baseline `statusgen/requirements.go` defines ID, acceptance and satisfied-by fields and has no source-revision or applicability-decision field
- decision-records: DECISIONS records and the offline corroboration seam in `statusgen/decisiongateanchor.go` already exist and are reused for acceptance
- source-input: only local, explicitly supplied synthetic sources; the loader never fetches the forge
- independence: runs offline without graph-execution/15

layering: `domain-core`. Validation of source revisions, mappings and acceptance binding is pure over explicit inputs (no clock, no forge); reading files, DECISIONS records and corroboration receipts is the adapter. Task 3 and 5 name the boundary; rows 2–3 test the core with the candidate-input validator bypassed.

design-fit:
  owner: `statusgen/requirements.go` keeps REQ meaning; the new source/mapping/applicability contract is `spec/project-obligations-v1.md` (planned) and its loader
  contract: S-decision-acceptance — applicability acceptance reuses DECISIONS records and the existing ruling corroboration; no second acceptance check
  retires: []
  weight: verbs 0, flags 0, refusals 0, rule-text lines 0 (the loader and spec sit outside the ratcheted `tools/desk` set); one register contract added beside DECISIONS
  why-add: n/a (no ratcheted growth). The new record exists because a REQ cannot carry source revision and project applicability without a second lifecycle in the REQ parser, which spec section 2 excludes; extending DECISIONS alone was considered and rejected because a decision has no source or mapping revision to bind

single-point-of-failure: the applicability-decision validator, the one place a forged or stale acceptance is refused. Behind it, the REQ dereference through the production parser fails on a dangling reference on its own signal (row 3), and the permission filter keeps denied content out before any decision exists (row 1); row 2 proves the validator rejects forged acceptance with the candidate-input validator bypassed.

## Read first

- [Project assurance specification](project-assurance-spec.md), especially sections 3–6.
- [Control exports](../graph-execution/brief-15-control-evidence.md) and [existing evidence contracts](../../evidence-bundle.md).
- [Requirements specification](../../../spec/registers-v1.md).

## Human decision

What is being decided: whether the source-applicability contract is the approved design, and
whether it becomes a control for adopting projects. The contract exists as a tested, offline
library. It records which edition of a source (a standard, regulation, contract or policy) an
obligation comes from and what use of its text is permitted, a mapping from one clause of that
edition to the project's existing requirement entries, and an applicability decision
(applicable, not applicable with a reason, or unresolved). A decision counts as accepted only
when it links to an existing decision record whose body carries a digest of the exact source,
mapping, project profile and proposal; the linked decision issue was closed by the named
reviewer; the disposition is "approved"; the reviewer is not the proposer; and a separate
review record is referenced. Anything short of that stays unresolved, and the library never
fetches from the forge. Nothing calls the library today, so an adopter's obligations file,
even one with a forged acceptance, fails nothing.

Negative-path evidence prepared at pickup, all passing on current main: source text whose use
is denied, unknown or absent never reaches model input (metadata only); a forged reviewer, a
forged corroboration reference, self-approval, a widened scope, a moved source, mapping or
profile revision, a typed name in place of the digest, replay of another subject's record, a
supplied receipt object and a bypassed shape validator are each refused or held unresolved; a
dangling or withdrawn requirement reference rejects the mapping; the existing requirement lint
is unchanged.

Limits accepted with option 1 or 2, none checked mechanically: a digest added to a decision
record after its issue closed is caught only by review of the change that adds it; one closed
issue corroborates every digest its record carries; the review reference is neither digested
nor corroborated, so it shows a review was claimed, not which review covered this subject; and
nothing checks that an interpretation is semantically right.

Options:
1. **Approve the contract as the design; keep it a library** — an approved design decision is
   recorded for it; this brief then moves to implemented and independent verification, and
   later review-packet work may build on the contract. No adopter project is gated by it until
   a separate, later decision activates it.
2. **Approve the contract and activate it** — as option 1, plus a follow-up brief wires the
   library into the lint so a supplied obligations file with a forged, stale or unresolved
   acceptance fails a check. The documentation calls it enforced only once that wiring is
   verified.
3. **Revise before approval** — name the limit or rule that must change; the contract is
   reworked in a new change and this decision is asked again on the revised contract. Nothing
   advances meanwhile.
4. **Decline** — the contract is not wanted; this brief is withdrawn, the library stays
   unwired, and the planned review-packet, change-impact and qualification work loses its
   upstream.

Default if no answer: none — blocks until answered. No response authorizes activation.

**Ruling (2026-10-25): option 1** — approve the contract as the design; keep it a library, not
activated. Recorded as [DR-iso-9001-08](../decisions/DR-iso-9001-08.md). No activation and no
lint wiring follow from this ruling; activation needs a separate, later decision.

https://github.com/medici-finance/assay/issues/4400#issuecomment-6399479447

## Ground rules

- Never git push, trigger workflows or run mutating infrastructure commands unless explicitly instructed. Feature branch and draft PR only; no merge, deployment, external provider or live infrastructure access.
- Preserve the owner-set stream status and priority; reprioritization is a separate owner decision.
- Public examples and fixtures are synthetic. No licensed normative text or adopter records.
- Stop at implemented; independent verification and normal review own later states.
- Unknown or missing evidence never becomes a pass. Required upstream behavior must be independently verified before operational reliance.
- If an instruction is unclear or contradicts the repository state, report NEEDS_CONTEXT rather than guess.

## Task

1. Implement the source-revision, obligation-mapping and applicability-decision contract in specification sections 3–4. Keep the existing REQ lifecycle and parser authoritative; the new mapping references REQ IDs and dereferences them. Extend the existing register specification with the link contract, without adding a second requirement state machine.
2. Accept only local, explicitly supplied sources. Record access/permitted-use references before content becomes eligible for model input; unknown or denied use permits authorized metadata only. Keep licensed text out of fixtures and public records. An external citation and human-authored interpretation are valid inputs where AI processing is not permitted.
3. Separate candidate analysis from authorized applicability. Reuse DECISIONS records and the offline corroboration seam in `statusgen/decisiongateanchor.go`, with provenance rules documented by `statusgen/decisionruling.go`; accept only an explicitly approved disposition whose interpretation has been reviewed. Corroboration establishes who/where, not whether prose approves or rejects. With no trusted corroboration receipt, applicability remains unresolved; this loader must not fetch the forge. Bind the accepted decision to source, mapping and project/profile revisions. Specify the mapping's acceptance link as the existing decision ID plus subject digest, explicit approved disposition, trusted corroboration reference and qualified-review reference; bind source/mapping/profile revisions in that subject digest. These are references to existing decision/review records, not new approvals. A supplied JSON object is not itself trusted corroboration. A typed name, model output or copied approval text is insufficient. Unresolved or conflicting applicability cannot be silently excluded.
4. Preserve prior revisions and supersession, including not-applicable decisions and reasons. Validate mappings against existing requirement entries and reject unknown references for this new review contract without changing unrelated legacy lint behavior.
5. Add named tests below through the production loader/validator. Prove the lower decision-validation boundary rejects forged acceptance even when the candidate-input validator is bypassed. Document the distinction between source identity, permitted use and semantic correctness.

## Interface contract

Permitted source revisions + authenticated project applicability decisions → mappings referencing existing REQ IDs; unresolved or unauthorized inputs cannot produce accepted mappings.

## Verify

These commands are future implementation obligations. No execution evidence is asserted by authoring. The named test must exist and execute; a zero-test exit is not evidence.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +dereference | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssuranceSourcePermissions$" .` | exit 0; named TestAssuranceSourcePermissions executes, with no [no tests to run]; A1: stable revisions and metadata-only input; denied AI use never enters model payload |
| 2 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssuranceApplicability$" .` | exit 0; named TestAssuranceApplicability executes, with no [no tests to run]; A2: forged identity, stale decision and bypass of candidate validation are rejected |
| 3 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssuranceReqDereference$" .` | exit 0; named TestAssuranceReqDereference executes, with no [no tests to run]; A3: a real fixture REQ resolves; dangling REQ fails through the production parser |
| 4 | check:ci +neighbour | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestRequirementValidEntryIsClean$" .` | exit 0; existing requirements without new mappings still parse unchanged |

## Pre-mortem and detection

- Denied source content reaching model input: row 1 asserts metadata-only input for a source without permitted use.
- Forged, stale or wrong-subject acceptance: row 2, including the candidate-input validator bypass.
- Mapping to a requirement that does not exist: row 3 fails through the production parser.
- Semantically wrong but correctly cited interpretation: review-only; the qualified reviewer must inspect the source and record disagreement. A presence check cannot settle it.

## Evidence

<!-- No implementation or independent verification is claimed. Append actual runs at implementation and verification. -->
### Independent verification — 2026-10-30 (run 1, source 5fdae0d07, container bracket)

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main 5fdae0d07b58aba9042b112258727ac33862bd93, detached home, unchanged except this Evidence section. Admission: this run's own attestation (medici-finance/assay#4988). `deskdispatch --check-verifier` exited 0 before any row ran, again at the bracket's open and close steps, and again after this Evidence edit.

Delivery: the work landed in medici-finance/assay#2380 (b7217d55004a68182290209f1c0f2dc8f35d711f), an ancestor of the observed source. The Verify section is byte-identical at the delivery and at the observed source, and no commit after the delivery changes the loader, its tests, its fixtures, the project-obligations spec or its schema. Later commits touched this brief only to record the human ruling (option 1, recorded as DR-iso-9001-08).

Runner class: local container, `--network none`. Every row is check:ci and the darwin host has no `unshare --net`, so all four rows ran inside one closed admission bracket: `statusgen verifyrun --open-bracket` on the host, then `statusgen verifyrun --sandbox=container-netns` inside the container, then `statusgen verifyrun --close-bracket` on the host, with no edit to the home between the steps. `statusgen verifyrun --bracket-digest` on the closed witness reports verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1. The image is the published verify-runner v1.0.67 image, run by digest (ghcr.io/medici-finance/verify-runner@sha256:f073d208c69e1e2f5ce5b680ad536d7a3cef3c80756823b9643cab4f7d1068aa; linux/amd64, go1.27.3, its own installed statusgen v1.0.67, the same release as the host's), read-only root, all capabilities dropped, no-new-privileges, as the host user. Mounts: the home read-only at its own path; the home's repository git directory read-only at its own path; the admission receipt read-only; a new directory for the witness file, the only writable mount. No module cache was mounted; the rows build with GOPROXY=off, GOFLAGS=-mod=readonly, GOTOOLCHAIN=local, KUBECONFIG=/dev/null, and no forge token in the environment. The container step recomputed the mounted home's tree digest and matched the value the host recorded at the open step. The Output cell is a digest of each row's stdout.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssuranceSourcePermissions$" .` | pass exit=0 sandbox=container-netns | sha256:0108a32b5866 | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssuranceApplicability$" .` | pass exit=0 sandbox=container-netns | sha256:3b77a3207bf7 | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssuranceReqDereference$" .` | pass exit=0 sandbox=container-netns | sha256:15ca1981bc89 | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestRequirementValidEntryIsClean$" .` | pass exit=0 sandbox=container-netns | sha256:442c6a8887e0 | 2026-10-30 | assay-verifier-app[bot] @ 5fdae0d07b58 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:cfdf07f40a2df989e39bf8731b9ab1560820a9f060814e7bcda533b83a0590da
Container-Image: sha256:f073d208c69e1e2f5ce5b680ad536d7a3cef3c80756823b9643cab4f7d1068aa
Verification-Attestation: medici-finance/assay#4988 run=c9b6c82f71f0f76e9874d73b256a33ef5be1bfc1e16544f7044254ca8bec5a7a source=5fdae0d07b58aba9042b112258727ac33862bd93 brief=docs/streams/iso-9001/brief-08-source-applicability.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=c9b6c82f71f0f76e9874d73b256a33ef5be1bfc1e16544f7044254ca8bec5a7a source=5fdae0d07b58aba9042b112258727ac33862bd93 tree=sha256:cfdf07f40a2df989e39bf8731b9ab1560820a9f060814e7bcda533b83a0590da witness=sha256:d667b709b717ef58a8f7c8226398a2be5f2c017d70a5f20470b032c96b1f333a opened=2026-10-30T16:12:38Z closed=2026-10-30T16:15:23Z before=receipt after=check-verifier

Supplemental probe S1 (no acceptance credit): the same four commands, run again in the same image with the same read-only mounts outside the bracket, with stdout kept. Each exited 0 and printed its named top-level `--- PASS:` line and an `ok` package line; none printed "no tests to run". Subtests observed passing: source permissions 5 (metadata-only payload where use is denied, unknown or absent; stable revisions; stored text needs permission and a matching digest), applicability 30 (positive control; forged reviewer and forged corroboration reference; self-approval; widened scope; moved source, mapping and profile revisions; a typed name in place of the digest; replay of another subject's record; a supplied receipt object; no trusted corroboration stays unresolved; and the decision boundary refusing what the shape validator passes), requirement dereference 10 (a real fixture REQ resolves; dangling and withdrawn REQs are refused through the production parser; legacy requirements lint unchanged). The neighbour row's test has no subtests.

No row calls the forge, so the ambient-identity concern does not apply to any row here.

Risk-bearing values. The brief carries risk regulatory: yes, so the enumeration ran over the delivery diff's non-test Go source, the new schema and the new spec. Literals found: obligationsSchema = "project-obligations-v1" (statusgen/projectobligations.go:48); the closed vocabularies poKinds, poUses = "permitted"/"denied"/"unknown" and poOutcomes (lines 52-54); the identifier patterns SRC-, MAP- and APD- with a 5-40 character slug (lines 58-60); poDigestRe (line 61); poMaxParaphrase = 600 (line 66, with the schema's maxLength 600 at schemas/project-obligations-v1.json line 66); mapping revision >= 1 (line 391); the model-input predicate (lines 275-276); and the required disposition "approved" (line 733). The corroborating closer's login is read from configuration (the bless login), not a literal. Ranking: the model-input predicate first, because a wrong value would let denied source text into a model payload, and a disclosure cannot be recalled by an edit. The acceptance disposition and the digest pattern come next, because they govern whether a forged acceptance is refused. Today these are reversible: under the ruling the library is not activated and nothing calls it. The paraphrase bound follows. The identifier patterns and the schema id are format constants and rank last.

- RISK-VALUE: DERIVED — aiEligible requires use.storage == "permitted" and use.ai == "permitted" (plus a non-empty access reference and text) @ statusgen/projectobligations.go:275-276 — the spec treats absent or unknown use as denied (spec/project-obligations-v1.md line 29) and admits content to a model only when both storage and ai are permitted, else metadata only (lines 178-180); "permitted" is the only granting value of the closed tri-state at line 53, so the predicate fails closed on denied, unknown and absent.
- RISK-VALUE: DERIVED — acceptance disposition must equal "approved" @ statusgen/projectobligations.go:733 — the brief's Task 3 accepts "only an explicitly approved disposition", and the spec's acceptance rule 7 states the disposition is approved (spec/project-obligations-v1.md lines 74-75 and 127); any other value is refused as disposition-not-approved.
- RISK-VALUE: DERIVED — poDigestRe = `^sha256:[0-9a-f]{64}$` @ statusgen/projectobligations.go:61 — a SHA-256 digest is 256 bits, which is 64 lowercase hex digits, matching the subject-digest form the spec binds (spec section 3).
- RISK-VALUE: NAMED, NOT DERIVED — poMaxParaphrase = 600 @ statusgen/projectobligations.go:66 — the spec (line 57), the schema (line 66) and the loader agree on 600 characters, but neither the spec nor the brief derives why 600 characters is short enough to keep licensed clause text out of a public record. That is a permitted-use policy judgment for the accountable owner, and the brief makes the paraphrase's adequacy review-only. The value is reversible by an edit.

Verdict accounting: 4 required rows, 4 pass, all witnessed in one closed container bracket; 0 fail; 0 without a result. Probe S1 carries no acceptance credit. The item is gate: human with risk regulatory: yes: this run records Evidence only and makes no lifecycle change. Sign-off belongs to the accountable owner, and the paraphrase-bound derivation above is routed to that review.

**VERIFY: PASS** — 4 of 4 rows pass (container-netns bracket, verdict=match); Evidence only, held for the human gate (gate: human, regulatory risk).

## Review

Gate: human. Confirm source rights, applicability/authority binding, consumer compatibility and honest completeness claims. Review semantic adequacy separately from mechanical evidence.
