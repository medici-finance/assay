---
brief: methodology/39
title: Defense-in-depth as the default authoring posture — layered designs for core-system briefs
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by Fable session (number 38 taken on this branch; a concurrent methodology-stream brief (cadence research) is being authored on another branch — expect a possible brief-number race, renumber the loser at merge)
sources: ["human:<name> 2026-07-12: \"when writing briefs, we should be choosing defense in depth. especially in the core system.\"", "CLAUDE.md Important Constraints §3 (pure verification — the anti-assert-spam boundary)", "methodology/24–25 (gate-why NOTICE→hard-lint precedent for the MAY hook)", "methodology/22 (rule-ownership) + methodology/26 (out-of-repo mirror pattern)", "issue #221 (out-of-repo edit protocol)", "freshness-checked 2026-07-12 @ d0222490 (no defense-in-depth/layered/single-point rule in either skill body)"]
why: >-
  Briefs today can pass every gate with a design where ONE control stands between a fault
  and funds/auth damage — the gates check that a design is verified and reviewed, not that
  it is layered. The repo's best mechanisms are already defense-in-depth (CircuitBreaker
  asserted on-ledger, settlement checks + bridgeCallerToken + reconciler each backstopping
  the layer above); this brief makes that posture the authoring DEFAULT for core-system
  work instead of an accident of good taste.
---

# Brief 39 — Defense-in-depth as the default authoring posture (core system)

