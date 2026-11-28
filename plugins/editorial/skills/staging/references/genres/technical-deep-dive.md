# Genre playbook — `technical-deep-dive`

```yaml
id: technical-deep-dive
name: Technical deep dive
mandate: >
  Make a competent skeptic believe the mechanism is real, that you have run it
  or could, and that you know what it costs. Not to impress them — to be
  argued with on the merits.
default_lane: table          # fallback when the brief leaves audience use unspecified
```

Use the lane selected in [staging](../../SKILL.md), CLASSIFY, Move 1. The
technical genre determines the evidence the audience needs in either lane.

## Audience

```yaml
audience:
  assumed_knowledge: >
    They know the domain's primitives cold and will be insulted by an
    explanation of them. State the level explicitly and hold it; the whole
    genre's economy comes from what you are allowed to skip.
  what_they_distrust:
    - mechanism claimed but not shown
    - a design with no stated cost ("what does this make worse?")
    - anything that reads as never having been run against reality
    - benchmark/comparison axes chosen because you win on them
    - novelty claims — they will mentally map every part to prior art, and if
      you have not done that mapping out loud they will assume you cannot
  decision_they_hold: >
    Usually NOT approval. They hold a credibility judgment — is this worth
    engaging with — and, downstream, whether they will adopt or recommend.
    Staging an approval-ask at them is a category error.
  attention_budget: >
    High for mechanism, near-zero for preamble. They will read the artifact
    later, alone, more carefully than they listen to it now.
```

## Selection

```yaml
pick_when:
  - the audience can evaluate the mechanism and will
  - the artifact's value IS the mechanism and its tradeoffs
  - the material has a genuine cost/limit that can be stated honestly
  - the artifact will be forwarded to people who were not in the room
skip_when:
  - the audience cannot evaluate the mechanism → executive-explainer
  - one specific decision is being requested today → internal-decision
  - the goal is for them to be able to DO it afterwards → teach-workshop
  - the room is large, one-way and projected → conference-keynote
  - there is no honest cost to state (there always is; if you cannot find it,
    you do not understand the design well enough to present it)
instead_use:
  executive-explainer: audience holds budget/strategy, not implementation
  teach-workshop: success is measured by capability, not belief
  internal-decision: a named person must choose today
```

## Arc — 7 beats

```yaml
arc:
  - beat: ground
    intent: state the shared substrate and the level of abstraction, once
    minutes_share: 0.08
  - beat: rupture
    intent: the specific failure that actually happened, named and dated
    minutes_share: 0.11
  - beat: diagnosis
    intent: the root is structural, not disciplinary — this is the pivot
    minutes_share: 0.08
  - beat: mechanism
    intent: how it works, in the order it works; the largest block
    minutes_share: 0.33
  - beat: proof
    intent: the moment the mechanism is seen doing the thing
    minutes_share: 0.08
  - beat: cost
    intent: what this makes worse, what stays unsolved, what is still open
    minutes_share: 0.17
  - beat: standing
    intent: how it gets built and adopted; what you want from them
    minutes_share: 0.15
```

Beat notes:

- **`ground` is one slot.** Two is an early sag. The temptation to
  re-establish context for the least-expert reader is how this genre dies at
  the front.
- **`rupture` must be a named event, not a category.** "Review state is
  mutable" is a property. "This verdict was erased by that approval on that
  date" is a rupture. The genre lives or dies on this substitution.
- **`diagnosis` carries the artifact's one `pivot`.** It is the sentence that
  converts a list of incidents into one bug.
- **`mechanism` is the largest block and runs in mechanism order.** Nothing
  may be inserted into it.
- **`cost` is mandatory and non-negotiable in this genre.** See
  `anti_patterns`. Its absence is the strongest negative signal available to
  this audience and they read it instantly.

## Slot sequence

```yaml
slide_sequence:
  - {slot: 1,  function: cover,      beat: ground,     required: true}
  - {slot: 2,  function: statement,  beat: ground,     required: true}   # the arguable thesis
  - {slot: 3,  function: fact,       beat: rupture,    required: true}   # the named incident
  - {slot: 4,  function: claim,      beat: diagnosis,  required: true}   # the pivot claim
  - {slot: 5,  function: mechanism,  beat: mechanism,  required: true}
  - {slot: 6,  function: mechanism,  beat: mechanism,  required: true}
  - {slot: 7,  function: mechanism,  beat: mechanism,  required: false}
  - {slot: 8,  function: mechanism,  beat: mechanism,  required: false}
  - {slot: 9,  function: comparison, beat: mechanism,  required: true}   # old model vs new, on THEIR axes
  - {slot: 10, function: statement,  beat: proof,      required: true}   # ← the bold moment sits here
  - {slot: 11, function: objection,  beat: cost,       required: true}   # the killer objection first
  - {slot: 12, function: objection,  beat: cost,       required: true}   # the cost/limit, stated as theirs
  - {slot: 13, function: mechanism,  beat: standing,   required: true}   # how it gets adopted
  - {slot: 14, function: evidence,   beat: standing,   required: false}  # prior art / build-vs-reuse
  - {slot: 15, function: ask,        beat: standing,   required: true}
  - {slot: 16, function: end,        beat: standing,   required: true}
  - {slot: A1, function: appendix,   beat: null,       required: false}  # untimed presenter reference or reading copy
```

