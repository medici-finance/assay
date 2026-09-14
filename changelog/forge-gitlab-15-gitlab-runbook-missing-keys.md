### Fixed
- GitLab runbook (`docs/adopting-assay-gitlab.md`): named the `ASSAY_REPO_FORGES` forge-binding
  key beside the existing boot-time keys, added a dedicated board-push-credential subsection for
  `STATUSGEN_PUSH_TOKEN` (token kind, scope, minimum role, variable visibility) and corrected the
  runners section's pointer, which previously sent readers to the wrong section for the wrong
  credential, and added a source-pin-lane subsection cross-referencing the other forge's runbook
  for GitLab-plus-native-Windows adopters.
- Generated GitLab CI scaffold (`statusgen/init.go`): the `STATUSGEN_PUSH_TOKEN` comment now
  points at the new board-push-credential subsection by name.
