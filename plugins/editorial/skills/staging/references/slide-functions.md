# Slot functions — a closed vocabulary of 18

A **slot** is one unit of attention: a slide in a deck, a scene in a video.
Every slot has exactly one function. The vocabulary is **closed** — if a slot
does not fit one of these eighteen, the slot is doing two jobs and must be
split, or doing none and must be cut.

Seven names match the vocabulary slide engines already use (`cover`, `intro`,
`section`, `statement`, `fact`, `quote`, `end`), so a staged deck maps onto a
Slidev/Marp layout name without translation. Eleven are extensions the engines
do not have, because engines model geometry and this vocabulary models
argument.

Each function carries a **sequencing rule**: what may precede it, and what it
must not be separated from. The rules exist because a function's meaning
depends on its neighbours — an `objection` slot with no `claim` behind it is
just a list of doubts.

---

## The seven engine-compatible functions

### `cover`
The artifact's identity: what this is, whose it is, when.
- **May precede:** nothing. Always slot 1.
- **Must not split from:** the slot after it. Cover then a full-bleed agenda
  is two slots of nothing; cover then the thesis is a start.
- **Claim rule:** the cover still carries a claim. A cover whose headline is
  the subject noun ("Queues") wastes the highest-attention moment in the
  artifact.

### `intro`
Orientation: where we are, who is speaking, why now. The *situation* beat.
- **May precede:** `cover` only.
- **Must not split from:** the first turn. An `intro` that is not followed
  within one or two slots by something that disturbs it is the classic
  early-sag failure.
- **Budget:** one slot, two at the absolute outside. Orientation is the beat
  that most reliably eats a deck alive.

### `section`
A boundary marker: the argument is changing direction and you are telling
people so.
- **May precede:** any slot except `cover`.
- **Must not split from:** the slot it introduces.
- **Rule:** a `section` slot must name a *direction*, not a *topic*. "Part
  two: the mechanism" is a table of contents. "Now the part where it refuses"
  is a direction. In artifacts under ~12 slots, `section` is usually
  unaffordable — the transition line does the work instead.

### `statement`
One sentence, standing alone, that the audience is meant to carry out of the
room. The thesis; the reframe; the verdict.
- **May precede:** anything.
- **Must not split from:** nothing — a `statement` is deliberately isolated.
  Give it its own slot or do not give it one at all.
- **Budget:** at most three per artifact (open, turn, close). Four statements
  means none of them is the statement.

### `fact`
A single number, date, or named event, presented as the thing itself.
- **May precede:** any slot.
- **Must not split from:** the `claim` it supports — a fact that arrives more
  than one slot away from what it proves reads as trivia.
- **Rule:** one fact per `fact` slot. A `fact` slot with five numbers on it is
  an `evidence` slot that has not admitted it.

### `quote`
Someone else's words, load-bearing.
- **May precede:** any slot.
- **Must not split from:** the claim it authorizes.
- **Rule:** a quote must do work the presenter cannot do — supply outside
  authority, supply a hostile view fairly, or supply the exact wording of a
  ruling. A quote used for decoration fails the dead-slot test.

### `end`
The last slot. Where the artifact stops.
- **May precede:** everything.
- **Must not split from:** the `ask` — `ask` then `end` is the pair; `ask`,
  thank-you, contact, questions, `end` is four slots of decay.
- **Rule:** close with the use of what was learned, or a synthesis that makes a
  comparison or next decision possible. A bare list of earlier points or
  "Questions?" discards the last high-attention moment. For `teach-workshop`,
  checked transfer and a usable next task can supply the close without a sales ask.

---

## The eleven extensions

### `claim`
An assertion the artifact intends to prove, stated as a complete declarative
sentence.
- **May precede:** any slot.
- **Must not split from:** its first piece of support. `claim → claim → claim
  → evidence` is a manifesto; the audience stops tracking which claim the
  evidence lands on.
- **Rule:** every `claim` slot must be followed, within two slots, by
  `evidence`, `mechanism`, or `fact`. An unsupported claim is a slogan.

### `evidence`
Support for the claim behind it: data, a record, a citation, a demonstration.
- **May precede:** `claim`, `mechanism`, `comparison`, `objection`, `practice` — never
  `cover` or `intro`.
- **Must not split from:** the claim above it. If a `section` boundary falls
  between a claim and its evidence, the boundary is in the wrong place.
- **Density rule:** this is the one function whose density is set by the room
  and not by taste. In **table** lane, dense is correct. In **hall** lane, one
  piece of evidence per slot, and everything else routes to a companion
  document.

### `comparison`
Two or more things set against each other on stated axes: before/after,
ours/theirs, option A/B/C.
- **May precede:** any slot after `intro`.
- **Must not split from:** itself — a comparison spread across two slots is
  not a comparison, because the audience can only compare what is
  simultaneously present. **This is the hardest sequencing rule in the set and
  the most often broken.**
- **Rule:** the axes must be stated, and they must be the axes the *audience*
  would choose. A comparison on axes you picked because you win on them is
  read instantly as a sales table.

