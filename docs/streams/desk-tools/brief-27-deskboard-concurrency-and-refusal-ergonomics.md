---
brief: assay:assay:desk-tools:27
title: "`deskboard`'s last serial repo loops onto the pool, `throughput` from one root resolution, `deskflip`'s cheapest gate first, and refusals that name the offline check"
why: >-
  Every board-wide verb was meant to move onto the bounded worker pool `sweep.go` was written
  for, and three did not. `actions` and `health` sweep the roster six-wide; `prs`,
  `stalled` and the always-on policy-drift probe still walk it one repo at a time — and the
  drift probe is the worst of the three because it rides INSIDE `actions`, so the hottest verb
  in the suite (8,328 of 9,614 recorded `deskboard` invocations on one operating desk host)
  carries a ten-deep serial chain of forge reads beside a pool that is idle while it runs.
  `throughput` pays a different tax: it derives four stage depths by re-running three whole
  verbs, resolving and pinning every configured root twice over, for two integers. The same
  shape appears in two other places: `deskflip` evaluates `checks-green` sixth, behind the
  reviews read and the model-floor read, though `checks-green` is the single largest refusal
  condition it reports (651 of 1,185 condition-named non-OK outcomes measured on the same
  host), so the commonest refusal is also the most expensive one to reach; and the drift
  self-check resolves only the bare `desk-tools` pin name, so an adopter who pins the
  per-platform `desk-tools-<os>-<arch>` line the distribution contract tells them to pin gets
  a permanent could-not-check reported as STALE. Last, the refusal ERGONOMICS: two thirds of
  `deskpost`'s refusals are body-schema refusals that `--dry-run` would have caught for free,
  and not one of them names `--dry-run`; `deskpr` has no offline check at all; and 1,043
  recorded audit rows are nothing but somebody typing `--help`, each one a `refused` row in an
  append-only ledger that the write budget counts. This brief is one pass over all six: the
  same pool, one read where there were three, the cheapest gate first, the pin name the
  contract actually specifies, a refusal that tells you how to rehearse it, and a help screen
  that is not a refusal.
wave: 2
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-14 by a worker-desk authoring session, from a desk-tools performance review
  recorded 2026-09-14 and a same-day read of every surface named below
sources:
  - "Desk-tools performance review, 2026-09-14: a boot-and-per-tick usage inventory of the desk binaries, and a performance reading of the high-use ones."
  - "freshness-checked 2026-09-14 @ e428134c (origin/main) — every source fact below re-read against the tree at that commit, and every measurement below taken on one operating desk host on 2026-09-14."
  - "The pool that already exists and is already used by two verbs: `tools/desk/cmd/deskboard/sweep.go` § `sweepConcurrency` (= 6) / `sweepConcurrent` / `sweepRepos`, with its three stated invariants (no forge CLI, fail-closed on the LOWEST-INDEX error, exactly `limit` worker goroutines). Its callers today are `cmdActions` (board.go, per-repo and again per-PR) and `assessBranchHealth` (health.go)."
  - "The three serial loops: `tools/desk/cmd/deskboard/board.go` § `cmdPRs` (a plain `for … range deskkit.AllowedRepos()`) and § `assessPolicyDrift` (a plain `for … range deskkit.AllowedRepos()`, one `RepoVisibility` read each, called unconditionally from `cmdActions`); `tools/desk/cmd/deskboard/stalled.go` § `cmdStalled` (serial over repos AND, inside each, serial over that repo's PRs with up to four reads per candidate)."
  - "The per-root serial loops: `tools/desk/cmd/deskboard/dispatch.go` § `cmdDispatch` (one `nextUpForRoot` — a `statusgen --next-up` subprocess — per resolved root, in order) and `tools/desk/cmd/deskboard/nextup.go` § `cmdAwaiting` (one `gateScoresForRoot` — a `statusgen --gate-scores` subprocess — per resolved root, in order), each preceded by its own identical root-resolution, pin-resolution and `statusgen` version probe."
  - "The triple re-run: `tools/desk/cmd/deskboard/throughput.go` § `cmdThroughput` calls `cmdDispatch`, `cmdActions` and `cmdAwaiting` in full and reads one integer out of each, plus `dv.Roots`. It is also the file that states the rule this brief must not break: BLIND IS NEVER GREEN — a stage whose depth could not be read is could-not-check and is EXCLUDED from bottleneck selection, never counted as zero."
  - "The drift self-check: `tools/desk/cmd/deskboard/main.go` § `staleState` / `deskToolsPinReal`, which looks up exactly one artifact name, `desk-tools`, via `deskkit.ArtifactPin`."
  - "The pin contract that makes the bare name insufficient: `tools/desk/internal/deskkit/pins.go` § `ArtifactPin` — a TRAILING-SPACE PREFIX MATCH, so `desk-tools ` deliberately never matches `desk-tools-linux-amd64 `; and `docs/distribution.md` § Report packs, which tells an adopter to `pin a <artifact>-<platform> line`. `tools/desk/cmd/deskinstall/install.go` § `detectPlatform` spells the platform token `runtime.GOOS + \"-\" + runtime.GOARCH`."
  - "The flip gate: `tools/desk/cmd/deskflip/flip.go` § `flipConditions` (the ordered, test-pinned condition list) and the gate body, where `pr-open-draft` reads the PR document, `model-floor` and `reviewer-approved` read next, and `checks-green` reads the rollup sixth."
  - "The duplicated flip read: `tools/desk/cmd/deskflip/flip.go` § `readChecks` and § `checkRunsAtHeadReader` — two functions, each calling `fg.ChecksAtHead(fr, head)` for the same head in the same run, deliberately separate so a refusal names the right condition."
  - "The offline check that exists and is never advertised: `tools/desk/cmd/deskpost/writeflow.go` § `dryRun` / `deskkit.ResultDryRun` and `tools/desk/cmd/deskpost/main.go`'s usage block; `tools/desk/cmd/deskreply/deskreply.go`, whose `--dry-run` refuses unless `--workpad` is also given; and `tools/desk/cmd/deskpr/`, which has no offline check of any kind."
  - "The help-as-refusal path: every verb's subcommand builds a `flag.FlagSet` with `flag.ContinueOnError` and turns any parse error, `flag.ErrHelp` included, into `deskkit.Refused(\"bad flags: \" + …)` — e.g. `tools/desk/cmd/deskpr/deskpr.go` § `cmdCreate`, whose `defer ac.finalize(err)` is armed BEFORE `fs.Parse` runs, so the audit row is written for a help screen. The top-level `-h` in `tools/desk/cmd/deskpr/main.go` § `run` already returns before `deskkit.Guard()` and writes no row; the SUBCOMMAND form does not."
  - "The ledger the help rows land in: `tools/desk/internal/deskkit/audit.go` § `Log` (append-only, never rotated) and `ratelimit.go`, which counts its rows per canonical tool key for the write budget and the circuit breaker."
exec-tier: strong
exec-tier-why: >-
  (b) this moves three fail-closed read paths onto concurrency and REORDERS a security-adjacent
  gate. Both hazards are silent on the happy path. A concurrency conversion that drops the
  fail-closed rule turns one repo's unreadable state into a board that reads clean — and a
  green suite looks identical either way unless a row forces a repo to fail and asserts the
  whole run fails with it, naming that repo. The flip reorder is the sharper one: the gate's
  conditions are individually necessary and a reorder can only change WHICH refusal is
  reported first — unless a reorder accidentally moves a condition ahead of the read that
  supplies its input, at which point the condition evaluates against a zero value and passes
  vacuously. That is a fail-OPEN on a ready-flip, and only a row that drives each condition to
  refuse from its new position distinguishes it from a correct reorder.
