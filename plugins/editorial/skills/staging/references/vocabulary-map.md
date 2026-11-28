# Vocabulary map — contract tokens that must never reach the screen

The structural deck checker reads the first column of this table. Use the audience-facing meaning on screen and keep the production term in the storyboard.

## Machine-readable form

This is a Markdown table whose **first column is the banned token, verbatim,
one per row** — a linter greps column 1 case-insensitively against slide
text and on-screen copy. Do not reformat rows across multiple lines, do not
combine two tokens in one cell, and keep the token text exact (including
case where the contract enforces `staging.yaml`'s own casing, e.g.
`motion_role`, not `Motion Role`).

| Banned token | Where it comes from | Replacement shape (a thing that happened, never a category name) |
|---|---|---|
| `hold` | `motion_role` value (`../references/motion-roles.md`) | Describe what is being held still — the specific claim or number the frame gives without developing it. Never print the word "hold." |
| `accumulate` | `motion_role` value | Describe what is growing and by how much — "three years of premiums, one bar at a time," never the label "accumulate." |
| `strain` | `motion_role` value | Name the load and what is under it — "the seal holds at 40 psi... just," never "strain." |
| `rupture` | `motion_role` value | Name the break itself as an event — the winning track's pattern: `RUPTURE` → **"THE BREACH."** A rupture scene's on-screen text names WHAT broke, not that a rupture occurred. |
| `arrest` | `motion_role` value | Name the refusal — "the transfer stops here," never "arrest." |
| `assemble` | `motion_role` value | Name what is being built — "the four filings become one record," never "assemble." |
| `divest` | `motion_role` value | Name what is being given up — "the pitch drops its own diagram," never "divest." |
| `cadence` | `motion_role` value | Name the closing image itself, never the word "cadence." |
| `figures[].name` | `staging.yaml` per-scene field (`SKILL.md` line ~299) | The figure's `name` is an authoring handle for design conversation ("the vault-door diagram"); on-screen text names the CLAIM the figure renders, never the figure's internal name or its field path. |
| `function` | `staging.yaml` slot field, closed vocabulary in `references/slide-functions.md` (`cover`, `mechanism`, `proof`, …) | Never print the function name as a rail label or section header — e.g. a `mechanism` slot's on-screen text describes the mechanism, not the word "mechanism." |
| `beat` | `staging.yaml` arc field (`arc: [{beat: …}]`) | Beats are named in house words for internal sequencing; the on-screen rail may show a scene's CLAIM in short form, never the literal `beat` name or the word "beat." |
| `seed` | `staging.yaml` / design-handoff field naming the concrete image or motif a figure starts from | The seed is a design INPUT (what to draw from); on-screen and narration describe the resulting image or claim, never the word "seed" or the seed's field value verbatim. |
| `BOLD MOMENT` | `staging.yaml` `bold_moment` block / prose shorthand | Name what the bold moment actually shows — the single loudest claim's own words, in whatever design language the brand supports (a stamp, a color shift, a scale change). Never print the phrase "BOLD MOMENT" or "bold moment" on a slide or in a rail label. |
| `COVER · COMMON-GROUND` (and any `FUNCTION · FUNCTION` rail label built from two `function` values) | Composite of two `function` values, printed as a rail label | Rail labels describe what the audience is being shown ("the shared premise"), never the underlying slot-function pair joined with the pipeline's own delimiter. |
| any other `staging.yaml` key name (`motion_turn`, `energy`, `peak`, `runtime_shares`, `gates:`, `motion_budget`, …) | `staging.yaml` schema, generally | The general rule this table is an instance of: a `staging.yaml` **key name** is a pipeline-internal identifier, never viewer language. If a new field is added to the schema and its name could plausibly leak onto a slide (a short, quotable identifier), add it as its own row above rather than relying on this catch-all. |

## Why a "thing that happened," not a category name

A category name (`RUPTURE`, `hold`) describes the SHAPE of the motion, which
is exactly the information the viewer does not need and the internal
process does. A replacement in the shape of "a thing that happened" carries
the CONTENT instead — the winning track's `THE BREACH` names what broke,
not that breaking (as a category) occurred. This is the same test
`../SKILL.md`'s composition contract already applies to `motion_role`
generally (see `motion-roles.md`, "The line that is still held"): a role
names what the argument does in time, never what the frame looks like — and
neither may appear as the frame's own text.
