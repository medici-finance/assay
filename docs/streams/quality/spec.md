# quality — spec

**Version:** v0.1-design
**Status:** design draft. MUST/SHOULD express design intent for the eventual v1
specification, not a conformance claim; no implementation exists yet.
**Reference implementation (intended):** a statusgen-sibling Go binary, working name
`qualgen`.
**Licensing:** the tool and every reference adapter in this stream are OSS. House
integration is *configuration* against the interfaces below, supplied out of stream;
the only genuinely-private artifact is the shared-telemetry-corpus privacy ruling that
gates §7.3.

## 1. Scope and motivation

A flow board measures **delivery** (throughput, DORA-like metrics) but not **code
quality**. This tool is the missing layer: a git-history miner answering three
questions delivery metrics cannot:

1. **Are we getting better?** Durable landed change vs rework/bug-churn over time, and
   whether the ratio improves as practice evolves.
2. **Where is the codebase brittle?** Per-file/per-package risk (hotspots, defect
   density, knowledge concentration, hidden coupling) — feedable to the agents working
   the code at PR-risk time and at authoring time.
3. **How do we compare?** The industry measures AI-era code quality with GitClear's
   published metrics and SZZ-derived defect lineage. We produce the same metrics,
   computed the published way, so the numbers are comparable to public baselines — and,
   unlike the proprietary scoring engines, auditable.

The tool targets **any git repository** and mines **full history**. Both are
load-bearing: "are we getting better" needs a pre-adoption (and pre-AI) baseline from
the same codebase's own history, and comparability is only testable on codebases that
never adopted the methodology. §3.1 defines the capability ladder by target profile.

### 1.1 Why the industry anchors matter

- **GitClear** (proprietary SaaS) published the dominant AI-code-quality studies (211M
  changed lines, 2020–2025: churn roughly doubled, copy/paste exceeded refactored code,
  duplicate blocks up ~8×). Their scoring engine is a black box, but the underlying
  *metric definitions* are published and reproducible from git history alone.
  Implementing those definitions gives industry-comparable numbers plus a credibility
  edge: ours are inspectable.
- **SZZ** is the academic standard (200+ studies) for tracing a bug-*fixing* change back
  to the bug-*introducing* change. Fully specified, multiple OSS reference
  implementations, the honest basis for bug-churn as a trend metric.

### 1.2 The provenance extension

Where a repo carries a provenance chain — a change traces to a task/brief, a brief to a
stream and a spec — no industry tool exploits it. Once SZZ names the bug-introducing
change, the chain can be walked backwards to classify **at what stage the defect was
introduced** — spec, plan, implementation, or review escape (§6). That per-stage defect
ledger is the novel measurement, and it makes "are we getting better" answerable *per
stage of the process* rather than only in aggregate. It is a capability behind a
pluggable provenance-linkage adapter (§3.1, §6), not a hard dependency on any one
process.

## 2. Non-goals

- **Not a linter or static analyzer of source semantics.** The tool mines *history*
  (commits, diffs, PR/issue linkage) plus one cheap per-file complexity proxy.
  Rule-engine code health (CodeScene, staticcheck) is out of scope.
- **Not a delivery-metrics platform.** DORA collection stays where it is; this tool
  *joins* to it (§8), it does not replace it.
- **Not a dashboard product.** Output is generated artifacts (JSONL + a single-writer
  rendered view, §9.3) in the STATUS.md discipline. Visualization is a consumer.
- **Not real-time.** Batch runs (CI cadence and on-demand) suffice for every consumer.

## 3. Architecture

A single repo-agnostic Go binary in the statusgen mold:

- **Language/library:** Go, `go-git` for history access (no cgo, no libgit2), shelling
  to `git` permitted where go-git is materially slower (blame at scale) — decided by
  benchmark, recorded in the implementation brief.
- **Distribution:** pinned release binary via the same version-pin mechanism as
  statusgen (`.assay-versions`).
- **State:** a committed-artifact model, not a database. Each run emits/updates JSONL
  artifacts (§9) that are diffable and single-writer. A local uncommitted cache MAY hold
  expensive intermediates (blame results).
- **Modes:** `mine` (full/incremental extraction), `report` (render views, trends),
  `pr <n>` (per-PR risk features, §9.1), `check <paths>` (brittleness screen for a named
  file set, §9.2). All modes are read-only against the repo; writes happen only to the
  artifact paths.

