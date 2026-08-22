---
brief: derived-board/06
title: "v1.0.0 — deskmigrate statusgen-regen op, the v0.13.0→v1.0.0 migration, paired-versions bump, same-tag pin lint, brief-reading tools refuse v2 below v1"
why: >-
  A hand-edited surface becoming generated is the first contract-breaking change the
  tooling has shipped; it has to be a major version with a migration an adopter can
  dry-run, not a release note. The same cut closes the bundle gap — statusgen and
  desk-tools pinned at different tags reading one tree — with a lint and a refusal
  instead of a version matrix nobody maintains.
wave: 3
depends: ["derived-board/04"]
unblocks: ["derived-board/07"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
gate-why: >-
  The release cut is irreversible under immutable releases (v0.9.0 is a permanent
  tombstone for exactly this reason), and v1.0.0 is the version adopters will read as
  "stable". The human confirms the migration's dry-run output on a real adopter tree and
  the release-note prose, then pushes the tag; the brief prepares everything up to the tag.
issues: []
schema: brief-v1
authored: 2026-08-22 by derived-board scoping session
sources:
  - "docs/streams/derived-board/spec.md §6 (bundle versioning), §7 (migration)"
  - "tools/desk/internal/deskkit/migrate.go — migration format; only op today is ensure-line"
  - "tools/desk/cmd/deskmigrate/main.go, tools/desk/cmd/upgrade-assay/main.go, plugins/assay/skills/upgrade-assay/SKILL.md — the runner and the flow"
  - "plugins/assay/paired-versions.yaml (plugin 0.3.0 ↔ statusgen v0.13.0); examples/adopter-scaffold/.assay-versions"
  - "freshness-checked 2026-08-22 @ f78ea24 — latest release v0.13.0; the only migration in the tree is a test fixture"
exec-tier: strong
exec-tier-why: release mechanics under immutable releases; a wrong sequence burns a tag permanently
consumers:
  - "plugins/assay/.claude-plugin/plugin.json (version 1.0.0): fixed-here"
  - "plugins/assay/paired-versions.yaml: fixed-here"
  - "examples/adopter-scaffold/.assay-versions + its briefs (schema: brief-v2): fixed-here"
  - ".assay-versions of each consumer repo: follow-up derived-board/07"
---

# Brief 06 — v1.0.0 migration + cut preparation

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
- `migrations/0001-v0.13.0-to-v1.0.0-derived-board.md` (planned) — the first REAL migration — `apply:` =
  `statusgen-regen` + `ensure-line` in `docs/UPGRADING.txt`; body = the adopter-facing
  release note (what changes on their board, the trailer they must now write, the
  reconcile step they must add to their workflow — with the exact YAML).
- `statusgen/main.go` + brief-reading desk tools (`deskboard`, `deskpr`, `deskclaim`,
  `deskevidence`) — on `schema: brief-v2` in the tree with a binary `< v1.0.0`: exit 6
  "tree is brief-v2; this <tool> is vX; run assay:upgrade-assay".
- `statusgen` `--lint` — PROBLEM when `.assay-versions` artifact tags differ.
- `plugins/assay/.claude-plugin/plugin.json` 1.0.0; `paired-versions.yaml` plugin 1.0.0 ↔
  statusgen v1.0.0 (sha256 lines left as `<harvest-after-release>` placeholders the cut
  skill fills — never hand-invented); `examples/adopter-scaffold/` migrated.
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

## Ground rules
- NEVER git push / trigger workflows / push a tag. Commit on the feature branch only.
- Stop at `implemented`; the human runs the cut (cut-release skill) after review.
- No sha256 is ever typed by hand.

## Task
1. `statusgen migrate brief-v1-to-v2` with dry-run + idempotency tests on a fixture tree.
2. `statusgen-regen` op in deskkit with pinned-binary resolution + tests (dry-run writes
   nothing; unknown verb refused).
3. The real migration file + release note; `upgrade-assay` fixture updated to exercise it.
4. Refusals in brief-reading tools + the same-tag lint, each with a test.
5. Version bumps; adopter-scaffold migrated; `docs/UPGRADING.txt` convention documented.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go test . -run Migrate -count=1` | `ok` |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run 'StatusgenRegen' -count=1 && go test ./internal/deskkit/ -run 'Migrat' -count=1` | `ok` |
| 3 | `cd tools/desk && go run ./cmd/deskmigrate --from v0.13.0 --to v1.0.0 --root ../../examples/adopter-scaffold --dry-run; echo rc=$?` | `rc=0`; output lists `0001-v0.13.0-to-v1.0.0`; `git status --porcelain examples/ \| wc -l` → `0` (dry-run wrote nothing) |
| 4 | `rm -rf "$TMPDIR/adopt" && cp -r examples/adopter-scaffold "$TMPDIR/adopt" && cd tools/desk && go run ./cmd/deskmigrate --from v0.13.0 --to v1.0.0 --root "$TMPDIR/adopt" && go run ./cmd/deskmigrate --from v0.13.0 --to v1.0.0 --root "$TMPDIR/adopt"; echo rc=$?; grep -c 'schema: brief-v2' "$TMPDIR"/adopt/docs/streams/example-service/*.md; grep -c -E '^brief: [a-z0-9-]+:[a-z0-9-]+:example-service:0[12]$' "$TMPDIR"/adopt/docs/streams/example-service/*.md; grep -c '^version: 1' "$TMPDIR"/adopt/docs/streams/example-service/*.md` | `rc=0` twice (idempotent); every brief `1` on all three greps |
| 4b | `rm -rf "$TMPDIR/adopt2" && cp -r examples/adopter-scaffold "$TMPDIR/adopt2" && rm "$TMPDIR/adopt2/docs/streams/graph-repos.yaml" && cd tools/desk && go run ./cmd/deskmigrate --from v0.13.0 --to v1.0.0 --root "$TMPDIR/adopt2"; echo rc=$?` | `rc=5`; stderr names `graph-repos.yaml` (registry required before ids can be rewritten) |
| 5 | `printf 'statusgen v1.0.0 aaaa\ndesk-tools-linux-amd64 v0.13.0 bbbb\n' > "$TMPDIR/adopt/.assay-versions" && statusgen --root "$TMPDIR/adopt" --lint; echo rc=$?` | `rc=1`; output contains `artifact tags differ` |
| 6 | `cd tools/desk && go build -ldflags '-X main.version=v0.13.0' -o "$TMPDIR/deskboard-old" ./cmd/deskboard && "$TMPDIR/deskboard-old" --root "$TMPDIR/adopt"; echo rc=$?` | `rc=6`; stderr contains `tree is brief-v2` |
| 7 | `python3 -c "import json,yaml;p=json.load(open('plugins/assay/.claude-plugin/plugin.json'))['version'];y=yaml.safe_load(open('plugins/assay/paired-versions.yaml'));assert p==y['plugin']=='1.0.0' and y['statusgen']['tag']=='v1.0.0';print('ok')"` | `ok` |
| 8 | `! grep -n -E 'v1\.0\.0 [0-9a-f]{64}' plugins/assay/paired-versions.yaml` | exit 0 (no hand-typed hash for the unreleased tag) |
| 9 | `cd tools/desk && go run ./cmd/upgrade-assay --root "$TMPDIR/adopt" --to v1.0.0 --dry-run \| grep -c 'What changed'` | ≥ 1 (release-note prose surfaced before consent) |
| 10 | `statusgen --root . --lint` | exit 0 on this repo's own tree after migration |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: human (from frontmatter). The human records the ruling after running rows 3 and 9
on a real adopter checkout and reading the release note; then cuts `v1.0.0` via the
cut-release skill.
