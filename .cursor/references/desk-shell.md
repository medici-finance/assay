<!-- GENERATED from plugins/assay/references/desk-shell.md by deskinstall --harness cursor. Do not edit. -->
# Desk-shell mechanics

<!-- assay:harnesslint non-matrix-reference — harness-neutral shell/transport mechanics, not a per-harness capability binding; the capability-to-mechanism matrix and the per-skill degradation cells live in claude-code.md, codex.md and cursor.md -->

Shell and transport mechanics every desk role re-derives from a refusal — how a session
addresses a repo across tool calls, keeps its work isolated, stamps its commits, carries
its loop and session markers, authenticates a fetch or push, asks which repositories
its role can reach, and runs a forge CLI call as its role. None of it is judgment; all of it is the same handful of mechanics,
so it lives here once instead of being rediscovered per session.

This is the fourth file in this bundle's `references/` directory and the first that is not a
per-harness capability binding: [`claude-code.md`](./claude-code.md),
[`codex.md`](./codex.md) and [`cursor.md`](./cursor.md) each bind the neutral
`capability:<name>` vocabulary to one harness, whereas this file is harness-neutral — shell,
git, and desk-tooling mechanics that hold on any harness. The directory's purpose is not
narrowed by it. Where a neutral `capability:<name>` already names a mechanism, this file uses
that vocabulary and names a harness only where the mechanism genuinely is harness-specific.

**House values never appear in this file.** Account slugs and bot user ids, absolute checkout
paths, the roster/config path, the stream-root map, the cell name and comms gateway address, and the
driver's name are all deferred to
the consuming project's own house-rules doc (`CLAUDE.md`) — this reference states the mechanism
and the shape, never a concrete value. That deferral is the whole point of the file: one neutral
reference the dispatch kit loads, with the values resolved by the project layer.

## One call, one chain

**Mechanism.** A session's working directory is not guaranteed to persist between tool calls —
each call may start a fresh shell. So every repo-touching command is either a single
`cd <absolute-path> && …` chain, or uses the tool's own `-C <absolute-path>` flag. A bare `cd`
on its own line, or a relative path that assumes the previous call left you in the right
directory, is a bug waiting for the next call.

**Signal.** A command that "worked last call" now runs somewhere else: `fatal: not a git
repository (or any of the parent directories)`, an edit that lands in the wrong tree, or a
guard refusal because you were addressing the shared checkout without meaning to.

**Correct form.** `git -C /abs/path status`, or `cd /abs/path && <command>`. Always an
absolute path; never a bare `cd`; never a relative path spanning two calls.

## Workspace isolation, and content-triggered write-guard refusals

**Mechanism.** Work in your own worktree, never a shared checkout. A write guard refuses
mutations aimed at a shared checkout — and it can refuse on the *text* of a command, not only
its literal target: a board-writing tool named on the command line, or a shared path appearing
inside a heredoc body, can trip the guard even when the real write is somewhere else entirely.

**Signal.** A guard block that names the shared path or the token it matched and refuses to
run the command — including cases where the command's actual target was outside the shared
checkout and only the *wording* matched.

**Correct form.** Reach the write a different way, not a different shell form of the same
write: use your harness's file-write capability, or an absolute target outside the shared
checkout. Rephrasing the same shared write as a second shell incantation to slip past the
match is routing around the guard. A guard refusal is a STOP — quote it verbatim and escalate;
never evade it.

## Commit identity

**Mechanism.** Role identity is supplied inline, per commit, with `-c user.name` and
`-c user.email` on the `git commit` invocation. A repo-level identity write (`git config
user.*`) inside a **linked worktree** does not stay local to that worktree — it lands in the
**shared** config and changes the identity of every other worktree on that checkout. A bot's
noreply email is keyed to the bot's **user id** (the numeric id read from the hosting forge's
users API for the `<slug>[bot]` account), not to the App id — the two are different numbers,
and building the address from the App id mis-attributes the commit.

**Signal.** Commits authored by the wrong identity; a sibling worktree's author silently
changed after you ran `git config` in yours; a noreply address the forge does not link back to
the bot account.

**Correct form.** Stamp every commit inline:

```
git -C /abs/worktree \
  -c user.name='<role-bot-name>' \
  -c user.email='<bot-user-id>+<slug>[bot]@users.noreply.github.com' \
  commit -m '<message>'
```

