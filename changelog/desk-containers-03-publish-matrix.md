### Added
- Authored the `docker-publish.yml` `desk-images` job (builds and publishes the
  shared `desk-base` image and the five per-desk images — `intake-desk`,
  `worker-desk`, `pr-review-desk`, `verify-desk`, `the-desk` — version-locked
  to the same base tag, alongside the existing `desk-tools` image, gated by
  `containers/scripts/layer-secret-scan.sh` against all six images before any
  push). Parked at `docs/streams/desk-containers/pending-docker-publish.yml`
  pending application by a human with `workflows` permission — the worker
  App's push of `.github/workflows/*` is rejected on this repo.
