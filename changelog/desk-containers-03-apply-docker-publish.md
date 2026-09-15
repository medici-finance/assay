### Added
- `.github/workflows/docker-publish.yml` now also builds and publishes the
  shared `desk-base` image and the five per-desk images (`intake-desk`,
  `worker-desk`, `pr-review-desk`, `verify-desk`, `the-desk`), version-locked
  to a single build's tags and gated by `containers/scripts/layer-secret-scan.sh`
  before any push (brief desk-containers/03 Task 2). This content had been
  parked at `docs/streams/desk-containers/pending-docker-publish.yml` since
  the worker App's token cannot push a `.github/workflows/*` change; applied
  by hand onto the live workflow file (which carries no other changes since
  the content was parked) and the parked file removed.
