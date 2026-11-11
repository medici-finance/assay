---
id: DR-transition-chain
date: "2026-11-10"
title: "Chain approved acceptance transitions link by link, and let a row approved for Command+Expect replace its whole Command cell"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/6064#issuecomment-6773695768"
alternatives:
  - "Keep the one-span Command rule for rows approved for both cells (Q1, option 2). Ruled out: the first table waiting on this change, desk-containers/17, has two rows whose Command cells are rewritten whole while their Expect changes, so they would still refuse and that brief would have to be re-derived at a new delivery."
  - "Require every link of every chain, including the first, to carry a new approval in the chain format (Q2, option 2). Ruled out: the held statusgen/36 correction already has an approval in today's one-link format and needs a second link; refusing it would mean asking for that approval again for no added binding."
  - "Cap the number of links in a chain (Q3, option 2). Ruled out: each link already needs its own human approval, which bounds a chain, and a cap would force a longer chain to re-derive at a new delivery."
accepted:
  - "A row approved with the token `Command+Expect` may replace its whole Command cell. Its row ID, order, class and shell stay fixed. A row approved for Command alone keeps today's one-span rule."
  - "The first link of a chain may cite an existing one-link approval. It is judged under today's one-link rules, with no combined cells. Every later link uses the chain format."
  - "A chain has no length cap. Each link needs its own human approval, bound to that link's position, predecessor and table."
  - "One-line (v1) acceptance-transition records keep today's verdicts byte-for-byte."
---

**Status:** routed — the driver approved this record on 2026-11-10 in the `ruling:` comment above, and statusgen/40 cites it.
**Routes-to:** docs/streams/statusgen/

## Basis

The decision text above comes from the `## Human decision` section of
[statusgen/40](../statusgen/brief-40-chain-approved-acceptance-transitions.md) and from the
driver's answers on its scoping issue, #6064. Each answer was posted from the driver's own
account on 2026-11-10 and has not been edited. Each picks option 1, the recommended option, of
its question (the comments call it "A"):

- Q1: A (whole-cell Command replacement on a `Command+Expect` row; row ID, order, class and shell fixed): https://github.com/medici-finance/assay/issues/6064#issuecomment-6773099410
- Q2: A (a one-link approval may serve as link 1, judged under the one-link rules with no combined cells): https://github.com/medici-finance/assay/issues/6064#issuecomment-6773126747
- Q3: A (no chain-length cap; each link needs its own human approval): https://github.com/medici-finance/assay/issues/6064#issuecomment-6773147331

The direction they settle is the driver's earlier ruling on #5211, option B:
https://github.com/medici-finance/assay/issues/5211#issuecomment-6771619511

## Ruling

On 2026-11-10 the driver approved this record on #6064, the issue whose body carries this
record's decision-gate marker. The comment is unedited and was posted from the driver's own
account. Its text reads:

> DR-transition-chain: approved — Q1 A, Q2 A, Q3 A as recorded above.

That comment is this record's `ruling:`. It names the record, so `decided-by` above stays the
public-repository placeholder and is confirmed through that link (`spec/registers-v1.md` §7.5).
No bot relay or PR approval stands in for it. The three answer comments above stay the
record of what each answer was.

## What this record does not decide

It does not stand in for the brief's `gate: human` acceptance at done. It does not decide the
chain record format, the per-link payload binding or the refusals; those are design in the
brief's Task section and are reviewed there. It does not loosen the approval reader, the
Evidence preservation rule, the version-increment rule or the fresh-witness audit. Row
retirement, class and shell changes, row add, delete and renumber, supplemental-row promotion
and table-layout migration stay refused. The held statusgen/36 proposal's own reuse questions
stay with that proposal.

`consequence: major`: if this design is wrong, a protected row could be credited against a
Command or Expect that no human approved.
