---
id: DR-scan-admission
date: "2026-11-10"
title: "The shared secret scan admits a high-entropy run only through a declared benign-shape registry, each entry bounded by refusals it must keep; the existing admissions move into it unchanged"
consequence: critical  # the transcriber's classification, not part of a ruling
decided-by: "human:ian"
ruling: "https://github.com/medici-finance/assay/issues/4284#issuecomment-6775008196"
alternatives:
  - "Adopt the registry for new shapes only and leave the existing predicates as code (brief 61's option 2, not taken). Brief 61's reason: the unaudited part stays as it is, and the scan keeps two ways to admit a run."
  - "Keep fixing one false positive at a time with one more predicate or closed-list word (brief 61's option 3, not taken). Issue 4284's reason: that is the mechanism that produced the class. It has meant ten widenings since August, three of which had to be narrowed again after a widening admitted credential-shaped material."
  - "Raise the run length or the entropy threshold globally. Rejected in the class analysis on issue 4284: it admits real credentials of the same length as the benign shapes."
  - "Exempt whole files by path. Rejected in the class analysis on issue 4284: a credential pasted into an exempt file would pass."
  - "Admit an exact-string list of vendor documentation example credentials (brief 63's option 3, not taken)."
accepted:
  - "The migration is one large change to a credential control. Its safety rests on the recorded-verdict golden showing zero verdict change between the old and the new scanner, run against the delivery-base tree, and on a human reading that evidence before any later brief widens an entry."
  - "Every new benign shape costs one registry entry with at least two bounding refusals and its fixtures. This is more work per shape than adding a predicate, and it is accepted as the price of an auditable list."
  - "The registry admits only what its entries describe. A benign shape no entry covers keeps being refused until a later ruling on this record adds it, so some false positives stay open until then."
---

This is the capability-level record for what the desk's shared secret scan may admit. It holds
the rulings on the admission mechanism and on the first set of entries. Later rulings on which
shapes the registry admits are added to this same record (see "Rulings" below), rather than
each getting a record of its own.

## The decision

The ruling's own words, from the comment the `ruling:` key links: "A — restates the rulings at
#4284 comments 6774184501 (desk-tools/61 A: declared benign-shape registry, existing admissions
moved unchanged, zero-verdict-change golden) and 6774321215 (desk-tools/63 A: the four open
shape groups with paired refusals plus the word-shaped entropy cap; no vendor-example allowlist;
labelled 32-hex md5 stays refused)."

Each letter is the first option of the brief it names. The rest of this section is the design
of those options as briefs 61 and 63 state it. It is not the ruling's own text.

- Brief desk-tools/61 (`docs/streams/desk-tools/brief-61-benign-shape-registry.md`), the
  mechanism. The scan refuses every run of 32 or more base64-alphabet characters. A run is
  admitted only when an entry in one declared registry of benign shapes recognises it. Each
  entry names its context, the issue that added it, the examples it admits and the refusals
  that bound it. A test proves, entry by entry, that widening the entry flips one of its
  bounding refusals. Every admission that exists today moves into the registry unchanged. A
  recorded-verdict golden, run against both the old and the new scanner, proves that no verdict
  changed.
- Brief desk-tools/63 (`docs/streams/desk-tools/brief-63-registry-acceptance-shapes.md`), the
  entries. Go build-info hashes, platform-suffixed digests from a closed architecture set, sops
  armor anchored to its own file, and Kubernetes qualified keys with `+`-joined identifiers each
  become an entry with paired refusals. A cap stops word-shaped admission from clearing
  high-entropy text.

Both briefs cite this record with `design:`.

## Rulings

Each ruling is a comment by the human on the decision issue, unedited. Every ruling gets a row
here. The `ruling:` key above holds the newest ruling's link, and it must be a comment whose
text names this record's id. A pull request that adds a row moves that key to the new link, and
the earlier link stays in its row. A ruling that narrows or reverses an earlier one says so in
its row, and the earlier row is kept (the register is append-only).

The first two rows named this record by an earlier id, DR-secret-scan-admission. That id has a
21-character slug, and the register allows at most 20. The third row restates both of them
under this record's id, and the `ruling:` key links it.

| Date | Scope | Ruling | Briefs |
|---|---|---|---|
| 2026-11-10 | Mechanism: adopt the declared registry, with every existing admission moved in unchanged and proven by a zero-verdict-change golden. Option A on the decision issue, which is option 1 of brief 61's Human decision. History: it names the earlier id. | https://github.com/medici-finance/assay/issues/4284#issuecomment-6774184501 | desk-tools/61 |
| 2026-11-10 | Entries: the four open shape groups with paired refusals, plus the word-shaped entropy cap. No vendor-example list. The labelled 32-hex digest stays refused. Option A on the decision issue, which is option 1 of brief 63's Human decision. History: it names the earlier id. | https://github.com/medici-finance/assay/issues/4284#issuecomment-6774321215 | desk-tools/63 |
| 2026-11-10 | Restates the two rulings above under this record's id, with no change in scope. | https://github.com/medici-finance/assay/issues/4284#issuecomment-6775008196 | desk-tools/61, desk-tools/63 |

## Not decided here

- Which diff lines the branch-diff scan reads. That is scan scope, not admission, so it is
  DR-secret-scan-scope.
- The literal credential arms. They are refusals, not admissions, and no ruling on this record
  changes them.
- The labelled 32-hex digest, which stays refused per the ruling on issue 3280.
