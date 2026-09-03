---
brief: desk-tools/09
title: "`desktoken coverage <role>` — list the repositories a role's App installations can see"
why: >-
  Before a roster flip or a cross-org dispatch, the coordinator has to answer one question:
  does role X's App see repo Y? Today the answer is a hand-written probe — sign an RS256 JWT,
  list the App's installations, mint a token per installation, list its repositories — and a
  24-hour sweep of fifteen desk-role and worker session transcripts found one coordinator
  session hand-writing that probe eight times. Every hand-written probe handles the private
  key and a fresh token in an ad-hoc shell, and every one is a chance to print either.
  `desktoken` already signs the JWT and already lists installations to resolve one; the
  read-only enumeration is the same calls with the answer printed instead of discarded.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-02 by a worker-desk authoring session, from a 24-hour transcript sweep across
  fifteen desk-role and worker sessions (tallied per session)
sources:
  - "freshness-checked 2026-09-02 @ 547b708 — `tools/desk/cmd/desktoken/main.go` usage offers `desktoken <role> [--repo] [--ttl] [--fresh]` and `--forge gitlab` only; `desktoken.go` § resolveInstallID lists `GET /app/installations` to match ONE owner and discards the rest; no verb enumerates installations or their repositories."
  - "The JWT, installation and token-exchange primitives this reuses: `tools/desk/cmd/desktoken/desktoken.go` — `buildJWT`, `resolveInstallID`, `exchangeJWT`, `writePerms`, the `<role>-token-<installID>` cache naming."
  - "The per-owner token semantics the output must reflect: `tools/desk/internal/deskkit/roletoken.go` § RoleTokenForOwner (an installation token resolves only ITS installation's repositories)."
  - "Forge API base and the audit contract: `tools/desk/internal/deskkit/forge_github.go`, `audit.go`; exit codes `exitcodes.go`."
  - "Brief and Verify shape: `spec/brief-v1.md`; status semantics: `spec/lifecycle-v1.md`."
exec-tier: strong
exec-tier-why: "(c) auth plumbing — a per-installation token written into the wrong cache
  slot, or echoed in a debug line, survives every test that only checks the repo list."
---

# Brief 09 — `desktoken coverage <role>`: list the repositories a role's App installations can see

## Dependencies
None. Every API call the verb makes is one `desktoken` already makes to mint a token.

## Context

risk note — this brief declares `tools/desk/cmd/desktoken/main.go`, which is on the security-path
trigger list, while answering all four risk questions "no". The answers stand and here is why,
so a reviewer checks the reasoning rather than re-deriving it: the verb is read-only on the forge
and on disk, mints nothing that outlives the call, writes no cache and no sidecar, and prints no
credential material; it changes no trust decision any other verb makes, and the minting path it
sits beside is untouched. If a reviewer finds a write or a printed secret, the correction is to
flip `sensitive-data` to yes and take the human gate — not to leave the answers as-is.

files:
- `tools/desk/cmd/desktoken/main.go` (usage, `coverage` dispatch)
- `tools/desk/cmd/desktoken/coverage.go` (planned) — enumeration and rendering
- `tools/desk/cmd/desktoken/coverage_test.go` (planned) — an `httptest` GitHub with two installations
- `tools/desk/README.md` (contract)

facts:
- today's flow (`desktoken.go` § cmdToken, checked 2026-09-02): read PEM + App ID from the
  search path → `buildJWT` → `resolveInstallID(jwt, owner)` → `exchangeJWT(jwt, installID)` →
  cache as `<role>-token-<installID>` (0600) + `.perms` sidecar → print the PATH.
- `GET /app/installations` (JWT-authenticated) returns every installation of the App with
  `id`, `account.login`, `account.type`, `repository_selection` (`all` or `selected`).
- `GET /installation/repositories` (installation-token-authenticated, paginated by `page`/
  `per_page=100`) returns the repositories THAT installation can see, `full_name` each.
- the verb is READ-ONLY on the forge and on disk: it never writes a token cache or a `.perms`
  sidecar (a cache written under enumeration would shadow the next real mint's permission
  view — the exact masking `--fresh` exists to undo). Tokens minted for the listing are held
  in memory and dropped.
