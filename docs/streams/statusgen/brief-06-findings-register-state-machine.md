---
brief: assay:assay:statusgen:06
title: Findings register becomes a corroborated state machine — bounded shelving (parked) + transition guard on resolved/affects/parked
wave: 1
depends: []
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-08-20 (authored clean for the statusgen board)
exec-tier: strong
exec-tier-why: >-
  Cross-artifact safety-plumbing where a subtle error survives the brief's own tests and silently
  re-opens a falsification hole: the schema, alarms.go, corroborate.go, the CI wiring and any
  migration must agree on ONE state model, and design decisions the facts do not fully pre-specify
  (the exact field schema, the re-annunciation semantics on expiry, the authorized-park corroboration
  path) have to be made coherently.
sources:
  - "The register-guard gap: it catches DELETION of a finding but not in-place gutting (mutation of resolved/affects)"
  - "The toothless-park gap: a >7-day standing alarm fires identically whether a finding is neglected or consciously parked, so 'park' as a disposition does nothing"
  - "A long-standing standing-alarm finding — an alarm set that never goes quiet (ISA-18.2 rule-1 violation); resolves as an instance of shelving once it exists"
  - "The precedent that anti-falsification / integrity-check changes are a human-gate decision, not a correctness rejection"
  - "The ISA-18.2 / EEMUA-191 alarm-rationalization lineage the design cites (bounded shelving = a snooze, not a mute)"
  - "The invariant that orient integrity is paramount"
  - "The non-self-writable lifecycle + the tombstone/deletion guard this corroboration machinery extends"
gate-why: >-
  This brief modifies statusgen's anti-falsification / integrity-check logic — the corroboration
  guard (corroborate.go) and the standing-alarm suppression (alarms.go). Standing policy is that
  integrity-check changes are a HUMAN gate: a wrong guard fails in two catastrophic directions — too
  loose lets falsification through (a self-park to 2099 = a perfect silent kill), too tight bricks
  legitimate register edits and forces reviewers to instruct un-lintable cleanups. irreversible: yes
  because a landed guard that bricks edits (or a landed hole) governs every future PR touching the
  register until reverted. The human confirms: the bounded-shelving field schema, the corroboration
  rule for parked/resolved transitions, the fail-closed fallback, and — the load-bearing dependency —
  that shelving and the guard SHIP TOGETHER (shelving without the guard makes the gutting hole
  strictly worse).
why: >-
  The FINDINGS register is the orient-integrity layer the whole methodology rests on — yet it is
  untrustworthy in two directions at once. An agent can silently GUT a finding that demoted its own
  brief (flip resolved: yes or empty affects:, uncorroborated, CI green), and a
  deliberately-accepted-deferred finding CANNOT be shelved: the >7-day standing alarm fires
  identically whether a finding is neglected or consciously parked, so "park" as a disposition does
  nothing. A register that cannot distinguish a silenced finding from a resolved one, nor a parked
  finding from a neglected one, is not a register you can orient on.
version: 1
id: 9976ad4e-63f3-4359-907b-f07b6f5fdf3d
---

# Brief 06 — Findings register becomes a corroborated state machine

statusgen's source and the findings register both live in this repo, so the schema, `alarms.go`,
`corroborate.go` and CI wiring changes and the register migration land together here (as two PRs, not
two repos): the code half is one PR against `statusgen/`, the register migration another.

## Context

files:
- `statusgen/model.go` (Finding struct — add Parked state fields)
- `statusgen/parse.go` / the findings loader (parse the new frontmatter fields)
- `statusgen/alarms.go` (`standingAlarmNotices` / `computeAlarms` — shelving suppression)
- `statusgen/corroborate.go` (extend the guard to `resolved`/`affects`/`parked-*` transitions)
- `statusgen/*_test.go` (TDD coverage for every new branch)
- the board CI wiring that invokes the guard on register-touching PRs
- `docs/streams/statusgen/README.md` (status-table row + waves for statusgen/06)
- the findings register (migrate any free-text `parked:` entries to the bounded form; none exist on
  a freshly-bootstrapped board, so this half is conditional — it applies as soon as a park is filed)

facts:
- register source of truth: per-entry `docs/streams/findings/<date>-<slug>.md` files; the
  `FINDINGS.md` register is a generated, main-CI-only view.
