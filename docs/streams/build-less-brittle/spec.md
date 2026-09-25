# build-less-brittle: scoping document

**Status:** routed — authored 2026-09-24 from a read-only review. Approval is the driver's own act: the merge of the pull request that lands this document stamps it (`spec/lifecycle-v1.md` §8.4). That same pull request lands briefs 01–13, whose `sources:` cite this path, so the document lands `routed`, not `approved` (§8.2). D-A and D-B are proposed in §10 and ratified by the same merge.
**Routes-to:** docs/streams/build-less-brittle/
**Owner:** the methodology track (intake, worker, review and authoring procedure).
**Pinned reads:** `origin/main` @ `f7bde6bfa` (2026-09-24 16:32 -0500).

---

## 1. The problem

This stream changes the **processes** that decide what code gets written: intake, brief
authoring, the worker and review. It changes no desk tool's behaviour. The machinery is
redesigned elsewhere. This stream targets the habits that produced it, so they don't
reproduce it inside whatever replaces it.

### 1.1 What the evidence shows

A 2026-09-24 review of the desk tooling measured the following:

| Measure | Value | How taken |
|---|---|---|
| Installed desk verbs | 63 | `ls` of the installed bin directory |
| Production Go under `tools/desk` | ~161k lines | non-test `.go`, `cmd` + `internal` |
| Occurrences of `refus` in production Go | 5,485 | `grep -o` (these are **occurrences**, not refusal sites; see §1.2) |
| Refusal-constructor call sites | ~845 | `Refused(` / `RefusedWithCause(` / `RefusedFinding(` calls, non-test, at `f7bde6bfa` (this document's own count) |
| Five desk-role skill bodies | 3,569 lines | `wc -l` at `f7bde6bfa` |
| Desk-machinery issues opened or closed in 7 days | 360 (284 open) | read on 2026-09-24, one primary class per issue |
| Chains in which one fix exposed the next bug | 9, hand-traced | e.g. #1145 → #1274 → #1631 → #1650 |

The chains share one shape. The fix is applied where the symptom surfaced, usually as another
layer (an override, a classifier, a trailer, a guard) inside the component that misfired. In
the credential chain, #1145 (a child process could not authenticate) led to #1274 (a shim
exports the ambient token), then to #1631 (a verb reads that token as an operator override),
then to #1650 (a scoped wrapper plus an identity probe). That is two new layers, and neither
removes the shim that started the chain.

### 1.2 What the evidence does not show (binding on this stream)

An adversarial audit of that review checked it. The audit's limits bind every
claim in this stream:

- **The counts recompute; the causal certainty does not.** All 360 rows and the class totals
  add up. Several arrows in the chains go further than their source issues support. One
  report shows only a disagreement between two components, not that manual status caused it.
  One peer report was never reproduced. So this stream calls the chains **hand-traced**. Its
  baseline (§5.4) re-grades every arrow into one of four evidence levels:
  *same symptom*, *shared mechanism*, *introduced by commit*, *unconfirmed*.
- **Occurrences are not sites, and sites are not waste.** The 5,485 figure counts a word.
  This stream's counter (brief 03) counts constructor call sites instead. It treats every
  weight number as a **proxy**, never as proof of waste. Some refusals are correct.
- **Issues are not independent incidents.** One failure can be filed in two repositories.
  A decision issue is not a defect, and a request for a new gate is not waste. Class counting
  (brief 04) therefore records a **kind** and an **incident group** for each symptom. Only
  confirmed defects and false positives count toward a design trigger.
- **One ranking was wrong.** Human-only gates had 35 open issues and credentials 33, so
  credentials were not the largest open class.
- **The time window was not reproducible** from the delivered table. The baseline re-reads
  the issues with an explicit window, timezone and collection timestamp.

**What this stream therefore claims.** The recurring friction is real and measured. The
point-fix pattern is visible in the traced chains. The five dynamics in §2 are this stream's
**hypothesis** about why. The stream is a bet with a baseline, a re-measurement and a stop
rule (§5), not a proven cure.

## 2. The dynamics, and the change aimed at each

| # | Dynamic (hypothesis) | Change that targets it | Brief |
|---|---|---|---|
| D1 | The unit of work is the symptom. Intake files "X refused Y", and a worker fixes exactly that. | Intake files by error **class**. Symptoms attach to a class issue, and a class that recurs gets a **design brief**, not another point fix. | 04 |
| D2 | Reviewers check that the diff is correct, not whether it is the right layer or should exist. | A **design-fit stage** runs before correctness review for any change that adds weight. "Wrong layer" is a `design-fit` finding: advisory at landing, blocking after a later recorded promotion (D-A). | 06 |
| D3 | Additions feel safe and deletions feel risky. Fail-first and mutation tests pin every patch in place. | A **weight ratchet**, a `retires:` line in every brief, a defect-class clause that prefers removing the hazard over adding a guard, and retired tests that go with their retired refusal. | 02, 03, 05 |
| D4 | Nobody owns the design. No contract exists for briefs to fit, so logic gets duplicated (readiness is computed seven ways). | A **semantic-owner index** (one owner per meaning) that briefs must cite, plus a **rule register** with a monthly rule diet. | 01, 07 |
| D5 | Cheap models on repeated fix rounds leave residue. | Round-two work and design work run at **strong tier**. Mechanical fixes stay cheap, and only after the design-fit stage has passed. | 04, 05, 06 (folded; see §3) |
| D6 | Nobody knows where the next fix will land, so attention is spread evenly over 161k lines while 2% of files take 28% of the changes. | A **hotspot ranking** from git history nominates, class history confirms, a `brittle` mark binds to an **investigation** of intent versus what happened, and three **fitness functions** hold the shape rules in CI. | 08, 09, 10 (SOTA amendment) |
| D7 | A redesign is coded against recollection. The fix's test leaves silently (five deletions in a week, none explained), so the incident's pin is gone, and the brief that recorded the intent no longer matches the code (about 28 briefs marked done or implemented with no deliverable in the tree), so nobody can say what a refactor must preserve. | Regression tests become a **ratchet** (a tag, a `Retires-test:` trailer, a report review judges). Before any `redesign`, the worker assembles an **oracle**: intent, failure modes mapped to tests, characterization tests triaged intent-versus-accident, invariants, and an acceptance rule. One strong-tier session runs investigation, oracle and refactor itself, asking the driver one typed decision only for the residue. | 11, 12, 13 (third-pass amendment, 2026-09-24) |

## 3. The proposed changes: kept, refined, cut, added

