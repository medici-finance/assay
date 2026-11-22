---
name: video-author
description: Author a staged explainer as a narration script, seekable HTML preview and native final render using supplied audio.
---

# Video author

Paths below are relative to this installed skill. Set VIDEO_SKILL to its absolute directory. Keep the script, visuals and generated files in the author’s project. Use narration the project supplies.

## 0. Review the story and choose its visual treatment

Read [staging's video editorial review](../staging/references/video-editorial-review.md).
Consume its viewer contract and `editorial-review.md` with `staging.yaml`.
If the review is missing, draft the script and perform that review first;
return structural findings to staging before animation. Honor already approved
content and the user's requested scope: a renderer-only repair does not require
an unrelated script rewrite. A content revision reopens the affected criteria.

Carry the review forward with separate script, visual, voice/caption, render
and agent teach-back / human audience-test statuses. Follow the shared review's
fresh-context procedure at script stage and on the completed film when available;
record pending voice and film checks at this skill's authoring boundary.
Authoring gates do not add a user-permission step.
A renderer PASS cannot clear an editorial finding or a pending voice review.

Honor `staging.yaml`'s `distribution` block; §7 owns brand-source precedence and
fallback. For `personal-executive`, use
[the personal-executive reference](../staging/references/executive-presence.md) and
any optional project-supplied `brand_source`; do not require an external README or
career repository. Do not substitute product branding or invent personal history;
record the project's visual choices rather than deriving a personal theme from the
product. For `linkedin-native`, open immediately with the staged problem or claim
rather than a logo/title-only runway; keep the staged 30–75 second experiment when it
fits, provide full-speech captions, and preview the intended square or vertical feed
crop at phone size while preserving any required 16:9 master. End on the operational
insight or specific operating question, not a bare “watch the full video” instruction.
Do not mechanically trim the YouTube cut or inherit its editorial pass. For
`youtube-companion` and `article-embed`, preserve the deeper demonstration/reference
job and an independently meaningful opening. In every lane, record the staged
`primary_metric` as viewing/engagement evidence, never proof of comprehension or
audience seniority.

Read [creative development](../staging/references/creative-development.md) and
[screen grammar](references/screen-grammar.md). For a new film or substantive redesign, explore distinct visual treatments
at a scale appropriate to the request, then test the promising sequence as an
animatic before detailed animation. A minor repair retains the existing treatment
and rechecks only affected criteria. Record the alternatives, visual discovery,
metaphor limits and choice in `editorial-review.md`; no extra approval is required.
Use approved existing or explicitly authorized temporary audio when available.
A silent study leaves audiovisual rhythm and voice alignment pending.

For a requested seekable explainer or script/storyboard review, use the installed
`assay:system-demo` workflow. Show the current scene alongside its synchronized
storyboard notes and chapter navigation, with a readable narrow-screen arrangement.
Keep one source for scene order, narration and notes so seeking cannot show a stale
shot. Label illustrative states and provide a text alternative. Verify forward and
backward seeks, pause/resume and scene-note synchronization in the actual preview.
A working preview does not establish final voice, caption or film quality. An ordinary
film request does not acquire an extra interactive deliverable through this rule.

Carry staging's narrative bridges into the visual timing: let the narration orient
the viewer before a new diagram or actor appears, then reveal the relationship it
explains. Return an abrupt or missing transition to staging; a transition effect
cannot repair disconnected prose. Keep inspection time after the reveal and verify
phrase-level alignment on the actual voice take, leaving that check pending until
it exists.

At handoff, name `explainer.html` as the preview and the native project/build as
final-picture source, with their full directories, beside the §9 deliverables.
Identify the shared timeline and prepared audio/caption inputs, or mark them
pending. The `explainer-video` README owns the `render-final` mechanics; production
runs only within §9 authorization. A browser demo or HTML gate pass cannot
substitute for the native recording-source and delivered-file checks.
For site publication, if a publication skill such as `linkedin-post` is installed,
hand off to its full held assay-site bundle workflow; otherwise stop at the rendered
deliverable and say so. Keep animated maintainer source outside the deployed public
tree. Preserve approved narration and scene order while packaging; label any
remaining production checks.

## 1. The composition contract (hard)

```
staging       owns WHAT is said and IN WHAT ORDER   → emits staging.yaml
video-author  owns HOW IT LOOKS AND MOVES           → consumes staging.yaml
```

- Staging owns claims and order. Design may propose a better explanation,
  develop rough shots and return findings upstream. Update `staging.yaml` and
  re-review affected evidence/script criteria before detailed production; never
  quietly reorder or rewrite approved content in the timeline. Phrasing changes
  for speech or line breaks must preserve meaning and consequential limits.
- Motion fields express semantic intent, not fixed aesthetic ceilings. Refine
  them with staging when exploration improves the sequence, recording the reason.
  One pipeline scene can contain several shots without changing narration rows.
- A failed safeguard names the upstream repair. Creativity does not waive it.

## 2. The shared script and HTML preview contracts

`tools/native_export.py` is the default final export path: a native
fframes picture plus prepared sound/captions. The native rendering reference documents its input binding
and checks. The legacy `explainer-video` and `render-job` commands still record
HTML; select them only as an explicit documented fallback, not for a new default
export. This skill does not reroute or deploy them.
Narration is measured before either view is retimed. Preserve the following
script and HTML contracts; the native counterpart uses the same scene identities,
measured timings and take-sanity bounds ([native rendering](references/native-rendering.md)).

**`SCRIPT.md`**
- A first `# ` title heading — it slugifies to the output filename (≤100
  chars).
- One Markdown table, one row per scene, in play order. The pipeline matches
  `id`, `narration`, and (optionally) `subtitle` columns case-insensitively;
  house-added columns (`~sec (est)`, `words`, `act`, `motion role`,
  `on-screen text`) are legal and ignored by the parser.
- **No `subtitle` column** is the sanctioned default: the narration is reused
  as the subtitle (the accessible answer). On-screen text stays 2–5 words —
  it is what the viewer *reads* while the voice says the sentence, never a
  transcript. The current caption generator emits one cue per scene; final
  caption segmentation and speech alignment remain a production check, as
  specified in the shared editorial review.
- Optionally a `## Description` H2 section — its verbatim body becomes the
  YouTube description `.txt` and mp4 metadata.
- The "8-scene table" in older house prose is a **description, not a
  contract**. Scene count comes from one-idea-per-scene against the staged
  arc; use roughly 9–30 s for narrated explainer scenes. For `teach-workshop`,
  split at task checkpoints and preserve time to inspect state. Longer deliberate
  demonstrations or practice screens need a stated purpose and checked timing;
  they are not permission to stretch narration or force a cut mid-operation.

**Words per second — the pacing unit.** "Author at a natural pace" has a
unit, and it is *words*:

- conversational / explainer register: **150 wpm** → 1 s = 2.5 words
- technical / deep-dive register: **140 wpm** → 1 s = 2.333 words

Write every narration cell to a word budget; `~sec = words ÷ rate`. Recount
the declared word counts against the narration text before shipping. Every
`~sec` is an **ESTIMATE, and must be labelled as one** in `SCRIPT.md` — no
TTS take exists yet, and the measured take overwrites it. Sanity-check the
ceiling against a take that runs 10 % slow. A film that mixes registers
(a relay) runs each act at its own rate; a single blended rate makes one
register silently wrong.

Treat those rates as planning anchors, not performance targets. With actual takes,
compare speech-only pace by passage as well as whole-film pace including holds;
an acceptable average can hide a rushed opening or dense explanation. Direction
uses the approved performance and [timing guidance](references/voice-delivery.md):
explain to one colleague, vary emphasis naturally, and leave room for ideas and visuals
to land. Adapt that default to the audience and argument. Budget speech, thought pauses
and inspection holds separately; the planning rates above exclude added silence.
Audition representative passages when establishing a materially different delivery;
reuse selected audition takes in the full film. A prompt, word count or transcription
cannot establish how delivery sounds; record listening as pending until performed.
Preserve an explicit speaker choice or existing approved track unless a change is
authorized. Realign captions and both renderers after changed takes or inserted pauses.
Do not globally time-stretch speech to meet an arbitrary words-per-minute target.

For tutorials, consume staging's `learning_contract`. Separate speech estimates,
demonstration/inspection holds and viewer-controlled pause time. The renderer
replaces scene durations with measured takes: an estimate column or `hold` motion
role does not insert silence. Record how a planned encoded hold will be implemented
and checked by the production owner; leave it pending until verified. Use explicit
pause/resume prompts for learner work outside the encoded runtime. Preserve the
exact actions, readable task state and expected result even where interface or
code text must exceed the usual 2–5-word label guidance. Keep solutions after the
attempt opportunity and chapter boundaries at recoverable checkpoints.

**`explainer.html`**
- **Self-contained**: inline CSS/JS, all fonts base64-embedded
  (`document.fonts.check` must resolve them), inline/data-URI images, zero
  external `src`/`href`/`url()` references. The pipeline copies the file to a
  work dir before recording; any external reference breaks there.
- **Exactly ONE `window.SCENE_DURATIONS` assignment** in the whole file —
  comments and JS strings included. The pipeline rewrites the *first* match
  of `window\.SCENE_DURATIONS\s*=\s*\[[^\]]*\]\s*;?`; a second assignment
  silently steals the injection and drifts the film off the voice with
  nothing in the logs. Make a duplicate a build error.
- Declare it **empty** (`window.SCENE_DURATIONS = [];`) and carry the
  authored estimates in a separate `FALLBACK` array the runtime uses with a
  console warning — so which duration path ran is never ambiguous, and both
  paths are testable (§8).
- **`document.body.dataset.done = "1"`** when the timeline finishes, plus
  `dataset.doneReason = complete | error | watchdog` — a failed render must
  never look like a successful one. A broken build (no scenes, a thrown
  boot) reports `error` in seconds; a watchdog bounds the recording.
- **Scene count == `SCRIPT.md` row count**, always. Give every scene
  `<section class="scene" data-scene="<id>">` with the SCRIPT row's id.
- The narration is the clock and the visuals follow — author each scene's
  beats at natural pace and let the runtime reconcile the authored span
  against the measured slot (§6). Never hard-code a timing that assumes a
  narration length.
- **`window.__player.total` is reported in SECONDS, `window.__player.scene`
  in the 1-based scene index — never the other way round.** One bake-off
  track exposed `__player.total = 9` in scenes where nine exposed it in
  seconds; `house-player/verify.mjs` fails that shape. Freezing a frame is
  **`?at=<sec>`**, not `?still=`: it seeks to an absolute time and pauses,
  driving every `document.getAnimations()` entry to the matching
  `currentTime` so the frame is deterministic. `?scene=N` (1-based) starts
  playback at a scene instead. Full shim and both units:
  [the driver contract](references/driver-pattern.md).

## 3. Apply semantic motion intent

Use [motion roles](../staging/references/motion-roles.md), including its R1–R6
review principles and compatibility fields. Preserve the distinction between
observed events, conditions, metaphors and proposed behavior. Do not invent a
causal event to justify an effect. The story's focal moment need not have the
largest movement, and several discoveries can carry the same argument.

Use the genre's time shape as a planning starting point, then check the actual
word budget and cumulative runtime. If the visual/script development changes the
arc, revise staging deliberately and retain the user's runtime and content
constraints. Do not accelerate narration or silently drop qualifications to hit
a preset climax percentage. Record actual emphasis and inspection time.

## 4. Select and audit a motion language

Use [motion language](../staging/references/motion-spec.md): provisional choices
for exploration, a selected language before detailed production, then an audit
of the delivered sequence. Its restrained technical-explainer preset is optional.
Purposeful anticipation, rotation, overshoot, varied shot scale and rhythm are
available; evidence, readability, accessibility and deterministic playback remain
constraints. Record why a revised choice helps and recheck affected boundaries.

Review at speed and in stills. Name the strongest discovery, weakest passage,
repetitive framing and likely wrong inference. Record what helped, competed or
stayed still with reasons; no quota of cuts, figures, curves or failures. Check
against actual voice/captions when available and retain pending statuses otherwise.

## 5. The seam ritual

Every scene boundary is designed, declared, and numerically verified. The
wiring reference is `../staging/references/seam-gate.md`; the tool is
`../staging/tools/seam-gate.mjs` (vendored Apache-2.0, licence file beside
it; node ≥22 + local Chrome, zero npm).

1. **Declare ALL boundaries** in `seams.json` beside the film, **before**
   the master timeline exists — the ledger is what forces the seam design.
   A 22-cut film with 4 declared seams has 18 unaudited cuts. Opt a boundary
   out only when it is genuinely not a cut, with the reason in `_doc`.
2. **The current verifier supports motion cuts, editorial hard cuts, match cuts and morphs.**
   Production requires zero overlap: the gate measures it for motion cuts and hard
   cuts; separately check it for match cuts and morphs as the wiring reference describes.
   Keep those production boundaries passing. Dissolves are not artistically
   forbidden, but the current gate cannot validate them: delivery would first
   need appropriate verification, not disabled tests or a false PASS. A stationary
   match cut may preserve a reference point; do not invent travel to satisfy a rule.
3. Design the eye path through each cut. Choose movement, scale and direction for
   the relationship being explained; there is no film-wide quota of Z moves or
   required entry/exit ratio beyond the declared verifier checks. Keep the ledger
   consistent with what the viewer actually sees.
4. Expose the **~15-line `__player` shim** in the shipped file — bridge the
   runtime's master timeline to `window.__player` (`pause`/`play`/`seek`/
   `duration`) plus `window.__playerReady` and `window.__renderReady` — so
   the gate is re-runnable by anyone against the delivered artifact. The
   render check (§8) reuses the same shim. Verbatim pattern in
   `../staging/references/seam-gate.md`.
