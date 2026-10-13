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
