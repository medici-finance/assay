# Genre playbook — `executive-explainer`

```yaml
id: executive-explainer
name: Executive explainer
mandate: >
  Make a non-specialist decision-holder understand what changes and why it
  matters to them — well enough to repeat it accurately to someone who was
  not in the room.
default_lane: hall-leaning mixed   # sparse slots, one idea each; but declare
                                   # read-fallback, because it will be forwarded
```

The success test for this genre is not "did they follow it." It is **can they
retell it**. Everything below is arranged around that.

## Audience

```yaml
audience:
  assumed_knowledge: >
    They know their own domain, not yours. They can hold ONE new mechanism if
    it is concrete and arrives in the middle. They cannot hold four, and a
    second mechanism displaces the first rather than adding to it.
  what_they_distrust:
    - enthusiasm with no downside attached
    - numbers whose provenance is not named
    - being sold to under the description of being informed
    - anything that seems to require them to take the speaker's word
    - being asked to decide something they were not given the basis to decide
  decision_they_hold: >
    Ranges from a resource decision to nothing more than a credibility
    judgment. Name which. The ending differs completely between them, and the
    single most common failure of this genre is staging a resource ask at an
    audience that holds only a credibility judgment.
  attention_budget: >
    ~10 minutes live; far less if the material feels like it is warming up.
    They will decide within the first minute whether this is for them, and
    that decision is very hard to reverse later in the artifact.
```

## Selection

```yaml
pick_when:
  - the audience decides about the thing but does not build it
  - what matters is the consequence, and the mechanism is only needed as far
    as it makes the consequence believable
  - the artifact must survive being retold by someone who half-listened
skip_when:
  - the audience can and will evaluate the mechanism → technical-deep-dive
  - money is being requested from funders → investor-pitch
  - a governing body is exercising oversight on a cadence → board-update
  - a purchase is being progressed → sales-proposal
  - the register the room rewards is inspiration, not explanation →
    conference-keynote
instead_use:
  technical-deep-dive: >
    when skipping mechanism would leave the audience unable to argue with it,
    AND they want to argue with it
  investor-pitch: the ask is capital
  internal-decision: a named person chooses today, in the room
```

## Arc — 5 beats

```yaml
arc:
  - beat: common-ground
    intent: >
      Say the true, uncontroversial thing about the current world — including
      the part where the incumbent works. Earn the nod.
    minutes_share: 0.11
  - beat: change
    intent: >
      What is now different, and why the old arrangement fails under it. Not
      "the old thing is bad" — "the old thing was built for a case that is no
      longer the case."
    minutes_share: 0.22
  - beat: consequence
    intent: >
      The cost, made specific and singular. One instance, not a category.
      ← the bold moment lives here
    minutes_share: 0.17
  - beat: move
    intent: >
      What is done about it, at the level of what changes — mechanism only as
      deep as belief requires.
    minutes_share: 0.28
  - beat: standing
    intent: >
      What is real today vs what is intended, the honest limit, and the ask.
    minutes_share: 0.22
```

Beat notes:

- **`common-ground` must concede.** An explainer that opens by attacking the
  incumbent loses everyone who chose it. Open by agreeing it works, then
  change the conditions.
- **`change` is where the persuasion happens.** Under-slotting it is the
  characteristic failure of this genre: solutions get four slots and the
  problem gets one, and the room never wanted the solution because it never
  felt the problem.
- **`consequence` is one instance.** A category ("this creates risk") is
  forgettable; an instance is retellable, and retellability is the mandate.
- **`move` states the mechanism at the shallowest depth that supports belief
  — and no shallower.** "It's more secure" is too shallow: it asks for trust.
  One nameable structural property is the right depth.
- **`standing` is where the honesty lives.** What is built vs designed, what
  it costs, what stays unsolved.

## Slot sequence

```yaml
slide_sequence:
  - {slot: 1,  function: cover,      beat: common-ground, required: true}
  - {slot: 2,  function: statement,  beat: common-ground, required: true}  # the arguable sentence, conceding first
  - {slot: 3,  function: fact,       beat: change,        required: true}  # what is now different — one number/scale fact
  - {slot: 4,  function: comparison, beat: change,        required: true}  # the seams, one line of consequence each
  - {slot: 5,  function: fact,       beat: consequence,   required: true}  # ← the bold moment: ONE named instance
  - {slot: 6,  function: pivot,      beat: consequence,   required: true}  # structural, not disciplinary
  - {slot: 7,  function: mechanism,  beat: move,          required: true}  # what is actually done — ONE slot
  - {slot: 8,  function: comparison, beat: move,          required: true}  # before/after in THEIR terms
  - {slot: 9,  function: objection,  beat: standing,      required: true}  # the cost/limit, volunteered
  - {slot: 10, function: ask,        beat: standing,      required: true}
  - {slot: 11, function: end,        beat: standing,      required: true}
```

