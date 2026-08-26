# Cursor — capability bindings

How each neutral-core capability (named in the skill bodies as `capability:<name>`)
is realised on **Cursor**. Target set is **both surfaces, headless-first** (Ian,
2026-08-26): the headless `cursor-agent` CLI is the **primary** surface (best fit to
the desk/automation model and the isolation/evidence/review-gate floor), the in-editor
IDE agent the **secondary** end-user surface. Harness tool names are legal here — that
is this file's whole purpose — and illegal in a skill body (`tools/harnesslint bodies`
enforces it).

Every mechanism and degradation cell below cites the measured Cursor capability matrix
(HP/12, documentary + public-docs sweep, 2026-08-26 — **no live Cursor environment**;
the rows a live install must settle stay flagged `[needs: live-install confirmation]`).

Source of the capability set: the closed vocabulary block in
[`../../../docs/streams/harness-portability/README.md`](../../../docs/streams/harness-portability/README.md).

## The non-negotiable floor

Three guarantees **never degrade** on any harness (stream README §3; ruling C):
**isolation**, **evidence-not-claims**, and **the review gates**. Where Cursor
cannot provide the underlying capability, the affected skill **refuses** rather
than dropping the guarantee. Only *convenience* gaps (no parallel dispatch, no
reliable background notification) may degrade — explicitly, and in-session.

## Why Cursor is lighter than Codex

Cursor's 2026 convergence (native `AGENTS.md`, the same `SKILL.md` open standard,
hooks, MCP, background agents in isolated git worktrees) makes it a *smaller* lift than
Codex. Cursor's capability matrix has **zero `absent` rows** (HP/12 §3) — where Codex
CLI's `workspace-isolation` was `absent` and forced `worker-desk` to refuse, Cursor's
background agents run in isolated git worktrees natively. So more skills `run` here.

## Capability → mechanism

| Capability | Cursor mechanism (HP/12 §) |
|---|---|
| `capability:dispatch-worker` | IDE: in-session subagents + **cloud background agents in isolated git worktrees** (Firecracker microVMs, ~8 parallel) (§2.5). Headless (`cursor-agent -p`): scriptable invocation; whether parallel headless dispatch reaches the ~8-agent worktree model is `[needs: live-install confirmation]` (§2.5). **Where headless parallel dispatch is not confirmed**, the convenience-degradation floor applies: the fan-out runs **serially, one item at a time, stating the degradation**. |
| `capability:message-agent` | Follow-up messages to a background/subagent in the IDE; headless follow-up via `cursor-agent` session continuation (`--resume`) — present, durability across process restarts is `partial`/unconfirmed (§2.6). Prefer re-dispatch fresh when durability matters. |
| `capability:isolate-workspace` | IDE: cloud background agents run in **isolated git worktrees** natively (§2.9 `supported`) — the isolation Codex CLI lacked. Headless: `cursor-agent` inherits the shell's git, so the skill runs `git worktree add` itself as on Claude Code; whether the default headless permission posture permits that ref-write unprompted (§2.8) — and whether the background-worktree isolation is reachable from the adopter's own flow — is `[needs: live-install confirmation]` (§2.9). **If reachable, isolation runs; if the sandbox blocks the worktree ref-write, the no-isolation floor applies and the affected skill refuses rather than implement in the shared checkout.** |
| `capability:invoke-skill` | `SKILL.md` discovery from `.cursor/skills/` and `.agents/skills/` — the **same `SKILL.md` open standard** (agentskills.io, Cursor v2.4) Assay's plugin already uses (§2.2 `supported`); frontmatter `name` + `description`, progressive disclosure. Description-driven auto-trigger is documented; whether headless `cursor-agent` fires it with IDE ergonomics is `[needs: live-install confirmation]` (§2.3) — **invoke-by-name is the floor** where auto-trigger is unconfirmed (parity of availability, not of trigger ergonomics). |
| `capability:session-notifications` | Headless: `cursor-agent -p --output-format json\|stream-json` returns a structured completion signal the caller reads directly (§2.7 `supported`). IDE: `stop`/`subagentStop` hook events + in-app agent-complete signals (§2.7 `supported`). Judge liveness by these signals and elapsed time, never by an empty output file. |

