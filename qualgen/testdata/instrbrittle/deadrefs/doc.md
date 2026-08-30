# Instruction doc fixture — planted reference set

This doc plants a KNOWN, fixed mix of references so the reference-validity
test can dereference the pass against a known answer rather than merely
checking that something got flagged. It carries EXACTLY five references, no
more, no fewer — do not add a sixth backtick span to this file.

- a live file-path reference: `lib/live.go` (exists in this fixture's tree)
- a dead file-path reference: `lib/ghost.go` (never existed)
- a dead symbol reference: `GhostFunc()` (never defined anywhere but here)
- a dead typed-ID reference: `TASK-9999` (never recorded anywhere but here)
- an unclassifiable reference: `some vague thing` (not a path, symbol, or
  typed ID — reported could-not-measure, distinct from measured-dead)
