---
brief: assay:assay:cell-api:10
title: Serve deskd roadmap and portfolio reads from the board library
why: "The service's roadmap and portfolio reads, over both its HTTP route and its agent tools, answer could-not-check today because they name a statusgen output that does not exist and nothing invokes it. A person or agent asking the service what is in flight gets no answer. With the board library and its queue projections available, the service can link them and return real rows for every cell whose tree it can read, still saying could-not-check for the rest."
wave: 4
depends: ["statusgen/39", "statusgen/42"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "These two reads start returning planning rows from every configured cell, private cells included, through a network-facing route and an agent tool that today return nothing. The human confirms three boundaries: an agent caller is held to its own cell, a web viewer is held to the repositories that viewer may read, and the two tools are removed from the question-answering feature so no row reaches a model backend. In each case rows are filtered before any count is computed. The human also chooses how the service module obtains the library, since one of the options changes how the service image is built."
decision-trigger: start
issues: []
schema: brief-v2
version: 1
authored: 2026-11-15 by statusgen newbrief
sources:
  - "deskd/internal/mcp/query.go lines 73 to 85 and deskd/internal/serve/v1.go lines 474 to 505: the two reads and their delegated could-not-check answers"
  - "statusgen/39 (board library and whole-board view) and statusgen/42 (queue projections): the calls this brief links"
  - "docs/library-first.md lines 237 to 249: an external pin of the statusgen module fails at module resolution today, recorded there as an open gap"
  - "docs/streams/cell-api/README.md lines 113 to 134: read-only serving, no new credential on the service, and filter before private joins and counts"
  - "deskd/README.md: the module's build, its single image build context and its relocation status"
  - freshness-checked 2026-11-14 @ 876ca4bca (origin/main)
exec-tier: strong
exec-tier-why: "The module-resolution route is not specified by the facts, and correctness depends on authorization being applied before counts across two serving surfaces."
domain: complicated
outcome: none
consumers:
  - "deskd/internal/boardread (planned), deskd/internal/serve/v1.go, deskd/internal/serve/server.go, deskd/internal/mcp/query.go, deskd/internal/contract/openapi.yaml, deskd/internal/contract/client.go, deskd/cockpit/src/client/v1.d.ts, deskd/web/panes.go, deskd/web/templates/roadmap.html, deskd/go.mod, deskd/go.sum, deskd/README.md: follow-up cell-api/10"
  - "deskd/internal/chat/binding.go (the question-answering feature's tool allow-list, which binds desk__roadmap and desk__portfolio today): follow-up cell-api/10 (Task step 6 removes both tools from that allow-list; its tool call carries no caller identity and its answers come from a configured model backend, so returning planning rows through it needs a separate ruling)"
  - "desk__team_view and the scoped roll-ups of desk__metrics_query: out-of-scope (the board view carries no owning-team dimension and no delivery percentiles; both keep their could-not-check answer)"
  - "a tree source that reads a cell's repository through the forge instead of a local directory: out-of-scope (this brief ships the directory source only; a cell with no configured directory stays could-not-check, and a forge-backed source is a separate brief once a deployment needs it)"
  - "the public service composition of cell-api/02: out-of-scope (it composes existing readers at its own planned path; the package this brief adds is written to be moved there unchanged, and that brief is not edited here)"
  - "clients of the roadmap and portfolio routes outside this repository: out-of-scope (the response keeps its envelope and adds typed rows where it returned null; a client that ignored the null keeps working)"
---

# Brief 10 — Serve deskd roadmap and portfolio reads from the board library

## Context

files: `deskd/internal/boardread/` (planned: `source.go`, `read.go`, `map.go` and tests), `deskd/internal/serve/v1.go`, `deskd/internal/serve/server.go`, `deskd/internal/mcp/query.go`, `deskd/internal/mcp/authz_test.go`, `deskd/internal/contract/openapi.yaml`, `deskd/internal/contract/client.go`, `deskd/cockpit/src/client/v1.d.ts`, `deskd/web/panes.go`, `deskd/web/templates/roadmap.html`, `deskd/internal/chat/binding.go`, `deskd/cmd/deskd/main.go`, `deskd/go.mod`, `deskd/go.sum`, `deskd/README.md`, `deskd/testdata/boardread/` (planned: two fixture trees), `changelog/deskd-board-reads.md` (planned).

facts (876ca4bca, 14 November 2026):
- `desk__roadmap` and `desk__portfolio` are registered with `Scope: ScopeNone` and the handler
  `delegatedRead` (`deskd/internal/mcp/query.go:73-85`, `:484-496`), which returns a fixed
  could-not-check naming "statusgen --json over each cell's streams_root". `GET /v1/roadmap` and
  `GET /v1/portfolio` return `v1DelegatedResponse{Meta, Streams: nil, Assessment}` with the same
  verdict (`deskd/internal/serve/v1.go:474-505`); the un-versioned roadmap handler returns
  `RoadmapPayload` (`deskd/internal/serve/server.go:313`).
- No whole-board `--json` mode exists in statusgen. statusgen/39 adds the library call that
  returns it, and statusgen/42 adds the Next-up and awaiting projections.
- `ScopeNone` is meant only for a tool that reads no cell data (`deskd/internal/mcp/authz.go:20-31`).
  A tool that returns rows must be `ScopeCell`: the call names a configured cell, the caller is
  a loop of that cell, and any repository it names belongs to that cell.
- The existing test does not enforce that for these two tools. `TestEveryDataToolIsCellScoped`
  (`deskd/internal/mcp/authz_test.go:153-172`) holds an `unscoped` allow-list that names
  `desk__roadmap` and `desk__portfolio` (line 155) and errors only when a `ScopeNone` tool is
  missing from that list or mutates. Its one refusal assertion is on a planted probe tool. It
  passes today, and it would still pass if both tools returned rows while left at `ScopeNone`.
- `deskd/web/panes.go:363` labels a tile "desk__portfolio (delegated)".
- That scope is an agent-tool dispatch check only. The HTTP routes (`deskd/internal/serve/v1.go:86-87`),
  the un-versioned roadmap route (`deskd/internal/serve/server.go:56`) and the roadmap page
  (`deskd/web/web.go:133`, `deskd/web/panes.go:75-88`) take no cell argument and do not pass
  through it. On those surfaces read access is the viewer's per-repository authorizer
  (`SurfaceAuthorizer`, `deskd/internal/serve/server.go:70-72`), applied by `cellAuthorized` and
  `itemAuthorized` (`deskd/internal/serve/v1.go:405-437`). With no authorizer wired, every cell
  is served and the surface login gate is the only front.
- The question-answering feature binds both tools in its allow-list
  (`deskd/internal/chat/binding.go:77-84`). Its tool call carries no caller identity
  (`deskd/internal/chat/chat.go:15-21`), and its answers come from a configured model backend.
  It is off by default and enabled per cell in configuration (`deskd/web/web.go:75-80`). While
  the tools return a fixed verdict this is harmless.
- A filesystem built from a directory path follows symbolic links, so a link planted in a
  configured tree can point outside it.
- Each cell declares `repos` and a required `streams_root` (`deskd/config/schema/cells.go:422-423`,
  `:666`). Nothing reads `streams_root` at run time, and the service holds no checkout: its data
  comes from polling the forge into a SQLite index.
- The service module requires only `tools/desk`, at a published pseudo-version, with no
  `replace` (`deskd/go.mod`). That pinned revision of `tools/desk` required no other module of
  this repository. The current `tools/desk` manifest requires statusgen at the placeholder
  version with a relative `replace` (`tools/desk/go.mod:18`, `:129`), so moving the existing pin
  forward meets the same resolution problem as requiring statusgen directly. `make deskd-image` builds with `deskd/` as the only build context
  and `GOWORK=off` (`images/deskd/Dockerfile`, `deskd/README.md`), so a relative `replace` to a
  sibling module cannot resolve inside the image build.
- The statusgen module requires five sibling modules at a placeholder pseudo-version that no
  proxy serves, each with a relative `replace` (`statusgen/go.mod:6-10`, `:52-60`). A consumer
  ignores a dependency's `replace`. `docs/library-first.md:243-245` records the consequence:
  "an external pin of the statusgen module fails at module resolution; this is an open gap".
- The statusgen model imports one sibling module today (`statusgen/model.go:4`,
  `deskcore/issueflow`), so the board packages may carry that import.
- The service polls on an interval (`--interval`, `deskd/cmd/deskd/main.go:136`) and its default
  authentication is loopback (`:140`).

real head: `docs/library-first.md` says "an external pin of the statusgen module fails at module
resolution; this is an open gap", which does not establish that the service module can link the
board library at all. Module resolution from the service's own build is unproven, so it is the
real head: Task step 1 settles it before anything else is written.

layering: a new package `deskd/internal/boardread` holds a tree source (one filesystem per
configured repository), one loader value for the process, and the mapping from a board view to
the two response types. The handlers in `serve` and `mcp` call it and apply the existing
authorization. The package imports the board library and the service's own domain types and
nothing else from the service, so the planned public composition can move it unchanged. Task
steps 2 to 4 build it; Verify row 6 checks its imports.

single point of failure: a caller seeing rows from a repository it may not read. There are two
kinds of caller and each has its own first layer. An agent tool call is refused by the existing
cell scope when the caller is not a loop of the named cell, before the handler runs (row 3). An
HTTP or page request has no cell scope: its handler builds the allowed repository set from the
viewer's per-repository authorizer, the same check `cellAuthorized` uses (row 13). Behind both
sits one independent second layer: the mapping takes the allowed set as an argument and drops
every row outside it before counting, so a handler that skipped its check still returns nothing
for a foreign repository (row 4). Row 13 exists because rows 3 and 4 alone would pass a handler
that handed the mapping the wrong set. A third route, the question-answering feature, is closed
rather than scoped: both tools leave its allow-list (row 14).

design-fit:
  owner: statusgen
  contract: board-view-v1 (owned by statusgen/39; this brief is a reader and defines no new meaning)
  retires: ["the delegated could-not-check answer of the roadmap and portfolio reads for every cell whose tree is configured", "the reference to a statusgen --json mode that does not exist"]
  weight: "verbs 0, flags +1 (the repository root map), refusals 0, rule-text lines 0"
  why-add: "The service has no checkout and must be told where one is. Reusing the streams_root key was considered and rejected: it is a path inside a repository, not a location of one."

## Human decision

The service that answers "what is in flight" for people and agents returns nothing for its
roadmap and portfolio questions today. To answer them it must include a planning-board library
that lives in a different module of the same repository. That library's module cannot currently
be fetched as a dependency by a build that sees only the service's own directory, which is how
the service image is built. A way to obtain it has to be chosen, and the choice needs a human
because one option changes how the released service image is built.

Separately, once wired, these two questions return planning rows from every configured cell,
including private ones. This sign-off also confirms three boundaries:

- An agent asking through the tool interface gets rows only for a cell it is a loop of.
- A person asking through the web interface gets rows only for repositories that person is
  authorized to read. Where the service runs with no per-person authorization configured, the
  sign-in gate in front of it is the only protection, exactly as for its other reads today.
- The service has an optional question-answering feature that can ask these two questions and
  pass the answers to a configured language-model backend. The two questions are removed from
  that feature by this work, so no planning row reaches a model backend. Putting them back
  would need its own decision.

In every case rows are filtered before any count is made.

Options:
1. **Pin by commit.** The service names the library module, and each sibling module the library
   imports, at an exact commit. The image build is unchanged. Each later library change reaches
   the service only when someone moves the pin. Chosen only if the first task step proves this
   resolves from the service's own build.
2. **Build from the repository root.** The service image is built with the whole repository as
   its context and uses the sibling modules directly. The service always matches the library at
   the same commit. The image build and the release step that builds it both change, and the
   image's "service directory only" property is given up.
3. **Wait for the public service composition.** Do no wiring in the current service module; the
   two questions keep answering "could not check" until the planned public service is composed
   elsewhere, which depends on work several waves away.

Default if no answer: none — blocks until answered. The recommended option is 1 when the first
task step succeeds, and 2 when it does not.

## Read first

- `docs/streams/statusgen/brief-39-multi-root-objective-reader.md` and
  `docs/streams/statusgen/brief-42-queue-projections-from-one-board-load-next-up-awaiting-and-brief-info.md`.
- `docs/streams/cell-api/README.md` lines 113 to 134.
- `docs/library-first.md` lines 237 to 249.
- `deskd/internal/mcp/authz.go`, `deskd/internal/mcp/query.go`, `deskd/internal/serve/v1.go`.
- `deskd/internal/domain/confidence.go`: the three assessment constructors.
- `docs/three-state-instrument-rule.md`.

## Ground rules

- Isolated branch and draft PR. No push to main, no merge, no release, no workflow trigger.
- Read-only. The service gains no write path, no new credential and no process launch. The
  package reads files under the configured directories and nothing else: each directory is
  opened through a root that refuses a path, or a symbolic link, leaving it.
- No planning row goes to a model backend. Do not scope the question-answering feature's use of
  these two tools; remove them from its allow-list.
- Do Task step 1 first and stop there if it fails. Do not change the image build, the release
  workflow or any other module's manifest to make it pass; that is option 2 of the human
  decision, and it is not this brief's to take.
- Unknown is not empty. A cell with no configured directory, an unreadable directory or a
  partial view answers could-not-check with the reason, exactly as the reads do today.
- Filter before counting. No count, total or error text may reveal a row the caller may not read.
- A working tree is not the remote. Every answer names the directory it was read from and when,
  and never claims to be the head of the repository's default branch.
- Stop at `implemented`. A different, non-implementing identity verifies and moves the brief on.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Prove the module route.** In `deskd/`, with `GOWORK=off` and no `replace`, add a
   requirement on the statusgen module, and on each sibling module the board packages import,
   at exact commit pseudo-versions, and build a throwaway program that calls the board loader.
   Account for the edge that already exists: either leave the `tools/desk` pin where it is, or
   show that the same route resolves it at a newer revision whose manifest requires statusgen.
   The criterion is `go build` and `go mod verify` both succeeding from `deskd/` alone. Record
   the result on the brief's decision issue. If it fails, STOP: report the exact resolution
   error and wait for the human decision.
2. **Tree source.** Add a `--roots` flag taking the same `<owner>/<repo>=<path>` list the desk
   tools use. The source maps each configured cell repository to a filesystem confined to that
   directory, or to "not configured". A symbolic link that leaves the directory is refused, and
   the file behind it is reported unreadable. It reads the map at start and on each poll cycle.
3. **One loader, refreshed on the poll cycle.** Hold one board loader for the process. On each
   poll cycle load every cell's configured repositories once and keep the resulting view in
   memory; requests are served from that view and never trigger a load. Memory is bounded by the
   loader's own bound.
4. **Mapping.** Map a view to the two responses, taking the caller's allowed repository set as
   an argument and dropping every other row first.
   - Roadmap: per stream, its status and priority, counts of briefs by lifecycle state, the count
     held or blocked, and the Next-up rows from statusgen/42.
   - Portfolio: per repository, the same counts summed, the count of awaiting rows whose class is
     a human action, and each stream's declared model routing (the view's `tiering` field,
     which statusgen/39 carries; absent stays absent). Last-record age is reported as
     could-not-check: the view does not carry it.
   - Facts the projections ask for. The service answers each one explicitly, so a missing fact
     never reads as "nothing outstanding". Drive manifests and verify-outcome records are files
     in the tree and are read through the same confined filesystem. Claims, last touch and
     delivery history need git or the forge, which the service does not read here: each is
     answered could-not-check. The Next-up rows are then marked an unfiltered superset with
     the claims state not known, and an awaiting row whose class depends on a missing fact is
     marked not valid, never dropped. The response's assessment lists the facts it lacked.
