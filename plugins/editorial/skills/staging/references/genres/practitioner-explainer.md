# Practitioner explainer

**Mandate:** help someone who operates a system understand a choice, see its
consequences, and take one useful next step. Suitable for a short educational
video accompanying an article, or a standalone operator introduction.

## Audience

```yaml
audience:
  assumed_knowledge: >
    They run the system day to day and know their own workflow, but not our
    tools, vocabulary or incident. They can follow one carried example through
    several controls; they cannot hold a catalogue of settings.
  what_they_distrust:
    - advice that ignores the constraints of their own setup
    - numbers with no denominator, sample or method
    - a product introduced before the problem it answers is established
    - a tutorial disguised as an explanation, or the reverse
  decision_they_hold: >
    What to try, measure or change next in their own practice. Not a purchase
    and not a budget.
  attention_budget: >
    2–4 minutes for the video; a viewer from search decides in the first
    15–25 seconds whether the film is for them.
```

## Pick and reject

Pick when the viewer knows their own work but may not know our tools, vocabulary
or incident. The decision is “what should I try or measure?” rather than “should
I buy this?” Reject `product-pitch` when no purchase is being asked for. Reject
`technical-deep-dive` when proving implementation details would crowd out the
operator's decision. Use the deep-dive when those details are the actual promise.

If the viewer must perform a task, reproduce UI actions or diagnose a new case,
use [teach-workshop](teach-workshop.md), with a worked example and learner practice.
If the explanation needs several independent mechanisms, narrow its promise or
make a series. A shorter film need not summarize every section of its article.

## Arc and time budget

Default to **orient → follow a case → expose choices → show bounded evidence →
choose a next step**. The case creates continuity through the controls; it may
be an explicitly illustrative workflow. Evidence can arrive earlier when that
makes the argument clearer. Record adaptations rather than forcing material
into a fixed number of scenes.

For a three-minute video, this is a starting allocation:

| Beat | Seconds | Job |
|---|---:|---|
| Orient | 20 | Name the subject, audience benefit and motivating situation. |
| Follow a case | 30 | Establish the whole journey and the relevant failure or tradeoff. |
| Expose choices | 45 | Explain the few controls that change this journey, including consequences. |
| Bounded evidence and method | 55 | Show one useful finding and its limits; explain what to record or check. |
| Next step | 30 | Apply the principle, give a feasible action and point to the deeper material. |

Beat ids used below: `orient`, `follow-a-case`, `expose-choices`,
`bounded-evidence`, `next-step`.

This totals 180 seconds; it is a drafting budget, not a timing contract. A
2–4-minute film can adapt it. At 140–150 words/minute, 180 seconds holds
420–450 words before explicit pauses. Recount the finished text, test a 10%
slower take, and state the resulting estimate. Never silently stretch a hard
runtime limit. Use as many scenes as the explanation needs, not “always eight.”

## Slot sequence

```yaml
slide_sequence:
  - {slot: 1, function: cover,      beat: orient,           required: true}   # video: dissolves into the cold open
  - {slot: 2, function: statement,  beat: orient,           required: true}   # subject, benefit, motivating situation
  - {slot: 3, function: fact,       beat: follow-a-case,    required: true}   # the carried example, end to end
  - {slot: 4, function: mechanism,  beat: expose-choices,   required: true}   # the few controls, in the order the case meets them
  - {slot: 5, function: comparison, beat: expose-choices,   required: false}  # one choice's consequences, on the operator's axes
  - {slot: 6, function: evidence,   beat: bounded-evidence, required: true}   # ← the bold moment: the one finding
  - {slot: 7, function: objection,  beat: bounded-evidence, required: true}   # its limits, volunteered, then answered
  - {slot: 8, function: ask,        beat: next-step,        required: true}   # one feasible action
  - {slot: 9, function: end,        beat: next-step,        required: true}   # points to the companion material
```

Sequencing constraints specific to this genre:

- **The case never leaves.** Every `mechanism` and `comparison` slot shows its
  control acting on the example introduced in slot 3.
