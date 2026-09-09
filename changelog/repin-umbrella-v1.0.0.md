### Changed
- The plugin's paired-versions manifest now pins statusgen and desk-tools at the published
  umbrella **v1.0.0**, on all ten platform lines (darwin arm64/amd64, linux amd64, windows
  amd64/arm64 for each). Every digest was harvested from the release's own checksum manifest
  and compared field-for-field against it, so a cold `assay:install` resolves the v1.0.0
  binaries and verifies them byte-for-byte. `linux-arm64` stays deliberately unpinned in both
  sections — v1.0.0 publishes no such asset, and the acquisition refuses rather than guesses
  when a detected platform has no pin line.
- The adopter-scaffold example's v1.0.0 composition manifest carries the real release digests
  instead of fixture placeholders, so the upgrade target an adopter dry-runs against now shows
  the same values their own pin file will hold.
