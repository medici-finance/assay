### Added
- **`forge-neutral` brief 14 — run and gate-approval verbs (`deskrun`) onto the forge
  resolver.** Specifies `RunWorkflow`, `ApproveGate` and `RunStatus` on the `Forge`
  interface, both backends, plus the `deskrun` verb that wraps them: today a release or a
  gated CI run is started by a human's own ambient `gh` session because GitHub's
  `actions: write` permission cannot be scoped down to just dispatch-and-approve (it also
  cancels runs, deletes run logs, and disables workflows repo-wide). The brief documents
  `repository_dispatch` as an alternative trigger and states why it is not adopted as the
  default, prefers GitLab's narrow, start-only pipeline trigger token over a broader
  project token, and specifies `deskrun`'s refusal (exit 5) whenever the roster's
  run-credential binding resolves to a human rather than a dedicated role credential. Doc
  only; no tool behaviour changes in this PR — implementation is a separate, human-gated
  follow-on.
