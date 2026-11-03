# forgeread

The importable fact reader: one narrow `Reader` API (`OpenIssues(ctx, repos)`) for the open issues
of an explicit repository set, and the types, envelope parser and offline/frozen readers that go
with it. Design record: `docs/library-first.md` ("The first shared read") and the decision
`docs/streams/decisions/DR-forge-neutral-36.md`, superseded on 2026-10-25 by
`docs/streams/decisions/DR-forge-library.md` (one shared forge library for reads and writes,
which composes this module's adapters).

## Layout

| Path | What it is |
|---|---|
| `reader.go`, `envelope.go`, `offline.go` | The root package: `Reader`, `Issue`, `Result`, `Unavailable`, `Reconcile`, `ParseIssuesEnvelope`, `OfflineReader`, `FrozenReader`. Imports nothing online (no subpackage, no `net/http`, no `os/exec`); both are tested. |
| `adapters/` | The `Access` interface (one page of an already-admitted repository) and the paging error. |
| `adapters/github`, `adapters/gitlab` | Provider paging and mapping for open issues, written against `Access`. They hold no credential, resolve no host and read no environment. |
| `adapters/github/issuestates.go` | Positive state/title observations for explicitly numbered issues, through a typed `IssueStateAccess` port. The fixed query has at most twenty active aliases; identity, missing/null fields, unknown state and GraphQL errors refuse the result. No credential or arbitrary query input is exported. |
| `testdata/parity/` | Fake-provider observations and the expected results every path is held to: the adapters, the `deskread` verb and statusgen. |

## Who links what

statusgen links the root package only (offline and frozen inputs; its online reads run the
`deskread` verb until desktools-v2/77 moves them onto the shared forge library). desk-tools
links the adapters and composes them over its own authenticated transport; under
`DR-forge-library` that composition moves into the shared forge library (desktools-v2/75), which
statusgen then links instead of linking any adapter directly. An external consumer can use the
root package, or write its own `Access`. The root package stays free of any transport or
credential either way.

## Three states, never two

A requested repository is exactly one of: read (possibly with no open issues), or unavailable with a
reason and a class (`unreadable`, `truncated`, `offline`, `missing`). A provider that keeps
paginating past the walk ceiling yields `truncated` and no list, because a consumer reads an
issue's absence from a list as evidence about it. An envelope with an unknown schema or kind is
refused (`ErrUnsupportedSchema`), not best-effort read.

## Versions

`APIVersion` versions the Go surface; `EnvelopeSchema` versions the JSON envelope. They move
independently. The module has no dependencies, and the repository root has no `go.work`; a
consumer inside the repository links it by `require` plus a relative `replace`.

Go API version 1.1 adds the typed positive-state adapter without changing the root
offline/frozen reader or JSON envelope schema. Desk-tools composes this adapter
over its existing admitted, authenticated transport and explicit read-only
wrappers. Statusgen does not link authenticated adapters: its online reads remain
on `deskread` until desktools-v2/77, and after it reach the adapters only through the
shared forge library. The adapter is an observation, never closure inferred from absence
or permission to retire a placeholder. Comment, trust and label histories retain
their separate readers and all original completeness/identity checks.

For several explicit state reads, batching reduces HTTP attempts. GraphQL point
cost remains unknown for this new fixed operation; fewer HTTP sends do not prove
fewer provider points or a production latency saving. Singleton and unsupported
provider paths retain their original serial reader. A failed or incomplete batch
never retries through that path.

## Tests

```sh
cd forgeread && GOWORK=off go test ./...
```

## Mirror kernel

`mirror/` owns provider-qualified observations, admission-bound scopes, coverage,
explicit-time refresh/scan plans and admitted queries over caller-supplied ports.
`QueryScope` checks current admission before reading a Store; the Store must also
check admission independently. Complete-empty is available only with complete,
fresh coverage. Partial scans retain prior rows and their completeness watermark;
full inventory absence means not-in-inventory, never closed. Full-inventory time
is tracked separately so incremental success cannot postpone repair indefinitely.

The package has no network, credential, filesystem, process, database or scheduler
implementation. `mirror/mirrortest` is synthetic reference-store/conformance support,
not production persistence. Permission loss returns unavailable coverage and a
retention-decision-required signal, without choosing keep-history or purge. The
permission hold persists across failed or partial scans and only a successful
complete full inventory may clear it, under current matching admission and with
both LastComplete and LastInventory strictly after the held LastAttempted.
Incremental, not-modified and stale/replayed updates cannot recover permission.
Store adapters enforce that transition independently of the planner. The
persisted mirror integration retains PostgreSQL transactions/restart, authenticated
source mapping, refresh coalescing, review population completeness, domain mapping,
retention policy and reader rollout. These are not supplied by the kernel.
`RetainedFrom` is optional and zero means unknown; the durable Store must derive it
from actual retained coverage under the selected policy, never the oldest visible
row. Trusted composition must canonicalize GitHub owner/repository case before
constructing a Scope; this pure package does not resolve aliases.

The root API and its JSON envelope stay unchanged. Later shared-library composition
can consume the kernel without moving credentials into this package. Targeted
kernel checks run with `go test -run '^TestMirror' ./mirror/...`.

## Incremental source reads (Go API 1.2)

The provider adapters add `ChangesUpdatedSince` and `IssuesUpdatedSince` over the same
already-admitted `Access`. `adapters.UpdateQuery` carries an inclusive time bound; results
retain provider/repository/query provenance, page count, observed update bound and an explicit
`Incomplete` flag. `BodyKnown=false` and nil collections preserve omitted fields as unknown;
explicit empty collections remain non-nil. A failed page supplies no successful population. These are endpoint
observations, never complete reviews, checks or threads, and do not alter the root issues
wire envelope. `EnvelopeSchema` stays at 1.

GitHub's pulls endpoint has no server-side since parameter: its bounded update-ordered walk
filters locally and reports `ServerSideSince=false`. Conditional validators and 304 reuse are
not implemented; `ConditionalSupported` is false while every requested page is read in full.
Desk-tools' `MirrorReadSource` is the concrete consumer through existing scoped backends and
read-only wrappers. Persistent mirror integration and the later shared forge-library custody
migration retain their separate owners.

These update-descending walks use offset pages over a changing provider population.
A concurrent update can move an item between pages; duplicate numbers and missed observations
are possible. Later ingestion must apply records by qualified identity, poll with an overlap
window and periodically repair from a full inventory. Only a successful, complete walk's nonzero
`ObservedUpTo` can inform the next provider-time bound, with that overlap; it is never a wall-clock
watermark or an atomic snapshot. Incomplete walks or walks with no source rows return zero.
A complete filtered-empty walk can report an older provider stamp; preserve the prior bound
instead of regressing it. Only advance the previous complete bound when the observed stamp is newer.

`RequestedReviewers` describes individual user accounts only. GitHub's `requested_teams` is not
read by this contribution, so an explicitly empty user collection does not mean no team review
was requested. Complete reviewer assignment/approval coverage remains with mirror integration.
`MirrorReadSource` currently has no production command caller; these operations remain inert
until that separately qualified integration.

Each operation accepts at most 8 MiB of aggregate raw JSON payload before decoding and
retaining additional records. A page that crosses that budget ends the walk with
`Incomplete=true`, `LimitReason=byte-limit` and a zero `ObservedUpTo`. `ResponseBytes` includes
that fetched page and `PageCount` includes its request. The byte count measures raw JSON,
not compressed network bytes. A page-ceiling stop uses `LimitReason=page-limit` and also clears
the bound. This bounds accepted multi-page payload, not total decoded-object overhead.

Desk-tools separately bounds each native incremental response to 8 MiB of decoded JSON
before either provider SDK buffers or decodes it, including error responses. Its private
mirror client copies reuse the admitted transport, cancellation and scope checks; ordinary
reads, logs, archives and writes keep their previous behavior. The bound reads at most one
additional decoded byte to detect overflow, closes the original body, and returns an
inspectable limit/could-not-check cause without a population or observed bound.
On native guard failure, known HTTP error status remains inspectable alongside the original
limit or read cause. The added HTTP metadata contains status only. Automatic
HTTP gzip decoding and explicitly encoded gzip responses use the same decoded-byte cap;
unsupported content encodings refuse. Content-Length never substitutes for counting bytes.
This native contribution does not qualify the future mapper's preservation of persisted
watermarks, PostgreSQL, scheduling, coalescing, retention or reader cutover. Those remain
with the joined mirror delivery, including its row-23 production-flow acceptance. The
shared-library migration reuses this native composition rather than adding a second bound.

## Store-injected read-through coordination

`refresh` composes the existing mirror query/planner/source/store ports. It has no
production gate, store, provider walker, credential, listener or caller cutover.
`New` requires an explicit host lifetime, clock, admitted source/store, trusted
caller-authority checker and shared gate. Missing dependencies hold; there is no
process-local gate or failure-cache fallback. The root package and pure mirror
never import orchestration.

Trusted caller policy supplies current admission/access/configuration/policy epochs,
allowed freshness, finite concurrent/queued demand, per-caller/admission fill-window
reservations, owner/wait deadlines and failure capacity/expiry. One owner attempts
at most one Source.Scan. A fill reservation counts coordinator work, not HTTP pages,
provider retries or GraphQL points. Optional observations count own scan calls,
cache hits and joined waiters; missing instrumentation is unknown.

A miss uses the real kernel, joins/acquires the shared gate, scans, plans an update,
applies through the gate-bound Store seam and requeries. A partial nil-error result
or insufficient review/not-modified observation remains unfulfilled and cannot
advance completeness. Caller/admission refusal creates no shared source-failure
cooldown or confirmed permission-loss record. Source-confirmed permission coverage
remains a separate kernel outcome, with retention still human-gated. Current caller
and repository admission are checked after store I/O before releasing any rows.
The same checks and caller deadline apply after gate cleanup I/O.
After the last observer/cleanup callback, release checks current permission and
deadline, then uses the mirror owner's pure `EvaluateSnapshot` with trusted current
time and no later callback or I/O. The evaluator reuses canonical validation/freshness, performs no I/O and
grants no authorization; holders of its typed snapshot must still authorize release.
Scan-call diagnostics follow the actual invocation; completion time is captured at
Scan return before diagnostics, so a slow observer cannot inflate complete freshness.

Base fill identity stays separate from bounded observation `Requirements` (currently
complete existing reviews). `Demand` carries that requirement class; no request id,
projection or arbitrary freshness value fragments it. After its actual requery,
each waiter reports a typed `Outcome` through `Leave`. The shared gate atomically
records requirement-qualified failure/cooldown and waiter accounting, independently
checking current ticket generation and queried snapshot version. Unchecked
cancellation, deadline, admission or epoch refusals install no failure. Repeated
cached outcomes cannot slide the original cooldown, and a late older observation
cannot poison a later qualified recovery. This remains a canonical R2 obligation.
`Completion.ReviewOnlyPartial` preserves the owner's requirement-only downgrade
before a backend normalizes its query. Persist that qualified class with generation
and applied version: a joined weaker base waiter may perform the existing bounded
refetch, and cannot poison itself with the stronger owner's review-only failure.
Actual partial-source cooldown and LastComplete preservation remain unchanged.

The R2 implementation must bind the injected store to its canonical PostgreSQL
backend and atomically check refresh generation, apply/query through
`ApplyCompletion`, and commit completion/cooldown. An old owner with a still-matching
snapshot version must be refused before apply. The package supplies no such SQL
adapter. Its tests share a stateful fake gate and reference Store between two
independently constructed coordinators; that proves port/coordinator behavior,
not process durability, SQL atomicity, authenticated serving or activation. Full
forge-neutral/37 retains its joined persisted mapper/watermark criterion.

The current mirror ScanResult/native budget receipt and refresh Completion do not
carry a qualified provider retry floor. Fixture cooldown/RetryAt therefore proves
local shared failure suppression only; it is not HTTP Retry-After compliance or
permission to retry a real provider. Real source/gate integration remains held until
the joined owner propagates typed qualified retry timing or an explicit unknown-retry
hold. Do not infer it from error text, retain response bodies/headers here, or install
a blind fixed interval as a production substitute.
