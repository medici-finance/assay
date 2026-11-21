---
brief: assay:assay:desk-containers:40
title: Host-surface role secrets — per-role OS identity launch and its verified checks
why: >-
  Role secrets bind on the container surface only. The scope ruling on #6065 deferred the host
  surface, so `cellctl check` refuses every `CELL_ROLE_SECRET_SURFACE=host` binding, and two
  Verify rows of desk-containers/34 name host tests that do not exist yet. This brief builds
  the host surface that those rows check: a role-to-identity map, the identity, elevation and
  writable-path checks that verify it, and a launch that starts each role window under its
  own OS identity.
wave: 10
depends: []
unblocks: []
effort: M
gate: human
design: DR-desk-containers-34
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  This lifts the blanket host-surface refusal on a credential custody path, and it makes the
  launcher start windows as other OS users. The human confirms how the launcher switches
  identity (the one question below), and that every check refuses a host that does not
  separate the holder roles from the rest.
decision-trigger: creation
issues: [6520]
schema: brief-v2
version: 1
authored: 2026-11-17 by assay-worker-app (issue #6520)
sources:
  - "Issue #6520: author the follow-up brief for the host-surface work behind rows 5 and 9 of desk-containers/34."
  - "Issue #6301: the driver's option-A ruling (leave the desk-containers/34 Verify table as written and file the host-surface follow-up)."
  - "Issue #6065: the driver's scope ruling that cut desk-containers/34 to the container surface and named host-surface identity launch as a follow-up brief."
  - "PR #6516: the first independent verification of desk-containers/34 (VERIFY: FAIL; rows 5 and 9 name tests not defined on main)."
  - "docs/streams/decisions/DR-desk-containers-34.md: the approved design record, which already covers per-role OS identity on a host."
  - "freshness-checked 2026-11-17 @ e259796bb (origin/main): tools/desk/cmd/cellctl/{rolesecret.go,rolesecret_test.go,launch.go,desk.go,cell.go,envkeys.go,hotfix_unix.go}."
  - "freshness-checked 2026-11-21 @ 09c51fcf7 (origin/main): tools/desk/cmd/cellctl/{launch.go,git.go,sessionroots.go,cadence.go,envkeys.go,rolesecret.go} (the launcher's git calls, the run directory mode, both host-refusal texts)."
exec-tier: strong
exec-tier-why: >-
  Yes to (a) the launch mechanism is a design choice the facts leave open, (b) the checks span
  the launcher, the dispatcher's worktrees, tmux and the operator's host, and (c) a check that
  reads a mode bit instead of probing as the identity can pass its own unit tests and still
  leave the secret readable.
domain: complicated
outcome: none
consumers:
  - "tools/desk/cmd/cellctl/: follow-up desk-containers/40 (role-to-identity map key, host-surface checks, launch under the role identity)"
  - "containers/secrets.md: follow-up desk-containers/40 (host-surface recipe beside the container recipe)"
  - "docs/cellctl.md: follow-up desk-containers/40 (the identity-map key and the host surface)"
---

# Brief 40 — Host-surface role secrets: per-role OS identity launch

## Context

files: `tools/desk/cmd/cellctl/{rolesecret.go,rolesecret_test.go,envkeys.go,check.go,launch.go,desk.go}`,
`tools/desk/cmd/cellctl/rolesecret_host.go` (planned),
`tools/desk/cmd/cellctl/rolesecret_host_test.go` (planned),
`tools/desk/cmd/cellctl/rolesecret_flow_test.go` (planned),
`containers/secrets.md`, `docs/cellctl.md`,
`changelog/role-secrets-host-surface.md` (planned).

facts (freshness-checked 2026-11-17 @ e259796bb; the launcher-git fact 2026-11-21 @ 09c51fcf7):

- `rolesecret.go` `roleSecretSurfaceProblem()` refuses the host surface outright:
  `CELL_ROLE_SECRET_SURFACE=host is not built yet (per-role OS identity launch is a follow-up)`.
  `cellctl desk` calls `roleSecretLaunchGate()` before composing any window (`desk.go`).
- `envkeys.go` registers `CELL_ROLE_SECRETS` and `CELL_ROLE_SECRET_SURFACE` as cell keys. No
  role-to-identity key exists. The holder roles are `worker-desk` and `verify-desk`
  (`roleSecretHolderRoles`).
- `roleSecretFileProblem` expects the secret file's owner to be `roleSecretOwnerUID()`, a
  package seam set to `os.Getuid` (the launcher's own uid). `roleSecretACLProbe` is the
  access-control-list seam, with real probes for Linux and macOS.
- A host window's environment passes through `unsetRoleSecretEnv` (`launch.go`), so no host
  window carries a role-secret path today.
- A scrubbed host cell starts every role window in ONE private tmux server, at the socket
  `<cell dir>/run/tmux.sock` (`launch.go`), and gives desk verbs ONE home, `<cell dir>/home`
  (`cell.go`). The tmux server's identity is every window's identity, and anyone who can
  reach a window's socket can type into it.
- The launcher runs git as itself in checkouts other identities may write: in the cell repo
  at launch (`launch.go` `worktree add` and `worktree lock`; `git.go` fetch of main), in the
  stream roots (`git.go`), and in every `CELL_ROOTS` checkout, which `sessionroots.go`
  fetches into at boot and before every cadence pass (`cadence.go`).
- Precedent for running a child as another uid: `hotfix_unix.go` `hotfixDropAttr` sets
  `syscall.Credential` when cellctl runs as root under sudo. Nothing else in cellctl switches
  identity.
- `TestRoleSecretReadIsolation` (desk-containers/34 row 8) passes today. Its `surface-host`
  case expects a refusal that starts `MISS  role secrets: refused — CELL_ROLE_SECRET_SURFACE`.
- `grep -rn -E "func TestRoleSecret(LaunchFlow|ReadDenyFlow)" tools/desk` exits 1.

single-point-of-failure: the host-surface check in `cellctl check`, which decides whether a
host provides per-role separation. The layers behind it: the launch re-runs the same check
and refuses a window it cannot place under its identity; the probes run AS each non-holding
identity rather than reading mode bits, so an access-control entry or group the code did not
model still counts; the dispatcher strip from desk-containers/34 still removes the path
variables from any untrusted child; a cell that sets no identity map binds no secret.

design-fit:
  owner: cellctl (it already owns role-secret custody and window launch)
  contract: none — extends the cell config key registry and `containers/secrets.md`
  retires: [the blanket host-surface refusal in roleSecretSurfaceProblem]
  weight: verbs +0, flags +0, config keys +1 (the role-to-identity map, counted in desk-containers/34's design-fit and not built there), refusal predicates +identity/elevation/root-equivalent/launcher-writable/host-writable/ancestor/socket/home
  why-add: The map and the checks were approved in DR-desk-containers-34 as part of the custody path; leaving the host surface refused was the considered alternative, and the driver chose to build it (#6301).

## Read first

- [Brief 34](brief-34-project-secrets-reach-dispatched-windows.md) Tasks 1–3 (scope unit, the
  check, read isolation with its write-path rules, launch). This brief builds their host half
  and changes none of their text.
- [DR-desk-containers-34](../decisions/DR-desk-containers-34.md) `accepted:` entries 2 and 4.
- [Runtime credential contract](../../../containers/secrets.md) §2, §4.4 (the container
  role-secret recipe Task 7's host recipe sits beside), §5.

## Human decision

Project secrets can reach worker and verifier windows today only in containers. On an
ordinary host, every role window runs as one operating-system user, so file mode cannot keep
a reviewer window from opening the worker's secret file, and the launcher refuses such a
binding. The approved design lets a host qualify when each role runs as its own OS user, no
non-holding user can become a holder or root without a password prompt, and no non-holding
user can write anything a holding window runs. Creating those users stays with the operator.

What is not decided: how the launcher, running as one user, starts a window as another user.

Options:
1. **The operator grants the launching user a password-free switch to each role user, and
   to nothing else (recommended).** The launcher runs unprivileged and starts each window
   through that switch. The check refuses the cell if any non-holding role user can make the
   same switch without a prompt, and if the launching user is any role's user. No part of
   the launcher ever runs as root. Cost: the operator maintains one switch rule per role user.
2. **The launcher is run as root for each window launch, and drops to the role user before
   starting the window.** The same pattern already exists for building a local tool hotfix.
   No standing switch rule is needed, but the launcher reads the cell configuration as root
   at every launch, so a fault in that code runs with root rights.
3. **Hold.** The host stays refused; secret-bound checks on a host keep waiting for an
   operator, and the two host checks named by the earlier verification stay failing.

Default if no answer: none — blocks until answered. Until it is answered the host stays refused (option 3); a host credential path is never opened by silence.

## Ground rules

- Authoring this brief implements nothing. Implementation is a draft PR on its own branch.
  No test touches a real credential, a live service or a cluster. Fixtures use synthetic
  files whose contents are plainly fake words.
- Do not edit desk-containers/34's Task, Verify table, Evidence or status. Its rows 5 and 9
  name the two flow tests this brief delivers; after this brief merges, a fresh independent
  verification of desk-containers/34 runs those rows unchanged.
- Never weaken an existing custody control: the container surface, owner-only checks, the
  access-control-list refusal, the dispatcher strip, `unsetRoleSecretEnv` for every window
  that is not a verified holder, and the leak sweep stay as they are. If a step needs one
  loosened, stop and report it.
- Tests never provision OS users or edit privilege rules. The operator provisions the host
  for the two flow tests (Task 6). The in-tree tests inject the identity probe.
- Stop at implemented; a non-implementer verifies.

## Task

1. **Role-to-identity map.** Add the cell key `CELL_ROLE_IDENTITIES`, registered in
   `envkeys.go` as a cell key, never a machine key:
   `<desk-role>=<os login>[,<desk-role>=<os login>...]`. Every role the cell launches must be
   mapped. A missing role, an unknown role, a duplicate role, an empty login, or a login that
   does not resolve to an account refuses. The dry run and `cellctl check` print the map,
   which holds no secret.
2. **Host-surface check.** With `CELL_ROLE_SECRET_SURFACE=host`, `cellctl check` replaces the
   blanket refusal with these checks, and refuses the binding on the first that fails. A
   host-surface cell with no identity map keeps a refusal starting
   `MISS  role secrets: refused — CELL_ROLE_SECRET_SURFACE`, so desk-containers/34 row 8
   passes unchanged.
   - **Identities.** Every non-holding identity exists, is not uid 0, and differs from every
     holder's identity and from the secret file's owner. The launching identity (the user
     running cellctl) is no role's identity.
   - **Owner.** On this surface the expected owner of a secret file is the identity the map
     gives its bound role, not the launcher's uid. Generalise `roleSecretOwnerUID` so the host
     surface passes the role's uid and the container surface keeps today's value.
   - **Elevation.** As each non-holding identity, a non-interactive attempt to become root,
     to become each holder identity and to become the launching identity must fail. The probe
     runs the host's password-free forms of the switch (`sudo -n`, and `doas -n` where
     installed). A success, or a probe that cannot run, refuses.
   - **Root-equivalent access.** A non-holding identity that can write a rootful container
     runtime's daemon socket can start a container that mounts the host filesystem and read
     the secret file without any prompt, so the switch probe above does not see it. As each
     non-holding identity, a write-permission probe (`access(2)` with `W_OK`, which honours
     group membership such as a `docker` group) must fail on every rootful container runtime
     socket present on the host: Docker's, rootful Podman's, containerd's, and any socket
     that identity's `DOCKER_HOST` or `CONTAINER_HOST` names. A success refuses with a reason
     naming root-equivalent access and the socket. A rootless runtime socket that runs as the
     probed identity itself is not root-equivalent and is not refused. A runtime socket that
     belongs to a holder or to the launching identity (a rootless runtime under that user's
     runtime directory) is holder-equivalent instead: the same probe must fail on it, so in
     effect every container runtime socket not owned by the probed identity is probed. A
     probe that cannot run refuses. This list is not exhaustive. A daemon listening on
     unauthenticated loopback TCP, and membership of groups such as `lxd`, `libvirt` or
     `disk`, also give root without a prompt and no socket probe sees them; the host recipe
     (Task 7) names their absence as an operator precondition.
   - **Launching identity.** Under either launch option in the Human decision, the launching
     identity can become every holder without a prompt (root under option 2), so the check
     treats it as holder-equivalent. Besides the elevation target above, as each non-holding
     identity a write-permission probe (`access(2)` with `W_OK`) must fail on everything the
     launching identity runs or reads as configuration: the cellctl binary and its
     directory; every directory on the launcher's PATH, where the switch command and `tmux`
     resolve; the launcher's home and the shell startup and configuration files its shell
     reads there; the files the switch reads its rule from (for example a sudoers drop-in);
     the cell directory with its `run` directory and `<cell dir>/home`; and the git common
     directory, its `config` and `hooks`, of every checkout the launching identity runs git
     in. Those are the cell repo (where the launch fetches main and runs `worktree add` and
     `worktree lock`), each stream root, and each `CELL_ROOTS` checkout the session-root
     refresh fetches into at boot and before every cadence pass. Git runs repository-controlled
     code in those commands: a `post-checkout` hook on `worktree add`, a
     `reference-transaction` hook on fetch, and command-valued config such as
     `core.sshCommand`, `core.fsmonitor` and a credential helper. So a non-holder that can
     write any of them runs code as the launcher. A non-holding
     identity also must not be able to connect to the launcher-owned `<cell dir>/run/tmux.sock`
     (probed as `W_OK` on the socket and `X_OK` on `run`). A success, or a probe that cannot
     run, refuses.
   - **Writable paths.** As each non-holding identity, a write-permission probe (`access(2)`
     with `W_OK`, which honours access-control lists and groups) must fail on every path a
     holder executes or reads as configuration: the git common directory, its `config` and
     `hooks`, of each checkout a holder role uses; the cell config, `cell.env` and the
     identity map; every directory on the holder window's PATH, the cell's shim directory
     included; the holder's home; and the directory holding each secret file. A probe that
     cannot run refuses.
   - **Ancestors and ownership.** A probe of the named path alone is not enough. A non-holder
     that can write a directory's parent can rename the directory and put its own in its
     place, and a non-holder that owns a path whose write bit is cleared fails `W_OK` yet can
     `chmod u+w` it at any time. So for every path in the Root-equivalent access, Launching
     identity and Writable paths sets, and for each of that path's ancestors up to `/`, the
     check refuses when a non-holding identity can write it (the same `W_OK` probe, run as
     that identity) or owns it (its owner uid is that identity's). One exemption: an ancestor
     with the sticky bit set (such as `/tmp`) that a non-holder can write is allowed only when
     the child under it on the path is owned by a trusted identity: root, the holder, or the
     launching identity. Each refusal names the path, the ancestor and whether it was write
     access or ownership.
   - **Window separation.** A holder role's window gets its own tmux server and socket, and
     its own home, both owned by the holder identity, writable by no other identity, and not
     reachable by any non-holding identity (the socket's directory is owner-only). The one
     shared `<cell dir>/run/tmux.sock` server stays launcher-owned and still hosts every
     non-holding role's window, but each such window's process runs under its role identity
     through the switch, never as the launcher. Because `run` is owner-only and owned by the
     launcher, a process in a non-holding window cannot reach the server it was started in;
     the Launching identity probe above checks that this holds. On a host surface no window
     uses the launcher-owned `<cell dir>/home`: each non-holding window gets a home owned by
     its own role identity, separate from the launcher's and every holder's. A window's
     terminal still belongs to the launcher-owned server, so the pane command is the switch
     itself, run directly, and the pane closes when it exits (no `remain-on-exit`, no respawn
     into a launcher shell): no launcher shell is ever left in a pane to read input that a
     lower-privileged process injected.
   - **Dev-env file.** If the cell sets `CELL_DEV_ENV_FILE` (the cell dev-env file contract
     of desk-containers/35; desk-containers/37 Task 1 adds the key), probe as each holder identity: that file must be unreadable to
     every holder, else refuse. Where the key does not exist yet, this check is a no-op with a
     comment naming the contract.
   - **Seam.** All identity probes go through one package seam, `roleSecretIdentityProbe`, so
     the in-tree tests inject results. The real probe runs as the identity through the switch
     the Human decision picks; it never reads mode bits as a stand-in. The owner of a path
     and an ancestor's sticky bit (Ancestors and ownership) are read with `stat` as well as,
     never instead of, the probe.
3. **Launch under the role identity.** On a verified host surface, `cellctl desk` re-runs the
   Task 2 check, then starts the role's window under the mapped identity through the switch
   the Human decision picks. A holder window's environment carries its role-secret path
   variables, built after `unsetRoleSecretEnv` the same way the container plan builds them,
   and never a value. A non-holder window keeps `unsetRoleSecretEnv`. A window that cannot be
   started under its identity refuses, and never falls back to the launcher's identity. The
   launch receipt from desk-containers/34 records the identity login beside role, service
   and path.
4. **Lift the refusal.** `roleSecretSurfaceProblem()` accepts `host` only once Task 2's check
   passes. `roleSecretLaunchGate()` and every other call site keep refusing an unverified
   host surface. The `CELL_ROLE_SECRET_SURFACE` description in `envkeys.go` also says
   `host is not built yet`; rewrite it to describe the host surface, since Verify row 7
   greps the whole package.
5. **Checkouts.** A holder role works in checkouts its own identity owns. If the dispatcher
   would create a holder's worktree from a checkout a non-holder can write, Task 2's
   writable-path check refuses; it does not create a separate clone. Record the operator
   recipe (one clone per holder identity) in `containers/secrets.md`.
6. **Flow tests and operator recipe.** Add the two host flow tests below. They read the
   provisioned identities from the test-only variable `CELLCTL_TEST_ROLE_IDS`, the same
   grammar as Task 1 plus one key, `elevating=<login>`, naming an identity the operator gave
   password-free elevation for the negative case. Unset, both tests skip, and a skip fails
   their Verify rows. Each test builds the cellctl binary and runs it against a scratch cell
   under a temporary cells root; no output may contain the fixture contents.
   - `TestRoleSecretLaunchFlow` (planned): the compiled cellctl checks a scratch cell that declares
     the host surface with the provisioned identities, launches a worker window whose
     dispatched child sees only the path variable, then refuses the same cell once the
     fixture file is made group-readable.
   - `TestRoleSecretReadDenyFlow` (planned): the compiled cellctl launches the reviewer and worker
     windows from the scratch cell's own identity map. A child of the reviewer window fails to
     open the fixture by its path with a permission error, and a child of the worker window
     opens it. A second scratch cell whose map puts the reviewer on the worker's identity is
     refused at check, before any window starts. A third, whose reviewer is the `elevating`
     identity, is refused by the real elevation probe.
7. **Docs.** `containers/secrets.md`: the host-surface recipe (provision one OS user per role,
   the switch rule from the Human decision, one clone per holder identity, the identity-map
   key) beside the container recipe, with its operator preconditions the check cannot see: no
   container daemon on unauthenticated loopback TCP, and no non-holding user in a group such
   as `lxd`, `libvirt` or `disk`. `docs/cellctl.md`: the identity-map key and what the host
   check refuses. A changelog fragment.

## Interface contract

- `CELL_ROLE_IDENTITIES` grammar is Task 1's. desk-containers/34's existing keys keep their
  grammar; `CELL_ROLE_SECRET_SURFACE` gains no new value (`host` already parses).
- `roleSecretIdentityProbe` is the only way the check learns whether an identity can write a
  path or elevate. Its in-tree fakes answer per (identity, path) and per (identity, target); a target is
  root, a holder identity or the launching identity, and a path includes each container
  runtime socket and each ancestor of a probed path. The fakes also give each path's owner
  and sticky bit, so the ancestor and ownership cases run in-tree.
- The refusal text for an unmapped host surface keeps the prefix in Task 2, which
  desk-containers/34 row 8 matches.
- Test-only: `CELLCTL_TEST_ROLE_IDS` is read by `_test.go` files only, never by the binary.

## Definition of Done

Every Verify row passes. On a host provisioned per the recipe, the two flow tests pass
without a skip. desk-containers/34's Task, Verify table and status are untouched, and its
rows 1, 3 and 8 still pass. The `host is not built yet` text is gone from the source.
`containers/secrets.md` and `docs/cellctl.md` describe the host surface. No custody control
listed in the Ground rules is loosened.

## Verify

All tests below are implementation deliverables, not results claimed by this authoring PR.
Named tests must execute; zero matches or a skip fails independent verification.

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestRoleSecretHostSurface$' > "$o" 2>&1 && grep -F -- '--- PASS: TestRoleSecretHostSurface ' "$o")` | Named PASS with the identity probe injected; each case refuses with its own reason: no map, a role missing from the map, a reviewer on a holder's identity or the file owner's, a reviewer at uid 0, a reviewer that elevates without a prompt to root, to a holder or to the launching identity, a reviewer that can write a rootful container runtime socket or a holder's or the launcher's rootless one (a rootless socket of its own passes), a coordinator or intake on a holder's identity, the launcher being a role identity, a writable git common directory, hooks, cell config, identity map, PATH directory, holder home or secret directory, a writable cellctl binary or its directory, launcher PATH directory, launcher home or shell startup file, switch rule file, cell directory, `run` directory or launcher-owned cell home, a writable git common directory, `config` or `hooks` of the cell repo, a stream root or a `CELL_ROOTS` checkout the launcher runs git in, a probed path whose ancestor a reviewer can write, a probed path or ancestor a reviewer owns with its write bit cleared, a reviewer-writable sticky ancestor over a reviewer-owned child (a sticky ancestor over a trusted-owned child passes), a launcher tmux socket a non-holder can connect to, a non-holding window handed the launcher-owned cell home, a shared tmux socket or home for a holder, a dev-env file a holder can read, and a probe that cannot run; a fully separated map passes and the expected owner is the role's identity. |
| 2 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestRoleSecretHostLaunch$' > "$o" 2>&1 && grep -F -- '--- PASS: TestRoleSecretHostLaunch ' "$o")` | Named PASS with the switch seam injected; a verified host cell launches each window under its mapped identity, the worker window alone carries the path variable and no value, reviewer, intake and coordinator windows carry none, a window that cannot be switched refuses rather than falling back to the launcher's identity, and the receipt names the identity login. |
| 3 | check +flow | host:multi-user | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestRoleSecretLaunchFlow$' > "$o" 2>&1 && grep -F -- '--- PASS: TestRoleSecretLaunchFlow ' "$o")` | Named PASS on a POSIX host provisioned per Task 6; the compiled cellctl checks a host-surface scratch cell, launches a worker window whose dispatched child sees only the path variable, then refuses once the fixture is group-readable; no output contains the fixture contents. The same command as desk-containers/34 row 5. |
| 4 | check +flow | host:multi-user | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestRoleSecretReadDenyFlow$' > "$o" 2>&1 && grep -F -- '--- PASS: TestRoleSecretReadDenyFlow ' "$o")` | Named PASS on a POSIX host provisioned per Task 6; a reviewer child fails to open the fixture with a permission error and a worker child opens it; a map putting the reviewer on the worker's identity is refused at check before any window starts; a reviewer mapped to the elevating identity is refused by the real elevation probe; no output contains the fixture contents. The same command as desk-containers/34 row 9. |
| 5 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestRoleSecretReadIsolation$' > "$o" 2>&1 && grep -F -- '--- PASS: TestRoleSecretReadIsolation ' "$o")` | Named PASS; desk-containers/34 row 8 still holds after the host refusal is lifted: an unmapped host surface, an unset surface and an unknown surface each still refuse, and the host-surface cases row 8's Expect lists (a reviewer on a holder's identity, uid 0, elevation, a writable git directory, map, PATH or home) are still exercised under this test, as its own subtests or a table it shares with `TestRoleSecretHostSurface`. |
| 6 | check | tree | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/deskdispatch/ -run '^TestRoleSecretUntrustedHead$' > "$o" 2>&1 && grep -F -- '--- PASS: TestRoleSecretUntrustedHead ' "$o")` | Named PASS; the dispatcher still strips the path variables from an unblessed item and every review-kit child, now that a host worker window can carry them. |
| 7 | check | tree | `(! grep -rn -F 'host is not built yet' tools/desk/cmd/cellctl/)` | exit 0; the blanket refusal text is gone from the source. |
| 8 | check | tree | `statusgen --root . --lint` | exit 0; output contains "LINT: PASS". |
| 9 | check | tree | `statusgen --consumers --brief desk-containers/40 --base $(git merge-base origin/main HEAD)` | Pre-merge, on the implementation branch: exit 0; the declared implementation sites and follow-up references corroborate. |

## Threat model / pre-mortem

- The check reads mode bits and misses an access-control entry or group that lets a reviewer
  write a holder's hook or PATH directory: rows 1, 4 (probes run as the identity).
- A reviewer identity has password-free sudo and becomes the worker: rows 1, 4.
- Every holder window shares the cell's one tmux server, so a reviewer attaches and types into
  the worker window, or plants a file in the shared home: row 1.
- The launch cannot switch identity and quietly starts the worker window as the launcher, which
  a reviewer process could reach: row 2.
- The holder works in a worktree of a checkout the reviewer can write, and a planted hook runs
  as the worker: rows 1, 4.
- Lifting the host refusal also lifts it for a host cell with no map, or breaks desk-containers/34
  row 8: rows 1, 5, 7.
- A host worker window carries the path into an untrusted dispatched child: row 6.
- The flow tests skip on a single-user host and a verifier reads the skip as a pass: rows 3, 4
  (the grep requires `--- PASS`; a skip fails).
- The dev-env file of a holder's cell becomes a second channel a holder can read: row 1.
- A reviewer identity in a `docker` group, or with write access to another rootful container
  runtime socket, starts a container that mounts the host filesystem and reads the worker's
  secret without any prompt: row 1 (the root-equivalent probe).
- The launching identity can become every holder, and a reviewer plants code where the
  launcher runs it: a startup file in the launcher's home, a directory on its PATH, the
  cellctl binary, the switch rule file, or the launcher-owned cell directory and home. The
  next launcher shell runs the code and switches to the worker: row 1 (the launcher is
  holder-equivalent; probes and elevation target).
- A reviewer that can write the shared cell repo, a stream root or a `CELL_ROOTS` checkout
  plants a `post-checkout` or `reference-transaction` hook, or sets `core.sshCommand` there.
  The next launch or cadence pass runs `git worktree add` or `git fetch` as the launcher, which
  runs the planted code and switches to the worker: row 1 (the git common directory, `config`
  and `hooks` of every checkout the launcher runs git in are in the launcher probe set).
- A reviewer cannot write a holder's PATH directory but can write its parent, so it renames
  the directory and puts its own in its place after the check has passed; or it owns a probed
  path with the write bit cleared and restores the bit later: row 1 (every probed path's
  ancestors up to `/` are probed, and a non-holder-owned path or ancestor refuses).
- A non-holding window started inside the launcher-owned tmux server reaches that server and
  types into a holder's launch: row 1 (the launcher socket is unreachable to non-holders).
  A pane left running a launcher shell after the role process exits would read injected
  input as the launcher: review-only (the pane command is the switch itself and the pane
  closes on exit).
- Root without a prompt by a route no socket probe sees (a daemon on unauthenticated loopback
  TCP, `lxd`, `libvirt` or `disk` group membership): no row; the host recipe names these as
  operator preconditions.
- The operator's switch rule grants more than the role users (for example root): no row;
  review-only. The check refuses non-holders that can elevate, not an over-broad rule on the
  launcher; the recipe in `containers/secrets.md` states the narrow rule.

## Evidence

_Not yet run._

## Review

Human gate. Confirm the launch mechanism the Human decision picks, that a reviewer, intake or
coordinator identity can neither open a holder's secret nor write anything a holder runs, and
that no window ever falls back to the launcher's identity.
