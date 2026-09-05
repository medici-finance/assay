### Changed
- The plugin's `paired-versions.yaml` is re-pinned to statusgen / desk-tools **v0.26.0** and
  plugin **0.5.1** — both sides of the pairing move together, with every per-platform sha256
  refreshed from the v0.26.0 release's own `checksums.txt` rather than edited in place.
- `examples/adopter-scaffold/.assay-versions` moves off the long-stale `v0.9.1` pins to
  **v0.26.0**, so the scaffold an adopter copies no longer illustrates a tool seventeen releases
  behind the skills shipped beside it.

### Added
- **Windows is pinned, not deferred.** v0.26.0 is the first release to publish
  `statusgen-windows-{amd64,arm64}.exe` and `desk-tools-windows-{amd64,arm64}.tar.gz`, so the
  Windows arms of both the pairing manifest and the adopter scaffold carry real, published
  sha256 values. The scaffold's two all-zero placeholder digests — which would have failed an
  adopter's verification the moment a Windows install was attempted — are gone.