- output, one block per installation, stable order (by `account.login`):
  `installation <id> account=<login> type=<Org|User> selection=<all|selected> repos=<n>` then one
  indented `<owner/name>` line per repository, sorted. A `--repo <slug>` filter prints only the
  installation that sees it and exits 0 if one does, **5 if none does** — the question the
  sweep was asking, answered by exit code. `--json` emits the same as one object.
- pagination: follow until a page returns fewer than `per_page`; a page read that fails is
  **exit 6 naming the installation** — never a shorter list presented as complete
  (three-state: could-not-check is not "not covered").
- secrets: neither the JWT nor any installation token is printed, logged or audited; the
  audit line records role, installation count, repository count, filter, result.
- tests: `httptest.Server` standing in for the API base (`deskkit.GitHubAPIBase` is a var the
  existing tests already redirect), two installations (one `all`, one `selected` with two
  pages), a fixture PEM generated in the test — nothing contacts the real forge.

## Ground rules
- No test or Verify row contacts the real forge; the `httptest` fixture is the only server.
- Never print a token or a JWT; never write a token cache from this verb.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **`desktoken coverage <role> [--repo <slug>] [--json]`** in `main.go`, taking `Guard()` and
   the audit line like every other verb; `--forge gitlab` refuses with exit 5 naming the
   GitHub-only scope (a PAT has no installation to enumerate).
2. **Enumeration** in `coverage.go`: reuse `buildJWT`; list installations; per installation
   `exchangeJWT` into memory only; page `/installation/repositories`; render per the facts.
   No cache write, no `.perms` write — assert this in a test by checking the search-path head
   directory is byte-identical before and after.
3. **`--repo` filter** with the 0 / 5 exit contract; `--json` shape documented in the README.
4. **Tests**: two-installation fixture; pagination across two pages; `--repo` hit exits 0 and
   names the installation; `--repo` miss exits 5; a failing second page exits 6 and prints no
   partial list; no token bytes in stdout/stderr/audit (grep for the fixture token value); no
   file created under the config home.
5. **README contract** paragraph under `desktoken`.
6. **Nothing else.** No change to minting, caching or `--fresh`.

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 |
| 2 | check:ci | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestCoverageListsEveryInstallation$' -count=1` | exit 0 — two installations, sorted, repo counts match the fixture including the paginated one |
| 3 | check:ci | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestCoverageRepoFilterExitCodes$' -count=1` | exit 0 — a seen repo exits 0 naming its installation; an unseen repo exits 5 |
| 4 | check:ci | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestCoveragePageFailureIsUnverifiable$' -count=1` | exit 0 — a 500 on page two exits 6 and the output carries NO repository lines (never a shorter list read as complete) |
| 5 | check:ci | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestCoverageWritesNoCacheAndPrintsNoToken$' -count=1` | exit 0 — the config-home head directory is unchanged and the fixture token bytes appear nowhere on stdout, stderr or the audit line |
| 6 | check:ci | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestCoverageRefusesGitLabForge$' -count=1` | exit 0 — exit 5 before any network call |
| 7 | check:ci | `cd tools/desk && go test ./... -count=1` | exit 0 |
| 8 | check:ci | `gofmt -l tools/desk/cmd/desktoken > /tmp/dt-fmt.out; test ! -s /tmp/dt-fmt.out` | exit 0 |
| 9 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| A page fails and the list prints short, read as "not covered" | row 4 |
| The enumeration token is cached and masks the next real mint's permissions | row 5 |
| A token or the JWT reaches stdout in a debug line | row 5 |
| `--repo` match done on `name` not `full_name`, so a same-named repo in another org matches | row 3 (the fixture carries a same-named repo under the other installation) |
| Installations rendered in API order, so two runs differ | row 2 (sorted assertion) |
| The verb runs against GitLab and prints nothing useful with exit 0 | row 6 |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

## Review

Gate: model (all four risk answers no). The reviewer records verdict + date in the stream README
table and confirms: the verb performs no disk write under the config home (row 5), prints no
credential material (row 5), and cannot present a partial enumeration as complete (row 4).
