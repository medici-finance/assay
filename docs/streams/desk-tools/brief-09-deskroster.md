---
brief: desk-tools/09
title: 'deskroster — self-declared open-work → session roster (out-of-git), sessions register via their skills'
wave: 1
depends: ["desk-tools/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session (human:<name> direction)
sources: ["human:<name> 2026-07-10: a list of open work → session-name (bob/billy/jill/...); shouldn't sit in git", "the shared the-org identity problem (GitHub can't attribute a PR to a session — the same gap the desk-App question raises)", "issue #270 (claims dir — already carries {brief,session,ts}, folded in)", "INTAKE [I-28](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-loop-monitoring-dashboard-a-wip-website-over-the-standing.md) (the dashboard renders this roster as its who-owns-what panel)", "issue #221 (out-of-repo skill edit protocol — the self-registration wiring)", "freshness-checked 2026-07-10 @ post-#312 main"]
why: >-
  Every session commits as the-org, so GitHub cannot say which session owns which PR — human:<name>
  is canvassing sessions by hand ("I'm asking everyone who works on something"). A
  self-declared roster + a lister that joins it with live PR state answers "open work →
  session" without the manual round-trip. It is ephemeral machine-local state (like the STOP
  flags and the claims dir), so it lives out of git by construction.
---

# Brief 09 — deskroster

## Context
files: `../assay-toolkit/tools/desk/cmd/deskroster/` (NEW; uses `../assay-toolkit/tools/desk/internal/deskkit`, brief 01)
out-of-repo files: `~/.claude/skills/{batch-fanout,the-desk,pr-review-desk,verify-desk}/SKILL.md`
(self-registration wiring — #221 protocol)
facts:
- **Storage (out-of-git, machine-local):** `~/.claude/desk-tools/roster/<session>.json` — one
  beacon per session, `{session, role, updated, open_work:[{repo, pr, what}]}`. Same home and
  lifecycle as the claims dir and the DISABLED/STOP flags; the `~/.claude` stopgap repo already
  gitignores `desk-tools/**` (verified 2026-07-10) — nothing here is ever committed anywhere.
- **Session name resolution:** `$DESK_SESSION` env if set, else `$CLAUDE_SESSION_ID`, else a
  `--session <name>` flag; unresolvable → exit 6 (C-10 — never guess a session identity).
- Subcommands:
  - `deskroster set --repo R --pr N --what "..."` — upsert this session's entry for a PR
    (idempotent on (repo,pr); updates `what`/`updated`).
  - `deskroster drop --repo R --pr N` — remove an entry (on merge/close — pairs with the
    "merged PR = worker STOPS" rule).
  - `deskroster list` — join ALL beacons with live `gh pr view` state across the deskkit repo
    set; fold in the per-brief dispatch claims (`~/.claude/desk-tools/claims/*.claim`, which
    already carry `session`); print `session | pr | state | what`. A beacon entry whose PR is
    MERGED/CLOSED is auto-pruned on list (self-healing — stale entries never accumulate).
  - `deskroster mine` — just this session's entries (a session's self-check).
- **Self-declaration honesty ([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)):** the roster is what sessions SAY they own, not a proof —
  a session that doesn't register is invisible. `list` surfaces open PRs across the repos that
  NO beacon or claim covers under an `--- unclaimed (no session registered) ---` section, so
  the gap is visible, not silent (this is what catches jill / an agents-repo worker who hasn't
  wired registration yet). The real-attribution fix is the desk-App (a separate future intake);
  until then, self-declaration + unclaimed-surfacing is the honest best.
- deskkit: kill-switch/audit apply; these are LOCAL reads/writes (no outward GitHub write —
  `list` READS gh but writes nothing remote), so the outward-write rate limit does not apply.

## Ground rules
- NEVER git push / trigger workflows / mutating kubectl. Leave commits per task only.
- Out-of-repo skill edits per #221 (declared; apply last; diffs in PR body).
- Stop at `implemented`. NEEDS_CONTEXT over guessing.

