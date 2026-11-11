---
brief: assay:assay:statusgen:21
title: Author skills consume the shared context handoff
why: "The spec and brief authoring skills are where plan changes are actually made. Until both call the same shipped context operation and emit the same handoff, a spec's obligations still reach brief authors as prose and missing amendments are found late, if at all."
wave: 4
depends: ["statusgen/20", "statusgen/23"]
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 3
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
  - "plugins/assay/skills/worker-desk/SKILL.md, plugins/assay/references/implementation-pickup.md, tools/desk/cmd/deskdispatch/prompt.go, tools/desk/cmd/deskdispatch/references, tools/desk/cmd/deskdispatch/pickup_test.go: follow-up statusgen/21"
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

## Product binding amendment — 13 October 2026

Consume statusgen/23's supported product-binding extension after /20; distinguish the
product-target and architecture revisions in both skills. Existing unbound briefs remain
explicitly unbound, and a proposed target never invalidates approved work. Carry the original
selection revision and checkpoint disposition into amendments. The manual bootstrap records
unsupported bindings and coverage; it must not pretend the parser already supports them.
This is an amendment to an unfinished brief, not a change to statusgen/19's merged contract or Evidence.

## Implementation pickup amendment — 18 October 2026

Existing manual pickup instructions are now in the brief spec, author skills and both
worker prompt kits. This brief replaces their manual-only integration with the same
shipped context operation and typed receipt, not a second checker. Include worker-desk,
`tools/desk/cmd/deskdispatch/prompt.go`, both worker kits and planned `pickup_test.go` in
files/delivery. Effort becomes L for cross-module dispatch integration. No installed
cache is edited. Preserve scope and active-work acknowledgement on material amendments.

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

6. At dispatch and implementation pickup/resume, bind the selected architecture and
   applicable guidelines to /20's recheck result. Emit the receipt/assessment into both
   worker kits; permit compatible/unrelated changes, require reconciled amendments for
   material conflicts and report missing mandatory evidence as a scoped hold before
   launch. Preserve the explicit bounded manual bootstrap for unsupported installs;
   never silently admit a receipt/schema version the dispatcher cannot validate.
7. Update worker-desk and the one pickup reference, keeping author/worker wording
   pointed at the same procedure. Qualify source-bound instructions, actual dispatch
   behavior and release/pin support; prose alone is not a runtime gate. Extend existing
   CI cross-module/path coverage to the deskdispatch consumers through the workflow owner.

## Interface contract

Skills call only `statusgen context discover|recheck|check` and read or write only
the shipped `context-handoff` / `amendment-manifest` versions, including /23’s product-binding extension. A future schema version requires a skill update
and a citation-test change in the same PR.

## Verify

The named tests are deliverables of this brief and do not exist at a435badbd.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextSpecToBriefJourney$" . > "$out" && grep -F -- "--- PASS: TestContextSpecToBriefJourney " "$out")` | exit 0; named PASS; a fixture spec target flows through discover to a candidate amendment that `context check` accepts, and the same journey with one amendment removed fails although its impact row still says "amend"; mutation: let `check` accept an impact row as the amendment — the test must fail |
| 2 | check:ci +dereference +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestSkillsCiteShippedContextVerbs$" . > "$out" && grep -F -- "--- PASS: TestSkillsCiteShippedContextVerbs " "$out")` | exit 0; named PASS; every `statusgen context` subcommand and schema version cited in both skill files resolves to the shipped dispatch and schema files; mutation: cite a nonexistent subcommand in a skill fixture — the test must fail |
| 3 | check:ci | `cd tools/skillslint && go run . --root ../..` | exit 0; any non-zero exit (a violation or could-not-check) fails the row |
| 4 | check:ci | `grep -q "manual bootstrap" plugins/assay/skills/author-spec/SKILL.md && grep -q "manual bootstrap" plugins/assay/skills/author-brief/SKILL.md` | exit 0; both skill files name the manual bootstrap path |
| 5 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestSkillsProductTargetHandoff$" . > "$check_output" && grep -F -- "--- PASS: TestSkillsProductTargetHandoff " "$check_output")` | exit 0; named PASS; both skill journeys preserve separate product/architecture bindings and original context; missing extension invokes explicit bootstrap; mutation: silently drop outcome revision |
| 6 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPickupAdmission$' ./cmd/deskdispatch > "$out" && grep -q '^--- PASS: TestPickupAdmission ' "$out")` | exit 0; actual dispatch path admits compatible/unrelated changes, holds material conflict or unavailable mandatory input with zero child launch, carries receipt into both kits, and identifies unsupported-version bootstrap; mutation: omit recheck before launch and require the refused fixture to fail |

