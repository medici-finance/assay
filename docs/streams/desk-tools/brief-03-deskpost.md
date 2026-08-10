---
brief: desk-tools/03
title: deskpost — review/comment/ready as the reviewer App, with the constraints in code
wave: 1
depends: ["desk-tools/01"]
unblocks: ["desk-tools/06"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session ([I-23](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-desk-tools-zero-prompt-workflow-plumbing-purpose-built-binar.md), scoping.md)
sources: ["docs/streams/desk-tools/scoping.md (TM-1, TM-2, C-2, C-3, C-5, C-7, C-10)", "~/.claude/skills/pr-review-desk/mint-reviewer-token.go (absorbed)", "CLAUDE.md 'PR review loop' (the policy being encoded)", "freshness-checked 2026-07-10 @ b98e1e84"]
why: >-
  Posting the reviewer verdict, replying on PRs, and flipping converged drafts ready are the
  desk's standing-authorized job, yet each one prompts today (~70 mutating gh calls measured
  across recent sessions). Centralizing them in one App-identity tool also hardens the
  tamper-evident review gate: token minting and posting live in one audited place instead of an
  ad-hoc script every reviewer invokes.
---

# Brief 03 — deskpost — review/comment/ready as the reviewer App

## Context
files: create `../assay-toolkit/tools/desk/cmd/deskpost/` (new); absorb
`~/.claude/skills/pr-review-desk/mint-reviewer-token.go` (4.4k — signs a short JWT with the
App private key, exchanges it for an ~hourly installation token; env: `REVIEWER_APP_ID`,
`REVIEWER_INSTALL_ID`, key path — read that file for the exact env names and endpoints and keep
them identical); uses `../assay-toolkit/tools/desk/internal/deskkit` (brief 01)
facts:
- The policy being encoded (CLAUDE.md "PR review loop", verbatim obligations): reviews are
  posted BY THE APP identity (`reviewer-app[bot]`) so a PR author cannot self-approve;
  the ready-flip happens ONLY when the App has APPROVED at the CURRENT head and checks are
  green; ready = "ready for HUMAN review"; merge is always human.
- Subcommands and their constraint sets:
  - `deskpost review <repo> <pr> --verdict approve|request-changes --head <sha> --body-file F`
    — `--head` is REQUIRED: the SHA the verdict was formed against; refuse exit 5 if the PR's
    current head differs (a verdict must never land on unreviewed code)
  - `deskpost comment <repo> <pr> --body-file F`
  - `deskpost ready <repo> <pr>`
- **C-2 (ready preconditions, ALL verified in-tool immediately before acting):** (a) PR state
  OPEN and draft=true; (b) the reviewer App's latest review on the PR is APPROVED and was
  submitted at the PR's CURRENT headRefOid; (c) combined status/check rollup at that head is
  green (no FAILURE/ERROR/PENDING-required); (d) repo ∈ deskkit set. Any check unverifiable
  (API error, ambiguous rollup) → exit 6 (C-10). (e) **security-review gate (#216,
  methodology/30):** if the PR is risk-classed (same path-trigger computation as deskboard —
  shared helper in deskkit, not duplicated), an App review at the CURRENT head must carry the
  literal line `Security-Review: pass` → otherwise exit 5, no flip; determination unverifiable
  (API error, reviews unfetchable) → exit 6 (C-10). TOCTOU note: re-read headRefOid immediately
  before the flip call; if it changed since (b) was checked → exit 5, no flip.
