# fframes final export; HTML preview

Read when authoring a film's renderer, preparing a production handoff, or changing
renderers on an existing film. Default to fframes for the final picture and keep
`explainer.html` as the seekable preview. The final-export entrypoint is
`tools/native_export.py`; its manifest and invocation are documented below. The old browser recorder remains an explicit fallback,
not an automatic response to a failed native build.

## One film, two views

Use one scene-order/narration/timing source for both views. Derive native frame
state from `frame_index / fps`; do not advance mutable animation state each time
`render_frame` is called. Native rendering may evaluate frames concurrently or
out of order. This is different from counting browser rAF callbacks, which still
causes wall-clock drift and remains prohibited. HTML uses the existing house
player and its seeking, reduced-motion, duration-injection and done contracts.

Author native SVG/Skia scenes alongside the preview. Reuse scene data, assets,
font choices and motion functions where practical; do not independently edit two
narration tables or two sets of measured timings. For an existing HTML film, a
one-time extraction of layout can preserve text wrapping and positions, followed
by an explicit native port of its transforms. That is an adaptation technique,
not a general HTML-to-fframes converter. Browser screenshots inside an fframes
wrapper do not establish native rendering performance.

Keep the selected creative treatment. SVG supports useful vector choreography,
but CSS perspective, filters, layout and font fallback do not transfer verbatim.
Inspect the difficult shots early. Implement an equivalent effect, deliberately
revise the shot and re-review it, or record a renderer exception when fidelity
requires HTML. Do not silently flatten a camera move, remove a qualification or
simplify the visual argument to fit the backend. An exception changes the export
path, not the evidence, timing or review requirements.

## Audio is the clock

For new authorized narration, use the [voice-delivery default](voice-delivery.md).
Apply thought pauses and reading holds to the shared measured timeline before
building either view; adding silence only at mux time leaves motion and captions stale.

Reuse an authorized voice take and prepared music/foley mix when they exist.
Changing the renderer is not a reason to regenerate speech, select a new voice,
change speed or rebalance sound. Bind the exact source hashes. Changes to narration,
scene identity/order, takes, alignment or timing invalidate affected downstream
artifacts; rebuild the native executable when its schedule is compiled in.

Carry measured scene start/duration, lead-ins, inspection holds, tail and
phrase-level reveal cues into both views. Compare scene IDs, count and narration,
not merely array lengths. Apply the take-sanity bounds before native rendering as
well as in the HTML player. Longer speech does not justify stretching every gesture;
keep deliberately timed movement and use appropriate reading holds. A condition
must be stated and readable before the motion it authorizes begins.

A renderer comparison should change the picture path while holding sound constant.
The native final-export helper can copy the existing reference AAC and subtitle
tracks and verify all packet payload hashes and timestamps. For a new production,
prepare the mix and synchronized SRT/VTT once, then encode audio once while copying
the native picture. FFmpeg remains useful for this assembly; fframes itself uses
FFmpeg libraries. Replacing FFmpeg is not the purpose of this default.

Keep one caption-cue source and generate SRT/VTT from it. Preserve spoken limitations;
put captions below the picture in review players so paused controls cannot obscure
qualifications. Verify seeking, scene navigation, pause/resume, end behavior and
narrow-screen layout with the actual encoded files. A synchronized two-player
comparison should play the shared soundtrack once, not twice.

## Native parity and delivery checks

Retain the HTML render/seam gates. They validate the preview and cannot certify the
native export. Also inspect native scene entries, middles, resolves, representative
mid-motion frames and every seam at matching timestamps. Check labels, actual
font weights, missing glyphs, text wrapping, clipping, carried-object continuity,
conditional ordering and end holds. Include arrows, checkmarks and other symbols
in the font test: a webfont's Latin subset may rely on browser fallback absent in
the native process. Package licensed fonts/assets deterministically.

