### Added
- New shared guardrail `default-forward-reversibility` across all five desk-role skills (the-desk,
  worker-desk, pr-review-desk, verify-desk, intake-desk) and `.claude/guardrails/GUARDRAILS.md`. It
  encodes the driver's reversibility test: before parking an item on the driver, ask whether a wrong
  guess is still caught by a gate the driver controls (a draft PR awaiting merge, a filed issue, a
  flip a human must still make). If yes, the desk default-forwards — authors, dispatches, opens the
  DRAFT PR, makes the best-guess call, and NOTIFIES, filing the `needs-decision`/`question` issue as
  a notification rather than a park. It stops only for a fixed one-way / outside-the-gate set (merge,
  a ready-flip that is not the role's, an unauthorized `main` push, a tag or release, weakening a
  security control, secrets/PII/exploit exposure, money movement, identity/auth changes, durable-data
  loss, and anything that leaves the repo).

### Changed
- `escalation-labels` guardrail and resident rule R8 now say a `question` parks an item only when the
  fork is one-way; a reversible item proceeds on its stated default with the label riding on it. The
  desk boot, autonomous-drive, file-and-exit, needs-decision, and ask-decision passages are aligned
  to run the reversibility test first, so a reversible fork moves on its default instead of waiting.
