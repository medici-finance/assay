---
brief: issue-loop/09
title: 'Wire intake triage into the-desk — triage at boot + on the alarm; no fourth window'
wave: 4
depends: ["issue-loop/07", "issue-loop/08"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by Fable desk session (I-intake-desk)
sources: ["I-intake-desk (the intake-desk design — this is its owner/wiring half: 'the coordinate loop (the-desk) at boot + on the alarm — triage is arbitration/scoping judgment, which is already that window''s remit')", "human:<name> 2026-07-12: 'we need to create an ''intake-desk'' similar to the ''issue-desk'' — as intake requests come in we need to work on them to get them into briefs/issues/flagged as decisions etc etc.'", "issue-loop/07 (the alarm this answers), issue-loop/08 (the verbs this speaks)", "issue-loop/04 (the sibling wire brief — scanner onto pr-review-desk's cadence; same no-fourth-window move)", "issue #221 (out-of-repo skill edit protocol)", "docs/streams/methodology/evidence/brief-20-reviewer-delta.md (the delta-mirror evidence-file pattern)", "I-loops-reference (this loop becomes the eighth documented loop: PURPOSE front-door drain · FEEDS-FROM intake register + alarm · FEEDS-INTO author-brief, issue-loop, decision queue, rejections)", "freshness-checked 2026-07-12 @ ab92e96e (~/.claude/skills/the-desk/SKILL.md Boot sequence has no intake-triage step)"]
why: >-
  A sensor nobody answers and verbs nobody speaks drain nothing — the loop is only closed when
  triage is OWNED. Per I-intake-desk's no-fourth-window principle (the same move issue-loop/04
  made for the scanner), the owner is the existing coordinator window: the-desk triages the
  front door at boot and whenever the intake-debt alarm fires, so untriaged age stays bounded
  by the desk's own cadence instead of by luck.
---

# Brief 09 — Wire intake triage into the-desk

> **F-41 re-target (2026-07-17).** This brief's original design — *"no fourth window; the-desk owns
> intake triage at boot + on the alarm"* — was **superseded by
> [F-41](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md)**: the inbound loop got its own
> standing window, and intake triage moved there. The triage step no longer lives in
> `~/.claude/skills/the-desk/SKILL.md` (that user-level skill was removed, stopgap `ebacc22`); it now
> lives in the **in-repo** issue-loop desk skill `../oit/.claude/skills/intake-desk/SKILL.md` (brief-11,
> merged PR #576) — the "intake lane" section with the four triage exits. That un-orphans the step the
> earlier VERIFY: FAIL flagged (Evidence row 2 below re-targeted to the in-repo skill). Mechanics
> unchanged — the four exits, the single decision queue, the queue-only authoring tier gate, the
> tombstone-never-delete rule all still hold; only the host window moved.

## Context
files: `docs/streams/issue-loop/README.md` (loop-owner conventions line);
`docs/streams/issue-loop/evidence/brief-09-the-desk-delta.md` (planned) — the in-repo delta
mirror of the skill edit
out-of-repo files: `~/.claude/skills/the-desk/SKILL.md` (triage step in "Boot sequence" +
respond-to-alarm rule in "Operating rules")
facts:
- **The wiring (I-intake-desk §3):** the-desk, at boot AND whenever the board/`--lint` carries
  the issue-loop/07 `intake debt:` NOTICE, works the untriaged set: for each `disposition: new`
  entry, apply exactly ONE of issue-loop/08's four exits. Triage is arbitration/scoping
  judgment — already the coordinator window's remit; **no fourth standing window** (the
  issue-loop stream's founding principle; issue-loop/04 is the sibling precedent).
- **Tier gate restated in the skill text:** triage decides the ROUTE only. `scoped → <stream>`
  QUEUES authoring (author-brief flow, strong tier); `scoped → issue #NN` files the issue;
  `decision-needed` files/links the needs-decision issue per issue-loop/06 and sets
  `decision-issue:`; `rejected — <why>`/`watching` record the reason. Triage never
  cheap-authors and never deletes entries (tombstone rules unchanged).
- **#221 protocol (the declaration above is the claim):** max ONE out-of-repo brief in flight
  across all streams — dispatcher checks for in-flight out-of-repo PRs before dispatching
  this one. The implementer stages the edit as a diff in the PR body, applies it to the live
  file only as the LAST step before `implemented`, and commits the applied edit in the
  `~/.claude` stopgap git repo.
- **Delta mirror (the established evidence pattern, brief-20-reviewer-delta.md):** the full
  added skill text also lands in-repo at
  `docs/streams/issue-loop/evidence/brief-09-the-desk-delta.md` (planned), so the reviewed
  artifact survives in this repo's history even though the live file is out-of-repo.
- README conventions gain the loop-owner line: intake-triage loop = sensor (07) + verbs (08) +
  the-desk cadence (09) — the front-door drain, eighth loop per I-loops-reference.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Out-of-repo skill edit per #221: declared above; apply the live edit LAST before
  `implemented`; before/after diff in the PR body; commit in the ~/.claude stopgap repo.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write the delta: a triage step in the-desk's "Boot sequence" (work the untriaged set per
   08's verbs) + an "Operating rules" line (the intake-debt NOTICE is a pull — answer it on
   the cadence you see it; queue authoring, never author inline at cheap tier; never dispose
   without a reason; never delete).
