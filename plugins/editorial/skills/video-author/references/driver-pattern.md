# Seekable HTML player

## The contract

### `window.SCENE_DURATIONS` / `window.FALLBACK`

- Declare `window.SCENE_DURATIONS = [];` **exactly once**, empty, before
  loading `player.js`. The pipeline rewrites that one assignment (regex
  `window\.SCENE_DURATIONS\s*=\s*\[[^\]]*\]\s*;?`) with measured seconds
  before recording. A second assignment anywhere in the file — including
  inside a comment or a string, since the pipeline's regex cannot tell —
  silently steals the injection.
- Declare `window.FALLBACK = [<authored seconds per scene>];` alongside it.
  The engine uses `SCENE_DURATIONS` when it is non-empty (**injected**), and
  falls back to `FALLBACK` otherwise, always with a named `console.warn`.
- A `durations.length` that doesn't match the scene count warns too, once.

### `window.__player`

```js
window.__player = {
  scene: 1,      // 1-BASED index of the scene the clock is currently inside
  t: 0,          // current elapsed time, in SECONDS, from film start
  total: 6,      // the film's total length, in SECONDS (== sum(durations))
  durations: [2, 2, 2],  // per-scene seconds, in SECONDS
  seek(t) {},    // t is SECONDS: jump to an absolute time and PAUSE
  play() {},
  pause() {},
};
```

**Both units matter and are easy to get backwards**: the bake-off shipped
one track reporting `__player.total = 9` in *scenes* where nine reported it
in *seconds* — `total`, `t`, `durations` and `seek(t)`'s argument are always
SECONDS; `scene` is the only 1-based scene-count field on the object.

### Freeze hooks

Two incompatible freeze hooks shipped in the same bake-off
(`?still=T` and `?at=T`). This engine picks **`?at=<sec>`** and nothing else:

- `?scene=N` (1-based) starts playback at the first frame of scene N and
  plays.
- `?at=<sec>` seeks to an absolute time and **pauses** — deterministically.
  Every beat the engine tracks (`document.getAnimations()` entries, whether
  they originated as a CSS `@keyframes` animation or a WAAPI
  `element.animate()` call) is driven to the `currentTime` that time
  implies, then paused. The engine never relies on the browser's own live
  ticker to reproduce a frame — every visual frame, playing or paused, is a
  pure re-derivation from the master clock via `setVisual(t)`. That is what
  makes `verify.mjs --determinism` pass: two independent loads at the same
  `?at=` produce byte-identical screenshots.
- The freeze contract is exercised by `verify.mjs --determinism`, not by the
  bare `verify.mjs <html>` conformance run — that bare run checks player-ready,
  `total`-in-seconds, and the reduced-motion rule only. A film that honours
  `?still=` but not `?at=` returns two different frames for the same `?at=` and
  so fails `--determinism`; run that mode (both `--at` controls) to enforce the
  freeze contract.
- A `?at=` freeze also disarms the watchdog: a deliberately-held frame is not a
  hung film, so it never flips `doneReason` to `watchdog` however long the
  capture holds it. `verify.mjs --freeze-watchdog` is the guard for that.

Why this matters: the `--virtual-time-budget` flag in headless Chrome fast-forwards
`requestAnimationFrame` while leaving every CSS animation at
`currentTime: 0`, so a film whose visuals live in CSS `@keyframes` renders
every mid-animation screenshot as the scene-*entry* state unless the
harness explicitly drives `getAnimations()`. This engine's `?at=` handler is
what `presentation-skills/05`'s render harness is built on.

### `document.body.dataset`

- `dataset.done = "1"` when the timeline finishes.
- `dataset.doneReason = complete | error | watchdog`:
  - `complete` — reached the end of the authored timeline.
  - `error` — no scenes, a thrown boot, or the take-sanity gate below.
    `dataset.error` carries the message.
  - `watchdog` — `total + 0.5s tail + 5s` elapsed with no `complete`/`error`
    on a PLAYING film. A `?at=` freeze disarms the watchdog, so a frame held
    for inspection never reaches this reason no matter how long it is held.

### The take-sanity gate (video-author SKILL.md §8b)

On the **injected** path only (never on the fallback path — there is nothing
to compare a fallback against): if any scene's injected duration exceeds
`max(authored × 1.8, authored + 8)`, or the injected total exceeds
`authored total × 1.3`, the engine treats this as a **poisoned
measurement**, not a long film, and fails immediately —
`dataset.doneReason = "error"` naming the scene — rather than holding the
frame for however long the bad number says to. This check runs
synchronously at boot, before the first frame plays, so a poisoned load
reaches `error` in single-digit milliseconds, never after waiting out the
bad duration.

### Reduced motion

`player.js` injects its own
`@media (prefers-reduced-motion: reduce)` stylesheet rule into every page it
boots (a film cannot omit the degradation by forgetting it), and the JS side
independently snaps every beat straight to its final frame when
`matchMedia('(prefers-reduced-motion: reduce)').matches`. The film still
runs its full authored schedule and still reaches
`done=1 reason=complete` — only the motion is skipped, never the timing or
the completion signal.

### `window.__explainer`

```js
window.__explainer = {
  durations: [2, 2, 2],
  total: 6,
  scenes: 3,
  failure: null,        // the take-sanity/boot error message, or null
  syncMarks: [ { scene, timeline, wall, drift }, ... ],
};
```

`syncMarks` is populated at every scene boundary during real playback, and
the same line is logged to the console:
`[sync] scene N timeline=Xs wall=Ys drift=Zs` — the wall-clock discipline
telemetry SKILL.md §8b requires.

### Authoring a beat

```html
<section class="scene" data-scene="1">
  <div class="box" data-at="0.4" data-dur="1.2" data-beat="waapi"></div>
</section>
<section class="scene" data-scene="2">
  <div class="pulse" data-at="0.3" data-dur="1.4" data-beat="css"></div>
</section>
```

- `data-at` — scene-relative start time, in seconds.
- `data-dur` — beat duration, in seconds (default `0.6`).
- `data-beat="waapi"` (default) — the engine calls
  `el.animate(keyframes, {duration, fill:'forwards', easing:'linear'})` and
  drives its `currentTime`. Custom keyframes: `data-waapi='[...]'` (JSON).
- `data-beat="css"` — the element's own CSS declares the `@keyframes`
  animation with `animation-play-state: paused;` so the `CSSAnimation`
  exists (idle, at `currentTime: 0`) from page load; the engine finds it via
  `el.getAnimations()[0]` and drives it exactly like a WAAPI beat. **Never**
  use a `transition` on a class toggle as a beat's sole mechanism — a
  transition triggered only by adding a class is not discoverable/seekable
  the same way, and is the exact failure mode `verify.mjs --determinism`'s
  negative control (row 6) is built to catch.

Use `tools/render-check.mjs` to verify the preview at rest and during playback.
