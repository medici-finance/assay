---
brief: assay:assay:desk-tools:24
title: "Audit ledger — bounded tail read in `Guard`, no `desktoken` cache-reuse rows, daily rotation, and a `deskaudit tail` read verb"
why: >-
  Every desk verb's mandatory first call is `deskkit.Guard()`, and `Guard` ends by asking one
  question — was the LAST audit line a `disabled` line? — by parsing the ENTIRE audit ledger
  into a slice and looking at the final element. On a host one month into normal desk use that
  ledger is roughly 200k rows and 105 MB, so the question costs ~0.6 s of CPU on every
  invocation of every verb, read-only verbs included; measured against that ledger,
  `deskversion` (which does nothing but print a build stamp) has a p50 of 0.59 s while
  `deskack --help`, which returns before `Guard`, is instant. A WRITE verb pays a full parse
  three times over — `Guard`, the outward-write flow, and the rate-limiter's own read — two of
  them inside the audit flock, so concurrent writers serialise behind each other. At ~800–1,100
  rows/hour the suite spends on the order of 10 CPU-minutes per hour per host re-parsing a file
  to read its last line, and the cost grows linearly and without bound because the ledger is
  never rotated. This brief makes the three reads proportional to the ANSWER instead of to the
  file — a bounded tail read for the last entry, a bounded reverse window read for the meters
  that provably falls back to the full parse when its answer is not yet determined — stops
  `desktoken` from appending a row for a cache reuse that performed no act (the single largest
  contributor of rows), rotates the ledger into daily segments so the file every append and
  every tail read touches stays one day long, and adds the read verb the ledger has never had.
  No meter's verdict changes, no row is deleted, and no control is relaxed.
wave: 2
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1035]
schema: brief-v2
authored: >-
  2026-09-14 by a worker-desk authoring session, from issue #1035 and a same-day read of the
  kill-switch, audit, rate-limit, recovery and token surfaces below
sources:
  - "Issue #1035 — the measurement this brief acts on: `deskkit.Guard()` parses the whole `audit.jsonl` on every verb to read its last line; ~0.59-0.72 s per standalone replay of `LoadEntries` against a ~105 MB / ~200k-row ledger, `deskversion` p50 0.59 s against the same, `desktoken` 0.81 s wall of which ~0.64 s user CPU, 800-1,100 rows/hour, three full parses on a write path."
  - "freshness-checked 2026-09-14 @ e428134c (origin/main) — every fact below re-read against the source at that commit."
  - "The guard and its one question: `tools/desk/internal/deskkit/killswitch.go` § `Guard` / `guard` / `lastResultWas` — the mandatory first call of every desk tool; its final step is a disarm-transition check that calls `LoadEntries()` and reads `entries[len(entries)-1].Result`."
  - "The ledger: `tools/desk/internal/deskkit/audit.go` § `Entry` / `Log` / `LoadEntries` / `FirstTS` — one JSON line per invocation, `O_APPEND|O_CREATE|O_WRONLY` 0600 in a 0700 directory, never truncated or rewritten; `LoadEntries` scans every line with a 4 MiB line cap and REFUSES (Unverifiable, exit 6) on any malformed line rather than skipping it."
  - "The meters that read it: `tools/desk/internal/deskkit/ratelimit.go` § `pointsFor` / `chargedInWindow` / `breakerRun` / `checkBreaker` / `checkBreakerBackstop` — `rateWindow` is one hour, `BreakerTrip` is 5, `BreakerCooldown` is 15 minutes, `BreakerBackstopTrip` is the tool-wide twin; and `tools/desk/internal/deskkit/idempotent.go` § `AlreadyDoneIn` / `AlreadyDone`."
  - "The outward-write flow: `tools/desk/cmd/deskpost/writeflow.go` § `attemptOutward` — holds the audit flock across `LoadEntries` → `AllowWriteAt` → `plan` → append, and hands the loaded slice to `plan`, which is where `deskkit.AlreadyDoneIn`, `alreadyCommented` (comment.go) and `reviewAlreadyPostedIn` (review.go) consume it."
  - "Why a plain file move is not rotation: `tools/desk/internal/deskkit/auditrecover.go` § package comment and `RecoverCorruptAudit` — the counter and the idempotency store are pure functions of the surviving entries, so moving the whole file aside returns every budget to full and makes the store forget every prior write. Recovery quarantines the LINE and carries every good entry forward, under `lockAudit`, keeping raw lines byte-for-byte so nothing is reserialised."
  - "The row that records a no-op: `tools/desk/cmd/desktoken/desktoken.go` § `auditCtx` / `finalize` / `cmdToken` — one deferred audit line per invocation, and the cache-reuse path (`age < cacheMaxAge`, 50 minutes) sets `ac.detail = \"reused cached …\"` and returns nil, so the deferred `finalize(nil)` writes an `ok` row for an invocation that contacted nothing and changed nothing."
  - "The maintenance verb and its contract: `tools/desk/cmd/deskaudit/main.go` — `recover` is its only verb today; its package comment states the ledger is load-bearing state and that no row is ever lost."
  - "The state directory and its one resolution path: `tools/desk/internal/deskkit/killswitch.go` § `deskDir` / `StateDir` — `~/.config/assay`, created 0700, with the `dirOverride` test hook every state-writing surface in the package already uses, deliberately not wired to any env var because the kill switch lives in that directory."
  - "The canonical-key registry a new verb name must satisfy: `tools/desk/internal/deskkit/audittoolkey.go` § `canonicalToolKeys`, pinned against `cmd/` by `audittoolkey_test.go` § `TestRegistryCoversCmdBinaries`. `deskaudit` is already registered — this brief adds a subcommand, not a binary."
  - "The mutation-gate precedent in this package: `tools/desk/internal/deskkit/trace-mutations.json` and its siblings, run by `tools/desk/cmd/muhar` with a baseline, a positive control, and one entry per control the change must not lose."
