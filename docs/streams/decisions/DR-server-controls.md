---
id: DR-server-controls
date: "2026-09-16"
title: "Server-side enforcement is three forge primitives — a uniform ruleset menu, one readable ruleset API, and required status checks reported by a non-author-controllable runner — never fine-grained per-invariant privileges"
consequence: critical
decided-by: "human:<name>"
alternatives:
  - "Provision fine-grained per-invariant privileges on the forge so the platform enforces every custody rule directly — ruled out because it is not a capability any forge offers. GitHub and GitLab expose a fixed App/PAT permission menu, a fixed ruleset menu on the protected ref, and required status checks; there is no arbitrary per-rule privilege between the ruleset menu and a required check. Pursuing it wastes the ask on an impossible target and leaves the real gaps (uneven menus, unreadable classic protection, unenforced fine rules) open."
  - "Close the finer invariants with a second CONVENTION — another rule each desk App honours voluntarily, like deskflip's merge restraint — ruled out because a second convention shares the first's failure mode exactly: something a well-behaved actor honours and a compromised or misconfigured one does not. It adds no independent layer. The finer rule must be enforced by the server (a required check the ruleset itself demands), not by more restraint."
  - "Add the fine rule as a required check that runs in the PR branch's own `pull_request` workflow — ruled out: that workflow's code lives on the head the author controls, so the author can edit the check to pass. It degrades immediately to self-attestation — a green lamp the policed party wired. The check must run from the BASE repository, report under an identity the author does not hold, and have its source on a protected path, or it enforces nothing."
  - "Grant every role App `administration: read` so deskflip can read classic branch protection, and leave classic protection in place — a legitimate option for the #1020 read gap, but ruled out as the DEFAULT because `administration: read` is a broad grant (it exposes far more than the required-checks list) and it leaves two protection APIs to read instead of one. Standardizing on rulesets and retiring classic protection closes the read gap with one readable API and no widened grant. The grant is kept as the fallback where a repo cannot yet be migrated."
  - "Treat #997 as fully stale because `require_last_push_approval` is already set — ruled out as an overstatement. That setting closes author≠approver, which is real and worth stating, but it does not close the finer residual: an independent/second approver, and cross-operator collusion where two distinct role Apps (one pushing, a different one approving) satisfy the forge, which cannot see they may share one operator. Declaring #997 solved would leave that residual untracked."
accepted:
  - "Uniformity is an operational commitment, not a one-time act: the target ruleset menu must be re-audited as repos are added, because a new repo defaults to no ruleset and silently reintroduces the divergence the audit exists to catch. The audit brief reads; a human applies; and nothing but a repeat read proves it stayed applied."
  - "A required check is a real widening of trust in the runner: the merge gate now depends on the check's identity, execution context, and source integrity. Those three properties are the security surface and each is a Verify obligation (a negative-path row proving the check reddens when its precondition is violated), not a claim. Where any of the three cannot be assured, the check must NOT be relied upon as a server-side control and must be described as advisory."
  - "Author≠approver is enforced today on this repo (require_last_push_approval + require_extra_approval_for_unattributed_changes); the honest residual is the finer independent-approver and cross-operator-collusion case, which no forge setting can see and only a required check can close. The stream states both halves and rounds neither."
  - "Retiring classic branch protection is an admin act with a blast radius (it changes what the platform enforces on the default branch); it is performed by a human, verified by a read-back, and never done to make a client tool's could-not-check go green without confirming the equivalent ruleset is in place first."
  - "The credential/identity design decisions this depends on (#900 runtime credential contract, #903 in-process transport/auth, #942 the cell-issues mint role) are forge-granularity-independent and remain open human gates; the enforcement design here does not pre-empt them and the credential-contract briefs wait on their rulings."
---

**PROPOSED — no ruling is recorded.** `decided-by:` is a placeholder until a human rules on
the design. This record is lint-clean at `proposed` (the design-approval gate binds only at
`in-progress`+; the `human:<name>` placeholder passes the has-human-reviewer shape check), so
it can be authored and reviewed before any brief it gates moves.

