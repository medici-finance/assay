### Added
- statusgen brief 18 (authoring only) and design-decision record `DR-gate-rederive`
  (`docs/streams/decisions/`). The record transcribes the maintainer's answer on #2405, option
  c, detect only: the issue scan never changes a stored placeholder gate. The brief specifies
  the notice that answer asks for: `--scan-issues` prints one `NOTICE` for each open issue
  whose labels and title derive `gate: human` while its placeholder reads `gate: model`,
  writes nothing and leaves the exit code alone. Raising the gate stays a hand edit, and the
  first run of the notice lists the existing placeholders to look at.
