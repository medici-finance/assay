---
brief: assay:assay:cell-api:11
title: Qualify the Kubernetes worker-desk mirror caller
why: "The worker desk needs a way to read its repository mirror from Kubernetes while keeping the service private and refusing stale or foreign workload identities."
wave: 4
depends: ["forge-neutral/39"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "A caller with the wrong workload identity could receive private mirror data or start a source fill. The operator must confirm the issuer, live binding, credential custody, and revocation and resource bounds for this profile."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
authored: 2026-11-26 by statusgen newbrief
outcome: none
exec-tier: strong
exec-tier-why: "The same workload identity and current authority must hold at ingress, in the refresh owner, and at final data release."
domain: complicated
sources:
  - https://github.com/medici-finance/assay/issues/7810
  - docs/proposals/non-ci-architecture/21-cell-operator-surface.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - docs/streams/cell-api/brief-03-qualify-mounted-cell-api-authorization-and-privacy.md
  - docs/streams/forge-neutral/brief-56-read-only-forge-serving-behind-qualified-caller-authority.md
  - docs/streams/forge-neutral/brief-57-qualified-local-read-only-caller-authority-and-current-state-producer.md
  - spec/forge-service-v1.md
  - https://kubernetes.io/docs/reference/kubernetes-api/definitions/token-review-v1-authentication/
  - https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/
  - "Freshness checked on 2026-11-26 at public main 1577845621395ce2fcc42d2f8e0f41fbcbe1dab3."
consumers:
  - "tools/desk/internal/forgeservice/auth/ and tools/desk/internal/forgeservice/server/: follow-up cell-api/11 (R7 request authority and mirror handler adaptation)"
  - "forgeread/refresh and workflow/store: follow-up cell-api/11 (consume the existing CallerAuthority and admission contracts)"
  - "tools/desk/cmd/forgeserve and the private serving workload: follow-up forge-neutral/56 (compose the qualified caller profile at launch)"
  - "the broader mounted cell API: follow-up cell-api/03 (retains its admission, dependencies and private-data proof)"
  - "the same-engine Linux producer and its custody evidence: out-of-scope (forge-neutral/57 retains ownership)"
---

# Brief 11 — Qualify the Kubernetes worker-desk mirror caller

## Context

files:
- `tools/desk/internal/forgeservice/auth/`
- `tools/desk/internal/forgeservice/server/mirror_query.go`
- `tools/desk/internal/forgeservice/server/mirror_page.go`
- `tools/desk/internal/forgeservice/server/mirror_kubernetes_test.go` (planned)
- `changelog/kubernetes-mirror-caller.md` (planned)

facts:
- R7 owns mounted caller authority. Amendment 21 specifies machine mTLS, and the local authority ruling on [#5428](https://github.com/medici-finance/assay/issues/5428#issuecomment-6631318014) covers a separate same-engine Linux profile.
- The merged mirror handler takes `*auth.Verifier` in `OpenMirrorQuery`. `Handle`, `HandlePage` and `WarmMetadata` use a signed local grant. `BindRefresh` supplies `refresh.CallerAuthority` to the coordinator and PostgreSQL refresh gate. The handler separately enforces repository admission and rechecks authority after encoding.
- Kubernetes TokenReview establishes an authenticated identity against an expected audience and can return bound Pod and ServiceAccount facts. The response's audience list describes the intersection requested and accepted. An empty match cannot establish the configured application audience. Require the concrete admitted Kubernetes version and authentication configuration to return the stated bound identity.
- The typed query and the restricted runtime role can be exercised with disposable credentials and synthetic source data. The serving listener, publication of its image and qualification of a live workload remain with their existing owners.

real head: Amendment 21 requires “mTLS client certificates for machine callers.” That establishes transport policy but does not establish which certificate and current pod binding jointly identify the worker desk. The configuration and binding decision therefore precede authenticator pickup. The typed contract is the brief's code dependency. The full mounted API and write executor remain on their own delivery paths.

layering: R7 owns the mounted caller proof. The forge-service adapter consumes a request-bound authority through the existing refresh port and retains the independent repository and source checks. Keep the change inside the existing desk module and preserve the query and page codecs.

design-fit:
  owner: R7 mounted caller authority
  contract: none — the existing forge-service contract keeps the query wire and privacy semantics
  retires: []
  weight: "verbs +0; flags +0; refusals and rule-text changes measured at implementation"
  why-add: "Existing local signed grants cannot represent observed R7 transport and pod identity. Reuse the admission, refresh and release owners through a narrow internal interface."

## Read first

Read the [R7 ownership record](README.md#authority-and-admission), [machine authentication requirement](../../proposals/non-ci-architecture/21-cell-operator-surface.md#cell-api-requirements-carried-from-send), [forge service ownership](../../proposals/non-ci-architecture/22-forge-service-and-source-queries.md#authentication-and-authorization), [read-only serving brief](../forge-neutral/brief-56-read-only-forge-serving-behind-qualified-caller-authority.md), `spec/forge-service-v1.md`, and `docs/go-engineering.md`. Use [#7810](https://github.com/medici-finance/assay/issues/7810) for the owner handoff. The source-pinned pickup above covers the repo contracts and current Go interfaces. The deployment trust and issuance facts are pending.

## Human decision

Review one concrete read-only profile for an interactive worker-desk workload, one repository, and PR metadata query, pagination and background warm-up. The proposed fixture uses machine mTLS and a projected pod-bound ServiceAccount token checked by TokenReview. The owner must supply a revision-bound packet before seeking a ruling. Include the trusted client certificate issuer and workload identity, allowed audience and API trust roots, the operator-controlled exact binding and its update owner, revocation and revalidation bounds, credential mounts, and the negative fixture paths.

1. **Admit the bounded profile for offline implementation and qualification (recommended).** Keep machine mTLS and require matching current pod proof. Preserve independent qualification, image publication and operator rollout.
2. **Hold for a named correction.** Specify the missing identity, custody, scope or timing evidence before selecting a profile.

Default if no answer: none — blocks until answered

## Ground rules

- Keep the implementation and tests in an owned branch. Worker activity uses disposable fixtures, with no cluster or production access.
- The selected fixture supplies synthetic identities and data. Mark offline fixtures UNQUALIFIED for operational use.
- Keep proof outside of the query payload and cursor. Preserve the existing local caller and grant path.
- Scope excludes work and review tools, human actions, write executors, general cell API ingress, and deployment.

## Task

1. Inspect the actual handler and refresh boundaries at pickup, then prepare the profile packet and record the operator ruling. Specify the accepted Kubernetes identity fields, how the approved issuer binds each client certificate to exactly one workload incarnation, and the named current-binding reader. Include the expected API server or authenticator behavior and the admitted maximum sizes, deadlines, rate limits and revocation window. Reuse the current read and warm-up budgets. Account for proof calls separately from forge traffic.
2. Use verified machine mTLS on the private listener. Authenticate the projected token through a separate API credential at the configured CA-verified TokenReview endpoint. Accept a successful review for the single expected audience with the expected ServiceAccount username and UID and one matching Pod name and UID. Require the current named binding to match the same certificate principal, account and Pod incarnation, and the approved cell, role, repository, operation, source and epochs. Reject absent or ambiguous evidence. Keep bearer material and private keys out of logs, storage and error messages.
3. Adapt the handler narrowly so the trusted transport supplies a non-serializable request authority. The coordinator and SQL gate use the existing `CallerAuthority` seam. Check the same current scope at query entry, before each native attempt, before protected refresh completion and after response encoding. Use bounded request and fill leases. Limit retained in-memory proof to the authorized lifetime. Revalidate with current authority before renewal. A cursor alone supplies no authority. Denials omit rows, counts, cursors, freshness and retry metadata.
4. Keep the client credential, API verifier credential, restricted database login and admitted source credential separate. The verifier can submit TokenReview and read only the approved current binding. It cannot mint credentials, impersonate callers, or write bindings. Demonstrate real transport validation and SQL admission using local TLS, a synthetic TokenReview peer, disposable PostgreSQL and a synthetic forge source. Any control-plane-specific proof is supplied through the owning qualification workflow before the live profile is used.
5. Record the implemented interface, refusal behavior and measured proof overhead. Deliver the negative controls alongside the implementation. Hand the private serving image and workload inputs back to forge-neutral/56 with their remaining activation gates.

## Interface contract

The profile is selected by trusted startup composition. The network request uses the existing mirror query and page codecs. Its authority fixes one caller and represented actor, one repository and change-metadata kind, and one admitted source. No request value can widen them. A fixture peer may emulate a TokenReview response, with verified TLS and the same parsing and binding code used by the eventual connector.

## Verify

Each named test below is a planned deliverable. The integration test must use the existing explicitly selected disposable DSN, print the exact named pass, and report a skip as unavailable evidence. The guard tests run the restored production predicates and record a failure for each unsafe planted change.

| # | Kind | Command | Expect |
|---|------|---------|--------|
| 1 | check +dereference +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestKubernetesMirrorCaller$' ./internal/forgeservice/auth > "$o" 2>&1 && grep -F -- '--- PASS: TestKubernetesMirrorCaller ' "$o")` | Named PASS. A matched certificate, audience, account and pod succeeds. Foreign audience, expired proof, wrong certificate or incarnation, ambiguous review, changed binding, oversized proof and proof outage refuse. Removing either trust check turns its corresponding case red. |
| 2 | check +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestKubernetesMirrorRelease$' ./internal/forgeservice/server > "$o" 2>&1 && grep -F -- '--- PASS: TestKubernetesMirrorRelease ' "$o")` | Named PASS. A changed binding before refresh completion stops the protected commit. A change after encoding strips all data. Revocation between attempts stops further source work. Each bypass of a current-authority check has a failing control. |
| 3 | check +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -race -count=1 -v -run '^TestPGKubernetesMirrorCaller$' ./internal/forgeservice/server > "$o" 2>&1 && grep -F -- '--- PASS: TestPGKubernetesMirrorCaller ' "$o")` | Named PASS with disposable PostgreSQL. One scoped fill commits one snapshot. Warm first page and continuation make no further source calls. A foreign scope and a deliberately bypassed ingress still meet lower admission before data access. |
| 4 | check +neighbour | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorQueryRejectsClaimsBeforeStore$' ./internal/forgeservice/server > "$o" 2>&1 && grep -F -- '--- PASS: TestMirrorQueryRejectsClaimsBeforeStore ' "$o")` | Named PASS on the existing local-grant path. Wrong caller and scope leave the response stripped and store untouched. |

## Threat model and detection

| Failure mode | Detection |
| --- | --- |
| A valid certificate is paired with a copied or foreign pod token | Row 1 requires both proofs to match the same configured workload incarnation. |
| Stale or unavailable policy lets a revoked caller fill or read the mirror | Row 2 changes current state during refresh and after encoding. |
| A foreign repository or a replayed cursor escapes the ingress checks | Rows 2 and 3 assert stripped denials and independent lower admission. |
| A new ingress path alters the local signed-grant boundary | Row 4 exercises the existing caller. |
| The verifier gains write or impersonation access, leaks proof in its audit output, or the fixture is used as rollout authority | No row. Review-only inspection of the proposed credential and API audit policy, and independent control-plane qualification before activation. |

## Evidence

<!-- Independent verifier records exact test result, runner, date and artifact here. -->

## Review

Gate: human. Record the named implementing owner and revision-bound profile ruling before implementation. Independent verification and operator acceptance govern runtime qualification.
