---
brief: assay:assay:forge-neutral:18
title: statusgen off gh — one desk-tools read verb on the seam, offline lint by default, one git walk
why: >-
  statusgen is the last tool in the suite that reaches a forge on its own. It is a separate Go
  module that does not import `deskkit`, and it carries a dozen of its own `gh` shell-outs, so
  every guarantee the seam gives the desk verbs — the enumerated operation set, the refusal
  instead of a fallback, the per-forge backend — stops at the module boundary. The cost is
  measured, not asserted: on this repository a `--lint` spends 16.7 s of its 23.7 s inside 11
  `gh` subprocesses, 13.9 s of that fetching every issue ever opened in ten configured repos in
  order to print ONE advisory line, and the same run makes 254 git subprocesses to read a tree
  of 165 briefs. Taking statusgen off `gh` and taking `--lint` offline are the same change: the
  reads that belong to a forge go through the desk-tools read verb, and the check that runs in
  CI stops reaching the network at all.
wave: 5
depends: ["forge-neutral/08"]
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-14 by forge-neutral authoring session
sources:
  - "docs/streams/forge-neutral/README.md — the stream's own measured matrix, whose `statusgen` table already enumerates the surfaces this brief closes, and the permit-register ratchet that is the stream's progress metric"
  - "docs/streams/forge-neutral/brief-07-statusgen-acting-identity.md (forge-neutral/07, implemented) — the acting-identity vocabulary this brief preserves unchanged; its roster parity is what lets statusgen name a forge identity at all"
  - "docs/streams/forge-neutral/brief-08-statusgen-forge-aware.md (forge-neutral/08, implemented) — the hard dependency: it made statusgen's auto-flip corroboration and claim decay forge-AWARE, routing them through statusgen's own forge-aware read path. This brief replaces that path's transport with the desk-tools read verb; without 08 there is no forge-aware read path to re-home"
  - "docs/streams/forge-neutral/brief-06-read-verbs-on-the-seam.md (forge-neutral/06, done) — soft: establishes the read-verbs-on-the-seam shape, already landed"
  - "docs/streams/forge-neutral/brief-12-deskboard-non-board-reads-onto-the-seam.md (forge-neutral/12, done) — soft: put deskboard's peripheral reads on typed ops, which is the precedent for a read verb that adds NO new operation"
  - "tools/desk/internal/deskkit/forge.go — the frozen `Forge` interface; `ListOpenIssues`, `SearchIssues`, `ReviewsAtHead`, `ChecksAtHead`, `ListComments`, `GetPullRequest` and `SearchOpenChanges` already exist and already have both backends, which is why this brief adds no operation"
  - "tools/desk/internal/deskkit/forgeresolve.go:412,425 — `ForgeFor` / `ResolveForge`, the resolver the read verb calls"
  - "tools/desk/internal/forgeban/allowlist.go:72 — allowedInvocationCeiling = 7 at the freshness base"
  - "freshness-checked 2026-09-14 @ e428134c (origin/main) — 26 `exec.Command(\"gh\", …)` sites across 17 statusgen files; `statusgen/go.mod` declares its own module and no statusgen source imports `deskkit`; `issues.go:764` is `openIssueDebtNotice`; `attribution.go:437-438` is the per-brief author pair; `brieffile.go:388` is `parseBriefFile` with no cache; `main.go:662,772,978` each call `LoadHistory` on the same path; `linkcheck.go:651` is `buildSourceIndex`; `run()`'s `changed []string` is threaded at `main.go:32,84,111,114,334,422,474`"
exec-tier: strong
exec-tier-why: >-
  Two of the four halves can fail silently in the direction that reads as a pass. A lint made
  offline can go green because it stopped looking rather than because it looked and found
  nothing, and a scope-narrowed lint can go green because the defect was outside the scope — both
  are the could-not-check-as-pass shape this stream exists to remove, and both survive a
  happy-path test suite. The perf half has the same hazard in a different place: a memoised
  parse or a single git walk that answers a STALE value is indistinguishable from a fast correct
  one until a brief changes mid-run.
