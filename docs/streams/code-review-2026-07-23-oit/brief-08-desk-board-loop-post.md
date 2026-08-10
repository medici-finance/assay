---
brief: code-review-2026-07-23-oit/08
title: desk board/loop/post robustness (T2, T4, T5, T6)
why: >-
  Four correctness bugs in the desk tooling that drive the review pipeline: deskboard fetches PRs
  and reviews with no pagination (default 30 / first-page-only), so beyond 30 open PRs it prints
  false "MERGED" tombstones and can FLIP off a stale page-2 CHANGES_REQUESTED; the boards print
  titles raw in the ACTIONABLE lane, so an ANSI/injection payload in a PR/issue title renders raw;
  the loopengine reclaim is stat→Remove→create (non-atomic) so two engines double-dispatch and the
  Branch field is never populated (branch-liveness guard dead); and deskpost's securityPassAtHead is
  order-insensitive, so a pass→fail→approve sequence at one head flips green.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-23 by Opus session (code-review-2026-07-23 authoring pass); relocated 2026-08-01 from oit:docs/streams/code-review-2026-07-23/brief-08-desk-board-loop-post.md by assay-selfcontain/03 (paths re-pointed local — tools/desk/ already lives in this repo)
sources: ["2026-07-23 codebase review, findings T2/T4/T5/T6", "SOURCE repo (findings sourced at): oit @ ef1de62a, tools/desk/ — code has since moved to this repo (medici-finance/assay-toolkit), so file paths below are local"]
exec-tier: strong
exec-tier-why: (c) T5 is a concurrency double-dispatch and T6 an order-sensitive gate flip — both are safety plumbing where a subtle error survives the brief's own tests.
---

# Brief 08 — desk board/loop/post robustness

## Provenance

Moved here from `oit` (oit) `docs/streams/code-review-2026-07-23/brief-08-*`
by `oit:assay-selfcontain/03`, because its fix targets `tools/desk/`, which is
canonical in **this** repo. **Alias**: this brief is `code-review-2026-07-23/08` in oit's own
cross-references — old links to that ID should repoint here
(`assay-toolkit:code-review-2026-07-23-oit/08`).

## Context
SOURCE repo (findings originally sourced from): **oit** @ `ef1de62a`.
The code itself has since moved here — these are oit-native desk commands, now living in this
repo's `tools/desk/` (Go).
files:
- tools/desk/cmd/deskboard/board.go (T2: `~100-102` `fetchOpenPRs` no `--limit` (default 30); `~113-123` `fetchReviews` first-page-only; false "MERGED" tombstones `~685-722`; `cmdQueue ~827` unpaginated) (T4: `~528,672,877` print titles raw in the actionable lane)
- tools/desk/cmd/issueboard/board.go (T4: `~553` prints titles raw)
- tools/desk/internal/loopengine/claim.go (T5: `~120-141` reclaim is stat→Remove→create (non-atomic); Branch never populated `~104` so branch-liveness guard is dead)
- tools/desk/cmd/deskpost/ready.go (T6: `~162-172` `securityPassAtHead` order-insensitive)
facts:
- T4 distinction: quarantine lanes already inert titles; the ACTIONABLE lane does not — that's the
  injection surface. Sanitize/escape titles before printing in the actionable lane; also track
  title-rename-after-bless.
- T6: the security verdict at a head must be order-sensitive — a later `fail` must not be overridden
  by an earlier/later `approve`; the LAST verdict (or any fail) at the head governs.
- Build/test: `cd tools/desk && go build ./... && go test ./... -count=1`.

## Status note (2026-08-01)

T2, T4, T5, and T6 were already fully implemented and tested on `main` before this brief was
relocated here — PR #219 (`fix(desk): order-sensitive security verdict, live branch guard,
paging, title sanitization`, merged 2026-07-31, Refs #216) ported the same four fixes from
`oit#1150` independently. Verified present: `prListLimit`/paged `fetchReviews`/paged `cmdQueue`
(T2, tools/desk/cmd/deskboard/board.go); `deskkit.StripControl`-sanitized ACTIONABLE-lane titles in both
boards (T4); O_EXCL-lock atomic reclaim + populated `Branch` field with a passing concurrency
test (`claim_race_test.go`, T5); order-sensitive `securityPassAtHead` (last verdict per author
governs, any-fail-at-head fails closed) with dedicated tests in `security_verdict_test.go` (T6).
The only gap against this brief's Verify table: row 5's `-run Order` selector matched zero tests
in `cmd/deskpost` (the existing ordering test there is `TestReadySecurityPassThenFailRefuses`,
which doesn't match the pattern). Closed by adding
`TestReadySecurityOrderPassFailApproveStaysRefused` — same pass→fail→approve scenario, named to
match. No functional change.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Commit only per task instructions.
- Stop at `implemented`. Report NEEDS_CONTEXT rather than guess on any gate semantics.
- Fail-closed: an ambiguous/paginated-truncation case must not read as MERGED/approved/green.

## Task
1. **T2** — `deskboard`: paginate `fetchOpenPRs` (raise/remove the 30-default limit) and
   `fetchReviews` (fetch all pages), and `cmdQueue`. Beyond 30 open PRs the board must not emit
   false "MERGED" tombstones or flip on a stale first page. Verify against a >30-PR fixture or a
   pagination unit test.
2. **T4** — `deskboard` + `issueboard`: escape/sanitize PR/issue titles before
   printing them in the ACTIONABLE lane (strip ANSI / control chars). Also flag a title changed
   after a bless.
3. **T5** — `loopengine`: make reclaim atomic (e.g. atomic rename / O_EXCL create / lock)
   so two engines cannot double-dispatch. Populate the `Branch` field so the branch-liveness guard
   is live. Add a concurrency test asserting no double-claim.
4. **T6** — `deskpost`: make `securityPassAtHead` order-sensitive so a `pass→fail→approve`
   sequence at one head does NOT read as green (any fail at the head, or the last verdict, governs).
   Add a test for the pass→fail→approve ordering.

## Verify (executable — from this repo's `tools/desk/`)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go build ./...` | exit 0 |
| 2 | `go test ./... -count=1` | exit 0; new T2/T5/T6 tests pass |
| 3 | `grep -n "limit\|Limit\|per_page\|paginat" cmd/deskboard/board.go` | pagination present on PR + review + queue fetches |
| 4 | `grep -n "O_EXCL\|Rename\|Lock\|atomic" internal/loopengine/claim.go` | reclaim is atomic; `Branch` populated |
| 5 | `go test ./cmd/deskpost/... -run Order -count=1` | exit 0; pass→fail→approve stays NOT-green |

## Evidence
<!-- one row per Verify item, filled by a non-implementer -->

## Review
Gate: model (review loop). Reviewer records verdict + date in the stream README table.