### 3.1 Target profiles and the capability ladder

Each layer degrades independently and *visibly* (three-state rule) as provenance thins.
Three profiles bound the range:

| Profile | M1 (comparability) | M2 (SZZ) | M3 (stage attribution) |
|---------|--------------------|----------|------------------------|
| **A — provenance-native** (task/brief/board linkage, defect-labeled issues) | full | full: evidence tiers 1–3 | full |
| **B — foreign repo** (any git repo; issue-tracker linkage optional) | full — M1 needs nothing but history | tiers degrade: tier 1 only where an issue-tracker adapter is configured; else tiers 2–3 (branch/commit taxonomy, keywords), reported as such | unavailable — emitted as *could-not-attribute*, never silently zero; per-file defect density still works |
| **C — historical backfill** (either profile, mined before/across adoption) | full | as per profile | pre-adoption defects are `untraceable` by construction |

Profile-B rules:

- **No in-repo writes.** Artifacts land in an operator-chosen tracking root
  (`--out <dir>`), never committed into the target. The single-writer `QUALITY.md`
  discipline applies to the tracking root.
- **Fix identification is adapter-based**: a small linkage interface (issue is
  defect-classed; commit/PR closes issue). A GitHub-labels adapter is the first
  reference adapter; others are configuration, not new mining code.
- **Identity classes are configurable**: the human/agent/automation partition (§4.2,
  §4.4) maps to author patterns supplied per target; unmapped authors form an explicit
  `unclassified` class.

Profile-C (historical) rules:

- `mine` walks **full history by default**; incremental runs extend, never replace, the
  baseline.
- **Era markers**: per-target configuration dates epochs (e.g. `pre-AI`, `AI-assisted`,
  `adopted`) and every windowed metric reports per-era, so before/after comparisons are
  first-class. This is how "did adopting the practice bend the churn/defect curves" is
  answered on one's own repos, and how a prospective adopter's codebase is baselined
  before adoption.
- Renames, rewritten history, and shallow clones floor what backfill can see; the mine
  records its horizon (earliest reachable commit, detected discontinuities) in the
  artifact header.

### 3.2 Three-state instrument invariant

Global to every layer and profile: every check and every metric distinguishes
*measured*, *measured-zero*, and *could-not-measure*. A blame that failed, a PR with no
linked issue, a commit unreachable through a squash — each is reported as unmeasured,
never silently as zero. This is a design requirement from day one, not a retrofit.

## 4. Layer M1 — comparability metrics (GitClear-aligned)

All M1 metrics are computed per commit, aggregated per file, package, PR, stream,
author-identity, and time window. Definitions follow GitClear's published methodology so
the numbers are comparable to their public baselines; where we deviate, the deviation is
stated in the artifact itself.

### 4.1 Line-operation taxonomy

Each changed line in each commit is classified: `added`, `deleted`, `updated`, `moved`
(relocated, content-identical/near-identical), `copied` (duplicated — identical to a
line that *remains*), `churned` (added/updated by this identity within the churn window
and revised again now). Detection of `moved` vs `copied` uses block-level content
matching (≥ N similar lines, N configurable, default 4 — GitClear's published
duplicate-block granularity). Headline comparisons: **copy/paste ratio**
(`copied`/(`moved`+`copied`)) and **duplicate-block rate**.

### 4.2 Churn / rework rate

**Churn**: a line revised or deleted within **14 days** of landing (GitClear's published
window; configurable, 14d the comparable default). Reported as churned-lines/new-lines,
per window, stream, and author-identity class (human / agent / automation). This is the
"premature or low-quality commit" signal — the industry number that rose from ~3.3%
(pre-AI) to ~7.1% (2025).

### 4.3 Hotspots (churn × complexity)

Per file: `hotspot = change_frequency(decayed) × complexity_proxy`. Change frequency is
commit-touch count with exponential time decay (recent weighs more). Complexity proxy is
indentation-based complexity (language-agnostic) as the base; cyclomatic complexity MAY
refine it per language. The proxy is deliberately cheap — the *product* with change
frequency predicts defects, not the proxy alone.

### 4.4 Knowledge distribution (SPOF)

Per file/package: ownership concentration (share of surviving lines per author-identity)
and **bus factor** (minimum identity set owning > K% of the code). In an agent-fleet
repo, "author" is the identity class *and* the dispatching role — concentration in a
single *role* is a process SPOF even when line-authors vary.

