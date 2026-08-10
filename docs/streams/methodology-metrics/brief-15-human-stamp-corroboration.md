---
brief: methodology-metrics/15
title: 'Human-stamp corroboration — a PR adding human:<name> needs that account''s approval/comment on the PR (#237)'
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [237]
schema: brief-v1
authored: 2026-07-10 by Fable desk session (issue #237, human:<name>'s rule 2026-07-10)
sources: ["issue #237 (human:<name>'s rule: questioned human sign-offs resolve by platform action — approver mark or explicit approval comment from the human's own account)", "methodology/17 (tamper-evident reviewer gate — the identity principle being extended)", "methodology/16 (the honest limitation this closes: shared git author)", "verify-gate #171 ALLOWED_CLOSERS (prior art: platform proof for the done-gate)", "freshness-checked 2026-07-10 @ post-mm12 main"]
why: >-
  A human:<name> stamp in a Reviewed/Verified cell is agent-authored markdown — a worker
  can mint one without the human ever acting, and methodology/16 documents (honestly) that
  the shared git author makes this unverifiable from the repo alone. human:<name>'s rule makes the
  resolution platform-level: only a GitHub action from the named human's own account
  corroborates. This brief is the mechanical half; the desk protocol half is already live
  on the issue.
---

# Brief 15 — Human-stamp corroboration

## Context
files: `../assay-toolkit/statusgen/` (a `--corroborate` check) or `../assay-toolkit/tools/desk/cmd/deskboard/`
(coordinate with desk-tools/02 — implementer picks the home that ships sooner and records
why), + tests
facts:
- Check: for a given PR (or the current branch's diff vs main), find every ADDED
  `human:<name>` token in Verified/Reviewed cells and Evidence rows; map <name> to a
  GitHub login (a small committed map — `ian: human:<name>` etc.; unknown name → MISSING by
  definition); then query the PR's reviews + comments (`gh pr view --json reviews,comments`)
  for that login with either state APPROVED or a comment containing an explicit approval
  phrase ("approve"/"approved"/"sign off"). Output per stamp: CORROBORATED (with the
  review/comment URL) or MISSING-CORROBORATION.
- Consumers: pr-review-desk's flip precondition (a PR with MISSING-CORROBORATION is not
  flip-eligible — the skill edit is the protocol half, tracked on #237, NOT this brief);
  deskboard renders the state per PR when it lands; retro can sweep historically
  (`--corroborate` over merged PRs adding stamps).
- Honest scope ([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md) discipline): this verifies the named human ACTED on the PR — it
  cannot verify they actually executed a deferred live check; that remains what the
  sign-off MEANS. Say so in the check's output header.
- Network-dependent by nature: this is a desk/CI-adjacent tool, NEVER wired into the
  offline lint gate (same boundary as mm/15's degrade rule).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement the check per facts (home decision recorded in the PR); name→login map
   committed with a comment that adding a name is a reviewed change.
2. Tests against fixture PR JSON: stamp + APPROVED review → CORROBORATED; stamp + approval
   comment → CORROBORATED; stamp + comment by a DIFFERENT login → MISSING; stamp + agent
   comment quoting the human → MISSING (author login is what counts); no stamps → clean.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... ./tools/desk/... -count=1` | exit 0; includes every Task-2 case |
| 2 | run `--corroborate` against a real merged PR that added a `human:ian` stamp | correct verdict with URL, or MISSING honestly reported |
| 3 | `statusgen --root . --lint; echo $?` | 0 (lint gate untouched by the new check) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `7f524e40`):

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/... ./tools/desk/... -count=1` | 0 | `ok github.com/medici/statusgen` + 7 desk pkgs ok. All five Task-2 cases present and passing: `TestCheckStamps_ApprovedReview`, `_ApprovalComment`, `_DifferentLoginComment`, `_AgentCommentQuotingHuman`, `_NoStamps` (+ `_UnknownName`, `_MultipleStamps`, `_ApprovedReviewWinsOverComment`, `_CaseInsensitiveLogin`) | 2026-07-13 | opus-verifier |
| 2 | `go run ./tools/statusgen --corroborate --pr 405,376,357,356` (the real merged PRs that added `human:ian`) | 1 | honest-scope header, then 8 × `human:ian … MISSING-CORROBORATION — no APPROVED review or explicit approval comment from human:<name> on PR #<N>`. Verdict independently confirmed: `gh pr view 405 --json reviews,comments` → `reviews: []`, one comment by `the-org` (the agent account). No false negative | 2026-07-13 | opus-verifier |
| 3 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | NOTICEs only. Re-run with `gh` stripped from `PATH` → still exit 0, so the offline gate gained **no** network/`gh` dependency. `grep -rl corroborate .github/` → not referenced in any workflow | 2026-07-13 | opus-verifier |

Wording gap (cosmetic, not a defect): the brief's row 2 says "run `--corroborate` against a PR"; the shipped
flag form is `--corroborate --pr <N,...>` (bare `--corroborate <N>` exits 2 with usage).

### Falsification attempt (the guard tested against what it exists to catch)

Forged exactly the target attack: flipped this brief's README row `implemented → done`, minted a
`2026-07-13 human:ian` Reviewed cell, and wrote a self-authored Evidence table plus "Signed off by human:ian"
into the brief — i.e. a model minting its own human sign-off. Forgery reverted; tree confirmed clean.

1. **`--lint` does NOT catch it** — exit 0, zero notices. This is *by design* (the brief's own fact: "NEVER
   wired into the offline lint gate"), but it means the offline gate is not the guard and never will be. The
   catch depends on someone running `--corroborate` on the PR — the desk-protocol half, tracked on #237 and
   **out of this brief's scope**. It is in no CI workflow and no skill today.
2. **The corroborate check DOES catch it.** Both forged stamps were seen. The adversary's best moves — the agent
   account (`the-org`, which is what `gh auth` actually is here) posting an `APPROVED` review, an "Approved.
   Sign-off recorded." comment, a comment quoting human:<name> verbatim, and a bot "approved" comment — **all four →
   `MISSING-CORROBORATION`**. Author login drives the verdict, not body text. Control: a genuine `human:<name>`
   `APPROVED` review → `CORROBORATED` with the review URL.
3. **The un-forgeability is real, not nominal.** `human:<name>` is a distinct repo collaborator (write) and is *not*
   the account the agent tooling authenticates as (`gh api user` → `the-org`, admin). An agent cannot mint the
   corroborating signal with its own credentials — the same distinct-actor principle as the reviewer App
   (methodology/17).

**VERIFY: PASS** — all three rows executed and green on merged main; the guard demonstrably catches the
self-minted `human:<name>` forgery, and the offline lint gate gained no dependency. Caveat, not a failure: the
guard is **opt-in** — nothing invokes `--corroborate` yet (protocol half on #237), so today the forgery ships
unless the desk runs it.

### Substantive finding for the desk (not a defect in this brief)

Run against real history, the check retro-flags **every `human:ian` stamp in the repo** as unbacked: PRs
**#405, #376, #357, #356** (reconciler-spinout/10, agentic-first/03, methodology/36, agentic-first/01) all carry
agent-authored `human:ian` sign-offs with **zero platform action from `human:<name>`** — no review, no comment.
`human:<name>` has never reviewed or commented on this repo; all 90 issue comments are `the-org`/dependabot, and PRs
are merged by `the-org`. This is the guard working as intended on real data, and it independently reproduces
the standing rule from PR #236 (a human gate needs an authorization artifact, not an agent-written cell). Those
four human-gate sign-offs are **unbacked by platform proof** and are candidates for demotion or a retroactive
on-PR sign-off from human:<name>'s own account.

## Review
Gate: model. Reviewer confirms author-login (not body text) drives corroboration, the
honest-scope header is present, and the offline lint gate gained no dependency.
