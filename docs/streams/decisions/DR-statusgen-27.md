---
id: DR-statusgen-27
date: "2026-11-16"
title: "Notice ratchet (statusgen/27): the existing --diff-base differential promotes an actionable NOTICE newly introduced on an added, edited or renamed brief to a PROBLEM, compared structurally by rule, path and row; proposed, not ratified"
consequence: major # The transcriber's classification.
decided-by: "human:<name>"
alternatives:
  - "A separate lint command for new notices. Ruled out by the brief's Task 1, which extends the existing differential instead."
  - "Comparing rendered NOTICE lines as text. Ruled out by the brief's Task 1: explanatory wording and temporary absolute roots must not create a new finding, so comparison is on normalized rule, repository-relative path and row identity."
  - "Promoting every newly introduced NOTICE. Ruled out by the brief's Task 2: historical summaries, environment and eligibility unknowns, operational debt counters and informational lines are classified explicitly and stay NOTICE, so they cannot become authoring failures by accident."
  - "Reporting a clean result when the base comparison is unavailable. Ruled out by the brief's Task 3: an unavailable comparison promotes nothing, keeps the existing full-strength PROBLEMs and prints an explicit could-not-check line."
accepted:
  - "Notices carried over from the merge-base stay NOTICE; only newly introduced actionable findings on changed briefs fail the run (the brief's `why:`)."
  - "An edit to a brief already closed at the merge-base is compared finding by finding only under `--all`, because by default statusgen/28 collapses those findings into path-less historical totals (the delivering pull request and `docs/verify-row-strength.md`)."
  - "The gate is not purely stricter: the `normalizeDiffProblem` dot-root fix lets a base-side PROBLEM whose line contains a dot match and be demoted to NOTICE, as the `--diff-base` contract documents (the delivering pull request's security note)."
  - "Some NOTICE emitters are not reachable from a small fixture; they are covered by the classifier patterns and planted lines, not by their real output (the delivering pull request's could-not-check note)."
---

## State and provenance

**Approval state: proposed, not ratified.** The `decided-by` stamp above is the public
placeholder and names nobody. This record carries no `ruling:` link, so `statusgen --corroborate`
reports it `placeholder-unratified` until the driver posts, on the decision issue, an unedited
comment from the driver's own login that names `DR-statusgen-27`. The `ruling:` link to that comment, and
the brief's `design:` key, are added only after it exists.

**No human decision is recorded yet.** The brief's decision issue,
[#5854](https://github.com/medici-finance/assay/issues/5854), is open with no ruling. The brief's
`## Human decision` offers three options: approve the implementation as verified, request the
identified corrections before approval, or hold the change. Its stated default is none: it
blocks until answered, and no sign-off is inferred. This record does not choose among them.

**Written after implementation.** statusgen/27 was delivered by
[#4134](https://github.com/medici-finance/assay/pull/4134), merged 2026-10-24, before this record
existed. It is written on demand under the ruling on #5145, which limits back-filled design
records to held briefs that block other work: statusgen/29 and statusgen/35 each declare
`depends:` on statusgen/27, and both are `todo`.

This record does not mint a decision. It is taken from the brief, the delivering pull request and
the code on main; nothing here adds rationale beyond what those record.

## The design

As the brief asks and #4134 delivered:

1. `runDiffLintOne`, the existing `--lint --diff-base` differential, gains a notice ratchet. No
   new command is added.
2. Within the ratchet, one classifier in `statusgen/noticeratchet.go` (`parseLintNotice`) turns
   each rendered NOTICE into a structured finding (rule, repository-relative path, Verify row or
   brief-level subject); `evaluateNoticeRatchet` compares head against base on that structure. A
   renamed brief is compared with its old path. The ratchet's rule membership lives in its own
   tables in that file (`ratchetRowRules`, `ratchetBriefRules` and `ratchetUntagged`), beside
   statusgen/28's `noticeRuleInventory` in `statusgen/notice_report.go`, which #4140 delivered
   before #4134 merged; none of the ratchet's tables reads that inventory. This record discloses
   the two tables and does not say whether that is a defect; that stays with the driver's
   decision on #5854. DR-statusgen-28 records the historical-notice report's side.
3. The actionable set and the classified non-actionable set are documented in
   `docs/verify-row-strength.md`. A newly introduced actionable finding prints as
   `PROBLEM: [notice-ratchet] ...` and counts toward the exit code.
4. The changed-brief listing (seam `changedBriefs`) accepts only a full object id as its base;
   anything else is a could-not-check and never reaches git.

The brief and its delivering diff are the authority; this summary does not amend either.

## What this record does not decide

It does not choose among the brief's three decision options. It does not record a verification
result or move the brief's board row. It does not decide statusgen/29 or statusgen/35.
