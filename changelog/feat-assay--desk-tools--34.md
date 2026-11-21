### Changed
- The pinned golangci-lint ratchet's debt baseline is re-measured against the current analysis pins (Go 1.27.1 package loader): 30 modules, 36 scans, 13,435 recorded findings. Records whose identity and diagnostic did not change keep their first-observed date.
- The runner drops staticcheck "related information" notes before identity, so two notes at one position no longer make the gate refuse an ambiguous duplicate. A note with no primary finding of its rule still refuses. The fingerprint contract is now `source-window/v3`.
- The human CI runsheet fetches and pushes this brief's branch.
