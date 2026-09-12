---
brief: assay:desk-tools:22
title: Trust-gate account-liveness check — a configured trusted login that's gone or banned should stop being honored
why: >-
  `TrustedAuthor`/`TrustedHumanAuthor` (trust.go) match a configured login string against
  `ASSAY_TRUSTED_LOGINS` / the bless authority forever, with no check that the GitHub account
  behind that login is still there. A deleted, renamed, or platform-banned account (compromised
  credential, ToS enforcement, a departed collaborator) keeps being honored as a trusted author
  until a human notices and hand-edits roster.env — an unbounded window where a name on a static
  list, not a live account, is the actual trust anchor. Cheapest closed form: periodically
  confirm each configured login still resolves to a live GitHub user account, and surface —
  fail-closed — the ones that don't.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-12 by an intake-desk session (driver-directed external prior-art review)
sources:
  - "Driver-directed review of ColeMurray/background-agents (\"Open-Inspect\", https://github.com/ColeMurray/background-agents) — its automation-ownership model binds authority to a live account, re-checked periodically, and pauses runs on suspension — the design gap named here"
  - "freshness-checked 2026-09-12 @ c0f0921e (assay origin/main): tools/desk/internal/deskkit/trust.go's TrustedAuthor/TrustedHumanAuthor/IsBlessAuthority do a pure case-insensitive string/ID match against EffectiveConfig() with no network call anywhere in the file — no liveness dimension exists today"
exec-tier: strong
exec-tier-why: >-
  Question (a) yes — cadence, caching, and the fail-closed default on an unverifiable account
  state are design decisions the facts don't fully pre-specify. Question (c) yes — this sits on
  the trust boundary that gates what inbound content a desk ever acts on; a bug that fails OPEN
  (treats "couldn't check" as "still trusted") reintroduces exactly the gap this brief closes.
---

# Brief 22 — Trust-gate account-liveness check

## Context
files: `tools/desk/internal/deskkit/trust.go` (new liveness check, additive — do not change
`TrustedAuthor`'s existing signature or callers), a new small file alongside it (e.g.
`trustliveness.go`) for the GitHub Users API call + result cache, `tools/desk/cmd/deskroster/`
(a new read-only verb or an extension to an existing one surfaces the result — see Task step 3),
`tools/desk/README.md` (document the new verb/output).
facts:
- `ASSAY_TRUSTED_LOGINS` / `ASSAY_BLESS_LOGIN` are the roster's trusted-identity list
  (`rosterconfig.go`, `EffectiveConfig()`) — comma-separated `login:id` pairs, read from
  adopter configuration outside any PR-authorable ref (trust.go's own header comment).
- GitHub's `GET /users/{username}` returns `404` for an account that no longer resolves under
  that login — deleted, renamed off it, or (empirically, not contractually documented by GitHub)
  a platform-suspended/banned account. A `200` with `type: "Organization"` where a `User` was
  configured is also a mismatch worth flagging (roster drift, not a banned account).
- No HTTP client exists in `deskkit` today for this purpose — this is new capability, not a
  rewire of an existing call.
- Today's checked identities: `ASSAY_TRUSTED_LOGINS=deviant-ozzie:97862316,jojig-dao:135049062,
  kryton:20959` plus `ASSAY_BLESS_LOGIN=kryton:20959` (this house's roster.env, read
  2026-09-12 — an example shape for the implementer, not a hardcoded value to bake in).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Fail-closed is mandatory, in both directions this brief could get wrong**: a login that
  cannot be verified (API error, rate limit, offline) must NEVER be silently treated as still
  live — surface it as `could-not-check`, distinct from `confirmed-live` and `confirmed-gone`.
  And `could-not-check` must NEVER itself flip a login to untrusted — that would let a transient
  API hiccup revoke a legitimate operator's trust. This brief adds a NOTICE/alarm surface; it
  does **not** wire the result into `TrustedAuthor`'s pass/fail return. Auto-revocation on
  `confirmed-gone` is an explicit follow-up, not this brief's scope (a wrong auto-revoke that
  silently drops a human's trusted status is a worse failure than a slow human-in-the-loop one).

## Task
1. Add a liveness checker in `deskkit` (new file, e.g. `trustliveness.go`): given a login, call
   `GET /users/{login}` (unauthenticated is fine — this is a public read) and classify the result
   as one of `confirmed-live` (200, `type` matches the roster entry's expected kind),
   `confirmed-gone` (404), `mismatch` (200 but `type` differs from expected), or `could-not-check`
   (network error, non-404 error status, rate-limited). Cache results in-process for the run
   (no persistent cache needed for v1 — this runs at most once per desk-boot/health-sweep cycle).
2. Add `TrustedLoginsLiveness() []LoginLivenessResult` iterating every configured login from
   `TrustedLogins()` (trust.go already exports this) plus the bless authority
   (`BlessAuthorityLogin()`), returning one result per login.
3. Surface it as a read-only verb — either a new `deskroster trust-health` subcommand (mirrors
   `deskroster preflight`'s pattern: one line per login, a NOTICE for `confirmed-gone`/`mismatch`,
   nothing for `confirmed-live`, and a distinct NOTICE for `could-not-check` so a blind sweep is
   never read as clean) or folded into an existing periodic sweep if one already runs at desk
   boot — the implementer picks based on what's cheapest to wire without duplicating a health-check
   loop; state which you chose and why in the PR description.
4. Document the new verb/output in `tools/desk/README.md`'s roster/trust section.

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/internal/deskkit/... -run TestTrustedLoginsLiveness -v` | exit 0; a table-driven test covers 200/live, 404/gone, 200-type-mismatch, and a simulated transport error (`could-not-check`), each asserting the correct classification via an injected HTTP round-tripper (no live network call in the test) |
| 2 | `go test ./tools/desk/internal/deskkit/... -run TestTrustedLoginsLiveness_FailClosed -v` | exit 0; a `could-not-check` result never appears as `confirmed-live` and is never silently dropped — the test asserts the result set's length equals the input login count even when every check errors |
| 3 | the new verb run against a roster with one login rewritten to a known-nonexistent GitHub username | output contains a NOTICE naming that login as `confirmed-gone`; exit code is 0 (a NOTICE, not a hard failure — this brief doesn't revoke anything) |
| 4 | `go build ./tools/desk/...` | exit 0 — no existing caller of `TrustedAuthor`/`TrustedHumanAuthor` changed signature |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: model (from frontmatter).
