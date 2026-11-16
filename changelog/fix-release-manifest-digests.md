### Fixed
- `MANIFEST.sha256` now sits beside the bytes it hashes: `make desk-manifest` (and the Windows build script) write it into the install dir and delete the old `tools/desk/MANIFEST.sha256`, which kept describing an earlier from-source build after the binaries were replaced, and `deskinstall` writes one from the files it places (#6604).

### Added
- `tools/release/desk_manifest.py` writes and checks a desk-tools tarball's `MANIFEST.sha256` (every member, unlisted files refused), and the release workflow change that ships it in every tarball is staged as `tools/release/desk-tools-manifest.release.yml.patch` for a maintainer to apply. `docs/distribution.md` now says which file hashes what: `checksums.txt` for downloaded assets (including standalone statusgen/qualgen), the tarball manifest for extracted members.
