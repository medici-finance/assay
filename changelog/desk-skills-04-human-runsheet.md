### Added
- New portable methodology skill `plugins/assay/skills/human-runsheet/SKILL.md` — writes the acts
  owed to the driver (a guard refusal, a scope a role's token lacks, a permission its App must not
  hold, a human-only gate override) as exact `! <command>` runsheet entries, distinct from
  `ask-decision` (a decision) and `author-drive-plan` (a multi-session operation record). Each of
  the five desk-role skill bodies now points at it from its own escalation step.
