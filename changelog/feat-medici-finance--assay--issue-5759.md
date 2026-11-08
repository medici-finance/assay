### Fixed
- `deskevidence` refuses an Evidence append on a brief that is already verified or done when its latest verdict is any verdict other than PASS or FAIL (BLOCKED, NEEDS-CONTEXT, PARTIAL, HELD and the rest), naming the brief, its status and the verdict; PASS and FAIL still land. The status is read from the target branch's board at its tip, not the verifier's older home. The check is the new `statusgen evidenceverdict` subcommand, which reads the latest verdict with the same selector as the verified-flip check and returns could-not-check when it cannot resolve the brief's status.

### Added
- `deskevidence --outcome-record <file> --outcome-only` records such a run in the outcome record only. The record names no Evidence commit, states `evidence_withheld`, keeps the receipt-completeness checks of every new record, and is accepted only when the Evidence append would be refused.
- `verifyloop` marks verified and done briefs outcome-only, so a dispatched verifier lands a non-decisive result as an outcome record and opens no Evidence PR.
