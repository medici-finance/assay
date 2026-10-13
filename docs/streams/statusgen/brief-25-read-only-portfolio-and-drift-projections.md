---
brief: assay:assay:statusgen:25
title: Read-only portfolio and drift projections
why: "Users need one trustworthy view of product, work and investment drift instead of separate dashboards calculating conflicting answers."
wave: 4
depends: ["statusgen/23", "statusgen/24", "continuing-operations/02"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-13 by statusgen newbrief
sources:
  - docs/proposals/non-ci-architecture/21-product-intent-and-portfolio.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - freshness-checked 2026-10-13 @ d484dd5df24f2aa8d557dafce41687b38173f227
exec-tier: strong
exec-tier-why: Cross-contract identity, compatibility and evidence semantics require design and consumer reasoning.
domain: complicated
outcome: none
id: 5cec2f6f-26ad-4cd1-aee2-56133468a7b1
consumers:
  - "statusgen/portfolio.go, statusgen/main.go, schemas/portfolio-view-v1.json: follow-up statusgen/25"
  - "public API and cockpit: out-of-scope (R7 owner decision and serving brief still owed; consume this API without recomputation)"
  - "adopter live trial: out-of-scope (owner must select area, burden bound and activation independently)"
---

# Brief 25 — Read-only portfolio and drift projections

## Context

files: `statusgen/portfolio/`, `statusgen/portfolio.go` (planned), `statusgen/main.go`, `schemas/portfolio-view-v1.json` (planned), `docs/portfolio-views.md` (planned), `statusgen/testdata/portfolio/`, `docs/contracts.md`; `changelog/statusgen-25-portfolio.md` (planned).

facts (d484dd5df, 13 October 2026):
- /23 supplies product binding assessment; /24 supplies capacity; operations/02 supplies obligations.
- R7 public serving/cockpit ownership remains a separate decision; this slice supplies the pure consumer API.
- The operations prerequisites remain parked until explicitly admitted; a dashboard is not the first unblocking move.

layering: pure typed records/evaluators in `statusgen/portfolio`; filesystem and CLI adapters remain outside the package. No network, credentials, scheduler or runtime database. The flow and boundary fixtures below enforce this separation.

single-point-of-failure: an incorrect record interpretation could create a false alignment or completion claim; schema/record validation and independently supplied source/approval/coverage fixtures exercise different failure inputs, while existing admission and publication controls remain outside this read-only capability.

design-fit:
  owner: statusgen/portfolio
  contract: portfolio-view-v1 (planned extension; register its semantic owner in docs/contracts.md)
  retires: []
  weight: "one bounded contract/evaluation capability; no new service, scheduler or dispatch authority"
  why-add: "Existing records do not carry this meaning; extend their owning contracts rather than adding an independent product-state system."

## Read first

- `docs/proposals/non-ci-architecture/21-product-intent-and-portfolio.md` — exact record semantics, boundaries and owner mapping.
- `spec/registers-v1.md`, `spec/brief-v1.md` and the declared dependency briefs.
- `docs/streams/continuing-operations/spec.md` for operational identity and impact ownership.

## Ground rules

- Isolated branch and draft PR; no merge, release or activation.
- Synthetic offline inputs only; no collection from real transcripts or production systems.
- Preserve existing records and populated Evidence; stop at implemented for independent verification.
- Named tests and planned interfaces below must be delivered; they are not current commands claimed to pass.

## Task

1. Compose separate views for product condition, architecture conformance references, product binding mismatch, allocation divergence and observation expiry. Include subject/revision, policy/window, provenance, coverage and responsible owner; missing architecture evidence stays unknown. Use prerequisite evaluators, never duplicate their arithmetic or authority.
2. Expose a pure BuildView-style API and thin offline statusgen portfolio --snapshot DIR --as-of TIME --json adapter with a versioned wire schema. Filter source visibility before joins/counts and scope cache/digest identity to the admitted source set; no provider reads, effects or live policy changes.
3. Add a synthetic end-to-end journey and CLI/schema parity fixtures, plus consumer documentation for future R7/adopter clients. Report a missing supported producer as unavailable; never implement a fallback in a renderer. Include a manual shadow exercise and stop criteria for burden and missing obligations, without selecting an adopter pilot or activating a recurring process.

## Interface contract

Input: approved/observed snapshots from prerequisite contracts, authorized source scope and explicit as-of. Output: portable portfolio-view-v1 with five separate drift dimensions and coverage. Offline CLI and future service share the same evaluator; no new dispatch path.

## Verify

Run from the repository root after implementation. Each named test asserts the stated
behavior and includes the specified failing mutation. A missing test/package or absent
named PASS fails the row; do not substitute a skipped or zero-test run.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioProjectionJourney$" . > "$check_output" && grep -F -- "--- PASS: TestPortfolioProjectionJourney " "$check_output")` | exit 0; named PASS; actual CLI composes target change, compatible disposition, 5/2 allocation divergence and due observation without changing dispatch or completion; mutation: renderer recomputes a target |
| 2 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioProjectionVisibility$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioProjectionVisibility " "$check_output")` | exit 0; named PASS; source filtering precedes counts and shared-subject joins; unauthorized input leaks no hidden total; mutation: filter only final rows |
| 3 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioProjectionWireParity$" . > "$check_output" && grep -F -- "--- PASS: TestPortfolioProjectionWireParity " "$check_output")` | exit 0; named PASS; pure result, actual CLI JSON and schema agree on revisions/unknowns, including missing producer; mutation: flatten unknown to zero |
| 4 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioProjectionNoEffects$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioProjectionNoEffects " "$check_output")` | exit 0; named PASS; hostile effect adapter remains unused for every projection path and input/output snapshots remain unchanged; mutation: publish a proposed policy |

## Evidence

<!-- Independent verifier records subject revision, command, exit, named result and environment. No implementation or execution evidence is asserted here. -->

## Review

Gate: model; all risk answers no because this slice processes synthetic/explicit offline
snapshots and makes no live policy, publication or deployment change. Review semantic
compatibility and the real consumer flow; later live integration derives its own risk gate.
