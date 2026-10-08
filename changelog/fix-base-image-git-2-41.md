### Fixed
- The desk base image now ships git 2.56.0, built from a pinned, sha256-checked upstream release, instead of bookworm's 2.39.5. Verifier admission needs `git --attr-source` (2.41+), so verifier homes built on the old base refused every brief with a converted file (#2318).

### Added
- `containers/scripts/git-floor-check.sh` proves the git floor: it runs at base-image build time, and with an image ref it checks an already-built image. It fails if any git on `PATH` is below 2.41 or if `--attr-source` is rejected or ignored.
