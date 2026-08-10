---
brief: issue-loop/05
title: 'Reviewer desk-flags become issues — non-blocking review residuals feed the issue-loop, never dropped'
wave: 2
depends: ["issue-loop/02"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-11 by Fable desk session (human:<name> direction)
withdrawn: 2026-07-15
superseded-by: ".claude/skills/pr-review-desk/SKILL.md --out-of-scope-discoveries rule (PR #512)"
sources: ["human:<name> 2026-07-11: review desks post 'one item flagged for the desk (not a blocker)' comments that get dropped — file them as issues so they aren't lost; the issue-loop is the issue-desk that receives them", "docs/streams/issue-loop ([I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md) — the loop this feeds), brief-02 (the scanner extended here)", "methodology/28 (reviews-as-issues pattern), methodology/17 (the App review channel these markers ride)", "issue #221 (out-of-repo skill edit protocol)", "freshness-checked 2026-07-11 @ post-#304 main"]
why: >-
  A reviewer surfaces a non-blocking item ('flagged for the desk'), the PR merges BECAUSE it's
  non-blocking, and the flag evaporates — nobody actions it. It's the rotting-outside-the-model
  leak, specific to review residuals. Filing each flag as an issue routes it into the
  issue-loop ([I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md)), which already turns issues into placeholder briefs on Next-up — so 'not
  dropped' becomes structural, not a promise.
---

# Brief 05 — Reviewer desk-flags become issues (WITHDRAWN 2026-07-15)

**Withdrawn:** the `DESK-FLAG:` marker convention this brief chartered is retired by the
out-of-scope-discoveries rule in `../oit/.claude/skills/pr-review-desk/SKILL.md` (PR #512,
2026-07-14). That rule mandates reviewers file issues directly at discovery time — no
intermediate marker, no review-body scanner. The problem this brief set out to solve
("non-blocking review residuals rot outside the model") is solved by the new rule instead.

## Context
files: `../assay-toolkit/statusgen/` (extend `--scan-issues`, brief-02, to also scan review bodies);
review-marker convention documented where the review-gate rules live
out-of-repo files: `~/.claude/skills/pr-review-desk/SKILL.md` (the marker convention reviewers emit)
facts:
- **Marker convention (structured, not prose-parsed):** a reviewer emits a machine-readable
  line per flag in its POSTED review body: `DESK-FLAG: <one-line description>` (optionally
  `DESK-FLAG[label]: ...` to route). The scanner keys on `^DESK-FLAG:` — free-form
  'flagged for the desk' prose does NOT auto-file (it must be the marker), so the reviewer
  decides what is worth an issue rather than every nitpick becoming one (noise control).
- **The scanner (extends brief-02's `--scan-issues`):** for each PR, read the App-posted
  reviews, extract `DESK-FLAG:` lines, and file ONE issue per flag — title from the flag text,
  body carrying provenance (source PR #, review permalink, the verbatim flag line), label
  `desk-flag`. Idempotency: dedup on (PR#, flag-text-digest) — a re-review repeating the same
  flag files nothing new; the existing-issue check is by that digest recorded in the issue body.
- **It then IS an issue-loop input:** the filed `desk-flag` issue is scanned by the issue-loop
  (brief-02) like any open issue → gets a placeholder → flows to Next-up. So this brief only
  needs to FILE the issue correctly; the loop carries it from there (do not re-implement
  placeholder creation here).
- **Exclusion symmetry:** `desk-flag` issues are WORK issues (unlike `verify-gate`/`live-verify`
  system tokens) — they are NOT on the issue-loop's excluded-label list; they SHOULD become
  placeholders. State this explicitly so brief-02's exclusion list isn't mis-extended.
- Provenance-only body (no agent summary): the issue body carries the reviewer's VERBATIM flag
  line + the PR/review link — never an agent-reworded version (same anti-injection discipline
  as the verifier-remediation on-ramp: the human/reviewer's words, not a paraphrase).
- Network/offline: rides `--scan-issues` (already gh-dependent, never in the offline lint gate).

## Ground rules
- NEVER git push / trigger workflows / mutating kubectl. Leave commits per task only.
- Out-of-repo skill edit per #221 (declared; apply last; diff in PR body).
- Stop at `implemented`. NEEDS_CONTEXT over guessing.

## Task
1. Extend `--scan-issues`: parse App-review bodies for `^DESK-FLAG:` lines; file one
   `desk-flag`-labelled issue per flag with provenance; idempotent on (PR#, digest).
2. Document the `DESK-FLAG:` marker where the review-gate rules live + the pr-review-desk skill
   (reviewers emit it; #221 protocol).
3. Tests (fake gh): a review with two DESK-FLAG lines → two issues; a re-review repeating one →
   no duplicate; free-form 'flagged for the desk' prose (no marker) → nothing filed; the filed
   issue body carries the verbatim line + PR link, no paraphrase.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes the Task-3 cases |
| 2 | `statusgen --root . --scan-issues --dry-run \| grep -ci "desk-flag"` | reports would-file desk-flag issues from DESK-FLAG markers (on a fixture) |
| 3 | `grep -c "DESK-FLAG" .claude/skills/pr-review-desk/SKILL.md` (post out-of-repo apply) | ≥1 (convention documented for reviewers) |
| 4 | PR body carries the out-of-repo pr-review-desk diff (#221) | present |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- non-implementer rows. -->
Independent assessment (non-implementer — reviewer-app[bot], PR #512 branch `0310d656`):

| # | Check | Result | Date | Runner |
|---|-------|--------|------|--------|
| 1 | DESK-FLAG review-body parser exists in `tools/statusgen` | Not built — zero hits for `DESK-FLAG` outside brief-05's own text | 2026-07-15 | reviewer-app[bot] |
| 2 | `DESK-FLAG:` convention retired by out-of-scope-discoveries rule in SKILL.md (PR #512) | Retired; new rule = file issues directly at discovery time | 2026-07-15 | reviewer-app[bot] |
| 3 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 (advisory NOTICEs only on unrelated streams) | 2026-07-15 | reviewer-app[bot] |

**WITHDRAWN** — the problem this brief set out to solve (non-blocking review residuals rotted outside the model) is solved by the direct-file-issues rule instead of a DESK-FLAG marker + scanner.

## Review
Gate: model. Reviewer confirms (a) only the structured `DESK-FLAG:` marker files an issue (not
prose — noise control), (b) idempotency prevents re-review duplicates, (c) the issue body is
the reviewer's verbatim line (no agent paraphrase), (d) `desk-flag` is a work-issue that the
issue-loop picks up (not on the excluded list).