domain: complicated
consumers:
  - "tools/desk/cmd/deskread: fixed-here (the new read verb)"
  - "statusgen/forgeread.go: fixed-here (planned file — the `forgeReader` interface and its two implementations)"
  - "statusgen/issues.go: fixed-here (`openIssueDebtNotice` off `gh` and off by default in `--lint`)"
  - "statusgen/attribution.go, statusgen/gitinfo.go: fixed-here (the one-walk authorship index)"
  - "statusgen/brieffile.go: fixed-here (the parse memo)"
  - "statusgen/main.go: fixed-here (the single history read, the `--forge` and `--changed-only` flags)"
  - "statusgen/linkcheck.go: fixed-here (the source index scoped to docs/)"
  - "statusgen/autoflip.go, statusgen/autonomy.go, statusgen/briefdecision.go, statusgen/briefflowreview.go, statusgen/citationcorroborate.go, statusgen/claimdecay.go: follow-up forge-neutral/18-implementation (the remaining call sites, enumerated in the DoD; slice 1 lands the interface and the first site, and the row stays in-progress until they are all on it)"
  - "docs/telemetry.md: fixed-here (the contract for what `--lint` may reach)"
  - "tools/desk/internal/deskkit/forge.go: out-of-scope (this brief adds NO operation — every read it needs is already enumerated and already has both backends, so the freeze rule is satisfied by consuming the surface rather than widening it)"
  - "tools/desk/internal/forgeban/allowlist.go: out-of-scope (statusgen is a separate module and has never had a permit row; the register counts desk-tools call sites, and this brief adds none)"
version: 1
id: 6ccc64a7-c32b-48ba-b6ac-d3a8165f15f9
---

# Brief 18 — statusgen off `gh`: one desk-tools read verb on the seam

## Context

The driver's direction of 2026-09-14 is one sentence, and it decides the shape: **statusgen
must be forge-independent — it must stop calling `gh` itself; desk-tools owns the forge seam,
so add to desk-tools exactly the read that statusgen needs, and make it performant.**

Every clause is load-bearing. *Forge-independent* rules out teaching statusgen a second
backend. *desk-tools owns the seam* rules out statusgen importing `deskkit` — the module
boundary stays, and statusgen reaches the seam the way any other consumer would, by running a
verb. *Exactly the read statusgen needs* rules out a general-purpose forge client. *Performant*
is not a separate ask bolted on: the same `--lint` that is the CI gate is also the slowest
thing in the loop, and the largest single cause is the forge reads this brief removes.

files:
- `tools/desk/cmd/deskread/` (planned) — the new read verb.
- `statusgen/forgeread.go` (planned) — the `forgeReader` interface and its two implementations.
- `statusgen/issues.go` — `openIssueDebtNotice` at `:764` and its lister at `:761`.
- `statusgen/attribution.go` — the per-brief author pair at `:437-438`.
- `statusgen/gitinfo.go` — `gitPathFirstAuthorIdentity` (`:189`) and
  `gitPathLastAuthorIdentity` (`:170`), the two per-brief git reads the walk replaces.
- `statusgen/brieffile.go` — `parseBriefFile` at `:388`.
- `statusgen/main.go` — `run()`'s existing `changed []string` plumbing (`:32,84,111,114,334,422,474`),
  the three `LoadHistory` calls on one path (`:662,772,978`).
- `statusgen/linkcheck.go` — `buildSourceIndex` at `:651`.
- `docs/telemetry.md` — the style this brief's `--lint` reach contract follows.

**Why the risk answers are all `no`.** This brief changes no credential, mints nothing, and
takes no trust decision. The reads it re-homes are reads; the identity that performs them is
resolved by the verb through `forge-neutral/07`'s roster parity and `forge-neutral/01`'s
resolver, unchanged. What it does change is the REACH of a check — and the two places that
could go wrong are the offline default and the scope-narrowed mode. Neither removes a control:
the issue-debt line is a NOTICE that is already `gh`-guarded and already degrades to the empty
string on any failure (`issues.go:769-780`), so making it opt-in retires an advisory, not an
assertion; and `--changed-only` is specified as REFUSING outright in the CI gate rather than
narrowing quietly. Both are guarded by mandatory negative-path rows (5, 6, 11).

