---
brief: desk-tools/20
title: "Cross-repo triage/verify evidence binds to the remote — a sibling checkout must be cross-checked, not trusted as-is"
why: >-
  A triage or verify session that cites another repo's current state from a local sibling
  checkout (`~/<path>/<repo>`) had nothing telling it that checkout could be stale. A
  concrete case: a checkout 70 commits behind its remote was read as current and the
  resulting claim was explicitly badged "re-verified" — every cited detail (a file, line
  numbers, a since-removed code pattern) reproduced perfectly against the stale tree and
  matched nothing on the authoritative remote. The downstream work item then carried a fix
  request against code that no longer existed. Worse, `git fetch` against a sibling can
  itself fail silently (a rewritten remote, a dead credential), so "the desk fetched first"
  is not sufficient on its own — the fetch has to be cross-checked against an independent
  read of the same ref before its content is trusted.
wave: 1
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-06 by a worker session, from a cross-repo triage incident
sources:
  - "freshness-checked 2026-09-06 @ 2bc925a (origin/main) — `plugins/assay/skills/intake-desk/SKILL.md` §Shared desk rules has a `Refresh, don't remember` bullet about GitHub issue state but no rule at all about cross-repo/sibling-checkout state; `plugins/assay/skills/verify-desk/SKILL.md` §The loop has a `Sibling repos are in scope` clause that says to `Resync the sibling` before reading it but does not define resync as more than an attempted `git fetch` (no confirmation step), so a silent fetch failure passes unnoticed."
  - "The incident this generalizes: a sibling-repo grep read against a checkout 70 commits behind its remote, badged as a re-verified claim, reproduced every detail of the stale tree and none of the current one."
exec-tier: any
exec-tier-why: "(a) a bounded prose amendment to two already-published skill bodies, verified by
  grep-count and skillslint rather than by writing or changing any code."
---

# Brief 20 — Cross-repo evidence binds to the remote, not a bare sibling checkout

## Dependencies
None. Prose-only change to two already-published skill bodies; no code and no on-disk schema change.

## Context

files:
- `plugins/assay/skills/intake-desk/SKILL.md` (§Shared desk rules — new bullet)
- `plugins/assay/skills/verify-desk/SKILL.md` (§The loop — tightens the existing sibling-repo clause)
- changelog fragment intake-desk-cross-repo-remote-verify.md (rolled into `CHANGELOG.md` §v0.28.0 at release and cleared from `changelog/`)

