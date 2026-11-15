---
id: DR-process-extract
date: "2026-11-15"
title: "Move the one process runner into loopadmin behind a mandatory host-custody port, with no second runner"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Move the runner's files literally into loopadmin. Ruled out: the old runner's public wrappers imported desk-internal cache and telemetry packages, so a literal move cannot compile outside the desk module, and dropping those imports to make it compile removes cache custody and launch telemetry."
  - "Keep the custody step as an optional callback, or offer an alternate observed entrypoint next to the custodied one. Ruled out: a caller could pick the route without enrollment and start a child outside cache custody. Every public launch route goes through the one admission boundary instead, and a runner built without a custodian refuses."
  - "Leave the runner inside the desk module and give workflows a copy. Ruled out: two runners drift apart, and the process-launch audit and custody guards would have to cover both. The old package is retired in the same increment and no duplicate is kept."
  - "Move cache storage policy, credential resolution or the recorder into loopadmin together with the runner. Ruled out: those meanings have existing desk owners. The process package may not import desk-internal packages, and it reaches custody only through the port."
accepted:
  - "loopadmin/process is the one process runner. Run, RunInteractive and their observed forms all pass through one custody wrapper. A runner built with a nil custodian refuses with ErrNoCustody, and a custodian that admits without an Admission to finish refuses with ErrNoAdmission. Both refusals happen before any child starts."
  - "The desk supplies the custodian from tools/desk/internal/cellrunner, which binds the cache lease and its enrollment telemetry. Within tools/desk, only that package may import the runner. The import guard walks tools/desk only, so another module could import the runner with its own custodian. That gap stays open until the shared supervisor increment widens the guard."
  - "Uncertain process-tree cleanup is reported as uncertain, and the custodian keeps its dirty record until confirmed recovery. A crash or an unproven cleanup is never rounded up to clean."
  - "A nil environment means an empty environment, never permission to inherit the parent's variables. The residue test pins this."
  - "The process-launch audit scans loopadmin, and its one moved exec site is re-keyed. An unresolved argv stays could-not-check, never a forge permit."
  - "Windows containment is cross-compiled and vetted here. Native Windows runtime qualification is owed separately and reported as unavailable, never as passed."
---

**PROPOSED — no ruling is recorded.** `decided-by:` is a placeholder until a human rules on
this record on the brief's decision issue. Until then this record is not an approved
design, and the brief that cites it (graph-execution/40, `design: DR-process-extract`)
may not be accepted on its account (the design-approval gate, `spec/lifecycle-v1.md`
§4.4). The ruling comment must name `DR-process-extract`. The record then gets a
`ruling:` link to that comment, which `statusgen --corroborate` resolves
(`spec/registers-v1.md` §7.5).

## The decision, and the constraint behind it

Process-tree execution is extracted once into the shared `loopadmin` module, so desks and
workflows can share one runner. The constraint is that the move must not open a launch
route that skips cache custody, or one that the process-launch audit cannot see. The
design meets it with one narrow, mandatory port: the caller supplies a `Custody`, the
runner admits every launch through it before the child starts, and finishes the
admission exactly once after the tree is handled. Everything the port stands for (cache
storage policy, credentials, telemetry) stays with its existing desk owner.

The runner move landed before this record. This record is the design that delivery
implemented, written down so the human gate can rule on it. The residue change that adds
this record also adds the nil-environment test and trims the brief's consumer list.

## Why `consequence: major`

If this design is wrong, a caller can start a child outside cache custody or outside the
audit. That weakens a guarded boundary for every launch. A route back exists, though. The
runner is one package, the guards that would catch a second raw caller carry planted
controls, and a corrected port shape is a source change, not a data migration.

## What this record does NOT decide

It does not widen the import guard to modules outside `tools/desk`, which waits for the
shared supervisor increment. It does not qualify native Windows runtime containment. It
does not accept the brief: independent verification of its Verify table and the human's
final acceptance stay separate steps. It does not remove the brief's decomposition-review
`gates:` edge, which is left for its own follow-up change.
