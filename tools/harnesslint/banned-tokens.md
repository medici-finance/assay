# harnesslint — banned tokens for neutral-core skill bodies

Config for `harnesslint bodies`. Each line below the marker is one banned token,
in the form `TOKEN :: REASON`. A skill body under `plugins/assay/skills/*/SKILL.md`
that contains a banned token is a lint error (exit 1), the token and its reason
named at `file:line`.

**Neutrality cuts both ways.** The neutral core may name NO harness's tools — not
just not-Claude's. So the list bans Claude Code tool/hook/env names AND Codex
tool names. A harness tool name is LEGAL inside `plugins/assay/references/*.md`
(that is their whole purpose) and ILLEGAL in a skill body.

Matching is a **case-sensitive substring** match. Token forms are chosen to be
specific: `` `Agent` `` is banned *only in its backticked tool-reference form*, so
the plain English word "agent" ("dispatch a worker agent", "a reviewer agent")
is untouched — exactly the distinction the fixture suite proves in both
directions. Extend this list (with a reason) when a new harness touchpoint is
found; the extension travels with the tool via `//go:embed`.

**Deliberately NOT banned — `AGENTS.md` as a bare string.** `AGENTS.md` is a
Codex resident-rules *mechanism*, but it is also a legitimate governance-file
reference the bodies already make ("never retry it under another identity
(AGENTS.md, …)"). Banning the bare string would flag those legal references. The
resident-rules delivery mechanism is harness-portability/05's surface, not a
skill-body touchpoint, so the Codex mechanism tokens banned here are the dispatch
tools (`spawn_agent` et al.), not the rules-file name.

<!-- assay:banned-tokens
`Agent` :: Claude Code dispatch tool (backticked tool-reference form) — a skill body names the `dispatch-worker` capability, not the harness tool; the reference file maps it. Plain-word "agent" is fine.
SendMessage :: Claude Code agent-messaging tool — use the `message-agent` capability; the reference file maps it to SendMessage on Claude.
Task tool :: Claude Code's Task dispatch tool — use the `dispatch-worker` capability.
CLAUDE_PLUGIN_ROOT :: Claude Code plugin-root env var — a harness-specific path mechanism; keep it in the Claude reference file.
claude.ai :: Claude-specific host — a skill body must name no harness's endpoints.
SessionStart :: Claude Code hook event — the resident-rules channel is a per-harness mechanism (see harness-portability/05); the body must not name it.
spawn_agent :: Codex dispatch tool (V1) — neutrality cuts both ways; the core names no harness's tools. Use `dispatch-worker`.
wait_agent :: Codex dispatch tool (V1) — use `dispatch-worker`; the Codex reference file maps it.
close_agent :: Codex dispatch tool (V1) — use `dispatch-worker`; the Codex reference file maps it.
resume_agent :: Codex dispatch/resume tool (V1) — use `message-agent`; the Codex reference file maps it.
send_input :: Codex subagent-messaging tool (V1) — use `message-agent`.
send_message :: Codex subagent-messaging tool (V2) — use `message-agent`.
followup_task :: Codex subagent follow-up tool (V2) — use `message-agent`.
interrupt_agent :: Codex subagent tool (V2) — use `dispatch-worker`/`message-agent` per intent.
multi_agent :: Codex dispatch config flag — a harness-specific enablement detail; keep it in the Codex reference file.
`Monitor` :: Claude Code durable-watch tool (backticked tool-reference form) — a skill body names the `durable-monitor` capability, not the harness tool; the reference file maps it. Plain-word/lowercase "monitor" in observability prose is fine (harness-portability/11).
`TaskList` :: Claude Code tool for checking an existing monitor before arming a second — use the `durable-monitor` capability; the reference file maps it (harness-portability/11).
`EnterWorktree` :: Claude Code worktree tool (backticked form) — use the `isolate-workspace` capability; the reference file maps it to `git worktree add` on Claude (harness-portability/11).
persistent: true :: Claude Code Monitor arg for a cross-turn durable poll — a harness-specific mechanism; name the `durable-monitor` capability, keep the arg in the Claude reference file (harness-portability/11).
-->
