# Cockpit integration — proposed consumer views

These schematic wireframes accompany the [draft spec](spec.md). They describe user
behavior and information hierarchy, not a replacement cockpit shell or implemented UI.
Every example value is synthetic. Preserve an adopter's existing navigation, selection,
deep links and command authorization. The same Operations projection serves the CLI.

## 1. Operations overview

```text
+---------------------------------------------------------------------+
| Existing cockpit shell            Scope: example project   As of ... |
| Streams | Operations | ...                                          |
+---------------------------------------------------------------------+
| Journeys 3       Workflows 8        Assessments due 2    Data gaps 1   |
|                                                                     |
| Needs attention                 Owner          Evidence      Next    |
| Account setup: completion       Product        current       Assess  |
| Change review: queue time       Delivery       missing       Inspect |
|                                                                     |
| [Journeys] [Workflows] [Practices] [Measurements]                      |
| Health: 7/8 sources current. Unknown coverage is shown, not green.    |
+---------------------------------------------------------------------+
```

An operator can open the due assessment, evidence gap or owning subject directly.
Counts share one projection watermark and visibility scope. An unavailable source
does not become zero due work. Empty installation offers the documented manual setup;
an inaccessible scope offers an explanation, not data from another scope.

## 2. Journey view

```text
+---------------------------------------------------------------------+
| Operations / Journeys / Complete account setup       Owner: Product |
| Goal: user reaches a usable account                                  |
| Discover --> Begin --> Provide details --> Confirm --> First use     |
|                         ! friction                                  |
+---------------------------------------------------------------------+
| Success: completion rate [definition v2]      Coverage: partial      |
| Supporting workflows: identity review | welcome delivery             |
| Changes: Brief A [done; observation pending]                          |
| Practices: weekly experience review                                  |
| [Evidence] [Related briefs] [Assessments] [Propose definition change]  |
+---------------------------------------------------------------------+
```

The user sees the journey goal and stages before internal task counts. Many-to-many
workflow links preserve context. Rename or stream archival leaves the deep-linked
identity and assessment obligations intact. Definition changes enter normal review.

## 3. Workflow view

```text
+---------------------------------------------------------------------+
| Workflow / Identity review v3                 Owner: Operations      |
| Supports: Complete account setup                                    |
| Receive --> Queue --> Review --> Resolve                             |
+---------------------------------------------------------------------+
| Lead time: observed     Service time: unknown      Rework: observed   |
| Measurement window / cohort / units / definition revisions           |
| Change history: v2 --> v3             Baseline comparability: check   |
| Context lens: Cynefin v1 (optional)                                  |
| Assessment: contextual judgment / assessor / date / rationale         |
| Practice: case review    Workflow pattern: workflow-pattern-v1 ref   |
+---------------------------------------------------------------------+
```

Service/wait splits require actual evidence; elapsed time cannot stand in for both.
The lens explains a judgment and its provenance. It neither silently reclassifies the
workflow nor grants a different permission set.

## 4. Expected impact in the brief room

```text
+---------------------------------------------------------------------+
| Existing brief room / Brief A                Delivery: verified      |
| Context | Work | Verification | Expected impact                      |
+---------------------------------------------------------------------+
| Affects: Account setup journey / Identity review workflow v3         |
| Disposition: predicted                                              |
| Hypothesis: clearer evidence requests reduce repeated submissions    |
| Metric: repeat-submission rate v1      Baseline: recorded / link     |
| Expectation: lower, range and window fixed in reviewed declaration   |
| Guardrail: review errors must remain within the agreed bound         |
| Evaluator: Product review      Trigger: exposed cohort window closes |
| Pinned declaration revision: 2       Outcome: awaiting observation    |
+---------------------------------------------------------------------+
```

Editing intent creates a reviewed revision; it cannot overwrite the pinned prediction
behind an assessment. Enabling, learning and no-direct-impact dispositions have honest
forms. Old briefs show unrecorded rather than retrospective predictions. Delivery
verification and later outcome have separate labels.

