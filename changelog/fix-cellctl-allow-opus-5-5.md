### Fixed
- `cellctl`'s the-desk model policy no longer over-bans the whole Opus-5 family. Opus 5.5
  (`claude-opus-5-5`, including the `[1m]` variant) is now accepted as a valid top tier, while
  Opus 5.0 (`claude-opus-5`, plus its `[1m]`, gateway, `Opus5`, `-5-0` and `-5.0` spellings), the
  bare `opus` alias and older opus tiers stay refused for the coordinator. The built-in deny
  patterns are anchored at end-of-token (`*opus-5` / `*opus5`, no trailing wildcard), and the
  "which opus tiers the-desk refuses" decision now lives in one shared helper (`isOpusPin`) that
  both the legacy resolver and the policy resolver consult, so the carve-out cannot fork between
  sites.