5. **Serve.** `GET /v1/roadmap`, `GET /v1/portfolio`, the un-versioned roadmap handler, the
   roadmap page and the two agent tools return the mapped rows with an assessment that names the
   directory and the load time. Each surface derives the allowed repository set itself and hands
   it to the mapping:
   - The two agent tools change to cell scope. Their allowed set is the repositories of the cell
     the scope admitted. Update their descriptions. Remove both names from the `unscoped`
     allow-list in `deskd/internal/mcp/authz_test.go` (line 155), so the existing test fails if
     either tool is ever returned to `ScopeNone`, and add the refusal test of Verify row 3.
   - Update the tile labels in `deskd/web/panes.go` that call these reads "(delegated)".
   - Each HTTP and page handler builds its allowed set from the viewer's per-repository
     authorizer, asking it about every repository of every configured cell, the same check
     `cellAuthorized` makes. With no authorizer wired, the set is every configured repository,
     as for the service's other reads. No handler passes "every repository" for any other reason.
   - An error on these surfaces names no stream, repository or path outside the allowed set.
   Regenerate `openapi.yaml` and the typed client declarations with the existing generator.
6. **Close the model-backend route.** Remove `desk__roadmap` and `desk__portfolio` from the
   question-answering feature's tool allow-list in `deskd/internal/chat/binding.go` and add both
   names to its forbidden list, so a later registry cannot bind them back without a code change.
