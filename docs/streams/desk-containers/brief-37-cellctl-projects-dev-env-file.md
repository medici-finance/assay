---
brief: assay:assay:desk-containers:37
title: cellctl projects the cell dev-env file path to each role and gives the coordinator its write verb
why: >-
  The contract only holds if the launcher enforces it: the file's path must reach the roles that
  read it, the coordinator must be the one role able to change it, and every change must leave a
  record. This brief makes cellctl check the file, export its path, refuse a cell that cannot
  prove the coordinator is the only writer, and give the coordinator a verb that writes the file
  and its change receipt.
wave: 2
depends: [desk-containers/35]
unblocks: [desk-containers/38]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  The launcher decides which role windows can reach a credential file and which can write it.
  The human confirms the per-role projection, the write-surface proof, and whether reviewer
  slots that build untrusted heads keep the write reach to the cell database that every role
  has today.
decision-trigger: creation
issues: [6615]
decision-issue: 6615
schema: brief-v2
version: 2
authored: 2026-11-16 by assay-worker-app (issue #6615)
sources:
  - "Issue #6615, the driver's ruling and amendment on it (cited in `## Human decision`)."
  - "docs/streams/desk-containers/brief-35-cell-dev-env-file-contract.md: the contract this brief enforces."
  - "docs/streams/decisions/DR-desk-containers-34.md: per-role read isolation and write-path rules for role secrets."
  - "freshness-checked 2026-11-16 @ 0d369ac33 (origin/main): tools/desk/cmd/cellctl/{envkeys.go,launch.go,celldb.go,check.go,rolesecret.go,kube_compat.go,codex_environment.go,desk.go}, docs/postgres.md."
exec-tier: strong
exec-tier-why: >-
  Per-role custody across host and container surfaces, and a reconciliation with an existing
  ruled custody record; a missed surface hands a credential to the wrong window.
domain: complicated
outcome: none
consumers:
  - "tools/desk/cmd/cellctl/envkeys.go: follow-up desk-containers/37 (CELL_DEV_ENV_FILE key)"
  - "tools/desk/cmd/cellctl/launch.go, celldb.go, codex_environment.go, kube_compat.go: follow-up desk-containers/37 (per-role projection; refusal where the surface cannot prove the writer)"
  - "tools/desk/cmd/cellctl/check.go: follow-up desk-containers/37 (file and surface rows)"
  - "tools/desk/cmd/cellctl/devenv.go (planned): follow-up desk-containers/37 (coordinator write verb and change receipts)"
  - "docs/cellctl.md, docs/postgres.md: follow-up desk-containers/37 (launcher behaviour; the cell database pointer's class)"
---

# Brief 37 — cellctl projects the cell dev-env file

## Context

files: `tools/desk/cmd/cellctl/{envkeys.go,launch.go,celldb.go,check.go,desk.go,codex_environment.go,kube_compat.go}`,
`tools/desk/cmd/cellctl/devenv.go` (planned),
`tools/desk/cmd/cellctl/devenv_test.go` (planned), `docs/cellctl.md`, `docs/postgres.md`,
`changelog/cellctl-dev-env-file.md` (planned).

facts (freshness-checked 2026-11-16 @ 0d369ac33; line numbers re-read on main 2026-11-16):

- `CELL_DB_DSN` and `CELL_DB_PASSFILE` are composed into every role window: `launch.go`
  lines 279-281 (the cell database pointer "composed for every role and absent when the cell
  has none", `c.cellDBEnv(env)`), `celldb.go`, and `docs/postgres.md` ("every role window of
  the cell gets `ASSAY_CELL_DB_DSN`"). `codex_environment.go` forwards both; `kube_compat.go`
  refuses them on a Kubernetes compatibility cell; `desk.go`'s dry run prints the DSN and the
  passfile path. `deskdispatch` strips only `ASSAY_ROLE_SECRET_*` from its children
  (`tools/desk/cmd/deskdispatch/exec.go`, `roleSecretChildEnv`), so review-kit children,
  including reviewer slots that build an untrusted pull-request head, inherit the database
  pointer today.
- `CELL_ROLE_SECRETS` (`envkeys.go`) binds a role secret to worker or verifier only, container
  surface only; `rolesecret.go` checks the file without opening it and appends receipts to
  `<cell>/state/role-secrets.receipts`.
- DR-desk-containers-34 rules that only worker and verifier hold a role secret, that a
  non-holding role must be unable to open the file, and that no non-holding identity may write
  what a holder reads as configuration.

## Read first

- Brief 35, `containers/secrets.md` §4.4 and §5, DR-desk-containers-34.
- `rolesecret.go` (the check and receipt pattern to reuse), `celldb.go`, and the role-environment
  composition in `launch.go`.

## Human decision

Ruled: one file per cell, from which a Verify row receives only the names it declares; names
and the file sha256 are recorded, never values.
Ruled 2026-11-16, by the driver in their own login:
https://github.com/medici-finance/assay/issues/6615#issuecomment-6921966886

Amended: the coordinator role owns and maintains the file and is the only role that may write
it; implementer, reviewer and verifier read it only; the verifier refuses a file any other role
can write; every change records the sha256 and the names added or removed, never values.
Amended 2026-11-16, by the driver in their own login:
https://github.com/medici-finance/assay/issues/6615#issuecomment-6922038844

Open, and not this brief's to answer: brief 35's question of how a surface proves the
coordinator is the only writer, and how the file stays clear of role-secret holders. This brief
depends on brief 35 (`depends:` above) and does not presuppose that answer. Task 2's surface
checks, record and mounts, Task 3's rule that a holding window gets no path, and Verify rows 1,
2, 4 and 8 are written for brief 35's option 1, the recommended one, and apply only if
`DR-desk-containers-35` chooses it; if it chooses another option, they are re-derived from that
record before this brief leaves `todo`. Under brief 35's default, hold, no cell binds a file and
this brief does not start.

Two further questions this brief puts to the driver:

1. **Which reviewer and worker children reach the file's path.** The amendment gives reviewer
   windows read access. DR-desk-containers-34 keeps role secrets from reviewers because reviewer
   slots read untrusted pull-request heads, and its dispatcher strips the role-secret path from
   any child that runs an unblessed head and from every review-kit child. This brief keeps the
   two channels apart by class: the dev-env file holds dev-only, throwaway values (brief 35
   Task 1), so a reviewer that leaks one leaks nothing usable outside a local dev environment;
   role secrets stay on their own channel and never enter the file.
   Recommended: accept, as ruled, for the reviewer window. Alternative: `deskdispatch` strips
   `ASSAY_CELL_DEV_ENV_FILE` from review-kit children and from any worker child that runs an
   unblessed head, as it does the role-secret path, so only the windows, not a slot building an
   untrusted head, can reach it. The question covers both: review-kit children and worker
   children running an unblessed head.
2. **Should reviewer slots that build untrusted heads keep write reach to the cell database?**
   Today every role window, and every child `deskdispatch` starts, including a reviewer slot
   building an untrusted pull-request head, receives the cell database pointer and its passfile
   (facts above), and with them write access to the cell's database. This brief changes none of
   that reach and widens nothing. It asks whether that reach should stand. DR-desk-containers-34
   would not give a role secret to such a slot; the database pointer is a separate channel it
   does not govern.
   Options: (a) keep today's reach and document the database pointer as cell infrastructure on
   its own channel, never placed in the dev-env file; (b) strip the database pointer from
   review-kit children and unblessed-head children, or hand them a read-only role, which breaks
   any review step that writes to the cell's database. Recommended: (a) for windows, with the
   driver choosing between (a) and (b) for reviewer slots building untrusted heads.

Default if no answer: hold. Each answer sets a credential's reach, so neither is taken by
silence, and the brief stays `todo` until both are answered.

This brief leaves `todo` only once `DR-desk-containers-35` (brief 35) is approved.

## Ground rules

- Fixtures use temporary directories and plainly fake values; no real cell, no live database.
- Never weaken a control: role-secret binding, its receipts and its refusals stay as they are.
  The dev-env path variable is never the role-secret variable, and `deskdispatch`'s strip of
  `ASSAY_ROLE_SECRET_*` is unchanged.
- No dry run, check row, receipt or log prints a value from the file.

## Task

1. **Key.** `envkeys.go` gains `CELL_DEV_ENV_FILE` (cell class, absolute path), read only from
   the cell's own `cell.env`.
2. **Check and record (surface rows under brief 35's option 1).** `cellctl check` adds rows,
   without printing a value: the file exists, is a regular file, and lies outside every git
   checkout and worktree; and it passes the surface proof brief 35's decision picks. Under
   option 1: container, a read-write mount for the coordinator container only, and a read-only
   mount only for worker, reviewer and verifier containers that mount no role secret; a worker
   or verifier container with a role secret mounted gets no dev-env mount, so the coordinator's
   writable mount is never shared with a holding container (DR-desk-containers-34, write-path
   clause); host, owned by the coordinator role's identity, with no other identity able to
   write it, no access-control list granting write, and every directory on its path owned by
   the system administrator, the launcher or the coordinator and writable only by its owner.
   A cell whose role windows share one OS user binds no file and its check says why. Any
   ownership, mode, access-control or surface state the check cannot determine fails the check.
   The launcher records, from the cell's identity map, the coordinator role's identity and, on
   a container cell, each role's mount mode for the file, and for every window the role-secret
   files it can reach (none, or their paths as that window sees them), in one record per
   launcher at a fixed name in the launcher anchor directory (brief 35 Interface contract),
   never under `<cell>/state/`, which the coordinator owns, and never at a path from
   `cell.env`, the cells root or a variable; brief 36 reads that record and refuses when it is
   absent. The launcher writes the record as its own identity, which owns the anchor and is
   none of the role identities (the coordinator's included), with no group or other write and
   no access-control list entry granting write. A launcher whose identity is a role identity,
   or that does not own the anchor, writes no record, binds no file, and its check says why. A
   record, the anchor or any directory above it that any identity but its owner can write, or
   that is a symlink, fails the check rather than being reused. On a container cell the
   launcher binds that one record file read-only into each verifier container at the fixed
   in-container path and into no other container; the cell directory stays unmounted.
   On a container cell, on every option, the dev-env file, its directory and the launcher
   anchor join the launch plan's protected paths, as cellctl adds the cell directory after the
   plan is built (`tools/desk/cmd/cellctl/container_native.go:62`), so a directory bind of any
   role, such as the incoming directory or a role's config directory, that contains one of them
   refuses before launch. The file and the record reach a container only through their own
   file mounts, and those two mounts are the one exemption: the plan's directory-identity check
   and `CheckFiles` (`tools/desk/internal/cellcontainer/config.go`) admit a bind whose source is
   exactly the dev-env file or the record, at its fixed target, for the containers this Task
   names and no other. Neither existing carve-out (`/run/secrets/` targets, directory binds) is
   widened to make room for them; that would weaken a control (ground rules).
3. **Projection.** `cellctl desk` and `up` export `ASSAY_CELL_DEV_ENV_FILE` (the path) into the
   coordinator, reviewer, and the worker and verifier windows that bind no role secret; a
   worker or verifier window that binds one gets no path, and intake gets none, per brief 35.
   A launching shell's stale value is removed, as `celldb.go` does for the database pointer.
   The Codex environment forwards it; a Kubernetes compatibility cell refuses the key by name.
   Whether review-kit and unblessed-head children keep it follows decision 1.
4. **Write verb.** `cellctl devenv set <cell> NAME` (value read from stdin, never argv) and
   `cellctl devenv unset <cell> NAME`. `set` refuses a name outside brief 35's allow-list and a
   value shorter than 8 characters, naming the name and never echoing the value. Each write is
   atomic, rewrites the nonce line with at least 128 bits from the operating system's
   cryptographic random source, and appends to `<cell>/state/dev-env.receipts` (directory
   `0700`, file `0600`, writable by the coordinator role's identity only): time, role, the new
   sha256, names added, names removed, never a value. The verb refuses when run from any role
   window but the coordinator's; that refusal is a client-side check on the window's declared
   role, not the control. The binding control is the surface proof (Task 2; rows 2 and 4): on a
   cell that passes it no other role can write the file whatever verb it runs.
5. **Cell database.** Document in `docs/postgres.md` and `docs/cellctl.md` how the database
   pointer relates to the dev-env file, as decision 2 picks.
6. Changelog fragment.

## Definition of Done

Every Verify row below passes; no output in any test contains a fixture value.

## Verify

All tests below are implementation deliverables, not results claimed by this authoring PR.
Named tests must execute; zero matches or a skip fails independent verification. Rows 1, 2, 4
and 8 apply under brief 35's option 1 and are re-derived with Tasks 2 and 3 if
`DR-desk-containers-35` chooses otherwise; row 4's protected-source case holds on every option.

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check +flow | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestDevEnvProjection$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvProjection ' "$o")` | Named PASS; coordinator and reviewer windows, and worker and verifier windows that bind no role secret, get the path variable; a worker or verifier window that binds a role secret does not, and intake does not; a stale inherited value is removed. |
| 2 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestDevEnvCheck$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvCheck ' "$o")` | Named PASS; a symlink, a file inside a worktree, a group-writable file, a file owned by a non-coordinator role, a directory on its path another identity can write, undeterminable ownership or ACL state, and a single-user cell each fail check; a passing check writes the identity and mount record, including each window's role-secret reach; the dry run contains no fixture value. |
| 3 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestDevEnvWriteVerb$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvWriteVerb ' "$o")` | Named PASS; set and unset from the coordinator rewrite the file, change the nonce line, and append a receipt with the sha256 and the names changed, no value; a name outside the allow-list and a 7-character value refuse; the same verb from a worker, reviewer or verifier window refuses. |
| 4 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestDevEnvMounts$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvMounts ' "$o")` | Named PASS; on a container cell the generated mounts are read-write for the coordinator container only and read-only for the reviewer container and for worker and verifier containers that mount no role secret; a worker or verifier container with a role secret mounted has no dev-env mount; intake has no mount; a role's directory bind (the incoming directory or a role config directory) that contains the dev-env file, its directory or the launcher anchor refuses before launch, because each is in the launch plan's protected paths; the dev-env file mount and the verifier's record mount are admitted by exact source only, and a bind of any other file from the same directory still refuses. |
| 5 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestRoleSecret' > "$o" 2>&1 && grep -F -- '--- PASS: TestRoleSecretScope ' "$o" && ! grep -F -- '--- FAIL' "$o")` | The role-secret tests still pass, none fails. |
| 6 | check | host | `statusgen --root . --lint --acceptance-forge` | exit 0; output contains "LINT: PASS". |
| 7 | check | tree | `statusgen --consumers --brief desk-containers/37 --base $(git merge-base origin/main HEAD)` | Pre-merge, on the implementation branch: exit 0. |
| 8 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestDevEnvRecordMode$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvRecordMode ' "$o")` | Named PASS; the launcher record is written at the fixed name in the launcher anchor directory, never under `<cell>/state/`, owned by the anchor's owner (`example-launcher` in the fixture), with no group or other write and no ACL write grant, and the launcher's identity is not the coordinator's nor any other role identity's; a launcher whose identity is the coordinator's or any other role identity's, or that does not own the anchor, writes no record and fails check; a pre-existing record, anchor or directory above it that is group-writable, writable by an identity other than its owner, owned by `example-operator`, or a symlink fails check and is not reused; on a container cell only verifier containers get the record, as a read-only file mount at the fixed path. |

## Threat model / pre-mortem

- A worker or reviewer window writes the file: rows 2 and 4 (the surface proof, the binding
  control); row 3 covers only the verb's client-side refusal.
- A cell claims a surface it does not provide: row 2 (single-user and undeterminable refusals).
- The value leaks through a dry run, check row or receipt: rows 2, 3.
- The file's path reaches intake, or the role-secret path reaches a reviewer: rows 1, 5.
- The coordinator's writable mount is shared with a container that holds a role secret, or the
  path reaches a window that binds one: rows 1 and 4; brief 36 row 16 refuses the row if it
  happens anyway.
- A holder's broader directory bind exposes the file without a dev-env mount: row 4 (the file
  and its directory are protected mount sources).
- A role window rewrites the launcher record to claim it binds no role secret, or to name
  itself the coordinator: row 8 (the record's owner, mode and anchor); brief 36 row 3 refuses
  a record whose owner is not the launcher identity anchored outside it, so a writer the record
  does not name as a role is refused too.
- A reviewer slot or an unblessed-head worker child reads a dev value: no row; decision 1 above.
- A reviewer slot building an untrusted head writes to the cell database: no row; today's reach,
  unchanged here; decision 2 above.

## Evidence

_Not yet run._
