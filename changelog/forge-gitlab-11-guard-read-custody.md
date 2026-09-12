### Added
- forge-gitlab briefs 11 and 12 + design record `DR-forge-gitlab-11`: the token-custody design that
  closes forge-gitlab/08's shell-exec ban to zero — `deskroster` reads as the session role,
  `repohardenguard` reads as a dedicated read-only `auditor` identity through one enumerated
  `RepoHardeningRead(repo, kind)` op over a closed kind set (GitHub kinds in 11, GitLab kinds in 12).
  Authoring only; the human gate on 11 decides the custody shape before any code lands.
