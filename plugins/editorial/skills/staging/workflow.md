# staging

Staging is the layer between "here is the material" and "here is the deck."
It owns **what is said and in what order**. It is medium-aware — every genre
playbook carries a **deck lane** and a **video lane** over one shared arc.

## How this skill is invoked

Run staging **first**, before any authoring or design skill, whenever a deck,
talk, tutorial, or explainer video is requested and no ruled `staging.yaml` exists yet.
Its main deliverable is `staging.yaml`, written beside where the artifact will
be built; video also carries the editorial review described below; the downstream skill (`deck-author` for slides, `video-author` for
film) consumes that contract and owns everything visual. If the brief already
comes with a reviewed `staging.yaml`, skip this skill and go straight to the
authoring skill. All file references in this skill (`references/…`, `tools/…`)
are relative to the skill's own directory, wherever it is installed; paths to
the material being staged come from the brief, never from this skill.

## Video editorial gate

For a new film or a substantive script revision, read
[the video editorial review](references/video-editorial-review.md). Stage the
audience knowledge, motivating problem, stakes, opening promise, carried example
and scope first; then develop and review the script alongside rough visual ideas
using [creative development](references/creative-development.md), before detailed animation.
Run the reference's fresh-context teach-back and save its answers and findings
in `editorial-review.md` beside the film. Plan what visuals must explain;
`video-author` chooses the appealing visual treatment. This is an authoring
check, not a new user-approval requirement.
A reviewed staging contract does not substitute for reviewing its script.
Renderer-only fixes retain the existing editorial decision unless they change
meaning, timing or accessibility.

For video, stage the **distribution job** before the arc: `linkedin-native`,
`youtube-companion`, or `article-embed`. These are different viewing contexts, not
three exports of one default cut. A native LinkedIn cut makes a complete argument without
a click and gets its own opening, payoff, runtime, and later editorial review; do not
mechanically trim a YouTube film. Start with a 30–75 second experiment only when that
fits the claim, never as a universal ideal. YouTube and article embeds serve deeper
demonstration, search/reference, onboarding, or intentional sharing. No destination
is evidence of audience preference by itself; name the primary measurement instead
(impressions are not watch time, and retention is not comprehension).

## Video development revision — October 2026

The v2 fields and executable safeguards remain. Motion roles now express semantic
intent; numeric taste limits are replaced by evidence-backed creative review.
Explore visuals and script together, select a motion language after a rough cut,
and audit it in playback. Concrete visual choices stay in `editorial-review.md`;
`staging.yaml` remains the argument contract. Historical lineage is not a second
source of active style rules.

## The composition contract (hard)

```
staging  owns WHAT is said and IN WHAT ORDER    → emits staging.yaml
design   owns HOW IT LOOKS                       → consumes staging.yaml
```

Two rules, both testable, neither negotiable:

1. **Staging never names a chart type, a layout, a palette, a font, a
   transition animation, or a slide count used as geometry.** If a staging
   artifact contains the word "bar chart", "two-column", "hero image" or a hex
   code, staging has leaked into design and the artifact is invalid. The
   closest staging may come to visuals is a *function* from the closed
   vocabulary (`evidence`, `comparison`, `mechanism`) and a *claim*. How that
   claim is shown is not staging's business.

   **The one exception, and it is narrow: `diagram:` on a `mechanism` slot.**
   A slot whose `function` is `mechanism` may carry an OPTIONAL `diagram:`
   key — a *redraw spec in words*: what the eye must find, and in what order,
   never a chart type, an axis orientation, a colour, or a layout. Worked
   example: *"the top of the stack is a flat line all the way across, at the
   value of the collateral… the eye should find it before it finds anything
   else."* The rule that makes the exception safe is that it changes nothing
   about what counts as a leak: `diagram:` prose is checked against the exact
   same banned-token list as every other field in the contract (chart type,
   layout word, hex code, font name), and the instant it names one it has
   re-crossed into design like anything else here — the key does not buy an
   exemption, only a place to put a sentence design still has to obey.
