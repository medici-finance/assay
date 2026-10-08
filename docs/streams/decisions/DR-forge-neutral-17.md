---
id: DR-forge-neutral-17
date: "2026-10-08"
title: "Widen the run-log read to the worker and reviewer roles, and keep run retry behind the same roster-bound run credential that refuses a human-bound entry"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Grant run retry to the worker and reviewer roles along with the log read, as one broad read-and-retry grant — ruled out: retry needs `actions: write` on GitHub (`api` on GitLab), the same scope that also cancels runs, deletes run logs and disables workflows repo-wide. Bundling it with the read would let one over-broad grant quietly cover both, which is the exposure `forge-neutral/14` made dispatch roster-bound to avoid."
  - "Put the log read behind the roster-bound run credential too, as a single-credential verb — ruled out: a log read needs only `actions: read` (GitHub) or `read_api` (GitLab), which cannot cancel, delete or disable anything. Requiring the release-runner credential for it would leave a repo with a human-bound or unset binding unable to read its own CI failure at all, so the problem that motivated the brief (a worker blind to why its check is red) would stay unsolved."
  - "On a human-bound or unset run-credential binding, fall through to an ambient CLI credential for retry — ruled out for the same reason as in `forge-neutral/14`: a human-bound binding is a deliberate state meaning nobody automated that repo's run control on purpose. `deskrun retry` refuses it (exit 5) before any mint or request."
accepted:
  - "A repo whose run-credential binding is unset or human-bound cannot be retried by any desk verb; that stays a human action until an operator deliberately binds a `release-runner` credential. This is the intended fail-closed posture."
  - "The log read is available to the worker and reviewer roles on every repo they already hold a token for, with no per-repo roster binding. The set is closed in code (a named two-role set), so widening it to another role is a deliberate edit, not a side effect."
  - "GitHub's retry is the narrower of its two endpoints: it re-runs only the failed jobs of a run. GitLab has no run-level retry, so the backend retries each failed job of the pipeline and leaves passed, running and manual jobs alone. A run with no failed job is could-not-check and writes nothing."
  - "Log text is bounded: each job's tail is capped, the part count is bounded, and terminal control sequences are stripped before anything is printed."
---

The driver (`human:<name>`) ratified `forge-neutral/17`, which adds the run-log and
run-retry operations (`RunLog`, `RetryRun` on the forge seam and the `deskrun log` and
`deskrun retry` verbs that consume them). The ratification was given at the brief's own
human decision gate.

The ruling is recorded on the brief's decision-gate issue,
[issue #1557](https://github.com/medici-finance/assay/issues/1557). The answer, "approve as
briefed", was posted under the driver's own login at 2026-09-23T18:52:16Z in
[this comment](https://github.com/medici-finance/assay/issues/1557#issuecomment-5800933286).
That confirms the two points the brief's `gate-why` puts to the human: that the read-only
`actions: read` / `read_api` grant is safe to widen to both the worker and the reviewer
roles, and that `retry` carries the same over-broad-scope shape as dispatch and so earns
the same roster-bound refusal rather than a lighter gate because it only retries.

This closes the human gate the brief's frontmatter declares (`gate: human`, `risk:
{sensitive-data: yes}`). The constraint behind the decision is the asymmetry of the two
permissions: reading a log can destroy nothing, while retrying needs the scope that can also
cancel runs and delete their logs. The design keeps the two apart. The read runs under the
calling role's own token. The retry runs only under the repo's roster-bound run credential,
and a human-bound entry is refused before any mint is attempted. As a second, independent
layer, the backends refuse an unminted token outright.

**What this record does not decide.** It does not widen the log read to any role beyond the
worker and reviewer, and it does not widen retry beyond the roster-bound credential. It does
not attest the chosen design is correct: that the alternatives were weighed is recorded here,
and whether the narrower grant was actually chosen in the code is the review gate's judgement,
then the change's own validation after it lands.
