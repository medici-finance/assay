### Added
- `tools/release/stamp-spec-header-version.sh` writes the release tag into `spec/brief-v1.md`'s `Describes reference implementation:` header, the way the release already stamps the plugin manifest; `check-spec-header-version.sh` proves the stamp ran. The `release.yml` wiring (stamp then check at cut time, a tag-push gate, and the post-cut bump in the changelog roll) is staged in `tools/release/release.yml.patch` for a maintainer to apply.

### Fixed
- `spec/brief-v1.md`'s header names the current release (`v1.0.39`) again instead of `v1.0.12`.
