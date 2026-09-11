### Changed
- The plugin's paired-versions manifest now pins statusgen and desk-tools at the published
  umbrella **v1.0.6** on all ten platform lines, every digest re-harvested from that release's
  own checksum manifest and compared back against it. A cold install resolves the v1.0.6
  binaries and verifies them byte-for-byte.
- The adopter-scaffold example gains a v1.0.6 composition manifest with real digests, and its
  notes now name v1.0.6 as the umbrella an upgrade moves to. The v1.0.0 through v1.0.5
  manifests stay: a tree pinned at any of them still has to resolve, and the brief-v1 →
  brief-v2 migration's span ends at v1.0.0.

### Added
- Recorded, with measurements, that **v1.0.6 supersedes v1.0.5 as the upgrade target without
  superseding the flag day** — the same shape the v1.0.1 through v1.0.5 re-pins recorded. An
  adopter already on v1.0.0 through v1.0.5 has no migration to run, only a re-pin, while an
  adopter on v0.28.0 upgrading straight to v1.0.6 still runs the brief-v1 → brief-v2 migration
  on the way through rather than being skipped past it.
- The v1.0.6 umbrella this pins to carries statusgen's scoped same-tag pin lint (#794): an
  exempt or non-umbrella artifact line in `.assay-versions` no longer makes the whole file
  unlintable, so an adopter carrying one can re-pin and lint clean again. The re-pinned
  desk-tools also accept the channel-D `desk-tools-source` pin shape (#797).
