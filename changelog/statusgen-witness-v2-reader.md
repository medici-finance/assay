### Added
- `statusgen` reads a content-bound witness (schema v2): a Runner cell marked `[w2]` names a canonical witness record, and coverage releases the row when the recorded change request landed on the default branch and every recorded input digest still equals the content at the item's revision. A squash merge no longer orphans a valid witness, and a whole-tree scope no longer refuses a witness whose real inputs never changed. A schema v1 cell is judged by the unchanged ancestry rule, and an unknown `[w<N>]` marker is a named coverage gap, never read as v1 or v2.
- A new `inputs:` Context field declares the files and directories a brief's Verify rows read; `--lint` reports a path a row's Command names literally that the declaration omits.
- `statusgen witnessdigest --json` prints the canonical v2 content fields from the one digest owner in `statusgen/evaluation`, and `--coverage --json` carries a versioned `witness` field per claim (`schema`, `judgment`, `gap`, `cr`, `runner`, `independent`, `evidence_actor`).

### Changed
- Rows that depend on the network, a code-hosting service or a cluster are never released by digest: a host or back-filled run is environment-free only when its command is readable and names no network tool (looking through `env`, `timeout`, `sudo`, `xargs`, `sh -c` and similar wrappers; `make` and other opaque runners are unreadable).
