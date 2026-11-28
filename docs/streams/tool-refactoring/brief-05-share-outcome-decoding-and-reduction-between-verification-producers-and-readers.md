---
brief: assay:assay:tool-refactoring:05
title: "Share outcome decoding and reduction between verification producers and readers"
why: "The outcome writer can accept values its queue readers reject, while row pass and verified acceptance mean different things. A shared reducer prevents contradictory or incomplete records from stranding work or skipping closure checks."
wave: 0
depends: []
unblocks: ["tool-refactoring/10", "tool-refactoring/11"]
effort: M
gate: human
decision-issue: 7061
ruling: "https://github.com/medici-finance/assay/issues/7061#issuecomment-7066059966"
design: DR-tool-refactoring-05
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 2
id: cca3ff32-d298-45a7-9f43-d7cfc226253a
outcome: none
authored: 2026-11-12 by author-brief session
sources:
  - "docs/streams/tool-refactoring/spec.md — draft scope; not implementation or activation approval"
  - "freshness-checked 2026-11-12 @ 680102a0877755feab9ef85dc4e1d3f52a137a54; source inspection and bounded issue/PR snapshots; not a reproduced production failure"
  - "medici-finance/assay#6002 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5827 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5950 — related class evidence; resolution not asserted"
  - "medici-finance/assay#6272 — related class evidence; resolution not asserted"
  - "medici-finance/assay#1830 — related class evidence; resolution not asserted"
  - "desk-tools/46 — retained owning work; see existing-work.md"
  - "desk-tools/47 — retained owning work; see existing-work.md"
  - "desk-tools/49 — retained owning work; see existing-work.md"
  - "statusgen/37 — retained owning work; see existing-work.md"
gate-why: "Outcome normalization controls whether verified-closure and held-row checks run. Human acceptance confirms a row result or unknown outcome cannot bypass those independent gates."
decision-trigger: start
exec-tier: strong
exec-tier-why: "Cross-component class refactoring and negative controls; one local happy-path test cannot prove that every production caller migrated."
domain: complicated
consumers:
  - "statusgen/evaluation/outcome.go: follow-up tool-refactoring/05 (this brief; source implementation is still owed)"
  - "statusgen/evaluation/outcome_test.go: follow-up tool-refactoring/05 (this brief; source implementation is still owed)"
  - "statusgen/awaiting_bucket.go: follow-up tool-refactoring/05 (this brief; source implementation is still owed)"
  - "tools/desk/internal/deskkit/verifyoutcomes.go: follow-up tool-refactoring/05 (this brief; source implementation is still owed)"
  - "tools/desk/internal/deskkit/verifywake.go: follow-up tool-refactoring/05 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskevidence/outcome.go: follow-up tool-refactoring/05 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskevidence/outcomerecord.go: follow-up tool-refactoring/05 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/deskevidence/outcomeprotocol_test.go: follow-up tool-refactoring/05 (this brief; source implementation is still owed)"
---

# Brief 05 — Share outcome decoding and reduction between verification producers and readers

## Context

files:
- statusgen/evaluation/outcome.go
- statusgen/evaluation/outcome_test.go
- statusgen/awaiting_bucket.go
- tools/desk/internal/deskkit/verifyoutcomes.go
- tools/desk/internal/deskkit/verifywake.go
- tools/desk/cmd/deskevidence/outcome.go
- tools/desk/cmd/deskevidence/outcomerecord.go
- tools/desk/cmd/deskevidence/outcomeprotocol_test.go
- docs/contracts.md
- docs/library-first.md
- tools/desk/README.md
- changelog/tool-refactoring-05.md

facts (2026-11-12, source `680102a08777`):
- Planned paths above identify new Go/test deliverables and existing starting seams; no out-of-repository files are changed.
- Class: share outcome decoding and reduction between verification producers and readers; the issue cases in sources nominate this mechanism, not a claim that every old report still reproduces.
- Existing owner: statusgen/evaluation outcome package with adapters in deskkit and statusgen.
- Required reading: [scope](spec.md), [retained work and active PRs](existing-work.md), `docs/library-first.md`, `docs/contracts.md`, and the source paths above. Source inspection precedes any implementation effect.
- All dependency and caller changes are this brief's bounded scope. Retained work: desk-tools/46, desk-tools/47, desk-tools/49, statusgen/37. Completed Evidence and dated sources stay unchanged.

