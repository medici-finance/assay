### Changed
- The board lint now treats the D1 MUTATION obligation as a MERGE GATE, not an
  advisory notice (mistake-proofing/06). When this branch's diff changes a
  check-shaped control a brief declares, that brief MUST carry a `+mutation`
  Verify row or the lint refuses — promoting the methodology's sharpest
  requirement (a control must be shown to fire) from a prose MUST to a machine
  gate. flow and dereference stay advisory; only mutation is promoted. The gate
  is transition-scoped by construction — only a brief whose own file the branch
  edited is evaluated — so the 300-plus inherited tables are never made fatal, and
  a diff whose branch base cannot be read fails closed (could-not-check refuses,
  distinct from "nothing owed").
- The check-shaped path set is now an explicit, narrow, rationale-carrying
  ENUMERATION in source (lint/check source, desk guard, CI workflow, reviewed
  verify script) rather than one over-firing inline regex, with its coverage
  boundary recorded beside it. The failure message names the triggering path,
  points at the `tools/desk/cmd/muhar` mutation harness as the recommended way to
  produce the demonstration, and states that it checks the row's PRESENCE not its
  adequacy — it adds a floor and does not replace the reviewer. This check carries
  its own positive control (the rule applied to itself).