Sequencing constraints specific to this genre:

- Slots 5–8 are **one unit**. No fact, quote or section may be inserted.
- Slot 9 must be a single slot. A comparison split across two slots is not a
  comparison.
- Slot 11 carries the objection that would kill the argument, not the easiest
  one. The easy objections go in the appendix or nowhere.

## Bold moment

```yaml
bold_moment:
  beat: proof
  form: >
    Something the mechanism DOES, shown happening — a refusal, a transcript,
    a live failure, an artifact the audience can check. Not a strong sentence,
    not a striking number: an event.
  why: >
    This audience converts "described" into "believed" only on contact with an
    instance. The bold moment is the instance. It is also the only place in
    the genre where a rehearsed dramatic beat is not read as salesmanship —
    because what is dramatic is the machine's behaviour, not the presenter's.
  placement: >
    AFTER the mechanism block, never before it. The bold moment must be
    something the audience can now evaluate, and they can only evaluate it
    once they hold the mechanism. Placing it at the front — as an opening hook
    — spends the strongest asset on an audience not yet equipped to be
    impressed by it, and leaves the mechanism block with nothing to build
    toward. That ordering rule is the part that matters and it is medium-
    independent; the PERCENTAGE is not.
  placement_deck: >
    55-70% of the way through the SLOT SEQUENCE, by index. Slot 10 of 16 = 63%.
  placement_video: >
    Initial plan: 72–85% by time, derived from this lane's runtime shares.
    Record cumulative time for the chosen arc rather than borrowing deck indices.
    Revise the placement explicitly after rough-cut review when that improves
    understanding; retain the agreed runtime, evidence and consequential limits.
  count: 1
```

## Time shape

```yaml
time_shape:
  deck:
    minutes: {min: 6, ideal: 9, max: 15}
    seconds_per_slot_floor: 30      # table lane: dense slots need dwell
    slides: {min: 10, ideal: 16, max: 22}   # excluding appendix
    # This timing example assumes a table-lane briefing.
    minutes_per_beat_at_9min:
      ground: 0.75
      rupture: 1.0
      diagnosis: 0.75
      mechanism: 3.0
      proof: 0.75
      cost: 1.5
      standing: 1.25
    arithmetic: >
      9 min × 60 = 540s ÷ 14 content slots = ~38s/slot, above the 30s floor.
      At 20 slots the same 9 minutes gives 27s/slot — below floor. Cut slots.
    appendix: excluded from the arithmetic; it is read, never walked
  video:
    minutes: {min: 3, ideal: 5, max: 6}
    hard_ceiling_reason: >
      Engagement falls off sharply past roughly six minutes; a longer piece is
      several pieces. Segment rather than extend.
    cold_open: 20-40s, before any title
    runtime_shares: {rupture+diagnosis: 0.35, mechanism: 0.40, proof: 0.10, cost: 0.10, ask: 0.05}
    word_budget: 130-150 wpm — staging is zero-sum; adding a beat deletes a beat
    devices:
      - misconception first: surface the wrong model the viewer probably holds,
        let it be uncomfortable, THEN correct it. A clean exposition delivered
        to someone holding a wrong model underperforms saying nothing.
      - progressive reveal in narration order — nothing on screen before the
        voice reaches it
      - the cost beat is the trust tell; commercial video omits it and viewers
        of this genre notice
      - end on the cut. No summary, no "in conclusion", no fade.
    cover_becomes: >
      There is no `cover` slot in a film. It dissolves into a cold open
      (20-40s, before any title) plus a title that lands at scene 2 or 3.
      Both video lanes needed this and neither playbook used to say it.
    beats_with_no_share: >
      `ground` and `standing` get NO bucket in runtime_shares. That is not an
      oversight — it is the lane's largest single edit, and it fires the kill
      condition "the material needs 40 minutes and the slot is 10" with its
      own instruction attached: cut BEATS, not depth. Route the adoption path
      and the build-vs-reuse map to a companion document and have the ask
      point at it.
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
  name: cold thesis
  do: >
    Slot 2 is one arguable sentence, stated flat, with no warm-up and no
    agenda. Then immediately the named incident.
  avoids: >
    The preamble failure — credentials, agenda, roadmap, "a bit of background
    first". This audience's attention is highest in the first ninety seconds
    and lowest during orientation; an agenda slide spends the best moment on
    the least content. It also avoids the opposite failure, the mystery open,
    which reads as a magic trick to people whose job is to distrust magic.

closing_move:
  name: hand over the open questions
  do: >
    End by naming what is unresolved and asking them to resolve it — the
    specific open items, by number, with what a good answer would look like.
  avoids: >
    The finished-artifact failure: a summary close implies the design is done,
    which converts a potential critic into a passive reader. It also avoids
    ending on Q&A, which discards the last high-attention moment, and ending
    on a thank-you slide, which discards it twice.
```

