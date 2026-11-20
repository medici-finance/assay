### Added

- `deskread` has two new kinds for the GitLab auto-flip. `commit-changes --repo R --sha S` lists the changes that contain a commit, in any state (`ListCommitChanges`), and an empty list is printed as `[]`. `change-trust --change R#N` reports when a change's description was last edited (`PRTrustEvents`). The `reviews` kind now includes each entry's `body`, left out when it is empty, so a reader can tell a GitLab approval, which has no body, from a "Verdict: approve" note.

### Changed

- `statusgen --auto-flip-model` on a GitLab repo no longer reports a standing could-not-check. It reads the merge request through `deskread`: commit-to-change resolution, the change record and file list, approvals, verdict notes and the description-edit time. A verified `gate:model` brief flips only when the merge request carries the reviewer identity's approval and an approve-verdict note by that identity (a whole `Verdict: approve` line) naming the merged head SHA, with no later request-changes note by that identity. A security pass, a relay or any other note that merely names the head does not count. Any truncated, partial or unreadable read is a could-not-check, and a missing approval or note is a named refusal; neither flips. (#7202)
