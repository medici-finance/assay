### Fixed
- `deskboot`'s first-sweep-prereqs step and `deskboard`'s drift banner now accept the
desk-tools pin line the installer writes, `desk-tools-<os>-<arch>.tar.gz <tag> <sha256>`.
Before, a pin file fresh from `assay-install.sh pin --kind desk-tools` was refused at boot
and read as could-not-check by the board. This adds one accepted spelling: each name is
still looked up in full, the bare and suffixless lines keep precedence, and a platform
token that is not `<os>-<arch>` no longer selects any line.