2. **Design never silently changes claims or order.** It may develop rough
   treatments, split a slot into shots, reword without changing meaning, and
   propose structural improvements. Return those findings to staging, update
   the contract and re-review affected content before detailed production.
   Approved content and scope remain binding. This is a development loop, not
   a requirement to finish the script before discovering its visual idea.

A failed gate never gets patched at the gate. It names the upstream step to
return to (`CLASSIFY`, `STAGE`) and you go back there.

## The flow

`INTAKE → CLASSIFY → LOAD → STAGE ↔ rough visual/script development → GATE → HANDOFF`

### 1. INTAKE — at most three questions, and skip them if you can

Do not interview. Three questions, maximum, and only the ones the brief has
not already answered:

1. **Who is in the room, and what decision do they hold?**
2. **How long, and will it be presented or read?**
3. **What must they believe by the end — or, for a tutorial, be able to do and check?**

**Skip the interview entirely** when the brief already names the audience, the
decision or judgment they hold, and the time slot. Say so out loud: *"INTAKE
skipped — the brief answers all three."* If a shared answer ledger exists
(house bake-offs, a client intake doc), read from it verbatim rather than
asking again.

Anything else you want to know is a **recorded assumption**, not a question.
Write it down and keep going. Never stall a staging pass on an unanswered
question.

For an unfamiliar topic, distinguish domain knowledge from topic knowledge:
an SDLC leader may understand delivery without knowing the research vocabulary.
Stage what the idea means, why this audience cares, and one concrete example
before relying on specialist terms. Put the explanatory relationship in the
claim/supporting text (or the existing mechanism `diagram:` field); leave the
choice of illustration or metaphor to design. Names and acronyms are not explanations.

The one-sentence belief must be **arguable**. "We use a queue for incoming work" is a topic. "GitHub's data model, not our discipline, is what lets a
security block get erased" is a claim. If the sentence cannot be disagreed
with, there is nothing to stage. For `teach-workshop`, use a testable capability
instead: starting state → learner action → observable success. The capability
occupies `belief` for compatibility; it need not be a contentious claim.

### 2. CLASSIFY — name the genre you chose AND the ones you rejected

Route in two moves.

**Move 1 — the room.** Choose the lane from how the audience will use the
pages, before applying a genre default. A timed, presenter-led talk defaults
to **hall**, even in a small room or when the presenter receives a PDF by
email. Choose **table** when the audience will read independently or use the
pages to deliberate. When both uses matter, record which leads in
`staging.yaml`. Use these cues to interpret the brief:

| Axis | → **hall** | → **table** |
|---|---|---|
| Room size | dozens to thousands | roughly 2–25 |
| Interaction | one-way, presenter drives | two-way, audience interrupts |
| Medium | projected, read at a distance | read close, often *also* read alone |

Hall and table are house names for a split the canon settled long ago: the
persuasion-in-a-small-room idiom and the perform-to-a-crowd idiom are
different crafts, and the most common structural failure is running one in the
other's room. Consequences:

- **hall** — sparse slots, one idea per slot, a plan for a deliberate change of
  gear roughly every ten minutes, no appendix in the walk, the artifact does
  **not** stand alone without its narrator.
- **table** — dense slots permitted, evidence and footnotes legal, appendix
  legal, an objection loop expected, and the artifact **must** stand alone,
  because half the audience will read it without you.
- **Mixed** — stage the walk for the leading audience. For a hall-led talk
  with a reading copy, put detailed evidence in the untimed appendix.

**Move 2 — the genre.** Choose by the viewer’s job:

