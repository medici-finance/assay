---
id: DR-witness-read-sets
date: "2026-10-26"
title: "Judge a verification witness against the read set its run recorded, behind a go compile-input floor"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/4512#issuecomment-6420851894"
alternatives:
  - "Derive an input set for existing go rows from their commands, with no re-run. Ruled out: it releases none of the 36 items held at authoring time, because each has other row shapes or edited or dirty witnesses, and it misses files that tests open at run time, so a stale result can pass."
  - "Add no new mechanism, and have the verify desk re-run the rows at the head of main before each flip. Ruled out: the check is not narrowed, so every unrelated merge still costs a re-run and gate:model briefs stay held at verified."
accepted:
  - "A witness whose row carries a recorded read set is judged only against the paths in that set. An under-recorded set could let a stale witness pass. The go compile-input floor catches that for go rows using a different signal: statusgen runs the go tool over the witness tree."
  - "A read set counts only when it arrived in the same commit as its row. A set evaluated with no floor is could-not-check, and so is a run off Linux or any input the recorder cannot attribute. Each of these keeps today's whole-tree check."
  - "Rows that read repository history stay on today's manual route."
  - "The verify desk re-runs the already-witnessed items once, at one main revision."
  - "The command-text input parser stays until a measurement shows it adds nothing."
---

**Status:** draft. The pull request that adds this record does not mark it approved. Approving a
design record on a `gate: human` brief is the driver's act; a model or a worker cannot set it.
The driver's merge of that pull request ratifies the record.

## Basis

The decision text above comes from the `## Human decision` section of
[build-less-brittle/15](../build-less-brittle/brief-15-witness-inputs-are-recorded-not-parsed.md)
and from that brief's decision issue, #4512, where the driver ruled option 1 on 2026-10-26.
`ruling:` above links that comment. `decided-by` keeps the placeholder a public repository uses,
and the `ruling:` link is what names the human.

The author of this record could not read the ruling comment, so the record does not quote it.
`statusgen --corroborate` passes a ruling only when the comment's own text names this record's id.
If the comment does not name `DR-witness-read-sets`, the driver posts a new ruling comment that
does, and this record links that comment instead.

## What this record does not decide

It does not stand in for the brief's `gate: human` acceptance at done. It does not loosen the
command-text guard, the Expect guard, the witness landing check, or the conservative scope for a
`+dirty` or `+unknown` tree. The only `+dirty` change is that a brief's own new read-set files,
named by its own Evidence, join that brief's own-brief exemption. No flag, environment variable
or override can switch off the floor, default-deny recording or the introduction binding.

`consequence: major`: if this design is wrong, a stale witness could release a brief to done
after a change it read but did not record.
