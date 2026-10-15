### Fixed
- `statusgen reconcile` no longer treats a merged PR that the brief's own `delivery:` list declares a partial delivery (a skeleton, or one slice of several) as the witness that the brief is implemented. That merge now derives `in-progress`, the same in the trailer fold, the `--backfill` match and `regen`'s drift comparator. A partial claim that cannot be resolved makes the brief `unknown` and refuses `--apply`.
- forge-neutral/10 and forge-neutral/18 declare their skeleton, authoring and slice PRs as partial deliveries, so their rows no longer read as implemented.
