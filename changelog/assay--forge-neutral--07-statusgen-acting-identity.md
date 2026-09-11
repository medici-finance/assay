### Added
- **`statusgen` recognises the acting FORGE identity (forge-neutral/07).** The roster parser now
  accepts the forge-qualified `[role=]<forge>:<slug-or-login>[:<id>]` grammar (unqualified reads as
  github, recorded as inferred), mirroring the desk-tools reader. The Evidence-actor lint matches an
  Evidence committer by the accepted verifier's forge — id-pinned on GitHub, GitLab service-account
  address shape + username on GitLab — so a correctly-verified GitLab row reads BACKED instead of the
  false "0 rows are backed" the GitLab pilot hit. A verifier bound to a forge the build does not
  understand is could-not-check naming the forge, never a pass.
- **The `verifyrun` execution witness names the acting forge identity.** The witness `Runner` now
  resolves the git identity through the roster for the repo's forge ahead of the CI-env and git-config
  fallbacks, and records which source produced it (forge-identity / ci-env / git-config), so a stamped
  acting identity is distinguishable from a host-derived one. The no-identity and forbidden-runner-flag
  refusals are unchanged: the runner stays derived, never caller-supplied.
