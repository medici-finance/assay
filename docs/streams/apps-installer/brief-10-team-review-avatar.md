---
brief: assay:assay:apps-installer:10
title: "`deskavatar --tier team` emits a third tile for `<org>-review`, proofed at 20 px against read and act"
why: >-
  The team tier now installs three Apps (apps-installer/09), but the avatar generator still
  makes two tiles, so the review App would land with GitHub's default avatar — and a reviewer
  approval that looks like the act bot's comment at 20 px in a timeline defeats the point of a
  distinct approver. A third tile, proofed against the other two, keeps the three bots telling
  apart where humans actually see them.
wave: 0
depends: []
unblocks: ["apps-installer/06"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-10-15 by the-desk (house-the-desk-20261015T140050Z)
sources:
  - "apps-installer/09 — the team tier gains `<prefix>-review`, bound to the reviewer role."
  - "./design.md §5 avatars: Read + Act is inversion, not hue; every set is proofed at 20 px; deterministic."
  - "freshness-checked 2026-10-15 @ 3bf2bf99b (origin/main) — `specsFor(org, TierTeam, …)` in tools/desk/internal/avatar/generate.go returns two tiles (`-read`, `-act`, both GlyphStar); the family reviewer tile uses GlyphDisc."
consumers:
  - "tools/desk/internal/avatar/generate.go: follow-up apps-installer/10"
  - "tools/desk/cmd/deskavatar/main.go: follow-up apps-installer/10"
  - "docs/desk-tools/deskavatar.md: follow-up apps-installer/10"
  - "docs/streams/apps-installer/design.md: follow-up apps-installer/10"
version: 1
id: a45786fd-217c-4794-9e43-1391edcd0d95
---

# Brief 10 — Team-tier review avatar

## Context
files:
- `tools/desk/internal/avatar/generate.go` (team case of `specsFor`), its tests.
- `tools/desk/cmd/deskavatar/main.go` (the `--tier` help text: "team (read+act)").
- `docs/desk-tools/deskavatar.md` (`--tier team` line); `docs/streams/apps-installer/design.md` §5
  (the Read + Act bullet).

facts:
- Team tiles today (generate.go @ 3bf2bf99b): read = dark body `colBodyDark`, accent frame and
  accent GlyphStar; act = accent body, dark frame, dark-ink GlyphStar. Accent is
  `hslToRGB(seedHue(org), 0.72, 0.58)`.
- The proof (proof.go): every pair is rendered at 20 px; a pair fails when mean-colour ΔE (CIE76)
  is below 25 AND silhouette IoU is above 0.6. A failing set writes nothing and exits 5.
- The family reviewer glyph is GlyphDisc; GlyphRing is the fallback the family set already uses
  where a disc collides.
- Output files are `<org>-review.svg` / `.png` beside the existing two.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl.
- Stop at `implemented`.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Never lower the proof thresholds to make a set pass.

## Task
1. Add a third team tile, `App: org + "-review"`, in the adopter's hue, with the reviewer's
   family glyph (GlyphDisc). Choose body/frame/glyph colours so the set passes the proof against
   both read and act; if no colouring with GlyphDisc passes, use GlyphRing, and say which in the
   commit message.
2. The read and act tiles stay byte-identical (same org → same bytes as before this change).
3. Update the help text, deskavatar.md and design.md §5 to describe three team tiles.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go build -o /tmp/deskavatar ./cmd/deskavatar && rm -rf /tmp/av10 && /tmp/deskavatar --org example --tier team --out /tmp/av10 --sizes 200 && ls /tmp/av10 \| grep -cE '^example-(read\|act\|review)\.(svg\|png)$'` | 6 | check +flow |
| 2 | `cd tools/desk && go test ./internal/avatar/ -run 'TestTeam.*Proof' -count=1 -v` | exit 0 — the three-tile team set passes the 20 px proof for org logins `example`, `acme`, `a`, and a 39-character login | check |
| 3 | `cd tools/desk && go test ./internal/avatar/ -run 'TestTeamReadActUnchanged' -count=1 -v` | exit 0 — the read and act SVG bytes for org `example` equal golden files captured from origin/main before this change | check +dereference |
| 4 | `cd tools/desk && go test ./internal/avatar/ -run 'TestProofRejects' -count=1 -v` | exit 0 — a forced review tile identical to read (same glyph and colours) makes `Generate` return a `*ProofError` naming both Apps | check |

## Evidence
<!-- appended at implementation time: one row per Verify item -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