- Finding struct today: `ID, Date, Title, Affects []string, Ack, Resolved bool, FileRel`. No
  park/quiescence state.
- standing alarm today (`alarms.go` `standingAlarmNotices`): keys ONLY on age > a 7-day threshold
  for un-`Resolved` findings. The NOTICE text literally says "resolve or park it" — but there is no
  park: a `parked:` marker is ignored, so an accepted-deferred finding alarms identically to a
  neglected one (ISA-18.2 rule-1: a console that is never quiet).
- guard today (`corroborate.go`): a standalone `--corroborate <pr>` subcommand scans a PR diff for
  `human:<name>` stamps and corroborates each against the PR's reviews/comments (an APPROVED review
  by the mapped login, or an approval-phrase comment). It does NOT look at `resolved`/`affects`/
  `parked` changes at all. The tombstone/deletion guard catches DELETION of an entry but not in-place
  field mutation.
- CI: downloads/builds statusgen, runs `--lint`; there is currently NO CI invocation of
  `--corroborate` on register-touching PRs.
- a login map (`corroborate.go`) maps a human name to a GitHub login; adding a name to this map is
  itself a reviewed change (the map IS the name=login claim).

## Ground rules
- NEVER git push / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done. This is `gate: human`: a human signs off
  before `verified` (integrity-check human-gate; and irreversible → human-before-verified).
- NEVER commit `STATUS.md` on this branch (generated, single-writer = main CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## The problem (three folded items, one underlying gap)

statusgen models a finding as a `resolved` boolean plus a deletion guard, when the register actually
needs a small **state machine** with each transition corroborated:

```
open ──park(until, by, reason)──▶ parked ──expiry──▶ open (re-annunciate, louder)
  │                                  │
  └──────────resolve(by, evidence)───┴──▶ resolved
```

1. **Gutting:** in-place mutation of a finding's `resolved`/`affects` is unguarded. An agent whose
   brief a finding demoted flips `resolved: yes` (or empties `affects:`) in the same PR,
   uncorroborated, and `--lint` passes. Because `affects:` is what blocks the brief, gutting it
   silently unblocks the brief — the exact self-serving move the register exists to prevent.
2. **Toothless park:** `alarms.go` suppresses the >7-day standing NOTICE ONLY on `resolved: yes`. A
   `parked:` marker is ignored — a consciously accepted-deferred finding alarms exactly like a
   neglected one, so "park" does nothing.
3. **Never-quiet alarm set:** an alarm set that can never go quiet, with no shelving/quiescence, is
   ISA-18.2-broken. Same alarm-management theme; a persistent standing alarm resolves as an
   **instance** of the shelving mechanism once it exists.

## The design to spec (present; the human approves before implementation)

### A. Bounded shelving (ISA-18.2 / EEMUA-191)
A park is a **snooze, not a mute**, and must be bounded:
- `parked-until: <YYYY-MM-DD>` — REQUIRED. No open-ended parks (an unbounded park is a disguised
  resolve). Missing/empty on a parked finding → `--lint` PROBLEM.
- `parked-by: <authority>` — REQUIRED. The authorizing party (`human:<name>` form, same vocabulary
  as lifecycle stamps).
- `parked-reason: <prose>` — REQUIRED. Why it is accepted-deferred.
- `alarms.go` suppresses the standing NOTICE **only while `now < parked-until` AND the park is
  authorized** (see B).
- **On expiry (`now >= parked-until`) it RE-ANNUNCIATES, louder** than a plain standing alarm: a
  distinct "park expired — re-decide (extend, resolve, or act)" NOTICE that the desk/retro cannot
  mistake for a fresh standing alarm. A park buys a bounded window, then forces a fresh decision — it
  never silently becomes permanent.
- Flood accounting (`computeAlarms`): decide and STATE whether a validly-parked finding counts
  toward the flood threshold (recommended: parked findings are excluded from the active-flood count
  while their park is live, since they are consciously shelved — but an EXPIRED park counts again).

### B. The guard half — MANDATORY, ships in the SAME change
The `parked-*` and `resolved` transitions get the **same corroboration** `corroborate.go` already
applies to `human:<name>` lifecycle stamps:
- A PR that flips `resolved: no → yes`, empties/narrows `affects:`, or ADDS/EXTENDS a `parked-until`
  on a finding **present at the PR merge-base** must be **independently attributed** — an authorized
  `parked-by`/resolver corroborated against the PR's reviews/comments (the existing APPROVED-review /
  approval-phrase path), OR a `Verified-by`-style trailer. An agent **cannot self-park or
  self-resolve**.
