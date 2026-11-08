### Fixed
- `deskevidence` refuses an Evidence append whose latest verdict is BLOCKED or could-not-check on a brief that is already verified or done, naming the brief, its status and the verdict; PASS and FAIL still land. The check is the new `statusgen evidenceverdict` subcommand, which reads the latest verdict with the same selector as the verified-flip check.

### Added
- `deskevidence --outcome-record <file> --outcome-only` records such a run in the outcome record only. The record names no Evidence commit, states `evidence_withheld`, and is accepted only when the Evidence append would be refused.
- `verifyloop` marks verified and done briefs outcome-only, so a dispatched verifier lands a non-decisive result as an outcome record and opens no Evidence PR.
