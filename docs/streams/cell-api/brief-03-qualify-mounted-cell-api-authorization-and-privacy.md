---
brief: assay:assay:cell-api:03
title: Qualify mounted cell API authorization and privacy
why: "Network exposure and cached projections can leak private information even when a route table looks protected. The mounted server must enforce caller scope and current admission independently of clients and lower-level stores."
wave: 13
depends: ["cell-api/02", "forge-neutral/40", "graph-execution/18"]
unblocks: ["cell-api/04", "cell-api/05"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Confirm the concrete issuer and caller mapping, entitlement and revocation bounds, and effective private-data isolation from the demonstrated design. Do not infer approval of a later draft authentication mechanism from an older architecture merge."
issues: []
schema: brief-v2
version: 3
id: 16d1b169-2541-4b85-90cc-d0d42945e86c
authored: 2026-10-20 by cell-api authoring
decision-trigger: spec
exec-tier: strong
exec-tier-why: Cross-component compatibility, privacy and authority boundaries require design judgment and independent negative controls.
domain: complicated
outcome: none
sources:
  - docs/streams/cell-api/README.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - docs/proposals/non-ci-architecture/16-delivery-sequence.md
  - docs/proposals/non-ci-architecture/20-amendment-manifest.md
  - docs/proposals/non-ci-architecture/21-cell-operator-surface.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - docs/proposals/non-ci-architecture/23-runtime-contract-alignment.md
  - docs/proposals/non-ci-architecture/28-interactive-kubernetes-desks.md
  - https://github.com/medici-finance/assay/pull/2890
  - https://github.com/medici-finance/assay/issues/3669
  - freshness-checked 2026-10-20 @ 15b60414f3ec134942a0ec3162b550cd95cbbc8e
consumers:
  - "tools/desk/internal/cellapi/: follow-up cell-api/03"
  - "tools/desk/cmd/deskd/: follow-up cell-api/03"
  - "tools/desk/testdata/cellapi/: follow-up cell-api/03"
  - "docs/cell-api-security.md: follow-up cell-api/03"
  - "changelog/cell-api-ingress.md: follow-up cell-api/03"
  - "legacy source consumers: out-of-scope (private source owner retains their exact crosswalk; this brief records public dispositions and does not edit them)"
---

# Brief 03 — Qualify mounted cell API authorization and privacy

## Context

files: `tools/desk/internal/cellapi/` (planned), `tools/desk/cmd/deskd/` (planned), `tools/desk/testdata/cellapi/` (planned), `docs/cell-api-security.md` (planned), `changelog/cell-api-ingress.md` (planned)

facts:
- pin: public main 15b60414f, inspected 2026-10-20; public mounted service is planned.
- gates: graph-execution/18 supplies complete R5 proof; forge-neutral/40 supplies the R3/R4 serving boundary; missing evidence blocks admission.
- ratification: PR #2890 merged 2026-10-14, retaining read/plan-only API and deferred apply.
- auth: amendment 21 names mTLS/signed-token requirements; later forge-service issuer mechanisms retain separate approval scope.

layering: Authenticated adapter composes the existing admission/store boundaries; no second issuer or broker. Task 1–4 and Verify 1–3 exercise the production mux and bypass the upper layer.

single-point-of-failure: the mounted caller-to-subject authorization mapping can leak a result before projection filtering. Verify 1–2 test the actual mux and private/count/SSE results; Verify 3 bypasses ingress and meets independent store admission and R4 executor role/epoch checks. A route allowlist alone does not provide those lower checks.

design-fit:
  owner: R7 mounted ingress; R3/R4 and forge-neutral/40 own capability and executor custody
  contract: none — R7's transport responsibility is registered by cell-api/01; existing semantic owners and contracts remain controlling
  retires: []
  weight: verbs +0; flags +0; refusals +0; rule-text +0 (no new operator vocabulary proposed; record any actual implementation delta for review)
  why-add: Reuse the existing owners and one client seam; remove duplicate service composition only after the last-consumer proof. Package/process additions are bounded by the declared files, not permission to add another controller.

All planned paths and named tests below are implementation deliverables. The source pin
proves the starting point, not that these deliverables exist or were tested. The
[stream refactor map](README.md#bounded-refactor-map) defines the public transfer boundary.

## Read first

Read the [R7 ownership and admission record](README.md#authority-and-admission),
[cell operator contract](../../proposals/non-ci-architecture/21-cell-operator-surface.md),
[forge-service boundary](../../proposals/non-ci-architecture/22-forge-service-and-source-queries.md),
[interactive Kubernetes direction](../../proposals/non-ci-architecture/28-interactive-kubernetes-desks.md),
`spec/cell-op-v1.md`, `docs/contracts.md` and `docs/go-engineering.md`.
Resolve every declared dependency's current contract and independent Evidence before
pickup. The package mapping is a reviewable proposal; a conflicting contract is
NEEDS_CONTEXT, never permission to select new custody or duplicate its owner.

## Human decision

Confirm the concrete issuer and caller mapping, entitlement and revocation bounds, and effective private-data isolation from the demonstrated design. Do not infer approval of a later draft authentication mechanism from an older architecture merge.

Options:
1. **Accept the demonstrated bounded design** — accept the evidenced compatibility and containment for its declared scope; provisioning, deployment and source retirement retain their separate operator acts.
2. **Hold for correction** — name the missing proof or incompatible behavior; retain the current supported path until corrected.

Default if no answer: none — blocks until answered

The decision becomes concrete from the implementation design and evidence. No answer
keeps acceptance pending; it grants no time-based default or activation authority.

## Ground rules

- Work in an owned branch; no merge, ready flip, release, installation or live infrastructure contact.
- Public source and fixtures must be self-contained and synthetic. Never copy private locators, configuration, credentials, incidents or history into Assay.
- Preserve the single R2 PostgreSQL runtime, R3 isolation, R4 role authority and R6 forge mirror. A service, terminal, restored file or cached result grants no authority.
- Keep this stream parked until admitted. Stop at implemented; independent verification, human acceptance and operational cutover remain distinct.

## Task

1. Compose the actual served mux using 02 and the existing forge-neutral/40 capability/admission interfaces. Bind caller class server-side, cell/audience, provider/repository scope, access epoch, expiry and revocation. Resolve any inconsistency between the ratified mTLS/signed-token requirements and a later issuer proposal before implementing that branch; do not add a new login scheme by convenience.

2. Filter current authorized inputs before joins, aggregate counts, errors, pagination, ETags, events and cached responses. Recheck access on every read and resumed SSE stream; revoke active streams within the approved bound. A foreign cell, stale grant, swapped admission or unavailable authorization must not reveal object existence or totals. Test actual mounted handlers with the UI bypassed.

3. Expose only read/status/plan operations in this slice. Keep generic apply and human named-action execution absent until their separately qualified contract. Keep human keys and persistent human credentials out of the service. Agent discovery and direct invocation cannot expose the legacy human-answer or adopt tools; no universal token, role switch or raw proxy. Reuse per-role executors and their existing refusal/audit behavior.

4. Exercise store admission with ingress deliberately bypassed, and existing executor authority with client checks bypassed. Prove operator terminal/log outputs are unavailable to external/agent callers and scrubbed before serving. Quarantine free-text requests unless the existing contained-reader and answer-only effect boundary are qualified; do not add a second comms ingress. Preserve rate limits and bounded request/reply audit digests without credentials.

5. Document the explicit exposure profile, chosen revocation contract and remaining gates; no real credential provisioning or listener deployment. Keep each negative control re-runnable against the real production composition.

## Interface and acceptance contract

Consume cell-api/01's versioned HTTP/JSON and SSE seam and the existing cell-op-v1
result/plan meanings. Authentication, current authorization, provider custody and
acceptance are separate. Required coverage/freshness, exact subject and unknown outcomes
survive every adapter. No agent-facing legacy human-answer or adopt tools; no generic role,
URL, SQL-write, token or apply escape hatch. Human named actions remain closed and
separately qualified. Initial service exposure is read/plan-only; approval-based apply
stays absent until amendment 21's five R7/R3 properties are independently qualified.

Update the declared documentation and generated schema/client/profile examples affected
by the Task. Preserve prior Evidence and source history; unresolved consumer or contract
coverage is a hold. Do not invent deployment performance/recovery thresholds: reuse the
inherited workload and require the operator to select missing targets before exposure.

## Verify

These exact named tests must be delivered and run, with a recorded failing control
before the restored pass. Each uses real production composition and an independently
specified expected result; zero selected tests or a skipped required fixture is failure.
Only explicit disposable fixtures may open local listeners/databases. No production or
ambient kubeconfig is permitted. Real-profile qualification remains a separate operator
act; offline passes do not establish it.

For row 4, the independent verifier obtains the actual implementation delivery from its
merged PR and records its full immutable SHA in `ASSAY_DELIVERY_SHA` and its actual
first-parent SHA in `ASSAY_DELIVERY_BASE_SHA`. Neither defaults to HEAD, origin/main or
this authoring PR. Both commits and the fetched mainline must exist locally; the command
requires the delivery to have landed on that mainline before checking its immutable
snapshot. Missing, unknown, not-yet-landed or wrong-parent inputs fail closed. Record
exact corroborated/disproved/unchecked counts; unchecked required consumers remain held
and omitted consumers remain a review obligation. These are future delivery inputs.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +neighbour | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellMountedAuth$' ./cmd/deskd/... > "$o" && grep -F -- '--- PASS: TestCellMountedAuth ' "$o")` | exit 0; named PASS. Real mux rejects unauthenticated, forged operator, wrong audience/cell, expired/revoked and direct apply calls; inserting an unguarded sibling mount fails. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellPrivateViews$' ./internal/cellapi/... > "$o" && grep -F -- '--- PASS: TestCellPrivateViews ' "$o")` | exit 0; named PASS. Two-cell/private fixtures filter counts, joins, errors, ETags and cached/SSE events before response; mid-stream revocation and false zero coverage fail independent expectations. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellLowerBounds$' ./internal/cellapi/... > "$o" && grep -F -- '--- PASS: TestCellLowerBounds ' "$o")` | exit 0; named PASS. Bypassed ingress still meets store admission and separate executor role/epoch checks. Agent direct human-act calls, unsanitized terminal output and unavailable reader are refused. |
| 4 | check:ci +dereference | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && git merge-base --is-ancestor "$ASSAY_DELIVERY_SHA" refs/remotes/origin/main && statusgen --root . --consumers --brief cell-api/03 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 only for the named landed implementation delivery and its first parent; record exact corroborated/disproved/unchecked counts, 0 disproved; unchecked required consumers stay held and omitted consumers require review. |
| 5 | check | `statusgen --root . --lint` | exit 0; no PROBLEM; no generated lifecycle cell or historical Evidence hand-edited. |

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| Guarded route table masks an unguarded actual mount | row 1 |
| Counts, cursors or cached results disclose a foreign/private source | row 2 |
| The caller bypasses the upper gate or gains human/publisher powers | row 3 |
| A fixture is mistaken for deployment authority or a private inventory is called complete without its owner | No row; review-only: inspect decision provenance, source-owner receipt and explicit operational limits before acceptance. |

## Evidence

<!-- Independent verifier appends one row per Verify item: exact subject/source revision,
command, exit, named result, failing control and restored pass, environment, date and
runner. Empty at authoring; no implementation or deployment result is asserted. -->

## Review

Gate: human. Review the actual mounted/effect boundary and retained ownership, the
compatibility inventory and the independent negative controls. Test success grants
neither deployment nor credential provisioning, source closure or a human decision.
