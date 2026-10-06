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
depends: ["forge-neutral/08", "forge-neutral/33"]
unblocks: ["forge-neutral/35"]
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
  - "freshness-checked 2026-09-14 @ e428134c (origin/main) — 26 `exec.Command(\"gh\", …)` sites across 15 non-test statusgen files; `statusgen/go.mod` declares its own module and no statusgen source imports `deskkit`; `issues.go:764` is `openIssueDebtNotice`; `attribution.go:437-438` is the per-brief author pair; `brieffile.go:388` is `parseBriefFile` with no cache; `main.go:662,772,978` each call `LoadHistory` on the same path; `linkcheck.go:651` is `buildSourceIndex`; `run()`'s `changed []string` is threaded at `main.go:32,84,111,114,334,422,474`"
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
  - "statusgen/autoflip.go, statusgen/autonomy.go, statusgen/briefdecision.go, statusgen/briefflowreview.go, statusgen/citationcorroborate.go, statusgen/claimdecay.go: fixed-here (the remaining call sites, enumerated in Task 6. They are THIS brief's own deliverable, not a follow-on: Verify row 3 — zero forge-CLI sites in `statusgen/` — is this brief's completion test, so the board row stays `in-progress` until every one of them is on the reader. There is no follow-on brief and deliberately no forward reference to one)"
  - "statusgen/decisionruling.go, statusgen/decisionruling_test.go, statusgen/decisionruling-mutations.json, statusgen/ghfetch.go: follow-up forge-neutral/35 (the ruling resolver, and the `gh auth token` fallback at `decisionruling.go:643`, move in their own human-gated brief because the move rewrites the inputs of the ruling-authenticity control; this brief does not touch them, and its row 3 excludes `statusgen/decisionruling.go` by path)"
  - "statusgen/transcribescan.go `ghCommentResolver`: follow-up forge-neutral/35 (the transcribe lanes' sign-off resolver at `:104-129`, with its launch at `:109` at `e6cb7d2a0`, moves in the same human-gated brief as the ruling resolver because both enactment gates read their sign-off check from it; this brief does not touch that function, and its row 19 pins the one line row 3 still counts to it. `ghAuthorResolver` at `:76` in the same file is this brief's)"
  - "statusgen/transcribeverdict.go `ghVerdictIssueResolver`: follow-up forge-neutral/35 (the verdict-issue read at `:505-531`, with its launch at `:506` at `e6cb7d2a0`, feeds the verdict and scan-delta lanes' author pins and edited-issue refusals; its old source never set the edit flag, so its move turns those refusals on, and it moves in the same human-gated brief. This brief does not touch that function, and its row 20 pins the one line row 3 still counts to it. `ghVerdictMainHealth` at `:579` in the same file is this brief's)"
  - "tools/desk/cmd/deskread/main.go `ciTransportKinds`: follow-up forge-neutral/18 (this brief; flips to fixed-here when the implementation edits the path — it adds `issue`, and every other kind its CI-lane sites need, one reviewed entry per kind)"
  - "docs/telemetry.md: fixed-here (the contract for what `--lint` may reach)"
  - "tools/desk/internal/deskkit/forge.go: out-of-scope (this brief adds NO operation — every read it needs is already enumerated, or added by forge-neutral/33, and has both backends, so the freeze rule is satisfied by consuming the surface rather than widening it)"
  - "tools/desk/internal/forgeban/allowlist.go: out-of-scope (statusgen is a separate module and has never had a permit row; the register counts desk-tools call sites, and this brief adds none)"
version: 2
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
- `statusgen/issues.go` — `ghIssueMetricLister` declared at `:571` with its single
  `exec.Command("gh", …)` at `:577`, and `openIssueDebtNotice` at `:764`, whose
  `exec.LookPath("gh")` guard is at `:768` and whose per-repo lister calls are at `:773`.
- `statusgen/attribution.go` — the per-brief author pair at `:437-438`.
- `statusgen/gitinfo.go` — `gitPathFirstAuthorIdentity` (`:189`) and
  `gitPathLastAuthorIdentity` (`:170`), the two per-brief git reads the walk replaces.