| Proposal | Verdict | What changed, and why |
|---|---|---|
| 1. Intake files by error class; ~3 instances trigger a design brief | **Kept, refined** | The class is a **mechanism**, not the review's 16 broad buckets ("tool self-hygiene" is not a design unit). A design brief is triggered by **3 counted instances or a 2nd merged fix** in the class, whichever comes first. The fix counter is proposal 3 at class level, so one record carries both counters. Only `confirmed-defect` and `false-positive` kinds count, deduped by incident group (the audit). Symptoms in a design-owed class are parked with the existing `blocked` status token, not closed. They close when the design PR merges. |
| 2. Brief template gains a required Design fit section | **Kept, refined** | It is a `design-fit:` block in `## Context`, a sibling of the existing `consumers:` and `layering:` lines, not a new top-level section. It is required on every **new** brief. It has five keys: `owner`, `contract`, `retires`, `weight`, `why-add`. Requiring it everywhere avoids a trigger classifier (the diff-classifier chain is a warning). `n/a` is a legal value when the weight delta is zero. There is no lint yet: the grammar settles first, the same precedent as `consumers:`. |
| 3. Worker two-strikes, plus net lines, flags and refusals in PR bodies | **Kept, refined** | "Same area" means **the same class issue**, not a file-path heuristic. On strike two the worker delivers a design note instead of a fix. There is one bleed exception (driver decision D-B in §10). The `## Weight` PR section is the counter's output at merge-base and head, not a hand count. The existing defect-class clause (worker kit 14, "add a class guard") is amended: remove the hazard or make it unrepresentable first, and treat a new guard as weight. |
| 4. Design-review lane on the strongest model; "wrong layer" blocks | **Refined: a stage, not a lane** | A separate lane needs new selection code (lanes are code-keyed, `deskkit.LanesFor`) and doubles dispatch. Instead, the correctness reviewer answers the design-fit questions **first**. It is dispatched at strong tier when the weight delta is above zero. At landing a design-fit "no" is recorded and the correctness pass continues. Once the class is promoted to blocking (D-A, §10), the reviewer stops before the correctness pass when design-fit blocks. The block needs a scope basis, so a fifth basis, `design-fit`, is added to the machine-checked review-scope table. The new finding class enters the existing reversal-rate calibration. If reviewers mostly get overruled, it demotes to advisory by the rule that already exists. |
| 5. CI weight ratchet, standalone now, replaceable later | **Kept, refined** | It is a **Go test package** in `tools/desk`, not a verb and not a workflow. CI already runs `go test ./...` there, and a workflow change would need a scope the worker App lacks. It follows the precedent of the forge-CLI ratchet (`forgeban`). It ratchets verbs, flag registrations, refusal-constructor calls and rule-text lines. Production Go lines are reported, not ratcheted (the audit: lines are a secondary proxy). Workaround memories are **not** CI-countable: they are private, per-machine files, so the project layer measures them outside the tree (§5.4). |
| 6. One living contracts document that briefs must cite | **Kept, merged with 7** | `docs/contracts.md` already exists for *seam* contracts (artifact + source gate + consumer run). A **semantic-owner index** goes into it, with one row per meaning. Each row names its owner module, its decision record, its known duplicate implementations and its enforcement points. A row is not called a "contract" until it has the doc's three parts, which keeps the existing definition honest. Changing a row means amending its decision record, never adding a local exception. |
| 7. Rule register and monthly rule diet; zero catches alarms, never auto-deletes | **Kept, scoped** | The register lives in the same doc, and each rule cites the semantic row it enforces. It is seeded with the rules implicated in the nine chains plus every **new** rule. It is not a back-fill of every refusal: 845 rows would be a register nobody reads. The diet reports three states (could-not-check / proven-able-to-fire / zero-without-proof) and files **one** decision issue per month, answered in one typed reply. It never deletes automatically. |
| 8. Model tiering | **Cut as a separate change; folded in** | Most of it already exists. Intake and authoring already require strong tier, `exec-tier` is already derived from three questions, and review tier is risk-keyed. The missing pieces become one line each in 04 (the class decision is strong tier), 05 (round ≥ 2 on a finding class, and every design note, runs strong) and 06 (the design-fit stage runs strong). One new `exec-tier` question in 02: "is this a design brief from a class trigger?" |
| 9. Health metrics | **Kept, extended** | The baseline and the re-measurement (§5.4) take **primary outcomes** (fix-caused-next-bug links, workaround memories, operator relays per merged PR), **secondary weight**, and **counter-metrics**. The counter-metrics catch the stream's own failure modes: design-fit reversal rate and PR time-to-ready (§5). |
| **Added:** skill and kit edits are net ≤ 0 lines | **New constraint** | This stream edits four skills and two kits. Each edit must offset its own additions, by moving incident narrative to findings links or cutting restatements. Otherwise the stream adds the weight it argues against. Each such brief checks it with a post-merge line-count row. |
| **Added:** deletion bundling | **New** | A class design brief may retire several mechanisms in one PR (`retires:` lists them). A small deletion then stops costing a whole brief/review/verify cycle. The existing review exemption for fail-first on non-behavioural diffs already covers pure removals. |
| **Added:** a stop rule | **New** | If the counter-metrics worsen past their bars at re-measurement, the design-fit stage returns to advisory and the ratchet to report-only. That takes one typed reply (§5.4). |
| 10. **Hotspot metric and the `brittle` mark** (SOTA amendment) | **Added** | The driver's proposal: code metrics past a threshold mark a part `brittle`. The literature (§11) says the metric that predicts where the next defect lands is change history, not static complexity, and that hotspots are a ranking, not a threshold. So the mark has two keys: the metric (churn × indentation complexity over 90 days, top 2% by score with ≥ 2 fix commits) **nominates**, and the class defect history (04) **confirms**. Temporal coupling is read, not keyed. The mark lands in `docs/contracts.md` and on the board as a findings entry, using what exists. It is never a CI gate: rankings are not pass/fail and hosted checkouts are shallow. Brief 08. |
| 11. **Brittle investigation** (SOTA amendment) | **Added** | The driver's proposal: a strong-model tier investigates the original intent, the issues found, and how to reconcile the two. Kept, given a template with a closed vocabulary: which of three divergences holds (drifted / intent-changed / intent-right-implementation-wrong) and which of three acts follows (`reconcile` with a deletion bundle, `redesign` via a DR amendment and a design brief that prefers a strangler seam, `accept` by amending the record). `none`/`clear` is a legal outcome, so the investigation is not a patch generator. Every recommendation carries a verifiable end state. Brief 09. |
| 12. **Architectural fitness functions** (SOTA amendment) | **Added** | Executable shape rules in the test suite CI already runs: dependency direction (internal never imports cmd; cmd never imports another cmd), the hub's import allow-list as a frozen-rule ratchet, and one declared implementation per registered meaning (a marker the semantic index checks). A separate desk-tool redesign's rule "no logic in main()" is not here; its intent, that new things compose shared building blocks rather than grow unique logic, is what rule 3 holds. Brief 10. |
| 13. **Regression tests are a ratchet** (third-pass amendment) | **Added** | Fail-first already exists (worker kit §9, review kit §3, the fail-first lane, the `test-evidence` finding class) and proves a test can fail on the day it lands. Nothing holds it afterwards: five test functions left the public tree in one week with no recorded reason, and a rename left a Verify row passing with "no tests to run" (#1306). Every fix's test now carries `// regression: #<N>` (a comment, not a `TestRegression_` prefix: names describe behaviour, a rename is the #1306 defect, one test can pin several incidents, and the prefix's only advantage is the count gate this stream declines, §4.11). A deletion or rename carries `Retires-test: <name> — <why>`. A `go test` report lists every departure without one; review judges under the existing class. **It never blocks by itself: a rubric, not a ruling.** The mutation entry guards the fix at the PR that lands it; the tag guards the test afterwards. Brief 11. |
| 14. **The refactor oracle** (third-pass amendment) | **Added** | The driver's question: coding is cheap; what do we code AGAINST, and are briefs plus incidents enough? They are not: a brief records intent at authoring time and drifts (about 28 briefs marked done or implemented with no deliverable in the tree), an incident records what broke rather than what must hold, and a partially delivered brief is not the code. The literature's answer (Fowler: a harness; Feathers: characterization tests before change; Ousterhout: interfaces and invariants) becomes an oracle assembled before any `redesign`: (1) intent, from the investigation's reconciled reading; (2) failure modes, each mapped to its tagged test; (3) current behaviour, characterization tests generated over the public surface and **triaged** keep / drop / unknown, because generating is cheap and the triage is the judgement; (4) invariants, the contracts row and the fitness functions; (5) an acceptance rule: the refactor lands only when 1–4 hold and the triage table is in the PR body. A redesign's system-scale parity harness is the same idea, a soft link (§6). Brief 12. |
| 15. **Incident-time refactor by an agent, one human residue** (third-pass amendment) | **Added** | 09 and 12 as hand-offs leave the mark to go cold between sessions. One strong-tier session now runs the whole loop on a threshold class, a brittle mark or a two-strikes stop: reads the record (brief, DR, class history, git history mapped to issues, Verify tables and Evidence, mutation specs and tags, findings, research), rules (09), assembles the oracle with generated and triaged tests (12), and drafts the refactor as its own PR after the fix. Human contact is one typed decision, only for the `intent-changed?` residue the record cannot settle, with a recommendation and a default; `drifted` and `implementation-wrong` need no human beyond the merge. The only other contact is the three hard gates that already exist (weakening a control, a consumer-facing interface change, crossing a trust boundary). Verify proves a complete record ends with zero asks and a gapped one with exactly one. This is why 01, 04 and 11 come first: the record must be good enough to research. Brief 13. |

## 4. Design

### 4.1 Intake by error class (brief 04)

- **Class issue.** It carries the label `error-class` and the title `class: <mechanism in one line>`.
  Its body holds a hypothesised root invariant, the owning semantic row (or `unknown`), and an
  instance table. It lives in the repository that owns the mechanism.
- **Attach, don't file.** A new symptom that fits an existing class is attached to it with
  `deskfile attach` (an existing verb, which does not count against the new-issue budget). The
  attachment records `kind` (`confirmed-defect` / `false-positive` / `intended-control` /
  `requested-capability` / `uncertain`), `incident-group` (mirrors share one), and an optional
  `introduced-by: <ref> (<evidence level>)` using §1.2's four levels.
- **Trigger.** The class becomes **design-owed** at 3 counted instances (distinct incident
  groups of kind `confirmed-defect` or `false-positive`) or at its 2nd merged fix, whichever
  comes first. It gets the label `design-owed` and rides the existing un-briefed-issue dispatch
  lane at **strong** tier. Its deliverable is a design brief, not a code fix.
- **Parking.** The symptom's placeholder takes the existing `blocked` status token with a
  pointer to the class issue. The design PR's `Closes` line closes the symptoms.
- **Never parked: production-down or security.** A symptom with production-down or security
  impact is not parked. It stays `todo`, so it reaches the worker, where two-strikes (§4.3)
  applies and the `bleed` reply (D-B) is the only way past strike two. A symptom parked before
  that impact was known is released the same way: the driver's `bleed` reply naming it sets
  its placeholder back to `todo`. The class stays `design-owed` either way.
- **Review recurrence.** The review desk's "recurrence-promotion" (a finding raised three times
  across PRs) currently promotes a guardrail. It is redirected to attach to, or open, a class
  issue. Promotion goes to design, not to a new guard.

