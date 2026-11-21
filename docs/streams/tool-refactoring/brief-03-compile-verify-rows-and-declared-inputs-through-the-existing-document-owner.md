---
brief: assay:assay:tool-refactoring:03
title: "Compile Verify rows and declared inputs through the existing document owner"
why: "A Verify table can be accepted by one reader and rejected or misexecuted by another. One document compiler makes row identity and declared input syntax agree before an expensive verification run."
wave: 0
depends: []
unblocks: ["tool-refactoring/04", "tool-refactoring/10"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
decision-issue: 7060
ruling: "https://github.com/medici-finance/assay/issues/7060#issuecomment-7066010310"
design: DR-tool-refactoring-03
issues: []
schema: brief-v2
version: 1
id: 18d445a4-baf8-4f70-8eca-7ad0092dffa9
outcome: none
authored: 2026-11-12 by author-brief session
sources:
  - "docs/streams/tool-refactoring/spec.md — draft scope; not implementation or activation approval"
  - "freshness-checked 2026-11-12 @ 680102a0877755feab9ef85dc4e1d3f52a137a54; source inspection and bounded issue/PR snapshots; not a reproduced production failure"
  - "medici-finance/assay#5575 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5097 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5013 — related class evidence; resolution not asserted"
  - "medici-finance/assay#4199 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5807 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5434 — related class evidence; resolution not asserted"
  - "statusgen/29 — retained owning work; see existing-work.md"
  - "statusgen/30 — retained owning work; see existing-work.md"
  - "statusgen/34 — retained owning work; see existing-work.md"
  - "statusgen/37 — retained owning work; see existing-work.md"
  - "statusgen/40 — retained owning work; see existing-work.md"
gate-why: "Declared inputs and executable row syntax feed verification admission. Human acceptance confirms unsupported or ambiguous declarations cannot expand allowed source or turn prose into an executed/credited check."
decision-trigger: start
exec-tier: strong
exec-tier-why: "Cross-component class refactoring and negative controls; one local happy-path test cannot prove that every production caller migrated."
domain: complicated
consumers:
  - "statusgen/briefdoc/verification.go: follow-up tool-refactoring/03 (this brief; source implementation is still owed)"
  - "statusgen/briefdoc/verification_test.go: follow-up tool-refactoring/03 (this brief; source implementation is still owed)"
  - "statusgen/verifyrun.go: follow-up tool-refactoring/03 (this brief; source implementation is still owed)"
  - "statusgen/evaluation/declared_api.go: follow-up tool-refactoring/03 (this brief; source implementation is still owed)"
  - "tools/desk/internal/deskkit/declaredscope.go: follow-up tool-refactoring/03 (this brief; source implementation is still owed)"
  - "tools/desk/internal/deskkit/verificationdefinition_test.go: follow-up tool-refactoring/03 (this brief; source implementation is still owed)"
---

# Brief 03 — Compile Verify rows and declared inputs through the existing document owner

## Context

files:
- statusgen/briefdoc/verification.go
- statusgen/briefdoc/verification_test.go
- statusgen/verifyrun.go
- statusgen/evaluation/declared_api.go
- tools/desk/internal/deskkit/declaredscope.go
- tools/desk/internal/deskkit/verificationdefinition_test.go
- docs/contracts.md
- docs/library-first.md
- tools/desk/README.md
- changelog/tool-refactoring-03.md

facts (2026-11-12, source `680102a08777`):
- Planned paths above identify new Go/test deliverables and existing starting seams; no out-of-repository files are changed.
- Class: compile verify rows and declared inputs through the existing document owner; the issue cases in sources nominate this mechanism, not a claim that every old report still reproduces.
- Existing owner: statusgen/briefdoc for syntax; statusgen/evaluation for normalized evaluation inputs.
- Required reading: [scope](spec.md), [retained work and active PRs](existing-work.md), `docs/library-first.md`, `docs/contracts.md`, and the source paths above. Source inspection precedes any implementation effect.
- All dependency and caller changes are this brief's bounded scope. Retained work: statusgen/29, statusgen/30, statusgen/34, statusgen/37, statusgen/40. Completed Evidence and dated sources stay unchanged.

layering: keep pure syntax/state/judgment in its named existing library owner and source, process, credential, forge and persistence effects in adapters. Module boundaries are real: desk-tools may import public statusgen subpackages; statusgen cannot import internal deskkit. No new daemon, effect journal, generic bypass or public command is needed.

single-point-of-failure: a caller can bypass the shared decision — the class guard must detect a second planted caller; independent current-state, custody and conditional-write/process checks still refuse at their own effect boundary. Pure-library tests prove no effect authority.

design-fit:
  owner: statusgen/briefdoc for syntax; statusgen/evaluation for normalized evaluation inputs
  contract: S-context-handoff; S-evidence-coverage
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

1. Extend briefdoc with one pure compilation of Verify row identity, class, shell, explicit command, Expect and Context files/inputs declarations. Reuse existing section/fence grammar and evaluation declaration APIs; keep source enumeration and path confinement in adapters.
2. Characterize current accepted forms before migrating: inline/list/backticked declarations, NEW annotations, prose tails, escaped pipes, numeric and lettered row IDs, gate:model prose and explicit cmd markers. Preserve intentional legacy profiles. Do not choose the union of permissive parsers or execute a prose code span.
3. Migrate verifyrun and the deskkit declared-scope projection to the same compiled record. Lint/newbrief keep their existing structural and strength rules, consuming shared syntax where applicable. Unsupported syntax holds at authoring/preflight with the same typed cause, before row execution.
4. Remove local lexical copies and retain actual-caller tests with a planted divergent parser. This owns syntax consolidation only: statusgen/29–31 retain authoring/debt repair, /34 scratch, /37 witness-v2 writes and /40 approved acceptance transitions.
5. Update the named API/CLI reference and existing semantic-owner row; add the changelog fragment. Keep consumer routing deferred until delivery actually edits each site. Publish the class census and retired-callsite diff.

## Class guard

Enumerate Verify-row and Context-declaration parsers/projections across statusgen and deskkit. A planted local parser that accepts an ambiguous declaration or prose code span must fail the census/behavior control; legacy differences need a named profile, not a second grammar. Preserve the source-pinned regression cases, show a pre-fix red control and retain the planted positive fixture.

## Interface contract

Statusgen/briefdoc owns document syntax. Statusgen/evaluation owns normalized evaluation inputs; adapters retain source enumeration and path confinement. Inputs are explicit observations; outputs are typed decisions/records with unavailable states. Internal refactoring does not change a public schema, numeric exit contract, gate policy or custody boundary without its existing owner approval. Source-only success is not released/adopted capability.

## Verify

Row 4 runs only in a clean isolated checkout of the independently resolved implementation delivery D, with its protected delivery binding checked by the existing owner. There HEAD is D and HEAD^ is its first parent. Do not select current integration HEAD as a convenient substitute. If D or its binding cannot be established, record could-not-check. Other rows keep their admitted source and recorded witness identity.

The named tests below are implementation deliverables, not tests claimed to exist today. Each row requires the named `--- PASS:` line as well as exit 0, so a vacuous selector cannot pass. Class controls plant a second defect site outside the reported instance and prove the guard catches it; retain that positive control. Record the pre-fix red and post-fix green.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && GOWORK=off go test -v ./briefdoc -run '^TestVerificationDefinitionMatrix$' -count=1` | exit 0; output contains "--- PASS: TestVerificationDefinitionMatrix"; document and row grammar matrix | check:ci +dereference |
| 2 | `cd statusgen && GOWORK=off go test -v ./briefdoc -run '^TestVerificationDefinitionClassGuard$' -count=1` | exit 0; output contains "--- PASS: TestVerificationDefinitionClassGuard"; no prose execution and second-parser plant | check:ci +mutation +neighbour |
| 3 | `cd tools/desk && GOWORK=off go test -v ./internal/deskkit -run '^TestVerificationDefinitionFlow$' -count=1` | exit 0; output contains "--- PASS: TestVerificationDefinitionFlow"; runner to landing scope flow | check:ci +flow |
| 4 | `statusgen --root . --consumers --brief tool-refactoring/03 --delivery "$(git rev-parse HEAD)" --base "$(git rev-parse HEAD^)"` | exit 0; zero disproved and zero unchecked consumer routes for the delivered brief | check:ci +flow |

## Definition of Done

Every Task path has an actual caller or documented retention reason, consumer flows and planted controls pass, obsolete duplicate bodies are removed, and docs/reference plus changelog explain the surviving contract. An independent verifier fills Evidence; this brief does not self-award acceptance. No change to historical Verify or Evidence is included.

## Evidence

<!-- Independent verification appends actual commands, exits, output/digests, date and runner after implementation. Empty at authoring. -->

### Non-implementer verifier run: 2026-11-21, container-netns bracket, merged main f8a8dad43f8d7c3ce50c9ec196126d0a89bcda98

The runner is assay-verifier-app[bot] (model claude-opus-5-5, tier strong). It is not the implementer. This is the
first verification of this brief.

Source: f8a8dad43f8d7c3ce50c9ec196126d0a89bcda98. The delivering change is #7065, merged as
128fd2eafec64a4a4ddefbbae8b073c73817fae2. Its first parent, 995d1308d7f778d2f71221ced851a6c2c4c6158f, is the
fail-first base. This brief's file is byte-identical at the delivery and at the source.

The brief answers sensitive-data: yes, so every row is risk-bearing and owes a fail-first witness. The original
table is the four rows above: `deskpathguard rederive` reports 4 from-base rows and 0 author-added rows.

This is DISPATCH-FOR-EVIDENCE. The run gathers Evidence and a draft outcome record only. The desk never flips this
brief's status: the human's merge of the Evidence PR is the flip.

Admission: `deskdispatch --check-verifier --root <home> --brief docs/streams/tool-refactoring/brief-03-compile-verify-rows-and-declared-inputs-through-the-existing-document-owner.md`
exited 0 for this detached home before any row ran. It returned attestation #7388,
run=eb7b1d9b9525e98e295c93949eec5665e4db45c7724565fdf7de8551a2c88f9f and source f8a8dad43f8d, with desk tools
v1.0.90.

Every row ran under the declared verifier Git environment: all GIT_* variables and SSH_ASKPASS were unset, then
GIT_CONFIG_GLOBAL=/dev/null, GIT_CONFIG_NOSYSTEM=1, GIT_CONFIG_COUNT=0 and GIT_TERMINAL_PROMPT=0 were set, along
with KUBECONFIG=/dev/null.

Rows 1-3 ran in the container-netns admission bracket:

- On the host, `statusgen verifyrun --open-bracket` wrote the receipt and a fresh module cache outside the home.
  It opened at 19:27:23Z.
- The container ran verify-runner v1.0.90, sha256:4e8e3c80a075, under `--network none --read-only` with
  GOPROXY=off and GOTOOLCHAIN=local. It ran
  `statusgen verifyrun --sandbox=container-netns --rows 1,2,3 --fail-first --delivering-commit 128fd2eafec6 --base 995d1308d7f7 --base-log <dir> --history-pack <pack>`.
- On the host, `statusgen verifyrun --close-bracket` closed it at 19:39:44Z.
- `statusgen verifyrun --bracket-digest` exited 0 with verdict=match (witness sha256:ab872245cfd9).

Row 4 was outside the `--rows 1,2,3` selection (reasons below).

Two attempts ran inside this bracket:

- Attempt 1 (19:35:56Z-19:37:07Z) kept no base log. It was superseded by attempt 2, which retained the base output.
- The witness below is attempt 2 (19:38:16Z-19:39:21Z).

Both attempts exited 1. Both recorded rows 1-3 as passing at the head and as "non-discriminating — green at base
995d1308d7f7".

Rows 1-3: could-not-run, Expect-reader defect (#7389, in the #1702 class).

The witness records each row as pass exit=0 at the head. Its fail-first result reads each row as
non-discriminating, green at base, and the run exits 1. Neither reading observed the row's property. Each
Expect requires the named "--- PASS:" line as well as exit 0, and verifyrun does not read an "output contains"
Expect: it decides on exit status alone (#7389).

The retained base logs show what the base actually printed:

- Rows 1 and 2 have the same output, sha256:6386466e2731: "testing: warning: no tests to run" and
  "ok github.com/medici-finance/assay/statusgen/briefdoc [no tests to run]".
- Row 3, sha256:f06b221dad41: the same pair for tools/desk/internal/deskkit.

None of them prints a "--- PASS:" line. Read against its Expect, each base run is the pre-fix red, and each row
would discriminate. The green-at-base reading is the reader's artefact, not a property of the rows.

The rows are classed could-not-run, never fail, under reviewer-lane practice:
https://github.com/medici-finance/assay/pull/7228#pullrequestreview-5979645423 and
https://github.com/medici-finance/assay/pull/7230#pullrequestreview-5979708866. The witness and its exit 1 still
record them as non-discriminating.

Row 4: could-not-run, check-definition (#6978).

The row must run in a clean isolated checkout at the delivery D, where HEAD is D. The admitted home is detached at
f8a8dad43f8d, and the brief forbids substituting it. verifyrun has no admitted lane for a row that needs a
separate checkout at the delivery head (#6978), so the row was left out of the bracket's selection. It has no
acceptance witness.

Supplemental host probes. These carry no acceptance credit and were run after the bracket closed:

- Head output. The three row commands ran verbatim at the source in the same image, network none. Each exited 0
  and printed its named line: "--- PASS: TestVerificationDefinitionMatrix",
  "--- PASS: TestVerificationDefinitionClassGuard" and "--- PASS: TestVerificationDefinitionFlow".
- Row 4 at D. A local clone outside the home was checked out detached at 128fd2eafec6, with HEAD^ at
  995d1308d7f7, and row 4's command ran there verbatim. It exited 0 and printed "summary: 5 corroborated,
  0 disproved, 1 unchecked". The unchecked entry is statusgen/evaluation/declared_api.go, routed as a
  follow-up to this brief, which says "source implementation is still owed". The diff does not touch that file.
  DR-tool-refactoring-03 defers it: "Deferred, not decided here: the evaluation declaration API". At D, that
  entry does not meet the row's "zero unchecked" Expect.

Risk-bearing values. Every literal in the delivery's non-test diff was enumerated:

- `MaxScopePaths = 4096` @ statusgen/briefdoc/declared.go:36, used at :191.
- The declaration length bound 4096 @ declared.go:475.
- The brace-expansion bound 256 @ declared.go:523.
- The path length bound 4096 @ declared.go:554.
- The fence-indent bound `<= 3` @ declared.go:59.
- `CommandMarker = "cmd:"` @ statusgen/briefdoc/verification.go:146.

Each is a move, byte for byte, from the base: tools/desk/internal/deskkit/declaredscope.go:59, :499, :547, :578
and :83, and verifyCommandMarker @ statusgen/verifyrows.go:153. All are reversible.

The scope bounds rank first, because the declared scope bounds what a landing may touch.

RISK-VALUE: NAMED, NOT DERIVED — MaxScopePaths = 4096 @ statusgen/briefdoc/declared.go:36 — no source pins or derives the bound, and the brief names none. The value is unchanged from the base's maxScopePaths = 4096 @ tools/desk/internal/deskkit/declaredscope.go:59, so this move changes no behaviour. The same holds for the 4096 and 256 bounds @ declared.go:475, :523 and :554.
RISK-VALUE: DERIVED — fence indent <= 3 @ statusgen/briefdoc/declared.go:59 — a CommonMark fenced code block may be indented by at most three spaces.
RISK-VALUE: DERIVED — CommandMarker = "cmd:" @ statusgen/briefdoc/verification.go:146 — spec/brief-v1.md:580 and docs/brief-rules.md:75 define the marker as `cmd:`.

Tool reads after the Evidence was written:

- `statusgen verifyrun --check` exited 2 and printed "3 pass, 0 fail, 1 could-not-run/missing (of 4 Verify rows);
  0 author-added with no acceptance credit". It reports rows 1-3 as "pass — witness matches the row and passed"
  and row 4 as "could-not-run — no execution witness in Evidence".
- The pass that `--check` gives rows 1-3 rests on the witness's exit-status reading (#7389). The same witness's
  fail-first run exited 1, reading those rows as non-discriminating. So this entry does not count rows 1-3 as
  passing. It classes them could-not-run for the reasons above.
- `statusgen verifyrun --bracket-digest --witness` on this brief exited 0 with verdict=match (bracket=L268,
  witness sha256:ab872245cfd9).
- With `--credit --source f8a8dad43f8d`, it exited 1 and printed credit=not-credited: "L259 differs from HEAD; an
  uncommitted line has no author".
- Post-edit admission: the same `deskdispatch --check-verifier` ran again at 2026-11-21T19:44:37Z and exited 0
  with the same binding: run=eb7b1d9b9525e98e295c93949eec5665e4db45c7724565fdf7de8551a2c88f9f, source
  f8a8dad43f8d, model claude-opus-5-5, tier strong, attestation #7388.

Tally: 0 of 4 rows pass. Rows 1-4 are could-not-run, and no acceptance obligation was observed to fail.

**VERIFY: BLOCKED** — 0/4 rows pass at f8a8dad43f8d; no acceptance obligation was observed to fail. Rows 1-3 are could-not-run: verifyrun does not read their "output contains --- PASS" Expects, so its fail-first reads a base with no such test as green (#7389). Row 4 is could-not-run because it needs a checkout at the delivery and has no admitted lane (#6978). A supplemental probe at the delivery reports 1 unchecked consumer route.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `cd statusgen && GOWORK=off go test -v ./briefdoc -run '^TestVerificationDefinitionMatrix$' -count=1` | pass exit=0 sandbox=container-netns | sha256:4c7becbd8c24 | 2026-11-21 | assay-verifier-app[bot] @ f8a8dad43f8d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=995d1308d7f7 green rc=0 · head=f8a8dad43f8d · delivering=128fd2eafec6 output=sha256:6386466e2731d2f2ce0d214bfac06206b04c2948f6809937157789816d51b8bd log=row-000001.log |
| 2 | `cd statusgen && GOWORK=off go test -v ./briefdoc -run '^TestVerificationDefinitionClassGuard$' -count=1` | pass exit=0 sandbox=container-netns | sha256:1fe196a998de | 2026-11-21 | assay-verifier-app[bot] @ f8a8dad43f8d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=995d1308d7f7 green rc=0 · head=f8a8dad43f8d · delivering=128fd2eafec6 output=sha256:6386466e2731d2f2ce0d214bfac06206b04c2948f6809937157789816d51b8bd log=row-000002.log |
| 3 | `cd tools/desk && GOWORK=off go test -v ./internal/deskkit -run '^TestVerificationDefinitionFlow$' -count=1` | pass exit=0 sandbox=container-netns | sha256:bdceb83781b2 | 2026-11-21 | assay-verifier-app[bot] @ f8a8dad43f8d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=995d1308d7f7 green rc=0 · head=f8a8dad43f8d · delivering=128fd2eafec6 output=sha256:f06b221dad411e4c5c0c347dba3f0cb0e1747ab97d0f72fe454a36b24ee48901 log=row-000003.log |
Tree-Digest: sha256:f7547b5f941d9e923b299c8059e1e6ca9c72cb052fb28dbced711e3319afaee7
Container-Image: sha256:4e8e3c80a075c552de0966c886784c6020153c27e624f8a1ce988aa3b0590217
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7388 run=eb7b1d9b9525e98e295c93949eec5665e4db45c7724565fdf7de8551a2c88f9f source=f8a8dad43f8d7c3ce50c9ec196126d0a89bcda98 brief=docs/streams/tool-refactoring/brief-03-compile-verify-rows-and-declared-inputs-through-the-existing-document-owner.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=eb7b1d9b9525e98e295c93949eec5665e4db45c7724565fdf7de8551a2c88f9f source=f8a8dad43f8d7c3ce50c9ec196126d0a89bcda98 tree=sha256:f7547b5f941d9e923b299c8059e1e6ca9c72cb052fb28dbced711e3319afaee7 witness=sha256:ab872245cfd9b144f1dae23126b3823e79aeadd00a7abf37ac48254b463a5f66 opened=2026-11-21T19:27:23Z closed=2026-11-21T19:39:44Z before=receipt after=check-verifier

## Review

Review the named lower-boundary refusals, all migrated callers and module/import constraints, not only the helper. Pre-mortem: a helper can exist without being called (flow row); two copies can agree on an unsafe result (mutation/neighbor row); incompatible records can strand adopters (joined qualification in /11). Scope adequacy and unenumerated consumers remain review obligations.
