### Fixed
- **`--harness codex` no longer passes a Claude model name straight to `codex -m`.** `cellctl
  desk`/`cellctl up` previously resolved a role's model from `DESK_MODEL_<role>`/`DESK_MODEL_DEFAULT`
  regardless of harness, so a cell pinned Claude-only (`fable`, `opus`, `sonnet`, ...) handed codex a
  name it does not understand, and the reverse (a codex-only pin reaching the claude arm) was
  equally broken (`#986`).

### Added
- **Per-harness model-pin namespaces.** `DESK_MODEL_<role>`/`DESK_MODEL_DEFAULT` are now explicitly
  the **claude** namespace (unchanged, backward compatible); codex gets its own —
  `CODEX_MODEL_<role>` per-role, `CODEX_MODEL_default` as its harness-wide fallback (no compiled
  default). The two are never cross-read.
- **A top/mid/fast tier-map fallback** for a role/harness with neither its own per-role pin nor that
  harness's default: `the-desk` resolves at `top`, every other role at `mid` (`fast` is defined but
  not auto-assigned). Compiled defaults — `fable`/`sonnet`/`haiku` for claude,
  `gpt-5.6-terra` (the one codex model id proven live, `docs/codex-smoke-runs/2026-09-12-codex-0.154.0.md`)
  for all three codex tiers today — are each overridable in `cell.env` via
  `TIER_MODEL_<TIER>_<HARNESS>`. This is what lets a cell pinned Claude-only today boot
  `--harness codex` with a real, working model and no manual re-pin.
- `cellctl set <cell> <role> [--harness claude|codex] --model <m>` — role-sugar that writes whichever
  namespace the ACTIVE harness uses (the flag given, else the cell's own `CELL_HARNESS`), so a codex
  call writes `CODEX_MODEL_<role>`, never `DESK_MODEL_<role>`. `cellctl desk ... --harness codex
  --model <m> --set` persists into the same namespace for a live boot.
- `cellctl check` prints one `model pin: role=<role> harness=<harness> model=<m> (from <source>)`
  row per role the cell runs, on the cell's pinned harness — a role with no per-harness pin and no
  tier match is a `MISS` naming exactly what was checked, visible before boot rather than discovered
  as a startup failure.
- An explicit `--model <m>` (or `DESK_MODEL_OVERRIDE`) still passes through verbatim to the selected
  harness on either arm — it is never routed through the namespace/tier resolution above.
- `tools/cellctl/tests/model-namespace.test.sh` — a plain-bash, no-network test covering the
  tier-map fallback with no manual re-pin, the unaffected claude arm, per-role/default codex pins
  winning over the tier map, `--model` bypassing all resolution, the no-pin/no-tier-match refusal on
  both `desk` and `check`, the `TIER_MODEL_<TIER>_<HARNESS>` override, and the harness-aware
  `cellctl set` role-sugar form. `tools/cellctl/tests/harness.test.sh` is updated where it asserted
  the old (buggy) pass-through behavior.
