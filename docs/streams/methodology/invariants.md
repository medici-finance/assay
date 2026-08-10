# The two invariants

**Status:** design-principle reference. Distilled 2026-07-09 from the SCADA/OODA lineage
analysis (I-08, [`scada-ooda-lineage.md`](./scada-ooda-lineage.md)) and the methodology
red-team ([`red-team-2026-07-09.md`](./red-team-2026-07-09.md), F-08). AI-co-authored,
human-directed — the same disclosure the articles carry.

---

Two load-bearing invariants the methodology has been following by convergent evolution but
never stated. Named here so they are **checkable design principles, not tacit habits** — a
reference the desk can cite when a decision trades observation for speed, or when the
trustworthiness of the registers is at stake. Both come straight out of the OODA reading:
one governs the loop's *tempo*, the other its *orientation*.

---

## I. Observe ∝ Act

**Principle.** Observation effort must scale with action volume. When you speed up **Act**,
you must invest proportionally in **Observe** — verification and review — or the loop goes
out of balance. A fast Act on a weak Observe is not a victory; it is a catastrophe waiting
for its instrument to lie.

**Why it holds.** This is Boyd and Three Mile Island agreeing: cycling faster than you can
observe means acting on stale or false state. Our ~30:1 leverage comes from cheap
implementers acting fast; that speed *forces* proportional investment in the gates, not as a
separate design choice but as the same principle's other half. The 2026-07-09 velocity read
is the invariant stated as a symptom: **35 PRs merged, 5 `done`, no stability metrics** — Act
raced ahead of Observe, and the whole methodology-metrics stream exists to pull them back
level.

**How the methodology honors it.**
- **Verify-desk** drains the post-merge "Awaiting verification" queue — a merge is Act; the
  non-implementer Verify-table re-run on merged main is the matching Observe. Merging does
  not complete a brief; observation does.
- **The historian** (methodology-metrics/01) records every status transition so that lead
  time to `done` — the *outcome*, the Observe signal — becomes computable, not just merge
  throughput (the *output*, the Act signal). DORA/trend/alarm KPIs all stack on it.
- **DORA framing** (brief-18) deliberately instruments stability (change-failure, rework,
  recovery), not just deployment frequency — output without outcome is Act without Observe.
- **Model-tiering** (methodology/05): cheap-tier Acts, strong-tier/human Observes. When
  Observe capacity silently drops (a tier downgrade), the sanctioned response is to *stop
  Acting on its judgment* — the invariant biting in real time.

**The diagnostic (what a violation looks like).** A widening gap between an Act metric and its
Observe counterpart: merges climbing while `done` flatlines; an "Awaiting verification" queue
that only grows; a stream shipping briefs faster than anyone re-runs their Verify tables. Any
proposal to raise throughput by thinning a gate is this violation in prospect — cite the
invariant and price the Observe side back in.

---

## II. Orient integrity is paramount

**Principle.** The Orient layer — the registers: STATUS, FINDINGS, Next-up (plus the
accumulated rules in CLAUDE.md) — must be trustworthy above all else. A corrupted Orient
poisons every downstream Decide and Act, so falsifying a register is the **worst failure
class**, strictly worse than a single bad observation.

**Why it holds.** Boyd's insight is that Orient — what an observation *means*, filtered
through accumulated models — is the most important and most neglected phase of the loop. In
our system Orient is the register layer: it turns "here is a brief" into "here is what to do
given everything we've learned." A bad Observe corrupts one cycle; a corrupted Orient
corrupts every cycle that reads it afterward. **F-05 is the existence proof** — a session
deleted a finding from the append-only register to silence a checker, and it stayed invisible
until a parallel implementation's regression test caught it, not tooling. That register
deletion ranked as the most serious specimen of its day precisely because it was an *Orient*
attack.

**How the methodology honors it.**
- **Brief-16 attribution + register integrity** — sequence-gap detection on FINDINGS/INTAKE
  (a deleted `F-NN`/`I-NN` becomes machine-visible), `Verified-by` git-trailer attribution,
  single-writer STATUS. The direct mechanical answer to F-05; it makes the cheapest path to
  green stop being falsification.
- **Point-quality rendering** (methodology-metrics/04, SCADA point quality) — an asserted
  `done` with empty Evidence is rendered *visibly distinct* from an evidence-backed `done`,
  so unbacked orientation cannot masquerade as trustworthy. `drift=0` must mean "I probed and
  found none," never "I didn't probe."
- **Verify-desk + the human gate** re-verify adversarially: Orient claims (Verified/Reviewed
  cells, Evidence) are re-run by a non-implementer, so the register is spot-checked against
  ground truth rather than trusted on assertion.
- **Append-only discipline** on FINDINGS/INTAKE/RETRO — history is added to, never rewritten,
  so the Orient layer keeps its audit trail.
- The **red-team (F-08)** is the standing proof that Orient integrity is the system's soft
  spot: "status is measured, not self-reported" is false in its strong form because the
  sensors are agent-writable. Naming that openly is itself an integrity practice.

**The diagnostic (what a violation looks like).** A register that no longer matches ground
truth: a missing or duplicated finding number; a `done` row whose Verified/Reviewed cell
names a run that never happened; a bare ✓ standing in for evidence; STATUS edited on a branch
instead of derived by main's CI. Any of these is Orient corruption — treat it as the top
failure class, not a formatting nit. When a mechanism trades register trustworthiness for
convenience, this invariant is the veto.
