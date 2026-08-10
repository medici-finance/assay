---
brief: methodology/29
title: exec-tier — complex briefs signal a minimum execution-model tier; dispatch enforces it
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session (human:<name> direction)
sources: ["human:<name> 2026-07-10: some briefs are going to be complex — the author brief should signal this and force the brief only to be handled by a larger model", "methodology/05 (stream-level tiering: field — the guidance-only precursor)", "CLAUDE.md brief-completion protocol (effort-keyed execution tiering)", ".claude/skills/author-brief/SKILL.md model-tier gate (the authoring-side mirror)", "issue #221 (out-of-repo skill edit protocol)", "freshness-checked 2026-07-10 @ fb9223ce"]
why: >-
  effort keys SIZE, not required capability: today's dispatch default sends every M/L
  implementation to a cheap-tier worker, so a subtle-but-small brief (cross-component
  reasoning, design judgment the facts can't fully pre-specify, safety-critical code)
  has no way to demand a strong implementer. Complexity failures then surface only at
  the review gate — the most expensive place to catch them — or worse, pass it.
  Authoring already has this protection (the author-brief model-tier gate); execution
  does not.
---

# Brief 29 — exec-tier: per-brief minimum execution tier

## Context
files: `../assay-toolkit/statusgen/` (frontmatter parse + lint + Next-up render),
`../oit/.claude/skills/author-brief/SKILL.md` (in-repo wrapper — derivation guidance),
`CLAUDE.md` (brief-completion protocol tiering paragraph);
out-of-repo per issue #221 protocol: `~/.claude/skills/author-brief/SKILL.md` (core template),
`~/.claude/skills/batch-fanout/SKILL.md` (dispatch model choice)
facts:
- Field: `exec-tier: any | strong` — OPTIONAL brief-v1 frontmatter, absent = `any`
  (additive; no schema bump; mirrors gate-why's phase-1 pattern).
- DERIVED, not chosen (mirror of the gate/risk rule). Author answers three complexity
  questions; any yes → `exec-tier: strong`:
  1. Does the Task require design decisions the facts do not fully pre-specify?
  2. Does correctness depend on cross-component/cross-artifact reasoning (shared values,
     end-to-end flows, sweeping a pattern across sites)?
  3. Is it code where a subtle implementation error survives the brief's own tests
     (auth, funds, concurrency, safety plumbing)?
  A strong brief SHOULD carry a one-line `exec-tier-why`; statusgen NOTICEs its absence
  (same phasing as gate-why: NOTICE now, hard lint later if it earns it).
- Relation to methodology/05's stream-level `tiering:`: that field is free-text
  dispatcher GUIDANCE for a whole stream; exec-tier is per-brief and carries teeth at
  two points. Precedence: brief exec-tier overrides stream tiering; absent exec-tier
  falls back to stream tiering, then the effort-keyed default.
- Enforcement point 1 — DISPATCH (the real control): batch-fanout and any desk dispatch
  pick the worker model from exec-tier: `strong` → session-tier model (opus+/fable-class),
  never an economy tier; `any` → cheap-tier per the existing default. The desk chooses the
  model, so this point is mechanically real.
- Enforcement point 2 — PICKUP STOP (the mirror of author-brief's model-tier gate): a
  cheap/economy-tier session that finds itself holding an `exec-tier: strong` brief STOPS,
  reports which model it is, and hands back — it does not implement "just this once".
  Exact same rationale: a cheap draft anchors the strong model that reviews it.
- Honest limitation ([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md) scope-honesty, same wording discipline as methodology/05's
  paragraph): statusgen never verifies which model actually ran; pickup-side compliance is
  honor-system self-report. The enforced surface is desk dispatch + the review gate. Say
  so in every doc this brief touches; do not overclaim.
- exec-tier is NEVER a Next-up score input ([F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md) scope note) — it is rendered as a marker,
  not weighed.
- Backfill is desk-confirmed, not unilateral: the implementer proposes candidates from
  existing `todo` briefs (any brief answering yes to a derivation question); the desk
  confirms on the PR which rows take the field. Adding exec-tier to a todo brief changes
  no Verify table (mid-flight tweak rule: just do it once confirmed).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- Out-of-repo skill files follow issue #221's protocol: declare the exact paths, apply the
  live edit as the LAST step before `implemented`, paste before/after diffs into the PR body.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. statusgen: parse `exec-tier` (default `any`); invalid value → lint PROBLEM naming the
   field and the allowed values; `strong` without `exec-tier-why` → NOTICE. Render a
   `[exec:strong]` marker on Next-up rows and the Awaiting-verification queue.
2. `../oit/.claude/skills/author-brief/SKILL.md` (in-repo wrapper): add the three derivation
   questions + the template line, and the honest-limitation sentence.
3. `CLAUDE.md` brief-completion protocol: amend the effort-keyed tiering sentence —
   effort keys inline-vs-dispatch; **exec-tier keys the dispatch model**; include the
   pickup-STOP rule in one line.
4. Out-of-repo (per #221): author-brief core template gains the field + derivation
   questions; batch-fanout dispatch template gains the model-choice rule and the
   pickup-STOP text for its worker prompts.
5. Backfill proposal: list existing `todo` briefs answering yes to any derivation
   question in the PR description; apply the field to the rows the desk confirms.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes invalid-exec-tier and strong-marker tests |
| 2 | `grep -c "exec-tier" .claude/skills/author-brief/SKILL.md` | ≥1 |
| 3 | `grep -c "exec-tier" CLAUDE.md` | ≥1 |
| 4 | PR body contains before/after diffs of both out-of-repo skill edits (#221 protocol) | present |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence

### Non-implementer verify — VERIFY: PASS (R4 partial) — glm-5.2-verifier, 2026-07-24

Isolated worktree `/private/tmp/vrf-meth-trio` off `origin/main` `e890be13`. PR **#659** (merged
2026-07-17). R1 (unit-test row) F-34 guard-blocked (`go test ./tools/statusgen` — statusgen token;
dispatched agents are shared-checkout-homed too; no `--root` anchor for `go test`) — UNRUN, not
assumed. Corroboration: `--lint` exit 0 (compile+run); `brieffile_test.go` read on main —
`TestExecTier` (5 subtests: invalid-value-PROBLEM, strong-without-why-NOTICE, strong-with-why-clean,
any-clean, valid-parse) + `TestExecTierMarker` ([exec:strong] renders in Next-up + Awaiting,
suppressed for `any`); 4 testdata fixtures (brief-70..73). The `[exec:strong]` markers are live on
the board today (several Awaiting rows carry them), proving the render path works end-to-end.

| # | command | exit | result | date | runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen -count=1` | BLOCKED | UNRUN — F-34 writeguard (statusgen token); corroborated: lint exit 0, TestExecTier(5)+TestExecTierMarker read on main, `[exec:strong]` markers live on the board | 2026-07-24 | glm-5.2-verifier |
| 2 | `grep -c "exec-tier" .claude/skills/author-brief/SKILL.md` | 0 | PASS — count **4** (field + 3 derivation questions + F-08 limitation) | 2026-07-24 | glm-5.2-verifier |
| 3 | `grep -c "exec-tier" CLAUDE.md` | 0 | PASS — count **1** (brief-completion protocol: effort + exec-tier keys dispatch model; pickup-STOP) | 2026-07-24 | glm-5.2-verifier |
| 4 | PR body has before/after diffs of BOTH out-of-repo skill edits (#221) | — | PARTIAL — PR #659 carries the author-brief out-of-repo diff; the batch-fanout edit landed in its IN-REPO canonical home (`../oit/.claude/skills/batch-fanout/SKILL.md`, where its canonical body lives) rather than as a pasted out-of-repo diff. Both substantive edits delivered; form is 1-of-2 pasted | 2026-07-24 | glm-5.2-verifier |
| 5 | `--lint; echo $?` | 0 | PASS — exit 0 | 2026-07-24 | glm-5.2-verifier |

**VERIFY: PASS** (R2/R3/R5 clean; R1 guard-blocked + corroborated; R4 substance delivered, form
1-of-2). Status flipped `implemented → verified` (gate: model, risk all-no).

## Review
Gate: model. Reviewer confirms (a) the derivation questions are in BOTH author-brief homes
(in-repo wrapper and core, the latter via pasted diff), (b) the dispatch rule names the model
choice mechanically (not "prefer"), (c) no doc overclaims enforcement beyond dispatch+review
([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md) wording discipline), (d) exec-tier stays out of Next-up scoring.
