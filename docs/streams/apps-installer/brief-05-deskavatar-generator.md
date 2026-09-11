---
brief: assay:assay:apps-installer:05
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
schema: brief-v2
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
version: 1
id: e4b0a613-87e6-4798-916c-5ba0a89c62b2
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
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd tools/desk && go build ./... && go test ./cmd/deskavatar/ ./internal/avatar/ -count=1` | exit 0 |
| 2 | check | `cd tools/desk && go build -o /tmp/deskavatar ./cmd/deskavatar && D=$(mktemp -d) && /tmp/deskavatar --org example-org --tier family --out $D && ls $D \| grep -cE -e '\.png$'` | 6 |
| 3 | check | `D=$(mktemp -d) && /tmp/deskavatar --org example-org --tier team --out $D && ls $D \| grep -cE -e 'example-org-read\.png' -e 'example-org-act\.png'` | 2 |
| 4 | check | `D1=$(mktemp -d); D2=$(mktemp -d); /tmp/deskavatar --org example-org --tier family --out $D1 && /tmp/deskavatar --org example-org --tier family --out $D2 && diff -rq $D1 $D2; echo $?` | no diff lines; exit 0 (deterministic) |
| 5 | check | `cd tools/desk && go test ./internal/avatar/ -run 'TestGolden20px' -count=1` | exit 0 |
| 6 | check +mutation | `cd tools/desk && go test ./internal/avatar/ -run 'TestProofFailsOnCollapsedPalette' -count=1 -v 2>&1 \| grep -cE -e 'reviewer.*worker' -e 'exit 5'` | ≥ 1 — with every role hue forced to #3366FF the proof reddens (the guarded distinguishability control) and names a colliding pair |
| 7 | check | `cd tools/desk && go test ./internal/avatar/ -run 'TestUploadedOmitsFinenessMark' -count=1` | exit 0 — the generated SVG contains no `A·999` text node |
| 8 | check | `cd tools/desk && ! go list -deps ./cmd/deskavatar 2>/dev/null \| grep -qxE 'C'` | exit 0 (no cgo dependency) |
| 9 | check | `grep -cE -e '20 px' -e 'ΔE' -e 'deterministic' docs/desk-tools/deskavatar.md` | ≥ 3 |
| 10 | check | `statusgen --root . --consumers --brief apps-installer/05` | exit 0 (routing claims corroborated against the diff) |

## Evidence
<!-- appended at implementation time -->

Implemented on `feat/apps-installer-05`. New: `tools/desk/cmd/deskavatar/`
(`main.go`), `tools/desk/internal/avatar/` (`color.go`, `glyph.go`, `compose.go`,
`generate.go`, `raster.go`, `proof.go` + tests and
`testdata/golden/{team,family}-20px.png`), `docs/desk-tools/deskavatar.md`, and
the `oksvg`/`rasterx` rasteriser dependency in `tools/desk/go.mod`.

Verify table run locally (all pass):

| # | Result |
|---|--------|
| 1 | `go build ./... && go test ./cmd/deskavatar/ ./internal/avatar/ -count=1` → exit 0 |
| 2 | family tier → 6 PNGs |
| 3 | team tier → `example-org-read.png`, `example-org-act.png` (2) |
| 4 | two family runs byte-identical (`diff -rq` exit 0) |
| 5 | `TestGolden20px` → exit 0 |
| 6 | `TestProofFailsOnCollapsedPalette` names `reviewer / worker` (grep count 1 ≥ 1) |
| 7 | `TestUploadedOmitsFinenessMark` → exit 0 (no `A·999` node) |
| 8 | `go list -deps ./cmd/deskavatar` has no `C` (no cgo) → exit 0 |
| 9 | docs grep `20 px`/`ΔE`/`deterministic` → 15 (≥ 3) |
| 10 | `statusgen --root . --consumers --brief apps-installer/05` → exit 0 |

Design note recorded in `docs/desk-tools/deskavatar.md`: the palette has hues
under the 20 px ΔE floor, so every colour-close pair is routed through a
silhouette-distinct glyph; reviewer (disc) and worker (ticket) are the deliberate
colour-only-separated pair the collapse test (row 6) targets. The golden strips
are the independent layer behind the proof metric.
### Non-implementer verifier run — 2026-09-11 sonnet-5-verifier (verify-desk dispatch), offline — **VERIFY: PASS — first non-implementer pass**

Pin: `medici-finance/assay` main `86c7d62c8081189147baf37424b602f907b139aa` (git rev-parse == gh api commits/main). Fresh clone. `gate: model`, `risk {all no}`, `irreversible: no`.

| # | Result |
|---|---|
| 1 | PASS — exit 0, both packages `ok` |
| 2 | **FAIL as literally written (stale count, not an implementation defect)** — got **7** PNGs, not 6. Root cause: PR #898 (`bcff2126`, "add board-writer tile to the family suite") landed AFTER this brief's own implementation commit and after the brief-v1→v2 flag day — a genuine, unrelated later change grew the family tier from 6 to 7 roles. The golden `family-20px.png` (row 5) was already regenerated to include board-writer; row 6's collapse test still correctly targets reviewer/worker. |
| 3 | PASS — 2 (read/act) |
| 4 | PASS — byte-identical across two runs (7 files each) |
| 5 | PASS — `TestGolden20px`, confirmed via source it's `bytes.Equal` against a committed golden, not a fuzzy metric |
| 6 | PASS — collapse test genuinely forces every family accent to one hex color via a real `forceAccent` option, confirmed the proof reddens naming reviewer/worker specifically |
| 7 | PASS — confirmed via source: asserts absence of the fineness mark AND a positive presence of the identity text node (not vacuous) |
| 8 | PASS — no cgo |
| 9 | PASS — docs grep count 15 (≥3) |
| 10 | **FAIL (known class)** — same tool-wide gap as `apps-installer/01`/`composability/00` (`assay#822`): both the short-id and colon-id forms of `--brief` fail, for the id-format break and the inherent diff-scoping-on-merged-main reasons respectively. Not a new defect. |