layering: keep pure syntax/state/judgment in its named existing library owner and source, process, credential, forge and persistence effects in adapters. Module boundaries are real: desk-tools may import public statusgen subpackages; statusgen cannot import internal deskkit. No new daemon, effect journal, generic bypass or public command is needed.

single-point-of-failure: a caller can bypass the shared decision — the class guard must detect a second planted caller; independent current-state, custody and conditional-write/process checks still refuse at their own effect boundary. Pure-library tests prove no effect authority.

design-fit:
  owner: statusgen/evaluation outcome package with adapters in deskkit and statusgen
  contract: S-evidence-coverage; S-verification-domain
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

1. Extract the existing record vocabulary, normalization and reduction into an importable pure package; retain legacy/per-file adapters and exact record naming compatibility. Row pass is not an overall verified outcome. Reject unknown values rather than let writers create records readers cannot interpret.
2. Use one typed reduction of row_results, held-row selection, run coverage and outcome. Distinguish a blocked row from a closure-only blocker; an empty held set cannot erase observed non-pass rows. Preserve exact row IDs and explicitly represent absent evidence as unavailable.
3. Keep append-only diagnostic/outcome observation separate from changing accepted lifecycle state. A later BLOCKED diagnostic on an already-verified delivery neither demotes prior acceptance nor authorizes a new verified flip. Apply current protected-closure rules at the actual state-changing boundary.
4. Migrate actual writer and queue readers, delete copied enum/reducer bodies, and qualify old/new record readers. Do not implement fixed-delivery policy, PASS reuse or wake deduplication here: those remain with desk-tools/46–51 and their precise decisions.
5. Update the named API/CLI reference and existing semantic-owner row; add the changelog fragment. Keep consumer routing deferred until delivery actually edits each site. Publish the class census and retired-callsite diff.

## Class guard

Enumerate every outcome writer and queue/closure reader, including legacy and per-file layouts. Plant a second reader/writer with a divergent enum or held-row reduction; the shared codec/reducer contract must detect it. Preserve the source-pinned regression cases, show a pre-fix red control and retain the planted positive fixture.

## Interface contract

The statusgen/evaluation outcome package owns outcome decoding and reduction. Deskkit and statusgen retain their source and publication adapters. Inputs are explicit observations; outputs are typed decisions/records with unavailable states. Internal refactoring does not change a public schema, numeric exit contract, gate policy or custody boundary without its existing owner approval. Source-only success is not released/adopted capability.

## Verify

Row 4 reads the immutable delivery snapshot and its first-parent diff from local Git history. It executes no code from the snapshot. Missing history is could-not-check.

