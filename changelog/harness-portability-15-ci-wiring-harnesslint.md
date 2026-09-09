### Fixed
- The generated Codex plugin manifest was two minor versions behind the Claude manifest it is
  generated from (`0.5.1` against `1.0.0`) and had been since the version bump merged —
  `harnessgen`'s committed-manifest and version-parity checks were red on `main` and nothing was
  running them. Regenerated, and the leg that catches the next one is wired.
- The shipped `ask-decision` and `install` skill bodies no longer name Claude Code's plugin-root
  environment variable or its session-start hook event. The bodies name the neutral mechanism —
  a `<bundle>` placeholder for the installed bundle's directory, and "the session-start
  resident-rules injection channel" — and the Claude Code binding reference now carries the
  expansion for both, so a reader on any harness can still run the inbox and still knows which
  surface needs the documented Windows workaround.

### Added
- `harnesslint bindings` understands a reference file that DECLARES itself out of the
  per-harness binding matrix, via a one-line
  `<!-- assay:harnesslint non-matrix-reference — <reason> -->` marker. `references/desk-shell.md`
  — harness-neutral shell and transport mechanics, never a capability binding — carries the
  declaration and stops producing nineteen violations. The skip is narrow and loud: the reason is
  mandatory, every skipped file is named on stderr, an undeclared reference is still fully
  checked, and declaring every reference out is a could-not-check rather than a clean sweep.
- A staged `ci.yml` patch (`tools/harnesslint/ci.yml.patch`) that runs the `harnessgen`,
  `harnesslint` and `plugindrift` suites in CI instead of only building and vetting them, and adds
  a job running the neutrality lint against the real bundle tree — the fixture-based unit suite
  never reads it.
