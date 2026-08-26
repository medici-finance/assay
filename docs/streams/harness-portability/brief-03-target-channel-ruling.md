---
brief: harness-portability/03
title: "Ruling: target harnesses, delivery channel, degradation matrix"
why: >-
  Three commitments only Ian can make sit at the neck of this stream: which Codex
  surfaces v1 targets (CLI, App, or both), how adopters receive the Codex artifacts
  (bundle-install now vs the OpenAI plugin marketplace — the latter a public act on
  content this repo has not yet published), and the ruled run/degrade/refuse
  posture per skill. Every wave-2+ brief builds to these; deciding them mid-build is
  how scope drifts and how a marketplace push accidentally front-runs the publication
  gate.
wave: 1
depends: ["harness-portability/01"]
unblocks: ["harness-portability/04", "harness-portability/05", "harness-portability/06"]
effort: S
gate: human
gate-why: >-
  The decisions are commitments, not analyses: target-set and channel bind spend and
  public exposure, and the marketplace option interacts with the one-way publication
  (the publication review gates the copy; a marketplace listing is downstream of it).
  The human is confirming the three RULED lines — not the supporting analysis, which the
  model prepares. Per house convention the ruling artifact must be authored or confirmed
  on-thread by the human maintainer; a model's restatement of an in-session direction is
  not a ruling.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-07 by harness-portability authoring session
sources: ["authoring dispatch (Ian, 2026-08-07)", "harness-portability/01's capability matrix (the facts the ruling is made on)", "the publication stream (the marketplace channel's gate: publication is a fresh history-free copy into the public repo; nothing distributes from a private repo)", "superpowers precedent: distribution into openai-codex-plugins is a PR into OpenAI's fork repo (scripts/sync-to-codex-plugin.sh) — i.e. a public act", "freshness-checked 2026-08-07 (no prior harness-target ruling exists anywhere in docs/)"]
---

# Brief 03 — Ruling: targets, channel, degradation

## Context

files:
- **create** `docs/harness-portability-ruling.md` (planned) — the three rulings + the per-skill
  degradation matrix, each ruling a single `RULED:` line with date and the deciding
  account
- **create** a GitHub issue carrying the decision request (the ruling lands as a
  maintainer comment there; the doc records and links it)

facts:
- The decision inputs are 01's measured matrix — this brief does not re-measure.
- **Decision A — target set**: Codex CLI only, Codex App only, or both for v1. The two
  differ materially (App: managed detached-HEAD worktrees + Seatbelt sandbox blocking
  branch/push; CLI: no built-in worktree management) per superpowers' findings, as
  re-measured by 01.
- **Decision B — delivery channel**: (i) bundle-install — adopters get the Codex
  artifacts from the published `assay` repo via the adopt flow (works the day the
  publication stream lands, no third party); (ii) marketplace — a listing in
  `openai-codex-plugins` (superpowers-style PR into OpenAI's repo), which is a PUBLIC
  act and therefore sits strictly behind the publication gate; or (iii) both, staged.
  The authoring-time recommendation to confirm or overturn: **(iii) staged — bundle
  first, marketplace as a post-publication follow-up brief**.
- **Decision C — degradation matrix**: per skill × target, one of `runs` / `degrades
  (how)` / `refuses (why)`. The non-negotiable floor (from the stream README): isolation,
  evidence, and review gates never degrade — no dispatch capability → batch-fanout runs
  serially with an explicit in-session statement; no isolation → implementer work
  refuses rather than touching a shared checkout.
- The four risk answers are `no` — the ruling itself is revertible text; what makes the
  gate human is WHO may commit it, not blast radius.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the
  task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- The model PREPARES options and a recommendation; it never records a `RULED:` line on
  its own authority. Check the issue for an existing maintainer comment before asking —
  the decision may already be on-thread.

## Task

1. Draft `docs/harness-portability-ruling.md` (planned): for each of A/B/C, the options, the
   01-matrix facts that bear on them, a one-paragraph recommendation, and an empty
   `RULED:` line.
2. File the decision issue (label `decision-needed`) summarizing the three questions
   with links; request Ian's ruling there.
3. When the maintainer's ruling lands, fill the three `RULED:` lines (decision, date,
   deciding account, comment permalink) and the degradation matrix cells.

## Verify (executable — no prose-only DoD items)

Presence-gate rows (prose deliverable); row 3 is the human act and stays BLOCKED until
the ruling exists — it is the brief's actual completion condition.

| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/harness-portability-ruling.md; echo $?` | `0` |
| 2 | `grep -c '^RULED:' docs/harness-portability-ruling.md` | exactly `3` — one per decision, none dodged |
| 2a | **Positive control for row 2** — `grep -c '^RULED-NEVER-WRITTEN:' docs/harness-portability-ruling.md \|\| true` | output `0` — proves the anchored-count probe distinguishes present from absent (same shape row 2 counts; `\|\| true` neutralises grep's exit-1 on the zero-match success path) |
| 3 | **BLOCKED (needs Ian)** — each `RULED:` line cites a github.com comment permalink; `gh api` the permalink's comment and check `.user.login` | the maintainer's account for all three. A `RULED:` line whose permalink resolves to any other account (an agent identity is spoofable) is RED, not a formality |
| 4 | `grep -cE -e '[\|] *runs' -e '[\|] *degrades' -e '[\|] *refuses' docs/harness-portability-ruling.md` | `>= 7` — every bundled skill has a degradation cell for the ruled target set (separate `-e` patterns; `\|` "alternation" inside `-E` is a literal pipe — a grep gotcha logged upstream) |
| 5 | Decision issue exists: `gh issue list --repo medici-finance/assay --state all --search "harness portability ruling" --json number \| jq length` | `>= 1` |

## Evidence

<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     Row 3's three permalinks + resolved account logins must be pasted verbatim.
     "verified" requires a non-implementer. -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `test -f docs/harness-portability-ruling.md; echo $?` | 0 | `0` — deliverable present | 2026-08-15 | assay-worker-app[bot] (HP/03 ruling-package session) |
| 2 | `grep -c '^RULED:' docs/harness-portability-ruling.md` | 0 | `3` — one empty `RULED:` line per decision A/B/C, none dodged, all awaiting the maintainer | 2026-08-15 | assay-worker-app[bot] |
| 2a | `grep -c '^RULED-NEVER-WRITTEN:' docs/harness-portability-ruling.md \|\| true` | 1→0 | `0` — positive control: the anchored probe distinguishes present from absent | 2026-08-15 | assay-worker-app[bot] |
| 3 | **BLOCKED (needs Ian)** — each `RULED:` cites a maintainer comment permalink | — | RED by design: all three `RULED:` lines are intentionally empty pending Ian's ruling on the decision issue. This is the brief's completion condition, not a defect. Filled by Task 3 when the maintainer comment lands. | — | BLOCKED |
| 4 | `grep -cE -e '[\|] *runs' -e '[\|] *degrades' -e '[\|] *refuses' docs/harness-portability-ruling.md` | 0 | `9` (≥7) — every one of the 9 bundled skills carries a degradation cell for the ruled-recommended target (Codex CLI); verdict tokens deliberately unbolded so the space-only pattern matches the ruled column | 2026-08-15 | assay-worker-app[bot] |
| 5 | `gh issue list --repo medici-finance/assay --state all --search "harness portability ruling" --json number \| jq length` | 0 | `>= 1` — decision issue filed under the worker App identity (see PR description) | 2026-08-15 | assay-worker-app[bot] |
### Non-implementer verifier run — VERIFY: PASS — 2026-08-16 opus-4.8[1m]-verifier (verify-desk dispatch), independent re-run against merged main

Runner ≠ implementer (implementer was the HP/03 ruling-package worker session). Own isolated worktree reset to `origin/main`. All Verify rows re-executed fresh; implementer Evidence not trusted. `gate: human`, `risk {regulatory: no, customer: no, irreversible: no, sensitive-data: no}`. **Material change since prior run: Ian has ruled** — the three `RULED:` lines now trace to a live maintainer comment.

| # | Command | Exit | Observed | Runner |
|---|---------|------|----------|--------|
| 1 | `test -f docs/harness-portability-ruling.md; echo $?` | 0 | `0` — deliverable present | 2026-08-16 opus-4.8[1m]-verifier |
| 2 | `grep -c '^RULED:' docs/harness-portability-ruling.md` | 0 | `3` — one per decision A/B/C, none dodged | 2026-08-16 opus-4.8[1m]-verifier |
| 2a | `grep -c '^RULED-NEVER-WRITTEN:' … \|\| true` | 0 | `0` — positive control fires (present-vs-absent distinguished) | 2026-08-16 opus-4.8[1m]-verifier |
| 3 | `gh api` the permalink cited by all three RULED lines, `--jq '.user.login'` | 0 | resolves to the maintainer's account — comment on the decision issue, created 2026-08-16, body `A1, B3, C: Non-negotiable floor…` matches the three RULED lines. No longer BLOCKED — Ian has ruled | 2026-08-16 opus-4.8[1m]-verifier |
| 4 | `grep -cE -e '[\|] *runs' -e '[\|] *degrades' -e '[\|] *refuses' docs/harness-portability-ruling.md` | 0 | `9` (≥7) — every one of the 9 bundled skills carries a Codex-CLI degradation cell | 2026-08-16 opus-4.8[1m]-verifier |
| 5 | `gh issue list … --search "harness portability ruling" --json number \| jq length` | 0 | `4` (≥1) — decision issue present | 2026-08-16 opus-4.8[1m]-verifier |

RISK-VALUE: DERIVED — the deciding account resolves live (via `gh api` on the cited permalink comment) to the maintainer's `.user.login` on the decision thread; the comment body matches all three RULED lines verbatim; the ruling is genuinely maintainer-authored on-thread, not a model-laundered in-session direction.

RISK-VALUE: N/A — enumeration over numeric literals (thresholds/bounds/tolerances/timeouts/limits) in the diff found none; the irreversible-in-spirit act here is the ruling's authorship, DERIVED above.

VERIFY: PASS — all 5 runnable rows met their Expect; row 3 (formerly BLOCKED) now resolves to the maintainer. Status advanced `implemented → verified`; `done` remains gated on the human Reviewed stamp (`gate: human`). Non-blocking follow-up filed: the ruling doc's closing section still carries stale pre-ruling text ("stays RED until Ian rules").

## Review

Gate: **human** (from frontmatter). The reviewer confirms one narrow thing: all three
`RULED:` lines trace to maintainer-authored artifacts (row 3), and the degradation matrix
honours the non-degradable floor — no cell degrades isolation, evidence, or a review
gate. The analysis quality is the model loop's job; the rulings' authorship is the
human gate's.