exec-tier: strong
exec-tier-why: >-
  (b) the change is a performance rewrite of the code path that every kill-switch decision, every
  write budget and every idempotency decision runs through, and the failure mode of a bounded read
  is silent and one-directional: a reader that stops too early returns a SMALLER history, and a
  smaller history means a lower charged count, a shorter consecutive-refusal run, and a
  "not already done" answer — every one of which reads as PERMISSION. A green suite looks
  identical whether the bounded reader is exactly equivalent to the full parse or merely equivalent
  on the fixtures someone happened to write, which is why the equivalence rows below run BOTH
  readers over the same planted ledger and compare the meters' verdicts rather than their counts.
  The second hazard is the rotation: the ledger is load-bearing state whose own recovery verb
  exists because a plain move of the file resets both meters, so a rotation whose readers do not
  cross the segment boundary is that same reset on a daily timer.
consumers:
  - "`tools/desk/internal/deskkit/killswitch.go`: fixed-here (`lastResultWas` switches from `LoadEntries` to the new bounded `LastEntry`; `Guard`'s precedence order, its refusals and its audit writes are untouched)."
  - "`tools/desk/internal/deskkit/audit.go`: fixed-here (`LastEntry`, the reverse segment reader, segment enumeration and the rotation hook are added; `Entry`'s fields, `Log`'s append semantics and `LoadEntries`' whole-ledger contract and refusal behaviour are unchanged, the last now reading across segments)."
  - "`tools/desk/internal/deskkit/ratelimit.go`: fixed-here (`pointsFor` gains the bounded reverse read with its determinacy stop and its fail-closed fallback; every tier function, every constant and every refusal string is untouched)."
  - "`tools/desk/internal/deskkit/auditrecover.go`: fixed-here (`RecoverCorruptAudit` reads and repairs across segments; the quarantine-the-line-not-the-file principle, the flock and the byte-for-byte carry-forward are unchanged)."
  - "`tools/desk/cmd/desktoken/desktoken.go`: fixed-here (the cache-reuse success path suppresses its audit row; every mint, every refusal and every failure still records exactly one row)."
  - "`tools/desk/cmd/deskaudit/main.go`: fixed-here (the `tail` verb is added alongside `recover`)."
  - "`tools/desk/internal/deskkit/idempotent.go`: out-of-scope (read, NOT changed — `AlreadyDoneIn` stays a pure predicate over whatever slice it is handed, and this brief hands it the same whole-ledger slice it gets today)."
  - "`tools/desk/cmd/deskpost/writeflow.go`: out-of-scope (read, NOT changed — see the Dependencies note: its `LoadEntries` feeds the idempotency predicates, which are whole-ledger by contract, so it keeps the full parse. What changes for it is that the two OTHER parses on its path are gone)."
  - "A retention or pruning policy for rotated segments: out-of-scope (no segment is deleted by anything in this brief; the ledger's contract is that no row is lost, and choosing to discard history is an operator decision with its own gate)."
  - "A bounded idempotency horizon: out-of-scope (it is the one remaining full parse on a write path, and bounding it is a semantic change — a duplicate older than the horizon would be re-posted — which needs its own decision rather than riding in on a performance brief)."
version: 1
id: 94a186f1-cb05-445c-9c52-6eebc917797f
---

# Brief 24 — Bounded tail read in `Guard`, no cache-reuse rows, daily rotation, `deskaudit tail`

## Dependencies

None. Every deliverable is inside `tools/desk/`, and every surface it touches exists on main at
`e428134c`.

**On brief 23 and the `desktoken` reuse row.** Removing the cache-reuse audit row removes an
operator-visible signal, and brief 23's opt-in perf record is the natural replacement for it —
one perf row per invocation, reuse included, when `ASSAY_TELEMETRY` is armed. This brief
nevertheless declares NO dependency on 23, because the reuse row is not a control and nothing
downstream reads it: the only occurrences of the string `desktoken` in audit-reading code are the
canonical-key registry entry and two `exec.LookPath`/`exec.Command` call sites that invoke the
binary. No budget, no breaker, no idempotency decision and no gate consults a `desktoken` row —
`desktoken` does not call `AllowWrite` at all. What the removal costs is forensic detail about
invocations that performed no act; what the ledger keeps is one row per real mint and one row per
refusal or failure. Making 24 wait on 23 would hold a measured, unbounded cost behind an unrelated
opt-in feature for a signal nothing reads.

## Context

single-point-of-failure: **the determinacy rule of the bounded readers — a bounded read returns an
answer ONLY when that answer is provably identical to the one the full parse would give, and
otherwise falls back to `LoadEntries`** — with three layers behind it, each tripping on a different
signal in a different component. (1) The EQUIVALENCE rows run both readers over one planted ledger
and compare the meters' VERDICTS, not their counts: a bounded reader that stopped early admits a
write the full parse refuses, and that difference is the assertion (rows 5 and 6). (2) The
CORRUPTION path is unchanged and independent: a malformed line inside the bounded window raises the
same exit-6 `Unverifiable` the whole-file parse raises, so a bounded read can never launder a
corrupt ledger into a pass, and `deskaudit recover` remains the only sanctioned rewrite (rows 4
and 9). (3) The FLOCK is untouched and lives in a different component again: rotation takes
`lockAudit`, the same exclusive lock the outward-write flow and recovery already take, so no reader
can observe a half-renamed ledger and no rotation can race an append (row 8). The three are
genuinely independent — a determinacy bug is caught by the equivalence rows alone, a corrupt line
by the refusal path alone, and a rotation race by the lock alone — which is why the bounded reader
is allowed to be an optimisation rather than a new source of truth.

