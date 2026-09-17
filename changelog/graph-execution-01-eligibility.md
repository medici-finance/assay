### Added
- `statusgen --eligibility` (`--json` for the full structure): the eligibility
  evaluator computes, per brief, `eligible` / `held` / `eligible-with-notice`
  from its `gates:`/`feathers:`/`depends:` declarations — three-state
  (`satisfied` / `unsatisfied` / `could-not-check`), offline by construction.
- Next-up (`eligibleBase`) and the drive frontier (`briefFrontierState`) now
  read the evaluator's verdict for a brief-v1/v2 brief's `depends:`/`gates:`
  decision, instead of walking `depends:` in isolation — closing a gap where a
  brief-v2 brief fell through to the legacy whole-wave rule and its
  `depends:`/`gates:` were never consulted at all.
- `--lint` NOTICEs a `[eligibility-could-not-check]` line naming any brief
  held by an unresolvable edge (an unpublished cross-repo alias, an absent
  sibling checkout, a forge-backed target), so the gap is visible on a full
  lint run, not only in a dispatcher's output.

### Changed
- The `gates:`/`feathers:` "(reserved, not gating)" `--lint` NOTICE is
  retired: those fields are executed as of this change, so restating
  "reserved" would be false.
- **Behavior change, not just a new field:** a `brief-v2` todo brief stops
  being whole-wave gated. Previously every lower-wave sibling in the same
  stream had to be `done`/`verified` before a v2 brief was eligible; now a v2
  brief is gated by the evaluator's verdict on its own `depends:`/`gates:`
  alone, so an unfinished wave-0 sibling no longer holds it. This is a
  loosening on any brief-v2 tree with unsatisfied whole-wave gating but
  satisfied `depends:` — on this repo's own board it admits two previously
  held briefs (`apps-installer/02`, `desk-supervision/08`) to Next-up.
