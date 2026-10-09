### Added
- `deskclose triage --disposition human-decided` now writes a `human-decision-v1` record of
  each ruled close. The record holds:
  - the offered option ids, each with a digest of the option's text;
  - the recommended id;
  - the picked id, or `ambiguous` / `unparsed`;
  - the ruler, always the role `driver`;
  - how long the decision waited, from the issue's opening and from the ask.

  It never holds option or ruling text, and never a login. Two copies are written as the same
  bytes: a hidden block in the close comment, and a line of
  `~/.config/assay/decision-records.jsonl`. Neither copy can block the close: an invalid
  record is skipped, and a failed local write is named in the audit line.
- `deskkit.Issue` carries the forge's `CreatedAt` on both backends.

### Changed
- The decision-issue Options parse moved from `deskinbox` into `deskkit`
  (`ParseDecisionOptions`), so the inbox and the record read the same options. The inbox's
  output is unchanged.
- ask-decision §Recording a ruling: close a ruled decision through the human-decided lane, so
  the record is written. A hand close writes none.