risk note — all four risk answers are `no`, and the change sits on a security-adjacent path. The
answers stand because nothing here relaxes a control: the kill switch's precedence order, its four
layers and its refusals are untouched; every rate-limit constant, tier and refusal string is
untouched; the idempotency predicate is untouched and is handed the same whole-ledger slice it gets
today; and no row is deleted by anything in this brief. The only control-shaped change is a
REMOVAL of a row that no control reads (the `desktoken` cache reuse), evidenced in the Dependencies
note above and asserted by row 7. A reviewer who finds a bounded read that can return fewer entries
than the full parse without falling back, a rotation that any reader fails to cross, or any row
that a meter reads and this brief stops writing, flips `irreversible` to yes and takes the human
gate.

The `risk-files-crossread` lint NOTICE fires here and is answered rather than ignored: the declared
paths sit under `tools/desk/internal/deskkit/`, a security-path trigger for this repo, and all four
risk answers are `no`. The answers stand because every change in that directory is a read-path
change under an equivalence obligation, plus one append-path change (rotation) that moves bytes
between files without altering, dropping or reordering a single row. Rows 4, 5, 6, 8 and 9 are the
mechanical evidence; a reviewer who finds otherwise flips the answer.

files:
- `tools/desk/internal/deskkit/audit.go` (existing) — `LastEntry`, `segmentPaths`, the reverse
  line reader (`scanBackwards`), `rotateIfNeeded` called from `Log`, and `LoadEntries` reading
  across segments. `Entry`, `Log`'s append semantics and `LoadEntries`' refusal contract unchanged.
- `tools/desk/internal/deskkit/audittail.go` (planned) (new, same package) — the bounded reverse
  reader and its byte counter, kept in its own file so the tail mechanics are reviewable apart from
  the ledger's schema.
- `tools/desk/internal/deskkit/killswitch.go` (existing) — `lastResultWas` calls `LastEntry`. One
  function body; `Guard` itself is untouched.
- `tools/desk/internal/deskkit/ratelimit.go` (existing) — `pointsFor` gains the bounded reverse
  read, its determinacy stop and its fail-closed fallback.
- `tools/desk/internal/deskkit/auditrecover.go` (existing) — `RecoverCorruptAudit` across segments.
- `tools/desk/internal/deskkit/audittail_test.go` (planned), `auditrotate_test.go` (planned),
  `ratelimitbounded_test.go` (planned) — all new in the same package.
- `tools/desk/internal/deskkit/audittail-mutations.json` (planned) (new, same package) — the
  `muhar` spec for row 10.
- `tools/desk/cmd/desktoken/desktoken.go` (existing) — the reuse path suppresses its row.
- `tools/desk/cmd/deskaudit/main.go` (existing) — the `tail` verb, its usage text and its tests.
- `tools/desk/README.md` (existing) — the ledger section gains rotation, the segment names and
  `deskaudit tail`.
- `changelog/perf-desk-tools--24-09141630.md` (planned) (new).

facts (read at `e428134c`, 2026-09-14):

- **`Guard` asks one question of the whole file.** `guard(tool)` runs its four stop layers and then
  ends with `if lastResultWas(ResultDisabled) { … }`. `lastResultWas` is four lines: call
  `LoadEntries()`, return false on error or empty, else compare `entries[len(entries)-1].Result`.
  It is already best-effort by contract — its own comment says a corrupt or unreadable file returns
  false here and that corruption surfaces through the outward-write flow instead — so a tail read
  that cannot parse the final line inherits exactly the behaviour that is already documented.
- **`LoadEntries` is a whole-file parse with a strict refusal.** It scans with a 4 MiB line cap,
  skips blank lines, `json.Unmarshal`s every other line into an `Entry`, and returns an
  `Unverifiable` naming the line number on the FIRST malformed one. A missing file is empty
  history (`nil, nil`), never an error. Both properties are contracts other code depends on and
  neither changes here.
- **A write verb pays the parse three times.** `attemptOutward` (deskpost/writeflow.go) takes the
  audit flock, calls `LoadEntries()`, calls `AllowWriteAt(...)` — which calls `pointsFor`, which
  calls `LoadEntries()` again — and then hands the slice to `plan`. `Guard` already ran a third
  parse before any of it. Two of the three are inside the flock, which is why concurrent writers
  serialise on it.
- **Correction to the dispatch note, recorded rather than dropped** (worker-kit clause 7). The
  dispatch described all three parses as last-entry reads that a tail read replaces. Two of them
  are; the third is not. `attemptOutward`'s `entries` are passed to `plan`, and every `plan`
  consumes them as the IDEMPOTENCY history — `deskkit.AlreadyDoneIn` (ready.go), `alreadyCommented`
  (comment.go) and `reviewAlreadyPostedIn` (review.go) each scan the slice for a prior `ok`/`noop`
  entry at the same `(repo, pr, head, verb)`. That question is answered by the ABSENCE of a match,
  and absence is only provable by reading everything, so no tail read and no time window can
  replace it without turning a "not found yet" into "not done". It therefore keeps the full parse
  in this brief, and bounding it is named as out-of-scope follow-up under `consumers:`. The
  conclusion the dispatch drew is unaffected: a write path drops from three full parses to one, and
  a read path from one to none.
