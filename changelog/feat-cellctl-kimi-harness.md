### Added
- `cellctl` gains `kimi` (the Kimi Code CLI) as a third harness beside `claude` and `codex`
  (#1303): `CELL_HARNESS=kimi`, `--harness kimi` on `desk`/`up`/`set`, its own model-pin
  namespace (`KIMI_MODEL_<role>` / `KIMI_MODEL_default`, tier column `TIER_MODEL_<TIER>_KIMI`),
  a `check` block (binary, `--version`, `kimi doctor`, skills dir) and a launch arm that execs
  `kimi -m <model> --skills-dir <bundle skills>` in the role worktree. The invoke-by-name bootstrap
  is printed for the operator (kimi takes no positional prompt) and no resident-rules fragment is
  appended (whether kimi reads a project instructions file is could-not-check). House/k8s kinds
  only; the `claude` and `codex` arms are unchanged.
