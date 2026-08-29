# Claude Code — capability bindings

How each neutral-core capability (named in the skill bodies as `capability:<name>`)
is realised on **Claude Code**. The skill bodies name the capability; this file
names the mechanism. Harness tool names are legal here — that is this file's whole
purpose — and illegal in a skill body (`tools/harnesslint bodies` enforces it).

Source of the capability set: the closed vocabulary block in
[`../../../docs/streams/harness-portability/README.md`](../../../docs/streams/harness-portability/README.md).
Amending the set is a stream-README edit in the same PR.

## Capability → mechanism

| Capability | Claude Code mechanism |
|---|---|
| `capability:dispatch-worker` | The **Agent** tool. One dispatch per Agent call; several in a single message run concurrently as background subagents. A dispatched worker inherits the session's model tier and reports back on completion. |
| `capability:message-agent` | **SendMessage** — resume an existing subagent by id/name with its context intact, e.g. to hand a delta-review a running reviewer's prior findings. |
| `capability:isolate-workspace` | A git worktree the skill creates itself: `git worktree add <path> refs/remotes/origin/main --detach` (spell the base `refs/remotes/origin/main` in full; `--detach` is load-bearing — see the skill bodies for the war stories). Claude Code's Bash tool runs `git worktree add` with no sandbox block, so isolation is always available. |
| `capability:invoke-skill` | The **Skill** mechanism: `SKILL.md` frontmatter (`name`, `description`) is discovered from the plugin bundle, and **description-driven auto-triggering** loads the body when the request matches. Invoke-by-name is also available. |
| `capability:session-notifications` | Background **task notifications**: a dispatched worker finishing re-invokes the parent session. Judge liveness by these completion signals and elapsed time, never by an empty output file. |
| `capability:durable-monitor` | The **Monitor** tool with `persistent: true` — a re-arming poll that survives across turns and re-invokes the session on a new event or a fixed cadence. Check **TaskList** for an existing monitor before arming a second (never arm two). It is **best-effort by construction — NOT the sole wake signal**: pair it with a fixed-cadence board sweep as the liveness backstop, so a dead monitor is loud rather than a silent all-clear. The durable liveness home is the always-on observability service, not this tool. |

## Degradation — per skill

On Claude Code every capability is `supported` (Agent dispatch, SendMessage,
worktree isolation, Skill discovery, and task notifications are all native), so
**every skill `runs`** with no degradation. The column is trivial here by
construction; it is the Codex binding ([`codex.md`](./codex.md)) and the Cursor
binding ([`cursor.md`](./cursor.md)) that carry the `degrades`/`refuses` cells.

| Skill | Claude Code |
|---|---|
| `adopt` | runs |
| `author-brief` | runs |
| `dailies` | runs |
| `intake-desk` | runs |
| `market-intelligence` | runs |
| `pr-review-desk` | runs |
| `the-desk` | runs |
| `verify-desk` | runs |
| `worker-desk` | runs |

## Freshness

The Claude Code column is behavioural fact about a live harness and is registered
in `freshness.yaml` (harness-portability/07) so it rots on a clock rather than by
incident, alongside the Codex and Cursor bindings.
