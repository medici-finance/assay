---
brief: desk-containers/04
title: interactive desktop launch script (desk-run.sh)
wave: 3
depends: ["desk-containers/02", "desk-containers/03"]
unblocks: ["desk-containers/07"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [63]
schema: brief-v1
authored: 2026-08-22 by desk-containers scoping session
sources:
  - "medici-finance/assay#63 — the request (a shell script I can use to fire them off from my desktop and run them interactively like I currently do)"
  - "docs/streams/desk-containers/spec.md — interactive launch section"
  - "containers/secrets.md (brief 02) — the mount/env contract the script implements"
  - "freshness-checked 2026-08-22 @ b3a2067 — no launch script exists anywhere in the tree"
why: >-
  This is the primary aim of the request: one command on a desktop that fires up a
  chosen desk interactively, exactly like today's terminal sessions, with the
  credentials mounted correctly every time instead of hand-assembled docker flags.
---

# Brief 04 — interactive launch script

## Context

files:
- `containers/desk-run.sh` (new) — the launcher.
- `containers/desk-run.test.sh` (new) — dry-run + negative-path tests (no docker
  daemon required: tests exercise `--dry-run` output and refusal paths).
- `docs/docker.md` — add the "run a desk from your desktop" section.

facts:
- Usage: `desk-run.sh <desk-name> [--version <tag>] [--dry-run] [--attach]`; desk-name
  validated against exactly: intake-desk, worker-desk, pr-review-desk, verify-desk,
  the-desk.
- Composed run (from spec; exact flags this brief's deliverable): `docker run -it
  --name <desk-name>` + per-desk named volume on `/work` + read-only bind of the PEM to
  `/run/secrets/assay/app.pem` + `--env-file` for model credentials + the versioned
  image `ghcr.io/medici-finance/assay/<desk-name>:<tag>`.
- Default host locations for the PEM and env-file are documented in
  `containers/secrets.md` (planned); the script honours overrides via flags/env, and NEVER
  echoes credential VALUES (paths only) in any output including --dry-run.
- Fail-closed preflight: missing PEM file, missing env-file, or unknown desk name →
  refuse with a message naming the expected location; no fallback to ambient host
  credentials.
- Re-attach: if a container named `<desk-name>` already exists, offer/perform
  `docker start -ai <desk-name>` (`--attach`), so a desk's session survives desktop
  restarts with its /work volume intact.
- POSIX-sh or bash with shellcheck-clean discipline; no dependencies beyond docker +
  coreutils.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `desk-run.sh` per the facts above: validation, fail-closed preflight,
   volume + secret + env-file composition per `containers/secrets.md` (planned), `--dry-run`
   printing the exact docker command, `--attach` re-attach path.
2. Implement `desk-run.test.sh` covering: dry-run output for each of the five desks,
   unknown-desk refusal, missing-PEM refusal, missing-env-file refusal, and that no
   test output line contains fixture credential VALUES.
3. Document the desktop flow in `docs/docker.md` (prereqs: docker, the PEM + env-file
   locations; first run vs re-attach).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `shellcheck containers/desk-run.sh` | exit 0 |
| 2 | `sh containers/desk-run.test.sh` | exit 0; prints a PASS line per case listed in Task 2 |
| 3 | `containers/desk-run.sh pr-review-desk --dry-run` (with fixture PEM + env-file paths exported) | exit 0; output contains `-it`, `--name pr-review-desk`, `/work`, `/run/secrets/assay/app.pem`, `ro`, `--env-file`, `ghcr.io/medici-finance/assay/pr-review-desk` |
| 4 | `containers/desk-run.sh nonsense-desk --dry-run` | exit non-zero; output lists the five valid desk names (negative-path row) |
| 5 | `containers/desk-run.sh worker-desk --dry-run` with the PEM path pointing at a missing file | exit non-zero; output names the expected PEM location (fail-closed row) |
| 6 | `containers/desk-run.sh worker-desk --dry-run \| grep -c 'BEGIN'` (fixture PEM in place) | count 0 — the script prints credential PATHS, never contents |

## Definition of Done
- Verify rows green, recorded in Evidence by a non-implementer.
- Credentials are injected at runtime per the brief-02 contract only — the script never
  copies a credential into an image, never bakes one into a derived image, and never
  prints a credential value (row 6). **No secret in any image layer** remains true of
  everything this script produces or runs.
- A first-time user can go from a fresh desktop (docker + PEM + env-file present) to an
  interactive desk session using only `docs/docker.md`.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — a host-side launcher implementing the
human-gated brief-02 contract; it holds no credential itself). Reviewer runs rows 4-6
(the refusal + no-value-echo paths) and confirms the composed docker command matches
`containers/secrets.md` (planned) exactly.
