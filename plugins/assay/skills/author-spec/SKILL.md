---
name: author-spec
description: >-
  Create, adopt, or revise a specification or scoping document before decomposing work
  into briefs. Use for requirements, system design, architecture proposals, and reviewing
  an existing spec for decision readiness. Use author-brief for execution decomposition.
---

# Author Spec

Produce a sufficient, evidence-backed source of intent that downstream briefs can cite.
Specify outcomes, boundaries and consequential decisions; leave execution scheduling to
`author-brief`. This skill chooses no product architecture, taxonomy or technology for an
adopter. Read applicable project guidance for preferred techniques and actual constraints;
an absent local profile is valid. Do not turn a preference into a binding constraint.

## Start from the controlling source

Find the existing spec, relevant decisions, affected consumers and current implementation.
Choose **create** only when no sufficient source exists, **adopt** when an existing artifact
already settles the needed questions, or **revise** for a bounded change to that source.
Keep adequate formats and content. Adoption may need only citations and a short gap note;
do not manufacture a parallel spec or reformat an approved one to satisfy this skill.

Pin material source revisions or dates. Distinguish observed behaviour, approved intent,
proposals, assumptions and unresolved decisions. Inspect relevant code/tests for brownfield
work; a completed brief or incident report alone is not evidence of current behaviour.
Unavailable evidence stays explicit. Do not substitute absence of access for absence of a
constraint. For a revision, preserve completed implementation and Evidence; identify which
unfinished work and consumers the delta affects.

## Resolve the consequential questions

Scale depth to uncertainty and impact: a small change may take a paragraph and examples;
an architectural change needs the boundaries below; exploration may end in a bounded spike
with an unanswered question and discriminating observation. These are questions to answer
where relevant, not required headings or a section-count gate.

- **Outcome and scope.** Whose problem is supported by what evidence? Name the workflow,
  success observable, exclusions and domain terms whose ambiguity changes behaviour.
- **Existing obligations.** Identify inherited decisions and the semantic owner of each
  affected meaning. Link the canonical contract or record the gap. A theme, plane,
  substrate, directory or stream label does not confer ownership or authority. Surface a
  conflict with a controlling decision; do not silently override it in a local design.
- **Behaviour and boundaries.** Describe inputs, outputs, states and relevant failure,
  recovery and partial-success paths. Identify producers and consumers across seams, the
  authority to act, and capabilities that must not cross a boundary, including transitively.
  Name reusable library/API entrypoints when they exist. Justify an additional process by
  a concrete isolation, custody, deployment or lifecycle need; do not assume every logical
  layer needs a service or that sharing code transfers authority to its callers.
  Check proposed reuse from the consumer's actual module/package and version boundary;
  code existing somewhere is not proof it is importable or compatible. If extraction is
  needed, identify its owner and the prerequisite before promising reuse.
- **Design and alternatives.** Separate required outcomes from the proposed mechanism and
  incidental implementation choices. Record consequential alternatives and tradeoffs.
  For a recurring defect, identify the hypothesised mechanism and owning layer; consider
  removing the hazard or reusing its owner before adding another guard or exception.
  State what the proposal retires and what added complexity remains justified. Apply the
  project's existing design-fit procedure rather than creating another register or gate.
- **Change and compatibility.** Explain migration, version compatibility, affected callers,
  rollback or recovery, and retirement of the previous path where applicable. Distinguish
  the desired end state from a transition that intentionally carries two implementations.
- **Proof and decisions.** Give observable acceptance examples, including a relevant
  failure or boundary case, and the evidence that could falsify the design. State what can
  be checked now versus after implementation. Separate facts from hypotheses; do not claim
  a metric or retrospective comparison proves causation. Name remaining decisions and
  their authorized owner, with options and a recommendation. Continue independent work
  while a decision is pending; do not silently choose on that owner's behalf.

Keep the minimum durable rationale needed to prevent independent implementers choosing
incompatible meanings. Reuse existing decision records, contracts and evidence references.
Use [review prompts](references/review-prompts.md) when a brownfield redesign or a
spec-quality review needs a deeper check; do not load it for every small edit.

## Review and hand off

Check the proposal against its actual sources and a plausible counterexample. Report
material gaps with the source and consequence; missing preferred formatting is not a
defect. An adequate existing spec is a legitimate no-change result. For redesigns, separate
wrong or changed intent from code that drifted or implemented correct intent incorrectly.
Do not rewrite correct intent to compensate for an implementation bug.

Follow the existing [spec lifecycle](https://github.com/medici-finance/assay/blob/main/spec/lifecycle-v1.md#8-spec-and-scoping-doc-lifecycle)
for Assay-tracked specs: draft, approved, routed; the ruling and citing briefs establish
the transitions. A review by this skill grants no approval or implementation authority.
When authorized to decompose, use `author-brief`, whose
[Context contract](https://github.com/medici-finance/assay/blob/main/spec/brief-v1.md#4-body-structure)
owns `design-fit`, layering and architectural-contract references. Carry the controlling
source and unsettled obligations into briefs; do not duplicate that schema here or invent
a second task/status system. If this task is only spec authoring, deliver the spec, its
material open decisions and the next authorized step without auto-starting implementation.
