### Added
- `deskfile new` gains a **blocker-evidence gate**: a filing labelled `needs-decision`,
  `help wanted` or `question` is a blocker claim and is refused (exit 5) unless its body
  carries an `### Evidence` heading followed by a fenced block. `human-only` is an act, not a
  claim, and is not gated; `attach` observations are unaffected; the refusal takes the
  audited `--force-new --reason` bypass.
- `deskfile new --correction "<message>" --label skill-bug --section … --reading …` composes
  a **skill-bug** issue from this session's last `deskack` receipt (receipt line, correction
  verbatim, `$DESK_LOOP`, skill+section, and the desk's reading) — the tool composes the body,
  not the desk — and refuses when no receipt was recorded in the last 30 minutes.

### Changed
- The four desk skills (`worker-desk`, `pr-review-desk`, `verify-desk`, `the-desk`) carry the
  blocker-evidence rule as a sibling of the idle HARD GATE and a correction-capture clause
  under the receipt rule; `worker-desk` holds the one definition, the other three point at it.