5. **Run the gate** from the repo root, and require exit 0:

   ```bash
   node "$VIDEO_SKILL/../staging/tools/seam-gate.mjs" verify \
     --ledger <film>/seams.json \
     --url http://127.0.0.1:1 \
     --comp-url "file://$PWD/<film>/explainer.html"
   ```

   `--url` is required-but-unused (the dead address skips the server-spawn
   branch); `--comp-url` is what is navigated. **Never pass `--project`** —
   that branch runs `npx --yes hyperframes preview` and installs from the
   network.
6. Cut times are the cumulative durations **the page is actually running**;
   if those are estimates, the ledger is in estimate time and says so. Any
   edit to a scene's first/last ~1 s, or to any duration, invalidates every
   later boundary's audit — re-derive and re-run, and note it in `_doc`.

## 6. Final renderer and HTML preview

Use the dependency-free [player](tools/player.js) for seekable HTML previews. Inline it into the finished HTML. The [driver contract](references/driver-pattern.md) describes measured durations, deterministic seeking, readiness and completion. For native output, follow [native rendering](references/native-rendering.md). Prepare a locally built fframes executable and a soundtrack with captions, or reuse the sound and captions from an authorized reference movie.

## 7. Diagram and code-on-screen grammar

Use [screen grammar](references/screen-grammar.md) for readable records, clear
relationships and shot continuity. Text floors, full-speech captions and truthful
state remain safeguards. The default diagram layout, record count or entrance
style is not a mandatory aesthetic. Choose an illustrative composition or
perspective shift when it explains more than a familiar card sequence.

