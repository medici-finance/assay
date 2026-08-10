---
brief: methodology/30
title: Security-review gate — machine-checkable verdict convention + risk-classed dispatch rule at the review desk
wave: 0
depends: []
unblocks: ["methodology/31"]
effort: S
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [216]
decision-issue: 728
schema: brief-v1
authored: 2026-07-10 by Fable authoring session (issue #216)
sources: ["issue #216", "[F-16](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verifier-tier-practice-contradicts-the-tiering-default-and-t.md) (same disease at the verify stage)", "CLAUDE.md 'Review-gate tooling' paragraph (the prose mandate being mechanized)", "methodology/17 (the App-verdict machinery this extends)", "daml-hardening/01 / PR #101 (the precedent: merged with no recorded /security-review)", "freshness-checked 2026-07-10 @ 5d529c27"]
gate-why: >-
  This brief DEFINES the gate that protects the repo's highest-risk change class (DAML /
  auth / funds). All four risk answers are no — the change itself is a revertible process-doc
  and skill edit that only STRENGTHENS a gate — but a wrong verdict convention or a hole in
  the path-trigger list creates false assurance exactly where assurance matters most, and it
  edits a live out-of-repo skill. human:<name> confirms: (a) the Security-Review verdict marker format,
  (b) the path-trigger list, (c) that the ready flip on risk-classed PRs now requires BOTH
  verdicts — matching his 2026-07-10 intent in #216.
