### Fixed
- The `tools/desk` engine tests no longer redden under whole-module `go test ./...` load: every wait for `loopengine.Run` in `internal/loopengine` and `cmd/commsloop` now uses a 60-second wedge ceiling instead of a 2-5 second one, and a new module-root test keeps short literal deadlines out of every test that drives the engine.
- Five `tools/desk` files that were not gofmt-formatted are formatted, and a new module-root test fails on any unformatted Go file in the module, so formatting drift no longer lands silently.
