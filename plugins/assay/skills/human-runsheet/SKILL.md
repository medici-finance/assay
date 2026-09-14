---
name: human-runsheet
description: >-
  Write the acts owed to the driver — not decisions, ACTS only the driver can perform — as exact
  runnable commands in a durable file. Use when a session hits a change a client-side guard
  refuses, a staged-to-live promote whose scope its token does not hold, a workflow dispatch its
  App must not be granted, or a gate override reserved to a human, and would otherwise describe
  the blocker as prose in a console nobody is re-reading. Not for presenting a DECISION (that is
  `ask-decision`) and not for a multi-session operation's record (that is `author-drive-plan`).
---

# Human runsheet — the acts owed to the driver, written as exact commands

A session that hits a structural block — a guard it cannot clear, a scope its own token does not
hold, a permission its App must never be granted, a gate only a human may override — has exactly
one honest thing to do with that fact: hand it to the driver as something they can *run*, not as
a paragraph they have to reconstruct into a command. "I am blocked, here is what I think you need
to do" is lossy the moment the session that wrote it ends. A file of exact `! <command>` lines is
not.

## When it runs

A session invokes this the moment it recognises the blocker is not "I don't know how" but
"I structurally cannot" — the four classes below cover the shapes that recur:

- a change a client-side classifier or guard refuses to let the session write or commit;
- a staged-to-live promote whose scope the session's own token does not carry;
- a workflow dispatch that the session's App is deliberately not granted the permission to run;
- a gate override reserved to a human (a required check, a merge, a tag) that no App holds
  authority to waive.

It does not run for a DECISION — a fork where the driver picks among options and the session can
proceed once told which. That is `ask-decision`'s job: it presents options with a recommended
default and relays the ruling. This skill is for the opposite shape: the driver has already,
implicitly, made the call by being the only party who *can* act — there is nothing to decide,
only something to run.

## The entry — four parts, every time

`RUNSHEET.md`, kept in the session's own scratch home — **never the repo tree**. It is session
state describing what this session could not do, not a repo artifact anyone else's tooling reads.
One entry per owed act, in this shape:

1. **The command.** A line starting `! `, followed by the exact command, runnable as written,
   every placeholder already resolved to a real value. A driver who copies the line and presses
   enter should not have to fill anything in first.
2. **Why it is owed to the driver.** The specific structural reason — quote the guard's refusal,
   name the scope the token lacks, name the permission the App does not hold, name the gate.
   "I couldn't do it" is not a reason; the refusal text is.
3. **What the session did instead.** Parked the change at a stated path, filed the escalation
   issue, worked around it within its own authority where one existed — whatever actually
   happened, stated plainly.
4. **The resume step.** What the session will do once the act lands, and how it will notice —
   which file changed, which run turned green, which label came off. A runsheet entry with no
   resume step is a dead end dressed as a hand-off.

## Never a credential in a runsheet

A line names the **identity** the command must run under — "as the driver's own account", "with
admin rights on the repo" — and never carries a token, a key, or a password, and never instructs
the driver to hand one over to the session. A runsheet is a set of commands, not a place secrets
pass through. If a command as actually run would need a secret filled in, the entry says so and
leaves it to the driver to supply it directly when they run it — the file itself never carries it.

## Three boundaries

- **Against `ask-decision`.** `ask-decision` presents a DECISION — the driver chooses among
  options and the session proceeds on the answer. This presents an ACT — there is no option set,
  only one command only the driver can run. A blocker that is genuinely a choice belongs there,
  not here.
- **Against `author-drive-plan`.** `author-drive-plan` is the durable record of a multi-session
  OPERATION — where a migration or a long sweep left off, across many sessions and many briefs.
  A runsheet is narrower and more perishable: the acts owed right now, by the session that wrote
  it, this run. A drive plan may reference that a runsheet exists; it does not replace one, and a
  runsheet is not a substitute for a drive plan when the work truly spans sessions.
- **Not a second escalation channel.** The filed issue (or PR comment) the session already opens
  for the blocker remains the escalation — the thing the driver's queue surfaces and the thing
  that ages if unanswered. The runsheet accompanies it as the exact command form; it is never
  written instead of filing, and it is never treated as having escalated anything on its own.

## Four example entries

Stripped of any house's own tool names — each names a class from `## When it runs` above, in the
four-part shape.

**Example 1 — a client-side guard refuses the write.** A content guard on the session's own write
path refuses a diff whose content matches a pattern it cannot clear on its own authority.

! git apply /path/to/parked.patch

Why: the guard binds this session's write path only, on the *text* of the change, and only a
human accepting the material outside that guard can land it. What the session did: left the
diff at the stated path and filed the blocking issue. Resume: on next boot, check whether the
patch's target file changed on the branch; if so, continue from the following step.

**Example 2 — a staged-to-live promote outside the token's scope.** The session's own token is
scoped to staging; promoting to the live environment needs a broader scope no App should hold
standing.

! deploy-promote --from staging --to production --release <release-id>

Why: `deploy-promote` is the driver's own promote tool, run under the driver's own account —
this skill names it only as an example command, never as a verb it defines or grants. What the
session did: staged the release and stopped short of promoting it. Resume: poll the release's
own status until it reads live, then continue the rollout checklist.

**Example 3 — a workflow dispatch the App must not hold.** Firing a given workflow needs a write
scope the session's App is deliberately kept without, because that scope can also cancel
unrelated runs or disable other workflows.

! gh workflow run <workflow-file> --ref <branch>

Why: the App's permission is deliberately read-only on workflow dispatch; broadening it to
dispatch one workflow also grants canceling and disabling every other one. What the session
did: prepared the branch the workflow should run against and stopped. Resume: watch for the
run to appear and pick up at the step gated on its result.

**Example 4 — a gate override reserved to a human.** A required check is red for a reason the
session has verified is unrelated to its own change, and only a human may override the merge
gate.

! gh pr merge <number> --admin --merge

Why: an admin override of a required check is a human-only gate by design — no App holds merge
authority over a protected branch. What the session did: posted the evidence that the red check
is unrelated and left the PR open. Resume: once merged, continue with the next dependent item.

Four illustrative entries, not a catalogue to keep current — a real runsheet holds only the acts
a given session actually owes, in whatever number that turns out to be.

## The closing rule

A runsheet accompanies the filed escalation; it never replaces it. If a driver reads only the
runsheet and the underlying issue was never filed, the escalation did not happen — the file is
the command form of something that must also exist as a durable, driver-visible record. Write
both, every time.