### 4.5 Change coupling

Files that change together (co-commit/co-PR frequency above baseline) without a static
dependency. The consumer-facing signal is the inverse: **a PR touching file A but not
its historical coupling partner B** is flagged (§9.1) — the strongest cheap brittleness
predictor in the coupling literature.

### 4.6 Instruction-layer brittleness (context rot)

The same brittleness analysis applied to the *instruction layer* the agents run on:
config/instruction files, task briefs, specs, and skills referencing paths/symbols that
no longer exist, and normative docs drifting from the code they describe. Agents are only
as good as their instructions. Signals:

- **Reference validity**: dead file paths, dead symbols, dead typed IDs in
  instruction-bearing docs, trended over time (decaying reference-validity = rot).
- **Doc↔code co-change**: an instruction doc whose described code changes repeatedly
  without the doc changing is presumptively stale (§4.5 coupling, applied doc-to-code).

A point-in-time source↔render drift detector is the existing precedent; this layer is its
measured, trend-lined generalization and SHOULD share that detector rather than build a
second one.

## 5. Layer M2 — defect lineage (SZZ)

### 5.1 Fix identification

A commit/PR is a **defect fix** when any of (precedence order):

1. It closes/references a defect-labeled issue (`Fixes #N` where N carries a
   bug/defect/incident label — including a repo's verdict-issue lane where it has one,
   which makes the linkage near-total for agent work).
2. Its PR is classified `fix` by the repo's PR taxonomy (branch prefix `fix/`,
   conventional-commit `fix:` title).
3. Keyword fallback (fix/bug/defect/regression in the message) — flagged as the weakest
   evidence tier, reported separately, never silently merged with tiers 1–2.

Fix identification is behind a **pluggable linkage adapter**; the GitHub-labels adapter
is the reference implementation. Each defect-fix records its evidence tier; tier
composition is itself a reported metric.

### 5.2 Inducing-commit trace

B-SZZ with standard refinements: for each fix, blame the deleted/modified lines at the
fix's parent; exclude cosmetic/format-only inducers; exclude inducers that postdate the
defect report. Output: `(fix_pr, inducing_commit(s), inducing_pr(s), confidence)`.

**Disclosed limits** (~40% of cases are not reachable by blame alone per the SZZ
literature): blameless bugs (omissions), multi-hop histories, squash-merge floors.
Unreachable traces are recorded as *could-not-trace* (§3.2); the trace-rate is published
alongside every derived number.

### 5.3 Derived metrics

- **Defect-inducing change rate**: inducing-PRs/merged-PRs per window — the bug-churn
  trend line.
- **Per-file defect density**: inducing-commit count per file (feeds §9.1).
- **Fix latency**: inducing-merge → fix-merge time.
- **DORA CFR refinement**: change-failure-rate re-based on traced defects (§8).

## 6. Layer M3 — stage attribution (provenance-gated)

For each traced defect, walk the provenance chain of the *inducing* PR
(`inducing PR → task/brief → stream → spec/ruling`) plus the review verdicts at-head
when it merged, and classify the escape stage:

| Stage | Test |
|-------|------|
| `spec` | The change faithfully implements its brief, the brief faithfully reflects the spec as it stood — the *requirement* was wrong. |
| `brief` | Implementation matches the brief, but the plan did not cover the defect surface — the *plan* was wrong. |
| `implementation` | The plan covered it and the change violates it — the *work* was wrong. |
| `review-escape` | Orthogonal overlay, recorded for every defect: which lanes approved the inducing PR at head. |
| `untraceable` | Chain broken (pre-provenance history, no linkage, could-not-trace) — reported as such, never binned elsewhere. |

Classification is evidence-gathering + judgment: the tool assembles the dossier (brief
text at inducing time, diffs, verdicts, postdating rulings) deterministically; the stage
call MAY be model-assisted but every call records its dossier and is spot-auditable.
Runs at fix time, lands as an append-only artifact, correctable by tombstone amendment,
never silent edit. Chain-walking is behind the **provenance-linkage adapter** (§3.1); a
generic commit→issue adapter ships as reference, richer chains are configuration.

