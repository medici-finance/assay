---
id: DR-desk-tools-30
date: "2026-11-24"
title: "Verify desk lands verdicts by signed verdict issue where the lane is armed; direct deskevidence write is break-glass there (desk-tools/30): option 2"
consequence: major  # the transcriber's classification, not part of the ruling
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/2352#issuecomment-7137862829"
alternatives:
  - "Option 1, the signed-issue path everywhere: every project's verify desk lands through the lane and the direct write is break-glass. Not chosen: the ruling approves option 2. Its consequence: a project that has not armed the lane has no routine landing except break-glass or pull requests, so its verify desk stalls."
  - "Option 3, no change: the skill keeps the direct write as the landing path and the lane as a future pointer. Not chosen, for the same reason. Its consequence: the shared doctrine and an adopter's ruled cutover disagree, and the desks follow the shared text."
accepted:
  - "Where a project has armed the lane and bound a verdict filer, routine PASS verdicts land only through the signed verdict issue and `statusgen --transcribe-verdict`. No session pushes `main` on that path."
  - "Break-glass stays a rule the desk follows, not a control: `deskevidence` cannot tell armed from unarmed, and the session that sets `VERIFIER_MAIN_OK` also records the break-glass. Gating or removing the direct write is an unauthored follow-up."
  - "Evidence that is not PASS, outcome records and irreversible-brief Evidence take the residual PR-form route, which keeps its own recorded-ruling precondition."
  - "Where the lane is not armed, `deskevidence` stays the standing landing path, unchanged. A could-not-check read of the lane state holds and never falls through to the direct write."
---

The ruling is the driver's comment on the brief's decision issue, created on 24 November 2026 at 16:33:53 UTC and never edited: "DR-desk-tools-30: approved, option 2". It names this record, and the issue carries the decision-gate marker for desk-tools/30, whose `design:` cites this record (`registers-v1.md` §7.5, conditions 5 and 6).

An earlier human comment on the same issue picked option 2 for the brief and is the brief's own `ruling:`. It predates this record and does not name it, so it is not this record's ruling.

The design is the brief's option 2. It touches doctrine and docs only: the verify-desk skill's Landing section, the shared git-push-policy block, and a comment on the `deskevidence` rate-limit cap. It switches no lane on, sets no operator config, and changes no cap value, workflow or statusgen code. The `accepted:` entries above transcribe that design. The ruling text approves option 2 and says nothing further, so no entry is attributed to the human beyond that choice.

`major` ranks the consequence if this design is wrong. Routine verify landings could move to a lane a project has not proven, or the direct write could be read as still routine where the lane is armed. A fresh doctrine change reverses either. No ruling states a severity, so this field is not attributed to the human.
