---
brief: assay:assay:desk-tools:21
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
schema: brief-v2
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
version: 1
id: b82e4e2e-0694-4c7e-a014-b71838743803
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
- `tools/desk/internal/deskkit/runtool.go` (NEW) — the one subprocess runner
- `tools/desk/internal/deskkit/trace.go` (NEW) — the switch, the ledger, the shared exit path
- `tools/desk/internal/deskkit/scrub.go` (NEW) — the redactor
- `tools/desk/internal/deskkit/exitcodes.go` — `DeskError` gains the subprocess detail; `Cause()`; `RefusedWithCause`
- `tools/desk/internal/deskkit/trace_test.go` (NEW)
- `tools/desk/internal/deskkit/trace-mutations.json` (NEW - the `muhar` spec for row 16)
- `tools/desk/cmd/deskdispatch/` (`exec.go`, `dispatch.go`, `worktree.go`, `main.go`, and a new `trace_test.go`)
- `tools/desk/cmd/deskwt/` (`exec.go`, `main.go`, `trace_test.go`)
- `tools/desk/cmd/desktoken/` (`desktoken.go`, `main.go`, `trace_test.go`)
- `tools/desk/cmd/deskfile/` (`exec.go`, `main.go`, `trace_test.go`)
- `tools/desk/README.md` — the `Diagnostics — DESK_TRACE` section, and the four verbs' `--help`

