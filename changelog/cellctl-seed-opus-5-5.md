### Changed
- The shipped `cellctl` provider catalog seed (`providers.json`, created by `cellctl providers init`) now pins the Anthropic `strong` tier (the Opus alias `pr-review-desk` and `verify-desk` run on) to `claude-opus-5-5[1m]` instead of `claude-opus-4-8[1m]`. An existing `$CELLS_ROOT/providers.json` is never overwritten; operators edit theirs by hand.
