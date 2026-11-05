### Added
- Install mikefarah yq v4.54.1 in the verify-runner image, fetched over https and checked against a per-architecture sha256 pin before install, so rows that call `yq` can run offline; the release capability probe now runs `yq` and requires its version line to name mikefarah, and a static test rejects an unpinned, unchecked or second yq.
