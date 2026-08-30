# Instruction doc fixture — planted doc-drifts-from-code history

This doc is committed together with the code file it describes exactly ONCE,
establishing a §4.5 change-coupling pair. The test harness then commits N
further changes to that code file alone, with this doc left untouched — the
planted doc-drifts-from-code history the staleness tests dereference: the
pass must report the code-only change count as exactly N and flag the pair
presumptively stale.
