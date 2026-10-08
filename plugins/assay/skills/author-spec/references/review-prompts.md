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
