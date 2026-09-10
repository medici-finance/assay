### Changed
- The plugin's paired-versions manifest now pins statusgen and desk-tools at the published
  umbrella **v1.0.2** on all ten platform lines, every digest re-harvested from that release's
  own checksum manifest and compared back against it. A cold install resolves the v1.0.2
  binaries and verifies them byte-for-byte.
- The adopter-scaffold example gains a v1.0.2 composition manifest with real digests, and its
  notes now name v1.0.2 as the umbrella an upgrade moves to. The v1.0.0 and v1.0.1 manifests
  stay: a tree pinned at either still has to resolve, and the brief-v1 → brief-v2 migration's
  span ends at v1.0.0.

### Added
- Recorded, with measurements, that **v1.0.2 supersedes v1.0.1 as the upgrade target without
  superseding the flag day** — the same shape the v1.0.1 re-pin recorded. An adopter already
  on v1.0.0 or v1.0.1 has no migration to run, only a re-pin, while an adopter on v0.28.0
  upgrading straight to v1.0.2 still runs the brief-v1 → brief-v2 migration on the way through
  rather than being skipped past it.