### 4.2 Design fit in every new brief (brief 02)

```
design-fit:
  owner: <the one module that owns the meaning this brief touches, or n/a>
  contract: <semantic-owner row id, or none — and why none>
  retires: [<mechanism/refusal/flag/test this brief removes>, ...]   # [] is an answer
  weight: <signed delta per dimension: verbs, flags, refusals, rule-text lines>
  why-add: <required when any weight delta is positive: why the capability cannot
           live in the owner, and what was considered for removal instead>
```

Two rules keep this from becoming a delete-everything bias:

1. **Consolidate meaning, preserve independent enforcement.** A second *owner* of one meaning
   is rejected. A second *enforcement point* at a different trust boundary, failing for a
   different reason, is a legitimate layer under the defense-in-depth rule. It is recorded in
   the rule register, not treated as a duplicate.
2. **Retiring a control at a trust boundary** names the layer that still refuses the same threat
   and carries a Verify row proving it with the retired layer absent. Tests that pinned a
   retired refusal are retired with it. The reviewer checks that the **invariant** is still
   covered at its owner, not that every old test survives.

### 4.3 Worker two-strikes and the weight report (brief 05)

- **Strike detection.** The dispatched item's class issue (brief 04) already records a merged
  fix. For an item with no class issue, the worker checks the tracker for one before coding.
- **On strike two** the worker stops and posts a **design note** on the class issue: the root
  invariant, where it should live (owner), what the prior fixes added that the design would
  retire, and a proposed design-brief title. The item goes back to intake as design-owed. The
  existing reuse-ladder exception ("a briefed deliverable is never re-litigated") gains one
  carve-out: strike two.
- **PR body `## Weight`.** The counter's line at merge-base and at head, plus
  `git diff --shortstat`. It is a material claim, so a wrong Weight line is a review finding
  under the existing material-claim basis.
- **Defect-class clause (kit 14) amended.** It keeps "name the class". "Add a class guard"
  becomes "prefer removing the hazardous path or making it unrepresentable; a new guard is
  weight and must appear in `## Weight` and the rule register". The wording stays identical
  across both implementer kits (held by the existing kit-parity test). That test currently
  **requires the token `ALLOW-LIST`** in the clause, so the add-a-guard obligation is itself
  pinned by a test (D3 in miniature). Brief 05 retires the token with the obligation.