**Brand.** When `staging.yaml` declares `distribution.brand_surface`, that
declaration takes precedence over any `brand.toml` fallback. A supplied
`distribution.brand_source` resolves first. `personal-executive` never falls back
to an explicit-path, consuming-repo, or shipped-default `brand.toml`: with no
project authority, use the brand-neutral treatment in
`../staging/references/executive-presence.md` and record the visual choices or open
decision. `product` and `other` require `brand_source`; if it is missing, record
the open decision and stay brand-neutral rather than resolving a product default.
Only a film with no declared `brand_surface` uses the legacy order: an explicit
path, `brand.toml` at the consuming repo's root, or the shipped default instance at
a project-supplied brand contract, whose marks, palette, and accent supply
the emphasis colour. Schema: `../deck-author/references/brand-contract.md`. With no
contract, the film is brand-neutral by the same honesty rule deck-author states. For genuine chart
scenes (a curve, a plotted formula) the house dataviz discipline applies
unchanged: validated palette, direct labels in text tokens, one axis, and
the renderer as a clock over a hand-written sampler, not a maths engine.

## 8. The render-and-look gate — mandatory

Headless render check plus a human or model **actually looking at frames**.
Automated assertions catch overflow and contract breaks; the defects that
survive them — an illegible soft-wrap reading as a second command, a stamp
landing on legible text, a scene that plays out in three seconds and sits
still for seventeen — are invisible to measurement and obvious to an eye.
*An overflow measurement tests whether pixels are inside the frame, not
whether the frame is legible.*

