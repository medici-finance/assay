### Added
- A reference, `plugins/assay/references/intake-handoff.md`, now carries the whole intake handoff procedure: when a judged issue needs a recorded and published `intake-handoff-v1` companion, the judgment table with one record per row, the `scanloop land` command, the publication step (`handoff:` pointer and quoted `handoff-opened:` time), the read-back through the uncapped dispatch queue, and the failure handling. Its examples are checked in CI against the shipped assessment, and a planted disagreeing example proves the check fails.

### Changed
- The intake-desk skill's accountable-handoff passage is shortened to the rule and a pointer to that reference, and now says an issue routed by judgment is not routed until its handoff is recorded and published. The scrub rule for a companion's free-text fields stays in the skill body, and the generated harness copy is regenerated to match.
