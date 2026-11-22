# The seam gate — how to wire it

`tools/seam-gate.mjs` is a **numeric** verifier for scene boundaries. It seeks a
paused timeline to one frame either side of every declared cut, measures what is
actually moving from `getBoundingClientRect`, and fails the boundary if the
measurement disagrees with what you declared. It checks boundary continuity, not narrative or aesthetic quality.

It is vendored (Apache-2.0, from `heygen-com/hyperframes`; see the header of the
script and `LICENSE-hyperframes-Apache-2.0.txt` beside it). Zero npm
dependencies — node ≥ 22 and a local Chrome, which it finds itself in
`~/.cache/puppeteer` or `/Applications`. `CHROME_PATH` overrides.

## What it enforces, per seam

| Row | Rule |
|---|---|
| `type` | only `cut`, `hard-cut`, `match-cut`, and `morph` are supported; other types FAIL |
| `hard-cut-plan` | positive cut time, distinct exit/entry selectors and a non-empty editorial reason |
| `hard-cut-selectors` | both selected elements exist at every sample |
| `hard-cut-switch` | outgoing only before the cut; incoming only exactly at and after it |
| `ledger` | exit and entry vectors agree **in the plan** (same axis, same sign) |
| `exit-moving` / `entry-moving` | no settled exits, no from-rest entries — the boundary is crossed mid-motion |
| `exit-direction` / `entry-direction` | the measured sign matches what the ledger declared |
| `speed-match` (WARN) | entry velocity ≈ exit velocity, within 3× |
| `zero-overlap` | For `cut` rows, **one side visible per frame, never both**; match cuts need a separate overlap check |
| `z-sign-scan` | on a Z seam, the incoming scene's own entrances do not scale against the seam's sign |
| `carrier-*` | on a match-cut/morph, the carrier's rect is continuous across the cut (12 px centre / 5 % size) |

It checks nothing about content. A film can pass every seam and still be a
slideshow — which is why the render check exists beside it.

## Running it standalone (the wiring that is not in the usage line)

The script was written to drive a HyperFrames preview server. Two flags make it
run against a plain self-contained `explainer.html` instead, and the combination
is discoverable only by reading `ensureServer()`:

```bash
node skills/staging/tools/seam-gate.mjs verify \
  --ledger <film>/seams.json \
  --url http://127.0.0.1:1 \
  --comp-url "file://$PWD/<film>/explainer.html"
```

- `--url` is **required but unused**. Supplying any value skips the server-spawn
  branch entirely. `http://127.0.0.1:1` is a deliberately dead address: if the
  wiring ever regressed to actually fetching it, the run would fail loudly
  rather than quietly reach the network.
- `--comp-url` is what is actually navigated to. A `file://` URL is fine, and
  nothing is fetched.
- **Never pass `--project`.** That branch spawns `npx --yes hyperframes preview`,
  which installs from the network.
- Exit code 0 = gate passed. `--json` for machine output. `--fps 30` default.

## The shim the page must expose

The harness hard-requires three globals before it will run — `window.__player`
with `pause()` / `seek(t)`, plus `window.__playerReady` and
`window.__renderReady`. Current house-player supplies these; do not overwrite
its richer interface. For a compatible legacy runtime missing them, bridge the
master timeline at the end of its boot script:

```js
(function bridge() {
  var ex = window.__explainer;                 // whatever your runtime exposes
  if (!ex || !ex.master) { setTimeout(bridge, 40); return; }
  window.__player = {
    pause: function () { ex.master.pause(); },
    play:  function () { ex.master.play(); },
    seek:  function (t) { ex.master.pause(); ex.master.seek(t); },
    duration: function () { return ex.total; },
  };
  window.__playerReady = true;
  window.__renderReady = true;
})();
```

Purely additive — the recorder never touches it — and it belongs **in the
shipped file**, so the gate is re-runnable by anyone against the delivered
artifact rather than against a private build.

