---
brief: forge-neutral/04
title: Write verbs B — deskpr, deskfile, deskclose and deskevidence onto the resolver
why: >-
  These four carry the rest of the fleet's outward writes — opening a change, filing an issue,
  closing one, and landing an Evidence row. Three of them shell `gh` under the caller's ambient
  credential by documented design, and deskevidence performs a GitHub App JWT exchange and a
  Contents-API write end to end. Routing them through the resolver is what makes "the desk
  verbs are the only sanctioned write path" true rather than aspirational, and it is where the
  identity-class blocker in the permit register actually gets answered instead of moved.
wave: 2
depends: ["forge-neutral/01"]
unblocks: ["forge-neutral/10"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: [274]
schema: brief-v1
authored: 2026-09-02 by forge-neutral authoring session
sources:
  - "docs/streams/forge-neutral/brief-01-forge-resolution-contract.md — the resolver and its custody binding"
  - "tools/desk/internal/forgeban/allowlist.go:21-31 — the (identity) blocker class these three rows carry, and why retiring them is a custody decision"
  - "docs/streams/forge-gitlab/pilot-report.md D-8 — on GitLab the verifier's Evidence row has no direct-main lane and must travel as a merge request"
  - "freshness-checked 2026-09-02 @ deae247 — deskpr shells gh (exec.go:87, create at deskpr.go:293); deskfile at exec.go:50 (create at deskfile.go:473); deskclose at exec.go:48 (close at github.go:193-195); deskevidence builds net/http against deskkit.GitHubAPIBase (github.go:29), mints an installation token (:220) and writes via the Contents API (:358)"
exec-tier: strong
exec-tier-why: "it changes WHICH identity performs three writes that currently run under an ambient credential, and it re-homes the Evidence landing — a subtle error here writes as the wrong actor or lands Evidence the Evidence-actor check will then wrongly believe (questions a and c)."
gate-why: >-
  `deskfile`, `deskclose` and `deskpr` reach the forge under the caller's ambient CLI
  credential by documented design — `deskfile` states it "NEVER mints an App installation
  token; there is no desktoken call on any path". Routing them through the resolver changes WHO
  performs the write, which the permit register explicitly calls a token-custody decision and
  not a transport change. The human is confirming the new acting identity for each of the
  three, and separately confirming the Evidence-landing lane: on a forge with no direct-default-
  branch push, the verifier's Evidence row must travel as a change with a reviewer verdict, and
  that structural difference must be a stated design, not a discovered one.
domain: complicated
consumers:
  - "tools/desk/cmd/deskpr: fixed-here"
  - "tools/desk/cmd/deskfile: fixed-here (acting identity changes — see gate-why)"
  - "tools/desk/cmd/deskclose: fixed-here (acting identity changes — see gate-why)"
  - "tools/desk/cmd/deskevidence: fixed-here"
  - "tools/desk/internal/forgeban/allowlist.go: fixed-here (three rows removed, ceiling lowered to 14)"
  - "plugins/assay/skills/verify-desk/SKILL.md: follow-up forge-neutral/10 (the Evidence-landing lane gains a hop on a forge with no direct-default-branch push; the conformance round trip is where the loop shape is proved before the skill text is changed)"
---

# Brief 04 — Write verbs B: deskpr, deskfile, deskclose, deskevidence

## Context
files:
- `tools/desk/cmd/deskpr/exec.go`, `tools/desk/cmd/deskpr/deskpr.go` — draft-change creation.
- `tools/desk/cmd/deskfile/exec.go`, `tools/desk/cmd/deskfile/deskfile.go` — issue filing.
- `tools/desk/cmd/deskclose/exec.go`, `tools/desk/cmd/deskclose/github.go`,
  `tools/desk/cmd/deskclose/authority.go` — issue/change closing and the authority read.
- `tools/desk/cmd/deskevidence/github.go`, `tools/desk/cmd/deskevidence/deskevidence.go` —
  the App mint and the Evidence write.
- `tools/desk/internal/forgeban/allowlist.go` — three permit rows removed, ceiling lowered.

single-point-of-failure: for the three ambient-credential verbs the single control is WHICH
token the resolver hands the backend — get it wrong and the write lands as an identity nobody
chose. Two independent layers: the backends refuse an unminted token outright
(`forge_github.go:68-72`, `forge_gitlab.go:108-112`), and the commit-identity preflight
independently compares the resulting actor against the roster entry (`preflight.go:752-781`) —
a credential fault and an actor fault trip different checks in different components.

facts:
- The permit register's identity class covers exactly these rows: `deskclose/exec.go::runGH`
  (`allowlist.go:74`), `deskfile/exec.go::gh` (`:96`), `deskpr/exec.go::gh` (`:143`). Its
  header states each tool *"gates WHETHER and WHAT, never WHO"* and mints no token on any
  path, so routing it through the seam *"changes WHO performs the write, which is a
  token-custody decision"* (`allowlist.go:22-31`).
- `deskpr` is the exception among the three: it DOES mint a worker installation token
  (`tools/desk/cmd/deskpr/exec.go:30,90`) but retains a documented `--as-app=false` ambient fallback
  (`tools/desk/cmd/deskpr/deskpr.go:218`).
- `deskevidence` is fully GitHub-App-shaped: `RoleAppLogin("verifier")`
  (`tools/desk/cmd/deskevidence/github.go:44-46`), installation lookup (`:109`), JWT→installation-token
  exchange (`:220`), and the Evidence write itself via `PUT /repos/…/contents/…` (`:358`),
  with the bot commit identity built at `:455-456`.
- `Forge` already carries `CreateDraftChange` (`forge.go:196`), `FileIssue` (`:205`) and
  `CloseIssue` (`:207`). The Contents-API write `deskevidence` performs is **not** an
  enumerated operation and has no `Forge` method today.
- On the pilot, the default branch was push = No one for every identity including the owner,
  so the GitHub carve-out that lets the verifier commit an Evidence row directly has no
  GitLab equivalent; the Evidence row travelled as a merge request with a reviewer verdict
  (`docs/streams/forge-gitlab/pilot-report.md` D-8).
- `#274` reports that `forge-gitlab/07`'s call-site migration for `deskpr`/`deskfile`/
  `deskclose` did not land, and that its Verify row 3 passed vacuously because it grepped for
  `exec.Command("gh")` while the tools shell `gh` through a `runCmd` wrapper. This brief
  closes that gap for those verbs; the ban test and row 7 below are written against the
  wrapper form.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Every verb's existing test suite stays green **unmodified**.
- Do not remove or weaken the `--as-app=false` refusal semantics without saying so in the PR;
  narrowing an escape hatch is in scope, widening one is not.

## Task
1. **Name the acting identity first, then migrate.** For `deskfile`, `deskclose` and `deskpr`,
   record in the PR body which role identity each verb writes as after this change and why —
   this is the custody decision the permit register defers, and it is what the human gate
   reviews. Retire `deskpr`'s ambient `--as-app=false` fallback or state in the same place why
   it survives.
2. Route `deskpr`'s draft-change creation through `CreateDraftChange`, `deskfile`'s filing
   through `FileIssue`, `deskclose`'s close through `CloseIssue` and its authority read
   through `GetIssue`/`GetPullRequest`. Delete the shell helpers and their permit rows.
3. **Evidence landing.** Add ONE typed operation to `Forge` for writing a file at a path on a
   branch, with `deskevidence` as its consuming call site in the same change; implement it on
   both backends (GitHub Contents API ↔ GitLab Repository Files API) and record it in
   `docs/streams/forge-gitlab/inventory.md`. Delete `deskevidence`'s `apiBaseURL` and its
   hand-rolled JWT/installation exchange, which moves to the resolver's custody binding.
4. **The Evidence lane on a forge with no direct-default-branch push.** `deskevidence` asks
   the resolved forge whether the default branch accepts a direct write. When it does not, it
   lands the Evidence row on a branch and opens a draft change instead — and says so on
   stdout. It does NOT attempt the direct write and report success, and it does NOT silently
   skip the row.
5. Lower `allowedInvocationCeiling` from 17 to **14** and remove the three retired rows.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | `cd tools/desk && go test ./cmd/deskpr/... ./cmd/deskfile/... ./cmd/deskclose/... ./cmd/deskevidence/... -count=1` | exit 0 — all four suites green |
| 3 | `git diff --stat origin/main -- tools/desk/cmd/deskpr tools/desk/cmd/deskfile tools/desk/cmd/deskclose tools/desk/cmd/deskevidence \| grep -c '_test.go' \|\| true` | prints `0` — no verb's own tests were edited to make the migration pass |
| 4 | `grep -n 'allowedInvocationCeiling' tools/desk/internal/forgeban/allowlist.go` | shows `= 14` |
| 5 | `cd tools/desk && go test ./internal/forgeban/... -count=1 -v` | exit 0 — the ratchet passes at 14 |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1 -v` | exit 0 |
| 7 | `grep -rnE -e 'runCmd\([^)]*"gh"' -e 'runGH\(' -e 'exec\.Command(Context)?\([^)]*"gh"' tools/desk/cmd/deskpr tools/desk/cmd/deskfile tools/desk/cmd/deskclose --include='*.go' \| grep -v _test.go \| wc -l` | prints `0` — written against the WRAPPER form, which is what `#274` reports the old row was blind to |
| 8 | `grep -rn -e 'apiBaseURL' -e 'access_tokens' tools/desk/cmd/deskevidence --include='*.go' \| grep -v _test.go \| wc -l` | prints `0` — the hardcoded host and the hand-rolled installation exchange are gone, not merely unused |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run TestWriteFileOpBothBackends -count=1 -v` | exit 0 — the new file-write op runs the same scenario names against both backends' recorded fixtures |
| 10 | `cd tools/desk && go test ./cmd/deskevidence/... -run TestEvidenceLandsAsChangeWhenDefaultBranchClosed -count=1 -v` | **negative path**: with the resolved forge reporting the default branch not directly writable, the run opens a draft change and performs NO direct write — asserted by a recording transport showing zero direct-write calls — and exits 0 with the change named on stdout |
| 11 | `cd tools/desk && go test ./cmd/deskfile/... ./cmd/deskclose/... -run TestRefusesUnmintedToken -count=1 -v` | **negative path**: with no minted token for the resolved forge, each verb refuses (class 5) naming the remedy and reads NO ambient credential — asserted by an environment carrying a decoy ambient credential that must go untouched |
| 12 | `cd tools/desk && go test ./cmd/deskpr/... -run TestAmbientFallback -count=1 -v` | exit 0 — whichever way task 1 rules on `--as-app=false`, the test states the ruling: either the flag is gone and its use is refused, or it survives with its refusal semantics unchanged |
| 13 | `statusgen --root . --consumers --brief forge-neutral/04` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |

## Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| The migration lands but the wrapper-shelled `gh` path stays reachable — `#274` repeated verbatim | row 7, written against `runCmd(…, "gh", …)` / `runGH(` rather than `exec.Command("gh")`; plus row 6's launch-site walk as a second instrument |
| The acting identity silently changes and nobody notices which account now files issues | row 11 (refuses without a minted token, so the identity is never implicit) + the human gate, which reads task 1's recorded decision |
| An ambient credential is quietly used as a fallback when the mint fails | row 11's decoy credential must go untouched |
| A file-write op is added with no consuming call site, violating the freeze rule | row 9 + the inventory entry |
| On a forge with no direct-default-branch push, `deskevidence` reports success having written nothing | row 10 asserts a change was opened AND zero direct writes occurred |
| The Evidence row is skipped entirely rather than re-routed, so a verified brief has no Evidence | row 10 asserts exit 0 with the change named — a skip would produce neither |
| The ratchet is not lowered so the gain is not locked in | rows 4 + 5 |
| `deskevidence`'s bot commit identity is lost when the JWT exchange moves to the resolver | row 2 (its suite covers the commit identity) + `TestCommitIdentityPerForge` (planned) from forge-neutral/02 |
| The verify-desk skill text still describes a direct-to-default-branch Evidence landing | **no row** — deliberately deferred to forge-neutral/10, which proves the loop shape before the prose changes; recorded in `consumers:` as a follow-up rather than left implicit |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`). Reviewer records verdict + date in
the stream README table.

Core-system reviewer questions, answered in the verdict:
1. What single control stands between a write and the wrong identity performing it? (The
   resolver's custody binding.) Is it acceptable alone? (No — the backends' unminted-token
   refusal and the commit-identity preflight are the two independent layers behind it.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER bypassed? (Row 11:
   with the custody binding yielding nothing, the backend itself refuses rather than falling
   through to the decoy ambient credential.)
