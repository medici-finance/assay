---
brief: desk-tools/21
title: "`DESK_TRACE` and cause-carrying errors — one subprocess runner, and a swallowed child's message reaches the operator on the first read"
why: >-
  The desk tools shell out constantly, and until now each command decided for itself how much
  of a child process's stderr survived into the error an operator finally reads. Three runners
  had grown independently and diverged in exactly the way that matters: one forwarded the
  child's message whole, one reduced it to its FIRST stderr line — which for a desk tool is the
  `assay-config:` echo and never the diagnosis — and one dropped it entirely. The
  operator-visible result was the same in all three: a bare exit status with the answer one
  process away, and a bisect by hand re-running the child to see what it had already said. And
  when the one-line message genuinely is not enough there is no way to ask for more: no switch
  anywhere prints the cause chain, the command lines run, or the child exit codes. This brief
  makes the child's own words the default and puts the rest behind one switch.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-09 by a worker-desk authoring session, from an operator request recorded
  2026-09-09 and a same-day read of the four verbs named below
sources:
  - "Operator request, 2026-09-09: more descriptive error messages out of the desk tools — a debug/trace option, and a way of exposing the actual error message instead of swallowing it."
  - "freshness-checked 2026-09-09 @ e82c565e (origin/main) — each symptom below re-read against the source at that commit rather than taken from the report. Two of the five reported symptoms did NOT reproduce as described and are recorded here as corrections, not as facts."
  - "The three divergent runners: `tools/desk/cmd/deskdispatch/exec.go` § runCmd/toolMessage (forwards whole, but only at ONE of its call sites), `tools/desk/cmd/deskwt/exec.go` § runGitStreams (appends git's text, carries no exit status or argv), `tools/desk/cmd/deskfile/exec.go` § runCmd (appends gh's text after StripControl)."
  - "The typed-error contract this extends: `tools/desk/internal/deskkit/exitcodes.go` § DeskError, Refused/Unverifiable/RefusedFinding, ExitCodeOf. The out-of-band-payload precedent is ScanFinding: carried in its own field, reached via errors.As, DELIBERATELY not rendered by Error()."
  - "The redaction patterns reused: `tools/desk/internal/deskkit/bodycheck.go` § reGitHubToken / reJWT / reAWSKeyID, and `deskkit.StripControl`."
exec-tier: strong
exec-tier-why: >-
  (c) the change adds a surface that prints argv, child stderr and environment overrides —
  the three places a credential actually travels. A redactor that is merely present passes
  every happy-path test; only a planted-secret table distinguishes one that works from one
  that does not. The second hazard is silent: a refusal that grows a cause must not become
  a weaker refusal, and an exit code flattened in passing looks identical in a green suite.
consumers:
  - "Every other desk verb: OUT OF SCOPE and unchanged. `deskkit.ReportError` is opt-in per verb — a verb that still calls `fmt.Fprintln(os.Stderr, err.Error())` behaves exactly as it does today. The four retrofitted here are the first wave, chosen because each carries a symptom that was actually observed."
  - "`tools/desk/internal/deskkit/bodycheck.go`: read, not changed. The new redactor REUSES its compiled patterns; a prefix added there is redacted in traces on the same commit."
---

# Brief 21 — `DESK_TRACE` and cause-carrying errors

## Dependencies
None.

## Context

single-point-of-failure: **`deskkit.Scrub`, the one redactor every diagnostic surface passes
through** — it is the only thing standing between a trace and a printed credential. Behind it
are two layers that trip on different signals: the tools never place a token in an argv in the
first place (`desktoken` writes a 0600 file and prints its PATH; `deskdispatch` passes `GH_TOKEN`
in a child ENVIRONMENT, not on a command line), and the outward-write body scan refuses any body
carrying a token shape, so a trace pasted into a PR or an issue is refused at that boundary by a
different mechanism in a different component. That second layer is genuinely independent —
different signal, different component, different moment — but it only fires if the trace is
written outward, so it does not excuse a weak scrubber. The Verify table's row 5 is the direct
test of the SPOF itself: a secret planted in a child's stderr, in the process environment and in
an argv, asserted absent from the trace.

