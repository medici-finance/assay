---
brief: assay:assay:forge-neutral:24
title: Served claim store — the same directory store behind a small HTTP serve mode, member-initiated, holding no forge credential
why: >-
  A cell that spans hosts — a laptop plus cluster pods — cannot share a directory, and today
  its only option is forge-stored claims and the repository write they require. Serving the
  same directory store over HTTP gives such a cell one arbiter without any forge grant.
  Because it sits on the dispatch critical path and is reachable over a network, its
  authentication, its bind default and its behaviour when unreachable have to be exact.
wave: 4
depends: ["forge-neutral/23"]
unblocks: ["forge-neutral/28", "forge-neutral/30"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: [1267]
schema: brief-v2
authored: 2026-09-17 by forge-neutral authoring session (issue 1267)
sources:
  - "the rulings of 2026-09-17 recorded in the spec's §10 — this brief is written on them (D: the minimal serve mode ships in this repository's desk tools; H: anything in a container or pod uses this store, permanently)"
  - "#1267 — the problem statement, the driver's direction of 2026-09-17, and the required spec contents"
  - "docs/streams/forge-neutral/reviewer-write-boundary.md — the scoping doc this brief implements; section numbers below refer to it"
  - "the spec's §4.2 — the properties this brief implements: member-initiated only, one-way reachability, the placement rule, no forge credential, cell token, loopback default, fail closed"
  - "tools/desk/cmd/deskdispatch/dispatch.go:94-98 — `itemKeyRe`, the claim key grammar the serve mode re-validates"
  - "docs/cellctl.md:37-38 and tools/cellctl/cellctl:1990 — the per-cell service the docs mention is built outside this tree, which is why a minimal serve mode ships here (spec §10 D)"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "questions (a) and (c): a new wire contract plus a network listener; an ambiguous reply mapped to the wrong claim state, or a request accepted without the token, survives every happy-path test"
gate-why: >-
  This brief adds a network service that arbitrates dispatch claims for a cell — claim
  custody. It is one brief rather than two because client and server share one wire contract
  and neither is verifiable end to end alone. The human confirms it holds no forge
  credential and mints nothing, refuses any request without the cell token before parsing
  it, binds to loopback unless configured, refuses a non-loopback bind without TLS, and that
  an unreachable service stops dispatch rather than selecting another store.
decision-trigger: start
domain: complicated
consumers:
  - "tools/desk/cmd/deskclaim-ref (the `serve` verb and the service client): follow-up forge-neutral/24 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/claimstore_service.go: follow-up forge-neutral/24 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/README.md: follow-up forge-neutral/24 (this brief; flips to fixed-here when the implementation edits the path)"
  - "cell launchers that export DESK_CLAIM_SERVICE and DESK_CLAIM_TOKEN_FILE: follow-up forge-neutral/28"
  - "an external cell service implementing the same API: out-of-scope (it lives outside this tree; this brief publishes the API it would implement)"
version: 1
id: 83a44057-bb03-4c92-9024-c280d958493e
---

# Brief 24 — Served claim store

## Context
files:
- `tools/desk/internal/deskkit/claimstore_service.go` (planned) — the client backend.
- `tools/desk/cmd/deskclaim-ref/serve.go` (planned), `serve_test.go` (planned) — the serve mode.
- `docs/desk-tools/claim-service-api.md` (planned) — the API, so another service can implement it.
- `tools/desk/internal/deskkit/mutations.json`; `tools/desk/README.md`;
  `changelog/<branch-slug>.md` (planned)

single-point-of-failure: the cell-token check in front of the handler — behind it, the accept
set (six verbs, key grammar, rostered repos — a request that passes the token still cannot
name a path or an arbitrary operation) and the loopback bind default (an unconfigured service
is not reachable from another host at all). Different components, different failure reasons.

facts:
- One claim logic. The serve mode is a listener in front of a `file` store on its own disk;
  the client is a `ClaimStore` whose methods are HTTP calls. Both pass the conformance table.
- Six operations: acquire, progress, release, steal, show, list. JSON bodies, strict parsing:
  unknown field, unknown verb, oversize field or a key outside the grammar is a typed refusal.
- **Member-initiated only.** The service never opens a connection to a member and keeps no
  member address. Liveness is the existing TTLs. The requirement is one-way reachability.
- **Placement rule** (README and API doc): the service runs where the least-reachable member
  can still reach it. A laptop-only cell needs no service — `file` covers it.
- **No forge credential.** The serve mode never reads an App key, a role token or the token
  cache, and links no forge backend. A test asserts its import graph excludes them.
- **Cell token**: read from the 0600 file named by `DESK_CLAIM_TOKEN_FILE` on both sides; a
  file with wider permissions is a refusal; compared in constant time; never logged. Checked
  before the body is read.
- **Bind**: loopback unless an address is configured. A non-loopback bind without TLS
  configured refuses to start.
- **Client outcomes**: connection failure, timeout, non-2xx without a typed body, malformed or
  mismatched reply → `unverifiable` (exit 6). Never acquired, never free, never another store.
- The token authenticates membership of the cell, not the role; the owner recorded on a claim
  is what the member declares, as on every store today. One audit line per request.
- The single-host declaration is NOT required for `service`: the service is the single arbiter.
  The mixed-store refusal of brief 23 applies unchanged for as long as the removal window lasts.
- Ruled (spec §10 H): this is the store for **anything running in a container or pod,
  permanently** — there is no shared-volume alternative. The serve mode itself is a plain
  process over a `file` store on local disk; when it is run in a container, its own directory
  is that container's private volume, never one shared with members.

## Human decision
A cell whose desks run on more than one computer cannot share a folder. The proposal adds a
small service, shipped with the desk tools, that keeps the cell's dispatch claims and answers
six simple requests about them. Desks always call the service; the service never calls a desk,
so it only has to be reachable from every desk, and it should run wherever the hardest-to-reach
desk can still reach it. It holds no credentials for the code-hosting platform and cannot write
to it. Desks prove they belong to the cell with a shared secret kept in a protected file. By
default the service listens only on its own computer; listening on a network requires
encryption to be configured. If the service cannot be reached, dispatch stops; the tools never
quietly use different storage instead. The decision is whether to ship this service here.

Options:
1. **Approve as specified** — ship the minimal service in the desk tools.
2. **Publish the interface only** — no service ships here; cells that span computers must
   supply their own or keep claims on the platform.
3. **Reject** — cells that span computers keep claims on the platform.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Do not start until `docs/streams/forge-neutral/reviewer-write-boundary.md` reads `**Status:** approved`.** Verify row 1 checks it. A
  `draft` spec means the questions in its §10 are still open; building on a default that is
  later ruled the other way is rework on the identity chain.
- No token value, address or key path enters the tree, a fixture, a log line or a message. Tests generate their own token in a temp dir.

## Task
1. API document first: operations, bodies, typed refusals, status codes, the four properties.
2. Serve mode over the `file` store; token check before parse; accept set; audit line; bind
   rules.
3. Client backend; outcome mapping per the facts; add to the conformance table.
4. Resolver preconditions for `service`: both keys set, token file 0600, service answering a
   list call. Unmet → exit 6 before any worktree.
5. Mutation entries: (a) token check after parse and ignored on `show`; (b) timeout mapped to
   acquired; (c) non-loopback bind allowed without TLS.
6. README: running it, the placement rule, what an outage looks like to a member.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | `1` — the spec this brief implements is approved; `0` means STOP, do not start |
| 2 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestClaimStoreConformance' -count=1 -timeout 300s -v` | exit 0; the `service` backend (client against an in-process serve mode) passes every row (spec V1) |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskclaim-ref/ -run 'TestServeRefusals' -count=1 -timeout 120s -v` | exit 0; distinct typed refusals for: no token, wrong token, unknown verb, bad key, repo not allowed, unknown field — and no audit line leaks the token |
| 4 | check:ci | `cd tools/desk && go test ./cmd/deskclaim-ref/ -run 'TestServeBindRules' -count=1 -timeout 120s -v` | exit 0; default bind is loopback; non-loopback without TLS refuses to start |
| 5 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestServiceClientOutcomeMapping' -count=1 -timeout 120s -v` | exit 0; refused connection, timeout, malformed reply each → unverifiable (spec V6) |
| 6 | check:ci +mutation | the three `mutations.json` entries named `claimserve-token-after-parse`, `claimclient-timeout-reads-as-acquired`, `claimserve-open-bind-without-tls` | each reddens its row (3, 5, 4) |
| 7 | check:ci | `cd tools/desk && go test ./cmd/deskclaim-ref/ -run 'TestServeLinksNoForgeCredentialCode' -count=1 -timeout 120s` | exit 0 — the serve mode's import graph excludes the token mint, custody and forge backends |
| 8 | check +flow | Start the serve mode on loopback with a temp token; with a config home whose reviewer grant records repository **read**: run a review dispatch against a fixture repository; then stop the service and repeat | first run: `store service`, claim visible to `deskclaim-ref show` and `desksupervise status`, no ref written to the fixture (spec V5); second run: exit 6 before any worktree, message names the service as unreachable, no other store used (spec V6) |
| 9 | check +dereference | Compare each operation and refusal in `docs/desk-tools/claim-service-api.md` (planned) with a real request to the running serve mode | every documented status code and refusal token matches what the service returns |
| 10 | check | `(cd statusgen && go build -o /tmp/statusgen-fn24 .) && /tmp/statusgen-fn24 --root . --consumers --brief forge-neutral/24` | exit 0 |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| A request without the token is served | rows 3, 6(a) |
| A lost reply is read as success and two dispatchers proceed | rows 5, 6(b) |
| The service is exposed on a network in clear | rows 4, 6(c) |
| The service grows a forge credential "to be helpful" | row 7 |
| An outage silently moves dispatch to another store | row 8, second half |
| The API document drifts from the service, and an external implementation follows the document | row 9 |
| The service is placed where some members cannot reach it | review-only — placement is an operator decision; the README states the rule and row 8's refusal is what the operator sees |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`; a network service arbitrating claims). Reviewer records
verdict + date in the stream README table.

Reviewer questions for an identity-chain brief, answered in the verdict:
1. What single control stands between a process outside the cell and the cell's claims? (The cell token; beneath it the accept set and the loopback default.)
2. Does any row prove the lower layer with the upper bypassed? (Row 6 removes each control in turn; row 8 runs with a credential that cannot write to the forge and then with the service absent.)