| id | one-line mandate |
|---|---|
| `technical-deep-dive` | make a competent skeptic believe the mechanism is real and the tradeoffs are honest |
| `practitioner-explainer` | help an operator understand a choice, follow its consequences, and try one useful next step |
| `executive-explainer` | make a non-specialist decision-holder understand what changes and why it matters to them |
| `investor-pitch` | make a funder believe this team, this market, this moment |
| `board-update` | let a governing body exercise oversight without reading the appendix |
| `sales-proposal` | move a buying group from interest to a next commitment |
| `internal-decision` | get one specific decision made, in the room, today |
| `conference-keynote` | leave a room of strangers holding one idea they will repeat |
| `teach-workshop` | leave people able to *do* something they could not do before |
| `product-pitch` | make a buyer believe this specific priced product is worth exactly what it costs |

Full playbooks live in `references/genres/`. `technical-deep-dive`,
`executive-explainer`, `practitioner-explainer`, `product-pitch`, and
[`teach-workshop`](references/genres/teach-workshop.md) are complete; the rest are stubs in
`references/genres/STUBS.md` (id + mandate + pick_when only) and must be
written out before use.

**The output of CLASSIFY is a stated rejection.** Write, in the artifact:

```
CHOSE   <genre>  because <the audience's decision + the room>
REJECTED <genre> because <the specific pick_when it fails>
REJECTED <genre> because …
```

Two rejections minimum. A classification with no rejections was not a
decision. If two genres both fit, one **leads** (its arc, its time shape, its
gates) and the other **contributes named beats** — say which, and which beats.

### 3. LOAD — exactly one playbook

Read the chosen genre file end to end. Check its `kill_conditions` **now**,
before staging anything: a kill condition that fires means this is the wrong
artifact entirely (a memo, a doc, a conversation), and saying so is a
legitimate, valuable output of this skill.

Supporting references, loaded as needed:

| File | Read it when |
|---|---|
| `references/audience.md` | always — the audience block drives density, order, and the objection loop |
| `references/slide-functions.md` | while building the slot sequence — the function vocabulary is closed |
| `references/transitions.md` | at every beat boundary, and when the deck feels flat in the middle |
| `references/genres/<genre>.md` | the one chosen playbook |
| `references/video-editorial-review.md` | **always for video content work** — audience contract, spoken-script review and release evidence |
| `references/motion-roles.md` | **always, when the medium is video** — semantic motion roles and R1–R6 review guidance |
| `references/motion-spec.md` | video, at the design handoff — the selected and audited motion language |
| `references/seam-gate.md` | video, before authoring the boundaries — how to declare and verify seams |

### 4. STAGE — four passes for a deck, five for a film, in this order

1. **Arc.** Take the playbook's beats. Delete any beat the material cannot
   fill honestly; never invent material to fill a beat. Beats are named in
   house words, not the playbook's — the playbook's beat names are a template,
   your arc's beat names are about *this* subject.
2. **Slot sequence.** Assign each beat one or more slots, each with a
   **function** from the closed vocabulary. Obey the sequencing rules in
   `slide-functions.md` — some functions may not be split from their
   neighbour, some may not open a section.
3. **Headline claim per slot.** One declarative sentence per slot, stating the
   *insight*, not the topic. "Three problems with the fact log" is a label.
   "The fact log makes erasure structurally impossible" is a claim. Every slot
   gets one, including the cover. A slot that cannot carry a claim is a slot
   that should not exist. A `practice` slot's claim states the capability being
   tested; its prompt lives in the learning contract and is not replaced by a slogan.
4. **Bold moment.** Place exactly **one**. Read the playbook's
   `bold_moment.placement` rule and obey it — the placement is genre-specific
   and it is the single most common thing a bare draft gets wrong. **Read the
   lane-specific figure**: `placement_deck` is measured on slot indices,
   `placement_video` is measured on TIME and is derived from that lane's
   `runtime_shares`. Show the derivation. Record the beat, the form, and *why
   this one*. For video this names the narrative focal moment, not the only strong image or
   the largest movement. Treat genre timing windows as starting plans; a better
   rough-cut arc can revise them explicitly while preserving the runtime and evidence.
