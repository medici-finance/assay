---
stream: methodology
serves: assay
status: active
priority: P1
track: platform
---

# Methodology Stream

Sharpen the initiative-streams system itself and publish it. Three sub-goals, one
stream: (a) adopt the five research-verified mechanisms from the design spec §11
(evidence enforcement, risk-gated done, findings demotion, pre-flight validation,
tiering-as-policy) plus the deny-hooks enforcement layer; (b) extract the portable
toolkit so the methodology is adoptable outside this repo; (c) write and publish the
three-article set (system / architecture / practice). Scoping inputs:
`../oit/docs/superpowers/specs/2026-07-08-initiative-streams-design.md` §11–§13 and INTAKE
I-02 / I-04 / I-05.

**Publication gate (applies to briefs 09/10/11):** articles publish only after lived
Next-up/FINDINGS/RETRO data exists (earliest R-01 retro: 2026-07-15) and only on a
human's explicit go — drafting may start immediately.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [statusgen v1.1 — brief-file schema + pre-flight validation](./brief-01-statusgen-brief-schema.md) | 0 | M | done | 2026-07-08 sonnet-verifier | 2026-07-08 model:sonnet + /review on PR #77 |
| 02 | [Evidence enforcement at the verified gate](./brief-02-evidence-enforcement.md) | 1 | S | done | 2026-07-08 verified (independent re-run) | 2026-07-08 /review approve (model) |
| 03 | [Risk-gate enforcement at the done gate](./brief-03-risk-gate-enforcement.md) | 1 | S | done | 2026-07-09 fable-verifier (independent re-run on a4e3e04b) | 2026-07-09 model:opus (review gate) |
| 04 | [Findings demotion — scope-change re-entry](./brief-04-findings-demotion.md) | 0 | S | done | 2026-07-09 opus-verifier (independent re-run) | 2026-07-09 model:opus |
| 05 | [Tiering as per-stream policy](./brief-05-tiering-policy.md) | 0 | S | done | 2026-07-09 opus-verifier (independent re-run on 37c0eab2) | 2026-07-09 model:opus (close-out review) |
| 06 | [Implementer deny-hooks (enforcement layer c)](./brief-06-implementer-deny-hooks.md) | 0 | S | done | 2026-07-09 opus-verifier (independent re-run on 37c0eab2) | 2026-07-09 model:opus (close-out review) |
| 07 | [Toolkit extraction — standalone repo](./brief-07-toolkit-extraction.md) (superseded by assay-dogfood/02) | 2 | L | done | 2026-07-09 opus-verifier (independent re-run on 07bcecaa) | 2026-07-09 human:ian; accepted 2026-07-10 human:ian |
| 08 | [RETRO bootstrap + first retro (R-01)](./brief-08-retro-bootstrap.md) | 0 | S | done | 2026-07-10 opus-verifier | 2026-07-10 reviewer-app[bot] |
| 09 | [Article 1 — "Status is a build artifact"](./brief-09-article-status-build-artifact.md) | 2 | M | in-progress | — | — |
| 10 | [Article 2 — "Prevention and reconciliation" (rescoped per F-11)](./brief-10-article-convergence-thesis.md) | 0 | M | done | 2026-07-09 opus-verifier (independent re-run on 37c0eab2) | 2026-07-09 human:ian; accepted 2026-07-10 human:ian |
| 11 | [Article 3 — "Writing specs that can converge"](./brief-11-article-convergable-specs.md) | 3 | M | done | 2026-07-13 opus-verifier | 2026-07-18 human:ian |
| 12 | [Model-tier gate for brief authoring](./brief-12-authoring-model-gate.md) | 0 | S | done | 2026-07-08 sonnet-verifier | 2026-07-08 model:sonnet |
| 13 | [Name the methodology](./brief-13-name-the-methodology.md) | 0 | S | done | 2026-07-09 opus-verifier (independent re-run on 07bcecaa) | 2026-07-09 human:ian; accepted 2026-07-10 human:ian |
| 14 | [CLAUDE.md + skill-description diet](./brief-14-claude-md-diet.md) | 1 | S | done | 2026-07-10 opus-verifier | 2026-07-11 reviewer-app[bot] |
| 15 | [STATUS.md single-writer — lint on PRs, regen on main](./brief-15-status-single-writer.md) | 0 | M | done | 2026-07-08 opus-verifier (live regen fired ×2) | 2026-07-08 /review approve (model) · 2026-07-09 model:opus (close-out review) |
| 16 | [Non-self-writable lifecycle — register integrity + machine-attributable gates](./brief-16-nonselfwritable-lifecycle.md) | 1 | M | done | 2026-07-09 opus-verifier | 2026-07-09 model:opus |
| 17 | [Un-forgeable PR review gate — dedicated reviewer identity + GitHub approval gating](./brief-17-unforgeable-review-gate.md) | 0 | M | done | 2026-07-09 opus-verifier (independent re-run on 07bcecaa) | 2026-07-10 human:ian |
| 18 | [DORA metrics — instrument outcomes (lead time + stability), not just merge throughput](./brief-18-dora-metrics.md) | 1 | M | done | 2026-07-10 opus-verifier | 2026-07-10 reviewer-app[bot] |
| 19 | [Verification-gate hardening — risk-keyed verifier floor + Verify-table structure lint](./brief-19-verification-floor.md) | 0 | M | done | 2026-07-10 opus-verifier | 2026-07-10 reviewer-app[bot] |
| 20 | [Fix-briefs sweep the defect class — authoring rule + reviewer questions](./brief-20-class-sweep-rule.md) | 0 | S | done | 2026-07-10 opus-verifier | 2026-07-12 reviewer-app[bot] |
| 21 | [Mechanical isolation backstop — main-commit guard hook + dispatch-isolation protocol (F-13)](./brief-21-isolation-backstop.md) | 0 | M | done | 2026-07-10 opus-verifier | 2026-07-10 reviewer-app[bot] |
| 22 | [Single-home the operating rules — desk skills into the repo, reconcile doc-vs-practice drift](./brief-22-single-home-operating-rules.md) (superseded by assay-dogfood/02) | 0 | M | done | 2026-07-12 opus-verifier | 2026-07-12 reviewer-app[bot] |
| 24 | [gate-why backfill — brief-specific rationale for every risk-gated brief (phase 2)](./brief-24-gate-why-backfill.md) | 0 | M | done | 2026-07-10 opus-verifier | 2026-07-12 reviewer-app[bot] |
| 25 | [gate-why hard lint — flip the missing-gate-why NOTICE to a PROBLEM (phase 3)](./brief-25-gate-why-hard-lint.md) | 1 | S | done | 2026-07-12 opus-verifier | 2026-07-12 reviewer-app[bot] |
| 23 | [INTAKE + FINDINGS become directories of per-entry files with a generated view (I-21)](./brief-23-registers-as-directories.md) | 0 | L | done | 2026-07-10 opus-verifier | 2026-07-11 reviewer-app[bot] |
| 26 | [Authoring freshness check — deliverable not already satisfied on main](./brief-26-authoring-freshness-check.md) | 0 | S | done | 2026-07-12 opus-verifier | 2026-07-12 reviewer-app[bot] |
| 27 | [Every brief carries a `why:` — human-justifiable motivation](./brief-27-why-field.md) | 0 | S | done | 2026-07-13 opus-verifier | 2026-07-12 reviewer-app[bot] |
| 28 | [The coordinator desk dispatches reviews as issues — never runs code/security review inline](./brief-28-desk-dispatches-reviews.md) | 0 | S | done | 2026-07-20 glm-5.2-verifier | 2026-07-20 human:ian |
| 29 | [exec-tier — complex briefs signal a minimum execution-model tier; dispatch enforces it](./brief-29-exec-tier.md) | 0 | S | verified | 2026-07-24 glm-5.2-verifier | — |
| 30 | [Security-review gate — verdict convention + risk-classed dispatch rule (#216)](./brief-30-security-review-gate.md) | 0 | S | done | 2026-07-27 k3-verifier | 2026-07-27 human:ian |
| 31 | [statusgen — security-review recorded at done for risk-classed briefs, NOTICE (#216)](./brief-31-security-review-done-lint.md) | 1 | S | done | 2026-07-24 glm-5.2-verifier | 2026-07-18 assay-reviewer-app[bot] |
| 32 | [Worker PR watch-loop — mergeable alongside reviews (#212)](./brief-32-worker-mergeable-watch.md) | 0 | S | done | 2026-07-12 opus-verifier | 2026-07-12 reviewer-app[bot] |
| 33 | [Register references become links — author-brief convention + 94-brief backfill + lint](./brief-33-register-reference-links.md) | 1 | M | done | 2026-07-18 glm-5.2-verifier | 2026-07-20 human:ian |
| 34 | [LinkedIn article — the model mix: cheap models, work-unit design, and gates](./brief-34-article-model-mix.md) | 2 | M | done | 2026-07-27 k3-verifier | 2026-07-27 human:ian |
| 35 | [Register IDs become letter-prefixed slugs (F-<slug>, 10-20 chars) — no counter, no collisions](./brief-35-register-slug-ids.md) | 1 | M | done | 2026-07-18 glm-5.2-verifier | 2026-07-20 human:ian |
| 36 | [Register tombstone check scopes to origin/main — branch-only cleanup allowed (#269)](./brief-36-register-tombstone-scope.md) | 0 | S | done | 2026-07-11 opus-verifier | 2026-07-11 human:ian |
| 37 | [CLAUDE.md word budget as a lint gate — statusgen --budget + CI wiring (#280)](./brief-37-claude-md-word-budget-lint.md) | 0 | S | done | 2026-07-12 opus-verifier | 2026-07-12 reviewer-app[bot] |
| 38 | [Cadence-compression research — re-clock every weekly/monthly loop against measured velocity](./brief-38-cadence-compression-research.md) | 0 | M | done | 2026-07-18 human:ian | 2026-07-18 human:ian |
| 39 | [Defense-in-depth as the default authoring posture — layered designs for core-system briefs](./brief-39-defense-in-depth-authoring.md) | 0 | M | todo | — | — |
| 44 | [Verify-command #509 sweep — fix every unfailable grep/go-test row + close the detection gap](./brief-44-verify-command-509-sweep.md) | 0 | M | implemented | — | — |
| 45 | [why: backfill (phase 2) — add why: to the 32 live briefs; grandfather the 75 closed](./brief-45-why-backfill-live-only.md) | 0 | M | todo | — | — |
| 46 | [Desk rename pass — batch-fanout→worker-desk + issue-loop→intake-desk (four-loop taxonomy)](./brief-46-desk-rename-pass.md) | 1 | M | todo | — | — |
| 40 | [Multi-repo dispatch — batch-fanout reads every product repo's Next-up board](./brief-40-multirepo-fanout-dispatch.md) | 0 | M | todo | — | — |
| 41 | [Phase-1 board provisioning — reconciler and platform-repo generate a board from the pinned statusgen release](./brief-41-board-provisioning-single-source.md) | 0 | M | todo | — | — |
| 42 | [GitHub-durable dispatch claim — replace the machine-local batch-fanout claim lock](./brief-42-github-durable-dispatch-claim.md) | 0 | M | todo | — | — |
| 43 | [Intra-brief file-scoped parallelism — split one large brief across concurrent workers](./brief-43-intra-brief-parallelism.md) | 1 | L | todo | — | — |
| 47 | [Findings register becomes a corroborated state machine — bounded shelving + transition guard (closes #721 + toothless-park, folds F-36)](./brief-47-findings-register-state-machine.md) | 0 | L | todo | — | — |

## Self-critique (2026-07-09 methodology red-team)

An adversarial Fable review of the methodology landed 2026-07-09 (stream E of the
needs-fixing genius sweep). It is the strongest input this stream has. **Full report (the
FATAL/WOUNDING/SURVIVABLE argument behind each finding, and the article-by-article
breakdown): [`red-team-2026-07-09.md`](./red-team-2026-07-09.md)** — the source for the
article briefs' pre-empted-objection sections. Distilled into findings **F-08…F-12**,
flagging the briefs they hit:
- **F-08** (the load-bearing objection): "status is measured, not self-reported" is false in
  its strong form — the sensors are agent-writable. → brief-16 is the fix; brief-09 must reframe.
- **F-09**: Next-up scoring lacks a value/effort term + FINDINGS-demotion is an unverified DoS.
  → brief-04 amendment (desk-ack before demoting in-progress) + R-01 knob.
- **F-10**: prose Verify tables gate presence, not quality (grep-DoD). → briefs 09/10/11/02.
- **F-11**: Article 2's three domains collapse to one + the exemplar reconciler violates the
  thesis today. → brief-10 rescope + sequence behind needs-fixing fixes.
- **F-12**: the ≈30:1 leverage and tier "controlled experiment" are unsourced/uncontrolled.
  → briefs 09/10/11 must not publish the numbers without computation.

The reframe these point to is *better* than the original: **lead the articles with F-05 and
the falsification, not the wins** — a system built to distrust unreliable narrators, falsified
from the inside in week one, is a credible story; the triumphal version is prey.

**Second external review (2026-07-09, assay-review-1):** a full good/bad/ugly review of Assay
as operated — [`../oit/docs/assay-review-1/README.md`](https://github.com/example-org/oit/blob/main/docs/assay-review-1/README.md). Where the
red-team attacked the claims, this one audited the operations: verification economics, scheduler
semantics, and spec-vs-practice drift. Derived artifacts: briefs 19–22 here,
methodology-metrics/09–10, F-16, INTAKE I-17/I-18. Filed as candidates for the desk/R-01 to
sequence — not enacted inline (the one-change budget applies to enactment). Follow-on (human:<name>,
same day): the multi-person/SME growth design —
[`assay-growth-2026-07-09.md`](./assay-growth-2026-07-09.md), tracked as INTAKE I-19, scoping
input for a future `assay-adoption` stream.

**Resolved 2026-07-09** by the brief-amendment set (F-08…F-12 flipped in FINDINGS.md):
F-08 → brief-09 reframed + now depends on brief-16 (which remains the fix); F-09 → brief-04
desk-ack amendment + R-01 agenda seeds in brief-08 + brief-05 scope note; F-10 →
presence-gate Verify preambles on 09/10/11, brief-02 + conventions note; F-11 → brief-10
rescoped ("Prevention and reconciliation", publication behind needs-fixing C4/H10–H12);
F-12 → forbidden-numbers rules + grep-absence rows on 09/10/11.

## Critical path

```
01 (brief-file schema, opt-in) → 02 (evidence) → 07 (toolkit extraction) → 11 (article 3)
```

**Smallest unblocking move: brief 01** — everything enforcement-shaped (02/03) and the
extraction (07) build on statusgen parsing brief files.

**Verified head (the "real head" check):** a naive brief 01 that validates EVERY
brief file is dead-on-arrival — the ~55 legacy briefs across the other 11 streams
have no YAML frontmatter (verified: only briefs authored after the author-brief
upgrade carry `schema: brief-v1`). Brief 01 is therefore **opt-in by schema marker**:
files without frontmatter are exempt until migrated. That constraint is in the brief;
do not "fix" it by migrating all legacy briefs first — that's a separate, optional
tail of work, not the head.

## Dependency waves

```
Wave 0: [01, 04, 05, 06, 08, 10, 12, 13, 15]   [19, 20, 21, 22] (assay-review-1, 2026-07-09 — no pending deps)
Wave 1: [02, 03] ← 01   [14] (deliberately wave 1: let today's rules settle a few days first)   [16] ← 15
Wave 2: [07] ← 01,02,03   [09] ← 08,16
Wave 3: [11] ← 07,08
```

Critical path: 01 → 02 → 07 → 11. Article 1 (09) now also waits on brief-16 (F-08): the
"measured, not self-reported" claim must become true before it's published as true.

## Shared conventions

- **Rule-ownership (methodology/22):** every operating rule has exactly **one home** in this repo
  (skill body, README, or CLAUDE.md per the brief-14 placement rule). Other surfaces point, never
  restate. Session-memory is a cache, never the sole home of a load-bearing rule. When
  practice-vs-written-rule drift is found, reconcile it in the in-repo single home — do not let a
  user-level `~/.claude` file become the de-facto rule by drift. The desk skills (`batch-fanout`,
  `pr-review-desk`, `verify-desk`, `the-desk`) live in `.claude/skills/` with their Go tooling;
  user-level copies are thin pointers (replacement stubs in
  `docs/streams/methodology/evidence/brief-22-user-level-deltas.md`).
- **Dispatch isolation (methodology/21, F-13):** any subagent dispatch that will run shell/git in
  this repo passes `isolation: "worktree"` — prose "stay on your branch" instructions are NOT
  sufficient (F-13: a prose-told subagent committed 3 times to the shared checkout's `main` and
  self-reported success). After the subagent reports, the dispatcher VERIFIES landing with `git log
  --all --oneline | grep <expected-msg-substring>` rather than trusting the self-report. Mechanical
  backstop: the versioned `.githooks/pre-commit` refuses commits to `main` unless
  `ASSAY_MAIN_COMMIT_OK` is set — activate per clone with `git config core.hooksPath .githooks`;
  enacting it in the shared checkout is the desk's/R-01's call (recorded delta for the user-level
  batch-fanout skill; on enactment, F-13 flips `Resolved: yes`).
- **Two written invariants** ([`invariants.md`](./invariants.md)): *"Observe ∝ Act"* (observation
  effort must scale with action volume) and *"Orient integrity is paramount"* (the registers —
  STATUS/FINDINGS/Next-up — must be trustworthy above all). Cite them when a decision trades
  observation for speed, or when register trustworthiness is at stake.
- All statusgen work: repo-agnostic (no medici paths), stdlib + yaml.v3 only, TDD,
  `go test ./tools/statusgen/` + `go vet` + `--lint` exit 0 before commit.
- STATUS.md has a single writer — main's CI (`../oit/.github/workflows/status-regen.yml`)
  regenerates and commits it. NEVER commit STATUS.md on a branch; PRs run `--lint`
  (no drift compare) plus a guard blocking STATUS.md edits. On a local STATUS.md merge
  conflict, take either side and rerun `go run ./tools/statusgen` — never hand-merge a
  generated file.
- Checker changes NEVER weaken existing checks; new checks may be opt-in (schema
  marker) but never presence-conditional on the environment (the sibling-fallback
  lesson, final review 2026-07-08).
- Human-reviewer rule (methodology/03): a `gate: human` brief at `done` must have a
  Reviewed-column entry naming a human — format `human:<name>` (e.g. `2026-07-15
  human:ian`). A bare model sign-off does not close a risk-flagged brief; statusgen
  flags it.
- **Security-review token (methodology/30+31):** a risk-classed brief (frontmatter:
  `gate: human` OR any risk answer `yes`) at `done` must carry the literal substring
  `security-review` in its Reviewed cell (e.g. `2026-07-12 model:opus
  +security-review(pass)`). `statusgen` emits a NOTICE when missing (methodology/31);
  this is advisory this phase — a follow-on brief flips it to a hard error after
  backfill, mirroring the methodology/24-to-25 pattern for gate-why.
- Articles: source in `docs/articles/`, PDF/deck via the whitepaper-program pipeline
  in the decks repo (`../decks`, renderer `tools/render-whitepaper-pdf.cjs` there).
  Never create decks in this repo.
- Prose-deliverable Verify tables are **presence gates** (F-10): they check required
  elements exist, never quality — quality is owned by the human review gate. State this
  in the brief's Verify section; posing grep rows as quality DoD is checkmark-DoD.
  `statusgen` now ENFORCES presence (methodology/19): every brief-v1 file must carry a
  `## Verify` section with at least one `Command`/`Expect` table row — structure only, the
  lint never judges the row's content.
- **Class-sweep (methodology/20):** a fix that resolves an instance of a mechanical/greppable defect
  CLASS (a parse error returned as empty success, a timestamp-format mismatch, an unchecked type
  assertion) must sweep the whole class, not just the reported site. Enumerate the siblings with a
  literal grep, route each one (fixed-here / follow-up brief / out-of-scope-with-reason), and carry
  the grep as a Verify row. This binds non-brief fix work (issue-driven patches) too — an unlisted
  sibling is exactly how #147 (a third parse-error site left live, a fourth unrouted) and #104 (the
  same parse bug untouched in the policy/resolver_canton.go sibling) happened. Full rule + reviewer
  questions: `../oit/.claude/skills/author-brief/SKILL.md` § Class-sweep rule.
- Enforcement layer (c) boundary (methodology/06): mutating cluster/CI operations
  (kubectl apply/delete/edit/scale/rollout/patch, flux reconcile/suspend/resume, `gh
  workflow run`/`gh run rerun`, force-push) are mechanically denied repo-wide via
  `.claude/settings.json` `permissions.deny`. `git commit` and plain branch `git push`
  are deliberately NOT on that list — they stay process-governed (Ground rules + the
  I-12 review loop + review gates), because the coordinator and the sanctioned
  agent-run-branch-push + draft-PR loop legitimately use them.
- **Tiering (methodology/05 + verifier floor methodology/19):** default policy is cheap-tier
  implements; the VERIFICATION tier is **risk-keyed, not a blanket "strong-tier verifies"**. The
  floor is keyed on **capability, not price** (#298): a **risk-clear** brief (all four risk answers
  `no`, gate `model`) may be verified by any runner; a **risk-flagged** brief (`gate: human` OR any
  risk answer `yes`) must be verified by a human or a runner outside the below-floor model-family
  list — `statusgen` makes a below-floor runner in a risk-flagged brief's Verified cell a PROBLEM
  (methodology/19). The family list is a single explicit set matched by name segment, not a
  substring pattern, and it lives in exactly one place in code: `belowFloorModels` in
  `statusgen/attribution.go` (see that file for the current families and the rule for changing it —
  a family is added only for being too WEAK to verify, never for being cheap to run). Irreversible
  briefs are stricter still: a human must sign off before `verified` (#159), so the floor defers to
  that rule for them. **Verify-desk dispatch implication:** dispatch **opus+ or a human** verifier
  to risk-flagged briefs; a strong-but-cheap verifier (e.g. glm-5.2, which clears the floor on
  capability) is fine for risk-clear briefs. A stream
  may override the implement side via an optional free-text `tiering:` frontmatter field (e.g.
  `tiering: implement=sonnet verify=fable`); `statusgen` parses it, requires it non-empty when
  present (empty is a PROBLEM), and renders it in the STATUS.md roll-up's Notes column
  (`·`-separated from the external pointer when both are present). The `tiering:` field is
  dispatcher guidance only and is never a Next-up score input (F-09 scope note); the floor is a
  check on the WRITTEN Verified-cell runner, not proof of which model actually ran. Execution-side
  default is effort-keyed: Effort S may run inline at session tier; Effort M/L should be planned at
  your tier then DISPATCHED to cheap-tier subagents behind the verify/review gates (a session cannot
  downlevel itself). Role→tier table and full rationale: `../oit/.claude/skills/author-brief/SKILL.md`.
- Gate entries (Verified/Reviewed cells, Evidence rows) are landed by a commit from a
  NON-implementer. The methodology/16 convention calls for a `Verified-by: <id>` git trailer;
  statusgen never reads trailers — its checks are on the README cells: sequence-gap detection
  when a brief moves `implemented→verified→done` without filler commits between (sibling-brief
  adjacency), runner-attribution on the Verified cell + Evidence rows, and — at `done` — a
  dated-runner shape on the Reviewed cell (methodology/19). Every session currently commits as
  one shared git author (`shared-agent`), so these are best-effort, not a hard guarantee that the
  verifier was actually a different agent. Don't overclaim enforcement the tooling can't deliver
  (F-08 scope-honesty).
