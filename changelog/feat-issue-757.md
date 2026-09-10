### Fixed
- **`deskdispatch` now accepts the Windows worktree home `deskwt` produces, so dispatch
  works on native Windows.** After `deskwt add` succeeded, the `worktree-create` step
  validated the reported home with a POSIX-only leading-slash test (`strings.HasPrefix(home,
  "/")`), which rejected the drive-rooted `C:\...\.claude\worktrees\...` path `deskwt`
  legitimately selects on Windows — aborting every dispatch (no worker, reviewer, or verifier
  could be started). The check now uses a portable absoluteness test (matching the one
  `brief.go` already uses), so the producer (`deskwt`, which picks the Windows prefix) and the
  consumer (`deskdispatch`, which accepts it) judge "absolute" the same way per OS. This is
  the consumer half of the earlier `deskwt`-side portability fixes.
- **A refused `worktree-create` no longer leaves a phantom held claim.** This abort happens
  after the durable dispatch claim is acquired; it now releases that claim the same way the
  adjacent `deskwt add`-failed branch does, instead of returning with the claim still held —
  which had wedged the item behind a claim nobody was acting on until a human hand-deleted the
  ref. A new regression test pins that `deskwt` picking the Windows prefix and `deskdispatch`
  accepting it are one contract.