**Output: the per-stage defect ledger** — defects/window by stage, per stream. "Are we
getting better" decomposes into: is the spec stabilizing, are plans covering their
surface, is implementation quality rising, are review lanes catching more. Each stage has
a different remedy; aggregate counts hide which to apply.

## 7. Layer M4 — methodology reflexivity

M1–M3 measure the *code*. M4 turns the same instruments on the *process and harness*, so
process changes are judged on outcomes. Everything is a join of M1–M3 outputs against
artifacts already recorded — no new mining.

### 7.1 Gate-yield accounting

Per review lane: defects caught pre-merge (request-changes findings whose flagged surface
matches a later trace) vs escapes attributed to that lane by M3's review-escape overlay.
Output: catch-rate, escape-rate, latency cost per gate — the evidence to strengthen,
re-scope, or retire a gate.

### 7.2 Ritual effectiveness (natural experiments)

A fleet generates variation (rich vs minimal Verify tables, strong vs default tier,
differing lane coverage). M4 joins those authoring attributes to downstream M1/M2
outcomes. Headline metrics: **cost per durable KLOC by model tier × brittleness band**
(does the strong tier measurably churn less on hotspot code, by enough to pay its
premium?) and **Verify-depth vs escape rate**. These are observational joins, not
controlled experiments; confounders (harder code gets stronger tiers *and* more churn)
are acknowledged in every readout, and brittleness-band stratification is the minimum
control.

### 7.3 Session forensics (harness telemetry × outcome)

The join no code-only miner can make: per-session harness telemetry (retries, context
length, tool-call churn, interruptions, refusals) against the M1/M2 outcomes of the PRs
those sessions produced. Output: which *harness behaviors* predict defective or
churn-heavy PRs — measuring the scaffolding, not the model. Telemetry is read through a
**pluggable telemetry-source interface**; a file-based reference adapter ships in this
stream. The telemetry *data* and any shared/telemetry-corpus inclusion, retention, and
audit are governed by a separate privacy ruling (a downstream, human-gated decision), not
by this stream.

## 8. DORA join

The delivery-metrics collector keeps collecting. This tool contributes: a **quality
denominator** (durable-change volume = landed lines minus 14-day churn minus `copied`),
a **CFR refinement** (traced defect-inducing rate alongside incident-based CFR, with the
evidence-tier split visible), and joins on **PR number + merge SHA + stream/task ID** —
the keys the board already uses. Delivery metrics are read through a pluggable source
interface. Schema naming SHOULD follow DevLake's domain-layer names where a concept
matches (commits, pull_requests, incidents), purely for cross-tool comparability; DevLake
itself is not a dependency.

## 9. Consumers

- **9.1 PR riskscore feed** — `qualgen pr <n>` emits per-touched-file features (hotspot
  percentile, traced defect density, top-identity ownership share, missing coupling
  partners, a three-state `measured` flag). Consumers weight them; thresholds live in the
  consumer's config, not here. **Evolution:** once the M2 corpus is large enough, the
  hand-weighted features SHOULD graduate to a JIT defect-prediction model (Kamei-style)
  trained on the repo's own traced defects — the heuristic features remain the fallback
  and the explanation layer.
- **9.2 Authoring/CI screen** — `qualgen check <paths>` screens a named file set against
  the same features; a set touching a file above the brittleness threshold SHOULD be
  flagged for stronger execution tier, added coverage over the hotspot's defect history,
  and an explicit coupling-partner check. Advisory NOTICE first; hard gating is a later,
  separate decision.
- **9.3 Trend view** — a generated, single-writer `QUALITY.md` (CI is the only writer,
  local runs read-and-discard) rendering churn trend, copy/paste ratio, duplicate-block
  rate, defect-inducing rate, per-stage ledger, top-10 hotspots, bus-factor alarms — each
  with its industry-comparable number beside the local number where one exists.
- **9.4 Artifacts** — relative to the tracking root (§3.1): `docs/quality/metrics.jsonl`
  (M1 aggregates, append-only), `docs/quality/defects.jsonl` (M2 traces with tier +
  confidence), `docs/quality/attribution/` (M3 per-defect dossiers, append-only,
  tombstone amendments, one file per defect). Cache (uncommitted) holds blame/match
  intermediates.
- **9.5 Auto-filed refactor work** — duplicate-block clusters and decaying hotspots above
  threshold generate refactor issues/briefs automatically, through a **pluggable
  issue-filer** (a GitHub-issues reference adapter ships here). Filing is advisory and
  budgeted; a human or an intake process triages — the tool never self-dispatches work.
