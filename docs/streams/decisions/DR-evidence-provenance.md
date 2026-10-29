---
id: DR-evidence-provenance
date: "2026-10-29"
title: "Use corroborated evidence provenance and accountable-principal duty checks through the shared pure domain contract"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Keep provenance report-only (option 2 on decision #4800): not selected. Consumers could not claim the stronger independence guarantee until a later qualified profile. No additional human rationale for rejecting this option was recorded."
  - "Treat distinct provider Apps, keys or issuers as proof of independent duties: excluded by the accepted brief; accountable and delegating principals are the comparison, not credentials."
accepted:
  - "Option 1, corroborated provenance, is the recorded direction. Admission and publication consumers must obtain corroboration through trusted composition; a model-supplied claim cannot select its own evidence."
  - "The delivered implementation is a pure stdlib-only contract in statusgen/domain, with the evidence-provenance-v1 schema, fixtures, duty-flow checks and enforcement-model limits. It does not authenticate actors or verify signatures itself."
  - "EvaluateDuty requires a trusted Selector and corroborates every supplied claim before reading its fields. A caller that supplies a selector which echoes its input defeats that trust boundary; composing a trusted selector remains the separate consumer obligation."
  - "Responsible and delegating principals determine duty overlap. Issuer, key and sponsor do not establish independence; a represented principal must be folded into responsible/delegation by the trusted adapter."
  - "Missing corroboration holds the stronger claim. A bot acting for a person is not a human act. Declared proof never raises effective proof, and no waiver or automatic one-human exception is created."
  - "Consumers graph-execution/14, /25 and /15, runtime wiring, deployment and any exception remain separately scoped. No production selector, credential grant, roster change, enrollment, custody qualification or live activation is supplied by this record."
---

## Status and recorded human direction

**Proposed record; identifier-bound ratification is pending.** The maintainer already
selected option 1 for graph-execution/31 in the unedited human comment on
[decision #4800](https://github.com/medici-finance/assay/issues/4800#issuecomment-6481737472):

> graph-execution/31: 1 — use corroborated provenance

The decision is closed as human-decided. That comment does not name this DR identifier,
so it records the selected option but is not asserted to satisfy registers-v1 §7.5
for this newly transcribed record. No `ruling:` is supplied until a qualifying unedited
human comment naming `DR-evidence-provenance` exists. No bot relay or PR review is
substituted for it.

## Delivered scope

[PR #4801](https://github.com/medici-finance/assay/pull/4801) merged at
`f1db3bc1b6c8b5e4d20895c2a5cafbcf89417abd`. The delivered files include
`statusgen/domain/provenance.go`, its tests, `schemas/evidence-provenance-v1.json`,
`spec/evidence-provenance-v1.md`, provenance fixtures and the shared owner/enforcement
records. The delivering PR documents why it left the board row at todo: the design
record and decision pointer were missing.

The record preserves the delivered Selector trust-root limitation and the separation
between attributed actor, effective proof and independent duty. Pure validation and
synthetic fixtures do not qualify hardware custody or independently authenticate a
real actor. No new implementation or runtime activation is authorized here.

## Reconciliation and acceptance boundary

The brief now names this design record and existing decision #4800. Task, Verify,
version, historical Evidence, gate and risk are unchanged. Its own delivered consumer
route is recorded as fixed-here; external consumer work remains follow-up.

The generated lifecycle row must remain todo while the design-record gate is
unsatisfied. After a qualifying human ruling, bind its exact URL as `ruling:`, run the
canonical reconciliation reader and retain its complete admitted proposal. Any
implemented promotion comes through the native writer, not a manual board-cell edit.
Independent verification and final human acceptance remain separate acts.