consumers:
  - "`tools/desk/cmd/deskboard/sweep.go`: out-of-scope (read, NOT changed — `sweepConcurrent`, `sweepRepos` and `sweepConcurrency` keep their exact shape and semantics; this brief adds CALLERS to the existing pool and writes no second one)."
  - "`tools/desk/cmd/deskboard/board.go`, `stalled.go`, `dispatch.go`, `nextup.go`, `throughput.go`, `main.go`: fixed-here (each a bounded change named under `files:` — three loops re-expressed as pool callers, one shared per-root pool, one report-to-depth refactor, one pin-name fallback)."
  - "`tools/desk/cmd/deskboard/delta.go`: out-of-scope (read, NOT changed — the snapshot schema, the fail-to-FULL-output rule and the advance-only-on-a-rendered-read rule are untouched; a cursor-based delta is named as follow-up and implemented nowhere here, because a cursor that advances on a read the desk did not render is exactly the blindness that file's doc comment refuses)."
  - "`tools/desk/cmd/deskflip/flip.go`: fixed-here (the condition ORDER and one memoized rollup read; no condition is added, removed, or weakened, and every one still has to hold)."
  - "`tools/desk/internal/deskkit/`: fixed-here, additively (one help-request helper and one offline-check-hint helper, both new; `audit.go`, `ratelimit.go` and `pins.go` are read-only references — in particular `ArtifactPin`'s trailing-space prefix match is NOT relaxed, and Verify row 11 asserts a mutation that relaxes it is caught)."
  - "`tools/desk/cmd/deskpost/`, `cmd/deskpr/`, `cmd/deskreply/`, `cmd/deskwt/`, `cmd/deskfile/`, `cmd/desktoken/`: fixed-here (the six verbs retrofitted with the help helper; `deskpost`, `deskpr` and `deskreply` additionally with the offline-check hint and, for `deskpr`, one new `--check` flag)."
  - "Every OTHER desk verb: out-of-scope (a verb not retrofitted keeps today's behaviour exactly, so the help retrofit is additive and per-verb; widening it past the six named here is untracked follow-up, not a gap this brief leaves open)."
  - "`statusgen`: out-of-scope (invoked, never changed — the per-root concurrency runs only its READ-only query modes, `--next-up` and `--gate-scores`, against DISTINCT roots; no write mode is ever run concurrently and no flag of its is added)."
version: 1
id: 0505fb4e-15b5-4898-8bdc-7e3c2488cc37
---

# Brief 27 — `deskboard` on the pool, `throughput` from one read, `deskflip`'s cheapest gate first, refusals that name the offline check

## Dependencies
None typed, and the reason is recorded rather than left to be rediscovered.

The dispatch note for this brief asked for `depends: ["desk-tools/25"]`, **for the timing
numbers only**. Checked against the tree (worker-kit clause 7): there is no brief 25 on this
stream's board at `e428134c`, so a typed edge to it does not resolve —
`statusgen --lint` reports `depends "desk-tools/25" references unknown brief "25" in stream
desk-tools`, a PROBLEM that would redden the board for every reader, not just this brief. The
edge is therefore NOT encoded, and the relationship is stated here instead.

What that relationship actually is: nothing in this brief reads a perf record, and no
deliverable or Verify row below needs one. The before/after wall-clock rows are measured with
the shell's own timer on the same repo set in the same window — which is exactly how the
baselines in the `facts` section were taken. What a per-invocation timing record would add is
that the improvement keeps being visible AFTER this branch merges, instead of living in one PR
body. That is a nice-to-have on a brief that is otherwise self-contained, which is why losing
the typed edge costs this brief nothing. When a timing-record brief is on the board, the edge
can be encoded then — on that brief's `unblocks:`, or here — and it will resolve.

<!-- graph: not-a-gate -->

## Context

single-point-of-failure: **`sweepConcurrent`'s fail-closed rule — that any item's error fails
the WHOLE call, deterministically, with the LOWEST-INDEX item's error.** Three read paths that
today fail closed in a plain `for` loop start depending on it at once. Behind it are three
layers, each tripping on a different signal in a different component: (1) the rule is
ALREADY-WRITTEN and already load-bearing for two verbs — this brief adds callers to it and
writes no second implementation, so a regression in it reddens `actions` and `health` as well
as the three new callers, in tests that already exist (row 12); (2) each converted caller keeps
its OWN per-repo error wrapping, so the propagated error still NAMES the repo and the condition
it came from, and rows 1 and 2 drive a repo to fail and assert both the non-zero exit and the
repo's name in the message; (3) the partial-result discipline — each worker returns its own
value and touches no shared state, and the merge happens after the sweep in roster order — is
what makes a lost error impossible to hide behind a plausible board, and row 3 asserts the
concurrent output is BYTE-IDENTICAL to the serial one on a fixture where every repo succeeds.
The three are independent: a broken fail-closed rule is caught by (1)'s existing tests even if
the new callers are perfect; a caller that swallows its own error is caught by (2) even if the
pool is perfect; and a merge that reorders or drops a row is caught by (3) even if both of the
others hold.

risk note — all four risk answers are `no`, and the change touches a ready-flip gate. The
answers stand because no condition is added, removed, weakened or made conditional: the same
nine conditions must all hold, the reorder changes only which refusal is reported FIRST when
more than one would fire, and the shared rollup read returns the same bytes to the same two
consumers, each still wrapping it in its own condition's message. A reviewer who finds a
condition that can now be reached before the read supplying its input, a condition that can be
skipped, or a refusal whose condition name changed, flips `irreversible` to yes and takes the
human gate.

The `risk-files-crossread` lint NOTICE fires here and is answered rather than ignored: the
declared paths include `tools/desk/internal/deskkit/` and `tools/desk/cmd/deskflip/`, both
security-path triggers for this repo, and all four risk answers are `no`. The answers stand
because the `deskkit` changes are two NEW files carrying two pure helpers that read no
configuration, write nothing, and are called only from verb `main()`s; no trust gate, budget,
scan, exit code or refusal predicate is read or written by either. The `deskflip` change moves
existing evaluations and de-duplicates one read; it deletes no check. Rows 8, 9, 10 and 12 are
the mechanical evidence, and a reviewer who finds otherwise flips the answer.

files:
- `tools/desk/cmd/deskboard/board.go` (existing) — `cmdPRs` re-expressed as a `sweepRepos`
  caller over a new per-repo partial (the out-of-installation carve-out moves INSIDE the
  worker as a field on that partial); `assessPolicyDrift` likewise.
- `tools/desk/cmd/deskboard/stalled.go` (existing) — `cmdStalled` re-expressed as a
  `sweepRepos` caller, with its per-PR stage as a nested `sweepConcurrent` exactly as
  `cmdActions` already nests one.
- `tools/desk/cmd/deskboard/roots.go` (planned) (new, same package) — `rootConcurrency`, the
  shared per-root resolution (`resolveRootsOnce`) and the concurrent per-root runner both
  `cmdDispatch` and `cmdAwaiting` call.
