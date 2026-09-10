### Changed
- The plugin's paired-versions manifest now pins statusgen and desk-tools at the published
  umbrella **v1.0.5** on all ten platform lines, every digest re-harvested from that release's
  own checksum manifest and compared back against it. A cold install resolves the v1.0.5
  binaries and verifies them byte-for-byte.
- The adopter-scaffold example gains a v1.0.5 composition manifest with real digests, and its
  notes now name v1.0.5 as the umbrella an upgrade moves to. The v1.0.0, v1.0.1, v1.0.2,
  v1.0.3 and v1.0.4 manifests stay: a tree pinned at any of them still has to resolve, and the
  brief-v1 → brief-v2 migration's span ends at v1.0.0.

### Added
- Recorded, with measurements, that **v1.0.5 supersedes v1.0.4 as the upgrade target without
  superseding the flag day** — the same shape the v1.0.1 through v1.0.4 re-pins recorded. An
  adopter already on v1.0.0 through v1.0.4 has no migration to run, only a re-pin, while an
  adopter on v0.28.0 upgrading straight to v1.0.5 still runs the brief-v1 → brief-v2 migration
  on the way through rather than being skipped past it.
- The v1.0.5 umbrella this pins to carries statusgen's header-keyed base-cell resolution for
  the `--corroborate` pre-existing-stamp exemption (#785): the base cell is located by column
  HEADER NAME rather than by the branch's positional index, so a sign-off whose cell text is
  unchanged still reads `PRE-EXISTING` when the board table has been RE-SHAPED under it,
  instead of being re-gated. Every fail-closed guard is preserved, including the branch-column-
  absent-from-base case, which now carries its own committed regression test (#788).
