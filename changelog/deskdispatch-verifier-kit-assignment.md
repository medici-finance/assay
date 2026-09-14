### Fixed
- `deskdispatch --kit verifier` now emits a verifier-shaped Assignment header instead of the
  implementer's: no "Open the draft PR" scaffold, no `export DESK_LOOP=worker-desk`, and the
  dispatch claim is released once the verdict lands rather than on a branch push. Before the
  fix, `assemblePrompt` recognized only `--kit review` as non-worker, so `verifier` fell into
  the worker `else` branch and inherited its scaffold wholesale — a verifier agent following
  the prompt literally would have opened a spurious draft PR under the worker App's identity
  for a plain verify pass. (#1029)
