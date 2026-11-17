---
brief: assay:assay:desktools-v2:83
title: Admit acceptance transitions for cross-repository briefs
why: "A brief tracked in one repository and delivered in another cannot run an approved acceptance transition on any lane, because `--acceptance-record` refuses every cross-repository (version 2) admission. Once such a brief's protected Verify table is amended under human approval, the transition itself can never be credited. Post-merge fail-first runs for such a brief still wait on #2721, so this brief alone does not bring every one of them to verified."
wave: 2
depends: ["desktools-v2/81"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
gate-why: "It relaxes a verification control that adopters' protected tables rely on: after this lands, an approved acceptance transition may be credited for a brief whose table lives in one repository and whose code lives in another. A defect here would credit a row against text no human approved, or against a source the admission did not bind. The driver approves the design record before work starts, and accepts the change at done."
decision-trigger: creation
decision-issue: 6701
ruling: "https://github.com/medici-finance/assay/issues/6701#issuecomment-6932269450"
issues: [6697]
schema: brief-v2
version: 1
authored: 2026-11-16 by worker-desk (authoring for #6697)
sources:
  - "#6697 — the scoping issue: where `--acceptance-record` refuses version 2 admission today, what a fix must keep, and the refusals it needs"
  - "#6701 — the decision issue; ruled option 1 (extend both lanes)"
  - "docs/streams/desktools-v2/brief-81-run-acceptance-transition-rows-inside-a-closed-container-netns-bracket.md — the same-repository transition bracket this brief extends; it lists version 2 receipts as out of scope"
  - "docs/streams/decisions/DR-transition-netns.md — the host/container split for transition planning"
  - "docs/streams/decisions/DR-cross-repo-witness.md — a proposed, unratified record for binding the tracking tree in the version 2 bracket; read for context, not cited as this brief's design"
  - "#5544 (open) and #5550, #5553 (merged under the driver's ruling A on #5544) — the cross-repository fail-first base is bound to exactly the deliverable merge-base and refuses otherwise; adjacent, not a dependency"
  - docs/protected-paths.md (§Explicit human-approved acceptance transitions)
  - docs/brief-rules.md (rules 45 and 48)
  - "statusgen/rulingrecord.go — `statusgen --corroborate --ruling-record`, the network check row 10 uses to dereference the design approval"
  - freshness-checked 2026-11-16 @ 6a13cb4a8 (origin/main)
exec-tier: strong
exec-tier-why: "It moves an approval-bound security control across the cross-repository admission boundary: the two-repository plan binding, the coordinate semantics and the fail-closed paths each need adversarial review, and a mistake credits unapproved acceptance."
domain: complicated
outcome: none
consumers:
  - "statusgen/verifyrun.go (the `--acceptance-record` admission refusal), statusgen/containerbracket.go (`hostAcceptance`, `containerAcceptance`, the receipt `acceptance` member), statusgen/protectedclosure.go and statusgen/acceptancetransition.go (planning for a brief outside the home): follow-up desktools-v2/83"
  - "statusgen/transitioncrossrepo_test.go (new file) and the removal of the `version2` case from `TestTransitionBracketRefusals` in statusgen/transitionbracket_test.go: follow-up desktools-v2/83"
  - "docs/brief-rules.md rule 45, docs/protected-paths.md (the transition-execution paragraph), plugins/assay/skills/verify-desk/SKILL.md (the `--acceptance-record` paragraph) and the generated Cursor mirror: follow-up desktools-v2/83"
  - "`--in-container`, `--check`, and (under version 2 admission) `--fail-first` with `--acceptance-record`: out-of-scope (stay refused)"
  - "fail-first base selection and history-pack import for version 2 briefs: out-of-scope (delivered by #5550 and #5553)"
  - "row retirement, class or shell changes, row add, delete or renumber, and table-layout migration: out-of-scope (stay refused)"
---

# Brief 83 — Admit acceptance transitions for cross-repository briefs

## Context

files: statusgen/verifyrun.go, statusgen/containerbracket.go, statusgen/protectedclosure.go, statusgen/acceptancetransition.go, statusgen/transitioncrossrepo_test.go (new), statusgen/transitionbracket_test.go (removal of the `version2` case only), docs/brief-rules.md (rule 45 only), docs/protected-paths.md, plugins/assay/skills/verify-desk/SKILL.md, .cursor/skills/verify-desk/SKILL.md (generated mirror).

facts (main `6a13cb4a8`):
- The host run refuses outright. `statusgen/verifyrun.go:2668`-`:2671`: with `--acceptance-record`, any of `--in-container`, `--check` or an admitted deliverable home (`admittedHome != ""`, which is how a version 2 run is admitted) exits could-not-run with "statusgen verifyrun: --acceptance-record requires host execution and same-repository admission".
- The bracket refuses at both ends. `hostAcceptance` (`statusgen/containerbracket.go:1382`) returns the same message when the check binding's version is not 1 (`:1387`-`:1388`). `containerAcceptance` (`:1429`) refuses "container acceptance refused; no row ran: --acceptance-record requires host execution and same-repository admission" on the same condition (`:1438`-`:1439`).
- The version 2 bracket already exists for rows that do not need a transition. `crossRepoBindingProblem` (`statusgen/containerbracket.go:357`) requires two different repositories (`binding.Repo`, the tracking repository, and `binding.SourceRepo`), a 40-hex `source` and `tracking-source`, the brief outside the home, and the brief's bytes equal to the binding's `BriefSHA256` (`:383`-`:384`).
- `hostAcceptance` binds the plan to the home's source: `coverageRepo{root: home}.ResolveCommit("HEAD")` must be clean and equal `binding.Source`, or it refuses "transition execution requires a clean immutable source" / "acceptance source differs from the host check binding". It binds no tracking commit.
- Planning reads the brief's own repository. `protectedAcceptanceFor` (`statusgen/protectedclosure.go:89`) parses `Verification-Table: delivery=<sha> base=<sha> original=<sha>` and resolves all three with `deliveryCoordinates(dir, ...)` (`:322`), where `dir` is the brief's directory: `delivery` must be an ancestor of that repository's HEAD. For a version 2 brief that is the tracking checkout. The approval read in `acceptanceTransition` takes its repository from `gitcore.OriginAuthority(root)` of the brief's repository (`statusgen/acceptancetransition.go:447`-`:458`), requires only a github.com origin, and writes that repository into the approval payload's `repo=`. Nothing compares it with the version 2 binding's `binding.Repo`.
- So today a version 2 plan has three unresolved questions: which repository each `Verification-Table` coordinate names (the table and its history live in the tracking repository; the delivered code lives in the deliverable repository), which commit of each repository the receipt binds, and which repository the approval must come from.
- `docs/protected-paths.md` states: "Execution with `--acceptance-record` and `--in-container`, `--check` or cross-repository admission remains unsupported and refuses."
- Adjacent: #5544 asked for fail-first base selection and history-pack import on version 2 briefs, and is still open. #5550 and #5553 merged both under the driver's ruling A on #5544 (`failFirstCrossRepoBaseRev`, `statusgen/failfirst.go:487`; `importCrossRepoHistory`, `statusgen/containerhistory.go:76`). The security property they deliver is the bound: a cross-repository fail-first base must be exactly the deliverable merge-base of HEAD and origin/main, and every other case refuses before any row runs (`statusgen/failfirst.go:471`-`:513`). Container and post-merge cross-repository fail-first runs still refuse until #2721 binds the base to the delivering change. On the container lane, protected execution already refuses `--fail-first` (`statusgen/verifyrun.go:2554`-`:2556`). On the host lane, `--fail-first` combines with `--acceptance-record` today (`:2584` refuses only `--check` and `--in-container`); this brief keeps that combination refused under version 2 admission (Task 2).
- Known environment class: under the admitted row environment, git fixtures for acceptance transitions refuse "injected Git configuration is unsupported" (#2715). desktools-v2/81's container witness was blocked by it. New fixtures here meet the same class.

Design-fit: this extends desktools-v2/81's transition bracket and the host lane to version 2 receipts. It keeps DR-transition-netns's split: whatever needs the forge, the origin or git history stays on the dispatch host, and the container gets it only as an explicit input bound into the receipt. For a version 2 brief that host input comes from the tracking checkout. The planned execution table, the descriptor digest and the approval are computed there, and the receipt binds the tracking commit and brief digest alongside the deliverable source. The container still reads no git and no forge. It runs the receipt's planned table in the deliverable home. Rule 48 applies: this brief may not leave `todo` until an approved design-decision record is cited in `design:`. No approved record covers it today (see Design approval required, below).

single-point-of-failure: the two-repository plan binding. If a container could run a plan whose tracking brief, tracking commit or deliverable source the host did not bind, an approval could be read from a repository other than the attested tracking repository, or the close step could accept a plan computed against a different tracking commit or an approval that no longer stands, a row amended without human approval would be credited, or credited against code the approval never saw. It is backed by the version 2 binding's `Repo`, `BriefSHA256` and `tracking-source`, by the receipt's descriptor and planned-table digests, by the close step's independent re-plan and approval re-read, and by desktools-v2/71's credit rule, which is unchanged.

## Human decision

Decide before work starts: how an approved acceptance transition is admitted for a cross-repository brief.

1. **Recommended: extend both lanes.** The host lane (`admittedHome` set) and the closed container-netns transition bracket both accept `--acceptance-record` for a version 2 receipt. The dispatch host plans from the tracking checkout: brief history, `Verification-Table` coordinates and the approval read all resolve in the tracking repository. The receipt's `acceptance` member also binds the tracking commit and brief digest, and these must equal the version 2 binding's. Rows still run only in the deliverable home. A new design-decision record states which repository each coordinate names, and the driver approves it before `in-progress`.
2. **Host lane only.** Admit version 2 transitions on the host lane and keep the version 2 bracket refused. This change is smaller. It does not help a host without `unshare --net`, where every check:ci row of such a brief stays could-not-run.
3. **No mechanism change.** Keep the refusal. Each affected brief is reshaped or split so that its amended rows sit in a same-repository brief (DR-cross-repo-witness option 3). Every such change is itself a reviewed Verify-table change, and the next cross-repository brief with an approved transition reopens the gap.

Ruled: option 1, https://github.com/medici-finance/assay/issues/6701#issuecomment-6932269450

The ruling fixes only the design the implementer builds to. It does not approve the design record below, and it never stands in for the `gate: human` acceptance at done.

### Design approval required

Option 1 requires a new design-decision record, `DR-crossrepo-transition`, and the driver's approval of its content before this brief moves to `in-progress`. It is not approved yet, so this brief carries no `design:` key.

- **What the record states.** For a version 2 brief: which repository each `Verification-Table` coordinate (`delivery`, `base`, `original`) names; which commit of each repository the receipt binds (deliverable `source`, `tracking-source`, brief digest); and that the approval repository is `binding.Repo`, the tracking repository the binding attests. A tracking checkout whose `origin` names any other repository refuses.
- **How it is approved.** The driver comments from the driver's own account on a decision issue that carries this brief's decision-gate marker (#6701 does). The comment names both the record id and this brief's board id, and states the coordinate rule. The record's `ruling:` links that comment. Only then is `design: DR-crossrepo-transition` added to this brief.
- **Who may not approve it.** The implementer, a desk, or any App identity. An agent may draft the record as a proposal marked unratified, but cannot supply the ruling. A record stamped `decided-by: human:<name>` with no ruling comment from a mapped human does not pass row 10.
- **When it is checked.** Before dispatch (rule 48, at the todo to in-progress transition), and again at verify time by row 10, which resolves the ruling through the forge.

## Ground rules

Normal worker admission, one isolated worktree, one draft PR. Do not start until `design: DR-crossrepo-transition` is cited and its ruling stands (Design approval required, above). Tests use fixtures and fakes: a fake `deskdispatch` and a fake forge client through `transitionClient`. No container engine and no network. Do not relax `sandboxRefusal`, the loopback precheck, the CI-lane refusal, `RunWorld.Recheck`, `crossRepoBindingProblem` or any admission refusal. `--in-container` and `--check` with `--acceptance-record` stay refused with today's message. The implementation PR touches the hermetic witness, which is a security control: it takes the security lane and needs `Security-Review: pass` before ready. The change must be released and pinned before an independent verifier uses it for a real brief. Stop at implemented.

## Task

Written for option 1, as ruled.

1. Design record. Confirm `DR-crossrepo-transition` is approved as Design approval required states and is cited in `design:`. If it is not, stop: the implementer does not write, stamp or approve it. Build to the record. It decides which repository each coordinate names, what the receipt binds, and that the approval repository is `binding.Repo`.
2. Host lane. Under version 2 admission, `--acceptance-record` (plus `--acceptance-forge` for a transition) is accepted on the host. Planning runs against the tracking checkout that holds the brief. The approval repository is `binding.Repo`. The tracking checkout's `origin` authority must name that same repository (case-insensitive owner/name), or the run refuses. The approval payload's `repo=` must equal it, and its `brief=` must name the tracking brief. Rows run only in the admitted deliverable home, whose HEAD must be clean and equal the binding's `source`. The tracking checkout's HEAD must equal the binding's `tracking-source`. The brief's bytes must equal `BriefSHA256` before and after planning. `--fail-first` with `--acceptance-record` refuses under version 2 admission, as protected container execution already refuses it.
3. Bracket open. `hostAcceptance` accepts a version 2 binding. It plans as Task 2 does, including the `binding.Repo` comparison, and writes the `acceptance` member with two extra fields: the tracking commit and the brief digest. Each must equal the binding's. A version 1 receipt's `acceptance` member stays byte-identical to today's.
4. Container run and close. `containerAcceptance` accepts a version 2 receipt only when the member's tracking fields equal the binding's, in addition to every check desktools-v2/81 makes. The container still reads no git and no forge. The close step re-plans from the tracking checkout, re-reads the approval from `binding.Repo`, and requires the same plan, table bytes and tracking fields. A close step whose approval re-read is unapproved, edited or unreadable appends no `Admission-Bracket:` line.
5. Refusals. Each of these refuses at the open or in the container before any row runs, and writes no witness:
   - a deliverable source that differs from the binding;
   - a tracking commit or brief digest that differs from the binding;
   - a dirty deliverable home or tracking checkout;
   - a tracking checkout whose `origin` names a repository other than `binding.Repo`;
   - an approval whose payload `repo=` or `brief=` names the deliverable repository or any repository other than `binding.Repo`;
   - an unapproved, edited or unreadable approval, an approval by a bot or unmapped login, and an approval on an issue without the `acceptance-transition: <brief>` marker;
   - a descriptor or planned-table digest mismatch;
   - a version 2 `acceptance` member missing its tracking fields;
   - `--fail-first` with `--acceptance-record` under version 2 admission.

   At the close step, which runs after the rows, each of these appends no `Admission-Bracket:` line and credits nothing: a re-planned tracking commit that differs from the receipt's, and an approval re-read that is unapproved, edited or unreadable.
6. Docs. Amend rule 45, the verify-desk `--acceptance-record` paragraph and `docs/protected-paths.md`. Each uses the exact phrase "cross-repository transition admission" and states that cross-repository admission may carry an acceptance record, and that `--in-container` and `--check` still refuse. Regenerate the Cursor mirror.
7. Tests. Put the new named tests in `statusgen/transitioncrossrepo_test.go` (planned). The only change to `statusgen/transitionbracket_test.go` is removing the `version2` case from `TestTransitionBracketRefusals`: the `"version2"` entry in its name list and its `case "version2":` arm, nothing else (row 5 checks this mechanically). The new file asserts the version 2 behaviour instead. Leave `containerbracket_test.go`, `containercredit_test.go`, `containercrossrepo_test.go`, `acceptancetransition_test.go`, `protectedrecord_test.go` and `mixedtransitionflow_test.go` unchanged.

## Verify

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +mutation | tree | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && go test -run '^TestCrossRepoHostTransition$' -count=1 -v . > "$out" 2>&1 && grep -q -- '--- PASS: TestCrossRepoHostTransition ' "$out")` | exit 0 and named PASS; fail-first: absent at the base. A version 2 host run with an approved transition plans from the tracking checkout, reads the approval from `binding.Repo` on the fake forge, and runs the planned rows in the deliverable home. For an unselected Expect drift, it judges by the original Expect. Mutation: plan against the deliverable home instead of the tracking checkout, and the named test must fail |
| 2 | check:ci +flow +mutation | tree | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && go test -run '^TestCrossRepoBracketTransition$' -count=1 -v . > "$out" 2>&1 && grep -q -- '--- PASS: TestCrossRepoBracketTransition ' "$out")` | exit 0 and named PASS; fail-first: absent at the base. End to end for a version 2 receipt (open, container run, close): zero `deskdispatch`, forge and git launches inside the container step; the receipt's `acceptance` member carries the tracking commit and brief digest equal to the binding's; the close step appends one `Admission-Bracket:` line and `--bracket-digest` reports `verdict=match`. Mutation: drop the tracking-commit comparison in the container, and the named test must fail |
| 3 | check:ci +mutation | tree | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && go test -run '^TestCrossRepoTransitionRefusals$' -count=1 -v . > "$out" 2>&1 && grep -q -- '--- PASS: TestCrossRepoTransitionRefusals ' "$out")` | exit 0 and named PASS. Each Task 5 open or container case refuses before any row runs and writes no witness, including: a tracking `origin` other than `binding.Repo`; an approval whose `repo=` or `brief=` names the deliverable repository; an approval by a bot login; an edited approval; an approval on an issue without the `acceptance-transition: <brief>` marker; and `--fail-first` with `--acceptance-record` under version 2 admission. `--acceptance-record` with `--in-container` or `--check` keeps today's message. At the close step, after the container ran the rows: a re-planned tracking commit that differs from the receipt's appends no `Admission-Bracket:` line, and an approval re-read that comes back unapproved, edited or unreadable appends no `Admission-Bracket:` line. Mutation: drop the `BriefSHA256` re-check after planning, and the named test must fail; mutation: accept a dirty tracking checkout, and the named test must fail; mutation: take the approval repository from the tracking checkout's `origin` without comparing it with `binding.Repo`, and the named test must fail; mutation: skip the close-step approval re-read, and the named test must fail |
| 4 | check:ci | tree | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && for t in TestOpenBracketTransition TestContainerNetnsTransition TestTransitionBracketRefusals TestContainerNetnsReceipt TestCloseBracket TestBracketCredit TestBracketCreditCrossRepo TestAcceptanceTransition TestTransitionExecution TestProtectedRecordRun TestProtectedRecordRefusal TestProtectedRecordCredit TestMixedTransitionProductionFlow; do if ! go test -run "^$t\$" -count=1 -v . > "$out" 2>&1; then exit 1; fi; if ! grep -q -- "--- PASS: $t " "$out"; then exit 1; fi; done)` | exit 0; each of the 13 existing tests runs alone and prints its own named PASS; same-repository transitions, receipts and credit are unchanged |
| 5 | check | host | `(b=$(git merge-base refs/remotes/origin/main HEAD) && git diff --exit-code "$b" -- statusgen/containerbracket_test.go statusgen/containercredit_test.go statusgen/containercrossrepo_test.go statusgen/acceptancetransition_test.go statusgen/protectedrecord_test.go statusgen/mixedtransitionflow_test.go && d=$(mktemp -d) && trap 'rm -rf "$d"' 0 && git show "$b:statusgen/transitionbracket_test.go" > "$d/base" && awk 'skip > 0 { skip--; next } /case "version2":/ { skip = 3; next } { sub(/"dropped", "version2", /, "\"dropped\", "); print }' "$d/base" > "$d/allowed" && if cmp -s "$d/base" statusgen/transitionbracket_test.go; then exit 0; else cmp -s "$d/allowed" statusgen/transitionbracket_test.go; fi)` | pre-merge, on the implementation branch: exit 0. The 6 files holding row 4's other tests are byte-identical to the merge-base, and `statusgen/transitionbracket_test.go` (which holds `TestOpenBracketTransition`, `TestContainerNetnsTransition` and `TestTransitionBracketRefusals`) is either unchanged or differs only by the removal of the `version2` case (Task 7). Any other edit to those 7 files, including a weakened assertion in a same-repository test, exits 1. Mutation evidence is in the authoring PR (#6700) |
| 6 | check:ci | tree | `(for f in docs/brief-rules.md plugins/assay/skills/verify-desk/SKILL.md docs/protected-paths.md; do if ! grep -q -- 'cross-repository transition admission' "$f"; then exit 1; fi; done)` | exit 0; fail-first: at the base no file contains the phrase, so the row exits 1 there. Each of the three files names cross-repository transition admission (Task 6) |
| 7 | check:ci | tree | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | exit 0; no Cursor-mirror drift |
| 8 | check | host | `statusgen --root . --consumers --brief desktools-v2/83 --base $(git merge-base refs/remotes/origin/main HEAD)` | pre-merge, on the implementation branch: exit 0; the declared sites and follow-up references corroborate; out-of-scope judgments stay UNCHECKED and reviewer-owned |
| 9 | check | host | `statusgen --root . --lint --diff-base $(git merge-base refs/remotes/origin/main HEAD)` | pre-merge, on the implementation branch: exit 0 with `0 diff-introduced problem(s)` |
| 10 | check +dereference | host | `statusgen --root . --corroborate --ruling-record docs/streams/decisions/DR-crossrepo-transition.md --ruling-brief docs/streams/desktools-v2/brief-83-admit-acceptance-transitions-for-cross-repository-briefs.md` | exit 0 and a `CORROBORATED for desktools-v2/83` line: the design record's `ruling:` resolves on the forge to an unedited comment by a mapped human, on a decision issue for both the record and this brief, that names both. Exit 1 when the record is absent, carries no `ruling:`, or its ruling is by a bot or unmapped login, edited, or does not name the record and the brief; exit 2 is could-not-check, never a pass. Fail-first: at the authoring head the record does not exist and the row exits 1 (#6700) |

Pre-mortem (threat model):
- The plan is computed against the deliverable home, so the table, history or approval come from the wrong repository. Row 1 catches it (mutation).
- The container runs a plan bound to a different tracking commit or brief. Rows 2 and 3 catch it (tracking-field comparisons; `BriefSHA256` mutation).
- A dirty tracking checkout supplies an edited brief that the approval never saw. Row 3 catches it.
- A genuine approval recorded on a different repository with the same history and brief path (a fork or mirror) is replayed against this run. Row 3 catches it (`binding.Repo` comparison and its mutation).
- An approval is forged, written by a bot or unmapped login, posted on an issue without the transition marker, or names the deliverable repository. Row 3 catches it.
- An approval is edited between open and close. Row 3 catches it (close-step approval re-read and its mutation).
- `--in-container`, `--check` or `--fail-first` loosens while version 2 is admitted. Row 3 keeps today's message for the first two and pins the `--fail-first` refusal.
- Same-repository behaviour drifts, or a same-repository test is weakened to let it drift. Row 4 runs the 13 tests; row 5 proves the 7 files holding them are unchanged apart from the `version2` case removal.
- The design record is written or stamped by an agent instead of approved by the driver. Row 10 catches it (the ruling is dereferenced on the forge).
- The `Verification-Table` coordinate semantics the approved record states are wrong. No row; review-only (the driver's approval of the record and the security lane). Row 10 proves who approved, not that the content is right.

The implementation must create the exact named tests. An empty go-test match is not a PASS. If the new fixtures fail only under the admitted row environment's git pins (#2715), the PR says so with evidence.

## Evidence

<!-- Independent postmerge verifier records actual commands, source and outcomes here. -->

## Review

Gate: human. The implementation PR takes the security lane: `Security-Review: pass` is required before it is ready. Human acceptance is recorded here at done.
