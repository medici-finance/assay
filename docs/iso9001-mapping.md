# ISO 9001 clause map — what Assay's shipped artifacts speak to, and what they do not

**Status: an analysis mapping. Not a conformance assessment, not a gap analysis of your
organisation, and not an audit opinion.** This page reads the artifacts that ship in this
repository against the ISO 9001 clause skeleton (clauses 4–10) and says, per clause, which
artifact is relevant, whether it is machine-enforced or merely recorded, and what an
adopter still has to supply themselves. Adopting Assay does not confer, contribute to, or
substitute for certification, and nothing on this page should be quoted to an auditor as
though it did.

> **Confirm the clause identifiers against the standard before using this page with an
> auditor.** We do not hold a licensed copy of ISO 9001; the clause numbering and the
> "what it asks" column are our paraphrase of the publicly documented clause structure,
> not a verified citation of normative text. Treat the left-hand columns as *our reading
> of the concern*, in the same spirit as the SOC2 CC8.1 table in
> [`docs/evidence-bundle.md`](evidence-bundle.md) — that page is this one's template, and
> its Enforcement column is reused here for the same reason.

**Edition.** This maps to the **2015** clause numbering. The sixth edition (ISO 9001:2026)
preserves the clause 4–10 harmonized structure, and its changes are clarifying and
additive rather than structural — climate change folded into the clause 4 context
requirement, quality culture and ethical behaviour added at 5.1.1 and 7.3, strategic
direction at 5.2.1, and the clause 6.1 risk requirement split so that risks and
opportunities are addressed separately. None of those move an artifact from one row of
this table to another. 2015 certificates remain valid through the transition. If your
certificate is against the 2026 edition, read this page as the 2015 baseline plus those
additions, all of which land in rows this page already marks as the adopter's own.

**ISO/IEC/IEEE 90003** is the guidance standard for applying ISO 9001 to software. It adds
nothing to ISO 9001's requirements and is explicitly not assessment criteria for
certification. It is a useful translation aid when arguing that a software practice
satisfies a clause; it is not a second bar and this page does not map to it.

---

## How to read this page

**The Enforcement column is the load-bearing one**, for the same reason it is in the
evidence bundle: the difference between "a convention we honour" and "a control that would
have stopped it" is the difference between a pass and a finding.

- **Enforced** — a check fails and the artifact cannot pass lint or cannot land. Machine-checked.
- **Advisory** — a convention the adopting team honours and which the artifact *records*,
  but which nothing prevents a participant from bypassing. An advisory row is evidence
  that something was **claimed**, not that it was **done**.
- **—** — no mechanism; the row is here so its absence is visible rather than silent
  ([`docs/mistake-proofing.md`](mistake-proofing.md) **D6**: honesty about non-coverage is
  itself a device).

**The rightmost column is the one an adopter should read first.** Assay is a delivery
methodology and a toolchain. It is not a quality management system, and several clauses —
the whole management-system wrapper — are wholly the adopter's. Those rows say so.

**One structural limit sits underneath every row and is not visible in the tables.** The
board and the brief records are *derived from agent-authored artifacts with consistency
linting*, not measured from ground truth
([`docs/lifecycle.md`](lifecycle.md) §"The honest claim about the board"). The linter checks
the internal consistency of documents written by the same agents whose work they report.
That disclosure is a strength at an audit — it is exactly the "sufficiently specific and
traceable, verifiable information" question a competent auditor asks — but it does mean
that where a row says Advisory, the record is a claim and not an observation.

---

## Clause 4 — Context of the organization

