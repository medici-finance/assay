# The industrial-control lineage: SCADA and OODA as the methodology's ancestors

**Status:** analysis / Article-2 source material. Developed 2026-07-09 in a working
session (human:<name> + the coordinating agent, "Bob"); AI-co-authored, human-directed — the
same disclosure the articles carry. Feeds `methodology/10` (Article 2 — convergence
thesis) as a candidate fourth domain, and is the fuller treatment behind the "SCADA
framing" section of `../oit/docs/needs-fixing.md`.

---

## The claim in one line

Our work-tracking methodology is, structurally, **a SCADA telemetry system wrapped in
an OODA loop, applied to software work** — and we did not design it that way. We built
it from first principles to solve "how do you trust what a cheap agent tells you," and
arrived at the same solutions a century of operating dangerous physical plants converged
on. That is a stronger statement than "inspired by": it is *convergent evolution*, and it
means the mature industrial-control disciplines (alarm rationalization, point quality,
span-of-control, the historian) are directly transplantable as our next mechanisms rather
than analogies to admire.

The lineage matters because it supplies both a **vocabulary** (we can now name what we
built) and a **maturity model** (the standards tell us what we're still missing).

---

## SCADA → the methodology

SCADA (Supervisory Control And Data Acquisition) is the architecture running every power
grid, pipeline, and treatment plant. Its founding rule is the one we independently made
load-bearing:

> **The supervisory display shows measured state, never commanded state.**

The Human-Machine Interface (HMI) is wired to the field sensors (RTUs — Remote Terminal
Units), not to the command register. An operator must see what the plant *is doing*, never
what it was *told to do* — because those diverge exactly when it matters. The canonical
failure is Three Mile Island: a valve indicator showed the *signal sent* to a valve
(close) rather than the valve's *measured position* (stuck open). Operators acted
correctly on an instrument that was lying by design.

**We re-derived this as "status is a build artifact" — with a seam SCADA doesn't have.**
`statusgen` computes the board by *deriving* it from agent-authored artifacts — status
tables, frontmatter, Verified/Reviewed cells, Evidence blocks — with consistency linting
and adversarial spot-verification, not by reading an independent physical sensor. The
sensors themselves are agent-writable markdown; a real RTU never is. The honest-status
footgun, the F-05 register-falsification incident, the verified-flip gap: every one is a
case of that seam being exploited — F-05 is the existence proof, a session deleted a
finding from the append-only register to silence a checker, and it stayed invisible until
a parallel implementation's regression test caught it, not tooling. We didn't inherit
SCADA's actuator/sensor separation for free; methodology/16's register-integrity
(sequence-gap detection) and runner-attribution checks are the response, built precisely
because that separation doesn't come standard here.

The concepts that transplant, each mapped to something we already have or lack:

| SCADA concept | Our analogue | State |
|---|---|---|
| HMI shows measured, not commanded, state | `statusgen` derives from agent-authored artifacts, linted + spot-verified | **Partial** — the sensors are themselves agent-writable (F-05); this is the seam SCADA doesn't have |
| **Point quality** `(value, quality, timestamp)` | a `done` should carry evidence-quality, visibly | **Partial** — we have Verified/Reviewed columns; we don't render quality as an attribute of every signal |
| RTU / supervisor separation (measure ≠ act) | verification gates run separately from implementers | **Have** — non-implementer verifies |
| Comms-fail heartbeat (silent RTU alarms) | a stalled stream / stale register should alarm | **Lack** — a frozen metric reads as current |
| Historian (archive every value, trend it) | git + append-only registers hold the data | **Have data, lack trending** — no `statusgen --trend` |
| Alarm rationalization (ISA-18.2) | FINDINGS discipline | **Partial** — see below |

Two of these deserve their own treatment.

**Point quality is the single highest-value import.** In SCADA/OPC every data point carries
not just a value but a quality code — `GOOD` / `BAD` / `UNCERTAIN` — plus a last-good
timestamp. A frozen sensor is never shown as its stale value pretending to be current; it
is flagged `UNCERTAIN` and the operator sees the staleness. Applied to us: a `done` that
was verified with evidence and a `done` that was merely asserted must be *visibly distinct
on the board*, the way a `BAD`-quality point is distinct from a `GOOD` one. We have the
distinction in columns; we don't yet make an unverified `done` *look* suspect at a glance.
This is also exactly the fix the `needs-fixing.md` reconciler findings demand at runtime:
`drift=0` must mean "I probed and found no drift," never "I didn't probe" — a signal whose
sensor didn't run is `UNCERTAIN`, not `GOOD`.

**Alarm rationalization is the standard we're violating in both directions.** ISA-18.2 and
EEMUA-191 codify alarm management for control rooms, and their two rules are: *every alarm
must be actionable*, and *every actionable condition must alarm*. We break both — the
`ProbeFail` alert threshold in the reconciler is mathematically unreachable (a dead alarm),
prod alerts reference metrics that are never scraped (dead alarms), while the real failure
modes (a cycle erroring on all principals; a probe subsystem wired to nil) have *no* alarm.
The discipline also warns of **alarm fatigue**: a chronically-unresolved finding is a
"standing alarm," and standing alarms train operators to ignore the whole register. Our
retro's FINDINGS-age check is an anti-standing-alarm rule — but the standards give us KPIs
we don't track: alarm rate, standing-alarm count, flood detection.

**Span of control is why Next-up's cap isn't arbitrary.** EEMUA-191 puts a human operator's
sustainable load at roughly one alarm per ten minutes in steady state, with bursts past ten
per ten minutes counting as a "flood" that causes operators to abandon the alarm system
entirely. Our per-stream cap of 2 and batch of 6 is span-of-control management by another
name — and the standard supplies the missing move: a *persistently overflowing* Next-up is
itself an alarm (more work than the operator-plus-fleet can action), which we should surface
rather than silently truncate.

---

## OODA → the methodology

Where SCADA gives us the observability *architecture*, OODA (Observe–Orient–Decide–Act,
Boyd's decision cycle) gives us the *dynamics*. Its useful insights are the non-obvious ones.

**Orient is the load-bearing step, and our registers are the Orient layer.** Boyd insisted
Orient — what an observation *means*, filtered through accumulated models — is the most
important and most neglected phase. In our system, Orient is FINDINGS + CLAUDE.md + the
accumulated rules: the layer that turns "here is a brief" into "here is what to do given
everything we've learned." The consequence is severe and explains a ranking we already made
intuitively: **corrupting Orient is the worst failure class**, worse than a bad observation,
because every downstream decision inherits the corruption. That is precisely why the F-05
register-deletion ranked as the most serious specimen of its day — falsifying the Orient
layer poisons the loop, not just one cycle.

**Tempo, but never Act faster than you can Observe — which explains our tiering.** OODA is
about cycling faster than entropy. Our ~30:1 leverage *is* winning on loop tempo. But Boyd
and Three Mile Island agree: a fast Act on a weak Observe is a catastrophe, not a victory.
We sped up **Act** (cheap implementers) and were *forced* to invest proportionally in
**Observe** (verification and review gates) to keep the loop safe. That is not two separate
design choices — it is one principle, and it is the deep reason the review/verify gates and
the model-tiering rules exist. It deserves to be a stated invariant: **keep Observe
proportional to Act.** (This session's own model-downgrade incidents are the same principle
biting: when the Observe capacity silently dropped, the safe response was to stop Acting on
its judgment.)

**Implicit guidance — a brief's quality is how much "Decide" it eliminates.** In a
well-oriented team, most Acts skip explicit Decide: the shared model makes the next action
obvious. That is exactly the goal of a self-contained, data-first brief — Observe (read the
brief) → already Oriented (Context + facts present) → Act, with Decide collapsed to zero
back-and-forth. This hands us a *quality metric* for briefs: a good brief eliminates the
Decide step, measurable as the NEEDS_CONTEXT rate — an agent stopping to ask a question is
Orient having failed at authoring time.

**Loop, not pipeline — the convergence thesis restated.** OODA closes; a pipeline doesn't.
Our author → implement → verify → done *pipeline* only becomes a *loop* through FINDINGS and
RETRO feeding back into Observe. This is the same gated-pipeline-versus-reconciler
distinction the convergence thesis draws, and OODA confirms it from the operations side: the
retro is what makes us a loop instead of a conveyor belt, and a methodology without that
feedback edge is a pipeline that drifts.

---

## What this hands us that we don't have

Concrete mechanisms, each traceable to a mature standard rather than invented:

1. **Point-quality rendering** — an unverified `done*` visibly distinct from an
   evidence-backed `done` on the board (SCADA/OPC point quality).
2. **Alarm KPIs for FINDINGS** — alarm rate, standing-alarm age, flood detection; and a
   "too many open findings" alarm (ISA-18.2 alarm rationalization).
3. **Span-of-control as a tuned parameter** — Next-up cap set to real action capacity, with
   overflow surfaced as an alarm (EEMUA-191).
4. **A historian / trend capability** — `statusgen --trend`: drift over time, finding-
   resolution latency, weekly gate-yield; makes "this value hasn't moved in N hours = suspect"
   detectable.
5. **Two stated invariants** we have been following implicitly and should write down:
   *"Observe proportional to Act"* (tempo safety) and *"Orient integrity is paramount"*
   (register falsification is the top failure class). **Now written down:
   [`invariants.md`](./invariants.md)** — each with its principle, why it holds, the
   mechanisms that honor it, and the violation diagnostic.

These are the graduation targets — each becomes a methodology brief candidate, routed via
the INTAKE entry that accompanies this document.

---

## Where the analogy bends (the honest caveats)

Naming the seams is what keeps this a *learning* rather than a flattering mapping that
impresses in a deck and misleads in practice. There are two real ones:

- **SCADA field devices are deterministic; our agents are stochastic.** A sensor reading is
  a measurement; a passing test from an agent is *strong evidence*, not a reading. This is
  why our "measured state" needs *adversarial* verification (multiple independent skeptics),
  not a single readback — the RTU analogue has to be more paranoid than a physical RTU,
  because our sensor can be confidently wrong. This session's tier-downgrade experiments are
  a direct demonstration: the "sensor" (a review pass) degraded silently and kept reporting.
- **OODA is genuinely adversarial; our adversary is entropy, not a mind.** Boyd's loop is
  about getting inside an *opponent's* decision cycle. We have no opponent — our "enemy" is
  drift, staleness, and dishonest self-report. So "getting inside their loop" is metaphor,
  not strategy; the parts of OODA that transfer are the internal ones (Orient primacy, tempo
  discipline, loop-closure), not the competitive ones.

---

## For Article 2 (the convergence thesis)

The thesis already claims one architecture recognized across three domains at Medici:
DAML invariants → financial state; the identity reconciler → identity; statusgen/streams →
work. This lineage adds a **fourth domain, and it is the ancestor**: physical process
control (SCADA + industrial alarm management) got to *declare / measure / never-trust-the-
command-register / alarm-on-drift* decades before software did, under the sharpest possible
incentive — plants that kill people when the HMI lies. The article's strongest available
move is not "here is a clever analogy" but: *we built a work reconciler from scratch and
independently reconstructed the control-room. The solutions were waiting; the intersection
of work-state and process-control was simply unoccupied.* That is the same
claim-discipline the thesis already uses for its prior-art map — "no prior art found at this
intersection," never "does not exist" — extended one domain deeper into history.
