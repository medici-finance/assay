---
brief: "assay:assay:agent-execution:07"
title: "Evaluate governed harness refinement with isolated offline fixtures"
why: "Harness improvement can overfit the task it probes or weaken its own controls. A bounded experiment must separate useful profile proposals from unsafe changes and optimistic evaluation before any live adoption."
wave: 3
depends: ["agent-execution/01", "agent-execution/02"]
unblocks: []
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
authored: "2026-10-16 by execution architecture authoring session"
sources: ["docs/proposals/non-ci-architecture/27-agent-execution-environments.md", "freshness-checked 2026-10-16 @ ec8d77e56627a7c2269f3d143ef6a31f3e41bd02 (origin/main)", "google-research/harness2 @ 1cfe254e8d25b15f37048c3730134686abe60335; README.md and docs/EXPERIMENTS.md"]
exec-tier: "strong"
exec-tier-why: "Experimental validity, protected authority boundaries and cross-task evidence require design judgment."
domain: "complex"
outcome: "none"
consumers: ["loopadmin/experiments/harnessrefinement: follow-up agent-execution/07 (this brief; flips to fixed-here when the implementation edits the path)", "docs/reports/agent-execution/harness-refinement.md: follow-up agent-execution/07 (this brief; flips to fixed-here when the implementation edits the path)", "performance-collection/09 proposal loop: out-of-scope (retains proposal scheduling, approved effects and promotion; this investigation only supplies experimental evidence)", "installed skills and runtime profiles: out-of-scope (no live edit or promotion in this brief)"]
version: 1
id: "a39c5fdb-370a-4459-a7a7-59b083eb7585"
---

# Brief 07 — Evaluate governed harness refinement with isolated offline fixtures

## Context

files: `loopadmin/experiments/harnessrefinement/` (planned), `loopadmin/testdata/harnessrefinement/` (planned), `docs/reports/agent-execution/harness-refinement.md` (planned) — and a changelog fragment. These are an offline experiment and report, not a production refinement engine.

single-point-of-failure: the allowed-change boundary on a proposed profile — behind it, execution fixtures have no publisher/operator capability and an independently owned held-out evaluator rejects regressions and contaminated results.

facts: Source inspected 16 October 2026 at Assay main ec8d77e56627a7c2269f3d143ef6a31f3e41bd02. No Harness² integration or harness-refinement experiment exists at the named paths. Harness² revision 1cfe254e8d25b15f37048c3730134686abe60335 describes propose–probe–compose, retained per-domain lessons and existing Codex/OpenCode executors. Its experiment guide records unrestricted network access and a rollout retrieving its public task rubric. These are source observations, not reproduced benefit or a claim of AX integration.

Contract-consumption: agent-execution/01 supplies qualified profiles and immutable component-bundle identity; agent-execution/02 supplies prepared-environment receipts. Neither grants authority to promote a proposed bundle. Existing performance-collection/09 and reviewed profile-change owners retain proposal/promotion responsibilities.

## Read first

