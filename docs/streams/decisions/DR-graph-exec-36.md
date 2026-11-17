---
id: DR-graph-exec-36
date: "2026-11-16"
title: "PostgreSQL foundation (graph-execution/36): explicit connections, compatible-schema Open and one forward-only Goose migration and checksum ledger in workflow/store, qualified on a disposable PostgreSQL fixture; proposed, not yet ratified as a design record"
consequence: major # The transcriber's classification.
decided-by: "human:<name>"
alternatives:
  - "Option 2 on decision issue #5405, hold acceptance and name an unsupported boundary or correction. Not taken: the driver's comment on #5405 chose option 1. No rationale beyond that comment is recorded, and none is supplied here."
  - "A mirror-specific database or a temporary embedded backend. Ruled out by the brief's design-fit `why-add:` and facts: R2 PostgreSQL is the only coordination-store engine, SQLite and a second online backend are forbidden, and consumers would otherwise duplicate migrations and fixture lifecycle."
  - "A custom pool, protocol parser or migration runner. Ruled out by the brief's `## Dependency admission`: pgx/v5 and Goose/v3 are reused per `docs/go-engineering.md`, with a narrow project-policy Store adapter keeping checksum and version records in the same ledger rather than a second runner or ledger."
  - "Wait for the full graph-execution/19 store, with admission, outbox and executor integration. Ruled out by the brief's `why:`: read-only mirror persistence needs only the migration and test substrate, so it is split out and delivered first."
accepted:
  - "No production database, provider identity or executor is configured; no connection, live migration, role grant, service enrollment, tool cutover or activation follows (the brief's `## Human decision`)."
  - "The new CI job runs repository source with runner and checkout-token access against a pinned disposable PostgreSQL service with no secrets; this is the boundary the brief's `gate-why:` puts to human acceptance. The delivering pull request moved that job to the existing GitHub-hosted Ubuntu runner class, recorded there as a desk-decided default."
  - "Every nonempty PG-prefixed environment variable is refused; callers must pass explicit configuration (the delivering pull request's review corrections and `workflow/README.md`)."
  - "Foundation acceptance is never full graph-execution/19 acceptance: the domain-store, mirror-retention, deployment and activation gates stay in place (the brief's ground rules and the driver's comment on #5405)."
---

## State and provenance

**Approval state: proposed, not ratified as a design record.** The `decided-by` stamp above is
the public placeholder and names nobody. This record carries no `ruling:` link, so
`statusgen --corroborate` reports it `placeholder-unratified` until the driver posts, on the
decision issue, an unedited comment from the driver's own login that names `DR-graph-exec-36`.
The `ruling:` link to that comment, and the brief's `design:` key, are added only after it exists.

**What the driver has already ruled.** The brief's decision issue,
[#5405](https://github.com/medici-finance/assay/issues/5405), carries the decision-gate marker for
`assay:assay:graph-execution:36`. On 2026-11-04 the driver's own login commented, unedited,
"graph-execution/36: 1", accepting the bounded PostgreSQL foundation in #5418 on the independent
security review and the correctness review of the credential, migration and CI boundary, with the
complete domain-store, mirror-retention, deployment and activation gates kept in place. The brief
already cites that comment as its `ruling:`. It names the brief, not this record, so it cannot
serve as this record's `ruling:` link (`spec/registers-v1.md` §7.5, `record-not-named`).

**Written after implementation.** graph-execution/36 was delivered by
[#5418](https://github.com/medici-finance/assay/pull/5418), merged 2026-11-04, before this record
existed. It is written on demand under the ruling on #5145, which limits back-filled design
records to held briefs that block other work: graph-execution/19 and forge-neutral/52 each declare
`depends:` on graph-execution/36, and both are `todo` on their boards.

This record does not mint a decision. It is taken from the brief, the delivering pull request and
the code on main; nothing here adds rationale beyond what those record.

## The design

As the brief asks and #5418 delivered:

1. A `github.com/medici-finance/assay/workflow` module whose `store` package takes explicit
   caller-supplied PostgreSQL configuration. `Open` validates the recorded schema and never
   migrates; `Migrate` is a separate operation. Missing or unreachable input is an error, never
   another backend.
2. One append-only migration manifest under `workflow/store/migrations/`, executed by Goose with
   its PostgreSQL session locker. Each migration and its checksum and version record commit in one
   transaction; duplicate, out-of-order, changed, unknown-future and incomplete ledgers refuse. No
   down migrations, reset verb or silent repair.
3. `spec/coordination-store-v1.md` labels the implemented foundation schema separately from the
   planned graph-execution/19 domain schema.
4. A disposable fixture harness in `workflow/internal/pgtest`, an `assay-workflow.yml` CI job on a
   disposable PostgreSQL service, and the `S-coordination-store` owner row in `docs/contracts.md`.

The brief and its delivering diff are the authority; this summary does not amend either.

## What this record does not decide

It does not record a verification result: the brief's Evidence is empty and its Verify table stays
the independent verifier's. It does not move the brief's board row. It does not decide
graph-execution/19, forge-neutral/52 or any later consumer of this foundation.
