---
brief: assay-launch/07
title: Automated publish pipeline — daily jobs push into assay-site, no manual step
why: >-
  human:<name>'s requirement: "the daily jobs need to be able to publish into [assay-site] without manual
  intervention." A metrics page that shows "how it runs" is only credible if it stays current on
  its own — a stale snapshot that needs someone to remember to regenerate it is worse than none.
  This wires the existing daily harvest to regenerate the public snapshot and push it to
  assay-site, where Cloudflare Pages redeploys automatically, with the leak guard as the safety.
wave: 2
depends: ["assay-launch/06", "assay-launch/03", "methodology-metrics/22"]
unblocks: ["assay-launch/05"]
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-17 by Opus session (human:<name>'s assay-site auto-publish direction)
sources: ["human:<name> 2026-07-17: 'the daily jobs need to be able to publish into it (without manual intervention)'", "methodology-metrics/22 (daily artifact harvest — the scheduled AI-free collector this extends; .github/workflows/daily-harvest.yml)", "assay-launch/06 (the assay-site repo + predeploy-check.sh this pushes to)", "assay-launch/03 (the metrics page + its one-command regen step)", "../site-repo (Cloudflare Pages serves byte-for-byte — a push IS the deploy; no build step)", "cross-repo pairing #272 (workflow in this repo pushes to assay-site — record both SHAs)"]
gate-why: >-
  customer-facing standing automation + a cross-repo write credential: this creates a pipeline
  that publishes to a PUBLIC site with no human in the loop, and it needs a write credential to
  the assay-site repo (a deploy key / scoped token) that only human:<name> can provision as a secret.
  An automated publisher is exactly where a leak or a mis-publish becomes un-reviewable, so the
  pipeline MUST fail closed on predeploy-check.sh and human:<name> confirms: the credential scope, that
  the auto-push only carries the aggregate metrics snapshot (no per-user/internal data), and that
  it goes live only after the first launch (assay-launch/05).
exec-tier: strong
exec-tier-why: (c) CI + credential + auto-publish plumbing where a subtle error silently leaks internal detail or publishes wrong content to a public site.
---

# Brief 07 — Automated publish pipeline

**CROSS-REPO (paired, #272):** the workflow change lands in THIS repo
(`../oit/.github/workflows/daily-harvest.yml`, shared with methodology-metrics/22); it pushes generated
content into `../assay-site` (assay-launch/06). Record both repo SHAs in Evidence.

## Context
files: `../oit/.github/workflows/daily-harvest.yml` (extend methodology-metrics/22's harvest job — do
NOT fork a second scheduled workflow), a small publish script
`scripts/publish-assay-metrics.sh` (planned), `../assay-site/` (push target)
facts:
- **Extend the existing daily harvest, don't add a second scheduler.** methodology-metrics/22
  owns `daily-harvest.yml` (scheduled, AI-free, `ubuntu-latest`, `[skip-status-regen]` marker
  discipline). Add a publish STEP: regenerate the metrics snapshot (assay-launch/03's one-command
  regen from `statusgen --dora --json` / harvest artifacts), then push it into `assay-site`.
- **Fail closed on the leak guard.** The step runs `../assay-site/predeploy-check.sh` (06) BEFORE
  any push; a non-zero exit aborts the publish — no push, no deploy. This is the whole safety
  model of an unattended publisher.
- **A push IS the deploy.** Cloudflare Pages serves `assay-site` byte-for-byte; pushing the
  regenerated snapshot to the deploy branch triggers the redeploy — no build step, no manual
  action. Only the metrics snapshot is auto-updated; the hand-authored pages/landing are not
  touched by the job.
- **Credential is human:<name>'s.** The job needs write to `assay-site` (a deploy key or a fine-grained
  token stored as a repo secret). The implementer wires the workflow to read the secret by name
  and documents the exact secret + scope needed; human:<name> creates it. Until then the job runs in
  dry-run (regenerate + predeploy-check + `git diff`, no push).
- **Goes live after launch.** The pipeline is built + tested in dry-run now; the first real
  auto-push happens only after assay-launch/05 (first launch). Guard the push behind the secret's
  presence so it is inert until human:<name> provisions it.

## Ground rules
- NEVER git push to a public remote / create secrets / trigger the workflow for real. Build +
  dry-run only; report BLOCKED-ON-IAN for the write credential + enabling.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- docs-site: if this changes operational tooling docs, note the pipeline in the ops runbook.

## Task
1. Add a publish step to `daily-harvest.yml` (coordinate with methodology-metrics/22 — same file):
   regenerate the metrics snapshot, run `../assay-site/predeploy-check.sh`, and — only if a named
   `assay-site` write secret is present — push the snapshot to the assay-site deploy branch.
2. Implement the publish script (fail-closed on predeploy-check; dry-run `git diff` when the
   secret is absent).
3. Document the exact secret name + scope human:<name> must create, and the enable step, in the runbook +
   the BLOCKED-ON-IAN report.
4. Update the stream-README row; record the dry-run output + both repo SHAs in Evidence.

## Verify (executable — dry-run + safety gates; the live push is post-launch/human:<name>)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c "predeploy-check" scripts/publish-assay-metrics.sh` | ≥1 (leak guard invoked before any push) |
| 2 | `bash -n scripts/publish-assay-metrics.sh; echo $?` | 0 (script parses) |
| 3 | `yq eval '.' .github/workflows/daily-harvest.yml > /dev/null; echo $?` | 0 (workflow YAML valid) |
| 4 | `grep -ciE "secret" .github/workflows/daily-harvest.yml` | ≥1 (push gated behind a named secret — inert until human:<name> provisions it) |
| 5 | `ASSAY_SITE_TOKEN= bash scripts/publish-assay-metrics.sh --dry-run 2>&1 \| grep -ciE -e "dry-run" -e "no token"` | ≥1 (fails safe / dry-runs when the secret is absent) |
| 6 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- one row per Verify item, filled by a non-implementer. Both repo SHAs (this repo +
     assay-site) recorded here. The first live auto-push is post-launch, recorded by human:<name>. -->

## Review
Gate: **human** (customer-facing standing automation + cross-repo credential). Reviewer confirms
fail-closed-on-leak-guard, inert-without-secret, snapshot-only scope, and the credential ask; the
`done` Reviewed cell is `human:ian`.
