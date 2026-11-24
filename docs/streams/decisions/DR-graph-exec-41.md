---
id: DR-graph-exec-41
date: "2026-11-24"
title: "Finish the loop-admin journal in PostgreSQL: typed process records, one locked shared-capacity admission with an append-only launch ledger, restore provenance from outside the database, and passive backup with an artifact inventory"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Keep consumed allowance as a counter row updated in place. Ruled out: an update can lower the count, a restore of an older copy lowers it silently, and nothing records which launches spent it. An append-only row per launch keeps each spend attributable, and the window total is derived from the rows."
  - "Admit against capacity in application memory or per supervisor, and write the journal afterwards. Ruled out: two supervisors sharing a profile would each see spare capacity and both launch. Admission reads the budget and the recorded launches, and records the new launch, in one transaction under a database lock that every caller takes."
  - "Treat a restored database as current once its schema checks out. Ruled out: a restore would return spent allowance and could revive an old execution group. Restore provenance comes from outside the database and is compared with the protected operator incarnation, so a restored journal holds paused until the operator rotates and confirms the old group stopped."
  - "Use an embedded or in-process database for tests or as a fallback. Ruled out by the brief's layering: PostgreSQL with pgx and Goose is the only engine, and the existing no-embedded-engine test keeps it that way."
  - "Let the test fixture connect to whatever server or database the environment names. Ruled out: a mistyped setting could drop a database the fixture does not own. The fixture accepts only a loopback server's admin database and drops only databases carrying its own marker."
accepted:
  - "A new forward-only migration adds the typed records and the launch ledger. The delivered first migration is not edited, and no migration has a down step."
  - "Unknown, unreadable or unverifiable usage refuses a launch. A pacing estimate derived from recorded launches may delay a launch but never refuses one on its own."
  - "Restore is passive. It verifies the referenced-artifact inventory, holds the journal and never launches. Operational restore and activation stay separately authorized adapters."
  - "No production database, role grant, service enrollment or activation is configured by the implementing change; the grant model chosen on the decision issue is expressed as migration and fixture checks only."
  - "The test fixture keeps every disposability rule of the brief's Task 6: a literal-loopback server and its admin database with only the allowed settings, create and drop only of databases carrying its own marker, the configuration redacted from String() and the database password kept out of every error, and PostgreSQL environment variables scrubbed from the test and child environment."
  - "Verify rows that need a PostgreSQL server run only where a disposable server is supplied; anywhere else they are reported as could-not-check, never as passed."
---

**PROPOSED — no ruling is recorded.** `decided-by:` is a placeholder until a human rules on
this record on the brief's own decision issue (filed from its `## Human decision` section
and recorded in the brief as `decision-issue:`). Until then this record is not an approved
design, and the brief that cites it (graph-execution/41, `design: DR-graph-exec-41`)
may not be accepted on its account (the design-approval gate, `spec/lifecycle-v1.md`
§4.4). The ruling comment must name `DR-graph-exec-41` and one of the brief's options.
The record then gets a `ruling:` link to that comment, which `statusgen --corroborate`
resolves (`spec/registers-v1.md` §7.5).

## The decision, and the constraint behind it

The journal's base is already on main: launch intent and receipt commit together, a lost
receipt holds the request as unknown, a missing or mismatched operator incarnation pauses
the journal, and no embedded engine may enter the module. This record covers the rest of
graph-execution/41 (Tasks 1 to 6 beyond that base).

The constraint is the brief's `gate-why:`: losing or restoring the database must never give
back launch allowance or revive an old execution group. The design meets it three ways.
Spent allowance is an append-only ledger, one row per launch, so the window total is
derived and attributable. Admission and the ledger write share one transaction under one
lock, so concurrent callers cannot both spend the last unit. Restore provenance comes from
outside the database and is compared with the protected incarnation, so a restored copy
pauses rather than resuming.

The open question is the privilege boundary. If the runtime role may update or delete
ledger rows, or raise the profile budgets and admitted limits that admission reads, the
append-only property holds only by convention. The brief's options are a split between a
migration-owner role and a runtime role that may only insert and read ledger rows and only
read budgets and limits, checked by a new Verify row, or a single trusted role with in-place
tampering declared out of scope.

## Why `consequence: major`

If this design is wrong, a crash, restore or concurrent caller could hand back spent launch
allowance or restart an execution group the operator believed stopped. Both are
irreversible for the launches that follow. A route back exists before activation: the store
is not yet configured anywhere, the migrations are forward-only source, and a corrected
design is a new migration rather than a data repair.

## What this record does NOT decide

It does not configure any database, role grant or service, and it does not activate a
shared supervisor; activation stays its own authorized step. It does not define operational
restore. It does not choose the protected incarnation source's installed custody, which
`spec/operator-incarnation-v1.md` leaves to a separate decision. It does not accept the
brief: independent verification of its Verify table and the human's final acceptance stay
separate steps.
