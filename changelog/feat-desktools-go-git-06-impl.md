### Changed
- `deskpr` and `verifyloop`'s durable-Evidence step now push in-process (`gitcore.Push`) with the role App's token held in memory and sent only to the resolved forge's canonical URL. No credential helper, keychain entry or `insteadOf` rewrite takes part, a push that would need force is rejected, and the checkout's pre-push hook still runs first.
- The preflight write-transport check no longer spawns `git push --dry-run`. It proves reachability with an authenticated in-process List of the landing repo and keeps the same clean / failed / could-not-check verdict.
- `count-git-exec.sh` now also counts `exec.CommandContext(<ctx>, "git", ...)` spawns.
