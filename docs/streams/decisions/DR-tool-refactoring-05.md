---
id: DR-tool-refactoring-05
date: "2026-11-21"
title: "One verification-outcome protocol (tool-refactoring/05): the outcome vocabulary, row reduction and record codec are defined once in statusgen/evaluation and every writer and reader calls them; proposed, not yet ratified"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/7061#issuecomment-7066059966"
alternatives:
  - "Option 2 on #7061: keep the existing behaviour and request a narrower migration. Not chosen: the ruling on #7061 picks option 1."
  - "Keep the record codec in statusgen/evaluation/outcome.go and widen the library's pure-dependency allow-list to admit JSON, io/fs and time. Ruled out: it weakens a guard (TestEvaluationPureDependencyBoundary)."
  - "Keep a codec copy in each module (statusgen and deskkit). Ruled out: two hand-synced copies are the defect class this brief retires."
accepted:
  - "Proposed, not yet ratified. The vocabulary, the row reduction and the brief-key grammar live in statusgen/evaluation/outcome.go. The record codec lives in its subpackage statusgen/evaluation/outcomerecord, which takes an fs.FS and an explicit now. statusgen and deskkit keep only root and wall-clock adapters (#7067)."
  - "Proposed, not yet ratified. deskevidence --outcome-record refuses a verified record that holds or fails rows. This is the one new writer refusal, and it applies only to new writes (#7067)."
  - "Proposed, not yet ratified. The word pass no longer counts as an outcome pass in the verified-ends-credited attempts check. A stored legacy pass record whose attempts do not end credited is no longer refused by the deskperf rerun rollup, which is diagnostic (#7067)."
  - "The record layout, numeric exit codes, wake-receipt schema and gate policy are unchanged. No independent current-state, custody or conditional-write check is removed or moved."
  - "TestOutcomeProtocolClassGuard is a lexical drift detector, not an enforcement point: a reversed operand, an EqualFold copy or a private reducer would pass it."
---

## State and provenance

**Approval state: proposed, not yet ratified.** The decision issue,
[issue #7061](https://github.com/medici-finance/assay/issues/7061), has a ruling comment from the
driver's own account choosing option 1: approve the bounded refactor once its independent class
and caller controls pass. That comment is the `ruling:` link above. It does not name this record's
id, so `statusgen --corroborate` is expected to report `record-not-named` until the driver posts a
new, unedited comment on #7061 that names `tool-refactoring/05` and `DR-tool-refactoring-05`. The
`ruling:` link then moves to that comment.

The `decided-by` stamp is the public placeholder and names nobody (`spec/registers-v1.md` §7.5).

The brief's `design:` key is deliberately absent. The design-gate lint
(`spec/lifecycle-v1.md` §4.4) checks only that a cited record exists, not that it is approved, so
citing this record now would release that hold before the record is ratified. The `design:` key is
added in the same push that re-points `ruling:` to a comment naming this record.

**Written after implementation.** The design was implemented in
[#7067](https://github.com/medici-finance/assay/pull/7067) before this record existed. This record
back-fills the design-approval gate for that change. It adds no rationale beyond what the brief,
#7061 and #7067 record.

## The decision

`docs/streams/tool-refactoring/brief-05-share-outcome-decoding-and-reduction-between-verification-producers-and-readers.md`
asks whether to replace the copies of the verification-outcome protocol with one shared definition.
The copies are a private outcome vocabulary, exact-spelling comparisons, a second brief-key grammar,
a second row-result vocabulary, and two hand-synced record codecs. Each copy lets a writer produce a
record, or a guard accept one, that another reader reads differently. The options on #7061:

1. **Approve the bounded refactor after its independent class and caller controls pass.**
2. **Keep the existing behaviour and request a narrower migration.**

The ruling chooses option 1.

## What this record does not decide

It does not record a verification result. It does not move the brief's board row. It does not
decide fixed-delivery policy, PASS reuse or wake deduplication (desk-tools/46 to 51), or any brief
that depends on tool-refactoring/05.