5. **Motion intent — VIDEO ONLY.** Read `references/motion-roles.md` and
   assign `motion_role`, relative `energy`, `peak`, `motion_turn` and `figures`.
   Distinguish events from conditions and illustrative transformations; do not
   manufacture an event to justify movement. Plan attention and discoveries,
   then refine from the rough cut. No fixed peak count, figure cap or energy
   average establishes quality. Mark unavailable playback checks pending.

### 5. GATE — shared and medium-specific checks, each naming its return step

| Gate | Test | Fails to |
|---|---|---|
| **Helicopter** | Read ONLY the headline claims, in order, with nothing else. Do they form the whole argument, standing alone? | `STAGE` pass 3 |
| **Bold moment** | One narrative focal moment; deck placement follows its playbook, video placement follows the recorded arc after rough-cut review; name the discovery or supported event | `STAGE` pass 4 |
| **Dead slot** | Every slot must change something — a belief, a stake, a level of detail, a direction. A slot where nothing turns is cut, not improved | `STAGE` pass 2 |
| **Kill conditions** | Re-check the playbook's list against the arc you actually built | `CLASSIFY` |
| **Time shape** | slots × per-slot floor ≤ minutes. State the arithmetic | `STAGE` pass 2 (cut slots) or `INTAKE` (renegotiate minutes) |

The dead-slot test is the ruthless one and it is worth stating in full: a slot
earns its place by turning a value — the audience believed X and now believes
Y, the stakes were low and now are high, the thing was abstract and is now
concrete. Repetition earns its place when the viewer retrieves, compares, predicts
or consolidates a model needed for the next step. Name that work. A recap that only
repeats earlier sentences, or an agenda that only lists topics, does not earn a slot.

The time-shape gate is arithmetic and must be shown: `minutes × 60 ÷
content_slots = seconds per slot`, compared against the playbook's floor. If
the answer is below the floor, **cut slots** — do not talk faster, and do not
quietly extend the time.

In the **video lane** the time-shape gate is a different and harder arithmetic:
`words = seconds × wpm ÷ 60`, apportioned across the lane's named `runtime_shares`.
On a deck the time gate cuts slides. On a film the time gate **writes the
script** — it decides how many words each beat gets, and a beat with no share
gets deleted rather than compressed.

For `teach-workshop`, use its separate presentation, demonstration/inspection and
learner-work budgets. Narration uses only speaking time; live practice is included
in session time, while viewer-controlled pauses are stated separately from encoded
video runtime. The tutorial playbook's timing and checkpoint rules govern these
slots; do not cut necessary practice to satisfy a presentation-only per-slot floor.

**Genre-specific extra gates run here too, alongside the five above, never in
place of them.** A playbook may author its own named gates when its evidence
calls for one; `product-pitch` is the current example, with three: `mechanism_by`
(the first `mechanism`-function slot lands at ≤35% of content slots),
`problem_share` (the problem beat holds 30–45% of the time budget), and
`cost_slot` (a `cost` slot is present and its figure is a `ship` or
`ship_with_as_of` row in the `number_gate`, never `withhold`). A failure in one
of these returns to the same upstream step its own playbook names, exactly
like a failure in the five above.

#### The video GATE — editorial before detailed animation, then production checks

These checks span authoring stages. A script-only handoff records unavailable
visual/render/voice checks as not-yet-checked with their next owner; it does not
run production prematurely or claim a finished film. Required production checks
must pass on the delivered version before production-ready status.

