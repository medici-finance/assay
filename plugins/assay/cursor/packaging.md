# Cursor packaging — coverage roster

Machine-readable accounting for the Cursor bundle, read by `tools/harnessgen cursor`.

This is the SOURCES.yaml coverage discipline (the same one `codex/packaging.md` runs)
applied to Cursor packaging: **every** `skills/*/SKILL.md` in the bundle tree must be
listed below as `packaged` or `excluded` with a written reason. A skill on disk that
appears in neither is a hard error (exit 2) — "every packaged skill is valid" means
nothing until the packaged set is known to be the whole set. A new skill cannot slip
into the bundle unaccounted-for.

Cursor is **lighter than Codex**: Cursor reads `SKILL.md` skills, `AGENTS.md`, and
`.cursor/rules/*.mdc` directly from the repo tree (HP/12 §2.10 `install-mechanism`), so
there is no per-harness plugin manifest — the packaging IS the instruction files. The
one generated artifact is `cursor/assay.mdc` (the Cursor-native resident-rules rule).
Cursor **also** reads the shared Codex `AGENTS.md` fragment (`../codex/AGENTS-assay.md`)
natively (HP/12 §2.1), so the adopt flow offers either resident-rules channel.

Dispositions:

- **packaged** — bare skill name on its own line. Shipped in the Cursor bundle (the same
  `./skills/` tree Claude Code and Codex use — the neutral core is what makes one tree
  servable to every harness). A skill HP/12 ruled `degrades`/`refuses` (headless) on
  Cursor still ships — the refusal/degradation text is method, carried in the binding
  file (`../references/cursor.md`); refusal is not exclusion.
- **excluded** — `<name> :: EXCLUDED: <reason>`. NOT shipped; needs a written reason
  citing HP/12's matrix or human:<name>'s 2026-08-26 ruling, never convenience. There are no
  exclusions today: every current skill either `runs`, `degrades`, or (headless,
  conditionally) `refuses` on Cursor — all of which SHIP (see `../references/cursor.md`).

`tools/harnessgen cursor --check`, exercised by `TestCursor*` in `tools/harnessgen`, enforces
this; those tests cover the coverage, skew, drift, and parse-error paths. `ci.yml`'s
module walk auto-discovers `tools/harnessgen` (build + vet); the suites themselves are
proven per the harness-portability/14 Verify table.

<!-- assay:cursor-packaging
adopt
ask-decision
author-brief
install
intake-desk
pdfingest
pr-review-desk
pr-shepherd
the-desk
upgrade-assay
verify-desk
worker-desk
# Excluded skills, if any, go here as:  <name> :: EXCLUDED: <reason citing HP/12 or human:<name>'s 2026-08-26 ruling>
# (none today — every current skill runs/degrades/refuses on Cursor, all of which ship)
-->
