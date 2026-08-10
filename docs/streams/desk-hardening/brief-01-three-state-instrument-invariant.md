---
brief: desk-hardening/01
title: Three-state instrument invariant + fleet audit + mutation-test Verify rule
why: >
  The dominant defect class of the whole desk pipeline is not wrong answers — it is confident
  answers from instruments that never examined the thing they reported on. Eight instances in
  a single day (a deploy Job reporting a broken pipeline green, a CI rollup reading page 1 of N
  as "all passed", a board printing "worker must act" without checking a worker exists). A green
  board invites no second look, so this failure is strictly more dangerous than being wrong and
  it compounds. One invariant — no instrument reports a pass or a negative unless it can show it
  looked — is the antidote, and it must be enforced fleet-wide, not per incident.
wave: 0
depends: []
unblocks: ["desk-hardening/03", "desk-hardening/04", "desk-hardening/05"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [30, 33, 35, 36, 21]
schema: brief-v1
authored: 2026-07-24 by Opus session (desk-hardening authoring pass)
sources:
  - "freshness-checked 2026-07-24 @ 1768aee (net-new control — verified absent on origin/main)"
  - "assay-toolkit#30 (the dominant-defect-class finding)"
  - "assay-toolkit#33 (deskboard: checks-passed vs no-checks-ran)"
  - "assay-toolkit#35 (BLOCKED means orphaned — board never checks a worker exists)"
  - "assay-toolkit#36 (PR monitor phantom events from a partial snapshot)"
  - "assay-toolkit#21 (green Verify table proves the feature acts, not that it didn't break its neighbour)"
  - "in-flight: #25, #34 (three-state instances already dispatched — cover the REMAINING fleet, not these)"
exec-tier: strong
exec-tier-why: >
  cross-instrument reasoning (audit the whole fleet against one invariant) and a
  methodology change (brief-rules.md) whose error compounds through every downstream Verify table.
---

# Brief 01 — Three-state instrument invariant + fleet audit + mutation-test Verify rule

## Context
files:
- `[toolkit]` `docs/brief-rules.md` (add the mutation-test + neighbour Verify-row rules), `docs/brief-template.md` (reflect them)
- `[toolkit]` a new `docs/three-state-instrument-rule.md` (planned) (the canonical invariant + the fleet audit table)
- `[oit]` desk tooling: `deskboard.go` (ACTION computation — #33, #35), `deskpost` CI rollup (#33), the PR-monitor snapshot script (#36)
- `[oit]` `statusgen` intake untriaged-age alarm + `why`-field lint (member instances of the class), `medici-deploy` Job success condition
out-of-repo files: none
facts:
- the invariant (from #30): three states, never two — *checked-clean / checked-failed / could-not-check*; absence of evidence must never render as evidence of absence; silent truncation (page cap, node cap, `--limit`, sampling) is a lie; a compliance check whose cheapest satisfying input is content-free is not a check
- #33 requires a notion of *expected* checks per repo (workflow-derived or configured) so "no checks ran / never started" is UNKNOWN and blocks a flip exactly like red — "whatever ran" is the bug
- #35: `lastCommit < lastReview` ⇒ `ORPHANED` (reviewed, nobody home), distinct from `BLOCKED` (worker alive, owes a fix); both timestamps are already in `deskboard.go`
- #36: a per-repo `gh` failure must be `could-not-check`, must NOT overwrite the prior good snapshot, and must emit a `MONITOR-WARN` — the guard today only catches a *fully* empty snapshot, never a partial one
- #21: every well-scoped Verify table is blind to *collateral regression* in a sibling feature sharing a code path/flag/data source; the tighter the row, the blinder — this is structural, not a lapse
- the `verify-desk` skill already states the general case ("a row that cannot run is recorded as explicitly unrun, not silently skipped or assumed-pass") — this brief promotes it from the verify loop to every instrument

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Codify the invariant.** Write `docs/three-state-instrument-rule.md` (planned): the three states, the
   three sub-rules (absence≠absence-of, truncation-is-a-lie, content-free-input≠a-check), and
   the positive-control requirement (prove the instrument FAILS when the guarded thing is
   broken, before trusting a clean report).
2. **Harden the Verify-table methodology (`docs/brief-rules.md`).** Add two required rows for
   any brief that adds a *check* or touches *shared code*:
   (a) **mutation-test row** — revert the fix / break the guarded thing, confirm the check goes
   red (six of the eight #30 instances would have been caught in minutes by this);
   (b) **neighbour row** (#21) — for a change touching a shared lister/flag/query, one Verify
   row exercises the *adjacent consumer*, not the deliverable. Reflect both in `brief-template.md`.
3. **Fleet audit.** Produce an audit table (in the new doc) enumerating every desk instrument —
   `deskboard` ACTION, `deskpost` CI rollup, the PR monitor, the intake untriaged-age alarm,
   the `why`-field lint, `medici-deploy`'s success condition, the debug-pod script exit codes,
   `statusgen` alarms — and for each record: *what it prints when it cannot see*, current
   state-count (2 or 3), and disposition (`fixed-here` / `fixed-upstream` / `follow-up: #NN` / `follow-up: desk-hardening/NN` /
   `out-of-scope <why>`). Do NOT re-cover the instances already dispatched (#25, #34).
4. **Land the member fixes that are cheap and self-contained** (as part of this brief or as
   enumerated follow-ups, at the implementer's judgment given effort L):
   - #33: an *expected-checks* notion per repo; UNKNOWN (expected check absent/never-started)
     blocks the ready-flip exactly like red.
   - #35: `ORPHANED` vs `BLOCKED` in `deskboard.go` (+ an orphan-age column); route an orphan
     back to batch-fanout rather than letting it read as in-flight.
   - #36: per-repo exit-code capture; carry the prior snapshot forward on failure; `MONITOR-WARN`.
5. **Candidate approaches for the audit remediation (pick per instrument, record the choice):**
   (a) fix in place now; (b) split into a per-instrument follow-up brief in this stream; (c)
   declare out-of-scope with a reason. The audit table is the deliverable even where a fix is
   deferred — the class is closed by *coverage*, not by fixing all at once.

## Verify (executable — no prose-only DoD items)
Presence gates for the prose deliverables; the human/model review gate owns quality.
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/three-state-instrument-rule.md && grep -ci 'could-not-check' docs/three-state-instrument-rule.md` | exit 0; count ≥ 3 |
| 2 | `grep -ci 'mutation-test\|neighbour\|collateral' docs/brief-rules.md` | exit 0; count ≥ 2 |
| 3 | `grep -cE '\`fixed-(here|upstream)\`|follow-up[: ]|out-of-scope' docs/three-state-instrument-rule.md` | exit 0; count ≥ audited instrument count (14) |
| 4 | (if #35 landed here) revert the ORPHANED discriminator, run the deskboard test suite | the test that asserts `lastCommit < lastReview ⇒ ORPHANED` goes RED (mutation-test of this brief's own fix) |
| 5 | `cd status* && go run . --root .. --lint; echo $?` | exit 0 (stream lints clean) |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
The reviewer MUST apply this brief's own rule to it: does Verify row 4 actually go red when the
fix is reverted? A green table that was never mutation-tested is the very defect this brief closes.