- **The meters' questions are bounded by construction, even though their reader is not.**
  `chargedInWindow` discards everything older than `rateWindow`, which is ONE HOUR
  (`rateWindow = time.Hour`). `breakerRun` walks newest-first and `break`s at the first in-scope
  entry that is not non-progress, so it reads only as far back as the trailing consecutive
  refusal run; `BreakerTrip` is 5 and `BreakerBackstopTrip` is its tool-wide twin, so a run is
  decided the moment it reaches the trip. What is unbounded is `pointsFor`, which loads and
  converts the whole ledger before any of them look at it.
- **`nonProgress` and `breakerIgnores` are tiny closed predicates** — non-progress is `refused`
  or `unwritten`; `ratelimited`, `disabled`, `dryrun` and `noop` are invisible to the walk,
  neither counting toward the trip nor resetting it. A bounded reader must therefore keep reading
  past an invisible result rather than treat it as a stop.
- **A plain move of the ledger is a state reset, and the code says so.** `auditrecover.go`'s
  package comment: the rate-limit counter, the circuit breaker and the idempotency store all
  derive from the file, so moving it aside returns every budget to full and makes the store forget
  every prior write. `RecoverCorruptAudit` exists to quarantine the bad LINE instead, carrying
  every good line forward byte-for-byte (no reserialisation, so no schema drift) under `lockAudit`.
  That is the standard any rotation in this brief has to meet.
- **`desktoken`'s reuse row records a no-op.** `cmdToken` defers `ac.finalize(err)` so exactly one
  row is written per invocation. On the reuse path — a cached token whose age is under
  `cacheMaxAge` (50 minutes) — it prints the token PATH, sets
  `ac.detail = "reused cached … (Nm old)"` and returns nil, so `finalize` logs `ResultOK`. No
  network call was made, no credential was minted, and nothing changed. Issue #1035 measured
  145,639 such rows against 2,583 real mints on one host.
- **`deskaudit` has no read verb.** Its only verb is `recover`; there is no supported way to look
  at the tail of the ledger through the tool, which is why operators reach for `tail`/`jq` on a
  file whose location and rotation state the tool owns.
- **The state directory has one resolution path.** `deskDir()`/`StateDir()` return
  `~/.config/assay`, and `dirOverride` is the package's established test hook — deliberately not
  wired to any environment variable, because the kill-switch files live in that directory. Every
  test below uses it; nothing in this brief adds a second way to relocate the ledger.

facts — the design:

- **The reverse reader.** `scanBackwards(path, fn)` opens the file, seeks to EOF, and reads fixed
  64 KiB blocks backwards, splitting on `'\n'` and handing each non-blank line to `fn` newest-first;
  `fn` returns whether to continue. A line longer than the 4 MiB cap `LoadEntries` already uses is
  a refusal, not a silent truncation. It counts the bytes it reads into a package counter a test
  can read (`tailBytesRead()`), which is what makes "O(1) bytes" an assertion rather than a claim.
- **`LastEntry() (Entry, bool)`.** Walks the newest segment backwards for the first non-blank line
  and unmarshals it; when the live file is empty — the state immediately after a rotation — it
  continues into the newest prior segment, so the disarm-transition check cannot be blinded by a
  day boundary. Returns `ok=false` for a missing ledger, a wholly empty one, or a final line that
  does not parse, which is exactly `lastResultWas`' existing best-effort contract. It reads at most
  one 64 KiB block per segment it has to touch, whatever the ledger's size.
- **`pointsFor`'s bounded read, and the determinacy rule that governs it.** It walks the ledger
  backwards, converting entries as it goes, and stops as soon as the answer is FULLY DETERMINED —
  which means all three of:
  1. the cursor has passed `now - rateWindow`, so no further entry can enter any budget window;
  2. the per-target breaker walk has terminated — an in-scope entry that is neither non-progress
     nor ignored has been seen (the run is reset), or `BreakerTrip` in-scope non-progress entries
     have been accumulated (the run trips at that count whatever lies beyond it);
  3. the same, for the tool-wide backstop walk against `BreakerBackstopTrip`.

  Until all three hold it keeps reading. If it reaches the start of history, it has read
  everything and the answer is exact. If it reaches a hard cap — 200,000 lines or 200 MiB,
  whichever comes first — it DISCARDS the partial read and falls back to `LoadEntries()`, because
  an answer that is not determined is not an answer. Nothing about the meters changes: the same
  `auditPoint` slice, the same oldest-first ordering, the same stable sort, the same fail-closed
  `Unverifiable` on an unparseable timestamp.
- **Why the stop rule is equivalence and not a horizon.** A time horizon alone would be a
  fail-OPEN on the breaker: `BreakerTrip` consecutive refusals spread over a week, with the newest
  one a minute ago, is an open breaker today; a reader that stopped at one hour would count one
  refusal and admit the write. The stop conditions above are the meters' own termination
  conditions, so the bounded reader stops exactly where the meter would have stopped reading
  anyway — which is why the equivalence rows can compare verdicts rather than approximations.
