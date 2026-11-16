---
brief: assay:assay:desk-containers:35
title: Cell dev-env file contract — one coordinator-owned file per cell, names declared by Verify rows
why: >-
  Every desk window today loads its own .env file. The copies drift, the same dev credential
  sits in five places, and a .env file written by the implementer side can change what a Verify
  row sees with nothing recording it. This brief writes the contract that replaces them: one
  dev-env file per cell, written only by the coordinator role, outside every worktree, from which
  a Verify row receives only the variable names it declares.
wave: 1
depends: []
unblocks: [desk-containers/36, desk-containers/37, desk-containers/38]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  This defines a new custody path for credentials into every role window of a cell and a new
  input to verification. The human confirms the file rules (coordinator role as sole writer,
  other roles read only, outside every worktree, dev-only values), the deny-by-default
  allow-list of declarable names, how a surface proves that only the coordinator can write,
  and that a Verify row's attestation records names and a digest, never values.
decision-trigger: creation
issues: [6615]
decision-issue: 6615
schema: brief-v2
version: 1
authored: 2026-11-16 by assay-worker-app (issue #6615)
sources:
  - "Issue #6615: cell dev environment, one dev-env file per cell, admitted to Verify rows by declared name."
  - "The driver's ruling and amendment on #6615 (cited in `## Human decision`)."
  - "docs/streams/decisions/DR-desk-containers-34.md: the role-secret custody record this contract sits beside."
  - "freshness-checked 2026-11-16 @ 0d369ac33 (origin/main): statusgen/rowclass.go, statusgen/verifieradmission.go, containers/secrets.md §4.4, docs/brief-rules.md rule 45, spec/brief-v1.md, docs/cellctl.md, tools/desk/cmd/cellctl/envkeys.go."
exec-tier: strong
exec-tier-why: >-
  A custody contract and a grammar change to the Verify table that admission, the launcher and
  three role skills all build against; an ambiguity here becomes a credential leak downstream.
domain: complicated
outcome: none
consumers:
  - "containers/secrets.md: follow-up desk-containers/35 (new §4.5 cell dev-env file; inventory row)"
  - "docs/brief-rules.md, spec/brief-v1.md, docs/brief-template.md: follow-up desk-containers/35 (Env grammar gains declared names)"
  - "statusgen/rowclass.go: follow-up desk-containers/35 (lint accepts and checks the declared-names form)"
  - "docs/cellctl.md: follow-up desk-containers/35 (the CELL_DEV_ENV_FILE key, documented; launcher behaviour lands in desk-containers/37)"
---

# Brief 35 — Cell dev-env file contract

## Context

files: `containers/secrets.md`, `docs/brief-rules.md`, `spec/brief-v1.md`,
`docs/brief-template.md`, `docs/cellctl.md`, `statusgen/rowclass.go`,
`statusgen/rowenv_test.go`, `changelog/cell-dev-env-contract.md` (planned).

facts (freshness-checked 2026-11-16 @ 0d369ac33):

- A Verify row says where it runs in the optional `Env` column: `tree`, `host[:<name>]`,
  `service:<name>`, `cluster:<name>` (`docs/brief-rules.md` rule 45). `statusgen/rowclass.go`
  resolves the cell (`resolveRowEnv`, `rowEnvService = "service"`) and lints it
  (`rowEnvProblem`). The grammar names a service; it has no way to name the variables a row
  needs.
- An admitted verifier run builds the row environment with `admittedRowEnv`
  (`statusgen/verifieradmission.go`). It strips shell-startup variables, four forge token
  variables and inherited `GIT_*`, and passes everything else through, including any
  `ASSAY_ROLE_SECRET_*` path variable a holding window carries. So a row today sees whatever
  any `.env` file the window loaded put into the environment, and nothing records which names
  it read.
- Project role secrets have their own channel (`containers/secrets.md` §4.4,
  DR-desk-containers-34): a per-project, per-role, per-service owner-only file whose path
  reaches only worker and verifier windows. Reviewer, intake and coordinator never hold one.
  Its receipts record path and times, never a value or a hash of one. DR-desk-containers-34's
  write-path rule also binds here: no non-holding identity may write anything a holding window
  executes or reads as configuration, the holder's PATH and home included. The coordinator is a
  non-holding role, and worker and verifier windows are holders.
- The cell database pointer (`CELL_DB_DSN`, `CELL_DB_PASSFILE`) is composed into every role
  window today (`docs/postgres.md`). Brief 37 recommends keeping it on its own channel as cell
  infrastructure rather than a Verify-row credential, and puts its reach to the driver.

## Read first

- `containers/secrets.md` §4.4 and §5, `docs/streams/decisions/DR-desk-containers-34.md`.
- `docs/brief-rules.md` rule 45 and `statusgen/rowclass.go` from `resolveRowEnv` to
  `verifyRowClassProblems`.
- The doc comment on `admittedRowEnv` in `statusgen/verifieradmission.go`, including its
  "Out of scope" paragraph.

## Human decision

Each desk window loads its own `.env` file today. This decides the contract for one shared
file per cell.

Ruled already: one file per cell; the verifier admits into a Verify row only the names that
row declares and records the names plus the file's sha256, never values; admitted values are
redacted from captured output; per-desk `.env` files retire.
Ruled 2026-11-16, by the driver in their own login:
https://github.com/medici-finance/assay/issues/6615#issuecomment-6921966886

Amended: the coordinator role owns and maintains the file and is the only role that may write
it; implementer, reviewer and verifier roles read it only; the verifier refuses a file any
other role can write; every change to the file records the sha256 and the names added or
removed, never values.
Amended 2026-11-16, by the driver in their own login:
https://github.com/medici-finance/assay/issues/6615#issuecomment-6922038844

Still open, and the one question this brief puts to the driver: **how a cell proves that only
the coordinator role can write the file.** On a cell where every role window runs as one OS
user, file mode cannot tell the coordinator from the worker; DR-desk-containers-34 found the
same for role secrets.

Options:
1. **Prove it per surface; a cell that cannot prove it binds no file (recommended).** On a
   container cell the file is mounted read-write only into the coordinator container and
   read-only into the worker, reviewer and verifier containers, and the launcher records that
   mount layout where the verifier can read it. On a host cell the file is owned by the
   coordinator role's OS identity, and no other role identity can write it or its directory
   (mode, owner, access-control list). A cell whose role windows share one OS user binds no
   dev-env file; rows that declare names stay could-not-check until the cell can prove it. This
   matches DR-desk-containers-34's read-isolation rule. Its write-path rule is met separately,
   whichever option is chosen, by the allow-list in Task 3 and brief 36's role-secret strip
   (rows 8 to 13 here, and brief 36's matching rows), not by this option.
2. **Accept a single-user cell with an advisory write-deny.** The file is owner-only and a
   harness rule denies non-coordinator windows a write. Works on today's desktop cells, but the
   deny is client-side and advisory, so the verifier's refusal would rest on nothing it can
   check.
3. **Hold.** No shared file. Per-desk `.env` files stay, and so does the gap the issue names.

Default if no answer: option 3 (hold). A credential channel is never opened by silence.
Briefs 36 and 37 are written against option 1 and say so; they are re-derived if the answer is
another option.

Rule 48 requires an approved design-decision record before this brief or briefs 36 and 37
leave `todo`. It does not exist yet: `DR-desk-containers-35` is written from the ruling, the
amendment and the answer to this question, and the driver approves it by name.

## Ground rules

- Authoring and implementing this brief changes documents and one lint. No live service,
  database or real credential is touched. Fixtures use plainly fake words.
- Never weaken a custody control: the role-secret channel, owner-only checks, read-only mounts,
  the no-secret-in-a-layer scan and the leak sweep all stand. The dev-env file is a separate,
  narrower class and never carries a value that a role secret or the cell database passfile
  covers. That claim rests on checked rows, not on this sentence: the allow-list (Task 3; rows 8
  to 13) keeps every declarable name out of what a shell, loader, interpreter, tool or the
  role-secret channel reads, so the coordinator, a non-holding role, writes nothing a holding
  window executes or reads as configuration, as DR-desk-containers-34's write-path rule
  requires; and brief 36 removes every role-secret variable from a row that declares names, so
  no row holds both.

## Task

1. **File rules** — `containers/secrets.md` gains §4.5 "Cell dev-env file", and a row in §1's
   inventory. State, as normative text:
   - one file per cell, named by the cell's own `cell.env` key `CELL_DEV_ENV_FILE` (an
     absolute path); never `defaults.env`, never the launching shell;
   - the coordinator role is the only writer; implementer, reviewer and verifier roles read it
     only; intake neither reads nor writes it (a reversible default this brief takes: the ruling
     does not name intake, and intake runs no Verify row);
   - it lies outside every git checkout and worktree;
   - the format is `NAME=value` lines and `#` comment lines only; each name is in the Task 3
     allow-list; no shell expansion, no `export`; dev-only, throwaway values (a local database,
     a local API stub); production values, role secrets, forge tokens, App keys, model keys and
     the cell database password are never placed in it;
   - every value is at least 8 characters long; the write verb (brief 37) refuses a shorter one,
     and admission (brief 36) refuses a row whose declared value is shorter, so every admitted
     value is long enough to redact;
   - every write appends a change receipt: time, the file's new sha256, the names added and the
     names removed, never a value;
   - the write path adds one nonce comment line on every write, carrying at least 128 bits from
     the operating system's cryptographic random source, so the file digest that receipts and
     attestations record is not a guessing oracle for a value (a reversible default this brief
     takes; DR-desk-containers-34 forbids hashing a bare secret for that reason);
   - the proof that only the coordinator can write, per surface, as `## Human decision` rules,
     and how the verifier learns the coordinator's identity (Interface contract).
2. **Declared names** — extend the Env grammar in `docs/brief-rules.md` rule 45,
   `spec/brief-v1.md` and `docs/brief-template.md`: a `service:<name>` cell may carry the names
   the row needs, `service:<name>:<NAME>[,<NAME>…]` (for example
   `service:pg-local:DEVENV_PGHOST,DEVENV_PGPORT`). `<name>` keeps its grammar, which admits
   neither `:` nor `,`, so the form parses without ambiguity. A row that declares no names
   receives none from the file. A row passes a declared value to a tool explicitly in its own
   command (for example `PGHOST="$DEVENV_PGHOST" psql …`), where review sees it.
3. **Allow-list, deny by default** — a declarable name matches `DEVENV_[A-Z0-9_]+` and nothing
   else. No shell, dynamic loader, interpreter, forge client, git or Assay component reads a
   name in that namespace on its own, so a value the coordinator writes reaches a row only
   through a name the row's command reads explicitly. Every other name refuses, which covers by
   construction each class below; the lint and admission (brief 36 Task 1) each name these
   classes in their refusal and test one case per class:
   - path and home: `PATH`, `HOME`, `XDG_CONFIG_HOME`;
   - dynamic loader: `LD_PRELOAD`, `LD_LIBRARY_PATH`, `DYLD_*`;
   - interpreter options: `NODE_OPTIONS`, `PYTHONPATH`, `PERL5LIB`;
   - tool configuration: `GH_CONFIG_DIR`, `GIT_*`, the four forge token variables, the
     shell-startup set `admittedRowEnv` strips;
   - agent sockets: `SSH_AUTH_SOCK`;
   - Assay variables: `ASSAY_ROLE_SECRET_*`, `ASSAY_CELL_DEV_ENV_FILE` and every other
     `ASSAY_*` name.

   `statusgen/rowclass.go` accepts the Task 2 form and PROBLEMs: a malformed name; a duplicate
   name; a name outside the allow-list, naming its class where it is one of the above; and
   declared names on any kind but `service`.
4. **Key documented** — `docs/cellctl.md` §`cell.env` documents `CELL_DEV_ENV_FILE` as
   specified here and points at `containers/secrets.md` §4.5; the launcher behaviour is
   desk-containers/37's.
5. Changelog fragment.

## Interface contract

Brief 36 builds admission against: the key `CELL_DEV_ENV_FILE`, projected into windows as the
path variable `ASSAY_CELL_DEV_ENV_FILE` (brief 37); the file format and the 8-character floor
in Task 1; the Env form in Task 2; the allow-list in Task 3. Under option 1 the verifier learns
the coordinator role's identity, and on a container cell the mount layout, only from what the
launcher records from the cell's identity map (brief 37 Task 2), never from the file, the
worktree or a variable a window can set; when that record is absent or unreadable the verifier
refuses. Brief 37 builds the launcher against Task 1 and the surface rule the decision picks.

## Definition of Done

Every Verify row below passes; §4.5 states each Task 1 rule; the grammar, the allow-list, the
lint and the three documents agree.

## Verify

All tests below are implementation deliverables, not results claimed by this authoring PR.
Named tests must execute; zero matches or a skip fails independent verification.

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check +dereference | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestEnvDeclaredNames$' > "$o" 2>&1 && grep -F -- '--- PASS: TestEnvDeclaredNames ' "$o")` | Named PASS; `service:pg-local:DEVENV_PGHOST,DEVENV_PGPORT` resolves to the service and two names; a malformed name, a duplicate name and names on a non-`service` kind are each a PROBLEM. |
| 2 | check | tree | `grep -F '### 4.5 Cell dev-env file' containers/secrets.md` | One match. |
| 3 | check | tree | `grep -F 'CELL_DEV_ENV_FILE' docs/cellctl.md` | At least one match. |
| 4 | check | tree | `grep -F 'service:<name>:<NAME>' docs/brief-rules.md` | At least one match. |
| 5 | check +flow | host | `statusgen --root . --lint --acceptance-forge` | exit 0; output contains "LINT: PASS"; the lint reads the grammar in the documents and the briefs together. Reads the forge, so it runs on a host with forge read access. |
| 6 | check | tree | `statusgen --consumers --brief desk-containers/35 --base $(git merge-base origin/main HEAD)` | Pre-merge, on the implementation branch: exit 0. |
| 7 | gate:human | tree | Human review: section 4.5 of the runtime credential contract against this brief's Human decision | The ruled rules and the chosen surface option each appear, and no rule loosens §4.4. |
| 8 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestEnvNameAllowList$/^path$' > "$o" 2>&1 && grep -F -- '--- PASS: TestEnvNameAllowList/path ' "$o")` | Named PASS; declaring `PATH`, `HOME` or `XDG_CONFIG_HOME` is a PROBLEM naming the path class. |
| 9 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestEnvNameAllowList$/^loader$' > "$o" 2>&1 && grep -F -- '--- PASS: TestEnvNameAllowList/loader ' "$o")` | Named PASS; declaring `LD_PRELOAD`, `LD_LIBRARY_PATH` or `DYLD_INSERT_LIBRARIES` is a PROBLEM naming the loader class. |
| 10 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestEnvNameAllowList$/^interpreter$' > "$o" 2>&1 && grep -F -- '--- PASS: TestEnvNameAllowList/interpreter ' "$o")` | Named PASS; declaring `NODE_OPTIONS` or `PYTHONPATH` is a PROBLEM naming the interpreter class. |
| 11 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestEnvNameAllowList$/^toolconfig$' > "$o" 2>&1 && grep -F -- '--- PASS: TestEnvNameAllowList/toolconfig ' "$o")` | Named PASS; declaring `GH_CONFIG_DIR`, `GH_TOKEN`, a `GIT_*` name or `BASH_ENV` is a PROBLEM naming the tool-configuration class. |
| 12 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestEnvNameAllowList$/^socket$' > "$o" 2>&1 && grep -F -- '--- PASS: TestEnvNameAllowList/socket ' "$o")` | Named PASS; declaring `SSH_AUTH_SOCK` is a PROBLEM naming the agent-socket class. |
| 13 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C statusgen test -count=1 -v . -run '^TestEnvNameAllowList$/^assay$' > "$o" 2>&1 && grep -F -- '--- PASS: TestEnvNameAllowList/assay ' "$o")` | Named PASS; declaring an `ASSAY_ROLE_SECRET_*` name, `ASSAY_CELL_DEV_ENV_FILE` or any other `ASSAY_*` name is a PROBLEM naming the Assay class; a name outside every class and outside `DEVENV_` is still a PROBLEM (deny by default). |

## Threat model / pre-mortem

- An implementer-side window rewrites the file to change what a Verify row sees: Task 1 and the
  surface rule; enforced by briefs 36 and 37; row 7.
- A row reads a value it never declared: Task 2, row 1; enforced by brief 36.
- The coordinator, a non-holding role, steers what a holding verifier or worker window executes
  or reads as configuration by declaring `PATH`, a loader or interpreter variable, a tool
  configuration name, an agent socket or an `ASSAY_*` name: the Task 3 allow-list, rows 8 to 13;
  enforced again at admission by brief 36.
- A row declares `GH_TOKEN` or a `GIT_*` name to pull a stripped value back in: outside the
  allow-list, row 11.
- A digest of a one-line file lets a reader guess a short value: the nonce line and the
  8-character floor, Task 1.
- A production value or role secret lands in the dev-env file: no row; review-only, row 7 and
  the leak sweep. Mitigation is the class rule in Task 1.

## Evidence

_Not yet run._