- `tools/desk/cmd/deskboard/dispatch.go` and `nextup.go` (existing) — each loses its private
  root-resolution/pin/version preamble and its serial per-root loop to the shared pair above;
  neither changes its report shape, its population note, or its fail-closed behaviour.
- `tools/desk/cmd/deskboard/throughput.go` (existing) — `cmdThroughput` resolves roots once and
  reads depths through the depth-only halves instead of re-running three whole verbs.
- `tools/desk/cmd/deskboard/main.go` (existing) — `deskToolsPinReal` gains the per-platform
  fallback lookup. No other change to `staleState`'s order or its could-not-check arm.
- `tools/desk/cmd/deskboard/concurrency_test.go` (planned), `roots_test.go` (planned),
  `throughput_depth_test.go` (planned), `staleplatform_test.go` (planned) — all new.
- `tools/desk/cmd/deskboard/mutations.json` (planned) (new) — the mutation spec for row 11.
- `tools/desk/cmd/deskflip/flip.go` (existing) — the reordered `flipConditions` and gate body,
  and one memoized raw rollup reader behind `readChecks` and `checkRunsAtHeadReader`.
- `tools/desk/cmd/deskflip/mutations.json` (existing) — entries added for the reorder and the
  shared read; existing entries re-anchored where this diff moves their anchor text.
- `tools/desk/internal/deskkit/helprequest.go` (planned) (new) — `HelpOnly`, `ErrHelpRequested`,
  `IsHelpRequest`.
- `tools/desk/internal/deskkit/offlinecheck.go` (planned) (new) — `OfflineCheckHint`.
- `tools/desk/internal/deskkit/helprequest_test.go` (planned) and `offlinecheck_test.go` (planned) — new.
- `tools/desk/cmd/deskpost/`, `cmd/deskpr/`, `cmd/deskreply/`, `cmd/deskwt/`, `cmd/deskfile/`,
  `cmd/desktoken/` (existing) — the six-verb help retrofit; `deskpost`/`deskpr`/`deskreply`
  additionally route their body/schema refusals through the hint; `deskpr` gains `--check`.
- `tools/desk/README.md` (existing) — a short section on `deskpr --check` and on what
  `--dry-run` covers in the two verbs that already have it.
- `changelog/perf-desk-tools--27-09141630.md` (planned) (new).

facts (read at `e428134c`, 2026-09-14; measurements taken the same day on one operating desk
host with a ten-repo roster and four configured stream roots):

- **The pool exists, is documented, and has two callers.** `sweep.go` states its own purpose in
  its header — every board-wide verb "used to walk `deskkit.AllowedRepos()` in a plain serial
  `for` loop", and a measured `actions` run "spent ~3.5s of CPU across ~98s of wall clock — 96%
  of it waiting on the forge". It names `actions / prs / queue / health / stalled / policydrift`
  as the verbs that shape applies to. `sweepRepos` and `sweepConcurrent` are called from exactly
  two places today: `cmdActions` (per-repo, and again per-PR) and `assessBranchHealth`. The
  other four never moved.
- **`cmdPRs` is a plain serial loop.** It walks `deskkit.AllowedRepos()`, calls `fetchOpenPRs`
  per repo, and inside that repo walks the returned PRs — running `prBlessed` (a forge read) for
  every untrusted author and `probeZeroCI` (another) for every zero rollup. Measured: **20.10 s
  wall, 9.87 s user** over ten repos.
- **`cmdStalled` is serial twice over.** It walks the roster serially and, inside each repo,
  walks that repo's PRs serially, spending up to four forge reads on each candidate
  (`fetchReviews`, `fetchHeadCommit`, `fetchLastAuthorComment`, `fetchBehindMain`). Measured:
  **58.48 s wall, 27.64 s user**.
- **`assessPolicyDrift` is a serial loop INSIDE the hottest verb.** `cmdActions` calls it
  unconditionally, before the pooled PR sweep, and it walks the whole roster one repo at a time
  issuing one `RepoVisibility` read each — ten serial round trips on a ten-repo roster, with the
  six-wide pool idle beside it. `actions` measured **70.79 s wall, 60.11 s user**, and is
  **8,328 of 9,614** recorded `deskboard` invocations on this host — 86.6 %. Every one of them
  pays this chain. `assessBranchHealth`, which rides the same verb for the same reason, is
  already pooled; the drift probe is the one that was missed.
- **`throughput` re-runs three whole verbs for four integers.** `cmdThroughput` calls
  `cmdDispatch`, `cmdActions` and `cmdAwaiting` in full. `cmdDispatch` and `cmdAwaiting` EACH
  begin with the same four steps — `deskkit.ConfiguredRoots()`, `resolveStatusgen()`, a
  `deskkit.ResolveRoot` loop over every configured root, `resolveStatusgenPin(resolved)` and
  `statusgenVersionOf(bin)` — and then run one `statusgen` subprocess per root, serially. So a
  single `throughput` run resolves and pins every root twice and spawns 2N `statusgen`
  subprocesses in a serial chain. Measured: **82.82 s wall**, of which the `actions` call alone
  accounts for ~70.8 s; the two report re-runs are the remaining ~12 s, and `throughput` reads
  exactly two integers and one `Roots` slice out of them.
- **The two gate-score reads are NOT the same read, and must not be merged.**
  `execGateScores` (used by `actions` and `stalled`) runs `statusgen --gate-scores` against
  `findRepoRoot()` — the ONE root the process is standing in. `gateScoresForRoot` (used by
  `awaiting`) runs it against each CONFIGURED root. Folding one into the other would silently
  change `actions`' scope from single-root to multi-root, which is a behaviour change wearing a
  performance change's clothes. It is named here and left alone.
- **`deskflip` reaches its commonest refusal last but one among the reading conditions.**
  `flipConditions` is ordered `caller-role, app-token, pr-open-draft, model-floor,
  reviewer-approved, checks-green, mergeable, security-verdict, head-stable`. Measured over
  2,704 recorded `deskflip` invocations (1,514 OK, 621 unverifiable, 569 refused): of the 1,185
  non-OK outcomes that name a condition, **`checks-green` names 651 — 54.9 %**, more than twice
  the next (`reviewer-approved`, 224) and more than `security-verdict` (148), `mergeable` (99),
  `pr-open-draft` (43) and `model-floor` (16) combined. Every one of those 651 paid the PR read,
  the model-floor read and the reviews read before learning a check was red.
  **Correction to the dispatch note, recorded rather than dropped** (worker-kit clause 7): that
  note put `checks-green` at "~70 % of refusals" and called it the SEVENTH condition. Measured,
  it is 54.9 %, and it is the SIXTH entry in `flipConditions`. The conclusion the figure
  supports — that it is by a wide margin the largest single refusal condition and the last of
  the reading conditions to be reached — is unchanged; the figure and the ordinal are corrected
  here rather than repeated.
- **`mergeable` is NOT re-read — the dispatch note's second claim does not hold.** Checked
  against the primary artifact (worker-kit clause 7): the `mergeable` arm of the gate reads
  `pr.Mergeable`, a field `readPR` already populated from the single `GetPullRequest` call in
  `pr-open-draft`. It issues no forge call of its own. So `mergeable` is a ZERO-READ condition
  sitting SEVENTH, behind three conditions that each cost at least one read — which makes it a
  larger and simpler ordering win than the one the note described, not a smaller one, and it is
  taken below on that basis instead.
