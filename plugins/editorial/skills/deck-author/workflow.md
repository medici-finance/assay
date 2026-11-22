# Deck author

## The composition contract (hard, and it binds THIS skill)

```
staging  owns WHAT is said and IN WHAT ORDER    → emits staging.yaml
design   owns HOW IT LOOKS                       → consumes staging.yaml
```

What this skill may never do:

1. **Never reorder slots and never rewrite a headline claim.** The slot
   sequence and every claim in `staging.yaml` are the argument, already gated.
   Legal: splitting one slot across two surfaces; rewording a claim's
   *phrasing* for line-breaking (same insight, same force); typographic
   emphasis inside a claim.
2. **A slot you cannot render is refused, not fixed.** Refusal returns to
   staging with a reason ("this claim carries three ideas; no slide holds
   them"). A design pass that changed the argument's order or content has
   broken the contract — the resulting deck is invalid, however good it looks.
3. **No staging.yaml, no deck.** If the brief arrives without one, invoke the
   `staging` skill (or ask for its output). Improvising the arc inside the
   design pass is exactly the leak the contract exists to stop — and the
   bake-off's finding is that the model's improvised staging is wrong in
   *predictable* places (leads with its best material, over-slots, omits the
   cost, over-asks).

## Live talks start with a visual storyboard

Read the lane in `staging.yaml`. If it conflicts with the brief, return to
[staging](../staging/SKILL.md), CLASSIFY, Move 1, before designing the slides.
For a presenter-led talk, use the storyboard below.

Before drafting slide copy, sketch what the audience will see at each turn.
Carry a concrete subject through the talk when it fits: follow the request,
the object under test, or the decision. Pick a view that reveals what changes:
an actual screen detail, a route across a boundary, the same observation before
and after an intervention, or a measured comparison. A single open question
can earn a pause while the speaker sets up the observation that answers it.

Put the words needed to interpret the picture on screen, including any scope,
status, units or denominator essential to its truth. Give explanation and
source details to the speaker notes, and inventories and full results to an
untimed appendix. In the contact-sheet review, look across the timed sequence:
can you follow the example and point to what changed? Where successive slides
repeat a grid of prose, redraw the relationship or move that detail to the
reading material. Choose the treatment by what it teaches; a decorative image
or a quota of diagrams is a poor substitute for that judgment.

## Choose the stage and brand

Work from the project's template or its brand.toml. The [brand contract](references/brand-contract.md) defines the palette, fonts, marks and optional footer. For an unbranded deck, copy the [two-slide starting file](assets/deck.html) into the project and replace its argument. Its local fonts and landscape print layout work offline. Keep each slide inside a 1280 × 720 stage. Embed assets or use project-local files so the deck can be inspected offline.

### Render-layer rules on top of the skeleton

- **One accent, and it stays one.** The brand contract's `accent` is the only
  frequent colour; at most `accent_budget` (default two) visible accent uses
  per slide. Functional colours (red/green/amber) only functionally —
  `REFUSED`/fail, `pass` — never decoration. No gradient text; no shadows —
  depth comes from border contrast.
- **Anchor discipline.** Content slides top-aligned so kicker + headline land
  in the same place; centred composition reserved for the cover, the bold
  moment, and the close — so a change of alignment *means* something.
- **Density follows the lane.** A table-lane deck may run smaller type and up
  to six elements per slide; a hall-lane deck runs larger type and rarely more
  than four. Same skeleton, different rhythm. If content exceeds the mode,
  split the slide (a `tight` variant exists for one stubborn table) — never
  shrink below the floor.
- **Static slide markup, always.** Slides are literal `<section class="slide">`
  elements. A deck that builds its DOM from a script array is invisible to the
  machine gate (a static parser) and un-diffable in review. The bake-off's
  staged decks made this mistake; do not repeat it.
- **Headline claims render as the slide's `<h1>`/`<h2>`, verbatim in force.**
  The ghost-deck output of the linter must read as staging's helicopter test.

## Explanatory graphics — use them where they teach

Consider a graphic when the audience needs to understand a relationship,
mechanism or bottleneck. Choose a literal diagram, chart or screenshot when
precision matters; use an **explanatory visual metaphor** when a familiar scene
makes an unfamiliar mechanism easier to understand. For example, a conveyor
feeding a constrained review station can explain why more coding capacity grows
a queue. Map the objects and arrows to the actual process; do not imply measured
capacity, causation or guarantees the evidence does not support.

Use these selectively, not as a per-slide quota. One coherent metaphor at a key
explanation may be enough; keep evidence tables and precise comparisons literal.
If a graphic adds no understanding beyond its caption, simplify or omit it.

Record the chosen claim, object-to-concept mapping and any misleading implication
to avoid in the existing `deck.md` notes. This is a rendering decision, not permission
to change staging's argument. Keep essential explanation on the slide for read-alone decks.

Use editable vectors for precise relationships and labels. When an illustrated
metaphor or reference style calls for raster artwork, use the available image-generation
workflow, save the asset locally with its prompt, and keep critical text in the slide
layout where possible. Respect the resolved brand and the user's style; a sketchnote
reference is not a new universal house style. Give the image useful alt text.

## Motion on decks — the governor, not the mood

Default is **static**. Motion enters only when `staging.yaml` carries motion
fields (a restraint-off run) or the brief explicitly lifts restraint — and then
it is *governed*, never ambient. The following are DECK-SPECIFIC defaults
retained from the animated deck bake-off. They are owned here, not derived
from video's `references/motion-roles.md`; the video exploration guidance does
not impose or remove these slide-specific caps. The example runs motivate
them, rather than proving a universal quality threshold:

- **Peaks are derived, not chosen.** Walk the claims: only a claim containing
  an EVENT (a thing that happens, with a before and after) may be enacted.
  Count the events — that is the peak budget, and there is no other number to
  argue about. (The animated explainer found exactly two: an erasure, and the
  same erasure refused — one peak and its answer.)
- **Absences are decisions.** The slide after a peak takes a `hold` — no
  entrance, no build; the room needs a still frame to sit in. The limits/
  objection slide is *drained* — accent pulled, field monochrome — so the deck
  audibly stops selling at the moment it admits what is not built.
  "Animate everything" cannot produce either of these, by construction.
- **The ornament gate.** At most **two** animated figures per slide (the bold
  moment may spend three); every figure names the clause of the claim it
  renders; a figure with no clause is decoration and is cut. This gate exists
  because the intensity budget was obeyed on a real run and four
  micro-diagrams shipped anyway, two of them decoration.
- **Motion is not free in the time shape.** A build must be watched; re-run
  staging's time-shape arithmetic with per-slot dwell and record the new
  runtime as a re-gated judgment (the animated explainer bought one extra
  minute, deliberately). Never quietly extend the time.
- **Degradation and safety.** Under `prefers-reduced-motion` the deck degrades
  to its static twin — every final frame complete, argument-bearing, reachable
  with zero input. No build runs on a timer the presenter does not control;
  every step advances on the same key as the deck; stepping backward lands on
  a coherent frame. No spring, bounce, or overshoot (house motion register)
  unless a single deliberate violation is recorded in the notes.

## Check the deck

Set DECK_SKILL to this installed skill's directory and DECK_DIR to the presentation directory. Run the structural checker with Python 3.11 or later. It discovers a project brand.toml or accepts an explicit --brand path.

```sh
python3 "$DECK_SKILL/tools/check_deck.py" "$DECK_DIR/index.html"
node "$DECK_SKILL/../video-author/tools/render-check.mjs" --contact-sheet "$DECK_DIR"
node "$DECK_SKILL/../video-author/tools/render-check.mjs" "$DECK_DIR"
```

Inspect every slide in shots/contact.png. Check the narrowest label, the densest table, the line breaks in the headline, and any mark close to a slide edge at full size. Fix clipping by shortening the copy or splitting a slide. Apply the project’s publication checks to exported files. For checked PDF output, run the packaged renderer with the project’s page credit or classification. It checks content fit, the footer on every page, link destinations and document metadata before replacing the output.

```sh
node "$DECK_SKILL/tools/render-deck-pdf.cjs" "$DECK_DIR/index.html" "$DECK_DIR/slides.pdf" --footer "Example Studio — Public"
```

Install Puppeteer in your dependency directory and set EDITORIAL_RENDER_DEPS to it. The PDF checks require Poppler, qpdf, MuPDF (mutool) and ExifTool. Supply the project’s approved link destinations using --allow when the deck has external links.

## Review and handoff

Read the slide titles alone in order. If the argument disappears, revise the staged claims. Keep an action title and speaker notes per slide in deck.md, plus the cut list for a shortened talk. Check each source note and qualification at presentation size. If the checker reports a skipped brand rule or an unmeasured contrast pair, inspect it in the render before treating it as checked. Use a scoped .deckcheck.json exception only with a reason explaining that specific match.

Deliver the HTML, its referenced assets, deck.md and staging.yaml together. Record slide count, intended speaking time, the contact sheet, any fixes made after viewing it, and the checked PDF if requested.

## Credits

The structural checker retains the MIT notice from create-html-deck (tt-a1i). The presentation guidance also draws on open-design (Apache-2.0, with a credit to Refero Design), open-codesign (OpenCoworkAI Contributors), pitch-deck-mastery (Steve Kaplan), and frontend-slides (Zara Zhang). The staging skill carries its own method lineage and the HyperFrames seam checker's Apache-2.0 notice.
