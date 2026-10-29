### Fixed
- `desksourceguard` now reads the desk-tools binary pin under both spellings the shared
reader accepts: the suffixless `desk-tools-<os>-<arch>` line, then the
`desk-tools-<os>-<arch>.tar.gz <tag> <sha256>` line the installer writes. Before, a pin file
fresh from the installer read as having no binary pin, so the guard reported could-not-check.
Each name is still looked up in full, so another component's or another platform's line
never stands in, a malformed line still fails closed, and a `--platform` value that is not
`<os>-<arch>` now selects no line instead of reading the `desk-tools-source` line as the
binary pin.
