---
id: DR-graph-exec-35
date: "2026-11-01"
title: "A run-time signal gate releases the activation hold: each activating brief carries its own human gate, bound inputs and a liveness source"
consequence: major
decided-by: "human:ian"
ruling: "https://github.com/medici-finance/assay/issues/5147#issuecomment-6560646821"
alternatives:
  - "Release the hold for the gate-red class only, keeping outage and regression held until a further ruling: option B on the decision issue, which stated its cost as another decision when another class is ready. The ruling chose A."
  - "Keep the hold after the check lands, with activation needing a separate ruling later: option C on the decision issue, which stated that the spec would then describe a check it forbids anyone to use. The ruling chose A."
accepted:
  - "The gate can't enforce the culprit-class binding duty. It can't tell a bound value from an unbound one, and an exact `mechanical` selects the mechanical exit whatever produced it. The activating brief's own human gate and review have to catch a caller that breaks the duty."
  - "No brief supplies a culprit-class binding today (#5150). Until one does, its caller must pass none: a class with two exits then takes its judgement exit, and a run can't select the mechanical exit."
  - "The gate trusts its caller for every input: trigger, ref, liveness, class, culprit class, node, root, pattern and expected digest. A caller can also skip it."
  - "No brief owns a liveness result for any source (#5148). Until one does, the gate holds every run of a caller that follows the spec. The gate can't tell an observed result from a constant `live`, and it can't tell a fresh result from one reused from an earlier node."
  - "At run time the gate enforces only the faults brief 35 lists. For the other lint rules, the lint and review are the only layers."
  - "Effect limits stay with the activating brief, and only auto-triage/02 and /03 own them, for gate-red."
---

**Ruling recorded (2026-11-01): option A.** The driver (`human:ian`) ruled on the brief's
decision-gate issue under the driver's own login. The ruling comment:
https://github.com/medici-finance/assay/issues/5147#issuecomment-6560646821
It is dated 2026-11-01T16:25:49Z and reads `graph-execution/35: A — DR-graph-exec-35`. This
record transcribes the design that issue put for approval. It does not mint a new decision: the
human act is the driver's comment, not this file.

**The decision.** `docs/streams/graph-execution/brief-35-signal-pattern-runtime.md`: a stateless
`deskdispatch --signal-gate`, called before each node, with verdicts admit (exit 0), refuse (5)
and hold (6). Exit 0 is the only proceed. Task 10 replaces graph-execution/34's rule that a signal
pattern is a reviewed definition only and MUST NOT be activated.

Under option A the hold is released once the check is implemented, for any brief that:

- carries its own human gate;
- binds every caller input to a value the run can't choose;
- passes a culprit class only once it supplies that binding;
- names a liveness source that is observed again for each node.

**The gate.** It refuses a run whose trigger or ref fails its class's admission, a node off its
class's route and a pattern file whose sha256 differs from the one its caller expects. It holds
when the source is dead or unchecked.

**Not active.** The brief activates nothing and no caller exists. Each later activation gets its
own ruling.

**Where the wording is looser or narrower than this record.**

- The fork-test line on #5147 says "bound inputs and a named liveness source". The option text
  governs, and it also states the culprit-class duty and the observation for each node.
- The Needed section of #5150 still offers one brief for all activating briefs. Option A and
  Task 10 put the duty on the activating brief, and this record decides nothing further about who
  writes a binding.
- auto-triage/02 and /03 state the duty in their contract notes, with no Task step or Verify row
  of their own. Liveness holds them too, so neither can go live before that is settled.

**Consequence: major.** If the design is wrong, a later activating brief could rely on a gate that
doesn't cover what it assumed, and a signal pattern could go live on that basis. The gate has no
caller until such a brief lands, and that brief needs its own human gate and ruling.
