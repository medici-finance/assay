### Fixed
- `desktoken <role> --ttl` refuses a cached-token path that is a symlink (or not a regular file) instead of reporting the TTL and age of whatever the link points at.
- `repohardenguard` mints its auditor token through the in-process minter instead of forking whichever `desktoken` is first on `PATH`.

### Added
- The minter class guard now also fails on any non-test Go file outside `cmd/desktoken` that names the `desktoken` program, so a new caller cannot step around the in-process minter by forking the binary. The sites that still fork are listed in the guard with the reason each one stays.