- **C-3 (body validation, both review and comment):** size ≤16 KiB; must-carry structure for
  reviews (a `## ` heading and a verdict line — match the desk's posted-review convention);
  secret scan REFUSES on: runs of ≥32 [A-Za-z0-9+/=_-] chars, `ghp_`/`github_pat_`/`ghs_`/
  `gho_` prefixes, `AKIA[0-9A-Z]{16}`, `-----BEGIN` PEM headers, `eyJ`-prefixed 3-dot JWT
  shapes, and `sops`/`ENC[` ciphertext markers. Exit 5 on any hit; NO override flag exists.
  Body is read from file only — no stdin/inline body path.
- **C-5:** deskkit semantics (v2): keys ready=(repo,pr,head), review=(repo,pr,head,verdict),
  comment=(repo,pr,head,bodyDigest); only result∈{ok,noop} counts as done; flock across
  check→call→append; audit line after remote success; noop prints what it deduplicated against.
- **C-7:** the tool has NO other verbs. No merge, no close, no un-ready, no edit, no label.
  The GitHub App token is scoped by the App's own permissions — do not request new App scopes.
- Token handling: the JWT→installation-token mint moves INTO this tool (env names/endpoints
  identical to mint-reviewer-token.go); token held in memory only; mint-on-expiry mid-operation
  is the ONE allowed internal retry (C-10 exemption — it re-verifies nothing about the world);
  missing/unreadable PEM → exit 6 naming the manual path. Never write token or key material to
  audit/stdout/any file (audit `detail` carries counts/SHAs only).
- Verdict schema (C-3): DEFINE it in the tools/desk README verdict-format section as a
  deliverable of THIS brief (required sections + verdict line for review bodies; plain comments
  get scan+size only); brief 06 wires the loop skills to it. bodycheck comes from deskkit
  (brief 01) — do not reimplement; git SHAs (40/64 lowercase hex) must pass, with test vectors.
- App identity facts (from the absorbed script + memory): App id <app-id> posts as
  `reviewer-app[bot]`; installation tokens expire ~hourly — mint fresh per invocation,
  no caching.
- **AMENDED 2026-07-13 ([F-29](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-deskpost-ci-rollup-unpaginated-latent-fail-open.md) / [F-30](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-deskpost-ready-cannot-flip-ci-less-report-repos-decks-reconciler-proposals.md) fix, PR TBD) — the CI-rollup facts behind C-2(c):**
  - The rollup must be read IN FULL. GitHub's default page size is **30**, so an unpaginated
    status/check-runs read silently truncates a longer rollup and can report a red head as
    green ([F-29](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-deskpost-ci-rollup-unpaginated-latent-fail-open.md), the original fail-open). Both rollups are paginated (`per_page=100`), AND
    `evalCI` reconciles the items held against the reported `total_count` — short by one →
    `ciPending` (exit 6), never green. A rollup that cannot be read in full degrades CLOSED.
  - Scoping C-4's "only `medici-examples` is CI-less" **is superseded**: after [F-23](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-10-review-verify-gates-cover-medici-finance-report-repos.md) the desk
    set is seven repos, of which four have no PR CI. The CI-required policy is now a column of
    the deskkit repo table (`deskkit.CIRequired`, one source with `IsAllowedRepo`), not a
    second list — CI-less: `medici-examples`, `medici-finance/{reconciler,decks,proposals}`;
    CI-required: `oit`, `agent-runtime`, `medici-finance/assay-toolkit`
    (it DOES run a workflow). Verified against the GitHub API 2026-07-13. Adding a repo to the
    set now forces an explicit CI decision, so the two cannot drift again ([F-30](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-deskpost-ready-cannot-flip-ci-less-report-repos-decks-reconciler-proposals.md)).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state (App env names differ, review-convention
  shape unclear): report NEEDS_CONTEXT, don't guess — C-10 applies to building this too.

## Task
1. Implement the three subcommands with the exact constraint sets above; `deskkit.Guard()`
   first; audit every path.
2. Port the JWT→installation-token exchange from mint-reviewer-token.go verbatim in behavior
   (same env vars, same endpoints); unit-test with a fake HTTP server.
3. Implement the C-3 validator as a standalone `internal/bodycheck` package inside deskpost's
   cmd tree with table-driven tests — every refusal pattern in facts has a positive (refused)
   and negative (clean body passes) case.
4. Implement C-2 ready preconditions with the TOCTOU re-read; test each precondition failing
   — including (e): risk-path PR, App-approved at head, green CI, NO `Security-Review: pass`
   line → exit 5 and the fake server records no flip call
   individually against a fake GitHub API (draft=false, approval-at-stale-head, red CI,
   unknown repo, API 500 → exit 6).
5. Negative tests additionally: oversized body, each token pattern, repeat-invocation noop
   (assert NO HTTP call made on the repeat — fake server records hits), rate-limit breach →
   exit 4, kill switch → exit 3.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/cmd/deskpost/... -count=1` | exit 0; includes every negative test named in Tasks 3-5, including the security-review refusal test (#216) |
| 2 | `go vet ./tools/desk/...` | exit 0 |
| 3 | `go build -o /tmp/deskpost ./tools/desk/cmd/deskpost && printf 'ghp_%s' "$(head -c 40 /dev/zero \| tr '\0' 'A')" > /tmp/dp-bad.md && /tmp/deskpost comment oit 1 --body-file /tmp/dp-bad.md; echo $?` | 5 (refused by secret scan; no network call — verify no comment appears on PR 1) |
| 4 | `DESK_TOOLS_DISABLED=1 /tmp/deskpost ready oit 1; echo $?` | 3 |
| 5 | live (post-06, desk-run): a real review posts as `reviewer-app[bot]` and a converged draft flips ready with zero prompts | observed in the review loop |
| 6 | `statusgen --root . --lint; echo $?` | 0 |

> Verify items 3-4 run the COMPILED binary (`go build -o /tmp/deskpost` first) rather than
> `go run`: on the go1.26 toolchain `go run` collapses a non-zero child exit to `1` (it prints
> `exit status N` to stderr but exits 1 itself), masking the tool's real 5/3 codes. The
> installed `/opt/desk-tools/bin/deskpost` returns the codes directly. (Sibling briefs that
> assert exit codes through `go run` — e.g. deskboard 02 item 4 — carry the same latent
> masking; flagged to the desk, not fixed here.)

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `7f524e40`):

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/desk/cmd/deskpost/... -count=1` | 0 | `ok github.com/medici/desk/cmd/deskpost`; `ok .../internal/bodycheck` | 2026-07-13 | opus-verifier |
| 2 | `go vet ./tools/desk/...` | 0 | no diagnostics | 2026-07-13 | opus-verifier |
| 3 | `deskpost comment <repo> 1 --body-file <body with ghp_ prefix>` | 5 | `refused: body contains a GitHub token prefix` — no network call, no comment posted | 2026-07-13 | opus-verifier |
| 4 | `DESK_TOOLS_DISABLED=1 deskpost ready <repo> 1` | 3 | `refused: desk tools disabled (result=disabled)` | 2026-07-13 | opus-verifier |
| 5 | live desk-run: review posts as `reviewer-app[bot]`, converged draft flips ready | UNRUN | partial: App APPROVED reviews observed on #392/#409/#412/#413/#415/#416 (read-only); the flip half is not observable as a distinct deskpost action from outside a desk loop — not assumed | 2026-07-13 | opus-verifier |
| 6 | `go run ./tools/statusgen --root . --lint` | 0 | NOTICEs only, no lint errors | 2026-07-13 | opus-verifier |

**VERIFY: FAIL** — the table is green (rows 1-4, 6; row 5 UNRUN), but the merged deliverable still carries
a **confirmed fail-open on its own ready gate**. Both standing findings reproduce at this SHA:

- **[F-29](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-deskpost-ci-rollup-unpaginated-latent-fail-open.md) CONFIRMED (executed, not inferred).** `combinedStatusAt` (`../assay-toolkit/tools/desk/cmd/deskpost/github.go:336`)
  and `checkRunsAt` (`github.go:345`) each issue ONE unpaginated request; `TotalCount` is parsed
  (`github.go:259,267`) but referenced nowhere outside test fixtures, and `evalCI` (`ci.go:27`) reduces only
  the returned slices. `listReviews`/`listFiles` do paginate — these two calls are the only unpaginated
  readers. Repro against the in-package fake: head with `total_count: 31`, page 1 = 30 successes, the 31st
  check run a `failure` on page 2 → one request issued, `runReady` exit 0, **flip executed on a PR whose CI
  is red**. This inverts the file's own stated contract (`ci.go:3-7`: "deliberately fail-closed (C-10)").
  Not triggerable on today's repos (~14 checks), but the defect is in the merged ready gate.
- **[F-30](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-deskpost-ready-cannot-flip-ci-less-report-repos-decks-reconciler-proposals.md) CONFIRMED.** `ciNotRequired` (`ci.go:22`) holds exactly one repo; `allowedRepos`
  (`deskkit/config.go:12`) holds seven after [F-23](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-10-review-verify-gates-cover-medici-finance-report-repos.md). `deskpost ready` therefore exits 6 (`unverifiable`) and
  can never flip `medici-finance/{decks,reconciler,proposals}` — corroborated live 2026-07-13: decks and
  reconciler 404 on `.github/workflows`, proposals is empty. Fails closed (operability gap, not a safety
  hole), but silently defeats brief-03's flip path on the repos [F-23](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-10-review-verify-gates-cover-medici-finance-report-repos.md) brought into the gate.

Held at `implemented`: a brief whose merged deliverable still carries a confirmed fail-open on its own ready
gate is not verified, however green its table. Fix routes through [F-29](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-deskpost-ci-rollup-unpaginated-latent-fail-open.md)/[F-30](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-deskpost-ready-cannot-flip-ci-less-report-repos-decks-reconciler-proposals.md) (both remain `resolved: false`).

---

**Re-verify (glm-5.2, non-implementer) — origin/main `421d7cde`, 2026-07-16.** [F-29](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-deskpost-ci-rollup-unpaginated-latent-fail-open.md) and [F-30](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-deskpost-ready-cannot-flip-ci-less-report-repos-decks-reconciler-proposals.md) are both fixed in code at this SHA (finding files flipped `resolved: true`); regression tests pin both:

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|------------|------|--------|
| 1 | `go test ./tools/desk/cmd/deskpost/... -count=1` | 0 | ok; TestReadyRedCheckOnPageTwoRefuses ([F-29](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-deskpost-ci-rollup-unpaginated-latent-fail-open.md)), TestReadyTruncatedCheckRollupFailsClosed, TestReadyEmptyCIReportReposGreen/{decks,reconciler,proposals} ([F-30](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-deskpost-ready-cannot-flip-ci-less-report-repos-decks-reconciler-proposals.md)), TestReadyEmptyCIAssayToolkitUnverifiable, 10 bodycheck refusal patterns | 2026-07-16 | glm-5.2-verifier |
| 2 | `go vet ./tools/desk/...` | 0 | no diagnostics | 2026-07-16 | glm-5.2-verifier |
| 3 | `deskpost comment` with ghp_ token body | 5 | refused: "body contains a GitHub token prefix" (bodycheck before client construction — no mint/HTTP) | 2026-07-16 | glm-5.2-verifier |
| 4 | `DESK_TOOLS_DISABLED=1 deskpost ready` | 3 | refused: desk tools disabled | 2026-07-16 | glm-5.2-verifier |
| 5 | live desk-run (App post + converge) | UNRUN | needs live pr-review-desk loop + App PEM; UNRUN-partial by design | 2026-07-16 | glm-5.2-verifier |
| 6 | `statusgen --lint` | 0 | NOTICEs only; none re desk-tools/03 | 2026-07-16 | glm-5.2-verifier |

**VERIFY: PASS** (rows 1-4, 6 green; row 5 UNRUN as designed). [F-29](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-deskpost-ci-rollup-unpaginated-latent-fail-open.md) (CI rollup paginated, total_count consulted) + [F-30](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-11-deskpost-ready-cannot-flip-ci-less-report-repos-decks-reconciler-proposals.md) (CI-less report repos via `deskkit.CIRequired`) both resolved with regression tests.

## Review
Gate: model — but this brief carries the suite's heaviest safety load: reviewer must verify
(a) no code path writes token/key/body material to audit/stdout, (b) the ready-flip
preconditions are re-verified in-tool (not caller-trusted) with the TOCTOU re-read present,
(c) refusal tests assert refusal + absence of network side effect. Run `/security-review` in
addition to `/code-review`.
