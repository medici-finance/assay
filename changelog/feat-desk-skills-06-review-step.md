### Added

- `deskdispatch --kit review`: clause 15 ("Scoped prompt-audit — on a PR that changes prompt
  text"), a scoped prompt-audit step the reviewer runs on any PR
  that changes a `**/SKILL.md` file, a `**/references/*.md` file (a skill's own, a bundle-level
  reference, or a dispatched kit itself), or any `CLAUDE.md` — posts High/Medium
  findings only, under `Prompt-audit (scoped):`, governed by clause 12's blocking boundary.
- `pr-review-desk` §"The reviewer's bar": a matching bullet pointing at the new clause.
