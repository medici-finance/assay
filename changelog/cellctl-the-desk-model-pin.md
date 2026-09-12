### Changed
- **`the-desk` scaffolds pinned to the top-tier model (`fable`) instead of `opus`, and `cellctl`
  refuses to run it on Opus.** All three `cellctl new` templates (k8s/github, k8s/gitlab, house)
  now write `DESK_MODEL_the_desk=fable` — the coordinator role is the one window that spends its
  tier on judgment, synthesis and arbitration across streams, and `opus` is no longer the top tier.
  `cellctl desk <cell> the-desk` (its `DRY_RUN=1` plan included) refuses when the resolved model —
  from `DESK_MODEL_the_desk` or, unset, `DESK_MODEL_DEFAULT` — is the `opus` alias or a
  `claude-opus-*` id, naming the resolved value and the variable to change; `cellctl check` shows
  the resolved the-desk model and flags an Opus pin the same way, as a MISS. Every other role's
  pin, `opus` included, is untouched — an operator can still pin `the-desk` itself to any
  non-Opus id.
- `tools/cellctl/tests/model-pin.test.sh` — a plain-bash, no-network test covering the scaffolded
  default on all three kinds/forges, the dry-run and `check` refusal on `opus` and a full
  `claude-opus-*` id (including the `DESK_MODEL_DEFAULT` fallback path), acceptance of `fable` and
  a full `claude-fable-*` id, and that a non-the-desk role pinned to `opus` is left alone.
