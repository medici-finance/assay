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
  names are admitted and only in a window that can open no role secret, and that no literal
  value reaches a record, a hash input or Evidence. Redaction matches the literal value only: a
  row that prints a URL-encoded, base64 or JSON-escaped form of a value puts that form in
  Evidence unredacted (threat model).
decision-trigger: creation
issues: [6615]
decision-issue: 6615
schema: brief-v2
version: 4
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
coordinator is the only writer, and how the file stays clear of role-secret holders. This
brief depends on brief 35 (`depends:` above) and does not presuppose that answer. Tasks 2
and 4 and Verify rows 3 and 16 are written for brief 35's option 1, the recommended one, and
apply only if `DR-desk-containers-35` chooses it; if it chooses another option, they are
re-derived from that record before this brief leaves `todo`.
Brief 35's option 2 conflicts with DR-desk-containers-34 and cannot be implemented from this
brief without that record amending it. Brief 35's
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
2. **Only in a window that cannot open a role secret (under brief 35's option 1).**
   DR-desk-containers-34 requires a non-holder to be unable to open a role-secret file, not only
   unaware of its path. Removing variables gives only the second, so a row that declares
   dev-env names is admitted only when both hold:
   - the launcher's record for this window (brief 37 Task 2) states that it binds no role
     secret;
   - for every role-secret file path the whole record lists, for every window of every cell
     the launcher starts, an open for read from the row's environment (its identity, its
     mounts) fails.
   The probe never depends on which entry the verifier reads as its own window's, so an entry
   key that names the wrong window or cell cannot narrow it (brief 35 Interface contract,
   record location). Each probe open is non-blocking and reads no byte; it is closed at once.
   An open that succeeds, a record that lists a role secret for this window, or a record that
   is missing or unreadable makes the row could-not-check, naming the service and never a
   path's contents. As defence in depth, the row also runs with every
   `ASSAY_ROLE_SECRET_*` variable removed; that removal is not the separation and no check
   relies on it. A row that needs both a role secret and a dev-env value stays could-not-check;
   `DR-desk-containers-35` may revisit that, this brief does not.
3. **Fail closed.** The row is could-not-check, never pass, with a reason naming the variable
   but never a value and never quoting a file line, when: the path variable is unset or empty;
   the file is missing or unreadable; a line is malformed or unparseable; a name appears on
   more than one line of the file (malformed, so no parser picks one); a declared name is
   absent from the file; a declared name's value is shorter than 8 characters; the window can
   reach a role secret (step 2); the file fails a trust check (step 4).
4. **Trust checks (under brief 35's option 1; see `## Human decision`).** Open the file without
   following a symlink, then run every check on the opened descriptor (`fstat`, and the
   access-control list read through the descriptor), never on the path, and read the contents
   from that same descriptor, so the file cannot change between check and read. The file is
   refused for every row in the run unless all of these hold. Throughout this brief and brief
   37, *writable only by its owner* means no identity but the owner holds, by a group or other
   mode bit or by any access-control entry, a right that can write, replace, rename or remove
   an entry: write, append, delete, delete-child, add-file, add-subdirectory, write-security
   (permissions) or change-owner. A walk of *every directory on its path, opened without
   following a symlink* is a descriptor chain: open the filesystem root, then each component
   no-follow relative to its already-checked parent's descriptor, and open the leaf from the
   last checked descriptor.
   - it is a regular file, not a symlink;
   - it lies outside every git checkout and worktree (walk up from the file, as cellctl does for
     role secrets);
   - on a host cell: its owner is the coordinator role's identity, read from the launcher's
     record of the cell's identity map (brief 37 Task 2), so a file owned by the worker,
     reviewer, verifier, intake or any other identity refuses; it is writable only by its owner
     as defined above, so a group or other write bit, or any right in that list that an
     access-control entry gives another identity, refuses; and every
     directory on its path, from the filesystem root down, opened without following a symlink
     and checked the same way, is owned by the system administrator, the launcher identity
     (brief 35 Interface contract) or the coordinator, and is writable only by its owner by mode
     and access-control list, so no other identity can rename or replace the file or a
     directory above it. There is no sticky-bit exception: a sticky-bit directory that another
     identity can write refuses like any other, as brief 37 Task 2's launcher check does;
   - on a container cell: the verifier's own mount of the file is read-only, and the launcher's
     mount record (brief 37 Task 2) states that only the coordinator container mounts it
     read-write. The verifier's own mount says nothing about the other roles' mounts, so where
     only that mount is visible and the cell gives no such record, the file refuses.
   - the launcher's record itself (brief 37 Task 2) is trusted only through the anchor in brief
     35's Interface contract: an allow-list naming the one identity that may own it, never a
     list of role identities to deny. The verifier opens it only at the fixed name in the
     launcher anchor directory, never at a path from the record, `cell.env`, the cells root or
     a variable a window can set. The launcher identity is pinned when the verifier is built or
     installed, together with the anchor path (`DR-desk-containers-35`), never derived at run
     time from who owns the anchor; the anchor's owner must equal that pinned identity (brief
     35 Interface contract, launcher identity). The verifier reads the pin and the anchor path
     only from the one source that contract's pin storage bullet names, the source brief 37's
     launcher reads; a pin it cannot read from there refuses.
     On a host cell it refuses unless all of these hold: the record's owner and the anchor
     directory's owner are both the pinned launcher identity; that identity is
     neither the coordinator identity the record names nor the verifier's own; the record is
     writable only by its owner as defined above; it is not a symlink; and every component of the
     anchor path, from the
     filesystem root down, opened without following a symlink, is owned by the system
     administrator or the launcher identity and writable only by its owner. On a container
     cell it refuses unless the record is the verifier container's own read-only file mount at
     the fixed in-container path, checked as the dev-env mount is. Every check runs on opened
     descriptors, as above.
   - Anything the check cannot determine refuses as could-not-check, never falls through to
     admit: a failed open, `fstat` or access-control query; a filesystem that cannot report
     access-control lists; a filesystem that is not on an allow-list of known local filesystem
     kinds whose reported owner and mode reflect who can write them, so an unknown kind
     refuses, and an allow-listed kind mounted to ignore ownership refuses too; a missing or
     unreadable identity or mount record; a surface kind the cell does not declare or the
     verifier does not know. The allow-list is fixed in the verifier's code and grows only by
     a reviewed change; brief 37's launcher builds from the same definition (brief 35
     Interface contract, pin storage). Network and user-space filesystem kinds are not on it:
     their reported owner and mode need not reflect who can write them. These rules apply to
     the dev-env file, the launcher record, the anchor and every directory above either.
