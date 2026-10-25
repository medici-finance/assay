# forgeread

The importable fact reader: one narrow `Reader` API (`OpenIssues(ctx, repos)`) for the open issues
of an explicit repository set, and the types, envelope parser and offline/frozen readers that go
with it. Design record: `docs/library-first.md` ("The first shared read") and the decision
`docs/streams/decisions/DR-forge-neutral-36.md`.

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
`deskread` verb). desk-tools links the adapters and composes them over its own authenticated
transport. An external consumer can use the root package, or write its own `Access`.

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
on `deskread`. The adapter is an observation, never closure inferred from absence
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
