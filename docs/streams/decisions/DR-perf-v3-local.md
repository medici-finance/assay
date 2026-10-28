---
id: DR-perf-v3-local
date: "2026-10-19"
title: "Accept bounded local perf-v3 attribution with explicit workflow consent"
consequence: major
decided-by: "human:ian"
ruling: "https://github.com/medici-finance/assay/issues/3536#issuecomment-6282045526"
alternatives:
  - "Hold for correction of a field, lifecycle or failure boundary: this was the second offered option; the human selected acceptance instead."
  - "Collect workflow linkage in tool-only or implicit legacy records: excluded by the accepted profile and consent boundary."
  - "Add remote collection to this local contract: excluded from this decision and retained behind separate activation controls."
accepted:
  - "The explicit workflow profile may retain bounded, validated local work and agent linkage; opt-out and tool-only use cannot retain that linkage."
  - "Invalid optional metadata and record-write failures preserve the underlying operation's result; metadata grants no operational authority."
  - "The existing recorder owns the versioned contract; legacy meanings remain separate and unavailable measurements are not reported as measured zero."
  - "This acceptance covers the demonstrated local disclosure and isolation boundary, while independent review, verification and remote activation controls remain separate."
---

This record transcribes the human's option 1 comment on the closed decision issue.
It records an existing decision and makes no new approval or verification claim.

https://github.com/medici-finance/assay/issues/3536#issuecomment-6282045526

The accepted local contract is described in
[performance attribution](../../performance-attribution-spec.md) and implemented
by desktools-v2/63. Its explicit profiles, bounded field validation, restrictive
local storage and retention prevent tool-only records from silently acquiring
workflow identity. Downstream propagation and provider adapters retain their own
delivery and acceptance obligations.

`consequence: major` records the local privacy consequence of incorrectly retaining
identifiable work linkage. It does not grant permission to upload it, weaken a
control, change credential custody or skip independent review and verification.