- `statusgen/brieffile.go` — `parseBriefFile` at `:388`.
- `statusgen/main.go` — `run()`'s existing `changed []string` plumbing (`:32,84,111,114,334,422,474`),
  the three `LoadHistory` calls on one path (`:662,772,978`).
- `statusgen/linkcheck.go` — `buildSourceIndex` at `:651`.
- `tools/desk/cmd/deskread/main.go` — `ciTransportKinds`, the CI-transport kind set the
  CI-transport brief in #2314 opens at `issues`, `trust` and `comments`.
- `docs/telemetry.md` — the style this brief's `--lint` reach contract follows.

**Why the risk answers are all `no`.** This brief adds no credential, mints nothing, moves
no human-ruling resolver, and changes no control's accept or refuse rule. Several sites it
moves do feed a control. Each keeps the same signal from a new source, and the table in Task 6
names the old and the new source for each. Every site it moves is a read: the
same data, fetched through `deskread` instead of `gh`, under the identity the verb resolves —
`forge-neutral/07`'s roster parity and `forge-neutral/01`'s resolver locally, and in CI the
opt-in workflow-token transport that #2253's ruling chose (the CI-transport brief in #2314).
For the CI-lane reads that identity is new: today they run under the job's own `gh` login, and
afterwards under the transport, which is why this brief depends on that brief and adds `issue`,
plus each further kind its CI-lane sites need, to `ciTransportKinds` by reviewed diff. The
acting identity statusgen RECORDS is unchanged (row 14). Two resolvers and one issue read are
deliberately not here, and forge-neutral/35, human-gated, moves all three. The first is the ruling resolver in
`decisionruling.go`, with its `gh auth token` fallback at `:643`. Moving it rewrites the inputs
of the ruling-authenticity control (deleted-comment detection, the comment-to-issue binding,
edit detection, the bot check) and the credential it reads under. The second is
`ghCommentResolver` in `transcribescan.go`, the sign-off check both transcribe lanes'
enactment gates read. Moving it changes how the comment is found (a thread read selected by
id instead of a single read by id) and where the author type comes from. The third is
`ghVerdictIssueResolver` in `transcribeverdict.go`, the issue read behind the verdict and
scan-delta lanes' author pins and edited-issue refusals. Its old source never set the edit flag,
so moving it turns those refusals on, which changes an accept-or-refuse outcome. Row 3 excludes
`decisionruling.go` by path and counts the two lines left in `ghCommentResolver` and
`ghVerdictIssueResolver`; rows 19 and 20 pin each line to its function. One read this brief does move sits next to a human ratification:
the decision-gate anchor's issue read (`decisiongateanchor.go:229`), whose closer the anchor
matches against the blessing authority's login (`:157`). The anchor's rule is unchanged, and
the closer login comes from `Issue.ClosedBy` instead of REST `closed_by`, so its signal does
not change. What this brief
does change is the REACH of a check, and the two places that could go wrong are the offline
default and the scope-narrowed mode. Neither removes a control: the issue-debt line is a NOTICE
that is already `gh`-guarded and already degrades to the empty string on any failure
(`issues.go:769-780`), so making it opt-in retires an advisory, not an assertion; and
`--changed-only` is specified as REFUSING outright in the CI gate rather than narrowing quietly.
Both are guarded by mandatory negative-path rows (5, 6, 11).

single-point-of-failure: for the offline default, the one control between "this lint is green
because it looked" and "this lint is green because it stopped looking" is the three-state
report — every forge-backed check must render could-not-check as itself when the verb was not
invoked. Two independent layers stand behind it. First, the offline stub implementation is the
DEFAULT wiring, so a forge-backed check that forgets to handle could-not-check meets one on
every test run, against a reader whose every method returns one, rather than at runtime against
a live forge that happens to answer. Go does not make a caller handle a returned value, so this
layer holds through the checks' own tests, not through the compiler. Second, the CI gate asserts the process made zero network calls
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
- **The remaining `gh` sites, enumerated.** 26 `exec.Command("gh", …)` sites across 15 non-test files.
  The ones that run under `--lint` or the lifecycle flips are `autoflip.go:535,615,656,666`
  (review corroboration and head resolution), `autonomy.go:451,479` (merged-change lists),
  `briefdecision.go:41` (a decision-issue list), `briefflowreview.go:72,103` (a change's
  reviews), `citationcorroborate.go:437,463` (comment lists), `claimdecay.go:43` (the open-change
  list) and `issues.go:577` (the one issue-list site, in `ghIssueMetricLister`, reached under
  `--lint` from `openIssueDebtNotice` at `:764` via its calls at `:773`). The rest belong to
  report modes outside the
  gate and are named in the DoD as the remaining work.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Add no operation to `Forge`.** Every read named here is already enumerated, or added by
  forge-neutral/33, with both backends. If a later call site genuinely needs one that is not, that is a separate brief
  under the freeze rule, not a widening slipped in behind a read verb.
