---
brief: assay:assay:desktools-v2:81
title: Run acceptance-transition rows inside a closed container-netns bracket
why: "An approved acceptance transition runs only on the host, so on a host with no `unshare --net` its check:ci rows are could-not-run, and a Linux runner cannot take them over because admission binds the dispatch host's runtime."
wave: 1
depends: []
unblocks: ["desk-tools/38", "desktools-v2/66", "agent-execution/01"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: "Changes which runs count as admitted network-off witnesses for protected acceptance rows (the hermetic witness is a security control; proposal 11 independent verification). Human acceptance of the transition bracket is required at done."
decision-issue: 1800
ruling: "https://github.com/medici-finance/assay/issues/1800#issuecomment-6665625151"
issues: [1800]
schema: brief-v2
version: 1
authored: 2026-11-05 by statusgen newbrief
sources:
  - 'desktools-v2: Run acceptance-transition rows inside a closed container-netns bracket'
  - "the driver's own comment https://github.com/medici-finance/assay/issues/1800#issuecomment-6665625151 (2026-11-05): a closed container-netns admission bracket meets the network-off requirement for protected acceptance-transition rows, and commission the statusgen change for option 1"
  - "#1800 finding, 2026-11-04: a Linux runner alone does not clear transition rows, because the verifier admission binds the dispatch host's runtime"
  - desktools-v2/71 (the container bracket, DR-container-witness) and desktools-v2/74 (`--rows`)
  - docs/brief-rules.md (rule 45)
  - docs/protected-paths.md (§Explicit human-approved acceptance transitions)
  - freshness-checked 2026-11-05 @ ae3e2b12d (origin/main)
exec-tier: strong
exec-tier-why: "The change moves protected-acceptance planning across the container admission boundary; the fail-closed paths and the plan binding need adversarial review."
domain: complicated
outcome: none
consumers:
  - "statusgen/verifyrun.go, statusgen/containerbracket.go, statusgen/acceptancetransition.go, statusgen/protectedclosure.go: follow-up desktools-v2/81"
  - "docs/brief-rules.md rule 45 and docs/protected-paths.md (the transition-execution paragraph): follow-up desktools-v2/81"
  - "plugins/assay/skills/verify-desk/SKILL.md (the `--acceptance-record` paragraph), plus the generated Cursor mirror: follow-up desktools-v2/81"
  - "tools/desk/cmd/deskdispatch/worldruntime_test.go (the runtime recheck through `--check-verifier`): one new test file only, no code change anywhere under tools/desk/: follow-up desktools-v2/81"
  - "a deskdispatch admission target for the runner's runtime (for example `--runtime linux-amd64` bound at dispatch): out-of-scope (deskdispatch, not statusgen; the bracket never re-admits inside the runner, so it does not need one)"
  - "cross-repository (version 2) receipts, `--in-container` and `--check` with `--acceptance-record`: out-of-scope (stay refused)"
---

# Brief 81 — Run acceptance-transition rows inside a closed container-netns bracket

## Context

files: statusgen/verifyrun.go, statusgen/containerbracket.go, statusgen/acceptancetransition.go, statusgen/protectedclosure.go, statusgen/containerhistory.go, statusgen/ (new test files only, see Task 6), docs/brief-rules.md, docs/protected-paths.md, plugins/assay/skills/verify-desk/SKILL.md, tools/desk/cmd/deskdispatch/worldruntime_test.go — containerhistory.go is read only; the only file under tools/desk/ is that one new test, and no desk-tool code changes (the deskkit facts below are read-only references); docs/brief-rules.md is rule 45 only.

facts:
- `--acceptance-record` is host only. `verifyrun` refuses it with `--in-container`, with `--check` and with cross-repository admission: "statusgen verifyrun: --acceptance-record requires host execution and same-repository admission" (`statusgen/verifyrun.go:2456`-`:2458`; flag text "host only" at `:2174`). The host bracket steps refuse it as well (`:2332`, `:2340`). `docs/protected-paths.md:319`-`:320` refuses transition execution inside `--in-container` and with cross-repository admission; it says nothing about container-netns.
- A protected or transition run does not execute the current Verify table. It executes `protectedExecution.Verify` plus the current author-added rows (`statusgen/verifyrun.go:2562`-`:2575`). That table is the original table read from git at the base, with only the approved columns rewritten (`statusgen/protectedclosure.go:258`; `statusgen/acceptancetransition.go:289`: "Unselected Expect drift retains its original obligation"). So for an unselected Expect change, or an original row the delivery edited, the executed Command or Expect differs from the current file. Every table read in `protectedAcceptanceFor` goes through `historyGit` (`protectedclosure.go:143`, the `read` helper), so a container with no git cannot rebuild that table. verify-desk states today: "The container executes the current table" (`plugins/assay/skills/verify-desk/SKILL.md:263`).
- A transition run needs four host inputs. It needs the forge: `--acceptance-forge` is required (`verifyrun.go:2540`-`:2541`; `statusgen/acceptancetransition.go:252`), and the approval is read with `resolveRulingOn` (`acceptancetransition.go:260`). It needs the origin, through `gitcore.OriginAuthority` (`:239`). It needs git history: the amendment ancestry (`:61`) and its parents. And it binds the full HEAD as its source through `coverageRepo.ResolveCommit("HEAD")` (`verifyrun.go:2655`-`:2665`), refusing "transition execution requires a clean immutable source" (`:2662`). A container started with no network has no forge. When the home is a linked worktree, it also has no git directory.
- darwin has no `unshare --net`. `netnsFacility` refuses on any GOOS other than linux: "the network sandbox uses `unshare --net`, a Linux facility, and this host is" (`statusgen/verifyrun.go:1062`-`:1064`). The non-Linux build of the helper refuses too (`statusgen/netns_other.go:11`). So on a darwin host every check:ci transition row is could-not-run.
- A container-netns run reads the host receipt (`containerAdmission`, `verifyrun.go:2410`; `statusgen/containerbracket.go:357`) and takes its Runner cell from it (`verifyrun.go:2515`, `:2652`-`:2653`). `readBracketReceipt` decodes the receipt with plain `json.Unmarshal` (`containerbracket.go:237`), so an unknown member is ignored, not refused. The open step can already export admitted history as a pack (`--history-out`, `containerbracket.go:570`-`:580`). Today only container-netns `--fail-first` imports that pack, for baseline checkouts, and head rows still run on `--root` (`verifyrun.go:1997`-`:2002`, `:2611`-`:2618`).
- The Linux admission refusal is raised in deskkit. `AdmitWorld` records the host runtime `GOOS-GOARCH` as `World.Inputs.runtime` (`tools/desk/internal/deskkit/preflight.go:592`, `:2088`, `:2261`). `RunWorld.Recheck` compares it (`:2514`-`:2515`) and refuses with "environment changed since admission: runtime — the run is invalid; re-admit it before landing" (`:2527`-`:2529`). `deskdispatch --check-verifier` runs that recheck on every bound run (`tools/desk/internal/deskkit/verifierattestation.go:449`-`:452`). On #1800 a run dispatched from a darwin host and checked on a Linux runner was refused with exactly this message.
- #4299 maps a container bracket's 12-hex tree token to the run's full sha (`transitionWitnessTree`, `acceptancetransition.go:32`-`:43`). It resolves the token with git (`:37`), which a linked-worktree container does not have. Its only caller is inside `acceptanceTransition` (`:195`), so it belongs to host planning: the open and close steps and the lint audit, never the container.

Design-fit: as authorized by the driver's own comment https://github.com/medici-finance/assay/issues/1800#issuecomment-6665625151
the closed bracket of desktools-v2/71 is the network-off route for protected acceptance
rows too. The split follows DR-container-witness: whatever needs the forge, the origin or
git history stays on the host, and the container gets it only as an explicit input bound
into the receipt. The host open step plans the transition, which is the same
`protectedAcceptanceFor` / `acceptanceTransition` call a host run makes today. It writes
the plan into the receipt. The container checks that the plan matches the descriptor and
the bytes it was mounted. The plan includes the planned execution table itself (the
original table with only the approved columns rewritten), because the container has no
git to rebuild it from. The container runs exactly that table's rows, never the current
table's definitions, with no forge and no git. The host close step re-plans, re-reads
the approval, and requires the same plan and the same table byte for byte. Admission
runs only on the dispatch host, at open and at close. That host's runtime is the one the
attestation bound, so the runtime recheck passes there unchanged. It never runs in the
container, and it is not relaxed.

This brief does not build option 1 as the 2026-11-04 finding defines it (a dispatch
path that issues the attestation for the runner's runtime); that is listed out of scope
under `consumers:`. For these rows the bracket makes option 1 unnecessary: no attestation
is checked on the runner, because admission stays on the dispatch host, and the runtime
the rows actually ran on is pinned by the `Container-Image:` digest. Whether that
satisfies the ruling's second part is the driver's choice, recorded under Human decision.
The receipt, `Tree-Digest`, `Admission-Bracket` and credit rules are reused unchanged. A
transition row earns credit only on `credit=credited`, exactly as a non-transition
container row does.

single-point-of-failure: the plan binding. If a container could run a transition whose
plan the host did not compute, run a table other than the planned one, or the close step
could accept a plan or table different from the receipt's, a row amended without human
approval would be credited. It is backed by the tree digest (the brief bytes the plan was
computed from sit inside the digested home), by the planned table's sha256 in the
receipt, by the close step's independent re-plan, byte-exact table comparison, per-row
command comparison and approval re-read, and by Task 8's credit rule of desktools-v2/71,
which is unchanged.

## Human decision

For acceptance at done: the transition bracket below.
The direction is fixed by the driver's own comment https://github.com/medici-finance/assay/issues/1800#issuecomment-6665625151
which reads: "a closed container-netns admission bracket meets the network-off
requirement for protected acceptance-transition rows, and commission the statusgen
change for option 1." Option 1 is the first remedy in the 2026-11-04 finding on the
same tracker: a dispatch path that issues the attestation for the runner's runtime.

The ruling has two parts: (a) the closed bracket meets network-off for these rows, and
(b) commission the statusgen change for option 1. This brief builds (a). It does not
build option 1 as the finding defines it. There are two readings of (b):

1. Recommended: the bracket answers (b) for these rows. Admission stays on the dispatch
   host, no attestation is checked on the runner, and the `Container-Image:` digest pins
   the runtime the rows ran on. Option 1 itself is not built.
2. Alternative: (b) still requires option 1 literally, a deskdispatch admission target
   that binds the runner's runtime at dispatch. That is a separate deskdispatch change,
   out of scope here, and needs its own brief alongside this one.

No default. Reading 1 narrows what was ruled, so silence does not settle it. Before
`in-progress` (rule 48), this brief needs a design-decision record that the driver
approves, and that record must state the driver's explicit choice between reading 1 and
reading 2. The record cites the ruling as the driver's own comment
https://github.com/medici-finance/assay/issues/1800#issuecomment-6665625151
and that comment names neither the record nor this brief's board id, so it cannot
anchor one on its own. The choice fixes only the design the implementer builds
to. It never stands in for the `gate: human` acceptance at done.

## Ground rules

Normal worker admission, one isolated worktree, one draft PR. Tests use fixtures and
fakes: a fake `deskdispatch` and a fake forge client through `transitionClient`. No
container engine and no network. Do not relax `sandboxRefusal`, the loopback precheck,
the CI-lane refusal, `RunWorld.Recheck` or any admission refusal. `--in-container`,
`--check` and cross-repository admission with `--acceptance-record` stay refused. The
implementation PR touches the hermetic witness, which is a security control. It takes
the security lane and needs `Security-Review: pass` before ready. Stop at implemented.

## Task

1. Host open step. `--open-bracket` accepts `--acceptance-record <file>`, plus
   `--acceptance-forge` when the descriptor is a transition, and refuses either without
   the other where today's host run refuses. It plans on the host exactly as a host run
   does today: the `acceptanceRecordKind` / `protectedAcceptanceFor` path (including
   `acceptanceTransition`, its approval read and origin check), and the clean-source and
   full-HEAD check of `verifyrun.go:2655`-`:2665`. The planned source must equal the
   check's binding source. It writes one new receipt member, `acceptance`: the
   descriptor's sha256, the full source sha, the protected run set (row IDs), the
   `Acceptance-Changes` value, the author-added row IDs, and the planned execution
   table with its sha256. The planned table is the exact table text the host run would
   execute for the run set (`protectedExecution.Verify`, the original table with only the
   approved columns rewritten), so each planned row carries its planned Command and
   Expect. `transitionWitnessTree` and every other git read run here, on the host, and
   in the close step and the lint audit, never in the container. Any planning refusal writes
   no receipt and exits non-zero with the host run's message. `--open-bracket` without
   `--acceptance-record` writes byte-identical receipts to today's.
2. Container run. Under `--sandbox=container-netns --admission-receipt`,
   `--acceptance-record` is accepted. `--acceptance-forge` is refused there, because the
   container reads no forge. The run refuses before any row runs unless all of these
   hold: the receipt carries `acceptance`; the descriptor file's sha256 equals it; the
   planned table's sha256 equals the receipt's digest; the planned table's row IDs are
   exactly the receipt's run set; the current Verify section's author-added rows are
   exactly the receipt's author-added IDs; and any `--rows` selection lies inside the
   run set. The history-dependent checks were made at the open, over bytes the tree
   digest binds, and the container does not repeat them. The container executes exactly
   the planned table's rows, with their planned Command and Expect, plus the current
   author-added rows. It never reads a run-set row's definition from the current table:
   for an unselected Expect drift it judges by the original Expect. A receipt that
   carries `acceptance` while the run has no `--acceptance-record` is refused too, so a
   plan is never silently dropped. The execution source is the receipt binding's full
   source, not `ResolveCommit("HEAD")`, and no git process starts (so
   `transitionWitnessTree` is never called here). The witness carries the descriptor, the
   `Acceptance-Changes:` line and `author-added` states exactly as a host transition
   run writes them, then the `Tree-Digest:`, `Container-Image:` and attestation lines of
   desktools-v2/71.
3. Host close step. `--close-bracket` accepts the same `--acceptance-record` and
   `--acceptance-forge`. Before appending, it re-plans on the host (approval re-read
   included, and `transitionWitnessTree` runs here as in a host run) and requires the
   result to equal the receipt's `acceptance` member and the witness's descriptor and
   `Acceptance-Changes:` lines. Its re-planned execution table must equal the receipt's
   planned table byte for byte, and each witness row's command cell must equal that
   row's planned Command (compared as the host transition compares witness commands,
   through `normalizeCommandText`). Any difference, an edited approval comment, or a
   forge read that fails appends nothing and exits non-zero.
   The `Admission-Bracket:` line keeps exactly its desktools-v2/71 form.
4. Runtime. Neither step changes `RunWorld.Recheck`. Rule 45 and verify-desk state the
   consequence. `deskdispatch --check-verifier` runs only on the dispatch host. Running
   it on a different-runtime runner still refuses with
   `environment changed since admission: runtime`, and that is the expected refusal.
   The answer is the bracket, never re-admission inside the runner.
5. Docs. Amend rule 45, the `--acceptance-record` paragraph of verify-desk (`SKILL.md:252`)
   and `docs/protected-paths.md:319`. Each uses the exact phrase "container-netns
   transition bracket" and states that a protected or transition record may run inside
   a closed container-netns transition bracket that was opened and closed with the same
   descriptor; that inside it the container executes the receipt's planned table, not
   the current table; that `--in-container`, `--check` and cross-repository admission
   stay refused; and that credit is unchanged (`credit=credited` only). In verify-desk,
   amend the sentence "The container executes the current table" (`SKILL.md:263`) so it
   holds only outside a transition bracket; that exact sentence no longer appears.
   Regenerate the Cursor mirror.
6. Tests. Write the new named tests in new files: `statusgen/transitionbracket_test.go` (planned)
   and `tools/desk/cmd/deskdispatch/worldruntime_test.go` (planned), which drives the
   recheck through `deskdispatch`'s `--check-verifier` path as `flow_50_test.go` there
   does. Leave
   `containerbracket_test.go`, `containercredit_test.go`, `admittedrowenv_test.go`,
   `acceptancetransition_test.go` and `protectedrecord_test.go` unchanged.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && go test -run '^TestOpenBracketTransition$' -count=1 -v . > "$out" 2>&1 && grep -q -- '--- PASS: TestOpenBracketTransition ' "$out")` | exit 0 and named PASS; fail-first: absent at the base, so the grep fails there. An approved transition descriptor with a fake forge writes a receipt whose `acceptance` member carries the descriptor digest, full source, run set, changes, and the planned execution table with its sha256 (for a fixture with an unselected Expect drift, the planned table holds the original Expect, not the current one); an unapproved, edited or unreadable approval, a non-github origin, an unclean source and a source differing from the check binding each write no receipt and exit non-zero; without `--acceptance-record` the receipt is byte-identical to today's; mutation: skip the approval read at the open, and the named test must fail |
| 2 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && go test -run '^TestContainerNetnsTransition$' -count=1 -v . > "$out" 2>&1 && grep -q -- '--- PASS: TestContainerNetnsTransition ' "$out")` | exit 0 and named PASS; fail-first: absent at the base. End to end, open step, container run, close step: a matching receipt and descriptor run the run set under container-netns with zero `deskdispatch`, zero forge and zero git launches inside the container step (fakes on PATH record the calls) and write the descriptor, `Acceptance-Changes:`, `Tree-Digest:`, `Container-Image:` and attestation lines; the Runner cell's source is the receipt binding's full source; the close step then appends one `Admission-Bracket:` line and `--bracket-digest` reports `verdict=match` with `span-lines=tree-digest:1,container-image:1,attestation:1`; with an unselected Expect drift (the current table edits a run-set row's Expect, the approval does not select it), the container judges that row by the original Expect from the receipt's planned table: output matching only the drifted Expect fails the row; mutation: make the container call `ResolveCommit("HEAD")` again, and the named test must fail; mutation: make the container execute the current Verify table instead of the receipt's planned table, and the named test must fail |
| 3 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && go test -run '^TestTransitionBracketRefusals$' -count=1 -v . > "$out" 2>&1 && grep -q -- '--- PASS: TestTransitionBracketRefusals ' "$out")` | exit 0 and named PASS; fail closed: each of these refuses before any row runs and writes no witness: container-netns `--acceptance-record` with no `--admission-receipt`; a receipt with no `acceptance` member; a descriptor whose digest differs; a planned table whose sha256 differs from the receipt's digest; a planned table whose row IDs differ from the run set; current author-added rows that differ from the receipt's author-added IDs; `--rows` outside the run set; `--acceptance-forge` in the container; a receipt carrying `acceptance` with no `--acceptance-record`; a version 2 receipt; and `--acceptance-record` with `--in-container` or `--check`, which keep today's message. A close step whose re-plan, approval re-read or witness descriptor differs from the receipt appends no `Admission-Bracket:` line, and so does one whose re-planned execution table differs from the receipt's planned table by any byte, or where a witness row's command cell differs from that row's planned Command; mutation: drop the descriptor digest comparison, and the named test must fail; mutation: drop the close step's byte-exact table comparison, and the named test must fail |
| 4 | check:ci | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && for t in TestContainerNetnsRequiresAdmissionReceipt TestContainerNetnsReceipt TestCloseBracket TestBracketCredit TestBracketCreditCrossRepo TestCreditRecordFixtures TestAdmittedRowsStripGitEnv TestAdmittedRowsPinGrep TestAdmittedRowEnvShape TestRowEnvSingleSource TestAdmittedRowsShellEnv TestAdmittedRowsKeepNarrowing TestAdmittedEnvNarrowOnly TestAdmittedRowsDropForgeTokens TestAdmittedEnvDropsForgeTokens TestSandboxPrecheckLoopback TestSandboxPrecheckExtraIface TestSandboxRunRefusedNoExec TestSandboxWitnessRecordsMode TestSandboxCILaneRefuses TestSandboxFlagRefusals TestSandboxOfReadsResultOnly TestAcceptanceTransition TestTransitionExecution TestProtectedRecordRun TestProtectedRecordRefusal TestProtectedRecordCredit TestMixedTransitionProductionFlow; do if ! go test -run "^$t\$" -count=1 -v . > "$out" 2>&1; then exit 1; fi; if ! grep -q -- "--- PASS: $t " "$out"; then exit 1; fi; done)` | exit 0; each of the 28 existing tests is run alone and prints its own named PASS; the receipt, tree-digest, bracket, credit, row-environment, sandbox and host transition rules are unchanged |
| 5 | check | `git diff --exit-code $(git merge-base refs/remotes/origin/main HEAD) -- statusgen/containerbracket_test.go statusgen/containercredit_test.go statusgen/admittedrowenv_test.go statusgen/acceptancetransition_test.go statusgen/protectedrecord_test.go statusgen/testdata/credit-records` | pre-merge, on the implementation branch: exit 0; the existing receipt, credit and transition tests and fixtures are not edited, so row 4 tests today's rules |
| 6 | check:ci +dereference +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && go test ./cmd/deskdispatch/ -run '^TestWorldRecheckRuntime$' -count=1 -v > "$out" 2>&1 && grep -q -- '--- PASS: TestWorldRecheckRuntime ' "$out")` | exit 0 and named PASS; fail-first: absent at the base. Through the `--check-verifier` landing check, a bound run whose admitted world records a runtime other than the checking host's (for example `darwin-arm64` checked under `linux-amd64`) refuses with `environment changed since admission: runtime`, and an unchanged runtime lands; mutation: skip the `WorldRuntime` compare at `preflight.go:2514`, and the named test must fail |
| 7 | check:ci | `(for f in docs/brief-rules.md plugins/assay/skills/verify-desk/SKILL.md docs/protected-paths.md; do if ! grep -q -- 'container-netns transition bracket' "$f"; then exit 1; fi; done; ! grep -q -- 'The container executes the current table' plugins/assay/skills/verify-desk/SKILL.md)` | exit 0; fail-first: at the base no file contains the phrase `container-netns transition bracket` and verify-desk still says "The container executes the current table", so the row exits 1 there. Each of the three files names the transition bracket (Task 5), and verify-desk's old sentence is amended |
| 8 | check:ci | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | exit 0; no Cursor-mirror drift |
| 9 | check | `statusgen --root . --consumers --brief desktools-v2/81 --base $(git merge-base refs/remotes/origin/main HEAD)` | pre-merge, on the implementation branch: exit 0; the declared implementation sites and follow-up references corroborate; out-of-scope judgments stay UNCHECKED and reviewer-owned |
| 10 | check | `statusgen --root . --lint --diff-base $(git merge-base refs/remotes/origin/main HEAD)` | pre-merge, on the implementation branch: exit 0; it reports `0 diff-introduced problem(s)` and promotes no notice to PROBLEM, so the change adds no PROBLEM over the base (PROBLEMs already on the base are demoted to NOTICE and are not this brief's) |

Pre-mortem (threat model):
- A container runs a transition the host never planned, or a different descriptor. Row 3 catches it (receipt, digest and plan checks), and row 1 shows the open step plans only an approved transition.
- The container reaches git or the forge, or falls back to `HEAD`. Row 2 catches it (zero launches; mutation).
- The container runs the current table's Command or Expect instead of the planned one, crediting text the human never approved. Row 2 catches it (unselected Expect drift; the current-table mutation), and row 3 catches a planned table that differs from the receipt's digest or from the close re-plan, and a witness command that differs from the planned Command.
- A plan is dropped silently, so amended rows run as ordinary rows. Row 3 catches it (`acceptance` with no record, and the reverse).
- An approval is edited between open and close. Row 3 catches it (the close-step re-read).
- `--in-container`, `--check` or cross-repository admission loosens while the flags are added. Row 3 keeps today's message.
- The receipt, credit or sandbox rules drift. Rows 4 and 5 catch it.
- Someone "fixes" the Linux runner case by relaxing the runtime recheck. Row 6 catches it.
- Docs that do not name the transition bracket, or that keep saying the container executes the current table. Row 7 catches it. Docs that credit on `verdict=match` alone are review-owned; Task 5 requires `credit=credited` only.
- Plan equality checked against the wrong bytes because the brief was edited after the open. The tree digest of desktools-v2/71 catches it, and row 4 runs those tests; the close step's byte-exact table comparison (row 3) catches a re-plan that moved.

The implementation must create the exact named tests. An empty go-test match is not a PASS.

## Unblocks

| Brief | Rows | What it needs from this brief |
|---|---|---|
| desk-tools/38 | 1-3 | All three pass in a closed bracket. Row 3's command was amended under an approved transition, and that transition planned on the host but could not run there (no `unshare --net`). It needs the transition and rows 1-3 run together inside one closed bracket. |
| desktools-v2/66 | 5 | Rows 1-4 pass in a closed bracket. Row 5's approved transition read the approval (`Acceptance-Changes: 5=Command`) but could not run on the host. It needs that transition run inside the bracket. |
| agent-execution/01 | 4 | Rows 1-3 pass in a closed bracket. Row 4's approved transition could not run on the host, and on a Linux runner `--check-verifier` refused with `environment changed since admission: runtime`. It needs the transition run inside a bracket opened and closed on the dispatch host. |

## Evidence

<!-- Independent postmerge verifier records actual commands, source and outcomes here. -->

## Review

Gate: human. The implementation PR takes the security lane: `Security-Review: pass` is
required before it is ready. Human acceptance of the transition bracket is recorded here
at done.
