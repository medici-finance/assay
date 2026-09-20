---
name: adopt
description: >-
  Use when installing or adopting the Assay methodology into a project — "install Assay",
  "adopt the streams/briefs methodology here", "set up the board/desks", "onboard this repo
  onto Assay", or standing Assay up across several repos, or carving a subsystem out of an
  existing project into its own Assay-tracked unit. Routes to the right adoption scenario
  (green-field / existing-suite / carve-out) and holds the install PRIMITIVEs + the
  human-gate escalation points. Read the full runbook at docs/adopting-assay.md.
---

# Adopt Assay — install runbook

You are installing the Assay methodology. This skill is the operational entry; the full,
step-by-step runbook with exact commands and a Verify check per step is
**`docs/adopting-assay.md`** in your `assay` checkout. Follow that guide — this skill routes
you and holds the non-negotiables so you don't miss them.

## 1. Pick ONE scenario
- **Green-field** — a brand-new repo (or small set), no history to preserve → guide **SCENARIO 1**.
- **Existing suite** — 2+ repos that already have code/history/CI → guide **SCENARIO 2**.
- **Carve-out** — extract part of an existing project into its own Assay-tracked unit → guide **SCENARIO 3**.

If unsure between a repo split and a stream-set, do the reversible thing first (a stream-set in place),
not the irreversible one (a repo split with rewritten history).

Scenario is one axis; **harness is the other** — the method text is harness-neutral, only the
container differs. **Claude Code** → the CORE PRIMITIVEs in §2. **Codex CLI** → §5 below plus the
guide's **"Running Assay on Codex"** section (the plugin-install primitive in §2 is the Claude Code
arm and does not run there). Cursor and any other harness the guide's table covers → route by that
table.

## 2. Install the CORE PRIMITIVEs (idempotent; run each one's Verify before the next)
`install-statusgen` · `scaffold-registers` · `scaffold-streams` · `add-statusgen-ci` · `install-desk-plugin`
· `install-main-guard` · `first-board` · `setup-reviewer-app`. Full commands + Verify checks: guide **CORE §3**.

> **`install-statusgen` installs the sha256-pinned release binary** (`.assay-versions` + `gh release
> download`) — it does **not** copy `statusgen/` source. Vendoring is retired as a recommendation; the primitive was named `vendor-statusgen` until 2026-08-02.

> **BOOTSTRAP-SAFE GUARD (required in `add-statusgen-ci`):** the regen step must guard on
> `git status --porcelain -- STATUS.md`, **never** `git diff --quiet -- STATUS.md` — the latter can't
> see the untracked first board, so a fresh repo never generates one.

## 3. NEVER autonomous — STOP and escalate to a human
- **Reviewer GitHub App** creation/install — the identity that posts approvals, which a plain worker
  session cannot post as; a placeholder or self-minted stand-in defeats the entire mechanism.
  Claim only that (**not** "tamper-evident" — a retired overclaim): guide **§1a** has the three
  recorded reasons the stronger claim is false; "tamper-evident" and kin are retired as overclaims.
- **Repo creation** + admin/permission grants.
- **Merge to main / `git push origin main` / release tag / the first ready-flip.**
- **git history rewrite** (`filter-repo`) for a carve-out.
- **Private-repo CI auth** (`GOPRIVATE` / cross-repo checkout token).

Hand the human the exact values (App name + permissions, repo slug + module path, etc.), wait for
confirmation, and never fabricate the outcome. The agent authors branches and opens **draft** PRs only.

## 4. Prove the install
Walk ONE trivial seed brief through the FULL lifecycle — `todo → in-progress → implemented → verified →
done` — so the desks, the board, the reviewer App, and the human-merge gate each fire exactly once
(guide's "hello-world loop"). If that completes, the machinery works.

## 5. Codex CLI — the second-harness install arm

Same method, different container. The guide's **"Running Assay on Codex"** section owns the exact
commands and the per-step Verify; hold these non-negotiables:

- **Not the plugin commands.** The §2 plugin-install primitive is the Claude Code arm. Codex CLI has
  its own plugin/marketplace arm (demonstrated end to end on codex-cli 0.154.0) and a plain
  file-placement arm — pick per the guide's Codex §1, never by transplanting Claude commands.
- **File-placement arm copies BOTH directories**: `plugins/assay/skills/` into the adopter repo at
  `.agents/skills/` **and** `plugins/assay/references/` as its sibling — the bodies'
  `../../references/*.md` includes go dead if only `skills/` is copied.
- **Resident rules travel in `AGENTS.md`, not a hook.** Install the bundle's generated fragment
  `plugins/assay/codex/AGENTS-assay.md` into the adopter repo's root `AGENTS.md` — append it, or drop
  it in as an `@`-include. It is GENERATED from the single resident-rules source (`harnessgen
  resident`, byte-checked in CI): never hand-edit it, never retype the rules into your own file.
  Mind the composition cap — Codex truncates the combined `AGENTS.md` (32 KiB by default), so a long
  existing file can silently push the method's rules past the cut. Verify the way the guide's Claude
  side does: start a session and ask it to state a resident rule without being handed the text.
- **The dispatch-config step is part of the install** (guide Codex §3 + the binding file): the
  subagent feature flag has graduated on codex-cli 0.154.0 and no longer gates dispatch — the control
  that matters is the per-session concurrency cap: a desk pool wider than the cap queues rather than
  fans out, and a cap of 1 serializes outright. Exact config keys live in the binding file, not
  here. The non-negotiable is the *stated* degradation: where parallel dispatch is unavailable, the
  fan-out runs serially, one item at a time, and says so in session — never silently.
- **Sandbox floor.** Under the default `workspace-write` sandbox, branch/ref writes and network are
  blocked, so skills that must create isolated worktrees or write to the forge REFUSE rather than
  half-run. That refusal is correct behaviour — do not work around it; those windows need the
  full-access sandbox posture the guide names.
- Per-skill posture — `runs` / `degrades` / `refuses` — is canonical in
  [`plugins/assay/references/codex.md`](../../references/codex.md). Read it there; never reproduce
  the table.

## Carry the honest framing
The board is *derived from agent-authored artifacts with linting + independent re-verification* — **not**
measured from ground truth. When you report what you installed, claim the weaker true thing.
