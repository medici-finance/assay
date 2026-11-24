# Loop-admin execution journal

The loop-admin journal (`loopadmin/journal`) keeps the process facts a shared supervisor
needs across crashes, restarts and restored backups: what was launched, how much launch
allowance each profile has spent, and which launches are unknown. It lives in the cell's
PostgreSQL coordination database, in its own `loopadmin` schema. The design record is
[`DR-graph-exec-41`](streams/decisions/DR-graph-exec-41.md), accepted under option 1
(enforced grants) on [#7549](https://github.com/medici-finance/assay/issues/7549).

Nothing here configures a database, a login, a grant to a login, or an activation. Each of
those is an operator act.

## Migrations and roles

`journal.Migrate` is the operator step. `Open` never migrates. Migrations are embedded,
forward-only and recorded in `loopadmin.journal_schema`:

- `00001_journal.sql`: activation, invocation, receipt and the append-only event table.
- `00002_ledger.sql`: profile budgets, admitted limits, the append-only `launch` ledger,
  usage reports, checkpoints and referenced artifacts, and the privilege split.

The second migration creates two roles that cannot log in:

| Role | May |
|---|---|
| `loopadmin_owner` | Owns the schema and every table in it. Schema changes and budget and limit writes run as this role or another operator role. |
| `loopadmin_runtime` | Use the schema; read every table; insert invocations, receipts, events, launches, usage reports, checkpoints and artifacts; update an invocation's `state` and `updated_at`, and an activation's `paused` and `recorded_at`. Nothing else. |

The migrating role must be able to create roles, or the roles must already exist. A runtime
login is a separate role, created by the operator and granted `loopadmin_runtime`. That
login cannot update, delete or truncate a launch row, write a budget or limit, or create,
alter or drop anything in the schema. The database refuses each with SQLSTATE 42501, which
`TestJournalRuntimeGrants` checks on a real server.

## Admission

`Intend` records the launch intent and its receipt in one transaction before a child may
start. Within that transaction it admits the launch:

1. It takes one transaction-scoped advisory lock that every admission takes, so two callers
   on the same database cannot both spend the last unit.
2. It reads the profile's budget (`journal.SetBudget`: at most `Max` launches in any window
   of `Window`). A profile with no budget is refused (`ErrNoBudget`).
3. It counts the profile's launches in the window and its invocations held `unknown`. An
   unreadable count, or any unknown invocation, refuses (`ErrUsageUnknown`). A full window
   refuses (`ErrBudgetSpent`).
4. It inserts the launch row. The ledger is append-only, so the window total is derived from
   attributable rows and never from a counter.

The returned `Decision.NotBefore` is a pacing estimate derived from the last recorded
launch. A caller may wait until then. A zero value means no estimate is available, and that
alone never refuses a launch.

Admitted limits (`journal.AdmitLimit`) are immutable: admitting another value for the same
name is `ErrConflict`. Budgets are operator settings and the runtime role only reads them.

## Backup and restore

Backup and restore of the database itself are the operator's separately authorized adapter.
The journal only checks around them, and never launches or activates:

- `journal.Inventory` is the backup step. It reads the schema version and every referenced
  artifact with its digest, verifies each artifact against the operator's `ArtifactStore`,
  and returns the `Manifest` to keep beside the copy. It writes nothing.
- `journal.CheckRestore` runs against the restored database. It first pauses the cell's
  activation, then requires the schema version and the artifact inventory to equal the
  manifest and each artifact to match its digest. A missing, corrupt or extra artifact is
  `ErrRestoreHeld`.

A passing check does not reopen the journal. The operator's incarnation source reports the
restore as a `RestoredControl` (snapshot, incarnation and key reference), and the journal
holds paused while the current incarnation equals a restored one in identity or key
reference. Reopening takes a rotated incarnation and a confirmed stop of the old execution
group, through `journal.Activate`. The restored ledger keeps every launch it held, so a
restore never returns spent allowance.

## Test fixture

The PostgreSQL tests need `ASSAY_TEST_POSTGRES_DSN` and fail, never skip, without it. The
fixture accepts only this shape:

```
postgresql://<admin>:<password>@127.0.0.1:<port>/postgres?sslmode=disable
```

The host is a literal loopback address, the database is `postgres`, the user, password and
port are present, and the only settings allowed are `sslmode=disable` and a
`connect_timeout` from 1 to 30, each at most once. The fixture creates uniquely named
databases and runtime logins, marks each with its own comment, and drops only those whose
marker, owner and creator match. The password never appears in `String()` or an error, and
`PG*` variables are cleared from the test and child environment.
`TestJournalFixtureDisposability` checks these rules with no server.

In CI, `.github/workflows/loopadmin-journal.yml` runs the server tests against a disposable
PostgreSQL service. The copies that add graph-execution/41's tests to it, and to `ci.yml`'s
floor skip, wait in `ci/staged-workflows/` for a maintainer to promote them
([README](../ci/staged-workflows/README.md)).
