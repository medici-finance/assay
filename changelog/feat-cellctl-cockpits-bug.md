### Fixed
- **`cellctl down` no longer rejects itself.** The dispatcher always forwarded a phantom 3rd
  positional to `cmd_down` even on a bare `cellctl down <cell>` — an empty string quoted as one
  argument is still an argument, and `cmd_down`'s flag loop rejected it as
  `down: unexpected argument ''`. It also silently dropped any flag *value* past the first token
  (`--keep-deskd --cockpit tmux` lost `tmux`). `down` now forwards `"${@:2}"` the way `up` always
  has, so every flag combination reaches `cmd_down` intact.
- **The herdr cockpit arm now matches herdr's real grammar.** `herdr agent start`'s
  `-- AGENT_ARG...` list is appended directly to the KIND's canonical executable (confirmed live
  against herdr 0.8.2: a bare `agent start <name> --kind claude --pane <id>` launches literally
  `claude`) — it is not a wrapping shell command, so `-- bash -lc "<cmd>"` never ran `<cmd>`. `up`
  now creates the tab (`herdr tab create --label <l>`), takes the pane id from the real
  `result.root_pane.pane_id` JSON shape, and drives it with `herdr pane run <pane_id> <cmd>` —
  verified live to actually execute the command and return its output. `down` looks the tab up by
  label via `herdr tab list` and closes it by `tab_id` (`herdr tab close <tab_id>` — real herdr
  0.8.2 has no `--label` on `close` at all, unlike the code that shipped before this fix assumed).
- **The orca cockpit arm is proven live, not merely "refuses when the app is closed."** Its
  create-a-terminal "where" flag is `--worktree <selector>` (`path:<dir>`), never `--cwd`/`--path`/
  `--directory` — confirmed live that a real orca advertises no such flag on `terminal create` at
  all — and that selector 404s (`selector_not_found`) until the path is registered once with
  `orca repo add --path <dir>` (idempotent; now called automatically before the first terminal).
  `down` now closes every terminal orca owns for the cell in one call
  (`orca terminal close --worktree path:<cell-dir> --all`) instead of only printing a by-hand
  notice. Along the way: `down_orca`'s own `local roles; roles="$(up_roles 0)" r` was a malformed
  `local` line (a stray `VAR=value r` command, not a second local variable) that made every
  `cellctl down --cockpit orca` die with `r: command not found` before this fix — undetected
  because nothing had ever driven the arm to completion.
### Added
- **`--provider <name>` / `CELL_PROVIDER`: a model endpoint and credential switch, independent of
  `--model`.** `--model` only ever changed the model *name* — a non-Anthropic model
  (`--model glm-5.3`) still talked to Anthropic and failed, because nothing switched the API base
  or the credential. A provider name resolves to `CELL_PROVIDER_<NAME>_BASE_URL` and
  `CELL_PROVIDER_<NAME>_TOKEN_ENV` in `cell.env` — the latter names an environment variable
  (never a token value) the launching shell is expected to carry — and `cellctl desk`/`up` export
  `ANTHROPIC_BASE_URL`/`ANTHROPIC_AUTH_TOKEN` from those before exec'ing `claude`, printing
  `provider=<name>` on the launch line and in `DRY_RUN=1` output. Missing any piece is a refusal
  naming exactly what's absent — the provider's base URL, its token-env variable name, or that
  variable being unset in this shell — never a silent fall-through to Anthropic. `CELL_PROVIDER` in
  `cell.env` is the default (unset = Anthropic, unchanged behaviour); `--provider` overrides it for
  one run, threading onto every role window `up` opens the same way `--model` does.
  `cellctl check` carries the default provider's three preconditions (n/a when unset, since a
  provider is opt-in), and `cellctl set` accepts `CELL_PROVIDER`/`CELL_PROVIDER_<NAME>_*` without
  `--force`. `tools/cellctl/tests/{down,herdr-orca-launch,provider}.test.sh` cover all of the above
  offline, against stubs shaped from live probes of the real herdr/orca binaries.
