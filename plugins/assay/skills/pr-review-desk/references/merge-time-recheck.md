# Merge-time re-check, and the body/Verify re-check

Two clauses of the reviewer's bar, kept here because the reviewer prompt needs them at length and
the skill needs them at a glance. `pr-review-desk/SKILL.md` § The reviewer's bar carries the
short form and points here; `deskdispatch --kit review` §5/§6 carries the generic wording the
dispatched agent receives. If you edit one, check all three still say the same thing.

## Merge-time re-check — review against MERGED main, not the tree you were handed

Review asks "is this correct against main?" and answers it against the main that existed at review
time. The merge lands it in a different main. Five near-misses in one evening lived in that gap;
four were caught only because someone diffed against **merged main** instead of trusting the PR
— and every one of the five merged CLEANLY. Nothing else in the loop re-asks the question at merge
time, so the reviewer carries it.

- **Diff 3-dot against merged main, never against the prior head.** `git diff
  refs/remotes/origin/main...HEAD` (three dots — the merge-base form) for the branch's own work,
  re-read against **current** `refs/remotes/origin/main`, not the SHA the review started at. Spell
  the ref in full: a bare `origin/main` resolves through `refs/heads/` first when a stale local
  branch of that name exists, which puts the whole review on a base dozens of commits behind.
- **A CONFLICTING resolution that touches the PR's own files is a NEW CHANGE ⇒ mandatory
  RE-REVIEW**, and it may never be waived. A keep-current merge that resolved a conflict is
  authored work, authored by whoever resynced — usually not the person the review approved.
- **Run the merge-time re-check before an approval or a flip:** the repo's merge-check verb, given
  a build command on a compiled-language PR — the only way it can see a signature-level collision. Read its four states as four different
  instructions: `MERGE-INTRODUCED` is a blocker the branch alone cannot show you; `PRE-EXISTING` is
  a blocker the merge did not cause; **`STALE-BASE` means the branch is stale, NOT broken** — a CI
  job exiting 127 on a script the base has and the branch's tree lacks is a currency fact, and
  sending the worker to debug it is sending them after a phantom. `could-not-check` is never a
  pass: an instrument that did not look has cleared nothing, and an approval resting on it is
  unfounded.
- **A clean merge is the WEAKEST evidence in the report.** "No conflict" means git could combine the
  bytes, not that the combination is correct. Semantic collisions — two changes valid alone and
  invalid together — are textually invisible by construction: a function that grew a parameter on
  main while an open PR still called it at the old arity; two briefs allocating the same rule
  number in parallel.
- **Name the safe merge order.** When a PR shares a file with another open or recently-merged PR,
  the review says which merges first and what the exact resolution is. "They'll conflict" is not a
  finding a worker can act on.
- **Verify any artifact against its SOURCE, never against a previous render.** A render agrees with
  itself.

## Body and Verify table are re-checked against the CURRENT diff

Every delta re-review reads the PR body and the brief's Verify table against the diff **as it now
stands**, and treats any claim the diff contradicts as a **blocker, not a nit**.

- A materially-changed diff — a version bump, a changed part/artifact count, a reverted or replaced
  design decision, a dropped deliverable — must re-derive the body and the Verify table **in the
  same push**. A body describing an earlier version of the diff is a stale copy of the diff, and
  the reviewer is the check on it.
- **On `gate: human` briefs this is not optional**, because there the human signs the BODY. A stale
  body means the signature attests to fiction — the recorded instance is a PR body asserting a
  funds-protection property the code had already reverted.
- **Approval staleness: know what you can and cannot tell.** Any resync push invalidates approvals
  outright, so a lost approval is often the price of becoming mergeable and not a finding at all —
  say which it is. And do not build a verdict on a review's `commit_id`: it has been observed to
  disagree with the head named in the review's own body, and **the direction and frequency of that
  disagreement are unmeasured** (one observation, a single observation; a later 10-PR sweep established no direction). It is not a sound staleness signal in either direction — not because
  it is known to under-report, but because its error is uncharacterised. When the question is "was
  this approved at the tree that will merge", the honest answer from that signal is
  could-not-check, and you may not upgrade that to "the approval is fine".