- **The real duplicated read in `deskflip` is the check rollup.** `readChecks` calls
  `fg.ChecksAtHead(fr, head)`; `checkRunsAtHeadReader` calls `fg.ChecksAtHead(fr, head)` for the
  SAME head in the same run, for `reviewer-approved`'s check-only exemption. Its doc comment
  states why they are separate and it is a good reason — "a refusal has to name the condition it
  belongs to or it sends the operator to the wrong gate" — but that reason argues for two
  WRAPPERS, not two round trips. Both already reconcile the forge's asserted total against what
  it served; both fail closed on a short read; neither inspects anything the other does not.
- **The drift self-check resolves one pin name and the contract specifies two.**
  `deskToolsPinReal` calls `deskkit.ArtifactPin(dir, "desk-tools")` and nothing else.
  `ArtifactPin` selects by a TRAILING-SPACE PREFIX MATCH, and pins.go's header states plainly
  that this is why `desk-tools ` "never matches `desk-tools-linux-amd64`, and vice versa" —
  a deliberate, load-bearing disambiguation. But `docs/distribution.md` tells an adopter to
  "pin a `<artifact>-<platform>` line", and `deskinstall` spells that platform token
  `runtime.GOOS + "-" + runtime.GOARCH`. So a consumer whose pin file carries
  `desk-tools-darwin-arm64` and no bare `desk-tools` line hits `found=false`, falls through the
  channel-D arm (no `desk-tools-source` line either) to the in-tree fallback, finds no
  `tools/desk` path in its own checkout, and lands on the could-not-check arm — which reports
  `stale: true` BY DESIGN, because an unverifiable drift check is not evidence of freshness.
  The verdict is right for what it observed; the observation is the bug.
- **`--help` is charged as a refusal, and the top-level form already proves it need not be.**
  `tools/desk/cmd/deskpr/main.go`'s `run` handles a bare `-h`/`--help`/`help` before `deskkit.Guard()`,
  prints usage, returns `ExitOK`, and writes no audit row — the comment says so in as many
  words. The SUBCOMMAND form does not: `cmdCreate` arms `defer ac.finalize(err)` and only then
  calls `fs.Parse`, so `deskpr create --help` turns `flag.ErrHelp` into
  `Refused("refused: bad flags: flag: help requested")`, exits 5, and appends a `refused` row.
  Measured: **1,043** such rows on this host — `deskpr` 538 (12.1 % of its 4,446 rows),
  `deskwt` 373 (6.7 % of 5,608), `deskfile` 110, `desktoken` 20, two others 1 each. They land in
  a ledger that `ratelimit.go` counts rows of for the write budget and the circuit breaker, and
  that `audit.go` never rotates.
- **The offline check exists in two verbs, is absent from a third, and is advertised by none of
  the refusals it would have prevented.** `deskpost --dry-run` runs every check and stops before
  the write, exit 0, audited `dryrun` — "a result class NEITHER meter counts". 261 `dryrun` rows
  were recorded, against 1,611 `deskpost` refusals of which **1,065 (66.1 %)** are two
  body-schema classes: a review body with no verdict line (602) and a review body with no `## `
  heading (463). Neither refusal names `--dry-run`. `deskreply`'s `--dry-run` refuses outright
  unless `--workpad` is also given, so the plain reply path has no rehearsal at all; its own
  largest refusal class is the workpad marker (196 of 589). `deskpr` has no offline check of any
  kind, and after the 538 help rows its largest refusal classes are a body with no `Brief:` /
  `Issue:` trailer (306) and a missing `--title` (84) — both decidable with no network at all.

facts — the design:

- **The pool is reused, never re-implemented.** All five converted loops call the existing
  `sweepRepos` / `sweepConcurrent` at the existing `sweepConcurrency` of 6. No new pool, no
  second implementation of the ordering rule, no change to `sweep.go`. Reuse-ladder rung 2.
- **Per-repo partials, merged after.** Each converted loop's body becomes a function returning
  ONE repo's own partial result — rows, quarantine rows, unreadable-repo entries, open ids,
  truncation flag — touching no state another goroutine can see. The merge runs after the sweep
  in roster order, and each report's existing final `sort` still imposes a total order, so the
  output is byte-identical to the serial version regardless of finish order. This is exactly the
  structure `cmdActions` already uses and `sweep.go` already documents.
- **`cmdPRs`' out-of-installation carve-out moves INSIDE the worker.** Today the `continue` on
  `outOfInstallation(err)` is what keeps a watched-but-uninstalled repo from failing the whole
  sweep. In the pooled form a worker cannot `continue` a loop it is not running, so the carve-out
  becomes: the worker returns `(partial{unreadable: …}, nil)` for that case and a real error for
  every other, and the merge appends the unreadable entries in roster order. The predicate is
  unchanged and the classification decision stays in one place.
- **`cmdStalled` nests, at the same bound `actions` already nests at.** Repos under
  `sweepRepos(…, sweepConcurrency, …)`; each repo's PRs under a nested
  `sweepConcurrent(prs, sweepConcurrency, …)`, which is the pairing `cmdActions` already runs.
  Concurrent forge reads are therefore bounded by the product, 36 — the same bound the suite
  already operates at, not a new one. The per-PR worker returns one of three outcomes (skip,
  unassessable row, stalled row) and the merge appends them in input order; no row is dropped
  and none is reclassified.
- **`assessPolicyDrift` keeps its never-fail contract.** Its worker returns
  `(visibilityObservation{repo, vis, observed bool}, nil)` — ALWAYS a nil error, because the
  whole point of the always-on probe is that an unreadable repo is left out of the `observed`
  map and reported NOT OBSERVED by `VisibilityDrift`, never a killed board. Fail-closed does not
  apply to this one caller and the code says so at the call site; the other four are fail-closed.
- **Per-root concurrency is its own bound, and a different KIND of work.** `rootConcurrency = 4`
  in the new `roots.go`, deliberately separate from `sweepConcurrency`: the per-root work is a
  LOCAL `statusgen` subprocess against a distinct directory, not a forge read, so it is bounded
  by CPU and disk rather than by a forge's secondary rate limit, and raising or lowering one must
  not silently move the other. Roots are few (four on this host); 4 is one wave.
- **Only `statusgen`'s READ modes run concurrently, and only against distinct roots.**
  `--next-up` and `--gate-scores` are query modes that write nothing. No write mode of
  `statusgen` is ever run under the pool, and a Verify row asserts each root's parsed rows are
  identical to a serial run's on the same roots.
- **One root resolution, shared.** `resolveRootsOnce()` in `roots.go` performs
  `ConfiguredRoots` → `resolveStatusgen` → the `ResolveRoot` loop → `resolveStatusgenPin` →
  `statusgenVersionOf` exactly once and returns the resolved set plus the pin/version facts.
  `cmdDispatch` and `cmdAwaiting` each call it (so their standalone behaviour is unchanged);
  `cmdThroughput` calls it ONCE and hands the result to both depth readers.
- **`throughput` reads depths, not reports.** `cmdDispatch` and `cmdAwaiting` are each split into
  a depth-producing half that takes an already-resolved root set and a rendering half. The verbs
  keep their exact current output; `throughput` calls only the depth halves, plus `cmdActions`
  exactly once, as it does today.
