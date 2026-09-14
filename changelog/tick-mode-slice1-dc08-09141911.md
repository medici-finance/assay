### Added
- **A tick contract for the five desk roles** — `plugins/assay/references/tick-contract.md`.
  When the harness passes `--tick`, or the environment carries `ASSAY_TICK=1` (compared
  exactly), a desk role runs ONE bounded pass — boot, one fresh sweep, act up to its width,
  wait bounded for what it dispatched, print a summary line, exit — arming no durable wake,
  scheduling no cadence and waiting in line for no answer. Absent both spellings every run is
  a standing window and behaves exactly as before, so the contract is inert until a caller
  asks for it. The five desk bodies each gain a short `## Tick mode` section, derived from one
  declared guardrail block rather than hand-copied, so the gating plugin-tree lint keeps all
  five byte-identical.
- **`plugins/assay/scripts/tick-summary.sh`** — the one executable form of the summary-line
  grammar (`tick role=… outcome=… swept=… acted=… filed=… duration=…`), with `regexp`,
  `validate` and `check` verbs, plus its hermetic case suite. Its cross-field rules are what
  keep the line honest: `could-not-check` requires `swept=-` and `noop` forbids it, so a pass
  that could not read its queue cannot produce a well-formed "queue was empty" line.

### Changed
- The desk-role bodies now say what a scheduled, one-shot invocation should do. Previously
  they described only the standing window, so a role invoked as a bounded job could end only
  in its own deadline, with nothing printed.
