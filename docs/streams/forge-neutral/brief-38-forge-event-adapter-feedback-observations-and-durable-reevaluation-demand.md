---
brief: assay:assay:forge-neutral:38
title: Forge event adapter, feedback observations and durable reevaluation demand
why: "Review feedback that arrives while work is running is easy to lose: a missed notification, an out-of-order delivery or a crash at the wrong moment means the comment is never acted on, or is acted on twice by competing writers. Turning each refreshed piece of feedback into one durable, generation-bound reevaluation requirement means it is acted on once, by the right attempt, even across restarts and lost events."
wave: 8
depends: ["forge-neutral/37", "graph-execution/28", "graph-execution/29", "graph-execution/19", "graph-execution/04"]
unblocks: ["forge-neutral/45"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "sensitive-data is yes because inbound forge deliveries and the comments and check results they point at carry private repository content, and the adapter verifies deliveries with a shared secret. The human confirms that an unauthenticated delivery records and triggers nothing, that an event is only a prompt to re-read and never approval, and how much of a delivery's payload is kept (the decision below)."
issues: []
outcome: none
schema: brief-v2
version: 2
id: 3542bd60-8abe-4927-ae46-fb3cd5d3e1f8
authored: 2026-10-13 by statusgen newbrief (non-CI R6 authoring follow-up)
sources:
  - docs/proposals/non-ci-architecture/20-amendment-manifest.md — R6 owner row and the "R2–R8 amendment scope" R6 paragraph ("translate refreshed feedback into durable reevaluation; partial scans stay incomplete")
  - docs/proposals/non-ci-architecture/16-delivery-sequence.md — package R6 (lost/reordered events preserve uncertainty)
  - docs/proposals/non-ci-architecture/14-storage-forge-graph.md — "Reuse and expand the existing forge-mirror work" (authenticated deliveries, delivery IDs, reorder tolerance, polling repair) and "Feedback received during an active attempt"
  - docs/proposals/non-ci-architecture/15-regression-from-day-one.md — webhooks duplicate, arrive out of order or are missed; event resume and dedup detect retention gaps
  - "medici-finance/assay#2500 — maintainer ratification of the whole non-CI target @ 23f655f5d (approval activates nothing)"
  - "medici-finance/assay#2739 — the step-3 amendment manifest (merge 025afaf6b)"
  - docs/streams/forge-neutral/brief-37-scoped-forge-mirror-with-persisted-freshness-coverage-and-polling-repair.md — the mirror, refresh planner and coverage states this brief feeds and reuses
  - docs/streams/graph-execution/brief-28-verifier-domain-contracts.md — qualified work identity, candidate and reevaluation generation
  - docs/streams/graph-execution/brief-04-recovery-contract.md — intent and outbox handoff (as amended by the R2 follow-up)
  - freshness-checked 2026-10-13 @ 4696d598e (origin/main)
exec-tier: strong
exec-tier-why: "Question (c), silent failure toward a pass: a delivery accepted before it is durable, a reordered event that rolls state back, or an older run clearing a newer feedback generation each pass a single-delivery test and lose or duplicate work."
domain: complicated
decision-trigger: start
consumers:
  - "forgeread/events/ (new): follow-up forge-neutral/38"
  - "forgeread/mirror/: follow-up forge-neutral/38 (comment and check/status observation kinds; hint intake into the refresh planner)"
  - "tools/desk/internal/deskkit/mirrorsource.go: follow-up forge-neutral/38 (feedback kinds composed over the existing `ListCommentsTyped` and `ChecksAtHead`; no new operation)"
  - "workflow/store/ (as planned by graph-execution/19 after its R2 amendment): follow-up forge-neutral/38 (the delivery inbox table, the use of R2's pending-feedback record and the mapping from a mirror observation to the graph-execution/28 work identity; no second demand table)"
  - "statusgen/: out-of-scope (forge-neutral/18 owns statusgen's forge reads; this brief adds no statusgen link and no deskread kind)"
  - "a mounted receiver route, network listener or deployment: out-of-scope (R7's public API and the activation sequence own mounting and exposure; this brief ships a library and an in-process test handler only)"
  - "tools/forge-import-check/: fixed-here (source-only verification helper relocation; not a runtime mirror implementation)"
---

# Brief 38 — Forge event adapter, feedback observations and durable reevaluation demand

## Context

files: `tools/forge-import-check/check_env_imports.go` (source-only helper), `tools/forge-import-check/check_env_imports_test.go` (source-only compiler fixtures), `forgeread/events/` (new), `forgeread/mirror/` (feedback kinds, hint intake),
`tools/desk/internal/deskkit/mirrorsource.go` (planned) and tests, `workflow/store/` (planned by
graph-execution/19; delivery inbox, the pending-feedback use and the graph-execution/28 mapping),
`forgeread/README.md`,
`changelog/forge-events-reevaluation.md` (new).

facts (4696d598e, 2026-10-13):
- forge-neutral/37 supplies the mirror, its refresh planner, coverage states and `mirror.Store`;
  this brief adds kinds and an intake path, not a second planner or store.
- deskkit already reads typed comments (`ListCommentsTyped`, issue or change target) and check
  rollups at a commit (`ChecksAtHead`) on both backends. This brief composes them and adds no
  operation to the frozen set.
- graph-execution/28 defines qualified work identity, candidate and a reevaluation generation;
  stale generations are refused. Its Go types are planned in `statusgen/domain`, and it defines no
  pull or merge request identity.
- `forgeread/go.mod` has no `require` and no `replace`, and statusgen requires forgeread through a
  relative `replace`. A forgeread import of a statusgen package would be a module cycle, so the
  mapping from a mirror observation to graph-execution/28 records happens only in the `workflow`
  module, which links both.
- The R2 follow-up amends graph-execution/19 to carry durable pending feedback in the PostgreSQL
  store, and amends graph-execution/04 for a transactional intent and outbox handoff. This brief
  writes through those records; it does not define its own.
- No inbound receiver exists in the repository, and none is authorised by this brief. An earlier
  design or this brief's approval does not authorise a public inbound receiver or broader machine
  authentication (doc 14).

layering: `forgeread/events` is pure. It takes a delivery's header values, body bytes and a secret
supplied by composition, and returns an authenticated hint or a refusal. It has no HTTP server,
no client, no process launch and no direct environment read: no shipped `events` file imports `os` or
`syscall`. A cgo route through the `"C"` pseudo-package is outside what the import check parses. HTTP framing, secret custody and mounting belong to the host that later exposes it.
Rows 2, 9, 10 and 13 hold the split; row 13 parses and decodes import paths to enforce this direct-import boundary.

design-fit:
  owner: forgeread/events (delivery authentication and hint extraction); forgeread/mirror (intake and feedback kinds)
  contract: "forge event hint v1 — provider, delivery ID, event kind, subject identity, observed time; versioned with forge mirror v1"
  retires: [nothing in this brief; per-verb inbound polls retire only when their consumers move in later briefs]
  weight: "verbs 0, flags 0, refusals 1 (unauthenticated delivery); one library package, one store table"
  why-add: "Polling alone repairs freshness but adds latency to every feedback loop. An authenticated, deduplicated hint path that only prompts a re-read keeps the mirror the single source of state and keeps events from becoming a second authority."

single-point-of-failure: the reevaluation generation check at the R2 pending-feedback record:
admission consumes a known generation, and a consumption naming an older generation is refused.
Behind it are the store's transaction that records demand together with attempt ownership and the
outbox (row 6, with the library call bypassed), and the refresh-before-act rule: a successor
attempt re-reads authoritative state through the mirror before reevaluating, so a replayed hint
can never carry instructions of its own (row 7).

Read first: `docs/proposals/non-ci-architecture/14-storage-forge-graph.md` §"Feedback received
during an active attempt", forge-neutral/37, graph-execution/28 §Task 1, graph-execution/04.

## Relationship to forge-neutral/18 — 2026-10-13

forge-neutral/18 is in progress. This brief does **not** rebind it: /18's Task, Verify, sites,
dependencies and completion test are unchanged. No statusgen file, `deskread` kind or statusgen
import is added, and statusgen cannot link `forgeread/events` or `forgeread/mirror`, which are
subpackages (row 10). Statusgen's credential-free online reads stay on /18's `deskread` transport.

## Toward R7 — 2026-10-13

R7 (public API and thin cockpit) has no owner yet, so no R7 brief exists to name in `unblocks:`.
This brief supplies two things R7 consumes: the hint intake that a mounted receiver route calls,
and the pending-feedback visibility the cockpit shows. When the R7 briefs are authored, the one
that mounts the receiver and the one that shows pending feedback each declare
`depends: ["forge-neutral/38"]`, and `statusgen newbrief` writes the inverse edge here. Mounting,
exposure and receiver credentials stay with R7 and the activation sequence.

## Human decision

When a forge notifies the system that something changed (a new comment, a review, a closed issue),
the delivery carries a copy of private repository content. This design treats every delivery as a
prompt to re-read the current state from the forge, never as the record itself. The question is
how much of each delivery to keep.

Options:
1. **Hints only**: keep the delivery ID, provider, event kind, the subject it names and the
   receive time, then discard the body after authentication and parsing. Current state always
   comes from a re-read. The private-data footprint is smallest. A comment edited or deleted
   before the re-read is lost, because the earlier text was never kept.
2. **Hints plus retained payload snapshots**: also keep the delivery body for a configured
   window. Earlier text survives an edit or deletion for that window, at the cost of a larger
   store of private content and a retention and purge obligation.

Default if no answer: none. The event adapter stays disabled (polling alone keeps freshness) until
answered.

## Ground rules

- Offline fixtures and fake provider transports only; real PostgreSQL only in the disposable test
  harness graph-execution/19 provides after its R2 amendment. No listener, mounted route,
  deployment, provider webhook registration or production secret. The adapter ships disabled.
  Approval activates nothing.
- No new operation in the frozen forge set. Feedback kinds compose existing reads.
- `forgeread` stays a module with no dependencies: no `require` or `replace` in `forgeread/go.mod`
  and no import of a statusgen or `workflow` package (row 14). No shipped `events` file imports `os`
  or `syscall`; the secret and all configuration arrive as parameters from composition (row 13).
- An event never grants approval, admission, acceptance or a human decision; it only requests a
  refresh.
- If the R2 amendments to graph-execution/19 (pending feedback) or /04 (intent and outbox) have not
  landed, stop and report NEEDS_CONTEXT. Do not build a substitute demand table or outbox.
- Stop at `implemented`; an independent verifier owns verified and done.

## Task

1. **Delivery authentication (`forgeread/events`).** Verify each provider's documented delivery
   authentication with a constant-time comparison: GitHub's HMAC-SHA256 signature header and
   GitLab's secret token header. The secret comes from composition. Refuse missing, malformed or
   wrong credentials before parsing, and record or trigger nothing for a refused delivery. Parse an
   authenticated delivery into a hint: provider, delivery ID, event kind, subject (qualified
   repository plus change or issue number), provider event time. Payload handling follows the
   recorded human decision; row 15 pins the chosen option.
2. **Durable intake.** `Accept` writes the hint into a store inbox keyed by (provider, delivery ID)
   and returns acknowledgement only after that write commits. A duplicate delivery ID, including a
   provider redelivery, is acknowledged and records nothing new. Processing is asynchronous: hints
   coalesce per subject into forge-neutral/37 refresh requests. Unsupported event kinds are
   acknowledged, counted and dropped, and polling covers them. A detected delivery gap (a provider
   redelivery window passed, or an inbox outage) marks the affected scopes `partial` until /37's
   polling repair completes. Deduplication is not replay protection: GitHub's signature covers the
   body only, not the delivery ID header, and GitLab's token header authenticates the sender, not
   the body. A captured body replayed under a fresh delivery ID is therefore a new inbox row, but
   it creates no new work: it only prompts a refresh, and reevaluation demand is keyed on the
   refreshed observation, never on the delivery. The inbox purges rows a configured bound past the
   provider redelivery window, so replays cannot grow it without limit.
3. **Order and loss.** A hint never writes record state. The refresh reads current state, and the
   mirror keeps the newest provider-updated observation per record, so an older delivery processed
   after a newer one cannot roll a record back. A lost delivery is repaired by /37's overlap poll;
   the resulting feedback creates the same single reevaluation requirement it would have created
   from the event.
4. **Feedback observations.** Add comment observations (issue and change targets, provider
   identity, author identity and type, created and edited times) and check/status observations at
   the observed head revision to `forgeread/mirror`, composed in `mirrorsource.go` (planned) over
   `ListCommentsTyped` and `ChecksAtHead`. Missing check permission sets coverage
   `unavailable: permission` and never reads as "no failing checks".
5. **Reevaluation demand.** When a refresh observes new relevant feedback (a comment, review or
   check change) on a change bound to a work item with an active attempt, record one scoped
   reevaluation requirement in R2's pending-feedback record, in the same transaction as the
   attempt-ownership read and the outbox entry. The mirror reports the observed change by its own
   identity (forge-neutral/37 Task 1) plus the head revision. Composition in `workflow/store/`
   resolves the bound work item and writes the requirement with the graph-execution/28 work
   identity, the candidate, the observed source revision and the next reevaluation generation.
   `forgeread` imports no statusgen or `workflow` package.
   Admission consumes a known generation; later arrivals stay pending; an older run cannot clear a
   newer generation. Distinct instructions, decisions and effect requests stay separate attributed
   records: coalescing refresh demand references them and never merges their content or their
   authorization. A crash between an attempt's terminal state and the enqueue is repaired from the
   durable demand on restart, and a duplicate or replayed delivery cannot create a competing
   attempt.
6. **Events are not authority.** A hint naming an approval, a merge or a label change changes no
   acceptance, admission or human-gate state. Only the refreshed observation exists, and it is
   assessed under the existing gates.
7. **Docs.** Update `forgeread/README.md` (events package, disabled by default, no listener) and
   add the changelog fragment.

## Verify

Named tests are implementation deliverables and fail at authoring (they do not exist yet). Each one
keeps a fail-first mutation and exercises the production path. Fake provider deliveries and
transports only.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd forgeread && GOWORK=off go test -count=1 ./...` | exit 0: the whole module, `events` and the extended `mirror` included, passes with no workspace file |
| 2 | check:ci +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEventsRejectUnauthenticated$" ./events/ > "$result" && grep -F -- "--- PASS: TestEventsRejectUnauthenticated " "$result")` | exit 0; named PASS. For both providers, missing, malformed and wrong signatures or tokens are refused before parsing. The fake inbox records zero rows and the fake planner zero refresh requests, and a correctly signed control is accepted. **Mutation**: skip the signature check, and an unsigned delivery is recorded and the test fails. Fails at authoring (test absent) |
| 3 | check:ci +flow +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEventsDuplicateDeliveryOnce$" ./... > "$result" && grep -F -- "--- PASS: TestEventsDuplicateDeliveryOnce " "$result")` | exit 0; named PASS. The same delivery ID received three times (two of them provider redeliveries) yields one inbox row, one coalesced refresh and one reevaluation requirement. The same signed body replayed under a fresh delivery ID adds one inbox row but no second reevaluation requirement, because demand is keyed on the refreshed observation. Inbox rows older than the configured bound past the redelivery window are purged. **Mutation**: key the inbox on receive time instead of delivery ID, and duplicates multiply and the test fails. **Mutation**: key demand on the delivery, and the replay creates a second requirement and the test fails |
| 4 | check:ci +flow +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEventsReorderedNeverRegress$" ./... > "$result" && grep -F -- "--- PASS: TestEventsReorderedNeverRegress " "$result")` | exit 0; named PASS. A "closed" delivery processed after a later "reopened" delivery leaves the record open, because both trigger a refresh of current state and the newest provider-updated observation wins. **Mutation**: let a hint write state directly, and the record rolls back to closed and the test fails |
| 5 | check:ci +flow +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEventsLostRepairedByPoll$" ./... > "$result" && grep -F -- "--- PASS: TestEventsLostRepairedByPoll " "$result")` | exit 0; named PASS. A comment delivery is dropped. The next overlap poll observes the comment and creates exactly one reevaluation requirement, the same one the event would have created. A detected redelivery gap marks the scope `partial` until that poll completes. **Mutation**: advance the poll watermark on event receipt, and the dropped comment is never observed and the test fails |
| 6 | check:ci +flow +mutation | `(cd workflow && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestFeedbackDemandGenerationPostgres$" ./store/... > "$result" && grep -F -- "--- PASS: TestFeedbackDemandGenerationPostgres " "$result")` | exit 0; named PASS (a skip prints no PASS and fails the row). Against real PostgreSQL in the graph-execution/19 harness: feedback arriving while attempt A runs is pending at generation g+1. A's completion consumes only g, and g+1 stays pending. With the library call bypassed, a direct consumption naming g after g+1 exists is refused by the store. A crash injected between A's terminal state and the outbox enqueue is repaired on restart from the durable demand, with one successor attempt. **Mutation**: drop the generation predicate, and the older run clears g+1 and the test fails |
| 7 | check:ci +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEventHintIsNotAuthority$" ./... > "$result" && grep -F -- "--- PASS: TestEventHintIsNotAuthority " "$result")` | exit 0; named PASS. A signed delivery claiming an approval, a merge and a label leaves acceptance, admission and human-gate inputs unchanged; only a refresh request is produced. A replayed hint from a superseded attempt produces a refresh, not an instruction. **Mutation**: map an approval hint to an approval observation, and the test fails |
| 8 | check:ci +flow +mutation | `(cd tools/desk && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorFeedbackPermissionLoss$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestMirrorFeedbackPermissionLoss " "$result")` | exit 0; named PASS. For both provider fixtures, comments and checks compose through the existing operations. A 403 on the check read sets check coverage `unavailable: permission`, which reads as could-not-check and never as "no failing checks", and comments for the same change stay readable. **Mutation**: map a refused check read to an empty rollup, and the test fails |
| 9 | check +dereference +mutation | `test -d forgeread/events && (cd forgeread && GOWORK=off go list -deps ./events > "${TMPDIR:-/tmp}/b38-r9.out") && { grep -x -F -e net/http -e os/exec -e database/sql "${TMPDIR:-/tmp}/b38-r9.out" \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0`: the events package's closure has no HTTP server or client, no process launch and no database driver. A missing package prints nothing and fails the row (it fails at authoring). **Mutation**: an events file importing `net/http` counts `1` |
| 10 | check +dereference +mutation | `test -d statusgen && { grep -rnE --include='*.go' --exclude='*_test.go' '"([^"]*/)?forgeread/[^"]+"' statusgen \|\| [ $? -eq 1 ]; } \| { grep -v -E '^[^:]+:[0-9]+:[[:space:]]*//' \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0`: statusgen links no `forgeread` subpackage, `events` and `mirror` included, so forge-neutral/18 is not rebound. **Mutation**: a planted statusgen import of `.../forgeread/events` counts `1` |
| 11 | check:ci +mutation | `(cd tools/desk && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestForgeSurfaceUnchangedByDeskread$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestForgeSurfaceUnchangedByDeskread " "$result" && GOWORK=off go test -count=1 -v -run "^TestAmbientTokenGuardCoversForgeread$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestAmbientTokenGuardCoversForgeread " "$result")` | exit 0: the forge surface is unchanged by this brief (no new operation), and the ambient-token rule finds no forge-token read in `forgeread/`. That rule recognises only six forge-token variable names, so it does not guard the webhook secret; row 13 does. **Mutation**: add an operation to the forge surface, and the surface test fails. **Mutation**: an events file reading `GH_TOKEN` from the environment produces a finding, and the guard test fails |
| 12 | check:ci +dereference | `statusgen --consumers --root .` | exit 0: the diff-aware consumers gate corroborates every routing token against the implementing branch's diff (each `follow-up forge-neutral/38` self-reference is satisfied by this brief's own changes and flips to `fixed-here` at implementation) |
| 13 | check:ci +mutation | `GOWORK=off GOPROXY=off GOTOOLCHAIN=local go run tools/forge-import-check/check_env_imports.go forgeread/events` | exit 0 and output `0`: the shared source-only checker parses every non-test Go file below the target and decodes each import path with `strconv.Unquote`. No decoded `os` or `syscall` import is allowed, including raw, escaped, plain, aliased and dot imports. Build-constrained files are scanned regardless of the current platform; `*_test.go` files remain excluded. Missing/empty targets, parse/decode/walk errors and symbolic links fail closed with no clean count. It fails at authoring because the target package is absent. The companion `check_env_imports_test.go` compiles synthetic raw/escaped imports offline without calling their environment-read functions, then runs both rows' exact commands; ordinary aliases, build-tagged files, excluded tests and clean imports remain controls. **Mutation**: use the former literal matcher or stop decoding import literals; compiler-valid raw/escaped `os` and `syscall` plants evade rejection and the fixture test fails. **Mutation**: plant any of those imports in shipped `events` source; the row reports a nonzero count and exit status |
| 14 | check +mutation | `test -f forgeread/go.mod && { grep -n -w -e require -e replace forgeread/go.mod \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0`: `forgeread` keeps no module dependency, so `events` cannot import `statusgen/domain` or a `workflow` package, and the graph-execution/28 mapping stays in `workflow`. Passes at authoring as an invariant guard. **Mutation**: a planted `require github.com/medici-finance/assay/statusgen v0.0.0` line counts `1` |
| 15 | check:ci +mutation | `(cd workflow && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEventsPayloadRetentionPostgres$" ./store/... > "$result" && grep -F -- "--- PASS: TestEventsPayloadRetentionPostgres " "$result")` | exit 0; named PASS (a skip prints no PASS and fails the row). **Decision-conditional**: the worker implements only after the human decision is recorded, and the edit that records it deletes the unchosen option's sentence and mutation from this cell. Against real PostgreSQL in the graph-execution/19 harness, signed fixture deliveries carry private canary strings in their bodies. Option 1 (hints only): after acceptance and processing, a scan of every table the adapter writes finds no canary. Option 2 (retained snapshots): each canary is present within the configured window and absent after the purge runs past it. **Mutation (option 1)**: store the body in the inbox row, and a canary is found and the test fails. **Mutation (option 2)**: make the purge a no-op, and a canary survives the window and the test fails |

