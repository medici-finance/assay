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
version: 3
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
| 4 | `containers/desk-run.sh nonsense-desk --dry-run` | exit 2; output lists the five valid desk names (negative-path row) |
| 5 | `containers/desk-run.sh worker-desk --dry-run` with the PEM path pointing at a missing file | exit 3; output names the expected PEM location (fail-closed row) |
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

### Non-implementer verifier run — 2026-11-02 claude-opus-5-5 verifier at dbfa166d5d96 (re-verify at current main; rows 1, 2, 3 and 6 pass, rows 4 and 5 held on their check definition)

Fresh non-implementer verification on merged main dbfa166d5d96, re-run from scratch because medici-finance/assay#5227 changed docs/docker.md (an input of this brief) after the earlier pass at ca6f21abfc90. Between those two commits only docs/docker.md changed among this brief's inputs (75 added lines, the new compose section); the launcher, its test suite and containers/secrets.md are byte-identical. Admitted by the dispatcher check (exit 0, binding medici-finance/assay#5310 run=4bf30acc56478736, carried in the attestation lines under the tables). No Verify row is classed check:ci, so the container bracket does not apply to any row. The two tables below are the witnesses `statusgen verifyrun` (v1.0.69) wrote in write mode, run on the darwin host from the admitted home on a clean tree, with TMPDIR outside the home. Two runs were needed because rows 5 and 6 ask for opposite fixtures for worker-desk. First run (rows 1, 2, 3, 4 and 6): ASSAY_DESK_PEM_FILE named a three-line placeholder PEM whose first line contains BEGIN (text, not a key) and ASSAY_DESK_ENV_FILE named a one-line placeholder env-file, both outside the home. Second run (row 5 only): both variables unset and XDG_CONFIG_HOME pointing at a directory outside the home that holds the placeholder env-file at assay/desk.env and no worker-desk.pem, so the default PEM location is missing.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `shellcheck containers/desk-run.sh` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-02 | assay-verifier-app[bot] @ dbfa166d5d96 (on-behalf-of human:ian) (forge-identity) |
| 2 | `sh containers/desk-run.test.sh` | pass exit=0 | sha256:7268c29b5dff | 2026-11-02 | assay-verifier-app[bot] @ dbfa166d5d96 (on-behalf-of human:ian) (forge-identity) |
| 3 | `containers/desk-run.sh pr-review-desk --dry-run` | pass exit=0 | sha256:f155a2b1564e | 2026-11-02 | assay-verifier-app[bot] @ dbfa166d5d96 (on-behalf-of human:ian) (forge-identity) |
| 4 | `containers/desk-run.sh nonsense-desk --dry-run` | fail exit=2 | sha256:b2779a18adfb | 2026-11-02 | assay-verifier-app[bot] @ dbfa166d5d96 (on-behalf-of human:ian) (forge-identity) |
| 6 | `test -x containers/desk-run.sh && containers/desk-run.sh worker-desk --dry-run \| { grep -c 'BEGIN' \|\| [ $? -eq 1 ]; }` | pass exit=0 | sha256:9a271f2a916b | 2026-11-02 | assay-verifier-app[bot] @ dbfa166d5d96 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5310 run=4bf30acc56478736fa2636441d598771ee3da38642411e8cac2b84564a39a9ac source=dbfa166d5d967904f4b14956b8735933d48579d6 brief=docs/streams/desk-containers/brief-04-launch-script.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 5 | `containers/desk-run.sh worker-desk --dry-run` | fail exit=3 | sha256:44d555a05162 | 2026-11-02 | assay-verifier-app[bot] @ dbfa166d5d96 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5310 run=4bf30acc56478736fa2636441d598771ee3da38642411e8cac2b84564a39a9ac source=dbfa166d5d967904f4b14956b8735933d48579d6 brief=docs/streams/desk-containers/brief-04-launch-script.md model=claude-opus-5-5 tier=strong