| Clause | What it asks | Assay artifact (this repo) | Enf. | What the adopter must still supply |
|---|---|---|---|---|
| 4.1 Context | Determine and monitor external/internal issues relevant to purpose and strategic direction | — | — | The whole clause. Assay ships no context analysis and no cadence for revisiting one. (An auditor's habitual move here is to check the revision date on the issues register — a context written once at implementation and never touched is the characteristic finding.) |
| 4.2 Interested parties | Determine relevant parties and their requirements; monitor them | [`CONTRIBUTING.md`](../CONTRIBUTING.md), [`SECURITY.md`](../SECURITY.md), [`CODE_OF_CONDUCT.md`](../CODE_OF_CONDUCT.md), and the INTAKE register ([`docs/streams/INTAKE.md`](streams/INTAKE.md)) as an external-request front door with a recorded disposition per entry | A | The parties list and their requirements. Assay gives you a front door and a disposition vocabulary, not a requirements register or a monitoring cadence |
| 4.3 Scope of the QMS | A documented scope: boundaries, applicability, justification for exclusions | [`topology.yaml`](../topology.yaml) and [`docs/streams/graph-repos.yaml`](streams/graph-repos.yaml) declare which repos exist and how they are classed — machine-readable, one declared source, diffed rather than hand-copied | A | The scope *statement*. The roster is raw material an adopter can generate a scope boundary from; it is not a scope statement and makes no exclusion justification |
| 4.4 QMS processes | Determine processes, their inputs/outputs, sequence and interaction, criteria and methods, responsibilities, and evaluation | [`docs/how-assay-works.md`](how-assay-works.md) (the desk loop is the process map, with who may act at each step), [`docs/lifecycle.md`](lifecycle.md), [`docs/brief-rules.md`](brief-rules.md), [`docs/registers.md`](registers.md). Process *interaction* is the typed brief dependency graph (`depends:`/`unblocks:`, waves) — see [`docs/dependency-graph-design.md`](dependency-graph-design.md) | **E** (lint enforces the artifact shapes and the dependency-graph invariants) | An explicit inputs/outputs/owner table per process, if your auditor wants one in that shape. The dependency graph is a stronger process-interaction artifact than most process maps, but it is drawn over *work items*, not over organisational processes |

---

## Clause 5 — Leadership

| Clause | What it asks | Assay artifact (this repo) | Enf. | What the adopter must still supply |
|---|---|---|---|---|
| 5.1.1 Leadership and commitment | Top management accountable for the QMS; ensure policy and objectives; integrate requirements into business processes; ensure resources | The human merge gate — nothing in the loop merges itself ([`docs/how-assay-works.md`](how-assay-works.md), the desk loop's **HUMAN MERGE** step; [`docs/adopting-assay.md`](adopting-assay.md) §"Human-gate quick reference (never autonomous)") | A | All of it. Evidence that decisions are *attributed* is available; evidence of commitment to a management system is not, because Assay ships no management system to commit to |
| 5.1.2 Customer focus | Ensure customer and statutory requirements are determined and met; address risks; enhance satisfaction | Brief `risk: {customer: yes/no}`, which is one of the four answers the review `gate:` is derived from | **E** (risk/gate consistency is lint-enforced) | The customer-requirement register and the satisfaction measurement (see 9.1.2) |
| 5.2 Quality policy | Establish, maintain and communicate a policy appropriate to purpose, committing to satisfying requirements and to continual improvement | Not an artifact. Doctrine that a policy could draw on exists in prose: [`README.md`](../README.md) ("the aim is not a green dashboard but a set of gates that make a passing state hard to fake"), [`docs/lifecycle.md`](lifecycle.md) §"claim the weaker, true thing", [`docs/mistake-proofing.md`](mistake-proofing.md) §1 ("errors are inevitable; defects are not") | — | The policy. One approved, versioned, communicated page, owned by the adopter's top management. Assay has opinions about engineering honesty; it does not have your quality policy and must not be read as one |
| 5.3 Roles, responsibilities and authorities | Assign, communicate, and ensure understanding of responsibilities and authorities | Separate identities for author, reviewer and verifier — a review verdict must come from an identity the implementer cannot write on its own behalf ([`docs/how-assay-works.md`](how-assay-works.md) claim 3); the trust gate names whose issues the tooling will act on at all (`statusgen/trustgate.go`); `statusgen --lint`'s attribution family (`attribution.go`, `evidenceactor.go`, `humanstamp.go`) refuses several self-attestation shapes | **E** (attribution, human-stamp validation, evidence-actor) | The organisational assignment: who owns document control, who owns nonconformity handling, by name. Assay assigns authority over *work items*, not over the QMS |

---

## Clause 6 — Planning

| Clause | What it asks | Assay artifact (this repo) | Enf. | What the adopter must still supply |
|---|---|---|---|---|
| 6.1 Risks and opportunities | Determine risks and opportunities; plan proportionate actions; integrate them into processes; evaluate their effectiveness | Per-brief `risk: {regulatory, customer, irreversible, sensitive-data}` with `gate:` **derived** from the four answers (any `yes` ⇒ `gate: human`) and `gate-why:` required on a risk-gated brief; [`docs/mistake-proofing.md`](mistake-proofing.md) §2 classifies a device by timing × mode × bypass, which is a controls-strength taxonomy; the FINDINGS register is the realised-risk log | **E** (gate derivation, risk/gate consistency, `gate-why` presence) | The organisation-level risk register — Assay's risk model is per work item and is four fixed booleans, a deliberate simplification an auditor may probe. Also the 6.1 effectiveness evaluation, which has no artifact here. Note: ISO 9001 does not mandate a risk register or any methodology; it mandates evidence that risks were considered and that action effectiveness was evaluated |
| 6.2 Quality objectives | Measurable objectives consistent with policy, monitored, communicated, updated; plans stating what, who, when, and how results are evaluated | Not an artifact. The *mechanism* to hold an objective already exists — an objective with a plan is a stream with a Verify table — and the measurement substrate is dense (`statusgen --dora`, `--trend`, `--bottleneck`, `--autonomy`, `--alarms`, `--gate-telemetry`, `--lint-audit`; `qualgen/`) | — | The objectives themselves, with targets, owners and a review cadence. **A caution worth taking from the corpus:** every report in that list carries a written non-gate disposition, and [`docs/mistake-proofing.md`](mistake-proofing.md) **D7** says plainly that a metric which can be satisfied without the quality it proxies will be. Setting objectives on *control coverage and control latency* (every control-mode device carries a current fired-demonstration; every instrument has a recorded could-not-check disposition; no finding sits unresolved and un-parked past N days) is Goodhart-resistant in a way throughput targets are not |
| 6.3 Planning of changes | Changes to the QMS carried out in a planned manner: purpose, consequences, integrity, resources, responsibilities | Applied to work rather than to the QMS: a brief's `## Verify` table is its contract, and [`docs/brief-rules.md`](brief-rules.md) rule 14 routes any mid-flight tweak by one test — does the Verify table change? If yes, amend the brief in the same commit and demote it so it re-gates. Rule 15 makes splitting a brief mid-execution an authoring act | **E** (lifecycle/gate consistency) | Change planning for the *management system*. Assay's change discipline is a good model to copy; it is not a QMS change procedure |

---

## Clause 7 — Support

| Clause | What it asks | Assay artifact (this repo) | Enf. | What the adopter must still supply |
|---|---|---|---|---|
| 7.1.1–7.1.2 Resources / people | Determine and provide the resources and people needed | Fleet capacity is modelled explicitly rather than left implicit — per-stream concurrency caps and board span-of-control limits in the Next-up computation ([`docs/lifecycle.md`](lifecycle.md) §"Next-up semantics") | **E** (the caps are code) | The resourcing decision and its record. A scheduler constant is not a reviewed commitment |
| 7.1.3 Infrastructure | Determine, provide and maintain infrastructure | [`containers/`](../containers/), [`images/`](../images/), [`Dockerfile`](../Dockerfile), [`docs/docker.md`](docker.md), the pinned toolchain contract in [`docs/distribution.md`](distribution.md) | A | A maintenance and currency record for the infrastructure itself |
| 7.1.4 Environment for process operation | Determine, provide and maintain the environment | Worker isolation — each worker owns its own workspace and never the shared checkout ([`docs/how-assay-works.md`](how-assay-works.md), the fan-out step); [`.claude/guardrails/GUARDRAILS.md`](../.claude/guardrails/GUARDRAILS.md) as the single declared home for rules more than one skill must state | **E** (isolation), A (the guardrail text) | Nothing structural, but be honest with your auditor about what a permission deny-list can and cannot see: string-prefix matching cannot read inside a script a session has already committed |
| **7.1.5 Monitoring and measuring resources** | Resources suitable and maintained, with retained evidence of fitness for purpose; act on prior results when a resource is found unfit | **See [the 7.1.5 section below](#715--the-positive-control-discipline-as-the-calibration-analog).** The short form: the positive-control / mutation discipline is the calibration analog, it is doctrine, and it is a fail-closed release gate | **E** (the mutation gate blocks a release; mutation-row presence is a brief rule) | A retained, *shippable* record. Today the demonstration runs and is thrown away — it lives in CI logs, not in an artifact. Also the re-run interval and the action-on-prior-results rule; see the section |
| 7.2 Competence | Determine required competence; ensure it; where action is taken, evaluate its effectiveness; retain records | `effort:` keys the execution tier and the optional `exec-tier: any \| strong` **tightens only** ([`docs/brief-rules.md`](brief-rules.md) rules 12–13); every Verified and Reviewed cell names a dated runner, and `attribution.go` enforces a verifier-tier floor that refuses a cheap-model verifier | **E** (`exec-tier` value set, attribution format, tier floor) | **The humans.** Competence criteria per role, records against them, and — the obligation organisations reliably miss — evidence that any competence action *worked*. ISO 9001's competence clause is about persons; a model tier is not a competence record for a person, and the human at the merge gate is the load-bearing control with no competence record in this repo at all |
| 7.3 Awareness | Persons are aware of the policy, relevant objectives, their contribution, and the implications of not conforming | Delivered mechanically: the plugin's `SessionStart` hooks ([`plugins/assay/hooks/inject-resident-rules.sh`](../plugins/assay/hooks/inject-resident-rules.sh), [`inject-board-state.sh`](../plugins/assay/hooks/inject-board-state.sh)) inject the method into every session. Any rule more than one skill must state verbatim has **one declared home**, [`.claude/guardrails/GUARDRAILS.md`](../.claude/guardrails/GUARDRAILS.md), with a `make skillslint` target that byte-diffs every copy against it and a `make guardrail-sync` target that regenerates them | **E** (injection is not optional once installed), A (the byte-diff is a Make target and is not wired into a workflow in this repo, so nothing fails on drift here) | Awareness of *your* policy and objectives — neither of which Assay ships. The delivery mechanism is already built and is re-delivered every session; there is nothing organisational in it yet. Note also that 7.3 is evidenced by interview, not by documents |
| 7.4 Communication | Determine internal and external communication: what, when, with whom, how, and by whom | The register front doors ([`INTAKE`](streams/INTAKE.md), [`FINDINGS`](streams/FINDINGS.md)), the PR review flow ([`docs/desk-tools/deskpr.md`](desk-tools/deskpr.md)), the label and identity conventions | A | A communication plan in ISO's what/when/whom/how/who shape. Assay enumerates channels; it does not state obligations against them |
| 7.5.1–7.5.2 Documented information — create and update | Identification and description, format, review and approval before use | Brief frontmatter carries a hierarchical typed `brief:` ID, `authored:` date and author, and `sources:` (an empty `sources:` is untraceable and is a lint failure); approval is the PR review by a non-author identity. [`tools/freshness/`](../tools/freshness/) re-review machinery reads a manifest of per-artifact `last-reviewed` / `max-age-days` / `upstreams` plus optional per-claim anchors, so a single stale sentence inside a current document is catchable | **E** (required keys, `sources:` presence, typed IDs) | The freshness manifest itself — `tools/freshness` ships, the `freshness.yaml` it reads is yours to write and yours to keep covering enough of the document set to mean anything |
| 7.5.3 Control of documented information | Availability, distribution, access, storage, version control, **retention**, disposition; control of external-origin documents; obsolete documents identified | Version control is git plus the brief `version:` discipline. Tamper-visibility is real and machine-checked: registers are append-only, withdrawal is a tombstone and never a deletion, and `statusgen --lint` enforces duplicate-id detection and a tombstone check against history (`registers.go`) — an entry that has ever existed on main and is absent from the working tree is a lint failure. Generated artifacts have a **single writer**: `STATUS.md` is regenerated only by main's CI and PR CI blocks any diff that touches it ([`docs/lifecycle.md`](lifecycle.md) §"STATUS.md — a single-writer generated artifact"). Register-reference link integrity and view-drift are lint-checked (`registerrefs.go`, `viewlinks.go`, `linkcheck.go`) | **E** (single-writer, duplicate-id, tombstone, reference integrity, view drift) | **Retention, which has no answer here at all.** No artifact in this repo states a retention period or a disposition rule for any record class; "append-only forever" is an implicit retention rule nobody wrote down. Also external-origin document control — `sources:` per brief is the nearest thing and is not that. Internalize the split your auditor will use: documents are *maintained*, records are *retained* |

---

## Clause 8 — Operation

| Clause | What it asks | Assay artifact (this repo) | Enf. | What the adopter must still supply |
|---|---|---|---|---|
| 8.1 Operational planning and control | Process and acceptance criteria; resources; records sufficient for confidence the process ran as planned | Streams, waves and the typed dependency graph; the computed Next-up queue with its priority, staleness, per-stream cap, findings exclusion and claim exclusion — and a **DEGRADED banner** on the board when the claim read fails, rather than a silently unfiltered list ([`docs/lifecycle.md`](lifecycle.md)). Acceptance criteria are the `## Verify` table's Expect column; the `## Evidence` section is the retained confidence record | **E** (lint, single-writer board), A (dispatch discipline) | Nothing structural. Read the honest-claim caveat above: in the general case the 8.1 confidence record is authored, not observed |
| 8.2.1 Customer communication | Product information, enquiries, feedback and complaints, customer property, contingency | [`SECURITY.md`](../SECURITY.md) for vulnerability reporting, [`CONTRIBUTING.md`](../CONTRIBUTING.md), GitHub issues, the INTAKE front door with its per-entry disposition | A | A feedback and complaint process distinct from issue triage, and a contingency-requirements statement |
| 8.2.2 Determining requirements | Requirements defined including statutory and regulatory; the organisation can meet the claims it makes | Brief `## Context` (`files:`, `facts:`), `sources:`, `depends:`, and `risk: {regulatory: …}` | **E** (presence) | Statutory and regulatory determination for your product. Assay records that a brief was *asked* about regulatory risk; it does not determine what applies to you. The REQUIREMENTS register ([`../spec/registers-v1.md`](../spec/registers-v1.md) §6, entries under `docs/streams/requirements/`) records the ask itself — who wanted it, its ranked impact, and the acceptance criteria — and a brief may cite it with the `satisfies:` key. That is a record of what was asked and what was claimed to satisfy it: no brief is required to cite a requirement, but a citation naming a requirement that does not exist (`dangling-satisfies`) is a hard PROBLEM (`registers-v1.md` §6.5) — and none of it makes a requirements traceability matrix an ISO 9001 requirement — see row 8.5.2, which says why |
| 8.2.3 Review of requirements | Review before commitment; resolve differences; confirm capability; retain the results | [`docs/mistake-proofing.md`](mistake-proofing.md) **B5** (pre-mortem mapped to detection rows), **B6** (negative control — confirm the Verify table goes red on a wrong-but-plausible deliverable), **B7** (a do-confirm checklist at the dispatch pause), **B8** (a fresh model instance answers comprehension questions from the brief alone before dispatch — source inspection applied to the specification) | A (B5–B8 are prose rules; only row *presence* is lint-checkable) | The retained *results*. B8's comprehension probe leaves no artifact today, so the review happened and cannot be shown to have happened |
| 8.2.4 Changes to requirements | Amend the documented information and make people aware | The brief `version:` discipline and the rule-14 routing test above; a Verify-table change is a contract change and re-gates | **E** | Nothing material |
| 8.3.2 Design & development planning | Stages and controls, reviews, verification and validation activities, responsibilities, resources, interfaces | The lifecycle `todo → in-progress → implemented → verified → done` with stage-specific authority (an implementer **stops at `implemented`**); `wave:`/`depends:` sequencing; `effort:`/`exec-tier:` as the complexity-to-resource mapping; `gate: model \| human` as the level of control | **E** | Nothing material |
| 8.3.3 Design & development inputs | Functional and performance requirements, prior-design information, statutory/regulatory, standards committed to, **the potential consequences of failure**; adequate, complete, unambiguous, conflicts resolved | `## Context` with `files:` and `facts:`; `sources:` (empty ⇒ untraceable); `depends:`; `risk:`; [`docs/mistake-proofing.md`](mistake-proofing.md) **B10** (a `facts:` entry carries the command that re-establishes it, so staleness is detectable) and **B5** (the pre-mortem is where consequences-of-failure land) | **E** (`sources:` presence), A (adequacy) | Adequacy, and conflict resolution among inputs, which have no mechanism beyond review. This is the clause where the practitioner literature is sharpest about agile artifacts: a backlog item is not a design input, largely because consequences of failure almost never appear in one. A brief with a real pre-mortem is a better answer than a user story — but the pre-mortem is advisory here, so say so |
| **8.3.4 Design & development controls** | **Review** (can this meet requirements), **verification** (do outputs meet inputs), **validation** (does the result meet requirements for the intended use); act on problems; retain records | **Review and verification are separated, named and structurally enforced — this is Assay's core claim.** A **non-implementer** re-runs the Verify table on the merged result before a brief can read `verified`; merging is explicitly not verification. Review is a separate recorded verdict, dated and attributed, from an identity the implementer cannot write on its own behalf ([`docs/how-assay-works.md`](how-assay-works.md) claims 2 and 3; `done` requires both). The enforcement behind those words is heavier than the prose: `attribution.go` (Verified/Reviewed cell format plus a verifier-tier floor), `evidenceactor.go` (a verified row whose Evidence runner *is* the implementing session), `humanstamp.go` (`human:<name>` validated against git identity and the roster), `corroborate.go` (`--corroborate --pr` cross-checks a human stamp against the PR's actual reviews and **exits 1 on disproof**), `verifyrows.go` (a Verify command *structurally incapable of failing* — an automated detector for manufactured evidence), `unrun.go` (Verify rows minus Evidence rows = unproven work, derived and never declared) | **E** for review and verification | **Validation is the missing third activity, and auditors insist the three be distinguishable.** Assay has no artifact that asks whether the change achieved the purpose the brief existed for, in the environment it will be used in. The nearest hook is [`docs/brief-rules.md`](brief-rules.md) rule 43 ("dereferencing vs. presence") — at least one row *capable of failing on a wrong-but-well-formed deliverable*. That is validation-flavoured and it is a row obligation, not a named activity. "We code-review everything" as the answer to all of 8.3.4 draws a finding; do not let this row become that |
| 8.3.5 Design & development outputs | Meet input requirements; adequate for subsequent provision; reference acceptance criteria and monitoring requirements; retained | Deliverables plus `## Evidence` plus the Verify table as the referenced acceptance criteria. **An execution witness exists**: `statusgen verifyrun --brief <path>` runs each Verify row in a fresh subshell at the repo root and appends a witness row carrying the command, exit code, sha256 of combined output, date, runner identity and the tree SHA. Runner attribution is **derived, never supplied** — there is no `--runner` flag and passing one is a dedicated usage error; with no derivable identity it writes nothing and exits 2. Three exit states (`pass` / `fail` / `could-not-run`), and `could-not-run` deliberately contains the substring `not-run` so it cannot launder an unproven row into coverage. `witnessgate.go` makes it load-bearing: a brief whose Evidence records a **failed** witness may not read `verified` or `done` | **E** (Evidence presence at `verified`/`done`; a witness contradiction is a PROBLEM) | Read the tool's own honesty note rather than over-claiming it: the environment is not a cryptographic anchor, and the witness is evidence for a reviewer reading a diff, not an unforgeable attestation. Witness *absence* is a lower severity than contradiction, so an unwitnessed `verified` can still land |
| 8.3.6 Design & development changes | Identify, review and control changes during or after development; retain the change, the review result, the authorization, and the actions taken | `version:` bumps, `supersedes:` lineage, and `consumers:` frontmatter **corroborated against the branch diff** by `statusgen --consumers` (`consumers.go`), which exits 1 on a claim the diff contradicts and reports could-not-check when re-run after merge, because the diff that made the claim true is gone | **E** | Little. The `consumers:` corroboration is an unusually strong prevent-adverse-impact control: every reader of a changed shared value must be enumerated and routed, and the routing is machine-checked against the actual diff |
| 8.4 External providers | Criteria for selection, evaluation, performance monitoring and **re-evaluation**, with records; control proportionate to impact; requirements communicated to the provider | **See [the 8.4 / 8.6 section below](#84--86--supply-chain-and-release-control)** | **E** (pin verification), — (supplier evaluation) | The supplier file. Also: the *model provider* is an external provider and appears nowhere in this repo as one |
| 8.5.1 Control of production and service provision | Documented information defining characteristics and results; monitoring at appropriate stages; suitable environment; competent persons; **actions to prevent human error**; release and post-delivery | The brief is the work instruction, the Verify table is the acceptance criterion, the lifecycle stage gates are the monitoring points, worker isolation is the environment control. [`docs/mistake-proofing.md`](mistake-proofing.md) in its entirety is the actions-to-prevent-human-error clause done to an unusual depth — source over informative over judgment inspection, control versus warning, the bypass axis, and the seven device rules D1–D7 | **E** + A per device; the classification itself records which | Nothing structural, but present the device classification honestly: several devices are warnings, and [`docs/mistake-proofing.md`](mistake-proofing.md) **D3** requires every deliberate bypass path to be logged, attributed and where possible expiring — check that yours are |
| 8.5.2 Identification and traceability | Identify outputs and their status with respect to monitoring requirements; control unique identification where traceability is required; retain | Hierarchical typed brief IDs, register slug IDs (`F-<slug>`, `I-<slug>`), the PR `Brief:` trailer, findings `affects:` typed IDs, `sources:` provenance, and status **derived** from the artifacts rather than asserted as frontmatter. The umbrella release tag is stamped into the binary via `-ldflags`, so a running tool reports which release produced a record | **E** | Nothing material. Note that ISO requires traceability records only *where traceability is a requirement*; a requirements traceability matrix is borrowed from regulated domains and is not an ISO 9001 requirement |
| 8.5.3 Property belonging to customers or external providers | Identify, verify, protect and safeguard; report loss or damage | [`containers/secrets.md`](../containers/secrets.md), the layer secret scan ([`containers/scripts/layer-secret-scan.sh`](../containers/scripts/layer-secret-scan.sh)), [`.gitleaks.toml`](../.gitleaks.toml), and the leak-sweep workflows | **E** (the sweeps and the layer scan) | Custody policy for your own credentials, and the customer-data question, which is entirely yours. [`docs/telemetry.md`](telemetry.md) is relevant in the other direction: telemetry is opt-in, off by default, counts-only, and ships with no receiver configured |
| 8.5.4 Preservation | Preserve outputs to the extent necessary | Git; append-only registers with tombstone-not-delete and the history-comparison deletion check; single-writer generated views | **E** | Ties to the 7.5.3 retention gap: preservation here is unbounded by default, with no stated period and no disposition rule |
| 8.5.5 Post-delivery activities | Extent determined by statutory requirements, undesired consequences, nature and lifetime, customer requirements and feedback | [`docs/distribution.md`](distribution.md) (`deskmigrate` migration runner, the upgrade skill, and "there is no rollback" stated plainly), [`SECURITY.md`](../SECURITY.md), [`tools/bugs-gc/`](../tools/bugs-gc/) | A | A support commitment, a defined end-of-support policy for a released umbrella version, and — see 8.7 — a path for notifying adopters of a defect found in a **shipped** release. That last one is the item most tool vendors omit and it is the adopter's own nonconformity trigger |
| 8.5.6 Control of changes | Review and control changes to production/service provision; retain the review result, who authorized, and the actions | `version:` plus `supersedes:` plus the `consumers:` corroboration plus single-writer generated artifacts plus `deskmigrate`, with the review verdict recorded with date and identity | **E** | Nothing material |
| **8.6 Release of products and services** | Planned arrangements completed before release; retain evidence of conformity with acceptance criteria **and traceability to the person(s) authorizing release** | **See [the 8.4 / 8.6 section below](#84--86--supply-chain-and-release-control)** | **E** (the guard, the mutation bar, asset immutability, the environment approval), A (authorization traceability) | The authorizing-person record in a form your auditor can read, and a release-ranged evidence pack |
| 8.7 Control of nonconforming outputs | Identify and control nonconforming output to prevent unintended use or delivery, including after delivery; correct, segregate, inform the customer, or obtain a **concession**; re-verify after correction; retain the nonconformity, the actions, any concession, and who decided | The **FINDINGS register**: per-entry files with YAML frontmatter (`id`, `date`, `title`, `affects`, `ack`, `resolved`), append-only, tombstoned, duplicate-id and deletion detection — **plus the segregation control**: a brief with an unresolved finding against it is held out of the Next-up queue until the finding resolves ([`docs/lifecycle.md`](lifecycle.md) §"Next-up semantics"). **Parking** is bounded shelving: `parked-until` (a hard expiry), `parked-by` (`human:<name>` authority) and `parked-reason`, all three required together — a park is a snooze, not a mute. Alarm-management KPIs are applied to the register itself (`alarms.go`: standing-alarm age, alarm rate, flood thresholds, an intake-untriaged alarm, and a Next-up **overflow alarm** rather than silent truncation). `findingcontrol.go` surfaces a recurring-class finding that has landed no permanent control | **E** (exclusion, duplicate-id, tombstone), **A** (`findingcontrol` and the alarm KPIs are advisory NOTICEs) | Three things. (a) The disposition vocabulary covers correction and segregation; **parking is a genuine concession mechanism** — bounded, authorised by a named human, auto-re-annunciating — but is not *labelled* as a concession and is not reported as one. (b) "Informing the customer" has no path. (c) The register carries the nonconformity and the action; it does not carry the *result* of the corrective action — see 10.2 |

---

## Clause 9 — Performance evaluation

| Clause | What it asks | Assay artifact (this repo) | Enf. | What the adopter must still supply |
|---|---|---|---|---|
| 9.1.1 Monitoring and measurement | Determine what to monitor, the methods, **when it is performed**, and **when results are analysed and evaluated**; retain evidence | Dense and generated rather than asserted: `statusgen --dora`, `--trend` (a status-transition historian appended by `--record`), `--bottleneck`, `--autonomy`, `--alarms`, `--gate-telemetry`, `--verif-backlog`, `--intake-debt`, `--ladder`, `--signoff-digest`, `--roadmap`; plus [`qualgen/`](../qualgen/) (churn, hotspots, coupling, fix identification, PR risk features). **The measurement system monitors itself**: `--lint-audit` (`lintaudit.go`) samples recent daily commits, tallies per-rule firings and flags **COLD** zero-firing rules as retirement candidates — an internal audit of the control set, advisory and never auto-retiring | **E** (the metrics are generated from the artifacts, not typed) | ISO's two timing questions. *When* measurement happens is partly answered by CI and cron; *when results are analysed and evaluated* is not answered at all, and there is no evaluation record — no artifact saying "we looked at these numbers on date D, concluded C, and did A" |
| 9.1.2 Customer satisfaction | Monitor customers' perception of the degree to which needs and expectations are fulfilled; determine the methods | — | — | The whole clause, and Assay ships nothing that helps you collect it. A satisfaction or feedback disposition on the INTAKE register would be a natural place to put it |
| 9.1.3 Analysis and evaluation | Analyse and evaluate data on conformity, satisfaction, QMS performance, risk-action effectiveness, external-provider performance, and improvement needs | The generators above produce the data | — | The analysis-to-conclusion step, which has no home here. It is the same hole as 9.3 seen from the other side, and one artifact closes both |
| 9.2 Internal audit | Audit at planned intervals against your own arrangements and the standard; establish and maintain an audit **programme** (frequency, methods, responsibilities, planning, reporting) informed by process importance, changes, and previous results; define criteria and scope per audit; select auditors so as to ensure objectivity and impartiality; report to management; act without undue delay; retain records | The audit *method* is well developed — independence is a first-class concept throughout (non-implementer verification; a review verdict from an identity the implementer cannot write on its own behalf), and the corpus's own close rule is a good one: a finding closes when its check goes green, never on a fix commit | A | **Everything except the method: the programme.** What is audited, how often, by whom, against what criteria, reported to whom, retained where — plus an index of the audits already performed. Two cautions. The blunt "auditors shall not audit their own work" sentence is **ISO 9001:2008**, not the current text; 2015 replaced it with a principles-based objectivity-and-impartiality outcome, which is what makes a documented mitigation arguable for a small organisation. And a run of internal audits with zero findings is read as evidence of a weak audit, not a strong system |
| 9.3 Management review | Top management reviews the QMS at planned intervals; the 9.3.2 input list is enumerated and auditors tick it off; the 9.3.3 outputs must include decisions and actions on improvement, QMS change needs, and resource needs; records required | The *design* exists: [`docs/streams/RETRO.md`](streams/RETRO.md) specifies a cadence retrospective whose inputs are **generated or logged only** — "a retro that reads its own narrative measures the narrator, not the system" — walking board totals delta, streams untouched since the last retro, gate yield, findings age, intake entries still new, open bugs, hygiene and knob tuning, with a one-process-change-max rule. **It reads "No retros yet."** | — | The review itself, and do not let anyone tell you a retrospective *is* a management review. The 9.3.2 input list reaches external-provider performance, resource adequacy and risk-action effectiveness — the three inputs organisations miss most often — and none of them is on the retro's input list. What the retro design gives you is that most of the *other* inputs are already machine-generated, so the writing-down is cheap. Minutes recording discussion but no decisions, owners or dates fail the output requirement |

---

## Clause 10 — Improvement

| Clause | What it asks | Assay artifact (this repo) | Enf. | What the adopter must still supply |
|---|---|---|---|---|
| 10.1 Improvement — general | Determine and select improvement opportunities and implement the actions | The FINDINGS-to-brief pipeline; the INTAKE disposition `scoped → <stream>`; [`docs/mistake-proofing.md`](mistake-proofing.md) §5 adoption ladder, an explicit value-per-cost ordering for adopting the discipline; the one-change-max retro rule | **E** (findings exclusion forces the loop) | Nothing material — the improvement machinery is a first-class part of the methodology rather than an afterthought |
| 10.2 Nonconformity and corrective action | React and correct; **evaluate the need to eliminate the cause** including whether similar nonconformities exist or could occur elsewhere; implement; **review the effectiveness of the action taken**; update risks; retain records of the nature of the nonconformity, the actions, and **the results** | React and correct: a FINDINGS entry plus the Next-up exclusion of affected briefs. Cause and extent: [`docs/brief-rules.md`](brief-rules.md)'s class-sweep discipline and the retro's gate-yield input (recurring classes graduate to standing rules or findings) are the "does it exist elsewhere" half. Effectiveness exists in two partial forms: rule 16 — a brief that adds a *check* must carry a mutation-test row proving the check goes **RED**, so where the corrective action is "add a control", effectiveness *is* demonstrated; and `findingcontrol.go`, which surfaces a recurring-class finding that has landed no permanent control | **E** (rule 16 row presence, findings exclusion), **A** (`findingcontrol` is a NOTICE) | **`resolved: yes` records that the action was TAKEN, not that it WORKED.** The findings entry schema carries no effectiveness field, no date and no runner, and nothing re-establishes that the original failure mode can no longer occur. `findingcontrol` gets closest and is advisory, scoped to recurring-class findings only, and checks that a control *landed* rather than that it *fires*. Two evidence points carry the weight at an audit: a root cause that is not a restatement of the symptom, and an effectiveness verification at a later date. Note also that "CAPA" is not ISO 9001 vocabulary — 9001:2015 says corrective action and deleted the preventive-action clause outright |
| 10.3 Continual improvement | Continually improve suitability, adequacy and effectiveness, informed by analysis and management-review outputs | The adoption ladder, the one-change-max cadence rule, `qualgen/` as the measurement substrate | — | This depends entirely on 9.3 running. Without it, the improvement loop is driven by findings and direction rather than by evaluated performance data |

---

## 7.1.5 — the positive-control discipline as the calibration analog

**Does a tool like this land in 7.1.5 at all?** The test is narrower than people assume: the
clause bites where monitoring or measuring is used **to verify the conformity of products
and services to requirements**. A tool that merely helps people work is infrastructure
(7.1.3). A tool that produces the *verdict* on whether output conforms is in scope. For a
gate that can refuse a merge or a release, the honest answer is usually "in scope", and an
adopter should assume so rather than argue otherwise.

**7.1.5.2 measurement traceability is normally not applicable.** There is no metrological
chain for a software check. The substitute practitioners reach for is a fixed known-answer
corpus with published expected verdicts, re-run per version, playing the role calibration
plays for a gauge. **That analogy is practitioner convention, not standard text** — say so
when you use it.

**What this repo actually does, and what it emits.** The corpus argued its way to the same
requirement from first principles, and then built most of it:

| Layer | Artifact | What it establishes |
|---|---|---|
| Doctrine | [`docs/three-state-instrument-rule.md`](three-state-instrument-rule.md) §"Positive-control requirement" | Break the guarded thing, run the instrument, confirm it reports `checked-failed`. "A green table that was never mutation-tested is the very defect this rule closes." |
| Doctrine, generalised | [`docs/mistake-proofing.md`](mistake-proofing.md) **D1** | Every control-mode device MUST have a demonstration that an injected instance of the error it claims to stop reddens it. "When did this last stop something?" is a legitimate audit question, and "never, and we cannot make it" is a finding |
| Authoring obligation | [`docs/brief-rules.md`](brief-rules.md) rules 16 and 17 | A brief adding a check carries a mutation-test row; a brief touching a shared code path carries a neighbour row |
| Machinery | [`tools/desk/cmd/muhar/`](../tools/desk/cmd/muhar/), driven by six `mutations.json` specs | A spec-driven mutation harness that disarms each refusal path in turn and requires the suite to catch it |
| Gate | [`.github/workflows/release.yml`](../.github/workflows/release.yml), the `test` job | Six mutation steps, each asserting `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate` and failing the release on a survivor with "the gate is not load-bearing as written" |
| Paired control | `statusgen/shardcheck.go` and its fixtures | A check that only ever refused would be a constant, not a detector; the pair is what distinguishes the two |
| Coverage audit | `statusgen --lint-audit` (`lintaudit.go`) | Which lint rules actually fired over a recent window, and which are COLD |

**What an adopter can cite today.** That a control-mode gate in this toolchain is
demonstrated to fire is a *release-blocking* condition, not a convention — the release does
not exist if a mutation survives. That is a stronger answer to "how do you know your
automated quality gate works?" than a passing unit-test suite, because a green suite proves
the code behaves as written and proves nothing about whether the refusal is load-bearing.

**What an adopter cannot cite today, stated plainly.** Four things stand between this and
calibration evidence, and all four are known:

1. **There is no shippable record.** The demonstration runs in CI and is thrown away. "Show
   me the calibration record for check X" is log archaeology. Emitting it is a reporting
   job, not a build.
2. **There is no unified instrument register.** The four-column table
   ([`docs/three-state-instrument-rule.md`](three-state-instrument-rule.md) §"Auditing a
   fleet against this rule": Instrument / what it prints when it cannot see / States /
   Disposition) is *specified* and has no maintained instance. The regimes above cover
   disjoint populations, and firing count is **usage**, not calibration — a rule can fire
   often and still be blind to the case it was built for.
3. **There is no interval.** A brief's rule-16 mutation row runs once at authoring and is
   never re-run. 7.1.5 asks for maintained fitness, not initial fitness.
4. **There is no action-on-prior-results rule.** Nothing says what happens to work already
   verified by an instrument later found blind — no re-verification trigger, no back-out.

Items 1 and 2 are being worked as [`docs/streams/iso-9001/`](streams/iso-9001/) brief 01;
items 3 and 4 are named there and are not yet scoped.

---

## 8.4 / 8.6 — supply chain and release control

**8.4 — the toolchain as an external provider.** The integrity half is strong and machine-checked:

- `.assay-versions` pins `<artifact> <tag> <sha256>` per platform, harvested from the
  *published* release's `checksums.txt` and never from a local build — a pinned sha256 is
  the one thing a re-tagged release cannot silently swap out. See
  [`docs/distribution.md`](distribution.md) and the worked
  [`examples/adopter-scaffold/.assay-versions`](../examples/adopter-scaffold/.assay-versions).
- `deskpins --check` validates the pin file; `deskversion` reports three-state
  (`known` / `known-inconsistent` / `could-not-determine`) and never assumes latest; a
  missing or malformed pin fails closed.
- [`plugins/assay/paired-versions.yaml`](../plugins/assay/paired-versions.yaml) pins the
  plugin-to-binary compatibility pair with per-platform tag and sha256 lines and never a
  floating tag — and platforms with no harvested hash are **deliberately absent rather than
  pinned to an invented one**, with the install path refusing when the line for a detected
  platform is missing.
- The Go toolchain itself is sha256-verified against a pinned digest **before** it is
  unpacked into a PATH-prepended directory in the release build.
- Release assets are immutable: a same-named asset already present is a hard error, never a
  delete-and-reupload, so the release token cannot swap bytes a consumer has pinned.

**What 8.4 asks for that none of that provides.** Integrity control is not *performance*
control. There is no supplier evaluation record, no selection criteria, no performance data
and no re-evaluation date for the toolchain — and the clause is explicit that criteria for
selection, evaluation, monitoring and **re-evaluation** are what it wants, with records.
Re-evaluation never happening is the most common 8.4 finding. For open-source tooling with
no legal counterparty the defensible line is to evaluate *the artefact and its project* —
release discipline, issue tracker, test evidence, maintenance activity — rather than a
company, and to write that reasoning down. Auditors accept it when it is written down; the
finding gets written when the tool is simply in use with nothing in the file.

**And the sharpest one: the model provider is an external provider too.** The entity that
performs the actual work — the model vendor and the agent harness — appears nowhere in this
repo as a supplier. There are no evaluation criteria, no performance monitoring, and no
re-evaluation trigger for a model deprecation, a silent behaviour change or a tier
retirement. The *controls* are in fact good (tier assignment, workspace isolation,
independent re-verification by a different identity, three-state reporting); they are simply
not written down as supplier controls, which is the whole of the clause. This is the adopter's
own supplier file to write, and it is the gap an auditor finds in minutes.

**8.6 — release.** The planned-arrangements chain is real:
[`.github/workflows/release.yml`](../.github/workflows/release.yml) runs `resolve → guard →
test → release`, where `guard` refuses to move or re-publish an existing tag (consumers pin
by tag and sha256, so a moved tag is a silent substitution), `test` carries the gate bar —
golden corpus both directions, the path-rule differential both directions, fleet-workflow
acceptance, and the six mutation steps above — and `release` builds per-platform binaries
with the version stamped via `-ldflags` and emits one `checksums.txt` over every asset. A
dry run builds and checksums and stops before tagging. The `release` job runs in a GitHub
`release` **environment**, so with a required reviewer configured a canonical release pauses
for a named human to approve — which converts "any holder of the shared token can cut a
release" into "a person clicked approve".

**What 8.6 asks for that is not provided, stated plainly:**

- **Traceability to the person authorizing the release is not recorded in the release.** On
  the dispatch path the actor's login is interpolated into the annotated tag *message*
  ("dispatched by …"), which is durable but is not visible in the release notes and is not
  written to any composition manifest; on the tag-push path there is no actor record at all.
  The environment approval likewise lives in the Actions run, not in the artifact. An
  auditor asking "who authorized this release" gets an answer only from run history.
  This is [`docs/streams/iso-9001/`](streams/iso-9001/) brief 04.
- **The evidence set is not assembled at release time.** `statusgen --export-evidence` is
  date-ranged, not release-ranged; there is no "evidence pack for umbrella vX.Y.Z".
- **Integrity is sha256-only.** There is no provenance attestation and no signing anywhere in
  the pipeline. A pinned sha256 stops a silent re-tag, which is the threat the pin contract
  claims to address; it does not establish *who built the artifact*. Anyone reading this page
  as a supply-chain assurance should stop at that sentence.
- **Release notes are not classified by behavioural impact.** An adopter's revalidation
  trigger is a change to the software, its configuration, its intended use or the underlying
  risk. Undifferentiated notes force either full revalidation every release or an
  undocumented judgement call.

---

## What Assay does not claim, and what the adopter must supply

**Assay makes no compliance claim of any kind.** It is not an audit opinion, not a
certification, not a conformance assessment, and adopting it does not confer or contribute
to certification of your management system. The framing in
[`docs/evidence-bundle.md`](evidence-bundle.md) is the one to keep: what the tooling
evidences is the **recorded** process, derived from authored artifacts; it is an **input to**
a compliance review, not a compliance artifact in itself.

Two things worth being explicit about, because they are load-bearing and easy to get wrong:

- **A vendor's own certificate would not validate this tool for your intended use.** It would
  be one input to your supplier evaluation and nothing more. This repo holds no certificate
  and claims none.
- **Validation is intended-use-specific and is therefore inherently your act.** Nobody can
  perform it for you. What a tool can do is make it cheap — a known-answer corpus you run in
  *your* environment produces a dated record on your side, in your name, which is what the
  clause asks you to retain. Where this repo ships such fixtures, that is what they are for.

**The claims discipline here is prose, not machine-checked.** Elsewhere in this corpus the
pattern is to make a claim boundary a fail-closed check rather than an editorial intention.
No such manifest ships in this repository today, so the disclaimers on this page are held by
review, not by a gate. Read that as a limitation of this page, and treat any restatement of
it downstream accordingly.

**What the adopter supplies, in full:**

| The adopter's own | Clause |
|---|---|
| The quality policy — approved, versioned, communicated, available to interested parties | 5.2 |
| Quality objectives with targets, owners, plans and a monitoring cadence | 6.2 |
| Context of the organization and the interested-parties requirements, with a revision record | 4.1, 4.2 |
| The documented QMS scope, with any exclusion justified | 4.3 |
| Organisational roles, responsibilities and authorities — who owns document control and nonconformity handling, by name | 5.3 |
| The organisation-level risk register and the evaluation of risk-action effectiveness | 6.1 |
| Competence criteria per role, records against them, and evaluation that competence actions worked — **for the humans**; a model tier is not a person's competence record | 7.2 |
| Awareness of *your* policy and objectives (evidenced by interview, not by documents) | 7.3 |
| The communication plan in what/when/whom/how/who shape | 7.4 |
| **Retention periods and disposition rules** for every record class — nothing here states one | 7.5.3 |
| The supplier file: selection criteria, evaluation records, performance data, re-evaluation dates — for the toolchain **and for the model provider** | 8.4 |
| Statutory and regulatory determination for your product | 8.2.2 |
| **Validation** as an activity distinguishable from review and verification | 8.3.4 |
| Customer communication, complaint handling, and the customer-notification path for a nonconformity found after delivery | 8.2.1, 8.5.5, 8.7 |
| Customer satisfaction monitoring, from more than one channel | 9.1.2 |
| The analysis-and-evaluation record: what the numbers were read to mean, on what date, and what was decided | 9.1.3 |
| The internal audit **programme** — frequency, methods, responsibilities, reporting, auditor objectivity — and an index of audits performed | 9.2 |
| The management review, at planned intervals, walking the full 9.3.2 input list and recording 9.3.3 decisions, owners and dates. A retrospective is not a management review | 9.3 |
| Corrective-action **effectiveness** verification at a later date, and a root cause that is not a restatement of the symptom | 10.2 |

---

## Related pages

- [`docs/evidence-bundle.md`](evidence-bundle.md) — the export format and the SOC2 CC8.1
  mapping this page is modelled on, including its "what the bundle does not claim" list.
- [`docs/mistake-proofing.md`](mistake-proofing.md) — the device discipline (D1–D7, B1–B10).
- [`docs/three-state-instrument-rule.md`](three-state-instrument-rule.md) — the
  positive-control requirement and the instrument-register table shape.
- [`docs/how-assay-works.md`](how-assay-works.md) — the four claims and the desk loop.
- [`docs/lifecycle.md`](lifecycle.md) — the lifecycle, the single-writer board, and the
  honest claim about what the board is derived from.
- [`docs/streams/iso-9001/`](streams/iso-9001/) — the work this page's gaps became.
