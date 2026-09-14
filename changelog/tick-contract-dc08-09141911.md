### Added
- `desk-containers/08` — a brief for a **tick contract**: when the harness passes `--tick`
  (or the environment carries `ASSAY_TICK=1`), a desk role runs ONE bounded pass — boot, one
  fresh sweep, act up to its width, wait bounded for what it dispatched, print a
  fixed-grammar summary line, exit — arming no durable wake and asking no human anything.
  The five desk skills are standing loops with no second mode, so a scheduled one-shot run
  of one can only ever end in its own `timeout`, with an empty log; the brief adds the
  missing mode as a contract stated once in a shared reference and derived into all five
  bodies, with the summary-line grammar as its machine-readable verdict channel. The
  standing-window behaviour is unchanged.
