---
id: DR-forge-neutral-19
date: "2026-09-18"
title: "Merge to the protected branch is not server-side enforced today; close the gap with the `human-approved` required status check the brief specifies"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Close the gap with a Code-Owners review requirement instead of a required status check — ruled out: put to the driver as option (B) on #1256 and not taken; the ruling selected (A). The brief's own reasoning is that the merge gap must close on a SERVER-SIDE control the ruleset itself enforces; the `human-approved` check is the shape the brief specifies (trusted-human login, `APPROVED` state, `commit_id` at the PR's head), and the ruling adopts that shape rather than a review-requirement variant."
  - "Accept the characterisation but defer the fix as accepted risk — ruled out: put to the driver as option (C) on #1256 and not taken; the ruling selected (A). The brief's `## Context` names the merge surface's single point of failure as every desk App's own restraint with NO second layer behind it today, and a deferral would leave that as the only control — which the brief itself says is not acceptable alone (its Review question 1)."
accepted:
  - "The characterisation stands as written: merge to the protected branch is not server-side enforced today — `pull_requests: write` suffices, and the `protect-main` ruleset's approving-review count carries no restriction on which identity supplies the approval. Until the fix lands, the only control on that surface remains each desk App's documented restraint."
  - "The fix shape is fixed to the `human-approved` required status check as the brief's Task 1 specifies (trigger `pull_request_review`; trusted-human login on a repo-scoped, human-editable allow-list; `APPROVED`; `commit_id` equal to the PR's current head; `success`/`failure` status posted, never nothing on a reviewed-but-failing case; workflow token `statuses: write` + `pull-requests: read` only)."
  - "The workflow file (`.github/workflows/human-approved-gate.yml` (planned)) and the ruleset entry adding `human-approved` to `protect-main`'s required checks stay the repo admin's own follow-on acts, per the brief's Ground rules — neither is performed by the brief, by this record, or by any desk tool."
  - "The brief's `## Evidence` rows may now be filled; Verify rows 3 and 4 remain fixture-repo-only and row 5 is a future re-run after the ruleset edit lands, exactly as the brief's Verify table states."
---

**Ruling recorded (2026-09-18): the characterisation stands and the fix shape is the
`human-approved` required status check — answer A.** The driver (`human:<name>`) ruled on
the brief's implementing pull request,
[PR #1256](https://github.com/medici-finance/assay/pull/1256) — the
[ruling comment](https://github.com/medici-finance/assay/pull/1256#issuecomment-5737614997)
(2026-09-18T23:58:39Z, relayed by the desk from the driver). The question was put as three
options: (A) accept the characterisation plus the `human-approved` required-check design,
with the desk authoring this record for approval-by-merge; (B) accept the characterisation
and close the gap with a Code-Owners review requirement instead; (C) accept the
characterisation and defer the fix as accepted risk. The recorded answer is **A**. This
record transcribes that ruling into the register; it does not mint a new one — the human act
is the driver's answer relayed on #1256 and the driver's merge of the pull request that lands
this file, not this file itself.

**The decision.** `docs/streams/forge-neutral/brief-19-human-only-surfaces-server-side.md`
states, surface by surface, which human-only actions are backed by a permission the platform
refuses to grant and which are backed only by every App's own restraint. Its item 1 finds
the merge-to-protected-branch surface in the second class: the role Apps hold `contents:
write` + `pull_requests: write`, which together suffice to merge, and the live ruleset read
recorded in the brief's `sources:` shows the `protect-main` approving-review requirement does
not restrict which identity may supply the approval. The ruling confirms that
characterisation and adopts the brief's proposed fix: a `human-approved` commit status,
posted by a workflow that checks the reviewing login against a trusted-human allow-list at
the PR's head SHA, added by a human to the ruleset's required-status-checks list so the
ruleset itself refuses a merge no listed human approved at head.

**The constraint behind it.** The brief's `single-point-of-failure` line names the one
control on this surface today — every desk App's restraint (`deskflip`'s convention) — and
states there is NO second layer behind it. The fix is deliberately a server-side control
rather than a second convention, because a second convention shares the first one's failure
mode: a well-behaved actor honours it and a compromised or mis-configured one does not. That
is why option (C), deferral, was put and not taken, and why the adopted shape is a required
status check the ruleset enforces.

**What this record does not decide.** It does not land the workflow file or perform the
ruleset edit — both stay the repo admin's follow-on acts, and the workflow's own first push
is a human-reviewed act under the `workflows` scope the brief's item 2 describes. It does
not attest that the `human-approved` check, once implemented, cannot be satisfied by a bot
posing as a trusted human — the brief's pre-mortem names that as the first row the
follow-on's own Verify table must cover. It does not, by itself, prove the approver is a
different identity from the brief's author — the same attribution-not-identity limit
`lifecycle-v1.md` §7.1.2 declares for verification, and `registers-v1.md` §7.4 for this
register. And it does not attest the design is correct: that the alternatives were weighed
is recorded here; whether the check closes the gap is the brief's Verify rows 3-5 once the
follow-on lands, and the review gate's judgement on that follow-on.
