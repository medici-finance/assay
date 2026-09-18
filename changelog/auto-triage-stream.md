### Added
- New `auto-triage` stream (`status: parked`, draft spec): a scoping doc + four briefs that
  automate the RESPONSE to an all-stop CI signal — identify the culprit, file the bug, and
  either open a fixing draft PR (mechanical) or route it (judgement), with a never-invisible
  watchdog escalating any red no responder acted on. Authoring-only; nothing is implemented,
  no gate is weakened, and the three autonomous-action briefs are `gate: human` pending
  `DR-auto-triage` approval. (#1119)
