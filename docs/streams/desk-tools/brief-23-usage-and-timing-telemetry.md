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
  so it is append-only, never rotated, and on one operating desk host it stood at
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
  - "The ledger this does NOT extend: `tools/desk/internal/deskkit/audit.go` § `Entry` / `Log` / `LoadEntries` — one JSON row per invocation, `O_APPEND` only, never truncated or rewritten, and read back whole by `LoadEntries` under the outward-write flock. The struct's fields are `ts, tool, verb, argsDigest, bodyDigest?, repo, pr, headSHA, result, detail, sourceSHA, builtAt, sessionTag, title?` — no duration, no exit code, no child count."
  - "Why the ledger cannot simply be extended or rotated: `tools/desk/internal/deskkit/ratelimit.go` counts audit rows for the per-tool write budget and the circuit breaker, and `tools/desk/internal/deskkit/idempotent.go`'s `AlreadyDoneIn` reads them for idempotency; `tools/desk/internal/deskkit/auditrecover.go` exists precisely because a plain move of the file resets both. Growth measured on one operating desk host, 2026-09-14: 108,122,055 bytes / 204,252 rows spanning ts 2026-08-13T12:09:25Z to 2026-09-14T14:30:35Z (~6,400 rows/day, ~529 bytes/row)."
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
version: 1
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
  those properties can be neither rotated nor re-shaped for a performance dataset.
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

| # | Exit | Key observed output |
|---|------|---------------------|
| — | — | not yet run — this brief is authored, not implemented |

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