### 4.4 The design-fit review stage (brief 06)

- **When.** The PR's verified weight delta is above zero in any ratcheted dimension, or the PR
  adds a rule-register row. The reviewer re-runs the counter; it does not trust the body.
- **Who.** The correctness reviewer, dispatched at **strong** tier for such PRs, answers first:
  1. *Right layer?* Does the change live in the owner the semantic index names?
  2. *Should it exist?* What would happen if the symptom were fixed by removal instead?
  3. *What does it replace?* Are `retires:` / `why-add:` true and sufficient?
- **Blocking, and the landing state (D-A, proposed; §10).** A "no" is a finding with scope basis
  `design-fit`. It is a fifth row in the machine-checked review-scope table, and it needs a
  concrete reason tied to the semantic index or the ratchet. The class lands **advisory**:
  the finding is recorded and the correctness pass continues. Once promoted to `blocking`
  (§10 D-A), a "no" holds the PR and the reviewer stops before the correctness pass.
- **Self-correction.** `design-fit` joins the finding-class register, `advisory` at landing,
  `blocking` by the promotion decision, and thereafter subject to the existing reversal-rate
  demotion (above 50% reversed for two consecutive months makes it advisory again). That is
  the calibrated guard against a reviewer's taste becoming policy.

### 4.5 The weight ratchet (brief 03)

- `tools/desk/internal/weight/` holds a counter, a committed `ceiling.txt`, and a test.
- **Ratcheted:** verbs (`cmd/*` main packages), flag registrations (syntactic), refusal-
  constructor calls, rule-text lines (the desk-role, authoring and shepherd skills plus the
  dispatch kits). **Reported only:** production Go lines.
- **Mode (D-A, proposed; §10).** `ceiling.txt` carries `# mode: advisory` at landing: growth past a
  ceiling is logged as `GROWTH-NOTICE` and the test passes. In `# mode: blocking` it
  **fails** when any ratcheted count exceeds its ceiling. The promotion is the recorded
  decision in §10. In either mode it **notices** slack when a count is below. Slack
  consumption between tightenings is caught by the review stage's per-PR delta (§4.4), not
  by CI. That gap is stated here, and the monthly diet (§4.6) lowers ceilings.
- **Raising a ceiling** needs the driver's typed reply `grow <PR#>` on a single standing
  decision issue ("weight growth awaiting approval"). The reviewer attaches each growth PR's
  head and delta there. The ceiling line cites the reply's URL, and the review stage checks
  the reply's author. The approved number is written into the ceiling, so further growth at a
  later head goes red again. That binds the reply to the delta it approved, so an edited option cannot reuse an old reply.
- **Could-not-check** (a sibling directory absent in a consumer checkout) skips with a reason
  and never passes silently.

### 4.6 Semantic-owner index, rule register, rule diet (briefs 01, 07)

- **Index** (in `docs/contracts.md`): `S-<slug>` rows for readiness/eligibility, delivery,
  identity and credential resolution, claims, worktree lifecycle, decision acceptance,
  publication scanning, status derivation and exit/refusal codes. Each row has owner, decision
  record, duplicates (with paths) and enforcement points. **Seeded from the tree, dated.**
- **Register:** `R-<slug>` rows: rule (one line), where enforced, serves `S-<slug>`, owner,
  invariant, justifying issue, catch source (a gate-telemetry class or `none`), last reviewed.
  Seeded with the chain-implicated rules. Every new rule adds a row in the same PR.
- **Diet (monthly):** each row gets its three-state catch status. Rows that are
  `zero-without-proof`, high-false-positive or orphaned (serving no `S-` row) are listed in
  **one** decision issue whose options are immutable at filing. The reply, from the driver's
  own login (§4.7), names rows and verdicts, e.g. `retire R-a R-b; keep rest`. Default: keep. Retirement is a design brief,
  never an in-loop edit.

### 4.7 Human gates: one typed reply, no new machinery

| Gate | Where | Reply | Single control / layers behind it |
|---|---|---|---|
| Ratchet increase | one standing decision issue | `grow <PR#>` | the human merge; behind it, the CI ceiling (visibility, forcing the bump) and the review stage checking the reply's author |
| Rule-diet retirements | one decision issue per month | `retire <R-ids>` or `keep` | the retirement design brief's own review and merge |
| Two-strikes bleed exception | the class issue | `bleed` | the class stays design-owed; the bleed fix still needs its own review and merge |
| Stop rule (§5.4) | the re-measurement decision issue | `keep` or `revert` | revert is two table edits, reviewed and merged |
| Promotion to blocking (D-A) | one decision issue, go-live + one month | `promote ratchet` / `promote design-fit` / `promote both` / `hold` | a one-line mode edit and a one-cell register edit, by PR; the reversal-rate demotion behind it |
| Refactor residue (§4.13) | one decision issue per `redesign` whose oracle holds an `unknown` | `residue: default`, or `residue: keep <tests>; drop rest` | the refactor PR stays draft until answered; its design brief's review and the merge |

**A typed reply counts only from the driver's own login.** The project layer names that
login. Whoever acts on a reply reads its author from the forge, never from the reply's text,
the way §4.5 checks `grow`: the review stage for `grow`, the worker for `bleed`, the diet's
role for `retire`, the go-live and close-out briefs for `promote`/`hold` and `keep`/`revert`,
and the incident-refactor run for `residue:`. A reply in the grammar from any other login is
quarantined: it is noted on the issue and never acted on. On a public repository anyone can
comment, so this author check is the single control in front of every gate above. The layer
behind it is review plus the driver's own merge of the PR the reply produces.

The SOTA amendment adds **no gate**. The brittle mark is made by the monthly pass on two
recorded keys; the investigation's `redesign` outcome lands as a design brief through the
existing authoring gate; a hub allow-list growth reuses the `grow <PR#>` reply. The
third-pass briefs (11–13) add the two typed replies above and nothing else: the ledger report (11)
never blocks, the oracle's acceptance rule (12) is rows in a Verify table, and the
agent-run refactor (13) asks once, in the `ask-decision` shape that already exists. Its
hard-gate exceptions are the gates that already exist.

### 4.8 Hotspot metric and the brittle mark (brief 08)

- **Instrument.** `tools/desk/internal/hotspot`, a test package like the weight counter. Over
  a window (default the trailing 90 days) on the first-parent line of `origin/main`, per
  non-test Go file: `churn` (commits), `fixes` (commits whose subject matches `fix`/`revert`,
  a stated proxy), `complexity` (indentation sum at the window end), `score = churn ×
  complexity`, `rank-pct`. Temporal coupling: pairs co-changing in ≥ 5 commits with ratio
  ≥ 0.3 among commits touching ≤ 8 files. A shallow clone is `could-not-check`.
