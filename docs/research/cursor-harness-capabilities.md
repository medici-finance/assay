# Cursor harness capability ground-truth

**Measured matrix, not inherited prior art.**
**Date**: 2026-08-26 | **Environment**: none live (research + public docs sweep)
**Authored for**: at:harness-portability/12 (Cursor — the third harness column)

---

## 1. Scope

Two surfaces, reported separately where they differ. Ian's ruling (2026-08-26) is
**target BOTH, headless-first**: the headless `cursor-agent` CLI is the **primary**
surface (best fit to the desk/automation model and the isolation/evidence/review-gate
floor); the in-editor IDE agent is the **secondary** end-user surface.

| | `cursor-agent` (headless CLI) — PRIMARY | Cursor IDE agent — SECONDARY |
|---|---|---|
| **What** | Scriptable terminal binary (`cursor-agent -p/--print`, `--output-format text\|json\|stream-json`, `--model`, `--force/--yolo`); SSH/CI-capable | In-editor Agent/Composer + cloud background agents |
| **Distribution** | Install script / package; runnable headless in CI and over SSH | Cursor application (desktop) |
| **Isolation** | Inherits the shell's git; `git worktree add` runnable like any terminal (to confirm live) | Cloud **background agents** run in isolated git worktrees (Firecracker microVMs, ~8 parallel) |

Every verdict cites its probe: a URL + retrieval date for documentary evidence.