2. Land the delta mirror at
   `docs/streams/issue-loop/evidence/brief-09-the-desk-delta.md` (planned) —
   full added text + one-paragraph rationale, per the brief-20 pattern.
3. README (this stream): the loop-owner conventions line per facts.
4. LAST, per #221: apply the delta to `~/.claude/skills/the-desk/SKILL.md`, commit it in the
   stopgap repo, paste the before/after diff into the PR body.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -n "intake" docs/streams/issue-loop/evidence/brief-09-the-desk-delta.md` (planned) | exit 0; delta names the four exits and the alarm trigger |
| 2 | `grep -n "intake" .claude/skills/intake-desk/SKILL.md` | exit 0; the in-repo issue-loop desk skill carries the triage lane (re-targeted from the removed `~/.claude/skills/the-desk/SKILL.md` per F-41) |
| 3 | `grep -n "front-door" docs/streams/issue-loop/README.md` | exit 0 (loop-owner conventions line landed) |
| 4 | PR body carries the before/after diff of the skill edit (#221 protocol) | present |
| 5 | `statusgen --root . --lint` | exit 0 standalone |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

### Non-implementer verifier run — VERIFY: FAIL (glm-5.2-verifier, merged main `daa09b14`, 2026-07-16)

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `grep -n "intake" docs/streams/issue-loop/evidence/brief-09-the-desk-delta.md` | 0 | 7 hits; L23 names the `intake debt:` NOTICE trigger; L25-34 enumerate the four exits |
| 2 | `grep -n "intake" ~/.claude/skills/the-desk/SKILL.md` | **2** | **FAIL** — `No such file or directory`. The user-level the-desk skill was deleted by stopgap commit `ebacc22` (2026-07-16, "remove user-level the-desk duplicates"), ~1h40m after brief-09's own correct stopgap `d6e28a4` and before PR #573 merged |
| 3 | `grep -n "front-door" docs/streams/issue-loop/README.md` | 0 | L23 + L96 (the-desk owns the front door) |
| 4 | PR body carries before/after diff of skill edit (#221) | — | PASS — merged PR #573 body carries the full `diff --git a/skills/the-desk/SKILL.md` (Boot-sequence triage block + Operating-rules intake line) |
| 5 | `go run ./tools/statusgen --root . --lint` | 0 | exit 0 (advisory NOTICEs only) |

**VERIFY: FAIL — row 2 as written.** brief-09's OWN artifacts are all correct (delta mirror, README, PR #573 diff, stopgap `d6e28a4` was well-formed at implementation time), but the brief's substantive goal — intake triage **live in a skill the-desk actually loads** — is not met on current main: the live edit was removed by a later stopgap (`ebacc22`) whose cited relocation target (a dedicated issue-loop desk skill / [F-41](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md) / PR #576) is **unlanded and unregistered** ([F-41](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md) is nowhere in the repo; PR #576 is OPEN; the project-level `../oit/.claude/skills/the-desk/SKILL.md` carries 0 triage hits). The intake-triage step is currently **orphaned** — surviving only in the in-repo delta mirror + the PR #576 diff. Brief stays `implemented`; row 2 should be re-run once PR #576 (issue-loop desk skill) lands and re-homes the triage step. Regression tracked in bug (filed alongside).

### Re-verify run — VERIFY: FAIL (stale command; goal MET) — glm-5.2-verifier, merged main `bfba03ca`, 2026-07-17

The 2026-07-16 blocker has resolved: PR #576 (brief-11, issue-loop desk skill) **merged**, `../oit/.claude/skills/intake-desk/SKILL.md`
now exists and carries the intake lane (alarm → four exits → brainstorming→INTAKE), and [F-41](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md) is registered.
So brief-09's **substantive goal — intake triage live in a loaded skill — is now MET**, independently
confirmed by brief-11's VERIFY: PASS (rows 1-2 assert the skill exists with the triage).

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `grep -n "intake" docs/streams/issue-loop/evidence/brief-09-the-desk-delta.md` | 0 | 7 hits (delta mirror intact) |
| 2 | `grep -n "intake" ~/.claude/skills/the-desk/SKILL.md` | 1 | **FAIL (stale target)** — the-desk skill now exists (935 B) but carries 0 intake; triage correctly moved to `../oit/.claude/skills/intake-desk/SKILL.md` per [F-41](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md). Row 2 targets the OLD host and exits 1 forever as written. |
| 3 | `grep -n "front-door" docs/streams/issue-loop/README.md` | 0 | issue-loop desk owns the front door |
| 4 | PR #573 (brief-09) + #576 (brief-11) bodies carry skill diffs | present | both MERGED 2026-07-16 |
| 5 | `go run ./tools/statusgen --root . --lint` | 0 | exit 0 (advisory NOTICEs only) |

**VERIFY: FAIL — row 2 as written, but a stale-target fail, NOT a mechanics defect.** The goal is met via the
issue-loop skill; the remaining step is the **[F-41](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md) re-target** ([F-41](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md) "Fix direction" + README L25/L75): swap
row 2's grep target + the brief's host-window wording from `the-desk` → `../oit/.claude/skills/intake-desk/SKILL.md`
(small edit; keep the mechanics). Verifier recommends **re-target over supersede** — brief-09 owns the triage
mechanics/delta/PR #573, brief-11 owns the host window; complementary. Brief stays `implemented`; the re-target
amends the brief's Verify table (mid-flight-tweak → amend in-commit). Not self-passed: a fresh non-implementer
verifier should re-run the amended table once the re-target lands.

### Non-implementer verifier run — VERIFY: PASS — glm-5.2-verifier, merged main `700e1c9e`, 2026-07-20

The F-41 re-target has landed: the brief's current Verify table row 2 now points at the in-repo
`../oit/.claude/skills/intake-desk/SKILL.md` (not the removed `~/.claude/skills/the-desk/SKILL.md`). All 5 current
rows run clean; the two prior FAILs were the stale-target artifact, now fixed in the brief itself.

| # | Command (current table) | Exit | Key output |
|---|---------|------|------------|
| 1 | `grep -n "intake" docs/streams/issue-loop/evidence/brief-09-the-desk-delta.md` | 0 | 7+ hits; delta mirror intact (intake-debt NOTICE trigger + four exits) |
| 2 | `grep -n "intake" .claude/skills/intake-desk/SKILL.md` | 0 | loaded in-repo skill carries the triage lane (L190 "intake lane", L201 untriaged-age alarm, four exits, queue-only authoring gate, tombstone rules) |
| 3 | `grep -n "front-door" docs/streams/issue-loop/README.md` | 0 | L43 + L128 — intake-desk owns the front door |
| 4 | PR #573 body carries before/after skill diff (#221) | present | #573 MERGED 2026-07-16; body carries the full `diff --git a/skills/the-desk/SKILL.md` |
| 5 | `go run ./tools/statusgen --root . --lint` | 0 | exit 0 (advisory NOTICEs only) |

**VERIFY: PASS — substantive goal MET on current main.** Intake triage lives in a skill the pipeline loads
(`../oit/.claude/skills/intake-desk/SKILL.md`), routing through issue-loop/08's four exits with the queue-only
authoring gate + tombstone rules preserved; no fourth standing window (rides intake-desk's boot/alarm cadence).
No stale-command artifact remains.

## Review
Gate: model. Reviewer confirms (a) the skill delta routes only through 08's four exits and
carries the queue-only authoring tier gate, (b) the #221 sequence was followed (diff in PR
body, live edit applied last, stopgap-repo commit referenced), (c) the in-repo delta mirror
matches the applied edit, (d) no fourth standing window was created — the cadence rides
the-desk's existing boot/alarm loop.
