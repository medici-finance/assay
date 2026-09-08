---
id: DR-harness-code-dehouse
date: "2026-09-08"
title: "Land harness-portability/14's code de-house (44 files, neutralised) as briefed"
consequence: critical
decided-by: "human:<name>"
alternatives:
  - "Approve with changes — re-dispatch the brief with folded-in changes before it lands — ruled out: no changes were requested against the briefed neutralisation, staging (37 clean files first, then the 7 rewritten files), or defense-in-depth design; the ratifying comment approves it as written."
  - "Hold / reject — park the item at its human gate pending further review — ruled out: the ratifying comment is an affirmative approval (\"lgtm\"), not a hold, so the de-house is clear to land and the five held briefs (01/02/06/07/12) are clear to move toward their own public Evidence runs."
accepted:
  - "The private tree's copies of these 44 files are not retired by this landing. They remain a named follow-on per the brief's own `consumers:` list, so a residual house-tree duplicate persists until that follow-on lands."
  - "The reviewer's caveat — \"don't remember the cursor scratch repo, might need to re-run if you need it\" — is accepted as an open verification note, not a blocking condition: a later verifier re-running Verify rows 2a/3a (the canary scratch trees) may need to recreate that scratch state rather than assume one already exists."
---

The driver (`human:<name>`) ratified harness-portability/14 — the code de-house that
copies 44 files (three Go modules, the bundle's provenance/packaging files, two capability
matrices, the smoke protocol) from the private source tree into this public tree — and,
with it, the four desk-proposed harness-degradation cells in
`plugins/assay/references/{claude-code,codex,cursor}.md` (`ask-decision`, `install`,
`pdfingest`, `upgrade-assay`) that had carried a "(proposed — pending the driver's ruling
on #626)" marker pending exactly this decision.

The ruling was recorded as a comment on the brief's own decision-gate issue,
[issue #626](https://github.com/medici-finance/assay/issues/626#issuecomment-5588628639):

> lgtm. don't remember the cursor scratch repo. might need to re-run if you need it

Issue #626 offered three options — approve as briefed, approve with changes, or hold /
reject — per the decision-gate template. The comment above is option 1: approve as
briefed.

This closes the human gate the brief's frontmatter declares (`gate: human`, `risk:
{irreversible: yes, sensitive-data: yes}`). Per the brief's own `gate-why`, the human's
approval confirms: (1) the neutralisation of the 7 previously-withheld files rewrote the
provenance narrative rather than deleting it; (2) all three leak layers (the local sweep,
the control-based commit status, and the in-repo pattern/structural gates) ran and agreed,
not just the one the author drove; and (3) the residual house-tree copies are being
retired deliberately in a named follow-on, not orphaned.

**What this record does not decide:** retiring the private tree's duplicate copies (a
house-side change to a repository this brief does not touch, named as a follow-on in the
brief's `consumers:` list) and the public Evidence runs for briefs 01, 02, 06, 07 and 12
(each is its own verify-desk item once this lands, per the brief's own wording).
