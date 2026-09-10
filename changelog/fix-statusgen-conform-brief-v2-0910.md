### Added
- `schemas/brief-v2.json` — the machine-readable brief-v2 contract, alongside the
  existing brief-v1 one. It covers the whole brief-v1 surface plus the hierarchical
  `brief: <cell>:<repo>:<stream>:<NN>` id, the `version:` revision counter, and the
  reserved `id` / `supersedes` / `gates` / `feathers` / `verify` keys, and is held in
  lockstep with the reference validator by `TestBriefV2SchemaCoverage`.
- `statusgen conform --emit-schema --schema brief-v2` prints the brief-v2 contract, so
  every embedded artifact stays reproducible from a pinned build.

### Fixed
- `statusgen conform` now selects its contract **per file** from that file's own
  `schema:` marker, so a tree migrated by `statusgen migrate brief-v1-to-v2` validates
  instead of reporting `could-not-check` for every brief and exiting 2 — which reddened
  the schema-contract check on the very PR that landed an adopter's flag day. A tree
  mid-migration holding both versions validates each file against the version it
  declares. The three-state behaviour is unchanged: a marker no embedded contract
  describes is still `could-not-check` / exit 2, and a brief-v2 field error is a real
  `checked-failed` / exit 1.
