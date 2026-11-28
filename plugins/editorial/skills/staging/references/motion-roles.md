# Motion roles — semantic intent for video

Read alongside staging's slot functions. These eight pipeline identifiers remain
stable for existing artifacts; they are not viewer-facing labels (see
`vocabulary-map.md`). They describe what a scene does over time, not its look.
Concrete effects, palettes and layouts belong in the design review, not staging.

## Fields retained for compatibility

| Field | Meaning |
|---|---|
| `motion_role` | One of the eight semantic roles below. |
| `energy` | Relative 0–5 planning estimate within this film; not a calibrated score, motion allowance or pass threshold. |
| `peak` | An intended moment of heightened emphasis or discovery. |
| `motion_turn` | What changes in the viewer's understanding, or why inspection/continuity needs time. |
| `figures` | Named visual subjects/groups and their explanatory or attention-guiding purpose. Refine during design. |

| Role | Semantic intent |
|---|---|
| `hold` | Give the viewer time to read, compare or think. |
| `accumulate` | Make growth or repeated instances understandable. |
| `strain` | Show a supported relationship under pressure. |
| `rupture` | Reveal a consequential break or change. |
| `arrest` | Show an attempted action stopped or refused. |
| `assemble` | Bring parts into a comprehensible relationship. |
| `divest` | Remove what no longer helps; expose a limit or essential structure. |
| `cadence` | Resolve or return to an image with new understanding. |

A role does not prescribe amplitude, effect or figure count. A quiet reveal can
be the narrative climax; a busy assembly can remain one readable subject.

## Review principles (R1–R6)

**R1 — Earn attention.** Name the explanatory, orienting or emotional job of a
major move. Motion may guide attention and expectation without literally acting
out a clause. A move that adds neither understanding nor useful experience is a
candidate to remove, not a reason to manufacture a claim.

**R2 — Create contrast.** Review pace, scale and stillness across the sequence.
A peak needs perceptible context, not a mandatory quiet neighboring scene or an
energy-number test. Sustained action and several nearby discoveries can work.

**R3 — Preserve the argument's emphasis.** Identify the narrative focal moment
and why it matters. It need not have the largest displacement or the film's only
strong image. Other shots may be visually striking without competing with it.

**R4 — Use repetition deliberately.** A repeated gesture can establish continuity,
comparison, expectation, rhythm or a pattern that a later shot breaks. State its
job; reject repetition that merely fills time.

**R5 — Test rhythm in playback.** There is no required mean energy or fraction of
quiet scenes. Use the rough cut and actual take to find overload, dead time and
missing inspection time. Energy values describe the plan, not audience response.

**R6 — Make relationships readable.** No fixed figure cap. Several related parts
can clarify a mechanism; two unrelated heroes can overwhelm it. Explain each
figure's job, then inspect hierarchy and eye movement. Record real cuts and
revisions rather than inventing decoration to satisfy an audit.

## Truth remains a constraint

Distinguish an actual event from a condition, illustration or proposed behavior.
Do not animate a break, refusal or measured change as if it happened when the
source establishes only a condition. A labelled conceptual transformation may
explain a contrast, but its metaphor must not imply a product guarantee, measured
magnitude or causal event absent from the evidence. Check likely mistaken inferences.
Peaks are editorial choices justified by the story, not the count of EVENT claims.

## Review record

Retain existing `motion_budget` keys so artifacts remain readable. They summarize
the design and review, with no global numerical taste thresholds:

```yaml
motion_budget:
  peaks_earned: N          # intended discoveries and why they matter
  ceiling_order: "relative emphasis; narrative focal moment identified"
  quiet_scenes: []         # actual inspection/rest points, if any
  mean_energy: N.NN        # descriptive only, not a quality score
  adjacency: "contrast reviewed; findings or pending playback"
  rhymes: []               # repeated gestures and their purpose
  figures_total: N
  ornament_gate: "attention reviewed; actual findings or pending design"
```

At staging, mark unavailable playback checks pending. Update affected intent in
staging when visual exploration improves the story; never change claims/order
silently in the timeline. Use [creative development](creative-development.md) and
[motion language](motion-spec.md) for design choices and review.
