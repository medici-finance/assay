---
brief: issue-loop/04
title: 'Wire the scanner into pr-review-desk + close-out semantics (closed issue → resolved placeholder)'
wave: 2
depends: ["issue-loop/02", "issue-loop/03"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session ([I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md))
sources: ["INTAKE [I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md) (pr-review-desk owns the scanner; close-out open question)", "issue-loop/02 (the scanner), issue-loop/03 (the await state)", "~/.claude/skills/pr-review-desk/SKILL.md (the monitor cadence the scan rides)", "freshness-checked 2026-07-10 @ post-#288 main"]
why: >-
  The loop is only closed when creation AND retirement are automatic and OWNED. [I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md)'s "no
  fourth window" principle means the scan rides pr-review-desk's existing monitor cadence;
  and a placeholder for a now-closed issue must retire, or the board fills with ghosts.
---

# Brief 04 — Wire + close-out

> **F-41 re-target (2026-07-17).** The scanner's HOST WINDOW moved: it now rides the
> **issue-loop desk** window's monitor cadence (`../oit/.claude/skills/intake-desk/SKILL.md`, brief-11),
> **not** pr-review-desk's. The mechanics are unchanged — the same `--scan-issues` close-out code,
> the same branch→PR scan-and-commit act — so this brief's code Verify (rows 1/2/4) stays valid and
> the brief stays `verified`. The "pr-review-desk owns the scanner" wording below and Verify row 3's
> pr-review-desk diff are **historical provenance** of the original wiring, superseded by
> [F-41](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-16-issue-loop-gets-own-desk-window.md). The in-repo issue-loop skill
> carries the scanner step today (`grep -c 'scan-issues' .claude/skills/intake-desk/SKILL.md` > 0).

## Context
files: out-of-repo `~/.claude/skills/pr-review-desk/SKILL.md` (scan step in the boot/loop);
`../assay-toolkit/statusgen/` (close-out detection in `--scan-issues`)
out-of-repo files: `~/.claude/skills/pr-review-desk/SKILL.md`
facts:
- **Wire ([I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md): pr-review-desk owns the scanner):** the review window, on its monitor
  cadence, runs `--scan-issues --dry-run`, eyeballs the batch, then commits new placeholders
  (path-specific add, on a branch → PR like any doc change; the scan is cheap, the commit is
  the reviewed act). One scan step in the skill; no new window.
- **Close-out ([I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md) open question, decided):** `--scan-issues` also detects placeholders
  whose issue is now CLOSED and marks them `status: done` with a `resolved-by-issue-close`
  note (it does NOT delete — the placeholder is a work record; a closed issue = the work
  landed or was declined, both terminal). A closed issue that reopens re-activates on the
  next scan.
- Ordering: close-out runs in the same sweep as creation, so one `--scan-issues` both adds
  new placeholders and retires resolved ones — the board self-heals each cadence.

## Ground rules
- NEVER git push to main / trigger workflows / mutating kubectl. Leave commits per task only.
- Out-of-repo skill edit per #221 (declared; apply last; diff in PR body).
- Stop at `implemented`. NEEDS_CONTEXT over guessing.

## Task
1. Add close-out detection to `--scan-issues` (closed issue → placeholder `status: done` +
   note; reopened → re-activate).
2. pr-review-desk skill: the scan-and-commit step on the monitor cadence (per #221).
3. Tests: closed-issue placeholder → done; reopened → active; created + retired in one sweep.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes the Task-3 cases |
| 2 | `statusgen --root . --scan-issues --dry-run` on a fixture with a closed-issue placeholder | reports the retire action |
| 3 | PR body carries the out-of-repo pr-review-desk diff (#221) | present |
| 4 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- non-implementer rows. -->

Non-implementer verifier run (glm-5.2-verifier, merged main `4a3ed56a`, 2026-07-16). All 4 rows RUN, none UNRUN.

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `go test ./tools/statusgen/... -count=1` | 0 | `ok …/statusgen 2.249s`; `TestCloseOut*` 9/9 PASS (Retire, Reactivate, CreateAndRetireOneSweep, AlreadyDoneSkipsRetire, AlreadyTodoSkipsReactivate, DryRun, FileModificationRetire/Reactivate, BlockedPlaceholderRetired) |
| 2 | `go run ./tools/statusgen --root . --scan-issues --dry-run` | 0 | `would retire docs/streams/issue-loop/issue-300.md (…#300, issue closed)` |
| 3 | PR body carries the out-of-repo pr-review-desk diff (#221) | — | PASS — merged PR #568 body has the `### Out-of-repo edit (#221)` section with the full pr-review-desk SKILL.md patch (scanner step). In-repo `../oit/.claude/skills/pr-review-desk/SKILL.md` carries the scanner step (1 hit) |
| 4 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | advisory NOTICEs only ([F-31](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-ledger-hardening-18-deliverable-unmerged-on-agent-runtime-pr10.md)/[F-37](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-14-preprod-blocked-claim-partially-stale-pin.md)/[F-38](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-14-preprod-sync-probe-cap-quoted-as-sync-cost.md) unresolved; verification-debt) — exit 0 |

VERIFY: PASS. (Row 2 is environmentally coupled to `issue-300.md` + closed #300 — durable coverage is Row 1's `TestCloseOut*`; it would go red if #300 reopens or the placeholder retires.)

## Review
Gate: model. Reviewer confirms close-out retires-not-deletes (the placeholder is a record),
reopen re-activates, and the scan-and-commit is a reviewed branch→PR act, not a silent
main write.
