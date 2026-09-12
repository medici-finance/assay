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

`cellctl` needs `git`, `tmux` (the default cockpit), `curl`, `openssl` and `python3` on `PATH` — `cellctl check` reports
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
cellctl desk <cell> <role> [--model <m>] [--set] [--harness <claude|codex>] [CLAUDE_CONFIG_DIR]
```

`<role>` is one of `the-desk`, `intake-desk`, `worker-desk`, `pr-review-desk`, `verify-desk`. The
config dir defaults to `$CLAUDE_CONFIG_DIR`, then `~/.claude`, and is resolved against the real
`HOME`. `DRY_RUN=1` prints the plan — cell, role, config dir, worktree, session name, shim target,
model and harness — and touches nothing. `--model` and `--set` are the per-run override and the
sugar that persists it — see *Pinned models* below for the full shape; `--harness` is the same
per-run-override shape for the harness — see *Harnesses* below.

Each window gets: its own worktree under `worktrees/<role>` fast-forwarded to `origin/main` and
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
skills agree on the worktree's name and the next boot fast-forwards the same tree. A `deskwt` that
is absent or refuses the probe leaves cellctl's own worktree path in charge; `CELLCTL_DESKWT=0`
forces that path.

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
                  [--automate '<cron>'] [--model <m>] [--harness <claude|codex>] [CLAUDE_CONFIG_DIR]
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
actually boots resolves the same override the plan named.

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
| **herdr** | one **labelled tab per window**, `<cell>-<role>`, each started as a `claude`-kind agent under that label — so the cockpit's agent state drives its sidebar per desk. The first (`deskd`/`cell`) window is a tab too |
| **orca** | one **terminal per role** under the cell directory, running the same `cellctl desk` command; or, with `--automate`, one scheduled automation per role instead |

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

- a herdr build with no `tab create` still gets labelled windows from `agent start --label`, with a
  notice; a build whose `agent` has no `start` is a refusal, because nothing could host a window;
- orca's create-a-terminal verb and its command / working-directory / name flags are read from its
  own help. Where the verb or the command flag is absent, `cellctl` **prints the exact per-role
  commands to run by hand** and refuses, rather than guessing a spelling;
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
```

Rewrites an existing `KEY=` line **in place** (comment lines and every other line's ordering
untouched) or **appends** a key that has no active line yet. Writes exactly **one backup**,
`cell.env.bak-<ts>`, before the first edit of a call — a multi-key `set` is one backup for the
whole call, not one per key. Refuses a key that is not a known `cell.env` key (see the table below;
the `DESK_MODEL_<role>` family counts as known for each of the five roles) unless `--force`, and a
refusal — unknown key **or** the Opus rule below — touches nothing: no backup, no edit. Prints each
key's before/after value. Applies the **same** the-desk/Opus refusal to `DESK_MODEL_the_desk` as a
live boot and `check` do, and `--force` does not bypass it — `--force` widens which *keys* `set`
will touch, not which *values* the Opus rule allows.

**Sugar: override now and persist it in one call.**

```bash
cellctl desk <cell> <role> --model <m> --set
```

`--set` applies `--model <m>` for this run **and** persists it — equivalent to `--model <m>`
followed by `cellctl set <cell> DESK_MODEL_<role>=<m>`. It needs a value to persist, so it is
refused without `--model` (or `DESK_MODEL_OVERRIDE`) alongside it. Under `DRY_RUN=1` it prints what
it *would* persist and writes nothing — a dry run touches nothing, `--set` included.

**`CELL_HARNESS` is a known key too** — `cellctl set <cell> CELL_HARNESS=codex` persists the harness
pin the same way, with the same value check as the harness flag itself: only `claude` or `codex` is
accepted (not bypassable by `--force`, which only widens which *keys* `set` will touch). There is no
`--harness ... --set` sugar — `cellctl desk`/`up --harness` is a per-run override only; persist it
with `cellctl set` directly.

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
this arm and is not passed; the model comes from the same `DESK_MODEL_DEFAULT`/`DESK_MODEL_<role>`
resolution `cellctl desk` always uses.

**`--sandbox danger-full-access` is required, honestly.** Per the ruled capability matrix (the
`#937` live smoke run is the evidence codex CLI's default `workspace-write` sandbox blocks the
`.git/refs/heads/` write a fresh worktree needs — codex has no built-in worktree management, so a
skill that must isolate has to run `git worktree add` itself, and that is exactly what
`workspace-write` blocks), the worktree this window runs in could not have been **created** under
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

## `cell.env`

`cellctl new` writes it; edit it directly afterwards.

| Variable | What it is |
|---|---|
| `CELL` | the cell name (also the tmux session prefix) |
| `CELL_KIND` | `k8s` (default — a cell scaffolded before kinds existed carries none) or `house` (the operator's own desks; see *House cells*) |
| `CELL_ROOTS` | the stream-root map, `<owner>/<repo>=<abs path>,...`, exported to every role window as `DESK_ROOTS`; **required** on a house cell, optional on k8s (unset = the verbs' compiled placeholder topology, and `desk` says so) |
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
| `DESK_MODEL_DEFAULT` | the model every role window launches on (default `sonnet`) |
| `DESK_MODEL_<role>` | per-role model override — role name with `-` as `_`, e.g. `DESK_MODEL_the_desk=fable`; an Opus pin here (or via `DESK_MODEL_DEFAULT`) is refused for `the-desk` |
| `CELL_HARNESS` | the harness every role window boots on: `claude` (default) or `codex`; `--harness` overrides it per run (see *Harnesses*) |
| `TMUX_SESSION` | override the tmux session name (default `<cell>-cell`) |
