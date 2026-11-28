# Genre playbook — `product-pitch`

```yaml
id: product-pitch
name: Product pitch
mandate: >
  Make a buyer believe this specific, priced product is worth exactly what it
  costs — not the category, not the vision behind it, this thing, at this
  price, today. If the room cannot say the price back afterwards, the pitch
  did not happen.
default_lane: hall-leaning mixed   # presented once, forwarded to the rest of the buying group
```

Authored from the RWA deck bake-off's messaging verdict (2026-08-28): six
design tracks across three messaging contracts rendered a product pitch that
never said what it costs, because nothing in the transport had a slot for the
number or a gate that could fail on its absence. This playbook is that slot
and that gate, written out.

## Audience

```yaml
audience:
  assumed_knowledge: >
    They know their own problem and what it already costs them. They do not
    know how the product works internally and do not need to — mechanism is
    staged here only as deep as belief in the outcome requires.
  what_they_distrust:
    - a strong claim with no price attached anywhere in the artifact
    - jargon introduced and never explained, on a slot they will forward
    - a mechanism explained at a length that reads as padding around a price
      the deck is reluctant to state
    - a number that cannot be traced back to where it came from
    - a mechanism block that runs long enough to feel like the ask is being
      delayed
  decision_they_hold: >
    Usually a purchase, or a recommendation to buy. Name which — a sole buyer
    decides today; a buying-group member recommends upward and the artifact
    must survive being forwarded without you in the room.
  attention_budget: >
    They decide, inside the problem beat, whether the rest is worth their
    attention. If the price never lands anywhere in the artifact, it is read
    as evasive rather than expensive — the single most common failure this
    genre exists to close.
```

## Selection

```yaml
pick_when:
  - a specific, priced product is on offer to a buyer who can say yes
  - the room's overriding question is "what does this cost, and is it worth it"
  - the purchase is for one product, not a market thesis or a company
skip_when:
  - capital is being raised from funders, not a purchase → investor-pitch
  - several stakeholders must independently sign off and the deck will be
    forwarded objection by objection → sales-proposal
  - no price is being asked for yet, only understanding → executive-explainer
  - a named person must decide something today with no purchase involved →
    internal-decision
instead_use:
  sales-proposal: >
    a multi-stakeholder buying group, table lane, objections from absent
    members pre-answered — this genre is the one-buyer, hall-leaning case
  investor-pitch: the ask is capital, not a purchase
  executive-explainer: the audience holds only a credibility judgment, no wallet
```

## Arc — 6 beats

```yaml
arc:
  - beat: hook
    intent: the one arguable thesis about what the status quo already costs
    minutes_share: 0.08
  - beat: problem
    intent: >
      the buyer's specific cost today, quantified and named, and what it
      keeps costing if nothing changes — ONE dense beat, not a tour
    minutes_share: 0.32
  - beat: mechanism
    intent: how the product removes the cost, only as deep as belief requires
    minutes_share: 0.25
  - beat: proof
    intent: a concrete instance the buyer can check, not a claim about the product
    minutes_share: 0.12
  - beat: cost
    intent: the price, stated once, plainly, tied to the number just earned
    minutes_share: 0.13
  - beat: ask
    intent: what you want, from whom, by when
    minutes_share: 0.10
```

Beat notes:

- **`problem` gets ONE dense beat, not several thin ones.** The bake-off's B3
  reading is the failure mode this caps: a forwarded reader who hits five
  problem slots before the product decides in the first minute that this is
  advocacy, not a pitch. One well-evidenced beat outperforms five — see
  `problem_share` below.
- **`mechanism` opens EARLY relative to slot count, not late.** The four
  measured mechanism positions from the bake-off — 31% (slot 4 of 13, graded
  pass, best in set), 43%, 46%, 57% (graded a clean B3 fail) — are the entire
  evidence base for the `mechanism_by` gate this playbook authors below. This
  arc's own slot sequence reproduces the winning number exactly, not as a
  coincidence: it is the worked example the gate's ceiling was derived from.
