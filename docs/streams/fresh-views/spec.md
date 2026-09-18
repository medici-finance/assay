# fresh-views — scoping document

**Status:** draft — proposed 2026-09-16; NOT yet approved. A stream is scaffolded only from a
scoping doc whose header parses `**Status:** approved` (`spec/lifecycle-v1.md` §8.1, the
`stream-source` lint), so this stream ships `status: parked` in its README until a human
approves this doc by landing the flip. `draft` is the recognized not-yet-approved lifecycle
token (`statusgen/lifecycleheader.go`: the states are `draft` / `approved` / `routed`); this
document deliberately does not fake `approved`.
**Routes-to:** docs/streams/fresh-views/

## 1. The problem, stated from observation

A **derived view** is any artifact a tool computes from the state of `main`: a dispatch plan, a
board snapshot, the set of open PRs, a mergeability verdict, a mirrored version string, a
reconciled lifecycle cell, a landed-commit sha. Every such view is a *photograph* of `main`
taken at one instant, and `main` moves continuously. The recurring defect across the desk
tools and `statusgen` is the same shape every time: **a tool photographs `main`, then acts on
the photograph after `main` has moved past it** — and nothing tells the tool its photograph is
stale. The tool guesses instead of refusing.

The house discipline "**refresh, don't remember**" (re-fetch, never trust recall; every
decision binds to this-cycle reads) exists precisely to paper over this gap by hand. A
discipline that agents must remember to follow is a control with a single point of failure: the
agent. This stream's job is to make freshness a **property the code enforces** rather than a
habit the code assumes.

The open evidence — every one of these is the same seam surfacing in a different tool
(read 2026-09-16, all OPEN on `medici-finance/assay`):

| Issue | Derived view | How it goes stale |
|---|---|---|
| #334 | `verifyloop plan`'s dispatchable list | re-lists briefs as dispatchable after they were flipped `verified` on `main`; the plan reads a cached/pre-flip board snapshot |
| #339 | the ready-flip mergeability verdict | mergeability is checked once at flip time; an unrelated merge to `main` regresses a ready PR to `CONFLICTING` and nothing re-checks it |
| #228 | `scanloop`'s "open scan PRs" set | the coalesce view is session-local — it cannot see another session's or a hand-cut scan PR, so duplicates pile up |
| #806 | the landed-commit sha `deskevidence` prints | prints a locally-*predicted* sha that does not exist on `main`; a push race rebased the real commit |
| #882 | the shared append-only log `verify-outcomes.jsonl` | concurrent Evidence PRs each append one line and serial-conflict on the single file; no `merge=union` discipline |
| #1192 | the `statusgen` version mirrored in `spec/brief-v1.md`'s header | a value copied from the release drifts (`v0.22.0` vs released `v1.0.9`); nothing re-checks the mirror at release time |
| #859 | the vendor values mirrored in `plugins/assay/references/codex.md` | a mirrored default drifted from its source with no re-measurement note |
| #1176 | `statusgen reconcile`'s per-brief lifecycle cell | derives the cell by exact-string-matching an *immutable* pre-migration PR trailer; after the brief-v2 id flag-day the match is a 100% false-negative |

## 2. The design principle

**Treat every derived view as a pure function of `main` at a known sha.**

Three obligations follow, and the stream drives toward all three:

1. **Stamp the input.** Every derived read records the sha of `main` (or the PR-set/vendor
   source) it was computed against. A view with no input stamp cannot be checked for staleness;
   the stamp is what makes the function's input observable.

2. **Refuse when stale — do not guess.** Before a tool *acts* on a derived view (dispatches,
   flips, coalesces, cuts a release), it re-reads the current head of its input and compares it
   to the stamp. If the head has advanced, the tool REFUSES with a clear message naming the
   stamped sha and the current head, and the caller re-derives. A refusal is a work item, never
   a silent wrong action. This is the "refresh, don't remember" discipline turned into a
   fail-closed check.

3. **Shared logs get single-writer or union-merge discipline.** An append-only log written by
   many concurrent producers (`verify-outcomes.jsonl`) must not serial-conflict on
   semantically-independent appends: `merge=union` in `.gitattributes` (the standard remedy,
   already used for the shared blog index) keeps both sides' lines. A written value that
   downstreams key on (a landed sha) must be re-read from the forge after the write, never
   predicted, so the reported view matches ground truth.

The **mirror-freshness** corollary (#1192, #859) is the same principle applied to a value
mirrored from a source outside `main` — a release tag, a vendor reference doc. The mirror
carries a source + a measured-date, and a mechanical gate (a release-time assertion, a
freshness leash) fails when the mirror drifts from its source, rather than waiting for a human
to notice.

## 3. What this stream is NOT

- **Not `derived-board`.** That stream makes a board's lifecycle cell a pure function of *PR
  history* (single-writer generated table, derived from trailers/witnesses/approvals). It
  answers *what a cell may claim and from what*. This stream answers a different question:
  *is the snapshot a tool is acting on still current, and does the tool refuse when it is
  not*. `derived-board` derives the content; `fresh-views` guards the freshness of any
  derived read and the concurrency of the shared logs. `derived-board/03` (`reconcile`) is
  where the two touch — #1176 is a `reconcile` correctness defect — but the mechanism
  (ref-resolution across a schema migration) is a freshness-of-derivation concern, not a
  new claim about lifecycle cells, so it is homed here and cross-referenced there.
- **Not a rewrite of the loop engine.** The stall/liveness taxonomy in
  `tools/desk/internal/loopengine` is `desk-supervision`'s territory (a running worker going
  silent). This stream is about the *inputs* a plan/flip/coalesce step reads, not about
  supervising the workers those steps launch.
- **Not "add more assertions."** The refuse-when-stale check is one designed control at the
  act boundary of each tool, keyed on an observable input stamp — not a check duplicated
  across call sites.

## 4. Scope — the briefs

Six briefs in two waves. The shared helper (brief 01) is the head of the critical path; the
read-side applications (02, 03) consume it; the write-side / mirror / reconcile mechanisms
(04, 05, 06) are independent parallel families addressing the same seam.

See [README.md](README.md) for the generated board, the critical path, and the waves.

## 5. Open questions (each owned by a brief or a human ruling)

1. **Where the input stamp lives** (brief 01). A plan/board artifact can carry the stamp
   inline (a header line); an ephemeral in-process view carries it in memory. The helper
   defines the stamp shape; each consumer chooses inline vs in-memory.
2. **Refuse vs auto-refresh** (briefs 02, 03). The default is REFUSE-and-let-the-caller-
   re-derive (fail-closed, observable). A tool that can cheaply re-derive in place MAY do so,
   but must log that it did. The helper offers both; the per-tool choice is recorded in each
   brief.
3. **`merge=union` ordering/dedup** (brief 04). #882 flags that a consumer needing strict line
   ordering or dedup would be broken by union-merge. Brief 04 must confirm no consumer of
   `verify-outcomes.jsonl` depends on order or uniqueness before setting the attribute.
4. **Approval.** This doc is `draft`. Activating the stream (`status: parked` → `active`, and
   this header `draft` → `approved`) is a human ruling; until then the briefs are authored and
   tracked but the stream is not in the active-stream cap and its work is not dispatchable.
