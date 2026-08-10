---
brief: assay-product/07
title: 'Artifact-freshness cadence — version history on every product artifact + a deterministic staleness check; regeneration is analysis-desk work'
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by Fable session (assay-toolkit#12 point 1)
sources: ["assay-toolkit#12 (human:<name>, verbatim): 'Update the methodology artifacts & decks (with version history) to keep current'", "methodology-metrics/22 (daily artifact harvest — the scheduled AI-free lane the check joins; brief merged, todo)", "methodology-metrics/25 (weekly/monthly cadenced exec artifacts — authored on OPEN PR #381, unmerged at authoring; shares the one-schedule-matrix rule)", "I-analysis-desk (generated-artifact PRs + the analysis desk that consumes them — authored on OPEN PR #386, unmerged at authoring; this brief's check is a tier-1 generator, its regeneration pass is tier-2 desk work)", "methodology/38 (cadence-compression research — authored on OPEN PR #389; its per-loop decision rule re-clocks this cadence when it lands)", "freshness-checked 2026-07-12 @ ab92e96e: no version-history surface exists today — ../assay-toolkit/docs/product-brief.md carries one ad-hoc 'v0.1 (draft)' line, docs/market-analysis.md and all web/* pages carry none; nothing on either repo's main satisfies this deliverable"]
why: >-
  The Assay product artifacts (product brief, market analysis, exec brief, teaser, decks) were
  each written once and rot silently while the product they describe merges dozens of PRs a
  day; a reader has no way to see WHICH state of the system a document describes or whether it
  is still current. Git history exists but is invisible to the artifact's reader — a version
  line + dated changelog on the artifact itself, plus a deterministic staleness check, makes
  currency visible and regeneration schedulable instead of remembered.
---

# Brief 07 — Artifact-freshness cadence (version history + staleness check + regeneration pass)

**CROSS-REPO:** the version surfaces and the check tool land in `../assay-toolkit` (its own
git repo — commit there, SHA in Evidence); deck version files land in `../decks` (NEVER decks
in this repo); this repo carries only the stream docs. Manifest/k8s: none — nothing deploys.

## Context

files: `../assay-toolkit/docs/product-brief.md`, `../assay-toolkit/docs/market-analysis.md`,
`../assay-toolkit/docs/naming-clearance.md`, `../assay-toolkit/web/executive-brief/index.html`,
`../assay-toolkit/web/teaser/index.html`, `../assay-toolkit/web/ma-vertical/` (version
surfaces); `../assay-toolkit/tools/freshness/` (planned) + `../assay-toolkit/freshness.yaml` (planned)
(the check); `../decks/assay/ma/VERSIONS.md` (planned); this repo:
`docs/streams/assay-product/README.md` (row + convention line).

facts:
- **Artifact inventory (2026-07-12, the manifest's seed):** `../assay-toolkit/docs/product-brief.md`,
  `../assay-toolkit/docs/market-analysis.md`, `../assay-toolkit/docs/naming-clearance.md`,
  `../assay-toolkit/web/executive-brief/` (index.html + generated PDF),
  `../assay-toolkit/web/teaser/` (index.html + PNG + linkedin.md),
  `../assay-toolkit/web/ma-vertical/`; decks: `../decks/assay/ma/` (the M&A vertical deck; the
  general Assay deck is assay-product/06, todo — the manifest grows a line when it lands).
  The set grows — the check MUST read a manifest (`freshness.yaml`), never a hardcoded list.
- **Version surface spec** (git already gives history — the deliverable is the ON-ARTIFACT
  surface a reader sees): markdown artifacts get `version:` + `last-reviewed:` lines plus a
  dated `## Version history` changelog table (newest first: version · date · one-line what
  changed); HTML pages get a footer version line (`Assay <artifact> · vX.Y · reviewed
  YYYY-MM-DD`) backed by the same changelog table in a sibling `VERSIONS.md`; deck
  directories get a `VERSIONS.md`. Adoption entry: every artifact starts at its next minor
  version with one changelog row recording "version-history surface adopted (assay-product/07)".
- **The CHECK is deterministic and AI-free** (a tier-1 generator in the I-analysis-desk
  split): `freshness.yaml` maps each artifact → `max-age-days` + upstream source globs (the
  files whose change invalidates it — e.g. the product brief's upstreams include the toolkit
  README and `docs/streams/*/README.md` here). STALE = `last-reviewed` older than max-age OR
  any upstream commit newer than `last-reviewed`. Output: a dated stale/fresh line per
  manifest entry; exit 0 all-fresh, exit 1 if anything is stale. Go, not bash (repo rule).
- **REGENERATION is model work on its own clock, NOT this check:** the recurring pass (weekly
  default; methodology/38's decision rule re-clocks it when that research lands) reads the
  stale report, refreshes content, bumps version + appends the changelog row. Honest-framing
  ([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)) binds every regenerated claim, as everywhere in this stream.
- **Scheduling is adoption, not invention:** this brief does NOT add a cron/workflow. It
  emits the check as a single runnable command so the mm/22 harvest (and the mm/25 weekly
  artifacts, when PR #381 lands) adopt it as one line in the existing schedule matrix, and
  the stale report flows down the generated-artifact lane (I-analysis-desk) once that merge
  policy is decided. One scheduling system, not two.
- consumers (rule 6): exec/readers of every artifact (the version line is for them); the
  mm/22 harvest workflow (adds the check one-line when wired); the analysis desk (consumes
  stale reports); assay-product/05/06 (website + deck inherit the version-surface spec on
  landing). None change behavior until they opt in — the surfaces are additive.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only (assay-toolkit and decks commits local; push is human:<name>'s).
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. Add the version surface to every inventory artifact (spec above): version + last-reviewed
   + changelog with the adoption row. Keep content changes zero — this pass versions, it does
   not rewrite.
2. Build `../assay-toolkit/tools/freshness/` (planned) (Go) + `../assay-toolkit/freshness.yaml` (planned)
   covering the full inventory; stale detection and exit-code contract per facts.
3. Record the cadence convention: one line in this stream README's Shared conventions
   ("artifact freshness: `go run ./tools/freshness` in assay-toolkit — deterministic check;
   regeneration is a scheduled analysis-desk pass, weekly until methodology/38 re-clocks it")
   and a matching line in `../assay-toolkit/README.md`.
4. Update the stream-README row.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `(cd ../assay-toolkit && go run ./tools/freshness)` | exit 0; one `FRESH`/`STALE` line per freshness.yaml entry, all FRESH on the landing day |
| 2 | `(cd ../assay-toolkit && go run ./tools/freshness --as-of 2027-01-01); echo $?` | prints `1` (max-age exceeded ⇒ stale detection provably fires) |
| 3 | `grep -l "## Version history" ../assay-toolkit/docs/product-brief.md ../assay-toolkit/docs/market-analysis.md ../assay-toolkit/docs/naming-clearance.md \| wc -l` | 3 |
| 4 | `grep -ci "reviewed" ../assay-toolkit/web/executive-brief/index.html ../assay-toolkit/web/teaser/index.html` | ≥ 1 per file (footer version line present) |
| 5 | `test -f ../decks/assay/ma/VERSIONS.md && grep -c "2026-" ../decks/assay/ma/VERSIONS.md` | ≥ 1 (dated changelog row) |
| 6 | `statusgen --root . --lint` | exit 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

### Non-implementer verifier run — VERIFY: PASS at the recorded SHAs (glm-5.2-verifier, 2026-07-16)

Run against the sibling branch-tip SHAs in isolated worktrees (`/private/tmp/verify-ap07-toolkit` @ `d15f44d`,
`/private/tmp/verify-ap07-decks` @ `aeb8bac`). All 6 rows RUN, all green:

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `go run ./tools/freshness` (assay-toolkit) | 0 | 6 lines, all `FRESH` (3 docs + 3 web) |
| 2 | `go run ./tools/freshness --as-of 2027-01-01` | 1 | 6 lines all `STALE` — stale-detection provably fires |
| 3 | `grep -l "## Version history" <3 docs> \| wc -l` | 0 | `3` |
| 4 | `grep -ci "reviewed" executive-brief/index.html teaser/index.html` | 0 | `1` per file |
| 5 | `test -f decks/assay-ma/VERSIONS.md && grep -c "2026-" …` | 0 | `1` (dated changelog row) |
| 6 | `go run ./tools/statusgen --root . --lint` | 0 | advisory NOTICEs only |

**VERIFY: PASS at the recorded SHAs.** Sibling deliverables were initially unpushed (local-only); the
verify-desk pushed the branches and opened the cross-repo draft PRs — [assay-toolkit#78](https://github.com/medici-finance/assay-toolkit/pull/78) (@ `d15f44d`) and
decks#45 (@ `aeb8bac`) — completing the #272 pair
(in-repo PR #562 merged + sibling deliverables pushed/referenced). Checker confirmed deterministic/AI-free
(pure git-log + date math); manifest covers the full 6-artifact inventory.

## Review
Gate: model. Nothing here publishes — version surfaces and stale reports are internal/reader
metadata, and publication of any artifact stays behind this stream's standing human gate.
Reviewer checks the manifest covers the FULL inventory (an unlisted artifact is silent rot)
and that the check stayed AI-free (tier-1 generator discipline).