## Task
1. Implement deskroster (set/drop/list/mine) per facts; `deskkit.Guard()` first; audit writes.
   `list` self-prunes merged/closed and surfaces the unclaimed section.
2. Skill self-registration (#221): a worker `deskroster set`s when it opens its draft PR and
   `deskroster drop`s when its PR merges/closes (batch-fanout worker template); each standing
   loop (the-desk/pr-review-desk/verify-desk) `deskroster set`s its role-level entry on boot.
3. Tests: set upserts idempotently; drop removes; list joins live state (fake gh) + folds a
   claims fixture + surfaces an unclaimed open PR; unresolvable session → exit 6; merged-PR
   entry auto-pruned on list.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/cmd/deskroster/... -count=1` | exit 0; includes the Task-3 cases |
| 2 | `DESK_SESSION=test go run ./tools/desk/cmd/deskroster set --repo oit --pr 1 --what x && go run ./tools/desk/cmd/deskroster list \| grep -c test` | ≥1 |
| 3 | `go run ./tools/desk/cmd/deskroster list` with no --session on a fixture claim present | shows the claim's session in the dispatch-claims section |
| 4 | `git -C ~/.claude check-ignore desk-tools/roster/test.json` | prints the path (confirms out-of-git) |
| 5 | PR body carries the out-of-repo skill diffs (#221) | present |
| 6 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended by a NON-implementer: one row per Verify item. -->

### Non-implementer verifier run — VERIFY: PASS — k3-verifier (verify-desk dispatch), merged main `26766ba6`, 2026-07-27

Run by the verify-desk as `assay-verifier-app[bot]` (commit `e771dd82`, 2026-07-27) on merged
main `26766ba6`. Impl PR **#692** (merged 2026-07-17, PRE-App-gate — null-login approval only;
cannot model-close to `done`, joins the #1444 retrospective-review list). All 6 rows PASS.

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/desk/cmd/deskroster/... -count=1` | 0 | suite green incl. Task-3 cases (idempotent upsert, drop, exit-6, auto-prune merged/closed, fold-claims, surface-unclaimed) | 2026-07-27 | k3-verifier |
| 2 | `DESK_SESSION=test go run ./tools/desk/cmd/deskroster set --repo oit --pr 1 --what x && go run ./tools/desk/cmd/deskroster list \| grep -c test` | 0 | ≥1 — set+list round-trip works (test beacon auto-pruned by the tool's self-healing; roster dir NO-DIFF before/after) | 2026-07-27 | k3-verifier |
| 3 | `go run ./tools/desk/cmd/deskroster list` (fixture claim present, no --session) | 0 | claim's session shown in the dispatch-claims section | 2026-07-27 | k3-verifier |
| 4 | `git -C ~/.claude check-ignore desk-tools/roster/test.json` | 0 | prints the path — roster is git-ignored (out-of-git per #221) | 2026-07-27 | k3-verifier |
| 5 | PR body carries the out-of-repo skill diffs (#221) | n/a | present — #692 body carries the skill-diff section; all four installed skills have the wiring live | 2026-07-27 | k3-verifier |
| 6 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | no NEW problems attributable to this brief | 2026-07-27 | k3-verifier |

Installed `/opt/desk-tools/bin/deskroster` byte-identical to in-repo `go run`. Read-only rows
re-confirmed stable at the fix branch (row 1 green again, `ok … 0.863s`).

**VERIFY: PASS.** `gate: model` + Evidence PASS → flipped `implemented → verified` by the
verify-desk (commit `e771dd82`); `done` awaits a retrospective App verdict or human close
(#1444 — pre-App-gate PR).

## Review
Gate: model. Reviewer confirms (a) the roster is out-of-git (Verify 4) and machine-local,
(b) list surfaces UNCLAIMED open PRs (the honest gap, not silent), (c) session resolution
fails closed on ambiguity (exit 6, never guesses), (d) all four skills self-register per #221.
