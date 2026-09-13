### Added
- `cellctl desk`/`cellctl up` accept `--model <m>` — a per-run override of the `cell.env` model
  pin (`DESK_MODEL_OVERRIDE` is the equivalent env form); `up --model` applies it to every role
  window it opens. The `[launch]` line and `DRY_RUN=1` output print `model=<m> (override)` so the
  source of the value is visible in the transcript. The existing the-desk/Opus refusal applies to
  an override exactly as it does to a pin.
- `cellctl set <cell> KEY=VALUE [...]` persists a `cell.env` change in place (comments and ordering
  preserved), with a `cell.env.bak-<ts>` backup, a refusal on an unknown key unless `--force`, and
  the same the-desk/Opus refusal on `DESK_MODEL_the_desk`. `cellctl desk ... --model <m> --set` is
  sugar for "override this run and persist it".