Reading of the witnesses (plain prose, no row credit):
- `statusgen verifyrun --check` on the brief: 4 pass, 2 fail, 0 could-not-run or missing of 6 Verify rows.
- Rows 4 and 5 meet their written Expect, but the witness cannot credit them. Their Expect cells say exit non-zero without a number, and statusgen verifyrun reads an Expect with no exit number as exit 0, so it records the observed exit 2 and exit 3 as fail. The same shape as the earlier pass at ca6f21abfc90. A supplemental hand re-run of each row command from the home, with the same fixtures (no row credit), reproduced all six witness digests exactly. Row 4 exits 2 and prints "unknown desk name (13 characters; not echoed)" and then "valid desks: intake-desk worker-desk pr-review-desk verify-desk the-desk", which lists the five valid desk names. Row 5 exits 3 and prints "the bot App PEM is missing", then "expected a readable file at:" followed by the fixture directory's assay/worker-desk.pem marked (default), which names the expected PEM location. Neither is a deliverable defect; the hold is on the check definition. The fix is to re-author the Expect cells of rows 4 and 5 to exit 2 and exit 3 (the launcher's documented codes for an unknown desk and a refused credential preflight), then re-verify.
- Row 2 printed 73 PASS lines and "all cases passed". The Task 2 cases are among them: a dry-run line for each of the five desks, the unknown-desk refusal with the five names listed, the missing-PEM refusal, the missing-env-file refusal, and "no output line of any case contains a fixture credential value".
- Row 3's output is one line: docker run -it --name pr-review-desk -v pr-review-desk-work:/work --mount type=bind,src=(fixture PEM path),dst=/run/secrets/assay/app.pem,ro --env-file (fixture env-file path) ghcr.io/medici-finance/assay/pr-review-desk:latest. Every token the Expect names is present, and the shape matches the docker run in containers/secrets.md section 4.1 word for word, as the Review section asks.
- Row 6 prints 0 and exits 0 although the fixture PEM's first line contains BEGIN. Supplemental mutation probe in a throwaway copy of the launcher outside the home (no row credit): making the dry-run branch print the PEM's contents first changes the row's output to 1, with exit 0. So row 6 discriminates on its output, not its exit status.
- docs/docker.md at this source still carries the "Run a desk from your desktop" section (prerequisites with the per-desk PEM and env-file locations, first run, dry-run and re-attach). The change from medici-finance/assay#5227 adds a compose section after it and leaves the desktop section unchanged. The Definition of Done's walk from a fresh desktop to a live session needs a docker engine and a published image; no Verify row covers it, and it is not attested here.

Enumeration (risk-bearing values). The brief carries risk fields that are all no, and it is not irreversible. Literals the launcher introduces in containers/desk-run.sh: registry = ghcr.io/medici-finance/assay (line 51), the five desk names (line 52), pem_dst = /run/secrets/assay/app.pem (line 53), version default latest (line 124), the 128-character tag limit (line 214), the default PEM path config/assay/(desk).pem and env-file path config/assay/desk.env (lines 300 and 301), the per-desk volume name (desk)-work on /work (line 369), the ro mount option (line 370), and exit codes 2, 3 and 4 (lines 42 to 44). Ranked by what breaks if wrong: the PEM target first (a desk would start without its App key at the path the entrypoint reads), then the ro option (a writable credential mount), then the per-desk default PEM path (two desks sharing one App identity), then the rest. Every one is undone by an edit; none is irreversible.

RISK-VALUE: DERIVED — pem_dst = /run/secrets/assay/app.pem @ containers/desk-run.sh:53 — it is the path the runtime contract fixes in containers/secrets.md section 2 (line 55) and section 4.1, and the default the image's entrypoint reads (containers/entrypoint.sh:34), so the launcher mounts the key exactly where the container looks for it.

RISK-VALUE: DERIVED — mount options = type=bind,src=$pem,dst=$pem_dst,ro @ containers/desk-run.sh:370 — containers/secrets.md line 39 requires the PEM as a read-only mounted file and section 4.1 gives the same --mount line ending in ro; the test suite also pins ro as the last mount option.

RISK-VALUE: DERIVED — default PEM = $cfg/assay/$desk.pem @ containers/desk-run.sh:300 — one file per desk, so two desks on defaults never mount the same App key; docs/docker.md states the supported minimum of three distinct App identities (implementer, reviewer, verifier), which a shared default would break.

RISK-VALUE: DERIVED — tag length limit = 128 @ containers/desk-run.sh:214 — the docker image reference grammar allows a tag of one word character followed by at most 127 word, dot or dash characters, so 128 is the largest tag docker accepts.

**VERIFY: BLOCKED** — 4/6 Verify rows pass with 0 observed failed obligations at dbfa166d5d96; rows 4 and 5 are held on their check definition (Expect says exit non-zero with no number, which the witness reads as exit 0; observed exit 2 and exit 3 meet the written Expect). Blocker kind check-definition, routed at https://github.com/medici-finance/assay/issues/63#issuecomment-6568642807. Gate: model; status left where it is.
### Non-implementer verifier run — 2026-11-06 claude-opus-5-5 verifier at d497f2a732e4 (brief version 3; all six Verify rows pass)

