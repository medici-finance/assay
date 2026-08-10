---
brief: code-review-2026-07-23-assay-toolkit/01
title: statusgen anti-falsification & tripwire robustness (T1, T3, T7, T8, T9, T10) [assay-toolkit]
why: >-
  Six fail-open holes in the review-gate machinery itself — the code that decides whether a human
  actually approved. A same-second edit doesn't void a Kryton blessing; the approval-phrase regex
  matches "do NOT approve"/"needs sign-off from X", corroborating a human stamp from negated text;
  a landed register-file RENAME leaves the old path a permanent false tombstone; idvalidate falls
  back to base=HEAD when origin/main is unresolvable, grandfathering the very ID it exists to
  reject; TrustedAuthor pins ids only on REST surfaces not the boards that feed work queues; and the
  scancoupling tripwire pins a FROZEN in-repo copy while the running statusgen is the release binary,
  so the drift it exists to catch stays green. Each is fail-open in anti-falsification — the worst
  failure direction for a trust gate.
wave: 0
depends: []
unblocks: []
effort: M
gate: human
gate-why: >-
  Implementation PRs modify statusgen anti-falsification/trust-machinery logic
  (blessing-void gate, approval corroboration, id grandfathering, unblock
  authorization, register tombstones) — changes to the review-gate's own
  integrity checks are human-gate per standing policy.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-23 by Opus session (code-review-2026-07-23 authoring pass)
sources: ["2026-07-23 codebase review, findings T1/T3/T7/T8/T9/T10", "canonical SOURCE: this repo (assay-toolkit) statusgen/; the oit tools/statusgen/ copy is FROZEN and does not run"]
exec-tier: strong
exec-tier-why: (c) anti-falsification safety logic — a subtle fix that is itself fail-open survives green tests; (b) T1 is a cross-repo coupling (frozen copy vs running release binary).
---

# Brief 01 — statusgen anti-falsification & tripwire robustness

## Context
SOURCE repo: **THIS repo (`assay-toolkit`)** `statusgen/` — this is the canonical statusgen that
runs as the pinned release binary. The `oit` `tools/statusgen/`
copy is FROZEN and does NOT run; fixing it changes nothing (that mismatch IS finding T1). Work
this brief in-repo. (Finding line numbers below cite the frozen oit copy; match by SYMBOL.)
files:
- `statusgen/trustgate.go` (T3: `.After(bless)` on ties — a same-second edit doesn't void a blessing, ~190/197/205/208; T10: `TrustedAuthor(login)` fed with no numeric id — board surfaces vs REST surfaces)
- `statusgen/corroborate.go` (T7: `approvalPhraseRe` `~48` matches negated text like "do NOT approve"/"needs sign-off from X")
- `statusgen/registers.go` (T8: `deletedRegisterFiles`/landed-set `~167` — a landed register-file RENAME leaves the old path a permanent false tombstone)
- `statusgen/idvalidate.go` (T9: `~57` `base := "HEAD"` fallback when `origin/main` unresolvable grandfathers the brand-new numeric ID the check exists to reject)
- `statusgen/scanissues.go` + the scancoupling tripwire test (T1: `unblockPlaceholders` non-bot gate `~617` treats ANY non-bot comment as "human answered", not specifically human:<name>; and the tripwire pins the frozen copy)
facts:
- "Fail-open" here = the anti-falsification check passes when it should flag. Every fix must move
  the ambiguous/tie/error case to FAIL-CLOSED (flag), never silently pass.
- human:<name> is human:<name>'s human GitHub account; agents post as the-org / App bots. "Human answered" must
  mean the numeric-id-pinned human account, not any non-bot login.
- Build/test: `cd statusgen && go build ./... && go test ./... -count=1`. Lint the streams tree:
  `cd statusgen && go run . --root .. --lint` (exit 0).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Commit only per task instructions.
- Stop at `implemented`. Report NEEDS_CONTEXT rather than guess on any trust semantics.
- Do NOT weaken an existing check — every change tightens fail-open toward fail-closed.

## Task
1. **T3** — `trustgate.go`: change the blessing-void comparison from `edited.After(bless)` to
   `!edited.Before(bless)` (i.e. an edit at the SAME timestamp as the bless voids it). A tie must
   fail-closed.
2. **T7** — `corroborate.go`: `approvalPhraseRe` must NOT corroborate negated/conditional text.
   Reject bodies containing negations ("do not approve", "not approved", "needs sign-off from",
   "pending approval", etc.) before matching, or match only an affirmative approval phrase. Add a
   test with the negated strings asserting NO corroboration.
3. **T8** — `registers.go`: a landed register file that is RENAMED must not leave its old path in
   the "landed" set forever (permanent false tombstone). Track landed entries by register ID /
   follow renames, not by frozen path. Add a test for a rename.
4. **T9** — `idvalidate.go`: when `origin/main` is unresolvable, do NOT silently fall back to
   `base=HEAD` (which grandfathers the new ID). Fail-closed: treat the ID as new (subject to the
   uniqueness check) or emit a hard PROBLEM that the base couldn't be resolved — never grandfather.
5. **T10** — `trustgate.go`: `TrustedAuthor` must pin the numeric account id on the BOARD surfaces
   that feed work queues, not only the REST surfaces (recycled-login defense). Ensure the
   board/queue path passes the numeric id.
6. **T1** — `scanissues.go` `unblockPlaceholders`: only a comment from the human:<name> (numeric-id-pinned
   human) account un-blocks a placeholder; ANY non-bot login must NOT. AND make the scancoupling
   tripwire assert the CANONICAL (this-repo) `scanRepos` shape, so a widening of the running binary
   trips it — a frozen-copy pin is a no-op. If the tripwire's cross-repo nature can't be closed
   here, add a failing-visible NOTICE/test documenting the gap and report NEEDS_CONTEXT.

## Verify (executable — from THIS repo `statusgen/`)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go build ./...` | exit 0 |
| 2 | `cd statusgen && go test ./... -count=1` | exit 0; new T3/T7/T8/T9/T1 tests pass |
| 3 | `grep -n "Before(bless)" statusgen/trustgate.go` | tie now fails-closed (`!edited.Before(bless)`) |
| 4 | `grep -n "not approve\|sign-off from\|negat" statusgen/corroborate.go` | negated text no longer corroborates |
| 5 | `grep -n "HEAD" statusgen/idvalidate.go` | no silent grandfathering fallback to base=HEAD |
| 6 | `cd statusgen && go run . --root .. --lint` | exit 0 (streams tree still lints clean) |

## Evidence
<!-- one row per Verify item, filled by a non-implementer -->

## Review
Gate: human. Implementation PRs modifying statusgen anti-falsification/trust-machinery logic escalate to human-gate.