5. **Attest.** The run's attestation and the row record carry the declared names and the file's
   sha256, never a value. The digest is over the file bytes as read, nonce line included.
6. **Redact.** Before a row's output is hashed, passed to `captureOutput`, or written anywhere,
   replace every occurrence of each admitted value with `[redacted:<NAME>]`, longest value
   first, so a value that is a substring of another never leaves part of the longer one in the
   output. Every admitted value is redacted: step 3 refuses one shorter than 8 characters, so no
   exception is needed. The output hash is over the redacted bytes. Only the literal value is
   matched; encoded forms are not redacted (threat model).
7. Changelog fragment.

## Interface contract

The attestation gains two fields, the declared names (sorted) and the dev-env file sha256.
Readers that do not know them ignore them; brief 38's verify-desk text cites them. Under
option 1, admission reads the coordinator identity, the container mount record and each
window's role-secret reach only from what brief 37's launcher records, per brief 35's Interface
contract, and trusts that record only through the launcher anchor that contract defines.

## Definition of Done

Every Verify row below passes. A row that declared names runs with exactly those names from
the file, only in a window that cannot open any role-secret file the whole launcher record
lists, for every window of every cell (row 16), and no record produced by the run contains a
fixture value in its literal form. Encoded forms of a value are outside this guarantee (threat
model).

## Verify

