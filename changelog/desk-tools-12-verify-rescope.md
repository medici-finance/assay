### Changed
- `desk-tools/12`: re-scoped Verify row 8's whole-directory `gofmt -l statusgen` to the brief's touched files (`statusgen/briefinfo.go statusgen/briefinfo_test.go`), so the row stops failing on the unrelated pre-existing `statusgen` files that flag only under a newer local gofmt (#555's module-wide drift); CI still runs the whole check.
