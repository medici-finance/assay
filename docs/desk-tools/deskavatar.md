# `deskavatar` — deterministic per-adopter App avatars

`deskavatar` generates the on-brand avatar set an adopter uploads for their Assay
GitHub Apps. GitHub renders an App avatar at **20 px** in issue and PR timelines
(about 40 px in review headers), cropped to a circle, where only two signals
survive: the tile colour and one bold silhouette. A family of thin blue strokes
inside the same blue octagon is six identical dots — the reader has to read the
login to know who spoke. `deskavatar` hands the installer a set that is on brand,
distinguishable in a comment thread, tuned to the org, and **byte-identical every
time it is regenerated**.

It is offline (no network — an org avatar, when used, is passed in as a file) and
**deterministic**: no time, no randomness. `go test -count=2` produces identical
bytes.

```
deskavatar --org <login> --tier team|family --out <dir> [--sizes 200,512,1000] [--avatar org.png]
```

- `--tier team` — the two-App set installed first: `<org>-read` and `<org>-act`.
- `--tier family` — the full six-role suite: `reviewer`, `worker`, `verifier`,
  `desk`, `issue-loop`, `intake-loop`.
- Output per App: `<app>.svg` (source of truth) and `<app>.png` at 512 px. With
  `--sizes` each size is written as `<app>-<size>.png`.
- The 20 px proof runs **before** any file is written; a failing set writes
  nothing and exits **5**.

`--org` is validated against the GitHub login grammar
(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`) so it cannot smuggle a separator or `..` into
the output file stems, and each `--sizes` value is capped (`MaxRenderSize`, 4096
px) so a size cannot drive an unbounded allocation.

## The rules (design of record §5, verbatim)

> - **The octagon is constant.** It is the Assay stamp and the one thing that
>   marks every Assay App.
> - **At 20 px only two signals survive: tile colour and one bold silhouette.**
>   GitHub renders App avatars at 20 px in timelines and about 40 px in review
>   headers, cropped to a circle.
> - **Full suite: hue belongs to the role and colours the whole tile** (frame and
>   glyph). Six hues spaced for maximum separation on a near-black ground. Glyphs
>   are solid, about sixty percent of the canvas, no stroke thinner than 7 units
>   on a 64 grid, each with a distinct outline (ring, check, hammer, disc, star,
>   ticket, funnel) so the set survives monochrome viewing.
> - **Read + Act: inversion, not hue.** The adopter's hue (hashed from the org
>   login) colours both tiles; read is a dark tile with a light glyph, act a
>   filled tile with a dark glyph.
> - **Adopter identity sits behind the glyph** — initials or a quantized org
>   avatar as a low-contrast field, visible at 64 px and above, invisible at 20 px
>   where the role must win.
> - **The fineness mark leaves the uploaded avatar.** It stays on the large
>   masters; at 20 px it is unreadable and steals a third of the height.
> - **Every set is proofed at 20 px before it is written.** Pairwise colour
>   distance and silhouette overlap; a pair under threshold fails the run and
>   names the two roles.
> - **Deterministic.** Same org, same tier, same bytes.

### The uploaded avatar omits the fineness mark by design

The generated SVG carries the identity field (a low-contrast monogram behind the
glyph) but **no fineness mark** (`A·999`): it stays on the large masters and never
enters the uploaded file, because at 20 px it is unreadable and steals a third of
the height. Verify row 7 pins this — the generated SVG contains no `A·999` node.

## The 20 px proof (the metric)

The proof is the single control against an indistinguishable set, so it runs
before anything is written. Each tile is rendered to **20×20** and reduced to two
numbers:

- **Mean tile colour** in CIELAB. A pair is colour-indistinct when the CIE76
  **ΔE < 25**.
- **Binary glyph silhouette** — the glyph's pixels above a luminance threshold
  (a light glyph on a dark body reads as a solid mark; a dark glyph punched into a
  bright body, as on the `act` tile, reads as no mark). A pair is
  silhouette-indistinct when the intersection-over-union **IoU > 0.6**.

A pair fails **only when both** hold — indistinct in colour AND in silhouette —
so any pair separable by colour *or* by shape passes. On a failing pair the run
names both Apps and exits 5.

### Why the glyph assignment is not arbitrary

The role palette has hues that sit under the ΔE floor as 20 px mean tiles —
desk (`#5C85FF`) vs issue-loop (`#D46BFF`), and intake-loop (`#2DD4BF`) vs both
reviewer (`#00CC66`) and verifier (`#E6E9F2`). Every one of those colour-close
pairs is routed through a silhouette-distinct glyph (intake-loop the ring, desk
the check — near-zero IoU against anything), so the proof separates them by
shape. reviewer (disc) and worker (ticket) are the deliberate opposite: two big
central blobs that overlap in silhouette but are far apart in colour, so colour —
and only colour — tells them apart. That is the pair the collapse test targets.

## Golden strips — the independent layer

Alongside the metric, a committed **20 px PNG strip per tier**
(`internal/avatar/testdata/golden/{team,family}-20px.png`) is compared
byte-for-byte by `TestGolden20px`. The metric is a floor; the golden is the
backstop — a palette or geometry regression the metric happens to accept is still
a visible diff a reviewer must approve. Verify row 6 breaks the proof (every role
hue forced to one blue) and names the colliding pair; the golden catches
regressions the metric passes.

Regenerate the goldens after a deliberate visual change and have a reviewer eye
the diff:

```
cd tools/desk && go test ./internal/avatar/ -run TestGolden20px -update
```

## Rasteriser

SVG is rendered to PNG with a pure-Go rasteriser
(`github.com/srwiley/oksvg` + `github.com/srwiley/rasterx`) — **no cgo, no system
library** — so the desk-tools release matrix is unchanged (Verify row 8). The
identity monogram is drawn with the embedded Go font; both `image/png` encoding
and font rasterisation are deterministic, so a regenerated PNG is byte-identical.

## Library

`internal/avatar` exposes `Generate(org, tier, Options) ([]Avatar, error)` for
`deskapps` to call in-process during an install. The hue seed is derived from
the org login via FNV-1a; the caller passes only the login.
