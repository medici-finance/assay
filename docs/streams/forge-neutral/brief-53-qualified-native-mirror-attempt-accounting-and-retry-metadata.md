---
brief: assay:assay:forge-neutral:53
title: Qualified native mirror attempt accounting and retry metadata
why: "A shared refresh must know the HTTP work it actually caused and when a provider permits another attempt. Typed native receipts and a pre-request budget guard prevent unknown usage or lost retry metadata from becoming a fresh allowance."
wave: 5
depends: ["forge-neutral/48", "forge-neutral/50", "forge-neutral/51"]
unblocks: ["forge-neutral/54"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "The new receipt crosses the admitted native transport and refresh boundary. Accept its demonstrated scope binding, safe metadata and bounded attempt semantics; it grants no credential, service exposure or shared-budget activation."
issues: []
schema: brief-v2
version: 1
authored: 2026-11-03 by statusgen newbrief
outcome: none
design: DR-forge-library
decision-trigger: spec
split-from: forge-neutral/37
exec-tier: strong
domain: complicated
exec-tier-why: "SDK retries, native response errors and independent budget/source checks must retain one truthful attempt boundary across adapters."
sources:
- docs/streams/forge-neutral/brief-37-scoped-forge-mirror-with-persisted-freshness-coverage-and-polling-repair.md
- docs/streams/forge-neutral/brief-48-bound-native-mirror-response-allocation-before-decoding.md
- docs/streams/forge-neutral/brief-50-join-admitted-incremental-reads-to-mirror-observations.md
- docs/streams/forge-neutral/brief-51-store-injected-mirror-read-through-coordinator-and-shared-refresh-gate-port.md
- docs/library-first.md
- docs/go-engineering.md
- docs/performance-attribution-spec.md
- "freshness-checked 2026-11-03 against worker-App-fetched main214b4c8308b3cc57beed91feccf45e6a03f2eecd; mapper2677323c9bdf81aad1a1e3b4ad4407e0703ee25c and coordinator18009487a85e7c1cde48778cdb2e3e265c0e0e54 are published; the later mixed-demand/Leave repair must be consumed before implementation"
- https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api
- https://docs.gitlab.com/administration/settings/user_and_ip_rate_limits/
consumers:
- "tools/desk/internal/deskkit native leaf and admitted mapper: follow-up forge-neutral/53 (implementation owner)"
- "forgeread/mirror ScanPlan/ScanResult and forgeread/refresh Ticket/Completion/coordinator: follow-up forge-neutral/53 (implementation owner) (narrow typed grant/receipt only)"
- "workflow/store canonical SQL attempt authority and joined proof: follow-up forge-neutral/54"
- "forgeread root, statusgen offline/frozen paths and current operation inventories: out-of-scope (unchanged)"
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

single-point-of-failure: entering the admitted native base RoundTripper with a valid attempt grant — the Source constructor/closed-operation guard independently binds scope, and the canonical attempt authority independently checks the current grant before the leaf can send.

facts:
- Actual50 ScanResult has observations/completeness/failure only. UpdateCoverage counts fetched pages and decoded JSON bytes, not SDK retries or every failed native attempt. A page, Scan invocation, reservation debit and HTTP attempt are different units.
- Actual51 Gate.Complete already owns the typed ApplyCompletion seam. Ticket/Completion lack native budget/receipt fields. Its mixed-demand and final-Leave repair is in progress at this read: consume its published Requirements/Outcome API before coding, never copy its fake gate or repair it here.
- Actual48 native allocation guard carries status only through rejected errors. It deliberately retains no raw header/body/endpoint/credential. Existing status/cause/classification behavior is a compatibility obligation.
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

Add typed grant/receipt data to the existing ScanPlan/ScanResult and refresh Ticket/Completion flow. Define one opaque grant identity bound by the authority to source population, admission, canonical provider/host/repository/kind, generation, policy epoch, finite HTTP ceiling and expiry. Ticket carries the trusted grant; the coordinator attaches it after its final re-plan; callers cannot select it through query text. The Source constructor binds the narrow AttemptAuthority. Its BeforeAttempt operation validates/debits that grant before each native base RoundTrip, including SDK retries. It is not a generic callback, HTTP interface, SQL port, credential or action capability.53 defines this seam and qualifies its use against a stateful fixture;54 implements its only production authority in the existing R2 store.

An HTTP attempt means entering the existing native base RoundTripper. DNS/transport failure, no response and response+error may leave wire completion unknown; none means no attempt. An authority debit is a separate unit. A debit whose response is uncertain is not measured zero and is never guessed refundable. The lower guard refuses before the base call on missing/exhausted/expired/foreign/taken-over grant, even if the upper Source budget check is bypassed.

The receipt has explicit knowledge states for own HTTP attempts, decoded bytes and retry evidence. Unknown is distinct from measured0. Retry evidence distinguishes qualified not-before, fully observed no provider constraint, qualified no-response, and unknown/incomplete response metadata. Scope/grant/source provenance is bound and verified; copied waiter totals are not own work. No general map, raw header string, URL, response body or token is part of the receipt.

## Task

1. Add only the bounded typed grant, knowledge and receipt fields to the existing owners and propagate them across all success, partial, nil-error incomplete, failure, cancellation and caller-denial paths. Complete receives the source's actual receipt, never a fabricated Scan-to-HTTP conversion. Source binding and grant/operation admission happen before authority access/send; malformed trusted input holds visibly.
2. Instrument the existing native leaf once per actual base RoundTrip. SDK retries count separately; a preflight/authority refusal has zero base calls only when that boundary is fully observed. Ordinary reads/writes and offline/frozen paths keep their previous transport/counting behavior. Keep source Scan invocations separately measured by the existing observer. Known bytes mean consumed decoded JSON at the existing bound, including its rejection prefix; compressed wire bytes/provider points stay unknown unless separately measured.
3. Enforce the authority-approved finite per-fill HTTP ceiling below pagination/SDK retry. The authority may be stateful test support here; production global windows, caller/admission/source budget and generation are54's canonical SQL responsibility. Uncertain authorization stops the invocation and preserves unknown accounting, never a second allowance. Repeated denied SDK calls cannot reach the base transport.
4. Parse only a bounded documented retry whitelist using stdlib parsers and an explicit trusted observation clock. Honor Retry-After and qualified provider reset evidence by taking the maximum applicable floor across actual attempts. [GitHub documents](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api) an exhausted primary window through remaining0 plus its epoch reset; [GitLab documents](https://docs.gitlab.com/administration/settings/user_and_ip_rate_limits/) Retry-After. Preserve current native status/body classification and do not infer confirmed permission loss. Missing required, malformed, overflowing, contradictory or discarded metadata yields unknown/held, never an early timestamp. A known response whose retry metadata was lost is not a qualified no-response transport failure. No provider probe or fixed one-minute production substitute is introduced.
5. Do not infer safety from the absence of a header unless the admitted native boundary positively observed all relevant response metadata. Known no-observed-constraint is an observation, not a universal provider allowance. Never use a remaining-quota value as this process's own work, multiply a shared fill by waiters or claim exact provider point cost from HTTP count. Provider-specific policy remains with its existing owner.
6. Extend actual source/coordinator tests through the typed boundary and keep51's base FillKey, bounded Requirements and checked Leave Outcome semantics. Current caller/admission denial is not source permission loss or a global failure. No53 fallback may skip54's atomic gate/store completion contract. Update README, semantic-owner register and actual CI reader coverage for each out-of-module input.
7. Plant actual production faults for counting outside the leaf, ignoring SDK retries, accepting foreign grant, skipping the lower attempt guard, treating unknown as0, losing response retry metadata and shortening a provider floor. Restore exact bytes; compile/setup failure does not qualify. Show positive in-scope1 and known0 preflight controls beside refused cases.

## Verify

Named tests are prospective. Missing packages, test selectors or required fixtures fail; no skips count. Native requests use synthetic local transports only.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorNativeAttemptReceipt$' ./internal/deskkit > "$out" && grep -F -- '--- PASS: TestMirrorNativeAttemptReceipt ' "$out")` | Named PASS; real admitted SDK path counts paginated requests and SDK retries at the leaf once each; Scan/page/debit/HTTP units differ. Preflight0 is known only with complete coverage; response+error and no-response controls retain actual attempts. Counting outside the leaf or treating unknown as0 fails. |
| 2 | check +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorNativeAttemptGrant$' ./internal/deskkit > "$out" && grep -F -- '--- PASS: TestMirrorNativeAttemptGrant ' "$out")` | Named PASS; actual lower native guard refuses missing/expired/taken-over/exhausted/uncertain grant before base calls. GrantA cannot spend on admissionB, other repository/kind/source operation. Bypass upper Source validation; removing lower guard fails. Valid matching grant permits exactly1. |
| 3 | check +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorNativeRetryReceipt$' ./internal/deskkit > "$out" && grep -F -- '--- PASS: TestMirrorNativeRetryReceipt ' "$out")` | Named PASS; bounded delta/date/reset parsing, maximum retry floor across attempts, malformed/absent-required/overflow/lost metadata and strict response+error/no-response distinctions. Current native48 error/status/cause behavior remains byte-compatible where promised. Removing floor/unknown preservation fails; no raw header/body/token escapes. |
| 4 | check +flow +mutation | `(cd forgeread && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRefreshSourceReceiptFlow$' ./refresh > "$out" && grep -F -- '--- PASS: TestRefreshSourceReceiptFlow ' "$out")` | Named PASS; actual coordinator moves its acquired grant through final re-plan to Source and receipt to Complete; shared waiters do not duplicate own work. Cancellation/current refusal preserves knowledge and51 query-failure semantics. Drop grant/receipt propagation or substitute1Scan=1HTTP and the flow fails. |
| 5 | check +dereference +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorNativeReceiptBoundary$' ./internal/deskkit > "$out" && grep -F -- '--- PASS: TestMirrorNativeReceiptBoundary ' "$out")` | Named PASS; actual typed field/owner contract and current operation inventory agree; kernel/root imports stay free of HTTP/SQL/custody; out-of-module source inputs trigger real CI. A raw metadata field, new operation or missing trigger/input fails without weakening the check. |
| 6 | check | `cd forgeread && GOWORK=off go vet ./... && GOWORK=off go test -race -count=1 ./refresh` | Exit0; bounded synthetic lifetime; no provider contact. |
| 7 | check +dereference | `statusgen --root . --consumers --brief forge-neutral/53 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | Actual pre-merge delivery corroborates fixed-here paths; downstream routing grants no implementation. Post-merge use independent immutable delivery bounds. |

## Evidence

<!-- Independent verifier only. -->

## Review

Gate: human. Review typed knowledge/provenance, exact native attempt boundary, independent lower budget refusal and compatibility.53 fixture proof supplies no SQL quota, persistent integration, provider authorization or activation acceptance.
