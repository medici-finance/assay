### Fixed
- `MANIFEST.sha256` now sits beside the bytes it hashes: `make desk-manifest` (and the Windows build script) write it into the install dir and delete the old `tools/desk/MANIFEST.sha256`, which kept describing an earlier from-source build after the binaries were replaced, and `deskinstall` writes one from the files it places (#6604).

### Added
- `tools/release/desk_manifest.py` writes and checks a desk-tools tarball's `MANIFEST.sha256` (every member, unlisted files refused), and the release workflow's desk-tools package step now writes and checks one in every tarball it builds; releases published before that step carry none. `docs/distribution.md` now says which file hashes what: `checksums.txt` for downloaded assets (including standalone statusgen/qualgen), the tarball manifest for extracted members.
