### Added
- **Test policy independent of any one brief (`docs/test-policy.md`).** A methodology-level
  policy that guards the baseline the per-brief Verify table does not: it names the four test
  **tiers** (unit / integration / live / drill) and maps each onto the existing Verify row
  `Class` vocabulary, states the **regression floor** as a property (a merged change may not
  reduce the set of behaviours the standing suite asserts) and explains why a coverage
  percentage is not that floor, classifies **flakes** into three actioned classes with no
  "ignore" bucket (a quarantined test reports `could-not-check`, never pass), defines the
  **standing truth suite** as the CI-owned baseline distinct from delta-probing Verify rows,
  and adds **plan-in-PR** for effort-M/L briefs as a rework signal that is not a gate.
- **Standing truth-suite workflow.** Runs the repo's test corpus plus the release mutation
  gate on push to the default branch and on a daily schedule, reporting three-state.
  Delivered staged at `ci/staged-workflows/truth-suite.yml` because no App holds
  workflow-push permission; a maintainer promotes it to `.github/workflows/truth-suite.yml`
  to activate it.
- Cross-reference from `docs/brief-rules.md` and an honesty note in `docs/how-assay-works.md`
  recording that a Verify table proves the delta, not the baseline.