single-point-of-failure: for the offline default, the one control between "this lint is green
because it looked" and "this lint is green because it stopped looking" is the three-state
report — every forge-backed check must render could-not-check as itself when the verb was not
invoked. Two independent layers stand behind it. First, the offline stub implementation is the
DEFAULT wiring, so a forge-backed check that forgets to handle could-not-check fails at compile
time against a reader whose every method returns one, rather than at runtime against a live
forge that happens to answer. Second, the CI gate asserts the process made zero network calls
(row 4), which trips on a different signal — an observed connection attempt — in a different
place (the test harness's network layer) from the report the checks render. For
`--changed-only` the single control is the CI-gate refusal, and its second layer is that the
mode prints a scope banner naming exactly which paths it examined, so a narrowed run that
somehow reached a gate is visibly narrowed in the log rather than reading as a full pass.

facts — all measured on this repository at `e428134c`, 24 streams and 165 brief files:

- **`--lint` wall time is dominated by `gh`.** With `gh` on `PATH`: 23.66 s real. With `gh`
  absent from `PATH` and nothing else changed: **6.05 s** real, same verdict (`LINT: PASS`),
  zero PROBLEM lines. The forge reads are 74 % of the wall clock of the check that gates every
  change.
- **11 `gh` subprocesses, 16.70 s of subprocess time.** Ten of them are one
  `gh issue list --state all --limit 1000` per repo in the operator's configured scan set,
  run SERIALLY, totalling 13.86 s; the eleventh is `gh pr list --state all --limit 1000` for
  dead-claim decay, 2.85 s. The slowest single call took 6.13 s.
- **Those ten calls exist to print one advisory line.** `openIssueDebtNotice` (`issues.go:764`)
  renders `issue debt: N open, K over <days>d, oldest #<n> at <age>` and nothing else. Its
  inputs are, provably, only OPEN issues and their numbers and creation times: the stale
  computation iterates `openIssues` alone (`issues.go:391-406`), and `Stale.Open` is set from
  `rep.Open` (`:408`). The `--state all` read therefore fetches every closed issue in ten repos
  and discards all of it.
- **The seam already serves that read, with no new operation.**
  `Forge.ListOpenIssues(repo) ([]IssueSummary, error)` returns exactly `Number`, `Title`,
  `Author`, `Labels`, `CreatedAt`, `URL` (`forge.go:593-606`) — a superset of what the notice
  consumes and a strict subset of what `gh issue list --state all` transfers. Both backends
  implement it and both are golden-pinned. This is the single strongest argument for the
  verb's shape: the first read statusgen needs was already enumerated, so the freeze rule
  (spec §6, an added op needs a consuming call site in the same change) is satisfied by adding
  NO op at all.
- **254 git subprocesses per `--lint`**, for a tree of 165 briefs: 144 `log`, 62
  `blame`, 30 `show`, 9 `merge-base`, 3 `rev-parse`, 2 `ls-tree`, and one each of `status`,
  `remote`, `ls-remote`, `diff` and `config`. Subprocess overhead, not work, dominates the
  offline 6.05 s: 3.73 s user + 2.74 s sys.
- **One whole-tree walk answers the 144-plus-62 majority of them.**
  `attribution.go:437-438` calls `gitPathFirstAuthorIdentity` then `gitPathLastAuthorIdentity`
  per brief, each a `git log` over one path. A single
  `git log --format='%H %ae' --name-only -- docs/streams` runs in **0.03 s** and emits 2,260
  lines carrying the first and last author of every path under `docs/streams` in one read.
- **`parseBriefFile` is called 3,351 times for 172 distinct paths** in one `--lint`, up to
  **23 times for a single file** — measured by instrumenting `brieffile.go:388` to log its
  argument. There are 40 call sites (`grep -c 'parseBriefFile(' statusgen/*.go`, excluding
  tests: 30 non-test sites), most of them independent whole-tree walks, and none share a cache.
- **`LoadHistory` reads the same `.history.jsonl` three times from `main.go` alone**
  (`:662`, `:772`, `:978`), each re-reading and re-decoding the whole append-only log, plus
  further reads from the report paths that run under `--lint`.
- **`buildSourceIndex` walks the repository ROOT** (`linkcheck.go:651`), not `docs/`, so every
  vendored, generated and binary path in the tree is stat-ed to resolve links that can only
  point inside `docs/`.
- **The `--changed-only` plumbing already exists.** `run()` takes `changed []string`
  (`main.go:32`) and eight checks already consume it — scope derivation (`:84`), the stream cap
  and source lints (`:111,:114`), register integrity (`:334`), the sync check (`:422`) and the
  verify-script notices (`:474`). What does not exist is a way to ASK for that behaviour from
  the command line for a lint, or any refusal preventing it becoming the CI gate.
- **statusgen is a separate module with no `deskkit` import.** `statusgen/go.mod` declares
  `github.com/medici-finance/assay/statusgen` with two dependencies; no statusgen source file
  imports `deskkit`. That boundary is deliberate and this brief keeps it: the read verb is
  reached by running a process, not by linking a package.
- **The remaining `gh` sites, enumerated.** 26 `exec.Command("gh", …)` sites across 17 files.
  The ones that run under `--lint` or the lifecycle flips are `autoflip.go:535,615,656,666`
  (review corroboration and head resolution), `autonomy.go:451,479` (merged-change lists),
  `briefdecision.go:41` (a decision-issue list), `briefflowreview.go:72,103` (a change's
  reviews), `citationcorroborate.go:437,463` (comment lists), `claimdecay.go:43` (the open-change
  list) and `issues.go:577,761` (the issue lists). The rest belong to report modes outside the
  gate and are named in the DoD as the remaining work.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Add no operation to `Forge`.** Every read named here is already enumerated with both
  backends. If a later call site genuinely needs one that is not, that is a separate brief
  under the freeze rule, not a widening slipped in behind a read verb.
- **Offline must never mean green.** A forge-backed check that did not reach the forge reports
  could-not-check as itself. Rounding it up to a pass is the exact failure this stream was
  opened to remove, and it is the one way this brief could make things worse rather than
  faster.
- **A cache must not outlive its subject.** The parse memo and the authorship walk are keyed so
  a file that changes mid-run is re-read. Speed bought with a stale answer is not speed.
- Do not weaken a lint to make it faster. Every check that runs today still runs; what changes
  is how it gets its inputs and what it reports when it cannot.

## Task

### 1. `deskread` — a new verb, not a `deskboard` subcommand

Add `tools/desk/cmd/deskread/` (planned), a read-only verb on the seam. It resolves its `Forge` through
`deskkit.ForgeFor(repo, role)`, performs one enumerated read, and writes JSON to stdout.

**Why a new verb rather than `deskboard read`** — state this in the PR body, the decision is
part of the deliverable:
1. `deskboard`'s eleven subcommands all COMPOSE a desk view (`prs`, `queue`, `health`,
   `next-up`, `throughput`, `stalled`, …) under the roster's scope and staleness semantics. A
   raw read is the only one whose output would not be a board, and statusgen would have to opt
   out of every board-shaped default to use it.
