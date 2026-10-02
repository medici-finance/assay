### Changed
- deskmerge reads a pull request's state and its sign-off comment through the in-process forge client, using a token minted for the session's App role and for the repository being read. It no longer shells `gh` under the operator's ambient identity. An unresolvable role, a failed mint or an empty token is a could-not-check, and no request is sent.
- The forge-surface ban script's Go matcher now counts wrapper calls (`runCmd`, `runCmdIn`, `execCommand`, `exec.CommandContext`) whose binary is `gh`, not only direct `exec.Command("gh", …)`.

### Added
- A class guard over the desk tree refuses any new function that reads a forge token (`GH_TOKEN`, `GITHUB_TOKEN` and the like) from the process environment unless a reviewed permit names it.
- A pull request read now reports whether its head lives in the base repository or in a fork, on both the GitHub and GitLab backends.
- Design record `DR-desktools-v2-03` for the read-path custody ruling.
