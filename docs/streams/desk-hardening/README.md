---
stream: desk-hardening
status: active
priority: P1
track: platform
serves: assay
tiering: implement=cheap verify=strong
---

# Desk Hardening Stream

Turns ~20 open desk-process **findings** — filed by the intake / batch-fanout / pr-review /
verify desks over 2026-07-13…07-23 — into tracked, dispatchable briefs. The concrete,
single-instance fixes are already dispatched as their own PRs (see *In-flight fixes* below);
this stream owns the **systemic remedies** — the invariants, gates, and loops that stop the
*class* of defect from recurring, not the one instance that surfaced it.

**Scoping source:** the intake-desk triage of the open assay-toolkit queue (2026-07-24).
Every brief cites its source issue(s); a scoping brief does **not** close its finding — the
finding closes when the brief's *work* lands, so the issues are referenced, never `Closes`d,
by the PR that adds this stream.

**Cross-repo note.** Several deliverables land outside this repo: the desk loop **skills**
live in `oit` `.claude/skills/` (SKILL.md files); some desk
tooling (`deskboard.go`, `deskpost`, the PR monitor) and the durable-watchdog **service**
(oit `#627/#651`, `docs/streams/observability/`) live in oit / a desk-tooling home. Those
briefs carry a cross-repo pairing (sibling draft PR + SHA) at execution time; tracking stays
here (this repo is the open methodology source).

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Three-state instrument invariant + fleet audit + mutation-test Verify rule](./brief-01-three-state-instrument-invariant.md) | 0 | L | todo | — | — |
| 02 | [Fanout dispatch hygiene — claim-at-dispatch, branch-from-fresh-main, foreign-commit self-check](./brief-02-fanout-dispatch-hygiene.md) | 0 | M | implemented | — | — |
| 03 | [Source↔render drift + stale-normative-doc detection](./brief-03-source-render-drift.md) | 1 | M | todo | — | — |
| 04 | [Disclosure & audience controls — leak + candour scan for public artifacts](./brief-04-disclosure-audience-controls.md) | 1 | M | todo | — | — |
| 05 | [Merge-time re-check + cite-by-expression + body/Verify re-derive](./brief-05-merge-time-recheck.md) | 1 | M | todo | — | — |
| 06 | [Durable desk watchdog + autonomous drive + file-at-discovery](./brief-06-durable-watchdog-autonomous-drive.md) ([contract](./brief-06-contract-durable-watchdog.md)) | 0 | L | implemented | — | — |
| 07 | [Register-drain loop + ID hygiene (findings drain, resolved-vocabulary, sequential-ID collisions)](./brief-07-register-drain-id-hygiene.md) | 0 | M | implemented | — | — |
| 08 | [Unreviewed desk authority — evidence-not-verdict dispatch + verify-before-apply](./brief-08-desk-authority-verify-before-apply.md) | 0 | M | implemented | — | — |
| 09 | [Coverage & attribution — port bugs/ carrier + expected-visibility drift check](./brief-09-coverage-attribution.md) | 0 | M | todo | — | — |
| 10 | [deskadvisory — recompute-at-the-gate verification for security-advisory fixes](./brief-10-deskadvisory.md) | 0 | M | implemented | — | — |

Status legend: `todo` (unclaimed) · `in-progress` · `implemented` (implementer stops here) ·
`verified` (a non-implementer ran the Verify table + filled Evidence) · `done` (+ recorded
review). All rows start `todo`.

## Critical path

```
                 [EXTERNAL HEAD — Brief 06's durable watchdog:
                  the always-on service is oit #627/#651 (observability/), NOT in this stream]
                 [EXTERNAL GATE — Briefs 06/08 skill-filing identity: oit #556 (App issues:write) / #120]
                          |
  Brief 01  (three-state invariant + Verify-table rule)  ─►  03  (render-drift)
   the dominant defect class — the one real in-stream    ─►  04  (leak/audience)
   ordering: land the invariant so new instruments are   ─►  05  (merge-gate)
   born three-state-compliant
```

**Smallest unblocking move:** land **Brief 01** — the three-state invariant + the
mutation-test/neighbour Verify-row rule in `docs/brief-rules.md`. It is self-contained (a
methodology + audit change; nothing upstream blocks it), and it is what 03/04/05 must comply
with, so authoring it first stops three new instruments (render-drift check, leak scanner,
merge-gate) from each inventing their own two-state phrasing.

