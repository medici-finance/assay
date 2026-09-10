### Changed
- The plugin's paired-versions manifest now pins statusgen and desk-tools at the published
  umbrella **v1.0.3** on all ten platform lines, every digest re-harvested from that release's
  own checksum manifest and compared back against it. A cold install resolves the v1.0.3
  binaries and verifies them byte-for-byte.
- The adopter-scaffold example gains a v1.0.3 composition manifest with real digests, and its
  notes now name v1.0.3 as the umbrella an upgrade moves to. The v1.0.0, v1.0.1 and v1.0.2
  manifests stay: a tree pinned at any of them still has to resolve, and the brief-v1 →
  brief-v2 migration's span ends at v1.0.0.

### Added
- Recorded, with measurements, that **v1.0.3 supersedes v1.0.2 as the upgrade target without
  superseding the flag day** — the same shape the v1.0.1 and v1.0.2 re-pins recorded. An
  adopter already on v1.0.0, v1.0.1 or v1.0.2 has no migration to run, only a re-pin, while an
  adopter on v0.28.0 upgrading straight to v1.0.3 still runs the brief-v1 → brief-v2 migration
  on the way through rather than being skipped past it.
- The v1.0.3 umbrella this pins to carries `statusgen conform`'s brief-v2 contract (the
  `--emit-schema --schema brief-v2` payload and the `schemas/brief-v2.json` it stays in
  lockstep with), the public flag-day corroboration addition (`statusgen --corroborate` now
  fires its human-stamp anchor for a decision record, `DR-<slug>.md`, not just a brief file),
  and desk-tools fixes (`deskdispatch` worker prompts now name their own loop identity and the
  exact `deskpr create` trailer, and the SessionStart banner resolves the plugin version from
  the manifest instead of a stale literal).
