---
id: DR-desk-tools-49
date: "2026-11-16"
title: "Verifier filings deduplicated by canonical work identity (desk-tools/49): option 1, approve the delivered implementation after independent verification, is the proposed design; no driver ruling is recorded, so this record is placeholder-unratified"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Option 2 — request corrections and keep the change pending; option 3 — hold the change (the brief's `## Human decision` and decision issue #5835). Not ruled out: no driver ruling chooses between the options. The line `Hold the change.` after the brief's options is a leftover, not a choice (desk comment on #5835)."
  - "A global suppress-all-on-brief rule, or automatic issue closure (excluded by the brief's own `## Interface contract`)."
accepted:
  - "Proposed, not ruled. A filing key `fk1/<16 hex>`, a hash of repository, canonical brief id, raising role and case id, is stamped as a body marker; a caller-supplied marker is refused (#4229)."
  - "Proposed, not ruled. One exact filing-key search runs on the typed issue-search seam before the existing fuzzy title matching. A truncated or failed read is unknown, never absent; a hit is confirmed locally (open, exact marker, same role, trusted author). `deskfile new --filing-brief B --filing-case C` exits 5 on a hit and 6 on unknown; the legacy title matcher and the audited `--force-new --reason` path stay (#4229)."
  - "Proposed, not ruled. No private cache is added and the dry-run repair sink is not activated (the brief's `## Task`, item 4)."
  - "Under any option, approval grants no workflow activation or additional write scope (the brief's `## Human decision`), and independent verification stays a separate act; this record supplies neither."
---

## State and provenance

**Approval state: placeholder-unratified. This is not an approved design record.** No recorded
driver ruling exists for desk-tools/49. Its decision issue,
[issue #5835](https://github.com/medici-finance/assay/issues/5835), is open and carries no
driver comment choosing an option.

The `decided-by` stamp above is the public placeholder and names nobody
(`spec/registers-v1.md` §7.5). This record carries no `ruling:` link, because there is no driver
comment to link. A local `statusgen --corroborate` run is expected to report it
`placeholder-unratified` until the driver posts an unedited comment on #5835, from the driver's own account, naming
`desk-tools/49` and `DR-desk-tools-49` with the chosen option. The `ruling:` key is added by a
later push, only after that comment exists.

The brief's `design:` key is deliberately absent. The design-gate lint
(`spec/lifecycle-v1.md` §4.4) checks only that a cited record exists, never that it is approved,
so citing this record now would release the hold before the record is ratified. The `design:` key
is added in the same push that adds this record's `ruling:` link.

**Written after implementation.** desk-tools/49 was delivered by
[#4229](https://github.com/medici-finance/assay/pull/4229), merged 2026-10-24, before this record
existed. This record was written afterwards to back-fill the design-approval gate for a delivered
brief that other work depends on: desk-tools/51 and verification-handoff/02 each declare
`desk-tools/49` in `depends`. It did not precede the implementation. It is written on demand
under the ruling on #5145, which limits back-filled records to held briefs that block other work.

This record does not mint a decision. The options are taken from the brief and #5835; the
delivered behaviour is taken from #4229, and the interface contract and the no-cache rule
from the brief's own `## Interface contract` and `## Task`. Nothing here adds rationale
beyond what those record.

## The decision put to the driver

`docs/streams/desk-tools/brief-49-deduplicate-verifier-filings-by-their-canonical-work-identity.md`
asks the driver to confirm that the reviewed implementation preserves the current human-only
acceptance and source-provenance boundaries. The options are:

1. **Approve the concrete implementation after independent verification.**
2. **Request corrections and keep the change pending.**
3. **Hold the change.**

Default if no answer: none. The brief blocks until answered.

## What this record does not decide

It does not choose an option. It does not record a verification result. It does not move the
brief's board row or activate any workflow. It does not decide desk-tools/51,
verification-handoff/02 or any other dependent.
