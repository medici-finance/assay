# Codex packaging — coverage roster

Machine-readable accounting for the Codex plugin bundle
(`plugins/assay/.codex-plugin/plugin.json`), read by `tools/harnessgen codex`.

This is the SOURCES.yaml coverage discipline ported to Codex packaging: **every**
`skills/*/SKILL.md` in the bundle tree must be listed below as `packaged` or
`excluded` with a written reason. A skill on disk that appears in neither is a hard
error (exit 2) — "every packaged skill is valid" means nothing until the packaged
set is known to be the whole set. A new skill cannot slip into the bundle
unaccounted-for.

Dispositions:

- **packaged** — bare skill name on its own line. Shipped in the Codex bundle. The
  manifest ships the whole `./skills/` tree via a directory pointer (HP/01 §3.2,
  superpowers 6.2.0 shape), so a `packaged` skill must be present under `skills/`
  on disk. A skill HP/03 ruled `refuses`/`degrades` on Codex still ships — the
  refusal/degradation text is method, carried in the binding file
  (`../references/codex.md`); refusal is not exclusion.
- **excluded** — `<name> :: EXCLUDED: <reason>`. NOT shipped; needs a written
  reason citing HP/03's ruling or HP/01's matrix, never convenience. Because the
  manifest ships the whole `./skills/` directory, an excluded skill must ALSO be
  absent from `skills/` on disk — otherwise the pointer would ship what the roster
  excludes, and that contradiction is itself a build error (exit 2). There are no
  exclusions today: HP/03 ruled every current skill either `runs`, `degrades`, or
  `refuses` on Codex CLI — all of which SHIP (see `../references/codex.md`).

`tools/harnessgen codex --check`, exercised by `TestCodex*` in `tools/harnessgen`, enforces
this; those tests cover the coverage, skew, and parse-error paths. `ci.yml`'s module walk
auto-discovers `tools/harnessgen` (build + vet); the suites themselves are proven per the
harness-portability/14 Verify table.

<!-- assay:codex-packaging
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
# Excluded skills, if any, go here as:  <name> :: EXCLUDED: <reason citing HP/03 or HP/01>
# (none today — every current skill runs/degrades/refuses on Codex CLI, all of which ship)
-->
