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

### Non-implementer verifier run — VERIFY: FAIL (row 1 only: shellcheck exit 1 persists on current merged main; rows 2-15 green) — 2026-09-23 claude-opus-4-8-verifier

Fresh classification pass, own temp worktree detached at merged origin/main, offline envelope (KUBECONFIG=/dev/null), not the implementer. Merged SHA 39866201. Row 1 was already FAIL on the 2026-09-20 pass (filed #1355); it remains red. The original item commit 58a019271 (PR #1263) introduced the `value_in $KIND_VALUES/$COCKPIT_VALUES/$HARNESS_VALUES` unquoted-expansion (SC2086) findings #1355 tracks; those exact lines were re-touched by #1308 keeping the same pattern, and #1401 added further SC2030/SC2031 findings — none fixed the row-1 defect.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | shellcheck tools/cellctl/cellctl | exit 0 | exit 1 — 7 SC2086 (info) unquoted-expansion findings on value_in $KIND_VALUES / $COCKPIT_VALUES / $HARNESS_VALUES (lines 993, 996, 1044, 1271, 1272, 2134, 2608) plus 4 new SC2030/SC2031 subshell findings (lines 925, 1320). Same defect class as #1355 (originally exit-1 on this item's commit 58a019271); still red on merged main. FAIL | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pass | exit 0 | exit 0 — 8 ok / 0 FAIL (PEM row, roster ASSAY ALLOWED REPOS row, harness login under cell home, config home real-dir + mode 0700) | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case env-scrub | exit 0 | exit 0 — 13 ok / 0 FAIL: no GH-TOKEN / SSH-AUTH-SOCK / ANTHROPIC-API-KEY canary reaches the harness; allowlist keys present with cell-relative values; composed PATH is EXACTLY the 7 named elements in order, no canary dir | 2026-09-23 | claude-opus-4-8-verifier |
| 4 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case plan-grammar | exit 0 | exit 0 — 6 ok / 0 FAIL: existing dry-run line first, then plan env KEY-sorted, argv, cwd, lock; env KEY set dereferences the allowlist variable minus the inactive harness var | 2026-09-23 | claude-opus-4-8-verifier |
| 5 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-ready | exit 0 | exit 0 — 8 ok / 0 FAIL: codex arm invoked with exec --ephemeral --sandbox read-only, claude arm with -p, each under the cell HOME; prints READY | 2026-09-23 | claude-opus-4-8-verifier |
| 6 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case smoke-not-ready | exit 0 | exit 0 — 4 ok / 0 FAIL: READY-but-exit-2 and wrong-but-well-formed answer each give exit 1 with "smoke: not ready:" | 2026-09-23 | claude-opus-4-8-verifier |
| 7 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case lock | exit 0 | exit 0 — 6 ok / 0 FAIL: second desk while pid alive exits 4 naming "cellctl down"; dead pid → status prints stale-lock; down clears; after down, desk proceeds | 2026-09-23 | claude-opus-4-8-verifier |
| 8 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-pem | exit 0 | exit 0 — 5 ok / 0 FAIL: symlink PEM → MISS naming symlink, exit 1; regular 0644 → MISS naming 0600, exit 1; regular 0600 → ok | 2026-09-23 | claude-opus-4-8-verifier |
| 9 | bash tools/cellctl/tests/house-cell.test.sh | exit 0 | exit 0 — 53 ok / 0 FAIL (house kind untouched, regression guard) | 2026-09-23 | claude-opus-4-8-verifier |
| 10 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-roster | exit 0 | exit 0 — 3 ok / 0 FAIL: two-slug value MISS, empty value MISS, exact slug ok | 2026-09-23 | claude-opus-4-8-verifier |
| 11 | bare scrubbed cell.env in a temp CELLS_ROOT; cellctl check x; test exit == 1 | exit 0 (overall) | exit 0 overall — check loaded kind=scrubbed (no exit-3 die), printed MISS rows and exited 1 | 2026-09-23 | claude-opus-4-8-verifier |
| 12 | for k in HOME ZDOTDIR PATH TMPDIR KUBECONFIG ASSAY_CONFIG_HOME GH_CONFIG_DIR GIT_CONFIG_GLOBAL GIT_CONFIG_NOSYSTEM GIT_TERMINAL_PROMPT CODEX_HOME CLAUDE_CONFIG_DIR DESK_LOOP DESK_SESSION DESK_ROOTS; do grep -qF "\| $k \|" docs/cellctl.md \|\| { echo "missing env row: $k"; exit 1; }; done | exit 0 | exit 0 — no "missing env row" line printed; all 15 variable-name rows present in the docs env table | 2026-09-23 | claude-opus-4-8-verifier |
| 13 | grep -c '^## Scrubbed cells' docs/cellctl.md | exit 0; prints 1 | exit 0, printed 1 | 2026-09-23 | claude-opus-4-8-verifier |
| 14 | statusgen --consumers --root . --base git merge-base origin/main HEAD | exit 0 | exit 0 — no brief files in the diff against merged main (base == HEAD), nothing to corroborate, trivially green | 2026-09-23 | claude-opus-4-8-verifier |
| 15 | bash tools/cellctl/tests/scrubbed-cell.test.sh --case check-home-mode | exit 0 | exit 0 — 3 ok / 0 FAIL: 0755 config home → MISS naming 0700, exit 1; 0700 → ok | 2026-09-23 | claude-opus-4-8-verifier |

Execution witness (statusgen verifyrun, left uncommitted in the brief file for the desk): rows 11, 12, 13 pass; every check:ci row (1-10, 14, 15) is could-not-run on this host because the hermetic network-off sandbox needs Linux `unshare --net` and this host is darwin — recorded verbatim as could-not-check for the witness lane, neither pass nor fail. Row 1's FAIL above is my own direct manual run of shellcheck, a static linter that needs no network, so the offline envelope does not limit it.

RISK-BEARING VALUE (brief risk all-no, gate model, merged-main diff empty — the fail-safe trigger does not strictly fire; enumerated over the brief's own pinned literals anyway):
- RISK-VALUE: DERIVED — pem-mode = 0600 @ tools/cellctl/cellctl:1639 — repo's standing custody convention for key/PEM files; check_scrubbed fails closed on any widening (row 8 green).
- RISK-VALUE: DERIVED — config-home-mode = 0700 @ tools/cellctl/cellctl:1615 — owner-only traversal for the directory holding the 0600 PEMs (a group/world-readable dir leaks PEM names and mtimes); check fails closed (row 15 green).
- RISK-VALUE: DERIVED — scrubbed-path-tail = /usr/bin:/bin:/usr/sbin:/sbin @ tools/cellctl/cellctl:1998 — the fixed system tail behind the shim prefix and the one named parent-derived element (the resolved harness dir); never a wholesale parent PATH; row 3 proves the exact 7-element order and canary-dir absence.
- Remaining literals rank last as reversible operational knobs, no derivation owed: SCRUBBED_ENV_KEYS allowlist @ tools/cellctl/cellctl:1943 (rows 3/4 pin it), lock-refusal exit 4 + message @ tools/cellctl/cellctl:2039 (row 7), unknown-kind exit 3 @ tools/cellctl/cellctl:325 (row 11).

VERIFY: FAIL — Verify row 1 only (shellcheck exit 1). The feature behaviour (rows 2-15) is verified sound on current merged main. Per the fail rule the item does NOT advance: status stays implemented, no flip.

Row 1 (shellcheck on the cellctl script) still fails: the SC2086 findings tracked at #1355 plus four newer SC2030/SC2031 findings; rows 2-15 pass.


## Review
Gate: model (all four risk answers no — a new cell kind plus three verbs on a host launcher;
it composes an environment with STRICTLY LESS of the operator's authority than the existing
house kind grants, adds no credential path, and every row runs against stubs). Reviewer
confirms rows 3, 6, 7, 8, 10 and 15 are NEGATIVE-path (a wrong-but-well-formed cell must go red)
and that the allowlist in the script is the single source both the plan and the docs table read.
Row 3 in particular pins the composed `PATH` to exactly seven elements — the one parent-derived
element (the harness dir) named, every other leak red — so its isolation assertion is
satisfiable rather than self-contradictory.
