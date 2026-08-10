---
brief: issue-loop/03
title: 'Await/unblock state — a worker question parks the placeholder on the issue, a comment resumes it'
wave: 1
depends: ["issue-loop/01"]
unblocks: ["issue-loop/04"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session ([I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md))
sources: ["INTAKE [I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md) (the conversation-channel design + blocked:awaiting-issue-response state)", "issue-loop/01 (placeholder schema)", "methodology-metrics/08 (Next-up eligibility this gates on)", "freshness-checked 2026-07-10 @ post-#288 main"]
why: >-
  Issues differ from briefs in one way that matters: the spec is often incomplete and the
  worker needs to ASK. Without a parked state, an under-specified issue either gets guessed
  (wrong) or re-dispatched in a loop. The await/unblock state makes the GitHub issue the
  conversation channel and keeps the desk from re-dispatching a question already asked.
---

# Brief 03 — Await/unblock state

## Context
files: `../assay-toolkit/statusgen/` (Next-up eligibility), placeholder frontmatter (a `blocked:`
field), the worker-dispatch protocol (out-of-repo: batch-fanout's worker prompt, #221)
out-of-repo files: `~/.claude/skills/batch-fanout/SKILL.md` (worker-question protocol line)
facts:
- Mechanism ([I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md) open question, decided): the placeholder gains
  `blocked: awaiting-issue-response` (frontmatter field, NOT a GitHub label mirror — keeps
  it in-repo and offline-checkable). A worker with a blocking question POSTS it on the
  GitHub issue, sets the field, commits the placeholder change, and STOPS.
- statusgen excludes `blocked: awaiting-issue-response` placeholders from Next-up (same
  eligibility layer as claim-awareness) — the desk won't re-dispatch.
- Un-block: a NEW non-bot comment on the issue after the block timestamp clears it. The
  scanner (brief 02, wired in 04) detects "newest non-bot comment is newer than the block"
  and clears the field on its next sweep — a human answering IN the issue resumes the work,
  no board edit needed.
- Bot/desk comments do NOT un-block (author-login check, the #237 identity principle) —
  only a human (or a non-bot actor) answering the question counts.

## Ground rules
- NEVER git push to main / trigger workflows / mutating kubectl. Leave commits per task only.
- Out-of-repo skill edit per #221 (declared; apply last; diff in PR body).
- Stop at `implemented`. NEEDS_CONTEXT over guessing.

## Task
1. Add the `blocked: awaiting-issue-response` field + Next-up exclusion; the scanner's
   un-block detection (newest non-bot comment newer than block ts → clear).
2. batch-fanout worker prompt: a worker on an `issue-<NN>` placeholder with a blocking
   question posts on the issue, sets the field, stops (per #221 protocol).
3. Tests: blocked placeholder excluded from Next-up; non-bot comment after block → cleared;
   bot comment after block → still blocked (author-login gate).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes the Task-3 cases |
| 2 | scratch Next-up with a `blocked: awaiting-issue-response` placeholder | the row is absent |
| 3 | PR body carries the out-of-repo batch-fanout diff (#221) | present |
| 4 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- non-implementer rows. -->

Non-implementer verifier run (glm-5.2-verifier, merged main `3d3708ad`, 2026-07-16). All 4 rows RUN.

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `go test ./tools/statusgen/... -count=1` | 0 | `ok …/statusgen 2.042s`; Task-3 cases (`TestBlockedPlaceholder*`) all green |
| 2 | scratch Next-up with a `blocked: awaiting-issue-response` placeholder | 0 | blocked row absent from "Next up"; surfaced in "Incomplete briefs" with the marker |
| 3 | PR body carries the out-of-repo batch-fanout diff (#221) | — | PRESENT on merged PR #366 |
| 4 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0, 0 PROBLEM lines |

VERIFY: PASS. (Implementation tightened the bot-detection gate with a `<!-- desk-automation -->` HTML marker beyond the brief's author-login check — assay-toolkit#26; covered by `TestBlockedPlaceholderMarkerCommentDoesNotUnblock`.)

## Review
Gate: model. Reviewer confirms un-block is author-login-gated (a desk/bot comment can't
resume the work — only a human answer), and the blocked state is in-repo frontmatter, not
a fragile label mirror.
