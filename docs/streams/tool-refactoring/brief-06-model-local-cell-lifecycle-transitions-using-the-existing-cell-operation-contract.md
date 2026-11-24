---
brief: assay:assay:tool-refactoring:06
title: "Model local cell lifecycle transitions using the existing cell-operation contract"
why: "The reported holder-loss bug already has a source repair. Up, reconcile and down still interpret ownership observations in CLI-local paths; consolidating decision-only logic behind the existing cadence owner prevents a sibling path from reintroducing the class."
wave: 0
depends: []
unblocks: ["tool-refactoring/07"]
effort: M
gate: human
decision-issue: 7066
ruling: "https://github.com/medici-finance/assay/issues/7066#issuecomment-7053556919"
design: DR-cell-lifecycle
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: 9c11fcfe-e4f4-42e7-9695-0b33e634328a
outcome: none
authored: 2026-11-12 by author-brief session
sources:
  - "docs/streams/tool-refactoring/spec.md — draft scope; not implementation or activation approval"
  - "freshness-checked 2026-11-12 @ 680102a0877755feab9ef85dc4e1d3f52a137a54; source inspection and bounded issue/PR snapshots; not a reproduced production failure"
  - "medici-finance/assay#6251 — related class evidence; resolution not asserted"
  - "medici-finance/assay#6170 — related class evidence; resolution not asserted"
  - "medici-finance/assay#6217 — related class evidence; resolution not asserted"
  - "medici-finance/assay#6148 — related class evidence; resolution not asserted"
  - "desk-containers/15 — retained owning work; see existing-work.md"
  - "desk-containers/17 — retained owning work; see existing-work.md"
  - "desk-containers/18 — retained owning work; see existing-work.md"
  - "graph-execution/20 — retained owning work; see existing-work.md"
  - "graph-execution/26 — retained owning work; see existing-work.md"
gate-why: "Changes how a role process is identified as stopped or stale. Human acceptance confirms that pid reuse, owner generation and independent lease/process checks still prevent signaling or settling another owner."
decision-trigger: start
exec-tier: strong
exec-tier-why: "Cross-component class refactoring and negative controls; one local happy-path test cannot prove that every production caller migrated."
domain: complicated
consumers:
  - "tools/desk/internal/cellcadence/lifecycle.go: follow-up tool-refactoring/06 (this brief; source implementation is still owed)"
  - "tools/desk/internal/cellcadence/lifecycle_test.go: follow-up tool-refactoring/06 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/cellctl/down_settle.go: follow-up tool-refactoring/06 (this brief; source implementation is still owed)"
  - "tools/desk/internal/cellcadence/interactive.go: follow-up tool-refactoring/06 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/cellctl/lifecycleplan_test.go: follow-up tool-refactoring/06 (this brief; source implementation is still owed)"
---

# Brief 06 — Model local cell lifecycle transitions using the existing cell-operation contract

## Context

files:
- tools/desk/internal/cellcadence/lifecycle.go
- tools/desk/internal/cellcadence/lifecycle_test.go
- tools/desk/cmd/cellctl/down_settle.go
- tools/desk/internal/cellcadence/interactive.go
- tools/desk/cmd/cellctl/lifecycleplan_test.go
- docs/contracts.md
- docs/library-first.md
- tools/desk/README.md
- changelog/tool-refactoring-06.md

facts (2026-11-12, source `680102a08777`):
- Planned paths above identify new Go/test deliverables and existing starting seams; no out-of-repository files are changed.
- Class: model local cell lifecycle transitions using the existing cell-operation contract; the issue cases in sources nominate this mechanism, not a claim that every old report still reproduces.
- Source repair already present: `down_settle.go` captures Sequence/LastStart and `recordIsOwners` matches the checkpoint after holder removal for issue 6251; preserve it, do not implement it again.
- Existing owner: tools/desk/internal/cellcadence; loopadmin remains process-supervision owner.
- Required reading: [scope](spec.md), [retained work and active PRs](existing-work.md), `docs/library-first.md`, `docs/contracts.md`, and the source paths above. Source inspection precedes any implementation effect.
- All dependency and caller changes are this brief's bounded scope. Retained work: desk-containers/15, desk-containers/17, desk-containers/18, graph-execution/20, graph-execution/26. Completed Evidence and dated sources stay unchanged.

