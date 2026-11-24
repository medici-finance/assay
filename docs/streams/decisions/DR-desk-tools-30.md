---
id: DR-desk-tools-30
date: "2026-11-24"
title: "Verify desk lands verdicts by signed verdict issue where the lane is armed; direct deskevidence write is break-glass there (desk-tools/30): option 2 is the proposed design; no ruling naming this record is recorded yet, so this record is placeholder-unratified"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Option 1, the signed-issue path everywhere: every project's verify desk lands through the lane and the direct write is break-glass. Ruled out by the decision-issue comment, which picks option 2. Its consequence: a project that has not armed the lane has no routine landing except break-glass or pull requests, so its verify desk stalls."
  - "Option 3, no change: the skill keeps the direct write as the landing path and the lane as a future pointer. Ruled out by the same comment. Its consequence: the shared doctrine and an adopter's ruled cutover disagree, and the desks follow the shared text."
accepted:
  - "Proposed, not yet ruled by a comment naming this record. Where a project has armed the lane and bound a verdict filer, routine PASS verdicts land only through the signed verdict issue and `statusgen --transcribe-verdict`. No session pushes `main` on that path."
  - "Proposed, not yet ruled. Break-glass stays a rule the desk follows, not a control: `deskevidence` cannot tell armed from unarmed, and the session that sets `VERIFIER_MAIN_OK` also records the break-glass. Gating or removing the direct write is an unauthored follow-up."
  - "Proposed, not yet ruled. Evidence that is not PASS, outcome records and irreversible-brief Evidence take the residual PR-form route, which keeps its own recorded-ruling precondition."
  - "Proposed, not yet ruled. Where the lane is not armed, `deskevidence` stays the standing landing path, unchanged. A could-not-check read of the lane state holds and never falls through to the direct write."
---

The brief's decision issue (medici-finance/assay#2352) carries a human comment, created on 23 November 2026 at 16:02:04 UTC and never edited. It picks option 2: "signed-issue path where switched on; direct write stays elsewhere and is break-glass where the lane is on." That comment rules the brief's `## Human decision` and is the brief's `ruling:`. It was posted before this record existed and does not name it, so it cannot ratify this record (`registers-v1.md` §7.5, condition 5). This record carries no `ruling:` link and does not create a ruling.

The design is the brief's option 2 as the comment states it. It touches doctrine and docs only: the verify-desk skill's Landing section, the shared git-push-policy block, and a comment on the `deskevidence` rate-limit cap. It switches no lane on, sets no operator config, and changes no cap value, workflow or statusgen code.

`major` ranks the consequence if this design is wrong. Routine verify landings could move to a lane a project has not proven, or the direct write could be read as still routine where the lane is armed. A fresh doctrine change reverses either. No ruling states a severity, so this field is not attributed to the human.

To ratify the record, the driver posts a comment on the decision issue naming `DR-desk-tools-30`. The record then gains that comment's URL as `ruling:`.
