---
brief: assay:assay:tool-refactoring:01
title: "Bind operation subjects to explicit tracking, delivery and evidence repositories"
why: "Tracking decisions, delivery candidates and Evidence destinations are different repositories in cross-repository work. Re-deriving those coordinates per command repeatedly sends reads, prompts or writes to the wrong home."
wave: 0
depends: []
unblocks: ["tool-refactoring/02", "tool-refactoring/08", "tool-refactoring/09", "tool-refactoring/10"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: 7a0c0968-a001-4aab-acea-681909b3034f
outcome: none
authored: 2026-11-12 by author-brief session
sources:
  - "docs/streams/tool-refactoring/spec.md — draft scope; not implementation or activation approval"
  - "freshness-checked 2026-11-12 @ 680102a0877755feab9ef85dc4e1d3f52a137a54; source inspection and bounded issue/PR snapshots; not a reproduced production failure"
  - "medici-finance/assay#6274 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5525 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5500 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5999 — related class evidence; resolution not asserted"
  - "milestone-support/01 — retained owning work; see existing-work.md"
  - "milestone-support/08 — retained owning work; see existing-work.md"
  - "graph-execution/28 — retained owning work; see existing-work.md"
gate-why: "A wrong tracking, delivery or evidence coordinate can direct a credentialed adapter at another repository. Human acceptance confirms exact subject binding and independent effect authorization survive the consolidation."
decision-trigger: start
decision-issue: 7087
ruling: "https://github.com/medici-finance/assay/issues/7087#issuecomment-7065937224"
design: DR-tool-refactoring-01
exec-tier: strong
exec-tier-why: "Cross-component class refactoring and negative controls; one local happy-path test cannot prove that every production caller migrated."
domain: complicated
consumers:
  - "statusgen/domain/operation.go: fixed-here (the pure operation subject and its projections)"
  - "statusgen/domain/operation_test.go: fixed-here (fixture matrix and class guard)"
  - "tools/desk/internal/deskkit/workselection.go: fixed-here (OperationBrief copies the selected subject)"
  - "tools/desk/internal/deskkit/operationcontext.go: fixed-here (the observation adapter)"
  - "tools/desk/internal/deskkit/operationcontext_test.go: fixed-here (adapter flow and census)"
---

# Brief 01 — Bind operation subjects to explicit tracking, delivery and evidence repositories

## Context

files:
- statusgen/domain/operation.go
- statusgen/domain/operation_test.go
- tools/desk/internal/deskkit/workselection.go
- tools/desk/internal/deskkit/operationcontext.go
- tools/desk/internal/deskkit/operationcontext_test.go
- docs/contracts.md
- docs/library-first.md
- tools/desk/README.md
- changelog/feat-assay--tool-refactoring--01.md

facts (2026-11-12, source `680102a08777`):
- Planned paths above identify new Go/test deliverables and existing starting seams; no out-of-repository files are changed.
- Class: bind operation subjects to explicit tracking, delivery and evidence repositories; the issue cases in sources nominate this mechanism, not a claim that every old report still reproduces.
- Existing owner: statusgen/domain composed with statusgen/workref; deskkit remains the observation adapter.
- Required reading: [scope](spec.md), [retained work and active PRs](existing-work.md), `docs/library-first.md`, `docs/contracts.md`, and the source paths above. Source inspection precedes any implementation effect.
- All dependency and caller changes are this brief's bounded scope. Retained work: milestone-support/01, milestone-support/08, graph-execution/28. Completed Evidence and dated sources stay unchanged.

layering: keep pure syntax/state/judgment in its named existing library owner and source, process, credential, forge and persistence effects in adapters. Module boundaries are real: desk-tools may import public statusgen subpackages; statusgen cannot import internal deskkit. No new daemon, effect journal, generic bypass or public command is needed.

single-point-of-failure: a caller can bypass the shared decision — the class guard must detect a second planted caller; independent current-state, custody and conditional-write/process checks still refuse at their own effect boundary. Pure-library tests prove no effect authority.

design-fit:
  owner: statusgen/domain composed with statusgen/workref; deskkit remains the observation adapter
  contract: S-verification-domain; S-context-handoff
  retires: ["the duplicated projections or inference paths explicitly migrated in Task; inventory and prove deletion before claiming class retirement"]
  weight: "verbs +0; public flags +0; new bypasses +0; duplicate rule bodies decrease after migration; adapter refusal reasons consolidate without dropping independent checks"
  why-add: "Reuse existing owners and profiles. Add only the typed internal seam/tests required below; record actual source and rule-text deltas in delivery."

## Human decision

This migration changes a control-bearing shared meaning or publication boundary.
The implementation must preserve exact subject binding, current permission and
source checks, and honest unavailable outcomes at independent effect boundaries.

Options:
1. Approve the bounded refactor after its independent class and caller controls pass.
2. Keep the existing behavior and request a narrower migration.

Default if no answer: none — blocks until answered.

## Ground rules

- Isolated branch and draft PR; no merge, release, live infrastructure call or control weakening. Stop at implemented.
- At pickup/resume, record Architecture pickup against the current target, contracts, actual APIs and affected work; adapt only within scope. An active material amendment needs its owner checkpoint.
- Refresh issue/PR status and reuse landed repairs before changing code. A scope refusal is a stop.

## Task

Workspace identity, request idempotency, allocation generations and scratch references already belong to `workspace/contract.go` and `spec/cell-workspaces-v1.md`. An operation subject only references those supplied observations; it must not redefine allocation identity, state or path ownership. The current source exposes validation/ports, not proof that every allocator effect is shipped.

1. Extend the existing qualified work/candidate records with an immutable, supplied-observation operation subject. Identify the declaring brief revision, tracking source, delivery candidate, review target, decision target and evidence destination separately. Reuse WorkSelection, BriefSubject and workref parsing; do not mint another work or claim identity.
2. Define named dispatch, review, PR-publication and evidence-publication projections. Missing coordinates stay unknown; one repository cannot silently substitute for another. A source path is a locator, never authorization. Preserve legacy identifiers and exact-version selection through existing profiles.
3. Build the same-repository and two-repository fixture matrix, including review of a delivery PR, a tracking decision and outcome publication. Keep credentials, alias/file/forge reads and custody outside the pure owner. Return structured problems; no new public wire schema is required.
4. Publish the consumer census for briefs 02, 09 and 10; update the existing owner rows rather than add a second registry. Preserve milestone-support/08 subject/claim work and its capability negotiation.
5. Update the named API/CLI reference and existing semantic-owner row; add the changelog fragment. Keep consumer routing deferred until delivery actually edits each site. Publish the class census and retired-callsite diff.

## Class guard

Enumerate all operation-subject constructors and every dispatch/review/PR/Evidence consumer in the declared files. A second consumer with a wrong tracking/effect coordinate must be rejected; a declared locator or alias cannot override the exact observed subject. Preserve the source-pinned regression cases, show a pre-fix red control and retain the planted positive fixture.

## Interface contract

statusgen/domain, composed with statusgen/workref, owns the operation-subject contract. Deskkit remains its observation adapter. Inputs are explicit observations; outputs are typed decisions/records with unavailable states. Internal refactoring does not change a public schema, numeric exit contract, gate policy or custody boundary without its existing owner approval. Source-only success is not released/adopted capability.

## Verify

Row 4 runs only in a clean isolated checkout of the independently resolved implementation delivery D, with its protected delivery binding checked by the existing owner. There HEAD is D and HEAD^ is its first parent. Do not select current integration HEAD as a convenient substitute. If D or its binding cannot be established, record could-not-check. Other rows keep their admitted source and recorded witness identity.

The named tests below are implementation deliverables, not tests claimed to exist today. Each row requires the named `--- PASS:` line as well as exit 0, so a vacuous selector cannot pass. Class controls plant a second defect site outside the reported instance and prove the guard catches it; retain that positive control. Record the pre-fix red and post-fix green.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && GOWORK=off go test -v ./domain -run '^TestOperationSubjectMatrix$' -count=1` | exit 0; output contains "--- PASS: TestOperationSubjectMatrix"; subject matrix | check:ci +dereference |
| 2 | `cd statusgen && GOWORK=off go test -v ./domain -run '^TestOperationSubjectClassGuard$' -count=1` | exit 0; output contains "--- PASS: TestOperationSubjectClassGuard"; class guard and planted wrong-repository caller | check:ci +mutation +neighbour |
| 3 | `cd tools/desk && GOWORK=off go test -v ./internal/deskkit -run '^TestOperationContextAdapterFlow$' -count=1` | exit 0; output contains "--- PASS: TestOperationContextAdapterFlow"; real observation adapter flow | check:ci +flow |
| 4 | `statusgen --root . --consumers --brief tool-refactoring/01 --delivery "$(git rev-parse HEAD)" --base "$(git rev-parse HEAD^)"` | exit 0; zero disproved and zero unchecked consumer routes for the delivered brief | check:ci +flow |

## Definition of Done

Every Task path has an actual caller or documented retention reason, consumer flows and planted controls pass, obsolete duplicate bodies are removed, and docs/reference plus changelog explain the surviving contract. An independent verifier fills Evidence; this brief does not self-award acceptance. No change to historical Verify or Evidence is included.

## Evidence

<!-- Independent verification appends actual commands, exits, output/digests, date and runner after implementation. Empty at authoring. -->

Verifier pass, 2026-11-21, by a non-implementer (model claude-opus-5-5, tier strong) at merged main 995d1308d7f778d2f71221ced851a6c2c4c6158f. This is the brief's first verification. Delivering change: #7092, merge commit 7ee2c936c0ca36727de3aaa8c7aef0cc884ccb11 (on the first-parent chain of this source), first parent 24613d8d998c6cee5482da529ad74f2e25c3c320. None of the 13 files that merge delivered differs between it and this source. The human decision (option 1, approve the bounded refactor) is recorded at https://github.com/medici-finance/assay/issues/7087#issuecomment-7065937224. Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief before any row (run e9f53cef39d7, attestation #7386, tool v1.0.90). The Verify table re-derived with 4 rows and 0 author-added (deskpathguard rederive exit 0, base 995d1308d7f7). The Verify section is byte-identical at the delivery's first parent, at the delivery and at this source.

This is a DISPATCH-FOR-EVIDENCE pass. The brief carries risk sensitive-data: yes, so the desk does not flip its status. This entry is evidence only, and the human's merge of the Evidence PR that carries it is the flip.

Rows 1-4 (check:ci) ran in one closed container bracket. Image: verify-runner v1.0.90, pinned by the image index digest the house image lock names (Container-Image below), run on the host's local Docker engine with --network none, --read-only, tmpfs scratch, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly, KUBECONFIG=/dev/null and a fresh module cache filled by the open step (Module-Cache below). The home, the receipt, the base history pack, the roster copy and the module cache were mounted read-only; only the witness output directory and the tmpfs were writable, both outside the home. No forge credential entered the container. Every row is risk-bearing because the brief answers a risk question yes, so the run used --fail-first with --delivering-commit 7ee2c936c0ca and --base 24613d8d998c. The close step re-admitted the run and --bracket-digest reports verdict=match (bracket=L11 header=L1).

Rows 1-3 are classed could-not-run. The witness records each as pass exit=0 at head with base=24613d8d998c green rc=0, and the container summary printed "non-discriminating — green at base 24613d8d998c and pass at head" for each. That base reading is wrong. At 24613d8d998c none of the three named tests exists: statusgen/domain/operation_test.go, tools/desk/internal/deskkit/operationcontext.go and tools/desk/internal/deskkit/operationcontext_test.go are all absent there. So go test -run selects nothing and exits 0 with "testing: warning: no tests to run" and no "--- PASS" line (S1-S3 below). Each row's Expect requires output containing "--- PASS: " plus the named test, and that base output does not contain it, so by its own Expect each row is red at base. verifyrun's Expect reader decides an exit code and the "output is" convention, not "output contains". Both the head pass and the base green are therefore exit-status-only readings, and the fail-first property was never observed by the instrument. This is the Expect-reader defect class, #1702. It is classed could-not-run per reviewer-lane practice (https://github.com/medici-finance/assay/pull/7228#pullrequestreview-5979645423 and https://github.com/medici-finance/assay/pull/7230#pullrequestreview-5979708866). Plainly: the witness records these three rows pass at head and non-discriminating at base, and verifyrun --check records the result given after the table. On the host at this source, S4-S6 show each named test's "--- PASS" line.

Row 4 is classed could-not-run. Plainly: the witness records it fail exit=2, with base=24613d8d998c red rc=2. The property was never observed. Inside the container git cannot read this home, because its .git is a linked-worktree gitfile that points at a git directory outside the mount. statusgen printed "--consumers: COULD-NOT-CHECK: git diff --name-only ...HEAD: exit status 128: fatal: gitfile does not point to a valid repository" and "no diff, so no routing claim was corroborated", then exited 2, at head and at base. The row's preamble also requires a clean isolated checkout whose HEAD is the delivery 7ee2c936c0ca. This bracket's home is merged main 995d1308d7f7, so a readable git would still not have run the row as written. The row routes to #3481 under the consumers ruling medici-finance/assay#4560 option (a): #3481 lands before any row amendment. S7 ran the consumers gate on the host pinned to this brief's own delivery; it is supplemental and carries no acceptance credit.

Attempts. A first container try at 2026-11-21T19:15:53Z ran zero rows: docker refused the platform flag with "cannot overwrite digest" for the pinned image and exited 125. The flag was removed from the container script and nothing else changed. The one attempt that ran rows started 2026-11-21T19:16:22Z, finished 2026-11-21T19:17:45Z, ran all 4 rows and is the one recorded below.

Rows S1-S8 are supplemental and carry no acceptance credit. S1-S3 ran each row's command against a tracked-file snapshot of 24613d8d998c outside the home. S4-S7 ran on the host from this home's root. S8 ran the brief's mutation manifest from a scratch directory outside the home; the manifest copies the tree before planting. The home was clean after each.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| S1 | row 1's go test command at the 24613d8d998c snapshot | the named test is absent, so no "--- PASS" line | exit 0; "testing: warning: no tests to run"; "ok github.com/medici-finance/assay/statusgen/domain [no tests to run]"; zero "--- PASS" lines (no supplemental credit) | 2026-11-21 | assay-verifier-app[bot] verifier agent, model claude-opus-5-5 (on-behalf-of human:ian) |
| S2 | row 2's go test command at the 24613d8d998c snapshot | the named test is absent, so no "--- PASS" line | exit 0; "testing: warning: no tests to run"; "[no tests to run]"; zero "--- PASS" lines (no supplemental credit) | 2026-11-21 | assay-verifier-app[bot] verifier agent, model claude-opus-5-5 (on-behalf-of human:ian) |
| S3 | row 3's go test command at the 24613d8d998c snapshot | the named test is absent, so no "--- PASS" line | exit 0; "testing: warning: no tests to run"; "ok github.com/medici-finance/assay/tools/desk/internal/deskkit [no tests to run]"; zero "--- PASS" lines (no supplemental credit) | 2026-11-21 | assay-verifier-app[bot] verifier agent, model claude-opus-5-5 (on-behalf-of human:ian) |
| S4 | row 1's go test command at 995d1308d7f7 | exit 0 and the named PASS line | exit 0; "--- PASS: TestOperationSubjectMatrix (0.00s)" (no supplemental credit) | 2026-11-21 | assay-verifier-app[bot] verifier agent, model claude-opus-5-5 (on-behalf-of human:ian) |
| S5 | row 2's go test command at 995d1308d7f7 | exit 0 and the named PASS line | exit 0; "--- PASS: TestOperationSubjectClassGuard (0.01s)" (no supplemental credit) | 2026-11-21 | assay-verifier-app[bot] verifier agent, model claude-opus-5-5 (on-behalf-of human:ian) |
| S6 | row 3's go test command at 995d1308d7f7 | exit 0 and the named PASS line | exit 0; "--- PASS: TestOperationContextAdapterFlow (0.55s)" (no supplemental credit) | 2026-11-21 | assay-verifier-app[bot] verifier agent, model claude-opus-5-5 (on-behalf-of human:ian) |
| S7 | statusgen --root . --consumers --brief tool-refactoring/01 --delivery 7ee2c936c0ca36727de3aaa8c7aef0cc884ccb11 --base 24613d8d998c6cee5482da529ad74f2e25c3c320 | zero disproved and zero unchecked routes | exit 0; "summary: 5 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing" over operation.go, operation_test.go, workselection.go, operationcontext.go and operationcontext_test.go; it also printed "PRODUCER-DELIVERY: COULD-NOT-CHECK" (producer identity is not evaluated) (no supplemental credit) | 2026-11-21 | assay-verifier-app[bot] verifier agent, model claude-opus-5-5 (on-behalf-of human:ian) |
| S8 | python3 -I statusgen/domain/mutate.py | every manifest plant turns a named test red | exit 0; "PASS unmutated snapshot", then 37 of 37 cases printed KILLED, among them six operation cases: operation-decision-in-delivery, operation-path-locator, operation-alias-override and operation-second-consumer against TestOperationSubjectClassGuard, and operation-decision-fallback and operation-shared-candidate against TestOperationSubjectMatrix (no supplemental credit) | 2026-11-21 | assay-verifier-app[bot] verifier agent, model claude-opus-5-5 (on-behalf-of human:ian) |

Risk-bearing values. Enumerated over every non-test file in the diff of 24613d8d998c..7ee2c936c0ca (statusgen/domain/operation.go, tools/desk/internal/deskkit/operationcontext.go, tools/desk/internal/deskkit/workselection.go, statusgen/domain/mutate.py; the rest is docs, the changelog fragment and the decision record). workselection.go adds a field copy and no literal. Literals found:

- operationUUID = ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$ @ statusgen/domain/operation.go:93
- sameRepo = strings.EqualFold (case-insensitive repository compare) @ statusgen/domain/operation.go:96
- locator refusal set = absolute path, a "../" prefix, "..", a non-clean path, and any of backslash, tab, CR, LF, space or colon @ statusgen/domain/operation.go:99
- brief Version 0 refused @ statusgen/domain/operation.go:107
- work Revision 0 means unpinned, any other value must equal the brief version @ statusgen/domain/operation.go:115
- ReviewDelivery = "delivery", ReviewAuthoring = "authoring" @ statusgen/domain/operation.go:47-48
- decision target and evidence destination both bound to the tracking repository @ tools/desk/internal/deskkit/operationcontext.go:51-52
- refusal state "could-not-check" maps to Unverifiable (exit 6), every other refusal to Refused (exit 5) @ tools/desk/internal/deskkit/operationcontext.go:87
- mutation harness timeouts -timeout=30s and timeout=60 @ statusgen/domain/mutate.py:116,118,138

The repository and commit grammars this file applies (repoRE and objectRE) are pre-existing and not introduced here. No value is irreversible; each is undone by an edit and a release. A wrong one mis-binds, refuses or admits an operation subject. The review-kind spellings and the harness timeouts are reversible naming or operational bounds and rank last. Ranked top entries:

- RISK-VALUE: DERIVED — decision target and evidence destination = tracking repository @ tools/desk/internal/deskkit/operationcontext.go:51-52 — Task 1 requires the decision target and evidence destination to be identified separately from the delivery candidate, and the Class guard requires a second consumer with a wrong tracking or effect coordinate to be rejected; S8's operation-decision-in-delivery and operation-decision-fallback plants are killed.
- RISK-VALUE: DERIVED — sameRepo = strings.EqualFold @ statusgen/domain/operation.go:96 — GitHub owner and repository names are case-insensitive, so an exact-case compare would refuse one repository as two; the Task's rule that one repository cannot silently substitute for another is held by the per-coordinate checks that S8's operation-second-consumer plant exercises.
- RISK-VALUE: DERIVED — locator refusal set @ statusgen/domain/operation.go:99 — Task 2 says a source path is a locator, never authorization; the set refuses every path that could leave the tree or carry a repository or ref separator, and S8's operation-path-locator plant is killed.
- RISK-VALUE: DERIVED — could-not-check to Unverifiable (exit 6), else Refused (exit 5) @ tools/desk/internal/deskkit/operationcontext.go:87 — the Interface contract forbids changing a numeric exit contract, and these are the existing ExitRefused = 5 and ExitUnverifiable = 6 in tools/desk/internal/deskkit/exitcodes.go:51,55, keeping the unavailable outcome distinct from a refusal.
- RISK-VALUE: DERIVED — operationUUID @ statusgen/domain/operation.go:93 — Task 1 reuses the existing BriefSubject identity rather than minting another; the pattern is the canonical lower-case UUID form, and a non-matching identity yields the unknown state brief-revision-unavailable rather than a refusal.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `cd statusgen && GOWORK=off go test -v ./domain -run '^TestOperationSubjectMatrix$' -count=1` | pass exit=0 sandbox=container-netns | sha256:9aaac2ea1b74 | 2026-11-21 | assay-verifier-app[bot] @ 995d1308d7f7 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=24613d8d998c green rc=0 · head=995d1308d7f7 · delivering=7ee2c936c0ca |
| 2 | `cd statusgen && GOWORK=off go test -v ./domain -run '^TestOperationSubjectClassGuard$' -count=1` | pass exit=0 sandbox=container-netns | sha256:0f7e8eec377b | 2026-11-21 | assay-verifier-app[bot] @ 995d1308d7f7 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=24613d8d998c green rc=0 · head=995d1308d7f7 · delivering=7ee2c936c0ca |
| 3 | `cd tools/desk && GOWORK=off go test -v ./internal/deskkit -run '^TestOperationContextAdapterFlow$' -count=1` | pass exit=0 sandbox=container-netns | sha256:985a3a8b491e | 2026-11-21 | assay-verifier-app[bot] @ 995d1308d7f7 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=24613d8d998c green rc=0 · head=995d1308d7f7 · delivering=7ee2c936c0ca |
| 4 | `statusgen --root . --consumers --brief tool-refactoring/01 --delivery "$(git rev-parse HEAD)" --base "$(git rev-parse HEAD^)"` | fail exit=2 sandbox=container-netns | sha256:f84dba80ef63 | 2026-11-21 | assay-verifier-app[bot] @ 995d1308d7f7 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=24613d8d998c red rc=2 · head=995d1308d7f7 · delivering=7ee2c936c0ca |
Tree-Digest: sha256:66d83da105e38041f45a39e5d75dc9234f0f6358f0accbb3cc85ce2681a6312a
Container-Image: sha256:4e8e3c80a075c552de0966c886784c6020153c27e624f8a1ce988aa3b0590217
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7386 run=e9f53cef39d775b0db23d768828f41ea147a10d4cf4acd06d0b08dd0baf1f987 source=995d1308d7f778d2f71221ced851a6c2c4c6158f brief=docs/streams/tool-refactoring/brief-01-bind-operation-subjects-to-explicit-tracking-delivery-and-evidence-repositories.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=e9f53cef39d775b0db23d768828f41ea147a10d4cf4acd06d0b08dd0baf1f987 source=995d1308d7f778d2f71221ced851a6c2c4c6158f tree=sha256:66d83da105e38041f45a39e5d75dc9234f0f6358f0accbb3cc85ce2681a6312a witness=sha256:bf825e208757cdb738bd993a8064cb36bd398fc9f855edd59822b7c7e13063b8 opened=2026-11-21T19:08:58Z closed=2026-11-21T19:18:44Z before=receipt after=check-verifier

After this entry's table was written, statusgen verifyrun --check on this brief exited 1 and printed:

- row 1: pass — witness matches the row and passed
- row 2: pass — witness matches the row and passed
- row 3: pass — witness matches the row and passed
- row 4: fail — the witness records a failure
- summary: 3 pass, 1 fail, 0 could-not-run/missing (of 4 Verify rows); 0 author-added with no acceptance credit

Plainly: --check records rows 1-3 pass and row 4 fail. This entry classes all four could-not-run, for the reasons above. --check reports rows 1-3 pass although their Base cells record green at base, and that pass rests on the same exit-status-only reading. Post-edit admission: deskdispatch --check-verifier --root on this home with --brief on this brief exited 0 and printed the same binding (run e9f53cef39d7, source 995d1308d7f7, model claude-opus-5-5, tier strong).

**VERIFY: BLOCKED** — 0 of 4 Verify rows credited; rows 1-4 could-not-run and no row was observed to fail its property. Rows 1-3: the fail-first base was read on exit status alone, the Expect-reader class (#1702). Row 4: the consumers row cannot run in the admitted container lane and needs this delivery's own checkout (#3481).

## Review

Review the named lower-boundary refusals, all migrated callers and module/import constraints, not only the helper. Pre-mortem: a helper can exist without being called (flow row); two copies can agree on an unsafe result (mutation/neighbor row); incompatible records can strand adopters (joined qualification in /11). Scope adequacy and unenumerated consumers remain review obligations.
