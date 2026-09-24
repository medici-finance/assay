### Added
- `deskautolane`, the verb of a narrow auto-approve lane: a PR is admitted by category (every
  changed path inside an area a named human opted in, no tripwire) and ejected, one-way, by a
  demotion score recomputed at each gate. It ships inert: the lane is closed unless all four new
  `ASSAY_AUTOAPPROVE_*` roster keys are set, every write it can make requires a signed `R-8`
  ruling line resolved to the blessing authority, and this release carries no merge mutation
  (`merge --dry-run` reports what a merge would need and writes nothing; `merge` refuses at
  `merge-write`). The enactment gate reads the register through the forge at the default
  branch and requires the sign-off comment, on a thread in the register's own repo, to be a
  User's explicit `Enact: R-8` acceptance with no rejection in it. An ejection latches both in
  the local audit log and through the reviewer App's marked comment on the PR, and the lane
  admits only PRs based on the default branch.
- The roster keys `ASSAY_AUTOAPPROVE_AREAS`, `ASSAY_AUTOAPPROVE_EJECT_LINE`,
  `ASSAY_AUTOAPPROVE_FPY_FLOOR` and `ASSAY_AUTOAPPROVE_DAILY_CAP` are recognised by both the desk
  tools and statusgen, so a roster that sets them no longer refuses the whole configuration.

- deskkit's `Account` carries the forge's actor `Type` where the read reports one (GitHub's
  comment read), so a gate can refuse an App or Bot artifact.

### Changed
- deskflip's checks-green conclusion set now delegates to the shared `deskkit.ConclusionGreen`,
  the same set the lane's `ci-nonsuccess` signal reads. Behaviour is unchanged.
- `deskautolane`'s App-token condition mints through `deskkit.GitHubRoleToken`, the forge-aware
  GitHub arm, instead of calling the raw App minter. The token, its scope and its custody file
  are unchanged; a repo the roster binds to another forge is now refused before any mint.
