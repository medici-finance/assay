### Changed
- The plugin's paired-versions manifest now pins statusgen and desk-tools at the published
  umbrella **v1.0.4** on all ten platform lines, every digest re-harvested from that release's
  own checksum manifest and compared back against it. A cold install resolves the v1.0.4
  binaries and verifies them byte-for-byte.
- The adopter-scaffold example gains a v1.0.4 composition manifest with real digests, and its
  notes now name v1.0.4 as the umbrella an upgrade moves to. The v1.0.0, v1.0.1, v1.0.2 and
  v1.0.3 manifests stay: a tree pinned at any of them still has to resolve, and the brief-v1 →
  brief-v2 migration's span ends at v1.0.0.

### Added
- Recorded, with measurements, that **v1.0.4 supersedes v1.0.3 as the upgrade target without
  superseding the flag day** — the same shape the v1.0.1, v1.0.2 and v1.0.3 re-pins recorded. An
  adopter already on v1.0.0, v1.0.1, v1.0.2 or v1.0.3 has no migration to run, only a re-pin,
  while an adopter on v0.28.0 upgrading straight to v1.0.4 still runs the brief-v1 → brief-v2
  migration on the way through rather than being skipped past it.
- The v1.0.4 umbrella this pins to carries statusgen's committer-identity cross-check on
  verified/done briefs (a second, git-derived signal beside the free-string attribution check,
  a NOTICE that degrades loudly when git cannot answer and never over-rejects an honest
  verification whose distinct runners share one identity), and the forge-neutral brief-13
  planning for the write verbs (`deskpr` / `deskfile` / `deskclose` re-seated onto the forge
  resolver — doc/plan only, no tool behaviour change).