## Transitions

```yaml
transitions:
  - {after_beat: ground,    type: escalate, note: "from the shared substrate to what broke on it"}
  - {after_beat: rupture,   type: turn,     note: "the artifact's single pivot: not discipline, structure"}
  - {after_beat: diagnosis, type: narrow,   note: "from the diagnosis into the first mechanism"}
  - {after_beat: mechanism, type: fold,     note: "the parts become one property — sets up the bold moment"}
  - {after_beat: proof,     type: escalate, note: "now the cost, in their terms, before they raise it"}
  - {after_beat: cost,      type: narrow,   note: "from limits to the concrete adoption path"}
  - {after_beat: standing,  type: gate,     note: "the one boundary where the artifact asks"}
```

## Kill conditions

```yaml
kill_conditions:
  - condition: no honest cost can be stated
    do: stop; you are staging advocacy. Go back to the material.
  - condition: the mechanism cannot be shown doing anything
    do: >
      There is no bold moment available. Either find the instance, or accept
      this is a design memo and ship a document instead of a deck.
  - condition: the audience cannot evaluate the mechanism
    do: re-CLASSIFY as executive-explainer
  - condition: a specific decision is required from a named person today
    do: re-CLASSIFY as internal-decision
  - condition: the material needs 40 minutes and the slot is 10
    do: >
      Cut BEATS, not depth. Fewer beats at full depth survives; all beats at
      one-third depth is the shape people leave without.
```

## Anti-patterns

```yaml
anti_patterns:
  - symptom: the deck reads as a tour of the document's table of contents
    cause: >
      The arc was inherited from the source material's section order instead
      of built from the audience's objection order.
    fix: >
      Rebuild the middle from the objection list (see audience.md rule 1).
      Section order is authoring order; objection order is reading order.
  - symptom: everything is impressive and nothing is doubted
    cause: only supporting evidence was selected
    fix: >
      Give the killer objection its own slot BEFORE the ask, in the
      audience's own words. The artifact gets shorter and more credible.
  - symptom: the audience agrees but does not engage afterwards
    cause: closed on summary; no open questions handed over
    fix: apply closing_move; name the unresolved items by number
  - symptom: the middle sags around the mechanism block
    cause: mechanism slots in authoring order, not in mechanism order
    fix: re-sequence so each slot is the next thing that actually happens
  - symptom: the strongest asset (a demo, a transcript, a refusal) opened the deck
    cause: bold moment placed as a hook
    fix: >
      Move it to the proof beat. Keep the named incident at the front as
      EVIDENCE — evidence and bold moment are different jobs and the front
      slot wants the former.
  - symptom: independent readers cannot interpret the forwarded deck
    cause: the brief called for a reading copy and essential context stayed in speaker notes
    fix: put the claims and their qualifications on the page and move detailed evidence to the appendix
```

## Gates

```yaml
gates:
  - id: helicopter
    check: the headline claims alone, in order, form the complete argument
  - id: mechanism-order
    check: mechanism slots run in the order the mechanism runs, uninterrupted
  - id: bold-after-mechanism
    check: >
      DECK: after the mechanism, 55-70% by index. VIDEO: the viewer has enough
      context to evaluate the focal discovery; verify the recorded arc and
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
  - id: cost-present
    check: at least one slot states what this makes worse, in the audience's terms
  - id: killer-objection-first
    check: the first objection slot carries the argument-killing doubt
  - id: no-approval-ask
    check: the ask matches the decision they actually hold
  - id: dead-slot
    check: every slot advances understanding; a recap must support retrieval, comparison or synthesis rather than repeat earlier sentences
  - id: time-shape
    check: minutes × 60 ÷ content_slots ≥ 30s; show the arithmetic
  - id: standalone
    check: for a read-primary brief, every headline claim is a complete sentence
```