- **No `pivot` is required.** The argument is cumulative, not a reversal. Add
  one only when the material genuinely turns ("what you assumed" → "what is
  true").
- **No `appendix` in the video.** Depth routes to the companion article.
- Slots 6 and 7 are a pair: the finding, then what it cannot show.

## Bold moment

```yaml
bold_moment:
  beat: bounded-evidence
  form: >
    The consequence landing on the carried example. For an observed case,
    show what actually happened (or a measured result about cases like it),
    with its denominator and limits. For an explicitly illustrative case,
    show the mechanism's outcome under stated assumptions, labeled illustrative;
    it is not evidence of measured effectiveness. Never invent a production
    incident to create drama. A mechanism-only film may use emphasis with
    zero rupture/arrest peaks, as the video-direction section allows.
  why: >
    It is the moment the controls the viewer just learned acquire
    consequences, and it is what the viewer carries into the next step.
  placement: >
    After the choices are explained (the viewer must know what the finding
    bears on) and before the next step; never in the orientation, where the
    motivating incident is context rather than payoff.
  placement_deck: >
    At the first `evidence` slot, roughly 55–70% through the SLOT SEQUENCE
    by index. Slot 6 of 9 = 67%.
  placement_video: >
    Initial plan from runtime_shares below: orient 0.11 + follow-a-case 0.17 +
    expose-choices 0.25 = 0.53, so the bold moment OPENS at ~53% of runtime
    and closes within the evidence share (0.53 + 0.30 = 0.83). At 180s: ~95s–150s. State
    the arithmetic for the film's own shares. A better rough-cut arc may revise
    the window; record the discovery, revised placement and runtime/evidence checks.
  motion_when_no_peak: >
    A narrative focal moment need not be a motion peak or the largest
    displacement. A quiet held discovery can carry it. Name what the viewer
    learns; do not fabricate an event to justify motion.
  count: 1
```

## Time shape

```yaml
time_shape:
  deck:
    minutes: {min: 5, ideal: 7, max: 10}
    seconds_per_slot_floor: 40
    slides: {min: 7, ideal: 9, max: 12}
    presented_vs_read: read-primary (it accompanies an article), presented-fallback
    arithmetic: >
      7 min × 60 = 420s ÷ 9 content slots ≈ 47s/slot, above the 40s floor.
      A 12-slot version of the same 7 minutes gives 35s/slot — below floor;
      cut slots, starting with the optional comparison.
    note: >
      The deck carries the same argument with standalone headline claims. Do
      not import the video's seconds-per-scene or motion requirements.
  video:
    seconds: {min: 120, ideal: 180, max: 240}
    cold_open: orientation lands in the first 15–25s
    runtime_shares: {orient: 0.11, follow-a-case: 0.17, expose-choices: 0.25, bounded-evidence: 0.30, next-step: 0.17}
    word_budget: >
      140–150 wpm. 180s ≈ 420–450 words before pauses; apportion words by the
      shares above (e.g. bounded-evidence 0.30 × 180s = 54s ≈ 126–135 words).
    peaks_for_this_genre: >
      Editorial choices, not an event-count quota. Distinguish reported
      events, conditions and illustrative transformations; earn emphasis by
      what the viewer discovers, and review contrast in the rough cut.
```

The shares are the seconds table above divided by 180 and rounded; a film
that adapts the table restates its own shares.

## Narrative choices

- Open with what the film will help the viewer understand or decide. Follow
  with the motivating incident once they know why it is relevant.
- Carry one example across the film. Introduce controls at the point where
  their consequences become visible, including failure, repair or recovery.
- Define unfamiliar terms immediately before they are needed. Introduce our
  product only when its role answers a question the film has established.
- Prefer one well-qualified finding to a procession of impressive numbers.
  State what was counted, what was excluded and what the evidence cannot show.
  Starting settings are hypotheses unless comparative evidence establishes more.
- End with a useful action the viewer can do: capture a complete case, compare
  alternatives under fixed criteria, or inspect a named setting. The companion
  article carries extensive defaults, methodology and additional evidence.

## Video direction at handoff

Apply the shared motion roles and motion spec. Reveal a relationship when its
narration explains it; preserve enough time to read it. Three clauses can reveal
three parts of one explanatory figure without creating three competing heroes.
Record the figure's relationship and the clauses it serves.

Peaks can mark a discovery, comparison or supported event. Do not invent an
incident to justify movement. The initial focal-moment window comes from this
playbook's runtime shares; a better rough-cut arc can revise it explicitly.
Stationary match cuts can preserve a meaningful carrier; deliberate hard cuts
can change viewpoint. Declare the actual seam type and run its applicable
checks instead of adding travel for its own sake.

The deck lane uses the same argument with its own time shape (above) and
standalone headline claims.

## Opening and closing moves

```yaml
opening_move:
  name: promise first, then the motivating situation
  do: >
    State what the film helps the viewer understand or decide, then the
    situation that makes it matter to them.
  avoids: >
    The unexplained cold incident (a viewer from search cannot tell what the
    film is about) and the credentials or logo open, which spends the
    highest-attention seconds on us.
closing_move:
  name: one feasible action, then the deeper material
  do: >
    Name one thing the viewer can do in their own setup, then point to the
    companion article for defaults, method and further evidence.
  avoids: >
    The "learn more" close when a useful action is available, and a recap
    that merely repeats. A synthesis that makes the next decision possible earns
    its place.
```

## Transitions

```yaml
transitions:
  - {after_beat: orient,           type: narrow,   note: "from the general situation to one case"}
  - {after_beat: follow-a-case,    type: turn,     note: "the case meets a choice point"}
  - {after_beat: expose-choices,   type: escalate, note: "the choices have consequences"}
  - {after_beat: bounded-evidence, type: gate,     note: "what to try, given what the evidence can and cannot show"}
```

## Kill conditions

```yaml
kill_conditions:
  - condition: the viewer must reproduce exact UI steps
    do: route to a walkthrough/tutorial (teach-workshop), not this genre
  - condition: there is no concrete case to carry through the film
    do: >
      Find or construct an explicitly illustrative one. Without it the arc
      is a list of tips; do not stage it.
  - condition: the explanation needs several independent mechanisms
    do: narrow the promise to one, or plan a series
  - condition: a purchase is being asked for
    do: re-CLASSIFY as product-pitch
  - condition: proving implementation details IS the promise
    do: re-CLASSIFY as technical-deep-dive
  - condition: there is no feasible next action for this audience
    do: >
      The film has nowhere to land; write a note or article instead, or wait
      until the action exists.
```

## Anti-patterns

```yaml
anti_patterns:
  - symptom: the film compresses the article's headings into scenes
    cause: authored from the article outline, not the viewer's question
    fix: one takeaway; move the rest to the companion article
  - symptom: controls are explained with no case in sight
    cause: the carried example was dropped after the opening
    fix: show each control acting on the slot-3 case
  - symptom: several impressive numbers, none remembered
    cause: evidence treated as decoration
    fix: one qualified finding with its denominator and limits
  - symptom: the product appears before the viewer knows why it matters
    cause: product-first ordering
    fix: introduce it where its role answers an established question
  - symptom: the ending says "learn more"
    cause: no concrete next action identified at staging
    fix: name one feasible action the viewer can take in their own setup
```

## Gates

Run [the shared editorial review](../video-editorial-review.md), plus staging's
five content gates and, for video, its production gates. The bold-moment gate
reads the deck placement window or the recorded video arc after rough-cut review; the kill-conditions gate reads the list
above; the video time gate apportions words by `runtime_shares`. In
particular, also verify:

```yaml
gates:
  - id: one-case
    check: the slot-3 example is visible in every choices and evidence slot
  - id: bold-in-window
    check: >
      the bold moment sits in the evidence beat. DECK: ~55-70% by index.
      VIDEO initial plan: after orient + case + choices and before next-step;
      show the arithmetic or record the reviewed rough-cut revision, its
      discovery and the preserved runtime/evidence obligations
  - id: evidence-limits
    check: >
      an observed finding states what was counted, what was excluded and what
      it cannot show; an illustrative outcome states its assumptions and
      limits, is labeled illustrative, and claims no measured effectiveness
  - id: feasible-action
    check: the ask is one action this audience can take in their own setup
  - id: word-budget
    check: recounted words per beat ≤ share × seconds × wpm ÷ 60, with a 10% slower take stated
```

An attractive storyboard cannot pass those checks by itself.

## Worked revision



The reusable choice is the progression. The subject, palette, scene count,
numerical defaults and 53% case finding are specific to that film.