- **Blind stays blind, and a shared read that fails blinds BOTH stages it fed.** The existing
  `appendBlind` contract is unchanged: a stage whose depth could not be read reports
  could-not-check with the reason, is excluded from bottleneck selection, and is NEVER counted
  as zero. The one new obligation the sharing creates is stated rather than discovered: when the
  shared root resolution fails, the dispatch AND verify stages both go blind, each with a note
  naming the shared resolution as the thing that failed — so a reader is never left thinking one
  stage's depth is real because the other's was.
- **The drift fallback is a SECOND LOOKUP, not a looser match.** `deskToolsPinReal` tries
  `ArtifactPin(dir, "desk-tools")` first (today's behaviour, unchanged, and it still WINS when
  both lines are present), and only on a miss tries
  `ArtifactPin(dir, "desk-tools-"+runtime.GOOS+"-"+runtime.GOARCH)`. `ArtifactPin`'s
  trailing-space prefix match is not touched: the disambiguation that keeps `desk-tools ` from
  matching a per-platform line is the control, and this brief supplies the exact name instead of
  relaxing the matcher. A run whose verdict came from the fallback SAYS so in its detail line,
  naming the artifact it matched, so the two sources are distinguishable in the banner.
- **The flip condition order becomes cost-ordered — as far as a RECORDED RULE allows.**
  New order: `caller-role, app-token, pr-open-draft, mergeable, reviewer-approved, checks-green,
  model-floor, security-verdict, head-stable`. The reasoning, per condition: `caller-role` and
  `app-token` stay first because they are the no-network and credential gates and nothing may
  precede the credential one; `pr-open-draft` stays third because it performs the ONE read every
  condition after it consumes; `mergeable` moves UP from seventh to fourth because it costs
  NOTHING — it reads a field already in hand; `model-floor` moves DOWN from fourth to seventh
  because it is the condition that made every earlier refusal expensive (it buys a PAGINATED
  label-event timeline); `security-verdict` keeps its place before the end because it walks a
  paginated changed-file list; and `head-stable` stays LAST because its whole purpose is to be
  the final thing checked before the mutation. The `flipConditions` list is test-pinned, so the
  reorder is a deliberate, reviewed edit of both the list and the test — not a silent drift.
- **`reviewer-approved` KEEPS its place ahead of `checks-green`, and the reason outranks cost.**
  Cost alone would put `checks-green` fourth. It does not go there, because
  `tools/desk/cmd/deskflip/checkonlycr_test.go` records the opposite rule in two tests with their reasoning
  written out: when a standing CHANGES_REQUESTED claims the check-only exemption, the exemption
  must decide FIRST, "or an operator sent to the CI gate for a rejection that was never about CI
  goes looking in the wrong place". Moving `checks-green` ahead of it reddens both tests — one
  of them because a still-running cited run would then be reported as a pending rollup (exit 6)
  instead of as a rejected exemption claim (exit 5). Neither outcome flips a PR that should not
  flip, so this is a DIAGNOSTIC regression rather than a safety one, but it is a deliberate
  recorded decision and a performance brief does not get to overturn it quietly. The cost
  argument for swapping them has largely evaporated anyway: both conditions read the SAME rollup,
  and it is now read once, so the second of the two is free. Whether the exemption's precedence
  should be revisited is a design question with a named owner, not a worker's judgement — it is
  recorded here and left open.
- **What a REFUSAL costs after the reorder, in forge reads.** A `mergeable` refusal costs ONE —
  the PR document — down from four. A `checks-green` refusal costs THREE (the PR document, the
  reviews list, the rollup), down from four; the one dropped is the paginated timeline.
  **Correction to the dispatch note** (worker-kit clause 7): that note asked for a red-check
  refusal costing "exactly one forge read", which is not reachable — the head the rollup is
  addressed by comes from the PR document, so a rollup read is always preceded by a PR read —
  and with `reviewer-approved` correctly staying ahead it is three, not two. The rows below
  assert the reachable bounds and name what each read is.
- **One rollup read, two wrappers.** A memoized `checksAtHeadOnce(fg, fr, head)` performs
  `fg.ChecksAtHead` at most once per run and caches the RAW result and the RAW error.
  `readChecks` wraps it in `condChecksGreen`'s message (including its short-read reconcile) and
  `checkRunsAtHeadReader` wraps the same raw outcome in `condReviewerApproved`'s message
  (including its own, differently-worded short-read reconcile). The property the existing doc
  comment protects — a refusal names the condition it belongs to — is preserved exactly; what is
  removed is the second round trip, not the second message.
- **The help helper is two tiers, and the first cannot false-positive.** `deskkit.HelpOnly(args)`
  returns true only when the arguments after the leading subcommand token are EXACTLY one token
  and that token is exactly `-h`, `-help` or `--help`. A lone token cannot be another flag's
  value, so there is no argument shape it can misread; it is called at the top of each verb's
  `run`, BEFORE `deskkit.Guard()`, and prints usage and returns `ExitOK` with no audit row.
  Everything else falls through to normal parsing, where `flag.ErrHelp` is recognised by
  `deskkit.IsHelpRequest(err)`: the verb prints its subcommand usage, returns `ExitOK`, and its
  `auditCtx.finalize` writes NO row for it. Tier two runs after `Guard`, which is deliberate —
  a kill-switched session still records its own `disabled` row, which is the one row about a
  help screen that is worth having.
- **The hint is one string, produced in one place.** `deskkit.OfflineCheckHint(tool, flag)`
  returns a fixed template naming the tool and the flag and nothing else — no caller-supplied
  free text reaches it. Every body/schema refusal in `deskpost`, `deskpr` and `deskreply` is
  routed through one per-verb constructor that appends it, and a golden test drives the measured
  top refusal shapes through the real code paths and asserts the hint is present in each.
- **`deskpr --check` is offline by construction and three-state about what it could not check.**
  It runs every LOCAL gate the write path runs — flag validity, branch state, the `Brief:` /
  `Issue:` trailer, the secret scan, the public-repo self-containment scan, the push-transport
  gate — and STOPS before the push and before any forge write. It mints no token and opens no
  connection. Exit 0 only when every local gate passed; a failing gate returns its OWN refusal
  with its own exit code, so `--check` is a gate run early, never a preview that disagrees with
  the real thing. The categories it cannot decide offline — chiefly the bare-`#N` notice, which
  needs a reference number from the forge — are REPORTED as not checked, by name, rather than
  passing silently. It is audited `dryrun`, reusing `deskkit.ResultDryRun`; no new result class.
- **What is deliberately NOT here: a cursor-based `--delta`.** `delta.go` states its own
  fail-dangerous direction — "a delta mode that wrongly diffs against a stale/corrupt snapshot
  silently HIDES a new actionable row, and the tests pass while the desk goes blind" — and it is
  built so that the snapshot advances ONLY on a successful read that also RENDERED the rows. A
  cursor is precisely the mechanism that breaks that: it advances on a read, and a row the desk
  never rendered is then never shown again. Making it safe needs its own flag, its own
  reset-on-any-doubt rule, and its own row proving a row skipped by the cursor still surfaces.
  That is a brief, not a bullet in this one.

## Ground rules
- **The pool is reused, never re-implemented.** No second worker pool, no second ordering rule,
  no change to `sweep.go`'s three invariants. A diff that edits `sweepConcurrent` is out of scope
  and is a finding.
