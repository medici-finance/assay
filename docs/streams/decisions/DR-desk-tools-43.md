---
id: DR-desk-tools-43
date: "2026-11-16"
title: "Review callers reuse one qualified observation (desk-tools/43): accepting the delivered explicit-opt-in path with legacy rollback is the proposed design; no driver ruling is recorded, so this record is placeholder-unratified"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Require all installed callers to use the new path immediately, or introduce an implicit shared snapshot source (the desk-decided alternative recorded on #4311). Not ruled out by the driver: no driver ruling exists, so this record does not say which path is taken."
  - "Hold or revise the delivered implementation (options 2 and 3 of the brief's `## Human decision`). Not ruled out: no driver ruling chooses between the options."
accepted:
  - "Proposed, not ruled. The monitor gains an explicit classified mode that collects one complete review observation and plans through the shared candidate-plus-priority planner; the default legacy mode stays compatible and is the rollback (#4311; the brief's `## Architecture pickup`)."
  - "Proposed, not ruled. One optional `deskboard throughput --review-observation` flag lets reports reuse a complete current population. It validates the collector's local policy digest, requested scope and five-minute cutoff; a failure blinds review depth and never launches a second review scan (the brief's `## Architecture pickup`; #4311)."
  - "Proposed, not ruled. Incomplete, quarantined, stale or incompatible inputs hold visibly; claims, trust, lane admission and fresh consequential checks keep their existing owners. No service, cache, new credential, executor switch, release, pin change or activation is added (#4311)."
  - "Under any option, the source specification's approval stays as recorded, broader fleet qualification stays with desk-tools/44, and independent verification of the brief's Verify table stays a separate act; this record supplies none of them."
---

## State and provenance

**Approval state: placeholder-unratified. This is not an approved design record.** No recorded
driver ruling exists for desk-tools/43, and the brief has no decision issue. Its `## Review`
records that the source specification's approval exists and that this brief adds implementation
acceptance only. The `## Human decision` section added beside this record puts that acceptance to
the driver.

The `decided-by` stamp above is the public placeholder and names nobody
(`spec/registers-v1.md` §7.5). This record carries no `ruling:` link, because there is no driver
comment to link. `statusgen --corroborate` is expected to report it `placeholder-unratified`
until the driver posts an unedited comment, from the driver's own account, naming
`desk-tools/43` and `DR-desk-tools-43` with the chosen option. The `ruling:` key is added by a
later push, only after that comment exists.

The brief's `design:` key is deliberately absent. The design-gate lint
(`spec/lifecycle-v1.md` §4.4) checks only that a cited record exists, never that it is approved,
so citing this record now would release the hold before the record is ratified. The `design:` key
is added in the same push that adds this record's `ruling:` link.

**Written after implementation.** desk-tools/43 was delivered by
[#4311](https://github.com/medici-finance/assay/pull/4311), merged 2026-10-24, before this record
existed. This record was written afterwards to back-fill the design-approval gate for a delivered
brief that other work depends on: desk-tools/44, forge-neutral/45 and graph-execution/27 each
declare `desk-tools/43` in `depends`. It did not precede the implementation. It is written on
demand under the ruling on #5145, which limits back-filled records to held briefs that block
other work.

This record does not mint a decision. The options are taken from the brief's `## Human decision`;
the delivered behaviour and the alternative are taken from the brief's `## Architecture pickup`
and #4311. Nothing here adds rationale beyond what those record.

## The decision put to the driver

`docs/streams/desk-tools/brief-43-migrate-review-callers-with-qualified-versions-and-rollback.md`
migrates review callers off repeated queue discovery onto one complete observation and the shared
planner, keeping a supported legacy recipe for rollback. The options are:

1. **Accept the delivered implementation.** The brief moves to `implemented` and independent
   verification proceeds.
2. **Hold.** The brief stays `in-progress` until a named condition is met.
3. **Revise.** Name the missing coverage or authority boundary; a follow-up change fixes it before
   acceptance.

Default if no answer: none. The brief blocks until answered.

## What this record does not decide

It does not choose an option. It does not record a verification result: the brief's Evidence is
the independent verifier's. It does not move the brief's board row, activate any installed cell,
or change a release or pin. It does not decide desk-tools/44, forge-neutral/45,
graph-execution/27 or any other dependent.