## Context
files: `../oit/.claude/skills/author-brief/SKILL.md` (in-repo project layer — gains the
core-system definition, exemplar table, reviewer questions),
`docs/streams/methodology/evidence/brief-39-user-level-delta.md` (planned) (in-repo record
of the out-of-repo edit, mirroring brief-22's delta-file pattern),
`docs/streams/methodology/README.md` (row)

out-of-repo files: `~/.claude/skills/author-brief/SKILL.md` (user-level core — gains the
general rule). Per the #221 protocol this declaration IS the claim — max ONE such claim in
flight; apply the live edit LAST (immediately before setting `implemented`) and commit it
in the `~/.claude` stopgap repo. It cannot ride this repo's commit — the delta file above
is its in-repo shadow.

facts:
- **The rule (normative content — transcribe, don't redesign):** when a brief's design has
  alternatives, prefer the LAYERED one: no single control between a fault and the damage.
  Mandatory for **core-system** briefs; recommended everywhere.
- **Core system, defined for this repo** (≈ the `gate: human` / any-`risk: yes`
  territory): (1) DAML templates on settlement/funds paths — vault/pool balances, the six
  settlement choices, commission/fee flows; (2) the auth/identity chain — Keycloak realm,
  token minting/validation, identity-reconciler, Canton user rights; (3) the Ledger
  Service boundary — the only TS→Canton path, including `bridgeCallerToken` and plane
  tagging. A brief touching any of these is a core-system brief.
- **What a core-system brief must carry** (the three authoring obligations):
  (a) a **single-point-of-failure note in Context** — one or two lines answering "what is
  the ONE control this design depends on?" If the honest answer is one thing, the design
  needs another layer before the brief is authored;
  (b) **at least two INDEPENDENT layers in the Task** where feasible — independent means
  they fail for different reasons in different components (e.g. on-ledger assertion +
  service-side validation + alarm/monitor). This repo's exemplars, cite them in the skill:
  CircuitBreaker (`../oit/daml/OptionIndex/Governance.daml`) asserted by every trading choice —
  Canton rejects even if the frontend is bypassed; the settlement C-01/N-2/REG-03 +
  min-attestation checks preserved through the PublishedPrice authority migration;
  `bridgeCallerToken` per-handler + reconciler-enforced rights (60s converge) + agent-SDK
  boot hard-fail — three planes catching the same class of identity fault; the
  pure-verification closed-form invariants alongside on-ledger assertions;
  (c) **Verify rows that exercise each layer INDEPENDENTLY**, including at least one
  **negative-path row**: demonstrate the lower layer catches the fault when the layer
  above it is bypassed or broken (e.g. submit past the service straight to the ledger and
  watch the DAML assertion reject).
- **Boundary (anti-assert-spam):** CLAUDE.md Important Constraints §3 stands — "pure
  verification: invariant checks are closed-form math, not runtime assertions". Layers are
  DESIGNED controls at distinct trust boundaries, not the same check pasted into N places.
  Duplicating one assertion is redundancy, not depth; the independence test in (b) is what
  distinguishes them.
- **Residence (per the CLAUDE.md placement rule):** the general rule is authoring
  methodology → user-level core (out-of-repo, above); the core-system definition +
  exemplars + reviewer questions are repo-specific → in-repo project layer (rides the PR).
  **No CLAUDE.md line**: only brief authors need this rule, skill bodies load on invoke,
  and the word budget (methodology/14 cap, methodology/37 gate) is under standing
  pressure. If a future finding shows non-authoring sessions violating the posture, a
  compressed one-liner can be proposed then — argued here so the reviewer sees it was
  weighed, not missed.
- The gate-why precedent (methodology/24 → 25): a new authoring obligation starts as an
  advisory NOTICE and hardens to a lint only after backfill. The MAY hook in Task 4
  mirrors exactly that path — it is NOT the deliverable.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Out-of-repo edit order (#221): in-repo commits first, the `~/.claude` live edit LAST
  before reporting `implemented`, committed in the `~/.claude` stopgap repo.

## Task
1. **User-level core** (`~/.claude/skills/author-brief/SKILL.md`, applied LAST per Ground
   rules): add a short "Defense in depth — the default design posture" section under the
   template rules: prefer the layered alternative; the three obligations (SPOF note in
   Context, ≥2 independent layers where feasible, per-layer Verify rows incl. one
   negative-path); the independence test (different failure reasons, different
   components); the anti-assert-spam boundary; a pointer that project layers define their
   own "core system". Keep it general — no medici/repo specifics in the user-level file.
2. **In-repo project layer** (`../oit/.claude/skills/author-brief/SKILL.md`): add a
   "Defense-in-depth rule — core system (methodology/39)" section carrying the concrete
   core-system definition, the exemplar table from Context facts (b), the SPOF-note
   format, and two reviewer questions for the Review gate: "what is the single control
   between fault and damage, and is that acceptable?" and "does any Verify row prove a
   lower layer catches the fault with the upper layer bypassed?".
3. **Delta record**: write
   `docs/streams/methodology/evidence/brief-39-user-level-delta.md` (planned) — the exact
   text added to the user-level file, so the out-of-repo edit is reconstructable from the
   repo (brief-22 pattern).
4. **MAY (scoped, not the core deliverable)**: a statusgen advisory NOTICE when a brief
   with any `risk.*: yes` lacks a `layers:` frontmatter field or a `single-point-of-failure`
   token in its Context — mirroring the gate-why NOTICE (methodology/24) before any hard
   lint (methodology/25's job, later, separate brief). Only if cheap and it weakens no
   existing check; if skipped, record the decision in the PR body. No backfill of existing
   briefs either way.
5. README row; lint green.

## Verify (executable — no prose-only DoD items)
Presence gates ([F-10](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verify-tables-for-prose-deliverables-are-presence-checks-not.md)): these rows check the required elements EXIST; quality of the rule
text is owned by the review gate.
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -ci "defense.in.depth" .claude/skills/author-brief/SKILL.md` | ≥1 (project-layer section present) |
| 2 | `grep -c "single-point-of-failure\|single point of failure" .claude/skills/author-brief/SKILL.md` | ≥1 (SPOF-note obligation documented) |
| 3 | `grep -ci "negative-path\|negative path" .claude/skills/author-brief/SKILL.md` | ≥1 (per-layer/negative-path Verify obligation documented) |
| 4 | `grep -ci "defense.in.depth" ~/.claude/skills/author-brief/SKILL.md` | ≥1 (user-level core carries the general rule — live file, cannot ride the commit) |
| 5 | `test -f docs/streams/methodology/evidence/brief-39-user-level-delta.md && grep -ci "defense.in.depth" docs/streams/methodology/evidence/brief-39-user-level-delta.md` | ≥1 (in-repo shadow of the out-of-repo edit) |
| 6 | `statusgen --root . --lint; echo $?` | 0 |

**Deferred first-use check (not an executable row):** the first core-system brief authored after this merges must carry the single-point-of-failure note and ≥1 negative-path Verify row; the verify-desk confirms it as part of that brief's verification and cites it in THIS brief's Evidence.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. Reviewer confirms: the user-level text stays repo-agnostic; the in-repo
section defines core system concretely (DAML settlement/funds, auth/identity chain,
Ledger Service boundary) and cites real exemplars; the anti-assert-spam boundary is
stated (pure verification preserved); the #221 out-of-repo protocol was followed (edit
last, stopgap-repo commit, delta file present); the MAY hook, if taken, is NOTICE-only
and weakens no existing check.
