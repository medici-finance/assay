---
id: DR-scan-diff-reader
date: "2026-11-10"
title: "A branch-diff secret-scan refusal names its file, line and side through the one shared diff reader the outbound push check uses, with the scanned text proven byte-identical"
consequence: critical  # the transcriber's classification, not part of a ruling
decided-by: "human:ian"
ruling: "https://github.com/medici-finance/assay/issues/4284#issuecomment-6775569997"
alternatives:
  - "Add a separate reader for the report only, leaving the outbound check's reader untouched (brief 62's option 2, not taken). Brief 62's reason: two readers of diff structure would exist and could drift apart."
accepted:
  - "The shared reader is the outbound push check's input path for every added line. Extending it is a change to a credential control's input, accepted because a byte-identity test proves the scanned text is unchanged, a parity test proves the added lines match the outbound reader's, and the outbound check's own tests stay unedited."
  - "The change makes refusals easier to act on and changes no verdict. A defect it causes shows up as a misplaced report or a failed parity test, not as a new admission."
---

This record holds the ruling on how the desk's branch-diff secret scan locates a refusal. It
does not change what the scan refuses. That is DR-scan-admission (what is admitted) and
DR-secret-scan-scope (which lines are read).

## The decision

The ruling's own words, from the comment the `ruling:` key links: "A — restates the ruling at
#4284 comment 6774271356 (desk-tools/62 A: extend the shared diff reader, with a byte-identity
test and the outbound tests left unedited)."

A is option 1 of brief 62's Human decision, by order; the decision issue lists no lettered
options. The rest of this section is brief 62's design for that option, not the ruling's own
text.

Brief desk-tools/62 (`docs/streams/desk-tools/brief-62-diff-finding-file-line.md`) extends the
diff reader the outbound push check already uses, so that only one diff reader exists. It maps
each line of the stripped diff to its file, its line in that file, and its side: head for added
and context lines, base for removed lines. A refusal reports that location and never prints the
matched span. A test proves the scanned text is byte-identical to what deskpr scanned before,
and a parity test proves the added lines match the outbound reader's. The outbound check's own
tests stay unedited. Brief 62 cites this record with `design:`.

## Rulings

Each ruling is a comment by the human on the decision issue, unedited. Every ruling gets a row
here. The `ruling:` key above holds the newest ruling's link, and it must be a comment whose
text names this record's id. A pull request that adds a row moves that key to the new link, and
the earlier link stays in its row. A ruling that narrows or reverses an earlier one says so in
its row, and the earlier row is kept (the register is append-only).

The first row chose the option but named no record id, so it cannot be this record's `ruling:`.
The second row restates it under this record's id, and the `ruling:` key links it.

| Date | Scope | Ruling | Briefs |
|---|---|---|---|
| 2026-11-10 | Extend the shared diff reader, with a byte-identity test and the outbound tests left unedited. Option A on the decision issue, which is option 1 of brief 62's Human decision. History: it names no record id. | https://github.com/medici-finance/assay/issues/4284#issuecomment-6774271356 | desk-tools/62 |
| 2026-11-10 | Restates the ruling above under this record's id, with no change in scope. | https://github.com/medici-finance/assay/issues/4284#issuecomment-6775569997 | desk-tools/62 |

## Not decided here

- What the scan admits: DR-scan-admission.
- Which diff lines the branch-diff scan reads: DR-secret-scan-scope.
- The outbound push check's verdicts. This record changes none of them.
