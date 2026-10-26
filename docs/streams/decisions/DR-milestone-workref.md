---
id: DR-milestone-workref
date: "2026-10-24"
title: "Use one pure shared work-reference library with explicit legacy consumer profiles"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/4219#issuecomment-6389330807"
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

The 23 October comment names the brief. The maintainer reaffirmed that same bounded
design on 24 October in an unedited own-account comment explicitly naming this
record, linked by `ruling:` above. This records the exact identifier required by
registers-v1 §7.5 without treating a bot relay or a PR approval as the human's comment.
The [reconciliation request](https://github.com/medici-finance/assay/issues/4347)
preserves independent verification and final acceptance authority.

## Acceptance boundary

The independent container-bracket PASS is recorded in merged
[PR #4256](https://github.com/medici-finance/assay/pull/4256). This design record
does not alter its rows, runner identity, source, attestation or human acceptance boundary.
A witnessed implemented row with a recorded PASS can satisfy a dependent brief under
canonical eligibility without marking this gate:human brief verified or done.
