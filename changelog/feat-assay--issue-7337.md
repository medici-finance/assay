### Changed
- The public installer pairing (`plugins/assay/paired-versions.yaml`) now selects the published v1.0.90 statusgen, desk-tools and harness image, with every binary hash taken from the v1.0.90 `checksums.txt` and the image digest from its `release-images.json`; the coupled `containers/base/Dockerfile` desk-tools image pin moves with it.
