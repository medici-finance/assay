### Added
- `statusgen lint --check citation-alias-resolution` resolves every aliased ref (`<alias>:<stream>/<NN>`, `<alias>#<NNN>`, `<cell>:<alias>:<stream>/<NN>`) in a brief's `sources:`, `consumers:` and `## Evidence` through `docs/streams/graph-repos.yaml`. An alias the registry does not define is a PROBLEM naming it. `statusgen lint` runs named checks only; an unknown check name exits 2 and never passes.
- `statusgen verify-gate-close --ref <ref> [--dry-run]` does the human done-close by a ref in any brief form. It resolves the alias through the registry first: exit 5 when the alias is unknown or belongs to another repo, 6 when the tree has no registry.

### Fixed
- `statusgen --close-verify` no longer drops the repo alias of a `<cell>:<repo>:<stream>:<NN>` id. Before this fix, an id naming another repo flipped this tree's same-numbered brief to `done`. Aliased ids now resolve through the registry and are refused unless they name this tree's own alias.