## Supported boundary types

The production contract requires zero overlap. The verifier checks it on `cut`
rows; match cuts need the separate check described below. Keep supported boundaries passing;
a dissolve would need appropriate verification before production delivery, not
an exemption or a false PASS. This is a tooling constraint, not a universal
judgment about cinematic transitions. Deliberate `hard-cut` rows are also supported: declare distinct exit/entry selectors and an editorial `reason`. The verifier checks the correct side is visible at both pre-cut samples, exactly at the cut, and both post-cut samples; missing selectors, overlaps and gaps fail. No velocity or continuity is claimed. Hard cuts require a positive cut time with an outgoing interval; a boundary at t=0 is an opening frame, not a two-sided seam, and fails this check. Unsupported types (including `dissolve`) FAIL rather than receiving only a carrier warning. Existing `cut` vector checks are unchanged. Stationary match cuts are supported: preserve
a meaningful carrier and declare the type rather than inventing motion.

## The declared-seam ledger

`seams.json` beside the film. One row per boundary; write it **before** the
master timeline, not after — the ledger is what forces the seam design.

```json
{
  "fps": 30,
  "seams": [
    { "id": "concede->seams", "cut": 19.6, "technique": "cut-the-curve LEFT",
      "exit":  { "selector": "#c-concede", "axis": "x", "dir": -1 },
      "entry": { "selector": "#c-seams",   "axis": "x", "dir": -1 } },

    { "id": "fork->anatomy", "cut": 141.2, "technique": "zoom-through (Z+) — register handoff",
      "exit":  { "selector": "#c-fork",    "axis": "z", "dir": 1 },
      "entry": { "selector": "#c-anatomy", "axis": "z", "dir": 1, "scanRoot": "#c-anatomy" } }
  ]
}
```

- `cut` — seconds on the master clock: the cumulative sum of the durations
  **the page is actually running**. If those are estimates, the ledger is in
  estimate time and says so.
- `axis` / `dir` — `x −1` leftward, `y −1` upward, `z +1` push (growing),
  `z −1` pull (shrinking).
- `selector` — the element that *carries* the seam motion (a per-scene
  `.carrier` wrapper is the easy answer), not the scene section, whose opacity
  the runtime owns.
- `type: "match-cut"` / `"morph"` swaps vector checks for carrier continuity.
  Declare `"carrier": {"out": "#outgoing .subject", "in": "#incoming .subject"}`;
  without it the tool warns that it has nothing to verify. Treat that warning
  as incomplete verification. The current tool checks zero-overlap only for
  `cut` rows; separately inspect/assert one visible scene on both sides of a
  match cut. Motion may start after a stationary matched boundary.

## When to opt a boundary in

Default: **all of them.** A film with 20 boundaries and 4 declared seams has
16 unaudited cuts, and the gate's whole value is that it is exhaustive.

Opt a boundary *out* only when it is genuinely not a cut — an intra-scene step,
or a boundary where the piece deliberately holds a single frame across the join
— and record the reason in the ledger's `_doc`. Choose Z or lateral vectors
for meaningful continuity and emphasis; no fixed film-wide quota applies. All
applicable declared vector, carrier and overlap checks still have to pass.

**Editing note.** Any change to a scene's first or last ~1 s — or to any
duration, which moves every later cut — invalidates that boundary's audit.
Re-derive the cut times and re-run. Say so in the ledger's own `_doc` field.

### Deliberate editorial hard cut

```json
{"id":"evidence->history","cut":24,"type":"hard-cut","reason":"Move from an evidence receipt to its retained event history","exit":{"selector":"#evidence"},"entry":{"selector":"#history"}}
```

Use this only for an actual discontinuous editorial cut. It does not certify a dissolve, camera match or motion continuity. Run `node tools/seam-gate-hard-cut-test.mjs` for the positive and negative fixtures.