## 5. Outcome assessment

```text
+---------------------------------------------------------------------+
| Assessment / Release group R             Independent evaluator: ... |
| Intent --> Delivery --> Exposure --> Observation --> Assessment      |
| pinned     verified    confirmed    partial         inconclusive     |
+---------------------------------------------------------------------+
| Expected vs observed     Guardrails       Coverage / confounders     |
| Briefs A + B share exposure: individual causal credit unsupported     |
| Evidence: exact artifact, cohort, window and metric revision links   |
| Decision: [Continue] [Investigate] [Propose change] [Escalate]         |
| Next owner / next review condition / rationale                       |
+---------------------------------------------------------------------+
```

Each proposed decision records its evidence and routes through authorized commands or
reviewed records. Missing capability offers an export/manual path. Wrong-version joins
are visible errors. Late evidence supersedes an assessment without rewriting the
decision previously made under it. No button implies deployment or rollback authority.

## 6. Practice and cadence

```text
+---------------------------------------------------------------------+
| Practice / Experience review                  Host: existing desk    |
| Scope: Account setup    Profile: Cynefin v1    Mode: manual           |
| Trigger / window / deadline / owner / escalation / budget            |
+---------------------------------------------------------------------+
| Occurrence 2026-W40   Due       Missing source: receipt feed          |
| Occurrence 2026-W39   Assessed  Decision: continue                     |
|                                                                     |
| Automation capability: unavailable / prerequisites not accepted      |
| [Review occurrence] [Inspect gaps] [Propose cadence change]           |
+---------------------------------------------------------------------+
```

Manual mode works before cadence automation. An enabled view uses the existing
instance/claim/recovery boundary; refresh or double-click cannot create duplicate
occurrences. Show missed windows, cost consumed, pause/cancellation and next owner.
Changing cadence is an operational-policy change, not a display preference.

## 7. Method comparison and switch preview

```text
+---------------------------------------------------------------------+
| Compare interpretations                    Same evidence snapshot S |
| Current: Cynefin v1           Candidate: flow-improvement v1          |
+---------------------------------------------------------------------+
| Shared inputs: arrivals, completion, rework, exposure, evidence health|
| Candidate gap: service-time coverage unavailable                     |
| Current judgment: context ... Candidate: bottleneck inconclusive     |
| New collection required: estimate scope, sensitivity and cost        |
| Prior assessments remain pinned to their original profile            |
| [Replay] [Review differences] [Propose scoped profile binding]        |
+---------------------------------------------------------------------+
```

Changing the display lens is immediate read-only exploration. Adopting a profile for
future decisions is a reviewed binding with effective date and rollback; any collection,
permission or cadence change is separately explicit. A missing input cannot be filled
with a guessed value to make the switch look cheap.

## Consumer delivery slices

| Slice | Dependencies | Owning seam | Acceptance |
|---|---|---|---|
| Subject, impact and outcome rooms (views 1–5) | O07 public projection; adopter's existing typed projection and room infrastructure | Extend existing client adapters/routes | Same source facts as CLI; deep links survive rename/archive; delivery/outcome separate; stale, missing, empty and denied states exercised |
| Practice and profile views (6–7) | O09 comparison; O11 for automation-enabled controls; manual view can follow O10 | Existing command/capability boundaries | One logical occurrence after double submit/restart; backend denies unauthorized action even if UI is bypassed; profile switch preserves facts and history |

The client owner must author these into its own runnable briefs and refresh capability
contracts before implementation. Public Assay owns the portable projection and examples;
the consumer owns its shell, accessibility, navigation and interaction tests. Validation
must include narrow layouts, keyboard traversal, understandable status text without
color, and screen-reader relationships between metrics and evidence gaps.

No client implementation is delivered by this planning PR. A missing cockpit does not
block the manual adoption milestone in the public stream.
