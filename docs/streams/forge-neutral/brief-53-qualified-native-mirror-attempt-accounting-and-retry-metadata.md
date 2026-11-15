---
brief: assay:assay:forge-neutral:53
title: Qualified native mirror attempt accounting and retry metadata
why: "A shared refresh must know the HTTP work it actually caused and when a provider permits another attempt. Typed native receipts and a pre-request budget guard prevent unknown usage or lost retry metadata from becoming a fresh allowance."
wave: 5
depends: ["forge-neutral/48", "forge-neutral/50", "forge-neutral/51"]
unblocks: ["forge-neutral/54"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "The new receipt crosses the admitted native transport and refresh boundary. Accept its demonstrated scope binding, safe metadata and bounded attempt semantics; it grants no credential, service exposure or shared-budget activation."
issues: []
schema: brief-v2
version: 5
authored: 2026-11-03 by statusgen newbrief
outcome: none
design: DR-forge-library
decision-trigger: spec
split-from: forge-neutral/37
exec-tier: strong
domain: complicated
exec-tier-why: "SDK retries, native response errors and independent budget/source checks must retain one truthful attempt boundary across adapters."
sources:
- "authoritative contract source: commit 9febd828b10a4a12bea41eb05ae66c0b2b7689bb; this authoring adoption carries no runtime implementation or acceptance claim"
- docs/streams/forge-neutral/brief-37-scoped-forge-mirror-with-persisted-freshness-coverage-and-polling-repair.md
- docs/streams/forge-neutral/brief-48-bound-native-mirror-response-allocation-before-decoding.md
- docs/streams/forge-neutral/brief-50-join-admitted-incremental-reads-to-mirror-observations.md
- docs/streams/forge-neutral/brief-51-store-injected-mirror-read-through-coordinator-and-shared-refresh-gate-port.md
- docs/library-first.md
- docs/go-engineering.md
- docs/performance-attribution-spec.md
- "freshness-checked 2026-11-03 against worker-App-fetched main 214b4c8308b3cc57beed91feccf45e6a03f2eecd; mapper 2677323c9bdf81aad1a1e3b4ad4407e0703ee25c and coordinator 18009487a85e7c1cde48778cdb2e3e265c0e0e54 are published; the later mixed-demand/Leave repair must be consumed before implementation"
- https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api
- https://docs.gitlab.com/administration/settings/user_and_ip_rate_limits/
consumers:
- "forgeread/mirror/types.go: fixed-here (typed native grant, individual observation and aggregate receipt)"
- "forgeread/refresh/types.go: fixed-here (shared ticket/completion receipt propagation)"
- "forgeread/refresh/coordinator.go: fixed-here (grant binding, receipt preservation and native observation-age bound)"
- "forgeread/refresh/coordinator_test.go: fixed-here (actual source/receipt flow and bounded-age controls)"
- "forgeread/README.md: fixed-here (qualified native boundary and separate canonical SQL obligations)"
- "docs/contracts.md: fixed-here (extend the same mirror owner without a second budget or authority)"
- "tools/desk/internal/deskkit/mirror_attempts.go: fixed-here (closed native attempt and retry boundary)"
- "tools/desk/internal/deskkit/mirror_attempts_test.go: fixed-here (actual SDK/transport controls and independent scope/failure expectations)"
- "tools/desk/internal/deskkit/mirroringest.go: fixed-here (trusted native source binding and qualified observation time)"
- "tools/desk/internal/deskkit/mirroringest_test.go: fixed-here (existing mapper contract compatibility)"
- "tools/desk/internal/deskkit/forge_response_bounds.go: fixed-here (one bounded native response observation before SDK return)"
- "tools/desk/internal/deskkit/forgetransport.go: fixed-here (preserve admitted native transport and TLS custody)"
- "tools/desk/internal/deskkit/citrigger_test.go: fixed-here (actual native out-of-module reader coverage)"
- "workflow/store canonical SQL attempt authority and joined proof: follow-up forge-neutral/54"
- "forgeread root, statusgen offline/frozen paths and current operation inventories: out-of-scope (this adds only the admitted mirror native boundary; root evaluators, offline/frozen inputs and operation inventories consume no new port)"
- "provider quota selection, credential custody, retention and service activation: out-of-scope (existing owners and gates)"

---

# Brief 53 — Qualified native mirror attempt accounting and retry metadata

## Context

files:
- `forgeread/mirror/types.go`
- `forgeread/refresh/types.go` (planned)
- `forgeread/refresh/coordinator.go` (planned)
- `forgeread/refresh/coordinator_test.go` (planned)
- `forgeread/refresh/boundary_test.go` (planned)
- `tools/desk/internal/deskkit/forge_response_bounds.go`
- `tools/desk/internal/deskkit/forgetransport.go`
- `tools/desk/internal/deskkit/mirrorsource.go`
- `tools/desk/internal/deskkit/mirroringest.go` (planned)
- `tools/desk/internal/deskkit/mirror_attempts.go` (planned)
- `tools/desk/internal/deskkit/mirror_attempts_test.go` (planned)
- `tools/desk/internal/deskkit/mirroringest_test.go` (planned)
- `tools/desk/internal/deskkit/forge_response_bounds_test.go`
- `tools/desk/internal/deskkit/citrigger_test.go`
- `forgeread/README.md`
- `docs/contracts.md`
- `changelog/mirror-native-accounting.md` (planned)

single-point-of-failure: entering the admitted native base RoundTripper with a valid attempt grant — the Source constructor and native closed-operation guard independently bind scope, and the canonical attempt authority independently checks/debits each reservation before the leaf can send.

facts:
- Actual50 ScanResult has observations/completeness/failure only. UpdateCoverage counts fetched pages and decoded JSON bytes, not SDK retries or every failed native attempt. A page, Scan invocation, reservation debit and HTTP attempt are different units.
- Actual51 Gate.Complete already owns the typed ApplyCompletion seam. Ticket/Completion lack native budget/receipt fields. Its mixed-demand and final-Leave repair is in progress at this read: consume its published Requirements/Outcome API before coding, never copy its fake gate or repair it here.
- The pinned GitLab client explicitly uses WithoutRetries and its no-limiter adapter; go-gh/v2 v2.13.0 REST calls use the HTTP client without a retry engine. No new retry engine is enabled to manufacture evidence. Actual48 native allocation guard carries status only through rejected errors. It deliberately retains no raw header/body/endpoint/credential. Existing status/cause/classification behavior is a compatibility obligation.
- Source53 supplies the port/leaf implementation and stateful budget fixtures only. Actual SQL budget/generation enforcement and real joined persistence belong to54. Full37 retains all23 Verify pairs, including joined persisted row23, and fullGE19 retains13.

layering: Reuse the existing admitted native transport, mapper and refresh coordinator. The pure mirror/refresh modules retain stdlib-only types and no HTTP/credential/SQL implementation. No provider walker, retry engine, general transport callback, endpoint selector or quota owner is added.

design-fit:
  owner: existing native leaf and admitted source; shared budget authority remains canonical R2
  contract: S-forge-mirror gains typed scoped attempt grant/receipt and a narrow BeforeAttempt authority port
  retires: []
  weight: "production verbs0; flags0; backends0; drivers0; one typed observation/attempt boundary"
  why-add: "The current page/scan counters cannot prove actual native work or a provider retry floor. Extend the existing leaf instead of copying SDK/provider accounting."

## Read first and pickup

Read current37,48,50,51,54, the actual selected SDK/native transport, library-first, Go engineering and perf-v3. Record immutable source revisions and continue/adapt/hold. Genuinely consume the published mapper and final51 Requirements/Outcome/Leave repair in an isolated implementation home after canonical dispatch. Authoring-only contracts or mutable worker bytes do not satisfy runtime pickup. Existing sources may be supplied by genuine feature merges before main; that supplies no mainline or acceptance credit. If the native leaf or required51 API is missing, hold and name the missing source rather than inventing a parallel port.

## Human decision

Accept or hold a demonstrated synthetic transport observation boundary. It counts actual admitted native request attempts and prevents a request without a valid bounded budget grant. Only typed status, count and retry timing facts leave the transport; no raw response header, body, address or credential is retained. Acceptance does not activate a budget, private ingestion, remote serving or a credential.

Options:
1. Accept the bounded contribution, retaining separate durable-budget, joined integration and activation gates.
2. Hold for a named correction to scope binding, accounting or safe retry timing.

Default if no answer: none — blocks until answered

This gates acceptance and activation. Offline implementation and synthetic fixture qualification may proceed.

## Ground rules

- Own isolated branch/draft; no main push, merge, ready flip, release, workflow dispatch, live provider, cluster or operational database contact. Export KUBECONFIG=/dev/null.
- Reuse current SDK retry mechanics and classification. No new retry loop, provider polling, raw endpoint API, credential lookup, body/header stash, shared cache backend or independent quota ledger.
- A grant is bounded refresh-work accounting, never authorization. Current Admission and closed native operation/source checks remain independent; only trusted composition supplies the authority and grant.
- Preserve full parent Tasks, all23/13 Verify pairs and every retention/custody/exposure gate. Stop at implemented; Evidence remains the independent verifier's.

## Typed interface contract

Add typed grant/receipt data to the existing ScanPlan/ScanResult and refresh Ticket/Completion flow. Define one opaque grant identity bound by the authority to source population, admission, canonical provider/host/repository/kind, generation, policy epoch, finite HTTP ceiling and expiry. Ticket carries the trusted grant; the coordinator attaches it after its final re-plan; callers cannot select it through query text. The Source constructor binds the narrow AttemptAuthority. Its BeforeAttempt operation validates/debits that grant and returns a canonical unique per-attempt reservation before each native base RoundTrip. ObserveAttempt accepts only the narrow typed evidence for that reservation. Any future configured SDK retry must traverse this same boundary; current production configurations have no hidden retry engine. It is not a generic callback, HTTP interface, SQL port, credential or action capability.53 defines this seam and qualifies its use against a stateful fixture;54 implements its only production authority in the existing R2 store.

An HTTP attempt means entering the existing native base RoundTripper. DNS/transport failure, no response and response+error may leave wire completion unknown; none means no attempt. An authority debit is a separate unit. BeforeAttempt linearizes against the current durable retry floor/hold. ObserveAttempt publishes scoped evidence/floor atomically; authorizations later than that publication must refuse, while earlier authorizations remain explicitly accounted in-flight under finite trusted capacity/deadlines. No universal capacity or mandatory single-flight native threshold is selected. A debit whose response is uncertain is not measured zero and is never guessed refundable. The lower guard refuses before the base call on missing/exhausted/expired/foreign/taken-over grant, even if the upper Source budget check is bypassed.

Each observation binds its own canonical attempt reservation: duplicate/foreign/out-of-order reports cannot close/refund another slot, weaken a floor or invent missing observations. A qualified late report may only record its own facts/monotonic floor; it cannot complete a stale refresh or clear a later hold. An uncertain/missing report stays in-flight/held, never a new allowance through eviction, policy-epoch rollover or final aggregate Completion. This is durable visibility at the authority boundary, not instantaneous cross-process knowledge of a header that has not been reported.

The receipt has explicit knowledge states for own HTTP attempts, decoded bytes and retry evidence. Unknown is distinct from measured0. Retry knowledge and the observed maximum floor are independent: later unknown/malformed evidence holds without erasing any prior positive floor. Consumed decoded-prefix bytes may be known while full response coverage is unknown. Retry evidence distinguishes qualified not-before, fully observed no provider constraint, qualified no-response, and unknown/incomplete response metadata. Scope/grant/source provenance is bound and verified; copied waiter totals are not own work. No general map, raw header string, URL, response body or token is part of the receipt.

## Native data completion bound — 3 November 2026

`AttemptReceipt.ObservedAt` is the qualified latest decoded native-data completion
time, captured before `ObserveAttempt` I/O. It is separate from individual
`AttemptObservation.ObservedAt` retry observations, callback completion and quota or
lease clocks. A slow authority report cannot make already-read empty inventory fresh.
Successful production native scans require a nonzero qualified bound; missing or
invalid bounds hold, with no silent fallback. Legacy fake zero remains explicitly
unknown rather than native measurement. Native observation stamps use this bound;
provider `ObservedUpTo` retains its existing provider-stamp semantics.

The coordinator plans coverage at the earlier of the captured actual Scan-return
time and the qualified native bound. Its current query/completion time remains the
actual Scan-return time, so data that aged during authority I/O can be unfulfilled.
Canonical R2 lease, budget, expiry and eligibility use canonical database time; this
source timestamp never selects or resets them. Qualify the actual empty-inventory
callback-age case red then green, preserving unknown and caller-denial semantics.

## Task

1. Add only the bounded typed grant, knowledge and receipt fields to the existing owners and propagate them across all success, partial, nil-error incomplete, failure, cancellation and caller-denial paths. Complete receives the source's actual receipt, never a fabricated Scan-to-HTTP conversion. Source binding and grant/operation admission happen before authority access/send; malformed trusted input holds visibly. The native leaf independently validates actual method, host, provider repository, closed operation and query, including raw/escaped paths, encoded GitLab projects/case, pagination and redirects. Post-decode provenance is too late to prevent a credential-bearing foreign request. Refuse redirects only in the explicitly metered mirror composition, preserving ordinary GitHub behavior and existing admitted TLS/header custody; go-gh ignores a supplied client CheckRedirect, so the supported guard must reside in the actual metered transport. Removing that upper redirect refusal must still leave the independent native operation guard effective.
2. Instrument the existing native leaf once per actual base RoundTrip. Current pinned SDK behavior is independently checked as no hidden retries; actual pagination and repeated entries through the composed native leaf count separately. A future retry path must prove it re-enters the same guards; no new retry mechanism is enabled. A preflight/authority refusal has zero base calls only when that boundary is fully observed. The guard/count lives below any supported opt-in HTTP cache; a cache hit is not an HTTP attempt. Ordinary reads/writes and offline/frozen paths keep their previous transport/counting behavior. Keep source Scan invocations separately measured by the existing observer. Known bytes mean consumed decoded JSON at the existing bound, including its rejection prefix; compressed wire bytes/provider points stay unknown unless separately measured.
3. Enforce the authority-approved finite per-fill HTTP ceiling below pagination/SDK retry. The authority may be stateful test support here; production global windows, caller/admission/source budget and generation are54's canonical SQL responsibility. Uncertain authorization stops the invocation and preserves unknown accounting, never a second allowance. Repeated denied entries cannot reach the base transport. Before every repeated native entry, enforce the invocation's maximum observed floor or unknown-retry hold using the trusted clock, before authority/base access. Do not add a sleep or retry engine. Report each base outcome once through ObserveAttempt after the existing bounded body read and before returning control to the SDK. The reservation remains finitely in-flight until that durable publication, not instantaneous header observation. Known consumed-prefix bytes remain separate from full response/retry knowledge; there is no new byte quota or two-stage callback. On failure/cancellation make a bounded conservative observation or leave the reservation held; uncertainty prevents subsequent native work and cannot be treated as measured0 or automatically retried/refunded.
4. Parse only a bounded documented retry whitelist using stdlib parsers and an explicit trusted observation clock. Honor Retry-After and qualified provider reset evidence by taking the maximum applicable floor across actual attempts. [GitHub documents](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api) an exhausted primary window through remaining0 plus its epoch reset; [GitLab documents](https://docs.gitlab.com/administration/settings/user_and_ip_rate_limits/) Retry-After. Preserve current native status/body classification and do not infer confirmed permission loss. Missing required, malformed, overflowing, contradictory or discarded metadata yields unknown/held, never an early timestamp. A known response whose retry metadata was lost is not a qualified no-response transport failure. No provider probe or fixed one-minute production substitute is introduced.
5. Do not infer safety from the absence of a header unless the admitted native boundary positively observed all relevant response metadata. Known no-observed-constraint is an observation, not a universal provider allowance. Never use a remaining-quota value as this process's own work, multiply a shared fill by waiters or claim exact provider point cost from HTTP count. Provider-specific policy remains with its existing owner.
6. Extend actual source/coordinator tests through the typed boundary and keep51's base FillKey, bounded Requirements and checked Leave Outcome semantics. Current caller/admission denial is not source permission loss or a global failure. No53 fallback may skip54's atomic gate/store completion contract. Per-page fresh native response coverage must be qualified: cached stale replay with0 HTTP cannot become fresh complete source coverage or advance LastComplete/ObservedUpTo. Preserve the qualified pre-authority-I/O data completion bound, including empty inventories, without replacing current query or canonical budget time. Unknown/custom transport coverage holds; bypass only an explicitly supported legacy cache without changing admitted TLS/custody. Never substitute DefaultTransport for arbitrary trusted composition. Conditional304/full-scan reuse remains full37, not an inferred fresh inventory here. Update README, semantic-owner register and actual CI reader coverage for each out-of-module input.
7. Plant actual production faults for counting outside the leaf, ignoring repeated native leaf entries (future SDK retry activation owes real path proof), accepting foreign grant, skipping the lower attempt guard, treating unknown as0, losing response retry metadata and shortening a provider floor. Restore exact bytes; compile/setup failure does not qualify. Show positive in-scope1 and known0 preflight controls beside refused cases.

## Verify

Named tests are prospective. Missing packages, test selectors or required fixtures fail; no skips count. Native requests use synthetic local transports only.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorNativeAttemptReceipt$' ./internal/deskkit > "$out" && grep -F -- '--- PASS: TestMirrorNativeAttemptReceipt ' "$out")` | Named PASS; real admitted SDK path proves pinned current no-hidden-retry configuration and counts pagination at the leaf once each; repeated composed-leaf invocations are labeled boundary proof, not SDK retry evidence; Scan/page/debit/HTTP units differ. Preflight0 is known only with complete coverage; response+error and no-response controls retain actual attempts. Counting outside/before an HTTP cache, ignoring repeated leaf entries or treating unknown as0 fails. Any later retry enablement owes actual retry-path proof. |
| 2 | check +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorNativeAttemptGrant$' ./internal/deskkit > "$out" && grep -F -- '--- PASS: TestMirrorNativeAttemptGrant ' "$out")` | Named PASS; actual lower native guard refuses missing/expired/taken-over/exhausted/uncertain grant before base calls. GrantA cannot spend on admissionB, other repository/kind/source operation. Raw/escaped paths, GitLab encoded projects/case, foreign pagination/redirect targets and method/query changes refuse before authority/base. Bypass upper Source/redirect validation; removing lower guard fails. Valid matching grant permits exactly1. |
| 3 | check +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorNativeRetryReceipt$' ./internal/deskkit > "$out" && grep -F -- '--- PASS: TestMirrorNativeRetryReceipt ' "$out")` | Named PASS; bounded delta/date/reset parsing, maximum retry floor across attempts, malformed/absent-required/overflow/lost metadata and strict response+error/no-response distinctions. A prior response floor/unknown hold refuses repeated native entry before authority/base. Current native48 error/status/cause behavior remains byte-compatible where promised. Removing floor/unknown preservation fails; no raw header/body/token escapes. |
| 4 | check +flow +mutation | `(cd forgeread && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRefreshSourceReceiptFlow$' ./refresh > "$out" && grep -F -- '--- PASS: TestRefreshSourceReceiptFlow ' "$out")` | Named PASS; actual coordinator moves its acquired grant through final re-plan to Source and receipt to Complete; shared waiters do not duplicate own work. Cancellation/current refusal preserves knowledge and51 query-failure semantics. Drop grant/receipt propagation, shorten an observation age through callback completion, or substitute1Scan=1HTTP and the flow fails. |
| 5 | check +dereference +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorNativeReceiptBoundary$' ./internal/deskkit > "$out" && grep -F -- '--- PASS: TestMirrorNativeReceiptBoundary ' "$out")` | Named PASS; actual typed field/owner contract and current operation inventory agree; kernel/root imports stay free of HTTP/SQL/custody; out-of-module source inputs trigger real CI. A raw metadata field, new operation or missing trigger/input fails without weakening the check. |
| 6 | check +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorNativeObservationBoundary$' ./internal/deskkit > "$out" && grep -F -- '--- PASS: TestMirrorNativeObservationBoundary ' "$out")` | Named PASS; BeforeAttempt returns a unique reservation and each native outcome reports only its own typed evidence. Concurrent authority fixtures distinguish pre-publication in-flight reservations from later authorizations; observed floor/unknown holds later work. Duplicate/foreign/out-of-order/uncertain reports cannot close another slot or synthesize completion. Removing report propagation or replay/floor refusal fails; SQL proof remains54. |
| 7 | check +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorNativeFreshnessBoundary$' ./internal/deskkit > "$out" && grep -F -- '--- PASS: TestMirrorNativeFreshnessBoundary ' "$out")` | Named PASS; stale legacy-cache replay/unknown transport coverage with0 HTTP cannot promote source completeness/watermarks; fresh supported admitted native pages remain positive. Actual empty inventory that ages during authority I/O retains its pre-callback completion time and cannot become freshly fulfilled; a missing production native bound holds. Preserve explicit TLS/header custody and ordinary transport behavior. Removing freshness proof or replacing custom composition with DefaultTransport fails. |
| 8 | check | `cd forgeread && GOWORK=off go vet ./... && GOWORK=off go test -race -count=1 ./refresh` | Exit0; bounded synthetic lifetime; no provider contact. |
| 9 | check +dereference | `statusgen --root . --consumers --brief forge-neutral/53 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | Actual pre-merge delivery corroborates fixed-here paths; downstream routing grants no implementation. Post-merge use independent immutable delivery bounds. |

## Evidence

<!-- Independent verifier only. -->

## Review

Gate: human. Review typed knowledge/provenance, exact native attempt boundary, independent lower budget refusal and compatibility.53 fixture proof supplies no SQL quota, persistent integration, provider authorization or activation acceptance.