**Resident-rules channel** (not one of the five dispatch capabilities, but the
load-bearing delivery mechanism): Cursor reads **`AGENTS.md` natively** and
`.cursor/rules/*.mdc` (§2.1 `supported`). Two native channels: the portable
`AGENTS.md` fragment (**shared** with the Codex path — Cursor reads AGENTS.md
natively) OR the generated `.cursor/rules` always-apply rule (`harnessgen cursor`).
Assay's only hook is a session-start resident-rules injection, which Cursor does
**not** need — the rules arrive through `AGENTS.md`/`.cursor/rules`, not a hook, so the
headless hook-drop caveat (§2.4) is a watch-item for future hook use, never a current
blocker. (The shared `AGENTS.md` fragment and the generated `.cursor/rules` rule are
produced by `tools/harnessgen`, which arrives with the tool de-house follow-on.)

## Degradation — per skill (Cursor, both surfaces, headless-first)

Reproduced from the ruled matrix (ruling C floor). Cursor carries **fewer
degradations than Codex** because isolation is native on the IDE surface and the one
`refuses` risk is a headless-permission live-confirm, not a measured `absent`.

| Skill | Cursor |
|---|---|
| `adopt` | **runs** — needs only `resident-rules-channel` §2.1, `skills-discovery` §2.2, `install-mechanism` §2.10, all `supported`; no dispatch/isolation/gate surface. |
| `author-brief` | **runs** — pure authoring/planning text; touches no dispatch, no isolation, no gate. |
| `dailies` | **runs** — report generation is sequential CLI work; any report fan-out **degrades: runs serially with an explicit in-session statement** if headless parallel dispatch is unconfirmed (§2.5); dispatch is a convenience here, not a safety guarantee. |
| `intake-desk` | **runs** — triage/classification is interactive text plus `gh` reads/writes; `gh` write verbs need the adopter's permission posture to allow them (§2.8) — a config precondition, not a gate degradation. |
| `market-intelligence` | **runs** — research + reads; no dispatch/isolation/gate needs. |
| `pr-review-desk` | **runs** — the review **verdict** is text and is never degraded; posting the review and the draft→ready flip need `gh` writes → permission-posture precondition (§2.8). The review gate is upheld or the skill refuses; it never silently degrades. |
| `the-desk` | **runs** — coordination; its worker fan-out **degrades: serial with an in-session statement** if headless parallel dispatch is unconfirmed (§2.5); the review/merge gate is unchanged (the human merges). Isolation of dispatched implementers is delegated to `worker-desk` (below). |
| `verify-desk` | **runs** — executes the Verify table and records **real command evidence** (`evidence-not-claims` intact); `gh` writes need the permission posture (§2.8). The evidence guarantee is never degraded. |
| `worker-desk` | **runs on the IDE surface** — cloud background agents provide **native isolated git worktrees** (§2.9 `supported`), so the pool isolates each implementer as the floor requires. **Headless (`cursor-agent`): conditional on `[needs: live-install confirmation]`** — if the headless posture permits `git worktree add` (§2.8) so the skill can create its own isolated worktree (as on Claude Code), it **runs**; if the sandbox blocks that ref-write, per the no-isolation→refuse floor it **refuses rather than implement in the shared checkout** (the outcome Codex CLI hit). Dispatch gap → **degrades: the pool runs serially, one brief at a time, with an explicit in-session statement**. Isolation is never silently degraded. |

**Floor check** — isolation: `worker-desk` **runs** on the IDE surface (native worktrees)
and, headless, either runs or **refuses** when it cannot isolate (never degrades).
Evidence: `verify-desk` **runs** with real evidence (never degraded). Review gates:
`pr-review-desk` / `the-desk` uphold the gate, merge stays human (never degraded).
Dispatch (a convenience) degrades to serial-with-statement in `worker-desk`, `the-desk`,
and `dailies` fan-out only where headless parallel dispatch is unconfirmed — the one
permitted degradation, always stated.

## Freshness

This binding is behavioural fact about a live harness, measured documentarily on
2026-08-26 with no live environment. It is registered in `freshness.yaml`
(harness-portability/12) so it expires on a clock, and the
`[needs: live-install confirmation]` rows are re-measured once a Cursor environment
exists (the live smoke is the stream's external head, Ian's).
