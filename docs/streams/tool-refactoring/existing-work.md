# Existing work and overlap disposition

Observed on public main `680102a0877755feab9ef85dc4e1d3f52a137a54`, 2026-11-12. Board state is a source observation,
not proof of a release, consumer pin, live activation or absence of an open defect.
The source/digest inventory is [source-observations.json](source-observations.json).
All listed scopes are **retained**. New briefs add missing typed seams or caller
migration/qualification; they do not claim to replace these owners.

| Existing brief | Observed state / version | Disposition and relation |
|---|---|---|
| [statusgen/15](../statusgen/brief-15-extract-canonical-eligibility-and-coverage-evaluation-api.md) | verified; version 1 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [statusgen/19](../statusgen/brief-19-context-handoff-contracts.md) | verified; version 1 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [statusgen/20](../statusgen/brief-20-offline-context-resolver.md) | verified; version 4 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [statusgen/21](../statusgen/brief-21-author-skill-handoff.md) | verified; version 3 | Retain; reused or preserved by /02. |
| [statusgen/29](../statusgen/brief-29-run-the-existing-structural-row-checks-before-writing-a-new-brief.md) | todo; version 2 | Retain; reused or preserved by /03. |
| [statusgen/30](../statusgen/brief-30-repair-deterministic-defects-in-active-verify-rows-with-a-reviewed-dry-run.md) | todo; version 1 | Retain; reused or preserved by /03. |
| [statusgen/34](../statusgen/brief-34-one-owned-scratch-path-for-verify-rows.md) | todo; version 2 | Retain; reused or preserved by /03. |
| [statusgen/36](../statusgen/brief-36-content-bound-witness-schema-v2-reader.md) | implemented; version 2 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [statusgen/37](../statusgen/brief-37-content-bound-witness-schema-v2-writers.md) | todo; version 3 | Retain; reused or preserved by /03, /04, /05, /10, /11. |
| [statusgen/38](../statusgen/brief-38-one-time-v1-to-v2-witness-backfill.md) | todo; version 2 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [statusgen/40](../statusgen/brief-40-chain-approved-acceptance-transitions.md) | implemented; version 2 | Retain; reused or preserved by /03. |
| [desk-tools/30](../desk-tools/brief-30-verify-desk-landing-by-verdict-issue.md) | todo; version 1 | Retain; reused or preserved by /10. |
| [desk-tools/45](../desk-tools/brief-45-expose-the-canonical-verification-queue-to-its-consumers.md) | implemented; version 4 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [desk-tools/46](../desk-tools/brief-46-reuse-accepted-verification-inputs-to-stop-repeated-pass-dispatch.md) | todo; version 3 | Retain; reused or preserved by /05, /10. |
| [desk-tools/47](../desk-tools/brief-47-resume-interrupted-evidence-landing-through-the-existing-sanctioned-paths.md) | todo; version 3 | Retain; reused or preserved by /05, /10. |
| [desk-tools/48](../desk-tools/brief-48-report-remaining-landing-capacity-from-the-enforced-budget-owners.md) | todo; version 2 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [desk-tools/49](../desk-tools/brief-49-deduplicate-verifier-filings-by-their-canonical-work-identity.md) | todo; version 2 | Retain; reused or preserved by /05. |
| [desk-tools/50](../desk-tools/brief-50-bind-verifier-environment-evidence-to-the-run-admission.md) | todo; version 2 | Retain; reused or preserved by /10. |
| [desk-tools/51](../desk-tools/brief-51-drive-the-verifier-skill-from-the-current-queue-and-landing-contract.md) | todo; version 5 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [desk-tools/61](../desk-tools/brief-61-benign-shape-registry.md) | todo; version 1 | Retain; reused or preserved by /09. |
| [desk-tools/62](../desk-tools/brief-62-diff-finding-file-line.md) | todo; version 1 | Retain; reused or preserved by /09. |
| [desk-tools/63](../desk-tools/brief-63-registry-acceptance-shapes.md) | todo; version 1 | Retain; reused or preserved by /09. |
| [desk-tools/64](../desk-tools/brief-64-diff-scan-scope.md) | todo; version 1 | Retain; reused or preserved by /09. |
| [desktools-v2/06](../desktools-v2/brief-06-installation-token-scoping.md) | todo; version 5 | Retain; reused or preserved by /02, /07. |
| [desktools-v2/14](../desktools-v2/brief-14-regression-floor.md) | verified; version 4 | Retain; reused or preserved by /11. |
| [desktools-v2/58](../desktools-v2/brief-58-extract-the-role-credential-provider-and-retire-internal-desktoken-callouts.md) | todo; version 3 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [desktools-v2/73](../desktools-v2/brief-73-make-deskevidence-own-the-on-behalf-of-principal-on-runner-cells.md) | todo; version 1 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [desktools-v2/75](../desktools-v2/brief-75-build-the-shared-forge-library-and-make-the-desk-verbs-thin-wrappers.md) | todo; version 3 | Retain; reused or preserved by /08. |
| [milestone-support/01](../milestone-support/brief-01-extract-one-shared-typed-work-reference-resolver.md) | done; version 1 | Retain; reused or preserved by /01. |
| [milestone-support/08](../milestone-support/brief-08-normalize-pr-trailers-and-claims-through-the-shared-resolver.md) | todo; version 1 | Retain; reused or preserved by /01, /09. |
| [milestone-support/14](../milestone-support/brief-14-qualify-the-two-binary-rollout-and-prepare-release-adoption.md) | todo; version 1 | Retain; reused or preserved by /11. |
| [graph-execution/04](../graph-execution/brief-04-recovery-contract.md) | verified; version 5 | Retain; reused or preserved by /08. |
| [graph-execution/20](../graph-execution/brief-20-runner-contract.md) | verified; version 2 | Retain; reused or preserved by /06. |
| [graph-execution/25](../graph-execution/brief-25-publication-review-bridge.md) | todo; version 2 | Retain; reused or preserved by /11. |
| [graph-execution/26](../graph-execution/brief-26-shared-loop-admin.md) | todo; version 12 | Retain; reused or preserved by /06. |
| [graph-execution/27](../graph-execution/brief-27-standing-desk-loop-admin.md) | todo; version 11 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [graph-execution/28](../graph-execution/brief-28-verifier-domain-contracts.md) | verified; version 7 | Retain; reused or preserved by /01. |
| [graph-execution/29](../graph-execution/brief-29-verifier-regression-floor.md) | verified; version 2 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [agent-execution/01](../agent-execution/brief-01-qualified-execution-profile.md) | implemented; version 2 | Retain; reused or preserved by /02. |
| [agent-execution/02](../agent-execution/brief-02-prepared-environment-receipts.md) | todo; version 3 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [agent-execution/03](../agent-execution/brief-03-local-readiness-enforcement.md) | todo; version 3 | Retain; reused or preserved by /02, /07. |
| [desk-containers/15](../desk-containers/brief-15-cell-op-machine-seam.md) | implemented; version 2 | Retain; reused or preserved by /06. |
| [desk-containers/17](../desk-containers/brief-17-up-converges-the-cell.md) | implemented; version 3 | Retain; reused or preserved by /06. |
| [desk-containers/18](../desk-containers/brief-18-drain-pause-resume-handoff.md) | 9; version 1 | Retain; reused or preserved by /06. |
| [desk-containers/19](../desk-containers/brief-19-restart-rolling.md) | todo; version 3 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [desk-containers/20](../desk-containers/brief-20-status-overview.md) | todo; version 3 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [desk-containers/21](../desk-containers/brief-21-doctor.md) | todo; version 3 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [desk-containers/23](../desk-containers/brief-23-upgrade.md) | todo; version 3 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [desk-containers/24](../desk-containers/brief-24-gc.md) | todo; version 3 | Retain; reused or preserved by future/neighbor consumer or preserved foundation. |
| [desk-containers/28](../desk-containers/brief-28-package-in-cluster-cellctl-and-bind-the-interactive-desk-profile.md) | todo; version 5 | Retain; reused or preserved by /07. |
| [desk-containers/29](../desk-containers/brief-29-qualify-the-kubernetes-desk-cell-and-operator-recovery-end-to-end.md) | todo; version 6 | Retain; reused or preserved by /07. |

