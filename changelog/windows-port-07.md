### Added
- `deskinstall --harness cursor --forge <github|gitlab> --repo <path>` places Cursor's
  install (skills + a sibling `references/` tree so `../../references/*.md` includes
  resolve, plus the generated `.cursor/rules/assay.mdc` when present) and writes the
  shared `AGENTS.md` bindings block, forge-substituted (`gh` on github, `glab` /
  `--forge gitlab` on gitlab). Idempotent; `--check` reports drift (missing / extra /
  content-differs) without writing.