- **Fail-closed stays fail-closed.** Every converted loop that fails the whole run today still
  fails the whole run, still names the repo, and still exits non-zero. The ONE exception is
  `assessPolicyDrift`, which never failed and still never fails — and that is stated at its call
  site, not inferred.
- **Byte-identical output.** Every converted verb's stdout and JSON must be byte-identical to the
  serial version on a fixture where every repo succeeds. A change to a row, a column, an order or
  a count is a finding, not an improvement.
- **No condition is added, removed, weakened or made conditional in `deskflip`.** The reorder
  changes evaluation ORDER only. A condition that can be skipped, short-circuited, or reached
  before the read supplying its input is a finding.
- **No refusal's condition NAME changes.** Callers key on them. `checks-green` failing must still
  say `condition checks-green`, whatever position it is evaluated from.
- **`ArtifactPin`'s trailing-space prefix match is not relaxed.** The fallback is a second
  lookup with an exact name. A diff that changes the matcher is a finding.
- **The hint carries no free text.** `OfflineCheckHint` takes a tool name and a flag name and
  returns a fixed template. A variant that interpolates a body, a path or an error message is out
  of scope.
- **`--help` must not become a way past a gate.** Tier one returns before `Guard` for the
  single-token shape ONLY; it performs no work, opens nothing, and reads no configuration beyond
  the usage string compiled into the binary.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Deliverables

1. **`prs`, `stalled` and the drift probe onto the existing pool — `board.go`, `stalled.go`.**
   - `cmdPRs`: a `prsPartial` type (rows, external rows, open ids, truncated flag, unreadable
     entries) and a `sweepPRsRepo(repo, now) (prsPartial, error)` worker; `cmdPRs` becomes a
     `sweepRepos(repos, sweepConcurrency, …)` call plus a roster-order merge. The
     `outOfInstallation` carve-out becomes an `unreadable` entry on the partial.
   - `cmdStalled`: a `stalledPartial` and a `sweepStalledRepo` worker; inside it, the per-PR
     stage becomes a nested `sweepConcurrent(prs, sweepConcurrency, …)` returning a three-way
     per-PR outcome, merged in input order.
   - `assessPolicyDrift`: a `sweepRepos` call whose worker returns
     `(visibilityObservation, nil)` unconditionally; the merge builds `observed` in roster order.
   - No change to any report shape, sort, count, coverage line or audit detail.
2. **Shared root resolution + per-root concurrency — `roots.go` (new), `dispatch.go`,
   `nextup.go`.**
   - `const rootConcurrency = 4`, with a doc comment stating why it is separate from
     `sweepConcurrency` (local subprocesses, not forge reads).
   - `resolveRootsOnce() (rootSet, error)` carrying the resolved roots, the statusgen binary
     path, the pinned tag, the pin repo and the running version — performing each step exactly
     once, fail-closed, in today's order.
   - `runPerRoot[T](rs rootSet, work func(deskkit.RootConfig) (T, error)) ([]T, error)` — a thin
     `sweepConcurrent` wrapper at `rootConcurrency`, results in root order, fail-closed on the
     lowest-index root's error.
   - `cmdDispatch` and `cmdAwaiting` each call `resolveRootsOnce` then `runPerRoot`; their
     reports, populations, degraded-claim lines and fail-closed behaviour are unchanged.
3. **`throughput` from one resolution and one `actions` — `throughput.go`.**
   - `cmdDispatch` and `cmdAwaiting` each split into `…Depth(rs rootSet)` and the existing
     report-building entry point, so the verbs are unchanged and `throughput` can call the
     depth half.
   - `cmdThroughput` calls `resolveRootsOnce` ONCE, then the two depth halves, then `cmdActions`
     once (as today). `dv.Roots` is taken from the shared resolution.
   - The blind contract is unchanged and EXTENDED to the shared read: a failed shared resolution
     blinds both the dispatch and verify stages, each with a note naming it. No stage is ever
     counted as zero because a read failed.
4. **Per-platform drift pin — `main.go`.** `deskToolsPinReal` tries the bare `desk-tools` name
   first and, only on a miss, `desk-tools-<GOOS>-<GOARCH>`. The returned detail names which
   artifact line the verdict came from. `staleState`'s arms, order and could-not-check text are
   otherwise unchanged.
5. **Cost-ordered flip gate + one rollup read — `flip.go`.**
   - `flipConditions` reordered to `caller-role, app-token, pr-open-draft, mergeable,
     reviewer-approved, checks-green, model-floor, security-verdict, head-stable`, with the gate
     body moved to match and a doc comment stating the cost reasoning, the measured distribution
     behind it, and why `reviewer-approved` keeps its place ahead of `checks-green`.
   - `checksAtHeadOnce(fg, fr, head)` — a memoized RAW reader; `readChecks` and
     `checkRunsAtHeadReader` become wrappers over it, each keeping its own condition-named
     message and its own short-read reconcile.
   - `mutations.json`: entries for the reorder (moving a condition ahead of the read that feeds
     it must be CAUGHT) and for the shared read (a wrapper returning the other condition's name
     must be CAUGHT); existing entries re-anchored where this diff moves their anchor text.
6. **Refusals name the offline check — `deskkit/offlinecheck.go` (new), `deskpost`, `deskpr`,
   `deskreply`.**
   - `OfflineCheckHint(tool, flag string) string` — one fixed template, no free text.
   - `deskpost`: every body/schema refusal routed through one constructor that appends the hint
     naming `--dry-run`.
   - `deskreply`: `--dry-run` widened to the plain reply path (report what would be posted, post
     nothing); every schema refusal names it.
   - `deskpr`: a new `--check` on `create`, `update` and `edit` as specified above, plus the hint
     on every body/schema refusal.
   - `README.md`: one short section covering all three.
7. **`--help` writes no audit row — `deskkit/helprequest.go` (new) + six verbs.**
   - `HelpOnly(args []string) bool`, `ErrHelpRequested`, `IsHelpRequest(err error) bool`.
   - Retrofitted into `deskpr`, `deskwt`, `deskfile`, `desktoken`, `deskpost`, `deskreply`: the
     tier-one call at the top of `run`, and the tier-two `flag.ErrHelp` recognition in each
     subcommand's parse, with `finalize` writing no row for it. Every other verb is untouched.
8. **Changelog fragment** under `changelog/` (one `### Changed` bullet and one `### Fixed`).
9. **Nothing else.** No cursor-based delta, no new pool, no `sweep.go` change, no `statusgen`
   change, no gate-score read merged across root sets, no retrofit past the six named verbs.

## Definition of done

- `prs`, `stalled` and the drift probe run under `sweepRepos` at `sweepConcurrency`, and each
  verb's stdout and `--json` are byte-identical to the serial version on an all-succeed fixture.
- A repo that fails still fails the whole run for `prs` and `stalled`, non-zero, with that repo
  named in the message; the drift probe still never fails a run.
- `dispatch` and `awaiting` resolve roots once each through `resolveRootsOnce` and read their
  roots under `runPerRoot`; `throughput` resolves roots exactly once for the whole run.
- A stage `throughput` could not read is reported blind with a reason and excluded from
  bottleneck selection; no stage is ever rendered as a zero it did not measure.
- A pin file carrying only `desk-tools-<GOOS>-<GOARCH>` yields an in-sync (or a drift) verdict
  naming that artifact, not could-not-check; a file carrying both still uses the bare line.