## Concrete reconciliation

- `statusgen/15`, /19–23 and graph-execution/28–31 already provide importable
  semantics and handoff foundations. Do not re-author them as a new domain service.
  The shared-policy consolidation is already implemented in the inspected source;
  its old proposal table is not an unimplemented backlog.
- `statusgen/37` is the existing writer/reader migration, not a new protocol to
  duplicate. New /03 consolidates document syntax; /04 supplies typed observations;
  /05 consolidates outcomes. /37 retains v2 record writing and adapter migration.
  Its only amendment in this authoring bundle is a new inverse dependency to /11.
- Scanner design is already fully scoped in `desk-tools/61–64`. No new scanner
  rewrite, exception list, scope narrowing or override is proposed. The new PR and
  Evidence adapters consume the reviewed result of that work.
- `desk-tools/46–51` retain fixed-delivery, reuse, continuation, capacity, dedupe,
  environment and skill-routing obligations and their decisions. /05 and /08–10
  supply their semantic/publication seams. Nothing here accepts draft policy or
  silently migrates a historical accepted record.
- `desktools-v2/06`, /58 and /75 retain custody and forge-library extraction.
  Pure consumers cannot import internal credential code, and making a library
  reusable does not give another process its authority.
- `graph-execution/19–27` and `agent-execution/01–09` retain durable coordination,
  supervised execution, workspace preparation and real backend qualification.
  /06–07 only remove duplication in the current local cell adapter. They add no
  replacement scheduler, store, automatic desk population or new backend.