2. The permit register is keyed per call site, and `deskboard`'s row was deliberately NARROWED
   by `forge-neutral/06` with its exit condition already spent by `forge-neutral/12`. Adding a
   new consumer inside `deskboard` re-widens a row this stream just closed.
3. statusgen pins the JSON contract it parses. Coupled to `deskboard`, every board-schema
   change becomes a statusgen compatibility question across a module boundary and a pinned
   release.
4. It is separately installable. An adopter who runs statusgen but not the full desk gets one
   small binary, not the board tool.

**Shape.**
```
deskread <kind> --repo <owner/repo> [--repo <owner/repo> …] [--role <role>] [--json]
```
- `<kind>` is one of a CLOSED set, one kind per enumerated read. Slice 1 lands `issues`
  (`ListOpenIssues`). The remaining kinds — `merged-changes`, `change`, `reviews`, `checks`,
  `comments` — are enumerated in the DoD and land on the same shape.
- **`--repo` is repeatable and the verb makes ONE invocation serve a repo SET.** This is the
  performance half of the verb, and the reason it is not "one call per repo in a loop": the
  measured 13.86 s is ten serial process starts, ten token resolutions and ten sequential
  round trips. One invocation resolves once and performs the reads concurrently with a bounded
  worker count.
- Output is a versioned envelope, `{"schema": 1, "kind": …, "repos": [ … ], "partial": [ … ]}`.
  `schema` is checked by the consumer and an unrecognised value is a REFUSAL, never a
  best-effort parse (the same fail-closed direction `parseBriefFile` already takes on an
  unknown brief schema, `brieffile.go` note at `:388`).
