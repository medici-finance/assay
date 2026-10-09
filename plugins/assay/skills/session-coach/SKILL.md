---
name: session-coach
description: >-
  Review a finished (or the current) agent session's own history and write a three-part coaching
  note for the next one. Use at the end of a session, or on demand mid-session, when asked to
  "coach this session", "review what this session did", "what should the next window do
  differently", or "look at my session history". Not a standing record of loop state (that is the
  standing note) and not a code or PR review.
---

# Session coach — ask the model to review your own session, then act on it

A session ends and its window closes, and nobody asks what the next one should do differently
because of what this one did. The cheapest fix is the one practitioners already report: point a
model at the session's own history and ask it to coach you. At the Transcend 3 roundtable,
Winslow (Thrive Market) described exactly this — ask the LLM to look at your session history,
what you worked on, which models you used, how you could have approached it differently, and turn
that into a personal coach. It costs one read of material the session already produced, and the
output is advice that is specific to what actually happened rather than to what usually happens.

This skill is that practice, written down so it survives the person who first did it.

## When it runs

- At the end of a session, before the window closes — the natural moment, because the history is
  complete and the operator is still there to accept or reject what comes back.
- On demand, mid-session, when the operator asks for a read on how it is going.

How often a desk or an operator runs it, and whether it is wired into any end-of-session routine,
is each adopter's own choice. The skill states the method and nothing about the cadence.

## What it reads

The session's own record: the transcript, the tool calls and their results, any subagent
transcripts, dispatch and claim records the session produced, and whatever rollups the project
keeps of operator messages. It reads only what the session itself produced or was handed — it does
not go looking through other sessions. Where a piece of that record is missing or unreadable, the
note says so for that piece; a part of the note built on material that could not be read is marked
as such, not written as though it had been checked.

## Where the note goes — outside any repository, never committed

A session transcript carries prompts, paths, account names and tool output. The note is derived
from it and inherits all of that. So:

- The note is written **outside any repository**, in the operator's own scratch or notes location.
- The note is **never committed** — not to the project, not to a docs tree, not into a PR body, an
  issue, a comment or an example. Quoting a line of it into any of those is committing it.
- A note shown to a reader other than the operator is shown by the operator, on their own judgement.

## It never writes a memory file

The skill **never writes a memory file** — not the operator's, not the project's, not an agent's
auto-memory. A coaching note often ends in "next time, remember X", and the pull to save that
straight into memory is exactly the failure this rule exists to stop: an unreviewed model
judgement about one session becomes a standing instruction for every later one.

If a memory edit is warranted, the skill **proposes** it: the exact text, the file it would go in,
and the evidence in the session for it. The operator accepts or rejects it, and if accepted the
operator (or a separate, explicit step they invoke) makes the edit. The proposal is part of the
note; the edit is not.

## The note — three questions, three headings

The note has exactly these three sections, under these headings, in this order. They are the
three questions from the roundtable, kept verbatim so the practice stays recognisable.

### What did I work on

What the session actually spent its effort on, set against what it claimed. Read the history, not
the closing summary: count where the tool calls, retries, reads and waiting actually went, name
the two or three threads that took most of it, and say plainly where that differs from the
summary the session gave of itself. A session that reported "implemented the change" and spent
most of its calls on a flaky environment did not work on the change; say so. Include the models
and routes used, as facts, because the next section reasons from them.

### What did it cost

Cost here is calls, time and model spend, whichever the record lets you measure — and the note
says which it measured. Name **one** routing or model choice that would have been cheaper with no
loss of quality: a specific step, the model or route it ran on, the model or route that would have
done it as well, and the evidence in the session that the cheaper one was enough (the step was
mechanical, the output was checked by something else, an earlier run of the same step on the
cheaper route had passed). One swap for one step, not a general "use smaller models more". If no
such choice exists in this session, say that and name the step that came closest; do not invent
one to fill the section.

### What would I do differently

Name **one** process correction: a manual step the session repeated that should have been a tool,
a skill, or a verb. State the step, how many times it was repeated, what it cost each time, and
what shape the replacement should take (a one-line verb, a skill, a checklist item). One, not a
list — a note with ten corrections gets none of them done. If the repeated step already has a
tool the session did not use, the correction is "use it", and the note names it.

## Boundaries

- **Not the standing note.** A standing note is a record of loop state that a successor reads to
  resume the work. This is a review of how the session went. The two can reference each other;
  neither replaces the other.
- **Not a code or PR review.** It judges how the session spent its effort, not whether the
  change it produced is correct.
- **Advice, not enforcement.** The note proposes; it changes no configuration, no routing table
  and no memory by itself.

A short synthetic specimen of the note, with invented content, is in [`example.md`](example.md).