Fresh non-implementer verification of the whole current Verify table on merged main d497f2a732e4, after #5312 (brief version 3) named the exit codes in rows 4 and 5 (exit 2 and exit 3). Nothing from earlier Evidence is reused. Since the earlier run at dbfa166d5d96 the only change among this brief's inputs is the brief itself (the two Expect cells and the earlier Evidence landing); the launcher, its test suite, containers/secrets.md, containers/entrypoint.sh and docs/docker.md are byte-identical. Admitted by the dispatcher check (exit 0, binding #5539 run=ab2d3a0dc105aca0, carried in the attestation lines under the tables). No Verify row is classed check:ci, so the container bracket does not apply; the two tables below are the witnesses `statusgen verifyrun --fail-first` (v1.0.75) wrote in write mode on the darwin arm64 host from the admitted home on a clean tree, with TMPDIR and every fixture outside the home and KUBECONFIG = /dev/null. Fail-first base = 40a7aace82c8 (first parent of the merge of #5152, which delivered containers/desk-run.sh and its suite). Two runs were needed because rows 5 and 6 ask for opposite fixtures for worker-desk. First run (rows 1, 2, 3, 4 and 6): ASSAY_DESK_PEM_FILE named a three-line placeholder PEM whose first line contains BEGIN (text, not a key) and ASSAY_DESK_ENV_FILE named a one-line placeholder env-file. Second run (row 5 only): both variables unset and XDG_CONFIG_HOME pointing at a directory that holds the placeholder env-file at assay/desk.env and no worker-desk.pem, so the default PEM location is missing.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `shellcheck containers/desk-run.sh` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ d497f2a732e4 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=d497f2a732e4 |
| 2 | `sh containers/desk-run.test.sh` | pass exit=0 | sha256:7268c29b5dff | 2026-11-06 | assay-verifier-app[bot] @ d497f2a732e4 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=d497f2a732e4 |
| 3 | `containers/desk-run.sh pr-review-desk --dry-run` | pass exit=0 | sha256:1cefd35f3512 | 2026-11-06 | assay-verifier-app[bot] @ d497f2a732e4 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=d497f2a732e4 |
| 4 | `containers/desk-run.sh nonsense-desk --dry-run` | pass exit=2 | sha256:b2779a18adfb | 2026-11-06 | assay-verifier-app[bot] @ d497f2a732e4 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=d497f2a732e4 |
| 6 | `test -x containers/desk-run.sh && containers/desk-run.sh worker-desk --dry-run \| { grep -c 'BEGIN' \|\| [ $? -eq 1 ]; }` | pass exit=0 | sha256:9a271f2a916b | 2026-11-06 | assay-verifier-app[bot] @ d497f2a732e4 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=d497f2a732e4 |
Verification-Attestation: medici-finance/assay#5539 run=ab2d3a0dc105aca0980a1ca31e4e1cc61ee3704a2f01b1ad50b13371684e728d source=d497f2a732e444873b55fcac2d20c3577954feb6 brief=docs/streams/desk-containers/brief-04-launch-script.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 5 | `containers/desk-run.sh worker-desk --dry-run` | pass exit=3 | sha256:64a259b491cc | 2026-11-06 | assay-verifier-app[bot] @ d497f2a732e4 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=d497f2a732e4 |
Verification-Attestation: medici-finance/assay#5539 run=ab2d3a0dc105aca0980a1ca31e4e1cc61ee3704a2f01b1ad50b13371684e728d source=d497f2a732e444873b55fcac2d20c3577954feb6 brief=docs/streams/desk-containers/brief-04-launch-script.md model=claude-opus-5-5 tier=strong

