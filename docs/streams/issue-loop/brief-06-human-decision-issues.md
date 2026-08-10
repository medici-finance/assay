---
brief: issue-loop/06
title: 'Human-decision issues — any human gate surfaces as a labeled, self-contained decision issue (situation prose + pros/cons)'
wave: 2
depends: ["issue-loop/01"]
unblocks: ["issue-loop/08"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by Fable desk session (human:<name> direction)
sources: ["human:<name> 2026-07-12: 'for methodology/11 it hit a human gate. for these we need an issue created, and probably a label for things which needs a human''s decision. whenever we need a human decision, we need pros/cons, and prose describing the situation in the issue itself'", "methodology/11 (gate: human, customer+irreversible yes — the exemplar that sat gated with NO surfaced decision request)", "methodology-metrics/12 (verify-gate sign-off issues + the standing trade-off/prior-state card standard — the per-decision quality bar this generalizes)", "issue #228 (gate:human binds APPROVAL not dispatch — this brief covers the decision points that DO block)", "I-intake-desk (its decision-needed disposition routes to the same label)", "issues #266/#267/#268/#283/#284/#293/#311/#331/#332 (the desk's trade-off analyses — the format precedent)", "freshness-checked 2026-07-12 @ post-#371 main"]
why: >-
  gate:human briefs wait for a human decision, but today the wait is INVISIBLE: methodology/11
  has sat gated with no artifact telling human:<name> a decision is wanted, what the question is, or
  what the options cost. The verify-gate issues (mm/12) solved this for post-implementation
  sign-offs; nothing solves it for pre/mid-flight decisions — so gated work stalls silently,
  which is the exact "what is waiting on humans and WHY" visibility human:<name> keeps asking for.
  A decision the human can act on in one read (situation, options, pros/cons, recommendation)
  converts gate latency from days-of-silence into a queue item.
---

# Brief 06 — Human-decision issues

## Context

files: `../assay-toolkit/statusgen/` (detection + lint of the linkage), `docs/streams/issue-loop/README.md`
(conventions), GitHub label (created once via gh).

facts:
- Exemplar: methodology/11 (`gate: human`, customer+irreversible `yes`) reached the top of
  Next-up with its decision unsurfaced — nothing existed for human:<name> to answer.
- mm/12 owns the VERIFY-stage gate issues (sign-off at `implemented`, `verify-gate` label,
  ALLOWED_CLOSERS human-close). This brief owns the DECISION-stage: a brief whose gate
  question must be answered before/while work proceeds. Do not duplicate mm/12's machinery —
  reuse its issue-creation plumbing (`verifyissues.go`) where it fits.
- The mm/12-amended card standard (trade-off section: "why we want it / what it limits",
  prior-state prompt) is the WRITING BAR for decision bodies — the desk's existing analyses
  on #266/#267/#268/#283/#284/#293 are worked examples of the expected depth.
- Issue-loop convention: system-emitted labels are excluded from the issue-scanner
  (brief-02) — add the new label to that exclusion list (a decision issue is a closeable
  STATE, not schedulable work).
- consumers (rule 6 — this adds a shared label + a brief-frontmatter key): issue-loop/02
  scanner (exclusion list), mm/11 gate-queue prioritization (decision issues join the
  human queue it orders), [I-28](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-loop-monitoring-dashboard-a-wip-website-over-the-standing.md) WAITING-ON-HUMAN panel (renders the open set), I-intake-desk
  (decision-needed intake entries route here), statusgen lint (link-integrity check below).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **Label**: create `needs-decision` (distinct color from `verify-gate`), description
   "blocked on a human decision — body carries situation + options + pros/cons".
2. **Decision-issue template (conventions section in this stream's README)** — every
   decision issue is SELF-CONTAINED, readable without opening the repo:
   - **Situation** (prose): what the brief is doing, what fork was hit, why it can't
     proceed on a default;
   - **Options** (2-4), each with pros/cons at the mm/12 trade-off bar ("why we'd want
     it / what it limits or costs"), plus the author's recommendation and WHY;
   - **What happens on each answer** (which brief rows/PRs move, what gets unblocked);
   - links: brief typed-ID, PR if any, related findings/issues.
   - The decider is the human; the close (with the chosen option stated) is the decision
     record. #237 applies: the close/choice must come from a verified human account.
3. **Brief linkage**: a gated brief carrying an open decision gets
   `decision-issue: <NN>` in frontmatter (schema addition, optional field). statusgen:
   (a) NOTICE when a `gate: human` brief is top-of-Next-up (or dispatched) with NO open
   decision issue and no recorded decision; (b) lint PROBLEM when `decision-issue:` points
   at a closed issue whose decision isn't reflected (brief unamended) — start advisory
   (NOTICE) if the false-positive rate is unclear.
4. **Backfill the exemplar**: file the methodology/11 decision issue (its gate-why is the
   situation seed) as the first live instance — the Verify row below proves the loop end-to-end.
5. **Scanner exclusion**: add `needs-decision` to issue-loop/02's excluded-labels list
   (amend that brief's Context in the same commit — it is `todo`, no demotion).
6. **Close guard (enforces task 2's #237 clause — amended in-flight per the #427 review)**:
   `../oit/.github/workflows/needs-decision-close.yml`, label-scoped to `needs-decision` — a bot
   or non-allowlisted close is REOPENED + commented (verify-gate-close.yml's guard pattern).
   A valid human close flips NO lifecycle state (a decision close is not a verify sign-off);
   the part (b) lint nags until the outcome is reflected in the brief.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `gh label list --repo oit \| grep needs-decision` | exit 0, one row |
| 2 | `gh issue list --repo oit --label needs-decision --state open --json number,title \| jq length` | ≥ 1 (the methodology/11 backfill exists) |
| 3 | `gh issue view <methodology/11-issue> --json body -q .body \| grep -c -iE -e "pros" -e "cons" -e "option"` | ≥ 3 (options + pros/cons present). `<methodology/11-issue>` is an unsubstituted metavariable — substitute the concrete issue number before running (see #509 residual-limit note in methodology/44) |
| 4 | `statusgen --root . --lint` | exit 0; output contains the gated-without-decision-issue NOTICE for any qualifying brief |
| 5 | `grep -n "needs-decision" docs/streams/issue-loop/brief-02-issue-scanner.md` | exit 0 (exclusion recorded) |
| 6 | `yq eval '.jobs.guard.if' .github/workflows/needs-decision-close.yml \| grep needs-decision` | exit 0 (close guard label-scoped) |

## Evidence
Implementation run (implementer, worktree, branch `feat/issue-loop-06-decision-issues`):

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `gh label list --repo oit \| grep needs-decision` | 0 | `needs-decision  blocked on a human decision — body carries situation + options + pros/cons  #F9D0C4` | 2026-07-16 | deepseek-v4-pro |
| 2 | `gh issue list --repo oit --label needs-decision --state open --json number,title \| jq length` | 0 | `4` (>= 1; includes #424 for methodology/11) | 2026-07-16 | deepseek-v4-pro |
| 3 | `gh issue view 424 --repo oit --json body -q '.body' \| grep -c -iE "pros\|cons\|option"` | 0 | `13` (>= 3; options + pros/cons present) | 2026-07-16 | deepseek-v4-pro |
| 4 | `go run ./tools/statusgen --root . --lint` | 0 | exit 0; 18 `has no decision-issue` NOTICEs for qualifying briefs; methodology/11 NOTICE suppressed by `decision-issue: 424` | 2026-07-16 | deepseek-v4-pro |
| 5 | `grep -n "needs-decision" docs/streams/issue-loop/brief-02-issue-scanner.md` | 0 | line 28: exclusion recorded in documented-constant list | 2026-07-16 | deepseek-v4-pro |
| 6 | `yq eval '.jobs.guard.if' .github/workflows/needs-decision-close.yml \| grep needs-decision` | 0 | `contains(github.event.issue.labels.*.name, 'needs-decision')` — close guard label-scoped | 2026-07-16 | deepseek-v4-pro |

Task items completed:
- Item 1 (label `needs-decision`): already exists on GitHub, distinct color `#F9D0C4`
- Item 2 (decision-issue template): already in stream README conventions section
- Item 3 (brief linkage): `decision-issue:` frontmatter field in BriefFile struct, parsed by `parseBriefFile`, lint NOTICE in `checkBriefFiles`, `--decision-issues` flag wired in main.go — all code in place
- Item 4 (backfill methodology/11): decision issue #424 exists with full Situation/Options/What-happens body, `decision-issue: 424` added to methodology/11's frontmatter
- Item 5 (scanner exclusion): `needs-decision` in `scanExcludedLabels` map (`scanissues.go`) + listed in brief-02's documented-constant list
- Item 6 (close guard): `../oit/.github/workflows/needs-decision-close.yml` label-scoped to `needs-decision`, enforces #237 (only allowlisted human may close), bot/non-allowlisted close triggers reopen + comment

### Non-implementer verifier run (glm-5.2-verifier, merged main `3d3708ad`, 2026-07-16)

All 6 Verify rows RUN, none UNRUN — every expectation met on merged main.

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `gh label list … \| grep needs-decision` | 0 | label present with description + color `#F9D0C4` |
| 2 | `gh issue list --label needs-decision --json … \| jq length` | 0 | `4` (≥1) |
| 3 | `gh issue view 424 … \| grep -c -iE "pros\|cons\|option"` | 0 | `13` (≥3); placeholder `<methodology/11-issue>` = #424 |
| 4 | `go run ./tools/statusgen --root . --lint` | 0 | emits the gated-without-decision-issue NOTICE for qualifying briefs; methodology/11 suppressed via `decision-issue: 424` |
| 5 | `grep -n needs-decision brief-02-issue-scanner.md` | 0 | exclusion recorded (L28) |
| 6 | `yq eval '.jobs.guard.if' needs-decision-close.yml \| grep needs-decision` | 0 | close guard label-scoped to `needs-decision` |

VERIFY: PASS. (Implementer row-4 NOTICE count `18` is now `15` on current main — qualitative expectation still met; the count drifts as briefs flip state.)

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