layering: keep pure syntax/state/judgment in its named existing library owner and source, process, credential, forge and persistence effects in adapters. Module boundaries are real: desk-tools may import public statusgen subpackages; statusgen cannot import internal deskkit. No new daemon, effect journal, generic bypass or public command is needed.

single-point-of-failure: a caller can bypass the shared decision — the class guard must detect a second planted caller; independent current-state, custody and conditional-write/process checks still refuse at their own effect boundary. Pure-library tests prove no effect authority.

design-fit:
  owner: tools/desk/internal/cellcadence; loopadmin remains process-supervision owner
  contract: existing cell-op-plan/result and loop-admin-runner-v1 contracts
  retires: ["the duplicated projections or inference paths explicitly migrated in Task; inventory and prove deletion before claiming class retirement"]
  weight: "verbs +0; public flags +0; new bypasses +0; duplicate rule bodies decrease after migration; adapter refusal reasons consolidate without dropping independent checks"
  why-add: "Reuse existing owners and profiles. Add only the typed internal seam/tests required below; record actual source and rule-text deltas in delivery."

## Human decision

Role lifecycle decisions determine which process a cell may settle or restart.
The refactor must preserve the existing owner-generation and lease boundaries.

Options:
1. Approve the decision-only consolidation after the independent owner and process controls pass.
2. Keep the current paths and request a narrower change.

Default if no answer: none — blocks until answered.

## Ground rules

- Isolated branch and draft PR; no merge, release, live infrastructure call or control weakening. Stop at implemented.
- At pickup/resume, record Architecture pickup against the current target, contracts, actual APIs and affected work; adapt only within scope. An active material amendment needs its owner checkpoint.
- Refresh issue/PR status and reuse landed repairs before changing code. A scope refusal is a stop.

## Task

Cell-role/process transitions are distinct from workspace allocating/ready/active/retained/retiring/retired states already owned by `workspace/contract.go`. Consume its allocation generation and lifecycle observations; do not create another workspace state machine, allocator, lease or collector. This brief models only current role up/down/settle behavior.

1. Add a pure local lifecycle transition model to the existing cellcadence owner over explicit holder identity, pid/start identity, checkpoint generation, pause/drain state and observed process result. Reuse existing plan/result schemas and runner identity; do not create a second runtime store or lease manager.
2. Express up/reconcile/down/settle and plan-stale decisions through that model. Preserve the already implemented signalled-owner/checkpoint capture repair as a characterization case; do not infer a different owner from a reused pid. An absent or unreadable observation remains could-not-check, and repeated down/up must not invent a completed transition.
3. Keep checkpoint persistence, process signalling and locks at existing adapters. Preserve compare-and-apply identity and one-shot plan confirmation. Remove only duplicated transition judgment after real caller migration; retain independent process-identity/fencing checks.
4. Bind to the supported current local adapter and existing runner protocol. graph-execution/26–27 retain durable launch supervision and standing-client migration; desk-containers/19–24 retain new operator commands. A local refactor neither implements nor activates those future capabilities.
5. Update the named API/CLI reference and existing semantic-owner row; add the changelog fragment. Keep consumer routing deferred until delivery actually edits each site. Publish the class census and retired-callsite diff.

## Class guard

Enumerate up, reconcile, down and settle transitions and checkpoint writers. Plant a second path that uses a reused pid or changes checkpoint state without the recorded owner generation; reject it independently of its formatted message. Preserve the source-pinned regression cases, show a pre-fix red control and retain the planted positive fixture.

## Interface contract

The existing internal/cellcadence owner owns role-state judgments. Loopadmin retains process supervision, and cellop retains operator plan envelopes. Inputs are explicit observations; outputs are typed decisions/records with unavailable states. Internal refactoring does not change a public schema, numeric exit contract, gate policy or custody boundary without its existing owner approval. Source-only success is not released/adopted capability.

