---
id: DR-statusgen-28
date: "2026-11-16"
title: "Historical notices (statusgen/28): one shared notice report collapses findings on briefs closed at the merge-base into per-rule historical totals, with --all detail and a --lint --json report; proposed, not ratified"
consequence: major # The transcriber's classification.
decided-by: "human:<name>"
alternatives:
  - "A second, independent notice-classification table beside the differential. Ruled out by the brief's Task 3: one reporting and normalization owner is shared with the differential."
  - "Rewriting or deleting closed evidence to quiet the notices. Ruled out by the brief's ground rules and Task 3: recorded Evidence and frozen closed commands are not rewritten, and a rising historical count is distinguished from a newly closed defective brief rather than falsified."
  - "Hiding ordinary PROBLEMs or changing their exit code. Ruled out by the brief's Task 2."
  - "Blanket-exempting active documents or creating historical security-review records. Ruled out by the brief's Task 4; only docs/archive is excluded, and from live lifecycle classification only."
accepted:
  - "`security-review-absent` is counted per rule but held actionable, never demoted: closure at the merge-base does not show the closure predates the rule, and the corroborated closure-date boundary (statusgen/31) was not built when it was delivered (the delivering pull request)."
  - "By default, findings on a brief closed at the merge-base no longer print individually; `--lint --all` restores every one as its own NOTICE line (the brief's Task 2)."
  - "The three private closed-brief aggregates in the Verify-row audit are removed; each finding is an ordinary per-record line counted by the shared report (the delivering pull request)."
  - "Historical counts can rise when a new detector discovers old debt; that is history becoming visible, not a newly defective closed brief (the brief's Task 3 and the delivering pull request's notes)."
---

## State and provenance

**Approval state: proposed, not ratified.** The `decided-by` stamp above is the public
placeholder and names nobody. This record carries no `ruling:` link, so `statusgen --corroborate`
reports it `placeholder-unratified` until the driver posts, on the decision issue, an unedited
comment from the driver's own login that names `DR-statusgen-28`. The `ruling:` link to that comment, and
the brief's `design:` key, are added only after it exists.

**No human decision is recorded yet.** The brief's decision issue,
[#5855](https://github.com/medici-finance/assay/issues/5855), is open with no ruling. The brief's
`## Human decision` offers three options: approve the implementation as verified, request the
identified corrections before approval, or hold the change. Its stated default is none: it
blocks until answered, and no sign-off is inferred. This record does not choose among them.

**Written after implementation.** statusgen/28 was delivered by
[#4140](https://github.com/medici-finance/assay/pull/4140), merged 2026-10-24, before this record
existed. It is written on demand under the ruling on #5145, which limits back-filled design
records to held briefs that block other work: statusgen/30, statusgen/31 and statusgen/35 each
declare `depends:` on statusgen/28, and all three are `todo`.

This record does not mint a decision. It is taken from the brief, the delivering pull request and
the code on main; nothing here adds rationale beyond what those record.

## The design

As the brief asks and #4140 delivered:

1. `statusgen/notice_report.go` is the single owner of notice classification: a closed rule
   inventory, the standing of the brief record a finding lands on, and the text and JSON
   renderings. A notice belongs to the file in the emitter's leading slot, never to a path or tag
   quoted inside it.
2. A finding about a brief already `done` or `verified` at the merge-base is counted in a
   `[historical-notices]` per-rule total. Active, newly closed, unknown-status and non-brief
   findings keep their individual NOTICE lines.
3. `--lint --all` lists every historical finding; `--lint --json` prints a deterministic report
   with per-rule and per-status counts, the actionable and historical partitions, and
   could-not-check states, which also print as `NOTICE: [notice-report-could-not-check]` lines.
4. `docs/archive` is excluded from live lifecycle classification only. PROBLEMs, exit codes and
   history are unchanged.

The brief and its delivering diff are the authority; this summary does not amend either.

## What this record does not decide

It does not choose among the brief's three decision options. It does not record a verification
result or move the brief's board row. It does not decide statusgen/30, statusgen/31 or
statusgen/35.
