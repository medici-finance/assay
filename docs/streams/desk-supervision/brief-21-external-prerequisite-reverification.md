---
brief: "assay:assay:desk-supervision:21"
title: "Reverify changed external prerequisites without a synthetic push"
why: "An unchanged PR can become valid when its external prerequisite lands. A commit-only review gate forces unrelated pushes and repeated reviews; accept independently verified prerequisite changes without allowing unchanged code findings to be cleared."
wave: 1
depends: ["desk-supervision/19"]
unblocks: []
effort: "M"
gate: "human"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
gate-why: "Changes which review findings can hold a PR or how a standing rejection can clear; approve the precise rule before changing that control."
decision-trigger: "start"
decision-issue: 1403
ruling: "https://github.com/medici-finance/assay/issues/1403#issuecomment-5762224251"
design: "DR-external-prereq-21"
issues: [1387]
schema: "brief-v2"
authored: "2026-09-20 by recovery planning session"
sources: ["https://github.com/medici-finance/assay/issues/1387", "docs/streams/desk-supervision/recovery-increments.md", "source freshness 2026-09-20 @ 3db05fb44"]
consumers: ["tools/desk/internal/deskkit/checkonlycr.go: fixed-here", "tools/desk/cmd/deskboard/board.go: fixed-here", "tools/desk/cmd/deskpost/ready.go: fixed-here", "tools/desk/cmd/reviewloop/: fixed-here", "tools/desk/cmd/deskdispatch/references/review-prompt.md: fixed-here", "plugins/assay/skills/pr-review-desk/SKILL.md: fixed-here"]
exec-tier: "strong"
exec-tier-why: "Must distinguish genuine changed evidence from a bypass of a standing review rejection."
domain: "complicated"
version: 1
id: "b7bf8dad-179d-4cdc-93f7-cbb9b2f69bf8"
---

# Brief 21 — Reverify changed external prerequisites without a synthetic push

## Context

Home: medici-finance/assay. Planning only; issue 1387 explicitly records a proposed fix, not implementation authority. This brief contributes part of that issue; do not close the issue until both 20 and 21 and the class-counting behavior in 19 are accepted.

files:
- tools/desk/internal/deskkit/checkonlycr.go
- tools/desk/internal/deskkit/reviewfinding.go (planned in 19)
- tools/desk/internal/deskkit/externalprerequisite.go (planned)
- tools/desk/cmd/deskboard/board.go
- tools/desk/cmd/deskpost/ready.go
- tools/desk/cmd/reviewloop/
- tools/desk/cmd/deskdispatch/references/review-prompt.md
- plugins/assay/skills/pr-review-desk/SKILL.md
- tools/desk/cmd/deskpost/externalprerequisite_test.go (planned)

facts:
- Review-prompt clause 8 currently requires a whole-diff search on re-review and treats repository search as optional where cheap; it does not mandate a complete first-pass inventory.
- The canonical skill already caps full rounds at three per finding class. Preserve that threshold and human arbitration authority.
- Clause 11 forbids same-head clearance except a narrow required-check case. The code has an explicit check-only marker parser; this is an existing extension point, not license for prose-inferred exemptions.
- A prompt-only exception cannot repair a contradictory ready gate. Coordinate every producer and reader named in this brief.

## Read first

