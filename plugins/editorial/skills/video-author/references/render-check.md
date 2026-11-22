# The HTML render-and-look gate — the shipped harness

Keep this harness for the HTML preview. It does not validate the native final
video: also run [native parity and delivery checks](native-rendering.md#native-parity-and-delivery-checks).
A preview PASS never clears a native-render or voice/caption check. Recorder
wall-clock tests below apply to the browser path; native output is checked against
its exact media frame clock, not against the time taken to export it.

Two halves, both mandatory: a headless assertion pass, and a human or model
**looking at the frames**. The harness is shipped:
`tools/render-check.mjs` (node builtins + Playwright +
ffmpeg). Run it; do not copy a recipe out of this file and rebuild it.

```bash
cd "$VIDEO_SKILL"
node tools/render-check.mjs --contact-sheet <artifact-dir>   # make the sheet, then the gate
node tools/render-check.mjs <artifact-dir>                   # the gate: needs a current sheet
```

`<artifact-dir>` holds the artifacts at its top level: every `*.html` carrying
`data-scene` is a film (the house-player contract), every `*.html` carrying
`class="slide"` sections is a deck. Everything the run writes goes to
`<artifact-dir>/shots/` — frames, `contact.png`, `INDEX.md`, `manifest.json`
and the full transcript `render-check.txt` — and `shots/` is git-ignored.
Exit 0 and a last line of `PASS` only when every pass below holds; otherwise
every failure prints as `FAIL: <check>: <detail>` and the exit is non-zero.
Anything the run could not check prints as `NOTICE (not checked)` and is
counted on a `NOT CHECKED:` line — it is never a pass.

## Browser dependencies

Install Playwright in a project or a dedicated dependency directory. Set EDITORIAL_RENDER_DEPS to the absolute directory containing node_modules when running elsewhere. Install Chromium and the recording helper with `npx playwright install chromium ffmpeg` in that directory. The checker also calls the system ffmpeg. To use an existing Chrome installation, set RENDER_CHECK_CHROMIUM to its executable.

## Playwright without npm install in the working repo

The harness imports Playwright by absolute path from the house pipeline's own
install, `tools/explainer-video/node_modules/playwright/index.mjs`. When that
is absent it stops with the install line
(`cd "$VIDEO_SKILL" && npm install && npx playwright install chromium`)
and exit 2 — it never degrades to a source-only check, because a render gate
that rendered nothing is NOT RUN. Pass D's `recordVideo` also needs
Playwright's own ffmpeg (`npx playwright install ffmpeg`). On a machine whose
Playwright browsers are not installed, `RENDER_CHECK_CHROMIUM=<binary>` points
the launch at a local Chromium.

## The freeze — how every seeked frame is taken

A frame the gate looks at is only evidence if the state in it is the state at
the time it claims. The harness takes every seeked frame the same way, in this
order:

1. install a **fake JS clock** (Playwright `page.clock`) at a fixed epoch and
   pause it, before any film script runs — `Date`, `performance.now()`,
   `setTimeout`/`setInterval` and `requestAnimationFrame` move only when the
   harness moves them;
2. navigate to the film with **`?at=<sec>`** — the ONE freeze convention
   (`presentation-skills/02` chose it; a film honouring only `?still=` fails
   the engine's own conformance suite, `renderers/house-player/verify.mjs
   --determinism`, not this harness);
3. wait for `window.__playerReady`. A page that never raises it is **not a
   house-player film** and fails `load` saying so: the GSAP adapter exposes
   neither `__playerReady` nor `?at=`, so this gate cannot freeze it (SKILL
   §6) — preview it with `renderers/gsap/qa.mjs`;
4. advance the held JS clock to `<sec>` after load (two frames, then one jump
   that fires the pending `requestAnimationFrame` and each due timer once),
   then hold it — a canvas, JS tween or counter computed from **elapsed time**
   (SKILL §8b: never frame-counted) lands at `<sec>`;
5. walk **`document.getAnimations()`** and, for every entry, `pause()` it and
   then set its `currentTime` to the offset the film's schedule implies at
   `<sec>`: an engine beat (`data-at`/`data-dur` inside a `[data-scene]`) sits
   at `clamp(sec − sceneStart − data-at, 0, data-dur)`; any other animation in
   a scene runs on that scene's clock from scene entry (its own delay lives in
   its effect timing). Pause **first** — `pause()` on a running animation is
   asynchronous and takes its hold time a frame later, so set-then-pause lets a
   `.scene.on`-triggered `@keyframes` drift past its offset (measured +8.5 ms);
6. pause every outermost `<svg>` timeline (**SMIL**) and set it to `<sec>`;
   pause every **`<video>`**, seeking one that was playing (or autoplays) to
   `<sec>` (modulo its length when it loops) and waiting for the seek;
7. **read every `currentTime` back**, then capture unscaled (the PNG must be
   exactly the viewport, one CSS pixel per image pixel) — **twice, 150 ms
   apart in real time**.

**The settle check.** The two captures of a kept frame must be byte-identical.
Anything still moving is content the freeze did not hold — an animated
GIF/APNG/WebP, an `<iframe>`'s own document, a worker's own timers — and the
frame **fails** (`FAIL: settle: unsettled motion in <frame> … painted there:
<element>`); it is not counted mid-animation and is never passed as frozen.
Fixture that must fail: `renderers/house-player/demo/fixtures/unsettled/` (an
animated GIF).

**What the freeze guarantees, exactly.** Every frame that passes is *still*:
it did not move after the freeze. Content that is a function of time (not of
`Math.random()` or load speed) is then the same frame on every run — measured
byte-identical across two runs of `held-motion`, all 12 frames. It is *at
`<sec>`* exactly for engine beats and every walked animation. JS time, SMIL and video run on one clock that starts at page load,
which is also when real playback starts — so they are at `<sec>` for content
that starts at load and keys on elapsed time; content that starts later (a
video that only begins when its scene shows, a loop started by a scene
change) is held at a deterministic frame that real playback may not show at
`<sec>`. Drive such motion from the engine (a `data-at` beat) when the frame
must be exact. Fixture that must pass, with a rAF canvas, SMIL, a `Date.now()`
counter, a `<video>` and a `.scene.on` `@keyframes` the engine does not track:
`renderers/house-player/demo/fixtures/held-motion/`.

The walk is what catches an animation the engine does not track: the `?at=`
seek alone leaves a `.scene.on`-triggered `@keyframes` bar near its entry
state (on `held-motion` at t = 2.8 s, `after-seek` reads a few to tens of ms —
7.8 to 67.1 ms measured, machine-dependent — against the `800.0ms` the schedule
implies); the walk holds
it at 800 ms. `--at 2.8 --dump-animation-times` on `held-motion` exits 0 and
goes red if the walk is removed — the demo alone cannot show that, because
`?at=` already positions every engine beat.

**Banned: `--virtual-time-budget`.** Measured in the RWA deck bake-off
(verified via `document.getAnimations()`): headless Chrome's
`--virtual-time-budget` fast-forwards `requestAnimationFrame` while leaving
every CSS animation at `currentTime: 0`, so every mid-animation screenshot
comes back as the scene-*entry* state — an empty or half-built frame that reads
exactly like a broken build. It is not a speed optimisation. The harness never
passes it; the comment at its launch-args site says why, and a guard refuses
any virtual-time flag there. Do not reintroduce it in any tool that takes a
frame which must show animation state.

Prove the freeze on any film with the pair:

```bash
node tools/render-check.mjs --at 6.0 --dump-animation-times <dir>              # every entry at its offset — exit 0
node tools/render-check.mjs --at 6.0 --no-freeze --dump-animation-times <dir>  # negative control — exit non-zero
```

`--no-freeze` holds the page's own clock at load and skips both the seek and
the walk — what a harness that trusts time-advancement instead of driving the
timeline reads: every `currentTime` at 0. If the second command ever exits 0,
the first one is measuring nothing.

## Pass A — LOAD (static + seeked assertions)

- scene count == `window.SCENE_DURATIONS`/`__player.durations` length ==
  `SCRIPT.md` numbered table rows (a film with no `SCRIPT.md` beside it gets a
  NOT CHECKED notice, never a pass)
- exactly one `SCENE_DURATIONS` assignment matches the pipeline regex
  `window\.SCENE_DURATIONS\s*=\s*\[[^\]]*\]\s*;?` in the file source
- no remote references (`http(s):`/`//` `src`/`href` fails); an on-disk
  relative reference is reported NOT CHECKED — inline it before hand-off
  (SKILL §2)
- every `@font-face` resolves (`document.fonts` status and `check()`)
- shim exposed: `__player{seek,play,pause}`, `__playerReady`, `__renderReady`;
  `__player.total` is SECONDS and equals the durations' sum
- zero page errors, zero console errors/warnings (only the runtime's own named
  FALLBACK warning is allowed — a compression or length-mismatch warning
  fails), zero requests off `file:`/`data:`/`blob:`
- **overflow, measured per element, frozen at 60 % of every scene**: walk the
  active scene, skip invisible elements (computed visibility/display and
  *effective* opacity through ancestors), and fail anything whose rect leaves
  the 1920×1080 stage by >2 px. This is the only check that catches an element
  clipped invisible by its own container (the exemplar found one at x −1273
  this way).

## Pass B — SHOTS + the front-load probe

- **Front-load probe**: for every scene, count visible inked elements (text,
  image/canvas/video, SVG shapes, a painted background or border) at 15 % and
  at 85 % of its slot; fail any scene with *less* at 85 %, naming it. The
  executable form of the terminal-hold pile-up check — a scene that plays its
  whole choreography up front and empties. Fixture that must fail:
  `renderers/house-player/demo/fixtures/front-loaded/`.
- **Mid-animation frames, measured**: every animation (read with each scene
  active in turn) is frozen at the midpoint of its active interval — and at
  25 %/75 % too while fewer than six distinct frames result. A frame counts as
  mid-animation only when, read back after the freeze, some finite animation's
  `currentTime` sits strictly inside its active interval, and the frame passed
  the settle check. **Fewer than six measured mid-animation frames fails the
  run.** Frame files are named for what
  they show: `<film>-t<sec>s-scene-NN-<target>-<pct>.png`.

## Pass C — DONE, both duration paths, and a proven-negative control

- **Injected path**: rewrite the file through the pipeline's *own* regex with
  0.6 s per scene, load, wait for `dataset.done === "1"`, assert
  `doneReason === "complete"` on the injected array.
- **Fallback path**: empty the declaration and shrink `window.FALLBACK` to
  0.6 s per scene; assert the same on the fallback array.
- **Negative control — proven negative.** A green check that cannot go red
  proves nothing. The harness breaks the build for real and asserts
  `doneReason === "error"`, and fails as could-not-check if its break changed
  nothing. The exemplar's lesson, which picks the break: renaming the `.scene`
  CSS class did nothing (the runtime selects on `[data-scene]`); renaming
  `data-scene` everywhere also rewrote the runtime's own selector string, so the
  page rebuilt itself perfectly. Only renaming the attribute **on the
  `<section>` tags alone** actually breaks it — which is what the harness does.
  If a control passes on the first try, suspect the control before celebrating.
- **Take-sanity gate (SKILL §8b)**: inject the authored array with scene 1
  blown up ~12×; the page must reach `doneReason=error` naming the scene in
  **under a second** (measured: 0.06 s hybrid adapter, 0.2 s hand-rolled
  driver, 9–32 ms on the house-player demo). Authored × 1.15 (a real pacing
  drift) must *not* fire — a gate that trips on honest pacing is a broken gate.

## Pass D — WALL CLOCK under the recorder (SKILL §8b)

The 12:14-against-7:06 incident (2026-08): a runaway TTS take (327.68 s wav
for a 25.71 s row) was measured and injected into `SCENE_DURATIONS`, and
every downstream guard trusted it — `doneReason=complete`, green gates, film
1.72× long. The injected array is a measurement, not ground truth. The harness
plays the film in real time twice — plain, then under the `record.mjs`
conditions (viewport 1920×1080, `recordVideo: { size: { width: 1920, height:
1080 } }`) — and asserts:

- **Sync telemetry.** Every `[sync]` boundary in `__explainer.syncMarks` is
  within max(2 % of elapsed timeline time, 0.1 s) on both runs (the 0.1 s floor
  is boundary detection at frame granularity), and the plain and recorded runs
  agree to the same bound — a gap between them is load-proportional slip and
  fails. Measured 2026-08-18: the hybrid exemplar holds every boundary within
  ±0.03 s over 7 min *while recording*; the hand-rolled animated driver within
  +0.26 s over 3 min (0.14 %). A drift that grows scene-over-scene means the
  live ticker is not tracking the wall clock (lag smoothing re-enabled, a
  frame-counting driver) or a scene cannot hold real time at 1080p — fix the
  ticker or cheapen the frame; never ship the drift.
- **Webm duration proof.** The recorded webm must decode (`ffmpeg -i <webm> -f
  null -`; Playwright webms carry no duration header) to at least
  sum(`SCENE_DURATIONS`) + 0.5 s tail − 0.25 s (the webm starts at the
  recorder's first frame, after first paint, not at load: measured 6.44–6.72 s
  for a 6.5 s film whose last paint is early) and at most sum + tail + 2 s
  recorder pad, +2 %.
  The webm lives in a temp dir and is deleted; the mp4 encode stays the
  human's.

## The contact sheet — the artifact the look-pass leaves

`--contact-sheet` freezes **every scene at its midpoint and every deck slide**
(every build stepped to its end, an endless animation held; the slide's JS
gets 1 s of held-clock time, SMIL and `<video>` are held at it; every tile
passes the same settle check) and tiles them, with ffmpeg, into ONE PNG per
artifact directory: `shots/contact.png`, 480×270 tiles, up to four across,
row-major. Beside it:

- `shots/INDEX.md` — one numbered table row per tile: artifact, surface
  (`scene N of M` / `slide N of M`), the time it was frozen at, what it should
  show, and the frame file; then the mid-animation frames (not tiles) with what
  was moving at capture. The gate counts the tile table's rows only.
- `shots/manifest.json` — `expected` (= scenes + slides), `tiles`, the grid,
  and, per artifact, the sha256 of its HTML and of every local file it
  references (`src`/`href`, CSS `url(...)`: the engine's `player.js`, fonts,
  images, video), folded into one `digest`. The scan is one level deep: a file
  referenced only from a referenced stylesheet or script (an `@font-face`
  `url()` inside an external `.css`) is not hashed and does not stale the sheet.

The sheet is a gate condition, not a courtesy. The bare gate run fails when:
`contact.png` is missing; its tile count, INDEX.md's rows or `manifest.expected`
differ from this run's measured `scenes + slides` (the sheet covers **every**
surface — the bake-off's reviewers each covered ~20 % and one lane went
unopened); a tile slot in the decoded sheet is empty; a tile frame is missing;
or the HTML or any local file it references changed since the sheet was made —
the failure names the file (a sheet of a different build is not a look at
this one). Frames and sheets are build outputs: generate, index,
git-ignore — never commit.

## Then LOOK

Open `shots/contact.png`, then the mid-animation frames, and read each frame
as a viewer:

- Is every line legible, or did something soft-wrap into a false second
  line?
- Did anything land on top of anything (a stamp over text passes every
  overflow check)?
- Does the mid-slot frame of each scene look *mid-motion*, or already
  finished?
- Is the bold moment visibly the loudest frame in the piece, and the
  arrest visibly stopped short?

Record what the looking found (even "nothing") in NOTES, **naming the sheet by
path**. A NOTES entry claiming a look-pass without a sheet path is not a pass,
and a gate that was not run is reported as NOT RUN — never implied. The class
of defect this catches — illegible, overlapping, pace-dead frames — survived
every automated assertion on every bake-off run.
