### Fixed
- `deskevidence` no longer refuses appends to `docs/streams/verify-outcomes.jsonl` past the
  general 256 KiB per-file cap — the append-only aggregate sidecar now carries its own
  documented 4 MiB ceiling (#1338).

### Added
- A rotation-aware `verify-outcomes*.jsonl` glob-union reader in `statusgen`, so a future
  date-sharded rotation of the verify-outcomes sidecar has read-side support in place before
  any rotation is attempted.