Never `git config user.*` in a linked worktree. The concrete bot name and user id are house
values — they resolve from the project's house-rules doc, not from here.

## Loop and session markers

**Mechanism.** The loop-role marker and the session marker are read from the environment by the
desk verbs that need them. Because a fresh shell may back each tool call, they must be exported
in the **same** shell — the same `&&` chain, or the same tool call — as the verb that reads
them. An export in a previous call is already gone.

**Signal.** A desk verb refuses with an empty-marker complaint — no session marker set, or the
loop-role marker unset — even though "it was exported earlier."

**Correct form.** Export the markers alongside the verb in one chain:

```
export DESK_SESSION='<session-id>' && export <LOOP_ROLE_MARKER>='<role>' && <desk-verb> …
```

The concrete marker names and values are the project's config; the invariant is that the export
and the verb share a shell.

## Scratch files

**Mechanism.** A per-invocation scratch file — a body for `deskpr create`/`deskreply`, a JSON
snapshot passed between two verbs — is minted fresh on every invocation, never a fixed path in
a shared scratch directory. A fixed name is a race: two concurrent sessions (or two parallel
workers sharing one scratch dir) converging on the same path can each overwrite the other's
file, so the loser posts — or plans against — the OTHER session's content. The mechanism this
name resolves to is platform-specific but the property (unique, private, collision-proof) is
not: unix/POSIX shells use `mktemp` (`mktemp "${TMPDIR:-/tmp}/name.XXXXXX"` — the explicit
template argument is portable across BSD and GNU `mktemp`), PowerShell uses
`New-TemporaryFile`, and Go code uses `os.CreateTemp("", "name-*")`. All three create the file
with an exclusive/atomic open and hand back the name that won, so no `$$`/date/session suffix
can alias it.

**Signal.** Two sessions' writes interleaved in one file; a body or plan carrying content that
does not match what this session intended to write; a scratch directory that never empties
because nothing owns cleanup of a fixed name.

**Correct form.** Mint the file where the step runs, immediately before use — never a path
composed ahead of time and handed across a call boundary:

```
BODY=$(mktemp "${TMPDIR:-/tmp}/pr-body.XXXXXX")     # unix
$body = New-TemporaryFile                            # PowerShell
f, _ := os.CreateTemp("", "pr-body-*")                # Go
```

A skill body names this mechanism as "a per-invocation scratch file (desk-shell.md §Scratch
files)" rather than spelling `mktemp` as if every adopter's shell has it.

## Managed task scratch

House-cell interactive sessions and cadence passes receive a machine-owned scratch
workspace through `TMPDIR`, `TMP`, and `TEMP`. Use it for unique body files and
disposable output. Keep source changes in the assigned Git worktree. For standalone
bounded commands, `cellctl scratch <cell> run --source <owned-worktree> --task <task>
--session <session> -- <command>` supplies the same lifecycle. Its optional tracked
snapshot and explicit extra inputs have a byte budget; never recursively copy a
working directory with generated history or scratch output into another scratch tree.

The top-level owner waits for every task sharing that scratch, persists required
outcomes/evidence to the existing canonical destination, reads them back, then calls
`cellctl scratch <ASSAY_SCRATCH_CELL> ack --id <ASSAY_SCRATCH_ID>
--receipt <canonical-evidence-reference>` before the final completion message.
A delegated agent never acknowledges its parent's scratch. Pending evidence and
resumable work stay protected. No further required output may be created after ack.
On failures, publish required diagnostics before acknowledgment too; the runtime
retains only its bounded output tail, receipt and result after handoff.

The runtime releases/reclaims scratch on ordinary completion or catchable termination;
restart cleanup requires the execution lease and child-liveness proof. One owner,
`cellctl scratch`, handles dry-run/apply, budgets and conservative recovery. Git
worktrees remain `deskwt`'s responsibility. Unmarked legacy or harness owned
directories are inventory-only; neither names nor age establish deletion authority.
See `docs/cellctl-scratch.md` in the toolkit for flags, retention limits and supported
host paths. Do not add shell deletion hooks to individual roles.

## Config home