- In-place mutation of these fields is guarded exactly like deletion is (the tombstone guard's
  sibling). The guard diffs the finding's `resolved`/`affects`/`parked-*` against the merge-base
  version and **hard-fails an unattributed change**.
- **Fail-closed:** if corroboration cannot be evaluated (no PR context, gh unavailable), the guard
  fails closed on a register-field change — never green-by-default.

> **Load-bearing dependency (state it in the PR, do not split):** shipping shelving WITHOUT the guard
> makes the gutting hole strictly WORSE — it hands the attacker a perfect silent kill
> (`parked-until: 2099`, uncorroborated, alarm muted for 73 years). The two halves are ONE change. A
> reviewer who sees only the shelving half must bounce it.

### C. A never-quiet alarm set resolves as an instance
Once bounded shelving exists, a set of permanently-standing alarms that cannot clear (because their
underlying work is still open) is **parked** — bounded, attributed, reasoned — instead of falsely
resolved or left screaming. Such a finding is not fixed BY this brief's code; it becomes **resolvable
via** this mechanism (a separate follow-up parks it). Note this in that finding's disposition when the
mechanism lands; do not resolve it in this brief.

## Task

**In-repo deliverables (the register migration is a separate PR from the code):**

1. **Migrate any free-text-parked findings** to the bounded form. Replace each free-text `parked:
   "<date> — …"` with `parked-until:`, `parked-by:`, `parked-reason:`. Where the authorizing human is
   not already recorded, FLAG in the migration that it needs a human's attribution at sign-off (do
   not fabricate a `human:` stamp). Pick a concrete `parked-until` per finding — a DECISION-NEEDED
   item the human confirms. (On a freshly-bootstrapped board with no parks, this step is a no-op until
   the first park is filed.)
2. **Update `docs/streams/statusgen/README.md`**: add the statusgen/06 status-table row and place it
   in the waves block.

**The statusgen code PR (spec — TDD, stdlib + yaml.v3):**

3. `model.go`: add park state to `Finding` (`ParkedUntil string`, `ParkedBy string`,
   `ParkedReason string`; keep `Resolved bool`).
4. findings parser: parse the three `parked-*` fields; treat all-absent as `open`.
5. `alarms.go`: shelving suppression + louder re-annunciation on expiry + flood accounting
   (design §A). Add `--lint` PROBLEM for a park missing any required field.
6. `corroborate.go`: extend the guard to `resolved`/`affects`/`parked-*` transitions against the
   merge-base, fail-closed (design §B).
7. CI: invoke the extended guard on register-touching PRs (there is no such CI step today — add one;
   mirror the pinned/built-binary discipline).
8. Tests for every new branch: authorized park suppresses; unauthorized park PROBLEMs; expired park
   re-annunciates; self-resolve fails; self-gut of `affects` fails; corroborated resolve passes.

## Verify (executable — no prose-only DoD items)

| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -rl '^parked-until:' docs/streams/findings/ 2>/dev/null; echo done` | lists every bounded-parked file (none on a fresh board) then `done` — parks use the bounded `parked-until` form, not the free-text `parked:` marker |
| 2 | `grep -rn -e '^parked:' docs/streams/findings/ 2>/dev/null; echo rc=$?` | no free-text `parked:` key survives the migration |
| 3 | `statusgen --root . --lint` | exit 0; a park missing `parked-until` PROBLEMs; an expired park emits the louder re-annunciation NOTICE |
| 4 | `git diff --name-only $(git merge-base HEAD origin/main) HEAD -- STATUS.md` | empty output — STATUS.md NOT modified on the branch |
| 5 | `cd statusgen && GOWORK=off go test .` | exit 0 with the new park tests present: authorized park suppresses the standing NOTICE, a park missing a required field PROBLEMs, an expired park re-annunciates, self-park / self-resolve / self-gut FAIL, corroborated transitions PASS |
| 6 | inject `resolved: no→yes` on a merge-base finding with no corroboration, run the guard | exit 1 (hard-fail, fail-closed) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. This is
     gate: human + irreversible, so a human signs off before any row is marked verified. -->
### Non-implementer verifier run — VERIFY: FAIL — rows 1-6 pass, Deliverable 7 absent (#1855) — 2026-09-30 claude-opus-5-5-verifier

Runner is not the implementer. Isolated worktree at merged main `b89b3957225e227e69d5b5ec7949344f580d9966` (HEAD == the forge's `commits/main`). Implemented by PR #84 (head 9c0a7d5, merged 2026-08-23). `gate: human`, `irreversible: yes` — **Evidence only; nothing flips.** Expectations derived from the brief text before reading the diff. Fixture rows ran in a throwaway clone with `origin/main` pinned to a fixture base, the register's one finding backdated to raise a standing alarm, and an empty or scratch roster; binary built from `statusgen/` at the target sha; tree clean afterwards.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `grep -rl '^parked-until:' docs/streams/findings/ 2>/dev/null; echo done` | no free-text parks | PASS — exit 0; only `done` (1 register entry, no park) | 2026-09-30 | claude-opus-5-5-verifier |
| 2 | `grep -rn -e '^parked:' docs/streams/findings/ 2>/dev/null; echo rc=$?` | rc=1 | PASS — `rc=1` | 2026-09-30 | claude-opus-5-5-verifier |
| 3a | `statusgen --root . --lint` (main tree) | LINT: PASS | PASS — exit 0, `LINT: PASS` | 2026-09-30 | claude-opus-5-5-verifier |
| 3b | fixture: park without parked-until; `statusgen --root . --lint` | PROBLEM, alarm still fires | PASS — exit 1; `malformed park — missing/invalid: parked-until (a bounded YYYY-MM-DD expiry — no open-ended parks)`; standing alarm still printed | 2026-09-30 | claude-opus-5-5-verifier |
| 3c | fixture: park expired 2026-09-15; lint | louder re-annunciation | PASS — exit 0; `NOTICE: park EXPIRED — re-decide: … parked until 2026-09-15`; plain alarm replaced, not duplicated | 2026-09-30 | claude-opus-5-5-verifier |
| 3d | fixture: live park to 2026-12-31; lint | alarm suppressed | PASS — exit 0; baseline `standing alarm: … open 29 days` gone; `LINT: PASS` | 2026-09-30 | claude-opus-5-5-verifier |
| 3e | fixture: parked-until = today (now >= parked-until); lint | EXPIRED | PASS — exit 0; `NOTICE: park EXPIRED — re-decide: … parked until 2026-09-30` | 2026-09-30 | claude-opus-5-5-verifier |
| 4 | `git diff --name-only $(git merge-base HEAD origin/main) HEAD -- STATUS.md`, plus `git diff --name-only e15addf 9c0a7d5 -- STATUS.md` | empty | PASS — both empty (the first is vacuous after merge; PR #84's own range touched 7 files, none STATUS.md) | 2026-09-30 | claude-opus-5-5-verifier |
| 5 | `cd statusgen && GOWORK=off go test .` | ok | PASS — exit 0; `ok github.com/medici-finance/assay/statusgen 41.633s`; 14 park and gutting tests pass in a verbose run | 2026-09-30 | claude-opus-5-5-verifier |
| 6a | fixture: resolved no→yes in the working tree, no authorization; lint | PROBLEM | PASS — exit 1; `register field-gutting (unauthorized): … resolved flipped no->yes vs the version landed at the merge-base with origin/main` | 2026-09-30 | claude-opus-5-5-verifier |
| 6b | same edit, committed; lint | PROBLEM | PASS — exit 1; same line | 2026-09-30 | claude-opus-5-5-verifier |
| 6c | fixture: `affects: []`; lint | PROBLEM | PASS — exit 1; `affects dropped [windows-port/02]` | 2026-09-30 | claude-opus-5-5-verifier |
| 6d | fixture: self-park to 2099-01-01, unmapped parked-by; lint | PROBLEM | PASS — exit 1; `parked-until added [2099-01-01] (mutes the standing alarm until then)` | 2026-09-30 | claude-opus-5-5-verifier |
| 6e | fixture: resolve + authorized-by naming a mapped fixture human (scratch 0600 roster); lint | pass | PASS — exit 0, `LINT: PASS` | 2026-09-30 | claude-opus-5-5-verifier |
| 6f | fixture: live park added, `GITHUB_ACTIONS=true`, `ASSAY_HUMAN_LOGIN_MAP` unset (as this repo's lint job runs); lint | fail closed | PASS — exit 1; `register field-gutting (unauthorized): … parked-until added [2026-12-31]`; with the map set, exit 0 | 2026-09-30 | claude-opus-5-5-verifier |
| 6g | fixture: `refs/remotes/origin/main` deleted, resolve flip committed; lint | fail closed (§B) | **FINDING F3** — exit 0; `NOTICE: register field-gutting guard is running degraded: origin/main could not be resolved … already-COMMITTED gutting cannot be detected`, `LINT: PASS` (an uncommitted flip still exits 1) | 2026-09-30 | claude-opus-5-5-verifier |

RISK-VALUE (enumerated from the diff and the brief's Deliverables, ranked most-irreversible first):

- RISK-VALUE: NAMED, NOT DERIVED — park authority binding `parked-by` OR `authorized-by` @ statusgen/registers.go:802-803,914 — §B requires park authority "corroborated against the PR's reviews/comments"; the offline half checks only that the name is in the login map, and the reviews half (`--corroborate`) runs in no workflow on main (F1). Sufficiency without it is a human design ruling.
- RISK-VALUE: NAMED, NOT DERIVED — no maximum park horizon (absence of a literal) @ statusgen/alarms.go:175-190 — the brief says "bounded" but pins no cap; a mapped name can park to 2099 (6e/6f pattern). Whether to cap is for the human gate.
- RISK-VALUE: DERIVED — expiry boundary `!now.Before(until)` @ statusgen/alarms.go:183 — logically now >= until, exactly §A; parse and clock are both UTC; fixture 3e observed EXPIRED on the boundary day.
- RISK-VALUE: DERIVED — date layout `"2006-01-02"` @ statusgen/alarms.go:182 — Go's layout for the brief's YYYY-MM-DD; strict parse, so a bad expiry is a hard PROBLEM, never a permanent mute.
- RISK-VALUE: DERIVED — extend guard `curUntil > baseUntil` @ statusgen/registers.go:782 — zero-padded YYYY-MM-DD strings sort in date order and both sides passed the strict parse on any lint-green tree; guards add and extend, leaves narrowing unguarded, matching §B's "ADDS/EXTENDS".
- Stamp regex (corroborate.go:380), YAML keys, and the 7-day standing and flood thresholds predate this brief or are reversible knobs; not derived.

Findings — filed as #1855: **(F1, the FAIL)** Deliverable 7, CI invocation of the corroboration guard on register-touching PRs, is absent from main (`grep -rn -e '--corroborate' .github/workflows/` finds nothing); commit 9c0a7d5 defers it because the authoring App lacks the workflows permission, and nothing tracked it. So in this repo a legitimate park or resolve can never pass CI (6f), and where a map is set, a mapped name passes without that human acting on the PR (6e). **(F2)** The PROBLEM text near registers.go:810 claims the reference CI wires `--corroborate` into the lint job; it runs only `--lint`. **(F3)** The origin/main→HEAD fallback passes a committed gutting with only a NOTICE (6g); unreachable in this repo's CI (`fetch-depth: 0`), reachable for a shallow adopter checkout. **(F4)** The comment near registers.go:793 says `authorized-by` does not also authorize an unattributed park; the code at :801-803 accepts it (fixture exits 0). Scope notes: the guard lives in registers.go rather than corroborate.go as the files list says; the register migration was a no-op; §C has no instance on this board. Review-gate items 2 and 3 need F3, F4 and the horizon cap decided.

## Review
Gate: **human** (integrity-check / anti-falsification logic; irreversible). The human records the
verdict + date (with a `human:<name>` token) in the README table. The human confirms: (1) the
bounded-park field schema and re-annunciation semantics; (2) the guard's merge-base corroboration rule
+ fail-closed fallback; (3) that shelving and the guard ship as ONE change; (4) the `parked-until`
dates + `parked-by` attribution chosen for any migrated findings. A bare model sign-off does not close
this brief.