## Pre-mortem

Likely wrong builds: skills that describe the handoff but let an impact row stand in for a patch
(row 1), skills citing a verb or version that never shipped (row 2), house values leaking into a
neutral body (row 3) and the bootstrap path silently dropped (row 4). Whether the instructions are
clear enough to follow stays a review judgment.

## Evidence

<!-- Independent verifier records command, exit, named result, subject revision, environment and date. No implementation evidence is asserted by this authoring change. -->
### Non-implementer verifier run — 2026-11-11 claude-opus-5-5 verifier at b856f281f758 (rows 1–6 check:ci in a closed container bracket)

Fresh non-implementer pass, admitted by `deskdispatch --check-verifier` (rc=0, binding run 74af40c5f9eb, source b856f281f758, model claude-opus-5-5, tier strong) before any row, and again after the run (rc=0, same binding). Earlier runs of this item today were invalidated by the desk-tools upgrade (medici-finance/assay#6125); none of their output is reused. Detached home at merged main b856f281f758; host darwin/arm64; installed statusgen and deskdispatch v1.0.82 (no locally built binary in the witnessed run). Delivery is PR #6084 (merge d3a334a1072a, first parent 57544d5a8b76); the delivered paths are byte-unchanged from that merge to this main.

#### Rows 1 to 6 — container execution witness (closed bracket)

These are the closed witness file's bytes as the three bracket steps wrote them, unedited. Host open at 20:26:16Z with a fresh module cache (digest sha256:7f8fa9800956), container run 20:26:41Z to 20:27:15Z over all six rows, host close at 20:27:35Z (rc=0). `statusgen verifyrun --bracket-digest` on the closed file reported `verdict=match` with `span-lines=tree-digest:1,container-image:1,attestation:1`.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextSpecToBriefJourney$" . > "$out" && grep -F -- "--- PASS: TestContextSpecToBriefJourney " "$out")` | pass exit=0 sandbox=container-netns | sha256:92e75395ed35 | 2026-11-11 | assay-verifier-app[bot] @ b856f281f758 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestSkillsCiteShippedContextVerbs$" . > "$out" && grep -F -- "--- PASS: TestSkillsCiteShippedContextVerbs " "$out")` | pass exit=0 sandbox=container-netns | sha256:754556aa10a9 | 2026-11-11 | assay-verifier-app[bot] @ b856f281f758 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/skillslint && go run . --root ../..` | pass exit=0 sandbox=container-netns | sha256:b0dae4317b4a | 2026-11-11 | assay-verifier-app[bot] @ b856f281f758 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -q "manual bootstrap" plugins/assay/skills/author-spec/SKILL.md && grep -q "manual bootstrap" plugins/assay/skills/author-brief/SKILL.md` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-11 | assay-verifier-app[bot] @ b856f281f758 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestSkillsProductTargetHandoff$" . > "$check_output" && grep -F -- "--- PASS: TestSkillsProductTargetHandoff " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:80515a1b7200 | 2026-11-11 | assay-verifier-app[bot] @ b856f281f758 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPickupAdmission$' ./cmd/deskdispatch > "$out" && grep -q '^--- PASS: TestPickupAdmission ' "$out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-11 | assay-verifier-app[bot] @ b856f281f758 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:011afe00d9b059b9d46ddfd98ea5c561ba761ec50bb9275d87a28f2ab403dde9
Container-Image: sha256:43db6da8cd159b2c822af62bb859f1a305637c066ea1a750df16dfd5d7b99563
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6186 run=74af40c5f9eb875e7c92ac129a2e6930453610bf6443e21a3e4f84d54e9acc32 source=b856f281f758fd9517595b9d6ca496b3dfbdf734 brief=docs/streams/statusgen/brief-21-author-skill-handoff.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=74af40c5f9eb875e7c92ac129a2e6930453610bf6443e21a3e4f84d54e9acc32 source=b856f281f758fd9517595b9d6ca496b3dfbdf734 tree=sha256:011afe00d9b059b9d46ddfd98ea5c561ba761ec50bb9275d87a28f2ab403dde9 witness=sha256:3358e3d0ec72276baa6d506dd9c915eec9fc5e7f9a0068f0527e1ab603bfb23f opened=2026-11-11T20:26:16Z closed=2026-11-11T20:27:35Z before=receipt after=check-verifier

Execution envelope: image ghcr.io/medici-finance/verify-runner:v1.0.82@sha256:43db6da8cd15 (the v1.0.82 release's pinned runner; its verify-runner-image.txt matches the release checksums.txt entry), linux/arm64 native, statusgen v1.0.82, go1.27.3. Container flags: network none, read-only root filesystem, all capabilities dropped, no-new-privileges, non-root uid matching the host user, a tmpfs scratch mount, a throwaway HOME inside that mount, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly. Mounts: the home and its git directory read-only at their host paths; the receipt read-only; the module cache read-only; a writable output directory for the witness. No forge credential in the run environment. After the close, `git status --porcelain --ignored` in the home was empty.

Credit: `--bracket-digest --credit --source` on the unlanded witness printed `credit=could-not-check` (the witness file is not yet inside a git work tree), so this attempt is reported as uncredited, credit-unchecked, until the landed lines can be blamed.

- **Row 1** (2026-11-11): expected the journey test's named PASS; observed exit 0, the grep matched `--- PASS: TestContextSpecToBriefJourney `.
- **Row 2** (2026-11-11): expected the citation test's named PASS; observed exit 0, matched `--- PASS: TestSkillsCiteShippedContextVerbs `.
- **Row 3** (2026-11-11): expected skillslint exit 0; observed exit 0.
- **Row 4** (2026-11-11): expected both skill files to name the manual bootstrap; observed exit 0 (author-spec SKILL.md lines 26 and 59, author-brief SKILL.md lines 53 and 89).
- **Row 5** (2026-11-11): expected the product-target test's named PASS; observed exit 0, matched `--- PASS: TestSkillsProductTargetHandoff `.
- **Row 6** (2026-11-11): expected the pickup test's named PASS; observed exit 0, matched `--- PASS: TestPickupAdmission `.

Supplemental mutation probes (no acceptance credit; a throwaway export of this main outside the home, real Go caches, throwaway HOME, host go1.27.3, 20:28:31Z to 20:28:58Z; each mutation restored, with the export clean, before the next; each ran the row's own command):
- Row 1, `check` lets an impact row stand in for the amendment (the missing-obligation refusal in the contexthandoff rules.go, line 144, made to `continue`): exit 1, `skillhandoff_test.go:160: an amend without a patch: exit 0, want 1 naming the missing obligation`.
- Row 2, a nonexistent verb (`statusgen context reconcile`) appended to the author-spec skill: exit 1, `cites what does not ship: [verb reconcile]`.
- Row 5, the outcome revision dropped from a bound product binding (the contexthandoff product.go, line 445, no longer sets Selected): exit 1, `skillhandoff_test.go:253: discover: exit 1`. The dropped revision is caught as an invalid receipt, not silently carried.
- Row 6, the pickup recheck omitted before launch (the deskdispatch dispatch.go, line 242, no longer calls pickupAdmit): exit 1, `pickup_test.go:147: the dispatch was not stopped: err=<nil>`, plus the missing receipt section in both kits. An unmutated baseline of row 6 in the same export exited 0.

All four mutation rows discriminate.

Note, not a Verify obligation: Task 5's workflow path filter is moot as delivered. The statusgen-board workflow has no `paths:` filter on pull requests, so it already runs on every PR, and the delivery registers both new tests in the cross-module CI registry (citrigger_test.go) on the ci.yml floor jobs, with the two skill files among the paths each test reads.

Fail-first: not applicable. All four risk answers are no and the brief is not irreversible, so no row carries a fail-first obligation.

Risk-bearing value. The frontmatter risk answers are all present and all no. Enumeration over the delivery diff (57544d5a8b76..d3a334a1072a: deskdispatch pickup.go, dispatch.go, cli.go and prompt.go, both skill files, the new test file), plus the Deliverables:
- `pickupMaxReceipt = 8 << 20` @ tools/desk/cmd/deskdispatch/pickup.go:37 (its comment says it matches the `statusgen context` input bound)
- `pickupSchemas = [context-handoff-v1, context-handoff-v2, context-handoff-v3]` @ tools/desk/cmd/deskdispatch/pickup.go:40 (resolved at statusgen/contexthandoff/records.go:6-8)
- supported-version citation `v1.0.55` @ plugins/assay/skills/author-spec/SKILL.md:51 and plugins/assay/skills/author-brief/SKILL.md:81
- supported-version citation `v1.0.63` (context-handoff-v3) @ plugins/assay/skills/author-spec/SKILL.md:52 and plugins/assay/skills/author-brief/SKILL.md:82
- `pickupHeldMarker = "dispatch HELD on context pickup"` @ tools/desk/cmd/deskdispatch/pickup.go:34 (display string)
- test-only literals: the bogus-citation expectation of 3 findings and the required-word lists in statusgen/skillhandoff_test.go

Ranked: the schema admission set comes first, because it decides which receipts deskdispatch admits and which it refuses. The two version citations come next, because they tell adopters which pin carries the verb. The receipt size bound is a fail-closed hold that only delays a dispatch. The display string and the test literals rank last. Every entry can be undone by an edit and a release.

RISK-VALUE: DERIVED — pickupSchemas = [context-handoff-v1, context-handoff-v2, context-handoff-v3] @ tools/desk/cmd/deskdispatch/pickup.go:40 — exactly the schema enum the shipped decoder accepts (statusgen/contexthandoff/decode.go:410 enumerates the same three constants); any other version returns the schema-version refusal that names the manual bootstrap, so nothing is admitted unchecked.
RISK-VALUE: DERIVED — supported version = v1.0.55 @ plugins/assay/skills/author-spec/SKILL.md:51 (and author-brief SKILL.md:81) — CHANGELOG.md's v1.0.55 entry is the first to ship `statusgen context discover|recheck|check` with context-handoff-v1; no earlier release section names `statusgen context`.
RISK-VALUE: DERIVED — supported version = v1.0.63 @ plugins/assay/skills/author-spec/SKILL.md:52 (and author-brief SKILL.md:82) — CHANGELOG.md's v1.0.63 entry is the first to name context-handoff-v3 (outcome-revision binding); no earlier release section does.
RISK-VALUE: NAMED, NOT DERIVED — pickupMaxReceipt = 8 << 20 @ tools/desk/cmd/deskdispatch/pickup.go:37 — its comment claims parity with `statusgen context`'s own input bound, but that bound is `ctxMaxInputBytes = 16 << 20` @ statusgen/context.go:46. A receipt between 8 MiB and 16 MiB is valid for the verb but held by deskdispatch. The failure direction is safe (a hold, never an unchecked admit), but the stated derivation does not hold. The owner needs to change either the value or the comment. That is a follow-up, not a Verify obligation.

**VERIFY: PASS** — rows_passed 6 / rows_total 6 at b856f281f758. All six check:ci rows passed in the closed container-netns bracket (verdict=match), and the supplemental probes show all four mutation obligations discriminate. Gate is model and all four risk answers are no. One risk value is named, not derived: the pickup receipt bound, which fails safe. It is routed as a follow-up.

## Review

Gate: model (all four risk answers no). The reviewer reads both skill diffs for neutrality and for
any weakened existing check.
