### Added
- `statusgen migrate brief-v1-to-v2 [--dry-run]` — the brief-v1 → brief-v2 flag-day migration: rewrites each brief's `schema:`, mints the hierarchical `<cell>:<repo>:<stream>:<NN>` id from `docs/streams/graph-repos.yaml`, adds `version: 1` and a uuid v4 `id:` where absent, and wraps each stream README's Briefs table in the generated-region markers with `board: generated`. Idempotent; refuses (exit 5) when the alias registry is absent.
- `deskmigrate` `statusgen-regen` op — a declarative, dry-runnable migration step that runs the pinned statusgen's `migrate` verb over the adopter tree (an unknown verb/target is refused, not run blind).
- The first REAL migration, `0001-v0.13.0-to-v1.0.0-derived-board`, plus `docs/release-notes/v1.0.0.md` (same prose): what changes on an adopter's board, the `Brief:` trailer they must now write, and the `statusgen reconcile --backfill` step they add to their board workflow.
- Same-tag pin lint: `statusgen --lint` PROBLEMs a `.assay-versions` whose artifact tags differ (one tag, one tree — no version matrix).
- The brief-reading version gate: `statusgen`, `deskboard`, `deskpr`, `deskclaim` and `deskevidence` built below v1.0.0 refuse a `brief-v2` tree (exit 6), pointing at `assay:upgrade-assay`. Unstamped local builds behave as latest and are never gated.

### Changed
- Bumped the plugin to `1.0.0` and re-paired `paired-versions.yaml` to statusgen/desk-tools `v1.0.0`; the per-platform `sha256`s are left as the reserved `<harvest-after-release>` placeholder for the cut-release skill to fill (a hash for an uncut tag is never hand-typed). `check-paired-versions.sh` now accepts that one reserved placeholder while still rejecting every other non-64-hex digest.
- Migrated the `examples/adopter-scaffold` fixture to exercise the migration end to end: added its `graph-repos.yaml` alias registry, `releases/{v0.13.0,v1.0.0}.yaml` composition manifests, and an umbrella-pinned `.assay-versions`.
