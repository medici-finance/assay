### Fixed
- `harnessgen`, `skillslint` and `plugindrift` remediation text, and the docs that echo it, now print the module-scoped command (`cd tools/<tool> && go run . … --root ../..`) instead of the root-relative `go run ./tools/<tool>`, which fails with `go.mod file not found` in this multi-module tree. A new test keeps the root-relative form from coming back.
