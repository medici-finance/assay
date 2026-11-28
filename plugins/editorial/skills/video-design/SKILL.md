---
name: video-design
description: Review visual direction, storyboards, pacing, chart grammar and motion in an explainer.
---

# Video design

Decide what the viewer must notice in each scene. Check the hardest shot in a rough cut before committing to a renderer.

## Mission

- Give each video exactly one mission claim and sequence every scene as a step toward it; a scene that does not advance the claim is cut, not kept for interest.

## Script

- Trace every scene duration to a measured narration take; when no TTS run exists yet, wire the estimate in as the renderer's native SCENE_DURATIONS fallback and label it an estimate, never a measurement.

## Storyboard

- Keep the slides genuinely self-contained — fonts embedded, no relative script or asset references — because the pipeline copies the file to a work dir before recording and any external reference breaks there.

## Pacing & VO sync

- Author each scene at a natural pace and let the runtime scale it to the measured SCENE_DURATIONS; never hard-code timings that assume a narration length, because the voice-over is the clock and the visuals follow it.

## Chart scenes

- Label curves directly in text tokens (white or #AAAAAA), never in the series colour, and keep one axis per chart with a recessive grid.
- Treat the renderer as a clock, not a maths engine — carry ordinary scenes as declarative markup but build any chart, curve, or morph scene as an imperative timeline over a hand-written sampler.

## On-screen copy

- Write on-screen text in plain verbs with one job per element, and hold the subtitle to the same discipline as the on-screen text.

## Brand and review

Use the project's typography and marks. Develop the key explanatory image on the hardest scene first. Annotate the storyboard with the clause each visual answers, the cue that directs attention and the hold needed to read it. Watch once without sound and listen once without pictures. Review the combined cut for premature reveals, mistimed captions and labels that disappear before their comparisons finish.

## Renderer choice

Use a seekable HTML player for previews. Native frames, SVG, canvas and 3D can support the same timing contract. Choose a renderer that reproduces the hardest shot with the available assets, then compare its frame at a named time with the storyboard. See the sibling video-author skill for delivery and timing checks.

### Direct the renderer from the storyboard

| Picture path | What to author | What to verify |
|---|---|---|
| Bundled HTML player | Inline the classic script. Give it one empty SCENE_DURATIONS assignment and authored fallback durations. Spend extra measured time on a reading hold. | Seek into a moving scene, pause, and capture it twice. Check that both frames agree and that reduced motion leaves the claim readable. |
| GSAP in an existing project | Build an independent timeline per scene. Drive a plotted curve with a sampler, and reveal dashed guides without replacing their dash patterns. | Test callbacks while seeking as well as during normal playback. A counter updated only during live playback can freeze in an exported frame. |
| Manim | Use a matched coordinate system for mathematical objects and their labels. Allocate the measured narration time across the transformations and pauses. | Inspect the rendered video for label collisions. Check total frames and the shared audio clock before joining scenes. |
| Motion Canvas | Express related positions, labels and chart values as functions of the same signal. Bundle fonts with the render entry. | Wait for font loading, inspect the final text wraps, and match frame rates when intercutting with another renderer. |
| Native fframes | Derive every frame from its time index. Share scene IDs, reveal cues and measured durations with the HTML preview. | Compare the difficult shot in both renders. Check the encoded picture, caption timing and bound audio with the video-author exporter. |

When a render fails, identify the first failing layer. Confirm the script row and storyboard cue, then the scene at a named time, then the measured duration and completion signal, and finally the encoded frame and soundtrack. Record the failing scene, time, expected result and observation so the next editor can reproduce it.
