---
brief: assay:assay:desk-tools:22
title: "Trust-gate account-liveness NOTICE — `deskroster liveness` reads what GitHub currently says about a trusted login, without touching `TrustedAuthor`'s verdict"
why: >-
  `TrustedAuthor` and `TrustedHumanAuthor` (trust.go) decide who a desk queue acts on by
  comparing a configured login (and, where pinned, its numeric id) against a static roster —
  a pure string/id comparison that never asks whether the GitHub account behind that login is
  still there. A deleted, renamed, or reclaimed trusted login keeps being honored exactly as
  before, indefinitely, because nothing ever re-checks it against the live account — the
  roster only changes when a human happens to notice and hand-edits it. This brief adds a
  separate, read-only check that looks up each configured identity on GitHub right now and
  prints a NOTICE when something changed, so a human has a chance to notice a stale or
  reclaimed login before it is abused. It deliberately changes nothing about who is trusted
  today: that stays the pass/fail path's own fail-closed logic, untouched. Wiring a liveness
  finding into that verdict is a separate, explicitly human-gated decision — not this brief.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [933]
schema: brief-v2
authored: 2026-09-12 by a worker-desk authoring session, from GitHub issue
  medici-finance/assay#933 (assay-issue-loop-app)
sources:
  - "GitHub issue medici-finance/assay#933 (assay-issue-loop-app), 2026-09-12: 'Authors desk-tools/22 — a trust-gate account-liveness check... This brief scopes a read-only, fail-closed liveness check surfaced as a NOTICE — it does not wire into TrustedAuthor's pass/fail return; auto-revocation is explicit follow-up.'"
  - "freshness-checked 2026-09-12 @ b35225c6 (origin/main) — `tools/desk/internal/deskkit/trust.go` still matches a configured login as a pure string/id comparison with no liveness read anywhere in the package; no `GetAccount`/liveness surface exists yet (`grep -rn GetAccount tools/desk` is empty at this commit)."
  - "The pattern reused: `tools/desk/internal/deskkit/repovis.go` § `RepoInfoFetcher` / `HTTPRepoInfoFetcher` / `stubRepoInfoFetcher` — a minimal, single-purpose read interface with a real HTTP implementation and a test stub, kept OUTSIDE the frozen `Forge` interface because the operation is not one every forge consumer needs. `tools/desk/internal/deskkit/repovis_http_test.go` is the hermetic httptest pattern for the real implementation's wire behavior."
  - "The existing account identity shape reused: `tools/desk/internal/deskkit/forge.go` § `Account{Login, ID}` (already the actor-identity shape every forge surfaces) and § `Forge` interface's freeze note (`adding a method requires a consuming tool in the same change` — the reason `GetAccount` is added as a plain method on `*GitHubForge`, never to the frozen interface, since GitLab account-liveness is explicitly out of scope here)."
  - "The transport reused: `tools/desk/internal/deskkit/forge_github.go` § `GitHubForge.doJSON` / `ForgeAPIError` / `IsForgeNotFound` — the same authenticated go-gh REST client and 404 classification every other GitHub read on this seam uses."
  - "The roster shape read: `tools/desk/internal/deskkit/rosterconfig.go` § `Config.Humans` / `Config.Bless` / `Config.Bots` — documented as GitHub-only maps (a GitLab identity lives only in `BotIdents`/`Logins`), which is why enumerating these three maps needs no forge filtering to stay GitHub-scoped."
  - "The consuming command's existing token/forge seam: `tools/desk/cmd/deskroster/forge.go` § `forgeFor` / `mintTokenFn` — the same per-repo minted-token resolution `deskroster list` already runs through."
exec-tier: strong
exec-tier-why: >-
  (a) the classification taxonomy (Alive/Renamed/Reclaimed/Deleted/Unpinned/CouldNotCheck) and
  the choice to keep `GetAccount` outside the frozen `Forge` interface are design decisions the
  facts above narrow but do not fully pre-specify; (c) this sits directly beside the trust
  gate's own code (same package, same file it doc-comments) — a subtle error that quietly
  widens the read surface into a `Trusted*`/`Blessed` return value, or that collapses "deleted"
  and "could not tell" into one bucket, would pass every happy-path test and only show up as a
  security regression once an abandoned login is actually reclaimed.