- **9.6 Quality error-budgets** — per-stream churn and defect-inducing budgets in an
  alarm posture: a breach is an alarm, not a dashboard line. Budgets are config, set only
  after ≥ 2 windows of measurement.
- **9.7 Retrospective inputs** — a cadence retrospective whose inputs must be
  generated/logged only. This tool's outputs (churn trend, gate yield, per-stage ledger,
  budget status) are exactly that input set.

## 10. Honest-claims discipline

- **Comparability**: "computed per GitClear's *published* definitions" — never
  "GitClear-equivalent" (the scoring engine is a black box; equivalence is unverifiable).
  Where windows or thresholds differ, the artifact states both.
- **SZZ**: every derived number ships with its trace-rate and evidence-tier composition.
  "X% defect-inducing rate at Y% trace coverage", never bare X.
- **Stage attribution**: "evidence-assembled, judgment-classified, spot-audited" — never
  "measured." The dossier is the defensible part; the stage call is a recorded judgment.
- **Three-state everywhere**: could-not-measure is a first-class output.

## 11. Phasing

| Wave | Deliverable | Brief |
|------|-------------|-------|
| 0 | Miner skeleton: go-git extraction, incremental runs, three-state plumbing | 01 |
| 1 | M1 metrics + `QUALITY.md` view + trend JSONL; instruction-layer reference-validity | 02–05 |
| 2 | M2 SZZ: fix identification (adapter + tiers), B-SZZ trace, defect density | 06–07 |
| 3 | `pr` mode + riskscore features; `check` mode + brittleness screen | 08–09 |
| 4 | M3 stage attribution: dossier, ledger, per-stage trend | 10 |
| 5 | DORA join: quality denominator + traced CFR | 11 |
| 6 | Gate-yield accounting + ritual-effectiveness joins (needs M3 corpus) | 12 |
| 7 | Session forensics: telemetry × outcome join | 13 |
| 8 | Auto-filed refactor + error-budgets + RETRO feed; learned-riskscore graduation | 14–15 |

Waves 0–2 are pure history mining — useful standalone. Waves 6–8 are deliberately last:
each consumes an M1–M3 corpus that must exist and season first.

## 12. Prior art (informative)

- GitClear: *AI Copilot Code Quality* (2025), *The Maintainability Gap* (2026) — metric
  definitions and industry baselines for §4.
- Śliwerski, Zimmermann, Zeller (2005) and successors; SZZ Unleashed / OpenSZZ / PR-SZZ —
  §5 algorithm family and its measured limits.
- Kamei et al., *A large-scale empirical study of just-in-time quality assurance* — §9.1
  learned-riskscore feature family.
- Tornhill, *Your Code as a Crime Scene* / code-maat / CodeScene — §4.3–4.5 hotspot,
  coupling, and knowledge-map analyses.
- Apache DevLake — domain-schema naming for §8; not a dependency.

## 13. Open questions (for the review)

1. **Name.** `qualgen` is a placeholder (statusgen symmetry). Alternatives welcome.
2. **Squash policy.** SZZ resolution floors at merge granularity; a draft-PR-per-task flow
   preserves fine-grained history *if* merges keep it. Decide merge-commit vs squash
   consciously as part of adoption.
3. **Author-identity classes.** Exact partition (human / agent / automation / model tier)
   for §4.2/§4.4 — model-tier attribution would let §7.2's "needs strong tier" claim be
   *tested*.
4. **Thresholds.** Hotspot percentile and brittleness cutoffs for §9.1/§9.2 — ship
   measuring for ≥ 2 windows before any threshold gates anything.
5. **Where M3's model-assisted classification runs**, and who audits it.
6. **Rollout order across targets.** House fleet first, or a foreign-repo profile-B target
   early to keep the adapter surface honest?
7. **Session-telemetry provenance and privacy (§7.3).** Any shared/telemetry-corpus
   inclusion, retention window, and audit needs an explicit ruling before wave 7 ships —
   this is the one house-gated decision the OSS tool defers to its operator.
8. **M4 observational validity (§7.2).** Minimum stratification before a
   ritual-effectiveness readout may be published, and whether any M4 number ever appears
   in an outward artifact given it is a natural experiment, not a controlled one.
