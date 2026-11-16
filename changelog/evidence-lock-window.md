### Fixed
- `deskevidence` holds the shared desk audit lock only for its write window (budget read, conditional forge write, audit append) instead of the whole run, so a landing's preparation and statusgen lint no longer starve other desk writers that wait on the same lock. Preparation keeps its own per-landing-root lock, so two landings on the same root still never stage into it at once.
