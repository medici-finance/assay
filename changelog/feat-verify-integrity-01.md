### Added
- `deskpathguard check` — a PR that touches a protected verifier path (a brief's `## Verify`
  table, `.github/workflows/**`, `.claude/guardrails/**`, `tools/skillslint/**`, a
  `verify.d/**` scripted-rows directory, or `**/testdata/**`) alongside a non-brief,
  non-fixture file is labelled `wrote-to-the-test` and force-gated to `gate: human` at the
  status transition, unless the author is the desk/verifier identity, the PR carries a
  `regen:` label, or the diff is pure authoring (brief/fixture files only). See
  `docs/protected-paths.md`.
- `deskpathguard rederive` — verify-desk's pre-change re-read: reports a Verify-table row
  present only at HEAD (not at the merge-base with `origin/main`) as `author-added`, and
  runs every other row using the merge-base's own command/expect text.
- `pr-review-desk` runs the check at every new head before an APPROVE verdict; `verify-desk`
  runs the re-derivation on a labelled brief before any row.
