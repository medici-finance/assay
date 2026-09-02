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
  worktrees/<role>/    one worktree per role, fast-forwarded to origin/main at every boot
  shim/                generated — every desk verb wrapped to run with HOME=<cell>/home
```

The **cells root** defaults to `${CELLS_ROOT:-${XDG_DATA_HOME:-$HOME/.local/share}/assay/cells}`.
The **desk-tools bindir** the shims wrap defaults to `${DESK_TOOLS_BIN:-/opt/desk-tools/bin}` — the
path the pinned desk-tools tarball installs to (`docs/adopting-assay.md`, PRIMITIVE:
`install-desk-tools`). The **operator's real config home**, which the App-key symlinks point into, is
`${ASSAY_CONFIG_HOME:-$HOME/.config/assay}`. Override any of the three in the environment.

Running two cells on one laptop is two directories under the cells root, each with its own roster,
its own Apps, its own `deskd` port, and its own tmux session. They share nothing but the binaries.

---

## Why the session keeps the real `HOME`

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

Two lines. Copy the script onto your `PATH` and make it executable:

```bash
install -m 0755 tools/cellctl/cellctl ~/.local/bin/cellctl     # from a checkout of this repo
# or, from an extracted release tarball:
install -m 0755 ./cellctl ~/.local/bin/cellctl
```

`cellctl` needs `git`, `tmux`, `curl`, `openssl` and `python3` on `PATH` — `cellctl check` reports
each one. It does not need a Go toolchain.

> **Not yet in the tarball.** Shipping `cellctl` inside `desk-tools-<platform>.tar.gz` is a change to
> the release workflow and is deliberately not part of the change that added this document. Until
> that lands, the checkout line above is the install.

---

## `cellctl new` — scaffold, then four hand steps

```bash
cellctl new <cell> \
  --repo /path/to/checkout \
  --cells-yaml /path/to/cells-<cell>.yaml \
  --orgs org-a,org-b \
  --deskd-app-pem "$HOME/.config/assay/<cell>-desk-app.pem" \
  [--deskd-app-id-var DESK_APP_ID] \
  [--port 8787]
```

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

## `cellctl check` — the preconditions

```bash
cellctl check <cell>
```

One `ok` / `MISS` line per precondition, exit 1 if any row missed: the checkout named by
`CELL_REPO`, the cells slice, the operator config home, `roster.env`, `apps.env`, that every App-key
symlink under `home/.config` resolves (a dangling symlink is the common outcome of step 2), the
linked `gh` config, `bin/deskd`, the `deskd` App key being readable, the desk-tools bindir, `tmux`,
and whether this cell's `deskd` answers on its address.

Run it after `new`, and again after any key rotation.

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

---

## `cellctl desk`, `up`, `down`

**One window:**

```bash
cellctl desk <cell> <role> [CLAUDE_CONFIG_DIR]
```

`<role>` is one of `the-desk`, `intake-desk`, `worker-desk`, `pr-review-desk`, `verify-desk`. The
config dir defaults to `$CLAUDE_CONFIG_DIR`, then `~/.claude`, and is resolved against the real
`HOME`. `DRY_RUN=1` prints the plan — cell, role, config dir, worktree, session name, shim target —
and touches nothing.

Each window gets: the `assay@assay` plugin enabled in that config dir; its own worktree under
`worktrees/<role>` fast-forwarded to `origin/main`; the real `HOME` with `shim/` first on `PATH`;
`DESK_LOOP` and `DESK_SESSION` set; its pinned model; and `/assay:<role>` as its first prompt.

Each window is named **`<cell>-<short role>`** — the role without its `-desk` suffix, except
`the-desk`, which keeps its full name (`<cell>-the-desk`, `<cell>-pr-review`, `<cell>-verify`,
`<cell>-intake`, `<cell>-worker`). That one string is used for **both** surfaces: `DESK_SESSION`, the
roster beacon, and `claude --name`, the session's display name — so the cell's coordinator sees the
same identity in the roster and in its agent listing rather than two names for one window.

Two details in the boot are there for a reason. The shared `fetch` is **serialised with a lock
directory**, because several windows starting at once fetch the same `.git` and race on the ref lock.
And a `fetch` that exits non-zero from that race has still written `FETCH_HEAD` — which is the only
thing the boot reads — so it is reported as a notice and the boot continues.

**The whole cockpit:**

```bash
cellctl up <cell> [--no-the-desk] [--no-attach] [CLAUDE_CONFIG_DIR]
```

A tmux session named `<cell>-cell`: a `deskd` window (standing it if it is not already up, otherwise
watching `/healthz`) and one window per role in `ROLES`. Windows start two seconds apart, so they do
not all arrive at the fetch lock together. `up` attaches unless you pass `--no-attach`; re-attach
later with `tmux attach -t <cell>-cell`.

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
cellctl down <cell> [--keep-deskd]
```

