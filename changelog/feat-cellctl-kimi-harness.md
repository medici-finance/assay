### Added
- `cellctl` gains built-in provider presets `kimi` and `glm` for running the claude harness against
  Anthropic-compatible endpoints (#1303): `--provider kimi|glm` on `desk`/`up`/`set` works with no
  `cell.env` line beyond the operator exporting `KIMI_API_KEY` / `ZAI_API_KEY` in their shell
  (cellctl never stores or prints a token value); a new `CELL_PROVIDER_<NAME>_MODEL` key (preset
  defaults `k3[1m]` / `glm-5.3[1m]`) is the model a provider window runs absent `--model` or a
  per-role pin; the launch unsets `ANTHROPIC_API_KEY` and exports `ANTHROPIC_MODEL` plus the three
  `ANTHROPIC_DEFAULT_{OPUS,SONNET,HAIKU}_MODEL` aliases; `cellctl check`/`show` report the endpoint,
  the token env var's name, set/unset and the model, each tagged preset/cell.env; codex + provider
  is refused.
- Every per-run `cellctl` choice is now a flag, persistable and readable (#1303): `desk`/`up`
  accept `--kind`, `--cockpit`, `--harness`, `--provider`, `--model` for one run; `--set` persists
  every override given in that invocation (each to its own `cell.env` key, one backup first);
  `cellctl set <cell> --kind/--cockpit/--harness/--provider` is sugar for the matching
  `KEY=VALUE` under the same validation; a kind change refuses before writing when the target
  kind's precondition (`CELL_CONTAINER_LAUNCHER`, `CELL_ROOTS`, `CELL_REPO_SLUG`) is missing; and
  a new `cellctl show <cell>` prints each effective value with its source
  (`flag` / `cell.env` / `default`).