The named tests below are implementation deliverables, not tests claimed to exist today. Each row requires the named `--- PASS:` line as well as exit 0, so a vacuous selector cannot pass. Class controls plant a second defect site outside the reported instance and prove the guard catches it; retain that positive control. Record the pre-fix red and post-fix green.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -C statusgen/evaluation -v -count=1 -timeout 5m -run '^TestOutcomeProtocolMatrix$' . > "$o" && grep -F -- '--- PASS: TestOutcomeProtocolMatrix ' "$o" && grep -F -- '--- PASS: TestOutcomeProtocolMatrix/rows:_the_reduction_of_counts,_held_rows_and_row_results ' "$o")` | exit 0; output contains "--- PASS: TestOutcomeProtocolMatrix"; wire vocabulary and reducer matrix | check:ci +dereference |
| 2 | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -C statusgen/evaluation -v -count=1 -timeout 5m -run '^TestOutcomeProtocolClassGuard$' . > "$o" && grep -F -- '--- PASS: TestOutcomeProtocolClassGuard ' "$o")` | exit 0; output contains "--- PASS: TestOutcomeProtocolClassGuard"; writer/reader drift and second enum plant | check:ci +mutation +neighbour |
| 3 | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -C tools/desk -v -count=1 -timeout 5m -run '^TestOutcomeProtocolFlow$' ./cmd/deskevidence > "$o" && grep -F -- '--- PASS: TestOutcomeProtocolFlow ' "$o")` | exit 0; output contains "--- PASS: TestOutcomeProtocolFlow"; publication and queue reader flow | check:ci +flow |
| 4 | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief tool-refactoring/05 --delivery d14b0b9429679c545d185df9d2fdf954ea0791ae --base 0e5d78dae6dfbfea84d4ea66c356d498d2428329 > "$o" && cat "$o" && grep -Fx -- 'summary: 8 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing' "$o")` | Exit 0 and exactly 8 corroborated, 0 disproved, 0 unchecked. Zero unchecked routes are required. | check:ci +flow |

## Definition of Done

Every Task path has an actual caller or documented retention reason, consumer flows and planted controls pass, obsolete duplicate bodies are removed, and docs/reference plus changelog explain the surviving contract. An independent verifier fills Evidence; this brief does not self-award acceptance. No change to historical Verify or Evidence is included.

## Evidence

<!-- Independent verification appends actual commands, exits, output/digests, date and runner after implementation. Empty at authoring. -->

### Non-implementer verifier run: 2026-11-24, container-netns bracket, merged main 034614a8c462b094fa7ec9396c4d70f6e6595a80

The runner is assay-verifier-app[bot] (model claude-opus-5-5, tier strong). It is not the implementer. This is the
first verification of this brief.

Source: 034614a8c462b094fa7ec9396c4d70f6e6595a80. The delivering change is #7067, merged as
d14b0b9429679c545d185df9d2fdf954ea0791ae; its implementation commit is ee047b5e5f79, with the trailer
"Brief: tool-refactoring/05". D's first parent, 0e5d78dae6dfbfea84d4ea66c356d498d2428329, is the fail-first
base. D is an ancestor of the source. The Verify section is byte-identical at the merge-base f8a8dad43f8d, at
the first parent, at D and at the source.

The brief answers sensitive-data: yes, so every row is risk-bearing and owes a fail-first witness. The original
table is the four rows above: `deskpathguard rederive` exited 0 and reported 4 from-base rows and 0 author-added
rows (base 034614a8c462).

This is DISPATCH-FOR-EVIDENCE. The run gathers Evidence and a draft outcome record only. The desk never flips this
brief's status: the human's merge of the Evidence PR is the flip.

Admission: `deskdispatch --check-verifier --root <home> --brief docs/streams/tool-refactoring/brief-05-share-outcome-decoding-and-reduction-between-verification-producers-and-readers.md`
exited 0 for this detached home before any row ran. It returned attestation #7555,
run=80fcd217464ee86e8008095d41d05359d2a85ebec1d67fc85ba21b8f9ab7f339 and source 034614a8c462, with desk tools
v1.0.92 (darwin-arm64, verifier-attestation=checked-clean).

Every row ran under the declared verifier Git environment: all GIT_* variables and SSH_ASKPASS were unset, then
GIT_CONFIG_GLOBAL=/dev/null, GIT_CONFIG_NOSYSTEM=1, GIT_CONFIG_COUNT=0 and GIT_TERMINAL_PROMPT=0 were set, along
with KUBECONFIG=/dev/null.

Rows 1-3 ran in the container-netns admission bracket:

- On the host, `statusgen verifyrun --open-bracket` wrote the receipt, a fresh module cache and the baseline
  history pack outside the home. It opened at 16:32:31Z.
- The container ran verify-runner v1.0.92, sha256:d3d958719dea, under `--network none --read-only` with
  GOPROXY=off, GOTOOLCHAIN=local and GOFLAGS=-mod=readonly. It ran
  `statusgen verifyrun --sandbox=container-netns --rows 1,2,3 --fail-first --delivering-commit d14b0b942967 --base 0e5d78dae6df --base-log <dir> --history-pack <pack>`.