- [Recovery increments](recovery-increments.md).
- [Issue 1387](https://github.com/medici-finance/assay/issues/1387).
- The existing source paths above; new test and schema paths are explicitly planned.

## Human decision

Approve the precise review-control change described here before implementation begins.
A standing rejection based solely on explicitly recorded external prerequisites could clear without a source commit, only after independent, fresh verification of every prerequisite. Mixed code findings, missing evidence and unsatisfied security reviews would still block. Closure alone would never establish that a human decision was made.

Options:
1. **Approve this evidence rule** — implement the narrow exception consistently in the board, planner and ready gate with negative-path tests.
2. **Retain the commit-only rule** — keep the current refusal and existing human resolution path.

Default if no answer: none — blocks until answered.

**Ruling recorded (2026-09-21): APPROVED — option 1.** The driver ruled on the brief's
decision-gate issue in their own login, in the ruling comment on decision issue #1403
(the brief's `ruling:` key links it).
The ruling approves the narrow same-head external-prerequisite exception: it applies only to
an authenticated review explicitly stating that external prerequisites are the sole blockers;
every prerequisite is independently verified from fresh evidence and revalidated at ready
time; mixed code/content findings, missing evidence, later revocations and unsatisfied
security reviews still block; issue closure alone does not establish a human ruling; and the
board, planner and ready gate must agree — no empty or comment-only source commit is needed.
The design-decision record is `docs/streams/decisions/DR-external-prereq-21.md` (the brief's `design:` key).

## Interface contract

Extend the typed finding contract from brief 19 with an explicitly declared external-prerequisite-only rejection and references to every blocking condition. Legacy free text cannot qualify. The authenticated reviewer must state that there are no outstanding code/content findings; ambiguous, mixed or unreadable records fail closed. Preserve the existing check-only exemption and its tests.

A fresh independent reader must prove each named prerequisite changed after the rejection: for example an exact referenced PR merged at a cited commit, or an authorized decision event satisfied the recorded question. Issue closure alone is not proof of the required decision. Bind evidence to repository, object, condition, observation time and reviewer. No production query is introduced.

Permit a same-head re-review only when the source tree is unchanged, all the declared conditions are independently satisfied, and no later blocking finding or revocation exists. The board, review planner and ready gate must agree. Revalidate at the ready boundary to avoid a stale cached external pass. A security lane still supplies its own required current-head verdict. A worker marker or fabricated reference never grants approval. Do not ask for a comment-only or empty commit merely to move the SHA.

## Ground rules

- No runtime activation, production probes, automatic merge or reinterpretation of an existing human ruling.
- Implement in one isolated branch and draft PR after the decision; stop at implemented.
- Use synthetic fixtures. Do not copy private incident identifiers or review transcripts into this public repository.

## Task

1. Implement the exact approved contract in the named canonical producers and readers; do not change the numeric round threshold.
2. Keep the new review behavior consistent with worker instructions and the existing filing, verdict and ready boundaries.
3. Add synthetic regression cases for the named positive and negative paths. Record fail-first evidence for guard changes.
4. Document the contract and compatibility, add a changelog fragment and any required warmup marker. Release and pin through the existing procedure; source merge is not adoption.

## Verify

Named tests below are planned. The verifier must see each named PASS; exit zero with no matching test is not acceptance. Use offline injected forge state.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/deskpost/ -run ^TestExternalPrerequisiteSameHead$ -v -count=1` | named PASS for TestExternalPrerequisiteSameHead; a sole typed prerequisite becoming satisfied allows independent re-review at unchanged head and consistent board/ready behavior |
| 2 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/deskpost/ -run ^TestExternalPrerequisiteMixedAndForged$ -v -count=1` | named PASS for TestExternalPrerequisiteMixedAndForged; mixed content findings, worker-authored clearance, quoted markers, unrelated objects and unreadable source data cannot clear the rejection |
| 3 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/deskpost/ -run ^TestExternalPrerequisiteFreshnessAndLanes$ -v -count=1` | named PASS for TestExternalPrerequisiteFreshnessAndLanes; wrong revision, prerequisite predating the rejection, later revocation and a standing security failure continue to block |
| 4 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/deskpost/ -run ^TestExternalPrerequisiteRestartAndCheckOnly$ -v -count=1` | named PASS for TestExternalPrerequisiteRestartAndCheckOnly; restart preserves the exact condition and observation; existing check-only behavior and rejection of unchanged content findings remain intact |
| 5 | check +mutation | `cd tools/desk && GOWORK=off go run ./cmd/muhar -spec cmd/deskpost/externalprerequisite-mutations.json` | Harness healthy (baseline GREEN, positive control CAUGHT); `Totals: 6 caught, 0 NOT CAUGHT` — disarming any single fail-closed guard in the same-head decision (could-not-check, unrelated-object, revocation, wrong-revision, freshness, standing-security-failure) reddens its named negative-path test. The mixed-content guard is doubly layered and covered by TestExternalPrerequisiteMixedAndForged (see the spec's note) |

## Evidence

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/deskpost/ -run ^TestExternalPrerequisiteSameHead$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:0932fdefbc15 | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/deskpost/ -run ^TestExternalPrerequisiteMixedAndForged$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:2a51eb4de905 | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/deskpost/ -run ^TestExternalPrerequisiteFreshnessAndLanes$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:2a7fa7031951 | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/deskpost/ -run ^TestExternalPrerequisiteRestartAndCheckOnly$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:200ce99f0c58 | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && GOWORK=off go run ./cmd/muhar -spec cmd/deskpost/externalprerequisite-mutations.json` | pass exit=0 sandbox=container-netns | sha256:a62e01dd0ecd | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (on-behalf-of human:ian) (forge-identity) |

Verification pass 2026-10-20 by a dispatched verifier (strong tier, not the implementer) at source b1cf3d7907080ca6ff8dc3b023a0e7558af1123e. The witness table above is a mechanical re-run of all five Verify rows by statusgen verifyrun in a local docker container (image assay-verify:go1.27, built from golang:1.27.2-trixie, git 2.47) started with `--network none`, sandbox=container-netns, the engine's default seccomp profile, offline module cache, under the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). verifyrun exited 0 and `statusgen verifyrun --check` reported 5 pass, 0 fail, 0 could-not-run of 5 Verify rows. Because the witness records exit status and an output hash only, each row was also run by hand at the same source in a scratch clone on the host (darwin/arm64, go1.27.2) to read the named results the Expect cells require:

- Row 1: exit 0; `--- PASS: TestExternalPrerequisiteSameHead`, `ok` for the deskpost package.
- Row 2: exit 0; `--- PASS: TestExternalPrerequisiteMixedAndForged` with 8 subtests passing and none failing.
- Row 3: exit 0; `--- PASS: TestExternalPrerequisiteFreshnessAndLanes` with 5 subtests passing and none failing.
- Row 4: exit 0; `--- PASS: TestExternalPrerequisiteRestartAndCheckOnly` with 3 subtests passing and none failing.
- Row 5: exit 0, run alone in its own scratch clone, which was left clean afterwards; `Harness healthy: baseline GREEN, positive control CAUGHT.`, each of the six named mutations (could-not-check, unrelated-object, revocation, wrong-revision, freshness, standing-security-failure) CAUGHT by its named negative-path subtest, and `Totals: 6 caught, 0 NOT CAUGHT, 0 could-not-mutate.`

Prior Evidence: none. Before this pass the section held only the authoring placeholder, so this is the first acceptance pass; the deliverable landed in the implementation PR medici-finance/assay#1417, with later fixes to the same decision code in #1602, #2136 and #2311, all present at this source.

Human decision: the gate's ruling is the driver's own-login comment on decision issue medici-finance/assay#1403 (2026-09-21, option 1 approved), which matches this brief's `ruling:` key; that issue is closed and still carries its `needs-decision` label. A re-filed decision-gate issue for this item, medici-finance/assay#3711, is open and carries no reply yet.

Risk-bearing values (enumerated over the decision code in the deskkit and deskpost externalprerequisite sources). The step runs because the brief touches the deskkit package, which the repository classes as a security path; every risk answer is no and none is irreversible, so each value below can be undone by an edit and a redeploy:

- RISK-VALUE: DERIVED — externalPrereqOnly = regexp `(?i)^[ \t]*External-Prereq-Only:[ \t]*(\S.*?)[ \t\r]*$` @ tools/desk/internal/deskkit/externalprerequisite.go:58 — the explicit, typed declaration the ruling requires; it is read with fenced lines skipped and only as the sole marker value, so legacy free text cannot qualify; the row 2 subtest "quoted declaration is not a declaration" shows that a quoted marker does not qualify either.
- RISK-VALUE: DERIVED — clearedPrereq = regexp `(?i)^[ \t]*Cleared-Prereq:[ \t]*(\S.*?)[ \t\r]*$` @ tools/desk/internal/deskkit/externalprerequisite.go:69, with `len(fields) < 3` @ line 102 — each clearance must name condition, object and satisfying ref, which is the binding to object and condition that the interface contract asks for; a shorter value is kept as malformed and refused, not dropped.
- RISK-VALUE: DERIVED — authority binding `login == want` where want = the roster's reviewer App login @ tools/desk/cmd/deskpost/github.go:66 — only the authenticated reviewer identity can declare or clear, so a worker marker never grants approval (row 2 checks this).
- RISK-VALUE: DERIVED — `len(d.Conditions) == 0` refuses @ tools/desk/internal/deskkit/externalprerequisite.go:261 — a declaration that lists no blocking condition fails closed instead of clearing on an empty set.
- RISK-VALUE: DERIVED — freshness `!obs.ObservedAt.After(in.CRSubmittedAt)` @ tools/desk/internal/deskkit/externalprerequisite.go:327 — a strict later-than check with zero tolerance, which matches the contract's requirement that the prerequisite changed after the rejection; a zero rejection time also refuses (rows 3 and 5 check this).
- The three-round finding cap is untouched by this work: the delivering diff only cites it as unchanged, as the brief requires.

Verification-Attestation: medici-finance/assay#3712 run=88ad4b85e792134f91b3a68e0b670e3807c64fe4fc669da9ba59cf0063a59932 source=b1cf3d7907080ca6ff8dc3b023a0e7558af1123e brief=docs/streams/desk-supervision/brief-21-external-prerequisite-reverification.md model=claude-opus-5-5 tier=strong

**VERIFY: PASS** — all five Verify rows pass at b1cf3d790708 in the offline container witness, and the by-hand run shows each named test passing and the mutation totals; this is the first stamp, and the gate:human sign-off is still pending.

## Review

Gate: human. Verify the recorded decision and every bypass-negative before acceptance. This is a review-control change, not automatic approval of the incident PR.
