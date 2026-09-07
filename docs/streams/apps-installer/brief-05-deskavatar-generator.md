---
brief: apps-installer/05
title: "`deskavatar` — deterministic per-adopter App avatars with a 20 px legibility proof"
why: >-
  App avatars are seen at 20 px in a PR timeline, where only the tile's colour and one bold
  silhouette survive; a family of thin blue strokes inside the same blue octagon is six identical
  dots, and the reader has to read the login to know who spoke. The installer asks the person to
  drop an avatar per App anyway, so it should hand them a set that is on brand, distinguishable in a
  comment thread, tuned to their org, and identical every time it is regenerated.
wave: 0
depends: []
unblocks: ["apps-installer/06"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-05 by apps-installer authoring session
sources:
  - "./design.md §5 — the avatar rules (constant octagon; hue per role in the full suite, inversion for read/act; solid glyphs ≥ 7/64; identity field behind the glyph; no fineness mark on the uploaded file; 20 px pairwise proof; deterministic)."
  - "GitHub renders App avatars at 20 px in issue/PR timelines and ~40 px in review headers, cropped to a circle; the uploader accepts raster formats (PNG)."
  - "Geometry of the existing stamp family: 512×512 canvas, rounded-corner near-black ground (#0A0A0A), octagon body (#10141B) with an Electric-Blue frame (#3366FF, 10 units) and a thin inner punched edge; glyph centred. The hues for the full suite: reviewer #00CC66, worker #F08A2D, verifier #E6E9F2, desk #5C85FF, issue-loop #D46BFF, intake-loop #2DD4BF."
  - "freshness-checked 2026-09-05 @ 38e96f7 (origin/main) — no avatar generation exists in `tools/desk`; the GitLab provisioner (#354) ships static avatars, not generated ones."
exec-tier: any
consumers:
  - "tools/desk/go.mod: fixed-here (one pure-Go SVG rasteriser dependency; no cgo, no system library)"
  - "docs/desk-tools/deskavatar.md: fixed-here (new)"
---

# Brief 05 — `deskavatar`

## Context
files:
- `tools/desk/cmd/deskavatar/` (new): `main.go`, `compose.go` (SVG templates), `raster.go`,
  `proof.go` (20 px pairwise check), `testdata/golden/`.
- `tools/desk/internal/avatar/` (new, importable by `deskapps`): `Generate(org, tier, hueSeed)
  ([]Avatar, error)`.
- `tools/desk/go.mod`, `go.sum`.
- `docs/desk-tools/deskavatar.md` (new).

single-point-of-failure: the pairwise proof is the ONE control against an indistinguishable set.
The independent layer: the golden files — a committed 20 px PNG strip per tier that the test
compares byte-for-byte, so a palette or geometry regression that the metric happens to accept is
still a visible diff a reviewer must approve. Row 6 breaks the proof and shows the golden catches
the regression.

facts:
- Rasteriser: a pure-Go SVG renderer (for example `github.com/srwiley/oksvg` +
  `github.com/srwiley/rasterx`); the choice is the implementer's, the constraint is no cgo and no
  system dependency, so the desk-tools release matrix is unchanged.
- Output per App: `<app>.svg` (source) and `<app>.png` at 512 px; `--sizes 200,512,1000` optional.
- Hue for the two-App tier: HSL hue = `fnv32a(org-login) mod 360`, saturation and lightness fixed
  so the frame reads on #0A0A0A; the read tile uses the hue on the frame and glyph over the dark
  body; the act tile fills the body with the hue and draws the glyph in #0A0A0A.
- Identity field: initials (first letters of up to two hyphen- or space-separated words of the
  org login, upper-cased) in a serif at ~22/64, colour #2A3350 (low contrast on the dark body);
  when `--avatar <png>` is supplied, a two-tone quantized copy at 30% opacity instead.
- Proof metric at 20 px: render each tile to 20×20, compute mean tile colour in CIELAB and the
  binary silhouette (glyph pixels above a luminance threshold); a pair fails when ΔE < 25 AND
  silhouette IoU > 0.6. Failure names both Apps and exits 5.
- Determinism: no time, no randomness; `go test -count=2` produces identical bytes.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl.
- Stop at `implemented`.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- No network in `deskavatar`; the org avatar, when used, is passed in as a file.

## Task
1. `internal/avatar`: templates for the octagon frame, the seven glyphs (ring, check, hammer,
   disc, star, ticket, funnel), the identity field; `Generate` for `team` (read, act) and `family`
   (six roles).
2. `raster.go`: SVG → PNG at requested sizes.
3. `proof.go`: the 20 px metric; `deskavatar --org X --tier T --out DIR` runs it before writing
   and refuses (exit 5) on a failing pair.
4. Golden strips: `testdata/golden/team-20px.png`, `family-20px.png`; test compares bytes.
5. Docs: rules (design §5 verbatim), the metric, how to regenerate goldens (`-update`), and the
   sentence that the uploaded avatar omits the fineness mark by design.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/deskavatar/ ./internal/avatar/ -count=1` | exit 0 |
| 2 | `cd tools/desk && go build -o /tmp/deskavatar ./cmd/deskavatar && D=$(mktemp -d) && /tmp/deskavatar --org example-org --tier family --out $D && ls $D \| grep -cE -e '\.png$'` | 6 |
| 3 | `D=$(mktemp -d) && /tmp/deskavatar --org example-org --tier team --out $D && ls $D \| grep -cE -e 'example-org-read\.png' -e 'example-org-act\.png'` | 2 |
| 4 | `D1=$(mktemp -d); D2=$(mktemp -d); /tmp/deskavatar --org example-org --tier family --out $D1 && /tmp/deskavatar --org example-org --tier family --out $D2 && diff -rq $D1 $D2; echo $?` | no diff lines; exit 0 (deterministic) |
| 5 | `cd tools/desk && go test ./internal/avatar/ -run 'TestGolden20px' -count=1` | exit 0 |
| 6 | `cd tools/desk && go test ./internal/avatar/ -run 'TestProofFailsOnCollapsedPalette' -count=1 -v 2>&1 \| grep -cE -e 'reviewer.*worker' -e 'exit 5'` | ≥ 1 — with every role hue forced to #3366FF the proof names a colliding pair |
| 7 | `cd tools/desk && go test ./internal/avatar/ -run 'TestUploadedOmitsFinenessMark' -count=1` | exit 0 — the generated SVG contains no `A·999` text node |
| 8 | `cd tools/desk && ! go list -deps ./cmd/deskavatar 2>/dev/null \| grep -qxE 'C'` | exit 0 (no cgo dependency) |
| 9 | `grep -cE -e '20 px' -e 'ΔE' -e 'deterministic' docs/desk-tools/deskavatar.md` | ≥ 3 |
| 10 | `statusgen --root . --consumers --brief apps-installer/05` | exit 0 (routing claims corroborated against the diff) |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table. Reviewer also eyeballs the
golden 20 px strips in the PR: the metric is a floor, taste is the reviewer's.
