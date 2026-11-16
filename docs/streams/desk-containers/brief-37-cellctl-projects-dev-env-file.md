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
  The human confirms the per-role projection, the write-surface proof, and how the cell database
  pointer, which reaches all five roles today, sits beside DR-desk-containers-34's worker and
  verifier limit.
decision-trigger: creation
issues: [6615]
decision-issue: 6615
schema: brief-v2
version: 1
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

facts (freshness-checked 2026-11-16 @ 0d369ac33):

- `CELL_DB_DSN` and `CELL_DB_PASSFILE` are composed into every role window: `launch.go` (the
  cell database pointer "composed for every role and absent when the cell has none"),
  `celldb.go`, and `docs/postgres.md` ("every role window of the cell gets
  `ASSAY_CELL_DB_DSN`"). `codex_environment.go` forwards both; `kube_compat.go` refuses them on a
  Kubernetes compatibility cell; `desk.go`'s dry run prints the DSN and the passfile path.
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

Ruled: one file per cell; the coordinator role owns and maintains it and is the only role that
may write it; implementer, reviewer and verifier read it only; the verifier refuses a file any
other role can write; every change records the sha256 and the names added or removed, never
values.

Ruled 2026-11-16, by the driver in their own login:
https://github.com/medici-finance/assay/issues/6615#issuecomment-6921966886

Amended 2026-11-16 (the coordinator role owns, maintains and alone writes the file), by the driver in their own login:
https://github.com/medici-finance/assay/issues/6615#issuecomment-6922038844

Two consequences this brief puts to the driver:

1. **The reviewer reads the file.** The amendment gives reviewer windows read access.
   DR-desk-containers-34 keeps role secrets from reviewers because reviewer slots read untrusted
   pull-request heads. This brief keeps the two apart by class: the dev-env file holds dev-only,
   throwaway values (brief 35 Task 1), so a reviewer that leaks one leaks nothing usable outside
   a local dev environment; role secrets stay on their own channel and never enter the file.
   Recommended: accept, as ruled. Alternative: the reviewer reads the file but `deskdispatch`
   strips the path variable from review-kit children, so only the window, not a slot building a
   head, can reach it.
2. **The cell database pointer.** It reaches all five roles today, and DR-desk-containers-34
   limits role secrets to worker and verifier. Recommended: the cell database is cell
   infrastructure, not a Verify-row credential; it keeps its own channel and its all-role reach,
   and is documented as a distinct class that never goes into the dev-env file. Alternative:
   narrow it to worker and verifier, which breaks any coordinator, reviewer or intake use of the
   cell's own database.

Default if no answer: hold. Each answer widens or narrows a credential's reach, so neither is
taken by silence, and the brief stays `todo` until both are answered.

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
2. **Check.** `cellctl check` adds rows, without printing a value: the file exists, is a
   regular file, lies outside every git checkout and worktree, and passes the surface proof
   brief 35's decision picks (container: read-write mount for the coordinator container only,
   read-only for worker, reviewer and verifier; host: owned by the coordinator role's identity,
   no other role identity can write it or its directory, no access-control list grants write).
   A cell whose role windows share one OS user binds no file and its check says why.
3. **Projection.** `cellctl desk` and `up` export `ASSAY_CELL_DEV_ENV_FILE` (the path) into the
   coordinator, worker, reviewer and verifier windows; intake gets none, per brief 35. A
   launching shell's stale value is removed, as `celldb.go` does for the database pointer. The
   Codex environment forwards it; a Kubernetes compatibility cell refuses the key by name.
4. **Write verb.** `cellctl devenv set <cell> NAME` (value read from stdin, never argv) and
   `cellctl devenv unset <cell> NAME`, run from the coordinator role only (refused from any other
   role window). Each write is atomic, rewrites the nonce line, and appends to
   `<cell>/state/dev-env.receipts` (directory `0700`, file `0600`): time, role, the new sha256,
   names added, names removed, never a value.
5. **Cell database.** Document in `docs/postgres.md` and `docs/cellctl.md` how the database
   pointer relates to the dev-env file, as the decision picks.
6. Changelog fragment.

## Definition of Done

Every Verify row below passes; no output in any test contains a fixture value.

## Verify

All tests below are implementation deliverables, not results claimed by this authoring PR.
Named tests must execute; zero matches or a skip fails independent verification.

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check +flow | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestDevEnvProjection$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvProjection ' "$o")` | Named PASS; coordinator, worker, reviewer and verifier windows get the path variable, intake does not; a stale inherited value is removed. |
| 2 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestDevEnvCheck$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvCheck ' "$o")` | Named PASS; a symlink, a file inside a worktree, a group-writable file and a single-user cell each fail check; the dry run contains no fixture value. |
| 3 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestDevEnvWriteVerb$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvWriteVerb ' "$o")` | Named PASS; set and unset from the coordinator rewrite the file and append a receipt with the sha256 and the names changed, no value; the same verb from a worker, reviewer or verifier window refuses. |
| 4 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestDevEnvMounts$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDevEnvMounts ' "$o")` | Named PASS; on a container cell the generated mounts are read-write for the coordinator container only and read-only for worker, reviewer and verifier; intake has no mount. |
| 5 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestRoleSecret' > "$o" 2>&1 && grep -F -- '--- PASS: TestRoleSecretScope ' "$o" && ! grep -F -- '--- FAIL' "$o")` | The role-secret tests still pass, none fails. |
| 6 | check | host | `statusgen --root . --lint --acceptance-forge` | exit 0; output contains "LINT: PASS". |
| 7 | check | tree | `statusgen --consumers --brief desk-containers/37 --base $(git merge-base origin/main HEAD)` | Pre-merge, on the implementation branch: exit 0. |

## Threat model / pre-mortem

- A worker or reviewer window writes the file: rows 2, 3, 4.
- A cell claims a surface it does not provide: row 2 (single-user refusal).
- The value leaks through a dry run, check row or receipt: rows 2, 3.
- The file's path reaches intake, or the role-secret path reaches a reviewer: rows 1, 5.
- A reviewer slot building an untrusted head reads a dev value: no row; accepted by the ruling
  for dev-only values, decision 1 above.

## Evidence

_Not yet run._
