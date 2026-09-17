---
brief: assay:assay:fresh-views:06
title: reconcile ref-resolution across the brief-v2 id flag-day
why: >-
  statusgen reconcile derives each brief's lifecycle cell by exact-string-matching the immutable
  Brief: trailer in merged PR bodies. The brief-v2 flag-day rewrote every brief's id from the flat
  form (derived-board/02) to the hierarchical form (assay:assay:derived-board:02), but merged PR
  bodies are immutable and still carry the OLD flat trailer text — so the exact-string match is a
  100% false-negative and every brief in the tree derives cell=todo, including briefs known to be
  done (#1176). A derived view computed across a schema migration must RESOLVE the ref, not
  string-match a witness written before the migration.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1176]
schema: brief-v2
authored: 2026-09-16 by fresh-views scoping session
sources:
  - "docs/streams/fresh-views/spec.md §2, §3 (reconcile is a derived view whose input — an immutable PR trailer — predates a schema flag-day; it must resolve old->new, not exact-match)"
  - "medici-finance/assay#1176 — reconcile PR-witness matching 100% false-negative after the brief-v2 id flag-day (bb2079bd / PR #736); every brief derives cell=todo (176 briefs)"
  - "statusgen/lifecycle.go:120 (keys the PR-witness map by pr.BriefRef, unnormalized), statusgen/ghfetch.go:238-256 (singleBriefTrailer — no ref resolution), docs/streams/graph-repos.yaml (the alias registry that resolves the repo segment)"
  - "docs/streams/derived-board/spec.md (the derivation this corrects) — reconcile is homed in derived-board; this brief cross-references it and touches the matching logic only"
  - "freshness-checked 2026-09-16 @ e9fa19d3 (origin/main): #1176 confirmed OPEN; lifecycle.go + ghfetch.go present; all 177 briefs are schema: brief-v2"
exec-tier: strong
exec-tier-why: (c) a subtly wrong normalization re-introduces a false match (attributing a merged PR to the wrong brief) that survives a happy-path test and makes the board lie with authority
domain: complicated
version: 1
id: 0114dd3e-fa90-4f12-9e73-6e89c1bb6f87
consumers:
  # Authoring PR: code-path consumers routed to the deferred, self-targeting disposition (rule 6);
  # each flips to fixed-here in the implementation commit that edits the path.
  - "statusgen/lifecycle.go: follow-up fresh-views/06 (this brief; the PR-witness map keys on a NORMALIZED brief ref — flips to fixed-here when implemented)"
  - "statusgen/ghfetch.go: follow-up fresh-views/06 (this brief; trailer parse resolves the ref via graph-repos.yaml — flips to fixed-here when implemented)"
---

# Brief 06 — reconcile ref-resolution across the brief-v2 id flag-day

## Context
files:
- `statusgen/lifecycle.go` (line ~120) — the PR-witness map is keyed by `pr.BriefRef`, the literal unnormalized trailer text. Key it instead on a NORMALIZED brief ref so an old flat trailer (`derived-board/02`) and the current hierarchical id (`assay:assay:derived-board:02`) collapse to the same key.
- `statusgen/ghfetch.go` (lines ~238-256, `singleBriefTrailer`) — resolve the `Brief:` trailer ref to the canonical brief id (via `graph-repos.yaml` for the repo segment + the flat↔hierarchical mapping), rather than returning raw text.
- `docs/streams/graph-repos.yaml` — READ ONLY: the alias registry (`assay: medici-finance/assay`) that resolves the repo segment of a hierarchical id.
- `statusgen/lifecycle_test.go` (+ ghfetch tests) — tests.

facts:
- #1176 repro (merged main `0bf1166`, read-only token, medici-finance/assay itself): `go run . reconcile --root . --repo medici-finance/assay --json` yields `Counter({'todo': 176})` — every brief `todo`, `lookedAt=true`. Briefs independently known done (e.g. `derived-board/02`, merged PR #80; the reporting brief's own PR #199) derive `todo`.
- root cause: flag-day commit `bb2079bd` (PR #736, 2026-09-10) rewrote every brief's `brief:` id from flat (`derived-board/02`) to hierarchical (`assay:assay:derived-board:02`). Merged PR bodies are immutable, so every pre-flag-day PR still carries the OLD flat `Brief:` trailer. `lifecycle.go:120` keys by the literal trailer text; `ghfetch.go` `singleBriefTrailer` does no resolution — the exact-string match never hits.
- resolution rule: a brief's canonical id is the hierarchical form `<cell>:<repo>:<stream>:<NN>`; its flat alias is `<stream>/<NN>`. Both must map to one key. The repo segment resolves through `graph-repos.yaml` (`assay` ↔ `medici-finance/assay`). A trailer that resolves to no known brief stays unmatched and is reported as such (never silently attributed).
- single point of failure (rule 10): the ONE control is the normalization function. Second, independent layer: reconcile already reports `lookedAt` and per-brief provenance — the derivation must mark a brief whose cell it could NOT witness as `unknown`/unmatched WITH the reason, never as a quiet `todo`, so a normalization miss is visible as could-not-check rather than a false negative. (This is the three-state instrument invariant `derived-board` already holds; apply it here.)

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add a pure `normalizeBriefRef(raw, repos) -> canonical` in statusgen that maps both the flat (`<stream>/<NN>`) and hierarchical (`<cell>:<repo>:<stream>:<NN>`) forms to one canonical key, resolving the repo segment through `graph-repos.yaml`. Unresolvable refs return a typed "unmatched" result, never a guess.
2. Key the `lifecycle.go` PR-witness map on the normalized ref; make `ghfetch.go`'s `singleBriefTrailer` return the resolved id (or the raw text tagged unresolved).
3. Ensure a brief whose cell cannot be witnessed derives `unknown`/unmatched WITH a reason (three-state invariant), not `todo`.
4. Tests: an old flat trailer matches its hierarchical brief; a brief known done (fixture mirroring `derived-board/02` + PR #80's flat trailer) derives a non-`todo` cell; an unresolvable trailer stays unmatched and is reported, not attributed.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && go test ./...` | exit 0 | check:ci |
| 2 | `cd statusgen && go test ./... -run TestReconcileMatchesLegacyFlatTrailer -v` | exit 0; a flat `Brief: derived-board/02` trailer matches the hierarchical `assay:assay:derived-board:02` brief and derives a non-`todo` cell — the trailer-to-derived-cell path end to end | check:ci +mutation +flow |
| 3 | `cd statusgen && go test ./... -run TestNormalizeBriefRefUnresolved -v` | exit 0; a trailer resolving to no known brief returns unmatched (reported), never silently attributed | check:ci +dereference |
| 4 | `cd statusgen && go vet ./...` | exit 0 | check:ci |
| 5 | `statusgen --consumers --root .` | exit 0 — the diff-aware consumers gate corroborates every routing token against the branch diff | check:ci +dereference |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
