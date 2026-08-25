# tools/desk — desk-tools binaries

Purpose-built, policy-in-code Go tools that make the methodology's standing loops
(pr-review-desk, verify-desk, worker-desk, next-job) zero-prompt for **workflow
verbs** while keeping per-brief payload gating intact. The scoping and threat
model — constraints **C-1…C-10**, binding here — are maintained in the desk-tools
operator notes, kept outside this published tree.

Layout mirrors the root [`statusgen/`](../../statusgen/README.md) tree: this directory is its own Go module
(`github.com/medici-finance/assay/tools/desk`) wired into the repo `go.work`.

```
tools/desk/
  go.mod
  internal/deskkit/   ← shared safety plumbing (brief 01)
  cmd/<tool>/         ← one binary per workflow verb (deskpr = brief 04; 02/03/05/07 pending)
  dist/               ← desk-build output (unprivileged, git-ignored)
  MANIFEST.sha256     ← written by `sudo make desk-install`; LOCAL, not committed
```

**New here?** The table below is the whole surface, one line per binary; then
[Your first hour with desk-tools](#your-first-hour-with-desk-tools) is the how-to.
Everything after that is the deep reference, one section per tool, and you do not need
it on day one.

## Tool reference

| Tool | Verb(s) | Class | Write budget + breaker |
|------|---------|-------|------------------------|
| `deskboard` | `prs`, `actions`, `reviews`, `queue`, `health`, `awaiting` (alias `nextup`), `dispatch` (aliases `todo`, `next`, `next-up`), `scope`, `policydrift`, `stalled`, `diff`, `files` | read-only | no |
| `issueboard` | `board`, `issues`, `intake` | read-only | no |
| `verifyloop` | `plan` | read-only (spawns nothing, writes nothing) | no |
| `reviewloop` | `plan` — pr-review-desk's BOARD REACTOR: classifies a `deskboard` sweep against an action table derived from deskboard's own ACTION constants, coalesces outward verbs on `(repo, pr, head, verb)`, and answers the #79 idle question in THREE states. Not a drain: it does not link `internal/loopengine` | read-only (spawns nothing, writes nothing, makes no GitHub call) | no |
| `deskpost` | `review`, `comment`, `ready` — as the reviewer App | outward write | yes |
| `deskpr` | `create` (draft-only), `update` (follow-up push) | outward write | yes |
| `deskreply` | PR reply comment under the **worker** identity | outward write | yes |
| `deskfile` | `new`, `attach`, `check` — the issue-filing gate (dedupe first) | outward write | yes |
| `deskclose` | `duplicate`, `superseded`, `review-request`, `manifest` — the issue-CLOSING gate (a fetched human authorization or nothing) | outward write | yes |
| `deskdigest` | (no verbs) `--dry-run` / `--post` — the weekly batched decision queue; reports only, and writes exactly one issue: its own | outward write | yes |
| `deskdisposition` | `set`, `read`, `sweep` — machine-readable PR disposition records (#728/#827); records a verdict, never closes | outward write (`set`) / read-only (`read`, `sweep`) | yes |
| `deskmerge` | `check` — merge-currency in three states, writes nothing; `merge` — merges main INTO a PR branch, gated on a fetched human sign-off of R-5 (unsigned today, so it merges nothing) | read-only (`check`) / outward write (`merge`) | yes (`merge`) |
| `deskscanbody` | `emit`, `check` — derives the issue-loop scan PR title/body from the branch diff (#685) | local-only (git read) | no |
| `deskevidence` | `commit` — Evidence via the Contents API, as the verifier App | outward write | yes |
| `deskrelease` | `cut <tag>` — create-only tag ref, as the desk App | outward write | yes |
| `deskclaim` | `acquire`, `release`, `list` — the flock-backed claimable-action lock | local-only (claims dir) | no |
| `opmetrics` | (no verbs) — operator-layer collector: reads transcripts/beacons/claims, writes one **aggregates-only** day-file | local-only (strictly read-only against every input) | no |
| `deskwt` | `add`, `remove`, `prune` under sanctioned prefixes | local-only | no |
| `deskgit` | `fetch` (bare / `--prune` / `--pr <N>` / `--branch <B>`) — the only git verb | local-only (inbound refs) | no |
| `desktoken` | `<role>` — mint/reuse an App installation token | local-only (token cache) | no |
| `deskroster` | `set`, `drop`, `list`, `mine`, `preflight` | local-only, out-of-git (`preflight` mints a token and runs one read-only transport probe) | no |
| `muhar` | `-spec <file>` mutation harness | local diagnostic (no `Guard`) | no |
| `writeguard` | PreToolUse hook (F-34 isolation backstop) | hook | n/a |
| `deskpushguard` | pre-push hook — refuses a push to a MERGED/CLOSED branch, one carrying a foreign/laundered commit, a single-parent merge masquerade, or a branch point sitting on a stray local `origin/main`, or one introducing a register-entry `id:` collision with an in-flight sibling branch (#22, #72). Cannot determine the base → prints `COULD-NOT-CHECK` and allows (fail-open, brief-10); that line means UNVERIFIED, not clean | git hook | n/a |
| `desksourceguard` | CI gate — refuses a materialised desk-tools source tree that is not the pinned commit | CI | n/a |

Read it by **class**: read-only tools only query GitHub and print; outward-write tools
can change something on GitHub and are the only ones the rate meters gate; local-only
tools touch state under your own config home; hooks and CI gates are invoked by git and
Actions, not by you. The safety contract behind the last column — `Guard`, the audit
line, the two meters, the configured repo scope — is in
[Guard, the two meters, and repo scope](#guard-the-two-meters-and-repo-scope).

## Your first hour with desk-tools

Read this once, in order. It is the operator on-ramp; every rule it states is specified
in full further down and linked from here.

**What these tools are.** They are not a wrapper over `gh`. Each one is a single workflow
verb with the policy compiled into it — who may run it, on which repos, how often, and
what it refuses — so an agent session running the verb cannot talk its way past the
policy, and every attempt leaves an audit line.

### 1. Get a build, and put it on `PATH`

```bash
make desk-build          # unprivileged; builds every cmd/* into tools/desk/dist/
sudo make desk-install   # HUMAN ONLY (C-1) — installs to /opt/desk-tools/bin, root-owned
export PATH=/opt/desk-tools/bin:$PATH
```

Skills and allow-rules invoke these tools by **absolute path**
(`/opt/desk-tools/bin/<tool>`), which is what anchors a `Bash(...)` permission on a
specific installed binary. That is deliberate — but the install dir must *still* be on
`PATH`, because some tools shell out to each other **by bare name**: `deskpr` runs
`desktoken worker --repo <repo>` (`cmd/deskpr/exec.go`) to authenticate as the worker
App. Off `PATH`, that shell-out fails and the write verb dies at the mint step with a
message about `desktoken`, not about `PATH` — the first-hour trap recorded in #794/#567.
Then check the binary is not stale:

```bash
/opt/desk-tools/bin/deskboard --version   # deskboard sourceSHA=<sha> builtAt=<ts>
```

Drift against `origin/main` is reported by the `stale` / `staleState` header fields, and
`unknown` reads as **stale** — see [Version check](#version-check-stale-binary-detection).

### 2. Start read-only

The read-only rows in the table cannot change anything. Live in them first:

```bash
deskboard prs --table      # every open PR across the configured repo set
deskboard queue            # what is waiting on whom
deskboard health           # default-branch health, per repo
issueboard board           # the issue side of the same view
deskclaim list             # which claimable actions are held right now
```

Each run echoes the effective configuration (`assay-config:` lines) before its output —
the allowed-repo set, the trusted identities, the risk triggers. That echo is the point:
a widened scope shows up in run output and CI history rather than in someone's memory.

### 3. Learn the exit codes before you run a write verb

This is the single most common new-operator mistake on this toolchain, so learn it here:

| Exit | Meaning | What you do |
|------|---------|-------------|
| `0` | ok (or an idempotent noop) | nothing — it worked, or it had already been done |
| `3` | **disabled** — kill switch or a stop flag is armed | stop. Do not work around it; a human clears the flag |
| `4` | **rate-limited** — a write meter or the breaker is open | read `retry-after:`, sleep it **once**, attempt **once**. Never poll |
| `5` | **refused** — a deliberate safety stop | stop and fix the input. **Never a fallback trigger** |
| `6` | **unverifiable** — the tool could not positively confirm state | the documented raw-command fallback is authorised |

The distinction that matters is **5 versus 6**. Exit 5 is the tool telling you the thing
you asked for is not allowed — a body that failed the secret scan, a push to the default
branch, an extra positional argument. Reaching for `gh` or raw `git` to do the same write
anyway is defeating a safety gate, not routing around a bug. Exit 6 means the tool could
not *tell*, which is why the fallback exists and why it is documented per verb in each
skill. Exit 3 is likewise a stop, not a puzzle.

Two refusals by design you can reproduce right now, without writing anything:

```bash
deskreply <owner/repo> <pr> comment --body-file f.md
# → exit 5: refused: unexpected extra arguments after <owner/repo> <pr>

deskroster preflight --help
# → exit 0 if this build has the verb, exit 5 if it does not
```

The first is worth internalising: `deskreply` takes **exactly two positionals** —
`deskreply <owner/repo> <pr> --body-file F`. There is no `comment` subcommand, and the
extra token is a refusal, not a parse warning. The second is the deliberate capability
probe: `0` means the verb exists in this build, `5` means it does not, answered in one
call with no output parsing — which is also how you notice you are on a stale binary.

### 4. The write verbs, and their fallbacks

| You want to | Verb | Fallback, on exit 3 or 6 only |
|---|---|---|
| Open your draft PR | `deskpr create --title T (--body-file F \| --body-min B)` | `git push -u origin <branch>` + `gh pr create --draft`, then **verify `isDraft: true`** |
| Push a follow-up | `deskpr update` | `git push` |
| Reply on **your own** PR as the worker | `deskreply <owner/repo> <pr> --body-file F` | `gh pr comment` |
| Post a review verdict / flip ready | `deskpost review\|comment\|ready` | (reviewer desk only) |
| File an issue | `deskfile new -R <owner/repo> --title T --body-file F` | none — dedupe is the point |

`deskpr create` is **draft-only by construction**: `--draft` is hardcoded into the
`gh pr create` argv it builds, so there is no `--draft` flag for *you* to pass — passing
one is an unexpected argument and exits 5. The git argv is likewise built literally so no
force-push flag can be emitted, and there is no ready/edit/close/merge verb anywhere in
this tree. `deskpr update` takes only `[--as-app]`: it pushes follow-up commits and
cannot edit a PR body — use `gh pr edit` for that. Flipping a PR ready and merging it are
somebody else's decision, and the tools cannot make them for you.

That draft guarantee is a property of `deskpr create`, **not** of the fallback beside it.
When exit 3 or 6 sends you to `gh pr create --draft` by hand, nothing is enforcing the
flag for you, so confirm the result rather than assume it:

```bash
gh pr view <pr> -R <owner/repo> --json isDraft   # must print true
```

Two constraints worth knowing before you reach for them. `deskreply` replies **only on
your own PR** — it compares the branch checked out in this worktree against the PR's head
branch and refuses with exit 5 otherwise, so it is not a way to comment on someone else's
work. And `deskpost comment` posts under the reviewer App identity
(`assay-reviewer-app[bot]`), not yours; it offers no worker identity, which is precisely
why `deskreply` exists.

Every write verb runs `deskkit.BodyCheck` over what you are about to post — titles,
branch name, body, and for `deskpr create` the diff against the default branch. A hit is
exit 5. It also refuses a body that claims a named human's ruling, because no desk write
path ever posts as a human.

Rehearse where you can. The verbs that carry `--dry-run` (`deskpost`, `deskrelease`) run
every check and stop before the write, auditing `result=dryrun` — invisible to both
meters, and it never suppresses the real write afterwards.

### 5. Know how to stop everything

```bash
touch ~/.config/assay/DISABLED     # arm — every tool exits 3, having audited it
rm    ~/.config/assay/DISABLED     # disarm
export DESK_TOOLS_DISABLED=1       # env-var arm, same effect, one session
```

`STOP` halts every loop at its next iteration boundary; `STOP.<loop-name>` halts one
loop. Precedence is `DISABLED` > `STOP` > `STOP.<name>`, and stop flags are **not**
self-clearing. The full semantics, including the optional `HEARTBEAT` dead-man lease,
are in [Runtime state](#runtime-state-not-created-by-this-repo).

Everything a tool attempts appends one JSON line to `~/.config/assay/audit.jsonl` —
append-only, never truncated by a tool. If it is unreadable, tools refuse with exit 6
rather than guess; recovery is a human moving the file aside.

### 6. Where to go from here

- The safety plumbing every binary shares: [deskkit](#deskkit--the-shared-foundation-brief-01).
- What the meters actually count, and the configured repo scope:
  [Guard, the two meters, and repo scope](#guard-the-two-meters-and-repo-scope).
- Whether your credentials and transport are healthy *before* you claim work:
  [`deskroster preflight`](#deskroster-preflight--the-operating-envelope-check).
- Every configuration variable, its level and what UNSET means:
  [`docs/roster-configuration.md`](../../docs/roster-configuration.md).

## deskkit — the shared foundation (brief 01)

`internal/deskkit` is imported by every desk binary. It carries the safety plumbing so
no tool reinvents it with holes:

| Helper | Contract |
|--------|----------|
| `Guard() error` | **Mandatory first call** of every tool. Checks the kill switch (C-6) before anything else; armed → audits `result=disabled` and returns exit-3 error. |
| `IsAllowedRepo(repo) bool` | Fixed C-4 repo set, compiled in; no flag/env widens it. |
| `Log(Entry) error` | Appends one JSON line to the audit log (O_APPEND; never truncates). |
| `AllowWrite(tool) error` / `AllowWriteAt(tool, now)` | **Two meters** (C-5/TM-4, #209). Budget: ≤20/tool/rolling hour (`RateLimitPerPRPerHour`, raised 10→20 by PR #1053, 2026-08-14), charged only by attempts that may have reached the remote (`ok`, `unverifiable`). Breaker: 5 consecutive non-progress attempts (`refused`, `noop`) → open for 15m. Neither meter counts `ratelimited`/`disabled` — its own output — which is what made the old single counter non-recovering; nor `dryrun` (#214), which wrote nothing. Exit-4 errors carry a **retry-after** (`RetryAfterOf(err)`). The cap is a **chosen throughput ceiling**, not a figure derived from an incident — `ratelimit.go` states what each value is chosen against. |
| `AlreadyDone(repo, pr, head, verb) bool` / `AlreadyDoneIn(entries, …)` | Idempotency: only prior `ok`/`noop` entries count as done (C-5). `dryrun` never counts — a rehearsal must not suppress the real write. |
| audit `result` | `ok` · `noop` (attempted, idempotency short-circuited it) · `dryrun` (`--dry-run`: stopped **before** the write, invisible to both meters, #214) · `refused` · `disabled` · `ratelimited` · `unverifiable`. |
| `BodyCheck([]byte) error` | Shared secret scan (C-3); refuses token/PEM/JWT/AKIA/sops shapes + high-entropy runs. Exempts 40/64-char lowercase-hex git SHAs and **paths** — absolute or repo-relative — whose slash-separated segments are *word-shaped* (#1052, #209). Opaque material between slashes still refuses, including an AWS **secret access key**, whose `/` characters defeat a length-only segment gate. **Also refuses impersonated human rulings (#45)**: a body claiming a configured human's decision BY NAME — `"Decision (Alex, ...)"`, `"Ruling: ... — Alex"`, `"I (Alex) have decided"` — is refused categorically, since no desk write path ever posts as a human; see `ImpersonatedRulingClaim` (`impersonation.go`). |
| `Version()` / `WarnIfUnpinned(w)` | Reports the embedded `sourceSHA`/`builtAt`; warns loudly when unpinned (C-1). |
| exit codes | `ExitOK 0`, `ExitDisabled 3`, `ExitRateLimited 4`, `ExitRefused 5`, `ExitUnverifiable 6`. `ExitCodeOf(err)` maps a typed error, failing **closed** to 6 for any unexpected error. |

**Fail-closed (C-10):** any state a helper cannot positively verify (unreadable/malformed
audit file, missing HOME, unparseable timestamp) is a typed `Unverifiable` refusal
(exit 6), never a silent default toward success.

### Runtime state (NOT created by this repo)

At runtime the tools read/write, under `~/.config/assay/`:

- `audit.jsonl` — append-only audit log (dir 0700, file 0600 on first use). On
  corruption a tool refuses (exit 6) and prints the recovery: a **human** moves the
  file to `audit.jsonl.corrupt-<ts>` (tools never truncate/rewrite it).
- `DISABLED` — kill switch. `touch ~/.config/assay/DISABLED` (or export
  `DESK_TOOLS_DISABLED=1`) halts the whole suite: every tool exits 3 after auditing
  `result=disabled`. Its first line is shown as the reason.
- `STOP` — all-loops stop flag (brief 08). `touch ~/.config/assay/STOP`
  halts every loop on its next iteration boundary; removed with `rm`. The Guard
  checks it BEFORE every outward verb.
- `STOP.<loop-name>` — per-loop stop flag (brief 08). Halts only the loop whose
  `DESK_LOOP` env var matches `<loop-name>`; other loops and plain tool invocations
  (no DESK_LOOP set) are unaffected. Removed with `rm`. Precedence chain:
  `DISABLED` (C-6) > `STOP` > `STOP.<name>`. A DISABLED that is cleared re-enables
  everything; stop flags are NOT self-clearing (C-10: a switch that un-flips itself
  is not a switch).

  `<loop-name>` is matched against a compiled-in roster
  (`internal/deskkit/loopnames.go`), NOT by bare string comparison, because both
  ways of missing used to fail silently — the worst mode for a safety stop:

  - **A rename must not orphan a held flag.** A loop's identity is an equivalence
    class of names, so a flag held on ANY name in the class halts it. A human
    holding `STOP.batch-fanout` keeps their stop after that loop renames itself
    `worker-desk`, and vice versa. Retired names are never removed from the roster.
  - **An unrecognised `DESK_LOOP` is `could-not-check`, not `checked-clean`.** The
    Guard cannot name the flag file that would speak for a loop it does not know,
    so it refuses **exit 6 (unverifiable)** rather than reporting "not stopped".
    `DESK_LOOP=btach-fanout` is a loud refusal naming the value and the known set,
    not a silent green. Exit 6 — not exit 5 — because this is the tool being unable
    to look, which is what `Guard` already returns 6 for; exit 5 stays reserved for
    a deliberate human refusal.

  The blast radius is bounded to the mis-named session alone: `DESK_LOOP` unset is
  unaffected, and every correctly-named loop is unaffected. The roster is compiled
  in rather than read from a runtime file precisely so no single corrupt file can
  refuse every loop on the machine at once (cf. the `roster.env` outage, #819).
  Adding a loop is therefore a PR — that is the intended price.

  `deskboard` marks a `STOP.<name>` whose name matches no known loop as **inert**:
  it halts nothing, and a human must not read it as a stop that is being honoured.
- `HEARTBEAT` — dead-man lease (brief 08, optional). If the file EXISTS and its
  mtime is older than 24h, it is treated as STOP-ALL. Absent file = feature off
  (no behaviour change until Ada opts in by touching it once). **Renewal is a HUMAN
  act** — Ada or an Ada-owned cron touches it; no agent/loop may touch or delete
  HEARTBEAT (that would defeat the dead-man semantics).
- Loop-name registry (canonical names for `DESK_LOOP` and `STOP.<name>`):
  `pr-review-desk`, `verify-desk`, `worker-desk`, `the-desk`. Ad-hoc `/loop`
  or Monitor sessions pick a name at arm time and state it in their description.
  Each loop session sets `DESK_LOOP=<canonical-name>` at boot so Guard honours its
  per-loop flag.

Restart = `rm <flag>` + re-arm the loop. Flags are checked at iteration boundaries
only (top of every monitor poll cycle, scheduled wakeup, before each loop-skill
cycle); a started outward write completes (C-5 audit intact). On hit: Guard emits
one line naming the exact flag file, exits cleanly with exit 3.

Deskboard (brief 02) banners active STOP/stale-HEARTBEAT flags on every board read
so a silently-stopped loop is visible, not discovered by absence.

### Migration (from the old `~/.claude/desk-tools/` state dir)

The state root moved from `~/.claude/desk-tools/` to `~/.config/assay/` to co-locate
runtime state with the config and keys the tools already read from there (`roster.env`,
`apps.env`, the `<role>-app.pem` keys, cached `<role>-token-*`). Existing operators with
live state at the old path should move it once:

```sh
mv ~/.claude/desk-tools/* ~/.config/assay/ 2>/dev/null; rmdir ~/.claude/desk-tools 2>/dev/null || true
```

The audit ledger, roster beacons, and any active `STOP`/`DISABLED` flags carry over. New
installs start clean — no migration needed.

## Trust gate (deskkit/trust.go)

With example-org/example-k8s public, desk scanning loops read repos where arbitrary
external users can author issues, PRs, and comments. **The rule (Ada, 2026-07-23): a
desk ignores EVERYTHING unless it is authored by a trusted desk identity or ada,
or ada has commented on it.** Unvetted third-party text must never enter a desk
work queue or steer a desk action.

- **Trusted identities (compiled in, no env/flag widens them):** `ada`,
  `example-org`, and the six desk Apps in both GitHub renderings —
  `assay-{desk,reviewer,verifier,worker,issue-loop,intake-loop}-app[bot]` (REST) /
  `app/assay-…` (gh CLI). Bare App slugs are rejected (a plain user could register
  one). Where the surface has the **numeric user id** (REST `user.id`, GraphQL
  `databaseId`), login AND id must both match the pinned identity — a recycled login
  cannot impersonate a trusted account (`TrustedAuthorID`).
- **Blessing:** one ada comment (or PR review / review comment) admits an
  externally-authored item. **A blessing covers the content as of the blessing** —
  a body edit, or any untrusted comment added or edited AFTER ada's latest
  comment, voids it (bless-then-edit; timestamps come from GraphQL `lastEditedAt`,
  which tracks content edits only — labels don't re-quarantine). A follow-up
  hash-pinning upgrade (compare a stored hash of the blessed body at act time) is
  tracked as an issue; the timestamp comparison is v1.
- **Quarantine visibility:** boards (`deskboard prs/actions/queue`, `issueboard`)
  list untrusted items under **EXTERNAL / UNBLESSED** — counted, visible (so Ada
  sees what awaits blessing), never given an ACTION. All public-origin text in that
  section renders inert-quoted (control chars/ANSI escaped) — titles are data, not
  instructions. `deskpost review|comment|ready` **refuses** (exit 5, audited) on a
  PR failing the gate.
- **Sensitive review notes on PUBLIC repos** never post publicly: they go to a private
  review channel configured by the operator, as an issue labeled `needs-human`; the
  public PR gets only a neutral "review notes recorded internally" comment. The desk
  tools have **no compiled-in name for that channel and no write authority to it** — see
  the `allowedRepos` note below; filing there is a human act.
- **There is no repo-scoped token.** This bullet previously told desks on public repos to
  mint "repo-scoped tokens (`desktoken <role> --repo <owner/name>`) … so a gate bypass has
  minimum blast radius". `desktoken` has never done that: `--repo` picks only the
  installation **owner** (`cmd/desktoken/desktoken.go:405-412` — the repo name is
  discarded), and `exchangeJWT` (`:246-248`) POSTs to `/app/installations/{id}/access_tokens`
  with a **nil body**, so GitHub returns a token at **full installation scope**. The token
  cache path (`<role>-token-<installID>`) is keyed on the install ID alone, so `--repo a/x`
  and `--repo a/y` share one token. Both App installations are currently
  `repository_selection: all` (measured 2026-08-02), so the real blast radius of any minted
  desk token is **every repo in that account** — including private repos the desk tools
  themselves refuse to act on. Known defect, documented not fixed:
  `docs/github-apps-setup.md` § "Known defects".
- **PUBLIC repos are risk-classed unconditionally**. Every PR on a public
  repo needs a `Security-Review: pass` at head before the desk may flip it, whatever
  the diff touches. This is the one public-repo protection that IS machine-enforced;
  the sensitive-notes rule above it is still prose, and the repo-scoped token above it
  does not exist at all. It inverts an asymmetry that ran the wrong way:
  the risk-path triggers were five tracker-shaped paths applied to every repo, and
  `example-org/example-k8s` — public, holding the Ledger/Identity manifests — has no
  `k8s/` directory at all, so it could never be risk-classed and the PUBLIC
  infrastructure repo got strictly LESS mandatory scrutiny than the private
  application one. **This is a policy call standing in for a human decision** (the
  issue's suggested-fix item 3, taken in the secure direction pending Ada); relaxing
  it is a one-line change to `VisibilityRiskClassed`.

## Risk classification — how a PR becomes risk-classed

One computation, `deskkit.RiskPathTriggered(repo, changedFiles)`, consumed by
`deskpost ready`'s gate (e) and `deskboard actions`' SECURITY-REVIEW-REQUIRED. It
answers TRUE on every uncertain input; the ONLY way to get a waiver is a repo in the
compiled-in allowed set, compiled in as `VisibilityPrivate`, with a complete, readable
changed-file list matching none of that repo's triggers.

Adopting this gate for your own repositories (the three modes, the callout JSON/exit
contract, and the fail-closed guarantees) is documented in
[`docs/desk-tools/risk-classification.md`](../../docs/desk-tools/risk-classification.md).

| input | classification |
|---|---|
| repo outside the fixed allowed set | risk-classed (no policy is never a waiver) |
| repo compiled in as PUBLIC | risk-classed |
| repo in the set with no stated visibility (zero value) | risk-classed |
| changed-file list empty | risk-classed (a PR always changes ≥1 file) |
| changed-file list short of GitHub's `changed_files` | **unverifiable** (exit 6) |
| a blank/malformed path entry | risk-classed |
| any path — or a rename's `previous_filename` — matches a trigger | risk-classed |
| otherwise | not risk-classed |

Triggers are the universal, generic base list (`secrets/`, `.github/workflows/`,
`k8s/*/rbac.yaml`) plus per-repo additions — `base/ deploy/ admin/ hack/ .github/workflows/`
for example-k8s, the desk's own gate code for the source repo. A per-repo list may only ever
ADD. A deployment's real product trigger paths ride the additive-only
`ASSAY_RISK_PATH_TRIGGERS_EXTRA` config, layered on top of the compiled base. **Every path
source — compiled base, per-repo list, and the env var — can only ever WIDEN the set; no
knob narrows it**: a knob that could narrow a security gate is a waiver waiting to be set.

### Visibility is compiled in — and drift-checked

`repoPolicy.Visibility` lives beside `CIRequired` in `deskkit/config.go`, not fetched at
decision time: a gate whose decision needs the network is a gate that fails when the
network does. The price of a hand-maintained value is staleness, which is the drift anti-pattern verbatim
— so it is paid for by a check, not by hope:

```
deskboard policydrift        # compiled-in visibility vs the GitHub API, GET-only
```

Exit 0 and a per-repo table when the table matches the world; **exit 6 and a loud
stderr report** on any disagreement — a repo whose visibility flipped, a repo the check
could not read, a repo with no compiled-in value, or an API `visibility` string this
code does not understand (`internal`, `""`). A repo it could not observe counts as
drift, never as a pass. Run it after any repo visibility change, and before trusting a
flip on a repo whose status you are unsure of.

## deskpr — worker draft-PR verb (brief 04)

`cmd/deskpr` is the worker's push-and-open-draft tool. It is **draft-only by
construction**: `gh pr create` is always called with `--draft` (no flag omits it), git
argv is built literally so `--force`/`--force-with-lease` can never be emitted, and
there is no ready/edit/close/merge verb. Two subcommands:

- `deskpr create --title T (--body-file F | --body-min B) [--base main]` — verify
  preconditions → `git push -u origin <branch>` → `gh pr create --draft` → print the URL.
- `deskpr update` — follow-up push of the current branch to its EXISTING open draft PR
  (the fix→re-review hot path); refuses if no open PR exists for the branch or it is not
  a draft.

Preconditions re-verified in-tool (C-2, all fail closed): inside a git worktree on a
**non-default** branch (detached HEAD → exit 6; `main`/`master` → exit 5; unreadable
`origin/HEAD` → exit 6), ≥1 commit ahead of the default branch, `origin` ∈ the C-4 repo
set, and no staged-but-uncommitted changes. The shared secret scan (`deskkit.BodyCheck`)
runs over the title, branch name, body, and the diff-vs-default before any push (C-3,
best-effort). An open PR already on the head branch → idempotent noop printing its URL
(exit 0), never a duplicate (#140/#148 class).

### The logged scan override (`--force-scan-override`)

`deskpr create`, `deskpr update` and `deskreply` accept
`--force-scan-override "<why>"`, which proceeds past a **secret-scan refusal** and writes
an audit row naming the tool, the verb, the surface digest, the stated reason and the
identity that stated it. The reason is required and must be long enough to be one.

**Policy is unchanged: exit 5 without the flag is a STOP**, and it is still never a
fallback trigger. The flag exists because a hard block with no override had only two
outcomes once the scan was wrong, and the house paid for both: the PR stranded for days
(#775 — a fix left unpushed in two abandoned worktrees, because rewriting the flagged text
would have desynced a recorded Evidence row from what was actually run), or the worker left
the sanctioned transport for raw `git push` (#585), discarding every other guarantee the
wrapper provides and leaving no record that a scan was skipped. The override replaces an
INVISIBLE bypass with an AUDITED one; it does not soften the refusal. Refusal messages
advertise it, because #585's worker was told "no override flag by design".

Not everything is overridable: the impersonation guard (a body written in a configured
human's voice) is not, since "the operator asserts it is fine" is the assertion that guard
exists to disbelieve, and rewording is always available there.

Reviewing bypasses is one command:

```bash
grep scan_override ~/.config/assay/audit.jsonl
```

The scan's accuracy in both directions, the acceptance run behind these verbs, and the
proof each refusal can fail, are the ship bar in `docs/desk-tools-gate-bar.md`.

## deskboard health — default-branch health (#295)

A PR's own green check says the PR is fine **in isolation**; it says nothing about the
branch it merges into. Before this verb the board could not tell "main is healthy" from
"main's health was never checked", so the desk reviewed, approved, flipped and merged PR
`example-org/example-k8s#9` onto a `main` that had been red for 29 minutes.

```bash
deskboard health            # JSON (loops consume this)
deskboard health --table    # human table, alarms first
```

The same probe rides on `deskboard actions`: its header carries `mainHealth`, the
`MAIN-RED` / `MAIN-UNKNOWN` lines print **above** every row (a red `main` outranks a
merge onto it), and each row on an affected repo is flagged `baseBranchRed` with a note.
An **absent** `mainHealth` field means the verb did not probe — never that it found
nothing wrong. `prs`, `queue`, `awaiting`, `nextup`, `dispatch`, `todo`, `next`,
`next-up`, `scope`, `policydrift`, `stalled`,
`reviews`, `diff` and `files` do not probe. That list is not prose upkeep: `TestNonProbingVerbs_OmitMainHealth` asserts
the absence for every verb on it (JSON **and** `--table`), `TestVerbInventory_Complete`
fails if a verb is added without being classified, and `TestREADME_NamesEveryNonProbingVerb`
fails if this sentence drifts from the guard.

**Three states, never two** (`docs/three-state-instrument-rule.md`):

| State | Meaning |
|-------|---------|
| `green` / `red` / `pending` | checked, with the commit the verdict was taken at. `green` = "green at that commit, for the jobs that ran there" — there is no per-repo expected-check list, so a path-filtered job set that passed reads green (the tracked `expected-checks per repo` gap, `docs/three-state-instrument-rule.md` → `follow-up: #33`) |
| `no-ci` / `no-commits` | nothing to assess, and it says which — not "green" |
| `unknown` | **could-not-check**: read failed, page cap hit, or a CI-running repo had no check runs anywhere in the lookback window |

**Noise budget.** A signal that fires constantly gets ignored, so: a `cancelled` run is
not red (on a default branch it is nearly always a concurrency supersede); a head commit
with no check runs is not an alarm but a reason to walk back up to 5 commits to the last
commit CI actually ran on (reported as `behindHead`); and only red/unknown print a line
each — everything else collapses into one always-printed summary that names the repo set
covered, so silence can never be mistaken for health.

**Read cost.** The probe is 2 `gh api` GETs per watched repo on the common path (one
`commits?`, one `check-runs`), worst case 1 + 5 when every commit in the lookback window
is check-less. `actions` is the desk's hot path, so this is the budget to watch as the
watched-repo set grows; nothing is cached.

**It annotates; it does not block.** deskpost owns the ready-flip gate. Whether a red
`main` should hard-block a flip on that repo is the open human call on #295.

## deskboard zero-CI disambiguation (#1652)

A `0✓ 0pend 0fail` CI cell used to cover two opposite facts — "checks never ran" and
"no checks are configured for this diff" — and the flip signal read both as green:
`#332` sat on the board with zero runs (CI genuinely never fired) and was
indistinguishable from `examples#24`, whose zero was legitimate path filtering. On a
board whose top line is MERGE-NOW that is the three-state rule
(`docs/three-state-instrument-rule.md`) violated in the dangerous direction: absence of
evidence rendered as evidence.

Every row whose rollup counts zero checks now goes through the **zero-CI probe**
(`cmd/deskboard/zeroci.go`), on `prs` and `actions` alike, and the cell names WHICH
zero it is:

| `ciZero` | Meaning | Green? |
|----------|---------|--------|
| `no-checks` | verified: no check runs and no commit statuses at head, AND no workflow at head would fire on this diff (no workflows, or branch/path filters exclude it) | vacuous green ONLY on repos with no PR CI (deskkit policy); the MERGE-NOW note annotates it as a CHECKED zero |
| `checks-never-ran` | verified: nothing ran AND ≥1 workflow at head WOULD fire on this diff — the PR is **UNVALIDATED** | never green, on any repo; the row classifies **CI-NEVER-RAN** (retrigger/investigate CI, never a flip) |
| `unverified` | **could-not-check**: a read failed, a list truncated, a workflow had a shape the parser does not model, check runs/statuses exist that the rollup counted as zero, or every run was SKIPPED/NEUTRAL (no verdict produced) | never green; the row classifies **CHECK** |

The JSON rows carry `ciZero` + `ciZeroDetail` (absent when the rollup counted at
least one check — absence never means green), and the `actions` audit line tallies the
never-ran/unverified rows so a later reader can see the probe ran.

Two flip-side rules follow directly: "CI green" requires at least one completed
successful check, not the absence of a failure (ask 2); and a distinction the probe
cannot compute is SAID on the line — `0 checks (unverified)` — never smoothed into a
bare zero (ask 3). The probe reads check runs + the combined commit status at the
head sha, the `.github/workflows` listing and each workflow's triggers at the head sha
(hand-rolled parser, dependency-free, **fail-closed**: any unmodelled shape is
`unverified`, never a guessed `no-checks`), and — only when a workflow carries a path
filter — the PR's changed files. It never errors the board: a failed probe degrades
its one row to `unverified`, the same contract as branch health. deskpost's ready
gate is unchanged (a `ciEmpty` on a CI-required repo is still exit 6 there); making
the gate consume the same distinction is the natural follow-up, not this PR.

## deskboard stalled — the stalled-draft detector

The review loop assumes a persistent worker: the reviewer requests changes, the worker
fixes and replies. Sessions die, so sometimes nobody replies — and nothing noticed. The
measurement that motivated this verb: **all 80 open PRs across at+tracker were drafts, 74 of
them sitting at CHANGES_REQUESTED with no responder.** `stalled` turns that silent state
into a routed event.

```bash
deskboard stalled                       # human table (the default — this is a scan verb)
deskboard stalled --json                # the dispatch shape loops consume
deskboard stalled --min-age-hours 24    # tighten/loosen the stall window (default 48)
```

Note the inverted default: every other deskboard verb emits JSON and takes `--table`;
`stalled` is a discovery verb read by a human or a desk, so it prints the table and takes
`--json`. Both carry the STALE/audit banners; passing `--table` **and** `--json` is a
refusal (exit 5) rather than a silent pick, as is a `--min-age-hours` this tool cannot
parse — a board run with the operator's window quietly swapped for the default is a board
they did not ask for.

**A PR is stalled when ALL of these hold**, and the row names the evidence for each:

| Condition | Signal read |
|-----------|-------------|
| open | `state` from the PR list — MERGED/CLOSED never appear (#247) |
| draft | `isDraft` |
| blocked at head | the reviewer App's latest decisive review, reduced by `deskkit.ReduceAppVerdict` |
| no push in the window | the head commit's **committer** date, when the commit is the **author's** |
| no author reply in the window | the latest PR conversation comment **by the PR author** |

Both clocks must be past the window. An author who pushed OR replied is working, not
stalled — and a comment by anyone else (a reviewer nagging an abandoned PR) does not
reset it.

**Both clocks are author-scoped, and that needs the two renderings of one identity.**
GitHub names the same App differently depending on the endpoint: `gh pr list --json
author` gives `app/<slug>`, while every REST payload (comments, reviews, commits) gives
`<slug>[bot]`. Comparing the two as plain strings answers "different actor" for **every**
App — and the fleet is largely App-authored, so the responder clock would be dead on
nearly every PR this verb exists to triage, while `lastAuthorCommentAt` reported "the
author never replied" as established fact. The fold lives once, in `deskkit.SameActor`.

The push clock counts only a push **attributable to the author**. `prsync --push` merges
`origin/main` and pushes, and the desk's loop tells workers to merge main on conflict; if
any such push reset the clock, a draft abandoned months ago would read as freshly pushed
forever. The row carries `lastPushBy` and `pushIsAuthors` so a recent `lastPushAt` on a
stalled row is legible rather than contradictory. A commit GitHub attributes to **no**
account is UNKNOWN, not foreign — it stays on the author's clock, because inventing a
stall from missing data is the same defect in the opposite direction. When nothing at all
is attributable to the author, the stall is measured from the decisive review — the
moment the ball entered their court.

**The canonical reduction, and its first consumer**
([#268](https://github.com/medici-finance/assay/issues/268)). `stalled` consumes
`deskkit.PRState` — head SHA, draft, open/merged/closed, the reduced App verdict, the CI
verdict — rather than re-deriving PR state locally. `PRUnknown` is the zero value of the
state kind on purpose: **"not stated" must never read as "open"**, which is the shape
[#247](https://github.com/medici-finance/assay/issues/247) named. Note what this
does **not** yet claim: `board.go`'s `reduceReviews` is still live and untouched (it
carries security-marker and approved-age concerns `deskkit` does not), so two reducers
exist today and migrating the older one remains outstanding against #268.

**Unassessable is a row, not a silence** ([#236](https://github.com/medici-finance/assay/issues/236)).
If a PR's verdict, push date, or comments cannot be read, that PR is reported as an
`unassessable` row naming **which** signal failed — it is never dropped, and never
resolved to stalled or clean. The rest of the board still renders and the run exits 0:
partial data is labelled, not fabricated. The granularity boundary is deliberate — a
whole-repo PR-list failure is **fatal** (exit 6), because "no stalled drafts in that
repo" would be a claim nothing verified.

The same rule applies to the enumeration's own limits: a repo whose open-PR list comes
back **at** the listing cap is named in a `truncated` array (and a `TRUNCATED` line in
the table), because some of its open PRs were never examined. The stderr WARN that
already existed is invisible to a loop consuming `--json` — the array is empty, never
absent, so "we enumerated everything" is a positive statement.

**Disposition is advisory.** `shepherd` is the default; `close-candidate` requires
positive evidence — the branch is more than 200 commits behind main, or its owning brief
is `done`/`superseded`. A signal that could not be read never produces a
`close-candidate` (an unreadable compare reports `behindMain: -1`, not `0`). PR ownership
is invisible: the desk or the human decides whether to adopt, and no loop auto-adopts.

Cron/loop wiring is not this verb's business — desks call `deskboard stalled --json` from
their existing loops.

## deskboard actions — HUMAN-GATE, CI-UNKNOWN, and honest tombstones

**`HUMAN-GATE` (#241).** The classifier read any App `APPROVED` at head plus green CI as
flip-and-merge-eligible, braked only by the risk-path/security pair. "Risk-classed" is a
*security-surface* list; **human-gate is a different set** — integrity-check logic,
irreversible operations, statusgen gate semantics — so `#223` (`[HUMAN GATE]` in its
title) was ranked **first** on the board as *"desk flips, then merge now"*. A declaration
the board can read now produces a terminal `HUMAN-GATE` action instead, and the row is
excluded from the MERGE-NOW count and decay banner. Any ONE of these declares it:

| Form | Example |
|---|---|
| label | `human-gate` (also `gate: human`; case-insensitive) |
| title | `[HUMAN GATE] …` / `[human-gate] …` |
| body line | `Gate: human` (mirrors the owning brief's key) |

Prose does **not** count — a gate that describes intent rather than asserting a
machine-checkable property is the defect #223 exists to close. Note the deliberate
narrowing versus the issue's suggested fix: a human gate bars the **merge** and the
done-flip, **not** the desk's ready-flip, so the note says both halves — *the desk may
flip it ready, then STOP; the merge is the human's*. The deskpost half of #241 (`ready`
refusing the flip on the same declaration) is **not** in this change and #241 stays open
for it.

**`CI-UNKNOWN` and the StatusContext blind spot (#268, divergence 3).** The rollup is a
union: `CheckRun` nodes carry `status`+`conclusion`, `StatusContext` nodes (legacy commit
statuses) carry `state` and no status. Only the first half was decoded, so every
StatusContext fell through `status != "COMPLETED"` and counted as **pending forever** —
a repo on commit statuses could never leave `WAIT-CI` while deskpost's REST reducer
called the same head green. Both shapes now decode, and a fourth counter (`ciUnknown`)
holds entries neither shape explains: it blocks the approve+green path rather than being
absorbed into `pending` (never clears) or `pass` (fail-open). The `?unk` term prints only
when there is something to report.

**Security-marker divergence (#268, divergence 2).** deskpost accepts
`(?i)^[ \t]*Security-Review:[ \t]*pass[ \t\r]*$`; this board requires the trimmed line to
be exactly `Security-Review: pass`. So `security-review: pass` flips via deskpost while
the board reports SECURITY-REVIEW-REQUIRED forever and the desk re-dispatches a review
deskpost considers done. The board does **not** widen its accepted set (its set is a
strict subset — the safe side); it now *reports* the disagreement, naming the offending
line and the canonical form. The structural fix (one PR-state snapshot in `deskkit` that
both tools render) is #268's own ask and is not done here.

**`CI-UNVERIFIED` — an empty rollup is not a green one (#400).** `CI-UNKNOWN` covered the
rollup entry the board *could not read*. It did not cover the rollup that carries **no
entry at all**: on a CI-required repo an empty or absent `statusCheckRollup` meant nothing
failed and nothing was pending, and the late `FLIP` arm — which never consulted `ciGreen`
— printed *"APPROVED at head, **CI green**, draft — desk flips ready"* over checks that
had never reported. `deskpost ready` refuses that identical state (`case ciEmpty` on a
CI-required repo → unverifiable, exit 6), so one tool exited 6 while the other recommended
the flip. The two now agree: **`CI-UNVERIFIED`**, naming the empty rollup. The same arm
also catches a rollup carrying only `SKIPPED`/`NEUTRAL` entries — no check reported a
verdict there either.

**`MERGE-STATE-UNKNOWN` — mergeability, read as four states (#400, #54).** `mergeConflict` was
`DIRTY || BLOCKED`, so **everything else** counted as mergeable — including `UNKNOWN`
(GitHub has not finished computing it) and `""` (the field could not be resolved), both of
which classified `MERGE-NOW`. `readMergeState` now maps the field to *mergeable /
blocked / could-not-check / behind*, and its **`default` arm is the load-bearing one**: `UNKNOWN`,
absent, and any value GitHub adds to the enum later all arrive as could-not-check. A draft
in that state still classifies `FLIP` (the ready-flip does not depend on mergeability, and
the note says so); a non-draft classifies `MERGE-STATE-UNKNOWN` and the merge
recommendation is withheld until the state is re-read.

**`MERGE-BEHIND` — the review-time/merge-time gap (#54).** `BEHIND` used to fold into the
same mergeable bucket as `CLEAN`. It isn't a could-not-check like `UNKNOWN` — it is GitHub
*measuring* that the base has moved since this head was last synced, which means the App's
`APPROVED` verdict verified the diff against a `main` that is no longer the merge target.
The retro that opened #54 found five near-misses in one evening where a stale-base merge
silently reverted already-landed work; none were caught by any gate, only by a human
diffing against merged `main` by hand. `readMergeState` now splits `BEHIND` into its own
verdict, and `classify` withholds `MERGE-NOW` on it exactly as it withholds it on
`MERGE-STATE-UNKNOWN`: a draft still classifies `FLIP` (draft-ness alone gates the
ready-flip), a non-draft classifies `MERGE-BEHIND`, and both notes say to sync with main and
get a fresh review before merging.

**`EXPECTED` is not a passing status (#400 T1).** The rollup is a union, and its legacy
half (`StatusContext`) carries GitHub's `StatusState`: `SUCCESS` / `PENDING` / `FAILURE` /
`ERROR` / **`EXPECTED`**. `EXPECTED` — *"Status is expected"*, i.e. a required context that
has been declared and has **not reported** — was mapped onto `pass` beside `SUCCESS`. One
such context, approved at head, non-draft, `CLEAN`, produced `pass=1 ciGreen=true
MERGE-NOW` with *"CI green"* in the note, off a check that had said nothing. It counts as
`pending`: a check still owed. The shape enumeration could not have caught this — it
enumerates rollup *shapes*, and this is a *value* — so `statusStateBuckets` now declares
the whole enum, and a parity check fails if `board.go`'s own list of the values that field
can hold grows past the table.

**The open-PR population states whether it is complete (#400 T2).** Every count the board
prints — `mergeNowCount`, `unreviewedCount`, the row set itself — rides on `gh pr list`
capped at `--limit 100`. A read *at* the cap may be short, and that was a **stderr warning
and nothing else**: the machine consumer reads the JSON, where a truncated population was
indistinguishable from a complete one. Every verb that reads a PR list now carries
`prPopulation` in its header — `complete`, the `cap`, and `truncatedRepos` — and the table
path prints `POPULATION TRUNCATED` naming the repos, saying in words that the counts below
are a floor. Three states as everywhere else: the field is **absent** on verbs that read no
PR list (`queue`, `awaiting`, `scope`, `health`), which never means "complete". `headOfPR`
gets the same distinction: over a capped read it says the PR *may be past the cut*, instead
of asserting it is not open.

> Both were found by *enumerating* rather than by adding another case: `TestClassify_ActionInventory`
> drives every rollup shape × every `mergeStateStatus` × every review state through the real
> `buildClassifyInput`, and fails if any ACTION the classifier defines is produced by nothing
> — the positive control that stops an absence assertion passing vacuously. It is what proved
> that **every `FLIP` the board had ever printed came from a row whose CI was never
> established**. `TestDispatch_VerbInventory` does the same for the verb set (parsed from the
> dispatcher, not hand-listed), and `TestReadme_VerbParity` for the verb list in this file.

**Tombstones say how a PR left (#247).** The `#209` lane labelled every vanished PR
`MERGED — drop from your list`, inferred from absence. Absence proves it left the open
set, not which way: `tracker#1580` was closed **unmerged** and was printed as MERGED, then
relayed to a human as fact — and the intake-desk's close-on-fix-landed lane makes an
irreversible write off exactly this bit. Each tombstone now reads the PR and reports
`merged` / `closed` (unmerged) / `open` (reopened) / `unknown`, with the `merged` boolean
**omitted** in the unknown case so a consumer cannot read a zero value as an answer.

## deskboard awaiting — the cross-repo AWAITING-VERIFICATION board

> **Renamed from `nextup` (#321).** `nextup` still works as an alias, and every report
> now states its population in-band. Measured on the live board: it returned 21 rows —
> 20 `implemented`, 1 `verified`, **zero `todo`** — and eight real Next-up briefs
> grepped against its output scored a hit count of 0. It is the **verification
> backlog**, and `implemented` work is exactly what a dispatcher must *not* hand out;
> a desk adopting it as a dispatch source sends every worker at finished work. The
> population is not a bug in this file — `statusgen --gate-scores` selects
> implemented/verified by construction — the NAME was the lie.
>
> **There is no dispatch queue here.** `deskboard dispatch` / `todo` / `next` /
> `next-up` are
> **refused** (exit 5) naming the reason, because an empty dispatch board would be
> indistinguishable from "nothing to pick up". Read Next-up from statusgen itself.

`deskboard awaiting` merges every configured repo root's awaiting-brief queue into one
board. It is the READ side of the multi-repo methodology (dispatch is a separate lane):
statusgen now emits one board per repo root, so the desk needs a single view across them.

```bash
# Documented invocation — from the tracker repo root, with the sibling checkouts in place:
deskboard awaiting            # JSON (loops consume this)
deskboard awaiting --table    # human table
deskboard nextup  --table     # deprecated alias; prints a NOTE saying so

# Point it at checkouts elsewhere (PATHS only — the repo set is compiled in):
DESK_ROOTS="example-org/tracker=/src/tracker,medici-finance/assay=/src/toolkit" \
  deskboard awaiting --table
```

JSON carries `population` (`awaiting-verification`), `populationStatuses`
(`["implemented","verified"]`), `populationNote`, and `aliasUsed` when reached through
`nextup`.

## deskboard scope — what the board covers, and what it does not (#359)

The board sweeps a **compiled-in** repo set. Rendering a confident, complete-looking
list of a subset means "no PRs in that repo" and "that repo is not read at all" produce
the same output, so a merge-ready PR in an unwatched repo is not missed — it is
structurally invisible, and nothing knows to alert. Two halves:

- **Every sweeping verb states its scope.** `prs`, `actions`, `queue`, `health`, `scope`,
  `policydrift` and `stalled` carry a `scope` object in the JSON header (repos, count, source) and
  print one scope line on `--table`. Verbs taking an explicit repo (`reviews`, `diff`,
  `files`) **omit** the field entirely — absent means "swept nothing", never "the set was
  empty". This sentence is not prose upkeep: `TestReadme_SweepingVerbSentence_359` fails
  if it names a subset of the verbs `verbScopeClass` declares as repo-sweeping — the
  defect this whole section is about, committed in its own documentation.
- **`deskboard scope` reconciles.** It derives the owner set from the watched repos,
  asks GitHub which repos under those owners have open PRs, and prints an `UNWATCHED:`
  line per repo the board does not read, with the oldest PR's age. A search it could
  not perform is exit 6 — never a clean report.

```bash
deskboard scope --table
# UNWATCHED: other-org/unwatched-repo has 1 open PR(s) the board does not read — oldest #1, open 369h
# scope-check: 9 watched · 9 repo(s) with open PRs observed under example-org, medici-finance · 3 UNWATCHED · observed via `gh search prs` under the CALLER'S token: repos it cannot see, and any owner outside the list above, are NOT observable here and are not counted in UNWATCHED
```

**What `scope` itself cannot see, said out loud.** `gh search prs` returns no rows *and
no error* for a repo the caller's token cannot read (private repo, App not installed, org
SSO not authorised), and it is only ever asked about the owners derived from the watched
set. So `gap: false` means **no gap was observed**, not **no gap exists** — the clause
above (JSON: `observabilityBound`) states that bound in-band, rather than letting the
reconciliation verb reproduce, one level up, the confident-subset defect it exists to
close.

**The temporal half of the same defect.** A review sweep covers what is open when it
starts and nothing ages an older PR back into view: measured 2026-08-02, every PR opened
after ~13:23 was reviewed and three opened before it never were, one of them another
window's work. `deskboard actions` now names any PR with **no reviewer verdict at any
head** that has been open past `--unreviewed-threshold` (default 30m):

```
UNREVIEWED: #391 has been open 47m with NO assay-reviewer-app[bot] verdict at any head (>30m0s) — a sweep starting now will not backfill it; pick it up explicitly
```

This line is a **neglect alarm, never the review trigger**: a fresh PR classifies
`NEEDS-REVIEW` immediately, at any age, and the desk's cadenced
`actions --delta --quiet` sweep is what dispatches it. The alarm firing means the
trigger path missed a PR. (The default was 2h while the quiet cadence loop had no
classified sweep — `--delta`/`--quiet` existed only on verbs without review state — so
this alarm was the desk's first loud signal and 2h had become the de-facto review
latency. 30m keeps it well past a healthy ~5-minute cadence while making a dead
trigger path loud in half an hour.)

The age is itself a three-state read. A never-reviewed PR whose `createdAt` is missing or
unparseable can be neither confirmed nor cleared against the threshold, so it gets its own
line rather than dropping out of the alarm — otherwise it renders as an ordinary
`NEEDS-REVIEW` row with a blank `OPEN` column and "never reviewed, age unknown" looks
exactly like "reviewed and fine":

```
UNREVIEWED-AGE-UNKNOWN: #391 has NO assay-reviewer-app[bot] verdict at any head and its open age could NOT be read (createdAt missing or unparseable) — it cannot be cleared against the 30m0s threshold; check it by hand
```

It is counted separately (`unreviewedAgeUnknownCount` / `unreviewedAgeUnknownPRs`, both
absent when empty) and deliberately **not** folded into `unreviewedCount`, which means
"aged past the threshold" — a measured claim that an unmeasured row must not join.

Silent on an ordinary board: a PR opened minutes ago, and any PR that *was* reviewed,
raise nothing — "reviewed and waiting" is not "never seen", and only the second is an
alarm. Neither line appears when every open PR has been looked at.

| Knob | Meaning |
|------|---------|
| `DESK_ROOTS` | `"<owner>/<repo>=<path>,…"` — overrides root **paths**, replacing the defaults (`.` for tracker, `../assay`). A repo outside the fixed C-4 set is **refused** (exit 5): env configures location, never trust. |
| `STATUSGEN_BIN` | The statusgen binary to exec. Default: `statusgen` on `PATH` — install the **pinned** release; format and rules in [`docs/distribution.md` § the `.assay-versions` pin file](../../docs/distribution.md#the-assay-versions-pin-file). |

**It runs the pinned binary, never a source tree.** The canonical `statusgen` source is
[`statusgen/`](../../statusgen/README.md) at this repo's root; every consumer — including
this one — takes it as a **pinned release binary** verified by sha256 against
`.assay-versions`. `nextup` therefore execs the installed `statusgen` and never `go run`s a
checked-out copy, so the board is always generated by the binary the pin names.

> This paragraph previously described a `tools/statusgen` frozen copy with CI-tripwired Go
> files. That layout was the **consumer** repo's, and it has since been deleted there
> (verified 2026-08-02: `example-org/tracker` has no `tools/statusgen`), and
> it never existed in this repo at all. The dead
> `tools/statusgen/README.md` link is fixed above. Neither was visible to `--lint`'s
> linkcheck, which walks `docs/**` only — see
> [#274](https://github.com/medici-finance/assay/issues/274) finding 4.

**Fail-closed (C-10), and this one is the point of the subcommand.** ANY root error —
unreadable root, a root with no `docs/streams/`, a non-zero statusgen exit, unparseable
`--gate-scores` JSON, a missing `.assay-versions` statusgen pin — aborts the whole run
with a non-zero exit naming the root. It never warns-and-continues, because a partial
board is *worse* than no board: a short queue reads as "nothing open" and the desk acts
on it. For the same reason the empty board states how many roots it read rather than
printing nothing, and the coverage lines print the **resolved** root path, so `roots`
and every row's `root` name the same directory.

**The statusgen pin is discovered, not compiled in (#511).** `nextup` reads
`.assay-versions` from *whichever* configured root actually carries the file — it walks
the resolved roots in their sorted order and uses the first one with a readable pin —
rather than requiring one specific repo (`example-org/tracker`) to be
among the configured roots. That used to make the board unrunnable for any adopter whose
`DESK_ROOTS` names only their own repo: the only way to satisfy the old check was naming
a third party's repo in `DESK_ROOTS` too, which in turn required widening
`ASSAY_ALLOWED_REPOS` — the desk's *write* authority — just to read a version pin. A root
with no pin file is not an error (the statusgen source repo itself never
carries one); a root whose pin file is present but unreadable or malformed fails closed
immediately, naming that root. The repo the pin actually came from rides in the JSON as
`statusgenPinRepo` and in the `--table` header line.

**Validating a pin file.** `deskpins --check --root <dir>` (built by `make desk-build` into
`tools/desk/dist/`) validates `<dir>/.assay-versions` against the published contract
([`docs/distribution.md` § the `.assay-versions` pin file](../../docs/distribution.md#the-assay-versions-pin-file))
and reports three-state — `checked-clean` / `checked-failed` / `could-not-check` — rejecting a
missing sha256, a malformed or non-namespaced tag, and a duplicate artifact **name** (the
identical-tag/sha `statusgen` + `statusgen-linux-amd64` pair is NOT a duplicate). The generalised
reader behind it (`deskkit.ArtifactPin`, with `StatusgenPin` a thin wrapper) preserves the
trailing-space prefix match, so a lookup for `desk-tools` never matches `desk-tools-linux-amd64`.

**Repo attribution must agree with configuration.** Multi-root statusgen stamps each
`--gate-scores` row with the repo the root itself declares (stream `repo:` frontmatter).
That declaration is checked against the repo the root was *configured* under, never
preferred over it: a disagreement is fail-closed (exit 6) naming both sides. Otherwise a
checkout could re-attribute its briefs to any repo string — including one `DESK_ROOTS`
would have refused outright — and silent misattribution is the failure this board exists
to eliminate. A row with no `repo` field (pre-multi-root statusgen) takes the configured
repo.

Version skew between the running statusgen and the `.assay-versions` pin is **reported,
not fatal**: `statusgenSkew` in the JSON header and a `WARN` line on `--table`.
`--gate-scores` is per-root regardless of statusgen version, so refusing would cost
availability for no correctness gain — but the skew stays visible so it cannot rot.

## deskboard `--delta` / `--quiet` — console discipline

The standing desk windows sweep the board every loop and print the whole thing each time,
so the signal — a needs-decision item, a verdict, an exception — drowns in iteration
noise. `--delta` and `--quiet` reshape stdout on `prs`, `actions`, `queue`, and `nextup`:

```bash
deskboard prs --quiet           # prs: 51 open (48 draft) | Δ +2 ~1 -1 | 3 ci-red/conflicting (see `actions` for the ACTION class)
deskboard prs --delta           # only the rows that CHANGED since the last invocation
deskboard prs --quiet --delta   # the quiet line first, then the changed rows
deskboard actions --quiet --delta   # the CLASSIFIED board's quiet line + changed rows —
                                    # the review desk's cadence sweep (see below)
```

**`actions` is the review desk's cadence verb.** Its quiet line counts
**NEEDS-REVIEW + RE-REVIEW** — the dispatch gate — so one
`deskboard actions --delta --quiet` per cadence tick surfaces a fresh PR as actionable
within one tick, **at any age**. (Before `actions` carried these flags, the quiet loop
could only sweep `prs`, whose payload has no review state — so the first loud "this PR
was missed" signal was the UNREVIEWED neglect banner at the threshold age, and the
neglect alarm had become the de-facto review trigger.) The quiet line's summary also
restates the standing counts every sweep — MERGE-NOW / FLIP buckets and the
UNREVIEWED / DECAY alarms — because `--delta` silences an unchanged row after its first
sighting and those are standing duties, not transitions. Row signatures track
classification state (action, draft, CI counts, risk/security/human-gate); ages, score
and note are deliberately excluded so the ordinary passage of time never marks every
row changed.

| Flag | Behaviour |
|------|-----------|
| `--delta` | Prints only rows that are **new (`+`), field-changed (`~`), or removed (`-`)** since the prior invocation. Nothing moved → one `no change (N items unchanged)` line. |
| `--quiet` | One line: the state-bucket summary, the `Δ` segment, and a labelled attention count. Prints **no rows** — by design. Composable with `--delta`. |

Both select the **text** path; they are refused (exit 5) on a subcommand that does not
support them, so a desk that relied on a flag for noise discipline never has it silently
ignored. Machine consumers pass neither and keep the JSON output unchanged.

The third segment is **not** a repeat of the summary count. Each subcommand labels the
subset it can actually prove: `prs` counts only CI-red / un-mergeable rows, because the
`prs` payload carries no review state — the full ACTION class needs `deskboard actions`.
`actions` counts `NEEDS-REVIEW + RE-REVIEW` (the review desk's dispatch gate);
`nextup` counts `todo + in-progress`; `queue` counts the verify-gate issues, which are
actionable by definition.

**The baseline is what the desk was SHOWN, not what the tool read.** A `--quiet`-only run
prints counts, never rows, so it does **not** advance the snapshot. Its `Δ` segment is an
unread badge that persists — and keeps accumulating — until a run actually renders the
rows. The consuming runs are `--delta` and `--quiet --delta`; `--quiet` alone always holds
(recorded in the audit line as `snapshot=held(quiet-only)`).

Without that hold, the natural loop shape would eat its own signal: a quiet line says
`Δ +1`, the desk drills in with `--delta`, and the follow-up compares against the
already-advanced state and answers `no change` — the new row unrecoverable through the
delta path. That is the fail-**dangerous** direction (PR #483 review, finding 3), pinned by
`TestQuietOnly_HoldsBaseline`.

**The snapshot, and why every failure is noisy.** State lives in
`~/.config/assay/snapshots/<sub>-<repoSetHash>.json`, keyed by (subcommand, repo set)
so a changed `DESK_ROOTS` can never diff against the wrong baseline. The fail-**dangerous**
direction is a bad diff that HIDES a new actionable row while the tool exits 0, so every
unassessable path falls back to FULL output with a label — the #236 lesson applied to
stdout. What each state prints differs by flag, because `--quiet` alone has no rows to
fall back to:

| Snapshot state | `--delta` (with or without `--quiet`) | `--quiet` alone |
|----------------|---------------------------------------|-----------------|
| absent | full output + `first run — no prior snapshot` | `Δ first run` — the label, never a `Δ 0`; baseline **held** |
| corrupt / unreadable / schema-mismatched | full output + `snapshot reset` (the bad file is overwritten by this run, healing it) | `Δ reset`; baseline **held** (the bad file is left for the next rendering run to heal) |
| state dir unresolvable, or a report shape its extractor cannot read | full output + a reset label + a `WARNING` on stderr | identical — this path renders the full report and warns regardless of flag |
| usable | the delta rows, or `no change (N items unchanged)` | `Δ +a ~c -r`, or `Δ 0` |

Read together: **`--delta` never prints an empty diff, and `--quiet` never prints a `Δ` it
did not verify** — on an untrusted baseline it names the state (`Δ first run` / `Δ reset`)
instead of quoting a zero. The tool may be noisy by accident, never quiet by accident. The
corollary of the hold is that a quiet-**only** loop keeps reporting `Δ first run` rather
than adopting an unseen board as already-seen; one `--delta` establishes the baseline.

Snapshots advance **only on a successful full read that rendered the rows** — deskboard's
fetchers are already fail-closed (C-10), so a `gh`/API error aborts (exit 6) before a
partial read could poison the baseline, and the next diff is always against known-good
state the desk has seen. Each run still writes its mandatory audit line (C-5), with the
delta recorded in `Detail` (`delta=+2~1-0`, `delta=first-run`, `delta=reset`, plus
` snapshot=held(quiet-only)` on a non-consuming run).

**Known limit.** Both modes report *transitions*, not standing state: a row that has been
actionable for days is silent after its first sighting, and the quiet line's count gives no
way to enumerate it. A window that owes a class-1 "always print actionable" contract needs a
periodic full sweep (`--table`, or `deskboard actions`) alongside the quiet loop.

The **skill half** of this — the one-line-per-quiet-iteration console contract the role
windows adopt, since no binary can gate what the model prints between tool calls — is the
console noise floor, tracked in the desk-tools operator notes.

## Build & install

```bash
make desk-build          # unprivileged local build → tools/desk/dist/ (agents/CI ok)
sudo make desk-install   # HUMAN ONLY — see below
```

**`sudo make desk-install` is a HUMAN act (C-1).** Agents never run it. It installs
each `cmd/*` binary to `/opt/desk-tools/bin/<tool>`, **root-owned 0755** — the sudo
password is the manual permission gate; agents cannot write there. It stamps each
binary with `-ldflags "-X …/deskkit.SourceSHA=<sha> -X …/deskkit.BuiltAt=<ts>"` and
writes `tools/desk/MANIFEST.sha256` for drift verification. That manifest is a
**local, generated artifact — it is NOT committed** (`git ls-tree origin/main tools/desk/`
lists no such file); it describes only what is installed on this machine.

The install target builds as the invoking user (`$SUDO_USER`) and then installs the
result as root, so the module cache is not written as root. Both `desk-build` and
`desk-install` guard the `cmd/*` glob and succeed with zero commands present (as today,
before briefs 02-05/07 add binaries).

## deskpost — the reviewer App's verdict / comment / ready-flip (brief 03)

`deskpost` posts the review verdict, plain comments, and the draft→ready flip **AS the
reviewer App** (`assay-reviewer-app[bot]`) — the review-gate identity a plain
worker session cannot post as (the stronger "unforgeable"
framing is retired — `docs/adopting-assay.md` §1a, `docs/messaging-guide.md`. The
same retired phrase still sits in `deskpost`'s own source comments; that residue is
tracked in #395, not fixed here). The reviewer App is one of the six-App **assay** desk-App family
(reviewer / worker / verifier / desk / issue-loop / intake-loop); the canonical provisioning
record — App IDs and installation IDs — lives in operator-private deployment config,
not in this tree. The App ID is never baked into source: it comes from per-deployment
config (`REVIEWER_APP_ID`, exported by the config home's `apps.env`) and `deskpost` fails loud
if it is unset. It absorbs the App-token mint from
`~/.claude/skills/pr-review-desk/mint-reviewer-token.go` (same env: `REVIEWER_APP_ID`,
`REVIEWER_INSTALL_ID`, `REVIEWER_PEM`; same endpoints) but holds the installation
token **in memory only** — it mints fresh per invocation and never caches it to disk.

```bash
deskpost review  <owner/repo> <pr>     --verdict approve|request-changes --head <full-40-char-sha> --body-file F
deskpost comment <owner/repo> <number> --body-file F     # <number> = a PR **or** an issue
deskpost ready   <owner/repo> <pr>

# modifiers, accepted by all three verbs, both off by default:
deskpost review  ... --dry-run        # run every check, stop before the write
deskpost review  ... --wait 30m       # ride out a rate-limit refusal instead of falling back
```

**`--dry-run` — rehearse the write for free (#355).** Runs every check in the same order the
real invocation does and stops immediately before the one mutating call: exit 0, audited
`dryrun`, a result class **neither meter counts** (deskkit `chargesBudget` and
`breakerIgnores` both exclude it, same treatment `deskrelease --dry-run` got in #214). It
waives nothing — a body that a real post would refuse, a dry run refuses identically, with
the same exit code and the same audit line.

Why it exists: before it, the only way to discover that a body would be refused was to
**spend a charged attempt**, one offender at a time. #355 records the ratchet — 49 → 37 → 33
characters across three charged attempts on a single body — and its consequence: the
pressure to get past the scanner led a reviewer to split the reviewed PR's **own head SHA**
across a backtick boundary, corrupting the one field a verdict's verifiability rests on. Free
rehearsal removes the pressure without loosening a single check.

**`--wait <dur>` — the budget refusal degrades to waiting, not to a weaker command (#197).**
On exit 4, waits in-tool up to `<dur>` (max 90m — the budget window is a rolling hour) for
the budget to free, then retries. Bounded and explicit: a retry-after longer than `<dur>`
returns the plain exit 4 and says why.

Use this instead of the raw `gh pr review` fallback. That command has **no head-pinning
flag**: on #195 a `CHANGES_REQUESTED` written against `151ebe99` landed on `2bbf529c`, a
commit that had already fixed two of its three findings. Under a saturated limiter the
fallback is the *common* path, so `--head` — the guarantee that a verdict names the code it
assessed — was off more often than on. Looping is safe by construction: a `ratelimited`
audit line neither charges the budget nor feeds the non-progress breaker, so waiting cannot
extend its own wait (#209).

`--dry-run` and `--wait` are mutually exclusive (a rehearsal has no write to wait for).

Why the App identity is load-bearing: a PR is authored by the `example-org` account, and
GitHub blocks an author from approving its own PR. Only a holder of the App private key can
post as the bot, and `deskpost` mints from that key alone — it **never** falls back to the
caller's `gh` token. So a worker cannot forge an approval, and the ready-flip's "the App
APPROVED at the current head" precondition cannot be satisfied by anyone but the App.

Constraints in code:

- **`review`** requires `--head` — the SHA the verdict was formed against, as the **FULL
  40-char lowercase hex** SHA (`gh pr view <N> --json headRefOid -q .headRefOid`). An
  abbreviated or otherwise malformed value is a **usage error (exit 2, nothing audited)**
  naming the form, not a head mismatch (#214): a short SHA can never equal the resolved
  head, so without the form gate it surfaced as a "mismatch" between two SHAs differing
  only in length — which reads as *the branch moved, re-review it* when the fix is *pass
  the full SHA*. It then refuses (exit 5) if the PR's current head genuinely differs: a
  verdict must never land on unreviewed code. The review is submitted with
  `commit_id = head`, pinning it.
- **`security-review`** posts the **security** verdict — same arguments as
  `review`, same body checks, same `--head` assertion, same trust gate, budget, audit line
  and both idempotency guards (it is literally the same write path, `postVerdictReview`).
  What differs is the GitHub **event**, and that is the whole of the fix for
  #513/#438:
  - `--verdict pass` submits the **COMMENT event** → review state **`COMMENTED`**. It is a
    *review*, so it carries a `commit_id` that `ready`'s gate (e) can read and head-pin on.
    That `commit_id` is **caller-supplied** on `POST /pulls/{n}/reviews`, not server-derived
    — it is truthful because deskpost verifies it against the freshly-read live head before
    submitting (same assertion as `review`, above). And `COMMENTED` never enters GitHub's
    approval reduction, so the board is **not** flipped to APPROVED while correctness
    findings stand.
  - `--verdict fail` submits **REQUEST_CHANGES**, unchanged — a retraction stays as loud as
    GitHub can make it.

  The verb refuses (exit 5, no network) a body that is not a security body, and one whose
  `Security-Review:` line disagrees with `--verdict`.

  **Never post a clean security pass as a plain `comment`.** `ready` reads
  `GET /pulls/{n}/reviews` and nothing else — a comment-shaped pass is invisible to gate
  (e), so a risk-classed PR whose security lane legitimately passed could not be flipped
  through deskpost at all (five live artifacts on 2026-08-07, incl. #455). It
  failed CLOSED — a blocked flip, not an exposure — which is why it went unnoticed for as
  long as it did.
- **`ready`** re-verifies EVERY precondition in-tool immediately before flipping (it never
  trusts the caller): PR open+draft; the App's latest **correctness** verdict is APPROVED at
  the current head; CI green at that head; **no standing `Security-Review: fail` at the head
  (risk-classed or not)**; and — for a **risk-classed** PR — the App's **last** security
  verdict at the head is `Security-Review: pass` (#216). A PR is
  risk-classed when its repo is **PUBLIC** (unconditionally — the public-repo risk rule), when its repo's
  visibility is not stated in the compiled-in policy, when the changed-file list could not
  be read in full, or when any changed path (**including a rename's previous path**)
  matches one of that repo's path triggers. That reduction is
  order-sensitive on purpose: a `pass` later retracted by a `Security-Review: fail` at the
  same head is NOT green, and a body carrying both markers fails closed. **The two verdict
  lanes are reduced SEPARATELY** (#238): both kinds submit as the same GitHub event, so a
  single login+state reduction let a `Security-Review: pass` (an APPROVED) satisfy the
  correctness gate over a live correctness `request-changes` at the same head. A body whose
  kind cannot be read counts toward the CORRECTNESS lane only — never the security one.
  A security **pass** is counted only from a review whose state is **`APPROVED`** (the
  legacy `review --verdict approve` shape) or **`COMMENTED`** (what `security-review
  --verdict pass` posts) — an allow-list, so `PENDING` (an unsubmitted draft, which this
  endpoint serves to its own author) and `DISMISSED` grant nothing. A **fail** still counts
  from any state. It re-reads the head one
  last time immediately before the flip (TOCTOU) and refuses if it moved. `ready` only flips
  draft→ready; there is no un-ready, merge, close, edit, or label verb (C-7). Ready =
  "ready for HUMAN review"; the merge stays Ada's.
- **`comment` targets a PR *or* an issue** (#296). GitHub numbers issues and pull requests
  from ONE per-repo sequence, so `<number>` names exactly one object — deskpost resolves
  WHICH KIND against the API before posting (PR read first; a 404 there is re-resolved
  through the issues endpoint, whose `pull_request` sub-object is the documented
  discriminator). There is no `--issue` / `--pr` flag on purpose: a caller-declared kind is
  a second source of truth that can disagree with the remote, and the remote decides where
  the comment lands. Both kinds get the same repo gate, body checks, trust gate, write
  budget, audit line and idempotency; the PR idempotency key is unchanged
  (`comment:<digest>` at the head), an issue keys on `comment:issue:<digest>` with no head.
  `--head` is **optional** here (an issue has no head at all) and **enforced when given**:
  the comment refuses (exit 5, nothing posted) if the PR head has moved since the comment
  was written, and the assertion runs BEFORE the idempotency read so a stale head can never
  be answered with a success-shaped no-op. Without it the tool stamps whichever head is live
  at POST time — on #505 the reviewer examined `f9c9ca94` and `d7b2d1d3` was
  recorded, because a merge landed mid-review; that one was a clean keep-current merge so
  the conclusions still held, but the tool guaranteed nothing (#513).
  `review`, `security-review` and `ready` stay PR-only and, given an issue number,
  **refuse (exit 5) naming `comment`** rather than reporting exit 6. That distinction is the second half of #296:
  6 means "the write may or may not have landed, verify before retrying", which is exactly
  what a lookup 404 rules out — and 6 charges the outward-write budget where a refusal does
  not. Until this landed the desk could not comment on an issue at all, so the register
  updates that are most of its outward writes went out through a raw `gh issue comment`,
  outside every control in this list. A non-404 failure from the PR read (403, 5xx,
  transport) is NEVER re-resolved — it says nothing about the kind, so it propagates
  unchanged rather than letting a permission failure be answered as "an issue".
- **A 403 is reported as a permission ANSWER, not a transient failure (#348 / #252).** Every
  App-token call names, on a 403, the permission that gates the endpoint (`statuses: read`
  for the legacy combined-status rollup, `checks: read` for check-runs, `pull_requests:` /
  `issues:` for the rest) and the installation the token was minted for. Both CI grants are
  required, not one: `ready` reads **both** rollups and aborts on either, which is why
  #252's proposed partial fix — fall back to check-runs when combined-status 403s — could
  not have worked. The exit code is unchanged (6, fail-closed); what changed is that
  `returned HTTP 403` no longer reads like a network blip. It read like one for ten days:
  22 × 403 on the status endpoint, 24 of 30 `ready` invocations lost to exit 6, and every
  one of those flips re-run through a raw `gh pr ready` that verifies nothing. A gate that
  fails closed into an ungated fallback is not failing closed, and the refusal now says so.
- **Bodies** are read from `--body-file` only (no stdin/inline), capped at 16 KiB, and run
  through deskkit's shared secret scan; there is no override flag.
- Every invocation runs the kill switch first (exit 3 when disabled), is rate-limited to
  10 real outward writes / tool / rolling hour and a 5-strike non-progress breaker (both
  exit 4), is idempotent (a repeat is a no-op), and
  audits exactly one line. Exit codes: `0` ok/noop · `3` disabled · `4` rate-limited · `5`
  refused · `6` unverifiable · `2` usage. The App token / PEM / body material is NEVER
  written to the audit log, stdout, or any file (the audit `detail` carries only counts,
  SHAs, and the refusal reason).
- **On exit 4, read the `retry-after:` in the message, sleep it ONCE, and attempt ONCE — or
  hand the artifact back to the desk. NEVER arm a poll loop.** Exit 4 now comes from one of
  two gates and the message says which: the **outward-write budget** (10 real writes per
  tool per rolling hour) or the **circuit breaker** (5 consecutive attempts that changed
  nothing — that is a loop, and retrying it without fixing the input is pointless). Neither
  gate is fed by its own refusals any more (#209), so a retry storm can no longer stall
  every other writer the way it did on 2026-07-24 and 2026-07-30 — but each retry still
  appends an audit line for nothing, and the budget is shared across concurrent writers of
  the same tool. On the breaker, the fix is the input: the refusal reason is in the audit
  `detail`.
- **If you fall back to raw `gh pr review` on exit 4, RE-ASSERT THE HEAD FIRST** (#197).
  `gh pr review` has no head-pinning flag: it attaches the verdict to whatever the head is
  at post time. Observed on #195 — a `CHANGES_REQUESTED` written against `151ebe99` landed
  on `2bbf529c`, a commit that had already fixed two of its three findings; both commands
  exited 0 and nothing in the output said so. A stale `CHANGES_REQUESTED` blocks a flip on
  closed findings; a stale `APPROVED` certifies a commit nobody reviewed. `deskpost review`
  now prints the assertion recipe itself on exit 4 — read its stderr rather than
  reconstructing the command from memory.

### Verdict format (the machine-checkable review body)

Review bodies (`deskpost review`) must carry, in addition to the size cap + secret scan:

1. at least one Markdown **H2 heading** (`## …`), and
2. a **verdict line** — one of:
   - `Verdict: approve` or `Verdict: request-changes` (the correctness verdict), or
   - `Security-Review: pass` or `Security-Review: fail` (the security verdict).

The security-review body follows the security-review spec verbatim: it starts with a `## Security
review` heading and carries the literal line `Security-Review: pass` or
`Security-Review: fail`. **Post it with `deskpost security-review --verdict pass|fail`**,
not with `review` — the pass shape is a COMMENT-event review (see the verb list above).
`deskpost review --verdict approve` with a security body still works and is still read at
the gate, so historical artifacts keep counting; it is the legacy shape, and it flips the
board's review state to APPROVED, which is what `security-review` exists to avoid.
`deskpost ready` and `deskboard`
each reduce the App's security verdicts **at the current head** in submission order and take
the LAST one per author: `pass` satisfies the #216 gate, `fail` retracts an earlier `pass`,
a review carrying neither marker changes nothing, and a body carrying BOTH is ambiguous and
counts as `fail`. Absence of any verdict at head is never a pass.

**Write the verdict line BARE — emphasis around it is not the format.** `**Security-Review:
pass**` is what verdicts were written as in good faith before #219 anchored the parse, and
an anchored regexp does not match one; the gate then reported "no security verdict at head"
on PRs that had one, and could not see a `**Security-Review: fail**` RETRACTION at all
(#232). The **write** gate refuses an emphasised line (exit 5), so today's artifacts stay
canonical. The **read** path unwraps a **surrounding** run of `*`, `_` or backticks before
matching — purely so the historical bodies remain legible. It is a compatibility shim, not
a second accepted format, and it is deliberately STRUCTURAL rather than a delete-everywhere
strip, because deleting `*` also deletes a Markdown bullet's `*` and turns a list item into
a verdict. What is unwrapped is a run of emphasis characters at the very start or end of the
trimmed line and **immediately adjacent to the marker text** — so `**Security-Review: pass**`
reads, while `* Security-Review: pass` (a star bullet), `- Security-Review: pass`, a quoted
(`> `), table-cell or mid-sentence marker, and a marker with emphasis spliced *inside* a word
(`Security-Rev*iew: pass`) all still are not verdicts. Lines inside a fenced code block, and
the fence delimiter lines themselves, are not verdicts either — **on the PASS path only**. A
`fail` marker is read wherever it appears, including inside a fence: the two exclusions run
in the same direction (a documentation example never GRANTS a pass; a retraction is never
silently unread), which is the only pair of choices under which a fence cannot be used to
hide a verdict from the gate.

The marker reduction is **one shared reader, not two separate implementations**
(#408). It lives in `deskkit.HasSecurityReviewPass` /
`deskkit.HasSecurityReviewFail` (`tools/desk/internal/deskkit/verdictmarker.go`) —
deskkit, not deskpost's `bodycheck` package, because deskkit is the only tree both
deskpost's `cmd/deskpost/internal/bodycheck` (a Go "internal" package, importable only by
code rooted under `cmd/deskpost/...`) and `deskboard/board.go` (a sibling tree) can reach.
`bodycheck.HasSecurityReviewPass`/`Fail` delegate to the deskkit functions verbatim, and
`deskboard`'s `hasSecurityPassLine`/`hasSecurityFailLine` call the same deskkit functions
directly, so both surfaces classify a body on the identical, emphasis-tolerant,
case-insensitive, fence-aware reading — including an emphasised retraction like
`**Security-Review: fail**`, which board.go's prior hand-rolled exact-match compare could
not see. The risk-CLASSIFICATION was already shared (`deskkit.RiskPathTriggered`); the
marker reduction now is too. The authoritative gate is still deskpost's `ready`, which
re-verifies in-tool immediately before the flip — `deskboard` is advisory, but it now
states the correct reason.

**Exactly ONE verdict kind per body, and the kind is part of the idempotency key** (#220).
The two lines name the two *different* artifacts a risk-classed PR requires at one head,
and in the all-clear case both used to post as `--verdict approve` — so the flag alone does
not identify the write. The audit/idempotency verb is `review:<kind>:<flag>`
(`review:security:pass` vs `review:correctness:approve`); keyed on the flag alone, the
second verdict to arrive was matched as a repeat of the first and dropped as a silent
"idempotent no-op", leaving the gate with one artifact where it requires two. A body
carrying BOTH lines is **refused** at post time (exit 5, no network) rather than guessed
at; to reference the other lane's verdict in prose, quote it (`> Security-Review: pass`).
The two rules compose in the safe direction: #220 stops such a body being *written*, and
#216's read-side reduction above still counts any that already exist as `fail`.
Git SHAs (exactly 40- or 64-char lowercase hex) pass the secret scan — the methodology
quotes them constantly. **`file:line` references pass too**
(`tools/desk/internal/deskkit/bodycheck.go:45`), absolute or repo-relative, as do Go module
paths and pasted `go test` output. Findings are *mandated* to cite their evidence that way,
and the scanner used to refuse any path run ≥32 chars that was not rooted under a mac-home
prefix — so the denser and better-evidenced a verdict was, the more certainly it was
refused, and each refusal then spent outward-write budget (#209, #1255). What still
refuses is opaque material between slashes: a token whose segments are not word-shaped is
not a path, whatever its punctuation. Plain comments (`deskpost comment`) get the size cap
+ secret scan only; they carry no verdict schema. Brief 06 wires the loop skills to this
format.

## deskwt — worktree lifecycle: add / remove / prune (brief 05)

`cmd/deskwt` makes the isolate-first rule (CLAUDE.md "Parallel sessions & worktrees") the
path of least resistance, and makes cleanup safe by construction. Every verb acts ONLY on
paths that RESOLVE under the sanctioned prefixes `/private/tmp/tracker-*` or
`<repo-root>/.claude/worktrees/`, refuses the shared checkout by IDENTITY (git-common-dir's
parent), and there is **no `--force` flag anywhere**. It is a local-only verb class: it
takes the C-5 audit line and the C-6 kill switch but NOT the outward-write rate limit.

```bash
deskwt add <name> [--branch B] [--base origin/main]   # create tracker-<name> on a tracking branch
deskwt remove <path>                                   # remove ONE proven-safe worktree
deskwt prune [--repo <path>] [--interval <dur>]        # bulk-reduce stale worktrees, safely
deskwt prune --reclaim-stale-locks [--lock-ttl 24h]    # …and retire locks whose session is gone
```

- **`remove`** refuses a dirty TRACKED tree, unpushed commits, a no-upstream branch, an
  unregistered path, or anything resolving outside the prefixes; untracked build artifacts
  (`node_modules`, `target/dist`, …) do NOT block. It shares its tracked-clean check and its
  delete+prune+verify step with `prune` (one implementation, no drift).

- **`prune`** exists because stale worktrees accumulate (the desk loops spawn one per worker
  and rarely remove them — ~1065 observed) until tooling breaks: the bash sandbox hits
  **E2BIG** (one deny-path per registered worktree) and the #742 writeguard mis-classifies
  top-level worktrees as the shared checkout. It runs two steps:
  1. **Step A (always safe):** `git worktree prune` — drops admin entries whose directories
     are already gone (pure bookkeeping, no working tree touched). Unconditional: no flag
     turns it on or off.
  2. **Step A2 (opt-in, `--reclaim-stale-locks`):** the lock lifecycle — see below.
  3. **Step B (safe gate):** walk the registered worktrees under the sanctioned prefixes and
     remove — via the exact same safe primitive as `remove` — ONLY those it can prove safe:
     **not** the shared checkout, **not** the current worktree, **tracked-clean**, AND
     **fully merged into `origin/main`** (`git merge-base --is-ancestor HEAD origin/main`; a
     detached HEAD that is an ancestor also qualifies). The merge check is the
     **active-worker guard** — an UNMERGED branch (an open PR still in flight) is never an
     ancestor of `origin/main`, so it is LEFT untouched. Dirty / unpushed / unmerged / the
     current worktree / out-of-prefix are all LEFT and reported as skipped with a reason. No
     `--force`. `--repo <path>` targets a repo without a session cwd (used by the multi-repo
     sweep script). Exit `0` ok/noop · `3` disabled · `5` refused · `6` unverifiable.

Every sweep prints one summary line with four counts — **pruned** (bookkeeping entries
dropped), **removed**, **held** (with **locked-held** broken out), and **locks-reclaimed** —
so an operator can tell a repo that is draining from one that is stuck, and on which gate.

### `--reclaim-stale-locks` — giving the lock a lifecycle

A locked worktree is always held: git refuses to deregister it, so deleting its directory
first would strand the registration (#264). But nothing ever *unlocks* one. A session locks
its worktree at boot (`role-init`, and the loop skills' cooperative
`git worktree lock --reason "<role> live session session=$CLAUDE_SESSION_ID"`) and then
simply ends — so a dead session's lock is **permanent**, the locked population grows
monotonically, and a lock on a worktree whose directory is already *gone* even keeps Step A
from dropping the dangling admin entry. Measured on a busy checkout: hundreds of registered
worktrees, dozens of them locked and prune-immune, some pointing at paths that no longer
exist.

`--reclaim-stale-locks` (default **OFF**) retires the locks it can prove stale. It is
deliberately weak:

- It only **unlocks**. It removes nothing. Every gate in Step B then runs on the unlocked
  worktree unchanged — dirty / unpushed / unmerged / fresh-at-tip are still LEFT — so a
  reclaim can never lose work.
- It acts only on worktrees under the sanctioned prefixes; never the shared checkout, never
  the current worktree.
- It acts on **evidence**, in this order:
  1. **Session death (preferred).** The lock reason carries a `session=<id>` stamp; that id
     is looked up in the roster beacons the desk already keeps
     (`<StateDir()>/roster/<session>.json`, re-stamped with `updated` while the session
     lives). No beacon, or a beacon that stopped being re-stamped for an hour, is positive
     evidence the session is gone. A beacon inside the window is positive evidence it is
     **alive**, and a live session's lock is held even past its TTL.
  2. **Age (`--lock-ttl`, default `0` = disabled).** Git keeps exactly one timestamp for a
     lock — the mtime of the `locked` file in the worktree's admin directory. Older than the
     TTL ⇒ stale. This is the fallback for locks that name no session. `--lock-ttl` without
     `--reclaim-stale-locks` is **refused** rather than silently inert.
- Anything else is held: an unreadable roster, an unparseable beacon timestamp, a missing
  admin file. None of them are evidence of death, so none of them reclaim a lock.
- Every unlock prints the worktree, the lock reason it carried, and why it was judged stale.

### The prune loop — one command, two supervisors

The **periodic** prune mechanism is the portable ticking mode, identical on the mac and in
a k8s desk pod:

```bash
deskwt prune --interval 30m     # sweep, sleep 30m, sweep, … forever
```

`--interval` (Go `time.ParseDuration`, e.g. `30m`) loops the same safe sweep, logs one
summary line per tick (timestamp + counts) to **stdout** (captured by pod logs), re-checks
the C-6 kill switch / STOP flags between ticks (clean exit 3 on STOP/DISABLED), and exits 0
on SIGTERM/SIGINT for a clean shutdown. No external scheduler is involved — the loop travels
with whatever runs it. Only the **supervisor** differs by environment:

- **k8s desk pod:** run `deskwt prune --interval 30m` as a sidecar / in the pod supervisor;
  the pod's `restartPolicy` keeps it alive.
- **local mac:** the `launchd` agent `scripts/launchd/<reverse-dns-label>.deskwt-prune.plist`
  (`RunAtLoad` + `KeepAlive`) keeps the same command running across login and crash.
  **Installing/loading it is Ada's act** (a machine action — like `sudo make desk-install`);
  no agent loads it. Install steps: `scripts/launchd/README.md`.

Each desk **loop skill** (the-desk, worker-desk, pr-review-desk, verify-desk, issue-loop)
also runs a one-shot `deskwt prune` at boot so a session starts on a pruned workspace; the
`--interval` supervisor is the steady-state timer. For a manual sweep across all sibling
repos, `scripts/deskwt-prune-all.sh` invokes `deskwt prune --repo <path>` per existing repo
(one-shot; safe with no session active).

## Operator reference

Ported from #1169, re-verified against this tree. Where the original recorded a fact
that is no longer true — or that was never true here — it is corrected below rather than
transcribed.

### Install / upgrade

Install is a **human act** (C-1). The sudo password IS the manual permission gate — agents
never run it, and no brief automates it.

```bash
make desk-build          # unprivileged; builds every cmd/* into tools/desk/dist/
sudo make desk-install   # HUMAN ONLY
```

`desk-install` (root `Makefile`) does four things, in order:

1. Re-runs `desk-build` **as the invoking user** (`$SUDO_USER`, never root) so the Go
   module cache is not written as root, stamping each binary with the `SourceSHA` /
   `BuiltAt` ldflags.
2. Installs the already-built `dist/` binaries to `/opt/desk-tools/bin/<tool>`,
   root-owned 0755 — agents cannot overwrite them.
3. Runs `desk-hook-install`: copies `tools/desk/hooks/pre-push` to `.githooks/pre-push`
   (the `deskpushguard` shim, activated by the repo's `core.hooksPath = .githooks`). It is
   idempotent, and **refuses to clobber** a pre-existing non-`deskpushguard` pre-push hook
   unless invoked with `FORCE=1`.
4. Runs `desk-manifest`: writes `tools/desk/MANIFEST.sha256` from the installed binaries.

`/opt/desk-tools/bin` is deliberately **not on `PATH`** — every desk tool is invoked by its
absolute path, which is what makes the `Bash(/opt/desk-tools/bin/<tool> …)` allow rules
anchor on a specific installed binary rather than on whatever a name resolves to.

**Post-install drift check.** `desk-manifest` generates the manifest with
`( cd /opt/desk-tools/bin && shasum -a 256 * )`, so its entries are **bare basenames** and
`shasum -c` resolves them against the **current directory**. It must therefore be run from
the install dir, against an absolute manifest path — running it from the repo root makes
`shasum` look for `deskboard` in the repo root and fail to open every entry:

```bash
( cd /opt/desk-tools/bin && shasum -a 256 -c /path/to/assay/tools/desk/MANIFEST.sha256 )
```

### App credentials (`desktoken`, `deskpost`, `deskevidence`)

All three tools mint App tokens, and all three resolve the role PEM and `apps.env` across the
same **App-credential search path** (`$ASSAY_CONFIG_HOME`, then `~/.config/assay`);
`desktoken` writes its token cache to the head of that path. The mechanism, the reasoning and
its scope limits are documented once, under
[Where the App credentials live — the #794 ruling](#where-the-app-credentials-live--the-794-ruling).

Worth stating here because it is the half that is easy to miss: `deskpost` and `deskevidence`
do **not** shell out to `desktoken`. Each signs its own JWT and mints its own installation
token, so fixing the resolution in `desktoken` alone left both of them reading from a
hardcoded `~/.config/assay` — a deployment provisioning elsewhere still could not post a
review or write an Evidence commit from a cold shell, with the App **ID** resolving fine off
the search path, so the failure read as a broken key rather than a wrong directory.

`<ROLE>_PEM` and `<ROLE>_TOKEN` still override an individual file outright, in all three.

### Version check (stale-binary detection)

Every tool with a `--version` flag reports its embedded stamp in one shape:

```bash
/opt/desk-tools/bin/deskboard --version     # deskboard sourceSHA=<sha> builtAt=<ts>
```

A **STALE signal** means the `tools/desk` tree at the installed binary's `sourceSHA`
differs from the `tools/desk` tree at `origin/main` (`staleState`, `cmd/deskboard/main.go`).

**How the signal surfaces depends on the output mode, and the default is not the banner:**

- **default (JSON)** — the path the loops consume — writes *nothing* to stderr. The signal
  rides in the JSON header as `stale` (bool) and `staleDetail` (string).
- **`--table`** — the human path — additionally prints the STALE banner to stderr
  (`printBanners`, "--table path only").

So a boot step must read the `stale` header field from the JSON; grepping stderr for a
banner finds nothing on the default path.

**Three states, and `stale` fails closed (#236).** `stale` used to report `false` when
the check could not run at all, with `staleDetail` saying "drift not assessable" — the
boolean and its own detail line contradicting each other, on the field the guidance
names as authoritative. A fail-open on a drift detector is the one answer it must never
give. The header now carries `staleState` beside it:

| `staleState` | `stale` | Meaning |
|---|---|---|
| `in-sync` | `false` | checked: the installed tree matches `origin/main` |
| `drift` | `true` | checked: they differ — reinstall (banner `STALE:`) |
| `not-applicable` | `false` | unpinned `go run`: there is no installed binary that *could* drift. A statement about the world, not a failed measurement — and it stays quiet, because alarming on every developer run is how a signal gets ignored |
| `unknown` | **`true`** | **COULD-NOT-CHECK** — git unavailable or refs missing. Reported as stale, with banner `STALE-UNKNOWN:`, because an unverifiable drift check is not evidence of freshness |

To check by hand — this mirrors what `staleState` actually does. Prefer the `stale` header
field; this block is for when you only have a shell:

```bash
INSTALLED=$(/opt/desk-tools/bin/deskboard --version 2>&1 | sed -n 's/.*sourceSHA=\([^ ]*\).*/\1/p')
git fetch -q --no-tags origin main
if [ "$INSTALLED" = unpinned ]; then
  echo "unpinned build — no install to drift from"
elif [ "$(git rev-parse "${INSTALLED}:tools/desk")" != "$(git rev-parse origin/main:tools/desk)" ]; then
  echo "STALE: installed $INSTALLED — re-run sudo make desk-install"
fi
```

**The braces around `${INSTALLED}` are load-bearing in zsh, which is the shell the desk
runs.** Written `"$INSTALLED:tools/desk"`, zsh parses the `:t` as the *parameter modifier*
`:t` (tail) even inside double quotes, so the expansion becomes `<sha>ools/desk`,
`git rev-parse` errors, the left side is empty, and the comparison never matches — the
block then prints STALE on **every** run, drift or not, and can never report "current".
Verified in `zsh` both directions with `${INSTALLED}`: identical trees print nothing,
a genuinely divergent `origin/main` prints STALE. `bash` is unaffected either way.

Four things a hand-rolled check has to get right, each of which a naive version gets wrong:

- `--version` prints `deskboard sourceSHA=<sha> builtAt=<ts>`, so field 2 is the whole
  `sourceSHA=<sha>` token — the value must be split off the `=`, not taken with
  `awk '{print $2}'`.
- The embedded `SourceSHA` is a **commit** sha (`Makefile`, `git rev-parse --short HEAD`),
  so it must be resolved to a tree with `<sha>:tools/desk` before comparing. Comparing it
  directly against a tree object id compares two different object types and never matches.
- The comparison is against **`origin/main`**, not local `HEAD` — a branch merely ahead of
  or behind main is not drift.
- An unpinned build reports `sourceSHA=unpinned` (`internal/deskkit/version.go`), which is
  not a rev — `git rev-parse unpinned:tools/desk` fails and a check without the guard line
  above would call it STALE, the opposite of the tool's own answer ("no install to drift
  from"). Short-circuit it.

### Kill switch and stop flags

Arm and disarm are file operations under `~/.config/assay/`; the flag semantics,
the `HEARTBEAT` dead-man lease, and the precedence chain
`DISABLED` > `STOP` > `STOP.<name>` are specified once in
[Runtime state](#runtime-state-not-created-by-this-repo) above.

```bash
touch ~/.config/assay/DISABLED                        # arm — every tool exits 3
echo "maintenance window" > ~/.config/assay/DISABLED  # optional reason (first line shown)
rm ~/.config/assay/DISABLED                           # disarm
export DESK_TOOLS_DISABLED=1                               # env-var arm (same effect)
```

`Guard` writes a `result=disabled` audit line on every refusal while armed, and one
`kill-switch disarmed (DISABLED cleared)` line on the first invocation after the file is
removed — so a disarm is visible after the fact rather than silent. The flag file lives in
an agent-writable directory: it stops a faithful runaway loop, which is the incident class
it exists for, not an adversarial one.

### Rollback

Remove the installed binaries and the pre-push shim:

```bash
sudo rm -rf /opt/desk-tools          # every installed desk tool
rm .githooks/pre-push                # the deskpushguard shim (only if you installed it here)
```

After rollback, loops fall back to their documented raw-command fallback paths (each skill
lists them for exits 3/5/6). Consumer repos that grant `Bash(/opt/desk-tools/bin/…)` allow
rules keep those rules; with nothing installed at the path they are inert, and they are
managed in the consumer repo, not here.

### Guard, the two meters, and repo scope

Every tool in the [Tool reference](#tool-reference) table calls `deskkit.Guard()` first
(C-6) and writes exactly one audit line
(C-5); exit codes are `0` ok/noop · `3` disabled · `4` rate-limited · `5` refused ·
`6` unverifiable. **Four** do not call `Guard()`, each for a stated reason — and those four
are the whole exemption set, verified per-tool against the source. Three are ordinary
exemptions: `writeguard` and `muhar` are local diagnostics that make no outward write, and
the CI gate (`desksourceguard`) runs in GitHub
Actions, not in a desk session. The fourth, `deskpushguard`, is the deliberate inversion — it
skips `Guard` because an armed kill switch must not stop it (a halted desk still must not
orphan commits by pushing to a merged branch), and it fails **OPEN**, allowing the push on
any ambiguity or error.

**"Rate-limited" now means two meters, not one** (#213, closing #209 — the table in
#1169 predated this and said only "10/hr"):

- **The write budget** — now **two conjunctive tiers**, both per tool per rolling hour
  (#439): `RateLimitPerPRPerHour` = **20 per PR/issue** (raised 10→20 by PR #1053,
  2026-08-14 — a supervised-drain loosening, not a re-measurement) and `RateLimitPerRepoPerHour` =
  **100 per repo**. Both must admit the write. The per-PR tier is the old
  `RateLimitPerHour` number re-scoped, so one runaway agent on PR #431 can no longer
  starve PR #424; the per-repo tier keeps a fleet of agents spread across many PRs from
  collectively flooding one repo. Only results that **may have reached the remote** charge
  either (`ok`, `unverifiable`, and anything unclassified, failing closed). A `refused` no
  longer charges: a body-check or guard refusal is a local reject with no amplification to
  cap. Neither do `noop`, `ratelimited` or `disabled`.

  **A missing scope narrows the budget; it never removes one.** A write that *records* no
  PR number (`deskevidence`'s branch commits, a `deskrelease` cut) lands in the repo's
  **unnumbered bucket**, capped at the same 20 — not waved through to the 100. A call site
  with no repo at all falls back to a **tool-wide** budget, also at 20, which is exactly the
  retired single-tier behaviour. The first draft of the two tiers skipped a tier per missing
  field instead, which silently moved three live write paths from 10/hr to 100/hr and left
  `deskrelease` bounded only by the breaker — and the breaker counts *consecutive
  non-progress*, so a run of successful releases never trips it.

  **The bucket a gate reads must be the bucket its writes land in.** Narrowing a scope is
  not enough on its own: a gate can be aimed at a bucket the call site never fills, which
  reads like a tight cap and enforces nothing. `deskpr create` is the case that proved it —
  it gated on the unnumbered bucket while auditing every success with the *real number of
  the PR it had just created*, so no successful create could land in the bucket its own gate
  counted. It ran at 100/hr behind a comment claiming 10, and the meter was inverted: ten
  *failed* creates (which record no number) locked the tool out for an hour while
  ninety-nine successes did not move it.

  Creates therefore gate on `AllowWriteRepoWide` — every charged write the tool made on that
  repo, held at the **per-PR cap**. That scope is a superset of wherever a create's line
  lands, whatever number it gets, and it keeps the created number in the audit trail (the
  one field that makes a create traceable) rather than blanking it to force the buckets to
  agree. `TestGateReadsTheBucketItsWritesLandIn` pins the invariant for every gated call
  site: seed the shape that call site really writes, and its own gate must refuse.

  **The cap values are pinned by test** (`TestCapValuesArePinned`). Every other test seeds
  by reading the constant the code reads, so raising 10 → 20 was green across the whole
  suite; the numbers are policy, and changing one now fails loudly and wants its throughput
  argument updated alongside.
- **The circuit breaker** — `BreakerTrip` = **5 consecutive non-progress attempts**
  (`refused` / `noop`) opens it for `BreakerCooldown` = **15 minutes**, measured from the
  most recent such attempt. `ok` / `unverifiable` reset it. **Scoped to the same
  (repo, PR) bucket as the per-PR budget** (#447: the 2026-08-06 outage — one PR's
  oversized-body refusal loop halted every desk write on three repos): a run on
  `repo#392` blocks `repo#392` only, and its cooldown clock is its own — an unrelated
  refusal elsewhere neither trips it nor extends it. A **tool-wide backstop** trips at
  `BreakerBackstopTrip` = **20** consecutive non-progress attempts across all targets
  (the storm spread too thin to form any per-target run), and its refusal names the
  run's members so a blameless blocked caller can attribute the stall instead of being
  told to "fix the input".

Both meters are immune to their own output, which is the property the single counter
lacked: a `ratelimited` line is invisible to both, so retries append only inert lines and
both clocks run down monotonically. Exit 4 comes from either gate and the message says
which; read the `retry-after:`, sleep it ONCE, attempt ONCE — never arm a poll loop.

Configuration reference — every variable, its level, its format and what UNSET means:
**`docs/roster-configuration.md`**.

**Repo scope is one CONFIGURED set, shared by every tool** (`internal/deskkit/config.go` +
`internal/deskkit/rosterconfig.go`, `ASSAY_ALLOWED_REPOS`; C-4). It is adopter configuration,
but configuration read from **outside every ref the tools evaluate** — a repository/organization
Actions variable, or the config-home file — so no flag, no PR-supplied input and no environment
read on the write path can widen it, and an UNSET set refuses every repo rather than admitting
them all. The effective set is echoed on every run, so a widening appears in run output and CI
history. There is no narrower write scope: the outward-writing tools gate on the same
`IsAllowedRepo` as the read-only boards.

An `ASSAY_ALLOWED_REPOS` entry is either explicit (`owner/name[:ci|:no-ci][:public|:private]`)
or a bare **PATTERN** `owner/*` (generalised to configuration): a pattern admits
every current *and future* repo under that owner to `IsAllowedRepo` with no further entry
required — this is how an operator reproduces the org-default write scope this brief
originally shipped as a compiled-in `owner/*` census. That census enumerated only a
subset of the owner's repositories, so a newly-created repo was invisible to the tools
the day it was created. A pattern carries **no** ci/visibility policy: it widens
`IsAllowedRepo` alone.

- `CIRequired` / `RepoVisibility` / `VisibilityDrift` / `AllowedRepos()` all read the
  **explicit** entries only — a repo matched only by a pattern reads as CI-required and
  visibility-UNKNOWN (both fail-closed) until an explicit entry states otherwise.
- `AllowedRepos()` never returns a pattern element — every caller passes each element to
  `gh api repos/<repo>`, so an `owner/*` slug there is a live break (C-10 fails the whole
  board run), not a cosmetic one. Patterns are display-only, in `AllowedRepoScope()`.
- Writes to a **public** repo additionally require a verified `+1` from the configured
  blessing authority (`ASSAY_BLESS_LOGIN`) on the associated issue/PR (the public-repo trust
  gate, `PublicRepoGate` / `IsBlessAuthorityIDStrict`); commands with no issue/PR number
  refuse outright there.

**The desk writes where it does not watch.** The write gates use `IsAllowedRepo`, which a
pattern widens; every *scan* — `deskboard prs/board/queue/policydrift`, `deskroster`,
`issueboard`, branch-health — iterates `AllowedRepos()`, the **explicit** set only. So a
pattern-matched repo is writable but never appears on a board, in the roster, or in a drift
report: `deskboard policydrift` builds its observed set from `AllowedRepos()`, so its
"observed but not in the allowed set" arm cannot fire and a pattern-only repo is reported as
nothing at all. Only the audit log (`~/.config/assay/audit.jsonl`) records a write to
one. Nothing forces a policy decision for such a repo — the fail-closed defaults are what
stands in, not a prompt. `deskboard policydrift` proves the **explicit** set still matches
the world; it says nothing about repos reachable only through a pattern.

**An explicit entry goes stale from outside this repository**, and no hermetic test can see
it: the other repo adds a `pull_request` workflow, or flips visibility, and the configured
value is wrong with nothing here changing (this is how `example-org/examples` and
`example-org/example-reconciler` went stale on CI, and `medici-finance/assay` on visibility,
back when these were compiled
in). Re-check the test-fixture set this package's own suite asserts against, against the
live API, with:

```bash
cd tools/desk && DESKKIT_LIVE_CENSUS=1 \
  go test ./internal/deskkit/ -run TestCensusMatchesLive -count=1
```

It checks both halves: every fixture row's `CIRequired` against that repo's live `on:`
blocks, and every **public** repo in the org against the fixture (a public repo absent, or
carried at the wrong visibility, fails). Run it whenever a real deployment's
`ASSAY_ALLOWED_REPOS` value changes, or a repo is created in the org it covers. It needs
network and an authenticated `gh`, so it is env-gated and does not run in CI.

The **parsing** it depends on is not env-gated. `internal/deskkit/cicensus_parse_test.go` holds
`workflowTriggersPullRequest` and a hermetic table over every `on:` spelling GitHub accepts —
mapping key, block sequence, inline sequence (quoted and bare), bare scalar, flow mapping,
`pull_request_target`, and the YAML-1.1 `true:` key a round-tripped workflow can carry. Three of
those spellings were missed before #310 R1, all in the fail-open direction: a missed spelling
reads as "no PR CI", which CONFIRMS a `CIRequired: false` row on a repo that does run PR CI.
That table runs in ordinary CI, so the matcher cannot be refactored blind.

A **private review channel is deliberately not part of that configured set.**
It used to be a literal entry compiled into this tree, which meant
the channel's repo name shipped inside every desk binary — the disclosure was the name,
not the comment above it, so stripping prose alone would have changed nothing. Converting
the set to adopter configuration carries that property forward rather than reopening it:
nothing in this source tree names the channel, so there is nothing here to ship. The desks
cannot write to one; sensitive review notes are filed there by a human.

**The allowed-repo set is not the token's scope.** It is a client-side refusal inside these tools, not a
server-side restriction: the minted installation token reaches **every repo in its account**
(`repository_selection: all` on both installations, measured 2026-08-02). A repo in the
installation but not in the configured set is still fully reachable — by a raw `gh api` call, a script,
or any session holding the token. The configured set is what the tools *act on*; the installation is
what they *can reach*. For a blast-radius question the answer is the installation.
See `docs/github-apps-setup.md` § "Actual installation scope".

## deskfile — the issue-filing gate (issue-flow brief 02)

`cmd/deskfile` makes the 2026-08-02 filing-discipline ruling binding instead of remembered:
**dedupe before filing, attach instances to class issues, budget filings.** An issue that
should have been a comment on a class issue never gets minted.

```bash
deskfile new    -R <owner/repo> --title <t> --body-file <f> [--label ...] [--raised-by <role>] [--force-new --reason <r>]
deskfile attach -R <owner/repo> --to <N> --body-file <f>
deskfile check  -R <owner/repo> --title <t>
```

- **`new`** runs a dedupe search over the repo's OPEN issues first. A candidate at/above the
  match threshold is a **refusal (exit 5)** that prints the candidates, their scores, and the
  exact `deskfile attach` command for the top one. Then the per-session new-issue budget
  (exit 4 over), then `gh issue create`.
- **`attach`** posts the observation as a comment on issue `N` — a class issue or a duplicate
  target. **Never charged to the new-issue budget**: attaching is the motion the gate exists
  to force, so it must never be the path the budget refuses. A CLOSED target refuses (exit 5)
  with reopen-or-new guidance; deskfile has no reopen verb (C-7).
- **`check`** is the dry run: same dedupe, same 0/5 exit, writes nothing. This is the verb
  skills embed in an authoring loop before composing a `new`.

### `--raised-by <role>` — the provenance stamp

`new` takes `--raised-by <role>`, which stamps the label `raised-by:<role>` on the issue it
files. The question it answers is **which loop NOTICED the problem** — not which App posted
it, which is what the issue author already records and is a different fact. Without it the
by-desk issue metric can only report `unattributed`, and the
self-improvement metric has no agent-raised signal to compute from.

**One declared source.** The role vocabulary is not a list in `deskfile`. It is DERIVED from
the roster's role-bindings — the `role=` prefixes on `ASSAY_TRUSTED_BOT_SLUGS`, already the
only place a desk role exists (`deskkit/raisedby.go`, `RaisedByRoles`). A role the roster does
not bind is **refused (exit 5)** with the bound set printed. That refusal is the one raised-by
condition that stops a filing, and it is a caller error with a fix in hand: stamping an
unbound role would mint a metric category nothing else will ever populate.

Note the consequence for the strings: the roles are the ROSTER's names (`reviewer`,
`verifier`, `worker`, `issue-loop`, `intake-loop`, `desk`), **not** the skill file names
(`pr-review-desk`, `verify-desk`, `worker-desk`, `intake-desk`). The skills' own
`--raised-by` instructions are copies, and `raisedbyskills_test.go` diffs them against the
roster so a skill naming an unbound role fails in CI rather than at exit 5 mid-filing.

**The stamp never blocks the filing.** `raised-by:` is a metric annotation, not a safety gate,
and the labels do not exist in any repo yet — 0 of 421 issues on the home repo carried one when
this shipped. A hard gate against an already-drifted corpus reds everything on day one and
teaches the fleet to route around the verb; the precedent is `statusgen/mergedstatus.go`,
which shipped its reconciliation at NOTICE severity for exactly that reason. So there are
**four outcomes**, three of which file the issue anyway and are distinguished on the audit
line:

| audit token | condition | what the issue reads as |
|---|---|---|
| `raised-by=<role>` | the label exists on the repo and was applied | stamped |
| `raised-by=UNSTAMPED:not-requested` | no `--raised-by` was given | **unknown** |
| `raised-by=UNSTAMPED:label-missing` | the role is valid, the repo has no such label | **unknown** |
| `raised-by=UNSTAMPED:could-not-check` | the label-existence probe went unanswered | **unknown** |

Each of the three unstamped outcomes prints a NOTICE, and the `label-missing` one prints the
exact one-off `gh label create` command. They are kept distinct because they need different
remedies — create the label, pass the flag, or investigate an outage — and a caller told
"create the label" during an API outage creates one that already exists and is still not
stamped.

**Why the label is probed before it is applied.** `gh issue create --label <missing>` fails
outright, so applying an unverified stamp would convert a missing metric label into a lost
filing — the annotation taking down the thing it annotates. deskfile does **not** create the
label itself: its mutating vocabulary is `issue create` and `issue comment`, and widening it
to `label create` for a metric is not a trade this tool makes. The labels are a one-off
human/admin action per repo, provisioned at adoption by the `create-labels` primitive
(`docs/adopting-assay.md`, CORE §3), which is the canonical list of the six `raised-by:*`
labels alongside `review-request`.

**UNKNOWN IS NOT "HUMAN-RAISED".** This is the load-bearing reading rule, and it belongs to
the consumer as much as to this tool. An unstamped issue is an issue whose provenance was
never recorded; a human-raised issue is a claim about who raised it. `deskkit.RaisedByOf`
returns three states — `stamped` / `unknown` / `indeterminate` (two conflicting `raised-by:`
labels, or one with an empty role) — with **unknown as the zero value**, so a consumer that
forgets to handle the state gets the non-answer rather than an answer. Two stamps is not
"pick the first"; it is a could-not-check.

**Backfill is a judgement, not a mechanical consequence.** The standing corpus is unstamped
and stays that way: nothing here bulk-labels history, and the count is reported rather than
silently absorbed. The measured backfill number and the decision behind it are recorded
in the operator's methodology-metrics notes.

**This gate is OPT-IN, not a chokepoint — yet.** Nothing here removes `gh issue create` or
the REST API, so every refusal below binds only a caller that chose to call `deskfile`.
Wiring the skills so that filing goes through it is **issue-flow brief 07**; until that
lands, read "gate" as "an available gated path", not as enforcement.

**Fail-closed direction.** A dedupe-search API failure makes `new` **exit 6, not proceed** —
minting a possibly-duplicate issue is the expensive error, and absence of duplicates cannot
be inferred from an unanswered search. So does a search that answers with **nothing at all**
(exit 0, empty stdout): a real `gh --json` prints `[]`, and "the search produced nothing"
must never read as "no duplicates exist". A title that normalises to **no scorable tokens**
(only stopwords, single characters or punctuation) refuses too — it could never match any
candidate, so passing it would be a free bypass of the matcher.

`--force-new --reason <r>` is the stated escape hatch for urgent filings during an outage: it
skips the search, requires a non-empty reason, and writes that reason into the audit line.
**The hatch stays usable during the outage it exists for**: a `new` that failed before the
`gh issue create` call — search outage, unreadable `--body-file` — sent nothing and therefore
charges no session budget. Only a create that was actually invoked charges, including one
whose outcome could not be confirmed (that one may have minted an issue, so not charging it
would be fail-open).

**The dedupe query is built from tokens, never the raw title.** The title is caller text on
its way into GitHub's search *query language*, where `word:value` is a qualifier. Desk titles
routinely carry colons ("bugs-gc: prune closed-issue files"), and a rescoped search returns a
candidate set without the duplicate in it — which reads to the gate as "no duplicate exists".
Normalised tokens are `[a-z0-9]` runs only, so nothing in a title can rescope the search.

**Remote text is sanitized at ingest.** Two repos in the fixed set are PUBLIC, so issue
titles are authored by arbitrary external users; candidate titles/URLs, `issue view`
state/URL, and `gh`'s own stderr all pass through `deskkit.StripControl` as they enter, not
at each print site. A crafted title cannot carry `ESC[K` / `CR` into an operator's terminal
or an agent's context.

**`check` audits as `dryrun`, and that is load-bearing.** `check` finding a duplicate is the
verb *working*, but it exits 5 — and an audit line saying `refused` is non-progress to
deskkit's breaker and, when a search outage makes it `unverifiable`, a charged write to
deskkit's budget. Both readings are wrong for a verb that writes nothing: a run of correct
dedupe hits opened a breaker against `attach` (the exact motion the refusal message names),
and `RateLimitPerPRPerHour` unanswered searches spent the repo's outward-**write** budget with
no write ever attempted. `check` performs no outward write on any path — deskkit's stated
precondition for `ResultDryRun` — and that result is invisible to both meters.

#454 scoped the primary breaker to the `(repo, PR)` bucket, which independently
separates `check` (no issue number) from `attach` (numbered). The `dryrun` result is still
what keeps `check` off `checkBreakerBackstop`, the tool-wide stop at
`BreakerBackstopTrip` consecutive non-progress lines, and off the write budget entirely.

`new`'s refusals still feed the breaker, and deliberately: `new` is a write verb that asked to
write and was told no. Following the guidance defuses it indefinitely — an `attach` succeeds,
which resets the run, so refuse-then-attach can repeat forever. No per-verb breaker exemption
is carved for `attach`: that is exactly the "call site opts out of a budget" shape
#439 removed from `ratelimit.go`, and after #454 it is not needed.

**The matcher prints its score.** Jaccard over normalised, stop-worded title tokens, plus a
small boost when the candidate carries the `class` label (a class issue is the canonical
collector for its instances, so a moderate match against one should redirect to attach). The
boost cannot manufacture a match from zero token overlap. Deliberately simple: for this gate
the human-audit trail is worth more than matcher cleverness.

**The new-issue budget is NEW accounting, not the C-5 limiter.** `RateLimitPerPRPerHour` has
no session dimension and no 24h window, so it cannot express "3 new issues per session per
rolling 24h". deskfile computes its own count over the audit log's `sessionTag` + `tool` +
`verb` + `repo` fields; `attach` lines never count. Both meters apply to `new` — this budget
AND the ordinary outward-write budget, which for `new` uses `AllowWriteRepoWide` (a create's
number cannot be known in advance, the #439 lesson).

Rotating `$CLAUDE_SESSION_ID` does reset the bucket — a new session is a new session — but
**every `new` audit line records the sessionTag it charged**, so rotating to buy budget
leaves a trail rather than erasing one. That trace is the control here, not a hard block.
Sessions with the variable unset all share the single `unknown` bucket (the conservative
direction). deskfile **gates WHETHER and WHERE, never WHO**: the caller's ambient `gh`
credential is the filing identity and no App token is ever minted. Repo scope comes from
`deskkit.IsAllowedRepo` — there is no second repo list, and a test parses the sources to
prove it.

## deskclose — the issue-CLOSING gate (issue-flow brief 03)

`cmd/deskclose` is the counterpart to `deskfile`. Filing is cheap and closing was
impossible, so the backlog only grew: the only sanctioned exit was a full fix-PR cycle and
the reject/duplicate exit sat at 2-4% of closes. `deskclose` makes a small number of narrow
close lanes executable — and makes every other close impossible rather than merely
forbidden.

```bash
deskclose duplicate      -R <owner/repo> <N> --of <M> --mined <summary>
deskclose superseded     -R <owner/repo> <N> --by <ref>
deskclose review-request -R <owner/repo> <N>
deskclose manifest       -R <owner/repo> --file <manifest.yaml> [--resume-from <N>] [--max-wait <dur>]
```

### The authorization gate is the whole tool

`deskclose` closes other people's work in bulk. The single property that makes that safe is
that every closure traces to a human-authored artifact that the tool **fetched and
verified** — never to a flag the caller set, a login the caller claimed, or a file the
caller wrote. Two gates, both fail-closed, both before any write:

1. **The ruling gate** (every mode). `R-1` in the issue-flow ruling register must
   carry a `Sign-off` URL, and that URL must resolve to a comment authored by the
   **configured blessing authority** (`deskkit.IsBlessAuthorityIDStrict` — one human login,
   pinned to a numeric user id). The file states the CLAIM; the fetch is what makes it
   authority, so editing the register in your own worktree only changes which URL gets
   fetched. **R-1 is unsigned today, so `deskclose` currently closes nothing in any mode.**
2. **The manifest gate** (manifest mode). The manifest names its own authorizing comment,
   and that comment's body must carry the manifest's content **digest**. Authorization
   therefore binds to an exact row set: an approval cannot be replayed onto a different
   manifest, and a row added, retargeted or re-moded after the human looked invalidates it.

Why login alone cannot gate this: the desk, the workers and the human reach GitHub through
overlapping identities, and a shared automation account reports `"type": "User"` exactly as
a person does. Type-checking is necessary and not sufficient — the author must be the
roster's single pinned blessing authority. `deskkit.TrustedAuthor` is deliberately NOT used
here: it is a much wider set containing every desk App, and using it would let the desk
authorize its own batch.

**There is no `--force`, no `--yes`, and no environment override.** A test parses the
package sources and fails on any of them, or on any `os.Getenv` call. The escape hatch for
"the human has not authorized this batch" is the human authorizing the batch.

### The refusal matrix

| Refusal | Exit | Why |
|---|---|---|
| unknown mode | 5 | the mode set is closed; the dispatcher's default arm is the enumeration |
| `needs-decision` / `human-decided` label | 5 | decision items exit via the decision digest (brief 06) where a human reads them one at a time — never a sweep; absolute, including manifest rows |
| referenced PR closed-unmerged | 5 | read from the **pulls** endpoint's `merged`, not the issues endpoint's `state`; work that did not land cannot supersede a live issue |
| review-request body naming 0 or ≥2 PRs | 5 | a guess here closes the wrong item |
| authorization by an App / a shared automation account / the right login with a wrong id | 5 | see above |
| the manifest's digest absent from the authorizing comment | 5 | the approval is for some other row set |
| unknown manifest key, mode, or missing `target`/`mined` | 5 | a dropped line is a row the human approved and the tool did something else with |
| repo (or a cross-repo target) outside `deskkit.IsAllowedRepo` | 5 | no second repo list, no widening flag |
| PR target recorded `NEEDS-REBASE` | 5 | the disposition record says live work |
| PR target with no disposition record | 5 | `deskclose` executes a finding; it does not make one |
| record Evidence naming a different target than the caller | 5 | the tool does not pick a winner between them |
| rulings file / label set / PR state / authorization / record **unreadable** | 6 | could-not-check is never authorization, and never "clean" |
| write budget spent | 4 | wait-and-resume — see below |

Every refusal above ships with a positive control in `deskclose_test.go`: the same scenario
with the one refused property fixed, asserted to reach exit 0 and perform its two writes. A
check never observed to both pass and fail is a comment that happens to compile.

### Batch discipline

A manifest applies **one row at a time**, each with its own audit line, and each row is a
comment **then** a close — two charged writes, in that order, so the trail naming the lane,
the canonical target and the authorizing artifact survives even a batch that turns out to be
wrong. Reopen is cheap; an unexplained close is not.

- **A hard error stops the run**, leaving every later row untouched, and prints the exact
  `--resume-from <N>` to continue with.
- **A rate-limit refusal is not an error.** Exit 4 makes the loop sleep for the stated
  retry-after and retry the **same row**. It never advances the cursor and never skips: a
  batch closer that silently skips is worse than one that stops, because the operator
  believes the batch is done. If the accumulated wait would exceed `--max-wait` the run
  exits cleanly on exit 4, still naming the unapplied row.
- **Already-closed rows are idempotent no-ops**, so a resumed run is always safe.

### Composition with the disposition record

A pull-request target must already carry a machine-readable disposition
record (the `disposition:<verdict>` label plus the `<!-- desk-disposition v1 -->` marker
comment). `deskclose` **reads** that record by shelling out to `deskdisposition read
--json`; it does not re-derive the verdict and does not parse the marker itself. Two copies
of that parser would be two things to drift, and the drift would be silent in the dangerous
direction — a marker `deskclose` failed to recognise reads as "no record", which on the
write side means "close it anyway". A `deskdisposition` that is absent or errors is
`could-not-check` (exit 6), never "no record found".

The split is deliberate and is stated on both sides: the record captures the **finding**
(a worker's terminal verdict, with evidence); `deskclose` supplies the **authorization** and
the execution. Neither half can close an item on its own.

## deskdigest — one batched weekly decision queue (issue-flow brief 06)

`cmd/deskdigest` turns N decision interrupts into one sitting. Measured live on 2026-08-13
across the two tracked repos, **21 open `needs-decision` issues** were queued on one person,
the oldest open since 2026-07-15 — 29 days. Each reached him as its own issue hoping to be
noticed, and several parked real work behind them. The digest batches them into one weekly
issue: repo#N, age, the question, an R-3 classification with its reason, the desk's
recommendation, and a default-if-no-answer.

```bash
deskdigest --repos <a,b> [--dry-run]                       # print the digest, write nothing
deskdigest --all-repos --post --digest-repo <owner/repo>   # create-or-update this week's issue
           [--rulings <path>] [--now <RFC3339>] [--week 2026-W33]
```

The read scope is **chosen, never defaulted**: pass `--repos <owner/repo,…>` to name the repos,
or `--all-repos` to read the roster's whole scan scope. A bare run reads nothing and refuses,
for the same reason `--post` refuses to default its write target — a probe that picks its own
targets is a probe nobody aimed.

### Three invariants, and they are why it exists

1. **An empty week is a REPORT, not a skip.** A digest that omits a week with nothing in it
   is byte-identical, from the reader's side, to a digest whose generator crashed, whose
   credential expired, or whose schedule was deleted. So the zero case says the most: it
   states `0 items`, names every repo it read, and says how the reader tells this apart from
   silence. `--post` publishes it.
2. **No safe default.** An item the engine cannot place renders `unclassified` — needs a look
   — and never silently as `reversible`. The desk-decides lane is entered only by a POSITIVE
   signal; nothing arrives there by exhausting the alternatives. The suite carries a
   positive control (`TestNoDefaultControlCanFail`) that runs the same predicate against a
   defaulting classifier and asserts it is caught.
3. **Could-not-read is its own state**, at three granularities: an item whose comment thread
   did not come back, a repo that did not answer, and a label that does not EXIST yet.
   That last one is not hypothetical — `gh issue list --label human-only` returns `[]` and
   exit 0 in a repo where the label has never been created, so without the label-inventory
   probe the digest would report an empty human-only queue every week for a label nobody
   has applied. `human-only` is in exactly that state in both tracked repos today.

### A note on the two names

The brief spells the classes `reversible | <the human>-only` and names the infra label after
the one human. Both ship here as **`human-only`**, because this tree's publication boundary
refuses a personal name or handle in source that stages for publication: `leaksweep`'s
`tree-sweep` fails the branch on either token, and "genericise to `human:<name>`" is the
standing instruction it prints. The substance is unchanged — `human-only` IS the brief's
lane, and the `human-only` label IS the brief's infra-asks label. Consequence for the next
brief in the stream: **a later sweep must apply `human-only`**, or the digest will
keep reporting a label nobody created.

### Classification (R-3), highest precedence first

| # | Input | Effect |
|---|---|---|
| 1 | a comment by the roster's blessing authority carrying `decision-class: …` | always wins; latest comment operative |
| 2 | the `human-only` label | human-only — an infra ask only the human can perform |
| 3 | `decision-class: …` in the issue body, **trusted author only** | override |
| 4 | R-3's own criteria | any irreversible/mechanism/security/spend signal → human-only; a one-commit-reversal signal and no human-only signal → reversible |
| 5 | nothing fired | **unclassified** |

Two of those rows carry a security property rather than a convenience. The blessing-authority
check is the strict id-pinned form (`IsBlessAuthorityIDStrict` plus `type == "User"`), because
a login is a claim about a name and shared automation accounts report `type: User` exactly as
a person does. And the body override is honoured **only for a trusted author**: the body of an
issue is written by whoever opened it, so obeying an untrusted one would let an outside author
move their own item into the lane the desk decides without asking. That is not an override, it
is a self-service authority grant; it is refused, and the item lands on `unclassified`.

The tie-break inside row 4 is deliberate: an item that is both a docs-wording change and a
security change is a security change. A false human-only costs one item staying in the human's
queue that could have left it. A false reversible costs a decision taken without them.

### R-3's standing, and what the digest says about it

`deskdigest` reports `R-3`'s sign-off standing from the issue-flow ruling register in the
digest's first section. It does not parse the register itself: it delegates to
`deskkit.ReadSignOff`, the house's ONE reader for "has a human signed this ruling?", and maps
that reader's three states onto its own trio. A rulings register records process authority no
agent holds, so every gate that consumes one must parse it the same way — a second parser is a
second answer, and the two drift silently. `deskclose` reads the same file through the same
function to ACT (it fetches the artifact and verifies its author before closing anything);
`deskdigest` reads it through the same function to REPORT (it prints the claim and never
fetches). Two verify steps on one parse, not two parsers.

Where the register carries no sign-off, the desk holds no delegated decision authority, the
reversible column is a **proposal for what could be delegated** rather than a claim that it has
been, and a reversible row renders `no default — R-3 unsigned, so the desk cannot decide it
either`. Where the register **records** a sign-off artifact — R-3 has carried one since
2026-08-13 (`pull/931`) — the digest reports the URL as a **claim** and a reversible row renders
`desk may decide under R-3; 7-day veto once recorded here`. Reporting the claim does not enact
anything: the digest still only prints a table, and the veto surface is the digest itself.
`deskdigest` labels the URL a claim and never verifies it — the tool that acts on R-3
(`deskclose`) is the one that fetches and verifies the artifact's author, because reporting
"R-3 is signed" on the strength of a file any caller can edit would put a false authority
statement in front of the one person the digest exists to inform.

Because the parse is the shared reader's, a truncated sign-off block, two sign-off blocks in
one section, and two URLs in one block are each **could-not-read** — never a false `unsigned`
against a signed register, and never a false `claimed` picked silently from among candidates.

### It reports, and only reports

The weekly issue is the ONE thing `deskdigest` writes. Everything adjacent belongs elsewhere
and is named rather than performed:

- superseding last week's digest is a **close**, so `deskclose superseded` is PRINTED for a
  human to run and never executed;
- filing anything else is `deskfile`'s;
- the decision-SLA / ESCALATE lane is `issueboard`'s — the age column here is
  plain item age, not a second threshold drifting alongside that one.

`TestReportOnlyNeverMutates` enumerates every argv the read paths construct and fails on any
mutating gh verb, which is what makes this a property of the binary rather than a claim in a
comment. The write path itself is `gh issue create` **or** `gh issue edit`, on the tool's own
weekly issue and nothing else.

### Weekly lifecycle

The ISO week is the identity. `Decision digest <ISO week>` is matched **exactly**: same week
→ the existing issue is edited in place, so a Monday digest re-run on Thursday does not mint a
second one; new week → no match → a new issue. There is no fuzzy fallback, because a
near-match is a DIFFERENT week's report and editing it would overwrite a record the human may
still be working through. Two issues with the identical title is a refusal, not a coin-flip.
A closed digest for the current week is refused rather than reopened — whether the week is
still live is a judgement, and this tool renders reports rather than making them.

**A partial read still publishes.** A repo that did not answer produces a digest that names
the gap under `Read gaps` and an exit 6 that carries the same news to the caller. A partial
digest that admits it is partial beats a silent week.

## deskmerge — merge-currency, reported honestly and fixed only with permission (issue-flow brief 09)

`cmd/deskmerge` answers one question about an open PR — **is it current with `main`?** — and,
once a human grants R-5, performs the one deterministic git operation that fixes it.

```bash
deskmerge check -R <owner/repo> <pr> [--probe] [--json] [--repo-root <dir>]
deskmerge merge -R <owner/repo> <pr> [--dry-run] [--probe] [--rulings <path>]
```

**Two verbs because there are two different authority questions.** `check` writes nothing
anywhere, so it is not ruling-gated — reading is not authorship — and it is the half that is
useful while R-5 is unsigned. `merge` reads R-5's `Sign-off` line, fetches the artifact it
names, and requires the author to be the roster-pinned blessing authority. **R-5 is unsigned
today, so `merge` refuses (exit 5) before touching git and merges nothing.**

### What it can and cannot detect

The failure class next door is *individually-green PRs that are invalid against each other*,
and a checker that only looks for textual conflicts sees none of it. So the boundary is
stated rather than implied:

- **Textual**: behind-count, would-conflict and where, conflicts confined to the compiled-in
  regenerable list. Computed by a real trial merge in a detached scratch worktree.
- **CI-contract drift**: what CI machinery `main` gained since the merge base
  (`.github/workflows`, `.github/scripts`). #898 added a script plus a job that
  runs it, so any older branch fails that check with exit 127 — a merge-currency failure
  wearing a defect's clothes. Reported separately, and NOT as a failure: merging cures it.
- **Semantic collisions with no textual conflict**: only under `--probe`, which BUILDS the
  merged tree and reports the compiler's verdict. Targets are DERIVED from the merged tree —
  every Go module either side touched since the merge base — so no repo name lives in this
  source, and a tree with no touched buildable module is could-not-check, not clean. This is the #912/#913 class
  (`emit()` grew an 8th parameter on main while a green PR still called it with 7 args — zero
  conflicts, two green PRs, red main). Without `--probe`, `semanticValidity` reads the literal
  string `not-checked`; it is never inferred from textual cleanliness.
- **Not detectable at all**: a semantic collision the compiler cannot see. `docs/brief-rules.md`
  carries duplicate rule numbers 25 and 26 today for exactly this reason.

Three states throughout: **0** clean or deskmerge-fixable · **5** checked-failed (conflicted,
or a failed probe — worker work) · **6** could-not-check. An unfetchable ref, a moved head, a
missing checkout or a merged tree with no touched buildable module are each **6**, never 0.

### The zero-authorship boundary, compiled in

| Guard | Behaviour |
|---|---|
| Direction | Merges `main` INTO the PR branch only. Never pushes to a default branch, never calls `gh pr merge`, never rebases. **Merge is the human's.** |
| Two-parent | After the commit and before the push, the COMMIT is inspected: exactly two parents, parent 1 = the PR head, parent 2 = the fetched base head. A fast-forward, rebase, squash or single-parent masquerade (#72) is refused with nothing pushed |
| Conflicts | Zero hunks, or every conflicted path on the compiled-in regenerable list and resolved by re-running its generator. A MIXED conflict is refused whole — no partial resolution. Regenerated files are scanned for conflict markers before staging |
| Flip bound | A PR already flipped ready is refused: a desk merge replaces the head that the desk's own approval-at-head check is evaluated against |
| Hygiene | The merge runs in a detached scratch worktree removed on every exit path |

Every refusal above carries a mutation in `cmd/deskmerge/mutations.json`, run by `muhar` in
CI: disarm it, and the suite must redden. Eight of them survived as first written and each
got the test that catches it (docs/desk-tools-gate-bar.md §4).

## deskevidence — Evidence commits as the verifier App

`cmd/deskevidence` commits an Evidence row (or a whole brief file) via the GitHub
**Contents API** as `assay-verifier-app[bot]`, replacing hand-editing + `git commit` in the
verify-desk loop.

```bash
deskevidence <owner/repo> <branch> --evidence-file <repo-path> [--brief-path <repo-path>]
```

**A Contents-API commit is not a local commit.** The PUT carries the branch, so the commit
lands on the **remote** branch as soon as the call returns: there is nothing left to stage
and nothing left to push, including on `main`. Three gates follow from that, all refusing
(exit 5, audited) **before any network call** — the first two added by #1282, whose tool
half reached this repo without them:

- **C-4 repo set.** `deskevidence` was the only outward-writing desk command with no
  `deskkit.IsAllowedRepo` check, so a typo'd or hostile `owner/repo` reached the App-token
  commit path unchecked. It now gates on the same compiled-in set as every other tool.
- **`main` / `master`.** Refused unless `VERIFIER_MAIN_OK` is exactly `1` — the same exact-`1`
  convention as the kill switch (`internal/deskkit/killswitch.go`), so `VERIFIER_MAIN_OK=0`
  reads as off and behaves as off. Unset, empty and any other value all fail closed. The
  comparison strips a leading `refs/heads/`, so `refs/heads/main` cannot walk past it. Any
  other branch needs no sanction.
- **`STATUS.md`.** Refused on **every** branch, for both `--evidence-file` and `--brief-path`.
  `STATUS.md` is generated and main's CI is its single writer. Since the gate above makes
  `main` a *sanctioned* channel and `VERIFIER_MAIN_OK` is set routinely in the verify-desk
  window, one coarse env var must not also open `main` to a generated file. The refusal keys
  on the basename, so `docs/…/status.md` and `STATUS-notes.md` still commit normally.

**The `flock`** (the third of #1282's guards, ported in #227).
`cmd/deskevidence/writeflow.go` now holds a `syscall.Flock(…, LOCK_EX|LOCK_NB)` over the
whole C-5 window — `AllowWrite → commitFile → audit append` — on the same
`~/.config/assay/audit.lock` as `deskpost` and `deskrelease`, so all three serialise
against each other over the one `audit.jsonl` they share. Before it, two concurrent
invocations each read a budget below the cap and both committed; measured, seeding
`RateLimitPerPRPerHour-1` charged entries and holding one invocation inside its PUT produced
**two commits and 11 charged writes against a 10 cap**.

Two details are load-bearing and easy to undo:

- **Defer order.** `lock.release()` is registered BEFORE the deferred audit append, so
  (Go runs defers LIFO) it runs after it. Swapping them puts the append outside the
  critical section, which is the exact window the lock exists to close. Guarded at source
  by `TestDeferOrderIsSourceGuarded` — the swap is not observable from a behavioural test
  (the gap is microseconds), so a source guard is the only thing that catches the refactor.
- **It is a lock, not a retry.** The second caller does not lose a race and try again; it
  waits, then re-reads the ledger the first caller actually wrote. A lock it cannot get
  inside 60s is Unverifiable (exit 6) — never "assume free" (C-10). A retry loop would
  only make the double-spend rarer. `TestContendedLockTimesOutUnverifiable` holds the lock
  from a second file description and pins both halves: it waits, and it then refuses.

Two limits, so the guarantee is not read as more than it is:

- **`flock` is per-host.** Two desk sessions on machines that do not share the filesystem
  do not serialise against each other. Each also writes its own `~/.config/assay/
  audit.jsonl`, so the rate-limit ledger they are protecting is per-host too — the lock is
  exactly as wide as the thing it guards, and no wider.
- **The holder is bounded by an HTTP timeout, not by the lock.** Every network call runs
  inside the critical section, so `httpClient` carries a 30s per-request `Timeout`
  (`cmd/deskevidence/github.go`). Without it a hung connection would pin the *shared*
  `audit.lock` and starve `deskpost` and `deskrelease` with it. `deskpost` and
  `deskrelease` still use `http.DefaultClient` and do not have this bound. The timeout is
  guarded at source by `TestHTTPClientIsBoundedNotDefault`, not behaviourally: every test
  replaces `httpClient` with one wired to the fake, so the production declaration is never
  exercised and reverting it to `http.DefaultClient` used to survive the suite green.
- **Failure mode: refuse, never proceed.** A lock that cannot be acquired — for any reason,
  including the 60s deadline — returns `Unverifiable` from `runOutward` before
  `cmdEvidence` is called, so nothing outward happens and exactly one audit line is
  written. There is no arm that degrades to "proceed unserialised", which would be worse
  than no lock because it would report success.
  `TestLockFailureStillWritesOneAuditLine` and `TestContendedLockTimesOutUnverifiable` both
  assert `putCalls == 0` alongside exit 6.

**Installation discovery, and the attribution post-condition** (#228). The
verifier App's installation is resolved at runtime from
`GET /repos/{owner}/{repo}/installation`, authenticated with the App's own JWT — so the
installation cannot belong to a different App by construction. It used to be a hardcoded
table returning `100000001` / `100000002`, which are the **reviewer** App's installations
(`gh api /orgs/medici-finance/installations`: `assay-reviewer-app id=100000001`,
`assay-verifier-app id=100000003`) and are still hardcoded verbatim in
`cmd/deskpost/github.go`. `VERIFIER_INSTALL_ID` still overrides; there is no silent
fallback, and `TestInstallForOwnerIsGone` fails if a corrected constant is ever seated
back into the package — a right constant is the same class of bug as a wrong one.

That source guard is necessary but not sufficient: it greps for the two reviewer literals,
so a fallback to any *other* ID is invisible to it. Two things cover the rest.

`TestLookupFailureFailsClosed` covers it behaviourally, across **all six** ways
`resolveInstallID` can fail: an unbuildable request, a transport error, 404, a non-2xx, an
unparseable body, and a body carrying no id or `0`. Each asserts **zero mint attempts**,
which is the load-bearing assertion. The exit code is not: a build that fell back to a
wrong ID also exits 6 (the mint is rejected), so only *which installation was tried* tells
fail-closed apart from fell-back-and-got-caught.

It covered two of the six until #406 R-1a, and the reason is worth stating: the
fixture could only script an HTTP *status*, so the four branches that never see a usable
response were unreachable from a test, and a fallback seated in any of them left the whole
package green. `fakeGH` now also carries `lookupHijack` (drop the connection) and
`lookupBody` (a raw 200 body); the sixth arrives through an unparseable `apiBaseURL`.

`TestResolveInstallIDHasNoSilentFallback` covers the branch that does not exist yet, which
no behavioural test can: it walks the AST and requires every return in `resolveInstallID`
to be either `return "", <non-nil error>` or an ID from one of exactly three sanctioned
sources (the `VERIFIER_INSTALL_ID` override, the process cache, GitHub's parsed answer).
Adding a fourth source is then a deliberate edit to a named allow-list, in a diff.

After the PUT, the commit GitHub actually created is checked: `commit.author.name` must be
`assay-verifier-app[bot]`. Three states, not two — matches → ok; a **different** name →
Unverifiable (exit 6), naming what landed, because a Contents-API commit is already on the
remote and a silent exit 0 is how Evidence signed by the wrong identity gets counted as
verify-desk output; **no** name in the response → warned on stderr and recorded in the
audit as `attribution=could-not-check`, exit 0, since an unreadable author is not evidence
of a wrong one.

Otherwise it inherits the usual plumbing: `Guard` first (C-6), `BodyCheck` over the content
that would be committed (C-3), idempotent noop when the remote already holds that exact
content, `AllowWrite` (C-5) charged only once the write is actually attempted, and one
audit line per invocation.

## deskgit — the narrow git verb (#1555 F-1)

`cmd/deskgit` gives the desk loops the one git verb they legitimately need unprompted —
refresh refs from origin — through a binary whose own argv parser refuses everything else,
replacing the allowlist rule `Bash(git fetch *)`.

`git fetch *` was **not** read-only (#1555, F-1): the wildcard has no end anchor, so
`git fetch --upload-pack="sh -c …" <local-path>` runs that program on the local "remote"
end — unprompted arbitrary code execution, proven by execution. A glob has no `main()`;
this tool does. Same structural argument as `deskrelease`.

```bash
deskgit fetch                 # refs/remotes/origin/*
deskgit fetch --prune         # + drop stale remote-tracking refs
deskgit fetch --pr <N>        # pull/<N>/head -> local branch pr<N>   (N digits only)
deskgit fetch --branch <B>    # origin's <B> -> local branch <B>      (see --branch guards)
```

`--branch` refuses `main`/`master` **in any case** (`Main`, `MASTER`, `mAiN`, …). Separately,
**both ref-writing modes** (`--branch` *and* `--pr`) refuse a destination that differs only by
CASE from an existing local branch — the check is keyed on the ref each mode actually writes
(`--pr N` writes `pr<N>`), not on the string the caller typed. Both matter because these modes
write into `.git/refs/heads/`, and the desk machine's filesystem (darwin/APFS) is
case-**insensitive**: `refs/heads/Main` and `refs/heads/main` are the same ref. A
case-sensitive guard was therefore a false guarantee — `--branch Main` rewrote local `main`
with an unrelated commit at exit 0, and the no-force refspec did not help because git
believed it was *creating* a ref, so no fast-forward check ran (security review blocker,
closed). The same shape was later reproduced on `--pr` — a differently-cased local `pr<N>`
replaced by an unrelated commit at exit 0 — which is why the check is keyed on the destination
ref rather than the flag. An **exact**-spelling destination on an existing branch is unaffected
and still subject to git's normal fast-forward rules.

Safe by construction — but note that **a fixed argv closes flags, not config or the
environment**. git honours the upload-pack program from `.git/config`
(`remote.origin.uploadpack`) and from injected env config, so an argv-only guard is still
RCE. deskgit closes the proven vectors:

- **Fixed argv from validated values.** Each mode builds a literal argv; the only
  caller-derived values are the digits-only `<N>` and the ref-ish `<B>` (no leading `-`,
  no `+`, no `:`), which flow only into a constructed refspec. Unknown flags and positional
  operands are refused, never ignored.
- **Transport-exec options refused BY NAME.** `--upload-pack`, `--exec`, `--exec-path`,
  `--receive-pack`, `--upload-archive`, `--config-env`/`-c`, and `-C` are refused
  explicitly, before the flag table sees the argv, in any position — including after a
  positional operand, where Go's `flag` has already stopped parsing. Matching is on the
  option name with prefix matching in both directions, because git honours unambiguous
  long-option abbreviations (`--upload-p`). See `transportexec.go`.
- **CLI pins beat config/env.** Every fetch carries `--upload-pack=git-upload-pack` (the
  CLI value overrides `remote.origin.uploadpack` from config *or* env — the code-execution
  vector), `--refmap=` plus an explicit `+refs/heads/*:refs/remotes/origin/*` (so a
  malicious `remote.origin.fetch` cannot redirect writes to local branches), and
  `--no-recurse-submodules` (so `fetch.recurseSubmodules` cannot expand one gated fetch
  into fetches of arbitrary `.gitmodules` URLs, escaping the repo gate).
- **Scrubbed child env.** `runGit` passes only an allowlist (`PATH`, `HOME`,
  `SSH_AUTH_SOCK`, locale, …) and drops every `GIT_*` var — `GIT_SSH_COMMAND`,
  `GIT_CONFIG_*`, `GIT_ASKPASS` — plus forces `GIT_TERMINAL_PROMPT=0`.
- **Effective-URL repo gate.** It gates on `git ls-remote --get-url origin`, which expands
  `url.<base>.insteadOf` (and makes no network call), so an insteadOf rewrite cannot present
  an allowed identity while fetching elsewhere. It rejects remote-helper (`<helper>::…`)
  transport forms and requires an exact `owner/repo` path for any **host-bearing** URL, so a
  padded URL can't smuggle an allowed slug in trailing components. The repo must be in the
  fixed C-4 set.

**Residuals on the gate — read before assuming it binds identity.** The exact-path rule
only bites on URLs the parser ROUTES to the host-bearing branch, and that routing is where
the bypasses were (security review S-1, both now closed and both covered by
`TestParseRepo_AuthorityParsing`): a `scheme://` URL whose *path* contained `@` had its real
host swallowed as userinfo, and a scp-like URL with **no** `user@` component fell through to
the lenient bare-path branch — while the same URL *with* a user was correctly refused. What
remains open:

- **Host is not bound** to `github.com` — the desk's origins use ssh host aliases
  (`git@github-example:owner/repo`), so an exact allowed `owner/repo` on an unexpected host
  still passes and does a read-only fetch; host-allowlist binding is a follow-up.
- **Bare local paths are gated by the local-roots allowlist** (#215, formerly a
  residual). A real local repo lives at an arbitrary absolute path, so no `owner/repo` shape
  applies — but the parser no longer reads its identity off the last two path components
  either. `deskkit.RepoForLocalPath` resolves the path to its **canonical absolute form**
  (symlinks included) and admits it only when it **is** a configured local root
  (`DESK_ROOTS` / topology) — by equality, not descendant containment, so a bare repo planted
  at `<root>/vendor/x.git` cannot inherit the root's identity — taking the repo from the
  **root**, not the path. A root that resolves to the **process working directory** (the
  compiled `.` home root when `DESK_ROOTS` is unset) is skipped: a root meaning "wherever I
  run" cannot anchor identity. `file://` URLs route through the **same** gate, since they too
  name a purely local path. So an `insteadOf` rewrite to a directory merely *named* to spell
  an allowed slug — the old bypass, which landed foreign content on the tracking refs the
  desk loops consume — is now refused with no lenient fallback.
- **The audit line is not a second gate**, but it is no longer blind. C-5 remains a ledger of
  what the gate *allowed* — it refuses nothing. It now records the **effective origin URL**
  alongside the parsed slug on the success path (security review Q6), with any userinfo
  redacted so the ledger never becomes a credential store. Before that change a legitimate
  refresh, a foreign-host fetch and the (then-open) local-path smuggle produced byte-identical
  lines in every security-relevant field; recording the effective URL made the smuggle
  **detectable** even before the local-path gate above closed it outright.

It is **not a sandbox**, and the boundary is wider than "a compromised repo" (security review
S-2). `cmdFetch` binds to `os.Getwd()` and does not check the worktree against deskkit's
known roots, so the **caller chooses the repo** and with it the `.git/config` governing the
fetch. In the #1555 threat model the caller *is* the adversary, so an attacker-controlled
`.git/config` is the ordinary reachable state, not an edge case. Under it, **any config key
that names a program** is an execution route — the class, not just the examples:
`core.sshCommand`, `core.gitProxy` (its env twin `GIT_PROXY_COMMAND` *is* scrubbed, which
makes it easy to misread as closed), `core.fsmonitor`, and `remote.<n>.vcs` (git runs
`git-remote-<name>` while `ls-remote --get-url` still reports an innocent URL, so the gate is
structurally blind to it).

deskgit also **trusts `PATH`** (security review S-3): `PATH` is allowlisted and `runGit`
invokes `git` by bare name, so the binary that runs is whatever `PATH` resolves — and the
`--upload-pack=git-upload-pack` pin binds that program's *name*, not its path. The "a glob
has no end anchor, but a `main()` does" argument holds only while `PATH` is trusted.

`git` is **not the only lever** (security review Q3). Anything git shells out to that is not
a `git-*` core helper resolves from the inherited `PATH` — confirmed for **`ssh`**, which is
executed from `PATH` for both `ssh://` and scp-like origins. (A stand-in `git-upload-pack` is
*not* reachable: git prepends its own `--exec-path` for core helpers, and `--exec-path` is
refused by name while `GIT_EXEC_PATH` is scrubbed — that surface is closed on both sides.)
So resolving `git` to an absolute path is **necessary but not sufficient**: the child must
also stop inheriting `PATH`. The follow-up is therefore *both* halves — absolute `git` **and**
a fixed system `PATH` in the allowlist's place; the first half alone leaves the `ssh` lever
open. It is deferred because a minimal `PATH` risks breaking ssh/credential helpers on the
desk machine.

deskgit closes the proven upload-pack, env, fetch-refspec and submodule vectors, and the
insteadOf **identity-substitution** vector for **both** host-bearing URLs and bare local
paths (the latter via the local-roots allowlist, #215). It claims no more.

It is a **local-read verb**: network read-only, no outward write, no credentials, so — like
`deskwt` — it takes the audit line (C-5) and kill switch (C-6) but NOT the rate limit.
Exit: `0` ok · `3` disabled · `5` refused · `6` unverifiable.

**The allowlist half lives in the consuming repo**, not here — this repo ships the binary.
A repo adopting `deskgit` removes `Bash(git fetch *)` from its own `.claude/settings.json`
and allows the **fully anchored** `/opt/desk-tools/bin/deskgit fetch` / `… --prune` (no
trailing `*`, so nothing is appendable). The `--pr`/`--branch` modes carry a value that
would need a trailing `*`, so they are deliberately NOT allowlisted and prompt. The `go run`
form must be excluded: it would compile and run agent-writable source, which is the same
unprompted-code-execution class this tool exists to remove. Until `sudo make desk-install`,
`deskgit fetch` prompts (fail-safe). For the first such adoption, see #1556.

## writeguard — the F-34 isolation backstop (PreToolUse hook)

`cmd/writeguard` is the Claude Code PreToolUse hook (registered in `.claude/settings.json`)
that mechanically refuses writes to the SHARED checkout: Edit/Write/MultiEdit/NotebookEdit
targets inside it, and Bash commands that write into it or cd into it. Full decision model:
the `guard.go` package doc comment. It fails OPEN (any parse/git error allows) so a broken
guard can never brick every session.

**Deployment (#20).** This repo's own `.claude/settings.json` now wires the
installed `/opt/desk-tools/bin/writeguard` binary into `hooks.PreToolUse` for both the
file-edit tools and `Bash` — mirroring the wiring already live in the sibling
a sibling adopter repo. Before #20, the guard was built and
unit-tested here (`go test ./cmd/writeguard/...`) but never actually armed for sessions
dispatched against this repo's own shared checkout: prompt discipline (worker-prompt
essentials in the batch-fanout skill, §Guardrails' `git -C <shared> status --porcelain`
dirt-poll) was the only backstop for THIS repo, which is exactly the F-34/F-35 class this
hook exists to mechanically close. Any repo that adopts `tools/desk` and wants the same
backstop copies the same `hooks.PreToolUse` block into its own `.claude/settings.json`.

**The shared-homed exemption is OPT-IN (#1035).** A session homed in the shared checkout
(project dir AND payload cwd both resolve there, per #1007) is exempt ONLY when the
exemption is explicitly CLAIMED, in one of two ways:

```bash
export WRITEGUARD_SHARED_OK=1                       # direct human shell, once per shell
# or, in a HUMAN TERMINAL (outside Claude Code — see below):
mkdir -p ~/.config/assay
printf '%s\n' /path/to/shared-checkout > ~/.config/assay/writeguard-shared-ok
```

This mirrors the `.githooks/pre-commit` `ASSAY_MAIN_COMMIT_OK` main-commit gate. Without a
claim, a shared-homed session's writes to the shared checkout are BLOCKED with a message
naming both mechanisms and the isolation alternative — because sanctioned boot
instructions ("cd into the checkout so the skill resolves") create dispatched sessions
that are shared-homed by construction (the #1035 billy incident). The claim never widens
anything else: a worktree-homed session that inherits it stays fully guarded.
Dispatched/worker sessions must NOT claim it — isolate instead
(`git worktree add ../tracker-<name> -b <branch> origin/main`, absolute sibling path).

**Why the sentinel file exists (#1190).** The env var alone made the sanctioned exemption
UNREACHABLE from the Claude Code Bash tool: every Bash call runs a fresh shell, and the
hook reads its own process env, so an inline `WRITEGUARD_SHARED_OK=1 cmd` prefix can never
be seen by the guard. The file survives the per-call shell — and, so it cannot quietly
become a permanent machine-wide exemption, it **expires** (12h from mtime by default,
`WRITEGUARD_SHARED_OK_TTL`; renew by touching it again, drop it with `rm`) and can be
**scoped** by writing shared-checkout paths into it, one per line (an empty file claims any
checkout). `WRITEGUARD_SHARED_OK_FILE` overrides the path; `XDG_CONFIG_HOME` is honored.

**The claim is HUMAN-ONLY, mechanically (#1259 review).** The guard REFUSES every
tool-surface write to the sentinel path — Bash and Edit/Write/MultiEdit/NotebookEdit
alike, from every session, before the exemption gate, so an exempt session cannot renew
its own claim either. Make the claim (or renew or drop it) in your own terminal, outside
Claude Code, where no hook runs.

The reason is that a file, unlike the env var, is settable by exactly the population the
gate exists to exclude: a dispatched worker can `touch` it as easily as a human can, and
dispatched workers are shared-homed by construction (#1035) — one Bash call and every
shared-homed session on the machine was exempt for 12h, with the block message supplying
the recipe. `WRITEGUARD_SHARED_OK` was un-settable from a session not by policy but by
construction (the hook reads its own process env); refusing writes to the sentinel path
restores that property for the file. What distinguishes a human act from an agent act
here is not intent, which the guard cannot read, but the tool surface: agents write
through hooks, humans at a terminal do not.

This is a hook-level control, not a sandbox. Like every other rule in the guard it keys on
the command string, so an agent that writes a helper script and runs it, or reaches the
filesystem from an interpreter (`python3 -c`, `perl -i`), is not stopped — the same limit
the whole guard has. It removes the one-token reflex path, which is the failure mode that
was observed.

**A write indicator means a WRITE (#1190).** The guard blocks on what a command WRITES,
never on a keyword appearing in it:

- Indicator regexes run against the command with **literal text masked** — quoted spans
  and heredoc bodies. `awk 'NF>1 {print $1}'` is not a redirect, a `--body '> quoted'` is
  a markdown blockquote, and an issue body naming `statusgen` runs nothing. Text that is
  itself a command stays visible: the argument of `sh -c` / `eval`, a heredoc fed to a
  shell, and a quoted command NAME (`"rm" -rf x`), so `sh -c 'rm <shared>/x'` still blocks.
  A `<<WORD` inside a comment or a quoted string does not introduce a heredoc.
- The tool indicators require an **invocation**: `<worktree>/tools/statusgen/registers.go`
  mentions the name mid-path, `grep -rn statusgen x` runs grep. Wrappers keep the tool in
  command position (`timeout 60 statusgen`, `xargs statusgen`, `sh -c 'statusgen'`).
  Read-only board modes (`--lint`, `--check`, `--dry-run`) write nothing and never block —
  but only within the invocation's own segment, so a `--lint` on a later line cannot
  disarm an earlier real run.
- Git's mutating subcommands must END AT A SHELL BOUNDARY: `git merge-base` is not
  `git merge`. Read-only modes of mutating subcommands (`stash list`, `clean -n`,
  `apply --check`, any `--help`) write nothing.
- Relative targets resolve against the **effective cwd** — the payload cwd, advanced ONLY
  along a leading `&&`-chain of literal `cd`s (`cd /abs/worktree && …`, the shape a
  dispatched worker whose Bash cwd resets to the payload cwd actually writes). `&&` is
  what makes that sound: if the cd fails, nothing after it runs. Every other cwd-changing
  construct — a `;`- or newline-separated `cd`, a subshell or `sh -c` cd, `pushd`/`popd`,
  a target the guard cannot resolve — leaves the frame at the payload cwd, fail-safe.
  (A `cd` INTO the shared checkout is blocked outright, so the frame only moves out.)
- That `cd`-into-shared detection reads the **literal-masked** command too, on the same
  rule as the indicators: a `cd` quoted in prose or a heredoc body is not a `cd`, so
  `grep -n 'cd ..' f.go` and `gh issue create --body "run: cd docs && ls"` write nothing
  and pass, while `sh -c 'cd <shared> && …'` and `bash <<EOF … cd <shared> …` — text that
  IS a command — stay visible and still block.
- A **`$( … )` substitution inside double quotes stays visible**, on that same "text that IS
  a command" rule: the shell executes it before the surrounding command runs, so
  `echo "$(cd <shared> && rm -rf docs)"` really does delete the directory. Its own quoting
  cannot end the span early (a `)` inside `'…'` is a literal), nesting is counted, and `\$(`
  is prose. Single quotes need no equivalent — no substitution happens inside them.
  **Deliberately not mirrored in the cwd-frame masking**: `$( … )` runs in a subshell, so a
  `cd` inside it must stay hidden or `echo "$(cd /tmp)" && rm -rf docs` would move the frame
  and fail open.
- The **heredoc delimiter's quoting** decides the same thing for a heredoc body. An
  UNQUOTED word (`<<EOF`) makes the shell expand the body before the introducing command
  ever runs, so `cat <<EOF … $(rm -rf <shared>/docs) … EOF` genuinely deletes — `cat` never
  sees it, and `gh pr comment --body-file - <<EOF` is a daily desk shape. `$( … )` spans in
  such a body therefore stay visible, with the same depth counting, inner-quote handling and
  `\$(` escape as above. A QUOTED word (`<<'EOF'` / `<<"EOF"`) suppresses expansion
  entirely, so its body is literal text and is masked as prose. Blank/keep are computed for
  ALL heredocs before any byte is written, so two heredocs on one line — which share a body
  offset — cannot un-preserve each other's executed code in either delimiter order.
- **Expansion means all expansion**: `` ` … ` `` is kept visible wherever `$( … )` is, i.e.
  in an unquoted heredoc body and inside double quotes. The backtick is the markdown
  code-span character, so `gh pr comment --body-file - <<EOF` carrying one is ordinary desk
  text that bash runs. A quoted delimiter and single quotes suppress it, and `` \` `` is
  prose, so markdown recipes written the normal way still pass.
- **The masker fails CLOSED, because the write detector reads its output.** Indicator
  scanning runs over `maskLiterals`, so anything wrongly blanked is an admit. Spans open and
  close on the shell's rules — an escaped `\'` opens nothing, `\"` inside double quotes
  closes nothing, and a quote inside a `#` comment is not a quote — and an UNTERMINATED span
  is restored verbatim rather than blanked. A comment's text is still masked (it executes
  nothing); only its effect on quote state is removed. Rule of thumb: masking may blank only
  what it positively understands.
- A path whose head is a shell expansion is **resolved where the command says what it is**:
  `$HOME`, `$(pwd)`/`` `pwd` ``, a literal `NAME=value` bound earlier in the same command
  (`D=<shared>; rm -rf $D/dist` blocks), or an exported variable in the hook's environment.
  Shell-dynamic variables (`$OLDPWD`, `$RANDOM`, …) are never read from that environment.
  A head that survives all of it is unknown and is judged against the frame, exactly as
  before — so it blocks from a frame inside the shared checkout and passes from outside.
  A literal shared prefix is always evidence: `<shared>/$SUB` blocks.
- **Copy-like commands** (`cp`, `rsync`, `install`, `ln`) write only their destination;
  their sources are reads — unless a flag makes the source a write too
  (`rsync --remove-source-files`). `mv`/`rm` mutate every argument, so those are unchanged.

**Known-uncovered** (true before this PR and unchanged by it; not regressions): escaped or
split command names (`\rm`, `r"m"`, `'git' clean`), interpreters (`python3 -c`,
`perl -i -pe`), `find . -delete`, `rsync --backup-dir`, and the git mutations not in the
alternation (`revert`, `pull`, `branch -f`, `worktree add`, `config`).

Backtick substitution was listed here as uncovered-and-unchanged. **That was wrong and is
measured wrong** (#1259 security review S-3): this PR's masking is what hid it, so main
refused shapes an intermediate head admitted, verified by canary. Backticks are now handled
wherever the shell expands them, and `#1330` covers only what remains — a backtick payload
in a context nothing expands.

Also uncovered, and a **deliberate trade** made by masking the `cd` scan: writing a script
whose body contains a `cd` into the checkout and running it in the SAME call —
`cat > f <<'EOF' … cd <shared> … EOF` then `bash f`. The heredoc introducer is `cat`, not a
shell, so the body is prose to the guard. Splitting the write and the run across two tool
calls defeats every version of this guard including the pre-PR one, so the masking is not
what makes this reachable; it is the price of not blocking every issue body that quotes a
shell recipe.

**Known-over-covered** (a write the guard refuses even though it lands outside the
checkout): a target whose head is a command substitution the guard cannot evaluate —
`T=$(mktemp -d); echo x > $T/f` — is judged against the frame, so it blocks from a cwd
inside the shared checkout. `$(mktemp -d)` is a common agent idiom, so expect to meet
this; it is the fail-safe working as designed, not a regression (main behaves the same),
and the fix is to name the destination literally or `cd` there along an `&&`-chain first.

The shape you are most likely to meet first is a **`;`-separated `cd`**:
`cd /private/tmp && … ; cd /private/tmp/wg && git fetch … && git checkout …`. The `;` makes
the second `cd` non-honorable, so the frame stays at the payload cwd — and if that cwd is
the shared checkout, the `git fetch`/`git checkout` read as targeting it and block. Correct
and unchanged by this PR: only an `&&`-chain guarantees the `cd` actually succeeded. Chain
with `&&` throughout, or pass `git -C <dir>`.

## muhar — the mutation harness that proves each mutation landed (#34)

`cmd/muhar` is a mutation-testing harness with a **proven-landed guarantee**. Mutation
testing — break the thing a check guards, confirm the check screams — is the load-bearing
habit of the review system, but a harness that silently fails to mutate reports "the gate
does not fire" for a gate it never actually broke, and a **false NOT-CAUGHT is
indistinguishable from a real one** (#34: a reviewer's ad-hoc `sed -i ''`
no-op'd under GNU sed, ran the suite against pristine source, and reported every guard in
the PR as untested). `muhar` makes that impossible by construction:

1. **A mutation is not applied until it is proven applied.** `applyEdit` hard-fails on a
   no-op edit (target text absent, `old == new`, or content unchanged), and `runOne`
   **re-reads the file from disk after writing** to confirm the intended bytes landed. No
   proof, no result.
2. **Three states, never two:** `CAUGHT` (checked-failed) / `NOT_CAUGHT` (checked-clean) /
   `COULD_NOT_MUTATE` (could-not-check). A failed edit is its own state — it never
   collapses into `NOT_CAUGHT`.
3. **A mandatory positive control** — a mutation the suite MUST catch (a structural
   invariant). If the control is not caught, the harness is broken and the **entire run is
   discarded**, not reported as a clean gate. A harness with no failing control is a green
   light with no bulb.
4. **A baseline run on pristine source must be green**, or the run is discarded (a red
   baseline makes every "caught" meaningless).

```bash
muhar -spec mutations.json   # exit 0 = healthy run (see stdout); exit 2 = HARNESS BROKEN, discard
```

The spec is JSON: `{root, test, control, mutations[]}` where `test` is the suite command
(non-zero exit == mutation caught) and `control` is the always-caught mutation. A broken
run prints only `HARNESS BROKEN` and its reason — never per-guard verdicts, because those
are the very thing that cannot be trusted.

`muhar` does **not** call `deskkit.Guard()`: it is a local diagnostic that makes no
outward writes (no GitHub, no shared-state mutation — it edits a source file and restores
it in the same run), so the C-6 kill switch and outward-write rate limit do not apply, the
same documented exemption class as `writeguard`. Halting local mutation testing serves no
safety purpose.


## desksourceguard — the source pin is a commit, not a tag (#519)

A consumer of desk-tools pins two things. The **binaries** are tag + sha256, and that half
has always been safe: a digest committed in `.assay-versions` is something a re-tagged
release cannot silently substitute. The **source** — the `tools/desk` tree a consumer
materialises so it can run the `//go:build consumer` structural tests — was pinned by **tag
alone**. A release tag is a mutable ref, so the tree a consumer's own `gate-pins` job
compiled could be changed *after* the pin was reviewed, by repointing the tag, with the
consumer's diff unchanged and nothing to show for it. Reproduced with a real git origin: the
same consumer tree, the same `.assay-versions`, two clones, two different compiled trees.

`cmd/desksourceguard` closes it by moving the anchor from the tag to the **commit**, which is
content-addressed and cannot be re-resolved:

```bash
desksourceguard --source "$RUNNER_TEMP/assay-src" --repo-root .
```

Three agreements, all fail-closed (`5` refused / `6` unverifiable — nothing collapses to 0):

1. **One release.** `desk-tools-source` and `desk-tools-<platform>` name the same tag, so the
   two halves of the pin cannot drift into naming different releases.
2. **The compiled tree is the pinned commit.** `git rev-parse HEAD` of the materialised
   checkout equals the pinned 40-hex SHA. A repointed tag goes RED here; the refusal names
   both commits so an operator can tell a repoint from a stale pin.
3. **The pinned commit is the binaries' commit.** The pin must have this binary's `SourceSHA`
   stamp as a prefix. That stamp arrived inside a tarball whose sha256 the consumer already
   verified against `.assay-versions`, so it is **hash-anchored** — it is what ties the source
   pin to the binary pin. Without it a consumer could pin a source commit unrelated to the
   binaries it runs and (1) and (2) would both still pass.

Two ordering choices are load-bearing. The guard is a **binary**, not a script in the cloned
tree: it runs before a line of that tree is compiled, and a check living in the tree could be
edited by the same repoint it exists to catch. And the consumer **clones the tag and then
verifies**, rather than fetching the pinned SHA directly — fetching by SHA would quietly use
the right tree and never report that a release tag had moved.

An unstamped build (`go run`, or `go build` without the release `-ldflags`) is refused
outright rather than warned about: agreement (3) is the whole anchor, and an unstamped binary
has nothing to anchor to.

`cmd/desksourceguard/mutations.json` is the `muhar` spec for its refusals — eight mutations,
each deleting or inverting exactly one refusal, all of which must redden the suite. It runs
in CI (`.github/workflows/tools.yml`), not only on request: a guard whose refusals can be
deleted with the suite still green is not a guard.

`desksourceguard` does **not** call `deskkit.Guard()`: it runs in GitHub Actions, not in a
desk session, and a desk kill switch must not decide whether a supply-chain check runs.

### Consumer wiring

The guard ships in the tarball, so a consumer gets it with the next pin bump — and by the
pin-file rule "a pin bump travels in the same commit as whatever depends on it"
(`docs/distribution.md` rule 5), that bump is where the wiring belongs. Two edits.

`.assay-versions` gains the source pin, harvested from the tag being pinned:

```bash
git ls-remote origin "refs/tags/desk-tools/vX.Y.Z^{}"   # the COMMIT, not the tag object
```

```
desk-tools-source       desk-tools/vX.Y.Z <40-hex-commit>  # the commit the binaries were built from
```

and the source-materialising step verifies before it hands the tree over — after the clone,
before anything compiles:

```bash
git clone --depth 1 --branch "$tag" --filter=blob:none --sparse "$repo_url" "$src"
git -C "$src" sparse-checkout set tools/desk
desksourceguard --source "$src" --repo-root .     # exits 5/6 rather than compile an unreviewed tree
mv "$src/tools/desk" "$GITHUB_WORKSPACE/tools/desk"
```


## deskrelease — the guarded release-tag cutter (#1538)

`cmd/deskrelease` creates a release tag — `desk-tools/vX.Y.Z` or `statusgen/vX.Y.Z` — as
the **desk App**, via the GitHub **git-data API**. It exists so that cutting a release
does not require granting an agent the `git push` verb.

**Why a binary rather than an allow-list rule.** Claude Code's Bash wildcard "matches any
sequence of characters **including spaces**", so one `*` spans arguments and a glob has no
end-of-command anchor. `Bash(git push origin desk-tools/v*)` therefore also matched
`git push origin desk-tools/v0.1.1 +HEAD:refs/heads/main`. Four escapes were confirmed
against a throwaway bare repo (#1538): `^{}:refs/heads/main` moved a branch, a bare
` +main` force-moved one with no `-f`, bundled `-uf` forced past all four `--force` deny
rules, and ` :refs/heads/<branch>` deleted a non-default branch. Nothing downstream caught
them — branch protection is unavailable on this plan (HTTP 403), F-13 is a pre-**commit**
hook so a refspec write to `main` never reaches it, and `deskpushguard` neither inspects
destination refs nor fails closed. The allow-list was the only layer.

Deny-rule patches were evaluated and rejected as the durable answer: the first candidate
set blocked 8 of 23 dangerous shapes, was quote-defeatable (`<tag> "+main"` evades
`* +*`), and a rule ending `:*` parses as a legacy **prefix** rule, so the obvious
`Bash(git push *:*)` is inert. See #1555 — 27 of 29 allow rules end in unanchored
wildcards; this is one instance of a general gap.

`Bash(/opt/desk-tools/bin/deskrelease cut *)` has the same trailing-wildcard shape, but
the **last** layer that wildcard reaches is **this binary's argv parser**, which refuses
everything it does not recognise. Two precisions, since the whole argument here is that
absolute-sounding claims about matchers are what produced #1538:

- The wildcard does **not** reach only this parser — the **shell** sees the command text
  first, and this binary cannot improve that layer. What it supplies is the **end anchor**
  the glob was missing: whatever argv survives the shell arrives somewhere that refuses
  anything unrecognised, rather than at `git push`, which does not.
- The rule must name the **absolute path**. `/opt/desk-tools/bin` is **not on `PATH`** —
  every desk tool is invoked as `/opt/desk-tools/bin/<tool> …` — so `Bash(deskrelease cut *)`
  could never match the command text and would be an **inert rule**, the same dead-rule
  defect described above for `git -C … push` against `^git push origin …`. Putting the bare
  name on `PATH` to make it match is worse: the grant would then authorise whatever
  `deskrelease` resolves to, including a `tools/desk/dist/deskrelease` built from any
  branch — the same argument `deskTokenPath` already makes one level down.

**What it does, and nothing else (C-7):**

| | |
|---|---|
| verbs | `cut <tag>` only. No delete, force, move, override, or waiver verb exists. |
| tag validation | two independent layers: a refspec-metacharacter screen (`:` `+` `^` `~` `@` `*` `?` `[` `\`, whitespace, control chars) and the anchored pattern `^(desk-tools\|statusgen)/v[0-9]+\.[0-9]+\.[0-9]+$`. Each refuses with its own reason. |
| argv | **exactly** `<tag>` (+ optional `--dry-run`). Any extra operand or unknown flag is REFUSED (exit 5, audited), never ignored. |
| input | argv only — never stdin, never an environment variable. |
| `--dry-run` | validates, reads the remote, and stops **before** `POST /git/refs`. **When the tag is still free** — the case the flag is for — exit 0, audited `result=dryrun` — a class **neither rate-limit meter counts** (#214). It used to audit `noop`, which the non-progress breaker counts, so five rehearsals opened a 15-minute breaker against the real cut: checking whether a release was safe could lock the release out. A dry run is still *subject* to both meters; it just does not feed them, and it is IGNORED rather than treated as progress, so it cannot reset a breaker a real refusal loop opened either. Against a tag that already **exists**, the stop is never reached: the row below settles the invocation first, and the rehearsal audits `noop`/`refused`/`unverifiable` like the real thing. |
| transport | `POST /repos/{owner}/{repo}/git/refs`. `git push` is never executed; no local checkout is touched, so a `git -C` variance and a stale sibling clone are both impossible. |
| release point | re-read at act time from `GET …/git/ref/heads/main`. There is no `--sha` — a caller cannot choose the commit that gets released. |
| existing tag | Never moved or re-pointed. Consumers pin tag+sha256, so a moved tag is the substitution the pin prevents. `POST /git/refs` is itself create-only (422), so this holds at the transport too. An existing **lightweight** tag already at the current `main` HEAD is a NOOP (exit 0, nothing written) so a transient error cannot burn a version; at any other commit — or when `main` cannot be resolved — it is REFUSED. That discrimination needs the ref to point at a **commit**: an **annotated** tag (every tag the `workflow_dispatch` path cuts) is UNVERIFIABLE (exit 6, nothing written) before either outcome is reached — see "One interaction to know about" below. Fix forward with the next patch version. |
| tag object | **LIGHTWEIGHT** — a ref, not a tag object. Consequences below. |
| repo | compiled in (`medici-finance/assay`); no `--repo` flag exists. |
| identity | `desktoken desk --repo …` — releases carry a named App, not a session's ambient credential. |

It leaves `release-desk.yml`, `release-statusgen.yml` and `releaseguard` untouched: the
tag ref still fires `on: push: tags:`, and the unwaivable `on-main` / `tag-format` checks
still gate the build (empirically confirmed — an API-created tag produced a real release
with 4 assets).

### The tag is lightweight — and that closes the waiver channel

`POST /git/refs` creates a **ref**, not a tag object: `desk-tools/v0.1.2`, cut this way,
reports `object.type = commit` where an annotated tag would report `tag`. There is
therefore **no tag message**, and nobody composes one.

That is deliberate and it is the strongest property here. `releaseguard` reads
`Release-Override:` trailers only via `git cat-file tag`, and only when the tag is
annotated (`tools/releaseguard/main.go`, `guard.go`) — precisely so a lightweight tag's
*commit* message can never reach `parseOverrides`. So a caller of `deskrelease` **cannot
author a waiver by any route**. The override channel is not relocated into this binary;
it is structurally absent from it. "Whoever composes the tag message can self-author
waivers" stops being a question you have to answer.

**What it costs, stated plainly.** `releaseguard`'s three waivable checks —
`behind-main`, `version-order`, `tree-unchanged` — become **unwaivable in practice for
anything cut with this tool**. That contradicts `cut-release/SKILL.md`'s standing
instruction ("Annotated (`-a`), always"), and a releaser needs to know it *before* the
guard refuses, not after. `on-main` and `tag-format` were never waivable and are
unaffected.

**When a waiver is genuinely warranted** — the two cases `releaseguard` was built for are
a hotfix off an older line (`behind-main` + `version-order`, e.g. patching `0.3.x` when
`0.5.0` is already out) and a re-cut after a bad upload (`tree-unchanged`). Neither is
routine, and neither can be done with `deskrelease`. The answer is **not** to add an
override flag to this tool — that would rebuild the channel it just closed. It is that a
waived release is a **human action**: Ada cuts an annotated tag with the `Release-Override:`
trailers by hand, under the same review the trailer text is meant to receive. The tool
covers the routine path (tag current `main`, no waiver) and refuses to be the vehicle for
the exceptional one.

**The `workflow_dispatch` release path (2026-08-02, `4dfcac9`) closes the channel the
same way.** All three release workflows now take a `version` input and cut the tag inside
the publishing run. That tag *is* annotated (`git tag -a`), but the annotation is composed
by the workflow — `<component> <version> (dispatched by <actor>)` — not supplied by the
dispatcher, so it can carry no `Release-Override:` trailer either. A hand-cut annotated
tag pushed the original way remains the only route to a waiver, which is the intended
shape: the exceptional path stays human.

**One interaction to know about.** Because dispatched tags are real tag objects,
`getRef` (`cmd/deskrelease/github.go:167`) sees `object.type == "tag"` and returns
**unverifiable** rather than a commit sha — and it does so at `cut.go:184`, *before* the
branch that compares where the tag points. So `deskrelease cut <tag>` aimed at a version
already cut through the dispatch path always exits **6 (unverifiable)**, where the same
name as a **lightweight** tag would take one of the two outcomes in the "existing tag" row
above: **0 (noop)** when it already points at the current `main` HEAD, **5 (refused)**
anywhere else. Nothing is written on any of the three, so "never move a published tag" is
untouched — the tool simply can no longer tell the two cases apart. Three things follow,
and only the first is cosmetic:

- **The message is less specific.** "points at a `"tag"` object, not a commit" names the
  ref shape, not the release fact.
- **The exit is non-zero where it used to be 0.** Re-running a version already cut is no
  longer a quiet noop, so anything treating `deskrelease cut` as idempotent must expect an
  exit 6 for a dispatch-cut tag.
- **It charges the outward-write budget.** `unverifiable` is one of the two classes that
  do (top-of-file table): the call is assumed to have possibly reached the remote. `noop`
  and `refused` charge nothing. Repeated re-cut attempts against a dispatched tag therefore
  spend a real release budget for a write that could never have happened.

**The NOOP branch is unreachable for dispatch-cut tags — but the case it was written for
is not.** `cut.go:169-176` keeps the noop so a transient error cannot burn a version: if
`POST /git/refs` errors *after* GitHub created the ref, the retry sees the tag already at
main's HEAD and exits 0 instead of refusing. That protection is intact, because a ref this
tool creates is always **lightweight** — the retry still reads `object.type == "commit"`.
What is gone is the same courtesy for tags `deskrelease` did **not** create: a
dispatch-cut (annotated) tag re-offered to `cut` is exit 6, whether or not it sits exactly
where the cut would have put it. Peeling `^{commit}` in `getRef` would restore it; that is
a code change, deliberately not made here.

**`--dry-run` changes none of the above.** The dry-run stop is at `cut.go:233`, *below* the
existing-tag branch, so a rehearsal against a tag that already exists returns that branch's
outcome (0 / 5 / 6, audited `noop` / `refused` / `unverifiable`) and never the meter-exempt
`dryrun` class. The flag only reaches `dryNoop` when the tag is still free — which is the
case it is for.

**Release notes.** The tool writes none, intentionally. The release body is produced by
`release-desk.yml` / `release-statusgen.yml` from the tag event; the assets and
`checksums.txt` are the deliverable that consumers pin. An empty body is the status quo
for tags cut this way, not a regression introduced here — and a notes string would be one
more caller-composed field on a path whose whole point is that no caller composes text.

```bash
/opt/desk-tools/bin/deskrelease cut desk-tools/v0.1.4 --dry-run   # validate + report, write nothing
/opt/desk-tools/bin/deskrelease cut desk-tools/v0.1.4
```

Allow-list rule this makes safe: `"Bash(/opt/desk-tools/bin/deskrelease cut *)"` — the
**absolute** path, for the two reasons given above. A rule naming the bare `deskrelease`
would be inert, because that is not how the tool is invoked.

**Tests.** 86 subtests; every refusal path is proven to refuse AND proven to have written
nothing. The suite is mutation-checked with `muhar` — 19 mutations plus a positive
control, all CAUGHT, 0 NOT-CAUGHT, 0 could-not-mutate.

## deskclaim — the flock-backed claimable-action lock (#287)

`cmd/deskclaim` is the CLI over the canonical claim primitive (`deskkit/claim.go`). It is a
mutual-exclusion **LOCK**: it decides who owns a handoff so two racers cannot both dispatch,
route, file, close, or verify the same item.

```bash
deskclaim acquire --kind dispatch --item stream-a/01 [--branch B] [--owner O]
deskclaim release --item stream-a/01
deskclaim list
```

`acquire` exits **0** when the claim is acquired (a fresh create, or a stale reclaim), **5**
when a live holder already owns the item (do NOT proceed), and **6** when the lock could not
be held or the claim could not be read/written. That exit-6 is the load-bearing contract:
**no path may degrade to "proceed unclaimed / assume free."** A lock this process cannot hold
is `Unverifiable`, never a silent success.

**Why a binary and not a shell line (the #146 close).** The atomic-claim idiom used to live
in a shell `(set -C; … > "$f")`. The `writeguard` hook blocks the `>` redirect, so an agent
falls back to the Write tool — a blind overwrite with **no `O_EXCL`** — and every racer
"succeeds": double-dispatch. `deskclaim`/`deskkit.Acquire` do the create-or-reclaim under a
directory-wide `flock`, so atomicity lives where the hook cannot force it away.

**Schema unification (#278 item 2).** Three disjoint on-disk claim shapes used
to coexist — loopengine's `{itemID,runner,branch,claimed}`, deskroster's `{brief,session,ts}`,
and the bash idiom's `{brief,repo,session,ts}`. The canonical WRITE shape is now
`{kind,item,owner,branch,ts}`, and the READ is tolerant: `deskkit.List` maps every legacy
field name onto the canonical fields, so `deskclaim list` and `deskroster list` surface every
claim regardless of which writer wrote it. `loopengine.Claim/ReleaseClaim` are thin delegates
over `deskkit.Acquire/Release` (Kind=`dispatch`), so the verifyloop/worker-desk dispatch
paths keep their exact behaviour on the shared primitive.

## `deskroster preflight` — the operating-envelope check

Roughly **one in five** of the verifier desk's open issues is not a finding about the work.
It is the desk discovering **its own operating envelope** mid-pass, three quarters of the way
through, and spending the rest of the pass writing an issue about itself:

| discovered mid-pass | issue |
|---|---|
| the App pem is in a directory the minter never searches — a fresh shell cannot mint once the ~1h token cache lapses | #794 |
| the installation lacks the scope the role's duty needs (PR comment), found at comment time | #571 |
| no permitted outward write transport at **landing** time — two briefs verified to a clean PASS, neither landable | #823 |
| weeks of commits under an email whose numeric prefix is the **App id** instead of the **bot USER id**, so nothing is account-linked | #638 |
| a sibling checkout a queued brief's rows need is simply not there | #679 |

```bash
deskroster preflight --role verifier [--root DIR] [--remote NAME] [--branch NAME] [--verbose]
```

Five checks, run **before any work is claimed**. Each answers one of three states with a
**named remediation**:

| check | proves | refs |
|---|---|---|
| `token-mint-cold` | `desktoken <role>` succeeds in a **fresh process with a scrubbed environment** | #794 #567 |
| `app-scopes-vs-duties` | the installation's recorded grant covers `pull_requests:write`, `issues:write`, `contents:write` | #571 |
| `write-transport` | a **read-only** probe (`git push --dry-run`) of the role's landing path is permitted | #823 |
| `commit-identity` | the commit email's numeric prefix is the roster's **bot USER id**, not the App id | #638 |
| `sibling-checkouts` | the out-of-repo checkouts the **queued** briefs declare are present | #679 |

**Four properties are the whole point.**

1. **Three-state, zero value RED.** `checked-clean` / `checked-failed` / `could-not-check`,
   and the zero value of `CheckState` is `could-not-check`. A check that returns without
   looking cannot read green by omission — which is the exact class of bug the preflight
   exists to catch, so it must not contain one.
2. **A red preflight is `could-not-run` for the WHOLE pass.** Both non-green states map to
   exit **6**: from the pass's point of view "the envelope is broken" and "we could not tell
   whether it is broken" are the same fact — the pass did not run. The desk prints **one**
   line and stops. It does not claim work, burn a pass, or file an issue about its own
   envelope; each failing check names the issue that already owns it.
3. **A probe rejection is a STOP.** Per AGENTS.md ("Scope rejections") a rejection is never
   retried under another identity, and this package supplies no way to. The write-transport
   probe is invoked **exactly once** (`TestPreflightProbeRejectionIsNotRetried`).
4. **Every check ships with a positive control.** `preflight_test.go` carries a red run for
   each of the five — a check with no proof it can fail is indistinguishable from a check
   that always passes.

`deskroster preflight --help` exits **0**; an unknown subcommand exits **5**. That pair is
the capability probe: a consumer answers "does this build have the verb?" in one call with
no output parsing.

There is **no `--skip-preflight` and no env silencer.** A desk that can switch its own
envelope check off has an envelope check only on the passes that would have been green.

**Reference consumer.** `verifyloop plan` runs it at boot before the Awaiting queue is even
read (`cmd/verifyloop/preflight.go`). That file is what an adopting desk copies.

### Tier→runner config — the `ASSAY_RUNNER_*` table

Native ACP dispatch spawns a real agent per verify item. **Which** agent it
spawns is a config table, not code: each dispatchable `Tier` (`local`, `cheap`, `session`) maps
to a pinned runner. `TierHuman` is **non-dispatchable** (it routes via `Land`) and has no entry.
This is the first real home for dual-track runner selection (Claude / Codex / Gemini) — a new
runner is a config row, not a code change (`cmd/verifyloop/runnertable.go`, spec §4.3).

The namespace is **`ASSAY_*` only** — this is generic methodology config, never product-deploy
config (the `config-namespace-split` ruling: product values must not wear the `ASSAY_` prefix,
and this table must not read product keys).

Two config forms; set one:

| Key | Shape | Notes |
|---|---|---|
| `ASSAY_RUNNER_TABLE` | path to a JSON file `{"local": {…}, "cheap": {…}, "session": {…}}` | authoritative when set; per-tier keys ignored |
| `ASSAY_RUNNER_LOCAL` / `ASSAY_RUNNER_CHEAP` / `ASSAY_RUNNER_SESSION` | one JSON entry each | used when `ASSAY_RUNNER_TABLE` is unset |

Each **entry** is a JSON object:

```json
{ "cmd": ["npx", "@agentclientprotocol/claude-agent-acp"], "model": "session-model", "pin": "0.4.1" }
```

- `cmd` (**required**) — the ACP agent argv the engine spawns.
- `model` — the model the runner runs (recorded in the derived `RunnerID`).
- `pin` (**required**) — the runner **version pin**. `RunnerID` is derived from the resolved row
  (`acp:<tier>:<cmd>@<pin>#<model>`) and stamped on every `Result` — attribution the engine
  knows at spawn, not a worker self-report.
- `isolate` — optional; defaults to `true`. An explicit `false` is a **config error** (see below).

**Fail-closed rules** (every violation refuses, deskkit exit **5**; the whole boot pass stops):

1. **Mandatory pinning** — an entry with no `pin` is a config error, same posture as the repo's
   `.assay-versions` pins: the ACP adapter ecosystem renames and releases fast, so an unpinned
   runner is a moving target (spec §7.3).
2. **Unknown / non-dispatchable tier** — a key that is not `local`/`cheap`/`session` refuses (a
   `human` entry is a config error — `TierHuman` is never dispatched).
3. **The C floor** (HP/03 ruling `#1126`) — a runner that cannot **isolate** the implementer
   **REFUSES** the work; it never silently degrades to a shared checkout. `isolate: false` is a
   config error, not an accepted-but-degraded entry. Only *convenience* capability gaps may
   degrade-with-statement — never isolation.
4. **Reachable-but-unconfigured tier** — at boot, the table is validated against the tiers the
   loop's `TierPolicy` can actually emit (`local` always; `session` only when the middle-rung
   flag is on). A reachable tier with no runner is a **startup** error, not a dispatch-time
   surprise (`cmd/verifyloop/preflight.go`).

With **no** `ASSAY_RUNNER_*` key present the loop stays on its legacy single-value
path and this validation is a no-op — additive-and-inert-by-default.

### Where the App credentials live — the #794 ruling

`desktoken`, `deskpost` and `deskevidence` resolve the role pem and `apps.env` — and, for
`desktoken`, the token cache — across an **App-credential search path**, not a fixed
directory:

```
$ASSAY_CONFIG_HOME   (when set — prepended, never replacing)
~/.config/assay      (the shipped default)
```

The key is **read** from the first directory that has it; a new token cache is **written** to
the **head** of the path — so one setting satisfies #794's closing requirement that the
key-lookup dir, the cache dir, the `apps.env` dir and the provisioning dir be the same one.

**Why a knob and not a hardcoded second directory.** This repo's App-provisioning walkthrough
(`docs/github-apps-setup.md`) provisions into a *deployment-specific* directory. That
directory is a product **value**, and a generic tool must not compile one into its resolution
logic — the same rule that keeps product config out of the `ASSAY_` namespace. So the tool
ships the **mechanism** and the deployment supplies the **policy**, exactly as
`ASSAY_RELEASE_REPO` and `ASSAY_WRITEGUARD_CALLOUT` do. Unset is a *complete* configuration,
not a degraded one.

**The tooling settled it, not the argument.** #794 asks for the house directory to be named
in the tool. `leaksweep run --tree` refuses: the literal path is a registered house-local
token and `cmd/desktoken/desktoken.go` ships in the publication tree, so a build carrying it
fails the tree sweep (observed on this brief's own first CI run). Both constraints are
satisfiable at once, which is what the tool does: the **mechanism** is the search path, the
**value** is the deployment's, and the **refusal** names every directory searched, the knob
that adds one, and the withheld walkthrough that records where this deployment provisions.
#794's symptom was a bare `private key not found at <one path>` that named none of the three,
so a fresh-shell mint failure read as a broken tool.

**Scope.** The knob governs the **App-credential plane only** — pem, `apps.env`, token cache.
It deliberately does **not** move `roster.env`. The roster is the trust surface and it fails
**closed**: pointing the whole config home at a directory with no `roster.env` would take the
trust roster down fleet-wide, which is precisely what a bad roster key did once already
(#819).

**Grant sidecar.** A mint also writes `<token-cache>.perms` (0600), the permission map
GitHub returned. The grant is observable *only* in the access-token response, so the minter is
the only component that can record it; `app-scopes-vs-duties` reads it rather than
re-implementing JWT signing to ask GitHub the same question twice. A missing sidecar is
`could-not-check`, never a pass.

## opmetrics — the operator-layer collector

`cmd/opmetrics` measures the **human at the top of the loop**: how much of his message
traffic is plumbing rather than decision. Every other tool here measures the machinery;
this one measures whether the machinery still needs a person as a message bus. It emits
one file per day:

```
<root>/docs/reports/daily/<date>/opmetrics.json
```

```bash
opmetrics --root . --date 2026-07-22 --gh-fixture /tmp/opm-gh.json --trend 7
```

### It reads a person. Here is the line it does not cross.

The inputs are private session transcripts (`~/.claude/projects/**/*.jsonl`), roster
beacons, and dispatch claims. The output is **aggregates only** — counts, ratios,
percentiles, and fixed status codes. It carries no message text, no prompt excerpt, no
file path taken from a transcript, no session id, no branch name, no PR title, no
person's name. Three independent mechanisms hold that line, because one would be a
single point of failure:

| Instrument | What it proves | Where |
|-----------|----------------|-------|
| **Type allowlist** | The `Report` type has no string-typed leaf outside a pinned list of 17 paths. Adding a free string field fails the suite before it can ever run against a real transcript. | `TestLeakEmitStringFieldsAreAllowlisted` |
| **Closed vocabulary** | Every string in the EMITTED json is a fixed literal, a `status`/`reason`/`unmeasured` enum value, or a date. | `TestLeakEmittedStringValuesAreEnumerated` |
| **Sentinel + fragment sweep** | A sentinel planted in a fixture transcript, and every ≥12-character substring of every fixture message, is absent from the artifact — and a **control** test plants a leak to prove the scanner still detects one. | `TestLeakNoTranscriptContentReachesTheArtifact`, `TestLeakScannerSeesPlantedLeak` |

A `reason` is a machine CODE, never a sentence — sentences are where paths leak
(`cannot read /Users/<name>/.claude/…` is a home-directory disclosure committed to a
repo). Human-readable detail goes to stderr, which nobody commits.

**Read-only against its inputs**, measured rather than promised:
`TestLeakTranscriptsAreNeverModified` hashes the fixture transcript tree and the
desk-tools state tree before and after a full emit and fails on any difference.

**No test touches the real home directory.** Every test sets `HOME` to a temp dir and
passes `--transcripts` / `--desk-tools` / `--gh-fixture` at fixture paths. The
production defaults are asserted only for their SHAPE, under an isolated `HOME`.

### Where it runs

On the **operator's machine**, not in CI. None of its inputs exist on the runners, so
`#766`'s "run by the daily harvest" is wrong as written: the day-file rides into
`docs/reports/daily/` **alongside** the harvest, not via it. Wiring the local schedule
is out of scope here — the one-line invocation above is the deliverable.

### Flags

| Flag | Meaning |
|------|---------|
| `--root DIR` | repo root the day-file is written under (required unless `--stdout`) |
| `--date YYYY-MM-DD` | the day to report (default: today) |
| `--transcripts DIR` | session transcripts (default `~/.claude/projects`) |
| `--desk-tools DIR` | state dir holding `roster/` and `claims/` (default `~/.config/assay`, i.e. `deskkit.StateDir()` — **not** the pre-migration `~/.claude/desk-tools/`) |
| `--gh-fixture FILE` / `--gh-json FILE` | gh export of merged PRs (see below). Omitted → the gh-derived blocks report `could-not-check`, never `0` |
| `--trend N` | also compare against the `N` prior day-files under `--root` |
| `--now RFC3339` | the instant staleness is measured from; makes zombie detection deterministic |
| `--tz LOCATION` | IANA zone the day boundary is measured in (default: local). Pin it whenever the answer must be machine-independent |
| `--stdout` | print the JSON instead of writing the day-file (a pure read — creates nothing) |

**opmetrics does not call `gh` itself.** A collector that shells out is one whose tests
either hit the network or exercise a different path than production. It is instead a
pure function of files; the operator's routine produces the export:

```bash
gh pr list --repo <owner>/<repo> --state merged --limit 200 \
   --search "merged:2026-07-22" \
   --json number,createdAt,mergedAt,labels > /tmp/opm-gh.json
```

Optional `readyAt` / `decisionRequestedAt` fields are honoured when a richer (GraphQL
timeline) export supplies them.

### The day-file schema (`opmetrics/1`)

Consumed by the downstream operator metrics. Every numeric field is a **nullable
pointer**: `null` means *not computed*, and the sibling `status`/`reason` say why.
`0` always means a real zero.

```jsonc
{
  "schema": "opmetrics/1",
  "date": "2026-07-22",
  "generatedAt": "2026-07-22T15:00:00Z",
  "classifierVersion": "opmetrics-relay/1",   // see "the ruler" below
  "relay_ratio": 0.5,                          // headline mirror of operator.relay_ratio

  "operator": {
    "status": "ok",                            // ok | could-not-check
    "reason": "",                              // machine code, "" when ok
    "messages_total": 15,                      // operator turns on the day
    "messages_classified": 14,                 // total minus content-free turns
    "relay_messages": 7,
    "substantive_messages": 7,
    "empty_messages": 1,                       // image-only/whitespace; OUT of the denominator
    "relay_ratio": 0.5,                        // relay / classified
    "relay_families": { "sync": 1, "state_echo": 1, "poke": 2, "lookup": 2, "duplicate": 1 },
    "transcript_files": 2,
    "unparseable_lines": 1                     // a half-read transcript must not read as a quiet day
  },

  "intervention": {
    "status": "ok",
    "operator_messages": 15,
    "merged_prs": 5,
    "messages_per_merged_pr": 3
  },

  "decision_latency": {
    "status": "ok",
    "samples": 5,
    "p50_minutes": 120,
    "p90_minutes": 1440,
    "basis": { "ready_flip": 2, "decision_request": 1, "created_at": 2 }
  },

  "session_hygiene": {
    "status": "ok",
    "sessions_observed": 2,
    "sessions_over_24h": 1,
    "roster_beacons": 5,
    "zombie_agents": 2,                        // beacon >60 min stale AND holding open work
    "claims_filed": 2,                         // dispatch claims timestamped on the day
    "claims_open": 3,
    "sessions_active": 2,                      // distinct claim owners
    "unparseable_files": 2
  },

  "correction_recurrence": {
    "status": "ok",
    "corrective_messages": 3,
    "recurrence_candidates": 1                 // CANDIDATES — a prompt to look, not a finding
  },

  "unmeasured": ["prompt-blockage", "operator-think-time"],

  "trend": {                                   // present only with --trend N
    "status": "ok",                            // ok | no-prior-data
    "window_days": 7,
    "prior_files_found": 2,
    "prior_files_other_classifier": 1,         // EXCLUDED from the mean — see below
    "relay_ratio": { "prior_mean": 0.5, "delta": 0 },
    "messages_per_merged_pr": { "prior_mean": 3, "delta": 0 }
  }
}
```

`reason` codes: `transcripts-unreadable` · `no-transcript-files` ·
`no-operator-messages` · `gh-json-not-supplied` · `gh-json-unreadable` ·
`no-merged-prs` · `no-latency-samples` · `roster-dir-unreadable` ·
`claims-dir-unreadable` · `no-prior-day-files`.

### `classifierVersion` — the ruler, versioned

Relay-vs-substantive is a **judgement encoded in code**, so a trend line can move for
two unrelated reasons: the operator's behaviour changed, or the ruler changed. Only the
version tells them apart. Every day-file carries it, and `--trend` **excludes** prior
day-files written by a different classifier from the mean, reporting how many it
excluded (`prior_files_other_classifier`). Every heuristic lives in one file
(`cmd/opmetrics/classify.go`); changing any rule requires bumping the constant in the
same commit, which `TestClassifierVersionIsPinnedToItsScore` enforces.

### What it can and cannot measure

The classifier is scored against a hand-labelled corpus
(`cmd/opmetrics/testdata/labelled/corpus.json`, 44 messages), not asserted:

| `opmetrics-relay/1` | measured |
|---------------------|----------|
| accuracy | **0.8864** (39/44) |
| relay precision | **0.9286** (tp 26, fp 2) |
| relay recall | **0.8966** (fn 3) |

**The failure direction is deliberate and asserted.** Missed relays (3) outnumber
invented ones (2), because a long message is classified substantive even when it
contains a relay cue. So **the emitted relay ratio is a FLOOR** — the operator is doing
at least that much plumbing, probably more. For a diagnostic that must never become a
scorecard, erring toward flattering the subject is the safe direction; the opposite
error would manufacture evidence against a person. `TestClassifierAccuracy…` fails if
that direction ever inverts, naming the README as equally in need of correction.

Deliberately **not** measured, and listed in `unmeasured` rather than faked:
`prompt-blockage` (transcripts do not reliably record permission-prompt wait spans) and
`operator-think-time`. Correction recurrence is reported as **candidates** — the
heuristic cannot separate "the same rule twice" from "two rules sharing vocabulary".

### Three states, everywhere

A block whose inputs could not be read reports `status: "could-not-check"` with a reason
code and leaves its numbers `null`. It never reports `0`. Zero relays and a blind
collector are different facts: an empty transcript tree and a mis-pointed `--transcripts`
look identical from the inside, and collapsing them is how a broken collector reads as a
perfect day. The same rule applies to the trend block, whose "nothing to compare against"
is `no-prior-data`, never a delta of `0` (which would read as *steady*).

## Handoff coverage

Every point where one desk role hands work to another is a place two racers can both act. The
table below is the handoff map, marking which handoffs are guarded by a claim today and which
are still **UNCLAIMED** — a visible gap is the point (adversarial-review PF-4). The "nine" is
a rhetorical count from that review, **reconstructed** here; treat the rows, not the total, as
the record.

| # | Handoff (from→to) | Lock | Owner |
|---|---|---|---|
| 1 | Board Next-up → worker (brief dispatch) | CLAIMED (deskclaim/loopengine) | the-desk, worker-desk |
| 2 | Issue → worker (issue-lane dispatch) | CLAIMED (same primitive) | intake-desk |
| 3 | Worker → PR (branch-as-claim) | CLAIMED (#146 closed: skill docs call `deskclaim`) | intake/fanout |
| 4 | Inbound event → routing action | UNCLAIMED (#129) | intake-desk vs the-desk |
| 5 | Review pass → new issue (filing) | PARTIAL — pr-review-desk's own dual-track race closed (#157: hold-until-both-tracks + union-dedup + `deskfile` compose); #156's fan-out-to-two-workers mechanism (intake-desk claims lock) tracked separately, still OPEN | pr-review-desk |
| 6 | PR → reviewer lanes (review dispatch) | PARTIAL (resume-worker only) | pr-review-desk |
| 7 | Reviewer verdict → ready-flip/merge | UNCLAIMED (human-gated) | pr-review-desk |
| 8 | Merged/Awaiting brief → verify dispatch | CLAIMED (loopengine/verifyloop) | verify-desk |
| 9 | Merged PR → issue closure | UNCLAIMED (#158) — BLOCKED on deskclose #282 | intake-desk |

#696 delivered the primitive + CLI covering **dispatch / issue / branch**
(rows 1-3) and kept **verify dispatch** (row 8) on the same lock. The immediate follow-up —
swapping the desk skills' bash `(set -C; … > "$f")` idiom for `deskclaim acquire` in
`worker-desk` and `intake-desk` (`.claude/skills/` and `plugins/assay/skills/`) — is what
closes #146 end to end: the primitive alone did not help until the skill docs actually called
it instead of the shell idiom the writeguard hook blocks. **Routing, filing, and closing**
(rows 4, 5, 9) remain the next adopters: closing waits on `deskclose` (#282), and filing
composes with `deskfile` (#283).
