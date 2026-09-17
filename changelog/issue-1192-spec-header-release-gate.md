### Added
- The release workflow now refuses to cut a tag when `spec/brief-v1.md`'s
  `Describes reference implementation:` version does not match the tag being
  cut (`tools/release/check-spec-header-version.sh`, wired into `release.yml`'s
  `guard` job) — the release-time floor brief-13's Task 4 named but never
  shipped, closing the recurring staleness class (v0.8.0-vs-v0.19.0, then
  v0.22.0-vs-v1.0.9). (#1192)

### Fixed
- `spec/brief-v1.md`'s header freshened from the stale `statusgen v0.22.0` to
  the actual current release, `v1.0.12`. (#1192)
