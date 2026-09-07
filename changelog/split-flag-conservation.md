### Added
- **Split-flag conservation gate** — splitting a brief may no longer silently
  DOWNGRADE its risk. A child brief must carry at least as strict a `gate` and at
  least as high each of the four canonical `risk` answers as the brief it was
  split from (gate = the stricter of parent and child; risk = MAX per key). A
  human-gated, irreversible parent split into a `gate: model`, everything-`no`
  shard — the move where a human gate is most likely to evaporate, because risk is
  a property of what the change does, not the size of the diff — is now a hard
  `statusgen --lint` PROBLEM (`splitflags.go`). The parentage is read from
  whichever signal is present: the numeric-stem convention (`02a` is a shard of
  `02`; when `02` was retired in the same change the strictest sibling shard sets
  the floor, so a faithful `02b` catches a downgraded `02a`/`02c`), or a new
  optional `split-from: <stream>/<NN>` `brief-v1` frontmatter key for splits whose
  lineage is not in the numbering (across streams, or renumbered). Fails safe
  toward more gating; three-state (an unresolved `split-from` is a
  `could-not-check` NOTICE, never a silent pass).
