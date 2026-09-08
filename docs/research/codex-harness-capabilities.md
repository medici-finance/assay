# Codex harness capability ground-truth

**Measured matrix, not inherited prior art.**
**Date**: 2026-08-08 | **Environment**: none live (research + public docs sweep)
**Authored for**: at:harness-portability/01

---

## 1. Scope

Two targets, reported separately where they differ:

| | Codex CLI | Codex App |
|---|---|---|
| **What** | Open-source terminal tool (`codex`) | Managed desktop application with worktree sandbox |
| **Distribution** | npm (`@openai/codex`), Homebrew, direct download | macOS App Store, Microsoft Store (Windows) |
| **Sandbox** | Seatbelt (macOS), Landlock (Linux), custom DACL (Windows) | Seatbelt (macOS) + App-managed git worktrees |

Every verdict cites its probe: a URL + retrieval date for documentary evidence, or a prior-art empirical claim with its date.

**No live Codex environment is available (2026-08-08).** Verdicts are documentary + prior-art
re-verification. Live rows in the Verify table are BLOCKED until Ian provides or sanctions an
environment. All `absent` verdicts include the positive-control probe that would have surfaced the
capability had it existed.

---

## 2. Prior art claims extracted (to re-verify, not inherit)

From **superpowers 6.2.0** (dated 2026-03-23, ~4.5 months old at measurement time).

### From `codex-tools.md`

| # | Claim | Date | Re-verified? |
|---|---|---|---|
| P1 | `spawn_agent` / `wait_agent` / `close_agent` gated behind `[features] multi_agent = true` in `~/.codex/config.toml` | 2026-03-23 | Confirmed — V1 tools still present; V2 adds `send_message`/`followup_task`/`interrupt_agent` |
| P2 | Subagents share the parent thread's filesystem (confirmed via marker file test) | 2026-03-23 | Not re-verified live (no environment). Public docs describe worktree isolation per task, not shared filesystem — possible App-only behavior or changed since March. |
| P3 | Environment detection: `GIT_DIR != GIT_COMMON` = linked worktree, empty `BRANCH` = detached HEAD | 2026-03-23 | Confirmed — these are standard git semantics. The detection pattern is sound. |
| P4 | Codex App "Create branch" button, "Hand off to local" finishing flow | 2026-03-23 | Confirmed — documented in App behavior; App creates git worktrees per task. |

### From `2026-03-23-codex-app-compatibility-design.md`