risk note — all four risk answers are `no`, and the change touches a disclosure surface. The
answers stand because the diagnostic block is OPT-IN and OFF by default (rows 3 and 6 pin that
default output is byte-identical), and because the redactor is bounded by a per-shape table
including a NEGATIVE control that ordinary diagnostic text survives unaltered (row 4). A
reviewer who finds a diagnostic surface that bypasses `Scrub`, or a default-output change beyond
the `— <tool> said: …` suffix, flips `sensitive-data` to yes and takes the human gate.

files:
- `tools/desk/internal/deskkit/runtool.go` (NEW — the one subprocess runner)
- `tools/desk/internal/deskkit/trace.go` (NEW — the switch, the ledger, the shared exit path)
- `tools/desk/internal/deskkit/scrub.go` (NEW — the redactor)
- `tools/desk/internal/deskkit/exitcodes.go` (`DeskError` gains the subprocess detail;
  `Cause()`; `RefusedWithCause`)
- `tools/desk/internal/deskkit/trace_test.go` (NEW)
- `tools/desk/internal/deskkit/trace-mutations.json` (NEW - the `muhar` spec for row 16)
- `tools/desk/cmd/deskdispatch/` (`exec.go`, `dispatch.go`, `worktree.go`, `main.go`, and a
  new `trace_test.go`)
- `tools/desk/cmd/deskwt/` (`exec.go`, `main.go`, `trace_test.go`)
- `tools/desk/cmd/desktoken/` (`desktoken.go`, `main.go`, `trace_test.go`)
- `tools/desk/cmd/deskfile/` (`exec.go`, `main.go`, `trace_test.go`)
- `tools/desk/README.md` (the `Diagnostics — DESK_TRACE` section) and the four verbs' `--help`

facts — the symptoms, each re-read at `e82c565e` (observed 2026-09-09 in an operating desk
session, then verified against the source rather than taken from the report):

1. **`deskdispatch` reports the config echo, not the tool's message.** Nine diagnostic sites are
   built with `firstLine(r.stderr)`. Every desk tool opens stderr with `assay-config: …` and, on
   an unpinned build, a drift warning, so the first line is *always* preamble. The observed shape
   is `step claim-acquire: the claim on <key> could not be established (assay-config: …)`.
   `tools/desk/cmd/deskdispatch/exec.go` already carries a `toolMessage` that strips this preamble — with a comment
   describing this very failure — but only ONE call site uses it. The fix is to make that the
   rule rather than the exception.
2. **`deskdispatch`'s `gitOut` swallows the child entirely.** `tools/desk/cmd/deskdispatch/worktree.go` returns the raw
   `*exec.ExitError`, i.e. the string `exit status 128`. This is the hardest shape in the suite
   to bisect: there is nothing in it to search for and nothing naming which of several `git`
   reads failed.
3. **`desktoken` prints a token FILE PATH and nothing says so.** A caller that uses stdout as the
   credential gets `401 Bad credentials` from its next forge call — three processes downstream,
   naming neither `desktoken` nor the path.
4. **CORRECTION — `deskfile` was reported to hide the search API's status on its exit-6 path; it
   does not.** `tools/desk/cmd/deskfile/exec.go` § `runCmd` formats `%w (%s)` with gh's stderr, so gh's
   `HTTP 401` / `403` / `429` line already reaches the message. What is genuinely absent is the
   STRUCTURED half — no argv and no child exit status on the error — so a trace would have had
   nothing to print for the commonest `deskfile` failure there is. Scope adjusts accordingly:
   this is a carry-the-detail change, not a surface-the-status one.
5. **CORRECTION — `deskwt`'s stderr was reported as discarded; its TEXT is not.** `runGitStreams`
   appends git's stderr in parentheses. What it produces is an `*fmt.wrapError` carrying no exit
   status and no argv. Same conclusion as (4): the structured half is what is missing.
6. **No trace switch exists anywhere.** No verb prints a cause chain, a command line, a child
   exit code or a timing, under any environment variable or flag.
