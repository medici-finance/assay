### Added
- `deskrun` — a new outward-write verb that starts a workflow run (`deskrun <owner/repo>
  <workflow> --ref <r> [-f k=v …]`), clears ONE named deployment gate on it (`deskrun approve`),
  or reads its lifecycle (`deskrun status`), under a per-repo **run credential** the roster binds
  in the new `ASSAY_RUN_CREDENTIALS` key. The dedicated `release-runner` credential proceeds; a
  repo bound to `human:<name>` is refused (exit 5) before anything is minted, so no ambient
  `gh`/`glab` credential is ever read; an unbound repo is could-not-check (exit 6).
- Three forge-seam ops on both backends — `RunWorkflow`, `ApproveGate`, `RunStatus` (inventory
  rows 49–51). GitHub resolves the run a dispatch created by a correlation read and refuses an
  ambiguous match rather than guessing; an approval is resolved against the run's pending gates
  and refuses a name that matches none. On GitHub an App credential cannot approve a
  required-reviewer gate (approval needs `Deployments: write`, and required reviewers are users
  or teams), so `deskrun approve` there reports could-not-check and writes nothing.
  GitLab starts pipelines with the pipeline trigger token and approves on the gate shape the roster declares (`manual-job` or `environment`).
- `desktoken` recognises the `release-runner` role (its own App on GitHub; on GitLab a trigger
  token, which `desktoken` never tries to self-rotate).
