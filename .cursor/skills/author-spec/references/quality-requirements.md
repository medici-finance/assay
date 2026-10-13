<!-- GENERATED from plugins/assay/skills/author-spec/references/quality-requirements.md by deskinstall --harness cursor. Do not edit. -->
# Quality requirements that influence design

Use these prompts when creating or reviewing a spec to select the concerns that could change
its design or acceptance. They are a menu, not required headings or a compliance claim.
A small bounded change may inherit existing requirements unchanged; cite them. For a
consequential omission, record the gap and its consequence. Do not produce an N/A table for
every category or introduce a separate quality register.

## Select the material concerns

Existing author-spec prompts cover boundaries, failure paths, compatibility and reduced
complexity. Deepen those answers where quality matters instead of duplicating them here.

| Concern | Questions that can change the design or acceptance |
| --- | --- |
| Performance and capacity | What input sizes, growth, concurrency and invocation rates matter? What latency, throughput, memory, storage or request budgets apply, under which conditions? Where does work multiply? Use the [cost review prompts](review-prompts.md#cost-and-quality-at-design-time) for consequential algorithm choices. |
| Reliability and recovery | What happens when dependencies fail, requests time out or capacity is exhausted? What degraded behaviour is acceptable? State recovery-time and data-loss limits where needed; identify how restart, retry, restore and recovery will be demonstrated. |
| Data integrity and lifecycle | What must remain true under duplication, partial writes, reordering and concurrent changes? Specify required freshness, consistency, provenance, retention and deletion, including copies and backups where relevant. Name the authority for conflicting records. |
| Security and privacy | What assets, actors and trust boundaries matter? How are access, revocation, abuse and tenant isolation checked? What data is necessary, where can it travel, who can see it and when is it removed? Include telemetry and exports; successful authentication alone does not establish privacy. |
| Operability and diagnosis | How does an operator distinguish success, failure, partial completion and stale state? What evidence supports diagnosis without exposing sensitive data? Who responds, and what configuration, rollout, rollback and recovery actions must be feasible? |
| Usability and accessibility | Which users and tasks must work, with what error prevention and recovery? Consider keyboard and assistive-technology access, understandable feedback, language and format needs. Name applicable accessibility criteria and the automated plus human evaluation needed. |
| Compatibility and portability | Which API/schema versions, operating systems, runtimes and constrained or offline environments are supported? How are mixed-version operation, migration, replacement and exit/export handled? Specify the actual support boundary, not an unbounded portability promise. |
| Maintainability and testability | Which likely change should be localized, and what ownership or coupling would obstruct it? What can be tested deterministically, with which oracle? Account for new dependencies, temporary dual paths and retirement work; avoid arbitrary complexity-score targets. |
| Safety and misuse | Where failure could cause harm, identify hazardous outcomes, safe degraded states, stop/recovery behaviour and the evidence needed to assess them. Include foreseeable misuse and human intervention where relevant. Do not infer a safety certification from a test pass. |
| Cost and resource stewardship | What is affordable per operation and at expected volume? Include idle capacity, external API charges, storage, telemetry and operator effort where material. State resource or energy constraints when relevant; make the tradeoff with latency, resilience and retention explicit. |

Applicable contractual, regulatory or organizational constraints cut across these concerns.
Cite the controlling version, scope and responsible owner; never invent an obligation or
claim compliance from this menu. For probabilistic output, define an evaluation population,
error tolerance, fallback and who judges unacceptable outcomes when these affect acceptance.

## Turn the important concerns into evidence

Identify the stakeholders whose outcomes depend on a quality requirement. Distinguish a hard
constraint from a target or preference, and prioritize the scenarios that could invalidate
the design. Record conflicts and who resolves them: for example, diagnostic detail versus
privacy, caching versus freshness, or redundancy versus cost. Do not silently weaken an
inherited security, safety or contractual constraint to meet another target.

Express a material requirement in ordinary prose or the spec's existing table:

> Under **these conditions**, when **this event or change** affects **this boundary**, the
> system or operator must produce **this response**, meeting **this observable criterion**.

Add only the context needed to interpret it: workload/environment, measurement window or
percentile where relevant, exclusions, source and whether the target is approved or proposed.
Use observable pass/fail behaviour when a number adds no meaning. An unknown threshold needs
an owner and a way to resolve it, not a made-up number. Avoid copying illustrative targets
from a framework into the product's requirements.

Name the evidence method and owner: analysis or a prototype now, a benchmark or fault test
after implementation, or operational observation after release. State what could falsify the
design and which uncertainty must be resolved before dependent work. Design-time estimates
are not measured proof. Preserve the same acceptance boundary through the brief handoff;
system recovery, accessibility and privacy cannot be proved solely by local unit tests.

Examples of useful questions, not default requirements:

- If a worker stops after persisting half a batch, can retry complete it without duplicate
  effects? Identify the fault-injection point, state oracle and agreed recovery limit.
- Can the intended user complete and correct the primary form using only a keyboard, with
  errors announced by supported assistive technology? Name the applicable criteria and test.
- When a record is deleted, which replicas, logs and backups retain it under the approved
  policy, and what evidence proves the promised disposition?
- If a batch corpus and rule count grow independently, does runtime stay within the agreed
  budget while outputs and refusal behaviour remain equivalent?

## Source basis

Research consulted 2026-10-13. This reference adapts ideas; it does not reproduce a standard
or require adopters to implement any framework in full.

- [ISO/IEC 25010:2023, public abstract](https://www.iso.org/standard/78176.html): a product
  quality model supports specifying and evaluating quality. The full standard was not used;
  the menu above is a practical synthesis, not its official taxonomy.
- [SEI, Deriving Architectural Tactics (2003)](https://sei.cmu.edu/documents/704/2003_005_001_14213.pdf):
  quality scenarios make architectural concerns concrete through stimulus, environment,
  affected artifact, response and response measure.
- [arc42, Quality Requirements](https://docs.arc42.org/section-10/): capture concrete usage
  and change scenarios and reuse already stated quality goals. This motivates proportional
  detail rather than mandatory headings.
- [Google SRE, Implementing SLOs](https://sre.google/workbook/implementing-slos/): user-relevant
  indicators extend beyond availability and latency to freshness, correctness and durability;
  the measurement definition matters alongside the objective.
- [AWS Well-Architected pillars](https://docs.aws.amazon.com/wellarchitected/latest/framework/the-pillars-of-the-framework.html):
  operations, cost and sustainability complement security, reliability and performance.
  These concerns apply here without prescribing a cloud provider or deployment model.
- [W3C, WCAG overview](https://www.w3.org/WAI/standards-guidelines/wcag/): accessibility has
  testable success criteria; select the applicable version and scope rather than saying
  merely accessible.
- [NIST, Privacy Framework introduction](https://www.nist.gov/privacy-framework/getting-started-0):
  consider risks to people from data processing across its lifecycle, beyond security alone.
