# Desk-shell mechanics

<!-- assay:harnesslint non-matrix-reference — harness-neutral shell/transport mechanics, not a per-harness capability binding; the capability-to-mechanism matrix and the per-skill degradation cells live in claude-code.md, codex.md and cursor.md -->

Shell and transport mechanics every desk role re-derives from a refusal — how a session
addresses a repo across tool calls, keeps its work isolated, stamps its commits, carries
its loop and session markers, authenticates a fetch or push, and asks which repositories
its role can reach. None of it is judgment; all of it is the same handful of mechanics,
so it lives here once instead of being rediscovered per session.

This is the fourth file in this bundle's `references/` directory and the first that is not a
per-harness capability binding: [`claude-code.md`](./claude-code.md),
[`codex.md`](./codex.md) and [`cursor.md`](./cursor.md) each bind the neutral
`capability:<name>` vocabulary to one harness, whereas this file is harness-neutral — shell,
git, and desk-tooling mechanics that hold on any harness. The directory's purpose is not
narrowed by it. Where a neutral `capability:<name>` already names a mechanism, this file uses
that vocabulary and names a harness only where the mechanism genuinely is harness-specific.

**House values never appear in this file.** Account slugs and bot user ids, absolute checkout
paths, the roster/config path, the stream-root map, and the driver's name are all deferred to
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

## Authenticated transport

<!-- BEGIN authenticated transport
     This block exists to be DELETED. When desk-tools/08 lands (`deskgit push` and
     `deskgit fetch`, authenticated transport from the role's token file), this whole block
     collapses to naming those two verbs. It is bracketed by the BEGIN/END markers so that
     retirement is a single edit, not a hunt across the file. -->

**Mechanism.** The canonical fetch/push form reads the role's token from its **file** and
presents it through a git credential helper as HTTP Basic auth — username `x-access-token`,
the token as the password. A token embedded directly in the remote URL is refused. Use the
explicit `:443` host form so a local URL rewrite (an `insteadOf` rule) does not silently
re-point the remote. A `401` on push or fetch means one of two things: the token has a short
TTL and has expired (re-mint it), or an OS keychain credential helper is shadowing the one you
supplied (reset `credential.helper` to empty first, then supply yours).

**Signal.** `401 Unauthorized` / `Authentication failed`; or a remote-URL rewrite sending the
push to an unexpected host; or a refusal to accept a token embedded in the URL.

**Correct form.**

```
TOK=$(cat "$ROLE_TOKEN_FILE")
git -C /abs/worktree \
  -c credential.helper='!f(){ echo username=x-access-token; echo "password=$TOK"; };f' \
  push https://github.com:443/<owner>/<repo> HEAD:<branch>
# On 401: either re-mint the short-TTL token, or clear a shadowing OS keychain helper first —
#   git -C /abs/worktree -c credential.helper= -c credential.helper='!f(){ … };f' push …
```

When desk-tools/08 lands, this reduces to `deskgit push` for the push side and `deskgit fetch`
for the fetch side (the latter selects the acting role through a role selector); the flags,
output and refusal texts of those verbs belong to that brief, not to this file.

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