- **Partial is a first-class result, not an error.** A repo the verb could not read appears in
  `partial` with its reason and does NOT appear in `repos`. The exit code is 0 for a partial
  read — a caller that treats "some repos unreadable" as "no issues anywhere" is the failure
  mode this shape exists to prevent — and non-zero only when NO repo could be read.
- Read-only by construction: the verb consumes no write operation and holds no write path.

### 2. `forgeReader` — statusgen's own interface, two implementations

Add `statusgen/forgeread.go` (planned):

```go
type forgeReader interface {
        OpenIssues(repos []string) (map[string][]forgeIssue, []forgeUnavailable, error)
        // … one method per read kind, added as each call site is migrated
}
```

The method set is derived from the call sites, not invented: open/closed issue lists per repo
with labels, state and timestamps; merged-change lists with merge time, author and body
trailers; a change's head, reviews and check rollup by number; and comment lists for
corroboration. Slice 1 lands `OpenIssues`; the rest are added one per migrated site so no
method exists without a consumer, mirroring the seam's own freeze rule.

Two implementations, and **the offline one is the default**:
- `offlineReader` returns a could-not-check for every method, naming the reason (`--forge` not
  given). It performs no process start and no network call.
- `deskreadReader` runs `deskread`, parses the envelope, and maps `partial` entries onto
  per-repo could-not-check values.

A caller that has not handled could-not-check does not compile against the offline reader's
signature, which is why the three-state result is a return VALUE and not a logged warning.

### 3. `--lint` is offline by default

- The issue-debt NOTICE becomes opt-in: `--lint` alone never emits it and never starts a
  process; `--lint --forge` wires the `deskreadReader` and emits it. The flag's help text says
  what it costs.
- Every other forge-backed lint check reads could-not-check offline and renders it AS ITSELF.
  None of them may read green because no read happened.
- `--lint` with no `--forge` makes **zero network calls** and starts **no forge process**.
  That is row 4's assertion and it is the property the whole half stands on.

### 4. The performance half

- **One authorship walk.** Replace the per-brief `gitPathFirstAuthorIdentity` /
  `gitPathLastAuthorIdentity` pair with one `git log --format='%H %ae' --name-only --
  docs/streams` per root, parsed into a path → {first, last} index built once. The existing
  `hasNoGitDir` silent-skip and the LOUD per-brief degradation when a specific brief's history
  is unreadable (`attribution.go:440-447`) both survive: a path missing from the index is the
  same unreadable case it is today, reported the same way.
- **Memoise `parseBriefFile`** on (path, mtime, size). A file whose stamp changes is re-read.
  The memo is per-run, not persisted. Instrument a counter so row 8 can assert distinct-path
  parses rather than total calls.
- **Read `.history.jsonl` once** per root and hand the decoded entries to the call sites that
  re-read it today.
- **Scope the source index to `docs/`** rather than the repository root.
- **Batch `git show <base>:<path>`** through one `git cat-file --batch` per root, which is the
  same content by the same object ids in one process instead of 30.

### 5. `--lint --changed-only <paths>` — gated, banner'd, and refused in the CI gate

A scoped lint for a local pre-push check, built on `run()`'s existing `changed` plumbing.

- It prints a LOUD scope banner naming the paths examined and stating that a full-tree defect
  outside them was NOT looked for.
- **It REFUSES (non-zero) when invoked as the CI gate** — detected from the CI environment the
  gate runs under — with a message saying the gate requires a full lint. There is no flag that
  overrides this. A scoped lint that can be the gate is a gate that stops checking the moment
  someone finds it convenient.

### 6. The reach contract