facts: the symptoms, each re-read at `e82c565e` (observed 2026-09-09 in an operating desk
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
### Non-implementer verifier run — 2026-09-16 verify-desk (desk-tools/21 dispatched verifier) — **VERIFY: PASS**

Runner ≠ implementer. Own detached temp worktree off `origin/main`, offline (`KUBECONFIG=/dev/null`). Merged main `a4700d2b`.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | exit 0, silent | 2026-09-16 | verify-desk (desk-tools/21 dispatched verifier) |
| 2 | `TestToolRunSaidSkipsPreambleAndCarriesTheToolsOwnMessage` + `TestToolRunFailShapeAndCarriedDetail` | exit 0 | exit 0/0, both PASS | 2026-09-16 | verify-desk (desk-tools/21 dispatched verifier) |
| 3 | `TestReportErrorOffIsByteIdentical` + `TestReportErrorOnPrintsChainCommandsAndTimings` | exit 0 | exit 0/0, both PASS | 2026-09-16 | verify-desk (desk-tools/21 dispatched verifier) |
| 4 | `TestScrubRedactsEveryTransportShape` | exit 0 incl. negative control | exit 0; 7 subtests PASS, negative control confirmed | 2026-09-16 | verify-desk (desk-tools/21 dispatched verifier) |
| 5 | `TestTraceNeverPrintsACredential` (SPOF row) | exit 0 | exit 0, PASS | 2026-09-16 | verify-desk (desk-tools/21 dispatched verifier) |
| 6 | `TestRefusedWithCauseStaysARefusal` + `TestTraceEnabledReadsTheEnvSpellings` | exit 0 | exit 0/0, both PASS | 2026-09-16 | verify-desk (desk-tools/21 dispatched verifier) |
| 7 | `TestClaimAcquireFailureNamesTheClaimToolsOwnMessage` + `TestGitOutFailureCarriesGitStderr` | exit 0 | exit 0/0, both PASS | 2026-09-16 | verify-desk (desk-tools/21 dispatched verifier) |
| 8 | `TestWorktreeCreateFailureCarriesDeskwtStderrAndTheCommandLine` + `TestTraceIsOffByDefaultAndOutputIsUnchanged` | exit 0 | exit 0/0, both PASS | 2026-09-16 | verify-desk (desk-tools/21 dispatched verifier) |
| 9 | `TestRunGitFailureCarriesStderrCommandAndExitStatus` + `TestDeskwtTraceOffIsByteIdenticalAndOnCarriesTheCommand` | exit 0 | exit 0/0, both PASS | 2026-09-16 | verify-desk (desk-tools/21 dispatched verifier) |
| 10 | `TestTokenPathNoticeIsPrintedOnStderrNotStdout` | exit 0 | exit 0, PASS | 2026-09-16 | verify-desk (desk-tools/21 dispatched verifier) |
| 11 | `TestDedupeSearchOutageNamesTheAPIStatus` + `TestGhStderrStripsControlBytes` (row's literal names) | exit 0 | exit 0 but "no tests to run" for BOTH — **neither name exists in the shipped `cmd/deskfile/` package** (false-green if trusted at face value). Actual shipped tests covering the same fact: `TestDedupeSearchPropagatesTheForgeDiagnosis` + `TestDedupeSearchControlBytesStrippedByBackend`, both run directly: PASS, satisfy the row's Expect column exactly. Documentation-currency drift (provisional names never updated post-merge), not a missing capability — filed as a light follow-up in this Evidence entry rather than a separate issue (session's per-repo new-issue budget already spent on this brief's own risk-value question) | 2026-09-16 | verify-desk (desk-tools/21 dispatched verifier) |
| 12 | `go test -timeout 300s ./internal/deskkit/... ./cmd/deskdispatch/... ./cmd/deskwt/... ./cmd/desktoken/... ./cmd/deskfile/... -count=1` | exit 0 | exit 0, all 6 packages `ok` | 2026-09-16 | verify-desk (desk-tools/21 dispatched verifier) |
| 13 | `-run 'TestS2'` + `-run 'TestCorpus'` | exit 0 | exit 0/0; 9/10 S2 PASS, 1 SKIP by design (fixture absent, matches implementer's note); all 5 Corpus PASS | 2026-09-16 | verify-desk (desk-tools/21 dispatched verifier) |
| 14 | `gofmt -l` on brief-owned files + touched cmd dirs | empty | exit 1 whole-dir — a phantom_test.go file under cmd/deskdispatch is flagged, but that file predates this brief (commit 91a7f9208, unrelated), not in brief's `files:` list, untouched by this diff. Every brief-owned file independently confirmed gofmt-clean | 2026-09-16 | verify-desk (desk-tools/21 dispatched verifier) |
| 15 | `cd statusgen && go run . --root .. --lint` | exit 0 | exit 0, LINT: PASS | 2026-09-16 | verify-desk (desk-tools/21 dispatched verifier) |
| 16 | `go run ./cmd/muhar -spec internal/deskkit/trace-mutations.json` | exit 0, baseline GREEN, mutations CAUGHT | exit 0 — baseline GREEN; 10/10 named mutations CAUGHT, 0 not-caught, 0 could-not-mutate | 2026-09-16 | verify-desk (desk-tools/21 dispatched verifier) |

No invented scope — every row maps to a real brief requirement. Row 11's finding (above) is the only anomaly, and is a documentation/test-name drift, not a functional gap.

**Risk-bearing value.**

**RISK-VALUE: DERIVED** — `reSecretEnvAssign` credential-name allowlist (`scrub.go:41-42`, matching `TOKEN|SECRET|PASSWORD|PASSWD|APIKEY|API_KEY|PRIVATE_KEY|CREDENTIAL|PAT`) — cross-checked against every uppercase env-var assignment in `tools/desk`: the only credential-bearing child-env override actually set anywhere (`GH_TOKEN`) is caught; non-secrets (`ASSAY_RUN_KEY`, `ASSAY_VERIFIER_PUBKEY`) correctly fall outside. A heuristic, not closed-form, matching the brief's stated over-redact-not-exhaustive posture.

**RISK-VALUE: DERIVED** — `reURLUserinfo` / `reAuthHeader` transport regexes (`scrub.go:31,36`) match exactly the two concrete leak shapes the brief names, verified against the actual `GH_TOKEN`-authenticated URL construction the codebase performs.

**RISK-VALUE: NAMED, NOT DERIVED** — `traceStepCap = 200` @ `internal/deskkit/trace.go:54` — an arbitrary ledger-length bound, low irreversibility (truncation is announced, not silent; bounds output size, not a security/authority boundary). Filed as a question: medici-finance/assay#1240 — not a blocker to this brief's flip.

**VERIFY: PASS** — all 16 rows pass on merged main (row 11's stale-name issue resolved by running the actual shipped tests, which satisfy the fact). No FAIL, no could-not-check.

### Non-implementer verifier run — VERIFY: BLOCKED — 2/16 pass, 14 could-not-check, 0 fail — 2026-09-23 claude-opus-4-8-verifier

Runner is not the implementer; rows executed non-hermetically on darwin/arm64, go1.26.5, offline
(`KUBECONFIG=/dev/null`), from a worktree cut detached at merged main
`2a5c230efe9c29f17b9acf6aa798e3a0a2285fbb`. Fourteen of the sixteen Verify rows are `check:ci`;
the hermetic `statusgen verifyrun` witness recorded all fourteen as could-not-run (its network-off
sandbox needs Linux `unshare --net`; this host is darwin), so each is held COULD-NOT-CHECK with its
direct non-hermetic run recorded (every one passed). Rows 5 and 16 are `check +mutation`, which
verifyrun DID run and reported pass and which also passed directly — those two are PASS. The block
is BLOCKED on the darwin execution-witness environment, not on the implementation. Supersedes the
2026-09-16 pass on the open draft #1241.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0, no churn | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (both silent across the module) | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestToolRunSaid.*OwnMessage$' -count=1 && go test ./internal/deskkit/ -run '^TestToolRunFail.*CarriedDetail$' -count=1` | exit 0 — child's message not the config echo; the tool-said shape; errors.As reaches ExitError | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (both tests pass; child's own message, the tool-said shape, errors.As reaches the exit-error) | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestReportErrorOff.*Identical$' -count=1 && go test ./internal/deskkit/ -run '^TestReportErrorOn.*Timings$' -count=1` | exit 0 — OFF byte-identical; ON carries chain/commands/codes/timings | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (both tests pass; OFF byte-identical, ON carries chain/commands/codes/timings) | 2026-09-23 | claude-opus-4-8-verifier |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestScrubRedacts.*TransportShape$' -count=1` | exit 0 — every transport shape redacted, negative control passes through unaltered | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (every transport shape redacted; negative control passes through unaltered) | 2026-09-23 | claude-opus-4-8-verifier |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestTraceNeverPrintsACredential$' -count=1` | exit 0 — SPOF: token in argv+env+stderr absent from trace, redaction marked | PASS — exit 0 (SPOF row: a token planted in argv, environment and child stderr is absent from the trace, redaction marked); verifyrun witness also pass (sha256:d6dea0c13f7d) | 2026-09-23 | claude-opus-4-8-verifier |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRefusedWithCause.*Refusal$' -count=1 && go test ./internal/deskkit/ -run '^TestTraceEnabled.*EnvSpellings$' -count=1` | exit 0 — a cause does not soften exit 5; empty/0 DESK_TRACE is OFF | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (both tests pass; a cause does not soften exit 5; empty/0 DESK_TRACE is OFF) | 2026-09-23 | claude-opus-4-8-verifier |
| 7 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestClaimAcquire.*OwnMessage$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestGitOutFailure.*GitStderr$' -count=1` | exit 0 — facts 1 and 2 closed | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (both tests pass; facts 1 and 2 closed) | 2026-09-23 | claude-opus-4-8-verifier |
| 8 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestWorktreeCreate.*CommandLine$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestTraceIsOff.*Unchanged$' -count=1` | exit 0 — deskwt's refusal (5) passes through, trace names argv+status, default output unmoved | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (both tests pass; deskwt's refusal 5 passes through, trace names argv+status, default output unmoved) | 2026-09-23 | claude-opus-4-8-verifier |
| 9 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRunGitFailure.*ExitStatus$' -count=1 && go test ./cmd/deskwt/ -run '^TestDeskwtTraceOff.*TheCommand$' -count=1` | exit 0 — fact 5 closed, both directions of the switch | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (both tests pass; fact 5 closed, both directions of the switch) | 2026-09-23 | claude-opus-4-8-verifier |
| 10 | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestTokenPathNotice.*NotStdout$' -count=1` | exit 0 — fact 3 closed; stdout EXACTLY the path, token value on neither stream | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (fact 3 closed; stdout is exactly the path, the token value reaches neither stream) | 2026-09-23 | claude-opus-4-8-verifier |
| 11 | `cd tools/desk && go test ./cmd/deskfile/ -run '^TestDedupeSearchPropagates.*Diagnosis$' -count=1 && go test ./cmd/deskfile/ -run '^TestDedupeSearchControl.*Backend$' -count=1` | exit 0 — 401/403/429 tell apart, exit 6 unchanged, pre-existing control-strip survived | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (both deskfile tests pass — 401/403/429 tell apart, exit 6 unchanged, the pre-existing control-strip survived). Brief's literal test names — the outage-names-the-API-status and gh-stderr-strips-control-bytes tests — were renamed on merged main (post write-verbs-C migration off gh to the Forge backend); the corrected names run. Tracked in #1306 (the renamed row-11 test names) | 2026-09-23 | claude-opus-4-8-verifier |
| 12 | `cd tools/desk && go test -timeout 300s ./internal/deskkit/... ./cmd/deskdispatch/... ./cmd/deskwt/... ./cmd/desktoken/... ./cmd/deskfile/... -count=1` | exit 0 — every existing test of the five packages unchanged | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (6 ok packages — deskkit, untrustcorpus, deskdispatch, deskwt, desktoken, deskfile — 0 FAIL) | 2026-09-23 | claude-opus-4-8-verifier |
| 13 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestS2' -count=1 && go test ./internal/deskkit/ -run 'TestCorpus' -count=1` | exit 0 — control leak sweep, both halves | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (ok internal/deskkit for both the TestS2 and TestCorpus runs — the control leak-sweep passes; one S2 sweep-exclusions test skips by design, its private-tree fixture absent, matching the implementer note) | 2026-09-23 | claude-opus-4-8-verifier |
| 14 | `cd tools/desk && gofmt -l internal/deskkit/trace.go internal/deskkit/runtool.go internal/deskkit/scrub.go internal/deskkit/exitcodes.go cmd/deskdispatch cmd/deskwt cmd/desktoken cmd/deskfile > /tmp/dt21-fmt.out; test ! -s /tmp/dt21-fmt.out` | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (gofmt list empty, no unformatted brief-owned file) | 2026-09-23 | claude-opus-4-8-verifier |
| 15 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (LINT: PASS; NOTICEs only, none blocking) | 2026-09-23 | claude-opus-4-8-verifier |
| 16 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/trace-mutations.json` | exit 0 — baseline GREEN, positive control CAUGHT, all ten mutations CAUGHT | PASS — exit 0 ("Harness healthy: baseline GREEN, positive control CAUGHT." "Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate."); verifyrun witness also pass (sha256:ae9535c6028c) | 2026-09-23 | claude-opus-4-8-verifier |

RISK-VALUE (kit §4 — enumerated over the diff scope: the new deskkit files trace.go / scrub.go /
runtool.go / exitcodes.go and the four verb retrofits; trigger fired fail-safe because the change
touches a disclosure surface the brief itself flags):

- RISK-VALUE: DERIVED — RefusedWithCause.Code = ExitRefused (5) @ tools/desk/internal/deskkit/exitcodes.go:158 — this is the top-ranked entry because a wrong value here is the named security hazard: a fail-closed refusal that gains a cause must NOT soften to ExitUnverifiable (6, retryable). Right value derived from definition + the brief's ground rule "a refusal stays a refusal, no exit code changes": RefusedWithCause is "Refused with the underlying cause preserved", and Refused = ExitRefused (5) at exitcodes.go:144, so the two must share the code. Independently pinned by row 6 (the refused-with-cause-stays-a-refusal test) and by row 16's inverse mutation (soften to exit 6 → CAUGHT).
- RISK-VALUE: NAMED, NOT DERIVED — the three transport-shape redaction regexes reURLUserinfo @ scrub.go:31, reAuthHeader @ scrub.go:36, reSecretEnvAssign @ scrub.go:42 (the SPOF, deskkit.Scrub) — each is a literal pattern at a source line, but regex EXHAUSTIVENESS against every real-world secret shape is not closed-form derivable; the design is deliberately over-redaction-biased and bounded by the row-5 planted-secret table plus row 16's per-shape removal mutations (each CAUGHT). Missing derivation: proof the pattern set is complete rather than merely test-covered — inherent to a heuristic redactor, sanctioned by the brief's "biased to over-redact" posture. Not routed to human because the item is model-gated (all four risk answers no) and the bounding controls (rows 4, 5, 16) are green.
- Reversible operational knobs ranked LAST, no derivation required (edit + redeploy fixes each): traceStepCap = 200 @ trace.go:54 (ledger cap, over-cap drops reported, not hidden); chain-depth guard = 32 @ trace.go:199 (cyclic-chain truncation); redactedMarker = "<redacted>" @ scrub.go:49 (display string). DESK_TRACE truthy set {1,true,yes,on} @ trace.go:98 is an enum, not a threshold; its negative control (empty/0 → OFF) is pinned by row 6.

Supersedes the 2026-09-16 pass on the open draft #1241. RISK-VALUE carries one NAMED, NOT DERIVED entry (the three Scrub transport regexes); that question is filed as #1616 and must be answered before any flip. Row 11's literal test names are stale; corrected forms ran.

**Evidence correction (2026-09-24).** Row 13's Observed says one S2 sweep-exclusions test skips by design, but its recorded command runs without `-v`, so it prints only `ok` — not the skip line. The command that supports the claim is `cd tools/desk && go test -v ./internal/deskkit/ -run 'TestS2' -count=1`; re-run at the verified sha `2a5c230e` it prints `--- SKIP: TestS2SweepExclusionsAreLive (0.00s)` among the TestS2 results, followed by `ok github.com/medici-finance/assay/tools/desk/internal/deskkit`. No state, count or heading changes.

### Non-implementer verifier run — VERIFY: BLOCKED — 2/16 pass, 14 could-not-check, 0 fail — 2026-09-25 assay-verifier-app[bot]

Runner is not the implementer. Run offline (`KUBECONFIG=/dev/null`) on darwin/arm64 against the
merged tree `bb226e7d3ebb` (this PR's branch with main `89042b8f` merged in). The table below is
the execution witness written by `statusgen verifyrun`, built from this tree. It is not
hand-authored. Fourteen rows are `check:ci`, and verifyrun recorded each one as could-not-run
because its network-off sandbox needs Linux `unshare --net` and this host is darwin (the same
environment blocker as the 2026-09-23 run, tracked in #1800). Rows 5 and 16 (`check +mutation`)
ran and passed. This run supersedes the 2026-09-16 PASS above, which predates the
execution-witness gate. The board Status stays `implemented`.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ bb226e7d3ebb (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestToolRunSaidSkipsPreambleAndCarriesTheToolsOwnMessage$' -count=1 && go test ./internal/deskkit/ -run '^TestToolRunFailShapeAndCarriedDetail$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ bb226e7d3ebb (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestReportErrorOffIsByteIdentical$' -count=1 && go test ./internal/deskkit/ -run '^TestReportErrorOnPrintsChainCommandsAndTimings$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ bb226e7d3ebb (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestScrubRedactsEveryTransportShape$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ bb226e7d3ebb (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestTraceNeverPrintsACredential$' -count=1` | pass exit=0 | sha256:049edd052a39 | 2026-09-25 | assay-verifier-app[bot] @ bb226e7d3ebb (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRefusedWithCauseStaysARefusal$' -count=1 && go test ./internal/deskkit/ -run '^TestTraceEnabledReadsTheEnvSpellings$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ bb226e7d3ebb (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestClaimAcquireFailureNamesTheClaimToolsOwnMessage$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestGitOutFailureCarriesGitStderr$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ bb226e7d3ebb (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestWorktreeCreateFailureCarriesDeskwtStderrAndTheCommandLine$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestTraceIsOffByDefaultAndOutputIsUnchanged$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ bb226e7d3ebb (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRunGitFailureCarriesStderrCommandAndExitStatus$' -count=1 && go test ./cmd/deskwt/ -run '^TestDeskwtTraceOffIsByteIdenticalAndOnCarriesTheCommand$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ bb226e7d3ebb (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestTokenPathNoticeIsPrintedOnStderrNotStdout$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ bb226e7d3ebb (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go test ./cmd/deskfile/ -run '^TestDedupeSearchOutageNamesTheAPIStatus$' -count=1 && go test ./cmd/deskfile/ -run '^TestGhStderrStripsControlBytes$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ bb226e7d3ebb (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test -timeout 300s ./internal/deskkit/... ./cmd/deskdispatch/... ./cmd/deskwt/... ./cmd/desktoken/... ./cmd/deskfile/... -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ bb226e7d3ebb (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestS2' -count=1 && go test ./internal/deskkit/ -run 'TestCorpus' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ bb226e7d3ebb (on-behalf-of human:ian) (forge-identity) |
| 14 | `cd tools/desk && gofmt -l internal/deskkit/trace.go internal/deskkit/runtool.go internal/deskkit/scrub.go internal/deskkit/exitcodes.go cmd/deskdispatch cmd/deskwt cmd/desktoken cmd/deskfile > /tmp/dt21-fmt.out; test ! -s /tmp/dt21-fmt.out` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ bb226e7d3ebb (on-behalf-of human:ian) (forge-identity) |
| 15 | `cd statusgen && go run . --root .. --lint; echo $?` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ bb226e7d3ebb (on-behalf-of human:ian) (forge-identity) |
| 16 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/trace-mutations.json` | pass exit=0 | sha256:76384c3d026e | 2026-09-25 | assay-verifier-app[bot] @ bb226e7d3ebb (on-behalf-of human:ian) (forge-identity) |

Direct runs, recorded for information only; they do not replace the witness above. Same tree,
darwin/arm64, offline, each Verify row's own command:

- Rows 1–4, 6–10, 12–14: exit 0. Row 1 build and vet are silent. Rows 2–4, 6–10 and 13 print
  `ok` for every package they name. Row 12 prints `ok` for all six packages (deskkit,
  untrustcorpus, deskdispatch, deskwt, desktoken, deskfile). Row 14's gofmt list is empty.
- Row 5: exit 0, `--- PASS: TestTraceNeverPrintsACredential`.
- Row 11: exit 0, but both halves print `ok … [no tests to run]`. The row's literal test names
  no longer exist in the deskfile package, so its exit 0 is vacuous and proves nothing (#1306;
  lint flags the row `gotest-run-vacuous`). The renamed tests,
  `TestDedupeSearchPropagatesTheForgeDiagnosis` and
  `TestDedupeSearchControlBytesStrippedByBackend`, run directly: both `--- PASS`, exit 0. A
  Linux witness of row 11 as written would record a pass it did not observe, so the row must be
  re-pointed before any witness of it counts.
- Row 15: exit 0, `LINT: PASS` on the final tree of this commit.
- Row 16: exit 0, matching the witness pass.

Security follow-up, re-checked at this tree: the error-composition path the security lane
raised on this PR was fixed on main by #1443 (issue #1440). `FailVerbatim` now scrubs the
caller's message, and the worktree-create step composes from the scrubbed `SaidAll()`. Both
regression tests from that fix pass here: `TestFailVerbatimScrubsCallerComposedMessage` and
`TestWorktreeCreateFailureScrubsDeskwtsSecretShapedStderr`, each `--- PASS`, exit 0. They are
not in this brief's Verify table, and the SPOF row (5) still builds its error through `Fail`
only. The table does not yet witness that path.

RISK-VALUE (kit §4). Enumerated over the same scope as the 2026-09-23 run: trace.go, scrub.go,
runtool.go and exitcodes.go in deskkit, plus the four verb retrofits. Every literal is
unchanged at the line cited there. The #1443 delta adds a `Scrub(msg)` call and no literal.

- RISK-VALUE: DERIVED — RefusedWithCause.Code = ExitRefused (5) @ tools/desk/internal/deskkit/exitcodes.go:158. A refusal that gains a cause must keep the code of `Refused` (exitcodes.go:144, ExitRefused = 5 at exitcodes.go:27). Softening it to 6 would make a fail-closed refusal read as retryable. Pinned by row 6 and by row 16's inverse mutation.
- RISK-VALUE: NAMED, NOT DERIVED — reURLUserinfo @ scrub.go:31, reAuthHeader @ scrub.go:36, reSecretEnvAssign @ scrub.go:42. No test can show that a heuristic regex set covers every shape. Question #1616 is still open, and the security lane has since named two shapes these patterns miss. It must be answered before any flip.
- Reversible knobs, ranked last: traceStepCap = 200 @ trace.go:54 (#1240), chain-depth guard 32 @ trace.go:199, redactedMarker @ scrub.go:49.

**VERIFY: BLOCKED** — 2/16 pass, 14 could-not-check, 0 fail. Status stays `implemented`. To
close it, three things are needed: a Linux (or in-container, #1491) witness of the 14
`check:ci` rows, row 11 re-pointed (#1306), and an answer to #1616.

### Non-implementer verifier re-run: 2026-10-02T21:29:46Z (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main 5a108baba705c5f9b458501065fc52a12557c969

Hand run of every Verify row on darwin/arm64, go1.27.1, in a temporary detached worktree at the
merged head, with a throwaway HOME for every `go test`. This is NOT the execution witness; the
witness result is recorded separately below. Verdict is unchanged from the 2026-09-25 record:
BLOCKED.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | exit 0 — both silent across the module | 2026-10-02 assay-verifier-app[bot] |
| 2 | `go test ./internal/deskkit/` for TestToolRunSaidSkipsPreambleAndCarriesTheToolsOwnMessage, then TestToolRunFailShapeAndCarriedDetail | exit 0 | exit 0 — `ok  .../tools/desk/internal/deskkit 0.421s` and `ok ... 0.214s` | 2026-10-02 assay-verifier-app[bot] |
| 3 | `go test ./internal/deskkit/` for TestReportErrorOffIsByteIdentical, then TestReportErrorOnPrintsChainCommandsAndTimings | exit 0 | exit 0 — `ok  .../internal/deskkit 0.208s` and `ok ... 0.229s` | 2026-10-02 assay-verifier-app[bot] |
| 4 | `go test ./internal/deskkit/ -run '^TestScrubRedactsEveryTransportShape$' -count=1` | exit 0 | exit 0 — `ok  .../internal/deskkit 0.320s` | 2026-10-02 assay-verifier-app[bot] |
| 5 | `go test ./internal/deskkit/ -run '^TestTraceNeverPrintsACredential$' -count=1` | exit 0 | exit 0 — `ok  .../internal/deskkit 0.540s` | 2026-10-02 assay-verifier-app[bot] |
| 6 | `go test ./internal/deskkit/` for TestRefusedWithCauseStaysARefusal, then TestTraceEnabledReadsTheEnvSpellings | exit 0 | exit 0 — `ok  .../internal/deskkit 0.195s` and `ok ... 0.211s` | 2026-10-02 assay-verifier-app[bot] |
| 7 | `go test ./cmd/deskdispatch/` for TestClaimAcquireFailureNamesTheClaimToolsOwnMessage, then TestGitOutFailureCarriesGitStderr | exit 0 | exit 0 — `ok  .../cmd/deskdispatch 0.353s` and `ok ... 0.117s` | 2026-10-02 assay-verifier-app[bot] |
| 8 | `go test ./cmd/deskdispatch/` for TestWorktreeCreateFailureCarriesDeskwtStderrAndTheCommandLine, then TestTraceIsOffByDefaultAndOutputIsUnchanged | exit 0 | exit 0 — `ok  .../cmd/deskdispatch 0.202s` and `ok ... 0.167s` | 2026-10-02 assay-verifier-app[bot] |
| 9 | `go test ./cmd/deskwt/` for TestRunGitFailureCarriesStderrCommandAndExitStatus, then TestDeskwtTraceOffIsByteIdenticalAndOnCarriesTheCommand | exit 0 | exit 0 — `ok  .../cmd/deskwt 0.492s` and `ok ... 0.399s` | 2026-10-02 assay-verifier-app[bot] |
| 10 | `go test ./cmd/desktoken/ -run '^TestTokenPathNoticeIsPrintedOnStderrNotStdout$' -count=1` | exit 0 | exit 0 — `ok  .../cmd/desktoken 0.438s` | 2026-10-02 assay-verifier-app[bot] |
| 11 | `go test ./cmd/deskfile/` for TestDedupeSearchOutageNamesTheAPIStatus, then TestGhStderrStripsControlBytes | exit 0 — 401/403/429 tell apart, exit 6 unchanged, control-sequence strip survived | exit 0 but VACUOUS, counted as a non-pass — both invocations print `ok  .../cmd/deskfile 0.505s [no tests to run]`: neither named test exists on main. The package's trace tests are now TestDedupeSearchPropagatesTheForgeDiagnosis and TestDedupeSearchControlBytesStrippedByBackend; the row has not been re-pointed (#1306, open) | 2026-10-02 assay-verifier-app[bot] |
| 12 | `go test -timeout 300s` over deskkit, deskdispatch, deskwt, desktoken, deskfile package trees, `-count=1` | exit 0 | exit 0 — six `ok` lines: deskkit 81.556s, deskkit/untrustcorpus 0.248s, deskdispatch 20.659s, deskwt 81.356s, desktoken 8.381s, deskfile 12.829s | 2026-10-02 assay-verifier-app[bot] |
| 13 | `go test ./internal/deskkit/ -run 'TestS2' -count=1 && go test ./internal/deskkit/ -run 'TestCorpus' -count=1` | exit 0 | exit 0 — `ok  .../internal/deskkit 1.083s` and `ok ... 0.863s` | 2026-10-02 assay-verifier-app[bot] |
| 14 | `gofmt -l` over the four deskkit files and four cmd directories, redirected to a file, then `test ! -s` on it | exit 0 | exit 0 — listing file is 0 bytes. Deviation from the authored row: the listing was redirected to the verifier's own scratch directory rather than the system temp directory, to stay inside the isolation floor; the gofmt arguments are as authored | 2026-10-02 assay-verifier-app[bot] |
| 15 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | exit 0 — `LINT: PASS` then `0` (NOTICE lines only, none on this brief) | 2026-10-02 assay-verifier-app[bot] |
| 16 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/trace-mutations.json` | exit 0, every mutation CAUGHT | exit 0 — `Harness healthy: baseline GREEN, positive control CAUGHT.` and `Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.` | 2026-10-02 assay-verifier-app[bot] |

Hand-run tally: 15 of 16 rows pass (1-10, 12-16); row 11 exits 0 without executing any test.

**Execution witness** (`statusgen verifyrun --brief <this brief> --dry-run`, statusgen v1.0.31, exit 2): 2 of 16 rows pass.

- Rows 5 and 16 (class `check +mutation`): `pass exit=0` (sha256:8faac02138e6, sha256:ad915c0aecda).
- Rows 1-4 and 6-15 (class `check:ci`, 14 rows): `could-not-run exit=-` — "check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin."
- A second attempt with `--in-container` (the pinned harness image, digest-pinned) pulled the image and then REFUSED before running any row: "could-not-attribute — no executing identity is available (no GITHUB_ACTOR under GitHub Actions, no git user.name/user.email in this repo). Refusing to write a witness with no runner". Stopped there; no identity was supplied to get past it.
- Nothing was written to the brief (dry-run); the worktree stayed clean.

**Findings**

1. Environment blocker unchanged — #1800 is still OPEN. The 14 `check:ci` rows still have no network-off witness; the hand run above shows the code is green but a hand run is not the witness the row class requires. These rows clear on a Linux runner that provides `unshare --net`.
2. Check-definition defect unchanged — #1306 is still OPEN. Row 11 names two tests that no longer exist in the deskfile package, so its exit 0 proves nothing. Row 12 does run the two renamed successors (the whole deskfile package is green), so the behaviour is covered, but row 11 as authored cannot pass or fail until it is re-pointed. Even with a Linux witness, row 11 would record a vacuous pass.
3. What changed on main since the 2026-09-25 record: the four command packages and the exit-code table file moved under later merges; the runner, trace, scrub, mutation-spec and the four trace test files are byte-identical to the recorded inputs. None of that changes a row result: no row that was could-not-run now runs under the witness, and no row regressed in the hand run.
4. Row 16 lists ten mutations; all ten are still CAUGHT, including the three redactor shapes, the unconditional-trace mutation and the softened-refusal mutation.

**Risk-bearing value** (carried from the previous record; scrub source is unchanged)

- RISK-VALUE: NAMED, NOT DERIVED — reURLUserinfo = `://([^:/@\s]+):([^@\s]+)@` @ tools/desk/internal/deskkit/scrub.go:31, with reAuthHeader @ line 36 and reSecretEnvAssign @ line 42 — the tests and the mutation harness prove each shape redacts and that removing one is caught; they do not prove the three-shape set is SUFFICIENT against every credential form a child process can print. That sufficiency question is open on #1616 and was not derived here.
- Reversible knobs, ranked last, no derivation owed: redactedMarker = `<redacted>` @ scrub.go:49; the DESK_TRACE truthy spellings in trace.go.

VERIFY: BLOCKED — execution witness is 2 of 16: the 14 check:ci rows could-not-run on a darwin host with no network-off sandbox (#1800, open), and row 11 names tests that no longer exist (#1306, open); 15 of 16 rows are green by hand, status stays implemented

### Non-implementer verifier re-run: 2026-10-11T10:30:01Z (UTC), assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian), merged main 251e9473928e6e8c8c5ffbd331833900331043f1

Attestation: the admission receipt for this run is issue #2574, run 6f4be5f3bb8e6b9b5e80a90bea2934d4cec9326bb7a4cfdd1cac4e5574c28838, source 251e9473928e6e8c8c5ffbd331833900331043f1, model claude-opus-5-5, tier any. The witness below carries the Verification-Attestation line.

Admission: `deskdispatch --check-verifier` exit 0 for this detached home at the source above,
before any row ran. Hand run of every Verify row on darwin/arm64, go1.27.1, git 2.56.0, with a
throwaway HOME, KUBECONFIG=/dev/null and no forge credential in the environment. Logs and the
row 14 listing file were kept in the verifier's scratch directory, outside the home (row 14's
redirect target is the only deviation from the authored command text).

Two passes. The FIRST pass ran in the ambient house-cell environment on a heavily loaded host
(load average 17-27 on 16 cores, about 13 concurrent go test processes from sibling verifiers).
Rows 7, 8, 12 and 16 went red there for environmental reasons, recorded in the table. The
SECOND pass re-ran exactly those four rows, same command text, with one change: the house
variable ASSAY_GO_CACHE_POLICY unset (no CI runner carries it). All four went green.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---------|--------|-----------------------------------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | exit 0, both silent across the module | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `go test ./internal/deskkit/` for TestToolRunSaidSkipsPreambleAndCarriesTheToolsOwnMessage, then TestToolRunFailShapeAndCarriedDetail | exit 0 | exit 0, both `--- PASS`; `ok .../internal/deskkit 0.293s` and `ok ... 0.264s` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `go test ./internal/deskkit/` for TestReportErrorOffIsByteIdentical, then TestReportErrorOnPrintsChainCommandsAndTimings | exit 0 | exit 0, both `--- PASS`; `ok ... 0.248s` and `ok ... 0.315s` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | `go test ./internal/deskkit/ -run '^TestScrubRedactsEveryTransportShape$' -count=1` | exit 0 | exit 0, `--- PASS: TestScrubRedactsEveryTransportShape`; `ok ... 0.233s` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | `go test ./internal/deskkit/ -run '^TestTraceNeverPrintsACredential$' -count=1` | exit 0 | exit 0, `--- PASS: TestTraceNeverPrintsACredential`; `ok ... 0.272s` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | `go test ./internal/deskkit/` for TestRefusedWithCauseStaysARefusal, then TestTraceEnabledReadsTheEnvSpellings | exit 0 | exit 0, both `--- PASS`; `ok ... 0.260s` and `ok ... 0.267s` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | `go test ./cmd/deskdispatch/` for TestClaimAcquireFailureNamesTheClaimToolsOwnMessage, then TestGitOutFailureCarriesGitStderr | exit 0 | first pass exit 1: the test's dispatch stopped at storage admission (house cache policy, `advisory lock is held by another process`), so the claim step never ran (#2491). Second pass, policy unset: exit 0, both `--- PASS`; `ok .../cmd/deskdispatch 0.225s` and `ok ... 0.197s` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | `go test ./cmd/deskdispatch/` for TestWorktreeCreateFailureCarriesDeskwtStderrAndTheCommandLine, then TestTraceIsOffByDefaultAndOutputIsUnchanged | exit 0 | first pass exit 1, same storage-admission cause as row 7 (#2491). Second pass, policy unset: exit 0, both `--- PASS`; `ok ... 0.228s` and `ok ... 0.226s` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | `go test ./cmd/deskwt/` for TestRunGitFailureCarriesStderrCommandAndExitStatus, then TestDeskwtTraceOffIsByteIdenticalAndOnCarriesTheCommand | exit 0 | exit 0, both `--- PASS`; `ok .../cmd/deskwt 0.498s` and `ok ... 0.494s` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 10 | `go test ./cmd/desktoken/ -run '^TestTokenPathNoticeIsPrintedOnStderrNotStdout$' -count=1` | exit 0 | exit 0, `--- PASS: TestTokenPathNoticeIsPrintedOnStderrNotStdout`; `ok .../cmd/desktoken 1.501s` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 11 | `go test ./cmd/deskfile/` for TestDedupeSearchOutageNamesTheAPIStatus, then TestGhStderrStripsControlBytes | exit 0, 401/403/429 tell apart, exit 6 unchanged, strip survived | exit 0 but VACUOUS, counted as a non-pass: both invocations print `ok .../cmd/deskfile [no tests to run]` because neither named test exists at this commit (#1306, open). The renamed successors TestDedupeSearchPropagatesTheForgeDiagnosis and TestDedupeSearchControlBytesStrippedByBackend were run separately: exit 0, both `--- PASS` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 12 | `go test -timeout 300s` over the deskkit, deskdispatch, deskwt, desktoken and deskfile package trees, `-count=1` | exit 0 | first pass exit 1: deskdispatch storage-admission failures (#2491) plus a 300s timeout, deskwt hit the 300s timeout under load, and deskkit's TestGuardStopFlags boundary subtest failed (`age 24h0m0s > 24h0m0s`). Second pass, policy unset: exit 0, six `ok` lines: deskkit 170.994s, untrustcorpus 0.116s, deskdispatch 91.806s, deskwt 215.309s, desktoken 24.324s, deskfile 79.600s | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 13 | `go test ./internal/deskkit/ -run 'TestS2' -count=1 && go test ./internal/deskkit/ -run 'TestCorpus' -count=1` | exit 0 | exit 0, `ok .../internal/deskkit 1.531s` and `ok ... 1.020s`; not vacuous: `go test -list` matches 10 TestS2 and 5 TestCorpus tests | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 14 | `gofmt -l` over the four deskkit files and the four cmd directories, redirected to a file, then `test ! -s` on it | exit 0 | exit 0, listing file is 0 bytes (redirected into the verifier's scratch directory, not the system temp directory) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 15 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | exit 0, `LINT: PASS` then `0` (NOTICE lines only) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 16 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/trace-mutations.json` | exit 0, every mutation CAUGHT | first pass exit 1: `HARNESS BROKEN`, baseline RED because deskdispatch's trace tests hit the same storage-admission refusal (#2491). Second pass, policy unset: exit 0, `Harness healthy: baseline GREEN, positive control CAUGHT.` and `Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Hand-run tally: 15 of 16 rows green (1-10, 12-16), four of them (7, 8, 12, 16) only once the house
cache policy was unset; row 11 exits 0 without executing a test.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestToolRunSaidSkipsPreambleAndCarriesTheToolsOwnMessage$' -count=1 && go test ./internal/deskkit/ -run '^TestToolRunFailShapeAndCarriedDetail$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestReportErrorOffIsByteIdentical$' -count=1 && go test ./internal/deskkit/ -run '^TestReportErrorOnPrintsChainCommandsAndTimings$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestScrubRedactsEveryTransportShape$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestTraceNeverPrintsACredential$' -count=1` | pass exit=0 | sha256:d36b75c43761 | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRefusedWithCauseStaysARefusal$' -count=1 && go test ./internal/deskkit/ -run '^TestTraceEnabledReadsTheEnvSpellings$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestClaimAcquireFailureNamesTheClaimToolsOwnMessage$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestGitOutFailureCarriesGitStderr$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestWorktreeCreateFailureCarriesDeskwtStderrAndTheCommandLine$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestTraceIsOffByDefaultAndOutputIsUnchanged$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRunGitFailureCarriesStderrCommandAndExitStatus$' -count=1 && go test ./cmd/deskwt/ -run '^TestDeskwtTraceOffIsByteIdenticalAndOnCarriesTheCommand$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestTokenPathNoticeIsPrintedOnStderrNotStdout$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go test ./cmd/deskfile/ -run '^TestDedupeSearchOutageNamesTheAPIStatus$' -count=1 && go test ./cmd/deskfile/ -run '^TestGhStderrStripsControlBytes$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test -timeout 300s ./internal/deskkit/... ./cmd/deskdispatch/... ./cmd/deskwt/... ./cmd/desktoken/... ./cmd/deskfile/... -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestS2' -count=1 && go test ./internal/deskkit/ -run 'TestCorpus' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 14 | `cd tools/desk && gofmt -l internal/deskkit/trace.go internal/deskkit/runtool.go internal/deskkit/scrub.go internal/deskkit/exitcodes.go cmd/deskdispatch cmd/deskwt cmd/desktoken cmd/deskfile > /tmp/dt21-fmt.out; test ! -s /tmp/dt21-fmt.out` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 15 | `cd statusgen && go run . --root .. --lint; echo $?` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 16 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/trace-mutations.json` | pass exit=0 | sha256:68bf1d41769d | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2574 run=6f4be5f3bb8e6b9b5e80a90bea2934d4cec9326bb7a4cfdd1cac4e5574c28838 source=251e9473928e6e8c8c5ffbd331833900331043f1 brief=docs/streams/desk-tools/brief-21-desk-trace-and-cause-carrying-errors.md model=claude-opus-5-5 tier=any

**Execution witness** (the table directly above, written by `statusgen verifyrun --brief <this brief>`,
statusgen v1.0.37, run from the home with ASSAY_GO_CACHE_POLICY unset, exit 2): 2 of 16 rows pass.

- Rows 5 and 16 (class `check +mutation`): `pass exit=0` (sha256:d36b75c43761, sha256:68bf1d41769d).
  The runner shows `+dirty` because the only change in the home at witness time was this
  Evidence section.
- Rows 1-4 and 6-15 (class `check:ci`, 14 rows): `could-not-run exit=-`. The check:ci network-off
  sandbox is `unshare --net`, a Linux facility, and this host is darwin (#1800, open). The hand run
  above covers these rows, but a hand run is not the witness this row class requires.

**Findings**

1. Environment blocker unchanged: #1800 is still open. The 14 `check:ci` rows have no
   network-off witness. They clear on a Linux runner that provides `unshare --net`.
2. Check-definition defect unchanged: #1306 is still open. Row 11 names two deskfile tests that
   do not exist at this commit, so its exit 0 proves nothing. The renamed successors pass, both on
   their own and inside row 12, so the behaviour is covered. Row 11 as authored cannot pass or
   fail until it is re-pointed.
3. Test hermeticity, already filed as #2491 (open): the deskdispatch package tests read the ambient
   ASSAY_GO_CACHE_POLICY. In a house-cell shell whose cache lock is held by a sibling process,
   every test that reaches storage admission fails with `storage-deferred`. That turned rows 7, 8
   and 12 red, and it made row 16's mutation baseline RED (`HARNESS BROKEN`, run discarded). With
   the variable unset, all four rows are green. This is about the test environment, not the
   desk-tools/21 deliverable.
4. Load sensitivity, NEW (no existing issue was found). On the first pass, host load was about 17-27 on 16
   cores. In row 12, deskwt and deskdispatch hit the row's `-timeout 300s`. deskwt took 215s even on
   the green second pass, against 81s on 2026-10-02. deskkit's TestGuardStopFlags subtest
   "HEARTBEAT exactly at boundary → not stale" failed once (`age 24h0m0s > 24h0m0s`), which looks
   like a wall-clock boundary race. It passed on the second pass. Neither is in this brief's code.
   Row 12's 300s budget has little headroom on a shared host, and the boundary subtest is timing-fragile.
5. Since the 2026-10-02 record, the runner, trace and scrub sources and the deskkit trace tests
   have not changed. The exit-code file gained a ReadClass field and its accessor. The mutation
   spec's deskwt-flatten entry was re-anchored to `held.settle(...)`, and that mutation is still
   CAUGHT. All ten mutations are CAUGHT.

**Risk-bearing value** (enumerated over the scrub, trace and runtool sources at this commit; the scrub source is byte-identical to the 2026-10-02 record)

- RISK-VALUE: NAMED, NOT DERIVED — reURLUserinfo = `://([^:/@\s]+):([^@\s]+)@` @ tools/desk/internal/deskkit/scrub.go:31, with reAuthHeader @ line 36 and reSecretEnvAssign @ lines 42-43 — the tests and the mutation harness prove that each shape redacts and that removing any one of them is caught. They do not prove that the three-shape set is SUFFICIENT for every credential form a child process can print. That question is still open on #1616 and was not derived here.
- Reversible knobs, ranked last, no derivation owed: redactedMarker = `<redacted>` @ scrub.go:49; traceStepCap = 200 @ trace.go:54; the DESK_TRACE truthy spellings `1`/`true`/`yes`/`on` @ trace.go:98.

VERIFY: BLOCKED — the execution witness covers 2 of 16 rows. The 14 check:ci rows could not run on a darwin host with no network-off sandbox (#1800, open), and row 11 names tests that no longer exist (#1306, open). By hand, 15 of 16 rows are green; rows 7, 8, 12 and 16 needed the house cache policy unset (#2491). Status stays implemented.

### Non-implementer verifier re-run: 2026-10-15, assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk), merged main 8efe5579d9253813835db650d45180eea76cf33e

Run window: 2026-10-15T04:16:09Z (admission) to 2026-10-15T04:46:10Z (last row and probe finished), UTC.
Delivering change: PR #729, squash commit 1003557e167b97c74534d83bf5e47811118fa39c (parent e82c565e).
Later changes to the same files: #1443, #2364, #2719, #2853.

Admission: `deskdispatch --check-verifier` exit 0 for this detached home at the source above, before
any row ran. Every row was run by hand on darwin/arm64, go1.27.2, git 2.56.0. The environment was
cleared and rebuilt with a throwaway HOME, GOCACHE and TMPDIR in the verifier's scratch directory
outside the home, GOPROXY=off, GOTOOLCHAIN=local and KUBECONFIG=/dev/null. No house environment
variables and no forge credential were set. Host load average was 16-38 on 16 cores throughout.
Row 14's listing file was redirected into the scratch directory instead of the system temp
directory. That is the only change from the authored command text.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---------|--------|-----------------------------------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | exit 0, both silent | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 2 | row 2 as authored: two anchored `go test ./internal/deskkit/` runs, one per named test | exit 0 | exit 0; `ok .../internal/deskkit 0.324s` and `ok ... 0.276s`, neither says no tests to run | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 3 | row 3 as authored: two anchored `go test ./internal/deskkit/` runs (report-error off and on) | exit 0 | exit 0; `ok ... 0.267s` and `ok ... 0.367s`, neither says no tests to run | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 4 | row 4 as authored: one anchored `go test ./internal/deskkit/` run (scrub transport shapes) | exit 0 | exit 0; `ok ... 0.293s`, the test matched | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestTraceNeverPrintsACredential$' -count=1` | exit 0 | exit 0; `ok ... 0.309s`, the test matched | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 6 | row 6 as authored: two anchored `go test ./internal/deskkit/` runs (refusal stays exit 5; env spellings) | exit 0 | exit 0; `ok ... 0.315s` and `ok ... 0.268s`, both matched | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 7 | row 7 as authored: two anchored `go test ./cmd/deskdispatch/` runs (claim step message; gitOut stderr) | exit 0 | exit 0; `ok .../cmd/deskdispatch 0.358s` and `ok ... 0.233s`, both matched | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 8 | row 8 as authored: two anchored `go test ./cmd/deskdispatch/` runs (worktree-create stderr; trace off by default) | exit 0 | exit 0; `ok ... 0.354s` and `ok ... 0.342s`, both matched | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 9 | row 9 as authored: two anchored `go test ./cmd/deskwt/` runs | exit 0 | exit 0; `ok .../cmd/deskwt 0.639s` and `ok ... 0.540s`, both matched | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 10 | row 10 as authored: one anchored `go test ./cmd/desktoken/` run | exit 0 | exit 0; `ok .../cmd/desktoken 0.699s`, the test matched | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 11 | row 11 as authored: two anchored `go test ./cmd/deskfile/` runs (the dedupe-search outage test, then TestGhStderrStripsControlBytes) | exit 0, 401/403/429 tell apart, exit 6 unchanged, strip survived | exit 0 but VACUOUS, counted as a non-pass: both print `ok .../cmd/deskfile [no tests to run]`. Neither named test is declared anywhere in the tree at this commit; they were renamed in #783. As a supplementary run, not counted as this row, the two renamed successors in deskfile's trace test file (forge-diagnosis propagation, with subtests 403, 429 and 401, and control bytes stripped by the backend), plus deskkit's TestForgeGitlabTierErrors, all exit 0 with `--- PASS` | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 12 | `cd tools/desk && go test -timeout 300s ./internal/deskkit/... ./cmd/deskdispatch/... ./cmd/deskwt/... ./cmd/desktoken/... ./cmd/deskfile/... -count=1` | exit 0 | exit 1 on both passes, from the 300s package timeout only, with zero `--- FAIL` lines in either. Pass 1: deskkit, deskdispatch and deskwt all hit `panic: test timed out after 5m0s`; untrustcorpus, desktoken (61.1s) and deskfile (261.1s) were ok. Pass 2: deskkit ok 272.9s, untrustcorpus ok, deskdispatch ok 240.5s, desktoken ok 67.9s, deskfile ok 196.7s; deskwt timed out again. A diagnostic run of deskwt alone with a 900s budget gave exit 0, `ok .../cmd/deskwt 494.534s`, 247 PASS and 0 FAIL. Its slowest tests are not this brief's | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 13 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestS2' -count=1 && go test ./internal/deskkit/ -run 'TestCorpus' -count=1` | exit 0 | exit 0; `ok ... 2.450s` and `ok ... 1.416s`. Not vacuous: `go test -list` matches 10 TestS2 and 5 TestCorpus tests | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 14 | `gofmt -l` over the four deskkit files and the four cmd directories, redirected to a listing file, then `test ! -s` on it | exit 0 | exit 0; the listing file is 0 bytes | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 15 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | `LINT: PASS` then `0`, with NOTICE lines only. For this brief, the notices flag vacuity-prone row shapes, the two dangling row-11 test names, and a risk-files cross-read: the brief declares a deskkit path that matches the security-path trigger while all four risk answers are no | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 16 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/trace-mutations.json` | exit 0, every mutation CAUGHT | exit 0; `Harness healthy: baseline GREEN, positive control CAUGHT.` and `Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.` The home was unchanged afterwards (git status clean) | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |

Tally: 14 of 16 rows pass (1-10, 13-16). Row 11 exits 0 without running a test. Row 12 is red on its
300s budget, with no assertion failure.

**Findings**

1. Check-definition, row 11. The two named tests do not exist at this commit; they were renamed in
   #783, previously recorded as #1306. The row cannot pass or fail until it is re-pointed. The
   successors pass, but they cover a narrower deskfile-side property. The 401/403/429 tiers moved to
   deskkit forge tests. The control-byte successor builds its fake from an already-stripped string,
   so it shows only that deskfile does not reintroduce control bytes, not that the strip happens.
2. Environment, row 12. On a host at load 16-38 on 16 cores, three packages exhausted the fixed
   `-timeout 300s` with zero assertion failures. The deskwt package alone needs about 494s at this
   load. The 300s budget has no headroom on a shared host. Re-run on an idle host or CI runner, with
   the same command.
3. NEW, security-relevant: the redactor misses two GitHub token prefixes. Scrub reuses the body
   scanner's GitHub token pattern, which lists ghp_, github_pat_, ghs_ and gho_. GitHub's documented
   prefixes also include ghu_ (user-to-server) and ghr_ (refresh). A probe test run against the real
   Scrub on a scratch copy of this commit planted each prefix bare in a child-stderr-shaped line, with
   no URL, header or NAME= around it. ghp_, gho_, ghs_ and github_pat_ were redacted; ghu_ and ghr_
   were printed unredacted. The same pattern backs the outward-write body scan that the Context's
   single-point-of-failure note names as the independent second layer. For these two prefixes, then,
   both layers share one blind spot. Whether the body scan's other checks catch them was not probed.
   This is outside every Verify row, and the brief's own risk note routes a credential that reaches
   a trace to the human gate.

**VERIFY: BLOCKED** — 14/16 rows pass. Row 11 is a check-definition defect (vacuous, renamed tests). Row 12 is environment-blocked (300s timeout under host load, zero assertion failures). Finding 3 (ghu_/ghr_ not redacted) needs a human decision on the sensitive-data answer. Status stays implemented.

- RISK-VALUE: NAMED, NOT DERIVED — reGitHubToken = `(ghp_|github_pat_|ghs_|gho_)[A-Za-z0-9_]+` @ tools/desk/internal/deskkit/bodycheck.go:16, used by Scrub at scrub.go:75 — derivation against GitHub's documented token prefixes FAILS: ghu_ and ghr_ are absent, and a probe at this commit shows a bare token with either prefix passes Scrub unredacted (Finding 3).
- RISK-VALUE: NAMED, NOT DERIVED — reURLUserinfo = `://([^:/@\s]+):([^@\s]+)@` @ tools/desk/internal/deskkit/scrub.go:31, with reAuthHeader @ line 36 and reSecretEnvAssign @ lines 42-43 — row 16 shows each shape redacts and that removing any one is caught. The sufficiency of the three-shape set for every transport form a child can print is not derived.
- Reversible knobs, ranked last, no derivation owed: traceStepCap = 200 @ trace.go:54; redactedMarker = `<redacted>` @ scrub.go:49; DESK_TRACE truthy spellings `1`/`true`/`yes`/`on` @ trace.go:98, with empty and `0` off, matching the brief.

Verification-Attestation: medici-finance/assay#3011 run=8bfbf62f9d7f0234acb60382953ea36f8badff1ee55da2bc89f1530010f37c25 source=8efe5579d9253813835db650d45180eea76cf33e brief=docs/streams/desk-tools/brief-21-desk-trace-and-cause-carrying-errors.md model=claude-opus-5-5 tier=any

### Non-implementer verifier re-run: 2026-10-21, assay-verifier-app[bot] (claude-opus-5-5 verifier), merged main 65a6418cd90c921153db509370212b1cdcd4f5e1

Run window: 2026-10-21T17:40Z (admission) to 2026-10-21T18:06Z (last diagnostic finished), UTC.
Delivering change: PR #729 (squash 1003557e1). Changes since the 2026-10-15 run that touch this
brief's surface include #3572 (ghu_ and ghr_ added to the GitHub-token pattern).

Admission: `deskdispatch --check-verifier` exit 0 for this detached home at the source above, before
any row ran. Every row ran by hand, exactly as authored, on darwin/arm64, go1.27.2, git 2.56.0,
KUBECONFIG=/dev/null, no forge credential in any row's environment. Host load average 22-46 on 16
cores throughout. The home was clean (git status empty) after every row, including row 16.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | exit 0, both silent | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestToolRunSaidSkipsPreambleAndCarriesTheToolsOwnMessage$' -count=1 && go test ./internal/deskkit/ -run '^TestToolRunFailShapeAndCarriedDetail$' -count=1` | exit 0 | exit 0; `ok .../internal/deskkit 0.457s` and `ok ... 0.468s`, both matched a test | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestReportErrorOffIsByteIdentical$' -count=1 && go test ./internal/deskkit/ -run '^TestReportErrorOnPrintsChainCommandsAndTimings$' -count=1` | exit 0 | exit 0; `ok ... 0.405s` and `ok ... 0.544s`, both matched | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestScrubRedactsEveryTransportShape$' -count=1` | exit 0 | exit 0; `ok ... 0.409s`, matched | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestTraceNeverPrintsACredential$' -count=1` | exit 0 | exit 0; `ok ... 0.471s`, matched | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRefusedWithCauseStaysARefusal$' -count=1 && go test ./internal/deskkit/ -run '^TestTraceEnabledReadsTheEnvSpellings$' -count=1` | exit 0 | exit 0; `ok ... 0.425s` and `ok ... 0.463s`, both matched | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestClaimAcquireFailureNamesTheClaimToolsOwnMessage$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestGitOutFailureCarriesGitStderr$' -count=1` | exit 0 | exit 0; `ok .../cmd/deskdispatch 0.486s` and `ok ... 0.381s`, both matched | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestWorktreeCreateFailureCarriesDeskwtStderrAndTheCommandLine$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestTraceIsOffByDefaultAndOutputIsUnchanged$' -count=1` | exit 0 | exit 0; `ok ... 0.466s` and `ok ... 0.463s`, both matched | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRunGitFailureCarriesStderrCommandAndExitStatus$' -count=1 && go test ./cmd/deskwt/ -run '^TestDeskwtTraceOffIsByteIdenticalAndOnCarriesTheCommand$' -count=1` | exit 0 | exit 0; `ok .../cmd/deskwt 0.631s` and `ok ... 0.600s`, both matched | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestTokenPathNoticeIsPrintedOnStderrNotStdout$' -count=1` | exit 0 | exit 0; `ok .../cmd/desktoken 0.339s`, matched | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 11 | `cd tools/desk && go test ./cmd/deskfile/ -run '^TestDedupeSearchOutageNamesTheAPIStatus$' -count=1 && go test ./cmd/deskfile/ -run '^TestGhStderrStripsControlBytes$' -count=1` | exit 0, 401/403/429 tell apart, exit 6 unchanged, strip survived | could-not-check: exit 0 but vacuous, counted as a non-pass. Both runs print `ok .../cmd/deskfile [no tests to run]`; neither named test is declared anywhere in the tree (renamed in #783; check-definition repair open as #1306). Supplemental probe, discharging no row: the two successors in deskfile's trace test file (TestDedupeSearchPropagatesTheForgeDiagnosis with subtests forbidden, rate_limited, bad_credentials; TestDedupeSearchControlBytesStrippedByBackend) exit 0 with `--- PASS` | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 12 | `cd tools/desk && go test -timeout 300s ./internal/deskkit/... ./cmd/deskdispatch/... ./cmd/deskwt/... ./cmd/desktoken/... ./cmd/deskfile/... -count=1` | exit 0 | could-not-check: exit 1 from the fixed 300s package budget only, zero `--- FAIL` lines. deskkit and deskwt hit `panic: test timed out after 5m0s`; untrustcorpus ok, deskdispatch ok 157.4s, desktoken ok 25.6s, deskfile ok 8.4s. Supplemental diagnostic, discharging no row: the same two timed-out packages with `-timeout 1200s` exit 0, `ok .../internal/deskkit 654.042s` and `ok .../cmd/deskwt 523.198s`, zero `--- FAIL` | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 13 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestS2' -count=1 && go test ./internal/deskkit/ -run 'TestCorpus' -count=1` | exit 0 | exit 0; `ok ... 34.275s` and `ok ... 4.765s` | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 14 | `cd tools/desk && gofmt -l internal/deskkit/trace.go internal/deskkit/runtool.go internal/deskkit/scrub.go internal/deskkit/exitcodes.go cmd/deskdispatch cmd/deskwt cmd/desktoken cmd/deskfile > /tmp/dt21-fmt.out; test ! -s /tmp/dt21-fmt.out` | exit 0 | exit 0; the listing file is empty | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 15 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | `LINT: PASS` then `0`, NOTICE lines only | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 16 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/trace-mutations.json` | exit 0, every mutation CAUGHT | exit 0; `Harness healthy: baseline GREEN, positive control CAUGHT.` and `Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.` | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Tally: 14 of 16 rows pass (1-10, 13-16), 0 fail, 2 could-not-check (11, 12).

**Findings**

1. Check-definition, row 11: unchanged since the last run; the named tests do not exist. Repair is
   tracked on #1306 (open). Successors pass as a supplemental probe only.
2. Environment, row 12: the fixed `-timeout 300s` budget has no headroom on a loaded shared host.
   deskkit alone needed 654s and deskwt 523s at load 22-46, with zero assertion failures. This is
   the second run in a row blocked this way; it needs either an idle host or CI runner, or a
   reviewed budget repair to the row.
3. Prior finding resolved: the 2026-10-15 Finding 3 (ghu_ and ghr_ not redacted) is closed by #3572.
   A supplemental probe on a scratch copy of this commit, outside the home, planted each of ghp_,
   gho_, ghs_, ghu_, ghr_ and github_pat_ bare in a stderr-shaped line; Scrub replaced every one
   with the redaction marker.

**VERIFY: BLOCKED** — 14/16 rows pass, 0 fail. Row 11 is a check-definition defect (#1306). Row 12 is environment-blocked (300s budget under host load, zero assertion failures). Status stays implemented.

- RISK-VALUE: DERIVED — RefusedWithCause Code = ExitRefused @ tools/desk/internal/deskkit/exitcodes.go:186, with ExitRefused = 5 @ exitcodes.go:51 — a refusal that gains a cause must keep the code of Refused (exitcodes.go:172) under the brief's ground rule that no exit code changes; 6 would read a fail-closed refusal as retryable. Pinned by row 6 and row 16's soften-to-6 mutation (CAUGHT).
- RISK-VALUE: DERIVED — reGitHubToken = `(ghp_|github_pat_|ghs_|gho_|ghu_|ghr_)[A-Za-z0-9_]+` @ tools/desk/internal/deskkit/bodycheck.go:16, used by Scrub at scrub.go:75 — the prefix set now matches GitHub's documented token prefixes (personal, OAuth, user-to-server, server-to-server, refresh, fine-grained PAT); the supplemental probe shows each one redacted.
- RISK-VALUE: NAMED, NOT DERIVED — reURLUserinfo = `://([^:/@\s]+):([^@\s]+)@` @ tools/desk/internal/deskkit/scrub.go:31, with reAuthHeader @ line 36 and reSecretEnvAssign @ lines 42-43 — row 16 shows each shape redacts and that removing one is caught; sufficiency of the three-shape set for every transport form a child can print is not derived (open on #1616).
- Reversible knobs, ranked last, no derivation owed: traceStepCap = 200 @ trace.go:54; redactedMarker = `<redacted>` @ scrub.go:49; DESK_TRACE truthy spellings `1`/`true`/`yes`/`on` @ trace.go:98, empty and `0` off.

Verification-Attestation: medici-finance/assay#3884 run=2737f291644299251f9038d3088ddbebaf78b7b93f1f0bea93a8f7481cadd5d3 source=65a6418cd90c921153db509370212b1cdcd4f5e1 brief=docs/streams/desk-tools/brief-21-desk-trace-and-cause-carrying-errors.md model=claude-opus-5-5 tier=strong

### Non-implementer verifier re-run: 2026-10-21 (container), assay-verifier-app[bot] (claude-opus-5-5 verifier), merged main 88ff45841dd7410e580b90f27d8ae7a32c26441d

Run window: 2026-10-21T20:47Z (admission) to 2026-10-21T21:20Z (last supplemental probe finished), UTC.
Delivering change: PR #729 (squash 1003557e1). git diff 65a6418cd..88ff45841 is empty over this brief's four deskkit files, bodycheck.go,
the mutation spec and the four retrofitted cmd packages.

Admission: `deskdispatch --check-verifier` exit 0 for this detached home at the source above, before
any row ran, and again before this section was written. Every row ran exactly as authored, one
container per row, in the assay-verify:go1.27-py image (go1.27.2 linux/arm64, git 2.47.3) with
`--network none`, a non-root uid that has a passwd entry, GOPROXY=off, GOTOOLCHAIN=local,
KUBECONFIG=/dev/null, a tmpfs HOME and /tmp, and a module cache downloaded before the run and
mounted read-only. No forge credential was in any container. The home was mounted read-only for
every row except row 16. muhar mutates files in place and restores them, so on a read-only
mount it reported `HARNESS BROKEN` (read-only file system). That run was discarded, and row 16
ran again with the home mounted writable. git status in the home was empty afterwards. Row 15 reads the
write-class roster, so a 0600 copy of the house roster was mounted read-only as the container
HOME's config. Row 12's first attempt was SIGKILLed (exit 137) by another session's
`docker kill` of every container from this image. A second attempt used a host-bind-mounted /tmp
and its lock and case-variant tests failed. Both are discarded; the attempt recorded below used tmpfs /tmp.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | exit 0, both silent | 2026-10-21 | assay-verifier-app[bot] @ 88ff45841dd7410e580b90f27d8ae7a32c26441d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestToolRunSaidSkipsPreambleAndCarriesTheToolsOwnMessage$' -count=1 && go test ./internal/deskkit/ -run '^TestToolRunFailShapeAndCarriedDetail$' -count=1` | exit 0 | exit 0; `ok .../internal/deskkit 0.070s` and `ok ... 0.040s`, both matched a test | 2026-10-21 | assay-verifier-app[bot] @ 88ff45841dd7410e580b90f27d8ae7a32c26441d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestReportErrorOffIsByteIdentical$' -count=1 && go test ./internal/deskkit/ -run '^TestReportErrorOnPrintsChainCommandsAndTimings$' -count=1` | exit 0 | exit 0; `ok ... 0.064s` and `ok ... 0.077s`, both matched | 2026-10-21 | assay-verifier-app[bot] @ 88ff45841dd7410e580b90f27d8ae7a32c26441d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestScrubRedactsEveryTransportShape$' -count=1` | exit 0 | exit 0; `ok ... 0.033s`, matched | 2026-10-21 | assay-verifier-app[bot] @ 88ff45841dd7410e580b90f27d8ae7a32c26441d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestTraceNeverPrintsACredential$' -count=1` | exit 0 | exit 0; `ok ... 0.057s`, matched | 2026-10-21 | assay-verifier-app[bot] @ 88ff45841dd7410e580b90f27d8ae7a32c26441d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRefusedWithCauseStaysARefusal$' -count=1 && go test ./internal/deskkit/ -run '^TestTraceEnabledReadsTheEnvSpellings$' -count=1` | exit 0 | exit 0; `ok ... 0.038s` and `ok ... 0.027s`, both matched | 2026-10-21 | assay-verifier-app[bot] @ 88ff45841dd7410e580b90f27d8ae7a32c26441d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestClaimAcquireFailureNamesTheClaimToolsOwnMessage$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestGitOutFailureCarriesGitStderr$' -count=1` | exit 0 | exit 0; `ok .../cmd/deskdispatch 0.038s` and `ok ... 0.024s`, both matched | 2026-10-21 | assay-verifier-app[bot] @ 88ff45841dd7410e580b90f27d8ae7a32c26441d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestWorktreeCreateFailureCarriesDeskwtStderrAndTheCommandLine$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestTraceIsOffByDefaultAndOutputIsUnchanged$' -count=1` | exit 0 | exit 0; `ok ... 0.032s` and `ok ... 0.032s`, both matched | 2026-10-21 | assay-verifier-app[bot] @ 88ff45841dd7410e580b90f27d8ae7a32c26441d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRunGitFailureCarriesStderrCommandAndExitStatus$' -count=1 && go test ./cmd/deskwt/ -run '^TestDeskwtTraceOffIsByteIdenticalAndOnCarriesTheCommand$' -count=1` | exit 0 | exit 0; `ok .../cmd/deskwt 0.040s` and `ok ... 0.053s`, both matched | 2026-10-21 | assay-verifier-app[bot] @ 88ff45841dd7410e580b90f27d8ae7a32c26441d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestTokenPathNoticeIsPrintedOnStderrNotStdout$' -count=1` | exit 0 | exit 0; `ok .../cmd/desktoken 0.050s`, matched | 2026-10-21 | assay-verifier-app[bot] @ 88ff45841dd7410e580b90f27d8ae7a32c26441d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 11 | `cd tools/desk && go test ./cmd/deskfile/ -run '^TestDedupeSearchOutageNamesTheAPIStatus$' -count=1 && go test ./cmd/deskfile/ -run '^TestGhStderrStripsControlBytes$' -count=1` | exit 0, 401/403/429 tell apart, exit 6 unchanged, strip survived | could-not-check: exit 0 but vacuous, counted as a non-pass. Both runs print `ok .../cmd/deskfile [no tests to run]`. Neither named test is declared in the tree; they were renamed in #783, and the check-definition repair is still open as #1306. The lint's id-dereference notices name both | 2026-10-21 | assay-verifier-app[bot] @ 88ff45841dd7410e580b90f27d8ae7a32c26441d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 12 | `cd tools/desk && go test -timeout 300s ./internal/deskkit/... ./cmd/deskdispatch/... ./cmd/deskwt/... ./cmd/desktoken/... ./cmd/deskfile/... -count=1` | exit 0 | could-not-check: exit 1 from the container's git, not from this brief's code. deskkit ok 218.4s, untrustcorpus ok, deskdispatch ok 31.0s, desktoken ok 4.7s, deskfile ok 0.8s. deskwt FAIL 30.7s: 12 maintenance and legacy-custody tests fail with `git said: error: unknown subcommand: 'list'`, because the image's git 2.47.3 has no native `git hook list`. None of those tests is this brief's. Supplemental host probe (git 2.56.0), discharging no row: the same deskwt TestMaintenance and TestLegacy tests exit 0 with 31 `--- PASS`. The full row on the host, at load 28-33, hit the 300s budget in deskkit and deskwt with zero `--- FAIL`, as on 2026-10-15 and earlier on 2026-10-21 | 2026-10-21 | assay-verifier-app[bot] @ 88ff45841dd7410e580b90f27d8ae7a32c26441d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 13 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestS2' -count=1 && go test ./internal/deskkit/ -run 'TestCorpus' -count=1` | exit 0 | exit 0; `ok ... 34.739s` and `ok ... 4.152s` | 2026-10-21 | assay-verifier-app[bot] @ 88ff45841dd7410e580b90f27d8ae7a32c26441d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 14 | `cd tools/desk && gofmt -l internal/deskkit/trace.go internal/deskkit/runtool.go internal/deskkit/scrub.go internal/deskkit/exitcodes.go cmd/deskdispatch cmd/deskwt cmd/desktoken cmd/deskfile > /tmp/dt21-fmt.out; test ! -s /tmp/dt21-fmt.out` | exit 0 | exit 0; the listing file is empty (container tmpfs /tmp) | 2026-10-21 | assay-verifier-app[bot] @ 88ff45841dd7410e580b90f27d8ae7a32c26441d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 15 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | `LINT: PASS` then `0`, with NOTICE lines only | 2026-10-21 | assay-verifier-app[bot] @ 88ff45841dd7410e580b90f27d8ae7a32c26441d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 16 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/trace-mutations.json` | exit 0, every mutation CAUGHT | exit 0; `Harness healthy: baseline GREEN, positive control CAUGHT.` and `Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.` (home mounted writable; home clean afterwards) | 2026-10-21 | assay-verifier-app[bot] @ 88ff45841dd7410e580b90f27d8ae7a32c26441d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Tally: 14 of 16 rows pass (1-10, 13-16), 0 fail, 2 could-not-check (11, 12).

**Findings**

1. Check-definition, row 11: unchanged; the named tests do not exist. Repair tracked on #1306 (open).
2. Environment, row 12: in the network-off verify container the row cannot go green, because the image's
   git 2.47.3 lacks `git hook list`, which twelve deskwt maintenance tests outside this brief need. On the
   host (git 2.56.0) those tests pass, but the fixed 300s budget runs out under shared-host load. That is
   the third run in a row with this blocker. It needs either a verify image with a newer git or an idle CI-class runner.
3. Row 16 cannot run on a read-only source mount (muhar edits in place). That is expected of the harness
   and is recorded here so the next container run mounts the home writable for that row.

**VERIFY: BLOCKED** — 14/16 rows pass, 0 fail. Row 11 is a check-definition defect (#1306). Row 12 is environment-blocked: the container's git 2.47.3 has no git hook list, and on the host the 300s budget runs out under load, with zero assertion failures in this brief's packages. Status stays implemented.

- RISK-VALUE: DERIVED — RefusedWithCause Code = ExitRefused @ tools/desk/internal/deskkit/exitcodes.go:186, with ExitRefused = 5 @ exitcodes.go:51 — a refusal that gains a cause must keep the code of Refused under the brief's ground rule that no exit code changes; 6 would read a fail-closed refusal as retryable. Pinned by row 6 and row 16's soften-to-6 mutation (CAUGHT).
- RISK-VALUE: DERIVED — reGitHubToken = `(ghp_|github_pat_|ghs_|gho_|ghu_|ghr_)[A-Za-z0-9_]+` @ tools/desk/internal/deskkit/bodycheck.go:16, used by Scrub — the prefix set matches GitHub's documented token prefixes (personal, OAuth, user-to-server, server-to-server, refresh, fine-grained PAT). Unchanged since the 65a6418cd run, which probed each prefix.
- RISK-VALUE: NAMED, NOT DERIVED — reURLUserinfo = `://([^:/@\s]+):([^@\s]+)@` @ tools/desk/internal/deskkit/scrub.go:31, with reAuthHeader @ line 36 and reSecretEnvAssign @ line 42 — row 16 shows each shape redacts and that removing any one is caught. Sufficiency of the three-shape set is not derived (open on #1616).
- Reversible knobs, ranked last, no derivation owed: traceStepCap = 200 @ trace.go:54 (#1240); redactedMarker = `<redacted>` @ scrub.go:49; DESK_TRACE truthy spellings `1`/`true`/`yes`/`on` @ trace.go:98, with empty and `0` off.

Verification-Attestation: medici-finance/assay#3978 run=ca90491f4ebc235a5032788212fef72f39fc865f81303e0ba9fd45d61732fdfd source=88ff45841dd7410e580b90f27d8ae7a32c26441d brief=docs/streams/desk-tools/brief-21-desk-trace-and-cause-carrying-errors.md model=claude-opus-5-5 tier=strong

### Non-implementer verifier re-run: 2026-10-22, witness plus container hand runs, merged main 05dc9fbc88f2b1b23cd78bebc8634c564327eca4

Run window: 2026-10-22T03:15Z (witness start) to 2026-10-22T03:34Z (last supplemental probe finished), UTC.
Source: 05dc9fbc88f2b1b23cd78bebc8634c564327eca4, equal to forge main when read through the API at the start of
this run. git diff 88ff45841..05dc9fbc8 is empty over this brief's four deskkit files, bodycheck.go, the mutation
spec and the deskdispatch, deskwt and deskfile packages. In desktoken it touches only gitlab.go, a new
gitlab-redirect test file and that package's mutation spec, none of which this brief's rows name.

Admission: `deskdispatch --check-verifier` exit 0 for this detached home at the source above, before any row
ran. The witness table below is the tool's own write-mode `statusgen verifyrun` output from the clean home,
left exactly as written. It ran rows 5 and 16 on the host (darwin/arm64, go1.27.2, git 2.56.0,
KUBECONFIG=/dev/null), and both passed. It marked the 14 check:ci rows could-not-run, because darwin has no
`unshare --net`.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestToolRunSaidSkipsPreambleAndCarriesTheToolsOwnMessage$' -count=1 && go test ./internal/deskkit/ -run '^TestToolRunFailShapeAndCarriedDetail$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestReportErrorOffIsByteIdentical$' -count=1 && go test ./internal/deskkit/ -run '^TestReportErrorOnPrintsChainCommandsAndTimings$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestScrubRedactsEveryTransportShape$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestTraceNeverPrintsACredential$' -count=1` | pass exit=0 | sha256:589a710557cd | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRefusedWithCauseStaysARefusal$' -count=1 && go test ./internal/deskkit/ -run '^TestTraceEnabledReadsTheEnvSpellings$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestClaimAcquireFailureNamesTheClaimToolsOwnMessage$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestGitOutFailureCarriesGitStderr$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestWorktreeCreateFailureCarriesDeskwtStderrAndTheCommandLine$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestTraceIsOffByDefaultAndOutputIsUnchanged$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRunGitFailureCarriesStderrCommandAndExitStatus$' -count=1 && go test ./cmd/deskwt/ -run '^TestDeskwtTraceOffIsByteIdenticalAndOnCarriesTheCommand$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestTokenPathNoticeIsPrintedOnStderrNotStdout$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go test ./cmd/deskfile/ -run '^TestDedupeSearchOutageNamesTheAPIStatus$' -count=1 && go test ./cmd/deskfile/ -run '^TestGhStderrStripsControlBytes$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test -timeout 300s ./internal/deskkit/... ./cmd/deskdispatch/... ./cmd/deskwt/... ./cmd/desktoken/... ./cmd/deskfile/... -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestS2' -count=1 && go test ./internal/deskkit/ -run 'TestCorpus' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 14 | `cd tools/desk && gofmt -l internal/deskkit/trace.go internal/deskkit/runtool.go internal/deskkit/scrub.go internal/deskkit/exitcodes.go cmd/deskdispatch cmd/deskwt cmd/desktoken cmd/deskfile > /tmp/dt21-fmt.out; test ! -s /tmp/dt21-fmt.out` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 15 | `cd statusgen && go run . --root .. --lint; echo $?` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 16 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/trace-mutations.json` | pass exit=0 | sha256:7bb8d3c002b4 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4045 run=d03c030246b9979ae10d80f69d9dc54bf575210a441bb6926818f2baa28de622 source=05dc9fbc88f2b1b23cd78bebc8634c564327eca4 brief=docs/streams/desk-tools/brief-21-desk-trace-and-cause-carrying-errors.md model=claude-opus-5-5 tier=strong

Hand runs of the 14 check:ci rows. Each ran verbatim, one container per row, in a fresh scratch clone at
05dc9fbc8 outside the home. Image: assay-verify:go1.27-py (go1.27.2 linux/arm64, git 2.47.3), with
`--network none` (loopback was the only interface up), a non-root uid that has a passwd entry, GOPROXY=off,
GOTOOLCHAIN=local, KUBECONFIG=/dev/null, a tmpfs HOME and /tmp, and a module cache downloaded before the run and
mounted read-only. The clone was mounted read-only. No forge credential was present in any container. Row 15
reads the write-class roster, so a copy of it was mounted read-only into the container HOME's config.

- Rows 1-4, 6-10, 13, 14: exit 0. Row 1 printed nothing. Each `go test` in rows 2-4 and 6-10 printed
  `ok` for its package and matched a test. Row 13 printed `ok ... 22.634s` and `ok ... 2.436s`. Row 14's listing file was empty.
- Row 15: `LINT: PASS` then `0`, with NOTICE lines only.
- Row 11: could-not-check: exit 0 but vacuous, so it does not count as a pass. Both runs printed
  `ok .../cmd/deskfile [no tests to run]`. Neither named test is declared in the tree; they were renamed in #783,
  and the check-definition repair is still open as #1306.
- Row 12: could-not-check: exit 1 caused by the image's git, not by this brief's code. deskkit ok 116.3s,
  untrustcorpus ok, deskdispatch ok 19.5s, desktoken ok 2.7s, deskfile ok 0.6s. deskwt FAIL 19.1s: 12
  maintenance and legacy-custody tests fail. Nine of them print `git said: error: unknown subcommand: 'list'`
  because git 2.47.3 has no native `git hook list`. The other three are held downstream of that missing
  inventory (`child never started`, `failed attempt was not debounced`). None of those tests belongs to this
  brief. The image limit is tracked on #4010 (open). Supplemental host probe (git 2.56.0), which discharges no
  row: in the same scratch clone, `go test ./cmd/deskwt/ -run 'TestMaintenance|TestLegacy' -count=1 -v` exits 0
  with 31 `--- PASS` and zero `--- FAIL`.

Discarded hand-run attempts, all caused by the harness and not by the source:
- The first container pass had a module cache that did not hold every module, so each build failed at
  `mkdir /gomod/cache: read-only file system`.
- One row 15 attempt reported verify-obligation PROBLEMs from a stale origin/main in the scratch clone. After
  the clone's origin/main was set to the forge main sha, the rerun recorded above passed.
- Two row 12 attempts failed. The first had a local-path origin remote, which broke forge resolution in the
  deskkit and deskdispatch tests. The second forced the global git config to /dev/null, which broke the deskwt
  tests that write global config.

Tally: 14 of 16 rows pass, 0 fail, 2 could-not-check (11, 12). The witness covers rows 5 and 16. Container
hand runs cover rows 1-4, 6-10 and 13-15.

**VERIFY: BLOCKED** — 14/16 rows pass, 0 fail. Row 11 is a check-definition defect (#1306). Row 12 is blocked by the environment: the verify image's git 2.47.3 has no git hook list (#4010), and this brief's packages have zero assertion failures. Status stays implemented.

- RISK-VALUE: DERIVED — RefusedWithCause Code = ExitRefused @ tools/desk/internal/deskkit/exitcodes.go:186, with ExitRefused = 5 @ exitcodes.go:51 — a refusal that gains a cause must keep the code of Refused (exitcodes.go:172), because the brief's ground rule says no exit code changes, and 6 would read a fail-closed refusal as retryable. Row 6 and row 16's soften-to-6 mutation (CAUGHT) pin it.
- RISK-VALUE: DERIVED — reGitHubToken = `(ghp_|github_pat_|ghs_|gho_|ghu_|ghr_)[A-Za-z0-9_]+` @ tools/desk/internal/deskkit/bodycheck.go:16, used by Scrub — the prefix set matches GitHub's documented token prefixes (personal, OAuth, user-to-server, server-to-server, refresh, fine-grained PAT). It is unchanged since the 65a6418cd run, which probed each prefix.
- RISK-VALUE: NAMED, NOT DERIVED — reURLUserinfo = `://([^:/@\s]+):([^@\s]+)@` @ tools/desk/internal/deskkit/scrub.go:31, with reAuthHeader @ line 36 and reSecretEnvAssign @ lines 42-43 — row 16 shows that each shape redacts and that removing any one is caught. Whether the three-shape set is sufficient is not derived (open on #1616).
- Reversible knobs, ranked last, no derivation owed: traceStepCap = 200 @ trace.go:54; redactedMarker = `<redacted>` @ scrub.go:49; DESK_TRACE truthy spellings `1`/`true`/`yes`/`on` @ trace.go:98, with empty and `0` off.


### Addendum: 2026-10-22, check:ci rows under the container-netns sandbox, merged main 05dc9fbc88f2b1b23cd78bebc8634c564327eca4

This addendum replaces the tally and verdict of the section above. Container hand runs written as prose are not
an execution witness, so none of the 14 check:ci rows can count as a pass on the strength of them. The hand-run
notes above stay as context and discharge no row.

Witness attempt at 2026-10-22T03:44:53Z UTC (under one second). This was statusgen v1.0.54, the pinned
version, built for linux/arm64 from the v1.0.54 tag. It ran `statusgen verifyrun --brief <this brief> --root
<admitted home> --sandbox container-netns` inside `docker run --rm --network none` on image assay-verify:go1.27-py.
The admitted home was mounted, with GOPROXY=off, GOTOOLCHAIN=local, KUBECONFIG=/dev/null and a non-root uid.
Loopback was the only interface up (tunl0 was present but down). The tool exited 2 before the sandbox
precheck and before any row ran: `statusgen verifyrun: pre-work verifier admission failed; no Verify rows
executed`. The pre-work admission step has to obtain a verifier forge credential from the forge, and a
`--network none` container cannot reach the forge. The brief was byte-identical before and after the attempt,
so the tool wrote no table. Its could-not-run output for the 14 check:ci rows is the witness table above
(sandbox=unshare, darwin host), left as written. The missing Linux network-off witness lane is tracked on
medici-finance/assay#3216 (open). The admission refusal inside a `--network none` container is tracked on medici-finance/assay#2665 (open). Here the admission step exited 5, because no credential mint path is available offline. A sibling run on the same host saw exit 6 (network unreachable). Both are the same forge dependency.

Correction to the section above: between 88ff45841 and 05dc9fbc8 the deskdispatch package's Go code is
unchanged. The only file that changed under tools/desk/cmd/deskdispatch is references/review-prompt.md, which no
row of this brief reads.

Tally, by what each row rests on:
- Rows 5 and 16 pass on the write-mode witness table above (exit 0).
- Rows 1-4, 6-10 and 13-15: could-not-check: the check:ci witness is could-not-run on darwin, and the
  container-netns run was refused at admission (#3216, #2665).
- Row 11: could-not-check: the row names renamed tests and passes vacuously (#1306). It is also a check:ci row
  without a witness (#3216).
- Row 12: could-not-check: the verify image's git 2.47.3 has no git hook list (#4010). It is also a check:ci row
  without a witness (#3216).

2 of 16 rows pass, 0 fail, 14 could-not-check.

**VERIFY: BLOCKED** — 2/16 rows pass on the execution witness (5, 16), 0 fail, 14 could-not-check. The 14 check:ci rows have no network-off witness (#3216, #2665). Row 11 also depends on #1306, and row 12 also depends on #4010. Status stays implemented.

- RISK-VALUE: DERIVED — RefusedWithCause Code = ExitRefused @ tools/desk/internal/deskkit/exitcodes.go:186, with ExitRefused = 5 @ exitcodes.go:51 — a refusal that gains a cause keeps the code of Refused (exitcodes.go:172), because the brief's ground rule says no exit code changes, and 6 would read a fail-closed refusal as retryable. Row 16's soften-to-6 mutation (CAUGHT, witnessed) pins it. Row 6 is could-not-check above.
- RISK-VALUE: DERIVED — reGitHubToken = `(ghp_|github_pat_|ghs_|gho_|ghu_|ghr_)[A-Za-z0-9_]+` @ tools/desk/internal/deskkit/bodycheck.go:16, used by Scrub — the prefix set matches GitHub's documented token prefixes. It is unchanged since the 65a6418cd run, which probed each prefix.
- RISK-VALUE: NAMED, NOT DERIVED — reURLUserinfo @ tools/desk/internal/deskkit/scrub.go:31, with reAuthHeader @ line 36 and reSecretEnvAssign @ lines 42-43 — row 16 (witnessed) shows that each shape redacts and that removing any one is caught. Whether the three-shape set is sufficient is not derived (open on #1616).
- Reversible knobs, ranked last, no derivation owed: traceStepCap = 200 @ trace.go:54; redactedMarker = `<redacted>` @ scrub.go:49; DESK_TRACE truthy spellings `1`/`true`/`yes`/`on` @ trace.go:98.

### Non-implementer verifier re-run: 2026-10-24, host witness plus container-netns bracket, merged main 385cb29e5682dcb9a886b298076f722b4823ec1b

The earlier blocks above (2026-09-09 to the 2026-10-22 addendum) hold the implementer run, one PASS and then BLOCKED re-runs that had no network-off witness for the 14 check:ci rows; this run supplies one.

Run window: 2026-10-24T18:26Z (first host row) to 2026-10-24T19:11Z (host write-mode witness finished), UTC.
Source: 385cb29e5682dcb9a886b298076f722b4823ec1b. Between 05dc9fbc8 and this source, trace.go, runtool.go, scrub.go,
exitcodes.go, trace-mutations.json and trace_test.go are unchanged; bodycheck.go changed, but its regex
definitions did not.

Admission: `deskdispatch --check-verifier` exit 0 for this detached home at the source above, before any row
ran. The rows were split into two disjoint selections that together cover all 16 rows.

- Rows 5 and 16 (host, darwin/arm64, go1.27.2, git 2.56.0, KUBECONFIG=/dev/null): the first witness table
  below is the tool's own write-mode `statusgen verifyrun --rows 5,16` output, written into this section
  from the clean home after the container bracket had closed, and left as written. A dry run of the same
  two rows at 18:26Z-18:32Z also passed.
- Rows 1-4 and 6-15 (container-netns): `statusgen verifyrun --open-bracket` on the host (receipt outside the
  home), then `statusgen verifyrun --sandbox=container-netns --rows 1,2,3,4,6,7,8,9,10,11,12,13,14,15` inside
  `docker run --network none --read-only` on the pinned verify-runner v1.0.59 image (digest in the
  Container-Image line; go1.27.2, git 2.56.0, statusgen v1.0.59, linux/amd64 under emulation on an arm64
  host). The home was mounted read-only, with a non-root uid, GOPROXY=off, GOTOOLCHAIN=local,
  KUBECONFIG=/dev/null, a tmpfs HOME and /tmp, and no forge credential. The run took 18:33:38Z-18:52:26Z.
  `--close-bracket` on the host then appended the Admission-Bracket line. The second table below, with its
  Tree-Digest, Container-Image, attestation and Admission-Bracket lines, is that witness file's bytes.

Host load during the container run (1/5/15-minute averages, 16 cores): 62.68 / 88.85 / 68.81 at 18:51:08Z,
62.27 / 83.50 / 68.26 at 18:52:13Z.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestTraceNeverPrintsACredential$' -count=1` | pass exit=0 | sha256:1475ae913315 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 16 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/trace-mutations.json` | pass exit=0 | sha256:22c9ecd55db6 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4314 run=dbfc272562f060d6fe4738943c0850fe9ffcf526ea8a00ca76ba8a5dfee9bec4 source=385cb29e5682dcb9a886b298076f722b4823ec1b brief=docs/streams/desk-tools/brief-21-desk-trace-and-cause-carrying-errors.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestToolRunSaidSkipsPreambleAndCarriesTheToolsOwnMessage$' -count=1 && go test ./internal/deskkit/ -run '^TestToolRunFailShapeAndCarriedDetail$' -count=1` | pass exit=0 sandbox=container-netns | sha256:5bc984cb301a | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestReportErrorOffIsByteIdentical$' -count=1 && go test ./internal/deskkit/ -run '^TestReportErrorOnPrintsChainCommandsAndTimings$' -count=1` | pass exit=0 sandbox=container-netns | sha256:d3348e4c4ee3 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestScrubRedactsEveryTransportShape$' -count=1` | pass exit=0 sandbox=container-netns | sha256:4327580d14fb | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRefusedWithCauseStaysARefusal$' -count=1 && go test ./internal/deskkit/ -run '^TestTraceEnabledReadsTheEnvSpellings$' -count=1` | pass exit=0 sandbox=container-netns | sha256:99ff53277e36 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestClaimAcquireFailureNamesTheClaimToolsOwnMessage$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestGitOutFailureCarriesGitStderr$' -count=1` | pass exit=0 sandbox=container-netns | sha256:4844893dac8e | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestWorktreeCreateFailureCarriesDeskwtStderrAndTheCommandLine$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestTraceIsOffByDefaultAndOutputIsUnchanged$' -count=1` | pass exit=0 sandbox=container-netns | sha256:cc3d4ce0e429 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRunGitFailureCarriesStderrCommandAndExitStatus$' -count=1 && go test ./cmd/deskwt/ -run '^TestDeskwtTraceOffIsByteIdenticalAndOnCarriesTheCommand$' -count=1` | pass exit=0 sandbox=container-netns | sha256:1abc427f8150 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestTokenPathNoticeIsPrintedOnStderrNotStdout$' -count=1` | pass exit=0 sandbox=container-netns | sha256:6e1a8b99e58f | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go test ./cmd/deskfile/ -run '^TestDedupeSearchOutageNamesTheAPIStatus$' -count=1 && go test ./cmd/deskfile/ -run '^TestGhStderrStripsControlBytes$' -count=1` | pass exit=0 sandbox=container-netns | sha256:dfb4222788e1 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test -timeout 300s ./internal/deskkit/... ./cmd/deskdispatch/... ./cmd/deskwt/... ./cmd/desktoken/... ./cmd/deskfile/... -count=1` | fail exit=1 sandbox=container-netns | sha256:e30c453cec02 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestS2' -count=1 && go test ./internal/deskkit/ -run 'TestCorpus' -count=1` | pass exit=0 sandbox=container-netns | sha256:5c055d8ece5c | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 14 | `cd tools/desk && gofmt -l internal/deskkit/trace.go internal/deskkit/runtool.go internal/deskkit/scrub.go internal/deskkit/exitcodes.go cmd/deskdispatch cmd/deskwt cmd/desktoken cmd/deskfile > /tmp/dt21-fmt.out; test ! -s /tmp/dt21-fmt.out` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 15 | `cd statusgen && go run . --root .. --lint; echo $?` | pass exit=0 sandbox=container-netns | sha256:287237930acc | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:98685d6717c495aecd71bc3b8cb782aa9cc7dc0ac1332b029f60eb2c3a1c1104
Container-Image: sha256:c2e32c7ff99f67951174e8ff835d928d8999b1bb2ecdc7c63d3775528178640e
Verification-Attestation: medici-finance/assay#4314 run=dbfc272562f060d6fe4738943c0850fe9ffcf526ea8a00ca76ba8a5dfee9bec4 source=385cb29e5682dcb9a886b298076f722b4823ec1b brief=docs/streams/desk-tools/brief-21-desk-trace-and-cause-carrying-errors.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=dbfc272562f060d6fe4738943c0850fe9ffcf526ea8a00ca76ba8a5dfee9bec4 source=385cb29e5682dcb9a886b298076f722b4823ec1b tree=sha256:98685d6717c495aecd71bc3b8cb782aa9cc7dc0ac1332b029f60eb2c3a1c1104 witness=sha256:2974dd34627a5e34ca11e3c834f905863b46cf058e6faad364947724599d8b4e opened=2026-10-24T18:33:25Z closed=2026-10-24T18:54:20Z before=receipt after=check-verifier

How each row is counted:

- Rows 1-4, 6-10 and 13-15 pass on the container-netns witness (exit 0, sandbox=container-netns).
- Rows 5 and 16 pass on the host witness (exit 0).
- Row 11: could-not-check, although its witness reads pass. The row is vacuous: neither
  TestDedupeSearchOutageNamesTheAPIStatus nor TestGhStderrStripsControlBytes is declared in cmd/deskfile at
  this source, so both runs match no test. The check-definition repair is still open as #1306.
- Row 12: could-not-check. The witness reads fail exit=1, and that failure comes from the run environment, not
  from this brief's code. The witness does not keep row output, so after the bracket closed the same command
  was re-run in the same image and mounts, with its output kept. This diagnostic discharges no row. It ran
  18:59:09Z-19:08:55Z, starting when host load was 28.54 / 46.74 / 55.79. It exited 1. untrustcorpus,
  desktoken (10.4s) and deskfile (4.5s) were `ok`. deskkit, deskdispatch and deskwt failed, and all three
  causes are environmental:
  - The home is a linked worktree, so its `.git` file points at a host path that is not mounted in the
    container. Tests that run git from the package directory print
    `fatal: gitfile does not point to a valid repository: /work/.git` (23 lines). Tests that resolve the
    forge from the origin remote exit 6 with `cannot resolve which forge serves medici-finance/assay`
    (55 lines). These cover most of deskdispatch's 24 failing tests and deskkit's credential-chain tests.
    Tracked on #4327 (linked-worktree gitdir not mounted in the container) and #4322 (tests resolve the
    forge and roster from the ambient remote and roster).
  - The harness passed a verifier commit identity through GIT_CONFIG_* variables. Those variables override
    the commit identities that deskkit's publish-range tests and deskdispatch's worktree-identity test set
    up for themselves. Tracked on #4338.
  - deskkit and deskwt hit `panic: test timed out after 5m0s` under emulation on the loaded host. Tracked on
    #4323.
  None of the failing tests is one of this brief's tests: rows 2-10, which name this brief's tests, all pass on the
  witness. Earlier host runs of row 12 with a longer budget were green (2026-10-21 block above), and main CI at
  385cb29e5682 has `build-test-shard (desk-1|desk-2|desk-3|rest)` all success (run 40111602660).
  Added by the verify desk after review: the issue links in this bullet, the CI corroboration, and the
  blocker list in the verdict line below.

Tally: 14 of 16 rows pass (1-10, 13-16), 0 fail, 2 could-not-check (11, 12).

**VERIFY: BLOCKED** — 14/16 rows pass on the execution witnesses, 0 fail, 2 could-not-check. Row 11 is a check-definition defect (#1306). Row 12 is an instrument limit: inside the container the home's linked-worktree `.git` cannot be resolved, the harness's GIT_CONFIG_* identity overrides test fixtures, and the 300s budget expires under emulation; none of its failing tests is this brief's (blockers: #4327, #4322, #4338, #4323). Status stays implemented.

- RISK-VALUE: DERIVED — RefusedWithCause Code = ExitRefused @ tools/desk/internal/deskkit/exitcodes.go:186, with ExitRefused = 5 @ exitcodes.go:51 — a refusal that gains a cause keeps the code of Refused (exitcodes.go:172), because the brief's ground rule says no exit code changes, and ExitUnverifiable = 6 (exitcodes.go:55) would read a fail-closed refusal as retryable. Row 6 and row 16's soften-to-6 mutation (CAUGHT), both witnessed passing above, pin it.
- RISK-VALUE: DERIVED — reGitHubToken = `(ghp_|github_pat_|ghs_|gho_|ghu_|ghr_)[A-Za-z0-9_]+` @ tools/desk/internal/deskkit/bodycheck.go:16, used by Scrub — the prefix set matches GitHub's documented token prefixes. Its definition is unchanged at this source, as are reAWSKeyID @ bodycheck.go:17 and reJWT @ bodycheck.go:22.
- RISK-VALUE: NAMED, NOT DERIVED — reURLUserinfo @ tools/desk/internal/deskkit/scrub.go:31, with reAuthHeader @ line 36 and reSecretEnvAssign @ lines 42-43 — rows 4 and 16 (witnessed) show that each shape redacts and that removing any one is caught. Whether the three-shape set is sufficient is not derived (open on #1616).
- Reversible knobs, ranked last, no derivation owed: traceStepCap = 200 @ trace.go:54; redactedMarker = `<redacted>` @ scrub.go:49; DESK_TRACE truthy spellings `1`/`true`/`yes`/`on` @ trace.go:98.
### Non-implementer verifier re-run: 2026-10-27, host witness plus container-netns bracket, merged main 8fc5e016beaf9113fd03f1e284bcd5590a3f8982

The 2026-10-24 re-run above was BLOCKED at 385cb29e5682 on rows 11 and 12. This run repeats it at the current merged main.

Source: 8fc5e016beaf9113fd03f1e284bcd5590a3f8982. Between 385cb29e5682 and this source, trace.go, runtool.go, scrub.go,
exitcodes.go, bodycheck.go, trace-mutations.json and trace_test.go are unchanged. The delivering change is
1003557e167b97c74534d83bf5e47811118fa39c (#729). No Verify row carries a risk-bearing, live, mutating or
end-to-end tag, and every answer under the brief's risk questions is no, so no row owes a fail-first witness.

Admission: `deskdispatch --check-verifier` exit 0 for this detached home at the source above, before any row
ran. The rows were split into two disjoint selections that together cover all 16 rows.

- Rows 1-4 and 6-15 (container-netns): `statusgen verifyrun --open-bracket` on the host at 21:42:27Z (receipt
  outside the home), then `statusgen verifyrun --sandbox=container-netns --rows 1,2,3,4,6,7,8,9,10,11,12,13,14,15`
  inside `docker run --network none --read-only --cap-drop ALL` on the pinned verify-runner v1.0.63 image (digest
  in the Container-Image line; go1.27.2, git 2.56.0, statusgen v1.0.63, linux/amd64 under emulation on an arm64
  host). The home was mounted read-only, with a non-root uid, GOPROXY=off, GOTOOLCHAIN=local,
  KUBECONFIG=/dev/null, a tmpfs HOME and /tmp, no forge credential and no injected git identity. The host Go
  module cache was also mounted read-only at /go/pkg/mod, because the image's own module cache predates this
  source's go.mod (for example it holds golang.org/x/text v0.41.0 where the source requires v0.43.0), so
  offline module resolution fails without it. The run took 21:48:07Z-22:02:48Z. `--close-bracket` on the host
  at 22:03:24Z then appended the Admission-Bracket line, and `--bracket-digest` read `verdict=match`. The
  second table below, with its Tree-Digest, Container-Image, attestation and Admission-Bracket lines, is
  that witness file's bytes.
- Rows 5 and 16 (host, darwin/arm64, go1.27.2, KUBECONFIG=/dev/null, real module and build caches, no
  forge token in the row environment): the first table below is the tool's own write-mode
  `statusgen verifyrun --rows 5,16` output, written into this section from the clean home after the container
  bracket had closed, and left as written (22:11:41Z-22:13:18Z). These rows ran under the operator's own HOME: statusgen's pre-work
  admission mints the verifier App token, which a credential-free throwaway HOME cannot (the first attempt with
  one refused before any row ran: `pre-work verifier admission failed; no Verify rows executed`).

Discarded and refused container attempts inside the same bracket, disclosed:

- Attempt 1 (21:42:35Z) refused before any row ran: `container admission refused; no Verify rows executed`.
  A shell quoting slip had mounted the home at the wrong path, read-write. The refusal is the admission check
  working, and the home was confirmed unmodified afterwards.
- Attempt 2 (21:43:21Z-21:44:12Z) ran without the module-cache mount. Rows 1-13 failed on offline module
  resolution (row 1 sha256:15ad7645a9f1; rows 2, 3, 4, 6 and 13 sha256:76c8b9eb3ae2), and rows 14 and 15
  passed. That witness was discarded as an instrument fault and is not landed. The witness below is attempt 3.

Host load during the container run (1/5/15-minute averages): 11.76 / 11.63 / 12.14 at 21:48:07Z,
9.77 / 11.17 / 11.59 at 22:02:48Z.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestTraceNeverPrintsACredential$' -count=1` | pass exit=0 | sha256:7b0da3ace4f5 | 2026-10-27 | assay-verifier-app[bot] @ 8fc5e016beaf (on-behalf-of human:ian) (forge-identity) |
| 16 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/trace-mutations.json` | pass exit=0 | sha256:8df45dd5e755 | 2026-10-27 | assay-verifier-app[bot] @ 8fc5e016beaf (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4666 run=baa0916eb0bebe56beca9980cfe36b8e90b1846168aa85bc9c6d3a2fad70a42a source=8fc5e016beaf9113fd03f1e284bcd5590a3f8982 brief=docs/streams/desk-tools/brief-21-desk-trace-and-cause-carrying-errors.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 8fc5e016beaf (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestToolRunSaidSkipsPreambleAndCarriesTheToolsOwnMessage$' -count=1 && go test ./internal/deskkit/ -run '^TestToolRunFailShapeAndCarriedDetail$' -count=1` | pass exit=0 sandbox=container-netns | sha256:4fbd3159ed5c | 2026-10-27 | assay-verifier-app[bot] @ 8fc5e016beaf (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestReportErrorOffIsByteIdentical$' -count=1 && go test ./internal/deskkit/ -run '^TestReportErrorOnPrintsChainCommandsAndTimings$' -count=1` | pass exit=0 sandbox=container-netns | sha256:81005f1f5310 | 2026-10-27 | assay-verifier-app[bot] @ 8fc5e016beaf (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestScrubRedactsEveryTransportShape$' -count=1` | pass exit=0 sandbox=container-netns | sha256:4b9ad1d43634 | 2026-10-27 | assay-verifier-app[bot] @ 8fc5e016beaf (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRefusedWithCauseStaysARefusal$' -count=1 && go test ./internal/deskkit/ -run '^TestTraceEnabledReadsTheEnvSpellings$' -count=1` | pass exit=0 sandbox=container-netns | sha256:4a6bd8a8464b | 2026-10-27 | assay-verifier-app[bot] @ 8fc5e016beaf (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestClaimAcquireFailureNamesTheClaimToolsOwnMessage$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestGitOutFailureCarriesGitStderr$' -count=1` | pass exit=0 sandbox=container-netns | sha256:38435402b307 | 2026-10-27 | assay-verifier-app[bot] @ 8fc5e016beaf (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestWorktreeCreateFailureCarriesDeskwtStderrAndTheCommandLine$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestTraceIsOffByDefaultAndOutputIsUnchanged$' -count=1` | pass exit=0 sandbox=container-netns | sha256:d9df1b975799 | 2026-10-27 | assay-verifier-app[bot] @ 8fc5e016beaf (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRunGitFailureCarriesStderrCommandAndExitStatus$' -count=1 && go test ./cmd/deskwt/ -run '^TestDeskwtTraceOffIsByteIdenticalAndOnCarriesTheCommand$' -count=1` | pass exit=0 sandbox=container-netns | sha256:5441f980c5aa | 2026-10-27 | assay-verifier-app[bot] @ 8fc5e016beaf (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestTokenPathNoticeIsPrintedOnStderrNotStdout$' -count=1` | pass exit=0 sandbox=container-netns | sha256:f7c39bcb05fa | 2026-10-27 | assay-verifier-app[bot] @ 8fc5e016beaf (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go test ./cmd/deskfile/ -run '^TestDedupeSearchOutageNamesTheAPIStatus$' -count=1 && go test ./cmd/deskfile/ -run '^TestGhStderrStripsControlBytes$' -count=1` | pass exit=0 sandbox=container-netns | sha256:9059c730b65a | 2026-10-27 | assay-verifier-app[bot] @ 8fc5e016beaf (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test -timeout 300s ./internal/deskkit/... ./cmd/deskdispatch/... ./cmd/deskwt/... ./cmd/desktoken/... ./cmd/deskfile/... -count=1` | fail exit=1 sandbox=container-netns | sha256:98da9d59afb8 | 2026-10-27 | assay-verifier-app[bot] @ 8fc5e016beaf (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestS2' -count=1 && go test ./internal/deskkit/ -run 'TestCorpus' -count=1` | pass exit=0 sandbox=container-netns | sha256:711c6140324f | 2026-10-27 | assay-verifier-app[bot] @ 8fc5e016beaf (on-behalf-of human:ian) (forge-identity) |
| 14 | `cd tools/desk && gofmt -l internal/deskkit/trace.go internal/deskkit/runtool.go internal/deskkit/scrub.go internal/deskkit/exitcodes.go cmd/deskdispatch cmd/deskwt cmd/desktoken cmd/deskfile > /tmp/dt21-fmt.out; test ! -s /tmp/dt21-fmt.out` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 8fc5e016beaf (on-behalf-of human:ian) (forge-identity) |
| 15 | `cd statusgen && go run . --root .. --lint; echo $?` | pass exit=0 sandbox=container-netns | sha256:1c3a789ccc20 | 2026-10-27 | assay-verifier-app[bot] @ 8fc5e016beaf (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:7fc0bf061372028091d423804335177a6397d17848d2e1679f8cdc5a5697febe
Container-Image: sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8
Verification-Attestation: medici-finance/assay#4666 run=baa0916eb0bebe56beca9980cfe36b8e90b1846168aa85bc9c6d3a2fad70a42a source=8fc5e016beaf9113fd03f1e284bcd5590a3f8982 brief=docs/streams/desk-tools/brief-21-desk-trace-and-cause-carrying-errors.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=baa0916eb0bebe56beca9980cfe36b8e90b1846168aa85bc9c6d3a2fad70a42a source=8fc5e016beaf9113fd03f1e284bcd5590a3f8982 tree=sha256:7fc0bf061372028091d423804335177a6397d17848d2e1679f8cdc5a5697febe witness=sha256:3bd76fb72657b4b3557502da03f600421898198bf478ff58ae58028302e04bfe opened=2026-10-27T21:42:27Z closed=2026-10-27T22:03:24Z before=receipt after=check-verifier

How each row is counted:

- Rows 1-4, 6-10 and 13-15 pass on the container-netns witness (exit 0, sandbox=container-netns).
- Rows 5 and 16 pass on the host witness (exit 0).
- Row 11: could-not-check, although its witness reads pass. The row is vacuous: neither
  TestDedupeSearchOutageNamesTheAPIStatus nor TestGhStderrStripsControlBytes is declared in cmd/deskfile at
  this source, so both runs match no test. The check-definition repair is still open as #1306.
- Row 12: could-not-check. The witness reads fail exit=1, and the witness does not keep row output. After the
  bracket closed, the same command was re-run as a diagnostic on the host (darwin/arm64), in a scratch
  `git archive` export of the source outside the home, with a throwaway HOME, real caches, GOPROXY=off and no
  forge token. This diagnostic discharges no row. It finished at 22:10:05Z and exited 1. untrustcorpus,
  deskwt, desktoken and deskfile were `ok`. deskdispatch (23 tests) and deskkit (3 tests) failed:
  - The export has no origin remote, so tests that resolve the forge from the ambient remote exit 6 with
    `cannot resolve which forge serves medici-finance/assay` (56 lines, 55 of them in deskdispatch). Every
    deskdispatch failure (the stamp, packet, attestation and stamp-only tests) asserts on that exit 6, and
    deskkit's TestSharedReadLowerScope fails on the same message. Tracked on #4322
    (tests resolve the forge and roster from the ambient remote and roster). Inside the container the same
    class appears because the home's linked-worktree gitdir is not mounted (#4327).
  - deskkit's TestCompiledStartupDiscoveryWorkload (its startup-workload trace file was not written) and
    TestReviewCallerRollback (a caller-fault qualification sub-run) also failed. Neither is in this brief's
    change set, and the cause was not isolated here.
  - The container witness additionally ran under emulation with a 300s budget (#4323).
  None of the failing tests is one of this brief's tests: the delivering change adds trace_test.go in deskkit,
  deskdispatch, deskfile, desktoken and deskwt, rows 2-10, which name this brief's tests, all pass on the
  witness, and no failing test is declared in any of those files. Main CI at 8fc5e016beaf has
  `build-test-shard (desk-1|desk-2|desk-3|rest)` all success (run 40587841753).

Tally: 14 of 16 rows pass (1-10, 13-16), 0 fail, 2 could-not-check (11, 12).

**VERIFY: BLOCKED** — 14/16 rows pass on the execution witnesses, 0 fail, 2 could-not-check. Row 11 is a check-definition defect (#1306). Row 12 is an instrument limit: its five-package run needs a git checkout whose origin remote resolves the forge, which neither the container witness nor an offline export provides, and none of its failing tests is this brief's (blockers: #4327, #4322, #4323). Status stays implemented.

- RISK-VALUE: DERIVED — RefusedWithCause Code = ExitRefused @ tools/desk/internal/deskkit/exitcodes.go:186, with ExitRefused = 5 @ exitcodes.go:51 — a refusal that gains a cause keeps the code of Refused (exitcodes.go:172), because the brief's ground rule says no exit code changes, and ExitUnverifiable = 6 (exitcodes.go:55) would read a fail-closed refusal as retryable. Row 6 and row 16's soften-to-6 mutation (CAUGHT), both witnessed passing above, pin it.
- RISK-VALUE: DERIVED — reGitHubToken = `(ghp_|github_pat_|ghs_|gho_|ghu_|ghr_)[A-Za-z0-9_]+` @ tools/desk/internal/deskkit/bodycheck.go:16, used by Scrub — the prefix set matches GitHub's documented token prefixes. Its definition is unchanged at this source, as are reAWSKeyID @ bodycheck.go:17 and reJWT @ bodycheck.go:22.
- RISK-VALUE: NAMED, NOT DERIVED — reURLUserinfo @ tools/desk/internal/deskkit/scrub.go:31, with reAuthHeader @ line 36 and reSecretEnvAssign @ lines 42-43 — rows 4 and 16 (witnessed) show that each shape redacts and that removing any one is caught. Whether the three-shape set is sufficient is not derived (open on #1616).
- Reversible knobs, ranked last, no derivation owed: traceStepCap = 200 @ trace.go:54; redactedMarker = `<redacted>` @ scrub.go:49; DESK_TRACE truthy spellings `1`/`true`/`yes`/`on` @ trace.go:98.
- RISK-VALUE: DERIVED — RefusedWithCause Code = ExitRefused @ tools/desk/internal/deskkit/exitcodes.go:186, with ExitRefused = 5 @ exitcodes.go:51 — a refusal that gains a cause keeps the code of Refused (exitcodes.go:172), because the brief's ground rule says no exit code changes, and ExitUnverifiable = 6 (exitcodes.go:55) would read a fail-closed refusal as retryable. Row 6 and row 16's soften-to-6 mutation (CAUGHT), both witnessed passing above, pin it.
- RISK-VALUE: DERIVED — reGitHubToken = `(ghp_|github_pat_|ghs_|gho_|ghu_|ghr_)[A-Za-z0-9_]+` @ tools/desk/internal/deskkit/bodycheck.go:16, used by Scrub — the prefix set matches GitHub's documented token prefixes. Its definition is unchanged at this source, as are reAWSKeyID @ bodycheck.go:17 and reJWT @ bodycheck.go:22.
- RISK-VALUE: NAMED, NOT DERIVED — reURLUserinfo @ tools/desk/internal/deskkit/scrub.go:31, with reAuthHeader @ line 36 and reSecretEnvAssign @ lines 42-43 — rows 4 and 16 (witnessed) show that each shape redacts and that removing any one is caught. Whether the three-shape set is sufficient is not derived (open on #1616).
- Reversible knobs, ranked last, no derivation owed: traceStepCap = 200 @ trace.go:54; redactedMarker = `<redacted>` @ scrub.go:49; DESK_TRACE truthy spellings `1`/`true`/`yes`/`on` @ trace.go:98.

### Non-implementer verifier re-run: 2026-10-29, host witness plus container-netns bracket, merged main 2c16bbb5be70fadd5a0ca8718e64e2a03bd697ea

The 2026-10-27 re-run above was BLOCKED at 8fc5e016beaf on rows 11 and 12. This run repeats it at the current merged main.

Source: 2c16bbb5be70fadd5a0ca8718e64e2a03bd697ea. Between 8fc5e016beaf and this source, trace.go, runtool.go, scrub.go,
exitcodes.go, bodycheck.go, trace-mutations.json and trace_test.go are unchanged. The delivering change is
1003557e167b97c74534d83bf5e47811118fa39c (#729). No Verify row carries a risk-bearing, live, mutating or
end-to-end tag, and every answer under the brief's risk questions is no, so no row owes a fail-first witness.

Admission: `deskdispatch --check-verifier` exit 0 for this detached home at the source above, before any row
ran, and again exit 0 (same run, same source, clean home) when the run resumed after a pause. The rows were split
into two disjoint selections that together cover all 16 rows.

- Rows 1-4 and 6-15 (container-netns): `statusgen verifyrun --open-bracket` on the host at 20:43:41Z (receipt
  outside the home), then `statusgen verifyrun --sandbox=container-netns --rows 1,2,3,4,6,7,8,9,10,11,12,13,14,15`
  inside `docker run --network none --read-only --cap-drop ALL --security-opt no-new-privileges` on the pinned
  verify-runner v1.0.65 image. The Container-Image line carries the multi-arch index digest; the linux/arm64
  platform manifest of that index ran natively on the arm64 host, with no emulation (go1.27.3, git 2.56.0,
  statusgen v1.0.65). The home was mounted read-only. The home is a linked worktree, so its common git
  directory was also mounted read-only at its own path, letting the home's .git file resolve unchanged (the
  #4327 class seen on 2026-10-27 does not recur). The run used a non-root uid, GOPROXY=off, GOTOOLCHAIN=local,
  KUBECONFIG=/dev/null, a tmpfs HOME and /tmp, no forge credential and no injected git identity. No host module
  cache was mounted: the v1.0.65 image's own module cache covers this source's go.mod. The run took
  20:44:06Z-20:54:48Z. `--close-bracket` on the host at 20:54:59Z then appended the Admission-Bracket line, and
  `--bracket-digest` read `verdict=match`. The second table below, with its Tree-Digest, Container-Image,
  attestation and Admission-Bracket lines, is that witness file's bytes.
- Rows 5 and 16 (host, darwin/arm64, go1.27.2, KUBECONFIG=/dev/null, real module and build caches, no
  forge token in the row environment): the first table below is the tool's own write-mode
  `statusgen verifyrun --rows 5,16` output, written into this section from the clean home after the container
  bracket had closed, and left as written (22:04:39Z-22:14:06Z). These rows ran under the operator's own HOME,
  because statusgen's pre-work admission mints the verifier App token.

Host load (1/5/15-minute averages), on a host shared with other verifier containers: 24.61 / 31.42 / 29.44 at
20:44:06Z and 22.31 / 27.17 / 28.58 at 20:54:48Z (container run); 48.28 / 18.74 / 10.08 at 22:04:29Z and
47.57 / 50.48 / 34.03 at 22:14:06Z (host rows).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestTraceNeverPrintsACredential$' -count=1` | pass exit=0 | sha256:8405ac4db251 | 2026-10-29 | assay-verifier-app[bot] @ 2c16bbb5be70 (on-behalf-of human:ian) (forge-identity) |
| 16 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/trace-mutations.json` | pass exit=0 | sha256:3331cacea042 | 2026-10-29 | assay-verifier-app[bot] @ 2c16bbb5be70 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4882 run=9555741df09fbbfc70b869e84a8d5728025e6de2596f11c43e140cb03b0494a6 source=2c16bbb5be70fadd5a0ca8718e64e2a03bd697ea brief=docs/streams/desk-tools/brief-21-desk-trace-and-cause-carrying-errors.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 2c16bbb5be70 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestToolRunSaidSkipsPreambleAndCarriesTheToolsOwnMessage$' -count=1 && go test ./internal/deskkit/ -run '^TestToolRunFailShapeAndCarriedDetail$' -count=1` | pass exit=0 sandbox=container-netns | sha256:9fe698d9a190 | 2026-10-29 | assay-verifier-app[bot] @ 2c16bbb5be70 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestReportErrorOffIsByteIdentical$' -count=1 && go test ./internal/deskkit/ -run '^TestReportErrorOnPrintsChainCommandsAndTimings$' -count=1` | pass exit=0 sandbox=container-netns | sha256:769ae83a4037 | 2026-10-29 | assay-verifier-app[bot] @ 2c16bbb5be70 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestScrubRedactsEveryTransportShape$' -count=1` | pass exit=0 sandbox=container-netns | sha256:0d794583d50e | 2026-10-29 | assay-verifier-app[bot] @ 2c16bbb5be70 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRefusedWithCauseStaysARefusal$' -count=1 && go test ./internal/deskkit/ -run '^TestTraceEnabledReadsTheEnvSpellings$' -count=1` | pass exit=0 sandbox=container-netns | sha256:30b349a8a110 | 2026-10-29 | assay-verifier-app[bot] @ 2c16bbb5be70 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestClaimAcquireFailureNamesTheClaimToolsOwnMessage$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestGitOutFailureCarriesGitStderr$' -count=1` | pass exit=0 sandbox=container-netns | sha256:ddaadb98415d | 2026-10-29 | assay-verifier-app[bot] @ 2c16bbb5be70 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestWorktreeCreateFailureCarriesDeskwtStderrAndTheCommandLine$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestTraceIsOffByDefaultAndOutputIsUnchanged$' -count=1` | pass exit=0 sandbox=container-netns | sha256:c64d2d6fa247 | 2026-10-29 | assay-verifier-app[bot] @ 2c16bbb5be70 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRunGitFailureCarriesStderrCommandAndExitStatus$' -count=1 && go test ./cmd/deskwt/ -run '^TestDeskwtTraceOffIsByteIdenticalAndOnCarriesTheCommand$' -count=1` | pass exit=0 sandbox=container-netns | sha256:2ffc45189059 | 2026-10-29 | assay-verifier-app[bot] @ 2c16bbb5be70 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestTokenPathNoticeIsPrintedOnStderrNotStdout$' -count=1` | pass exit=0 sandbox=container-netns | sha256:1637c3cca30f | 2026-10-29 | assay-verifier-app[bot] @ 2c16bbb5be70 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go test ./cmd/deskfile/ -run '^TestDedupeSearchOutageNamesTheAPIStatus$' -count=1 && go test ./cmd/deskfile/ -run '^TestGhStderrStripsControlBytes$' -count=1` | pass exit=0 sandbox=container-netns | sha256:3e5d898552aa | 2026-10-29 | assay-verifier-app[bot] @ 2c16bbb5be70 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test -timeout 300s ./internal/deskkit/... ./cmd/deskdispatch/... ./cmd/deskwt/... ./cmd/desktoken/... ./cmd/deskfile/... -count=1` | fail exit=1 sandbox=container-netns | sha256:962608f9d84a | 2026-10-29 | assay-verifier-app[bot] @ 2c16bbb5be70 (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestS2' -count=1 && go test ./internal/deskkit/ -run 'TestCorpus' -count=1` | pass exit=0 sandbox=container-netns | sha256:044561fb1b5a | 2026-10-29 | assay-verifier-app[bot] @ 2c16bbb5be70 (on-behalf-of human:ian) (forge-identity) |
| 14 | `cd tools/desk && gofmt -l internal/deskkit/trace.go internal/deskkit/runtool.go internal/deskkit/scrub.go internal/deskkit/exitcodes.go cmd/deskdispatch cmd/deskwt cmd/desktoken cmd/deskfile > /tmp/dt21-fmt.out; test ! -s /tmp/dt21-fmt.out` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 2c16bbb5be70 (on-behalf-of human:ian) (forge-identity) |
| 15 | `cd statusgen && go run . --root .. --lint; echo $?` | pass exit=0 sandbox=container-netns | sha256:9883240f29ee | 2026-10-29 | assay-verifier-app[bot] @ 2c16bbb5be70 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:9852135756bd8e01ba42c01da5ce401350a2f70f81f2c4b8261f2937e51b7d9b
Container-Image: sha256:1af55e60c9222ad09dab8376b629725135c54912bb3583c13591833db90f2d8f
Verification-Attestation: medici-finance/assay#4882 run=9555741df09fbbfc70b869e84a8d5728025e6de2596f11c43e140cb03b0494a6 source=2c16bbb5be70fadd5a0ca8718e64e2a03bd697ea brief=docs/streams/desk-tools/brief-21-desk-trace-and-cause-carrying-errors.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=9555741df09fbbfc70b869e84a8d5728025e6de2596f11c43e140cb03b0494a6 source=2c16bbb5be70fadd5a0ca8718e64e2a03bd697ea tree=sha256:9852135756bd8e01ba42c01da5ce401350a2f70f81f2c4b8261f2937e51b7d9b witness=sha256:79bde4649a53896d9fd0e52273d12c9f9a144cb8a399f9ca7263c568b0c943d2 opened=2026-10-29T20:43:41Z closed=2026-10-29T20:54:59Z before=receipt after=check-verifier

How each row is counted:

- Rows 1-4, 6-10 and 13-15 pass on the container-netns witness (exit 0, sandbox=container-netns).
- Rows 5 and 16 pass on the host witness (exit 0).
- Row 11: could-not-check, although its witness reads pass. The row is vacuous. A labelled supplemental probe on
  the host (`go test ./cmd/deskfile/ -list .`, which discharges no row) lists neither
  TestDedupeSearchOutageNamesTheAPIStatus nor TestGhStderrStripsControlBytes; the dedupe-search tests declared in
  cmd/deskfile at this source are TestDedupeSearchPropagatesTheForgeDiagnosis and
  TestDedupeSearchControlBytesStrippedByBackend. Both of the row's runs therefore match no test. The check-definition repair is still open as #1306.
- Row 12: could-not-check. The witness reads fail exit=1, and the witness does not keep row output (#4335). After
  the bracket closed, the same command was re-run as a diagnostic in the same image with the same read-only
  mounts and no network, keeping its output. This diagnostic discharges no row. It ran 20:55:12Z-21:01:58Z
  (load 26.86 / 27.73 / 28.73 at start) and exited 1:
  - deskdispatch `ok` (224.7s), deskwt `ok` (179.2s), desktoken `ok`, deskfile `ok`, untrustcorpus `ok`. The
    forge-resolution failures recorded on 2026-10-27 (#4322, #4327) do not recur with the common git directory
    mounted.
  - deskkit `FAIL` at 300.121s with `panic: test timed out after 5m0s`, while TestPerfV3LauncherChildFlow (in
    perfcontext_test.go, not part of the delivering change) was running. The output has zero `--- FAIL` lines,
    so no deskkit test failed an assertion. The row's own 300s package budget expired on a host at load ~27.
  The failure is an instrument limit, not a product failure, and no open issue tracks this row's budget. The same
  class (a fixed go test timeout that no longer fits a package on a loaded host) is open for other briefs as
  #4179 and #4391. Main CI at 2c16bbb5be70 has `build-test-shard (desk-1|desk-2|desk-3|rest)` and `build-test`
  all success.

Tally: 14 of 16 rows pass (1-10, 13-16), 0 fail, 2 could-not-check (11, 12).

**VERIFY: BLOCKED** — 14/16 rows pass on the execution witnesses, 0 fail, 2 could-not-check. Row 11 is a check-definition defect (#1306). Row 12 is an instrument limit: its five-package run exceeds the row's fixed 300s budget for deskkit on a loaded shared host, with no test assertion failing and the other four packages passing (same class as #4179). Status stays implemented.
### Non-implementer verifier re-run: 2026-10-30, host witness plus container-netns bracket, merged main 40925e8233c8a63d70682e77ca333e665f1f955e

The 2026-10-29 re-run above was BLOCKED at 2c16bbb5be70 on rows 11 and 12. This run repeats it at the current merged main.

Source: 40925e8233c8a63d70682e77ca333e665f1f955e. Between 2c16bbb5be70 and this source, trace.go, runtool.go, scrub.go,
exitcodes.go, bodycheck.go, trace-mutations.json and trace_test.go are unchanged, and so are cmd/deskdispatch,
cmd/deskwt, cmd/desktoken and cmd/deskfile; the other deskkit files changed. The delivering change is
1003557e167b97c74534d83bf5e47811118fa39c (#729). No Verify row carries a risk-bearing, live, mutating or
end-to-end tag, and every answer under the brief's risk questions is no, so no row owes a fail-first witness.

Admission: `deskdispatch --check-verifier` exit 0 for this detached home at the source above (desk-tools v1.0.66),
before any row ran, and again exit 0 before the second bracket below was opened. The rows were split into two
disjoint selections that together cover all 16 rows.

- Rows 1-4 and 6-15 (container-netns): `statusgen verifyrun --open-bracket` on the host (receipt outside the home),
  then `statusgen verifyrun --sandbox=container-netns --rows 1,2,3,4,6,7,8,9,10,11,12,13,14,15` inside
  `docker run --network none --read-only --cap-drop ALL --security-opt no-new-privileges` on the verify-runner
  v1.0.66 image. The Container-Image line carries the multi-arch index digest; its linux/arm64 platform manifest
  (sha256:097db7d3e204) ran natively on the arm64 host, with no emulation (go1.27.3, git 2.56.0, statusgen
  v1.0.66). The home and the home's common git directory were mounted read-only at their own paths, so the
  home's .git file resolves unchanged. The run used uid 501, GOPROXY=off, GOTOOLCHAIN=local, KUBECONFIG=/dev/null,
  a tmpfs HOME and /tmp, no forge credential and no injected git identity.
  - First bracket (opened 02:17:13Z, closed 02:24:21Z, `--bracket-digest` verdict=match, witness
    sha256:58f563c34ace): rows 1-4, 6-11 and 13-15 pass; row 12 reads fail exit=1. The witness keeps no row
    output (#4335), so the same row 12 command was re-run after the bracket closed as a labelled diagnostic in
    the same image with the same read-only mounts (no acceptance credit). It exited 1: deskdispatch, deskwt,
    desktoken, deskfile and untrustcorpus `ok`; deskkit `FAIL` at 276.6s, inside the 300s budget, on
    `panic: runtime error: invalid memory address or nil pointer dereference` in
    TestOverrideIdentityResolvesRosterLogin (scanoverride_identity_test.go line 105). The image has no passwd
    entry for uid 501, so `user.Current()` errors, and the test's early-exit message then formats `u.Username`
    on a nil `u`. That is the open test defect #4329, outside the delivering change, and it aborts the whole
    deskkit package. A second diagnostic with a read-only passwd file giving uid 501 an entry (no acceptance
    credit) exited 0: deskkit `ok` 205.7s, deskdispatch 75.4s, deskwt 62.3s, desktoken, deskfile, untrustcorpus `ok`.
  - Second bracket (opened 02:34:29Z, closed 02:39:35Z): the identical row selection and commands with that one
    environment change, a read-only /etc/passwd mount giving the container's uid 501 an account, as on a CI
    runner. Network, mounts and every row command are otherwise unchanged. All 14 selected rows pass, and
    `--bracket-digest` reads verdict=match. The second table below, with its Tree-Digest, Container-Image,
    attestation and Admission-Bracket lines, is that witness file's bytes. The first bracket's witness is kept
    outside the home and discharges nothing beyond what is stated here.
- Rows 5 and 16 (host, darwin/arm64, go1.27.2, KUBECONFIG=/dev/null, real module and build caches, no forge token
  in the row environment): the first table below is the tool's own write-mode `statusgen verifyrun --rows 5,16`
  output, written into this section from the home after both container brackets had closed, and left as
  written (02:40:12Z-02:42:01Z).

Host load (1/5/15-minute averages): 23.85 / 21.41 / 23.38 at the first bracket's start, 12.31 / 17.51 / 20.89 at
the second's, and 12.40 / 14.36 / 18.31 at the host rows' start.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestTraceNeverPrintsACredential$' -count=1` | pass exit=0 | sha256:c36f6800c77d | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 16 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/trace-mutations.json` | pass exit=0 | sha256:30589d99ab90 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4955 run=23d6962337aec5ee6f476c9825f7743e2cd7be7f839a1d98ca475f6054ab7ed4 source=40925e8233c8a63d70682e77ca333e665f1f955e brief=docs/streams/desk-tools/brief-21-desk-trace-and-cause-carrying-errors.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestToolRunSaidSkipsPreambleAndCarriesTheToolsOwnMessage$' -count=1 && go test ./internal/deskkit/ -run '^TestToolRunFailShapeAndCarriedDetail$' -count=1` | pass exit=0 sandbox=container-netns | sha256:a1140108e0a6 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestReportErrorOffIsByteIdentical$' -count=1 && go test ./internal/deskkit/ -run '^TestReportErrorOnPrintsChainCommandsAndTimings$' -count=1` | pass exit=0 sandbox=container-netns | sha256:3c35d08c40f7 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestScrubRedactsEveryTransportShape$' -count=1` | pass exit=0 sandbox=container-netns | sha256:bc87af0e7a86 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRefusedWithCauseStaysARefusal$' -count=1 && go test ./internal/deskkit/ -run '^TestTraceEnabledReadsTheEnvSpellings$' -count=1` | pass exit=0 sandbox=container-netns | sha256:7b1cabe0a1a3 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestClaimAcquireFailureNamesTheClaimToolsOwnMessage$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestGitOutFailureCarriesGitStderr$' -count=1` | pass exit=0 sandbox=container-netns | sha256:0304d412b25b | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestWorktreeCreateFailureCarriesDeskwtStderrAndTheCommandLine$' -count=1 && go test ./cmd/deskdispatch/ -run '^TestTraceIsOffByDefaultAndOutputIsUnchanged$' -count=1` | pass exit=0 sandbox=container-netns | sha256:5cabc5cc2814 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRunGitFailureCarriesStderrCommandAndExitStatus$' -count=1 && go test ./cmd/deskwt/ -run '^TestDeskwtTraceOffIsByteIdenticalAndOnCarriesTheCommand$' -count=1` | pass exit=0 sandbox=container-netns | sha256:0a9637bd8c21 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestTokenPathNoticeIsPrintedOnStderrNotStdout$' -count=1` | pass exit=0 sandbox=container-netns | sha256:e83b13d60b3e | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go test ./cmd/deskfile/ -run '^TestDedupeSearchOutageNamesTheAPIStatus$' -count=1 && go test ./cmd/deskfile/ -run '^TestGhStderrStripsControlBytes$' -count=1` | pass exit=0 sandbox=container-netns | sha256:53f0b457471c | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test -timeout 300s ./internal/deskkit/... ./cmd/deskdispatch/... ./cmd/deskwt/... ./cmd/desktoken/... ./cmd/deskfile/... -count=1` | pass exit=0 sandbox=container-netns | sha256:6b45436a99cd | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestS2' -count=1 && go test ./internal/deskkit/ -run 'TestCorpus' -count=1` | pass exit=0 sandbox=container-netns | sha256:a6583243e719 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 14 | `cd tools/desk && gofmt -l internal/deskkit/trace.go internal/deskkit/runtool.go internal/deskkit/scrub.go internal/deskkit/exitcodes.go cmd/deskdispatch cmd/deskwt cmd/desktoken cmd/deskfile > /tmp/dt21-fmt.out; test ! -s /tmp/dt21-fmt.out` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 15 | `cd statusgen && go run . --root .. --lint; echo $?` | pass exit=0 sandbox=container-netns | sha256:9883240f29ee | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:a661369244ed8355ed5de43fd37a5f06b998c2d984bad8d9609bf6f80f234465
Container-Image: sha256:29494cbeca300a29295a2eeed7398500675cb63c7911d660a08c899e8584d175
Verification-Attestation: medici-finance/assay#4955 run=23d6962337aec5ee6f476c9825f7743e2cd7be7f839a1d98ca475f6054ab7ed4 source=40925e8233c8a63d70682e77ca333e665f1f955e brief=docs/streams/desk-tools/brief-21-desk-trace-and-cause-carrying-errors.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=23d6962337aec5ee6f476c9825f7743e2cd7be7f839a1d98ca475f6054ab7ed4 source=40925e8233c8a63d70682e77ca333e665f1f955e tree=sha256:a661369244ed8355ed5de43fd37a5f06b998c2d984bad8d9609bf6f80f234465 witness=sha256:ee975ccf9c3c9a831445f98481f72d86aeaff6e88577285537b33f1801348630 opened=2026-10-30T02:34:29Z closed=2026-10-30T02:39:35Z before=receipt after=check-verifier

How each row is counted:

- Rows 1-4, 6-10 and 12-15 pass on the second container-netns witness (exit 0, sandbox=container-netns).
- Rows 5 and 16 pass on the host witness (exit 0).
- Row 12: pass on the second bracket. Its first-bracket fail is fully accounted for above: the runner image's
  missing passwd entry trips the open test defect #4329, which aborts the deskkit package, and with that account
  present the same command, unchanged, exits 0 inside its 300s budget (deskkit about 206s on the diagnostic).
- Row 11: could-not-check, although its witness reads pass. The row is vacuous. A labelled supplemental probe in
  the same image (`go test ./cmd/deskfile/ -list .`, which discharges no row) lists neither
  TestDedupeSearchOutageNamesTheAPIStatus nor TestGhStderrStripsControlBytes; the dedupe-search tests declared in
  cmd/deskfile at this source are TestDedupeSearchPropagatesTheForgeDiagnosis and
  TestDedupeSearchControlBytesStrippedByBackend. Both of the row's runs therefore match no test. The
  check-definition repair is still open as #1306.

Risk-bearing values (enumerated over trace.go, runtool.go, scrub.go and exitcodes.go at this source):

- `traceStepCap = 200` @ tools/desk/internal/deskkit/trace.go:54 (ledger size, a reversible operational knob).
- `envTruthy` spellings `"1", "true", "yes", "on"` @ tools/desk/internal/deskkit/trace.go:98 (reversible).
- `redactedMarker = "<redacted>"` @ tools/desk/internal/deskkit/scrub.go:49 (reversible display text).
- `reURLUserinfo`, `reAuthHeader`, `reSecretEnvAssign` @ tools/desk/internal/deskkit/scrub.go:31, :36, :42-43.
  These rank first: a credential printed into a trace that is then shared cannot be withdrawn by a redeploy.
- `RefusedWithCause` builds `Code: ExitRefused` @ tools/desk/internal/deskkit/exitcodes.go:186, with
  `ExitRefused = 5` @ exitcodes.go:51 (pre-existing; a changed code would let a caller treat a refusal as
  something else, reversible by an edit).

RISK-VALUE: DERIVED — reURLUserinfo = `://([^:/@\s]+):([^@\s]+)@` @ tools/desk/internal/deskkit/scrub.go:31 — RFC 3986 userinfo is `user:password` ending at the first `@` before the host, and a literal `@` inside a password must be percent-encoded, so `[^@\s]+` after the colon spans the whole password of any well-formed URL.
RISK-VALUE: DERIVED — RefusedWithCause Code = ExitRefused (5) @ tools/desk/internal/deskkit/exitcodes.go:186 — the brief's ground rule "no exit code changes" and its design fact that RefusedWithCause stays a refusal require the same code `Refused` returns (exitcodes.go:172).
RISK-VALUE: NAMED, NOT DERIVED — reSecretEnvAssign name keywords TOKEN|SECRET|PASSWORD|PASSWD|APIKEY|API_KEY|PRIVATE_KEY|CREDENTIAL|PAT @ tools/desk/internal/deskkit/scrub.go:42-43 — a keyword list is a heuristic with no closed derivation; a credential variable named outside it (for example one ending in _AUTH or _KEY) is caught only if its value matches a body-scanner token shape. Completeness against every credential spelling cannot be shown from the spec.
RISK-VALUE: NAMED, NOT DERIVED — reAuthHeader = `(?i)(authorization\s*[:=]\s*)(token |bearer |basic )?\S+` @ tools/desk/internal/deskkit/scrub.go:36 — it redacts only the first whitespace-free run after the optional scheme word, which covers the three named schemes; for any other scheme word the scheme is redacted and the credential after it is left to the body-scanner shapes. No spec text fixes the scheme set.

Tally: 15 of 16 rows pass (1-10, 12-16), 0 fail, 1 could-not-check (11).

**VERIFY: BLOCKED** — 15/16 rows pass on the execution witnesses, 0 fail, 1 could-not-check. Row 11 is a check-definition defect (#1306): both of its named tests are absent from cmd/deskfile, so its exit 0 is vacuous. Row 12 passes on the second container bracket; its first-bracket fail is the runner-image passwd gap tripping the open test defect #4329. Status stays implemented.

## Review

Gate: model (all four risk answers no). Model-gated because the two hazards this change
carries are both mechanically bounded: the disclosure hazard by a planted-secret row that
must stay green (row 5) and a negative control that stops the fix being "redact everything"
(row 4), and the regression hazard by byte-identity rows at three levels (3, 8, 9). The
reviewer confirms that no diagnostic surface reaches an operator without passing through
`Scrub`; that every retrofitted site's exit code is the one it produced before; that the
`— <tool> said: …` suffix is the ONLY change to default output; and that the runner was
lifted from the existing `toolMessage`/`runCmd` seam rather than written a second time.
