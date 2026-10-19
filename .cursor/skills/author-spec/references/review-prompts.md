<!-- GENERATED from plugins/assay/skills/author-spec/references/review-prompts.md by deskinstall --harness cursor. Do not edit. -->
# Deeper spec review

Use only the prompts relevant to the proposed change. These supplement judgement, not a
new approval gate, document template or universal checklist.

## Brownfield design

Read the originating intent alongside the current public surface and relevant failures.
Could the same evidence mean changed requirements, incomplete migration, or an implementation
bug? State the next observation that separates those explanations. For every consequential
claim, retain a source reference that another reader can inspect.

Identify which behaviours must survive, which are known accidents, and which are unsettled.
For an actual redesign, use the project's refactor-oracle procedure if one exists. Its
characterization and regression tests belong to implementation/verification; a prose spec
review cannot claim to have run them. A spec can identify the preservation obligations now.

At a seam, ask whether two consumers could compute the same meaning differently, whether
sharing an API accidentally gives a caller a privileged capability, and whether a proposed
service adds a necessary boundary or merely wraps a reusable library. Read the real caller
and owner contracts before deciding. A single semantic owner need not mean a single process.

## Requirement coverage and contradictions

For each consequential obligation in scope, identify the conditions that change its
meaning: relevant actors, triggers or states, limits, refusal and recovery paths, and
ordering. Map those conditions to observable acceptance criteria in the spec's existing
format. Report an uncovered condition with its consequence; merely repeating the obligation
is not evidence of coverage. A small change may need only a sentence and one boundary case.

Split obligations or acceptance outcomes when one can pass while another fails. A coherent
output shape can remain one criterion; conjunction counts, sentence length and punctuation
are review hints, not proof of multiple obligations. Keep related criteria together and
retain cross-cutting invariants rather than fragmenting every sentence into a new record.

Check the reverse direction too: a criterion or example that adds an expiry period,
permission exception, retry limit or other material behaviour needs a governing obligation
or controlling decision. Resolve the missing intent or move the example; do not silently
promote an illustrative value into an approved requirement. Keep required outcomes distinct
from proposed mechanisms while retaining sourced technology and compatibility constraints.

Compare the affected obligations with one another, their criteria, inherited constraints
and accepted decisions. Look for incompatible responses under overlapping conditions,
numeric boundary disagreements, and exceptions that contradict a broader promise. Cite the
conflicting passages and the observable consequence. Report contradictions before coverage
gaps, then decomposition or wording issues; an adequate spec may need no change.

## Revisions and evidence applicability

A consequential edit needs a reading of the whole affected obligation and its criteria,
including unchanged text, linked decisions and consumers. Identify what the edit adds,
changes or supersedes; check that earlier examples and obligations still agree. Record
which downstream verification claims need reassessment when their governing intent changes,
even if their commands or test names did not change. Preserve historical Evidence and its
original subject; do not relabel an old result as proof of the revised requirement.

Distinguish three questions: does a check map to the intended criterion, did it execute
against the relevant revision and conditions, and does its result support acceptance?
A test-name link or matching content hash establishes neither execution nor semantic
adequacy. Use existing project evidence and review procedures; these prompts introduce no
new identifier scheme, freshness gate, automatic demotion or authority to approve intent.
An implementation defect does not justify weakening the governing requirement.

## Cost and quality at design time

Consider performance, resource use, reliability, security, operability and maintainability
where they affect the outcome. Give measurable expectations and their source or proposed
status; do not invent budgets to fill a template.

For a consequential performance claim, ask what happens when each independent input grows:
records, rules, bytes, callers or requests, and invocation frequency. Consider algorithmic
growth alongside repeated parsing, normalization, process startup and remote calls; a
favourable complexity class alone does not establish an acceptable cost. For example, a batch
matcher may repeat preparation and expensive matching for every rule over the same corpus.
Compare shared preparation and candidate filtering with the simple baseline before selecting
an index; account for construction cost, peak memory and rules that cannot be filtered.
Any filter must preserve required matches and failure behaviour.

Could the proposed benchmark distinguish the alternatives at the expected workload, including
an unfavourable input shape? Record the environment, corpus dimensions, acceptance criterion
and correctness oracle; measure setup and end-to-end cost as well as the suspected hot path.
A paper cost model identifies a risk, not a measured speedup. If a simple implementation
already meets supported requirements with adequate headroom, retain it unless evidence
justifies added complexity. A small change outside a sensitive path needs no benchmark ritual.

## Test the method as well as the proposal

For a process or methodology change, record the expected benefit, the burden it adds, and
an observable that would warrant revising or stopping the experiment. Prefer a bounded
trial to a universal rule inferred from one failure. Leave existing gates and thresholds
under their current authority.

For historical evaluation, pin the candidate method and source cutoff before scoring cases.
Separate available-at-cutoff evidence from later outcomes. Declare outcome knowledge and
selection bias; retrospective detection is not proof of prevention. Include already-covered
obligations and cases where the skill should recommend no change. Distinguish a missed
design obligation from a failure to implement or enforce an obligation already present.

Reuse applicable [build-less-brittle procedures](https://github.com/medici-finance/assay/blob/main/docs/streams/build-less-brittle/spec.md) for semantic ownership, retirement, evidence grading and investigations.

## Source note

The coverage, independent-failure and contradiction prompts adapt ideas from the Loft
[writing guidance](https://github.com/Ask-Loft/loft/blob/9bed930a32cd6bdc09a7274f45a4c665daf305de/crates/loft-cli/skill/loft-spec/references/writing-rules.md),
[review guidance](https://github.com/Ask-Loft/loft/blob/9bed930a32cd6bdc09a7274f45a4c665daf305de/crates/loft-cli/skill/loft-spec/references/reviewing.md)
and [reconciliation guidance](https://github.com/Ask-Loft/loft/blob/9bed930a32cd6bdc09a7274f45a4c665daf305de/crates/loft-cli/skill/loft-spec/references/reconciling.md),
reviewed 2026-10-16. They do not adopt its format, hierarchy, tooling or numerical heuristics.
