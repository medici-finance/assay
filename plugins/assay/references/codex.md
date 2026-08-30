# Codex CLI — capability bindings

How each neutral-core capability (named in the skill bodies as `capability:<name>`)
is realised on the **Codex CLI** — the open-source `codex` terminal tool. Target
set is **Codex CLI only for v1**; the hosted Codex App is a later, separately-ruled
surface. Both rulings are recorded in the harness-target ruling (Decision A1 + C,
human:<name> 2026-08-16).

Every mechanism and degradation cell below cites the measured Codex capability
matrix (HP/01, documentary + prior-art, 2026-08-08 — **no live Codex environment**;
the live-harness rows stay BLOCKED until human:<name> provides one). Harness tool names
are legal here — that is this file's purpose — and illegal in a skill body.

Source of the capability set: the closed vocabulary block in
[`../../../docs/streams/harness-portability/README.md`](../../../docs/streams/harness-portability/README.md).

## The non-negotiable floor

Three guarantees **never degrade** on any harness (stream README §3; ruling C):
**isolation**, **evidence-not-claims**, and **the review gates**. Where Codex CLI
cannot provide the underlying capability, the affected skill **refuses** rather
than dropping the guarantee. Only *convenience* gaps (no parallel dispatch, no
reliable background notification) may degrade — explicitly, and in-session.

## Capability → mechanism

