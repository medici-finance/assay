---
brief: assay:assay:auto-triage:02
title: "Mechanical responder — file the bug and open a fixing draft PR, autonomously, for the mechanical class"
why: >-
  For the mechanical class of ambient red (a gofmt nit, a stray file, a stale string —
  #611, #1119), the fix is deterministic and today a human writes it by hand every time.
  This brief lets the automation file the bug and open a fixing DRAFT PR on its own, so the
  recurring mechanical toil is discharged without a person — while the human still merges
  every fix and no gate is ever weakened to make the red pass.
wave: 2
depends: ["auto-triage/01"]
unblocks: ["auto-triage/04"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  This brief WIDENS the automation's standing authority: after it, the system opens a draft
  PR and files an issue on a red gate WITHOUT a fresh human act. That is a design-approval
  decision, not a model's to sign off on its own reading of a mechanical diff — the human is
  confirming three things the review gate leans on: (1) the responder opens DRAFT PRs only
  and has no code path that merges, flips ready, or self-approves; (2) it acts only on the
  MECHANICAL class and refuses every judgement/opaque red (leak-sweep can never reach it);
  (3) its fix edits the culprit file:line and NEVER a gate/workflow config to make a red go
  green. The gate is `human` by authority-boundary, not by a `risk` boolean (cf.
  desk-tools/17): all four risk answers are honestly `no` because every action is reversible
  by a human at zero cost to `main`, but arming autonomous authoring still needs the record.
design: DR-auto-triage
issues: [611, 1119]
schema: brief-v2
authored: 2026-09-16 by the-desk auto-triage authoring session
sources:
  - "docs/streams/auto-triage/spec.md — §2 step 3 (mechanical → file + open fixing draft PR); §6 non-goals"
  - "docs/streams/decisions/DR-auto-triage.md — the design record this brief is gated on (PROPOSED)"
  - "medici-finance/assay#611 — gofmt nit hand-filed + hand-fixed (the exact mechanical toil this automates)"
  - "medici-finance/assay#1119 — gofmt drift hand-filed (same class)"
  - "freshness-checked 2026-09-16 @ e9fa19d3 (origin/main) — no tools/autotriage/ responder exists"
exec-tier: strong
exec-tier-why: >-
  (c) this is autonomous-action code where a subtle error acts publicly (opens a wrong PR,
  files a spurious issue) and (a) the draft-only / never-merge / never-weaken guarantees are
  design decisions the diff must embody, not just satisfy incidentally.
consumers:
  - "tools/autotriage/ (the Classify seam from brief 01): follow-up auto-triage/02 (this brief; consumes 01's output schema, flips to fixed-here when the responder implementation reads it)"
version: 1
id: 739df04b-fc17-473d-8697-32ecdc7a3c36
---

# Brief 02 — mechanical responder

## Context

single-point-of-failure: the classifier's `mechanical` decision (brief 01). If it
mis-classifies a judgement defect as mechanical, this responder would draft a wrong fix.
That single control is NOT left alone — three independent layers stand behind it, each
failing for a different reason in a different component:
- **Draft-only + human merge (forge).** Every fix is a DRAFT PR a human reviews and merges;
  holds even if the classifier is wholly wrong, because it trips on a different actor at a
  different time.
- **Mechanical-class-only guard (this responder).** It acts only when `class == mechanical`
  and refuses every other class; leak-sweep and judgement reds can never reach the auto-fix
  path.
- **Never-weaken-gate invariant (this responder).** The generated fix edits the culprit
  `file:line` only; a culprit whose path is a gate/workflow config is refused, not "fixed".

files:
- `tools/autotriage/` — the mechanical responder (a subcommand/entry over brief 01's
  `Classify`), plus tests and fixtures.
- `tools/autotriage/testdata/` — mechanical, judgement, and leak-sweep culprit fixtures.

facts:
- Input: a `Culprit` from brief 01 (`check`, `file`, `line`, `problem`, `class`).
- On `class == mechanical`: (a) compose a bug issue body naming the check + `file:line` +
  the deterministic remedy; (b) produce a fix on a fresh branch editing ONLY the culprit
  `file:line`; (c) open the PR as a **draft**. Filing + PR-open go through the sanctioned
  desk write path — never a hand-rolled token mint, never a merge or ready-flip.
- `--dry-run` prints the intended issue body and the fix/PR PLAN (branch name, the single
  edited path, `draft: true`) and writes nothing — the default mode for tests and the
  reviewable surface.
- Refusals (all → route to brief 03, never a silent no-op): `class != mechanical`;
  `check == leak-sweep`; a culprit `file` under `.github/workflows/` or any gate config.
- Idempotency: a culprit already carrying an open responder PR/issue does not get a second
  (dedupe on the `check`+`file:line` key), so a still-red gate across polls opens one PR,
  not many.
- Hard bound: NO code path in this responder merges, flips a PR ready, self-approves, or
  edits a gate to make a red go green. This is asserted by test, not left to review.

## Ground rules
- NEVER merge, ready-flip, or self-approve — draft PRs only; the human merges.
- NEVER narrow, silence, or edit a gate to make a red go green.
- File issues / open PRs ONLY through the sanctioned desk write path; never a hand-rolled
  mint. All live writes are gated behind `--dry-run`-off, which a human arms.
- Stop at `implemented` — you do not set verified/done. Feature branch + draft PR only.
- NEVER commit `STATUS.md` / `FINDINGS.md` on a branch.
- Fixtures carry no real secret/withheld content.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Build the mechanical responder over brief 01's `Classify`: on a `mechanical` culprit,
   compose the issue body, generate the single-file fix, and assemble a DRAFT-PR plan.
2. Implement `--dry-run` (print issue + PR plan, write nothing) as the default; live writes
   only when explicitly armed, and only through the sanctioned desk write path.
3. Implement the three refusal paths (non-mechanical class, leak-sweep, gate/workflow-config
   culprit) — each routes to brief 03, never a silent success.
4. Implement dedupe so a persistently-red gate opens exactly one PR/issue.
5. Assert in tests that no code path merges, flips ready, self-approves, or edits gate config.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/autotriage && go build ./... && go vet ./...` | exit 0 |
| 2 | `cd tools/autotriage && go test ./...` | exit 0; all green |
| 3 | `cd tools/autotriage && go test -run TestMechanicalDryRunPlansDraftPR -v` | exit 0; on a gofmt culprit the dry-run prints an issue body naming the `file:line` and a PR plan whose single edited path is the culprit file and whose `draft` flag is true — dereferences the responder's central behaviour |
| 4 | `cd tools/autotriage && go test -run TestNeverMergeNeverReadyNeverApprove -v` | exit 0; asserts there is NO code path that merges, flips ready, or self-approves (mutation/negative row: a responder that could reach a merge/ready call fails it) |
| 5 | `cd tools/autotriage && go test -run TestRefusesJudgementAndLeakSweep -v` | exit 0; a judgement-class culprit and a leak-sweep culprit each yield REFUSE→route-to-03 and open nothing (negative row: the auto-fix path is unreachable for opaque/judgement reds) |
| 6 | `cd tools/autotriage && go test -run TestNeverEditsGateConfig -v` | exit 0; a culprit whose `file` is under `.github/workflows/` is refused, and no generated fix ever edits gate/workflow config — the never-weaken-gate invariant, proven by breaking it (feeding a workflow-file culprit) and asserting refusal |
| 7 | `cd tools/autotriage && go test -run TestDedupeSingleOpenPR -v` | exit 0; the same culprit across two runs plans exactly one PR/issue |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item.
     This brief is gate: human — Evidence green rows are INPUT to the human review, not a
     substitute for it. Status stays `implemented` until a human confirms the three
     gate-why guarantees (draft-only/never-merge, mechanical-only, never-weaken-gate) and
     DR-auto-triage is approved. -->

## Review
Gate: human (autonomous outbound action; authority widening — see gate-why; design:
DR-auto-triage). Reviewer answers both core-control questions: (1) the single control
between a mis-classification and a bad outcome is the classifier, and the acceptable-making
layers are draft-only+human-merge, mechanical-only, and never-weaken-gate — confirm all
three are independent and present; (2) which Verify row proves a LOWER layer catches the
fault with the classifier bypassed — rows 4/5/6 break the upper assumption and prove the
responder still refuses. Verdict + date in the stream README table.
