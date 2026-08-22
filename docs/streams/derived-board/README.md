---
stream: derived-board
repo: medici-finance/assay
serves: assay
status: active
priority: P1
track: platform
issues: []
---

# derived-board Stream

Make every lifecycle cell on a stream board — `todo` / `in-progress` / `implemented` /
`verified` / `done` — **derived by `statusgen` from things that already exist** (an open or
merged PR carrying a `Brief:` trailer, a `verifyrun` witness, an App approval at head, a
human ruling) and never hand-asserted again. The stream README's Briefs table becomes a
generated surface with exactly one writer, the same single-writer shape `STATUS.md` already
has. Brief frontmatter keeps only what cannot be derived: the authoring facts (`wave`,
`depends`, `effort`, `gate`, `risk`, …).

The change is a contract break for every adopter (a hand-edited surface becomes generated),
so it ships as **brief-v2 + statusgen/desk-tools `v1.0.0`**, with a `deskmigrate` migration
that `assay:upgrade-assay` applies, and a per-repo backfill that derives the historical
state from PR history and puts the drift in front of a human as a PR.

Why: the board lies in a specific, recurring way. A brief's deliverable merges and its row
still says `todo`, because the only thing that could flip it was a person remembering to
edit a prose table in a different file (measured 2026-08-22: `desk-containers/02` merged
with no README change; the class-1 sweep of 2026-08-21 hand-flipped dozens). Rule 30 of
`docs/brief-rules.md` already settled the principle for `verified`/`done` — a cell with a
machine-readable witness must not stay hand-asserted. This stream finishes the job for the
other three states, and removes the hand-edited table entirely. See [spec.md](spec.md).

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [brief-v2 spec: derived lifecycle, generated table, public re-stage of brief-rules/template](brief-01-brief-v2-spec.md) | 0 | M | todo | — | — |
| 02 | [`Brief:` trailer — the PR→brief link, required by `deskpr`, linted on main](brief-02-brief-trailer.md) | 0 | M | in-progress | — | — |
| 03 | [`statusgen reconcile` — derive lifecycle state from PRs + witnesses + approvals](brief-03-reconcile-derivation.md) | 1 | L | todo | — | — |
| 04 | [generated Briefs table + single-writer lint](brief-04-generated-table.md) | 2 | M | todo | — | — |
| 05 | [desk skills: reference the brief, never flip the cell (both copies)](brief-05-skill-updates.md) | 1 | S | todo | — | — |
| 06 | [v1.0.0 cut: migration op + file, paired-versions, same-tag pin lint](brief-06-v1-migration-and-cut.md) | 3 | M | todo | — | — |
| 07 | [per-repo rollout + historical backfill as a drift-report PR](brief-07-rollout-backfill.md) | 4 | L | todo | — | — |

## Critical path

`derived-board/01` (the spec — what a cell may claim and from what) → `03` (the derivation
engine) → `04` (the generated table that replaces the hand-edited one) → `06` (the v1.0.0
migration and cut) → `07` (rollout + backfill across the five stream-bearing repos).

**The head was verified before authoring.** The tempting head is 03 (the engine) — it is the
biggest piece and the one people want to see working. It is not the head: without 01 the
engine has no contract for what "derived" means when the inputs disagree (a hand-asserted
`implemented` with no merged PR; a merged PR with no trailer; an offline run), and without 02
it has no reliable PR→brief edge to derive from and would fall back to guessing from branch
names — the class of "confident answer from an instrument that never looked" the house's
three-state invariant exists to forbid. Both 01 and 02 are unblocked at the base SHA: the
spec files exist in both repos (`docs/brief-template.md`, `docs/brief-rules.md`; the public
copies are 139 and 40 lines behind the private ones, which 01 also fixes), `deskpr create`
already owns the PR-body write, and `statusgen --lint` already parses `schema: brief-v1` on
416 of 422 briefs across the two repos. Smallest unblocking move: land 01 and 02 in parallel.

Tempting-but-wrong first step: "just add a `status:` field to the frontmatter and have the
tool keep it current". That relocates the hand-asserted cell from the README into the brief
file and still needs a second actor to write it — the exact shape that produced the phantom
rows. The cell is derived or it is not trustworthy; there is no third option.

## Dependency waves

- **Wave 0** — `derived-board/01`, `derived-board/02` (independent; parallelizable).
- **Wave 1** — `derived-board/03` (depends on 01 + 02), `derived-board/05` (depends on 01 + 02;
  small, parallel with 03).
- **Wave 2** — `derived-board/04` (depends on 03).
- **Wave 3** — `derived-board/06` (depends on 04; human gate — the release cut is irreversible
  under immutable releases).
- **Wave 4** — `derived-board/07` (depends on 05 + 06; workflow-file pushes need the human).

Path: `01 → 03 → 04 → 06 → 07`, with `02 → 03` joining at wave 1 and `05` running beside 03.

## Shared conventions

- Every brief here is a toolkit brief: code under `statusgen/` and `tools/desk/`, docs under
  `docs/`, skills under `plugins/assay/skills/`. The private toolkit's copies of the skills
  and spec docs are consumers (`consumers:` lists them; re-stage is overlay, never mirror).
- Three-state instrument invariant (brief-rules §"Three-state instrument invariant") binds
  every derived cell: a cell is `todo`/`in-progress`/`implemented`/`verified`/`done` **only
  when the instrument looked**; when it could not (no network, API error, no trailer) the cell
  is `unknown` and the board says so. A derived negative is never a default.
- Feature branch + draft PR per brief; never commit `STATUS.md` on a branch; the PR body
  carries `Brief: derived-board/<NN>` from brief 02 onward (and should from day one — it is
  what the backfill in 07 will read).
- Public-tree content is self-contained: no private issue numbers, internal slugs, or
  withheld-doc paths in bodies, comments, or docs.