- **The two-key mark.** A module (an `S-` owner path or a `cmd/<verb>` directory) is marked
  `brittle` at the monthly pass when (1) a file of it is in the top N% by score with `fixes ≥
  2` (N is a project value, default 2, and the cut score is pinned by the baseline), and (2)
  an `error-class` issue records ≥ 2 counted instances in it, or a re-graded chain arrow at
  `introduced-by-commit` lands in it. Key 1 alone is `watch`. Key 2 alone is a class matter.
- **Where it lands.** A row in `docs/contracts.md` §Brittle marks (module, S- row, since, the
  four figures, coupling partners, class issue, investigation, cleared) and a findings entry
  `F-brittle-<module>` that the board already renders under "Unresolved findings" and already
  counts in the change-fail proxy. The class issue takes the label `brittle`. A mark clears
  when the investigation's next act has merged and the next pass no longer nominates the
  module; the findings entry flips `resolved: true` in that PR.
- **Effect on the other briefs.** Intake (04): every class instance names its `module:`; a
  class in a marked module is design-owed at its first counted instance. Review (06): a diff
  touching a marked module runs the design-fit stage at any weight delta, and the reviewer
  reads the investigation if one exists.

### 4.9 The brittle investigation (brief 09)

- **Trigger.** A class issue carrying `design-owed` and `brittle`, on the strong-tier
  un-briefed lane 04 already uses. Deliverable: `docs/investigations/<date>-<module>.md`
  from `docs/brittle-investigation-template.md` (planned); a design brief only on `redesign`.
- **Reads.** The originating and last-redesign briefs (`git log --follow --reverse` to the
  first commits, then the PR's `Brief:` trailer), the `DR-` record and `S-` row; the class
  instance table, the window's fix commits, findings whose `affects:` touch the module, chain
  arrows landing in it; and a per-fix history table (what each added, in which layer, inside
  or outside the owner). Coupling partners outside the owner are read as seams.
- **Says.** Exactly one divergence: `drifted`, `intent-changed`, `intent-right,
  implementation-wrong`, or `none`. Exactly one recommendation: `reconcile` (a fix brief whose
  `retires:` is the deletion bundle), `redesign` (a DR amendment and a design brief, strangler
  seam before rewrite), `accept` (amend the record, clear the mark), or `clear`. Plus
  `single-point-of-failure:` on core surfaces and an `end-state:` line the next pass can check.
  An unfindable intent is `NEEDS_CONTEXT`, never invented.

### 4.10 Fitness functions (brief 10)

- `tools/desk/internal/arch`, run by the same `go test ./...`. Rule 1, dependency direction:
  `internal/**` never imports `cmd/**`; `cmd/<x>` never imports `cmd/<y>`; ceiling 0, and at
  the pin the count is 0, so nothing is grandfathered. Rule 2, the hub's allow-list:
  `internal/deskkit` (imported by 66 packages) may import only the `internal/` packages listed
  in `hub-allow.txt` (`gitcore`, `topology` at the pin); red on an unlisted import and on a
  listed import that has gone (lock in the gain); growth cites a `grow <PR#>` reply. Rule 3,
  one implementation per meaning: `// semantic: S-<slug>` markers on the functions that
  compute a registered meaning; the test reads the S- rows and holds every marker to the
  owner path or a listed duplicate, the count to a committed ceiling, and requires the owner
  to be declared. What it cannot see, an undeclared duplicate, is the design-fit stage's
  question 1 at review; the register row says so. Go's own compiler already enforces
  `internal/` boundaries and the absence of import cycles; the package relies on it and
  says so. Clone density is reported when a detector is present, never ratcheted.

### 4.11 Regression tests are a ratchet (brief 11)

- **What exists.** Fail-first is the worker's obligation (kit §9: a red run or a committed
  mutation entry; §14.3: the class guard red against a planted second instance), the
  reviewer's rule (review kit §3), a lane for every unknown or blessed-once author
  (`review-lanes.md`), a `blocking` finding class (`test-evidence`), and brief-rules rule 16
  for Verify tables. All of it proves the test can fail on the day it lands.
- **The tag.** `// regression: #<N>[, #<M>…]` directly above the test (a findings id or a
  class issue are also legal). A comment, not a `TestRegression_` name prefix: names
  describe behaviour, a mass rename is the #1306 defect, one test may pin several incidents,
  `go/parser` reads it like 10's `// semantic:` marker, and the prefix's one advantage (a
  `-run` selector for a count gate) is a gate this stream declines.
- **The trailer.** `Retires-test: <TestName> — <why>`; for a rename, `Retires-test: <Old> —
  renamed <New>; <why>`, so the same PR re-points every Verify row that named the old name.
- **The report.** `tools/desk/internal/testledger`, a test package: over an explicit
  `-base..-head` (never a merge-base default, #1657) it logs `retired-untrailed`,
  `renamed-untrailed` (body hash or tag match) and `verify-rows-naming` lines, or `clean`,
  and skips `could-not-check` on an unresolvable base. It passes whatever it finds. The
  reviewer runs it with `-v` and answers three questions per line: is the behaviour still
  pinned and by which test; did the reason land as a trailer; are the rows re-pointed. An
  unjustified departure is a `test-evidence` finding. **A rubric, not a ruling.**
- **Division of labour.** The mutation entry guards the FIX (revert it and the test is red)
  at the PR that lands it. The tag and trailer guard the TEST across later PRs. Neither
  substitutes for the other. The planned statusgen lint for `go test -run` names that match
  no test catches the effect in Verify rows; this report names the cause in the test tree.

### 4.12 The refactor oracle (brief 12)

- **Sufficiency.** Briefs plus incidents are not enough to refactor against. A brief records
  intent at authoring time and drifts (a 2026-09-24 count: about 28 briefs marked done or
  implemented with no deliverable in the tree; unpublished, and a later status audit is to
  re-derive it). An incident records what broke, not what must hold. A partially delivered
  brief is not the code. Gordon's measured roundtrip (AIEWF 2026; from a note-taker's record of
  the talk, secondary) puts numbers on the drift: ~30% design-code fidelity at baseline, worse over 12
  pure-LLM iterations, ~90% only under deterministic guardrails.
