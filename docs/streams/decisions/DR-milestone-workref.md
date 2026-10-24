---
id: DR-milestone-workref
date: "2026-10-24"
title: "Use one pure shared work-reference library with explicit legacy consumer profiles"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Keep independent QID and trailer parsers: rejected because equivalent references can drift across consumers and revisions."
  - "Use a helper binary or subprocess protocol: rejected because the approved design requires direct importable library calls without another runtime or authority boundary."
  - "Broaden legacy APIs to accept every new reference: rejected because the existing consumers intentionally accept different strict legacy grammars."
accepted:
  - "The shared statusgen/workref package remains pure and stdlib-only; it owns parsing and resolution over explicit context and alias inputs."
  - "Both production modules call the common library through explicit legacy profiles that preserve their existing acceptance sets."
  - "Typed adapters preserve reference kind, exact authored revision, alias/root binding and one logical work identity; they grant no credential, claim or approval authority."
  - "Capability negotiation, operational prefixed authoring, milestone loading and dispatch integration remain separately scoped follow-ups."
---

## Decision and provenance

This records the bounded design of milestone-support/01, delivered by
[PR #4224](https://github.com/medici-finance/assay/pull/4224). Its existing decision
[issue #4219](https://github.com/medici-finance/assay/issues/4219) presents option 1 as
approval of that bounded design while preserving independent authority checks and
legacy parser acceptance sets. The maintainer selected
[option 1 on 23 October](https://github.com/medici-finance/assay/issues/4219#issuecomment-6373998126).

That comment names the brief, not this newly recorded DR identifier. It is preserved
as the existing design decision; it is not falsely asserted as a corroborated
`ruling:` for this record. The maintainer requested reconciliation on 24 October in
[the approval and reconciliation change](https://github.com/medici-finance/assay/issues/4347).
Before merging this record, link a new, unedited maintainer comment on #4219 naming
`DR-milestone-workref` as required by registers-v1 §7.5. Until that link is recorded,
this public placeholder is unratified; a bot relay or PR approval is not its replacement.

## Acceptance boundary

The independent container-bracket PASS is recorded in merged
[PR #4256](https://github.com/medici-finance/assay/pull/4256). This design record
does not alter its rows, runner identity, source, attestation or human acceptance boundary.
A witnessed implemented row with a recorded PASS can satisfy a dependent brief under
canonical eligibility without marking this gate:human brief verified or done.
