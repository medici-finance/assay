# Mistake-proofing (poka-yoke) — the device discipline

**Status: normative.** This document gives the methodology a shared vocabulary and a set of
rules for its *devices* — the checks, gates, guards, and scaffolds that stand between an
error and a defect. It extends [`three-state-instrument-rule.md`](three-state-instrument-rule.md)
(which governs what a single instrument may report) with rules for where devices sit, what
kind they are, and how the fleet keeps them honest. The brief-authoring half binds authors
alongside [`brief-rules.md`](brief-rules.md).

The vocabulary is Shigeo Shingo's, from the Toyota Production System. It earns its place
here because the methodology already practices its core claims without naming them; naming
them makes the discipline teachable and the gaps visible.

## 1. Doctrine

**Errors are inevitable; defects are not.** A defect is an error that crossed a boundary
and reached downstream work. Quality work is therefore not "make fewer errors" — it is
"put a device at the boundary so the error cannot cross."

**Inspection has three strengths.** *Judgment inspection* sorts good from bad after the
fact (a reviewer reading a finished artifact); it accepts a defect rate by construction.
*Informative inspection* feeds the error back quickly (a failing CI check on a PR).
*Source inspection* checks that the conditions for correct work exist **before the work
runs**, and refuses to proceed — a schema that will not parse an illegal state, a
scaffolder that cannot emit a malformed document, a claim that cannot be acquired twice.
Prefer source over informative over judgment.

**Devices come in two modes.** A **control** makes the error impossible or halts the
line with no human decision (a required check that fails closed; a generated file whose
hand edit cannot merge). A **warning** alerts a human and relies on the response (a
NOTICE, a review comment, a convention in a document). Controls are preferred wherever
the condition is mechanically checkable; a warning is the right device only where
judgment is genuinely required.

**A third axis this fleet needs: who can bypass the device.** Classical poka-yoke assumes
an operator who wants to comply. An agent fleet must assume an operator that will route
around a client-side guard standing between it and its deliverable — not out of malice,
out of goal-completion. A device's real strength is set by the *weakest identity that can
bypass it*: an OS/harness-level refusal outranks a server-side required check, which
outranks a client-side tool refusal, which outranks a rule in prose. Classify honestly.

**The executor will not ask.** The empirical record on language-model executors is that
ambiguity in a specification is resolved *silently and divergently*, not surfaced as a
question — measured clarification rates in ambiguous coding tasks are low, and the same
model resolves the same ambiguity differently across runs. A "report missing context,
don't guess" ground rule is fighting a measured tendency and cannot be the primary
device. Specifications aimed at model executors need source-level devices, not appeals.

## 2. Classifying a device

Every check, gate, guard, or scaffold added to an Assay repo SHOULD be classifiable in one
sentence: **timing** (source / in-process / downstream) × **mode** (control / warning) ×
**bypass** (who can get past it, and how it would be noticed). Examples across the ladder:

| Device shape | Timing | Mode | Typical bypass |
|---|---|---|---|
| Scaffolder that cannot emit an invalid document | source | control | hand-author instead of scaffolding — visible in the diff |
| Schema/closed-value-set lint, required on PR | in-process | control | none short of changing the lint — visible |
| Generated single-writer file, CI-regenerated, hand edits refused | source | control | none — the regeneration overwrites |
| Atomic claim (create-if-absent ref) before dispatch | source | control | ignore the claim step — visible in audit |
| Fail-closed tool refusal (exit non-zero, no fallback) | in-process | control (client-side) | use a different tool — MUST be logged |
| Required CI check on a protected branch | in-process | control (server-side) | admin override — attributed |
| NOTICE emitted by a linter | in-process | warning | ignore it |
| Rule stated in a document | — | warning (weakest) | never read it |
| Post-hoc review of a finished artifact | downstream | warning | rubber-stamp |

The reading rule: strength grows toward the top-left. When a device proves weak, the first
move is to shift it **up-left** (earlier, and from warning to control), not to add a second
device in the same row.

## 3. Device rules (normative)