## Verify

Row 4 runs only in a clean isolated checkout of the independently resolved implementation delivery D, with its protected delivery binding checked by the existing owner. There HEAD is D and HEAD^ is its first parent. Do not select current integration HEAD as a convenient substitute. If D or its binding cannot be established, record could-not-check. Other rows keep their admitted source and recorded witness identity.

The named tests below are implementation deliverables, not tests claimed to exist today. Each row requires the named `--- PASS:` line as well as exit 0, so a vacuous selector cannot pass. Class controls plant a second defect site outside the reported instance and prove the guard catches it; retain that positive control. Record the pre-fix red and post-fix green.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && GOWORK=off go test -v ./internal/cellcadence -run '^TestCellLifecycleMatrix$' -count=1` | exit 0; output contains "--- PASS: TestCellLifecycleMatrix"; transition and ownership matrix | check:ci +dereference |
| 2 | `cd tools/desk && GOWORK=off go test -v ./internal/cellcadence -run '^TestCellLifecycleClassGuard$' -count=1` | exit 0; output contains "--- PASS: TestCellLifecycleClassGuard"; reused pid and stale-generation plants | check:ci +mutation +neighbour |
| 3 | `cd tools/desk && GOWORK=off go test -v ./cmd/cellctl -run '^TestCellLifecyclePlanFlow$' -count=1` | exit 0; output contains "--- PASS: TestCellLifecyclePlanFlow"; checkpoint/holder adapter flow | check:ci +flow |
| 4 | `statusgen --root . --consumers --brief tool-refactoring/06 --delivery "$(git rev-parse HEAD)" --base "$(git rev-parse HEAD^)"` | exit 0; zero disproved and zero unchecked consumer routes for the delivered brief | check:ci +flow |

## Definition of Done

Every Task path has an actual caller or documented retention reason, consumer flows and planted controls pass, obsolete duplicate bodies are removed, and docs/reference plus changelog explain the surviving contract. An independent verifier fills Evidence; this brief does not self-award acceptance. No change to historical Verify or Evidence is included.

## Evidence

<!-- Independent verification appends actual commands, exits, output/digests, date and runner after implementation. Empty at authoring. -->

### Non-implementer verifier run: 2026-11-24, container-netns bracket, merged main 034614a8c462b094fa7ec9396c4d70f6e6595a80

The runner is assay-verifier-app[bot] (model claude-opus-5-5, tier strong). It is not the implementer. This is the
first verification of this brief.

Source: 034614a8c462b094fa7ec9396c4d70f6e6595a80. The delivering change is #7076, merged as
efe590fbbbb05d2251adca8353435d873e91beb4. Its first parent, 64b655c6c009e8153b161cc0dbed397935730653, is the
fail-first base. The delivery is on the source's history. This brief's file, and every delivered code, test and
plant file, is byte-identical at the delivery and at the source. Only shared documentation and the changelog
(the fragment was folded into v1.0.91) changed since.

The brief answers sensitive-data: yes, so every row is risk-bearing and owes a fail-first witness. The original
table is the four rows above: `deskpathguard rederive` reports 4 from-base rows and 0 author-added rows.

This is DISPATCH-FOR-EVIDENCE. The run gathers Evidence and a draft outcome record only. The desk never flips this
brief's status: the human's merge of the Evidence PR is the flip.

Admission: `deskdispatch --check-verifier --root <home> --brief docs/streams/tool-refactoring/brief-06-model-local-cell-lifecycle-transitions-using-the-existing-cell-operation-contract.md`
exited 0 for this detached home at 2026-11-24T16:28:44Z, before any row ran. It returned attestation #7556,
run=60599cbb2e397d6da67838690609b3e036879439cec5019c411f8caa43b709ad and source 034614a8c462, with desk tools
v1.0.92 on darwin arm64 and git 2.56.1.

Every row ran under the declared verifier Git environment: all GIT_* variables and SSH_ASKPASS were unset, then
GIT_CONFIG_GLOBAL=/dev/null, GIT_CONFIG_NOSYSTEM=1, GIT_CONFIG_COUNT=0 and GIT_TERMINAL_PROMPT=0 were set, along
with KUBECONFIG=/dev/null.

Rows 1-3 ran in the container-netns admission bracket:

- On the host, `statusgen verifyrun --open-bracket` wrote the receipt, a history pack and a fresh module cache
  outside the home. It opened at 16:32:12Z.
- The container ran verify-runner v1.0.92, sha256:d3d958719dea (go1.27.4), under `--network none --read-only`
  with the home mounted read-only, an exec-enabled tmpfs scratch, GOPROXY=off and GOTOOLCHAIN=local. It ran
  `statusgen verifyrun --sandbox=container-netns --rows 1,2,3 --fail-first --delivering-commit efe590fbbbb0 --base-log <dir> --history-pack <pack>`.
- On the host, `statusgen verifyrun --close-bracket` exited 0 and closed it at 16:39:48Z.
- `statusgen verifyrun --bracket-digest` exited 0 with verdict=match (witness sha256:97d6012caf1b).

Row 4 was outside the `--rows 1,2,3` selection (reasons below).

One attempt ran inside this bracket, 16:38:00Z-16:39:15Z. It exited 1. It recorded rows 1-3 as passing at the
head and as "non-discriminating — green at base 64b655c6c009". The witness below is that attempt.

Rows 1-3: could-not-run, Expect-reader defect (#7389, in the #1702 class).

The witness records each row as pass exit=0 at the head. Its fail-first result reads each row as
non-discriminating, green at base, and the run exits 1. Neither reading observed the row's property. Each
Expect requires the named "--- PASS:" line as well as exit 0, and verifyrun does not read an "output contains"
Expect: it decides on exit status alone (#7389).

The retained base logs show what the base actually printed:

- Rows 1 and 2, sha256:0458878a898e and sha256:615a38fecc35: "testing: warning: no tests to run", "PASS" and
  "ok github.com/medici-finance/assay/tools/desk/internal/cellcadence [no tests to run]".
- Row 3, sha256:64b6602c8123: the same three lines for tools/desk/cmd/cellctl.

None of them prints a "--- PASS:" line. The three named tests do not exist at the base: git grep finds
TestCellLifecycleMatrix, TestCellLifecycleClassGuard and TestCellLifecyclePlanFlow at the delivery and at the
source, and none of them at 64b655c6c009. Read against its Expect, each base run is the pre-fix red, and each row
would discriminate. The green-at-base reading is the reader's artefact, not a property of the rows.

The rows are classed could-not-run, never fail, under reviewer-lane practice:
https://github.com/medici-finance/assay/pull/7228#pullrequestreview-5979645423 and
https://github.com/medici-finance/assay/pull/7230#pullrequestreview-5979708866. The witness and its exit 1 still
record them as non-discriminating.

Row 4: could-not-run, check-definition (#6978).

The row must run in a clean isolated checkout at the delivery D, where HEAD is D. The admitted home is detached at
034614a8c462, and the brief forbids substituting it. verifyrun has no admitted lane for a row that needs a
separate checkout at the delivery head (#6978), so the row was left out of the bracket's selection. It has no
acceptance witness.

Supplemental probes. These carry no acceptance credit and were run after the bracket closed:

- Head output. The three row commands ran verbatim at the source in the same image, network none, home read-only.
  Each exited 0, printed no "--- FAIL" line, and printed its named line:
  - "--- PASS: TestCellLifecycleMatrix", with subtests process_identity, record_class, owner_generation, settle,
    plan_observation and checkpoint_writer passing.
  - "--- PASS: TestCellLifecycleClassGuard", with subtests "positive control: planted second sites" and
    "production tree has no second site" passing.
  - "--- PASS: TestCellLifecyclePlanFlow", with its four subtests passing: preconditions re-measured identically,
    a reused pid reads dead and is never signalled, a plan goes stale when the owner runs again, and settle
    finishes only the owner generation down signalled.
- Row 4 with explicit commits. On the host, in the home and not in a checkout at D, the consumers gate ran with
  `--delivery efe590fbbbb0 --base 64b655c6c009` in place of the row's HEAD and HEAD^ substitutions. It exited 0
  and printed "summary: 5 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing" over the immutable
  delivered snapshot. Each of the five entries is a self-route carried by this diff: lifecycle.go,
  lifecycle_test.go, down_settle.go, interactive.go and lifecycleplan_test.go. The probe left the home clean. It is
  not the row as written, because the row's checkout is not D.

Risk-bearing values. Every literal and predicate in the delivery's non-test diff was enumerated:

- The owner-attribution predicate Generation.Attributes @ tools/desk/internal/cellcadence/lifecycle.go:135
  (start match while the owner record survives; otherwise Sequence != 0 with the same sequence, pid and start).
- The checkpoint-writer guard Generation.OwnsUnfinished @ lifecycle.go:143-146, including the `g.Sequence == 0 ||`
  branch, with Generation.Named @ lifecycle.go:123.
- The legacy unscoped lease-holder acceptance in OwnsCheckpointPID @ lifecycle.go:107.
- The settle refusal on an unidentified owner, `in.Owner.PID == 0` @ lifecycle.go:210, and the retry on any class
  but dead @ lifecycle.go:219.
- The class labels owned, dead, alive and could-not-check @ lifecycle.go:25-28, the plan kinds up, stale and
  absent @ lifecycle.go:229-231, `ProcessGone ProcessVerdict = iota + 1` @ lifecycle.go:45, and
  `downStoppedSummary = "stopped by cellctl down"` @ tools/desk/cmd/cellctl/down_settle.go:28.

The first three rank first, because they decide whose checkpoint a teardown may finish and so whether one owner's
record can be closed out by another's stop. The labels and the summary string are reversible names with no
authority, and rank last.

RISK-VALUE: DERIVED — Generation.Attributes @ tools/desk/internal/cellcadence/lifecycle.go:135 — a byte-for-byte move of the base's recordIsOwners @ tools/desk/cmd/cellctl/down_settle.go:186 at 64b655c6c009, whose comment derives it from #6251 (a clean exit removes the owner record, so the read checkpoint binds the owner; a zero, unread sequence never matches).
RISK-VALUE: DERIVED — Generation.OwnsUnfinished `g.Sequence == 0 ||` @ tools/desk/internal/cellcadence/lifecycle.go:146 with Named @ :123 — the base writer Lease.RecordStopped @ tools/desk/internal/cellcadence/interactive.go:101 at 64b655c6c009 accepted any running interactive record with pid > 0 and a matching pid; the new guard keeps that check and adds Named and, when the checkpoint was read, a sequence and start match, so it is never weaker than the base. The down path reaches the writer only after JudgeSettle has required Attributes.
RISK-VALUE: DERIVED — legacy unscoped holder acceptance `holder.Cell == "" && holder.Role == ""` @ tools/desk/internal/cellcadence/lifecycle.go:107 — carried verbatim from the base's reconcile.go:418 at 64b655c6c009; it widens nothing.

Tool reads after the Evidence was written:

- `statusgen verifyrun --check` exited 2 and printed "3 pass, 0 fail, 1 could-not-run/missing (of 4 Verify rows);
  0 author-added with no acceptance credit". It reports rows 1-3 as "pass — witness matches the row and passed"
  and row 4 as "could-not-run — no execution witness in Evidence".
- The pass that `--check` gives rows 1-3 rests on the witness's exit-status reading (#7389). The same witness's
  fail-first run exited 1, reading those rows as non-discriminating. So this entry does not count rows 1-3 as
  passing. It classes them could-not-run for the reasons above.
- `statusgen verifyrun --bracket-digest --witness` on this brief exited 0 with verdict=match (bracket=L275,
  witness sha256:97d6012caf1b).
- With `--credit --source 034614a8c462`, it exited 1 and printed credit=not-credited: "L266 differs from HEAD; an
  uncommitted line has no author".
- Post-edit admission: the same `deskdispatch --check-verifier` ran again at 2026-11-24T16:43:14Z and exited 0
  with the same binding: run=60599cbb2e397d6da67838690609b3e036879439cec5019c411f8caa43b709ad, source
  034614a8c462, model claude-opus-5-5, tier strong, attestation #7556.

Tally: 0 of 4 rows pass with credit. Rows 1-4 are could-not-run. No credited row failed, and no probe observed an
obligation failing.

**VERIFY: BLOCKED** — 0/4 rows pass with credit at 034614a8c462; no credited row failed and no probe saw a failure. Rows 1-3 are could-not-run: verifyrun does not read their "output contains --- PASS" Expects, so its fail-first reads a base where the named tests do not exist as green (#7389). Row 4 is could-not-run because it needs a checkout at the delivery and has no admitted lane (#6978). Supplemental, no credit: the head prints each named PASS line, and the consumers gate pinned to the delivery reports 5 corroborated, 0 disproved, 0 unchecked.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `cd tools/desk && GOWORK=off go test -v ./internal/cellcadence -run '^TestCellLifecycleMatrix$' -count=1` | pass exit=0 sandbox=container-netns | sha256:e852bcceb371 | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=64b655c6c009 green rc=0 · head=034614a8c462 · delivering=efe590fbbbb0 output=sha256:0458878a898e2ba0bef3612b5c247e276e9af6271e4d4b3eaada4de7fd0b575d log=row-000001.log |
| 2 | `cd tools/desk && GOWORK=off go test -v ./internal/cellcadence -run '^TestCellLifecycleClassGuard$' -count=1` | pass exit=0 sandbox=container-netns | sha256:744ededde509 | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=64b655c6c009 green rc=0 · head=034614a8c462 · delivering=efe590fbbbb0 output=sha256:615a38fecc35b4de3c9270a91c6f62a3ca7d91f55753e7361c48505fc2a2e126 log=row-000002.log |
| 3 | `cd tools/desk && GOWORK=off go test -v ./cmd/cellctl -run '^TestCellLifecyclePlanFlow$' -count=1` | pass exit=0 sandbox=container-netns | sha256:602174e83471 | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=64b655c6c009 green rc=0 · head=034614a8c462 · delivering=efe590fbbbb0 output=sha256:64b6602c8123e31931599ebbe04e4b7cc8471dece3aad8a07a56c716993a360f log=row-000003.log |
Tree-Digest: sha256:0d2fc44c1c8d265eb7dca088c90ea5f3c8585cc0ebd893ea66e55cd0a81eb622
Container-Image: sha256:d3d958719dea47fc611e096612c1478bcc07376b7f9783f92433ac8faeeee380
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7556 run=60599cbb2e397d6da67838690609b3e036879439cec5019c411f8caa43b709ad source=034614a8c462b094fa7ec9396c4d70f6e6595a80 brief=docs/streams/tool-refactoring/brief-06-model-local-cell-lifecycle-transitions-using-the-existing-cell-operation-contract.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=60599cbb2e397d6da67838690609b3e036879439cec5019c411f8caa43b709ad source=034614a8c462b094fa7ec9396c4d70f6e6595a80 tree=sha256:0d2fc44c1c8d265eb7dca088c90ea5f3c8585cc0ebd893ea66e55cd0a81eb622 witness=sha256:97d6012caf1b338e96a038898a97dbff9d5cb23447de13aec12577f1da8d28f6 opened=2026-11-24T16:32:12Z closed=2026-11-24T16:39:48Z before=receipt after=check-verifier

## Review

Review the named lower-boundary refusals, all migrated callers and module/import constraints, not only the helper. Pre-mortem: a helper can exist without being called (flow row); two copies can agree on an unsafe result (mutation/neighbor row); incompatible records can strand adopters (joined qualification in /11). Scope adequacy and unenumerated consumers remain review obligations.