**The gate is a shipped harness, not a recipe to rebuild:**

```bash
cd "$VIDEO_SKILL"
node tools/render-check.mjs --contact-sheet <film-dir>   # freezes every scene, writes shots/contact.png + INDEX.md
node tools/render-check.mjs <film-dir>                   # the gate; exit 0 + last line PASS, or it is not a pass
```

Required, all of it, and the harness asserts every item: zero page/console
errors, zero off-`file://` requests, fonts resolved, scenes == SCRIPT rows,
one `SCENE_DURATIONS` assignment, nothing overflowing the 1920×1080 stage
(measured per element, seeked — off-frame elements can be clipped invisible
by their own container), the front-load probe (visible content at 85 % of
each slot ≥ at 15 %), **both duration paths** (injected through the
pipeline's own regex, and fallback) reaching `done=1 reason=complete` on the
right array, **≥6 frames measured mid-animation**, a **negative control
proven to actually be negative** (a genuinely broken build reports
`doneReason=error` — two of three naive controls in the exemplar run passed
because the film was fine, not because the control worked), §8b's wall-clock
checks, and a **contact sheet** — `shots/contact.png`, one tile per scene
(plus one per slide when a deck sits beside the film), indexed in
`shots/INDEX.md`. The harness fails when the sheet is missing, stale, or its
tile count is not `scenes + slides`. It gates **house-player films** (and
decks): a film that never raises `window.__playerReady` — a GSAP-adapter film
(§6 choice 2) — fails `load` as not freezable, never passes. Frames and sheets
are git-ignored build outputs; never commit them.

Every seeked frame goes through one freeze: `?at=<sec>`, JS time
(`requestAnimationFrame`, timers, `Date`, `performance.now()`) held on a fake
clock advanced to `<sec>`, `document.getAnimations()` driven to that time and
paused, SMIL timelines and `<video>` paused. Engine beats and walked animations
land exactly at `<sec>`; JS, SMIL and video run on one clock from page load.
Every kept frame is then captured twice, apart in real time, and must not have
moved — motion no freeze can hold (an animated GIF, an iframe's own document)
fails the gate instead of passing as frozen. **Never
`--virtual-time-budget`**: it fast-forwards `requestAnimationFrame` and leaves
every CSS animation at `currentTime: 0`, so every "mid-animation" frame is the
scene-entry state. Freeze mechanism, each pass's exact assertions, the sheet's
contract and the negative-control lore: `references/render-check.md`.

Then look: open the contact sheet, then the mid-animation frames, and read
every frame as a viewer. Record what the looking found in NOTES **with the
sheet's path**. A NOTES entry claiming a look-pass without a contact-sheet
path is not a pass. A gate that was not run is reported as NOT RUN — never
implied.

Also complete the shared editorial review's **visual explanation and visual
appeal** criteria in `editorial-review.md`, citing the inspected frames and
playback. Judge the composition before judging movement: hierarchy, readable
type, useful imagery, connector clarity and continuity of the carried example.
Call out a weak visual treatment even when every machine assertion passes.
Check phrase-level reveals against the actual voice when it is available;
scene-duration scaling alone cannot establish that sync. Pending voice or final
film teach-back remains explicit, never cleared by silent HTML playback.

For a native final export, also complete the native parity/delivery checks in
[native rendering](references/native-rendering.md). Keep preview and native check
results distinct; the browser harness does not inspect the MP4 produced by fframes.

## 8b. Timing sanity and browser wall-clock discipline

Rule 1 applies to both renderers. Rules 2–4 retain the HTML preview/recorder
checks; native frames instead use the deterministic media clock and exact frame
count/duration checks. Native export wall time is not playback duration.

A 2026-08 render shipped a 12:14 film against a 7:06 script with every gate
green. The mechanism was **not** recorder lag: the per-scene TTS produced one
runaway take (327.68 s of continuous speech for a 25.71 s row), the pipeline
measured it and injected it into `SCENE_DURATIONS`, and every downstream
guard — the scene scaling, the film's watchdog, `record.mjs`'s wait budget —
**trusted the poisoned number**, so the film obediently held one scene for
five and a half minutes and reported `doneReason=complete`. The injected
array is a *measurement*, and a measurement can be wrong; nothing downstream
of the injection may treat it as ground truth against the authored script.

Four rules, all mandatory:

1. **Take-sanity gate in the film.** On the injected path, compare every
   injected duration against the authored estimate (`FALLBACK`). Real TTS
   pacing drifts ~1.0–1.4× off a wpm estimate; it never explodes past it.
   Fail the render (`doneReason=error`, naming the scene) when a scene's
   measured duration exceeds `max(authored × 1.8, authored + 8 s)` or the
   total exceeds authored × 1.3 — a runaway take must produce a **red
   render**, never a long one. (Patched into the hybrid and animated
   exemplar films 2026-08-18; poisoned-injection test fails in <1 s.)
2. **lagSmoothing(0) on the LIVE ticker.** GSAP's default lag smoothing
   (`500ms/33ms`) freezes timeline time during jank, stretching wall clock
   under load. Call `gsap.ticker.lagSmoothing(0)` on the gsap instance that
   actually drives the master timeline — and *prove* it executes with the
   sync telemetry below, because a call sitting in dead adapter code passes
   every grep. Hand-rolled drivers must sequence on **elapsed time**
   (`performance.now()` deltas), never by counting rAF frames.
3. **Per-frame cost budget.** Every scene must hold real time at 1920×1080
   *with `recordVideo` on* — the recorder is part of the load. If a scene's
   sync drift exceeds ~2 % under recording conditions, cheapen the frame
   (canvas-bake grain/atmosphere as the staged-animated film does, reduce
   filter/blur/glow work) until it holds; do not accept the drift.
4. **Sync telemetry + webm proof in the render gate.** The film logs
   `[sync] scene N timeline=Xs wall=Ys drift=Zs` at every scene boundary
   (and exposes `__explainer.syncMarks`). Verification runs `record.mjs`
   (or an equivalent `recordVideo` run) for real and asserts: every
   boundary drift within ~2 % of elapsed, `doneReason=complete`, and
   **webm duration within 2 % of sum(SCENE_DURATIONS) + tail + ~1.5 s
   pad**. The webm is the test; the mp4 encode stays the human's (§9).

Recipe and measured exemplar numbers: `references/render-check.md`.

## 9. Production handoff

Deliver SCRIPT.md, the seekable HTML and the staging and editorial reviews. For a final export, also deliver the measured timeline, native manifest and rendered movie. Bind the exact inputs and inspect the final picture against the soundtrack and subtitles. The plugin accepts supplied media. Publication is a separate action.
