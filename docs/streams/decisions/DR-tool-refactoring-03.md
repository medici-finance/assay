---
id: DR-tool-refactoring-03
date: "2026-11-21"
title: "Verify-row and declared-input syntax (tool-refactoring/03): one pure compilation in statusgen/briefdoc that every reader of a brief's Verify table and Context files: declaration calls, with the lexical copies in statusgen and tools/desk removed"
consequence: major # Getting it wrong would execute or credit the wrong Verify command, or widen a declared scope; a route back exists (revert the move).
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/7060#issuecomment-7066535885"
alternatives:
  - "Keep the existing behaviour and request a narrower migration (option 2 of the brief's `## Human decision` and decision issue #7060). Not taken: the driver's ruling on #7060 chose option 1, the bounded refactor."
  - "Keep the ported copies in `tools/desk/internal/verifycmd` and `deskkit` and pin them to statusgen with a drift test (the approach being retired). Ruled out by the brief's Task 4 (\"remove local lexical copies\"): a drift test compares copies, it does not stop a third reader from appearing."
  - "Adopt the union of the permissive readers as the one grammar. Ruled out by the brief's Task 2 (\"do not choose the union of permissive parsers or execute a prose code span\"): the union would accept a prose code span as a command."
  - "Put the compilation in a new package or a desk-tools internal package. Ruled out by the brief's layering: statusgen/briefdoc is the existing syntax owner, and statusgen cannot import internal deskkit."
accepted:
  - "Behaviour is preserved for every existing caller: the moved code is the base code apart from exported names, and refusals keep their messages and exit code (deskkit maps a `*briefdoc.Refusal` onto its refusal exit)."
  - "Two named profiles stay outside the strict grammar: the permissive diagnostic `files:` reader in `statusgen/brieffile.go`, which never refuses, and `verifycmd.SplitRow`, which drops one outer pipe at each end. They are profiles, not second grammars."
  - "Deferred, not decided here: the evaluation declaration API (`statusgen/evaluation/declared_api.go`) consuming the compiled record, routing the remaining Verify header and row walkers in the unrun and fail-first readers through the owner, and moving the unsupported-syntax hold earlier in authoring and preflight."
  - "A class census plus a planted divergent parser guards the class (`TestVerificationDefinitionClassGuard`). It replaces the retired drift test and grants no execution, credential or scope authority."
---

## State and provenance

**Approval state: ratified.** The `decided-by` stamp above is the public placeholder and names
nobody; the record's authority is its `ruling:` link. On 2026-11-21 the driver's own login
commented, unedited, on decision issue [#7060](https://github.com/medici-finance/assay/issues/7060)
([ruling comment](https://github.com/medici-finance/assay/issues/7060#issuecomment-7066535885)):
"DR-tool-refactoring-03: approved as the design record for tool-refactoring/03." That comment
names this record, so it serves as the record's `ruling:` (`spec/registers-v1.md` §7.5).

**What the driver has already ruled.** Decision issue #7060 carries the decision-gate marker for
`assay:assay:tool-refactoring:03`. On 2026-11-21 the driver's own login commented, unedited:
"tool-refactoring/03: 1 - approve the bounded refactor; #7065 may proceed once its independent
class and caller controls pass." The brief cites that comment as its `ruling:`. That comment names
the brief, not this record, so it serves the brief's decision-gate hold (`spec/lifecycle-v1.md`
§4.5); the record's own ruling is the later comment cited above.

This record does not mint a decision. It is taken from the brief, the ruling above and the
delivering pull request [#7065](https://github.com/medici-finance/assay/pull/7065); nothing here
adds rationale beyond what those record.

## The design

As the brief asks and #7065 delivers:

1. `statusgen/briefdoc` (`verification.go`, `declared.go`) holds the one pure reading of a Verify
   row (row split, row identity, Class cell, explicit `cmd:` marker, first code span, prose-led,
   shell-comment and no-op refusal) and of the strict Context `files:` grammar.
2. statusgen's verify code and the `tools/desk` executors (`verifycmd`) and declared-scope
   projection (`deskkit`) call it and keep no lexical rule of their own. Source enumeration and
   path confinement stay in the adapters.
3. The semantic-owner row in `docs/contracts.md` names the owner; the class census and the planted
   second parser are the guard.

The brief and its delivering diff are the authority; this summary does not amend either.

## What this record does not decide

It does not record a verification result: the brief's Evidence is empty and its Verify table stays
the independent verifier's. It does not decide tool-refactoring/04 or /10, which depend on this
brief.
