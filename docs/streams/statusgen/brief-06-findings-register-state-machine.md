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
decision-issue: 2012
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
version: 2
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

## Human decision

**2026-10-04 — ruled: option 2, approve with changes**
([ruling](https://github.com/medici-finance/assay/issues/2012#issuecomment-5977123755) on the
decision gate #2012, approving the
[recommendation](https://github.com/medici-finance/assay/issues/2012#issuecomment-5977069464) as
written). The changes, folded into the design and Task below:

1. **Bounded park horizon.** `parked-until` may be at most **90 days** after the date of the commit
   that sets or extends it; a later value is a `--lint` PROBLEM, and MISSING in the online lane,
   however the park is authorized (§A, §B).
2. **Only `parked-by` authorizes a park.** `authorized-by` keeps authorizing `resolved` / `affects` /
   `ack` transitions, never a park add or extension — offline and online (§B). This settles the
   comment/code mismatch reported in #1855 (F4) in the comment's direction.
3. **No-base fails closed offline too.** When the exact base ref cannot be resolved, `--lint`
   refuses register transitions instead of comparing against HEAD with a NOTICE (#1855 F3; §B).
4. **The online check runs a guard built from the PR's base side** (the merge commit's first
   parent), never from the PR's own tree. The CI wiring (Task 7) therefore lands as a follow-up
   after the code PR merges, and the remaining question of which merge-base the online lane trusts
   is judged against that wiring.
5. **Unchanged:** shelving and the guard ship together.

Left as is: the definition of "acted on the PR" stays the stamp lane's existing one; any change to it
is its own item.

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
- **Horizon (Human decision, item 1):** a park that is ADDED or EXTENDED may run at most **90 days**
  past the date of the commit that sets that `parked-until` value (a value not yet committed counts
  as set today; a commit dated in the future counts as today, so post-dating cannot stretch it). A
  later date is a PROBLEM offline and MISSING online whoever authorized it — no approval buys a
  longer snooze; a park still needed at expiry is extended again, a fresh guarded transition. A
  landed park left as it is, or narrowed, is not re-judged.
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
- **Authority per transition (Human decision, item 2):** a `resolved` / `affects` / `ack` move is
  authorized by a mapped human under `authorized-by:`; a park add/extend by a mapped human under
  `parked-by:` and nothing else. `authorized-by` never authorizes a park, offline or online.
- In-place mutation of these fields is guarded exactly like deletion is (the tombstone guard's
  sibling). The guard diffs the finding's `resolved`/`affects`/`parked-*` against the merge-base
  version and **hard-fails an unattributed change**.
- **Fail-closed:** if corroboration cannot be evaluated (no PR context, gh unavailable), the guard
  fails closed on a register-field change — never green-by-default. The same holds offline (Human
  decision, item 3): when the exact base ref (`refs/remotes/origin/main`, resolved exactly, never by
  git's short-name rules) does not resolve to a merge-base, `--lint` refuses register transitions
  with one PROBLEM that no `--changed` scope removes, rather than comparing against HEAD — where a
  committed transition compares with itself and passes.
- **Guard provenance (Human decision, item 4):** the online check in CI runs a statusgen built from
  the PR's BASE side (the merge commit's first parent), never from the PR's own tree, so a PR that
  edits `statusgen/` cannot run its own version of the guard that judges it.

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
   merge-base, fail-closed (design §B), with the Human-decision rules in both the offline `--lint`
   gate and the online lane:
   - the 90-day park horizon, measured from the commit that sets or extends the park (item 1);
   - `parked-by` as the only authority for a park add/extend (item 2);
   - offline, an unresolvable exact base refuses register transitions instead of falling back to
     HEAD (item 3).
7. CI: invoke the extended guard on register-touching PRs (there is no such CI step today — add one;
   mirror the pinned/built-binary discipline). Per the Human decision (item 4) the step builds the
   guard from the PR's base side, and it lands as a follow-up PR after the code PR merges.
8. Tests for every new branch: authorized park suppresses; unauthorized park PROBLEMs; expired park
   re-annunciates; self-resolve fails; self-gut of `affects` fails; corroborated resolve passes; a
   park added or extended past 90 days fails however it is authorized; `authorized-by` alone does
   not authorize a park; an unresolvable base refuses offline.

## Verify (executable — no prose-only DoD items)

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -rl '^parked-until:' docs/streams/findings/ 2>/dev/null; echo done` | lists every bounded-parked file (none on a fresh board) then `done` — parks use the bounded `parked-until` form, not the free-text `parked:` marker |
| 2 | check | `grep -rn -e '^parked:' docs/streams/findings/ 2>/dev/null; echo rc=$?` | no free-text `parked:` key survives the migration |
| 3 | check | `statusgen --root . --lint` | exit 0; a park missing `parked-until` PROBLEMs; an expired park emits the louder re-annunciation NOTICE |
| 4 | check | `git diff --name-only $(git merge-base HEAD origin/main) HEAD -- STATUS.md` | empty output — STATUS.md NOT modified on the branch |
| 5 | check +flow | `cd statusgen && GOWORK=off go test .` | exit 0 with the new park tests present: authorized park suppresses the standing NOTICE, a park missing a required field PROBLEMs, an expired park re-annunciates, self-park / self-resolve / self-gut FAIL, a park past the 90-day horizon FAILs, `authorized-by` alone never authorizes a park, an unresolvable base refuses register transitions offline, corroborated transitions PASS |
| 6 | check +mutation | inject `resolved: no→yes` on a merge-base finding with no corroboration, run the guard | exit 1 (hard-fail, fail-closed) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. This is
     gate: human + irreversible, so a human signs off before any row is marked verified. -->
### Non-implementer verifier run: 2026-09-30, assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian). VERIFY: FAIL — Verify rows 1-6 pass by hand, but brief Deliverable 7 (CI invocation of the extended corroboration guard on register-touching PRs) is absent on main b0088804294b, so §B's no-self-park/no-self-resolve property is not enforced end to end (tracked #1855). The execution witness also cannot close: Verify row 6 is prose → could-not-run → check-verified exit 1.

- FAIL (Evidence only; gate: human, irreversible: yes — status stays implemented). Verify rows 1-6 all produce their stated Expect by hand. Deliverable 7, the CI step that runs the corroboration guard on register-touching PRs, is absent on main, as already tracked in #1855. The execution witness cannot close: Verify row 6 is prose, so verifyrun records it could-not-run; the row re-author is filed separately. This pass's tables are the Evidence of record, since #1855's own Evidence never landed on the brief.

**Hand run**:
| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `grep -rl '^parked-until:' docs/streams/findings/ 2>/dev/null; echo done` | bounded-parked files then done | exit 0; output is only "done" — no parked findings on main (one finding, F-fleet-audit-gap, open, no park fields) | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `grep -rn -e '^parked:' docs/streams/findings/ 2>/dev/null; echo rc=$?` | no free-text parked: key | exit 0; output "rc=1" (grep found no free-text parked: key) | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `statusgen --root . --lint` | exit 0; missing parked-until PROBLEMs; expired park re-annunciates louder | main: exit 0, "LINT: PASS" (statusgen built from main source, version dev). Throwaway-clone fixtures: a park with parked-by and parked-reason but no parked-until gives exit 1, "PROBLEM: findings register: F-vfx-nountil: malformed park — missing/invalid: parked-until"; a park with parked-until 2026-09-01 gives "NOTICE: park EXPIRED — re-decide: F-vfx-expired was parked until 2026-09-01"; a live park to 2027-01-01 emits no standing NOTICE while an unparked sibling does | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | `git diff --name-only $(git merge-base HEAD origin/main) HEAD -- STATUS.md` | empty output | exit 0, empty output (HEAD = origin/main b0088804294b) | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | `cd statusgen && GOWORK=off go test .` | exit 0 with park tests present | exit 0, "ok github.com/medici-finance/assay/statusgen 38.740s"; the park and gutting tests run verbose: 40 PASS, 0 FAIL (park suppresses, malformed park PROBLEMs, expired park re-annunciates, park add/extend unauthorized fails, resolved flip / affects emptied / affects narrowed unauthorized fails, authorized transitions pass) | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | `perl -pi -e 's/^resolved: false/resolved: true/' docs/streams/findings/F-fleet-audit-gap.md && statusgen --root . --lint` (throwaway clone, origin/main = b0088804294b) | exit 1 hard-fail | exit 1, "PROBLEM: register field-gutting (unauthorized): docs/streams/findings/F-fleet-audit-gap.md — resolved flipped no->yes vs the version landed at the merge-base"; same exit 1 when the flip is committed, when affects is emptied, and when parked-until 2099-01-01 is added | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian) |

**Execution witness**:
statusgen verifyrun --dry-run --brief docs/streams/statusgen/brief-06-findings-register-state-machine.md --root <wt> → exit 2 (rows 1-5 pass, row 6 could-not-run). Runner restamped (forge-identity) → (git-config) per dispatch; nothing else edited.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -rl '^parked-until:' docs/streams/findings/ 2>/dev/null; echo done` | pass exit=0 | sha256:d117fa006ba9 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (git-config) |
| 2 | `grep -rn -e '^parked:' docs/streams/findings/ 2>/dev/null; echo rc=$?` | pass exit=0 | sha256:91d957f8f274 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (git-config) |
| 3 | `statusgen --root . --lint` | pass exit=0 | sha256:a9236fffe42e | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (git-config) |
| 4 | `git diff --name-only $(git merge-base HEAD origin/main) HEAD -- STATUS.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (git-config) |
| 5 | `cd statusgen && GOWORK=off go test .` | pass exit=0 | sha256:02906d388652 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (git-config) |
| 6 | `resolved: no→yes` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:6a46790d7c5a | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (git-config) |

**check-verified** (hypothetical README flip on a throwaway clone):
Throwaway clone <wt>/.vtmp/cv (git clone --no-hardlinks, detached b0088804294b, refs/remotes/origin/main pinned to it); hand table + witness table inserted in ## Evidence before ## Review. Nothing committed, nothing pushed.
- `statusgen --root . brief --check-verified statusgen/06` (form as dispatched) → exit 0, but it did NOT run the brief subcommand: the leading --root made statusgen do a full regen ("wrote STATUS.md", "wrote register views (INTAKE.md, FINDINGS.md)"). Exit 0 here is not a check-verified pass. Correct form per `statusgen brief` usage is `statusgen brief --root . --check-verified statusgen/06`.
- `statusgen brief --root . --check-verified statusgen/06`, status left `implemented` → exit 1: `brief: statusgen/06: verified outcome requires status verified or done, got "implemented"`.
- Same, README row hypothetically flipped to `verified` → exit 1: `brief: statusgen/06: verified outcome requires passing execution witnesses: row 6: could-not-run — the witness records could-not-run — the row produced no verdict`.
- `statusgen --root . --lint`, Evidence inserted, status `implemented` (the real landing shape) → exit 0, `LINT: PASS`; brief-06 NOTICEs only: [verify-obligation] owes a `+flow` Verify row; gate:human at implemented with no decision-issue.
- `statusgen --root . --lint` with the hypothetical `verified` flip → exit 1, 2 PROBLEMs, both expected for a model-only flip: "risk.irreversible=yes but the verified Reviewed entry "" names no human" and "verified Evidence section is not backed by the roster's verifier role — address "not.committed.yet"" (uncommitted throwaway).
- `statusgen verifyrun --check` on the inserted table → exit 2: 5 pass, 0 fail, 1 could-not-run (row 6).

RISK-VALUE: NAMED, NOT DERIVED — parkAuthorityKeys = "parked-by" (statusgen/registers.go:914) | "authorized-by" (statusgen/registers.go:875, accepted for a park add/extend at statusgen/registers.go:801-803) — the binding is the only thing between an agent and a silent long park or resolve. It is right only when paired with the online `statusgen --corroborate` check, and no workflow on main b0088804294b invokes that check (rc=1 grep over .github/workflows). A bare model can't derive it as correct while half of the control is missing. Already tracked as #1855 (F1, F4).
RISK-VALUE: DERIVED — parkExpiry = time.Parse("2006-01-02", parked-until) with !now.Before(until) @ statusgen/alarms.go:182-183 — brief §A defines expiry as now >= parked-until. The date parses to 00:00 UTC on parked-until, so the park is live strictly before that instant and re-annunciates from it on, which matches the brief's inequality. A date that does not parse under the strict zero-padded layout never shelves (classifyPark → parkMalformed) and is a hard --lint PROBLEM (statusgen/alarms.go:361). Fixture: parked-until 2026-09-01 → "park EXPIRED — re-decide"; 2027-01-01 → silent.
RISK-VALUE: DERIVED — parkExtendGuard = curUntil > baseUntil (lexical) @ statusgen/registers.go:782, plus the add case baseUntil == "" && curUntil != "" @ statusgen/registers.go:780 — any parked-until that can shelve must pass the strict YYYY-MM-DD parse. For zero-padded 4-digit-year dates, lexical order equals chronological order, so ">" is exactly "later". A non-canonical value fails lint anyway, and a lexically larger non-canonical value is flagged too (the safe direction). Fixtures: adding 2099-01-01 quoted or unquoted with no mapped authority → exit 1.

Notes:
- Verify rows 1-6 all produce their stated Expect by hand at b0088804294b (table above). The code half of the brief (model/parse fields, shelving suppression, louder expiry re-annunciation, flood exclusion for live parks, malformed-park PROBLEM, offline merge-base guard on resolved/affects/ack/parked-until) is on main and behaves as specified. It was checked with independent fixtures on throwaway clones: resolved flip in the working tree or committed, affects emptied, park added to 2099 with an agent/unmapped parked-by → each exit 1.
- FAIL basis — Deliverable 7 (CI invokes the extended guard on register-touching PRs) is ABSENT from main. `.github/workflows/assay-statusgen.yml`'s PR job still runs only `statusgen --root . --lint`, and no workflow calls `--corroborate`. The implementing commit 9c0a7d57f (PR #84) itself records the CI wiring as a follow-up. So brief §B's "an agent cannot self-park or self-resolve" is not machine-enforced end to end on this repo. The field-gutting PROBLEM text (statusgen/registers.go:811) still claims the reference CI wires `--corroborate` into that lint job. All of this is ALREADY TRACKED in #1855 (open, raised by an earlier verify pass today at b89b3957225e), along with the no-origin/main fail-open (F3: my fixture reproduces it — committed resolve flip with origin/main deleted → exit 0 plus the degraded NOTICE) and the authorized-by-authorizes-a-park comment/code mismatch (F4: the comment at statusgen/registers.go:790-796 contradicts :801-803). No new filing for these. #1855 says its Evidence is in the brief's 2026-09-30 Evidence section, but on main b0088804294b the brief's ## Evidence section is EMPTY, so that earlier pass's Evidence never landed. This pass's tables are landed as the Evidence of record.
- Check-definition finding (filed separately): Verify row 6 is prose ("inject `resolved: no→yes` … run the guard"). verifyrun runs its first code span `resolved: no→yes` → exit 127 could-not-run, so check-verified can never pass on this row as written. The lint prose-led-command detector did NOT flag it: brief-06 has no [prose-led-command] NOTICE. Suggested re-author: an executable fixture command, as in hand row 6.
- Tooling finding (filed separately): `statusgen --root . brief --check-verified <key>` (flag before subcommand) silently runs a full regen and exits 0, writing STATUS.md and the register views, instead of the brief check. A desk reading exit 0 would take it as a check-verified pass. Not #1925 or #1926.
- Design gap for the human gate (no literal to enumerate, so not a RISK-VALUE entry): no maximum park horizon exists. "Bounded" is satisfied by any parseable date, so 2099 is accepted once authorized. ISA-18.2 shelving normally caps shelve duration. Already raised in #1855.
- Reversible knobs, ranked last, no derivation owed: defaultStandingAgeDays = 7 @ statusgen/alarms.go:39, defaultFloodThreshold = 7 @ statusgen/alarms.go:43, alarmRatePeriodDays = 7 @ statusgen/alarms.go:46 (all pre-date the diff, from 2026-08-11), and the flood comparison ActiveCount > FloodThreshold @ statusgen/alarms.go:269.
- Other lint NOTICEs on brief-06: owes a `+flow` Verify row; gate:human with no decision-issue.

VERIFY: FAIL — Verify rows 1-6 pass by hand, but brief Deliverable 7 (CI invocation of the extended corroboration guard on register-touching PRs) is absent on main b0088804294b, so §B's no-self-park/no-self-resolve property is not enforced end to end (tracked #1855). The execution witness also cannot close: Verify row 6 is prose → could-not-run → check-verified exit 1.
### Correction: 2026-09-30, assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian). VERIFY: FAIL

- The execution-witness table in the entry above is not verbatim. On all six rows the runner source tag was hand-changed from the emitted `(forge-identity)` to `(git-config)`, following a desk dispatch instruction that was out of date. That tag is a derived value, never caller text. The table is restated below exactly as `statusgen verifyrun` emitted it; the "Runner restamped" sentence above does not apply to it. No other cell changed, and the verdict is unchanged.
- The two findings marked "filed separately" above are the Verify row 6 re-author on #1927 and the flag-before-subcommand regen bug #1954.
- In the check-verified hypothetical above, only the README Status cell was set to `verified`; the Verified cell was left empty.

**Execution witness** (restated as emitted):
| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -rl '^parked-until:' docs/streams/findings/ 2>/dev/null; echo done` | pass exit=0 | sha256:d117fa006ba9 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -rn -e '^parked:' docs/streams/findings/ 2>/dev/null; echo rc=$?` | pass exit=0 | sha256:91d957f8f274 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 3 | `statusgen --root . --lint` | pass exit=0 | sha256:a9236fffe42e | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 4 | `git diff --name-only $(git merge-base HEAD origin/main) HEAD -- STATUS.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && GOWORK=off go test .` | pass exit=0 | sha256:02906d388652 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 6 | `resolved: no→yes` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:6a46790d7c5a | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |

## Review
Gate: **human** (integrity-check / anti-falsification logic; irreversible). The human records the
verdict + date (with a `human:<name>` token) in the README table. The human confirms: (1) the
bounded-park field schema and re-annunciation semantics; (2) the guard's merge-base corroboration rule
+ fail-closed fallback; (3) that shelving and the guard ship as ONE change; (4) the `parked-until`
dates + `parked-by` attribution chosen for any migrated findings. A bare model sign-off does not close
this brief.
