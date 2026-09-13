### Added
- `cellctl desk`/`cellctl up` accept `--harness <claude|codex>` — a per-run choice of which harness
  a role window boots on, default `claude` (never touches `cell.env`); `up --harness` applies it to
  every role window it opens. `CELL_HARNESS` is the persisted `cell.env` pin, scaffolded by
  `cellctl new`.
- The codex arm execs `codex --sandbox danger-full-access -C <worktree> -m <model> "Invoke the
  \"assay:<role>\" skill now."`, with the same `DESK_LOOP`/`DESK_SESSION`/`DESK_ROOTS`/shim-`PATH`
  env the claude arm gets, the model from the same `DESK_MODEL_*` resolution, and the resident-rules
  fragment appended to the worktree's `AGENTS.md` idempotently (codex has no `SessionStart` hook).
  The the-desk/Opus refusal binds the claude arm only — codex refuses nothing and prints the
  resolved model.
- `cellctl check` gains a codex harness block (binary + version, authentication, `multi_agent`, the
  resident-rules fragment, skills discoverability) when `CELL_HARNESS=codex`; `n/a` on a claude
  cell.
- `[dry-run]`/`[launch]` print `harness=<h>`, and a non-claude `DESK_SESSION` carries a `-codex`
  suffix, so the roster beacon shows which harness a window is on.
