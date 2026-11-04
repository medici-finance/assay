# Workflow persistence foundation

`store` is the canonical PostgreSQL connection/migration substrate. It uses pgx v5.12.0
and Goose v3.29.0, selected by `docs/go-engineering.md`; version-specific assessment and
the exact driver age exception are recorded in [issue 5396](https://github.com/medici-finance/assay/issues/5396).
The bounded mirror Store shares this substrate. No controller, execution claim, outbox,
activation or server is present.
The schema contract is [coordination-store-v1](../spec/coordination-store-v1.md).

Call `store.Migrate(ctx, explicitConfig)` for an explicitly authorized migration operation.
Call `store.Open(ctx, explicitConfig)` to validate an already compatible schema; Close the
returned handle. Open performs no DDL. Production calls never read a DSN or select an
ambient connection/credential. Every nonempty environment variable whose name starts with
`PG` is refused, including image metadata such as `PG_MAJOR`, `PG_VERSION` and `PGDATA`;
callers must remove those variables before using this deliberately strict configuration.
No default endpoint, alternate backend, migration-down/reset API or secret-bearing error.
The database role owns migration DDL only when that operation is explicitly authorized;
provisioning, restricted application roles and activation remain operator-owned.

The foundation has been qualified only against PostgreSQL 18.6. Durability assumes normal
PostgreSQL durable commits and a functioning volume; backup rollback, whole-database loss,
operator incarnation and domain recovery remain full graph-execution/19 obligations.
The optional Goose CLI and its all-dialect drivers are not linked by this module.
Transitive checksum entries for upstream test dependencies are not runtime backends.

## Disposable container profile

Tests need a selected local Docker context whose endpoint the operator verified as local,
plus an explicit disposable PostgreSQL DSN. They never start Docker, select a context,
download a binary, inspect kubeconfig or contact an ambient database by default. The example
Compose file pins PostgreSQL 18.6; run it only with an explicit chosen local context. Bind
its host port to loopback, use a unique project name, inspect the mapped port after any
restart and supply the resulting synthetic-credential URI to the test command. Teardown
only the resources that exact project created; no shared-volume prune or daemon-wide cleanup.
Do not paste an operational DSN into this profile.

```sh
# From workflow/. Choose the context explicitly; inspect is local metadata only.
: "${ASSAY_TEST_DOCKER_CONTEXT:?select the verified local Docker context}"
endpoint=$(docker context inspect "$ASSAY_TEST_DOCKER_CONTEXT" --format '{{.Endpoints.docker.Host}}')
case "$endpoint" in unix://*) test -S "${endpoint#unix://}" ;; *) exit 1 ;; esac
printf 'Disposable fixture target: context=%s endpoint=%s\n' "$ASSAY_TEST_DOCKER_CONTEXT" "$endpoint"
ASSAY_TEST_PROJECT="assay-pg-$(python3 -c 'import uuid;print(uuid.uuid4().hex)')"
# Compose creates only this unique project's network and volume, with project labels.
docker --context "$ASSAY_TEST_DOCKER_CONTEXT" compose -p "$ASSAY_TEST_PROJECT" -f testdata/compose.yaml up -d --wait
published=$(docker --context "$ASSAY_TEST_DOCKER_CONTEXT" compose -p "$ASSAY_TEST_PROJECT" -f testdata/compose.yaml port postgres 5432)
case "$published" in 127.0.0.1:*) port=${published##*:} ;; *) exit 1 ;; esac
export ASSAY_TEST_POSTGRES_DSN="postgresql://fixture_admin:fixture_only@127.0.0.1:${port}/postgres?sslmode=disable"
```

After a container restart, repeat the `port` inspection and DSN assignment: some local
engines change a randomly published port when restarting. Keep the selected context and
unique project identity for teardown. Inspect Compose's project labels before deleting;
then the exact project cleanup is:

```sh
docker --context "$ASSAY_TEST_DOCKER_CONTEXT" compose -p "$ASSAY_TEST_PROJECT" -f testdata/compose.yaml ps
docker --context "$ASSAY_TEST_DOCKER_CONTEXT" compose -p "$ASSAY_TEST_PROJECT" -f testdata/compose.yaml down --volumes
```

Do not reuse a project name or apply this cleanup to another invocation's project.
No global `prune`, shared volume removal or default context is part of the profile.

```sh
# ASSAY_TEST_POSTGRES_DSN must already name the explicitly provisioned container.
test -n "$ASSAY_TEST_POSTGRES_DSN"
GOWORK=off go test -count=1 -timeout=120s ./store/...
GOWORK=off go vet ./...
```

The harness accepts a `postgresql://user:synthetic-password@127.0.0.1:port/postgres?sslmode=disable`
URI (an explicit IPv6 loopback is also supported). Missing input fails rather than skipping.
Every test uses a unique database with an owner marker; it does not reset the admin database.
The standalone CI job uses the GitHub-hosted Ubuntu Docker host, supplies its own pinned
disposable container and executes the complete suite, including the race detector. The
regular repository builder has no Docker daemon and cannot host that service.
No provider, cluster or production contact is part of these tests.

Schema/migration changes are forward-only additions to `store/migrations`; do not modify
recorded SQL checksums. Add future domain tables there, never in a competing ledger.
`TestPGFoundationSchema` dereferences the actual database catalog in both directions.
`TestPGFoundationConcurrent` launches two independent test processes against the same
explicit fixture; local goroutines alone do not qualify cross-process migration locking.

Migration annotation policy follows the pinned Goose comment normalization, including
no-space and repeated-dash spellings and case-insensitive commands. Only forward `Up`,
`StatementBegin` and `StatementEnd` annotations are admitted; transaction disabling,
environment substitution and every other annotation refuse before connecting.

For repeatable production fault controls in an owned clean linked worktree on an owned
feature branch (not `main`, `master` or detached HEAD):

```sh
test -n "$ASSAY_TEST_POSTGRES_DSN"
python3 store/testdata/mutations.py
```

The script snapshots/restores exact source bytes and rejects compile-only failures as
invalid controls. It never starts a server or grants independent verification credit.
Its nontransactional fault deliberately disables both forward-only transaction guards
and forces the wrong upstream execution mode, so the real failed-DDL rollback assertion
must fail; this is one composite execution fault, not three independently qualified guards.
Cleanup after an acknowledged fixture creation is bounded. An uncertain creation whose
ownership cannot be established is retained and reported, never deleted speculatively.


## Bounded mirror Store

After explicit migration/open, call `db.Mirror(store.MirrorLimits{MaxRecords: ..., MaxBytes: ...})`
with finite positive limits selected by composition. There is no production threshold default.
The returned `mirror.Store` admits direct Read/Apply calls independently, partitions by full
provider/host/repository/kind/admission identity, and commits SQL CAS, rows and coverage together.
The byte limit includes scope strings, opaque validators/revision and encoded record facts;
row limits also bound incoming update/absence lists. Oversize input or persisted state refuses.

Codec1 stores opaque strings as bytea and retains BodyKnown, nil versus known-empty
collections, review provenance and exact nanosecond instants. Coverage uses numeric Unix
nanoseconds. An old visible row never supplies retention history; RetainedFrom stays unknown.
SQL transitions independently refuse incomplete watermark advancement, partial absence,
regression and unqualified permission-hold recovery. This contribution performs no retention
operation, source request or credential lookup. A status-only hold retains synthetic facts
while QueryScope withholds their content. Current authorization is checked before and after
I/O and immediately before commit; revocation detected after a durable commit cannot undo it.

Naked Apply has no shared refresh-generation fence. The shared refresh adapter below checks
the current generation and joins Apply plus completion/cooldown in the same SQL transaction.
Do not use this Store alone to claim a production coordinator safely fences stale workers.
Full forge-neutral/37 joined persisted evidence, retention and activation remain owed.

`TestPGMirrorConformance` runs the unchanged kernel conformance suite against PostgreSQL.
The remaining named mirror tests cover raw SQL bypasses and atomic rollback, separate-process
first/update races, independent client restart and canonical owned-table rebuild, mid-I/O
revocation, exact field presence/history, and schema/CI boundaries. Table rebuild is solely
a synthetic fixture control; no production reset or automatic reconstruction API is exposed.
The canonical ledger is neither deleted nor rewound during that control.

Run the mirror controls separately with the same explicit fixture:

```sh
python3 store/testdata/mirror_mutations.py
```

The seventeen faults cover current authorization, post-I/O and precommit revocation,
admission partitioning, SQL CAS, complete/inventory/observed/monotonic guards, hold recovery
including NULL proof, partial/incremental absence refusal, new-row absence, rebuild completeness, codec framing and derived
history. The split-transaction fault is one composite execution error: it commits the
header early and disables the matching same-transaction row guard so the injected second-row
failure exposes the prematurely committed version/coverage. It does not claim independent
qualification of two guards. The complete-proof fault similarly removes both SQL clauses
that implement that one proof. Every control restores exact production bytes and rejects
build/setup failures as invalid evidence. These are implementer controls, not acceptance.

The read-only refresh adapter is constructed with `DB.Refresh` over its exact
canonical `MirrorStore`, an explicit `RefreshConfig`, current caller authority and
a required trusted `refresh.Clock` shared with its source/coordinator. This clock
validates data age and supplies completion-query time; it is never a lease, window
or retry authority. Nil/typed-nil/zero clocks refuse before SQL I/O, and later
samples are checked after I/O and before commit. No clock default or skew tolerance
is selected, and original source observations are not restamped.
It implements the shared refresh Gate and native AttemptAuthority against the same
pool and Goose ledger. It creates no policy or accounting: missing, unknown and
mismatched SQL profiles hold. Only synthetic tests initialize known usage here;
production initialization, repair and activation remain separately owned.
Shared profiles must explicitly use `Limits.Refresh.FailureBackoff == 0`. Nonzero
values refuse before I/O because the Gate owns shared retry eligibility; the
standalone mirror planner is unchanged.

Migration00003 separates source budgets, admission/caller usage, base fill leases,
waiters and individual native reservations. HTTP is authorized before each actual
base-transport entry and observed once after the bounded read. Known relative retry
delays use database observation time; unknown/missing observations retain a hold
without refund. The private joined Store commits observations and fenced completion
together. Standalone Store Apply remains available without granting refresh ownership.
`NativeLimits.Reservations` is the lifetime retained reservation count for the
source identity, including fully observed attempts. Reaching the cap holds new work
even after a window rollover or process restart; runtime has no pruning/reset/repair
API. This storage bound is distinct from per-window usage and concurrent attempts.
Future archival/reconstruction requires a separate canonical accounting contract.
Held/done bookkeeping fences the scope version without decoding observations.
Canonical lock order is admission, source budget, caller, fill, waiter/attempt, then
mirror scope/records. Native I/O and waiting retain no SQL locks or pool connection.
A superseded waiter releases its exact ticket debit without applying its checked outcome
to the later generation; a new freshness demand can still acquire a new generation.

The deskkit test-support import adds the local workflow module to the desk module
graph. Go minimal version selection therefore also raises selected production
transitives: go-winio0.6.2→0.6.3, logr1.4.3→1.4.4, isatty0.0.20→0.0.24,
multierr1.9.0→1.11.0, yaml/v3 3.0.4→3.0.5 and the protobuf pseudo-version
→1.36.12. These assessed selections are part of the Gate contribution even though
the workflow import is in test support. The later four-checksum repair changes no
selected version; that narrower statement does not describe the whole contribution.

The Gate's profile and base identity use version1 canonical JSON over byte-array
identity fields, preserving arbitrary opaque string bytes across processes. This
private encoding is distinct from the existing bounded observation-fact codec.
No retention action, execution API, endpoint, credential selection or production
cutover is supplied by this adapter.
