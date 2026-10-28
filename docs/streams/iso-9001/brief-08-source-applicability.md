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

## Review

Gate: human. Confirm source rights, applicability/authority binding, consumer compatibility and honest completeness claims. Review semantic adequacy separately from mechanical evidence.