consumers:
  - "tools/desk/internal/deskkit/trust.go: fixed-here (one doc-comment sentence only) — every `Trusted*`/`Blessed`/`ItemTrusted*` function's behavior is explicitly out of scope and asserted unchanged by Verify row 9 and the absence-grep in row 10."
  - "tools/desk/internal/deskkit/forge_gitlab.go: out-of-scope (`GetAccount` is added to `*GitHubForge` only, never to the frozen `Forge` interface both implementations must satisfy, so GitLab needs no change) — GitLab account-liveness is untracked follow-up, named as such in the README update (Task step 6)."
version: 1
id: fd2202a7-8490-4e94-9327-dac1ee832c39
---

# Brief 22 — Trust-gate account-liveness NOTICE

## Dependencies
None.

## Context

single-point-of-failure: **a fetcher failure must classify as CouldNotCheck and never be
silently coerced into Alive.** Behind it: (1) `TrustedAuthor`/`TrustedHumanAuthor`/`Blessed`
are UNCHANGED by this brief and keep their own independent fail-closed defaults (an
unconfigured roster trusts nobody, a login carrying the wrong id is untrusted) regardless of
what this check ever reports — a bug in the new surface cannot widen who is trusted, because
the new surface has no read path INTO that decision; (2) the acting layer is a human reading
the NOTICE — nothing here auto-revokes, so a missed or wrong NOTICE has no automatic
consequence, which is exactly why auto-revocation is named in the issue as separate,
explicitly human-gated follow-up rather than bundled into this brief.

files:
- `tools/desk/internal/deskkit/trustliveness.go` (new) — `AccountFetcher`, `RosterIdentity`,
  `RosterIdentities`, `LivenessClass` + its five values, `LivenessFinding`,
  `CheckRosterLiveness`, `RenderLivenessNotices`.
- `tools/desk/internal/deskkit/trustliveness_test.go` (new).
- `tools/desk/internal/deskkit/forge_github.go` (existing file) — `GetAccount`, a new plain
  method on `*GitHubForge`, NOT added to the `Forge` interface.
- `tools/desk/internal/deskkit/forge_github_getaccount_test.go` (new) — hermetic httptest,
  modeled on `repovis_http_test.go`; no golden-corpus entry needed since `GetAccount` is
  outside the frozen interface.
- `tools/desk/internal/deskkit/trust.go` (existing file) — ONE doc-comment addition, no code
  change: a cross-reference from the package comment to `trustliveness.go`/
  `deskroster liveness`, stating explicitly that no `Trusted*`/`Blessed`/`ItemTrusted*`
  function reads it.
- `tools/desk/cmd/deskroster/liveness.go` (new) — `cmdLiveness`.
- `tools/desk/cmd/deskroster/liveness_test.go` (new).
- `tools/desk/cmd/deskroster/main.go` (existing file) — the `usage` text and the `liveness`
  case in the subcommand switch.
- `tools/desk/README.md` (existing file) — a short "Roster liveness" subsection.
- `changelog/desk-tools-22-trust-liveness.md` (planned) (new).

facts (read at `b35225c6`, 2026-09-12):
- `TrustedAuthor`/`TrustedHumanAuthor`/`TrustedAuthorID`/`Blessed` all resolve a login (and,
  where pinned, a numeric id) purely from `EffectiveConfig()` — a value parsed once from
  configuration, never a live forge read. Nothing in `trust.go` or its callers ever asks
  GitHub whether the account still exists. This is the gap the issue names.
