# `cellctl` — running one Assay cell on a laptop

`topology.yaml` (see `docs/adopting-assay.md` §5) says what a **cell** *is*: one lead plus its agent
fleet, accountable for its own repo set. This document is the other half — how that cell **runs** on
one machine: a persistent `deskd`, one window per desk role, and each of those windows resolving the
cell's own roster and App keys rather than the operator's.

`tools/cellctl/cellctl` is a single bash script that does it. It exists because doing it by hand is a
dozen steps with three sharp edges (a harness login that vanishes when `HOME` is swapped, four
windows racing on one `.git`, a tmux window name that has to match what `down` looks for), and a
script is the only place those stay fixed.

`cellctl` is **optional**, in the same sense as the desk-tools binaries: it automates a pipeline you
can also stand up by hand. Nothing else in Assay depends on it.

For an existing container deployment, the **container** kind provides registration and lifecycle
delegation instead of host worktrees and credential symlinks. See [Container cells](#container-cells).
For a harness that must run on the host but must NOT inherit the launching shell's credentials, the
**scrubbed** kind composes a fully isolated environment instead. See
[Scrubbed cells](#scrubbed-cells----kind-scrubbed).

---

## What a cell is, on a laptop

One directory. Everything the cell needs is under it, except the private keys, which stay in the
operator's config home and are reached by symlink.

```
<cells-root>/<cell>/
  cell.env             the per-cell variables (written by `cellctl new`)
  cells-<cell>.yaml    this cell's slice of cells.yaml — and nothing else
  home/                the CELL config-home (see below)
    .config/assay/
      roster.env       THIS cell's trust roster and repo sets
      apps.env         THIS cell's App ids
      <role>-app.pem   symlinks into the operator's real config home
    .config/gh         symlink to the operator's real gh config
    .gitconfig         symlink to the operator's real gitconfig
  bin/                 deskd + deskcli for this cell
  index/               the persistent deskd index (survives restarts)
  worktrees/<role>/    one worktree per role, merged up to origin/main at every boot
  shim/                generated — every desk verb wrapped to run with HOME=<cell>/home
```

That is the **k8s** kind — a full cell with its own `deskd`, roster and App keys. A **house** cell
(`--kind house`, below) is the same shape minus what it does not need: `home/.config/assay` is one
symlink to the operator's real config home, there is no `cells-<cell>.yaml`, `bin/` or `index/`
unless `DESKD=1`, and `cell.env` carries the stream-root map (`CELL_ROOTS`).

The **cells root** defaults to `${CELLS_ROOT:-${XDG_DATA_HOME:-$HOME/.local/share}/assay/cells}`.
The **desk-tools bindir** the shims wrap defaults to `${DESK_TOOLS_BIN:-/opt/desk-tools/bin}` — the
path the pinned desk-tools tarball installs to (`docs/adopting-assay.md`, PRIMITIVE:
`install-desk-tools`). The **operator's real config home**, which the App-key symlinks point into, is
`${ASSAY_CONFIG_HOME:-$HOME/.config/assay}`. Override any of the three in the environment.

Running two cells on one laptop is two directories under the cells root, each with its own roster,
its own Apps, its own `deskd` port, and its own tmux session. They share nothing but the binaries.

---

## Why the session keeps the real `HOME`

This section applies to the **house** and **k8s** host launch paths. A container launcher assigns
the harness its own container home and supplies that environment's authentication separately.

This is the one design rule worth reading before anything else, because getting it wrong looks like a
harness bug rather than a configuration mistake.

The obvious way to give a cell its own identity is to run the whole agent session with
`HOME=<cell>/home`. **Do not.** The harness keys its login credentials, installed plugins and memory
by `HOME`; move `HOME` and the session comes up logged out, with no plugins, in a cell directory that
has none of that — and the failure presents as "it asked me to log in again", not as "the cell home
is wrong".

What actually needs the cell's config-home is much smaller: the **desk verbs**, which read
`$HOME/.config/assay/roster.env` for the trust roster and the App bindings. So `cellctl` generates a
`shim/` directory holding one wrapper per desk-tools binary —

```bash
#!/usr/bin/env bash
exec env HOME="<cell>/home" "/opt/desk-tools/bin/deskboard" "$@"
```

— and puts `shim/` **first** on the session's `PATH`. The session keeps the operator's real `HOME`
and `CLAUDE_CONFIG_DIR`; every desk verb it invokes resolves the cell's roster and the cell's Apps.
The split is exact: identity for the tools that write, the operator's own environment for the harness
that hosts them.

The shims are regenerated on every `cellctl desk`, so installing a new desk-tools release picks up
automatically.

---

## Install

`cellctl` ships inside `desk-tools-<platform>.tar.gz` (#850) — the same tarball, on the same
`checksums.txt`-pinned channel, as every other desk-tools binary; the `install-desk-tools`
PRIMITIVE (`docs/adopting-assay.md`) and `deskinstall` both extract it onto your bindir alongside
the rest, no extra step. It is a Go program (`tools/desk/cmd/cellctl`), cross-compiled PER
PLATFORM like every other desk verb, so each platform's tarball carries its own build.
`make desk-install` in this repo installs it too, from the same `desk-build` output.

The filename inside the tarball is unchanged, so the from-tarball line still reads:

```bash
install -m 0755 ./cellctl ~/.local/bin/cellctl                 # from an extracted release tarball
# or, from a checkout of this repo (needs a Go toolchain):
cd tools/desk && go build -o ~/.local/bin/cellctl ./cmd/cellctl
```

`cellctl` needs `git` and `tmux` (the default cockpit) on `PATH` — `cellctl check` reports each
one. A checkout build needs a Go toolchain; a tarball install does not.

A packaged copy reports the umbrella release tag it shipped at via `cellctl --version` (or
`cellctl version`) — the same contract `statusgen --version` uses, so a stale copy is detectable.
The tag is stamped at link time (`-ldflags -X main.cellctlVersion=<tag>`) exactly as every other
desk binary's is; a plain source build honestly reports `dev`.

**Windows.** The tarball's `windows-amd64` / `windows-arm64` legs carry a real `cellctl.exe`
rather than a shell script no Windows shell runs — the package itself cross-compiles for
`GOOS=windows` today. That is a BUILD, not a delivered Windows launcher: proving a Windows cell
actually boots (and the `internal/deskkit` unix-only syscall sites brief
`docs/streams/windows-port/` 00 owns) belongs to the windows-port stream, not here. Until that
stream delivers, treat the Windows binaries as untested.

---

## `cellctl new` — scaffold, then four hand steps

`new` takes the cell's **forge** explicitly with `--forge github|gitlab` (default `github`). The
custody flags are forge-specific — a GitHub cell mints installation tokens from an App PEM, a
GitLab cell reads a hand-provisioned role token store — so `--deskd-app-pem` and `--orgs` are
required on the **github** path only, and a **gitlab** cell requires its group instead:

```bash
# GitHub cell
cellctl new <cell> --forge github \
  --repo /path/to/checkout \
  --cells-yaml /path/to/cells-<cell>.yaml \
  --orgs org-a,org-b \
  --deskd-app-pem "$HOME/.config/assay/<cell>-desk-app.pem" \
  [--deskd-app-id-var DESK_APP_ID] \
  [--port 8787]

# GitLab cell — NO App PEM, NO --orgs; the group and a role token store instead
cellctl new <cell> --forge gitlab \
  --repo /path/to/checkout \
  --cells-yaml /path/to/cells-<cell>.yaml \
  --group my-gitlab-group \
  [--gitlab-api-base https://gitlab.example.com/api/v4] \
  [--gitlab-token-store /path/to/store] \
  [--port 8787]
```

On the **gitlab** path `cellctl new` mints nothing and requires no App PEM: GitLab role tokens
rotate by hand (forge-neutral/01), so the scaffold leaves a **role token store** to fill —
`gitlab-<role>.token` files at mode `0600`, one per role — and the per-cell README names it as a
hand step. A GitLab cell's roster entries are **forge-qualified** (`role=gitlab:<slug>:<id>`, per
forge-neutral/02) and its `ASSAY_REPO_FORGES` binds the cell's repos to `gitlab`, so the verbs do
not refuse a cell stood up for GitLab.

`--deskd-app-id-var` names the variable in the **cell home's** `apps.env` holding the `deskd` App's
id, and defaults to `DESK_APP_ID`. That default is right for most cells: `cellctl deskd` sources the
cell home's `apps.env`, not the operator's, and a cell home names its Apps by generic **role**
(`DESK_APP_ID`, `REVIEWER_APP_ID`, …) rather than by the operator's own naming. Pass the flag only
when this cell's `apps.env` calls it something else.

That writes `cell.env`, creates `home/`, `bin/`, `index/`, `worktrees/`, copies the cells slice in,
and symlinks `.config/gh` + `.gitconfig` to the operator's real ones. It then prints a per-cell
`README.md` naming what is left.

**Four steps remain, and `cellctl new` deliberately does not do them.** Each moves key material or
states who the cell trusts — custody acts, which are the ones a human should perform knowingly:

1. **`home/.config/assay/roster.env`** — this cell's roster: `ASSAY_TRUSTED_LOGINS`,
   `ASSAY_BLESS_LOGIN`, `ASSAY_TRUSTED_BOT_SLUGS` with `role=` bindings to **this** cell's Apps, and
   `ASSAY_ALLOWED_REPOS` / `ASSAY_SCAN_REPOS` naming **this** cell's repos only. Starting from
   another cell's file is fine; leaving one of its App bindings in place is not — that points this
   cell's writes at another cell's identity.
2. **`home/.config/assay/apps.env`** — the cell's App ids under the generic role names
   (`DESK_APP_ID`, `REVIEWER_APP_ID`, …) — plus one `<role>-app.pem` **symlink** per role into
   `${ASSAY_CONFIG_HOME:-$HOME/.config/assay}`. Symlink, never copy: the private keys keep one
   custody location, and a cell directory you delete takes no key material with it. If this cell
   names its `deskd` App id something other than `DESK_APP_ID`, set `DESKD_APP_ID_VAR` in `cell.env`
   to the name used here — `cellctl deskd` names both the file and the variable when it cannot find
   one, rather than guessing.
3. **`bin/deskd` and `bin/deskcli`** — build them from the desk console (its `cmd/deskd` and
   `cmd/deskcli`), or take them from the release tarball if your distribution channel ships them.
4. **`cells-<cell>.yaml`** — the cell's slice of `cells.yaml` and nothing else. It must validate on
   its own, and no other cell's repos may appear in it. A slice that still carries the fleet's repos
   makes `deskd` read repos this cell has no installation on, which surfaces as owner-level `404`s
   rather than as a config error.

Then `cellctl check <cell>`.

---

## House cells — `--kind house`

Everything above is the cell that runs *someone else's* repos on this laptop: its own `deskd`, its
own roster, its own Apps. The other case is the operator's **own** desks — the five role windows for
the repos whose roster and App keys already live in `~/.config/assay`. Before `--kind house` those
were booted by hand, and a hand boot has three ways to go wrong that a cell boot does not: the
window starts inside a shared checkout (and the write guard then refuses every mutation), the
stream-root map is not exported (and every desk verb silently falls back to its compiled
placeholder topology), and the model is whatever the CLI default is that week.

```bash
cellctl new house --kind house \
  --repo /path/to/checkout \
  --roots 'example-org/example-repo=/path/to/checkout,example-org/other=/path/to/other' \
  [--roles "the-desk worker-desk"] [--port 8787]
cellctl check house
cellctl desk house worker-desk          # one window
cellctl up house                        # every role, in the resolved cockpit (see *Cockpits*)
```

`new --kind house` needs `--repo` (the checkout the role worktrees are created from — it must be a
git checkout) and `--roots` (the `DESK_ROOTS` map: `<owner>/<repo>=<absolute path>`, comma-
separated; every path must exist). It writes `cell.env` with `CELL_KIND=house` and
`CELL_ROOTS=<the map>`, links `home/.config/assay` to `${ASSAY_CONFIG_HOME:-$HOME/.config/assay}`
— one directory symlink, so the desk verbs read the roster, `apps.env` and the `<role>-app.pem`
files exactly as a hand boot would; **nothing is copied** — and links `.config/gh` and
`.gitconfig` as a github cell does. There are no custody hand steps: the custody is the
operator's own. It refuses to overwrite an existing cell of the same name.

`check` on a house cell proves what a hand boot gets wrong rather than what a k8s cell needs: the
checkout is a git checkout; the roster **parses** under the cell home (`deskroster repos --scope
scan` exits 0 through the cell's `HOME`), not merely exists; every entry of `CELL_ROOTS` is well-
formed, exists and carries `docs/streams/`; the desk verbs a role needs are installed under the
desk-tools bindir; `claude` is on `PATH` and the `assay@assay` plugin is enabled for the checkout.
`deskd` is reported `n/a` unless `cell.env` sets `DESKD=1`, in which case the cell is checked, stood
and torn down exactly as a k8s cell's is.

`desk` and `up` on a house cell differ from a k8s cell in three ways and no others: no `deskd`
window or "deskd is not up" notice unless `DESKD=1`; `DESK_ROOTS` is exported from `CELL_ROOTS`
(on a k8s cell too, when `cell.env` carries one); and `DESK_SESSION` — also the `claude --name` —
is `<cell>-<role>-<UTC boot stamp>` rather than `<cell>-<short role>`, because house windows are
re-booted by hand across days and the roster beacon should tell one boot from the next. The
worktree, the shims, the pinned model and the `/assay:<role>` first prompt are the same code path.

---

## Scrubbed cells — `--kind scrubbed`

A house cell keeps the operator's real `HOME` on purpose (*Why the session keeps the real `HOME`*
above) — the desks ARE the operator's desks. A **scrubbed** cell is the opposite case: a harness
that must run on THIS laptop but must NOT inherit anything of the launching shell — no real
`HOME`, no SSH agent, no forge or model credentials, no cluster access — because the session
carries no standing trust of its own. Where a house cell's config home is a **symlink** to the
operator's, a scrubbed cell's config home is a **real directory**, scoped to exactly one repo,
with its own harness login. Nothing is copied from the operator's config at `new` time, and
nothing crosses at boot time beyond an explicit allowlist.

```bash
cellctl new scr1 --kind scrubbed \
  --repo /path/to/checkout \
  --repo-slug example-org/example-repo \
  [--roots 'example-org/example-repo=/path/to/checkout,...'] [--roles "worker-desk"]
# hand steps (see the scaffolded README): copy the role PEM(s) in as regular 0600 files,
# log the harness in under the cell's own home
cellctl check scr1
cellctl smoke scr1                    # one-shot, tool-free, read-only readiness probe
cellctl desk scr1 worker-desk         # the one role this cell boots at a time
cellctl status scr1                   # running <session> | stopped | stale-lock <pid>
cellctl down scr1
```

`new --kind scrubbed` needs `--repo` (a git checkout) and `--repo-slug` (`<owner>/<repo>` — the
ONE repo this cell is scoped to). It writes `cell.env` (`CELL_KIND=scrubbed`, `CELL_REPO`,
`CELL_REPO_SLUG`, optionally `CELL_ROOTS`), creates `home/.config/assay/` as a REAL directory
(mode 0700, never a symlink) carrying a `roster.env` skeleton whose `ASSAY_ALLOWED_REPOS` is
exactly the slug, and creates `home/.config/gh/` and an empty `home/.gitconfig` — real files, not
links. It refuses an existing cell of the same name, exactly like every other kind.

### The composed environment

The launch is `env -i` plus an explicit allowlist — the parent shell contributes nothing by
default. This is the exact list `cellctl` exports (the `SCRUBBED_ENV_KEYS` variable in the script
is the single source this table, the `[plan]` lines below, and the live launch all trace back to):

| Variable | Value |
|---|---|
| HOME | `<cell>/home` |
| ZDOTDIR | `<cell>/home` |
| SHELL | the bash `cellctl` itself runs under (never the operator's login shell) |
| PATH | `<cell>/shim:<desk-tools bindir>:<dir of the resolved harness binary>:/usr/bin:/bin:/usr/sbin:/sbin` — exactly seven elements; `cell.env`'s `CELL_PATH` overrides only the trailing system part, never the shim prefix or the harness dir |
| TMPDIR | `<cell>/tmp` (created 0700 if absent) |
| KUBECONFIG | `/dev/null` |
| ASSAY_CONFIG_HOME | `<cell>/home/.config/assay` |
| GH_CONFIG_DIR | `<cell>/home/.config/gh` |
| GIT_CONFIG_GLOBAL | `<cell>/home/.gitconfig` |
| GIT_CONFIG_NOSYSTEM | `1` |
| GIT_TERMINAL_PROMPT | `0` |
| CODEX_HOME | `<cell>/home/.codex` — codex arm only |
| CLAUDE_CONFIG_DIR | `<cell>/home/.claude` — claude arm only |
| CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION | `false` — claude arm only |
| DESK_LOOP | the role (or `smoke`) |
| DESK_SESSION | `<cell>-<role>-<UTC boot stamp>[-codex]` |
| DESK_ROOTS | `CELL_ROOTS`, when `cell.env` carries one |
| TERM | passed through from the parent — a TUI harness needs it |
| LANG | passed through from the parent |

`CODEX_HOME` and `CLAUDE_CONFIG_DIR` are mutually exclusive — only the active harness's variable
is ever exported. No `SSH_AUTH_SOCK`, no `GH_TOKEN`, no `ANTHROPIC_*`, no `AWS_*`, and no
wholesale parent `PATH` — the harness's own directory is the ONE parent-derived `PATH` element,
resolved once (`command -v`) before the launch switches to `env -i`, and named rather than
inherited wholesale.

The App PEM reaches the tools through the cell, never the parent shell: exporting
`ASSAY_CONFIG_HOME` is the whole custody path, since the desk tools resolve a role's key from
`<config-home>/<role>-app.pem`. `check` proves every PEM present under the cell's config home (or
named by its `apps.env`) is a regular, non-symlink, mode-0600 file — a house-style symlink into the
operator's real config home is a MISS on this kind, by design — and that the config-home directory
itself is mode 0700 (a group- or world-readable directory holding 0600 PEMs still exposes their
names and mtimes).

### `smoke` — a one-shot, tool-free, read-only readiness probe

```bash
cellctl smoke <cell> [--harness <claude|codex>] [--model <m>]
```

Separate from `check`: `smoke` actually asks the harness something, in the composed environment,
and passes iff the harness exits 0 AND its last non-empty stdout line is exactly `READY`. The
prompt is the literal `Reply with the single word READY and nothing else.` — nothing that invokes
a tool. codex arm: `codex exec --ephemeral --sandbox read-only --skip-git-repo-check -m <model>
"<prompt>"` — the sandbox flag enforces "tool-free, read-only" in the argv itself. claude arm:
`claude -p --model <model> "<prompt>"` — `-p`/`--print` is non-interactive with no session, plus
the tool-free prompt; there is no claude-side argv sandbox equal to codex's `--sandbox read-only`
here, so the isolation rests on those two things together. Both arms print the resolved model
before first contact. `DRY_RUN=1 cellctl smoke <cell>` prints the plan and the argv and runs
nothing. Live `smoke` is never something a Verify row runs — every row in this project runs it
against a stub harness.

A failure prints `smoke: not ready: <the harness's last line>` and exits 1 — a wrong-but-well-
formed answer (the harness exits 0 but says something other than `READY`) is exactly as much a
failure as a nonzero exit.

### `status` — a read, not a check

```bash
cellctl status <cell>
```

Prints exactly one of `running <session>`, `stopped`, or `stale-lock <pid>` (a lock directory
naming a pid that is no longer alive — reported here, cleared only by `down`), exit 0 in every
case that is not a load error.

### The session lock — two independent layers

A scrubbed cell is single-occupancy: one role, one harness process, at a time.

1. The tmux session lives on a **private socket** (`<cell>/run/tmux.sock`, one session named
   `<cell>-cell`) — a `new-session` on an existing name refuses, so a second cockpit cannot attach
   as a second owner, and nothing about this socket is shared with the operator's own default tmux
   server.
2. A **lock directory** (`<cell>/run/lock.d`, holding `pid`) taken by atomic `mkdir` — never
   `flock(1)`, which is absent from macOS by default — before the exec, held for the life of the
   harness process, released by `down`. A second `desk` while the held pid is still alive is
   refused, exit 4: `cell <cell> is already running (pid <n>, session <name>); cellctl down <cell>
   to release`. A lock naming a pid that is no longer alive is taken over rather than left to wedge
   every future boot — `status`/`down` are the tools that report or clear it explicitly for an
   operator who is just looking.

`desk` attaches when stdin is a tty; otherwise it prints the attach line
(`tmux -S <cell>/run/tmux.sock attach -t <cell>-cell`) and returns without blocking. `down` kills
the private-socket session and clears the lock; the workspace under `<cell>/worktrees/` is KEPT.

### `[plan]` — the dry-run grammar

`cmd_desk`'s existing `[dry-run] cell=… kind=…` line stays, unchanged, on every kind. The scrubbed
arm ADDS, immediately after it, under `DRY_RUN=1`: one `[plan] env KEY=VALUE` line per exported
variable (`KEY`s sorted), then one `[plan] argv <shell-quoted argv>` line, then `[plan] cwd <path>`,
then `[plan] lock <lock dir>`. This grammar is a contract, not a courtesy — a later parity harness
(desk-containers/10) diffs against it.

### `check` — the stricter rows

On top of the generic preconditions (*`cellctl check` — the preconditions* below), a scrubbed cell
proves: the config home is a REAL directory (never a symlink) at mode 0700; every PEM present is
regular, non-symlink, mode 0600; the roster's `ASSAY_ALLOWED_REPOS` is EXACTLY the cell's
`CELL_REPO_SLUG` (more than one entry, a different entry, or empty is a MISS); the harness is
logged in UNDER THE CELL HOME — codex via `codex login status` with `CODEX_HOME=<cell>/home/.codex`,
claude via a presence check of `<cell>/home/.claude` plus `claude --version` under
`CLAUDE_CONFIG_DIR=<cell>/home/.claude`; the roster parses under the cell home; every `CELL_ROOTS`
entry (when set) exists and carries `docs/streams/`; the desk verbs are installed; and the lock/run
directory is writable. The generic `codex harness preconditions` block (*`cellctl check` — the
codex harness block*) still runs unconditionally too, exactly as on any other kind.

A registration carrying a retired kind (`CELL_KIND=local`, the out-of-tree bridge this brief
retires the reason for) is not silently accepted: `load_cell` refuses it, exit 3, naming every
known kind.

---

## `cellctl check` — the preconditions

```bash
cellctl check <cell>
```

One `ok` / `MISS` line per precondition, exit 1 if any row missed: the checkout named by
`CELL_REPO`, the cells slice, the operator config home, `roster.env`, that every App-key symlink
under `home/.config` resolves (a dangling symlink is the common outcome of step 2), the configured
forge endpoint, `bin/deskd` and `bin/deskcli`, the desk-tools bindir, `tmux`, the resolved
cockpit and its reason (see *Cockpits*), whether this cell's `deskd` answers on its address, and —
when `CELL_HARNESS=codex` — the codex harness block (see *Harnesses*): `codex` on `PATH`, a
working `--version`, authentication, `multi_agent`, the resident-rules fragment, and skills
discoverability. A claude cell (the default) reports that block `n/a`, never silently skipped.

**One `model pin: role=<role> harness=<harness> model=<m> (from <source>)` row per role the cell
runs (`ROLES`)** (`#986`) — what `cellctl desk <cell> <role>` (no `--model`) would resolve to on
this cell's pinned `CELL_HARNESS`, via the same namespace/tier chain a live boot uses (see *Per-harness
namespaces and the tier-map fallback*). `<source>` names exactly which key produced the value —
`DESK_MODEL_the_desk`, `CODEX_MODEL_default`, or `tier:top (TIER_MODEL_TOP_CODEX)` — so a namespace
mismatch (a role with no per-harness pin and no tier match) is a `MISS` naming what was checked,
surfaced HERE before boot rather than discovered as a startup failure. This is in addition to, not a
replacement for, the dedicated `the-desk model: …` row the Opus refusal has always printed.

The forge-specific preconditions are keyed on the cell's `CELL_FORGE`. A **github** cell also
checks `apps.env`, the linked `gh` config, the readable `deskd` App key, and `ORGS`. A **gitlab**
cell instead checks the `GITLAB_GROUP`, the role token store directory, and the readable
`gitlab-deskd.token`. A precondition that belongs to the **other** forge is reported explicitly —
`n/a` where it does not apply, or `MISS` for a stray artifact of the wrong forge (a GitHub App PEM
on a gitlab cell) — never silently skipped, so a half-provisioned or mis-forged cell reads as such
rather than clean.

Run it after `new`, and again after any key or token rotation.

---

## `cellctl deskd` — attended only

```bash
CELL_ATTENDED=1 cellctl deskd <cell>
```

`deskd` serves live cross-org reads on freshly minted GitHub App installation tokens. `cellctl`
**refuses to start it** without `CELL_ATTENDED=1`, so minting is never something a background job
does on its own — the operator starts it from their own shell, knowingly.

What it does, in order:

- signs a short-lived JWT with the cell's `deskd` App key (`DESKD_APP_PEM`) and the App id read from
  `apps.env` under the variable named by `DESKD_APP_ID_VAR`;
- mints **one installation token per org** in `ORGS`, exported as
  `DESKD_GITHUB_TOKEN_<ORG>`. One token for several orgs does not work and does not fail cleanly: an
  installation token is scoped to its own installation, so it returns `404` for every other org's
  repos, which reads as "repo missing" rather than "wrong token". Tokens are 1-hour and are never
  echoed;
- runs a **go/no-go**: `deskd --once` across the cell's repos. A bad slice or a missing installation
  fails here, before anything is served;
- `exec`s the persistent `deskd` on `DESKD_ADDR` against `DESKD_INDEX`.

The index is persistent, so a restart resumes rather than re-crawls. Because the tokens expire in an
hour, `deskd` is a foreground process you restart when it stops — not a daemon you install.

### How far the attended affirmation carries

`cellctl up` stands `deskd` for you, which means it passes that affirmation on. It may only do so
when **`up` is itself attended**, or the flag would widen from "an operator ran this" to "anything
that ran `cellctl` at all" — the opposite of what it is for. Attended means one of two positive
signals:

- a **terminal on stdin** — an operator at their own shell; or
- **`CELL_ATTENDED=1` already set** in the environment — the explicit form, for a wrapper that has
  its own affirmation.

A cron or CI invocation has neither. There `up` opens the role windows but **does not stand
`deskd`**, prints why on stderr, and leaves the `deskd` window holding the command to run by hand.
Neither outcome is silent: the attended path announces that it is carrying your affirmation, and the
unattended path announces that it is not.

---

## `cellctl desk`, `up`, `down`

**One window:**

```bash
cellctl desk <cell> <role> [--model <m>] [--set] [--provider <name>] [--harness <claude|codex>] [CLAUDE_CONFIG_DIR]
```

`<role>` is one of `the-desk`, `intake-desk`, `worker-desk`, `pr-review-desk`, `verify-desk`. The
config dir defaults to `$CLAUDE_CONFIG_DIR`, then `~/.claude`, and is resolved against the real
`HOME`. `DRY_RUN=1` prints the plan — cell, role, config dir, worktree, session name, shim target,
model, provider and harness — and touches nothing. `--model` and `--set` are the per-run override
and the sugar that persists it — see *Pinned models* below for the full shape. `--provider`
switches the model **endpoint and credential** (`--model` alone only changes the model *name*, and
still talks to Anthropic) — see *Providers* below. `--harness` is the same per-run-override shape
for the harness — see *Harnesses* below.

Each window gets: its own worktree under `worktrees/<role>` **merged up to `origin/main`** (see
*Worktree currency* below) and
**locked** (`git worktree lock`, so a worktree prune never takes a live window's tree); the real
`HOME` with `shim/` first on `PATH`; `DESK_LOOP` and `DESK_SESSION` set, and `DESK_ROOTS` when
`cell.env` carries `CELL_ROOTS` (a cell without one boots with a notice that the desk verbs are on
their compiled placeholder topology); and its pinned model. What runs on top of that depends on
the **harness** (`CELL_HARNESS` in `cell.env`, default `claude`; `--harness` overrides it for this
run only) — see *Harnesses* below for the full shape. The claude arm is unchanged: the
`assay@assay` plugin is enabled in the config dir and the window opens on `/assay:<role>` as its
first prompt.

When the installed desk-tools ship a `deskwt role-init` that supports the role (probe: `deskwt
role-init --help` exits 0), `cellctl desk` lets **it** create the role worktree on first boot — its
last output line is the path — and links `worktrees/<role>` to that tree, so cellctl and the desk
skills agree on the worktree's name and the next boot merges the same tree up to main. A `deskwt`
that is absent or refuses the probe leaves cellctl's own worktree path in charge; `CELLCTL_DESKWT=0`
forces that path.

**Worktree currency.** An existing role worktree is brought up to the fetched `origin/main` with a
real merge — a fast-forward when the tree carries nothing of its own, a two-parent merge commit
when it does — never a rebase, and never left behind. (The earlier `--ff-only`-or-notice arm booted
the desk on whatever the tree was: a role worktree that had ever carried a local commit could never
fast-forward again, so every later boot ran days behind main, silently, until the desk's first
board read said `STALE:drift` and its loop refused every flip — #1157.) On a conflict the generated
single-writer files — `STATUS.md` and `docs/streams/FINDINGS.md`, overridable as the
space-separated `CELLCTL_GENERATED_FILES` — are taken from main outright, never hand-merged; any
**other** conflict **stops the boot**: the merge is aborted, the tree is left exactly as it was,
the conflicting paths are named, and nothing is launched. Resolve by hand (`git -C <worktree>
merge <sha>` — merge, never rebase), then re-run. Inside the session, `deskboot`'s own
`worktree-current` step re-proves the same thing against the same `FETCH_HEAD` and refuses a tree
that is behind, so a hand boot that skipped the launcher is caught too.

Each window is named **`<cell>-<short role>`** — the role without its `-desk` suffix, except
`the-desk`, which keeps its full name (`<cell>-the-desk`, `<cell>-pr-review`, `<cell>-verify`,
`<cell>-intake`, `<cell>-worker`). That one string is used for **both** surfaces: `DESK_SESSION`, the
roster beacon, and `claude --name`, the session's display name — so the cell's coordinator sees the
same identity in the roster and in its agent listing rather than two names for one window. A
non-claude harness gets a **`-codex`** suffix on that name (`<cell>-<short role>-codex`, or
`<cell>-<role>-<UTC stamp>-codex` on a house cell), so which harness a window is on is visible from
the roster the same way the model is.

Two details in the boot are there for a reason. The shared `fetch` is **serialised with a lock
directory**, because several windows starting at once fetch the same `.git` and race on the ref lock.
And a `fetch` that exits non-zero from that race has still written `FETCH_HEAD` — which is the only
thing the boot reads — so it is reported as a notice and the boot continues.

**The whole cockpit:**

```bash
cellctl up <cell> [--no-the-desk] [--no-attach] [--cockpit auto|tmux|herdr|orca] \
                  [--automate '<cron>'] [--model <m>] [--provider <name>] [--harness <claude|codex>] \
                  [CLAUDE_CONFIG_DIR]
```

One window per role in `ROLES`, plus a `deskd` window (watching `/healthz` if it is already up,
otherwise standing it — see *How far the attended affirmation carries* above), in whichever
**cockpit** resolves for this run — see *Cockpits* below. On the default tmux cockpit that is a
session named `<cell>-cell`, and `up` attaches unless you pass `--no-attach`; re-attach later with
`tmux attach -t <cell>-cell`. Windows start two seconds apart in every cockpit, so they do not all
arrive at the fetch lock together.

`--model <m>` applies the per-run override (see *Pinned models* below) to **every** role window this
run opens, the-desk included — there is no per-role `--model-<role>` form, since that case is
already `cellctl desk <cell> <role> --model <m>` on the one window that needs it. It is a live
cockpit's `cellctl desk <cell> <role>` invocation itself that carries `--model`, so the window that
actually boots resolves the same override the plan named. `--provider <name>` threads onto every
role window the same way, for the same reason — see *Providers* below.

`DRY_RUN=1 cellctl up <cell>` prints the resolved cockpit and the per-role commands and launches
nothing. `--harness <h>` overrides `CELL_HARNESS` for **every** role window this run opens (the-desk
included — there is no per-role `--harness-<role>` form) by threading `--harness <h>` onto each
role's own `cellctl desk <cell> <role>` invocation; `[dry-run] harness=<h> — applied to every role
window below` announces it once. See *Harnesses* below.

**`the-desk` is a default window, not an opt-in.** It is first in `ROLES_DEFAULT`; if a hand-edited
`cell.env` `ROLES` omits it, `up` prepends it anyway, and the `the-desk` window is the one selected
when the session comes up. `--no-the-desk` opts out and leaves the four loop roles. `--with-the-desk`
is accepted and does nothing — it was the flag when the coordinator was opt-in, and silently
ignoring it is better than failing a command that asks for what already happens.

Each window re-invokes `cellctl` by its **absolute** path, resolved once at startup from
`BASH_SOURCE`. A tmux window runs in the cell directory, where the relative `$0` a shell-invoked
script carries (`./cellctl`) does not resolve.

**Down:**

```bash
cellctl down <cell> [--keep-deskd] [--cockpit auto|tmux|herdr|orca]
```

Kills the session and this cell's `deskd` — matched on its own `--config` path, so another cell's
`deskd` on the same laptop is untouched. `--keep-deskd` leaves it standing. The tmux session is
torn down whichever cockpit resolves, and what a non-tmux cockpit opened is closed where that
cockpit offers a verb for it and **named for you to close by hand where it does not**.

**List:** `cellctl ls` prints the cells under the cells root. No cells is not an error.

**Persisting a `cell.env` change:** `cellctl set <cell> KEY=VALUE [...]` — see *Pinned models* below
for the common case (a model pin) and *`cell.env`* below for the full key list.

---

## Cockpits

A **cockpit** is only the surface the role windows appear in. Nothing else about a cell changes
with it: the per-role locked worktree, the roster beacon, the pinned model, the shim `PATH` and the
`/assay:<role>` first prompt are identical in all three, and every window runs the same command —
`cellctl desk <cell> <role>`. No cockpit is ever required.

```
CELL_COCKPIT=auto        # cell.env: auto (default) | tmux | herdr | orca
cellctl up <cell> --cockpit herdr    # override for this run
```

### What `auto` picks, and why

`auto` resolves by **presence on PATH** — never a flag someone has to remember, the same stance
the per-item worktree arm takes:

1. **`herdr` on PATH** → herdr. It has labelled tabs and a semantic agent state
   (`idle | working | blocked | done`), so the five desks show what they are each doing in its
   sidebar rather than as five anonymous panes, and its background server keeps them alive.
2. **else `orca` on PATH *and reachable*** → orca. Its CLI is a thin client of its desktop app, so
   an `orca` binary alone is not enough: `cellctl` runs a cheap read verb first, bounded natively —
   `timeout`/`gtimeout` when either is on PATH, else a background-and-watchdog fallback that needs
   no external binary — so the probe returns within the bound regardless of how the CLI itself
   behaves when its app is closed, and an app that does not answer **falls through** rather than
   failing.
3. **else** → tmux, the always-works arm.

An **explicit** cockpit — `--cockpit <v>` or `CELL_COCKPIT` — that is not available is a refusal
naming exactly what is missing (`herdr is not on PATH`; `orca is on PATH but its desktop app is not
reachable`), never a silent fall-through to something else. `--cockpit` beats `cell.env`, which
beats the default.

### Which one was chosen

Every `up`, `down` and `check` says so, in one line, with the reason:

```
[cockpit] herdr (auto: on PATH)
[cockpit] tmux (fallback: no herdr/orca on PATH)
[cockpit] tmux (orca on PATH but app unreachable)
[cockpit] orca (explicit: --cockpit)
```

`cellctl check <cell>` carries the same resolution as a precondition row, and states orca's
reachability whenever orca is installed — so "which surface will my windows appear in, and why" is
answerable before booting rather than after.

### The three shapes

| Cockpit | What `up` opens |
|---|---|
| **tmux** | a session `<cell>-cell`: the `deskd`/`cell` window plus one window per role. Unchanged |
| **herdr** | one **labelled tab per window**, `<cell>-<role>`, each fed the same `cellctl desk <cell> <role>` command every other cockpit runs (via `herdr pane run`, into a pane the tab's own `tab create` cut). The first (`deskd`/`cell`) window is a tab too. If herdr has no window (a "workspace" in herdr's own grammar) open yet, `up` starts one first — the same trigger point and create-if-absent shape as tmux's `<cell>-cell` session — before any tab lands; an already-open window is unchanged |
| **orca** | one **terminal per role** under the cell directory (registered with Orca via `orca repo add` first — Orca 404s an unregistered path), running the same `cellctl desk` command; or, with `--automate`, one scheduled automation per role instead |

### The `--automate` recipe (orca only)

```bash
cellctl up <cell> --cockpit orca --automate '*/30 * * * *'
```

One automation per role on that trigger, each fronted by an **exit-code precheck** —
`cellctl check <cell>`, which is exit-code honest (0 when every precondition holds, 1 when one does
not). A tick on a cell that is not fit to boot records a skipped run and launches no model at all.
`--automate` is refused on any other cockpit rather than quietly ignored.

Two limits worth knowing before you use it. A scheduled run is launched by the cockpit, not by
`cellctl`, so it does **not** carry the cell's shim `PATH`, `DESK_ROOTS` or pinned model — use the
live-terminal shape where those matter. And automations **outlive `cellctl down`** on purpose:
taking the windows down is not the same act as cancelling a schedule, so `down` names them rather
than deleting them.

### Fall-through rules

These cockpit CLIs move fast, so every verb and flag whose spelling `cellctl` cannot see is
**probed from `--help` at run time**, never hard-coded as a truth that may have drifted:

- herdr hosts the composite `cellctl desk` command via `herdr pane run <pane_id> <cmd>`, on a pane
  from `herdr tab create` — **not** `herdr agent start`, whose `-- AGENT_ARG...` list is appended
  directly to the KIND's canonical executable rather than run as a wrapping shell command (verified
  live against herdr 0.8.2), so it cannot host cellctl's fetch-worktree-shim-then-exec sequence. A
  build with no `tab create` or no `pane run` is a refusal naming `--cockpit tmux`, because nothing
  could host a window; `herdr down` looks up each window's tab by label (`herdr tab list`) and
  closes it by `tab_id` (`herdr tab close <tab_id>` — real herdr has no `--label` on `close`);
  before any of that, `up` checks `herdr workspace list` (herdr's own noun for what this file calls
  a "window") and, finding none open, brings one up itself with
  `herdr workspace create --label <cell>-<the first window>` — mirroring the tmux arm's
  `tmux has-session || tmux new-session` — before creating a single tab, dropping the workspace's
  own auto-seeded default tab once the cell's real tabs exist in it. A build that cannot list or
  create workspaces, or whose `workspace create` fails, is a refusal naming herdr and the exact
  command tried, never a silently-opened nothing (#985);
- orca's create-a-terminal verb and its command / worktree-selector / name flags are read from its
  own help (`--worktree path:<dir>`, not `--cwd` — orca advertises no such flag on this verb). Where
  the verb or the command flag is absent, `cellctl` **prints the exact per-role commands to run by
  hand** and refuses, rather than guessing a spelling; `orca down` closes everything for the cell in
  one call (`orca terminal close --worktree path:<cell-dir> --all`) when the build advertises it;
- `--automate` refuses unless `orca automations create` advertises the flags the shape depends on —
  above all `--precheck`, which is the whole point of it;
- anything a cockpit cannot close on `down` is **named**, never left unsaid.

In every one of these cases `--cockpit tmux` is the answer that always works.

---

## Pinned models

**Every role window launches on a model named in `cell.env`. The CLI default is never used.** A
default that moves under a running cell changes what five long-lived loops do without anything in the
cell saying so, which is the kind of drift a cell exists to keep out.

```
DESK_MODEL_DEFAULT=sonnet     # every role that has no override
DESK_MODEL_the_desk=fable     # per-role override: the role name with `-` replaced by `_`
```

An override is `DESK_MODEL_<role>` with hyphens replaced by underscores — `DESK_MODEL_the_desk`,
`DESK_MODEL_pr_review_desk`, `DESK_MODEL_verify_desk`, and so on. Values are whatever
`claude --model` accepts: an alias (`fable`, `opus`, `sonnet`, `haiku`) or a full model id, including
a long-context variant. `DESK_MODEL_DEFAULT` itself falls back to `sonnet` if `cell.env` omits it, and
`cellctl new` scaffolds both lines above so a fresh cell is pinned from the start.

**Why the defaults are shaped that way.** The four loop roles are mechanical dispatchers: they read a
board, claim an item, open a worktree, and hand the actual judgment to the agent they dispatch. The
coordinator window is where judgment happens in the loop itself. So the loops get the cheaper model
and `the-desk` gets the top tier available — and the loops' pin can be moved per cell, which is the
point of putting it in `cell.env` rather than in the script.

**The coordinator's pin cannot be moved down to Opus.** `opus` is no longer the top tier, and the
coordinator role is defined to run on whichever model is. `cellctl desk <cell> the-desk` (including
its `DRY_RUN=1` plan) and `cellctl check` both refuse a resolved `DESK_MODEL_the_desk` that is the
`opus` alias or a `claude-opus-*` id — whether that value came from `DESK_MODEL_the_desk` itself or
fell through to `DESK_MODEL_DEFAULT` — and print the value plus the variable to change. Every other
role's pin, including `opus`, is untouched.

`cellctl desk` prints the resolved model on its launch line and in `DRY_RUN=1` output, so which model
a window is on is visible without reading the config.

### Per-harness namespaces and the tier-map fallback (`#986`)

**Every harness keeps its own pin namespace — a Claude model name means nothing to Codex, and vice
versa.** `DESK_MODEL_<role>` / `DESK_MODEL_DEFAULT` are the **claude** namespace, unchanged from
above. Codex gets its own: `CODEX_MODEL_<role>` (per-role override) and `CODEX_MODEL_default` (the
harness-wide fallback, no compiled default — see the tier map below). The two namespaces are never
cross-read: pinning `DESK_MODEL_the_desk=fable` says nothing about what `--harness codex` resolves
to, and pinning `CODEX_MODEL_the_desk` says nothing about the claude arm.

```
DESK_MODEL_DEFAULT=sonnet        # claude namespace — every role, absent a per-role override
DESK_MODEL_the_desk=fable        # claude namespace — per-role
CODEX_MODEL_default=gpt-5.6-terra  # codex namespace — every role, absent a per-role override
CODEX_MODEL_the_desk=gpt-5.6-terra # codex namespace — per-role
```

**The tier-map fallback.** A role with *neither* its harness's per-role pin *nor* that harness's
`_DEFAULT`/`_default` falls back to a TIER: `the-desk` resolves at **top**, every other role at
**mid** (the same "coordinator gets the top tier, loops get the cheaper one" split the Why section
above already documents — `fast` exists in the table but is not assigned to any role automatically;
pin a role there directly if you want it). This is what lets a cell pinned Claude-only today (the
common case — every cell `cellctl new` scaffolds is claude-only until an operator adds Codex pins)
boot `--harness codex` with a real, working model name and no manual re-pin:

| Tier | claude | codex |
|---|---|---|
| top (`the-desk`) | `fable` | `gpt-5.6-terra` |
| mid (every other role) | `sonnet` | `gpt-5.6-terra` |
| fast (not auto-assigned) | `haiku` | `gpt-5.6-terra` |

These are the **compiled defaults** (`gpt-5.6-terra` is the one codex model id proven live against
a real build — `docs/codex-smoke-runs/2026-09-12-codex-0.154.0.md` — used for all three codex tiers
until a confirmed cheaper/faster id replaces one of them). Every entry is overridable in `cell.env`
by its own key, the same one-key-one-value override shape `DESK_MODEL_<role>` and
`CELL_PROVIDER_<NAME>_*` already use elsewhere in this file:

```
TIER_MODEL_TOP_CLAUDE=fable
TIER_MODEL_MID_CLAUDE=sonnet
TIER_MODEL_FAST_CLAUDE=haiku
TIER_MODEL_TOP_CODEX=gpt-5.6-terra
TIER_MODEL_MID_CODEX=gpt-5.6-terra
TIER_MODEL_FAST_CODEX=gpt-5.6-terra
```

Resolution order, per role and per the ACTIVE harness (`resolve_role_model` in the script): (1) that
harness's own per-role pin, (2) that harness's own default, (3) the tier map, by this role's tier
and the harness's own column. Claude always resolves at step 2 today — `DESK_MODEL_DEFAULT` carries
a compiled default (`sonnet`) — which is exactly what keeps `--harness claude` unaffected by any of
this. A role/harness with nothing at any of the three steps (an operator has, deliberately or by
typo, blanked a `TIER_MODEL_<TIER>_<HARNESS>` entry to the empty string, and set neither the
per-role nor the default pin either) is a clean refusal — `cellctl desk` dies naming every place it
checked, rather than launching a harness on an empty or bogus model name; `cellctl check` surfaces
the same gap as a MISS before boot (see *`cellctl check` — the codex harness block* is a different
row — the per-role rows are covered right below it in *`cell.env`*'s table and by the *`cellctl
check`* section above).

**`--model <m>` is never routed through any of this.** An explicit override — flag or
`DESK_MODEL_OVERRIDE` — passes straight to the selected harness verbatim, on either harness,
exactly as *`--model` — a per-run override* below describes; it is not a namespace or a tier, and
bypasses both.

**`cellctl set` writes to whichever namespace is ACTIVE.** The role-sugar form
`cellctl set <cell> <role> [--harness claude|codex] --model <m>` computes the KEY itself from the
harness given (or, absent `--harness`, the cell's own `CELL_HARNESS`) — `CODEX_MODEL_<role>` on
codex, `DESK_MODEL_<role>` on claude — so `cellctl set <cell> the-desk --harness codex --model X`
writes `CODEX_MODEL_the_desk=X`, never `DESK_MODEL_the_desk`. The plain `KEY=VALUE` form (`cellctl
set <cell> CODEX_MODEL_the_desk=X`) still works too — the role-sugar form is convenience, not the
only path. See *`cellctl set` — persisting a change* below for the shared mechanics (backup,
before/after, `--force`).

### `--model` — a per-run override

Running one role (or a whole cell) on another model for a while does not have to mean hand-editing
`cell.env` first:

```bash
cellctl desk <cell> <role> --model <m>     # this ONE window, this run only
cellctl up   <cell>        --model <m>     # every role window this run opens
DESK_MODEL_OVERRIDE=<m> cellctl desk <cell> <role>   # the equivalent env form, for wrappers
```

`--model <m>` **wins over** `DESK_MODEL_<role>` and `DESK_MODEL_DEFAULT` for that invocation only —
it never touches `cell.env`. `DESK_MODEL_OVERRIDE` in the environment does the same thing for a
wrapper that cannot pass a flag; an explicit `--model` wins when both are given. `cellctl up
--model <m>` threads the override onto **every** role window it opens (the-desk included) by
passing it on to each role's own `cellctl desk <cell> <role> --model <m>` invocation — there is no
per-role `--model-<role>` form, since a single role's override is already `cellctl desk <cell>
<role> --model <m>`.

The source of the value is never left implicit: the `[launch]` line and every `DRY_RUN=1` plan print
`model=<m> (override)` rather than just `model=<m>`, so the transcript shows whether a window is on
its `cell.env` pin or on a for-this-run override.

**The Opus refusal is not an escape hatch via `--model`.** `cellctl desk <cell> the-desk --model
opus` (or a `claude-opus-*` id, or `DESK_MODEL_OVERRIDE=opus`) is refused with exactly the same
message as an Opus **pin** — the rule binds the resolved model, whichever source produced it.

**`cellctl check` is never affected by `--model` or `DESK_MODEL_OVERRIDE`.** It has no `--model`
flag and does not read the env form, so its the-desk-model row always reports what a plain boot —
no override — would resolve to.

**`--model` changes the model *name* only.** A non-Anthropic model still needs
`ANTHROPIC_BASE_URL` / `ANTHROPIC_AUTH_TOKEN` set in the shell that runs `cellctl` — `cellctl`
**inherits** them (they reach the `claude` process it execs, same as any other environment
variable) but does **not** set, validate or manage them itself. Exporting the right pair for the
model you are overriding to is on you.

### `cellctl set` — persisting a change

```bash
cellctl set <cell> DESK_MODEL_the_desk=glm-5.3
cellctl set <cell> KEY=VALUE [KEY=VALUE...] [--force]
cellctl set <cell> <role> [--harness claude|codex] --model <m>   # role-sugar, harness-aware (#986)
```

Rewrites an existing `KEY=` line **in place** (comment lines and every other line's ordering
untouched) or **appends** a key that has no active line yet. Writes exactly **one backup**,
`cell.env.bak-<ts>`, before the first edit of a call — a multi-key `set` is one backup for the
whole call, not one per key. Refuses a key that is not a known `cell.env` key (see the table below;
the `DESK_MODEL_<role>`, `CODEX_MODEL_<role>`, `TIER_MODEL_<TIER>_<HARNESS>` families count as
known for each of the five roles / three tiers / two harnesses) unless `--force`, and a
refusal — unknown key **or** the Opus rule below — touches nothing: no backup, no edit. Prints each
key's before/after value. Applies the **same** the-desk/Opus refusal to `DESK_MODEL_the_desk` as a
live boot and `check` do (codex's `CODEX_MODEL_the_desk` is never subject to it — see *Harnesses*),
and `--force` does not bypass it — `--force` widens which *keys* `set` will touch, not which
*values* the Opus rule allows.

**The third form is role-sugar for the model pin specifically.** It computes the KEY itself from
the ACTIVE harness — `--harness` on this call if given, else the cell's own `CELL_HARNESS` — so
`cellctl set <cell> <role> --harness codex --model X` writes `CODEX_MODEL_<role>=X`, and the same
call with `--harness claude` (or no `--harness` on a claude-pinned cell) writes `DESK_MODEL_<role>=X`
— never the other namespace's key. It needs `--model`; a role name with no `--model` is refused, and
it cannot be combined with a `KEY=VALUE` pair in the same call. See *Per-harness namespaces and the
tier-map fallback* above for why the namespace choice matters.

**Sugar: override now and persist it in one call.**

```bash
cellctl desk <cell> <role> --model <m> --set
```

`--set` applies `--model <m>` for this run **and** persists it — equivalent to `--model <m>`
followed by `cellctl set <cell> DESK_MODEL_<role>=<m>`. Under `DRY_RUN=1` it prints what it *would*
persist and writes nothing — a dry run touches nothing, `--set` included.

**`CELL_HARNESS` is a known key too** — `cellctl set <cell> CELL_HARNESS=codex` persists the harness
pin the same way, with the same value check as the harness flag itself: only `claude` or `codex` is
accepted (not bypassable by `--force`, which only widens which *keys* `set` will touch).

### Every per-run choice — override for one run, `--set` to persist, `show` to read (`#1303`)

Since `#1303` the same shape covers **every** per-run choice, not just the model: the kind (how and
where the windows run), the cockpit, the harness and the provider.

```bash
cellctl desk <cell> <role> [--kind <k>] [--cockpit <c>] [--harness <h>] [--provider <p>] [--model <m>] [--set]
cellctl up   <cell>        [--kind <k>] [--cockpit <c>] [--harness <h>] [--provider <p>] [--model <m>] [--set]
cellctl set  <cell>        [--kind <k>] [--cockpit <c>] [--harness <h>] [--provider <p>]
cellctl show <cell>        [--kind <k>] [--cockpit <c>] [--harness <h>] [--provider <p>] [--model <m>]
```

- **Each flag overrides its `cell.env` key for that run only** (`CELL_KIND`, `CELL_COCKPIT`,
  `CELL_HARNESS`, `CELL_PROVIDER`, the model pin) without touching the file. `--kind` is applied
  *before* the cell loads — the kind's own preconditions are asserted at load, so `--kind house` on
  a cell with no `CELL_ROOTS`, or `--kind container` with no `CELL_CONTAINER_LAUNCHER`, refuses
  right there naming the missing key, exactly as a `cell.env` carrying that kind would. `--cockpit`
  on a single `desk` window is accepted (and persistable) but drives nothing in that window —
  a `desk` boot opens in the calling terminal; the cockpit is `up`'s concern. A per-run `--kind`
  is reported as `kind=<k> (override)` on the `[dry-run]`/`[launch]` lines.
- **`--set` persists EVERY override given on that invocation**, each to its own key, through the
  same one-backup path `cellctl set` uses (`cell.env.bak-<ts>` first, then each `[set] KEY:
  before -> after` line). An override not given is never re-written. On `desk`, `--model` persists
  to the ACTIVE harness's `<FAMILY>_MODEL_<role>`; on `up` it persists to that key for every role
  window the run opens (the-desk included unless `--no-the-desk`), since that is exactly the set of
  windows the override applied to. `--set` with nothing to persist is refused; the the-desk Opus
  rule and the kind precondition apply to the persisted values as they do to the run.
- **`cellctl set <cell> --kind/--cockpit/--harness/--provider`** is sugar for the matching
  `KEY=VALUE`, validated by the same rules: an unknown kind/cockpit/harness is refused before
  anything is written, and a kind change refuses when the target kind's own precondition
  (`container`: an absolute executable `CELL_CONTAINER_LAUNCHER`; `house`: `CELL_ROOTS`; `scrubbed`:
  `CELL_REPO_SLUG`) is neither already in `cell.env` nor given in the same call — so `cellctl set
  <cell> --kind house CELL_ROOTS=…` in one call passes while `--kind house` alone on an
  un-rooted cell does not, and nothing (no backup either) is written on the refusal. With a role
  name, `--harness` keeps its `#986` meaning — it *selects* the namespace for `--model` and is not
  itself persisted; without a role it is the `CELL_HARNESS` sugar. `--model` on `set` still needs a
  role (the harness-wide default is the `KEY=VALUE` form).
- **`cellctl show <cell>` is a read.** One greppable line per choice — `[show] KEY=VALUE (source)`
  with source `flag` (given on this invocation), `cell.env` (the file's own line) or `default`
  (cellctl's compiled fallback; the provider prints `unset (default: anthropic)`) — then one
  `[show] model <role>=<m> (source)` line per role, resolved exactly as a plain `desk` boot on the
  shown harness would (`cell.env <KEY>`, `default: <KEY>` for a compiled default, `default:
  tier:<t> (<KEY>)` for the tier map, `flag` for an explicit `--model`). It accepts the same flags
  as `desk`/`up`, so "what would this invocation resolve to" is answerable before booting it, and a
  `--kind` the cell is not provisioned for refuses just as `desk` would. Nothing is launched or
  written.

---

## Harnesses

**Every role window boots on a harness pinned in `cell.env`, the same way its model is.** Default
`claude`. `CELL_HARNESS=claude|codex`; `--harness <h>` on `cellctl desk`/`cellctl up` overrides it
for that run only, without touching `cell.env`.

```
CELL_HARNESS=claude          # cell.env: claude (default) | codex
cellctl desk <cell> the-desk --harness codex     # this ONE window, this run only
cellctl up   <cell>          --harness codex     # every role window this run opens
```

`cellctl up --harness <h>` threads the override onto **every** role window it opens (the-desk
included) by passing `--harness <h>` on to each role's own `cellctl desk <cell> <role> --harness
<h>` invocation — there is no per-role `--harness-<role>` form, the same shape the model override
uses. The source is visible without reading the config: `[dry-run]`/`[launch]` print `harness=<h>`,
and a non-claude window's `DESK_SESSION` carries a `-codex` suffix (see *`cellctl desk`, `up`,
`down`* above).

### The claude arm — unchanged

Exactly what this document already describes above: `claude --name <session> --model <model>
"/assay:<role>"`, with the `assay@assay` plugin enabled in the config dir first.

### The codex arm

```bash
codex --sandbox danger-full-access -C <worktree> -m <model> "Invoke the \"assay:<role>\" skill now."
```

The same exported env the claude arm gets — `DESK_LOOP`, `DESK_SESSION`, `DESK_ROOTS` (when
`cell.env` carries `CELL_ROOTS`), and `shim/` first on `PATH`. `CLAUDE_CONFIG_DIR` is irrelevant on
this arm and is not passed; the model comes from the **codex namespace** — `CODEX_MODEL_<role>` /
`CODEX_MODEL_default`, falling back to the tier map — never the claude arm's `DESK_MODEL_<role>` /
`DESK_MODEL_DEFAULT` (`#986`: the two were conflated before this, which is why a Claude-only pin
used to reach `codex -m` unchanged and fail there). See *Per-harness namespaces and the tier-map
fallback* above for the full resolution order.

**`--sandbox danger-full-access` is required, honestly.** Per the ruled capability matrix (the
`#937` live smoke run is the evidence codex CLI's default `workspace-write` sandbox blocks the
`.git/refs/heads/` write a fresh worktree needs — codex-cli 0.154.0 gained its own `--worktree`
flag, but a skill isolating via its own `git worktree add` call still hits the same
`workspace-write` refusal), the worktree this window runs in could not have been **created** under
a lesser sandbox in the first place. This is not a weakening introduced here — it is the existing
precondition `cellctl desk`'s own worktree-creation step depends on, stated rather than glossed
over. `#939` tracks where the capability matrix itself has drifted against newer codex
releases; that staleness does not change what this flag is *for* on this exec line.

**The-desk's Opus refusal binds the claude arm only.** `opus` / `claude-opus-*` is a Claude-family
alias with no meaning to codex, so on `--harness codex` the refusal never fires — the resolved
model prints as is, whatever it is. Pin the-desk to a sane default on a codex cell the same way you
would for claude; nothing stops a codex the-desk from being pointed at a nonsense model name, the
same as any other role.

**The resident-rules fragment.** Codex has no Claude-style `SessionStart` hook to carry the
methodology's resident operating rules, so on this arm they travel in `AGENTS.md` instead
(`plugins/assay/codex/AGENTS-assay.md`, generated from `resident-rules.md` — see
`docs/adopting-assay.md`'s *Running Assay on Codex*). `cellctl desk --harness codex` appends that
fragment to the **worktree's own** `AGENTS.md` at boot, idempotently (a marker-string check skips
the append when it is already present) — belt-and-suspenders alongside a checkout whose root
`AGENTS.md` already carries it, which `cellctl check` verifies separately (below).

### `cellctl check` — the codex harness block

Only asked for when `CELL_HARNESS=codex` (a claude cell reports the whole block `n/a`, never
silently skipped):

| Row | What it proves |
|---|---|
| `codex on PATH` | `command -v codex` |
| `codex --version` | the binary actually runs |
| `codex authenticated` | `codex login status` (or its `--json` form), whichever the installed build answers |
| `[features] multi_agent = true` | read via a `codex config get`-style verb when the build advertises one, else a literal read of `~/.codex/config.toml` (`CODEX_HOME` respected); a `-c` override on the invocation itself is not visible to this row, which the row's own text says |
| resident-rules fragment present | the checkout's own root `AGENTS.md` (`$CELL_REPO/AGENTS.md`) carries the fragment's marker text |
| skills discoverable | **either** the marketplace plugin arm (`codex plugin list` reports `assay`) **or** the file-placement arm (`.agents/skills/` holds at least one skill directory) — see `docs/adopting-assay.md`'s two Codex install arms |

Every codex CLI verb this block probes has moved across releases (per the codex-smoke-run
findings), so the probe is defensive: it tries the documented spelling first and never hard-codes
a single one as the only truth.

---

## Providers

**`--model` only changes the model *name*.** `claude --model glm-5.3` still talks to Anthropic and
fails, because the endpoint and the credential are separate settings `--model` never touches. A
**provider** is the missing piece: a named endpoint + credential pair, declared in `cell.env` and
switched with `--provider <name>` or `CELL_PROVIDER`.

```bash
# cell.env
CELL_PROVIDER=zai                                              # optional default (unset = Anthropic)
CELL_PROVIDER_ZAI_BASE_URL=https://api.z.ai/api/anthropic
CELL_PROVIDER_ZAI_TOKEN_ENV=ZAI_API_KEY                        # the NAME of an env var, never a token

cellctl desk <cell> worker-desk --provider zai                 # override for one run
cellctl up   <cell> --provider zai                              # every role window this run opens
```

A provider name (`zai`, anything — or a built-in preset, below) resolves to two `cell.env` variables,
`CELL_PROVIDER_<NAME>_BASE_URL` and `CELL_PROVIDER_<NAME>_TOKEN_ENV` (the name upper-cased, `-` as
`_`). The launched `claude` process gets `ANTHROPIC_BASE_URL` from the first and
`ANTHROPIC_AUTH_TOKEN` from `${!CELL_PROVIDER_<NAME>_TOKEN_ENV}` — the **value** of whichever
environment variable `_TOKEN_ENV` names, read from the shell that ran `cellctl`. **The token itself
is never written to `cell.env`** — only the name of the variable that carries it — so a leaked or
mistakenly public `cell.env` leaks no credential. Missing any piece (the provider unnamed, its
`_BASE_URL` unset, its `_TOKEN_ENV` unset, or the named variable itself unset in this shell) is a
refusal naming exactly what is missing, before anything launches.

`--provider` on `cellctl desk`/`up` overrides `cell.env`'s `CELL_PROVIDER` for one run, the same way
`--model` overrides a pin — it never edits `cell.env`. `cellctl set <cell> CELL_PROVIDER=zai` (and
the `CELL_PROVIDER_<NAME>_*` keys) persist a default the way any other `cellctl set` key does.

Both `cellctl desk`'s launch line and `DRY_RUN=1` plan print `provider=<name>` (or `provider=anthropic`
when none is set), so which endpoint a window is on is visible without reading `cell.env`.
`cellctl check` carries three rows for a cell's default `CELL_PROVIDER` (base URL declared, token-env
variable named, and that variable actually set in *this* shell) plus a model row — unset
`CELL_PROVIDER` is `n/a`, not a `MISS`, because a provider is opt-in. Every row prints the endpoint,
the env var's **name**, the model and *set*/*unset* — never a token value.

### Built-in presets: `kimi` and `glm` (`#1303`)

Two providers resolve with **no** `CELL_PROVIDER_<NAME>_*` line at all — the operator exports the
one env var and boots:

| Preset | `ANTHROPIC_BASE_URL` | token env NAME | default model |
|---|---|---|---|
| `kimi` | `https://api.kimi.com/coding` | `KIMI_API_KEY` | `k3[1m]` |
| `glm` | `https://api.z.ai/api/anthropic` | `ZAI_API_KEY` | `glm-5.3[1m]` |

```bash
export KIMI_API_KEY=…                                 # in your shell, never in cell.env
cellctl desk <cell> worker-desk --provider kimi       # this window, this run
cellctl up   <cell> --provider glm --set              # every window, and persist CELL_PROVIDER=glm
cellctl set  <cell> --provider kimi                   # persist without booting
cellctl show <cell> --provider kimi                   # the effective endpoint / env NAME (set|unset) / model
```

Any `CELL_PROVIDER_<NAME>_{BASE_URL,TOKEN_ENV,MODEL}` line overrides the matching preset value
**piecewise** (a proxy endpoint for `glm` with the preset's token env and model, say); a name with
no preset needs its own lines exactly as before. `cellctl check`/`show` tag each value `preset`,
`cell.env` or `unset`.

**`CELL_PROVIDER_<NAME>_MODEL` — the provider's model.** A provider window resolves its model as
`--model` (or `DESK_MODEL_OVERRIDE`) > the role's own `DESK_MODEL_<role>` pin > the provider model
(`CELL_PROVIDER_<NAME>_MODEL`, else the preset's) > the usual `DESK_MODEL_DEFAULT`/tier chain. The
harness-wide default and the tier map are Anthropic names a provider endpoint rejects, which is why
the provider model sits above them; a per-role pin is still honoured because it was set on purpose
(`cellctl new` pins the-desk to `fable`, so a provider the-desk needs that pin changed — or
`--model` — to run on the provider's model; `[dry-run]`/`show` print what resolved). `--set`
persists `--model` per the widened-scope rules above, into `DESK_MODEL_<role>`.

**What the launch exports with a provider active** (and touches not at all without one):

- `ANTHROPIC_API_KEY` is **unset** for the launched process — an inherited API key wins over the
  auth token and silently routes to Anthropic.
- `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN` as before.
- `ANTHROPIC_MODEL` = the model this window launches with, and
  `ANTHROPIC_DEFAULT_OPUS_MODEL` / `ANTHROPIC_DEFAULT_SONNET_MODEL` /
  `ANTHROPIC_DEFAULT_HAIKU_MODEL` = a name the endpoint accepts — the provider model (else the
  launch model), or that tier's per-tier model where one applies (next paragraph).

**Per-tier provider models** (`assay#1352`): a provider may name a DIFFERENT model for one tier —
`CELL_PROVIDER_<NAME>_MODEL_TOP` / `_MODEL_MID` / `_MODEL_FAST` (cell.env line, else the preset) —
and it applies in two places: a role window whose cellctl tier is that one launches on it (the
flat `MODEL` stays every other tier's default), and the matching launch alias
(`OPUS`→`MODEL_TOP`, `SONNET`→`MODEL_MID`, `HAIKU`→`MODEL_FAST`) maps to it, else to the flat
provider model. The built-in `glm` preset ships one: `MODEL_MID` = `glm-5.3-flash[1m]`, so on a
glm cell every mid-tier window and every sonnet ask inside any window runs the flash variant,
while the-desk (TOP) keeps the full `glm-5.3[1m]`. `cellctl check` prints the sonnet slot as its
own row when it differs. A per-role pin or `--model` still wins over both, verbatim as ever.

**Tier flags** (`--model-top` / `--model-mid` / `--model-fast`): override the provider's
per-tier models for ONE run — refused without a provider (the keys are provider-keyed), and
`--set` persists each given flag to its `CELL_PROVIDER_<NAME>_MODEL_<TIER>` key, making it the
cell's default. `cellctl up` accepts the same three flags and threads them onto every role
window it opens (via the `CELL_TIER_MODEL_<TIER>` environment), and the raw keys are settable
directly: `cellctl set <cell> CELL_PROVIDER_GLM_MODEL_MID=glm-5.3-flash[1m]`.

A per-tier provider model refines the launch path — it does not replace the flat model. The
launch `--model` resolves through the normal precedence (`--model` > per-role pin > provider flat
`MODEL` (preset or cell.env line) > harness default), so a provider configured with only a tier
key and no flat `MODEL` launches on that resolved default model NAME — the model name is
unaffected by the provider — while the per-tier aliases (`ANTHROPIC_DEFAULT_{OPUS,SONNET,HAIKU}_MODEL`)
still take the provider's tier/flat model. Configure the provider's flat `MODEL` too when you want
the launch model itself to be provider-sourced rather than the harness default. The one refusal
here that IS enforced: a tier flag or `CELL_TIER_MODEL_*` value reaching `desk` with no provider
at all is refused, the same way `up` refuses it — a tier model is provider-keyed and has nothing to
hang on without one.

A provider is a **claude-harness** seam: `--harness codex` with a provider (flag or `CELL_PROVIDER`)
is refused rather than launching codex against Anthropic with a provider the operator asked for.

---

## `cell.env`

`cellctl new` writes it; edit it directly afterwards.

| Variable | What it is |
|---|---|
| `CELL` | the cell name (also the tmux session prefix) |
| `CELL_KIND` | `k8s` (default — a cell scaffolded before kinds existed carries none), `house` (the operator's own desks; see *House cells*), `container` (see *Container cells*), or `scrubbed` (a composed environment; see *Scrubbed cells*) |
| `CELL_ROOTS` | the stream-root map, `<owner>/<repo>=<abs path>,...`, exported to every role window as `DESK_ROOTS`; **required** on a house cell, optional on k8s/scrubbed (unset = the verbs' compiled placeholder topology, and `desk` says so) |
| `CELL_REPO_SLUG` | **scrubbed only** — `<owner>/<repo>`; the ONE repo this cell is scoped to (checked against the roster's `ASSAY_ALLOWED_REPOS`) |
| `CELL_PATH` | **scrubbed only** — overrides the trailing system part of the composed `PATH` (never the shim prefix or the resolved harness dir) |
| `CELL_COCKPIT` | the surface `up` opens the role windows in: `auto` (default — herdr if on PATH, else orca if on PATH and its desktop app answers, else tmux), `tmux`, `herdr` or `orca`; `--cockpit` overrides it per run (see *Cockpits*) |
| `DESKD` | **house** — `1` to require and stand a `deskd` as a k8s cell does (default `0`: no deskd, and `check` reports it `n/a`) |
| `CELL_FORGE` | the cell's forge, `github` or `gitlab` (default `github` — a cell scaffolded before forge support carries none and is a GitHub cell by construction) |
| `CELL_REPO` | the checkout the role worktrees are created from |
| `CELLS_CONFIG` | this cell's `cells.yaml` slice (default `<cell-dir>/cells-<cell>.yaml`) |
| `FORGE_API_BASE` | the forge API endpoint, derived from the forge (github: `https://api.<GITHUB_HOST>`; gitlab: the GitLab API base) — the single home of the host, so no verb spells a literal |
| `DESKD_ADDR` | the address `deskd` serves on (default `127.0.0.1:8787` — give a second cell its own port) |
| `DESKD_INDEX` | the persistent `deskd` index path |
| `DESKD_APP_PEM` | **github** — the `deskd` read App's private key, in the operator's config home |
| `DESKD_APP_ID_VAR` | **github** — the variable name in the **cell home's** `apps.env` holding that App's id (default `DESK_APP_ID`, the generic role name a cell `apps.env` uses) |
| `ORGS` | **github** — comma-separated orgs to mint one installation token each for |
| `GITLAB_GROUP` | **gitlab** — the GitLab group this cell reads |
| `GITLAB_API_BASE` | **gitlab** — the GitLab API base deskkit's GitLab custody reads (kept equal to `FORGE_API_BASE`) |
| `GITLAB_TOKEN_STORE` | **gitlab** — the directory holding the hand-provisioned `gitlab-<role>.token` files (default: the cell config home) |
| `DESKD_GITLAB_TOKEN_FILE` | **gitlab** — the `deskd` read token file (default `<store>/gitlab-deskd.token`, mode `0600`); never minted by cellctl |
| `ROLES` | the role windows `up` opens (default: all five) |
| `DESK_MODEL_DEFAULT` | the **claude**-namespace model every role window launches on absent a per-role pin (default `sonnet`) |
| `DESK_MODEL_<role>` | **claude**-namespace per-role model override — role name with `-` as `_`, e.g. `DESK_MODEL_the_desk=fable`; an Opus pin here (or via `DESK_MODEL_DEFAULT`) is refused for `the-desk` |
| `CODEX_MODEL_default` | the **codex**-namespace equivalent of `DESK_MODEL_DEFAULT` — no compiled default; absent, a role falls to the tier map (see *Per-harness namespaces and the tier-map fallback*) |
| `CODEX_MODEL_<role>` | **codex**-namespace per-role model override, same `-`-as-`_` role naming; the Opus refusal does NOT apply here (Opus is a Claude-only concept) |
| `TIER_MODEL_TOP_CLAUDE` / `_MID_CLAUDE` / `_FAST_CLAUDE` | overrides one entry of the claude column of the tier-map fallback (compiled defaults `fable`/`sonnet`/`haiku`) |
| `TIER_MODEL_TOP_CODEX` / `_MID_CODEX` / `_FAST_CODEX` | overrides one entry of the codex column of the tier-map fallback (compiled default `gpt-5.6-terra` for all three today) |
| `CELL_HARNESS` | the harness every role window boots on: `claude` (default) or `codex`; `--harness` overrides it per run (see *Harnesses*); `--set` persists the override, `cellctl set --harness` the same without a boot |
| `CELL_KIND` / `CELL_COCKPIT` / `CELL_PROVIDER` | overridable per run with `--kind` / `--cockpit` / `--provider` on `desk`/`up`, persisted by `--set` or `cellctl set --kind/--cockpit/--provider`, read back by `cellctl show` (see *Every per-run choice*) |
| `CELL_PROVIDER` | the default provider name for `cellctl desk`/`up` (unset = Anthropic); `--provider` overrides it per run — see *Providers* |
| `CELL_PROVIDER_<NAME>_BASE_URL` | the provider's endpoint — exported as `ANTHROPIC_BASE_URL` when this provider is resolved |
| `CELL_PROVIDER_<NAME>_TOKEN_ENV` | the **name** of an env var (never the token itself) whose value is exported as `ANTHROPIC_AUTH_TOKEN`; that env var must be set in the shell running `cellctl` |
| `TMUX_SESSION` | override the tmux session name (default `<cell>-cell`) |

## Container cells

Register an existing container launcher to make the cell visible to `cellctl ls`
and start it through the same command entry point:

```sh
cellctl new sample --kind container --repo example-org/example-repo \
  --launcher /absolute/path/to/container-launcher
cellctl ls
cellctl check sample
cellctl up sample
cellctl desk sample the-desk --model sonnet
cellctl down sample
```

Registration creates only `<cells-root>/sample/cell.env`, mode 0600. It records
`CELL_KIND=container`, `CELL_REPO`, `CELL_CONTAINER_LAUNCHER`, `CELL_HARNESS`, model
pins and `ROLES`. It does not clone a host checkout, link a config home, copy keys,
contact Docker, or log into a model provider. The repo value identifies the
container's repository; it need not name a directory on the host.

The launcher must already exist at an absolute executable path. `cellctl` invokes
it directly as an argument vector, never as a shell command string:

| Command | Launcher receives |
| --- | --- |
| `check sample` | `check` |
| `desk sample the-desk` | `desk the-desk --harness claude --model <resolved-pin>` |
| `up sample` | The same coordinator launch as `desk sample the-desk` |
| `down sample` | `down` |

The default role list is `the-desk`. This first integration supports `up` only
when that is the sole configured role. For additional roles, register them with
`--roles` and use explicit `desk` commands. Multi-role container cockpits,
`--no-attach`, scheduling and host `deskd` are outside this integration; those
options refuse instead of falling through to host launches. Whether an already
running container is attached or reported as running belongs to the launcher.

Model resolution, the coordinator's Opus refusal, `--model`, `--harness`, and
`desk --set` use the existing `cellctl` rules. Unsupported host config-directory
and provider arguments refuse: model credentials belong to the container
launcher. `DRY_RUN=1` prints the intended call without invoking the launcher or
persisting a model override. A nonzero launcher exit is preserved.

The launcher receives a clean environment containing only `HOME`, `PATH`,
`TERM`, `CELL`, `CELL_KIND`, `CELL_DIR`, `CELL_REPO`, `CELL_ROOTS`, `CELL_HARNESS`
and `ROLES`. For `desk`, the argument vector's harness and model are authoritative,
including per-invocation overrides. Forge tokens, model tokens, SSH-agent settings
and the operator's `CLAUDE_CONFIG_DIR` are not forwarded from the parent shell.
The launcher is trusted **host code**, not sandboxed by this environment cleanup.
It can read host files with the operator's authority; this contract does not
establish isolation from another unrestricted host process.

The deployment launcher owns the remaining checks and actions:

- `check`: validate the local engine target, pinned image, own-cell volumes,
  credential-file presence/mode and supported harness. Report unavailable state
  as a failure, not as an empty or stopped cell. Keep this check free of model
  execution and forge credential minting.
- `desk`: ensure the selected role uses its own writable workspace, roster and
  credential mounts; verify container ownership before attaching; pass the
  supplied model to the harness and invoke the selected role's Assay skill.
  Enforce the runtime credential contract before opening the agent.
- `down`: stop only containers verified to belong to this cell. Preserve working
  volumes and refuse ambiguous ownership.

No container runtime implementation is bundled by this registration change.
The existing [container credential contract](../containers/secrets.md) still
applies to deployments using the published desk images. A container launcher
must not mount the operator's whole home, credentials directory or engine socket
into an agent merely because those paths are available on its host.

---

## Parity with the shell oracle

`cellctl` was a 3,000-line shell script before it was a Go program, and the script is still in
the tree at `tools/cellctl/cellctl`. It is not dead weight and it is not a fallback: it is the
**ORACLE** the Go program is proved against, and it stays until a human signs the cutover.

**The harness.** `tools/cellctl/tests/parity.test.sh` runs both implementations over the same
hand-built fixtures and diffs what they produce:

```bash
CELLCTL_A=tools/cellctl/cellctl CELLCTL_B=tools/desk/cellctl bash tools/cellctl/tests/parity.test.sh
```

For every cell in the matrix it runs `DRY_RUN=1 <impl> <verb> <args>` under both, normalises the
two things that legitimately differ (each copy's own path, and the fixture root each was given,
plus the clock-derived tokens the launcher itself stamps into a session name or a backup
filename), and diffs stdout, stderr and the exit code. Any difference is a divergence naming
`<kind>/<harness>/<cockpit>/<verb>`, and the harness exits 1.

**The matrix** is 200 cells: kinds {k8s, house, container, scrubbed} × harness {claude, codex} ×
cockpit {tmux, herdr, orca} × verbs {check, desk, up, down, set, ls, smoke, status}, plus `new`
per kind × forge {github, gitlab}. `new` has no dry run, so parity there is a byte-diff of the
whole tree each implementation scaffolds — paths, mode bits and file contents. `PARITY_ONLY=<substring>`
narrows a run to one cell or one verb.

**`deskd` is deliberately outside the matrix.** It has no `DRY_RUN` plan path in the oracle, so
there is nothing to diff, and giving it one would mean editing the oracle. It is also the single
most credential-sensitive verb — the script hand-built an RS256 App JWT with `openssl` and
exchanged it for per-org installation tokens. The Go port mints nothing of its own: it goes
through `deskkit.RoleTokenForRepo`, the same custody code every other desk verb's credential
travels, and the package contains no `crypto/rsa`, no `crypto/x509`, no JWT and no `openssl`
shell-out. That is asserted at the source level rather than by diff.

**Two independent layers, not one.** The plan diff above is the first. The second is the
seventeen behavioural suites beside it (`tools/cellctl/tests/*.test.sh`), which assert what was
WRITTEN, what a stub RECORDED and which exit code came back — so they fail for different reasons
than a textual diff:

```bash
for s in tools/cellctl/tests/*.test.sh; do
  case "$s" in *parity*) continue;; esac
  CELLCTL=tools/desk/cellctl bash "$s" || echo "suite red: $s"
done
```

Each suite honours `$CELLCTL`, defaulting to the oracle, so the SAME suite runs against either
implementation. A handful of cases observe a MECHANISM the two implementations do not share — a
stub recording the `$HOME` of a shell-out, a grep of the implementation's own source, the
sed-stamp packaging exception — and those state themselves `n/a` against the binary rather than
failing.

**The negative control.** A harness that diffs nothing is a green lamp wired to nothing. Building
with `-tags parity` compiles in a divergence injector that drops one named `[plan] env` line, and
the harness must then go RED:

```bash
cd tools/desk && go build -tags parity -o /tmp/cellctl-parity ./cmd/cellctl && cd ../..
CELLCTL_PARITY_MUTATE=KUBECONFIG CELLCTL_A=tools/cellctl/cellctl CELLCTL_B=/tmp/cellctl-parity \
  bash tools/cellctl/tests/parity.test.sh     # expected: exit 1, naming scrubbed/…/desk cells
```

`KUBECONFIG` is the canary on purpose: it is the cluster-isolation control, the most damaging
line to lose silently. The injector lives in exactly one file, behind `//go:build parity`, so a
release build — the one a tagless `go build` produces — contains none of that code and ignores
the variable entirely.

## Per-role provider, model and effort policy

For shared provider model defaults and per-desk effort with cell-level exceptions,
see [Shared provider defaults](cellctl-provider-defaults.md). For a complete standalone
policy, see [Cell model policy](cellctl-model-policy.md); `CELL_MODEL_POLICY` takes
precedence over shared defaults. Existing legacy cell pins apply when neither is configured.


### Claude prompt suggestions

Every `cellctl desk` Claude launch, including provider-backed GLM/Kimi sessions,
exports `CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false`. Scrubbed desk and smoke
launches compose the same value; an inherited `true` does not override it.
The dry-run output shows the value. Container base and harness images also default
it to `false`, covering direct Claude processes started in those images.

External schedulers that launch Claude themselves, such as Orca automations,
do not inherit a future `cellctl desk` environment. Set the same variable in their
job environment or the Claude configuration they load. For direct host invocations,
set `"env": {"CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION": "false"}` in the relevant
Claude `settings.json`; this covers cron without relying on interactive shell rc
files. Claude settings can override inherited environment variables, so remove any
contradictory `true` from higher-priority settings. Existing processes do not acquire
new shell exports; use `/config` or restart the affected session after rollout.
