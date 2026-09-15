### Fixed
- `deskpost review` now refuses a body that carries a `Security-Review:` line, mirroring the
  refusal `deskpost security-review` already applied to a body carrying a `Verdict:` line. The
  guard was one-directional: handed a security-lane body — which happened when two reviewer
  lanes dispatched to one PR shared a scratchpad and one lane's default body filename was read
  by the other — `review --verdict approve` submitted a `Security-Review: pass` as an
  **APPROVED** review. That is the exact same-head APPROVE shape the verb split exists to keep a
  security pass out of (a pass posts as COMMENTED so the flip gate can read it while GitHub's
  review roll-up, and any standing CHANGES_REQUESTED from the shared App, are left alone). The
  stray review had to be dismissed by hand. The refusal is exit 5 before any network call and
  names the verb to use; `--verdict request-changes` with a security body is refused the same
  way. The refusal uses the same emphasis-tolerant reader the flip gate uses, so a body carrying
  `Verdict: approve` plus `**Security-Review: pass**` — invisible to the strict write-side parse
  but read downstream as a security pass — is refused too; a `> `-quoted citation of the other
  lane's line still posts. Existing tests that posted security verdicts through `review` now use
  `security-review`.
