---
stream: statusgen
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
board: generated
---

# statusgen Stream

The planning board for `statusgen` itself — the tool that reads these streams and
generates the `STATUS.md` board. `statusgen`'s source lives in this repo (`statusgen/`),
so its own planning lives here too, alongside the code it plans.

Briefs are self-contained `brief-NN-*.md` files; each carries its own scope, rules,
task, and an executable Verify table. The opening set of briefs is statusgen's own
open planning work — issue and self-improvement metrics, the adoption-ladder indicator,
the drives anti-starvation floors + critical tier, the lint-firing audit, and the
findings-register state machine.

## Portfolio extension path — 13 October 2026

See the [contract and delivery amendment](../../proposals/non-ci-architecture/26-product-intent-and-portfolio.md).
The merged /19 contract and its Evidence are unchanged. /22 supplies product records; /23 extends /20 with bindings;
/24 supplies capacity accounting; /25 composes their views with continuing-operations/02.
/21 consumes /23 after both contracts ship. Dependency waves are 1: /22, 2: /24 and
operations/01, 3: /23 and operations/02, 4: /21 and /25 (alongside the existing /19→/20 path).
Operations admission remains a blocker for /25; the verifier runtime does not wait on this lane.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [30-day statusgen check-firing audit — retire cold --lint rules](brief-01-lint-firing-audit.md) | 1 | S | done | 2026-08-26 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #156 @ 112b206fee74b470016be325dc7c2dfeff670931) |
| 02 | [Issue metrics — statusgen --issues: standard counts + age/sitting-time + internal-vs-external + by-raising-desk](brief-02-issue-metrics.md) | 1 | L | done | 2026-08-26 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #156 @ 112b206fee74b470016be325dc7c2dfeff670931) |
| 03 | [Self-improvement metric — loops that self-diagnose AND self-resolve (agent-raised + agent-fixed, no human touch) vs human-touched](brief-03-self-improvement-metric.md) | 2 | M | done | 2026-09-15 sonnet-5-verifier | 2026-09-15 assay-reviewer-app[bot] (approved PR #1137 @ b9aed2c3fe00690c94043a16e30a9d6b5e96125e) |
| 04 | [Ladder-position indicator — one computed adoption-step number (behavioral axes, never tooling) on the board + roadmap deck](brief-04-ladder-position-indicator.md) | 1 | S | done | 2026-08-26 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #156 @ 112b206fee74b470016be325dc7c2dfeff670931) |
| 05 | [Drives phase 3 — anti-starvation floors + the hard critical tier (≤15/20 slots via a 2-pass fill, ≤6/8 workers, effectiveCap; a lexicographic never-buried tier fed by a stamped security label + a dependency-edge reciprocity lint so blockedCount is not gameable)](brief-05-drives-phase3-floors-critical-tier.md) | 1 | L | implemented | — | — |
| 06 | [Findings register becomes a corroborated state machine — bounded shelving (parked) + transition guard on resolved/affects/parked](brief-06-findings-register-state-machine.md) | 1 | L | implemented | — | — |
| 07 | [New brief-flow metrics in statusgen](brief-07-brief-flow-metrics.md) | 1 | L | done | 2026-09-01 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #306 @ 4f37b243efb70e1b1d3e726bc4019967ad64ad99) |
| 08 | [Composite AssayScore computation](brief-08-assayscore-computation.md) | 2 | M | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-05 assay-reviewer-app[bot] (approved PR #403 @ 8c10c90a0ed62049f45f647e7dad20aedcb212e1) |
| 09 | [Opt-in statusgen telemetry — anonymized fleet-drift corpus (off by default)](brief-09-optin-telemetry.md) | 1 | M | implemented | — | — |
| 10 | [statusgen graph export — derived-only DOT + JSONL from the existing parse tree, evaluated on real multi-hop questions](brief-10-graph-export.md) | 1 | M | done | 2026-09-01 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #318 @ 6ab8de53a40c1a4f71fa6c0a0ddccb4b27a000c8) |
| 11 | [DORA/insights hybrid — Apache DevLake for commodity metrics, our methodology metrics retained](brief-11-devlake-hybrid-metrics-split.md) | 1 | L | done | 2026-09-11 opus-4.8[1m]-verifier (assay 553dc2ae; rows 1-7 PASS, docs landed #574; 2026-09-01 FAIL superseded) | 2026-09-11 assay-reviewer-app[bot] (approved PR #860 @ 4e50f17e2722421e97f6880606baf6f8fadccc5e) |
| 12 | [`homed-in: <owner/repo>` brief field — exclude a brief whose deliverable lives in another repo from THIS board's Next-up, keep its tracking row, carry the target repo](brief-12-homed-in-field.md) | 1 | M | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-04 assay-reviewer-app[bot] (approved PR #404 @ 894d5e5f73ce417aa49c55134d10db3dc3675cfb) |
| 13 | [Cadenced roadmap artifacts — `--cadence weekly\|monthly` window computation reusing the roadmap renderer, a `theme:` render rule, config-driven priority order and brand](brief-13-cadenced-roadmap-artifacts.md) | 1 | M | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-04 assay-reviewer-app[bot] (approved PR #409 @ 3b022c17ea158700be8cfab679d1719c75afb7a4) |
| 14 | [`--lint`: flag a Verify row whose `go test -run` selector can pass on "no tests to run" (vacuous-selector rule)](brief-14-vacuous-gotest-run-lint.md) | 1 | M | done | 2026-09-30 assay-verifier-app[bot] @ 43420f7ecd74 (claude-opus-5-5) | 2026-09-30 assay-reviewer-app[bot] (approved PR #1659 @ f0355f1b57794105b59a1408522a44c7de59eb93) |
| 15 | [Extract canonical eligibility and coverage evaluation API](brief-15-extract-canonical-eligibility-and-coverage-evaluation-api.md) | 1 | L | verified | 2026-10-19 assay-verifier-app[bot] @ 4b15482af5c6 (claude-opus-5-5) | — |
| 16 | [Hold an issue out of dispatch when a non-human removed its excluded label: both scanners hold the placeholder, the issue board holds the un-briefed row](brief-16-label-removal-hold.md) | 1 | L | implemented | — | — |
| 17 | [`--scan-issues`: a `risk:high` label derives `gate: human` on the issue's placeholder (`risk:med` does not)](brief-17-risk-high-label-gate.md) | 1 | S | verified | 2026-10-19 assay-verifier-app[bot] @ 54c024fca36e (claude-opus-5-5) | — |
| 18 | [`--scan-issues`: print a notice when an open issue derives `gate: human` and its placeholder reads `gate: model` (detect only, no write)](brief-18-gate-mismatch-notice.md) | 2 | S | todo | — | — |
| 19 | [Context handoff contracts and synthetic fixtures](brief-19-context-handoff-contracts.md) | 1 | M | verified | 2026-10-19 assay-verifier-app[bot] @ 4b15482af5c6 (claude-opus-5-5) | — |
| 20 | [Offline future-state resolver with receipt and amendment validation](brief-20-offline-context-resolver.md) | 2 | L | verified | 2026-10-25 assay-verifier-app[bot] @ 0c3b558efec3 (forge-identity) | — |
| 21 | [Author skills consume the shared context handoff](brief-21-author-skill-handoff.md) | 4 | L | todo | — | — |
| 22 | [Product target records and compatible requirement parsing](brief-22-product-target-records-and-compatible-requirement-parsing.md) | 1 | M | verified | 2026-10-19 assay-verifier-app[bot] @ 4b15482af5c6 (claude-opus-5-5) | — |
| 23 | [Product target binding and context drift assessment](brief-23-product-target-binding-and-context-drift-assessment.md) | 3 | M | verified | 2026-10-27 assay-verifier-app[bot] @ 47ef38f961aa (forge-identity) | — |
| 24 | [Portfolio allocation and capacity accounting](brief-24-portfolio-allocation-and-capacity-accounting.md) | 2 | M | todo | — | — |
| 25 | [Read-only portfolio and drift projections](brief-25-read-only-portfolio-and-drift-projections.md) | 4 | M | todo | — | — |
| 26 | [Extend statusgen phase measurements through the shared performance recorder](brief-26-extend-statusgen-phase-measurements-through-the-shared-performance-recorder.md) | 4 | M | in-progress | — | — |
| 27 | [Reject newly introduced actionable lint notices on changed briefs](brief-27-reject-newly-introduced-actionable-lint-notices-on-changed-briefs.md) | 1 | M | todo | — | — |
| 28 | [Separate frozen historical notices from actionable lint findings](brief-28-separate-frozen-historical-notices-from-actionable-lint-findings.md) | 1 | M | todo | — | — |
| 29 | [Run the existing structural row checks before writing a new brief](brief-29-run-the-existing-structural-row-checks-before-writing-a-new-brief.md) | 2 | M | todo | — | — |
| 30 | [Repair deterministic defects in active Verify rows with a reviewed dry run](brief-30-repair-deterministic-defects-in-active-verify-rows-with-a-reviewed-dry-run.md) | 2 | M | todo | — | — |
| 31 | [Backfill derivable active brief metadata without fabricating evidence](brief-31-backfill-derivable-active-brief-metadata-without-fabricating-evidence.md) | 2 | M | todo | — | — |
| 32 | [Connect board drift notices to the existing reconciliation owners](brief-32-connect-board-drift-notices-to-the-existing-reconciliation-owners.md) | 1 | M | todo | — | — |
| 33 | [Drive active notice severity from the existing registry after cleanup](brief-33-drive-active-notice-severity-from-the-existing-registry-after-cleanup.md) | 4 | M | todo | — | — |
| 34 | [One owned scratch path for Verify rows](brief-34-one-owned-scratch-path-for-verify-rows.md) | 1 | L | todo | — | — |
| 35 | [Drain actionable notice debt in this repository without rewriting closed records](brief-35-drain-actionable-notice-debt-in-this-repository-without-rewriting-closed-records.md) | 3 | M | todo | — | — |
| 36 | [Content-bound witness schema v2: the reader and the one digest owner](brief-36-content-bound-witness-schema-v2-reader.md) | 1 | L | todo | — | — |
| 37 | [Content-bound witness schema v2: the writers and the desk readers](brief-37-content-bound-witness-schema-v2-writers.md) | 2 | L | todo | — | — |
| 38 | [One-time v1-to-v2 witness backfill where the witnessed content is still fetchable](brief-38-one-time-v1-to-v2-witness-backfill.md) | 3 | M | todo | — | — |
| 39 | [Multi-root objective reader: prerequisite graph, critical path and ranked moves](brief-39-multi-root-objective-reader.md) | 1 | L | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path
statusgen/02 (issue metrics) → statusgen/03 (self-improvement metric). The
self-improvement classifier extends the `--issues` infrastructure, so 02 leads 03.
statusgen/07 (brief-flow metrics) → statusgen/08 (composite AssayScore): the score rolls up
the brief-flow metrics, so 07 leads 08. statusgen/13 (cadenced roadmap artifacts) reuses the
landed `--roadmap` renderer over a computed window — independent, no new critical-path edge.
statusgen/14 (the vacuous `go test -run` Verify-row rule, issue #1581 part 5) adds one
advisory rule to the existing Verify-row lint — independent, no new critical-path edge.
statusgen/17 (`risk:high` gates a placeholder at first write) → statusgen/18 (the scan's notice
for a placeholder whose gate the issue has outgrown): 18 reports on every label and title word
that derives `human`, the label 17 adds among them, and extends the skill sentence 17 writes,
so 17 leads 18.
statusgen/19 (context handoff contracts) → statusgen/20 (offline resolver with receipt and
amendment validation) → statusgen/21 (author skills consume the handoff): the resolver
produces the 19 records and the skills may call only the shipped 20 verb, so 19 leads 20.
The portfolio extension adds typed edges (see the
[contract and delivery amendment](../../proposals/non-ci-architecture/26-product-intent-and-portfolio.md)):
statusgen/22 (product-target records) → statusgen/24 (allocation and capacity ledger), and
22 → continuing-operations/01 → continuing-operations/02; statusgen/23 (product binding)
needs 22 and 20; statusgen/21 now needs 20 and 23; statusgen/25 (read-only projections)
needs 23, 24 and continuing-operations/02. With /19 merged, the head of the
`20 → 23 → 25` path is statusgen/20 (plus /19's independent verification).
statusgen/36 (witness schema v2 reader and the one digest owner) → statusgen/37 (the v2
writers and the desk readers) → statusgen/38 (one-time v1-to-v2 backfill). The writers may
emit v2 only in a release whose reader understands it, and the backfill needs both, so 36
leads 37 and 37 leads 38 (medici-finance/assay#2026, medici-finance/assay#3658).
statusgen/39 (multi-root objective reader) has no in-repo `depends:`; its one prerequisite is
the v1 objective skill of #4107, carried as a `feathers:` ordering edge (a notice, never a
hold) because only the final skill-swap step needs it.
Every other brief is independent and self-contained.

## Dependency waves
- **Wave 1** — statusgen/01, statusgen/02, statusgen/04, statusgen/05, statusgen/06,
  statusgen/07, statusgen/09, statusgen/10, statusgen/11, statusgen/12, statusgen/13,
  statusgen/14, statusgen/15, statusgen/19, statusgen/22, statusgen/36, statusgen/39 (all
  independent; parallelizable).
- **Wave 2** — statusgen/03 (depends on statusgen/02), statusgen/08 (depends on statusgen/07),
  statusgen/18 (depends on statusgen/17), statusgen/20 (depends on statusgen/19),
  statusgen/24 (depends on statusgen/22; cross-stream: continuing-operations/01 is also wave 2),
  statusgen/37 (depends on statusgen/36).
- **Wave 3** — statusgen/23 (depends on statusgen/20 and statusgen/22; cross-stream:
  continuing-operations/02, depending on continuing-operations/01, is also wave 3),
  statusgen/38 (depends on statusgen/37).
- **Wave 4** — statusgen/21 (depends on statusgen/20 and statusgen/23), statusgen/25 (depends on
  statusgen/23, statusgen/24 and continuing-operations/02).

## Conventions
- `statusgen --lint-audit` reports 30-day per-rule firing counts; COLD (0-firing,
  un-tested) rules are retirement candidates — retirement stays a human call. Implemented
  by statusgen/01, PR #78.

## Library-first work — 2026-10-08

New work: `statusgen/15` extracts the evaluation API independently of forge reads.
`graph-execution/19` consumes it; no workflow-store dependency flows back into extraction.
See [the component contract](../../library-first.md). Existing completed history is unchanged.

## Non-CI architecture reconciliation — 2026-10-13

The maintainer ratified the non-CI verifier target (medici-finance/assay#2500, revision
23f655f5d); approval activates nothing. This stream owns the delivery-sequence step-2 package
(shared context handoff and authoring support): `statusgen/19` contracts, `statusgen/20`
resolver and receipt/amendment validation, `statusgen/21` skill integration with release and
adoption notes. Until 20 ships and is pinned, the bounded manual bootstrap packet remains the
documented path. Owner rows for every package: the
[amendment manifest](../../proposals/non-ci-architecture/20-amendment-manifest.md).
Existing completed history is unchanged.

## Performance reuse — 2026-10-16

Statusgen/26 extends the already-shared perfrec recorder after desktools-v2/61 (wave 2).
It measures discovery, parsing, evaluation, rendering and the existing deskread child;
invocation-local snapshot reuse must match fresh evaluation. It has no dependency on
the online mirror, perfdata storage or a database and acquires no credential. Context
resolver /20 retains independent membership enumeration and exact receipt semantics.
The desktools-v2 README holds the cross-stream performance priority and critical path.

## Lint debt delivery — 2026-10-17

The [notice-debt contract](#lint-notice-debt) maps source briefs /27–/33 and /35.
Wave 1: /27, /28, /32. Wave 2: /29 after /27; /30 and /31 after /28.
Wave 3: /35 after /27, /28, /29, /30, /31 and /32. Wave 4: /33 after /35 and
participating-consumer cleanup receipts. The longest source chain is /28 → /30
and /31 → /35 → /33. /27 prevents replenishment before cleanup is accepted.
Consumers separately verify a released artifact, adopt the pin and complete their
own cleanup. An Authors PR delivers planning only; it does not satisfy these briefs.

## Lint notice debt

Updated 2026-10-17 against source revision 8f1206fe5eb4b4ab19f5f2f7266c515aa71f8eae.
This is an execution contract for the existing statusgen stream. Authoring these
briefs implements no lint rule, activates no workflow and publishes no release.

### Current implementation and remaining gap

The existing differential compares PROBLEMs only; head NOTICEs pass through.
The row audit already rejects structural defects on new closures, requires witnesses,
and summarizes several rule families on pre-existing closed records. The generator
derives gate, wave and inverse dependencies and rejects some unusable commands.
Keep those protections and extend their owners. Do not build a second classifier,
Evidence writer, dehousing field or state reconciler.

Remaining work: a no-new-actionable-notices authoring ratchet; general historical
scoping and count reporting; full pre-write row validation; conservative row and
metadata repair tools; remaining board-drift loop consumers; and registry-owned
conditional severity after cleanup. Source ownership is the statusgen module;
consumer CI and pinned adoption are separate delivery boundaries.

### Public rule-family inventory

This inventory defines the acceptance sets used by /27–/33 and /35 at the source revision
above. It describes existing checks, not a new runtime registry or a severity
change. Rule identities below come from the existing lintRuleRegistry or emitter
constants, retaining their exact spelling; not every registered identity is
currently emitted as a bracketed tag. For an unregistered check, the existing Go
function plus the stated condition is the inventory key, not an invented emitted
tag. Implementers must preserve these identities in structured reports and
account for every included finding.

| Family | Included current identifiers and source owner | Repair disposition |
|---|---|---|
| A — structural Verify rows | `ere-literal-pipe`, `grep-zero-count`, `pipeline-exit-sunk`, `rE2-literal-pipe`, `unsubstituted-metavar`, `gorun-exit`, `bre-alternation`, `shredded-cell`, `moving-ref`, `gnu-only`, `gotest-run-vacuous`, `prose-led-command`, `cmd-marker-ambiguous`, `cmd-marker-overrides-command`, `cmd-marker-not-honoured`, `cmd-marker-vacuous`, `grep-pipe-zero-count` in `statusgen/verifyrows.go`; the older `verify-row-portability` check in `statusgen/verifyportability.go` | /30 may repair only a unique, semantics-preserving rewrite supported by the original command, Expect and immutable source. Missing metavariable values, ambiguous cells/markers, intended test names or output assertions require author evidence. Every active A finding must be resolved before /35 acceptance; automation is not required to invent a repair. |
| B — row strength and source dereference | `trivially-green`, `no-output-assertion`, `table-touches-no-files` in `statusgen/strength.go`; `id-dereference` in `statusgen/linkcheck.go` | Author-owed unless the existing contract and source uniquely establish the intended assertion or identifier. A nonexistent identifier is never repaired by inventing a test; explicitly planned names retain the existing planned-name handling. |
| B — risk, rationale and declared paths | `risk-files-crossread` in `statusgen/riskfilescrossread.go`; `checkBriefFiles` in `statusgen/brieffile.go`, specifically absent/substance-invalid `why` and absent `exec-tier-why` | /31 may derive a truthful file list only from an unambiguous immutable carrying-PR record. Risk answers, rationale and a missing/ambiguous carrying record are author-owed. An unreadable file list is could-not-check, never a zero finding count. |
| B — consumer routing | `consumers-unrouted`, `consumers-followup-no-target`, `consumers-followup-one-way`, `consumers-prose`, `consumers-no-verify-row`, `consumers-missing-list`, `consumers-out-of-scope-no-reason`, and the already-fatal `consumers-followup-missing-brief` in `statusgen/consumers.go` | /31 may add a back-reference only at the uniquely named, existing active follow-up target with corroborated source provenance. Selecting a target, inventing a consumer/reason or editing a closed target is author-owed. Existing fatal checks remain fatal. |
| B — ordering and legacy review metadata | `orderingGateNotices` in `statusgen/orderinggate.go`, for gate-shaped prose with no corroborating typed edge; `checkBriefFiles` in `statusgen/brieffile.go`, for a risk-classed done record with no security-review claim | Ordering suggestions require author review, never automatic dependency choices. Never manufacture a review claim. Corroborated pre-2026-08-15 closure dates retain the stated grandfather boundary; unknown dates remain could-not-check. Closed records are reporting inputs, not repair targets. |
| C — board ownership and merged delivery | `already-merged-unflipped`, `out-of-repo-deliverable`, `dehoused`, `re-homed`, `statusgen-source-elsewhere`, `deferred-by-gate` in `statusgen/boardhonesty.go`; `sibling-merge-unreconciled` in `statusgen/siblingmerge.go`; `mergedPRStatusNotices` in `statusgen/mergedstatus.go`, for a merged implementation whose row remains todo/in-progress | /32 routes each observed class to existing eligibility/reconciliation owners. A heuristic mention is not an ownership declaration; unavailable forge/sibling evidence stays unknown. An Authors trailer never proves delivery. |
| C — accepted-run attribution and register links | `verified-runner-attribution` in `statusgen/verifiedrunneragree.go`; `registerRefProblems` in `statusgen/registerrefs.go`, specifically references to a generated register view rather than a durable entry | /32 derives new attribution from the accepted run; historical Evidence stays frozen. The existing register backfill may rewrite a uniquely identified durable entry. Missing/ambiguous targets remain author-owed and no target is invented. |

“Derivable Family B” is an instance-level disposition, not every rule in a B row:
the report must name the immutable source and unique derivation for each repair.
All other B instances are author-owed, with a named owner and tracked disposition.
The /35 zero count covers all active A findings and exactly that corroborated B
subset; historical A/B counts are separate. /33's proposed promotion set is A plus
`risk-files-crossread`, subject to its human gate and cleanup/adoption receipts;
other B and C rules are not implicitly promoted. /27 includes actionable A/B
findings on changed briefs; /29 uses A and the existing identifier check. C remains
with the existing loop owners rather than becoming an authoring-notice failure.
Heuristic prompts such as `consumers-missing-list` do not by themselves prove an
actionable defect: record the author's disposition, and retain that existing
prompt-versus-verdict boundary when implementing the differential.

Exclusions are explicit: archived documents and pre-existing closed records are
outside the active repair/zero-count corpus; new closures keep all existing
closure, fail-first, witness and independence gates. Environment/eligibility
unknowns (including `eligibility-could-not-check`), operational debt counters,
informational summaries, and separate pattern/runtime-resource linters are not
members of A/B/C. Exclusion never converts unknown to clean or removes an existing
PROBLEM. Other detectors are outside this bounded cleanup/promotion set and must
be listed separately if observed; adding one requires an explicit inventory
amendment and its source owner, never an implicit expansion or silent suppression.

### Required behavior

1. Compare immutable base and head with one binary. Reject a new actionable defect
   on an added or edited brief without reddening an unrelated change for legacy debt.
   Preserve existing PROBLEMs and explicit could-not-check when comparison is unavailable.
2. Freeze pre-existing closed commands and Evidence. Default output summarizes historical
   findings; an explicit detail mode exposes them. A newly made closure still faces the
   existing closure, independence and witness checks. Reporting cannot bypass those gates.
3. Keep historical, active, environment/eligibility and informational categories distinct.
   Unknown is not clean. Historical totals may rise when a new detector discovers old
   records; do not mislabel discovery as a new defective closure or erase the old record.
4. Both repair tools are dry-run by default, atomic, idempotent and re-lint their candidate
   output. They never change Expect thresholds, risk choices, Evidence, or closed commands.
   Ambiguous semantics, absent provenance, missing identifiers, placeholders and missing
   rationale remain explicit author-owed items. Never invent plausible source facts.
5. Exclude archived documentation from live lifecycle classification. Preserve the
   pre-2026-08-15 security-review grandfather boundary only with corroborated closure dates;
   no fake review claim or retrospective rewrite is permitted.
6. Keep two-way consumer coverage. Backfill a genuine source reference only where the
   target and provenance exist; do not change a routing token to disguise deferred work.
7. The generator, authoring differential and closure checks are independent layers.
   A fixture that bypasses the generator must still fail at the PR boundary.
8. Runtime severity and its documentation have one owner. Promotion follows measured
   cleanup receipts and a verified release/adoption chain, not a source-merge assertion.

### Work on this board

| Brief | Deliverable | Wave |
|---|---|---|
| statusgen/27 | [Reject newly introduced actionable lint notices on changed briefs](brief-27-reject-newly-introduced-actionable-lint-notices-on-changed-briefs.md) | 1 |
| statusgen/28 | [Separate frozen historical notices from actionable lint findings](brief-28-separate-frozen-historical-notices-from-actionable-lint-findings.md) | 1 |
| statusgen/29 | [Run the existing structural row checks before writing a new brief](brief-29-run-the-existing-structural-row-checks-before-writing-a-new-brief.md) | 2 |
| statusgen/30 | [Repair deterministic defects in active Verify rows with a reviewed dry run](brief-30-repair-deterministic-defects-in-active-verify-rows-with-a-reviewed-dry-run.md) | 2 |
| statusgen/31 | [Backfill derivable active brief metadata without fabricating evidence](brief-31-backfill-derivable-active-brief-metadata-without-fabricating-evidence.md) | 2 |
| statusgen/32 | [Connect board drift notices to the existing reconciliation owners](brief-32-connect-board-drift-notices-to-the-existing-reconciliation-owners.md) | 1 |
| statusgen/35 | [Drain actionable notice debt in this repository without rewriting closed records](brief-35-drain-actionable-notice-debt-in-this-repository-without-rewriting-closed-records.md) | 3 |
| statusgen/33 | [Drive active notice severity from the existing registry after cleanup](brief-33-drive-active-notice-severity-from-the-existing-registry-after-cleanup.md) | 4 |

### Critical path and dependency waves

The real value head is statusgen/27: runDiffLintOne and its comparison seam already
exist, and the missing step is comparing actionable notices as well as PROBLEMs.
Historical classification in /28 is independently authorable; integrate its shared
reporting owner rather than forking classification in /27. Tool code exists in this
repository, so none of these source briefs is marked homed elsewhere.

Wave 1: /27 ratchet, /28 reporting, /32 remaining drift owners.
Wave 2: /29 generator after /27; /30 and /31 repairs after /28.
Wave 3: /35 source-corpus cleanup after /27, /28, /29, /30, /31 and /32.
Wave 4: /33 registry promotion after /35 and participating-consumer cleanup receipts.

The critical dependency chain is /28 → /30 and /31 → /35 → /33;
/27 prevents replenishment and must be available before cleanup is accepted.
Cleaning first without the ratchet is not a lasting result.

### Consumer boundary and acceptance

Publish the first reviewed release containing /27 through /32 through the existing
release procedure. Consumers verify published digests, adopt the pin and wire the
differential into PR checks. Each consumer gets its own bounded cleanup PR and an
independent verification receipt. Final /33 promotion and subsequent pin adoption
are a second step. Neither a local build nor an unreleased source commit is a
substitute for the consumer's pinned artifact.

For each participating root, record immutable base, binary version/digest, active
and historical counts by rule, and hashes proving frozen records stayed unchanged.
Active structural Family A and derivable Family B counts must reach zero. Every
remaining judgment-dependent item needs a named tracked owner and disposition.
An independent verifier runs the changed commands and a deliberately planted second
defect proves the ratchet still rejects new debt. Keep generated single-writer
status/register views out of authoring and cleanup branches.

### Boundaries

No rewriting done Evidence, no fabricated review/witness records, no invented why
text, no silent new security exception, no automatic tag, merge, infrastructure
probe or workflow activation. Operational decision-issue catch-up belongs to the
consumer's existing filer, including its trigger/ripeness and dedupe semantics;
it must not schedule two marker families for the same human decision.

## Pickup/context amendment — 18 October 2026

/20 includes applicable engineering guidelines and planned owners in the existing
scoped receipt recheck; /21 (effort L) integrates author/worker and dispatcher pickup
using that interface. Current manual instructions do not claim an automated gate.
Dependencies remain /19 → /20 → /23 → /21; no implementation is blocked waiting for
this automation when its authorized bounded bootstrap applies.