## Already implemented lifecycle repair

Issue 6251 remains a useful regression case, but `down_settle.go` already captures
Sequence/LastStart before signaling and `recordIsOwners` uses that checkpoint
after holder removal. New /06 preserves this repair and consolidates remaining
decision-only projections with the existing `internal/cellcadence` owner. It does
not introduce a rival cellop state machine or reimplement the reported repair.

## Workspace ownership

The public `workspace/contract.go`, `spec/cell-workspaces-v1.md` and
`docs/contracts.md` S-worktree row are inspected foundations. Their request,
record, generation and lifecycle meanings must be reused by /01–02 and /06–07.
Allocator effects and caller migration are existing work, not implemented merely
because the contract module exists. The new briefs add no worktree allocator,
retention controller, lease store or managed-path convention.

## Active repairs to preserve

These PRs were open in the source-pinned snapshot. Their state/head must be read
again at pickup. They are not assumed merged dependencies or permission to take
over another worker's claim.

| PR | Existing narrow work | Refactoring disposition |
|---|---|---|
| [6179](https://github.com/medici-finance/assay/pull/6179) | Resolve item key before human-gate judgment | Retain; /02 characterizes its final behavior and removes independent plan derivation. |
| [6193](https://github.com/medici-finance/assay/pull/6193) | Shared context input bound | Retain; no new size constant in /02. |
| [6184](https://github.com/medici-finance/assay/pull/6184) | Refresh and run three shell suites | Retain; /07 accounts for the full remaining suite census and real binary. |
| [6225](https://github.com/medici-finance/assay/pull/6225) | Clear cellctl maintenance verb state | Retain; /06–07 preserve it as a transition/round-trip case. |
| [5550](https://github.com/medici-finance/assay/pull/5550), [5553](https://github.com/medici-finance/assay/pull/5553) | Cross-repo fail-first base and closure | Retain; /01 and /04 share the resulting coordinate rule. |
| [5547](https://github.com/medici-finance/assay/pull/5547) | Exact blob bytes when GraphQL text is incomplete | Retain; /10 consumes the provider adapter. |
| [6284](https://github.com/medici-finance/assay/pull/6284) | GitLab Evidence compare when HEAD is the tip | Retain; /10 preserves equality and unknown-comparison cases. |
| [5035](https://github.com/medici-finance/assay/pull/5035) | Bounded Evidence lock-preparation repair | Retain; /08 proves complete input binding before further hoisting. |
| [5480](https://github.com/medici-finance/assay/pull/5480) | Read-only role credential use | Retain; adapters must consume the existing credential owner. |
| [5935](https://github.com/medici-finance/assay/pull/5935) | Loop-admin PostgreSQL journal | Retain; /06–07 do not replace that durable owner. |
| [6275](https://github.com/medici-finance/assay/pull/6275) | Human-readable plan IDs | Retain; this is not a new lifecycle contract or refactor prerequisite. |
| [5842](https://github.com/medici-finance/assay/pull/5842) | Landing-capacity projection | Retain desk-tools/48; the new publication seam changes no cap. |

Completed Evidence and dated source accounts are preserved. If a later worker
finds an old report already fixed, retain its regression obligation and route
tracker resolution through the existing close process. Do not manufacture an
implementation to make the issue-routing row look complete.
