---
brief: assay:assay:desk-containers:04
title: interactive desktop launch script (desk-run.sh)
wave: 3
depends: ["desk-containers/02", "desk-containers/03"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [63]
schema: brief-v2
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
version: 2
id: 8d6363a5-d747-4a47-b4da-52137ebcc3b9
---

# Brief 04 — interactive launch script

## Context

files:
- `containers/desk-run.sh` (new) — the launcher.
- `containers/desk-run.test.sh` (new) — dry-run + negative-path tests (no docker daemon required: tests exercise `--dry-run` output and refusal paths).
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
- CLI built on the desktools-v2 Cobra/Viper foundation (desktools-v2 cli-contract.md §7): every new command, verb or launcher this brief adds uses that shared foundation (added 2026-10-18).
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
| 6 | `test -x containers/desk-run.sh && containers/desk-run.sh worker-desk --dry-run \| { grep -c 'BEGIN' \|\| [ $? -eq 1 ]; }` (fixture PEM in place) | output is `0` — the script prints credential PATHS, never contents. Re-written 2026-10-03 (#1862): every grep stage tolerates only the no-match status, so a missing path or a grep error fails the row instead of passing it. While the script is absent the `test -x` leg fails the row. |

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

### Non-implementer verification — 2026-11-02 — merged main ca6f21abfc90

First verification of this brief, by a fresh non-implementer verifier (claude-opus-5-5, strong tier) in the admitted detached home at ca6f21abfc90cf1d85e6ac64541350d3c3af6205. Gate: model; risk metadata is present and all four answers are no. The Verify table has no class column, so every row is a host row with no network sandbox; no row is check:ci and no container bracket applies. Host: darwin/arm64, ShellCheck 0.11.0, statusgen v1.0.69. Both witness tables below were written in place by `statusgen verifyrun --rows`. The rows ran in two environments because rows 3 and 6 need the fixture PEM present and row 5 needs it missing:

- Rows 1, 2, 3, 4 and 6: ASSAY_DESK_PEM_FILE and ASSAY_DESK_ENV_FILE exported to a placeholder PEM (a fake BEGIN/END block around a fixed placeholder string) and a placeholder env-file, both in the verifier's scratch directory outside the home; XDG_CONFIG_HOME unset; TMPDIR in scratch.
- Row 5: both variables unset and XDG_CONFIG_HOME pointed at an empty scratch config directory that holds only the placeholder env-file, so the PEM path the launcher resolves (its default, the desk's own config-directory PEM) points at a missing file.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `shellcheck containers/desk-run.sh` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-02 | assay-verifier-app[bot] @ ca6f21abfc90 (on-behalf-of human:ian) (forge-identity) |
| 2 | `sh containers/desk-run.test.sh` | pass exit=0 | sha256:7268c29b5dff | 2026-11-02 | assay-verifier-app[bot] @ ca6f21abfc90 (on-behalf-of human:ian) (forge-identity) |
| 3 | `containers/desk-run.sh pr-review-desk --dry-run` | pass exit=0 | sha256:e66d806aaedd | 2026-11-02 | assay-verifier-app[bot] @ ca6f21abfc90 (on-behalf-of human:ian) (forge-identity) |
| 4 | `containers/desk-run.sh nonsense-desk --dry-run` | fail exit=2 | sha256:b2779a18adfb | 2026-11-02 | assay-verifier-app[bot] @ ca6f21abfc90 (on-behalf-of human:ian) (forge-identity) |
| 6 | `test -x containers/desk-run.sh && containers/desk-run.sh worker-desk --dry-run \| { grep -c 'BEGIN' \|\| [ $? -eq 1 ]; }` | pass exit=0 | sha256:9a271f2a916b | 2026-11-02 | assay-verifier-app[bot] @ ca6f21abfc90 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5303 run=45e92725c35491cfcca1fac8448bcb5bc9705e15754b236db3a0ad9bcb17c3d8 source=ca6f21abfc90cf1d85e6ac64541350d3c3af6205 brief=docs/streams/desk-containers/brief-04-launch-script.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 5 | `containers/desk-run.sh worker-desk --dry-run` | fail exit=3 | sha256:b207a985bdf1 | 2026-11-02 | assay-verifier-app[bot] @ ca6f21abfc90 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5303 run=45e92725c35491cfcca1fac8448bcb5bc9705e15754b236db3a0ad9bcb17c3d8 source=ca6f21abfc90cf1d85e6ac64541350d3c3af6205 brief=docs/streams/desk-containers/brief-04-launch-script.md model=claude-opus-5-5 tier=strong

Rows 4 and 5 in the witness. Both Expect cells say "exit non-zero" and name no number. statusgen verifyrun's Expect parser reads an exit only from a number, so for these rows it falls back to its default of exit 0. It then records exit 2 and exit 3 as fail ("exit 2, expected 0" and "exit 3, expected 0"), and `statusgen verifyrun --check` reports 4 pass and 2 fail. These two fail cells come from that fallback. They are not observed failures of the rows' obligations, and the witness gives rows 4 and 5 no credit, so both rows are held as could-not-check on a check-definition blocker. The witness Result cell records the exit, so the output halves of rows 2 to 6 were read from a hand run. Every row was run by hand, verbatim, under `bash -o pipefail -c`, in the same two environments and at the same source. For rows 1, 2, 3 and 6 the hand run reads the output half next to the witness credit. For rows 4 and 5 it is supplemental and earns no acceptance credit:

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | `shellcheck containers/desk-run.sh` | exit 0 | exit 0; no output (no finding) | 2026-11-02 | assay-verifier-app[bot] @ ca6f21abfc90 (claude-opus-5-5 verifier, by hand) (on-behalf-of human:ian) |
| 2 | `sh containers/desk-run.test.sh` | exit 0; a PASS line per Task 2 case | exit 0; 73 PASS lines, 0 FAIL, last line "all cases passed". Task 2 cases: "PASS: dry-run composes the secrets.md docker run for" each of intake-desk, worker-desk, pr-review-desk, verify-desk and the-desk; "PASS: unknown desk is refused and the five valid desks are listed"; "PASS: a missing PEM is refused; the refusal names the slot, not the supplied path"; "PASS: a missing env-file is refused; the refusal names the slot, not the supplied path"; "PASS: no output line of any case contains a fixture credential value" | 2026-11-02 | assay-verifier-app[bot] @ ca6f21abfc90 (claude-opus-5-5 verifier, by hand) (on-behalf-of human:ian) |
| 3 | `containers/desk-run.sh pr-review-desk --dry-run` (fixture PEM + env-file paths exported) | exit 0; output contains -it, --name pr-review-desk, /work, /run/secrets/assay/app.pem, ro, --env-file, ghcr.io/medici-finance/assay/pr-review-desk | exit 0; one line: "docker run -it --name pr-review-desk -v pr-review-desk-work:/work --mount type=bind,src=SCRATCH/app.pem,dst=/run/secrets/assay/app.pem,ro --env-file SCRATCH/desk.env ghcr.io/medici-finance/assay/pr-review-desk:latest" (SCRATCH stands for the verifier's scratch fixture directory). All seven required strings are present | 2026-11-02 | assay-verifier-app[bot] @ ca6f21abfc90 (claude-opus-5-5 verifier, by hand) (on-behalf-of human:ian) |
| 4 | `containers/desk-run.sh nonsense-desk --dry-run` | exit non-zero; output lists the five valid desk names | exit 2; "desk-run.sh: unknown desk name (13 characters; not echoed)" then "valid desks: intake-desk worker-desk pr-review-desk verify-desk the-desk" | 2026-11-02 | assay-verifier-app[bot] @ ca6f21abfc90 (claude-opus-5-5 verifier, by hand) (on-behalf-of human:ian) |
| 5 | `containers/desk-run.sh worker-desk --dry-run` (PEM path pointing at a missing file) | exit non-zero; output names the expected PEM location | exit 3; "desk-run.sh: worker-desk cannot start: the bot App PEM is missing." then "expected a readable file at: SCRATCH-XDG/assay/worker-desk.pem  (default)" then the override hint naming --pem, ASSAY_DESK_PEM_FILE and secrets.md section 2 | 2026-11-02 | assay-verifier-app[bot] @ ca6f21abfc90 (claude-opus-5-5 verifier, by hand) (on-behalf-of human:ian) |
| 6 | `test -x containers/desk-run.sh && containers/desk-run.sh worker-desk --dry-run \| { grep -c 'BEGIN' \|\| [ $? -eq 1 ]; }` (fixture PEM in place) | output is 0 | exit 0; output "0". The fixture PEM read by this run contains a BEGIN line, so a launcher that printed the file would have made the count 1 | 2026-11-02 | assay-verifier-app[bot] @ ca6f21abfc90 (claude-opus-5-5 verifier, by hand) (on-behalf-of human:ian) |

Supplemental observations (no acceptance credit):
- Row 5 with the override form instead (ASSAY_DESK_PEM_FILE pointing at a missing scratch file) also refuses with exit 3. Its second line is "expected a readable file at: the path given by env ASSAY_DESK_PEM_FILE  (value not echoed)": by the launcher's no-echo design a refusal names the slot, never an operator-supplied path, and suite cases "a missing PEM is refused; the refusal names the slot, not the supplied path" and "a missing PEM given by flag is refused; the refusal names --pem, not the path" pin it. Only a location the launcher composes itself is printed, which is why row 5 was run on the default path, where the expected location is named literally.
- `sh containers/desk-run.mutate.sh` (TMPDIR in scratch) exited 0 with "killed 29, survived 0, skipped 0". This includes the print-pem mutant (dry run prints the PEM content), which the row 6 assertion targets, and the ro-mount mutant.
- The composed command matches the containers/secrets.md section 4.1 recipe: docker run -it --name, a per-desk volume on /work, a read-only bind of the PEM to /run/secrets/assay/app.pem, and --env-file. docs/docker.md carries the desktop section: the launcher, first run, --dry-run and --attach.
- The home tree was clean apart from this Evidence edit after every run.

Risk-bearing values. Risk metadata is present and all four answers are no, including irreversible: no. The enumeration covered the delivered launcher, containers/desk-run.sh at ca6f21abfc90:
- registry = "ghcr.io/medici-finance/assay" @ containers/desk-run.sh:51
- desks = "intake-desk worker-desk pr-review-desk verify-desk the-desk" @ containers/desk-run.sh:52
- pem_dst = "/run/secrets/assay/app.pem" @ containers/desk-run.sh:53
- version = "latest" (default tag) @ containers/desk-run.sh:124
- tag length limit 128 @ containers/desk-run.sh:214
- mount option ",ro" in "type=bind,src=$pem,dst=$pem_dst,ro" @ containers/desk-run.sh:370
- volume "$desk-work:/work" @ containers/desk-run.sh:369
- default PEM path $cfg/assay/$desk.pem @ containers/desk-run.sh:300 and env-file path $cfg/assay/desk.env @ containers/desk-run.sh:301
- exit codes 2 (usage/unknown desk), 3 (preflight refused, e.g. :288 and :348) and 4 (docker unavailable or wrong container state, e.g. :385 and :392)

Ranking. Every value can be undone by an edit and a re-release. Nothing here moves funds or publishes anything. The ",ro" option and pem_dst rank first, because they govern whether a desk can write to, or misplace, the mounted App key. registry and the desks list come next, because they decide which image runs. The tag limit, the default tag, the exit codes and the default paths are operational and rank last.
- RISK-VALUE: DERIVED — mount option ",ro" @ containers/desk-run.sh:370 — containers/secrets.md section 1, the credential inventory (line 39), requires the bot App signing key to be a mounted file, read-only. Section 4.1 (line 123) spells the bind as dst=/run/secrets/assay/app.pem,ro. The launcher composes exactly that, and the ro-mount mutant is killed by the suite.
- RISK-VALUE: DERIVED — pem_dst = "/run/secrets/assay/app.pem" @ containers/desk-run.sh:53 — this is the in-container path that containers/secrets.md section 2 (line 55) fixes, and the default ASSAY_APP_PEM_FILE the container entrypoint reads (containers/entrypoint.sh:34). Any other destination would leave the in-container preflight without its key.
- RISK-VALUE: DERIVED — registry = "ghcr.io/medici-finance/assay" @ containers/desk-run.sh:51 — this is the image namespace the brief's facts name (ghcr.io/medici-finance/assay/<desk-name>:<tag>), so the five desk names compose to the five per-desk image references.

**VERIFY: BLOCKED** — rows_passed 4 / rows_total 6 at ca6f21abfc90. Rows 1, 2, 3 and 6 are met by the host execution witness, and the output halves of rows 2, 3 and 6 by hand. Rows 4 and 5 are held could-not-check on the check definition: their Expect says "exit non-zero" with no number, so verifyrun compares them with its default exit 0 and records fail. Hand runs, with no credit, observed exit 2 with the five desk names listed (row 4), and exit 3 with the expected default PEM location named (row 5). No acceptance obligation was observed to fail, and no deliverable defect was found. Gate is model and all four risk answers are no. Next actor: the driver, because re-authoring a delivered Verify row is an acceptance amendment. The remedy is to name the exits ("exit 2" and "exit 3") in the Expect cells of rows 4 and 5, then re-verify.

## Review
Gate: model (all four risk answers no — a host-side launcher implementing the
human-gated brief-02 contract; it holds no credential itself). Reviewer runs rows 4-6
(the refusal + no-value-echo paths) and confirms the composed docker command matches
`containers/secrets.md` (planned) exactly.
