### Added
- GitLab Ultimate refinements (forge-gitlab/06): `create-fleet-gitlab.sh --tier ultimate` scripts a custom reviewer role that cannot push (Reporter base + `admin_merge_request`) and registers an external-status-check verdict lane; each no-ops as could-not-check on a lesser instance rather than downgrading silently.
- The GitLab forge seam can post lane verdicts as an external status check against the MR head SHA at Ultimate, with a three-state (could-not-check) fallback when the tier does not expose the endpoint.

### Changed
- `docs/adopting-assay-gitlab.md`: the Ultimate section moves from a human-only checklist to the scripted `--tier ultimate` path with verification commands (the reviewer-role negative push test and the required-status-check check).