facts:
- `intake-desk`'s shared-desk rules are the neutral, adopter-facing statement of desk-wide
  conventions (the file's own header: "Stated once for every desk; this skill adds only what is
  its own above"); `verify-desk` already has a NARROWER version of the same failure mode (its own
  `Sibling repos are in scope` clause, scoped to verifying a brief's cross-repo deliverables) but no
  general clause for an arbitrary cross-repo fact-check during triage.
- the incident's actual failure was not "no fetch happened" — a resync/fetch step existing in
  prose does not by itself prove freshness, because `git fetch` can fail silently under a
  rewritten remote (SSH `insteadOf`, a dead credential) and leave the tree exactly as stale as
  before the fetch ran; comparing a checkout's own `HEAD` to its own `origin/main` after such a
  fetch proves nothing, since a silently-stale fetch moves neither ref.
- the fix that survives a silent fetch failure is an INDEPENDENT cross-check of the same ref —
  comparing the checkout's `git rev-parse origin/main` against the forge API's own report of that
  branch's SHA (`gh api repos/<owner>/<repo>/commits/<branch> --jq .sha`), a different protocol
  that a git-transport-level rewrite does not also misroute.

## Ground rules
- Prose only. No new tooling, no new value, no behavior change to any binary in `tools/desk/`.
- Stop at `implemented` — this brief does not set verified/done.
- If anything here contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. Add a new bullet to `intake-desk`'s `## Shared desk rules` (immediately after `Refresh, don't
   remember`): cross-repo evidence must resolve against the repo's remote (a forge read) or a
   sibling checkout fetched AND cross-checked current in the same cycle via an independent read of
   the same ref; state the SHA a citation was checked against; unconfirmed is could-not-check,
   never a claim badged as re-verified.
2. Tighten `verify-desk`'s existing `Sibling repos are in scope` clause: define "resync" as
   confirmed-current, not merely attempted, with the same independent cross-check (git ref vs.
   forge API), because the identical silent-fetch-failure risk applies there.
3. One changelog fragment recording both.
4. Nothing else — no other skill body, no tooling change.

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/skillslint && go run . --root ../..; echo $?` | 0 — SKILLSLINT/HIDDEN-CHARS/HOUSE-VALUES/GUARDRAILS/ENFORCEMENT-BLOCK all PASS |
| 2 | check | `grep -c -iE 'independent read' plugins/assay/skills/intake-desk/SKILL.md` | ≥ 1 |
| 3 | check | `grep -c -iE 'confirmed-current' plugins/assay/skills/verify-desk/SKILL.md` | ≥ 1 |
| 4 | check | `git diff origin/main --stat -- plugins/assay/skills/` | only `plugins/assay/skills/intake-desk/SKILL.md` and `plugins/assay/skills/verify-desk/SKILL.md` touched |
| 5 | check | `test -s changelog/intake-desk-cross-repo-remote-verify.md && echo present` | `present` |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| Rule added but never says WHAT to compare (so a reader just re-runs `git fetch` and calls it done) | row 2 / row 3 (both require the independent-cross-check language, not just "fetch") |
| Rule lands somewhere a reader has to know to look (buried in a different section) | row 2/3 target the specific sections named in the Task |
| Change silently touches unrelated skill bodies | row 4 |
| A fragment is added with no real content (gate satisfiable by `touch`) | row 5 checks non-empty, and `changelog-check` (CI) additionally rejects an empty fragment |

## Evidence
### Non-implementer verifier run — VERIFY: PASS (5/5 rows, offline) — 2026-09-06 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `67abbac`

Runner ≠ implementer. Isolated worktree off origin/main. Offline (`KUBECONFIG=/dev/null`). Frontmatter: `gate: model`, all risk `no`, `irreversible: no`. Implementing commit `9cce854`.

| # | command | expected | exit / observed | Date | Runner |
|---|---------|----------|-----------------|------|--------|
| 1 | tools/skillslint go run . --root ../.. | exit 0; the five named checks PASS | exit 0 — skillslint / hidden-chars / house-values / guardrails / enforcement-block all PASS (6 advisory context-bloat NOTICEs, exit stays 0) | 2026-09-06 | opus-4.8[1m]-verifier |
| 2 | grep -c -iE 'independent read' plugins/assay/skills/intake-desk/SKILL.md | ≥1 | exit 0, count 1 | 2026-09-06 | opus-4.8[1m]-verifier |
| 3 | grep -c -iE 'confirmed-current' plugins/assay/skills/verify-desk/SKILL.md | ≥1 | exit 0, count 1 | 2026-09-06 | opus-4.8[1m]-verifier |
| 4 | git diff origin/main --stat plugins/assay/skills/ | only intake-desk + verify-desk SKILL.md touched | empty on merged main (HEAD==origin/main); intent cross-checked via git show 9cce854 — exactly intake-desk (+18) + verify-desk (+9), no other skill body | 2026-09-06 | opus-4.8[1m]-verifier |
| 5 | test -s changelog/intake-desk-cross-repo-remote-verify.md | present | exit 0, present (899 bytes, real content) | 2026-09-06 | opus-4.8[1m]-verifier |

`RISK-VALUE: N/A — enumeration over commit 9cce854 (two skill-body prose edits + one changelog fragment) found no literal constant/bound/threshold; the additions are prose, the only literals are illustrative doc command strings, none inside a guard. Reversible prose amendment; irreversible:no.`

**VERIFY: PASS** — all 5 Verify rows PASS on merged main `67abbac`; RISK-VALUE N/A. `gate: model`, all risk `no` — the deliverable verifies clean.

<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

## Review

Gate: model (all four risk answers no). The reviewer confirms: the new/tightened rules name an
INDEPENDENT cross-check (not just "run a fetch first"), the could-not-check fallback is explicit
(no silent trust of an unconfirmed checkout), and no other skill body or tool changed.

**Advisory obligation notices (mistake-proofing/03), addressed here rather than with a new row:**
`+dereference` and `+flow` are advisory, adequacy-is-reviewer's-call obligations. This brief's
deliverable is prose in a skill body, not a claim about external code or a runtime path that
crosses component boundaries — rows 2 and 3 already resolve the one checkable fact the brief
asserts (the rule's own wording is present, by grep, in the file it was added to), and there is
no cross-component control flow to exercise. If the reviewer disagrees, the fix is a Verify row,
not a frontmatter override.
