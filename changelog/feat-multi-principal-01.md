### Added
- Every desk write verb (`deskpost`, `deskreply`, `deskpr`, `deskfile`, `deskevidence`,
  `deskflip`) now stamps an `On-behalf-of: human:<login>` composite-identity trailer —
  GitLab's "service account on behalf of `@human`" model, generalised to the shared-App
  fleet — resolved exclusively from the roster's config-home file (never an environment
  variable) and refusing (exit 5) rather than writing without one. `--dry-run` prints the
  trailer it would have written. See `docs/on-behalf-of.md`.
- `statusgen verifyrun`'s witness Runner cell annotates an App/bot runner with
  `on-behalf-of human:<login>` when a principal resolves.
- `statusgen --lint` flags an App-authored Evidence row with no on-behalf-of principal, or
  one naming a login outside the roster's human map, as a hard PROBLEM.

### Changed
- `deskpr edit`'s noop compare now strips a prior on-behalf-of trailer from the PR's live
  body before comparing against the caller's replacement, so a trailer-only delta still
  noops instead of re-posting.