Sequencing constraints specific to this genre:

- **`mechanism` gets ONE slot.** If the mechanism needs four slots, either the
  audience is more technical than classified (→ re-CLASSIFY) or the mechanism
  is being explained at a depth the mandate does not require.
- **No `appendix`.** Hall lane. Depth routes to a companion document, which
  the ask may point at.
- **No `section` slots** at this length. The transition line does the work.
- Slots 5 and 6 are a pair: the instance, then what it means. Do not separate.

## Bold moment

```yaml
bold_moment:
  beat: consequence
  form: >
    One named, dated, specific instance of the cost — told plainly, in one
    breath, with no hedging and no vocabulary the room does not have. A real
    thing that happened to real people, ideally to us.
  why: >
    This is the beat that gets retold. A decision-holder who retells the
    artifact retells the instance, and the instance carries the argument with
    it. Categories do not survive retelling; instances do.
  placement: >
    At the LAST problem beat, immediately before the pivot — never inside the
    solution. Two rules follow, and they hold in BOTH media:
    (1) it must arrive AFTER the audience has enough context to feel it and
    BEFORE any relief is offered, so the room sits with it for one beat;
    (2) it must not be a product moment. A bold moment inside the solution
    beat makes the solution the loudest thing in the artifact, and the room
    reads the whole piece as a pitch — after which the problem beats retro-
    actively become setup, and nothing is believed.
  placement_deck: >
    Roughly 40-50% through the SLOT SEQUENCE, by index. Slot 5 of 11 = 45%.
  placement_video: >
    Initial plan: 20–42% by time, derived from this lane's runtime shares.
    Record cumulative time for the chosen arc rather than borrowing deck indices.
    Revise the placement explicitly after rough-cut review when that improves
    understanding; retain the agreed runtime, evidence and consequential limits.
  count: 1
```

## Time shape

```yaml
time_shape:
  deck:
    minutes: {min: 4, ideal: 6, max: 10}
    seconds_per_slot_floor: 40      # hall lane: sparse slots need SPOKEN time
    slides: {min: 8, ideal: 11, max: 14}
    presented_vs_read: presented-primary, read-fallback
    minutes_per_beat_at_6min:
      common-ground: 0.67
      change: 1.33
      consequence: 1.0
      move: 1.67
      standing: 1.33
    arithmetic: >
      6 min × 60 = 360s ÷ 9 content slots = 40s/slot, exactly at the floor.
      A 14-slot version of the same 6 minutes gives 25s/slot — below floor,
      and the visible symptom is a presenter who never finishes a sentence.
      Cut to 9.
    read_fallback_rule: >
      Because it WILL be forwarded, every headline claim must be a complete
      sentence that stands without the narrator, even though the slots are
      sparse. Sparse ≠ cryptic.
  video:
    seconds: {min: 60, ideal: 90, max: 120}
    cold_open: 5-15s, before any title; the promise lands in the first 10-15s
    runtime_shares: {problem: 0.35, move: 0.40, proof: 0.15, ask: 0.10}
    word_budget: >
      ~150 wpm. 90s ≈ 210-230 words total, for the WHOLE piece. This is the
      constraint that makes video staging zero-sum: adding a beat deletes a
      beat, and the deck's 5 beats usually compress to 4.
    devices:
      - spend 30-40% on the problem before naming the solution; the solution
        gets roughly twice the problem's runtime only AFTER that
      - the credibility bridge — an explicit beat between "we understand this
        problem" and "we have standing to solve it". Short explainers drop it
        and the piece reads as generic
      - one message, one ask
      - the cost beat survives compression: keeping it is the trust signal
      - final image answers the opening image; the viewer saw it 80s ago
    cover_becomes: >
      There is no `cover` slot in a film. It dissolves into TWO things: a cold
      open carrying the promise (before any title), and a title that lands at
      scene 2 or later. Say which scene the title is on.
    motion_governor:
      status: semantic intent required; treatment developed through the rough cut
      fields_per_scene: [motion_role, energy, peak, motion_turn, figures]
      role_vocabulary: the eight in references/motion-roles.md
      rules: R1–R6 in references/motion-roles.md; no numerical taste thresholds
      treatment: >
        Explore the consequential relationship and the audience's likely wrong
        inference. Choose emphasis, contrast and inspection time from playback.
        A restrained treatment is available in references/motion-spec.md; it is
        a preset, not a required peak count, figure cap or animation style.
```

## Opening and closing moves