**Mechanism.** The desk tools' own config/roster/state directory is the literal path
`~/.config/assay`, where the leading `~/` is expanded via `os.UserHomeDir()`
(`tools/desk/internal/deskkit/appconfig.go`'s `expandHome`) — never `os.UserConfigDir()` and
never `%APPDATA%`. `os.UserHomeDir()` returns `%USERPROFILE%` on Windows, so the SAME literal
path resolves there too, as `%USERPROFILE%\.config\assay` — this is the brief-02 ruling
(`docs/streams/windows-port/portability-audit.md` §Config-home recommendation): keep
`~/.config/assay` unchanged rather than branch on `%APPDATA%`, because it already works on
Windows through the one home-dir resolver and stays a single documented path across every
platform. A skill body that spells this as a bare `~/.config/assay/…` literal is stating the
resolved unix path as if it were POSIX-only, when the mechanism (one `~/`-relative path,
expanded by `os.UserHomeDir()`) already covers Windows.

**Signal.** A step that reads as if it requires a POSIX home directory (`~/...`) when the
actual requirement is only "the config home, resolved by the one `~/`-expansion every
platform shares"; a Windows session unsure whether a `~/.config/assay/HEARTBEAT` instruction
applies to it.

**Correct form.** Name the mechanism, not just the unix-looking literal: "the config home
(desk-shell.md §Config home: `~/.config/assay`, expanded via `os.UserHomeDir()` — resolves to
`%USERPROFILE%\.config\assay` on Windows)" rather than a bare `~/.config/assay` with no
cross-platform note. The literal path is correct on every platform; what a skill body must
stop implying is that the `~/` spelling is a unix-only shell expansion rather than a resolved
Go path.

## Authenticated transport

<!-- BEGIN authenticated transport -->

Fetch and push under a role's identity with `deskgit fetch` and `deskgit push`, each in its
role-bound `--as <role>` form. Their flags, output, refusal texts and exit codes are
documented in `tools/desk/README.md` §"Authenticated transport — `--as <role>` (fetch and
push)", not in this file.

`deskgit` binds to the current working directory; it does not accept git's `-C`
option or a repository operand. Select the already-resolved repository directory
before invoking it, in the same shell call. For a worker's board-root refresh:

```bash
(cd "$root" && deskgit fetch --as worker) && fanoutloop plan --root "$root"
```

`root` is an absolute path from the configured board-root inventory. The subshell
keeps the caller's worktree as its working directory after the read. The `&&`
prevents a failed directory selection or fetch from producing a stale plan. Select
the bound role for other desks; never infer supported flags from `git`'s interface.

Never hand-roll the credential-helper recipe these verbs replace: they have no raw fallback,
whatever they exit with. When one refuses or cannot verify a step, fix the cause it names if
that cause is the session's own state (for example, check out a branch instead of working on a
detached HEAD); otherwise stop and escalate.

<!-- END authenticated transport -->

## Which repositories a role can see

**Mechanism.** Whether a push, fetch, or post will work depends on whether the role's App is
installed on the target repository — a coverage question, distinct from whether the credential
is valid. To list the repositories a role's App installations can reach, use
`desktoken coverage <role>` (desk-tools/09). Do not hand-roll a probe, and do not restate that
verb's output shape here — it belongs to that brief.

**Signal.** A push or post fails not on the credential but because the role's App is not
installed where you are aiming it.

**Correct form.** `desktoken coverage <role>`.

## Forge CLI as the role

<!-- BEGIN forge cli as the role -->

**Mechanism.** A desk's only forge credential is its role App's, minted by `desktoken <role>`.
`desktoken` prints the PATH of an owner-only custody file on stdout, never the token itself,
and exits 0 ok · 3 disabled · 5 refused · 6 unverifiable. The desk verbs resolve that same
credential on their own from `$DESK_LOOP` and have no ambient fallback. A bare `gh` call has no
such resolution. It runs as whatever login the local CLI holds, usually a trusted human's
personal one, or fails when there is none. A trusted human on the roster is a person whose
comments and rulings count. That login is not a role credential, and a desk never borrows it,
whether through the CLI's stored login or by reading it with `gh auth token`.

**Prefer a desk verb.** When a desk verb covers the call, use the verb and skip `gh`:
`deskread` for issue, change and comment reads (`deskread --help` lists its kinds),
`deskboard` for board views, and `deskfile`, `deskpr`, `deskreply` and `deskpost` for writes.
Use `gh` only for a call that no verb covers, and then only through the recipe below.