- **Offline must never mean green.** A forge-backed check that did not reach the forge reports
  could-not-check as itself. Rounding it up to a pass is the exact failure this stream was
  opened to remove, and it is the one way this brief could make things worse rather than
  faster.
- **A cache must not outlive its subject.** The parse memo and the authorship walk are keyed so
  a file that changes mid-run is re-read. Speed bought with a stale answer is not speed.
- Do not weaken a lint to make it faster. Every check that runs today still runs; what changes
  is how it gets its inputs and what it reports when it cannot.
- **Do not touch `statusgen/decisionruling.go`** or the ruling resolver's call at
  `corroborate.go:1481`, nor `ghCommentResolver` in `statusgen/transcribescan.go` or the three
  places `main.go` passes it to the transcribe modes (`:1980`, `:1987`, `:1995` at
  `e6cb7d2a0`), nor `ghVerdictIssueResolver` in `statusgen/transcribeverdict.go` or the two
  places `main.go` passes it (`:1987`, `:1994`). They are forge-neutral/35's.
  `ghAuthorResolver` and `ghVerdictMainHealth`, in the same two files, are this brief's.

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

The three-state result is a return VALUE and not a logged warning, so every test run against the
offline reader hands each caller a could-not-check. Go does not make a caller handle a returned
value, so a caller that drops it is caught by those tests, not by the compiler.

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

### 6. Every remaining forge-CLI call site, migrated — this brief's own completion test

