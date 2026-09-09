### Fixed
- `statusgen --lint` no longer reds on main after a release. The harness-portability/15 brief
  named its changelog fragment as a backticked path, and the v1.0.0 roll aggregated that fragment
  into `CHANGELOG.md` and cleared the directory — leaving a backticked path to a file that is gone
  by design. The brief's deliverable claim and its Verify row now point at the delivered content
  in the `v1.0.0` section instead of at the consumed file, so the claim survives the roll that
  fulfils it. The class — a deliverable reference that a release deletes — is under
  needs-decision on #722.
- Restored a v1.0.0 release-note entry that went missing when the harness-portability/15
  implementation PR overwrote the brief-authoring PR's changelog fragment instead of adding
  alongside it; the fragment was rolled up in its overwritten state, so the spec entry never
  reached the published notes.