**Correct form.** Run the whole recipe in one shell call (§One call, one chain). It works in
bash and zsh. The role comes from `$DESK_LOOP` and nowhere else: a role named by hand could
differ from the loop's. The `case` arms mirror the compiled loop-to-role table (`loopTokenRoles`
in `tools/desk/internal/deskkit/roletoken.go`, retired loop names included), and a test holds
the two in step. The recipe stops before any `gh` call unless `desktoken` exits 0, prints a
path, and that path names a file holding a token: something other than whitespace. A size
check alone is not enough. A file holding only a newline has a non-zero size but yields an
empty `GH_TOKEN`, and `gh` treats an empty `GH_TOKEN` as unset and runs as the host's stored
login, which is a human's, and exits 0.

```bash
repo='<owner>/<repo>'
case "${DESK_LOOP:-}" in
  the-desk)                 role=desk ;;
  worker-desk|batch-fanout) role=worker ;;
  pr-review-desk)           role=reviewer ;;
  verify-desk)              role=verifier ;;
  intake-desk|issue-loop)   role=issue-loop ;;
  *)                        role= ;;
esac
if [ -z "$role" ]; then
  printf '%s\n' "desk-auth: refused: \$DESK_LOOP is unset or names no desk role; export this window's loop name in this shell" >&2
  false
elif ! tokfile="$(desktoken "$role" --repo "$repo")"; then
  printf '%s\n' "desk-auth: unauthenticated: role App custody for '$role' is unavailable (desktoken failed; its stderr names the cause)" >&2
  false
elif [ -z "$tokfile" ] || [ ! -f "$tokfile" ] || ! grep -q '[^[:space:]]' "$tokfile"; then
  printf '%s\n' "desk-auth: unauthenticated: role App custody for '$role' is unavailable (desktoken printed no path, or the file is missing, empty or whitespace only)" >&2
  false
else
  GH_TOKEN="$(cat "$tokfile")" gh issue list -R "$repo" --limit 1 --json number
fi
```

Prefix each further `gh` call in the same shell call the same way. Scope `GH_TOKEN` to one
command. Never `export` it, and never leave it set in a long-lived or shared shell. Never print,
echo, copy or log the token or the custody file's contents. Only the path may appear in output.
Never run the recipe under shell tracing (`set -x`, `zsh -x`): a trace prints the expanded
`GH_TOKEN=` assignment.
In PowerShell, set the variable for the one call and remove it afterwards:

```powershell
$tok = (Get-Content -Raw -LiteralPath $tokfile)
if ([string]::IsNullOrWhiteSpace($tok)) { throw "desk-auth: unauthenticated: role App custody for '$role' is unavailable (the file is empty or whitespace only)" }
try { $env:GH_TOKEN = $tok.Trim(); gh issue list -R $repo --limit 1 --json number }
finally { Remove-Item Env:GH_TOKEN -ErrorAction SilentlyContinue; Remove-Variable tok -ErrorAction SilentlyContinue }
```

Before those lines, apply the same checks as the bash form: the `$env:DESK_LOOP` role mapping,
a zero `$LASTEXITCODE` from `desktoken`, and a path that names an existing file. The
`IsNullOrWhiteSpace` test is the value check: never set `GH_TOKEN` from a value that fails it.

**Two failures, two diagnoses.** Keep them apart. A report that merges them sends the repair to
the wrong identity.

- **Desk unauthenticated.** `desktoken` exits non-zero, prints no path, or names a missing or
  empty file, or one holding only whitespace. This is the ONLY desk-authentication failure.
  Report it as `desk-auth: unauthenticated: role App custody for '<role>' is unavailable`,
  with the refusal `desktoken` wrote to stderr, then stop and escalate the custody gap under
  the desk's escalation rules. A token that mints but is refused on the target repository
  is a coverage gap, not a custody one (§Which repositories a role can see).
- **No human CLI login.** `gh auth status` reports no login, or `gh auth token` fails for a
  roster human. That is a fact about the host, not about the desk. Report it, if at all, as
  `no human gh login on this host: not a desk-authentication failure`, and carry on under the
  role token. It matters only for a command the human driver runs as themselves, such as a
  ruling posted from the driver's own account. That is the driver's own path, and the driver
  repairs it.

**Personal OAuth is never the default repair.** Never repair a desk failure by running `gh auth
login` with a person's account, reading a roster human's token, or exporting a personal token.
The repair is the role's custody: the cause `desktoken` names, provisioning the App, its
installation coverage, or `desktoken <role> --fresh` after a permission change. When the session
cannot make that repair, it escalates.

<!-- END forge cli as the role -->

## Cross-desk transport — the comms lane

