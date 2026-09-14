---
id: DR-fork-ci-posture
date: "2026-09-12"
title: "Fork continuous-integration posture is keyed on trust tier, with never-build-unblessed enforced by a check rather than by a rule a reader must remember"
consequence: critical
decided-by: "human:<name>"
alternatives:
  - "Leave never-build-unblessed as written guidance only — ruled out: it is the single rule standing between an untrusted head and a runner holding credentials, and a rule whose only enforcement is that somebody remembers it has no failure signal at all. When it is broken, nothing anywhere reports it; the first indication is the consequence."
  - "Auto-approve fork workflow runs for every tier above unknown — ruled out as stated, because it grants on the wrong axis. Auto-approval is safe only where a run cannot reach a secret or a credentialed runner, and that is a property of the WORKFLOW, not of the author. The posture is therefore a matrix of tier against workflow class, and a workflow that can reach a secret is never auto-approved for any external tier."
  - "Move every fork-triggered job to `pull_request_target` so it runs with repository context — ruled out, and it is the classic inversion: that event grants the job a write-capable token in the base repository's context, so combining it with a checkout of the fork's head is the exact exposure being avoided. The one existing use of the event here is safe precisely because it never checks out the pull request's code, and that property must be asserted, not assumed."
  - "Block all fork continuous integration until a maintainer opts the pull request in individually — ruled out: it is close to today's posture, it makes the maintainer the bottleneck for every genuine contributor, and it buys nothing over a tiered posture whose lowest tier is exactly this behaviour."
accepted:
  - "Auto-approval is a real widening: after it, a run triggered by an external identity executes without a fresh human act. It is bounded to workflow classes proven unable to reach a secret or a credentialed runner, and that proof is a Verify obligation, not a claim."
  - "A fork pull request that touches continuous-integration configuration, dependency lockfiles, install scripts or container definitions is labelled automatically and drops to the strictest posture regardless of the author's tier. Those paths change what the runner executes, so authorship trust is the wrong input for them."
  - "The never-build-unblessed check is a control over the desk's own behaviour, so it can be evaded by a human acting outside the tools. It raises the cost of the mistake and gives it a signal; it is not a sandbox and must not be described as one."
  - "The audit of existing workflows may find an exposure. Finding one is the point; fixing it belongs to the same change, and weakening a control to make a check pass is never the resolution."
---

**PROPOSED — no ruling is recorded.** `decided-by:` is a placeholder until a human rules on
the brief's decision issue.

Two independent layers, failing for different reasons in different components, because this
is the surface where a single control is least acceptable:

1. **At the forge.** Workflow approval policy and the event each workflow listens on decide
   whether an external head can execute at all, and with what token. This layer holds even
   when every tool in this repository is bypassed, because it is the platform enforcing it.
2. **At the desk.** A check refuses the local build/test path for a head whose item is not
   blessed. This layer holds when a maintainer with approval rights approves a run they
   should not have, because it trips on a different signal (the item's blessing state) in a
   different place (the operator's machine) at a different time (before the command runs).

The single control that would otherwise stand alone is the maintainer's click on "approve and
run". Behind it: the workflow's own inability to reach a secret, and the desk-side refusal.

**What this record does not decide.** It does not flip the published contribution guidelines'
enforcement switches, it does not settle which runner pool fork jobs use, and it does not
claim the desk-side check is a sandbox.
