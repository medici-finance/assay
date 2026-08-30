---
brief: desk-tools/04
title: "Deterministic runner: execute rows, batch ~5 min, sign, file verdict issues"
why: >-
  With verify rows deterministic, the loop needs no model in its hot path — but the verify loop
  still lands via a session pen. This brief makes the drain-engine verify consumer produce SIGNED
  verdict issues instead: the desk's last main-write dependency becomes a plain issue filing, and
  the whole verification cadence survives with zero intelligence in the loop.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-17 by a re-scope session (pr-shepherd); re-homed to the desk-tools board 2026-08-26
sources:
  - "The issue-flow ruling that the verdict issue is authored by the verifier App and that the lane transcribes nothing for FAIL rows beyond including FAIL in the verdict."
  - "The drain engine, whose verify loop is the reference consumer this brief extends."
  - "Direction that ~5 minutes of rows batch into ONE issue and that verifier authorship is the load-bearing fact."
exec-tier: strong
exec-tier-why: "cross-component: consumes the verdict payload + row classes and produces what the transcriber verifies — a drift here fails silently at the lane."
---

# Brief 04 — Deterministic runner: execute rows, batch ~5 min, sign, file verdict issues

## Dependencies
The verdict payload/signing helper and the verify row-classes this originally depended on have
landed outside this stream (done + reviewed), so no typed `depends:` edge remains. This brief
consumes their payload shape and row classes.

## Context
files: `tools/desk/cmd/verifyloop/` (the drain-engine consumer), `tools/desk/internal/deskkit/`
(verdict composition + a verdict-issue rate bucket), plus the row-classes reference doc.

facts:
- executes `check` and `check:ci` rows locally (scripts, exit-code = verdict); gate:human briefs
  stay on verify-gate; gate:model rows stay on the existing model-verify path — this brief
  changes the LANDING, not the judgment lanes.
- batch window ~5 minutes; ONE issue per window, authored by the verifier App (that authorship
  is the load-bearing fact); body composed + signed via `deskverdict`.
- the old self-imposed 3-issues/repo/24h deskfile cap does NOT bind this lane; the cadence IS the
  throttle; GitHub's hard limits remain the backstop — give verdict filings their own deskkit
  bucket so the audit line still records them.
- FAIL rows: include FAIL in the verdict (the lane transcribes nothing for them) AND file/update
  the brief-freshness triage; the loop's ONLY failure behavior is filing — it never diagnoses
  (that judgment is outside the loop; see the escape-valve assist in desk-tools/05).
- fail-closed: missing PEM path / roster / signature failure = file nothing, report the envelope
  error loudly (the operating-envelope preflight pattern).
- session-provenance capture: at Result-time the ENGINE — deterministic conductor code, an
  other-actor by construction, which receives the runner's raw transcript before any model-written
  text touches the record — computes sha256 over that raw transcript and stamps {session id,
  digest, runner id} into the entry's `session` block. The transcript file is retained locally for
  on-dispute production; the digest is what lands. This is the trust rule pushed to execution
  granularity: attestation of what the engine RECEIVED. A session-written digest is worthless by
  design — only the engine populates the block.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Extend the verifyloop drain-engine consumer: pop queue → run the brief's `check`/`check:ci`
   scripts → accumulate row results into the verdict payload shape.
2. Batch: flush at the 5-minute window (or queue-drained, whichever first) → `deskverdict sign`
   → file ONE verifier-App-authored issue labeled `verify-verdict`.
3. Add the verdict-issue rate bucket in `tools/desk/internal/deskkit/ratelimit.go`
   (audit-visible, no daily cap).
4. Dry-run mode (`--dry-run`): compose + sign + print the would-be body without filing — the
   CI-testable surface.

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go test ./cmd/verifyloop/... -run Batch -count=1 && go test ./cmd/verifyloop/... -run Payload -count=1 && go test ./cmd/verifyloop/... -run FailRow -count=1` | exit 0 — batch-window flush logic, payload composition, and FAIL-row inclusion all pass (chained single-pattern runs; `-run` compiles RE2, so a table-cell alternation would match nothing) |
| 2 | `cd tools/desk && go test ./cmd/verifyloop/... -run 'DryRunSignVerify' -count=1` | exit 0 — a dry-run body over a test key verifies with `deskverdict verify`; one flipped byte then refuses |
| 3 | `cd tools/desk && go test ./cmd/verifyloop/... -run 'MissingPEM' -count=1` | exit 0 — a missing-PEM env makes the runner exit non-zero naming the envelope and file nothing |
| 4 | `cd tools/desk && go build -o dist/verifyloop ./cmd/verifyloop && ./dist/verifyloop --dry-run --root ../.. 2>&1 \| grep -q 'signed verdict'` | exit 0; output contains "signed verdict" (against the live local PEM + real queue). `tools/desk` is two levels below the repo root, so the scan root is `../..`; needs the config-home verifier PEM present, else a loud envelope error (could-not-check). |
| 5 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

## Evidence
The runner + batcher (Task steps 1–4) and the verdict-issue rate bucket (Task step 3,
`deskkit.VerdictIssueTool` / `AllowVerdictIssueWrite`) landed on main in
`feat(verifyloop): deterministic verdict runner — run rows, batch, sign`; this brief-PR
makes Verify row 2 a real (non-false-green) check by adding a `DryRunSignVerify` test, fixes
the row-4 scan root, and surfaces the fail-closed envelope error loudly on the CLI.

| # | Command | Exit | Output / hash | Date | Runner |
|---|---------|------|---------------|------|--------|
| 1 | `go test ./cmd/verifyloop/... -run Batch && … -run Payload && … -run FailRow -count=1` | 0 | `ok  …/cmd/verifyloop` ×3 (TestBatchDueToFlush + the ComposePayload test: payload compose, FAIL-row inclusion, provenance digest) | 2026-08-30 | opus-4.8[1m] |
| 2 | `go test ./cmd/verifyloop/... -run 'DryRunSignVerify' -count=1` | 0 | `--- PASS: TestDryRunSignVerify` — dry-run body verifies via `deskkit.VerifyVerdictBody`; one flipped byte → Refused. Fail-first: neutering the byte-flip reds at `want Refused` (verdictrun_test.go:166) | 2026-08-30 | opus-4.8[1m] |
| 3 | `go test ./cmd/verifyloop/... -run 'MissingPEM' -count=1` | 0 | `--- PASS: TestRunVerdictMissingPEM…` — missing PEM ⇒ Unverifiable, files/emits nothing | 2026-08-30 | opus-4.8[1m] |
| 4 | `cd tools/desk && go build -o dist/verifyloop ./cmd/verifyloop && ./dist/verifyloop --dry-run --root ../.. 2>&1 \| grep -q 'signed verdict'` | 0 | `signed verdict for 4 row(s) across 1 brief(s) (window 5m0s) — dry-run: not filed` (live config-home verifier PEM + real awaiting queue) | 2026-08-30 | opus-4.8[1m] |
| 5 | `cd statusgen && go run . --root .. --lint` | 0 | `LINT: PASS` | 2026-08-30 | opus-4.8[1m] |

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
