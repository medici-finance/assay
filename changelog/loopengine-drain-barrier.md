### Fixed
- `tools/desk` loopengine `TestDrain` no longer fails under load. It proved the pool runs items concurrently by sampling the in-flight peak after a short sleep. Each drill item now waits on a bounded barrier until a second item is in flight, so a correct pool always passes and a pool that runs items one at a time fails after 3s instead of hanging (#1951).
- A new module-root test, `tools/desk/peakassert_test.go`, fails on any `_test.go` file that asserts a sampled concurrency peak (`if max… < 2`). The two remaining sites are allow-listed and tracked for conversion.
