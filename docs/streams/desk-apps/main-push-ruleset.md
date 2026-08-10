# Main-push ruleset (F-13, server-side)

Deliverable of [brief-08 — F-13 server-side main-push ruleset](./brief-08-main-push-ruleset.md).
**Filled at execution (human:<name> applies).**

GitHub repo ruleset restricting pushes to `main` to `{assay-verifier-app[bot],
assay-desk-app[bot], ianholsman}` (verify human:<name>'s exact handle at apply time). Replaces the
client-side `ASSAY_MAIN_COMMIT_OK` hook (F-13) as the primary gate; the client hook stays as
defense-in-depth.

> **Not applicable yet — plan-blocked.** Verified 2026-08-02: the rulesets and branch-protection
> APIs both return **HTTP 403** (*"Upgrade to GitHub Pro or make this repository public"*) for
> `medici-finance/assay-toolkit` and `oit` — free org plan +
> private repos. There is no ruleset surface to apply this to until the plan or the repo
> visibility changes (human:<name>'s call). Until then the sanctioned-lander set below is **convention**,
> backed only by the client-side hook. See
> enforcement-model.md.

## Ruleset (JSON)

_(filled at brief-08 execution — the "restrict pushes" ruleset body, allowed-actors list)_

## Apply steps (per repo)

_(filled at brief-08 execution — Settings → Rules → per repo: oit, agents, examples, + the
medici-finance report repos)_
