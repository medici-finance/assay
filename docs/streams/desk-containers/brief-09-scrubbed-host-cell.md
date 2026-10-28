---
brief: assay:assay:desk-containers:09
title: "cellctl: host-local harness cell — scrubbed per-cell environment, `smoke`, `status`, session lock, stricter `check`"
wave: 0
depends: []
unblocks: ["desk-containers/10"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by the-desk dispatch (issue #1193)
sources:
  - "#1193 — the request: a product-cell session on the operator's laptop wrote an out-of-tree bridge (a `CELL_KIND=local` registration plus a shell function shadowing `cellctl`) because no cell kind ran the harness directly on the host inside a scrubbed environment; its `smoke`, `status`, session-lock and stricter-`check` behaviours have no `cellctl` equivalent ('What the bridge does' items 1-6)"
  - "tools/cellctl/cellctl:223-236 — `load_cell`'s kind switch: `k8s|house|container`, anything else dies with exit 3 before any verb runs (why `CELL_KIND=local` fails at cell.env parse)"
  - "tools/cellctl/cellctl:161-164 — the design rule the house kind is built on: the session keeps the operator's REAL HOME; only the desk verbs see the cell config-home, through `shim/`"
  - "tools/cellctl/cellctl:921-939 — `check_house`: the config home is a SYMLINK to the operator's; gitconfig and gh config are linked, never scrubbed"
  - "tools/cellctl/cellctl:1192-1205 — `check_codex_harness`: the codex login-status row already exists (`codex login status`, or its `--json` form); the scrubbed kind reuses it under the cell's own `CODEX_HOME`"
  - "tools/cellctl/cellctl:1311-1315 — the existing `[dry-run]` line `cmd_desk` prints under `DRY_RUN=1`; the plan grammar below extends it, it does not replace it"
  - "tools/cellctl/cellctl:1386-1405 — the two exec lines (codex / claude) every role window launches through; the scrubbed arm wraps them in `env -i`"
  - "tools/cellctl/cellctl:2168-2176 — the verb dispatch table `smoke` and `status` join"
  - "docs/cellctl.md §'Why the session keeps the real `HOME`' — the rule this kind deliberately inverts, and why the inversion is a KIND, not a flag on `house`"
  - "docs/cellctl.md §'House cells — `--kind house`' and §'Container cells' — the two nearest kinds; the scrubbed kind's section is written in the same register between them"
  - "docs/cellctl.md §'`cellctl check` — the codex harness block' — the row table the stricter rows extend"
  - "tools/cellctl/tests/house-cell.test.sh, tools/cellctl/tests/harness.test.sh — the fixture style every Verify row below follows: stub harness binaries on a private PATH, a temp operator config home, `DRY_RUN=1`, no network, no live model"
  - "freshness-checked 2026-09-16 @ 872ac03e — `git log origin/main -- tools/cellctl/cellctl` tops at #1189 (root drift + DESKD gating); no kind beyond k8s/house/container, no `smoke`/`status` verb, no session lock exists on main"
why: >-
  A cell that runs the harness directly on the laptop today has to keep the operator's whole
  shell — real HOME, forge login, gitconfig, SSH agent, cluster credentials — because the only
  host kind (`house`) is built to. A session that must NOT inherit any of that had no kind to
  register under, so it grew an out-of-tree bridge in a fourth language. Giving `cellctl` a kind
  that runs the harness on the host inside an environment it fully composes — nothing from the
  parent shell leaks in — plus the three verbs the bridge had to invent (`smoke`, `status`, the
  session lock) retires the reason the bridge exists.
version: 1
id: de019175-8488-4b78-a2f9-f7d804e44220
exec-tier: strong
exec-tier-why: (a) the env-composition and lock designs are fixed here but the codex/claude one-shot probe shapes must be verified against the installed CLIs at pickup; (c) an env leak that survives the tests is exactly the fault class this kind exists to close
consumers:
  - "tools/cellctl/testdata/cellctl-shell-oracle.sh: follow-up desk-containers/09 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/cellctl/tests/scrubbed-cell.test.sh: follow-up desk-containers/09 (this brief; new file, flips to fixed-here when it lands)"
  - "docs/cellctl.md: follow-up desk-containers/09 (this brief; the new kind's section + the two new verbs)"
  - "tools/cellctl/tests/house-cell.test.sh: out-of-scope (the house kind is untouched — row 9 proves the neighbour still passes; no edit is owed)"
---

# Brief 09 — cellctl: host-local harness cell — scrubbed per-cell environment, `smoke`, `status`, session lock, stricter `check`

## Context

files:
- `tools/desk/cmd/cellctl/` — the Go launcher that now carries this brief's behaviour (`loadCell` kind switch, `cmdNew`, `cmdCheck`, `cmdDesk`, `cmdSmoke`, `cmdStatus`, `cmdDown`); the bash implementation this brief landed (`load_cell` kind switch, a fourth kind `scrubbed`; `cmd_new --kind scrubbed`; `check_scrubbed`; the scrubbed arm of `cmd_desk`; two new verbs `smoke` and `status`; the session lock; `cmd_down` on a scrubbed cell) is kept as the parity oracle at `tools/cellctl/testdata/cellctl-shell-oracle.sh` (moved there in #1739).
- `tools/cellctl/tests/scrubbed-cell.test.sh` (planned) — the fixture suite, `--case <name>` selectable; every Verify row below names its case.
- `docs/cellctl.md` — new section *Scrubbed cells — `--kind scrubbed`* placed between *House cells* and *Container cells*; `smoke` and `status` added to the header usage block and the verb walk-through; the stricter rows appended to the `check` tables.
- `changelog/desk-containers-09-scrubbed-cell.md` (planned) — one `### Added` bullet.

facts:
- **Kinds today (2026-09-16 @ 872ac03e):** `load_cell` accepts exactly `k8s|house|container`
  (`tools/cellctl/cellctl:224-236`); any other `CELL_KIND` is `die` → exit 3 at cell.env parse,
  before the verb's own flags are read. This is why `cellctl up <cell> --harness claude` on a
  bridge-era `CELL_KIND=local` registration never reaches `--harness`.
- **Why a KIND and not a flag on `house`:** `house` links the cell config-home to the
  operator's real one by symlink and keeps the real `HOME` for the harness (`check_house`,
  `tools/cellctl/cellctl:921-939`; docs §*Why the session keeps the real `HOME`*). The
  scrubbed cell's whole point is the opposite: its own config home, its own harness login, its
  own roster scoped to one repo. A mode flag would leave one `check` function proving two
  contradictory contracts; a kind gives each its own `check_*` and leaves `house` byte-for-byte
  as it is (row 9).
- **The scrubbed environment is COMPOSED, not filtered.** The launch is `env -i` plus an
  explicit allowlist — the parent shell contributes nothing by default. Exported set, all
  values under the cell directory unless stated (this list IS the interface contract; the docs
  table and the dry-run plan both derive from it):
  `HOME=<cell>/home` · `ZDOTDIR=<cell>/home` · `SHELL` = the bash `cellctl` itself runs under
  (`$BASH`), never the operator's login shell · `PATH=<cell>/shim:<desk-tools bindir>:<dir of
  the harness binary, resolved once at launch>:/usr/bin:/bin:/usr/sbin:/sbin` (a cell.env
  `CELL_PATH` key overrides the trailing system part, never the shim prefix) · `TMPDIR=<cell>/tmp`
  (created 0700 if absent) · `KUBECONFIG=/dev/null` · `ASSAY_CONFIG_HOME=<cell>/home/.config/assay`
  · `GH_CONFIG_DIR=<cell>/home/.config/gh` · `GIT_CONFIG_GLOBAL=<cell>/home/.gitconfig` ·
  `GIT_CONFIG_NOSYSTEM=1` · `GIT_TERMINAL_PROMPT=0` · on codex `CODEX_HOME=<cell>/home/.codex`,
  on claude `CLAUDE_CONFIG_DIR=<cell>/home/.claude` (only the active harness's var is set) ·
  `DESK_LOOP=<role>` · `DESK_SESSION=<cell>-<role>-<UTC boot stamp>[-codex]` · `DESK_ROOTS`
  from `CELL_ROOTS` when set · `TERM` and `LANG` passed through from the parent (the only two
  value pass-throughs; a TUI harness needs them). The composed `PATH` has EXACTLY seven
  elements in order — `<cell>/shim`, the desk-tools bindir, `dirname` of the resolved harness
  binary, then `/usr/bin`, `/bin`, `/usr/sbin`, `/sbin`. The harness dir is the ONE
  parent-derived `PATH` element (resolving the harness is how it is found), so it is NAMED and
  PINNED rather than treated as a leak; every other element is fixed. No wholesale parent
  `PATH` is inherited, and nothing else crosses: no `SSH_AUTH_SOCK`, no `GH_TOKEN`, no
  `ANTHROPIC_*`, no `AWS_*`.
- **The App PEM reaches the tools through the cell, never the parent shell.** The desk tools
  resolve a role's key from the config home (`<config-home>/<role>-app.pem`, and the
  `<ROLE>_PEM=` entries of `apps.env`), so exporting `ASSAY_CONFIG_HOME` into the cell is the
  whole custody path. `check` proves every PEM the cell's `apps.env` names — and every
  `<role>-app.pem` present under the cell config home — is a **regular, non-symlink, mode 0600
  file** (`stat -f '%Lp'` on BSD, `stat -c '%a'` on GNU; the row tries both, exactly as the
  existing codex rows try two spellings). A house-style symlink into the operator's real
  config home is a MISS on this kind by design.
- **Config-home directory mode.** The directory the PEMs live in carries a custody claim of its
  own: `check` requires `<cell>/home/.config/assay` be mode **0700** (`stat -f '%Lp'` / `stat -c
  '%a'`, the same two-spelling row shape as the PEM row) — a group- or world-readable directory
  holding 0600 PEMs still exposes their names and mtimes, so the 0600 file row does not carry
  the directory claim. `new` creates it 0700; `check` fails closed on a widened mode.
- **Roster scope row:** the cell's `roster.env` must carry an `ASSAY_ALLOWED_REPOS=` line whose
  value is exactly `CELL_REPO_SLUG` (a new cell.env key, `<owner>/<repo>`, written by `new`
  from `--repo-slug`; `CELL_REPO` stays the checkout path). More than one entry, a different
  entry, or a bare `ASSAY_ALLOWED_REPOS=` is a MISS — the cell is one repo by construction.
- **Harness login row:** codex → the existing `codex login status` probe
  (`tools/cellctl/cellctl:1199-1200`) run with `CODEX_HOME=<cell>/home/.codex`; claude → a
  presence check of `<cell>/home/.claude` and `claude --version` under
  `CLAUDE_CONFIG_DIR=<cell>/home/.claude`. Neither row contacts a model.
- **`smoke` is a one-shot, tool-free, read-only readiness probe, separate from `check`.** Same
  composed environment as `desk`; the prompt is the literal `Reply with the single word READY
  and nothing else.`; the verb passes iff the harness exits 0 AND the last non-empty stdout
  line is exactly `READY`. codex arm: `codex exec --ephemeral --sandbox read-only
  --skip-git-repo-check -m <resolved model> "<prompt>"` (`--ephemeral`, `-s/--sandbox` and
  `--skip-git-repo-check` verified present in `codex exec --help`, codex-cli on the authoring
  host, 2026-09-16 — re-verify at pickup, the codex CLI's flags have moved across releases).
  claude arm: `claude -p --model <resolved model> "<prompt>"` (`-p/--print` verified in
  `claude --help`, 2026-09-16). `DRY_RUN=1 cellctl smoke` prints the plan and the argv and
  runs nothing. Live `smoke` is NEVER a Verify row: every row runs a stub harness.
  **Arm asymmetry, stated so it is a decision not a gap:** on codex, "tool-free, read-only" is
  enforced in the argv (`--sandbox read-only`); on claude it rests on `-p` (non-interactive,
  no session) plus the tool-free prompt, because `claude -p` has no argv sandbox equal to
  codex's. If the installed `claude` build offers a tool/permission-restriction flag, the
  implementer adds it and row 5 asserts it — verify at pickup rather than assuming one exists.
  Both arms print their resolved model before first contact so the plan is auditable; neither
  contacts a model on any Verify row.
- **`status <cell>`** prints exactly one of `running <session-name>` / `stopped` (plus
  `stale-lock <pid>` when a lock dir names a dead pid — reported, and cleared only by
  `down`), exit 0 in every case that is not a load error — it is a read, not a check.
- **Session lock, two independent layers.** (1) The tmux session lives on a **private socket**
  `tmux -S <cell>/run/tmux.sock` with one session named `<cell>-cell`; `new-session` on an
  existing name refuses, so a second cockpit cannot attach as a second owner. (2) A lock
  directory `<cell>/run/lock.d` taken by atomic `mkdir` and holding `pid` — taken by `desk`
  before the exec, held for the life of the harness process, released by `down`; a second
  `desk` while the pid is alive is refused with exit 4 and the message `cell <cell> is already
  running (pid <n>, session <name>); cellctl down <cell> to release`. `flock(1)` is NOT used:
  the CLI is absent from macOS by default (verified 2026-09-16), and a mkdir lock needs no
  fork to another language. `up` attaches when stdin is a tty, otherwise prints the attach
  line (`--no-attach` keeps its meaning). `down` kills the private-socket session and clears
  the lock; the workspace under `<cell>/worktrees/` is kept.
- **Launch shape on codex** is the existing arm's (`tools/cellctl/cellctl:1394`), sandbox
  included — this brief changes the ENVIRONMENT the exec runs in, not the argv; the bridge's
  `--sandbox workspace-write --ask-for-approval on-request --no-alt-screen` shape is recorded
  in #1193 item 6 and is NOT adopted here (docs §*The codex arm* explains why
  `danger-full-access` is the worktree-creation precondition). `up <cell> --model <m>` is
  accepted only when `status` is `stopped`; otherwise refused with the running session named.
- **Existing DRY_RUN line stays.** `cmd_desk` already prints one `[dry-run] cell=… kind=…`
  line (`tools/cellctl/cellctl:1311-1315`). The scrubbed arm ADDS, after it, the **plan**: one
  `[plan] env KEY=VALUE` line per exported variable, KEYs sorted, then one `[plan] argv <shell-
  quoted argv>` line, then `[plan] cwd <path>`, then `[plan] lock <lock dir>`. This grammar is
  what brief 10's parity harness diffs, so it is part of this brief's contract, not a
  courtesy.
- **No fork to another language.** The implementation stays bash + the tools already required
  (`git`, `tmux`, `curl`, `openssl`, `python3` per docs §*Install*); it adds no Python and no
  new binary. `python3` is already a stated dependency and may be used exactly where the
  existing script uses it (`plugin_enabled`), nowhere new.
- **Verification is offline by construction:** every row runs against a stub `codex` / `claude`
  / `tmux` / desk-verb set on a private `PATH` and a temp `CELLS_ROOT`, the shape
  `tools/cellctl/tests/house-cell.test.sh` and `harness.test.sh` already use. A row that would
  need a live model is not a row.

single-point-of-failure: the `env -i` allowlist in the scrubbed arm of `cmd_desk` — behind it: the
stub-harness test records the FULL environment it received and row 3 asserts the composed `PATH` is
exactly the seven named elements and every parent-shell canary (env var AND `PATH` dir) is absent
(fails in a different component — the recorded env file — for a different reason than the allowlist
itself), and `check`'s PEM/roster/config-home-mode rows fail closed on a cell whose custody was
wired house-style even when the launch env is right.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- Never run `smoke` against a live harness in a test or a Verify row; stubs only.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `load_cell`: add `scrubbed)` to the kind switch — `DESKD=0` forced (like `container`),
   `CELL_REPO` required and a git checkout, `CELL_REPO_SLUG` required (`<owner>/<repo>`),
   `CELL_ROOTS` optional. Update the `die` text to list `k8s|house|container|scrubbed`.
2. `cmd_new --kind scrubbed --repo <checkout> --repo-slug <owner/repo> [--roots …]`: write
   `cell.env` (`CELL_KIND=scrubbed`, `CELL_REPO`, `CELL_REPO_SLUG`, `CELL_HARNESS`, model
   pins, `ROLES`), create `home/.config/assay/` as a REAL directory (never a symlink), write a
   `roster.env` skeleton carrying `ASSAY_ALLOWED_REPOS=<slug>`, create `home/.config/gh/`,
   `home/.gitconfig` (empty), `tmp/` 0700, `run/`. Print the hand steps (copy the role PEMs in
   as regular 0600 files; log the harness in under the cell home) — nothing is copied from the
   operator's config home. Refuse an existing cell of the same name.
3. `check_scrubbed`: the rows in facts — config home is a real directory (not a symlink) AND
   mode 0700; every PEM regular/non-symlink/0600; `ASSAY_ALLOWED_REPOS` exactly the slug; harness login under
   the cell home; roster parses under the cell home (`deskroster repos --scope scan`, as house
   does); `CELL_ROOTS` rows when set; desk verbs installed; `tmux` on PATH; lock/run dir
   writable. Wire it in `cmd_check`'s kind case.
4. `cmd_desk` scrubbed arm: compose the environment per facts, take the lock, print the
   `[plan]` lines under `DRY_RUN=1` (after the existing `[dry-run]` line) and return; live,
   `exec env -i <allowlist> <harness argv>` inside the private-socket tmux session.
5. `cmd_smoke <cell> [--harness <h>] [--model <m>]`: same composition, the one-shot probe per
   facts; `READY` → exit 0 and print `READY`; anything else → exit 1 and print the harness's
   last line prefixed `smoke: not ready:`; `DRY_RUN=1` prints the plan only.
6. `cmd_status <cell>`: `running <session>` / `stopped` / `stale-lock <pid>`, exit 0.
7. `cmd_down` scrubbed arm: kill the private-socket session, clear the lock, keep worktrees.
   `cmd_up` scrubbed arm: refuse when `status` is not `stopped`; attach on a tty.
8. Add `smoke` and `status` to the dispatch table and the header usage block.
9. `tools/cellctl/tests/scrubbed-cell.test.sh` (planned) with `--case` selection; cases: `new`,
   `check-pass`, `check-pem`, `check-roster`, `check-home-mode`, `env-scrub`, `plan-grammar`,
   `smoke-ready`, `smoke-not-ready`, `lock`, `status`, `down`, `legacy-kinds`. Each case prints `ok`/`FAIL`
   rows in the existing suites' format and exits non-zero on any FAIL.
10. `docs/cellctl.md`: the new section, the env table (one row per exported var, generated
    from the same list the code holds — state the list ONCE in the script as a variable the
    docs row and the plan both read), the two verbs, the stricter `check` rows.
11. Changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `shellcheck tools/cellctl/testdata/cellctl-shell-oracle.sh` | exit 0 (red on the merge-base is not expected here — this row guards the edit) | check:ci |
| 2 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pass` | exit 0; a scrubbed fixture cell with regular 0600 PEMs, an exact `ASSAY_ALLOWED_REPOS`, a stub `codex` whose `login status` exits 0, passes every row. Red on the merge-base: the script does not exist (exit 127) and `cellctl check` on the fixture dies `CELL_KIND=scrubbed is not a known kind` (exit 3) | check:ci |
| 3 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case env-scrub` | exit 0; the case exports `GH_TOKEN=canary-parent`, `SSH_AUTH_SOCK=/nonexistent`, `ANTHROPIC_API_KEY=canary` AND a canary directory on the parent `PATH`, launches `desk` against a stub harness that dumps its environment, and asserts: (i) none of the three env canaries is present; (ii) every allowlisted KEY is present with its cell-relative value; (iii) the composed `PATH` equals EXACTLY `<cell>/shim`, the desk-tools bindir, `dirname` of the resolved stub harness, `/usr/bin`, `/bin`, `/usr/sbin`, `/sbin` — in that order and with no other element, so the harness dir is the one permitted parent-derived element and the injected canary dir does NOT appear. A leak of any additional parent-`PATH` element is still red. (the flow: parent shell → `cellctl desk` → composed env → what the harness process actually receives). Red on the merge-base: exit 127 | check:ci +mutation +flow |
| 4 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case plan-grammar` | exit 0; `DRY_RUN=1 cellctl desk <fixture> the-desk` prints the existing `[dry-run]` line THEN `[plan] env` lines in sorted KEY order, then `[plan] argv`, `[plan] cwd`, `[plan] lock`; the set of `[plan] env` KEYs equals the allowlist variable in the script (dereferencing: the plan is generated from the list, so a KEY added to one and not the other fails here). Red on the merge-base: exit 127 | check:ci +dereference |
| 5 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-ready` | exit 0; a stub harness printing `READY` makes `cellctl smoke` exit 0 and print `READY`; the stub records it was invoked with `exec --ephemeral --sandbox read-only` (codex) / `-p` (claude) and under `HOME=<cell>/home`. Red on the merge-base: exit 127 | check:ci |
| 6 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-not-ready` | exit 0; a stub printing `READY` but exiting 2, and a stub exiting 0 printing `I cannot`, each make `smoke` exit 1 with `smoke: not ready:` on stdout — the verb cannot pass on a wrong-but-well-formed answer | check:ci +mutation |
| 7 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case lock` | exit 0; with a lock dir holding a live pid, a second `desk` exits 4 with `is already running (pid`; with a dead pid, `status` prints `stale-lock` and `down` clears it; after `down`, `desk` proceeds | check:ci +mutation |
| 8 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pem` | exit 0; a PEM that is a symlink → `MISS` row naming `symlink`; a regular 0644 PEM → `MISS` row naming `0600`; a regular 0600 PEM → `ok`; `check` exits 1 on either MISS | check:ci +mutation |
| 9 | `bash tools/cellctl/tests/house-cell.test.sh` | exit 0 — the house kind is untouched (neighbour row; this suite passes on the merge-base too, so it is the regression guard, not the feature proof) | check:ci +neighbour |
| 10 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-roster` | exit 0; `ASSAY_ALLOWED_REPOS=example-org/example-repo,example-org/other` → `MISS`; `ASSAY_ALLOWED_REPOS=` → `MISS`; `ASSAY_ALLOWED_REPOS=example-org/example-repo` on a cell with that slug → `ok` | check:ci +mutation |
| 11 | `d=$(mktemp -d); mkdir -p "$d/x"; printf 'CELL_KIND=scrubbed\nCELL_REPO=%s\nCELL_REPO_SLUG=example-org/example-repo\nCELL_HARNESS=codex\n' "$PWD" > "$d/x/cell.env"; b="$(mktemp -d)/cellctl"; (cd tools/desk && go build -o "$b" ./cmd/cellctl); CELLS_ROOT="$d" "$b" check x; test $? -eq 1` | exit 0 — a bare scrubbed cell.env with an empty home is LOADED (no exit-3 die) and `check` reports MISS rows and exits 1. Red on a launcher without the `scrubbed` kind: `check` exits 3 (`is not a known kind`), so the `test` fails. Red on the merge-base: `tools/desk/cmd/cellctl` does not exist yet, so `go build` fails, running `"$b"` exits 127 and the `test` fails | check +dereference |
| 12 | `for k in HOME ZDOTDIR PATH TMPDIR KUBECONFIG ASSAY_CONFIG_HOME GH_CONFIG_DIR GIT_CONFIG_GLOBAL GIT_CONFIG_NOSYSTEM GIT_TERMINAL_PROMPT CODEX_HOME CLAUDE_CONFIG_DIR DESK_LOOP DESK_SESSION DESK_ROOTS; do grep -qF "\| $k \|" docs/cellctl.md \|\| { echo "missing env row: $k"; exit 1; }; done` | exit 0 — the docs env table (first column the bare variable name) carries a row per exported variable. Red on the merge-base: exits 1 at `ZDOTDIR` | check |
| 13 | `grep -c '^## Scrubbed cells' docs/cellctl.md` | exit 0; prints `1` | check |
| 14 | `statusgen --consumers --root . --base $(git merge-base origin/main HEAD)` | exit 0 — every `consumers:` routing above is corroborated or deferred by the diff | check:ci |
| 15 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-home-mode` | exit 0; a cell whose `home/.config/assay` is mode 0755 → `check` prints a `MISS` naming `0700` and exits 1; the same cell at 0700 → `ok` — the directory holding the PEMs carries its own custody row, not only the 0600 file row. Red on the merge-base: `--case check-home-mode` is unknown to the suite (exit 2) | check:ci +mutation |

## Definition of Done
- Verify rows green, recorded in Evidence by a non-implementer.
- A registration written for the retired bridge kind still refuses (row 11's sibling: a
  `CELL_KIND=local` cell.env exits 3 on this brief's merge — brief 11 owns the migration
  message; this brief must not silently accept `local`).
- `house`, `k8s`, `container` behaviour byte-identical (rows 9 and the existing suites
  `harness.test.sh`, `container-cell.test.sh`, `cockpit.test.sh` still exit 0).
- The plan grammar (`[plan] env|argv|cwd|lock`) is documented in `docs/cellctl.md` — brief 10
  depends on it.
- `docs/cellctl.md` regenerated with the new section, env table and verb rows (docs-regen item).

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->
### Non-implementer verifier run — VERIFY: FAIL (row 1 only: shellcheck regression introduced by this item's own merged commit; rows 2-15 + full suite green) — verify-desk-dispatch-20260920T0246Z (verify-desk dispatch), @ merged main `e4109205`, 2026-09-19/20

Own temp worktree off origin/main, offline envelope, not the implementer. Implementation commit in scope: 58a019271 (PR #1263). Filed: #1355 (the row-1 bug).

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | shellcheck tools/cellctl/cellctl | exit 0 | exit 1 — 7 SC2086 (info) findings, lines 726, 729, 768, 995, 996, 1801, 2188 (unquoted $KIND_VALUES/$COCKPIT_VALUES/$HARNESS_VALUES in value_in calls); parent 58a019271^ shellchecks clean (exit 0), so the findings were introduced by this item's own commit. **FAIL — filed #1355** | 2026-09-19 | verify-desk-dispatch-20260920T0246Z |
| 2 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pass | exit 0 | exit 0 — 8 ok rows (PEM row, roster ASSAY_ALLOWED_REPOS row, harness login under cell home, config home real-dir + mode 0700) | 2026-09-19 | verify-desk-dispatch-20260920T0246Z |
| 3 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case env-scrub | exit 0 | exit 0 — 12 ok rows: no GH_TOKEN / SSH_AUTH_SOCK / ANTHROPIC_API_KEY canary reaches the harness; allowlist keys present with cell-relative values; composed PATH is EXACTLY the 7 named elements in order — no canary dir | 2026-09-19 | verify-desk-dispatch-20260920T0246Z |
| 4 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case plan-grammar | exit 0 | exit 0 — 6 ok rows: existing [dry-run] line first, [plan] env KEY-sorted, then argv, cwd, lock; KEY set dereferences SCRUBBED_ENV_KEYS minus the inactive harness var | 2026-09-19 | verify-desk-dispatch-20260920T0246Z |
| 5 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-ready | exit 0 | exit 0 — 8 ok rows, both arms: claude invoked with -p, codex invoked with exec --ephemeral --sandbox read-only, each under the cell HOME; prints READY | 2026-09-19 | verify-desk-dispatch-20260920T0246Z |
| 6 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-not-ready | exit 0 | exit 0 — 4 ok rows: READY-but-exit-2 → exit 1 with "smoke: not ready:"; wrong-but-well-formed answer → exit 1, names what it said | 2026-09-19 | verify-desk-dispatch-20260920T0246Z |
| 7 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case lock | exit 0 | exit 0 — 6 ok rows: second desk while pid alive exits 4 naming "cellctl down"; dead pid → status prints stale-lock; down clears; after down, desk proceeds | 2026-09-19 | verify-desk-dispatch-20260920T0246Z |
| 8 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pem | exit 0 | exit 0 — 5 ok rows: symlink PEM → MISS naming symlink, exit 1; regular 0644 → MISS naming 0600, exit 1; regular 0600 → ok | 2026-09-19 | verify-desk-dispatch-20260920T0246Z |
| 9 | bash tools/cellctl/tests/house-cell.test.sh | exit 0 | exit 0 — 53 ok, 0 FAIL (house kind untouched) | 2026-09-19 | verify-desk-dispatch-20260920T0246Z |
| 10 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-roster | exit 0 | exit 0 — 3 ok rows: two-slug value MISS, empty value MISS, exact slug ok | 2026-09-19 | verify-desk-dispatch-20260920T0246Z |
| 11 | bare cell.env in a temp CELLS_ROOT: cellctl check x; test $? -eq 1 | exit 0 (overall) | exit 0 overall — check loaded kind=scrubbed (no exit-3 die), printed MISS rows and exited 1; find stderr noise on the absent .config dir is cosmetic | 2026-09-19 | verify-desk-dispatch-20260920T0246Z |
| 12 | all 15 SCRUBBED_ENV_KEYS names present in the docs env table (grep per key) | exit 0 | exit 0 — all 15 variable names found in the docs env table; no missing env row | 2026-09-19 | verify-desk-dispatch-20260920T0246Z |
| 13 | grep -c '^## Scrubbed cells' docs/cellctl.md | exit 0; prints 1 | exit 0, printed 1 | 2026-09-19 | verify-desk-dispatch-20260920T0246Z |
| 14 | statusgen --consumers --root . --base $(git merge-base origin/main HEAD) | exit 0 | exit 0 — no brief files in the diff (post-merge base == HEAD, empty diff, trivially green); a wider corroboration run at base 58a019271^ exits 1 but judges other briefs' claims in that window, not this item's | 2026-09-19 | verify-desk-dispatch-20260920T0246Z |
| 15 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-home-mode | exit 0 | exit 0 — 3 ok rows: 0755 config home → MISS naming 0700, exit 1; 0700 → ok | 2026-09-19 | verify-desk-dispatch-20260920T0246Z |

Supplementary (DoD corroboration): full suite scrubbed-cell.test.sh (all 13 cases incl. new, status, down, legacy-kinds) exit 0, 77 ok / 0 FAIL — including a retired CELL_KIND=local registration still refuses (exit 3) and k8s default untouched; neighbour suites harness.test.sh, container-cell.test.sh, cockpit.test.sh each exit 0 (byte-identical-behaviour clause).

RISK-BEARING VALUE (brief risk all-no, gate model, diff touches no risk-classed path — the fail-safe trigger does not strictly fire; enumerated anyway):
- RISK-VALUE: DERIVED — pem-mode = 0600 @ tools/cellctl/cellctl:1312 — the repo's standing custody convention for key/token files (same 0600 at cellctl:1417, :2808-2812); check fails closed on widening (row 8 green).
- RISK-VALUE: DERIVED — config-home-mode = 0700 @ tools/cellctl/cellctl:1288 (created 0700 at cellctl:2941) — owner-only traversal for the directory holding the 0600 PEMs; check fails closed (row 15 green).
- RISK-VALUE: DERIVED — scrubbed-path-tail = /usr/bin:/bin:/usr/sbin:/sbin @ tools/cellctl/cellctl:1670, full composed order at cellctl:1674 — fixed system tail (never wholesale parent PATH) behind the shim prefix and the one named parent-derived element; row 3 proves the exact 7-element order and canary-dir absence.
- Remaining literals (SCRUBBED_ENV_KEYS 19-key allowlist @ cellctl:1616; lock-refusal exit 4 + message @ cellctl:1711-1712; unknown-kind exit 3) are reversible operational knobs — rows 3/4/7/11 pin each; rank last, no derivation owed.

VERIFY: FAIL — Verify row 1 only (shellcheck exit 1, seven SC2086 findings, NEW in merged commit 58a019271; filed #1355). The unquoted expansions are functionally intentional (value_in takes multiple words) but need disable directives or quoting to satisfy the row's exit-0 expectation. Class: lint regression introduced by the merged implementation. Rows 2-15 all green — the feature behaviour itself is verified sound. Per the fail rule the item does NOT advance: status stays implemented, no flip.

### Non-implementer verifier run — VERIFY: FAIL — 3/15 pass, 11 could-not-check, 1 fail (row 1 shellcheck exit 1; rows 2-10,14,15 are check:ci with the hermetic witness owed on darwin; rows 11-13 plain check pass) — 2026-09-23 claude-opus-4-8-verifier

Fresh classification pass, own temp worktree detached at merged origin/main, offline envelope (KUBECONFIG=/dev/null), not the implementer. Merged SHA 39866201. Row 1 was already FAIL on the 2026-09-20 pass (filed #1355); it remains red. The original item commit 58a019271 (PR #1263) introduced the `value_in $KIND_VALUES/$COCKPIT_VALUES/$HARNESS_VALUES` unquoted-expansion (SC2086) findings #1355 tracks; those exact lines were re-touched by #1308 keeping the same pattern, and the SC2030/SC2031 subshell findings on lines 925/1320 arrived via the #1388 merge (git blame at 39866201) — none fixed the row-1 defect.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | shellcheck tools/cellctl/cellctl | exit 0 | FAIL — exit 1; 7 SC2086 (info) unquoted-expansion findings on value_in $KIND_VALUES / $COCKPIT_VALUES / $HARNESS_VALUES (lines 993, 996, 1044, 1271, 1272, 2134, 2608) plus 4 SC2030/SC2031 subshell findings (lines 925, 1320). Same defect class as #1355 (originally exit-1 on this item's commit 58a019271); still red on merged main 39866201 | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pass | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; 8 ok / 0 FAIL (PEM row, roster ASSAY_ALLOWED_REPOS row, harness login under cell home, config home real-dir + mode 0700) | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case env-scrub | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; 13 ok / 0 FAIL: no GH_TOKEN / SSH_AUTH_SOCK / ANTHROPIC_API_KEY canary reaches the harness; allowlist keys present with cell-relative values; composed PATH is EXACTLY the 7 named elements in order, no canary dir | 2026-09-23 | claude-opus-4-8-verifier |
| 4 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case plan-grammar | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; 6 ok / 0 FAIL: existing dry-run line first, then plan env KEY-sorted, argv, cwd, lock; env KEY set dereferences the allowlist variable minus the inactive harness var | 2026-09-23 | claude-opus-4-8-verifier |
| 5 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-ready | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; 8 ok / 0 FAIL: codex arm invoked with exec --ephemeral --sandbox read-only, claude arm with -p, each under the cell HOME; prints READY | 2026-09-23 | claude-opus-4-8-verifier |
| 6 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-not-ready | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; 4 ok / 0 FAIL: READY-but-exit-2 and wrong-but-well-formed answer each give exit 1 with "smoke: not ready:" | 2026-09-23 | claude-opus-4-8-verifier |
| 7 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case lock | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; 6 ok / 0 FAIL: second desk while pid alive exits 4 naming "cellctl down"; dead pid → status prints stale-lock; down clears; after down, desk proceeds | 2026-09-23 | claude-opus-4-8-verifier |
| 8 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pem | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; 5 ok / 0 FAIL: symlink PEM → MISS naming symlink, exit 1; regular 0644 → MISS naming 0600, exit 1; regular 0600 → ok | 2026-09-23 | claude-opus-4-8-verifier |
| 9 | bash tools/cellctl/tests/house-cell.test.sh | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; 53 ok / 0 FAIL (house kind untouched, regression guard) | 2026-09-23 | claude-opus-4-8-verifier |
| 10 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-roster | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; 3 ok / 0 FAIL: two-slug value MISS, empty value MISS, exact slug ok | 2026-09-23 | claude-opus-4-8-verifier |
| 11 | d=$(mktemp -d); mkdir -p "$d/x"; printf 'CELL_KIND=scrubbed\nCELL_REPO=%s\nCELL_REPO_SLUG=example-org/example-repo\nCELL_HARNESS=codex\n' "$PWD" > "$d/x/cell.env"; CELLS_ROOT="$d" bash tools/cellctl/cellctl check x; test $? -eq 1 | exit 0 (overall) | PASS — exit 0 overall; check loaded kind=scrubbed (no exit-3 die), printed MISS rows and exited 1 | 2026-09-23 | claude-opus-4-8-verifier |
| 12 | for k in HOME ZDOTDIR PATH TMPDIR KUBECONFIG ASSAY_CONFIG_HOME GH_CONFIG_DIR GIT_CONFIG_GLOBAL GIT_CONFIG_NOSYSTEM GIT_TERMINAL_PROMPT CODEX_HOME CLAUDE_CONFIG_DIR DESK_LOOP DESK_SESSION DESK_ROOTS; do grep -qF "\| $k \|" docs/cellctl.md \|\| { echo "missing env row: $k"; exit 1; }; done | exit 0 | PASS — exit 0; no "missing env row" line printed; all 15 variable-name rows present in the docs env table | 2026-09-23 | claude-opus-4-8-verifier |
| 13 | grep -c '^## Scrubbed cells' docs/cellctl.md | exit 0; prints 1 | PASS — exit 0, printed 1 | 2026-09-23 | claude-opus-4-8-verifier |
| 14 | statusgen --consumers --root . --base $(git merge-base origin/main HEAD) | exit 0 | COULD-NOT-CHECK — merged tree; base == HEAD so the diff is empty and no consumers: routing is corroborated. Re-run at 39866201: exit 0, "consumers: no brief files in the diff against 39866201ce48... — nothing to corroborate" (nothing corroborated, nothing disproved) | 2026-09-23 | claude-opus-4-8-verifier |
| 15 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-home-mode | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; 3 ok / 0 FAIL: 0755 config home → MISS naming 0700, exit 1; 0700 → ok | 2026-09-23 | claude-opus-4-8-verifier |

Execution witness (statusgen verifyrun, left uncommitted in the brief file for the desk): rows 11, 12, 13 pass; every check:ci row (1-10, 14, 15) is could-not-run on this host because the hermetic network-off sandbox needs Linux `unshare --net` and this host is darwin — recorded verbatim as could-not-check for the witness lane, neither pass nor fail. Row 1's FAIL above is my own direct manual run of shellcheck, a static linter that needs no network, so the offline envelope does not limit it.

RISK-BEARING VALUE (brief risk all-no, gate model, merged-main diff empty — the fail-safe trigger does not strictly fire; enumerated over the brief's own pinned literals anyway):
- RISK-VALUE: DERIVED — pem-mode = 0600 @ tools/cellctl/cellctl:1639 — repo's standing custody convention for key/PEM files; check_scrubbed fails closed on any widening (row 8 direct run: MISS on widening).
- RISK-VALUE: DERIVED — config-home-mode = 0700 @ tools/cellctl/cellctl:1615 — owner-only traversal for the directory holding the 0600 PEMs (a group/world-readable dir leaks PEM names and mtimes); check fails closed (row 15 direct run: MISS at 0755).
- RISK-VALUE: DERIVED — scrubbed-path-tail = /usr/bin:/bin:/usr/sbin:/sbin @ tools/cellctl/cellctl:1998 — the fixed system tail behind the shim prefix and the one named parent-derived element (the resolved harness dir); never a wholesale parent PATH; row 3's direct run shows the exact 7-element order and canary-dir absence.
- Remaining literals rank last as reversible operational knobs, no derivation owed: SCRUBBED_ENV_KEYS allowlist @ tools/cellctl/cellctl:1943 (rows 3/4 pin it), lock-refusal exit 4 + message @ tools/cellctl/cellctl:2039 (row 7), unknown-kind exit 3 @ tools/cellctl/cellctl:325 (row 11).

VERIFY: FAIL — 3/15 pass, 11 could-not-check, 1 fail. Row 1 (shellcheck exit 1, a static linter needing no network) is the fail; rows 2-10 and 15 are check:ci COULD-NOT-CHECK (hermetic witness owed on darwin, #1491), row 14 is the merged-tree consumers could-not-check; rows 11-13 (plain `check`) pass on the direct run. Per the fail rule the item does NOT advance: status stays implemented, no flip.

Row 1 (shellcheck on the cellctl script) still fails: the SC2086 findings tracked at #1355 plus the SC2030/SC2031 subshell findings. The check:ci rows (2-10, 14, 15) are could-not-check pending the Linux hermetic witness (#1491); rows 11-13 (plain `check`) pass on the direct run.

### Non-implementer verifier re-run — VERIFY: BLOCKED — 0 fail; row 1 now green; check:ci hermetic witness still owed on darwin — 2026-10-01 claude-opus-5-5[1m]

What moved since the last run (2026-09-23 @ 39866201): #1816 (merged 2026-09-30) made the bash oracle shellcheck-clean (#1355 closed), so row 1 has gone from FAIL to exit 0. #1739 moved the oracle to tools/cellctl/testdata/cellctl-shell-oracle.sh. #1970 (f60962e1a) re-pointed this brief's Context and `consumers:` at the Go launcher in tools/desk/cmd/cellctl, and row 1 now names the oracle path, which exists on main. Row 11 now builds and runs the Go port. Rows 2-10, 14 and 15 are unchanged. They are check:ci rows, and on this darwin host the hermetic network-off witness still cannot run them (#1491 open).

This run used a temporary detached worktree at merged origin/main 01015a725b5d6b8c78ce76a1f43b8966d2373c54, with KUBECONFIG=/dev/null and no network-dependent row. The verifier did not implement this item. Toolchain: GNU bash 5.3.20, shellcheck 0.11.0, go1.27.1 darwin/arm64, statusgen v1.0.29.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | shellcheck tools/cellctl/testdata/cellctl-shell-oracle.sh | exit 0 | Verify row 1. exit 0 with no findings. The path the row names exists on main. The direct run passes. The hermetic check:ci witness could-not-check on darwin (witness row 1 below) | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pass | exit 0; fully provisioned fixture passes every row | Verify row 2. exit 0: 8 ok, 0 FAIL (PEM row, ASSAY_ALLOWED_REPOS row, codex login under the cell home, config home is a real directory with mode 0700, generic codex block). The direct run passes. Hermetic witness could-not-check (darwin) | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case env-scrub | exit 0; no env canaries; allowlist keys cell-relative; composed PATH exactly 7 elements, no canary dir | Verify row 3. exit 0: 13 ok, 0 FAIL. The GH_TOKEN, SSH_AUTH_SOCK and ANTHROPIC_API_KEY canaries are all absent. HOME, TMPDIR, KUBECONFIG=/dev/null, GIT_TERMINAL_PROMPT=0 and CLAUDE_CONFIG_DIR are cell-relative, CODEX_HOME is absent on the claude arm, and DESK_ROOTS comes from CELL_ROOTS. Last line: "composed PATH is EXACTLY the 7 named elements, in order — no canary dir". Hermetic witness could-not-check (darwin) | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case plan-grammar | exit 0; dry-run line, then sorted plan env, argv, cwd, lock; env KEY set equals the allowlist variable | Verify row 4. exit 0: 6 ok, 0 FAIL. The dry-run line comes first, the plan env lines are KEY-sorted, and argv, cwd and lock follow in that order. The env KEY set equals the allowlist variable minus the inactive harness var. Hermetic witness could-not-check (darwin) | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-ready | exit 0; READY stub makes smoke exit 0; codex argv exec --ephemeral --sandbox read-only, claude -p; HOME is the cell home | Verify row 5. exit 0: 8 ok, 0 FAIL across both arms. claude was invoked with -p and codex with exec --ephemeral --sandbox read-only, each under the cell home, and each printed READY. Hermetic witness could-not-check (darwin) | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-not-ready | exit 0; READY-but-exit-2 and exit-0 "I cannot" each give smoke exit 1 with "smoke: not ready:" | Verify row 6. exit 0: 4 ok, 0 FAIL. READY with a nonzero exit gives exit 1 plus the "smoke: not ready:" line. A wrong but well-formed answer gives exit 1 and names what it said. Hermetic witness could-not-check (darwin) | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case lock | exit 0; live-pid second desk exits 4 "is already running (pid"; dead pid shows stale-lock, down clears, desk proceeds | Verify row 7. exit 0: 6 ok, 0 FAIL. While the pid is alive, a second desk exits 4 with the exact refusal, which names cellctl down. A dead pid shows stale-lock in status, down clears it, and desk proceeds after down. Hermetic witness could-not-check (darwin) | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pem | exit 0; symlink PEM MISS naming symlink; 0644 MISS naming 0600; 0600 ok; check exits 1 on either MISS | Verify row 8. exit 0: 5 ok, 0 FAIL. A symlink PEM gives a MISS naming symlink and exit 1. A 0644 PEM gives a MISS naming 0600 and exit 1. A regular 0600 PEM is ok. Hermetic witness could-not-check (darwin) | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 9 | bash tools/cellctl/tests/house-cell.test.sh | exit 0 (neighbour regression guard) | Verify row 9. exit 0: 54 ok, 0 FAIL, last line "house-cell.test.sh: OK". Hermetic witness could-not-check (darwin) | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 10 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-roster | exit 0; two-slug MISS; empty MISS; exact slug ok | Verify row 10. exit 0: 3 ok, 0 FAIL. Multiple entries give a MISS, an empty value gives a MISS, and the exact slug is ok. Hermetic witness could-not-check (darwin) | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 11 | d=$(mktemp -d); mkdir -p "$d/x"; printf 'CELL_KIND=scrubbed\nCELL_REPO=%s\nCELL_REPO_SLUG=example-org/example-repo\nCELL_HARNESS=codex\n' "$PWD" > "$d/x/cell.env"; b="$(mktemp -d)/cellctl"; (cd tools/desk && go build -o "$b" ./cmd/cellctl); CELLS_ROOT="$d" "$b" check x; test $? -eq 1 | exit 0; bare scrubbed cell.env loads and check exits 1 with MISS rows | Verify row 11. Overall exit 0. The Go build succeeded. check printed "[check] cell=x ... kind=scrubbed", so the cell loaded with no exit-3 die. It reported MISS rows (config home is a REAL directory, config home mode 0700, roster ASSAY_ALLOWED_REPOS exactly the slug, harness login under the cell home) and ended "[check] fix the MISS rows first" with exit 1. Witness row 11: pass | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 12 | for k in HOME ZDOTDIR PATH TMPDIR KUBECONFIG ASSAY_CONFIG_HOME GH_CONFIG_DIR GIT_CONFIG_GLOBAL GIT_CONFIG_NOSYSTEM GIT_TERMINAL_PROMPT CODEX_HOME CLAUDE_CONFIG_DIR DESK_LOOP DESK_SESSION DESK_ROOTS; do grep -qF "\| $k \|" docs/cellctl.md \|\| { echo "missing env row: $k"; exit 1; }; done | exit 0; a docs env-table row per exported var | Verify row 12. Two forms were run. (a) The raw markdown source of the cell, with its table-escape backslashes, pasted literally into bash -c gives "syntax error near unexpected token" and exit 2. (b) The table-rendered form, with plain pipes, exits 0 and prints no "missing env row" line. The rendered form is the authored command, so it is the observation counted. Witness row 12 (which unescapes the cell): pass | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 13 | grep -c '^## Scrubbed cells' docs/cellctl.md | exit 0; prints 1 | Verify row 13. exit 0, printed 1. Witness row 13: pass | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 14 | statusgen --consumers --root "$(git rev-parse --show-toplevel)" --base $(git merge-base origin/main HEAD) | exit 0; every consumers routing corroborated or deferred | Verify row 14. The as-authored root of "." was replaced with the absolute toplevel because the bare-dot root is write-guarded. exit 0: "consumers: no brief files in the diff against 01015a725b5d... — nothing to corroborate". The merge-base equals HEAD on merged main, so this result is vacuous. A supporting non-vacuous run at --base f60962e1a^ also exits 0: the oracle routing is CORROBORATED, and scrubbed-cell.test.sh and docs/cellctl.md are UNCHECKED, meaning they were not edited in that window. Hermetic witness could-not-check (darwin) | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 15 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-home-mode | exit 0; 0755 config home MISS naming 0700, exit 1; 0700 ok | Verify row 15. exit 0: 3 ok, 0 FAIL. A 0755 config home gives a MISS naming 0700 and exit 1. A 0700 config home is ok. Hermetic witness could-not-check (darwin) | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

Supporting runs that discharge no Verify row and are listed for scope traceability. They correspond to Definition of Done clauses:
- The full scrubbed-cell.test.sh suite (all 13 cases) against the default bash oracle: exit 0, 78 ok, 0 FAIL.
- The same full suite with CELLCTL set to a fresh Go build of tools/desk/cmd/cellctl: exit 0, 78 ok, 0 FAIL. All nine per-case Verify rows also pass individually against the Go build.
- harness.test.sh, container-cell.test.sh and cockpit.test.sh each exit 0 with 0 FAIL (the DoD byte-identical clause).
- A CELL_KIND=local cell.env still refuses with exit 3 and "is not a known kind (k8s|house|container|scrubbed)" on both the oracle and the Go build (the DoD retired-bridge clause).

Scope finding: the brief's Context now says the Go launcher carries this behaviour. But rows 2-10 and 15 run scrubbed-cell.test.sh with CELLCTL unset, so as written they exercise the bash oracle, and only row 11 exercises the Go port. The Go-port runs above passed, but no Verify row pins them. Tracked on #1968.

Execution witness (statusgen verifyrun --dry-run, as emitted; it was not written back to the brief):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `shellcheck tools/cellctl/testdata/cellctl-shell-oracle.sh` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (on-behalf-of human:ian) (forge-identity) |
| 2 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pass` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (on-behalf-of human:ian) (forge-identity) |
| 3 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case env-scrub` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (on-behalf-of human:ian) (forge-identity) |
| 4 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case plan-grammar` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (on-behalf-of human:ian) (forge-identity) |
| 5 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-ready` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (on-behalf-of human:ian) (forge-identity) |
| 6 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-not-ready` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (on-behalf-of human:ian) (forge-identity) |
| 7 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case lock` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pem` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (on-behalf-of human:ian) (forge-identity) |
| 9 | `bash tools/cellctl/tests/house-cell.test.sh` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (on-behalf-of human:ian) (forge-identity) |
| 10 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-roster` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (on-behalf-of human:ian) (forge-identity) |
| 11 | `d=$(mktemp -d); mkdir -p "$d/x"; printf 'CELL_KIND=scrubbed\nCELL_REPO=%s\nCELL_REPO_SLUG=example-org/example-repo\nCELL_HARNESS=codex\n' "$PWD" > "$d/x/cell.env"; b="$(mktemp -d)/cellctl"; (cd tools/desk && go build -o "$b" ./cmd/cellctl); CELLS_ROOT="$d" "$b" check x; test $? -eq 1` | pass exit=0 | sha256:ea447d5d2f35 | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (on-behalf-of human:ian) (forge-identity) |
| 12 | `for k in HOME ZDOTDIR PATH TMPDIR KUBECONFIG ASSAY_CONFIG_HOME GH_CONFIG_DIR GIT_CONFIG_GLOBAL GIT_CONFIG_NOSYSTEM GIT_TERMINAL_PROMPT CODEX_HOME CLAUDE_CONFIG_DIR DESK_LOOP DESK_SESSION DESK_ROOTS; do grep -qF "\| $k \|" docs/cellctl.md \|\| { echo "missing env row: $k"; exit 1; }; done` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (on-behalf-of human:ian) (forge-identity) |
| 13 | `grep -c '^## Scrubbed cells' docs/cellctl.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (on-behalf-of human:ian) (forge-identity) |
| 14 | `statusgen --consumers --root . --base $(git merge-base origin/main HEAD)` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (on-behalf-of human:ian) (forge-identity) |
| 15 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-home-mode` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 01015a725b5d (on-behalf-of human:ian) (forge-identity) |

RISK-BEARING VALUE. The brief's risk is all no and it is not irreversible. The metadata is present, and the diff scope (cellctl launcher, oracle, tests, docs) is not a risk-classed path, so the fail-safe trigger does not strictly fire. The values were enumerated anyway, from the Deliverables and the merged code in both implementations:
- pem-mode = 0o600 @ tools/desk/cmd/cellctl/check.go:392 (oracle: tools/cellctl/testdata/cellctl-shell-oracle.sh:1712)
- config-home-mode = 0o700 @ tools/desk/cmd/cellctl/check.go:352 (oracle :1688). The config home is created 0700 at tools/desk/cmd/cellctl/new.go:505 and oracle :3504
- path-tail = "/usr/bin:/bin:/usr/sbin:/sbin" @ tools/desk/cmd/cellctl/env.go:120 (oracle :2071)
- env allowlist = a 19-key list @ tools/desk/cmd/cellctl/env.go:19 (oracle :2016)
- lock-refusal exit = 4 @ tools/desk/cmd/cellctl/lock.go:28 (oracle :2113)
- unknown-kind exit = 3 via die @ tools/desk/cmd/cellctl/cell.go:340 (oracle :343)
- smoke prompt = "Reply with the single word READY and nothing else." @ tools/desk/cmd/cellctl/smoke.go:50 (oracle :2591)

Ranking: every value here can be undone by an edit and a redeploy. The three custody values rank highest because a wrong value silently widens what the cell exposes. The exit codes, the prompt and the allowlist are operational contract and rank last.
- RISK-VALUE: DERIVED — pem-mode = 0o600 @ tools/desk/cmd/cellctl/check.go:392 — an App private key must be readable and writable by its owner only. 0600 is the narrowest mode that still lets the owning user's tools read the key, and it is the same floor ssh enforces on private keys. Any group or other bit exposes the key to other local accounts. check fails closed on a widened mode or a symlink (row 8).
- RISK-VALUE: DERIVED — config-home-mode = 0o700 @ tools/desk/cmd/cellctl/check.go:352 — owner-only list and traverse on the directory holding the PEMs. 0600 on the files does not hide their names or mtimes in a group- or world-listable directory, which is exactly the gap the brief's facts name. 0700 is the minimum the owner needs to use the directory (row 15).
- RISK-VALUE: DERIVED — path-tail = "/usr/bin:/bin:/usr/sbin:/sbin" @ tools/desk/cmd/cellctl/env.go:120 — the default tail is the four OS system directories, and the parent PATH is never inherited wholesale. Three entries stay operator-influenced. First, the desk-tools bindir is DESK_TOOLS_BIN (cell.go:259, default /opt/desk-tools/bin). It is read from an environment seeded from the launching process (cell.go:52-61, :282) with cell.env layered over it (cell.go:290), and it is PATH element 2. Second, the harness directory is resolved on the parent PATH (env.go:29-38). Both come before the tail (env.go:124-126), so a binary in either can shadow a system tool. Third, a CELL_PATH from cell.env or the parent environment replaces the tail outright (env.go:121-122). The Deliverables name the harness directory and a cell.env CELL_PATH but call every other element fixed, so the parent-environment reach of DESK_TOOLS_BIN and CELL_PATH goes beyond what they describe. That tension is a follow-up on #1968, not a verdict here. With the default tail this is the brief's seven-element contract. Row 3 proves the default order and that a parent-PATH canary directory is absent.

rows_passed=3 rows_total=15. Rows 11, 12 and 13 have both a hand run and a witness pass. The 12 check:ci rows (1-10, 14 and 15) each exited 0 on a direct run, but the hermetic network-off witness could-not-check them on darwin. No row failed.

RISK-VALUE: DERIVED — pem-mode = 0o600 @ tools/desk/cmd/cellctl/check.go:392; config-home-mode = 0o700 @ tools/desk/cmd/cellctl/check.go:352; path-tail default = "/usr/bin:/bin:/usr/sbin:/sbin" @ tools/desk/cmd/cellctl/env.go:120, overridable by CELL_PATH from cell.env or the parent environment (env.go:121-122) and preceded by the desk-tools bindir DESK_TOOLS_BIN (cell.go:259) and the harness directory (env.go:124-126) (reasons above)

VERIFY: BLOCKED

### Non-implementer verifier re-run — 2026-10-09 assay-verifier-app[bot] @ ca489e07187b (claude-opus-5-5) (on-behalf-of human:ian)

Third verify pass, on merged main ca489e07187b23ed1687b01795ea9fa7f92e39a9. The verifier did not implement this item. Toolchain: GNU bash 5.3.20, shellcheck 0.11.0, go1.27.1 darwin/arm64, statusgen v1.0.34.

What moved since the last run (2026-10-01 @ 01015a725b5d): docs/cellctl.md (+205 lines), and the Go launcher files check.go, cell.go and new.go under tools/desk/cmd/cellctl (about 100 changed lines each), env.go (+1) and the bash oracle (+2). tools/cellctl/tests/scrubbed-cell.test.sh and house-cell.test.sh did not change.

| Row | Command | Exit | Observed | Date | Runner |
|-----|---------|------|----------|------|--------|
| 1 | Verify row 1 as written (throwaway HOME, KUBECONFIG=/dev/null) | 0 | no output, no findings | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | Verify row 2 as written (throwaway HOME, KUBECONFIG=/dev/null) | 0 | 8 ok, 0 FAIL; `scrubbed-cell.test.sh: OK` | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | Verify row 3 as written (throwaway HOME, KUBECONFIG=/dev/null) | 0 | 13 ok, 0 FAIL; the last line reads that the composed PATH is exactly the 7 named elements, in order, with no canary dir. Expect clauses (i) and (iii) are asserted; clause (ii) is asserted for only some keys (finding 1) | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | Verify row 4 as written (throwaway HOME, KUBECONFIG=/dev/null) | 0 | 6 ok, 0 FAIL | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | Verify row 5 as written (throwaway HOME, KUBECONFIG=/dev/null) | 0 | 8 ok, 0 FAIL; the claude stub was called with -p and the codex stub with exec --ephemeral --sandbox read-only, each under the cell home | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | Verify row 6 as written (throwaway HOME, KUBECONFIG=/dev/null) | 0 | 4 ok, 0 FAIL | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | Verify row 7 as written (throwaway HOME, KUBECONFIG=/dev/null) | 0 | 6 ok, 0 FAIL | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | Verify row 8 as written (throwaway HOME, KUBECONFIG=/dev/null) | 0 | 5 ok, 0 FAIL | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | Verify row 9 as written (throwaway HOME, KUBECONFIG=/dev/null) | 0 | 54 ok, 0 FAIL; `house-cell.test.sh: OK` | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (claude-opus-5-5) (on-behalf-of human:ian) |
| 10 | Verify row 10 as written (throwaway HOME, KUBECONFIG=/dev/null) | 0 | 3 ok, 0 FAIL | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (claude-opus-5-5) (on-behalf-of human:ian) |
| 11 | Verify row 11 as written (throwaway HOME, KUBECONFIG=/dev/null) | 0 | the Go build succeeds; `check x` loads the cell as kind=scrubbed without the exit 3 refusal, prints MISS rows (the roster, the config home's 0700 mode, the exact-roster test and the harness login among them), and ends `[check] fix the MISS rows first` | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (claude-opus-5-5) (on-behalf-of human:ian) |
| 12 | Verify row 12 with the table's escaped pipes unescaped (throwaway HOME, KUBECONFIG=/dev/null) | 0 | no `missing env row` line. The cell text taken literally, escapes included, is a bash syntax error (exit 2) | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (claude-opus-5-5) (on-behalf-of human:ian) |
| 13 | Verify row 13 as written (throwaway HOME, KUBECONFIG=/dev/null) | 0 | prints `1` | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (claude-opus-5-5) (on-behalf-of human:ian) |
| 14 | Verify row 14 as written (throwaway HOME, KUBECONFIG=/dev/null) | 0 | prints that there are no brief files in the diff and nothing to corroborate. On merged main the merge-base equals HEAD, so the row passes without checking anything | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (claude-opus-5-5) (on-behalf-of human:ian) |
| 15 | Verify row 15 as written (throwaway HOME, KUBECONFIG=/dev/null) | 0 | 3 ok, 0 FAIL | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (claude-opus-5-5) (on-behalf-of human:ian) |

Mutations. Each was made to tools/cellctl/testdata/cellctl-shell-oracle.sh, the implementation these rows run, and reverted afterwards; the oracle was byte-identical to main at the end. Nine of ten turned their row red:
- Row 3: `env -i` replaced by `env` at :2151 gives exit 1 with 3 FAILED (all three canaries reach the harness). The parent PATH appended to the composed PATH at :2075 gives exit 1 with 1 FAILED.
- Row 3, **survived**: the ASSAY_CONFIG_HOME value at :2046 changed from the cell's config home to the operator's. Row 3 exits 0, row 4 exits 0 and the full scrubbed suite exits 0 (finding 1).
- Row 6: dropping the exit-code half of the READY test at :2611 gives exit 1 with 2 FAILED. Dropping the READY half gives exit 1 with 2 FAILED.
- Row 7: the live-pid test at :2113 forced false gives exit 1 with 2 FAILED (the second desk no longer exits 4).
- Row 8: the PEM mode 600 changed to 644 at :1714 gives exit 1 with 2 FAILED. The symlink test at :1709 forced false gives exit 1 with 1 FAILED.
- Row 10: the exact roster comparison at :1721 loosened to a prefix match gives exit 1 with 1 FAILED.
- Row 15: the config home mode 700 changed to 755 at :1690 gives exit 1 with 2 FAILED.

Findings. None of them is the reason for the verdict below. Finding 2 is partly tracked already: #2067 for the drift between the two implementations, #1273 for the suite not running in CI. Findings 1, 3 and 4 are not yet filed; the filing follows this Evidence.
1. **Row 3 asserts less than its Expect.** Clause (ii) says every allowlisted key is present with its cell-relative value. The case checks values for HOME, TMPDIR, KUBECONFIG, GIT_TERMINAL_PROMPT, CLAUDE_CONFIG_DIR, CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION, DESK_ROOTS and PATH only. It asserts no value for ASSAY_CONFIG_HOME, GH_CONFIG_DIR, GIT_CONFIG_GLOBAL, GIT_CONFIG_NOSYSTEM, ZDOTDIR, SHELL, DESK_LOOP, DESK_SESSION, TERM or LANG, which is why the mutation above survived. Row 4 compares the dry-run plan's key names with the allowlist, not their values.
   - No implementation defect was observed here: the dry-run plan from both the oracle and a fresh Go build shows ASSAY_CONFIG_HOME, GH_CONFIG_DIR, GIT_CONFIG_GLOBAL, HOME, TMPDIR and ZDOTDIR under the cell directory, and GIT_CONFIG_NOSYSTEM=1.
   - The live harness values of the unasserted keys were not checked.
2. **Rows 2 to 10 and 15 run the bash oracle, and the Go launcher no longer passes the same suite.** The suite defaults CELLCTL to the oracle (scrubbed-cell.test.sh:21); only row 11 builds the Go launcher that Context says carries the behaviour.
   - Supporting run, not a Verify row: the full scrubbed suite with CELLCTL set to a fresh Go build and CELLCTL_DESKWT=0 exits 1 with 76 ok and 2 FAIL, both in the check-pass case. On 2026-10-01 the same run exited 0 with 78 ok.
   - Cause: the Go check now probes `codex features list` (tools/desk/cmd/cellctl/check.go:482, changed by #2007), and the suite's codex stub answers only `config get`. The oracle still probes `codex config get features.multi_agent` (:1948).
   - The other Go cases pass under CELLCTL_DESKWT=0. No workflow under .github/workflows runs this suite.
3. **The suite calls the real deskwt when one is installed.** It puts its stub deskwt in the fixture's desk-tools directory, but both implementations resolve `deskwt role-init` from the parent PATH (tools/desk/cmd/cellctl/git.go:220-228; oracle :1857-1859).
   - On a host with the desk tools installed, every live `desk` case therefore calls the installed deskwt. On the verify host each such call was refused, and each refusal wrote a row to the operator's live desk audit log: about twenty rows over this run, from the row 3, 7 and 9 runs and the supporting suite runs. With the Go build the cases env-scrub, lock, status and down stop at that refusal (exit 3) unless CELLCTL_DESKWT=0 is set.
   - The oracle discards that output and falls back to `git worktree add`, so rows 3 and 7 stay green.
4. Context lists changelog/desk-containers-09-scrubbed-cell.md as planned. It does not exist on main, and nothing under changelog/ names this item. Context also still marks tools/cellctl/tests/scrubbed-cell.test.sh as planned, though it exists.

Not checked: the Definition of Done neighbour suites (harness.test.sh, container-cell.test.sh, cockpit.test.sh), which are not Verify rows; row 14 against a non-empty diff; the live harness values named in finding 1.

RISK-BEARING VALUE. The brief's risk answers are all no. Every value named in the 2026-10-01 block still holds on this main; several line numbers moved, and the current ones are:
- RISK-VALUE: DERIVED — pem-mode = 0o600 @ tools/desk/cmd/cellctl/check.go:425 (oracle :1714) — owner-only read and write is the narrowest mode that still lets the owning user's tools read an App private key. Check fails closed on a wider mode or a symlink (row 8).
- RISK-VALUE: DERIVED — config-home-mode = 0o700 @ tools/desk/cmd/cellctl/check.go:385 (oracle :1690) — owner-only list and traverse on the directory holding the keys, the minimum the owner needs to use it (row 15). The directory is still created 0755 and set to 0700 in a second step (new.go:470-474).
- RISK-VALUE: DERIVED — path-tail = "/usr/bin:/bin:/usr/sbin:/sbin" @ tools/desk/cmd/cellctl/env.go:120 (oracle :2073) — the four system directories; the parent PATH is not inherited (row 3). The CELL_PATH override (env.go:121-122) and the DESK_TOOLS_BIN default (cell.go:295) are unchanged in meaning.
- Unchanged in value: the 19-key env allowlist (env.go:19), lock refusal exit 4 (lock.go:28), unknown-kind exit 3 (cell.go:383) and the smoke prompt (smoke.go:50).

Execution witness, as `statusgen verifyrun --brief <this brief> --root .` wrote it (exit 2). Rows 11, 12 and 13 pass. The twelve check:ci rows are could-not-run, because the verify host is darwin and has no network-off sandbox (#1800):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `shellcheck tools/cellctl/testdata/cellctl-shell-oracle.sh` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (on-behalf-of human:ian) (forge-identity) |
| 2 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pass` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (on-behalf-of human:ian) (forge-identity) |
| 3 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case env-scrub` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (on-behalf-of human:ian) (forge-identity) |
| 4 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case plan-grammar` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (on-behalf-of human:ian) (forge-identity) |
| 5 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-ready` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (on-behalf-of human:ian) (forge-identity) |
| 6 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-not-ready` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (on-behalf-of human:ian) (forge-identity) |
| 7 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case lock` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pem` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (on-behalf-of human:ian) (forge-identity) |
| 9 | `bash tools/cellctl/tests/house-cell.test.sh` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (on-behalf-of human:ian) (forge-identity) |
| 10 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-roster` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (on-behalf-of human:ian) (forge-identity) |
| 11 | `d=$(mktemp -d); mkdir -p "$d/x"; printf 'CELL_KIND=scrubbed\nCELL_REPO=%s\nCELL_REPO_SLUG=example-org/example-repo\nCELL_HARNESS=codex\n' "$PWD" > "$d/x/cell.env"; b="$(mktemp -d)/cellctl"; (cd tools/desk && go build -o "$b" ./cmd/cellctl); CELLS_ROOT="$d" "$b" check x; test $? -eq 1` | pass exit=0 | sha256:09a640689e36 | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (on-behalf-of human:ian) (forge-identity) |
| 12 | `for k in HOME ZDOTDIR PATH TMPDIR KUBECONFIG ASSAY_CONFIG_HOME GH_CONFIG_DIR GIT_CONFIG_GLOBAL GIT_CONFIG_NOSYSTEM GIT_TERMINAL_PROMPT CODEX_HOME CLAUDE_CONFIG_DIR DESK_LOOP DESK_SESSION DESK_ROOTS; do grep -qF "\| $k \|" docs/cellctl.md \|\| { echo "missing env row: $k"; exit 1; }; done` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (on-behalf-of human:ian) (forge-identity) |
| 13 | `grep -c '^## Scrubbed cells' docs/cellctl.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (on-behalf-of human:ian) (forge-identity) |
| 14 | `statusgen --consumers --root . --base $(git merge-base origin/main HEAD)` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (on-behalf-of human:ian) (forge-identity) |
| 15 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-home-mode` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-09 | assay-verifier-app[bot] @ ca489e07187b (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2448 run=073a714c27279f4c0297fc219caf1e477237349fd7fb43e7adbb00a69a250aa5 source=ca489e07187b23ed1687b01795ea9fa7f92e39a9 brief=docs/streams/desk-containers/brief-09-scrubbed-host-cell.md model=claude-opus-5-5 tier=any

rows_passed=3 rows_total=15. Rows 11, 12 and 13 have a hand run and a witness pass. Rows 1 to 10, 14 and 15 each exit 0 by hand and have no witness. No row failed.

VERIFY: BLOCKED — no row failed. Twelve check:ci rows have no execution witness on this host (#1800), so the run is not a pass. Separately, row 3 does not assert all of its Expect, and the Go launcher fails the check-pass case that the rows run only against the bash oracle.

**The board row is not changed by this run and stays `implemented`.**

### Non-implementer verifier re-run — 2026-10-11 assay-verifier-app[bot] @ 251e9473928e (claude-opus-5-5) (on-behalf-of human:ian)

Fourth verify pass, on merged main 251e9473928e6e8c8c5ffbd331833900331043f1, in a temporary detached worktree cut at that commit. The verifier did not implement this item. Toolchain: GNU bash 5.3.20, shellcheck 0.11.0, go1.27.1 darwin/arm64, tmux 3.7c, git 2.56.0, statusgen v1.0.37.

What moved since the last run (2026-10-09 @ ca489e07187b): the bash oracle, scrubbed-cell.test.sh and house-cell.test.sh are byte-identical. The Go launcher under tools/desk/cmd/cellctl changed through #2446 (per-role starting context), #2470 (a machine-wide defaults.env in the cells root) and #2510 (session and custody for desk helpers). The scrubbed PATH tail is now a named constant in envkeys.go, and docs/cellctl.md gained about 220 lines.

How each row ran: from the worktree root, with a fresh empty HOME per row, KUBECONFIG=/dev/null, GH_TOKEN and DESK_ROOTS unset, and the directories holding the installed desk tools removed from PATH. That keeps the suite from calling the installed deskwt (#2464). Row 11 built offline (GOPROXY=off, module cache already populated). No row contacted a network endpoint or a model.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | Verify row 1 as written | exit 0 | exit 0, no output, no findings | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | Verify row 2 as written | exit 0; provisioned fixture passes every row | exit 0; 8 ok, 0 FAIL; last line reads scrubbed-cell.test.sh: OK | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | Verify row 3 as written | exit 0; no canary reaches the harness; allowlisted keys carry cell values; PATH is exactly the 7 elements | exit 0; 13 ok, 0 FAIL; the three env canaries are absent and the last row reads that the composed PATH is exactly the 7 named elements, in order, with no canary dir. Clause (ii) is asserted for only some keys (finding 1) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | Verify row 4 as written | exit 0; plan grammar and sorted keys equal the allowlist | exit 0; 6 ok, 0 FAIL | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | Verify row 5 as written | exit 0; READY passes; stub argv and HOME recorded | exit 0; 8 ok, 0 FAIL; codex stub called with exec --ephemeral --sandbox read-only and claude stub with -p, each under the cell home | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | Verify row 6 as written | exit 0; wrong exit code or wrong answer makes smoke exit 1 | exit 0; 4 ok, 0 FAIL | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | Verify row 7 as written | exit 0; live pid refuses with exit 4; dead pid is stale-lock; down clears it | exit 0; 6 ok, 0 FAIL | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | Verify row 8 as written | exit 0; symlink PEM and 0644 PEM are MISS; 0600 is ok | exit 0; 5 ok, 0 FAIL | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | Verify row 9 as written | exit 0 | exit 0; 54 ok, 0 FAIL; last line reads house-cell.test.sh: OK | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 10 | Verify row 10 as written | exit 0; two entries and an empty value are MISS; the exact slug is ok | exit 0; 3 ok, 0 FAIL | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 11 | Verify row 11 as written | exit 0; a bare scrubbed cell.env loads and check exits 1 | exit 0; the Go build succeeds; check prints kind=scrubbed with no exit-3 refusal, lists MISS rows (roster, config home real directory, config home mode 0700, exact roster scope, harness login under the cell home) and ends with fix the MISS rows first | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 12 | Verify row 12 with the table's escaped pipes unescaped | exit 0; a docs env row for each of the 15 keys | exit 0; no missing env row line printed | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 13 | Verify row 13 as written | exit 0; prints 1 | exit 0; prints 1 | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 14 | Verify row 14 as written | exit 0 | exit 0; prints that the diff against 251e9473928e has no brief files and there is nothing to corroborate. On merged main the merge-base equals HEAD, so this exit 0 corroborates nothing | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 15 | Verify row 15 as written | exit 0; a 0755 config home is a MISS naming 0700; 0700 is ok | exit 0; 3 ok, 0 FAIL | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Mutations. Each mutation went into a scratch copy of tools/cellctl/testdata/cellctl-shell-oracle.sh outside the worktree. The suite ran against that copy through its CELLCTL variable, so the worktree was never edited. First, each of the six cases passed against an unmutated copy. Nine of ten mutations turned their row red, the same result as the 2026-10-09 run:
- Row 3: env -i replaced by env at oracle :2151 gives exit 1, 3 FAIL. Appending the parent PATH to the composed PATH at :2075 gives exit 1, 1 FAIL.
- Row 3, **survived**: the ASSAY_CONFIG_HOME value at :2046 changed from the cell config home to the operator's gives exit 0, 13 ok (finding 1).
- Row 6: dropping the exit-code half of the READY test at :2611 gives exit 1, 2 FAIL. Dropping the READY half gives exit 1, 2 FAIL.
- Row 7: the live-pid test at :2113 forced false gives exit 1, 2 FAIL.
- Row 8: PEM mode 600 changed to 644 at :1714 gives exit 1, 2 FAIL. The symlink test at :1709 forced false gives exit 1, 1 FAIL.
- Row 10: the exact roster comparison at :1721 loosened to a prefix match gives exit 1, 1 FAIL.
- Row 15: config home mode 700 changed to 755 at :1690 gives exit 1, 2 FAIL.

Findings. None of them is the reason for the verdict below.
1. **Row 3 asserts less than its Expect** (not yet filed). Clause (ii) says every allowlisted key is present with its cell-relative value. The env-scrub case checks values only for HOME, TMPDIR, KUBECONFIG, GIT_TERMINAL_PROMPT, CLAUDE_CONFIG_DIR, CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION, DESK_ROOTS and PATH. That gap is why the ASSAY_CONFIG_HOME mutation survived again.
2. **The Go launcher still fails the check-pass case** (#2465). Supporting run, not a Verify row: the full scrubbed suite against a fresh Go build, with CELLCTL_DESKWT=0, exits 1 with 76 ok and 2 FAIL, both in check-pass. The cause is unchanged: the Go check probes codex features list (tools/desk/cmd/cellctl/codex_capacity.go:83), but the suite's codex stub answers only config get. Rows 2 to 10 and 15 run only the bash oracle.
3. **The suite calls the installed deskwt** (#2464). This run avoided it by removing the desk-tools directories from PATH, and no desk audit row was written.
4. Context still lists changelog/desk-containers-09-scrubbed-cell.md and scrubbed-cell.test.sh as planned. The changelog fragment does not exist on main; the test suite does. The frontmatter sources still cite tools/cellctl/cellctl, which is gone from main (#1968 tracks that part). The changelog part is not yet filed.
5. **New: a parent-shell CELL_PATH replaces the composed PATH's system tail.** Context says a cell.env CELL_PATH key overrides the trailing system part, and that the parent shell contributes nothing else. Both implementations, though, read CELL_PATH from the launching process environment. The oracle inherits the exported variable (:2074). The Go launcher seeds its Env from os.Environ (tools/desk/cmd/cellctl/cell.go:58-66), and its defaults template documents the order as compiled default, then process environment, then defaults.env, then cell.env (envkeys.go:405). CELL_PATH is also a machine-wide key now (envkeys.go:122), so a cells-root defaults.env can set it too. Supporting run, not a Verify row: one scratch scrubbed cell with no CELL_PATH in its cell.env, then DRY_RUN=1 desk run under env -i.
   - With no CELL_PATH, both the oracle and a fresh Go build print the 7-element PATH.
   - With CELL_PATH=/parent-canary-dir exported by the parent, both print a PATH of shim, desk-tools bindir, harness dir and then /parent-canary-dir.
   - Row 3 cannot see this because the suite unsets CELL_PATH before its fixture (scrubbed-cell.test.sh:34).
   - Only the operator's own shell or cells root can set the value, so this widens no credential path. It is still the parent-to-cell leak class this kind exists to close, and the brief does not authorise it. Not yet filed.

Not checked: the Definition of Done neighbour suites (harness.test.sh, container-cell.test.sh, cockpit.test.sh), which are not Verify rows; row 14 against a non-empty diff; the live harness values of the keys named in finding 1.

RISK-BEARING VALUE. The brief's risk answers are all no. Enumeration covered the literals named in Context and Task, plus the diff since ca489e07187b in the scrubbed code paths (env.go, check.go, new.go, envkeys.go, cell.go). The only value change in scope is the PATH tail, which moved from a literal into a named constant with the same text. Current lines:
- RISK-VALUE: DERIVED — pem-mode = 0o600 @ tools/desk/cmd/cellctl/check.go:445 (oracle :1714) — owner-only read and write is the narrowest mode that still lets the owning user's tools read an App private key. Check fails closed on a wider mode or a symlink, and row 8 plus its two mutations prove it.
- RISK-VALUE: DERIVED — config-home-mode = 0o700 @ tools/desk/cmd/cellctl/check.go:405 (oracle :1690) — owner-only list and traverse on the directory holding the keys is the minimum the owner needs to use it (row 15 and its mutation). new still creates the directory 0755 and sets it to 0700 in a second step (new.go:561, :569).
- RISK-VALUE: DERIVED — scrubbedPathTail = "/usr/bin:/bin:/usr/sbin:/sbin" @ tools/desk/cmd/cellctl/envkeys.go:89, read at env.go:120 (oracle :2073) — the four standard system directories, with no parent PATH inherited (row 3). The override path is finding 5.
- Unchanged in value: the 19-key env allowlist (tools/desk/cmd/cellctl/env.go:19-23, oracle :2018), lock refusal exit 4 (lock.go:28), unknown-kind exit 3 (cell.go:533) and the smoke prompt (smoke.go:50).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `shellcheck tools/cellctl/testdata/cellctl-shell-oracle.sh` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pass` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case env-scrub` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case plan-grammar` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-ready` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-not-ready` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 7 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case lock` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pem` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 9 | `bash tools/cellctl/tests/house-cell.test.sh` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 10 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-roster` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 11 | `d=$(mktemp -d); mkdir -p "$d/x"; printf 'CELL_KIND=scrubbed\nCELL_REPO=%s\nCELL_REPO_SLUG=example-org/example-repo\nCELL_HARNESS=codex\n' "$PWD" > "$d/x/cell.env"; b="$(mktemp -d)/cellctl"; (cd tools/desk && go build -o "$b" ./cmd/cellctl); CELLS_ROOT="$d" "$b" check x; test $? -eq 1` | pass exit=0 | sha256:8a3287252fac | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 12 | `for k in HOME ZDOTDIR PATH TMPDIR KUBECONFIG ASSAY_CONFIG_HOME GH_CONFIG_DIR GIT_CONFIG_GLOBAL GIT_CONFIG_NOSYSTEM GIT_TERMINAL_PROMPT CODEX_HOME CLAUDE_CONFIG_DIR DESK_LOOP DESK_SESSION DESK_ROOTS; do grep -qF "\| $k \|" docs/cellctl.md \|\| { echo "missing env row: $k"; exit 1; }; done` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 13 | `grep -c '^## Scrubbed cells' docs/cellctl.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 14 | `statusgen --consumers --root . --base $(git merge-base origin/main HEAD)` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
| 15 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-home-mode` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e+dirty (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2562 run=f232d5a5593feaf989e034c38d6ea7573f88b80cb9999e9d55c5cb9599343599 source=251e9473928e6e8c8c5ffbd331833900331043f1 brief=docs/streams/desk-containers/brief-09-scrubbed-host-cell.md model=claude-opus-5-5 tier=any

rows_passed=3 rows_total=15. Rows 11, 12 and 13 have a hand run and a witness pass. Rows 1 to 10, 14 and 15 each exit 0 by hand and have no witness: the network-off sandbox is unavailable on this darwin host (#1800). Row 14 is vacuous at this source (merge-base = HEAD, no brief in the diff). No row failed. The witness stamps its runner `+dirty` only because this Evidence edit preceded it; the tree's only change is this Evidence section.

VERIFY: BLOCKED — no row failed. Twelve check:ci rows have no execution witness on this host (#1800), so the run is not a pass. Separately, row 3 does not assert all of its Expect (finding 1), the Go launcher fails the check-pass case that the rows run only against the oracle (finding 2, #2465), a parent-exported CELL_PATH replaces the scrubbed PATH tail (finding 5), and the changelog fragment is still owed (finding 4).

**The board row is not changed by this run and stays `implemented`.**

### Non-implementer verifier re-run — 2026-10-15 assay-verifier-app[bot] @ 44054ca37568 (claude-opus-5-5)

Fifth verify pass, on merged main 44054ca3756857c34eb818601a4ddcb19a7b4da5, in a temporary detached worktree cut at that commit. The verifier did not implement this item. The work was delivered by #1263 (squash 58a019271393, 2026-09-17). Follow-ups moved the oracle (#1739) and made it shellcheck-clean (#1816). Toolchain: GNU bash 5.3.20, shellcheck 0.11.0, go1.27.2 darwin/arm64, git 2.56.0, statusgen v1.0.42.

Expected values, taken from the brief before any test was read: every row exits 0. Row 13 prints 1. Row 11 loads a bare scrubbed cell.env with no exit-3 refusal, and its check exits 1 on MISS rows. The composed launch PATH is exactly the seven named elements. PEMs must be regular non-symlink 0600 files, and the config home must be a real directory at 0700. ASSAY_ALLOWED_REPOS must equal the cell's one slug. A second desk on a live lock exits 4. Smoke passes only on exit 0 with a last line of exactly READY.

What moved since the last run (2026-10-11 @ 251e9473928e): scrubbed-cell.test.sh and house-cell.test.sh are byte-identical. The oracle changed in the house up-window path only: a cell with no deskd opens no placeholder window. No scrubbed function changed. In the Go launcher, check.go, new.go, cell.go, envkeys.go and smoke.go changed. Their scrubbed literals are unchanged (see the risk lines below).

How each row ran: from the worktree root, each row in its own env -i shell with a fresh empty HOME and TMPDIR in a scratch directory outside the worktree. KUBECONFIG=/dev/null. PATH held only the system and Homebrew directories plus a scratch bin holding statusgen. No installed desk verb was on PATH, so the suite could not call the installed deskwt (#2464). Row 11 built offline (GOPROXY=off, GOTOOLCHAIN=local, scratch GOCACHE). No row contacted a model or wrote to the network. After every row the worktree had no modified, untracked or ignored file.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `shellcheck tools/cellctl/testdata/cellctl-shell-oracle.sh` | exit 0 | Verify row 1: exit 0, no output, no findings | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 2 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pass` | exit 0; the provisioned fixture passes every row | Verify row 2: exit 0; 8 ok, 0 FAIL. The rows cover the PEM, exact roster scope, codex login under the cell home, real config-home directory, mode 0700, and the generic codex block. Last line: scrubbed-cell.test.sh: OK | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 3 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case env-scrub` | exit 0; no env canary; allowlisted keys carry cell values; PATH is exactly the 7 elements | Verify row 3: exit 0; 13 ok, 0 FAIL. The GH_TOKEN, SSH_AUTH_SOCK and ANTHROPIC_API_KEY canaries are absent. Last assertion: composed PATH is EXACTLY the 7 named elements, in order, with no canary dir. The case checks clause (ii) for only some keys (finding 1), so it was discharged directly below | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 4 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case plan-grammar` | exit 0; dry-run line first, sorted env keys, then argv, cwd, lock; key set equals the allowlist | Verify row 4: exit 0; 6 ok, 0 FAIL. The plan env key set equals SCRUBBED_ENV_KEYS minus the inactive harness var | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 5 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-ready` | exit 0; READY passes; argv and HOME recorded per arm | Verify row 5: exit 0; 8 ok, 0 FAIL. The codex stub was called with exec --ephemeral --sandbox read-only and the claude stub with -p, each under the cell home | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 6 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-not-ready` | exit 0; READY with exit 2 and a well-formed wrong answer both make smoke exit 1 with smoke: not ready: | Verify row 6: exit 0; 4 ok, 0 FAIL | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 7 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case lock` | exit 0; a live pid refuses with exit 4; a dead pid reads stale-lock; down clears it; desk then proceeds | Verify row 7: exit 0; 6 ok, 0 FAIL | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 8 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pem` | exit 0; a symlink PEM and a 0644 PEM are MISS and check exits 1; a 0600 PEM is ok | Verify row 8: exit 0; 5 ok, 0 FAIL | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 9 | `bash tools/cellctl/tests/house-cell.test.sh` | exit 0 | Verify row 9: exit 0; 54 ok, 0 FAIL. Last line: house-cell.test.sh: OK | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 10 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-roster` | exit 0; two entries and an empty value are MISS; the exact slug is ok | Verify row 10: exit 0; 3 ok, 0 FAIL | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 11 | `d=$(mktemp -d); mkdir -p "$d/x"; printf 'CELL_KIND=scrubbed\nCELL_REPO=%s\nCELL_REPO_SLUG=example-org/example-repo\nCELL_HARNESS=codex\n' "$PWD" > "$d/x/cell.env"; b="$(mktemp -d)/cellctl"; (cd tools/desk && go build -o "$b" ./cmd/cellctl); CELLS_ROOT="$d" "$b" check x; test $? -eq 1` | exit 0; a bare scrubbed cell.env loads with no exit-3 refusal and check exits 1 | Verify row 11: exit 0. The Go build succeeded. check printed kind=scrubbed with no unknown-kind refusal. It listed MISS rows for the roster, real config-home directory, config home mode 0700, exact roster scope, harness login under the cell home, and roster parse. It ended with fix the MISS rows first | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 12 | `for k in HOME ZDOTDIR PATH TMPDIR KUBECONFIG ASSAY_CONFIG_HOME GH_CONFIG_DIR GIT_CONFIG_GLOBAL GIT_CONFIG_NOSYSTEM GIT_TERMINAL_PROMPT CODEX_HOME CLAUDE_CONFIG_DIR DESK_LOOP DESK_SESSION DESK_ROOTS; do grep -qF "\| $k \|" docs/cellctl.md \|\| { echo "missing env row: $k"; exit 1; }; done` | exit 0; one docs env-table row per key | Verify row 12, run with the table's escaped pipes unescaped: exit 0, no missing env row line | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 13 | `grep -c '^## Scrubbed cells' docs/cellctl.md` | exit 0; prints 1 | Verify row 13: exit 0; prints 1 | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 14 | `statusgen --consumers --root . --base $(git merge-base origin/main HEAD)` | exit 0 | Verify row 14: exit 0. Output: no brief files in the diff against 44054ca37568, nothing to corroborate. On merged main the merge-base equals HEAD, so this exit 0 corroborates nothing about the original diff | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 15 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-home-mode` | exit 0; a 0755 config home is a MISS naming 0700 and exits 1; 0700 is ok | Verify row 15: exit 0; 3 ok, 0 FAIL | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |

Row 3 clause (ii), checked directly (supporting run for Verify row 3, not a new row). A scratch scrubbed cell was made with the oracle's new verb. DRY_RUN=1 desk then ran for the-desk on the claude arm under env -i, with both the oracle and a fresh offline Go build. Both printed identical plan env lines. ASSAY_CONFIG_HOME was the cell's home/.config/assay. GH_CONFIG_DIR was the cell's home/.config/gh. GIT_CONFIG_GLOBAL was the cell's home/.gitconfig. HOME and ZDOTDIR were the cell home, TMPDIR was the cell tmp, and CLAUDE_CONFIG_DIR was the cell's home/.claude. KUBECONFIG=/dev/null, GIT_CONFIG_NOSYSTEM=1, GIT_TERMINAL_PROMPT=0 and DESK_LOOP=the-desk. DESK_SESSION was the cell, role and UTC stamp. SHELL was the bash running cellctl. TERM and LANG were passed through. CODEX_HOME was absent (inactive harness), and so was DESK_ROOTS (no CELL_ROOTS). PATH was the 7 elements. The behaviour meets clause (ii); only the suite's assertion is narrower.

Mutations. Each mutation went into a scratch copy of the oracle outside the worktree. The suite ran against that copy through its CELLCTL variable, so the worktree was never edited. First, all six cases passed against an unmutated copy. Nine of ten mutations turned their row red:
- Row 3: env -i changed to env at oracle :2152 gives exit 1 with 3 FAIL. Appending the parent PATH to the composed PATH at :2076 gives exit 1 with 1 FAIL.
- Row 3, **survived**: the ASSAY_CONFIG_HOME value at :2047 pointed at an operator path gives exit 0 with 13 ok (finding 1).
- Row 6: dropping the exit-code half of the READY test at :2612 gives exit 1 with 2 FAIL. Dropping the READY half gives exit 1 with 2 FAIL.
- Row 7: the live-pid test at :2114 forced false gives exit 1 with 2 FAIL.
- Row 8: PEM mode 600 changed to 644 at :1715 gives exit 1 with 2 FAIL. The symlink test forced false gives exit 1 with 1 FAIL.
- Row 10: the exact roster comparison at :1722 loosened to a prefix match gives exit 1 with 1 FAIL.
- Row 15: config home mode 700 changed to 755 at :1691 gives exit 1 with 2 FAIL.

Definition of Done supporting runs (not Verify rows):
- harness.test.sh exits 0 (60 ok), container-cell.test.sh exits 0 (9 tests OK) and cockpit.test.sh exits 0 (49 ok).
- A CELL_KIND=local cell.env makes check exit 3 on both the oracle and the Go build, with the message that local is not a known kind.
- The aggregated CHANGELOG.md carries this item's Added entry (CHANGELOG.md:2604). This resolves the changelog part of the earlier finding 4.

Execution witness. `statusgen verifyrun --dry-run` was attempted and wrote nothing.
- With a scratch HOME, its pre-work admission refused (no roster).
- With the operator HOME, admission needs to mint a verifier App token on the forge. That is outside this run's offline envelope, so the step was not taken.
- Even if admitted, its check:ci sandbox is unshare --net, a Linux facility this darwin host lacks.
- The pinned-container mode needs a running container engine. None was running.
- So no witness exists for rows 1 to 10, 14 and 15 from this run. The witness is could-not-check, not a pass.

Findings. None of them is a failed row.
1. **Row 3 asserts less than its Expect** (still open). The env-scrub case checks values only for HOME, TMPDIR, KUBECONFIG, GIT_TERMINAL_PROMPT, CLAUDE_CONFIG_DIR, the suggestion flag, DESK_ROOTS and PATH. The ASSAY_CONFIG_HOME mutation survives. The behaviour is right (direct check above). The test does not pin it.
2. **The Go launcher still fails the check-pass case** (#2465). Supporting run: the full scrubbed suite against a fresh offline Go build, with CELLCTL_DESKWT=0, exits 1 with 76 ok and 2 FAIL, both in check-pass. Rows 2 to 10 and 15 run only the bash oracle.
3. **A parent-exported CELL_PATH replaces the composed PATH's system tail** (still open). The scratch cell had no CELL_PATH in its cell.env. Without the variable, both implementations print the 7-element PATH. With CELL_PATH=/parent-canary-dir exported by the parent shell, both print shim, desk-tools bindir, harness dir and then /parent-canary-dir. The oracle reads it at :2075. The Go launcher seeds its env from the process environment (tools/desk/cmd/cellctl/cell.go:60). Context allows only a cell.env CELL_PATH to override the tail. Row 3 cannot see this because the suite unsets CELL_PATH before its fixture (scrubbed-cell.test.sh:33-34). This widens no credential path, but it is the parent-to-cell leak class this kind exists to close.
4. Frontmatter sources still cite tools/cellctl/cellctl, which is no longer on main (#1968).

RISK-BEARING VALUE. The brief's four risk answers are all no. Enumeration covered the literals named in Context and Task, plus the scrubbed paths of the oracle and the Go launcher at this source. No scrubbed literal changed since 251e9473928e.
- RISK-VALUE: DERIVED — pem-mode = 0o600 @ tools/desk/cmd/cellctl/check.go:470 (oracle :1715) — owner-only read and write is the narrowest mode that still lets the owning user's desk tools read an App private key. Check fails closed on a wider mode or a symlink (row 8 and its two mutations).
- RISK-VALUE: DERIVED — config-home-mode = 0o700 @ tools/desk/cmd/cellctl/check.go:430 (oracle :1691) — owner-only list and traverse on the directory holding the keys is the minimum its owner needs. Any group or other bit exposes key names and mtimes (row 15 and its mutation). new creates the directory 0755 and then sets it to 0700 (tools/desk/cmd/cellctl/new.go:496, :504).
- RISK-VALUE: DERIVED — scrubbedPathTail = "/usr/bin:/bin:/usr/sbin:/sbin" @ tools/desk/cmd/cellctl/envkeys.go:89 (oracle :2074) — the four standard system directories, with no parent PATH inherited (row 3 and its PATH mutation). The parent-override path is finding 3.
- Lower-ranked and unchanged: the 19-key env allowlist (tools/desk/cmd/cellctl/env.go:19-23, oracle :2019). The lock-refusal exit code 4 (tools/desk/cmd/cellctl/lock.go:28, oracle :2116). The unknown-kind exit 3 (tools/desk/cmd/cellctl/cell.go:533, oracle :346). The smoke prompt (tools/desk/cmd/cellctl/smoke.go:47, oracle :2594). These are reversible by an edit and a redeploy.

Verification-Attestation: medici-finance/assay#2990 run=96a691c43b93fd25f4370dd03fafd645c5bb7cd6b70c875ba3fda75b49b13f48 source=44054ca3756857c34eb818601a4ddcb19a7b4da5 brief=docs/streams/desk-containers/brief-09-scrubbed-host-cell.md model=claude-opus-5-5 tier=any

rows_passed=15 rows_total=15 by direct run. No row failed. The execution witness for the check:ci rows is could-not-check on this host, and the verified cell derives only from that witness.

**VERIFY: BLOCKED** — 15/15 rows exit 0 by direct run and 0 fail. The check:ci execution witness could not run here: no unshare on darwin, no running container engine, and admission needs a forge token mint outside the offline envelope. It must be produced on a Linux runner. Findings 1 to 3 are open test or contract gaps, not failed rows.

**The board row is not changed by this run and stays `implemented`.**

### Non-implementer verifier re-run — 2026-10-24 assay-verifier-app[bot] @ 385cb29e5682 (claude-opus-5-5)

Sixth verify pass, on merged main 385cb29e5682dcb9a886b298076f722b4823ec1b, in a clean detached worktree cut at that commit. The verifier did not implement this item. Prior attempts: 2026-09-19/20 FAIL (row 1 shellcheck), 2026-09-23 FAIL (row 1; check:ci rows could-not-check), 2026-10-01, 2026-10-09 and 2026-10-11 BLOCKED (no check:ci witness), and 2026-10-15 BLOCKED at 44054ca37568 (15/15 exit 0 by direct run on darwin, no check:ci witness). This run is the first to produce that witness, and it is the first to run the rows on Linux.

Expected values, taken from the brief before any row ran: every row exits 0, and row 13 prints 1.

How the rows ran. Admission (`deskdispatch --check-verifier`) passed first. The whole table was split into two disjoint selections that together cover all 15 rows:
- Rows 1 to 10 and 15 ran inside one admission bracket. The host opened it with `statusgen verifyrun --open-bracket`. The container then ran `statusgen verifyrun --sandbox=container-netns --rows 1,2,3,4,5,6,7,8,9,10,15` in the pinned verify-runner v1.0.59 image (linux/amd64, `--network none`, read-only root, all capabilities dropped, uid 501). The worktree was bind-mounted read-only at /work with no gitdir. The host closed the bracket with `--close-bracket`, and `--bracket-digest` on the witness file reports `verdict=match`.
- Rows 11 to 14 ran on the host with `statusgen verifyrun --rows 11,12,13,14 --dry-run`. Its output was captured outside the worktree, so the worktree stayed clean through every run and through the bracket close.

The bracket's witness is below, as the tool wrote it, unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `shellcheck tools/cellctl/testdata/cellctl-shell-oracle.sh` | could-not-run exit=127 sandbox=container-netns — the shell could not execute the command (exit 127) | sha256:ffe102138d8c | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 2 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pass` | fail exit=1 sandbox=container-netns | sha256:0a1529f2d106 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 3 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case env-scrub` | fail exit=3 sandbox=container-netns | sha256:fa8cd772e0d2 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 4 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case plan-grammar` | fail exit=1 sandbox=container-netns | sha256:364ee6d8863b | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 5 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-ready` | pass exit=0 sandbox=container-netns | sha256:ffe0fc02199a | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 6 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-not-ready` | pass exit=0 sandbox=container-netns | sha256:75434b257259 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 7 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case lock` | fail exit=3 sandbox=container-netns | sha256:c5164e6d02c5 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pem` | fail exit=1 sandbox=container-netns | sha256:9e2de1430e83 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 9 | `bash tools/cellctl/tests/house-cell.test.sh` | pass exit=0 sandbox=container-netns | sha256:42357ffdc08e | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 10 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-roster` | pass exit=0 sandbox=container-netns | sha256:98f07031cd37 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 15 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-home-mode` | fail exit=1 sandbox=container-netns | sha256:30695d9a04b1 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:98685d6717c495aecd71bc3b8cb782aa9cc7dc0ac1332b029f60eb2c3a1c1104
Container-Image: sha256:c2e32c7ff99f67951174e8ff835d928d8999b1bb2ecdc7c63d3775528178640e
Verification-Attestation: medici-finance/assay#4313 run=f66c23f8034e17f522fb96c077f305e7ea7493aa1d97538f9b0a47423433a685 source=385cb29e5682dcb9a886b298076f722b4823ec1b brief=docs/streams/desk-containers/brief-09-scrubbed-host-cell.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=f66c23f8034e17f522fb96c077f305e7ea7493aa1d97538f9b0a47423433a685 source=385cb29e5682dcb9a886b298076f722b4823ec1b tree=sha256:98685d6717c495aecd71bc3b8cb782aa9cc7dc0ac1332b029f60eb2c3a1c1104 witness=sha256:30e873cfdb6d88a38e89d98c9859b77c721d96e79c7a5de69a03fc0adc586d44 opened=2026-10-24T18:18:39Z closed=2026-10-24T18:20:55Z before=receipt after=check-verifier

The host run's own table follows, unchanged. Row 14 needs git history (`git merge-base`). It cannot run in the container, which has no gitdir, and on this darwin host the check:ci sandbox (`unshare --net`) does not exist.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 11 | `d=$(mktemp -d); mkdir -p "$d/x"; printf 'CELL_KIND=scrubbed\nCELL_REPO=%s\nCELL_REPO_SLUG=example-org/example-repo\nCELL_HARNESS=codex\n' "$PWD" > "$d/x/cell.env"; b="$(mktemp -d)/cellctl"; (cd tools/desk && go build -o "$b" ./cmd/cellctl); CELLS_ROOT="$d" "$b" check x; test $? -eq 1` | pass exit=0 | sha256:1a4064018ed5 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 12 | `for k in HOME ZDOTDIR PATH TMPDIR KUBECONFIG ASSAY_CONFIG_HOME GH_CONFIG_DIR GIT_CONFIG_GLOBAL GIT_CONFIG_NOSYSTEM GIT_TERMINAL_PROMPT CODEX_HOME CLAUDE_CONFIG_DIR DESK_LOOP DESK_SESSION DESK_ROOTS; do grep -qF "\| $k \|" docs/cellctl.md \|\| { echo "missing env row: $k"; exit 1; }; done` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 13 | `grep -c '^## Scrubbed cells' docs/cellctl.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 14 | `statusgen --consumers --root . --base $(git merge-base origin/main HEAD)` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4313 run=f66c23f8034e17f522fb96c077f305e7ea7493aa1d97538f9b0a47423433a685 source=385cb29e5682dcb9a886b298076f722b4823ec1b brief=docs/streams/desk-containers/brief-09-scrubbed-host-cell.md model=claude-opus-5-5 tier=strong

What each non-pass means. Diagnostic re-runs were made in the same image against a `git archive` copy of the tree at 385cb29e5682, outside the worktree. They are not credited.
- **Row 4 is a real failure, on every platform.** Its last assertion fails in a clean environment: `plan-grammar: [plan] env KEY set equals SCRUBBED_ENV_KEYS minus the inactive harness var`. The plan prints 18 keys. The expected set has 19, and the missing key is `ASSAY_TELEMETRY`. #3050 (c893a16f2, merged 2026-10-15, after the last pass) added `ASSAY_TELEMETRY` to `SCRUBBED_ENV_KEYS` at tools/cellctl/testdata/cellctl-shell-oracle.sh:2023. But the oracle prints that key only when the launching shell exports it (:1355, :2051), and the case still demands the whole allowlist. In the same image with `ASSAY_TELEMETRY=1` exported, the case passes. On the darwin host with `ASSAY_TELEMETRY` unset, it fails the same way. So the earlier darwin passes depended on the operator's environment.
- **Rows 2, 8 and 15 are real failures on Linux.** These are the custody mode checks. Row 2 has 4 FAIL lines: exits 0, all preconditions met, PEM row ok, config home mode 0700 row ok. Row 8 fails `a regular 0600 PEM is ok`. Row 15 fails `0700 config home is ok`. The cause is `scrubbed_mode_is` (oracle :1678-1682), which runs `stat -f '%Lp' "$p" || stat -c '%a' "$p"`. With GNU coreutils 9.1, `-f` means `--file-system`. stat treats `%Lp` as a missing file, prints the file-system block for `$p`, and exits 1. The fallback then appends `700`. The captured value is 6 lines long and never equals `700` or `600`, so every correctly-moded file is reported MISS. On darwin (BSD stat), the same three cases exit 0. The Go launcher's `modeIs` (tools/desk/cmd/cellctl/check.go:430, :470) uses os.Stat and is not affected.
- **Rows 3 and 7 are an instrument limit, not a deliverable verdict.** Both exit 3 with `cellctl: desk: tmux is required for a scrubbed cell (the private-socket session) and is not on PATH` (oracle :2136). The suite needs a real tmux, and the pinned image has none.
- **Row 1 is an instrument limit.** It is could-not-run, exit 127, because shellcheck is not in the pinned image.
- **Row 14 is an instrument limit.** It is could-not-run, per the host table above.
- Rows 5, 6, 9 and 10 pass in the container. Rows 11, 12 and 13 pass on the host, and row 13 printed `1` (its output digest sha256:4355a46b19d3 is the sha256 of `1` plus a newline).

Findings carried from 2026-10-15 were not re-examined in this run. They are: 1, row 3 asserts less than its Expect; 2, the Go launcher fails check-pass (#2465); 3, a parent-exported CELL_PATH replaces the PATH tail; 4, the sources cite the removed tools/cellctl/cellctl (#1968).

RISK-BEARING VALUE. The brief's four risk answers are all no, and the gate is model. The literals were enumerated again at 385cb29e5682. Their values have not changed; only the oracle line numbers moved.
- RISK-VALUE: DERIVED — pem-mode = 0o600 @ tools/desk/cmd/cellctl/check.go:470 (oracle :1719) — owner-only read and write is the narrowest mode that still lets the owning user's desk tools read an App private key. Note that on Linux the oracle's probe for this value is broken (rows 2 and 8 above).
- RISK-VALUE: DERIVED — config-home-mode = 0o700 @ tools/desk/cmd/cellctl/check.go:430 (oracle :1695) — owner-only list and traverse is the minimum the owner needs on the directory holding the keys. The oracle's probe for it is broken on Linux in the same way (rows 2 and 15).
- RISK-VALUE: DERIVED — scrubbedPathTail = "/usr/bin:/bin:/usr/sbin:/sbin" @ tools/desk/cmd/cellctl/envkeys.go:89 (oracle :2079) — the four standard system directories, with no parent PATH inherited.
- Lower-ranked and unchanged: the env allowlist (tools/desk/cmd/cellctl/env.go:19-23, oracle :2023, now 20 keys including ASSAY_TELEMETRY), the lock-refusal exit code 4 (tools/desk/cmd/cellctl/lock.go:28), and the smoke prompt (tools/desk/cmd/cellctl/smoke.go:47, oracle :2599). These are reversible by an edit and a redeploy.

Accounting: one container attempt was refused before any row ran. That attempt used a `-v` mount, and the tool reported `container admission refused; no Verify rows executed: the mounted root ... digests to sha256:e3b0c442...`. It produced no witness. The run was repeated under the same receipt with `--mount type=bind`, and that repeat produced the witness above. No other run was discarded.

rows_passed=7 rows_total=15. The witnessed results are: rows 5, 6, 9 and 10 pass in the container; rows 11, 12 and 13 pass on the host; rows 2, 4, 8 and 15 fail on deliverable defects; rows 3 and 7 are blocked by the missing tmux; rows 1 and 14 are could-not-run.

**VERIFY: FAIL** — witnessed under container-netns at 385cb29e5682. Row 4 fails on every platform: #3050 put `ASSAY_TELEMETRY` in the allowlist, but the oracle prints the key only when the launching shell exports it. Rows 2, 8 and 15 fail on Linux: the oracle's `stat -f || stat -c` mode probe misreads GNU stat. Rows 1, 3, 7 and 14 are instrument limits (no shellcheck or tmux in the pinned image, and row 14 needs git and unshare).

**The board row is not changed by this run and stays `implemented`.**

## Review
Gate: model (all four risk answers no — a new cell kind plus three verbs on a host launcher;
it composes an environment with STRICTLY LESS of the operator's authority than the existing
house kind grants, adds no credential path, and every row runs against stubs). Reviewer
confirms rows 3, 6, 7, 8, 10 and 15 are NEGATIVE-path (a wrong-but-well-formed cell must go red)
and that the allowlist in the script is the single source both the plan and the docs table read.
Row 3 in particular pins the composed `PATH` to exactly seven elements — the one parent-derived
element (the harness dir) named, every other leak red — so its isolation assertion is
satisfiable rather than self-contradictory.
