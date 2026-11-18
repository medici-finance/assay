### Fixed
- `deskevidence --evidence-file` now accepts a brief file up to 1 MiB (was 256 KiB), so completed verifications on briefs that have accumulated many Evidence passes can land their Evidence byte-for-byte. A single outcome record keeps its 256 KiB bound.
