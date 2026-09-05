### Fixed
- `statusgen` now recognises every `ASSAY_` roster key the desk tools recognise
  (`ASSAY_REPO_FORGES`, `ASSAY_RISK_CALLOUT`, `ASSAY_WITHHELD_IDENTIFIERS`,
  `ASSAY_ALLOW_CLUSTER`), so a roster that the desk verbs REQUIRE no longer makes
  `statusgen` report the whole trust roster unconfigured. Both binaries read the same
  `roster.env` and both fail closed on an unrecognised key in the `ASSAY_` namespace —
  correct for a typo, wrong for a sibling's key: while the two known-key sets disagreed,
  `ASSAY_REPO_FORGES` (the only way `deskpost` / `deskpr` / `deskfile` resolve a repo to a
  forge) made `statusgen --scan-issues` refuse fail-closed on every scan repo, and no
  roster edit could satisfy both tools at once. The four keys are **recognised, not
  applied**: `statusgen` consumes none of them, and the refusal for a genuinely unknown
  `ASSAY_` key is unchanged.
- The two known-key sets are now COUPLED rather than kept in step by comment. Both
  modules' readers expose their set (`scanKnownRosterKeys` / `knownRosterKeys`), the
  shared cross-tree vector file `statusgen/testdata/roster_coupling.json` declares the
  schema once, and each module's `TestRosterKeySchemaCoupling` asserts its own set equals
  that list exactly in BOTH directions. A key taught to one binary alone — or declared and
  taught to neither — cannot stay green. The two trees are separate Go modules and share
  no code, so the shared vector file is the binding a shared package cannot be.
