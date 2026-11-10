---
brief: assay:assay:desk-containers:34
title: Project secrets reach dispatched worker and verifier windows, never an untrusted head
why: >-
  A Verify row that needs a project credential or a live non-production service has no way to
  run in a dispatched window: the launcher carries the role App keys and model credentials, and
  nothing else. Such rows are witnessed today by an operator from the Environment-blocked queue,
  or not at all, and the gap invites the worst fix, the implementer pasting their own output.
  This brief designs the one channel by which a project secret reaches a dispatched window
  without a commit, without a CI variable a pipeline can read, and never beside an untrusted
  pull-request head.
wave: 9
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  This creates a new custody path for project credentials into agent processes. The human
  confirms the scope unit, which roles may ever hold a secret, that no reviewer slot or
  untrusted head can reach one, and the revocation and audit story, before any code lands.
decision-trigger: creation
issues: [6065]
schema: brief-v2
version: 1
authored: 2026-11-10 by assay-worker-app (issue #6065 part 2)
sources:
  - "Issue #6065: project secrets have no channel to reach dispatched reviewer and worker environments."
  - "freshness-checked 2026-11-10 @ 7cbf8a06b (origin/main): containers/secrets.md, cellctl envkeys.go and celldb.go, deskdispatch exec.go, verify-desk and pr-review-desk skills, docs/board.md bucket 4."
  - "containers/secrets.md: the normative runtime credential contract (path-by-env, read-only mount, fail-closed, no secret in an image layer)."
exec-tier: strong
exec-tier-why: >-
  Custody and trust boundaries across launcher, dispatcher and role skills; a green unit test
  can hide a secret reaching the wrong process.
domain: complicated
outcome: none
consumers:
  - "tools/desk/cmd/cellctl/: follow-up desk-containers/34 (role secret map, check, launch export, receipts)"
  - "containers/secrets.md: follow-up desk-containers/34 (inventory row and per-surface recipe for project secrets)"
  - "tools/desk/cmd/deskdispatch/: follow-up desk-containers/34 (strip the secret variables from any child that runs an unblessed head)"
  - "plugins/assay/skills/verify-desk/SKILL.md, plugins/assay/skills/pr-review-desk/SKILL.md: follow-up desk-containers/34 (retire the interim Environment-blocked note for rows the channel covers)"
---

# Brief 34 — Project secrets reach dispatched worker and verifier windows

## Context

files: `tools/desk/cmd/cellctl/{envkeys.go,check.go,launch.go,desk.go}`,
`tools/desk/cmd/cellctl/rolesecret.go` (planned),
`tools/desk/cmd/cellctl/rolesecret_test.go` (planned),
`tools/desk/cmd/deskdispatch/exec.go`, `containers/secrets.md`, `docs/cellctl.md`,
`plugins/assay/skills/{verify-desk,pr-review-desk,worker-desk}/SKILL.md`,
`changelog/role-secrets.md` (planned).

facts (freshness-checked 2026-11-10 @ 7cbf8a06b):

- A Verify row now states where it runs: the optional `Env` column (`tree`, `host[:<name>]`,
  `service:<name>`, `cluster:<name>`; `docs/brief-rules.md` rule 45). A `service:<name>` row
  names a non-production instance and is never `check:ci`. That name is the join key this
  channel resolves; nothing in the tree carries the value.
- The launcher already moves secrets by PATH, never by value: role App keys are files checked
  owner-only by `cellctl check`; the cell database password is a pgpass file named by
  `CELL_DB_PASSFILE`, whose mode `cellctl check` verifies and whose value no dry-run prints;
  containers mount the App key read-only and name it by `ASSAY_APP_PEM_FILE`
  (`containers/secrets.md` §2, §5 fail-closed).
- `deskdispatch` builds no per-agent environment: a dispatched agent inherits the desk
  window's process environment, and `exec.go` names a closed list of children that receive the
  dispatcher credential. So whatever a role window holds, every agent it dispatches holds.
- Reviewer slots read untrusted pull-request heads (the pr-review-desk rule is never to build
  one). Verifier slots run merged main. Worker slots implement trusted, dispatched items.
- The verifier App already signs verdict payloads (RS256, `verdict-v1`), and the transcriber
  checks the signature against the public key only. That lane ships inert behind its own ruling.
- Interim (this brief's issue, part 3): such a row is recorded could-not-check with its exact
  command, lands in the board's Environment-blocked queue, and an operator who holds the
  environment, never the implementer, runs it.

single-point-of-failure: the role scoping in the launcher (which windows get the path). The
layers behind it: the dispatcher strips the variables from any child running an unblessed
head; reviewer windows never receive the map at all; the secret file is owner-only and
outside every worktree, so no commit can carry it; the leak sweep gates every public tree.

design-fit:
  owner: cellctl (the launcher already owns every runtime credential path for a window)
  contract: none — extends the cell config key registry and `containers/secrets.md`
  retires: [implementer-reported runs of secret-bound rows, ad-hoc exported service credentials in a desk shell]
  weight: verbs +0, flags +0 required, config keys +1 (one role-secret map key), refusal predicates mode/owner/role/stale/missing
  why-add: The launcher is the only place that already knows the role and holds credential custody; a new broker or vault client would be a second custody owner.

## Read first

- [Runtime credential contract](../../../containers/secrets.md) §1, §2, §5.
- [cellctl](../../cellctl.md) `CELL_DB_PASSFILE` and `cellctl check`.
- `docs/brief-rules.md` rule 45, the `Env` column.

## Human decision

A Verify check that needs a database password or a service token for a non-production
instance cannot run in a dispatched agent window today. This decides how such a project
secret reaches those windows. Fixed under every option: a secret never enters a commit, a
container image or a pipeline variable that merge-request pipelines can read; an untrusted
pull-request head never runs in a process that holds one; reviewers never hold one; no secret
value is ever printed, logged or hashed into a record.

Options:
1. **Launcher-held file per project and role, plus a signed outcome for reviewers
   (recommended).** The operator keeps one owner-only file per project, role and named service,
   outside every checkout. The launcher checks its mode and owner and exports only its PATH,
   and only into worker and verifier windows. A dispatched child running an unblessed head gets
   the variable stripped. Reviewers never get it; they accept the verifier's signed outcome
   record for that row instead of a run. Revoke by rotating or deleting the file at its source,
   after which the launcher refuses to start the role. Audit is a launcher receipt naming role,
   service, path and time, never the value or a digest of it.
2. **Operating-system keychain lookup by project scope.** Same roles and scoping as option 1,
   but the launcher reads the value from the user keychain at window start and writes a
   short-lived owner-only file. Better at-rest protection on one desktop; no equivalent on
   Linux containers or Kubernetes, so the container path would still need option 1.
3. **Local-only verify lane holds the secret; nobody else does.** Only a dedicated verifier
   window on the operator's machine holds secrets; workers never do. It runs the row on merged
   main and signs the outcome; reviewers and the board accept the signature. Smallest exposure,
   but workers cannot run the check while implementing, so failures surface only after merge.
4. **Hold.** No channel. Such rows stay in the Environment-blocked queue for an operator.

Default if no answer: option 4 (hold). A credential channel is never opened by silence.

## Ground rules

- Authoring this brief implements nothing. Implementation is a draft PR on its own branch;
  no live service, cluster or real credential is touched by any test. Fixtures use
  synthetic secret files whose contents are plainly fake words, never a realistic shape.
- Never weaken an existing custody control: owner-only mode checks, read-only mounts, the
  no-secret-in-a-layer scan, the leak sweep, and the never-build-an-untrusted-head rule stay
  as they are. If an implementation step would need one loosened, stop and report it.
- Stop at implemented; a non-implementer verifies.

## Task

1. **Scope unit.** A secret is scoped to (project repository, role, service name). The service
   name is the `<name>` of a `service:<name>` Env cell. One cell config key names a directory
   or map of such files; the registry in `envkeys.go` classes it as a cell key, never a machine
   key. Roles that may hold one: worker, verifier. Reviewer, intake and the coordinator never.
2. **Check.** `cellctl check` refuses a role-secret file that is not a regular file, not
   owner-only, owned by another user, a symlink out of its directory, or inside any worktree or
   checkout. The dry run prints paths and role bindings, never a value.
3. **Launch.** `cellctl desk` exports, for an allowed role only, one variable per bound service
   naming the PATH. A disallowed role launched with a binding refuses rather than dropping it
   silently. Never pass a value in argv or in the environment.
4. **Dispatch boundary.** `deskdispatch` strips every role-secret variable from a child whose
   item is not trusted (an unblessed author or an unblessed head) and from every review-kit
   child. Add the variables to the closed inventory in `exec.go` as never-inherited for those
   children.
5. **Revocation and audit.** Rotation is replacing the file at its source; a removed file makes
   the next launch for that role refuse. The launcher appends a receipt (role, project, service,
   path, file modification time, launch time) to the cell's own state, never the value and never
   a hash of it, since a hash of a short password is a guessing oracle.
6. **Reviewer witness.** For a `service:` row, the review kit and the pr-review-desk skill
   accept the verifier's signed outcome for that row, never a run report. Until the signed
   verdict lane is enacted, the interim Environment-blocked rule stands.
7. Update `containers/secrets.md` (inventory row, per-surface recipe), `docs/cellctl.md`, the
   three role skills and a changelog fragment.

## Verify

All tests below are implementation deliverables, not results claimed by this authoring PR.
Named tests must execute; zero matches or a skip fails independent verification.

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestRoleSecretScope$' > "$o" 2>&1 && grep -F -- '--- PASS: TestRoleSecretScope ' "$o")` | Named PASS; worker and verifier get the path variable; reviewer, intake and coordinator do not, and a reviewer launched with a binding refuses. |
| 2 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestRoleSecretCheck$' > "$o" 2>&1 && grep -F -- '--- PASS: TestRoleSecretCheck ' "$o")` | Named PASS; group-readable, foreign-owned, symlinked-out and inside-a-worktree files each refuse; the dry run output contains no fixture value. |
| 3 | check +flow | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/deskdispatch/ -run '^TestRoleSecretUntrustedHead$' > "$o" 2>&1 && grep -F -- '--- PASS: TestRoleSecretUntrustedHead ' "$o")` | Named PASS; an unblessed item and every review-kit child run with no role-secret variable; a trusted worker item keeps it. |
| 4 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestRoleSecretRevoke$' > "$o" 2>&1 && grep -F -- '--- PASS: TestRoleSecretRevoke ' "$o")` | Named PASS; a removed file refuses the next launch; the receipt names role, service and path and contains neither the value nor any hash of it. |
| 5 | check +flow | host | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestRoleSecretLaunchFlow$' > "$o" 2>&1 && grep -F -- '--- PASS: TestRoleSecretLaunchFlow ' "$o")` | Named PASS on a POSIX host (real file modes and owners); the compiled cellctl checks a scratch cell, launches a worker window whose dispatched child sees only the path variable, then refuses the same cell once the fixture file is made group-readable; no output contains the fixture contents. |
| 6 | check | host | `statusgen --root . --lint --acceptance-forge` | exit 0; output contains "LINT: PASS". The flag reads the forge, so this row runs on a host with forge read access, never network-off. |
| 7 | check | tree | `statusgen --consumers --brief desk-containers/34 --base $(git merge-base origin/main HEAD)` | Pre-merge, on the implementation branch: exit 0; declared implementation sites and follow-up references corroborate; out-of-scope judgments stay reviewer-owned. |

## Threat model / pre-mortem

- A reviewer slot inherits the secret and builds an untrusted head that sends it out: rows 1, 3, 5.
- An unblessed item dispatched to a worker carries the variable into a hostile build: row 3.
- A secret file is readable by other local users or sits inside a checkout and gets committed: rows 2, 5.
- A secret is printed by a dry run, a receipt or a log, or a receipt carries a guessable hash of it: rows 2, 4.
- A revoked secret keeps working because the launcher cached it: row 4.
- A trusted worker's own dependency exfiltrates the secret it legitimately holds: no row; review-only. Mitigation is scope (non-production only, per-role, per-service) and rotation.
- A merge-request pipeline reads the secret from a CI variable: no row; review-only. The design adds no CI variable, and review confirms none appears.

## Evidence

<!-- Independent verifier records exact implementation SHA, commands, named results and
environment. No implementation evidence yet. -->

## Review

Human gate. Confirm the scope unit, that reviewer and intake windows can never hold a project
secret, that an untrusted head never runs beside one, and that revocation and audit record no
value. Passing tests do not provision any real secret.