- [Future-state amendment and Harness² evidence](../../proposals/non-ci-architecture/27-agent-execution-environments.md#harness-improvement-as-reviewed-profile-proposals).
- [Stream boundaries and dependencies](README.md).
- [Existing improvement proposal owner](../performance-collection/brief-09-bounded-performance-repair-loop.md).
- Harness² README and experiment guide at the exact source revision above; retain small attributed source fixtures and hashes for claims used in the report.

## Ground rules

- Isolated branch and draft PR; no merge, deployment, model/provider calls or live infrastructure contact.
- Synthetic tasks, deterministic fake editors/executors and non-secret fixtures only. No installation or execution of upstream scripts with sandbox-disabled settings.
- Never edit installed instructions, operator policy, authorization, enforced guardrails, acceptance criteria, credentials or an active invocation's bundle. A proposed composition remains untrusted until the existing review/profile-adoption owner accepts it.
- Stop at implemented; independent verification owns verification. Fixture protocol success cannot establish model-backed benefit or authorize a production optimizer.

## Task

1. Retain pinned, attributed source evidence and build a small offline proposal/probe/composition record using /01 profile/bundle references and /02 preparation receipts. Record origin, input task/version, every candidate and trial, budget accounting, selection rationale, retained lesson and the independently owned evaluation subject. Use existing profile validation; add no launcher, scheduler, new memory authority or credential path.
2. Restrict candidate edits to declared experiment-owned prompt/skill/tool-guidance artifacts. Reject attempted changes to operator policy, enforced controls, role/tool authorization or acceptance; a lesson cannot mutate a live bundle. Model output and retained lessons remain untrusted inputs. Bypass proposal filtering and show that the isolated executor still lacks effect/operator authority.
3. Run deterministic controlled fixtures: unchanged baseline, equal-budget retries, a refinement that helps a probe but fails an unseen task, a harmful composition of individually successful edits, protected-control mutation and hidden-rubric access. Keep the held-out oracle in a separate evaluator fixture, absent from executor inputs; inject a forbidden rubric-read request and require the experiment boundary to refuse it. Freeze selection before revealing results. Physical/network isolation remains unqualified until a separately authorized real-run trial. Separate domains/tasks and reject reuse across incompatible model, profile, seed or dataset revisions.
4. Produce a go/no-go/unknown report and a proposed separately authorized model-backed trial. Distinguish protocol validity from measured benefit. Include all baseline/reference, proposal, probe, composition, reflection, failed-attempt and review cost/coverage; evaluate accepted-and-verified work and regressions against both comparators. Do not import upstream benchmark gains as Assay results or treat same-task self-assessment as independent acceptance. No numeric benefit threshold or live promotion is selected here; the trial owner must settle them before running it.

## Interface contract

Immutable proposed bundle, trial manifest and scoped fixture observations enter; retained candidate provenance, independent outcomes, complete-or-unknown cost and a decision report leave. Rejection or missing evaluation remains visible. Nothing returned is a profile activation, acceptance decision or authority grant.

## Verify

Named tests and fixtures below are implementation deliverables. Each production experiment-validator path must fail under its stated mutation; report claims must dereference pinned source fixtures or actual retained trial results. Human review owns the adequacy of the proposed live trial.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +dereference | `(cd loopadmin && refinement_out=$(mktemp) && trap 'rm -f "$refinement_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestHarnessRefinementProtectedBundle$" ./experiments/harnessrefinement/... > "$refinement_out" && grep -q -- "--- PASS: TestHarnessRefinementProtectedBundle " "$refinement_out")` | exit 0; named PASS; proposed control edits fail; with filtering bypassed the executor still has no operator/effect authority; mutation: allow a candidate to overwrite a protected policy artifact — named test must fail |
| 2 | check:ci +flow +mutation +dereference | `(cd loopadmin && refinement_out=$(mktemp) && trap 'rm -f "$refinement_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestHarnessRefinementHeldoutAndContamination$" ./experiments/harnessrefinement/... > "$refinement_out" && grep -q -- "--- PASS: TestHarnessRefinementHeldoutAndContamination " "$refinement_out")` | exit 0; named PASS; probe-only overfit and a regressing composition are detected independently; forbidden rubric-read requests are refused and observed leakage invalidates the trial; mutation: let the executor read held-out expected results — named test must fail |
| 3 | check:ci +flow +mutation +dereference | `(cd loopadmin && refinement_out=$(mktemp) && trap 'rm -f "$refinement_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestHarnessRefinementCostAndEvidence$" ./experiments/harnessrefinement/... > "$refinement_out" && grep -q -- "--- PASS: TestHarnessRefinementCostAndEvidence " "$refinement_out")` | exit 0; named PASS; all experimental stages and failed attempts remain in totals; unknown figures stay unknown and mutated source/result evidence invalidates the report; mutation: omit probe and reflection cost from the comparison — named test must fail |
| 4 | check:ci | `statusgen --consumers --brief agent-execution/07 --base $(git merge-base origin/main HEAD)` | exit 0; delivery sites corroborate; out-of-scope scheduling and promotion judgments remain UNCHECKED and reviewer-owned |

## Pre-mortem and dispatch checks

Probe overfit, contaminated grading, harmful composition, control edits and omitted trial costs map to the named Verify rows. Each mutation must make its test fail; retain the executable mutation commands with the fixtures. Report interpretation and whether a proposed model-backed trial would change adoption remain review-only. Refresh source/claims at pickup; do not rewrite an active plan's acceptance or infer promotion authority from this investigation.

## Evidence

<!-- Independent verifier records exact command, result, subject/environment and date. -->

## Review

Gate: model. Review evidence separation, independent controls and the limits of deterministic fixtures. Live evaluation or profile adoption requires its existing separate authorization.
