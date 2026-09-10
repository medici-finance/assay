### Fixed
- De-flaked `cmd/fanoutloop` `TestPool` (and its sibling engine-integration tests) so a
  whole-module `go test ./...` no longer reddens intermittently under CPU load. The tests'
  wall-clock wedge safety-nets were calibrated to unloaded speed (5s); under the saturation of a
  full-module run their near-instant async conditions missed the deadline even though nothing was
  wedged. The deadlines now route through one load-tolerant `engineTestTimeout`, removing the
  timing/load assumption without weakening any assertion.