- A `deskflip` run refused on a CONFLICTING PR performs exactly ONE forge read — the PR document
  — and names `condition mergeable`; a run refused on a red check performs three (the PR
  document, the reviews list, the rollup) and names `condition checks-green`; and a run that
  reaches both rollup consumers performs the rollup read exactly once across them.
- Every measured top body/schema refusal class in `deskpost`, `deskpr` and `deskreply` carries
  the offline-check hint; `deskpr --check` runs every local gate, opens no connection, and exits
  0 only when they all pass.
- `deskpr create --help` (and the equivalent in the other five retrofitted verbs) prints usage,
  exits 0, and appends no audit row.
- Measured wall-clock for `prs`, `stalled`, `actions` and `throughput` on the same repo set
  before and after, cited from the run's own output.
- `cd tools/desk && go build ./... && go vet ./...` clean, `gofmt -l` clean on every touched
  file, and `statusgen --root .. --lint` reports `LINT: PASS`.
- The brief's board row is `implemented` and the changelog fragment is present.

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 — every converted caller, the new `roots.go`, the two new deskkit files and the six retrofitted verbs compile with no call-site churn beyond the files named under `files:` |
| 2 | check:ci | `cd tools/desk && go test ./cmd/deskboard/ -run '^TestPooledSweepsFailClosedNamingTheRepo$' -count=1` | exit 0 — the SPOF row, negative control: with a stub forge in which exactly one repo errors, `prs` and `stalled` each return a non-zero exit and an error naming THAT repo, and neither returns a partial board; run once with the failing repo first in roster order and once with it last, so the lowest-index rule is exercised from both ends |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskboard/ -run '^TestPooledSweepsAreByteIdenticalToSerial$' -count=1` | exit 0 — on an all-succeed fixture, `prs`, `stalled` and the drift probe produce output byte-identical to a `limit=1` run of the same fixture, for both the rendered table and the JSON, over at least 20 repeats so a finish-order dependence cannot pass by luck |
| 4 | check:ci | `cd tools/desk && go test ./cmd/deskboard/ -run '^TestPRsOutOfInstallCarveOut$' -count=1 && go test ./cmd/deskboard/ -run '^TestPolicyDriftNeverFailsTheRun$' -count=1` | exit 0 — an out-of-installation repo still lands in the coverage `unreadable` list and does NOT fail the `prs` sweep, while any other error on the same repo still does; and a repo whose visibility read fails is absent from `observed` and reported NOT OBSERVED, with the run still exiting 0 |
| 5 | check:ci | `cd tools/desk && go test ./cmd/deskboard/ -run '^TestPerRootPoolMatchesSerialAndFailsClosed$' -count=1` | exit 0 — `dispatch` and `awaiting` under `runPerRoot` produce the same parsed rows, in the same order, as a serial run over the same roots; a root whose `statusgen` invocation fails aborts the whole verb, non-zero, naming that root; and the lowest-index root's error is the one returned when two fail |
| 6 | check:ci | `cd tools/desk && go test ./cmd/deskboard/ -run '^TestThroughputResolvesRootsExactlyOnce$' -count=1` | exit 0 — a counting stub over the root-resolution seam records exactly ONE resolution, one pin walk and one `statusgen` version probe for a whole `throughput` run, against the two-of-each a serial run of the current code records |
| 7 | check:ci | `cd tools/desk && go test ./cmd/deskboard/ -run '^TestThroughputBlindStageIsNeverACountedZero$' -count=1` | exit 0 — the blind row: with the shared root resolution forced to fail, the dispatch AND verify stages are both reported could-not-check with a reason naming the shared resolution, both are excluded from bottleneck selection, neither renders as `0`, and the bottleneck line states how many of the four stages were actually read; separately, with only the `actions` call failing, the review stage alone goes blind and the other two still report |
| 8 | check:ci | `cd tools/desk && go test ./cmd/deskboard/ -run '^TestStalePinResolvesPerPlatformArtifact$' -count=1` | exit 0 — a fixture pin file carrying ONLY `desk-tools-<GOOS>-<GOARCH>` yields in-sync or drift (never could-not-check) and the detail names that artifact; a file carrying both lines still resolves through the bare one; a file carrying only some OTHER platform's line still falls through to could-not-check; and `ArtifactPin` is called with two exact names, never with a relaxed prefix |
| 9 | check:ci | `cd tools/desk && go test ./cmd/deskflip/ -run '^TestConditionListIsTheDocumentedContract$' -count=1 && go test ./cmd/deskflip/ -count=1` | exit 0 — `flipConditions` equals the new nine-entry order exactly (the pinned list, edited deliberately, with the reason recorded on the test); and the gate's whole existing suite — every condition's own refusal test, both check-only-exemption precedence tests included — still passes from the new positions, which is the vacuous-pass control for the reorder |
| 10 | check:ci | `cd tools/desk && go test ./cmd/deskflip/ -run '^TestMergeableRefusalCostsOneForgeRead$' -count=1 && go test ./cmd/deskflip/ -run '^TestRollupIsReadOnceAcrossBothConsumers$' -count=1 && go test ./cmd/deskflip/ -run '^TestSharedRollupFailureKeepsEachConditionsOwnName$' -count=1` | exit 0 — a counting fake forge records EXACTLY one read for a flip refused as CONFLICTING (the PR document, and no rollup, reviews, timeline or file list), and the refusal names `condition mergeable`; on a run that reaches `reviewer-approved`'s check-only exemption the rollup endpoint is hit exactly once; and a forced rollup failure still produces `condition checks-green` from one consumer and `condition reviewer-approved` from the other, on both the short-read and the unreadable branch |
| 11 | check +mutation | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskboard/mutations.json && go run ./cmd/muhar -spec cmd/deskflip/mutations.json` | exit 0 — baseline GREEN, positive control CAUGHT, and every mutation CAUGHT: the pool's fail-closed rule turned into a per-repo skip; a converted worker swallowing its own error; the per-root pool's error dropped; a blind stage rendered as a counted zero; `ArtifactPin`'s trailing-space prefix relaxed to a bare prefix; the per-platform fallback made to win over the bare line; a flip condition moved ahead of the read that supplies its input; and the shared rollup wrapper made to return the other condition's name |
| 12 | check:ci | `cd tools/desk && go test -timeout 300s ./cmd/deskboard/... ./cmd/deskflip/... -count=1` | exit 0 — every EXISTING test of the two changed binaries, unchanged: `sweep_test.go`'s pool invariants, the empty-scope guard, the truncation/coverage rows and the flip gate's own suite all still hold |
| 13 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestHelpOnlyHasNoFalsePositive$' -count=1` | exit 0 — `HelpOnly` returns true for the single-token `-h` / `-help` / `--help` shapes and FALSE for every shape where the token could be a value or is not alone: `--title --help`, `--body-file --help`, `-- --help`, `--help extra`, `create --help --json`, and an empty argument list |
| 14 | check:ci | `cd tools/desk && go test ./cmd/deskpr/ -run '^TestHelpWritesNoAuditRow$' -count=1 && go test ./cmd/deskwt/ ./cmd/deskfile/ ./cmd/desktoken/ ./cmd/deskpost/ ./cmd/deskreply/ -run '^TestHelpWritesNoAuditRow$' -count=1` | exit 0 — in each of the six retrofitted verbs, a subcommand `--help` prints usage, exits 0, and leaves the audit file byte-identical (length AND bytes) to what it was before the run; a genuinely bad flag in the same position still exits 5 and still writes its row |
| 15 | check:ci | `cd tools/desk && go test ./cmd/deskpost/ ./cmd/deskpr/ ./cmd/deskreply/ -run '^TestSchemaRefusalNamesTheOfflineCheck$' -count=1` | exit 0 — the golden row: each measured top refusal class is driven through the real code path and its rendered text contains the hint naming the right flag (`--dry-run` for two verbs, `--check` for the third); and a source-level assertion that the hint template literal occurs exactly ONCE in the tree, so a second copy cannot drift from the first |
| 16 | check:ci +flow | `cd tools/desk && go test ./cmd/deskpr/ -run '^TestCheckIsOfflineAndGatesRatherThanPreviews$' -count=1` | exit 0 — `deskpr create --check` against a forge stub that FAILS every call and a git stub that refuses any push: it still exits 0 on a well-formed body (no connection attempted, nothing pushed), exits 5 with the identical refusal text on a body with no trailer, and its output names the bare-`#N` category as not checked rather than passing it silently |
| 17 | check | `cd tools/desk && gofmt -l cmd/deskboard cmd/deskflip cmd/deskpost cmd/deskpr cmd/deskreply cmd/deskwt cmd/deskfile cmd/desktoken internal/deskkit > /tmp/dt27-fmt.out; test ! -s /tmp/dt27-fmt.out` | exit 0 |
| 18 | check | `for v in prs stalled actions throughput; do /usr/bin/time -p deskboard $v > /dev/null 2> /tmp/dt27-$v.time; grep '^real' /tmp/dt27-$v.time; done` | four `real` lines, each recorded and cited in the Evidence row with the baseline it is compared against (20.10 s / 58.48 s / 70.79 s / 82.82 s, measured 2026-09-14 on a ten-repo roster and four roots). The row PASSES on the numbers being recorded, not on a threshold — a run on a different roster, a different network or a warm cache is a different measurement, and inventing a pass/fail bar for it would be a claim the instrument cannot support. A measured REGRESSION on any verb is a finding |
| 19 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |
| 20 | check:ci +dereference | `cd statusgen && go run . --root .. --consumers --brief assay:assay:desk-tools:27; echo $?` | 0 — the routing claims in `consumers:` are RESOLVED against this branch's own diff: every entry marked `fixed-here` is touched by the diff and every `out-of-scope` one is not. Exit 2 is COULD-NOT-CHECK (no diff to take — a fully merged tree), reported AS ITSELF and never as a pass |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| A concurrency conversion downgrades one repo's hard error into a partial board that reads clean | row 2 (failing repo first AND last in roster order) + row 11's fail-closed mutations |
| A converted worker swallows its own error and returns an empty partial | row 2 (the message must name the repo) + row 11 |
| Output changes — a row reordered, a count off, a coverage line dropped — because the merge is not deterministic | row 3 (byte-identical over 20 repeats) + row 12 (the existing coverage/truncation tests) |
| The out-of-installation carve-out is lost in the conversion, so a watched-but-uninstalled repo kills the board | row 4 |
| The drift probe is converted fail-closed, turning an unreadable repo's metadata into a dead `actions` | row 4 (the run must still exit 0) |
| The per-root pool drops a root's failure, so `dispatch` reports a queue missing a whole root | row 5 + row 11 |
| `throughput` still resolves roots twice and the refactor changed nothing measurable | row 6 (a counting seam, not a timing claim) |
| A stage that could not be read is rendered as a zero, steering the desk to widen the wrong loop | row 7 + row 11's blind-stage mutation |
| The shared resolution fails and only ONE of the two stages it fed is reported blind | row 7 (both stages asserted blind, each with a reason) |
| The pin fallback is implemented by loosening `ArtifactPin`'s prefix match, collapsing the bare and per-platform lines | row 8 (two exact names asserted) + row 11's relaxed-prefix mutation |
| The per-platform fallback wins over a bare line that is present, silently changing an existing consumer's verdict | row 8 (both-lines case) + row 11's precedence mutation |
| A reordered flip condition is now evaluated before the read that supplies its input, and passes vacuously | row 9 (each of the nine driven to refuse from its new position) + row 11's move-ahead-of-the-read mutation |
| A refusal's condition NAME changes with its position, breaking every caller that keys on it | rows 9 and 10 (the names asserted in the refusal text) |
| The shared rollup read returns the wrong condition's message, sending an operator to the wrong gate | row 10 (a forced failure must produce both names, from their own consumers) |
| The rollup is memoized but still read twice because one consumer bypasses the memo | row 10 (the endpoint hit exactly once) |
| `HelpOnly` matches a `--help` that was a flag's VALUE, and a real invocation silently becomes a help screen | row 13 (six negative shapes) |
| `--help` stops writing a row but a genuine bad flag stops writing one too, hiding real misuse | row 14 (the bad-flag control in the same position) |
| The hint is added in some refusals and not others, or copied and allowed to drift | row 15 (the measured classes driven through the real paths + the single-occurrence assertion) |
| `deskpr --check` opens a connection or pushes something, so the "offline" claim is false | row 16 (a forge stub that fails every call and a git stub that refuses any push) |
| `--check` passes something the real write path would refuse, making it a preview rather than a gate | row 16 (identical refusal text on the no-trailer body) |
| The whole change is neutral or a regression in wall clock | row 18 (four before/after measurements, a regression on any is a finding) |
| The cost-ordering is taken at the expense of a deliberately-recorded diagnostic rule | row 9 (the gate's existing suite, including both check-only-exemption precedence tests, must pass from the new positions) |
| A mutation spec entry is silently disarmed because this diff moved its anchor text | row 11 (baseline GREEN and positive control CAUGHT are both asserted before any mutation runs) |
| A second worker pool or a second root resolver is written instead of the existing one being reused | review-only — the reuse ladder; the absence of a new pool and the single `resolveRootsOnce` are a diff-shape check |
| The two gate-score reads are merged across root sets, silently changing `actions`' scope | review-only — the reviewer confirms `execGateScores` still reads `findRepoRoot()` and `gateScoresForRoot` still reads configured roots, and that no call site crosses them |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

| # | Exit | Key observed output |
|---|------|---------------------|
| — | — | not yet run — this brief is authored, not implemented |

## Review

Gate: model (all four risk answers no). Model-gated because both hazards are mechanically
bounded: the concurrency hazard by row 2's two-ended fail-closed control, row 3's byte-identical
comparison and row 11's fail-closed mutations, and the gate-reorder hazard by row 9 driving every
one of the nine conditions to refuse from its NEW position — which is the only thing that
distinguishes a correct reorder from one that made a condition vacuous. The reviewer confirms in
the verdict: (1) that `sweep.go` is unchanged and no second pool exists anywhere in the diff;
(2) that every converted loop that was fail-closed still is, that `assessPolicyDrift` still is
not, and that the difference is stated at the call site rather than inferred; (3) that no
`deskflip` condition was added, removed, weakened, or moved ahead of the read that supplies its
input, and that every refusal still carries its original condition NAME; (4) that the pin
fallback is a second exact-name lookup and `ArtifactPin`'s trailing-space prefix match is
untouched; and (5) that `deskpr --check` is a gate — it refuses with the write path's own text —
and not a preview that can disagree with it.
