### Changed
- deskmerge reads a pull request's state and its sign-off comment through the in-process forge client, using a token minted for the session's App role and for the repository being read. It no longer shells `gh` under the operator's ambient identity. An unresolvable role, a failed mint or an empty token is a could-not-check, and no request is sent.
- A typed comment-thread read now walks a pull request's whole comment thread, as it already did for an issue, so a sign-off posted after the first 100 comments is found instead of being refused as deleted. A thread longer than the page cap, or a next page with no cursor, is a could-not-check. This covers deskmerge's and deskclose's sign-off reads.
- deskmerge's exit codes on its sign-off and pull-request reads: a sign-off comment that is no longer on its item exits 5 (refused), not 6; a pull request whose head repository was deleted exits 6 (could-not-check) instead of being refused as a fork; a `/pull/N` link whose number is an issue exits 6 with "could not resolve".
- The forge-surface ban script's Go matcher now counts wrapper calls (`runCmd`, `runCmdIn`, `execCommand`, `exec.CommandContext`) whose binary is `gh`, not only direct `exec.Command("gh", …)`.

### Added
- A class guard over the desk tree refuses any new function that reads a forge token (`GH_TOKEN`, `GITHUB_TOKEN` and the like) from the process environment unless a reviewed permit names it.
- A class guard refuses any shipped function that looks a comment up by id on the untyped first-page comment read; such lookups must use the typed read, which walks the whole thread.
- Rule-register rows `R-ambient-token-read` and `R-signoff-read-whole-thread` in `docs/contracts.md`.
- A pull request read now reports whether its head lives in the base repository or in a fork, on both the GitHub and GitLab backends.
- Design record `DR-desktools-v2-03` for the read-path custody ruling.