**Mechanism.** A hand-off to another desk is addressed to that role's LANE at the cell gateway,
through the client verbs `deskcomms send` (payload on stdin; `--to <role>`, `--verb <verb>`,
`--ref <id>` repeatable, `--class routine|sensitive`, `--to-cell <cell>` for the coordinator's
cross-cell sends only), `deskcomms poll --json` (this session's own per-role mailbox, including payloads) and
`deskcomms ack <id>` (moves, never deletes). The sender's `{cell, role}` come from the session
context the launcher exports — the cell marker and the role marker `deskcomms --help` names —
never from an argument: a caller says who a message is FOR, never who it is FROM. The gateway's
loopback address and the per-role signing key are read from the environment variables that same
help text names; their VALUES are the project's cell config (the key file is mode 0600, never
committed). The client preflight (reserved-verb → identity → parse → lane ACL → content scan →
rate limit → mint → submit) calls the same functions the gateway re-runs authoritatively, so a
refusal here is behaviour-identical to the gateway's — and an agent that never runs the client
verb meets the whole stack at the gateway anyway. There is no local-spool fallback: an
unreachable gateway is a refusal, not a queue. New message IDs use hyphenated UUID v4
format so a recipient can quote the ID in a reply without a bare secret-shaped hex run.
Caller-generated correlation/run IDs should use the same UUID format, preserving the
separators end to end. IDs identify messages; they grant no authority. Older opaque IDs
remain valid for poll/ack. The payload scan still applies: a field named correlation ID
is not a secret exemption, and a refused payload is never reformatted and retried to bypass
its refusal. Which verb a hand-off uses, and the five hand-off
kinds, are stated once in the "Cross-desk hand-offs — the lane verbs" section every desk-role
skill carries; this file owns only the transport.

**Signal.** `refused: reserved-verb …` (a human-gate move named as a verb); `refused:
cross-cell-verb …` or an out-of-lane refusal (the lane ACL); `session identity is not
established` (the cell or role marker is missing from THIS shell); a gateway-unreachable refusal
(the plane is not enabled here, or the address is unset). Exit **3** disabled · **4**
rate-limited · **5** refused · **6** unverifiable.

**Correct form (enabled lane only).** Export the markers in the SAME chain as the verb (§Loop and session markers),
then ONE send with the kind on the payload's first line:

```
printf '%s\n' 'request-act' '<the one action, plus its evidence pointers>' \
  | deskcomms send --to <role> --verb handoff --ref <issue-or-pr-id>
deskcomms poll --json     # every sweep after cutover: read your own lane and payloads
deskcomms ack <id>        # once acted on
```

`poll --json` needs a `deskcomms` from a desk-tools release that ships the flag; an older
binary refuses it as an unknown flag. Install matching binaries rather than reading plain
`poll` output, which omits payloads.

Select the transport from the project's recorded cutover state before invoking a comms verb.
The shared "Cross-desk hand-offs — the lane verbs" skill block owns the pre-cutover branch,
including skipping `poll` while explicitly config-off and continuing the work-queue sweep.
Absent identity or gateway configuration alone is not evidence that this branch applies;
a failed enabled lane never permits fallback. Unknown or conflicting state stops comms only, not
the work-queue sweep; the block names where the record is read and how an absent key reads.


## Optional local performance context

Operator-selected local collection uses the existing perfrec instrument.
Launchers never create opt-in; explicit workflow consent permits bounded local
work linkage. Keep immutable context per launch: new agents get new opaque
session/agent identities, ordinary tool children get fresh invocation identities,
and work is selected for each assignment. A typed child runner passes its own
invocation only as the child's parent. Explicit resume uses the existing
launcher's retained private native-ID mapping or reports a continuity gap.
Do not place this data in a shared global shell or public workpad.

The canonical flag/environment table is in the attribution specification.
Use the shared parser bindings and public perfrec resolver, including empty
clears and replacement work lists. Attribution never changes the operational
DESK_LOOP. When a child needs a different operational loop, preserve originating
desk attribution through --perf-loop-role on supported tool calls. Unsupported
adapters remain explicit gaps; optional writer failure cannot block the operation.

Apply validated handoff entries to each tool call with per-command `env KEY=value
... tool ...` arguments or that tool's supported `--perf-*` flags. Quote values;
never evaluate the handoff as shell code or export it into a persistent shell.
Use `--perf-loop-role` for descriptive desk attribution; never alter `DESK_LOOP`.
