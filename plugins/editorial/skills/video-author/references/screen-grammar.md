# Visual explanation, diagrams and code-on-screen grammar

No donor skill ships this, and it does more work in a technical explainer
than every chart rule combined: the typical subject is *record shapes* — a
JSON fact, a YAML warrant, a config block, a table, a chain, a cell diagram
— not plottable data. Distilled from the bake-off runs (the control run
invented it under fire; the staged and hybrid runs confirmed the floors).

## Choose an appealing visual treatment

The goal is a film worth watching that makes the subject easier to understand.
Choose the treatment from the explanation and audience, not from the easiest
diagram syntax. Use [creative development](../../staging/references/creative-development.md) to
compare treatments and test shots before polish. The renderer safeguards govern implementation;
visual richness does not require more moving objects.

- **Carry a recognizable subject.** Follow the same change, deposit, evidence
  artifact or decision across scenes. Preserve its visual identity while its
  state and relationships change; avoid rebuilding a new diagram for every noun.
- **Make flows legible and attractive.** Design clear entry, path, branch and
  outcome; give the main path hierarchy, use intentional spacing and connector
  routing, and label the consequential relationships. Reveal or trace an actual
  journey as the voice explains it. A graph's topology alone is not its design.
- **Choose visuals with a job.** An artifact close-up can establish evidence;
  a before/after can expose a changed state; a payoff curve can explain a tradeoff;
  a composed illustration can orient an unfamiliar subject. Metaphors need an
  explicit mapping and must give way to the real mechanism where that matters.
- **Compose the frame.** Use deliberate typography, scale, contrast, alignment
  and whitespace to establish a focal point and reading order. Apply the brand
  without turning every scene into the same three-card layout. Vary treatment
  where the explanatory job changes, while retaining a coherent visual language.
- **Use Mermaid as a draft when useful.** It can establish topology quickly.
  For a visually led explainer, a default Mermaid export or equivalent block
  chart is a starting diagram, not a finished visual treatment. Design the final
  hierarchy, paths, labels and emphasis in the supported HTML/SVG or renderer.
  This is a quality judgment, not a ban on Mermaid or a demand to animate a
  diagram that is clearer as a well-designed still.
- **Inspect a still and the sequence.** The still should communicate a deliberate,
  readable composition; playback should explain the relationship in the right
  order and allow inspection. Adding motion to prose cards does not meet this
  standard by itself. Record scene-specific findings in `editorial-review.md`.

One explanatory figure may contain several related parts. Showing a causal
relationship also described in speech is useful reinforcement, not redundant
decoration. Judge its animation by attention and legibility, not a fixed figure/energy cap;
do not hide unrelated competing heroes inside one nominal group. A pipeline
scene may contain several shots. A close-up followed by a wider reveal can
change what the viewer understands without adding a new narration row. Preserve
an eye-trace anchor where useful; use perspective or typography as an explanatory
device when their mapping is clear.

## Size floors (1920×1080 canvas)

| Element | Floor |
|---|---|
| Monospace record/code text | **≥23 px** — 25 px overflows an 11-line card; below 23 px is unreadable at video compression |
| Body/caption sans | ≥28 px |
| On-screen headline text | large enough to read at a glance; it is 2–5 words, so it can be |
| Diagram labels | ≥22 px, `nowrap` — a label that wraps mid-word (`irreversible-tier`) reads as two labels |

## Line and content budgets

- Start with one primary record. A readable side-by-side comparison can be
  stronger than alternating records; make the consequential difference easy to find.
- **≤11 lines** of mono per record. A record that needs more is quoted
  partially (elide with a comment line), or split across a before/after
  pair of scenes — never shrunk below the size floor.
- A literal record IS a legitimate scene — the audience that wants the
  mechanism wants the record shape — but it is *read*, so its scene needs
  reading time: budget narration that walks it, and usually reveal it as a block. Progressive typing is useful when the
  edit or entered sequence is the subject; allow reading time and label simulated
  activity so it cannot be mistaken for an actual product recording.
- Long command lines: author the continuation explicitly (`\` + an
  indented continuation style). A soft-wrapped command renders flush-left
  and reads as a *second command* — every automated check passes on it;
  only a looked-at frame catches it.

## Syntax colouring inside the brand rule

The accent rule (the brand contract's accent only, no new hues) governs
*emphasis*, not functional colour. The working grammar:

- keys / field names → the light structural accent (e.g. `--blue-lt`)
- comments → muted ink (`--muted`)
- values → primary/secondary ink (`--ink` / `--sec`)
- functional states keep the guide's functional palette: danger red
  **only** on failing verdicts/refusals (1–2 % of surface), never for
  emphasis or decoration. When the argument turns on a PASS/FAIL
  distinction, rendering both in accent blue destroys the claim.
- Any *semantic* hue beyond that (a second actor, a query lane) is a brand
  ruling, not an authoring choice — flag it PENDING with the fallback
  stated (collapse to accent + a caption).

## Build in narration order

- Every progressive reveal follows the sentence being spoken — members of
  a group arrive in a sequence the viewer can follow. Match semantic cues to
  the actual voice; neither fixed reading direction nor fixed stagger is universal.
- A reveal that the narration does not walk is revealed as one block.
- Structural diagrams draw connectors once, then allow inspection; avoid
  decorative marching ants, travelling dots or pulses. For an actual runtime
  journey, a moving subject or highlighted path may show transfer, branching or
  refusal when that event is the claim. Name what moves and what it means;
  do not imply data flow or execution in a purely structural relationship.
- Show numbers directly unless change itself is the explanation. A conceptual
  count-up must not masquerade as measured intermediate data. Any changing value
  must seek deterministically; GSAP `onUpdate` alone does not run on a seek.
- Tables arrive one row per narration clause (delivery, not evidence —
  budget its energy accordingly); a comparison table's axis order is the
  narration's axis order.
- Before/after is shown **in place** where the claim is "overwritten /
  replaced": the old value leaves by its own row height into the slot the
  new value takes. A value that travels off somewhere else reads as
  *moved*, not replaced — and travels off-frame where no check but a
  per-element measurement will see it.

## The two-audience register

- On-screen text: 2–5 words, plain verbs, one job per element — what the
  viewer reads while the voice says the sentence. Never the transcript.
- Subtitles: the full narration (omit the `subtitle` column; the pipeline
  reuses narration). This is the accessible answer and the sanctioned one.
- A deep-dive act may put denser records on screen than an explainer act,
  but preserve enough time and hierarchy to read it. Review density and
  movement together at the intended viewing size.
