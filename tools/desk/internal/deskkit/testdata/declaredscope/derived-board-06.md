<!-- Regression excerpt: stream identifiers are synthetic; declaration syntax is retained. -->
## Context
files:
- `tools/desk/internal/deskkit/migrate.go` — new op `statusgen-regen: {verb: migrate,
  args: [brief-v1-to-v2]}`: runs the PINNED statusgen (resolved from `.assay-versions`,
  never `$PATH`) with `--root`; dry-run prints the statusgen dry-run; idempotent.
- `statusgen/migrate.go` (new) — verb `migrate brief-v1-to-v2 [--dry-run]`: rewrite
  `schema: brief-v1` → `brief-v2` on every brief; rewrite `brief: <stream>/<NN>` →
  `brief: <cell>:<repo>:<stream>:<NN>` using the tree's `graph-repos.yaml` (refuse, exit 5, if
  absent — the adopter writes the registry first; the release note says so); add `version: 1`
  where absent; mint `id:` (uuid v4) on every brief that lacks one (spec §8 — minted at
  migration, once); wrap each stream README's Briefs table in
  the markers and add `board: generated`; refuse (exit 5) if any README has no
  recognisable table. Prints a per-file plan under `--dry-run`.
- `migrations/0001-v0.28.0-to-v1.0.0-derived-board.md` (planned) — the first REAL migration (source umbrella v0.28.0, the latest at cut time — medici-finance/assay#453) — `apply:` =
  `statusgen-regen` + `ensure-line` in `docs/UPGRADING.txt`; body = the adopter-facing
  release note (what changes on their board, the trailer they must now write, the
  reconcile step they must add to their workflow — with the exact YAML).
- `statusgen/main.go` + brief-reading desk tools (`deskboard`, `deskpr`, `deskclaim`,
  `deskevidence`) — on `schema: brief-v2` in the tree with a binary `< v1.0.0`: exit 6
  "tree is brief-v2; this <tool> is vX; run assay:upgrade-assay".
- `statusgen` `--lint` — PROBLEM when `.assay-versions` artifact tags differ.
- `plugins/assay/.claude-plugin/plugin.json` 1.0.0; `paired-versions.yaml` plugin 1.0.0,
  its statusgen/desk-tools pin held at the last REAL umbrella release (one tag, one tree,
  `check-paired-versions.sh` green — no hand-invented `sha256`); the v1.0.0 re-pin + the
  `sha256` harvest is the cut-release skill's post-tag step, since a hash for an uncut tag
  is never typed by hand; `examples/adopter-scaffold/` migrated.
- `docs/release-notes/v1.0.0.md` (new) — same prose as the migration body.

facts:
- Umbrella tag is bare `vX.Y.Z`; release.yml is draft → upload → publish under immutable
  releases; the tag push is the human's.
- `deskmigrate` selects migrations by `[from,to]` span; `upgrade-assay` re-pins
  `.assay-versions` then runs them; dry-run first, always.
- The pinned-binary resolution inside the new op must use the SAME code path
  `desk-install` uses (sha256-verified), so a migration never runs an unverified tool.
- The refusal in brief-reading tools is version-gated by the build stamp (`-ldflags`),
  so a local unstamped build behaves as "latest".
