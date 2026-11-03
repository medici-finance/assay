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
