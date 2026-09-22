### Changed
- harness: pin the v1.0.24 desk-tools image digest (`plugins/assay/paired-versions.yaml`
  `harness.tag`/`harness.digest`), replacing the fail-closed `PENDING-HARVEST` placeholder now that
  the image has been published and its digest harvested from two independent registry reads.
- `check-paired-versions.sh` now exempts `harness.tag` from its single-tag assertion: the harness
  image publishes on its own cadence and is verified by digest, not tag, so a harness tag ahead of
  the paired `statusgen`/`desk-tools` release is expected, not drift. `statusgen`/`desk-tools` and
  every per-platform pin line still have to share exactly one tag.
