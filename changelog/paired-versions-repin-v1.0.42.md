### Fixed
- `paired-versions.yaml` is re-pinned from the stale `v1.0.29` to `v1.0.42`: statusgen and desk-tools tags with every per-platform `sha256` taken from the `v1.0.42` release `checksums.txt`, and the harness image tag and registry digest. `containers/base/Dockerfile`'s `DESK_TOOLS_IMAGE` default, still on `desk-tools:v0.1.0`, now names the same `v1.0.42` image and digest.

### Changed
- `tools/pairedversions` (`make paired-versions`, run by the plugin-drift workflow) now fails when a pinned tag is behind the latest published `vX.Y.Z` release of its release home, and the message names the re-pin steps. Before, a plugin that moved on while the pins stood still passed every check. A latest release that cannot be read counts as could-not-check, not a pass.
- `check-paired-versions.sh` also checks that the base Dockerfile's `DESK_TOOLS_IMAGE` matches the harness `image:tag@digest`, so a re-pin can no longer leave the container on old desk tools.
