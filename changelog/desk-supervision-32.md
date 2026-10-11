### Added
- `deskclose triage --disposition human-decided` now writes a `human-decision-v1` record of
  each ruled close. The record holds:
  - the offered option ids, each with a digest of the option's text;
  - the recommended id;
  - the picked id, or `ambiguous` / `unparsed`;
  - the ruler, always the role `driver`;
  - how long the decision waited, from the issue's opening and from the ask.

  - whether any text it was read from was edited after the ruling (`edit_state`). An edited
    record carries no pick and no recommendation.

  It never holds option or ruling text. Its one free-form field, the brief id, is held to the
  `<stream>/<NN>` grammar. A ruling that a person could read as anything other than one
  option is recorded as `unparsed`. Two copies are written as the same bytes: a hidden block
  in the close comment, and a line of `~/.config/assay/decision-records.jsonl`. The record
  never stops the close. An invalid record is skipped. A block the outbound check refuses is
  dropped, and the comment is posted without it. A failed local write is named in the audit
  line.
- `deskkit.Issue` carries the forge's `CreatedAt` on both backends.

### Changed
- The decision-issue Options parse moved from `deskinbox` into `deskkit`
  (`ParseDecisionOptions`), so the inbox and the record read the same options. The inbox's
  output is unchanged.
- ask-decision §Recording a ruling: close a ruled decision through the human-decided lane, so
  the record is written. A hand close writes none.
