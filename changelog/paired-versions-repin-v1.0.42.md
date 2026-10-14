### Fixed
- `paired-versions.yaml` is re-pinned from the stale `v1.0.29` to `v1.0.42`: statusgen and desk-tools tags with every per-platform `sha256` taken from the `v1.0.42` release `checksums.txt`, and the harness image tag and registry digest. `containers/base/Dockerfile`'s `DESK_TOOLS_IMAGE` default, still on `desk-tools:v0.1.0`, now names the same `v1.0.42` image and digest.

### Changed
- `check-paired-versions.sh` also checks that the base Dockerfile's `DESK_TOOLS_IMAGE` matches the harness `image:tag@digest`, so a re-pin can no longer leave the container on old desk tools.
