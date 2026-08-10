---
brief: desk-apps/01
title: Desk-App brand icons — octagonal assay stamps + canonical assay-mark
why: >-
  The desk Apps need distinct avatars that read as a family and carry the Assay brand (the
  methodology product). The current reviewer-logo is a plumb bob — the Plumb product's motif —
  so it brands the reviewer desk as the wrong product. Drawing the assay-mark motif (Assay's one
  allowed differentiator, never rendered until now) onto an octagonal stamp gives every desk App
  an tamper-evident-looking "this was checked" identity that is visibly Assay, not Medici-the-product
  and not Plumb.
wave: 0
depends: []
unblocks: ["desk-apps/02"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by glm-5.2 session (human:<name>'s desk-apps direction, [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md))
sources: ["INTAKE [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md) (per-role GitHub Apps — incl. desk-app)", "INTAKE I-brand-system (three-brand system: Assay = Medici system + hallmark/assay-mark motif)", "docs/brand-guide.md (Medici system — colors, iconography §6)", "docs/brand/reviewer-logo.svg + docs/brand/README.md (the mis-carried plumb-bob motif + stale medici-stuff[bot] name)", "methodology/brief-22 (medici-stuff→reviewer-app rename commit 6433e178 missed docs/brand/README.md)", "agent repo-mapping 2026-07-12: assay-toolkit has no brand/ dir; no assay-mark asset exists in any repo"]
---

# Brief 01 — Desk-App brand icons (octagonal assay stamps)

**CROSS-REPO:** the icon family lands in `medici-finance/assay-toolkit` (`../assay-toolkit`) — the
Assay brand home per INTAKE I-brand-system. This repo's PR carries this brief + the stream row +
the parent-repo hygiene edit (Task 3). Record the assay-toolkit commit SHA in Evidence.

## Context
files: `../assay-toolkit/docs/brand/` (new dir — mirrors `oit/docs/brand/`);
deliverables: `assay-mark.svg` (canonical stamp, no role glyph) + six App-avatar SVGs
(`reviewer-app.svg`, `verifier-app.svg`, `worker-app.svg`, `desk-app.svg`,
`issue-loop-app.svg`, `intake-loop-app.svg`), PNG renders at 200/512/1000, `README.md`.
out-of-repo files: none (the assets land in the assay-toolkit repo, a sibling git repo, not a
`~/.claude/**` live surface — work in an assay-toolkit worktree, commit there).
facts:
- **Assay brand (INTAKE I-brand-system):** shares the FULL Medici system (`../oit/docs/brand-guide.md` —
  true-black canvas, Electric Blue `#3366FF` leads, Gold `#D4A843` sparingly, restrained tone);
  its ONE differentiator = the **hallmark/assay-mark stamp motif** ("the tamper-evident review gate
  is an assay mark"). Gold stays sparing; blue leads.
- **human:<name>'s form pick (2026-07-12):** the shared stamp FORM = **octagonal** (eight-sided QA/inspector
  stamp). Chosen over rectangle/seal, and explicitly NOT a shield — `../oit/docs/brand-guide.md` §6.2
  reserves the shield for Medici's "can't-be-liquidated" guarantee; an Assay shield would collide.
- **Per-role GLYPH inside the stamp:** reviewer = ✓ checkmark; verifier = shield-check / evidence
  (document-with-seal) mark; worker = hammer; **desk = compass-star** (4-point — the coordinator
  that orients the loops); **issue-loop = ticket** (the issues lane); **intake-loop = funnel**
  (ideas flow in the front door — the intake lane). Same octagonal frame + fineness mark; the glyph
  differentiates. The family covers all six agent-actor loops (decks/loops/deck.md); METRICS is
  zero-AI and RETRO is human, so neither gets an App.
- **Canvas + palette:** 512×512 viewBox, near-black canvas (`#050505`, the Assay web `--bg`),
  octagonal frame in Electric Blue (`#3366FF`, `#5C85FF` highlight, `#1A4DCC` depth), role glyph in
  Electric Blue or White, a Gold (`#D4A843`) assay fineness mark ("A·999" — the assay-mark pun).
  Functional green (`#00CC66`) only where a stamp reads "passed/approved" (the reviewer's check).
  Centered with margin — GitHub crops App avatars to a circle (per the existing reviewer-logo README).
- **Avatar category, not UI icon:** `brand-guide.md` §6 (no gradients / monochrome / ≤20px) governs
  *UI icons*. Desk-App avatars are **brand avatars** (like the Medici logo, `reviewer-logo.svg`,
  `jangchi-bot.svg`) — multi-color on-palette is correct for this category.
- **Reviewer-logo is mis-branded (the bug this brief fixes):** `docs/brand/reviewer-logo.svg` is a
  plumb bob (gold ferrule + blue weight + green tick) = the **Plumb** motif, per its own README. The
  reviewer desk is methodology → Assay. Brief 01 reworks it onto the octagonal assay stamp.

### Consumers of `reviewer-logo*` (shared-value change — rule 6)
Enumeration (`grep -rn reviewer-logo --include=*.md --include=*.go ... .`, 2026-07-12):
- docs/brand/README.md (lines 6, 20–24) — **fixed in this brief** (Task 3: re-describe motif,
  fix stale `medici-stuff[bot]` → `reviewer-app[bot]`, point to assay-toolkit canonical home).
- `.claude/settings.local.json` (lines 954–955, the `rsvg-convert` allowlist entries) — **no change
  needed** (the rsvg commands still work; the parent-repo `reviewer-logo.svg` stays at its path,
  reworked in place).
- No code/Go/TS references the SVG file. Contained.
- Stale-name sibling: `docs/brand/README.md:8` still says `medici-stuff[bot]`; methodology/brief-22
  reconciled the rename (`6433e178`) but its grep only checked `.claude/skills/`. **Fixed here.**

## Ground rules
- NEVER git push / trigger workflows. assay-toolkit commits are LOCAL — pushing that repo is human:<name>'s
  (memory `no-auto-commit`). Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Hand-author the SVG family in `../assay-toolkit/docs/brand/` (512×512 viewBox, circle-safe):
   - `assay-mark.svg` — the octagonal stamp alone (frame + Gold "A·999" fineness mark), no role
     glyph. The canonical Assay differentiator, reusable as a stamp/seal anywhere.
   - `reviewer-app.svg` — octagonal stamp + ✓ checkmark (Electric-Blue/White) + a green
     (`#00CC66`) "lands approved" accent. The reworked reviewer App avatar.
   - `verifier-app.svg` — octagonal stamp + evidence glyph (shield-check or document-with-seal).
   - `worker-app.svg` — octagonal stamp + hammer/commit glyph.
   - `desk-app.svg` — octagonal stamp + compass-star glyph.
   - `issue-loop-app.svg` — octagonal stamp + ticket glyph (the issues lane).
   - `intake-loop-app.svg` — octagonal stamp + funnel glyph (the intake lane).
   - All seven share the frame construction, canvas, and fineness mark — a viewer reads "family" first.
2. Render PNGs from each SVG at 200/512/1000 with `rsvg-convert`
   (`rsvg-convert -w 512 -h 512 <src>.svg -o <src>-512.png`, etc. — available at
   `/opt/homebrew/bin/rsvg-convert`).
3. Write `../assay-toolkit/docs/brand/README.md` (mirror docs/brand/README.md):
   document each asset, its role, the assay-mark motif rationale, and the re-render command.
4. **Parent-repo hygiene (same PR):**
   - Rework `docs/brand/reviewer-logo.svg` in place onto the octagonal reviewer stamp (matches
     `reviewer-app.svg`) so the existing path + PNG renders + the settings rsvg allowlist keep working.
   - Re-render `docs/brand/reviewer-logo-512.png` + `reviewer-logo-200.png` from the reworked SVG.
   - Update docs/brand/README.md: fix `medici-stuff[bot]` → `reviewer-app[bot]`; re-describe
     the motif (assay stamp, not plumb bob); note the canonical home is `assay-toolkit/docs/brand/`
     with the parent file as the App-upload mirror.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `for f in assay-mark reviewer-app verifier-app worker-app desk-app issue-loop-app intake-loop-app; do test -f ../assay-toolkit/docs/brand/$f.svg || echo MISSING $f; done` | prints nothing (all seven SVGs exist) |
| 2 | `grep -c 'viewBox="0 0 512 512"' ../assay-toolkit/docs/brand/*.svg` | ≥7 |
| 3 | `grep -ciE '#3366FF|#5C85FF|#1A4DCC' ../assay-toolkit/docs/brand/reviewer-app.svg` | ≥1 (Electric-Blue family present) |
| 4 | `grep -ciE '#D4A843|#E8C96A' ../assay-toolkit/docs/brand/assay-mark.svg` | ≥1 (Gold assay accent present) |
| 5 | `grep -ciE 'plumb|ferrule' ../assay-toolkit/docs/brand/reviewer-app.svg` | 0 (plumb-bob motif purged) |
| 6 | `for n in 200 512 1000; do for f in assay-mark reviewer-app verifier-app worker-app desk-app issue-loop-app intake-loop-app; do test -s ../assay-toolkit/docs/brand/$f-$n.png || echo MISSING $f-$n; done; done` | prints nothing (all PNGs non-empty) |
| 7 | `test -f docs/brand/README.md && ! grep -q 'medici-stuff' docs/brand/README.md` | exit 0 (stale bot name fixed in parent-repo hygiene). Guarded by `test -f docs/brand/README.md &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |
| 8 | `grep -ci 'octagonal\|assay.stamp\|assay.mark' docs/brand/README.md` | ≥1 (motif re-described) |
| 9 | **Flow (shared-value, rule 6):** `grep -rn reviewer-logo docs/ .claude/settings.local.json` and confirm every hit still resolves (reworked SVG at the same path; rsvg allowlist unchanged) | every consumer resolves; none dangling |
| 10 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output, date, runner). Include the assay-toolkit commit SHA. -->

**Verifier run (glm-5.2, non-implementer) — parent origin/main `421d7cde`; sibling assay-toolkit `d15f44d` on branch `fix/assay-product-07-artifact-freshness`. 2026-07-16.**

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|------------|------|--------|
| 1 | 7 brand SVGs exist (assay-mark + reviewer/verifier/worker/desk/issue-loop/intake-loop-app) | 0 | none MISSING | 2026-07-16 | glm-5.2-verifier |
| 2 | `viewBox="0 0 512 512"` count across SVGs | 0 | 7 (≥7) | 2026-07-16 | glm-5.2-verifier |
| 3 | Electric-Blue family (#3366FF/#5C85FF/#1A4DCC) in reviewer-app.svg | 0 | 2 (≥1) | 2026-07-16 | glm-5.2-verifier |
| 4 | Gold assay accent (#D4A843/#E8C96A) in assay-mark.svg | 0 | 3 (≥1) | 2026-07-16 | glm-5.2-verifier |
| 5 | plumb/ferrule in reviewer-app.svg | 1 | 0 matches (motif purged) | 2026-07-16 | glm-5.2-verifier |
| 6 | 21 PNGs (7 icons × {200,512,1000}) non-empty | 0 | none empty | 2026-07-16 | glm-5.2-verifier |
| 7 | "medici-stuff" in docs/brand/README.md | 1 | 0 matches (stale bot name fixed) | 2026-07-16 | glm-5.2-verifier |
| 8 | octagonal/assay.stamp/assay.mark in README | 0 | 4 (≥1, motif re-described) | 2026-07-16 | glm-5.2-verifier |
| 9 | reviewer-logo consumers resolve on main | 0 | no dangling consumers; SVGs exist at paths; plumb purged | 2026-07-16 | glm-5.2-verifier |
| 10 | `statusgen --lint` | 0 | NOTICEs only; none reference desk-apps/01 | 2026-07-16 | glm-5.2-verifier |

**VERIFY: PASS** (10/10; all four review criteria met). Sibling assay-toolkit assets live on a feature branch (not its main) — per the brief's ground rule that repo's push is human:<name>'s; does not affect the in-repo Verify.

## Review
Gate: model. Reviewer confirms: (a) the seven icons read as one octagonal-stamp family, circle-safe;
(b) the Assay motif + Medici palette are honored (blue leads, gold sparing); (c) the reviewer-logo
consumers all still resolve after the rework; (d) the plumb-bob/Plumb motif is fully gone.