- **Rotation: mechanism, exactly.** The live file remains `audit.jsonl`. Rotated segments are
  named `audit.jsonl.<YYYY-MM-DD>`, matching `^audit\.jsonl\.\d{4}-\d{2}-\d{2}(\.\d+)?$` EXACTLY —
  a pattern no other name in the directory can satisfy, so `audit.lock`, `audit.jsonl.corrupt-<ts>`
  and anything a future change adds are never mistaken for segments and never rewritten or renamed
  by rotation.
  - `Log` calls `rotateIfNeeded()` before it opens the file. That check is one `os.Stat`: if the
    live file's modification time falls on an earlier UTC day than `time.Now().UTC()`, rotation is
    due. An append-only file's mtime is the instant of its last append, so this costs one stat per
    invocation and reads no bytes.
  - When rotation is due, and only then, `rotateIfNeeded` takes `lockAudit` — the same exclusive
    flock the outward-write flow and `RecoverCorruptAudit` take — RE-CHECKS the condition under the
    lock (so the loser of a race does nothing), renames `audit.jsonl` to
    `audit.jsonl.<mtime's UTC date>`, and returns. A name that already exists gets `.1`, `.2`, …
    appended, the same collision rule `RecoverCorruptAudit` uses for its sidecar. The fresh
    `audit.jsonl` is created by `Log`'s existing `O_CREATE` open, so there is no window in which
    the ledger is absent to a reader that has the lock.
  - A rotation failure is never fatal to the verb: `rotateIfNeeded` swallows its own errors and
    `Log` appends to the live file regardless. The worst case of a failed rotation is a file that
    stays long — the state this brief starts from — never a lost or unwritten row.
- **Rotation carries the counter and the idempotency store forward because NOTHING IS DELETED and
  EVERY READER READS ACROSS SEGMENTS.** That is the whole mechanism, and it is what makes rotation
  different in kind from the plain move `auditrecover.go` warns about:
  - `segmentPaths()` returns the rotated segments sorted oldest-first by their date suffix,
    followed by the live `audit.jsonl`;
  - `LoadEntries()` concatenates them in that order, so the slice it returns is identical, row for
    row and in the same order, to the slice it would have returned had the file never been
    rotated. The rate-limit counter, the circuit breaker, `AlreadyDoneIn`, `FirstTS`, deskboard's
    reset banner and `loopengine`'s recovery therefore observe no change whatsoever;
  - the bounded readers walk the same list in reverse, crossing a segment boundary transparently;
  - `RecoverCorruptAudit` reads every segment and the live file, quarantines bad lines from all of
    them into ONE `audit.jsonl.corrupt-<ts>` sidecar, and rewrites only the files that carried one
    — each atomically (temp file + rename, 0600), each carrying its good lines byte-for-byte.
  - **No segment is ever deleted.** Rotation bounds the file every append and every tail read
    touches; it does not bound the history, because the ledger's contract is that no row is lost.
    Retention is an operator decision and is out of scope.
- **`deskaudit tail [N]`.** Prints the last N entries (default 10) oldest-first among those
  printed, as the RAW lines exactly as they sit on disk — no reserialisation, the same principle
  `RecoverCorruptAudit` applies for the same reason. It reads backwards through segments and stops
  at N, so it costs the same bounded read whatever the ledger's size. `N` must be a positive
  integer and is capped at 10,000. It reports three states distinctly: an absent ledger prints
  `no audit history — <path> does not exist` and exits `ExitOK` (it looked, and there is nothing);
  a ledger it cannot read exits `ExitUnverifiable` with the reason; a final line that does not
  parse is printed as-is with a `# unparseable line (run \`deskaudit recover\`)` note on stderr,
  never silently dropped. It takes no lock and writes nothing.

## Ground rules
- **No meter's verdict may change.** The budget tiers, the breaker, the backstop and the
  idempotency store must answer identically before and after, for every input. A bounded read that
  cannot prove its answer falls back to the full parse; it never narrows one.
- **No row is deleted, reordered, reserialised or rewritten** by anything in this brief except
  `RecoverCorruptAudit`, which already had that licence and keeps its exact semantics.
- **No new way to relocate the ledger.** `deskDir()` stays the one resolution path and
  `dirOverride` stays a test hook with no environment or flag binding — the kill switch lives in
  that directory.
- **The corruption contract is unchanged.** `LoadEntries` still refuses on the first malformed
  line, still names it, and still points at `deskaudit recover`. A bounded reader that meets a
  malformed line inside its window refuses the same way.
- **`Guard`'s four layers, their precedence order and their refusal strings are untouched.** This
  brief changes how the disarm-transition check READS, and nothing else in `guard`.
- **Rotation is best-effort and never load-bearing.** A rotation that cannot happen leaves a long
  file, never a missing row and never a changed exit code.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Deliverables

1. **The reverse reader — `tools/desk/internal/deskkit/audittail.go` (planned).**
   `scanBackwards(path string, fn func(line []byte) bool) error` reading fixed 64 KiB blocks from
   EOF, handing non-blank lines to `fn` newest-first, refusing a line over the 4 MiB cap; a package
   byte counter with a `tailBytesRead()` accessor for the bounded-read assertion.
2. **`LastEntry() (Entry, bool)` — `audit.go`.** The newest parseable line of the newest non-empty
   segment, reading at most one block per segment touched; `ok=false` for a missing, empty or
   unparseable-final-line ledger.
3. **`lastResultWas` — `killswitch.go`.** Calls `LastEntry`. One function body; `Guard` unchanged.
4. **Segment awareness — `audit.go`.** `segmentPaths()` (rotated segments oldest-first by date
   suffix, then the live file, matching the exact segment pattern and nothing else); `LoadEntries`
   reading them in that order with its contract and refusals unchanged.
5. **Rotation — `audit.go`.** `rotateIfNeeded()` as specified above: the one-stat due-check, the
   flock-and-re-check, the rename with the collision rule, errors swallowed; called from `Log`
   before it opens the file.
6. **The bounded meter read — `ratelimit.go`.** `pointsFor` walks backwards with the three-part
   determinacy stop, the hard cap, and the fail-closed fallback to `LoadEntries()`. No tier, no
   constant and no refusal string changes.