7. **The good example to model on:** `deskreply --workpad`'s refusals name the marker line, the
   entropy run and the forge-resolve gap. They are the standard the rest is being brought up to.

facts — the design:

- **`DeskError` carries the cause; it does not gain a second field for it.** `Err` already IS
  the cause — `Unwrap` returns it, `errors.Is`/`errors.As` reach through it, `Error()` renders
  it. A separate `Cause` field alongside `Err` would give one value two homes and two chances to
  disagree, which is the defect class this brief is about. A `Cause()` ACCESSOR is added instead.
  What is genuinely new is the SUBPROCESS detail — `Cmd`, `Stderr`, `ExitStatus` — carried out of
  band on the ScanFinding precedent: set by the runner, not rendered by `Error()`, printed only
  by the trace.
- **`Refused` keeps its signature.** It has 400-odd call sites; a signature change is churn on
  every one, and a variadic that silently accepts a second argument is a signature nobody can see
  at the call site. `RefusedWithCause(msg, err)` is a second constructor. It stays a refusal:
  same exit code, same fail-closed semantics, same message.
- **One runner, and no package gives up its argv assertions.** `deskkit.Run(ToolCall)` captures
  both streams, recovers the child's exit status (`ExitStatusOf`, so a wrapped tool's 5/6 passes
  THROUGH rather than being flattened) and strips the preamble. `ToolCall.Start` is the command
  constructor, so each package keeps its own `var execCommand = exec.Command` recording seam and
  its tests keep asserting on the real constructed argv. `ToolRun.Fail(code, format, …)` renders
  `<context> — <tool> said: <first stderr line>`; `FailVerbatim(code, msg)` is for a site that
  has already composed the whole message. **The verdict stays the caller's in both** — the runner
  never upgrades or downgrades a refusal.
- **The switch.** `DESK_TRACE=1` (also `true`/`yes`/`on`; empty and `0` are OFF, so an
  accidentally-exported empty variable does not trace a whole session), plus a global `--trace`
  stripped by `TakeTraceFlag` BEFORE any `FlagSet` sees it — which is what makes it work on a
  verb whose positional grammar would otherwise refuse an unknown flag. The env var is the
  PRIMARY form: a desk verb that shells out to another passes the environment along, so one
  export traces the chain, while a flag is lost by every child.
- **What ON prints:** the same first line, then the full cause chain (one numbered line per
  wrapper), the failing child's command line as executed, its exit status, its stderr in full,
  and every child process the run started with its status and elapsed time — a step that
  SUCCEEDED included, because the commonest question a stuck operator has is which steps ran.
- **What OFF prints:** `err.Error()` and a newline. Byte-identical to the `fmt.Fprintln` each
  `main()` called. The single intended change to default output is the `— <tool> said: …` suffix.
- **Redaction is not per-call-site.** Everything the trace prints goes through `Scrub`, which
  reuses the body scanner's own patterns and adds the three TRANSPORT shapes a command line
  carries and a body never does: URL userinfo (`https://x-access-token:…@`), an `Authorization:`
  header, and a secret-shaped `NAME=value` assignment. It runs `StripControl` FIRST — an ANSI
  sequence embedded mid-token would otherwise split a run the patterns match as one. Redactions
  are MARKED `<redacted>`, never silently elided, so an operator can tell "redacted" from
  "empty". It is biased to over-redact: a redacted value is a nuisance, a printed token is an
  incident.
- **The ledger is only kept while tracing is on.** With the switch off the runner records
  nothing — which is both why the off-path costs nothing and why a process that was never asked
  to trace retains no child stderr in memory. It is capped at 200 entries, oldest dropped, and
  the block says how many were dropped rather than presenting a truncated ledger as complete.
- **`desktoken` gets a NOTICE, not a `--print-token`.** The notice goes to STDERR, so stdout
  stays byte-identical for every caller that pipes it. A flag printing the value on stdout would
  exist precisely to defeat the posture this tool's own source states at both print sites
  ("Output only the token file path — never the token value"), and the caller it would serve is
  already served by `cat "$(desktoken …)"`. Adding one is a security-posture change requiring a
  recorded human ruling, not a worker's judgement — so it is not in this brief.

