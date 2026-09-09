### Added
- **Report packs** — a named, mechanically-checkable install unit for periodic reporting
  tools. A pack ships as a sha256-pinned release binary, emits its own CI via `<tool> init`,
  keeps its committed output single-writer, and loads operator values from config. The
  normative contract is `docs/report-packs.md` (linked from `docs/distribution.md`).
- `qualgen` joins the umbrella release as per-platform binaries with checksums, so an adopter
  pins a `qualgen-<platform>` line in `.assay-versions` and obtains the quality-report tool
  without building from source.
- `qualgen init` scaffolds the report pack into an adopter repo — a generated, single-writer
  quality-report workflow plus a `.assay-versions` pin — acquiring `qualgen` only as the
  pinned release binary.

### Changed
- The channel-conformance sweep now registers `qualgen` as a released pack tool, so a
  build-from-source or `go run` of it on an adopter surface reddens the sweep. The producing
  repository keeps self-hosting `qualgen` from source — the seam the pack contract names.
