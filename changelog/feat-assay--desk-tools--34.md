### Changed
- The pinned golangci-lint ratchet's debt baseline is re-measured against the current analysis pins (Go 1.27.1 package loader): 30 modules, 36 scans, 13,487 recorded findings. Records whose identity and diagnostic did not change keep their first-observed date.
- The runner drops staticcheck "related information" notes before identity, so two notes at one position no longer make the gate refuse an ambiguous duplicate. A note with no primary finding of its rule still refuses. The fingerprint contract is now `source-window/v3`.
- After merge, the human CI runsheet fetches main, pins a reviewed main commit and pushes a workflow-only `ci/go-lint-enable` branch for a normal pull request. Its shell blocks run under `set -eu`, so a failed guard stops the block.
- The runsheet and docs record that this change is the reviewed baseline migration: the branch run refuses by design, and the CI job is applied after merge.