**Substance checks (traced code, extended beyond the implementer's own coverage):**
1. The 20px proof metric is real: `toLab` is a correct sRGB→CIELAB (D65) conversion; `deltaE` is genuine CIE76 Euclidean distance; `collides()` is a real two-condition AND (`ΔE<25 && IoU>0.6`), not a single-condition shortcut. Silhouette computed from the glyph layer alone, so the shared frame can't swamp it.
2. Determinism confirmed clean by source: zero hits for `time.Now`/`math/rand`/`crypto/rand`/`os.Getenv`/`runtime.GOOS`/`os.Hostname` across the whole package; fixed PNG compression level; no map-iteration-order dependency in output bytes.
3. **Own adversarial construction**: brute-forced two org logins whose `fnv32a(login) mod 360` collide (`org-58`/`org-94`, hue=81). Direct package call confirmed ΔE=0.00, IoU=1.00 between the two orgs' own tiles — **cross-org hue collisions are entirely outside the proof's scope by architecture** (the proof runs once per org's own `Generate()` call, never compares across orgs). Consistent with the brief's actual stated threat model ("distinguishable in a comment thread" = one org's own bound Apps in that org's own PR timeline) — not a violation of this brief's DoD, but a real scope boundary worth noting if a shared cross-org dashboard ever renders multiple orgs' avatars together (~1/360 collision chance per org pair).
4. House-value leak sweep: clean — only hits are the repo's own legitimate module import path and `"medici-finance"` used as one of several generic test-fixture org logins alongside `example-org`/`acme`/`fintechco`/etc.

`RISK-VALUE: NAMED, NOT DERIVED` — `ΔE < 25` and `IoU > 0.6` are asserted constants with no cited external perceptual-uniformity standard (typical CIE76 JND is single-digit; ΔE=25 is far more permissive, tuned for casual small-icon viewing). `IoU > 0.6` borrows a conventional CV threshold family (PASCAL VOC/COCO's 0.5) without citation. The doc's own text shows the thresholds and the 7-hue palette were co-tuned together — reasonable engineering, but a future palette change is validated against constants shaped around today's palette, not an independent source. The brief's own Review section already anticipates this ("the metric is a floor, taste is the reviewer's") and gates the residual risk to human eyeballing of the golden strips, so this is a documented, intentional NAMED-NOT-DERIVED, not a buried gap.

**VERIFY: PASS.** Rows 1,3-9 pass cleanly; row 2 fails only on a stale count from unrelated later work (real row count is 7, correctly reflected in the regenerated golden); row 10 fails only for the same tracked tool-wide gap as other briefs in this fan-out. `gate: model`, `risk: {all no}`, `irreversible: no` — flip-eligible. New non-blocking finding (cross-org hue collision, out of this brief's stated scope) recorded for awareness, not filed separately.

## Review
Gate: model. Reviewer records verdict + date in the stream README table. Reviewer also eyeballs the
golden 20 px strips in the PR: the metric is a floor, taste is the reviewer's.