- **`proof` is an instance, never a slogan.** The substitution this beat makes
  — "described" into "believed" on contact with a checkable instance — is the
  same one `technical-deep-dive` uses for a skeptic; here the instance is
  aimed at a buyer instead of a peer.
- **`cost` is mandatory and sits with the ask.** See `cost_slot` below and
  `slide-functions.md`'s `cost` entry. Its figure is the one figure this
  entire genre exists to make sure the deck states.

## Slot sequence

```yaml
slide_sequence:
  - {slot: 1,  function: cover,      beat: hook,      required: true}   # the arguable thesis
  - {slot: 2,  function: fact,       beat: problem,   required: true}   # the quantified cost today
  - {slot: 3,  function: claim,      beat: problem,   required: true}   # what it keeps costing if nothing changes
  - {slot: 4,  function: mechanism,  beat: mechanism, required: true}   # ← FIRST mechanism slot: 4 of 13 = 30.8%
  - {slot: 5,  function: mechanism,  beat: mechanism, required: true}
  - {slot: 6,  function: mechanism,  beat: mechanism, required: false}
  - {slot: 7,  function: comparison, beat: mechanism, required: true}   # today vs with-product, on THEIR axes
  - {slot: 8,  function: statement,  beat: proof,     required: true}   # ← the bold moment sits here
  - {slot: 9,  function: objection,  beat: proof,     required: true}   # the killer objection: "why not build/keep doing this ourselves"
  - {slot: 10, function: objection,  beat: cost,      required: false}  # the price objection, answered before it is asked
  - {slot: 11, function: cost,       beat: cost,      required: true}   # the price, stated once
  - {slot: 12, function: ask,        beat: ask,        required: true}
  - {slot: 13, function: end,        beat: ask,        required: true}
  - {slot: A1, function: appendix,   beat: null,       required: false} # table-leaning mixed only
```

Sequencing constraints specific to this genre:

- Slots 4–7 are the mechanism run; nothing may be inserted between them, and
  a `mechanism_by` gate (below) bounds where slot 4 may fall in the first
  place.
- Slot 7 is a single comparison slot on the buyer's own axes — a comparison
  split across two slots is not a comparison (`slide-functions.md`).
- Slot 11 (`cost`) must sit immediately before the ask pair (12–13), never
  inside the mechanism run and never separated from the ask by unrelated
  material — the `cost` entry in `slide-functions.md` states this rule once
  for every genre that uses the function; this genre is the one that makes
  the slot REQUIRED rather than optional.
- At most one `cost` slot, at most one `pivot`-equivalent turn (this genre
  does not carry a `pivot` slot; the turn from problem to mechanism is
  carried by the transition line instead, because the arc is short enough
  that a dedicated pivot slot would be a fourth beat competing with `mechanism`
  for the same attention).

## Bold moment

```yaml
bold_moment:
  beat: proof
  form: >
    A concrete result that already happened to someone else who bought this —
    a live number from their own dashboard, a reference customer's own close
    time, a demo the buyer can independently check. A thing that happened,
    never a strong sentence and never a number about the product's design.
  why: >
    A buyer converts "described" into "believed" only on contact with an
    instance, exactly as a technical skeptic does — this genre aims the same
    substitution at a different distrust (not "is the mechanism real" but "did
    this actually work for someone whose money was also real").
  placement: >
    AFTER the mechanism run and its comparison, and BEFORE the cost slot.
    Two failure modes bound it on either side: before mechanism it is
    unmotivated (the audience has nothing to evaluate it against yet); after
    cost it reads as compensating for the price just stated, which raises
    the same suspicion the instance was meant to remove.
  placement_deck: >
    Slot 8 of 13 = 61.5%. The window is bounded by the beats it sits between:
    it may not land before the mechanism run closes (slot 7 ends at 7/13 =
    53.8%) and may not land after the cost beat opens (slot 10 begins at
    10/13 = 76.9%) — so 54–77% by index is the legal window, and slot 8 (or
    slot 9, the killer objection, at 69.2%) both sit inside it.
  placement_video: >
    Initial plan: 60–75% by time, derived from this lane's runtime shares.
    Record cumulative time for the chosen arc rather than borrowing deck indices.
    Revise the placement explicitly after rough-cut review when that improves
    understanding; retain the agreed runtime, evidence and consequential limits.
  count: 1
```