## Ground rules
- **A refusal stays a refusal.** No exit code changes, no fail-closed path becomes fail-open,
  no security control or its CI assertion is weakened to add detail. If detail cannot be added
  without weakening one, the detail does not land.
- **Off must stay byte-identical.** Any default-output change beyond the `— <tool> said: …`
  suffix is out of scope and is a finding.
- **A trace never prints a credential.** No diagnostic surface may bypass `Scrub`.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **`deskkit` core.** `runtool.go` (`ToolCall`/`ToolRun`/`Run`/`ExitStatusOf`/`ToolMessage`/
   `Said`/`SaidAll`/`CommandLine`/`Fail`/`FailVerbatim`), `trace.go` (`TraceEnabled`/`SetTrace`/
   `ResetTrace`/`TakeTraceFlag`/`TraceSteps`/`ReportError`), `scrub.go` (`Scrub`). `ToolMessage`
   is LIFTED from `tools/desk/cmd/deskdispatch/exec.go`, where it already exists — do not write a second one.
2. **`exitcodes.go`.** The three subprocess fields, `Cause()`, `RefusedWithCause`. `Error()`
   suppresses the wrapped-cause rendering for an error the runner built, so the message ends on
   the child's words and not on the bare `exit status N` that would otherwise follow them.
3. **First-wave retrofits**, each keeping its own `execCommand` seam: `deskdispatch` (every
   `firstLine(r.stderr)` diagnostic site, `gitOut`, and the terminal-error sites), `deskwt`
   (`runGitStreams`), `desktoken` (the NOTICE), `deskfile` (the runner, preserving the existing
   `StripControl` on gh's attacker-influenceable stderr by moving it into `SaidAll`). Each verb's
   `main()` routes its terminal error through `deskkit.ReportError` and calls `TakeTraceFlag`
   first.
4. **Fail-first tests, per retrofit**, asserting the OLD shape lacked the child's message and the
   new one carries it; that `DESK_TRACE=1` output contains the command line; and that a planted
   token in argv, environment and child stderr is redacted.
5. **Docs.** The README `Diagnostics — DESK_TRACE` section and a `DIAGNOSTICS:` paragraph in each
   retrofitted verb's `--help`; a changelog fragment.
6. **Nothing else.** No other verb is retrofitted, no exit code moves, no refusal is softened.

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 — no call-site churn beyond the retrofits |
| 2 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestToolRunSaidSkipsPreambleAndCarriesTheToolsOwnMessage$' -count=1 && go test ./internal/deskkit/ -run '^TestToolRunFailShapeAndCarriedDetail$' -count=1` | exit 0 — the child's own message, not the config echo; the `— <tool> said:` shape; `errors.As` still reaches `*exec.ExitError` |
| 3 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestReportErrorOffIsByteIdentical$' -count=1 && go test ./internal/deskkit/ -run '^TestReportErrorOnPrintsChainCommandsAndTimings$' -count=1` | exit 0 — OFF is byte-identical to `err.Error()`; ON carries chain, command lines, exit codes and timings |
| 4 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestScrubRedactsEveryTransportShape$' -count=1` | exit 0 — every transport shape redacted AND the negative control (ordinary diagnostic text) passes through unaltered |
| 5 | check +mutation | `cd tools/desk && go test ./internal/deskkit/ -run '^TestTraceNeverPrintsACredential$' -count=1` | exit 0 — the SPOF row: a token planted in argv, environment and child stderr is absent from the trace, and the redaction is marked |
| 6 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRefusedWithCauseStaysARefusal$' -count=1 && go test ./internal/deskkit/ -run '^TestTraceEnabledReadsTheEnvSpellings$' -count=1` | exit 0 — the NEGATIVE controls: a cause does not soften exit 5; an empty or `0` `DESK_TRACE` is OFF |
| 7 | check:ci | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestClaimAcquireFailureNamesTheClaimToolsOwnMessage$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestGitOutFailureCarriesGitStderr$' -count=1` | exit 0 — facts 1 and 2 closed |
| 8 | check:ci | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestWorktreeCreateFailureCarriesDeskwtStderrAndTheCommandLine$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestTraceIsOffByDefaultAndOutputIsUnchanged$' -count=1` | exit 0 — deskwt's refusal (5) still passes through, the trace names the argv and status, and the default output did not move |
| 9 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRunGitFailureCarriesStderrCommandAndExitStatus$' -count=1 && go test ./cmd/deskwt/ -run '^TestDeskwtTraceOffIsByteIdenticalAndOnCarriesTheCommand$' -count=1` | exit 0 — fact 5 closed, both directions of the switch |
| 10 | check:ci | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestTokenPathNoticeIsPrintedOnStderrNotStdout$' -count=1` | exit 0 — fact 3 closed; stdout is still EXACTLY the path and the token value reaches neither stream |
| 11 | check:ci | `cd tools/desk && go test ./cmd/deskfile/ -run '^TestDedupeSearchOutageNamesTheAPIStatus$' -count=1 && go test ./cmd/deskfile/ -run '^TestGhStderrStripsControlBytes$' -count=1` | exit 0 — 401/403/429 tell apart, exit 6 unchanged, and the pre-existing control-sequence strip survived the move |
| 12 | check:ci | `cd tools/desk && go test -timeout 300s ./internal/deskkit/... ./cmd/deskdispatch/... ./cmd/deskwt/... ./cmd/desktoken/... ./cmd/deskfile/... -count=1` | exit 0 — every existing test of the five touched packages, unchanged |
| 13 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestS2' -count=1 && go test ./internal/deskkit/ -run 'TestCorpus' -count=1` | exit 0 — the control-based leak sweep, split into its two halves so no regex alternation is spelled inside a table cell (the stream's Conventions rule); CI runs them in one invocation |
| 14 | check:ci | `cd tools/desk && gofmt -l internal/deskkit/trace.go internal/deskkit/runtool.go internal/deskkit/scrub.go internal/deskkit/exitcodes.go cmd/deskdispatch cmd/deskwt cmd/desktoken cmd/deskfile > /tmp/dt21-fmt.out; test ! -s /tmp/dt21-fmt.out` | exit 0 |
| 15 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |
| 16 | check +mutation | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/trace-mutations.json` | exit 0 - baseline GREEN, positive control CAUGHT, and every mutation CAUGHT: the redactor's three shapes each removed in turn, the trace block made unconditional, `RefusedWithCause` softened to exit 6, the preamble strip disabled, an empty `DESK_TRACE` treated as on, deskwt's refusal flattened in the worktree-create step, the claim step's report reverted to `firstLine(stderr)`, and `gitOut` reverted to dropping git's stderr |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| The trace prints a credential | row 5 (planted in three places at once) |
| The redactor is so broad every trace is unreadable | row 4's negative control |
| Default output moves, breaking a transcript or a scripted caller | rows 3, 8, 9 (byte-identity, at deskkit and at two verbs) |
| A refusal is softened while gaining a cause | row 6 (`RefusedWithCause` keeps exit 5), row 8 (deskwt's 5 still passes through deskdispatch) |
| A child's exit status is flattened into one generic failure | rows 8 and 9 (`ExitStatus` asserted, refusal-vs-unverifiable asserted) |
| `DESK_TRACE` set to an empty string traces a whole session by accident | row 6 |
| A verb's `execCommand` seam is lost to the shared runner, so argv assertions stop testing argv | row 12 (every existing argv assertion in the four packages) |
| `deskfile`'s `StripControl` on attacker-influenceable gh stderr is lost in the move | row 11 |
| A control is present but does not actually redden when the guarded thing is broken | row 16 (ten mutations, each the real inverse of one control) |
| A second runner is written instead of the existing one being lifted | review-only — the reuse ladder; `ToolMessage`'s provenance is stated in its doc comment |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). The fail-first reds for
     rows 7–11 are recorded on the PR under `## Fail-first`. -->

