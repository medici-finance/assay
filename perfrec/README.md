# perfrec

The one writer of the opt-in, local-only per-invocation performance record
(`perf-v1`). The desk tools (through their shared kit) and `statusgen` both
call it, so the schema, the `<state dir>/perf/<UTC day>.jsonl` layout, the
7-day retention and the install id each exist once.

- **Off by default.** `ASSAY_TELEMETRY` must be exactly `1`; otherwise nothing
  is created.
- **Local only.** Standard library only; the dependency closure carries no
  network or process-launch package (`deps_test.go`).
- **No free text.** Every string field is a constant, a roster member the
  caller compiled in, a hex build stamp, or random hex.
- **Never load-bearing.** A failed write is swallowed; it cannot change a
  caller's exit code.

The field table and the two-switch rule for any future remote sink live in
`docs/telemetry.md`. Consumers `require` this module at the placeholder
version and `replace` it with the relative sibling path, exactly like
`forgeread`.