## The reframe this record fixes

The request to *"provision fine-grained privileges"* on the forge is impossible: no forge
offers arbitrary per-invariant privileges. What it offers is three primitives, and every
server-side control the fleet needs is built from them:

1. **App / PAT scopes** — the coarse *permission ceiling*. Never merge-specific:
   `pull_requests: write` is sufficient to merge, and no narrower scope grants
   "open/comment/review a PR" without also granting "merge it."
2. **Ruleset settings** — the fixed *menu* on the protected ref (required-approval-count,
   require-last-push-approval, dismiss-stale-on-push, restrict-who-can-push /
   -dismiss, required signed commits, block-force-push, block-deletion, required status
   checks; GitLab adds *prevent approval by author* and *prevent approval by committers*).
   The whole lever here is setting the menu the SAME on every repo.
3. **Required status checks** — the one place the ruleset defers to operator code, and the
   ONLY mechanism for a rule finer than the menu.

## The required-check pattern, and its self-attestation caveat

Primitive #3 is the general tool for a fine invariant: the server enforces "check X is green,"
and check X is our code enforcing the fine rule. This repo already runs one — the `leak-sweep`
required status check named by the `leak-sweep` ruleset. The pattern is reusable, but only
under three conditions, because a required check written without them is worse than none — it
looks enforced while enforcing nothing:

- **Non-author identity.** The check reports under an identity the policed party does not
  hold. If the author can post the check's status, they can post `success`.
- **Base-repo execution context.** The check runs from the base repository, not the PR
  branch's own `pull_request` workflow. A `pull_request`-triggered workflow executes code from
  the head the author controls; the author edits the check to pass.
- **Protected source.** The check's own source (workflow file, action, ruleset entry) lives on
  a protected path, so changing the check is itself gated.

Violate any one and the check degrades to **self-attestation**. This is the same measured-status
mechanism the fleet already uses for verification witnesses, pointed at the merge gate: trust in
the green cell is exactly trust in who reported it and where it ran.

## The anti-collusion reality (#997), stated honestly

Live-read 2026-09-16, `protect-main` (id 20301257) sets `require_last_push_approval: true` and
`require_extra_approval_for_unattributed_changes: true`. So **author ≠ approver is already
enforced server-side** on this repo — the account that pushed the head cannot be the approval
that satisfies the count. #997's original premise is therefore partly stale.

The genuine residual is finer and no ruleset setting can reach it:

- an **independent / second approver** (more than one distinct approver, or from a named set);
- **cross-operator collusion** — App A pushes, App B approves; both are legitimate GitHub Apps
  with `pull_requests: write`, the forge is satisfied, and it cannot see that one operator may
  drive both, because "same operator" is not a property the platform models.

That residual is exactly a primitive-#3 job: a required check, reported by a runner neither App
controls, that evaluates the finer approver rule at the head SHA and reddens the gate otherwise.
The reference design for it is `server-controls`'s cross-operator-check brief; it is `gate:
human` and cites this record.

## Options considered per #997 and #1020

- **#997 (anti-collusion residual):** (a) accept author≠approver as sufficient — rejected, it
  leaves the cross-operator case open; (b) a second convention — rejected (shares the failure
  mode); (c) a required check under the pattern above — **chosen** as the design direction, to
  be ratified per-brief with negative-path Verify.
- **#1020 (readability):** (a) grant `administration: read` and keep classic protection —
  fallback only (broad grant, two APIs); (b) standardize on rulesets and retire classic
  protection — **chosen default** (one readable API, no widened grant); a repo that cannot yet
  migrate takes (a) as a documented interim.

## What this record does not decide

It does not apply any ruleset change, does not retire classic protection anywhere, does not
grant any permission, does not author the `human-approved` / cross-operator workflow files, and
does not settle #900 / #903 / #942. Those are admin or human-gated acts named by the briefs and
performed outside this record.