Add to `docs/telemetry.md` (or a sibling in the same style) a short contract stating what
`--lint` may reach: no network without `--forge`, no process start without `--forge`, and the
enumerated set of reads `--forge` performs. A reader auditing "what does the gate touch"
finds it in one place rather than by grepping for `exec.Command`.

## Verify (executable — no prose-only DoD items)

Test names are planned, created by the implementer. This brief follows the `Class`-column
convention: `+dereference` marks a row that resolves a claim rather than counting its
presence, `+flow` a row that exercises the cross-component path end to end.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd statusgen && go build ./... && go test ./... -count=1` | exit 0 |
| 2 | check:ci | `cd tools/desk && go build ./... && go test ./... -count=1` | exit 0 |
| 3 | check +dereference | `grep -rn 'exec.Command("gh"' statusgen/ --include='*.go' \| grep -v _test.go \| wc -l` | prints `0` — statusgen shells no forge CLI. Measured at the freshness base: 26 sites across 17 files |
| 4 | check:ci +flow | `cd statusgen && go test ./... -run TestLintOfflineMakesNoNetworkCall -count=1 -v` | **negative path**: a full `--lint` with no `--forge`, run against a harness whose network dial hook FAILS the test on any attempt and whose `PATH` contains no `gh` and no `deskread`, completes with the same verdict as a networked run. Fails if any connection is attempted or any forge process is started |
| 5 | check:ci | `cd statusgen && go test ./... -run TestForgeBackedChecksReportCouldNotCheckOffline -count=1 -v` | **negative path**: with the offline reader wired, every forge-backed check renders could-not-check AS ITSELF. The test fails if any of them renders clean, and it enumerates the checks so a newly-added one that forgets is caught rather than skipped |
| 6 | check:ci | `cd statusgen && go test ./... -run TestIssueDebtNoticeOptInOnly -count=1 -v` | **negative path**: `--lint` alone emits no issue-debt line and starts no process; `--lint --forge` emits it from the verb's JSON. The test fails if the notice appears without `--forge` |
| 7 | check:ci +flow | `cd tools/desk && go test ./cmd/deskread/... -run TestDeskreadIssuesRoundTrip -count=1 -v` | exit 0 — `deskread issues` over a repo SET returns the versioned envelope on BOTH the GitHub and the GitLab fixture backend, via `ForgeFor`, with one invocation per repo SET and not one per repo |
| 8 | check:ci | `cd tools/desk && go test ./cmd/deskread/... -run TestDeskreadPartialIsNotAnError -count=1 -v` | **negative path**: one unreadable repo in a set of three lands in `partial` with its reason, the other two in `repos`, and the exit code is 0; ALL repos unreadable exits non-zero. Fails if a partial read is rendered as an empty success |
| 9 | check:ci | `cd statusgen && go test ./... -run TestParseBriefFileMemoDistinctPaths -count=1 -v` | exit 0 — over a fixture tree the memo instrument reports distinct-path parses equal to the file count, and a file whose mtime/size changes mid-run is re-parsed. Measured before: 3,351 calls over 172 distinct paths, max 23 per file |
| 10 | check:ci +dereference | `cd statusgen && go test ./... -run TestAttributionOneWalkMatchesPerPathReads -count=1 -v` | exit 0 — for every brief in a fixture tree the walk-derived first/last author equals what the per-path `git log` pair returns, INCLUDING the unreadable-history case, which must still degrade loudly per brief |
| 11 | check:ci | `cd statusgen && go test ./... -run TestChangedOnlyRefusesInCIGate -count=1 -v` | **negative path**: `--lint --changed-only <paths>` under the CI-gate environment REFUSES non-zero and names the requirement; outside it, it runs and prints the scope banner. The test fails if any flag combination lets a scoped lint serve as the gate |
| 12 | check +dereference | git-subprocess count per `--lint`, counted with a `PATH` shim that logs every `git` argv | ≤ 100 after. Measured before on this repository: **254** (144 log, 62 blame, 30 show, 9 merge-base, 3 rev-parse, 2 ls-tree, 5 others) |
| 13 | check +dereference | wall time of `--lint` on a tree of 400 briefs or more, offline, best of three | at least 60 % faster than the same tree's pre-change offline time. Measured before on this 165-brief repository: 6.05 s offline, 23.66 s with `gh` on `PATH` |
| 14 | check:ci | `cd statusgen && go test ./... -run TestActingIdentityUnchanged -count=1 -v` | exit 0 — `forge-neutral/07`'s Evidence-actor and witness behaviour is byte-identical before and after. The reads moved; who is recorded as having acted did not |
| 15 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeSurfaceUnchangedByDeskread -count=1 -v` | exit 0 — the `Forge` interface gains no method; `.github/workflows/forge-surface-control.yml`'s three controls stay green and `allowedInvocationCeiling` is unchanged at 7 |
| 16 | check | `statusgen --root . --lint` | `LINT: PASS`, exit 0 |
| 17 | check:ci +dereference | `statusgen --root . --consumers --brief forge-neutral/18` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff. Note: a repo-wide brief-id format mismatch is open against `--consumers` (#954) and has produced a could-not-check on sibling briefs' equivalent row; this row is satisfied by exit 0 OR by that same could-not-check citing #954, not by a silent skip |

## Pre-mortem → detection map

*"This shipped and was wrong — what went wrong?"*

| Failure mode of the work | Caught by |
|---|---|
| `--lint` goes offline and the forge-backed checks quietly read GREEN instead of could-not-check — a faster lint that checks less and says so nowhere | row 5, which enumerates the checks rather than sampling one, plus row 4's zero-network assertion |
| The offline default is implemented as "try the forge, fall back on error", so a lint on a machine with credentials silently keeps reaching the network and the measured win never appears in CI | row 4's harness fails the test on any dial attempt or forge process start — it does not measure, it forbids |
| `deskread` is given a `--query` or endpoint-shaped argument "just for statusgen", reopening the passthrough the surface control forbids | row 15 + the closed `<kind>` set; the verb takes a kind, never an address |
| A partial read (one repo unreachable) is rendered as an empty success, so the issue-debt line reports zero debt because it could not look | row 8 |
| `deskread` is called once per repo in a loop, so the verb lands but the 13.86 s stays | row 7 asserts one invocation per repo SET |
| The parse memo returns a stale brief after a file changes mid-run, so a lint passes against content that is no longer on disk | row 9's mtime/size change half |
| The one authorship walk silently loses the per-brief LOUD degradation, turning an unreadable history into an implicit pass | row 10 explicitly includes the unreadable-history case |
| `--changed-only` becomes the CI gate — deliberately, to make CI fast — and defects outside the changed set stop being caught at all | row 11, and the refusal has no override flag |
| `--changed-only` is merely NOISY rather than refusing in CI, so the banner scrolls past in a green log | row 11 asserts a non-zero exit, not a warning |
| The perf work changes a check's RESULT, not just its speed — a batched `cat-file` reads a different object than the per-path `git show` did | row 10's equality assertion; row 16's end-to-end verdict on this repository |
| `forge-neutral/07`'s acting identity is disturbed because the reads that name an identity moved | row 14 |
| A new `Forge` operation is added because one call site was awkward, widening a frozen surface behind a read verb | row 15 + `allowedInvocationCeiling` unchanged |
| The remaining call sites are left on `gh` but the row is flipped to implemented anyway | row 3's `0` is the completion test for the whole brief; slice 1 leaves it non-zero and the row stays `in-progress` by construction |
| The measured win is claimed from a warm-cache run against a cold-cache baseline | row 13 specifies best-of-three on one tree, both sides offline |
| An adopter on a box with no desk-tools installed finds `--lint` broken rather than merely offline | **no row** — it is the offline DEFAULT that makes this safe: with no `--forge` the verb is never invoked, so a missing `deskread` is unreachable from the gate. A row asserting a missing binary's behaviour under `--forge` belongs with `forge-neutral/11`'s install work, where a box with no desk-tools actually exists |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **model** (from frontmatter; all four risk answers are `no` — see the note in
`## Context`). Reviewer records verdict + date in the stream README table, and confirms two
things beyond the rows: that the offline default cannot render a forge-backed check clean, and
that `--changed-only` has no path — flag, environment variable or argument order — by which it
can serve as the CI gate.