Kills the session and this cell's `deskd` — matched on its own `--config` path, so another cell's
`deskd` on the same laptop is untouched. `--keep-deskd` leaves it standing.

**List:** `cellctl ls` prints the cells under the cells root. No cells is not an error.

---

## Pinned models

**Every role window launches on a model named in `cell.env`. The CLI default is never used.** A
default that moves under a running cell changes what five long-lived loops do without anything in the
cell saying so, which is the kind of drift a cell exists to keep out.

```
DESK_MODEL_DEFAULT=sonnet     # every role that has no override
DESK_MODEL_the_desk=opus      # per-role override: the role name with `-` replaced by `_`
```

An override is `DESK_MODEL_<role>` with hyphens replaced by underscores — `DESK_MODEL_the_desk`,
`DESK_MODEL_pr_review_desk`, `DESK_MODEL_verify_desk`, and so on. Values are whatever
`claude --model` accepts: an alias (`opus`, `sonnet`, `haiku`) or a full model id, including a
long-context variant. `DESK_MODEL_DEFAULT` itself falls back to `sonnet` if `cell.env` omits it, and
`cellctl new` scaffolds both lines above so a fresh cell is pinned from the start.

**Why the defaults are shaped that way.** The four loop roles are mechanical dispatchers: they read a
board, claim an item, open a worktree, and hand the actual judgment to the agent they dispatch. The
coordinator window is where judgment happens in the loop itself. So the loops get the cheaper model
and `the-desk` gets the stronger one — and either can be moved per cell, which is the point of
putting it in `cell.env` rather than in the script.

`cellctl desk` prints the resolved model on its launch line and in `DRY_RUN=1` output, so which model
a window is on is visible without reading the config.

---

## `cell.env`

`cellctl new` writes it; edit it directly afterwards.

| Variable | What it is |
|---|---|
| `CELL` | the cell name (also the tmux session prefix) |
| `CELL_REPO` | the checkout the role worktrees are created from |
| `CELLS_CONFIG` | this cell's `cells.yaml` slice (default `<cell-dir>/cells-<cell>.yaml`) |
| `DESKD_ADDR` | the address `deskd` serves on (default `127.0.0.1:8787` — give a second cell its own port) |
| `DESKD_INDEX` | the persistent `deskd` index path |
| `DESKD_APP_PEM` | the `deskd` read App's private key, in the operator's config home |
| `DESKD_APP_ID_VAR` | the variable name in the **cell home's** `apps.env` holding that App's id (default `DESK_APP_ID`, the generic role name a cell `apps.env` uses) |
| `ORGS` | comma-separated orgs to mint one installation token each for |
| `ROLES` | the role windows `up` opens (default: all five) |
| `DESK_MODEL_DEFAULT` | the model every role window launches on (default `sonnet`) |
| `DESK_MODEL_<role>` | per-role model override — role name with `-` as `_`, e.g. `DESK_MODEL_the_desk=opus` |
| `TMUX_SESSION` | override the tmux session name (default `<cell>-cell`) |
