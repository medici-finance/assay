### Fixed
- `scripts/go-analysis.sh` keeps its cache outside the checkout: the default is a per-checkout directory under the user cache, and a `--cache` path inside the analyzed tree is refused. Tests that walk the checkout no longer read the toolchains and module cache.
- `go-analysis acquire` no longer writes to the checkout. Each module's `go mod download` runs against a copy of its go.mod and go.sum in the cache, and acquisition fails if the work tree changed while it ran.
