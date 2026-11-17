---
id: DR-desk-tools-52
date: "2026-11-16"
title: "Decision and verification cards linked without conflating authority (desk-tools/52): option 1, approve the delivered implementation after independent verification, is the proposed design; no driver ruling is recorded, so this record is placeholder-unratified"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Option 2 — request corrections and keep the change pending; option 3 — hold the change (the brief's `## Human decision` and decision issue #4890). Not ruled out: no driver ruling chooses between the options."
  - "Migrating human authority onto the cards, or replaying historical closes (excluded by the brief's own `## Interface contract`)."
accepted:
  - "Proposed, not ruled. Card emitters expose canonical brief identity, an explicit purpose, and source and verification revision links; decision and verification cards stay separate (#4898)."
  - "Proposed, not ruled. The classifier reads typed card lines only (a non-code line, after CRLF normalisation) and rejects malformed, foreign, mixed, wrong-purpose or combined cards (#4898)."
  - "Proposed, not ruled. The card is informational today: no emitter writes it into a `decision-gate:` issue body, and the verify-gate close still keys on its marker only (#4898)."
  - "Under any option, approval grants no workflow activation or additional write scope (the brief's `## Human decision`), and independent verification stays a separate act; this record supplies neither."
---

## State and provenance

**Approval state: placeholder-unratified. This is not an approved design record.** No recorded
driver ruling exists for desk-tools/52. Its decision issue,
[issue #4890](https://github.com/medici-finance/assay/issues/4890), is open and carries no
driver comment choosing an option.

The `decided-by` stamp above is the public placeholder and names nobody
(`spec/registers-v1.md` §7.5). This record carries no `ruling:` link, because there is no driver
comment to link. A local `statusgen --corroborate` run is expected to report it
`placeholder-unratified` until the driver posts an unedited comment on #4890, from the driver's own account, naming
`desk-tools/52` and `DR-desk-tools-52` with the chosen option. The `ruling:` key is added by a
later push, only after that comment exists.

The brief's `design:` key is deliberately absent. The design-gate lint
(`spec/lifecycle-v1.md` §4.4) checks only that a cited record exists, never that it is approved,
so citing this record now would release the hold before the record is ratified. The `design:` key
is added in the same push that adds this record's `ruling:` link.

**Written after implementation.** desk-tools/52 was delivered by
[#4898](https://github.com/medici-finance/assay/pull/4898), merged 2026-10-30, before this record
existed. This record was written afterwards to back-fill the design-approval gate for a delivered
brief that other work depends on: desk-tools/51 declares `desk-tools/52` in `depends`. It did
not precede the implementation. It is written on demand under the ruling on #5145, which limits
back-filled records to held briefs that block other work.

This record does not mint a decision. The options are taken from the brief and #4890; the
delivered behaviour is taken from #4898, and the interface contract from the brief's own
`## Interface contract`. Nothing here adds rationale
beyond what those record.

## The decision put to the driver

`docs/streams/desk-tools/brief-52-link-decision-and-verification-cards-without-conflating-their-authority.md`
asks the driver to confirm that the reviewed implementation preserves the current human-only
acceptance and source-provenance boundaries. The options are:

1. **Approve the concrete implementation after independent verification.**
2. **Request corrections and keep the change pending.**
3. **Hold the change.**

Default if no answer: none. The brief blocks until answered.

## What this record does not decide

It does not choose an option. It does not record a verification result. It does not move the
brief's board row, change who may close a decision or verify gate, or replay any historical
close. It does not decide desk-tools/51 or any other dependent.
