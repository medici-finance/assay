---
brief: issue-loop/02
title: '`statusgen --scan-issues` — emit placeholders for unhandled open issues across the repos'
wave: 1
depends: ["issue-loop/01"]
unblocks: ["issue-loop/04"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session ([I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md))
sources: ["INTAKE [I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md) (scanner design + label exclusions)", "issue-loop/01 (the schema this emits)", "docs/streams/desk-tools (the fixed repo set + gh-data discipline)", "freshness-checked 2026-07-10 @ post-#288 main"]
why: >-
  The manual half is the labor to kill — the desk hand-converts a handful of issues while
  ~22 app-layer bugs stay invisible to dispatch. The scanner drops a placeholder for every
  unhandled open issue so the WHOLE backlog enters the work model, once, mechanically.
---

# Brief 02 — Issue scanner

## Context
files: `../assay-toolkit/statusgen/` (new `--scan-issues` subcommand), reads `gh issue list` across the
fixed repo set (this repo + agent-runtime + medici-examples, matching deskkit C-4)
facts:
- `--scan-issues` lists OPEN issues per repo; for each issue lacking a placeholder
  (`docs/streams/issue-loop/issue-<NN>.md` absent) and lacking an EXCLUDED label
  (`verify-gate`, `live-verify`, `needs-decision`, and any label naming a system state — the list is a
  documented constant), it WRITES a placeholder per brief 01's schema with the derived gate.
- Idempotency ([I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md) open question): the placeholder FILE's existence is the marker — a
  second run creates nothing for an issue that already has one. A CLOSED issue whose
  placeholder still exists is brief 04's close-out concern, not this brief's.
- It EMITS (writes files) but does not push — the desk reviews the batch and commits (brief
  04 wires the cadence). Dry-run flag (`--scan-issues --dry-run`) lists what it WOULD create
  without writing, for the review-desk to eyeball.
- Network discipline: `--scan-issues` is a distinct subcommand; the offline `--lint` gate
  never calls it. gh failure on one repo → skip that repo with a NOTICE, don't abort the run.

## Ground rules
- NEVER git push to main / trigger workflows / mutating kubectl. The scanner WRITES files;
  committing is the desk's act (brief 04). Leave commits per task only.
- Stop at `implemented`. NEEDS_CONTEXT over guessing.

## Task
1. Implement `--scan-issues` (+ `--dry-run`) per facts; the excluded-label list is a
   documented constant with a comment explaining each entry.
2. Tests against fixture `gh` JSON: emits for an unhandled bug issue; skips one with an
   existing placeholder; skips a `verify-gate` issue; derived gate correct; one-repo gh
   failure degrades to NOTICE not abort.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes the Task-2 cases |
| 2 | `statusgen --root . --scan-issues --dry-run \| head` | lists would-create placeholders; excludes verify-gate/live-verify |
| 3 | `statusgen --root . --lint; echo $?` | 0 (lint never invokes the scanner) |

## Evidence
<!-- non-implementer rows. -->
Verifier run (independent, non-implementer — opus-verifier, merged main `cdf623c5`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | 0 | `ok github.com/medici/statusgen 1.518s` — Task-2 cases in the passing suite | 2026-07-12 | opus-verifier |
| 2 | `go run ./tools/statusgen --root . --scan-issues --dry-run \| head` | 0 | live `gh issue list` read succeeded; printed `would create docs/streams/issue-loop/issue-<NN>.md (…#NN, gate:model\|human)` with derived gates; verify-gate/live-verify issues excluded | 2026-07-12 | opus-verifier |
| 3 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (advisory NOTICEs only); scanner not invoked by lint (no network dependency) | 2026-07-12 | opus-verifier |

**VERIFY: PASS** — `--scan-issues --dry-run` reads open issues (live `gh`) and emits would-create placeholder rows with correctly-derived gates, excluding verify-gate/live-verify issues; lint stays scanner-free. Verified against merged main `cdf623c5` (current when dispatched; main has since advanced — feature is self-contained statusgen).

## Review
Gate: model. Reviewer confirms the exclusion list is complete (no system-label issue
becomes a placeholder), idempotency is file-existence based, and the offline lint gate
gained no network dependency.