## Time shape

```yaml
time_shape:
  deck:
    minutes: {min: 5, ideal: 8, max: 12}
    seconds_per_slot_floor: 35      # mixed lane: denser than hall's 40s, sparser than table's 30s
    slides: {min: 10, ideal: 13, max: 16}
    presented_vs_read: presented-primary, read-fallback (it WILL be forwarded)
    minutes_per_beat_at_8min:
      hook: 0.64
      problem: 2.56
      mechanism: 2.0
      proof: 0.96
      cost: 1.04
      ask: 0.8
    arithmetic: >
      8 min × 60 = 480s ÷ 13 content slots = ~37s/slot, above the 35s floor.
      At 16 slots the same 8 minutes gives 30s/slot — below floor. Cut slots,
      starting with the optional slot 6 and the optional slot 10, in that
      order — they are marked `required: false` for exactly this reason.
    appendix: excluded from the arithmetic; table-leaning mixed only
  video:
    seconds: {min: 75, ideal: 100, max: 130}
    cold_open: 10-20s, before any title — open on the cost of the status quo, not the product
    runtime_shares: {problem: 0.30, mechanism: 0.30, proof: 0.15, cost: 0.15, ask: 0.10}
    word_budget: ~150 wpm; the hook and the ask get no dedicated bucket, same device
      `technical-deep-dive` and `executive-explainer` both use for their thinnest beats —
      cut BEATS to fit, never compress every beat equally
    devices:
      - open on what the status quo costs, not on the product — a cold open
        that leads with the product before the room has felt the cost reads
        as an advert from the first second
      - the price beat survives compression before anything else does; cutting
        it to save time is the exact defect this genre exists to prevent
      - progressive reveal in mechanism order — nothing on screen before the
        narration reaches it
      - end on the ask, not a summary and not a logo card
    cover_becomes: >
      No `cover` scene in the film. It dissolves into the cold open (the
      status-quo cost, 10-20s, before any title) plus a title landing at
      scene 2 or 3.
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
  name: concede the cost, not the vision
  do: >
    Slot 1 states, as its claim, what the status quo already costs the buyer
    — not the product's mission, not a category trend. The product is not
    named as the hero of slot 1; the cost is.
  avoids: >
    The vision-first open, which is the specific failure the bake-off found
    six times over: a deck that opens on ambition and never lands a price
    reads as marketing, and this audience's first job is deciding whether
    marketing is all it is about to get.

closing_move:
  name: the price, then the smallest yes
  do: >
    State the price once more if useful, then ask for one specific, dated
    next step — a pilot start date, a signature, a follow-up meeting with a
    named attendee. Not three asks; the smallest one that is real.
  avoids: >
    The vanish close (a thank-you slide, a logo, "questions?") which spends
    the last high-attention moment on nothing, and the bundled ask (three
    things wrapped as one) which this audience reads as trying to slip
    something past them right after the price has landed.
```

## Transitions

```yaml
transitions:
  - {after_beat: hook,       type: escalate, note: "from the general cost to what it is costing THIS buyer"}
  - {after_beat: problem,    type: narrow,   note: "from the cost to the one mechanism that removes it"}
  - {after_beat: mechanism,  type: fold,     note: "the parts become one property — sets up the proof"}
  - {after_beat: proof,      type: escalate, note: "now the price, in the terms the proof just earned"}
  - {after_beat: cost,       type: gate,     note: "the one boundary where the artifact asks"}
```

## Kill conditions

```yaml
kill_conditions:
  - condition: no honest price can be stated today
    do: >
      stop — this is not a pitch yet, it is a briefing. Re-CLASSIFY as
      executive-explainer until a price exists.
  - condition: the mechanism cannot be shown doing anything a buyer can check
    do: >
      there is no bold moment available. Find the instance, or accept this is
      a promise rather than a pitch and stage it as one.
  - condition: several stakeholders must independently be convinced and the
      deck will be forwarded objection by objection
    do: re-CLASSIFY as sales-proposal
  - condition: the ask is capital, not a purchase
    do: re-CLASSIFY as investor-pitch
  - condition: the mechanism needs more than three slots to be believed
    do: >
      either the audience is more technical than classified (→
      technical-deep-dive) or the mechanism is being explained at a depth
      pride wants and belief does not. Cut to what belief requires.
```