- **The oracle**, one file beside the investigation, `<date>-<module>-oracle.md`:
  1. *Intent* — the owning brief(s) and DR quoted, and the investigation's `divergence:` with
     its reconciled reading of intent versus what the code does now (09's output, never
     re-derived).
  2. *Failure modes* — every class incident and every findings entry touching the module,
     each mapped to its regression test by `git grep 'regression: .*#<N>'` (11). An empty
     test cell is a refusal.
  3. *Current behaviour* — characterization tests generated over the owner package's public
     surface (Feathers: assert what it does), tagged `// characterization: <module>
     <oracle-file>`, then **triaged** in a table: `keep` (intent, with its source), `drop`
     (accident or bug, with the incident or divergence that says so), `unknown` (the record
     does not settle it). Generation is cheap; the table is the judgement and the record.
  4. *Invariants* — the S- row and contracts entry (01), the register rows that serve it
     (07), the arch rules (10), the weight line (03); the owner's `// semantic:` marker stays.
  5. *Acceptance* — the refactor lands only when 1–4 hold: every failure mode and every
     `keep` test passes at head, every `drop` test is gone with a `Retires-test: … accident
     per oracle §3` trailer, no `unknown` remains, the invariants are green, and the PR body
     carries `## Oracle` with the triage table verbatim. Five runnable rows, copied into the
     design brief's Verify table.
- **Scope.** Only `redesign` needs an oracle. The ledger (11) gains one report line for a
  tagged characterization test its oracle does not list.
- **A desk-tool redesign.** A system-scale parity harness (a fixed corpus chosen before the new behaviour, replay
  capsules, cutover briefs verified against Verify tables) is this oracle at system scale.
  Where a replay corpus exists, an oracle MAY cite it in part 3. Soft link only (§6).

### 4.13 Incident-time refactor by an agent, with one human residue (brief 13)

- **Trigger.** A class issue labelled `design-owed` (04), with `brittle` from 08 or a
  `strike two` note from 05: the three triggers are one object on the strong-tier un-briefed
  lane. No new verb, label, flag or reply.
- **The run**, fixed by a runbook (`docs/incident-refactor-run.md` (planned)): (1) read the record —
  brief(s) and DR, the class issue and every prior incident with kind and origin, the
  module's git history with each change mapped to the issue it answered, Verify tables and
  Evidence, mutation maps and regression tags, findings, research notes; a read that cannot
  be made is `could-not-read (<what>)`; (2) rule (09); on `reconcile`, `accept` or `clear`
  stop there; (3) on `redesign`, assemble the oracle (12), generating the characterization
  tests and triaging every row the record settles, and write the DR amendment and the
  design brief in the same authoring PR; (4) draft the refactor as its own PR after the
  incident's fix has landed; (5) if the oracle holds any `unknown`, file one
  `needs-decision` issue in the `ask-decision` shape, every unknown as one question with the
  agent's recommendation and a default, options immutable, one reply line. Zero unknowns,
  zero issues. Never a second ask.
- **Where the human is.** `drifted` and `intent-right, implementation-wrong` need no human
  beyond the merge. `intent-changed` is where the residue lives, asked once. The only other
  contact is the three hard gates that already exist: weakening or removing a control
  (worker kit §2, §4.2 rule 2), a consumer-facing interface change (a seam row or a
  `consumers:` entry outside the module), crossing a trust boundary (author-brief rule 10,
  a changed `single-point-of-failure:` line). Each makes the design brief `gate: human`
  under the existing template rule.
- **Provable.** The residue is a pure function of the oracle file (`Residue`,
  `DecisionBody` in the ledger package), and the brief rehearses the run twice on synthetic
  incidents: a complete record ends in a draft refactor PR with zero asks in the escalation
  vocabulary; a record with one unexplained behaviour ends in exactly one decision issue
  with one question. The rehearsal PRs are closed unmerged and the numbers are Evidence.
- **Why 01, 04 and 11 come first.** An agent can only research a record that exists: an
  owner and a DR to read intent from, incidents filed by class with kind and origin, and
  each incident mapped to the test that pins it. Without them every gap is a human ask and
  the residue grows back into the whole question.

## 5. Measures

### 5.1 Primary outcomes (what "worked" means)

| Id | Measure | Instrument |
|---|---|---|
| M1 | New fix-caused-next-bug links at evidence level *introduced-by-commit* or *shared-mechanism*, per 4 weeks | the `introduced-by:` field on class attachments, plus a hand audit at re-measurement using the baseline's method |
| M2 | Workaround memory notes (per-operator agent memory, outside the tree) | the project layer's own meter, at baseline and close-out |
| M3 | Operator relay messages per merged PR, and per operator-day | the existing operator-metrics collector (`opmetrics`), a stated floor |
| M4 | Share of desk-machinery fix PRs whose weight delta is ≤ 0 | the `## Weight` sections of merged PRs |
| M5 | Brittle marks made, investigations delivered (by recommendation), marks cleared; and the top-10 files' share of commits, both windows | the `docs/contracts.md` mark table, `docs/investigations/`, and the hotspot counter at one head over both windows |
| M6 | Untrailed test departures per 4 weeks (deleted or renamed test functions with no `Retires-test:` trailer); and, per `redesign`, whether it landed with a complete oracle and how many decision issues it filed (0 or 1) | the ledger report over the window (`-base`/`-head` pinned to the window's SHAs); the oracle files and the `needs-decision` issues citing them. Reported, not judged, at the first close-out |

### 5.2 Secondary: weight (proxies, per §1.2)

Verbs, flag registrations, refusal-constructor calls, rule-text lines, production Go lines,
and the project's resident rules-file lines (measured by the project layer). The same counter is applied at the baseline SHA and at the close-out SHA.

### 5.3 Counter-metrics (the stream's own harm)

| Id | Measure | Bar |
|---|---|---|
| C1 | `design-fit` finding reversal rate | ≤ 50% (the existing demotion threshold) |
| C2 | Median PR open-to-ready time, desk-machinery PRs | not above baseline + 25% |
| C3 | Design-owed classes older than 21 days with no design brief | ≤ 2 |
| C4 | Investigation lead time: median days from a brittle mark to its investigation merged | ≤ 21 |

### 5.4 Baseline, re-measurement, stop rule

- A baseline brief in the adopting project's own layer pins the baseline: SHAs, window,
  commands, values. A close-out brief re-measures **no earlier than 28 days** after the
  project's go-live (the brief that sets its project values and re-syncs the skills).
- **Success:** M1 and M3 improve on baseline, M2 and all secondary counts are not above
  baseline except for `grow`-approved deltas, and C1–C4 hold. M5 is reported, not judged, at
  the first close-out: one 28-day window is too short to expect a cleared mark.
- **Stop rule:** if any of C1–C4 fails, one decision issue asks `keep` or `revert`, answered
  from the driver's own login (§4.7). `revert`
  demotes `design-fit` to advisory and switches the ratchet and arch tests to report-only.
  Nothing else is undone; marks and investigations are records and stay.
- **Under D-A** the ratchet and `design-fit` land advisory, so a close-out that precedes the
  promotion decision has nothing to revert on those two; C1 and the month's `GROWTH-NOTICE`
  count are then the promotion decision's inputs (the go-live brief), and `revert` is not
  offered for them. M6 is reported alongside M5.

## 6. Independence from a desk-tool redesign

Nothing in this stream waits on, calls or assumes a redesigned desk tooling. The soft couplings,
all optional:

- The weight counter (03) MAY later be replaced by a redesign's meter, **only** if it reproduces this
  counter's numbers at a shared SHA. Until then this counter is authoritative for the ratchet.
- Semantic-index rows (01) change owner when a redesign lands. That is an ordinary row amendment.
- A redesign's policy-file catch counters MAY become the register's catch source (07), replacing
  gate-telemetry classes row by row.
- A freeze of the current tools, if ruled, is compatible: a freeze is a ceiling this ratchet already enforces.

If a redesign is cancelled, this stream is unaffected. If this stream is cancelled, a redesign loses
nothing it depends on.

## 7. Where it lands

| Home | Briefs | Content |
|---|---|---|
| this repository | `build-less-brittle/01–13` | semantic index + register (docs), brief spec + authoring skill, weight counter (Go test), intake / worker / review skills and dispatch kits (01–07); hotspot counter, investigation template, fitness functions (08–10); the test ledger, the oracle template, the incident-refactor runbook (11–13) |
| each adopting project's own layer | its own briefs | baseline, project values + go-live, the project's own rule diet, re-measurement |

Cross-repo edges are **not** written in `depends:`. A single-root lint reports an edge into
another repository's stream as an unknown stream (statusgen `brieffile.go`, verified at
`f7bde6bfa`). Such edges are `facts:` preconditions with a check command, and the README
shows them.

