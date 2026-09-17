### Added
- `topology.yaml` gains a strict-parse `comms:` key (`tools/desk/internal/topology`'s
  `CommsMode`) — one of the three independent off-switches (topology key, `ASSAY_COMMS_*`
  env, deployed gateway) the cell-comms enablement contract requires. Absent reads as
  disabled; an unrecognised value is a parse error naming the line; the key is declarative
  only and wires nothing on its own.

### Changed
- `commsgw`'s README documents the full three-part enablement contract and records the
  2026-09-17 human ruling on this cutover decision (Option 2, "Interim rung first", over
  the recorded full-enable target): receive-and-route live, every execution a proposed
  dispatch a person fires, full autonomous enablement not implemented by this change.

### Tests
- `cmd/commsgw`: `TestInertWithoutAllKeys` proves each `ASSAY_COMMS_*` key refuses
  individually, not just all-absent.
- `internal/topology`: `TestTopologyComms` / `TestTopologyCommsPositiveControl` pin the
  `comms:` key's parse and drift-detection behaviour.
