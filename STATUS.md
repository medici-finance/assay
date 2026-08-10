<!-- GENERATED FILE — do not edit. Source of truth: docs/streams/*/README.md.
     Regenerate: go run ./tools/statusgen -->

# Project Status

## Roll-up

### Product

| Stream | Priority | Status | Briefs done | Last touched | Notes |
|---|---|---|---|---|---|
| [assay-site](docs/streams/assay-site/README.md) | P2 | active | 0/7 | 2026-08-10 | implement=cheap verify=strong |
| [metrics-harvest](docs/streams/metrics-harvest/README.md) | P1 | active | 0/2 | 2026-08-10 | implement=cheap verify=strong |

### Platform

| Stream | Priority | Status | Briefs done | Last touched | Notes |
|---|---|---|---|---|---|
| [assay-dogfood](docs/streams/assay-dogfood/README.md) | P1 | active | 1/5 | 2026-08-10 |  |
| [code-review-2026-07-23-assay-toolkit](docs/streams/code-review-2026-07-23-assay-toolkit/README.md) | P1 | active | 0/2 | 2026-08-10 | implement=cheap verify=strong |
| [code-review-2026-07-23-oit](docs/streams/code-review-2026-07-23-oit/README.md) | P1 | active | 0/1 | 2026-08-10 | implement=cheap verify=strong |
| [desk-apps](docs/streams/desk-apps/README.md) | P1 | active | 4/9 | 2026-08-10 |  |
| [desk-hardening](docs/streams/desk-hardening/README.md) | P1 | active | 0/10 | 2026-08-10 | implement=cheap verify=strong |
| [desk-tools](docs/streams/desk-tools/README.md) | P1 | active | 8/12 | 2026-08-10 |  |
| [issue-loop](docs/streams/issue-loop/README.md) | P1 | active | 12/15 | 2026-08-10 |  |
| [loop-engine](docs/streams/loop-engine/README.md) | P1 | active | 0/7 | 2026-08-10 |  |
| [methodology](docs/streams/methodology/README.md) | P1 | active | 36/47 | 2026-08-10 |  |
| [methodology-metrics](docs/streams/methodology-metrics/README.md) | P1 | active | 23/42 | 2026-08-10 | implement=cheap verify=strong |
| [orchestra-review](docs/streams/orchestra-review/README.md) | P2 | active | 0/2 | 2026-08-10 | implement=cheap verify=strong |

### Ecosystem

| Stream | Priority | Status | Briefs done | Last touched | Notes |
|---|---|---|---|---|---|
| [assay-launch](docs/streams/assay-launch/README.md) | P1 | active | 1/7 | 2026-08-10 |  |
| [assay-product](docs/streams/assay-product/README.md) | P1 | active | 8/9 | 2026-08-10 | → ../assay-toolkit/STATUS.md |

## Next up

> **DEGRADED — claim filtering did not run.** `git ls-remote --heads origin` failed: fatal: 'origin' does not appear to be a git repository
> The rows below are an **unfiltered superset**: briefs already claimed by an open `origin` branch are NOT excluded, so some may already be in flight. Do not dispatch from this board until a run with a reachable `origin` regenerates it (assay-toolkit#305).

_Next-up: 15 of 26 eligible, UNFILTERED — see the degraded notice above — 11 held back (span-of-control cap 20). Overflow is itself an alarm (EEMUA-191): clear WIP before pulling more._

| Stream | Brief | Wave | Score |
|---|---|---|---|
| desk-hardening | 01 — Three-state instrument invariant + fleet audit + mutation-test Verify rule [exec:strong] | 0 | 3500 |
| methodology | 09 — Article 1 — "Status is a build artifact" | 2 | 3500 |
| methodology-metrics | 40 — opmetrics — local operator-layer collector: relay ratio, intervention rate, decision latency, session hygiene → aggregates-only day-file [exec:strong] | 1 | 3200 |
| methodology-metrics | 28 — Issue metrics — `--issues`: counts + age/sitting-time + internal-vs-external + by-raising-desk | 3 | 3000 |
| methodology | 42 — GitHub-durable dispatch claim — replace the machine-local batch-fanout claim lock | 0 | 2500 |
| desk-hardening | 09 — Coverage & attribution — port bugs/ carrier + expected-visibility drift check [exec:strong] | 0 | 2000 |
| desk-tools | 06 — Cutover — install, allowlist swap + local.json purge, skill wiring, drills | 2 | 2000 |
| desk-tools | 12 — repo scope widens to org-default for medici-finance, with a public-repo trust gate | 2 | 2000 |
| issue-loop | 14 — Desk emits briefs, not PRs — dispatch returns to batch-fanout; closed-issue briefs archive to per-stream done/ [exec:strong] | 6 | 2000 |
| issue-loop | 15 — Intake directory split by disposition — `ls` is the triage board; a move is a transition, id is identity [exec:strong] | 6 | 2000 |
| loop-engine | 07 — Interim durable Monitor for verify-desk (bridge until 01) [exec:strong] | 0 | 2000 |
| methodology | 39 — Defense-in-depth as the default authoring posture — layered designs for core-system briefs | 0 | 2000 |
| methodology | 40 — Multi-repo dispatch — batch-fanout reads every product repo's Next-up board | 0 | 2000 |
| methodology-metrics | 13 — Per-stream max-concurrent — Next-up offers at most N in-flight briefs from a serialized stream (#217) | 0 | 2000 |
| methodology-metrics | 24 — Roadmap deck pages 2..N — identical-skeleton per-stream pages (delta panel, wave ladder) | 2 | 2000 |

## Intake queue

_0 untriaged entries — the front door is clear._

## Awaiting verification / review (23 desk-actionable of 29 total — 23 at implemented, 6 verified awaiting review)

_Gate-queue ordered by score: priorityWeight + staleness×stalenessPerDay + valueWeight + unblocksWeight×blockedCount. The weights are an evolving heuristic (F-09 discipline) — not a claim of truth. Board segmented by blocker owner (methodology-metrics/34): the desk-actionable headline counts only the queue the desk can actually drain._

_`done‡` / `verified‡` = closed over an **UNRUN risk-bearing Verify row**: a live/mutating check with no completed Evidence row behind it (methodology-metrics/37). UNRUN is DERIVED from Verify-vs-Evidence coverage — a row counts as run only when an Evidence row names it with a date and a runner, so silence reads as unrun. `--lint` names each one and whether it was routed to a follow-up._


### Desk-actionable (23)

| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |
|---|---|---|---|---|---|---|---|
| assay-site | 01 | implemented | 4000 | 6 | — | — | — |
| desk-apps | 04 | implemented | 3000 | 2 | — | — | — |
| metrics-harvest | 01 | implemented | 2500 | 1 | — | — | — |
| methodology-metrics | 18 | verified | 2200 | 0 | — | 2026-07-24 glm-5.2-verifier | — |
| methodology-metrics | 21 | implemented | 2200 | 0 | — | — | — |
| code-review-2026-07-23-assay-toolkit | 01 [exec:strong] | implemented | 2000 | 0 | — | — | — |
| code-review-2026-07-23-assay-toolkit | 02 [exec:strong] | implemented | 2000 | 0 | — | — | — |
| code-review-2026-07-23-oit | 08 [exec:strong] | implemented | 2000 | 0 | — | — | — |
| desk-apps | 05 | verified‡ | 2000 | 0 | — | 2026-07-30 glm-5.2-verifier | — |
| desk-hardening | 02 [exec:strong] | implemented | 2000 | 0 | — | — | — |
| desk-hardening | 06 [exec:strong] | implemented | 2000 | 0 | — | — | — |
| desk-hardening | 07 [exec:strong] | implemented | 2000 | 0 | — | — | — |
| desk-hardening | 08 [exec:strong] | implemented | 2000 | 0 | — | — | — |
| desk-hardening | 10 | implemented | 2000 | 0 | — | — | — |
| desk-tools | 09 | verified | 2000 | 0 | — | 2026-07-27 k3-verifier | — |
| desk-tools | 11 | verified | 2000 | 0 | — | 2026-07-31 glm-5.2-verifier | — |
| issue-loop | 09 | verified | 2000 | 0 | — | 2026-07-20 glm-5.2-verifier | — |
| methodology | 29 | verified | 2000 | 0 | — | 2026-07-24 glm-5.2-verifier | — |
| methodology | 44 | implemented | 2000 | 0 | — | — | — |
| methodology-metrics | 34 | implemented | 2000 | 0 | — | — | — |
| methodology-metrics | 37 | implemented | 2000 | 0 | — | — | — |
| orchestra-review | 01 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| orchestra-review | 02 | implemented | 1000 | 0 | — | — | — |

### Awaiting human gate (3)

| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |
|---|---|---|---|---|---|---|---|
| assay-product | 04 | implemented | 5000 | 6 | — | — | — |
| loop-engine | 01 [exec:strong] | implemented | 4500 | 5 | — | — | — |
| assay-dogfood | 01 | implemented | 3500 | 3 | — | — | — |

### Awaiting implementer rework (3)

| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |
|---|---|---|---|---|---|---|---|
| assay-launch | 06 [exec:strong] | implemented | 4500 | 5 | — | — | — |
| assay-dogfood | 02 | implemented | 3000 | 2 | — | — | — |
| desk-apps | 06 | implemented | 2500 | 1 | — | — | — |

## Unresolved findings

_None._

## Incomplete briefs

### assay-dogfood (4 open)

- 01 Bootstrap ../assay-toolkit — repo, plugin scaffold, marketplace, governance permissions — implemented (wave 0)
- 02 Skills bundle v0.1 — the five loop skills + resident-rules SessionStart hook, as `assay:*` — implemented (wave 1)
- 04 Dogfood cutover — this repo installs the plugin, retires loose copies, CLAUDE.md shrinks — todo (wave 2)
- 05 Adopter drill — a clean consumer onboards from the marketplace alone; gaps become issues — todo (wave 3)

### assay-launch (6 open)

- 06 assay-site repo — migrate the built site + harden (mirror site-repo) — implemented (wave 0)
- 01 Methodology page suite — add overview + missing pages, deepen the migrated ones — todo (wave 1)
- 02 Downloadable methodology PDF — render + wire to a site download link — todo (wave 2)
- 03 Public "how it runs" live-metrics page — DORA/trend snapshot — todo (wave 1)
- 07 Automated publish pipeline — daily jobs push into assay-site, no manual step — todo (wave 2)
- 05 Launch readiness + go-live gate (human, irreversible) — todo (wave 3)

### assay-product (1 open)

- 04 Naming clearance + domain wiring — domain wiring (Plumb+Assay); formal TM search deferred 2026-07-14 — implemented (wave 0)

### assay-site (7 open)

- 01 Messaging spine + information architecture — hero honest claim, section map, voice guardrails — implemented (wave 0)
- 02 Three-tier open-core section — free / premium / on-prem, each pitch + who-it's-for + link — todo (wave 1)
- 03 What-it-does explainers — briefs, registers, lifecycle, statusgen — todo (wave 1)
- 04 Landing page build — static self-contained brand-compliant HTML — todo (wave 2)
- 05 Quickstart / GitHub / articles wiring — outbound CTAs, nav, footer, links resolve — todo (wave 3)
- 06 Honest-claims review — F-08 no-overclaim gate over the rendered copy — todo (wave 4)
- 07 Domain wiring — assay.guide host choice + records (execution is human:<name>'s) — todo (wave 5)

### code-review-2026-07-23-assay-toolkit (2 open)

- 01 statusgen anti-falsification & tripwire robustness (T1, T3, T7, T8, T9, T10) — implemented (wave 0)
- 02 DAR-drift CI gate + Job-name-bump enforcement (M3) — implemented (wave 0)

### code-review-2026-07-23-oit (1 open)

- 08 desk board/loop/post robustness (T2, T4, T5, T6) — implemented (wave 0)

### desk-apps (5 open)

- 04 [deskevidence — verify-desk commits Evidence as verifier-app[bot]](./brief-04-deskevidence.md) — implemented (wave 2)
- 05 worker App + deskpr/deskreply App-identity cutover — verified (wave 2)
- 06 desk App cutover — coordinator main-landing identity (replaces example-org/client-hook) — implemented (wave 2)
- 07 statusgen Evidence-actor check (tamper-evident verified rows, closes I-08) — todo (wave 3)
- 08 F-13 server-side main-push ruleset ({verifier-app, desk-app, human:<name>}) — todo (wave 3)

### desk-hardening (10 open)

- 01 Three-state instrument invariant + fleet audit + mutation-test Verify rule — todo (wave 0)
- 02 Fanout dispatch hygiene — claim-at-dispatch, branch-from-fresh-main, foreign-commit self-check — implemented (wave 0)
- 03 Source↔render drift + stale-normative-doc detection — todo (wave 1)
- 04 Disclosure & audience controls — leak + candour scan for public artifacts — todo (wave 1)
- 05 Merge-time re-check + cite-by-expression + body/Verify re-derive — todo (wave 1)
- 06 Durable desk watchdog + autonomous drive + file-at-discovery — implemented (wave 0)
- 07 Register-drain loop + ID hygiene (findings drain, resolved-vocabulary, sequential-ID collisions) — implemented (wave 0)
- 08 Unreviewed desk authority — evidence-not-verdict dispatch + verify-before-apply — implemented (wave 0)
- 09 Coverage & attribution — port bugs/ carrier + expected-visibility drift check — todo (wave 0)
- 10 deskadvisory — recompute-at-the-gate verification for security-advisory fixes — implemented (wave 0)

### desk-tools (4 open)

- 06 Cutover — install, allowlist swap + local.json purge, skill wiring, drills — todo (wave 2)
- 09 deskroster — self-declared open-work → session roster (out-of-git) — verified (wave 1)
- 11 deskboard orders actionable PRs by gate score (statusgen --gate-scores) — verified (wave 1)
- 12 repo scope widens to org-default for medici-finance, with a public-repo trust gate — todo (wave 2)

### issue-loop (3 open)

- 09 Wire intake triage into the-desk — triage at boot + on the alarm; no fourth window — verified (wave 4)
- 14 Desk emits briefs, not PRs — dispatch returns to batch-fanout; closed-issue briefs archive to per-stream done/ — todo (wave 6)
- 15 Intake directory split by disposition — `ls` is the triage board; a move is a transition, id is identity — todo (wave 6)

### loop-engine (7 open)

- 01 Drain engine + verify-desk reference consumer — implemented (wave 0)
- 02 Generalize: batch-fanout as second engine consumer — todo (wave 1)
- 03 issue-loop dispatch lane as third engine consumer — todo (wave 1)
- 04 Guardrail module — single home for the six duplicated rule blocks — todo (wave 1)
- 05 pr-review-desk board-reactor — formalize, do NOT drain-ify — todo (wave 1)
- 06 Companion article + generic cloneable drain-harness — todo (wave 1)
- 07 Interim durable Monitor for verify-desk (bridge until 01) — todo (wave 0)

### methodology (11 open)

- 09 Article 1 — "Status is a build artifact" — in-progress (wave 2)
- 29 exec-tier — complex briefs signal a minimum execution-model tier; dispatch enforces it — verified (wave 0)
- 39 Defense-in-depth as the default authoring posture — layered designs for core-system briefs — todo (wave 0)
- 44 Verify-command #509 sweep — fix every unfailable grep/go-test row + close the detection gap — implemented (wave 0)
- 45 why: backfill (phase 2) — add why: to the 32 live briefs; grandfather the 75 closed — todo (wave 0)
- 46 Desk rename pass — batch-fanout→worker-desk + issue-loop→intake-desk (four-loop taxonomy) — todo (wave 1)
- 40 Multi-repo dispatch — batch-fanout reads every product repo's Next-up board — todo (wave 0)
- 41 Phase-1 board provisioning — reconciler and platform-repo generate a board from the pinned statusgen release — todo (wave 0)
- 42 GitHub-durable dispatch claim — replace the machine-local batch-fanout claim lock — todo (wave 0)
- 43 Intra-brief file-scoped parallelism — split one large brief across concurrent workers — todo (wave 1)
- 47 Findings register becomes a corroborated state machine — bounded shelving + transition guard (closes #721 + toothless-park, folds F-36) — todo (wave 0)

### methodology-metrics (19 open)

- 13 Per-stream max-concurrent — Next-up offers at most N in-flight briefs from a serialized stream (#217) — todo (wave 0)
- 18 Daily factory-floor bottleneck report — --bottleneck: per-stage WIP, constraint, shift — verified (wave 1)
- 21 Consumer-enumeration gate — shared-value briefs carry a consumers: list whose routing claims are corroborated against the diff — implemented (wave 1)
- 24 Roadmap deck pages 2..N — identical-skeleton per-stream pages (delta panel, wave ladder) — todo (wave 2)
- 25 Cadenced exec artifacts — weekly WBR deck + monthly exec summary on the shared schedule — todo (wave 2)
- 28 Issue metrics — `--issues`: counts + age/sitting-time + internal-vs-external + by-raising-desk — todo (wave 3)
- 29 `raised-by:<desk>` stamping — the filing desks label issues they raise (makes 28's by-desk cut real) — todo (wave 4)
- 30 Self-improvement metric — self-diagnosed + self-resolved (agent-raised + agent-fixed, no human touch) vs human-touched — todo (wave 5)
- 33 Trim the always-unknown DORA metrics from the retro-facing emit — todo (wave 1)
- 34 Segment the Awaiting board by blocker owner (desk / human-gate / rework / paused / env) — implemented (wave 0)
- 35 30-day statusgen check-firing audit — retire cold `--lint` rules — todo (wave 1)
- 36 Drain-before-instrument — gate new metric/alarm briefs while the queue they measure is over threshold — todo (wave 0)
- 37 Make UNRUN a first-class board state — block `done` on an unrun risk-bearing Verify row — implemented (wave 0)
- 38 Daily human-gate sign-off digest + per-stream age-at-gate metric — todo (wave 1)
- 39 Auto-flip verified→done for gate:model briefs from the reviewer-App approval — todo (wave 1)
- 40 opmetrics — local operator-layer collector: relay ratio, intervention rate, decision latency, session hygiene → aggregates-only day-file — todo (wave 1)
- 41 Autonomy ratio + token efficiency + deterministic-gate share + rework — the step-3 gauges in statusgen/harvest — todo (wave 2)
- 42 Ladder-position indicator — one computed adoption-step number (behavioral axes, never tooling) on the board + roadmap deck — todo (wave 3)
- 43 verify-gate-close becomes the SOLE writer of a human:&lt;name&gt; stamp — `--lint` rejects it anywhere else — todo (wave 1)

### metrics-harvest (2 open)

- 01 Multi-domain harvester — per-domain repo config, cross-org gh/git snapshots, dated artifact tree — implemented (wave 0)
- 02 Cross-domain aggregator — per-domain totals + all-products roll-up over the harvest tree — todo (wave 1)

### orchestra-review (2 open)

- 01 Gate-effectiveness telemetry: override rate + catch rate — implemented (wave 0)
- 02 Docs cold-boot reconstruction drill — implemented (wave 0)

## Done briefs

_`done*` = unbacked (I-08 point quality): the row's Evidence section is empty and/or its Verified/Reviewed cells aren't dated+attributed per brief-16 — see `--lint` for the full list. Plain `done` is evidence-backed._

_`done‡` / `verified‡` = closed over an **UNRUN risk-bearing Verify row**: a live/mutating check with no completed Evidence row behind it (methodology-metrics/37). UNRUN is DERIVED from Verify-vs-Evidence coverage — a row counts as run only when an Evidence row names it with a date and a runner, so silence reads as unrun. `--lint` names each one and whether it was routed to a follow-up._

### assay-dogfood (1 done)

- 03 statusgen as a pinned release — build/tag in assay-toolkit, consumers verify by hash — done (wave 1)

### assay-launch (1 done)

- 04 `statusgen --launch` — launch-readiness rollup view — done (wave 0)

### assay-product (8 done)

- 01 Product brief — what Assay is, for whom, honestly — done (wave 0)
- 02 Market analysis — the AI-native PM / agent-orchestration landscape — done (wave 0)
- 03 Product repo hygiene — STATUS.md, roadmap, README doc index in assay-toolkit — done (wave 1)
- 05 Website — assay.guide landing + what-it-does explainers — done (wave 1)
- 06 Pitch deck — Assay presentation in ../decks/assay/pitch — done (wave 1)
- 07 Artifact-freshness cadence — version history on every artifact + deterministic staleness check — done (wave 1)
- 08 Periodic critical-thinking review — stale/accreted/would-not-build-today (anti-Rube-Goldberg) — done (wave 1)
- 09 Market-intelligence skill — product-agnostic field scan as assay:market-intelligence + first run — done (wave 2)

### desk-apps (4 done)

- 01 desk-app brand icons — octagonal assay stamps (reviewer/verifier/worker/desk/issue-loop/intake-loop) + canonical assay-mark — done (wave 0)
- 02 GitHub App setup guide — access matrix, public Apps, dual-install, key custody, token cache — done (wave 0)
- 03 desktoken — key-parameterized token minter (generalizes mint-reviewer-token.go) — done (wave 1)
- 09 INBOUND Apps — issue-loop + intake-loop (the two inbound lanes) — done (wave 3)

### desk-tools (8 done)

- 01 deskkit foundation — shared config/audit/kill-switch/rate-limit/version + install — done (wave 0)
- 02 deskboard v2 — read-only cross-repo board in tools/desk — done (wave 1)
- 03 deskpost — review/comment/ready as the reviewer App, constraints in code — done‡ (wave 1)
- 04 deskpr — push feature branch + open draft PR, draft-only by construction — done (wave 1)
- 05 deskwt — worktree add/remove under allowed prefixes only — done (wave 1)
- 07 deskreply — worker-identity PR replies (never the App voice) — done (wave 1)
- 08 Loop stop-flags — ALL/per-loop kill switch checked every iteration + heartbeat lease — done (wave 1)
- 10 deskpushguard — git pre-push hook refuses a push to a MERGED/CLOSED PR branch — done (wave 1)

### issue-loop (12 done)

- 01 Placeholder-brief schema + statusgen recognizes `issue-<NN>` briefs — done (wave 0)
- 02 `statusgen --scan-issues` — emit placeholders for unhandled open issues — done (wave 1)
- 03 Await/unblock state — worker questions park on the issue, comments resume — done (wave 1)
- 04 Wire the scanner into pr-review-desk + close-out semantics — done (wave 2)
- 05 Reviewer desk-flags become issues — non-blocking review residuals feed the loop — done (wave 2)
- 06 Human-decision issues — any human gate surfaces as a labeled decision issue (situation + pros/cons) — done (wave 2)
- 07 Intake untriaged-age alarm — NOTICE past 3 days + intake-debt board line — done (wave 3)
- 08 Intake triage verbs + decision queue — four exits; decision-needed routes into the single needs-decision queue — done (wave 3)
- 10 Reviewed issue-close via `bugs/` carrier + daily `bugs-gc` prune — done (wave 2)
- 11 Issue-loop desk skill + dedicated window — the inbound twin of pr-review-desk — done (wave 4)
- 12 Issue desk self-dispatch — fans out its own issue-placeholders (claims-locked); batch-fanout skips them — done (wave 5)
- 13 Reconceive the issue-desk as the generic intake-desk — rename + five-exit front door — done (wave 5)

### methodology (36 done)

- 01 statusgen v1.1 — brief-file schema + pre-flight validation — done (wave 0)
- 02 Evidence enforcement at the verified gate — done (wave 1)
- 03 Risk-gate enforcement at the done gate — done (wave 1)
- 04 Findings demotion — scope-change re-entry — done (wave 0)
- 05 Tiering as per-stream policy — done (wave 0)
- 06 Implementer deny-hooks (enforcement layer c) — done (wave 0)
- 07 Toolkit extraction — standalone repo — done (wave 2)
- 08 RETRO bootstrap + first retro (R-01) — done (wave 0)
- 10 Article 2 — "Prevention and reconciliation" (rescoped per F-11) — done (wave 0)
- 11 Article 3 — "Writing specs that can converge" — done (wave 3)
- 12 Model-tier gate for brief authoring — done (wave 0)
- 13 Name the methodology — done (wave 0)
- 14 CLAUDE.md + skill-description diet — done (wave 1)
- 15 STATUS.md single-writer — lint on PRs, regen on main — done (wave 0)
- 16 Non-self-writable lifecycle — register integrity + machine-attributable gates — done (wave 1)
- 17 Un-forgeable PR review gate — dedicated reviewer identity + GitHub approval gating — done‡ (wave 0)
- 18 DORA metrics — instrument outcomes (lead time + stability), not just merge throughput — done (wave 1)
- 19 Verification-gate hardening — risk-keyed verifier floor + Verify-table structure lint — done (wave 0)
- 20 Fix-briefs sweep the defect class — authoring rule + reviewer questions — done (wave 0)
- 21 Mechanical isolation backstop — main-commit guard hook + dispatch-isolation protocol (F-13) — done (wave 0)
- 22 Single-home the operating rules — desk skills into the repo, reconcile doc-vs-practice drift — done (wave 0)
- 24 gate-why backfill — brief-specific rationale for every risk-gated brief (phase 2) — done‡ (wave 0)
- 25 gate-why hard lint — flip the missing-gate-why NOTICE to a PROBLEM (phase 3) — done (wave 1)
- 23 INTAKE + FINDINGS become directories of per-entry files with a generated view (I-21) — done (wave 0)
- 26 Authoring freshness check — deliverable not already satisfied on main — done (wave 0)
- 27 Every brief carries a `why:` — human-justifiable motivation — done (wave 0)
- 28 The coordinator desk dispatches reviews as issues — never runs code/security review inline — done (wave 0)
- 30 Security-review gate — verdict convention + risk-classed dispatch rule (#216) — done‡ (wave 0)
- 31 statusgen — security-review recorded at done for risk-classed briefs, NOTICE (#216) — done (wave 1)
- 32 Worker PR watch-loop — mergeable alongside reviews (#212) — done (wave 0)
- 33 Register references become links — author-brief convention + 94-brief backfill + lint — done (wave 1)
- 34 LinkedIn article — the model mix: cheap models, work-unit design, and gates — done (wave 2)
- 35 Register IDs become letter-prefixed slugs (F-<slug>, 10-20 chars) — no counter, no collisions — done (wave 1)
- 36 Register tombstone check scopes to origin/main — branch-only cleanup allowed (#269) — done (wave 0)
- 37 CLAUDE.md word budget as a lint gate — statusgen --budget + CI wiring (#280) — done (wave 0)
- 38 Cadence-compression research — re-clock every weekly/monthly loop against measured velocity — done (wave 0)

### methodology-metrics (23 done)

- 01 Status-transition historian — statusgen logs every status change with a timestamp — done (wave 0)
- 02 DORA metrics emitter — `statusgen --dora` (5 metrics from the historian + git/gh + verify-desk) — done (wave 1)
- 03 `statusgen --trend` — the SCADA historian view over time — done (wave 1)
- 04 Point-quality rendering — an unverified `done*` visibly distinct from evidence-backed `done` — done (wave 0)
- 05 FINDINGS alarm KPIs — rate, standing-alarm age, flood detection (ISA-18.2) — done (wave 1)
- 06 Next-up span-of-control + overflow-as-alarm (EEMUA-191) — done (wave 0)
- 07 Two written invariants — "Observe ∝ Act" and "Orient integrity is paramount" — done (wave 0)
- 08 Next-up claim-aware — exclude briefs with an open branch/PR (#156) — done (wave 0)
- 09 Dependency-precise Next-up eligibility — gate on typed depends, not whole-wave completion — done (wave 0)
- 10 Verification-debt as a first-class alarm — Awaiting-queue depth/ratio on the board — done (wave 0)
- 12 Surface the verified-stage human gate — irreversible briefs get a sign-off issue at implemented (#231) — done (wave 1)
- 14 Next-up value term — explicit value field + held-up-by count in the score (F-09, R-01) — done (wave 1)
- 15 Human-stamp corroboration — human:<name> additions need that account's on-PR action (#237) — done (wave 1)
- 11 Gate-queue prioritization — review and verify loops drain by brief priority, not arrival order — done (wave 1)
- 16 DORA time series — --dora --series buckets CFR/frequency/lead-time per ISO week — done (wave 1)
- 17 Verify-desk lands results as verifiers return — continuous drain, not wave-end batches (#282) — done (wave 0)
- 19 Code-efficiency metrics — --code: SLOC/churn/defect-density/review-depth, ledger-sourced — done (wave 1)
- 20 Approved-idle alarm + merge-when-green — approval is perishable when main outruns the merge — done (wave 1)
- 23 Roadmap deck page 1 — --roadmap: all-streams portfolio grid (computed health, fixed order) — done (wave 1)
- 26 DORA breakdowns — --dora --by stream/goal: the four metrics per stream and per product goal — done (wave 1)
- 22 Daily artifact harvest — scheduled AI-free collector commits the day's metrics/board artifacts — done (wave 1)
- 31 Path-scope the repo-infra lint checks — a finance-app deploy check must not red-gate an unrelated doc PR — done (wave 0)
- 32 Product-scoped statusgen — reuse `serves:` + `--scope <product>` so each product lints only its own streams — done (wave 1)

## Totals

**15** streams (**15** active, **0** paused) · **93/177** briefs done · completed initiatives: see `docs/archive/`
