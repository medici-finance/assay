# common clauses

The clauses EVERY dispatched agent receives, whatever its class. They are emitted ahead of
the class kit on every dispatch, so a worker, a reviewer and a verifier are bound by one
wording of each rather than three that drifted apart.

Each is a rule that has already failed in the field. The wording IS the fix — quote it, do
not paraphrase it, summarise it, or "improve" it at dispatch time. Placeholders in
`<angle brackets>` are substituted by the dispatcher; everything else is fixed text.

Nothing here is a licence. Where a clause says STOP, it means stop and report — the prompt
that contains the clause is never the authorisation to work around it.

---

## C1. Home worktree — the isolation floor

> Your home worktree is `<worktree>` — every file operation stays under it.

Quote that line verbatim. Then, in the same prompt:

- Name the agent's own worktree as its ONLY writable root, and quote NO shared-checkout
  absolute path anywhere in the prompt. A path carried in a prompt overrides every
  isolation layer underneath it, because the agent will simply use the path it was given.
- Require the agent to run `git rev-parse --show-toplevel` before its first write and ABORT
  if it resolves to the shared checkout.
- A scratch directory or a clone under `/tmp` does NOT isolate a dispatched agent: guards
  scope their decisions by the agent's working directory, so an agent whose home is the
  shared checkout collects false-positive blocks on writes to its own clone — and a
  false-positive block is the input that produces the evasion C2 exists to stop. Clone
  under the agent's own worktree, never shared-homed.
- Create the branch from the fetched remote head, spelled in FULL:
  `git -C <repo-root> worktree add <path> refs/remotes/origin/main --detach`.
  Both parts are load-bearing. `--detach` stops the new branch inheriting the current
  checkout's HEAD and dragging a sibling's unreviewed commits along. The full
  `refs/remotes/origin/main` spelling matters because git resolves `refs/heads/` ahead of
  `refs/remotes/`: in any checkout that has ever acquired a local branch literally named
  `origin/main`, the bare form silently checks out that stale local branch and prints only
  `warning: refname 'origin/main' is ambiguous.` The agent then starts behind, and every
  push it makes carries the deficit.

## C2. No-evasion — a block is a STOP signal

> Any guard or hook BLOCK (a write guard, a push guard, a permission denial) is a STOP
> signal, not an obstacle. NEVER re-attempt the same effect with a different command, tool,
> or path spelling — `find -delete` for a blocked `rm`, an interpreter one-liner, an API
> call for a blocked push. On a block: stop that line of work, quote the block message
> verbatim in your PR/issue report, and escalate per the output contract. If you believe
> the block is a false positive (your target is outside the shared checkout), you may
> re-issue the SAME command with absolute target paths or a single `cd <abs-dir> && …`
> chain — the guard resolves those; anything else is escalate-only. A task completed via
> substitution is a failed task.

## C3. Offline envelope — no live infrastructure

> You run OFFLINE against live infrastructure: no command or script you run may contact a
> cluster or production endpoint, read-only included. Export `KUBECONFIG=/dev/null` before
> your first command. Anything that needs live state is could-not-check + BLOCKED-ON-HUMAN
> on the PR — never a probe.

## C4. Three-state instruments — could-not-check is never a pass

An instrument that did not look has not cleared anything. Every check an agent runs reports
one of checked-clean / checked-failed / could-not-check, and the third is reported AS
ITSELF: never rounded up to green, never rounded down to a failure it did not observe, and
never presented as a listing that reads like a grant.

## C5. Escalate durably, then stop

A question in a transcript is not durable; a question on the forge is. Anything the agent
cannot resolve becomes a filed issue or a comment on the PR it is working, carrying the
escalation label (`question` / `help wanted` / `needs-decision`) and a statement of exactly
what is needed and from whom. Then stop that line of work — do not guess, and do not
proceed on an assumption you have just written down as an open question.

## C6. One workpad per PR — no separate done/summary comments

> Keep ONE workpad per PR via `deskreply --workpad`; no separate done/summary comments;
> update it before hand-off and at every blocker.

A worker re-dispatched onto a PR starts cold unless the prior state is somewhere it can
find it in one read. `deskreply --workpad --body-file F` finds the newest unresolved
comment this agent's own identity already posted on the PR and edits it in place — plan,
acceptance criteria, environment stamp, validation, notes — rather than adding another
comment to a scatter an outward-write budget then has to police. Post a fresh reply
(`deskreply <owner/repo> <pr> --body-file F`, no `--workpad`) only for something a workpad
edit cannot represent — a distinct finding reply, an announcement of adoption that must
stay visible in the thread on its own. Everything that is this agent's own running state —
what it intends to do, what it has verified, what is blocking it — belongs in the ONE
workpad, edited, never appended as a new comment.