```yaml
opening_move:
  name: concede, then change the conditions
  do: >
    Slot 2 states plainly that the incumbent arrangement works, and works for
    the reason it was designed for. Then slot 3 changes the conditions.
  avoids: >
    Two failures at once. First, the attack open — starting on what is wrong
    with the thing the audience currently uses and chose, which makes
    defending it their first thought. Second, the credentials open — who we
    are, why we are qualified — which spends the highest-attention moment in
    the artifact on the speaker rather than on the audience's world.

closing_move:
  name: name the limit, then the smallest real step
  do: >
    State what this is NOT yet, then ask for the smallest thing that is
    genuinely useful to you — an argument, a review, a conversation, a pilot.
  avoids: >
    The oversell close, which is the specific failure this audience is
    watching for: the artifact that has no downside anywhere in it, whose ask
    is therefore large. Volunteering the limit immediately before the ask
    makes the ask credible, and shrinking the ask makes it grantable. It also
    avoids the vanish close (a summary slide, a thank-you, a logo) which asks
    for nothing at the moment attention peaks.
```

## Transitions

```yaml
transitions:
  - {after_beat: common-ground, type: escalate, note: "the conditions changed; the nod becomes a frown"}
  - {after_beat: change,        type: narrow,   note: "from four seams to the one that already cost us"}
  - {after_beat: consequence,   type: turn,     note: "the single pivot: structure, not discipline"}
  - {after_beat: move,          type: fold,     note: "the parts become one property they can retell"}
  - {after_beat: standing,      type: gate,     note: "the only place the artifact asks"}
```

## Kill conditions

```yaml
kill_conditions:
  - condition: the audience holds no decision and no judgment worth changing
    do: this is a briefing; write a one-page note instead
  - condition: the mechanism cannot be made believable in one slot
    do: >
      Either the audience is technical (→ technical-deep-dive) or the claim
      is not yet supportable at this depth. Do not solve it with four slots.
  - condition: there is no specific instance of the cost, only a category
    do: >
      There is no bold moment and the piece will not be retellable. Find the
      instance in the record, or reduce the claim to what you can instance.
  - condition: the ask is capital
    do: re-CLASSIFY as investor-pitch
  - condition: the honest answer to "what is built?" is "nothing yet"
    do: >
      Not a kill — but `standing` must say so plainly and the ask must be
      correspondingly small. An explainer that hides its maturity is the one
      thing this audience never forgives.
```

## Anti-patterns

```yaml
anti_patterns:
  - symptom: the room agrees warmly and cannot retell it the next day
    cause: consequence stated as a category, not an instance
    fix: replace with one named, dated event; cut a solution slot to pay for it
  - symptom: it reads as a pitch even though nothing is being sold
    cause: bold moment placed inside the solution beat
    fix: move it to the last problem beat; the solution goes quiet
  - symptom: the problem gets one slot and the solution gets five
    cause: authored from the solution outward
    fix: >
      Restore the 30-40% problem share. The room cannot want an answer to a
      question it has not been made to ask.
  - symptom: someone in the room asks "so what do you want from us?"
    cause: gate transition missing; the artifact stopped instead of asking
    fix: add the gate boundary before the ask; make the ask one thing
  - symptom: the deck is forwarded and lands as a set of pretty slogans
    cause: sparse slots with fragment headlines; read-fallback ignored
    fix: >
      Complete-sentence claims on every slot. Sparse and self-standing are
      compatible; sparse and cryptic is a staging failure.
  - symptom: four mechanisms explained, none remembered
    cause: mechanism over-slotted against the mandate
    fix: one slot, one nameable property; the rest to the companion document
```

## Gates

```yaml
gates:
  - id: helicopter
    check: headline claims alone, in order, form the argument
  - id: retell
    check: >
      Could a listener who half-listened retell this in three sentences?
      Name those three sentences. If you cannot, the arc is not staged.
  - id: problem-share
    check: problem beats hold ≥30% of the time budget
  - id: bold-before-relief
    check: >
      DECK: before the move beat, 40-50% by index. VIDEO: the consequence is
      understandable before relief is offered; verify the recorded arc and
      cumulative timing after rough-cut review, not a fixed percentage.
  - id: motion-budget          # VIDEO lane only
    check: >
      the motion_budget review in references/motion-roles.md: semantic purpose,
      contrast, emphasis, repetition, rhythm and attention; no numerical taste gate
  - id: ornament               # VIDEO lane only
    check: >
      figures have useful jobs and readable hierarchy; record actual findings
  - id: motion-language-reviewed     # VIDEO lane only
    check: >
      references/motion-spec.md selected after exploration, and its
      audit table run against the finished scenes
  - id: seams-declared         # VIDEO lane only
    check: >
      every scene boundary in a seams ledger and seam-gate PASSING (0 fail)
  - id: one-mechanism
    check: exactly one mechanism slot
  - id: limit-stated
    check: a slot volunteers the cost/limit before the ask
  - id: ask-matches-authority
    check: the ask is something this audience can actually grant
  - id: dead-slot
    check: every slot turns a value
  - id: time-shape
    check: minutes × 60 ÷ content_slots ≥ 40s; show the arithmetic
  - id: standalone
    check: every headline claim is a complete sentence (read-fallback)
```
