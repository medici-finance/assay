---
brief: assay-dogfood/01
title: Bootstrap ../assay-toolkit — repo, plugin scaffold, marketplace manifest, governance permissions
wave: 0
depends: []
unblocks: ["assay-dogfood/02", "assay-dogfood/03"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
decision-issue: 723
schema: brief-v1
authored: 2026-07-10 by Fable desk session ([I-30](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-dogfood-the-methodology-via-the-assay-marketplace-new-initia.md))
sources: ["INTAKE [I-30](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-dogfood-the-methodology-via-the-assay-marketplace-new-initia.md)", "INTAKE [I-24](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-methodology-self-containment-assay-tools-as-the-externally-g.md) (phasing + marketplace-primary amendment 2026-07-10)", "PR #206 comments 2026-07-10 (shadowing + marketplace trade-offs)", "freshness-checked 2026-07-10 @ fb9223ce"]
gate-why: >-
  Two acts here are human:<name>'s: creating the org repo, and setting the permission model that IS
  the governance boundary (consumer-repo agent identities get read + issues only — "agents
  can't change the methodology" becomes a 403, not discipline). Signing off the permission
  matrix is signing off who can rewrite the rules that govern every agent session.
why: >-
  Nothing distributes until the source-of-truth repo exists with its permission boundary in
  place. This is the head of the stream's critical path, and it is deliberately human-gated:
  the entire integrity story downstream ([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)'s answer at the methodology layer) rests on
  the permissions set here.
---

# Brief 01 — Bootstrap ../assay-toolkit

> **[F-26](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-10-assay-tools-merged-into-assay-toolkit-toolkit-survives.md) (2026-07-10):** the bootstrap was first laid down as a separate local repo
> (`assay-tools`); human:<name> directed it be merged into the EXISTING `medici-finance/assay-toolkit`
> (which survives). The scaffold now lives there (merge commit `b09c6b8`, preserving bootstrap
> commit `488a889`; distribution/governance doc at `../assay-toolkit/docs/distribution.md`). This brief's
> remaining work is the PERMISSION-MODEL half against `medici-finance/assay-toolkit` — repo
> creation is superseded (it exists, private); the reviewer-App question is an OPEN TENSION
> with [F-23](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-10-review-verify-gates-cover-medici-finance-report-repos.md) (gates cover assay-toolkit as a report repo) recorded in `../assay-toolkit/docs/distribution.md`,
> resolved by human:<name>.

## Context
files: repo `medici-finance/assay-toolkit` (private; worked locally at `../assay-toolkit` —
scaffold already merged in, see [F-26](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-10-assay-tools-merged-into-assay-toolkit-toolkit-survives.md) note above); this repo's PR carries only this brief's
row/Evidence updates
facts:
- Repo skeleton: `plugins/assay/` (the plugin: `.claude-plugin/plugin.json`, `skills/`,
  `hooks/`), `.claude-plugin/marketplace.json` at repo root (lists the `assay` plugin),
  `tools/` (statusgen relocation target, brief 03), `README.md` (what this repo is, the
  governance model, how to propose changes = issues here), `LICENSE` = **Apache-2.0**
  (human:<name> chose OSS over the BSL option, 2026-07-10, #262).
- Permission model: human:<name> = admin; the reviewer App is NOT installed here (its scope stays the
  consumer repos). **As applied (human:<name>, 2026-07-10, #262):** the repo lives in the
  `medici-finance` org, where the `the-org` machine account is an org member and therefore
  holds **write/admin** — an accepted, **time-boxed exception until Sep 2026**, NOT the
  read-only 403 boundary described as the target model below. (A per-repo Read grant cannot
  override org membership, so enforcing read+issues-only would require the repo to live
  outside an org `the-org` administers — deferred, not done.)
- Plugin naming: plugin `assay`, so skills surface as `assay:<name>` — the namespacing that
  structurally kills the personal-shadows-project hazard (PR #206 comment, 2026-07-10).
- Versioning: semver tags from v0.1.0; marketplace.json pins by tag; consumers upgrade by
  explicit version bump (two-human-gate composition with desk-tools C-1 stays: toolkit
  release + consumer bump).
- Bootstrap circularity note ([I-24](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-methodology-self-containment-assay-tools-as-the-externally-g.md)): assay-toolkit itself is a Claude-worked repo; until the
  plugin exists it is worked with the current loose-file methodology — acceptable, recorded.
- What does NOT move here yet: desk-tools binaries ([I-24](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-methodology-self-containment-assay-tools-as-the-externally-g.md) phase ④, later), the registers/
  briefs of consumer projects (never — state stays with the project).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Repo creation, permission
  grants, and the first push are IAN'S acts — the implementer prepares content locally and
  stops. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Prepare the full repo content locally at `../assay-toolkit` (git init, no remote): skeleton
   per facts, marketplace.json + plugin.json validated against the plugin docs schema,
   README with the governance model and change-proposal flow.
2. Write the permission matrix as a checklist in the README (who: human:<name> / agent identity /
   reviewer App; what: admin / read+issues / absent) for human:<name> to apply in GitHub settings at
   creation.
3. Hand human:<name> the create-push-permission runbook (3 commands + settings clicks); record his
   execution + the resulting repo URL in Evidence.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `jq -e '.plugins[0].name == "assay"' .claude-plugin/marketplace.json` | exit 0 |
| 2 | `jq -e '.name' plugins/assay/.claude-plugin/plugin.json` | exit 0, prints "assay" |
| 3 | `gh api repos/medici-finance/assay-toolkit --jq .permissions` run AS the `the-org` identity | repo exists (private); reflects human:<name>'s accepted model — the-org = write via `medici-finance` org membership, accepted until Sep 2026 (#262). The read-only boundary is the target, not the current state. |
| 4 | `grep -ci "read + issues\|read+issues" README.md` | ≥1 (governance model documented) |
| 5 | `statusgen --root . --lint; echo $?` (this repo) | 0 |

## Evidence

**Non-implementer verifier run — VERIFY: PASS** · 2026-07-20 · `glm-5.2-verifier` · merged main `73d01752`

**assay-toolkit repo**: medici-finance/assay-toolkit · **SHA verified at**: `34bdc2cb416d20109f0b504429f63d45e1afea92` (`origin/main`, "Merge pull request #75"). Verified in an isolated clone `/private/tmp/verify-ad01-toolkit-main`; the shared sibling checkout was **not** mutated.

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `jq -e '.plugins[0].name == "assay"' …/marketplace.json` | 0 | `true` |
| 2 | `jq -e '.name' …/plugins/assay/.claude-plugin/plugin.json` | 0 | `"assay"` |
| 3 | `gh api repos/medici-finance/assay-toolkit --jq '{permissions,private,license}'` (identity `the-org`) | 0 | `{"permissions":{"admin":true,maintain":true,"pull":true,"push":true,"triage":true},"private":true,"license":"Apache-2.0"}` — matches #262 accepted model (the-org write/admin via org membership, time-boxed to Sep 2026; not the read-only 403 target boundary) |
| 4 | `grep -ci "read + issues" …/README.md` | 0 | `2` (≥1) — permission matrix present (carried since 2026-07-18 re-verify) |
| 5 | `go run ./tools/statusgen --root . --lint` | 0 | NOTICEs only; no lint errors |

**Status: stays `implemented`.** `gate: human` (risk answers all `no`) — a model verifier records Evidence but **cannot** flip to `verified`; human:<name>'s sign-off is captured in decision issue #723 and the existing #262 acceptance (time-boxed to Sep 2026). The 2026-07-13 opus-verifier FAIL on row 4 (permission matrix missing from README) remains remediated; the 2026-07-18 glm-5.2 PASS state holds. Surface for human:<name>'s checkpoint PR.

**Re-confirm 2026-07-20 (glm-5.2-verifier, merged main `700e1c9e`):** all 5 rows still PASS against the live sibling `assay-toolkit` (HEAD `df09c23`). **DELTA:** `gh api repos/medici-finance/assay-toolkit` now reports `private: false` (repo is **public**), whereas row 3's Expect + prior Evidence recorded `private: true`. The permission model (the-org admin/write via `medici-finance` org membership, #262) and Apache-2.0 license are unchanged. The public flip is consistent with the OSS decision (#262 chose Apache-2.0 over BSL) but should be confirmed intentional by human:<name>.

### Human sign-off artifact + open confirm — verify-desk, 2026-07-27

`gate: human` sign-off artifact: **#723, Option A, 2026-07-18 human:ian** ("Sign off — accept the brief as-is") — accepting the #262 permission model (the-org write/admin via `medici-finance` org membership, time-boxed to Sep 2026) as the governance boundary, with the read+issues-only 403 model recorded as the deferred target. **Open confirm (carried in the checkpoint PR):** the repo flipped `private: true → false` (observed 2026-07-20, AFTER the #723 sign-off) — consistent with the #262 Apache-2.0 OSS decision but explicitly flagged for human:<name>'s confirmation; row 3's literal "(private)" note is stale by that flip.

### Public-flip outcome — human:<name>'s decision, 2026-07-27 (recorded from checkpoint PR #1438)

On the checkpoint PR's open confirm (repo flipped `private: true → false` after the #723 sign-off), **human:<name> decided: assay-toolkit was never meant to be public** — a separate PUBLIC `/assay` repo is being created with the needed content copied over (human:<name>, 2026-07-27, PR #1438 comment). The reviewer App recorded: "the premise is withdrawn, not ratified." The brief's verified flip proceeds on the bootstrap + permission-model substance (#723); repo-visibility remediation rides the /assay migration, not this brief.
