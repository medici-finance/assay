---
brief: methodology/47
title: Findings register becomes a corroborated state machine — bounded shelving (parked) + transition guard on resolved/affects/parked
why: >-
  The FINDINGS register is the orient-integrity layer the whole methodology rests on
  ("Orient integrity is paramount", invariants.md) — yet it is untrustworthy in two
  directions at once. An agent can silently GUT a finding that demoted its own brief
  (flip resolved: yes or empty affects:, uncorroborated, CI green — #721/F-register-gut),
  and a deliberately-accepted-deferred finding CANNOT be shelved: the >7-day standing
  alarm fires identically whether a finding is neglected or consciously parked, so "park"
  as a disposition does nothing (the toothless-park gap, surfaced 2026-07-23 by the
  findings drain). A register that cannot distinguish a silenced finding from a resolved
  one, nor a parked finding from a neglected one, is not a register you can orient on.
wave: 0
depends: []
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: [721]
schema: brief-v1
authored: 2026-07-23 by Opus session (folds F-register-gut + F-36 + the toothless-park gap; issue #721)
sources:
  - "issue #721 (register-guard catches deletion not gutting — REOPENED per PR #1108)"
  - "F-register-gut (the gutting hole — in-place mutation of resolved/affects is unguarded)"
  - "[F-36](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-13-reconciler-alarm-set-never-goes-quiet-isa-18-2-violated.md) (alarm set never goes quiet — ISA-18.2 rule-1 violation; resolves as an instance of shelving once it exists)"
  - "PR #1108 (bucket-B findings drain — introduced the free-text `parked:` marker on 4 findings; those are the migration set)"
  - "PR #1110 (bucket-A findings drain — sibling context)"
  - "PR #255 (statusgen integrity-check change closed by human:<name> as a GATING decision — the precedent that anti-falsification/integrity changes are human-gate)"
  - "[scada-ooda-lineage.md](./scada-ooda-lineage.md) (ISA-18.2 / EEMUA-191 alarm-rationalization lineage the design cites)"
  - "[invariants.md](./invariants.md) (\"Orient integrity is paramount\")"
  - "[brief-16](./brief-16-nonselfwritable-lifecycle.md) (non-self-writable lifecycle — the corroboration machinery this extends)"
  - "[brief-36](./brief-36-register-tombstone-scope.md) (the tombstone/deletion guard this complements)"
  - "freshness-checked 2026-07-23 @ 5a231dce"
gate-why: >-
  This brief modifies statusgen's anti-falsification / integrity-check logic — the
  corroboration guard (corroborate.go) and the standing-alarm suppression (alarms.go).
  Standing policy (PR #255, closed by human:<name> as a gating decision, not a correctness
  rejection) is that integrity-check changes are a HUMAN gate: a wrong guard fails in
  two catastrophic directions — too loose lets falsification through (a self-park to
  2099 = a perfect silent kill), too tight bricks legitimate register edits and forces
  reviewers to instruct un-lintable cleanups. irreversible: yes because a landed guard
  that bricks edits (or a landed hole) governs every future PR touching the register
  until reverted. human:<name> confirms: the bounded-shelving field schema, the corroboration
  rule for parked/resolved transitions, the fail-closed fallback, and — the load-bearing
  dependency — that shelving and the guard SHIP TOGETHER (shelving without the guard
  makes the gutting hole strictly worse).
exec-tier: strong
exec-tier-why: >-
  (a) design decisions the facts do not fully pre-specify (the exact field schema, the
  re-annunciation semantics on expiry, the authorized-park corroboration path); (b)
  cross-artifact reasoning — the schema, alarms.go, corroborate.go, CI wiring and the
  migration must agree on one state model; (c) safety-plumbing code where a subtle error
  (a guard that misses one transition, an off-by-one on the parked-until comparison)
  survives the brief's own tests and silently re-opens the falsification hole.
consumers:
  - "assay-toolkit/statusgen (canonical source — the schema/alarms/corroborate/CI changes land HERE, not tools/statusgen/): follow-up (the implementation PR in the sibling repo; this brief specs it)"
  - "tools/statusgen/ (frozen mirror — do NOT edit; refreshed only when the assay-toolkit release re-syncs): out-of-scope (frozen, assay-dogfood/03 tripwire)"
  - ".assay-versions (pins the statusgen release the change ships in): fixed-here (the pin bump is the in-repo half of the deploy — bump statusgen tag + sha256 to the release carrying this change)"
  - "docs/streams/findings/*.md (4 currently-parked findings use the free-text `parked:` form): fixed-here (migrate to the bounded parked-until/parked-by/parked-reason form)"
---

# Brief 47 — Findings register becomes a corroborated state machine

**Cross-repo brief.** The IMPLEMENTATION (statusgen source: schema, `alarms.go`,
`corroborate.go`, CI wiring) lands in **`medici-finance/assay-toolkit/statusgen`**,
then ships to this repo as a pinned release: bump `statusgen <tag> <sha256>` in
`.assay-versions`. The in-repo `../assay-toolkit/statusgen/**` is a **frozen mirror** — editing a
`.go` file there trips the `statusgen.yml` transition tripwire; do not touch it. This
follows the #1101/#128 pattern (statusgen behaviour change = assay-toolkit release +
pin bump, never an in-repo Go edit). The **finding migrations** and this brief live in
THIS repo.

## Context

files:
- `.assay-versions` (in-repo: bump the statusgen pin to the release carrying this change)
- docs/streams/findings/2026-07-17-register-guard-catches-deletion-not-gutting.md (migrate `parked:` → bounded form)
- docs/streams/findings/2026-07-17-bug-close-flow-four-step-chain.md (migrate)
- docs/streams/findings/2026-07-17-ready-flip-is-convention-only.md (migrate)
- docs/streams/findings/2026-07-17-shared-guardrails-duplicated-across-skills.md (migrate)
- `docs/streams/methodology/README.md` (status table row + waves for methodology/47)

out-of-repo files (the actual code — separate `medici-finance/assay-toolkit` PR, NOT edited from this repo's tree):
- `../assay-toolkit/statusgen/model.go` (Finding struct — add Parked state fields)
- `../assay-toolkit/statusgen/parse.go` / findings loader (parse the new frontmatter fields)
- `../assay-toolkit/statusgen/alarms.go` (`standingAlarmNotices` / `computeAlarms` — shelving suppression)
- `../assay-toolkit/statusgen/corroborate.go` (extend the guard to `resolved`/`affects`/`parked-*` transitions)
- `../assay-toolkit/statusgen/*_test.go` (TDD coverage for every new branch)
- the assay-toolkit CI wiring that invokes the guard on register-touching PRs

facts:
- register source of truth: `docs/streams/findings/<date>-<slug>.md` per-entry files (methodology/23); `FINDINGS.md` is a generated, main-CI-only view.
- Finding struct today (`model.go:71`): `ID, Date, Title, Affects []string, Ack, Resolved bool, FileRel`. No park/quiescence state.
- standing alarm today (`alarms.go` `standingAlarmNotices`): keys ONLY on `a.Over` (age > `defaultStandingAgeDays = 7`) for un-`Resolved` findings. The NOTICE text literally says "resolve or park it" — but there is no park: a `parked:` marker is ignored, so an accepted-deferred finding alarms identically to a neglected one (ISA-18.2 rule-1: a console that is never quiet).
- guard today (`corroborate.go`): a standalone `--corroborate <pr>` subcommand scans a PR diff for `human:<name>` stamps and corroborates each against the PR's GitHub reviews/comments (APPROVED review by the mapped login, or an approval-phrase comment). It does NOT look at `resolved`/`affects`/`parked` changes at all. The tombstone guard (deletedRegisterFiles, methodology/23/36) catches DELETION of an entry but not in-place field mutation.
- CI (`statusgen.yml`): downloads the pinned binary, runs `--lint --budget --changed`; there is currently NO CI invocation of `--corroborate`.
- currently-parked findings (PR #1108, free-text form): `parked: "2026-07-23 — …Accepted-deferred."` on the 4 files above — no `parked-until`, no attributed `parked-by`. These are the migration set.
- HumanLoginMap (`corroborate.go`): `ian → human:<name>`; adding a name to this map is itself a reviewed change (the map IS the name=login claim).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done. This is `gate: human`: **human:<name> signs off before `verified`** (integrity-check human-gate, PR #255 precedent; and irreversible → human-before-verified, #159).
- Do NOT edit `../assay-toolkit/statusgen/**` Go files (frozen mirror — tripwire). Code changes are a SEPARATE assay-toolkit PR; this repo gets only the `.assay-versions` pin bump + finding migrations + README row.
- NEVER commit `STATUS.md` on this branch (generated, single-writer = main CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## The problem (three folded items, one underlying gap)

statusgen models a finding as a `resolved` boolean plus a deletion guard, when the
register actually needs a small **state machine** with each transition corroborated:

```
open ──park(until, by, reason)──▶ parked ──expiry──▶ open (re-annunciate, louder)
  │                                  │
  └──────────resolve(by, evidence)───┴──▶ resolved
```

1. **Gutting (F-register-gut / #721):** in-place mutation of a finding's `resolved`/`affects`
   is unguarded. An agent whose brief a finding demoted flips `resolved: yes` (or empties
   `affects:`) in the same PR, uncorroborated, and `--lint` passes. Because `affects:` is
   what blocks the brief, gutting it silently unblocks the brief — the exact self-serving
   move the register exists to prevent.
2. **Toothless park (surfaced 2026-07-23):** `alarms.go` suppresses the >7-day standing
   NOTICE ONLY on `resolved: yes`. A `parked:` marker is ignored — a consciously
   accepted-deferred finding alarms exactly like a neglected one, so "park" does nothing.
3. **Reconciler-alarm (F-36):** an alarm set that can never go quiet, with no
   shelving/quiescence, is ISA-18.2-broken. Same alarm-management theme; it resolves as
   an **instance** of the shelving mechanism once it exists.

## The design to spec (present; human:<name> approves before implementation)

### A. Bounded shelving (ISA-18.2 / EEMUA-191, per scada-ooda-lineage.md)
A park is a **snooze, not a mute**, and must be bounded:
- `parked-until: <YYYY-MM-DD>` — REQUIRED. No open-ended parks (an unbounded park is a
  disguised resolve). Missing/empty on a parked finding → `--lint` PROBLEM.
- `parked-by: <authority>` — REQUIRED. The authorizing party (`human:<name>` form, same
  vocabulary as lifecycle stamps).
- `parked-reason: <prose>` — REQUIRED. Why it is accepted-deferred.
- `alarms.go` suppresses the standing NOTICE **only while `now < parked-until` AND the
  park is authorized** (see B). 
- **On expiry (`now >= parked-until`) it RE-ANNUNCIATES, louder** than a plain standing
  alarm: a distinct "park expired — re-decide (extend, resolve, or act)" NOTICE that the
  desk/retro cannot mistake for a fresh standing alarm. A park buys a bounded window, then
  forces a fresh decision — it never silently becomes permanent.
- Flood accounting (`computeAlarms`): decide and STATE whether a validly-parked finding
  counts toward the flood threshold (recommended: parked findings are excluded from the
  active-flood count while their park is live, since they are consciously shelved — but an
  EXPIRED park counts again).

### B. The guard half — MANDATORY, ships in the SAME change (this closes #721)
The `parked-*` and `resolved` transitions get the **same corroboration** `corroborate.go`
already applies to `human:<name>` lifecycle stamps:
- A PR that flips `resolved: no → yes`, empties/narrows `affects:`, or ADDS/EXTENDS a
  `parked-until` on a finding **present at the PR merge-base** must be **independently
  attributed** — an authorized `parked-by`/resolver corroborated against the PR's GitHub
  reviews/comments (the existing APPROVED-review / approval-phrase path), OR a
  `Verified-by`-style trailer. An agent **cannot self-park or self-resolve**.
- In-place mutation of these fields is guarded exactly like deletion is (the tombstone
  guard's sibling). The guard diffs the finding's `resolved`/`affects`/`parked-*` against
  the merged/merge-base version and **hard-fails an unattributed change**.
- **Fail-closed:** if corroboration cannot be evaluated (no PR context, gh unavailable),
  the guard fails closed on a register-field change — never green-by-default.

> **Load-bearing dependency (state it in the PR, do not split):** shipping shelving
> WITHOUT the guard makes the gutting hole strictly WORSE — it hands the attacker a
> perfect silent kill (`parked-until: 2099`, uncorroborated, alarm muted for 73 years).
> The two halves are ONE change. A reviewer who sees only the shelving half must bounce it.

### C. Reconciler-alarm (F-36) resolves as an instance
Once bounded shelving exists, F-36's three permanently-standing reconciler alarms (which
cannot clear while #35 is open) are **parked** — bounded, attributed, reasoned — instead
of falsely resolved or left screaming. F-36 is not fixed BY this brief's code; it becomes
**resolvable via** this mechanism (a separate follow-up parks them). Note this in F-36's
disposition when the mechanism lands; do not resolve F-36 in this brief.

## Task

**This brief's in-repo deliverables (the assay-toolkit code PR is separate):**

1. **Migrate the 4 currently-parked findings** to the bounded form. Replace each free-text
   `parked: "<date> — …"` with `parked-until:`, `parked-by:`, `parked-reason:`. Where the
   authorizing human is not already recorded, leave `parked-by` as the value the drain PR
   asserted and FLAG in the migration that it needs human:<name>'s attribution at sign-off (do not
   fabricate a `human:` stamp). Pick a concrete `parked-until` per finding (recommend one
   retro-cycle out, ~2026-08-06) — these are DECISION-NEEDED items human:<name> confirms.
2. **Bump `.assay-versions`** `statusgen <tag> <sha256>` to the assay-toolkit release that
   carries the schema/alarms/corroborate/CI change. (This step only lands once the
   sibling-repo PR is released — until then, note the pending pin in the PR body.)
3. **Update `docs/streams/methodology/README.md`**: add the methodology/47 status-table row
   and place it in the waves block (Wave 0, no deps).

**The assay-toolkit code PR (spec — implemented in the sibling repo, TDD, stdlib+yaml.v3):**

4. `model.go`: add park state to `Finding` (`ParkedUntil string`, `ParkedBy string`,
   `ParkedReason string`; keep `Resolved bool`).
5. findings parser: parse the three `parked-*` fields; treat all-absent as `open`.
6. `alarms.go`: shelving suppression + louder re-annunciation on expiry + flood accounting
   (design §A). Add `--lint` PROBLEM for a park missing any required field.
7. `corroborate.go`: extend the guard to `resolved`/`affects`/`parked-*` transitions against
   the merge-base, fail-closed (design §B).
8. assay-toolkit CI: invoke the extended guard on register-touching PRs (there is no such
   CI step today — add one; mirror `statusgen.yml`'s pinned-binary discipline).
9. Tests for every new branch: authorized park suppresses; unauthorized park PROBLEMs;
   expired park re-annunciates; self-resolve fails; self-gut of `affects` fails; corroborated
   resolve passes.

## Verify (executable — no prose-only DoD items)

| # | Command | Expect |
|---|---------|--------|
| 1 | `git -C . log origin/main --oneline -- .assay-versions \| head -1` | shows the pin bump commit (once the release lands) |
| 2 | `grep -L 'parked-until' docs/streams/findings/2026-07-17-register-guard-catches-deletion-not-gutting.md docs/streams/findings/2026-07-17-bug-close-flow-four-step-chain.md docs/streams/findings/2026-07-17-ready-flip-is-convention-only.md docs/streams/findings/2026-07-17-shared-guardrails-duplicated-across-skills.md` | empty output — every migrated file carries `parked-until` (no un-migrated `parked:`) |
| 3 | `grep -c 'parked-by\|parked-reason' docs/streams/findings/2026-07-17-register-guard-catches-deletion-not-gutting.md` | ≥ 2 (both attribution fields present) |
| 4 | `statusgen --root . --lint` (pinned binary carrying the change) | exit 0; a park missing `parked-until` PROBLEMs; an expired park emits the louder re-annunciation NOTICE |
| 5 | `git -C . diff --name-only origin/main...HEAD \| grep -qx STATUS.md; echo $?` | `1` (STATUS.md NOT modified on the branch) |
| 6 | (assay-toolkit) `go test ./statusgen/ -run 'Park|Corroborat|Shelv|Alarm'` | exit 0; self-park/self-resolve/self-gut all FAIL, corroborated transitions PASS |
| 7 | (assay-toolkit) inject `resolved: no→yes` on a merge-base finding with no corroboration, run the guard | exit 1 (hard-fail, fail-closed) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" requires this filled by a NON-implementer, at a verifier tier
     above the cheap-tier list (risk-flagged brief) and, being irreversible +
     integrity-check, only AFTER human:<name>'s human sign-off. -->

## Review
Gate: **human** (integrity-check / anti-falsification logic; irreversible). human:<name> records
the verdict + date in the methodology README table (`human:ian`). The human confirms:
(1) the bounded-park field schema and re-annunciation semantics; (2) the guard's
merge-base corroboration rule + fail-closed fallback; (3) that shelving and the guard
ship as ONE change; (4) the `parked-until` dates + `parked-by` attribution chosen for the
4 migrated findings. A bare model sign-off does not close this brief.
