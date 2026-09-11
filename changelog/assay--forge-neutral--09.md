### Added
- **Per-forge leak-gate verdict surface** (`docs/streams/forge-neutral/leak-gate-shape.md`):
  where the disclosure verdict lands on each forge, whether it blocks, and the three-state
  contract — a missing verdict reads as could-not-check, never as a pass.
- **Pipeline-side leak-sweep job** for GitLab CI
  (`docs/streams/forge-neutral/gitlab-ci-half.md`): the free-tier compensator that runs the
  in-tree controls in the change's own pipeline and fails it, the blocking layer on GitLab CE.
- **`cellctl` is forge-aware**: `cellctl new --forge github|gitlab`, with `--deskd-app-pem`
  and `--orgs` required on the github path only and a hand-provisioned GitLab role token store
  on the gitlab path; `deskd` and `check` follow per forge, and the hardcoded `api.github.com`
  host is replaced by the cell's configured forge endpoint.

### Changed
- **`deskflip` reads an absent required leak-gate verdict as could-not-check**: an otherwise-green
  rollup that is missing a branch-protection-required context (the `leak-sweep` status among them)
  no longer flips ready — absence of the verdict is never "no objection".