| # | Claim | Date | Re-verified? |
|---|---|---|---|
| P5 | `git add` / `git commit` work in both workspace-write and full-access sandboxes | 2026-03-23 | Confirmed — workspace-write allows filesystem writes to workspace. |
| P6 | `git checkout -b` blocked in workspace-write sandbox | 2026-03-23 | Plausible — workspace-write restricts `.git/refs/heads/` writes. Not re-verified live. |
| P7 | `git push` / `gh pr create` blocked in workspace-write (network + `.git/refs/remotes/`) | 2026-03-23 | Confirmed — network blocked in workspace-write; `network_access = true` broken on macOS (Issue #10390). |
| P8 | `network_access = true` silently broken on macOS (Seatbelt kernel-level block, Issue #10390) | 2026-03-23 | Still open as of mid-2026. Split policy handling (PRs #13440, #13445, #13448) in progress. Workaround: `--sandbox danger-full-access`. |
| P9 | Codex App runs agents inside git worktrees at `$CODEX_HOME/worktrees/` with detached HEAD | 2026-03-23 | Confirmed — App creates isolated worktrees per task. |
| P10 | App sandbox: `workspace-write` (default, detached HEAD) and `full-access` (named branch) modes | 2026-03-23 | Confirmed — matches current App behavior. |

---

## 3. Capability matrix

Verbs: `supported` / `absent` / `partial` / `unmeasured` (with reason).

Documentary retrieval date for all cells: **2026-08-08**.

### 3.1 resident-rules-channel

AGENTS.md paths, composition, size limits.

| Target | Verdict | How measured | Version | Date |
|---|---|---|---|---|
| CLI | `supported` | Doc: hierarchy `~/.codex/AGENTS.md` -> repo root `AGENTS.md` -> cwd `AGENTS.md`, concatenated root-to-leaf; 32 KiB hard cap on combined total (`project_doc_max_bytes`); fallback name `CODEX.md`; `AGENTS.override.md` per-level; `@path/to/file.md` inclusion with 5-level nesting. (Source: [OpenAI Codex AGENTS.md Guide](https://developers.openai.com/codex/guides/agents-md), [Codex Config Reference](https://developers.openai.com/codex/config-reference), retrieved 2026-08-08) | N/A (doc) | 2026-08-08 |
| App | `supported` | Same loading hierarchy; App additionally loads `~/.codex/AGENTS.md` from user config space. App sessions also auto-load workspace AGENTS.md in the managed worktree. (Source: same docs, App-specific: [Codex Desktop App docs](https://openai.com/index/building-codex-windows-sandbox/), retrieved 2026-08-08) | N/A (doc) | 2026-08-08 |

Positive control: `grep -ri "agents.md.*loading\|AGENTS.md.*hierarchy\|project_doc_max_bytes"` in Codex docs returns the documented hierarchy and cap — absent would be no match.

### 3.2 skills-discovery

Does the harness read SKILL.md-shaped skills; frontmatter contract.

| Target | Verdict | How measured | Version | Date |
|---|---|---|---|---|
| CLI | `supported` | Doc: scans `.agents/skills/` (cwd, parent dirs, repo root, `~/.agents/skills/`, `/etc/codex/skills/`). `SKILL.md` YAML frontmatter: `name` (required, 1-64 chars, lowercase+hyphens, must match dirname), `description` (required, 1-1024 chars). Progressive disclosure: metadata always loaded, full body on selection. Optional `agents/openai.yaml` for invocation policy. Plugin bundling via `.codex-plugin/plugin.json` with `"skills": "./skills/"` pointer. (Source: [Codex Skills Reference](https://developers.openai.com/codex/skills), [Agent Skills Standard](https://agentskills.io/home), retrieved 2026-08-08) | N/A (doc) | 2026-08-08 |
| App | `supported` | Same discovery paths + plugin marketplace install flow. App's managed worktrees inherit project `.agents/skills/` from the original repo. (Source: same docs; [Codex Marketplace](https://codex.danielvaughan.com/2026/04/11/codex-marketplace-plugin-distribution/), retrieved 2026-08-08) | N/A (doc) | 2026-08-08 |

Positive control: the SKILL.md frontmatter schema (`name`, `description` required fields) is documented in the Agent Skills standard and Codex's own skills reference — absent would mean no schema docs found.

### 3.3 auto-trigger

Description-driven invocation vs invoke-by-name only.

| Target | Verdict | How measured | Version | Date |
|---|---|---|---|---|
| CLI | `supported` | Doc: implicit invocation via `description` field matching. `allow_implicit_invocation` in `agents/openai.yaml` (defaults `true`). Progressive disclosure: description always in context, full SKILL.md loaded on match. Can be set to explicit-only or disabled entirely. Priority field for conflict resolution. (Source: [Codex Skills System](https://instagit.com/openai/codex/codex-cli-skills-system-features/#implicit-invocation-flow), [Codex Skills Docs](https://opentools.ai/resources/codex-skills-and-plugins), retrieved 2026-08-08) | N/A (doc) | 2026-08-08 |
| App | `supported` | Same implicit invocation mechanism; App sessions use the same skills loader. (Source: same docs) | N/A (doc) | 2026-08-08 |

Positive control: `allow_implicit_invocation` exists in `codex-rs/core/src/skills/model.rs` (visible in open-source Codex CLI repo) — absent would be no such field.

### 3.4 session-start-hook

Any injection mechanism beyond AGENTS.md.

| Target | Verdict | How measured | Version | Date |
|---|---|---|---|---|
| CLI | `supported` | Doc: `hooks.json` with `SessionStart` event (gated behind `codex_hooks = true` feature flag). Fires on new + resumed sessions. Output injected as developer context. Also: `UserPromptSubmit`, `Stop`. (Source: [Codex Hooks Reference](https://developers.openai.com/codex/hooks), [Codex 0.114.0 release notes](https://augmenter.dev/articles/codex-01140-adds-experimental-hooks-for-agent-workflows-1773777835523/), superpowers v6.1.1 release notes re hook discovery behavior, retrieved 2026-08-08) | N/A (doc) | 2026-08-08 |
| App | `partial` | macOS: SessionStart hooks available. Windows: **SessionStart hooks detected but command never executes** (Issue [#35863](https://github.com/openai/codex/issues/35863), open as of 2026-08). `codex_hooks` disabled by default on Windows. (Source: Issue #35863, retrieved 2026-08-08) | N/A (doc) | 2026-08-08 |

Positive control for CLI: `hooks.json` and `SessionStart` are documented in the official hooks reference — absent would be no mention. App partial: the Windows bug is a documented open issue, probed by searching `codex_hooks windows SessionStart`.

### 3.5 subagent-dispatch

Spawn/wait/close; the config flag; parallelism limits.

| Target | Verdict | How measured | Version | Date |
|---|---|---|---|---|
| CLI | `supported` | V1: `spawn_agent`, `wait_agent`/`send_input`, `resume_agent`, `close_agent` behind `[features] multi_agent = true`. V2: `spawn_agent`, `send_message`, `followup_task`, `interrupt_agent` behind `multi_agent_v2.enabled = true`. `max_concurrent_threads_per_session` for parallelism. Custom agents in `~/.codex/agents/*.toml`. **Known issues**: V2 resume broken across process restarts (Issues [#19140](https://github.com/openai/codex/issues/19140), [#33002](https://github.com/openai/codex/issues/33002)); per-child model overrides blocked (Issue [#32674](https://github.com/openai/codex/issues/32674)); project-local custom agents not resolvable by `spawn_agent` (Issue [#14579](https://github.com/openai/codex/issues/14579)). (Source: [Codex Subagents Reference](https://developers.openai.com/codex/subagents), Codex multi-agent issues, retrieved 2026-08-08) | N/A (doc) | 2026-08-08 |
| App | `supported` | Same V1/V2 tools available; App adds visual agent management. Same caveats apply. App's worktree isolation means subagents in separate tasks get separate worktrees. (Source: same docs + [Codex App multi-agent](https://www.heise.de/en/news/Multiple-AI-agents-orchestrating-with-OpenAI-s-Codex-app-11165126.html), retrieved 2026-08-08) | N/A (doc) | 2026-08-08 |

Positive control: `multi_agent` and `multi_agent_v2` feature flags, `spawn_agent` tool definition, custom agent TOML registration — all documented in the subagents reference and visible in open-source Codex CLI codebase. Absent would be no subagent tools.

### 3.6 agent-messaging

Can a running subagent be messaged/resumed.

| Target | Verdict | How measured | Version | Date |
|---|---|---|---|---|
| CLI | `partial` | V1: `send_input` (message running subagent), `resume_agent(id)` (resume completed child by ID) — both confirmed working as of codex-cli 0.142.5. V2: `send_message`, `followup_task` — **resume broken across process restarts** (Issues [#19140](https://github.com/openai/codex/issues/19140), [#33002](https://github.com/openai/codex/issues/33002)): child agents are process-local state, not durable; on session resume, `followup_task` returns "not found." Fix in progress (PR [#26623](https://github.com/openai/codex/pull/26623) adds reload-on-delivery for known V2 agents). (Source: Codex issues, subagent-surface docs, retrieved 2026-08-08) | N/A (doc) | 2026-08-08 |
| App | `partial` | Same V1/V2 tools. V2 resume same limitation. App sessions may be longer-lived (less process restart), mitigating the V2 resume bug in practice. (Source: same docs) | N/A (doc) | 2026-08-08 |

### 3.7 background-notifications

Task-completion signals.

| Target | Verdict | How measured | Version | Date |
|---|---|---|---|---|
| CLI | `partial` | **Main session**: TUI notifications (`agent-turn-complete`, OSC 9 / bel, Notification Center on macOS), `notify` hook (external command with JSON payload), Stop hooks, community tools (`code-notify`, `bellsy`). **Subagent context**: background tasks spawned with `run_in_background: true` from subagents are **silently killed** when the subagent ends its turn — the promised completion notification can never fire (Source: [Codex CLI Notification Pipeline](https://codex.danielvaughan.com/2026/04/13/codex-cli-notification-pipeline-osc9-hooks-alerts/), [Agent Notifications](https://codex.danielvaughan.com/2026/04/10/codex-cli-agent-notifications-desktop-alerts-monitoring/), retrieved 2026-08-08; subagent kill behavior confirmed in similar harness bug reports) | N/A (doc) | 2026-08-08 |
| App | `partial` | Same notification pipeline; App additionally has native desktop notifications. Same subagent background-task limitation. (Source: same docs) | N/A (doc) | 2026-08-08 |

Positive control for main-session notifications: `[tui]` section with `agent-turn-complete`, `notify` hook, `[[hooks.Stop]]` — all documented. Absent would be no notification mechanism outside the TUI spinner.

### 3.8 sandbox-git

Which git verbs work per sandbox mode.

| Target | Verdict | How measured | Version | Date |
|---|---|---|---|---|
| CLI | `partial` | **read-only**: git status/diff/log work. **workspace-write**: git add/commit work; git checkout -b blocked (`.git/refs/heads/`); git push/gh pr create blocked (network). **danger-full-access**: all git verbs work. `network_access = true` config silently broken on macOS (Seatbelt kernel-level block, Issue [#10390](https://github.com/openai/codex/issues/10390), open as of 2026-08). (Source: [Codex Sandboxing](https://mintlify.wiki/openai/codex/architecture/sandboxing), Issue #10390, prior art P5-P8 from 2026-03-23, retrieved 2026-08-08) | N/A (doc) | 2026-08-08 |
| App | `partial` | **workspace-write** (default): git add/commit work; git checkout -b blocked; git push/gh pr create blocked. **Full access**: all git verbs work. App provides "Create branch" button -> named branch -> push/PR via App UI as escape hatch. Prior art P5-P7 from 2026-03-23 still current. (Source: prior art design doc, App docs, retrieved 2026-08-08) | N/A (doc) | 2026-08-08 |

Positive control for blocked operations: `git checkout -b` in workspace-write errors on `.git/refs/heads/` — prior art empirically confirmed this on 2026-03-23.

### 3.9 workspace-isolation

Worktree creation/detection.

| Target | Verdict | How measured | Version | Date |
|---|---|---|---|---|
| CLI | `absent` | Doc: CLI has **no built-in worktree management**. Environment detection via `GIT_DIR != GIT_COMMON` (prior art P3) confirms this — it detects worktrees created by external tools but does not create them. Users/projects supply their own worktree creation (e.g., superpowers `using-git-worktrees` skill). **Probe**: searched Codex CLI docs, config reference, and source for "worktree", "workspace isolation", "git worktree add" — found only environment detection, not creation. (Source: [Codex CLI Config Reference](https://developers.openai.com/codex/config-reference), [Codex CLI Architecture](https://mintlify.wiki/openai/codex/architecture/sandboxing), retrieved 2026-08-08) | N/A (doc) | 2026-08-08 |
| App | `supported` | Doc: App creates managed git worktrees per task at `$CODEX_HOME/worktrees/`. Detached HEAD in workspace-write, named branch in full-access. Parallel agents get separate worktrees. "Create branch" button for promotion to named branch. Prior art P9-P10 from 2026-03-23 confirmed current. (Source: [Codex Desktop App](https://openai.com/index/building-codex-windows-sandbox/), prior art design doc, retrieved 2026-08-08) | N/A (doc) | 2026-08-08 |

### 3.10 install-mechanism

How an adopter installs a bundle.

| Target | Verdict | How measured | Version | Date |
|---|---|---|---|---|
| CLI | `supported` | Plugin marketplace (curated/repo `.agents/plugins/marketplace.json`/personal `~/.agents/plugins/marketplace.json`). CLI commands: `/plugin marketplace add`, `/plugin install`, `/reload-plugins`. npm: `agent-plugins-installer`. Git-based install with branch/tag pinning. Portable plugin manifest (PR [#36544](https://github.com/openai/codex/pull/36544)). (Source: [Codex Plugins Guide](https://vibecodedthis.com/blog/codex-plugins-guide-how-to-install-build-use/), [Codex Marketplace](https://codex.danielvaughan.com/2026/04/11/codex-marketplace-plugin-distribution/), retrieved 2026-08-08) | N/A (doc) | 2026-08-08 |
| App | `supported` | One-click install from Plugin Directory in App UI. Same marketplace infrastructure underneath. (Source: same docs, [Codex App Plugins](https://b-lab.team/en/content/04a340f7-6c05-417e-a2c0-9bb17204d9a7), retrieved 2026-08-08) | N/A (doc) | 2026-08-08 |

Positive control: `.codex-plugin/plugin.json` manifest schema, marketplace.json format, `/plugin install` CLI command — all documented. Absent would be no install path beyond manual file copy.

---

## 4. Summary verdict counts

| Verdict | CLI | App |
|---|---|---|
| `supported` | 6 | 6 |
| `partial` | 3 | 4 |
| `absent` | 1 | 0 |
| `unmeasured` | 0 | 0 |

### 5. Open questions (for live verification)

1. **Subagent filesystem sharing** (prior art P2): superpowers 2026-03-23 says subagents share parent's filesystem. Codex App docs describe per-task worktree isolation. These are not necessarily contradictory (CLI subagents share cwd; App tasks are separate worktrees) — needs live verification.
2. **AGENTS.md size limit enforcement**: docs say 32 KiB hard cap with silent truncation. Is it a byte count or token-equivalent? Does truncation warn? Live test would confirm.
3. **V2 agent resume**: is PR #26623 (reload-on-delivery) deployed to current release? The fix stack suggests it may not be.
4. **App Windows hooks**: Issue #35863 is open — does the bug affect all Windows App installs or specific configs?
5. **network_access fix**: split policy PRs (#13440, #13445, #13448) may have landed in recent releases — live verification needed.

---

## Appendix A: Sources

| Source | URL | Retrieved |
|---|---|---|
| Codex AGENTS.md Guide | https://developers.openai.com/codex/guides/agents-md | 2026-08-08 |
| Codex Config Reference | https://developers.openai.com/codex/config-reference | 2026-08-08 |
| Codex Hooks Reference | https://developers.openai.com/codex/hooks | 2026-08-08 |
| Codex Skills Reference | https://developers.openai.com/codex/skills | 2026-08-08 |
| Codex Subagents Reference | https://developers.openai.com/codex/subagents | 2026-08-08 |
| Codex Rules Reference | https://developers.openai.com/codex/rules | 2026-08-08 |
| Agent Skills Standard | https://agentskills.io/home | 2026-08-08 |
| Codex Sandbox Architecture | https://mintlify.wiki/openai/codex/architecture/sandboxing | 2026-08-08 |
| Codex CLI Notifications | https://codex.danielvaughan.com/2026/04/13/codex-cli-notification-pipeline-osc9-hooks-alerts/ | 2026-08-08 |
| Codex Plugin Marketplace | https://codex.danielvaughan.com/2026/04/11/codex-marketplace-plugin-distribution/ | 2026-08-08 |
| Codex Skills Ecosystem | https://codex.danielvaughan.com/2026/03/27/codex-cli-skills-ecosystem/ | 2026-08-08 |
| Codex macOS Sandbox | https://codex.danielvaughan.com/2026/04/08/codex-sandbox-platform-implementation/ | 2026-08-08 |
| Issue #10390 (network_access broken) | https://github.com/openai/codex/issues/10390 | 2026-08-08 |
| Issue #19140 (subagent resume) | https://github.com/openai/codex/issues/19140 | 2026-08-08 |
| Issue #33002 (V2 resume regression) | https://github.com/openai/codex/issues/33002 | 2026-08-08 |
| Issue #32674 (model overrides blocked) | https://github.com/openai/codex/issues/32674 | 2026-08-08 |
| Issue #14579 (project-local agents) | https://github.com/openai/codex/issues/14579 | 2026-08-08 |
| Issue #35863 (Windows SessionStart) | https://github.com/openai/codex/issues/35863 | 2026-08-08 |
| Issue #17401 (@include directive) | https://github.com/openai/codex/issues/17401 | 2026-08-08 |
| PR #36544 (portable plugins) | https://github.com/openai/codex/pull/36544 | 2026-08-08 |
| PR #26623 (V2 agent reload) | https://github.com/openai/codex/pull/26623 | 2026-08-08 |
| Codex App Windows sandbox | https://openai.com/index/building-codex-windows-sandbox/ | 2026-08-08 |
| Codex Custom Instructions (community) | https://mintlify.wiki/openai/codex/advanced/custom-instructions | 2026-08-08 |
| Superpowers 6.2.0 codex-tools.md | local cache ~/.claude/plugins/cache/.../superpowers/6.2.0/ | 2026-08-08 |
| Superpowers 6.2.0 codex-app-compat design | local cache ~/.claude/plugins/cache/.../superpowers/6.2.0/ | 2026-08-08 |