### `mechanism`
How the thing works. The moving parts and the order they move in.
- **May precede:** `claim`, `statement`, `comparison`, `objection`.
- **Tutorial entry/checkpoint:** may also follow `cover`, `intro`, `practice` or
  `evidence` when continuing the declared task sequence.
- **Must not split from:** its own sequence. If a mechanism takes four slots,
  those four slots are one unit. In `teach-workshop`, a practice/feedback pair
  may interrupt at a declared checkpoint with the same task state retained.
  Unrelated facts, quotes and section markers still break the sequence.
- **Rule:** mechanism slots run in *the order the mechanism runs*, never in
  the order the material was written. In the video lane, the reveal order is
  the argument order: nothing appears on screen before the narration reaches
  it. Keep already explained objects visible when they anchor the next step or
  a comparison; remove them when they no longer help.

### `practice`
The learner attempts, predicts or diagnoses something before seeing the answer.
- **May precede:** an explanation or worked example that supplies the needed rule;
  a declared diagnostic pre-test may instead precede instruction.
- **Must not split from:** feedback in the next `evidence` or `mechanism` slot.
  Give the learner an explicit opportunity to act before revealing that feedback.
- **Rule:** specify starting state, prompt, observable success, work time and
  answer/recovery in the `teach-workshop` learning contract. A "try it yourself"
  slide without these is an ask, not practice. No new renderer layout is implied.

### `objection`
The audience's next most likely doubt, stated in their words, then answered.
- **May precede:** `claim`, `mechanism`, `evidence`, `comparison`.
- **Must not split from:** its answer. An objection slot that raises a doubt
  and moves on has *strengthened* the doubt.
- **Rule:** objections run in **descending likelihood**, and the first one is
  the one that would kill the argument. Saving the strongest objection for
  last is how a room spends the whole middle waiting for you to get to it.
  See the objection loop in `audience.md`.

### `pivot`
The turn. The slot where the artifact stops going in one direction and starts
going in another — usually from *this is the problem* to *this is the shape of
the answer*, or from *what you assumed* to *what is actually true*.
- **May precede:** anything that established the first direction.
- **Must not split from:** the direction it turns into.
- **Rule:** exactly one `pivot` per artifact under fifteen slots. Two pivots
  reads as an argument that changed its mind.

### `decision`
The choice being put in front of the audience, with its options and their
consequences.
- **May precede:** `evidence`, `comparison`, `objection` — never `intro`.
- **Must not split from:** the `ask`.
- **Rule:** only appears when someone in the room actually holds the decision.
  Staging a `decision` slot for an audience with no authority to decide is the
  most demoralizing slot in business presenting.

### `cost`
What this costs, specifically: a price, a fee, a rate — the number a buyer
needs before an ask can be real.
- **May precede:** `ask` only.
- **Must not split from:** the `ask` it sits beside — it belongs with the ask,
  not inside the `mechanism` block. A `cost` slot buried among mechanism
  slots reads as apologetic; a `cost` slot separated from the ask by other
  material reads as a number the deck hoped nobody would connect to the
  request that follows it.
- **Rule:** at most one `cost` slot per artifact, and its figure must be the
  same figure the artifact's `number_gate` carries for this price — a `cost`
  slot may not introduce a number the gate has never seen. Genres that sell a
  specific priced thing (`product-pitch`) make this slot REQUIRED; the ones
  that do not name a price at all may omit it, but may not gesture at cost
  without stating it — see `audience.md`'s "name the cost" rule.

### `ask`
What you want, specifically, with a name and a date on it.
- **May precede:** everything.
- **Must not split from:** `end`.
- **Rule:** one ask. Not three. If there are genuinely three, they are one ask
  with three parts, and you must name the first step.

### `appendix`
Material deliberately outside the arc: reference tables, derivations, the
detail one person in the room will want.
- **May precede:** `end` only — appendix material never sits inside the arc.
- **Must not split from:** nothing.
- **Rule:** **illegal in the hall lane.** In table lane it is not only legal
  but load-bearing: the appendix is what lets the arc stay short. Anything
  that does not fit the storyline goes here rather than getting squeezed in.
  Appendix slots are excluded from the time-shape arithmetic — they are read,
  never walked.

---

## Quick sequencing checks

Run these against the slot list before the gates:

- Does any `comparison` span two slots? → merge or cut one side.
- Does any `mechanism` run have foreign material inserted? → move it out unless
  it is the tutorial's declared practice/feedback checkpoint.
- Is any `objection` more than one slot from its answer? → close the gap.
- Is any `claim` unsupported within two slots? → support it or demote it.
- Is there more than one `pivot`? → one is the pivot; the other is a
  `statement`.
- Is `appendix` present in a hall-lane artifact? → cut it to a companion doc.
- Do `ask` and `end` sit adjacent? → they must when an ask is used; a tutorial
  may close from checked transfer to `end`.
- Does each `practice` have an attempt opportunity before its adjacent feedback?
- Is `cost` more than one slot from `ask`, or inside the `mechanism` run? →
  move it beside the ask.