### Implementer run — 2026-09-09, worker session (darwin/arm64, go1.26.5, offline)

Implementer evidence, NOT a verification: rows 1–16 run in the implementation worktree off
`refs/remotes/origin/main` @ `e82c565e`. A non-implementer re-runs them at verify time.

| # | Exit | Key observed output |
|---|------|---------------------|
| 1 | 0 / 0 | `go build ./...` and `go vet ./...` both silent across the whole module — no call-site churn beyond the retrofits |
| 2 | 0 | both pass. Fixture guard inside row 2 asserts the OLD shape first: the raw first stderr line is `assay-config: …` and does NOT carry the diagnosis |
| 3 | 0 | OFF output equals `err.Error()+"\n"` exactly; ON carries `cause chain`, `child stderr (full)`, `child processes (2)`, `exit 0 in …` and `exit 5 in …` |
| 4 | 0 | 7 shapes redacted; negative control — 3 ordinary diagnostic strings pass through byte-identical |
| 5 | 0 | token planted in argv + environment + child stderr; absent from the trace, `<redacted>` present, `x-access-token` context preserved |
| 6 | 0 / 0 | `RefusedWithCause` stays exit 5 and `IsRefused`; `DESK_TRACE` empty / `0` / `false` / `no` / `maybe` all OFF |
| 7 | 0 / 0 | fail-first reds recorded before the fix: `the claim on <key> could not be established (assay-config: roster=… allowed=3 repos)` and `gitOut … message is only: exit status 128` |
| 8 | 0 / 0 | trace names `deskwt add`, `child exit status: 5`; deskwt's refusal still maps to exit 5; default output byte-identical |
| 9 | 0 / 0 | fail-first red recorded: `runGitStreams no longer produces a *DeskError: *fmt.wrapError`. After: `Cmd` and `ExitStatus` carried |
| 10 | 0 | stdout is EXACTLY the token path; notice on stderr only; the token value reaches neither stream |
| 11 | 0 / 0 | 401 / 403 / 429 each named in both the runner error and the exit-6 refusal; `\x1b`/`\x07` absent while `HTTP 422` survives |
| 12 | 0 | all five packages green: deskkit 33.9s, untrustcorpus 0.3s, deskdispatch 10.0s, deskwt 22.4s, desktoken 6.9s, deskfile 15.5s |
| 13 | 0 | 15 control tests PASS, 1 SKIP by design (`TestS2SweepExclusionsAreLive`, its private-tree fixture absent) — the same invocation the control-sweep CI job runs |
| 14 | 0 | no brief-owned file listed. Out of scope and pre-existing on main: `tools/desk/internal/deskkit/forge_writefile_test.go` and `tools/desk/cmd/deskdispatch/phantom_test.go`, neither in this diff |
| 15 | 0 | `LINT: PASS` (built from this tree; NOTICEs only) |
| 16 | 0 | `Harness healthy: baseline GREEN, positive control CAUGHT.` `Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.` |

Note on row 16: the sweep did real work rather than confirming a foregone conclusion. Its first
run reported **4 NOT CAUGHT** — the three per-shape redactor mutations and the empty-`DESK_TRACE`
one — because the mutation spec's `-run` filter did not reach the tests that cover them. The
tests were right; the spec was under-scoped. Widening the filter brought all four to CAUGHT with
no change to any assertion.

## Review

Gate: model (all four risk answers no). Model-gated because the two hazards this change
carries are both mechanically bounded: the disclosure hazard by a planted-secret row that
must stay green (row 5) and a negative control that stops the fix being "redact everything"
(row 4), and the regression hazard by byte-identity rows at three levels (3, 8, 9). The
reviewer confirms that no diagnostic surface reaches an operator without passing through
`Scrub`; that every retrofitted site's exit code is the one it produced before; that the
`— <tool> said: …` suffix is the ONLY change to default output; and that the runner was
lifted from the existing `toolMessage`/`runCmd` seam rather than written a second time.
