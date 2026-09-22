### Changed
- harness: pin the v1.0.24 desk-tools image digest (`plugins/assay/paired-versions.yaml`
  `harness.tag`/`harness.digest`), replacing the fail-closed `PENDING-HARVEST` placeholder now that
  the image has been published and its digest harvested from two independent registry reads.
- `check-paired-versions.sh`'s single-tag assertion stays as it is on `main` (no harness
  exemption) per the maintainer's ruling (option 1): re-pin `statusgen`/`desk-tools` to a real
  `v1.0.24` release instead of carving the harness image out of the rule.
- `statusgen:`/`desk-tools:` re-pinned to the published `v1.0.24` release — tag and every
  per-platform sha256 harvested from that release's `checksums.txt` — so all three
  `paired-versions.yaml` sections share one tag again and `check-paired-versions.sh` passes
  unchanged.
