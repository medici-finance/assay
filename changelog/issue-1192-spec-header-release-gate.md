### Added
- `tools/release/check-spec-header-version.sh`: a release-time check that
  `spec/brief-v1.md`'s `Describes reference implementation:` version matches
  the tag being cut — the release-time floor brief-13's Task 4 named but
  never shipped, closing the recurring staleness class (v0.8.0-vs-v0.19.0,
  then v0.22.0-vs-v1.0.9). Wiring it into `release.yml`'s `guard` job is
  **staged, not yet activated** under `tools/release/` (see
  `tools/release/README.md`) — the worker-desk App cannot push under
  `.github/workflows/` (server-side workflows-scope block); a
  workflows-capable identity applies `tools/release/release.yml.patch` to
  activate it. (#1192)

### Fixed
- `spec/brief-v1.md`'s header freshened from the stale `statusgen v0.22.0` to
  the actual current release, `v1.0.12`. (#1192)