- The roster's three GitHub-scoped identity maps are already exactly what a liveness check
  needs to enumerate: `Config.Humans map[string]int64` (login → pinned id, 0 = unpinned),
  `Config.Bless Identity{Login, ID}` (the single blessing authority — "an accountable human
  by construction", i.e. always a GitHub human, never a bot), and `Config.Bots map[string]int64`
  (GitHub App slug → bot user id, "GITHUB entries only" per its own doc comment). A GitLab
  identity lives exclusively in `Config.BotIdents`/`Config.Logins` and is untouched by this
  brief — reading only Humans/Bless/Bots is already GitHub-scoped with no filtering needed.
- `deskkit.Account{Login, ID}` (forge.go) is already the actor-identity shape every forge
  surfaces elsewhere in this package; the new fetcher returns `*Account`, not a new type.
- The `Forge` interface (forge.go) is FROZEN: "adding a method requires a consuming tool in
  the same change" and both implementations (GitHub, GitLab) must satisfy it. `GetAccount` is
  therefore added as a plain method on `*GitHubForge` ALONE, exactly as `RepoVisibility`'s
  pre-Forge-era sibling `HTTPRepoInfoFetcher.RepoVisibility(owner, repo string)` lives outside
  the interface too (repovis.go) — no GitLab implementation obligation, no interface-freeze
  churn, and an honest reflection that GitLab account-liveness is a different design this
  brief does not attempt.
- `GitHubForge.doJSON` already classifies a non-2xx as `*ForgeAPIError{Status,...}`, and
  `IsForgeNotFound(err)` already tells a 404 apart from every other failure — the exact
  distinction "deleted" (404) needs from "could not tell" (anything else: 500, timeout, auth
  failure). No new error-classification machinery is needed; `GetAccount` reuses both.
- `tools/desk/cmd/deskroster` already resolves a per-repo minted token and a `deskkit.Forge` through
  `forgeFor`/`mintTokenFn` (forge.go) for `deskroster list`'s PR reads. The same seam gives
  `deskroster liveness --repo OWNER/NAME` an authenticated `deskkit.Forge` to read from — the
  `--repo` flag exists only to resolve a token; `GET /users/{login}` is host-level and does
  not depend on which repo minted the credential.
- GitHub's REST semantics this design relies on, and no further: a login that no longer
  resolves to any account answers 404 (`IsForgeNotFound`). A login that still resolves
  answers 200 with the account's CURRENT canonical `id` and `login` — a renamed account keeps
  its id but its canonical login differs from the one configured; a reclaimed/squatted login
  answers with a DIFFERENT id than the one pinned in the roster. This design deliberately does
  NOT claim to detect a platform ban/suspension that leaves the same id resolving under the
  same login with a 200 — GitHub's public REST surface carries no reliable, permission-free
  "suspended" field, and asserting one here would be exactly the confident-but-unverifiable
  claim rule 11 warns against. That residual gap is recorded, not solved: existence (404) and
  identity continuity (id match) are the two signals this brief checks; a same-id, same-login,
  still-200 ban is out of scope and NOT claimed as covered.
- No credential or secret ever passes through this surface: the fetcher's only inputs are
  configured LOGIN STRINGS (already echoed in full by `EchoEffectiveConfig`, per the P3
  convention every desk tool already runs), and its only output is `{login, id, class}` per
  identity. There is nothing here for `Scrub` (brief-21) to redact, and this brief adds none.

## Ground rules
- **Never wire a `LivenessClass` into any `Trusted*`/`Blessed`/`ItemTrusted*` return value.**
  A diff that adds an early-return, a short-circuit, or even a log-only side channel inside
  `TrustedAuthor`, `TrustedHumanAuthor`, `TrustedAuthorID`, `trustedContentAuthor`, `Blessed`,
  `ItemTrusted`, or `ItemTrustedEvents` keyed on liveness is out of scope and a finding, full
  stop — auto-revocation is explicit, separately-gated follow-up per the issue.
- **Read-only, always.** `GetAccount` issues a GET and nothing else; `deskroster liveness`
  posts no comment, files no issue, edits no roster file, and mutates nothing on the forge.
- **An unconfigured roster refuses loudly.** It must never print the same "nothing to report"
  shape a genuinely all-clean, fully-configured run would print — that collapse is exactly the
  fail-open failure mode a "fail-closed" liveness check exists to avoid.
- **One identity's failure never suppresses another's finding.** A transport error on one
  login is that login's own `CouldNotCheck` finding; every other configured identity in the
  same run is still checked and reported.
- **A 404 and a transport error are never the same class.** `ErrAccountNotFound` (deleted)
  and every other failure (`CouldNotCheck`) must stay distinguishable — a stuck-open bisector's
  first question is always "did it not exist, or did we just fail to ask?"
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Core classifier — `trustliveness.go`.**
   - `AccountFetcher` interface: `GetAccount(login string) (*Account, error)` — one method,
     modeled on `RepoInfoFetcher`.
   - `RosterIdentity{Login string, PinnedID int64, Source string}` (`Source` ∈
     `"human"`/`"bless"`/`"bot"`) and `RosterIdentities(c Config) []RosterIdentity`, reading
     `c.Humans`, `c.Bless` (when `c.Bless.Login != ""`), and `c.Bots` — nothing else.
   - `LivenessClass` string constants: `LivenessAlive`, `LivenessRenamed` (same id, canonical
     login differs from configured), `LivenessReclaimed` (different id — a NEW account now
     answers to this login), `LivenessDeleted` (`errors.Is(err, ErrAccountNotFound)`),
     `LivenessUnpinned` (login still resolves to SOME account, but `PinnedID == 0` so identity
     continuity cannot be checked — recommend pinning), `LivenessCouldNotCheck` (any other
     fetcher error).
   - `LivenessFinding{Identity RosterIdentity, Class LivenessClass, Detail string}` and
     `CheckRosterLiveness(fetcher AccountFetcher, identities []RosterIdentity) []LivenessFinding`
     — one finding per identity, pure (no I/O beyond the fetcher call), never short-circuits on
     one identity's error.
   - `RenderLivenessNotices(findings []LivenessFinding) []string` — one `"NOTICE: ..."` line
     per finding whose class is NOT `LivenessAlive` (an alive identity produces no output — the
     same quiet-on-the-happy-path shape every other NOTICE in this codebase uses). Each class
     gets a distinguishable message shape so `LivenessReclaimed`/`LivenessDeleted` (the two the
     issue actually worries about) read differently from the advisory `LivenessRenamed`/
     `LivenessUnpinned`, which read differently again from `LivenessCouldNotCheck`.
2. **GitHub transport — `forge_github.go`.** `func (g *GitHubForge) GetAccount(login string)
   (*Account, error)`: `g.doJSON("GET", "/users/"+url.PathEscape(login), nil, &wire)` where
   `wire` decodes `id`/`login`; on `IsForgeNotFound(err)` return `(nil, ErrAccountNotFound)`;
   any other `doJSON` error is returned unchanged (the caller in step 1 classifies it
   `CouldNotCheck`). Not added to the `Forge` interface — `var _ AccountFetcher =
   (*GitHubForge)(nil)` is the only compile-time assertion needed.
3. **Consumer — `tools/desk/cmd/deskroster/liveness.go` (planned).** `cmdLiveness(args []string) error`:
   - Flag: `--repo OWNER/NAME` (required — resolves a minted token via the existing
     `forgeFor`/`mintTokenFn` seam in `tools/desk/cmd/deskroster/forge.go`; no other flags).
   - `cfg := deskkit.EffectiveConfig()`; `!cfg.Configured()` → `deskkit.Refused(...)` naming
     that the roster is unconfigured and nothing was checked (never a quiet 0-finding exit).
   - Resolve the repo's `deskkit.Forge` through `forgeFor`; type-assert to `*deskkit.GitHubForge`.
     A forge that is not GitHub-backed (or the assertion fails for any reason) produces exactly
     ONE `LivenessCouldNotCheck`-shaped line naming the gap ("GitHub-only in this version;
     GitLab account-liveness is untracked follow-up") — never silent skip, never a refusal
     (the roster itself may be perfectly configured; only THIS repo's forge is unsupported).
   - `identities := deskkit.RosterIdentities(cfg)`; `findings :=
     deskkit.CheckRosterLiveness(forge, identities)`; print
     `deskkit.RenderLivenessNotices(findings)` to stdout, one per line; print a one-line
     stderr summary (`N identities, M notices`). Exit `deskkit.ExitOK` whenever the check RAN
     to completion, whatever the findings say — this is a NOTICE surface, never a gate, so a
     `LivenessReclaimed` finding does not itself fail the command.
4. **Wiring — `tools/desk/cmd/deskroster/main.go`.** Add `case "liveness": err = cmdLiveness(rest)` to the
   subcommand switch and a `deskroster liveness --repo OWNER/NAME` line + one-paragraph
   description in the `usage` const: read-only, prints NOTICE lines, does not affect
   `TrustedAuthor`/`TrustedHumanAuthor`.
5. **Doc cross-reference — `trust.go`.** One sentence added to the package-level comment
   pointing at `trustliveness.go`/`deskroster liveness` as the companion read-only monitor,
   stating plainly that no function in this file consults it. No other line in `trust.go`
   changes.
6. **Docs — `tools/desk/README.md`.** A short "Roster liveness — `deskroster liveness`"
   subsection: what it checks (Humans/Bless/Bots against live GitHub), what it does NOT do
   (does not gate, does not auto-revoke, GitHub-only), and where auto-revocation is tracked as
   follow-up (name the issue this brief closes, #933, as the record of that follow-up).
7. **Changelog fragment** under `changelog/` (one `### Added` bullet).
8. **Nothing else.** No change to any `Trusted*`/`Blessed`/`ItemTrusted*` return value, no new
   write path, no GitLab implementation, no auto-revocation.

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 |
| 2 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCheckRosterLivenessClassifiesEveryCase$' -count=1` | exit 0 — a stub fetcher table drives Alive/Renamed/Reclaimed/Deleted/Unpinned/CouldNotCheck, INCLUDING a mixed batch where one identity errors and the others still get their own findings |
| 3 | check:ci +mutation | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCheckRosterLivenessNeverReportsAliveOnIDMismatch$' -count=1` | exit 0 — the NEGATIVE control: an id mismatch is never classified `LivenessAlive`. Mutation: with the `acct.ID != id.PinnedID` branch in `classifyLiveness` disabled, this test REDDENS (proving the guard is what the row exercises), and passes again once restored |
| 4 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestGetAccountDeletedIs404$' -count=1 && go test ./internal/deskkit/ -run '^TestGetAccountRenameKeepsIDMatchesDifferentLogin$' -count=1 && go test ./internal/deskkit/ -run '^TestGetAccountReclaimDifferentID$' -count=1` | exit 0 — the three live-wire shapes, hermetic httptest per `repovis_http_test.go`'s pattern |
| 5 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestGetAccountTransportFailureIsNotDeleted$' -count=1` | exit 0 — a 500/timeout is NOT `ErrAccountNotFound`, so it is never misclassified `LivenessDeleted` |
| 6 | check:ci | `cd tools/desk && go test ./cmd/deskroster/ -run '^TestLivenessCmdUnconfiguredRosterRefuses$' -count=1` | exit 0 — an unconfigured roster exits refused, printing zero findings for a reason NAMED as "unconfigured", never the same shape as a clean, fully-configured run |
| 7 | check:ci | `cd tools/desk && go test ./cmd/deskroster/ -run '^TestLivenessCmdReportsFindingsForConfiguredIdentities$' -count=1` | exit 0 — a configured roster against a stubbed GitHub forge prints one NOTICE per non-Alive identity, none for Alive ones, exit OK |
| 8 | check:ci | `cd tools/desk && go test ./cmd/deskroster/ -run '^TestLivenessCmdNonGitHubForgeNamesTheGap$' -count=1` | exit 0 — a non-GitHub-backed repo produces the explicit "GitHub-only" line, never silence and never a refusal |
| 9 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestTrustedAuthor$' -count=1 && go test ./internal/deskkit/ -run '^TestTrustedHumanAuthor$' -count=1 && go test ./internal/deskkit/ -run '^TestBlessed$' -count=1 && go test ./internal/deskkit/ -run '^TestItemTrusted$' -count=1` | exit 0 — every existing trust.go behavior test, byte-for-byte unchanged: this brief adds a sibling surface, it does not modify one |
| 10 | check:ci | `cd tools/desk && ( ! grep -n -e LivenessClass -e CheckRosterLiveness -e RenderLivenessNotices -e AccountFetcher internal/deskkit/trust.go )` | exit 0 — no reference from `trust.go` into the new surface: the ground rule holds by absence, not by promise |
| 11 | check:ci | `cd tools/desk && go test ./... -count=1` | exit 0 — the whole suite |
| 12 | check:ci | `cd tools/desk && gofmt -l internal/deskkit/trustliveness.go internal/deskkit/forge_github.go internal/deskkit/trust.go cmd/deskroster/liveness.go cmd/deskroster/main.go > /tmp/dt22-fmt.out; test ! -s /tmp/dt22-fmt.out` | exit 0 |
| 13 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| The classifier reports `LivenessAlive` even when the returned id doesn't match the pinned one | row 3 |
| A 404 (genuinely deleted) and a transport blip (500/timeout) collapse into the same "could not verify" bucket, hiding a real deletion behind "just a blip" | row 5 |
| One identity's transport failure aborts the whole batch, so findings for every OTHER identity are lost too | row 2 (mixed-batch case) |
| The command silently prints zero findings on an unconfigured roster, indistinguishable from "checked, all fine" | row 6 |
| A GitLab-backed roster entry (or repo) is silently skipped with no line at all | row 8 |
| The new surface quietly becomes a SECOND gate — a `Trusted*`/`Blessed` call site starts consulting `LivenessClass` | row 9 (unchanged tests) + row 10 (no reference from trust.go, by grep) |
| `GetAccount` is added to the frozen `Forge` interface instead of as a plain method, forcing an unwanted GitLab stub | row 1 (a real interface-signature change here fails `go vet`/`go build` against `forge_gitlab.go` unless a GitLab method is also added — the absence of that churn in the diff is a review-only check, not mechanically gated beyond compilation succeeding without touching `forge_gitlab.go`) |
| The `--repo` flag's forge resolution reuses a WRITE-capable token unnecessarily, widening this read-only tool's blast radius | review-only — the reused `forgeFor` seam is already read-scoped for `deskroster list`; a reviewer confirms no new scope is requested |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

### Implementer run — 2026-09-13, worker session (darwin/arm64, go1.26.5, offline)

Implementer evidence, NOT a verification: rows 1–13 run in the implementation worktree off
`refs/remotes/origin/main` @ `468a3082`. A non-implementer re-runs them at verify time.

| # | Exit | Key observed output |
|---|------|---------------------|
| 1 | 0 / 0 | `go build ./...` and `go vet ./...` both silent across the whole module |
| 2 | 0 | `TestCheckRosterLivenessClassifiesEveryCase` PASS — one mixed batch drives Alive/Renamed/Reclaimed/Deleted/Unpinned/CouldNotCheck together; the transport-error identity does not suppress the other 5 findings |
| 3 | 0 | `TestCheckRosterLivenessNeverReportsAliveOnIDMismatch` PASS — an id mismatch (pinned 42, live 99999) classifies `LivenessReclaimed`, never `LivenessAlive`. **Mutation demonstrated**: with `classifyLiveness`'s `acct.ID != id.PinnedID` branch short-circuited to `false`, the SAME test FAILs (`an id mismatch (pinned 42, live 99999) classified Alive — the negative control this test exists for`), confirmed by re-running with the mutation applied, then the file was restored byte-identical (`diff` confirmed) and the test re-passes |
| 4 | 0 / 0 / 0 | `TestGetAccountDeletedIs404`, `TestGetAccountRenameKeepsIDMatchesDifferentLogin`, `TestGetAccountReclaimDifferentID` all PASS against the hermetic httptest server |
| 5 | 0 | `TestGetAccountTransportFailureIsNotDeleted` PASS across 500/403/401 subtests — none classified `ErrAccountNotFound` |
| 6 | 0 | `TestLivenessCmdUnconfiguredRosterRefuses` PASS — refusal text contains "unconfigured", zero lines to stdout |
| 7 | 0 | `TestLivenessCmdReportsFindingsForConfiguredIdentities` PASS — stderr: `liveness: 4 identities, 2 notices`; exactly one RECLAIMED and one DELETED line, none for the two alive `ada` entries (human + bless) |
| 8 | 0 | `TestLivenessCmdNonGitHubForgeNamesTheGap` PASS — stderr: `liveness: 0 identities checked (non-GitHub forge), 1 notice`; stdout carries the explicit "GitHub-only" line |
| 9 | 0 / 0 / 0 / 0 | `TestTrustedAuthor`, `TestBlessed`, `TestItemTrusted` all PASS unchanged. `TestTrustedHumanAuthor` reports `[no tests to run]` — **could-not-check, not a pass**: no test of that exact name exists in this tree (verified via `grep -n 'func TestTrustedHumanAuthor' internal/deskkit/*_test.go`, zero hits; `TrustedHumanAuthor` itself is exercised incidentally inside `verifygatecard_test.go`, which also passes unchanged). This is a pre-existing gap in the brief's Verify row, not introduced by this change — flagged rather than silently rounded up to a pass |
| 10 | 0 | absence-grep passes: `trust.go` carries zero references to `LivenessClass`/`CheckRosterLiveness`/`RenderLivenessNotices`/`AccountFetcher` |
| 11 | not run in-agent | per worker-kit clause 12, the whole-module `go test ./...` is never run inside the agent. Bounded equivalents run instead: `go test ./internal/deskkit/... -count=1` → `ok` (33s) and `go test ./cmd/deskroster/... -count=1` → `ok` (3.2s); full-module `go build ./...` and `go vet ./...` (row 1) are clean. CI runs the full suite |
| 12 | 0 | `gofmt -l` on the five named files: empty output |
| 13 | 0 | `cd statusgen && go run . --root .. --lint` → `LINT: PASS`, exit 0 |

### Design deviation from the brief, recorded per worker-kit clause 7 (verify before applying)

The brief specifies `GetAccount` as a plain, exported method directly on `*GitHubForge`
(forge_github.go), modeled on `RepoInfoFetcher`/`HTTPRepoInfoFetcher`. That placement does
not compile clean against this tree: `tools/desk/internal/deskkit/forge_surface_test.go`'s
`TestForgeNoPassthrough/neither_backend_exports_a_method_outside_the_interface` — a
pre-existing invariant already present at the brief's own cited freshness-check commit
`b35225c6`, not something this change introduces — asserts by reflection that `*GitHubForge`'s
and `*GitLabForge`'s EXPORTED method sets equal the frozen `Forge` interface's exactly, with
no exceptions. An exported `GetAccount` on `*GitHubForge` trips it directly (reproduced: `go
test ./internal/deskkit/ -run '^TestForgeNoPassthrough$'` FAILs with "GitHubForge exports 1
method(s) outside the frozen Forge surface: GetAccount").

The brief's own cited precedent is actually the correct fix, just mis-attributed to the
wrong type: `RepoVisibility` is not a method on `*GitHubForge` either — it lives on
`HTTPRepoInfoFetcher`, a standalone fetcher struct that never implements `Forge` at all, so
the closed-surface test never sees it. This change places `GetAccount` the same way —
`HTTPAccountFetcher` (`tools/desk/internal/deskkit/trustliveness.go`), a standalone struct, not a
`GitHubForge` method — which satisfies every stated intent of the brief (no Forge interface
change, no GitLab implementation obligation, `AccountFetcher` still takes any fetcher a
caller hands it) while compiling clean against the pre-existing invariant. The consumer
(`tools/desk/cmd/deskroster/liveness.go`) still resolves the repo's `Forge` through the existing
`forgeFor` seam to decide GitHub-vs-not and to read the already-minted
`Token`/`BaseURL`/`Client` off the resolved `*deskkit.GitHubForge` value (plain exported
FIELD reads, not a method call), then hands those to `HTTPAccountFetcher` — the same
authenticated transport, just not reached through a new `GitHubForge` method. `trust.go`,
`forge_github.go`'s existing methods, the `Forge` interface, and `forge_gitlab.go` are all
untouched by this deviation.

## Review

Gate: model (all four risk answers no). Model-gated because the two hazards this change could
carry are both mechanically bounded here: the "widens who is trusted" hazard by row 10's
absence-grep plus row 9's unchanged trust-test suite, and the "hides a real problem behind a
false could-not-check, or the reverse" hazard by rows 3 and 5's discriminating classifier
tests. The reviewer confirms in the verdict: (1) that no `Trusted*`/`Blessed`/`ItemTrusted*`
function's source changed at all (a diff limited to the files this brief names, with `trust.go`
touched by exactly one doc-comment line); (2) that `LivenessDeleted` and `LivenessCouldNotCheck`
stay genuinely distinguishable in the implementation, not just in the type names; and (3) that
the unconfigured-roster path is a real refusal (`deskkit.Refused`, non-zero exit) rather than a
quiet empty-findings success.
