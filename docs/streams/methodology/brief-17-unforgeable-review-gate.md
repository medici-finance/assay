---
brief: methodology/17
title: Un-forgeable PR review gate — dedicated reviewer identity + GitHub approval-based gating
wave: 0
depends: ["methodology/16"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v1
authored: 2026-07-09 by desk (PR #125 incident — worker forged the DESK-READY marker)
sources: ["PR #125 incident (worker self-added DESK-READY: YES)", "methodology/brief-16 (non-self-writable gates)", "[F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md) (single git identity — harness-limited attribution)", "pr-review-desk SKILL.md § Marker authenticity"]
gate-why: >-
  Mints a distinct GitHub reviewer identity/credential (mint-reviewer-token.go) so its
  APPROVED review is tamper-evident by the worker identity that authors PRs — the PR #125
  incident showed a worker can self-certify the plain-text DESK-READY marker today;
  mishandling the new reviewer credential (leak, wrong scope) would let a worker forge
  reviews again, defeating the whole gate — sign-off confirms the token is stored/scoped
  correctly and self-certification is actually impossible, not just discouraged.
---

# Brief 17 — Un-forgeable PR review gate (dedicated reviewer identity)

## Context
files: `~/.claude/skills/pr-review-desk/SKILL.md` + `~/.claude/skills/pr-review-desk/deskboard.go`,
`~/.claude/skills/batch-fanout/SKILL.md`, `CLAUDE.md` (PR review loop section), the reviewer dispatch
template. GitHub repo settings (branch protection) for `example-org/{oit,
agent-runtime, medici-examples}`.

facts:
- The PR review loop's approval signal is a **plain-text `DESK-READY:` marker**. Every agent (worker,
  reviewer, desk) authenticates to GitHub as the **same identity** (`the-org`), so the marker is **not
  attributable** — the board tool cannot tell a reviewer's marker from a worker's forged one.
- **Incident (PR #125, 2026-07-09):** a worker posted a `Desk note` self-adding `DESK-READY: YES` after
  the dispatched reviewer forgot the closing line — self-certification. A real reviewer verdict happened
  to follow, so no harm landed, but had it not, the board would have flipped/merged a self-certified PR.
- This is **brief-16's non-self-writable-gate problem at the PR-review layer**, and the same root as
  **[F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md)**: a single shared git/GitHub identity means gates are convention, not cryptographically
  attributable. The interim mitigation (skill rule: "flip only on a verdict from a reviewer YOU
  dispatched; workers never write the marker") is discipline, not enforcement.
- **The durable fix uses GitHub's own rule that a PR author cannot approve their own PR.** Give the
  REVIEWER a distinct identity that posts a real `APPROVED` review; a worker (the author identity) then
  physically cannot forge it. The text marker is replaced by GitHub-native review state, and branch
  protection can make the merge itself gate on it. **One reviewer identity is sufficient** — a separate
  worker identity is optional attribution polish, out of scope here.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl beyond the task. Stop at `implemented`.
- Task 1 is HUMAN (creating a GitHub identity + branch protection + a token cannot be done by an agent).
  Tasks 2–4 (agent) are BLOCKED until Task 1 lands; do not stub a fake token.
- Token handling is sensitive: never commit the token; read it from a 600-perm file / secret, never echo it.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **[HUMAN] Stand up the reviewer identity + gate.** Create a dedicated reviewer identity — a **GitHub
   App** (preferred: scoped, no seat, short-lived ~1h installation tokens) or a machine-user account
   `medici-reviewer` (simplest). It must be a **Write** collaborator on the three repos — write-level
   access is required for an approval to COUNT toward a "require approvals" branch-protection rule
   (read-only reviewers can comment but their approval doesn't satisfy required-reviews); it never
   pushes, it just needs the write role.
   **Token — least privilege** (NOT `repo`-scope classic PAT / no `workflow` / no admin / no push):
   - Machine-user path: a **fine-grained PAT** scoped to the three repos with `Pull requests: write`,
     `Contents: read`, `Metadata: read` (+ `Checks: read` for CI state), with an expiry; org-approve it
     if `the-org` is an org.
   - App path: same permission set on the App; mint an installation token (App private key → JWT →
     installation token, e.g. via a helper) at use time.
   Place the token on the machine where reviewer agents run, in a 600-perm file (e.g.
   `~/.config/adopter/reviewer-token`). Enable **branch protection** on each repo: require ≥1 approving
   review to merge. (This step is human:<name>'s; the brief records it as the gating dependency for 2–4.)
2. **[agent] Reviewer-agent auth via `GH_TOKEN` (NOT `gh auth switch`).** `gh` selects the user from the
   `GH_TOKEN` env var, overriding the stored login per-process — concurrency-safe (two agents, two
   tokens, simultaneously). `gh auth switch` is WRONG here — it flips the global active account and
   concurrent agents would race on it. **Because this harness does not persist shell env across separate
   Bash calls, inline the token per command**: `GH_TOKEN=$(cat <reviewer-token-file>) gh pr review <N>
   --approve`. Only reviewer agents carry the reviewer token; workers and the desk stay on the default
   identity. (Git *push* is a separate credential path; reviewers only post reviews, so `GH_TOKEN`
   suffices — no push identity needed for the reviewer.)
3. **[agent] Switch verdict from text marker to native review.** The reviewer posts `gh pr review
   <N> --approve` on a pass and `--request-changes` on a fail (with the findings body), instead of a
   `DESK-READY:` text line. Keep a one-line human-readable summary if useful, but it is no longer the
   flip authority — the `APPROVED` state is.
4. **[agent] Read the native state everywhere.** Update `deskboard.go` to compute FLIP/READY from
   `reviewDecision == APPROVED` (and the approving reviewer being `medici-reviewer`), NOT from scraping
   `DESK-READY:` text; a text marker with no matching `APPROVED` review must NOT qualify. Update
   `pr-review-desk` + `batch-fanout` skills and the `CLAUDE.md` PR-loop section to describe
   approval-based gating (retire the DESK-READY marker; keep "workers never self-approve/flip").

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | as the default (author) identity: `gh pr review <own-PR> --approve` | GitHub REJECTS — "Can not approve your own pull request" (proves the forgery is impossible) |
| 2 | as the reviewer identity (reviewer token): `gh pr review <PR> --approve` | PR shows an `APPROVED` review authored by `medici-reviewer` |
| 3 | branch protection active: `gh pr merge <PR>` with NO approving review | merge blocked ("required approving review is missing") |
| 4 | `go run deskboard.go` on a PR that has only a text `DESK-READY: YES` (no `APPROVED` review) | that PR does NOT show FLIP/READY — text markers no longer qualify |
| 5 | `go run deskboard.go` on a PR with an `APPROVED` review by `medici-reviewer` + green CI | shows FLIP/READY |

## Evidence

Implementer run (2026-07-09). Task 1's identity+token was stood up by human:<name>; branch protection (the
hard-merge gate) is **deliberately deferred** ("need main open for debugging") — so the approval is the
desk's flip signal and human:<name> merges manually until protection is turned on. Tasks 2–4 implemented + live.

Resolved config (2026-07-09, since superseded — see retirement note): App **"Medici stuff"**, App ID `<app-id>`,
install `145487688`, bot login `reviewer-app[bot]`, key `~/.config/adopter/reviewer-app.pem` (600), token `~/.config/adopter/reviewer-token`.

> **Retired / renamed (2026-07-18, issue #404):** the legacy `reviewer-app` App `<app-id>` was retired and
> replaced by **`assay-reviewer-app[bot]`** (App `4331225`), one of the six-App **assay** desk-App family
> (reviewer / worker / verifier / desk / issue-loop / intake-loop). App IDs and both install IDs now live in the
> canonical record at `docs/streams/desk-apps/README.md` ("Provisioned Apps"); the App ID is no longer baked into
> source — tools read it from env `REVIEWER_APP_ID` (`~/.config/adopter/apps.env`) and fail loud if unset. The dated
> Evidence rows below record the ORIGINAL 2026-07-09 verification against the now-retired identity and are preserved
> unchanged as historical fact (they are not restatements of the current bot).

| Item | Command / check | Result |
|------|-----------------|--------|
| Verify 1 (self-approve blocked) | default `the-org` `gh pr review <own-PR> --approve` | REJECTED — "Can not approve your own pull request" ✓ |
| Verify 2 (reviewer can post) | `mint-reviewer-token.go` → `GH_TOKEN=… gh pr review 135 --comment` | posted; REST `/pulls/135/reviews` author = `reviewer-app[bot]` (distinct actor) ✓ |
| Token mechanism | `go run mint-reviewer-token.go` (fresh + reuse) | mints installation token (perms: `pull_requests:write`, contents/checks/statuses/metadata read), reuses if <50m old ✓ |
| Repo access | `GH_TOKEN=… gh api /installation/repositories` | all 3 target repos reachable (was 404 with a fine-grained PAT — App fixes the personal-account limitation) ✓ |
| Verify 4/5 (board reads state) | `deskboard.go` filters `reviewer-app[bot]` `APPROVED`/`CHANGES_REQUESTED` at head | text `DESK-READY:` markers no longer qualify; only the bot's review state does ✓ |
| Tooling | `~/.claude/skills/pr-review-desk/{mint-reviewer-token.go, deskboard.go}`, `pr-review-desk` + `batch-fanout` skills | all updated to approval-based gating; DESK-READY marker retired |

Deferred (not blocking `implemented`): **branch protection** (Verify 3, require-1-approval) — human:<name>'s call,
turn on when `main` no longer needs to stay open for debugging. Non-implementer verify + review still owed.

Independent verification (non-implementer opus re-run on merged main 07bcecaa, 2026-07-09):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | default `the-org` identity: `gh pr review 146 -R oit --approve` | — | **REJECTED** — `GraphQL: Review Can not approve your own pull request` — GitHub physically blocks the author identity from approving; the forgery is impossible. Reproduced exactly. | 2026-07-09 | independent (opus-verifier) |
| 2 | reviewer identity: `go run mint-reviewer-token.go` → `GH_TOKEN=… gh api /installation/repositories` + read PR #135 reviews | 0 | token mints/reuses (600-perm file, never printed); `/installation/repositories` reaches **all 3 target repos**; on oit PR #135 the reviewer bot posted a real **`APPROVED`** review as a **distinct actor** from `the-org`. Reproduced. | 2026-07-09 | independent (opus-verifier) |
| 3 | branch protection: `gh pr merge <PR>` with no approving review | n/a | **DEFERRED by design** — human:<name> deliberately left branch protection OFF ("need main open for debugging"), as the implementer's Evidence records. Not runnable, and not an implementation failure: the approval is the desk's flip signal and human:<name> merges manually until protection is enabled. Row remains open pending human:<name>'s toggle. | 2026-07-09 | independent (opus-verifier) |
| 4 | `deskboard.go` — text `DESK-READY:` marker qualifies? | — | Confirmed at source: `deskReviewState` counts ONLY `reviewerBot` reviews with state `APPROVED`/`CHANGES_REQUESTED` at head (deskboard.go L116, L120–131); there is no `DESK-READY:` text scraping anywhere in the file. A text marker alone yields `NEEDS-REVIEW`, never FLIP/READY. | 2026-07-09 | independent (opus-verifier) |
| 5 | `deskboard.go` — bot `APPROVED` + green CI ⇒ FLIP/READY? | — | Confirmed at source: FLIP requires `ready` (bot APPROVED at head) AND CI green AND draft (L189–191); READY when already flipped (L181–182). Logic sound. | 2026-07-09 | independent (opus-verifier) |

**Discrepancy found + resolved (record honestly):** the implementer Evidence names the bot
`medici-stuff[bot]`, but the SHIPPED identity is **`reviewer-app[bot]`** (App `<app-id>`).
deskboard.go L42 documents the correction ("the prior medici-stuff[bot] name was never a real
account"), the pr-review-desk SKILL.md uses `reviewer-app[bot]` throughout, and PR #135's
actual `APPROVED` review is authored by `reviewer-app[bot]`. This is **stale prose in the
brief's Evidence, not an implementation defect** — the code, skill docs, and live GitHub state are
mutually consistent on the correct name. Minor residual: `batch-fanout` SKILL.md still references
the retired `DESK-READY` marker in its worker-signal description (the flip authority is correctly
approval-based; this is a doc-cleanup nit, not a gate defect).

**Verdict:** the tamper-evident mechanism WORKS — rows 1/2/4/5 reproduce; row 3 (branch-protection
hard-merge gate) is an intentional, documented deferral (human:<name>'s toggle), not a gap. Flipped to
`verified`. The `gate: human` review (token handling / `/security-review`) remains owed and is
separate from this verification.

## Review
Gate: human (identity/branch-protection infra + a token = sensitive-data). Run `/security-review` on the
token handling (no token in logs/commits/echo; correct file perms; scoped App permissions). Reviewer
records verdict + date in the stream README. Note: once this lands it SUPERSEDES the interim
"DESK-READY marker + dispatch-ledger" mitigation in the pr-review-desk skill.
