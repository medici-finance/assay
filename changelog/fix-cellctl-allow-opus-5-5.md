### Fixed
- `cellctl`'s the-desk model policy now gates Opus by a VERSION FLOOR instead of a fixed
  allowlist: the coordinator ACCEPTS any Opus tier at or above **5.5** (`claude-opus-5-5`,
  `claude-opus-5-6`, `claude-opus-6`, and a future `claude-opus-9`, including `[1m]` and
  `-`/`.` spellings) and REFUSES anything below it (the bare `opus` alias, Opus 5.0 —
  `claude-opus-5` / `-5-0` / `-5.0` — and older tiers such as Opus 4.8). A future Opus tier
  therefore auto-qualifies as the-desk's top tier with no code edit — a deliberate, documented
  trade (a floor adopts a future Opus sight-unseen; the "Opus 5.0 was a bad tier despite its
  number" lesson makes that a choice, reversible by raising the floor). The built-in deny
  patterns stay anchored at end-of-token (`*opus-5` / `*opus5` / `*opus-5-0` / `*opus-5.0`), and
  the "which Opus tiers the-desk refuses" decision lives in one shared helper (`isOpusPin`) that
  both the legacy resolver and the policy resolver consult, so the carve-out cannot fork between
  sites.