Keep a native contact sheet and index covering every scene, bound to native sources,
timeline and final-file hashes; inspect the encoded frames as well as the native
stills. Re-evaluate affected cuts after any timing or geometry change. Record actual
HTML/native differences; a similar midpoint cannot establish correct motion.
An unavailable native check remains NOT CHECKED. The native helper checks input
hashes, timeline identity and duration sanity, final frame count/size/fps/duration,
H.264 format, full audio/video decoding and the existing loudness band. It does not
establish visual parity, phrase alignment, caption editorial quality or human listening.
Those remain explicit review obligations, not implied by an exit code.

Keep dependencies pinned (Cargo.toml and Cargo.lock), source and licensed assets in
the product's video directory; keep executables, frame caches, audio and MP4s out of
Git. Select the platform backend deliberately: Metal was exercised on macOS; other
backends need their own validation. A missing GPU/backend must produce a clear
failure or a deliberate tested fallback, never an unlabelled browser export.

## Compare renderers

When comparing renderers, report render/encode, audio assembly, cold setup/build and QA
time separately; name resolution, frame count, backend, codec settings and repeat
count. Compare the same sound, scene schedule and representative complexity, inspect
quality differences, and keep theoretical bounds separate from measured timings.

## Export a prepared film

```sh
python3 "$VIDEO_SKILL/tools/native_export.py" --manifest <film>/native-export.json \
  --out <output>/film.mp4 --check
python3 "$VIDEO_SKILL/tools/native_export.py" --manifest <film>/native-export.json \
  --out <output>/film.mp4
```

`--check` proves input binding and timeline sanity only, not the native picture,
audio quality or editorial review. Existing output filenames are refused; choose
a new revision name. Failed native rendering or validation does not replace the
final output. Outputs are MP4, a same-stem VTT and a `.render.json` receipt. Keep
all three and the local binding file outside Git.

The author/build step writes the following version-1 JSON binding. Every path is
relative to the binding file (absolute paths also work locally); each `sha256` is
computed from the exact file bytes. Include the native project's source files,
Cargo.lock, fonts and assets under additional input keys so changing any one
invalidates the binding. A newly hashed record is not creative approval: inspect
and recheck the affected output before calling the revision reviewed.

```json
{
  "version": 1,
  "format": {"width": 1920, "height": 1080, "fps": 30, "tail_seconds": 0.4},
  "authored_estimates": [10.0, 12.0],
  "inputs": {
    "script": {"path": "SCRIPT.md", "sha256": "<file sha256>"},
    "preview": {"path": "explainer.html", "sha256": "<file sha256>"},
    "timeline": {"path": "production-manifest.json", "sha256": "<file sha256>"},
    "native": {"path": "native/target/release/film", "sha256": "<file sha256>"},
    "reference": {"path": "previous-finished.mp4", "sha256": "<file sha256>"},
    "source:main": {"path": "native/src/main.rs", "sha256": "<file sha256>"},
    "source:lock": {"path": "native/Cargo.lock", "sha256": "<file sha256>"}
  }
}
```

The timeline supplies `runtime` (sum of scene durations) and ordered `scenes`
with `id`, exact `narration`, `start` and `duration`. It may also carry voice-take
hashes, phrase alignment and reveal cues consumed by the native authoring code.
Starts are contiguous from zero; deliberate leads/holds belong in that schedule.
`authored_estimates` must match the bound preview's `FALLBACK` array. Existing
per-scene and total take-sanity ceilings still apply. The explicit export tail is
additional to `runtime`; expected frame count is `ceil((runtime + tail) * fps)`.

Two sound modes:

- **Renderer comparison / unchanged sound:** bind `reference` to the finished MP4
  containing exactly one AAC track and one subtitle track. Both tracks are copied;
  the exporter verifies all payload hashes and PTS/DTS/duration values, and writes
  a VTT derivative from that same subtitle track. It does not rerun the fframes
  audio mixer, normalize again or regenerate voice.
- **New production:** replace `reference` with `mix`, `srt` and `vtt` entries for
  the prepared sound mix and captions from one cue source. The exporter checks the
  paired cue text/times, preserves the picture, encodes AAC once and pads only the
  declared output tail. Prepare and review the mix upstream; export does not choose
  a voice, add music, generate captions or alter the mix's loudness.
