---
brief: assay:assay:forge-neutral:58
title: Refresh the forge cache from GitHub and GitLab CI hints
why: "A newly opened pull or merge request can leave the shared cache looking fresh until its next scheduled refresh. CI can signal the cache to refresh sooner so the review desk and other readers can share one current snapshot without a public webhook or per-desk polling."
wave: 9
depends: ["forge-neutral/56", "forge-neutral/38"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Even a private repository ID and change number reveal activity. The operator must approve publisher identity and scope, broker custody and retention, and the first enabled project on each forge after the working cache is demonstrated."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
authored: 2026-11-26 by statusgen newbrief
outcome: none
exec-tier: strong
exec-tier-why: "A fresh-by-age snapshot can hide a newly opened change, and an older fill can clear a newer hint. Both failures pass a single-delivery test."
domain: complicated
sources:
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - docs/streams/forge-neutral/brief-38-forge-event-adapter-feedback-observations-and-durable-reevaluation-demand.md
  - docs/streams/forge-neutral/brief-54-canonical-postgresql-refresh-gate-and-admitted-read-through-integration.md
  - docs/streams/forge-neutral/brief-56-read-only-forge-serving-behind-qualified-caller-authority.md
  - "medici-finance/assay#7820 — background metadata warm-up and shared PostgreSQL reuse"
  - "freshness-checked 2026-11-26 @ 94ab49cdf5 (origin/main)"
consumers:
  - "forgeread/events/: follow-up forge-neutral/58 (CI hint adapter through the shared intake)"
  - "forgeread/refresh/: follow-up forge-neutral/58 (scoped freshness demand through the existing coordinator)"
  - "workflow/store/: follow-up forge-neutral/58 (durable demand and wake after commit through the canonical PostgreSQL path)"
  - "tools/desk/internal/forgeservice/server/: follow-up forge-neutral/58 (explicit subscriber composition)"
  - "plugins/assay/skills/pr-review-desk: out-of-scope (existing bounded tick, trust checks and claims consume the wake without changes to review authority)"
---

# Brief 58 — Refresh the forge cache from GitHub and GitLab CI hints

## Context

The [read-only serving brief](brief-56-read-only-forge-serving-behind-qualified-caller-authority.md) requires an independently qualified caller and restricted refresh profile. Its served-flow check exercises the actual client, handler, coordinator and PostgreSQL store. A local warm-up fixture or passing CI only qualifies that fixture. Start this follow-on after the operator records that the read-only pilot serves an admitted cold or stale query and a subsequent query reuses the committed snapshot. The [event-hint brief](brief-38-forge-event-adapter-feedback-observations-and-durable-reevaluation-demand.md) supplies the shared hint intake and refresh contract. Its library seam is required. A mounted webhook route is outside this work.

The [warm-up proposal](https://github.com/medici-finance/assay/pull/7820) shares the interactive freshness check. An observation can meet that age check even when a CI hint names a newer change. The refresh demand therefore needs a scope-bound generation or equivalent observation floor inside the existing gate. The [read-through contract](../../proposals/non-ci-architecture/22-forge-service-and-source-queries.md#read-through-and-failure-cache--3-november-2026) supplies the fallback and the budget for equivalent demand.

files:
- `forgeread/events/` (planned)
- `forgeread/refresh/`
- `workflow/store/`
- `tools/desk/internal/forgeservice/server/`
- `tools/desk/internal/deskkit/`
- `.github/workflows/assay-workflow.yml`
- `docs/forge-service.md`
- `changelog/forge-cache-ci-hints.md` (planned)

## Human decision

After pilot evidence and the offline delivery checks exist, select the first projects and approve the broker, publisher identity bindings, retention and request budget. Verify the actual GitHub Actions event policy and the GitLab CI configuration and identity claims for those projects. The proposed first scope is one trusted project on each forge, with separate publish and consume roles and bounded retention of identifiers only. The operator may approve that limited rollout or hold it for a named gap. Hold while the scope, custody or an authorization check is unresolved. Authoring this brief authorizes no operational change.

## Ground rules

- Exercise both publishers and the subscriber with fixtures and explicitly disposable infrastructure. Operational CI changes, runner assignment, broker provisioning, IAM and subscriber activation are operator acts. The delivered integration is disabled by default.
- Use outbound connections from CI and the cache host. The hint carries qualified forge instance, repository or project ID, change number, stable delivery identity and event kind. Keep credentials, descriptions, diffs and comments out of the envelope. A publisher may request a bounded re-read only. The cache keeps its existing read-only source identity and independent admission checks.
- Retain read-through and inventory repair. A skipped or failed CI run can leave no hint to replay. Preserve permission and partial-coverage outcomes and existing provider limits, backoff and request accounting.

## Task

1. Provide GitHub and GitLab examples for an operator-approved durable broker with an outbound streaming subscriber. Use a trusted GitHub base-branch workflow for `pull_request_target` `opened` and `synchronize`, with no checkout and an applicable Actions policy. Use `merge_request_event` in GitLab for opens and new commits on approved same-project MRs. Constrain the workload identity to the approved issuer, project and trusted pipeline configuration. Fence fork-controlled config and source-project claims out of publish access, including when a GitLab fork pipeline runs in the parent. Record deliveries the triggers cannot cover, including GitLab approvals, comments and closure without a new pipeline. [GitHub event policy](https://docs.github.com/en/actions/reference/security/securely-using-pull_request_target) · [GitLab MR pipeline rules](https://docs.gitlab.com/ci/pipelines/merge_request_pipelines/) · [GitLab ID token claims](https://docs.gitlab.com/ci/secrets/id_token_authentication/)
2. Deliver both publishers to the existing hint intake. Validate scope independently of message fields, coalesce by the admitted cache scope, and acknowledge the broker after durable acceptance. An accepted hint must make a snapshot observed before it insufficient for that demand even when its age is within policy. Refresh using the existing coordinator, Gate and source reader. Fence completion against a newer accepted generation, and retain owed demand across disconnect, redelivery or temporary provider hold. The latest source observation determines the state.
3. Signal the review desk through its existing bounded pass only after the relevant metadata commits. A missing or failed read leaves explicit status and pending or repairable work. Warm reads should share the committed snapshot, and duplicates must not create an extra fill or review dispatch after satisfaction. Keep unsupported events in the existing read-through and inventory-repair path. Record time from accepted hint to committed observation and provider requests per coalesced scope against the working pilot baseline. Capture replay, repair and publisher coverage so the operator can choose whether to enable it.
4. Update `docs/forge-service.md` with inactive publisher examples, measured results and the subscriber off switch. Wire both joined tests into the existing PostgreSQL CI checks, including every new input path in its triggers.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestCIHintPublishers$' ./internal/forgeservice/server > "$o" && grep -F -- '--- PASS: TestCIHintPublishers ' "$o")` | Both sample publishers deliver bounded metadata for supported events. Foreign project, wrong audience or issuer, and fork-controlled CI config fail before any durable demand. Removing the scope binding fails the witness. |
| 2 | check:ci +flow +mutation +neighbour | `(cd tools/desk && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorCIHints$' ./internal/deskkit > "$o" && grep -F -- '--- PASS: TestMirrorCIHints ' "$o")` | Both forges traverse the admitted source, Gate and disposable PostgreSQL store. A fresh-by-age stale snapshot refreshes, a newer hint survives an older in-flight fill, and a duplicate or restarted subscriber produces one effective fill per satisfied generation. The desk wakes after commit and a reader reuses the result. Bypassing the hint freshness check or acknowledging before durable intake fails the witness. |
| 3 | check:ci +flow +neighbour | `(cd tools/desk && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorCIHintRepair$' ./internal/deskkit > "$o" && grep -F -- '--- PASS: TestMirrorCIHintRepair ' "$o")` | Missing permission, partial reads and retry holds preserve the prior watermark and wake no desk for unconfirmed state. A skipped pipeline or missed delivery converges through normal read-through or inventory repair. A warm reader after convergence makes zero further source calls. |
| 4 | check +dereference | `test -n "$ASSAY_DELIVERY_COMMIT" && test -n "$ASSAY_DELIVERY_BASE" && statusgen --root . --consumers --brief forge-neutral/58 --delivery "$ASSAY_DELIVERY_COMMIT" --base "$ASSAY_DELIVERY_BASE"` | Set both variables from independently recorded immutable delivery bounds. Confirm the declared consumers against the actual diff. Unchecked and disproved routes require separate review evidence before acceptance. |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review

Gate: human. Review the demonstrated publisher scope, delivery and repair evidence before the operator enables a subscriber or publisher.