| Capability | Codex CLI mechanism (HP/01 §) |
|---|---|
| `capability:dispatch-worker` | `spawn_agent` / `wait_agent` / `close_agent` (V1) behind `[features] multi_agent = true` in `~/.codex/config.toml`; the V2 tool set (`spawn_agent`/`send_message`/`followup_task`/`interrupt_agent`) sits behind `multi_agent_v2.enabled`, with `max_concurrent_threads_per_session` capping parallelism (§3.5). **When `multi_agent` is off, dispatch is unavailable** → the convenience-degradation floor: the skill runs the fan-out **serially, one item at a time, stating the degradation** (§3.5). |
| `capability:message-agent` | V1 `send_input` (message a running child) and `resume_agent(id)` (resume a completed child); V2 `send_message` / `followup_task` (§3.6). Caveat: **V2 resume is broken across process restarts** (codex issues #19140, #33002) — child agents are process-local, not durable, so on session resume a follow-up can return "not found". Prefer V1 resume, or re-dispatch fresh, when durability matters. |
| `capability:isolate-workspace` | The CLI has **no built-in worktree management** (§3.9 `absent`) — it *detects* linked worktrees (`GIT_DIR != GIT_COMMON`) but does not create them, so the skill must run `git worktree add` itself. Under the default `workspace-write` sandbox that ref-write to `.git/refs/heads/` is **blocked** (§3.8), so isolation is **not creatable** → the no-isolation floor applies: the skill **refuses rather than implement in the shared checkout**. Under `--sandbox danger-full-access` worktrees are creatable and isolation runs normally. |
| `capability:invoke-skill` | `SKILL.md` discovery from `.agents/skills/` (cwd, parent dirs, repo root, `~/.agents/skills/`); frontmatter `name` + `description` (§3.2). `allow_implicit_invocation` (defaults `true`, `agents/openai.yaml`) gives **description-driven auto-triggering** (§3.3); **invoke-by-name is the floor** where auto-trigger is disabled — parity of availability, not of trigger ergonomics. |
| `capability:session-notifications` | Main-session completion signals: TUI `agent-turn-complete`, OSC 9, and the `notify` hook (§3.7 `partial`). **Caveat:** a background task spawned with `run_in_background: true` from *inside a subagent* is silently killed when the subagent ends its turn, so its completion notification can never fire — judge subagent liveness by elapsed time and the written verdict, not by a background signal alone. |
| `capability:durable-monitor` | **No reliable durable cross-turn wake signal (§3.6/§3.7 `absent`).** V2 child agents are process-local — resume is broken across process restarts (Issues #19140, #33002) — and an in-subagent `run_in_background: true` task is silently killed when its turn ends (§3.7), so there is no re-arming poll that survives a session restart. A durable wake-signal is a **convenience**, not one of the three never-degrade guarantees (isolation / evidence / gates) → the convenience floor applies: the capability **degrades**. The skill falls back to the **event-driven + fixed-cadence board sweep** (the same backstop Claude Code pairs with its monitor — it is the real liveness safeguard, the durable monitor was only best-effort on either harness) and **states the gap in-session**. The durable liveness home is the always-on observability service, not the harness. |

## Degradation — per skill (Codex CLI, v1 target)

Reproduced from the ruled matrix (ruling C, Codex CLI column) — the serial-fanout
degradation text and the isolation-refusal text are carried **verbatim** so a Codex
session states the ruled posture rather than improvising it.

| Skill | Codex CLI |
|---|---|
| `adopt` | **runs** — needs only `resident-rules-channel` §3.1, `skills-discovery` §3.2, `install-mechanism` §3.10, all `supported`; no dispatch/isolation/gate surface. |
| `author-brief` | **runs** — pure authoring/planning text; touches no dispatch, no isolation, no gate. |
| `dailies` | **runs** — report generation is sequential CLI work; any report fan-out **degrades: runs serially with an explicit in-session statement** (dispatch is a convenience here, not a safety guarantee). |
| `intake-desk` | **runs** — triage/classification is interactive text plus `gh` reads/writes; `gh` write verbs require the `danger-full-access` sandbox (§3.8, blocked under `workspace-write`) — a sandbox precondition, not a gate degradation. |
| `market-intelligence` | **runs** — research + reads; no dispatch/isolation/gate needs. |
| `pr-review-desk` | **runs** — the review **verdict** is text and is never degraded; posting the review and the draft→ready flip need `gh` writes → `danger-full-access` precondition (§3.8). The review gate is upheld or the skill refuses; it never silently degrades. |
| `pr-shepherd` | **refuses under `workspace-write`, runs under `danger-full-access`** — the same isolation floor as `worker-desk`: the skill mandates its own worktree for the adopted PR and must create it itself (§3.9 `absent`), which the `workspace-write` sandbox blocks (§3.8). Its watch loop needs no durable poll — the body already states the periodic `gh pr view` read as the primary form and `capability:durable-monitor` only as an alternative, so the §3.6/§3.7 gap **degrades to the stated poll**, not to blindness. Discovery-mode fan-out (one shepherd per PR) **degrades: serial, one PR at a time, stated in-session** if `multi_agent` is off (§3.5). Gates are unchanged: it never flips, merges or closes, and the security-gate carve-out is a refusal, never a degradation. *(Derived from this file's own capability rows plus the skill's declared needs — not a separate live measurement.)* |
| `the-desk` | **runs** — coordination; its worker fan-out **degrades: serial with an in-session statement** if `multi_agent` is off (§3.5); the review/merge gate is unchanged (the human merges). Isolation of dispatched implementers is delegated to `worker-desk` (below). |
| `verify-desk` | **runs** — executes the Verify table and records **real command evidence** (`evidence-not-claims` intact); `gh` writes need `danger-full-access` (§3.8). The evidence guarantee is never degraded. |
| `worker-desk` | **refuses under `workspace-write`** — the pool cannot create an **isolated worktree**: the CLI has **no built-in worktree management** (§3.9 `absent`), so the skill must create worktrees itself, and doing so (`git worktree add` / `checkout -b` writing `.git/refs/heads/`) is **blocked by the `workspace-write` sandbox** (§3.8). Per the no-isolation→refuse floor it **refuses rather than implement in the shared checkout**. **runs under `danger-full-access`** (worktrees creatable). Dispatch gap → **degrades: the pool runs serially, one brief at a time, with an explicit in-session statement**. Isolation is never silently degraded. |

## Freshness

This binding is behavioural fact about a live harness, measured documentarily on
2026-08-08 with no live environment. It is registered in `freshness.yaml`
(harness-portability/07) so it expires on a clock, and the live-harness rows are
re-measured once a Codex environment exists (the stream's true head).
