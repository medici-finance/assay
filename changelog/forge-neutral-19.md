### Documented
- forge-neutral/19: closes the series #992 tracks by stating, surface by surface, which
  human-only forge actions are genuinely server-side-enforced and which are not. Documents
  that merge-to-protected-branch is currently enforced only by desk-App convention (any App
  holding `pull_requests: write` can already post an approving review and merge — no
  server-side rule stops it), and specifies a `human-approved` required-status-check
  workflow contract to close that gap (workflow file and the required-check ruleset edit are
  named as human/repo-admin follow-on work, not landed here). Confirms workflow-file pushes,
  rulesets, CI variables, and App installs are already server-side-enforced today on both
  GitHub and GitLab (GitLab's `.gitlab-ci.yml` needs a protected-branch + CODEOWNERS rule in
  place of GitHub's dedicated `workflows` permission scope, since GitLab has no scope-level
  equivalent).
