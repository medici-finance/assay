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
  - "tools/cellctl/cellctl: follow-up desk-containers/09 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/cellctl/tests/scrubbed-cell.test.sh: follow-up desk-containers/09 (this brief; new file, flips to fixed-here when it lands)"
  - "docs/cellctl.md: follow-up desk-containers/09 (this brief; the new kind's section + the two new verbs)"
  - "tools/cellctl/tests/house-cell.test.sh: out-of-scope (the house kind is untouched — row 9 proves the neighbour still passes; no edit is owed)"
---

# Brief 09 — cellctl: host-local harness cell — scrubbed per-cell environment, `smoke`, `status`, session lock, stricter `check`

## Context

files:
- `tools/cellctl/cellctl` — `load_cell` kind switch (a fourth kind, `scrubbed`); `cmd_new
  --kind scrubbed`; `check_scrubbed`; the scrubbed arm of `cmd_desk`; two new verbs `smoke`
  and `status`; the session lock; `cmd_down` on a scrubbed cell.
- `tools/cellctl/tests/scrubbed-cell.test.sh` (planned) — the fixture suite, `--case <name>`
  selectable; every Verify row below names its case.
- `docs/cellctl.md` — new section *Scrubbed cells — `--kind scrubbed`* placed between *House
  cells* and *Container cells*; `smoke` and `status` added to the header usage block and the
  verb walk-through; the stricter rows appended to the `check` tables.
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
| 1 | `shellcheck tools/cellctl/cellctl` | exit 0 (red on the merge-base is not expected here — this row guards the edit) | check:ci |
| 2 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pass` | exit 0; a scrubbed fixture cell with regular 0600 PEMs, an exact `ASSAY_ALLOWED_REPOS`, a stub `codex` whose `login status` exits 0, passes every row. Red on the merge-base: the script does not exist (exit 127) and `cellctl check` on the fixture dies `CELL_KIND=scrubbed is not a known kind` (exit 3) | check:ci |
| 3 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case env-scrub` | exit 0; the case exports `GH_TOKEN=canary-parent`, `SSH_AUTH_SOCK=/nonexistent`, `ANTHROPIC_API_KEY=canary` AND a canary directory on the parent `PATH`, launches `desk` against a stub harness that dumps its environment, and asserts: (i) none of the three env canaries is present; (ii) every allowlisted KEY is present with its cell-relative value; (iii) the composed `PATH` equals EXACTLY `<cell>/shim`, the desk-tools bindir, `dirname` of the resolved stub harness, `/usr/bin`, `/bin`, `/usr/sbin`, `/sbin` — in that order and with no other element, so the harness dir is the one permitted parent-derived element and the injected canary dir does NOT appear. A leak of any additional parent-`PATH` element is still red. (the flow: parent shell → `cellctl desk` → composed env → what the harness process actually receives). Red on the merge-base: exit 127 | check:ci +mutation +flow |
| 4 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case plan-grammar` | exit 0; `DRY_RUN=1 cellctl desk <fixture> the-desk` prints the existing `[dry-run]` line THEN `[plan] env` lines in sorted KEY order, then `[plan] argv`, `[plan] cwd`, `[plan] lock`; the set of `[plan] env` KEYs equals the allowlist variable in the script (dereferencing: the plan is generated from the list, so a KEY added to one and not the other fails here). Red on the merge-base: exit 127 | check:ci +dereference |
| 5 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-ready` | exit 0; a stub harness printing `READY` makes `cellctl smoke` exit 0 and print `READY`; the stub records it was invoked with `exec --ephemeral --sandbox read-only` (codex) / `-p` (claude) and under `HOME=<cell>/home`. Red on the merge-base: exit 127 | check:ci |
| 6 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-not-ready` | exit 0; a stub printing `READY` but exiting 2, and a stub exiting 0 printing `I cannot`, each make `smoke` exit 1 with `smoke: not ready:` on stdout — the verb cannot pass on a wrong-but-well-formed answer | check:ci +mutation |
| 7 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case lock` | exit 0; with a lock dir holding a live pid, a second `desk` exits 4 with `is already running (pid`; with a dead pid, `status` prints `stale-lock` and `down` clears it; after `down`, `desk` proceeds | check:ci +mutation |
| 8 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pem` | exit 0; a PEM that is a symlink → `MISS` row naming `symlink`; a regular 0644 PEM → `MISS` row naming `0600`; a regular 0600 PEM → `ok`; `check` exits 1 on either MISS | check:ci +mutation |
| 9 | `bash tools/cellctl/tests/house-cell.test.sh` | exit 0 — the house kind is untouched (neighbour row; this suite passes on the merge-base too, so it is the regression guard, not the feature proof) | check:ci +neighbour |
| 10 | `bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-roster` | exit 0; `ASSAY_ALLOWED_REPOS=example-org/example-repo,example-org/other` → `MISS`; `ASSAY_ALLOWED_REPOS=` → `MISS`; `ASSAY_ALLOWED_REPOS=example-org/example-repo` on a cell with that slug → `ok` | check:ci +mutation |
| 11 | `d=$(mktemp -d); mkdir -p "$d/x"; printf 'CELL_KIND=scrubbed\nCELL_REPO=%s\nCELL_REPO_SLUG=example-org/example-repo\nCELL_HARNESS=codex\n' "$PWD" > "$d/x/cell.env"; CELLS_ROOT="$d" bash tools/cellctl/cellctl check x; test $? -eq 1` | exit 0 — a bare scrubbed cell.env with an empty home is LOADED (no exit-3 die) and `check` reports MISS rows and exits 1. Red on the merge-base: `check` exits 3 (`not a known kind`), so the `test` fails | check +dereference |
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

## Review
Gate: model (all four risk answers no — a new cell kind plus three verbs on a host launcher;
it composes an environment with STRICTLY LESS of the operator's authority than the existing
house kind grants, adds no credential path, and every row runs against stubs). Reviewer
confirms rows 3, 6, 7, 8, 10 and 15 are NEGATIVE-path (a wrong-but-well-formed cell must go red)
and that the allowlist in the script is the single source both the plan and the docs table read.
Row 3 in particular pins the composed `PATH` to exactly seven elements — the one parent-derived
element (the harness dir) named, every other leak red — so its isolation assertion is
satisfiable rather than self-contradictory.
