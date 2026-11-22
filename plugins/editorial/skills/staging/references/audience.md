# Audience — four dimensions, one fork, two rules

Audience work is not demographics. Nothing here asks what industry they are in
or how senior they are. It asks what they already know, what they will not
take on trust, what they are allowed to decide, and how long they will stay.

Those four answers set **density**, **order**, and **length**. Everything else
about an audience is colour.

---

## The four dimensions

### 1. Assumed knowledge — what you may skip

Write the sentence you are allowed to *not explain*. For an audience of git
users you may write "content-addressed history" and move on. For a
technical-adjacent audience you may write "version control" and must explain
the rest.

Two failure modes, and they are opposites:

- **Explaining what they know** burns attention budget at the front, where
  attention is most expensive, and reads as condescension. It is the most
  common structural fault in technical decks.
- **Assuming what they do not know** — the knowledge curse — loses them
  silently. They will not interrupt to say so. They will disengage and be
  polite about it.

The mitigation is not "explain everything briefly." It is: pick the level, say
it out loud in the staging artifact, and hold it. `assumed_knowledge` in
`staging.yaml` is a contract with yourself.

### 2. What they distrust — where the proof must go

Every audience has a default suspicion, and it is specific:

| Audience | Distrusts by reflex |
|---|---|
| Engineers | claims about mechanism with no mechanism shown; hand-waved tradeoffs; anything that sounds like it has never been run |
| Executives | numbers with no provenance; a plan with no cost; enthusiasm without a downside |
| Buyers | a comparison table where you win every row |
| Regulators / auditors | anything whose evidence is your own application code |
| Practitioners of the thing you are describing | that you have actually done it |

**Proof goes where the distrust is, not where the material is thickest.** A
deck that spends four slots proving something the room already accepts, and
one slot on the thing they doubt, has inverted its own evidence budget. This
is a staging decision, not a research decision — it changes *slot allocation*.

The corollary: **name the cost.** An artifact that presents only supporting
evidence reads as advocacy, and a technical audience discounts the whole thing
by the size of the omission they can smell. Include the contrary evidence you
would rather not — you will have to deal with it in the room anyway, and
dealing with it in the artifact is the only version where you choose the
framing.

### 3. The decision they hold — what the artifact may ask for

Be exact. There are only a few kinds, and they demand different endings:

- **A resource decision** (money, people, time) → the artifact ends on
  `decision` + `ask`.
- **A go/no-go on a specific proposal** → same, and the objection loop must
  be exhaustive, because the room will use any unanswered objection as a
  reason to defer.
- **A credibility judgment** ("should I take this seriously?") → the artifact
  ends on an invitation to *engage*, not to approve. Staging an approval-ask
  at an audience that holds no approval authority is a category error, and
  they feel it.
- **A behaviour change** (do the thing differently on Monday) → the artifact
  ends on the first concrete step, not on the principle.
- **Nothing at all** → this is a briefing. Ask hard whether it should be a
  document instead. If neither belief nor behaviour changes, there is nothing
  to stage.

### 4. Attention budget — the real ceiling

Not the meeting length. The length of time they will *track an argument*
before they need something to change.

Working numbers:

| Setting | Budget before a gear change is required |
|---|---|
| Live room, presented | ~10 minutes |
| Video, online | ~3–4 minutes; hard segmentation ceiling ~6 minutes |
| Read alone, self-paced | effectively unbounded, but abandonment is instant and silent |

A **gear change** is a deliberate discontinuity: a demonstration, a story, a
question put to the room, a shift from claim to mechanism, a change of medium.
It is a *staging* device — it lives in the slot sequence — not a delivery
trick.

If the material needs 40 minutes and the budget is 10, the answer is not
faster delivery. It is fewer beats at more depth, with the rest routed to a
document. Depth over breadth survives compression; breadth does not.

---

## The fork: presented vs read

Decide this before the slot sequence, because it changes what a slot *is*.

| | **Presented** | **Read** |
|---|---|---|
| Who supplies context | the narrator | the artifact |
| Slot density | low — the slot supports the speech | high — the slot is the argument |
| Headline claims | may be terse; the voice completes them | must be complete sentences that stand alone |
| Evidence | pointed at, then explained | present, readable, footnoted |
| Appendix | not walked | the reason the arc can stay short |
| Failure if wrong | a wall of text nobody reads while you talk over it | a set of evocative slides that mean nothing without you |

Apply [staging’s lane decision](../SKILL.md) in CLASSIFY, Move 1. When a brief
mentions forwarding the deck, identify who receives it and what they need
to do with it. Use the table above to decide what context those readers will
need on the page.

---

## Rule 1 — the objection loop

**Each later section answers the audience's next most likely objection.**

The structure of a persuasive middle is not "point one, point two, point
three." It is a loop: something is unsatisfactory → here is the resolution →
here is the concrete case → *and that raises the next doubt* → resolution →
case → and so on, until the doubts run out.

The mechanics:

1. Before staging the middle, **list the objections in the audience's own
   words**. Not your rebuttals — their sentences. Six to eight is normal.
2. Sort by likelihood, descending. The first is the one that would kill the
   argument if it went unanswered.
3. Each middle section takes the next objection in order and resolves it,
   ending on a concrete instance.
4. Stop when the remaining objections are ones this audience will not raise.
   Answering objections nobody has is how a deck doubles in length.

Two consequences worth stating:

- **The orientation beat happens once.** You establish the shared situation at
  the start and never re-establish it. Every subsequent cycle opens on a
  complication, not on context.
- **Early cycles build the argument; later cycles defend it.** If your
  strongest objection is the third one, the room spends the first two waiting.
  Move it up.

## Rule 2 — end each section owing

**A section boundary must land on tension, not on resolution.**

The rule: never hand the audience a piece of information before you have
created their need for it. The unit that creates the need is the complication
— the thing that is now unsatisfactory. So a section ends by *opening*
something, and the next section pays it down.

Concretely, at every boundary the last slot of the outgoing section should
leave a question live. If the last slot of a section resolves everything, the
audience has been given permission to stop, and in a read-primary artifact
many of them will.

This is also the deck's version of the video device where a curiosity is
opened on a clock and paid down deliberately. Decks have no clock, so the
section boundary is where the debt is issued.

The two exceptions: the **first** boundary may resolve (you have just
established common ground and you want them nodding), and the **last** slot of
the artifact resolves by definition. Everywhere else, owe them something.
