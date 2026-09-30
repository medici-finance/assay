# Proposed brief outlines

**Proposal only.** These are fourteen candidate scopes, not executable briefs. Numbers
are provisional local outline IDs, not claimed canonical brief IDs or lifecycle cells.
The stream remains parked. Each accepted scope needs a self-contained `brief-*.md`
with generated identity, real paths, typed dependencies, risk-derived gate, design-fit,
consumer mapping and discriminating executable Verify rows before dispatch.

Read [the specification](spec.md) and [stream admission/critical path](README.md).
The effort estimates are ceilings for review, not duration estimates. Split an L-sized
scope if its reviewed interfaces cannot fit one independently verifiable change.

## Dependency waves

Local waves are topological layers, not dates. External graph prerequisites may dominate
elapsed delivery and are never satisfied merely because an earlier local wave finishes.

| Candidate | Scope | Wave | Effort | Local prerequisites | External prerequisite owners |
|---|---|---|---|---|---|
| [O01](#o01) | Durable user journeys and workflows | 0 | L | None | None |
| [O02](#o02) | Portable measurement definitions | 1 | L | O01 | None |
| [O03](#o03) | Expected impact on brief-v2 | 2 | L | O01, O02 | None |
| [O04](#o04) | Recorded-data measurement adapter | 2 | M | O02 | graph-execution/06 |
| [O05](#o05) | Practices and optional method profiles | 2 | M | O01, O02 | None |
| [O06](#o06) | Prediction-to-outcome integration | 3 | M | O03, O04 | graph-execution/06, graph-execution/09, graph-execution/17 |
| [O07](#o07) | Derived Operations views | 4 | M | O05, O06 | None |
| [O08](#o08) | Impact-aware authoring and review guidance | 3 | M | O03, O05 | None |
| [O09](#o09) | Methodology-switch conformance | 4 | M | O05, O06 | None |
| [O10](#o10) | Manual outcomes-desk procedure | 5 | M | O05, O07, O08 | None |
| [O11](#o11) | Bounded cadence activation | 6 | L | O05, O10 | graph-execution/04, graph-execution/09, graph-execution/14, graph-execution/16 |
| [O12](#o12) | Adopter scaffolding and packaged examples | 6 | M | O07, O08, O09, O10 | None |
| [O13](#o13) | Independent portable acceptance exercise | 7 | M | O12 | None |
| [O14](#o14) | Brief-v3 migration and adoption default | 8 | L | O03, O08, O13 | None |

## Proposed scopes

All paths below describe proposed implementing surfaces unless the spec identifies an
existing contract owner. Refresh exact file names and source revisions during full
brief authoring. Acceptance scenarios below are design obligations; they do not claim
that named implementation tests or commands already exist.

<a id="o01"></a>
### O01 — Durable user journeys and workflows

- **Wave / effort:** 0 / L.
- **Depends on:** No local prerequisite. **External:** None.
- **Unblocks:** O02, O03, O05.
- **Why:** Give each enduring subject a stable identity, accountable owner and versioned definition.
- **Deliverables:** operational-subject-v1 specification/schema, per-object file conventions, typed relations and validation. Keep journey goals/touchpoints separate from workflow steps and execution patterns.
- **Owning surfaces:** New `spec/operational-subject-v1.md` (planned), `schemas/operational-subject-v1.json` (planned), subject validator and fixtures under `statusgen/`; source conventions under `docs/operations/`.
- **Acceptance scenarios:** A journey spans multiple workflows; a workflow supports multiple journeys. Rename preserves identity; dangling and wrong-kind references are rejected. Stream closure cannot retire its journey.
- **Review / activation considerations:** Set the identity/revision and retirement rules before choosing storage convenience. Wrong-kind, duplicate-ID and dangling-reference cases must be rejected.

<a id="o02"></a>
### O02 — Portable measurement definitions

- **Wave / effort:** 1 / L.
- **Depends on:** O01. **External:** None.
- **Unblocks:** O03, O04, O05.
- **Why:** Establish the common evidence vocabulary before collecting data or choosing a methodology.
- **Deliverables:** measurement-v1 definitions for outcomes, flow, effort, quality and evidence health; instrument/units/cohort/window semantics; bounded sampling/retention and extension rules. Reuse existing flow instruments where compatible.
- **Owning surfaces:** New `spec/measurement-v1.md` (planned), `schemas/measurement-v1.json` (planned), measurement validator/fixtures under `statusgen/`; consume existing flow instruments without redefining their semantics.
- **Acceptance scenarios:** Missing is distinct from zero; incompatible units or cohorts cannot be combined; metric revisions do not silently restate old measurements. No Cynefin field is required.
- **Review / activation considerations:** Approve the bounded common core, collection budget and extension process. Reject requests for universal telemetry or fabricated baselines.

<a id="o03"></a>
### O03 — Expected impact on brief-v2

- **Wave / effort:** 2 / L.
- **Depends on:** O01, O02. **External:** None.
- **Unblocks:** O06, O08, O14.
- **Why:** Preserve intent now without forcing an immediate brief-schema migration.
- **Deliverables:** change-impact-v1 and an explicitly validated optional v2 extension: subject relations, hypotheses, predictions, guardrails, evaluation owner/trigger, and honest dispositions. Preserve existing measures/outcome meanings; define revision and applicability rules.
- **Owning surfaces:** New `spec/change-impact-v1.md` (planned), matching schema, `schemas/brief-v2.json`, `statusgen/briefv2.go` and related parser/semantic validators; brief template example.
- **Acceptance scenarios:** A prediction is pinned before assessment; later edits create a new version. Unsupported readers report missing capability. Enabling and no-direct-impact work remain valid without fabricated targets.
- **Review / activation considerations:** Confirm optional-extension capability handling before adoption. If required semantics cannot be safely negotiated on v2, return the compatibility decision for review rather than silently ignoring them.

<a id="o04"></a>
### O04 — Recorded-data measurement adapter

- **Wave / effort:** 2 / M.
- **Depends on:** O02. **External:** graph-execution/06.
- **Unblocks:** O06.
- **Why:** Collect baseline evidence before changing the operating process.
- **Deliverables:** One neutral file/receipt adapter with source references and coverage reports; use the canonical record envelope. Provide two synthetic domains and a manual import path. No default production queries or vendor telemetry integration.
- **Owning surfaces:** Recorded-input adapter and fixtures under `statusgen/`; consume the run-record envelope owned by graph-execution/06. No new event ledger.
- **Acceptance scenarios:** Duplicate, delayed, missing and contradictory receipts retain provenance and honest coverage. Collection cost is visible. Replay of the same receipts reproduces the same measurement inputs.
- **Review / activation considerations:** Refresh graph-execution/06 acceptance evidence first. Synthetic imports must include duplicates, late arrivals and contradictory receipts, with a stable replay cutoff.

<a id="o05"></a>
### O05 — Practices and optional method profiles

- **Wave / effort:** 2 / M.
- **Depends on:** O01, O02. **External:** None.
- **Unblocks:** O07, O08, O09, O10, O11.
- **Why:** Separate continuing responsibility from the chosen analytical method.
- **Deliverables:** improvement-practice-v1: definitions, scoped bindings, cadence declarations, interpretation profile/version, declared input requirements and independent policy references. Profiles may consume shared evidence without changing its schema.
- **Owning surfaces:** New `spec/improvement-practice-v1.md` (planned), matching schema, practice/profile validator and fixtures; profile references separate from policy references.
- **Acceptance scenarios:** One subject supports several practices; a profile switch preserves subject and metric IDs. Missing profile inputs remain explicit; changing a display lens cannot change permission or cadence.
- **Review / activation considerations:** Approve the separation between interpretation and operational policy. A lens/profile change must not alter access, scheduling or permission.

<a id="o06"></a>
### O06 — Prediction-to-outcome integration

- **Wave / effort:** 3 / M.
- **Depends on:** O03, O04. **External:** graph-execution/06, graph-execution/09, graph-execution/17.
- **Unblocks:** O07, O09.
- **Why:** Join what was intended to what was exposed and observed.
- **Deliverables:** Consume and compatibly extend the existing lifecycle-link owner: exact impact snapshot, instance/attempt, release/exposure, observation, interpretation and next-decision references. Add intervention groups for releases containing several briefs.
- **Owning surfaces:** Extend the lifecycle-link validator/projection owned by graph-execution/17; consume instance/work-input identity from 09 and run records from 06.
- **Acceptance scenarios:** Wrong-artifact/version/cohort joins are refused. Unreleased, unobserved and inconclusive remain distinct. A two-brief release cannot acquire invented individual causal credit; null/adverse outcomes survive replay.
- **Review / activation considerations:** Confirm one canonical lifecycle-link owner. Support later/corrected observations without erasing the evidence used for earlier decisions.

<a id="o07"></a>
### O07 — Derived Operations views

- **Wave / effort:** 4 / M.
- **Depends on:** O05, O06. **External:** None.
- **Unblocks:** O10, O12.
- **Why:** Make continuing health and outstanding evaluations visible after delivery closes.
- **Deliverables:** statusgen Operations projection and JSON view, generated OPERATIONS.md, one publisher per destination, manifest/watermark and atomic stale-publish rejection. Reuse one evaluator for CLI and cockpit consumers.
- **Owning surfaces:** Operations evaluator/export in `statusgen/`, versioned JSON contract, publisher integration and deterministic fixtures; generated `OPERATIONS.md` is output only.
- **Acceptance scenarios:** Regeneration is deterministic and does not alter sources. Concurrent producers write separate records; duplicate/conflicting facts are handled explicitly. A done brief with a pending outcome stays visible with an owner.
- **Review / activation considerations:** Define one publisher per destination and visibility-scoped projections. Verify that a less-privileged projection cannot disclose restricted evidence metadata.

<a id="o08"></a>
### O08 — Impact-aware authoring and review guidance

- **Wave / effort:** 3 / M.
- **Depends on:** O03, O05. **External:** None.
- **Unblocks:** O10, O12, O14.
- **Why:** Make the new contracts normal authoring practice for every adopter.
- **Deliverables:** Public spec-authoring reference, author-brief/template changes, review and verify guidance. Teach user-journey linkage, portable measures, method-profile selection, uncertainty, measurement gaps and independent assessment. Keep one canonical reference.
- **Owning surfaces:** `plugins/assay/skills/author-brief/SKILL.md`, its canonical references, public brief template, and applicable review/verify guidance. Teach spec authors through a shared public reference rather than assuming a separate spec-writing skill exists.
- **Acceptance scenarios:** Author representative predicted, learning, enabling and no-direct-impact briefs. Each is interpretable without organization-specific context; a brief can deliver a valid disconfirming experiment without pretending the hypothesis succeeded.
- **Review / activation considerations:** Keep delivery Verify obligations distinct from later outcome evaluation. Include a disconfirmed learning hypothesis as successful experimental delivery.

<a id="o09"></a>
### O09 — Methodology-switch conformance

- **Wave / effort:** 4 / M.
- **Depends on:** O05, O06. **External:** None.
- **Unblocks:** O12.
- **Why:** Prove the method-neutral design with a second interpretation before it hardens.
- **Deliverables:** A Cynefin assessment profile and a bounded flow-improvement profile over the same fixture corpus; coverage preview and shadow/replay report. Keep contextual judgments explicit rather than inferring domains from metrics.
- **Owning surfaces:** Versioned profile fixtures and conformance harness under `statusgen/`; public method-switch guide and coverage report.
- **Acceptance scenarios:** Changing profile needs no rewrite of subjects, events or predictions. Prior decisions retain their original profile. Missing service-time evidence blocks a bottleneck conclusion instead of inventing one.
- **Review / activation considerations:** State profile coverage limits. Record contextual Cynefin judgments explicitly; do not classify domains mechanically from throughput.

<a id="o10"></a>
### O10 — Manual outcomes-desk procedure

- **Wave / effort:** 5 / M.
- **Depends on:** O05, O07, O08. **External:** None.
- **Unblocks:** O11, O12.
- **Why:** Establish accountable follow-through before adding another running service.
- **Deliverables:** A neutral outcomes responsibility, hostable by the existing desk or human: due queue, evidence review, independent assessment and no-change/investigate/propose-change/escalate exits. Route new work through intake.
- **Owning surfaces:** Public outcomes procedure/reference in `plugins/assay/`, with existing intake and verification interfaces; no new standing process required.
- **Acceptance scenarios:** An evaluator reconstructs a completed intervention, records an adverse or inconclusive result and routes the next decision without editing the aggregate or self-verifying implementation. Missing telemetry has an explicit disposition.
- **Review / activation considerations:** Name the accountable practice owner and independent evaluator; define delegation, overdue escalation and handoff after stream closure.

<a id="o11"></a>
### O11 — Bounded cadence activation

- **Wave / effort:** 6 / L.
- **Depends on:** O05, O10. **External:** graph-execution/04, graph-execution/09, graph-execution/14, graph-execution/16.
- **Unblocks:** cadence-enabled adopter/client rollout.
- **Why:** Automate due assessments without introducing a second scheduler or duplicate interventions.
- **Deliverables:** Bind practice triggers/windows/deadlines to existing admission, claims, recovery and budget machinery. Define occurrence identity, restart, coalescing, overdue behavior and stale-input handling. No automatic remediation or policy self-modification.
- **Owning surfaces:** Practice-to-instance adapter at existing graph admission/dispatch boundary; recovery, coalescing and budget conformance fixtures. No second scheduler.
- **Acceptance scenarios:** Repeated triggers and a crash resume one logical occurrence; cumulative budget survives restart; stale sources cannot initiate an unjustified intervention. Existing permission denial remains binding.
- **Review / activation considerations:** Require prerequisite contract acceptance and an explicit activation decision. Derive risk from actual scheduled effects; no production access is implied by the outline.

<a id="o12"></a>
### O12 — Adopter scaffolding and packaged examples

- **Wave / effort:** 6 / M.
- **Depends on:** O07, O08, O09, O10. **External:** None.
- **Unblocks:** O13.
- **Why:** Make this installable Assay functionality, not a private house convention.
- **Deliverables:** Optional public install/adopt scaffolding for docs/operations, neutral sample subjects/metrics/practices, bundle references, minimal manual-mode quickstart and capability reporting. Keep actual release publication in the normal release process.
- **Owning surfaces:** Public install/adopt skill references and packaged examples; optional `docs/operations/` scaffolding. Publication uses the normal release process.
- **Acceptance scenarios:** A clean project can declare a journey, import recorded evidence and generate the view without private repos, credentials, a cockpit, a hosted backend or scheduled agents.
- **Review / activation considerations:** Support a capability-limited manual install and version pinning. A clean adopter must not depend on organization-specific configuration.

<a id="o13"></a>
### O13 — Independent portable acceptance exercise

- **Wave / effort:** 7 / M.
- **Depends on:** O12. **External:** None.
- **Unblocks:** O14.
- **Why:** Test whether another adopter can actually complete the learning cycle.
- **Deliverables:** An independent evaluator uses the packaged flow in clean adopter fixtures for a product journey and a delivery-operation journey. Record authoring effort, evidence completeness, outcome handling, method-switch gaps and unresolved usability defects.
- **Owning surfaces:** Clean adopter fixtures, reproducible acceptance instructions and recorded evaluation report; evaluator must be independent of implementation.
- **Acceptance scenarios:** Complete intent → delivery witness → recorded exposure → assessment → next decision, then close the stream and repeat assessment on the surviving journey. Reject wrong-version and missing-evidence cases. This proves portability, not real-world savings.
- **Review / activation considerations:** Select an independent evaluator and predeclare completion criteria. Report usability defects and measurement cost, without claiming fixtures demonstrate real-world benefit.

<a id="o14"></a>
### O14 — Brief-v3 migration and adoption default

- **Wave / effort:** 8 / L.
- **Depends on:** O03, O08, O13. **External:** None.
- **Unblocks:** conditional new-author defaults and adopter migration.
- **Why:** Make impact dispositions enforceable only after the extension and adoption path have been exercised.
- **Deliverables:** Conditional on the schema-policy decision: dual readers, schema/validator parity, lossless migration and new-author defaults. Preserve IDs and existing semantics; mark historical impact as unrecorded. Explicit minimum-tool capability and rollback.
- **Owning surfaces:** Conditional new brief schema, dual readers, migration and authoring defaults across schemas, statusgen and bundled guidance. Keep schema and semantic validator behavior consistent.
- **Acceptance scenarios:** Mixed v2/v3 trees work under the declared policy; old readers refuse unsupported required semantics; migration is idempotent and invents no historical prediction. Impact changes invalidate the relevant evidence binding.
- **Review / activation considerations:** Remain conditional until the schema-policy ruling after 13. Historical missing predictions must stay unrecorded; migration needs an explicit compatibility and rollback plan.

## Consumer follow-through

Public contract delivery ends at the shared projection and its conformance examples.
Actual cockpit consumers need two separately authored scopes in their owning project:

1. Subject/impact/outcome rooms over O07, extending existing projection and room owners.
2. Method comparison and cadence views over O09/O11, preserving backend authorization.

[The cockpit appendix](cockpit.md) supplies their user journeys, wireframes and acceptance
cases. Manual public adoption does not wait for these client-specific changes. O11 is
also a separate automation branch: O12/O13 deliberately prove manual adoption without it.

## Authoring admission checklist

- Record the spec decision and stream-capacity decision; do not infer approval from
  merging a draft proposal. File the strong-tier authoring follow-on at spec approval.
- Refresh external owner files and acceptance evidence. Preserve their existing IDs and
  lifecycle; add full typed prerequisite edges to executable briefs.
- Derive each gate from the four risk answers at the actual scope. Permission-sensitive
  effects need their own review; a proposed UI or profile never grants authority.
- Include positive, missing-data and wrong-version cases across real production seams;
  future tests must fail when the claimed control is bypassed.
- Give every actual brief standalone context, exact interfaces and runnable independent
  verification. Leave Evidence empty until execution; never mark this outline delivered.
- Generate the README Briefs table from actual authored briefs. Keep proposed scopes out
  of lifecycle rows until then; central boards remain generated.