7. **Documents.** Describe the flag, the directory source and its limits, the three
   authorization boundaries and the removed chat binding in `deskd/README.md`. Add the changelog
   fragment.

## Interface contract

```
deskd --cells <cells.yaml> --roots <owner>/<repo>=<path>[,<owner>/<repo>=<path>...]
```

`GET /v1/roadmap` (abridged; the envelope and the `assessment` field are unchanged, `streams`
carries rows where it carried null):

```json
{"streams":[{"id":"<alias>:<stream>","repo":"…","status":"…","priority":"…",
             "counts":{"todo":0,"in-progress":0,"implemented":0,"verified":0,"done":0},
             "held":0,"next_up":[{"brief":"…","score":0}],"claims_known":false}],
 "assessment":{"confidence":"…","probe":{"source":"board library over <path>","method":"…","window":"…"}}}
```

## Verify

Commands run from this repository root. The named tests are planned deliverables. Each row greps
the exact PASS line, so a selector that matches nothing fails the row.

| # | Class | Env | Command | Expect |
|---|-------|-----|---------|--------|
| 1 | check:ci | tree | `(cd deskd && GOWORK=off go build ./... && GOWORK=off go mod verify)` | exit 0; output contains "all modules verified". The service module builds from its own directory with the board library required and no `replace` |
| 2 | check:ci | tree | `(cd deskd && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBoardReadRows$" ./internal/boardread/ > "$check_output" && grep -F -- "--- PASS: TestBoardReadRows " "$check_output")` | exit 0; PASS line printed. Over two fixture trees the roadmap rows carry each stream's counts by lifecycle state and its Next-up rows, the portfolio rows carry the summed counts, the human-action count and each fixture stream's `tiering` value, last-record age is could-not-check, and with claims, last touch and delivery history unavailable the Next-up rows are marked an unfiltered superset, an affected awaiting row is marked not valid and kept, and the assessment lists the three missing facts |
| 3 | check:ci | tree | `(cd deskd && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBoardToolsRefuseForeign$" ./internal/mcp/ > "$check_output" && grep -F -- "--- PASS: TestBoardToolsRefuseForeign " "$check_output")` | exit 0; PASS line printed. The planned test dispatches `desk__roadmap` and then `desk__portfolio` with a cell the caller is not a loop of and requires a forbidden error from each with its handler never run; it also requires that neither name is in the `unscoped` allow-list of `TestEveryDataToolIsCellScoped`. If either tool is left at `ScopeNone`, the dispatch reaches the handler and the test fails |
| 4 | check:ci +mutation | tree | `(cd deskd && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBoardReadFiltersFirst$" ./internal/boardread/ > "$check_output" && grep -F -- "--- PASS: TestBoardReadFiltersFirst " "$check_output")` | exit 0; PASS line printed. With the handler's scope check bypassed in the test, the mapping given cell A's repository set returns no row, count or stream name from cell B's tree; with the filter removed by the test hook the same assertion fails |
| 5 | check:ci | tree | `(cd deskd && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBoardReadUnknown$" ./internal/boardread/ ./internal/serve/ > "$check_output" && test "$(grep -c -F -- "--- PASS: TestBoardReadUnknown " "$check_output")" -eq 2)` | exit 0; the PASS line is printed once for each of the two packages, so a test present in only one fails the row. A cell with no configured directory, an unreadable directory and a partial view each answer could-not-check with a reason and never an empty list; a symbolic link inside a configured tree that points outside it is refused and the bytes behind it never appear in a row; a request between poll cycles does not trigger a load |
| 6 | check:ci | tree | `(cd deskd && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBoardReadImports$" ./internal/boardread/ > "$check_output" && grep -F -- "--- PASS: TestBoardReadImports " "$check_output")` | exit 0; PASS line printed. The package imports the board library and the service's domain package and no other service package, launches no process and opens no network connection |
| 7 | check:ci +neighbour | tree | `(cd deskd && GOWORK=off go test -count=1 ./...)` | exit 0; every existing service test passes, including the authority test that keeps the human-answer and adopt tools absent, the route allow-list test and the generated-schema drift test |
| 8 | check +flow +dereference | host | `(cd deskd && GOWORK=off go build -o "${TMPDIR:-/tmp}/deskd-10" ./cmd/deskd) && (cd deskd && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && DESKD_BIN="${TMPDIR:-/tmp}/deskd-10" DESKD_ROOT="$(cd .. && pwd)" GOWORK=off go test -count=1 -v -run "^TestRoadmapFlowRealTree$" ./cmd/deskd/ > "$check_output" && grep -F -- "--- PASS: TestRoadmapFlowRealTree " "$check_output")` | exit 0; PASS line printed (a skipped test prints no PASS line and fails the row). The built service, started on loopback with this repository as the one configured root, answers the roadmap route with a row for the `cell-api` stream whose brief counts equal the counts in that stream's own README table (flow row: flag, source, loader, mapping, route) |
| 9 | check | host | `make deskd-image` | exit 0, or exit 2 with a line starting "deskd-image: could-not-check" where no container runtime is reachable. Under option 1 of the human decision the image builds from the service directory alone |
| 10 | gate:model +dereference | tree | `grep -n -F -e "--roots" deskd/README.md && grep -n -F -e "roadmap" deskd/internal/contract/openapi.yaml` | exit 0; the reviewer then reads the matched sections: the README states the directory source, that a working tree is not the remote, and what stays could-not-check; the schema describes typed rows and no longer says "until wired" |
| 11 | check | tree | `(cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg-ca10" .) && "${TMPDIR:-/tmp}/sg-ca10" --root . --lint` | exit 0; output contains "LINT: PASS". If the run reports a problem on a brief this delivery did not touch, the row is could-not-check for that reason, never passed: record the problem lines and re-run where the tree lints clean, as the repository's own lint check does |
| 12 | check +flow | tree | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && "${TMPDIR:-/tmp}/sg-ca10" --root . --consumers --brief cell-api/10 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0, post-merge: `ASSAY_DELIVERY_SHA` is the merged delivery commit and `ASSAY_DELIVERY_BASE_SHA` its first parent. The report corroborates each `consumers:` line above with no unchecked obligation; record its exact counts |
| 13 | check:ci +mutation | tree | `(cd deskd && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBoardReadSurfacesDeny$" ./internal/serve/ ./web/ > "$check_output" && test "$(grep -c -F -- "--- PASS: TestBoardReadSurfacesDeny " "$check_output")" -eq 2)` | exit 0; the PASS line is printed once for each of the two packages. With trees configured for cells A and B and an authorizer wired that denies cell B's repository, `GET /v1/roadmap`, `GET /v1/portfolio`, the un-versioned roadmap route and the roadmap page each return cell A's rows and no row, count, stream name or error text from cell B's tree; a handler variant that hands the mapping every configured repository fails the same assertion |
| 14 | check:ci | tree | `(cd deskd && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestChatBindingNoBoardReads$" ./internal/chat/ > "$check_output" && grep -F -- "--- PASS: TestChatBindingNoBoardReads " "$check_output")` | exit 0; PASS line printed. Neither `desk__roadmap` nor `desk__portfolio` is in the question-answering feature's allow-list, both are in its forbidden list, and a registry that advertises them cannot be bound |

| Failure mode of the work | Caught by |
|---|---|
| The module cannot be resolved from the service's own build, found late | row 1, and Task step 1 runs before any other step |
| An agent caller sees another cell's rows or counts | row 3 (scope) and row 4 (filter, with the scope bypassed) |
| A web viewer sees rows from a repository they may not read, because a handler built the wrong allowed set | row 13, across all four HTTP and page surfaces |
| Planning rows reach a model backend through the question-answering feature | row 14 |
| A symbolic link in a configured tree exposes a file outside it | row 5 |
| An unconfigured or unreadable tree is served as "no work" | row 5 |
| A request triggers a full load and the service stalls | row 5 |
| The package grows a dependency that blocks moving it to the public composition | row 6 |
| The image build silently breaks | row 9 |
| The mapping counts differently from the board | row 8, against the stream's own table |
| A tree that is behind its remote is presented as current | review-only: row 10 has the reviewer read the stated limit; the assessment names the directory and load time |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: human (from frontmatter — sensitive-data is yes). The sign-off confirms the three
boundaries shown by rows 3, 4, 13 and 14 and records which option of the human decision was taken.
Reviewer records verdict + date in the stream README table.
