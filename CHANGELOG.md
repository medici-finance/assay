# Changelog

All notable changes to this repository — the canonical, releasing home for the
shared Assay tools (statusgen, desk-tools, drainloop) and the `assay` plugin —
are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this repo versions the
whole umbrella with a plain `vX.Y.Z` tag (see `.github/workflows/release.yml`),
so one section covers every shipped tool at that version.

Every notable change adds one human-legible highlight line under `## Unreleased`
BEFORE it merges (the `changelog-check` CI leg enforces this; a genuinely
non-notable PR carries the `changelog:skip` label). At release time the release
workflow lifts `## Unreleased` into the published release notes and rolls it down
under a dated `## vX.Y.Z — <date>` heading — descriptive highlights, never a raw
commit list.

## Unreleased

### Added
- The changelog discipline is ACTIVE: the changelog-check PR leg gates merges and release.yml refuses an empty Unreleased section, lifting highlights into the release body (#266 activation, #269).
- `qualgen` mines the instruction-brittleness M1 family for real: instruction reference-validity and doc↔code staleness now render into `QUALITY.md`'s trend view behind a new `--instruction-globs` flag, replacing the family's placeholder — and an unconfigured run reports could-not-measure, never a silent zero (#271).
- The first real-history `qualgen` mine of this repo lands: `docs/quality/{metrics.jsonl,mine.json}` over the full 693-commit history, so CI renders live M1 numbers (copy/paste, churn, hotspots, bus-factor, coupling) plus the instruction reference-validity and doc↔code staleness trends into `QUALITY.md`, replacing the all-"not measured" placeholder board (#272).

### Fixed

### Changed

## v0.22.0 — 2026-08-31

### Added
- CI grows five control legs (#255): a forge-surface control sweep, a leak-sweep
  pattern sweep, per-plugin shell suites, a gating skillslint leg, and a
  QUALITY.md render check — each exercising a control that `go build`/`go vet`
  alone would leave un-run.
- A quality trend view: churn-vs-durable, hotspot and brittleness reporting land
  behind a single-writer `QUALITY.md` (quality/01–06: #245–#248, #252, #254).
- A per-loop pool-width knob for the desk loops (#226).
- A deterministic verdict runner (#242).
- Roster-from-deployment resolution (#256).
- A PR-body self-containment scan (#227).
- `inbox --flow` / `--walk` / `--html` views (#225, #233).
- A two-role superseded lane for `deskclose` (#232).
- B-SZZ inducing-commit tracing plus derived defect metrics land in `qualgen`
  (#261).
- Spec-routing §8 spec-lifecycle enforcement: a linter and an authoring-owed
  emitter (#267).
- The CHANGELOG discipline itself — a per-notable-PR `## Unreleased` highlight
  line, with the release-time roll and its CI enforcement staged (#266).

### Fixed
- The board archives cleanly: statusgen now resolves streams under
  `docs/archive/` as known depends / unblocks / affects targets (#259), so
  archiving a finished stream no longer reds valid references from still-active
  work.
- The verify queue stops lying: `verifyloop` defers `blocked-until` briefs,
  buckets online-lane and human-gated work out of DISPATCH, and reads
  qualifier-carrying `## Verify (…)` headings (#251, #253, #257).
- Board regeneration no longer races on concurrent pushes: regen-push is
  serialized (#221).
- A latent drift-registry test red is fixed (#262): `statusgen`'s
  `blockingIssueLabels` is registered as a declared exception, greening the
  release-only (desk-tools) test leg.
- The archive fallback extends to the markdown link/backtick check (#264), so
  references into `docs/archive/` stay green there too.
- `muhar -j 0` auto-parallelism is capped at 2 mutants in flight (#268) — the
  release test leg is memory-bounded by construction; every mutation still runs.

### Changed
- `pr-shepherd` is de-housed into the `assay` plugin so adopters get it too
  (#234).
- GitLab forge support closes out with a forge tier matrix (#222, #230, #231).

### Consumer action
- Pin `statusgen` at ≥ this release to lint boards that reference archived
  streams under `docs/archive/` (#259).