## 8. Pre-mortem: this stream shipped and made things worse. How?

| Failure mode | Caught by |
|---|---|
| Design-fit becomes reviewer taste and blocks good work | C1 + the existing reversal-rate demotion (06) |
| Class issues become a second ceremony queue (the largest class today is ceremony) | classes are few (mechanism-level), attach is unbudgeted, one standing growth issue, C3 |
| Two-strikes stalls a production-down or security fix | intake never parks such a symptom (§4.1), and the driver's `bleed` reply (D-B, §4.7) lifts the worker's stop |
| The ratchet collides across parallel PRs (the forgeban collision class) | fail on growth only; slack is noticed, not failed; conflicts only on real growth |
| Workers game the counter by moving refusals into unmatched shapes | the review stage re-derives the delta and reads the diff; the counter's definitions are in the register and reviewable |
| Deletions remove an independent security layer | §4.2 rule 2 + the existing security lane + defense-in-depth Verify rows |
| The stream's own skill edits add weight | net ≤ 0 line-count rows on every skill-touching brief |
| The baseline overclaims again | the baseline re-grades every chain arrow and records kind and incident group |
| The hotspot list is correct and ignored (Google's bug predictor, 2013) | every mark binds to an investigation with an owner, a tier and a 21-day bar (C4); the list itself gates nothing |
| A mark becomes a rewrite licence | the investigation's closed vocabulary: `reconcile` and `accept` are outcomes; `redesign` prefers a strangler seam and needs a DR amendment; a rewrite needs the investigation to show the seam cannot be cut |
| The `fix` subject proxy miscounts | it only nominates; key 2 is the class record; the baseline records the proxy's precision on a 20-commit sample |
| Markers are put on wrappers, so rule 3 protects the wrong function | the 10 review question; the design-fit stage checks the declaration against the owner path |
| The `Retires-test:` trailer becomes boilerplate ("cleanup") and the report is scrolled past | the reviewer's three questions per line; an unjustified departure is a `test-evidence` finding; M6 counts untrailed departures, so a rising count is visible at close-out |
| Characterization tests freeze bugs in place, or the triage marks everything `keep` to land | a `keep` needs a source (an S- row, a DR, a brief fact, an incident); a `drop` needs one too; an `unknown` refuses to land; the 12 review question checks a real module's triage, not the worked example |
| The agent-run refactor asks the driver more than once, or hides an ask in prose | the residue is a pure function of the oracle; the rehearsal rows count decision issues (0 and 1) and escalation labels; the 13 review question reads the PR for prose asks |
| The agent-run refactor bypasses a hard gate to keep its ask count at one | the three gates are the existing ones (worker kit §2, seam contracts, rule 10) and are named in the runbook; the reviewer confirms no fourth gate was added and none narrowed |

## 9. Out of scope

Any desk tool's behaviour. Merge authority. A desk-tool redesign's architecture, including its "no logic
in main()" rule. A lint for `design-fit:` (deferred until the grammar settles). A back-fill of
the register to every existing refusal. Automatic deletion of anything. A CI gate on the
hotspot ranking. Ownership and bus-factor analysis (one contributor and a fleet of bots make
the knowledge map degenerate here; revisit if a second human joins). A CI gate on the count
of regression tests or on a vacuous `-run` selector (#1581's items 2 and 3): this stream makes
the ledger a rubric, not a ruling (§4.11), and the selector lint is another stream's. Stub-coverage
rules and a re-fix metric (#1581 items 3–4) stay with their issue. Characterization tests
over a module no investigation has marked for `redesign`.

## 10. Decisions for the driver

- **D-A. Blocking from day one? — PROPOSED: no (ratified by the merge, below).** The weight ratchet (03) and the
  `design-fit` check (06) start **advisory** (report only) for the first month. The promotion
  to blocking is a later, recorded decision keyed to the project's baseline
  measurements (§5.4): one decision issue, options `promote ratchet` /
  `promote design-fit` / `promote both` / `hold`, default `hold`, answered from the driver's
  own login (§4.7), landed by PR as a one-line
  `ceiling.txt` mode edit and a one-cell register edit (§4.4, §4.5, §4.7). The recommended
  alternative (blocking from the merge) was not taken.
- **D-B. Two-strikes: hard stop or fix-plus-note? — PROPOSED: hard stop (ratified by the merge, below).** The
  `bleed` reply on the class issue is allowed only for production-down or security fixes
  (05). The alternative (the fix ships and the note is mandatory) was not taken.
- **D-C. Approve this spec.** Decided by the driver's merge of the pull request that lands
  this document (§8.4 of `spec/lifecycle-v1.md`); the same pull request lands the stream README
  `active` and this header `routed`. No relayed ruling stands in for that merge.

D-A and D-B were relayed to the authoring session on 2026-09-24 as the driver's answers. No
record of either from the driver's own account exists on the forge, and a relayed answer does
not by itself pass a human gate, so this document states both as **proposed**, and the
affected briefs (03, 05, 06) say so. They are ratified the way D-C is: by the driver's own
merge of the pull request that lands this document (§8.4 of `spec/lifecycle-v1.md`). Until
that merge, neither binds. Briefs 11–13 were added in the same pass; they add no decision.

## 11. What state-of-the-art teams do about brittle code (the SOTA amendment)

The findings this stream binds to, with their sources:

| Practice | Evidence | Where it lands here |
|---|---|---|
| Hotspots = change frequency × complexity, with indentation as the complexity proxy; a ranking, not a threshold | Tornhill, *Your Code as a Crime Scene* (2015; 2nd ed. 2024), *Software Design X-Rays* (2018); Hindle, Godfrey & Holt, ICPC 2008 (indentation correlates with McCabe and Halstead); CodeScene's own examples put hotspots at 1–3% of code and 11–45% of changes or fixes | 08: the instrument and the top-N% nomination; here 1.8% of files took 28% of commits |
| Change history beats static complexity for predicting defects; relative churn; faults cluster and decay with age | Nagappan & Ball, ICSE 2005 (89% accuracy on Windows Server 2003); Graves et al., TSE 2000; Kim et al., ICSE 2007 (10% of files hold 73–95% of faults); Rahman & Devanbu, ICSE 2013; Nagappan, Murphy & Basili, ICSE 2008 (org structure beats both) | 08's key 1 is churn-weighted; key 2 is the class defect record; complexity alone marks nothing |
| A correct prediction that is not actionable changes nothing | Lewis et al., ICSE 2013: Google's bug predictor had "no identifiable change in developer behavior" | 09: every mark binds to an investigation with an owner, a tier and a next act; C4 |
| Quality has a measurable business cost, non-linear at the top | Tornhill & Borg, TechDebt 2022 (alert code: 15× defects, 124% longer, 9× max time); Borg et al., TechDebt 2024 | the `brittle` mark is a cost claim, so it is recorded with its figures and cleared with evidence |
| Instability is its own axis: change-fail rate and rework rate; ~70% of outages come from changes; error budgets brake feature work; postmortem action items need an owner and a verifiable end state | DORA metrics guide and 2024/2025 reports (AI adoption: −7.2% stability per +25% adoption in 2024; still negative in 2025); Google SRE book Introduction and ch. 3, 15; SRE workbook ch. 10 | M1 and the change-fail proxy the board already computes; the investigation's `end-state:` line; the two-strikes stop is this stream's error budget |
| Architectural fitness functions: shape rules as executable tests, frozen violations as a ratchet | Ford, Parsons & Kua, *Building Evolutionary Architectures* (2017; 2nd ed. 2022); ArchUnit `FreezingArchRule`; ThoughtWorks Radar (fitness functions Trial 2017, ArchUnit Trial 2018); Go `internal/` boundary; depguard, go-arch-lint | 10: three rules in the test suite CI runs; 03's ratchet is one already |
| Refactor before rewrite; strangler seams; tolerate duplication until the third instance; strategic investment of 10–20% | Fowler, *Refactoring* 2nd ed. (2018), "Strangler Fig Application" (2004/2024), Technical Debt Quadrant (2009); Ousterhout, *A Philosophy of Software Design* (2018); Foote & Yoder, "Big Ball of Mud" (1997); Lehman's laws (1980); Cunningham (1992); Metz, "The Wrong Abstraction" (2016) | 09's three recommendations and their order; 04's 3-instance trigger is the Rule of Three; 02's `retires:` is Cunningham's "paid back promptly" |
| Review design first, then correctness; ask whether the change belongs in the codebase at all; reviews find few defects and mostly improve maintainability; low review participation costs post-release defects | Google eng-practices reviewer guide; Sadowski et al., ICSE-SEIP 2018 (OWNERS + readability, median 24 lines, < 4 h); Bacchelli & Bird, ICSE 2013; McIntosh et al., MSR 2014; Ubl, "Design Docs at Google" (2020) | 06's stage order and its question 2; 01's owner index is the OWNERS file for meanings |
| Agent-written code: velocity up, complexity and warnings up and persistent; churn and copy-paste up (vendor data); passing tests overstate correctness | He et al., MSR 2026 (difference-in-differences, 807 repos: complexity +41.6%, warnings +30.3%, no duplication effect); GitClear 2024/2025 (vendor, correlational); DORA 2024/2025; METR 2025 RCT (−19%); Wang et al., ICSE 2026 (SWE-bench: 7.8% of "solved" fail the developer suite); Smith et al., FSE 2015 (overfitting patches) | 05's two strikes, 06's design-first stage and 10's rules are the counters; evidence that design-doc-first reduces agent churn is practice guidance, not a result, and the spec says so |
| Refactoring needs a test harness; for code you did not write, characterization tests capture current behaviour before you change it; what they must preserve is the interface and its invariants; spec-first for agent code is guidance, and generated tests must be triaged because a passing suite overstates correctness | Fowler, *Refactoring* 2nd ed. (2018), "self-testing code" as the precondition and the Rule of Three; Feathers, *Working Effectively with Legacy Code* (2004): "legacy code is code without tests", characterization tests, the legacy code change algorithm; Ousterhout, *A Philosophy of Software Design* (2018): interfaces, invariants, deep modules; GitHub Spec Kit, Kiro, Anthropic guidance (recommendations); Wang et al., ICSE 2026; the AIEWF 2026 roundtrip measurement (Gordon; secondary) | 12's four parts and acceptance rule; 11 maps each failure mode to a tagged test; 13 makes the triage the agent's and the residue the driver's one decision |

**From conference talks** (secondary: a note-taker's record, not revalidated against the
recordings, per the audit's evidence discipline): Roberts (Nearform, DevCon London) prescribes a forensic
hotspot sweep before any agent writes code and "slices, not rewrites"; Böckeler (Thoughtworks)
a weekly drift pass with import and dependency rules, triaged like security findings, and
the smell test of a routine change touching 41 files; Thomas (Meta) hotspot-ranked test
targeting at about 3 hours against half a week by hand; Gordon (ReWeaver, AIEWF 2026) measured
a pure-LLM edit loop degrading over 12 iterations and holding near 90% under deterministic
guardrails. None gives a rule for when to stop patching; they confirm 08–10's choices rather
than add a mechanism.

**Where the evidence is thin.** "Brittle by ownership" is not a phrase in the literature; the
nearest are knowledge loss, bus factor and Bird et al.'s minor-contributor finding, and this
tree has one human and a fleet of bots, so ownership analysis is out of scope (§9). Temporal
coupling is an architecture signal, not a defect predictor (Graves et al. found co-change a
poor predictor), and is read, not keyed. No peer-reviewed study shows that spec-first or
design-doc-first practice reduces agent-written churn; GitHub Spec Kit, Kiro and Anthropic's
guidance are recommendations. The hotspot percentages CodeScene quotes are examples, not a
study; this stream's own 1.8% / 28% figure is the number it uses.

