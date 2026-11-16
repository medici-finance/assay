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
  could-not-check and never a pass, that an untrusted file refuses, and that no value reaches a
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
  `admittedRowEnv` strips shell-startup variables, four forge token variables and inherited
  `GIT_*`; its doc comment lists, under "Out of scope", what passes through unchanged. Every
  other inherited variable reaches the row.
- A row's stdout and stderr are captured combined; `hashOutput` hashes them, and an opt-in
  `captureOutput` sink receives them. Nothing removes a credential value from either.
- Brief 35 defines the dev-env file (`ASSAY_CELL_DEV_ENV_FILE` names its path in a window), its
  write rules, and the Env form `service:<name>:<NAME>[,<NAME>…]` by which a row declares names.

## Read first

- Brief 35 in full, and `containers/secrets.md` §4.5 once it lands.
- `admittedRowEnv` and its doc comment; the row loop in `verifyrun.go` around `plan.env` and
  `captureOutput`.

## Human decision

The ruling settles this brief's design; the open question is brief 35's (how a surface proves
the coordinator is the only writer), and this brief enforces whatever that answer is. Ruled:
the verifier admits only the names a Verify row declares, records names plus the file sha256
(never values), and redacts admitted values from captured output; it refuses a file any other
role can write.

Ruled 2026-11-16, by the driver in their own login:
https://github.com/medici-finance/assay/issues/6615#issuecomment-6921966886

This brief leaves `todo` only once `DR-desk-containers-35` (brief 35) is approved.

## Ground rules

- Tests use synthetic dev-env files in a temporary directory with plainly fake values. No live
  service.
- Never weaken an existing admission strip: the shell-startup, forge-token and `GIT_*` rules
  stand, and a declared name never re-admits one of them.
- An unadmitted run keeps today's behaviour; this brief changes admitted runs only.

## Task

1. **Load declared names only.** For a row whose Env cell declares names, read the file named
   by `ASSAY_CELL_DEV_ENV_FILE`, parse it per brief 35, and add exactly the declared names to
   that row's environment. Before adding them, remove every inherited variable with a declared
   name, so the value comes from the file or not at all. A row that declares no names gets
   nothing from the file.
2. **Fail closed.** The row is could-not-check, never pass, with a reason naming the variable
   but never a value, when: the path variable is unset or empty; the file is missing or
   unreadable; a declared name is absent from the file; the file fails a trust check (step 3).
3. **Trust checks**, before reading: a regular file, not a symlink; outside every git checkout
   and worktree (walk up from the file, as cellctl does for role secrets); not writable by the
   verifier's own identity; on a host cell, owned by an identity other than the verifier's, with
   no group or other write and no access-control list granting write; on a container cell, on a
   read-only mount. Any failure refuses the file for every row in the run.
4. **Attest.** The run's attestation and the row record carry the declared names and the file's
   sha256, never a value. The digest is over the file bytes as read, nonce line included.
5. **Redact.** Before a row's output is hashed, passed to `captureOutput`, or written anywhere,
   replace every occurrence of each admitted value of length 4 or more with
   `[redacted:<NAME>]`. Shorter values are not redacted and the record says so by name, since a
   short pattern would mangle ordinary output. The output hash is over the redacted bytes.
6. Changelog fragment.

## Interface contract

The attestation gains two fields, the declared names (sorted) and the dev-env file sha256.
Readers that do not know them ignore them; brief 38's verify-desk text cites them.

## Definition of Done

Every Verify row below passes. A row that declared names runs with exactly those names from
the file, and no record produced by the run contains a fixture value.

## Verify

All tests below are implementation deliverables, not results claimed by this authoring PR.
Named tests must execute and are written to fail first against the unchanged code; zero
matches or a skip fails independent verification.

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAdmitsDeclared$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAdmitsDeclared ' "$o")` | Named PASS; a row declaring two names sees exactly those from the file; an inherited value under a declared name is replaced; an undeclared file name is absent. |
| 2 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvMissingIsCNC$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvMissingIsCNC ' "$o")` | Named PASS; unset path, missing file and a declared name absent from the file each yield could-not-check, never pass. |
| 3 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvTrustRefusal$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvTrustRefusal ' "$o")` | Named PASS; a symlink, a file inside a worktree, a file the verifier can write and a group-writable file each refuse. |
| 4 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAttestsDigest$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAttestsDigest ' "$o")` | Named PASS; the attestation carries the sorted names and the file sha256 and no fixture value. |
| 5 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvRedactsOutput$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvRedactsOutput ' "$o")` | Named PASS; a row that prints an admitted value leaves `[redacted:<NAME>]` in the captured output and the output hash equals the hash of the redacted bytes. |
| 6 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestAdmitted' > "$o" 2>&1 && grep -F -- '--- PASS: TestAdmittedRowEnvShape ' "$o" && ! grep -F -- '--- FAIL' "$o")` | The existing admission tests (`TestAdmitted*`) still pass, none fails. |
| 7 | check | host | `statusgen --root . --lint --acceptance-forge` | exit 0; output contains "LINT: PASS". |
| 8 | check | tree | `statusgen --consumers --brief desk-containers/36 --base $(git merge-base origin/main HEAD)` | Pre-merge, on the implementation branch: exit 0. |

## Threat model / pre-mortem

- An implementer-side window writes the file so a row passes: rows 3 (refusal) and 4 (digest
  recorded, so a change is visible).
- A row reads a credential it did not declare from the inherited environment: row 1 covers the
  file; ambient inheritance of other variables stays as `admittedRowEnv` documents.
- A missing value lets a row skip its check and pass: row 2.
- A row prints a value into Evidence or into the hashed output: row 5.
- A value shorter than four characters is printed: not redacted; review-only. Mitigation is the
  dev-only class rule in brief 35.

## Evidence

_Not yet run._