Reading of the witnesses (plain prose, no row credit):
- `statusgen verifyrun --check` on the brief: 6 pass, 0 fail, 0 could-not-run or missing of 6 Verify rows; 0 author-added rows.
- Fail-first: statusgen classes no row of this table risk-bearing (the brief's risk fields are all no and no row carries a risk-bearing, live, mutating or end-to-end tag), and the scoped lint names no finding on this brief, so the tool recorded every row not-selected at base. Supplemental base probe (no row credit): with the containers tree of base 40a7aace82c8 extracted outside the home and the same fixtures, every row command fails, because containers/desk-run.sh does not exist there: row 1 exit 2, rows 2, 3, 4 and 5 exit 127, row 6 exit 1 on its test -x leg. Row 4's exit 127 and row 5's exit 127 differ from the exit 2 and exit 3 their Expect cells now name, so neither can pass at base.
- A supplemental hand re-run of each row command from the home with the same fixtures (no row credit) reproduced all six witness digests exactly.
- Row 2 printed 73 PASS lines, no FAIL line, and "all cases passed". The Task 2 cases are among them: "dry-run composes the secrets.md docker run for" each of intake-desk, worker-desk, pr-review-desk, verify-desk and the-desk; "unknown desk is refused and the five valid desks are listed"; "a missing PEM is refused; the refusal names the slot, not the supplied path"; "a missing env-file is refused; the refusal names the slot, not the supplied path"; "no output line of any case contains a fixture credential value".
- Row 3's output is one line: docker run -it --name pr-review-desk -v pr-review-desk-work:/work --mount type=bind,src=(fixture PEM path),dst=/run/secrets/assay/app.pem,ro --env-file (fixture env-file path) ghcr.io/medici-finance/assay/pr-review-desk:latest. Every token the Expect names is present, and the shape matches the docker run in containers/secrets.md section 4.1 (line 123), as the Review section asks.
- Row 4 exits 2 and prints "desk-run.sh: unknown desk name (13 characters; not echoed)" then "valid desks: intake-desk worker-desk pr-review-desk verify-desk the-desk", the five valid desk names.
- Row 5 exits 3 and prints "desk-run.sh: worker-desk cannot start: the bot App PEM is missing." then "expected a readable file at:" followed by the fixture directory's assay/worker-desk.pem marked (default), which names the expected PEM location.
- Row 6 prints 0 and exits 0 although the fixture PEM's first line contains BEGIN. Supplemental mutation probe in a copy of the launcher outside the home (no row credit): making the dry-run branch print the PEM's contents first changes the row's output to 1, with exit 0. So row 6 discriminates on its output, not its exit status.
- docs/docker.md at this source carries the "Run a desk from your desktop" section (line 179: prerequisites with the per-desk PEM and env-file locations, first run, dry-run and re-attach). The Definition of Done's walk from a fresh desktop to a live session needs a docker engine and a published image; no Verify row covers it, and this run makes no claim about it.

Enumeration (risk-bearing values). The brief carries risk fields that are all no, and it is not irreversible. Literals the launcher introduces in containers/desk-run.sh (unchanged since delivery): registry = ghcr.io/medici-finance/assay (line 51), the five desk names (line 52), pem_dst = /run/secrets/assay/app.pem (line 53), version default = latest (line 124), the 128-character tag limit (line 214), the default PEM path cfg/assay/(desk).pem and env-file path cfg/assay/desk.env (lines 300 and 301), the per-desk volume (desk)-work on /work (line 369), the ro mount option (line 370), and exit codes 2, 3 and 4 (lines 42 to 44). Ranked by what breaks if wrong: the PEM target first (a desk would start without its App key at the path the entrypoint reads), then the ro option (a writable credential mount), then the per-desk default PEM path (two desks sharing one App identity), then the tag limit, then the rest (registry, names, volume, exit codes, default tag), which are operational. Every one is undone by an edit; none is irreversible.

RISK-VALUE: DERIVED — pem_dst = /run/secrets/assay/app.pem @ containers/desk-run.sh:53 — it is the path the runtime contract fixes in containers/secrets.md section 2 (line 55) and section 4.1 (line 123), and the default the image's entrypoint reads (containers/entrypoint.sh:34), so the launcher mounts the key exactly where the container looks for it.

RISK-VALUE: DERIVED — mount options = type=bind,src=$pem,dst=$pem_dst,ro @ containers/desk-run.sh:370 — containers/secrets.md line 39 requires the PEM as a read-only mounted file and section 4.1 (line 123) gives the same mount line ending in ro; the suite also pins ro as the last mount option.

RISK-VALUE: DERIVED — default PEM = $cfg/assay/$desk.pem @ containers/desk-run.sh:300 — one file per desk, so two desks on defaults never mount the same App key; docs/docker.md line 200 states the supported minimum of three distinct App identities (implementer, reviewer, verifier), which a shared default would break.

RISK-VALUE: DERIVED — tag length limit = 128 @ containers/desk-run.sh:214 — the docker image reference grammar allows a tag of one word character followed by at most 127 word, dot or dash characters, so 128 is the largest tag docker accepts.

**VERIFY: PASS** — 6/6 Verify rows pass at d497f2a732e4 with 0 failed and 0 could-not-run; rows 4 and 5 now meet their numbered Expect (exit 2 and exit 3). Gate: model.

### Non-implementer verifier run — 2026-11-06 claude-opus-5-5 verifier at 8f5bb307de31 (whole Verify table re-run at current main)

Fresh non-implementer verification of the whole current Verify table on merged main 8f5bb307de31. Nothing from earlier Evidence is reused. Since the earlier run at d497f2a732e4 the only change among this brief's inputs is the brief itself (the Evidence landing bf49c7a35015); the containers tree (desk-run.sh, desk-run.test.sh, secrets.md, entrypoint.sh) and docs/docker.md are byte-identical. Admitted by the dispatcher check (exit 0, binding medici-finance/assay#5611 run=bb2bad6666fd2089, carried in the attestation lines under the tables). No Verify row is classed check:ci, so the container bracket does not apply; the two tables below are the witnesses `statusgen verifyrun --fail-first --base 40a7aace82c8` (v1.0.75) wrote in write mode on the darwin arm64 host from the admitted home on a clean tree, under the normal HOME, with GH_TOKEN unset, KUBECONFIG = /dev/null, and TMPDIR and every fixture in a scratch directory outside the home. Fail-first base = 40a7aace82c8, the first parent of 282647d8e9a8 (the merge of medici-finance/assay#5152, which delivered containers/desk-run.sh and its suite). Two invocations were needed because rows 5 and 6 ask for opposite fixtures for worker-desk. First (rows 1, 2, 3, 4 and 6): ASSAY_DESK_PEM_FILE named a three-line placeholder PEM whose first line contains BEGIN (text, not a key) and ASSAY_DESK_ENV_FILE named a one-line placeholder env-file. Second (row 5 only): both variables unset and XDG_CONFIG_HOME pointing at a directory that holds the placeholder env-file at assay/desk.env and no worker-desk.pem, so the default PEM location is missing.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `shellcheck containers/desk-run.sh` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=8f5bb307de31 |
| 2 | `sh containers/desk-run.test.sh` | pass exit=0 | sha256:7268c29b5dff | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=8f5bb307de31 |
| 3 | `containers/desk-run.sh pr-review-desk --dry-run` | pass exit=0 | sha256:3ac810b4a3dd | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=8f5bb307de31 |
| 4 | `containers/desk-run.sh nonsense-desk --dry-run` | pass exit=2 | sha256:b2779a18adfb | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=8f5bb307de31 |
| 6 | `test -x containers/desk-run.sh && containers/desk-run.sh worker-desk --dry-run \| { grep -c 'BEGIN' \|\| [ $? -eq 1 ]; }` | pass exit=0 | sha256:9a271f2a916b | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=8f5bb307de31 |
Verification-Attestation: medici-finance/assay#5611 run=bb2bad6666fd208982a323d3afb50866146268ac70aa9d1c0424a5106a274293 source=8f5bb307de31c64b7c96e38d631926fec8c69211 brief=docs/streams/desk-containers/brief-04-launch-script.md model=claude-opus-5-5 tier=any

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 5 | `containers/desk-run.sh worker-desk --dry-run` | pass exit=3 | sha256:719ad221cb52 | 2026-11-06 | assay-verifier-app[bot] @ 8f5bb307de31 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=8f5bb307de31 |
Verification-Attestation: medici-finance/assay#5611 run=bb2bad6666fd208982a323d3afb50866146268ac70aa9d1c0424a5106a274293 source=8f5bb307de31c64b7c96e38d631926fec8c69211 brief=docs/streams/desk-containers/brief-04-launch-script.md model=claude-opus-5-5 tier=any

Reading of the witnesses (plain prose, no row credit):
- `statusgen verifyrun --check` on the brief: 6 pass, 0 fail, 0 could-not-run or missing of 6 Verify rows; 0 author-added rows (exit 0).
- Fail-first: statusgen classes no row of this table risk-bearing (the brief's risk fields are all no and no row carries a risk-bearing, live, mutating or end-to-end tag), so the tool recorded every row not-selected at base. Supplemental base probe (no row credit): with the containers tree of base 40a7aace82c8 extracted outside the home and the same fixtures, every row command fails, because containers/desk-run.sh does not exist there: row 1 exit 2, rows 2, 3, 4 and 5 exit 127, row 6 exit 1 on its test -x leg. Rows 4 and 5 at base exit 127, not the exit 2 and exit 3 their Expect cells name, so neither can pass at base.
- A supplemental hand re-run of each row command from the home with the same fixtures (no row credit) gave the same exit codes, and the row 3 and row 5 output digests (sha256:3ac810b4a3dd, sha256:719ad221cb52) reproduced exactly.
- Row 2 printed 73 PASS lines, no FAIL line, and "all cases passed". The Task 2 cases are among them: "dry-run composes the secrets.md docker run for" each of intake-desk, worker-desk, pr-review-desk, verify-desk and the-desk; "unknown desk is refused and the five valid desks are listed"; "a missing PEM is refused; the refusal names the slot, not the supplied path"; "a missing env-file is refused; the refusal names the slot, not the supplied path"; "no output line of any case contains a fixture credential value".
- Row 3's output is one line: docker run -it --name pr-review-desk -v pr-review-desk-work:/work --mount type=bind,src=(fixture PEM path),dst=/run/secrets/assay/app.pem,ro --env-file (fixture env-file path) ghcr.io/medici-finance/assay/pr-review-desk:latest. Every token the Expect names is present, and the shape matches the docker run in containers/secrets.md section 4.1 (line 123), as the Review section asks.
- Row 4 exits 2 and prints "desk-run.sh: unknown desk name (13 characters; not echoed)" then "valid desks: intake-desk worker-desk pr-review-desk verify-desk the-desk", the five valid desk names.
- Row 5 exits 3 and prints "desk-run.sh: worker-desk cannot start: the bot App PEM is missing." then "expected a readable file at:" followed by the fixture directory's assay/worker-desk.pem marked (default), which names the expected PEM location.
- Row 6 prints 0 and exits 0 although the fixture PEM's first line contains BEGIN. Supplemental mutation probe in a copy of the launcher outside the home (no row credit): adding one line that prints the PEM's contents at the top of the dry-run branch changes the row's output to 1, with exit 0. So row 6 discriminates on its output, not its exit status.
- docs/docker.md at this source carries the "Run a desk from your desktop" section (line 179: prerequisites with the per-desk PEM and env-file locations, first run, dry-run and re-attach). The Definition of Done's walk from a fresh desktop to a live session needs a docker engine and a published image; no Verify row covers it, and this run makes no claim about it.

Enumeration (risk-bearing values). The brief carries risk fields that are all no, and it is not irreversible. Literals the launcher introduces in containers/desk-run.sh (unchanged since delivery): registry = ghcr.io/medici-finance/assay (line 51), the five desk names (line 52), pem_dst = /run/secrets/assay/app.pem (line 53), version default = latest (line 124), the 128-character tag limit (line 214), the default PEM path cfg/assay/(desk).pem and env-file path cfg/assay/desk.env (lines 300 and 301), the per-desk volume (desk)-work on /work (line 369), the ro mount option (line 370), and exit codes 2, 3 and 4 (lines 42 to 44). Ranked by what breaks if wrong: the PEM target first (a desk would start without its App key at the path the entrypoint reads), then the ro option (a writable credential mount), then the per-desk default PEM path (two desks sharing one App identity), then the tag limit, then the rest (registry, names, volume, exit codes, default tag), which are operational. Every one is undone by an edit; none is irreversible.

RISK-VALUE: DERIVED — pem_dst = /run/secrets/assay/app.pem @ containers/desk-run.sh:53 — it is the path the runtime contract fixes in containers/secrets.md section 2 (line 55) and section 4.1 (line 123), and the default the image's entrypoint reads (containers/entrypoint.sh:34), so the launcher mounts the key exactly where the container looks for it.

RISK-VALUE: DERIVED — mount options = type=bind,src=$pem,dst=$pem_dst,ro @ containers/desk-run.sh:370 — containers/secrets.md line 39 requires the PEM as a read-only mounted file and section 4.1 (line 123) gives the same mount line ending in ro; the suite also pins ro as the last mount option.

RISK-VALUE: DERIVED — default PEM = $cfg/assay/$desk.pem @ containers/desk-run.sh:300 — one file per desk, so two desks on defaults never mount the same App key; docs/docker.md line 200 states the supported minimum of three distinct App identities (implementer, reviewer, verifier), which a shared default would break.

RISK-VALUE: DERIVED — tag length limit = 128 @ containers/desk-run.sh:214 — the docker image reference grammar allows a tag of one word character followed by at most 127 word, dot or dash characters, so 128 is the largest tag docker accepts.

**VERIFY: PASS** — 6/6 Verify rows pass at 8f5bb307de31 with 0 failed and 0 could-not-run. Gate: model.

### Non-implementer verifier run — 2026-11-08 claude-opus-5-5 verifier at d725ec020dd6 (whole Verify table re-run at current main, desk tools v1.0.78)

Fresh non-implementer verification of the whole current Verify table on merged main d725ec020dd6, a re-verification under desk tools v1.0.78 woken by a change to this item's inputs. Nothing from earlier Evidence is reused. Since the earlier run at 8f5bb307de31 the only change among this brief's inputs is the brief itself (the Evidence landing 569045495c69); the containers tree (desk-run.sh, desk-run.test.sh, secrets.md, entrypoint.sh) and docs/docker.md are byte-identical. Admitted by the dispatcher check (exit 0, binding medici-finance/assay#5715 run=5b8d9268bc676fad, carried in the attestation lines under the tables). No Verify row is classed check:ci, so the container bracket does not apply; the two tables below are the witnesses `statusgen verifyrun --fail-first --base 40a7aace82c8` (v1.0.78) wrote in write mode on the darwin arm64 host from the admitted home on a clean tree, under the normal HOME, with GH_TOKEN unset, KUBECONFIG = /dev/null, and TMPDIR and every fixture in a scratch directory outside the home. Fail-first base = 40a7aace82c8, the first parent of 282647d8e9a8 (the merge of medici-finance/assay#5152, which delivered containers/desk-run.sh and its suite). Two invocations were needed because rows 5 and 6 ask for opposite fixtures for worker-desk. First (rows 1, 2, 3, 4 and 6): ASSAY_DESK_PEM_FILE named a three-line placeholder PEM whose first line contains BEGIN (text, not a key) and ASSAY_DESK_ENV_FILE named a one-line placeholder env-file. Second (row 5 only): both variables unset and XDG_CONFIG_HOME pointing at a directory that holds the placeholder env-file at assay/desk.env and no worker-desk.pem, so the default PEM location is missing.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `shellcheck containers/desk-run.sh` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-08 | assay-verifier-app[bot] @ d725ec020dd6 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=d725ec020dd6 |
| 2 | `sh containers/desk-run.test.sh` | pass exit=0 | sha256:7268c29b5dff | 2026-11-08 | assay-verifier-app[bot] @ d725ec020dd6 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=d725ec020dd6 |
| 3 | `containers/desk-run.sh pr-review-desk --dry-run` | pass exit=0 | sha256:d340acbb4533 | 2026-11-08 | assay-verifier-app[bot] @ d725ec020dd6 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=d725ec020dd6 |
| 4 | `containers/desk-run.sh nonsense-desk --dry-run` | pass exit=2 | sha256:b2779a18adfb | 2026-11-08 | assay-verifier-app[bot] @ d725ec020dd6 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=d725ec020dd6 |
| 6 | `test -x containers/desk-run.sh && containers/desk-run.sh worker-desk --dry-run \| { grep -c 'BEGIN' \|\| [ $? -eq 1 ]; }` | pass exit=0 | sha256:9a271f2a916b | 2026-11-08 | assay-verifier-app[bot] @ d725ec020dd6 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=d725ec020dd6 |
Verification-Attestation: medici-finance/assay#5715 run=5b8d9268bc676fad8a4984d840a2b32efc7d815e66102ac46ceb0c2ac1086343 source=d725ec020dd66d4ff160ff8c02abbab6bb117631 brief=docs/streams/desk-containers/brief-04-launch-script.md model=claude-opus-5-5 tier=any

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 5 | `containers/desk-run.sh worker-desk --dry-run` | pass exit=3 | sha256:4325f6f98693 | 2026-11-08 | assay-verifier-app[bot] @ d725ec020dd6 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=d725ec020dd6 |
Verification-Attestation: medici-finance/assay#5715 run=5b8d9268bc676fad8a4984d840a2b32efc7d815e66102ac46ceb0c2ac1086343 source=d725ec020dd66d4ff160ff8c02abbab6bb117631 brief=docs/streams/desk-containers/brief-04-launch-script.md model=claude-opus-5-5 tier=any

Reading of the witnesses (plain prose, no row credit):
- `statusgen verifyrun --check` on the brief: 6 pass, 0 fail, 0 could-not-run or missing of 6 Verify rows; 0 author-added rows (exit 0).
- Fail-first: statusgen classes no row of this table risk-bearing (the brief's risk fields are all no and no row carries a risk-bearing, live, mutating or end-to-end tag), so the tool recorded every row not-selected at base. Supplemental base probe (no row credit): with the containers tree of base 40a7aace82c8 extracted outside the home and the same fixtures, every row command fails, because containers/desk-run.sh does not exist there: row 1 exit 2, rows 2, 3, 4 and 5 exit 127, row 6 exit 1 on its test -x leg. Rows 4 and 5 at base exit 127, not the exit 2 and exit 3 their Expect cells name, so neither can pass at base.
- A supplemental hand re-run of each row command from the home with the same fixtures (no row credit) gave the same exit codes, and the row 3 and row 5 output digests (sha256:d340acbb4533, sha256:4325f6f98693) reproduced exactly. Those two digests differ from the earlier run's only because their output names the scratch fixture paths, which are new for this run.
- Row 2 printed 73 PASS lines, no FAIL line, and "all cases passed". The Task 2 cases are among them: "dry-run composes the secrets.md docker run for" each of intake-desk, worker-desk, pr-review-desk, verify-desk and the-desk; "unknown desk is refused and the five valid desks are listed"; "a missing PEM is refused; the refusal names the slot, not the supplied path"; "a missing env-file is refused; the refusal names the slot, not the supplied path"; "no output line of any case contains a fixture credential value".
- Row 3's output is one line: docker run -it --name pr-review-desk -v pr-review-desk-work:/work --mount type=bind,src=(fixture PEM path),dst=/run/secrets/assay/app.pem,ro --env-file (fixture env-file path) ghcr.io/medici-finance/assay/pr-review-desk:latest. Every token the Expect names is present, and the shape matches the docker run in containers/secrets.md section 4.1 (line 123), as the Review section asks.
- Row 4 exits 2 and prints "desk-run.sh: unknown desk name (13 characters; not echoed)" then "valid desks: intake-desk worker-desk pr-review-desk verify-desk the-desk", the five valid desk names.
- Row 5 exits 3 and prints "desk-run.sh: worker-desk cannot start: the bot App PEM is missing." then "expected a readable file at:" followed by the fixture directory's assay/worker-desk.pem marked (default), which names the expected PEM location.
- Row 6 prints 0 and exits 0 although the fixture PEM's first line contains BEGIN. Supplemental mutation probe in a copy of the launcher outside the home (no row credit): adding one line that prints the PEM's contents at the top of the dry-run branch changes the row's output to 1, with exit 0. So row 6 discriminates on its output, not its exit status.
- docs/docker.md at this source carries the "Run a desk from your desktop" section (line 179: prerequisites with the per-desk PEM and env-file locations, first run, dry-run and re-attach). The Definition of Done's walk from a fresh desktop to a live session needs a docker engine and a published image; no Verify row covers it, and this run makes no claim about it.

Enumeration (risk-bearing values). The brief carries risk fields that are all no, and it is not irreversible. Literals the launcher introduces in containers/desk-run.sh (unchanged since delivery): registry = ghcr.io/medici-finance/assay (line 51), the five desk names (line 52), pem_dst = /run/secrets/assay/app.pem (line 53), version default = latest (line 124), the 128-character tag limit (line 214), the default PEM path cfg/assay/(desk).pem and env-file path cfg/assay/desk.env (lines 300 and 301), the per-desk volume (desk)-work on /work (line 369), the ro mount option (line 370), and exit codes 2, 3 and 4 (lines 42 to 44). Ranked by what breaks if wrong: the PEM target first (a desk would start without its App key at the path the entrypoint reads), then the ro option (a writable credential mount), then the per-desk default PEM path (two desks sharing one App identity), then the tag limit, then the rest (registry, names, volume, exit codes, default tag), which are operational. Every one is undone by an edit; none is irreversible.

RISK-VALUE: DERIVED — pem_dst = /run/secrets/assay/app.pem @ containers/desk-run.sh:53 — it is the path the runtime contract fixes in containers/secrets.md section 2 (line 55) and section 4.1 (line 123), and the default the image's entrypoint reads (containers/entrypoint.sh:34), so the launcher mounts the key exactly where the container looks for it.

RISK-VALUE: DERIVED — mount options = type=bind,src=$pem,dst=$pem_dst,ro @ containers/desk-run.sh:370 — containers/secrets.md line 39 requires the PEM as a read-only mounted file and section 4.1 (line 123) gives the same mount line ending in ro; the suite also pins ro as the last mount option.

RISK-VALUE: DERIVED — default PEM = $cfg/assay/$desk.pem @ containers/desk-run.sh:300 — one file per desk, so two desks on defaults never mount the same App key; docs/docker.md line 200 states the supported minimum of three distinct App identities (implementer, reviewer, verifier), which a shared default would break.

RISK-VALUE: DERIVED — tag length limit = 128 @ containers/desk-run.sh:214 — the docker image reference grammar allows a tag of one word character followed by at most 127 word, dot or dash characters, so 128 is the largest tag docker accepts.

**VERIFY: PASS** — 6/6 Verify rows pass at d725ec020dd6 with 0 failed and 0 could-not-run. Gate: model.

## Review
Gate: model (all four risk answers no — a host-side launcher implementing the
human-gated brief-02 contract; it holds no credential itself). Reviewer runs rows 4-6
(the refusal + no-value-echo paths) and confirms the composed docker command matches
`containers/secrets.md` (planned) exactly.
