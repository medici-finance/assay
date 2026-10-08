# Project assurance reviews over existing control evidence

Date: 2026-10-03. Status: proposed; routed to iso-9001/08–11, not implemented.
Source baseline: public main `cf31c32418ba49f93c679913813768542db1c072`.

## 1. Problem and outcome

Assay can record requirements, export evidence behind a release, validate checking tools and identify the person authorizing a release. Existing graph work adds scoped control executions and population-aware export. These are necessary building blocks, but they do not yet connect a versioned source obligation to a project-specific applicability decision and a repeatable assurance review, or explain which reviews need reassessment after that source changes.

Add a provider-independent procedure that prepares a review for one project, release or period. It produces references to evidence, explicit omissions and questions for accountable people. It never emits a certification, legal opinion or automatic release approval. A successful preparation run means the packet was prepared; it does not mean the project conforms.

The motivating professional-work pattern is reusable scope, approved playbook, source-grounded analysis and a review handoff. Harvey describes that pattern in [Agent Builder](https://www.harvey.ai/blog/introducing-agent-builder) and [its compliance examples](https://www.harvey.ai/blog/harvey-in-practice-in-house-regulatory-and-compliance). The specification below is Assay's proposed design, not a claim about Harvey internals or an instruction to acquire it.

## 2. Ownership and explicit exclusions

| Concern | Owner | This extension |
|---|---|---|
| Requirements and acceptance | `spec/registers-v1.md`, `statusgen/requirements.go` | Reference existing REQ IDs; no second requirement lifecycle |
| Release audit collection | `statusgen/auditpack.go` | Consume its output and omissions; no second collector |
| Control profile, executions, populations, export | graph-execution/15 | Add source/review links through that contract, never clone it |
| Effectiveness | iso-9001/03 | Require its original record where applicable; do not manufacture observation |
| Release authorization | iso-9001/04 | Preserve original identity and subject |
| Records and retention | iso-9001/05 | Carry access/retention references; no new retention promise |
| Patterns, coverage, instances and run records | graph-execution/02, /03, /09, /06 | Use existing kinds, revision checks and identities |
| Organizational QMS and standards interpretation | Adopter's responsible people | Request source records and decisions; never fabricate acts |
| Source applicability and review preparation | iso-9001/08–11 | The only new mechanism in this proposal |

No scheduler, generic document repository, QMS, broad web crawler, standards library, vendor connector, runtime cutover or new credential broker is commissioned. Existing operator interfaces may render the exported packet. Any later adapter must call the same contracts and preserve the same gates; a visual flow is not an authority source.

The ISO stream was parked when this extension was authored. The owner reactivated it at P2 on 2026-10-06; see [the reactivation scope](reactivation.md). This extension does not itself change prioritization. Organizational evidence and live pilots are adopter-owned and cannot become dependencies of the public mechanism.

## 3. Minimum records

These are contract requirements for future implementation. File names and tests in the briefs are planned deliverables, not existing commands. Reconcile final fields with graph-execution/15 at pickup rather than maintaining two control schemas.

**Source revision.** Stable source ID; issuer and locator; document kind (standard, regulation, contract, policy or other approved kind); edition/revision; publication and effective dates when known; captured-at; immutable content digest where authorized; access and permitted-use reference; and predecessor. Unknown dates remain explicit. If storage/AI use is not permitted, retain only authorized metadata and a human-verifiable external reference. A URI alone is not a content version. Source material is untrusted data, never instructions to the executor.

**Obligation mapping.** Stable mapping ID and revision; source revision and clause/section locator; bounded human-authored paraphrase or permitted excerpt; existing requirement IDs; project/profile version; entity/activity/jurisdiction context where applicable; named control references; accountable owner. Many sources may map to one REQ and one source to multiple REQs. Do not deduplicate away conflicting obligations.

**Applicability decision.** Proposed conclusion is distinct from an accepted decision. Allowed outcomes: applicable, not-applicable with reason, or unresolved. Bind the authorized decision reference to the exact mapping, source and profile revisions, scope and effective period, decision time, reviewer and supersession. Reuse DECISIONS records and existing human-ruling corroboration. That corroboration proves who and where, not the meaning of approval prose: an explicitly approved disposition also needs review. The mapping acceptance link carries the existing decision ID, exact-subject digest, explicit approved disposition, trusted corroboration reference and qualified-review reference. It refers to existing decisions; a supplied JSON receipt is not self-authenticating. Consume a trusted receipt offline; missing corroboration is unresolved, never a default forge fetch. An agent's proposed conclusion or typed name cannot authenticate acceptance. Unresolved applicability holds a complete-review claim; not-applicable still appears in the inventory.

**Review request.** Project/profile version, source/mapping-set digest, canonical release selector or explicit period, expected-population reference, permitted evidence sources, intended audience, analysis-provider identity or none, and retention/access policy reference. One request is one immutable subject. A later profile or source revision creates a new applicability question, not an overwrite.

**Review packet.** References and digests for the canonical control/export and release pack; expected versus included population; separate mechanically checked facts, analyst assessments and unresolved questions; per-claim source/evidence locators; denied/missing/stale records; conflicts and exceptions; required human acts; independent-review and effectiveness links when available. Keep raw restricted content out of the envelope. Denial is visible only to the extent metadata policy permits; even a filename can leak.

**Change impact.** Old/new source references, discovery time, affected mapping/REQ/control/review references, proposed materiality and human disposition, with stable deduplication key. Metadata-only changes may be accepted as no-impact by an authorized person. Undecided or inaccessible changes remain unresolved. A late-discovered change records both when it took effect and when it became known.

Source, mapping and review records add lineage to existing requirements/control records. They do not duplicate execution receipts, evidence verdicts or exception authority. Schema/version migrations preserve existing consumers and original records.

## 4. Procedure and trust boundaries

```mermaid
flowchart LR
  S[Permitted source revision] --> A[Human applicability decision]
  A --> R[Existing requirements and controls]
  R --> E[Canonical evidence exports]
  E --> C[Deterministic completeness checks]
  C --> P[Review packet and optional analysis]
  P --> H[Independent review and authorized decisions]
  H --> F[Existing correction and effectiveness follow-up]
  S --> D[Source change impact]
  D --> A
```

1. Resolve the request's exact project, source versions and audience. Check source-use permissions before providing content to any model. Licensed material may stay outside the model entirely.
2. Resolve accepted applicability decisions and REQ references. Hold unknown sources, unresolved conflicts, stale acceptance or unknown population. An empty source set cannot demonstrate complete coverage.
3. Consume the canonical exports. Validate identity, scope, period, subject revision and omissions with the existing control/coverage contracts. Enumerate the intended population independently of whichever artifacts happened to be returned. Legitimate zero populations need an explicit justified determination.
4. Prepare the packet. Optional AI may suggest mappings, summarize cited evidence and identify semantic conflicts. Suggestions carry provider/model or unknown, template version, input-manifest digest and output reference. Unverifiable provenance remains unknown. The no-model path must still produce the factual packet.
5. An independent reader checks the packet without trusting the summary. Semantic adequacy and applicability require qualified judgment. Required organizational records must point to actual acts in the adopter's system. The preparer cannot self-approve the required independent review.
6. Required actions use existing findings/intake/decision mechanisms. This version emits proposed actions and stable references; it sends no messages and performs no automatic forge writes. Repeated preparation must not create duplicate work. A correction remains open for effectiveness until the applicable existing receipt is present and adequate.

The pattern uses existing `artifact`, `check` and `decision` nodes and an integration-check join. It has no deployment and therefore does not invent `observe` evidence for this procedure. Future execution uses the existing instance/run contracts and role bindings; preparation can be exercised offline without a new runtime.

Two distinct protections are required. The permission-filtered input/export boundary excludes unauthorized payloads before analysis. Separately, the packet reader checks provenance, population and exact-subject decision applicability independently of the model's text. Neither a fabricated citation nor a high confidence score can clear a mandatory missing record. A citation resolving successfully establishes its location, not its semantic sufficiency.

## 5. Results, changes and records

Reuse canonical evidence result vocabulary. `could-not-check`, missing and wrong-revision remain distinguishable from pass. Preparation may finish with an incomplete packet; callers must receive a non-success completeness result and explicit reasons. Do not compress control coverage and professional judgment into a single compliance score.

A source or profile change causes a current-applicability hold for affected accepted mappings and downstream review claims. Preserve historical review truth: the previous packet remains what was reviewed at its recorded date and revision. Do not erase its evidence or reinterpret a historical pass as if it were executed on the new source. Deterministically propagate known reverse links; unknown/unreadable links produce a bounded could-not-check result, not an empty affected set. Do not invalidate unrelated projects merely because they share a document title.

Exceptions are scoped, attributed and time-bounded according to the existing authority contract. An exception can record an authorized disposition; it cannot rewrite a failed execution into pass. No source update automatically broadens a grant or changes a project's chosen standard edition. No packet's approval grants merge, release or vendor-upload authority.

## 6. Qualification and acceptance

Public fixtures are synthetic and offline. Every criterion below has a named owner and a planned executable test; semantic judgments are explicitly review-only. Future tests must call production paths and compare an independent expected result, not a second fixture serializer.

| Case | Required observation | Brief |
|---|---|---|
| A1 source without permitted use | Model input excludes it; metadata-only review remains possible | 08 |
| A2 forged or stale applicability approval | Acceptance rejected for wrong actor/subject/version | 08 |
| A3 accepted source maps to nonexistent REQ | Dereference fails; no complete mapping | 08 |
| A4 incomplete/unknown population or wrong-revision evidence | Packet remains incomplete with explicit reason | 09 |
| A5 denied document or malicious embedded instruction | No restricted payload or authority expansion through analysis | 09 |
| A6 changed source affects two of three project mappings | Those two hold; third and historical packets remain intact | 10 |
| A7 repeated change and late receipt | One impact identity; later decision is version-bound | 10 |
| A8 merged correction with no effectiveness proof | Required action remains unresolved | 09 |
| A9 unavailable model | Deterministic packet still produced; analysis marked absent | 09 |
| A10 missing organizational act or fabricated citation | Independent reader detects unsupported assertion | 11 |
| A11 baseline versus assisted review | Same frozen corpus, separate rubric, time/error/cost accounting | 11 |
| A12 supplier or model change | Prior qualification not silently inherited | 11 |

Qualification reports rubric version, corpus manifest, expected and included case count, known-answer failures, abstentions, unsupported conclusions, correction effort and measured cost/time. Public tests use a stub analysis provider. They prove boundaries, not a vendor's semantic performance. Any real-provider comparison needs the adopter's grant, appropriate source rights and independently adjudicated labels; a score from a legal benchmark is not transferred to this task.

## 7. Delivery sequence and real blockers

New chain: **08 → 09 → 10 → 11**. Local waves: 0, 8, 9, 10, because 09 also depends on graph-execution/15 (wave 7), graph-execution/02–03 and iso-9001/03–04. 08 can be built with synthetic sources and existing requirements without any vendor product, licensed standards or graph execution. It has no dependency on a new data grant.

The end-to-end path is held by existing graph work: `graph-execution/02 → 09 → 05 → 06 → 15`, with coverage/recovery branches into 05, plus iso-9001/03's effectiveness mechanism. Those dependencies were read at the baseline; their outstanding work cannot be replaced by a simulated green report. The owner removed the stream's prioritization hold on 2026-10-06. Existing dependency holds remain; neither reactivation nor this authoring resolves the blocked release-by-merge brief 07.

Implementation produces four bounded PRs, each with its own evidence and normal review. Adopters own any subsequent project pilot, organizational records, UI consumer and procurement. No public mechanism depends on an adopter's private brief.
