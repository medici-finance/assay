---
brief: issue-loop/11
title: Issue-loop desk skill + dedicated window — the inbound twin of pr-review-desk
why: >-
  The inbound loop had no operating manual — it was scattered briefs plus a bolt-on scanner step
  inside pr-review-desk, so nobody could "run the issue loop" the way they run the review loop. The
  front door (open issues + untriaged intake, 38 entries / 27 over 3 days at authoring) had no owner
  window. This gives it one: a standing desk, peer to pr-review-desk, that drops placeholders for
  inbound issues, triages intake into its four exits, files human-decision issues, and closes out
  resolved work — so inbound work stops rotting outside the model.
wave: 4
depends: ["issue-loop/02", "issue-loop/07"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by Opus desk session (human:<name> directive)
sources: ["human:<name> 2026-07-16: 'the issue-loop should have been setup to be similar to how the /pr-review-desk works. let's fix it to do that. it probably also needs to look at the intake queue as well.'", "human:<name> 2026-07-16 (follow-up): the issue/intake loop 'would need a smart model, and will be issuing PRs that other desks will review (the review-desk)' + 'the issue-loop also needs to deal with the intake queue' — hence the explicit smart-model requirement + PRs→pr-review-desk framing", "human:<name> 2026-07-16 (AskUserQuestion): 'Own desk + window' — revises the 2026-07-12 no-fourth-window principle", "[F-41](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md) — the design revision this implements", ".claude/skills/pr-review-desk/SKILL.md (the structural model)", "author-brief model-tier gate (the design-tier-work precedent the smart-model requirement mirrors)", "issue-loop/README.md §'One inbound loop' (the principle being revised)", "freshness-checked 2026-07-16 @ fcea7cdb (no .claude/skills/issue-loop existed; pr-review-desk step 4 still bolted the scanner on)"]
gate-why: not human-gated — the design decision (own desk + window) was human:<name>'s, confirmed 2026-07-16; this brief records and implements it. The artifact is a documentation/operating skill; every side-effectful action it describes (push, file issue, close, dispatch) remains governed by the existing per-action gates it cites.
---

# Brief 11 — Issue-loop desk skill + dedicated window

## Context
files: `../oit/.claude/skills/intake-desk/SKILL.md` (new), `../oit/.claude/skills/pr-review-desk/SKILL.md`
(step 4 re-pointed), `docs/streams/issue-loop/README.md` (revision note + brief row + waves),
`../oit/docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md` ([F-41](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md), new)
out-of-repo files: none — the skill is in-repo (`.claude/skills/`), invocable when the desk boots
into the repo checkout. A user-level `~/.claude/skills/issue-loop/` thin pointer is an OPTIONAL
follow-up (#221 protocol); not created here, so this brief stays fully in-repo and reviewable.
facts:
- Model: pr-review-desk skill — boot / board tool / per-item loop / escalation labels / push policy
  / persona Bob. This skill mirrors that shape for the inbound (issue + intake) side.
- Two lanes, one desk: GitHub issues (work-shaped) + intake register (idea-shaped). Routing test:
  hand to a worker as-is → issue; needs judgment → intake. Single decision queue = `needs-decision`
  issues (brief-06); the intake lane routes INTO it, never a second queue.
- Live machinery it drives: `--scan-issues` (brief-02, done), intake-debt alarm (brief-07). The rest
  of the stream (03–10) is this desk's own backlog.
- Reviewer-App has no `issues:write` (assay-toolkit#43) — this desk works issues as `the-org`.

## Ground rules
- NEVER git push / trigger workflows / mutating kubectl beyond the standing branch+draft-PR
  authorization. Leave commits per task instructions only. Deciding/closing a `needs-decision`
  issue is the human's.
- Stop at `implemented` — do not set verified/done.
- NEEDS_CONTEXT over guessing.

## Task
1. Author `../oit/.claude/skills/intake-desk/SKILL.md` on the pr-review-desk model: frontmatter with clear
   invocation triggers; role framing (inbound half, fourth window per [F-41](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md); **issues PRs that
   pr-review-desk reviews** — this desk never flips/merges its own); a **smart-model requirement**
   (the loop's core work — routing, triage classification, decision-issue authoring, scoping — is
   design-tier judgment, so it MUST run on a strong tier; a cheap tier does mechanical work only,
   mirroring the author-brief model-tier gate); dedicated-window + monitor-once rule; boot sequence
   (board = `--scan-issues --dry-run` + intake-debt `--lint` grep, arm monitor, announce); the
   two-lane loop (issue: create-placeholder / dispatch-is-not-ours / await-unblock / decision-issue /
   retire; **intake: alarm / four triage exits / routing test** — the second lane human:<name> stressed);
   shared escalation-labels + insight-routing; git push policy; "this desk owns finishing the stream".
2. Name the issue-loop desk as a peer in the pr-review-desk skill's pipeline framing, and flag the
   scanner bolt-on as moved. NOTE: the `--scan-issues` boot step lives only in the *user-level* copy
   of pr-review-desk (`~/.claude/skills/pr-review-desk/SKILL.md`), not the in-repo copy — so the
   in-repo edit is an additive cross-reference + a "your user-level copy is stale" note; physically
   removing the bolt-on from the user-level copy is an out-of-repo (#221) follow-up, folded into that
   skill's brief-22 thin-pointer conversion.
3. Record [F-41](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md) (the no-fourth-window revision) and update README §"One inbound loop" + brief table +
   waves so the principle and the skill agree.

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f .claude/skills/intake-desk/SKILL.md && head -1 .claude/skills/intake-desk/SKILL.md` | exit 0; prints `---` |
| 2 | `grep -c 'name: issue-loop' .claude/skills/intake-desk/SKILL.md` | ≥ 1 (frontmatter name present) |
| 3 | `grep -F 'issue-loop' .claude/skills/pr-review-desk/SKILL.md \| grep -i 'window\|desk'` | ≥1 line — step 4 now points at the issue-loop desk |
| 4 | `! statusgen --root . --lint 2>&1 \| grep -Eq -e '^ERROR' -e '^PROBLEM'` | exit 0 — no ERROR/PROBLEM lines |
| 5 | `test -f docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md && grep -c 'id: F-41' $_` | `1` |

## Evidence
<!-- filled by a non-implementer at verify time -->

Non-implementer verifier run (glm-5.2-verifier, merged main `bfba03ca`, 2026-07-17). **VERIFY:
PASS — all 5 rows.**

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `test -f .claude/skills/intake-desk/SKILL.md && head -1 …` | 0 | prints `---` (frontmatter present) |
| 2 | `grep -c 'name: issue-loop' .claude/skills/intake-desk/SKILL.md` | 0 | `1` (≥1 — name frontmatter present) |
| 3 | `grep -F 'issue-loop' .claude/skills/pr-review-desk/SKILL.md \| grep -i 'window\|desk'` | 0 | 3 lines — step 4 now points at the issue-loop desk ("inbound twin", "moved to the issue-loop desk") |
| 4 | `! go run ./tools/statusgen --root . --lint 2>&1 \| grep -Eq -e '^ERROR' -e '^PROBLEM'` | 0 | exit 0 = no ERROR/PROBLEM lines (226 advisory NOTICEs only; gate is ERROR/PROBLEM) |
| 5 | `test -f docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md && grep -c 'id: F-41' $_` | 0 | `1` ([F-41](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md) finding present) |

## Review
Gate: model. Reviewer records verdict + date in the README table.