All tests below are implementation deliverables, not results claimed by this authoring PR.
Named tests must execute and are written to fail first against the unchanged code; zero
matches or a skip fails independent verification. Rows 3 and 16 apply under brief 35's option
1 and are re-derived with Tasks 2 and 4 if `DR-desk-containers-35` chooses otherwise.

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAdmitsDeclared$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAdmitsDeclared ' "$o")` | Named PASS; a row declaring two `DEVENV_` names sees exactly those from the file; an inherited value under a declared name is replaced; an undeclared file name is absent. |
| 2 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvMissingIsCNC$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvMissingIsCNC ' "$o")` | Named PASS; unset path, missing file, a malformed line, a name on two lines of the file, a declared name absent from the file and a declared value of 7 characters each yield could-not-check, never pass, and no reason contains a fixture value or a file line. |
| 3 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvTrustRefusal$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvTrustRefusal ' "$o")` | Named PASS; each of these refuses: a symlink; a file inside a worktree; a file the verifier can write; a group-writable file; a file owned by a non-coordinator role's identity with mode 0644; a parent or higher directory writable by an identity other than its owner, or owned by `example-operator` (neither administrator, launcher nor coordinator); a sticky-bit parent directory writable by other identities, even where the next component is owned by the coordinator; ownership or access-control state that cannot be determined (a failing stat or ACL query stub); a host cell with no identity record, and a host cell whose identity record is unreadable; planted bypass records, each refused: one owned by `example-operator`, an identity the record names as no role, which a role-identity deny-list admits; one owned by the coordinator identity; one at the right path whose anchor directory's parent another identity can write; one at the right path whose anchor directory itself another identity can write, by a group or other write bit or an ACL entry; one with an ACL entry granting write to `example-operator`; one that is a symlink; and an anchor owned by the coordinator or by the verifier's own identity; a directory above the anchor owned by the coordinator identity and writable only by its owner; an anchor and record both owned by `example-operator`, which is not the pinned launcher identity; a parent directory whose only grant to another identity is an ACL right to delete, or to add an entry; a directory above the dev-env file, and a directory above the anchor, whose only grant to another identity is an ACL right to change permissions, and each whose only such grant is an ACL right to change owner; a dev-env file, and a record, whose only grant to another identity is an ACL right to append, to change permissions or to change owner; a file on a filesystem whose kind a stub reports as outside the allow-list of known local kinds, and one on an allow-listed kind mounted to ignore ownership, and a record and an anchor on each of those two; a pin source the verifier cannot read; a record offered at another path through a variable, which is never opened; a container cell whose record is not a read-only file mount at the fixed in-container path; an unknown surface kind; a container cell with no mount record. A record owned by the pinned launcher identity `example-launcher`, at the fixed name, with a clean anchor path, is accepted. A file swapped after the check is not read: checks and read use one descriptor. |
| 4 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAttestsDigest$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAttestsDigest ' "$o")` | Named PASS; the attestation carries the sorted names and the file sha256 and no fixture value. |
| 5 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvRedactsOutput$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvRedactsOutput ' "$o")` | Named PASS; a row that prints an admitted value leaves `[redacted:<NAME>]` in the captured output and the output hash equals the hash of the redacted bytes; with two admitted values where one is a substring of the other, no part of either is left in the output. |
| 6 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestAdmitted' > "$o" 2>&1 && grep -F -- '--- PASS: TestAdmittedRowEnvShape ' "$o" && ! grep -F -- '--- FAIL' "$o")` | The existing admission tests (`TestAdmitted*`) still pass, none fails. |
| 7 | check | host | `statusgen --root . --lint --acceptance-forge` | exit 0; output contains "LINT: PASS". |
| 8 | check | tree | `statusgen --consumers --brief desk-containers/36 --base $(git merge-base origin/main HEAD)` | Pre-merge, on the implementation branch: exit 0. |
| 9 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvStripsRoleSecret$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvStripsRoleSecret ' "$o")` | Named PASS; with an `ASSAY_ROLE_SECRET_*` variable in the inherited environment, a row that declares names runs without it, and a row that declares none keeps today's environment. Defence in depth only; row 16 is the separation. |
| 10 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAllowList$/^path$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAllowList/path ' "$o")` | Named PASS; a row declaring `PATH`, `HOME` or `XDG_CONFIG_HOME` (lint bypassed) is could-not-check naming the path class, and the file's value never reaches the row. |
| 11 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAllowList$/^loader$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAllowList/loader ' "$o")` | Named PASS; `LD_PRELOAD`, `LD_LIBRARY_PATH` or `DYLD_INSERT_LIBRARIES` is could-not-check naming the loader class. |
| 12 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAllowList$/^interpreter$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAllowList/interpreter ' "$o")` | Named PASS; `NODE_OPTIONS` or `PYTHONPATH` is could-not-check naming the interpreter class. |
| 13 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAllowList$/^toolconfig$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAllowList/toolconfig ' "$o")` | Named PASS; `GH_CONFIG_DIR`, `GH_TOKEN`, a `GIT_*` name or `BASH_ENV` is could-not-check naming the tool-configuration class. |
| 14 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAllowList$/^socket$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAllowList/socket ' "$o")` | Named PASS; `SSH_AUTH_SOCK` is could-not-check naming the agent-socket class. |
| 15 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvAllowList$/^assay$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvAllowList/assay ' "$o")` | Named PASS; an `ASSAY_ROLE_SECRET_*` name, `ASSAY_CELL_DEV_ENV_FILE` or any other `ASSAY_*` name is could-not-check naming the Assay class; a name outside every class and outside `DEVENV_` is still could-not-check (deny by default). |
| 16 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestDevEnvNoSecretReach$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvNoSecretReach ' "$o")` | Named PASS; a row that declares names is could-not-check when every `ASSAY_ROLE_SECRET_*` variable is already removed but a fixture role-secret file the launcher record lists is still openable from the row's environment (strip-only does not admit); when the record lists a role secret for this window; when the record is missing or unreadable; and when a role-secret path the record lists only under another window's or another cell's entry is still openable from the row's environment. A listed path that is a named pipe does not stall the probe, and no probe reads a byte. It is admitted only when the record lists none for the window and an open of every role-secret path the whole record lists fails. No reason contains a fixture value. |

## Threat model / pre-mortem

- An implementer-side window writes the file so a row passes: rows 3 (refusal, including a file
  a non-coordinator role owns and a directory on its path another identity can write) and 4
  (digest recorded, so a change is visible).
- The coordinator, a non-holding role, steers what a holding window executes or reads as
  configuration through a declared name: rows 10 to 15 (allow-list at admission).
- A row declares a `DEVENV_` host, reads a role secret by its literal path and sends it there,
  so the coordinator, a non-holder, picks where a holder's secret goes: row 16. The row is
  admitted only in a window that cannot open any role-secret file; removing the variables
  (row 9) would leave the file openable and is not relied on.
- A forged launcher record, owned by an identity the record does not name as a role, marks a
  holding window as binding none or empties the role-secret probe: row 3. The owner must be
  the launcher identity pinned at install with the anchor path, outside the record (brief 35
  Interface contract), so an owner the record leaves out is refused rather than missed.
- A window steers which record entry the verifier reads as its own, to narrow the role-secret
  probe to paths it cannot open: Task 2 probes every path the whole record lists; row 16.
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
