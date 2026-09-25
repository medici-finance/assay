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

Read the queue with the inbox, which already sorts it. Prefer the `deskinbox` verb (Go,
no `bash`/`jq` dependency, works on Windows — windows-port/13):

```
deskinbox walk --item 1 owner/repo [owner/repo ...]
```

If `deskinbox` is not on `PATH`, fall back to the bash oracle it was ported from — same
ordering, same five-part format (`<bundle>` is the installed Assay bundle's own directory;
each harness locates it its own way, and the expansion for yours is in
`../../references/<harness>.md`; substitute it before running):

```
bash <bundle>/scripts/assay-inbox.sh --walk --item 1 owner/repo [owner/repo ...]
```

**Before asking, read where the system is stuck:**
`bash <bundle>/scripts/assay-inbox.sh --flow` prints the pipeline stage by
stage with the bottleneck named, so an item's Context can say what it is actually holding up —
and so a question about a stage three steps downstream of the constraint can wait. (`--flow`
is not yet ported to `deskinbox` — windows-port/15.)

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

## The screen — four classes, and only one of them is asked

Before any item reaches the format below, the inbox classifies it. `--walk` puts only
**genuine** decisions to the driver; the other three classes are never asked, and never
silently dropped — each is counted in a tail line under every question and listed in full
by `--walk --screened`. Classes are tested in this order; the first that holds on
POSITIVE evidence wins, otherwise the item is genuine — an item the tool cannot read or
cannot classify (no known human-login list, say) is always genuine, because screening
only ever happens on evidence, never on its absence.

1. **already-ruled** — a comment from a known human login lands after the desk's own
   relay ("Ruling relayed from the driver…") or after the point the options were put. A
   desk relay ALONE is never a ruling — this is the exact mistake this screen exists to
   catch. **Desk action:** relabel and close per the label vocabulary, citing the ruling
   comment; never re-ask.
2. **no-fork** — the Options section (or the fork-test block) parses to fewer than two
   entries. One workable option is not a fork; it is a plan already picked. **Desk
   action:** proceed or re-route, and notify — do not park it on this skill.
3. **reversible-default** — the item carries a live `caught-by:`, a `default:`, and no
   `class:` line and no one-way term: the desk has already proceeded behind a gate the
   driver still holds a veto over. **Desk action:** proceed behind the gate; notify, and
   let the veto stand.
4. **genuine** — everything else. **Desk action:** ask, per the format below.

The two floors at the end of this skill ("never ask a question twice", "never ask a
question the desk can answer") are this screen, not a habit to remember by hand — the
tool, not recall, is what tests classes 1–3 before anything is put to the driver.

## The format — five parts for every GENUINE item

The script renders exactly this for every item it classifies genuine; when you compose
an item by hand, compose the same shape.

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
   An option with no stated consequence is not an option, it is a label. **An item with one
   workable option is not asked** — the screen above classifies it `no-fork` before it ever
   reaches this format; never pad a single real option with a second one just to fill the
   list.
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

## Ratification — a relay is not yet a ruling

The relay above records an answer; it is not itself the ruling. Until the driver ratifies it in
their own identity, what stands on the issue is a well-formed record of something the desk said
the driver said — which is exactly the artifact a confidently-worded comment can counterfeit.
Ratification is what makes the distinction visible on the issue itself, independent of how the
relaying session worded anything: the issue carries the state, not the prose.

The ratification relay comment has five parts, without exception:

1. **The relay header.** Names the driver's answer as relayed, with the date it was given.
2. **The disclaimer.** States explicitly that this comment is a relay RECORD and not itself the
   ruling — the ruling is what the ratifying identity writes.
3. **The chosen option, with the rationale as given.** The letter or word answered, and the
   reasoning the driver actually voiced — never an inferred one. If no rationale was given,
   the part says so rather than supplying one.
4. **What happens once ratified.** Who does what next, and which label comes off — stated so
   the ratifier can see, before acting, exactly what ratifying authorises.
5. **The state line.** Awaiting ratification, and by whom.

**Ratification is an act only the driver can perform.** No agent ratifies on the driver's
behalf under any phrasing: the ratifying comment is written in the ratifying identity's own
hand. Being an act owed to the driver, it is handed over as a `RUNSHEET.md` entry in the
`human-runsheet` skill's four-part shape — the exact act, why it is owed, what the desk did
instead, and the resume step. That skill owns the entry shape; this section does not restate
it.

**A relay is not a wait state for the desk.** Record the relay, move the labels the recording
section above allows, and continue with the queue: the issue itself holds the
awaiting-ratification state, visible to any reader, and the desk does not park on it. What the
desk must not do is act on the relayed answer as if ratified where the gate is a human gate —
a relayed ruling authorises nothing a human gate still holds.

**After ratification: the amendment dispatch.** The follow-on work the ruling implies is
dispatched in the normal way, citing the ratified comment — `deskdispatch` is the verb, and
its flags belong to that verb's own documentation. Until the ratifying comment lands, the
amendment does not go out on the relay alone.

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

For decisions the driver wants to read away from a terminal (`--html` is not yet ported to
`deskinbox` — windows-port/15; this still runs the bash oracle):

```
bash <bundle>/scripts/assay-inbox.sh --html /path/to/inbox.html owner/repo
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
  the desk's homework. The `no-fork` and `reversible-default` classes in "The screen" above are
  this floor made mechanical — an item with one workable option, or already proceeding behind a
  held gate, is desk work, not a question.
- **Never ask a question twice.** Before presenting an item, check whether a ruling is already
  recorded on it. A re-asked decision reads as the desk not having listened. The `already-ruled`
  class above is this floor made mechanical, not a check to remember by hand: it is what caught
  the driver being asked to re-confirm a ruling already given, and being walked through a
  settled design as "do it / drop it" — the incident that is this screen's whole reason for
  existing. Trust the tool's classification over your own recall of the thread.