7. **Recovery across segments — `auditrecover.go`.** `RecoverCorruptAudit` reads every segment and
   the live file, quarantines all bad lines into one sidecar, rewrites only the files that carried
   one, atomically and byte-for-byte; `AuditRecovery.Carried`/`Quarantined` count across the whole
   ledger.
8. **No row for a cache reuse — `tools/desk/cmd/desktoken/desktoken.go`.** `auditCtx` gains a suppression flag
   set only on the cache-reuse success path; `finalize` writes nothing when it is set AND the
   terminal error is nil. Every mint, every refusal and every failure still writes exactly one row.
9. **The read verb — `tools/desk/cmd/deskaudit/main.go`.** `tail [N]` as specified, its usage text, its three
   distinct states, and its tests.
10. **The mutation gate — `internal/deskkit/audittail-mutations.json` (planned).** Baseline, a
    positive control, and one entry per control this change must not lose (enumerated in row 10).
11. **Docs.** `tools/desk/README.md`'s ledger section gains the segment naming, what rotation does
    and does not do, and `deskaudit tail`.
12. **Changelog fragment** under `changelog/`.
13. **Nothing else.** No retention policy, no idempotency horizon, no change to any meter's
    constants or verdicts, no second state directory, no new binary.

## Definition of done

- `deskversion`'s p50 against a planted ≥100 MB ledger is under 50 ms, measured before and after
  on the same ledger and both figures recorded in the Evidence table.
- `Guard()` reads a bounded number of bytes — asserted by a counting reader against ledgers that
  differ in size by three orders of magnitude, with the byte count not growing.
- Every meter answers identically before and after: the bounded `pointsFor` and a full-parse
  `pointsFor` produce the same budget and breaker VERDICTS over a planted ledger that exercises
  the window edge, a trailing refusal run at the trip, and an in-scope progress entry that resets
  it.
- A ledger that crosses a rotation has identical charged counts, an identical breaker verdict and
  an identical `AlreadyDoneIn` answer to the same rows in one unrotated file.
- `deskaudit recover` quarantines a malformed line from a ROTATED segment and carries every good
  entry — in that segment and in the live file — forward.
- `desktoken` writes no audit row for a cache reuse and exactly one for a mint; a failed reuse
  still writes its row.
- `deskaudit tail` prints the last N entries across a segment boundary and distinguishes
  absent from unreadable.
- `go build ./...`, `go vet ./...`, `gofmt -l` clean, and `statusgen --root .. --lint` reports
  `LINT: PASS`.