- A first container start at 16:37:40Z was refused before any row ran: it named the brief by a relative path,
  and verifyrun did not match it to the receipt. It is not an attempt. The same command with the brief's
  absolute path under the mount is the single attempt, 16:37:57Z-16:38:57Z. It exited 1: rows 1-3 pass at the
  head and the fail-first reads each as "non-discriminating — green at base 0e5d78dae6df".
- On the host, `statusgen verifyrun --close-bracket` closed it at 16:40:17Z.
- `statusgen verifyrun --bracket-digest` exited 0 with verdict=match (witness sha256:59d3fab37006).

Row 4 was outside the `--rows 1,2,3` selection (reasons below).

Row 1: could-not-run, check-definition. The base is genuinely green.

The retained base log, sha256:9e745d61bce6, prints "--- PASS: TestOutcomeProtocolMatrix" with eleven subtests.
That is not a reader artefact. A test of the same name, which checks only the reader's outcome-word parse, was
added by da221b9cb538 ("Use the same overall verdict in verification writers and queue readers", merged by #6872
on 2026-11-18). That came after this brief was authored on 2026-11-12, and it is in the merge-base f8a8dad43f8d.
The delivery replaced that body with the wider matrix: writer acceptance, row and run reduction, and both record
layouts. As written, row 1 names a test that already passes at D's first parent, so it cannot show the pre-fix
red the Verify preamble requires. The head result is pass exit=0. This is a property of the row's definition
against this history, not an observed failure of the delivery.

