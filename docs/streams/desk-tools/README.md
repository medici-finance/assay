---
stream: desk-tools
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
board: generated
---

# desk-tools Stream

The planning board for the `tools/desk/` desk-tools suite — the desk binaries
(`deskboard`, `deskpost`, `deskpr`, `deskwt`, `verifyloop`, the loop engine, and the
`deskkit` internal library) that drive the process desks. The desk-tools source lives in
this repo (`tools/desk/`), so its planning lives here too, alongside the code it plans.

Briefs are self-contained `brief-NN-*.md` files; each carries its own scope, rules, task,
and an executable Verify table that runs in an assay checkout (`tools/desk/`, `statusgen/`).
This opening set re-homes the open desk-tools-source planning briefs onto the public board
where their code now lives: the `.assay-versions` binary-channel contract, the drain-engine
batch-fanout consumer, the published-tree residual-identity scrub, the deterministic verdict
runner, and the escape-valve `Decide()` primitive.

Brief 07 joins them by the same route. `clusterguard` was planned on a private board, on the
reasoning that a security control's mechanism is a targeting aid; a disposition review found
nothing in it specific to any one deployment — it is the same generic guard-family shape as
`writeguard`, reading a documented opt-in — so the brief lives here with the code it plans.

Briefs 08–16 come from a different source: a 24-hour sweep of fifteen desk-role and worker
session transcripts, tallied per session, looking for the verbs sessions kept re-implementing
by hand. Each was checked against the source on main first; two reported gaps (long Go test
names and `sha256:` digest pins tripping the body scan) were already clean and are not
authored. What remains is one brief per verb: an authenticated `deskgit` transport, an
installation-coverage listing for `desktoken`, a stale-claim probe with branch liveness for
`deskclaim`, a prunable-holder reading for `deskwt add`, a key-to-brief resolver in
`statusgen`, a paced PR monitor script, three measured body-scan false-positive classes with
an `--explain`, an operator-supplied home on `deskdispatch --dry-run`, and a block-level
no-op in `deskevidence`.

Brief 21 comes from an operator request recorded 2026-09-09: more descriptive errors out of
the desk tools, a debug/trace option, and the actual error message instead of a swallowed one.
Reading the four verbs it names turned up one defect and two corrections. The defect is real and
widespread — a desk tool's FIRST stderr line is its config echo, so every step report built from
`firstLine(stderr)` printed the echo and never the diagnosis — and one call site in the same file
already carried the strip that fixes it. The corrections are recorded in the brief rather than
quietly dropped: two of the five reported symptoms did not reproduce as described, and what was
actually missing in both was the STRUCTURED half (a command line, a child exit status) that a
trace needs, not the text. The brief also declines one thing the request floated: a
`--print-token` flag on `desktoken` would defeat a posture that tool states in its own source,
and changing it is a human ruling rather than a worker's judgement.