- The brief's board row is `implemented` and the changelog fragment is present.

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 — the bounded reader, the rotation hook, the segment-aware readers and the two verb changes compile with no call-site churn beyond the files named under `files:` |
| 2 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestGuardOnHundredMegabyteLedgerMedianUnderFiftyMilliseconds$' -count=1 -timeout 600s` | exit 0 — the test plants a ≥100 MB ledger in a `dirOverride` temp dir, measures `Guard()` over 20 runs with BOTH readers, prints each median, and GATES on the bounded median being under 50 ms. The full-parse median is printed for the record, not asserted on, so a faster machine cannot make the row vacuous |
| 3 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestLastEntryReadsBoundedBytes$' -count=1` | exit 0 — the counting-reader row: `tailBytesRead()` after `LastEntry` on a 1,000-row ledger and on a 1,000,000-row ledger are both under 128 KiB and differ by less than one block, while `LoadEntries` over the same two files reads bytes proportional to each. Equal-and-bounded is the assertion, not "smaller" |
| 4 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestLastEntryThreeStates$' -count=1` | exit 0 — a missing ledger, a wholly empty ledger and a ledger whose final line is malformed each return `ok=false` and no error to `lastResultWas`, matching its documented best-effort contract; a ledger whose LIVE file is empty after a rotation returns the newest entry of the newest prior segment, so a day boundary cannot blind the disarm check |
| 5 | check +mutation | `cd tools/desk && go test ./internal/deskkit/ -run '^TestBoundedPointsForMatchesFullParseVerdicts$' -count=1` | exit 0 — the SPOF row: over a planted ledger carrying entries either side of the one-hour window edge, a trailing consecutive-refusal run of exactly `BreakerTrip` whose oldest member is days old, an in-scope progress entry that resets a longer run, and results the breaker ignores interleaved throughout, the bounded `pointsFor` and a full-parse `pointsFor` yield the SAME verdict from `AllowWriteAt` for every tier. Mutation: stopping the bounded walk at the window edge alone REDDENS this test |
| 6 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestBoundedPointsForFallsBackWhenUndetermined$' -count=1` | exit 0 — the fail-closed control: with the hard cap forced to a small value and a ledger whose determinacy conditions are not met inside it, `pointsFor` discards the partial read and returns the FULL-parse result; and a malformed line inside the bounded window returns the same `Unverifiable` naming recovery that the whole-file parse returns, never a silent skip |
| 7 | check:ci | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestCacheReuseWritesNoAuditRowAndMintWritesOne$' -count=1` | exit 0 — one invocation that mints writes exactly one `ok` row; a second invocation served from the cache writes NONE and the ledger's length is unchanged; a reuse path that fails (a cache file whose mode is not 0600) still writes its refusal row, so suppression is scoped to the success path alone |
| 8 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRotationCarriesCounterAndIdempotencyForward$' -count=1` | exit 0 — the same rows written across a simulated day boundary produce identical charged counts, an identical breaker verdict and an identical `AlreadyDoneIn` answer to the same rows in one unrotated file; `LoadEntries` returns them in the same order with the same length; the rotated segment is named `audit.jsonl.<YYYY-MM-DD>`; and `audit.lock`, an `audit.jsonl.corrupt-<ts>` sidecar and a decoy name are untouched by rotation |
| 9 | check:ci +flow | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRecoverAcrossSegments$' -count=1 && go test ./cmd/deskaudit/ -run '^TestTailAcrossSegmentsAndThreeStates$' -count=1` | exit 0 — the cross-component path: a malformed line planted in a ROTATED segment and another in the live file are both quarantined into one sidecar, every good entry in both files is carried forward byte-for-byte, and the counter and idempotency answers after recovery equal those before the corruption; and `deskaudit tail 5` prints five raw lines spanning the segment boundary, prints a reason-named line and exits 0 on an absent ledger, and exits `ExitUnverifiable` on an unreadable one |
| 10 | check +mutation | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/audittail-mutations.json` | exit 0 — baseline GREEN, positive control CAUGHT, and every mutation CAUGHT: the determinacy stop reduced to the time window alone, the fail-closed fallback replaced by returning the partial read, the malformed-line refusal in the bounded path softened to a skip, rotation's segment-name pattern relaxed to a prefix match (so a sidecar or the lock file could be swept in), `segmentPaths` returning only the live file (the reset this brief exists to avoid), and `desktoken`'s suppression widened to the error path |
| 11 | check:ci | `cd tools/desk && go test -timeout 600s ./internal/deskkit/... ./cmd/desktoken/... ./cmd/deskaudit/... ./cmd/deskpost/... -count=1` | exit 0 — every existing test of the touched and adjacent packages, unchanged: the kill switch's own suite, the rate limiter's tiers and breaker, the recovery suite, and deskpost's outward-write flow whose full parse this brief deliberately leaves in place |
| 12 | check:ci | `cd tools/desk && gofmt -l internal/deskkit/audit.go internal/deskkit/audittail.go internal/deskkit/killswitch.go internal/deskkit/ratelimit.go internal/deskkit/auditrecover.go cmd/desktoken cmd/deskaudit > /tmp/dt24-fmt.out; test ! -s /tmp/dt24-fmt.out` | exit 0 |
| 13 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |
| 14 | check:ci +dereference | `cd statusgen && go run . --root .. --consumers --brief assay:assay:desk-tools:24; echo $?` | 0 — the routing claims in `consumers:` are RESOLVED against this branch's own diff, not counted: every entry marked `fixed-here` is touched by the diff and every `out-of-scope` one is not. Exit 2 is COULD-NOT-CHECK (no diff to take — a fully merged tree), which is reported AS ITSELF and never as a pass |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| The bounded meter read stops early and admits a write the full parse refuses | row 5 (verdict equivalence over a ledger built to straddle every stop condition) + row 10's window-only mutation |
| A partial read is returned as an answer when determinacy was never reached | row 6 + row 10's fallback mutation |
| A malformed line inside the bounded window is skipped instead of refusing, so a corrupt ledger reads as clean | row 6 + row 10's refusal-softening mutation |
| Rotation resets the budget and the idempotency store — the exact defect a plain file move causes | row 8 + row 10's `segmentPaths`-returns-only-the-live-file mutation |
| Rotation renames or rewrites something that is not a segment (the lock, a corrupt sidecar) | row 8 (decoys asserted intact) + row 10's pattern-relaxation mutation |
| `deskaudit recover` silently stops repairing once a segment exists, so corruption in an older segment is permanent | row 9 |
| The disarm-transition check goes blind on the first invocation of a new UTC day, when the live file is empty | row 4 |
| `Guard` still reads bytes proportional to the ledger because the tail read falls back on a common path | row 3 (two ledgers three orders of magnitude apart, byte counts asserted equal-and-bounded) |
| The whole change is fast on a small fixture and still slow in production | row 2 (a ≥100 MB ledger, a gated median, both readers measured on the same file) |
| Suppressing the `desktoken` row also suppresses a refusal or a failure, so a real fault stops being recorded | row 7 (the failing-reuse case) + row 10's widened-suppression mutation |
| A meter somewhere reads `desktoken` rows and this brief blinds it | the Dependencies note's grep, re-run by the reviewer; row 11 (every existing suite of the touched packages) |
| The performance win is real but the write path is untouched, and the brief claims otherwise | review-only — the diff shows `tools/desk/cmd/deskpost/writeflow.go` unchanged, and the Context states the residual full parse and names bounding it as out of scope |
| Rotation fails on a host and a verb's exit code changes because of it | review-only — `rotateIfNeeded` swallows its errors by construction; a reviewer confirms no error path from it reaches `Log`'s return |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

| # | Exit | Key observed output |
|---|------|---------------------|
| — | — | not yet run — this brief is authored, not implemented |
### Non-implementer verifier run — 2026-09-17 sonnet-5-verifier (verify-desk dispatch) — **VERIFY: PASS**

Runner ≠ implementer. Brief's own Evidence section was never filled in by the implementer despite merge ("not yet run — this brief is authored, not implemented") — nothing self-reported to void; verified from scratch. Deliverable squash-merge `497239d9c` (PR #1061) confirmed an ancestor of `origin/main` `e5f2b89dbab1e4be255fe15ca73ba00dc8764995`.

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `go build ./... && go vet ./...` | exit 0 | exit 0 | 2026-09-17 | sonnet-5-verifier |
| 2 | `TestGuardOnHundredMegabyteLedgerMedianUnderFiftyMilliseconds` | exit 0, bounded p50 <50ms | exit 0 — 109MB/460k rows: bounded p50=158.792µs, whole-parse=904.78ms | 2026-09-17 | sonnet-5-verifier |
| 3 | `TestLastEntryReadsBoundedBytes` | exit 0, equal+bounded byte count | exit 0 (asserts <128KiB AND identical bytes between 1k/100k-row ledgers) | 2026-09-17 | sonnet-5-verifier |
| 4 | `TestLastEntryThreeStates` | exit 0, 3(+1) states | exit 0, 4 subtests (missing/empty/malformed-final-line/empty-live-after-rotation) | 2026-09-17 | sonnet-5-verifier |
| 5 | `TestBoundedPointsForMatchesFullParseVerdicts` (SPOF row) | exit 0, verdict equivalence | exit 0 — 9 probes incl. `BreakerTrip` with a 96h-old oldest member (defeats a time-horizon reader), verdict (not count) equality asserted with a fixture-too-weak guard | 2026-09-17 | sonnet-5-verifier |
| 6 | `TestBoundedPointsForFallsBackWhenUndetermined` | exit 0, fail-closed fallback | exit 0, 3 subtests (hard-cap discard, malformed-line refusal, unparseable-timestamp fail-closed) | 2026-09-17 | sonnet-5-verifier |
| 7 | `TestCacheReuseWritesNoAuditRowAndMintWritesOne` | exit 0 | exit 0 | 2026-09-17 | sonnet-5-verifier |
| 8 | `TestRotationCarriesCounterAndIdempotencyForward` | exit 0 | exit 0 | 2026-09-17 | sonnet-5-verifier |
| 9 | `TestRecoverAcrossSegments` + `TestTailAcrossSegmentsAndThreeStates` | exit 0 | exit 0 both | 2026-09-17 | sonnet-5-verifier |
| 10 | `muhar -spec audittail-mutations.json` | baseline GREEN, all mutations CAUGHT | exit 0 — 10 caught, 0 not-caught, 0 could-not-mutate | 2026-09-17 | sonnet-5-verifier |
| 11 | whole-module test (`deskkit`/`desktoken`/`deskaudit`/`deskpost`) | exit 0 | exit 0, all packages ok | 2026-09-17 | sonnet-5-verifier |
| 12 | `gofmt -l` | empty | empty | 2026-09-17 | sonnet-5-verifier |
| 13 | `--lint` | exit 0 | exit 0, LINT: PASS | 2026-09-17 | sonnet-5-verifier |
| 14 | `--consumers --brief` on fully-merged tree | exit 0 or 2 could-not-check | **exit 2, could-not-check as expected on a fully-merged tree — but the underlying report was itself wrong**: ran `--consumers --base <real parent>` directly, which reported all six `fixed-here` claims DISPROVED. Traced the cause: a genuine `statusgen` tool bug — it never strips markdown backticks from a `consumers:` site token, so a backtick-quoted path (this house's standard convention) always fails to resolve. Independently corroborated every claim by hand-diffing (`git diff --name-only <parent> <merge>`): all six `fixed-here` paths genuinely present, both `out-of-scope` paths genuinely absent — the brief's own claims are accurate; only the tool's parsing is wrong. Filed: `medici-finance/assay#1296` | 2026-09-17 | sonnet-5-verifier |

