### Added
- `deskautolane`, the verb of a narrow auto-approve lane: a PR is admitted by category (every
  changed path inside an area a named human opted in, no tripwire) and ejected, one-way, by a
  demotion score recomputed at each gate. It ships inert: the lane is closed unless all four new
  `ASSAY_AUTOAPPROVE_*` roster keys are set, every write it can make requires a signed `R-8`
  ruling line resolved to the blessing authority, and this release carries no merge mutation
  (`merge --dry-run` reports what a merge would need; `merge` refuses at `merge-write`).
- The roster keys `ASSAY_AUTOAPPROVE_AREAS`, `ASSAY_AUTOAPPROVE_EJECT_LINE`,
  `ASSAY_AUTOAPPROVE_FPY_FLOOR` and `ASSAY_AUTOAPPROVE_DAILY_CAP` are recognised by both the desk
  tools and statusgen, so a roster that sets them no longer refuses the whole configuration.

### Changed
- deskflip's checks-green conclusion set now delegates to the shared `deskkit.ConclusionGreen`,
  the same set the lane's `ci-nonsuccess` signal reads. Behaviour is unchanged.
