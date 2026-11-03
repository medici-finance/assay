### Added
- `statusgen` reads a new, negotiated brief schema, `brief-v4` (`schemas/brief-v4.json`). It adds the colon-only prefixed definition id `<cell>:<repo>:<stream>:b-<NN>`, a required `version` and `id`, an explicit `requires-milestones` list of exact versioned milestone references, and typed brief references with an optional `@<version>` selector. An unversioned reference binds at the selected source snapshot; a pinned revision that is not there stays unknown and holds the brief instead of binding a newer one. A declared milestone requirement holds the brief as could-not-check, because this reader does not yet evaluate milestone acceptance. `brief-v4` has no supporting release yet, and no existing brief is migrated.
- `statusgen newbrief --schema brief-v4 --reader-caps FILE...` writes the prefixed id/schema pair only after statusgen's own advertisement and every supplied reader advertisement are assessed supported for the record's `reader-requirements`. With no advertisement, an unreadable or malformed one, or any unsupported reader, it refuses before writing anything. `--schema legacy` selects the stream's supported legacy schema explicitly, and `--requires-milestone` without `brief-v4` is refused rather than dropped.

### Changed
- `brief-v1`, `brief-v2` and `brief-v3` files that carry `requires-milestones` or a prefixed `b-` definition id are now refused, because a legacy reader would tolerate the key and silently lose the start requirement.

### Fixed
- `statusgen newbrief` wrote the new brief's hierarchical `brief:` id into a brief-v2 dependency's `unblocks:`, which `--lint` then refused. It now writes the `<stream>/<NN>` form (or `<stream>:b-<NN>` into a brief-v4 dependency).
