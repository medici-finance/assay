---
brief: assay:assay:desk-tools:23
title: "Opt-in local usage + timing telemetry — a per-invocation perf record with a 7-day history, and `deskperf` to read it"
why: >-
  The desk tools already record WHAT they did — `deskkit.Log` appends one JSON row per
  invocation to `~/.config/assay/audit.jsonl` — but nothing records HOW LONG it took or
  what it cost. The audit `Entry` struct carries no duration, no exit code, and no
  child-process count, so the commonest operational question a desk session has ("which
  verb is eating the window, and is it slower than it was last week?") has no answer on
  the machine that could answer it. Nor can the audit log grow one: it is load-bearing
  state — the rate-limit counter counts its rows and the idempotency store reads them —
  so it is append-only and never truncated (the daily rotation desk-tools/24 later added
  only renames a finished day aside, and every reader spans the segments), and on one operating desk host it stood at
  108,122,055 bytes across 204,252 rows after 32 days of use. Adding a duration field to
  a file with those properties is the wrong move twice: it changes a ledger whose schema
  other code depends on, and it puts an unbounded performance dataset behind a file that
  may never be pruned. This brief adds a SEPARATE, opt-in, local-only perf record written
  by the shared substrate at one place, kept for 7 UTC days and pruned on write, carrying
  a closed set of non-PII fields and no free-text field at all — plus one read verb,
  `deskperf`, that turns it into per-tool percentiles and a boot-cost breakdown. Nothing
  leaves the machine: there is no sender, no endpoint, and no network path in this brief.
wave: 2
depends: ["desk-tools/21"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-14 by a worker-desk authoring session, from an operator request recorded
  2026-09-14 and a same-day read of the audit, telemetry and subprocess-runner surfaces below
sources:
  - "Operator request, 2026-09-14: when telemetry is on, log non-PII tool usage and execution times; make the tool capable of logging it, and keep a 7-day history it can get performance information from next time."
  - "freshness-checked 2026-09-14 @ 48725dc0 (origin/main) — every fact below re-read against the source at that commit. Brief 21's deliverables are PRESENT in the tree at this commit (`tools/desk/internal/deskkit/runtool.go`, `trace.go`, `scrub.go` all exist), which is why the child-timing hook has a runner to attach to rather than needing one written here."
  - "The ledger this does NOT extend: `tools/desk/internal/deskkit/audit.go` § `Entry` / `Log` / `LoadEntries` — one JSON row per invocation, `O_APPEND` only, never truncated or rewritten, and read back whole by `LoadEntries` under the outward-write flock (re-read 2026-10-13 @ bcc02252e: since desk-tools/24, `LoadEntries` at `audit.go:367` reads every rotated segment plus the live file, in order). The struct's fields are `ts, tool, verb, argsDigest, bodyDigest?, repo, pr, headSHA, result, detail, sourceSHA, builtAt, sessionTag, title?` — no duration, no exit code, no child count."
  - "Why the ledger cannot simply be extended or moved aside: `tools/desk/internal/deskkit/ratelimit.go` counts audit rows for the per-tool write budget and the circuit breaker, and `tools/desk/internal/deskkit/idempotent.go`'s `AlreadyDoneIn` reads them for idempotency; `tools/desk/internal/deskkit/auditrecover.go` exists precisely because a plain move of the file resets both. Growth measured on one operating desk host, 2026-09-14: 108,122,055 bytes / 204,252 rows spanning ts 2026-08-13T12:09:25Z to 2026-09-14T14:30:35Z (~6,400 rows/day, ~529 bytes/row)."
  - "The promise this inherits: `docs/telemetry.md` — off by default, counts only and never content, the payload visible before anything is sent, and TWO independent switches so no inherited environment can arm it silently. `statusgen/telemetry.go` § `telemetryEnvVar` / `telemetryArmed` is the implementation: the env half is `ASSAY_TELEMETRY` compared EXACTLY to \"1\"."
  - "The leak-proofing pattern reused verbatim in spirit: `statusgen/telemetry.go` § `normalizeStatus` — a mapper whose return is ALWAYS one of its own constants, never any part of its input, so a hand-edited value cannot travel through a key. The desk-side equivalents already exist as compiled-in rosters: `tools/desk/internal/deskkit/audittoolkey.go` § `canonicalToolKeys` / `CanonicalToolKey` and `tools/desk/internal/deskkit/loopnames.go` § `CanonicalLoopName`."
  - "The subprocess seam child timing attaches to: `tools/desk/internal/deskkit/runtool.go` § `Run` / `ToolRun` — the ONE runner (brief 21), already measuring `Elapsed` and recovering `ExitCode` for every child on both the success and failure path, and already calling a single sink (`traceRecord`) on the way out."
  - "The aggregates-only day-file precedent: `tools/desk/cmd/opmetrics` § package comment and `emit.go` — a local collector that emits only counts, ratios, percentiles and fixed status codes, whose privacy posture is enforced rather than promised, and which reports could-not-check with a reason code instead of reporting zero."
  - "The registry a new binary must join or go red: `tools/desk/internal/deskkit/audittoolkey.go` § `canonicalToolKeys`, pinned against the `cmd/` directory by `audittoolkey_test.go` § `TestRegistryCoversCmdBinaries`."
  - "The platform-split precedent for a syscall-backed measurement: `tools/desk/internal/deskkit/filelock_unix.go` / `filelock_windows.go` and `custodyowner_unix.go` / `custodyowner_windows.go` — same package, one file per platform, the Windows arm honest about what it cannot do rather than faking it."
  - "The state directory and its one resolution path: `tools/desk/internal/deskkit/killswitch.go` § `deskDir` / `StateDir` — `~/.config/assay`, created 0700, with the `dirOverride` test hook every state-writing surface already uses."
exec-tier: strong
exec-tier-why: >-
  (c) this adds a NEW on-disk dataset derived from live invocations, which is the one place a
  desk tool could start recording what it has always refused to record. A field allowlist that
  is merely present passes every happy-path test; only a fixture that plants a repo, a PR
  number, a branch and an absolute path in the argv and then asserts none of them survive
  distinguishes an enforced allowlist from a documented one. The second hazard is quiet: a
  perf write that fails, blocks, or changes a verb's exit code turns an observability feature
  into an availability defect, and a green suite looks identical either way unless a row
  forces the failure.
consumers:
  - "`tools/desk/internal/deskkit/audit.go`: out-of-scope (read, NOT changed — `Entry`, `Log` and `LoadEntries` keep their exact shape and semantics; the perf writer never opens `audit.jsonl`, and Verify row 9 asserts the file is byte-identical across a verb run with the switch armed)."
  - "`tools/desk/internal/deskkit/ratelimit.go` and `idempotent.go`: out-of-scope (they READ the audit ledger, and this brief adds no row to it and removes none, so neither can observe a change)."
  - "`tools/desk/internal/deskkit/runtool.go`, `forge_github.go`, `forge_gitlab.go`, `audittoolkey.go` and the seven first-wave verb `main()`s: fixed-here (each a bounded additive change named under `files:` — two counter lines at the existing runner sink, one counting transport per backend, one registry line, and a one-line wrap per verb)."
  - "Every OTHER desk verb's `main()`: out-of-scope (a verb that is not wrapped records nothing and behaves exactly as it does today, so the retrofit is additive and per-verb; widening it past the first wave is untracked follow-up, not a gap this brief leaves open)."
  - "A remote telemetry sink for the perf record: out-of-scope (nothing here opens a socket, Verify row 10 asserts the absence by grep, and standing up a receiver is the Console-side work `docs/telemetry.md` § 'No receiver yet' already tracks; only the ON-DISK shape is fixed here so a later sender reads it unchanged)."
version: 2
id: 28b33fa3-d433-4318-a57e-2e269c8d5f82
---

# Brief 23 — Opt-in local usage + timing telemetry, and `deskperf`

## Dependencies
desk-tools/21 (`DESK_TRACE` and cause-carrying errors). Its deliverable
`tools/desk/internal/deskkit/runtool.go` is the ONE subprocess runner, and it is where the
per-child timing this brief aggregates is already measured (`ToolRun.Elapsed`,
`ToolRun.ExitCode`). This brief adds a counter at that existing sink; it does not write a
second runner and does not duplicate the measurement. Brief 21's deliverables are present in
the tree at `48725dc0`, so the dependency is satisfied at authoring time.

## Context

single-point-of-failure: **the perf record's field allowlist — a Go struct that carries no
free-text field at all, built by one constructor that accepts only typed, fixed-vocabulary
values.** Behind it are three layers, each tripping on a different signal in a different
component: (1) the OPT-IN switch — with `ASSAY_TELEMETRY` unset nothing is written, no
directory is created and no install id is minted, so a user who never asked has no file for a
bad record to land in (row 3); (2) LOCAL-ONLY by construction — this brief compiles in no
endpoint, no sender and no HTTP import in any file it adds, so a record that should not have
been written still never leaves the machine (row 10, an absence-grep); (3) the VOCABULARY
mappers — `tool`, `verb`, `role` and `result` each pass through a compiled-in roster
(`CanonicalToolKey`, the verb's own registered subcommand set, `CanonicalLoopName`, the audit
result constants) whose return is always one of its own constants and never any part of its
input, exactly as `statusgen`'s `normalizeStatus` already works (row 2). The three are
genuinely independent: a hole in the allowlist is caught by the mappers only for the four
mapped fields, by the switch only for a user who never opted in, and by the absence of a
network path always — which is why the record carries no field capable of holding a path, an
argv, a repo, or a body.

risk note — all four risk answers are `no`, and the change adds a disclosure-shaped surface.
The answers stand because the feature is OFF by default behind the same environment switch
`docs/telemetry.md` already governs, because nothing it writes can leave the machine in this
brief, and because the record has no free-text field to carry content in the first place. A
reviewer who finds a `string` field on the record that accepts caller-supplied text, a mapper
that can return its input, or any network path in the added files flips `sensitive-data` to
yes and takes the human gate.

The `risk-files-crossread` lint NOTICE fires here and is answered rather than ignored: the
declared paths sit under `tools/desk/internal/deskkit/`, which is a security-path trigger for
this repo, and all four risk answers are `no`. The answers stand because every change inside
that directory is ADDITIVE and touches no control: the new files are a writer and a reader
for a file no control consults; `runtool.go` gains two counter increments and no behavioural
change; `audittoolkey.go` gains one roster entry, which TIGHTENS the registry rather than
relaxing it (an unregistered binary is still refused); and the two forge files gain a
transport that counts and delegates, inspecting nothing. No trust gate, no budget, no
scan, no exit code and no refusal is read or written by any of it — rows 9, 12 and 14 are
the mechanical evidence for that, and a reviewer who finds otherwise flips the answer.

files:
- `tools/desk/internal/deskkit/perf.go` (planned) (new) — `PerfRecord`, `PerfEnabled`,
  `PerfRun`, `PerfStep`, `perfAppend`, `perfPrune`, `perfDir`, `perfInstallID`, the
  `result`-from-exit mapper, and the child/forge counters' accessors.
- `perfcpu_unix.go` (planned) and `perfcpu_windows.go` (planned), both new in the same
  package — the one platform-split call, modeled on `filelock_unix.go`/`filelock_windows.go`.
- `perfread.go` (planned) (new, same package) — `LoadPerfDays`, `PerfSummary`,
  `SummarisePerf`, `SummariseBootSteps` (the pure aggregation the read verb renders).
- `perf_test.go` (planned), `perfread_test.go` (planned), `perfnopii_test.go` (planned) — all
  new in the same package.
- `perf-mutations.json` (planned) (new, same package) — the `muhar` spec for row 13.
- `tools/desk/internal/deskkit/runtool.go` (existing) — `Run` increments the child counters
  alongside its existing `traceRecord(r)` call. No other change.
- `tools/desk/internal/deskkit/forge_github.go` and `forge_gitlab.go` (existing) — the
  counting `http.RoundTripper` installed in `restClient()` / `client()`, so `forgeCalls` is
  counted once per backend at the one place each already selects its transport.
- `tools/desk/internal/deskkit/audittoolkey.go` (existing) — one line: `"deskperf": {}` in
  `canonicalToolKeys`, without which `TestRegistryCoversCmdBinaries` goes red.
- `tools/desk/cmd/deskperf/` (planned) (new) — `main.go`, `report.go`, `boot.go`, and their
  tests.
- `tools/desk/cmd/deskboot/` (existing) — `boot.go` gains one `PerfStep` call per step.
- First-wave `main()` wraps, one line each, under `tools/desk/cmd/`: `deskpr`, `deskpost`,
  `deskboard`, `desktoken`, `deskwt`, `deskdispatch`, `deskboot`.
- `tools/desk/README.md` (existing) — a "Local performance history — `deskperf`" section.
- `docs/telemetry.md` (existing) — one short subsection recording that desk-tools carries a
  LOCAL-only perf log under the same promise and the same environment switch, and that the
  two-switch rule binds any future remote send.
- `changelog/desk-tools-23-usage-telemetry.md` (planned) (new).

facts (read at `48725dc0`, 2026-09-14):

- **The audit log records the act, not the cost.** `deskkit.Entry` (audit.go) is
  `ts, tool, verb, argsDigest, bodyDigest?, repo, pr, headSHA, result, detail, sourceSHA,
  builtAt, sessionTag, title?`. There is no duration field, no exit-code field and no
  child-process field, and `Log` fills only `TS`, `Tool`, `SessionTag`, `SourceSHA` and
  `BuiltAt` when unset. Timing has never been recorded anywhere in the suite.
- **The audit log cannot become the perf log.** It is load-bearing state twice over —
  `ratelimit.go` counts its rows per canonical tool key for the write budget and the circuit
  breaker, and `idempotent.go`'s `AlreadyDoneIn` reads them for idempotency — which is why
  `Log` is `O_APPEND` only and why `auditrecover.go` exists at all (its doc comment states
  that a plain move of the file resets the counter and the idempotency store). A file with
  those properties cannot be re-shaped for a performance dataset, nor pruned like one.
  **Correction, re-read 2026-10-13 @ `bcc02252e` (origin/main), recorded rather than
  dropped:** the ledger IS now rotated. desk-tools/24 landed daily rotation: `Log`
  (`tools/desk/internal/deskkit/audit.go:301`) calls `rotateIfNeeded` (`audit.go:173`),
  which renames `audit.jsonl` to `audit.jsonl.<YYYY-MM-DD>[.N]` on the first append of a new
  UTC day (`rotationDue`, `audit.go:154`; the exact segment-name match is `audit.go:101`).
  It deletes nothing, and every reader spans the segments (`segmentPaths`, `audit.go:123`;
  `LoadEntries`, `audit.go:367`), so the counter and the idempotency store still see every
  row ever written — the rows are still load-bearing and still never pruned. The conclusion
  is unchanged: a rename that keeps every row is not a retention policy, and the perf
  history still needs its own 7-day-pruned files under `perf/`, a path neither the rotation
  nor the segment match can name. Row 9 now also asserts a rotated segment is untouched.
- **Agent usage is not this record's job.** The perf record times desk CLI verbs on the
  CLIENT side — wall, CPU, child processes, forge round-trips. Per-invocation agent usage
  (tokens, cost) belongs to the runner contract — `loopadmin/runner/usage.go`'s
  `runner.Usage`, verified present at `bcc02252e` — and to the planned execution journal of
  graph-execution/26. This record neither reads nor duplicates those figures, and no
  `PerfRecord` field may carry them.
- **Its growth is real and unbounded.** Measured on one operating desk host on 2026-09-14:
  108,122,055 bytes across 204,252 rows, first row ts `2026-08-13T12:09:25Z`, last
  `2026-09-14T14:30:35Z` — 32 days, ~6,400 rows/day, ~529 bytes/row. The largest single
  contributor is `desktoken` at 148,670 rows (72.8% of the file).
  **Correction to the dispatch note, recorded rather than dropped** (worker-kit clause 7):
  that note put the `desktoken` share at ~94%; the measured share is 72.8%. The conclusion the
  number supports — that the ledger is dominated by high-frequency, low-value rows and grows
  without bound — is unchanged, but the figure is corrected here rather than repeated.
- **The double-switch contract already exists and already has an environment half.**
  `docs/telemetry.md` states the four promises (off by default, counts only and never content,
  the payload visible before anything is sent, two independent switches). `statusgen/telemetry.go`
  implements the env half as `const telemetryEnvVar = "ASSAY_TELEMETRY"` with
  `telemetryArmed(flagSet) = flagSet && os.Getenv(telemetryEnvVar) == "1"` — an EXACT `"1"`
  comparison, not a truthiness test. That exactness is deliberate and is reused verbatim here,
  so the two tools cannot disagree about whether a user opted in.
- **The mapper pattern that makes "never content" enforceable already exists on both sides.**
  `statusgen`'s `normalizeStatus` returns one of its own constants or `"none"`/`"other"`, never
  the input. The desk side already has two compiled-in rosters of the same shape:
  `CanonicalToolKey`/`canonicalToolKeys` (audittoolkey.go — 52 binaries plus the synthetic
  keys, refusing an unregistered key loudly rather than granting it a private bucket) and
  `CanonicalLoopName(raw) (canonical string, known bool)` (loopnames.go). A perf record's
  `tool` and `role` therefore need no new vocabulary — they need the two that exist.
- **The one subprocess runner is already measuring what the child cost.** `deskkit.Run`
  (runtool.go) populates `ToolRun.Elapsed` and `ToolRun.ExitCode` on BOTH the success and the
  failure path, and already ends with a single call to `traceRecord(r)`. Child timing attaches
  there, at the existing sink, and nowhere else.
- **The trace ledger is not the perf log and cannot be reused as one.** `traceRecord` returns
  immediately when `TraceEnabled()` is false (trace.go), the ledger is capped at 200 entries,
  and it lives in memory for the life of one process. It exists to diagnose one failing run;
  it has no history, no retention and no cross-invocation reading. The perf counters this brief
  adds are therefore separate from it, and are incremented unconditionally (two atomic adds per
  child) rather than behind the trace switch — see the Ground rules for why that is still off
  by default in every observable sense.
- **Every verb's `main()` has the same shape**, which is what makes a one-line wrap possible:
  `SetToolClass(...)`, `EchoEffectiveConfig(os.Stderr)`, `CheckVerbActivation(...)`, then
  `os.Exit(run(os.Args[1:]))` (verified in `tools/desk/cmd/deskpr/main.go`). Note that `os.Exit` runs no
  deferred function — which is exactly why the perf record must be written by a wrapper AROUND
  `run`, returning the exit code, and never by a `defer` inside it.
- **A new binary must join the canonical roster or the suite goes red.**
  `audittoolkey_test.go`'s `TestRegistryCoversCmdBinaries` diffs `canonicalToolKeys` against
  the `cmd/` directory. `deskperf` is a new directory there, so the registry line is part of
  the deliverable, not an afterthought.
- **Both forge backends route through a transport this package selects.** GitHub's
  `restClient()` chooses `http.DefaultTransport` unless `g.Client.Transport` is set and passes
  it to go-gh; GitLab's `client()` passes `g.Client` to `gitlab.WithHTTPClient` when non-nil
  and otherwise lets the library build its own. A counting `RoundTripper` therefore installs
  cleanly in the GitHub arm, and in the GitLab arm needs the client to be constructed when the
  caller supplied none — a three-line change at one place per backend, and the only way to make
  `forgeCalls` mean the same thing on both.
- **The state directory has one resolution path.** `deskDir()`/`StateDir()` (killswitch.go)
  return `~/.config/assay`, create it 0700, and carry the `dirOverride` hook every state test
  in the package already uses. The perf files go under a `perf/` SUBDIRECTORY of it, so no
  glob, prune or listing in this brief can ever name `audit.jsonl`.
- **No platform-portable CPU-time call exists in the tree yet** (`grep -rn Getrusage
  tools/desk` is empty at this commit), and the package's established answer to a
  platform-specific syscall is one file per platform in the same package —
  `filelock_unix.go`/`filelock_windows.go`, `custodyowner_unix.go`/`custodyowner_windows.go`.

facts — the design:

- **The record.** `PerfRecord` is written as one JSON line and carries exactly these fields
  and no others:

  | Field | Type | Source |
  |-------|------|--------|
  | `schema` | string | the constant `perf-v1` |
  | `ts` | string | RFC3339 UTC, second resolution |
  | `tool` | string | `CanonicalToolKey` of the compiled-in tool constant — a roster member or nothing |
  | `verb` | string | the subcommand, mapped through the verb's own registered set; anything else becomes `other`; a boot step is `step:<name>` from `deskboot`'s step constants |
  | `role` | string | `CanonicalLoopName($DESK_LOOP)`; unset or unknown becomes `none` |
  | `wallMs` | int | measured by the wrapper around `run` |
  | `cpuMs` | int, OMITTED when unmeasurable | user+sys for this process |
  | `exit` | int | the exit code `run` returned |
  | `result` | string | derived from `exit` by a fixed map, never read from the audit log |
  | `childProcs` | int | children started through `deskkit.Run` |
  | `childWallMs` | int | summed `ToolRun.Elapsed` of those children |
  | `forgeCalls` | int | HTTP round-trips through the counting transport |
  | `sourceSHA` | string | `deskkit.Version()`'s build stamp |
  | `install` | string | 32 hex chars from `crypto/rand`, generated once, host-anonymous |

- **What the record does NOT carry, enumerated so the absence is a contract and not an
  oversight:** no repo or owner, no PR or issue number, no branch or ref, no head SHA, no file
  path or directory, no argv and no `argsDigest`, no body and no `bodyDigest`, no `detail` or
  any other free-text field, no title, no session tag, no user name, no host name, no
  environment variable value, no forge URL or endpoint, no token or credential of any shape.
  The struct has no `string` field beyond the six enumerated above, each of which is either a
  constant, a roster member, a build stamp, or random hex — so there is no field for content
  to travel in even if a caller tried.
- **`install` is an identifier of the INSTALL, not of a person or a machine.** It is 16 random
  bytes from `crypto/rand`, written once to `<perf dir>/install-id` mode 0600 and read back
  thereafter. It is never derived from a hostname, user name, MAC address, home directory or
  any other machine property — a derived id would be a fingerprint, and this one is a coin
  flip. Deleting the file simply mints a new one; nothing depends on its stability.
- **Where it lives.** `~/.config/assay/perf/YYYY-MM-DD.jsonl`, one file per UTC day, appended
  with `O_APPEND|O_CREATE|O_WRONLY` mode 0600 in a directory created 0700. The `perf/`
  subdirectory is what keeps every operation in this brief structurally incapable of naming
  `audit.jsonl`: the writer joins `perfDir()`, the pruner lists only `perfDir()`, and the
  reader globs only `perfDir()`.
- **Retention: 7 UTC days, pruned on write.** On the first append of a process, the pruner
  lists `perfDir()`, matches file names against `^\d{4}-\d{2}-\d{2}\.jsonl$` EXACTLY, parses
  the date out of the matched name, and removes those older than `today-6` (so today plus the
  six prior days survive — seven files). A name that does not match the pattern is left alone,
  always: `install-id` is not a day file and neither is anything a future change adds. Two
  processes pruning at once is benign — the loser's `os.Remove` returns `ENOENT` and is
  ignored. Steady-state size, from the measured ~6,400 invocations/day above at ~230 bytes per
  record: ≈ 1.5 MB/day, ≈ 10 MB for the whole history.
- **Switch: the environment half only, and only for the local file.** `PerfEnabled()` is
  `os.Getenv("ASSAY_TELEMETRY") == "1"` — the identical exact comparison
  `statusgen/telemetry.go` makes, so one opt-in governs both tools. There is deliberately NO
  second flag for the local log, and the reasoning is stated in the code and the docs rather
  than left implicit: `docs/telemetry.md`'s two-switch rule exists so that no inherited
  environment can cause a SEND, and this log sends nothing — it is a file on the user's own
  disk that only they can read. What the single switch still buys is consent: a user who never
  asked for telemetry gets no new directory, no new file and no new identifier. The two-switch
  rule is restated in the doc as binding on any future remote sink, which is the follow-up this
  brief does not attempt.
- **Best-effort, never load-bearing.** `perfAppend` returns no error to its caller. A failure
  to resolve the directory, create it, open the file or write the line is swallowed: the perf
  log never changes a verb's exit code, never emits a line on stdout or stderr, and never
  blocks. It takes no lock — a record is a single `write(2)` of one line far under `PIPE_BUF`,
  which `O_APPEND` makes atomic on the platforms these tools target, and the file is read only
  by `deskperf`, which tolerates a truncated final line by skipping it and SAYING SO in its
  output rather than silently dropping it.
- **The read verb is `deskperf`, not a `deskaudit` subcommand.** `deskaudit` is the tool of the
  LEDGER: its contract is that no row is ever lost, its recovery verb exists to preserve rows
  a plain move would destroy, and its file is read by the budget and idempotency paths. The
  perf history is the opposite in all three respects — opt-in, deliberately pruned, and read by
  nobody but a human or a desk. Putting a prunable dataset behind a verb whose whole promise is
  "never loses a row" would make one tool assert two contradictory contracts, which is the
  defect class `audittoolkey.go` already documents from the other direction. A separate binary
  also keeps the registry honest: `deskperf` earns its own canonical key and its own budget
  bucket rather than borrowing `deskaudit`'s.
- **`deskperf` surface.**
  `deskperf report [--days N] [--tool X] [--role R] [--top N] [--json]` prints, per tool: the
  invocation count, p50 / p90 / max wall ms, total wall ms, that tool's share of total wall as
  a percentage, and its refusal ratio (`refused` + `unwritten` + `unverifiable` over count);
  then the top-N slowest individual invocations (default 10) as `ts / tool / verb / wallMs`
  and nothing else. `--days` defaults to 7 and is CAPPED at 7, because there is no eighth day
  to read and a `--days 30` that silently reported 7 would be a confident wrong answer.
  `deskperf boot [--days N] [--role R] [--json]` reads only the `step:*` records and prints
  per-step p50/p90/max and the summed boot cost, per role. `--json` emits the same aggregates
  as a single object for a desk to consume.
- **An empty history refuses loudly.** A missing perf directory, a directory with no day files,
  or a filter that matches no record prints a reason-NAMED line — "no perf history: local perf
  logging is off (ASSAY_TELEMETRY is not 1)" versus "perf history present but no record matches
  --tool X" — and exits `ExitUnverifiable` for the first case. It must never print the same
  all-zeroes shape a genuinely idle-but-armed history would print: "nothing was slow" and "we
  were never looking" are different facts, and `opmetrics`' three-state rule is the house
  answer to exactly this.
- **`cpuMs` is omitted, never zeroed, when it cannot be measured.** The unix arm reads
  `syscall.Getrusage(RUSAGE_SELF)` and sums user+sys; the Windows arm returns
  `(0, false)` and the field is omitted from the JSON. A zero would read as "this ran in no CPU
  time at all", which is a measurement claim the Windows build cannot make.

## Ground rules
- **No free-text field, ever.** A new field on `PerfRecord` that is a `string` not backed by a
  constant, a compiled-in roster, a build stamp or random hex is out of scope and is a finding.
  Adding any field at all bumps `schema` to `perf-v2`, updates the table in `docs/telemetry.md`,
  and adds a case to the no-PII test — all three, in the same change.
- **The audit ledger is not touched.** No new row, no changed row, no new field on `Entry`, no
  read of `audit.jsonl` from any perf code path. `audit.go`, `ratelimit.go`, `idempotent.go`
  and `auditrecover.go` are read-only references in this brief.
- **Nothing leaves the machine.** No endpoint, no sender, no HTTP client, no `net/*` import in
  any file this brief adds. The remote sink is named as follow-up and implemented nowhere.
- **The perf write never changes what the verb does.** Not its exit code, not its stdout, not
  its stderr, not its timing beyond the budget below. A perf failure is silent by design.
- **Off by default means nothing is created.** With `ASSAY_TELEMETRY` unset there is no
  directory, no day file and no install id — not an empty file, not a directory waiting to be
  filled.
- **The pruner only ever removes an exact day-file name it matched.** No glob, no wildcard
  removal, no directory removal, no path outside `perfDir()`.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Deliverables

1. **The substrate — `tools/desk/internal/deskkit/perf.go` (planned).**
   - `PerfRecord` with exactly the fields tabled above, every one `omitempty`-free except
     `cpuMs` (a pointer or an `omitempty` int, so "unmeasured" and "zero" stay distinguishable).
   - `PerfEnabled() bool` → `os.Getenv("ASSAY_TELEMETRY") == "1"`, with a doc comment naming
     `statusgen/telemetry.go`'s `telemetryArmed` as the surface it must not drift from.
   - `PerfRun(tool string, verbs []string, args []string, run func([]string) int) int` — the
     ONE wrapper: starts the clock and the counters, calls `run(args)`, builds the record
     (resolving `verb` by matching `args[0]` against `verbs`, the verb's own registered
     subcommand set, and falling back to `other`), appends it best-effort, and returns `run`'s
     exit code UNCHANGED. When `PerfEnabled()` is false it calls `run` and returns, doing
     nothing else at all.
   - `PerfStep(tool, step string, d time.Duration, exit int)` — the boot-step form, writing a
     record whose `verb` is `"step:" + step`.
   - `perfDir()` = `filepath.Join(StateDir(), "perf")`; `perfAppend(PerfRecord)` returning
     nothing; `perfPrune()` implementing the exact-name 7-day rule; `perfInstallID()` minting
     or reading `<perfDir>/install-id`.
   - `resultForExit(int) string` — the fixed map onto the existing `Result*` constants
     (`ExitOK`→`ResultOK`, `ExitDisabled`→`ResultDisabled`, `ExitRefused`→`ResultRefused`,
     `ExitUnverifiable`→`ResultUnverifiable`, anything else → `ResultUnverifiable`). It returns
     one of its own constants, never its input.
   - Two atomic counters with accessors, reset by `PerfRun` at entry: children started and
     their summed wall time; forge round-trips.
2. **Platform split — `perfcpu_unix.go` / `perfcpu_windows.go`.** One function,
   `processCPUMillis() (int64, bool)`. Unix: `syscall.Getrusage(syscall.RUSAGE_SELF)`, user+sys
   in milliseconds, `true`. Windows: `(0, false)`. Build tags mirroring `filelock_*.go`.
3. **Child counting — `runtool.go`.** `Run` increments the two child counters immediately
   beside its existing `traceRecord(r)` call, using `r.Elapsed`. Two lines; no other change to
   the runner, no second measurement, no new field on `ToolRun`.
4. **Forge counting — `forge_github.go`, `forge_gitlab.go`.** One counting
   `http.RoundTripper` in `deskkit`, installed where each backend already selects its transport:
   wrapping the chosen transport in `GitHubForge.restClient()`, and wrapping (constructing, when
   the caller supplied none) the `*http.Client` `GitLabForge.client()` hands to
   `gitlab.WithHTTPClient`. It counts and delegates — it never inspects, buffers, logs or
   modifies a request or a response.
5. **Reading — `perfread.go`.** `LoadPerfDays(days int) ([]PerfRecord, int, error)` returning
   the records, the count of unparseable lines skipped, and an error only for a genuinely
   unreadable directory; `SummarisePerf` and `SummariseBootSteps` as PURE functions over a
   record slice (no I/O), so the aggregation is testable from a fixture directory alone.
6. **The verb — `tools/desk/cmd/deskperf/`.** `report` and `boot` as specified above, `--json`
   on both, `--version`, `--help`, the loud empty-history refusal, and the skipped-line count
   reported rather than swallowed. Exit codes follow the deskkit contract.
7. **Registry — `audittoolkey.go`.** `"deskperf": {}` in `canonicalToolKeys`.
8. **Boot steps — `tools/desk/cmd/deskboot/boot.go`.** One `PerfStep` call per step, using the existing
   `step*` name constants, so `deskperf boot` reports the same step names the boot output does.
9. **First-wave `main()` wraps.** Seven verbs (listed under `files:`), each a single-line
   change from `os.Exit(run(os.Args[1:]))` to
   `os.Exit(deskkit.PerfRun("<tool>", verbs, os.Args[1:], run))`. Every other verb is untouched
   and records nothing.
10. **Docs.** A `tools/desk/README.md` section covering what is recorded, what is not, where it
    lives, the 7-day retention, and the two `deskperf` subcommands; and a `docs/telemetry.md`
    subsection recording the local log under the same promise, with the field table and the
    explicit statement that the two-switch rule binds any future remote sink.
11. **Changelog fragment** under `changelog/` (one `### Added` bullet).
12. **Nothing else.** No sender, no endpoint, no audit-log change, no second runner, no
    retrofit beyond the seven named verbs, no new field carrying free text.

## Definition of done

- `ASSAY_TELEMETRY=1` + any wrapped verb writes exactly one record to
  `~/.config/assay/perf/<UTC date>.jsonl`; with the variable unset or any other value, the
  `perf/` directory does not exist after the run.
- The record contains none of the excluded fields, proven against a fixture whose argv carries
  a repo slug, a PR number, a branch name and an absolute path.
- `audit.jsonl` is byte-identical before and after a wrapped verb runs with the switch armed.
- Day files older than 7 UTC days are gone after any write; `install-id` and every
  non-day-file name survive.
- `deskperf report` and `deskperf boot` reproduce known aggregates from a committed fixture
  directory, and both refuse loudly on an empty history.
- The per-record append costs under 1 ms at p50 and a forced write failure leaves the wrapped
  verb's exit code unchanged.
- `go build ./...`, `go vet ./...`, `gofmt -l` clean, and `statusgen --root .. --lint` reports
  `LINT: PASS`.
- The brief's board row is `implemented` and the changelog fragment is present.

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 — the new package, the new binary, and the seven one-line wraps compile with no call-site churn beyond them |
| 2 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPerfRecordGoldenShape$' -count=1` | exit 0 — one record marshals to the exact `perf-v1` key set and no other key; `tool`, `verb`, `role` and `result` each come back as a roster/constant value when fed an unregistered input (an unknown tool, an unknown subcommand, `DESK_LOOP=not-a-loop`, an exit code outside the contract), never as the input itself |
| 3 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPerfOffByDefaultCreatesNothing$' -count=1` | exit 0 — with `ASSAY_TELEMETRY` unset, and separately set to `0`, `true` and the empty string, a wrapped run creates no `perf/` directory, no day file and no install id; `PerfRun` still returns the wrapped exit code |
| 4 | check +mutation | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPerfRecordCarriesNoIdentifiers$' -count=1` | exit 0 — the SPOF row: a wrapped run whose argv carries a repo slug, a PR number, a branch name and an absolute path, with `DESK_SESSION` and a token-shaped variable in the environment, produces a record whose serialized bytes contain none of those six planted sentinels. Mutation: adding any one planted value to the record REDDENS this test |
| 5 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPerfPruneKeepsSevenDaysAndOnlyDayFiles$' -count=1` | exit 0 — a fixture of 12 day files plus `install-id` plus a decoy name: after one append, exactly 7 day files remain (today and the six prior), the day-8 file is gone, and `install-id` and the decoy are untouched |
| 6 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPerfWriteFailureNeverChangesExitCode$' -count=1` | exit 0 — the NEGATIVE control: with the perf directory forced unwritable, a wrapped verb returns the identical exit code and writes nothing to stdout or stderr that it would not have written with perf disabled |
| 7 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestSummarisePerfReproducesFixtureAggregates$' -count=1 && go test ./cmd/deskperf/ -run '^TestReportAndBootRenderFixtureDirectory$' -count=1` | exit 0 — p50/p90/max/total/share/refusal-ratio computed from a committed fixture match hand-checked values, and `deskperf report` / `deskperf boot` render them; an empty history refuses with a reason-named line rather than printing zeroes |
| 8 | check:ci | `cd tools/desk && go test ./cmd/deskboot/ -run '^TestBootWritesOneStepRecordPerStep$' -count=1` | exit 0 — a booted run with the switch armed writes one record per step, `verb` spelled `step:` plus the step's own name constant, and `deskperf boot` groups them by role |
| 9 | check:ci +flow | `cd tools/desk && go test ./internal/deskkit/ -run '^TestPerfNeverTouchesTheAuditLedger$' -count=1 && go test ./cmd/deskperf/ -run '^TestWrappedVerbToDeskperfEndToEnd$' -count=1` | exit 0 — the cross-component path end to end: a wrapped verb runs with the switch armed, writes its record through the substrate, and `deskperf report` reads that same record back out of the day file; in the same run `audit.jsonl`'s bytes and mtime are identical before and after, and no perf code path opens it |
| 10 | check:ci | `cd tools/desk && ( ! grep -rn -e 'net/http' -e 'net/url' -e 'http\.' internal/deskkit/perf.go internal/deskkit/perfread.go internal/deskkit/perfcpu_unix.go internal/deskkit/perfcpu_windows.go cmd/deskperf/ )` | exit 0 — no network path anywhere in the added files: local-only holds by absence, not by promise |
| 11 | check:ci | `cd tools/desk && go test -bench '^BenchmarkPerfAppend$' -benchtime 2000x -run '^$' ./internal/deskkit/ > /tmp/dt23-bench.out; grep -q 'BenchmarkPerfAppend' /tmp/dt23-bench.out && go test ./internal/deskkit/ -run '^TestPerfAppendMedianUnderOneMillisecond$' -count=1` | exit 0 — the benchmark runs, and the companion test writes 1000 records to a temp dir and GATES on the measured median being under 1 ms |
| 12 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRegistryCoversCmdBinaries$' -count=1 && go test ./internal/deskkit/ -run '^TestCanonicalToolKey$' -count=1` | exit 0 — `deskperf` is registered, so the new binary has its own canonical key and its own budget bucket rather than an unregistered one |
| 13 | check +mutation | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/perf-mutations.json` | exit 0 — baseline GREEN, positive control CAUGHT, and every mutation CAUGHT: the env check loosened from exact `"1"` to a truthiness test, the field allowlist widened by one planted value, the pruner's exact-name match relaxed to a prefix match, the retention window widened past 7 days, `resultForExit` made to return its input, and the perf write made to propagate its error into the verb's exit code |
| 14 | check:ci | `cd tools/desk && go test -timeout 300s ./internal/deskkit/... ./cmd/deskperf/... ./cmd/deskboot/... ./cmd/deskpr/... -count=1` | exit 0 — every existing test of the touched packages, unchanged |
| 15 | check:ci | `cd tools/desk && gofmt -l internal/deskkit/perf.go internal/deskkit/perfread.go internal/deskkit/perfcpu_unix.go internal/deskkit/perfcpu_windows.go internal/deskkit/runtool.go internal/deskkit/audittoolkey.go cmd/deskperf cmd/deskboot > /tmp/dt23-fmt.out; test ! -s /tmp/dt23-fmt.out` | exit 0 |
| 16 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |
| 17 | check:ci +dereference | `cd statusgen && go run . --root .. --consumers --brief assay:assay:desk-tools:23; echo $?` | 0 — the routing claims in `consumers:` are RESOLVED against this branch's own diff, not counted: every entry marked `fixed-here` is touched by the diff and every `out-of-scope` one is not. Exit 2 is COULD-NOT-CHECK (no diff to take — a fully merged tree), which is reported AS ITSELF and never as a pass |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| A repo, PR, branch, path or session id reaches the record through a field nobody audited | row 4 (six sentinels planted in argv and environment at once) |
| An unregistered tool name, subcommand or `DESK_LOOP` value travels verbatim through a mapped field | row 2 (each mapper fed an unregistered input) |
| The feature is on for a user who never opted in, because the env check accepts `true`/`yes`/any non-empty value | row 3 (the four spellings) + row 13 (the loosening mutation) |
| The perf log quietly becomes load-bearing — a write failure changes an exit code, or the append blocks | rows 6 and 11, and row 13's error-propagation mutation |
| History grows without bound because the pruner never fires or the window drifts | rows 5 and 13 (the widened-window mutation) |
| The pruner removes something that is not a day file | row 5 (`install-id` and a decoy name asserted intact) |
| The audit ledger is changed, re-shaped or read by the perf path | row 9 + row 14 (the ledger's own tests unchanged) |
| A remote sink is added "while we are here" | row 10 (absence-grep over every added file) |
| An empty history prints all-zeroes, indistinguishable from an armed-but-idle one | row 7 (the reason-named refusal) |
| The aggregates are wrong but plausible — a p90 computed off-by-one, a share that does not sum | row 7 (hand-checked fixture values) |
| The new binary writes audit rows under an unregistered key and escapes the budget | row 12 |
| `cpuMs` reports 0 on a platform that cannot measure it, and a reader treats it as "used no CPU" | row 2 (the golden shape asserts the field is OMITTED, not zeroed, when unmeasurable) |
| A second subprocess runner or a second timing measurement is written instead of the existing sink being used | review-only — the reuse ladder; `Run`'s two added lines and the absence of a new runner are a diff-shape check |
| The GitLab arm silently under-counts `forgeCalls` because its client was never constructed | review-only — the reviewer confirms both backends install the counting transport, since a GitLab round-trip cannot be exercised offline here |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

### Verifier pass — 2026-10-15, assay-verifier-app[bot] (verify-desk, non-implementer), merged main 9f2b7f886430

Delivering change: medici-finance/assay#2853, squash-merged as b2050d8c735a (single parent 86575b055da6), the commit on main carrying the `Brief: desk-tools/23` trailer. The brief itself was authored in medici-finance/assay#1038 (5610bdea178a). No perf source file changed between b2050d8c735a and the verified source 9f2b7f886430. Host: darwin/arm64, go1.27.2, git 2.56.0. Environment for every row: KUBECONFIG=/dev/null, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly, HOME and XDG_CONFIG_HOME pointed at an empty scratch directory outside the source tree, and ASSAY_TELEMETRY, DESK_LOOP and DESK_SESSION unset, so no row could reach the network or write into a real config directory. Each row was run as its own command from the stated directory; the Command cell names the Verify row whose command was run character for character.

Grounding, written from the brief before any diff or test was read: a perf-v1 line has exactly 14 keys (cpuMs omitted when unmeasurable); the switch is ASSAY_TELEMETRY compared to "1" exactly; files live under the state directory's perf subdirectory as YYYY-MM-DD.jsonl, mode 0600 in a 0700 directory; retention keeps today plus six prior UTC days; the exit map sends anything outside the contract to unverifiable; an empty history refuses with exit 6 and names the switch; median append under 1 ms; seven wrapped verbs; no network import in the added perf files; row 17 on a fully merged tree is expected to report COULD-NOT-CHECK (exit 2) as itself.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | Verify row 1, verbatim | exit 0 | exit 0; build and vet of every package under tools/desk print nothing | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 2 | Verify row 2, verbatim | exit 0 | exit 0; `ok  github.com/medici-finance/assay/tools/desk/internal/deskkit 0.229s`; with -v the golden-shape test reports `--- PASS` | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 3 | Verify row 3, verbatim | exit 0 | exit 0; `ok` for deskkit; with -v the four subtests unset, zero, true and empty all report PASS | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 4 | Verify row 4, verbatim | exit 0 | exit 0; `ok` for deskkit; with -v the no-identifiers test reports `--- PASS`. Mutation half: row 13 planted argv text into the verb field and DESK_SESSION into the role field, and both were CAUGHT | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 5 | Verify row 5, verbatim | exit 0 | exit 0; `ok` for deskkit; with -v the seven-day prune test reports `--- PASS` | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 6 | Verify row 6, verbatim | exit 0 | exit 0; `ok` for deskkit; with -v the write-failure exit-code test reports `--- PASS` | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 7 | Verify row 7, verbatim | exit 0 | exit 0; `ok` for deskkit then `ok  github.com/medici-finance/assay/tools/desk/cmd/deskperf 2.905s`. Hand recompute from the committed fixture by nearest rank, P_k = element ceil(k/100 x n): deskpr walls 100, 200, 300, 1000 give p50 200 and p90 1000, deskpost walls 50, 150, 250 give p50 150 and p90 250, which match the values the rendered table prints. Black-box: a built deskperf binary, run with an empty scratch HOME, printed `deskperf: no perf history: local perf logging is off (ASSAY_TELEMETRY is not 1)` and exited 6, and created no directory | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 8 | Verify row 8, verbatim | exit 0 | exit 0; `ok  github.com/medici-finance/assay/tools/desk/cmd/deskboot 0.640s`; with -v the one-record-per-step test reports `--- PASS` | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 9 | Verify row 9, verbatim | exit 0 | exit 0; `ok` for deskkit (audit-ledger test) then `ok  github.com/medici-finance/assay/tools/desk/cmd/deskperf 0.359s` (end-to-end test). Diff check: the delivering change modifies none of audit.go, ratelimit.go, idempotent.go or auditrecover.go (empty diffstat over 86575b055da6..b2050d8c735a) | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 10 | Verify row 10, verbatim | exit 0 | exit 0; grep printed no line and all five named paths exist (so the negated grep did not pass on a missing file). The counting transport that does import net/http lives in the separate forge transport file, which the brief names as the forge-counting deliverable; it counts and delegates | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 11 | Verify row 11, verbatim | exit 0 | exit 0; benchmark output `BenchmarkPerfAppend-16  2000  136284 ns/op`; companion gate test `ok`, its -v log reads `perf append over 1000 records: p50 156.708µs, p90 237.209µs, max 826.083µs` | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 12 | Verify row 12, verbatim | exit 0 | exit 0; `ok` for deskkit twice (registry-covers-cmd test, then canonical-tool-key test); the registry diff adds exactly one line, `"deskperf": {}` | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 13 | Verify row 13, verbatim | exit 0 | exit 0; `Harness healthy: baseline GREEN, positive control CAUGHT.` then CAUGHT for all seven mutations (env check loosened to truthiness; argv planted in verb; DESK_SESSION planted in role; pruner regex relaxed to a prefix match; retention widened to 8; resultForExit returns an encoding of its input; perf write error propagated to the exit code); `Totals: 7 caught, 0 NOT CAUGHT, 0 could-not-mutate.` Source tree byte-clean afterwards (git status empty) | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 14 | Verify row 14, verbatim | exit 0 | exit 0; `ok` for deskkit (187.550s), deskkit/untrustcorpus, cmd/deskperf, cmd/deskboot (14.878s) and cmd/deskpr (122.100s) | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 15 | Verify row 15, verbatim | exit 0 | exit 0; the gofmt output file is 0 bytes | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 16 | Verify row 16, verbatim | 0 | prints `0`; last lint line `LINT: PASS`; no PROBLEM line, NOTICE lines only, none naming desk-tools/23 | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 17 | Verify row 17, verbatim, plus the corroboration form the tool itself names | 0; exit 2 is COULD-NOT-CHECK, reported as itself | On merged main the row's own command reports COULD-NOT-CHECK, as its Expect anticipates: `statusgen: --consumers: COULD-NOT-CHECK: assay:assay:desk-tools:23 is not in the diff against 9f2b7f886430...` and the program exits 2 (go run relays `exit status 2` and the echo prints 1). Corroboration form, in a throwaway clone outside the source tree checked out at b2050d8c735a with `--consumers --brief assay:assay:desk-tools:23 --base 86575b055da6`: exit 0, `summary: 0 corroborated, 0 disproved, 5 unchecked`, every entry marked unchecked because the consumers block was written by the authoring change (medici-finance/assay#1038), not the delivering one. The verifier made the call the tool leaves to the reviewer, using the delivering diff's file list: every fixed-here entry is touched (runtool.go, forge_github.go, forge_gitlab.go, audittoolkey.go, and the main.go of deskpr, deskpost, deskboard, desktoken, deskwt, deskdispatch, deskboot); no out-of-scope entry is touched (audit.go, ratelimit.go, idempotent.go unchanged; no other cmd main.go changed; no sender added, see row 10). All five claims hold | 2026-10-15 | assay-verifier-app[bot] @ 9f2b7f886430 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |

Review-section checks, from the source at 9f2b7f886430: (1) PerfRecord in perf.go has 14 fields, of which eight are strings (schema, ts, tool, verb, role, result, sourceSHA, install) and the rest are integers (wallMs, cpuMs, exit, childProcs, childWallMs, forgeCalls). Each string is set only from a constant, a roster mapper (tool, verb, role), the exit map, a build stamp held to the hex pattern `^[0-9a-f]{7,40}$` or else "unpinned", or crypto/rand hex. No field takes caller text, so the excluded-field list holds against the struct. (2) PerfEnabled compares ASSAY_TELEMETRY to "1" exactly (perf.go line 90), the same comparison statusgen/telemetry.go line 65 makes. (3) Every perf path is joined under the perf subdirectory of the state directory, and the pruner removes only regular files whose name matches the anchored day pattern, so it cannot name audit.jsonl. (4) The empty-history path is a reason-named refusal with exit 6 (row 7 black-box). Both forge backends install the counting transport: the GitHub REST client wraps its chosen transport, and the GitLab client is handed a copied client whose transport counts, built fresh when the caller supplied none. The runner gains a single counter call beside its existing trace sink, and no second runner exists. Observation, not a defect: the seven wraps sit after the verb-activation check, so an invocation refused at activation exits before the wrapper and records nothing.

Risk-bearing value. The item carries risk metadata (all four answers no, irreversible no), and the diff touches tools/desk/internal/deskkit, which this repo treats as a security-path trigger, so the procedure applies. Enumerated over the delivering diff's added and changed source (perf.go, perfread.go, perfcpu files, forge transport, deskperf, deskboot) and the Deliverables: PerfSchema = "perf-v1" @ tools/desk/internal/deskkit/perf.go:45; perfEnvVar = "ASSAY_TELEMETRY" @ perf.go:48 compared `== "1"` @ perf.go:90; perfRetentionDays = 7 @ perf.go:51 with cutoff `AddDate(0, 0, -(perfRetentionDays - 1))` @ perf.go:385; perfDayFile = `^(\d{4}-\d{2}-\d{2})\.jsonl$` @ perf.go:282; directory mode 0o700 @ perf.go:310; day-file mode 0o600 @ perf.go:325; install-id mode 0o600 @ perf.go:406 and perf.go:415; install-id size `[16]byte` @ perf.go:401 with installIDPattern `^[0-9a-f]{32}$` @ perf.go:389; shaPattern `^[0-9a-f]{7,40}$` @ perf.go:259; resultForExit default = ResultUnverifiable @ perf.go:254; fallback constants "other", "none", "step:", "unpinned" @ perf.go:55-58; PerfMaxDays = perfRetentionDays @ tools/desk/internal/deskkit/perfread.go:20; percentile ranks 50 and 90 @ perfread.go:202-203 with `math.Ceil(k / 100 * float64(n))` @ perfread.go:318; report --top default 10 @ tools/desk/cmd/deskperf/report.go:27; median gate `time.Millisecond` @ tools/desk/internal/deskkit/perf_test.go:421. Ranked by irreversibility: nothing leaves the machine, so no value can cause an irreversible disclosure. The one irreversible act in reach is the pruner's deletion of files, so the day-file pattern ranks first. Retention, the opt-in comparison and the file modes come next, as consent and local-privacy values. The percentile ranks, the --top default, the hex patterns and the 1 ms gate are reversible reporting or operational knobs and rank last.

- RISK-VALUE: DERIVED — perfDayFile = `^(\d{4}-\d{2}-\d{2})\.jsonl$` @ tools/desk/internal/deskkit/perf.go:282 — anchored at both ends, it matches only the name the writer itself produces (`Format("2006-01-02") + ".jsonl"` @ perf.go:324). The pruner lists only the perf directory, acts only on regular files (perf.go:366), and deletes only when the date parsed from the matched name is before the cutoff, so a deletion can only ever hit a file this writer could have made. Row 13's prefix-relaxation mutation was CAUGHT.
- RISK-VALUE: DERIVED — perfRetentionDays = 7 @ tools/desk/internal/deskkit/perf.go:51 — the operator request asks for a 7-day history. The cutoff is today minus (7 - 1) = 6 days at UTC midnight (perf.go:385) and files with `day.Before(cutoff)` are removed (perf.go:373), so today plus the six prior days survive: 7 files, as the brief specifies. The reader cap PerfMaxDays is the same constant (perfread.go:20), so the reader cannot drift from the writer. Row 13's widen-to-8 mutation was CAUGHT.
- RISK-VALUE: DERIVED — perfEnvVar = "ASSAY_TELEMETRY" compared `== "1"` @ tools/desk/internal/deskkit/perf.go:48 and perf.go:90 — character-identical to statusgen's telemetryEnvVar (statusgen/telemetry.go:46) and its exact `== "1"` check (statusgen/telemetry.go:65), as the brief requires so one opt-in cannot mean two things. Row 3 rejected "0", "true" and the empty string, and row 13's truthiness mutation was CAUGHT.
- RISK-VALUE: DERIVED — directory mode 0o700 @ tools/desk/internal/deskkit/perf.go:310, file modes 0o600 @ perf.go:325, perf.go:406 and perf.go:415 — owner-only, the same 0o700 the state directory is created with (tools/desk/internal/deskkit/killswitch.go:256) and the modes the brief's Where-it-lives fact specifies. The data is non-identifying, so a looser mode would not be a disclosure, and the modes are fixed by an edit.
- RISK-VALUE: DERIVED — install-id size `[16]byte` @ tools/desk/internal/deskkit/perf.go:401 — 16 bytes from crypto/rand give the 32 hex characters the brief tables (pattern @ perf.go:389). That is 128 random bits, never derived from a host or user property, so it identifies an install and is not a fingerprint. Collisions do not matter because nothing depends on the id being stable or unique.

Verification-Attestation: medici-finance/assay#2969 run=dfeb1337871281fdac2103449d81e422055878ea39036f1e97aeccabdc6c4913 source=9f2b7f886430b9cb9bdf45bde2189a6bb7fe6245 brief=docs/streams/desk-tools/brief-23-usage-and-timing-telemetry.md model=claude-opus-5-5 tier=any

**VERIFY: PASS** — 17 of 17 rows met their Expect at 9f2b7f886430. Row 17's own command reports COULD-NOT-CHECK on the merged tree, as its Expect anticipates; it was discharged by the tool's named corroboration form (exit 0) plus the verifier's dereference of all five consumers claims against the delivering diff.

### Verifier pass — 2026-10-18, assay-verifier-app[bot] (verify-desk, non-implementer), merged main 6dfb616ef546

Re-verification at a newer merged main. The delivering change is still medici-finance/assay#2853 (b2050d8c735a, single parent 86575b055da6). Since the 2026-10-15 pass at 9f2b7f886430, later changes reshaped the perf surface: medici-finance/assay#3113 moved the perf-v1 writer, its day-file layout, retention, install id and per-OS CPU readers out of deskkit into the shared stdlib-only perfrec module (perfrec/perfrec.go, perfrec/cpu_unix.go, perfrec/cpu_windows.go), so the two deskkit perfcpu files the Verify table names no longer exist; medici-finance/assay#3051, #3111 and #3332 widened instrumentation past the first seven verbs and refined verb attribution. deskkit's perf.go now delegates to perfrec. The perf mutation spec follows the move: five of its eight mutation targets are perfrec/perfrec.go. Host: darwin/arm64, go1.27.2, git 2.56.0. Environment for every row: KUBECONFIG=/dev/null, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly, ASSAY_TELEMETRY, DESK_LOOP and DESK_SESSION unset, and HOME and XDG_CONFIG_HOME pointed at an empty scratch directory outside the source tree. The one exception is row 16, re-run with the operator HOME because statusgen's lint needs the roster file to run its trust-dependent checks (with the empty HOME it printed `LINT: REFUSED configuration` and exited 5, which is an environment refusal, not a lint result). Each row ran as its own command from the home's root; the Command cell names the Verify row whose command was run character for character. Every named test was re-run with -v to confirm it exists and reports `--- PASS`, which discharges the lint's gotest-run-vacuous NOTICEs for this pass.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | Verify row 1, verbatim | exit 0 | exit 0; build and vet of every package under tools/desk print nothing | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 2 | Verify row 2, verbatim | exit 0 | exit 0; `ok  github.com/medici-finance/assay/tools/desk/internal/deskkit 0.313s`; with -v `--- PASS: TestPerfRecordGoldenShape` | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 3 | Verify row 3, verbatim | exit 0 | exit 0; `ok` for deskkit (0.281s); with -v `--- PASS: TestPerfOffByDefaultCreatesNothing` | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 4 | Verify row 4, verbatim | exit 0 | exit 0; `ok` for deskkit (0.244s); with -v `--- PASS: TestPerfRecordCarriesNoIdentifiers`. Mutation half: row 13 planted the argv into the verb field and DESK_SESSION into the role field, and both were CAUGHT | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 5 | Verify row 5, verbatim | exit 0 | exit 0; `ok` for deskkit (0.240s); with -v `--- PASS: TestPerfPruneKeepsSevenDaysAndOnlyDayFiles` | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 6 | Verify row 6, verbatim | exit 0 | exit 0; `ok` for deskkit (0.259s); with -v `--- PASS: TestPerfWriteFailureNeverChangesExitCode` | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 7 | Verify row 7, verbatim | exit 0 | exit 0; `ok` for deskkit (0.310s) then `ok  github.com/medici-finance/assay/tools/desk/cmd/deskperf 0.135s`; with -v `--- PASS: TestSummarisePerfReproducesFixtureAggregates` and `--- PASS: TestReportAndBootRenderFixtureDirectory` | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 8 | Verify row 8, verbatim | exit 0 | exit 0; `ok  github.com/medici-finance/assay/tools/desk/cmd/deskboot 0.286s`; with -v `--- PASS: TestBootWritesOneStepRecordPerStep` | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 9 | Verify row 9, verbatim | exit 0 | exit 0; `ok` for deskkit (0.262s) then `ok` for cmd/deskperf (0.184s); with -v `--- PASS: TestPerfNeverTouchesTheAuditLedger` and `--- PASS: TestWrappedVerbToDeskperfEndToEnd`. Diff check: the delivering change modifies none of audit.go, ratelimit.go, idempotent.go or auditrecover.go (empty name list over 86575b055da6..b2050d8c735a). Grep of perf.go, perfread.go, perfrec/perfrec.go and the perfrec CPU readers finds no audit.jsonl open and no LoadEntries call, only two comments | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 10 | Verify row 10, verbatim, plus the same grep over the files the writer moved to | exit 0 | Row command: exit 0 and no match line, but grep also printed `No such file or directory` for the two deskkit perfcpu files, which medici-finance/assay#3113 removed. grep exits 2 on a missing file, and the negation turns any nonzero exit into 0, so at this commit the row's exit code alone would read 0 even if a match existed. Supplementary, same pattern plus `"net"`, over perfrec/perfrec.go, perfrec/cpu_unix.go and perfrec/cpu_windows.go: grep exit 1, no match. The suite's own guards over the moved writer pass: `--- PASS: TestPerfrecStaysLocalAndOffLedger` and `--- PASS: TestPerfrecClosureCarriesNothingOnline` (a go list -deps closure check on linux, darwin and windows). No network path in the added or moved perf files | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 11 | Verify row 11, verbatim | exit 0 | exit 0; benchmark output `BenchmarkPerfAppend-16  2000  516863 ns/op`; companion gate test `ok` (0.619s); its -v log reads `perf append over 1000 records: p50 231.458µs, p90 596.125µs, max 17.940625ms` and `--- PASS: TestPerfAppendMedianUnderOneMillisecond` | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 12 | Verify row 12, verbatim | exit 0 | exit 0; `ok` for deskkit twice (0.279s, 0.246s); with -v `--- PASS: TestRegistryCoversCmdBinaries` and `--- PASS: TestCanonicalToolKey` | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 13 | Verify row 13, verbatim | exit 0 | exit 0; `Harness healthy: baseline GREEN, positive control CAUGHT.` then CAUGHT for all seven mutations (switch loosened to truthiness; argv planted in verb; DESK_SESSION planted in role; pruner match relaxed to a prefix match; retention widened past 7 days; resultForExit returns an encoding of its input; perf write error propagated to the exit code); `Totals: 7 caught, 0 NOT CAUGHT, 0 could-not-mutate.` The home's git status was empty afterwards | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 14 | Verify row 14, verbatim | exit 0 | exit 0; `ok` for deskkit (262.731s), deskkit/untrustcorpus (0.108s), cmd/deskperf (0.154s), cmd/deskboot (4.059s) and cmd/deskpr (83.512s); no FAIL line | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 15 | Verify row 15, verbatim, plus gofmt over the files the writer moved to | exit 0 | Row command: exit 0, and the gofmt output file is 0 bytes. gofmt printed `lstat ... no such file or directory` on stderr for the two removed deskkit perfcpu files, so for those two names the row checked nothing. Supplementary: `gofmt -l perfrec` plus perf.go, perfread.go and forgetransport.go printed nothing (exit 0) | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 16 | Verify row 16, verbatim (operator HOME, see above) | 0 | prints `0`; last lint line `LINT: PASS`; 0 PROBLEM lines; NOTICE lines only. The NOTICEs naming this brief are gotest-run-vacuous on rows 2 to 6, 11 and 12 (discharged here by the -v runs) , verify-row-portability on rows 11 and 15, and the risk-files-crossread NOTICE that the brief's Context answers | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |
| 17 | Verify row 17, verbatim, plus the corroboration form the tool itself names | 0; exit 2 is COULD-NOT-CHECK, reported as itself | On merged main the row's own command reports COULD-NOT-CHECK, as its Expect anticipates: `statusgen: --consumers: COULD-NOT-CHECK: assay:assay:desk-tools:23 is not in the diff against 6dfb616ef546...`, the program exits 2 (go run relays `exit status 2`, the echo prints 1). Corroboration form, in a throwaway clone outside the home checked out at b2050d8c735a, with `--consumers --brief assay:assay:desk-tools:23 --base 86575b055da6`: exit 0, `summary: 0 corroborated, 0 disproved, 5 unchecked`, every entry unchecked because the authoring change (medici-finance/assay#1038) wrote the consumers block, not the delivering one. The verifier made the call the tool leaves to the reviewer, from the delivering diff's file list: every fixed-here entry is touched (runtool.go, forge_github.go, forge_gitlab.go, audittoolkey.go, and the main.go of deskpr, deskpost, deskboard, desktoken, deskwt, deskdispatch and deskboot); no out-of-scope entry is touched (audit.go, ratelimit.go, idempotent.go unchanged; no other existing cmd main.go changed; no sender added, see row 10). All five claims hold for this brief's diff. The later widening of instrumentation to other verbs (medici-finance/assay#3051) is separate tracked work, not this diff | 2026-10-18 | assay-verifier-app[bot] @ 6dfb616ef546 (on-behalf-of human:ian) (claude-opus-5-5 verifier, verify-desk) |

Review-section checks, from the source at 6dfb616ef546: (1) the record struct, now perfrec.Record (perfrec/perfrec.go:73-87), has the 14 tabled fields: eight strings (schema, ts, tool, verb, role, result, sourceSHA, install) and six integers, with cpuMs a pointer marked omitempty. Each string is set only from a constant, a roster mapper, the fixed exit map, a build stamp held to `^[0-9a-f]{7,40}$` (perfrec/perfrec.go:147), or crypto/rand hex. No field takes caller text, so the excluded-field list holds against the struct. (2) PerfEnabled (tools/desk/internal/deskkit/perf.go:65) returns perfrec.Enabled, which compares ASSAY_TELEMETRY to "1" exactly (perfrec/perfrec.go:40 and :99), the same comparison statusgen/telemetry.go makes (lines 46 and 65). (3) Every perf path joins the perf subdirectory of the state directory (perfrec/perfrec.go:113), and the pruner lists only that directory, acts only on regular files whose name matches the anchored day pattern, so it cannot name audit.jsonl or a rotated segment. (4) The empty-history refusal is exercised by row 7's render test. Observation for the desk, not a VERIFY failure: rows 10 and 15 name two files that no longer exist since medici-finance/assay#3113, so at this commit those rows' exit codes no longer discriminate for those names. The substance of both rows was checked here against the files the writer moved to. The Verify table should be re-pointed at perfrec's files in a follow-up.

Risk-bearing value. The item carries risk metadata (all four answers no, irreversible no), and the touched paths sit under tools/desk/internal/deskkit, a security-path trigger here, so the procedure applies. Enumerated over the perf source at this commit (perfrec/perfrec.go, perfrec/cpu_unix.go, perfrec/cpu_windows.go, tools/desk/internal/deskkit/perf.go, tools/desk/internal/deskkit/perfread.go) and the Deliverables: Schema = "perf-v1" @ perfrec/perfrec.go:37 (re-exported as PerfSchema @ tools/desk/internal/deskkit/perf.go:44); EnvVar = "ASSAY_TELEMETRY" @ perfrec/perfrec.go:40 compared `== "1"` @ perfrec/perfrec.go:99; RetentionDays = 7 @ perfrec/perfrec.go:43 with cutoff `AddDate(0, 0, -(RetentionDays - 1))` @ perfrec/perfrec.go:195 and `day.Before(cutoff)` @ perfrec/perfrec.go:217; dayFile = `^(\d{4}-\d{2}-\d{2})\.jsonl$` @ perfrec/perfrec.go:176; directory mode 0o700 @ perfrec/perfrec.go:313; day-file mode 0o600 @ perfrec/perfrec.go:328; install-id mode 0o600 @ perfrec/perfrec.go:243 and :252; install-id size `[16]byte` @ perfrec/perfrec.go:238 with InstallIDPattern `^[0-9a-f]{32}$` @ perfrec/perfrec.go:226; ShaPattern `^[0-9a-f]{7,40}$` @ perfrec/perfrec.go:147; PerfMaxDays = perfRetentionDays @ tools/desk/internal/deskkit/perfread.go:20 with the reader window `AddDate(0, 0, -(days - 1))` @ perfread.go:68; median gate 1 ms in the companion test. Ranked by irreversibility: nothing leaves the machine, so no value can cause an irreversible disclosure. The one irreversible act in reach is the pruner's file deletion, so the day-file pattern and the retention cutoff rank first. The opt-in comparison and the file modes come next, as consent and local-privacy values. The hex patterns, the reader window and the 1 ms gate are reversible reporting or operational knobs and rank last.

- RISK-VALUE: DERIVED — dayFile = `^(\d{4}-\d{2}-\d{2})\.jsonl$` @ perfrec/perfrec.go:176 — anchored at both ends, it matches only the name the writer itself produces (`Format("2006-01-02") + ".jsonl"` @ perfrec/perfrec.go:327). Prune lists only the perf directory, skips anything that is not a regular file, and removes only when the date parsed from a matched name is before the cutoff (perfrec/perfrec.go:202-222), so a deletion can only hit a file this writer could have made. Row 13's prefix-relaxation mutation was CAUGHT.
- RISK-VALUE: DERIVED — RetentionDays = 7 @ perfrec/perfrec.go:43 — the operator request asks for a 7-day history. The cutoff is UTC midnight today minus (7 - 1) = 6 days (perfrec/perfrec.go:195), and files strictly before it are removed (perfrec/perfrec.go:217), so today plus the six prior days survive: 7 files, as specified. The reader cap PerfMaxDays is the same constant (tools/desk/internal/deskkit/perfread.go:20), so reader and writer cannot drift. Row 5 asserted exactly 7 survivors, and row 13's widened-window mutation was CAUGHT.
- RISK-VALUE: DERIVED — EnvVar = "ASSAY_TELEMETRY" compared `== "1"` @ perfrec/perfrec.go:40 and :99 — character-identical to statusgen's telemetryEnvVar (statusgen/telemetry.go:46) and its exact check (statusgen/telemetry.go:65), as the brief requires so one opt-in cannot mean two things. Row 3 rejected "0", "true" and the empty string, and row 13's truthiness mutation was CAUGHT.
- RISK-VALUE: DERIVED — directory mode 0o700 @ perfrec/perfrec.go:313, file modes 0o600 @ perfrec/perfrec.go:328, :243 and :252 — owner-only, matching the 0o700 the package's other state writers create their directories with (for example tools/desk/internal/deskkit/killswitch.go:349) and the modes the brief's Where-it-lives fact specifies. The data is non-identifying and an edit fixes a mode, so this is reversible.
- RISK-VALUE: DERIVED — install-id size `[16]byte` @ perfrec/perfrec.go:238 — 16 bytes from crypto/rand encode to the 32 hex characters the brief tables (pattern @ perfrec/perfrec.go:226). That is 128 random bits, never derived from a host or user property, so it names an install and is not a fingerprint; nothing depends on it being stable.

Verification-Attestation: medici-finance/assay#3395 run=48698b4d22fd2d2c3b69a1344e04fe43d1f9058adbbf22406252450e8f861ae1 source=6dfb616ef5463fa886e89ab7b07803d30006c58a brief=docs/streams/desk-tools/brief-23-usage-and-timing-telemetry.md model=claude-opus-5-5 tier=strong

**VERIFY: PASS** — 17 of 17 rows met their Expect at 6dfb616ef546. Row 17's own command reports COULD-NOT-CHECK on the merged tree, as its Expect anticipates; the tool's named corroboration form (exit 0) plus the verifier's check of all five consumers claims against the delivering diff discharged it. Rows 10 and 15 name two files removed by medici-finance/assay#3113; their substance was re-checked against the moved files, and the stale names are an observation for a Verify-table follow-up.

## Review

Gate: model (all four risk answers no). Model-gated because the two hazards are both
mechanically bounded: the disclosure hazard by row 4's six planted sentinels, row 2's mapper
controls and row 10's absence of any network path, and the availability hazard by rows 6 and 11
plus row 13's error-propagation mutation. The reviewer confirms in the verdict: (1) that
`PerfRecord` carries no `string` field capable of holding caller-supplied text, and that the
excluded-field list in the Context is complete against the struct as implemented; (2) that
`PerfEnabled` compares `ASSAY_TELEMETRY` to `"1"` EXACTLY, matching `statusgen/telemetry.go`,
so one opt-in cannot mean two things; (3) that no perf code path opens, reads or writes
`audit.jsonl`, and that the pruner cannot name it; and (4) that the empty-history path is a real
reason-named refusal rather than a quiet zero report.