Not a follow-on and not a forward reference: the sites below are this brief's deliverable, and
Verify rows 3, 19 and 20 together (no forge-CLI launch, `exec.Command` or `exec.CommandContext`,
in `statusgen/` outside `decisionruling.go`, `transcribescan.go`'s `ghCommentResolver` and
`transcribeverdict.go`'s `ghVerdictIssueResolver`) are what say the brief is finished.
Each moves onto a `forgeReader` method added WITH its consuming call site, never ahead of it.

| File | Sites at the freshness base | The read kind it needs |
|---|---|---|
| `statusgen/autoflip.go` | `:535`, `:615`, `:656`, `:666` | a change's head, its reviews at head, and commit→change resolution |
| `statusgen/autonomy.go` | `:451`, `:479` | merged-change lists with merge time, author and body trailers |
| `statusgen/briefdecision.go` | `:41` | a label-filtered issue list |
| `statusgen/briefflowreview.go` | `:72`, `:103` | a change plus its full reviews array |
| `statusgen/citationcorroborate.go` | `:437`, `:463` | comment lists |
| `statusgen/claimdecay.go` | `:43` | the all-state change list the decay pass reduces |
| `statusgen/issues.go` | `:577` | the issue list (slice 1 — done) |

The remaining sites belong to REPORT modes outside the `--lint` gate:
`corroborate.go:712,981,1009` · `decisiongateanchor.go:229` · `doratiming.go:631` ·
`scanissues.go:114,870` · `selfimprovement.go:400` · `transcribescan.go:72,104` ·
`transcribeverdict.go:500,573` · `trustgate.go:204`. `transcribescan.go:104` is
`ghCommentResolver` and `transcribeverdict.go:500` is `ghVerdictIssueResolver`, both of which
forge-neutral/35 moves; every other site here is in scope for
row 3's count and is migrated the same way; they are listed separately only because none of them can
affect the gate, so none of them gates the offline-lint half.

**The two groups account for the whole census, and the arithmetic is the check:** 7 files × 13
sites in the gate group plus 8 files × 13 sites in the report group is 15 files and 26 sites —
the same 26/15 the freshness line and Verify row 3 state. A reader who greps the tree and gets a
different total has found either a drifted brief or a new call site, and either is worth knowing.
(Re-measured 2026-10-06 at `11228951d`: 31 grep matches across 16 files plus the
`exec.CommandContext` launch at `autonomy.go:539`. The current per-site list is forge-neutral/33's
census.)

**The two human-ruling resolvers, and one issue read, are not here.** The ruling resolver's one census site,
`decisionruling.go:643`, is not a read: it is `gh auth token`, the last-resort credential for
the client the resolver builds for its own two reads. Moving those reads rewrites the inputs of
the ruling-authenticity control. The sign-off resolver, `ghCommentResolver`
(`transcribescan.go:109` at `e6cb7d2a0`), is what `transcribeEnactmentGate` and
`transcribeVerdictEnactmentGate` read their sign-off check from; moving it changes how the
comment is selected and where its author type comes from. The verdict-issue read, `ghVerdictIssueResolver`
(`transcribeverdict.go:506` at `e6cb7d2a0`), feeds the verdict and scan-delta lanes' author pins
and their edited-issue refusals (`transcribeverdict.go:760`, `transcribescan.go:886`). Its old
source never set the edit flag, so its move turns those refusals on. All three moves are
forge-neutral/35, a human-gated brief whose `depends:` names forge-neutral/18. Row 3 excludes
`statusgen/decisionruling.go` by path and still counts the `ghCommentResolver` and
`ghVerdictIssueResolver` lines. A path filter cannot exclude either, because the same two files
hold `ghAuthorResolver`'s launch (identical text) and `ghVerdictMainHealth`'s, both this
brief's, so rows 19 and 20 name the function each line is in. 35's own rows run row 3 with no
exclusion and rows 19 and 20's commands, and take all three to nothing.

**Reads that feed a control keep their signal.** These sites feed a control's decision. Each
moves to a read that carries the same signal; none moves the control. Lines are at
`e6cb7d2a0`; the census above is at the freshness base, so its numbers differ.

| Site at `e6cb7d2a0` | The control it feeds | Old source | New source (forge-neutral/33's census) |
|---|---|---|---|
| `transcribescan.go:77` (`ghAuthorResolver`) | the R-7 lane's issue-author trust check | REST issue `user.login`, `user.id`, `user.type` | `GetIssueTyped`, with the author type 33 adds; an empty type is could-not-check, never `User` |
| `decisiongateanchor.go:229` | the decision-gate anchor, a human ratification: the issue must be closed by the blessing authority's login (`:157`) and name the brief | REST issue `closed_by.login`, state and body | `GetIssueTyped` with `Issue.ClosedBy`; an absent closer is not the blessing authority, as today |
| `corroborate.go:1195` | review and comment corroboration under `--corroborate` | `gh pr view --json reviews,comments` | `ReviewsAtHead` and `ListCommentsTyped` |
| `claimdecay.go:63` | the claim-decay fork guard (`:126-170`) | `isCrossRepository`, `headRepository`, `headRepositoryOwner` from `gh pr list` | `ListChanges` with the new `ChangeRef.CrossRepo` and `HeadRepo`; a missing value stays unattributed, and `Incomplete` is could-not-check |
| `autoflip.go:1415` | auto-flip's body-edited-after-review check | GraphQL `pullRequest.lastEditedAt` | `PRTrustEvents.BodyEdited`, a time; zero means never edited, as `null` does today |
| `autoflip.go:1498` | auto-flip's approval-at-head check | REST reviews with `commit_id` | `ReviewsAtHead` |
| `trustgate.go:208` (`ghIssueBlessChecker`) | the bless-then-edit trust gate | the GraphQL trust query (author `__typename` and `databaseId`, `lastEditedAt`) | `IssueTrustEvents` through `deskread trust`, which `deskreadIssueBlessChecker` (`:332`) already uses |
| `scanissues.go:953` (`issueCommentLister`, wired by `--transcribe-scan` at `main.go:1980`) | the un-block check: only a non-bot answer counts (`isBotComment`, `:1012`), and only the id-pinned blessing authority's (`:1036`) | REST comment `user.login`, `user.id`, `user.type`, `created_at` and `body` | `ListCommentsTyped` through `deskread comments`, as `deskreadCommentLister` (`:989`) already reads on `--scan-issues`, with the `authorType` 33 adds from GraphQL `__typename`; an empty type is could-not-check, never `User` |
| `citationcorroborate.go:435` | citation corroboration: the cited person must have commented on or reviewed the cited artifact (`authoredBy`, `:277`); a 404 is MISSING, any other failure could-not-check | REST issue comments (author login, body, `html_url`), which cover issues and changes alike | `GetIssue` for existence and kind, then `ListCommentsTyped` on the target that kind names, for the full thread; not-found stays MISSING, and a failed read is could-not-check, never MISSING |
| `citationcorroborate.go:461` | the same check's review half: a review by the cited person | REST change reviews (author login, body, state); a 404 on a plain issue means no reviews | `ReviewsAtHead`, which returns every review on the change (`forge.go:1467`), read only when `GetIssue` names a change; a failed read is could-not-check, never "no review" |
| `briefflowreview.go:103` | the `--review-rework` and `--first-pass-yield` counts of `CHANGES_REQUESTED` rounds (`:163-170`) | REST change reviews, every submission rather than one row per author | `ReviewsAtHead`, every review in ascending order; a failed read stays an error, as today |

A row of this table that cannot keep its signal is a reason to stop and route that site to its
own brief, as was done for the two resolvers and for `ghVerdictIssueResolver`
(`transcribeverdict.go:506`). That read's old source never set the edit flag, so its row moved
out of this table to forge-neutral/35, which turns the edited-issue refusal on under a human
gate.

**The CI transport.** The `--corroborate`, `--auto-flip-model` and transcribe sites read under
the CI-transport brief's workflow-token transport (#2314) in CI, so this brief depends on it.
That brief opens `ciTransportKinds` at `issues`, `trust` and `comments`. This brief adds `issue`
(first consumed by `decisiongateanchor.go:229` under `--corroborate`, and later by
forge-neutral/35), and every other kind a CI-lane site needs, one reviewed entry per kind, in
the diff that moves the site. Row 18 asserts the `issue` entry.

**Not in this brief: a structural guard behind row 3.** Row 3 is a text count, and its Expect
lists the launch forms it cannot see. A guard that closes them is a follow-up, not part of this
brief's DoD: an AST test over non-test statusgen that refuses any `exec.Command*`,
`os.StartProcess` or `syscall.Exec` whose program argument is not on an allow-list, or row 4's
no-`gh`-on-`PATH` harness widened beyond `--lint` to every mode. The same follow-up re-points
the older unanchored grep in `docs/statusgen-lint-reach.md` (`:43`, `:72`) at row 3's command.

### 7. The reach contract

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
| 3 | check +dereference | `test -d statusgen/ && { grep -rnF '"gh"' statusgen/ --include='*.go' \|\| [ $? -eq 1 ]; } \| { grep -v -E '^[^:]+_test\.go:' \|\| [ $? -eq 1 ]; } \| { grep -v -E '^[^:]+:[0-9]+:[[:space:]]*//' \|\| [ $? -eq 1 ]; } \| { grep -v -E '^statusgen/decisionruling\.go:' \|\| [ $? -eq 1 ]; } \| wc -l` | output is `2`, and rows 19 and 20 show those two lines are in `ghCommentResolver` and `ghVerdictIssueResolver`. If forge-neutral/35 has already landed, the output is `0`. Together the three rows prove one thing: no line outside test files, `//` comment lines, `statusgen/decisionruling.go`, `ghCommentResolver` and `ghVerdictIssueResolver` carries the double-quoted literal `"gh"`. That covers `exec.Command`, `exec.CommandContext` with any context argument, a variable or constant in statusgen holding the name, a call split across lines, and an aliased import. It does NOT see a launch whose name is: a back-quoted raw string; an escaped literal (hex, octal or Unicode); a path ending in `gh`, absolute or relative; inside a shell command (`sh -c "gh …"`); derived at run time (case conversion, bytes, concatenation); or supplied from outside the file (another package, an imported module, linker flags, an embedded or config file, the environment). A `0` is therefore necessary, not sufficient: 18's reviewer confirms every site in forge-neutral/33's census except `decisionruling.go:643`, `transcribescan.go:109` and `transcribeverdict.go:506` was moved onto a forge read rather than re-expressed in one of those forms. A structural guard for those forms is a named follow-up (Task 6), not part of this row. **The path filter excludes `statusgen/decisionruling.go`, and only that file.** Its one site, `:643`, is the ruling resolver's `gh auth token` fallback, which forge-neutral/35 owns because moving it rewrites the inputs of the ruling-authenticity control. The two lines it still counts are `ghCommentResolver`'s and `ghVerdictIssueResolver`'s launches, which forge-neutral/35 also owns. A path filter cannot drop them, because the same two files hold `ghAuthorResolver`'s launch (identical text) and `ghVerdictMainHealth`'s, both this brief's; rows 19 and 20 tell them apart by function. 35's own row runs this command without the filter and takes the full count to `0`. The filter is anchored at the start of the path field, like the test-file filter, so it cannot drop a line from any other file. The comment filter drops only lines that begin with `//`, so a `"gh"` inside a block comment or in a comparison is still counted, which errs toward a non-zero count. The test-file filter matches the path field only. Measured at the freshness base: 26 sites across 15 non-test files. Re-measured 2026-10-06 at `11228951d`: the command without the decisionruling filter printed `32`, one line per forge-CLI launch site. The earlier `exec.Command("gh"` form printed `31` (16 non-test files) because it missed the one `exec.CommandContext(ctx, "gh", …)` launch at `autonomy.go:539`. The one comment line naming `"gh"` (`doratiming.go:48`) is excluded by the comment filter, and forge-neutral/33's census lists each site. Re-measured 2026-10-06 at `35c303e47` with BSD and GNU grep: this command prints `31`, the same 32 less the one line at `decisionruling.go:643`. Re-measured at `e6cb7d2a0` with BSD and GNU grep: `31` again, `32` without the decisionruling filter. Of those 31, exactly two lie in 35-owned functions in files that keep other sites (`transcribescan.go:109` in `ghCommentResolver`, `transcribeverdict.go:506` in `ghVerdictIssueResolver`, per rows 19 and 20 with BSD and GNU awk), which is why the completion Expect is `2`. Re-written 2026-10-03 (#1862): every grep stage tolerates only the no-match status, so a missing path or a grep error fails the row instead of passing it. The `test -d` leg covers BSD grep, which stays silent on an absent directory under `--include`. |
| 4 | check:ci +flow | `cd statusgen && go test ./... -run TestLintOfflineMakesNoNetworkCall -count=1 -v` | **negative path**: a full `--lint` with no `--forge`, run against a harness whose network dial hook FAILS the test on any attempt and whose `PATH` contains no `gh` and no `deskread`, completes with the same verdict as a networked run. Fails if any connection is attempted or any forge process is started |
| 5 | check:ci +mutation | `cd statusgen && go test ./... -run TestForgeBackedChecksReportCouldNotCheckOffline -count=1 -v` | **negative path**: with the offline reader wired, every forge-backed check renders could-not-check AS ITSELF. The test fails if any of them renders clean, and it enumerates the checks so a newly-added one that forgets is caught rather than skipped |
| 6 | check:ci +mutation | `cd statusgen && go test ./... -run TestIssueDebtNoticeOptInOnly -count=1 -v` | **negative path**: `--lint` alone emits no issue-debt line and starts no process; `--lint --forge` emits it from the verb's JSON. The test fails if the notice appears without `--forge` |
| 7 | check:ci +flow | `cd tools/desk && go test ./cmd/deskread/... -run TestDeskreadIssuesRoundTrip -count=1 -v` | exit 0 — `deskread issues` over a repo SET returns the versioned envelope on BOTH the GitHub and the GitLab fixture backend, via `ForgeFor`, with one invocation per repo SET and not one per repo |
| 8 | check:ci +mutation | `cd tools/desk && go test ./cmd/deskread/... -run TestDeskreadPartialIsNotAnError -count=1 -v` | **negative path**: one unreadable repo in a set of three lands in `partial` with its reason, the other two in `repos`, and the exit code is 0; ALL repos unreadable exits non-zero. Fails if a partial read is rendered as an empty success |
| 9 | check:ci +mutation | `cd statusgen && go test ./... -run TestParseBriefFileMemoDistinctPaths -count=1 -v` | exit 0 — over a fixture tree the memo instrument reports distinct-path parses equal to the file count, and a file whose mtime/size changes mid-run is re-parsed. Measured before: 3,351 calls over 172 distinct paths, max 23 per file |
| 10 | check:ci +dereference +mutation | `cd statusgen && go test ./... -run TestAttributionOneWalkMatchesPerPathReads -count=1 -v` | exit 0 — for every brief in a fixture tree the walk-derived first/last author equals what the per-path `git log` pair returns, INCLUDING the unreadable-history case, which must still degrade loudly per brief |
| 11 | check:ci +mutation | `cd statusgen && go test ./... -run TestChangedOnlyRefusesInCIGate -count=1 -v` | **negative path**: `--lint --changed-only <paths>` under the CI-gate environment REFUSES non-zero and names the requirement; outside it, it runs and prints the scope banner. The test fails if any flag combination lets a scoped lint serve as the gate |
| 12 | check +dereference | git-subprocess count per `--lint`, counted with a `PATH` shim that logs every `git` argv | ≤ 100 after. Measured before on this repository: **254** (144 log, 62 blame, 30 show, 9 merge-base, 3 rev-parse, 2 ls-tree, 5 others) |
| 13 | check +dereference | wall time of `--lint` on a tree of 400 briefs or more, offline, best of three | at least 60 % faster than the same tree's pre-change offline time. Measured before on this 165-brief repository: 6.05 s offline, 23.66 s with `gh` on `PATH` |
| 14 | check:ci | `cd statusgen && go test ./... -run TestActingIdentityUnchanged -count=1 -v` | exit 0 — `forge-neutral/07`'s Evidence-actor and witness behaviour is byte-identical before and after. The reads moved; who is recorded as having acted did not |
| 15 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeSurfaceUnchangedByDeskread -count=1 -v` | exit 0 — the `Forge` interface gains no method; `.github/workflows/forge-surface-control.yml`'s three controls stay green and `allowedInvocationCeiling` is unchanged at 6 (`tools/desk/internal/forgeban/allowlist.go:94`, re-checked 2026-10-06 at `11228951d`; the freshness base had 7) |
| 16 | check | `statusgen --root . --lint` | `LINT: PASS`, exit 0 |
| 17 | check:ci +dereference | `statusgen --root . --consumers --brief forge-neutral/18` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff. Note: a repo-wide brief-id format mismatch is open against `--consumers` (#954) and has produced a could-not-check on sibling briefs' equivalent row; this row is satisfied by exit 0 OR by that same could-not-check citing #954, not by a silent skip |
| 18 | check:ci | `cd tools/desk && go test ./cmd/deskread/ -run '^TestCITransportKindsIssue$' -count=1 -v > "${TMPDIR:-/tmp}/b18-r18.out" 2>&1 && grep -F -e '--- PASS: TestCITransportKindsIssue' "${TMPDIR:-/tmp}/b18-r18.out"` | exit 0, `--- PASS:` printed. `TestCITransportKindsIssue` (planned) asserts that `issue` is on `ciTransportKinds` and on `readKinds`, and that a `deskread issue` call under the CI transport for the job's own repository reads, while the same call without the transport's opt-in does not use the job token. It also asserts every other kind this brief added to `ciTransportKinds` is on `readKinds` |
| 19 | check +dereference | `test -f statusgen/transcribescan.go && awk '/^func /{fn=$0; sub(/^func (\([^)]*\) )?/, "", fn); sub(/\(.*/, "", fn)} /"gh"/ && !/^[[:space:]]*\/\// {print fn}' statusgen/transcribescan.go` | output is exactly one line, `ghCommentResolver`. The command prints the name of the function enclosing each non-`//` line of `transcribescan.go` that carries `"gh"`, method receivers included. It is row 3's file-scoped twin: row 3 can drop `decisionruling.go` by path, but not this file, whose two launches are textually identical and belong to different briefs. Measured at `e6cb7d2a0` with BSD awk and GNU awk: two lines, `ghAuthorResolver` then `ghCommentResolver`. After this brief `ghAuthorResolver` reads through the reader; if forge-neutral/35 has landed, the output is empty. Any other name, or the same name twice, fails the row |
| 20 | check +dereference | `test -f statusgen/transcribeverdict.go && awk '/^func /{fn=$0; sub(/^func (\([^)]*\) )?/, "", fn); sub(/\(.*/, "", fn)} /"gh"/ && !/^[[:space:]]*\/\// {print fn}' statusgen/transcribeverdict.go` | output is exactly one line, `ghVerdictIssueResolver`. Row 19's twin for `transcribeverdict.go`, by the same command: the file's other launch, in `ghVerdictMainHealth`, is this brief's, and `ghVerdictIssueResolver` is forge-neutral/35's, so a path filter cannot separate them. Measured at `e6cb7d2a0` with BSD awk and GNU awk: two lines, `ghVerdictIssueResolver` then `ghVerdictMainHealth`. After this brief `ghVerdictMainHealth` reads through the reader; if forge-neutral/35 has landed, the output is empty. Any other name, or the same name twice, fails the row |

### Named mutations for the `+mutation` rows

Each row marked `+mutation` names the change that must REDDEN it. A control whose mutation was
never observed reddening is a control whose strength is asserted, not shown.

| Row | The mutation that must redden it |
|---|---|
| 5, 6 | make the issue-debt path ignore the reader's unavailable list — treat a PARTIAL read as a complete one. The line must then be emitted from a subset and the row must fail |
| 8 | render an unreadable repo in `repos` with an empty issue list instead of in `partial`. The row must fail on the count, not merely on a message |
| 9 | delete the memo lookup in `parseBriefFile`. Distinct parses then equal CALL count and the row fails |
| 10 | swap the two assignments in the authorship walk (take the first sighting as the introducing commit). The multi-author fixture file must then resolve first and last inverted |
| 11 | drop the CI-gate detection from `--changed-only` so it merely warns. The row must fail on the exit code |

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
| A CI-lane site is moved onto `deskread` but its kind is not on `ciTransportKinds`, so the read refuses in CI and the check reads could-not-check there | row 18 for `issue`; each further kind lands with its site in one diff, and the CI-transport brief's own test pins every entry to `readKinds` |
| The ruling resolver is moved here after all, so a change to a control's inputs lands under a model gate | row 3's path filter names `decisionruling.go` and nothing else; Review confirms this brief's diff does not touch it (forge-neutral/35 owns it) |
| The sign-off resolver is moved here along with `ghAuthorResolver`, its neighbour in the same file, so the comment-selection change for both enactment gates lands under a model gate | row 3 still counts its line and row 19 names it, so moving it here changes both rows' output; Review confirms the diff leaves `ghCommentResolver` and its three `main.go` call sites unchanged |
| The verdict-issue read is moved here as a like-for-like read, so the edited-issue refusal its old source never armed is switched on under a model gate | row 3 still counts its line and row 20 names it, so moving it here changes both rows' output; Review confirms the diff leaves `ghVerdictIssueResolver` and its two `main.go` call sites unchanged |
| A control-feeding read moves to a source whose signal differs, for example a missing cross-repo flag read as same-repo, or an empty author type read as `User` | the signal table in Task 6, whose last line routes such a site out; Review checks each table row against the diff |
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
can serve as the CI gate. The reviewer also confirms that this brief's diff does not touch
`statusgen/decisionruling.go` or the call at `corroborate.go:1481`, and that row 3's path
filter excludes that one file and nothing else. Both belong to forge-neutral/35. Likewise the
diff leaves `ghCommentResolver` in `statusgen/transcribescan.go` and its three `main.go` call
sites unchanged, and row 19 prints that one name. The diff also leaves `ghVerdictIssueResolver`
in `statusgen/transcribeverdict.go` and its two `main.go` call sites unchanged, and row 20 prints
that one name. For each row of the signal table in Task 6,
the reviewer confirms the moved read carries the stated signal and that a missing value is
could-not-check, not a pass.