| Gate | Test | Fails to |
|---|---|---|
| **Spoken story reviewed** | `editorial-review.md` addresses every criterion in the shared review reference with scene-specific evidence, plus the fresh-context teach-back and cited gaps; unresolved script failures return upstream before detailed animation, unavailable checks are explicitly pending | `STAGE` / script revision |
| **Visual quality reviewed** | Design records the visual treatment and inspects stills and playback for explanatory value and appeal; generic prose-card sequences and default block diagrams need a deliberate treatment, not merely animation | design |
| **Motion language reviewed** | `references/motion-spec.md` records the selected treatment after exploration and the delivered-sequence audit; revisions and unresolved findings are explicit | design |
| **Seams declared + gate passing** | Every scene boundary in a seams ledger, `tools/seam-gate.mjs` run against the delivered file, **0 fail** | design (the boundary is the fix) |
| **Motion intent checked** | `motion_budget` records semantic purpose, relative emphasis, contrast and repetition using R1–R6; numbers describe the plan, not a taste threshold | `STAGE` pass 5 |
| **Attention reviewed** | Figures have explanatory or orienting jobs; inspect hierarchy and pacing in playback, with actual findings and pending checks stated | design / `STAGE` pass 5 |
| **Render and LOOK** | Headless render of every scene, ≥ 6 screenshots including mid-animation, zero page errors, the done signal on both the injected and the fallback duration paths, scene count == script row count — **and a human or model actually looks at the frames** | design |

The last one is mandatory and the reason is worth stating: on the runs behind
v2, every automated check passed on a frame whose first command line soft-
wrapped and read as a second command, and on a scene that played its entire
choreography in three seconds and then sat still for seventeen. **An overflow
measurement tests whether pixels are inside the frame, not whether the frame is
legible**, and a scene-count check cannot see a pace. Both were caught by
looking at a screenshot.

### 6. HANDOFF — emit staging.yaml

```yaml
brief: <name>
genre: <id>
lane: hall | table | mixed(<leader>)
classify:
  chose: {genre: …, because: …}
  rejected: [{genre: …, because: …}, …]
audience: {assumed_knowledge: …, distrusts: …, decision_they_hold: …, attention_budget: …}
belief: "<the one arguable sentence>"
time_shape: {minutes: N, medium: deck|video, seconds_per_slot: N, floor: N}
arc:
  - {beat: …, minutes: …}
bold_moment: {slot: N, beat: …, form: …, why: …}
number_gate:
  - {figure: …, value: "…", source: "…", as_of: "…", rederivation: "…", verdict: ship|ship_with_as_of|withhold, reason: "…"}
glossary:
  - {term: "…", verdict: keep_gloss_once|rename|drop|internal_only|banned, outbound: "…"}
slots:
  - {n: 1, function: cover,     beat: …, claim: "…"}
  - {n: 2, function: mechanism, beat: …, claim: "…", supporting: "…", diagram: "…"}
transitions: [{after_slot: N, type: turn|narrow|escalate|fold|gate, line: "…"}]
gates: {helicopter: pass, bold_moment: pass, dead_slot: pass, kill_conditions: pass, time_shape: "…arithmetic…"}
assumptions: ["…"]
```

**`number_gate:` and `glossary:` are REQUIRED blocks** — a `staging.yaml` with
either missing is malformed, the same way one missing `slots:` would be.
Every figure or novel term the material actually carries gets a row; there is
no "trust the author" path left once a row is possible. Both are checked by
`tools/staging_lint.py`. Its parser accepts only the documented restricted YAML
forms: write `slots`, `number_gate`, `glossary`, and `arc` as block sequences
of single-line flow mappings. Inspect the reported row counts against the
source. A linter PASS is a limited content check, never an editorial verdict.

