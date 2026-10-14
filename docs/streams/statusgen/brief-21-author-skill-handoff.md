---
brief: assay:assay:statusgen:21
title: Author skills consume the shared context handoff
why: "The spec and brief authoring skills are where plan changes are actually made. Until both call the same shipped context operation and emit the same handoff, a spec's obligations still reach brief authors as prose and missing amendments are found late, if at all."
wave: 3
depends: ["statusgen/20"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-13 by worker-desk (author-brief), non-CI architecture reconciliation
sources:
  - docs/proposals/non-ci-architecture/18-authoring-context-delivery.md — packages "Canonical skill integration" and "Release and adoption support"
  - docs/proposals/non-ci-architecture/15-regression-from-day-one.md — "End-to-end spec-to-brief handoff" and "Bootstrap and activation phases" cases
  - docs/proposals/non-ci-architecture/16-delivery-sequence.md — step 2 (skills must not call an unshipped interface)
  - docs/proposals/non-ci-architecture/20-amendment-manifest.md — step-2 owner row
  - "medici-finance/assay#2500 — maintainer ratification of the whole target @ 23f655f5d (approval activates nothing)"
  - freshness-checked 2026-10-13 @ a435badbd (origin/main)
exec-tier: strong
exec-tier-why: Changes the canonical authoring skills every adopter installs; wording must stay neutral, cite only shipped interfaces and keep the existing consumer and dependency checks intact.
domain: complicated
outcome: none
consumers:
  - "plugins/assay/skills/author-spec/SKILL.md, plugins/assay/skills/author-brief/SKILL.md, docs/context-handoff.md, statusgen/skillhandoff_test.go, .github/workflows/assay-statusgen.yml: follow-up statusgen/21"
  - "installed skill copies in adopter repositories: out-of-scope (publishing source does not update installed skills; adopters update pins and overlays themselves)"
id: 334a6ed9-4e28-453c-be19-39cd67997ace
---

# Brief 21 — Author skills consume the shared context handoff

## Context

files: `plugins/assay/skills/author-spec/SKILL.md`, `plugins/assay/skills/author-brief/SKILL.md`,
`docs/context-handoff.md` (planned) (compatibility and adoption section; the file exists after statusgen/20),
`statusgen/skillhandoff_test.go` (planned), `statusgen/testdata/contexthandoff/journeys/` (planned) (extend),
`.github/workflows/assay-statusgen.yml` (path filter for the two skill files),
`changelog/statusgen-21-author-skill-handoff.md` (planned).

facts (a435badbd, 2026-10-13):
- The canonical `author-spec` and `author-brief` skills live under `plugins/assay/skills/`; their
  consumer inspection, provenance and contract-ownership rules are the baseline and stay.
- `tools/skillslint` gates skill structure, conformance limits and unresolved house values
  (`cd tools/skillslint && go run . --root ../..`); skill bodies must stay neutral.
- statusgen/20 ships `statusgen context discover|recheck|check` and statusgen/19 the
  `context-handoff-v1` and `amendment-manifest-v1` schemas. Neither exists at a435badbd; the
  frontmatter `depends:` edge on statusgen/20 carries that ordering.
- Until a statusgen release carrying the verb is pinned by an adopter, the bounded manual
  bootstrap (a hand-assembled packet with explicit coverage) remains the documented path; the
  skills must say so instead of assuming the verb is installed.

layering: the skills give instructions; all checking lives in the shipped verb. The new test reads
the skill files and the built binary; it adds no checking logic of its own beyond citation
resolution.

single-point-of-failure: the citation test (row 2) that skills name only shipped verbs and the
shipped schema version — behind it, `tools/skillslint` independently gates skill structure and
house-value leaks (row 3).

## Read first

- `docs/proposals/non-ci-architecture/18-authoring-context-delivery.md`.
- `docs/streams/statusgen/brief-19-context-handoff-contracts.md`,
  `docs/streams/statusgen/brief-20-offline-context-resolver.md`.
- Both skill bodies in full.

## Ground rules

- Isolated branch; draft PR only. No merge, release or tag.
- Neutral wording only; no adopter names, paths or house values in skill bodies.
- Do not remove or weaken an existing skill check. Stop at implemented.

## Task

1. `author-spec`: when a spec changes planned work, emit a target record and run
   `statusgen context discover`; record actionable impact and amendment obligations in
   `context-handoff-v1`, not prose alone.
2. `author-brief`: consume the handoff; produce candidate brief patches plus an
   `amendment-manifest-v1` file, and run `statusgen context check` before opening the PR. An
   "amend" obligation without a patch is a failure, not a note. Keep the existing consumer,
   dependency and Verify rules.
3. Both skills: state the supported statusgen version that carries the verb and the manual
   bootstrap path when the verb is unavailable.
4. Add `statusgen/skillhandoff_test.go` (planned): a journey test over fixtures (spec target → discover →
   candidate amendment → check) and a citation test that resolves every `statusgen context`
   subcommand and schema version named in the two skill files against the shipped dispatch and
   schemas.
5. Extend `docs/context-handoff.md` (planned) with compatibility, supported-version and neutral migration
   notes; add the workflow path filter and the changelog fragment.

## Interface contract

Skills call only `statusgen context discover|recheck|check` and read or write only
`context-handoff-v1` / `amendment-manifest-v1`. A future schema version requires a skill update
and a citation-test change in the same PR.

## Verify

The named tests are deliverables of this brief and do not exist at a435badbd.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextSpecToBriefJourney$" . > "$out" && grep -F -- "--- PASS: TestContextSpecToBriefJourney " "$out")` | exit 0; named PASS; a fixture spec target flows through discover to a candidate amendment that `context check` accepts, and the same journey with one amendment removed fails although its impact row still says "amend"; mutation: let `check` accept an impact row as the amendment — the test must fail |
| 2 | check:ci +dereference +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestSkillsCiteShippedContextVerbs$" . > "$out" && grep -F -- "--- PASS: TestSkillsCiteShippedContextVerbs " "$out")` | exit 0; named PASS; every `statusgen context` subcommand and schema version cited in both skill files resolves to the shipped dispatch and schema files; mutation: cite a nonexistent subcommand in a skill fixture — the test must fail |
| 3 | check:ci | `cd tools/skillslint && go run . --root ../..` | exit 0; any non-zero exit (a violation or could-not-check) fails the row |
| 4 | check:ci | `grep -q "manual bootstrap" plugins/assay/skills/author-spec/SKILL.md && grep -q "manual bootstrap" plugins/assay/skills/author-brief/SKILL.md` | exit 0; both skill files name the manual bootstrap path |

## Pre-mortem

Likely wrong builds: skills that describe the handoff but let an impact row stand in for a patch
(row 1), skills citing a verb or version that never shipped (row 2), house values leaking into a
neutral body (row 3) and the bootstrap path silently dropped (row 4). Whether the instructions are
clear enough to follow stays a review judgment.

## Evidence

<!-- Independent verifier records command, exit, named result, subject revision, environment and date. No implementation evidence is asserted by this authoring change. -->

## Review

Gate: model (all four risk answers no). The reviewer reads both skill diffs for neutrality and for
any weakened existing check.