## Anti-patterns

```yaml
anti_patterns:
  - symptom: the deck ends and nobody in the room can say what it costs
    cause: no REQUIRED cost slot, or the number lived only in a leave-behind
    fix: >
      `cost` is a required slot in this genre's slide sequence, its figure a
      `ship`/`ship_with_as_of` row in the `number_gate` — ship it on-screen,
      not folded into an appendix nobody opens live.
  - symptom: a customer-facing slot reads dense with unexplained shorthand
    cause: no glossary discipline; internal jargon leaked onto a forwarded slot
    fix: >
      every novel term gets a `glossary` row before it ships; a
      `keep_gloss_once` term carries its gloss at first use, in the same slot.
  - symptom: the mechanism runs long and the price arrives cold at the end
    cause: >
      mechanism over-slotted past what belief requires — the `mechanism_by`
      ceiling is missed by construction, not by a stray number
    fix: cut mechanism to what the 35% ceiling allows; route detail to appendix
  - symptom: five problem slots before the product ever appears
    cause: problem_share drifted past its cap chasing more proof of the pain
    fix: >
      enforce the 30–45% cap; one dense, well-evidenced problem beat
      outperforms five thin ones and is what the cap exists to protect
  - symptom: a number on screen cannot be re-derived from what is on screen
    cause: the figure existed in source material but never got a `number_gate` row
    fix: every figure in a `claim`/`supporting` string needs a row; withhold
      rather than guess
```

## Gates

```yaml
gates:
  - id: helicopter
    check: the headline claims alone, in order, form the complete argument
  - id: mechanism_by
    check: >
      the first `mechanism`-function slot lands at ≤35% of content slots
      (appendix excluded). Worked example carried by this playbook's own
      slot sequence: slot 4 of 13 = 30.8%. AUTHORED from four measured
      mechanism positions in the bake-off (31% pass-best-in-set, 43%, 46%,
      57% clean fail) — not inherited from a source.
  - id: problem_share
    check: >
      the `problem` beat holds ≥30% and ≤45% of the time budget. The FLOOR
      is inherited from `executive-explainer`'s `problem-share` gate; the
      CEILING is new, authored here from the same four measurements, to stop
      a "well-evidenced" problem block from creeping into the five-slots-
      before-the-product failure the bake-off's B3 rubric caught.
  - id: cost_slot
    check: >
      exactly one `cost`-function slot is present, sits adjacent to the
      `ask`, and its figure is a `ship` or `ship_with_as_of` row in the
      `number_gate` — never `withhold`. A withheld price is not shippable
      as this genre's cost slot; the pitch waits for the ruling instead.
  - id: bold-after-mechanism-before-cost
    check: >
      DECK: after mechanism and before cost, 54–77% by index. VIDEO: proof is
      understandable in context and cost remains consequential; verify the
      recorded arc and cumulative timing, not a fixed percentage.
  - id: motion-budget          # VIDEO lane only
    check: >
      the motion_budget review in references/motion-roles.md: semantic purpose,
      contrast, emphasis, repetition, rhythm and attention; no numerical taste gate
  - id: ornament               # VIDEO lane only
    check: figures have useful jobs and readable hierarchy; record actual findings
  - id: no-unglossed-term
    check: >
      every novel term in a `claim`/`supporting` string has a `glossary`
      row; a `keep_gloss_once` term carries its gloss at first occurrence
  - id: dead-slot
    check: every slot turns a value; no recap slot, no agenda slot
  - id: time-shape
    check: minutes × 60 ÷ content_slots ≥ 35s; show the arithmetic
  - id: standalone
    check: every headline claim is a complete sentence (it WILL be forwarded)
```
