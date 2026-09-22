### Changed
- harness: pin the v1.0.24 desk-tools image digest (`plugins/assay/paired-versions.yaml`
  `harness.tag`/`harness.digest`), replacing the fail-closed `PENDING-HARVEST` placeholder now that
  the image has been published and its digest harvested from two independent registry reads.
- `check-paired-versions.sh`'s single-tag assertion stays as it is on `main` (no harness
  exemption) per the maintainer's ruling on #1479 (option 1): the fix is a real `v1.0.24`
  release of `statusgen`/`desk-tools`, re-pinning every section of `paired-versions.yaml` back
  onto one tag, rather than carving the harness image out of the rule.
