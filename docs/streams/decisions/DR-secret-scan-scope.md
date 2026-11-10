---
id: DR-secret-scan-scope
date: "2026-11-10"
title: "deskpr's branch-diff secret scan reads added and removed lines, and skips unchanged context only when the diff's base is proven to be on the remote"
consequence: critical  # the transcriber's classification, not part of a ruling
decided-by: "human:ian"
ruling: "https://github.com/medici-finance/assay/issues/4284#issuecomment-6774347835"
alternatives:
  - "Keep reading the whole diff, context included. Ruled out: it keeps re-reporting credential-shaped text that sits unchanged next to an edit, which costs overrides and renames on text the change did not write."
  - "Read added lines only, the same scope as deskevidence and the outbound push check. Ruled out: deleting a leaked credential would raise no alarm, so nobody would be prompted to rotate it."
  - "Read added and removed lines, and on an update read only the commits the remote does not hold yet. Ruled out: content already pushed, including content pushed under an override, would never be judged again on later updates."
accepted:
  - "A credential already sitting unchanged on the remote's main branch, next to an edit, is no longer flagged again by this scan. Nothing else on the desk re-surfaces it."
  - "Context is skipped only when the diff's base is an ancestor of the default-branch head that the push destination itself reports. A base that was never pushed, or a remote read that fails, means context is scanned in full, so some context false positives stay on an unpushed or offline base."
---

This is the record for which lines of a branch diff deskpr's secret scan reads. It is about
scope, which lines are read at all. What the scan admits among the lines it reads is a
separate record, DR-secret-scan-admission (planned).

## The decision

The branch-diff scan reads the lines a change adds and the lines it removes. A removed
credential still raises the alarm, which is the prompt to rotate it. Unchanged context lines
around each hunk are skipped, but only when the diff's base is proven to be on the remote. That
proof is the push destination's own answer: the base is an ancestor of the default-branch head
that `git ls-remote` on the push URL returns. If that read fails, the ref is missing, the
commit is not present locally, or the base is not an ancestor, nothing is skipped and context
is scanned in full, as before this ruling. Tests prove both halves of that condition before
context is skipped.

The design is brief desk-tools/64 (`docs/streams/desk-tools/brief-64-diff-scan-scope.md`), whose
Verify rows pair each half of the condition with a mutant that must be caught.

## Rulings

Each ruling is a comment by the human on the decision issue, unedited, that names this record's
id in its own text. Every ruling gets a row here. The `ruling:` key above holds the newest
ruling's link, and an earlier link stays in its row.

| Date | Scope | Ruling | Briefs |
|---|---|---|---|
| 2026-11-10 | Scan added and removed lines, not unchanged context, with tests proving the base is on the remote before context is skipped. Option B on the decision issue, which is option 2 of brief 64's Human decision. | https://github.com/medici-finance/assay/issues/4284#issuecomment-6774347835 | desk-tools/64 |

## Not decided here

- What the scan admits among the lines it reads: DR-secret-scan-admission (planned).
- The remote check's breadth over commits. #6062 documented it, and this ruling keeps it.
- The scope of deskevidence and the outbound push check. Both already read added lines only.
