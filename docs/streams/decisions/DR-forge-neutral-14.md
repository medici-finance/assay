---
id: DR-forge-neutral-14
date: "2026-09-23"
title: "Add run and gate-approval verbs (deskrun) behind a roster run-credential binding that refuses a human-bound entry, with the narrowest trigger credential per forge as the default"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "GitHub `repository_dispatch` as the default trigger — ruled out: it fires with `contents: write` alone, a scope most desk Apps already hold for their own writes, so any of them could start a release-shaped event. That is a far wider \"who can start a release\" surface than a roster-bound, single-purpose credential. It is documented as an alternative for a future, narrower use and is not implemented or wired as a fallback."
  - "GitLab `POST /projects/:id/pipeline` under a project or user token as the default — ruled out: the same credential can also read and write issues, merge requests and repository content. The pipeline trigger token (`POST /projects/:id/trigger/pipeline`) can start pipelines and nothing else, so it is the default; the project-token path stays documented as the fallback for a deployment that needs project-scoped attribution."
  - "Treat a `human:<name>` run-credential binding as a gap to route around (fall through to an ambient `gh`/`glab` credential, or to whichever App happens to be bound) — ruled out: a human-bound binding is a deliberate state meaning nobody automated this repo's release trigger on purpose. `deskrun` refuses on it (exit 5) before any mint, and never reads an ambient credential."
  - "Repurpose an existing worker/reviewer/verifier App for the run credential — ruled out: none of them is scoped for `actions: write`-adjacent operations, and none should become so by accident. The `release-runner` binding is a dedicated, single-purpose credential an operator creates and roster-binds on purpose."
accepted:
  - "A repo whose run-credential binding is unset or human-bound cannot be dispatched or gate-approved by any desk verb; that stays a human action until an operator deliberately binds a `release-runner` credential. This is the intended fail-closed posture, not a regression."
  - "GitHub's `release-runner` credential needs `actions: write`, the one scope that also grants cancelling runs, deleting run logs and disabling workflows repo-wide. The binding narrows WHO holds it (one roster-bound identity per repo); it does not narrow what the scope itself grants."
  - "The GitHub workflow-dispatch endpoint returns no run id, so `RunWorkflow` resolves the created run by a follow-up list read. When two dispatches race inside the correlation window the verb refuses as could-not-check rather than guess the newest run."
  - "The GitLab mapping is pinned against recorded fixtures only. A live GitLab dry run is could-not-check by design in this repo: it carries no CI-reachable GitLab project."
---

The driver (`human:<name>`) ratified `forge-neutral/14`, which adds the run and
gate-approval verbs (`RunWorkflow`, `ApproveGate`, `RunStatus` on the forge seam and the
`deskrun` verb that consumes them). The ratification was given at the brief's own human
decision gate.

The ruling is recorded on the brief's decision-gate issue,
[issue #1556](https://github.com/medici-finance/assay/issues/1556). The answer, "approve as
briefed", was posted under the driver's own login at 2026-09-23T18:47:26Z in
[this comment](https://github.com/medici-finance/assay/issues/1556#issuecomment-5800844370).
The decision-gate template offered three options: approve as briefed, approve with
changes, or hold/reject. The recorded answer is option 1, approve as briefed, which
confirms the three points the brief's `gate-why` puts to the human:

- `repository_dispatch` is correctly left un-adopted as the default trigger. It needs only
  `contents: write`, a scope desk Apps already hold, which is exactly why it is the wider
  surface.
- The GitLab default is the narrow, start-only pipeline trigger token, not a project-level
  token that can also read and write.
- A run-credential binding naming `human:<name>` is a legitimate state that `deskrun`
  refuses on. It is not a bug to route around.

This closes the human gate the brief's frontmatter declares (`gate: human`, `risk:
{sensitive-data: yes}`). The constraint behind the decision is that GitHub ships one scope,
`actions: write`, for dispatching a workflow and approving a deployment gate. The same
scope also grants cancelling any run, deleting run logs and disabling workflows repo-wide,
so no desk App is safely grantable it, and until now both operations fell to whoever's
ambient CLI credential was active. The design keeps the credential out of every existing
desk identity and binds it per repo in the roster. A human-bound entry is refused before
any mint is attempted. As a second, independent layer, the backends refuse an unminted
token outright.

**What this record does not decide.** It does not adopt `repository_dispatch` or the GitLab
project-token pipeline endpoint for any use; both stay documented alternatives. It does
not choose a GitLab gating shape for any particular project: `ApproveGate` dispatches on
the shape the roster declares for the target project and refuses when the read disagrees.
It does not, by itself, prove the approver is a different identity from the brief's
author. That is the same attribution-not-identity limit `lifecycle-v1.md` §7.1.2 declares
for verification, and `registers-v1.md` §7.4 for this register. And it does not attest the
chosen design is correct. That the alternatives were weighed is recorded here; whether the
narrower credential was actually chosen in the code is the review gate's judgement, then
the change's own validation after it lands.

**Amended 2026-09-23 (correction of a factual premise; the ruling above is unchanged and
the choice below is the ratifying human's).** Two statements in this record are corrected
here rather than rewritten above:

- The constraint paragraph says GitHub ships one scope, `actions: write`, for dispatching a
  workflow AND approving a deployment gate. That is wrong for the approval. Dispatching
  needs `Actions: write`; approving a pending deployment
  (`POST /repos/{o}/{r}/actions/runs/{run_id}/pending_deployments`) needs
  `Deployments: write`, and GitHub lets only an environment's required reviewers approve —
  required reviewers are users or teams, never an App. So under the `release-runner` App
  credential the GitHub approve path cannot succeed: the forge reports the App may not
  approve, and `deskrun approve` refuses as could-not-check before writing anything. The
  ruling was given on the brief's premise; the correction is posted on
  [issue #1556](https://github.com/medici-finance/assay/issues/1556), where the ratifying
  human chooses whether the GitHub approve path ships as a documented could-not-check or is
  withdrawn. This record does not make that choice.
- The third `accepted` entry says a race inside the correlation window refuses. More
  exactly: it refuses when both runs are visible in the same list read. If another matching
  run is already listed and this dispatch's run is not yet, the first read can return the
  other run; the actor filter narrows that window to the `release-runner` identity when its
  App login is bound.
