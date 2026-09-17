### Added
- Every desk write verb (`deskpost`, `deskreply`, `deskpr`, `deskfile`, `deskevidence`,
  `deskflip`) now stamps an `On-behalf-of: human:<login>` composite-identity trailer —
  GitLab's "service account on behalf of `@human`" model, generalised to the shared-App
  fleet — resolved exclusively from the roster's config-home file (never an environment
  variable) and refusing (exit 5) rather than writing without one. `--dry-run` prints the
  trailer it would have written. See `docs/on-behalf-of.md`.
- `statusgen verifyrun`'s witness Runner cell annotates an App/bot runner with
  `on-behalf-of human:<login>` when a principal resolves.
- `statusgen --lint` flags an App-authored Evidence row whose on-behalf-of annotation
  names a login outside the roster's human map as a hard PROBLEM, and one with no
  annotation at all as a hard PROBLEM once dated at or after the write path's own
  landing date (a NOTICE for a row grandfathered from before it — no write path existed
  yet to stamp it).

### Changed
- `deskpr edit`'s noop compare now strips a prior on-behalf-of trailer from the PR's live
  body before comparing against the caller's replacement, so a trailer-only delta still
  noops instead of re-posting.
- Every write verb that appends the on-behalf-of trailer (`AppendOnBehalfOf`) now strips
  any On-behalf-of line the caller-supplied body already contains, wherever it sits,
  before appending its own — a caller can no longer plant or shadow the annotation.