**Head verification (per author-brief rule 6).** Brief 01 is the genuine head: it depends on
nothing and is the class every other brief is an instance of. The graph is deliberately
**wide and shallow** — most of these are independent process fixes that parallelise; there is
no deep chain to fake. The **tempting-but-wrong** first step is Brief 06's *durable watchdog*:
its true head is the observability **service** in another repo (oit `#627/#651`). Starting
there stalls behind an external build. Brief 06's *desk-side* deliverables (the "no idle claim
without a fresh sweep" gate, the autonomous-drive skill change, the heartbeat contract) can
proceed independently, but the durable service itself must not be assumed to exist.

## Dependency waves

```
Wave 0: [01, 02, 06, 07, 08, 09]
Wave 1: [03, 04, 05] ← 01
```

Longest chain: **01 → {03, 04, 05}** (length 2). Wave-0 briefs are mutually independent and
individually dispatchable. Cross-references that are *not* build dependencies (so not encoded
in `depends:`): Brief 02's ID-collision hazard is remedied by Brief 07 (§ID hygiene); Briefs
06 and 08 both change desk loop skills and both want oit `#556`'s filing identity.

## In-flight fixes (single-instance PRs — do NOT re-scope here)

These issues are the *instances* that surfaced the classes; their concrete fixes are dispatched
as their own PRs. This stream scopes the systemic remedy only, and references them:
`#25`, `#34` (three-state instances → Brief 01), `#49` (evidence-not-verdict concrete fix →
Brief 08 scopes the remainder), `#51`, `#53`, `#59`, `#73`, `#80`, `#84`, `#113`, `#115`,
`#119`.

## Blocked on human decision (NOT authored as briefs — human:<name>'s call)

These are custody / authority / policy decisions, not implementation work. One-line pointer
each; they are the *reason* several briefs stop where they do (a control can detect and
document, but cannot decide the boundary).

> **The table is a register of decisions, not a live open-issue list — check the issue, not
> this row.** Several entries have since been ruled and closed (`#44`, `#47` and `#129` on
> 2026-08-02; `#273` the same day) and are kept here because the *ruling* is the durable
> record. A row's presence does not mean its issue is open, and a ruling recorded here does
> not mean the ruled mechanism exists — where it does not, the row names the briefs that
> track building it.

| Issue | Decision needed |
|-------|-----------------|
| `#37` | Reviewer-App gate is forgeable — a worker can mint the App token and self-approve (reconciler#13: approve→flip→merge in 14s). Whether/how to bind the merge gate. |
| `#44` | Key custody: role separation is a naming convention, not a boundary. **RULED 2026-08-02 (human:<name>): sandboxed execution** — Desk Console or docker/pods, not one-OS-user-per-role; the shared-memory sub-question is moot. Mechanism = `desk-console/04`; limits documented in enforcement-model.md. **Issue CLOSED 2026-08-02** on the ruling; the sandbox itself does not exist — the work is tracked by `desk-console/03,04,06,07` (all `todo`), not by this row. |
| `#45` | Agents issue rulings *in human:<name>'s name* from the shared `the-org` account and reviewers defer. Authority-from-source policy. |
| `#47` | The merge gate is advisory — branch protection / rulesets are plan-locked on private repos (403, confirmed 2026-08-02). **RULED 2026-08-02 (human:<name>): document the limits + move to sandboxed execution** — docs corrected, see enforcement-model.md. The plan/visibility call that would unblock `desk-apps/08` remains human:<name>'s. |
| `#26` | Bot-vs-human comment detection is structurally impossible under one shared GitHub identity. Identity-model decision. |
| `#38` | Make "desk posts as the App" permanent in the skill files + retire the "tamper-evident" claim. Skill-custody + wording ruling. |
| `#120` | Worker App lacks `workflows` permission — `.github/workflows/` fixes fall back to the-org. Permission-grant decision. |
| `#129` | intake-desk ↔ coordinator-desk double-respond on inbound (claims cover dispatch, not routing). Which desk owns routing. |

Note `#38`/`#45`/`#37` interlock with Brief 08 (desk authority) and Brief 06 (skill drive):
those briefs build the *mechanical* controls; the *authority model* they operate under is
these decisions.

## Shared conventions

- **Three-state everywhere (Brief 01, the spine of this stream):** no instrument reports a
  negative or a pass unless it can demonstrate it looked — *checked-clean / checked-failed /
  could-not-check*, never two states. Every brief that adds a check inherits this.
- **Verify against the thing the artifact represents, not a convenient proxy** (Brief 05); and
  **prove the instrument can see the defect on a positive control before trusting a clean
  report** (Brief 01).
- Deliverable-target notation: `[toolkit]` = this repo · `[oit]` =
  oit (incl. `.claude/skills/` and desk tooling) · `[oit-obs]` = the oit
  observability service · `[repos]` = the desk-worked repo set. Cross-repo deliverables ride a
  sibling draft PR referenced from the tracking PR (per the cross-repo-pairing rule).