why: >-
  CLAUDE.md mandates "DAML/auth/funds briefs: ALSO /security-review" but nothing in the flow
  enforces it — the reviewer verdict has no security-review field and the ready-flip checks
  App-APPROVED + CI only. daml-hardening/01 (#101, on-ledger money algebra) merged with no
  recorded security review; daml 02/03/04/06/08 are eligible and would follow the same path.
---

# Brief 30 — Security-review gate: verdict convention + risk-classed dispatch rule

## Context
files: CLAUDE.md ("Review-gate tooling" paragraph); docs/streams/findings/ ([F-22](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-10-security-review-mandate-is-prose-only-desk-tools-02-03-specs.md) entry
file — the registers are per-entry directories since methodology/23);
docs/streams/methodology/README.md (row)
out-of-repo files: ~/.claude/skills/pr-review-desk/SKILL.md (dispatch template + FLIP step —
staged as a diff in the PR, applied to the live file only as the LAST step before
`implemented`; author-brief rule 7: at most ONE out-of-repo brief in flight — serialize
against any #212 skill-edit brief (methodology/32) and desk-tools/06)
facts:
- The verdict machinery to extend (methodology/17): reviews post as `reviewer-app[bot]`
  via `mint-reviewer-token.go`; a PR author cannot approve its own PR; the flip signal is the
  App's review STATE at the current head.
- **Risk-classed PR** (define once, in CLAUDE.md; every consumer references it): the PR's
  owning brief has `gate: human` OR any `risk:` answer `yes`; OR — fallback for brief-less or
  mislabeled PRs — the diff touches a path trigger: `daml/`,
  `services/ledger-service/internal/auth/`, `services/ledger-service/internal/api/`,
  `k8s/*/identity.yaml`, `k8s/*/canton/`.
- **Security-review verdict convention** (the machine-checkable record): a SECOND App review
  at the PR's CURRENT head whose body starts with a `## Security review` heading and carries
  a literal line `Security-Review: pass` (posted `--approve`) or `Security-Review: fail`
  (posted `--request-changes`). Greppable by deskboard v2 / deskpost (desk-tools/02+03,
  amended in the same authoring PR as this brief) and compatible with deskpost C-3 body
  validation (## heading + verdict line).
- **Dispatch separation (dispatch-neutral-wording rule)**: the standard correctness reviewer's
  prompt stays frame-free (existing SKILL.md rule). The security review is therefore a
  SEPARATE dispatch running the `/security-review` skill — never folded into the correctness
  reviewer's prompt.
- **Deskboard v1 limitation**: the current skill-dir board tool reads only the latest App
  review state and cannot distinguish the two verdicts — until desk-tools/02 lands, the desk
  checks the second verdict manually at FLIP (state this in the SKILL.md edit).
- CLAUDE.md is word-budgeted (methodology/14): main is at ~2846 words vs the ≤2850 cap — the
  Review-gate paragraph addition must be compensated by trims elsewhere in the same paragraph.
- Consumers of the "what a ready-flip requires" value: pr-review-desk SKILL.md (fixed here),
  CLAUDE.md review-gate paragraph (fixed here), desk-tools/02+03 specs (fixed by the
  amendment landing with this authoring set), statusgen done-gate (follow-up methodology/31),
  deskboard v1 skill-dir binary (explicitly out of scope — superseded by desk-tools/02;
  manual desk check until then), batch-fanout worker template (out of scope — workers need no
  change; the gate is desk-side).
- methodology/22 (single-home operating rules, todo) will relocate pr-review-desk SKILL.md
  into the repo — this brief's rule text must survive that move verbatim (note it in the diff).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. CLAUDE.md "Review-gate tooling" paragraph: append the mechanical rule — the risk-classed
   definition (frontmatter OR path triggers, list verbatim from facts), the
   `Security-Review: pass|fail` verdict convention, and "risk-classed PRs flip ready only with
   BOTH App verdicts at the current head". Compensate the word budget.
2. pr-review-desk SKILL.md (staged diff, applied last): (a) dispatch step — on
   NEEDS-REVIEW/RE-REVIEW of a risk-classed PR, dispatch the `/security-review` reviewer as a
   SEPARATE agent in addition to the standard reviewer; it posts the second App verdict per
   the convention; (b) FLIP step — for risk-classed PRs, require BOTH verdicts at the current
   head; a re-push invalidates both; (c) note the deskboard-v1 manual-check limitation and
   that desk-tools/02+03 mechanize it.
3. [F-22](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-10-security-review-mandate-is-prose-only-desk-tools-02-03-specs.md) already filed with this authoring set (per-entry file under
   docs/streams/findings/, desk-tools/02+03 amended in the same change) — verify it
   still reads true at implementation time.
4. `go run ./tools/statusgen --root . --lint` green.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c "Security-Review: pass" CLAUDE.md` | exit 0; ≥1 |
| 2 | `grep -c "Security-Review: pass" ~/.claude/skills/pr-review-desk/SKILL.md` | exit 0; ≥1 (after the staged diff is applied) |
| 3 | `grep -c "risk-classed" ~/.claude/skills/pr-review-desk/SKILL.md` | exit 0; ≥2 (dispatch step + FLIP step) |
| 4 | `grep -n "k8s/\*/identity.yaml" CLAUDE.md` | exit 0 (path-trigger list present, verbatim) |
| 5 | `wc -w < CLAUDE.md` | ≤2850 (methodology/14 budget) |
| 6 | `statusgen --root . --lint; echo $?` | 0 |
| 7 | live drill (desk-run, post-merge): the next risk-classed PR (daml-hardening 02/03/04/06/08) shows TWO App verdicts at head before its ready flip | observed; PR # + both review URLs recorded in Evidence |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->

### Non-implementer verifier run — OFFLINE-VERIFY: PARTIAL (row 7 live-drill UNRUN) — glm-5.2-verifier, merged main `700e1c9e`, 2026-07-20

`gate: human` → Evidence recorded, status stays `implemented`; human:<name> signs off (surfaced in verify-gate issue #937).

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `grep -c "Security-Review: pass" CLAUDE.md` | 0 | `1` (≥1) PASS |
| 2 | `grep -c "Security-Review: pass" ~/.claude/skills/pr-review-desk/SKILL.md` | 0 | `1` (out-of-repo); in-repo canonical `../oit/.claude/skills/pr-review-desk/SKILL.md` = `2` — PASS |
| 3 | `grep -c "risk-classed" ~/.claude/skills/pr-review-desk/SKILL.md` | 0 | `3` (out-of-repo target → PASS). **DISCREPANCY: in-repo canonical copy = `1`** (below the ≥2 the rule wants) — the methodology/22 relocation appears to have left the in-repo canonical pr-review-desk SKILL behind on the `risk-classed` rule text. The Verify row targets the out-of-repo path (passes), but the canonical copy should be reconciled. |
| 4 | `grep -n "k8s/\*/identity.yaml" CLAUDE.md` | 0 | L150 — full path-trigger list present (`daml/`, `…/internal/auth/`, `…/internal/api/`, `k8s/*/identity.yaml`, `k8s/*/canton/`) |
| 5 | `wc -w < CLAUDE.md` | 0 | `2846` (≤2850) PASS |
| 6 | `go run ./tools/statusgen --root . --lint` | 0 | exit 0 (NOTICEs only) |
| 7 | live drill: next risk-classed PR shows TWO App verdicts at head before ready-flip | UNRUN | post-merge desk-run observation — needs a live risk-classed PR |

**OFFLINE-VERIFY: PARTIAL** — 6 offline rows PASS; row 7 is the deliberately post-merge live drill (UNRUN). Flag for human:<name>: the in-repo canonical pr-review-desk SKILL `risk-classed` count (1) is behind the out-of-repo copy (3) — reconcile if the rule was meant to survive the relocation verbatim. Stays `implemented`.

## Review
Gate: human (gate-why above — human:<name> signs off the convention, the trigger list, and the
both-verdicts flip rule). Reviewer also confirms the staged skill diff was applied as the
LAST step and committed to the ~/.claude stopgap repo.

### Row 7 live drill — observed, PASS — verify-desk dispatch, 2026-07-27

| 7 | live drill: next risk-classed PR shows TWO App verdicts at head before ready-flip | observed | **PR #1144** (cr-2026-07-23/02, risk-classed via `../oit/services/ledger-service/internal/auth/jwt.go` + `../oit/services/ledger-service/internal/api/external_challenge.go`) — final head `fb89d21c`: correctness APPROVED 2026-07-23T23:11:56Z (#pullrequestreview-4768886479) + `Security-Review: pass` APPROVED 23:12:40Z (#pullrequestreview-4768890334), both while still DRAFT (ready_for_review 23:13:36Z; merged 2026-07-24 human:<name>). Also demonstrates the full gate cycle: prior head `2ea72d07` carried correctness + `Security-Review: fail` CHANGES_REQUESTED before the fix. Runner-up: #1140 (both verdicts at head `e2ffcf19`). | 2026-07-27 | k3-verifier |

All 7 rows now have Evidence. `gate: human` — status stays `implemented`; the flip rides the verify-desk checkpoint PR for human:<name>. (Standing flag from the offline run: in-repo canonical pr-review-desk SKILL `risk-classed` count (1) vs out-of-repo copy (3) — reconcile if the rule was meant to survive relocation verbatim.)
