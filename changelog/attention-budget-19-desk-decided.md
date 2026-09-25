### Added
- `deskpr create|edit --decided <file>` declares a desk-taken reversible default as a
  `## Desk-decided` PR-body section (decision:/alternative:/cost: triples) plus the
  `desk-decided` label, so the driver sees at merge time which pull requests carry a choice
  the desk made rather than one already ruled.
- `deskflip` gains a `desk-decided` condition: it refuses the ready-flip when the
  `desk-decided` label and body block disagree, or when the reviewer's latest verdict at the
  current head names an `Undeclared-desk-decision:` finding. Absence of a block alone is
  never refused.
- The reviewer prompt kit gains a clause asking reviewers to name any desk-taken decision a
  pull request does not declare; the worker prompt kit gains the matching `--decided`
  guidance, and the `default-forward-reversibility` guardrail now names the declaration step
  (with `pr-shepherd` added as a site).
