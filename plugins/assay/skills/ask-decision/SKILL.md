---
name: ask-decision
description: >-
  Put the pending human decisions to the driver ONE AT A TIME, each with its context, its
  options with a recommended default first, the exact shape of the reply, and how the desk
  verifies the act afterwards. Use when the driver says "ask me 1-by-1", "walk me through the
  decisions", "what do you need from me", "what's blocked on me", "go through the
  needs-decision queue", or when a desk has more than one human gate open and would otherwise
  dump them all into one message. Also renders the same queue as a self-contained page for
  reading away from the terminal. Any desk can invoke it; it decides nothing itself.
---

# Ask Decision — one decision per turn, with context

A desk that has five things blocked on a human and asks about all five in one message gets no
answer to any of them. The driver has to reconstruct each item's context, hold five open
questions at once, and write a paragraph. A desk that asks about ONE, states the options, and
says exactly what a reply must contain gets a one-word answer and moves on. That is the whole
skill: **the queue drains at the speed the questions are askable, not at the speed the desk
can enumerate them.**

This skill is a PRESENTATION and RELAY contract. It decides nothing, it rules on nothing, and
its recommended default is a recommendation — the ruling is the driver's, and it is recorded
as a relay.

## When it runs

- The driver asks for it: "ask me 1-by-1", "walk me through the decisions", "what do you need
  from me", "what's waiting on me", "go through the needs-decision queue".
- A desk window ends with more than one human gate open. Rather than narrating them, invoke
  this and ask the first.
- After a ruling lands and the desk has recorded it, to present the next item.

It does NOT run for a single question — one gate is just the format applied once, which is
cheap, but it needs no skill invocation to do it.

## The queue and its order

The queue is the escalation-label queue, nothing new: `urgent`, `needs-decision`, `question`,
`help wanted`. The label vocabulary — what each one means, who is expected to answer it, and
the rule that a bare label is unanswerable without a comment saying what is needed and from
whom — is defined in the `intake-desk` and `the-desk` skills. **Point at it; do not restate
it here.** This skill assumes the labels already mean what those skills say they mean.

Read the queue with the inbox, which already sorts it:

```
bash "${CLAUDE_PLUGIN_ROOT}/scripts/assay-inbox.sh" --walk --item 1 owner/repo [owner/repo ...]
```

**Before asking, read where the system is stuck:**
`bash "${CLAUDE_PLUGIN_ROOT}/scripts/assay-inbox.sh" --flow` prints the pipeline stage by
stage with the bottleneck named, so an item's Context can say what it is actually holding up —
and so a question about a stage three steps downstream of the constraint can wait.

**Ordering rule: the item whose ruling unblocks the most in-flight work goes first; ties break
by age, oldest first.** The script's mechanical order is urgency-then-age, which is the
computable approximation of that rule — it can see labels and dates, it cannot see what is
waiting downstream. So:

- Take the script's order as the default and walk it with `--item 1`, `--item 2`, …
- **Override it when you know better, and say so in one clause** ("taking #58 first — three
  PRs are held on it"). An override is a judgement the desk is allowed to make and the driver
  can see; a silent reorder is not.
- Never reorder to put the easy questions first. The queue is drained to unblock work, not to
  maximise the count of answers.

## The format — five parts, every time

The script renders exactly this; when you compose an item by hand, compose the same shape.

1. **Header** — `<repo>#<N> — question k of n`. The position is load-bearing: it tells the
   driver how long this will take, which is what makes it possible to say yes to starting.
2. **Context** — 3–6 lines. What the item is; **why it is blocked on a human** (which gate:
   a `gate: human` brief, a security control, a ruling only the driver can give); what it
   unblocks; the evidence links (PRs, commits, the issue comment that raised it). Six lines is
   a ceiling, not a target. If the issue does not state its context, say
   *"context not stated — desk to fill"* and fill it before asking; do not ask a question the
   driver has to research.
3. **Options** — lettered, **the recommended default FIRST and labelled "recommended"**, each
   with **its consequence in one clause**. Never more than four. A "do nothing" option only
   when doing nothing is genuinely viable — a fake option to look balanced wastes the turn.
   An option with no stated consequence is not an option, it is a label.
4. **Reply shape** — exactly what the answer must contain: a letter, a name, "done", "merge
   it". **The driver should be able to answer in one word.** If your question cannot be
   answered in one word, it is two questions or an unfinished one.
5. **Verification** — what the desk checks after acting (the API read, the file, the run id,
   the label state) and what it moves to next. This is the promise that the answer will not
   evaporate into a transcript.

### One question per turn

Ask ONE. Wait for the answer. Do not queue the next question in the same message, do not
append "and while I have you", and do not pre-empt the answer by acting on your own
recommendation. The recommended default exists so the driver can say "A" — not so the desk can
proceed as if they had. **This binds a ONE-WAY gate** — a fork whose wrong guess lands irreversibly
or reaches outside the gate the driver still holds. A REVERSIBLE fork is different: the desk has
ALREADY proceeded on its default behind a draft PR (the reversibility test), so the ask here is not a
pre-empt but a merge-or-decline, and the item does not park on this skill.

## Recording a ruling — always a relay, never the driver's voice

When the answer comes back, the desk records it **on the issue**, because a decision that
lives only in a session transcript is not durable and cannot be cited later.

Write it as a **relay**:

> **Ruling relayed from the driver** (<date>): <the ruling, in one or two sentences>.
> Asked as: <the options that were put>. Answer: <the letter or word given>.
> Acting on it: <what the desk will now do, and where that work is tracked>.

Rules that make the relay honest:

- **Never write in the driver's voice.** Do not sign it as them, do not phrase it as their
  own comment, do not present a model's inference as their words. An agent relaying a real
  human decision says so and links where it was given — the `pr-review-desk` skill states the
  same boundary for review verdicts, and it is the same boundary here.
- **Relay only what was actually said.** If the driver answered "B" and you believe B implies
  three follow-on choices, the relay records "B"; the follow-ons are NEW questions for a
  later turn.
- **A silence is not a ruling.** No answer means the item stays in the queue.
- **Then move the label.** Adjust or remove the escalation label per the vocabulary the
  `intake-desk` skill defines — that relabel, not the close, is what takes the item off the
  driver's queue. Close authority, and the rule that a decided issue's close comment must name
  the tracker carrying the remaining work, are that skill's; follow them there.

Where the project keeps a decision log — an operation's drive-plan record, a register, a
stream doc — the relay is copied there too, with the issue as its source. This skill does not
define that log; it feeds it.

## Verification, then the next item

Do not present item `k+1` until item `k`'s act is verified:

1. Re-read the issue and confirm the relay comment is there and the label moved.
2. Confirm the act the ruling authorised actually happened (the PR flipped, the workflow ran,
   the file changed) — by reading the artifact, not by asserting it.
3. If either check could not be made, say **could-not-check** and say so to the driver in one
   line. An unverified act is not a completed one, and rolling it up as done is how a queue
   comes back a week later.

Then present `k+1`.

## Rendering the queue as a page

For decisions the driver wants to read away from a terminal:

```
bash "${CLAUDE_PLUGIN_ROOT}/scripts/assay-inbox.sh" --html /path/to/inbox.html owner/repo
```

One self-contained file — inline CSS, no scripts, no external assets, the only links are the
issues themselves, light/dark via `prefers-color-scheme`. It renders every queued item as a
card in the same five-part format, so the page and the terminal cannot say different things.
It is a READING surface: the ruling still comes back through the conversation and is still
recorded on the issue as a relay.

## The floors

- **The desk never decides a ONE-WAY gate.** A recommended default is a recommendation; a strong
  model is not a human gate, and this skill's presence in a session is not an authorisation to proceed
  unanswered on a fork whose wrong guess is irreversible or reaches outside the driver's gate. Where
  the fork is REVERSIBLE the desk has already proceeded on the default behind a draft PR (the
  reversibility test) — the reply is merge-or-decline, not a belated go-ahead.
- **Ask about what you have read.** If the inbox could not read an item, it renders
  `could-not-check` and exits non-zero. Present that item as unread — do not skip it (it is
  still waiting) and do not summarise it from memory.
- **Never ask a question the desk can answer.** Anything resolvable by reading the repo, the
  CI log, or the spec is desk work, and putting it in this queue spends the driver's turn on
  the desk's homework.
- **Never ask a question twice.** Before presenting an item, check whether a ruling is already
  recorded on it. A re-asked decision reads as the desk not having listened.
