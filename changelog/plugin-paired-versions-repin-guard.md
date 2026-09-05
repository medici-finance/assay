### Fixed
- The adopter front door no longer installs a stale tool. `plugins/assay/paired-versions.yaml`
  had been left pinned for plugin `0.4.0` and statusgen `v0.13.0` while the shipped plugin
  moved to `0.5.0` — so a clean `assay:install` resolved a statusgen many minors behind the
  skills it ships alongside. The manifest is re-pinned to plugin `0.5.0` / statusgen `v0.25.1`,
  with every per-platform sha256 refreshed from that release's published `checksums.txt`.

### Added
- `tools/pairedversions` — a fail-closed guard for the plugin↔statusgen pairing, so the
  re-pin cannot be skipped silently again. It asserts that `plugin.json`'s version matches
  the manifest's `plugin`, that each paired tag is a *published* release of its release home,
  and that every pinned sha256 equals that release's own `checksums.txt` entry. A
  could-not-check reddens the run rather than passing it, and one invocation reports every
  disagreement at once. `make paired-versions` runs it; the CI workflow that makes it a gate
  is staged for a human to land at `tools/pairedversions/activation/plugin-drift.yml`.