**RISK-VALUE: DERIVED**
- `boundedReadMaxLines=200_000` / `boundedReadMaxBytes=200MiB` (`ratelimitread.go:49-50`) — top-ranked: on exceeding the cap the code discards the partial read and falls back to `LoadEntries()` (`:128`), confirmed fail-closed (row 6), never fail-open.
- `segmentPattern = ^audit\.jsonl\.\d{4}-\d{2}-\d{2}(\.\d+)?$` (`audit.go:101`) — governs which files rotation/recovery treat as segments; row 10's mutation confirms a prefix-relaxation is caught, row 8 confirms decoys untouched.
- `tailBlockSize=64KiB` (`audittail.go:42`) — read granularity only, no correctness exposure.
- No retention/deletion call exists anywhere in the diff (grepped for `os.Remove`/`RemoveAll`/`Truncate` — only hit is a pre-existing atomic-rename temp-cleanup, not data-destroying). The worst-case failure mode this brief names (rotation silently destroying audit history) has no code path that can do it.
- `desktoken`'s pre-existing `cacheMaxAge=50min` is unchanged; the brief only adds a `suppress` flag scoped strictly to the cache-reuse success path, confirmed at source.

**VERIFY: PASS** — 13 of 14 rows executed with real, non-vacuous passes (incl. the SPOF equivalence row, the fail-closed fallback row, and a fully-caught mutation gate 10/10); row 14 produced exactly the could-not-check the brief documents for a fully-merged tree, plus an independent manual corroboration after finding (and filing) a pre-existing, unrelated statusgen tool bug. No invented scope — diff matches the brief's declared files exactly.

## Review

Gate: model (all four risk answers no). Model-gated because both hazards are mechanically
bounded: the read-equivalence hazard by row 5's verdict comparison over a ledger built to straddle
every stop condition, row 6's fail-closed fallback and refusal controls, and row 10's mutations of
each; and the rotation hazard by row 8's counter/idempotency carry-forward, row 9's cross-segment
recovery, and the flock that rotation shares with every other writer. The reviewer confirms in the
verdict: (1) that the bounded `pointsFor` can only return an answer it has PROVED equal to the full
parse's, and that every other case falls back rather than narrows; (2) that every reader —
`LoadEntries`, the bounded readers, `RecoverCorruptAudit`, `FirstTS` — crosses the segment
boundary, so rotation deletes and resets nothing; (3) that rotation's name pattern can match a
segment and nothing else in the state directory; and (4) that `desktoken`'s suppression is scoped
to the cache-reuse SUCCESS path, leaving every mint, refusal and failure recorded.