## Pre-mortem

An unauthenticated sender drives provider reads or records (row 2). Redeliveries multiply work
(row 3). An out-of-order delivery rolls state back (row 4). A lost delivery is never repaired, or
a gap is reported as complete (row 5). An older run clears newer feedback, or a crash between
completion and enqueue loses it (row 6). An event is treated as approval (row 7). Missing check
permission reads as green (row 8). The library grows a server or driver (row 9). statusgen starts
linking event or mirror code (row 10). A new operation slips in, or a forge token is read from the
environment (row 11). The webhook secret or other configuration is read through a direct `os` or
`syscall` import under any name (row 13; cgo is outside that check). `forgeread` gains a dependency on statusgen and a module cycle forms (row 14). A
captured body replayed under a fresh delivery ID creates a second requirement (row 3). Payload
retention remains the human decision above; row 15 pins whichever option is chosen.

## Evidence

<!-- Independent verifier records subject revision, results and retained negative controls. -->

## Review

Gate: human (sensitive-data). The reviewer confirms that unauthenticated deliveries are inert,
that events stay prompts and never authority, and records the payload-retention option chosen.
Approval activates nothing; the adapter ships disabled and exposure stays with R7 and the
activation sequence in `docs/proposals/non-ci-architecture/16-delivery-sequence.md`.

## Write-through follow-on — 2026-10-14

The new forge-neutral/39–45 briefs consume this contract. The service maps an authenticated
principal to this mirror's exact composition-derived admission; it does not relax the
query-time/store admission rule or answer the existing retention/payload decision. Confirmed
provider receipts feed mirror updates through the existing R2/R4 owners. This dated note and
reciprocal edges change no historical Task, Verify, Evidence or human decision.

## Checker packaging correction — 2026-10-14

Move the unchanged source-only checker and its compiler-fixture harness to
`tools/forge-import-check/`. The exact Verify commands now name that path. The desk CI
bridge references this neutral developer-tool path; its dependency registry covers the
helper plus the actual authored-row input through whole-stream-root coverage. No withheld
stream name ships in desk source, and neither corpus nor secret guard is exempted or weakened.
The standalone harness still reads and executes the actual two Verify commands and all
22 compiler cases. Human retention choices and historical Evidence remain unchanged.