Briefs 17–19 are the three trust-gate rulings recorded 2026-09-06. Two of them RELAX a control
and one TIGHTENS one, and they are authored together because they answer one question between
them — where the human belongs in a public-repo loop. 17 and 18 move the human decision off the
per-item path (the board's author bar, the per-write `+1`) and onto the two surfaces that were
always the real controls: the configured trust roster, and the human merge. 19 moves in the
opposite direction, closing a fail-OPEN in `verifyloop plan` where the most serious risk answer
a brief can carry — `irreversible` — routed it to a dispatchable tier while its milder siblings
were correctly held back. Each carries a single-point-of-failure note naming the one control it
leaves standing and the layers behind it; each Verify table carries a negative control, because
a relaxation verified only on the cases it means to admit has verified nothing.

Brief 22 comes from an issue filed against the trust gate itself (#933): `TrustedAuthor` and
`TrustedHumanAuthor` match a configured login as a pure string/id comparison, with nothing ever
asking whether the GitHub account behind that login still exists, was renamed, or was reclaimed
by someone else. The brief adds a read-only, fail-closed liveness read surfaced as a
`deskroster liveness` NOTICE only — it does not wire into any `Trusted*`/`Blessed` verdict, and
it stays outside the frozen `Forge` interface (a plain method on `*GitHubForge` alone, GitLab
out of scope) precisely so it adds a sibling monitoring surface rather than touching the gate
it watches. Auto-revocation is named as separate, explicitly human-gated follow-up.

Brief 23 comes from an operator request recorded 2026-09-14: when telemetry is on, log non-PII
tool usage and execution times, and keep a short history a later run can read performance
information from. The tools already record WHAT they did — one audit row per invocation — but
that ledger carries no duration, no exit code and no child-process count, and it cannot grow
one: it is load-bearing state for the write budget and the idempotency store, so it is
append-only and never rotated (measured on one operating desk host: 108 MB across 204,252 rows
in 32 days). So the brief adds a SEPARATE, opt-in, local-only perf record written by the shared
substrate at one place, kept for 7 UTC days and pruned on write, with a closed field set and no
free-text field at all — and one read verb, `deskperf`, for per-tool percentiles and boot-step
cost. It inherits `docs/telemetry.md`'s promise and its exact `ASSAY_TELEMETRY` switch. A remote
sink is explicitly NOT in it: nothing here opens a socket, and only the on-disk shape is fixed,
so a later sender reads it unchanged. It is the stream's first wave-2 brief — the child timings
attach to brief 21's one subprocess runner rather than to a second measurement.

Brief 24 comes from issue #1035, filed against the same ledger brief 23 declined to extend, and
acts on the cost of reading it. `deskkit.Guard()` — the mandatory first call of every desk verb —
ends by asking whether the LAST audit line was a `disabled` line, and answers it by parsing the
whole file: ~0.6 s per invocation against a 105 MB ledger, on read-only verbs too, three times over
on a write path, two of those inside the audit flock. The brief makes each read proportional to its
ANSWER rather than to the file — a bounded tail read for the last entry, and for the rate limiter a
bounded reverse read whose stop conditions are the meters' own termination conditions, falling back
to the full parse whenever its answer is not yet determined, so no budget, breaker or idempotency
verdict can change. It stops `desktoken` writing an audit row for a cache reuse that performed no
act (the ledger's largest single contributor of rows), and rotates the ledger into daily
`audit.jsonl.<date>` segments — deleting nothing, and carrying the counter and the idempotency store
forward by making every reader, `deskaudit recover` included, read across the segment boundary. It
also gives the ledger the read verb it never had, `deskaudit tail`. One correction it records rather
than drops: only two of the three full parses on a write path are last-entry reads; the third feeds
the idempotency predicates, which are whole-ledger by contract, so it keeps its full parse and
bounding it is named as separate follow-up.

Brief 25 comes from the same 2026-09-14 performance review, filed as #1036: the path that turns
a role and an account into an App installation token has no memo at any layer, so a board read
that touches ten repositories forks the `desktoken` binary 140 times a tick for a credential
that has not changed — and underneath it the minter resolves the installation id, signing a JWT
and calling the installations endpoint, BEFORE it consults the token cache, because the cache
path is keyed by that id. So every "reuse cached token" is still a round trip. The brief fixes
both ends of one path: a per-process memo keyed (role, owner) in front of the minter, bounded
under the minter's own 50-minute reuse window; and a cache-first resolution order in
`desktoken`, with an owner sidecar that proves which App and which account a cached file belongs
to and a 24-hour install-id cache behind it. Every fast path is gated on a positive identity
match and falls through to authoritative resolution on absence, ambiguity or a malformed name;
no custody check is moved or relaxed, and the environment override stays first. It is
independent of every other brief in the stream, including the separately filed brief on the
shared substrate's per-invocation audit parse (#1035) — that one makes each invocation cheaper,
this one makes there be fewer of them.

Brief 26 comes from issue #1037, a measurement rather than a request: five desk windows booting
inside one minute each ran the `deskwt prune` boot step against one checkout carrying ~657
registered worktrees over a 5,779-commit `origin/main`, and all five sat at ~100 % CPU for 8–12
minutes. Enumeration is not the cost (~1.2 ms per worktree); ~97 % of it is three in-process
go-git walks repeated PER CANDIDATE, and most of that is work no gate reads — a full history
walk to render a commit COUNT inside a skip string nothing parses, an unmemoized ancestor walk
that runs to exhaustion for the 19-in-20 candidates that are genuinely unmerged, and a full
`Status()` over ~4,100 files run BEFORE the merge gate that would have held the worktree anyway.
This is the same defect the stream's own `tools/desk/internal/gitcore/contains.go` header
already diagnoses and fixes for a different caller, so the brief takes the same shape: ONE walk
per sweep into an
ancestor-hash set, the merge gate ahead of `Status()`, one shared object cache, and the count
dropped from the skip string. It closes three structural defects found alongside — prune takes
no lock of any kind, each removal runs its own prune plus a full worktree listing (a quadratic
term the measured sweep never paid only because it removed nothing), and `--dry-run` is not
read-only — and adds a prune singleton whose lock fails CLOSED while its TTL debounce fails
OPEN, so the stale-lock class it exists to avoid cannot be recreated. Every gate is preserved:
the brief changes the ORDER and the SHARING of the work, never which worktrees are eligible for
removal, and its Verify table compares removal SETS rather than timings for exactly that reason.

Brief 27 comes from a desk-tools performance review recorded 2026-09-14 — a boot-and-per-tick
usage inventory of the binaries, then a reading of the high-use ones against their own source.
Its finding is not a missing optimisation but an unfinished one: `sweep.go` was written to put
every board-wide verb on a bounded worker pool, and two of six got there. `prs`, `stalled` and
the always-on policy-drift probe still walk the roster one repo at a time — and the drift probe
is the one that matters most, because it rides inside `actions`, which is 86.6% of recorded
`deskboard` invocations, so the hottest verb in the suite carries a ten-deep serial chain of
forge reads beside a pool that is idle while it runs. Four smaller findings travel with it, each
the same shape of work already paid for twice: `throughput` re-runs three whole verbs and
resolves every configured root twice to read four integers; `deskflip` evaluates `checks-green`
sixth although it is by a wide margin the largest refusal condition it reports, and reads the
same check rollup twice under two different names; the drift self-check resolves only the bare
`desk-tools` pin name, so an adopter who pins the per-platform line the distribution contract
tells them to pin gets a permanent could-not-check reported as STALE; and two thirds of
`deskpost`'s refusals are body-schema refusals that its own `--dry-run` would have caught, not
one of which names `--dry-run`, while `deskpr` has no offline check at all and 1,043 recorded
audit rows are nothing but somebody typing `--help`. Two of the dispatch note's own claims did
not survive the read and are corrected in the brief rather than repeated — `mergeable` is not
re-read (it comes from the PR payload already, which makes it a better reordering win, not a
worse one), and `checks-green` is 54.9% of condition-named refusals, not ~70%. A cursor-based
`--delta` is explicitly NOT in it: a cursor advances on a read, and a row the desk never
rendered would then never be shown again, which is exactly the blindness `delta.go` is built to
refuse — that needs its own brief and its own reset rule.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Binary channel sealed — publish the `.assay-versions` contract, validate it, stamp desk-tools with its release tag](brief-01-binary-channel-and-pin-contract.md) | 1 | M | implemented | — | — |
| 02 | [Generalize — batch-fanout as the second drain-engine consumer (contract validation)](brief-02-generalize-batch-fanout.md) | 1 | M | implemented | — | — |
| 03 | [Published-tree residual-identity scrub — drive the cold-read to an independent CLEAN](brief-03-published-tree-residual-scrub.md) | 1 | M | implemented | — | — |
| 04 | [Deterministic runner: execute rows, batch ~5 min, sign, file verdict issues](brief-04-runner-verdict-batching.md) | 1 | M | done | 2026-09-01 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #306 @ 4f37b243efb70e1b1d3e726bc4019967ad64ad99) |
| 05 | [Escape-valve `Decide()` primitive in deskkit — enum-bounded agent consults for deterministic loops](brief-05-escape-valve-decide.md) | 1 | M | done | 2026-08-26 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #156 @ 112b206fee74b470016be325dc7c2dfeff670931) |
| 06 | [Roster from deployment — resolve trust / role-binding config from the cell registry + mounted secrets, not a machine-local `roster.env` (design direction)](brief-06-roster-from-deployment.md) | 1 | M | done | 2026-09-01 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #318 @ 6ab8de53a40c1a4f71fa6c0a0ddccb4b27a000c8) |
| 07 | [`clusterguard` — exec-boundary shim for cluster CLIs, operator opt-in](brief-07-clusterguard-exec-shim.md) | 1 | M | implemented | — | — |
| 08 | [`deskgit push` / `deskgit fetch --as <role>` — authenticated transport from the role's token file](brief-08-deskgit-authenticated-push-fetch.md) | 1 | M | done | 2026-09-16 verify-desk (desk-tools/08 dispatched verifier; 11/11 rows PASS; risk-values all DERIVED) | 2026-09-17 assay-reviewer-app[bot] (approved PR #1221 @ 4eb96c84e85cdac42891bc45323e583736bee93d) |
| 09 | [`desktoken coverage <role>` — list the repositories a role's App installations can see](brief-09-desktoken-coverage.md) | 1 | M | implemented | — | — |
| 10 | [`deskclaim stale` + branch-liveness on `acquire` — reclaim a dead session's claim through the tool, not by hand](brief-10-deskclaim-stale-probe-and-branch-liveness.md) | 1 | M | done | 2026-09-16 verify-desk (desk-tools/10 dispatched verifier; 11/11 rows PASS incl. 2 previously-HELD rows now resolved; risk-value DERIVED on beaconFreshWindow) | 2026-09-17 assay-reviewer-app[bot] (approved PR #1236 @ 55f5c35989cad8de01d21ab052e06275b2c162e0) |
| 11 | [`deskwt add` — a worktree whose directory is gone does not hold its branch](brief-11-deskwt-add-prunable-holder.md) | 1 | S | blocked | — | — |
| 12 | [`statusgen brief <stream/NN>` — resolve an item key to its file, frontmatter and board row, as JSON](brief-12-statusgen-brief-subcommand.md) | 1 | M | done | 2026-09-16 verify-desk (desk-tools/12 dispatched verifier; 9/9 rows PASS, corrects a prior over-broad row-8 HELD; risk-value DERIVED on exit-code contract) | 2026-09-17 assay-reviewer-app[bot] (approved PR #1237 @ 27030b470e5c4381915c3a25da4f9602c6ab771c) |
| 13 | [`pr-monitor.sh` — a paced, per-repo head-sha / draft-state PR monitor shipped in the plugin tree](brief-13-paced-pr-monitor.md) | 1 | M | done | 2026-09-15 sonnet-5-verifier (8/8 rows PASS incl. a negative-control call-count assertion; risk-value DERIVED) | 2026-09-15 assay-reviewer-app[bot] (approved PR #1118 @ d7af8a7ba30d4aa86e3d1b64d956b22396fd6cf8) |
| 14 | [bodycheck — three measured false-positive classes into the negative corpus, plus `--explain`](brief-14-bodycheck-negative-classes-and-explain.md) | 1 | M | done | 2026-09-07 opus-4.8[1m]-verifier | 2026-09-07 assay-reviewer-app[bot] (approved PR #609 @ f82d0db3ffafb37fb16d262a52b5f2988981869d) |
| 15 | [`deskdispatch --dry-run --worktree <path>` — render the prompt against an operator-supplied home](brief-15-deskdispatch-dryrun-worktree.md) | 1 | S | implemented | — | — |
| 16 | [`deskevidence` — an Evidence block equivalent to one already standing is a no-op, not a second block](brief-16-deskevidence-block-equivalence-noop.md) | 1 | S | done | 2026-09-15 sonnet-5-verifier (7/7 rows PASS incl. a negative-control equivalence test; risk-value DERIVED) | 2026-09-15 assay-reviewer-app[bot] (approved PR #736 @ 95a31a5698e3943ae16efb01cb642d08d573da57) |
| 17 | [One trust bar for public-repo authors — `deskboard` classifies on the same predicate `deskpost` gates on](brief-17-one-trust-bar-public-authors.md) | 1 | M | implemented | — | — |
| 18 | [Public-repo write gate — an allowed-repos entry tagged `:public` authorizes outward writes, replacing the per-item `+1` reaction check](brief-18-allowed-public-repos-write-gate.md) | 1 | M | implemented | — | — |
| 19 | [`verifyloop plan` fails safe on risk — any risk answer `yes` routes to ROUTE-HUMAN, and the Evidence-only lane says so](brief-19-verifyloop-risk-fail-safe-routing.md) | 1 | M | implemented | — | — |
| 20 | [Cross-repo triage/verify evidence binds to the remote — a sibling checkout must be cross-checked, not trusted as-is](brief-20-cross-repo-remote-verify.md) | 1 | S | done | 2026-09-06 opus-4.8[1m]-verifier | 2026-09-07 assay-reviewer-app[bot] (approved PR #556 @ b3294437716536ad815cf13b2b80490e7bf4a4df) |
| 21 | [`DESK_TRACE` and cause-carrying errors — one subprocess runner, and a swallowed child's message reaches the operator on the first read](brief-21-desk-trace-and-cause-carrying-errors.md) | 1 | M | verified | 2026-09-16 verify-desk (desk-tools/21 dispatched verifier; 16/16 rows PASS incl. mutation-testing row; risk-value NAMED-NOT-DERIVED on traceStepCap filed assay#1240) | — |
| 22 | [Trust-gate account-liveness NOTICE — `deskroster liveness` reads what GitHub currently says about a trusted login, without touching `TrustedAuthor`'s verdict](brief-22-trust-gate-account-liveness-notice.md) | 1 | M | done | 2026-09-16 verify-desk (desk-tools/22 dispatched verifier; 13/13 rows PASS; risk-values DERIVED x2, NAMED-NOT-DERIVED x2 filed assay#1242) | 2026-09-17 assay-reviewer-app[bot] (approved PR #1243 @ d156ff7d8e6cff02d849b652a9bb5075fd964cdf) |
| 23 | [Opt-in local usage + timing telemetry — a per-invocation perf record with a 7-day history, and `deskperf` to read it](brief-23-usage-and-timing-telemetry.md) | 2 | M | todo | — | — |
| 24 | [Audit ledger — bounded tail read in `Guard`, no `desktoken` cache-reuse rows, daily rotation, and a `deskaudit tail` read verb](brief-24-audit-ledger-tail-read-and-rotation.md) | 2 | M | done | 2026-09-17 sonnet-5-verifier (14/14 rows PASS incl. SPOF equivalence + 10/10 mutation-caught; found+filed unrelated statusgen consumers bug assay#1296; risk-values DERIVED) | 2026-09-18 assay-reviewer-app[bot] (approved PR #1297 @ 08829451c27fb6a8f943349b8e9b78e8965e1541) |
| 25 | [One token lookup per owner per process — a memo in front of the minter, and `desktoken` consulting its cache BEFORE it resolves the install id](brief-25-token-memo-and-cache-before-install-id.md) | 2 | M | implemented | — | — |
| 26 | [`deskwt prune` — one origin/main walk per sweep, the merge gate before `Status()`, batched removal, a read-only `--dry-run`, and a prune singleton](brief-26-deskwt-prune-one-walk-and-a-lock.md) | 2 | M | implemented | — | — |
| 27 | [`deskboard`'s last serial repo loops onto the pool, `throughput` from one root resolution, `deskflip`'s cheapest gate first, and refusals that name the offline check](brief-27-deskboard-concurrency-and-refusal-ergonomics.md) | 2 | M | in-progress | — | — |
<!-- statusgen:briefs:end -->

## Critical path
desk-tools/21 → desk-tools/23. That is the stream's only typed edge: brief 23's per-child
timing attaches to the ONE subprocess runner brief 21 delivers, so a second measurement is
never written. desk-tools/27 carries none: its dispatch note asked for a prerequisite on a
timing-record brief, and its Dependencies section records why that edge is not encoded —
nothing in 27 reads a perf record and every one of its Verify rows is provable with the shell's
own timer, which is how its baselines were taken. Every other brief is
independent and self-contained. The soft ordering their
source streams carried (a version-scheme brief ahead of 01, the drain engine ahead of 02, a set
of risk-path briefs ahead of 03, the verdict payload/row-classes ahead of 04) is satisfied by
work already landed outside this stream, so 21 → 23 is the only typed `depends:` edge in the
stream — see each brief's Dependencies note.

## Dependency waves
- **Wave 1** — desk-tools/01, /02, /03, /04, /05, /06, /07, /08, /09, /10, /11, /12, /13, /14,
  /15, /16, /17, /18, /19, /20, /21, /22 (all independent; parallelizable). desk-tools/06
  is a design-direction brief: it records the direction and names a follow-on implementation
  brief-set, implementing none of it.
- **Wave 2** — desk-tools/23 (depends on desk-tools/21's one subprocess runner, which is
  present in the tree; the brief is dispatchable now), desk-tools/24 (no typed dependency —
  it is wave 2 because it reshapes the same ledger brief 23 reads the growth figures from, and
  the two are cleaner landed in sequence than in parallel), desk-tools/25 (no `depends:` edge
  at all — it is wave 2 because it is a performance change to an existing credential path
  rather than an independent feature), desk-tools/26 (no typed
  dependency — wave 2 by sequencing, not by blocking: it rewrites the gate ORDER of a
  destructive verb in `cmd/deskwt`, so it is kept out of the first wave's parallel band rather
  than made to wait on anything), and desk-tools/27 (no typed prerequisite — every deliverable
  and every Verify row in 27 is implementable and provable with the shell's own timer, which is
  how its baselines were taken). All five are dispatchable now.
  <!-- graph: not-a-gate -->

## Design notes
- [superseded-confirmation.md](superseded-confirmation.md) — the two-role `deskclose superseded`
  lane (worker proposes, reviewer confirms or disputes, role read from the token's roster binding)
  and the brief-level semantics of "superseded" (recommendation: a brief is superseded only by a
  dated re-baseline of itself; the word stays reserved for artifacts).

## Conventions
- Verify rows must be able to fail: exit status is the assertion, counts are captured and
  gated with `[ … ]`, and `\|` is never used as a regex alternation inside a GFM table cell
  (a basic grep reads the rendered bare pipe as a literal). Use `grep -e A -e B` for
  alternation.
- The desk-tools binaries are consumed as pinned release binaries via `.assay-versions`;
  desk-tools source changes are authored here under `tools/desk/`.
