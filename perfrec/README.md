# perfrec

The one writer of the opt-in, local-only per-invocation performance record. The
desk tools (through their shared kit) and `statusgen` both call it, so the
schemas, the `<state dir>/perf/<UTC day>.jsonl` layout, the 7-day retention and
the install id each exist once.

- **Off by default.** `ASSAY_TELEMETRY` must be exactly `1`; otherwise nothing
  is created.
- **Local only.** Standard library only; the dependency closure carries no
  network or process-launch package (`deps_test.go`), and an external consumer's
  closure carries no network, process-launch, credential or desk package
  (`consumer_test.go`).
- **No free text.** Every string field is a constant, a roster member the
  caller compiled in, a hex build stamp, or random hex; perf-v2 labels also pass
  `Label`.
- **Never load-bearing.** A failed write is swallowed; it cannot change a
  caller's exit code.

## Two schemas

- `perf-v1` (`Schema`, built by `New`) is the legacy record, frozen: its fields
  and meanings never change. Boot and launch steps and `statusgen` still write it.
- `perf-v2` (`SchemaV2`, built by a `Session`) measures wall and CPU over one
  declared boundary and records counters with explicit coverage. The desk
  tools' `PerfRun` writes it.

`Record` marshals and unmarshals strictly per the schema its `schema` field
names: a perf-v1 record carrying a perf-v2 field (or the reverse) is refused,
and an invalid perf-v2 line does not decode. The field tables, the boundary and
exclusion statements and the reader rules live in `docs/telemetry.md`.

## Recorder API (perf-v2)

```go
s := perfrec.Begin(perfrec.SystemProbe(), perfrec.ProcessEntry(), []string{"load", "render"})
stop := s.Phase("load") // fixed tool-owned roster; any other name records as "other"
// ...
stop()                                // idempotent
s.Add(perfrec.CounterHTTP, n)         // a measured count
s.Observe(perfrec.CounterSubprocess)  // a measured zero: the seam was in place
rec := s.Finish(tool, verb, role, exitCode, buildSHA) // ends the measurement
if perfrec.Enabled() {
	perfrec.Append(rec) // best effort, outside the measurement
}
```

- `ProcessEntry()` is the `process-init` mark taken while this package
  initialises; `Probe.Mark()` takes a `caller` mark instead.
- Counters never `Add`ed or `Observe`d are recorded `unavailable`, never `0`.
- `Finish` reads the end wall time then the end CPU sample, back to back;
  `Append` (encode, install id, prune, write) happens after it.
- A nil `*Session` is valid and does nothing.
- `Decode(line)` reads one line of either schema.

## Using it from another module

Consumers `require` this module at the placeholder version and `replace` it
with the relative sibling path, exactly like `forgeread`:

```
require github.com/medici-finance/assay/perfrec v0.0.0-00010101000000-000000000000

replace github.com/medici-finance/assay/perfrec => ../perfrec
```

Build and test with `GOWORK=off`, so the `replace` — not a workspace — decides
which directory is used. `consumer_test.go` builds a synthetic external module
this way and checks it resolves this directory.

### Adoption checklist for a standalone tool (statusgen/26, qualgen, others)

1. Wire the module with the `require` + relative `replace` above; confirm
   `GOWORK=off go list -m -f '{{.Replace.Dir}}' github.com/medici-finance/assay/perfrec`
   names this directory.
2. Compile in a fixed tool key, verb roster and (optionally) phase roster; pass
   every label through your own roster first — `Label` is the last gate, not
   the mapper.
3. Start from `ProcessEntry()` unless the tool has a reason to measure from a
   later `caller` mark; say which in its docs.
4. `Add` or `Observe` only the counters your existing seams really see; leave
   the rest unavailable. Never add a counter by starting work (a credential,
   provider, network or process call) the tool would not otherwise make.
5. Call `Finish` before any output that is not part of the measured work, and
   `Append` only when `Enabled()`; never let the record change an exit code or
   a byte of output.
6. Test switch off creates nothing and changes nothing; switch on writes one
   record that `Decode` accepts.
