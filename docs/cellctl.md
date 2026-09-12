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
cellctl up house                        # every role, in tmux
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

## `cellctl check` — the preconditions

```bash
cellctl check <cell>
```

One `ok` / `MISS` line per precondition, exit 1 if any row missed: the checkout named by
`CELL_REPO`, the cells slice, the operator config home, `roster.env`, that every App-key symlink
under `home/.config` resolves (a dangling symlink is the common outcome of step 2), the configured
forge endpoint, `bin/deskd` and `bin/deskcli`, the desk-tools bindir, `tmux`, and whether this
cell's `deskd` answers on its address.

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
cellctl desk <cell> <role> [CLAUDE_CONFIG_DIR]
```

`<role>` is one of `the-desk`, `intake-desk`, `worker-desk`, `pr-review-desk`, `verify-desk`. The
config dir defaults to `$CLAUDE_CONFIG_DIR`, then `~/.claude`, and is resolved against the real
`HOME`. `DRY_RUN=1` prints the plan — cell, role, config dir, worktree, session name, shim target —
and touches nothing.

Each window gets: the `assay@assay` plugin enabled in that config dir; its own worktree under
`worktrees/<role>` fast-forwarded to `origin/main` and **locked** (`git worktree lock`, so a
worktree prune never takes a live window's tree); the real `HOME` with `shim/` first on `PATH`;
`DESK_LOOP` and `DESK_SESSION` set, and `DESK_ROOTS` when `cell.env` carries `CELL_ROOTS` (a
cell without one boots with a notice that the desk verbs are on their compiled placeholder
topology); its pinned model; and `/assay:<role>` as its first prompt.

When the installed desk-tools ship a `deskwt role-init` that supports the role (probe: `deskwt
role-init --help` exits 0), `cellctl desk` lets **it** create the role worktree on first boot — its
last output line is the path — and links `worktrees/<role>` to that tree, so cellctl and the desk
skills agree on the worktree's name and the next boot fast-forwards the same tree. A `deskwt` that
is absent or refuses the probe leaves cellctl's own worktree path in charge; `CELLCTL_DESKWT=0`
forces that path.

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

A tmux session named `<cell>-cell`: a `deskd` window (watching `/healthz` if it is already up,
otherwise standing it — see *How far the attended affirmation carries* above) and one window per role
in `ROLES`. Windows start two seconds apart, so they do
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

---

## `cell.env`

`cellctl new` writes it; edit it directly afterwards.

| Variable | What it is |
|---|---|
| `CELL` | the cell name (also the tmux session prefix) |
| `CELL_KIND` | `k8s` (default — a cell scaffolded before kinds existed carries none) or `house` (the operator's own desks; see *House cells*) |
| `CELL_ROOTS` | the stream-root map, `<owner>/<repo>=<abs path>,...`, exported to every role window as `DESK_ROOTS`; **required** on a house cell, optional on k8s (unset = the verbs' compiled placeholder topology, and `desk` says so) |
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
| `DESK_MODEL_DEFAULT` | the model every role window launches on (default `sonnet`) |
| `DESK_MODEL_<role>` | per-role model override — role name with `-` as `_`, e.g. `DESK_MODEL_the_desk=fable`; an Opus pin here (or via `DESK_MODEL_DEFAULT`) is refused for `the-desk` |
| `TMUX_SESSION` | override the tmux session name (default `<cell>-cell`) |