**D1 — A control must be shown to fire.** For every control-mode device there MUST exist a
demonstration that an injected instance of the error it claims to stop actually reddens
it — a mutation test, a positive control, a deliberate bad input in its test suite. A
control that has never fired is either unnecessary or broken, and without injection you
cannot tell which. "When did this last stop something?" is a legitimate audit question for
any device, and "never, and we cannot make it" is a finding. (This is the
[three-state instrument rule](three-state-instrument-rule.md)'s positive-control clause,
promoted from single instruments to every device in the system.)

**D2 — Fail closed; could-not-check is not a pass.** A device that cannot establish its
condition reports could-not-check and refuses, it does not wave through. (Restated from
the three-state instrument rule; listed here because it is the mode axis's load-bearing
half: a "control" that fails open under error is a warning wearing a control's label.)

**D3 — No silent bypass.** Every bypass path a device deliberately carries (an override
flag, an exemption file, an admin merge) MUST be logged, attributed to an identity, and
where possible expiring. Bypass normalization — each use making the next easier — is the
documented death of mistake-proofing systems, and the countermeasure is structural
(attribution and expiry), never exhortative ("use sparingly").

**D4 — Warnings do not compose.** Every additional warning weakens the ones before it;
alarm-fatigue literature puts sustained false-alarm rates at levels that produce
desensitization, not vigilance. Budget warnings. When a warning is routinely ignored,
either promote it to a control or delete it — a standing ignored warning is negative
value.

**D5 — Retire dead devices.** Devices accumulate; nothing retires them naturally. A device
whose error class no longer exists, or whose D1 demonstration cannot be produced, is a
removal candidate. An inventory that only grows is itself a smell.

**D6 — Honesty about non-coverage is a device.** A maintained, visible list of what the
devices do NOT check (known divergences, inert rules, fail-open paths) guards against the
worst failure mode: a green lamp nobody knows never looked. Keep such lists close to the
checks they describe, and prefer generating them from the checks themselves.

**D7 — Do not proof judgment.** Adequacy, taste, and materiality stay with review. The
honest split for a judgment-adjacent surface is: the *presence* of the judgment artifact is
checkable (a control), its *adequacy* is not (review). State which half a device covers.
Automating a judgment call is how Goodhart enters: a device whose metric can be satisfied
without the quality it proxies will be.

## 4. Brief authoring — mistake-proofing the specification itself

Briefs are specifications executed largely by model workers (§1: the executor will not
ask). The Verify table is the brief's strongest device; these rules extend the discipline
to the authoring act itself. They bind alongside [`brief-rules.md`](brief-rules.md).

**B1 — Scaffold, don't type.** The brief front door SHOULD be a generator, not a blank
file: every required key emitted (empty values still carry their keys), `gate` derived by
prompting the risk questions rather than accepting a hand-written answer, `wave` derived
from `depends`, the inverse `unblocks:` edge written into the named briefs in the same
change (making graph consistency structural rather than checked), and any freshness stamp
produced by a fetch the tool itself performs. Every field a generator derives is an
authoring mistake that stops existing.

**B2 — Verify rows carry their obligation class.** Where the format supports a row class
marker, the prose MUSTs of brief authoring ("a brief adding a check needs a mutation row";
"a change to a shared surface needs a flow row"; "a deliverable making factual claims
needs a dereference row") SHOULD be carried as typed row classes whose *presence* the lint
derives from the shape of the change and enforces. Presence is the control; adequacy stays
review (D7).

**B3 — Risk answers are cross-read against declared paths.** Self-declared risk booleans
whose `files:` contradict them — declared paths that trip the same risk classification the
review lane computes, while every answer says no — are a lint PROBLEM, not a reviewer
catch. This is the one authoring mistake that silently *downgrades a gate*, which makes it
the first to proof.

**B4 — Named identifiers dereference.** A test name, function name, file path, or link
named in a brief or its evidence MUST resolve against the tree it describes. Presence
checks on claims are judgment inspection; dereference is source inspection on the claim
itself.

**B5 — Pre-mortem, mapped to detection.** At authoring time, spend one prompt on
prospective hindsight: "this work shipped and was wrong — what went wrong?" Each named
failure mode is mapped to the Verify row that would catch it. A failure mode with no row
is the finding — either add the row or record why it is review-only. (This is the useful
core of FMEA — the detection column — without its discredited risk-priority arithmetic.)

**On a RISK-GATED brief this pre-mortem is REQUIRED and RECORDED, not optional** (sdlc/05,
[`../spec/brief-v1.md`](../spec/brief-v1.md) §4.7). A brief whose `gate` is `human`, or any
of whose four `risk` answers is `yes`, MUST carry the pre-mortem as its recorded threat
model: the named failure modes, each mapped to the Verify row that catches it, with an
explicit "no row; review-only" line for any mode no row covers. This is B5 made mandatory
for the briefs where a wrong design costs most — it is NOT a second pre-mortem concept, and
a design must plug the threat model into its existing single-point-of-failure note (the one
control the design leans on and the layer(s) behind it) rather than stand up a parallel
ceremony. The recorded threat model is the evidence a reviewer reads when answering the
defense-in-depth question: does a lower layer catch the fault with the upper one bypassed?

**B6 — Negative control on the Verify table.** A Verify table SHOULD be able to fail: for
a table guarding a deliverable, ask what a *wrong-but-plausible* deliverable looks like
and confirm at least one row goes red on it. A table of presence-greps that passes on a
factually wrong artifact is the canonical authoring defect this rule exists for. This is
D1 applied to the brief's own instrument.

**B7 — A do-confirm checklist at the dispatch pause.** Keep one short checklist (single
digits of items) covering ONLY what the lint cannot read — "verify rows discriminate, not
just detect presence"; "facts dated and checkable"; "self-contained: executable without
reading another brief"; "risk answers match the paths". Run it at the moment of dispatch,
do-confirm style. Length discipline is the device: a checklist that grows becomes a
document, and documents are warnings (§2).

**B8 — Probe comprehension, don't assert it.** For a brief whose self-containedness is in
doubt, the author writes three to five questions a competent executor must be able to
answer, and a *fresh* model instance answers them from the brief alone before dispatch. A
wrong answer is a defect in the brief found before it crossed the boundary — source
inspection on the specification, and cheaper than one failed worker run.

**B9 — Authoring guidance is derived, never hand-copied.** Any statement in authoring
guidance about what is and is not enforced ("no lint checks this yet") MUST be generated
from the enforcement source, or carry a check that fails when it drifts. A guidance
document that tells authors a live gate is decorative manufactures deliberate
non-conformance; hand-maintained second copies of normative sources are the documented
error class here, and derivation is the closed fix.

**B10 — Facts carry their recheck.** Entries in a brief's `facts:` block are assumptions
wearing a fact label the moment authoring ends. Where a fact is mechanically checkable,
prefer recording it WITH its check (the command that re-establishes it) so staleness is
detectable, not archaeological.

## 5. Adoption ladder

For a repo adopting this discipline incrementally, the value-per-cost ordering observed in
practice:

1. **Vocabulary + D1.** Classify the devices you have; demand the fired-demonstration for
   each control. This alone surfaces the theater.
2. **B4 + B6.** Dereference named identifiers; negative-control the Verify tables. Both
   catch the highest-frequency specification defect (checks that cannot fail).
3. **B5 + B7.** Pre-mortem mapping and the dispatch checklist — pure process, no tooling.
4. **B3 + B2.** Lint-side cross-reads and row classes — small tool changes.
5. **B1 + B9.** The scaffolder front door and derived guidance — larger builds that
   remove whole error classes rather than checking them.

## 6. Sources

Shingo, *Zero Quality Control: Source Inspection and the Poka-Yoke System* (1986).
Grout &amp; Toussaint, "Mistake-proofing healthcare" (device criteria). Norman, *The Design
of Everyday Things* (forcing functions, interlocks). Saltzer &amp; Schroeder, "The
Protection of Information in Computer Systems" (fail-safe defaults). King, "Parse, don't
validate". Klein, "Performing a Project Premortem" (HBR 2007). Gawande, *The Checklist
Manifesto*; Haynes et al., NEJM 2009 (checklist outcomes). Parnas &amp; Weiss, "Active
Design Reviews" (1985). Femmer et al., "Rapid quality assurance with Requirements Smells"
(2017). AIAG-VDA FMEA handbook (Action Priority replacing RPN). Vaughan, *The Challenger
Launch Decision* (normalization of deviance). On model executors and ambiguity: Orchid
benchmark (arXiv:2604.21505); ClarifyGPT (arXiv:2310.10996) and successors — measured
silent-resolution behavior under ambiguous specifications.
