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
### Non-implementer verifier run — VERIFY: FAIL (row 7 — pre-existing unrelated whole-module red, not this brief's defect) — 2026-09-06 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `67abbac`

Runner ≠ implementer. Isolated worktree off origin/main. Offline (`KUBECONFIG=/dev/null`); module-scoped from `tools/desk/`. Frontmatter: `gate: model`, all risk `no`, `irreversible: no`.

| # | command | expected | exit / observed | Date | Runner |
|---|---------|----------|-----------------|------|--------|
| 1 | tools/desk go build ./... && go vet ./... | exit 0 | exit 0, clean | 2026-09-06 | opus-4.8[1m]-verifier |
| 2 | go test ./cmd/desktoken/ -run coverage-lists-every-installation | exit 0 | exit 0 — RUN + PASS; ok desktoken | 2026-09-06 | opus-4.8[1m]-verifier |
| 3 | go test ./cmd/desktoken/ -run coverage-repo-filter-exit-codes | exit 0 | exit 0 — RUN + PASS | 2026-09-06 | opus-4.8[1m]-verifier |
| 4 | go test ./cmd/desktoken/ -run coverage-page-failure-is-unverifiable | exit 0 | exit 0 — RUN + PASS | 2026-09-06 | opus-4.8[1m]-verifier |
| 5 | go test ./cmd/desktoken/ -run coverage-writes-no-cache-and-prints-no-token | exit 0 | exit 0 — ok desktoken (token bytes appear nowhere on stdout/stderr/audit) | 2026-09-06 | opus-4.8[1m]-verifier |
| 6 | go test ./cmd/desktoken/ -run coverage-refuses-gitlab-forge | exit 0 | exit 0 — ok desktoken | 2026-09-06 | opus-4.8[1m]-verifier |
| 7 | cd tools/desk && go test ./... -count=1 | exit 0 | **exit 1 FAIL** — two deterministic reds in internal/deskkit (registry-covers-cmd-binaries: `deskinstall` not registered in canonical tool-keys; restamp-recovery floor: below-floor tier = notice-allow, want floor-refuse). PRE-EXISTING on merged main 67abbac, OUTSIDE dt09's deliverables (cmd/desktoken); last touched by unrelated PRs. Filed medici-finance/assay#555 | 2026-09-06 | opus-4.8[1m]-verifier |
| 8 | gofmt -l tools/desk/cmd/desktoken | empty | exit 0, empty (nothing unformatted) | 2026-09-06 | opus-4.8[1m]-verifier |
| 9 | statusgen --root .. --lint | exit 0 | exit 0 — LINT: PASS (notices only) | 2026-09-06 | opus-4.8[1m]-verifier |

`RISK-VALUE: DERIVED — coveragePerPage = 100 @ tools/desk/cmd/desktoken/coverage.go:58 — GitHub REST's documented per_page maximum. The completion test len(page) < coveragePerPage @ coverage.go:323 is only sound when the requested size does not exceed the server cap: a value above 100 would silently truncate (server returns ≤100, the break fires after page one, dropping later pages); 100 sits exactly at the cap. Reversible (edit + rebuild).`
`RISK-VALUE: N/A (token-redaction) — no literal constant governs redaction; the control is the absence of token bytes from output/audit, verified structurally by row 5.`

**VERIFY: FAIL — row 7.** desk-tools/09's own deliverable verifies CLEAN — rows 1-6, 8, 9 PASS, including the token-redaction (row 5) and the three-state page-failure (row 4) guarantees. The single failing row is a PRE-EXISTING whole-module red on merged main `67abbac` in an UNRELATED package (deskkit: `deskinstall` tool-key registration + a model-stamp floor-recovery weakening), surfaced by row 7's broad `go test ./...`, not introduced by this brief — filed medici-finance/assay#555. desk-tools/09 flips to `verified` once #555 is triaged and row 7 goes green. Status stays `implemented`. CFR sidecar row appended (verify-fail, 8/9).

<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

## Review

Gate: model (all four risk answers no). The reviewer records verdict + date in the stream README
table and confirms: the verb performs no disk write under the config home (row 5), prints no
credential material (row 5), and cannot present a partial enumeration as complete (row 4).