**No live Cursor environment is available (2026-08-26).** Verdicts are documentary
(Cursor's official docs + 2026 third-party write-ups, all dated). Rows whose truth
turns on a behaviour only a live install can settle are marked
**`[needs: live-install confirmation]`** in the verdict cell and re-listed in §5 — they
are *not* asserted as `supported`. All `absent`/`partial` verdicts carry the
positive-control probe that would have surfaced the capability had it existed.

**Two headline corrections to the stale "IDE vs terminal, Cursor has no skills/hooks"
framing** (both change the verdict, both dated in Part A of the scoping research):

1. **Cursor now has Skills, hooks, native `AGENTS.md`, and a headless CLI.** It adopted
   the identical `SKILL.md` open standard (agentskills.io) in **v2.4 (Jan 2026)**; it
   **reads `AGENTS.md` natively**; it shipped **hooks in v1.7 (Oct 2025)**; and it has a
   real scriptable headless CLI (`cursor-agent`).
2. Claude Code does **not** read `AGENTS.md` natively (only `CLAUDE.md`, via an import or
   symlink), so `AGENTS.md` is the *portable interchange* file, native on Cursor and
   Codex, imported on Claude Code.

The two products have converged to **~70–85% capability overlap** — a materially smaller
delta than Codex's was.

---

## 2. Capability matrix

Verbs: `supported` / `absent` / `partial` / `unmeasured` (with reason) /
`[needs: live-install confirmation]` (documented-as-supported, but the specific
behaviour Assay leans on is a live-only settle).

Documentary retrieval date for all cells: **2026-08-26**.

### 2.1 resident-rules-channel

AGENTS.md paths, composition, `.cursor/rules` mechanism.

| Surface | Verdict | How measured |
|---|---|---|
| CLI + IDE | `supported` | Doc: Cursor reads **`AGENTS.md` natively** (repo root + nested, root-to-leaf) and `.cursor/rules/*.mdc` rule files with four activation modes (`alwaysApply`, `description`-driven auto-attach, glob, manual); legacy `.cursorrules` is frozen but honoured. Two native resident-rules channels — the portable `AGENTS.md` fragment (shared with the Codex path) OR a `.cursor/rules/*.mdc` always-apply rule. (Source: [cursor.com/docs/context/rules](https://cursor.com/docs/context/rules), retrieved 2026-08-26) |

Positive control: the `.mdc` activation modes and native `AGENTS.md` reading are
documented on the rules page — absent would be no such documentation.

### 2.2 skills-discovery

Does the harness read `SKILL.md`-shaped skills; frontmatter contract.

| Surface | Verdict | How measured |
|---|---|---|
| CLI + IDE | `supported` | Doc: Cursor v2.4 adopted the **same `SKILL.md` open standard** (agentskills.io) Assay's plugin already uses; discovery from `.cursor/skills/` and `.agents/skills/` (plus user dirs); frontmatter `name` + `description`; progressive disclosure (metadata loaded, body on selection). (Source: [cursor.com/docs/skills](https://cursor.com/docs/skills), meshlaunch.com 2026 Cursor-Skills guide, retrieved 2026-08-26) |

Positive control: the `SKILL.md` frontmatter schema is documented in both Cursor's skills
reference and the agentskills.io standard — absent would be no schema docs.

### 2.3 auto-trigger

Description-driven invocation vs invoke-by-name only.

| Surface | Verdict | How measured |
|---|---|---|
| CLI + IDE | `[needs: live-install confirmation]` | Doc: skills carry a `description` used for selection (progressive disclosure), and `.cursor/rules` support description-driven auto-attach — so description-driven triggering is *documented*. Whether headless `cursor-agent` fires skill auto-trigger with the same ergonomics as the IDE (vs invoke-by-name) is a live-only settle. **Invoke-by-name is the floor** where auto-trigger is not confirmed — parity of *availability*, not of trigger ergonomics. (Source: [cursor.com/docs/skills](https://cursor.com/docs/skills), [cursor.com/docs/context/rules](https://cursor.com/docs/context/rules), retrieved 2026-08-26) |

### 2.4 session-start-hook

Any injection mechanism beyond AGENTS.md; headless hook-event coverage.

| Surface | Verdict | How measured |
|---|---|---|
| IDE | `supported` | Doc: `.cursor/hooks.json` shipped in **v1.7 (Oct 2025)** with `sessionStart`/`sessionEnd`, `pre/postToolUse`, `before/afterShellExecution`, `beforeMCPExecution`, `beforeSubmitPrompt`, `stop`, and subagent events. `sessionStart` is present. (Source: [cursor.com/docs/hooks](https://cursor.com/docs/hooks), infoq.com/news/2025/10/cursor-hooks, retrieved 2026-08-26) |
| CLI (`cursor-agent`) | `[needs: live-install confirmation]` | The headless CLI is **reported to drop `beforeSubmitPrompt`/`afterAgentResponse`/`stop`** in print mode (forum bug #169059, 2026). `sessionStart` is **not** on the reported-dropped list, so the resident-rules-via-hook path is *expected* intact headless — but this is exactly the row a live smoke must confirm, because Assay's only hook is `sessionStart`. (Source: forum.cursor.com CLI hooks bug #169059, retrieved 2026-08-26) |

Positive control: `.cursor/hooks.json` and the `sessionStart` event are documented in the
hooks reference — absent would be no hooks mechanism. **Note: Assay does not need this row
to be `supported` on Cursor** — the resident rules arrive via `AGENTS.md`/`.cursor/rules`
(2.1), not a hook. The hook row is a watch-item for any *future* hook-gated flow, not a
current blocker.

### 2.5 subagent-dispatch

Spawn/parallel; limits.

| Surface | Verdict | How measured |
|---|---|---|
| IDE | `supported` | Doc: in-session subagents (Task-style) + **cloud background agents in isolated git worktrees** (Firecracker microVMs, **~8 parallel**). `subagentStart`/`subagentStop` hook events exist. (Source: ssojet.com Cursor-3-agents write-up, cursor.com/docs, retrieved 2026-08-26) |
| CLI (`cursor-agent`) | `[needs: live-install confirmation]` | Headless dispatch of parallel workers (and whether the ~8 background-agent cap / worktree model is reachable from a scripted `cursor-agent` invocation) is a live-only settle. Where headless parallel dispatch is not confirmed, the **no-dispatch→serial-with-statement** floor applies: the fan-out runs serially, one item at a time, stating the degradation. (Source: [cursor.com/docs/cli/headless](https://cursor.com/docs/cli/headless), retrieved 2026-08-26) |

### 2.6 agent-messaging

Can a running/dispatched agent be messaged / followed-up.

| Surface | Verdict | How measured |
|---|---|---|
| IDE + CLI | `partial` | Doc: background/subagents accept follow-up messages in the IDE; headless follow-up over `cursor-agent` (`--resume`/session continuation) is documented but its durability across process restarts is a live-only settle — treat as `message-agent` present, durability unconfirmed. (Source: [cursor.com/docs/cli/headless](https://cursor.com/docs/cli/headless), retrieved 2026-08-26) |

### 2.7 background-notifications

Task-completion signals.

| Surface | Verdict | How measured |
|---|---|---|
| CLI | `supported` | Doc: `cursor-agent -p --output-format json\|stream-json` yields a structured completion signal a caller reads directly — the headless completion event is the notification. (Source: [cursor.com/docs/cli/headless](https://cursor.com/docs/cli/headless), retrieved 2026-08-26) |
| IDE | `supported` | Doc: `stop` / `subagentStop` hook events + in-app agent-complete signals. (Source: [cursor.com/docs/hooks](https://cursor.com/docs/hooks), retrieved 2026-08-26) |

### 2.8 sandbox-git

Which git verbs the agent may run.

| Surface | Verdict | How measured |
|---|---|---|
| CLI | `[needs: live-install confirmation]` | Doc: Cursor's permission surface is allowlist/denylist + Run-Everything (YOLO) + Cursor 3.6 Auto-review (allowlist→sandbox→classifier subagent). Under a permissive/YOLO posture the agent runs arbitrary terminal git (incl. `git worktree add`); under a tightened allowlist the adopter must permit the assay binaries and `git worktree add`. A reported `&&`-chaining allowlist-bypass edge case exists when tightening. Whether the default headless posture permits `git worktree add` unprompted is a live-only settle. (Source: outofcontext.dev / totalum.app Cursor-3.6-Auto-review write-ups, retrieved 2026-08-26) |
| IDE | `partial` | Same permission surface; cloud background agents run inside managed worktrees where git is available. (Source: same, retrieved 2026-08-26) |

Positive control: the allowlist/denylist/YOLO permission model is documented on Cursor's
security/permissions pages — a config posture, not a capability gap.

### 2.9 workspace-isolation

Worktree creation/detection — **the load-bearing capability for `worker-desk`.**

| Surface | Verdict | How measured |
|---|---|---|
| IDE | `supported` | Doc: cloud **background agents run in isolated git worktrees** (Firecracker microVMs, ~8 parallel) — native per-task isolation, unlike Codex CLI which *refused* it. (Source: ssojet.com Cursor-3-agents write-up, retrieved 2026-08-26) |
| CLI (`cursor-agent`) | `[needs: live-install confirmation]` | The headless CLI inherits the shell's git, so `git worktree add` is runnable in principle (like any terminal tool) — but whether that isolation is **reachable from the adopter's own headless flow** (and whether interactive/terminal `git worktree add` is permitted unprompted under the default sandbox, 2.8) is the single row a live install must settle. **If reachable, `worker-desk` runs on Cursor; if constrained, it refuses under the non-degradable isolation floor** — degraded never, silent never. (Source: [cursor.com/docs/cli/headless](https://cursor.com/docs/cli/headless), retrieved 2026-08-26) |

Positive control: the background-agent worktree model is documented; `git worktree add`
runnability under the headless permission posture is what §5 lists for live confirmation.

### 2.10 install-mechanism

How an adopter installs the bundle.

| Surface | Verdict | How measured |
|---|---|---|
| CLI + IDE | `supported` | Doc: Cursor reads `SKILL.md` skills from `.cursor/skills/` / `.agents/skills/`, `.cursor/rules/*.mdc`, `AGENTS.md`, `.cursor/mcp.json`, and `.cursor/commands/*.md` **directly from the repo tree** — so the adopt path is file-placement into the repo (the same `skills/` tree Claude Code and Codex use, plus the generated `.cursor/rules` fragment), no per-harness plugin manifest required. (Source: [cursor.com/docs/skills](https://cursor.com/docs/skills), [cursor.com/docs/context/rules](https://cursor.com/docs/context/rules), retrieved 2026-08-26) |

Positive control: the `.cursor/` config-file surface is documented — absent would be no
documented on-repo config path.

---

## 3. Summary verdict counts

| Verdict | CLI (primary) | IDE (secondary) |
|---|---|---|
| `supported` | 4 | 7 |
| `partial` | 1 | 2 |
| `[needs: live-install confirmation]` | 5 | 1 |
| `absent` | 0 | 0 |

**Zero `absent` rows** — a real shift from Codex CLI, whose `workspace-isolation` was
`absent`. Cursor's gaps are all "documented, live-confirm" rather than "missing".

## 4. Overall verdict

For a methodology built on Claude Code — shell-invoked CLI binaries in the terminal,
`AGENTS.md`/`CLAUDE.md` instructions, progressive-disclosure skills, MCP, hooks — transfer
to Cursor is **high, ~70–85% unchanged**. The portable core (terminal CLI + MCP + skills +
`AGENTS.md` + commands) moves with little friction; the enforcement/identity layer (exact
headless hook events, native filename, model family, permission config) is where an adopter
re-expresses rather than copies. **No identified hard blocker**, and the one hard guarantee
(isolation) is *more* likely to hold on Cursor than it did on Codex.

## 5. Rows requiring live-install confirmation (the `gate:human` smoke)

These are the cells above marked `[needs: live-install confirmation]` — documented as
present, but turning on a behaviour only a live Cursor install settles. They are **not**
asserted as `supported`; a live Cursor smoke run is the external-dependency acceptance step
(Ian's), the same posture the Codex stream held for its live smoke.

1. **Headless hook-event coverage (§2.4 CLI)** — does `cursor-agent -p` keep `sessionStart`
   (it reportedly drops `beforeSubmitPrompt`/`stop`)? Not a current blocker (Assay delivers
   resident rules via `AGENTS.md`/`.cursor/rules`, not a hook) but the row a live run confirms.
2. **Background-worktree isolation reachable from the adopter's flow (§2.9 CLI)** — does the
   headless `cursor-agent` permit `git worktree add` (2.8) so `worker-desk` gets a real
   isolated worktree? This is the row that decides whether `worker-desk` **runs** or
   **refuses** on Cursor under the isolation floor.
3. Auto-trigger ergonomics headless (§2.3), headless parallel dispatch reachability (§2.5),
   and default-posture `git worktree add` permission (§2.8) — secondary confirmations that
   sharpen the degradation cells but do not move the floor.

---

## Appendix A: Sources

| Source | URL | Retrieved |
|---|---|---|
| Cursor headless CLI (`cursor-agent`) | https://cursor.com/docs/cli/headless | 2026-08-26 |
| Cursor rules (`AGENTS.md`, `.mdc`, activation modes) | https://cursor.com/docs/context/rules | 2026-08-26 |
| Cursor Skills (`SKILL.md`, agentskills.io) | https://cursor.com/docs/skills | 2026-08-26 |
| Cursor hooks (`.cursor/hooks.json`, lifecycle) | https://cursor.com/docs/hooks | 2026-08-26 |
| Cursor CLI hooks bug (#169059, dropped headless events) | https://forum.cursor.com | 2026-08-26 |
| Cursor Plan Mode | https://cursor.com/blog/plan-mode | 2026-08-26 |
| Cursor MCP (`.cursor/mcp.json`, ~40-tool cap) | truefoundry.com / datamcp.app 2026 | 2026-08-26 |
| Cursor custom commands (`.cursor/commands/*.md`) | reflag.com/changelog, cursor.com/changelog/1-6 | 2026-08-26 |
| Cursor 3.6 Auto-review, Run-Everything | outofcontext.dev / totalum.app 2026 | 2026-08-26 |
| Cursor 3 background agents, Firecracker microVMs, worktree isolation, ~8 parallel | ssojet.com 2026 | 2026-08-26 |
| Cursor Skills 2026 guide (v2.4 adoption) | meshlaunch.com 2026 | 2026-08-26 |
| Claude Code reads CLAUDE.md not AGENTS.md; import/symlink | https://docs.claude.com/en/memory | 2026-08-26 |
