---
id: DR-desktools-v2-73
date: "2026-11-16"
title: "deskevidence owns the on-behalf-of principal on Runner cells (desktools-v2/73): option 1, the restamp-or-refuse split, is the driver's chosen option; no comment names this record, so it is placeholder-unratified"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Refuse in every mismatch case — stricter, but it forces verifiers back into hand-editing cells (the brief's `## Human decision` and decision issue #4070). The driver's comment on #4070 chose option 1 over it."
accepted:
  - "Proposed record of the chosen option. Restamp a non-principal or non-`human:` annotation to the resolved subject; leave a matching one byte-identical; refuse with exit 5, writing nothing, an annotation naming a different recognised human (the driver's comment on #4070; delivered by #4075)."
  - "Proposed record of the delivered hardening. A landing whose added Evidence a Markdown renderer and the attribution lint would read differently is refused with exit 5, naming the line; a decorative lead before a human runner is refused too and lands with a plain Runner cell (#4102; `docs/on-behalf-of.md`)."
  - "Independent verification of the brief's Verify table stays a separate act; this record supplies none."
---

## State and provenance

**Approval state: placeholder-unratified. This is not an approved design record.** The driver
has chosen an option for desktools-v2/73: an unedited comment from the driver's own account on
[issue #4070](https://github.com/medici-finance/assay/issues/4070#issuecomment-6477323944),
posted 2026-10-28, names `desktools-v2/73` and option 1, the restamp-or-refuse split. That
comment does not name `DR-desktools-v2-73`, because this record did not exist when it was posted.
`statusgen --corroborate` requires the comment's own text to name the record, so it is expected
to report this record `placeholder-unratified` until the driver posts a comment naming
`DR-desktools-v2-73`.

The `decided-by` stamp above is the public placeholder and names nobody
(`spec/registers-v1.md` §7.5). This record carries no `ruling:` link; the `ruling:` key is added
by a later push, only after a DR-naming comment exists.

The brief's `design:` key is deliberately absent. The design-gate lint
(`spec/lifecycle-v1.md` §4.4) checks only that a cited record exists, never that it is approved,
so citing this record now would release the hold before the record is ratified. The `design:` key
is added in the same push that adds this record's `ruling:` link.

**Written after implementation.** desktools-v2/73 was delivered by
[#4075](https://github.com/medici-finance/assay/pull/4075), merged 2026-10-22, with the residual
hardening in [#4102](https://github.com/medici-finance/assay/pull/4102), merged 2026-10-29, both
before this record existed. This record was written afterwards to back-fill the design-approval
gate for a delivered brief that other work depends on: verification-handoff/06 declares
`desktools-v2/73` in `depends`. It did not precede the implementation. It is written on demand
under the ruling on #5145, which limits back-filled records to held briefs that block other work.

This record does not mint a decision. The decision text and the alternative are taken from the
brief and #4070; the delivered behaviour is taken from #4075 and #4102. Nothing here adds
rationale beyond what those record.

## The decision put to the driver

`docs/streams/desktools-v2/brief-73-make-deskevidence-own-the-on-behalf-of-principal-on-runner-cells.md`
asks, for acceptance at done, for the restamp-or-refuse split:

1. **The split (recommended in the brief).** Restamp when the added row's annotation names no
   recognised roster human or is not in the `human:` form; leave it when it names the subject the
   roster resolves; refuse (exit 5, nothing written) when it names a different recognised human.
2. **Refuse in every mismatch case.**

The driver chose option 1 on 2026-10-28. What remains is a comment naming this record.

## What this record does not decide

It does not record a verification result. It does not move the brief's board row. It does not
decide verification-handoff/06 or any other dependent.
