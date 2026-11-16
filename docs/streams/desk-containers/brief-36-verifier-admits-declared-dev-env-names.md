---
brief: assay:assay:desk-containers:36
title: Verifier admits only declared dev-env names, attests names and digest, redacts values
why: >-
  A Verify row today inherits whatever the window's environment holds, so an implementer-side
  .env file can change what a row sees and the attestation says nothing about it, and a
  credential the row prints can land in Evidence that is often public. This brief makes the
  verifier load only the names a row declares from the cell's dev-env file, refuse a file it
  cannot trust, record names and the file digest, and strip the admitted values from everything
  it captures.
wave: 2
depends: [desk-containers/35]
unblocks: [desk-containers/38]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  This changes the verification-integrity control: what environment an admitted Verify row runs
  in and what its attestation claims. The human confirms that an undeclared or missing name is a
  could-not-check and never a pass, that an untrusted file refuses, that only allow-listed
  names are admitted and never into a row that holds a role secret, and that no value reaches a
  record, a hash input or Evidence.
decision-trigger: creation
issues: [6615]
decision-issue: 6615
schema: brief-v2
version: 1
authored: 2026-11-16 by assay-worker-app (issue #6615)
sources:
  - "Issue #6615 and the driver's ruling on it (cited in `## Human decision`)."
  - "docs/streams/desk-containers/brief-35-cell-dev-env-file-contract.md: the file and grammar contract this brief enforces."
  - "freshness-checked 2026-11-16 @ 0d369ac33 (origin/main): statusgen/verifieradmission.go (admittedRowEnv), statusgen/verifyrun.go (plan.env, captureOutput, hashOutput), statusgen/forgecredential.go."
exec-tier: strong
exec-tier-why: >-
  Verification integrity and credential handling in one code path; a green test can hide a value
  reaching the output hash or the Evidence capture.
domain: complicated
outcome: none
consumers:
  - "statusgen/verifieradmission.go: follow-up desk-containers/36 (admittedRowEnv takes the declared dev-env names)"
  - "statusgen/verifyrun.go: follow-up desk-containers/36 (load, attest and redact around each row's run)"
  - "statusgen/devenv.go (planned): follow-up desk-containers/36 (file parse, trust checks, digest)"
---

# Brief 36 — Verifier admits only declared dev-env names

## Context

files: `statusgen/verifieradmission.go`, `statusgen/verifyrun.go`,
`statusgen/devenv.go` (planned), `statusgen/devenv_test.go` (planned),
`changelog/verifier-dev-env-admission.md` (planned).

facts (freshness-checked 2026-11-16 @ 0d369ac33):

- `verifyrun.go` sets `plan.env = admittedRowEnv(os.Environ())` for an admitted run.
  `admittedRowEnv` (`statusgen/verifieradmission.go:146` at that commit) strips shell-startup
  variables, four forge token variables and inherited `GIT_*`; its doc comment lists, under
  "Out of scope", what passes through unchanged. Every other inherited variable reaches the
  row, including the `ASSAY_ROLE_SECRET_*` path variables a holding verifier window carries.
- A row's stdout and stderr are captured combined; `hashOutput` hashes them, and an opt-in
  `captureOutput` sink receives them. Nothing removes a credential value from either.
- Brief 35 defines the dev-env file (`ASSAY_CELL_DEV_ENV_FILE` names its path in a window), its
  write rules, the 8-character value floor, the `DEVENV_` allow-list, and the Env form
  `service:<name>:<NAME>[,<NAME>…]` by which a row declares names.

## Read first

- Brief 35 in full, and `containers/secrets.md` §4.5 once it lands.
- `admittedRowEnv` and its doc comment; the row loop in `verifyrun.go` around `plan.env` and
  `captureOutput`.

## Human decision

Ruled: the verifier admits only the names a Verify row declares, records names plus the file
sha256 (never values), and redacts admitted values from captured output.
Ruled 2026-11-16, by the driver in their own login:
https://github.com/medici-finance/assay/issues/6615#issuecomment-6921966886

Amended: the coordinator role alone writes the file, and the verifier refuses a file any other
role can write.
Amended 2026-11-16, by the driver in their own login:
https://github.com/medici-finance/assay/issues/6615#issuecomment-6922038844

Open, and not this brief's to answer: brief 35's question of how a surface proves the
coordinator is the only writer. This brief depends on brief 35 (`depends:` above) and does not
presuppose that answer. Task 3 and Verify row 3 are written for brief 35's option 1, the
recommended one, and apply only if `DR-desk-containers-35` chooses it; if it chooses another
option, both are re-derived from that record before this brief leaves `todo`. Brief 35's
default is hold, and under hold no cell binds a file, so every row that declares names stays
could-not-check. Every other Task item holds whichever option is chosen.

This brief leaves `todo` only once `DR-desk-containers-35` (brief 35) is approved.

## Ground rules

- Tests use synthetic dev-env files in a temporary directory with plainly fake values. No live
  service.
- Never weaken an existing admission strip: the shell-startup, forge-token and `GIT_*` rules
  stand, and a declared name never re-admits one of them (brief 35's allow-list excludes them).
- An unadmitted run keeps today's behaviour; this brief changes admitted runs only.

## Task

1. **Load declared, allow-listed names only.** For a row whose Env cell declares names, check
   each against brief 35's allow-list (`DEVENV_[A-Z0-9_]+`, deny by default): a name outside it
   makes the row could-not-check with a reason naming the name and its class (path, loader,
   interpreter, tool configuration, agent socket, Assay, or none), whatever the lint said. Then
   read the file named by `ASSAY_CELL_DEV_ENV_FILE`, parse it per brief 35, and add exactly the
   declared names to that row's environment. Before adding them, remove every inherited
   variable with a declared name, so the value comes from the file or not at all. A row that
   declares no names gets nothing from the file.
2. **No row holds both channels.** A row that declares dev-env names runs with every
   `ASSAY_ROLE_SECRET_*` variable removed from its environment, so `admittedRowEnv` never admits
   a dev-env name into a row that holds a role secret. The allow-list also forbids the name
   overlap: no `DEVENV_` name is an `ASSAY_*` name. A row that needs both a role secret and a
   dev-env value stays could-not-check; `DR-desk-containers-35` may revisit that, this brief does
   not.
3. **Fail closed.** The row is could-not-check, never pass, with a reason naming the variable
   but never a value and never quoting a file line, when: the path variable is unset or empty;
   the file is missing or unreadable; a line is malformed or unparseable; a declared name is
   absent from the file; a declared name's value is shorter than 8 characters; the file fails a
   trust check (step 4).
4. **Trust checks (under brief 35's option 1; see `## Human decision`).** Open the file without
   following a symlink, then run every check on the opened descriptor (`fstat`, and the
   access-control list read through the descriptor), never on the path, and read the contents
   from that same descriptor, so the file cannot change between check and read. The file is
   refused for every row in the run unless all of these hold:
   - it is a regular file, not a symlink;
   - it lies outside every git checkout and worktree (walk up from the file, as cellctl does for
     role secrets);
   - on a host cell: its owner is the coordinator role's identity, read from the launcher's
     record of the cell's identity map (brief 37 Task 2), so a file owned by the worker,
     reviewer, verifier, intake or any other identity refuses; it has no group or other write;
     no access-control list entry grants write to any identity but the coordinator's; and its
     parent directory, opened and checked the same way, is not writable by any other role's
     identity by mode, ownership or access-control list, so no other role can rename or replace
     the file;
   - on a container cell: the verifier's own mount of the file is read-only, and the launcher's
     mount record (brief 37 Task 2) states that only the coordinator container mounts it
     read-write. The verifier's own mount says nothing about the other roles' mounts, so where
     only that mount is visible and the cell gives no such record, the file refuses.
   - Anything the check cannot determine refuses as could-not-check, never falls through to
     admit: a failed open, `fstat` or access-control query; a filesystem that cannot report
     access-control lists; a missing or unreadable identity or mount record; a surface kind the
     cell does not declare or the verifier does not know.
5. **Attest.** The run's attestation and the row record carry the declared names and the file's
   sha256, never a value. The digest is over the file bytes as read, nonce line included.
6. **Redact.** Before a row's output is hashed, passed to `captureOutput`, or written anywhere,
   replace every occurrence of each admitted value with `[redacted:<NAME>]`. Every admitted value
   is redacted: step 3 refuses one shorter than 8 characters, so no exception is needed. The
   output hash is over the redacted bytes. Only the literal value is matched; see the threat
   model for encoded forms.
7. Changelog fragment.

## Interface contract

The attestation gains two fields, the declared names (sorted) and the dev-env file sha256.
Readers that do not know them ignore them; brief 38's verify-desk text cites them. Under
option 1, admission reads the coordinator identity and the container mount record only from
what brief 37's launcher records, per brief 35's Interface contract.

## Definition of Done

Every Verify row below passes. A row that declared names runs with exactly those names from
the file and no role-secret variable, and no record produced by the run contains a fixture
value.

## Verify

All tests below are implementation deliverables, not results claimed by this authoring PR.
Named tests must execute and are written to fail first against the unchanged code; zero
matches or a skip fails independent verification. Row 3 applies under brief 35's option 1 and
is re-derived with Task 4 if `DR-desk-containers-35` chooses otherwise.

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAdmitsDeclared$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAdmitsDeclared ' "$o")` | Named PASS; a row declaring two `DEVENV_` names sees exactly those from the file; an inherited value under a declared name is replaced; an undeclared file name is absent. |
| 2 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvMissingIsCNC$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvMissingIsCNC ' "$o")` | Named PASS; unset path, missing file, a malformed line, a declared name absent from the file and a declared value of 7 characters each yield could-not-check, never pass, and no reason contains a fixture value or a file line. |
| 3 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvTrustRefusal$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvTrustRefusal ' "$o")` | Named PASS; each of these refuses: a symlink; a file inside a worktree; a file the verifier can write; a group-writable file; a file owned by a non-coordinator role's identity with mode 0644; a parent directory another role's identity can write; ownership or access-control state that cannot be determined (a failing stat or ACL query stub); an unknown surface kind; a container cell with no mount record. A file swapped after the check is not read: checks and read use one descriptor. |
| 4 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAttestsDigest$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAttestsDigest ' "$o")` | Named PASS; the attestation carries the sorted names and the file sha256 and no fixture value. |
| 5 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvRedactsOutput$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvRedactsOutput ' "$o")` | Named PASS; a row that prints an admitted value leaves `[redacted:<NAME>]` in the captured output and the output hash equals the hash of the redacted bytes. |
| 6 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestAdmitted' > "$o" 2>&1 && grep -F -- '--- PASS: TestAdmittedRowEnvShape ' "$o" && ! grep -F -- '--- FAIL' "$o")` | The existing admission tests (`TestAdmitted*`) still pass, none fails. |
| 7 | check | host | `statusgen --root . --lint --acceptance-forge` | exit 0; output contains "LINT: PASS". |
| 8 | check | tree | `statusgen --consumers --brief desk-containers/36 --base $(git merge-base origin/main HEAD)` | Pre-merge, on the implementation branch: exit 0. |
| 9 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvStripsRoleSecret$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvStripsRoleSecret ' "$o")` | Named PASS; with an `ASSAY_ROLE_SECRET_*` variable in the inherited environment, a row that declares names runs without it, and a row that declares none keeps today's environment. |
| 10 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAllowList$/^path$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAllowList/path ' "$o")` | Named PASS; a row declaring `PATH`, `HOME` or `XDG_CONFIG_HOME` (lint bypassed) is could-not-check naming the path class, and the file's value never reaches the row. |
| 11 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAllowList$/^loader$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAllowList/loader ' "$o")` | Named PASS; `LD_PRELOAD`, `LD_LIBRARY_PATH` or `DYLD_INSERT_LIBRARIES` is could-not-check naming the loader class. |
| 12 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAllowList$/^interpreter$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAllowList/interpreter ' "$o")` | Named PASS; `NODE_OPTIONS` or `PYTHONPATH` is could-not-check naming the interpreter class. |
| 13 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAllowList$/^toolconfig$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAllowList/toolconfig ' "$o")` | Named PASS; `GH_CONFIG_DIR`, `GH_TOKEN`, a `GIT_*` name or `BASH_ENV` is could-not-check naming the tool-configuration class. |
| 14 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAllowList$/^socket$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAllowList/socket ' "$o")` | Named PASS; `SSH_AUTH_SOCK` is could-not-check naming the agent-socket class. |
| 15 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAllowList$/^assay$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAllowList/assay ' "$o")` | Named PASS; an `ASSAY_ROLE_SECRET_*` name, `ASSAY_CELL_DEV_ENV_FILE` or any other `ASSAY_*` name is could-not-check naming the Assay class; a name outside every class and outside `DEVENV_` is still could-not-check (deny by default). |

## Threat model / pre-mortem

- An implementer-side window writes the file so a row passes: rows 3 (refusal, including a file
  a non-coordinator role owns and a directory another role can write) and 4 (digest recorded, so
  a change is visible).
- The coordinator, a non-holding role, steers what a holding window executes or reads as
  configuration through a declared name: rows 10 to 15 (allow-list at admission), and row 9 (no
  row holds a role secret and dev-env names together).
- A file swapped between the check and the read: Task 4's single descriptor, row 3.
- A trust state the verifier cannot read lets the file through: row 3's undeterminable and
  unknown-surface cases refuse.
- A row reads a credential it did not declare from the inherited environment: row 1 covers the
  file; ambient inheritance of other variables stays as `admittedRowEnv` documents.
- A missing value lets a row skip its check and pass: row 2.
- A row prints a value into Evidence or into the hashed output: row 5; a value too short to
  redact cannot be admitted, row 2.
- A row prints an encoded form of a value (URL-encoded, base64, JSON-escaped): not redacted,
  since Task 6 matches the literal value only; review-only. Mitigation is brief 35's dev-only
  class rule.

## Evidence

_Not yet run._