Rows 2 and 3: could-not-run, Expect-reader defect (#7389, in the #1702 class).

The witness records each row as pass exit=0 at the head. Its fail-first reads each as non-discriminating, green at
base, and the run exits 1. Each Expect requires the named "--- PASS:" line as well as exit 0, and verifyrun does
not read an "output contains" Expect: it decides on exit status alone (#7389).

The retained base logs show what the base actually printed:

- Row 2, sha256:b07731628e1e: "testing: warning: no tests to run" and
  "ok github.com/medici-finance/assay/statusgen/evaluation [no tests to run]".
- Row 3, sha256:1bdf8980b65c: the same pair for tools/desk/cmd/deskevidence.

Neither prints a "--- PASS:" line. Read against its Expect, each base run is the pre-fix red, and each row would
discriminate. The green-at-base reading is the reader's artefact, not a property of the rows. The rows are classed
could-not-run, never fail, under reviewer-lane practice:
https://github.com/medici-finance/assay/pull/7228#pullrequestreview-5979645423 and
https://github.com/medici-finance/assay/pull/7230#pullrequestreview-5979708866. The witness and its exit 1 still
record them as non-discriminating.

Row 4: could-not-run, check-definition (#6978).

The row must run in a clean isolated checkout at the delivery D, where HEAD is D. The admitted home is detached at
034614a8c462, and the brief forbids substituting it. verifyrun has no admitted lane for a row that needs a
separate checkout at the delivery head (#6978), so the row was left out of the bracket's selection. It has no
acceptance witness.

Supplemental host probes. These carry no acceptance credit and were run after the bracket closed, on the darwin
host with the verifier Git environment:

- Head output. In a clean local worktree outside the home, detached at 034614a8c462, the three row commands ran
  verbatim. Each exited 0 and printed its named line: "--- PASS: TestOutcomeProtocolMatrix",
  "--- PASS: TestOutcomeProtocolClassGuard" and "--- PASS: TestOutcomeProtocolFlow".
- Row 4 at D. A local clone outside the home was checked out detached at d14b0b942967, with HEAD^ at
  0e5d78dae6df, and row 4's command ran there verbatim with statusgen v1.0.92. It exited 0 and printed
  "summary: 8 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing" (output sha256:8ddb7221f7bf).
  All eight routes are self-routes carried by this diff. The same output prints "PRODUCER-DELIVERY:
  COULD-NOT-CHECK" and "LIMIT: only the entries written above were judged".

Risk-bearing values. Every literal in the delivery's non-test diff was enumerated:

- `OutcomeWordVerified = "verified"` @ statusgen/evaluation/outcome.go:55, the only word mapped to OutcomePass
  (:65) in the closed vocabulary (:63-71). ClassifyOutcome (:99) lower-cases and trims; ParseOutcomeWord (:107)
  refuses any other word on the write side.
- The other outcome words: verify-fail and fail map to Fail; blocked, needs_context and needs-context map to Hold
  (:56-60).
- The row words `RowPass = "pass"`, RowFail, `RowCouldNotRun = "could-not-run"` and RowDeferred @ outcome.go:192-195.
- The completion predicates @ outcome.go:514 (exact rows: no non-pass row, no held row, at least one pass) and
  :516 (count-only: no held row, a total and a pass count present, total > 0, passed == total).
- `MaxClockSkew = 5 * time.Minute` @ statusgen/evaluation/outcomerecord/outcomerecord.go:36, re-exported by
  deskkit (tools/desk/internal/deskkit/verifyoutcomes.go:58). It is a move: both former copies were 5m.
- `OutcomeRecordsDir = "docs/streams/verify-outcomes"` @ outcomerecord.go:32 and the 12-hex digest truncation
  @ outcomerecord.go:86. Both are moves, unchanged.
- The record file mode 0o644 @ statusgen/outcomessplit.go:115, unchanged.

The verified-word binding and the completion predicates rank first. They decide whether a record closes a brief
and whether held-row and closure checks run, which is the irreversible lifecycle change. The clock-skew tolerance
ranks next. The directory, digest width and file mode are reversible naming and are unchanged.

RISK-VALUE: DERIVED — OutcomeWordVerified = "verified" as the only OutcomePass word @ statusgen/evaluation/outcome.go:55 and :65 — brief Task 1: "Row pass is not an overall verified outcome. Reject unknown values"; "pass" and "could-not-run" classify Unrecognised, and the writer refuses them.
RISK-VALUE: DERIVED — Complete predicates @ statusgen/evaluation/outcome.go:514 and :516 — brief Task 2: "an empty held set cannot erase observed non-pass rows", and closure needs at least one observed pass, never an empty set.
RISK-VALUE: NAMED, NOT DERIVED — MaxClockSkew = 5 * time.Minute @ statusgen/evaluation/outcomerecord/outcomerecord.go:36 — no source pins or derives the tolerance, and the brief names none. It is unchanged from both former copies, so the move changes no behaviour.
RISK-VALUE: N/A — OutcomeRecordsDir, the 12-hex digest truncation and file mode 0o644 — moved or untouched byte for byte; reversible naming, not risk-bearing.

Tool reads after the Evidence was written:

- `statusgen verifyrun --check` exited 0 and printed "3 pass, 0 fail, 1 could-not-run/missing (of 4 Verify rows);
  0 author-added with no acceptance credit". It reports rows 1-3 as "pass — witness matches the row and passed"
  and row 4 as "could-not-run — no execution witness in Evidence".
- The pass that `--check` gives rows 1-3 rests on the witness's exit-status reading (#7389). The same witness's
  fail-first run exited 1, reading those rows as non-discriminating. So this entry does not count rows 1-3 as
  passing. It classes them could-not-run for the reasons above.
- `statusgen verifyrun --bracket-digest --witness` on this brief exited 0 with verdict=match (bracket=L287,
  witness sha256:59d3fab37006).
- With `--credit --source 034614a8c462`, it exited 1 and printed credit=not-credited: "L278 differs from HEAD; an
  uncommitted line has no author".
- `statusgen --lint` on the home reports no finding for this brief. Its 3 PROBLEMs are other briefs' protected
  tables (desk-tools/37, desktools-v2/65, forge-neutral/54).
- Post-edit admission: the same `deskdispatch --check-verifier` ran again at 2026-11-24T16:44:20Z and exited 0
  with the same binding: run=80fcd217464ee86e8008095d41d05359d2a85ebec1d67fc85ba21b8f9ab7f339, source
  034614a8c462, model claude-opus-5-5, tier strong, attestation #7555.

Tally: 0 of 4 rows pass. Rows 1-4 are could-not-run, and no credited row failed. No-credit probes show every row's Expect met at the head and at D: rows 1-3 print their named "--- PASS:" lines, and row 4 reports 8 corroborated, 0 disproved, 0 unchecked.

**VERIFY: BLOCKED** — 0/4 rows pass at 034614a8c462; no credited row failed. Row 1 is could-not-run because the test it names already exists and passes at D's first parent (added by #6872 after authoring), so it cannot show the required pre-fix red. Rows 2-3 are could-not-run because verifyrun does not read their "output contains --- PASS" Expects, so its fail-first reads a base with no such test as green (#7389). Row 4 is could-not-run because it needs a checkout at the delivery and has no admitted lane (#6978). No-credit probes at the head and at D meet every Expect.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `cd statusgen && GOWORK=off go test -v ./evaluation -run '^TestOutcomeProtocolMatrix$' -count=1` | pass exit=0 sandbox=container-netns | sha256:53ca720a657e | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=0e5d78dae6df green rc=0 · head=034614a8c462 · delivering=d14b0b942967 output=sha256:9e745d61bce641dd8d06d5bb6e4be92e499bcfab987cba4275bc8ce6c1ed67fa log=row-000001.log |
| 2 | `cd statusgen && GOWORK=off go test -v ./evaluation -run '^TestOutcomeProtocolClassGuard$' -count=1` | pass exit=0 sandbox=container-netns | sha256:bd982a143f10 | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=0e5d78dae6df green rc=0 · head=034614a8c462 · delivering=d14b0b942967 output=sha256:b07731628e1e7d7c314e637e107151228cce0b775433bc91ed4c927c80abd62f log=row-000002.log |
| 3 | `cd tools/desk && GOWORK=off go test -v ./cmd/deskevidence -run '^TestOutcomeProtocolFlow$' -count=1` | pass exit=0 sandbox=container-netns | sha256:454e58522e81 | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=0e5d78dae6df green rc=0 · head=034614a8c462 · delivering=d14b0b942967 output=sha256:1bdf8980b65c58252038ae89dbdb5753861fb54f13111edccae3819af3adaa1b log=row-000003.log |
Tree-Digest: sha256:0d2fc44c1c8d265eb7dca088c90ea5f3c8585cc0ebd893ea66e55cd0a81eb622
Container-Image: sha256:d3d958719dea47fc611e096612c1478bcc07376b7f9783f92433ac8faeeee380
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7555 run=80fcd217464ee86e8008095d41d05359d2a85ebec1d67fc85ba21b8f9ab7f339 source=034614a8c462b094fa7ec9396c4d70f6e6595a80 brief=docs/streams/tool-refactoring/brief-05-share-outcome-decoding-and-reduction-between-verification-producers-and-readers.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=80fcd217464ee86e8008095d41d05359d2a85ebec1d67fc85ba21b8f9ab7f339 source=034614a8c462b094fa7ec9396c4d70f6e6595a80 tree=sha256:0d2fc44c1c8d265eb7dca088c90ea5f3c8585cc0ebd893ea66e55cd0a81eb622 witness=sha256:59d3fab3700690ef6ba37415e479fdcd675627b0f90a79049bf8fe499be5f3b0 opened=2026-11-24T16:32:31Z closed=2026-11-24T16:40:17Z before=receipt after=check-verifier

## Review

Review the named lower-boundary refusals, all migrated callers and module/import constraints, not only the helper. Pre-mortem: a helper can exist without being called (flow row); two copies can agree on an unsafe result (mutation/neighbor row); incompatible records can strand adopters (joined qualification in /11). Scope adequacy and unenumerated consumers remain review obligations.
