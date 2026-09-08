### Added
- Adopting-Assay runbook now documents Windows, GitLab, and Cursor as a combined install path: a
  forge/harness/OS chooser table, the two Windows install lanes (channel E release vs channel D
  from-source), Windows config-home/PATH/child-process rules, the GitLab parity-vs-provisioning
  split with the Free/CE core-lane stance (ruling #219), group-not-personal-namespace and
  three-credential-classes guidance, and a Cursor copy-skills install (no marketplace). Supersedes
  #650.

### Fixed
- Corrected the channel-D `.assay-versions` guidance: channel D pins the git commit its CI clones
  and rebuilds and does **not** write a `-source` line; the `statusgen-source` / `desk-tools-source`
  pin is the separate release-provenance grammar `<artifact> <tag> <40-hex-commit-SHA>`.