- **`number_gate`** — one row per figure that appears anywhere in the
  artifact's `claim:` or `supporting:` text, `verdict ∈ ship | ship_with_as_of
  | withhold`:
  - every figure in any `claim:`/`supporting:` string must have a row;
  - `withhold` requires a non-empty `reason`; `ship_with_as_of` requires a
    non-empty `as_of`;
  - two rows sharing a `figure` and a `source` but disagreeing on `value` ⇒
    **both** rows become `withhold` — a same-source conflict is never
    resolved by picking a side;
  - a `withhold` figure reaching any `claim:`/`supporting:` string is an
    error, not a warning — that is the exact defect this block exists to
    catch (a load-bearing number dropped from the deck while the pass was
    reported clean).
- **`glossary`** — one row per novel term used in any `claim:`/`supporting:`,
  `verdict ∈ keep_gloss_once | rename | drop | internal_only | banned`:
  - a `keep_gloss_once` term must carry its `outbound` gloss text in the slot
    where it FIRST occurs;
  - `rename` / `drop` / `banned` terms must not appear in ANY slot at all —
    the row records the decision, the check enforces it stayed made.
- **`slots[].supporting`** — an OPTIONAL second sentence on a slot, alongside
  `claim`: a subordinate detail, a citation, or the place a `number_gate` or
  `glossary` row's obligation gets satisfied without overloading the headline
  claim itself. Scanned for figures and terms exactly like `claim`.
- **`slots[].diagram`** — an OPTIONAL key, legal ONLY on a slot whose
  `function` is `mechanism` — see the composition-contract exception above.
  Illegal on every other function.

**Video adds five per-scene fields and one block** (see
`references/motion-roles.md`):

```yaml
scenes:
  - {n: 1, function: …, beat: …, claim: "…", seconds: N, words: N,
     motion_role: hold|accumulate|strain|rupture|arrest|assemble|divest|cadence,
     energy: 0-5, peak: bool, motion_turn: "…", figures: [{name: …, renders: "<relationship, clause or attention task>"}]}
motion_budget: {peaks_earned: N, ceiling_order: "…", quiet_scenes: […],
                mean_energy: N.NN, adjacency: …, rhymes: […],
                figures_total: N, ornament_gate: …}
editorial_review: <path to the script review and later production-review status>
viewer_contract: {problem: "…", stakes: "…", opening_promise: "…", question: "…", takeaway: "…", next_action: "…", carrying_example: "…", companion_scope: "…", runtime_seconds: N}
distribution: {destination: linkedin-native|youtube-companion|article-embed,
               continuation: none|article|youtube, primary_metric: "…",
               brand_surface: personal-executive|product|other,
               brand_source: <optional project authority>}
seams: <path to the declared-seam ledger>
motion_spec: <path to the selected motion-language instance>
```

For video, `distribution.destination` and `brand_surface` are required.
`linkedin-native` uses `continuation: none` and a self-contained payoff; depth links
cannot complete its argument. Its default brand is `personal-executive`, guided by
[the personal-executive reference](references/executive-presence.md): long-career
technology leaders drawing on applied AI, operational scale, regulated-system
controls, open-source work, and organizational leadership without depending on a
named person's external files. `brand_source` is optional only for
`personal-executive`; it is required for `product` and `other`, naming the current
publication/series or other authority. A missing required source records an open
visual decision and stays brand-neutral, never a fallback to the shipped product
default. A supplied source takes precedence over the `brand.toml` resolution in
video-author §7. `primary_metric` names behavior available in that
destination, such as native watch time, target-role comments, YouTube retention, or
tagged site engagement.

For video, `audience.assumed_knowledge` names concepts the viewer already knows;
`audience.concepts_to_explain` names unfamiliar topic and product concepts.
Industry seniority does not fill either field. Motion-budget numeric fields are descriptive; no former energy or figure cap
is an active video gate. The shared editorial review owns
the judgment checks and their production statuses; `staging_lint.py` does not
establish comprehension or visual appeal.

`teach-workshop` also requires the linked `learning_contract` described in its
playbook: prerequisites, demonstrated steps, practice/feedback pairs, transfer and
timing. It is reviewed manually; the content linter does not validate it. Keep
task state and success criteria there, and concrete screen treatment in design.

`staging.yaml` is a **deliverable in its own right**, not scaffolding. It is
the reviewable artifact — an argument you can disagree with before anyone has
spent an hour on a layout. Ship it beside the deck.

Nothing in `staging.yaml` may name a visual. If you are tempted to write
`visual_family: chart`, that temptation is the leak this contract exists to
stop.
