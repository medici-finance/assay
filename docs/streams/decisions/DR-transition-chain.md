---
id: DR-transition-chain
date: "2026-11-10"
title: "Chain approved acceptance transitions link by link, and let a row approved for Command+Expect replace its whole Command cell"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/6064#issuecomment-6773099410"
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

**Status:** draft — the pull request that adds this record does not mark it approved. Approving a
design record on a `gate: human` brief is the driver's act; a model or a worker cannot set it.
The driver's merge of that pull request ratifies the record.

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

`ruling:` above links the Q1 answer, because Q1 is the point that goes past the letter of the
#5211 ruling. `decided-by` keeps the placeholder a public repository uses, and the `ruling:`
link is what names the human.

`statusgen --corroborate` passes a ruling only when the comment's own text names this record's
id and the issue body carries a decision-gate marker for this record or for statusgen/40. None
of the three answers names `DR-transition-chain`, and #6064's body carries no decision-gate
marker. So this record does not corroborate yet. To ratify it, the driver posts one new comment
on #6064 that names `DR-transition-chain` and confirms Q1 to Q3, #6064's body gains the marker,
and this record's `ruling:` links that new comment instead.

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
