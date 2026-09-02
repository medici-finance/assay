### Added
- `statusgen` gains typed Verify-row OBLIGATION classes (`+mutation`, `+flow`,
  `+dereference`, `+neighbour`) as a second closed set on the `Class` cell,
  orthogonal to the existing WHO-EXECUTES values and encoded in a compound
  `<execution> +<obligation>` cell so the table's column set is unchanged and the
  legacy column-less hinge is untouched; an unknown obligation token is FATAL
  exactly as an unknown class is. A diff-scoped derivation
  (`statusgen/obligationderivation.go`) reuses the existing consumer-routing
  branch-diff helper — no new diff machinery, no network reach — to evaluate only
  a brief whose own file the branch edited, deriving owed obligations from the
  branch diff, declared paths, and task prose and emitting an advisory NOTICE for
  each owed-but-absent obligation; an unavailable diff is reported as
  could-not-check, never as "nothing owed". Presence is the control; adequacy
  stays the reviewer's call. Lands advisory (`obligationDerivationFatal = false`)
  per the phasing recorded in the source header (mistake-proofing/03).
