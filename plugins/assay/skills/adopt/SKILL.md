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
  Claim only that (**not** "tamper-evident"): guide **§1a** has the three recorded reasons the
  stronger claim is false; "tamper-evident" and kin are retired as overclaims.
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

## Carry the honest framing
The board is *derived from agent-authored artifacts with linting + independent re-verification* — **not**
measured from ground truth. When you report what you installed, claim the weaker true thing.
